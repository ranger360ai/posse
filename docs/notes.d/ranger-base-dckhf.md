# The reading that typed now rides on the record its keystrokes produced (ranger-base-dckhf)

Code bead, 2026-10-04, one seat, branch
`posse/dinesh-posse-ranger-base-dckhf`. Builds ADR 0066 D1 as amended by
ranger-base-o1aoi: the D5 stall record carries the settle gate's pane-state
reading and a pane capture from each side of the keystrokes. The design, the
prices and the rejected alternatives are in
`docs/notes.d/ranger-base-o1aoi.md`; the gap it closes is
`docs/notes.d/ranger-base-3xt9y.md` §4. This note records only what the build
found that the design did not say.

Nothing here is a new measurement of the fleet. The numbers the design rests
on are the note's, dated 2026-10-04 and labelled there.

---

## 1. What landed

| file | what |
|---|---|
| `internal/posse/readingslog.go` | `Reading.Gate` (optional second evidence block), `PaneCaptureAtPromptRegion`, `withPaneCaptureNamed`, `gateReading`, `ReadingEvidence.RegionOf`, and the ceiling over the new block |
| `internal/posse/dispatch.go` | `awaitAgent` returns the gate's detection; `launched.gate` carries it; `fire`'s typed arm takes the type-time capture and holds it on `pendingBead.gate` |
| `internal/posse/promptstall.go` | `logStall` writes the block; `stallGate` completes it with the stall-time capture |
| `scripts/readings-census.py` | the false-IDLE count, the gate's regions in the regions table, the gate block in the corpus export |
| `internal/posse/falseidle_qa_test.go` | five arms (below) |
| `internal/posse/herdr_test.go` | the fake's numbered `pane-text/<key>-<n>` lever |
| `internal/posse/livesplash_test.go` | the live pin asserts the gate reading it now receives |

## 2. Three things the design did not say, and what each one cost

**(a) The ceiling was keyed on a FIELD, not on the regions.** `redactRegions`
is keyed on no region name at all — that is deliberate and pinned
(`TestQAReadingsLogRedactsCeilingContentBeforeWrite`) — but `AppendReading`
reached it through `r.Herdr.Regions` and nothing else. A second block of
regions is therefore a second route for a capture of somebody's terminal to
reach a local file untouched (ADR 0050 D2), and the two captures are the
largest regions any record carries. Both blocks go through the ceiling now
and both blocks' classes join one `redacted` list.

**(b) Redacting through the `Gate` pointer would have reached back into a
bead still in flight.** `AppendReading` takes its `Reading` by value, which
makes every other field a copy; `Gate` is a pointer, so the redaction would
have rewritten the evidence `fire` is holding for a prompt whose verdict is
not yet decided. The visible cost would have been a stall-time capture
appended to a block whose type-time half had already lost its text — the
pair intact in neither record. The writer copies the block before redacting
it, and arm 5 pins the held evidence as well as the written record.

**(c) A fake that serves one screen cannot measure a claim about two
instants.** The whole purchase of this change is the ORDER of the two
captures (the o1aoi note's §3: the splash with its banner, then a composer
with the text gone). The fake herdr's `pane read` served one file per pane,
so both reads of a fixture pane returned the same bytes, and a writer that
captured once and copied it — or captured twice at the same instant — would
have been green on every assertion worth making. `pane-text/<key>-<n>` now
serves the n-th read of a pane where such a file exists, plain file
otherwise. It is keyed on a file that does not exist, so it is inert in every
other test.

## 3. The exclusions, and where each one is enforced

The amendment excludes the launch line, the pulse, the cockpit's resume and
relaunch's landing turn. Only the first needed a decision in this code, and
it needed none:

- **The launch line** carries no gate reading by CONSTRUCTION and not by a
  check. `launchWithPrompt` asks no settle gate — nothing is typed there
  (ADR 0013 §2) — so `launched.gate` is the zero detection, `fire`'s
  `delivered` arm sets no `pendingBead.gate`, and `stallGate` returns nil
  for a bead that holds none. It could not reach a D5 record anyway:
  `gather` asks `judgeStall` only for `!p.delivered`.

  CORRECTED 2026-10-05 (ranger-base-sua3t's verify, built in
  ranger-base-9k2s1). "Needed no decision" was true of the code and was
  read here as covering the pins too, and it did not: §5's arm 3 pins the
  two ENDS of that chain and not the link between them, because its
  `stallGate` half is driven over a hand-built `pendingBead`. MEASURED:
  `p.gate = d.gateReading(l.gate, l.target)` inserted into `fire`'s
  `delivered` arm reddened nothing in the tree. Arm 6 of
  `falseidle_qa_test.go` drives `fire` itself on the argv path and asserts
  both halves at that site — `p.gate` nil, and no pane read spent. An
  exclusion that needs no runtime check still needs a pin, because what it
  excludes is a COST: one unpriced `pane read` per `prompt: argv` dispatch,
  against an amendment that buys exactly one per typed prompt.
- **The cockpit's `d`** calls `launchSession` and therefore receives the
  gate reading, and spends nothing on it: it takes no capture, holds no
  pendingBead and writes no record. The field it ignores costs one struct
  copy.
- **The pulse and relaunch's landing** touch none of this code.

## 4. No `--dry-run` branch, and why that is a finding rather than an omission

`d3Evidence` exists because a Go argument is evaluated before the function
that would discard it, so a dry-run hold or refusal would fork a `pane read`
for a record the dry run then refuses to write. The type-time capture needs
no such guard: `fireLoop` returns at its `if d.DryRun` arm before `fire` is
called, so nothing is typed, no `pendingBead` exists, and neither capture is
reachable. The difference is real — a dry pass holds and refuses, and it does
not type — and it is stated where each call is rather than left as a missing
branch for the next reader to wonder about.

## 5. The pins

`internal/posse/falseidle_qa_test.go`, arm 2 (`make test-arm2`), beside
`promptstall_qa_test.go` because the mechanism is the stall verdict's. Each
arm names the process it protects:

1. **A typed prompt that stalls writes one D5 record carrying the gate block
   with both captures.** Protects: a false IDLE is diagnosable at all. Drives
   a whole pass to the hand-back verdict, asserts the gate's state, rule and
   herdr working, both captures by name and by content, that the two hold
   DIFFERENT bytes, that D5's own block grew no regions, and that the pass
   spent exactly two pane reads on the pane.
2. **A typed prompt whose turn starts writes nothing.** Protects: no record
   is written at type time, and the held capture is dropped with the bead's
   other in-flight state. Asserts the capture WAS taken first, or the arm
   measures nothing.
3. **The launch-line path carries no gate reading.** Protects the
   amendment's exclusion, at both ends — the launch hands back no reading,
   and the writer handed none writes no block and spends no pane read. With
   the typed path on the same fixture as the control.
4. **The census counts the seen-idle gate readings.** Protects: the number
   ADR 0066 opened with as UNKNOWN is one line of the real script. Reads 1
   over a log with a seen-idle gate, 0 over a log of records with no gate
   block (the shape every D5 record had before this bead), with the D5 row
   and `replayable` unchanged; checks the round trip through `ReadReadings`
   and both captures as corpus region files.
5. **The gate block's captures go through the data ceiling.** Protects ADR
   0050 D2 over the new block, and (b) above over the in-flight evidence.

## 6. Two notes for a reader of the record

**`replayable` is keyed on `herdr` alone, on purpose.** A D5 verdict is a
reading of a herdr wait and a commit count; the gate's captures are two
screens a different reader looked at. Counting them would turn the census's
one quality claim — "this record's verdict can be re-decided from the bytes
it carries" — into a count of records that happen to hold bytes.

**The false-IDLE line prints its residual.** It says `N of M D5 record(s) (K
carrying a settle-gate reading)`, so a D5 record with no gate block (a
launch-line prompt) and one whose gate was not seen-idle (a stall the gate
cannot be blamed for) are both visible rather than folded into the
numerator. The denominator the rate finally needs is the typed-prompt count,
which this log does not hold and the watch log does — 75 in the generation
the design measured.

## 7. The census run, over a fixture log

The bead's own measure-before-closing arm. A FIXTURE and not the fleet:
`scripts/readings-census.py` still reads **0 readings in 0 logs** on this box
for ranger-base-o1aoi §1's reason — the fleet runs a posse that predates the
log — so this is four records in the shape the writer produces (three D5, one
D3), run through the real script. Arm 4 of the pins runs the same script over
a log written by `AppendReading` itself, which is the half a hand-built
fixture cannot claim.

RE-RUN ON THE MERGE-BACK, 2026-10-05 (ranger-base-2xez0). `main` grew a
`truncated` count in this same header line between the two passes
(ranger-base-r5546), so the block below is a recorded output of a script
that had changed — the class `docs/notes.d/ranger-base-dpvlh.md` names, and
the resolution there is to re-run, never to patch the number in. Re-running
the merged script over a fixture rebuilt from this section's own
specification reproduced every figure below unchanged and added `0
truncated`: the fixture sets no region's `truncated` flag — `AppendReading`
derives it from `RegionBytes > len(RegionPreview)` and it is `omitempty` —
so `replayable` is still 1 and §6's rule still reads the way it did. That
every OTHER number came back identical is what makes the rebuilt fixture
the same fixture.

```
readings: 4 in 1 log(s) · 1 replayable · 0 truncated · 0 redacted

per day
  day         total  refusal  hand-back  settle-open  ghost-retirement  hold
  2026-10-04      4        1          1            0                 0     2

per decision
  decision                                        total  refusal  hand-back  settle-open  ghost-retirement  hold
  D3  unknown screen (what was herdr looking at)      1        1          0            0                 0     0
  D5  stall verdict (rewait/keep/hand back)           3        0          1            0                 0     2

  false-IDLE candidates: 1 of 3 D5 record(s) (2 carrying a settle-gate reading)
    a candidate is a stalled prompt whose gate read a screen it had SEEN, as idle
    — the screen from each side of the keystrokes is in its gate regions (ADR 0066 D1)

regions read
       2  pane_capture
       1  footer
       1  pane_capture_at_prompt
```

The three D5 rows are the three cases a reader has to be able to tell apart,
and the line reads them as 1 of 3 (2 with a gate): a seen-idle gate — the
candidate; a gate herdr did NOT see, which is a stall the gate cannot be
blamed for and is already a D3-shaped finding; and a record with no gate
block at all, which is a prompt that rode in on a launch line. `replayable`
is 1 and not 2, which is §6's rule: the D5 record's bytes are the gate's, and
the D5 verdict is not re-decidable from them.

## 8. One boundary the amendment's wording leaves, said out loud

The count is **seen and `idle`**, which is the amendment's own phrasing. The
settle gate accepts `done` as well (`awaitSettled`'s `until`), so a gate that
read a rule-matched `done` over the wrong screen is the same class and is not
in the numerator. It is not invisible: the line prints the D5 total and how
many carry a gate block beside the candidates, so that record is in two of
the three figures. Widening the rule belongs to whoever reads the first
fourteen days of real census (ADR 0066 D1's §5 third bullet) and not to this
build — a numerator nobody decided would price the error rate of a reading
that was never named.
