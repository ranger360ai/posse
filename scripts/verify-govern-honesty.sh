#!/usr/bin/env bash
# Verify that the governance surface stays HONEST when the thing it monitors
# is dead — the scratch-herdr half of rangerhq-mgvx, against the governance
# surface rangerhq-81y0 built.
#
# Usage: scripts/verify-govern-honesty.sh [path-to-posse]   (default: bin/posse-go, then $(command -v posse))
#        scripts/verify-govern-honesty.sh bin/posse-release  # a promote candidate
#
# The claim under test is the design's, quoted: "the view does not depend on
# the loop — `posse status` reads the stores directly and reports G7 itself,
# via the flock probe (release *is* death, no staleness class). What dies
# with the loop is *delivery* only." Three things have to be true at once for
# that to be more than a sentence:
#
#   1. With the loop alive the surface is CLEAR. Without this arm the rest
#      proves nothing: a check that says G7 whatever you do is not a probe,
#      it is a sticker (ranger-base-1fad, the control-arm rule).
#   2. With the loop killed -9 — no cleanup, no release, the kernel's job —
#      `posse status` says G7 and exits non-zero, and the cockpit says it in
#      the HEADER, which is the one line that does not scroll away.
#   3. The pulse, and only the pulse, dies with it: state/pulse.yaml stops
#      being written while status keeps answering off the live stores.
#
# and two arms that would each let a false G7 through unnoticed:
#
#   4. A stale dispatch-watch.pid naming a LIVE pid does not suppress G7.
#      That husk check is the mechanism the flock replaced (rangerhq-gir5);
#      a binary that still consulted it would pass arms 1-3 and fail here.
#   5. Autostart disarmed and the loop dead is NOT a condition — which is
#      also what makes arm 2's non-zero exit attributable to G7 rather than
#      to the rig.
#
# Safe direction, and only this direction (the rangerhq-snd wipe was the
# reverse — a real RHQ_HOME against a scratch socket): scratch RHQ_HOME on
# BOTH sides, the caller and the watch-loop process, plus a unique
# --session herdr server torn down on exit. The scratch config carries no
# `beads:` entries, so the bd half of the check has nothing to scan and G7
# is the only row that can move the verdict; `autostart_dry_run: true` is
# the seatbelt on the rig that is supposed to be isolated (rangerhq-v83).
set -uo pipefail
# extglob: the SGR strip in cockpit_frame is a `${...//}` pattern, not a
# `sed` fork (ranger-base-s8b4g).
shopt -s extglob

POSSE=${1:-}
if [ -z "$POSSE" ]; then
	here=$(cd -- "$(dirname -- "$0")/.." && pwd)
	if [ -x "$here/bin/posse-go" ]; then POSSE=$here/bin/posse-go; else POSSE=$(command -v posse || true); fi
fi
[ -x "$POSSE" ] || { echo "verify-govern-honesty: not executable: ${POSSE:-<none>}"; exit 2; }
POSSE=$(cd -- "$(dirname -- "$POSSE")" && pwd)/$(basename -- "$POSSE")

HERDR=${HERDR:-$(command -v herdr || true)}
[ -x "$HERDR" ] || { echo "verify-govern-honesty: herdr not on PATH"; exit 2; }

SESSION="govhonesty-$$"
FLEET_SOCK=${HOME}/.config/herdr/herdr.sock
SESS_SOCK=${HOME}/.config/herdr/sessions/${SESSION}/herdr.sock
RIG=$(mktemp -d "${TMPDIR:-/tmp}/posse-govern-honesty.XXXXXX")
HOMEDIR=$RIG/rhq
WORK=$RIG/cwd
mkdir -p "$HOMEDIR" "$WORK"

# Strip every herdr env inherited from a pane: the fleet socket is the one
# thing this script must never send a command to, and the one RHQ_HOME it
# must never read.
unset_herdr() {
	unset HERDR_ENV HERDR_SOCKET_PATH HERDR_CLIENT_SOCKET_PATH \
		HERDR_WORKSPACE_ID HERDR_PANE_ID HERDR_TAB_ID HERDR_SESSION \
		HERDR_BIN_PATH || true
}

# p runs the binary under test entirely inside the rig: scratch home, scratch
# socket, scratch cwd.
p() {
	unset_herdr
	(cd "$WORK" && env RHQ_HOME="$HOMEDIR" HERDR_SOCKET_PATH="$SESS_SOCK" "$POSSE" "$@")
}

hs() {
	unset_herdr
	"$HERDR" --session "$SESSION" "$@"
}

fail=0
check() { # check <name> <cond> <detail>
	if [ "$2" = 1 ]; then
		echo "PASS  $1"
	else
		echo "FAIL  $1  $3"
		fail=1
	fi
}

# ── the rig ──────────────────────────────────────────────────────────────────

# write_config <armed|disarmed> [pulse]
write_config() {
	{
		echo "# scratch rig — scripts/verify-govern-honesty.sh (rangerhq-mgvx)"
		echo "# \`beads:\` present and EMPTY: the bd half has no dir to scan, so"
		echo "# G2/G3/G9 stay silent and G7 is the only row that can move the verdict."
		echo "beads:"
		echo "autostart_dry_run: true"
		echo "autostart_max_beads: 1"
		[ "$1" = armed ] && echo "autostart_interval: 5m"
		[ "${2:-}" = pulse ] && { echo "pulse_interval: 2s"; echo "pulse_persona: nobody"; }
	} > "$HOMEDIR/config.yaml"
}

wait_server() {
	local n=0
	while [ "$n" -lt 100 ]; do
		[ -S "$SESS_SOCK" ] && unset_herdr && HERDR_SOCKET_PATH="$SESS_SOCK" "$HERDR" workspace list >/dev/null 2>&1 && return 0
		n=$((n + 1))
		sleep 0.1
	done
	echo "verify-govern-honesty: scratch herdr did not come up on $SESS_SOCK"
	return 1
}

LOOP_PID=
start_loop() {
	unset_herdr
	# `set -m` for this one launch, and `exec` inside the subshell: the job
	# becomes a process-GROUP leader whose leader pid IS the binary under
	# test, so the whole loop is reapable as `-$LOOP_PID` — by pid, never by
	# pattern. What stood here was a plain subshell reaped with `pkill -9 -P`,
	# and `pkill` is denied at every crew seat (operator ruling 2026-09-03,
	# ranger-base-jjx19), realized as a PATH shim — so the refusal landed
	# inside THIS SCRIPT, in a `2>/dev/null`, `kill -9 $LOOP_PID` took the
	# wrapper only, and the loop outlived its own reaper while the arm below
	# reported the flock stuck (ranger-base-q8hbz).
	set -m
	(cd "$WORK" && exec env RHQ_HOME="$HOMEDIR" HERDR_SOCKET_PATH="$SESS_SOCK" "$POSSE" dispatch --watch 5m > "$RIG/watch.log" 2>&1) &
	LOOP_PID=$!
	set +m
	# Off the job table. With monitor mode on, bash announces the -9 itself
	# ("Killed: 9   ( cd … )") on its own stderr, at whatever command happens
	# to be running when it notices — in the middle of the PASS lines.
	# Nothing waits on this job: kill_loop asks the kernel instead, and a
	# disowned child is still reaped by the shell (measured, no zombie).
	disown "$LOOP_PID" 2>/dev/null || true
	local n=0 ws=
	while [ "$n" -lt 100 ]; do
		ws=$(p dispatch --watch-status 2>/dev/null)
		case $ws in *'watch-loop: running'*) return 0 ;; esac
		n=$((n + 1))
		sleep 0.1
	done
	echo "verify-govern-honesty: watch loop did not take the lock"
	return 1
}

# group_alive <pid> — is anything of the job started as <pid> still there?
# Asked of the kernel, both ways: the leader itself, and anything it forked
# into its process group, because a child inherits the flock's fd and holds
# the lock after the leader is gone. `pgrep` lists, it does not kill, and the
# ruling leaves it running; `-g` takes a group id this script started, not an
# argv pattern that would match every other seat on the box.
group_alive() {
	kill -0 "$1" 2>/dev/null && return 0
	[ -n "$(pgrep -g "$1" 2>/dev/null)" ]
}

# kill -9, deliberately: the whole argument for the flock over a pidfile is
# that release is process death, cleanup path or none (rangerhq-gir5).
#
# Three outcomes, and only the middle one is this script's finding to report:
#   0  reaped, and the lock went with it        — the claim under test holds
#   1  reaped, CONFIRMED dead, lock STILL HELD  — the defect this arm exists
#                                                 for
#   2  not reaped                               — the apparatus failed, so
#      nothing was measured, and it must never be spelled as 1. A control that
#      cannot clean up has not measured the thing it claims to, and what this
#      printed as 1 was a 2 every time (ranger-base-q8hbz).
kill_loop() {
	[ -n "$LOOP_PID" ] || return 0
	local pid=$LOOP_PID n=0 ws=
	LOOP_PID=
	kill -9 -- "-$pid" >/dev/null 2>&1 || kill -9 "$pid" >/dev/null 2>&1 || true
	# Bounded, and asked of the KERNEL rather than of `wait`: `wait` on a job
	# the kill never reached blocks for that job's whole life, and reporting
	# that is the whole point of this function rather than sitting in it. Not
	# theory — the disabled-reaper mutant written to check the branch below
	# hung on exactly this until it was killed by hand. The window between
	# death and the shell's own reap is invisible to kill -0, pgrep and ps
	# alike, so polling first costs nothing.
	while [ "$n" -lt 100 ] && group_alive "$pid"; do
		n=$((n + 1))
		sleep 0.1
	done
	if group_alive "$pid"; then
		echo "verify-govern-honesty: the watch loop ($pid) survived kill -9 — the reaper failed, so NOTHING was measured"
		return 2
	fi
	n=0
	while [ "$n" -lt 100 ]; do
		ws=$(p dispatch --watch-status 2>/dev/null)
		case $ws in *'watch-loop: none'*) return 0 ;; esac
		n=$((n + 1))
		sleep 0.1
	done
	echo "verify-govern-honesty: the loop was CONFIRMED DEAD and the lock was still held"
	return 1
}

# The rig failing and the design failing are not the same result, so they do
# not share an exit: 2 is "nothing measured", 1 is a FAIL line naming what
# actually did not happen.
reap_loop() {
	kill_loop
	case $? in
	0) return 0 ;;
	1)
		check "lock-released-when-its-holder-dies" 0 "the flock outlived the process that held it"
		echo "verify-govern-honesty: FAIL"
		exit 1
		;;
	*) exit 2 ;;
	esac
}

STATUS_OUT=; STATUS_RC=
status_run() {
	STATUS_OUT=$(p status 2>&1)
	STATUS_RC=$?
}

# One cockpit frame off a non-tty (the displayOnly rendering), waited on
# until the asynchronous shop check has landed — the header says `gov …`
# until it does, and reading that as an answer would be the same
# unknown-as-clear mistake the surface exists to end.
COCKPIT_OUT=
cockpit_frame() {
	local out=$RIG/cockpit.$1.txt n=0 frame=
	: > "$out"
	unset_herdr
	# A group leader, for the reason start_loop is one: `kill -INT $cp` on a
	# subshell wrapper never reaches the cockpit, so this rig leaked a live
	# `posse cockpit` for every frame it took. MEASURED at ec0beaa: the run
	# that aborted at arm 2 had taken ONE frame and left exactly one
	# `posse-go cockpit` on ppid 1 beside the watch loop, still rendering
	# into a $RIG the EXIT trap had already removed (ranger-base-q8hbz).
	set -m
	(cd "$WORK" && exec env RHQ_HOME="$HOMEDIR" HERDR_SOCKET_PATH="$SESS_SOCK" "$POSSE" cockpit > "$out" 2>&1) &
	local cp=$!
	set +m
	disown "$cp" 2>/dev/null || true
	while [ "$n" -lt 200 ]; do
		frame=$(<"$out")
		case $frame in *'gov '[a-zA-Z0-9]*) break ;; esac
		n=$((n + 1))
		sleep 0.1
	done
	kill -INT -- "-$cp" >/dev/null 2>&1 || kill -INT "$cp" >/dev/null 2>&1 || true
	sleep 0.3
	kill -9 -- "-$cp" >/dev/null 2>&1 || kill -9 "$cp" >/dev/null 2>&1 || true
	local n2=0
	while [ "$n2" -lt 50 ] && group_alive "$cp"; do
		n2=$((n2 + 1))
		sleep 0.1
	done
	if group_alive "$cp"; then
		echo "verify-govern-honesty: the cockpit frame ($cp) survived kill -9 — the reaper failed, so NOTHING was measured"
		exit 2
	fi
	COCKPIT_OUT=$(<"$out")
	COCKPIT_OUT=${COCKPIT_OUT//$'\e'\[*([0-9;])[a-zA-Z]/}
	last_lines 40 "$COCKPIT_OUT"
	COCKPIT_OUT=$LAST_LINES
}

# The three helpers below decide arms, so none of them may fork a matcher
# (ranger-base-s8b4g, ranger-base-7hx87): a `grep`/`sed`/`tail` that is
# signalled, that cannot be exec'd under load, or that takes EPIPE reports the
# property false when the apparatus is what failed. Bash's own `case` and
# `${...}` cannot fail that way.
#
# has <text> <literal> — `grep -qF` without the fork. The pattern is quoted,
# so a `*` or `[` in <literal> is a byte like any other, as -F had it.
has() { case $1 in *"$2"*) return 0 ;; esac; return 1; }

# last_lines <n> <text> — the last <n> lines of <text> in $LAST_LINES, holding
# a ring of at most <n> in a string rather than an array: bash 3.2 (what
# /usr/bin/env bash is on this box) errors on an empty array expansion under
# `set -u`.
LAST_LINES=
last_lines() {
	local line buf= n=0
	while IFS= read -r line || [ -n "$line" ]; do
		buf=$buf$line$'\n'
		n=$((n + 1))
		if [ "$n" -gt "$1" ]; then
			buf=${buf#*$'\n'}
			n=$1
		fi
	done <<<"$2"
	LAST_LINES=${buf%$'\n'}
}

cleanup() {
	kill_loop >/dev/null 2>&1 || true
	unset_herdr
	"$HERDR" session stop "$SESSION" >/dev/null 2>&1 || true
	"$HERDR" session delete "$SESSION" >/dev/null 2>&1 || true
	rm -rf "$RIG"
}
trap cleanup EXIT

echo "verify-govern-honesty: $POSSE ($("$POSSE" version 2>/dev/null))"
echo "  session : $SESSION"
echo "  socket  : $SESS_SOCK"
echo "  home    : $HOMEDIR"

# The fleet's own socket is read once, and only to prove we are not it.
if [ "$SESS_SOCK" = "$FLEET_SOCK" ]; then
	echo "REFUSING: the scratch socket resolved to the fleet's own"
	exit 2
fi

# The scratch RHQ_HOME goes on the SERVER process too, not just the caller:
# a pane inherits the herdr server's environment, and a scratch server
# started without one is how eleven live metas were deleted (rangerhq-snd).
# This rig opens no pane; the seatbelt is worn anyway, because this is the
# rig that is supposed to be isolated.
unset_herdr
env RHQ_HOME="$HOMEDIR" "$HERDR" --session "$SESSION" server >/dev/null 2>&1 &
wait_server || exit 2

# ── 1 · the control arm: the loop is alive, the surface is clear ─────────────
write_config armed
start_loop || exit 2
status_run
check "alive-status-exit-zero" "$([ "$STATUS_RC" = 0 ] && echo 1 || echo 0)" "rc=$STATUS_RC out=$STATUS_OUT"
check "alive-status-clear" "$(has "$STATUS_OUT" 'nothing needs a human' && echo 1 || echo 0)" "$STATUS_OUT"
check "alive-no-G7" "$(has "$STATUS_OUT" 'G7' && echo 0 || echo 1)" "$STATUS_OUT"
cockpit_frame alive
check "alive-cockpit-header-clear" "$(has "$COCKPIT_OUT" 'gov clear' && echo 1 || echo 0)" "$COCKPIT_OUT"
check "alive-cockpit-no-loop-dead" "$(has "$COCKPIT_OUT" 'loop dead' && echo 0 || echo 1)" "$COCKPIT_OUT"

# ── 2 · the loop is killed -9: G7, non-zero, and the header says it ──────────
reap_loop
status_run
check "dead-status-exit-nonzero" "$([ "$STATUS_RC" != 0 ] && echo 1 || echo 0)" "rc=$STATUS_RC"
check "dead-status-G7-urgent" "$(has "$STATUS_OUT" 'URGENT' && has "$STATUS_OUT" 'G7' && echo 1 || echo 0)" "$STATUS_OUT"
check "dead-status-names-delivery" "$(has "$STATUS_OUT" 'nothing is being delivered' && echo 1 || echo 0)" "$STATUS_OUT"
cockpit_frame dead
check "dead-cockpit-header-says-loop-dead" "$(has "$COCKPIT_OUT" 'loop dead' && echo 1 || echo 0)" "$COCKPIT_OUT"
check "dead-cockpit-block-has-G7" "$(has "$COCKPIT_OUT" 'GOVERNANCE' && has "$COCKPIT_OUT" 'G7' && echo 1 || echo 0)" "$COCKPIT_OUT"

# ── 3 · a stale pidfile naming a LIVE pid does not suppress G7 ───────────────
# The husk check the flock retired: pid + argv, reconstructed from a file
# whose truth decays. $$ is alive and its argv is this script, so a binary
# reading the pidfile for liveness reads a running loop here, and one
# matching on argv reads none — the two ways to be wrong, both refused by
# asking the kernel instead.
printf 'pid: %s\nstarted: %s\ncmd: %s dispatch --watch 5m\n' \
	"$$" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$POSSE" > "$HOMEDIR/state/dispatch-watch.pid"
status_run
check "stale-pidfile-still-G7" "$(has "$STATUS_OUT" 'G7' && echo 1 || echo 0)" "$STATUS_OUT"
check "stale-pidfile-exit-nonzero" "$([ "$STATUS_RC" != 0 ] && echo 1 || echo 0)" "rc=$STATUS_RC"

# ── 4 · autostart disarmed: a dead loop is not a condition ───────────────────
# The row is "watch loop dead WHILE AUTOSTART ARMED". Disarmed, nobody
# promised a loop, so silence is the honest answer — and this is what makes
# arm 2's non-zero exit G7's doing rather than the rig's.
write_config disarmed
status_run
check "disarmed-no-G7" "$(has "$STATUS_OUT" 'G7' && echo 0 || echo 1)" "$STATUS_OUT"
check "disarmed-exit-zero" "$([ "$STATUS_RC" = 0 ] && echo 1 || echo 0)" "rc=$STATUS_RC out=$STATUS_OUT"

# ── 5 · what dies with the loop is delivery, and only delivery ───────────────
# The pulse is the one thing the design says goes with it. Its tick is the
# only writer of state/pulse.yaml, so a frozen file is delivery stopping;
# `posse status` answering the same second is the view not stopping with it.
#
# FROZEN IS MEASURED BY mtime, NOT BY CONTENT (ranger-base-thm0j). Until ADR
# 0027's 2026-09-05 simplification the record carried `at:`, so its bytes
# changed on every tick and a content comparison was a live-vs-dead
# discriminator by accident. The record is two delivery fields now and its
# bytes are IDENTICAL tick after tick on a shop with nothing to deliver — a
# content check would pass with the loop still running, which is the arm
# going quiet rather than the arm holding. `find -newer` is the POSIX
# spelling and works on both platforms `make test-linux` covers; `stat`'s
# format flags do not.
#
# Both directions, because "the file did not move" is worth nothing without
# the arm that shows it moves at all: MOVES while alive, then STOPS.
write_config armed pulse
rm -f "$HOMEDIR/state/pulse.yaml"
start_loop || exit 2
n=0
while [ "$n" -lt 100 ] && [ ! -f "$HOMEDIR/state/pulse.yaml" ]; do n=$((n + 1)); sleep 0.1; done
check "pulse-writes-while-alive" "$([ -f "$HOMEDIR/state/pulse.yaml" ] && echo 1 || echo 0)" "no state/pulse.yaml after 10s at pulse_interval 2s"
moved() { [ -n "$(find "$HOMEDIR/state/pulse.yaml" -newer "$1" 2>/dev/null)" ] && echo 1 || echo 0; }
touch "$RIG/pulse-mark-alive"
sleep 5 # two-and-a-half pulse intervals
check "pulse-keeps-writing-while-alive" "$(moved "$RIG/pulse-mark-alive")" "state/pulse.yaml did not move in 5s at pulse_interval 2s — the control arm for the check below"
reap_loop
touch "$RIG/pulse-mark-dead"
sleep 5 # two-and-a-half pulse intervals
check "pulse-stops-with-the-loop" "$([ "$(moved "$RIG/pulse-mark-dead")" = 0 ] && echo 1 || echo 0)" "state/pulse.yaml still moving after the loop died"
status_run
check "view-outlives-the-loop" "$(has "$STATUS_OUT" 'G7' && echo 1 || echo 0)" "$STATUS_OUT"

if [ "$fail" = 0 ]; then
	echo "verify-govern-honesty: PASS"
else
	echo "verify-govern-honesty: FAIL"
fi
exit "$fail"
