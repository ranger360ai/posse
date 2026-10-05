## A quality column that counted the lossy case, and three pins that could not fail (ranger-base-r5546)

ranger-base-qr0as verified four closes and left four live findings. All four
are the same shape at different distances: a claim that is true today, written
somewhere nothing reads, so the day it stops being true nothing says so.

### F1 — `replayable` counted a truncated region

`scripts/readings-census.py` produces the denominator ADR 0066 D1 exists for,
and `replayable` is the one quality claim it makes about a case. Its test was
`if regs and not rec.get("redacted")`: redaction only. The other way a case's
bytes stop being the bytes that were read is TRUNCATION — herdr previews a
region at 243 characters (`internal/posse/panework.go`), `whole_recent` and
`top_non_empty_lines(20|25)` are in the shipped manifests, and
`ReadingEvidenceOf` carries every region herdr evaluated. So the richest
records in the log, the D3s, routinely carry a few per cent of the bytes of
their biggest region and were counted reproducible.

`ReadingRegion.Truncated` was written by one line and read by nothing. It is
read by the census now, and the census reads THE FIELD rather than comparing
`bytes` against the text, for two reasons that are each sufficient: the data
ceiling rewrites the text after the flag is taken (`AppendReading`), and a
Python `len` counts characters where herdr's `bytes` counts bytes, so every
region carrying a `·` or a `→` would read as short by one or two.

Two outputs changed. The table line now reads
`readings: N in M log(s) · N replayable · N truncated · N redacted`, and each
exported corpus case carries a `truncated` field naming the regions herdr cut
down, beside the `redacted` field naming the ceiling classes.

MEASURED 2026-10-04, this box, mutants against
`TestQAReadingsCensusCountsByDayAndExportsTheCorpus` and
`TestQAReadingEvidenceMarksARegionHerdrCutDown` — all four now FAIL where the
first two were qr0as SURVIVORS:

| mutant | before | after |
|---|---|---|
| M5 `Truncated: false` in `ReadingEvidenceOf` | survived, 1.846s | FAIL |
| M4 drop ` and not rec.get("redacted")` | survived, 0.855s | FAIL |
| drop the new truncation clause | — | FAIL |
| drop the corpus case's `truncated` field | — | FAIL |

M4 survived because no record in the fixture was redacted, so the
`replayable != 4` arm had purchase on neither half of its own definition. The
fixture now carries one truncated record (`bytes` 4949, 243 characters of
text) and one redacted record, on a day of their own so the per-day arms keep
measuring what they were written for. The redacted one is appended by hand
through `appendRawReading`: `Redacted` is set by `AppendReading` from the
instance's ceiling and never by its caller, so an App with no ceiling
configured cannot be made to write one — and the redactor has its own pin.

Note for anyone reading the older fragment: `docs/notes.d/ranger-base-3xt9y.md`
quotes `5 replayable` over a seven-record fixture and says the column
"excludes a case whose bytes are not the bytes that were read". That was true
of redaction and false of truncation, and the line shape it quotes predates
this change. Left as the frozen reading it was.

### F2 — two of three never-under-home bounds were unreachable

`readingsLogOutOfBounds` carries three bounds: the state dir, the harness
home, and `~/.claude`. Arm 4 of
`TestQAReadingsLogLivesUnderTheSessionTreeAndNeverTheHome` made one real repo,
inside the state dir — and because the state dir is UNDER the harness home,
that one repo was refused by the first row left whichever of the other two was
deleted.

MEASURED 2026-10-04, one repo per bound now, each arm asking for ITS row's
reason and not merely for a refusal:

| mutant | before | after |
|---|---|---|
| M10 delete the `a.Home` row | survived, 1.884s | FAIL |
| M11 delete the `~/.claude` row | survived, 1.838s | FAIL |
| delete the `a.StateDir` row | survived (found here) | FAIL |

The third one is why the arms assert the refusal's text. The state dir is
under the home, so its row is unreachable by OUTCOME however many repos the
fixture makes — delete it and the home row refuses the same path, green. What
the row carries that the home row does not is the reason, and the operator
reading the refusal is owed the one that applies.

`~/.claude` is the one bound that is not per-App — it is the test binary's
temp home, shared with every other test in the package — so that repo is made
with `os.MkdirTemp`, or two live tests under `-count=2` share it.

### F3 — the public door table had no pin

`internal/treepins/README.md` carries the fourteen tree-check doors as a table,
one row per door with its subject. It was 14/14 correct the day it shipped —
same rows, same order as `make -n tree-check` — and nothing would have said so
when it stopped being: a door renamed, added or removed edits the Makefile and
`treewidedoor_qa_test.go`'s head comment, both pinned, and leaves the public
page saying what used to be true. The repo already refuses that duplication
one directory over (Makefile:735-742 declines to write the door COUNT twice).

Arm 4 (`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles`) reads the table
now, through `twdReadmeDoors`, and holds it to `mkPrereqs(t, src, "tree-check")`
with `slices.Equal` — membership AND order, because the table reads as the
order the doors run and a reader pricing a partial run from it is owed that.
The row matcher is anchored (`^| \`make <door>\` |`) on purpose: the page names
`make tree-check`, `make crew-check` and `make notes-check` in its prose too,
and a reader that took those would be reading advice as if it were the list.

MEASURED 2026-10-04: dropping a row, swapping two rows, and renaming one all
FAIL; clean is green, 0.344s.

### F4 — an arm-2 pin that inherited the order it depends on

`TestQASettledHolderRacingTheScanIsSkipped` arm 2 FAILED once in a full
`make test` at 1f672164 — "the queue was empty, so the bead never reached the
fire loop". Not reproducible solo: 25/25 PASS, including `-count=20` at load
average 43.4, heavier than the run that reddened.

The fixture stages a holder that settles BETWEEN the ready scan's listing and
the fire loop's, with ranger-base-rrg2's `unhide-when-locked` lever: hidden
from `workspace list` until the launcher lock is HELD. The lock is a statement
about the destructive TAIL, not about which phase is running, and a pass takes
it in more than one — the reap sweep and the land sweep both run ahead of the
ready scan and both can list workspaces under it. When one of them did, the
holder was visible by the time `interruptedRuns` asked, `interruptedRuns`
declined it (its documented behaviour without `--resume`), and the queue read
empty.

`unhide-after-bd` holds the lever shut until a named bd command line is in
`bd-calls.log`, checked AFTER the lock verdict so it narrows the window rather
than replacing it. The pin names the scan's own query,
`list --status in_progress`, which is the pass's only one: the other two
callers of `Bd.InProgress` are the cockpit's display and the governance
surface, neither of which a dispatch pass runs. The setup's `bd-calls.log` is
removed beside its `calls.log`, so nothing the staging said can open the gate.

MEASURED 2026-10-04, this box:

- 5/5 PASS with the gate in place; 12/12 PASS solo.
- Instrumented and run ungated 12 times solo: ZERO locked listings ahead of
  the scan. The order this fixture used to inherit is the right one on an idle
  box, which is why the flake needed the full parallel arm to show.
- POSITIVE CONTROL, which is what says the gate is load-bearing rather than
  decorative: plant a gate string that never appears in the bd log and the
  pass fatals on "the lever never fired" — the lever really is held shut for
  the whole pass by the plant, so a lock taken before the named phase can no
  longer open it.
