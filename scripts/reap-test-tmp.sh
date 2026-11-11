#!/usr/bin/env bash
# reap-test-tmp.sh — collect the temp HOMEs a killed test binary left behind
# (ranger-base-32bdv).
#
# internal/posse's TestMain gives the whole test tree ONE temp HOME and
# removes it when m.Run returns. Two of the parent's exits never get there: a
# `-timeout` panic exits from a watchdog goroutine, and a signal does not
# unwind at all. Those leave one `posse-testhome-*` dir per run, and on this
# box /tmp is a 14G tmpfs mounted usrquota that every seat shares as uid
# posse — one box-wide budget, so an uncollected pile is every seat's red
# (EDQUOT), not just the leaker's.
#
# Usage:
#       scripts/reap-test-tmp.sh                 what it WOULD remove
#       scripts/reap-test-tmp.sh --apply         remove it
#       scripts/reap-test-tmp.sh --root DIR      a TMPDIR other than /tmp,
#                                                repeatable
#       scripts/reap-test-tmp.sh --min-age MIN   keep anything younger
#                                                (default 60 minutes)
#
# A dir is collected only when BOTH hold:
#   - no live process references it — through HOME, TMPDIR, RHQ_HOME or
#     RHQ_QA_HOLD_HOME in its environment, its cwd, or any open fd; and
#   - its mtime is at least --min-age minutes old.
#
# Dry run by default, because the thing it deletes is another seat's run if
# either test is wrong.
#
# THE SCAN CAN ONLY BE TRUSTED WHEN IT IS NOT BLIND, so it proves it is not
# before deleting anything. It reads /proc, which hands over a process's
# environ only to that process's own uid — so on a box where the runs belong
# to somebody else, every dir reads "unheld" and the sweep eats live work
# while looking clean. The control is therefore the CAPABILITY and not a
# count of what was found: it counts the same-uid processes whose environ it
# can actually open, and refuses outright if that is zero while other
# same-uid processes exist. "Nothing is held" then means nothing is held.
#
# It is deliberately not "a test binary is running but I see no temp HOME".
# That reads as a blind scan on a perfectly sighted box, because the binary
# running is usually treepins.test, which never makes one — and a matcher
# over every process's argv finds ".test" in the text of this script, too
# (AGENTS.md, "Ending anything", has the same lesson about pattern kills).
set -euo pipefail

apply=0 min_age=60 roots=()
while [ $# -gt 0 ]; do
  case "$1" in
    --apply)   apply=1; shift ;;
    --root)    roots+=("${2:?--root needs a directory}"); shift 2 ;;
    --min-age) min_age="${2:?--min-age needs minutes}"; shift 2 ;;
    -h|--help) awk 'NR>1 && /^#/ {sub(/^# ?/, ""); print; next} NR>1 {exit}' "$0"; exit 0 ;;
    *) echo "reap-test-tmp.sh: unknown argument: $1" >&2; exit 2 ;;
  esac
done
# Default: /tmp AND this shell's $TMPDIR when that is somewhere else. The
# dirs belong to the RUNS, not to whoever sweeps — a seat working around the
# quota puts its temp HOME under $HOME and leaves one there on the same two
# exits, so a default of $TMPDIR alone would sweep the one root that is not
# the problem and report the other clean.
if [ ${#roots[@]} -eq 0 ]; then
  roots=(/tmp)
  [ -n "${TMPDIR:-}" ] && [ "${TMPDIR%/}" != "/tmp" ] && roots+=("${TMPDIR%/}")
fi
case "$min_age" in ''|*[!0-9]*) echo "reap-test-tmp.sh: --min-age wants a whole number of minutes, got '$min_age'" >&2; exit 2 ;; esac

[ -d /proc/self ] || { echo "reap-test-tmp.sh: no /proc — this reads process state and will not guess without it" >&2; exit 2; }

# Every temp-HOME path any live process names, with no root filter: the sweep
# set is drawn from this, and so is the positive control.
refs=$(
  for pid in /proc/[0-9]*; do
    tr '\0' '\n' < "$pid/environ" 2>/dev/null | sed -n 's/^\(HOME\|TMPDIR\|RHQ_HOME\|RHQ_QA_HOLD_HOME\)=//p'
    readlink "$pid/cwd" 2>/dev/null
    for fd in "$pid"/fd/*; do readlink "$fd" 2>/dev/null; done
  done 2>/dev/null | grep -oE '/[^[:space:]]*/posse-testhome-[0-9]+' | sort -u
) || true

me=$(id -u) others=0 readable=0
for pid in /proc/[0-9]*; do
  [ "$pid" = "/proc/$$" ] && continue
  [ "$(stat -c %u "$pid" 2>/dev/null)" = "$me" ] || continue
  others=$((others + 1))
  if (exec 3< "$pid/environ") 2>/dev/null; then readable=$((readable + 1)); fi
done
held_n=$(printf '%s\n' "$refs" | grep -c . || true)
echo "same-uid processes: $others   environs readable: $readable   live temp HOMEs seen: $held_n"
[ "$held_n" -gt 0 ] && printf '%s\n' "$refs" | sed 's/^/  HELD /'
if [ "$readable" -eq 0 ] && [ "$others" -gt 0 ]; then
  echo "reap-test-tmp.sh: $others same-uid processes and not one environ could be opened — the scan is blind here, refusing to delete" >&2
  exit 3
fi

held=$(mktemp); printf '%s\n' "$refs" | grep . > "$held" || true
trap 'rm -f "$held"' EXIT
stale=$(mktemp); trap 'rm -f "$held" "$stale"' EXIT
for root in "${roots[@]}"; do
  [ -d "$root" ] || continue
  find "$root" -maxdepth 1 -type d -name 'posse-testhome-*' -mmin "+$min_age" -print 2>/dev/null
done | sort -u | { grep -vxF -f "$held" || true; } > "$stale"

n=$(grep -c . < "$stale" || true)
if [ "$n" -eq 0 ]; then echo "nothing to collect (roots: ${roots[*]}, min-age ${min_age}m)"; exit 0; fi
echo "collectable: $n dirs, $(du -sh --total $(cat "$stale") 2>/dev/null | tail -1 | cut -f1) (roots: ${roots[*]}, min-age ${min_age}m)"
du -sm $(cat "$stale") 2>/dev/null | awk '$1>1 {print "  " $1 "M  " $2}' | sort -rn || true

if [ "$apply" -eq 0 ]; then echo "dry run — pass --apply to remove them"; exit 0; fi
xargs -a "$stale" -r -d '\n' rm -rf --
echo "collected $n dirs"
