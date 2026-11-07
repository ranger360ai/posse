#!/bin/sh
# ADR 0076 Amendment 3 shakedown probe (ranger-base-yqmkr) — the box-wide cost
# of `[ui.notifications] program_status = "on"`, and the admission gate read on
# a real box.
#
# Usage: sh docs/adr/0076-grok-shakedown.probe.sh [arm…]   (repo root; no args = all)
#   osc     grok in a HERDR pane with the key on: does it write OSC 7501, and
#           does any byte of it reach the rendered screen? Control arm: the
#           shipped `auto` default, same fixture otherwise.
#   detect  herdr's OWN grok detection over the same two homes: the state it
#           reads and the rule it matched, working and settled.
#   gates   the D12 precondition read through `posse gates` on a scratch
#           `multiplexer: rex` instance, over five fixture grok homes.
#           Read-only: nothing is launched, claimed or created.
#   REX=<path to rex> overrides the CLI (it is not on PATH by default).
#
# WHAT THIS PROBE WILL NOT DO, AND IT IS THE DESIGN (ranger-base-yqmkr).
# Amendment 3's remaining UNPROVEN is how a grok turn ENDS on Rex — `done` or
# `error` — and whether a dispatched grok seat SETTLES on either multiplexer.
# Both need a real model turn on the operator's own grok account, which is
# spend, and spend is never a persona's call (hard risk line 1; the codex
# precedent is ranger-base-x7ojx, where the operator authorized two turns by
# name and by count). So those two arms are NOT here: this probe measures
# everything that can be measured for nothing, and the turn-end arms live on
# the authorization bead. Do not add them here without one.
#
# No model turn is possible in any arm, by three locks (the ranger-base-1g8gf
# rig, as ADR 0076's own grok-hook probe uses it):
#   - every arm runs under its OWN fixture HOME, so its own grok home and its
#     own credential store, never the operator's;
#   - XAI_API_KEY is junk, and every *_PROXY points at 127.0.0.1:9 (nothing
#     listens), so an inference request dies locally before it is a request;
#   - the `gates` arm launches nothing at all — it is one read-only command.
# Telemetry is off by env and by the fixture config; auto_update is off.
#
# The herdr arms run against a SCRATCH herdr server, never the fleet's, and
# the trap kills it. READ THE WARNING in hserver() before changing how that
# server is started: the obvious spelling attaches to the fleet's own state.
set -u
REX=${REX:-"$HOME/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"}
GROK=$(command -v grok) || { echo "grok not on PATH"; exit 2; }
HERDR=$(command -v herdr) || { echo "herdr not on PATH"; exit 2; }
ROOT=$(pwd -P)
[ -x "$ROOT/bin/posse-go" ] || { echo "build it first: make build (this probe reads the WORKTREE's posse, not the promoted one)"; exit 2; }

# SHORT, because a unix socket path is capped at ~104 bytes by sockaddr_un and
# a scratchpad path is already past it (ranger-base-43mxx).
W=/tmp/yq-shake.$$
mkdir -p "$W/ws" || exit 2
HSOCK=$W/h.sock
echo "== $("$GROK" --version)   herdr $("$HERDR" --version 2>/dev/null | head -1)   $("$REX" version 2>/dev/null | head -1)   $(uname -sr)"
echo "== scratch $W"

HPID=
cleanup() {
  [ -n "$HPID" ] && kill "$HPID" 2>/dev/null
  # One pid, the one this script started. Never a `pkill -f herdr` pattern:
  # every seat on this box runs byte-identical herdr argv from a different
  # worktree, and a pattern kill has ended eleven sibling suites here
  # (AGENTS.md "Ending anything"). Both verbs are denied by every crew PID.
  :
}
trap cleanup EXIT INT TERM

pause() { python3 -I -c "import time; time.sleep($1)"; }
h() { env HERDR_SOCKET_PATH="$HSOCK" "$HERDR" "$@"; }

# hserver — a scratch herdr server, isolated from the fleet's.
#
# POINTING ONLY `HERDR_SOCKET_PATH` AT A NEW PATH IS NOT ISOLATION, and it
# looks exactly like it is (MEASURED 2026-11-07, ranger-base-yqmkr). A server
# started that way comes up on the new socket, answers `herdr status` as
# `running`, and serves the DEFAULT session's state — so `workspace list` on
# the "scratch" socket returned the fleet's fifteen live workspaces, every one
# of them a crew seat, and a `workspace close` there would have closed a
# persona's session. The session state is keyed on HOME, not on the socket.
# So: HOME is what forks the state, and the socket only keeps the path short
# enough to bind. `--session <name>` is the documented spelling and is NOT
# usable from a caged seat — its state dir is under the operator's own
# ~/.config/herdr, which the seatbelt refuses (`Operation not permitted`).
hserver() {
  mkdir -p "$W/hhome"
  POSSE_KEEP=ranger-base-yqmkr env HOME="$W/hhome" HERDR_SOCKET_PATH="$HSOCK" \
    "$HERDR" server > "$W/hserver.log" 2>&1 &
  HPID=$!
  pause 5
  n=$(h workspace list 2>/dev/null | python3 -I -c 'import sys,json; print(len(json.load(sys.stdin)["result"]["workspaces"]))' 2>/dev/null)
  [ "${n:-x}" = 0 ] || { echo "REFUSING TO RUN: the scratch herdr server answered with $n workspaces, so it is not a fresh one — see hserver()'s warning"; exit 2; }
  # A WARM-UP PANE, AND IT IS LOAD-BEARING (MEASURED 2026-11-07,
  # ranger-base-yqmkr). The FIRST workspace a fresh herdr server creates gets
  # a pty one column narrower than every later one — 119 against 120 here,
  # measured with `stty size` in three panes and no grok anywhere near it.
  # Without this pane, whichever arm runs first renders every box-drawing row
  # one character short, the two screens differ on every bordered line, and
  # the diff reads exactly like the stray bytes this probe exists to look
  # for. Burn the first pane on nothing, and the A/B is an A/B.
  h workspace create --label yq-warmup --cwd "$W/ws" --no-focus >/dev/null 2>&1
  pause 1
  echo "== scratch herdr server pid $HPID on $HSOCK, 0 workspaces + 1 warm-up"
}

# fixture <name> [program_status] — a private HOME with a quiet grok config.
fixture() {
  FH=$W/$1; mkdir -p "$FH/.grok"
  { printf '[cli]\nauto_update = false\n[features]\ntelemetry = false\n'
    [ -n "${2:-}" ] && printf '[ui.notifications]\nprogram_status = "%s"\n' "$2"
  } > "$FH/.grok/config.toml"
}

# launcher <name> <home> — the line a pane runs. grok takes the prompt as argv,
# which is how posse prompts it (prompt: argv), and the turn dies at the proxy.
launcher() {
  cat > "$W/$1.inner" <<EOF
#!/bin/sh
export HOME='$2' XAI_API_KEY=yq-shake-junk-key
export HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9
export https_proxy=http://127.0.0.1:9 http_proxy=http://127.0.0.1:9 all_proxy=http://127.0.0.1:9 NO_PROXY= no_proxy=
export DISABLE_TELEMETRY=1 GROK_TELEMETRY_ENABLED=0
cd '$W/ws' && exec '$GROK' 'yq shakedown: reply ok'
EOF
  chmod +x "$W/$1.inner"
  # script(1) keeps a copy of every byte grok writes to its terminal, which is
  # the POSITIVE half of the OSC question: "no garbage on screen" over a pane
  # that was never written to proves nothing (a control needs a failing arm).
  cat > "$W/$1.launch" <<EOF
#!/bin/sh
exec /usr/bin/script -q '$W/$1.ts' '$W/$1.inner'
EOF
  chmod +x "$W/$1.launch"
}

pane() { # pane <label> — create a workspace, echo its root pane id
  h workspace create --label "$1" --cwd "$W/ws" --no-focus 2>/dev/null \
    | python3 -I -c 'import sys,json; print(json.load(sys.stdin)["result"]["root_pane"]["pane_id"])'
}

ARMS=${*:-osc detect gates}
want() { case " $ARMS " in *" $1 "*) return 0;; esac; return 1; }

# --- osc: does the key put bytes on a herdr screen? --------------------------
# The claim under test is ADR 0076 Amendment 3's Consequences ASSUMED: "That
# herdr's emulator [consumes an OSC it does not know] unseen ... is ASSUMED."
if want osc; then
  echo "== osc: grok under script(1) in a herdr pane, program_status on vs the auto default"
  hserver
  for arm in on auto; do
    [ "$arm" = on ] && fixture "h$arm" on || fixture "h$arm" ""
    launcher "s$arm" "$W/h$arm"
    P=$(pane "yq-s$arm"); eval "P_$arm=\$P"
    h pane run "$P" "$W/s$arm.launch" >/dev/null 2>&1
  done
  pause 15
  for arm in on auto; do
    eval "P=\$P_$arm"
    printf '  %-5s ' "$arm"
    python3 -I - "$W/s$arm.ts" <<'PY'
import re, sys
b = open(sys.argv[1], 'rb').read()
seqs = re.findall(rb'\x1b\]7501;(.*?)(?:\x1b\\|\x07)', b, re.S)
states = [s.split(b':')[0].decode() for s in seqs]
print(f'grok wrote {len(seqs)} OSC 7501 strings to its terminal ({len(re.findall(rb"7501", b))} raw 7501 occurrences); states: {sorted(set(states)) or "none"}')
PY
    h pane read "$P" --format text --lines 200 > "$W/screen-$arm.txt" 2>&1
    printf '        rendered screen: %s bytes, %s match of 7501/n,auth/grok-build, %s stray control bytes\n' \
      "$(wc -c < "$W/screen-$arm.txt" | tr -d ' ')" \
      "$(grep -c -E '7501|n,auth|grok-build' "$W/screen-$arm.txt")" \
      "$(python3 -I -c 'import sys; b=open(sys.argv[1],"rb").read(); print(len([c for c in b if c<0x09 or 0x0e<=c<=0x1f or c==0x7f]))' "$W/screen-$arm.txt")"
  done
  # THE READING THAT MATTERS: the two screens, compared. Equal means the key
  # cost the pane nothing — which is the whole of the ASSUMED.
  #
  # TWO THINGS ARE NORMALIZED FIRST, and neither is a fudge: grok paints a
  # braille spinner frame and a tenths-of-a-second stopwatch into its own
  # status line, so two screens read a moment apart differ there whatever the
  # config says (MEASURED: with the key off on BOTH arms the same two fields
  # still differ). They are normalized by CLASS and the count is printed, so a
  # stray byte cannot hide inside the normalization: anything that is not a
  # spinner glyph or a timer survives into the diff.
  python3 -I - "$W/screen-on.txt" "$W/screen-auto.txt" <<'PY'
import re, sys
SPIN = re.compile(r'[⠀-⣿]')
TIME = re.compile(r'\b\d+\.\d+s\b')
raw = [open(p, encoding='utf-8', errors='replace').read() for p in sys.argv[1:3]]
norm, counts = [], []
for t in raw:
    n = SPIN.sub('@', t); c1 = len(SPIN.findall(t))
    n = TIME.sub('@s', n); c2 = len(TIME.findall(t))
    norm.append(n); counts.append((c1, c2))
print(f'  bytes: on={len(raw[0])} auto={len(raw[1])}; '
      f'normalized spinner/timer fields: on={counts[0]} auto={counts[1]}')
if norm[0] == norm[1]:
    print('  VERDICT: the rendered screens are IDENTICAL once grok\'s own spinner frame and')
    print('           stopwatch are normalized — the OSC is consumed unseen. ASSUMED -> MEASURED.')
elif raw[0] == raw[1]:
    print('  VERDICT: BYTE-IDENTICAL. The OSC is consumed unseen.')
else:
    print('  VERDICT: the screens DIFFER BEYOND the spinner and the stopwatch — read this before')
    print('           believing anything else (a one-column shift on every bordered row is the')
    print('           first-pane pty width, not the key: see hserver()\'s warm-up pane):')
    import difflib
    for ln in list(difflib.unified_diff(norm[0].splitlines(), norm[1].splitlines(),
                                        'on', 'auto', lineterm=''))[:24]:
        print('    ' + ln)
PY
fi

# --- detect: does herdr still read grok's state? -----------------------------
# script(1) is NOT used here: herdr identifies an agent by the pane's
# foreground argv0, so a wrapper makes the pane `script` and herdr finds no
# agent at all (MEASURED — `agent explain` answers `agent_not_found`). The
# wrapper buys the OSC bytes above and costs the detection reading, so the two
# questions need two panes.
if want detect; then
  echo "== detect: herdr's own grok detection, program_status on vs the auto default"
  [ -n "$HPID" ] || hserver
  for arm in on auto; do
    [ "$arm" = on ] && fixture "d$arm" on || fixture "d$arm" ""
    launcher "p$arm" "$W/d$arm"
    P=$(pane "yq-p$arm"); eval "Q_$arm=\$P"
    h pane run "$P" "$W/p$arm.inner" >/dev/null 2>&1
  done
  pause 12
  # explain <pane> <label> — herdr's reading, through a FILE and not a pipe.
  # `python3 -I - ARGS <<'PY'` cannot also read piped JSON: the heredoc IS
  # stdin, so the program text is what `json.load(sys.stdin)` would get and
  # every arm dies on an empty read. The reading goes to a file; the program
  # comes from the heredoc; the path is argv.
  explain() {
    h agent explain "$1" --json > "$W/explain.json" 2>/dev/null
    python3 -I - "$2" "$W/explain.json" <<'PY'
import sys, json
d = json.load(open(sys.argv[2], encoding='utf-8'))
if 'error' in d:
    print(f'  {sys.argv[1]:<22} ERROR {d["error"].get("code")}'); raise SystemExit
m = d.get('matched_rule') or {}
osc = [(r['id'], r['matched'], r['evidence'].get('region_preview'))
       for r in d.get('evaluated_rules', []) if r.get('region') in ('osc_progress', 'osc_title')]
print(f'  {sys.argv[1]:<22} state={d.get("state")} matched={m.get("id")} manifest={d.get("manifest_version")}')
print(f'  {"":<22} osc regions: {osc[:2]}')
PY
  }
  for arm in on auto; do eval "P=\$Q_$arm"; explain "$P" "$arm/working"; done
  for arm in on auto; do eval "P=\$Q_$arm"; h agent send-keys "$P" ctrl+c >/dev/null 2>&1; done
  pause 6
  for arm in on auto; do eval "P=\$Q_$arm"; explain "$P" "$arm/settled"; done
fi

# --- gates: the D12 precondition on a real box, nothing launched -------------
if want gates; then
  echo "== gates: the D12 read over five fixture grok homes, on a scratch multiplexer: rex instance"
  mkdir -p "$W/rexhome/agents"
  cat > "$W/rexhome/config.yaml" <<EOF
instance: yqshake
multiplexer: rex
default_dir: $W/ws
load_guard: 60
auto_reap: false
EOF
  # A seat whose labels match nothing, so only an explicit assignee could ever
  # route to it (laneFor: an assignee that canon-resolves is a lane of one).
  # This arm routes nothing at all, but the rig stays race-proof by shape.
  cat > "$W/rexhome/agents/grokseat.md" <<'EOF'
---
name: grokseat
description: rig seat — a grok seat on a rex instance, for this probe only
runtime: grok
tier: standard
cage: seatbelt
labels: [yqshakerig]
---
You are a rig seat for a probe. Say the word ok and stop.
EOF
  for a in absent auto off unreadable on; do
    GH=$W/gh-$a; mkdir -p "$GH"
    case $a in
      absent) ;;
      unreadable) printf '[ui.notifications]\nprogram_status = "on"\n' > "$GH/config.toml"; chmod 000 "$GH/config.toml" ;;
      *) printf '[ui.notifications]\nprogram_status = "%s"\n' "$a" > "$GH/config.toml" ;;
    esac
    echo "  --- GROK_HOME names a home whose key is: $a"
    # GROK_HOME is the HOME pin of ADR 0077 D1 — the same value posse types
    # onto the seat it hires, so the file posse reads here is the file that
    # seat's grok would open.
    env -u POSSE_HOME RHQ_HOME="$W/rexhome" GROK_HOME="$GH" RHQ_PERSONA= \
      "$ROOT/bin/posse-go" gates grokseat 2>&1 \
      | grep -E "launches here|REFUSED here by the program_status" | sed 's/^/  /'
  done
  chmod 644 "$W/gh-unreadable/config.toml" 2>/dev/null
fi

echo "== scratch left at $W (typescripts, screens, fixture homes); the herdr server is killed by the trap"
