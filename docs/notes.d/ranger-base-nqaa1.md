# A box-wide rename reported by a per-instance reader, and a class token nothing read (ranger-base-nqaa1)

Two findings from holden's verify of ranger-base-rrgnc's close
(ranger-base-q87lp). Both landed in that close; neither was a live exposure on
the day it was found, and both were cheap to pin.

## Finding 1 — the reading is one instance's and did not say so

`posse upgrade` renames ONE file, `~/.local/bin/posse`, and that file is
box-wide: every RHQ_HOME's `dispatch --watch` loop execs it, and rename(2)
leaves each of them on the old inode. Every reader of that fact is
per-instance. `ReadWatchImage(a *App)` asks one home's watch lock whether a
loop is running and reads one home's `dispatch-watch.pid` for its pid;
ShopCheck's G18 arm runs per instance; the verb runs in the delegate's
instance only. ADR 0092 D6 says the remedy is "restart the dispatch loopS" —
plural — and both renderings named one loop, with nothing saying which.

MEASURED 2026-11-21, this box, linux 7.2.15-1-omarchy, at tree 3c16c00c (which
carries ranger-base-rrgnc's commit 02412ade):

    pid 2066610  RHQ_HOME=/home/posse/homes/ranger  --watch 5m -n 6 --resume
    pid 1991667  RHQ_HOME=/home/posse/homes/weller  --watch 3m -n 2 --resume
    both /proc/<pid>/exe -> /home/posse/.local/bin/posse, dev:ino 54:12591441

Two instances, two loops, one binary. Both legitimate — each home's own pid
record names its own pid, so the per-home lock invariant holds. Neither was
stale when it was read, so **G18 was correctly silent for both**: coverage,
not exposure. What made it worth fixing is that nothing else would have said
it — the weller instance has no pulse state at all (no `pulse.feed`, no
`pulse.yaml`, no `pulse.wait.lock`), so no tick there ever evaluates G18, and
its `state/launch` is empty, so that loop has rendered no seat. The hole is
true of that instance's future launches and of nothing that has happened.

**The fix is the sentence, not the widening.** `WatchImage.Home` carries whose
loop was read and `watchScopeSentence` (watchimage.go) says it, on every
branch of both renderings:

    scope: one instance's loop — /home/posse/homes/ranger's, the pid its own
    state/dispatch-watch.pid names — while the binary an install renames is
    box-wide, so ADR 0092 D6's "restart the dispatch loops" reaches sibling
    instances this reading did not look at (ranger-base-nqaa1)

The stale branches were the easy half: a loop already being told to restart
only gets sharper. The two that needed it are `upgradeWatchLine`'s others —
**"already runs"** and **"no running dispatch --watch loop of this home could
be read"** — because those are the lines an operator finishes believing the
BOX is covered: the first sounds like an all-clear, the second like there is
nothing to restart. An abstention names its instance too, which is the one
place where "this home" was all the reader had.

Widening the reading is NOT done here and is **ranger-base-is1qp**: it needs a
notion of "the instances on this box", and no shipped code here has one —
nothing enumerates RHQ_HOMEs, and
`grep -rn 'HomesRoot\|"homes"\|homesDir' internal cmd`, which was EMPTY when
this finding was filed, now returns exactly one line: the two-instance fixture
below. A reader re-running it should know that hit is a test.
That bead carries the measurements and the shape of the
decision, including the cheaper half (the upgrade verb alone reads box-wide;
G18 stays per-instance, as every other G-row is). Restarting any loop stays
the operator's or the coordinator's (guardrail 3) either way.

ADR 0092 D6 carries the residual, so the next reader of the amendment meets
it where the plural is written.

## Finding 2 — `"G18": upgradeInstallSubject` was asserted by nothing

A surviving mutant, which is the whole of it. MEASURED 2026-11-21 at tree
3c16c00c by the verify: that one line edited to `upgradeBinaryRendered`
exercised 125 tests under
`go test -run '(Upgrade|upgradeCond|Install|Condition|G18)' ./internal/posse`
and red NONE. G18 appears after EVERY install with a live dispatch loop, by
construction — the rename creates it — so the wrong class there does not fail
occasionally: it rolls back every good install on this box, every time, and
the next reader's first hypothesis is that the new binary is broken.

What existed was the completeness pin
(`TestQAEveryStatusConditionCodeHasAnUpgradeClass`), which requires that a row
HAS a class and that no class names a dead row. It says nothing about WHICH,
and the values are what the rollback is computed over.

**The fix is the general shape rather than the one line the bead priced**,
because one line is one row somebody remembered: `upgradeClassValuePin`
(upgradeverify_test.go) spells every row's expected class a second time and
`TestUpgradeEveryConditionClassIsPinnedByValue` compares them. The copy cannot
drift, because the check is bidirectional — a classed row with no pin fails,
and a pinned row the table does not name fails — so a G-row added tomorrow
arrives with a value something asserts instead of a value somebody has to
remember to assert. The four reasoned spot checks that used to sit in
`TestUpgradeUnknownConditionCodeIsTreatedAsBinaryRendered` moved into that
table's `why` column, where the same claim is made over every row; that test
keeps the one thing only it says, the unknown-code rule. Two hand-kept lists
of classes is the folklore the file's own header warns about.

A table pin cannot force the thinking — a red can be answered by copying the
new value across — so the `why` column holds what the wrong value costs for
every row that has a named cost, and the failure message says not to answer it
that way.

## The mutants

Four, each applied by exact string replacement, md5'd before and after so the
edit is known to have applied, run under a `-run` filter, restored, and
re-md5'd to baseline. All four RED; every file byte-identical afterwards.

| mutant | pin that caught it |
|---|---|
| `"G18"` → `upgradeBinaryRendered` (the surviving one) | `TestUpgradeEveryConditionClassIsPinnedByValue` |
| the scope clause dropped from `WatchImage.Sentence` | both scope pins |
| the clause dropped from the verb's "already runs" branch alone | `TestQAEveryLoopReadingSaysWhichInstanceItRead` |
| one row removed from `upgradeClassValuePin` | the completeness arm of the same test |

`TestQAALoopReadingCoversItsOwnInstanceAndNotItsSiblings` is the measured box
as a fixture: two homes, two live loops exec'ing ONE file, each holding its own
watch lock and record, then the rename. It asserts that one rename leaves both
loops stale, that each instance's reading names its own pid and its own home,
that neither row names the other's, and that each row says whose loop it read.
It pins the per-instance SHAPE — it is not a widening, and the bead above says
so in as many words so the next reader does not take it for one.
