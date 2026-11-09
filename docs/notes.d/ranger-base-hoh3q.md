# D23's perimeter was a spelling, and the crew pins were built from half the set (ranger-base-hoh3q)

Two of the three findings on `ranger-base-hoh3q`, the findings bundle of the
four-close verify `ranger-base-41g0y`. Finding 3 (`verify_chain_max` cannot
fire at `verify_batch: 4`) is dispatch correctness and has its own bead,
`ranger-base-bu8rz`; nothing here touches it.

MEASURED 2026-11-09, darwin 25.6.0 arm64, laptop, at main `71b1f718`.

## Finding 1 — the perimeter asked a spelling where the decision was location

`pidWritableIsHostPath` was `filepath.IsAbs(ExpandTilde(w))`: it asked whether
the DECLARED STRING was absolute. ADR 0080 D23 decided something else — "Prep's
authority is posse's own perimeter: the session tree, the gates dir, ..." — and
`pidWritableExtras` resolves a relative entry with `filepath.Join(cwd, w)`,
which CLEANS `..`. So `writable: ["../../shared"]` was a path OUTSIDE the
session tree that D23 classed as relative, and all four of D23's surfaces were
silent on it at once. Reproduced at the bead's own door (`bwlFixture`, session
dir `<root>/001/proj`), one PID declaring `writable: ["../../d23-escape"]`, with
the pre-fix predicate restored:

```
PREP: kind=mkdir -p path=<root>/d23-escape
bwrapNoCreate = [<root>/001/proj/.beads]   <- the escape is not on it
r.Absent      = []                         <- nothing reported
os.Stat(<root>/d23-escape) after runBwrapPrep: EXISTS
```

At the real spelling cwd is `~/.posse/worktrees/posse/<session>`, so
`../shared` is a junk directory in the operator's `worktrees/` dir that no
session retires — which is `bwrapNoCreate`'s own stated reason for counting `~`
as outside — and enough `..` reaches `/opt/shared` itself, the path D23 was
written about.

**The prose said the opposite in three places**, one of them operator-facing
(`posse agent check`'s warning ended "A path relative to the session dir is
created with the session tree"), which is why fixing the prose alone would have
left the host write AND kept teaching it.

### What it asks now

`pidWritableIsHostPath(w, cwd)`, one rule with two arms:

- with a session dir: `!underDir(cwd, filepath.Join(cwd, w))`. `underDir`
  resolves BOTH sides, which it has to — on darwin a fixture cwd under
  `/var/folders/...` and its `/private/var/...` real path are one directory, and
  comparing one resolved side against one unresolved one classes every relative
  extra as a host path and kills the control arm.
- with none (`posse agent check`, which stats nothing and answers the same on
  every box): `!filepath.IsLocal(w)`. The string is the whole of what is known,
  and a cleaned relative path that opens with `..` climbs out of whatever tree
  it is joined to, on every host. Strictly the WEAKER arm, and that is the
  direction to err in — the lint's job is to name what the operator typed.

**Why a path and not a string.** The cheaper fix — re-class any entry whose
cleaned form opens with `..` — closes the reported spelling and reads the string
again. `TestQABwrapPrepReadsTheRESOLVEDPathAndNotTheSpelling` is the arm that
separates them: an entry UNDER a symlink that leaves the session tree resolves,
through the same `absResolve` `pidWritableExtras` grants through, to a host path
with no `..` anywhere in the PID. MEASURED — that arm is the ONLY one of the
four that reds under the string-only fix, and it needs no absolute path and no
`..` from the operator: a tracked `tools/cache -> /opt/shared` in the dispatched
repo plus `writable: [tools/cache/seat]`, and the launcher makes
`/opt/shared/seat`.

`resolveExisting` draws that arm's edge and the pin says so: a symlink whose
target does NOT exist resolves to nothing — `EvalSymlinks` fails, the climb
rejoins the tail — so the grant is the link path inside the tree. The escape
needs the target present and the tail under it missing.

### What moved

| | |
|---|---|
| `pidWritableResolve` (new, `seatbelt.go`) | the ONE resolution of a declared entry, factored out of `pidWritableExtras`. `pidHostWritable`'s comment used to ASSERT the two spellings were the same; this makes it structural, which matters now that a relative entry needs the join to be spelled at all. |
| `pidAbsoluteWritable` → `pidHostWritable(ag, cwd)` | named for the question. `docs/notes.d/ranger-base-sg7a7.md` still names the old one. |
| `bwrapAbsoluteWritableDifference` → `bwrapHostWritableDifference(..., cwd)` | the parity row stopped being directory-independent: an absolute path is absolute wherever you ask, a relative escape is not. `applyBwrapHostGrants` takes the same `dir` the rest of `checkParityIn` reads, and states nothing about a relative entry where there is no dir. |
| `pidcheck.go` | two warnings, not one, because the remedy differs: an absolute path gets created outside posse, a `..` gets re-spelled. |

### Mutants

Pre-fix predicate (`return false` for the non-absolute arm) — all four surfaces
red, each naming its own:

```
TestQABwrapPrepDoesNotCreateAPathOutsideTheSessionTree   prep, bind, Absent, the list
TestQABwrapNoCreateIsTheTwoMembersD23Names               the list
TestQAAgentCheckWarnsOnAnAbsoluteWritable                the lint
TestQAAnAbsoluteWritableThisHostLacksIsADeclaredDifference   the parity row
```

String-only fix (`!filepath.IsLocal(w)` in both arms) — only
`TestQABwrapPrepReadsTheRESOLVEDPathAndNotTheSpelling` reds, which is the point
of that arm.

The controls stay green either way:
`TestQABwrapPrepDoesNotCreateAnAbsoluteWritableExtra`'s `scratch` arm, and
`scratch/../scratch` — a `..` that resolves back inside the tree is session
furniture and keeps its prep.

## Finding 2 — a pin built from half the set it was pinning

`ranger-base-c7i8t` moved `DeriveCrewLiterals` onto `isShippedPosseName`,
widening the crew arm's exclusion by `path.Base(posseModule)` — the one name
`retirableExampleNames` lacks. `visibility.go:1632-1638` states the widening in
as many words. No pin read it: `shippedRoleNamesForTest` still returned
`retirableExampleNames(posse.Seed)`, so PIN (a)
`TestQACrewLiteralsAreTheStaffedPIDsLessTheShippedRoles` and PIN (d)
`TestQAShippedRoleNamesDoNotArmTheCrewArm` staffed the ten roles and never a
lane named `posse`.

MEASURED by holden on the bead: c7i8t's widening undone in the source was GREEN
in arm 1 whole (499.903s), arm 2 whole (325.172s), arm 3 whole (348.947s),
`cmd/posse` (193.995s) and the crew pins under a filter (6.578s).
`crewpathwall_qa_test.go` is `//go:build posse_arm2`, so an unfiltered
`go test ./internal/posse` never compiles the crew pins at all — which is why
the first reading of that mutant looked like a clean full-package pass.

**The fix is the fixture: read the SOURCE's own closed set.**
`shippedPosseNamesForTest` returns `shippedPosseNames()`, with the premise
asserting `posse` is among them — that member is the one whose loss the fixture
exists to catch, and a set that quietly stopped carrying it would make both pins
green over nothing again. PIN (a) then staffs a lane called `posse`, derives 3
literals with the exclusion and 4 without; PIN (d) staffs a home of nothing but
posse's own names and must derive none.

Re-MEASURED here with the same mutant (`isShippedPosseName` replaced by a local
predicate over `retirableExampleNames(posse.Seed)` alone, `visibility.go`
restored byte-identical, md5 `36c9e4e85fd13b02d69a4b403097535a`):

```
PIN (a)  the set is the staffed PIDs and nothing else, got [posse quint-us quintus zephyrina]
PIN (d)  must derive no crew literals, got [{Class:persona Value:posse PathsOnly:true}]
```

Boundary-free, `posse` hits 864 of this repo's 1,621 tracked paths, every
`internal/posse/...` among them — so on the first home that staffs such a lane,
losing the exclusion refuses every commit touching this package. Inert on this
box, which staffs no such lane, and nothing would have caught its loss.

## What is NOT closed here

A dangling symlink at a `writable:` path makes `bwrapRender` REFUSE the launch:
`resolveExisting` leaves the grant as the link path, and prep's `os.MkdirAll`
answers `file exists`. Found while building the symlink arm above; left alone
because the refusal is loud and names the path, which is this mechanism's own
preference, and because it is nobody's finding on this bead. Noted on
`ranger-base-hoh3q` rather than filed.
