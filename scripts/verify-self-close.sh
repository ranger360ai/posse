#!/usr/bin/env bash
# Does a process survive closing the workspace its own pane is in?
#
# That is the whole question under `posse relaunch <name> --no-land` typed
# INSIDE the session it names (ranger-base-hslbb, from ranger-base-521).
# RelaunchSession's destructive half is two steps in one critical section:
#
#     closeRecorded(m)    -> herdr workspace close <m.Workspace>, meta unlink
#     recreateSession()   -> workspace create, meta write, launch line typed
#
# In the self case `m.Workspace` is the workspace the posse process is
# running in. If closing a workspace kills the processes in its panes, the
# process that must run step 2 is gone after step 1 with the meta ALREADY
# removed: a session destroyed with no replacement and its name freed, which
# is the rangerhq-v52t loss shape, self-inflicted.
#
# Usage: scripts/verify-self-close.sh
#        HERDR=/path/to/herdr scripts/verify-self-close.sh
#
# SAFETY. Never aims at the fleet default server. Two independent fences:
#
#  1. A scratch HOME. herdr derives its whole root from $HOME (`herdr --help`
#     prints `Config: $HOME/.config/herdr/config.toml`), so a scratch HOME is
#     a fresh install with an EMPTY workspace list. A scratch SOCKET alone is
#     NOT isolation -- it restores the default session's whole layout out of
#     ~/.config/herdr and re-runs every workspace's command (12 husk copies of
#     the live fleet, measured 2026-09-01).
#  2. A named session under that scratch HOME, as scripts/verify-id-recycle.sh
#     does under the real one, with every ambient HERDR_* stripped so a pane's
#     socket cannot retarget a command.
#
# The scratch HOME is also what lets this run from inside a `cage: seatbelt`
# seat, where `mkdir ~/.config/herdr/sessions/<name>` is Operation not
# permitted and verify-id-recycle.sh therefore cannot start its server.
# /private/tmp is a write grant in the rendered profile and is short enough
# for sun_path (~104 bytes), which the session scratchpad path is not.
#
# The commands under test run in a PANE, and a pane's env is the herdr
# SERVER's: `herdr pane run` types into the pane's shell, so the script is a
# CHILD of that shell (measured: ppid is the pane shell) and HERDR_SOCKET_PATH
# is already the scratch socket there. That lineage is the fleet's own shape
# one level shallower -- in production `posse relaunch` is a grandchild of the
# pane process (pane runtime CLI -> the agent's tool shell -> posse).
set -euo pipefail

HERDR=${HERDR:-$(command -v herdr)}
[ -x "$HERDR" ] || { echo "verify-self-close: not executable: ${HERDR:-<none>}"; exit 2; }

SESSION="selfclose-$$"
FLEET_SOCK=${HOME}/.config/herdr/herdr.sock
# Short root: sun_path caps the socket path at ~104 bytes.
HHOME=$(mktemp -d /private/tmp/pselfclose.XXXXXX)
SESS_SOCK="$HHOME/.config/herdr/sessions/${SESSION}/herdr.sock"
OUT="$HHOME/out"
CWD="$HHOME/cwd"
mkdir -p "$OUT" "$CWD"

unset_herdr() {
	unset HERDR_ENV HERDR_SOCKET_PATH HERDR_CLIENT_SOCKET_PATH \
		HERDR_WORKSPACE_ID HERDR_PANE_ID HERDR_TAB_ID HERDR_SESSION \
		HERDR_BIN_PATH || true
}

# Workspace/pane commands, aimed at the scratch session by BOTH fences.
h() {
	unset_herdr
	env HOME="$HHOME" HERDR_SESSION="$SESSION" HERDR_SOCKET_PATH="$SESS_SOCK" \
		"$HERDR" "$@"
}

# Session-manager commands (status/stop/delete) take --session, not the env.
hs() {
	unset_herdr
	env HOME="$HHOME" "$HERDR" --session "$SESSION" "$@"
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
note() { echo "      $1"; }

py() { python3 -c "$1"; }
json_ids() { py 'import json,sys; print(" ".join(w["workspace_id"] for w in json.load(sys.stdin)["result"]["workspaces"]))'; }
json_labels() { py 'import json,sys; print(" ".join(w.get("label") or "" for w in json.load(sys.stdin)["result"]["workspaces"]))'; }
json_create_id() { py 'import json,sys; print(json.load(sys.stdin)["result"]["workspace"]["workspace_id"])'; }
json_create_pane() { py 'import json,sys; print(json.load(sys.stdin)["result"]["root_pane"]["pane_id"])'; }
json_status_sock() { py 'import json,sys; d=json.load(sys.stdin); print(d.get("server",{}).get("socket") or "")'; }
json_status_session() { py 'import json,sys; d=json.load(sys.stdin); print(d.get("server",{}).get("session") or "")'; }
json_status_running() { py 'import json,sys; d=json.load(sys.stdin); print("1" if d.get("server",{}).get("running") else "0")'; }

wait_running() {
	local n=0
	while [ "$n" -lt 100 ]; do
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
	echo "verify-self-close: named session did not come up on $SESS_SOCK"
	return 1
}

cleanup() {
	unset_herdr
	env HOME="$HHOME" "$HERDR" session stop "$SESSION" >/dev/null 2>&1 || true
	sleep 0.5
	env HOME="$HHOME" "$HERDR" session delete "$SESSION" >/dev/null 2>&1 || true
	rm -rf "$HHOME"
}
trap cleanup EXIT

echo "verify-self-close: $HERDR ($("$HERDR" --version 2>/dev/null | head -1))"
echo "  session : $SESSION"
echo "  socket  : $SESS_SOCK"
echo "  home    : $HHOME"
echo

unset_herdr
env HOME="$HHOME" "$HERDR" --session "$SESSION" server >"$HHOME/server.log" 2>&1 &
wait_running

st=$(hs status --json)
sock=$(printf '%s' "$st" | json_status_sock)
sess=$(printf '%s' "$st" | json_status_session)
# Every arm below closes a workspace. Refuse outright unless the server the
# commands reach is provably the scratch one.
if [ "$sock" != "$SESS_SOCK" ] || [ "$sess" != "$SESSION" ] || [ "$sock" = "$FLEET_SOCK" ]; then
	echo "REFUSING: aim is $sess $sock (want $SESSION $SESS_SOCK)"
	exit 2
fi
check "aimed-at-scratch-socket" 1 ""

ids=$(h workspace list | json_ids)
check "scratch-home-server-is-empty" "$([ -z "$ids" ] && echo 1 || echo 0)" "ids=[$ids] (a scratch SOCKET alone would restore the fleet here)"

# The body of every arm, parameterised. $1 tag, $2 the workspace id to close,
# $3 the label to create afterwards. Written as a file rather than typed as a
# compound so the pane's shell quoting cannot change what runs.
#
#   marker 1  the pane ran the script at all
#   close     rc line appended only if the close call RETURNED
#   create    the recreate's own output and rc
#   marker 2  the process outlived closing that workspace
write_arm() { # write_arm <tag> <ws-to-close> <new-label>
	cat >"$HHOME/arm-$1.sh" <<EOF
export HOME="$HHOME"
export HERDR_SOCKET_PATH="$SESS_SOCK"
echo before >"$OUT/$1.1"
"$HERDR" workspace close $2 >"$OUT/$1.close.out" 2>&1
echo "close rc=\$?" >>"$OUT/$1.close.out"
"$HERDR" workspace create --cwd "$CWD" --label $3 --no-focus >"$OUT/$1.create.out" 2>&1
echo "create rc=\$?" >>"$OUT/$1.create.out"
echo after >"$OUT/$1.2"
EOF
}

marker() { [ -e "$OUT/$1" ] && echo 1 || echo 0; }
rcline() { grep -c "rc=" "$OUT/$1" 2>/dev/null | head -1 || true; }

# ---------------------------------------------------------------- control --
# The SAME script, from a pane, closing a workspace that is NOT its own.
# Without this arm a missing marker 2 in the self case could be a broken rig,
# a pane that never ran, or a create that cannot be made from a pane at all.
ctl=$(h workspace create --cwd "$CWD" --label ctlpane --no-focus)
ctl_pane=$(printf '%s' "$ctl" | json_create_pane)
ctl_id=$(printf '%s' "$ctl" | json_create_id)
victim=$(h workspace create --cwd "$CWD" --label victim --no-focus | json_create_id)
write_arm ctl "$victim" ctlrecreated
h pane run "$ctl_pane" sh "$HHOME/arm-ctl.sh" >/dev/null
sleep 4

check "control-pane-ran" "$(marker ctl.1)" "marker 1 missing: the pane never ran the script"
check "control-outlives-closing-another-workspace" "$(marker ctl.2)" "marker 2 missing in the CONTROL: the rig is broken, not the self case"
check "control-close-returned" "$([ "$(rcline ctl.close.out)" -ge 1 ] && echo 1 || echo 0)" "$(head -c 200 "$OUT/ctl.close.out" 2>/dev/null)"
after_ctl=$(h workspace list | json_labels)
check "control-victim-is-gone" "$(echo "$after_ctl" | grep -qw victim && echo 0 || echo 1)" "labels=[$after_ctl]"
check "control-recreate-landed" "$(echo "$after_ctl" | grep -qw ctlrecreated && echo 1 || echo 0)" "labels=[$after_ctl]"

# ------------------------------------------------------------------- self --
# The measurement. A pane closes its OWN workspace and then tries to create
# the replacement, which is closeRecorded followed by recreateSession.
self=$(h workspace create --cwd "$CWD" --label selfpane --no-focus)
self_pane=$(printf '%s' "$self" | json_create_pane)
self_id=$(printf '%s' "$self" | json_create_id)
write_arm self "$self_id" selfrecreated
h pane run "$self_pane" sh "$HHOME/arm-self.sh" >/dev/null
sleep 4

check "self-pane-ran" "$(marker self.1)" "marker 1 missing: the pane never ran the script, so nothing below is measured"
after_self=$(h workspace list | json_labels)
check "self-close-actually-happened" "$(echo "$after_self" | grep -qw selfpane && echo 0 || echo 1)" "labels=[$after_self] (selfpane still listed: the close was a no-op and the arm measures nothing)"

self_survived=$(marker self.2)
self_landed=$(echo "$after_self" | grep -qw selfrecreated && echo 1 || echo 0)
note "self marker 2 : $self_survived    recreate landed: $self_landed"
note "self close.out: $(head -c 200 "$OUT/self.close.out" 2>/dev/null | tr '\n' ' ')"
note "self create.out: $(head -c 200 "$OUT/self.create.out" 2>/dev/null | tr '\n' ' ')"

if [ "$self_survived" = 1 ] && [ "$self_landed" = 1 ]; then
	echo "VERDICT  the caller OUTLIVES closing its own workspace and the recreate lands."
	echo "         A self \`posse relaunch --no-land\` reaches recreateSession as it stands."
else
	echo "VERDICT  the caller DIES with its own workspace (marker2=$self_survived recreate=$self_landed)."
	echo "         A self \`posse relaunch --no-land\` destroys the session between"
	echo "         closeRecorded and recreateSession: meta already unlinked, name freed."
fi

# ------------------------------------------------- self, detached (supp.) --
# Same self case with the close+create moved off the pane's process group, to
# say whether the death is a signal to the group or the loss of the terminal.
# This is design input for the fix, not part of the verdict above.
det=$(h workspace create --cwd "$CWD" --label detpane --no-focus)
det_pane=$(printf '%s' "$det" | json_create_pane)
det_id=$(printf '%s' "$det" | json_create_id)
write_arm det "$det_id" detrecreated
cat >"$HHOME/arm-det-outer.sh" <<EOF
echo outer >"$OUT/det.0"
nohup sh "$HHOME/arm-det.sh" >/dev/null 2>&1 &
EOF
h pane run "$det_pane" sh "$HHOME/arm-det-outer.sh" >/dev/null
sleep 5
after_det=$(h workspace list | json_labels)
det_survived=$(marker det.2)
det_landed=$(echo "$after_det" | grep -qw detrecreated && echo 1 || echo 0)
note "detached arm  : outer=$(marker det.0) inner-started=$(marker det.1) marker2=$det_survived recreate=$det_landed"
note "detached close: $(head -c 200 "$OUT/det.close.out" 2>/dev/null | tr '\n' ' ')"
note "detached labels: [$after_det]  (detpane absent = its close DID reach the server)"

# The self arm's markers, re-read after ~10s more have passed through the
# detached arm. An absence that is still an absence here is a dead writer,
# not a slow one -- the control wrote both of its markers inside 4s.
late_self=$(marker self.2)
check "self-marker-2-absent-is-final-not-slow" "$([ "$late_self" = "$self_survived" ] && echo 1 || echo 0)" \
	"marker 2 changed from $self_survived to $late_self between the two reads: the wait was too short"

# ----------------------------------------- self, new session leader (supp.) --
# `nohup` only ignores SIGHUP; it leaves the child in the pane's process
# group, so the arm above cannot tell "the close kills the group" from "the
# close kills the terminal". This one calls setsid(2) first -- new session,
# new process group, no controlling terminal -- which is what a detached
# re-exec of posse would have to do. macOS ships no setsid(1); python3 has it.
# If THIS arm survives, refusing the self case is not the only fix available.
sid=$(h workspace create --cwd "$CWD" --label sidpane --no-focus)
sid_pane=$(printf '%s' "$sid" | json_create_pane)
sid_id=$(printf '%s' "$sid" | json_create_id)
write_arm sid "$sid_id" sidrecreated
cat >"$HHOME/arm-sid-outer.sh" <<EOF
echo outer >"$OUT/sid.0"
python3 -c "import os,sys; os.setsid(); os.execv('/bin/sh', ['sh', '$HHOME/arm-sid.sh'])" >/dev/null 2>&1 &
EOF
h pane run "$sid_pane" sh "$HHOME/arm-sid-outer.sh" >/dev/null
sleep 6
after_sid=$(h workspace list | json_labels)
sid_survived=$(marker sid.2)
sid_landed=$(echo "$after_sid" | grep -qw sidrecreated && echo 1 || echo 0)
note "setsid arm    : outer=$(marker sid.0) inner-started=$(marker sid.1) marker2=$sid_survived recreate=$sid_landed"
note "setsid close  : $(head -c 200 "$OUT/sid.close.out" 2>/dev/null | tr '\n' ' ')"
note "setsid labels : [$after_sid]"
# The inner must be shown to have STARTED, or marker2=0 is unanswerable
# rather than a death.
check "setsid-arm-inner-started" "$(marker sid.1)" "the setsid child never ran; its marker 2 says nothing"

# One stable line for a pin to read. These three words are the sentence the
# relaunch decision rests on, so a herdr that ever changes them should turn a
# test red rather than quietly widen what `--no-land` may do.
verdict_word() { [ "$1" = 1 ] && echo survived || echo died; }
echo
echo "verify-self-close: SELF=$(verdict_word "$self_survived") DETACHED=$(verdict_word "$det_survived") SETSID=$(verdict_word "$sid_survived")"

if [ "$fail" = 0 ]; then
	echo "verify-self-close: PASS (all controls)"
else
	echo "verify-self-close: FAIL -- a control failed, so the line above is not evidence"
fi
exit "$fail"
