# ADR 0086 D3's sixth door: the exception that did not test its own reason (ranger-base-o8us6)

2026-11-11. From ranger-base-md7ag, the QA lane's verify of
ranger-base-8t6j1 — its one LIVE finding. All readings on the D1 host,
linux 7.2.15, git 2.56.0, from this bead's own session worktree.

## The shape, in one paragraph

ADR 0086 D3 says a PID that denies the whole tree gets no write grant into
any git dir of its session, with one exception: the store of record's own
git dirs, "because bd commits there, and it is another repo"
(`seatbelt.go`'s own words). The reason is *another repo*. The code tested
whether the store was already covered by what this PID was granted:

```go
grantedBeadsBase := cwd
if deniesFiles { grantedBeadsBase = filepath.Join(cwd, ".beads") }
if home := beadsHome(cwd); !underDir(grantedBeadsBase, home) {
        add(home)
        for _, g := range beadsGitDirs(home) { add(g) }
}
```

Those are two different questions, and for one shape they part company: a
`.beads/redirect` that stays inside the session's own repository but outside
`<cwd>/.beads` — `<cwd>/inner/.beads`, ranger-base-f5dg's REPRO 2, and the
shape ranger-base-8t6j1's own `recordTargets` narrowing deliberately kept
"granted and probed". The store DIRECTORY genuinely needs the grant there
(without it bd's database comes back `operation not permitted`), and
`beadsGitDirs` then resolves that store with `git -C <cwd>/inner rev-parse`
— which is this very worktree — and hands back the session's own git dirs.

## MEASURED, before and after

Through the shipped producer (`App.SeatbeltWritable`), whole-tree-deny PID
(`Deny: ["Edit"]`), a real linked worktree with a sibling session on the
same repo, narrowed to the fixture's own repository by location:

| shape | repo-local grants, at the finding | after |
|---|---|---|
| no redirect (control) | `<cwd>/.beads` | unchanged |
| `<cwd>/.beads/redirect` → `<cwd>/inner/.beads` | `<cwd>/.beads`, `<cwd>/inner/.beads`, `<cwd>/inner/.git`, `<repo>/.git/worktrees/<session>` (d), `<repo>/.git` (d) | `<cwd>/.beads`, `<cwd>/inner/.beads` |

What the grant reached is W14/W15 exactly as D3 states them, and WIDER than
the grant D3 removed: `<repo>/.git` is the common dir WHOLE, not narrowed
below it the way `sessionGitGrants` was (ranger-base-m2wf), so `objects` and
`refs/heads` are both writable. A seat that cannot edit a file mints a
commit by plumbing and moves `posse/<session>`, which the launcher
fast-forwards onto `main`; in a main checkout the same grant reaches
`refs/heads/main`.

The other two doors agreed with it rather than catching it, measured in the
same pin: `BwrapWritable` returns early for this PID class, so the set it
hands `BwrapArgs` is the producer's and both entries were bound read-write;
`recordTargets` kept the git dirs as targets, so the record-reach gate
PASSED over the grant.

Not exposed on this instance — all three stores here are reached through a
redirect into ANOTHER repository, so every dispatched launch takes the
branch where the exception is what D3 says it is. The grant predates
ranger-base-8t6j1.

## The fix, and why narrowing rather than refusing

Two answers were open on the finding: narrow the grant, or REFUSE such a
seat over an in-repo store the way the pre-narrowing record-reach gate
refused it. Narrowing, for three reasons that are already in the tree:

1. D3's headline is the senior rule and the exception's stated reason is
   "another repo". Testing for another repo is not a new rule, it is the
   written one.
2. `!underDir(cwd, home)` — "the store of record left the session's tree" —
   is how `launchWritableRoots`, `sessionHooksDirs`,
   `sessionGitConfigFiles` and `CageMounts` already ask this question.
   `SeatbeltWritable` was the outlier, and `launchWritableRoots` skips
   `beadsGitDirs` for an in-repo store today.
3. Refusing would re-open the gate-nothing-can-satisfy shape
   ranger-base-8t6j1 had just closed, one shape over: the seat would be
   refused for a write its own rule forbids. A bd that cannot commit still
   claims, comments and closes in the store directory, which stays granted.

So the grant skips `beadsGitDirs` when the PID denies the whole tree and the
store is inside `cwd`, and `recordTargets`'s narrowing moves to the same
sentence (`underDir(dir, home)`, from `underDir(<dir>/.beads, home)`) — the
gate asks for exactly what the wall grants, or one of them is wrong. Had the
gate been left alone it would have refused every such launch for the write
the grant had just stopped giving.

Broad in one shape, deliberately: a store inside `cwd` that is its own
NESTED repository is another repo by git and not by path, and this test
denies its git dirs too. That is `launchWritableRoots`' answer at the
self-sandboxing tier already, so the mechanisms agree rather than disagree;
no store on this box or in ADR 0012 D3-C's layout has that shape, and a bd
that cannot commit there still lands the bead write.

## The pins

The live-defect pin `seatbeltdenyinrepostore_qa_test.go` shipped GREEN over
the hole and told its reader what to do when it reds. It red with its own
message, and was deleted per its instruction:
`TestQAWholeTreeDenyGetsNoGitGrantInALinkedWorktree` now drives BOTH store
shapes through one closure — no redirect, and the in-repo redirect — so the
claim is asserted identically either way, which is the point: the exception
is about another repo, not about a path. It carries the bwrap half the
deleted pin held (no entry of the set `BwrapWritable` returns lies in either
git dir) and the open PID as its control in each half.
`TestQAWholeTreeDenyRecordTargetsSkipOnlyItsOwnGitDir` gains the in-repo
redirect shape between its local-store and another-repo halves.

Two mutants, each killing its own arm and nothing else in
`-run 'TestQAWholeTreeDeny|TestSeatbelt|TestBeadsGitDirs|TestRecordReach|TestQARecord|TestParity'`:

| mutant | result |
|---|---|
| the git-dir grant unconditional again (`if true`) | 12 errors in the redirect half of the linked-worktree arm: both git dirs, `objects`, `logs`, `packed-refs.lock`, the ref pair, `refs/heads/main`, the sibling worktree's dir, `<cwd>/inner/.git` as an extra tree grant, and both bwrap bindings |
| `recordTargets` back to `underDir(<dir>/.beads, home)` | the record-targets arm alone, naming the gate/wall disagreement |

## One thing recorded, not fixed: the count of five

ADR 0086 D3's as-built bullet says the rule reaches five doors, and
ranger-base-md7ag measured that one of them cannot fire:
`sessionGitGrantsFor`'s own `deniesFileWrite` guard is unreachable, since
`SeatbeltWritable` calls it only in the `else` arm of `if deniesFiles` and
that is its only caller in the tree. It is kept as defence in depth for a
second caller that does not exist, and the ADR now says so beside the sixth
door — a reader of that bullet was counting one door that is inert and none
that was missing.

## Handed off: the gate probes a directory no PID is granted

MEASURED here while checking that the gate still tracks the grant, and it is
a DIFFERENT defect with the same smell — filed as ranger-base-6hjm5
(-t bug -l code, discovered-from edge confirmed). For an OPEN PID in a linked worktree whose store of
record is in its own repo, `recordTargets` probes the common dir
`<repo>/.git` WHOLE, and since ranger-base-m2wf no PID is granted that
directory — only `objects`, `logs`, `packed-refs.lock` and the session's own
ref beneath it. Both shapes read the same way:

```
open-PID repo-local grants: <cwd>, <repo>/.git/worktrees/<session>,
    <repo>/.git/objects, <repo>/.git/logs, <repo>/.git/packed-refs.lock,
    <repo>/.git/refs/heads/posse/<session>{,.lock}
recordTargets:  <store>, <repo>/.git/worktrees/<session>, <repo>/.git
PROBED BUT NOT GRANTED: <repo>/.git        (local store AND <cwd>/inner/.beads)
```

A producer-level reading, not a live wall run: this fixture sits under
`/tmp`, whose blanket grant would mask the refusal, so a live reading needs
a fixture outside it (ADR 0086's own caveat, and `sbRoot`'s reason for
skipping). Not exposed on this box — every store here redirects into another
repo, where `beadsGitDirs` grants that repo's `.git` whole and the probe
passes. The fix is not one branch: narrowing by the same sentence would
remove the open PID's own `.git` probe in a main checkout, which is a real
check and ranger-base-8t6j1's control, so the question is whether the gate
should derive its targets from the grant set instead of from `beadsGitDirs`.

## Where it landed

- `internal/posse/seatbelt.go` — `SeatbeltWritable`'s store-of-record grant,
  the arm comment above it, and `sessionGitGrantsFor`'s doc, which claimed
  the store's grants are kept "for every PID".
- `internal/posse/reachability.go` — `recordTargets`' narrowing condition
  and its reason.
- `internal/posse/seatbeltdenygitgrant_qa_test.go` — both arms widened;
  `internal/posse/seatbeltdenyinrepostore_qa_test.go` deleted.
- `docs/adr/0086-a-cage-map-lowers-the-wall-never-a-pids-rule.md` — D3's
  sixth door and the inert guard.
