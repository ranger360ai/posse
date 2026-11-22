# ranger-base-ddn56 — two controls that shipped without a positive control

`ranger-base-xxpbt` verified four closes and filed two findings. Neither is a
defect in what those closes claim to have fixed; both are the class
`ranger-base-0eysw` named — a control with nothing to witness it. One is a
fixture guard that could only speak in a venue nobody types, and one is a
venue that read an abstention as a pass.

Environment for every reading below: linux 7.2.15 x86_64, go1.27.3, 16 cores,
at `91381cab`, 2026-11-22. Every mutant was applied ALONE and restored from a
golden copy taken before it.

## Finding 1 — the fixture guard with no witness

`internal/posse/wallseedprojectconfig_qa_test.go`'s fixture was
`wspFixtureIn(t, root)`, and its guard — `!underDir(bwrapTmpMount, root)`,
`t.Fatalf` — was the sentence `ranger-base-shf97`'s close called "the one
place in the source that can still say no to a third arm". It is the only
place: the tree-wide door reads a render a test writes as `BwrapArgs` or
`bwrapArgsWatched` along identifier edges, and these arms reach it through a
METHOD (`f.a.bwrapRender` inside `f.prep`), which that door deliberately does
not follow (`internal/treepins/bwraphomestanddown_qa_test.go`, "A METHOD CALL
IS NOT AN EDGE").

But the guard reads a root the CALLER passed, and a bare `t.TempDir()` is
under `/tmp` wherever the box's temp root is. RE-MEASURED here with both call
sites put back to a bare `t.TempDir()`, `go test -count=1 -run
TestQAWallSeedProjectConfig ./internal/posse`:

| shape | verdict |
|---|---|
| `TMPDIR` unset (every seat, this box, ci.yml's ubuntu legs) | `ok 0.881s` |
| `TMPDIR=/var/tmp/…` | RED, both arms, at the guard |

So the coverage the sentence claimed held only where `make tmpdir-check`'s
venue was already running, and a third arm written tomorrow with a bare root
passed everything a seat types.

### The fix is neither of the two the bead named

The bead offered a witness over the guard's predicate, or a source-level pin
that both call sites take `wallStandDownDir`. Both keep the parameter, and the
parameter is the whole defect: it is the thing that lets a caller be wrong.
`wspNewFixture(t)` takes its own root from `wallStandDownDir` and there is no
parameter to get wrong, so the witness is the compiler and not a run in one
venue. The guard went with it — an assertion that cannot fail on the path it
runs is worse than none — and the host question it was a second copy of is
`wallStandDownDir`'s own check against `bwrapTmpMount`, which fires for every
caller of it rather than for this file alone.

`wwTempRoot` (`wallwitness_qa_test.go`) keeps a guard of the same shape, and
it is not in this class: the root there is one the helper itself cut under
`/tmp` and resolved, so the check is the HOST question (darwin spells `/tmp`
`/private/tmp`) and no caller can hand it a bad value. Its cross-reference to
"wspFixtureIn's guard" now names `wallStandDownDir`'s.

## Finding 2 — the venue read a dark member as a pass

`make tmpdir-check` runs internal/posse's three arms with `$TMPDIR` at a
`/var/tmp` root (`ranger-base-m33n5`, ADR 0098 D5). Its two REFUSALS are
measured — a root inside a blanket temp grant, and a root it cannot create,
both exit 2, "because an abstention is never a pass (ADR 0097 D5)". The RUN
had no such arm: it is a plain `go test`, so a member that skips under the
venue prints `ok`.

RE-MEASURED, `-run 'TestQAWallPinnedDstPrivateSetGateStandsDownWithTheWall|
TestQAWallClaudeProjectsRendersOwnKeyInsideAReadOnlyProjects'
./internal/posse -v`:

| shape | the member | package line |
|---|---|---|
| `TMPDIR=/var/tmp/…` | `SKIP (0.00s)` | `ok 5.376s` |
| `TMPDIR` unset | `PASS (0.00s)` | `ok 1.125s` |

`wallpinneddst_qa_test.go:247` was the gate: `if bwrapWallRoot() != "" {
t.Skip("NOTHING MEASURED: this box's TMPDIR is not under /tmp, …") }`. Its
condition IS the venue's variable, so the door's own shape darkened it and the
door said `ok` over it.

And it was dark on the VENUE OF RECORD too, which is the part worth saying:
ADR 0098 D5 makes ci.yml's macos legs that venue, darwin's `t.TempDir()` is
under `/var/folders/…`, so this pin had never measured there on any push.

### The fix is the root, not a door that catches the skip

`wrcFixtureIn(t, wallStandDownDir(t))`. The stand-down this pin is named for
needs the home under `bwrapTmpMount`, and `wallStandDownDir` is a root there
whatever `TMPDIR` says — so the test measures in BOTH shapes and the host
cannot darken it. The guard is now a CONTROL that reds: with the home taken
from a root the fixture chose, a non-empty `bwrapWallRoot()` is this fixture
sitting somewhere it did not mean to, which is a defect and not a fact to live
with.

### The door for the class, and why it is a scan here

`TestQANoWallPinGoesDarkUnderTheOffTmpVenue`
(`internal/posse/wallvenuedark_qa_test.go`, `make pkgcensus-check`). No test in
the package may abstain BECAUSE the box has a wall root.

The direction is the whole of the rule, and one of the two is safe by
construction. `bwrapWallRoot()` is non-empty exactly when `$HOME` is NOT under
`/tmp`, and all this venue does is move temp roots OFF `/tmp` — so it can only
turn a wall root ON, never off. A `!= ""` guard is therefore the only
direction the venue can DARKEN; a `== ""` guard is one it can only UN-darken,
whatever that site's fixture does. That is why the rule reads the operator and
not the site: it does not have to know where any fixture put its home.

Fourteen sites guard on `== ""` and one guarded on `!= ""`. A condition in any
other shape is REPORTED, not classified: a guard whose direction cannot be
read is a skip nobody can say which venue darkens.

This is a scan although ADR 0098 D5 rules that the door for the fixture-home
class is a RUN, and the two are not in tension. That class needs dataflow —
`ranger-base-shf97` measured every static variant and the best found 2 of 4
members at 15 false reads. This one is one syntactic fact, a single call
compared against the empty string at every live site, with nothing to infer.
It costs 0.22s, takes no suite slot, and it graded against the live tree: with
`wallpinneddst_qa_test.go` restored to its pre-fix source the pin reds naming
`wallpinneddst_qa_test.go:247`, the test, and the fix.

The eleven-case control arm (`TestQAWallVenueDarkCensusCanStillSayNo`) plants
each shape: both directions, both operand orders, a negation, a conjunction, a
guard that `t.Fatalf`s instead of skipping, a skip guarded by something else, a
predicate read with no skip under it, an init-statement comparison, a skip
inside a nested `func` literal, and `Skipf`/`SkipNow`.

### The residual, said plainly

The key is `bwrapWallRoot` and not "anything the venue moves". The venue moves
`TMPDIR`, so a gate keyed on `os.TempDir()` or on where `t.TempDir()` lands is
in the same position and this pin does not read it. internal/posse's tests
carry 688 `t.Skip` call sites, 85 of them marked `NOTHING MEASURED`.

What the scan CAN say is that `bwrapWallRoot` is the only key spelled in the
package today — MEASURED 2026-11-22 over 764 test files, 14 guarded skips, all
of them `== ""`. What it cannot say is that no other key darkens a member, and
the thing that decides that is the RUN the bead asked for.

### The run, and it answers more than the bead expected

MEASURED 2026-11-22 on the FIXED tree (linux 7.2.15 x86_64, go1.27.3, 16
cores), arm 1 whole with `-v` in both shapes, each through
`scripts/test-times.sh` on the queued spelling so each took a box-wide suite
slot; `--- SKIP:` names extracted, sorted, `comm`-diffed:

| shape | abstentions | package |
|---|---|---|
| `TMPDIR=/var/tmp/posse-ddn56-venue` | 81 | `ok 271.2s` |
| `TMPDIR` unset | 81 | `ok 256.6s` |
| dark under the venue only | **0** | — |
| dark in the default shape only | **0** | — |

**The two SKIP sets are identical.** So the venue moves membership in NEITHER
direction on this tree, and the bead's own framing — "the venue moves
membership in both directions and nothing counts it either way" — holds only
for the one member this landing fixed. The reason is worth recording, because
it is not what the `== ""` count suggests: none of the fourteen is dark in the
default shape, because every one of them takes its home from a root that is
outside `/tmp` whatever `TMPDIR` is — `wallUngrantedDir` for eleven,
`newLandedCageFixture` for two, and a caller-passed home for the helper
`wrgHome`. They measure in both shapes already. The one site that did not was
the one on `t.TempDir()`.

The `wallvenueargv_qa_test.go:436` site the bead names as "the inverse — skips
in the DEFAULT shape and MEASURES under the venue" does not skip in either
shape here; its home is `newLandedCageFixture`'s. That reading describes a box
whose `$HOME` is under `/tmp`, which this one is not.

Both arms were `ok`, which is its own small result: arm 1 in this venue red
four live members when `make tmpdir-check` was priced (two held by
`ranger-base-shf97`, two by `ranger-base-mlw6q`), and with both of those landed
the venue is green.

A member that goes dark under the venue by some other key is found by that run
and by nothing in the scan — which is ADR 0098 D5's own answer for this class,
and the reason the run exists.

## What this did not do

`make tmpdir-check` still judges its run by `go test`'s exit status alone, and
that was a decision rather than an omission. The arm the bead points at —
`scripts/posse-wall-check`'s, which reads `go test -json` and answers exit 3
over a member that went dark — needs a MEMBER SET, and that venue's members
are the whole package: 688 skip sites, most of them legitimate (`runtime.GOOS`,
`bwrap` not on `PATH`, a nested wall). The only complete judge is the SKIP-set
diff above, and running it inside the door would double the door's 527s. ADR
0098 D5 priced and ruled that cost three hours before this bead was filed —
typed, not queued, with the macos legs as the venue of record — so the ruling
is the architecture lane's to revisit, not this bead's to re-open. Said on the
bead.

## Beside it

`internal/treepins/treewidedoor_qa_test.go`'s pin-count sentence read "one
hundred and sixty-three" against a Makefile holding 164 — `ranger-base-89fsg`,
a live main red that stopped `make test` before any arm for every seat on the
box. A new pkgcensus member cannot be added without touching that same
sentence, so this landing writes 165 and adds its own row to the enumeration
the count rests on. That is not a fix of `89fsg` taken quietly: it is the one
edit this change could not avoid, and both beads say so.
