#!/bin/sh
# ADR 0076 spike probe (ranger-base-ch2rb) — on Rex, can a codex or bob HOOK
# report OSC 7501 program status, and does grok report a record at launch?
# Usage: sh docs/adr/0076-hook-route.probe.sh [grok] [grokon] [codex] [bob] [ctl]   (repo root; no args = all)
#        REX=<path to rex> to override the CLI (it is not on PATH by default).
#
# No model turn is possible — two locks per prompted runtime, and no keystroke
# is sent to a screen the arm has not first recognised:
#   - every UserPromptSubmit hook exits 2, which both CLIs read as "block this
#     prompt" before any request is built;
#   - codex runs under a scratch CODEX_HOME whose only provider is a dead
#     loopback port;
#   - bob cannot start behind a dead proxy (MEASURED: it hangs at
#     "Initializing…"), so its second lock is a gate instead: the arm reads
#     bob's own /hooks panel and types no prompt unless the workspace
#     UserPromptSubmit hook is listed active. Its block is a throw inside
#     onTurnInput, before the turn is built (static read of the 2.0.7 bundle).
#   - grok is launched and never prompted.
# Nothing global is written: codex's hooks live in the scratch CODEX_HOME,
# bob's in the scratch workspace's .bob/settings.json. The operator's own
# global bob hooks (herdr-bob's bob-hook) still fire; outside herdr they exit 0.
# Each arm creates ONE Rex session labelled ch2rb-<arm>-$$ and kills only that.
set -u
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
[ -x "$REX" ] || { echo "rex not found at \$REX"; exit 2; }
CODEX=$(command -v codex) BOB=$(command -v bob) GROK=$(command -v grok) PATHS="PATH=$PATH"
W=$(mktemp -d "${TMPDIR:-/tmp}/ch2rb.XXXXXX")
echo "== rex $("$REX" version | head -1)   codex $(codex --version)   bob $(bob --version | head -1)   $(grok --version)"
echo "== scratch $W"

# The hook: map the event to a state, write the report to /dev/tty, and
# record what happened in a witness file (the hook's own stdout is a pipe).
cat > "$W/hook.sh" <<'EOF'
#!/bin/sh
rt=$1; W=$2
ev=$(python3 -I -c 'import sys,json; print(json.load(sys.stdin).get("hook_event_name",""))')
case $ev in
  SessionStart|Stop|Interrupt) st=idle ;;
  PermissionRequest) st=blocked:kind=permission ;;
  SessionEnd) st=clear ;;
  *) st=working ;;
esac
if printf '\033]7501;state=%s:app=%s-hook\033\\' "$st" "$rt" > /dev/tty 2> "$W/err.$$"; then r=ok; else r="FAILED: $(cat "$W/err.$$")"; fi
ids=$(python3 -I -c 'import os; print("pid=%d sid=%d pgid=%d" % (os.getppid(), os.getsid(0), os.getpgrp()))')
echo "$rt $ev -> $st  tty-write=$r  parent-$ids" >> "$W/witness"
if [ "$ev" = UserPromptSubmit ] && [ ! -e "$W/noblock" ]; then echo "ch2rb probe: prompt blocked by hook" >&2; exit 2; fi
exit 0
EOF
chmod +x "$W/hook.sh"

ARMS=${*:-grok grokon codex bob ctl}
want() { case " $ARMS " in *" $1 "*) return 0;; esac; return 1; }
pause() { python3 -I -c "import time; time.sleep($1)"; }
arm() { # arm <name> <cwd> <command> — the line goes in a launcher script,
  # because a typed line longer than the tty's 1024-byte canonical buffer is
  # cut (MEASURED here: a PATH-carrying line never started codex).
  S=ch2rb-$1-$$
  printf '#!/bin/sh\nexport %s\ncd %s && exec %s\n' "'$PATHS'" "'$2'" "$3" > "$W/$1.launch"; chmod +x "$W/$1.launch"
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  "$REX" send -s "$S" -b "$B" "$W/$1.launch"; "$REX" send-key -s "$S" -b "$B" enter
}
status() { printf '  %-28s ' "$1"; "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}"; echo; }
screen() { "$REX" capture -s "$S" -b "$B" --trim | grep -v '^\s*$' | tail -${1:-6} | sed 's/^/  | /'; }
prompt() { "$REX" send -s "$S" -b "$B" "${1:-ch2rb probe}"; "$REX" send-key -s "$S" -b "$B" enter; }
key() { "$REX" send-key -s "$S" -b "$B" "$@"; }
done_arm() { "$REX" kill "$S" >/dev/null 2>&1; }
witness() { echo "  witness:"; sed 's/^/    /' "$W/witness" 2>/dev/null | grep "^    $1" || echo "    (no hook ran)"; }

hooks_json() { # hooks_json <runtime> <event…>
  rt=$1; shift
  python3 -I - "$W" "$rt" "$@" <<'EOF'
import json, sys
w, rt, evs = sys.argv[1], sys.argv[2], sys.argv[3:]
cmd = f"'{w}/hook.sh' {rt} '{w}'"
print(json.dumps({"hooks": {e: [{"hooks": [{"type": "command", "command": cmd, "timeout": 5}]}] for e in evs}}, indent=1))
EOF
}

if want grok; then
echo "== grok: launch only, never prompted"
mkdir -p "$W/grok"
arm grok "$W/grok" "'$GROK'"
pause 3; status "after ~3s"; pause 3; status "after ~6s"; pause 6; status "after ~12s"
screen 8
done_arm
fi

if want grokon; then
echo "== grok: launch only, program_status forced on in the workspace's .grok/config.toml"
mkdir -p "$W/grokon/.grok"
printf '[ui.notifications]\nprogram_status = "on"\n' > "$W/grokon/.grok/config.toml"
arm grokon "$W/grokon" "'$GROK'"
pause 3; status "after ~3s"; pause 3; status "after ~6s"; pause 6; status "after ~12s"
screen 8
done_arm
fi

if want codex; then
echo "== codex: hooks in a scratch CODEX_HOME, dead provider; the hook-review dialog is answered in that scratch home"
mkdir -p "$W/codexhome" "$W/codex"
cat > "$W/codexhome/config.toml" <<'EOF'
check_for_update_on_startup = false   # posse's fleet pin: a fresh home otherwise draws
model = "probe"                       # the update menu, whose default runs brew upgrade
model_provider = "dead"
[model_providers.dead]
name = "dead"
base_url = "http://127.0.0.1:9/v1"
env_key = "CH2RB_DEAD_KEY"
wire_api = "responses"
EOF
hooks_json codex SessionStart UserPromptSubmit PreToolUse PermissionRequest PostToolUse Stop Interrupt SessionEnd > "$W/codexhome/hooks.json"
arm codex "$W/codex" "env CODEX_HOME='$W/codexhome' CH2RB_DEAD_KEY=x '$CODEX' -c 'projects={\"$W/codex\"={trust_level=\"trusted\"}}'"
pause 8; status "launched: review dialog up"; witness codex; screen 7
# Keys go only to a screen we recognise. The one time this arm typed blind, a
# fresh home's update menu took the Enter and ran brew upgrade (pinned; no-op).
if "$REX" capture -s "$S" -b "$B" --trim | grep -q 'Hooks need review'; then
  key down; key enter   # "2. Trust all and continue" — trust lands in the scratch CODEX_HOME
  pause 5; status "trusted, at the composer"; witness codex
  prompt; pause 6; status "after one blocked prompt"; witness codex; screen 3
  touch "$W/noblock"     # the hook stops blocking; the turn then hangs on the dead provider
  prompt "ch2rb probe two"; pause 8; status "unblocked turn, dead provider"; screen 4
  key escape; pause 4; status "after Esc (interrupt)"; witness codex
  prompt /quit; pause 8; status "after /quit"; witness codex; screen 3
  rm -f "$W/noblock"
else
  echo "  no hook-review dialog: not answering an unrecognised screen"
fi
done_arm
fi

if want bob; then
echo "== bob: workspace hooks in .bob/settings.json; prompt only if /hooks lists them active"
mkdir -p "$W/bob/.bob"
hooks_json bob SessionStart UserPromptSubmit PreToolUse PostToolUse Stop > "$W/bob/.bob/settings.json"
arm bob "$W/bob" "'$BOB' chat --accept-license --trust --auto-approve -w ."
pause 15; status "launched, no prompt"; witness bob
prompt /hooks; pause 3
if "$REX" capture -s "$S" -b "$B" --trim | grep -q 'UserPromptSubmit · Workspace · ● active'; then
  key escape; pause 1
  prompt; pause 6; status "after one blocked prompt"; witness bob; screen 3
else
  echo "  workspace UserPromptSubmit hook NOT listed active: no prompt typed"; screen 14
fi
prompt /quit; pause 5; status "after /quit"
done_arm
fi

if want ctl; then
echo "== control: the same hook run from the shell, no runtime in the foreground"
mkdir -p "$W/ctl"; arm ctl "$W/ctl" "/bin/sh"
"$REX" send -s "$S" -b "$B" "echo '{\"hook_event_name\":\"PreToolUse\"}' | '$W/hook.sh' control '$W' > /dev/null"; "$REX" send-key -s "$S" -b "$B" enter
pause 2; status "control hook from the shell"; witness control
done_arm
fi
echo "== scratch left at $W (witness, hooks, configs)"
