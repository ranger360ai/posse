# ADR 0059 — A session worktree may create AND unlink the shared repo's `packed-refs.lock`; `packed-refs` and `packed-refs.new` stay denied

*Status: accepted 2026-09-11 · owner: architect · source bead
ranger-base-agzhj, from ranger-base-71g2f (the zero-exit sequencer leak) ·
amends the "DECLARED, not granted" block above `sessionGitGrants` in
seatbelt.go (ranger-base-msex) and leaves its create-only rejection standing ·
neighbour of ADR 0038 (the cage's write-deny on the repo's persistent git
identity); no L4 twin, by ADR 0038 decision 4's own rule · builds in the
code beads cut from agzhj*

## Context

In a dispatched worktree every git operation that ends by DELETING a
pseudo-ref — `cherry-pick`, `revert`, `rebase`, on `--abort`, `--quit`,
`--continue`, `--skip`, and on a clean run that never conflicted — asks the
SHARED repo's `.git/packed-refs.lock`. The L2 grant (ranger-base-m2wf)
denies it, git prints the refusal, gives up the delete, and exits 0.
CHERRY_PICK_HEAD survives, and the session's next path-limited commit —
the only form its PID allows — dies at `fatal: cannot do a partial commit
during a cherry-pick`. ranger-base-71g2f made that loud (the git shim's
sequencer-audit arm refuses the zero exit and prints an `rm` recipe). Loud
is the floor: every merge-back seat still ends every sequencer verb by hand.

What was already decided and is NOT reopened: a **create-only** grant on the
lock was measured and rejected (ranger-base-msex; pinned by
`TestQAPackedRefsLockCreateGrantIsUnsafe`). Create-only buys the create and
not git's own unlink, so EVERY commit strands the lock file in the operator's
git dir, and a stray `packed-refs.lock` kills the operator's unsandboxed
`git gc` / `git pack-refs` at rc 128 until a human removes it.

What was never measured is the other shape: the lock as a WRITABLE subpath —
create and unlink — with `packed-refs` itself still denied. This record is
that measurement (2026-09-11, darwin 25.4.0, git 2.50.1 Apple Git-155,
sandbox-exec, the fleet's fixture shape: refs packed, branch `posse/x` two
directories deep, a sibling worktree; script beside this file), and the
decision it supports.

## Decision

**D1 — grant it.** `sessionGitGrants` gains one entry:
`<common>/packed-refs.lock`, rendered like the ref pair — a `(subpath …)`
naming a file — inside the ordinary allow block. Nothing else in the common
dir changes: `packed-refs`, `packed-refs.new`, `config`, `hooks/`, every
other ref and every other session's `worktrees/<name>` stay under the
default deny. The create-only spelling stays rejected and its pin stays.

**D2 — what the grant must keep true, each pinned by execution under the
rendered profile** (the numbers are in the MEASURED table):

1. every sequencer end — clean cherry-pick, conflicted cherry-pick
   `--abort`/`--quit`/`--continue`, conflicted revert `--abort`, conflicted
   rebase `--abort` — exits as git says, leaves no blocker pseudo-ref, no
   lock file, and prints no `packed-refs.lock` line; the path-limited commit
   afterwards lands;
2. an ordinary path-limited commit, before and after an operator's
   `pack-refs`, leaves no lock behind (the msex control, unchanged);
3. a rewrite git WANTS to make (delete a ref that is packed) fails at
   `packed-refs.new`, git rolls back and removes its own lock, and
   `packed-refs` is byte-identical afterwards;
4. the lock name cannot be turned into a write on anything else: hard-link
   of `packed-refs` onto it is refused; a symlink can be planted there but a
   write THROUGH it is refused (the profile resolves paths); rename of the
   lock onto `packed-refs` or onto `refs/heads/main` is refused; a write to
   `packed-refs.new` is refused.

**D3 — a stray `packed-refs.lock` is the operator's to remove, never a
session's, and no gate prescribes its removal.** Git's lock files carry no
holder identity and no expiry (fencing-and-leases: a lease without expiry;
liveness-and-identity: a name is not a holder). A session cannot tell a
stranded lock from a live operator's `gc` holding it, so the 71g2f recipe
keeps naming the pseudo-refs in the session's OWN git dir and nothing in the
common dir. The grant lets git clean up after ITSELF; it does not make the
session a cleaner of shared state.

**D4 — no L4 twin.** At L4 (cage.go) the common dir is `:ro` with three
overlays and no `.lock` sibling is ever bound, because a file bind whose
source is absent makes the source a DIRECTORY on the host (ADR 0038 decision
4, measured) — and `packed-refs.lock` is absent whenever nothing holds it.
A `packed-refs.lock` directory in the operator's git dir is the msex
landmine made permanent. The sequencer verbs at L4 keep the 71g2f arm as
their floor; the L4 engine is off-box and UNRUN for this shape.

## Alternatives rejected

- **Do nothing — keep the shim and the hand recipe** (the do-nothing
  option, priced first). Cost, MEASURED in shape: every cherry-pick, revert
  and rebase in every dispatched worktree — clean ones included — ends with
  a refused exit and an `rm -rf` the seat pastes; merge-back is a recurring
  dispatch type (a6gcp, and qz3cr counted 50 merge-back-blocked beads on
  2026-09-06). Cost of the grant it avoids: one file name, and ASSUMED
  rare strands (below). The recipe is a runbook wearing a cage decision.
- **Create-only on the lock** (msex). Stays rejected: strands on every
  commit, MEASURED then and its pin still green. The failure was the
  missing unlink, not the create; D1 grants the unlink.
- **The shim repairs instead of refusing** (71g2f considered it). A
  cooperative gate finishing git's cleanup hides the one signal that
  brought this decision forward, and with D1 there is nothing to repair on
  the common path. It also stays wrong at D3: a shim that removes shared
  state on a guess is the destructive act the wall exists to prevent.
- **Grant `packed-refs` (or the common dir) writable.** That is what m2wf
  narrowed away: a writable `packed-refs` moves any ref, `refs/heads/main`
  included, without `git push` ever being spelled (L1 and L3 both blind).
  MEASURED then; nothing here reopens it.
- **A per-session ref store** (the clever one: `extensions.refStorage=
  reftable`, or a private common dir spliced back at close). Reftable moves
  the same lock to `reftable/tables.list.lock` in the same shared dir, and
  both are repo-format or launcher-topology changes to the operator's
  checkout — expensive to reverse, unmeasured, and buying nothing D1 does
  not.
- **A git knob to skip the packed lock on delete.** None exists, and one
  cannot: MEASURED (D2.3), deleting only the loose copy of a packed ref
  resurrects the packed value — the lock is how git avoids that.

## Consequences

- One grant line, three test sites, one comment block: `sessionGitGrants`
  (+1 entry, and the "DECLARED, not granted" block rewritten to this
  record); `TestQAWorktreeGrantNamesObjectsLogsAndItsOwnRefOnly` (the set
  gains the lock); `TestQAPackedRefsLockCreateGrantIsUnsafe` (its candidate
  must be built on a writable set with the lock REMOVED, or the subpath
  grant outvotes the create-only line it exists to test);
  `TestQAWorktreeCommitLeavesNoStrayPackedRefsLock` (stays; gains D2's four
  arms). `posse gates` prints it as a `w` line — a reader sees exactly one
  new name.
- The 71g2f arm keeps firing where it still applies: a stray lock from
  anyone (the stray arm — git retries the lock for `core.packedRefsTimeout`,
  default 1000 ms, once per transaction, three transactions, 3.4–3.8 s wall,
  then exits 0 leaving the blocker, MEASURED), and L4.
  Its two texts that say "the cage denies it" and "seatbelt.go declines to
  grant" become false on main the day D1 lands and are reworded in the
  dependent bead; the recipe gains no `rm` of anything under the common dir
  (D3).
- The failure mode D1 adds is not a new class: a SIGKILL landing inside the
  window a git process holds the lock strands it. MEASURED: SIGTERM and
  SIGHUP to the holder remove the lock (git's own signal cleanup); SIGKILL
  strands it. The window for a whole `cherry-pick --abort` is 64–242 ms
  (ten hook-instrumented runs, two batches, the spread is box load). The
  SAME kill inside the SAME window under the SHIPPED grant
  already strands `refs/heads/<branch>.lock` and the per-worktree
  `index.lock`, and the operator's `gc` already dies at rc 128 on the
  stranded ref lock (the reflock control arm). One more file joins a class the fleet already
  carries and clears by hand; the recovery line is the same.
- The hand D1 gives a session (question 4): it can `touch` the lock and
  sit on it, and the operator's `pack-refs --all` and `gc` then die at
  rc 128 until it is removed. Under the shipped grant the same session
  planting its own `refs/heads/<branch>.lock` already kills `gc` at rc 128
  (`fatal: failed to run reflog`) and makes `pack-refs --all` skip the ref
  with an error at rc 0. So the new hand is the `pack-refs` exit code,
  not the ability to block the operator; a session that wants to hurt the
  shared repo has had a file to plant since m2wf. A symlink planted at the
  name blocks the same way (O_EXCL fails on it) and corrupts nothing.
- A session can also `rm` the lock. D3 says it must not; the PID cannot
  spell "only the lock you created", so this stays cooperative and is said
  so here rather than dressed as a wall. What it cannot do is reach
  `packed-refs`: on git 2.50.1 the lock is a pure lock and the content
  goes through `packed-refs.new` (MEASURED, D2.3), which is why removing a live lock costs
  at most a lost update between two UNSANDBOXED writers, not a corrupt
  file. ASSUMED that two operator-side writers racing on this repo is a
  case nobody runs.
- Exit hatch: delete the one entry; every pin above flips back to the
  shipped shape, and the 71g2f arm is already the floor.

## Verification (laurie's checklist)

1. Under the RENDERED profile for a dispatched tree, each of D2's four items
   runs as a QA test with the shipped profile as its control where the
   shipped profile can run the probe (items 1–3); item 4's refusals need no
   control.
2. `TestQAPackedRefsLockCreateGrantIsUnsafe` still strands under create-only
   — with the lock removed from the writable set it is built on — and still
   kills the operator's `pack-refs` on the fixture.
3. `posse gates <persona>` for a dispatched tree prints exactly one new `w`
   line, `<common>/packed-refs.lock`, and no `+` or `~` line for it.
4. In a live dispatched worktree after landing: `git cherry-pick <clean sha>`
   prints nothing to stderr, exits 0, `git rev-parse --git-dir` holds no
   CHERRY_PICK_HEAD, and `git commit -m x -- <path>` lands.
5. The 71g2f recipe text, on main after both beads land, names no path
   under `git rev-parse --git-common-dir`.

## MEASURED vs ASSUMED

MEASURED (2026-09-11, darwin 25.4.0, git 2.50.1, `0059-packed-refs-lock.probe.sh`
beside this record; every arm on a fresh fixture, shipped profile as control):

| arm | shipped grant | write+unlink grant |
|---|---|---|
| clean cherry-pick | rc 0, CHERRY_PICK_HEAD survives, 1 stderr line | rc 0, clean, silent |
| conflicted cherry-pick `--abort` / `--quit` / `--continue` | rc 0, blocker survives, 2–3 lines | rc 0, clean, silent |
| path-limited commit after that abort | rc 128 "partial commit during a cherry-pick" | rc 0 |
| conflicted revert `--abort` | rc 0, REVERT_HEAD survives | rc 0, clean |
| conflicted rebase `--abort` | rc 0, CHERRY_PICK_HEAD survives | rc 0, clean |
| plain commits, one after operator `pack-refs` | rc 0, stderr line, no stray | rc 0, silent, no stray |
| delete a PACKED ref (`update-ref -d`) | lock refused, loose copy deleted, packed survives | refused at `packed-refs.new`, lock removed, packed-refs unchanged |
| plant the lock, then operator `pack-refs` / `gc` | cannot plant | both rc 128 "File exists" until removed; session can `rm` it |
| plant own `refs/heads/<b>.lock` (both grants) | `pack-refs` rc 0 + error, `gc` rc 128 | same |
| hardlink `packed-refs` → lock / write through symlink / rename onto `packed-refs`, `refs/heads/main` / write `packed-refs.new` | all refused | all refused |
| SIGTERM, SIGHUP to the lock holder | n/a | lock removed |
| SIGKILL to the lock holder | n/a | lock stranded |
| SIGKILL to a plain commit holding its ref lock (shipped) | ref lock + index.lock stranded, `gc` rc 128 | same class |
| lock held per `cherry-pick --abort` | n/a | 64–242 ms, 3 transactions |
| stray lock from anyone, then `--abort` | rc 0 after 3 × 1 s retries, blocker survives | same; the 71g2f arm fires in both |

ASSUMED: the rate of SIGKILLs landing inside a ~70 ms window (the launcher's
kill path is SIGTERM then SIGKILL to the runtime's pid — loadguardkill.go —
and what reaches a git grandchild is unmeasured; measured only that killing
the `git cherry-pick` FRONTEND leaves the worker to finish and remove the
lock); that no operator-side pair of git writers races on this repo; that
git 2.55 (the runners) keeps the tempfile shape for `packed-refs.new` (the
msex test notes 2.55 only moved gc's exit code).
