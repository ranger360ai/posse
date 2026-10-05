# ranger-base-6uokf — the ADR 0066 D3 report: the failure line and the D3 record name the known screen(s)

*code, 2026-10-05. Built on the operator's ruling ranger-base-gy3io (option
B) and on ranger-base-76gc4 (the D3 record carries the pane capture).
Measurement behind it: `docs/notes.d/ranger-base-qk9tr.md`. ADR:
`docs/adr/0066-a-decision-model-is-a-second-reader-that-reports.md`, D3,
amended 2026-10-04 and built here.*

## What shipped

| file | what |
|---|---|
| `internal/posse/knownscreen.go` | the table of eight known screens and `KnownScreensIn`, the set-valued heading reader |
| `internal/posse/readingslog.go` | `Reading.LooksLike` (`looks_like`), and `ReadingEvidence.LooksLike()` which runs the reader over a record's regions |
| `internal/posse/unrecognized.go` | `WhatHerdrSaw` takes the set and appends one `looks like: …` row after herdr's working |
| `internal/posse/promptready.go`, `dispatch.go` | the three D3 record sites build the evidence once and read it twice — record and printed line |
| `scripts/readings-census.py` | the counted event: `looks_like_of`, `named_a_screen`, the `named_screens` figures, `print_named`, `--by screen`, and `looks_like` on every corpus case |
| `internal/posse/d3knownscreen_qa_test.go` | five arms, all mutation-checked (below) |

## §1 The one design choice: a separate table, not the interstitial registry

The bead flagged it and asked for one answer, not both. **A separate
known-screen table keyed by herdr rule id.** Three reasons, in the order
they decide it:

1. **The registry is read by a guard.** `DangerUnsilenced` walks
   `GrokInterstitials`/`CodexInterstitials`/… and `DangerRefusal` turns its
   answer into a launch refusal, on three surfaces that have to agree (ADR
   0013 §2: `dispatch.go launchSession`, `herdrback.go planLaunch`,
   `runtimepreflight.go`). ADR 0066 D2 says no guard reads the D3 report.
   Markers on those rows would put this reader's table *inside* the one
   table a refusal is derived from, and D2 would hold by care rather than
   by construction. In a separate file it holds by grep.
2. **The registry's subject is narrower than "known screen".** Every row
   carries `Where`, `Key`, `Silence` and a `Probe` — an operator-silenceable
   first-run screen. Three of the eight screens posse owns a capture of
   (codex's hooks review, trust directory, model picker) have no silence
   key, nothing for an operator to have done first and nothing for a probe
   to read. Those three rows would also grow three permanently-unknown rows
   in `posse runtime check`'s grid — the sentence `bobSignInSilence` exists
   to avoid writing.
3. **The two sets disagree in both directions, and on their keys.** The
   registry carries claude's two screens and bob's three, which posse owns
   no capture of and can write no marker for; it is per RUNTIME and keyed by
   a prose `Screen` sentence. The known-screen table is keyed by the herdr
   rule ids that name the screen today — the axis
   `scripts/d3-reader-eval.py`'s OPTIONS already uses, the axis a census
   groups by, and the axis the agreement pin can compare.

The registry is untouched. The one second copy of the markers is the spike
script's OPTIONS, and arm 1 compares the two rather than trusting them.

## §2 Why the reader is set-valued, and why it reads the capture

Both are the spike's measurements, not new decisions:

- **SET-valued**: 3 of the 15 labelled fixtures show two true screens at
  once (grok's startup splash with the consent banner drawn over it). A
  `choice` returns one, so a single-answer reader is wrong on a fifth of the
  corpus by construction — and wrong in the expensive direction, naming the
  screen whose fix is not the one needed.
- **The whole capture, not herdr's previews**: `agent explain` emits a
  243-character preview per region, and for 9 of the 15 labelled screens the
  heading is outside it. Over the previews the same reader names 5 of 11
  residue cases; over the capture, 11 of 11 with 0 false positives on the
  idle screens (MEASURED 2026-10-04, ranger-base-qk9tr). `PaneCaptureRegion`
  is on the record because of ranger-base-76gc4, and arm 5 pins the
  difference both ways — the capture names the screen, the previews alone
  name nothing — so a record that loses the capture reds here rather than
  degrading quietly.

## §3 What this saves, and how the number gets taken

The acceptance (Dave, via the 2026-10-05 comment) asked what it would save,
and the honest answer at build time was *unmeasured*: the readings log held
**0 records in 0 logs** on 2026-10-05 (`scripts/readings-census.py --by
decision`), because the fleet binary predates the log. So the saving is
built to be counted rather than argued.

A D3 record exists only because herdr recognized **nothing** on the screen —
that is what makes the reading consequential. So a non-empty `looks_like` on
a D3 record is, by construction, a screen herdr's rules did not name and
this reader did. The census prints it beside the D3 row:

```
  unknown screens NAMED by the heading reader: 2 of 3 D3 record(s)
    each one is an unknown-screen refusal diagnosed without the hand-launch and `posse peek`
    it used to cost (ADR 0066 D3, ranger-base-3j8) — a report only: no guard read it
           1  codex.update_menu
           1  grok.consent_banner
           1  grok.startup_splash
```

with `--by screen` grouping the second table by the SET (two screens on one
pane is one reading, not two) and `looks_like` on every exported corpus
case. Both figures are printed, so the residual is visible: a D3 record with
an empty set is an unknown screen that genuinely is not one of the eight,
which is the ordinary case and not a miss. The per-event human cost it is
measured against is ranger-base-3j8's — one hand-launch plus one `posse
peek` per screen.

**Not a quality claim.** The census counts what the reader *said*, never
whether it was right; nothing in a record labels the screen. Scoring against
labels stays `scripts/d3-reader-eval.py --corpus <dir> --labels <file>`,
which is why the corpus case carries the field.

## §4 Three things the bead did not say, now written down

**(a) The failure line cannot take its own reading.** `WhatHerdrSaw` is a
method on `AgentDetection`, and the capture is not on a detection — it is a
pane read posse took beside herdr's answer, and `AgentDetection` is what
herdr said. A reader keyed on the detection would be the preview-only
reader: 5 of 11. So the set is an ARGUMENT, computed once where the record's
evidence is built and passed to the line. That is why `logUnrecognized`
gained a return value and why the two dispatch sites bind `ev` before
logging instead of calling `d3Evidence` inline.

**(b) The reported route reads nothing, and should.**
`withPaneCaptureNamed` refuses a capture when `ev.Reported != ""` — a pane
herdr does not address is a pane posse may not read a screen off (ADR 0061
D3) — and such a detection evaluates no rules, so the evidence block has no
regions and the reader returns nil. The reported arm of the failure line is
therefore unchanged, and arm 4 pins that. Taking a capture there to feed
this reader would have been a new visibility decision smuggled in under a
diagnostic.

**(c) An empty marker must match nothing.** The natural spelling of a
conjunction is "every phrase is present", and `all` over no phrases is
*true* — so a row whose markers went missing in an edit would name its
screen on every pane. That is the one failure a substring reader cannot
report about itself, so `knownScreenMarkerIn` returns false on an empty
marker and arm 3 pins both halves (the guard, and that every shipped row
really has markers for the guard to be about).

## §5 Pins and their mutation checks

`internal/posse/d3knownscreen_qa_test.go`, arm 1 of the suite split, ~0.9s
for all five. No herdr call; `python3` for the two arms that read the script,
which is the shape `falseidle_qa_test.go` established (the real instrument,
never a second copy of its arithmetic).

| arm | claim | mutation | reds with |
|---|---|---|---|
| `TestQAKnownScreenTableIsTheSpikeScriptsOptions` | the shipped table and `scripts/d3-reader-eval.py`'s OPTIONS are one table, row for row, markers normalized | retyped `hooks need review` as `review your hooks` in `KnownScreens` | `codex.hooks_review: the shipped markers are [[review your hooks]] and the script's are [[hooks need review]]` (and arm 2 with `the reader names [] and the fixture shows [codex.hooks_review]`) |
| `TestQAKnownScreenReaderNamesEveryFixtureExactly` | every fixture under `etc/herdr/agent-detection/testdata/` is labelled and read exactly, both idle screens by name, at least one two-screen fixture | added `codex/zz-unlabelled.txt` | `codex/zz-unlabelled is shipped … and scripts/d3-reader-eval.py's EXPECTED does not label it` |
| `TestKnownScreensInIsSetValuedAndCanSayNothing` | the reader's own rules: conjunction, alternatives, empty marker, sorted, nothing on an idle pane | (covered by the two above; its own subject is the reader, not the tree) | — |
| `TestQAUnknownScreenFailureLineCarriesTheLooksLikeRow` | one row, appended last, herdr's working untouched, no row on an empty set, reported arm unchanged | dropped `+ looksLikeRow(looksLike)` from `WhatHerdrSaw` | `the appended row is "", want one indented line naming both screens` |
| `TestQAD3RecordCarriesLooksLikeAndTheCensusCountsIt` | the field round-trips, the REAL census counts 1 of 1, and 0 of 2 over pre-bead-shaped records | changed the field's json tag to `-` | `the record came back with looks_like=[]` and `the census says d3=1 named=0` |

**The fixture enumeration is deliberate and it is why arm 2 reads the
directory rather than a list.** The script only warns about an unlabelled
fixture on stderr and still exits 0, so a sixteenth capture would be measured
by nothing. Arm 2 is the shape `treewidedoor_qa_test.go`'s head comment calls
out as excluded from the tree-wide class in this package — an enumeration
rooted at a SUBDIRECTORY, which reds when that one directory changes, and
that bead is a detection bead which ought to red it (`detectionRig` and nine
siblings are the same shape). It needs no new Makefile door for the same
reason they do not.

## §6 What this is not

No model, no network, no config key, no spend, no new CLI surface, and no
change to any guard. ADR 0066 D3(c) stands: a model on this seam is not
recommended, its only territory is a reworded heading, and that class has
zero incidents in the record. D4 would make a live call two operator rulings
away, and nothing here moves toward one.
