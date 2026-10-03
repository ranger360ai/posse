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

# ----------------------------------------------- the matchers, and the rule
# IN AN ASSERTION ARM THE MATCHER MUST NOT FORK (ranger-base-t07yx,
# ranger-base-7hx87; pinned by internal/treepins/selftestforkarm_qa_test.go).
# A grep/sed/awk that is signalled (143/137), that cannot be exec-ed under
# load (127, silent but for a stderr line), or that takes EPIPE past the 64 KB
# pipe buffer (141) reports the property FALSE when the apparatus is what
# failed. Sixteen arms here used to ask that way and ranger-base-vf3pf swept
# them; two were worth naming. The `grep -q watch started` wait loop would
# have spun out its ten seconds and then called a watcher that started fine
# never started, and the `awk` extraction would have written an empty harness
# and reported the installed bob-watch malformed.
#
# The tool UNDER TEST still forks, because that fork is the measurement:
# replay.sh runs the plugin own match_rules, and jq reads the override (see
# the note above $T/jq.sh). What is gone is every fork BETWEEN the bytes and
# the verdict. These are the shapes already in the tree -- suite-lock.sh
# log_has, audit-silent-reverts.sh sr_has/sr_count, test-times.sh line_after.

# has <text> <literal> -- the quoted half of the pattern is literal, so a
# needle full of dots and brackets is a needle and not a pattern.
has() { case $1 in *"$2"*) return 0 ;; esac; return 1; }
# count_prefix <text> <prefix> -- how many lines begin with it. Prints a
# number ALWAYS, 0 included: an arm asking "none" has to be able to tell 0
# from a matcher that never ran, and a matcher that never ran cannot print.
count_prefix() {
  local line n=0
  while IFS= read -r line || [ -n "$line" ]; do
    case $line in "$2"*) n=$(( n + 1 )) ;; esac
  done <<<"$1"
  printf '%s' "$n"
}
# n_lines <text> and last_line <text> -- wc -l and tail -1 without the fork.
n_lines() {
  local line n=0
  while IFS= read -r line || [ -n "$line" ]; do n=$(( n + 1 )); done <<<"$1"
  printf '%s' "$n"
}
last_line() {
  local line out=
  while IFS= read -r line || [ -n "$line" ]; do out=$line; done <<<"$1"
  printf '%s' "$out"
}
# log_has <file> <literal> -- suite-lock.sh shape. `grep -q <pat> <file>` is
# the same defect as a piped grep and reads even more innocently.
log_has() {
  local c
  [ -r "$1" ] || return 1
  c=$(<"$1")
  case $c in *"$2"*) return 0 ;; esac
  return 1
}
# same_file <a> <b> -- `cmp -s` decided two of the seeding verdicts below, and
# a cmp that cannot be exec-ed reports two identical files as different. cmp
# is not on the scan tool list; the invariant is not about the list. The one
# difference this cannot see is a trailing newline, which $(<f) strips from
# both sides -- for a file the thing under test copies byte for byte, that is
# not the drift either arm is about.
same_file() {
  local a b
  [ -r "$1" ] && [ -r "$2" ] || return 1
  a=$(<"$1"); b=$(<"$2")
  [ "$a" = "$b" ]
}
# apparatus <rc> -- true when <rc> is not an ANSWER: 126/127 is "not
# exec-ed", which is the shape a failed fork under load takes, and >= 128 is
# signalled, EPIPE included. Neither means the property is false.
apparatus() {
  case ${1:-} in
    ''|*[!0-9]*) return 0 ;;
    12[67]) return 0 ;;
  esac
  [ "$1" -ge 128 ]
}
# harness <what> [detail] -- an apparatus failure is never a finding. The
# extraction arm below already refused to report verdicts from a harness it
# could not trust; this is that sentence, reusable. The detail is its own line
# because it is whatever the tool said to stderr and that is rarely short.
harness() {
  echo "verify-herdr-bob-rules: $1 -- that is the apparatus and not the property;" >&2
  echo "  refusing to report verdicts from it (ranger-base-vf3pf)" >&2
  [ $# -gt 1 ] && [ -n "$2" ] && echo "  it said: $2" >&2
  exit 1
}

[ -e "$OVERRIDE" ] || { echo "FAIL: no override at $OVERRIDE" >&2; exit 1; }
[ -d "$FIX" ]      || { echo "FAIL: no fixtures at $FIX" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || skip "jq is not on PATH"
[ -d "$SRC/bin" ] || skip "herdr-bob is not installed at $SRC"
have_sha="$(git -C "$SRC" rev-parse HEAD 2>/dev/null || echo unknown)"
[ "$have_sha" = "$PINNED_SHA" ] ||
  skip "herdr-bob is at $have_sha, match_rules was read from $PINNED_SHA -- re-check the extraction"

T="$(mktemp -d "${TMPDIR:-/tmp}/verify-herdr-bob-rules.XXXXXX")" || exit 1
WATCH_PIDS=""
cleanup() {
  local p
  for p in $WATCH_PIDS; do kill "$p" 2>/dev/null || true; done
  rm -rf "$T"
}
# drop_pid <pid> -- take a reaped pid out of WATCH_PIDS. The `sed` this
# replaces removed one spelling of the pid and a dead one removed none, and
# either way cleanup then signals a pid this run no longer owns.
drop_pid() {
  local keep="" q
  for q in $WATCH_PIDS; do
    [ "$q" = "$1" ] || keep="$keep $q"
  done
  WATCH_PIDS=$keep
}
# last_rules_path <file> -- the path in the LAST `watch started (... rules
# <p>)` line, nothing when there is none. The `sed -n s///p | tail -1` it
# replaces is two forks deciding the two seeding verdicts below: either one
# dying reports the watcher as having read nothing.
last_rules_path() {
  local line rest out=""
  [ -r "$1" ] || return 0
  while IFS= read -r line || [ -n "$line" ]; do
    case $line in
      *"watch started ("*"rules "*")")
        rest=${line##*"rules "}
        out=${rest%")"}
        ;;
    esac
  done <"$1"
  printf '%s' "$out"
}
trap cleanup EXIT INT TERM

# ------------------------------------------------------- the two jq questions
# JQ IS THE READER OF THE SUBJECT HERE AND IT CANNOT BE BASH. Both questions
# are about the CONTENT of a JSON file -- does it parse and how many rules has
# it, and does any pattern hold a multibyte character inside a bracket
# expression -- and bash has no JSON parser. The coarse bash answer (any `[`
# run in the raw file holding a non-ASCII byte) would red on this very
# override, whose alternations legitimately carry one. So this fork stays; it
# is the measurement, the way replay.sh running the plugin own match_rules is
# the measurement.
#
# WHAT THE INVARIANT ASKS FOR IS THE OTHER HALF: a jq that is signalled or
# cannot be exec-ed must not report the property false. So each call returns
# jq own status, the caller reads it against `apparatus` rather than trusting
# it, and an apparatus failure exits loudly instead of printing a FAIL.
#
# AND IT IS A RIG FILE, WHICH IS ALSO HOW IT PASSES THE SCAN -- say so plainly
# rather than leave the next reader to find it. selftestforkarm_qa_test.go
# classifies by SHAPE: a tool on an assertion line is a verdict, a tool inside
# a `bad`/`fail` argument is a message, a `cat` heredoc is a fixture. It has no
# role for a tool that IS the subject reader, so these two jq programs would
# red it wherever on an assertion path they stood, and inside this heredoc they
# do not. That is a real blind spot in the pin and not a property of this file:
# filed as ranger-base-mzzs1. The jq programs live here anyway for a reason
# that predates it -- the second one is four lines, and an embedded four-line
# jq program in the middle of an arm is the thing nobody reads.
cat > "$T/jq.sh" <<'JQ_EOF'
#!/usr/bin/env bash
# <override> <question> -> the answer on stdout, jq's own status on failure.
set -uo pipefail
case $2 in
  rules)
    jq -r '.rules | length' "$1" || exit $?
    ;;
  multibyte-brackets)
    jq -r '[.rules[] | (.all // []) + (.any // []) + (.none // []) | .[]]
           | map(select(test("\\[[^]]*[^\\x00-\\x7f]")))
           | if length == 0 then "none" else (. | join(", ")) end' "$1" || exit $?
    ;;
  *) exit 64 ;;
esac
JQ_EOF
chmod +x "$T/jq.sh"
jq_ask()  { "$T/jq.sh" "$OVERRIDE" "$1" 2>"$T/jq.err"; }   # status is jq's own
# jq_err -- the last line jq said, for the message. Kept rather than dropped
# on the floor: `jq -e . file >/dev/null 2>&1` used to decide this arm and the
# FAIL it printed named no reason at all.
jq_err()  { local c=""; [ -r "$T/jq.err" ] && c=$(<"$T/jq.err"); last_line "$c"; }

n_rules="$(jq_ask rules)"; jq_rc=$?
apparatus "$jq_rc" && harness "jq could not be run over $OVERRIDE (status $jq_rc)" "$(jq_err)"
case $n_rules in
  ''|*[!0-9]*) bad "the override is not valid JSON, or has no .rules array (jq status $jq_rc): $(jq_err)" ;;
  *)           ok "the override is valid JSON ($n_rules rules)" ;;
esac

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
#
# EXTRACTED BY BASH AND NOT BY AWK. `awk /^match_rules\(\) \{/,/^\}$/ > file`
# writes the harness, and an awk that is signalled writes an EMPTY one -- the
# arm below then reports the extraction wrong with 0 definitions and this
# script exits 1, on a box whose bob-watch is fine. The range is the same one:
# it opens on a line beginning `match_rules() {` and closes on a line that is
# exactly `}`, the end pattern tested on the opening line too.
extract_fn() { # file, opening-prefix
  local line inside=0
  while IFS= read -r line || [ -n "$line" ]; do
    if [ "$inside" = 0 ]; then
      case $line in "$2"*) inside=1 ;; esac
    fi
    [ "$inside" = 1 ] || continue
    printf '%s\n' "$line"
    [ "$line" = '}' ] && inside=0
  done <"$1"
}
mr="$(extract_fn "$SRC/bin/bob-watch" 'match_rules() {')"
printf '%s\n' "$mr" > "$T/match_rules.sh"
n_fn=$(count_prefix "$mr" 'match_rules() {')
if [ "$n_fn" = 1 ] &&
   has "$mr" 'grep -qiE -- "$pat"' &&
   has "$mr" '.rules[$i].all' &&
   has "$mr" '.rules[$i].any' &&
   has "$mr" '.rules[$i].none' &&
   [ "$(last_line "$mr")" = "}" ]; then
  ok "match_rules extracted from the installed bob-watch ($(n_lines "$mr") lines, all/any/none, grep -qiE)"
else
  bad "match_rules extraction looks wrong -- $n_fn definitions, $(n_lines "$mr") lines"
  echo "verify-herdr-bob-rules: the extraction is the harness; refusing to report verdicts from it" >&2
  exit 1
fi

# One replay: rules-file, fixture, lines-or-all -> rule id, or `-` for no match.
#
# THE SCREEN IS READ WITHOUT A FORK, and this one was measured rather than
# reasoned (ranger-base-vf3pf, 2026-10-03). `cat` and `tail -n` here are the
# APPARATUS; match_rules is the subject. With a `tail` on PATH whose whole body
# is `kill -TERM $$` the window came back empty, which reads exactly like the
# rule not firing: three override arms and all four locale arms reported MOVED
# and the script exited 1, with nothing in the output naming tail. bash reads
# the file twice instead -- once to count, once to emit the last $3 lines -- so
# no fork sits between the capture and the verdict. No arrays: `${arr[@]}`
# under `set -u` is an unbound-variable error on the bash 3.2 this box ships.
cat > "$T/replay.sh" <<'REPLAY'
#!/usr/bin/env bash
set -uo pipefail
. "$MR"
RULES="$1"
if [ "$3" = all ]; then
  screen="$(<"$2")"
else
  n=0
  while IFS= read -r l || [ -n "$l" ]; do n=$(( n + 1 )); done <"$2"
  skip=$(( n - $3 )); [ "$skip" -lt 0 ] && skip=0
  i=0; first=1; screen=""
  while IFS= read -r l || [ -n "$l" ]; do
    i=$(( i + 1 ))
    [ "$i" -le "$skip" ] && continue
    if [ "$first" = 1 ]; then screen=$l; first=0; else screen=$screen$'\n'$l; fi
  done <"$2"
fi
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
  local n; n=$(count_prefix "$out" '  FAIL ')
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
  # WHICH grep the first pair of arms ran, named ONCE and here rather than in
  # every label. `grep --version | head -1 | cut -d' ' -f1` was three forks
  # deciding a label, and on this box it only ever printed the word `grep`;
  # `command -v` is a builtin and the path is what a reader needs anyway. It
  # also makes a thing the old label hid visible: where /usr/bin/grep IS the
  # PATH grep, the BSD pair below re-runs the same binary.
  path_grep="$(command -v grep || true)"
  echo "grep x locale (verdicts must not move; PATH grep is ${path_grep:-not on PATH}):"
  multi="$(jq_ask multibyte-brackets)"; jq_rc=$?
  apparatus "$jq_rc" && harness "jq could not read the override patterns (status $jq_rc)" "$(jq_err)"
  case $multi in
    none) ok "no bracket expression in the override contains a multibyte character" ;;
    '')   bad "the override patterns could not be read at all (jq status $jq_rc): $(jq_err)" ;;
    *)    bad "bracket expression with a multibyte character: $multi" ;;
  esac
  for grepdir in "" "$T/bsdbin"; do
    for loc in en_US.UTF-8 C; do
      if [ -n "$grepdir" ]; then lbl="BSD grep LC_ALL=$loc"; else lbl="grep LC_ALL=$loc"; fi
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
    log_has "$T/state-$tag/herdr-bob.log" 'watch started' && break
    sleep 1; i=$(( i + 1 ))
  done
  kill "$p" 2>/dev/null || true
  wait "$p" 2>/dev/null || true
  drop_pid "$p"
  last_rules_path "$T/state-$tag/herdr-bob.log"
}

echo "seeding (the real bob-watch, fake herdr, scratch state and config):"
got="$(watch_once "$T/seed" seed)"
if [ "$got" = "$T/seed/rules.json" ] && same_file "$T/seed/rules.json" "$SRC/rules.json"; then
  ok "a fresh config dir is seeded from the plugin's own copy, and read from there"
else
  bad "fresh seeding: watcher read '${got:-nothing}', seeded file $(same_file "$T/seed/rules.json" "$SRC/rules.json" && echo matches || echo 'does NOT match') the plugin's"
fi

cp "$OVERRIDE" "$T/kept/rules.json"
got="$(watch_once "$T/kept" kept)"
if [ "$got" = "$T/kept/rules.json" ] && same_file "$T/kept/rules.json" "$OVERRIDE"; then
  ok "an installed override is preferred over the plugin's copy and never refreshed"
else
  bad "override not read: watcher read '${got:-nothing}', and the file $(same_file "$T/kept/rules.json" "$OVERRIDE" && echo survived || echo 'was OVERWRITTEN')"
fi

if [ "$fail_count" = 0 ]; then
  echo "verify-herdr-bob-rules: ok"
else
  echo "verify-herdr-bob-rules: $fail_count check(s) failed" >&2
  exit 1
fi
