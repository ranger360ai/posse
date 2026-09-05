#!/usr/bin/env bash
# test-times.sh — run a `go test` command and then say, in words, how long each
# package took and how much of its -timeout budget that was (ranger-base-7xla).
#
# Usage: scripts/test-times.sh <go test command...>    what `make test` runs
#        scripts/test-times.sh --self-test             prove the reporting works
#
# Environment:
#   SLOW_PACKAGE_SECONDS=300   the line above which a package is called slow
#   POSSE_TEST_SIGNAL_LOG=...  where the forensics of an outside signal go
#                              (default $TMPDIR/posse-test-signal.log)
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

# The box-wide suite queue (ranger-base-uvzjk). Sourced, not exec'd: the slot
# is an flock held on a file descriptor of THIS shell, so nothing new appears
# between this wrapper and `go test` — the pid this script prints below is
# still the one to kill, and the signal forensics still see the child they
# were written for.
#
# Guarded for the reason scripts/gotest.sh is: a wrapper that refuses to run
# because its QUEUE is missing is worse than one that runs unqueued and says
# so, and the loud failure for a genuinely missing file is
# `make verify-suite-lock`, a prerequisite of `make test`.
SUITE_LOCK_SIBLING=$(cd "$(dirname "$0")" && pwd)/suite-lock.sh
if [ -r "$SUITE_LOCK_SIBLING" ]; then
  . "$SUITE_LOCK_SIBLING"
else
  printf 'test-times: %s is missing — running unqueued (ranger-base-uvzjk)\n' "$SUITE_LOCK_SIBLING" >&2
  suite_lock_acquire() { :; }
  suite_lock_release() { :; }
fi

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

# ─── the signal from outside (ranger-base-6nx72) ─────────────────────────────
#
# WHAT IT LOOKS LIKE, and why it is the worst shape in this file. Twice on
# 2026-09-02 a full suite ended mid-package with nothing but
#
#     scripts/test-times.sh: line 621: 83846 Terminated: 15          "$@"
#          83849 Done                    | tee "$log"
#     make: *** [test] Terminated: 15
#
# No test is named. No package is red. The packages that DID finish are
# printed by report() above it, in the same words they use on a green run.
# Through the house filter it is a pass with a short tail — which is how one
# of the two got read as a green suite and closed a bead on it. A run that
# dies this way does not look like a red, and that is exactly the problem.
#
# WHO SENDS IT. Every occurrence traced so far — four suites across three
# sessions on 2026-09-02 — was ANOTHER SEAT ON THIS BOX, and there is only the
# one sender class. Reconstructed from the session transcripts to the
# millisecond (ranger-base-6nx72):
#
#   21:28:34Z  one seat ran `pgrep -f 'test-times.sh|go test -timeout 25m'`,
#              got back SIX pids, and ran `pkill -f test-times.sh` and
#              `pkill -f "go test -timeout 25m"` over all of them. Two of the
#              three pairs belonged to other sessions. Both of those suites
#              reported completion 1.7s later, 3 MILLISECONDS APART, from
#              launches 77 SECONDS apart — which is what one external kill
#              looks like and what no per-run clock can look like.
#   01:43:31Z  a seat ran `kill 83782 83846` on two pids read off the same
#              kind of pgrep, believing both were its own. One was not.
#
# THERE IS NO 600s HARNESS CEILING, and two of the three readers above
# concluded there was, because 598.6s of a suite that started 598.6s earlier
# is a very convincing number. MEASURED here to settle it: a
# `run_in_background` make → bash → child, output redirected to a file,
# exactly the shape that "died at the ceiling", ran 900s and exited 0
# untouched. A FOREGROUND Bash call is a different thing and really is capped
# at 600s — so a run somebody waited on in the foreground can die of its own
# caller's clock, and that is the only other sender this block admits to.
#
# Neither is the load guard, which is what both readers reached for first and
# spent an hour eliminating. Nothing in the process table says "you were
# killed"; the only witness is the process table at the instant it happened,
# and by the time anyone asks, it is gone.
#
# SO TAKE THE WITNESS. Two arms, because the killers hit two different
# processes and only one of them leaves a corpse this script can see:
#
#   - the CHILD arm: `go test` was signalled, the pipeline returns 128+N, and
#     this script is alive to say so (the 21:43Z shape).
#   - the WRAPPER arm: this script was signalled too, so there is no exit
#     status to read and a trap is the only thing that runs (the ceiling
#     shape, where the whole process group goes at once).
#
# Both write the same record and print the same block. TERM and HUP only: an
# INT is a person pressing ^C, which is the one signal nobody needs explained.
#
# WHAT THE RECORD IS FOR. A `pkill` is gone in milliseconds, but the gate
# shell that ran it is not — it holds the persona's whole Bash line in its
# argv for as long as the line runs, and its PATH carries
# `/state/gates/<persona>/bin`. So a process table taken at the signal names
# the seat. That is the whole instrument: not a guess about who, a snapshot
# of the room.
SIGNAL_LOG=${POSSE_TEST_SIGNAL_LOG:-${TMPDIR:-/tmp}/posse-test-signal.log}

# signal_name <number> — 15 -> SIGTERM. Only the ones a suite actually meets.
signal_name() {
  case $1 in
    1) echo SIGHUP ;; 2) echo SIGINT ;; 9) echo SIGKILL ;;
    15) echo SIGTERM ;; *) echo "signal $1" ;;
  esac
}

# signal_suspects — reads a process table on STDIN (pid first, then argv) and
# prints the lines that could have sent the signal. A gate shell holds the
# whole persona Bash line, so the `pkill` that did this is quotable while it
# runs, and the gate path in its PATH is the persona's name. Prints nothing
# when the table holds nothing, which is itself the answer: the harness
# ceiling kills from outside every seat and leaves no such line.
#
# It takes the table on stdin rather than running `ps` itself for two reasons
# that are the same reason — a caged verify seat is refused `ps` outright and
# has to fall back to `pgrep -fl`, and the self-test has to be able to hand it
# a table with a known sender in it. A digest nobody can feed a fixture is a
# digest nobody has ever seen name anybody.
signal_suspects() {
  local self=$1 who cmd
  echo "-- possible senders (a live kill/pkill line, and whose seat it is)"
  grep -E 'pkill|kill -|kill [0-9]' \
    | grep -vF 'pkill|kill -|kill [0-9]' \
    | while read -r pid rest; do
        # Exclude THIS run by pid and nothing else. The first spelling of this
        # excluded any row mentioning `test-times.sh`, which reads as "do not
        # accuse yourself" and is exactly backwards: this run's own row never
        # carries a kill word, while the killer's row usually names what it is
        # killing — the real 2026-09-02 line was
        # `pgrep -f 'test-times.sh|go test…'; pkill -f 'test-times.sh'`, and
        # that filter would have dropped the one line worth printing. The one
        # row that IS dropped by text is this digest's own `grep`, which `ps`
        # catches in its own pipeline and which would otherwise be named as a
        # suspect on every single event.
        [ "$pid" = "$self" ] && continue
        case $rest in
          *"/state/gates/"*) who=${rest#*/state/gates/}; who=${who%%/*} ;;
          *) who='(not a gate shell)' ;;
        esac
        # Quote the COMMAND, not the boilerplate. A gate shell's argv is a
        # ~600-character PATH preamble with the persona's actual line inside
        # `eval '…'`; printing the last 200 characters of that gives the
        # trailing shell plumbing and never the `pkill` that did this.
        cmd=$rest
        case $cmd in *"eval '"*) cmd=${cmd#*eval \'} ;; esac
        printf '      pid %s  seat %s\n        %s\n' "$pid" "$who" "${cmd:0:200}"
      done
}

# signal_capture <what> <signal number> — the snapshot, taken NOW. Called from
# the trap and from the child arm, both of which are milliseconds after the
# signal landed and both of which must not block: everything here is best
# effort and nothing here decides an exit status.
# SIGNAL_LATE is the honest caveat on the wrapper arm's clock. Bash holds a
# trap until the running command returns, so a wrapper signalled at 4s into a
# 25m run does not run its handler until the run ends — MEASURED here: a
# wrapper TERMed at 4s printed its block "120s in", which is the trap's clock
# and not the signal's. Saying "120s" flat would send the next reader to the
# wrong minute of the transcripts, which is the whole cost this block exists
# to avoid.
signal_capture() {
  local what=$1 num=$2 elapsed
  elapsed=$(( $(date +%s) - SIGNAL_START ))
  SIGNAL_ELAPSED=$elapsed
  SIGNAL_WHAT=$what
  SIGNAL_NUM=$num
  case $what in
    wrapper) SIGNAL_LATE=' (when the trap RAN; bash holds a trap until the running command returns, so the signal landed at or before this)' ;;
    *) SIGNAL_LATE='' ;;
  esac
  {
    echo "=============================================================="
    echo "$(date '+%Y-%m-%d %H:%M:%S %z')  $(signal_name "$num") to $what"
    echo "run:     pid $$ , ${elapsed}s in${SIGNAL_LATE}"
    echo "command: $SIGNAL_CMD"
    echo "load:    $(uptime 2>/dev/null | sed 's/^.*load average/load average/')"
    { ps -Awwo pid=,args= 2>/dev/null || pgrep -fl . 2>/dev/null; } | signal_suspects "$$"
    echo "-- full process table"
    ps -Awwo pid=,ppid=,pgid=,stat=,lstart=,args= 2>/dev/null \
      || pgrep -fl . 2>/dev/null \
      || echo "(no process listing from this seat: ps refused and pgrep is absent)"
  } >> "$SIGNAL_LOG" 2>&1
}

# explain_signal — the block. Printed once, after the report, so the reader who
# is looking at a wall of green package lines is told what they are not.
explain_signal() {
  [ -n "${SIGNAL_NUM:-}" ] || return 0
  {
    echo
    echo '=============================================================='
    echo "test-times: ENDED BY A SIGNAL FROM OUTSIDE — $(signal_name "$SIGNAL_NUM") to the $SIGNAL_WHAT, ${SIGNAL_ELAPSED}s in"
    [ -n "$SIGNAL_LATE" ] && echo "            ${SIGNAL_ELAPSED}s is${SIGNAL_LATE}"
    echo
    echo '  THIS IS NOT A SUITE RESULT. The package lines above are the'
    echo '  ones that had finished; every package after them was cut off'
    echo '  without a verdict, and none of that is a red. Nobody may'
    echo '  report "suite green" off this run.'
    echo
    echo '  WHO DID IT — the process table at the instant the signal'
    echo '  landed was written to'
    echo
    echo "      $SIGNAL_LOG"
    echo
    echo '  Its "possible senders" section lists every live kill or pkill'
    echo '  line on the box with the gate path that names the seat it was'
    echo '  typed in. A pkill is gone in milliseconds but the gate shell'
    echo '  that ran it holds the whole line while it runs, so the sender'
    echo '  is usually still quotable there. Start with that file.'
    echo
    echo '  If it names nobody, ask whether this run was launched in a'
    echo '  FOREGROUND Bash call: those are capped at 600s and end this'
    echo '  way at their cap. A background one is not capped — measured'
    echo "  900s untouched (ranger-base-6nx72). This run: ${SIGNAL_ELAPSED}s."
    echo
    echo '  AND WHEN YOU END A SUITE, END YOUR OWN. Every session on this'
    echo '  box runs a byte-identical argv, so `pkill -f test-times.sh`'
    echo '  and `pkill -f "go test -timeout 25m"` match all of them —'
    echo '  2026-09-02, one such line took six pids across three sessions.'
    echo '  Kill the pid the run printed when it started, and nothing else.'
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
  # Every arm below runs the REAL script, and one of them makes it record a
  # signal. Point the record at the throwaway dir before any arm runs, or a
  # self-test appends a process table to the box's own file.
  export POSSE_TEST_SIGNAL_LOG="$tmp/signal.log"

  # Every arm below hands the real script a `./...` command, which is a full
  # suite by the queue's own reading of the argv — so the arms would take and
  # release the BOX's slots, a dozen times, while a real suite waited behind
  # them. Point the queue at the throwaway dir instead, and clear any slot
  # this process inherited (arm 2 of suitelock_qa_test.go runs this from
  # inside a suite that holds one).
  export POSSE_SUITE_LOCK_DIR="$tmp/locks"
  unset POSSE_SUITE_LOCK_HELD

  # A stub stands in for `go`, so every arm drives the REAL script end to end
  # — its exit status included — instead of only the report function.
  cat > "$tmp/bin/faketest" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$STUB_ARGV"
cat "$STUB_OUT"
# The two shapes an outside kill takes (ranger-base-6nx72). `self` is the
# 21:43Z occurrence: the child dies of TERM and the wrapper lives to read
# 128+15 off the pipeline. `parent` is the ceiling shape: the wrapper is
# signalled while the child is still running, so only a trap can fire — the
# sleep holds the child open long enough that the trap is genuinely deferred,
# which is what bash really does and what the arm has to survive.
case ${STUB_SIGNAL:-} in
  self)   kill -TERM $$; sleep 5 ;;
  parent) kill -TERM "$PPID"; sleep 2 ;;
esac
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

  local out rc argv_seen=
  run_arm() { # <fixture> <rc> [extra args to the fake command]
    local fixture=$1 code=$2; shift 2
    STUB_OUT="$tmp/$fixture" STUB_RC="$code" STUB_ARGV="$tmp/argv" \
    STUB_SIGNAL="${STUB_SIGNAL:-}" \
      "$0" "$tmp/bin/faketest" test "$@" ./... > "$tmp/out" 2>&1
    rc=$?
    out=$(<"$tmp/out")
  }
  ok()  { printf '  ok    %s\n' "$*"; }
  bad() { printf '  FAIL  %s\n' "$*"; fail=1; }

  # AN ASSERTION THAT FORKS CAN REPORT THE PROPERTY FALSE BECAUSE THE APPARATUS
  # FAILED (ranger-base-7hx87). Two arms below used to read
  #
  #     if printf '%s' "$out" | grep -qE '<pattern>'; then
  #
  # and under this script's `set -o pipefail` that pipeline returns non-zero in
  # three ways that have nothing to do with $out: grep signalled (143/137),
  # grep not exec'd (127, and a fork failure under load prints only to stderr),
  # and — over a payload past the 64 KB pipe buffer — grep matching early while
  # printf takes the EPIPE (141; measured here: 0/100 at 65000 bytes, 92/100 at
  # 66000). All three print the arm's "no such line" text over an $out that
  # plainly carries the line, which is the once-in-~30-runs shape seen on
  # 2026-09-02 at load ~20 with three sessions on the box. Locale was the other
  # reading and it is ruled out: the em dash is the same three bytes in the
  # pattern and in the text, and it matched under en_US.UTF-8, C, POSIX,
  # en_US.ISO8859-1, an invalid locale and no LANG at all.
  #
  # So these two arms match with bash's own `case` and `${...}`: no fork, no
  # pipe, no temp file, nothing between the bytes and the verdict.

  # line_after <needle> — sets $seen to the text following the first occurrence of
  # <needle>, up to the end of that line; returns 1 when $out has no <needle>.
  # It reads the FIRST such line, which is deliberately stricter than the ERE
  # it replaces: a placeholder on the first DISK line no longer passes because
  # a later line is well formed.
  local seen=
  line_after() {
    local rest=${out#*"$1"}
    if [ "$rest" = "$out" ]; then seen=; return 1; fi
    seen=${rest%%$'\n'*}
    return 0
  }
  # digits <s> — non-empty and all digits, so "$$" verbatim or a placeholder
  # cannot stand in for a number the box produced.
  digits() { case ${1:-} in ''|*[!0-9]*) return 1 ;; esac; return 0; }
  # dump_out — the whole of $out, verbatim and as bytes. The one 2026-09-02
  # sighting was inconclusive because the arm printed a single grepped line;
  # anything invisible in the text (an encoding artefact, a truncation) has to
  # be in the record the next time this fails. Forks, but only on the way to a
  # failure that has already been decided.
  # indented <text> — every line of <text> behind the block's gutter. bash's
  # own, not `| sed 's/^/…/'` (ranger-base-t07yx): this runs on the way to a
  # failure that has already been decided, so a dying sed could not flip a
  # verdict — but it could blank the ONE record the next reader has of a
  # failure that only happens under load, which is exactly how the 2026-09-02
  # sighting stayed inconclusive.
  indented() {
    local line
    while IFS= read -r line || [ -n "$line" ]; do
      printf '        | %s\n' "$line"
    done <<<"$1"
  }
  dump_out() {
    local bytes
    printf '        --- $out verbatim (%s bytes) ---\n' "${#out}"
    indented "$out"
    printf '        --- $out as bytes ---\n'
    # od is the diagnostic itself, not a matcher, and it stays.
    bytes=$(printf '%s' "$out" | od -c 2>/dev/null)
    indented "$bytes"
    printf '        --- end ---\n'
  }

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
  # `$(<file)` and a bash substitution, not `$(tr … < file)` (ranger-base-t07yx):
  # a `tr` that is signalled or cannot be exec'd made this arm report the argv
  # as REWRITTEN — measured, with a dying `tr` on PATH it printed
  # "FAIL argv was rewritten: " over an argv file that was perfectly correct.
  # $(<…) drops the trailing newline, so the wanted string loses its trailing
  # space too.
  argv_seen=$(<"$tmp/argv")
  argv_seen=${argv_seen//$'\n'/ }
  if [ "$argv_seen" = "test -timeout 25m ./..." ]; then
    ok 'the command is passed through unchanged'
  else
    bad "argv was rewritten: $argv_seen"
  fi


  # I: the DISK line is printed BEFORE the packages, on every run, and carries
  # a number df actually produced rather than a placeholder.
  run_arm clean.log 0 -timeout 25m
  local disk_ok=0 mb= rest= want_mnt= got_mnt= df_out= df_row=
  if line_after 'test-times: DISK: '; then
    mb=${seen%% *}
    rest=${seen#* }
    case $rest in
      'MB free on '?*' — t.TempDir'*) digits "$mb" && disk_ok=1 ;;
    esac
  fi
  if [ "$disk_ok" = 1 ]; then
    ok 'disk: the preflight line names free MB, the filesystem and what fills it'
  else
    bad 'disk: no DISK line naming free MB, the filesystem and t.TempDir'
    dump_out
  fi

  # …and it is df's reading, not a constant. Cross-checked on the MOUNT POINT
  # rather than on the megabytes: the filesystem behind $TMPDIR does not move
  # under a running suite, while its free bytes do — a tolerance on the number
  # would be exactly the box-mood red this script exists to avoid. That the
  # number is a number and that it is compared to the floor are arms I and J.
  #
  # NEITHER SIDE OF THIS COMPARISON MAY BE PARSED BY A FORK (ranger-base-t07yx).
  # This arm used to read the mount point out of $out with
  # `printf | sed -n | head -1`, one line below the two conditions 0b5c1c4 had
  # just de-forked for exactly this reason, and it was measured failing in the
  # same way: with a `sed` on PATH whose body is `kill -TERM $$`, ONE run of
  # this self-test printed the arm above as ok and this one as "the line names
  # ''". The apparatus had failed and the arm called the script wrong.
  #
  # The rule the whole file now follows: RUN the thing under test — df here,
  # since asking the box is the measurement — but decide with bash's own
  # `${...}`, so nothing between the bytes and the verdict can be signalled or
  # fail to exec.
  #
  # Parsing df here in bash rather than with mount_of()'s awk also makes this a
  # genuinely independent reading. The old line used awk with the same NR==2
  # && NF>=2 { print $NF } expression the product code uses, so an awk that
  # agreed with itself was most of what this arm proved.
  want_mnt=
  df_out=$(df -kP "${TMPDIR:-/tmp}" 2>/dev/null)
  df_row=${df_out#*$'\n'}      # drop the header line
  if [ "$df_row" != "$df_out" ]; then
    df_row=${df_row%%$'\n'*}   # …and keep only the first filesystem
    want_mnt=${df_row##* }      # -P guarantees the mount point is last
  fi
  got_mnt=
  if line_after 'test-times: DISK: '; then
    rest=${seen#*' MB free on '}
    [ "$rest" != "$seen" ] && got_mnt=${rest%%' — '*}
  fi
  if [ -z "$want_mnt" ]; then
    # Said as an apparatus failure, not as a property failure, because that
    # distinction is the entire point of this block: df produced no table, so
    # this arm did not test the DISK line at all.
    bad 'disk: df printed no table for $TMPDIR — THIS ARM DID NOT RUN; nothing was learned about the DISK line'
    dump_out
  elif [ "$got_mnt" = "$want_mnt" ]; then
    ok "disk: the line names the filesystem df attributes \$TMPDIR to ($want_mnt)"
  else
    bad "disk: the line names '$got_mnt'; df says \$TMPDIR is on '$want_mnt'"
    dump_out
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

  # S: the run announces its own pid, because the alternative to knowing it is
  # a `pgrep` that lists every session's suite (which is how both 2026-09-02
  # runs died). The arm requires the REAL pid of the run, not the word "pid":
  # a line reading "$$" verbatim would satisfy any looser match.
  run_arm clean.log 0 -timeout 25m
  local pid_ok=0 pid=
  if line_after 'this run is pid '; then
    pid=${seen%% *}
    rest=${seen#* }
    # The same pid twice: the line is useless if the number it tells you to
    # kill is not the number it says the run is, and the ERE this replaces
    # matched two unrelated integers.
    case $rest in
      "— stop it with \`kill $pid\`"*) digits "$pid" && pid_ok=1 ;;
    esac
  fi
  if [ "$pid_ok" = 1 ]; then
    ok 'run line: names a real pid, the same one, and how to stop this run'
  else
    bad 'run line: no pid line naming one real pid and the kill for it'
    dump_out
  fi
  case $out in
    *'never with a `pkill -f` pattern'*) ok 'run line: says why a pattern kill is wrong here' ;;
    *) bad 'run line: does not warn off pkill' ;;
  esac

  # N: the outside signal, child arm (ranger-base-6nx72) — the 21:43Z shape,
  # where `go test` is killed and this script lives to read 128+15 off the
  # pipeline. The stub really signals itself: nothing here simulates a status.
  local before after
  # The record does not exist until something writes it, and `wc -c <` on a
  # missing file is a redirection error the `|| echo 0` never sees.
  record_size() { if [ -f "$tmp/signal.log" ]; then wc -c < "$tmp/signal.log"; else echo 0; fi; }
  before=$(record_size)
  STUB_SIGNAL=self run_arm clean.log 0 -timeout 25m
  case $out in
    *'ENDED BY A SIGNAL FROM OUTSIDE'*'SIGTERM to the go test child'*) ok 'signal child: block printed and names the signal and what it hit' ;;
    *) bad 'signal child: no block naming SIGTERM and the child' ;;
  esac
  case $out in
    *'THIS IS NOT A SUITE RESULT'*) ok 'signal child: refuses the run as a suite result' ;;
    *) bad 'signal child: does not say the run is not a result' ;;
  esac
  [ "$rc" = 143 ] && ok 'signal child: exit status 143 preserved' || bad "signal child: exit $rc, wanted 143"
  case $out in
    *'when the trap RAN'*) bad 'signal child: hedges a clock that is exact — the pipeline returned at the signal' ;;
    *) ok 'signal child: reports its elapsed without the trap caveat' ;;
  esac
  after=$(record_size)
  [ "$after" -gt "$before" ] && ok 'signal child: the record grew' || bad 'signal child: nothing was written to the record'
  # The last of the arms that forked to decide (ranger-base-7hx87): `read -d ''`
  # slurps the record with a builtin, so a grep that is signalled or cannot be
  # exec'd can no longer report the record empty. It returns 1 at EOF with the
  # record read, which is why its status is not the test.
  local record=
  IFS= read -r -d '' record < "$tmp/signal.log" 2>/dev/null
  # ANCHORED AT THE START OF A LINE, and that is the whole arm. The record's
  # second half is `ps -Awwo args=` — every command line on the box — so a bare
  # substring test is satisfied by any process whose argv happens to quote the
  # marker, this arm's own reader included: deleting the `-- full process table`
  # header left the arm green while a shell running `grep 'full process table'`
  # sat in the dump (measured 2026-09-03). ps cannot put a newline in a row, so
  # a line that STARTS with the header is one this script wrote.
  if [[ $record == *$'\n''-- full process table'$'\n'* &&
        $record == *$'\n''-- possible senders'* ]]; then
    ok 'signal child: the record holds a process table and a suspect section'
  else
    bad 'signal child: the record has no process table'
  fi

  # O: the wrapper arm — the ceiling shape, where this script is signalled
  # while the child runs on. Only the trap can fire, and it fires LATE by
  # design; the arm proves it fires at all and that the signal is re-raised
  # rather than swallowed (a wrapper that survived its own TERM would report a
  # green suite to make, which is the whole failure this bead is about).
  # The arm ends with a job dying of TERM, and this shell announces that on
  # its own stderr ("… Terminated: 15 …"). The arm's own output already goes
  # to $tmp/out, so nothing else can be lost in here.
  { STUB_SIGNAL=parent run_arm clean.log 0 -timeout 25m; } 2>/dev/null
  case $out in
    *'ENDED BY A SIGNAL FROM OUTSIDE'*'SIGTERM to the wrapper'*) ok 'signal wrapper: block printed from the trap' ;;
    *) bad 'signal wrapper: the trap printed no block' ;;
  esac
  case $out in
    *'when the trap RAN'*) ok 'signal wrapper: the elapsed is flagged as the trap clock, not the signal clock' ;;
    *) bad 'signal wrapper: reports a deferred trap clock as the time of death' ;;
  esac
  [ "$rc" = 143 ] && ok 'signal wrapper: the signal is re-raised, not swallowed' || bad "signal wrapper: exit $rc, wanted 143"

  # P: the NEGATIVE control for both arms. A run nobody signalled must print no
  # block and must not touch the record — "no block" is also what a broken
  # detector looks like, so N above is its positive witness and this is the
  # other half.
  before=$(record_size)
  run_arm clean.log 0 -timeout 25m
  case $out in
    *'ENDED BY A SIGNAL FROM OUTSIDE'*) bad 'unsignalled run: signal block printed on a run nobody killed' ;;
    *) ok 'unsignalled run: no signal block' ;;
  esac
  # And the arm that matters more, because it is the common case: a suite with
  # REDS in it exits 1 and has not been signalled by anybody. A detector that
  # fires on any nonzero status would call every failing run a kill.
  run_arm enospc.log 1 -timeout 25m
  case $out in
    *'ENDED BY A SIGNAL FROM OUTSIDE'*) bad 'failing run: a red suite was reported as killed from outside' ;;
    *) ok 'failing run: exit 1 is a red, not a signal' ;;
  esac
  after=$(record_size)
  [ "$after" = "$before" ] && ok 'unsignalled run: the record was not touched' || bad 'unsignalled run: wrote a record with no signal'

  # R: the digest that actually names a sender, fed a table with one in it.
  # The fixture is the real 2026-09-02 shape: a sibling seat's gate shell
  # holding the pkill line in its argv, its PATH naming the persona. Three
  # things have to survive: the line is selected, the seat is read out of the
  # gate path, and the innocent rows are left out.
  local table suspects
  # Verbatim shapes from 2026-09-02: jian-yang's line NAMES test-times.sh
  # (it is what it is killing), dinesh's is a bare `kill <pid>` off a pgrep,
  # 502 is an innocent bystander and 504 is this run itself.
  table="  501 /bin/zsh -c _rgp=; PATH=/Users/x/.config/posse/state/gates/jian-yang/bin:\$PATH; eval 'pgrep -f test-times.sh; pkill -f test-times.sh; pkill -f \"go test -timeout 25m\"'
  502 /Applications/Safari.app/Contents/MacOS/Safari
  503 /bin/zsh -c PATH=/Users/x/.config/posse/state/gates/dinesh/bin; eval 'kill 83782 83846'
  504 bash scripts/test-times.sh /opt/homebrew/bin/go test -timeout 25m ./...
  505 /bin/zsh -c eval 'make test; kill -0 505'"
  # A here-string, not `printf | …` (ranger-base-t07yx): the pipeline forked a
  # printf only to hand over a string this shell already holds, and a printf
  # that cannot be forked under load would empty $suspects and make all three
  # arms below report the digest as naming nobody. signal_suspects itself is
  # the thing under test, so ITS forks stay.
  suspects=$(signal_suspects 505 <<<"$table")
  case $suspects in
    *'seat jian-yang'*) ok 'suspects: the pkill line is selected and its seat named' ;;
    *) bad 'suspects: the pkill line did not name jian-yang' ;;
  esac
  case $suspects in
    *'seat dinesh'*) ok 'suspects: a bare `kill <pid>` line is selected too' ;;
    *) bad 'suspects: the bare kill line was missed' ;;
  esac
  case $suspects in
    *Safari*) bad 'suspects: an unrelated process was listed as a sender' ;;
    *) ok 'suspects: leaves the unrelated rows out' ;;
  esac
  # `ps` catches this digest's own grep in its own pipeline, and a suspect
  # section that names itself on every event teaches the reader to skip it.
  case $(signal_suspects 505 <<<"$table"$'\n''  506 grep -E pkill|kill -|kill [0-9]') in
    *'506'*) bad 'suspects: the digest lists its own grep as a sender' ;;
    *) ok 'suspects: does not name its own grep' ;;
  esac
  # The killer's line NAMES test-times.sh, so "mentions test-times.sh" can
  # never be the exclusion — only this run's own pid can be.
  case $suspects in
    *'pkill -f test-times.sh'*) ok 'suspects: keeps a killer line that names the suite it killed' ;;
    *) bad 'suspects: dropped the killer line because it mentioned test-times.sh' ;;
  esac
  # The command, not the 600 characters of PATH preamble it is buried in.
  case $suspects in
    *'PATH=/Users/x/.config'*) bad 'suspects: quotes the gate preamble instead of the command' ;;
    *) ok 'suspects: quotes the command out of the gate shell, not its preamble' ;;
  esac
  case $suspects in
    *'pid 505'*) bad 'suspects: lists the run itself as its own killer' ;;
    *) ok 'suspects: excludes this run by pid' ;;
  esac

  # Q: the block has to be actionable, not just loud — the two things a
  # reader does next are open the record and stop the run by pid. $out holds
  # the LAST arm's output, so this arm signals a run of its own rather than
  # reading a block that three arms ago left lying around.
  STUB_SIGNAL=self run_arm clean.log 0 -timeout 25m
  case $out in
    *"$tmp/signal.log"*) ok 'signal block: names the record path to open' ;;
    *) bad 'signal block: does not say where the process table went' ;;
  esac
  case $out in
    *'END YOUR OWN'*'Kill the pid the run printed'*) ok 'signal block: says how to end a suite without hitting siblings' ;;
    *) bad 'signal block: does not give the safe way to kill a run' ;;
  esac
  case $out in
    *'A background one is not capped'*) ok 'signal block: does not sell the 600s ceiling that measurement killed' ;;
    *) bad 'signal block: lost the measured statement about the cap' ;;
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

# WHOSE RUN THIS IS (ranger-base-6nx72). Every session on this box runs a
# suite with a byte-identical argv, so `pgrep -f test-times.sh` answers "which
# suites are running" and NEVER "which one is mine" — on 2026-09-02 one
# `pkill -f` line matched six pids across three sessions and ended all three,
# and a `kill <pid>` off the same pgrep ended a fourth. The pid is printed
# here so the session that started this run can stop THIS run later without
# going back to the process table to guess.
printf 'test-times: this run is pid %s — stop it with `kill %s`, never with a `pkill -f` pattern: every session on this box runs the same argv\n' "$$" "$$"

# The witness for an outside signal (ranger-base-6nx72). SIGNAL_START and
# SIGNAL_CMD are read by signal_capture from whichever arm fires; the traps
# re-raise so that `make` still reports Terminated and the exit status is the
# one it always was — this arm records, it does not swallow.
SIGNAL_START=$(date +%s)
SIGNAL_CMD="$*"
SIGNAL_NUM=""
SIGNAL_WHAT=""
SIGNAL_ELAPSED=0
SIGNAL_LATE=""
trap 'signal_capture "wrapper" 15; explain_signal; trap - TERM; kill -TERM $$' TERM
trap 'signal_capture "wrapper" 1;  explain_signal; trap - HUP;  kill -HUP  $$' HUP

# THE QUEUE (ranger-base-uvzjk), after the pid line and after the traps: a run
# that waits ten minutes for a slot must already have said which pid it is and
# must already be able to record a signal, or the wait is indistinguishable
# from a hang and the forensics have a window with nothing in them. A run that
# is not a full suite is not queued and does not notice this line.
suite_lock_acquire "$@"

# ...and the DISK line AFTER the queue, so its number describes the run that
# is about to start rather than the box as it was when this run got in line.
disk_preflight "$1"

"$@" | tee "$log"
status=${PIPESTATUS[0]}

# The slot goes back the moment `go test` is done. The reporting below reads a
# log file and costs milliseconds, but a slot is a slot: the next suite in the
# queue should not wait on it.
suite_lock_release

# 128+N is a child that died of signal N. `go test` exits 1 for reds and 2 for
# a build error, so nothing in the normal range reaches here.
if [ "$status" -gt 128 ]; then signal_capture "go test child" "$(( status - 128 ))"; fi

report "$log" "$budget" "$explicit"
explain_timeout "$log"
explain_disk "$log"
explain_std_break "$log"
explain_signal

exit "$status"
