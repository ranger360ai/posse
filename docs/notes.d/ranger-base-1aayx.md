# The fourteen macos arm-1 reds were twelve landings and one fixture root (ranger-base-1aayx)

2026-11-22, from a linux seat (Linux 7.2.15 x86_64, go1.27.3, 16 cores). No
darwin box was touched. The reds were READ off ci.yml's macos arm 1 and the
two that were left were **reproduced on this box by planting the shape**,
which is ranger-base-zbmxc's method one bead on.

## what the newest leg says

The bead was filed at 451a5aed (run 44984916356, job 137093271212) with
fourteen `--- FAIL` rows in internal/posse, and said in so many words to read
the newest completed leg before diagnosing anything. Read:

| commit | run / job | macos arm 1 |
|---|---|---|
| 451a5aed | 44984916356 / 137093271212 | internal/posse FAIL 713.808s, the fourteen |
| 2a50bcb5 | 44990274146 / 137109586827 | internal/posse FAIL 856.248s, **two** |

Twelve of the fourteen went green on the four landings in between, and the two
that stayed were joined by a thirteenth row that is new rather than one of the
fourteen — ranger-base-1b8yv's own L3 arm, written after the bead was filed:

```
--- FAIL: TestQAEveryWallResolvesTheStoreSpellingBeadsHomeAnswers (0.92s)
    beadshomespelling_qa_test.go:190: the render refuses this shape, so the L3
      arm measures nothing: bwrap cannot grant write on /private/var/folders/
      q2/…/T: it contains the per-seat wall root /private/var/folders/q2/…/T/
      TestQAEveryWallResolves…/001/.cache/posse-wall, where every bind source
      of every walled seat on this box lies …
--- FAIL: TestQABwrapRendersNoWriteBindInTheStoreRepoGitDir (0.51s)
    storerepogitwall_qa_test.go:183: the render refuses this shape, so nothing
      below is measured: <the same sentence>
```

The twelve were ranger-base-1b8yv's and ranger-base-zbmxc's: nine abstention
guards that compared `beadsHome(dir)` against a resolved store and read one
directory as two on every darwin box, and three fixture-side raw/resolved
rows. Neither bead's own description claimed the leg; this one reads it.

## the two that were left: one fixture root, on one host

`BwrapArgs` judges every write grant against the per-seat wall root composed
from `$HOME` and REFUSES a grant that CONTAINS it (`bwrapRefuseWallRootGrant`,
ADR 0100 D1/D2(a)). A blanket temp grant contains any fixture home cut under
that temp root, so a pin whose home is a plain `t.TempDir()` is asking the
render to refuse itself. The refusal stands down where the home is under
`/tmp` — granted unconditionally, so there is no seat there for a wall root to
protect — and `bwrapSeatTmp`'s private-/tmp mount stands down over the same
question.

So the whole of it is which temp root the host hands `t.TempDir()`:

| host | `t.TempDir()` | a blanket temp grant | under `/tmp` | the render |
|---|---|---|---|---|
| linux | `/tmp/…` | yes | **yes** | stands down, renders |
| darwin | `$TMPDIR` in `/var/folders/…` | yes | **no** | refuses the fixture's own home |

Both arms route their fixture through `wgFixtureAnyBox`, whose name is the
claim that broke: "a root EVERY box can use" was two different roots, and the
difference is invisible on the host this box is.

`wallStandDownDir` (wall_helpers_test.go) is the fix — `wallUngrantedDir`'s
opposite, a tolerant `os.MkdirTemp(bwrapTmpMount, "posse-standdown-")` root,
resolved, with the containment checked against the ANSWER and not the
spelling. `wgFixtureAnyBox` cuts both the fixture tree and `$HOME` under one
of those, so its 18 call sites across five files are on one box as well as on
one host.

**Neither alternative was taken, for ranger-base-zbmxc's reasons, said again
here because this is the second bead to reach them**: widening the stand-down
to every blanket temp grant weakens a live refusal on linux — `bwrapSeatTmp`'s
stand-down is specifically about the directory the mount REPLACES, and a
`$TMPDIR` under a real home is not that — and skipping the render off linux
retires the pin on the only host that runs it.

## MEASURED

Three shapes on this box, the first two ranger-base-zbmxc's table:

```
shape 1  TMPDIR unset        /tmp                     the linux leg
shape 2  TMPDIR=/var/tmp/…   granted, not /tmp        darwin's $TMPDIR
shape 3  TMPDIR=<a symlink>  granted, behind a link   darwin's /var -> /private/var
```

Pre-fix, the two arms by `git apply -R` of this bead's own diff:

```
shape 1  ok   0.401s                       (the green linux leg, as it has been)
shape 2  FAIL beadshomespelling_qa_test.go:196 + storerepogitwall_qa_test.go:183
shape 3  FAIL the same two lines
```

and the shape-2 message is the macos log's, sentence for sentence, with
`/var/tmp/posse-1aayx-tmpdir` where `/private/var/folders/q2/…/T` stands.

Post-fix, every arm of the five files that use `wgFixtureAnyBox`
(beadshomespelling, seatbeltworktreegit, recordreachgrant, seatbeltdenygitgrant,
storerepogitwall — 32 top-level tests) green in all three shapes: 5.755s,
2.661s, 2.990s. And the bead's own fourteen plus the thirteenth row, 15 of 15
PASS in each of the three.

`internal/treepins`' temp-root census reads **21 Go roots** over 1172 .go
files, all inside the `posse-` namespace, against a `trGoRoots` of 19 — so
that line is set to 21 here. One of the two is this bead's `posse-standdown-`
and the other was already live: ranger-base-zbmxc's `posse-bwsd-`, added
yesterday above a floor of 19 and passing in silence, which is exactly the
one-directional drift that file's own comment predicts of itself.

## Not closed here

- the macos leg's own verdict: nothing here ran sandbox-exec and nothing here
  claims a macos pass. What is claimed is that the two refusals are the
  fixture root, reproduced and fixed under the shape that produces them.
- ranger-base-jhmm7 — the CLASS has now bitten three fixtures in two days and
  is caught by nothing on this box.
- ranger-base-njj5z — macos arm **2** carries one red at the same commit
  (`TestFoldWritesNothingIntoAPlantersPipe`, "file type does not support
  deadline"), which is not this bead's arm and not this family.
