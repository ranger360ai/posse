#!/bin/sh
# The L4 worktree git grant, narrowed (ranger-base-t4f1, closing the gap
# ranger-base-6q5e assessed; ADR 0014 §4, and ADR 0038 decision 4's L4 twin,
# folded here as ranger-base-mugt2).
#
#   sh docs/adr/0014-l4-worktree-narrowing.probe.sh
#
# WHAT CHANGED. ranger-base-yu5 mounted a worktree session's git COMMON dir
# read-write whole. That hands a caged persona `refs/heads/main` (an
# update-ref is not a push: the L1 shim never sees it and L3's pre-push never
# fires), `packed-refs`, `config` — where a planted `core.hooksPath` DODGES
# the hooks-:ro overlay of ranger-base-3c3/h15 rather than being stopped by
# it — and other sessions' `worktrees/<name>`. At the one tier whose job is
# containing a prompt-injected persona. What ships now: the common dir `:ro`,
# with read-write overlays of `worktrees/<own>`, `objects` and `logs`, and the
# session launched on a DETACHED HEAD so no ref write is needed at all (the
# launcher splices the work back onto the branch at close).
#
# TWO PARTS, and they measure different walls.
#
#   PART A — the host rehearsal, no engine. The wall is uid permissions
#   (chmod) rather than a bind mount, so it is NOT the L4 measurement. What it
#   does answer is the half that is about GIT rather than about the engine:
#   given exactly those three writable regions and nothing else, does a
#   detached commit land, what does it print, what does `gc --auto` do, and
#   does the same commit ON the branch fail. Every arm carries the control
#   with the wall taken away.
#
#   PART B — the container tier itself, bind mounts on a real engine. This is
#   the arm that says the mount SET behaves as rendered.
#
# Part A is MEASURED (see the stamp under its banner). Part B is UNRUN, and
# that is stated rather than hidden: Docker was abandoned on the box this
# landed from by operator ruling of 2026-08-30 (ranger-base-6mz7) and no other
# engine is installed there. Its foundation is measured elsewhere —
# docs/adr/0014-path-scoped-writes.probe.sh, 7/7 on Docker 29.0.1 / VirtioFS,
# which is where "a read-write bind lands over a :ro bind of its parent,
# depth-ordered" comes from. What Part B adds is that composition on real git
# plumbing at the paths this bead names.
#
# Read the VALUES, not the exit status. A refusal proves nothing unless the
# arm without the wall succeeds.

cd / || exit 1
WORK=$(mktemp -d) || exit 1
WORK=$(cd "$WORK" && pwd -P)
trap 'chmod -R u+w "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT

# ─── the fixture both parts share ───────────────────────────────────────────
# A repo with TWO session worktrees, so "another session's tree" is a real
# neighbour and not a path that does not exist. `wt` is the session under
# test: its logs/ is made and its HEAD detached, which is what the launcher
# does (PrepareSessionHead, worktree.go).
#
# A FRESH one per arm: a control that mutates a shared fixture makes every
# later arm measure the leftovers.
#
# Resolved (`pwd -P`): mounts are same-path in and out, and on macOS mktemp
# answers under /var/folders while the real path is /private/var/folders. A
# fixture mixing the two spellings measures the symlink.
fixture() {
  f=$WORK/$1
  rm -rf "$f"; mkdir -p "$f/main"
  git -C "$f/main" init -q -b main . >/dev/null 2>&1
  git -C "$f/main" config user.email t@example.com
  git -C "$f/main" config user.name t
  echo seed > "$f/main/README.md"
  git -C "$f/main" add README.md
  # Path-limited: posse's own L3 commit wall refuses the bare form, and a
  # probe a persona cannot run is a probe nobody runs.
  git -C "$f/main" commit -q -m seed -- README.md
  git -C "$f/main" worktree add -q "$f/wt" -b posse/s-1
  git -C "$f/main" worktree add -q "$f/other" -b posse/s-2
  mkdir -p "$f/main/.git/logs"
  git -C "$f/wt" checkout -q --detach
  echo "$f"
}

echo "== git =="
git --version

# ═══ PART A — the host rehearsal (uid permissions, no engine) ═══════════════
#
# MEASURED 2026-09-05, darwin 25.4.0, git 2.50.1 (Apple Git-155),
# ranger-base-t4f1. Every arm answered its expect line and every control
# disagreed with the arm beside it: A1 commit=ok / A2 commit=refused bracket
# A3's commit=ok, and A3c has all seven paths writable where A3 refuses six of
# them (update-ref, config, hookspath, hooks, other-session, packed-refs) and
# grants only own-tree. Two of the arms are findings rather than
# confirmations:
#
#   - the DETACH is load-bearing and now measured: the identical commit ON
#     the branch fails at `cannot lock ref 'HEAD': Unable to create
#     refs/heads/posse/s-1.lock`. So the three overlays are enough only
#     because HEAD is detached, which is the whole shape of the narrowing.
#   - `<common>/logs` is NOT what a detached commit writes: in a linked
#     worktree HEAD's reflog is per-worktree (`worktrees/<own>/logs/HEAD`,
#     measured 4 lines), and `<common>/logs/refs/heads/<branch>` appears only
#     when a commit moves a SHARED ref — which is exactly what detaching
#     removes. The overlay and the launcher's mkdir are kept anyway, for the
#     reasons sessionCommonDirWrites states: L2 grants the same three, and a
#     git operation inside the cage that does update a shared ref (a fetch, a
#     note) gets a fatal rather than a silently skipped reflog.
#
echo
echo "═══ PART A — host rehearsal: uid permissions, not the L4 wall ═══"

# ro <common> — everything read-only. rw <paths> — carved back.
ro() { chmod -R a-w "$1" 2>/dev/null; }
rw() { chmod -R u+w "$@" 2>/dev/null; }

# One commit the way a persona makes it, in a subshell so a removed fixture
# cannot take the script's cwd with it.
commit() (
  cd "$1/wt" 2>/dev/null || { echo "commit=no-tree"; exit 0; }
  echo work > fix.txt
  git add fix.txt 2>>"$1/err"
  if git commit -q -m "caged: the fix" -- fix.txt 2>>"$1/err"; then echo commit=ok; else echo commit=refused; fi
)
said() { sed 's/^/   said: /' "$1/err" 2>/dev/null | head -2; }

echo
echo "-- A1 CONTROL: nothing read-only        (expect commit=ok)"
echo "   if this refuses, every arm below measured the fixture and not a wall"
F=$(fixture a1); commit "$F"; said "$F"

echo
echo "-- A2 CONTROL: whole common dir read-only, nothing carved back  (expect commit=refused)"
echo "   if this succeeds, the chmod is not a wall here and A3 measures nothing"
F=$(fixture a2); ro "$F/main/.git"; commit "$F"; said "$F"; rw "$F/main/.git"

echo
echo "-- A3 ARM: the three regions carved back, detached  (expect commit=ok)"
echo "   the capability the narrowing must not cost — 'L4 worktree sessions"
echo "   do not commit' is what ranger-base-6q5e assessed and rejected"
F=$(fixture a3); C=$F/main/.git; ro "$C"; rw "$C/worktrees/wt" "$C/objects" "$C/logs"
commit "$F"; said "$F"
( cd "$F/wt" 2>/dev/null || exit 0
  b=$(git -C "$C" rev-parse refs/heads/posse/s-1)
  h=$(git rev-parse HEAD)
  [ "$b" = "$h" ] && echo "   branch-moved=yes" || echo "   branch-moved=no"
  git update-ref refs/heads/main HEAD 2>/dev/null && echo "   update-ref=ok" || echo "   update-ref=refused"
  # config through GIT, not an append. Two reasons, and the second cost this
  # script a false control before it was found: `touch` on an existing file
  # the owner owns succeeds whatever the mode says (utimensat is a uid
  # promise, not a write), and a RAW append leaves `x` in .git/config, after
  # which every later git command in the fixture dies on "bad config line" —
  # so the control arm reported update-ref=refused for a corrupted config
  # rather than for a wall. `--local` is the common-dir config in a worktree
  # (--worktree is the other one), which is the file ADR 0038 names.
  git config --local probe.wall 1 2>/dev/null && echo "   config=writable" || echo "   config=refused"
  git config --local core.hooksPath /tmp/evil 2>/dev/null && echo "   hookspath=planted" || echo "   hookspath=refused"
  touch "$C/hooks/pre-push" 2>/dev/null && echo "   hooks=writable" || echo "   hooks=refused"
  touch "$C/worktrees/other/HEAD.probe" 2>/dev/null && echo "   other-session=writable" || echo "   other-session=refused"
  touch "$C/worktrees/wt/.probe" 2>/dev/null && echo "   own-tree=writable" || echo "   own-tree=refused"
  head=$(git rev-parse HEAD)
  git gc --auto >/dev/null 2>&1; echo "   gc-exit=$?"
  [ "$(git rev-parse HEAD)" = "$head" ] && echo "   head-intact=yes" || echo "   head-intact=no"
  # LAST, because a raw append is the only honest test of packed-refs and it
  # leaves the ref store unreadable for anything after it.
  { echo x >> "$C/packed-refs"; } 2>/dev/null && echo "   packed-refs=writable" || echo "   packed-refs=refused" )
rw "$C"

echo
echo "-- A3c CONTROL: the same seven paths with the wall removed  (expect all writable/ok/planted)"
F=$(fixture a3c); C=$F/main/.git
( cd "$F/wt" || exit 0
  git update-ref refs/heads/main HEAD 2>/dev/null && echo "   update-ref=ok" || echo "   update-ref=refused"
  git config --local probe.wall 1 2>/dev/null && echo "   config=writable" || echo "   config=refused"
  git config --local core.hooksPath /tmp/evil 2>/dev/null && echo "   hookspath=planted" || echo "   hookspath=refused"
  touch "$C/hooks/pre-push" 2>/dev/null && echo "   hooks=writable" || echo "   hooks=refused"
  touch "$C/worktrees/other/HEAD.probe" 2>/dev/null && echo "   other-session=writable" || echo "   other-session=refused"
  touch "$C/worktrees/wt/.probe" 2>/dev/null && echo "   own-tree=writable" || echo "   own-tree=refused"
  { echo x >> "$C/packed-refs"; } 2>/dev/null && echo "   packed-refs=writable" || echo "   packed-refs=refused" )

echo
echo "-- A4 ARM: the SAME grant, HEAD on the branch instead  (expect commit=refused)"
echo "   the arm that says the detach is load-bearing rather than decorative"
F=$(fixture a4); C=$F/main/.git; git -C "$F/wt" checkout -q posse/s-1
ro "$C"; rw "$C/worktrees/wt" "$C/objects" "$C/logs"; commit "$F"; said "$F"; rw "$C"

echo
echo "-- A5 ARM: <common>/logs absent, the launcher's mkdir removed"
echo "   (expect commit=ok, and common/logs still absent — a DETACHED commit's"
echo "    reflog is per-worktree; this is what the mkdir does NOT buy)"
F=$(fixture a5); C=$F/main/.git; rm -rf "$C/logs"; ro "$C"; rw "$C/worktrees/wt" "$C/objects"
commit "$F"; said "$F"
echo "   common/logs after:   $(ls -d "$C/logs" 2>/dev/null || echo absent)"
echo "   own/logs after:      $(ls "$C/worktrees/wt/logs" 2>/dev/null | tr '\n' ' ')"
rw "$C"

echo
echo "-- A5b ARM: the same tree, HEAD on the branch, logs absent"
echo "   (expect <common>/logs/refs/heads/posse/s-1 to be what a SHARED ref"
echo "    update needs — the reason the overlay stays in the set)"
F=$(fixture a5b); C=$F/main/.git; rm -rf "$C/logs"; git -C "$F/wt" checkout -q posse/s-1
commit "$F" >/dev/null; said "$F"
echo "   common/logs after:   $(find "$C/logs" -type f 2>/dev/null | sed "s|$C/||" | tr '\n' ' ')"

# ═══ PART B — the container tier (bind mounts, needs an engine) ═════════════
echo
echo "═══ PART B — the container tier: bind mounts on a real engine ═══"
ENGINE=${ENGINE:-docker}
# git INSIDE the image: this measures git against the mounts, not the shell.
IMG=${IMG:-alpine/git}
if ! command -v "$ENGINE" >/dev/null 2>&1; then
  echo "no $ENGINE on PATH — Part B not measured (that is not a pass)"; exit 2
fi
if ! "$ENGINE" version >/dev/null 2>&1; then
  echo "$ENGINE is on PATH but its daemon is down — Part B not measured (that is not a pass)"
  echo "(on the box this landed from that is the 2026-08-30 ruling, ranger-base-6mz7)"
  exit 2
fi
echo "engine: $("$ENGINE" version --format '{{.Server.Version}}' 2>/dev/null || "$ENGINE" --version)"

probe() { label=$1; shift; printf '%s\n' "-- $label"; "$ENGINE" run --rm "$@" 2>&1 | sed 's/^/   /'; }
# The narrowed mount set, exactly as cage.go renders it.
narrow() { echo "-v $1/wt:$1/wt -v $1/main/.git:$1/main/.git:ro -v $1/main/.git/worktrees/wt:$1/main/.git/worktrees/wt -v $1/main/.git/objects:$1/main/.git/objects -v $1/main/.git/logs:$1/main/.git/logs"; }
# The same with the wall removed: the common dir read-write whole (yu5).
wide()   { echo "-v $1/wt:$1/wt -v $1/main/.git:$1/main/.git"; }

echo
echo "== B1. commit, detached, common:ro + the three overlays =="
F=$(fixture b1)
probe "expect commit=ok branch-moved=no" $(narrow "$F") -w "$F/wt" "$IMG" sh -c '
git config user.email c@example.com; git config user.name c
before=$(git -C "$1/main/.git" rev-parse refs/heads/posse/s-1)
echo work > fix.txt; git add fix.txt >/dev/null 2>&1
git commit -q -m "caged: the fix" -- fix.txt >/dev/null 2>&1 && echo commit=ok || echo commit=refused
after=$(git -C "$1/main/.git" rev-parse refs/heads/posse/s-1)
[ "$before" = "$after" ] && echo branch-moved=no || echo branch-moved=yes' sh "$F"

echo
echo "== B1c. CONTROL: the three overlays REMOVED =="
F=$(fixture b1c)
probe "expect commit=refused" -v "$F/wt:$F/wt" -v "$F/main/.git:$F/main/.git:ro" -w "$F/wt" "$IMG" sh -c '
git config user.email c@example.com; git config user.name c
echo work > fix.txt; git add fix.txt >/dev/null 2>&1
git commit -q -m "caged: the fix" -- fix.txt >/dev/null 2>&1 && echo commit=ok || echo commit=refused'

echo
echo "== B2. the five paths the narrowing denies =="
echo "   (update-ref: not a push, so the mount is the only wall there is."
echo "    config+hooks: ADR 0038 decision 4's two paths, which this subsumes."
echo "    other-session: the sibling of the one worktrees/ dir that IS granted)"
F=$(fixture b2)
probe "expect update-ref=refused config=refused hookspath=refused hooks=refused other-session=refused own-tree=writable packed-refs=refused" \
  $(narrow "$F") -w "$F/wt" "$IMG" sh -c '
C=$1/main/.git
git update-ref refs/heads/main HEAD 2>/dev/null && echo update-ref=ok || echo update-ref=refused
git config --local probe.wall 1 2>/dev/null && echo config=writable || echo config=refused
git config --local core.hooksPath /tmp/evil 2>/dev/null && echo hookspath=planted || echo hookspath=refused
touch "$C/hooks/pre-push" 2>/dev/null && echo hooks=writable || echo hooks=refused
touch "$C/worktrees/other/HEAD.probe" 2>/dev/null && echo other-session=writable || echo other-session=refused
touch "$C/worktrees/wt/.probe" 2>/dev/null && echo own-tree=writable || echo own-tree=refused
{ echo x >> "$C/packed-refs"; } 2>/dev/null && echo packed-refs=writable || echo packed-refs=refused' sh "$F"

echo
echo "== B2c. CONTROL: the wall removed — common dir read-write whole (yu5) =="
F=$(fixture b2c)
probe "expect every one of them writable/ok/planted — this is the hole the bead closes" \
  $(wide "$F") -w "$F/wt" "$IMG" sh -c '
C=$1/main/.git
git update-ref refs/heads/main HEAD 2>/dev/null && echo update-ref=ok || echo update-ref=refused
git config --local probe.wall 1 2>/dev/null && echo config=writable || echo config=refused
git config --local core.hooksPath /tmp/evil 2>/dev/null && echo hookspath=planted || echo hookspath=refused
touch "$C/hooks/pre-push" 2>/dev/null && echo hooks=writable || echo hooks=refused
touch "$C/worktrees/other/HEAD.probe" 2>/dev/null && echo other-session=writable || echo other-session=refused
touch "$C/worktrees/wt/.probe" 2>/dev/null && echo own-tree=writable || echo own-tree=refused
{ echo x >> "$C/packed-refs"; } 2>/dev/null && echo packed-refs=writable || echo packed-refs=refused' sh "$F"

echo
echo "== B3. gc --auto after a commit, packed-refs :ro =="
echo "   (the one arm that is a corollary of nothing. L2 already records the"
echo "    sibling fact — every commit under the narrowed profile prints"
echo "    'Unable to create packed-refs.lock' and still succeeds, because git"
echo "    takes that lock speculatively and falls back. If gc's exit is"
echo "    NONZERO here, or HEAD is not intact after it, the narrowing costs a"
echo "    capability and this bead has to say so)"
F=$(fixture b3)
probe "expect commit=ok gc-exit=0 head-intact=yes" $(narrow "$F") -w "$F/wt" "$IMG" sh -c '
git config user.email c@example.com; git config user.name c
echo work > fix.txt; git add fix.txt >/dev/null 2>&1
git commit -q -m "caged: the fix" -- fix.txt >/dev/null 2>&1 && echo commit=ok || echo commit=refused
head=$(git rev-parse HEAD)
git gc --auto >/dev/null 2>&1; echo "gc-exit=$?"
git gc --auto 2>&1 | sed "s/^/gc-said=/" | head -3
[ "$(git rev-parse HEAD)" = "$head" ] && echo head-intact=yes || echo head-intact=no
git cat-file -e "$head" 2>/dev/null && echo object-alive=yes || echo object-alive=no'

echo
echo "== B4. the same grant with HEAD ON the branch =="
echo "   (the detach is the mechanism, not a detail — Part A measured this"
echo "    against uid permissions and it must answer the same way here)"
F=$(fixture b4); git -C "$F/wt" checkout -q posse/s-1
probe "expect commit=refused, naming refs/heads/posse/s-1.lock" $(narrow "$F") -w "$F/wt" "$IMG" sh -c '
git config user.email c@example.com; git config user.name c
echo work > fix.txt; git add fix.txt >/dev/null 2>&1
git commit -m "caged: the fix" -- fix.txt 2>&1 | sed "s/^/said=/" | head -2
git rev-parse --verify --quiet refs/heads/posse/s-1 >/dev/null && echo branch-exists=yes'

echo
echo "== verdict =="
echo "B1 is the capability; B2 is the narrowing; B3 is the one cost that could"
echo "still make it not worth having; B4 is why the launcher detaches."
echo
echo "If any arm disagrees with its expect line on YOUR engine, comment"
echo "DIVERGED: on ranger-base-t4f1 naming what the engine did. If a CONTROL"
echo "disagrees, the fixture is wrong and the arm beside it measured nothing"
echo "— fix the fixture before reading any refusal as a wall."
echo
echo "One trap that is not the engine's, for anyone re-running these binds by"
echo "hand in ZSH: \"\$R:\$R:ro\" is not what it looks like. \`:r\` is a zsh"
echo "modifier (strip the extension), so the word becomes \$R:\${R}o — docker"
echo "then binds an EMPTY auto-created directory read-WRITE at a destination"
echo "one character off, and the probe reads as 'the engine ignored :ro'."
echo "Spell it \"\${R}:\${R}:ro\". This script is /bin/sh and is not affected."
