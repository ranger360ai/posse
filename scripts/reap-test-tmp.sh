#!/usr/bin/env bash
# reap-test-tmp.sh — collect the scratch dirs a killed run left behind in a
# shared /tmp (ranger-base-32bdv; widened to every leak class by
# ranger-base-y9t65, which refused one — the harness's per-session
# scratchpads — and ranger-base-4arlo, which measured a gate that reaches
# them and took it).
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
#       scripts/reap-test-tmp.sh --claude-sessions DIR  the harness's live
#                                                session registry (default
#                                                ~/.claude/sessions)
#       scripts/reap-test-tmp.sh --claude-projects DIR  the harness's session
#                                                transcripts (default
#                                                ~/.claude/projects)
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
#   claude-scratch  claude-<uid>/        1440m    the harness's PER-SESSION
#                     <slug>/<uuid>               scratchpad, two levels down
#                                                 from a root we do not own, and
#                                                 the only class gated on
#                                                 something other than /proc and
#                                                 mtime. The next section is the
#                                                 whole of it.
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
#   - /tmp/node-compile-cache, which is a CACHE with a stable name, not a
#     leak. It regenerates, it does not accumulate, and taking it buys an
#     inode table nothing and costs a recompile.
#   - anything not owned by this uid. `find -uid` is on every candidate
#     walk, which is what keeps the generic `tmp.*` namespace from reaching
#     root's or another user's dirs.
#
# ─── THE HARNESS SCRATCHPADS, AND THE GATE THEY NEEDED (ranger-base-4arlo) ──
#
# ranger-base-y9t65 refused this class and handed it on, because NEITHER of
# the two gates above can reach it and both fail in the UNSAFE direction.
# MEASURED 2026-11-11, both halves:
#
#   - A Claude Code SESSION is not a process in the sense the holder scan
#     means. A live session mid-turn names its own scratchpad in no environ,
#     no argv, no cwd and no fd of ANY process on this box — the path is the
#     harness's, not the shell's. Sweeping every /proc entry for one session's
#     own scratchpad path, from inside that session mid-turn, returned exactly
#     one hit: the sweeping grep's own argv.
#   - And the dir's mtime is set when the session starts and never moves
#     again. The operator's own seat (session 7a2a9b8e) read 15h stale while
#     its transcript had been written to inside the hour. That is a dir BOTH
#     gates call collectable while a person is typing into it.
#
# WHAT DOES WORK IS THE HARNESS'S OWN REGISTRY. ~/.claude/sessions/<pid>.json
# holds {"pid", "sessionId", "procStart", "status", ...}, one file per LIVE
# session, and the scratchpad path carries that same sessionId as its last
# component. MEASURED on this box: 11 registry files, 11 live `claude`
# processes, the SAME 11 pids — no stale entry and no live session missing, so
# the harness removes the file at exit. `procStart` is /proc/<pid>/stat's
# field 22 (starttime), verified on 2 of 2, which is what makes pid reuse
# harmless rather than a hazard. 138 scratchpad dirs existed for those 11 live
# sessions; the other 127 were dead sessions' leavings.
#
# So a dir in this class is held by THREE things, any one of which keeps it:
#   - the /proc scan, as every class is — here it catches a NESTED session
#     whose cwd is inside another session's scratchpad, which this box makes
#     (the `-tmp-claude-1001-...` slugs are exactly that);
#   - the live registry: its uuid is the sessionId of a
#     ~/.claude/sessions/<pid>.json whose /proc/<pid> is alive AND whose
#     starttime still matches;
#   - its own transcript, ~/.claude/projects/<slug>/<uuid>.jsonl, whose mtime
#     moves on EVERY turn. The dir and the transcript must BOTH be older than
#     the grace.
# A MISSING transcript is not a death certificate and never collects the dir.
# MEASURED: all 11 live sessions had one, so "no transcript" never means
# "live" — but it does not mean dead either, and the 11 dirs here that had
# none were 2-3 inodes each, which is not worth a rule that guesses.
#
# IT HAS ITS OWN BLINDNESS CONTROL, for the reason the /proc scan has one: a
# registry the sweep cannot read reads as "nothing is live", which is the
# unsafe answer. If live `claude` processes exist and the registry names none
# of them, THIS CLASS refuses by name and the rest of the sweep goes on.
#
# THE GRACE IS 24h for the same reason mktemp's is — it is not our namespace.
# A posse-dispatched session never returns once its process is gone: a resume
# types into the LIVE composer and a dead pane is relaunched as a new session
# with a new uuid, so for the dispatched majority the registry alone would do.
# What 24h buys is the operator's hand-typed `claude --resume`, which DOES
# reuse the uuid and therefore the scratchpad path: MEASURED over 130
# transcripts, not one held a second sessionId, and the longest gap between
# consecutive entries inside one was 1.5h. The pressure arm drops this to 60m
# with every other class, which is the right shape — under pressure the
# registry is doing the work and the grace is only insurance.
#
# WHAT IT STILL WILL NOT TAKE, and this one is a hole with a name: a dir whose
# transcript the harness has aged out (its own retention, 30 days by default)
# has no signal left and becomes permanently uncollectable. /tmp is a tmpfs
# here, so a reboot is the collector of last resort for those.
#
# WHAT IT RECLAIMS. MEASURED 2026-11-11 against the live /tmp, 138 session
# dirs, 125308 inodes on the filesystem:
#
#   grace   dirs   inodes   % of the inode table
#     4h      98    48983   39.1
#     8h      79    17011   13.6
#    12h      53    13717   10.9
#    24h      39    11506    9.2
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
claude_sessions="${HOME:-/nonexistent}/.claude/sessions"
claude_projects="${HOME:-/nonexistent}/.claude/projects"
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
    --claude-sessions)  claude_sessions="${2:?--claude-sessions needs a directory}"; shift 2 ;;
    --claude-projects)  claude_projects="${2:?--claude-projects needs a directory}"; shift 2 ;;
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
# name|glob|grace in minutes|depth. Order matters only for reporting: a dir
# that two classes both match (posse-testhome-* is also posse-*) is reported
# under the first that accepts it, and the union is what gets removed.
#
# DEPTH is how far below a root the candidate sits, and every class but one is
# 1. claude-scratch is 3 because the thing a session owns is
# <root>/claude-<uid>/<slug>/<session-uuid> and the two dirs above it are
# shared — taking claude-<uid> itself would take every live session on the box.
# At depth 1 the glob is matched against the NAME; deeper it is matched against
# the path below the root, so a `*` there spans one component only because
# -mindepth/-maxdepth pin it.
class_table='posse-testhome|posse-testhome-*|60|1
posse-scratch|posse-*|120|1
go-build|go-build[0-9]*|120|1
go-testtmp|Test*[0-9]|120|1
mktemp|tmp.??????????|1440|1
claude-scratch|claude-[0-9]*/*/*|1440|3'

# EVERY CLASS BUT ONE HAS THE IDENTITY FILTER. claude-scratch's extra gate is
# the session's own transcript, ~/.claude/projects/<slug>/<uuid>.jsonl, whose
# mtime moves on every turn while the dir's does not: the dir AND the
# transcript must both be past the grace. A transcript that is missing is no
# signal at all and therefore keeps the dir — never collects it.
class_filter() {
  local cname="$1" grace="$2" d slug uuid tx
  if [ "$cname" != claude-scratch ]; then cat; return 0; fi
  while IFS= read -r d; do
    [ -n "$d" ] || continue
    uuid=$(basename "$d"); slug=$(basename "$(dirname "$d")")
    tx="$claude_projects/$slug/$uuid.jsonl"
    [ -f "$tx" ] || continue
    [ -n "$(find "$tx" -maxdepth 0 -mmin "+$grace" -print 2>/dev/null)" ] || continue
    printf '%s\n' "$d"
  done
}

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

held=$(mktemp); held3=$(mktemp); stale=$(mktemp); rootlist=$(mktemp)
livedirs=$(mktemp)
trap 'rm -f "$held" "$held3" "$stale" "$rootlist" "$livedirs"' EXIT

printf '%s\n' "${roots[@]}" > "$rootlist"

# Folded once per DEPTH the class table uses. A ref deeper than the depth
# folds up to it; a ref SHALLOWER than it holds nothing at that depth, which
# is the whole reason claude-scratch needed its own fold: every seat that is
# working puts /tmp/claude-<uid> in the depth-1 held set, and at depth 1 that
# one entry would protect every session on the box, live and dead alike. It is
# why y9t65's run reported the root as HELD and reclaimed none of it.
fold_refs() {  # $1 = depth, $2 = outfile
  printf '%s\n' "$rawrefs" | awk -v rootfile="$rootlist" -v d="$1" '
    BEGIN { n = 0; while ((getline line < rootfile) > 0) if (line != "") R[++n] = line }
    {
      for (i = 1; i <= n; i++) {
        r = R[i]
        if (index($0, r "/") == 1) {
          rest = substr($0, length(r) + 2)
          m = split(rest, part, "/")
          if (m < d) break
          p = r
          for (j = 1; j <= d; j++) {
            if (part[j] == "") { p = ""; break }
            p = p "/" part[j]
          }
          if (p != "") print p
          break
        }
      }
    }' | sort -u > "$2"
}
fold_refs 1 "$held"
fold_refs 3 "$held3"

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

# ─── the harness's live-session registry, and its own blindness control ─────
# Only class claude-scratch reads this. One file per LIVE session, removed at
# exit; `procStart` is /proc/<pid>/stat's starttime, so a recycled pid cannot
# impersonate a session that is gone. A live session's dir is added to the
# depth-3 held set beside the /proc refs.
#
# THE PROCESS COUNT IS READ FIRST ON PURPOSE. A session that exits between the
# two reads is counted as a process and not as a live session, which can only
# ever push this toward REFUSING; a session that starts between them has a
# registry file and no count, which cannot make the class take anything it
# otherwise would not. The other direction would hand the sweep a window where
# the registry reads empty and the class believes it.
claude_blind=0 claude_live=0 claude_procs=0
for pid in /proc/[0-9]*; do
  case "$(readlink "$pid/exe" 2>/dev/null)" in */claude) claude_procs=$((claude_procs + 1)) ;; esac
done
: > "$livedirs"
if [ -d "$claude_sessions" ]; then
  for f in "$claude_sessions"/*.json; do
    [ -f "$f" ] || continue
    rpid=$(sed -nE 's/.*"pid"[[:space:]]*:[[:space:]]*([0-9]+).*/\1/p' "$f" | head -1)
    rsid=$(sed -nE 's/.*"sessionId"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$f" | head -1)
    rstart=$(sed -nE 's/.*"procStart"[[:space:]]*:[[:space:]]*"?([0-9]+)"?.*/\1/p' "$f" | head -1)
    [ -n "$rpid" ] && [ -n "$rsid" ] || continue
    [ -d "/proc/$rpid" ] || continue
    if [ -n "$rstart" ]; then
      nowstart=$(sed -E 's/.*\) //' "/proc/$rpid/stat" 2>/dev/null | awk '{print $20}')
      [ "$nowstart" = "$rstart" ] || continue
    fi
    claude_live=$((claude_live + 1))
    for root in "${roots[@]}"; do
      for sd in "$root"/claude-[0-9]*/*/"$rsid"; do
        [ -d "$sd" ] && printf '%s\n' "$sd" >> "$livedirs"
      done
    done
  done
fi
if [ "$claude_procs" -gt 0 ] && [ "$claude_live" -eq 0 ]; then claude_blind=1; fi
echo "harness registry: $claude_live live sessions in $claude_sessions   live claude processes: $claude_procs"
sort -u "$livedirs" "$held3" -o "$held3"

# ─── the candidate walk, class by class ─────────────────────────────────────
# ADR 0089 D2's `flock -n "$d" true` test goes HERE, ahead of the held-set
# filter, once ranger-base-z056i lands: a busy lock is proof the creator is
# alive, and it makes posse-testhome-*'s grace stop carrying the whole
# weight.
: > "$stale"
while IFS='|' read -r cname cglob cgrace cdepth; do
  [ -n "$cname" ] || continue
  if [ "$cname" = claude-scratch ] && [ "$claude_blind" -eq 1 ]; then
    echo "class claude-scratch: REFUSED — $claude_procs live claude processes and the registry at $claude_sessions names none of them, so 'nothing is live' is not something this run knows"
    continue
  fi
  if [ ${#want_classes[@]} -gt 0 ]; then
    printf '%s\n' "${want_classes[@]}" | grep -qxF "$cname" || continue
  fi
  grace=$cgrace
  [ -n "$min_age" ] && grace=$min_age
  [ "$pressure" -eq 1 ] && grace=$pressure_min_age
  heldfile=$held
  [ "$cdepth" = 1 ] || heldfile=$held3
  found=$(
    for root in "${roots[@]}"; do
      [ -d "$root" ] || continue
      if [ "$cdepth" = 1 ]; then
        find "$root" -maxdepth 1 -mindepth 1 -type d -uid "$me" -name "$cglob" -mmin "+$grace" -print 2>/dev/null || true
      else
        # ROOTED AT THE MATCHING TOP-LEVEL DIR, not at the root with a -path
        # filter. A deep walk from /tmp itself descends into every other uid's
        # dirs to reach the depth, and find then exits 1 on the first
        # `Permission denied` — which, under `set -o pipefail`, took the whole
        # sweep down with it before this was scoped (MEASURED: exit 1 with the
        # four depth-1 classes already reported and nothing collected). The
        # trailing `|| true` is the second half of that lesson: a walk that
        # cannot read something must collect less, never abort.
        for top in "$root"/${cglob%%/*}; do
          [ -d "$top" ] || continue
          find "$top" -maxdepth "$((cdepth - 1))" -mindepth "$((cdepth - 1))" -type d -uid "$me" -path "$top/${cglob#*/}" -mmin "+$grace" -print 2>/dev/null || true
        done
      fi
    done | sort -u | { grep -vxF -f "$heldfile" || true; } | { grep -vxF -f "$stale" || true; } | class_filter "$cname" "$grace"
  )
  cn=$(printf '%s\n' "$found" | grep -c . || true)
  echo "class $cname ($cglob, grace ${grace}m, depth $cdepth): $cn collectable"
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

# The <slug> dir between the harness root and a session dir is the harness's
# and is left behind empty once its last session goes. rmdir, which can only
# ever take an empty one, so a slug that still holds a live session is safe by
# construction — 2 inodes each, 127 of them at the time of writing.
if [ ${#want_classes[@]} -eq 0 ] || printf '%s\n' "${want_classes[@]}" | grep -qxF claude-scratch; then
  for root in "${roots[@]}"; do
    [ -d "$root" ] || continue
    find "$root" -mindepth 2 -maxdepth 2 -type d -uid "$me" -path "$root/claude-[0-9]*/*" -empty -exec rmdir -- {} + 2>/dev/null || true
  done
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
