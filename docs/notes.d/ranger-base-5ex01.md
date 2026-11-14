# A candidate heads only on an anchor, and the lane's label stopped being the head seat's own (ranger-base-5ex01)

2026-11-14. The code half of ranger-base-d7lsc's decision, which is ADR 0011
§4 as amended that day (commit 8a0a93d1). The finding is
ranger-base-bskno's: a head candidate whose matched set was one label it
shares with another lane took that label as the lane whatever order any PID
listed its labels in, so a `[devops, team]` bead became a `team` lane holding
neither the only devops seat nor any seat that could do the work. §4's stated
remedy — the PID's label order — cannot reach it, because a one-element
matched set has nothing to order.

## The rule, as built

`labelMatches` (internal/posse/dispatch.go) now reads the roster as lanes in
the same `ListAgents`/`LoadAgent` walk it already made:

- a label's **lanes** are the first labels of every non-coordinator PID
  carrying it. One entry means the label belongs to one lane however many PIDs
  carry it; two or more and matching it alone says nothing about whose work
  the bead is. The coordinator is excluded from the counting exactly as she is
  from candidacy, so a label she carries for triage cannot read as a lane.
- a candidate's **anchor** is the first of its matched labels, in its PID's
  order, that is its own first label or belongs to exactly one lane
  (`routeMatch.anchor`).
- the head is the first candidate in the `route_order` sort that HAS an
  anchor, and the lane is that anchor. With no anchor anywhere — a bead
  carrying nothing but the shared label — `out[0]`'s head label is the lane,
  as before. Seats are unchanged: every candidate carrying the lane label.

Both arms of the anchor are load-bearing and neither subsumes the other. The
own-first-label arm is what leaves §4's first configuration error with the
remedy it has: a PID that lists the shared label FIRST declared that as its
lane and gets it. The one-lane arm is what keeps an alias working, and it is
the measurement that rejected primary-first: `bug` on a code PID is not that
PID's declared lane, but if every PID carrying `bug` is in the code lane then
matching it is a code match, so a `[bug, security]` code fix stays with the
code seat instead of going to a findings-only one (ranger-base-d4bk4, which
is ranger-base-equhp's shape reached a third way).

## What the rule reached that the bead did not name

`routeLane.label` and `routeWhy` read the lane's label off `seats[0].label`.
Until the anchor they could only agree, because the winning label WAS
`out[0]`'s own head label. Under the anchor they can differ: the anchored
candidate need not be the first the sort offers, and the first one can carry
the anchor second while its own head label is another lane's. Reading it off
`seats[0]` then printed a `label:` the lane was not chosen on — the same
question ranger-base-2yj5 built the why line to answer, answered with the
wrong label. So `labelMatches` returns the winning label beside the seats, and
`routeWhy` takes it as an argument. Pinned by
`TestQALanesLabelIsTheAnchorNotThePreferredSeatsOwnLabel`: a two-PID roster
where the preferred seat's own head label is `team` and the lane is `docs`.

## Measurement

MEASURED 2026-11-14 (linux 7.2.15 x86_64, go1.27.3, scratch census over `bd
list --all --json --limit 6000` and this instance's 10 non-coordinator PIDs),
carried from ranger-base-d7lsc's decision: 3,856 beads, 3,474 with a
candidate, and the anchor changes **0** of those routes. Primary-first changes
2. Exactly one label on the live roster spans more than one lane, and 0 beads
carry it, so the stranded rows are a shape the roster permits and no bead has
reached.

## The pin, and the two directions the fix can be wrong

`internal/posse/lanewinninglabel_qa_test.go` (arm 2).
`TestQALaneIsTheCrossCuttingLabelWhenTheHeadMatchesOnlyThat` — the
characterization pin ranger-base-bskno landed over the old behaviour — is
replaced by `TestQAHeadAnchorsOnItsOwnLaneOrASingleLaneLabel`, the table that
pin asked for: the same roster, and the lane HOLDING the seat that can do the
work. Nine rows: the finding's two over two head orders, the first
configuration error still making `team` the lane when the head lists it
first, a bead carrying only the shared label (nothing to anchor on), the
head's own lane winning with and without another lane's label on the bead,
and the alias row. The capable seat is asserted BY NAME, so a regression says
which seat went missing rather than only that a set differed.

VERIFIED by mutating `labelMatches` and reverting, both directions a rule like
this can be wrong:

- `win` back to `out[0].label` (the pre-anchor body): rows 1-4 red, reading
  `aaa-dev,zz-arch` (`team`) — the measured before-state — and the label pin
  reds with `team` for `docs`.
- the anchor as primary-first (`l == m.primary` alone): row 9 red, the
  `[bug, security]` code fix leaving the code seat for the findings-only
  security PID. That is the live route ADR 0011 §4's Alternatives rejected
  primary-first for, so the pin now holds the rejection and not only the
  decision.

`team`, `docs` and the fixture seat names are §4's spellings; the instance's
real label stays off tracked files (ADR 0024 D3, ADR 0048 D2).

## ranger-base-bskno's commit was rescued, and will not self-retire

ranger-base-bskno is dep-blocked on ranger-base-5ex01 and closes after it, so
its commit `ba014436` was still unlanded on `posse/gwart-posse-ranger-base-bskno`
when this bead was dispatched — and the doc comments and pin this bead
REWRITES existed only there. It is cherry picked onto this branch as its own
commit before the rewrite. Three of its four files applied clean;
`docs/notes.d/README.md` is generated, so it was regenerated with
`python3 scripts/notes-index.py` instead of taken from the patch. That changes
the commit's patch-id, so a replay of bskno's branch will NOT drop `ba014436`
as a duplicate: its close should read `git log main --grep bskno` first and
drop the commit rather than replay it.
