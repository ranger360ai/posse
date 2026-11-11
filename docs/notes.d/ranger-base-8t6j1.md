# ranger-base-8t6j1 — a whole-tree write deny gets no git-dir write grant, and five doors had to be told

ADR 0086 D3, as built. The decision is one sentence — **a PID that denies the
whole tree gets no write grant into any git dir of its session** — and D3
names one place to put it: `SeatbeltWritable`'s `deniesFiles` arm, which
dropped `<cwd>/.git` and the `sessionGitGrants` set. That is where the build
started. It is not where it ended, because four other doors answer a question
about this PID's git writes WITHOUT reading the PID.

## The five doors

| door | what it derives | what it did for a whole-tree-deny PID |
|---|---|---|
| `SeatbeltWritable` (seatbelt.go) | the writable set | granted `<cwd>/.git`, `<own>`, `objects`, `logs`, `packed-refs.lock`, the ref pair |
| `sessionRefDirs` via `RenderSeatbelt`/`SeatbeltReport` | the create-only ref directory | granted `file-write-create` on `<common>/refs/heads/posse` |
| `BwrapWritable` (bwrap.go) | **adds** the ref's parent directory + keep file | bound `<common>/refs/heads/posse` read-write |
| `bwrapRefGrantDifference` (bwrap.go) | parity's ref-grant row | declared a ref grant, and the peer-ref widening with it |
| `recordTargets` (reachability.go) | the record-reach gate's probe targets | DEMANDED a write in `<cwd>/.git` — see "The fifth door" below |

The first four are grants and the fifth is a gate, which is why it reads the
other way round: the first four handed back a piece of what D3 gave up, and
the fifth refused the launch for not having it.

The third is the one worth stating out loud. ADR 0080 D20's substitution is
not a filter: the ref leaf cannot be bound at a mount, so `bwrapGitSwap`
DROPS the leaf pair and APPENDS the parent directory, derived from `cwd` and
`repoBranch(cwd)` and not read off the list it was handed. So with D3 applied
to the producer alone, `SeatbeltWritable` answered no ref and the renderer
added one back — `refs/heads/posse` bound read-write, which is every peer
session's branch in the fleet's own naming scheme. MEASURED by mutation: with
`BwrapWritable`'s early return removed and everything else at HEAD, the
render asks the launcher for `refs/heads/posse/.posse-keep` and binds that
directory, for a PID whose whole point is that it cannot write.

The rule is therefore spelled once — `deniesFileWrite(ag.Deny)`, the
pre-existing any-of-the-three union (cageinner.go) — and read at each of the
four. Two of them read it through a named wrapper (`sessionGitGrantsFor`,
`sessionRefDirsFor`, both in seatbelt.go, where the whole reason is written);
`BwrapWritable` and the parity row take the PID as a parameter they did not
have before.

## What the parity row does instead, and why it is silence

ADR 0080 D20's row says what this mechanism's ref grant is, because an
operator cannot read it off anything else. After D3 the two mechanisms agree
for this PID — neither grants a ref — so the row goes **silent**, like the
detached-HEAD arm beside it, rather than gaining a third arm. A line printed
on Linux alone, over a wall both mechanisms now render identically, reads as
a capability of this mechanism: the reverse of what D3 did. What says the
wall is real for such a PID is D5's witness at the launch
(ranger-base-ctxeq); what says where it allows writes at all is the writable
set `posse gates` prints, from one list, on both mechanisms.

## MEASURED

The D1 host (linux 7.2.15, bubblewrap 0.13.1, git 2.56.0), 2026-11-10, load
12, from this bead's own session worktree.

**R0, reproduced and cleared against the live home.** The security PID's
real deny list read from this box's live home, rendered through `bwrapRender`
for cwd = this bead's own session worktree (a linked worktree), with no
write — the gates dir pointed at a temp dir and `CLAUDE_CONFIG_DIR` set as
the launcher sets it (ADR 0080 D19, which refuses the render first when it is
absent).

- at HEAD: **renders**. The whole set is the claude config dir,
  `<cwd>/.beads`, the home's `state`, that persona's memory dir, and the
  store of record's own `.beads` and `.git` in the repo the redirect names,
  plus `/tmp`, the gates dir and the home caches. No `<cwd>`, no
  `<cwd>/.git`, no git dir of the session's own repo, and no keep file.
- with the pre-D3 arm restored and nothing else changed: `RENDER REFUSED:
  bwrap cannot grant write on <cwd>/.git: it is not a directory` — R0's
  message, word for word over the real path, which is the ADR's reading
  taken again on a second day.

**The live wall, under the rendered argv**
(`TestLiveBwrapWholeTreeDenyGetsNoGitWriteInItsSession`, real bwrap, real
linked worktree). Refused: a create in cwd, `git hash-object -w`,
`git commit`, a write in the per-worktree git dir, a write in the shared
`objects`, a hand-written ref move. Allowed: `git status`, `git log`,
`git diff`, and a create in `<cwd>/.beads`. The operator's checkout still
reads `posse/s1` at the same sha afterwards, which is the half an exit code
cannot answer.

**What the store of record keeps, and that it is another repo.** D3 keeps
`beadsHome` and `beadsGitDirs` because bd commits there. On this instance
`<cwd>/.beads/redirect` points at the queue repo's `.beads`, so what a
whole-tree-deny seat can still commit in is THAT repo — not the one whose
`posse/<session>` the launcher fast-forwards onto `main`, which is the branch
W14/W15 was about. The grant and the capability it was feared to
carry are in different repositories here, by the redirect and not by the
rule; a box without that redirect would have them in one, and D3 says so.

## The fifth door, which is a gate and not a grant

D3 took a write away, and one gate was asking for it. The record-reach row
(ADR 0013 §4) probes `beadsHome(cwd)` **and its git dirs**, because at L2 a
real `bd sync` commits the JSONL and that commit takes `index.lock`. Where
the store of record is the session's own tree — no `.beads/redirect` — those
git dirs ARE what D3 just removed, so the probe refuses the launch for a
write the PID's own rule forbids. Nothing can satisfy that: the only way to
pass it is to re-grant what D3 removed.

MEASURED 2026-11-10 on the D1 host, the bwrap arm, through the two calls the
arm itself makes (`RenderWall` then `wallReachRow` over `recordTargets`), on
a main checkout with a local store **outside `/tmp`**:

| PID | before D3 | after D3 | after the narrowing |
|---|---|---|---|
| denies nothing | OK, 2 targets | OK, 2 targets | OK, 2 targets |
| denies Edit, Write | OK, 2 targets | **unrealized**: `<cwd>/.git … Read-only file system` | OK, 1 target |

The fixture had to sit outside `/tmp` to measure anything, and the first run
of it proves the point: under `/tmp` both PIDs read "OK, 2 targets", because
`SeatbeltWritable` grants `/tmp` unconditionally and the probe wrote into the
blanket grant. That is ADR 0086's own caveat about its "move main" row, met
again one file over.

So `recordTargets` takes the PID: for a whole-tree-deny PID whose store is
its own tree it names `home` alone — the same answer the CONTAINER arm
already gives one tier down, and for the same reason, that a `bd` which
cannot commit does not take `index.lock`. A store of record in ANOTHER repo
keeps its git dirs as targets, because D3 keeps those grants. The pin asserts
both halves (`TestQAWholeTreeDenyRecordTargetsSkipOnlyItsOwnGitDir`), and the
mutation that drops the narrowing reds it.

On this instance nothing was exposed: every store here is reached through a
`.beads/redirect` into another repo, so every dispatched launch takes the
other branch. It is the local-store configuration
— another box, or this one after a store moves — where the gate deadlocks.

## The fixture lesson, kept

D6's instruction is that every pin drives a whole-tree-deny PID in a REAL
linked worktree (`wgNewFixture`) and in a main checkout. One thing got in the
way: `wgNewFixture` takes `sbRoot`, which **skips wherever TMPDIR resolves
under `/tmp`** — the profile's blanket temp grant would cover a fixture
sitting inside it — and that is every Linux box, including the only host this
defect was ever reachable on. So the five set-and-render pins
(`seatbeltdenygitgrant_qa_test.go`) run under `wgFixtureIn`, the same fixture
body under a root they choose, and narrow the set by LOCATION instead
(`wgRepoGrants`: keep every entry inside this fixture's repo or worktree, drop
every entry outside it). That is exact rather than convenient — a grant on
`<cwd>/.git` or on the shared `objects` is still in the narrowed set and
still reds the arms — and it is available only because nothing in those five
arms runs a command under a wall. An execution pin cannot do this, which is
`sbRoot`'s own reason for skipping: a kernel reads the whole profile, and a
filter applied afterwards would be a claim about a wall nobody has.

**Each pin was mutation-checked, one mutation per door, against HEAD:**

| mutation | reds |
|---|---|
| the deny arm keeps `<cwd>/.git` and `sessionGitGrants` | the worktree set arm, the main-checkout arm, the bwrap render arm (at R0's own message), the profile arm |
| `sessionRefDirsFor` ignores the PID | the profile arm, naming the `file-write-create` block |
| `BwrapWritable`'s early return removed | the bwrap render arm, naming the keep file and the bound ref directory |
| the parity row ignores the PID | the parity arm |
| `recordTargets` ignores the PID | the record-targets arm |

## What else moved

`seatbeltredirect_test.go`'s two deny-PID pins asserted `<cwd>/.git` was
granted — the grant D3 removes — and now assert the opposite beside the
store of record's git dir, which stays. That pin is where the distinction
lives, because a redirect is what makes those two paths two different
repositories.

## Residual, stated

**The container tier (L4) still grants it.** `CageMounts` (cage.go) mounts
`<dir>/.git` read-write as a carve-out over the `:ro` repo for exactly these
PIDs, and its comment reasons from the premise D3 just removed — "L2 grants a
write-denying PID `cwd/.git` … and a tier that took it away would be
enforcing more than the gate". In a main checkout that reaches
`refs/heads/main`. It is narrower there than it sounds — the launcher detaches
HEAD for a caged worktree session, so `sessionCommonDirWrites` grants no ref
at all — and no PID on this box asks for both `cage: container` and a
whole-tree write deny today, so nothing is exposed now. It is a design
question at a tier this bead was not scoped to and it is filed as one
(ranger-base-56w67).
