# ranger-base-xim7t — a fixture that cannot separate the two rules

Verifying four closes turned up three findings with one shape between them, and
the shape is worth more than any of the three: **the fixture a pin ships with
decides which of the code's rules it actually pins, and a plausible fixture
often pins the weaker one.** All three pins were green, all three were written
by careful people, and in each case a mutant of the rule the pin's own comment
names left it green.

The question to ask of a pin is therefore not "does it fail when the code is
wrong" — it will, for *some* wrong — but **"which of the code's rules does this
fixture distinguish?"** Then build the fixture that distinguishes the rest.

Two of the three were pin gaps in `internal/posse` and are fixed here. The
third was in ranger-core's production code and went to the code lane as
ranger-base-me10w.

## 1. An unparseable fixture does not separate "the maximum" from "the last one"

`pulseFeedWrite` (`internal/posse/pulse.go`) takes a new entry's sequence
number as one past the **maximum** parseable one in the file:

```go
if n, ok := PulseFeedSeq(ln); ok && n >= seq {
	seq = n + 1
}
```

Its comment says why: "a corrupt tail would otherwise restart the sequence and
hand an armed waiter numbers it has already passed, which looks exactly like
nothing happening." `TestPulseFeedSequenceSurvivesACorruptTail` asserts it, and
its fixture appends `not-a-seq whatever`.

That line is **unparseable**, and `PulseFeedSeq` rejects it under either rule.
So the max rule and plain last-parseable-wins both answer 4 over that fixture,
and the `>= seq` half was asserted by nothing.

MEASURED 2026-11-14, linux, this box: with `ok && n >= seq` cut to a bare
`ok` and nothing else changed, the whole arm-3 pulse filter was green
(`go test -tags posse_arm3 -count=1 ./internal/posse -run '^(TestPulse|TestQAPulse|TestQAEmptySet|TestQAOldPulseState|TestQAHalfReadable|TestQAUnreadableDelivery|TestQAWritingOverAnOld|TestWatchPulse|TestShopCheck)'`
→ ok). Twelve other mutants over the same diff were killed by that same filter,
so the filter was not the weak part.

What separates the two rules is a **parseable** line whose number is lower than
one above it — entries `1, 2, 5, 3`. Out of order rather than corrupt, so there
is nothing for a parse to reject: max answers 6, last-wins re-issues 4, and 4
is a number a waiter on `--after 5` has already passed. That arm is now in the
test and it kills the mutant.

Worth saying beside it: a corrupt tail is not reachable through this writer at
all, because the rewrite is a rename (below). What the guard really protects
against is the out-of-order case, which is the fixture that was missing.

**The class: "corrupt" and "out of order" are different fixtures, and a guard
written against "a bad tail" usually needs the second one.**

## 2. An absence assertion on a temp file passes for a writer that creates none

`pulseFeedWrite` writes `path + ".tmp"` and renames, because this file has a
reader that is not its writer — `posse pulse wait` (ADR 0093 D1,
ranger-base-cgr1h) may be mid-read at any moment, and a reader of an
append-then-truncate feed can see a file with its oldest lines gone and its
newest not yet there.

`TestPulseFeedKeepsTheNewestEntriesAndKeepsCounting` closed with:

```go
// The rewrite is atomic, so it leaves nothing half-written beside it for
// a waiter to trip over.
if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
```

A writer that never creates a temp file at all satisfies that. MEASURED
2026-11-14: with the temp+rename replaced by a direct `os.WriteFile(path, …)`,
the same filter was green — so the one property the feed has for its reader's
sake was pinned nowhere.

The positive control is the temp path itself. `os.Mkdir(path + ".tmp")` plants
a path `os.WriteFile` cannot write, so **a writer that goes through it must
fail and a writer that does not must succeed**. The new
`TestPulseFeedRewriteGoesThroughATempFileAndARename` asserts that, then the
atomicity it buys (the feed is byte-identical after the failed write, never
half-replaced), then removes the plant and re-runs so the red is the plant and
not something about the fixture.

**The class is the house rule about absences, one layer in: an absence is true
of nothing until something in the same test makes it false. The absence of a
side effect is pinned by making the side effect's own path refuse.**

## 3. A rule split across two layers needs a pin where the narrow layer binds

This one is ranger-core's and is written up on ranger-base-me10w; it is here
because it is the same shape and it is the most expensive of the three.

`stored_candidate_articles` bounds its Cypher scan at the **loosest**
`freshness_days` any source entry asks for, with
`ORDER BY effective_date DESC ... LIMIT $limit`, and then applies the exact
per-entry cutoff in Python — deliberately, because the unit suite drives a
MagicMock and an arithmetic rule in Cypher would be in the one layer no test
can reach. That reasoning is sound.

Both halves had their own passing test. Neither put the `LIMIT` under any
pressure, so nothing pinned the interaction: a strict entry with enough rows
between its own cutoff and the loosest one fills the window, Python drops every
one of them, and a looser lane's eligible backlog was never returned by the
query at all. MEASURED at **0 articles returned** where the lane should serve
backlog, with the two freshness values the shipped config already has.

**The class: whenever a cheap query pre-filters for an exact filter downstream,
the pin that matters is the one where the pre-filter's LIMIT binds.** Two
passing tests either side of the seam say nothing about the seam.

## Two more, cheaper, recorded rather than fixed

- A **vocabulary grep is evidence about the vocabulary.** A scoping bead asked
  its note to price "the gates"; `grep -i gate` over the note returned nothing
  and the finding was half-written before reading the section, which prices
  them under the house's other name — "PID denies … are argv shims on that
  account's PATH". Read the section before calling a deliverable incomplete.
- **Count a census against its own breakdown.** A measured reading said
  "8 sessions" and then enumerated 6 + 1 + 2. It needs no re-run to check and
  it is the one arithmetic a later reader cannot redo.

## Method

Both mutation batteries ran over **copies** of the two trees in the session
scratchpad, never the live ones, so a battery and a full `make test` could be
in flight at once. Every batch opened with an unmutated control (green in all
three posse arms and over ranger-core's 226-test subset) and md5'd the target
before and after each plant, so a no-op edit could not score as a kill — which
caught one of mine that was a no-op by Python semantics (`set() or X` is `X`)
and would otherwise have been reported as a surviving mutant and a finding that
was not there.

25 mutants over the three code closes — 11 over `pulse.go`, 4 over the
launch-line census, 10 over ranger-core's selection code. 23 were killed on
arrival; the 2 that survived are §1 and §2 above.
