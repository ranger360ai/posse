#!/usr/bin/env bash
# Pins the four darwin portability bugs that kept the herdr-bob watcher from
# ever running on this box (ranger-base-mz8ud), and the patch that fixes them:
# etc/herdr/herdr-bob/darwin-portability.patch.
#
# Both arms run against COPIES of the installed plugin checkout in a temp dir,
# driven by a fake `herdr` -- nothing here touches the live herdr server, the
# live plugin state, or a real Bob. The control arm is the point: a check that
# only exercised the patched tree could not tell a fix from a harness that
# never reproduced the bug (the lesson of ranger-base-26y03).
#
#   control    the pristine tree, on darwin: the start arm leaves a DEAD pid,
#              every log line is untimestamped, and bob-watch exits -- status
#              0, silently -- at the first screen read of a registered pane.
#   patched    the same tree plus the patch: the start arm leaves a live
#              detached pid, log lines carry an ISO timestamp, an unclaimed
#              pane running Bob is adopted and labelled with NO prompt typed,
#              a blocked screen is reported once and cleared once, and the
#              heartbeat is read as fresh when it is fresh.
#   installer  scripts/herdr-bob-install.sh hands bob-watchctl the same roots
#              herdr's own [[startup]] hook does, and refuses to make a second
#              watcher under the other root.
#
# Skips, loudly, when the checkout is absent or has moved off the commit the
# patch was cut against -- same contract as verify-detection's fork point.
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PATCH="$REPO/etc/herdr/herdr-bob/darwin-portability.patch"
PINNED_SHA=e53bd475dbe362c8a9195b582b88ef2c0857590b
SRC="${HERDR_BOB_ROOT:-$HOME/.config/herdr/plugins/github/mloeper.herdr-bob-de923c8f068d}"

fail_count=0
skip() { echo "SKIP: $*"; exit 0; }
ok()   { echo "  ok   $*"; }
bad()  { echo "  FAIL $*"; fail_count=$(( fail_count + 1 )); }

# ------------------------------------------------------------- the matchers
# IN AN ASSERTION ARM THE MATCHER MUST NOT FORK (ranger-base-t07yx,
# ranger-base-7hx87; pinned by internal/treepins/selftestforkarm_qa_test.go).
# A grep/sed/awk that is signalled, that cannot be exec-ed under the load
# `make test` itself creates, or that takes EPIPE past the 64 KB pipe buffer
# reports the property FALSE when the apparatus is what failed. Every verdict
# below therefore decides with `case` and `${...}` over bytes already in a
# variable. These are the shapes already in the tree -- suite-lock.sh's
# log_has, audit-silent-reverts.sh's sr_has/sr_count, test-times.sh's
# line_after -- and the sibling scripts/verify-herdr-bob-rules.sh carries the
# same block for the same reason.
#
# The tools UNDER TEST still fork, because that fork is the measurement: the
# plugin's own bob-watch greps its rules, and the fake herdr runs jq.

# has <text> <literal> -- the quoted half of the pattern is literal, so a
# needle full of dots and brackets is a needle and not a pattern.
has() { case $1 in *"$2"*) return 0 ;; esac; return 1; }

# slurp <file> -- the contents, or empty. A file that cannot be read is empty
# here and the arm that asked says what it wanted, same as a miss.
slurp() { local c=; [ -r "$1" ] && c=$(<"$1"); printf '%s' "$c"; }

# file_has <file> <literal> -- suite-lock.sh's log_has shape. `grep -q <pat>
# <file>` is the same defect as a piped grep and reads even more innocently.
file_has() { has "$(slurp "$1")" "$2"; }

# line_has2 <file> <lit1> <lit2> -- ONE line holding both, in that order. Not
# `*"$1"*"$2"*` over the whole file, which would match the first on one line
# and the second on another and call that a hit.
line_has2() {
  local line
  while IFS= read -r line || [ -n "$line" ]; do
    case $line in *"$2"*"$3"*) return 0 ;; esac
  done <<<"$(slurp "$1")"
  return 1
}

# count_lines <file> <literal> -- lines containing it. Prints a number ALWAYS,
# 0 included: an arm asking "none" has to be able to tell 0 from a matcher
# that never ran, and a matcher that never ran cannot print.
count_lines() {
  local line n=0
  while IFS= read -r line || [ -n "$line" ]; do
    case $line in *"$2"*) n=$(( n + 1 )) ;; esac
  done <<<"$(slurp "$1")"
  printf '%s' "$n"
}

# iso_lines <file> / body_lines <file> -- how many lines open with an ISO
# timestamp, and how many non-empty lines there are. The glob IS the matcher;
# there is no regexp to fork and nothing to compile.
iso_lines() {
  local line n=0
  while IFS= read -r line || [ -n "$line" ]; do
    case $line in
      [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T*) n=$(( n + 1 )) ;;
    esac
  done <<<"$(slurp "$1")"
  printf '%s' "$n"
}
body_lines() {
  local line n=0
  while IFS= read -r line || [ -n "$line" ]; do
    [ -n "$line" ] && n=$(( n + 1 ))
  done <<<"$(slurp "$1")"
  printf '%s' "$n"
}

# first_line <file> / last_line <file> -- head -1 and tail -1 without the fork,
# used in verdict MESSAGES too: a message that forks can be emptied by the
# same failure the arm is reporting, and then says the property is absent.
first_line() { local line; while IFS= read -r line || [ -n "$line" ]; do printf '%s' "$line"; return 0; done <<<"$(slurp "$1")"; }
last_line()  { local line out=; while IFS= read -r line || [ -n "$line" ]; do out=$line; done <<<"$(slurp "$1")"; printf '%s' "$out"; }

# text_line_is <text> <exact line> and text_line_like <text> <glob> -- the two
# spellings of `grep -qxF` and `grep -qE` over output already captured. The
# glob is used UNQUOTED in the case, which is what makes it a pattern.
text_line_is() {
  local line
  while IFS= read -r line || [ -n "$line" ]; do [ "$line" = "$2" ] && return 0; done <<<"$1"
  return 1
}
text_line_like() {
  local line
  while IFS= read -r line || [ -n "$line" ]; do case $line in $2) return 0 ;; esac; done <<<"$1"
  return 1
}

[ -e "$PATCH" ] || { echo "FAIL: no patch at $PATCH" >&2; exit 1; }
[ -d "$SRC/bin" ] || skip "herdr-bob is not installed at $SRC"
command -v jq >/dev/null 2>&1 || skip "jq is not on PATH"
have_sha="$(git -C "$SRC" rev-parse HEAD 2>/dev/null || echo unknown)"
[ "$have_sha" = "$PINNED_SHA" ] ||
  skip "herdr-bob is at $have_sha, the patch was cut against $PINNED_SHA -- re-cut it"

T="$(mktemp -d "${TMPDIR:-/tmp}/verify-herdr-bob.XXXXXX")" || exit 1
STARTED_PIDS=""
cleanup() {
  local p
  for p in $STARTED_PIDS; do kill "$p" 2>/dev/null || true; done
  rm -rf "$T"
}
trap cleanup EXIT INT TERM

# ---------------------------------------------------------------- fake herdr
# Answers only the verbs bob-watch and bob-watchctl use, and records every
# call. $F/alive lists the panes `pane get` admits; $F/unclaimed is what
# `pane list` reports with agent=null; $F/screen is what `pane read` prints.
mkdir -p "$T/bin"
cat > "$T/bin/herdr" <<'FAKE'
#!/usr/bin/env bash
set -u
F="$FAKE_DIR"
printf '%s\n' "$*" >> "$F/calls.log"
case "${1:-} ${2:-}" in
  "pane list")
    printf '{"result":{"panes":['
    first=1
    while read -r p; do
      [ -n "$p" ] || continue
      [ "$first" = 1 ] || printf ','
      first=0
      printf '{"pane_id":"%s","agent":null}' "$p"
    done < "$F/unclaimed"
    printf ']}}\n'
    exit 0 ;;
  "pane process-info")
    # Both argv shapes posse measured on a live pane (ranger-base-mz8ud).
    printf '%s\n' '{"result":{"process_info":{"foreground_processes":[
      {"argv":["/opt/homebrew/Cellar/node/25.2.1/bin/node","/opt/homebrew/bin/bob","chat"]},
      {"argv":["node","/opt/homebrew/bin/bob","chat"]}]}}}'
    exit 0 ;;
  "pane get")
    grep -qxF -- "${3:-}" "$F/alive" && exit 0
    printf '%s\n' '{"error":{"code":"pane_not_found"}}'
    exit 1 ;;
  "pane read")
    cat "$F/screen"
    exit 0 ;;
esac
exit 0
FAKE
chmod +x "$T/bin/herdr"

F="$T/fake"
mkdir -p "$F"
: > "$F/calls.log"
printf 'w9TT:p1\n' > "$F/alive"
printf 'w9TT:p1\n' > "$F/unclaimed"
PANE=w9TT:p1

# The watcher's state machine is what these arms pin, so the blocked screen is
# a synthetic one written to match the plugin's shipped trust_folder_prompt
# rule. It is NOT a Bob capture: none of the five real fixtures in
# etc/herdr/agent-detection/upstream/bob matches rules.json at all, which is a
# separate upstream bug -- see the filing.
BLOCKED_SCREEN='Do you trust this folder?
  ~/src/scratch
  (y/n)'
IDLE_SCREEN='  ❯   Build Anything, @ for context, / for commands, $ for skills
  Agent Mode'

prepare() { # dest-name, apply-patch?
  local d="$T/$1"
  cp -R "$SRC" "$d" || return 1
  # $SRC is the LIVE checkout and `cp -R` copies its WORKING TREE, which
  # carries the patch from the moment install-herdr-bob has run -- it did, on
  # 2026-10-03 (ranger-base-knikn). So the copy must be staged from the pinned
  # COMMIT rather than from what was copied, or the control arm's "pristine"
  # tree is whatever is installed and the control silently stops being a
  # control. MEASURED: it does not then fail, it HANGS -- a patched bob-watch
  # no longer dies at the first screen read, so the foreground run the control
  # arm expects to exit runs the watch loop forever (300s and counting, two
  # live watchers, and no output at all).
  git -C "$d" checkout --force "$PINNED_SHA" -- . >/dev/null 2>&1 || return 1
  # And prove it, rather than trust the checkout: a tree the patch still
  # applies to is a tree that does not have it.
  git -C "$d" apply --check "$PATCH" >/dev/null 2>&1 || return 1
  if [ "$2" = patch ]; then
    git -C "$d" apply "$PATCH" || return 1
  fi
  mkdir -p "$T/$1-state/panes" "$T/$1-state/heartbeat" "$T/$1-config"
  printf '%s' "$d"
}

# Exports every variable the plugin reads, so neither arm can reach the live
# state directory, the live plugin config, or the real herdr socket.
run_in() { # root, state, config, cmd...
  local root=$1 state=$2 config=$3; shift 3
  env PATH="$T/bin:$PATH" \
      FAKE_DIR="$F" \
      HERDR_BIN_PATH="$T/bin/herdr" \
      HERDR_BOB_STATE_DIR="$state" \
      HERDR_PLUGIN_STATE_DIR="$state" \
      HERDR_PLUGIN_CONFIG_DIR="$config" \
      BOB_WATCH_INTERVAL=1 \
      BOB_ADOPT_EVERY=1 \
      /bin/bash "$@"
}

# wait_count <file> <literal> <at-least-n> <seconds>, and wait_for for n=1.
wait_count() {
  local i=0 n
  while [ "$i" -lt "$4" ]; do
    n=$(count_lines "$1" "$2")
    [ "${n:-0}" -ge "$3" ] && return 0
    sleep 1
    i=$(( i + 1 ))
  done
  return 1
}

# A pattern that has fired before fires again, so every wait counts rather
# than matches: waiting for the mere presence of a line the watcher logged two
# transitions ago returns at once and lets the next assertion race it.
wait_for() { wait_count "$1" "$2" 1 "$3"; }

# wait_line2 <file> <lit1> <lit2> <seconds> -- one line holding both.
wait_line2() {
  local i=0
  while [ "$i" -lt "$4" ]; do
    line_has2 "$1" "$2" "$3" && return 0
    sleep 1
    i=$(( i + 1 ))
  done
  return 1
}

# ------------------------------------------------------------------- control
if [ "$(uname -s)" = Darwin ]; then
  echo "control (pristine $PINNED_SHA, darwin $(uname -r)):"
  C="$(prepare base '')" || { echo "FAIL: could not stage the pristine tree" >&2; exit 1; }
  CS="$T/base-state"

  out="$(run_in "$C" "$CS" "$T/base-config" "$C/bin/bob-watchctl" start 2>&1)"
  pid="$(slurp "$CS/watch.pid")"
  sleep 2
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    STARTED_PIDS="$STARTED_PIDS $pid"
    bad "bug 1: the pristine start arm left a LIVE pid $pid -- setsid resolved?"
  else
    ok "bug 1: start reported \"${out##*$'\n'}\" and pid ${pid:-none} is already dead"
  fi

  # Registered pane + a screen: the pristine watcher should die reading it.
  printf '\n' > "$CS/panes/$PANE"
  printf '%s\n' "$IDLE_SCREEN" > "$F/screen"
  : > "$F/calls.log"
  # BOUNDED, because the thing being pinned here is a death: a pristine
  # bob-watch exits at this read, and anything that does not exit runs the
  # watch loop until something stops it. An unbounded foreground run turns the
  # one regression this arm exists to catch into a hang with no output.
  run_in "$C" "$CS" "$T/base-config" "$C/bin/bob-watch" >/dev/null 2>"$T/base.err" &
  w=$!
  rc=
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    kill -0 "$w" 2>/dev/null || { wait "$w"; rc=$?; break; }
    sleep 1
  done
  if [ -z "$rc" ]; then
    kill "$w" 2>/dev/null; wait "$w" 2>/dev/null
    bad "bug 4: the pristine bob-watch did NOT die within 10s of the first screen read -- it ran the loop, so the staged tree is not pristine"
  else
    # The status is whatever the previous command left -- 0 in a bare
    # reproduction, 1 after match_rules declines the screen -- so the pin is
    # that it DIED on the read, not that it died with a particular number. In
    # production stderr goes to /dev/null and nothing reaches the log.
    err_last="$(last_line "$T/base.err")"
    if file_has "$T/base.err" 'unbound variable' && ! file_has "$F/calls.log" 'report-agent'; then
      ok "bug 4: bob-watch died on the first screen read (rc=$rc, \"${err_last##*: }\"), reporting nothing"
    else
      bad "bug 4: expected a death on the first screen read, got rc=$rc, stderr: $(slurp "$T/base.err")"
    fi
    file_has "$T/base.err" 'declare: -A: invalid option' &&
      ok "bug 4: 'declare -A' is an invalid option on $(/bin/bash -c 'printf %s "$BASH_VERSION"')" ||
      bad "bug 4: no 'declare -A' rejection from /bin/bash"
  fi

  if [ "$(body_lines "$CS/herdr-bob.log")" != 0 ] && [ "$(iso_lines "$CS/herdr-bob.log")" = 0 ]; then
    ok "bug 2: every pristine log line is untimestamped (date -Is failed)"
  else
    bad "bug 2: expected untimestamped log lines, got: $(first_line "$CS/herdr-bob.log")"
  fi

  mtype="$(run_in "$C" "$CS" "$T/base-config" -c ". $C/lib/common.sh; type file_mtime" 2>&1)"
  if ! has "$mtype" '%Y'; then
    ok "bug 3: the pristine tree has no portable mtime helper (stat -c %Y inline)"
  fi
else
  echo "control: skipped, the four failures are darwin-only (this is $(uname -s))"
fi

# ------------------------------------------------------------------- patched
echo "patched (+ etc/herdr/herdr-bob/darwin-portability.patch):"
D="$(prepare fix patch)" || { echo "FAIL: the patch does not apply to $SRC" >&2; exit 1; }
DS="$T/fix-state"
: > "$F/calls.log"
printf '%s\n' "$IDLE_SCREEN" > "$F/screen"

out="$(run_in "$D" "$DS" "$T/fix-config" "$D/bin/bob-watchctl" start 2>&1)"
pid="$(slurp "$DS/watch.pid")"
sleep 2
if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
  STARTED_PIDS="$STARTED_PIDS $pid"
  ok "bug 1: start left a live watcher, pid $pid (pidfile agrees)"
else
  bad "bug 1: the patched start arm left no live pid ($out)"
fi

if wait_for "$DS/herdr-bob.log" 'adopted hand-started bob' 12; then
  ok "adoption: an unclaimed pane running Bob was adopted with no prompt typed"
else
  bad "adoption: no 'adopted hand-started bob' within 12s"
fi
line_has2 "$F/calls.log" "pane report-agent $PANE " '--state idle' &&
  ok "adoption: reported state idle through pane report-agent" ||
  bad "adoption: no report-agent --state idle in the call log"
line_has2 "$F/calls.log" 'pane report-metadata' '--display-agent' &&
  ok "adoption: reported the display name" ||
  bad "adoption: no report-metadata --display-agent"

n_iso="$(iso_lines "$DS/herdr-bob.log")"
n_body="$(body_lines "$DS/herdr-bob.log")"
if [ "$n_body" != 0 ] && [ "$n_iso" = "$n_body" ]; then
  first="$(first_line "$DS/herdr-bob.log")"
  ok "bug 2: every log line carries an ISO timestamp (${first%% *})"
else
  bad "bug 2: $n_iso of $n_body log lines carry an ISO timestamp; first is: $(first_line "$DS/herdr-bob.log")"
fi

printf '%s\n' "$BLOCKED_SCREEN" > "$F/screen"
if wait_for "$DS/herdr-bob.log" '-> blocked (trust_folder_prompt)' 12; then
  ok "bug 4: blocked reported from the screen (the read that killed bash 3.2)"
else
  bad "bug 4: no blocked transition within 12s"
fi
n_blocked="$(count_lines "$F/calls.log" '--state blocked')"
sleep 3
[ "$(count_lines "$F/calls.log" '--state blocked')" = "$n_blocked" ] &&
  ok "bug 4: blocked reported ONCE, not once per tick (the set remembers)" ||
  bad "bug 4: blocked re-reported while the screen did not change"

# Fresh heartbeat: the watcher must hand idle/working back to the hook bridge.
: > "$DS/heartbeat/$PANE"
printf '%s\n' "$IDLE_SCREEN" > "$F/screen"
if wait_for "$DS/herdr-bob.log" 'block cleared, deferring to hook' 12; then
  ok "bug 3: a fresh heartbeat read as FRESH, so the hook keeps idle/working"
else
  bad "bug 3: the fresh heartbeat was not read as fresh within 12s"
fi

# Stale heartbeat: fall back to idle rather than leave the pane blocked. The
# pane has to re-enter blocked first, and that is the SECOND blocked line --
# waiting for the first returns instantly and flips the screen back to idle
# before the watcher ever reads it.
printf '%s\n' "$BLOCKED_SCREEN" > "$F/screen"
if wait_count "$DS/herdr-bob.log" '-> blocked (trust_folder_prompt)' 2 12; then
  ok "bug 4: blocked fires again after the set was cleared"
else
  bad "bug 4: the pane did not re-enter blocked within 12s"
fi
touch -t 200001010000 "$DS/heartbeat/$PANE"
printf '%s\n' "$IDLE_SCREEN" > "$F/screen"
if wait_for "$DS/herdr-bob.log" 'idle (block cleared, no live hook)' 14; then
  ok "bug 3: a stale heartbeat read as STALE, so the watcher falls back to idle"
else
  bad "bug 3: a year-2000 heartbeat was not read as stale within 14s"
fi

# Pane goes away: release, forget, and drop it from the blocked set.
: > "$F/alive"
if wait_for "$DS/herdr-bob.log" "pane $PANE gone -> released" 12; then
  ok "release: a vanished pane is released and forgotten"
else
  bad "release: no release within 12s of the pane going away"
fi

run_in "$D" "$DS" "$T/fix-config" "$D/bin/bob-watchctl" stop >/dev/null 2>&1
sleep 1
if kill -0 "$pid" 2>/dev/null; then
  bad "stop: pid $pid is still alive"
else
  ok "stop: the watcher stopped"
  STARTED_PIDS="${STARTED_PIDS/ $pid/}"
fi

# ------------------------------------------------- the installer's own roots
# The patch is only half of what a start needs. The plugin resolves its state
# and config roots from HERDR_PLUGIN_STATE_DIR / HERDR_PLUGIN_CONFIG_DIR, which
# herdr sets when ITS [[startup]] hook runs the command and nothing sets when
# we do -- so `--start` with no environment silently chose the FALLBACK roots
# and read upstream's unproved rules.json (MEASURED 2026-10-03 on this box, the
# operator's finding on ranger-base-knikn: two watchers, and the plugin's own
# `stop` could not see the wrong-root one from the right root).
#
# These arms run the real scripts/herdr-bob-install.sh against a scratch XDG
# and the fake herdr, so the assertions are behavioural -- where the pidfile
# landed and which rules path the watcher logged -- rather than a grep of the
# script. Nothing here reaches the live server or the live plugin state. (It
# cannot stub bin/bob-watchctl to read its environment: that file is one of the
# three the patch touches, so a stub makes the installer's own `applied=` check
# read no.)
echo "installer (scripts/herdr-bob-install.sh, scratch XDG):"
I="$(prepare inst patch)" || { echo "FAIL: could not stage the installer arm" >&2; exit 1; }
mkdir -p "$T/xdg-state" "$T/xdg-config"
want_state="$T/xdg-state/herdr/plugins/mloeper.herdr-bob"
want_config="$T/xdg-config/herdr/plugins/config/mloeper.herdr-bob"
fallback_state="$T/xdg-state/herdr-bob"

install_sh() {
  env PATH="$T/bin:$PATH" FAKE_DIR="$F" \
      HERDR_BOB_ROOT="$I" \
      XDG_STATE_HOME="$T/xdg-state" XDG_CONFIG_HOME="$T/xdg-config" \
      BOB_WATCH_INTERVAL=1 BOB_ADOPT_EVERY=1 \
      HERDR_PLUGIN_STATE_DIR= HERDR_PLUGIN_CONFIG_DIR= HERDR_BIN_PATH= \
      /bin/bash "$REPO/scripts/herdr-bob-install.sh" "$@"
}
watchctl_in_arm() { # state-dir, args...
  local state=$1; shift
  env PATH="$T/bin:$PATH" FAKE_DIR="$F" \
      HERDR_PLUGIN_STATE_DIR="$state" HERDR_PLUGIN_CONFIG_DIR="$want_config" \
      HERDR_BIN_PATH="$T/bin/herdr" \
      /bin/bash "$I/bin/bob-watchctl" "$@"
}

# Control: this is the bug the exports prevent. With no HERDR_PLUGIN_* set --
# which is exactly what the pre-fix --start handed the watcher -- the plugin's
# own state_root()/config_root() answer the FALLBACKS, not the plugin roots, so
# a watcher started that way writes a different pidfile and reads a different
# rules.json from the one herdr's hook uses.
fb_state="$(env -u HERDR_PLUGIN_STATE_DIR -u HERDR_BOB_STATE_DIR \
  XDG_STATE_HOME="$T/xdg-state" /bin/bash -c ". $I/lib/common.sh; state_root")"
fb_config="$(env -u HERDR_PLUGIN_CONFIG_DIR \
  XDG_CONFIG_HOME="$T/xdg-config" /bin/bash -c ". $I/lib/common.sh; config_root")"
if [ "$fb_state" = "$fallback_state" ] && [ "$fb_config" = "$T/xdg-config/herdr-bob" ]; then
  ok "control: with no HERDR_PLUGIN_*, the plugin resolves the fallback roots (${fb_state##*/})"
else
  bad "control: expected the fallback roots, got state=$fb_state config=$fb_config"
fi

out="$(install_sh --check 2>&1)"
text_line_is "$out" "state:    $want_state" &&
  ok "--check reports the PLUGIN state root, read-only" ||
  bad "--check did not report state=$want_state; it said: $out"
text_line_is "$out" "config:   $want_config" &&
  ok "--check reports the PLUGIN config root" ||
  bad "--check did not report config=$want_config; it said: $out"

# `pane list` still offers an unclaimed Bob pane, so the started watcher has
# something to do; $F/alive was emptied by the arm above, so it releases it and
# carries on. Either way the two assertions are where it WROTE.
printf 'w9TT:p1\n' > "$F/alive"
out="$(install_sh --start 2>&1)"
rc=$?
[ "$rc" = 0 ] || bad "--start exited $rc: $out"
ipid="$(slurp "$want_state/watch.pid")"
if [ -n "$ipid" ] && kill -0 "$ipid" 2>/dev/null; then
  STARTED_PIDS="$STARTED_PIDS $ipid"
  ok "--start left the pidfile under the PLUGIN state root (pid $ipid), not the fallback"
else
  bad "--start left no live pid at $want_state/watch.pid (pid ${ipid:-none})"
fi
if [ -e "$fallback_state/watch.pid" ]; then
  bad "--start ALSO wrote $fallback_state/watch.pid -- the bug this arm exists for"
else
  ok "--start wrote nothing under the fallback root $fallback_state"
fi
if wait_line2 "$want_state/herdr-bob.log" 'watch started' "rules $want_config/rules.json" 10; then
  ok "--start's watcher logged the PLUGIN config root's rules, so it reads the installed override"
else
  bad "--start's watcher did not log rules under $want_config: $(last_line "$want_state/herdr-bob.log")"
fi
if has "$out" "HERDR_PLUGIN_STATE_DIR=$want_state" &&
   has "$out" "HERDR_PLUGIN_CONFIG_DIR=$want_config" &&
   text_line_like "$out" '*HERDR_BIN_PATH=/?*'; then
  ok "--start PRINTS the three roots it chose (a root chosen in silence is this bead's shape)"
else
  bad "--start did not print all three roots: $out"
fi

watchctl_in_arm "$want_state" stop >/dev/null 2>&1
sleep 1
if kill -0 "$ipid" 2>/dev/null; then
  bad "stop: the installer arm's watcher $ipid is still alive"
else
  STARTED_PIDS="${STARTED_PIDS/ $ipid/}"
fi

# A watcher left under the fallback root is invisible to a stop run under the
# plugin roots, so two of them end up reporting into one server. $$ is a live
# pid we own, which is the point -- nothing is started to test this.
mkdir -p "$fallback_state"
echo $$ > "$fallback_state/watch.pid"
out="$(install_sh --check 2>&1)"
if has "$out" 'a watcher is also running' && has "$out" 'bob-watchctl stop'; then
  ok "--check warns about a fallback-root watcher and names the stop command"
else
  bad "--check did not warn about the fallback-root watcher: $out"
fi
rm -f "$want_state/watch.pid"
out="$(install_sh --start 2>&1)"
if [ -e "$want_state/watch.pid" ]; then
  spid="$(slurp "$want_state/watch.pid")"
  kill "$spid" 2>/dev/null
  bad "--start started a SECOND watcher ($spid) with a fallback-root watcher alive"
elif has "$out" 'refusing to start'; then
  ok "--start REFUSES rather than make a second watcher"
else
  bad "--start neither started nor refused: $out"
fi
rm -f "$fallback_state/watch.pid"

if [ "$fail_count" = 0 ]; then
  echo "verify-herdr-bob: ok"
else
  echo "verify-herdr-bob: $fail_count check(s) failed" >&2
  exit 1
fi
