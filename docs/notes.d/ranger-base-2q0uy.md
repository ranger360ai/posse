## The host staging-root write is the stand-down shape's, not a fixture's, and no fixture root closes it (ranger-base-2q0uy)

ranger-base-aurb8 fixed the darwin half of one defect and named the linux half
here: a wall-render fixture whose `$HOME` is under `/tmp` renders no private
`/tmp`, keeps the blanket temp grants, and the prep then creates
`<temp root>/posse-upgrade` **on the host** from a test binary. That bead
prescribed the fix — "AT THE FIXTURES AND NOT AT THE FENCE", in one of two
shapes. This fragment is the measurement that says the prescription is not
sufficient, and prices the three candidates it leaves.

Nothing is fixed here, and nothing is going to be: **ADR 0100 D5** (ruled
2026-11-22 under ranger-base-xw7tc, commit 0634a6d1) took these readings and
ruled the stand-down render CORRECT — the blanket temp grants stay, every deny
under them is rendered, and the uncaged test binary's leftover is a stated
RESIDUAL. A9-A11 of that record are the three candidates below, all rejected.
This bead's own prescription and its done-when are withdrawn by D5. What
follows is the measurement that forced the ruling, kept because the ruling
rests on it.

### The chain, read rather than assumed

`bwrapSeatTmp(home, cwd)` answers `""` — no private `/tmp` — when **either**
the process `$HOME` or the session dir is under `/tmp` (wallseatcache.go, and
its own comment says every fixture in this package is). With `carve.Tmp` empty,
`bwrapRender` never calls `bwrapDropTmpGrants`, so the blanket temp grants
(`seatbeltTempGrants`: `/tmp`, `$TMPDIR`, darwin's `/private/tmp`) stay in the
write set. `upgradePrivateDenied` puts one deny under each of them, bwrap's
deny arm preps every absent deny that sits inside a write grant, and
`runBwrapPrep` makes it — admitted by the prep host fence, because
`os.TempDir()` is one of the fence's declared roots and on linux that IS the
box's shared `/tmp` (prephostfence.go).

seatbelt.go's own comment over that deny is what sent this bead the wrong way,
in one sentence: *"bwrap renders nothing for it and needs to — the seat's /tmp
is already a private bind."* True of every production launch. False in the
stand-down shape, which is every wall fixture in internal/posse. Under D5 the
render is right and the sentence is merely stale (ranger-base-x5ydb corrects
it); before D5 it read as a statement that the step could not exist, which is
why its appearance looked like a defect rather than a shape nobody had ruled
on.

The `$HOME` in that first predicate is the **process's** — `os.UserHomeDir()`,
which is TestMain's one temp home for the whole binary
(`os.MkdirTemp("", "posse-testhome-")`, herdr_test.go). A `t.Parallel` test
cannot move it: `t.Setenv` panics under one.

### MEASURED 2026-11-22 (linux 7.2.15-1-omarchy, go1.27.3, 16 cores, at 97979d7d)

Full arm 1 (`go test -count=1 -v ./internal/posse`), each run with a FRESH
`TMPDIR` under `/tmp` — which is the shape that shows the write, because this
box's `/tmp/posse-upgrade` already exists and the render's `!exists` arm then
skips it.

| run | wall | verdict | `posse-upgrade` left in the temp root |
|---|---|---|---|
| baseline, clean tree | 264.3s | **ok** | **yes** |
| candidate B (below) | 313.6s | FAIL, 46 tests | no |
| candidate C (below) | 286.0s | FAIL, 2 tests | no |

**The class is 37 prep attempts in 16 files.** Measured by a probe that refuses
in `runBwrapPrep` any step whose basename is `upgradePrivateRootName`, then
reading the reds: ~40 tests, by file —
`wallwitness_qa_test.go` 5, `bwrapprepauthority_qa_test.go` 5,
`bwraplaunchline_qa_test.go` 5, `readerprobeprep_qa_test.go` 3,
`wallseedprojectconfig_qa_test.go` 2, `walllaunchmechanism_qa_test.go` 2,
`walleddetach_qa_test.go` 2, `launchsitewiring_qa_test.go` 2, and one each in
`wallwatch_test.go`, `wallunixrender_qa_test.go`, `reportwallwrite_qa_test.go`,
`launchexec_qa_test.go`, `credentialdirrelaunch_qa_test.go`,
`bwrapmaincheckoutfallback_qa_test.go`, plus two parity-report rows. All 37
name the SAME path, so the host cost is one empty 0755 directory per temp root
per box and the thirty-seventh write is a no-op `MkdirAll`.

That is an order of magnitude more than the bead's "a change across several
wall fixtures".

### The three candidates, priced

A, B and C here are ADR 0100's **A9, A10 and A11**, which quote these prices
and reject all three.

**A (= A9) — move the wall-render fixtures off `/tmp`** (the bead's second shape).
It cannot close the class, and that is not a matter of churn. The stand-down
fires on `$HOME` **or** cwd, and at least one member needs its cwd under `/tmp`
as its stated premise: `TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsThe-`
`Create` (wallwitness_qa_test.go:450) says so in its own refusal —

```
PREMISE GONE: the real bwrap wall REFUSED the create in /tmp/posse-wwtmp-…/repo
  This arm needs a tree the rendered wall leaves writable, and the placement
  that buys it is a root under /tmp (wwTempRoot)
```

so its render keeps the blanket grant whatever its `$HOME` is, and keeps
emitting the step. `TestQAWallPinnedDstPrivateSetGateStandsDownWithTheWall`
(a CONTROL) and `TestLiveBwrapWithoutSeatTmpReachesTheHostsTmp` are in the same
position. Cost of the rest: 15 files, and every fixture that gains a
`t.Setenv("HOME", …)` loses `t.Parallel` in a 264s arm.

**B (= A10) — hoist `sbNoBlanketTempGrants` to TestMain** (the bead's first shape, made
parallel-safe by setting the var once before `m.Run` instead of per test).
It kills the step everywhere — 0 probe hits — and reds **46 tests**: the whole
`TestUpgrade*` family, because `upgradePrivateDenied` reads the same var and
returns nil, so the verb's own refusal fires ("the private copy would be staged
at …, which is not one of the names every rendered seat profile denies");
`TestQASeatbeltTempGrantsAreTheThreeBlanketOnes`, whose subject IS the grant;
the three `TestLiveBwrap*` launches; and `TestQAWallPinnedDst…`'s control.
Per-test it is parallel-unsafe and that is why it is confined to `sbRoot`
today.

**C (= A11) — bwrap drops the blanket temp grants in the stand-down shape too**
(`bwrapDropTmpGrants` unconditionally, at `bwrapRender` and at step 1c). 0
probe hits, no leftover, and **2 reds**, both of them pins whose subject is
exactly the behaviour it removes:

- `TestLiveBwrapWithoutSeatTmpReachesTheHostsTmp` (bwraptmp_qa_test.go:412) —
  the declared CONTROL for ranger-base-7wdb7's fix, which grants `/tmp` with
  `SeatbeltCarveOut{}` and asserts the seat reaches the host's `/tmp`
  (`map[create:refused overwrite:refused plant:present]`, want present and
  overwritten). Its own comment: *"Without this arm the one above passes on a
  probe that reaches nothing."*
- `TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate` — the premise
  above.

And it carries a production cost the two reds do not name: a seat whose `$HOME`
is under a blanket temp grant loses write on its own home, silently — the wall
comes up and `bwrapRefuseWallRootGrant` stands down there, so nothing says so.
Narrowing C to "drop unless the seat's `$HOME` is under the grant" keeps that
seat working and takes the test fixtures straight back to the baseline, since
their `$HOME` always is.

### What the ruling did with them

Every candidate either changes a pinned production property of the bwrap write
set or re-premises pins whose subject is the stand-down shape, so the bead's
own instruction — fix at the fixtures, leave the fence alone — is satisfiable
by none of them. **ADR 0100 D5** rules that this is not a defect to route
around: in the stand-down shape the seat's own tree IS in the shared
directory, so the blanket grant is what lets it write that tree, and a render
that took the grant away would hand the seat a wall it cannot work behind with
nothing to say so (`bwrapRefuseWallRootGrant` stands down there too). Every
deny inside those grants is therefore rendered, `upgradePrivateDenied`'s
staging root included, and for a LAUNCH its prep is the ranger-base-ua625 wall
doing its job.

The three candidates are A9 (fixtures), A10 (the TestMain stub) and A11 (drop
the grants) of that record, rejected in the terms measured above. The fence's
`os.TempDir()` and `bwrapTmpMount` roots stay: the fence exists to refuse a
step whose path the render took from a host input the fixture did NOT move —
the passwd home, `XDG_STATE_HOME` — and this path comes from the fixture's own
shape, in a world-shared temp root the fence admits by declaration. Both
controls stay as written, premised on D5 now rather than on a fixture
accident. Nothing was re-premised and nothing retired.

One scoping clause of D5 is worth reading off this run from the other side: a
test INSIDE a walled seat writes the seat's private `/tmp` (D1), so only an
uncaged run reaches the host. These readings are an uncaged one — this box's
`/tmp/posse-upgrade` was already there, timestamped hours before this session
touched anything, which is a `/tmp` shared with the rest of the box and not a
seat's private bind.

The comment that states the old premise in production code — seatbelt.go's
"bwrap renders nothing for it and needs to", with wallseatcache.go's and
prephostfence.go's neighbours — is ranger-base-x5ydb's, cut by the ruling as
comment-only work. Not touched here.

### Not closed here

Nothing. D5 is the decision, and the residual it states carries no open bead:
one empty 0755 directory per temp root, at the name `posse upgrade` creates
for itself anyway, from an uncaged run only.
