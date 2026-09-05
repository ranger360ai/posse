#!/usr/bin/env bash
# suite-lock.sh — at most N full `go test ./...` runs on this box at once
# (ranger-base-uvzjk).
#
# Usage, sourced by a test wrapper:
#     . "$(dirname "$0")/suite-lock.sh"
#     suite_lock_acquire "$@"      # blocks until a slot is free, or returns
#     "$@"                         # at once when this argv is not a full suite
#     suite_lock_release
#
#        scripts/suite-lock.sh --self-test    prove the slots work
#        scripts/suite-lock.sh --status       who holds the slots right now
#
# Environment:
#   POSSE_SUITE_SLOTS=2        full suites allowed to run at once. A value
#                              that is not a positive integer is named on one
#                              line and the default is used
#                              (_suite_lock_slots says why)
#   POSSE_SUITE_LOCK_DIR=...   where the slot files live
#                              (default ${XDG_CACHE_HOME:-~/.cache}/posse)
#   POSSE_SUITE_LOCK_POLL=5    seconds between attempts while queued
#   POSSE_SUITE_LOCK=0         run unserialized, and say so on one line
#   POSSE_SUITE_LOCK_HELD      set by an acquire; a nested wrapper reads it
#                              and does not take a second slot
#
# ─── WHY ─────────────────────────────────────────────────────────────────────
#
# MEASURED 2026-09-04 02:35Z (ranger-base-uvzjk, from the post-WindowServer-
# crash sitting). This box carried FIVE concurrent `go test ./...` runs from
# five crew worktrees at once — three bare `go test -timeout 25m ./...`, two
# under scripts/test-times.sh — on 14% free memory with 1.47M pageouts. The
# 1-minute loadavg was 899 against a load-guard ceiling of 60.
# `go run ./cmd/checkorphans` was clean, so none of it was a leak: it was five
# legitimate suites, on eight cores, plus a GUI relaunch storm.
#
# Two costs, and the second is the one that matters. Each suite is already
# 2-3x its solo time when the others are running (one pane reported the root
# package at 551s). And the AGGREGATE trips the load guard against the whole
# FLEET for as long as the suites overlap — so the shop stops hiring at
# exactly the moment five seats are about to free.
#
# `go test ./...` already runs packages in parallel up to GOMAXPROCS, which is
# 8 here. One full suite is sized to the box. Two is deliberate
# over-subscription and the queue's price for not making everyone wait; five
# is the incident. POSSE_SUITE_SLOTS is 2 because ranger-base-uvzjk asked for
# 2 — it is the bead's number, not a measured optimum, and nothing here claims
# otherwise. No load run was made to find a better one: the standing operator
# ruling (2026-08-31) is no load testing on this box.
#
# WHAT IS SERIALIZED, AND WHAT DELIBERATELY IS NOT. A full unfiltered package
# tree — `./...`, `all`, anything ending `/...` — takes a slot. A `-run`
# filter or a named package does not: those are the runs a person does while
# thinking, they are seconds long, and queueing them behind a 20-minute suite
# would make the guard's own cure the thing that stops work.
#
# flock(2), and never a pidfile or a lock DIRECTORY. This is the launcher
# lock's argument (internal/posse/launchlock.go, ADR 0011 §1) applied to a
# second resource: an flock is held by the open file description, so the
# kernel drops it when the LAST holder of that description dies — crash,
# kill -9, closed pane alike. Release *is* process death, which leaves no
# staleness class to detect and nothing to reap (the LAST holder is not the
# wrapper, and that distinction is the paragraph two below). A `mkdir` lock or
# a pidfile would need a reaper, and a suite that dies under a full disk at
# 02:00 would wedge the queue for everybody until somebody noticed. The
# self-test's kill arm is that claim.
#
# WHY THE LOCK IS TAKEN BY A CHILD AND HELD BY THE SHELL. flock(1) is
# util-linux and absent on macOS, where this runs; python3 is already a
# dependency of this repo's gate path (scripts/bd-argv-gate.py). So the shell
# opens the slot file on fd 9 and a one-line python3 child flocks that
# INHERITED fd. The child exits; the lock does not. flock is per open file
# description, and the child's fd is a dup of the shell's — closing one of
# them releases nothing while the other is open. MEASURED here before this
# script was written: a second process was refused the slot while the parent
# shell slept with only fd 9 open, and took it the moment that shell exited.
#
# The fd is inherited by `go test` and its children too, on purpose. A run
# whose wrapper is killed but whose test tree keeps running is still spending
# the box, and the slot should stay spent until the last of it is gone.
#
# WHICH BUYS LIVENESS AND NOT IDENTITY, and the paragraphs above are about
# the first only (ranger-base-2fgu4). flock has no staleness class: the
# kernel revokes the lock when the last inheritor of the description dies,
# and no snapshot of that decays. But it answers "SOME process holds this",
# never "the process the stamp names holds this" — a separate question, for
# which the stamp is an ordinary pidfile and decays exactly as one. The two
# come apart the moment the wrapper dies before its children do:
#
#   MEASURED 2026-09-04 19:11-19:32Z. A `make test` wrapper (pid 58268) took
#   slot 2 and was SIGKILLed at 19:17Z. `--status` read "slot 2: HELD by pid
#   58268" at 19:26Z and again at 19:32Z. The HELD was true and kernel-owned
#   — a child that inherited fd 9 had outlived the wrapper. The pid printed
#   beside it had been dead for fifteen minutes, and nothing on the box said
#   so, so a slot nobody was using printed the same line as a live suite. On
#   a two-slot box that is half the capacity; and `ps` is denied inside the
#   seatbelt cage (the refusal exits 0), so a seat that noticed could not
#   name what was holding it either.
#
# So the two answers are printed apart now. The kernel's HELD stays the only
# evidence. The stamp's pid is then asked `kill -0`, and the ONE definite
# direction is reported: ESRCH, there is no such process. A pid that answers
# is NOT reported as alive — pid numbers are recycled, so answering proves
# something is alive and never that the holder is (_suite_lock_gone carries
# the rest of that argument). Nothing is reclaimed on the strength of it: the
# paragraph above still governs, the slot stays spent until the last
# inheritor is gone, and what changed is only that the operator can tell
# which of the two they are looking at.
#
# IT NEVER MAKES THE SUITE UNRUNNABLE. No python3 and no flock(1) means one
# warning line and an unserialized run. That is the opposite of the launcher
# lock's rule, and the asymmetry is the point: an unserialized launch
# destroyed session records (rangerhq-9nso), while an unserialized suite only
# makes a box slow. A slow box is recoverable; a crew that cannot run its
# tests is not.

# ---------------------------------------------------------------- primitives

suite_lock_dir() {
	printf '%s' "${POSSE_SUITE_LOCK_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/posse}"
}

_suite_lock_say() { printf 'suite-lock: %s\n' "$*" >&2; }

# _suite_lock_slots — how many slots this box hands out. POSSE_SUITE_SLOTS is
# read HERE and nowhere else, so every reader gets the same number and a bad
# value is caught once instead of five times. Sets _SUITE_LOCK_SLOTS rather
# than printing it: a command substitution runs in a subshell, where the
# said-it-already flag below could not survive to keep the warning off every
# poll.
#
# A value that is not a positive integer is a typo, and the header's promise —
# IT NEVER MAKES THE SUITE UNRUNNABLE — decides what to do about one: say so
# on one line and use the default. Both spellings were measured on this box
# (ranger-base-jhyiv, from ranger-base-han3i) and both were silent:
#
#   - non-numeric. `seq 1 two` prints nothing, so the sweep tries no slot at
#     all, the acquire queues forever against holders it cannot name, and the
#     waiting line ends at "held by " with nothing after it. `make test` with
#     a typo in this variable hung with no diagnosis.
#   - zero or negative. `seq` counts DOWN when the end is below the start, so
#     POSSE_SUITE_SLOTS=0 iterated i=1 then i=0 and handed out TWO slots
#     (measured: "slot 1 of 0", acquired), and -1 handed out three. A value
#     meant to shut the queue off widened it instead.
#
# Neither is refused, because refusing is the one thing this file may not do.
_suite_lock_slots() {
	local n=${POSSE_SUITE_SLOTS:-2}
	case $n in
	'' | *[!0-9]*) ;;
	*)
		if [ "$n" -ge 1 ]; then
			_SUITE_LOCK_SLOTS=$n
			return 0
		fi
		;;
	esac
	_SUITE_LOCK_SLOTS=2
	if [ -z "${_SUITE_LOCK_SLOTS_SAID:-}" ]; then
		_SUITE_LOCK_SLOTS_SAID=1
		_suite_lock_say "POSSE_SUITE_SLOTS='$n' is not a positive integer — using $_SUITE_LOCK_SLOTS"
	fi
	return 0
}

# _suite_lock_flock — take an exclusive non-blocking flock on the CALLER's
# fd 9. Returns 0 when the slot is ours, 1 when somebody else holds it, and
# 2 when there is no way to ask, which is a different answer and gets a
# different arm above.
_suite_lock_flock() {
	case ${_SUITE_LOCK_TOOL:-} in
	python3)
		python3 -c 'import fcntl,sys
try:
    fcntl.flock(9, fcntl.LOCK_EX | fcntl.LOCK_NB)
except OSError:
    sys.exit(1)
' 9<&9 2>/dev/null
		;;
	flock)
		flock -n 9 9<&9 2>/dev/null
		;;
	*) return 2 ;;
	esac
}

# The tool is resolved once, at source time, so a wrapper that acquires and a
# self-test that acquires a hundred times pay one lookup.
if command -v python3 >/dev/null 2>&1; then
	_SUITE_LOCK_TOOL=python3
elif command -v flock >/dev/null 2>&1; then
	_SUITE_LOCK_TOOL=flock
else
	_SUITE_LOCK_TOOL=
fi

# suite_lock_wanted <command...> — is this argv a FULL, unfiltered package
# tree? Reads the same argv the wrapper was handed, so it is the command that
# will actually run that decides, never a caller's claim about it.
#
# `go test` argv puts a flag's VALUE in its own word, so `-run TestFoo` and
# `-run=TestFoo` are both filters and both have to be seen (this is the same
# trap scripts/gotest.sh documents in run()).
suite_lock_wanted() {
	local a tree=0
	for a in "$@"; do
		case $a in
		-run | --run | -run=* | --run=* | -test.run | -test.run=*) return 1 ;;
		esac
	done
	for a in "$@"; do
		case $a in
		all | ... | */...) tree=1 ;;
		esac
	done
	[ "$tree" = 1 ]
}

# _suite_lock_gone <pid> — 0 when the kernel says there is NO process with
# this pid. One direction, on purpose (ranger-base-2fgu4). `kill -0` has three
# answers and only one of them is worth printing:
#
#   rc 0                      something has this pid. NOT reported, because
#                             pids are recycled: this is liveness of some
#                             process, never identity of the holder, and a
#                             line saying "alive" would be the pidfile
#                             mistake the header rejects.
#   rc 1, "No such process"   ESRCH. Nothing has this pid. Definite, and the
#                             only thing this function says yes to.
#   rc 1, anything else       EPERM, usually — something HAS the pid and it
#                             is not ours. Alive, so: not gone.
#
# The message is bash's own, and an unrecognised one falls through to "not
# gone", which prints exactly what this file printed before the check
# existed. `ps` would answer identity too (argv, start time, the way NOTES's
# husk check does it for dispatch-watch.pid) but it is denied inside the
# seatbelt cage AND its refusal exits 0, so a seat asking it would read the
# empty output as "no such process" — the one wrong answer this must not give.
_suite_lock_gone() {
	local pid=${1:-} err
	case $pid in '' | *[!0-9]*) return 1 ;; esac
	# 0 is "every process in my group" to kill(2), which is not a question
	# about a holder; a stamp can only say 0 if it was written by something
	# that is not this file.
	[ "$pid" -gt 0 ] || return 1
	# `err=$(...)` carries the command's own status, and the `&&` keeps a
	# wrapper under `set -e` alive through the ordinary failing case.
	err=$(kill -0 "$pid" 2>&1) && return 1
	case $err in *'No such process'*) return 0 ;; esac
	return 1
}

# _suite_lock_who_holds <slotfile> — the processes with this file OPEN, as
# "pid comm" pairs, or nothing. The identity answer the stamp cannot give:
# lsof lists the actual holders of the open file description, which after the
# wrapper dies is the only way to find out what the slot is still spent on.
# `ps` is the obvious tool and is denied inside the seatbelt cage; lsof was
# measured working there (ranger-base-2fgu4, 2026-09-04).
#
# EMPTY IS SILENCE, never "nobody". A denied lsof exits 0 with no output —
# the same shape as ps's refusal — so an empty answer prints no line at all
# and --status degrades to exactly what it said before this existed. The one
# thing it must never do is turn a refusal into "no process holds this".
#
# Capped, because a live suite has a test binary per package and the list is
# a hint for a human, not a census.
_suite_lock_who_holds() {
	command -v lsof >/dev/null 2>&1 || return 1
	local out
	out=$(lsof -F pc "$1" 2>/dev/null | awk '
/^p/ { p = substr($0, 2); next }
/^c/ { if (!(p in seen)) { seen[p] = 1
		if (n < 8) { out = out (n ? ", " : "") p " " substr($0, 2) }
		n++ } }
END { if (n > 8) { out = out ", and " (n - 8) " more" }
	print out }')
	[ -n "$out" ] || return 1
	printf '%s' "$out"
}

# _suite_lock_holders — one line per slot that somebody holds, worktree first.
# A hint for the waiting line and for --status, never evidence: the kernel
# holds the lock and these bytes are a courtesy, exactly as launchlock.go's
# stamp is. A slot whose stamp cannot be read still counts as held.
_suite_lock_holders() {
	local dir i f who pid gone
	dir=$(suite_lock_dir)
	_suite_lock_slots
	for i in $(seq 1 "$_SUITE_LOCK_SLOTS"); do
		f=$dir/suite-slot.$i.lock
		[ -f "$f" ] || continue
		who=$(sed -n 's/^worktree: //p' "$f" 2>/dev/null | head -1)
		[ -n "$who" ] || who="another worktree"
		pid=$(sed -n 's/^pid: //p' "$f" 2>/dev/null | head -1)
		# " gone" and nothing more. This function reads STAMPS and does
		# not ask the kernel which slots are held, so it cannot say what
		# is holding one — only --status, which has just been refused
		# the lock, has standing for that sentence.
		gone=''
		_suite_lock_gone "$pid" && gone=' gone'
		printf '%s (pid %s%s, since %s)\n' "$who" "$pid" "$gone" \
			"$(sed -n 's/^since: //p' "$f" 2>/dev/null | head -1)"
	done
}

# _suite_lock_stamp <slotfile> — who has it now. Written through a SECOND fd,
# which truncates the file without touching the flock on fd 9: the lock lives
# on the open file description, not on the bytes.
_suite_lock_stamp() {
	local slotfile=$1
	shift
	{
		printf 'pid: %s\n' "$$"
		printf 'since: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
		printf 'worktree: %s\n' "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
		printf 'cmd: %s\n' "$(printf '%s ' "$@" | tr -d '\n')"
	} >"$slotfile" 2>/dev/null || true
}

# _suite_lock_sweep — try every slot once, newest-numbered last. Sets
# _SUITE_LOCK_SLOT and leaves fd 9 open and locked on success; leaves fd 9
# CLOSED on failure, so a caller may sweep again without leaking a descriptor.
#
# Every status here is captured with `|| rc=$?` rather than read from `$?`
# after a bare call. A wrapper that sources this file may be running under
# `set -e` (scripts/gotest.sh is), and there a bare command returning 1 —
# which is this function's ordinary "all slots busy" answer — kills the
# wrapper instead of queueing it.
_suite_lock_sweep() {
	local dir i f rc
	dir=$(suite_lock_dir)
	_suite_lock_slots
	for i in $(seq 1 "$_SUITE_LOCK_SLOTS"); do
		f=$dir/suite-slot.$i.lock
		exec 9>>"$f" || continue
		rc=0
		_suite_lock_flock || rc=$?
		if [ "$rc" = 0 ]; then
			_SUITE_LOCK_SLOT=$i
			return 0
		fi
		exec 9>&-
		if [ "$rc" = 2 ]; then return 2; fi
	done
	return 1
}

# ------------------------------------------------------------------- the API

# suite_lock_acquire <command...> — take a slot if <command...> is a full
# suite, and block until one is free. Always returns 0: this queues runs, it
# does not refuse them, and a wrapper under `set -e` must not die here.
suite_lock_acquire() {
	_SUITE_LOCK_SLOT=

	suite_lock_wanted "$@" || return 0

	if [ "${POSSE_SUITE_LOCK:-1}" = 0 ]; then
		_suite_lock_say 'POSSE_SUITE_LOCK=0 — running unserialized against every other suite on this box'
		return 0
	fi

	# A nested wrapper runs inside the slot its parent already holds. Taking
	# a second would double-count one run against the box and, at
	# POSSE_SUITE_SLOTS=1, deadlock a wrapper against itself. Same shape as
	# underLaunchLock's `held` token: the holder is passed down rather than
	# looked up.
	if [ -n "${POSSE_SUITE_LOCK_HELD:-}" ]; then
		_suite_lock_say "already inside suite slot $POSSE_SUITE_LOCK_HELD — not taking a second"
		return 0
	fi

	if [ -z "$_SUITE_LOCK_TOOL" ]; then
		_suite_lock_say 'no python3 and no flock(1) — running unserialized (see the header)'
		return 0
	fi

	local dir waited=0 announced=0 start
	dir=$(suite_lock_dir)
	# Resolved once, in THIS shell, before any subshell: the one warning a
	# bad value earns is said here, and the flag it sets is inherited by
	# every `$( )` below, so a queued run does not repeat it every poll.
	_suite_lock_slots
	if ! mkdir -p "$dir" 2>/dev/null; then
		_suite_lock_say "cannot create $dir — running unserialized"
		return 0
	fi

	start=$(date +%s)
	local rc
	while :; do
		rc=0
		_suite_lock_sweep || rc=$?
		if [ "$rc" = 0 ]; then break; fi
		if [ "$rc" = 2 ]; then
			_suite_lock_say 'the lock tool stopped working — running unserialized'
			return 0
		fi
		if [ "$announced" = 0 ]; then
			announced=1
			# Named before the wait, never after it: a run that has
			# stopped for a reason must never look like a run that
			# has hung (launchlock.go says the same thing).
			_suite_lock_say "waiting for suite lock held by $(_suite_lock_holders | paste -sd '; ' - )"
			_suite_lock_say "$_SUITE_LOCK_SLOTS full suites already running on this box; this one is queued (POSSE_SUITE_SLOTS to change, POSSE_SUITE_LOCK=0 to opt out)"
		fi
		sleep "${POSSE_SUITE_LOCK_POLL:-5}"
		waited=$(( $(date +%s) - start ))
		# A heartbeat, because a 20-minute silent wait and a hang look
		# the same from the outside.
		if [ "$announced" = 1 ] && [ "$waited" -ge 300 ]; then
			announced=2
			_suite_lock_say "still queued after ${waited}s — holders: $(_suite_lock_holders | paste -sd '; ' - )"
		elif [ "$announced" = 2 ] && [ $(( waited % 300 )) -lt 2 ]; then
			_suite_lock_say "still queued after ${waited}s"
		fi
	done

	_suite_lock_stamp "$dir/suite-slot.$_SUITE_LOCK_SLOT.lock" "$@"
	export POSSE_SUITE_LOCK_HELD=$_SUITE_LOCK_SLOT
	waited=$(( $(date +%s) - start ))
	if [ "$waited" -ge 2 ]; then
		_suite_lock_say "slot $_SUITE_LOCK_SLOT of $_SUITE_LOCK_SLOTS acquired after ${waited}s"
	else
		_suite_lock_say "slot $_SUITE_LOCK_SLOT of $_SUITE_LOCK_SLOTS"
	fi
	return 0
}

# suite_lock_release — drop the slot by closing the fd, which is the same
# thing process death does. Idempotent, so a wrapper may both call it and
# rely on its own exit.
suite_lock_release() {
	[ -n "${_SUITE_LOCK_SLOT:-}" ] || return 0
	exec 9>&-
	_SUITE_LOCK_SLOT=
	unset POSSE_SUITE_LOCK_HELD
	return 0
}

suite_lock_status() {
	local dir held who
	dir=$(suite_lock_dir)
	_suite_lock_slots
	held=$(_suite_lock_holders)
	printf 'suite-lock: %s slot(s), %s\n' "$_SUITE_LOCK_SLOTS" "$dir"
	if [ -z "$held" ]; then
		printf '  no slot file has ever been written here\n'
		return 0
	fi
	# The stamps say who wrote them LAST, which is not who holds them now.
	# Ask the kernel for that, one slot at a time.
	local i f
	for i in $(seq 1 "$_SUITE_LOCK_SLOTS"); do
		f=$dir/suite-slot.$i.lock
		[ -f "$f" ] || { printf '  slot %s: free (never used)\n' "$i"; continue; }
		if ( exec 9>>"$f"; _suite_lock_flock ); then
			printf '  slot %s: free\n' "$i"
		else
			printf '  slot %s: HELD by %s\n' "$i" \
				"$(sed -n 's/^worktree: //p;s/^pid: /pid /p' "$f" 2>/dev/null | paste -sd ' ' -)"
			# Two answers, printed apart. HELD is the kernel's and is
			# evidence; the pid is the stamp's and decays. When the
			# stamp's pid is DEFINITELY gone, the lock is being held
			# by something that inherited fd 9 from it — by design
			# (the header), and indistinguishable from a live suite
			# until this line existed. It is a diagnosis, not a
			# reclamation: nothing here frees the slot.
			if _suite_lock_gone "$(sed -n 's/^pid: //p' "$f" 2>/dev/null | head -1)"; then
				printf '          that pid is GONE — a process it forked inherited the slot; it frees when the last of them exits\n'
				# And WHICH processes, when the box will say. This
				# is the half a seat could not get at all: the
				# stamp names the dead acquirer and `ps` is denied
				# in the cage, so before this the only way to find
				# the survivor was to go and look by hand.
				who=$(_suite_lock_who_holds "$f") &&
					printf '          still holding it: %s\n' "$who"
			fi
		fi
	done
}

# ---------------------------------------------------------------- self-test
#
# Every arm below drives the REAL functions in a scratch lock dir, with REAL
# concurrent processes, because the only thing worth knowing about a lock is
# what a second process sees. And every positive arm has a control that must
# come out the other way: "the third run did not start" is also what
# measuring nothing looks like.

_suite_lock_selftest() {
	# tmp is deliberately NOT local, for the reason scripts/test-times.sh
	# records at the same spot: the EXIT trap runs in global scope, where a
	# function-local name is unset and the `${tmp:?}` guard aborts on the
	# way out — after the arms have already reported.
	local fail=0 waiting_re= m3_log=
	tmp=$(mktemp -d "${TMPDIR:-/tmp}/posse-suite-lock.XXXXXX") || return 1
	trap 'rm -rf "${tmp:?}"' EXIT

	export POSSE_SUITE_LOCK_DIR="$tmp/locks"
	export POSSE_SUITE_LOCK_POLL=0.2
	export POSSE_SUITE_SLOTS=2
	unset POSSE_SUITE_LOCK_HELD

	# A holder is a whole separate process, which is the only kind of
	# witness an flock has. It acquires, records the slot it got, and then
	# holds it until its hold-file is removed — or until it is killed,
	# which is arm 4's point.
	cat >"$tmp/holder.sh" <<'HOLDER'
#!/usr/bin/env bash
# POSSE_SUITE_LOCK_HELD is deliberately NOT unset here: arm 8 sets it on
# purpose, and the rig clears it once, for every arm, before any holder runs.
set -u
lib=$1; marker=$2; hold=$3; shift 3
. "$lib"
suite_lock_acquire "$@" 2>"$marker.log"
printf 'slot:%s\n' "${_SUITE_LOCK_SLOT:-none}" >"$marker"
while [ -e "$hold" ]; do sleep 0.05; done
HOLDER
	chmod +x "$tmp/holder.sh"

	# The same, except it hands the slot back BEFORE it exits. A wrapper
	# does this the moment `go test` is done, so its own reporting tail
	# does not sit on a slot the run no longer needs.
	cat >"$tmp/releaser.sh" <<'RELEASER'
#!/usr/bin/env bash
set -u
lib=$1; marker=$2; hold=$3; shift 3
. "$lib"
suite_lock_acquire "$@" 2>"$marker.log"
printf 'slot:%s\n' "${_SUITE_LOCK_SLOT:-none}" >"$marker"
suite_lock_release
printf 'released\n' >"$marker.released"
while [ -e "$hold" ]; do sleep 0.05; done
RELEASER
	chmod +x "$tmp/releaser.sh"

	# The same again, except it forks a child that OUTLIVES it and then
	# waits to be killed. `go test`, the compilers it runs and the test
	# binaries gotest.sh execs are exactly this to a wrapper: they inherit
	# fd 9 and they do not die when it does. Arm 14 is that case.
	cat >"$tmp/orphaner.sh" <<'ORPHANER'
#!/usr/bin/env bash
set -u
lib=$1; marker=$2; hold=$3; shift 3
. "$lib"
suite_lock_acquire "$@" 2>"$marker.log"
# The child acquires nothing and opens nothing. All it has is the INHERITED
# fd 9, which is the entire point of the arm.
( while [ -e "$hold" ]; do sleep 0.05; done ) &
printf 'slot:%s\npid:%s\nchild:%s\n' "${_SUITE_LOCK_SLOT:-none}" "$$" "$!" >"$marker"
while :; do sleep 0.05; done
ORPHANER
	chmod +x "$tmp/orphaner.sh"

	# House form, the same one scripts/gotest.sh and scripts/test-times.sh
	# print and the same one the QA pin requires by arm name.
	ok() { printf 'ok    %s\n' "$1"; }
	bad() { printf 'FAIL  %s: %s\n' "$1" "$2"; fail=1; }

	# NOTHING THIS SELF-TEST DECIDES WITH MAY BE PARSED BY AN EXEC'D MATCHER
	# (ranger-base-t07yx). `make test` gates on verify-suite-lock, so a `sed`,
	# `awk`, `grep` or `head` that is signalled, that cannot be exec'd under
	# load, or that takes EPIPE past the 64 KB pipe buffer would red the suite
	# before a package runs, with a message about lock slots. ranger-base-7hx87
	# measured all three mechanisms on the sibling script. So the helpers below
	# read their files with bash's own redirection and `${...}`: no pipe, no
	# exec, nothing between the bytes and the verdict. The forks that ARE the
	# measurement — the holder processes, `suite_lock_status`, `lsof` — stay.

	# wait_file <path> <seconds> — did it appear in time? Whole seconds, which
	# is what all eighteen call sites pass; bash arithmetic, not awk's.
	wait_file() {
		local n=0 lim=$(( $2 * 10 ))
		while [ "$n" -lt "$lim" ]; do
			[ -e "$1" ] && return 0
			sleep 0.1
			n=$((n + 1))
		done
		return 1
	}
	# marker_field <file> <key> — prints the first `<key>:<value>` line's
	# value, nothing when there is none. The command substitution at the ~30
	# call sites stays; what is gone from inside it is the exec and the pipe.
	marker_field() {
		local line
		[ -r "$1" ] || return 1
		while IFS= read -r line || [ -n "$line" ]; do
			case $line in
			"$2":*)
				printf '%s\n' "${line#"$2":}"
				return 0
				;;
			esac
		done <"$1"
		return 1
	}
	slot_of() { marker_field "$1" slot; }
	# log_has <file> <literal> — does the file contain the literal anywhere?
	# `grep -q <pat> <file>` is the same defect as a piped grep and reads even
	# more innocently: a signalled or un-exec'd grep answers "no such line" for
	# a log that plainly carries it, and the arm calls the lock broken. Five
	# arms below used to ask that way (ranger-base-t07yx).
	log_has() {
		local c
		[ -r "$1" ] || return 1
		c=$(<"$1")
		case $c in *"$2"*) return 0 ;; esac
		return 1
	}

	local h1 h2 h3
	touch "$tmp/hold1" "$tmp/hold2" "$tmp/hold3"

	# ARM 1: two concurrent full suites both get a slot. This is the
	# property POSSE_SUITE_SLOTS names.
	"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m1" "$tmp/hold1" go test -timeout 25m ./... &
	h1=$!
	wait_file "$tmp/m1" 10 || bad 'slots: two concurrent full suites both run' 'first holder never acquired'
	"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m2" "$tmp/hold2" go test -timeout 25m ./... &
	h2=$!
	if wait_file "$tmp/m2" 10 && [ "$(slot_of "$tmp/m1")" != "$(slot_of "$tmp/m2")" ] &&
		[ "$(slot_of "$tmp/m1")" != none ] && [ "$(slot_of "$tmp/m2")" != none ]; then
		ok 'slots: two concurrent full suites both run'
	else
		bad 'slots: two concurrent full suites both run' \
			"got slots '$(slot_of "$tmp/m1")' and '$(slot_of "$tmp/m2")'"
	fi

	# ARM 2, the control for arm 1: the THIRD is queued, not run. Without
	# this arm, arm 1 is equally green over a lock that never locks.
	"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m3" "$tmp/hold3" go test -timeout 25m ./... &
	h3=$!
	sleep 2
	if [ -e "$tmp/m3" ]; then
		bad 'queue: a third full suite waits' "it started anyway, on slot $(slot_of "$tmp/m3")"
	else
		ok 'queue: a third full suite waits'
	fi

	# ARM 3: and the waiting run SAYS whose lock it is waiting on, by
	# worktree. A queue nobody can read is a hang.
	waiting_re='waiting for suite lock held by .*/'
	m3_log=$([ -r "$tmp/m3.log" ] && printf '%s' "$(<"$tmp/m3.log")")
	if [[ $m3_log =~ $waiting_re ]]; then
		ok 'queue: the waiting line names the holding worktree'
	else
		bad 'queue: the waiting line names the holding worktree' \
			"got: $(tr '\n' '|' <"$tmp/m3.log" 2>/dev/null)"
	fi

	# ARM 4: a filtered run is NOT queued behind them. The second control
	# for arm 1, and the one that keeps the cure from being worse than the
	# disease: `-run TestFoo` is what a person types while thinking.
	touch "$tmp/hold4"
	"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m4" "$tmp/hold4" go test -timeout 25m -run TestFoo ./... &
	if wait_file "$tmp/m4" 5 && [ "$(slot_of "$tmp/m4")" = none ]; then
		ok 'unlocked: a -run filtered suite takes no slot'
	else
		bad 'unlocked: a -run filtered suite takes no slot' \
			"marker '$(slot_of "$tmp/m4")' after 5s with both slots held"
	fi
	rm -f "$tmp/hold4"

	# ARM 5: nor is a single package.
	touch "$tmp/hold5"
	"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m5" "$tmp/hold5" go test -timeout 25m ./internal/posse &
	if wait_file "$tmp/m5" 5 && [ "$(slot_of "$tmp/m5")" = none ]; then
		ok 'unlocked: a single-package run takes no slot'
	else
		bad 'unlocked: a single-package run takes no slot' \
			"marker '$(slot_of "$tmp/m5")' after 5s with both slots held"
	fi
	rm -f "$tmp/hold5"

	# ARM 6: the queue drains. A holder finishes, and the waiter that arm 2
	# left queued takes the freed slot — which is also the proof that arm
	# 2's silence was a queue and not a deadlock.
	rm -f "$tmp/hold1"
	if wait_file "$tmp/m3" 15 && [ "$(slot_of "$tmp/m3")" = "$(slot_of "$tmp/m1")" ]; then
		ok 'queue: a freed slot is taken by the waiter'
	else
		bad 'queue: a freed slot is taken by the waiter' \
			"waiter got '$(slot_of "$tmp/m3")', freed slot was '$(slot_of "$tmp/m1")'"
	fi
	wait "$h1" 2>/dev/null

	# ARM 7: a slot held by a KILLED run is free. This is the whole reason
	# the lock is an flock and not a pidfile or a mkdir: `kill -9` runs no
	# cleanup, and a lock that needed one would wedge the queue for the
	# whole crew until somebody noticed.
	kill -9 "$h2" 2>/dev/null
	wait "$h2" 2>/dev/null
	touch "$tmp/hold7"
	"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m7" "$tmp/hold7" go test -timeout 25m ./... &
	local h7=$!
	if wait_file "$tmp/m7" 15 && [ "$(slot_of "$tmp/m7")" = "$(slot_of "$tmp/m2")" ]; then
		ok 'crash: the slot of a kill -9 run is reclaimed'
	else
		bad 'crash: the slot of a kill -9 run is reclaimed' \
			"new run got '$(slot_of "$tmp/m7")', killed run had '$(slot_of "$tmp/m2")'"
	fi
	rm -f "$tmp/hold3" "$tmp/hold7"
	wait "$h3" "$h7" 2>/dev/null

	# ARM 8: a wrapper running INSIDE another wrapper's slot does not take
	# a second one. Two slots for one run would double-count it against the
	# box, and at POSSE_SUITE_SLOTS=1 it is a wrapper deadlocked on itself.
	touch "$tmp/hold8"
	POSSE_SUITE_LOCK_HELD=1 "$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m8" "$tmp/hold8" go test -timeout 25m ./... &
	rm -f "$tmp/hold8"
	if wait_file "$tmp/m8" 5 && [ "$(slot_of "$tmp/m8")" = none ] &&
		log_has "$tmp/m8.log" 'already inside suite slot'; then
		ok 'nested: a run inside a held slot takes no second slot'
	else
		bad 'nested: a run inside a held slot takes no second slot' \
			"got '$(slot_of "$tmp/m8")'"
	fi

	# ARM 9: the opt-out works and is LOUD. A silent opt-out is how a
	# serialization guard becomes decorative.
	touch "$tmp/hold9"
	POSSE_SUITE_LOCK=0 "$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m9" "$tmp/hold9" go test -timeout 25m ./... &
	rm -f "$tmp/hold9"
	if wait_file "$tmp/m9" 5 && [ "$(slot_of "$tmp/m9")" = none ] &&
		log_has "$tmp/m9.log" unserialized; then
		ok 'opt-out: POSSE_SUITE_LOCK=0 runs unserialized and says so'
	else
		bad 'opt-out: POSSE_SUITE_LOCK=0 runs unserialized and says so' \
			"got '$(slot_of "$tmp/m9")', log: $(tr '\n' '|' <"$tmp/m9.log" 2>/dev/null)"
	fi

	# ARM 10: an explicit release frees the slot while the releasing
	# process is still ALIVE. Arm 6 only proves the kernel drops the lock
	# at exit, which is equally true of a suite_lock_release that does
	# nothing at all.
	touch "$tmp/hold10" "$tmp/hold11"
	"$tmp/releaser.sh" "$SUITE_LOCK_LIB" "$tmp/m10" "$tmp/hold10" go test -timeout 25m ./... &
	local h10=$!
	if wait_file "$tmp/m10.released" 10; then
		"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m11" "$tmp/hold11" go test -timeout 25m ./... &
		if wait_file "$tmp/m11" 10 && kill -0 "$h10" 2>/dev/null &&
			[ "$(slot_of "$tmp/m11")" = "$(slot_of "$tmp/m10")" ]; then
			ok 'release: a slot handed back is free before the process exits'
		else
			bad 'release: a slot handed back is free before the process exits' \
				"releaser held '$(slot_of "$tmp/m10")', next run got '$(slot_of "$tmp/m11")'"
		fi
	else
		bad 'release: a slot handed back is free before the process exits' 'the releaser never released'
	fi
	rm -f "$tmp/hold10" "$tmp/hold11"

	# ARM 11: a wrapper under `set -euo pipefail` survives a QUEUED
	# acquire. scripts/gotest.sh runs that way, and "all slots busy" is
	# this library's ordinary answer — a non-zero return from a bare call
	# there kills the wrapper instead of queueing it, which turns a busy
	# box into a suite that will not start and says nothing about why.
	touch "$tmp/hold12" "$tmp/hold13"
	"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m12" "$tmp/hold12" go test -timeout 25m ./... &
	wait_file "$tmp/m12" 10 || bad 'set -e: a queued acquire does not kill the wrapper' 'rig holder never acquired'
	"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m13" "$tmp/hold13" go test -timeout 25m ./... &
	wait_file "$tmp/m13" 10 || bad 'set -e: a queued acquire does not kill the wrapper' 'rig holder never acquired'
	cat >"$tmp/strict.sh" <<'STRICT'
#!/usr/bin/env bash
set -euo pipefail
. "$1"
suite_lock_acquire go test -timeout 25m ./...
printf 'reached the run
'
suite_lock_release
printf 'reached the end
'
STRICT
	# Two slots are held, so this one queues. Give it long enough to have
	# swept and gone round the loop, then free a slot and require it to
	# have got all the way through.
	( POSSE_SUITE_LOCK_POLL=0.2 bash "$tmp/strict.sh" "$SUITE_LOCK_LIB" >"$tmp/strict.out" 2>&1; echo $? >"$tmp/strict.rc" ) &
	sleep 1
	rm -f "$tmp/hold12"
	if wait_file "$tmp/strict.rc" 15 && [ "$(<"$tmp/strict.rc")" = 0 ] &&
		log_has "$tmp/strict.out" 'reached the end'; then
		ok 'set -e: a queued acquire does not kill the wrapper'
	else
		bad 'set -e: a queued acquire does not kill the wrapper' \
			"rc=$(cat "$tmp/strict.rc" 2>/dev/null), out: $(tr '\n' '|' <"$tmp/strict.out" 2>/dev/null)"
	fi
	rm -f "$tmp/hold13"

	wait 2>/dev/null

	# ARM 12: a POSSE_SUITE_SLOTS that is not a positive integer still runs
	# the suite, on the default, and NAMES the value it would not use. The
	# header's promise is that nothing in this file can make the suite
	# unrunnable, and a typo in that variable broke it three ways at once
	# (ranger-base-jhyiv): `seq 1 two` printed nothing, so the sweep tried no
	# slot, the acquire queued forever, and the line it queued behind read
	# "held by " and named nobody. This arm is the one that hangs — not
	# merely fails — on the unfixed script, which is why it is bounded.
	# The holder is killed by PID and never merely waited for: a script
	# without the check does not FAIL this arm, it hangs in it — the holder
	# is still in the acquire loop, so it never reaches the line that
	# watches its hold file, and a bare `wait` here would sit behind a
	# process that is never coming back. Its own pid, never a pattern (the
	# operator ruling of 2026-09-03).
	local n hp rc12=0
	for n in two 0 -1; do
		rm -f "$tmp/m14" "$tmp/m14.log"
		touch "$tmp/hold14"
		POSSE_SUITE_SLOTS=$n "$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m14" "$tmp/hold14" go test -timeout 25m ./... &
		hp=$!
		if ! wait_file "$tmp/m14" 8 || [ "$(slot_of "$tmp/m14")" = none ] ||
			! log_has "$tmp/m14.log" 'not a positive integer'; then
			rc12=1
			bad 'slots: a bad POSSE_SUITE_SLOTS runs on the default and says so' \
				"POSSE_SUITE_SLOTS=$n gave slot '$(slot_of "$tmp/m14")' after 8s, log: $(tr '\n' '|' <"$tmp/m14.log" 2>/dev/null)"
		fi
		rm -f "$tmp/hold14"
		kill "$hp" 2>/dev/null
		wait "$hp" 2>/dev/null
	done
	[ "$rc12" = 0 ] && ok 'slots: a bad POSSE_SUITE_SLOTS runs on the default and says so'

	# ARM 13, the control for arm 12: the fallback is the DEFAULT width and
	# not a wider one. `seq` on this box counts DOWN when the end is below
	# the start, so before the fix POSSE_SUITE_SLOTS=-1 iterated i=1, 0, -1
	# and handed out THREE slots — a value meant to shut the queue off
	# widened it, silently. Without this arm, arm 12 is equally green over a
	# fallback that hands out as many slots as anyone asks for.
	touch "$tmp/hold15" "$tmp/hold16" "$tmp/hold17"
	local h15 h16 h17
	POSSE_SUITE_SLOTS=-1 "$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m15" "$tmp/hold15" go test -timeout 25m ./... &
	h15=$!
	wait_file "$tmp/m15" 10 || bad 'slots: a negative POSSE_SUITE_SLOTS does not widen the queue' 'first holder never acquired'
	POSSE_SUITE_SLOTS=-1 "$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m16" "$tmp/hold16" go test -timeout 25m ./... &
	h16=$!
	wait_file "$tmp/m16" 10 || bad 'slots: a negative POSSE_SUITE_SLOTS does not widen the queue' 'second holder never acquired'
	POSSE_SUITE_SLOTS=-1 "$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m17" "$tmp/hold17" go test -timeout 25m ./... &
	h17=$!
	sleep 2
	if [ -e "$tmp/m17" ]; then
		bad 'slots: a negative POSSE_SUITE_SLOTS does not widen the queue' \
			"a third suite ran anyway, on slot $(slot_of "$tmp/m17")"
	elif [ "$(slot_of "$tmp/m15")" = "$(slot_of "$tmp/m16")" ] ||
		[ "$(slot_of "$tmp/m15")" = none ] || [ "$(slot_of "$tmp/m16")" = none ]; then
		bad 'slots: a negative POSSE_SUITE_SLOTS does not widen the queue' \
			"the two that did run got slots '$(slot_of "$tmp/m15")' and '$(slot_of "$tmp/m16")'"
	else
		ok 'slots: a negative POSSE_SUITE_SLOTS does not widen the queue'
	fi
	rm -f "$tmp/hold15" "$tmp/hold16" "$tmp/hold17"
	# Same reason as arm 12: the third one may still be queued behind a
	# queue that never opens, and it is not coming back on its own.
	kill "$h15" "$h16" "$h17" 2>/dev/null
	wait "$h15" "$h16" "$h17" 2>/dev/null

	# ARM 14: the wrapper dies and a child of it does not. Arm 7 kills a
	# holder that has no children and watches the slot come back; this is
	# the case it cannot reach, and it is the one that cost this box half
	# its suite capacity for fifteen minutes (ranger-base-2fgu4). fd 9 is
	# inherited, so SIGKILL on the wrapper drops the wrapper's COPY of the
	# open file description and not the description itself.
	#
	# Three things have to hold at once, and the first two pull opposite
	# ways — which is why one arm and not two:
	#
	#   the slot stays HELD, because the header says it should: a test tree
	#   that outlives its wrapper is still spending the box. Without this
	#   half the arm goes green over a "fix" that gave fd 9 close-on-exec
	#   and silently reversed that decision.
	#
	#   --status NAMES the case, because until it did, a slot nobody was
	#   using and a live suite printed the same line.
	#
	#   and it names the SURVIVOR, because `ps` is denied in the cage, so
	#   the pid list from lsof is the only handle a seat has on what is
	#   still spending the slot.
	#
	# Its own lock dir at POSSE_SUITE_SLOTS=1, or "still held" is not
	# provable: with two slots a second suite takes the other one and the
	# arm measures nothing.
	local od=$tmp/orphan-locks arm14='orphan: a dead wrapper leaves the slot held by its child, and says so'
	local opid ochild before after queued=0 drained=0 named=0 h18 h19 n held_re=
	mkdir -p "$od"
	orphan_status() { ( export POSSE_SUITE_LOCK_DIR="$od" POSSE_SUITE_SLOTS=1; suite_lock_status ); }
	touch "$tmp/hold18"
	POSSE_SUITE_LOCK_DIR="$od" POSSE_SUITE_SLOTS=1 \
		"$tmp/orphaner.sh" "$SUITE_LOCK_LIB" "$tmp/m18" "$tmp/hold18" go test -timeout 25m ./... &
	h18=$!
	if ! wait_file "$tmp/m18" 10 || [ "$(slot_of "$tmp/m18")" = none ]; then
		bad "$arm14" "the orphaning holder never acquired: $(tr '\n' '|' <"$tmp/m18.log" 2>/dev/null)"
		kill "$h18" 2>/dev/null
	else
		opid=$(marker_field "$tmp/m18" pid)
		# The control, and it must be taken BEFORE the kill: a --status
		# that printed the GONE line over every held slot would satisfy
		# the other half of this arm exactly as well as a working one.
		before=$(orphan_status)
		kill -9 "$h18" 2>/dev/null
		wait "$h18" 2>/dev/null
		# The kernel has to agree the acquirer is gone, or what follows
		# is a race and not a leak. Its own pid, never a pattern.
		n=0
		while kill -0 "$opid" 2>/dev/null && [ "$n" -lt 50 ]; do
			sleep 0.1
			n=$((n + 1))
		done
		after=$(orphan_status)

		# ...and a real queued suite still cannot have it. This is the
		# half that would catch a fix that freed the slot.
		touch "$tmp/hold19"
		POSSE_SUITE_LOCK_DIR="$od" POSSE_SUITE_SLOTS=1 \
			"$tmp/holder.sh" "$SUITE_LOCK_LIB" "$tmp/m19" "$tmp/hold19" go test -timeout 25m ./... &
		h19=$!
		sleep 2
		[ -e "$tmp/m19" ] || queued=1
		# Let the child go. The slot must drain to the waiter — which is
		# also the proof that the child was what held it, and that this
		# is a slot spent by a survivor and not a permanent wedge.
		rm -f "$tmp/hold18"
		if wait_file "$tmp/m19" 15 && [ "$(slot_of "$tmp/m19")" = "$(slot_of "$tmp/m18")" ]; then
			drained=1
		fi
		rm -f "$tmp/hold19"
		kill "$h19" 2>/dev/null
		wait "$h19" 2>/dev/null

		# ...and, where the box has an lsof, the SURVIVOR is named. That
		# is the half a seat cannot get any other way, so it is required
		# whenever it is available and said out loud when it is not — a
		# quietly skipped assertion is how a pin stops measuring.
		named=1
		if command -v lsof >/dev/null 2>&1; then
			ochild=$(marker_field "$tmp/m18" child)
			# Whole-pid, without \b: BSD grep and GNU grep do not
			# agree on it, and 5933 must not match 59330. Every entry
			# in the list is "<pid> <comm>", after ": " or ", ".
			held_re="still holding it: (.*, )?$ochild "
			[[ $after =~ $held_re ]] || named=0
		else
			printf 'note  %s: no lsof on this box, so the survivor is not named\n' "$arm14"
		fi

		if [ "$queued" = 1 ] && [ "$drained" = 1 ] && [ "$named" = 1 ] &&
			[[ $before == *'slot 1: HELD'* ]] &&
			[[ $before != *GONE* ]] &&
			[[ $after == *'slot 1: HELD'* ]] &&
			[[ $after == *"pid $opid"* ]] &&
			[[ $after == *'that pid is GONE'* ]]; then
			ok "$arm14"
		else
			bad "$arm14" \
				"queued=$queued drained=$drained named=$named; alive: $(printf '%s' "$before" | tr '\n' '|'); dead: $(printf '%s' "$after" | tr '\n' '|')"
		fi
	fi

	wait 2>/dev/null

	if [ "$fail" = 0 ]; then
		echo 'suite-lock --self-test: all arms ok'
	else
		echo 'suite-lock --self-test: FAILED' >&2
	fi
	return "$fail"
}

# Sourced or run? Sourced is the normal case and must define functions and
# stop; run is the self-test and the operator's `--status`.
SUITE_LOCK_LIB=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
	set -u
	case ${1:-} in
	--self-test) _suite_lock_selftest; exit $? ;;
	--status) suite_lock_status; exit 0 ;;
	-h | --help) sed -n '2,20p' "$SUITE_LOCK_LIB"; exit 0 ;;
	*)
		printf 'usage: source this file, or scripts/suite-lock.sh --self-test | --status\n' >&2
		exit 2
		;;
	esac
fi
