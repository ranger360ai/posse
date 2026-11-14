# `make test`'s verdict moved with TMPDIR, for two unrelated reasons (ranger-base-6ca29)

Found while building `posse upgrade` (ranger-base-zhw0h, ADR 0092 D3.2), whose
guardrail 2 IS a `make test` verdict — so a verdict that moves with where the
suite's temp files live is a guardrail that can refuse a good commit, or be
re-run until it passes. docs/notes.d/ranger-base-zhw0h.md carries the matrix
that filed this; this fragment is what the two causes turned out to be and
what was done about each.

Both are TEST defects. No production code changed, and neither cause was a
product bug: in each case the code under test was doing its job and the
assertion was asking the wrong question.

## Class 1 — a raw temp path asserted against output posse abbreviates

`posse` renders a path under `$HOME` in its tilde form before printing it
(`AbbrevHome`, internal/posse/app.go). `cmd/posse`'s tests build their fixtures
with `t.TempDir()`, so a fixture path is wherever the box's TMPDIR points — and
most of this package's children inherit the operator's REAL `HOME` deliberately
(`main_test.go`'s `readyEnv`, `statusEnv`, so that the child abbreviates paths
against the same `$HOME` this process does; configdirfence_test.go's header
says why).

On a box whose TMPDIR is under the home, those two facts meet: the output names
the fixture in its tilde form, the test looks for the raw spelling, and the
test reds while the product is right. Pointing TMPDIR at the home is the
obvious move when `/tmp` is a full tmpfs, which is how this was found.

The three reds the original matrix surfaced are all one shape:

| test | the assertion |
|---|---|
| `TestQAStatusPrintsTheSecondStoreLine` | `strings.Contains(line, filepath.Join(repo, ".beads"))` |
| `TestClaudeConfigDirIsFenced` | `strings.Contains(out, fence)`, and the planted dir |
| `TestCutDestinationKeysChangeNothing` | `strings.ReplaceAll(out, home, "<home>")` — the replace misses, so three scratch homes differ by the path alone and the comparison the test exists for reds |

The fix is one helper in `cmd/posse/main_test.go` and four call sites:

    func posseRendered(p string) string { return posse.AbbrevHome(p) }

Asserting through it is not a weaker claim. Where TMPDIR is outside the home
`AbbrevHome` is the identity and the assertion is the one it always was; where
it is inside, the claim becomes "the path as posse renders it", which is what
the reader of that line is being promised either way. The wall-witness pin one
package over already spells its own assertion this way
(`AbbrevHome(tc.cwd)`, internal/posse/wallwitness_qa_test.go), so the
convention existed — it just had no name in `cmd/posse`.

**Where this class can and cannot occur**, since the original bead called the
real count unmeasured. It needs a test binary that keeps the operator's `HOME`.
`internal/posse` replaces `HOME` process-wide with a per-tree temp home
(`herdr_test.go`'s `TestMain`), and a `t.TempDir()` fixture is then a SIBLING of
that home rather than under it — so `AbbrevHome` is the identity there whatever
the box's TMPDIR is, and the class cannot arise. `cmd/posse` and
`internal/treepins` keep the real home (`internal/treepins`' `TestMain` fences
the CLI config dirs and not `HOME`), so those are the two packages where it
can.

## Class 2 — a fixture whose premise was derived from TMPDIR instead of named

`TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate` (ADR 0086 D5) is
half of a pair: ONE fixture body under TWO roots, where the wall must hold
under one and must NOT hold under the other. The "must hold" root is
`wallUngrantedDir` and names `/var/tmp` by hand. The "must not hold" root was
`wwTempRoot` → `gitTempDir` → `t.TempDir()`, on the stated premise that
"`SeatbeltWritable` grants `/tmp` and TMPDIR unconditionally … so the grant is
TMPDIR's own".

The second half of that sentence is wrong at the bwrap mechanism. The
launcher's TMPDIR is granted by `SeatbeltWritable` wherever it lies and is then
taken straight back out: `bwrapSeatTmp` (internal/posse/wallseatcache.go)
returns `""` — no private tmp, so the blanket `/tmp` grant survives and covers
the fixture tree — EXACTLY when the home or the cwd is under `/tmp`. Anywhere
else the render binds the seat's private tmp over `/tmp` and
`bwrapDropTmpGrants` drops BOTH `/tmp` and the launcher's TMPDIR from the write
set, because one temp directory shared by every seat on the box is what that
mount exists to keep out.

So the premise of this arm is a PLACEMENT — a root under `/tmp` — and it was
being inherited from wherever the box points TMPDIR.

**MEASURED 2026-11-13** (linux 7.2.15 x86_64, this box, bwrap), one `-run`
filter, through the production render over the same whole-tree-deny PID:

| TMPDIR | rendered argv | witness | the pin |
|---|---|---|---|
| unset (fixture under `/tmp`) | no private-tmp bind | the create LANDS | PASS |
| `/var/tmp/x` | `--bind <seat>/tmp /tmp`, `--setenv TMPDIR /tmp`, and NO bind of `/var/tmp/x` at all | the create is REFUSED | red in ten places quoting ADR 0086 D5 |

Same production code, same PID. The fix is to name the root the way its twin
already does (`const wwTmpRoot = "/tmp"`, `os.MkdirTemp` under it, `posse-`
prefixed because the parent is a shared temp directory) and to skip
`NOTHING MEASURED` on a host that has no `/tmp`, which is `wallUngrantedDir`'s
own shape one root over.

**ADR 0086 D5 itself needs no change**, and the original bead's reading that
"D5's pin is therefore making a claim about `/tmp` and saying nothing about
anywhere else" is the wrong way round: D5's POSITIVE arm — the wall must hold,
the one that grades the design — runs at `/var/tmp`, outside every blanket temp
grant. `/tmp` is only where its negative arm goes, and that arm's whole subject
is a wall that does not hold.

### The premise now reads as a premise

The arm judges a PARITY: six assertions, every one of them downstream of a
create that must land. A fixture the wall really walls therefore red all six at
once, each with ADR 0086 D5 in the message, over production code that had
refused a write exactly as the PID told it to — and reading those ten lines is
what cost the time, not fixing the root.

`wwRequireTheWallAllowsTheCreate` takes the witness over cwd alone and first,
and its failure says the fixture moved rather than that the design is broken.
It is the witness and not a containment test over the writable set, for the
same reason the arms use a real wall rather than a stub one: the bwrap render
substitutes mounts AFTER the set is computed, so "a grant reaches the tree" and
"a seat can write the tree" are two different claims and this is the one the
rows rest on.

Probed by planting the pre-fix placement (`wwTmpRoot` = `/var/tmp`): ten errors
become two, one per subtest, each naming the placement, `bwrapSeatTmp`,
`bwrapDropTmpGrants` and this bead.

## The temp-root floor was already one under the live count

`internal/treepins/temproot_qa_test.go`'s roots floor is `gos < trGoRoots`, and
`trGoRoots` is meant to BE the live count (ranger-base-sxez7: a floor under the
live count let seven roots leave the census in silence). It is exact in one
direction only — a landing that ADDS a root passes it — and one had.
**MEASURED 2026-11-13**: the census read 16 Go roots against a `trGoRoots` of
15 with this bead's own change backed out, and 17 with `wwTempRoot`'s `/tmp`
root. Set to 17, with the asymmetry written beside it, since the floor cannot
ask.

## The third cause, which is nobody's code

This box's `/tmp` is `tmpfs … usrquota` and was at the user's limit, so a full
run there answered `disk quota exceeded` from `git`, from the linker and from
the tests alike — ~30 failures, none naming the cause as a class. That is what
made moving TMPDIR the obvious response, which walked straight into the two
above. Unfixed and unfixable here: TMPDIR is the operator's dial and its
default is right. What the two fixes buy is that moving it is now a safe thing
to do — `/var/tmp` and a directory under the home both give a green suite.
