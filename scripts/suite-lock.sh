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
#   POSSE_SUITE_SLOTS=2        full suites allowed to run at once
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
# kernel drops it when the holder dies — crash, kill -9, closed pane alike.
# Release *is* process death, which leaves no staleness class to detect and
# nothing to reap. A `mkdir` lock or a pidfile would need a reaper, and a
# suite that dies under a full disk at 02:00 would wedge the queue for
# everybody until somebody noticed. The self-test's kill arm is that claim.
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

# _suite_lock_holders — one line per slot that somebody holds, worktree first.
# A hint for the waiting line and for --status, never evidence: the kernel
# holds the lock and these bytes are a courtesy, exactly as launchlock.go's
# stamp is. A slot whose stamp cannot be read still counts as held.
_suite_lock_holders() {
	local dir i f who
	dir=$(suite_lock_dir)
	for i in $(seq 1 "${POSSE_SUITE_SLOTS:-2}"); do
		f=$dir/suite-slot.$i.lock
		[ -f "$f" ] || continue
		who=$(sed -n 's/^worktree: //p' "$f" 2>/dev/null | head -1)
		[ -n "$who" ] || who="another worktree"
		printf '%s (pid %s, since %s)\n' "$who" \
			"$(sed -n 's/^pid: //p' "$f" 2>/dev/null | head -1)" \
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
	for i in $(seq 1 "${POSSE_SUITE_SLOTS:-2}"); do
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
			_suite_lock_say "${POSSE_SUITE_SLOTS:-2} full suites already running on this box; this one is queued (POSSE_SUITE_SLOTS to change, POSSE_SUITE_LOCK=0 to opt out)"
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
		_suite_lock_say "slot $_SUITE_LOCK_SLOT of ${POSSE_SUITE_SLOTS:-2} acquired after ${waited}s"
	else
		_suite_lock_say "slot $_SUITE_LOCK_SLOT of ${POSSE_SUITE_SLOTS:-2}"
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
	local dir held
	dir=$(suite_lock_dir)
	held=$(_suite_lock_holders)
	printf 'suite-lock: %s slot(s), %s\n' "${POSSE_SUITE_SLOTS:-2}" "$dir"
	if [ -z "$held" ]; then
		printf '  no slot file has ever been written here\n'
		return 0
	fi
	# The stamps say who wrote them LAST, which is not who holds them now.
	# Ask the kernel for that, one slot at a time.
	local i f
	for i in $(seq 1 "${POSSE_SUITE_SLOTS:-2}"); do
		f=$dir/suite-slot.$i.lock
		[ -f "$f" ] || { printf '  slot %s: free (never used)\n' "$i"; continue; }
		if ( exec 9>>"$f"; _suite_lock_flock ); then
			printf '  slot %s: free\n' "$i"
		else
			printf '  slot %s: HELD by %s\n' "$i" \
				"$(sed -n 's/^worktree: //p;s/^pid: /pid /p' "$f" 2>/dev/null | paste -sd ' ' -)"
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
	local fail=0
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

	# House form, the same one scripts/gotest.sh and scripts/test-times.sh
	# print and the same one the QA pin requires by arm name.
	ok() { printf 'ok    %s\n' "$1"; }
	bad() { printf 'FAIL  %s: %s\n' "$1" "$2"; fail=1; }

	# wait_file <path> <seconds> — did it appear in time?
	wait_file() {
		local n=0 lim
		lim=$(awk "BEGIN{print int($2/0.1)}")
		while [ "$n" -lt "$lim" ]; do
			[ -e "$1" ] && return 0
			sleep 0.1
			n=$((n + 1))
		done
		return 1
	}
	slot_of() { sed -n 's/^slot://p' "$1" 2>/dev/null | head -1; }

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
	if grep -q 'waiting for suite lock held by .*/' "$tmp/m3.log" 2>/dev/null; then
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
		grep -q 'already inside suite slot' "$tmp/m8.log" 2>/dev/null; then
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
		grep -q 'unserialized' "$tmp/m9.log" 2>/dev/null; then
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
	if wait_file "$tmp/strict.rc" 15 && [ "$(cat "$tmp/strict.rc")" = 0 ] &&
		grep -q 'reached the end' "$tmp/strict.out"; then
		ok 'set -e: a queued acquire does not kill the wrapper'
	else
		bad 'set -e: a queued acquire does not kill the wrapper' \
			"rc=$(cat "$tmp/strict.rc" 2>/dev/null), out: $(tr '\n' '|' <"$tmp/strict.out" 2>/dev/null)"
	fi
	rm -f "$tmp/hold13"

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
