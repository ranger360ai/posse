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
#   control  the pristine tree, on darwin: the start arm leaves a DEAD pid,
#            every log line is untimestamped, and bob-watch exits -- status 0,
#            silently -- at the first screen read of a registered pane.
#   patched  the same tree plus the patch: the start arm leaves a live detached
#            pid, log lines carry an ISO timestamp, an unclaimed pane running
#            Bob is adopted and labelled with NO prompt typed, a blocked screen
#            is reported once and cleared once, and the heartbeat is read as
#            fresh when it is fresh.
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

wait_count() { # file, pattern, at-least-n, seconds
  local i=0 n
  while [ "$i" -lt "$4" ]; do
    # grep -c PRINTS 0 and EXITS 1 on no match, so a `|| echo 0` fallback
    # yields the two-line "0\n0" that [ cannot compare.
    n=$(grep -cE -- "$2" "$1" 2>/dev/null)
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

# ------------------------------------------------------------------- control
if [ "$(uname -s)" = Darwin ]; then
  echo "control (pristine $PINNED_SHA, darwin $(uname -r)):"
  C="$(prepare base '')" || { echo "FAIL: could not stage the pristine tree" >&2; exit 1; }
  CS="$T/base-state"

  out="$(run_in "$C" "$CS" "$T/base-config" "$C/bin/bob-watchctl" start 2>&1)"
  pid="$(cat "$CS/watch.pid" 2>/dev/null || echo)"
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
  run_in "$C" "$CS" "$T/base-config" "$C/bin/bob-watch" >/dev/null 2>"$T/base.err"
  rc=$?
  # The status is whatever the previous command left -- 0 in a bare
  # reproduction, 1 after match_rules declines the screen -- so the pin is that
  # it DIED on the read, not that it died with a particular number. In
  # production stderr goes to /dev/null and nothing reaches the log.
  if grep -q 'unbound variable' "$T/base.err" &&
     ! grep -q 'report-agent' "$F/calls.log"; then
    ok "bug 4: bob-watch died on the first screen read (rc=$rc, \"$(sed -n '$p' "$T/base.err" | sed 's/.*: //')\"), reporting nothing"
  else
    bad "bug 4: expected a death on the first screen read, got rc=$rc, stderr: $(cat "$T/base.err")"
  fi
  grep -q 'declare: -A: invalid option' "$T/base.err" &&
    ok "bug 4: 'declare -A' is an invalid option on $(/bin/bash -c 'printf %s "$BASH_VERSION"')" ||
    bad "bug 4: no 'declare -A' rejection from /bin/bash"

  if [ -s "$CS/herdr-bob.log" ] && ! grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T' "$CS/herdr-bob.log"; then
    ok "bug 2: every pristine log line is untimestamped (date -Is failed)"
  else
    bad "bug 2: expected untimestamped log lines, got: $(head -1 "$CS/herdr-bob.log" 2>/dev/null)"
  fi

  if ! grep -q '%Y' <(run_in "$C" "$CS" "$T/base-config" -c \
       '. '"$C"'/lib/common.sh; type file_mtime' 2>&1); then
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
pid="$(cat "$DS/watch.pid" 2>/dev/null || echo)"
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
grep -qE "pane report-agent $PANE .*--state idle" "$F/calls.log" &&
  ok "adoption: reported state idle through pane report-agent" ||
  bad "adoption: no report-agent --state idle in the call log"
grep -q 'pane report-metadata .*--display-agent' "$F/calls.log" &&
  ok "adoption: reported the display name" ||
  bad "adoption: no report-metadata --display-agent"

if grep -qvE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T' "$DS/herdr-bob.log"; then
  bad "bug 2: a log line without an ISO timestamp: $(grep -nvE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T' "$DS/herdr-bob.log" | head -1)"
else
  ok "bug 2: every log line carries an ISO timestamp ($(head -1 "$DS/herdr-bob.log" | cut -d' ' -f1))"
fi

printf '%s\n' "$BLOCKED_SCREEN" > "$F/screen"
if wait_for "$DS/herdr-bob.log" '\-> blocked \(trust_folder_prompt\)' 12; then
  ok "bug 4: blocked reported from the screen (the read that killed bash 3.2)"
else
  bad "bug 4: no blocked transition within 12s"
fi
n_blocked=$(grep -c -- '--state blocked' "$F/calls.log")
sleep 3
[ "$(grep -c -- '--state blocked' "$F/calls.log")" = "$n_blocked" ] &&
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
if wait_count "$DS/herdr-bob.log" '\-> blocked \(trust_folder_prompt\)' 2 12; then
  ok "bug 4: blocked fires again after the set was cleared"
else
  bad "bug 4: the pane did not re-enter blocked within 12s"
fi
touch -t 200001010000 "$DS/heartbeat/$PANE"
printf '%s\n' "$IDLE_SCREEN" > "$F/screen"
if wait_for "$DS/herdr-bob.log" 'idle \(block cleared, no live hook\)' 14; then
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
  STARTED_PIDS="$(printf '%s' "$STARTED_PIDS" | sed "s/ $pid//")"
fi

if [ "$fail_count" = 0 ]; then
  echo "verify-herdr-bob: ok"
else
  echo "verify-herdr-bob: $fail_count check(s) failed" >&2
  exit 1
fi
