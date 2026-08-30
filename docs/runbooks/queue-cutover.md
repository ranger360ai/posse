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
rehearsal could not cover them and why they are called out below — with the
caveat, measured 2026-08-29, that the first of those denies is spelled
against the singular word only and `bd daemons` walks past it
(ranger-base-llp1; the note under *What the rehearsal broke* has it).

---

## Before the window

1. `make install` a posse that carries `queue_repo:` support
   (`internal/rhq/queuejsonl.go`). It is inert until the key is written, so
   installing early costs nothing.
2. Read the two config edits in step 5 — **the visibility stamp is the one
   that bites**, and it has to be in config *before* hooks are installed.
   Inside the retirement window they are typed in the **constitution** and
   committed, not at the home; step 5 says why, and where in the order.

## The window

Run it top to bottom. Steps 1–4 are one script.

**1. Stop the daemon — and *check* that it stopped.** It holds the database
open and re-exports it on a timer; moving the directory under a live daemon
leaves a process writing to a path nobody reads.

```sh
pkill -f 'posse cockpit' || true        # see the third bullet, do this first
bd daemons list                         # workspace paths and pids
bd daemons stop ~/src/ranger-base       # a pid works here too
bd daemons list                         # MUST come back without that workspace
```

Three things about this step were wrong or unstated until the live window
(**MEASURED 2026-08-28**, ranger-base-j2io). Read them before typing:

- **Plural, and it takes an argument.** This step used to read `cd
  ~/src/ranger-base && bd daemon stop`. bd 0.49.1 ships *both* groups —
  `bd daemon` (singular: `start`/`stop`/`status`/`logs`/`restart`/`killall`,
  acting on the current workspace) and `bd daemons` (plural:
  `list`/`health`/`stop <workspace-path|pid>`/`logs`/`killall`, acting
  across repos). The form that did the job at the window was the plural with
  the workspace named. Because both spellings parse, getting this wrong is
  not a usage error you see — it is a no-op you have to notice.
- **A success line is not a stopped daemon.** The stop printed success while
  a daemon its *own* invocation had auto-started went on running. Any bd
  command auto-starts one; the stop is therefore racing itself. That is what
  the second `bd daemons list` is for, and when it does not come back clean,
  killing the pid the list names is the honest form. Independently measured
  a day earlier in a different setting — `docs/notes.d/ranger-base-42mv.md`
  found the same exit-0-and-still-running behaviour against a fixture store,
  and reached the same conclusion: the pid is the handle.
- **Sweep orphan cockpit processes first.** Three week-old `posse cockpit`
  processes were respawning daemons faster than they could be stopped, so
  the list never emptied and step 3's preflight refused in a loop that
  cannot be won by retrying. Kill those before the first stop.

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
- clears that checkout out of the new repo's working tree (the index is
  kept, so the live store still reads as drift against the replayed history)
  and moves the live store **into** it, so the queue's `.beads` holds the
  live store and nothing else — an abort partway through the move then
  leaves no replayed copy for the rollback below to walk home on top of a
  live file that never left (ranger-base-iycc);
- leaves `daemon.pid`, `daemon.lock`, `daemon.log` and `bd.sock` behind,
  because they name a path and a process that both stop being true at the
  move;
- rewrites `.beads/deleted.jsonl`'s commit shas onto the replayed history
  (see *What the rehearsal broke*, below);
- leaves `~/src/ranger-base/.beads/` holding one file, `redirect`, with the
  untracking **staged and not committed**;
- rewrites `.beads/redirect` in `~/src/posse`, in every session worktree
  under `~/.posse/worktrees`, and in every other tree beside the
  constitution whose redirect still names it — the scan root is the
  constitution's parent (`--scan DIR`, `--no-scan`, `--scan-depth N`), and a
  redirect is rewritten when it *resolves* to the constitution's `.beads`,
  which is every spelling of it bd follows and no other directory (a trailing
  slash, stray blanks, a CRLF ending, `//`, `/./`, `/../`, a relative path or
  a symlink — ranger-base-4myz; a tree pointed at some other store is left
  alone);
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

**5. Two config edits.** *Where you type them* depends on whether a promote
is still ahead of you.

```yaml
beads_visibility:
  ~/src/ranger-queue: private     # BEFORE step 6. Unmarked is treated as
                                  # public, and the queue db is ops-class
                                  # from end to end.
queue_repo: ~/src/ranger-queue    # the launcher commits the jsonl here
```

**Inside the retirement window, type them in the CONSTITUTION** —
`~/src/ranger-base/rhq/config.yaml` — and commit them, *before* the home
half's promote. **MEASURED 2026-08-28** (ranger-base-j2io): this beat this
step's old edit-the-home instruction, and it is now the written order.
`config.yaml` is a **promoted path**, so an edit made only at the home is
drift that the next `posse promote` reverts to whatever the constitution
says — and the line whose reversion is silent for hours is the visibility
stamp, for exactly the reason step 6 spells out. Edited in the constitution
and committed, the promote carries them and the promoted home is right the
first time; it also makes them ratified rather than hand-applied, which is
what ADR 0015 §3 asks of anything in force.

The slot is **after step 4 and before the promote**, and not earlier.
Do not fold these into the pre-window commit (`retirement-window.md` P2):
before the promote the home is still a symlink onto the constitution, so a
P2 edit is live the moment it is written, and `queue_repo:` would name
`~/src/ranger-queue` for however long the window is away — a repo step 3 has
not created yet. `retirement-window.md`'s driver table carries the slot.

**Standalone, with no promote in the sequence**, edit the home's
`config.yaml` directly — `~/.config/rhq/config.yaml` before the cutover,
`~/.config/posse/config.yaml` after.

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

This step is no longer the *only* thing standing between a launcher commit
and an unguarded store of record (ranger-base-mp0v). Every queue commit
reconciles the slot first and then probes it — ADR 0023 identity plus
behavior, the same pair a launch applies to the session dir — and refuses
the commit when it cannot vouch for what is there:

- **slot missing** (this step skipped): written, stamped from config, and
  the commit goes through it.
- **stamp stale** (step 5 landed after step 6, or config re-marked later):
  restamped on the next close, so the drift lasts one close rather than
  forever.
- **slot foreign** (bd's own shim, a hand-rolled hook, a neutered one):
  install will not overwrite it and the probe cannot vouch for it, so the
  jsonl does **not** commit and the pass says so — `⚠ <id> the queue jsonl
  did NOT commit in <repo>: its beads visibility stamp is not armed …`. Run
  this step; `install-hooks` chains bd's shim rather than refusing it.

Run it anyway at the window: it is the earliest moment the wall can be up,
and it is the form that chains. What changed is that forgetting it is a
refusal you can see, not a stream of unguarded commits.

**7. Restart the daemon, in the new repo.**

```sh
cd ~/src/ranger-queue && bd daemon start
bd daemons list        # the queue workspace, and nothing naming the old path
```

Same instrument as step 1, for the same reason: the start is reported, the
daemon is observed.

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

**And a sixth the rehearsal could not have caught** (ranger-base-l9aa, found
2026-08-28, hours after the real move and fixed on 08-29): the fan-out took a *list*, and a
list can be short. A tree that redirected at the constitution but was on
nobody's list — here the archived pre-POSSE checkout, `~/src/rangerhq` —
kept its redirect, which after the move is hop one of a two-hop chain. bd
0.49.1 refuses the second hop, so that checkout resolved **no database at
all**: `Warning: redirect chains not allowed…` then `Error: no beads
database found`, plus a hint inviting `bd init` to mint a second store
there. The worse arm, measured on a throwaway rig the same day: when the
middle tree still holds a `beads.db`, the warning goes to stderr and the
command **exits 0 against the superseded store** — a chain that reads and
writes the wrong database with nothing on stdout to say so. The fan-out now
discovers those trees instead of being told about them
(`TestQueueCutoverFindsTheTreesTheListForgets`).

Not rehearsable from a persona session, and therefore untested until the
window: `bd daemon stop`/`start` and `bd migrate --update-repo-id` are both
denied to personas. Steps 1, 4 and 7 are the operator's, and step 4 is the
one to watch.

That deny is spelled against the **singular** word, and step 1 above now
types the plural. `bd daemons list` is not covered by it and runs from a
persona session today — measured 2026-08-29, filed as ranger-base-llp1. Do
not read step 1's new spelling as a fence you can lean on: it is the form
that works, not a form personas are stopped from typing.

## Rollback

The window's rollback is cheap because nothing is destroyed: the
constitution repo's `.beads` deletion is **staged, not committed**, and the
live store was moved rather than copied.

```sh
bd daemons stop ~/src/ranger-queue && bd daemons list   # verify, per step 1
# The store goes home — DOTFILES INCLUDED. `.beads/*` alone leaves
# `.beads/.gitignore` (tracked, and the only thing ignoring the database)
# and `.local_version` behind, so the constitution comes back with a 10MB
# `beads.db` untracked AND unignored, one `git add -A` from being committed
# (ranger-base-g1js). Same loop the script's own undos print.
for f in ~/src/ranger-queue/.beads/* ~/src/ranger-queue/.beads/.[!.]*; do
  [ -e "$f" ] && mv -f "$f" ~/src/ranger-base/.beads/
done
rm -f ~/src/ranger-base/.beads/redirect   # -f: an abort in stage `move` never wrote one
cd ~/src/ranger-base && git reset -q HEAD -- .beads .gitignore &&
  git checkout -- .gitignore .beads/.gitignore   # the root ignore AND the one that hides beads.db
# Put every redirect back — INCLUDING the ones the fan-out's scan found,
# which are not on any list here either. Anything still naming the queue's
# `.beads` after the store has gone home is a dangling redirect, and the
# same two-hop trap in reverse (ranger-base-l9aa).
printf '%s\n' ~/src/ranger-base/.beads > ~/src/posse/.beads/redirect
for w in ~/.posse/worktrees/*/*; do
  [ -d "$w/.beads" ] && printf '%s\n' ~/src/ranger-base/.beads > "$w/.beads/redirect"
done
find ~/src -maxdepth 3 -type d -name .beads | while read -r b; do
  # `x && y` as the last statement makes the loop exit 1 on a non-match, and
  # under `set -e` that kills the rest of a rollback. Measured; use `continue`.
  # Resolve rather than compare bytes: bd follows a trailing slash, stray
  # blanks, a CRLF ending, `//`, `/./`, `/../`, a relative path and a symlink
  # to the same store, and a redirect written by a hand rather than by the
  # script is spelled a hand's way (ranger-base-4myz). `-ef` is device+inode
  # — same directory, whatever it is called.
  cur=$(head -n 1 "$b/redirect" 2>/dev/null | tr -d '\r' | sed 's/^[[:blank:]]*//; s/[[:blank:]]*$//')
  case $cur in ''|/*) ;; *) cur=${b%/.beads}/$cur ;; esac
  [ -n "$cur" ] && [ -d "$cur" ] && [ "$cur" -ef ~/src/ranger-queue/.beads ] || continue
  printf '%s\n' ~/src/ranger-base/.beads > "$b/redirect"
done
cd ~/src/ranger-base && bd migrate --update-repo-id && bd daemon start
```

Then check the repo came back the way it went in — this is the step that
catches a half-rollback while it is still cheap:

```sh
git -C ~/src/ranger-base status --porcelain -- .beads .gitignore
```

Only ` M .beads/issues.jsonl` and ` M .beads/deleted.jsonl` (the window's
drift, which is real work and stays). A `??` line, or ` D .beads/.gitignore`,
means the store did not come home whole — do not commit anything until it
does.

Then unset `queue_repo:` in config. `rm -rf ~/src/ranger-queue` last, once
`bd ready` answers from the constitution repo again.
