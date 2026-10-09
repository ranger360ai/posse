# ranger-base-00a5l — the three findings ranger-base-5ayqc's verify left live

Filed by holden out of verifying four closes (ranger-base-5ayqc). Two of the
four verified clean; these are what the other two left behind, each recorded
by a pin that shipped GREEN over the hole. Both pins are gone now — their
subjects are fixed and the agreement is asserted in their place.

## Finding 1 — the `--chain` prescription refused on the second bd install

`internal/posse/gates.go`. ranger-base-2msgj added three surfaces that all
prescribe one command, `posse gates install-hooks --chain`: the l3Displaced
Degraded row, the install-hooks refusal over bd's shim, and INSTALL.md §9 —
which names the recurring state in as many words ("any later `bd init` (a
re-init, `--from-jsonl`) or `bd import` displaces the gates again").

It worked once. posse's first chain parks bd's shim at `bd-<slot>`; a second
bd install takes the slot back, and `chainBdShim`'s pre-existing guard
refused outright over that leftover — so the one prescribed command did not
run and both walls stayed down. Which wrong line the operator met first
depended on a suffix that is bd's to choose: with a fresh suffix the round-1
leftover still carried the marker and the row read l3Displaced over a STALE
file; with bd's one suffix reused, posse's own dispatcher landed on
`<slot>.backup`, carries no marker, and the verdict fell back to the
anonymous l3Foreign line the bead set out to replace. In both, posse's live
gate at `posse-<slot>` was named by nothing — `posse-<slot>` does not begin
with the slot, so the prefix scan cannot reach it.

Three changes:

- `chainBdShim` takes over a `bd-<slot>` that is itself bd's shim, and
  refuses everything else by name. Overwriting one copy of bd's shim with
  another loses nothing an operator could want: bd re-plants it on every
  install, and nothing on the dispatch path reads that file until the
  dispatcher execs it. A file there that is NOT bd's shim is the operator's
  own, and posse's rename would destroy it without a word — so that refusal
  stays, and now says which of the two it is.
- `displacedPosseHook` asks `posse-<slot>` first, separately from the prefix
  scan. That is the one displacement no renaming scheme can be read off, and
  it is the live gate in both suffix cases.
- `l3DegradeLine` and the install-hooks refusal word that case apart.
  `MOVED ASIDE` became `TAKEN BACK`, because posse put the file there and bd
  only took the slot in front of it — and the deletion advice ("delete it
  once the slot is chained") now rides only the file bd renamed. On
  `posse-<slot>` it would have told the operator to uninstall the wall they
  had just rebuilt.

MEASURED 2026-10-09, this box, over both of bd 0.50.3's rename suffixes:
round-2 `--chain` restores both walls, leaves bd's shim reachable at
`bd-<slot>`, and the row names `posse-<slot>`.

INSTALL.md §9 gained the table of what sits beside a chained slot and whose
each file is. It had said only that `<slot>.backup` is the operator's to
remove and had never mentioned `bd-<slot>` at all — the file that blocked the
retry, and one posse itself wrote.

## Finding 2 — the row/verb contradiction survived one config VALUE over

`internal/posse/backup.go`, `internal/posse/backuploop.go`.
ranger-base-0q7rp's complaint was general: "the governance row and the
command disagree about whether a store of record exists". `RunBackup` makes
THREE store-of-record refusals before it writes anything — `queue_repo:`
unset, the path holds no `.beads`, the path is not a git checkout — and that
close keyed its whole fix on the first. The other two reproduced the original
symptom unchanged, in the state the close's own control fixtures sat in
(`queue_repo: `+t.TempDir()).

MEASURED 2026-10-09 before the fix, shipped binary from this tree, scratch
RHQ_HOME, `backup_interval: 6h`, `backup_max_age: 12h`, `queue_repo:` naming
a plain directory: `posse status` raised LANE "no backup of the store of
record on this box — <dir> is empty", whose only remedy is `posse backup`;
`posse backup` exited 1 with "<q> has no beads store at <q>/.beads"; and
`posse backup status` was WORSE than the state the bead filed — its schedule
line still read "every 6h00m, from the dispatch --watch loop" with no clause
at all, the exact line ranger-base-0q7rp had named as the one claiming a
cadence nothing can keep.

The three refusals are now one question, `backupStoreGap`, asked in
RunBackup's own order and read by all four surfaces: the verb, `NoStore` /
`NoStoreWhy` on the freshness reading, `GovDetail`, and
`BackupScheduleLine`. Two halves — `backupUnsetQueueGap` + `backupNoStoreTail`
— because the surfaces have different room: the verb and the freshness line
carry the whole sentence, and the schedule line printed directly under the
freshness line carries the state alone rather than repeating it.
`backupNoStoreClause()` is gone; the pins now compare the row, the verb and
the schedule line against the READING's own words rather than against a
constant, which is the shape that let a one-of-three fix pass.

It costs one `git rev-parse --git-dir` fork on a reading that had none,
reached only when the two cheap questions pass. That is the price of the
agreement: a cheaper stand-in (a `.git` beside the path) answers differently
for a subdirectory of a repo and for a worktree, which is a new disagreement
in place of the one being closed. Every caller is a CLI invocation or a watch
tick.

## Finding 3 — `storelessReading` was unpinned

Stubbing it to `""` survived every test in the tree: the row would have
rendered "…so there is nothing to back up — : point queue_repo: …" with
nothing noticing. Its `default` arm was also suspected unreachable through
`GovDetail`. It is reachable — the row fires on `Stale`, and a DATABLE
archive past `backup_max_age:` is stale — and all three arms are pinned now
by `TestTheNoStoreRowReportsWhateverTheArchiveDirectoryHolds`, with the
directory evidence asserted in the main agreement pin as well.

## Pins

Gone (their holes are closed):

- `TestQADisplacementRemedyRefusesOnTheSecondBdInstall`
- `TestQABackupRowStillDisagreesWhenQueueRepoNamesNoStore`

In their place:

- `TestQADisplacementRemedyWorksOnTheSecondBdInstall` — both suffixes, the
  row names `posse-<slot>`, the prescription runs, bd's shim survives.
- `TestQAChainStillRefusesAForeignBdSlotNeighbour` — the refusal `bd-<slot>`
  still earns.
- `TestNoStoreOfRecordRowAgreesWithTheVerb` — a table over all three
  refusals; the row, the verb, the quiet line and the schedule line are each
  compared against the reading's own sentence.
- `TestWithAStoreOfRecordTheRowIsAboutTheArchive` — the control, on a real
  store: the condition goes back to being the missing archive.
- `TestTheNoStoreRowReportsWhateverTheArchiveDirectoryHolds` — finding 3.
- `TestBackupSurfacesAgreeWhenTheStoreHasNotMoved` (cmd/posse) — the same
  agreement where an operator meets it, CLI to CLI, over all three gaps.

That last one is where finding 2 escaped a second time: it is the CLI-level
agreement pin and its own control arm sat in the broken state
(`queue_repo: `+t.TempDir()), so it asserted the agreement for one of the
three refusals and asserted the other two away.

**`queue_repo: `+t.TempDir()` was the house fixture for "a store of record
exists", and it was never one.** Widening NoStore turned every rig built on
it into an instance of the gap, which is how the suite found them — four
across three files, each with a comment saying it presumed a store:

- `freshRig` (internal/posse/backupfresh_qa_test.go) — "every AGE reading
  below presumes a store exists"
- `TestStaleBackupRaisesACarryOverNotATenthGRow`, `TestArmedWithNoArchiveIsStale`
- `TestBackupSurfacesAgreeWhenTheStoreHasNotMoved`'s control (cmd/posse)
- `TestGovernanceRaisesStaleOnAnUndatableArchive`
  (internal/posse/backupfuture_test.go) — "that is the premise of every
  reading in this file"

Each now takes a real one: a git checkout with a `.beads` directory
(`freshQueue` in-package, `backupQueue` in cmd/posse; `backupClockQueue`
already existed for the loop tests and was the only rig that was right).
The lesson is the fixture, not the four edits — a premise spelled as the
cheapest thing that used to satisfy the check is a premise that stops
holding the moment the check gets stricter, and says nothing when it does.

Mutation-checked, 2026-10-09, five mutants and all five died:
`chainBdShim` refusing again over a bd shim; `displacedPosseHook` dropping
the `posse-<slot>` question; `NoStore` keyed on the unset key alone;
`storelessReading` stubbed to `""`; `BackupScheduleLine` keyed on the unset
key alone.
