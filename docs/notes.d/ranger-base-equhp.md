# A lane was the union of every persona sharing ANY of a bead's labels (ranger-base-equhp)

2026-11-13 · dinesh · linux 7.2.15 x86_64, go1.27.3, this box

`laneFor`/`labelMatches` (internal/posse/dispatch.go) took every
non-coordinator persona whose PID `labels:` intersect the BEAD's labels as one
candidate list, sorted it by `route_order:` then persona name, and let
`routeWhy` name the whole thing after the first candidate's label. A bead
carries the labels of all the work it touches, so a bead labelled
`[code, debt, security, vuln-private]` — a code deliverable discovered from a
security finding, which is the shape every such finding has by convention —
read as ONE `label:code` lane containing the security PID at its tail.

MEASURED 2026-11-13 in the dispatch watch log, over ranger-base-g8fmb, on an
instance with three code seats and one security seat:

- a four-name `code lane busy:` line on the pass where every seat was spent;
- then, with the three code seats busy, `label:code (seat 3/4: …)` onto the
  SECURITY seat, a session created on it, and the bead prompted — twice in one
  night, at strong tier both times.

That PID denies `Edit` and `Write` (findings only), so the seat cannot do a
code deliverable. It can only hand the bead back, which spends a strong-tier
seat and requeues the bead to lose the same race again on the next pass. The
cost is per occurrence and the shape recurs on every security finding routed
to the code lane.

## The fix is about the LANE, not about Edit/Write

The seat was wrong before the PID's denies were read, because the lane was
wrong. ADR 0020 §1 (folded into ADR 0011 §4) says a LANE is a set of labels
and a SEAT is a persona whose PID labels intersect THEM — and the bead's own
label set is not that lane. The alternative considered and rejected was a rule
excluding an Edit/Write-denied PID from a `code` lane: that is a second
special case over the same defect, it leaves the lane-busy line and the "first
of N" roster still counting a seat that cannot take the work, and it says
nothing about the next pair of labels that merge.

So `labelMatches` now narrows: it still sorts every overlapping persona by
`route_order:` then name, and then keeps only the personas carrying the
PREFERRED candidate's label. A bead labelled `[code, security]` whose code
seats are all busy waits for a later pass, which is what a busy lane does
(ADR 0020 §2). It does not overflow into the security PID.

Two details the narrowing has to get right:

- **It reads the PID's whole matching set, not the one label the walk
  recorded.** A PID carrying both `qa` and `code` matches on whichever it
  LISTS first; dropping it from a `code` lane for that would be an ordering
  accident inside someone's frontmatter.
- **The roster is the lane.** `routeWhy`'s "first of N" and the `code lane
  busy:` line are computed from the narrowed list, so an operator reading the
  hiring signal (ADR 0020 §4: lane concurrency is seat count) is not reading a
  count one too high.

## What decides WHICH label wins

The preferred candidate's — so `route_order:` ascending, then persona name,
which is the dial ADR 0011 §4 already states and the only one. Narrowing makes
that choice decide the whole lane rather than only its head: an instance where
a `[code, security]` bead should go to the security seat says so with the key,
and then gets a lane of security seats rather than a merged one. Pinned by
`TestQARouteOrderPicksWhichLabelWinsTheBead`.

This leaves a residual that was there before this bead and is unchanged by it:
with every `route_order:` tied, the winning label is whichever lane's
alphabetically-first persona sorts first across all matching PIDs. The union
had the same head, so the fix neither creates nor widens it — it only means
the head now takes the lane with it. The answer is the key, not more code.

## Pins

`internal/posse/lanewinninglabel_qa_test.go` (arm 2), five tests. Each was
confirmed RED against the pre-fix `return out` and green after:

| test | what it would catch |
|---|---|
| `TestQALaneIsOneLabelNotTheUnionOfTheBeadsLabels` | the lane itself: `[code, debt, security, vuln-private]` is the code seats, and a `[security, vuln-private]` bead still reaches the security seat |
| `TestQARouteWhyCountsTheLaneNotTheOverlap` | "first of 2", and the security PID absent from the roster |
| `TestQABusyCodeLaneWaitsAndDoesNotSeatTheSecurityPID` | the incident, over a whole pass: two seats fire two beads, the third WAITS, and the lane-busy line names two seats |
| `TestQARouteOrderPicksWhichLabelWinsTheBead` | the dial above |
| `TestQALaneKeepsAPIDThatCarriesTheWinningLabelSecond` | the frontmatter-ordering accident |

## Not closed here

- ranger-base-g8fmb (the code deliverable the misroute was costing passes on;
  it is still open and still needs a code seat)

## Correction (2026-11-13, ranger-base-5793u)

The residual paragraph above ("What decides WHICH label wins") is right about
one half and wrong about the other.

Right: with every `route_order:` tied, which lane's alphabetically-first
persona heads the sort was the same before this fix and after it.

Wrong: "the fix neither creates nor widens it". Before the narrowing, the
preferred PID's own label order moved only the label the route report PRINTED;
every overlapping persona was in the lane either way. After it, the same input
decides MEMBERSHIP (dispatch.go:1808) — so a head PID that lists `qa` before
`code` drops a code-only seat from a code bead, and one that lists a
cross-cutting label first puts the findings-only security PID in a code lane,
which is this fix's own incident shape reached the other way. That consequence
is new here. Three arms, re-measured and pinned:
`TestQALaneWinningLabelTurnsOnTheHeadPIDsFrontmatterOrder`,
docs/notes.d/ranger-base-5793u.md.

Also: "an instance where a `[code, security]` bead should go to the security
seat says so with the key" still holds, but the key is not the only dial —
`route_order:` changes WHO is preferred, never which of that PID's labels
wins.

SETTLED: ADR 0011 §4, amended 2026-11-13 (ranger-base-svoux, commit
70ab74b7). The head PID's label order is the SECOND routing key, not an
accident: a PID lists its primary lane first, `route_order:` decides who
heads, and the head's order decides which of its labels is the lane. A
follower's order still decides nothing, which is what this fix pinned. So
this fragment's "the answer is the key, not more code" was right in kind and
one key short, and the sentence it qualified — that the residual is unchanged
by this fix — stays wrong for the reason above. Listing a cross-cutting label
first is a stated configuration error with the order as its remedy; §4 has
the measurement (20 of 20 PIDs already list their lane first) and the
`lane:` exit hatch.
