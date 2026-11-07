#!/bin/sh
# ADR 0076 Amendment 4 probe (ranger-base-778y0) — on Rex, can bob be
# `prompt: argv`: does the top-level `bob -p <prompt>` open the interactive
# client with that prompt submitted, the way `codex <prompt>` and
# `grok <prompt>` do?
# Usage: sh docs/adr/0076-bob-argv.probe.sh [chat] [p] [pchat] [live]   (repo root; no args = the first three)
#        REX=<path to rex> overrides the CLI (it is not on PATH by default).
#
#   chat    `bob chat`, no prompt: the interactive client, as the control.
#   p       `bob -p <prompt>`: which client does the top-level flag start?
#   pchat   `bob -p <prompt> chat`: does a global -p reach `chat`'s TUI?
#   live    pchat under the real login, with blocking workspace hooks: is the
#           argv prompt submitted, and where does a hook's 7501 report land?
#           Its own locks are described at the arm.
#
# In chat, p and pchat NO MODEL TURN IS POSSIBLE, three locks, none of them a keystroke:
#   - every arm runs under its OWN scratch HOME, so bob reads no
#     ~/.bob/settings/auth-secrets.json and holds no token (bob resolves its
#     whole home from os.homedir(), which is $HOME on darwin);
#   - BOB_API_KEY and BOBSHELL_API_KEY are unset for the arm;
#   - HTTPS_PROXY/HTTP_PROXY/ALL_PROXY point at 127.0.0.1:9, where nothing
#     listens, and bob routes every request through that proxy.
# No key is sent to any screen; the prompt text rides only on argv.
# Nothing outside the scratch dir is written. The operator's global bob hooks
# do not load (their file is in the real HOME). Each arm creates ONE Rex
# session labelled b778y0-<arm>-$$ and kills only that.
set -u
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
[ -x "$REX" ] || { echo "rex not found at \$REX"; exit 2; }
BOB=$(command -v bob) || { echo "bob not on PATH"; exit 2; }
W=$(mktemp -d "${TMPDIR:-/tmp}/b778y0.XXXXXX")
echo "== rex $("$REX" version | head -1)   bob $("$BOB" --version | head -1)   node $(node --version)"
echo "== scratch $W"

ARMS=${*:-chat p pchat}
want() { case " $ARMS " in *" $1 "*) return 0;; esac; return 1; }
pause() { python3 -I -c "import time; time.sleep($1)"; }
arm() { # arm <name> <bob args…> — the line goes in a launcher script (a typed
  # line past the tty's 1024-byte canonical buffer is cut, ch2rb).
  name=$1; shift
  S=b778y0-$name-$$
  mkdir -p "$W/$name/home" "$W/$name/ws"
  {
    printf '#!/bin/sh\n'
    printf 'unset BOB_API_KEY BOBSHELL_API_KEY\n'
    printf 'export HOME=%s PATH=%s\n' "'$W/$name/home'" "'$PATH'"
    printf 'export HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9\n'
    printf 'cd %s && exec %s' "'$W/$name/ws'" "'$BOB'"
    for a in "$@"; do printf " '%s'" "$a"; done
    printf '\n'
  } > "$W/$name.launch"
  chmod +x "$W/$name.launch"
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  "$REX" send -s "$S" -b "$B" "$W/$name.launch"; "$REX" send-key -s "$S" -b "$B" enter
}
method() { "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"$1\"}"; }
reading() { printf '  %-22s status=' "$1"; method program_status; printf '\n  %-22s process=' ""; method process; echo; }
screen() { "$REX" capture -s "$S" -b "$B" --trim | grep -v '^\s*$' | tail -${1:-10} | sed 's/^/  | /'; }
done_arm() { "$REX" kill "$S" >/dev/null 2>&1; }

if want chat; then
  echo "== chat: bob chat --accept-license -w . (the interactive client, control)"
  arm chat chat --accept-license -w .
  pause 8; reading "after ~8s"; screen 12
  pause 7; reading "after ~15s"; screen 6
  done_arm
fi

if want p; then
  echo "== p: bob --accept-license -p 'b778y0 probe' (top-level -p)"
  arm p --accept-license -p 'b778y0 probe'
  pause 8; reading "after ~8s"; screen 12
  pause 7; reading "after ~15s"; screen 6
  done_arm
fi

if want pchat; then
  echo "== pchat: bob -p 'b778y0 probe' chat --accept-license -w . (a global -p before chat)"
  arm pchat -p 'b778y0 probe' chat --accept-license -w .
  pause 8; reading "after ~8s"; screen 12
  pause 7; reading "after ~15s"; screen 6
  done_arm
fi
# ─── live: the operator's real login, a hook that blocks every prompt ────────
# The three arms above cannot see past bob's login, so they cannot say whether
# `-p … chat` SUBMITS its prompt. This arm can, and still spends no turn:
#   - the workspace UserPromptSubmit hook exits 2, which bob 2.0.9 reads as
#     "blocked" (HZs) inside the turn-input path, before any request is built;
#   - nothing is typed unless bob's own /hooks panel lists that hook active.
# It runs under the REAL HOME, so the operator's global herdr-bob hooks load
# (outside herdr they exit 0, ch2rb), and `--trust` adds this scratch
# workspace to ~/.bob/trustedFolders.json — the one write outside $W, the same
# one ch2rb's bob arm made. The workspace file also sets disableGlobalHooks,
# to measure whether a WORKSPACE file can switch the global hooks off.
# Not in the default arm list: run it by name.
if want live; then
  echo "== live: bob -p '<prompt>' chat --trust, real login, workspace hooks that block"
  S=b778y0-live-$$ L=$W/live; mkdir -p "$L/ws/.bob"
  cat > "$L/hook.sh" <<'EOF'
#!/bin/sh
W=$1
ev=$(python3 -I -c 'import sys,json; print(json.load(sys.stdin).get("hook_event_name",""))')
case $ev in SessionStart|Stop) st=idle ;; *) st=working ;; esac
if printf '\033]7501;state=%s:app=bob-hook\033\\' "$st" > /dev/tty 2> "$W/err.$$"; then r=ok; else r="FAILED: $(cat "$W/err.$$")"; fi
ids=$(python3 -I -c 'import os; print("ppid=%d sid=%d pgid=%d" % (os.getppid(), os.getsid(0), os.getpgrp()))')
echo "$ev -> $st  tty-write=$r  $ids" >> "$W/witness"
[ "$ev" = UserPromptSubmit ] && { echo "b778y0 probe: prompt blocked by hook" >&2; exit 2; }
exit 0
EOF
  chmod +x "$L/hook.sh"
  python3 -I - "$L" > "$L/ws/.bob/settings.json" <<'EOF'
import json, sys
L = sys.argv[1]
cmd = f"'{L}/hook.sh' '{L}'"
evs = ["SessionStart", "UserPromptSubmit", "PreCompact", "PostCompact", "PreToolUse", "PostToolUse", "Stop"]
print(json.dumps({"disableGlobalHooks": True,
                  "hooks": {e: [{"hooks": [{"type": "command", "command": cmd, "timeout": 5}]}] for e in evs}}, indent=1))
EOF
  printf '#!/bin/sh\nexport PATH=%s\ncd %s && exec %s -p %s chat --accept-license --trust -w .\n' \
    "'$PATH'" "'$L/ws'" "'$BOB'" "'b778y0 argv prompt'" > "$W/live.launch"; chmod +x "$W/live.launch"
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  "$REX" send -s "$S" -b "$B" "$W/live.launch"; "$REX" send-key -s "$S" -b "$B" enter
  witness() { echo "  witness:"; sed 's/^/    /' "$L/witness" 2>/dev/null || echo "    (no hook ran)"; }
  pause 20; reading "argv, t+20s, untyped"; witness; screen 14
  if [ -s "$L/witness" ]; then echo "  a hook ran with nothing typed: the argv prompt reached the turn path"; fi
  # A menu on screen (trust, login, license) would take the Enter: type nothing.
  if "$REX" capture -s "$S" -b "$B" --trim | grep -qE '→ 1\.|Do you trust|[Ss]ign in|[Ll]og ?in'; then
    echo "  a menu is up: typing nothing"; screen 20; done_arm; echo "== scratch left at $W"; exit 0
  fi
  "$REX" send -s "$S" -b "$B" /hooks; "$REX" send-key -s "$S" -b "$B" enter; pause 3
  echo "  /hooks panel:"; "$REX" capture -s "$S" -b "$B" --trim | grep -E 'Global|Workspace|active|disabled' | sed 's/^/  | /'
  if "$REX" capture -s "$S" -b "$B" --trim | grep -q 'UserPromptSubmit · Workspace · ● active'; then
    "$REX" send-key -s "$S" -b "$B" escape; pause 1
    "$REX" send -s "$S" -b "$B" "b778y0 typed prompt"; "$REX" send-key -s "$S" -b "$B" enter
    pause 8; reading "typed, t+8s"; witness; screen 5
  else
    echo "  workspace UserPromptSubmit hook NOT listed active: no prompt typed"; screen 20
  fi
  "$REX" send -s "$S" -b "$B" /quit; "$REX" send-key -s "$S" -b "$B" enter
  pause 6; reading "after /quit"
  done_arm
fi
echo "== scratch left at $W"
