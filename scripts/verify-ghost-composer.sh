#!/usr/bin/env bash
# Is text previewed in a claude pane's composer TYPED, or a GHOST of a line
# that was already sent? (ranger-base-2hvtv, from ranger-base-wr624)
#
# THE QUESTION. posse reads `prompt_box_body` off `herdr agent explain` and
# calls any text there "a prompt sitting UNSENT in its box"
# (internal/posse/panework.go). ranger-base-wr624 measured that reading
# naming the operator's last SENT line for ~10 hours over a box `posse peek`
# showed empty, and took it out of the pulse's delivery path. Two callers
# still act on it — dispatch's --resume skip and govern's G2 row — and this
# script is the measurement that says whether a discriminator exists.
#
# WHAT IT MEASURES. herdr's region preview is ANSI-STRIPPED, so posse sees
# the characters and not how they were drawn. Read the same pane WITH the
# escapes and the two cases may not look alike:
#
#   typed  the operator (or `herdr agent prompt`) put text in the buffer
#   ghost  claude re-draws an already-sent line in an EMPTY box
#
# Both arms are run against a live claude in a scratch herdr, one after the
# other in the SAME pane, so nothing but the send separates them.
#
#   arm A  send-text "<marker>", never submitted   -> genuinely typed
#   arm B  submit it, wait for the turn to end     -> whatever the box holds
#
# The verdict line reports the SGR run covering the composer text in each
# arm. A discriminator exists if and only if the two differ.
#
# STATUS, read this before running it: from a `cage: seatbelt` seat it does
# not get that far — claude never reaches a composer there at all, for the
# reason and with the three arms written out above the launch line below. It
# is kept because that negative is the finding, and because from an UNCAGED
# shell the rig is sound. The bead it was written for was answered another
# way in the end: internal/posse/sentline.go asks claude's own submit log
# instead of asking the screen how the text was drawn.
#
# Usage: scripts/verify-ghost-composer.sh
#        HERDR=/path/to/herdr CLAUDE=/path/to/claude scripts/verify-ghost-composer.sh
#
# SAFETY, the same two fences as scripts/verify-self-close.sh — a scratch
# HOME (herdr derives its whole root from $HOME, so this is a fresh install
# with an EMPTY workspace list; a scratch SOCKET alone is NOT isolation) and
# a named session under it, with every ambient HERDR_* stripped. The fleet
# server is never addressed and is refused by name before any arm runs.
#
# claude itself runs with the REAL HOME, because auth and the trusted-folder
# list live there and a scratch HOME lands on onboarding instead of a
# composer. That is the one thing here that is not sandboxed; it costs one
# entry in the operator's project map for a cwd that already has one, and
# arm B costs one short turn.
set -euo pipefail

HERDR=${HERDR:-$(command -v herdr || true)}
CLAUDE=${CLAUDE:-$(command -v claude || true)}
[ -x "$HERDR" ] || { echo "verify-ghost-composer: not executable: ${HERDR:-<none>}"; exit 2; }
[ -x "$CLAUDE" ] || { echo "verify-ghost-composer: not executable: ${CLAUDE:-<none>}"; exit 2; }

REAL_HOME=$HOME
SESSION="ghostbox-$$"
FLEET_SOCK=${REAL_HOME}/.config/herdr/herdr.sock
# Short root: sun_path caps the socket path at ~104 bytes.
HHOME=$(mktemp -d /private/tmp/pghost.XXXXXX)
SESS_SOCK="$HHOME/.config/herdr/sessions/${SESSION}/herdr.sock"
OUT="$HHOME/out"
mkdir -p "$OUT"
MARKER=${MARKER:-"ghost probe $$ say ok"}

unset_herdr() {
	unset HERDR_ENV HERDR_SOCKET_PATH HERDR_CLIENT_SOCKET_PATH \
		HERDR_WORKSPACE_ID HERDR_PANE_ID HERDR_TAB_ID HERDR_SESSION \
		HERDR_BIN_PATH || true
}

h() {
	unset_herdr
	env HOME="$HHOME" HERDR_SESSION="$SESSION" HERDR_SOCKET_PATH="$SESS_SOCK" \
		"$HERDR" "$@"
}

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
	echo "verify-ghost-composer: named session did not come up on $SESS_SOCK"
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

# composer_sgr <ansi-capture-file>
# Prints the SGR parameters in force over the composer's text, and the text.
# The live composer is the LAST `❯` line in the capture.
composer_sgr() {
	CAP="$1" py '
import os, re, sys
raw = open(os.environ["CAP"], "rb").read().decode("utf-8", "replace")
lines = [l for l in raw.split("\n") if "❯" in l]
if not lines:
    print("NOCOMPOSER"); sys.exit(0)
line = lines[-1]
i = line.index("❯") + 1
rest = line[i:]
sgr = []
out = []
pos = 0
for m in re.finditer(r"\x1b\[([0-9;]*)m", rest):
    out.append(rest[pos:m.start()])
    if not "".join(out).strip(" \xa0\r"):
        sgr = [p for p in m.group(1).split(";") if p not in ("", "0")] or ["<reset>"]
    pos = m.end()
out.append(rest[pos:])
text = "".join(out).strip(" \xa0\r\n")
print("sgr=%s text=%r" % (",".join(sgr) if sgr else "<none>", text))
'
}

echo "verify-ghost-composer: $HERDR ($("$HERDR" --version 2>/dev/null | head -1))"
echo "  claude  : $CLAUDE ($("$CLAUDE" --version 2>/dev/null | head -1))"
echo "  session : $SESSION"
echo "  socket  : $SESS_SOCK"
echo "  home    : $HHOME"
echo "  marker  : $MARKER"
echo

unset_herdr
env HOME="$HHOME" "$HERDR" --session "$SESSION" server >"$HHOME/server.log" 2>&1 &
wait_running

st=$(hs status --json)
sock=$(printf '%s' "$st" | json_status_sock)
sess=$(printf '%s' "$st" | json_status_session)
if [ "$sock" != "$SESS_SOCK" ] || [ "$sess" != "$SESSION" ] || [ "$sock" = "$FLEET_SOCK" ]; then
	echo "REFUSING: aim is $sess $sock (want $SESSION $SESS_SOCK)"
	exit 2
fi
check "aimed-at-scratch-socket" 1 ""

# The launch line, written to a file so the pane shell's quoting cannot
# change what runs. CLAUDE_CODE_*/CLAUDECODE come from THIS session and would
# otherwise be inherited through the server into the pane.
#
# NO CREDENTIAL IS HANDED OVER, AND THAT IS A FINDING, NOT AN OMISSION.
# A claude launched from a `cage: seatbelt` seat cannot read the operator's
# keychain pair — ADR 0042: posse's L1 shims sit at the head of PATH for every
# process in the pane, and claude reads its OAuth token by shelling out to the
# keychain CLI — so it walks into the browser sign-in screen instead of drawing
# a composer. Three arms were run from such a seat on 2026-09-05, claude
# 2.1.261, and all three stop at the same screen:
#
#	no token at all                      OAuth sign-in
#	a synthetic CLAUDE_CODE_OAUTH_TOKEN  OAuth sign-in (validated at startup)
#	the real session mint out of the     OAuth sign-in
#	  persona's env set
#
# So this rig CANNOT reach its own arms from a dispatched seat, and the third
# arm is not worth keeping in a shipped script at all: reading a mint here and
# handing it to a spawned CLI is a credential surface bought for a measurement
# that does not come back, and a shipped runnable file has no promotion gate
# in front of it (ADR 0019 D3). It was removed; nothing below reads one. Run this by hand from an UNCAGED shell — where claude reads the
# keychain normally — or do not run it: the question it asks was answered a
# different way (sentline.go reads the store instead of the screen).
cat >"$HHOME/launch.sh" <<EOF
export HOME="$REAL_HOME"
exec env -u CLAUDECODE -u CLAUDE_CODE_ENTRYPOINT -u CLAUDE_CODE_SSE_PORT "$CLAUDE"
EOF

ws=$(h workspace create --cwd "$REAL_HOME" --label ghostprobe --no-focus)
pane=$(printf '%s' "$ws" | json_create_pane)
h pane run "$pane" sh "$HHOME/launch.sh" >/dev/null

# Wait for a composer: herdr recognising claude AND live_prompt_box matching.
#
# A claude whose binary updated under it re-shows its one-key setup choosers
# (the theme list, measured on 2.1.261's first run after the 2.1.258 update),
# and those are drawn over the composer. `enter` takes the default on each;
# nothing here depends on which theme, and a chooser that does not clear is
# reported by the check below rather than pressed at forever.
at_composer() {
	h agent explain "$pane" --json >"$OUT/boot.json" 2>/dev/null || return 1
	py 'import json,sys; d=json.load(open("'"$OUT"'/boot.json")); sys.exit(0 if (d.get("matched_rule") or {}).get("id")=="live_prompt_box" else 1)'
}
n=0
ready=0
nudges=0
while [ "$n" -lt 120 ]; do
	if at_composer; then
		ready=1
		break
	fi
	# Only after claude is actually up (herdr names the agent) and only a
	# bounded number of times.
	if [ "$n" -ge 10 ] && [ "$nudges" -lt 4 ] && [ $((n % 6)) = 0 ]; then
		h pane send-keys "$pane" enter >/dev/null 2>&1 || true
		nudges=$((nudges + 1))
	fi
	n=$((n + 1))
	sleep 0.5
done
note "setup choosers dismissed: $nudges"
check "claude-reached-a-live-composer" "$ready" "$(h pane read "$pane" --source detection 2>&1 | tail -12)"
[ "$ready" = 1 ] || exit 1

# ------------------------------------------------------------------ empty --
# The baseline: what an untouched composer looks like. Without it a dim arm
# A could be "claude draws every composer dim" rather than a discriminator.
h pane read "$pane" --source visible --format ansi >"$OUT/empty.ansi" 2>&1
empty_sgr=$(composer_sgr "$OUT/empty.ansi")
note "empty : $empty_sgr"

# ------------------------------------------------------------------ typed --
h pane send-text "$pane" "$MARKER" >/dev/null
sleep 1.5
h pane read "$pane" --source visible --format ansi >"$OUT/typed.ansi" 2>&1
h agent explain "$pane" --json >"$OUT/typed.json" 2>&1 || true
typed_sgr=$(composer_sgr "$OUT/typed.ansi")
note "typed : $typed_sgr"
check "typed-text-reaches-the-composer" \
	"$(printf '%s' "$typed_sgr" | grep -qF "$MARKER" && echo 1 || echo 0)" \
	"send-text did not land: $typed_sgr"

# ------------------------------------------------------------------ ghost --
h pane send-keys "$pane" enter >/dev/null
# The turn has to END, or the box is empty because claude is working.
h agent wait "$pane" --until idle --until done --timeout 180000 >/dev/null 2>&1 || true
sleep 2
h pane read "$pane" --source visible --format ansi >"$OUT/ghost.ansi" 2>&1
h agent explain "$pane" --json >"$OUT/ghost.json" 2>&1 || true
ghost_sgr=$(composer_sgr "$OUT/ghost.ansi")
note "ghost : $ghost_sgr"

# What posse itself would read in each arm — the ANSI-stripped preview.
preview() { py 'import json,sys
d=json.load(open(sys.argv[1]))
for r in d.get("evaluated_rules",[]):
    if r.get("region")=="prompt_box_body":
        print(repr(r["evidence"]["region_preview"])); break
else: print("<no prompt_box_body rule>")' "$1"; }
note "posse would read, typed arm: $(preview "$OUT/typed.json")"
note "posse would read, ghost arm: $(preview "$OUT/ghost.json")"

echo
echo "VERDICT"
echo "  empty $empty_sgr"
echo "  typed $typed_sgr"
echo "  ghost $ghost_sgr"
check "a-discriminator-exists" \
	"$([ "${typed_sgr%% text=*}" != "${ghost_sgr%% text=*}" ] && echo 1 || echo 0)" \
	"typed and ghost are drawn the same way — the SGR run is not a discriminator"

# Keep the captures: they are the fixtures a pin is built from.
KEEP=${KEEP:-}
if [ -n "$KEEP" ]; then
	mkdir -p "$KEEP"
	cp "$OUT"/*.ansi "$OUT"/*.json "$KEEP"/ 2>/dev/null || true
	echo "      captures kept in $KEEP"
fi

exit "$fail"
