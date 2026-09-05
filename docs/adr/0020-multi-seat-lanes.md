# ADR 0020 — Multi-seat lanes: a lane is a label set, a seat is a persona, selection is availability-first at dispatch time

*Status: superseded 2026-09-05 by ADR 0011 · ADR simplification, operator ruling 2026-09-05.*

The surviving decision is in [0011 — current contract](0011-dispatch-model.md), Decision §§4–5 and Lineage. This page keeps its number and dated evidence; the body below is historical, not current policy.

## Historical record (superseded in full)

*Status: accepted 2026-08-27 · owner: richard · amends ADR 0011 (the
routing rule and the pass's caps); the ADR 0006 §3 wording change for
the batched verify gate is tracked separately (ranger-base-bh7q) ·
ranger-base-4ur5 · §2 amended 2026-08-27: seat selection binds every
launcher, not only the pass (ranger-base-f8m9)*

## Context

2026-08-27 the crew went multi-seat: code is dinesh + gwart + jian-yang,
QA is laurie + holden, and the nine generic example PIDs are retired
(ranger-base-1t7r, placements on ranger-base-j0wr). Five dispatcher
behaviours are correct for one persona per lane and undefined for three
(ranger-base-4ur5): alphabetical unassigned routing elected holden over
laurie on day one; `verify_assignee:` is a scalar so a second QA seat
gets no automatic work; a persona is serial by an emergent property of a
busy key; the pass caps were sized for 8 seats; and the verify gate's
1:1 fan-in was the queue's amplifier (rho = 1.14 — MEASURED, 1t7r).
These are one unmodelled concept, twice over: **selection among peers in
a lane**, and **lane concurrency as distinct from persona concurrency**.

Constraint the operator set: design for THROUGHPUT (closes per day), not
utilisation or token cost. Cost is measured non-binding (a measured
per-bead cost, windows 5h 13% / 7d 15% — 1t7r).

## Decision

**1. Vocabulary. A LANE is a set of labels; a SEAT is a persona whose
PID `labels:` intersect them.** Lanes stay emergent from labels — no
lane registry, no new config. The roster IS the lane table.

**2. Selection among peers happens at dispatch time, availability-first,
with name order as a stated tiebreak.** Route() splits into two
questions it currently conflates:

- *Which lane?* An explicit assignee is a lane of one and never falls
  through (unchanged — silent rerouting hands work to the wrong actor).
  Otherwise the candidates are every non-coordinator persona whose
  labels intersect the bead's, in persona-name order (today's ReadDir
  order, agents.go ListAgents — now a documented tiebreak instead of an
  accidental priority scheme).
- *Which seat?* The fire loop seats the bead on the FIRST AVAILABLE
  candidate: not made busy earlier in this pass, no working/blocked
  session in the repo (personaActive), not held by a crew session. All
  seats busy → the bead waits for a later pass, and the report names the
  lane ("code lane busy: dinesh, gwart, jian-yang"), not one persona.
- The route report must say why a seat won — "label:code (seat 2/3:
  gwart; dinesh busy)" — the audit line ranger-base-2yj5 asked for.
- `--persona X` restricts seating to X: a bead whose lane contains X
  may seat only there, others are skipped as today.
- *Which launchers?* (amended 2026-08-27, ranger-base-f8m9) **All of
  them.** The original text said "the fire loop", and `LaunchBead` (the
  cockpit's `d`) kept taking Route's single head — an operator pressing
  `d` on an unassigned code bead always got the lane's first name and a
  "working — not dispatched" refusal while other seats sat free. A bead
  with NO HOLDER is seated availability-first wherever it launches from:
  LaunchBead answers WHICH SEAT with the same walk the pass uses —
  laneFor, then seatFor under the launcher lock it already holds, empty
  bench, no filter. All seats busy → the lane-busy line replaces the
  one-persona refusal. This also puts §4 in front of `d` for fresh
  launches: the old path read only the target session's status, so `d`
  could fan a persona two-wide by launching a second bead at a persona
  working elsewhere in the repo — the same class as a cap the pass
  honours but the `d` key walks past (the Dial E guard in the same
  function), and it closes the same way.
- *`d` on a holder never reseats.* An in-progress bead has no seat
  question: `d` there is resume, and the seat is the HOLDER — the
  assignee's joined session, exactly as today (an assigned bead is a
  lane of one by §2.1, so this is structure, not a carve-out). When an
  unclaim erased the assignee under a live run, the record answers
  before availability does: narrow the lane to the seat whose run
  record (`bead:`, ADR 0011 §3) names this bead; a hit is a lane of
  one. Only no assignee AND no record seats by availability. The guards
  after seating keep their bead-level classification (crew-held,
  settled holder, prompt grace — seatFor's own comment): falling
  through any of them hands one bead to a second persona.

Name order is a TIEBREAK, not a priority, and that is why it is enough:
under availability-first it decides only who takes the first bead when
several seats are free; the next bead in the same pass overflows to the
next seat. A PID named `aaa` gains a one-bead head start, not a lane.
Selection runs inside the launcher lock against the same liveness reads
the busy-skip already uses, so it changes who is asked, not the
concurrency invariant.

**3. Assignment is dispatch's job, not filing's.** A harness filer
cannot know who will be free when the bead is dispatched — filing-time
selection is a check-then-act whose check and act are hours apart.
verify-after therefore files UNASSIGNED by default (already the code's
default; DefaultVerifyAssignee is ""), and §2 seats the bead when it
fires. `verify_assignee:` survives as an operator PIN — a one-QA shop,
or a deliberate choice to serialize the verify lane through one persona
— not as the fan-out mechanism. The live config's `verify_assignee:
laurie` becomes a pin the moment this ADR lands: keeping it must be a
choice, and lifting it is placement (monica's, with the operator), once
§2 is built.

**4. A persona is one serial seat — ratified, now by decision.** "One
bead per persona per repo per pass" plus the personaActive skip
(dispatch.go fireLoop) makes a persona strictly serial fleet-wide in a
one-repo shop. That stays, because a persona is the SINGLE WRITER of its
identity: its ORDERS.md memory, its session namespace, its bd actor
line, its metrics. Fanning one persona N-wide makes every one of those a
multi-writer store. Therefore **lane concurrency == seat count, and
hiring is the concurrency knob** — deliberately: adding concurrency is
an operator act (a PID file in git), visible, attributable, reversible.

**5. The width caps stay denominated in beads; the coupling becomes law
instead of folklore.** An epoch's effective width is

    min(autostart_max_beads, floor(budget_pass / cost-per-bead), free seats with ready work)

so raising one bound without the others does nothing (MEASURED today:
cap vs budget/a measured per-bead cost worked out lower than seats
hired). The caps are blast-radius
and spend bounds, and spend is incurred per bead fired — beads are the
unit both are actually denominated in. A seat-denominated cap would
re-denominate spend authority so that a hire silently raises it, a
ratio change nobody decided (the same principle that keeps
`verify_batch` out of the seed config). Sizing rule for the operator:
`autostart_max_beads` ≈ seats you want started per epoch;
`budget_pass` ≥ that × measured per-bead cost. The formula is this
ADR's; the numbers are the operator's (question bead filed).

*Amended 2026-08-27 by ADR 0028 §2 (ranger-base-f0y3): the unit of both
bounds was THE PASS and is now the wall-clock epoch (`dispatch_epoch:`,
default 1h). Nothing else in this section moves — the same three bounds,
the same beads denominator, the same reason. The re-denomination is what
keeps the law true once the pass stops being a unit of time at all: ADR
0028 §1 makes the dispatch `Run` long-lived, so "per pass" would have
become "per evening" and this width would have bounded nothing. The
config key `budget_pass:` is unchanged; the window it names is the
epoch, and the output says so.*

**6. The verify gate's fan-in is batched — ratified as built**
(`verify_batch: N`, commit 8b7bed9, ranger-base-f7pk). One verify bead
per N closes divides the FILING amplification (the qa → code 0.49 leg
fires per batch, not per close) while verifying the same closes. I own
the partial-batch call dinesh made: **hold, bounded by
`verify_batch_age:` (default 24h)** — filing short makes N a ceiling
(most passes see one close, so N=4 would file 1:1 and buy nothing);
holding unbounded starves the tail when the shop goes quiet. An age
bound is the standard batching answer and neither failure survives it.
N stays 1 until the operator takes ranger-base-bah7 decision 2.

## Consequences

- A pass with three unassigned code beads and three free code seats
  fires all three; today it fires one and marks the lane's only routable
  persona busy. This is the throughput change, and it needs no new knob.
- holden's queue self-feeds once §3's pin is lifted; until then the pin
  is visible policy instead of a leftover scalar.
- Selection needs no new state and no new store: the candidate walk is a
  pure function of the roster, and availability is the liveness read the
  pass already makes under the launcher lock (ADR 0011 §1).
- The exit hatch if a real seniority preference ever shows up: an
  ordering key in PID frontmatter, composing with §2 (order by key, then
  name). Not built now — no evidence any preference exists (ASSUMED, and
  cheap to be wrong about: the tiebreak decides one bead per pass).
- ranger-base-2yj5 is retargeted to implement §2 (its generic-PID
  premise died with the retirement); a placement bead for §3's pin goes
  to monica; the §5 numbers go to the operator as a question bead.
- (f8m9) The cockpit's ready-work row keeps saying `unassigned`
  (MEASURED: issueCols renders the assignee or that word; no cockpit
  surface ever rendered a routed prediction — holderSession joins only
  IN PROGRESS rows, on assignee). That display is §3 in miniature: a
  seat predicted at render time is filing-time selection in a smaller
  window. Dispatch answers at `d`, and the result line's session name
  carries the persona who won.
- (f8m9) Accepted cost: two `d` presses inside PromptGrace on two beads
  of one lane may refuse the second instead of overflowing — herdr's
  lag hides the first launch from personaActive, then the grace guard
  refuses the chosen seat. Same classification the pass gives the grace
  (a bead skip, not a seat skip); the retry lands once herdr catches
  up. Window ASSUMED narrow (title-spinner latency, the pass lives with
  the same lag).
- (f8m9) `d`-resume on a holder's idle session while that persona works
  another bead still re-prompts — the operator's hand can run a persona
  two-wide on the RESUME path only. Known, out of this amendment's
  scope: whether §4 should bind an explicit resume is its own decision,
  and closing it here would change resume semantics nobody complained
  about.

## Alternatives rejected

- **Round-robin among peers.** Needs a cursor — a new store shared by
  three launchers, inside the lock's scope. Buys fairness that
  availability-first already provides when beads outnumber seats, which
  is the only regime this shop has seen (COST ASSUMED small, rejected on
  ADR 0011's fewer-stores principle).
- **Least-loaded (fewest open assigned beads).** Queue depth is a proxy
  read at selection time for a fact (who is free) the pass can read
  directly — and it is the wrong proxy: personas are serial, so an
  assigned backlog is not a busy seat. The live session IS the load
  signal at the only grain that matters.
- **`verify_assignee:` as a list, rotated at filing time.** Moves
  selection to the moment availability is unknowable, needs rotation
  state, and every future harness filer would need the same machinery
  again. One selection mechanism, at dispatch, covers all filers.
- **Per-persona concurrency (`max_concurrent: 2`).** The clever one I
  wanted: personas as stateless workers over a seat pool. It dies on the
  fact that a persona is its state — N concurrent sessions writing one
  ORDERS.md and one metrics line is a multi-writer corruption of the
  thing the shop is actually accumulating.
- **Seat-denominated pass caps.** A hire would silently raise spend
  authority (§5); the caps exist to make exactly that impossible without
  an operator's hand.
- **A lane registry (config `lanes:`).** A second copy of what PID
  labels already say, and the two would drift; the roster is the lane
  table.
- **(f8m9) Reading (b): `d` launches the persona the row displayed.**
  Priced against the display, and the display shows no persona for
  exactly the beads this amendment moves (MEASURED: the ready row says
  `unassigned`). For the rows that DO show one — IN PROGRESS holders,
  assigned beads — (b) is already enforced structurally (lane of one,
  plus the run-record narrow). Choosing (b) for unassigned rows would
  preserve a prediction the UI never made.
- **(f8m9) A lane column in the ready-work row** ("code·3" instead of
  `unassigned`). A roster walk per refresh to predict what dispatch
  decides one keypress later, and it evicts the assignment-state signal
  the column exists to show. Nothing needs it to act (COST ASSUMED
  small; rejected on value, not cost).
- **(f8m9) Grace-as-seat-skip in LaunchBead** (seed the bench from the
  prompt records so a just-prompted seat overflows). Diverges from the
  pass's own classification of the grace, for a window herdr closes by
  itself; one behaviour, honestly refused, beats two.
