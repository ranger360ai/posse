# The carve pins' prep splice position gets its witness (ranger-base-dv1ln)

`ranger-base-zrq1x` verified four closes and found one live thing, in
`ranger-base-a59w7`'s work: **a missing pin rather than a wrong one.** Nothing
in the shipped render needs to change — the splice is already in the right
place — so this is one test arm and no production diff.

MEASURED 2026-11-22, linux 7.2.15-1-omarchy, go1.27.3, 16 cores, this tree at
a7621641 unless another base is named. The bead's own repro was MEASURED
2026-11-21 at c7573efc and reproduces unchanged here.

## The half that had no witness

`BwrapArgs` step 2d marks two positions — one in the argv and one in the prep
list — and step 5d splices the carve-out's pins back into both
(`internal/posse/bwrap.go`). 2d's head comment argues the ordering for each.
The ARGV half is held three ways over: the two render arms in
`wallcarvepins_qa_test.go` assert grant < pin < deny and pin < mask,
pin < private, and `ranger-base-a59w7`'s own mutation run killed four mutants
over it.

The PREP half was asserted in that prose, realized in that code, and graded by
nobody. `PrepMaskPoint` and `PrepTouch` open their name with
`O_CREATE|O_EXCL` and create no parents, unlike `PrepSeed`, so a pin `mkdir -p`
that lands after them is a launcher that opens a file inside a directory
nothing has made: ENOENT, outside the namespace, before `bwrap` is exec'd, and
the seat does not launch.

## What it is worth: the mutants

`prep = slices.Insert(prep, carvePrepAt, pinPrep...)` → `append`, which is
"splice at 5d's own position" and the same list either way. The mutant needs
`_ = carvePrepAt` beside it or the package does not compile, which is why the
first run of this battery measured a build failure and nothing else.

| arm | `-run` set | verdict |
|---|---|---|
| baseline | the six names ranger-base-dv1ln lists | ok, 12.750s |
| splice → `append` | the same six | **FAIL, and the only red is this bead's own arm**, 25.049s |
| splice → `append` | this arm alone | FAIL, 0.012s |
| splice → index 0 | the same six | ok, 13.297s |
| splice → index 0 | this arm alone | ok, 0.014s |
| restored | this arm alone | ok |

The six are `BwrapArgsGitConfigRefusal`,
`BwrapArgsSkipsAMaskOverAWriteGrantedCredentialStore`, `TestQAWallCarvePins`,
`TestQAWallPinned`, `TestWallWatchEveryRenderedMountIsAWallOrANamedOpening`
and `TestQAWallMountGrammar` — the set `ranger-base-zrq1x` ran when it found
0 newly red. Row 2 is the whole finding: under the mutant that set now reds,
and the one name in it is the arm this bead added. `internal/posse/bwrap.go`
is md5-identical to HEAD after the battery and `git status` is clean.

**The constraint is one-sided, and the second direction says so.** Splicing
the pins at index 0 instead — ahead of step 2's own grant mkdirs — is green
everywhere, because `mkdir -p` creates parents: a pin mkdir can be too late
and cannot be too early. So the pin asserts `pin ≤ step` and not an exact
index, and there is no second arm to write.

## The fixture, and why one of it

ranger-base-dv1ln's repro: a write grant with `Deny`+`DenyFile`+`Mask` on
`<g>/a/b/cache.json`, `<g>/a` ABSENT, and a ref keep file at `<g>/a/keep`. The
shipped render's prep is

```
mkdir -p <g>/a
mkdir -p <g>/a/b
touch <g>/a/keep
mask point <g>/a/b/cache.json
```

Both kinds in ONE fixture because one splice position answers for both: the
mask point comes from step 5's masked-deny arm and the keep file from step 2b,
which is the one producer that lands BETWEEN 2d's mark and step 5. A reader
who wonders whether the mark is early enough for 2b has the answer in the
fixture instead of in two line numbers. The arm runs `runBwrapPrep` over it
and then runs the same render's steps with the mkdirs appended, over a fresh
fixture, where the launcher's prep refuses with
`open <g>/a/keep: no such file or directory`.

## Scope

INERT in every shipped render today: `TestQAWallCarvePinsCensusOverEveryRender`
logs 0 carve-out pins over all five argvs this package renders (claude, codex,
grok, judge, venue), so no production prep list carries a pin step at all. The
cost arrives with the first shipped render that grows a carve pin whose
directory is ABSENT and has a mask point or a keep file under it, and its
direction is fail-closed — the launcher's prep dies before the wall, the same
direction as the refused render `ranger-base-a59w7` fixed.
