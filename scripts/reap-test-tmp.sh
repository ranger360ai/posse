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
#       scripts/reap-test-tmp.sh --self-test     prove the claude-scratch gating
#                                                arms and the claude_procs `-p`
#                                                exclusion, against a scratch
#                                                root only (`make
#                                                verify-reap-test-tmp`, a
#                                                prerequisite of `make test`)
#
# ─── WHAT IT TAKES, AND WHY EACH CLASS HAS ITS OWN GRACE ────────────────────
#
# A dir is collected only when ALL THREE hold (ADR 0089 D2):
#   - `flock -n` on the dir succeeds, so no live creator is holding the lock
#     ADR 0089 D1 makes it take;
#   - no live process names it — through ANY value in its environment, its
#     argv, its cwd, its exe, or any open fd; and
#   - its mtime is at least its class's grace old.
#
# THE LOCK IS THE ONLY ONE OF THE THREE THAT IS A POSITIVE PROOF OF LIFE, and
# exactly one class has a creator we could make take it. For every other
# class the two gates below are all there is, so the next paragraph is what
# still holds for them.
#
# THE SCAN IS A BEST-EFFORT NEGATIVE AND THE GRACE IS THE REAL GATE. ADR 0089
# measured why: Go's os.Setenv does not rewrite /proc/<pid>/environ, so a test
# binary that made a dir and pointed HOME at it is INVISIBLE to the scan, and
# a shell's `d=$(mktemp -d)` is invisible for the same reason one step over —
# the path lives in a shell variable, which is not argv, not cwd and not an
# fd. So every class's grace below is set against the longest run that could
# still be holding one of its dirs, and the scan only ever makes the answer
# more conservative. ADR 0089 D1-D2 closed this for posse-testhome-*
# (ranger-base-z056i): internal/posse's TestMain opens its temp HOME and takes
# LOCK_EX on the directory fd for the whole life of the process, the kernel
# releases that lock on SIGKILL and on a -timeout panic alike, and the `flock
# -n` step in the candidate filter below reads a busy lock as a live creator.
# Release IS death there, so that class's grace stops being load-bearing —
# though it is NOT removed, because it is still the only thing covering the
# microseconds between MkdirTemp and the flock, and the only thing covering a
# binary BUILT BEFORE D1 that is still running. No other class has a creator
# we can make take a lock, so for them the grace is all there is — which is
# ADR 0089's own objection to grace, stated here rather than hidden.
#
# THE LOCK STEP RUNS FOR EVERY CLASS ANYWAY, and that is deliberate rather
# than sloppy: a busy lock can only ever make this script collect LESS, and a
# future creator in any class that announces itself the same way is then
# covered by construction instead of by an edit here.
#
#   class           glob                 grace   why that number
#   ───────────────────────────────────────────────────────────────────────
#   posse-testhome  posse-testhome-*      60m    internal/posse TestMain's
#                                                one temp HOME per test tree;
#                                                the original class, grace
#                                                unchanged from 32bdv — and
#                                                the one class whose live
#                                                creator holds a flock, so
#                                                the grace here is now
#                                                insurance rather than the
#                                                gate (ADR 0089 D1).
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
#                                                ranger-base-i78k9 moved the
#                                                last nine of those shell
#                                                roots in here from the
#                                                `mktemp` class below, where
#                                                the same leak waited 24h
#                                                for no reason but its name.
#                                                ranger-base-tbukc moved five
#                                                more in (posse-claude-trust-
#                                                key, posse-detection, posse-
#                                                herdr-bob, posse-herdr-bob-
#                                                rules, posse-gate-freshness)
#                                                from NO class at all: they
#                                                already HAD a template, so
#                                                they matched neither `posse-*`
#                                                nor the `tmp.??????????` below
#                                                and NO row reached them — a
#                                                leaked one was never
#                                                collected, which is worse
#                                                than i78k9's 24h wait.
#                                                Those three beads each
#                                                censused scripts/ BY HAND and
#                                                left nothing asserting the
#                                                result; ranger-base-w8n6w
#                                                pinned it instead, so a
#                                                `mktemp -d` under scripts/
#                                                now has to name
#                                                $TMPDIR/posse-<what>.XXXXXX
#                                                or carry its reason
#                                                (`make scripts-check`,
#                                                internal/treepins/
#                                                mktemproot_qa_test.go). That
#                                                pin is why a FOURTH hand
#                                                census is not how row 28
#                                                gets found.
#   go-build        go-build[0-9]*       120m    Go's $WORK. A RUNNING test
#                                                binary's /proc/<pid>/exe
#                                                names it, so this one the
#                                                scan can actually see; the
#                                                grace is the same 2x bound
#                                                for the compile-only case.
#   go-testtmp      Test*[0-9]           120m    t.TempDir() with TMPDIR=/tmp.
#                                                Bounded by the same -timeout.
#   coldexec-pins   coldexec-pins*       120m    cmd/coldexec's OLD build-dir
#                                                name, pre-ranger-base-7hyn8.
#                                                The fix renamed every FUTURE
#                                                one into posse-coldexec-pins-*,
#                                                which posse-scratch already
#                                                covers, so this class exists
#                                                only to drain the pre-fix
#                                                backlog under the dead name
#                                                (278 dirs, 914M, ranger-base-
#                                                m0j9n) — nothing on this box
#                                                creates this name again. Same
#                                                grace as go-build: the dir
#                                                held one `go build` of the
#                                                same shape.
#   testparallel-   testparallel-        120m    cmd/testparallel's OLD
#    selftest          selftest*                 self-test fixture name,
#                                                 pre-ranger-base-4dw7b: one
#                                                 os.MkdirTemp per test binary
#                                                 (sync.Once) and no remover,
#                                                 holding a built
#                                                 `testparallel` binary a
#                                                 RUNNING test execs — the
#                                                 scan sees it exactly the way
#                                                 go-build's does. Same bound.
#                                                 That fix gave it a TestMain
#                                                 remover and renamed every
#                                                 FUTURE one into posse-
#                                                 testparallel-selftest-*,
#                                                 which posse-scratch already
#                                                 covers, so this class now
#                                                 exists only to drain the
#                                                 pre-fix backlog under a dead
#                                                 name (115 dirs, 365M,
#                                                 MEASURED 2026-11-12) —
#                                                 nothing on this box creates
#                                                 this name again.
#   pkcensus-       pkcensus-            120m    scripts/pattern-kill-
#    selftest           selftest.*                census.py --self-test's OLD
#                                                  fixture-root name, ditto
#                                                  (ranger-base-4dw7b: a
#                                                  try/finally remover, and
#                                                  the name moved to posse-
#                                                  pkcensus-selftest.*). A few
#                                                  planted transcripts, no
#                                                  build, no long-running
#                                                  exec. No -timeout bounds it
#                                                  either, since it is a plain
#                                                  python3 invocation and not
#                                                  a `go test` arm, so it
#                                                  takes the same 120m the
#                                                  owned classes above use
#                                                  rather than a number argued
#                                                  from a run that does not
#                                                  exist. Backlog: 157 dirs,
#                                                  5.6M, MEASURED 2026-11-12.
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
#   - a leaked scratch FILE — and this one is a DECISION, not the oversight
#     it looks like (ranger-base-u1z89). Every candidate walk below is
#     `-type d`, so the table above is a table of DIRECTORY classes and a
#     file in a swept root sits outside all nine rows: not a 24h quarantine,
#     no walk that could reach it whatever it is named, even when its name
#     is exactly the `tmp.??????????` the last row globs. A `-type f` arm
#     was refused and the PRODUCERS were moved instead — every scratch file
#     this tree makes now lives inside a `posse-*` dir its own script owns,
#     so posse-scratch collects the parent and the file goes with it
#     (internal/treepins/mktemproot_qa_test.go pins both halves, so the next
#     one reds in the suite rather than in /tmp). Three reasons, in the
#     order they decided it:
#       - ALL THREE GATES ARE WEAKER ON A FILE. The lock is vacuous: no
#         creator on this box takes a lock on its scratch file, so step 1
#         stops being a positive proof of life and becomes a step that
#         always says yes. The /proc fd that names it is open only while
#         something is writing to it. And a shell's `f=$(mktemp)` is in no
#         argv, which is ADR 0089's own measurement one file type over. The
#         grace would be the whole gate, which is ADR 0089's objection to
#         grace — and the only class where no creator could be made to fix
#         it.
#       - THE GLOB COULD NOT BE `posse-*`. MEASURED 2026-11-12 on this box:
#         38 depth-1 files in /tmp against 10847 dirs, and three of the 38
#         are deliberate, stable-named artifacts a person is meant to come
#         back and read — posse-test-signal.log (726163 bytes: the process
#         table at the instant a suite was signalled, which AGENTS.md sends
#         the reader to), posse-coldexec-report and posse-make-install.log.
#         A `posse-*` file arm collects the evidence. A template-shaped glob
#         could tell them apart, which is one more rule to keep true.
#       - AND IT WOULD HAVE BOUGHT ONE INODE. Exactly one of those 38 files
#         was a leaked scratch file (posse-test-times.n3Ffyo, 0 bytes,
#         2026-11-11). The producers were 10 call sites in 5 files and every
#         one of them is ours.
#     What was already on the floor before the fix is the reboot's: /tmp is
#     a tmpfs here, the same collector of last resort the aged-out
#     claude-scratch dirs get. docs/notes.d/ranger-base-u1z89.md has the
#     census and the names.
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
apply=0 min_age="" roots=() want_classes=() quiet=0 selftest=0
claude_sessions="${HOME:-/nonexistent}/.claude/sessions"
claude_projects="${HOME:-/nonexistent}/.claude/projects"
pressure_at=70 pressure_min_age=60 bead_at=85 bead_repo="" no_bead=0 bead_cooldown=24

die2() { echo "$self: $*" >&2; exit 2; }

# ─── self-test (ranger-base-2smyx findings 2 and 4) ─────────────────────────
#
# THERE WAS NO TEST ANYWHERE IN THE TREE for the claude-scratch class's
# gating -- pid liveness, procStart, transcript freshness -- and the
# eleven-row table that licensed it at ranger-base-4arlo lived only in a
# close comment (`grep -rl reap-test-tmp --include='*_test.go' .` finds
# nothing). This is that table, committed, run against a fresh `mktemp -d`
# root every time rather than against the real /tmp -- plus a second rig
# below it for finding 4's fix, which this table cannot exercise because
# none of its own fixtures are claude-EXE processes.
#
# A `-p`/`--print` RUN NEVER WRITES A REGISTRY FILE, so counting it toward
# claude_procs made the blindness control (below, "the harness's live-session
# registry") refuse the whole class for a reason that is not blindness: on a
# shared box a sibling seat's own `claude -p` pin is exactly such a run. The
# fix excludes it; this rig proves the exclusion by MEMBERSHIP -- is the
# fixture's own pid in the set the counter reports -- and not by the blind
# flag, which only fires when the registry is EMPTY and which a busy box's
# own traffic cannot be made to guarantee either way.
#
# IT USED TO PROVE IT BY COUNT, differencing two box-wide censuses taken a
# second apart, and that is the one shape it cannot use (ranger-base-0wvu6).
# This box runs real, ambient, same-uid `claude` processes this script can
# see and SHOULD see (every seat here is uid posse), so any sibling seat's
# launch landing between the two readings shifted both of them: `10 -> 11 ->
# 12` with the rig's own contribution being `0 -> 0 -> +1`, both arms red
# together, the message naming the reaper, and `make test` dead before a
# single package ran because verify-reap-test-tmp is a prerequisite of it
# (CAUGHT 2026-11-11 and again 2026-11-12, both passing on a rerun minutes
# later). A membership test asks the same question with nobody else's traffic
# in the answer.
self_test() {
  local tmp fails=0
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/posse-reap-selftest.XXXXXX") || { echo "$self --self-test: mktemp failed" >&2; exit 2; }
  selftest_tmp=$tmp
  SELFTEST_PIDS=()
  trap 'kill "${SELFTEST_PIDS[@]}" 2>/dev/null || true; rm -rf -- "${selftest_tmp:?}"' EXIT

  mkdir -p "$tmp/root" "$tmp/sessions" "$tmp/projects"
  local me old slug
  me=$(id -u); old='202611010000'; slug='-selftest-reap'

  mk() {  # mk <uuid> <has-transcript:yes|no> <transcript-fresh:yes|no>
    # u/tx/fresh/d assigned one at a time, never on one `local` line: a
    # combined `local u="$1" d="...$u"` expands $u before the assignment to
    # u lands (ranger-base-2smyx's own note on the rig this is built from).
    local u tx fresh d
    u="$1"
    tx="$2"
    fresh="$3"
    d="$tmp/root/claude-$me/$slug/$u"
    mkdir -p "$d"; : > "$d/marker"
    if [ "$tx" = yes ]; then
      mkdir -p "$tmp/projects/$slug"
      : > "$tmp/projects/$slug/$u.jsonl"
      [ "$fresh" = yes ] || touch -t "$old" "$tmp/projects/$slug/$u.jsonl"
    fi
    touch -t "$old" "$d/marker" "$d"
  }
  # st <pid> -- /proc/<pid>/stat's field 22 (starttime), parsed with bash
  # parameter expansion rather than a forked `sed | awk` -- an assertion arm's
  # matcher must not fork (TestQANoAssertionArmDecidesThroughAForkedMatcher):
  # a `sed`/`awk` that is signalled, or that cannot be exec'd under load,
  # would read back as an empty procStart and the arm would misreport a KEEP
  # as something the test never actually exercised. `$(< file)` is bash's own
  # builtin read, not `cat`. comm (the parenthesised field) can itself hold a
  # ')', so stripping through the LAST ') ' -- `##*) ` is greedy -- is the
  # same rule the production parser uses two call sites below.
  st() {
    local line rest
    line=$(< "/proc/$1/stat" 2>/dev/null) || { printf '\n'; return 0; }
    rest=${line##*") "}
    local -a f=()
    read -r -a f <<< "$rest"
    printf '%s\n' "${f[19]:-}"
  }
  reg() {  # reg <uuid> <pid> <procStart|OMIT>
    local u p s
    u="$1"
    p="$2"
    s="$3"
    if [ "$s" = OMIT ]; then
      printf '{"pid":%s,"sessionId":"%s","status":"idle"}\n' "$p" "$u"
    else
      printf '{"pid":%s,"sessionId":"%s","procStart":"%s","status":"idle"}\n' "$p" "$u" "$s"
    fi > "$tmp/sessions/$p.json"
  }

  echo "$self --self-test:"

  # ONE PID PER REGISTRY FILE: the files are keyed by pid, so three arms
  # sharing one holder would overwrite each other and the live arm would
  # read as unregistered.
  sleep 60 >/dev/null 2>&1 </dev/null & local pa=$!
  sleep 60 >/dev/null 2>&1 </dev/null & local pc=$!
  sleep 60 >/dev/null 2>&1 </dev/null & local pd=$!
  mk A yes no;  reg A "$pa" "$(st "$pa")"   # KEEP: pid live, procStart matches
  mk B yes no;  reg B 999999 1              # COLLECT: registered pid is dead
  mk C yes no;  reg C "$pc" 1               # COLLECT: live pid, wrong procStart
  mk D yes no;  reg D "$pd" OMIT            # KEEP: no procStart field, pid alive
  mk E yes yes                              # KEEP: transcript fresh
  mk F no  no                               # KEEP: no transcript is no signal
  mk G yes no                               # COLLECT: unregistered, both old
  mk H yes no                               # KEEP: a process is cwd'd inside
  ( cd "$tmp/root/claude-$me/$slug/H" && exec sleep 60 ) >/dev/null 2>&1 </dev/null & local ph=$!
  SELFTEST_PIDS=("$pa" "$pc" "$pd" "$ph")
  sleep 1

  "$0" --root "$tmp/root" --class claude-scratch --apply --quiet \
    --claude-sessions "$tmp/sessions" --claude-projects "$tmp/projects" --no-bead \
    >/dev/null 2>&1 || true

  # THE FOUR HOLDERS MUST STILL BE ALIVE (ranger-base-bttuj). The sweep above
  # IS this rig's census, and every verdict below is about what it saw; a
  # holder that died before it ran makes the arm it holds decide over a
  # population that is not the one the row names. Checked after the sweep and
  # not before, because "alive on both sides of it" is what says alive DURING
  # it — these are sleeps, and a sleep does not come back.
  #
  # ARM C IS WHY THIS GATE EXISTS, and it is the only one of the four whose
  # failure direction is unsafe. C registers a live pid with a WRONG procStart
  # and wants COLLECTED; a dead pid is collected too, by the `[ -d
  # /proc/$rpid ]` gate one line above the procStart match it is there to
  # grade. So with a dead holder C passes whether that match exists or not.
  # MEASURED 2026-11-12 on this box: with the match at :1125 deleted and C's
  # holder left alive, C reds (`want COLLECTED, got KEPT`); with the same
  # deletion and the holder exiting at once, the whole self-test is green.
  # That is the vacuity ranger-base-lydqn finding 4's `-p` arm shipped with
  # and ranger-base-0wvu6 closed one arm up, in the same shape: a fixture
  # whose death is indistinguishable from the behaviour it asserts.
  # A, D and H are fail-safe — each wants KEPT and a dead holder collects its
  # dir, so they red on their own — but they red saying "want KEPT, got
  # COLLECTED", which reads as a reaper defect. They are named here too so
  # the rig reports itself instead.
  #
  # `kill -0` is a builtin: an assertion arm's matcher must not fork
  # (TestQANoAssertionArmDecidesThroughAForkedMatcher, ranger-base-t07yx).
  local hpid hname
  for hname in A:$pa C:$pc D:$pd H:$ph; do
    hpid=${hname#*:}
    if ! kill -0 "$hpid" 2>/dev/null; then
      echo "  FAIL arm ${hname%%:*}'s holder (pid $hpid) was already gone when the sweep ran — that arm proves nothing about the reaper, and the rig is what is broken"
      fails=$((fails + 1))
    fi
  done

  local u got
  for u in A D E F H; do
    [ -d "$tmp/root/claude-$me/$slug/$u" ] && got=KEPT || got=COLLECTED
    if [ "$got" = KEPT ]; then echo "  ok   $u kept"
    else echo "  FAIL $u: want KEPT, got COLLECTED"; fails=$((fails + 1)); fi
  done
  for u in B C G; do
    [ -d "$tmp/root/claude-$me/$slug/$u" ] && got=KEPT || got=COLLECTED
    if [ "$got" = COLLECTED ]; then echo "  ok   $u collected"
    else echo "  FAIL $u: want COLLECTED, got KEPT"; fails=$((fails + 1)); fi
  done

  kill "$pa" "$pc" "$pd" "$ph" 2>/dev/null || true
  wait "$pa" "$pc" "$pd" "$ph" 2>/dev/null || true
  SELFTEST_PIDS=()

  # ─── finding 4's own rig: does `-p` stay out of claude_procs? ─────────────
  # A real ELF file named "claude" is required: /proc/<pid>/exe resolves
  # through a symlink to its REAL target, so a symlink named "claude" would
  # read back as whatever it points at, never as ".../claude" itself. A
  # plain copy of a system binary is a distinct inode at a path that ends the
  # way the production `case "$(readlink exe)" in */claude)` wants it to.
  #
  # THE FIXTURE IS A COPY OF tee(1), NOT OF sleep(1) (ranger-base-0wvu6),
  # because `-p` has to be an argument the fixture SURVIVES. MEASURED
  # 2026-11-12 on this box (linux, coreutils 9.x): `sleep -p 30` is `invalid
  # option -- 'p'`, exit 1, gone before the rig's `sleep 1` is over — so the
  # `-p` arm was reading "the process I started is not counted" over a
  # process that had already exited, and would have read exactly the same
  # with claude_could_register deleted. `tee -p` is a real option
  # (--output-error=warn-nopipe) and tee with no file operand idles on stdin,
  # so this fixture is alive, countable, and carries the `-p` the production
  # filter has to see. Both arms assert that liveness AT THE MOMENT OF THE
  # CENSUS: a fixture that died is reported as a broken rig and never as a
  # passing exclusion. That is the vacuity the old spelling could not detect,
  # and the reason this arm's verdict is a membership test and not a delta —
  # a delta over an absent process and a delta over an excluded one are the
  # same number.
  local teebin fakeclaude fifo pkeep pfake cen cn cpids
  local -a cparr=()
  teebin=$(command -v tee) || { echo "  FAIL no tee(1) on PATH to copy for the fake claude binary"; fails=$((fails + 1)); teebin=""; }
  fakeclaude="$tmp/claude"
  fifo="$tmp/keepopen"

  # claude_census runs the production counter ONCE and hands back both facts
  # it reports about that population — the count and the pids — as
  # "<count>|<pid> <pid> ...", or "ABSENT|" when either line is missing. One
  # run, so the two are the same loop's answer and the arms below never
  # compare one census to another. Both lines are read with bash parameter
  # expansion — `${out#*"needle"}` then cut at the next newline, the same
  # shape as scripts/test-times.sh's `line_after` — and not a forked
  # `sed`/`grep`, for the reason `st` above gives.
  claude_census() {
    local out rest n pids
    out=$("$0" --root "$tmp/root" --class claude-scratch --quiet \
      --claude-sessions "$tmp/sessions" --claude-projects "$tmp/projects" --no-bead 2>&1) || true
    rest=${out#*"live claude processes: "}
    [ "$rest" = "$out" ] && { printf 'ABSENT|\n'; return 1; }
    n=${rest%%$'\n'*}
    rest=${out#*"claude pids counted:"}
    [ "$rest" = "$out" ] && { printf 'ABSENT|\n'; return 1; }
    pids=${rest%%$'\n'*}
    printf '%s|%s\n' "$n" "$pids"
  }

  # start_fake <argv...> — one live claude-exe process with that argv, its pid
  # in $pfake. The keeper holds the fifo's write end open so tee never reads
  # EOF; its pid is known too, so neither process is left to a grace period,
  # and both go in SELFTEST_PIDS so a signalled self-test takes them with it.
  #
  # THE KEEPER'S BOUND IS NOT A DEADLINE FOR THE ARM. Both processes are
  # killed by stop_fake the moment the census is read, so the only thing the
  # number does is cap what a SIGKILLed self-test could leave behind (two idle
  # processes, no CPU). It is 300s and not the census's few seconds because a
  # keeper that expires first ends the fake, and the arm would then report a
  # broken rig over nothing but a loaded box — which is the class of defect
  # this rewrite exists to remove, one apparatus down.
  start_fake() {
    sleep 300 > "$fifo" 2>/dev/null & pkeep=$!
    "$fakeclaude" "$@" < "$fifo" >/dev/null 2>&1 & pfake=$!
    SELFTEST_PIDS=("$pkeep" "$pfake")
    sleep 1
  }
  stop_fake() {
    kill "$pfake" "$pkeep" 2>/dev/null || true
    wait "$pfake" 2>/dev/null || true
    wait "$pkeep" 2>/dev/null || true
    SELFTEST_PIDS=()
  }

  if [ -n "$teebin" ]; then
    cp "$teebin" "$fakeclaude"
    chmod +x "$fakeclaude"
    mkfifo "$fifo"

    # The pid list is what the two arms below decide through, so it is worth
    # nothing unless it is the count's own evidence. This arm compares the two
    # WITHIN ONE RUN — no second census, so no sibling seat's launch can reach
    # it — and it is also where an unparseable or missing line is caught: in a
    # membership arm a list that could not be read reads as "my pid is not in
    # it", which is the answer the exclusion arm wants.
    cen=$(claude_census) || true
    cn=${cen%%|*}
    cpids=${cen#*|}
    cparr=()
    read -r -a cparr <<< "$cpids"
    if [ "$cn" = ABSENT ]; then
      echo "  FAIL the script printed no 'live claude processes: N' / 'claude pids counted:' pair — the arms below have nothing to read"
      fails=$((fails + 1))
    elif [ "${#cparr[@]}" = "$cn" ]; then
      echo "  ok   claude_procs ($cn) and the pids it reports (${#cparr[@]}) agree"
    else
      echo "  FAIL claude_procs says $cn but the script reported ${#cparr[@]} pid(s) ($cpids) — the list is not that count's evidence"
      fails=$((fails + 1))
    fi

    start_fake -p
    cen=$(claude_census) || true
    cn=${cen%%|*}
    cpids=${cen#*|}
    if ! kill -0 "$pfake" 2>/dev/null; then
      echo "  FAIL the '-p' fixture (pid $pfake) was already gone when the census ran — this arm proves nothing about the exclusion, and the rig is what is broken"
      fails=$((fails + 1))
    elif [ "$cn" = ABSENT ]; then
      echo "  FAIL could not read the counted pids back from the script's own output after the -p run"
      fails=$((fails + 1))
    else
      case " $cpids " in
        *" $pfake "*)
          echo "  FAIL a live 'claude -p' run (pid $pfake) IS in claude_procs — it must be excluded (counted:$cpids)"
          fails=$((fails + 1))
          ;;
        *)
          echo "  ok   a live 'claude -p' run (pid $pfake) is excluded from claude_procs"
          ;;
      esac
    fi
    stop_fake

    start_fake
    cen=$(claude_census) || true
    cn=${cen%%|*}
    cpids=${cen#*|}
    if ! kill -0 "$pfake" 2>/dev/null; then
      echo "  FAIL control: the non-print fixture (pid $pfake) was already gone when the census ran — the rig is what is broken"
      fails=$((fails + 1))
    elif [ "$cn" = ABSENT ]; then
      echo "  FAIL could not read the counted pids back from the script's own output after the non-print run"
      fails=$((fails + 1))
    else
      case " $cpids " in
        *" $pfake "*)
          echo "  ok   control: the same binary without -p IS counted (pid $pfake)"
          ;;
        *)
          echo "  FAIL control: a non-print claude process (pid $pfake) is NOT in claude_procs — the exclusion above is excluding everything, or the exe match is broken (counted:$cpids)"
          fails=$((fails + 1))
          ;;
      esac
    fi
    stop_fake
  fi

  # ─── ranger-base-x2blo: a candidate that vanished before the probe ────────
  # `flock` opens its lock path with O_CREAT, so before the `[ -d "$d" ]`
  # guard a candidate removed between the walk and the probe was RE-CREATED
  # as a 0-byte regular FILE, answered rc=0 and was offered as collectable —
  # an inode the sweep made itself and, every class's `find` being `-type d`,
  # one no later run could see.
  #
  # THIS ARM CALLS `flock_filter` DIRECTLY, alone among the arms here, and
  # that is the point rather than a shortcut: the window the guard closes is
  # between the `find` and the probe inside one pipeline, and nothing a
  # fixture can plant reopens that window on demand. Feeding the function the
  # one input the race produces — a candidate path that is not there — tests
  # the guard itself and not a race's timing. It is why the `--self-test`
  # dispatch sits below the function's definition.
  #
  # Every decision below is a shell builtin over a variable the probe already
  # filled, and the one FAIL message that quotes a file reads it with bash's
  # own `$(< file)` rather than a `cat` — the rule `st` above states, for the
  # reason it states: a forked matcher that is signalled, or that cannot be
  # exec'd under load, reads back empty and the arm misreports.
  local gone goneheld goneout freed freedout
  gone="$tmp/vanished-candidate"
  goneheld="$tmp/vanished-candidate.held"
  goneout=$(printf '%s\n' "$gone" | flock_filter "$goneheld")
  if [ -e "$gone" ]; then
    echo "  FAIL the flock probe re-created a vanished candidate (it exists and is not a dir): $gone"
    fails=$((fails + 1))
    rm -rf -- "${gone:?}"
  else
    echo "  ok   a vanished candidate is not re-created by the flock probe"
  fi
  if [ -n "$goneout" ]; then
    echo "  FAIL a vanished candidate was offered as collectable: $goneout"
    fails=$((fails + 1))
  else
    echo "  ok   a vanished candidate is not offered as collectable"
  fi
  # Dropped silently, not reported: there is no dir to keep and nothing for
  # the next reader to act on, so it belongs in neither stream.
  if [ -s "$goneheld" ]; then
    echo "  FAIL a vanished candidate was reported as kept: $(< "$goneheld")"
    fails=$((fails + 1))
  else
    echo "  ok   a vanished candidate is not reported as kept"
  fi
  # Control, so the guard cannot pass by rejecting everything: a real, free
  # dir still comes through the probe as a candidate.
  freed="$tmp/free-candidate"
  mkdir -p "$freed"
  freedout=$(printf '%s\n' "$freed" | flock_filter "$goneheld")
  if [ "$freedout" = "$freed" ]; then
    echo "  ok   control: a free dir still passes the flock probe"
  else
    echo "  FAIL control: a free dir did not pass the flock probe, got '$freedout'"
    fails=$((fails + 1))
  fi

  # ─── ranger-base-q0yxp: pin the three fixture-prefix classes ──────────────
  # coldexec-pins, testparallel-selftest and pkcensus-selftest share every
  # gate with posse-scratch/go-build/go-testtmp (no class-specific code, only
  # a row in the table), so this pins the ROW rather than new logic — exactly
  # the two gates ranger-base-m0j9n's ask named: "owned by posse, named by no
  # live process, older than the class grace". A dir still inside its grace
  # is kept regardless of anything else; a dir a live process cwd's into is
  # kept whatever its age; a dir with neither is collected. Each class gets
  # its own fresh `mktemp -d` root so the three runs cannot see each other's
  # fixtures.
  #
  # THE HELD FIXTURE COMES IN TWO (ranger-base-68631 finding 3). `( cd "$d"
  # && exec sleep 60 )` is named by /proc/<pid>/cwd AND by PWD in
  # /proc/<pid>/environ, so EITHER ref alone keeps the dir, and an arm built
  # on that shape pins neither — the pre-existing arm H above has it too,
  # which is where this rig copied it from. MEASURED, three mutants of the
  # ref scan below, `bash <copy> --self-test` each time: dropping the `cwd`
  # readlink passed every arm, dropping the `environ` read passed every arm,
  # dropping BOTH failed H and all three class arms. So nothing held the cwd
  # readlink, which is the route the header above says the scan exists for in
  # the first place: a Go holder whose os.Setenv never rewrites
  # /proc/<pid>/environ.
  #
  # `env -i` is the whole of the two single-route fixtures. It clears the
  # environment of the exec'd process, PWD included, so each dir's path is in
  # that process's /proc entry exactly ONCE and in none of the other refs the
  # scan reads — not cmdline (`env` consumes the assignment and then execs,
  # so the final argv is `sleep 60`), not exe (sleep's own path), not an fd
  # (all three are /dev/null):
  #
  #   agedcwd  named by /proc/<pid>/cwd alone      (cd into it, empty env)
  #   agedenv  named by /proc/<pid>/environ alone  (cwd /, one variable)
  #
  # so dropping EITHER ref from the scan reds exactly the arm that reads it —
  # which is what the two-ref fixture above could not do in either direction.
  # The binary is named by ABSOLUTE path because `env -i` clears PATH before
  # the exec, so a bare `sleep` would not be found; no absolute sleep is
  # reported as a FAIL rather than skipped, because a fixture that silently
  # does not start is an arm passing over an unmeasured route, which is this
  # finding again one level up.
  generic_rig() {  # generic_rig <class-name> <glob-prefix, without the *>
    local cname="$1" prefix="$2" groot fresh agedheld agedcwd agedenv agedfree pid cwdpid envpid sleepabs p
    groot=$(mktemp -d "${TMPDIR:-/tmp}/posse-reap-classrig.XXXXXX")
    fresh="$groot/${prefix}fresh1"
    agedheld="$groot/${prefix}heldAAAAAA"
    agedcwd="$groot/${prefix}cwdAAAAAAA"
    agedenv="$groot/${prefix}envAAAAAAA"
    agedfree="$groot/${prefix}freeAAAAAA"
    mkdir -p "$fresh" "$agedheld" "$agedcwd" "$agedenv" "$agedfree"
    touch -t "$old" "$agedheld" "$agedcwd" "$agedenv" "$agedfree"
    ( cd "$agedheld" && exec sleep 60 ) >/dev/null 2>&1 </dev/null & pid=$!
    cwdpid="" envpid=""
    sleepabs=$(command -v sleep) || sleepabs=""
    case $sleepabs in
      /*)
        ( cd "$agedcwd" && exec env -i "$sleepabs" 60 ) >/dev/null 2>&1 </dev/null & cwdpid=$!
        ( cd / && exec env -i "POSSE_REAP_SELFTEST_HOLD=$agedenv" "$sleepabs" 60 ) >/dev/null 2>&1 </dev/null & envpid=$!
        ;;
      *)  echo "  FAIL $cname: no absolute sleep(1) on PATH, so the single-route holders never started and the cwd and environ routes are UNMEASURED"; fails=$((fails + 1)) ;;
    esac
    sleep 1
    "$0" --root "$groot" --class "$cname" --apply --quiet --no-bead \
      >/dev/null 2>&1 || true
    if [ -d "$fresh" ]; then echo "  ok   $cname: dir inside its grace kept"
    else echo "  FAIL $cname: dir inside its grace was collected"; fails=$((fails + 1)); fi
    if [ -d "$agedheld" ]; then echo "  ok   $cname: dir a live process cwd's into kept"
    else echo "  FAIL $cname: dir a live process cwd's into was collected"; fails=$((fails + 1)); fi
    if [ -n "$cwdpid" ]; then
      if [ -d "$agedcwd" ]; then echo "  ok   $cname: dir named ONLY by a live process's cwd kept"
      else echo "  FAIL $cname: dir named ONLY by a live process's cwd was collected -- the /proc/<pid>/cwd readlink is the route this script's header claims, and this is the only arm that reads it"; fails=$((fails + 1)); fi
    fi
    if [ -n "$envpid" ]; then
      if [ -d "$agedenv" ]; then echo "  ok   $cname: dir named ONLY by a live process's environ kept"
      else echo "  FAIL $cname: dir named ONLY by a live process's environ was collected -- the environ read is what 32bdv widened to EVERY value, and this is the only arm that reads it alone"; fails=$((fails + 1)); fi
    fi
    if [ -d "$agedfree" ]; then echo "  FAIL $cname: aged, unheld dir was kept, want collected"; fails=$((fails + 1))
    else echo "  ok   $cname: aged, unheld, unnamed dir collected"; fi
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    for p in "$cwdpid" "$envpid"; do
      [ -n "$p" ] || continue
      kill "$p" 2>/dev/null || true
      wait "$p" 2>/dev/null || true
    done
    rm -rf -- "${groot:?}"
  }
  generic_rig coldexec-pins 'coldexec-pins'
  generic_rig testparallel-selftest 'testparallel-selftest'
  generic_rig pkcensus-selftest 'pkcensus-selftest.'

  echo
  if [ "$fails" -gt 0 ]; then echo "$self --self-test: $fails arm(s) FAILED"; return 1; fi
  echo "$self --self-test: all arms pass"
  return 0
}

while [ $# -gt 0 ]; do
  case "$1" in
    --apply)   apply=1; shift ;;
    --root)    roots+=("${2:?--root needs a directory}"); shift 2 ;;
    --min-age) min_age="${2:?--min-age needs minutes}"; shift 2 ;;
    --class)   want_classes+=("${2:?--class needs a name}"); shift 2 ;;
    --quiet)   quiet=1; shift ;;
    --self-test) selftest=1; shift ;;
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

# AND flock(1), for the same reason and with the same answer: refuse, never
# fall back (ADR 0089 D2). Without it the only gates left are the /proc scan —
# which ADR 0089 MEASURED reading a LIVE test home as collectable, because
# os.Setenv never rewrites /proc/<pid>/environ — and the grace, which a 60m
# `-timeout` arm on a loaded box outlives. A silent fall-back to those two is
# precisely the state this bead was filed to end, and it would look like a
# clean run. util-linux has shipped flock(1) everywhere this script can run
# (it needs /proc, so linux); a seat whose cage does not expose /usr/bin/flock
# gets this line and exit 2, and the operator names the dir on the launch
# line.
command -v flock >/dev/null 2>&1 ||
  die2 "no flock(1) (util-linux) — ADR 0089 D2's proof-of-death is the creator's own lock, and without it the remaining gates were MEASURED calling a live run's temp HOME collectable. Refusing rather than sweeping on the scan and the grace alone."

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
coldexec-pins|coldexec-pins*|120|1
testparallel-selftest|testparallel-selftest*|120|1
pkcensus-selftest|pkcensus-selftest.*|120|1
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

# ─── ADR 0089 D2 STEP 1: THE CREATOR'S OWN LOCK ─────────────────────────────
#
# The first of the three gates, and the only one that is a POSITIVE proof of
# life rather than a failure to find death. A dir whose creator is alive
# cannot be collected, whatever the scan missed and whatever the mtime says.
#
# `flock -n` and never a blocking one: the question is "is anybody holding
# this right now", and a wait would turn the reaper into a process that parks
# for the length of someone else's suite.
#
# -E 9 SO THE TWO WAYS OF FAILING STAY APART. A plain `flock` exits 1 both
# when the lock is taken and when it could not open the dir at all, and the
# two are different facts a reader needs: the first is a working seat, the
# second is a permission or a race the operator should know about. Both keep
# the dir — anything other than a clean acquisition does — but a sweep that
# reported them as one thing would hide the second behind the first forever.
#
# BUSY IS REPORTED, NOT SWALLOWED. A run whose numbers moved because a seat
# was working has to say so, or the next reader prices the reaper against a
# sweep that was never allowed to happen. The skipped dirs go in a FILE
# rather than on stdout because this function runs inside the `found=$( … )`
# command substitution, where stdout is the candidate list and where a
# counter increment would die with the subshell.
#
# A VANISHED CANDIDATE IS DROPPED BEFORE THE PROBE, because `flock` opens its
# lock path with O_CREAT (ranger-base-x2blo, found by ranger-base-9rbfq).
# MEASURED 2026-11-11, flock from util-linux 2.42.4: a free dir answers rc=0,
# a busy one rc=9, a mode-000 one rc=66 — the three answers the case below
# claims to distinguish — and a path that does NOT EXIST answers rc=0 with
# `flock` having CREATED it, `-rw-r--r--`. Without this guard a dir removed
# between the walk and the probe came back as a 0-byte regular FILE at that
# path, reported clean and offered as collectable; in a dry run nothing then
# removed it, and since every class's `find` is `-type d` no later run could
# ever see it again — the sweep leaking an inode it made itself, on the quota
# it exists to protect, and against this file's own claim two decisions up
# that "a busy lock can only ever make this script collect LESS". `-d` and
# not `-e`: a candidate is a directory or it is not a candidate. The removal
# loop at the foot of this script opens with the same O_CREAT and needs no
# guard, because its own `rm -rf` takes back whatever `flock` just made.
flock_filter() {  # $1 = file to record the dirs this run may not touch
  local out="$1" d rc
  : > "$out"
  while IFS= read -r d; do
    [ -n "$d" ] || continue
    [ -d "$d" ] || continue
    rc=0
    flock -n -E 9 "$d" true 2>/dev/null || rc=$?
    case "$rc" in
      0) printf '%s\n' "$d" ;;
      9) printf 'HELD (lock) %s\n' "$d" >> "$out" ;;
      *) printf 'UNREADABLE (flock exit %s) %s\n' "$rc" "$d" >> "$out" ;;
    esac
  done
}

# THE SELF-TEST DISPATCHES HERE, below `flock_filter`, and not up beside the
# argument parsing where it used to sit (ranger-base-x2blo): one of its arms
# calls `flock_filter` directly, and a shell function has to be DEFINED
# before it is called, not merely before it is read. The two preflights it
# now passes through on the way — /proc and flock(1) — are both things every
# arm's `"$0"` sub-invocation needs anyway, so a box missing either gets one
# clear line instead of a dozen failing arms.
if [ "$selftest" -eq 1 ]; then
  self_test
  exit $?
fi

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

# ONE DIR, SIX FILES INSIDE IT (ranger-base-u1z89). These were six bare
# `mktemp` FILES: six inodes, in a root this script itself sweeps, under
# mktemp(1)'s `tmp.XXXXXXXXXX` namespace — which the `mktemp` class globs by
# name and which every candidate walk below then excludes with `-type d`. So
# the reaper's own leavings were in the one leak class the reaper cannot see,
# on the quota it exists to protect. One `mktemp -d` is also one fork instead
# of six, and `$stale.body` (the bead arm, at the foot of this script) lands
# inside it as a seventh.
#
# The six are then CREATED EMPTY, which mktemp did for free: several of them
# are read before anything writes to them (`grep -c . < "$lockheld"` for a
# class whose walk found nothing, and `$stale` on the way into the dedupe
# filter), and a missing file there would be a diagnostic about this script
# instead of a count.
scratch=$(mktemp -d "${TMPDIR:-/tmp}/posse-reap.XXXXXX") || die2 "mktemp -d failed — there is nowhere to keep this run's candidate lists"
held="$scratch/held" held3="$scratch/held3" stale="$scratch/stale"
rootlist="$scratch/rootlist" livedirs="$scratch/livedirs" lockheld="$scratch/lockheld"
: > "$held"; : > "$held3"; : > "$stale"; : > "$rootlist"; : > "$livedirs"; : > "$lockheld"
trap 'rm -rf -- "${scratch:?}"' EXIT

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

# AND THIS RUN'S OWN SCRATCH DIR, which is now a `posse-*` dir at depth 1 in
# a root this script sweeps (ranger-base-u1z89). Appended BELOW the report
# above on purpose: it is not something the /proc scan found, and a line
# reading `HELD <this run's own dir>` under a count of what live processes
# name would be a false reading of the one number this script asks a reader
# to trust.
#
# The scan genuinely cannot see it. `rawrefs` is read BEFORE the dir exists,
# and even afterwards the path lives in a shell variable, which is ADR 0089's
# own "invisible to the scan" one step over. What has kept it out so far is
# the grace — 120m against a dir that lives seconds — but `--min-age 0`, or
# `--min-age` under any value a run can outlive, would nominate it, take its
# lock, and `rm -rf` the candidate list this script is reading from. Until
# this bead the HOLE was what protected it: the six files it replaced were
# `-type f` and no walk could see them.
#
# SAID OUT LOUD, because an exclusion nothing prints is an exclusion nobody
# can check: the window it covers needs a run lasting longer than its own
# `--min-age`, which no ordinary run reaches, so this line is the only thing
# a reader — or a rig — can read it off.
printf '%s\n' "$scratch" >> "$held"
[ "$quiet" -eq 1 ] || echo "this run's own scratch dir, held out of every class: $scratch"

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
#
# A `claude -p`/`--print` RUN IS NOT PART OF THIS POPULATION (ranger-base-2smyx
# finding 4). It never writes a registry file — there is no session to resume
# and no pane to idle in — so counting it alongside interactive sessions makes
# the blindness control refuse the whole class for a reason that is not
# blindness: this repo's own suite runs live `claude -p` pins, so a quiet box
# with one sibling seat mid-pin and no interactive session alive reads as
# blind when nothing is wrong. claude_could_register excludes exactly the runs
# that could never appear in $claude_sessions; an unreadable cmdline (the
# process died, or /proc denies it) counts the pid anyway, which keeps the
# control's fail-safe direction — it can only ever make claude_procs equal to
# or smaller than a hand-count would, never larger, so this can only move the
# control AWAY from a spurious refusal and never toward a missed one.
claude_could_register() {  # claude_could_register <pid-dir, e.g. /proc/1234>
  local arg
  while IFS= read -r -d '' arg; do
    case "$arg" in -p|--print) return 1 ;; esac
  done < "$1/cmdline" 2>/dev/null
  return 0
}
# IT REPORTS WHICH PIDS IT COUNTED and not only how many (ranger-base-0wvu6).
# Two readers need that. A human reading the REFUSED line below is told a
# number and a registry that does not name it, and the next question is always
# "which processes?" — a count cannot be chased and a pid can. And the
# `--self-test` arm for this exclusion can then ask the question it actually
# means — is MY fixture's pid in the counted set — instead of differencing two
# box-wide censuses taken a second apart, which is a subtraction any sibling
# seat's launch moves (that rig red two arms of `make test` on this box on
# 2026-11-11 and again on 2026-11-12, both times over an unrelated claude
# process that arrived mid-rig). The list is derived in the same loop as the
# count, so the two cannot disagree.
claude_blind=0 claude_live=0 claude_procs=0 claude_counted=""
for pid in /proc/[0-9]*; do
  case "$(readlink "$pid/exe" 2>/dev/null)" in
    */claude)
      if claude_could_register "$pid"; then
        claude_procs=$((claude_procs + 1))
        claude_counted="$claude_counted ${pid#/proc/}"
      fi
      ;;
  esac
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
# Its own line, below the count and never folded into it: the count line is
# parsed by `${out#*"live claude processes: "}` cut at the next newline, here
# and in --self-test, so anything appended to it would be read as part of the
# number. Empty when nothing was counted — the count line above says 0.
echo "claude pids counted:$claude_counted"
sort -u "$livedirs" "$held3" -o "$held3"

# ─── the candidate walk, class by class ─────────────────────────────────────
# ADR 0089 D2's three gates, in this order: `flock_filter` (step 1, the
# creator's own lock — a busy lock is proof the creator is alive, and it is
# what stopped posse-testhome-*'s grace from carrying the whole weight), then
# the held-set filter (step 2, the /proc scan, unchanged), then the grace,
# which `find -mmin` has already applied by the time anything reaches here
# (step 3, unchanged).
#
# THE LOCK IS FIRST BECAUSE IT IS THE CHEAPEST TRUE ANSWER: one open and one
# non-blocking syscall per candidate, against a scan of every /proc entry
# that is already done by now anyway. Order is not correctness here — the
# three gates are ANDed and any one of them keeps a dir — it is only which
# reason a reader is shown for a dir that two of them would have kept.
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
    done | sort -u | flock_filter "$lockheld" | { grep -vxF -f "$heldfile" || true; } | { grep -vxF -f "$stale" || true; } | class_filter "$cname" "$grace"
  )
  cn=$(printf '%s\n' "$found" | grep -c . || true)
  lk=$(grep -c . < "$lockheld" || true)
  if [ "$lk" -gt 0 ]; then
    echo "class $cname: $lk dir(s) kept by ADR 0089 D2 step 1 — a live creator's own flock, so the scan was not needed"
    if [ "$quiet" -eq 0 ]; then sed 's/^/  /' < "$lockheld"; fi
  fi
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

# REMOVED UNDER THE LOCK, one dir at a time (ADR 0089 D2). The walk above
# took the lock, let it go, and then went on to scan /proc and price the set —
# seconds in which a creator that was microseconds past its MkdirTemp can
# arrive and announce itself. Taking the lock again here closes that window:
# whatever this removes, it removes while holding the one thing a live
# creator would be holding instead. It costs nothing, because a creator never
# reopens its home by path, so there is nobody for this to block.
#
# This is why the removal is a loop rather than the one `xargs … rm -rf` it
# replaced: the lock is per-dir, and a dir that was claimed between the walk
# and now has to be SAID rather than taken. `${d:?}` because an empty
# expansion here would be `rm -rf --`, and the guard is what makes that
# impossible rather than merely unlikely (AGENTS.md).
if [ "$n" -gt 0 ]; then
  removed=0 claimed=0 failed=0
  while IFS= read -r d; do
    [ -n "$d" ] || continue
    rc=0
    flock -n -E 9 "$d" rm -rf -- "${d:?}" || rc=$?
    case "$rc" in
      0) removed=$((removed + 1)) ;;
      9) claimed=$((claimed + 1)); echo "  NOT removed, a creator took the lock since the walk: $d" ;;
      *) failed=$((failed + 1));  echo "  NOT removed, rm or flock exited $rc: $d" ;;
    esac
  done < "$stale"
  echo "collected $removed of $n dirs (claimed since the walk: $claimed, failed: $failed)"
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
