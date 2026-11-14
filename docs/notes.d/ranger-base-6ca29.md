# `make test`'s verdict moved with TMPDIR, for two unrelated reasons (ranger-base-6ca29)

Found while building `posse upgrade` (ranger-base-zhw0h, ADR 0092 D3.2), whose
guardrail 2 IS a `make test` verdict — so a verdict that moves with where the
suite's temp files live is a guardrail that can refuse a good commit, or be
re-run until it passes. docs/notes.d/ranger-base-zhw0h.md carries the matrix
that filed this; this fragment is what the two causes turned out to be and
what was done about each.

**THREE causes, not the two the bead's title names.** The third was found by
the full run the bead asked for and is in no sense a variant of the other two.
All three are TEST defects: no production code changed, and in every case the
code under test was doing its job while the assertion asked the wrong
question.

### A note on how the count was reached, because it bit twice

`make test` runs four commands and `make` aborts a recipe on its first
non-zero exit. So a measurement run stops at the first arm that reds, and the
arms after it do not run at all:

1. first run, `TMPDIR=$HOME/tmp` — red in arm 1 (`./...`), on
   `internal/treepins`. Arms 2 and 3 never ran.
2. second run, after that was fixed — arm 1 green, and red in **arm 3**, on a
   cause unrelated to anything above.

Between those two runs this fragment claimed the class was closed at four
sites, on the strength of run 1. It was not; run 1 had only ever measured a
third of the suite. **A red `make test` is a lower bound on what is wrong with
the tree, never a census of it** — and the arms are the units it is a lower
bound over. Anyone using a full run to enumerate a class here needs it GREEN
before the count means anything, or needs to run the arms independently
(`make test-arm1`, `-arm2`, `-arm3`) and read all three.

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

### The count, which the bead called unmeasured

**MEASURED 2026-11-13** (linux 7.2.15 x86_64, go1.27.3, this box): `make test`
with `TMPDIR=$HOME/tmp`, after the three reds the original matrix surfaced were
fixed, left one more of THIS class — the same pin one package over,
`internal/treepins`' own `TestClaudeConfigDirIsFenced`. So class 1 is FOUR
sites in two packages. (The run also reddened arm 3 on a cause that is not
this class at all; that is class 3 below.)

Reading would not have found the fourth. The three in the matrix were a `-run`
filter per test over a list somebody had already noticed; the fourth is a
second copy of one of them, in the other package, and nothing in that file
says so except a prose cross-reference. The full run under the moved TMPDIR is
what produced it, which is the whole argument for having paid for one.

| test | package | the assertion |
|---|---|---|
| `TestQAStatusPrintsTheSecondStoreLine` | `cmd/posse` | `strings.Contains(line, filepath.Join(repo, ".beads"))` |
| `TestClaudeConfigDirIsFenced` | `cmd/posse` | `strings.Contains(out, fence)`, and the planted dir |
| `TestCutDestinationKeysChangeNothing` | `cmd/posse` | `strings.ReplaceAll(out, home, "<home>")` — the replace misses, so three scratch homes differ by the path alone and the comparison the test exists for reds |
| `TestClaudeConfigDirIsFenced` | `internal/treepins` | the same two sites as its `cmd/posse` twin — found by the full run above, not by reading |

The fix is one helper and six call sites — one copy of the helper per package,
because a test helper does not cross a package:

    func posseRendered(p string) string { return posse.AbbrevHome(p) }

Asserting through it is not a weaker claim. Where TMPDIR is outside the home
`AbbrevHome` is the identity and the assertion is the one it always was; where
it is inside, the claim becomes "the path as posse renders it", which is what
the reader of that line is being promised either way. The wall-witness pin one
package over already spells its own assertion this way
(`AbbrevHome(tc.cwd)`, internal/posse/wallwitness_qa_test.go), so the
convention existed — it just had no name in `cmd/posse`.

**Where this class can and cannot occur**, which is why four is the whole of
it. The class needs a test binary that keeps the operator's `HOME`.
`internal/posse` replaces `HOME` process-wide with a per-tree temp home
(`herdr_test.go`'s `TestMain`), and a `t.TempDir()` fixture is then a SIBLING of
that home rather than under it — so `AbbrevHome` is the identity there whatever
the box's TMPDIR is, and the class cannot arise however many paths that package
asserts. `cmd/posse` and `internal/treepins` keep the real home
(`internal/treepins`' `TestMain` fences the CLI config dirs and not `HOME`), so
those are the two packages where it can — and the measured run found reds in
both and nowhere else, which is the prediction and the measurement agreeing
rather than either on its own.

A test added tomorrow in either of those two packages can reopen the class, and
nothing mechanical stops it: "a raw path compared against rendered output" is
not decidable from the source, since whether the output went through
`AbbrevHome` is a property of the producer. What the two `posseRendered`
helpers buy is that the convention has a name and a reason at the point where
the next author is already looking.

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

## Class 3 — a pin that read the typed line, 64 bytes from the cliff

`TestSeatbeltProfileAndLaunch` (internal/posse, arm 3) asserted that a
`cage: seatbelt` grok launch carries its bwrap wrap, by looking for
`GATES <wrap>grok` in the fake herdr's call log. Under `TMPDIR=$HOME/tmp` it
red with `bwrap wrap missing` over a launch that was correctly walled.

Nothing about the wall moved. A launch line over `PaneLineMax` (1023 bytes, the
tty's MAX_CANON — paneline.go) is not typed at all: it is written to
`state/launch/<session>.sh` and the pane types `. <script>` instead. So the
wrap was in the script, and the pin was reading the one place it wasn't.

**MEASURED 2026-11-13**, this test's grok line:

| TMPDIR | line | |
|---|---|---|
| unset | 959 bytes | typed — **64 bytes** of headroom |
| `$HOME/tmp/…` | 1325 bytes | spilled, and the pin red |

366 bytes for a TMPDIR 17 characters longer than `/tmp`, because the gate
prefix alone spells the gates dir four times and the rendered grok args spell
the fixture home a dozen more.

So this pin was passing on 64 bytes of luck. `ranger-base-rq83c` walked six
fixtures over this same cliff when ~110 bytes of an unrelated credential-dir
pin were added — this one had less room than any of those six, and the thing
that pushed it over was not a code change at all.

The fix is the helper the tree already has: `launchLog` reads both places, and
its own comment has carried this warning since `ranger-base-90y3c`
("the pin died on `no such file or directory` while the launch it was asking
about was correct"). This site had never adopted it.

The relaunch assertion needed a different fix. It was
`strings.Count(log, wrap+"grok") != 2` — launch plus relaunch — and that is two
length-dependent claims rather than one: the spilled script is REWRITTEN at
every launch, so once either line is over the cliff the log holds
`. '<script>'` twice and the wrapped line once, and the count can never be 2
again however right the relaunch is. It now reads the relaunch's own resolved
line, which is both length-independent and the stronger claim, with the
"a line was typed at all" half the count also bought kept explicitly as a
count of panes.

**This class is not closed and cannot be closed by this bead.** Any pin that
reads a launch line through `calls()` rather than `launchLog` is one long path
away from the same red, the margin is unknowable without measuring each one,
and `PaneLineMax` is a real constant that real launches cross every day
(paneline.go: every persona's line measured 1867-2058 bytes, so spilling is
the NORMAL case in production and the typed form is the exception these
fixtures happen to hit). A census of `calls(` sites that assert on a launch
line, against `launchLog`, is the mechanical door — it is decidable, unlike
class 1's — and it is filed as **ranger-base-zb476** rather than built here,
because it is a pin with its own subject, its own exemption decision and its
own door, not a line of this fix. The count of sites in the class is
UNMEASURED.

## The temp-root floor was already one under the live count

`internal/treepins/temproot_qa_test.go`'s roots floor is `gos < trGoRoots`, and
`trGoRoots` is meant to BE the live count (ranger-base-sxez7: a floor under the
live count let seven roots leave the census in silence). It is exact in one
direction only — a landing that ADDS a root passes it — and one had.
**MEASURED 2026-11-13**: the census read 16 Go roots against a `trGoRoots` of
15 with this bead's own change backed out, and 17 with `wwTempRoot`'s `/tmp`
root. Set to 17, with the asymmetry written beside it, since the floor cannot
ask.

## The environmental cause, which is nobody's code

This box's `/tmp` is `tmpfs … usrquota` and was at the user's limit, so a full
run there answered `disk quota exceeded` from `git`, from the linker and from
the tests alike — ~30 failures, none naming the cause as a class. That is what
made moving TMPDIR the obvious response, which walked straight into the three
above. Unfixed and unfixable here: TMPDIR is the operator's dial and its
default is right.

What the three fixes buy is that moving it is a safe thing to do.
**MEASURED 2026-11-13**, full `make test` on this box at HEAD: green with
TMPDIR at its default, and green with TMPDIR under the home. `/var/tmp` was
measured per-test rather than by a full run — every test named in this
fragment passes there — so that column is the weaker claim of the three and
is written as one.

## Not closed here

- ranger-base-zb476
