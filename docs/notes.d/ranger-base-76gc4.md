# The D3 record carries the screen now, and three things the design named had to be re-measured (ranger-base-76gc4)

Code bead, 2026-10-04, one seat, branch `posse/dinesh-posse-ranger-base-76gc4`
off `f354346c`. Implements ADR 0066 D3's amendment **(a)**
(ranger-base-qk9tr): D1's D3 record carries a fixture-shaped pane capture.
Every number below is **MEASURED** on this box (macOS darwin 25.4.0, APFS,
herdr 0.9.1, go1.26.5) with the command that produced it.

No model was called, no credential read, no network reached, nothing spent.
Nothing left the box.

---

## 1. What landed

Four D3 consequence sites, three functions:

| site | consequence | what it was | what it is |
|---|---|---|---|
| `promptready.go` `AwaitPromptable`, reported-not-seen refusal | refusal | `logUnrecognized(session, lastGuess, …)` | `logUnrecognized(session, target, lastGuess, …)` — and the capture is SKIPPED, see §4 |
| `promptready.go` `AwaitPromptable`, unrecognized-screen refusal | refusal | same | same, capture taken |
| `dispatch.go` `awaitDelivered` deadline | hold | `Herdr: ReadingEvidenceOf(lastGuess)` | `Herdr: d.d3Evidence(target, lastGuess)` |
| `dispatch.go` `awaitSettled` deadline | refusal | same | same |

New: `Herdr.PaneReadDetection` (herdr.go), `PaneCaptureRegion`,
`(*HerdrBackend).withPaneCapture` and `(*Dispatcher).d3Evidence`
(readingslog.go). One herdr call, on the refusal/hold path only, exactly as
`logPromptHandBack` already pays one `agent explain` on the hand-back path.
A read that errors adds nothing and says nothing.

`scripts/readings-census.py --export-corpus` needed no change: it writes
every region to `regions/<id>/<region>.txt`, so the capture lands on disk
as a file `herdr agent explain --file` can be pointed at.

## 2. DIVERGED: the call is not `PaneRead(target, 40)`

The ADR amendment and the bead both name `b.H.PaneRead(target,
paneModeReadLines)`, glossed as "the plain-text read, 40 lines, **the same
source the fixtures were captured from**". Those two halves are not the same
call. The gloss is the design; the spelling contradicts it twice, and both
are measured.

**The source is not a synonym.** `Herdr.PaneRead` passes no `--source`, so
it takes herdr's default, `recent`. The fixtures under
`etc/herdr/agent-detection/testdata/` are `--source detection`
(`etc/herdr/agent-detection/README.md`, `docs/runbooks/agent-detection-manifest.md`),
and `scripts/verify-detection.sh` replays them as such. MEASURED 2026-10-04
over the five live panes on this box, text format, md5 compared:

| pane | `recent` vs `detection` |
|---|---|
| w2RM:p1, w2RN:p1, w2RP:p1, w2RQ:p1 | byte-identical |
| w23H:p1 (quiet, with scrollback) | **differ**: 80 lines / 4,205 bytes against 50 / 2,670 |

Stable, not a moving screen: three paired reads of w23H:p1, `detection`
against `detection` identical every time and `detection` against `recent`
different every time. `recent` reaches back above the screen; a fixture is
the screen. (ranger-base-0sa5a's note left exactly this ASSUMED — "if
`recent-unwrapped` renders a wrapped dialog differently from `detection`, an
anchored pattern could miss". For `recent` against `detection`, on a pane
with scrollback, it does.)

**The tail cuts the wrong end.** A client-side tail of 40 keeps the LAST 40
rows. What a D3 record is missing is the FIRST ones — the heading under
15-16 rows of logo, which is the whole finding of ranger-base-qk9tr §5. The
tallest fixture posse owns is 51 lines
(`grok/idle-startup-splash-wide-boxed.txt`), so a 40-row tail of it drops 11
rows off the top: precisely the bytes the capture exists for.

So the capture is `PaneReadDetection` — `pane read <id> --source detection
--format text`, whole, no tail — which is what the gloss asked for. The
design does not change and nothing is handed back to the architect; what is
stale is one Go identifier inside an accepted amendment's parenthetical, and
it is recorded here rather than edited there. A reader who follows
ADR 0066 D3 to `PaneRead` finds the wrong source and the wrong window.

The argv is pinned, not just the behaviour
(`TestQAD3RecordCarriesTheScreenAndNotOnlyItsTruncatedPreviews`): substituting
`PaneRead(target, paneModeReadLines)` reds that arm and nothing else
(mutation-checked).

## 3. The "fits in a page" claim: withdrawn as stated, re-measured

`AppendReading`'s comment said one O_APPEND write of "a record that fits in
a page" was what kept two processes from interleaving a line. The bead asked
for that to be re-measured or for what tears to be named. Both halves were
wrong in the same direction — the page was never the mechanism, and it is no
longer true of the shape either.

**Record sizes**, MEASURED 2026-10-04 (`encoding/json` over a real
`Reading`, one line including the newline):

| record | bytes |
|---|---|
| six truncated previews, no capture | 2,218 |
| the same six + the tallest fixture posse owns (3,969 bytes) | **6,297** |
| the same six + a 60x200 screen of box-drawing glyphs (3 bytes a column) | **38,388** |

**Tearing**, MEASURED the same day: N processes, each appending fixed-size
records with `O_APPEND|O_CREATE|O_WRONLY` and one `os.File.Write` per record,
every resulting line checked for length and for a single writer's bytes.

| record size | 6 writers x 300 | 16 writers |
|---|---|---|
| 512, 4096, 4097, 8192, 16384, 65536, 131072 | 0 torn | — |
| 16384 | — | 0 torn (x200) |
| 65536 | — | 0 torn (x100) |

The kernel holds the inode across the call, so what keeps a line whole is
that there is ONE call — which is why `AppendReading` marshals first and
writes once, and why a second `Write` there would be the bug however small
the record. POSIX does not promise this for a regular file; this box does,
at every size the shape can produce.

**What tears, since something must be named**: not concurrency at any size
tried, but a SHORT write. `os.File.Write` (go1.26.5, `src/os/file.go`) does
not retry — it returns `io.ErrShortWrite` when the syscall took fewer bytes
than it was handed — so a full disk or a signalled partial write leaves a
fragment and returns an error `AppendReading` hands back and `LogReading`
swallows. `ReadReadings` then skips the unparseable line and
`scripts/readings-census.py` counts it torn: one reading, not the corpus.
That is the same failure the old comment described; the trigger is the disk,
never the page.

The comment now says all of this. `TestQAD3AReadingWithAPaneCaptureIsStillOneLine`
holds the half a unit test can hold — one record is one line at any size the
shape produces, and the 38 KB arm fails if it ever comes back under 4,096.

## 4. Where the capture is NOT taken, and why each is silent

- **The reported route** (ADR 0061 D3.3, `Reported != ""`). herdr addresses
  no screen there; the evidence is another authority's word. A capture would
  be posse reading past its own detection. Skipped, as the bead asked.
  ranger-base-qk9tr §1 already found the other half of this: `agent explain`
  refuses Bob outright, so a D3 record on that route carries no region bytes
  at all, for 1 of posse's 4 runtimes.
- **No target.** `awaitTarget` can fail before any pane is known.
- **A read that errors or comes back empty.** The log is best effort
  (`AppendReading`'s own rule), and a record that says what herdr saw and
  nothing else is the record there has always been.

The rule lives once, in `withPaneCapture`, rather than four times at the
call sites.

And one more, found while wiring it: **a `--dry-run` pass.** `logReading`
already refuses to write on a dry run (seatidle.go's rule — a pass that
acted on nothing must not leave state a later pass counts), but a Go
argument is evaluated before the function that would have discarded it, so
`logReading(…, Reading{Herdr: d.HB.withPaneCapture(…)})` forks a `pane read`
for a record the dry run then throws away. `(*Dispatcher).d3Evidence` carries
that rule one step earlier, in one place rather than at both dispatch sites.
The HAND path (`AwaitPromptable` → `logUnrecognized`, on `HerdrBackend`) has
no dry run to honour and is unchanged: `posse prompt` either refused or it
did not.

## 5. What this does not do

No second reader, no `WhatHerdrSaw` change, no failure-line change, no
model, no config key, no network. The heading reader is ranger-base-6uokf,
still dep-blocked on the operator's ruling ranger-base-gy3io, and this bead
is the input it was blocked on. Nothing here reads the capture; it is
written down.

## 6. Verification

- `go test -tags posse_arm3 -run TestQAD3 ./internal/posse` — 5 pins, green.
- **Mutation-checked**, each reds exactly one arm: `withPaneCapture`
  returning `ev` unchanged reds arm 1 ("the D3 record carries no
  `pane_capture` region"); substituting `PaneRead(target,
  paneModeReadLines)` reds arm 1 on the argv; dropping the `Reported != ""`
  guard reds arm 2 on both the region and the spent read; dropping
  `d3Evidence`'s `DryRun` return reds arm 5 on both the region and the
  spent read.
- Arm 1 refuses to pass if its own fixture's heading ever drifts inside the
  243-character cap, because then it is no longer telling a capture from a
  preview.
- `make test`, all three arms, exit 0, no FAIL line anywhere in the run:
  arm 1 `cmd/posse` 256.4s / `internal/posse` 397.2s / `internal/treepins`
  444.3s plus the fourteen tree-check doors, arm 2 341.7s, arm 3 342.1s.
  `make tree-check` re-run afterwards for the docs in this fragment, which
  landed after that suite.
