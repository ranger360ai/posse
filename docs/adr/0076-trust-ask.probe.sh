#!/bin/sh
# ADR 0076 amendment 2 probe, slice ranger-base-fwwl8 — does POSSE'S OWN ask
# do what D7 says: `codex app-server` asked once for `hooks/list` under the
# seat's own line, and ONE `-c hooks.state` built from the answer that trusts
# posse's hooks, switches every other one off, and draws no review dialog?
#
# Usage: sh docs/adr/0076-trust-ask.probe.sh [state] [order] [ask]   (repo root; no args = all, in order)
#        REX=<path to rex> to override the CLI (the `ask` arm only).
#
# This is the acceptance arm the slice's done-when names. It is the `ask` arm
# of docs/adr/0076-flag-trust.probe.sh re-pointed at posse's code: that probe
# answered the DESIGN question with a Python client (ask.py) and this one
# answers the IMPLEMENTATION question with the shipped one. Nothing below
# builds a declaration or a trust line of its own — `posse hook-report
# --declare codex` and `posse hook-report --ask codex` are asked for both, so
# a drift between what the probe measures and what a launch would type is not
# possible.
#
#   state   codex as its own acceptance test, with NO Rex and NO prompt: ask,
#           then ask again with the answer on the line, and read each hook's
#           trustStatus and enabled back out of codex both times. Untrusted
#           before, trusted after, every foreign hook disabled.
#   order   the one thing about the pair that is not obvious and is fatal:
#           codex applies `-c` in ORDER and `hooks={…}` sets the whole table,
#           so a trust value typed BEFORE the declaration is wiped by it. Both
#           orders, same two values, same home.
#   ask     the live arm: a Rex block, posse's declaration, posse's trust, and
#           an untrusted user-layer hooks.json in the home — no review dialog,
#           the user hook silent, a `working` record owned by codex, and the
#           home's config.toml byte-identical before and after.
#
# NO MODEL TURN IS POSSIBLE, the same lock as the sibling probes: every codex
# runs in a scratch CODEX_HOME holding no credential at all, whose only
# provider is a dead loopback port and whose `env_key` names a variable that
# is never set — so codex has nothing to send and nowhere to send it. The
# `state` and `order` arms submit no prompt at all; they are `initialize` plus
# `hooks/list`.
#
# WHERE THE SANDBOX GOES, AND WHERE IT CANNOT. Nested seatbelt is refused
# inside a caged seat — MEASURED 2026-11-06 from a dispatched `cage: seatbelt`
# seat: `sandbox-exec: sandbox_apply: Operation not permitted`, whatever the
# profile. The codex that takes a PROMPT is launched inside a Rex block, which
# Rex spawns outside this seat's cage, and there the same sandbox-exec runs
# clean. The app-servers posse itself starts are NOT wrapped: they submit
# nothing, and a wrapper that cannot apply would stop the arm rather than
# protect it.
set -u
W=$(cd "$(mktemp -d "${TMPDIR:-/tmp}/fwwl8.XXXXXX")" && pwd -P)   # codex keys project trust by the resolved path
NONET="sandbox-exec -p '(version 1)(allow default)(deny network*)'"

# The binary under test, built here rather than taken from PATH: PATH's
# `posse` in a crew seat is a gate shim, and a shim is exactly what the
# declaration must NOT name (ADR 0077 D3).
POSSE=$W/posse
go build -o "$POSSE" ./cmd/posse || exit 2
echo "== posse $("$POSSE" version 2>/dev/null | head -1)   $(codex --version 2>/dev/null | tail -1)"
echo "== scratch $W"

ARMS=${*:-state order ask}
want() { case " $ARMS " in *" $1 "*) return 0;; esac; return 1; }
pause() { python3 -I -c "import time; time.sleep($1)"; }
shq() { printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"; }

codexhome() { # codexhome <dir> — dead provider on a closed loopback port, update menu off,
  mkdir -p "$1"            # plus an UNTRUSTED user-layer hooks.json: the hook D7 has to switch off
  cat > "$1/config.toml" <<'EOF'
check_for_update_on_startup = false
model = "probe"
model_provider = "dead"
[model_providers.dead]
name = "dead"
base_url = "http://127.0.0.1:9/v1"
env_key = "FWWL8_DEAD_KEY_NEVER_SET"
wire_api = "responses"
EOF
  printf '{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"%s/userhook.sh %s","timeout":3}]}]}}\n' "$W" "$W" > "$1/hooks.json"
}
cat > "$W/userhook.sh" <<'EOF'
#!/bin/sh
echo "USER-LAYER HOOK RAN t=$(date +%s)" >> "$1/witness"; exit 0
EOF
chmod +x "$W/userhook.sh"

# inventory <home> <cwd> [extra -c values…] — what codex says about each hook
# that would load, read through posse's own ask. The ask prints the inventory
# on stderr and the `-c hooks.state` value on stdout, so this shows one and
# the caller keeps the other.
inventory() {
  h=$1 c=$2; shift 2
  args=""; for v in "$@"; do args="$args -c $(shq "$v")"; done
  eval "(cd $(shq "$c") && CODEX_HOME=$(shq "$h") $(shq "$POSSE") hook-report --ask codex$args)"
}

# ─── state: codex as its own acceptance test, no Rex and no prompt ───────────
if want state; then
echo "== state: ask, then ask again with the answer on the line"
H=$W/statehome; C=$W/state; codexhome "$H"; mkdir -p "$C"
TRUST="projects={\"$C\"={trust_level=\"trusted\"}}"
before=$(shasum -a 256 "$H/config.toml" | cut -c1-16)
echo "  -- round 1: the ask, with no trust on the line"
ask1() { inventory "$H" "$C" "$TRUST" > "$W/state.out" 2> "$W/state.err"; }
ask1 || { sed 's/^/    /' "$W/state.err"; echo "  posse REFUSED the ask — the sentence above is the deliverable"; exit 1; }
sed 's/^/    /' "$W/state.err"
STATE=$(cat "$W/state.out")
echo "  trust line: ${#STATE} bytes"
echo "  -- round 2: the SAME ask, with that trust on the line"
inventory "$H" "$C" "$TRUST" "$STATE" > /dev/null 2> "$W/state2.err" || { sed 's/^/    /' "$W/state2.err"; exit 1; }
sed 's/^/    /' "$W/state2.err"
after=$(shasum -a 256 "$H/config.toml" | cut -c1-16)
echo "  home config.toml sha256 before $before after $after ($( [ "$before" = "$after" ] && echo unchanged || echo CHANGED))"
echo "  (want: every hook untrusted in round 1 and trusted in round 2; the user-layer hook enabled=false — read it in the dialog-free \`ask\` arm, where \`enabled\` is what stops it running)"
fi

# ─── order: the declaration must be typed BEFORE the trust ───────────────────
# posse's ask puts the declaration first for this reason, so the only way to
# measure the wrong order is to hand codex the two values by hand. The values
# are still posse's; only their order is the probe's.
if want order; then
echo "== order: the same two values, both ways round"
H=$W/orderhome; C=$W/order; codexhome "$H"; mkdir -p "$C"
TRUST="projects={\"$C\"={trust_level=\"trusted\"}}"
DECL=$("$POSSE" hook-report --declare codex) || exit 1
STATE=$(inventory "$H" "$C" "$TRUST" 2>/dev/null) || exit 1
cat > "$W/order.py" <<'EOF'
import json, subprocess, sys, time, select
home, cwd = sys.argv[1], sys.argv[2]
argv = ["env", f"CODEX_HOME={home}", "codex", "app-server"] + [a for v in sys.argv[3:] for a in ("-c", v)]
p = subprocess.Popen(argv, cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
def send(o): p.stdin.write(json.dumps(o) + "\n"); p.stdin.flush()
def recv(i, limit=20):
    end = time.time() + limit
    while time.time() < end:
        if select.select([p.stdout], [], [], 0.5)[0]:
            line = p.stdout.readline()
            if not line: return None
            if json.loads(line).get("id") == i: return json.loads(line)
send({"id": 1, "method": "initialize", "params": {"clientInfo": {"name": "posse-probe", "version": "0"}}}); recv(1)
send({"method": "initialized"})
send({"id": 2, "method": "hooks/list", "params": {"cwds": [cwd]}}); r = recv(2)
p.terminate(); p.wait(5)
if not r or "result" not in r: sys.exit(f"hooks/list failed: {r}")
for h in (h for inv in r["result"]["data"] for h in inv.get("hooks", [])):
    print(f'    {h["source"]:<13} {h["trustStatus"]:<9} enabled={h["enabled"]!s:<5} {h["key"][-50:]}')
EOF
echo "  -- declaration, then trust (what posse types)"
python3 -I "$W/order.py" "$H" "$C" "$TRUST" "$DECL" "$STATE" || exit 1
echo "  -- trust, then declaration (the wrong order)"
python3 -I "$W/order.py" "$H" "$C" "$TRUST" "$STATE" "$DECL" || exit 1
echo "  (want: all trusted in the first, all untrusted in the second — a line holding every byte of the right answer)"
fi

# ─── ask: the live arm, a record owned by codex and no review dialog ─────────
if want ask; then
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
[ -x "$REX" ] || { echo "== ask: rex not found at \$REX — skipping the live arm"; echo "== scratch left at $W"; exit 0; }
echo "== ask: posse's declaration and posse's trust, in a Rex block"
echo "   rex $("$REX" version | head -1)"
PATHS="PATH=$PATH"
arm() { # arm <name> <cwd> <command>
  S=fwwl8-$1-$$
  printf '#!/bin/sh\nexport %s\ncd %s && exec %s\n' "'$PATHS'" "'$2'" "$3" > "$W/$1.launch"; chmod +x "$W/$1.launch"
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  T0=$(date +%s)
  "$REX" send -s "$S" -b "$B" "$W/$1.launch"; "$REX" send-key -s "$S" -b "$B" enter
}
status() { printf '  t+%-3s %-28s ' "$(( $(date +%s) - T0 ))" "$1"; "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}"; echo; }
screen() { "$REX" capture -s "$S" -b "$B" --trim | grep -v '^[[:space:]]*$' | tail -${1:-6} | sed 's/^/  | /'; }

H=$W/askhome; C=$W/ask; codexhome "$H"; mkdir -p "$C"; : > "$W/witness"
TRUST="projects={\"$C\"={trust_level=\"trusted\"}}"
DECL=$("$POSSE" hook-report --declare codex) || exit 1
echo "  declaration (${#DECL} bytes): $(printf '%s' "$DECL" | cut -c1-100)…"
inventory "$H" "$C" "$TRUST" > "$W/ask.state" 2> "$W/ask.err" || { sed 's/^/    /' "$W/ask.err"; echo "  posse REFUSED the ask"; exit 1; }
sed 's/^/    /' "$W/ask.err"
STATE=$(cat "$W/ask.state")
before=$(shasum -a 256 "$H/config.toml" | cut -c1-16)
# The declaration leads the trust on the line, per the `order` arm.
LINE="$NONET env CODEX_HOME=$(shq "$H") $(shq "$(command -v codex)") -c $(shq "$TRUST") -c $(shq "$DECL") -c $(shq "$STATE") $(shq 'fwwl8 argv probe marker')"
arm ask "$C" "$LINE"
for t in 2 3 5 10 20 20; do pause $t; status "posse's ask on the line"; done
"$REX" capture -s "$S" -b "$B" --trim | grep -q 'hook is new or changed\|hooks are new or changed\|Hooks need review' && echo "  review dialog: YES" || echo "  review dialog: no"
printf "  user-layer hook: "; [ -s "$W/witness" ] && sed 's/^/    /' "$W/witness" || echo "SILENT (never ran)"
screen 8
"$REX" kill "$S" >/dev/null 2>&1
after=$(shasum -a 256 "$H/config.toml" | cut -c1-16)
echo "  home config.toml sha256 before $before after $after ($( [ "$before" = "$after" ] && echo unchanged || echo CHANGED))"
echo "  (want: no review dialog, the user-layer hook SILENT, a working record with app=codex and owner={pid,codex}, the home unchanged)"
fi
echo "== scratch left at $W"
