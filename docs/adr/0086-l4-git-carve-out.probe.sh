#!/usr/bin/env bash
# ADR 0086 D8 (ranger-base-56w67): the container tier's git carve-out for a
# PID that denies the whole tree, BEFORE and AFTER D8, in both shapes a
# session tree has.
#
#   bash docs/adr/0086-l4-git-carve-out.probe.sh [parent-dir]
#
# The L4 engine is off this box (the seat cannot reach docker.sock), so this
# mirrors CageMounts' mount LIST under bwrap: `--ro-bind / /` stands for the
# `:ro` repo and the `:ro` common dir, and each `--bind` is one read-write
# overlay CageMounts renders today. What it measures is git's behaviour under
# that layering — which writes need which overlay — not the docker engine's
# bind semantics, which ADR 0014's probes measured (7/7, depth-ordered).
#
# Expected: "L4 today" commits by plumbing in both shapes (the linked
# worktree's commit is one the launcher's splice fast-forwards posse/s1 onto
# at close; the main checkout moves refs/heads/main directly); "proposed"
# refuses both, and `git status`/`git log` still work in every arm.
# Exit 0 always; read the lines. Writes only under a fresh temp dir it removes.
set -u
G=/usr/bin/git
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=p GIT_AUTHOR_EMAIL=p@x GIT_COMMITTER_NAME=p GIT_COMMITTER_EMAIL=p@x
R=$(mktemp -d "${1:-${TMPDIR:-/tmp}}/l4.XXXX"); cd "$R"
$G init -q -b main repo && cd repo && echo a > f && $G add f && $G commit -qm base && cd ..
$G -C repo branch posse/s1 && $G -C repo worktree add -q ../wt posse/s1
$G -C wt checkout -q --detach        # PrepareSessionHead's detach
mkdir -p repo/.git/logs
COMMON=$R/repo/.git; OWN=$COMMON/worktrees/wt
base() { bwrap --ro-bind / / --dev /dev --proc /proc "$@"; }
# the seat's attempt: a commit of content it could not edit, by plumbing
plumb_wt='cd '"$R"'/wt; : > newfile 2>/dev/null && echo "  cwd create: ok" || echo "  cwd create: refused";
  '"$G"' status --short >/dev/null 2>&1 && echo "  status: ok" || echo "  status: FAIL";
  '"$G"' log --oneline -1 >/dev/null 2>&1 && echo "  log: ok" || echo "  log: FAIL";
  b=$(echo injected | '"$G"' hash-object -w --stdin 2>/dev/null) && echo "  hash-object -w: ok" || echo "  hash-object -w: refused";
  '"$G"' update-index --cacheinfo 100644,${b:-0000000000000000000000000000000000000000},f 2>/dev/null && '"$G"' commit -qm injected 2>/dev/null && echo "  commit (detached): ok" || echo "  commit (detached): refused"'
echo "LINKED WORKTREE, L4 today (common :ro + rw own/objects/logs):"
base --bind $OWN $OWN --bind $COMMON/objects $COMMON/objects --bind $COMMON/logs $COMMON/logs bash -c "$plumb_wt"
tip=$($G -C wt rev-parse HEAD); br=$($G -C repo rev-parse posse/s1)
$G -C repo merge-base --is-ancestor $br $tip && [ "$tip" != "$br" ] && echo "  => branch posse/s1 fast-forwards to the injected commit (what spliceDetachedWork does at close): $($G -C repo show -s --format=%s $tip)"
$G -C wt reset -q --hard $br
echo "LINKED WORKTREE, proposed (no git-dir overlay):"
base bash -c "$plumb_wt"
[ "$($G -C wt rev-parse HEAD)" = "$br" ] && echo "  => worktree HEAD unchanged"
plumb_main='cd '"$R"'/repo; : > newfile 2>/dev/null && echo "  cwd create: ok" || echo "  cwd create: refused";
  '"$G"' status --short >/dev/null 2>&1 && echo "  status: ok" || echo "  status: FAIL";
  b=$(echo injected | '"$G"' hash-object -w --stdin 2>/dev/null) && echo "  hash-object -w: ok" || echo "  hash-object -w: refused";
  t=$(printf "100644 blob %s\tf\n" "${b:-0}" | '"$G"' mktree 2>/dev/null); c=$(echo injected | '"$G"' commit-tree ${t:-0} -p main 2>/dev/null);
  '"$G"' update-ref refs/heads/main ${c:-0} 2>/dev/null && echo "  update-ref refs/heads/main: ok" || echo "  update-ref refs/heads/main: refused"'
m0=$($G -C repo rev-parse main)
echo "MAIN CHECKOUT, L4 today (repo :ro + rw .git, :ro config & hooks):"
base --bind $COMMON $COMMON --ro-bind $COMMON/config $COMMON/config --ro-bind $COMMON/hooks $COMMON/hooks bash -c "$plumb_main"
[ "$($G -C repo rev-parse main)" != "$m0" ] && echo "  => refs/heads/main MOVED to: $($G -C repo show -s --format=%s main)"
$G -C repo update-ref refs/heads/main $m0
echo "MAIN CHECKOUT, proposed (no .git carve-out):"
base bash -c "$plumb_main"
[ "$($G -C repo rev-parse main)" = "$m0" ] && echo "  => refs/heads/main unchanged"
rm -rf "$R"
