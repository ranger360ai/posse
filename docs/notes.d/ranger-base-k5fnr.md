# What the mandatory spike bought: a bounded census (ranger-base-k5fnr)

ADR 0026's first done-when, and the reason the rung changed. The review that
produced the 2026-09-05 operator ruling asserted no *measured* benefit from
requiring a separate research bead for every knowledge gap. This is the
measurement that was owed, taken before the rung was rewritten.

## Window, population, method

**Window.** 2026-08-27 → 2026-09-06, eleven days. The lower bound is not a
round date: it is `b2d3bd2f` ("ladder: SPIKE between ASSUME and ASK — the gap
is knowledge, not permission", rangerhq-qe37), the commit that first rendered
the mandatory SPIKE rung into every work prompt. Before it no persona was
handed the instruction, so no earlier bead can be evidence about it.

**Population.** Every bead in the fleet queue whose title matches `spike:`
(case-insensitive) and whose `created_at` falls in the window — the shape the
rung tells a persona to file. Widened once to catch a spike filed under some
other name: every in-window bead whose title *or description* contains
"spike" or "ADR 0026" (38 beads) was read by title, and the extra 34 are ADR
work, verify batches, and beads *about* the rung rather than spikes filed by
it. So the population is four, not four-plus-a-dark-figure.

**Denominator.** 1,470 beads created in the same window (1,343 of them
closed). Four spikes against 1,470 beads is the first number: the rung that
reads as a mandate was exercised roughly once per 368 beads.

**Method.** MEASURED — `sqlite3 'file:…/ranger-queue/.beads/beads.db?mode=ro'`
on 2026-09-06, one read-only connection, against `issues`, `dependencies` and
`comments`; the dependency rows and the close comments are quoted below, not
summarised from memory. Nothing was written to the queue by this census. The
adopt/reject call at the end is a judgement over those rows and is labelled as
such.

## The four, and what each one supplied

| spike | filed → closed | separate dependency or deliverable? | blocked its deciding bead? |
|---|---|---|---|
| `ranger-base-j5s0` — measure what sessions legitimately write under `.git` | 08-27 → 08-31 | **yes, deliverable**: a measured path × writer × frequency table two later beads consumed (`ranger-base-7w8g0` design, `ranger-base-0syry` verify) | **no** — its only edge is `j5s0 --discovered-from--> ranger-base-vqvl`; no `blocks` row exists |
| `ranger-base-cpo9` — centralized dispatch vs idle-agent pull | 08-27 21:45 → 08-27 22:05 | **yes, deliverable**: field survey + options memo + the A-vs-B argument, landed as ADR 0028 and six implementation slices that carry `discovered-from` edges back to it | **no** — `cpo9` has no dependency rows at all, in either direction, other than those six children |
| `ranger-base-au0o4` — does `GET /v1/models` answer 200 to a session mint? | 09-01 → 09-02 | **yes, dependency**: a live probe needing a credential only a launched session holds; filed by richard, measured by gwart | **yes** — `mvrke --blocks--> au0o4` and `wkai3 --blocks--> au0o4` |
| `ranger-base-dvxac` — re-ask `/api/oauth/usage` outside the rate-limit window | 09-02 05:25 → 09-02 13:36 | **yes, venue**: the experiment needs an hour in which nothing else on the box asks, which the deciding session cannot manufacture | **yes** — `wkai3 --blocks--> dvxac`, created 2026-09-02T05:25:54 by gwart and still in the store (the close comment's "removed before closing" is a DIFFERENT edge, `dvxac --blocks--> ranger-base-uzyd2`, in the other direction) |

**Outcome: 4 of 4 supplied a distinct dependency or deliverable. 0 of 4 were
research receipts duplicating work that fit the deciding task.**

## What that does and does not license

It does **not** say the mandate was harmful. It says the mandate was never
what produced these four:

- Two of them (`au0o4`, `dvxac`) are separate because the *experiment* is
  separate — a credential only another session holds, an hour the deciding
  session cannot clear. ADR 0026's post-ruling rule files those anyway. The
  mandate is not what caused them.
- Two of them (`j5s0`, `cpo9`) were never split off a deciding task at all.
  `cpo9` has no incoming edge and closed twenty minutes after it was filed,
  by the persona who filed it, in a crew session with the operator: it is a
  design bead that carried its own research, which is exactly the shape the
  ruling now blesses. `j5s0` was a measurement handed to a later design.
- The half of the rung that was doing real work — "deciding waits on reading"
  — held in **two of four**. The other two carried no `blocks` row, so the
  deciding bead stayed in `bd ready` and the wait never happened. That is
  `ranger-base-rs8j`'s defect showing up in the sample, and it is why the
  rewritten rung keeps the block AND adds `bd dep list <id>` to confirm it.

The claimed benefit of mandatory splitting — a separately tracked artifact
that would otherwise be lost — is therefore **unmeasured in this window**, in
the sense that every artifact in the sample had an independent reason to
exist. Task count did not prove value here; it did not disprove it either.

## Uncertainty, stated

- **n = 4.** Four events over eleven days on one instance's queue. No
  interval around any of these numbers is worth writing.
- **The counterfactual is not measured.** "Would the research have happened
  without the mandate?" is not answerable from the store. What *is* measured
  is that the mandate's own marker never appeared: the rung asks for a
  `SPIKE: <question> → <sid>` comment on the deciding bead, and of the two
  in-window comments that contain that string, both are *quotations* of the
  rung — the close of `rangerhq-qe37`, which implemented it, and the verify
  batch that read that close. **No comment in the queue begins with
  `SPIKE:`.** The four spikes were filed by personas acting on the trigger,
  not by personas discharging the rung's bookkeeping.
- **Classification is a reading.** "Distinct dependency or deliverable" was
  judged from each bead's description and close comments, quoted above. A
  reader who counts `cpo9` as "research that fit its own deciding task" gets
  3 of 4 rather than 4 of 4; the direction of the conclusion does not move.
- **Silent misses.** A gap someone researched inline and never filed anything
  for leaves no row. The census cannot see it, and this is the one place the
  ruling's risk ("unresolved questions may hide in implementation") lives.
  The control for it is unchanged and is not this bead's: committed findings,
  and explicit acceptance on the deciding bead.

## What shipped against it

`internal/posse/dispatch.go` `EscalationLadder`: the SPIKE rung's default is
now bounded research **in the deciding bead**, with findings on the bead and
in a committed ADR section or notes artifact, numbers labelled MEASURED or
ASSUMED with date and environment. A separate `spike:` bead is for a distinct
dependency or deliverable — another lane's work, an experiment needing its own
venue, findings needing their own handoff — "never as proof that research
happened", carrying its time box, question and stopping condition. When one is
filed the mechanics are unchanged and the confirmation is new: no `--deps`,
`bd dep add <id> <sid>`, a `discovered-from:` comment on the spike, and
`bd dep list <id>` to confirm the block actually landed — the half the census
found missing in two of four.

Untouched, deliberately: the four triggers, research-before-invention, the
sourcing rules, committed findings, normal design→build handoffs, and the
acyclic-graph mechanics. The removal is the multiplication, not the research.

## Two corrections carried by the same rewrite

- `ranger-base-ytsp9` (open, P3): the old rung asserted "bd refuses the
  `dep add`" as universal, which `ranger-base-lpz0o` measured false — one
  store refuses, another accepts it silently and leaves the bead in
  `bd ready`. The rewritten sentence says the outcome (the block is lost) and
  leaves the two shapes on the `Provenance:` line, which is where a persona
  reading a zero exit as the stop has to be caught. Pinned both ways in
  `TestEscalationLadderSpikeFilesNoProvenanceEdge`.
- Instance-owned reinforcement is **not** touched here and is not this repo's:
  four PID sources in the private ops repo carry a "then file `spike:
  <question>` and dep-block the deciding bead on it (ADR 0014)" line —
  `posse/agents/{jared,richard,erlich,hoover}.md` — plus `richard.md`'s
  `spike` intent row. They still read as the mandate, and they cite the
  pre-restatement ADR number. Editing a PID and promoting it is the
  operator's path, not a persona's; filed as a `-l question` bead.
