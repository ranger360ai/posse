## Two true sentences that contradicted each other in one log (ranger-base-n73bx)

github issue 7, from the work-box shakedown (ranger-base-x4g3h, 2026-10-08).
A seat closes with one commit on its branch. The pass that judged the close
fast-forwarded it and said so:

```
⤴ a-1   1 commit(s) fast-forwarded from posse/ranger-a-1 onto main in ~/src/posse
```

The NEXT settle of the same bead — a relaunch, a re-prompt, a second judged
close — printed:

```
◑ a-1   closed with no commit on posse/ranger-a-1 — nothing to merge onto main
```

Both lines were true when they were written, and the second is the one an
operator reads to know whether work landed. It reads as a failure, and it
contradicts the line above it about the same bead.

MEASURED 2026-10-09, macOS 26.4.1 / darwin 25.4.0, git 2.50.1 (Apple Git-155).

### Why ancestry cannot tell the two apart

`mergeBack`'s arm was `o.Merged && o.Commits == 0` — the base holds the
session's work and the branch has nothing above it — and that one state has
two meanings:

- the branch never carried a commit, so there was never anything to land
  (ADR 0041's incident shape, and the eight-of-twelve ordinary case its §5
  measured: a design, question or verify close);
- the branch carried a commit and an earlier pass already landed it.

A branch that never committed sits at the commit it was cut from. A branch
whose work was fast-forwarded sits at the commit the base was MOVED to. Both
are reachable from the base, both count zero ahead of it, and no `rev-list`
or `merge-base` separates them without the cut point — which posse records
nowhere. So the arm was not choosing badly between two readings; it had only
one reading available and asserted it.

### The reflog is the record, and the shop already read it for this

The branch's own reflog holds the cut point, and ADR 0041's incident report
is written from exactly this instrument: *"the branch's reflog is a single
'Created from main'"*.

`branchEverMoved` (worktree.go) counts entries rather than parsing them: git
writes one reflog entry per ref UPDATE, so one entry is a branch that has
only ever been created and two is a branch that has been moved once —
whatever moved it (a commit in the worktree, the detached splice's
`branch -f`, a replay).

MEASURED in a scratch repo, `worktree add -b` then one commit then
`merge --ff-only`:

| after | `reflog show refs/heads/posse/s` | `rev-list --count main..posse/s` |
|---|---|---|
| `worktree add -b` | `branch: Created from main` | 0 |
| one commit in the tree | `commit: work` / `branch: Created from main` | 1 |
| `merge --ff-only` | unchanged, both entries | **0** |

The last row is the settle this bead is about: zero ahead, and the reflog
still says a commit happened.

The message is read on the single-entry case alone, and only to tell a
creation from an update that EXPIRED down to one — git keeps the newest
entries, so a lone `commit:` is a branch whose creation entry is gone, not a
branch that was never moved. Every spelling that creates a branch
(`branch`, `checkout -b`, `switch -c`, `worktree add -b`) writes
`branch: Created from <start>` (MEASURED, same environment).

### Unreadable is its own answer

The three ways the reflog says nothing, all MEASURED:

| case | `git reflog show refs/heads/<b>` |
|---|---|
| branch is gone | `fatal: ambiguous argument` — exit 128 |
| no reflog file | exit 0, no output |
| `reflog expire --expire=all` | exit 0, no output |

All three come back `known=false` and print as the unknown they are. Folding
them into "closed with no commit" would be the original defect with a
narrower population, and folding them into "already landed" would claim a
landing nothing measured.

### What prints now

`PriorLanding` (worktree.go) is the three answers; `landed()` sets it on the
two outcomes that merge nothing, and the wording stays at the one surface
that has this arm (`mergeBack`, dispatch.go):

```
≡ a-1   already landed: main holds posse/ranger-a-1 at 314f0d5579ae — nothing left to merge
◑ a-1   closed with no commit on posse/ranger-a-1 — nothing to merge onto main
◑ a-1   nothing to merge onto main from posse/ranger-a-1 — git's reflog for
        posse/ranger-a-1 does not say whether a commit landed on an earlier
        pass or none was ever made
```

The middle line is byte-for-byte what the pass printed before, because
ADR 0041 and closeddirty.go both quote it and that document's incident is
the case it is now MEASURED for rather than guessed at.

The other two surfaces that read a `MergeOutcome` are unchanged, deliberately.
`landClosedTrees` (landsweep.go) never reaches this arm — `nothingToLand`
answers true for an already-landed tree and the landing half is skipped before
any line is printed. `LandSessionTrees` (`posse worktrees --land`) prints
`· <branch> had nothing to land`, which is true under both readings and is a
command a human is watching.

### Pins

- `TestASecondSettleSaysTheWorkAlreadyLanded` — lands one commit, settles the
  same bead again, and asserts both halves: the ≡ line appears AND
  `closed with no commit` does not. The negative is the one that was failing.
- `TestASettleWithNoReflogClaimsNeither` — `reflog expire --expire=all`, then
  the second settle must claim neither reading.
- `TestClosedBeadWithNoCommitSaysSo` is the control, and it was already
  there: a close with nothing committed must still get ADR 0041's sentence
  verbatim.

MUTATION-CHECKED both ways. `moved, known := false, true` (always
"never committed") reds the two new pins; `true, true` (always "landed") reds
the control — `≡ a-1 already landed: main holds posse/ranger-003-a-1 at
314f0d5579ae` over a fixture that committed nothing.
