## A recipe nothing can execute goes stale in silence (ranger-base-l1vej)

The rollback half of the queue-cutover window was prose in the instance
tree's runbook, and six pins in `internal/posse/queuecutover_qa_test.go` ran
that prose — lifted the fenced block out of the page and drove it against a
fixture, because a prose assertion over a recipe goes green the moment the
recipe is reworded. When ADR 0024 D4 moved the runbook out of this repo as a
one-deployment procedure (92e67bd), the pins followed it by absolute path and
`t.Skip`ped when it was not there.

That is a skip on every box but one. MEASURED at `6893687`, the bead's own
two arms of one binary — `HOME` at the operator's and `HOME` at an empty
directory, which is `ubuntu-latest` running `make test`:

| arm | PASS | SKIP |
|---|---|---|
| instance tree present | 24 | 0 |
| instance tree absent (CI) | 19 | 5 |

and the 5 understates it. Two of the 19 report **PASS** with arms skipped
underneath: `TestQueueRollbackIsWrittenForAStateStepEightRemoves` skipped all
four of its subtests, and `TestQueueRollbackRunsCleanWhenNoRedirectWasEverWritten`
skipped its real arm and passed on its control alone — a test whose only
running arm is the one that proves the rig can see a defect. A gate that reads
exit codes sees a green suite. So the honest count is 7 tests carrying 11
dead arms, not 6 skips.

### The fix is the direction of the move

The two outcomes ranger-base-sssr named were "the pins move to the instance
tree with the runbook" or "they are retired deliberately". Neither was
reachable: the instance tree carries no Go module and no gate, so moving them
there trades a skip on CI for a suite nothing runs at all; and retiring them
drops the only executable coverage of the block an operator pastes with the
fleet quiesced. Copying the runbook's text back into this repo is the third
thing, and it is the one the routing rule refuses — the bytes came from the
private tree and nothing moves to a wider audience than its source.

So the *block* moved to where the pins are, instead. `queue-cutover.sh
--print-rollback` prints it with the run's own `$CONSTITUTION`/`$QUEUE`/
`$WORKTREES`/`$SCAN` already substituted, and does nothing else — the same
shape as the four partial UNDO blocks the abort trap has always printed, one
of which `queuecutover_undo_qa_test.go` already ran out of the script's own
stderr. The runbook keeps what a script cannot hold: when to roll back, and
what each line is for. Both arms are now 24 PASS / 0 SKIP.

Two things fall out of it. The pins' subject is under version control here,
so an edit to it cannot red or silence this suite from outside the tree —
and the source text names the instance's trees by variable, never by this
box's path, so `make ops-check` covers it like any other script text. And
`--print-rollback` is spelled with the verb: it prints, and a rollback stays
a thing a person decides to do.

### The rig is shown able to fail

Five mutations of the printed block, each run on the CI arm (`HOME` at an
empty directory), each reverted before the next:

| mutation | reds |
|---|---|
| the dotfile glob leaves the move loop | `…CarriesTheStoresDotfilesHome` |
| the `rm` loses its `-f` | `…RunsCleanWhenNoRedirectWasEverWritten` |
| the discovery walk goes away | `…BringsHomeTheTreesTheListForgets`, `…SendsHomeATreeSpelledAHandsWay` |
| the runtime-file ignore append goes away | `…VerificationFiresOnARollbackThatWorked` |
| step 8's commit is assumed to be `HEAD` | `…IsWrittenForAStateStepEightRemoves` |

All five killed. Before this bead every one of them would have gone
unnoticed on the gate.

**Still open:** the runbook page still carries the block it used to own, now
a second copy of what the script prints. A dispatched session under
`RHQ_CAGE=seatbelt` cannot write the instance tree (`EPERM`, measured), so
the edit is `ranger-base-gwckm` with the replacement text in its comments.
