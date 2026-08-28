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
	(cd "$WORK" && env RHQ_HOME="$HOMEDIR" HERDR_SOCKET_PATH="$SESS_SOCK" "$POSSE" dispatch --watch 5m > "$RIG/watch.log" 2>&1) &
	LOOP_PID=$!
	local n=0
	while [ "$n" -lt 100 ]; do
		p dispatch --watch-status 2>/dev/null | grep -q 'watch-loop: running' && return 0
		n=$((n + 1))
		sleep 0.1
	done
	echo "verify-govern-honesty: watch loop did not take the lock"
	return 1
}

# kill -9, deliberately: the whole argument for the flock over a pidfile is
# that release is process death, cleanup path or none (rangerhq-gir5).
kill_loop() {
	[ -n "$LOOP_PID" ] || return 0
	pkill -9 -P "$LOOP_PID" >/dev/null 2>&1 || true
	kill -9 "$LOOP_PID" >/dev/null 2>&1 || true
	wait "$LOOP_PID" 2>/dev/null || true
	LOOP_PID=
	local n=0
	while [ "$n" -lt 100 ]; do
		p dispatch --watch-status 2>/dev/null | grep -q 'watch-loop: none' && return 0
		n=$((n + 1))
		sleep 0.1
	done
	echo "verify-govern-honesty: the lock was still held after kill -9"
	return 1
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
	local out=$RIG/cockpit.$1.txt n=0
	: > "$out"
	unset_herdr
	(cd "$WORK" && env RHQ_HOME="$HOMEDIR" HERDR_SOCKET_PATH="$SESS_SOCK" "$POSSE" cockpit > "$out" 2>&1) &
	local cp=$!
	while [ "$n" -lt 200 ]; do
		if LC_ALL=C grep -q 'gov [a-zA-Z0-9]' "$out" 2>/dev/null; then break; fi
		n=$((n + 1))
		sleep 0.1
	done
	kill -INT "$cp" >/dev/null 2>&1 || true
	sleep 0.3
	kill -9 "$cp" >/dev/null 2>&1 || true
	wait "$cp" 2>/dev/null || true
	COCKPIT_OUT=$(LC_ALL=C sed 's/\x1b\[[0-9;]*[a-zA-Z]//g' "$out" | tail -40)
}

has() { printf '%s' "$1" | LC_ALL=C grep -qF -- "$2"; }

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
kill_loop || exit 2
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
# only writer of state/pulse.yaml, so a frozen mtime is delivery stopping;
# `posse status` answering the same second is the view not stopping with it.
write_config armed pulse
rm -f "$HOMEDIR/state/pulse.yaml"
start_loop || exit 2
n=0
while [ "$n" -lt 100 ] && [ ! -f "$HOMEDIR/state/pulse.yaml" ]; do n=$((n + 1)); sleep 0.1; done
check "pulse-writes-while-alive" "$([ -f "$HOMEDIR/state/pulse.yaml" ] && echo 1 || echo 0)" "no state/pulse.yaml after 10s at pulse_interval 2s"
before=$(cat "$HOMEDIR/state/pulse.yaml" 2>/dev/null)
kill_loop || exit 2
sleep 5 # two-and-a-half pulse intervals
after=$(cat "$HOMEDIR/state/pulse.yaml" 2>/dev/null)
check "pulse-stops-with-the-loop" "$([ "$before" = "$after" ] && echo 1 || echo 0)" "state/pulse.yaml still moving after the loop died"
status_run
check "view-outlives-the-loop" "$(has "$STATUS_OUT" 'G7' && echo 1 || echo 0)" "$STATUS_OUT"

if [ "$fail" = 0 ]; then
	echo "verify-govern-honesty: PASS"
else
	echo "verify-govern-honesty: FAIL"
fi
exit "$fail"
