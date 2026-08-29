#!/usr/bin/env bash
# test-times.sh — run a `go test` command and then say, in words, how long each
# package took and how much of its -timeout budget that was (ranger-base-7xla).
#
# Usage: scripts/test-times.sh <go test command...>    what `make test` runs
#        scripts/test-times.sh --self-test             prove the reporting works
#
# Environment:
#   SLOW_PACKAGE_SECONDS=300   the line above which a package is called slow
#
# THIS SCRIPT DOES NOT OWN THE TIMEOUT. ranger-base-2ggb put `-timeout 25m` in
# the Makefile's `test` recipe and suitetimeout_qa_test.go pins it there — the
# flag stays on the recipe line, visible to that pin, and this script READS it
# out of the command it was handed to compute the budget column. There is one
# number and it lives where the pin can see it. If the command carries no
# -timeout at all, that is itself reported: it means go's default of ten
# minutes per package, which is the condition 2ggb closed.
#
# WHY IT EXISTS ANYWAY. 2ggb raised the ceiling; ranger-base-7xla is the other
# half of the same bead, the half its author flagged as a decision rather than
# a default: "raising the timeout hides the growth. A gate that prints the
# per-package seconds and warns above a threshold keeps the signal." The pin
# holds the FLAG, not the runtime. Nothing measured the runtime between runs,
# so internal/rhq could walk from 9 minutes to 24 with every gate green and the
# only notice being the day it trips.
#
# The numbers that make that concrete, all darwin 25.4.0 / go1.26, 2026-08-29,
# pooled across three sessions (docs/notes.d/ranger-base-2ggb.md has the table):
# internal/rhq standalone 484.6s / 509.6s / 510.0s / 549.3s / 623.2s, and
# 600.8s / 601.0s / 601.1s under `go test ./...` — the last three not
# assertions but the 10m default arriving as a timeout panic. Native
# linux/arm64 runs the same package in 112.3s.
#
# AND WHEN THE BUDGET DOES EXPIRE, SAY SO. A timeout is a panic plus a full
# goroutine dump, which is what a deadlock in product code looks like; through
# the house filter (`| grep -E '^(---|ok|FAIL)'`) it is a bare `FAIL … 601.010s`
# naming no test at all. Raising the ceiling to 25m makes that rarer, not
# legible. This prints a block that names the package, says the clock ran out,
# says plainly that it CANNOT tell a slow package from a real hang, and gives
# the one command that can.
#
# SLOW_PACKAGE_SECONDS (300) is a separate number with a separate job, and it
# is deliberately not derived from the timeout: raising one to quiet the other
# is how the growth gets hidden. Five minutes is the claim — a package that
# takes longer than that to test as one binary wants splitting, whatever the
# ceiling happens to be. internal/rhq is over the line today (462-623s) and is
# expected to warn on every darwin run until it is not. That is the number
# working, not a miscalibration of it.
#
# IT NEVER FAILS ON A WALL CLOCK. `go test`'s exit status is the one you get
# back, unchanged. A gate that goes red on elapsed time is the class tvmh and
# fsil were: a deterministic-looking red that is really the box's mood.
set -uo pipefail

SLOW=${SLOW_PACKAGE_SECONDS:-300}

warn() { printf 'test-times: %s\n' "$*" >&2; }

# to_seconds <go duration> — "25m" -> 1500. Only the forms a -timeout carries.
to_seconds() {
  case $1 in
    *h) printf '%s\n' "$(( ${1%h} * 3600 ))" ;;
    *m) printf '%s\n' "$(( ${1%m} * 60 ))" ;;
    *s) printf '%s\n' "${1%s}" ;;
    *)  return 1 ;;
  esac
}

# budget_of <command...> — the -timeout the command carries, in seconds, or
# empty when it carries none. Both spellings, one dash or two, because
# `-timeout=25m` and `--timeout 25m` are the same instruction to go.
budget_of() {
  local a flag next=0
  for a in "$@"; do
    if [ "$next" = 1 ]; then to_seconds "$a"; return; fi
    flag=${a#-}; flag=${flag#-}
    case $flag in
      timeout)    next=1 ;;
      timeout=*)  to_seconds "${flag#timeout=}"; return ;;
    esac
  done
  return 1
}

# report <logfile> <budget-seconds> <budget-was-explicit 0|1>
# Kept a pure function of a captured log so --self-test can drive it over
# fixtures instead of over a ten-minute suite.
report() {
  local log=$1 budget=$2 explicit=$3
  local pkg secs count=0 slow_hit=0 slow_msg="" line times

  # Package result lines are tab-separated: "ok  \t<pkg>\t12.34s" and
  # "FAIL\t<pkg>\t600.12s". Field 1 on an ok line carries trailing spaces,
  # hence the pattern rather than ==. Plain POSIX awk only: this repo has
  # already been bitten by a gawk/mawk difference no developer box could see
  # (ranger-base-hhcu).
  times=$(awk -F'\t' '
    ($1 ~ /^ok[ ]*$/ || $1 == "FAIL") && NF >= 3 {
      # Field 3 is the seconds, but not always ONLY the seconds: with a -run
      # matching nothing go writes "0.356s [no tests to run]" into the same
      # field, and a cached result writes "(cached)" with no seconds at all.
      # Take the first blank-delimited token and require it to be a duration.
      split($3, f, " ")
      t = f[1]
      if (t ~ /^[0-9]+(\.[0-9]+)?s$/) { sub(/s$/, "", t); printf "%s\t%s\n", t, $2 }
    }' "$log" | sort -rn)

  if [ -z "$times" ]; then
    printf 'test-times: no package times in this run (cached, no test files, or the build failed)\n'
    return 0
  fi

  if [ "$explicit" = 1 ]; then
    printf '\ntest-times: package times, against the -timeout of %ss per package\n' "$budget"
  else
    printf '\ntest-times: package times, against go test'"'"'s DEFAULT %ss per package\n' "$budget"
  fi

  while IFS=$'\t' read -r secs pkg; do
    [ -n "$pkg" ] || continue
    count=$((count + 1))
    printf '  %8.1fs  %3d%% of budget  %s\n' "$secs" \
      "$(awk -v s="$secs" -v b="$budget" 'BEGIN{ printf "%d", (s + 0) * 100 / (b + 0) }')" "$pkg"
    if awk -v s="$secs" -v l="$SLOW" 'BEGIN{ exit !((s + 0) >= (l + 0)) }'; then
      slow_hit=$((slow_hit + 1))
      slow_msg="$slow_msg$pkg took ${secs}s
"
    fi
  done <<EOF
$times
EOF
  printf 'test-times: %d package(s) timed, %d over the %ss line\n' "$count" "$slow_hit" "$SLOW"

  # Warnings come AFTER the whole table: they go to stderr and the table to
  # stdout, so anything printed mid-loop lands in the middle of the table in a
  # merged stream and orphaned in a split one.
  if [ "$explicit" != 1 ]; then
    warn "this command carries no -timeout, so every package ran on go's default of"
    warn "  ${budget}s. internal/rhq has been measured past that (ranger-base-2ggb);"
    warn "  run the suite through \`make test\`, which carries the flag."
  fi
  if [ "$slow_hit" -gt 0 ]; then
    warn "SLOW PACKAGE(S), over the ${SLOW}s line:"
    printf '%s' "$slow_msg" | while IFS= read -r line; do [ -n "$line" ] && warn "  $line"; done
    warn "  A package this big tests as one binary and one clock; it wants splitting,"
    warn "  not a bigger -timeout. Raising the -timeout hides this, it does not fix it."
  fi
  return 0
}

# explain_timeout <logfile> — the block this script exists for.
explain_timeout() {
  local log=$1 pkg budget
  grep -q 'panic: test timed out after' "$log" || return 0
  pkg=$(awk -F'\t' '/panic: test timed out after/ { seen = 1 }
                    seen && $1 == "FAIL" && NF >= 2 { print $2; exit }' "$log")
  [ -n "$pkg" ] || pkg="(package not named in the log)"
  budget=$(sed -n 's/.*panic: test timed out after \([^ ]*\).*/\1/p' "$log" | head -1)
  {
    echo
    echo '=============================================================='
    echo "test-times: THE TEST CLOCK EXPIRED — $pkg, after ${budget:-the -timeout}"
    echo
    echo '  The goroutine dump above is the state at the moment the clock'
    echo '  ran out. That is what a real deadlock looks like AND what a'
    echo '  merely slow package looks like; the panic cannot tell them'
    echo '  apart and neither can this script. Separate them by running'
    echo '  the package alone with room to finish:'
    echo
    echo "      go test $pkg -count=1 -timeout 40m"
    echo
    echo '  Green there and slow  = the package outgrew its budget, and the'
    echo '  package times above say by how much. Still stuck there = a real'
    echo '  hang, and the dump is the evidence. Do not raise the -timeout to'
    echo '  make this go away without knowing which one it was.'
    echo '=============================================================='
  } >&2
}

self_test() {
  # tmp is deliberately NOT local: the EXIT trap that cleans it up runs in
  # global scope, where a function-local name is unset and `set -u` aborts.
  local fail=0
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/posse-test-times.XXXXXX") || return 1
  trap 'rm -rf "${tmp:?}"' EXIT
  mkdir -p "$tmp/bin"

  # A stub stands in for `go`, so every arm drives the REAL script end to end
  # — its exit status included — instead of only the report function.
  cat > "$tmp/bin/faketest" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$STUB_ARGV"
cat "$STUB_OUT"
exit "$STUB_RC"
STUB
  chmod +x "$tmp/bin/faketest"

  printf 'ok  \tgithub.com/ranger360ai/posse\t0.4s\nok  \tgithub.com/ranger360ai/posse/cmd/posse\t12.1s\nok  \tgithub.com/ranger360ai/posse/internal/rhq\t93.7s\n' > "$tmp/clean.log"
  printf 'ok  \tgithub.com/ranger360ai/posse\t0.4s\nok  \tgithub.com/ranger360ai/posse/internal/rhq\t549.3s\n' > "$tmp/slow.log"
  printf 'panic: test timed out after 25m0s\n\ngoroutine 1 [running]:\ntesting.(*M).startAlarm.func1()\nFAIL\tgithub.com/ranger360ai/posse/internal/rhq\t1500.8s\nFAIL\n' > "$tmp/timeout.log"
  # The shapes go really emits that are NOT a bare duration, from go1.26: a
  # -run that matched nothing, a cached package, a package with no test files,
  # and a coverage suffix in its own tab field.
  printf 'ok  \tgithub.com/ranger360ai/posse\t0.4s [no tests to run]\nok  \tgithub.com/ranger360ai/posse/cmd/posse\t(cached)\n?   \tgithub.com/ranger360ai/posse/x\t[no test files]\nok  \tgithub.com/ranger360ai/posse/internal/rhq\t640.0s\tcoverage: 12.0%% of statements\n' > "$tmp/shapes.log"

  local out rc
  run_arm() { # <fixture> <rc> [extra args to the fake command]
    local fixture=$1 code=$2; shift 2
    STUB_OUT="$tmp/$fixture" STUB_RC="$code" STUB_ARGV="$tmp/argv" \
      "$0" "$tmp/bin/faketest" test "$@" ./... > "$tmp/out" 2>&1
    rc=$?
    out=$(cat "$tmp/out")
  }
  ok()  { printf '  ok    %s\n' "$*"; }
  bad() { printf '  FAIL  %s\n' "$*"; fail=1; }

  echo 'test-times --self-test'

  # A: a timeout panic must produce the block and NAME the package. This is
  # the arm the script exists for.
  run_arm timeout.log 1 -timeout 25m
  case $out in
    *'THE TEST CLOCK EXPIRED'*'internal/rhq'*) ok 'timeout log: block printed and package named' ;;
    *) bad 'timeout log: no timeout block naming the package' ;;
  esac
  # Match the BLOCK's own line, not the bare duration: the fixture's panic
  # text is teed through to stdout, so `*'after 25m0s'*` was satisfied by the
  # passthrough and stayed green with the block's budget blanked out.
  case $out in
    *'internal/rhq, after 25m0s'*) ok 'timeout log: the block quotes the budget the panic named' ;;
    *) bad 'timeout log: block does not name the expired budget' ;;
  esac
  [ "$rc" = 1 ] && ok 'timeout log: exit status 1 preserved' || bad "timeout log: exit $rc, wanted 1"

  # B: the NEGATIVE control, with a positive witness. "no block printed" is
  # also what measuring nothing looks like (fm4p), so require the run to prove
  # it read all three packages before believing its silence.
  run_arm clean.log 0 -timeout 25m
  case $out in
    *'THE TEST CLOCK EXPIRED'*) bad 'clean log: timeout block printed with no panic in the log' ;;
    *) ok 'clean log: no timeout block' ;;
  esac
  case $out in
    *'3 package(s) timed, 0 over'*) ok 'clean log: witness — 3 packages parsed, none slow' ;;
    *) bad 'clean log: no witness line; the parse may have matched nothing' ;;
  esac
  case $out in
    *'SLOW PACKAGE'*) bad 'clean log: slow warning on a 93.7s package' ;;
    *) ok 'clean log: no slow warning under the line' ;;
  esac

  # C: over the visibility line, green. The warning must fire and name both
  # the package and its seconds.
  run_arm slow.log 0 -timeout 25m
  case $out in
    *'SLOW PACKAGE(S)'*'github.com/ranger360ai/posse/internal/rhq took 549.3s'*) ok 'slow log: named package and seconds' ;;
    *) bad 'slow log: no slow warning naming the package and seconds' ;;
  esac
  case $out in
    *'2 package(s) timed, 1 over'*) ok 'slow log: witness — 1 package over the line' ;;
    *) bad 'slow log: wrong or missing witness line' ;;
  esac

  # D: the shapes go actually emits. A suffixed duration must still parse (it
  # did not, first time round: field 3 was compared whole and "0.4s [no tests
  # to run]" fell through, so a real `-run` pass reported no times at all), a
  # cached package must not be counted, and a coverage suffix in its own field
  # must not hide the seconds in front of it.
  run_arm shapes.log 0 -timeout 25m
  case $out in
    *'2 package(s) timed, 1 over'*) ok 'go output shapes: suffixed and coverage lines parsed, cached and no-test-files skipped' ;;
    *) bad "go output shapes: wrong witness — $(printf '%s' "$out" | grep 'package(s) timed')" ;;
  esac
  case $out in
    *'internal/rhq took 640.0s'*) ok 'go output shapes: seconds read past a coverage field' ;;
    *) bad 'go output shapes: coverage-suffixed package not reported slow' ;;
  esac

  # E: the budget column is the COMMAND'S -timeout, not a number this script
  # keeps. Without this arm the whole "one number, and the pin can see it"
  # claim is unpinned: a hardcoded 25m would look identical on every other arm.
  run_arm clean.log 0 -timeout 25m
  case $out in
    *'against the -timeout of 1500s per package'*) ok 'budget read from the command: -timeout 25m -> 1500s' ;;
    *) bad "budget not read from the command: $(printf '%s' "$out" | grep 'package times')" ;;
  esac
  run_arm clean.log 0 -timeout=8m
  case $out in
    *'against the -timeout of 480s per package'*) ok 'budget read from the -timeout=8m spelling' ;;
    *) bad "the -timeout=<v> spelling was not read: $(printf '%s' "$out" | grep 'package times')" ;;
  esac

  # F: a command with NO -timeout is go's 10m default, and saying so is the
  # condition ranger-base-2ggb closed. A script that silently assumed a budget
  # here would report a comfortable percentage of a ceiling nobody set.
  run_arm clean.log 0
  case $out in
    *"DEFAULT 600s per package"*) ok 'no -timeout: reported against the 600s default' ;;
    *) bad "no -timeout: not reported as the default: $(printf '%s' "$out" | grep 'package times')" ;;
  esac
  case $out in
    *'carries no -timeout'*) ok 'no -timeout: warned about it' ;;
    *) bad 'no -timeout: no warning' ;;
  esac

  # G: a status this script could not produce by accident, to prove it is the
  # command's verdict being returned and not a hardcoded 0/1.
  run_arm clean.log 7 -timeout 25m
  [ "$rc" = 7 ] && ok 'exit status 7 passed through' || bad "exit status: got $rc, wanted 7"

  # H: the command is run with the arguments it was given, untouched. This
  # script must not add, drop or reorder a flag — the Makefile recipe is what
  # suitetimeout_qa_test.go reads, so what runs has to be what that pin sees.
  if [ "$(tr '\n' ' ' < "$tmp/argv")" = "test -timeout 25m ./... " ]; then
    ok 'the command is passed through unchanged'
  else
    bad "argv was rewritten: $(tr '\n' ' ' < "$tmp/argv")"
  fi

  if [ "$fail" = 0 ]; then echo 'test-times --self-test: ok'; else echo 'test-times --self-test: FAILED' >&2; fi
  return "$fail"
}

case ${1:-} in
  --self-test) self_test; exit $? ;;
  '') warn 'usage: scripts/test-times.sh <go test command...>'; exit 2 ;;
esac

budget=$(budget_of "$@") && explicit=1 || { budget=600; explicit=0; }

log=$(mktemp "${TMPDIR:-/tmp}/posse-test-times.XXXXXX") || { warn 'could not create a log file'; exit 1; }
trap 'rm -f "$log"' EXIT

"$@" | tee "$log"
status=${PIPESTATUS[0]}

report "$log" "$budget" "$explicit"
explain_timeout "$log"

exit "$status"
