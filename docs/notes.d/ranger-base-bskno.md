# The second route to a cross-cutting lane, and this one no order reaches (ranger-base-bskno)

2026-11-14 · gwart · linux 7.2.15 x86_64, go1.27.3, this box, at 0ee3ae34

ADR 0011 §4, as amended under ranger-base-svoux, names ONE consequence of the
head PID's label order and ONE remedy for it: "a PID that lists a
cross-cutting label first makes that label its lane, and every PID carrying it
a seat … That is a configuration error and its remedy is the order, not code."

There is a second route to the same lane. `labelMatches`
(internal/posse/dispatch.go) takes the lane from `out[0].label`, which is
`matched[0]` — the first of the preferred PID's labels the bead carries. When
that PID's `matched` set holds exactly ONE element, that element is the lane
whatever order any PID lists its labels in, because there is nothing to order.
Where the single element is a cross-cutting label, the lane is the
cross-cutting label and the seats are every PID carrying it.

§4's remedy has already been applied everywhere on this instance and the
misroute still happens. That is the whole finding.

## Measured

RE-MEASURED 2026-11-14 at 0ee3ae34, arm 2, through the package's own helpers
(`laneOf`, `writePersona`), over a roster where every PID lists its own lane
label first. `team` is §4's spelling for the real label several of this
instance's PIDs share: that is the instance's own vocabulary, so it is named
on the bead and not here (ADR 0024 D3, ADR 0048 D2 — both gwart and richard
were refused by the visibility gate for the real literal, under
ranger-base-5793u and ranger-base-svoux).

Roster: `aaa-dev [code, feature, bug, team]`, `ops-seat [devops, infra, ci]`,
`watcher [security, audit, vuln]`, `zz-arch [architecture, design, team]`.

| bead | lane | label | dropped |
|---|---|---|---|
| `[devops, team]` | aaa-dev, zz-arch | `team` | **ops-seat** — the only devops seat |
| `[security, team]` | aaa-dev, zz-arch | `team` | **watcher** — the only security seat |

Reordering the head's labels moves neither row — read over three orders of
`aaa-dev`'s four labels, its lane label first, last and alphabetical — because
`matched` is `[team]` however it is spelled.

This is sharper than §4's stated error and sharper than ranger-base-equhp's
incident. equhp was a lane that SOMETIMES reached a seat which could only hand
the bead back. Here the lane holds no seat that can do the deliverable at all,
so the bead cannot be seated on a capable seat on any pass. The dropped seat's
own label is not stranded, which the pin asserts as its control: a `[devops]`
bead is `ops-seat`'s lane as it always was, and only the deliverable that ALSO
carries the cross-cutting label loses it.

### Two controls, by mutation

Both by mutating `labelMatches` and reverting after the reading:

| mutation | result |
|---|---|
| pre-narrowing (`return out` for `return lane`) | all four arms RED: lane reads `[aaa-dev ops-seat zz-arch]` / `[aaa-dev watcher zz-arch]` |
| §4's rejected alternative (`win` as the alphabetically first of `out[0].matched`) | all four arms GREEN and byte-identical |

The first says the EXCLUSION half is new with ranger-base-equhp's narrowing —
before it the capable seat was always in the lane and only the printed label
was wrong, which is what ranger-base-5793u established for the
frontmatter-order half. The second says the shape is orthogonal to the (a)/(b)
choice §4 settled: a one-element matched set has the same first element in
either order, so neither the decided rule nor the rejected one reaches it.
This is not a reopen of ranger-base-svoux; it is a case §4 does not cover.

### The live roster, re-read

RE-MEASURED 2026-11-14 over this store (`bd list --all --json --limit 6000`,
so the page limit is named): **0 of 3,851 beads carry the cross-cutting label
at all**. laurie read 0 of 3,795 the day before, so the store has grown by 56
beads without one. The roster behind the two rows above holds on the real
PIDs: 11 of them, 5 declaring the label, every one of the 11 listing its own
lane label first, the shared label never first on any of them, exactly one
devops PID and exactly one security PID. So the defect has never fired, and
ONE bead filed with the label plus a lane label turns a row on.

## What landed here, and what did not

ADR 0011 §4's text is the only place the remedy can be written, so the
decision is the architecture lane's (ADR 0006 §2): **ranger-base-d7lsc**,
carrying the measurement, both controls, the three shapes and gwart's
recommendation, with ranger-base-bskno dep-blocked on it. This bead carries no
ADR diff.

What it does carry, all of it over unchanged logic:

- `TestQALaneIsTheCrossCuttingLabelWhenTheHeadMatchesOnlyThat`
  (internal/posse/lanewinninglabel_qa_test.go, arm 2) — the two rows, the
  three head orders, and the dropped seat's own lane as a control. A
  CHARACTERIZATION pin, the same posture arm C's table went in with under
  ranger-base-5793u while ranger-base-svoux was out. It fails BY NAME on the
  stranded seat appearing in the lane, so the remedy cannot land without
  someone rewriting the table deliberately — which is the point: under (b) it
  stands as written, under (a) or (c) it must become the table the finding
  asked for, the same roster with the lane HOLDING the seat that can do the
  work.
- the scope correction. Arm C's header and `routeMatch`/`labelMatches`' doc
  comments all stated §4's remedy without its scope — "its remedy is the
  PID's order, not code" — over a route the order cannot reach. That is the
  live finding ADR 0006 §6 gave this bead (ranger-base-v38q1): the pin was
  wrong about what it measures. Each now says the order is a key where the
  head matches more than one label, names the one-element route, and cites
  ranger-base-d7lsc for the open remedy. Arm C's fixture cannot reach the
  second route — its head matches on two labels, so the order really does
  decide there — which is why a new pin and not a new arm.

## Recommendation, offered and not decided

(b) now, (c) when the route report first shows it. §4 states its own trigger
for (c) — "if the error is ever observed in a route report" — and it has not
been observed: no bead carries the label. (b) is the only shape that is honest
at zero risk to a routing path 3,851 beads currently take; (a) without (c)
needs the static distinction between "cross-cutting" and "legitimately shared"
that §4 says in as many words does not exist, and (c) is a new PID key, which
is a seat-visible surface change for a defect that has never fired.
