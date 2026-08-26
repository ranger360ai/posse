#!/usr/bin/env bash
# Verify herdr's workspace-id allocator against a SCRATCH named session.
# Never aims at the fleet default server (rangerhq-6bg7 / rangerhq-6bbz).
#
# Usage: scripts/verify-id-recycle.sh
#        HERDR=/path/to/herdr scripts/verify-id-recycle.sh
#
# herdr's allocator is max(live id)+1, recomputed from the live set at every
# server process start — a restart and a live-handoff both. Nothing on disk
# carries a high-water mark. Within one process, closed ids are not reused.
# So workspace_not_found is a snapshot, not a stable fact: an id that is
# dead today can answer alive tomorrow for a stranger (measured).
#
# Safe direction: a unique --session name under ~/.config/herdr/sessions/,
# torn down even on failure. HERDR_* inherited from a fleet pane is stripped
# so we cannot talk to the default socket. A handoff fires only after
# `herdr status` names this session's socket.
set -euo pipefail

HERDR=${HERDR:-$(command -v herdr)}
[ -x "$HERDR" ] || { echo "verify-id-recycle: not executable: ${HERDR:-<none>}"; exit 2; }

SESSION="idrecycle-$$"
FLEET_SOCK=${HOME}/.config/herdr/herdr.sock
SESS_SOCK=${HOME}/.config/herdr/sessions/${SESSION}/herdr.sock
CWD=$(mktemp -d "${TMPDIR:-/tmp}/posse-id-recycle.XXXXXX")

# Strip every herdr env inherited from a pane. The fleet socket is the one
# thing this script must never send a command to.
unset_herdr() {
	unset HERDR_ENV HERDR_SOCKET_PATH HERDR_CLIENT_SOCKET_PATH \
		HERDR_WORKSPACE_ID HERDR_PANE_ID HERDR_TAB_ID HERDR_SESSION \
		HERDR_BIN_PATH || true
}

h() {
	unset_herdr
	env HERDR_SESSION="$SESSION" HERDR_SOCKET_PATH="$SESS_SOCK" "$HERDR" "$@"
}

# Session-manager commands (list/stop/delete) still need a clean env so a
# pane's HERDR_SOCKET_PATH cannot retarget them at the fleet server.
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

json_ids() {
	python3 -c 'import json,sys; print(" ".join(w["workspace_id"] for w in json.load(sys.stdin)["result"]["workspaces"]))'
}

json_create_id() {
	python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["workspace"]["workspace_id"])'
}

json_status_sock() {
	python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("server",{}).get("socket") or "")'
}

json_status_session() {
	python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("server",{}).get("session") or "")'
}

json_status_running() {
	python3 -c 'import json,sys; d=json.load(sys.stdin); print("1" if d.get("server",{}).get("running") else "0")'
}

json_label() {
	python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("result") or {}).get("workspace",{}).get("label") or "")'
}

sock_ino() {
	stat -f '%i' "$1" 2>/dev/null || stat -c '%i' "$1"
}

create() {
	h workspace create --cwd "$CWD" --label "$1" --no-focus | json_create_id
}

wait_running() {
	local n=0
	while [ "$n" -lt 50 ]; do
		if [ -S "$SESS_SOCK" ]; then
			local st running sock sess
			st=$(hs status --json 2>/dev/null || true)
			running=$(printf '%s' "$st" | json_status_running 2>/dev/null || echo 0)
			sock=$(printf '%s' "$st" | json_status_sock 2>/dev/null || true)
			sess=$(printf '%s' "$st" | json_status_session 2>/dev/null || true)
			if [ "$running" = 1 ] && [ "$sock" = "$SESS_SOCK" ] && [ "$sess" = "$SESSION" ]; then
				return 0
			fi
		fi
		n=$((n + 1))
		sleep 0.1
	done
	echo "verify-id-recycle: named session did not come up on $SESS_SOCK"
	return 1
}

stop_session() {
	unset_herdr
	"$HERDR" session stop "$SESSION" >/dev/null 2>&1 || true
	local n=0
	while [ "$n" -lt 50 ]; do
		local st running
		st=$(hs status --json 2>/dev/null || true)
		running=$(printf '%s' "$st" | json_status_running 2>/dev/null || echo 0)
		if [ "$running" != 1 ]; then
			return 0
		fi
		n=$((n + 1))
		sleep 0.1
	done
}

delete_session() {
	unset_herdr
	"$HERDR" session delete "$SESSION" >/dev/null 2>&1 || true
}

cleanup() {
	stop_session
	delete_session
	rm -rf "$CWD"
}
trap cleanup EXIT

echo "verify-id-recycle: $HERDR ($("$HERDR" --version 2>/dev/null | head -1))"
echo "  session : $SESSION"
echo "  socket  : $SESS_SOCK"
echo "  scratch : $CWD"
echo

# Fleet snapshot. Aim at the default socket EXPLICITLY so a pane's
# HERDR_SOCKET_PATH cannot retarget the read, and so we never confuse it
# with the scratch session we are about to start.
fleet_ids_before=$(
	unset_herdr
	HERDR_SOCKET_PATH="$FLEET_SOCK" "$HERDR" workspace list | json_ids
)
fleet_pid_before=$(lsof -t "$FLEET_SOCK" 2>/dev/null | head -1 || true)
fleet_ino_before=$(sock_ino "$FLEET_SOCK")
echo "  fleet   : pid=$fleet_pid_before ino=$fleet_ino_before ids=[$fleet_ids_before]"
echo

# --- start scratch server ---
unset_herdr
"$HERDR" --session "$SESSION" server >/dev/null 2>&1 &
wait_running

st=$(hs status --json)
check "aimed-at-named-socket" \
	"$([ "$(printf '%s' "$st" | json_status_sock)" = "$SESS_SOCK" ] && [ "$(printf '%s' "$st" | json_status_session)" = "$SESSION" ] && echo 1 || echo 0)" \
	"$(printf '%s' "$st" | json_status_sock)"

ids=$(h workspace list | json_ids)
check "fresh-server-empty" "$([ -z "$ids" ] && echo 1 || echo 0)" "ids=[$ids]"

# --- gilfoyle table ---
w1=$(create a1); w2=$(create a2); w3=$(create a3); w4=$(create a4)
check "fresh-create-x4" "$([ "$w1 $w2 $w3 $w4" = "w1 w2 w3 w4" ] && echo 1 || echo 0)" "got=$w1 $w2 $w3 $w4"

h workspace close w3 >/dev/null
h workspace close w4 >/dev/null
# workspace_not_found is how death looks *right now*.
if h workspace get w3 >/dev/null 2>"$CWD/get-w3.err"; then
	check "w3-not-found-after-close" 0 "get succeeded"
else
	if grep -q workspace_not_found "$CWD/get-w3.err"; then
		check "w3-not-found-after-close" 1 ""
	else
		check "w3-not-found-after-close" 0 "$(cat "$CWD/get-w3.err")"
	fi
fi
w5=$(create a5); w6=$(create a6)
check "same-process-no-reuse" "$([ "$w5 $w6" = "w5 w6" ] && echo 1 || echo 0)" "got=$w5 $w6"

# Middle hole is NOT filled in the same process (high-water, not min-unused).
h workspace close w2 >/dev/null
hole=$(create hole-same)
check "same-process-does-not-fill-hole" "$([ "$hole" = "w7" ] && echo 1 || echo 0)" "got=$hole want=w7 not=w2"

ino_before=$(sock_ino "$SESS_SOCK")
stop_session
unset_herdr
"$HERDR" --session "$SESSION" server >/dev/null 2>&1 &
wait_running
ino_after=$(sock_ino "$SESS_SOCK")
check "restart-recreates-socket-inode" "$([ "$ino_after" != "$ino_before" ] && echo 1 || echo 0)" "before=$ino_before after=$ino_after"
check "restart-socket-path-unchanged" "$([ -S "$SESS_SOCK" ] && echo 1 || echo 0)" "$SESS_SOCK"

live=$(h workspace list | json_ids)
check "live-ids-survive-restart" "$([ "$live" = "w1 w5 w6 w7" ] && echo 1 || echo 0)" "got=[$live]"

w8=$(create r1); w9=$(create r2)
check "restart-resumes-past-live-max" "$([ "$w8 $w9" = "w8 w9" ] && echo 1 || echo 0)" "got=$w8 $w9"

h workspace close w8 >/dev/null
h workspace close w9 >/dev/null
stop_session
unset_herdr
"$HERDR" --session "$SESSION" server >/dev/null 2>&1 &
wait_running
recycled=$(create recycled-restart)
check "restart-recycles-id-above-live-high-water" "$([ "$recycled" = "w8" ] && echo 1 || echo 0)" "got=$recycled want=w8"
lab=$(h workspace get w8 | json_label)
check "recycled-id-is-stranger-label" "$([ "$lab" = "recycled-restart" ] && echo 1 || echo 0)" "label=$lab"

# Close everything. Same process must NOT reset (in-memory high-water).
for id in $(h workspace list | json_ids); do
	h workspace close "$id" >/dev/null
done
same=$(create same-proc-empty)
check "same-process-close-all-does-not-reset" "$([ "$same" != "w1" ] && echo 1 || echo 0)" "got=$same"
h workspace close "$same" >/dev/null
stop_session
unset_herdr
"$HERDR" --session "$SESSION" server >/dev/null 2>&1 &
wait_running
reset=$(create reset-w1)
check "close-everything-restart-resets-to-w1" "$([ "$reset" = "w1" ] && echo 1 || echo 0)" "got=$reset"

# Control + handoff: w1 w2 w3, close w3, live-handoff, create -> w3.
w2=$(create c2); w3=$(create c3)
check "handoff-setup" "$([ "$reset $w2 $w3" = "w1 w2 w3" ] && echo 1 || echo 0)" "got=$reset $w2 $w3"
h workspace close w3 >/dev/null
st=$(hs status --json)
aim_sock=$(printf '%s' "$st" | json_status_sock)
aim_sess=$(printf '%s' "$st" | json_status_session)
if [ "$aim_sock" != "$SESS_SOCK" ] || [ "$aim_sess" != "$SESSION" ] || [ "$aim_sock" = "$FLEET_SOCK" ]; then
	echo "REFUSING handoff: aim is $aim_sess $aim_sock (want $SESSION $SESS_SOCK)"
	exit 2
fi
ino_pre=$(sock_ino "$SESS_SOCK")
# Same-binary handoff is the code path `herdr update --handoff` takes. No
# herdr update is run; the fleet binary is not replaced.
h server live-handoff --import-exe "$HERDR" >/dev/null
# The import server rebinds the same path; give it a moment.
sleep 0.5
wait_running
ino_post=$(sock_ino "$SESS_SOCK")
check "handoff-recreates-socket-inode" "$([ "$ino_post" != "$ino_pre" ] && echo 1 || echo 0)" "before=$ino_pre after=$ino_post"
live=$(h workspace list | json_ids)
check "handoff-live-ids-unchanged" "$([ "$live" = "w1 w2" ] && echo 1 || echo 0)" "got=[$live]"
h3=$(create recycled-handoff)
check "handoff-recycles-closed-max" "$([ "$h3" = "w3" ] && echo 1 || echo 0)" "got=$h3 want=w3"
lab=$(h workspace get w3 | json_label)
check "handoff-recycled-w3-is-stranger-label" "$([ "$lab" = "recycled-handoff" ] && echo 1 || echo 0)" "label=$lab"
h workspace close w3 >/dev/null
post=$(create post-handoff-same)
check "post-handoff-same-process-no-reuse" "$([ "$post" = "w4" ] && echo 1 || echo 0)" "got=$post"

# Fleet must be the same process, same socket inode, same workspace ids.
fleet_ids_after=$(
	unset_herdr
	HERDR_SOCKET_PATH="$FLEET_SOCK" "$HERDR" workspace list | json_ids
)
fleet_pid_after=$(lsof -t "$FLEET_SOCK" 2>/dev/null | head -1 || true)
fleet_ino_after=$(sock_ino "$FLEET_SOCK")
check "fleet-pid-untouched" "$([ "$fleet_pid_before" = "$fleet_pid_after" ] && echo 1 || echo 0)" "before=$fleet_pid_before after=$fleet_pid_after"
check "fleet-socket-inode-untouched" "$([ "$fleet_ino_before" = "$fleet_ino_after" ] && echo 1 || echo 0)" "before=$fleet_ino_before after=$fleet_ino_after"
check "fleet-workspace-ids-untouched" "$([ "$fleet_ids_before" = "$fleet_ids_after" ] && echo 1 || echo 0)" "before=[$fleet_ids_before] after=[$fleet_ids_after]"

echo
if [ "$fail" -ne 0 ]; then
	echo "verify-id-recycle: FAIL"
	exit 1
fi
echo "verify-id-recycle: PASS"
exit 0
