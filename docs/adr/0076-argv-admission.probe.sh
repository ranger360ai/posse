#!/bin/sh
# ADR 0076 amendment probe (ranger-base-zkeik) — on Rex, does a runtime that
# writes no OSC 7501 record at launch write one when its work prompt rides in
# on the launch line (ADR 0013 §2 `prompt: argv`), the way posse dispatches
# codex and grok?
# Usage: sh docs/adr/0076-argv-admission.probe.sh [grok] [grokon] [codexraw] [codex]   (repo root; no args = all)
#        REX=<path to rex> to override the CLI (it is not on PATH by default).
#
# No model turn is possible. Every arm runs its CLI inside
# `sandbox-exec (deny network*)` (the ADR 0013 rules-precedence probe's lock),
# under a scratch home with a synthetic key, so nothing leaves the machine:
#   - grok: scratch GROK_HOME, synthetic XAI_API_KEY, scratch --leader-socket;
#   - codex: scratch CODEX_HOME whose only provider is a dead loopback port,
#     and the ch2rb hook, whose UserPromptSubmit exits 2 (prompt blocked
#     before any request is built).
# No keystroke is sent to any screen except codex's "Hooks need review"
# dialog in the trust-seeding step, and only after the arm has recognised it.
# Each arm creates ONE Rex session labelled zkeik-<arm>-$$ and kills only that.
set -u
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
[ -x "$REX" ] || { echo "rex not found at \$REX"; exit 2; }
CODEX=$(command -v codex) GROK=$(command -v grok) PATHS="PATH=$PATH"
W=$(cd "$(mktemp -d "${TMPDIR:-/tmp}/zkeik.XXXXXX")" && pwd -P)   # codex keys project trust by the resolved path (/private/var)
NONET="sandbox-exec -p '(version 1)(allow default)(deny network*)'"
echo "== rex $("$REX" version | head -1)   codex $(codex --version)   $(grok --version)"
echo "== scratch $W"

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
printf '\033]7501;state=%s:app=%s-hook\033\\' "$st" "$rt" > /dev/tty 2>/dev/null && r=ok || r=FAILED
echo "$rt $ev -> $st tty-write=$r t=$(date +%s)" >> "$W/witness"
[ "$ev" = UserPromptSubmit ] && { echo "zkeik probe: prompt blocked by hook" >&2; exit 2; }
exit 0
EOF
chmod +x "$W/hook.sh"

ARMS=${*:-grok grokon codexraw codex}
want() { case " $ARMS " in *" $1 "*) return 0;; esac; return 1; }
pause() { python3 -I -c "import time; time.sleep($1)"; }
arm() { # arm <name> <cwd> <command> — the line goes in a launcher script (tty canonical buffer, ch2rb)
  S=zkeik-$1-$$
  printf '#!/bin/sh\nexport %s\ncd %s && exec %s\n' "'$PATHS'" "'$2'" "$3" > "$W/$1.launch"; chmod +x "$W/$1.launch"
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  T0=$(date +%s)
  "$REX" send -s "$S" -b "$B" "$W/$1.launch"; "$REX" send-key -s "$S" -b "$B" enter
}
status() { printf '  t+%-3s %-26s ' "$(( $(date +%s) - T0 ))" "$1"; "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}"; echo; }
screen() { "$REX" capture -s "$S" -b "$B" --trim | grep -v '^\s*$' | tail -${1:-6} | sed 's/^/  | /'; }
done_arm() { "$REX" kill "$S" >/dev/null 2>&1; }
witness() { echo "  witness:"; grep "^$1" "$W/witness" 2>/dev/null | sed 's/^/    /' || echo "    (no hook ran)"; }

codexhome() { # codexhome <dir> — dead provider, the hook on every event, update menu off
  mkdir -p "$1"
  cat > "$1/config.toml" <<'EOF'
check_for_update_on_startup = false
model = "probe"
model_provider = "dead"
[model_providers.dead]
name = "dead"
base_url = "http://127.0.0.1:9/v1"
env_key = "ZKEIK_DEAD_KEY"
wire_api = "responses"
EOF
  python3 -I - "$W" > "$1/hooks.json" <<'EOF'
import json, sys
w = sys.argv[1]; cmd = f"'{w}/hook.sh' codex '{w}'"
evs = ["SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse", "Stop", "Interrupt", "SessionEnd"]
print(json.dumps({"hooks": {e: [{"hooks": [{"type": "command", "command": cmd, "timeout": 5}]}] for e in evs}}, indent=1))
EOF
}
codexline() { # codexline <home> <cwd> [prompt]
  echo "$NONET env CODEX_HOME='$1' ZKEIK_DEAD_KEY=x '$CODEX' -c 'projects={\"$2\"={trust_level=\"trusted\"}}' ${3:+\"$3\"}"
}

if want grok; then
echo "== grok: work prompt on the launch line (argv), no network, scratch GROK_HOME, synthetic key"
mkdir -p "$W/grok" "$W/grokhome"
arm grok "$W/grok" "$NONET env GROK_HOME='$W/grokhome' XAI_API_KEY=xai-zkeik-synthetic '$GROK' --leader-socket '$W/leader.sock' 'zkeik argv probe marker'"
for t in 2 3 5 10; do pause $t; status "argv prompt"; done
screen 10
done_arm
fi

if want grokon; then
echo "== grok: the same argv launch, program_status forced on in the workspace's .grok/config.toml"
mkdir -p "$W/grokon/.grok" "$W/grokonhome"
printf '[ui.notifications]\nprogram_status = "on"\n' > "$W/grokon/.grok/config.toml"
arm grokon "$W/grokon" "$NONET env GROK_HOME='$W/grokonhome' XAI_API_KEY=xai-zkeik-synthetic '$GROK' --leader-socket '$W/leaderon.sock' 'zkeik argv probe marker'"
for t in 2 3 5 10 20 30 30 30; do pause $t; status "argv prompt, forced on"; done  # 131 s: a turn end never came
screen 10
done_arm
fi

if want codexraw; then
echo "== codex: argv prompt, hooks declared but NOT trusted (what posse's home would be without seeding)"
codexhome "$W/rawhome"; mkdir -p "$W/codexraw"
arm codexraw "$W/codexraw" "$(codexline "$W/rawhome" "$W/codexraw" 'zkeik argv probe marker')"
for t in 3 5; do pause $t; status "argv prompt, untrusted"; done
witness codex; screen 8
done_arm
: > "$W/witness"
fi

if want codex; then
echo "== codex: step 1, seed hook trust in a scratch home (answer the review dialog once, then /quit)"
codexhome "$W/codexhome"; mkdir -p "$W/codex"
arm codexseed "$W/codex" "$(codexline "$W/codexhome" "$W/codex")"
pause 8
if "$REX" capture -s "$S" -b "$B" --trim | grep -q 'Hooks need review'; then
  "$REX" send-key -s "$S" -b "$B" down; "$REX" send-key -s "$S" -b "$B" enter
  pause 4; echo "  trust written:"; grep -c trusted_hash "$W/codexhome/config.toml" | sed 's/^/    trusted_hash lines: /'
else
  echo "  no hook-review dialog: not answering an unrecognised screen"; screen 8
fi
done_arm; : > "$W/witness"
echo "== codex: step 2, argv prompt in the SEEDED home"
arm codex "$W/codex" "$(codexline "$W/codexhome" "$W/codex" 'zkeik argv probe marker')"
for t in 2 3 5 10; do pause $t; status "argv prompt, trusted"; done
witness codex; screen 6
done_arm
fi
echo "== scratch left at $W (witness, homes, launch lines)"
