#!/usr/bin/env bash
# Replays the six real Bob 2.0.5 captures in etc/herdr/agent-detection/upstream/bob
# through the herdr-bob plugin's OWN match_rules, and asserts which rule each
# one lands on -- for upstream's shipped rules.json and for posse's override,
# etc/herdr/herdr-bob/rules.json (ranger-base-0sa5a).
#
# scripts/verify-detection.sh cannot see this file: it globs
# etc/herdr/agent-detection/*.toml at one level, and rules.json is neither a
# .toml nor at that level. This is that arm, for herdr-bob's own rule format.
#
# THE CONTROL ARM IS THE POINT (the ranger-base-26y03 lesson). Upstream's file
# must first fail in both measured ways -- no rule on any of the three blocked
# screens, AND trust_folder_prompt on the idle command picker -- or this
# harness is not reading what it thinks it is reading and says so.
#
# Nothing here touches the live herdr server, the live plugin state, the live
# plugin config or a real Bob. The only live thing it reads is the installed
# checkout, read-only, for match_rules itself and for the fork point.
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIX="$REPO/etc/herdr/agent-detection/upstream/bob"
OVERRIDE="$REPO/etc/herdr/herdr-bob/rules.json"
PINNED_SHA=e53bd475dbe362c8a9195b582b88ef2c0857590b
SRC="${HERDR_BOB_ROOT:-$HOME/.config/herdr/plugins/github/mloeper.herdr-bob-de923c8f068d}"

# bob-watch's own default: `pane read --source recent-unwrapped --lines 40`.
# Every verdict is asserted twice, over the whole capture and over that window,
# and must agree -- a rule that only works because of a line far up the
# scrollback is a rule that stops working when the pane scrolls.
WINDOW=40

fail_count=0
skip() { echo "SKIP: $*"; exit 0; }
ok()   { echo "  ok   $*"; }
bad()  { echo "  FAIL $*"; fail_count=$(( fail_count + 1 )); }

[ -e "$OVERRIDE" ] || { echo "FAIL: no override at $OVERRIDE" >&2; exit 1; }
[ -d "$FIX" ]      || { echo "FAIL: no fixtures at $FIX" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || skip "jq is not on PATH"
[ -d "$SRC/bin" ] || skip "herdr-bob is not installed at $SRC"
have_sha="$(git -C "$SRC" rev-parse HEAD 2>/dev/null || echo unknown)"
[ "$have_sha" = "$PINNED_SHA" ] ||
  skip "herdr-bob is at $have_sha, match_rules was read from $PINNED_SHA -- re-check the extraction"

jq -e . "$OVERRIDE" >/dev/null 2>&1 &&
  ok "the override is valid JSON ($(jq -r '.rules | length' "$OVERRIDE") rules)" ||
  bad "the override is not valid JSON"

T="$(mktemp -d "${TMPDIR:-/tmp}/verify-herdr-bob-rules.XXXXXX")" || exit 1
WATCH_PIDS=""
cleanup() {
  local p
  for p in $WATCH_PIDS; do kill "$p" 2>/dev/null || true; done
  rm -rf "$T"
}
trap cleanup EXIT INT TERM

# ------------------------------------------------- the plugin's own match_rules
# Lifted verbatim from the installed bin/bob-watch rather than reimplemented: a
# second implementation of an eight-line matcher is a second thing to keep in
# sync by hand, and it would agree with itself while disagreeing with the
# plugin. The extraction is pinned below, so a drift in the function's shape is
# a failure here and not a silently narrower check.
#
# Driving the real daemon instead was the other option and is strictly worse
# for this question: on this box bash 3.2 kills bob-watch at its first screen
# read until ranger-base-mz8ud's patch is applied, so a daemon arm could not
# replay a screen at all without that patch, and it would cost one poll
# interval per assertion and read its verdict out of a log line.
awk '/^match_rules\(\) \{/,/^\}$/' "$SRC/bin/bob-watch" > "$T/match_rules.sh"
n_fn=$(grep -c '^match_rules() {' "$T/match_rules.sh")
if [ "$n_fn" = 1 ] &&
   grep -q 'grep -qiE -- "\$pat"' "$T/match_rules.sh" &&
   grep -q '\.rules\[\$i\]\.all' "$T/match_rules.sh" &&
   grep -q '\.rules\[\$i\]\.any' "$T/match_rules.sh" &&
   grep -q '\.rules\[\$i\]\.none' "$T/match_rules.sh" &&
   [ "$(tail -1 "$T/match_rules.sh")" = "}" ]; then
  ok "match_rules extracted from the installed bob-watch ($(wc -l < "$T/match_rules.sh" | tr -d ' ') lines, all/any/none, grep -qiE)"
else
  bad "match_rules extraction looks wrong -- $n_fn definitions, $(wc -l < "$T/match_rules.sh" | tr -d ' ') lines"
  echo "verify-herdr-bob-rules: the extraction is the harness; refusing to report verdicts from it" >&2
  exit 1
fi

# One replay: rules-file, fixture, lines-or-all -> rule id, or `-` for no match.
cat > "$T/replay.sh" <<'REPLAY'
#!/usr/bin/env bash
set -uo pipefail
. "$MR"
RULES="$1"
if [ "$3" = all ]; then screen="$(cat "$2")"; else screen="$(tail -n "$3" "$2")"; fi
match_rules "$screen" || printf '%s' -
REPLAY
chmod +x "$T/replay.sh"

verdict() { # rules-file, fixture, window
  env MR="$T/match_rules.sh" "$T/replay.sh" "$1" "$2" "$3"
}

# ------------------------------------------------------------- the expectation
# fixture <TAB> upstream's verdict <TAB> the override's verdict
EXPECT="blocked-execute-command.txt	-	bob_execute_command
blocked-execute-command-scrollback.txt	-	bob_execute_command
blocked-signin.txt	-	bob_signin
idle-composer.txt	-	-
idle-after-turn.txt	-	-
idle-command-picker.txt	trust_folder_prompt	-"

arm() { # label, rules-file, which-column
  local label=$1 rules=$2 col=$3 name want got_full got_win
  echo "$label"
  printf '%s\n' "$EXPECT" | while IFS=$'\t' read -r name up ov; do
    case $col in up) want=$up ;; *) want=$ov ;; esac
    [ -e "$FIX/$name" ] || { echo "  FAIL $name: no such fixture"; continue; }
    got_full="$(verdict "$rules" "$FIX/$name" all)"
    got_win="$(verdict "$rules" "$FIX/$name" "$WINDOW")"
    if [ "$got_full" = "$want" ] && [ "$got_win" = "$want" ]; then
      case $want in
        -) echo "  ok   $name -> no rule" ;;
        *) echo "  ok   $name -> $want" ;;
      esac
    elif [ "$got_full" != "$got_win" ]; then
      echo "  FAIL $name: whole capture -> ${got_full}, last $WINDOW lines -> ${got_win} (want $want, and the two must agree)"
    else
      echo "  FAIL $name: got ${got_full}, want ${want}"
    fi
  done
}

# A `while read` in a pipeline is a subshell, so fail_count cannot come back
# out of arm(). Count the FAILs it printed instead.
run_arm() { # label, rules-file, column
  local out; out="$(arm "$1" "$2" "$3")"
  printf '%s\n' "$out"
  local n; n=$(printf '%s\n' "$out" | grep -c '^  FAIL ')
  fail_count=$(( fail_count + ${n:-0} ))
}

# ----------------------------------------------------------------- control arm
run_arm "control (upstream's shipped rules.json, $PINNED_SHA):" "$SRC/rules.json" up
if [ "$fail_count" -gt 1 ]; then
  echo "verify-herdr-bob-rules: the control arm did not reproduce upstream's two failures." >&2
  echo "  Until it does, a green override arm would prove nothing (ranger-base-26y03)." >&2
  exit 1
fi

# ---------------------------------------------------------------- override arm
run_arm "override (etc/herdr/herdr-bob/rules.json):" "$OVERRIDE" ov

# Belt to that braces: no rule in the override may fire on an idle capture,
# stated as its own assertion rather than inferred from the matrix above,
# because this is the one that costs a dispatched seat its wait.
idle_hit=0
for name in idle-composer.txt idle-after-turn.txt idle-command-picker.txt; do
  [ "$(verdict "$OVERRIDE" "$FIX/$name" all)" = - ] || idle_hit=1
  [ "$(verdict "$OVERRIDE" "$FIX/$name" "$WINDOW")" = - ] || idle_hit=1
done
[ "$idle_hit" = 0 ] &&
  ok "no override rule reports blocked on any of the three idle captures" ||
  bad "an override rule reports blocked on an idle capture -- a false blocked stops waits"

# ----------------------------------------------------------- grep x locale arm
# The watcher does not choose which grep or which locale it gets: the herdr
# server's environment does, and a daemon started from launchd has no LANG.
# MEASURED 2026-10-03: a multibyte character in a BRACKET EXPRESSION is read
# byte-wise by BSD grep under LC_ALL=C, so `[>❯→]` matches a box-drawing line;
# alternation does not. Hence no bracket expression in the override holds one,
# and hence this arm.
if [ -x /usr/bin/grep ]; then
  mkdir -p "$T/bsdbin"; ln -sf /usr/bin/grep "$T/bsdbin/grep"
  echo "grep x locale (verdicts must not move):"
  jq -e -r '[.rules[] | (.all // []) + (.any // []) + (.none // []) | .[]]
            | map(select(test("\\[[^]]*[^\\x00-\\x7f]")))
            | if length == 0 then "none" else (. | join(", ")) end' "$OVERRIDE" \
    | { read -r multi
        [ "$multi" = none ] &&
          ok "no bracket expression in the override contains a multibyte character" ||
          bad "bracket expression with a multibyte character: $multi"; }
  for grepdir in "" "$T/bsdbin"; do
    for loc in en_US.UTF-8 C; do
      lbl="$([ -n "$grepdir" ] && echo 'BSD grep' || echo "$(grep --version 2>&1 | head -1 | cut -d' ' -f1)") LC_ALL=$loc"
      moved=0
      printf '%s\n' "$EXPECT" | while IFS=$'\t' read -r name up ov; do
        g="$(env PATH="${grepdir:+$grepdir:}$PATH" LC_ALL="$loc" MR="$T/match_rules.sh" \
             "$T/replay.sh" "$OVERRIDE" "$FIX/$name" "$WINDOW")"
        [ "$g" = "$ov" ] || echo "MOVED $name: $g != $ov"
      done > "$T/moved"
      if [ -s "$T/moved" ]; then
        bad "$lbl: $(tr '\n' ';' < "$T/moved")"
      else
        ok "$lbl: all six verdicts unchanged"
      fi
    done
  done
else
  echo "grep x locale: skipped, no /usr/bin/grep to compare against"
fi

# ------------------------------------------------------- the override is READ
# Two properties of bob-watch's seeding, end to end, against the real script:
# a fresh config dir is seeded with the PLUGIN's copy (so a clean install gets
# the broken file, which is why there is an install step at all), and an
# existing copy is preferred and never refreshed (so the override is durable).
#
# Safe on bash 3.2 without ranger-base-mz8ud's patch: with no registered pane
# the loop never reads a screen, which is the read that kills it. Started
# directly rather than through bob-watchctl, whose start arm is bug 1.
mkdir -p "$T/bin" "$T/state/panes" "$T/state/heartbeat" "$T/seed" "$T/kept"
cat > "$T/bin/herdr" <<'FAKE'
#!/usr/bin/env bash
set -u
case "${1:-} ${2:-}" in
  "pane list") printf '{"result":{"panes":[]}}\n' ;;
esac
exit 0
FAKE
chmod +x "$T/bin/herdr"

watch_once() { # config-dir, log-tag
  local cfg=$1 tag=$2 i=0
  rm -rf "$T/state-$tag"; mkdir -p "$T/state-$tag/panes" "$T/state-$tag/heartbeat"
  env PATH="$T/bin:$PATH" HERDR_BIN_PATH="$T/bin/herdr" \
      HERDR_BOB_STATE_DIR="$T/state-$tag" HERDR_PLUGIN_STATE_DIR="$T/state-$tag" \
      HERDR_PLUGIN_CONFIG_DIR="$cfg" BOB_WATCH_INTERVAL=1 \
      /bin/bash "$SRC/bin/bob-watch" >/dev/null 2>&1 &
  local p=$!
  WATCH_PIDS="$WATCH_PIDS $p"
  while [ "$i" -lt 10 ]; do
    grep -q 'watch started' "$T/state-$tag/herdr-bob.log" 2>/dev/null && break
    sleep 1; i=$(( i + 1 ))
  done
  kill "$p" 2>/dev/null || true
  wait "$p" 2>/dev/null || true
  WATCH_PIDS="$(printf '%s' "$WATCH_PIDS" | sed "s/ $p\$//; s/ $p / /")"
  sed -n 's/.*watch started (.*rules \(.*\))$/\1/p' "$T/state-$tag/herdr-bob.log" 2>/dev/null | tail -1
}

echo "seeding (the real bob-watch, fake herdr, scratch state and config):"
got="$(watch_once "$T/seed" seed)"
if [ "$got" = "$T/seed/rules.json" ] && cmp -s "$T/seed/rules.json" "$SRC/rules.json"; then
  ok "a fresh config dir is seeded from the plugin's own copy, and read from there"
else
  bad "fresh seeding: watcher read '${got:-nothing}', seeded file $(cmp -s "$T/seed/rules.json" "$SRC/rules.json" && echo matches || echo 'does NOT match') the plugin's"
fi

cp "$OVERRIDE" "$T/kept/rules.json"
got="$(watch_once "$T/kept" kept)"
if [ "$got" = "$T/kept/rules.json" ] && cmp -s "$T/kept/rules.json" "$OVERRIDE"; then
  ok "an installed override is preferred over the plugin's copy and never refreshed"
else
  bad "override not read: watcher read '${got:-nothing}', and the file $(cmp -s "$T/kept/rules.json" "$OVERRIDE" && echo survived || echo 'was OVERWRITTEN')"
fi

if [ "$fail_count" = 0 ]; then
  echo "verify-herdr-bob-rules: ok"
else
  echo "verify-herdr-bob-rules: $fail_count check(s) failed" >&2
  exit 1
fi
