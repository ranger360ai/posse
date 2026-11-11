#!/usr/bin/env bash
# reap-test-tmp.sh — collect the scratch dirs a killed run left behind in a
# shared /tmp (ranger-base-32bdv; widened to every leak class by
# ranger-base-y9t65).
#
# WHY THIS EXISTS. On this box /tmp is a 14G tmpfs mounted usrquota with
# nr_inodes=1048576, and every seat is uid posse — one box-wide budget for
# bytes AND for inodes, so an uncollected pile is every seat's red (EDQUOT,
# or a linker that cannot create a file), not just the leaker's. The seat
# that spends it is never the seat that reds. ranger-base-vw1cp measured
# 974889 inodes in 24679 dirs, 93% of the table.
#
# Usage:
#       scripts/reap-test-tmp.sh                 what it WOULD remove
#       scripts/reap-test-tmp.sh --apply         remove it
#       scripts/reap-test-tmp.sh --root DIR      a TMPDIR other than /tmp,
#                                                repeatable
#       scripts/reap-test-tmp.sh --min-age MIN   one grace for EVERY class,
#                                                overriding the table below
#       scripts/reap-test-tmp.sh --class NAME    only this class, repeatable
#       scripts/reap-test-tmp.sh --pressure-at PCT      default 70
#       scripts/reap-test-tmp.sh --pressure-min-age MIN default 60
#       scripts/reap-test-tmp.sh --bead-at PCT          default 85
#       scripts/reap-test-tmp.sh --bead-repo DIR        where to file it
#       scripts/reap-test-tmp.sh --no-bead       never file one
#       scripts/reap-test-tmp.sh --quiet         totals only, no per-dir list
#
# ─── WHAT IT TAKES, AND WHY EACH CLASS HAS ITS OWN GRACE ────────────────────
#
# A dir is collected only when BOTH hold:
#   - no live process names it — through ANY value in its environment, its
#     argv, its cwd, its exe, or any open fd; and
#   - its mtime is at least its class's grace old.
#
# THE SCAN IS A BEST-EFFORT NEGATIVE AND THE GRACE IS THE REAL GATE. ADR 0089
# measured why: Go's os.Setenv does not rewrite /proc/<pid>/environ, so a test
# binary that made a dir and pointed HOME at it is INVISIBLE to the scan, and
# a shell's `d=$(mktemp -d)` is invisible for the same reason one step over —
# the path lives in a shell variable, which is not argv, not cwd and not an
# fd. So every class's grace below is set against the longest run that could
# still be holding one of its dirs, and the scan only ever makes the answer
# more conservative. ADR 0089 D1-D2 closes this for posse-testhome-* by
# having the creator hold a flock the kernel releases at death; when
# ranger-base-z056i lands, the `flock -n` step goes in the candidate filter
# below, ahead of the held-set filter, and that class's grace stops being
# load-bearing. No other class has a creator we can make take a lock, so for
# them the grace is all there is — which is ADR 0089's own objection to
# grace, stated here rather than hidden.
#
#   class           glob                 grace   why that number
#   ───────────────────────────────────────────────────────────────────────
#   posse-testhome  posse-testhome-*      60m    internal/posse TestMain's
#                                                one temp HOME per test tree;
#                                                the original class, grace
#                                                unchanged from 32bdv.
#   posse-scratch   posse-*              120m    every other posse-named
#                                                scratch root — the shell
#                                                `mktemp -d` roots in
#                                                scripts/ and the Go
#                                                os.MkdirTemp roots in the
#                                                tests (posse-race-*,
#                                                posse-coldexec-*,
#                                                posse-cache-fence-*,
#                                                posse-l3-probe-*, ...). 120m
#                                                is 2x the longest -timeout
#                                                anything here carries (60m,
#                                                the coldexec-check arms).
#   go-build        go-build[0-9]*       120m    Go's $WORK. A RUNNING test
#                                                binary's /proc/<pid>/exe
#                                                names it, so this one the
#                                                scan can actually see; the
#                                                grace is the same 2x bound
#                                                for the compile-only case.
#   go-testtmp      Test*[0-9]           120m    t.TempDir() with TMPDIR=/tmp.
#                                                Bounded by the same -timeout.
#   mktemp          tmp.??????????      1440m    mktemp(1)'s DEFAULT template,
#                                                and therefore a namespace we
#                                                do NOT own: an operator's
#                                                hand-made scratch dir in a
#                                                terminal left open has no
#                                                bound at all. 24h, and it is
#                                                the first grace the pressure
#                                                arm drops.
#
# WHAT IT WILL NOT TAKE, AND WHY (ranger-base-y9t65):
#   - /tmp/claude-1001/..., the harness's per-session scratchpad root, which
#     was MEASURED 2026-11-11 at 44826 inodes — 43% of everything /tmp was
#     holding, the single biggest item on the filesystem. Neither gate can
#     reach it. A Claude Code SESSION is not a process: it sits idle between
#     turns with nothing on this box referencing its dir, and its mtime does
#     not move while it waits, so a dormant live session and a dead one read
#     identically. It is also the harness's directory and not posse's —
#     nothing here knows which session ids are live. Reclaiming it needs a
#     liveness signal from the harness side (the session's own transcript
#     under ~/.claude/projects is one candidate), which is ranger-base-4arlo.
#   - /tmp/node-compile-cache, which is a CACHE with a stable name, not a
#     leak. It regenerates, it does not accumulate, and taking it buys an
#     inode table nothing and costs a recompile.
#   - anything not owned by this uid. `find -uid` is on every candidate
#     walk, which is what keeps the generic `tmp.*` namespace from reaching
#     root's or another user's dirs.
#
# ─── THE BLINDNESS CONTROL ──────────────────────────────────────────────────
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
# It is deliberately not "a test binary is running but I see no scratch dir".
# That reads as a blind scan on a perfectly sighted box, because the binary
# running is usually treepins.test, which never makes one — and a matcher
# over every process's argv finds ".test" in the text of this script, too
# (AGENTS.md, "Ending anything", has the same lesson about pattern kills).
#
# ─── INODES, NOT JUST BYTES ─────────────────────────────────────────────────
#
# The near-outage this script now runs on a timer for was an INODE
# exhaustion, not a byte one: 9137 four-inode fixture dirs cost 42261 inodes
# and 36M, while `df -h` read 29% through the whole thing. So every run
# reports both, and the arms below fire on whichever of the two is worse:
#   - at or above --pressure-at, every class's grace drops to
#     --pressure-min-age for this run, and the run says so;
#   - if it is STILL at or above --bead-at after an --apply, a P1 bead is
#     filed on the devops lane, once per --bead-cooldown hours (a stamp
#     file, because a timer that files a bead every 15 minutes is a worse
#     outage than the one it is reporting).
#
# Dry run by default, because the thing it deletes is another seat's run if
# either gate is wrong.
set -euo pipefail

self=$(basename "$0")
apply=0 min_age="" roots=() want_classes=() quiet=0
pressure_at=70 pressure_min_age=60 bead_at=85 bead_repo="" no_bead=0 bead_cooldown=24

die2() { echo "$self: $*" >&2; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --apply)   apply=1; shift ;;
    --root)    roots+=("${2:?--root needs a directory}"); shift 2 ;;
    --min-age) min_age="${2:?--min-age needs minutes}"; shift 2 ;;
    --class)   want_classes+=("${2:?--class needs a name}"); shift 2 ;;
    --quiet)   quiet=1; shift ;;
    --pressure-at)      pressure_at="${2:?--pressure-at needs a percentage}"; shift 2 ;;
    --pressure-min-age) pressure_min_age="${2:?--pressure-min-age needs minutes}"; shift 2 ;;
    --bead-at)          bead_at="${2:?--bead-at needs a percentage}"; shift 2 ;;
    --bead-repo)        bead_repo="${2:?--bead-repo needs a directory}"; shift 2 ;;
    --bead-cooldown)    bead_cooldown="${2:?--bead-cooldown needs hours}"; shift 2 ;;
    --no-bead)          no_bead=1; shift ;;
    -h|--help) awk 'NR>1 && /^#/ {sub(/^# ?/, ""); print; next} NR>1 {exit}' "$0"; exit 0 ;;
    *) die2 "unknown argument: $1" ;;
  esac
done

# Default: /tmp AND this shell's $TMPDIR when that is somewhere else. The
# dirs belong to the RUNS, not to whoever sweeps — a seat working around the
# quota puts its scratch under $HOME and leaves one there on the same two
# exits, so a default of $TMPDIR alone would sweep the one root that is not
# the problem and report the other clean.
if [ ${#roots[@]} -eq 0 ]; then
  roots=(/tmp)
  [ -n "${TMPDIR:-}" ] && [ "${TMPDIR%/}" != "/tmp" ] && roots+=("${TMPDIR%/}")
fi
for pair in "min-age:$min_age" "pressure-at:$pressure_at" "pressure-min-age:$pressure_min_age" "bead-at:$bead_at" "bead-cooldown:$bead_cooldown"; do
  nm=${pair%%:*} val=${pair#*:}
  [ -z "$val" ] && continue
  case "$val" in ''|*[!0-9]*) die2 "--$nm wants a whole number, got '$val'" ;; esac
done

[ -d /proc/self ] || die2 "no /proc — this reads process state and will not guess without it"

# ─── the class table ────────────────────────────────────────────────────────
# name|glob|grace in minutes. Order matters only for reporting: a dir that
# two classes both match (posse-testhome-* is also posse-*) is reported under
# the first that accepts it, and the union is what gets removed.
class_table='posse-testhome|posse-testhome-*|60
posse-scratch|posse-*|120
go-build|go-build[0-9]*|120
go-testtmp|Test*[0-9]|120
mktemp|tmp.??????????|1440'

if [ ${#want_classes[@]} -gt 0 ]; then
  for c in "${want_classes[@]}"; do
    printf '%s\n' "$class_table" | cut -d'|' -f1 | grep -qxF "$c" ||
      die2 "no such class '$c' — have: $(printf '%s\n' "$class_table" | cut -d'|' -f1 | paste -sd' ' -)"
  done
fi

# ─── bytes and inodes, before ───────────────────────────────────────────────
# df's own percentages, per root, and the worst of the two over every root —
# that worst is what the pressure and bead arms read. `df -P` so a long
# device name cannot wrap the row out from under awk.
worst_pct=0 worst_why=""
fs_report() {
  local when="$1" r b i
  for r in "${roots[@]}"; do
    [ -d "$r" ] || continue
    b=$(df -P "$r"  2>/dev/null | awk 'NR==2 {gsub(/%/,"",$5); print $5+0}')
    i=$(df -Pi "$r" 2>/dev/null | awk 'NR==2 {gsub(/%/,"",$5); print $5+0}')
    [ -n "$b" ] || b=0
    [ -n "$i" ] || i=0
    echo "$when $r: bytes ${b}% used, inodes ${i}% used"
    if [ "$b" -gt "$worst_pct" ]; then worst_pct=$b; worst_why="bytes on $r"; fi
    if [ "$i" -gt "$worst_pct" ]; then worst_pct=$i; worst_why="inodes on $r"; fi
  done
}
fs_report "before:"
before_pct=$worst_pct before_why=$worst_why

# THE PRESSURE ARM. An explicit --min-age is the caller's number and is never
# overridden; without one, crossing --pressure-at replaces every class's
# grace with --pressure-min-age for this run. The 24h `mktemp` grace is what
# this is mostly for: a burst reaches exhaustion inside a few hours (one
# arm-2 run was priced at 310 dirs and 12200 inodes by ranger-base-b27pn),
# and a 24h quarantine cannot reach a burst. A 60m floor still covers every
# bounded run on this box by its own -timeout.
pressure=0
if [ -z "$min_age" ] && [ "$worst_pct" -ge "$pressure_at" ]; then
  pressure=1
  echo "PRESSURE: $worst_why is at ${worst_pct}% (>= ${pressure_at}%) — every class's grace drops to ${pressure_min_age}m for this run"
fi

# ─── every path a live process names, folded onto a top-level candidate ─────
# Widened from 32bdv's four named variables to EVERY value in the environ,
# plus argv and exe (ranger-base-y9t65). A shell script's `d=$(mktemp -d)`
# is in none of them, but the path it hands a child as an argument IS in that
# child's cmdline, and a running test binary built into Go's $WORK is named
# by its own /proc/<pid>/exe. Both are cheap and both only ever make the
# answer more conservative.
#
# Everything is folded to the one top-level entry under a root that would
# contain it, because the candidate set is -maxdepth 1 under those roots: a
# ref anywhere inside /tmp/tmp.ABCDEFGHIJ/r/.git holds /tmp/tmp.ABCDEFGHIJ.
rawrefs=$(
  for pid in /proc/[0-9]*; do
    tr '\0' '\n' < "$pid/environ" 2>/dev/null
    tr '\0' '\n' < "$pid/cmdline" 2>/dev/null
    readlink "$pid/cwd" 2>/dev/null
    readlink "$pid/exe" 2>/dev/null
    for fd in "$pid"/fd/*; do readlink "$fd" 2>/dev/null; done
  done 2>/dev/null | grep -oE '/[^[:space:]:,="'"'"']+' | sort -u
) || true

held=$(mktemp); stale=$(mktemp); rootlist=$(mktemp)
trap 'rm -f "$held" "$stale" "$rootlist"' EXIT

printf '%s\n' "${roots[@]}" > "$rootlist"
printf '%s\n' "$rawrefs" | awk -v rootfile="$rootlist" '
  BEGIN { n = 0; while ((getline line < rootfile) > 0) if (line != "") R[++n] = line }
  {
    for (i = 1; i <= n; i++) {
      r = R[i]
      if (index($0, r "/") == 1) {
        rest = substr($0, length(r) + 2)
        s = index(rest, "/")
        if (s > 0) rest = substr(rest, 1, s - 1)
        if (rest != "") print r "/" rest
        break
      }
    }
  }' | sort -u > "$held"

# ─── the blindness control ──────────────────────────────────────────────────
me=$(id -u) others=0 readable=0
for pid in /proc/[0-9]*; do
  [ "$pid" = "/proc/$$" ] && continue
  [ "$(stat -c %u "$pid" 2>/dev/null)" = "$me" ] || continue
  others=$((others + 1))
  if (exec 3< "$pid/environ") 2>/dev/null; then readable=$((readable + 1)); fi
done
held_n=$(grep -c . < "$held" || true)
echo "same-uid processes: $others   environs readable: $readable   candidates a live process names: $held_n"
if [ "$held_n" -gt 0 ] && [ "$quiet" -eq 0 ]; then sed 's/^/  HELD /' < "$held"; fi
if [ "$readable" -eq 0 ] && [ "$others" -gt 0 ]; then
  echo "$self: $others same-uid processes and not one environ could be opened — the scan is blind here, refusing to delete" >&2
  exit 3
fi

# ─── the candidate walk, class by class ─────────────────────────────────────
# ADR 0089 D2's `flock -n "$d" true` test goes HERE, ahead of the held-set
# filter, once ranger-base-z056i lands: a busy lock is proof the creator is
# alive, and it makes posse-testhome-*'s grace stop carrying the whole
# weight.
: > "$stale"
while IFS='|' read -r cname cglob cgrace; do
  [ -n "$cname" ] || continue
  if [ ${#want_classes[@]} -gt 0 ]; then
    printf '%s\n' "${want_classes[@]}" | grep -qxF "$cname" || continue
  fi
  grace=$cgrace
  [ -n "$min_age" ] && grace=$min_age
  [ "$pressure" -eq 1 ] && grace=$pressure_min_age
  found=$(
    for root in "${roots[@]}"; do
      [ -d "$root" ] || continue
      find "$root" -maxdepth 1 -mindepth 1 -type d -uid "$me" -name "$cglob" -mmin "+$grace" -print 2>/dev/null
    done | sort -u | { grep -vxF -f "$held" || true; } | { grep -vxF -f "$stale" || true; }
  )
  cn=$(printf '%s\n' "$found" | grep -c . || true)
  echo "class $cname ($cglob, grace ${grace}m): $cn collectable"
  [ "$cn" -gt 0 ] && printf '%s\n' "$found" | grep . >> "$stale"
done <<< "$class_table"

n=$(grep -c . < "$stale" || true)
if [ "$n" -eq 0 ]; then
  echo "nothing to collect (roots: ${roots[*]})"
else
  bytes=$(xargs -a "$stale" -r -d '\n' du -sc --block-size=1M -- 2>/dev/null | tail -1 | cut -f1)
  inodes=$(xargs -a "$stale" -r -d '\n' find -- 2>/dev/null | grep -c . || true)
  echo "collectable: $n dirs, ${bytes:-?}M, ${inodes:-?} inodes (roots: ${roots[*]})"
  if [ "$quiet" -eq 0 ]; then
    xargs -a "$stale" -r -d '\n' du -sm -- 2>/dev/null | awk '$1>1 {print "  " $1 "M  " $2}' | sort -rn | head -20 || true
  fi
fi

if [ "$apply" -eq 0 ]; then
  echo "dry run — pass --apply to remove them"
  exit 0
fi

if [ "$n" -gt 0 ]; then
  xargs -a "$stale" -r -d '\n' rm -rf --
  echo "collected $n dirs"
fi

# ─── bytes and inodes, after ────────────────────────────────────────────────
worst_pct=0 worst_why=""
fs_report "after: "
echo "worst: ${before_pct}% ($before_why) -> ${worst_pct}% ($worst_why)"

# ─── the bead arm ───────────────────────────────────────────────────────────
# Still over the line after taking everything the gates allow: that is a leak
# the reaper cannot keep up with, and it wants a person. Filed at most once
# per --bead-cooldown hours, because a 15-minute timer filing a P1 every tick
# is a worse outage than the one it is reporting. Every failure here is soft:
# a sweep that worked must not exit non-zero because the tracker was busy.
[ "$worst_pct" -ge "$bead_at" ] || exit 0
echo "OVER: $worst_why is at ${worst_pct}% (>= ${bead_at}%) after a full sweep"
[ "$no_bead" -eq 0 ] || { echo "--no-bead: not filing"; exit 0; }
if ! command -v bd >/dev/null 2>&1; then
  echo "$self: the tracker CLI is not on PATH — nothing filed. Add it to the unit's Environment=PATH=." >&2
  exit 0
fi
[ -n "$bead_repo" ] || bead_repo=$(cd "$(dirname "$0")/.." && pwd)

# THE REPO THAT OWNS THE STORE, which is not always the repo you are in.
# `--bead-repo` names a checkout; its `.beads` may be a one-line `redirect`
# at a store in a DIFFERENT repository, which is the shape on this box
# (a checkout whose .beads holds only a redirect at a queue repo's store).
# That matters
# because in no-db mode the tracker resolves $BEADS_DIR else $cwd/.beads and
# reads no redirect at all (ADR 0055) — so a chdir to the checkout with the
# variable shed would file into the redirect stub, invisibly. And handing it
# a store-selecting flag instead is the one thing AGENTS.md forbids: on the
# pinned version that flag was MEASURED importing the cwd repo's issues.jsonl
# into the named store (github issue 9, ranger-base-9mjxb).
#
# So: resolve the store, then name the repository that owns it, by
# scripts/prune-bd-relates-to.sh's three conditions — the store is a `.beads`
# directory, its parent is a git work tree's TOPLEVEL, and that parent's own
# `.beads` resolves BACK to this store. Refuse rather than guess.
store="$bead_repo/.beads"
if [ -f "$store/redirect" ]; then store=$(tr -d '[:space:]' < "$store/redirect"); fi
owner=""
if [ -d "$store" ] && [ "$(basename "$store")" = ".beads" ]; then
  cand=$(cd "$store/.." 2>/dev/null && pwd -P) || cand=""
  if [ -n "$cand" ]; then
    top=$(git -C "$cand" rev-parse --show-toplevel 2>/dev/null) || top=""
    [ -n "$top" ] && top=$(cd "$top" && pwd -P)
    if [ "$top" = "$cand" ]; then
      back="$cand/.beads"
      [ -f "$back/redirect" ] && back=$(tr -d '[:space:]' < "$back/redirect")
      if [ -d "$back" ] && [ "$(cd "$back" && pwd -P)" = "$(cd "$store" && pwd -P)" ]; then
        owner=$cand
      fi
    fi
  fi
fi
if [ -z "$owner" ]; then
  echo "$self: cannot name the repository that owns the store $bead_repo/.beads resolves to — nothing filed. A store is <repo>/.beads where <repo> is a git work tree's toplevel and <repo>/.beads resolves back to it; name that checkout with --bead-repo." >&2
  exit 0
fi
bead_repo=$owner
stamp="${XDG_CACHE_HOME:-$HOME/.cache}/posse/reap-test-tmp.bead-stamp"
mkdir -p "$(dirname "$stamp")" 2>/dev/null || true
if [ -f "$stamp" ] && [ -z "$(find "$stamp" -mmin "+$((bead_cooldown * 60))" -print 2>/dev/null)" ]; then
  echo "one was filed within the last ${bead_cooldown}h ($stamp) — not filing another"
  exit 0
fi
title="/tmp is at ${worst_pct}% ($worst_why) after a full reap — the leak is outrunning the reaper"
{
  echo "Filed by scripts/reap-test-tmp.sh on its timer (ranger-base-y9t65)."
  echo
  echo "$worst_why read ${worst_pct}% AFTER a sweep that took everything both"
  echo "gates allow, so this is not a reaper that has not run — it is a producer"
  echo "making scratch dirs faster than the graces release them, or a class the"
  echo "table does not name."
  echo
  echo "The sweep that preceded this took $n dirs:"
  head -20 < "$stale" | sed 's/^/  /'
  echo
  echo "Next: census the roots by class, find the class that is not in"
  echo "scripts/reap-test-tmp.sh's table, and either add it with a grace argued"
  echo "from the longest run that could hold one, or say why it cannot be reaped"
  echo "safely. The two classes the script refuses on purpose, and why, are in"
  echo "its header."
} > "$stale.body"
# The tracker reads the DIRECTORY, so the cd is the whole of the store
# selection and no store-selecting flag is handed over (AGENTS.md, github
# issue 9); and BEADS_DIR is shed for the call so this session's value cannot
# silently redirect it somewhere else.
newid=$( (cd "$bead_repo" && env -u BEADS_DIR bd create "$title" -t bug -p 1 -l devops --body-file "$stale.body") 2>&1 |
         grep -oE '[a-z][a-z0-9]*-[a-z0-9]+-[a-z0-9]+' | head -1 ) || true
rm -f "$stale.body"
if [ -n "$newid" ]; then
  date > "$stamp"
  echo "filed $newid"
  # An exit 0 and a printed line are not the record (ADR 0073): read it back.
  (cd "$bead_repo" && env -u BEADS_DIR bd show "$newid") >/dev/null 2>&1 ||
    echo "WARNING: $newid did not read back — look for it by title in the open list before filing again" >&2
else
  echo "$self: no id was printed — nothing recorded. Look for this title in the open list before filing by hand:" >&2
  echo "  $title" >&2
fi
exit 0
