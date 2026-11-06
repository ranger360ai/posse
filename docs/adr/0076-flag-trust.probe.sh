#!/bin/sh
# ADR 0076 amendment 2 probe (ranger-base-dok0a) — can posse admit codex on Rex
# WITHOUT writing any codex home: the 7501 hooks declared as `-c` session
# flags, their trust typed as `-c hooks.state`, and the operator's own hooks
# switched off by key instead of by `--disable hooks`?
# Usage: sh docs/adr/0076-flag-trust.probe.sh [seed] [flag] [userhook] [ask]   (repo root; no args = all, in order)
#        REX=<path to rex> to override the CLI.
#
# No model turn is possible, the same lock as 0076-argv-admission.probe.sh:
# every codex runs inside `sandbox-exec (deny network*)`, in a scratch
# CODEX_HOME whose only provider is a dead loopback port, and the hook's
# UserPromptSubmit exits 2. One keystroke pair is sent, to the "Hooks need
# review" dialog in `seed`, and only after that screen is recognised.
#
#   seed      hooks as session flags, no trust: does the review dialog fire,
#             and what key and hash does "Trust all" write into the home?
#   flag      a FRESH home, the same hooks, plus that trust as `-c
#             hooks.state` — no dialog, a `working` record owned by codex,
#             and the home's config.toml byte-identical before and after?
#   userhook  the flag arm plus a user-layer hooks.json in the home (the
#             herdr-agent-state.sh shape) disabled by key with enabled=false
#             and NO hash: does it still draw the review dialog?
#   ask       posse's route: `codex app-server` hooks/list under the seat's
#             own -c line, and every hook it lists typed onto the line —
#             posse's trusted, everyone else's disabled AND trusted. No hash
#             is modelled; the seed arm's answer is not used.
set -u
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
[ -x "$REX" ] || { echo "rex not found at \$REX"; exit 2; }
CODEX=$(command -v codex) PATHS="PATH=$PATH"
W=$(cd "$(mktemp -d "${TMPDIR:-/tmp}/dok0a.XXXXXX")" && pwd -P)   # codex keys project trust by the resolved path
NONET="sandbox-exec -p '(version 1)(allow default)(deny network*)'"
echo "== rex $("$REX" version | head -1)   $(codex --version)"
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
[ "$ev" = UserPromptSubmit ] && { echo "dok0a probe: prompt blocked by hook" >&2; exit 2; }
exit 0
EOF
cat > "$W/userhook.sh" <<'EOF'
#!/bin/sh
echo "USER-LAYER HOOK RAN t=$(date +%s)" >> "$1/witness"; exit 0
EOF
chmod +x "$W/hook.sh" "$W/userhook.sh"

ARMS=${*:-seed flag userhook ask}
want() { case " $ARMS " in *" $1 "*) return 0;; esac; return 1; }
pause() { python3 -I -c "import time; time.sleep($1)"; }
arm() { # arm <name> <cwd> <command>
  S=dok0a-$1-$$
  printf '#!/bin/sh\nexport %s\ncd %s && exec %s\n' "'$PATHS'" "'$2'" "$3" > "$W/$1.launch"; chmod +x "$W/$1.launch"
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  T0=$(date +%s)
  "$REX" send -s "$S" -b "$B" "$W/$1.launch"; "$REX" send-key -s "$S" -b "$B" enter
}
status() { printf '  t+%-3s %-30s ' "$(( $(date +%s) - T0 ))" "$1"; "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}"; echo; }
screen() { "$REX" capture -s "$S" -b "$B" --trim | grep -v '^\s*$' | tail -${1:-6} | sed 's/^/  | /'; }
done_arm() { "$REX" kill "$S" >/dev/null 2>&1; }
witness() { echo "  witness:"; [ -s "$W/witness" ] && sed 's/^/    /' "$W/witness" || echo "    (no hook ran)"; : > "$W/witness"; }

codexhome() { # codexhome <dir> — dead provider, update menu off, NO hooks in the home
  mkdir -p "$1"
  cat > "$1/config.toml" <<'EOF'
check_for_update_on_startup = false
model = "probe"
model_provider = "dead"
[model_providers.dead]
name = "dead"
base_url = "http://127.0.0.1:9/v1"
env_key = "DOK0A_DEAD_KEY"
wire_api = "responses"
EOF
}
# The hooks, as ONE session-flag inline table: every event, one group, one handler.
EVS="SessionStart UserPromptSubmit PreToolUse PermissionRequest PostToolUse Stop Interrupt SessionEnd"
HOOKS=$(python3 -I - "$W" $EVS <<'EOF'
import sys
w, evs = sys.argv[1], sys.argv[2:]
cmd = f"{w}/hook.sh codex {w}"   # unquoted: the line rides inside single quotes, and pwd -P paths hold no space
print("{" + ",".join(f'{e}=[{{hooks=[{{type="command",command="{cmd}",timeout=3}}]}}]' for e in evs) + "}")
EOF
)
codexline() { # codexline <home> <cwd> <extra -c args> [prompt]
  echo "$NONET env CODEX_HOME='$1' DOK0A_DEAD_KEY=x '$CODEX' -c 'projects={\"$2\"={trust_level=\"trusted\"}}' -c 'hooks=$HOOKS' $3 ${4:+\"$4\"}"
}
states() { # states <home> -> one `-c hooks.state={...}` built from the trust the seed arm wrote
  python3 -I - "$1" <<'EOF'
import sys, tomllib
st = tomllib.load(open(sys.argv[1] + "/config.toml", "rb")).get("hooks", {}).get("state", {})
body = ",".join(f'"{k}"={{trusted_hash="{v["trusted_hash"]}"}}' for k, v in st.items() if "trusted_hash" in v)
print(f"-c 'hooks.state={{{body}}}'" if body else "")
EOF
}

if want seed; then
echo "== seed: hooks as session flags in a scratch home, no trust; answer the review dialog once"
codexhome "$W/seedhome"; mkdir -p "$W/seed"
arm seed "$W/seed" "$(codexline "$W/seedhome" "$W/seed" "")"
pause 8
if "$REX" capture -s "$S" -b "$B" --trim | grep -q 'Hooks need review'; then
  echo "  review dialog: YES"
  "$REX" send-key -s "$S" -b "$B" down; "$REX" send-key -s "$S" -b "$B" enter
  pause 4
  echo "  trust written into the home:"
  python3 -I -c 'import sys,tomllib; [print("    %s = %s" % (k, v)) for k, v in tomllib.load(open(sys.argv[1],"rb")).get("hooks",{}).get("state",{}).items()]' "$W/seedhome/config.toml"
  # codex's trust identity (hooks/src/engine/discovery.rs hook_hash + config/src/fingerprint.rs
  # version_for_toml at rust-v0.153.4): sha256 of sorted-key compact JSON of
  # {event_name, hooks:[normalized handler]}, None fields absent. Recomputed here, not read.
  python3 -I - "$W" <<'EOF'
import sys, json, hashlib, tomllib
w = sys.argv[1]; cmd = f"{w}/hook.sh codex {w}"
st = tomllib.load(open(w + "/seedhome/config.toml", "rb"))["hooks"]["state"]
h = lambda o: "sha256:" + hashlib.sha256(json.dumps(o, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()
ok = sum(h({"event_name": k.split(":")[1], "hooks": [{"type": "command", "command": cmd, "timeout": 3, "async": False}]}) == v["trusted_hash"] for k, v in st.items())
print(f"  hash model reproduces codex's own trust: {ok}/{len(st)}")
EOF
else
  echo "  review dialog: no — not answering an unrecognised screen"; screen 10
fi
witness; done_arm
fi

if want flag; then
STATES=$(states "$W/seedhome")
[ -n "$STATES" ] || { echo "== flag: no seeded trust to type (run seed first)"; exit 1; }
echo "== flag: FRESH home, the same hooks, trust typed as -c hooks.state, argv prompt"
codexhome "$W/flaghome"; mkdir -p "$W/flag"
before=$(shasum -a 256 "$W/flaghome/config.toml" | cut -c1-16)
arm flag "$W/flag" "$(codexline "$W/flaghome" "$W/flag" "$STATES" 'dok0a argv probe marker')"
for t in 2 3 5 10 20; do pause $t; status "argv, trust on the line"; done
"$REX" capture -s "$S" -b "$B" --trim | grep -q 'Hooks need review' && echo "  review dialog: YES" || echo "  review dialog: no"
witness; screen 6; done_arm
after=$(shasum -a 256 "$W/flaghome/config.toml" | cut -c1-16)
echo "  home config.toml sha256 before $before after $after ($( [ "$before" = "$after" ] && echo unchanged || echo CHANGED))"
ls -A "$W/flaghome" | sed 's/^/  home holds: /'
fi

userhome() { # userhome <dir> — a codexhome plus a user-layer hooks.json (the herdr-agent-state.sh shape), never trusted
  codexhome "$1"
  printf '{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"%s/userhook.sh %s","timeout":3}]}]}}\n' "$W" "$W" > "$1/hooks.json"
}
if want userhook; then
STATES=$(states "$W/seedhome")
echo "== userhook: flag arm + an untrusted user-layer hooks.json, disabled by key ONLY (enabled=false, no hash)"
userhome "$W/userhome"; mkdir -p "$W/userhook"
UKEY="$W/userhome/hooks.json:session_start:0:0"
# Both spellings in ONE table: a second `-c hooks.state=` would replace the first.
STATES2=$(printf '%s' "$STATES" | sed "s|}}'\$|},\"$UKEY\"={enabled=false}}'|")
arm userhook "$W/userhook" "$(codexline "$W/userhome" "$W/userhook" "$STATES2" 'dok0a argv probe marker')"
for t in 2 3 5 10 20; do pause $t; status "argv, user hook disabled"; done
"$REX" capture -s "$S" -b "$B" --trim | grep -q 'hook is new or changed\|hooks are new or changed\|Hooks need review' && echo "  review dialog: YES" || echo "  review dialog: no"
witness; screen 6; done_arm
fi

# ask: posse's route. Nothing is modelled and nothing is read from the seed arm:
# `codex app-server` is asked for `hooks/list` under the SAME -c line the seat
# will get, and every hook it reports goes onto the line — posse's own as
# {trusted_hash}, every other source's as {enabled=false, trusted_hash}.
# A disabled hook never runs (discovery.rs hook_enabled); trusting it only
# keeps the startup review (tui hooks_rpc.rs hook_needs_review) from firing.
cat > "$W/ask.py" <<'EOF'
import json, subprocess, sys, time, select
home, cwd, hooks = sys.argv[1], sys.argv[2], sys.argv[3]
argv = ["sandbox-exec", "-p", "(version 1)(allow default)(deny network*)", "env", f"CODEX_HOME={home}", "DOK0A_DEAD_KEY=x",
        "codex", "app-server", "-c", f'projects={{"{cwd}"={{trust_level="trusted"}}}}', "-c", hooks]
t0 = time.time()
p = subprocess.Popen(argv, cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
def send(o): p.stdin.write(json.dumps(o) + "\n"); p.stdin.flush()
def recv(i, limit=20):
    end = time.time() + limit
    while time.time() < end:
        if select.select([p.stdout], [], [], 0.5)[0]:
            line = p.stdout.readline()
            if not line: return None
            o = json.loads(line)
            if o.get("id") == i: return o
send({"id": 1, "method": "initialize", "params": {"clientInfo": {"name": "posse-probe", "version": "0"}}}); recv(1)
send({"method": "initialized"})
send({"id": 2, "method": "hooks/list", "params": {"cwds": [cwd]}}); r = recv(2)
p.terminate(); p.wait(5)
if not r or "result" not in r: sys.exit(f"hooks/list failed: {r}")
rows = [h for inv in r["result"]["data"] for h in inv.get("hooks", [])]
print(f"  hooks/list answered in {time.time()-t0:.1f}s:", file=sys.stderr)
for h in rows: print(f'    {h["source"]:<13} {h["trustStatus"]:<9} managed={h["isManaged"]!s:<5} {h["key"][-58:]}', file=sys.stderr)
state = []
for h in rows:
    if h["isManaged"]: continue                      # cannot be disabled or re-trusted from a flag; a managed hook is the box's policy
    off = "" if h["source"] == "sessionFlags" else "enabled=false,"
    state.append(f'"{h["key"]}"={{{off}trusted_hash="{h["currentHash"]}"}}')
print("-c 'hooks.state={" + ",".join(state) + "}'")
EOF
if want ask; then
echo "== ask: user-layer hooks.json again, the whole hooks.state built from codex's own hooks/list"
userhome "$W/askhome"; mkdir -p "$W/ask"
ASKED=$(python3 -I "$W/ask.py" "$W/askhome" "$W/ask" "hooks=$HOOKS") || exit 1
before=$(shasum -a 256 "$W/askhome/config.toml" | cut -c1-16)
arm ask "$W/ask" "$(codexline "$W/askhome" "$W/ask" "$ASKED" 'dok0a argv probe marker')"
for t in 2 3 5 10 20 20; do pause $t; status "argv, inventory asked"; done
"$REX" capture -s "$S" -b "$B" --trim | grep -q 'hook is new or changed\|hooks are new or changed\|Hooks need review' && echo "  review dialog: YES" || echo "  review dialog: no"
witness; screen 6; done_arm
after=$(shasum -a 256 "$W/askhome/config.toml" | cut -c1-16)
echo "  home config.toml sha256 before $before after $after ($( [ "$before" = "$after" ] && echo unchanged || echo CHANGED))"
fi
echo "== scratch left at $W"
