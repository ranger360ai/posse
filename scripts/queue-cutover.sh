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
#   3. clears the replay's checkout out of the new repo's working tree —
#      keeping the index, so the drift below still reads as drift — and
#      moves the live store (database and friends) into it, so what is
#      there is the live store and nothing else (ranger-base-iycc);
#   4. leaves the constitution repo holding one file — `.beads/redirect` —
#      and stages the untracking, without committing;
#   5. rewrites `.beads/redirect` in every repo named with --redirect, in
#      every session worktree under ~/.posse/worktrees, and in every OTHER
#      tree under --scan that already redirected at the constitution (see
#      WHY THE FAN-OUT DISCOVERS, below);
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
# WHY THE FAN-OUT DISCOVERS rather than taking a list: a tree the fan-out
# misses does not keep working and it does not fail loudly — it keeps a
# redirect at the constitution, which now redirects onward, and bd 0.49.1
# refuses the second hop. Measured on ranger-base-l9aa, bd 0.49.1:
#
#   Warning: redirect chains not allowed, ignoring redirect in <middle>
#   Error: no beads database found
#   Hint: run 'bd init' to create a database in the current directory
#
# stderr and exit 1 — and the hint invites a second store in an archived
# checkout. Worse is the arm where the middle tree KEPT a database: the same
# warning goes to stderr and the command EXITS 0, reading and writing the
# SUPERSEDED store. So the list has to be discovered, because the cost of
# forgetting an entry is not an error message. The guard is that the redirect
# RESOLVES to $SRC_BEADS — every spelling of it bd follows, and nothing else,
# so a tree pointed at some other store is never touched (redirect_names,
# below, and ranger-base-4myz for what "every spelling" turned out to mean).
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
#                            [--scan DIR] [--scan-depth N] [--no-scan]
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
# Where to look for trees that already redirect at the constitution. The
# default is DERIVED from --constitution (its parent) below, after the
# arguments are read, rather than hard-coded to $HOME/src: every other path
# here is overridable, and a scan root that is NOT follows a --constitution
# override onto a fixture while still walking the live fleet — which would
# let a rehearsal, or a test, repoint the working crew's redirects at a
# throwaway queue. Empty means no scan at all.
SCAN_SET=0
[ "${SCAN+x}" = x ] && SCAN_SET=1
SCAN=${SCAN:-}
SCAN_DEPTH=${SCAN_DEPTH:-3}
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
    --scan)         SCAN=$2; SCAN_SET=1; shift 2 ;;
    --scan-depth)   SCAN_DEPTH=$2; shift 2 ;;
    --no-scan)      SCAN=''; SCAN_SET=1; shift ;;
    --dry-run)      DRYRUN=1; shift ;;
    --force-daemon) FORCE_DAEMON=1; shift ;;
    # Every comment line of the header, not a line range: the range this
    # used to carry (2,40) already cut the usage block's last line, and the
    # next edit to the header would have cut more.
    -h|--help)      awk 'NR>1 && /^#/{print} NR>1 && !/^#/{exit}' "$0"; exit 0 ;;
    *) echo "queue-cutover: unknown argument $1" >&2; exit 2 ;;
  esac
done

[ "$SCAN_SET" = 1 ] || SCAN=$(dirname "$CONSTITUTION")

say() { printf '%s\n' "$*"; }
run() { if [ "$DRYRUN" = 1 ]; then printf 'would: %s\n' "$*"; else eval "$@"; fi; }
die() { printf 'queue-cutover: %s\n' "$*" >&2; exit 1; }

SRC_BEADS=$CONSTITUTION/.beads
DST_BEADS=$QUEUE/.beads
RUNBOOK=docs/runbooks/queue-cutover.md

# ─── does this redirect name that directory? ─────────────────────────────────
# The fan-out's job is to find every tree bd can still RESOLVE through the
# constitution, so the question it has to ask is bd's question — "would bd
# follow this redirect to that directory?" — and not "are these two strings
# equal?". They are not the same question. MEASURED against bd 0.49.1
# (2026-08-30, ranger-base-4myz), every one of these is a live redirect bd
# follows to one store, and an exact compare recognises the first alone:
#
#   /s/.beads   /s/.beads/   /s/.beads␠   ␠/s/.beads   /s/.beads␉   ␉/s/.beads
#   /s/.beads<CR>   the same with a vertical tab or a form feed — bd's trim is
#   Go's strings.TrimSpace, so the `tr` below removes exactly what it removes
#   /s//.beads   /s/./.beads   /s/x/../.beads
#   ../s/.beads (relative — resolved against the tree holding the redirect,
#   not the caller's cwd; measured from three different cwds)
#   a symlinked spelling, and on a case-insensitive filesystem a different case
#
# A tree spelled any of those ways is left behind by the fan-out and becomes
# hop one of the two-hop chain the discovery exists to prevent (see WHY THE
# FAN-OUT DISCOVERS). That is not a hypothetical: hands are how this fleet
# actually gets redirects — the originating instance's own was repointed out
# of band, 41 bytes with no trailing newline where this script writes 42 —
# and a hand does not spell paths the way a script does.
#
# So ask the filesystem instead of the string: trim the blanks and the CR,
# resolve a relative path against the tree holding the redirect, and compare
# with `-ef`, which is device+inode identity — the same "same directory" bd
# means, and the one notion that gets the symlink and the case arms for free.
# This is looser about SPELLING and no looser about TARGETS: a tree pointed at
# some other store is a different inode and is still left alone, which
# TestQueueCutoverFindsTheTreesTheListForgets holds from the other side, and a
# redirect naming a path that is not a directory is one bd refuses to follow
# too (measured: "redirect target does not exist or is not a directory").
#
# `-ef` is not in POSIX. It is in every shell this can run under — measured on
# this box: /bin/sh, /bin/dash, /bin/bash and /bin/zsh all answer yes.
redirect_names() { # <a .beads dir> <a directory> — true when bd would follow it there
  _cur=$(head -n 1 "$1/redirect" 2>/dev/null | tr -d '\r\v\f' | sed 's/^[[:blank:]]*//; s/[[:blank:]]*$//')
  [ -n "$_cur" ] || return 1
  case $_cur in /*) ;; *) _cur=${1%/.beads}/$_cur ;; esac
  [ -d "$_cur" ] || return 1
  [ "$_cur" -ef "$2" ]
}

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
  redirects. A straggler does not keep working — it now names a
  '$SRC_BEADS' that redirects onward, and bd refuses the second hop. List
  them, then finish them by hand:
    find '$SCAN' -maxdepth $SCAN_DEPTH -type d -name .beads |
      while read -r b; do
        cur=\$(head -n 1 "\$b/redirect" 2>/dev/null | tr -d '\r\v\f' | sed 's/^[[:blank:]]*//; s/[[:blank:]]*\$//')
        case \$cur in ''|/*) ;; *) cur=\${b%/.beads}/\$cur ;; esac
        [ -n "\$cur" ] && [ -d "\$cur" ] && [ "\$cur" -ef '$SRC_BEADS' ] || continue
        echo "\$b"
      done
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

  # ...and then empty that working tree again, keeping the index. The
  # checkout above put the last COMMITTED projection into $DST_BEADS, and the
  # move loop below only overwrites the entries the live store happens to
  # share with it. An abort partway through that loop would leave $DST_BEADS
  # holding moved live files AND leftover replayed ones, and the trap's UNDO
  # — which walks everything in $DST_BEADS home — would then put the replayed
  # copies on top of the live files that had not moved yet: the live
  # projection replaced by the last commit, every uncommitted bead in it
  # gone, and the UNDO exiting 0 saying nothing (ranger-base-iycc, measured).
  # Emptying here makes the UNDO's assumption true instead of documenting it:
  # from this line on, $DST_BEADS holds nothing but what the loop moved, so
  # the same two-line UNDO is right in stage move, in stage redirect and in
  # the runbook's Rollback. It costs nothing — every byte removed IS HEAD and
  # `git checkout -- .beads` brings it back, while the live store it would
  # otherwise overwrite is recoverable from nowhere. The index still holds
  # the replayed state, so the queue's final `git add -A .beads` records the
  # live store's drift against the replayed history exactly as before.
  for f in "$DST_BEADS"/* "$DST_BEADS"/.[!.]*; do
    [ -e "$f" ] || continue
    rm -rf "$f"
  done

  # ─── 3. the live store, moved into the emptied tree ───────────────────────
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
# Any OTHER tree that already redirects at the constitution. These are the
# ones nobody remembers: a checkout retired months ago still resolves through
# the constitution, and after this script runs it holds hop one of a two-hop
# chain that bd refuses — silently, if that middle tree kept a database (see
# the header). The match is every spelling of $SRC_BEADS bd resolves and no
# other directory, so a tree pointed at a different store is left alone, and
# the constitution itself is skipped because step 4 already renamed its target
# to $DST_BEADS.
if [ -n "$SCAN" ] && [ -d "$SCAN" ]; then
  chained=$(find "$SCAN" -maxdepth "$SCAN_DEPTH" -type d -name .beads 2>/dev/null \
    | while read -r b; do
        redirect_names "$b" "$SRC_BEADS" || continue
        printf '%s\n' "${b%/.beads}"
      done) || true
  if [ -n "$chained" ]; then
    n=$(printf '%s\n' "$chained" | wc -l | tr -d ' ')
    say "scan: $SCAN (depth $SCAN_DEPTH) found $n tree(s) still pointed at the constitution"
    targets="$targets
$chained"
  fi
fi
targets=$(printf '%s\n' "$targets" | sed '/^$/d' | sort -u)
printf '%s\n' "$targets" | while read -r repo; do
  [ -n "$repo" ] || continue
  [ -d "$repo/.beads" ] || continue
  # Never point the store at itself. A redirect inside the queue repo is a
  # one-hop cycle: bd resolves it to the directory it is already in and the
  # cutover looks fine until something follows the chain twice. Discovery
  # walks a directory the operator names, so it CAN reach the queue repo —
  # and so does a `--redirect` the operator types, which is why this is an
  # `-ef` too and not a string compare: `--redirect $QUEUE/` spelled with one
  # trailing slash used to walk straight past this guard and write the cycle
  # (ranger-base-4myz).
  # Both forms, and the string one FIRST: under --dry-run the queue repo does
  # not exist yet, `-ef` cannot stat it, and a rehearsal must not start
  # printing a self-redirect it would never write.
  [ "$repo/.beads" = "$DST_BEADS" ] && continue
  [ -d "$DST_BEADS" ] && [ "$repo/.beads" -ef "$DST_BEADS" ] && continue
  cur=$(head -n 1 "$repo/.beads/redirect" 2>/dev/null || true)
  [ "$cur" = "$DST_BEADS" ] && continue
  redirect_names "$repo/.beads" "$DST_BEADS" && continue
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
say "  1. (cd $QUEUE && bd migrate --update-repo-id)  — NOT OPTIONAL. bd stamps"
say "     the database with an id derived from the repo it lives in, and the"
say "     queue repo is a different repo; until this runs, bd fails its daemon"
say "     closed and drops .beads/daemon-error warning that the mismatch 'may"
say "     treat your local issues as deleted'."
say "  2. config.yaml: beads_visibility: add '$QUEUE: private' BEFORE hooks are"
say "     installed there, and 'queue_repo: $QUEUE' so the launcher commits"
say "     the jsonl in it."
say "  3. posse gates install-hooks in $QUEUE — an unstamped repo is treated as"
say "     public and every launcher commit of the jsonl is refused. The launcher"
say "     also reconciles and probes that slot on every queue commit now"
say "     (ranger-base-mp0v): a missing one it writes, a stale stamp it"
say "     refreshes, a foreign one it refuses to commit through and says so."
say "  4. (cd $QUEUE && bd daemon start)  — never --auto-push, and leave the"
say "     repo with no remote."
say "  5. commit the constitution's staged untracking."
