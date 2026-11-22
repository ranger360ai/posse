# ranger-base-repzh — three arms of the retry mark that nothing drove

QA verify of four closes (ranger-base-kzk3u, ranger-base-2uvui,
ranger-base-3m7sj, ranger-base-oe884). All four verify. What landed here is
the suite half: three pins over arms of ranger-base-kzk3u's retry mark that
survived every mutant the close graded and every mutant a re-grade added.

## The arms, and why they were invisible

ranger-base-kzk3u keeps the record of a seat whose kill did not confirm,
marks it with `kill_pending` and `kill_beads`, and retries the kill on every
later tick until the seat is over and every bead in `kill_beads` has been
told. Its four rows reach the mark through a bead the tick FILED — both the
detach fixture and the orphan fixture file one — so three arms of the same
mechanism were driven by nothing.

MEASURED 2026-11-22 (linux 7.2.15-1-omarchy, go1.27.3, 16 cores), each mutant
applied ALONE over a checksummed golden copy of `internal/posse/wallwatch.go`
and graded by `go test ./internal/posse -run WallWatch` whole — 55 top-level
tests at d3957c06, 65 at main d5527330 and 68 with the three pins below, the
copy restored and its sha256 re-verified after each:

| mutant | at d3957c06 | at main d5527330 |
|---|---|---|
| the detach arm's `beads := commented` narrowed to the FILED id alone | SURVIVED | SURVIVED |
| the retry's "no bead holds the kill that did not confirm" line deleted | SURVIVED | SURVIVED |
| the failed retry's record re-write deleted | SURVIVED | SURVIVED |
| the orphan arm's already-filed bead not carried into `kill_beads` | SURVIVED | killed by ranger-base-6r6ox |
| the closer's own five, re-run UN-COMBINED | each red 1-4 | each red 1-4 |
| the retry arm removed; a marked record dropped under a live watcher; a retry that kills a seat already over | red 4 / 1 / 2 | red 4 / 1 / 2 |

The first is the one that mattered. A detach inside `wallWatchDedupe` of
another seat's files NOTHING and comments on the open bead instead, so `filed`
is empty and `commented` is the whole of what holds the kill — drop it and a
seat whose kill did not confirm is marked with no bead at all, which is the
state ranger-base-kzk3u was filed to end, one arm over.

The fourth row is the reason this page prices the same battery twice. It
survived at the close's own commit and is killed on main, because
ranger-base-6r6ox landed in between and brought a row that reaches the
already-filed orphan arm. A verify that graded only the tree the closer
graded would have filed a finding that was already closed; a verify that
graded only main would have credited the close with a pin it did not write.

## What the three pins hold

- `TestWallWatchBackstopMarksACommentedBeadWhenNothingWasFiled` — a detach
  that comments on an open bead rather than filing one: that bead is what the
  mark carries, it is told the tick is retrying, and the retry that confirms
  tells it the seat is over before the record goes.
- `TestWallWatchBackstopSaysWhenNoBeadHoldsTheUnconfirmedKill` — a kill that
  did not confirm and could not be filed: the verdict says it is the only
  record, and claims no bead was told.
- `TestWallWatchBackstopRetryRecordsItsLatestFailure` — a retry that fails
  again re-writes `kill_pending`, so the failure the eventual verdict and
  comment name is the latest one and not the first.

Each reds exactly its own mutant, one red each, and the thirteen-arm battery
is now wholly killed un-combined.

## Also measured here, nothing landed

- ranger-base-2uvui's replay: the `(author email, author date, subject)`
  pairing holds, one original in and one commit out, and the hand-merged
  deadline branch was re-read with ranger-base-mhl7h's own `0 * time.Second`
  mutant ON MAIN — one member line at `wallclaudeprojects_qa_test.go:416`,
  no panic, no SIGSEGV, PASS unmutated. The class behind it is closed
  tree-wide: a census of every `cmd` started inside a `go func` whose
  `cmd.Process` is read outside it finds 2 sites at 6c5b1488 (which are
  ranger-base-mhl7h's and ranger-base-eheo3's, its wrong arm) and 0 on main.
- ranger-base-3m7sj's close (another repository) was verified exhaustively;
  the reading is on that bead.

## Not closed here

- ranger-base-6felj

## Re-committed, 2026-11-22, laurie

Under ranger-base-8h17w: this page and its three pins were first committed
twice over, and both of those commits were held at the push gate and parked
unlanded. What ships here is one commit cut from the tree they were held
against, with the counts above as the only version of them that was ever
public — the held messages said "65" for a battery graded over two trees, and
the three counts are 55, 65 and 68 as the table's head says. No verdict
moves: the same mutants survived and the same ones red.
