# The record-reach gate asked for a directory no PID is granted (ranger-base-6hjm5)

2026-11-11. From ranger-base-o8us6's own handoff — measured there while
checking that the gate still tracked the grant it had just narrowed. All
readings on the D1 host, linux 7.2.15, git 2.56.0, from this bead's session
worktree.

## The shape, in one paragraph

ADR 0013 §4's reachability row probes, before dispatch, that the wall this
launch will run under can write the store of record "and its git dirs". The
gate derived that second half from `beadsGitDirs(home)` — `git -C <the
store's root> rev-parse --git-dir --git-common-dir`. In a LINKED WORKTREE
whose store of record is inside its own repository, that answers with the
COMMON dir `<repo>/.git` WHOLE, and since ranger-base-m2wf no PID is granted
that directory: a session worktree is granted `objects`, `logs`,
`packed-refs.lock` and its own loose ref beneath it, and nothing more. The
probe creates a file INSIDE each target, so the create in `<repo>/.git` is
refused, the row reads unrealized, and the launch refuses for a write the
wall deliberately withholds.

A gate nothing can satisfy, which is exactly the defect ADR 0086 D3 had just
removed one PID class over — the only way to pass it would be to re-grant
what ranger-base-m2wf took away. Here it was the PID that denies NOTHING.

## MEASURED

Through the two shipped producers (`App.SeatbeltWritable` and
`recordTargets`) over one real linked-worktree fixture with a sibling session
on the same repo, open PID (no deny at all), both sides narrowed to the
session's own repository BY LOCATION (`wgRepoGrants`) so the blanket `/tmp`
grant answers for nothing:

| | |
|---|---|
| open-PID repo-local grants | `<cwd>`, `<repo>/.git/worktrees/<session>`, `<repo>/.git/objects`, `<repo>/.git/logs`, `<repo>/.git/packed-refs.lock`, `<repo>/.git/refs/heads/posse/<session>{,.lock}` |
| `recordTargets`, at the finding | `<store>`, `<repo>/.git/worktrees/<session>`, `<repo>/.git` |
| **probed but not granted** | **`<repo>/.git`** |

Both store shapes read identically: a local store (`<cwd>/.beads`, no
redirect) and a redirect that stays in the repo (`<cwd>/inner/.beads`,
ranger-base-f5dg's REPRO 2).

PRODUCER-LEVEL, DELIBERATELY. The fixture is under `/tmp`, whose blanket
grant would mask the refusal under a live `sandbox-exec` or `bwrap` (ADR
0086's own caveat about its "move main" row, and `sbRoot`'s reason for
skipping) — so what is asserted is that the two shipped functions DISAGREE,
and now agree. A live wall reading needs a fixture outside `/tmp`, the shape
`TestQAWholeTreeDenyRecordTargetsSkipOnlyItsOwnGitDir`'s notes describe.

NOT EXPOSED ON THIS INSTANCE: all three stores here are reached through a
redirect into ANOTHER repository, where `beadsGitDirs` names that repo's
git dirs, `SeatbeltWritable` grants them whole for every PID (bd commits
there — ADR 0086 D3's stated exception), and the probe passes against its
own grant. The exposed shape is an instance whose store of record is its own
repo AND whose sessions are linked worktrees.

## What it is not

- **Not ranger-base-o8us6's finding or its regression.** That bead narrowed
  the GRANT, for a whole-tree-deny PID only. This reading is the PID that
  denies nothing, and it reads the same on both sides of that change.
- **Not the deadlock ranger-base-8t6j1 fixed.** That one was also the deny
  PID, and its control — "the open PID reads OK either way" — was taken in a
  MAIN CHECKOUT, where `cwd` is granted whole and covers `.git`. In a linked
  worktree the open PID's git grants are narrowed and that control does not
  transfer. It is the reason this is a bead and not a line in o8us6.

## The fix: ask the wall's producer, not bd's

The tempting one-line fix is to extend `recordTargets`' existing narrowing —
"store inside the session's tree -> probe the store alone" — from the deny
PID to every PID. **That is wrong, and silently.** In a main checkout `cwd`
is granted whole, so the `<cwd>/.git` probe there measures a REAL grant; it
is ranger-base-8t6j1's own control, and the one-liner drops it with nothing
to say so. The pin `TestQARecordTargetsStillProbeTheGitDirAMainCheckoutGrants`
is that mutant's gravestone.

So the git half moved into `recordGitTargets`, three answers, each a sentence
already written in `seatbelt.go`:

```go
func recordGitTargets(ag *AgentFile, dir, home string) []string {
        if !underDir(dir, home) {
                return beadsGitDirs(home)   // another repo: granted whole, bd commits there
        }
        if ag != nil && deniesFileWrite(ag.Deny) {
                return nil                  // ADR 0086 D3: no git grant at all
        }
        if g := sessionGitGrants(dir); len(g) > 0 {
                return g                    // a worktree: the narrowed set, m2wf's own list
        }
        return beadsGitDirs(home)           // a main checkout: the `<cwd>/.git` that `cwd` covers
}
```

`sessionGitGrants` is asked rather than re-derived, and that is the whole
point: the gate and the wall cannot disagree about what a session worktree
may write if only one of them computes it. Its main-checkout answer is empty
ON PURPOSE ("cwd's own grant already covers .git"), which is why the fourth
line is `beadsGitDirs` and not `nil` — in that shape it names exactly the one
directory `cwd` covers.

THE RULE THIS SETTLES, and it is the general one: **a reach target is a path
this launch's wall grants, never one bd would like.** A target the wall does
not grant refuses every launch; a grant the gate does not probe is a wall
nobody measures. ADR 0013 §4's clause and ADR 0086 D3's sixth door both now
say so.

## The pins

`internal/posse/recordreachgrant_qa_test.go` — four arms over the PAIR of
producers, because on this host no arm can run a live wall over a fixture
`/tmp` does not already grant:

1. the defect's shape — open PID, linked worktree, store in its own repo:
   every target inside some grant, with the pre-fix derivation run beside it
   as the non-vacuity witness (if `beadsGitDirs` stopped naming the common
   dir whole, or if `sessionGitGrants` stopped narrowing it, the arm says so
   and fatals rather than passing).
2. ranger-base-8t6j1's control kept: `<cwd>/.git` is STILL a target in a main
   checkout.
3. the store in another repo: its git dirs stay targets whole for BOTH PID
   classes, and the arm fatals if the wall stopped granting them.
4. the property, as a table: both store shapes × both checkout shapes × both
   PID classes, every target covered by the grant for that same launch.

Four mutants, each killing its own arm and nothing else under
`-run 'TestQARecordTargets|TestQAWholeTreeDeny|Reach|TestQAWorktreeGrant'`:

| mutant | result |
|---|---|
| the `sessionGitGrants` branch dropped (the pre-fix derivation) | arm 1, and arm 4's two `worktree/open` cells |
| the main-checkout fallback dropped (`return sessionGitGrants(dir)`) | arm 2, and `TestQAWholeTreeDenyRecordTargetsSkipOnlyItsOwnGitDir`'s own control |
| the `deniesFileWrite` branch dropped | arm 4's four `deny` cells, and the ADR 0086 D3 pin |
| the another-repo branch dropped | arm 3, and the D3 pin's third half |

## Where it landed

- `internal/posse/reachability.go` — `recordTargets`' git half extracted to
  `recordGitTargets`, and the reason above it.
- `internal/posse/recordreachgrant_qa_test.go` — new, the four arms.
- `docs/adr/0013-runtime-dispatch-contract.md` — §4 Reachability's clause,
  the header amendment, and the consequences entry.
- `docs/adr/0086-a-cage-map-lowers-the-wall-never-a-pids-rule.md` — D3's
  sixth door gains its gate half.
