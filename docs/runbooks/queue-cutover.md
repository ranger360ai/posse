# Runbook — move the beads queue into its own repo

*ADR 0015 §4 (ratified 2026-08-26, ranger-base-ap2x) · built and rehearsed
on ranger-base-tjfw · executes inside the rhq retirement window,
ranger-base-3rv9 step 4*

The store of record — `~/src/ranger-base/.beads` — moves to
`~/src/ranger-queue`. Every repo the crew touches reaches it through the
`.beads/redirect` mechanism it already uses, the constitution repo
included. Issue ids do not change: the prefix stays `ranger-base` and
nothing is renamed.

**What this buys** (ADR 0015 §4): after it, no session holds a write grant
into the constitution repo unless it was dispatched into it — the seatbelt's
redirect grant follows the redirect, so the grant moves with the store — and
the constitution's git log stops being queue flushes.

**Who runs it.** The operator. Three of the steps are denied to personas by
construction (`bd daemon`, `bd migrate`, `make install`), which is why the
rehearsal could not cover them and why they are called out below.

---

## Before the window

1. `make install` a posse that carries `queue_repo:` support
   (`internal/rhq/queuejsonl.go`). It is inert until the key is written, so
   installing early costs nothing.
2. Read the two config edits in step 5 — **the visibility stamp is the one
   that bites**, and it has to be in config *before* hooks are installed.

## The window

Run it top to bottom. Steps 1–4 are one script.

**1. Stop the daemon.** It holds the database open and re-exports it on a
timer; moving the directory under a live daemon leaves a process writing to
a path nobody reads.

```sh
cd ~/src/ranger-base && bd daemon stop
```

**2. Quiesce the fleet.** No dispatch pass and no persona session should be
mid-`bd close`. `posse dispatch --watch-status` says whether a loop is
armed; disarm it first.

**3. Run the cutover.**

```sh
~/src/posse/scripts/queue-cutover.sh          # add --dry-run first if you like
```

Defaults are the live paths, so there are no arguments. It refuses if a
daemon is up, if `~/src/ranger-queue` already exists, or if the store
already redirects. What it does:

- replays the `.beads/` history out of the constitution repo into the new
  one — same paths, same authors, same dates, **new shas** — and nothing
  else from that repo's history (152M of constitution objects became 5.6M
  of queue objects in the rehearsal);
- moves the live store on top of it, leaving `daemon.pid`, `daemon.lock`,
  `daemon.log` and `bd.sock` behind, because they name a path and a process
  that both stop being true at the move;
- rewrites `.beads/deleted.jsonl`'s commit shas onto the replayed history
  (see *What the rehearsal broke*, below);
- leaves `~/src/ranger-base/.beads/` holding one file, `redirect`, with the
  untracking **staged and not committed**;
- rewrites `.beads/redirect` in `~/src/posse` and in every session worktree
  under `~/.posse/worktrees`;
- commits the live store's drift in the queue repo — **last**, after the
  redirects, because it is the only step whose failure costs nothing but a
  commit.

**If it aborts, read what it printed.** Every step past the preflight names
the half-state it left and the commands that undo it (`ABORTED … in stage
"<x>"`, then `UNDO:` or `FINISH:`). The order above is what makes those undos
small: once the constitution's redirect is written, the store is whole in one
place and the fleet resolves, so nothing after that stage needs the rollback
below. Only an abort in stage `move` leaves the store split — that one is the
emergency, and its undo is printed with both paths filled in
(ranger-base-nzyn, hit for real on the rehearsal).

**4. Update the database's repo id. NOT OPTIONAL.**

```sh
cd ~/src/ranger-queue && bd migrate --update-repo-id
```

bd stamps the database with an id derived from the repo it lives in, and the
queue repo is a different repo. Its own words for the mismatch:

> ⚠️ CRITICAL: This mismatch can cause beads to incorrectly delete issues
> during sync! The git-history-backfill mechanism may treat your local
> issues as deleted because they don't exist in the remote repository's
> history.

That is the rangerhq-fuom silent-deletion mechanism, armed across the whole
queue. bd fails its daemon closed while the mismatch stands and drops
`.beads/daemon-error` saying so, which is the check: after the migrate, that
file should be gone and `bd info` should report a daemon.

**5. Two config edits**, in `~/.config/rhq/config.yaml` (`~/.config/posse`
after the promote):

```yaml
beads_visibility:
  ~/src/ranger-queue: private     # BEFORE step 6. Unmarked is treated as
                                  # public, and the queue db is ops-class
                                  # from end to end.
queue_repo: ~/src/ranger-queue    # the launcher commits the jsonl here
```

**6. Install the gate hooks in the queue repo.**

```sh
posse gates install-hooks ~/src/ranger-queue
```

The visibility verdict is stamped into the hook at install time, and an
unstamped repo is stamped `public`. Measured on the rehearsal, and the
reason this step has an order: a public stamp does **not** refuse every
commit — it refuses the ones whose added JSONL lines match an ops-class
pattern. So an unstamped queue repo commits happily for hours and then
starts refusing on the first bead that mentions a dollar figure, which reads
as a new bug rather than a missing config line. The guard also does not key
on `RHQ_PERSONA`: the launcher's commit is subject to it exactly as a
persona's is. Step 5 before step 6, always.

**7. Restart the daemon, in the new repo.**

```sh
cd ~/src/ranger-queue && bd daemon start
```

Never `--auto-push`. Never add a git remote: the queue repo is created
without one, and a repo with no remote cannot push whatever any future bd
flag decides to do. (`bd daemon start --auto-commit` exists and would commit
the jsonl on a 5s timer with no posse code at all; it was measured and not
used — it commits with no bead to name and its git failures, gate refusals
included, land in `daemon.log` where nobody reads them.)

**8. Commit the constitution's staged untracking.**

```sh
cd ~/src/ranger-base && git commit -m 'beads: the queue moves to its own repo (ADR 0015 §4)'
```

**9. Verify** — this is ADR 0015's verification item 6:

```sh
for d in ~/src/posse ~/src/ranger-base ~/.posse/worktrees/*/*; do
  ( cd "$d" && bd where | head -2 )
done
bd ready >/dev/null && echo "ready ok"
posse beads check                  # the census must read the replayed history
posse gates <persona>              # the constitution repo's .git in no writable set
```

Then close one real bead and confirm the pass printed
`⎘ <id> issues.jsonl committed in ~/src/ranger-queue (<sha>)` and that
`git -C ~/src/ranger-queue log --oneline -1` shows it — and that
`git -C ~/src/ranger-queue remote -v` is still empty.

---

## What the rehearsal broke

Rehearsed 2026-08-26 on a full copy of the live store (ranger-base-tjfw),
and the rehearsal itself verified on ranger-base-lpz4. Five things were wrong
or surprising, and the four the script owns are fixed in it — they are
recorded because each would have been silent in production:

1. **A fresh queue repo disarms the bead-loss census.** `LostBeads`
   (`internal/rhq/beadloss.go`) *is* the git log of `.beads/issues.jsonl` in
   whatever repo the redirect lands in. A queue repo starting at one commit
   has no census, so the alarm that exists because bd deletes rows silently
   reports nothing, forever, with no error anywhere. Hence the history
   replay — it is not sentiment about history, it is the alarm's input.
2. **The replay renames every commit, and the deletion ledger is keyed by
   commit.** `.beads/deleted.jsonl` records the sha of the removal each
   entry accounts for (rangerhq-6he5); after a replay those shas name
   nothing, the ledger silences nothing, and every deletion somebody already
   owned alarms again. Measured: the census re-reported `rangerhq-cdsu`. The
   script now rewrites those shas onto the replayed history, and the census
   went back to matching the live one exactly.
3. **A public stamp fails intermittently, not at the cutover.** With the
   queue repo unstamped, the launcher's commit went through for an ordinary
   bead and was refused for one carrying a dollar figure — the same commit
   path, minutes apart. Step 5's config edit is what prevents it; the
   measurement is why it is not filed under "nice to have".
4. **Redirect discovery could point the store at itself.** Walking a
   directory that contains the queue repo wrote a `redirect` inside it — a
   one-hop cycle that looks fine until something follows the chain twice.
   Guarded.
5. **An abort inside the window said nothing** (ranger-base-nzyn, hit while
   verifying the rehearsal). The queue's commit ran BEFORE the redirect and
   was unqualified — `git commit -m <msg>` — which a persona cage denying
   `Bash(git commit unless --)` refuses; `set -eu` then exited silently,
   leaving `~/src/ranger-base/.beads` empty, the store in the queue repo and
   every redirect in the fleet naming the empty directory. Fixed three ways:
   the commit is path-qualified (same tree, measured), it runs last, and
   every step past the preflight prints its half-state and its undo.

Not rehearsable from a persona session, and therefore untested until the
window: `bd daemon stop/start` and `bd migrate --update-repo-id` are both
denied to personas. Steps 1, 4 and 7 are the operator's, and step 4 is the
one to watch.

## Rollback

The window's rollback is cheap because nothing is destroyed: the
constitution repo's `.beads` deletion is **staged, not committed**, and the
live store was moved rather than copied.

```sh
cd ~/src/ranger-queue && bd daemon stop
mv ~/src/ranger-queue/.beads/* ~/src/ranger-base/.beads/     # the store goes home
rm  ~/src/ranger-base/.beads/redirect
cd ~/src/ranger-base && git reset -q HEAD -- .beads .gitignore && git checkout -- .gitignore
# put every redirect back
printf '%s\n' ~/src/ranger-base/.beads > ~/src/posse/.beads/redirect
for w in ~/.posse/worktrees/*/*; do
  [ -d "$w/.beads" ] && printf '%s\n' ~/src/ranger-base/.beads > "$w/.beads/redirect"
done
cd ~/src/ranger-base && bd migrate --update-repo-id && bd daemon start
```

Then unset `queue_repo:` in config. `rm -rf ~/src/ranger-queue` last, once
`bd ready` answers from the constitution repo again.
