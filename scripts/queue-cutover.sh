#!/bin/sh
# queue-cutover.sh — move the beads store of record out of the constitution
# repo into its own git repo (ADR 0015 §4, ranger-base-tjfw).
#
# WHAT IT DOES, in the order it does it:
#   1. refuses unless the ground is what it expects (a live daemon, a dirty
#      store, an existing queue repo and a store that already redirects are
#      all refusals — every one of them means the window is not open yet);
#   2. replays the `.beads/` history out of the constitution repo into a NEW
#      repo, keeping the `.beads/` path prefix and every commit's author,
#      date and message, and NOTHING else from that repo's history;
#   3. moves the live store (database and friends) on top of it;
#   4. leaves the constitution repo holding one file — `.beads/redirect` —
#      and stages the untracking, without committing;
#   5. rewrites `.beads/redirect` in every repo named with --redirect, and in
#      every session worktree under ~/.posse/worktrees;
#   6. commits the live store's drift in the QUEUE repo — LAST, because it is
#      the only step whose failure costs nothing but a commit
#      (ranger-base-nzyn: it used to run inside the window, and a persona
#      cage denying `Bash(git commit unless --)` aborted it there).
#
# Any of 3–6 can fail for ordinary reasons — a hook, a gate refusal, a full
# disk, a ^C. Each one prints the half-state it left and the commands that
# undo it; none of them exits silently.
#
# WHAT IT NEVER DOES: stop or start bd's daemon (the operator's, and this
# script refuses while one is up), commit in the constitution repo, add a git
# remote, or push anything anywhere. A queue repo with no remote cannot push
# even if some future bd flag tries to.
#
# WHY THE HISTORY REPLAY IS NOT OPTIONAL: posse's bead-loss census
# (internal/rhq/beadloss.go) IS the git log of `.beads/issues.jsonl` in
# whatever repo the redirect lands in. A queue repo that starts at one fresh
# commit has no census, so `LostBeads` reports nothing, forever, and the
# alarm that exists because bd deletes rows silently (rangerhq-fuom) is
# disarmed without a word. That is the same failure the alarm was built for.
#
# Usage:
#   scripts/queue-cutover.sh [--constitution DIR] [--queue DIR]
#                            [--redirect DIR]... [--worktrees DIR]
#                            [--dry-run] [--force-daemon]
#
# Defaults are the live paths, so the window step is one line with no
# arguments. Every path is overridable, which is what makes it rehearsable
# on a copy (ranger-base-tjfw's rehearsal ran it exactly this way).

set -eu

CONSTITUTION=${CONSTITUTION:-$HOME/src/ranger-base}
QUEUE=${QUEUE:-$HOME/src/ranger-queue}
WORKTREES=${WORKTREES:-$HOME/.posse/worktrees}
REDIRECTS=${REDIRECTS:-$HOME/src/posse}
DRYRUN=0
FORCE_DAEMON=0

while [ $# -gt 0 ]; do
  case $1 in
    --constitution) CONSTITUTION=$2; shift 2 ;;
    --queue)        QUEUE=$2; shift 2 ;;
    --worktrees)    WORKTREES=$2; shift 2 ;;
    --redirect)     REDIRECTS="$REDIRECTS
$2"; shift 2 ;;
    --only-redirect) REDIRECTS=$2; shift 2 ;;
    --dry-run)      DRYRUN=1; shift ;;
    --force-daemon) FORCE_DAEMON=1; shift ;;
    -h|--help)      sed -n '2,40p' "$0"; exit 0 ;;
    *) echo "queue-cutover: unknown argument $1" >&2; exit 2 ;;
  esac
done

say() { printf '%s\n' "$*"; }
run() { if [ "$DRYRUN" = 1 ]; then printf 'would: %s\n' "$*"; else eval "$@"; fi; }
die() { printf 'queue-cutover: %s\n' "$*" >&2; exit 1; }

SRC_BEADS=$CONSTITUTION/.beads
DST_BEADS=$QUEUE/.beads
RUNBOOK=docs/runbooks/queue-cutover.md

# ─── 0. what an abort leaves behind ──────────────────────────────────────────
# `set -eu` exits SILENTLY, and everything below the preflight is destructive
# until the step after it lands. Measured on ranger-base-lpz4 and filed as
# ranger-base-nzyn: an abort between the mv and the redirect left
# $CONSTITUTION/.beads EMPTY, the store in the queue repo, every redirect in
# the fleet naming the empty directory, and said nothing — bd was dead
# fleet-wide with no message to work from. STAGE names the half-state; the
# trap prints it, and its undo, on any exit that is not this script's own.
STAGE=preflight

on_exit() {
  status=$?
  [ "$status" = 0 ] && return 0
  [ "$DRYRUN" = 1 ] && return 0
  [ "$STAGE" = preflight ] && return 0   # refused before writing anything
  [ "$STAGE" = done ] && return 0
  printf '\n' >&2
  printf 'queue-cutover: ABORTED (exit %s) in stage "%s" — what is on disk now:\n' "$status" "$STAGE" >&2
  case $STAGE in
    queue-repo)
      cat >&2 <<EOF
  The live store was NOT touched: it is still $SRC_BEADS and bd works.
  $QUEUE holds a partial repo.
  UNDO:
    rm -rf '$QUEUE'
  then fix the cause and run this again.
EOF
      ;;
    move)
      cat >&2 <<EOF
  THE STORE IS SPLIT between $SRC_BEADS and $DST_BEADS
  and NOTHING redirects yet — bd is dead fleet-wide until this is undone.
  UNDO (every file back, dotfiles included, then drop the queue repo):
    for f in '$DST_BEADS'/* '$DST_BEADS'/.[!.]*; do
      [ -e "\$f" ] && mv -f "\$f" '$SRC_BEADS/'
    done
    rm -rf '$QUEUE'
  then check the store reads: (cd '$CONSTITUTION' && bd --no-daemon list >/dev/null)
EOF
      ;;
    redirect)
      cat >&2 <<EOF
  The store is in $DST_BEADS. The constitution is half-pointed at it:
  check '$SRC_BEADS/redirect' and 'git -C $CONSTITUTION status --short'.
  UNDO (the store goes home, dotfiles included, and the constitution reverts):
    rm -f '$SRC_BEADS/redirect'
    for f in '$DST_BEADS'/* '$DST_BEADS'/.[!.]*; do
      [ -e "\$f" ] && mv -f "\$f" '$SRC_BEADS/'
    done
    rm -rf '$QUEUE'
    (cd '$CONSTITUTION' && git reset -q HEAD -- .beads .gitignore && git checkout -- .gitignore)
EOF
      ;;
    fanout)
      cat >&2 <<EOF
  The move is DONE: the store is $DST_BEADS and
  '$SRC_BEADS/redirect' names it, so the constitution reads. Only the fleet's
  other redirects are partial — the ones printed above are done, the rest
  still name $SRC_BEADS.
  Re-running this script will (correctly) refuse: the store already
  redirects. Finish the stragglers by hand:
    printf '%s\n' '$DST_BEADS' > <repo>/.beads/redirect
EOF
      ;;
    commit)
      cat >&2 <<EOF
  The move is COMPLETE and every redirect is written — the fleet reads and
  writes normally. All that is missing is the queue repo's own commit of the
  live store's drift, and nothing depends on it.
  FINISH (after fixing the cause — a hook, a gate, a full disk):
    (cd '$QUEUE' && git add -A .beads &&
     git commit -m 'beads: the store of record moves into its own repo (ADR 0015 §4)' -- .beads)
  This one does NOT need a rollback.
EOF
      return 0
      ;;
  esac
  printf 'queue-cutover: full rollback: %s, "Rollback".\n' "$RUNBOOK" >&2
}
trap on_exit EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

# ─── 1. preflight ────────────────────────────────────────────────────────────
[ -d "$CONSTITUTION/.git" ] || die "$CONSTITUTION is not a git checkout"
[ -d "$SRC_BEADS" ] || die "$SRC_BEADS is not there — nothing to move"
[ -f "$SRC_BEADS/redirect" ] && die "$SRC_BEADS already redirects — the store has moved already"
[ -e "$QUEUE" ] && die "$QUEUE already exists — refusing to write into it"
[ -f "$SRC_BEADS/issues.jsonl" ] || die "no issues.jsonl in $SRC_BEADS — this is not the store of record"
git -C "$CONSTITUTION" ls-files --error-unmatch .beads/issues.jsonl >/dev/null 2>&1 ||
  die "$CONSTITUTION does not track .beads/issues.jsonl — there is no history to carry over"

# A daemon holds the database open and re-exports it on a timer; moving the
# directory under it leaves a live process writing to a path nobody reads.
# The stop is the operator's (`bd daemon stop` from $CONSTITUTION) — this
# script only refuses to run while one is up.
if [ -f "$SRC_BEADS/daemon.pid" ] && [ "$FORCE_DAEMON" = 0 ]; then
  pid=$(cat "$SRC_BEADS/daemon.pid" 2>/dev/null || true)
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    die "bd's daemon is running (pid $pid) — stop it first:
    (cd $CONSTITUTION && bd daemon stop)
  then run this again. --force-daemon overrides, and should not be needed."
  fi
fi

# The database is the store of record and the JSONL is a projection of it;
# the replay below carries the PROJECTION's history, so the projection has to
# be current or the queue repo's first state is behind its own database.
if [ "$DRYRUN" = 0 ]; then
  (cd "$CONSTITUTION" && bd --no-daemon sync --flush-only >/dev/null 2>&1) ||
    say "note: bd sync --flush-only did not run cleanly — check issues.jsonl is current"
fi

say "constitution: $CONSTITUTION"
say "queue:        $QUEUE"

# ─── 2. the queue repo, with the .beads history and nothing else ─────────────
# Objects first: a local clone gives the replay every blob and tree it needs
# without a single fetch. Its refs and its remote go away below, and the gc
# then drops every object the replayed chain does not reach — which is all of
# the constitution's prose.
STAGE=queue-repo
run "git clone --quiet --no-hardlinks --no-checkout '$CONSTITUTION' '$QUEUE'"
if [ "$DRYRUN" = 1 ]; then
  say "would: replay .beads/ history into $QUEUE and move the live store onto it"
  say "would: rewrite redirects (see the list below)"
else
  git -C "$QUEUE" remote remove origin

  # Replay. One commit per constitution commit that touched .beads/, each
  # holding a tree with a single entry — .beads — pointing at that commit's
  # own .beads subtree. `git mktree` and `git commit-tree` are the whole
  # mechanism: no checkout, no index, nothing to leave half-done.
  commits=$(git -C "$QUEUE" rev-list --reverse --topo-order HEAD)
  parent=
  n=0
  for c in $commits; do
    sub=$(git -C "$QUEUE" rev-parse -q --verify "$c:.beads" 2>/dev/null) || continue
    [ -n "$sub" ] || continue
    tree=$(printf '040000 tree %s\t.beads\n' "$sub" | git -C "$QUEUE" mktree)
    # An unchanged .beads is not a queue commit: the constitution's own
    # commits must not show up in the queue's log as empty noise.
    if [ -n "$parent" ] && [ "$tree" = "$(git -C "$QUEUE" rev-parse "$parent^{tree}")" ]; then
      continue
    fi
    msg=$(git -C "$QUEUE" log -1 --format=%B "$c")
    new=$(
      GIT_AUTHOR_NAME=$(git -C "$QUEUE" log -1 --format=%an "$c") \
      GIT_AUTHOR_EMAIL=$(git -C "$QUEUE" log -1 --format=%ae "$c") \
      GIT_AUTHOR_DATE=$(git -C "$QUEUE" log -1 --format=%aI "$c") \
      GIT_COMMITTER_NAME=$(git -C "$QUEUE" log -1 --format=%cn "$c") \
      GIT_COMMITTER_EMAIL=$(git -C "$QUEUE" log -1 --format=%ce "$c") \
      GIT_COMMITTER_DATE=$(git -C "$QUEUE" log -1 --format=%cI "$c") \
      git -C "$QUEUE" commit-tree ${parent:+-p "$parent"} -m "$msg" "$tree"
    )
    printf '%s %s\n' "$c" "$new" >> "$QUEUE/.git/queue-cutover-shamap"
    parent=$new
    n=$((n + 1))
  done
  [ -n "$parent" ] || die "the replay produced no commits — .beads has no history in $CONSTITUTION"
  say "replayed $n commit(s) of .beads/ history"

  # Only the replayed chain survives: point main at it, drop every ref the
  # clone brought, and let gc take the rest of the constitution with it.
  git -C "$QUEUE" symbolic-ref HEAD refs/heads/main
  git -C "$QUEUE" update-ref refs/heads/main "$parent"
  git -C "$QUEUE" for-each-ref --format='%(refname)' refs/remotes refs/tags |
    while read -r ref; do git -C "$QUEUE" update-ref -d "$ref"; done
  git -C "$QUEUE" for-each-ref --format='%(refname)' refs/heads |
    while read -r ref; do [ "$ref" = refs/heads/main ] || git -C "$QUEUE" update-ref -d "$ref"; done
  rm -f "$QUEUE/.git/FETCH_HEAD" "$QUEUE/.git/ORIG_HEAD"
  rm -rf "$QUEUE/.git/logs"
  git -C "$QUEUE" reflog expire --expire=now --all >/dev/null 2>&1 || true
  git -C "$QUEUE" gc --prune=now --quiet
  git -C "$QUEUE" checkout --quiet main

  # ─── 3. the live store, moved on top of the replayed tree ─────────────────
  STAGE=move
  # daemon.pid/lock/sock/log name a path and a process that both stop being
  # true at the mv; they are gitignored and per-location, and carrying them
  # is how a restarted daemon reads a socket nobody is listening on.
  for f in "$SRC_BEADS"/* "$SRC_BEADS"/.[!.]*; do
    [ -e "$f" ] || continue
    case ${f##*/} in
      daemon.pid|daemon.lock|daemon.log|bd.sock|redirect) continue ;;
    esac
    mv -f "$f" "$DST_BEADS/${f##*/}"
  done
  # The deletion ledger names the COMMIT each recorded deletion accounts for
  # (rangerhq-6he5), and the replay gave every one of those commits a new
  # sha. Left alone, `.beads/deleted.jsonl` silences nothing: LostBeads
  # compares the record's commit against the census's, they no longer match,
  # and every deletion somebody already owned alarms again on the next pass.
  # Measured, not predicted — the rehearsal's census re-reported rangerhq-cdsu.
  if [ -f "$DST_BEADS/deleted.jsonl" ] && [ -f "$QUEUE/.git/queue-cutover-shamap" ]; then
    : > "$QUEUE/.git/queue-cutover.sed"
    while read -r old new; do
      grep -q "$old" "$DST_BEADS/deleted.jsonl" 2>/dev/null || continue
      printf 's/%s/%s/g\n' "$old" "$new" >> "$QUEUE/.git/queue-cutover.sed"
    done < "$QUEUE/.git/queue-cutover-shamap"
    if [ -s "$QUEUE/.git/queue-cutover.sed" ]; then
      sed -f "$QUEUE/.git/queue-cutover.sed" "$DST_BEADS/deleted.jsonl" > "$DST_BEADS/deleted.jsonl.new"
      mv -f "$DST_BEADS/deleted.jsonl.new" "$DST_BEADS/deleted.jsonl"
      say "rewrote $(wc -l < "$QUEUE/.git/queue-cutover.sed" | tr -d ' ') deletion-ledger commit sha(s) onto the replayed history"
    fi
    rm -f "$QUEUE/.git/queue-cutover.sed"
  fi
  rm -f "$QUEUE/.git/queue-cutover-shamap"

  # ─── 4. the constitution keeps a redirect and nothing else ────────────────
  # This lands BEFORE the queue's commit, and that order is the fix for
  # ranger-base-nzyn: from here on the fleet RESOLVES. Everything after can
  # fail without leaving bd dead — the store is whole, in one place, and the
  # constitution names it.
  STAGE=redirect
  rm -rf "$SRC_BEADS"
  mkdir -p "$SRC_BEADS"
  printf '%s\n' "$DST_BEADS" > "$SRC_BEADS/redirect"
  git -C "$CONSTITUTION" rm -r --cached --quiet .beads
  grep -qx '.beads/' "$CONSTITUTION/.gitignore" 2>/dev/null ||
    printf '%s\n' '.beads/' >> "$CONSTITUTION/.gitignore"
  git -C "$CONSTITUTION" add .gitignore
  say "$CONSTITUTION now holds .beads/redirect only — the untracking is STAGED, not committed"
fi

# ─── 5. every other redirect in the fleet ────────────────────────────────────
# Session worktrees are seeded from their main checkout's redirect at
# creation (internal/rhq/worktree.go), so the ones cut AFTER this are right
# by construction; the ones already open are not, and they are where the
# fleet is working right now.
STAGE=fanout
targets=$(printf '%s\n' "$REDIRECTS")
if [ -d "$WORKTREES" ]; then
  more=$(find "$WORKTREES" -maxdepth 3 -type d -name .beads 2>/dev/null | sed 's|/\.beads$||' || true)
  [ -n "$more" ] && targets="$targets
$more"
fi
printf '%s\n' "$targets" | while read -r repo; do
  [ -n "$repo" ] || continue
  [ -d "$repo/.beads" ] || continue
  # Never point the store at itself. A redirect inside the queue repo is a
  # one-hop cycle: bd resolves it to the directory it is already in and the
  # cutover looks fine until something follows the chain twice. Discovery
  # walks a directory the operator names, so it CAN reach the queue repo.
  [ "$repo/.beads" = "$DST_BEADS" ] && continue
  cur=$(head -n 1 "$repo/.beads/redirect" 2>/dev/null || true)
  [ "$cur" = "$DST_BEADS" ] && continue
  run "printf '%s\n' '$DST_BEADS' > '$repo/.beads/redirect'"
  say "redirect: $repo -> $DST_BEADS"
done

# ─── 6. the live store's drift, committed in the queue repo ──────────────────
# Last on purpose. Everything above has to happen inside the window; this does
# not, and a failure here leaves a fleet that reads and writes normally with
# one commit outstanding (ranger-base-nzyn).
if [ "$DRYRUN" = 0 ]; then
  STAGE=commit
  git -C "$QUEUE" add -A .beads
  if ! git -C "$QUEUE" diff --cached --quiet; then
    # Path-qualified. `git add -A .beads` stages nothing else, so the commit
    # is byte-identical to the unqualified form (measured: same tree sha,
    # deletions included) — but the unqualified form is REFUSED by a persona
    # cage denying `Bash(git commit unless --)`, and that is what aborted the
    # window on ranger-base-lpz4. A step this script needs must not be a step
    # only some sessions can run.
    git -C "$QUEUE" commit --quiet -m "beads: the store of record moves into its own repo (ADR 0015 §4)" -- .beads
    say "committed the live store's drift from the last constitution commit"
  fi
fi

STAGE=done

say ""
say "next, and NOT this script's (ADR 0015 §4, the operator's window):"
say "  1. config.yaml: beads_visibility: add '$QUEUE: private' BEFORE hooks are"
say "     installed there, and 'queue_repo: $QUEUE' so the launcher commits"
say "     the jsonl in it."
say "  2. posse gates install-hooks in $QUEUE — an unstamped repo is treated as"
say "     public and every launcher commit of the jsonl is refused. The launcher"
say "     also reconciles and probes that slot on every queue commit now"
say "     (ranger-base-mp0v): a missing one it writes, a stale stamp it"
say "     refreshes, a foreign one it refuses to commit through and says so."
say "  3. (cd $QUEUE && bd daemon start)  — never --auto-push, and leave the"
say "     repo with no remote."
say "  4. commit the constitution's staged untracking."
