#!/usr/bin/env bash
# reap-selftest-ambient.sh — run `reap-test-tmp.sh --self-test` with AMBIENT
# claude-exe processes arriving all the way through it, and say whether its
# verdicts moved (ranger-base-0whbv).
#
# Usage:
#     scripts/reap-selftest-ambient.sh                    3 runs of the tree's own self-test
#     scripts/reap-selftest-ambient.sh --runs 5
#     scripts/reap-selftest-ambient.sh --script /path/to/another/reap-test-tmp.sh
#     scripts/reap-selftest-ambient.sh --interval 2 --cap 20
#
# Environment: none. Every knob is a flag, so the command a bead quotes is the
# command that ran.
#
# WHAT IT IS FOR. reap-test-tmp.sh's finding-4 rig asks whether a `claude -p`
# run is excluded from the blindness control's process count. That count is
# BOX-WIDE on purpose — every seat here is a claude process and the control
# wants all of them — so a rig that grades a DIFFERENCE of two such counts
# grades whatever the shop did between its two samples. It did exactly that
# until ranger-base-0wvu6, and it red `make test` for a seat that had changed
# nothing (verify-reap-test-tmp is a prerequisite): 3 of 8 back-to-back runs
# on 2026-11-12, naming the reaper and no test the reader could act on. The
# rig grades MEMBERSHIP of its own planted pid now, in the pid list the
# counter reports from the same loop as the count.
#
# This harness is what says that out loud. It cannot be done by waiting for a
# sibling seat to launch, which is why four beads in this family re-measured
# it by hand (ranger-base-2smyx, -0wvu6, -bttuj, -0whbv).
#
# WHY A RAMP AND NOT A CHURN. The arrivals are MONOTONIC — one new process a
# second, none of them killed until the run ends — so any two box-wide samples
# taken a second apart differ, and a delta-shaped verdict is wrong by
# construction rather than by luck. An on/off churn only reds when a
# transition happens to land between the two samples.
# MEASURED 2026-11-13 (linux, this box, 13 real claude seats live, loadavg
# 1.7-2.1), against the rig as it stood at 7725ba75^ — the last tree with the
# delta spelling — and against this tree's rig:
#
#   perturbation          pre-fix rig        this tree's rig
#   none                  0 of 8 red         0 of 8 red
#   on/off churn, 1-3s    2 of 6 red         0 of 8 red
#   this ramp             3 of 3 red         0 of 5 red
#
# So: a clean run proves nothing (the pre-fix rig passes 8 of 8 on a quiet
# minute), a churn proves something a third of the time, and the ramp reds the
# old spelling every time it is run. 26-35s per run either way at that tree.
#
# RE-MEASURED 2026-11-13 at a8575914 (same box, 16 cores, loadavg 2.0-2.9),
# after ranger-base-hxenz cut the self-test from 23-35s to 7-9s: the ramp still
# reds 7725ba75^ 3 of 3 and passes HEAD 3 of 3, but it now costs 8s and 8
# arrivals per run here, against 24-25s and 24-25 arrivals at the pre-fix tree
# -- which predates that cut, and is why that column did not move. The arrival
# count is not a knob: it is one process per second of the run this harness
# perturbs, so it fell with that wall and will rise again if the rig gets
# slower. Both halves of the "half a minute and 25-35 processes" figure every
# bead in this family quoted are the pre-hxenz reading (ranger-base-tq5tj).
#
# IT IS NOBODY'S PREREQUISITE, and that is deliberate. Even at 8s it would
# spend every seat's `make test` on 8 planted claude-exe processes to
# re-answer a question the tree's own membership arms already answer
# deterministically and for free -- and what was unprotected was never the
# BEHAVIOUR but the EDIT, which is held in the suite now by a shape pin that
# reads one file in milliseconds (internal/treepins/ambientcensus_qa_test.go,
# ranger-base-tq5tj). Type this when you have EDITED the finding-4 rig, or
# when you are judging a report that reads like a box-wide count: that pin is
# a proxy for this harness, not a substitute for it.
#
# BLAST RADIUS. One copy of tee(1), named `claude` under this run's own
# `mktemp -d` root, exec'd up to --cap times; each process idles on a fifo
# this harness holds open and is killed BY PID when the run ends (never a
# pattern — AGENTS.md "Ending anything"). They burn no CPU. The only reach
# outside this process tree is that a reaper sweep running on another seat
# counts them toward claude_procs, which can only push that run toward
# REFUSING to collect the claude-scratch class — the control's own fail-safe
# direction — and never toward taking something it otherwise would keep.
# If this harness is SIGKILLed, the fakes are not orphaned: the write end of
# the fifo lives in THIS shell only (every child gets the read end and has fd
# 9 closed), so the last thing to happen when it dies is EOF on their stdin,
# and tee with no file operand exits on EOF.
set -u

self=${0##*/}
script=scripts/reap-test-tmp.sh
runs=3 interval=1 cap=40

die() { echo "$self: $*" >&2; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --script)   script=${2:?--script needs a path}; shift 2 ;;
    --runs)     runs=${2:?--runs needs a count}; shift 2 ;;
    --interval) interval=${2:?--interval needs seconds}; shift 2 ;;
    --cap)      cap=${2:?--cap needs a count}; shift 2 ;;
    -h|--help)  sed -n '2,70p' "$0"; exit 0 ;;
    *)          die "unknown argument: $1 (see --help)" ;;
  esac
done
[ -x "$script" ] || die "$script is not an executable self-test to run"
case "$runs$interval$cap" in *[!0-9]*) die "--runs, --interval and --cap take non-negative integers" ;; esac
[ "$runs" -gt 0 ] || die "--runs must be at least 1"
[ "$cap" -gt 0 ] || die "--cap must be at least 1"

teebin=$(command -v tee) || die "no tee(1) on PATH to copy for the fake claude binary"

# A REAL ELF NAMED "claude", not a symlink: /proc/<pid>/exe resolves through a
# symlink to its target, so a link named claude reads back as whatever it
# points at and the counter never sees it (reap-test-tmp.sh's own note). tee
# and not sleep because the fake has to SURVIVE idling: `tee` with no file
# operand blocks on stdin, and `sleep -p` is an invalid option (the vacuity
# ranger-base-bttuj found in the rig this harness perturbs).
root=$(mktemp -d "${TMPDIR:-/tmp}/posse-reap-ambient.XXXXXX") || die "mktemp failed"
cp -- "$teebin" "$root/claude" || die "could not copy $teebin"
chmod +x "$root/claude"
mkfifo "$root/hold" || die "could not make the holding fifo"
# Read-write, so neither open blocks and nothing is ever written: fd 9 is the
# only writer and it lives in this shell alone. fd 8 is the read end the
# children get.
exec 9<> "$root/hold" || die "could not open the holding fifo"
exec 8< "$root/hold" || die "could not open the holding fifo for read"

fakes=()
reap_fakes() {
  [ ${#fakes[@]} -gt 0 ] || return 0
  kill "${fakes[@]}" 2>/dev/null
  wait "${fakes[@]}" 2>/dev/null
  fakes=()
}
cleanup() { reap_fakes; exec 9>&- 8>&-; rm -rf -- "${root:?}"; }
trap 'cleanup; exit 130' INT TERM
trap cleanup EXIT

echo "$self: $runs run(s) of $script --self-test, one ambient claude-exe arrival every ${interval}s (cap $cap)"
echo "$self: fake binary $root/claude (a copy of $teebin)"

red=0
for r in $(seq 1 "$runs"); do
  log="$root/run$r.log"
  start=$SECONDS
  "$script" --self-test > "$log" 2>&1 & stpid=$!
  arrivals=0
  while kill -0 "$stpid" 2>/dev/null && [ "$arrivals" -lt "$cap" ]; do
    # fd 9 closed in the child on purpose: see BLAST RADIUS above.
    "$root/claude" <&8 9>&- >/dev/null 2>&1 & fakes+=($!)
    arrivals=$((arrivals + 1))
    sleep "$interval"
  done
  wait "$stpid"; rc=$?
  reap_fakes

  # The report is read with bash's own `read`/`case` and not a forked
  # `grep -c`: a matcher that could not be forked under the load this harness
  # is deliberately adding would report zero FAIL lines over a red run, which
  # is the one answer this tool must never give (ranger-base-t07yx, and
  # internal/treepins/selftestforkarm_qa_test.go holds the rule for arms like
  # these).
  nfail=0 first=""
  while IFS= read -r line; do
    case "$line" in
      "  FAIL"*) nfail=$((nfail + 1)); [ -n "$first" ] || first=$line ;;
    esac
  done < "$log"
  echo "run $r: exit $rc  wall $((SECONDS - start))s  ambient arrivals $arrivals  FAIL arms $nfail"
  if [ "$nfail" -gt 0 ]; then
    red=$((red + 1))
    while IFS= read -r line; do
      case "$line" in "  FAIL"*) echo "    $line" ;; esac
    done < "$log"
  fi
  # A nonzero exit with no FAIL arm is its own answer: the self-test died
  # rather than judged, and the log is the only place that says how.
  if [ "$rc" -ne 0 ] && [ "$nfail" -eq 0 ]; then
    red=$((red + 1))
    echo "    exit $rc and no FAIL arm — the self-test did not finish judging. Its whole output:"
    while IFS= read -r line; do echo "      $line"; done < "$log"
  fi
done

if [ "$red" -eq 0 ]; then
  echo "$self: $runs of $runs run(s) green under ambient arrivals — the verdicts are not reading the box"
  exit 0
fi
echo "$self: $red of $runs run(s) moved under ambient arrivals — a verdict here is grading processes this rig does not own"
exit 1
