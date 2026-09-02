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
#
# AND THE OTHER ENVIRONMENTAL RED: THE DISK (ranger-base-krra). The clock is
# not the only thing this box runs out of. MEASURED 2026-08-29 on the machine
# that runs every session's suite: `make test` came back exit 2 with ~80 reds
# in internal/rhq, every one of them
#
#     --- FAIL: TestWatchPidRoundTrip (0.00s)
#         testing.go:1426: TempDir: mkdir /var/folders/.../TestWatchPid...:
#             no space left on device
#
# with 231Mi free, a 41G go build cache and 670 leaked Test* dirs going back
# two days. `t.TempDir()` calls `t.Fatal` on ENOSPC, so ONE full filesystem is
# reported ONCE PER TEST that wanted a temp dir: through the house filter
# (`grep -E '^(---|ok|FAIL)'`) it is a list of unrelated test names — worktree,
# watch, dispatch, merge — and reads exactly like a broken change. The word
# `disk` appears nowhere a reader looks first.
#
# So this script does for the disk what it already does for the clock, in the
# same two places and on the same terms:
#
#   BEFORE the packages run, one line saying how much room they have, because
#   the cheapest time to learn the box is full is before the ten minutes; and
#   AFTER a run whose log carries ENOSPC, a block that names the cause, so the
#   reader who is already staring at eighty red test names is told which of
#   them belong to the box.
#
# It does not delete anything and it does not go red on free space. What to
# clear on a shared box is the operator's call — `go clean -cache` slows every
# concurrent session and deleting from /var/folders can take a live test's
# TempDir out from under it — and a floor is a guess about a box, which is the
# same class of red DISK and CLOCK are here to explain rather than to throw.
set -uo pipefail

SLOW=${SLOW_PACKAGE_SECONDS:-300}

# The floor below which the DISK line becomes a warning. MEASURED, not picked:
# see the block above `disk_preflight`.
DISK_FLOOR_MB=${DISK_FLOOR_MB:-5120}

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

# ------------------------------------------------------------------- the disk
#
# THE FLOOR IS MEASURED, and the measurement is one run, so read it as a floor
# and not as a budget. A full green `go test -timeout 25m ./...` on this box
# (darwin 25.4.0, go1.26, 2026-08-29, `df -kP` sampled across the whole 830s
# run) took the filesystem behind TMPDIR from 35489 MB free to a low of
# 34586 MB: 903 MB CONSUMED BY ONE RUN, of which the build cache was +686 MB
# (5116 -> 5802 MB) and t.TempDir churn +77 MB. The box is shared and other
# sessions write the same volume, so 903 MB is an upper reading of one run,
# not a clean-room one.
#
# 5120 MB is that peak times ~5.7, and the multiplier is the two things one
# run does not measure: this box routinely has two or three sessions' suites
# running at once on the same volume, and a build cache that has just been
# cleared has to regrow this repo's working set (5.8 GB, measured above) —
# which is exactly the state a disk-pressure cleanup leaves behind, so the run
# after the cleanup is the one most likely to be short. Below 5120 MB, one run
# probably still fits and the ones around it may not.
#
# It is a WARNING and never a failure. A floor is a claim about a box, and this
# script's whole charter is that a red belonging to the box is the bug it
# exists to prevent, not one to introduce.

# avail_mb <path> — free megabytes on the filesystem holding <path>, or empty
# when df cannot say. `df -kP` and not `df -k`: the POSIX table is one line per
# filesystem (a long device name wraps onto its own line without it) with Avail
# in field 4 and the mount point last, on both darwin and linux, which the
# default tables are not.
avail_mb() {
  df -kP "$1" 2>/dev/null | awk 'NR==2 && NF>=4 && $4 ~ /^[0-9]+$/ { printf "%d\n", $4 / 1024 }'
}

# mount_of <path> — the mount point df attributes <path> to.
mount_of() {
  df -kP "$1" 2>/dev/null | awk 'NR==2 && NF>=2 { print $NF }'
}

# disk_line <path> <what lives there> — one DISK line. Returns 1 under the
# floor, so the caller can warn once for however many filesystems there were.
disk_line() {
  local path=$1 label=$2 mb mnt
  mb=$(avail_mb "$path")
  mnt=$(mount_of "$path")
  if [ -z "$mb" ]; then
    printf 'test-times: DISK: free space unreadable for %s (%s)\n' "$path" "$label"
    return 0
  fi
  printf 'test-times: DISK: %s MB free on %s — %s\n' "$mb" "${mnt:-?}" "$label"
  [ "$mb" -ge "$DISK_FLOOR_MB" ]
}

# disk_preflight <go-binary> — said BEFORE the packages run, because the
# cheapest moment to learn the box is full is not ten minutes into the suite.
#
# Two directories decide whether a `go test` run fits: TMPDIR, where every
# t.TempDir() lands, and GOCACHE, which grows for the whole run. On this box
# they are the same APFS volume and this prints one line; they are not required
# to be, so it asks df rather than assuming. GOCACHE comes from the go binary
# the run will actually use — `$(GOBIN)`, not necessarily the `go` on PATH —
# and is skipped unless the answer names a directory that exists, which is also
# what makes this safe to run under a stub in --self-test.
disk_preflight() {
  local gobin=$1 tmp cache tmnt cmnt low=0

  tmp=${TMPDIR:-/tmp}
  cache=$("$gobin" env GOCACHE 2>/dev/null)
  [ -n "$cache" ] && [ -d "$cache" ] || cache=""

  tmnt=$(mount_of "$tmp")
  cmnt=""
  [ -n "$cache" ] && cmnt=$(mount_of "$cache")

  if [ -n "$cache" ] && [ "$cmnt" = "$tmnt" ]; then
    disk_line "$tmp" 't.TempDir and the go build cache' || low=1
  else
    disk_line "$tmp" 't.TempDir' || low=1
    if [ -n "$cache" ]; then
      disk_line "$cache" 'the go build cache' || low=1
    fi
  fi

  [ "$low" = 0 ] && return 0
  warn "DISK BELOW THE ${DISK_FLOOR_MB} MB FLOOR — a red from this run may belong to the"
  warn "  box rather than to the diff. Every t.TempDir() allocates there, and go reports"
  warn "  ENOSPC as an ordinary test failure once per test that wanted one: ~80"
  warn "  unrelated-looking reds naming worktree, watch and dispatch (ranger-base-krra)."
  warn "  Running anyway. What to clear on a shared box is the operator's call, not this"
  warn "  script's: \`go clean -cache\` slows every concurrent session and deleting from"
  warn "  \$TMPDIR can take a live test's TempDir out from under it."
  return 0
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
    warn "  What has actually moved the number twice is concurrency inside the one"
    warn "  binary: \`go run ./cmd/testparallel <pkg>\` counts the tests that can take"
    warn "  t.Parallel and \`envroots\` names the helpers holding the rest serial, worst"
    warn "  first. \`make verify-parallel\` is the gate that keeps the marked set from"
    warn "  decaying again (ranger-base-pj87l)."
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

# explain_disk <logfile> — the block ranger-base-krra exists for. ENOSPC does
# not name the disk anywhere the reader looks first; it names eighty tests.
explain_disk() {
  local log=$1 hits reds
  grep -q 'no space left on device' "$log" || return 0
  hits=$(grep -c 'no space left on device' "$log")
  reds=$(grep -c '^--- FAIL' "$log")
  {
    echo
    echo '=============================================================='
    echo "test-times: THE DISK RAN OUT — ${hits} failure(s) said \"no space left on device\""
    [ "$reds" -gt 0 ] && echo "            (this run reported ${reds} failing test(s) in total)"
    echo
    echo '  Those reds belong to the box, not to the diff. t.TempDir()'
    echo '  calls t.Fatal on ENOSPC — and a test that merely WRITES into'
    echo '  one fails the same way — so ONE full filesystem is reported'
    echo '  ONCE PER TEST that wanted a temp dir. Through the house'
    echo "  filter it is a list of unrelated names (worktree, watch,"
    echo '  dispatch, merge) and reads exactly like a broken change.'
    echo '  Believe none of them until the suite has room and has re-run.'
    echo
    echo '  Where it goes, measured on this box (ranger-base-krra):'
    echo
    echo '      df -h "${TMPDIR:-/tmp}"'
    echo '      du -sh "$(go env GOCACHE)"          # reached 41G unattended'
    echo '      ls -d "${TMPDIR:-/tmp}"/Test* | wc -l   # 670 leaked TempDirs'
    echo
    echo '  Clearing either is the operator call this script will not make'
    echo '  for them: `go clean -cache` slows every concurrent session, and'
    echo '  deleting from $TMPDIR can take a live test'"'"'s TempDir away.'
    echo '=============================================================='
  } >&2
}

# explain_std_break <logfile> — the same bad hour wearing a different face. A
# std package reported missing FROM std is not a broken toolchain; MEASURED
# 2026-08-29 it appeared minutes after the build cache was emptied underneath a
# running build, and went away on its own — `go build ./...` was clean
# afterwards with no toolchain change. Worth naming because "package iter is
# not in std" sends a reader to reinstall go.
explain_std_break() {
  local log=$1 line
  line=$(grep -m1 -E 'package [a-z0-9_/]+ is not in std' "$log") || return 0
  {
    echo
    echo '=============================================================='
    echo 'test-times: A STD PACKAGE WAS REPORTED MISSING FROM std'
    echo
    echo "      ${line# }"
    echo
    echo '  Your go install is almost certainly fine. MEASURED on this box'
    echo '  (ranger-base-krra, 2026-08-29): this shape appeared in the'
    echo '  minutes after the build cache was emptied under a running'
    echo '  build, and went away by itself — `go build ./...` was clean'
    echo '  afterwards, same toolchain. It arrives as `[setup failed]`,'
    echo '  which is a BUILD error and so names no test at all.'
    echo
    echo '  Re-run the suite once before touching the toolchain:'
    echo
    echo '      go build ./... && make test'
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

  # The disk's two faces (ranger-base-krra). The ENOSPC fixture is the shape
  # measured on 2026-08-29 verbatim: the disk is named only inside a t.Fatal
  # message, and the test names on the --- FAIL lines have nothing to do with
  # storage. The std-break fixture is the same hour's build failure.
  printf -- '--- FAIL: TestWatchPidRoundTrip (0.00s)\n    testing.go:1426: TempDir: mkdir /var/folders/dy/x/T/TestWatchPidRoundTrip2288449404: no space left on device\n--- FAIL: TestDispatchMergeBack (0.00s)\n    testing.go:1426: TempDir: mkdir /var/folders/dy/x/T/TestDispatchMergeBack118: no space left on device\nFAIL\tgithub.com/ranger360ai/posse/internal/rhq\t12.3s\nFAIL\n' > "$tmp/enospc.log"
  printf -- '/opt/homebrew/Cellar/go/1.26.5/libexec/src/bytes/iter.go:8:2: package iter is not in std\nFAIL\tgithub.com/ranger360ai/posse/internal/rhq [setup failed]\nFAIL\n' > "$tmp/stdbreak.log"

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


  # I: the DISK line is printed BEFORE the packages, on every run, and carries
  # a number df actually produced rather than a placeholder.
  run_arm clean.log 0 -timeout 25m
  if printf '%s' "$out" | grep -qE 'DISK: [0-9]+ MB free on .+ — t\.TempDir'; then
    ok 'disk: the preflight line names free MB, the filesystem and what fills it'
  else
    bad "disk: no DISK line — $(printf '%s' "$out" | grep -i disk | head -1)"
  fi

  # …and it is df's reading, not a constant. Cross-checked on the MOUNT POINT
  # rather than on the megabytes: the filesystem behind $TMPDIR does not move
  # under a running suite, while its free bytes do — a tolerance on the number
  # would be exactly the box-mood red this script exists to avoid. That the
  # number is a number and that it is compared to the floor are arms I and J.
  want_mnt=$(df -kP "${TMPDIR:-/tmp}" 2>/dev/null | awk 'NR==2 && NF>=2 { print $NF }')
  got_mnt=$(printf '%s' "$out" | sed -n 's/.*DISK: [0-9]* MB free on \(.*\) — .*/\1/p' | head -1)
  if [ -n "$want_mnt" ] && [ "$got_mnt" = "$want_mnt" ]; then
    ok "disk: the line names the filesystem df attributes \$TMPDIR to ($want_mnt)"
  else
    bad "disk: the line names '$got_mnt'; df says \$TMPDIR is on '$want_mnt'"
  fi

  # J: the floor. BOTH arms, because "no warning" is also what a floor that is
  # never consulted looks like: the same fixture, the same box, the same free
  # space, and only the floor moved.
  DISK_FLOOR_MB=1 run_arm clean.log 0 -timeout 25m
  case $out in
    *'DISK BELOW THE'*) bad 'disk floor: warned with the floor at 1 MB' ;;
    *) ok 'disk floor: silent above the floor' ;;
  esac
  DISK_FLOOR_MB=999999999 run_arm clean.log 0 -timeout 25m
  case $out in
    *'DISK BELOW THE 999999999 MB FLOOR'*) ok 'disk floor: warns below the floor, and quotes the floor it used' ;;
    *) bad 'disk floor: no warning with the floor above any real disk — the floor is not consulted' ;;
  esac
  case $out in
    *'Running anyway'*) ok 'disk floor: says it is running anyway' ;;
    *) bad 'disk floor: does not say the run continues' ;;
  esac

  # K: the ENOSPC block, which is the whole bead. It must fire, count the
  # ENOSPC lines rather than the tests, and leave the exit status alone.
  run_arm enospc.log 1 -timeout 25m
  case $out in
    *'THE DISK RAN OUT'*'2 failure(s) said "no space left on device"'*) ok 'enospc: block printed and ENOSPC failures counted' ;;
    *) bad 'enospc: no block naming the count' ;;
  esac
  case $out in
    *'2 failing test(s) in total'*) ok 'enospc: the run total is reported beside the ENOSPC count' ;;
    *) bad 'enospc: no total-reds line' ;;
  esac
  case $out in
    *"operator call this script will not make"*) ok 'enospc: names the cleanup as the operator call, not the script'"'"'s' ;;
    *) bad 'enospc: does not say who clears the disk' ;;
  esac
  [ "$rc" = 1 ] && ok 'enospc: exit status 1 preserved' || bad "enospc: exit $rc, wanted 1"

  # L: the negative control for both new blocks, over a log with neither
  # string in it. A block that fires on a clean run is worse than none.
  run_arm clean.log 0 -timeout 25m
  case $out in
    *'THE DISK RAN OUT'*) bad 'clean log: ENOSPC block printed with no ENOSPC in the log' ;;
    *) ok 'clean log: no ENOSPC block' ;;
  esac
  case $out in
    *'REPORTED MISSING FROM std'*) bad 'clean log: std-break block printed on a clean log' ;;
    *) ok 'clean log: no std-break block' ;;
  esac

  # M: the build failure that reads as a broken toolchain. It must quote the
  # offending line, so the reader can see it is the same one they just read.
  run_arm stdbreak.log 1 -timeout 25m
  case $out in
    *'A STD PACKAGE WAS REPORTED MISSING FROM std'*'package iter is not in std'*) ok 'std break: block printed and the compiler line quoted' ;;
    *) bad 'std break: no block quoting the offending line' ;;
  esac
  case $out in
    *'before touching the toolchain'*) ok 'std break: sends the reader to re-run, not to reinstall go' ;;
    *) bad 'std break: no re-run advice' ;;
  esac

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

disk_preflight "$1"

"$@" | tee "$log"
status=${PIPESTATUS[0]}

report "$log" "$budget" "$explicit"
explain_timeout "$log"
explain_disk "$log"
explain_std_break "$log"

exit "$status"
