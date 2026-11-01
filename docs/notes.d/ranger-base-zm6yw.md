# The climb spelled past a Join's first element, and the HOME nobody can opt out of (ranger-base-zm6yw)

The code half of `ranger-base-wq349`'s two live findings (holden's verify of
four closes). Both were the same shape as the bead they rode beside — the fix
closed the spelling the bead named and the class had one more spelling in it —
and they close differently: FINDING 2 is a rule gap and is fixed, FINDING 1 is
a rule gap **wrapped around a jurisdiction decision**, so the gap is fixed and
the decision is filed as `ranger-base-er0gn` (`-l architecture`).

Environment for every measurement below: darwin 25.6.0, bash 3.2, go 1.26.10,
2026-11-01, worktree `gwart-posse-ranger-base-zm6yw`, reading at main
`5620d3b9`. No credential file was read and nothing ran as a privileged user.

## 1. The sweep's relative HOME (FINDING 2)

`ranger-base-l5v9k` closed the two config-dir variables: a relative
`CLAUDE_CONFIG_DIR` or `CLAUDE_SECURESTORAGE_CONFIG_DIR` is refused with exit 2
and the sweep says it measured nothing. `$HOME` it did not close. The `dirs`
array in `scripts/verify-credential-paths.sh` is seeded with `"$HOME/.claude"`
**before `add_dir` is ever called**, and the guard above it asked only whether
`HOME` was non-empty — never whether it was absolute. So the one directory the
sweep always scans, the one an operator cannot opt out of, and the arm the
script's own head comment says covers a present-but-empty
`CLAUDE_SECURESTORAGE_CONFIG_DIR`, was still resolved against the sweep's own
cwd.

MEASURED before the fix, `HOME=relhome` with a credential file planted in
`A/relhome/.claude` and both config-dir variables set-and-EMPTY so the arm is
HOME's alone:

| cwd | output | exit |
|---|---|---|
| `A` (holds the file) | `FINDING  relhome/.claude/<the matched file>` | 1 |
| `B` (does not) | `clean    relhome/.claude` | 0 |
| `HOME=.` | `absent   ./.claude` → `nothing measured, not a pass` | 2 |

(The matched name is labelled rather than spelled, per ADR 0024 D3: the glob
is in `scripts/verify-credential-paths.sh` and the measurement itself is on
`ranger-base-wq349`.)

Same box, same environment, same store of record, opposite verdicts — a
detective control saying clean about a directory it never looked in, and a
printed FINDING line naming a relative path that means nothing to whoever reads
the report. Both of `ranger-base-57045`'s readings.

**THE FIX** is `add_dir`'s own rule one variable over: a non-absolute `HOME`
prints `HOME=<value> is relative; … nothing measured, not a pass` and exits 2
before anything is scanned. It is **not** resolved against `$PWD`, for
l5v9k's reason — posse made the value absolute against the LAUNCHER's cwd, and
that cwd cannot be recovered after the fact, so resolving here would swap one
wrong guess for another.

`ranger-base-wq349` shipped the failing test for this
(`TestQACredentialPathsRelativeHomeIsStillCwdDependent`, committed at
`7ad87c94`): a LIVE-DEFECT pin, green while the hole was open, whose third
assertion — that the two cwds DISAGREE — no fix can satisfy. The fix reddened
it on all three assertions, as that bead measured it would, and it is rewritten
here as `TestQACredentialPathsRefusesARelativeHome` plus
`TestQACredentialPathsStillScansAnAbsoluteHome` (the other side, so the refusal
arm cannot be satisfied by a script that refuses everything).

MUTANTS, both killed, over `relhome`, `./relhome`, `relhome/` and `.`:

- **drop the refusal** (restore the hole) — 26 assertion failures; every one of
  the four values disagrees across the two cwds.
- **resolve against `$PWD`** instead of refusing — same four disagreements
  (`1` vs `0` for the three relative dirs, `1` vs `2` for `.`). This is the
  wrong fix wq349 named, and the `codes[0] != codes[1]` assertion is what
  catches it.

## 2. The climb spelled past a Join's first element (FINDING 1)

A reading of the tree whose root is **a bare root with a subdirectory joined
onto it** was in neither half of the tree-wide class derived by
`internal/treepins/treewidedoor_qa_test.go`. MEASURED through `twdAnalyze`
itself over probes that each read every file under `internal/` and differ ONLY
in how the root arrives:

| probe | root | `climbEnum` | `handRolledTreeWalk` | scored by |
|---|---|---|---|---|
| P3 | `dir := "../../internal"` | true | false | the fence |
| P4 | `filepath.Join("..", "..")`, walked | false | true | the hidden arm |
| P5 | `filepath.Join(wd, "..", "..")`, walked | false | true | the hidden arm |
| P1 | `filepath.Join(wd, "..", "..", "internal")` | **false** | **false** | **nothing** |
| P2 | `repo := filepath.Join("..", ".."); walk(repo/internal)` | **false** | **false** | **nothing** |

The fence (`twdClimbRoot`, `twdClimbSet.climbs`) reads a `filepath.Join`'s
FIRST element only — which in P1 is the cwd — and the hidden arm
(`twdRootExprs` via `twdJoinsTwoDotDots`) fires only when the walk's root IS
the bare root, because it wants the two `..` at the END of the argument list.
So P1 and P2 got no door, no census named them, and nothing said so.

`ranger-base-56eln`'s residual paragraph, live in `twdClimbSet.bind` until this
bead, asserted that the remaining shape "is the hidden arm's shape and is
scored by its dataflow". **It is not** — that is the sentence a later reader
trusts instead of re-censusing, and the table above is what it is worth. The
paragraph is corrected in place and now names its own measurement.

### What is fixed: P1, which needs no jurisdiction call

`twdJoinClimbsToASubpath` is the arm: a `filepath.Join` whose argument list
spells `"..", ".."` past its first element and then names at least one further
segment. Wired into `twdClimbRoot`'s `Join` case and into
`twdClimbSet.climbs`, which now falls through to it instead of returning on
the first element.

The jurisdiction line is `bind`'s, unchanged, which is why something must
FOLLOW the pair: `filepath.Join(wd, "..", "..")` evaluates to the repo root and
the root arms already score it (P4 and P5 above). After the change P1 and the
live instance score `climbEnum`, P4 and P5 still score `handRolledTreeWalk`
alone, and nothing double-fires.

**ONE live instance, the one wq349 named**, and the register found exactly it
and nothing else: `internal/posse/bobtripwire_qa_test.go`
`TestQABobFixturesAreStillUnknownToHerdr` — ADR 0060 D2's tripwire, which
enumerates `etc/herdr/agent-detection/upstream/bob` and goes red the day herdr
learns the `bob` kind. It took `os.Getwd()` and climbed two levels, was in no
`QA_*_PINS` variable, and its subject is what an unrelated bead puts in that
directory. Remediated to `qibRepoRoot(t)` — the house fix, and the same one
`ranger-base-8yv7z` applied to the known-screen reader one directory over —
and doored in `$(QA_SEED_PINS)`, whose own comment already says to type it when
you add a detection fixture. Verified: `make seed-check` runs it and its six
pane fixtures all execute (not skipped).

### What is filed: P2, which is a ruling and not a gap — ranger-base-er0gn

The same reading spelled through a bare-root **identifier** is still scored by
nothing, deliberately. `detectionRig`
(`internal/posse/verifydetection_qa_test.go:32`) is exactly that shape — three
`os.ReadDir`s, at lines 56, 71 and 80, all named by the new pin's live census —
and the house has ruled two incompatible ways on the radius:

- `twdRootExprs`' head comment rules detectionRig OUT of the class **by
  name**: "only a walk rooted AT the root is tree-wide" (`ranger-base-sx2dq`).
- `TestQAKnownScreenReaderNamesEveryFixtureExactly`
  (`d3knownscreen_qa_test.go:187`) enumerates
  `etc/herdr/agent-detection/testdata` — the SIBLING directory, the same
  radius — and was ruled IN (`ranger-base-8yv7z`, doored in
  `$(QA_SEED_PINS)`).
- `ranger-base-m94g2` / `ranger-base-na4rm` then made a DISCOVERED FILE SET
  the criterion outright: fifteen one-package censuses are members for reading
  `.` relatively, which is one directory and no climb at all. By that
  criterion detectionRig's callers are members.

Those cannot all stand, and a fence widened to this shape fires on detectionRig
on its first run — reversing a documented ruling and moving five call sites'
worth of tests into the class. That is richard's, so it is
`ranger-base-er0gn`, which carries both options and prices each.

What this bead ships for it instead is a LIVE-DEFECT pin in the house shape,
`TestQATheClimbFenceStillHasItsBareRootJoinHole` (doored in
`$(QA_REGISTER_PINS)`): three fixtures asserting all four scores are false,
plus `twdBareRootJoinReadings`, the detector, asserted to find at least one
LIVE instance so the hole cannot be confused with a broken detector. It ships
GREEN; when er0gn is decided the pin goes RED and its inversion message says
what to do — promote the detector into `twdClimbSet` beside
`twdJoinClimbsToASubpath`, remediate what it names, and rewrite the body as
the fence's census, which must then be empty.

The detector sits between the two rules it mediates on purpose — the bare root
through `twdRootExprs`, the subpath through `twdClimbNamesASubpath` — so it
cannot be wider or narrower than the fence and the hidden arm it is measuring
the gap between.

### The register's own bookkeeping

Two pins joined the class, so the head comment's enumeration and its count
sentence moved with them: one hundred and twelve pins → one hundred and
fourteen, seventeen doors unchanged.
`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` is what required both
edits and what caught the first one being left short.
