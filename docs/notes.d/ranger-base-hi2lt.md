# ranger-base-hi2lt — the bwrap render refuses a main checkout's `.git/config` deny

ADR 0095 D1-D3, implemented 2026-11-14. The design is
`docs/adr/0095-a-bwrap-seat-is-never-rendered-over-a-main-checkouts-writable-git-dir.md`;
this fragment carries what the implementation MEASURED and what the next
reader of that code would otherwise rediscover.

## D1: where the refusal sits, and what it is keyed on

`BwrapArgs`' deny arm, immediately after ranger-base-166va's
outside-every-grant skip. The two arms are the two halves of one shape:

| tree | the deny | a write grant contains it | what renders |
|---|---|---|---|
| linked worktree | `<repo>/.git/config` | no (objects, logs, the per-worktree dir, the ref pair) | nothing — 166va's skip |
| main checkout | `<repo>/.git/config` | yes, the cwd grant holds the whole `.git` | **refused** |

Both rows carry the SAME deny at the same spelling, because
`sessionGitConfigFiles` asks git for the config git would read and a linked
worktree's is the COMMON one. The condition is the containment, not the path.

The key is `bwrapGitConfigRepo` (bwrap.go, beside `grantInside`): a path whose
base is `config` and whose parent's base is `.git`, answering the repository.
Read off the PATH and not off git, deliberately — the renderer is handed lists
of paths and one cwd. The narrowness is pinned both ways:
`TestBwrapArgsGitConfigRefusalIsKeyedNarrowly` (the `config.lock` sibling,
`config.worktree`, a project's own `etc/config`, a git dir under another name
— none refused) and `TestBwrapArgsGitConfigOutsideEveryGrantStillOnlySkips`.

**Both halves of the condition are tighter than the ADR sentence reads, and
the tightening is MEASURED rather than reasoned.** The condition is

```
exists && writeGranted(own, d) && bwrapGitConfigRepo(d) == absResolve(cwd)
```

where `own` is the grants at or under the cwd. Taking "a write grant contains
it" literally refused **seven** renders in internal/posse's own wall pins:

| pin | why the literal reading refused it |
|---|---|
| `…WallClaudeHistorySeatbeltDeniesReadAndWriteForClaudeOnly` | linked-worktree seat, repo under the blanket `/tmp` grant |
| `…WallClaudeProjectsRendersOwnKeyInsideAReadOnlyProjects` | same, plus one genuine main-checkout row |
| `…WallClaudeProjectsSeatbeltRendersOwnKeyInsideADeniedProjects` | same blanket grant |
| `…WallClaudeSessionsSeatbeltDeniesReadAndWriteForClaudeOnly` | same |
| `…WallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate/a_linked_worktree` | same |
| `…WallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate/a_main_checkout` | honest shape, but the only writer was `/tmp` |
| `…WallWitnessTakesTheRowAwayWhenTheWallCannotBeRenderedAtAll` | same |
| `…LiveWallClaudeProjectsANonOwnCwd…` | genuine: it rendered with the main checkout as cwd |

Two separate causes, and neither is a design hole:

1. **The blanket `/tmp` grant.** `bwrapSeatTmp` answers "" when the home or
   the session dir is itself under `/tmp` — every fixture in internal/posse is
   — and with no private `/tmp` the render keeps the `/tmp` write grant
   `SeatbeltWritable` hands every seat. A repo under `/tmp` is then inside a
   write grant whatever tree shape it has. MEASURED by printing the set:
   `/tmp` was its last entry and `writeGranted(set, <repo>/.git/config)` read
   true for a worktree seat. Through that grant the seat really CAN write the
   config, so the refusal was not wrong — it was not D1's shape. Keying on the
   session dir's own git dir separates them: `<mainrepo>/.git/config` is not
   `<worktree>/.git/config`.
2. **A whole-tree-deny PID in a main checkout** (ADR 0086 D3): it holds no
   grant on its cwd at all, only `<cwd>/.beads`, so nothing of its own can
   write `.git/config` and there is no wall for a rename to take off. Asking
   every grant instead of the seat's own refused a PID ADR 0086 D3 had just
   cleared — which is what `wallwitness`'s `R0` message says out loud.

No production launch is in either shape: homes and worktrees live under the
home, so `carve.Tmp` is set and `bwrapDropTmpGrants` removes `/tmp` from the
set before the deny loop runs. Both discriminators carry a pin of their own
(`TestBwrapArgsGitConfigRefusalIsTheSessionDirsOwnGitDir`,
`TestBwrapArgsGitConfigRefusalAsksTheSeatsOwnGrants`), each built on a blanket
grant above the session dir so it cannot pass by accident.

**The residual, stated rather than closed:** a main checkout whose `.git` is
writable through some other grant OF ITS OWN — a PID `writable:` extra naming
the repo root — is not refused. `bwrapGitConfigRepo` is where that name joins.

## The two fixtures that changed, and why that is the decision arriving

ADR 0095's Consequences say a caged persona can no longer be a crew session in
the operator's own checkout on Linux. Two pins were measuring something else
THROUGH a seat of that shape, so they moved:

- `TestQAWallClaudeProjectsRendersOwnKeyInsideAReadOnlyProjects` dropped the
  `the main checkout` row of its non-own-cwd table and gained an arm asserting
  the refusal instead. The pure readers (`claudeOwnSessionTree`,
  `claudeProjectsBinds`) still answer for the main checkout and are still
  asserted there; only the render moved.
- `TestLiveWallClaudeProjectsANonOwnCwdRunsTheRealClaudeAndWritesNoTranscript`
  runs the real claude under a real wall in a plain directory now. The
  property it is about — a cwd that is not the seat's own session worktree —
  is carried identically (`claudeOwnSessionTree` answers "" for both, and the
  binds premise is asserted over the cwd actually used), and it asserts the
  main checkout is unrenderable with a message saying to put the run back
  there if that ever changes.

## The fixture trap behind most of that

**A wall fixture whose repo lives under `/tmp` refuses in a LINKED worktree
too, and it is not a bug in either half.** `bwrapSeatTmp` answers "" when the
home or the session dir is itself under `/tmp` — every fixture in
internal/posse is — and with no private `/tmp` the render keeps the blanket
`/tmp` write grant `SeatbeltWritable` hands every seat. A repo under `/tmp` is
then inside a write grant whatever tree shape it has, so D1's condition holds
for a worktree there as well. MEASURED by printing the set: `/tmp` was the
last entry of the writable set, and `writeGranted(set, <repo>/.git/config)`
read true for a worktree seat.

That is a correct answer about a box in that shape (the seat really can write
that config, and `bwrapSeatTmp`'s own comment says such a box has no seat to
protect), but it is not the shape D1 is about — and a fixture that cannot tell
the two apart measures nothing. `wallUngrantedDir(t)` (`/var/tmp`, with skips
for every blanket temp grant) is the root that can, and it is what arm 1 of
the D2 pins uses. No production launch is in that shape: homes and worktrees
live under the home, so `carve.Tmp` is set and `/tmp` is dropped from the
grants.

## D2: what the refusal costs dispatch (the ADR's own ASSUMED)

One reason line per pass. The bd writes are one claim and one unclaim on the
ARGV path and **nothing at all** on the typed one, because the typed path
claims AFTER the create and a refused create never reaches it. Either way the
fire loop's `default` arm benches the slot for the rest of the pass, so the
pass pays once and not once per bead.

WHICH RUNTIMES CAN REACH IT AT ALL, which is the part the ADR could not say
before the work:

| runtime | prompt | posse wall? | can meet D1 | cost |
|---|---|---|---|---|
| claude | typed | yes | yes — and it is this box's only dispatched bwrap seat | no bd write |
| grok | argv | yes | yes | one claim + one unclaim |
| codex | argv | **no** (SelfSandbox) | never | — |

So the storm D2 worried about cannot happen on the argv path either, and the
path this box actually runs is the quieter of the two. The worktree is not cut
(`PlanSessionTree` answering nil is the premise of the shape), so unlike ADR
0076 D7's refusals this one leaves nothing for `posse worktrees --land`.

## D3: already a row, now pinned

`posse gates <persona>` renders this wall through `BwrapReport`, and
cmd/posse/main.go already prints a refusing render as
`bwrap wall not rendered: <the refusal>`. Nothing was added; what was missing
was a pin, because that one generic arm is the whole of the distance between
D1's message and the operator.

Writing that pin turned up one precondition worth knowing for any later gates
fixture: **D19's `CLAUDE_CONFIG_DIR` demand fires one decision earlier than
D1.** A claude seat with the variable unset is refused at
`claude on cage seatbelt (bwrap) needs CLAUDE_CONFIG_DIR set in the launcher`,
so a fixture that does not set it reads that refusal as the row under test.
On this box the launcher sets it in its unit, which is what the D19 message
asks for.

## Verification

- `go test -run 'TestBwrapArgs|TestQADetachedHeadFallback' ./internal/posse`
- `go test -run TestQAGatesNamesTheMainCheckoutGitConfigRefusal ./cmd/posse`
- `make test` (all three arms) and `make tree-check`
- MUTATION-CHECKED: disabling the refusal block in `BwrapArgs` reds exactly
  the refusal-dependent arms — the main-checkout row of
  `TestBwrapArgsRefusesAMainCheckoutsGitConfigDenyInsideAGrant`, the first row
  of `TestBwrapArgsGitConfigRefusalIsKeyedNarrowly`, the ordinary-PID row of
  `…AsksTheSeatsOwnGrants`, `…IsTheSessionDirsOwnGitDir`'s control, the two
  updated wall pins' refusal arms, both D2 arms and the D3 pin — and leaves
  every linked-worktree row, including
  `TestBwrapArgsRendersNothingForAnExistingDenyNoGrantOverlaps`, green.
