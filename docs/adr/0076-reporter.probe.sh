#!/bin/sh
# ADR 0076 amendment 2 probe, slice ranger-base-z7kle — does POSSE'S OWN
# reporter and POSSE'S OWN declaration do what D6 says: one OSC 7501 report
# per codex hook event, landing as a record Rex keeps, owned by codex?
#
# Usage: sh docs/adr/0076-reporter.probe.sh [bytes] [codex] [space]   (repo root; no args = all, in order)
#        REX=<path to rex> to override the CLI.
#
# This is the acceptance arm the slice's done-when names. It is deliberately
# the sibling of docs/adr/0076-flag-trust.probe.sh and not an edit of it: that
# probe answered the DESIGN question (can trust ride the line at all) with a
# throwaway hook.sh, and this one answers the IMPLEMENTATION question (does
# posse's binary, run as that hook, report) with the shipped renderer. Nothing
# below writes a declaration of its own — `posse hook-report --declare codex`
# is asked for it, so a drift between what the probe measures and what a
# launch would type is not possible.
#
#   bytes   posse's reporter alone, on a real tty, no codex: every event D6's
#           table names piped in as hook JSON, and the `program_status` record
#           read back after each. The records this arm writes carry NO owner —
#           MEASURED 2026-11-06: the reporter is a short-lived child in a
#           pipeline the interactive shell has already finished waiting on, so
#           Rex has no foreground program to stamp. An unowned record is one
#           ADR 0076 §3 refuses for a seat, and rightly; this arm is about the
#           BYTES becoming the record, and the `codex` arm is the one that
#           answers the owner.
#   codex   the flag-trust `ask` arm with posse's reporter as the handler:
#           posse's declaration, trust built from codex's own `hooks/list`,
#           and a `working` record owned by codex after an argv prompt.
#   space   the one thing the renderer refuses rather than quotes — a handler
#           path holding a space. Does codex's split read it, read a
#           single-quoted one, or neither? The refusal in codexHookCommand
#           cites this arm as its re-measurement.
#
# NO MODEL TURN IS POSSIBLE, and the lock is three things: every codex that
# could make one runs inside `sandbox-exec (deny network*)`, in a scratch
# CODEX_HOME holding no credential at all, whose only provider is a dead
# loopback port and whose `env_key` names an environment variable that is NOT
# set — so codex has nothing to send and nowhere to send it. The sibling
# probe's fourth lock, a UserPromptSubmit hook that exits 2, is unavailable
# here on purpose: posse's reporter never exits 2 (that is the thing under
# test), and the `codex` arm needs the prompt submitted to see `working`.
#
# WHERE THE SANDBOX GOES, AND WHERE IT CANNOT. Nested seatbelt is refused
# inside a caged seat — MEASURED 2026-11-06 from a `cage: seatbelt` crew seat:
# `sandbox-exec: sandbox_apply: Operation not permitted`, whatever the
# profile. Every codex that takes a PROMPT is launched inside a Rex block,
# which Rex spawns outside this seat's cage, and there the same sandbox-exec
# runs clean (MEASURED, same seat, same day). `ask.py` is the one codex this
# script runs as its own child, and it is NOT wrapped: an `initialize` +
# `hooks/list` pair submits nothing, and a wrapper that cannot apply would
# stop the arm rather than protect it. Its lock is the scratch home with no
# credential in it.
set -u
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
[ -x "$REX" ] || { echo "rex not found at \$REX"; exit 2; }
PATHS="PATH=$PATH"
W=$(cd "$(mktemp -d "${TMPDIR:-/tmp}/z7kle.XXXXXX")" && pwd -P)   # codex keys project trust by the resolved path
NONET="sandbox-exec -p '(version 1)(allow default)(deny network*)'"

# The binary under test, built here rather than taken from PATH: PATH's
# `posse` in a crew seat is a gate shim, and a shim is exactly what the
# declaration must NOT name (ADR 0077 D3).
POSSE=$W/posse
go build -o "$POSSE" ./cmd/posse || exit 2
echo "== rex $("$REX" version | head -1)"
echo "== posse $("$POSSE" version 2>/dev/null | head -1)"
echo "== scratch $W"

ARMS=${*:-bytes codex space}
want() { case " $ARMS " in *" $1 "*) return 0;; esac; return 1; }
pause() { python3 -I -c "import time; time.sleep($1)"; }
arm() { # arm <name> <cwd> <command>
  S=z7kle-$1-$$
  printf '#!/bin/sh\nexport %s\ncd %s && exec %s\n' "'$PATHS'" "'$2'" "$3" > "$W/$1.launch"; chmod +x "$W/$1.launch"
  B=$("$REX" new "$S" --keep-open --json -- /bin/sh | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["initial_windows"][0]["block_ids"][0])') || exit 2
  T0=$(date +%s)
  "$REX" send -s "$S" -b "$B" "$W/$1.launch"; "$REX" send-key -s "$S" -b "$B" enter
}
record() { "$REX" api call -s "$S" block.method "{\"block_id\":\"$B\",\"method\":\"program_status\"}"; }
status() { printf '  t+%-3s %-28s ' "$(( $(date +%s) - T0 ))" "$1"; record; echo; }
screen() { "$REX" capture -s "$S" -b "$B" --trim | grep -v '^[[:space:]]*$' | tail -${1:-6} | sed 's/^/  | /'; }
done_arm() { "$REX" kill "$S" >/dev/null 2>&1; }

# ─── bytes: the reporter alone, on a real tty ────────────────────────────────
# Each event is one `posse hook-report codex` run with its hook JSON on stdin,
# exactly as codex invokes it, and the record is read after each. `clear` is
# checked for what it DOES (it removes the record) and not for a state value,
# which is the one row of D6's table the protocol's own five states do not name.
if want bytes; then
echo "== bytes: posse hook-report on a real tty, every event of D6's table"
mkdir -p "$W/bytes"
arm bytes "$W/bytes" "/bin/sh -i"
pause 2
for ev in SessionStart UserPromptSubmit PreToolUse PostToolUse PermissionRequest Stop Interrupt SessionEnd NoSuchEvent; do
  "$REX" send -s "$S" -b "$B" "printf '{\"hook_event_name\":\"$ev\"}' | '$POSSE' hook-report codex; echo \"rc=\$?\""
  "$REX" send-key -s "$S" -b "$B" enter
  pause 1.5
  printf '  %-18s rc/record: ' "$ev"
  "$REX" capture -s "$S" -b "$B" --trim | grep -o 'rc=[0-9]*' | tail -1 | tr -d '\n'
  printf ' '; record; echo
done
echo "  (NoSuchEvent must leave the record SessionEnd cleared — i.e. no records)"
screen 4; done_arm
fi

# ─── the codex machinery, shared by the `codex` and `space` arms ─────────────
codexhome() { # codexhome <dir> — dead provider on a closed loopback port, update menu off, NO hooks in the home
  mkdir -p "$1"
  cat > "$1/config.toml" <<'EOF'
check_for_update_on_startup = false
model = "probe"
model_provider = "dead"
[model_providers.dead]
name = "dead"
base_url = "http://127.0.0.1:9/v1"
env_key = "Z7KLE_DEAD_KEY_NEVER_SET"
wire_api = "responses"
EOF
}
# shq quotes one word for the launch script's own shell. It is here rather
# than the sibling probe's inline `'$3'` because the `space` arm deliberately
# feeds a declaration that CONTAINS single quotes, and the naive spelling ate
# the line before codex saw it (the first run of this arm measured nothing but
# `Usage: codex`).
shq() { printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"; }
codexline() { # codexline <home> <cwd> <hooks decl> <extra -c args> [prompt]
  printf '%s env CODEX_HOME=%s %s -c %s -c %s %s %s\n' \
    "$NONET" "$(shq "$1")" "$(shq "$(command -v codex)")" \
    "$(shq "projects={\"$2\"={trust_level=\"trusted\"}}")" "$(shq "$3")" \
    "$4" "${5:+$(shq "$5")}"
}

# ask.py: D7's route, run here as the probe's scaffolding and NOT as posse's
# code — building `hooks.state` is ranger-base-fwwl8's deliverable, and what
# this arm needs it for is only to get past the startup review so the handler
# runs at all. `codex app-server` is asked `hooks/list` under the SAME -c line
# the seat will get; posse's own hooks are typed {trusted_hash}, every other
# source's {enabled=false, trusted_hash}. No hash is modelled.
cat > "$W/ask.py" <<'EOF'
import json, subprocess, sys, time, select
home, cwd, hooks = sys.argv[1], sys.argv[2], sys.argv[3]
# Unwrapped, per the header: nested seatbelt is refused inside a caged seat,
# and this codex submits no prompt — `initialize` then `hooks/list`, against a
# scratch CODEX_HOME that holds no credential.
argv = ["env", f"CODEX_HOME={home}",
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
print(f"  hooks/list answered in {time.time()-t0:.1f}s, {len(rows)} hooks:", file=sys.stderr)
for h in rows: print(f'    {h["source"]:<13} {h["trustStatus"]:<9} managed={h["isManaged"]!s:<5} {h["key"][-58:]}', file=sys.stderr)
state = []
for h in rows:
    if h["isManaged"]: continue                      # D7 refuses the launch over one of these; the probe only needs to not trust it
    off = "" if h["source"] == "sessionFlags" else "enabled=false,"
    state.append(f'"{h["key"]}"={{{off}trusted_hash="{h["currentHash"]}"}}')
print("-c 'hooks.state={" + ",".join(state) + "}'")
EOF

# ─── codex: posse's declaration, posse's binary, a record owned by codex ─────
if want codex; then
echo "== codex: posse's own declaration, with posse's binary as the handler"
DECL=$("$POSSE" hook-report --declare codex) || { echo "  posse refused to render the declaration (see codexHookCommand)"; exit 1; }
echo "  declaration (${#DECL} bytes): $(printf '%s' "$DECL" | cut -c1-120)…"
codexhome "$W/codexhome"; mkdir -p "$W/codex"
before=$(shasum -a 256 "$W/codexhome/config.toml" | cut -c1-16)
ASKED=$(python3 -I "$W/ask.py" "$W/codexhome" "$W/codex" "$DECL") || exit 1
arm codex "$W/codex" "$(codexline "$W/codexhome" "$W/codex" "$DECL" "$ASKED" 'z7kle argv probe marker')"
for t in 2 3 5 10 20 20; do pause $t; status "posse reporter on the line"; done
"$REX" capture -s "$S" -b "$B" --trim | grep -q 'hook is new or changed\|hooks are new or changed\|Hooks need review' && echo "  review dialog: YES" || echo "  review dialog: no"
screen 8; done_arm
after=$(shasum -a 256 "$W/codexhome/config.toml" | cut -c1-16)
echo "  home config.toml sha256 before $before after $after ($( [ "$before" = "$after" ] && echo unchanged || echo CHANGED))"
fi

# ─── space: what codex's own split does with a handler path holding a space ──
# posse's renderer REFUSES such a path rather than guessing at the quoting
# (codexHookCommand). This arm is the measurement that refusal cites, so the
# declaration here is written by the probe and not by posse — the renderer
# would not produce either spelling.
if want space; then
echo "== space: a handler path holding a space, unquoted and single-quoted"
SPACEDIR="$W/a dir"; mkdir -p "$SPACEDIR"
cat > "$SPACEDIR/hook.sh" <<'EOF'
#!/bin/sh
echo "SPACE-HOOK RAN t=$(date +%s) argv0=$0" >> "$1/witness"
[ "$(python3 -I -c 'import sys,json; print(json.load(sys.stdin).get("hook_event_name",""))')" = UserPromptSubmit ] && exit 2
exit 0
EOF
chmod +x "$SPACEDIR/hook.sh"
spacedecl() { # spacedecl <the command string, already as codex would read it>
  python3 -I - "$1" <<'EOF'
import sys
cmd = sys.argv[1].replace("\\", "\\\\").replace('"', '\\"')
evs = ["SessionStart", "UserPromptSubmit"]
print("hooks={" + ",".join(f'{e}=[{{hooks=[{{type="command",command="{cmd}",timeout=3}}]}}]' for e in evs) + "}")
EOF
}
for spelling in unquoted single-quoted; do
  case $spelling in
    unquoted)     CMD="$SPACEDIR/hook.sh $W" ;;
    single-quoted) CMD="'$SPACEDIR/hook.sh' '$W'" ;;
  esac
  echo "  -- $spelling: command=$CMD"
  : > "$W/witness"
  DECL=$(spacedecl "$CMD")
  H="$W/spacehome-$spelling"; C="$W/space-$spelling"
  codexhome "$H"; mkdir -p "$C"
  ASKED=$(python3 -I "$W/ask.py" "$H" "$C" "$DECL") || { echo "    hooks/list failed — codex did not accept this declaration"; continue; }
  arm "space-$spelling" "$C" "$(codexline "$H" "$C" "$DECL" "$ASKED" 'z7kle argv probe marker')"
  for t in 3 5 10; do pause $t; status "$spelling"; done
  echo -n "    witness: "; [ -s "$W/witness" ] && sed 's/^/      /' "$W/witness" || echo "(the hook never ran)"
  screen 4; done_arm
done
fi
echo "== scratch left at $W"
