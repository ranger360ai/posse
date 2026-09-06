# ADR 0028 — Rolling seats: event-driven refill, the pass becomes an epoch

*Status: superseded 2026-09-05 by ADR 0011 · ADR simplification, operator ruling 2026-09-05.*

The surviving decision is in [0011 — current contract](0011-dispatch-model.md), Decision §5 and Lineage. This page keeps its number and dated evidence; the body below is historical, not current policy.

## Historical record (superseded in full)

*Status: accepted 2026-08-27 (spike + decision on ranger-base-cpo9, crew
session with the operator) · owner: richard · amends 0011 (kept-list) and
0020 §5 · implements 0016 · supersedes the design ask on ranger-base-l8u7 ·
amended 2026-09-04 (§1 the Run is long-lived, the PASS is bounded: the
gather has a window and carries what is still in flight —
ranger-base-3ryit) ·
amended 2026-09-02 (§2 a constitution refusal is not an attempt —
ranger-base-39jnl) ·
amended 2026-08-28 (§1 the refill re-offers every free seat and the settle
is the level-trigger, the reap rides it; §3 the busy map holds only this
Run's fires, readings expire with the fire pass — ranger-base-t8tq, as
built in b58c957)*

## Context

MEASURED (l8u7): a 5-prompt pass had four beads settle quickly while one ran
75+ minutes; the gather barrier (`dispatch.go` — a serial loop over pending,
seats re-offered only on the next `Run`) held five seats empty behind it.
Pass wall-clock is max(bead), bounded only by `WaitCeiling` (4h), and
`--watch` is strictly pass→sleep→pass with nothing waking it early.
MEASURED (1t7r audit): every seat runs at 36–56% of its own demonstrated
ceiling — 56.6 beads/day of spare capacity against a 33.5/day arrival gap.
Duty cycle, not seat count, is the shop's binding constraint, and the pass
barrier is a large component of it.

Two facts make the fix small. First, the pass is already mostly a clock,
not a policy unit: plan verdicts are per-bead (0013 §3), the load guard is
per-launch (`planLaunch`), step-down/tier/uncounted are per-bead,
verify-after keys on a per-repo watermark, and the reap predicate reads bd
state. Only four things remain denominated in the pass: `budget_pass`,
`-n`/`autostart_max_beads`, the in-memory busy map, and reap's
`justPrompted` exclusion. *(t8tq showed this list undercounted: it named
the values but not the lifetimes. Anything whose clock was "the pass" —
the reap's pass-start placement, the fire loop's cached seat readings —
silently re-denominates to "the process" once the Run stops returning;
§1/§3 as amended carry the fixes.)* Second, ADR 0016 (herdr event hints) is accepted
and unimplemented, and its motivation is this problem verbatim.

Operator constraints, verbatim-close (cpo9): one bead at a time per agent;
persona and memory preserved, so per-agent execution stays linear; the dead
time between a bead settling and the next central pass is what gets
leveraged. Field survey and the full options memo (A–D) are on cpo9; the
survey's load-bearing finding: every production pull system (CI runners,
Temporal, Celery) keeps policy central and enforced at claim time — pull
moves the clock, never the brakes. This ADR moves the clock and nothing else.

## Decision

**§1 — The refill** *(amended 2026-08-28, ranger-base-t8tq — as built)*.
The dispatch `Run` becomes long-lived. When a seat's bead settles —
signalled by a herdr settle event on the 0016 channel, or by the existing
backoff tick as backstop — the Run judges that bead exactly as `gather`
does today (judge-by-bead; mergeBack and commitQueue unchanged, under the
launcher lock), then immediately re-runs the whole fire path under the
launcher flock (`refire`): a fresh bd ready scan offered to **every free
seat**, not only the one that settled, narrowed only by the operator's
`--persona`, exactly as any pass narrows. Events are hints, not truth
(0016's own framing): every hint is verified against bd and herdr before
acting.

**The settle is the level-triggered tick.** As accepted, this section said
"re-runs the fire path for the freed seat" and rested its correctness on
"the level-triggered tick still sweeps everything, so a lost event costs
latency, never correctness" — but that tick is Watch's pass, and Watch
does not get its loop back while a rolling Run keeps refilling. A Run that
never returns is a tick that never fires, so under the narrowed refill a
seat this Run had not fired into was never offered work again. MEASURED
(dispatch-watch.log, 2026-08-28, one Run 08:46:37→15:56:51): ~90% of the
day's closes went to the one seat that kept settling while three seats
with ready beads in their lanes sat out seven hours. With the refill
running the whole fire path, each settle sweeps everything the tick would
have, so a lost settle costs latency only until the next settle or the
Run's return — the original guarantee, restored by moving the tick onto an
event that still happens.

*(amended 2026-09-04, ranger-base-3ryit — as built)* **The Run is long-lived;
the PASS is bounded.** "Long-lived" was built as "returns when the in-flight
set drains", and that set is fed by the very refill this section adds: every
settle launches, every launch joins the set, and on a shop busy enough to keep
refilling it never empties. MEASURED 2026-09-04: pass 1 held "4 prompt(s) in
flight, gathering" for 2h20m while refills kept landing work — the merge-back
sweep, the hook wall, the backup and guard tickers, the plan read, the epoch
accounting and any offer of ready work to a seat that freed with **no settle**
behind it did not run at all, and nothing said so; a P1 the operator asked for
by name sat ready and unhired for an hour with its seat empty. A pass whose
duties are time-based may not be gated on session-shaped waits. So the gather
takes a window (the loop's base interval): legs that land inside it are judged
and refill exactly as above, and legs still outstanding are **carried** — the
wait goroutines and their fan-in belong to the loop, not to the pass, so the
next pass takes them back, nothing is judged twice and nothing is dropped. The
busy map and the per-slot failure count are carried with them, because a
carried leg's seat is still occupied (§3). The cost is bounded and named: a
settle landing after the window is judged up to one interval later, which is
the latency 0016's hint exists to remove and which this ADR already prices as
latency, never correctness. And the loop gains the reading that was missing — a
pass that has not completed inside a budget derived from that window is a
finding, said once, naming the set holding it (watchdog.go).

The reap sweep rides the same event for the same
reason: it fired at "pass start", which a long-lived Run re-denominates
into "process start" (26 done-sessions piled up in a day); it now runs at
every settle — the event that makes a per-bead session sweepable — with
the Run-start and epilogue call sites kept for shapes with no settle. The
gather barrier is removed; nothing waits on an unrelated bead.

**§2 — The epoch.** The pass survives as an accounting window: a
wall-clock-aligned epoch (config `dispatch_epoch:`, default 1h — ASSUMED, a
tuning decision for the operator when the slice lands; wall-clock alignment
means a Run restart cannot reset spend authority). `budget_pass` now
denominates the epoch; `-n`/`autostart_max_beads` bound launch attempts per
epoch, preserving their original intent (bound unattended launches per unit
time). 0020 §5's width law re-denominates per-epoch and keeps its point: a
hire must not silently raise spend authority.

*(amended 2026-09-02, ranger-base-39jnl)* An **attempt** is a launch that
reached a runtime — successes and failures alike, because a failure still
cost the box a session and the persona a turn. ADR 0015 §3's launch verify
is the one refusal that costs neither: it fires in `planLaunch` before a
session is created, before the bead is claimed and before any prompt is
sent. It is handed back to the epoch, and the fire pass stops rather than
re-deciding one reading of one home once per seat. MEASURED 2026-09-02: an
older posse on PATH made every launch refuse there, and a `-n 30` epoch
was spent on thirty refusals that reached nothing — the fleet then sat out
the hour with the operator's fix already in place.

**§3 — Brakes stay exactly where they are.** Plan verdict per-bead, load
guard per-launch, step-down/tier/uncounted per-bead, the uncounted ledger's
rolling 7d, verify-after's watermark, and the reap predicate are all
untouched. (The overflow ledger this line also named was removed with its
mechanism by ADR 0010 §1, ranger-base-6xx37; nothing about the cadences
here moved with it.)
The two remaining migrations: the busy map's denominator changes from
per-pass to live seat occupancy — one bead per persona per repo *at a time*,
released at that seat's settle — which is 0020 §4's actual intent with the
pass artifact removed, still backed by the `personaActive` live read; and
reap's exclusion keys on `promptedRecently` from the session meta (already
persisted cross-process, 0011 §3) instead of the in-memory `justPrompted`
set.

*(amended 2026-08-28, ranger-base-t8tq)* The busy map's contents are
exactly **seats this Run fired into** — held from the fire to that seat's
settle, and nothing else and nothing longer. Everything a fire pass merely
*reads* about a seat on the way past — `personaActive`, another launcher's
prompt grace, a CLI benched after two session failures — is a reading
taken at an instant, not an occupancy: it lives for the fire pass that
took it (`seatMap`'s pass half, fresh per `fireLoop` call) and every later
offer re-reads the seat live. Under a one-shot Run the two lifetimes
coincided, so the distinction was invisible; under a rolling Run, caching
a reading for the life of the process locked seats out on facts about the
morning — the other half of the t8tq starvation.

*(amended 2026-09-03, ranger-base-ifjgm)* "Released at that seat's settle"
is the release, not the only one. A settle is an event, and a hold can stop
being true with no event to hang a release on: a settle that came back
`working` is counted, dropped from the gather, and never looked at again, and
a session can be reaped, killed or lost with its herdr server between one
refill and the next. MEASURED 2026-09-03: a seat held on a bead whose session
had been reaped read busy for 2h12m and every refill hired into 2 of 3 seats,
released only by bouncing the watch. So occupancy is also **reconciled
against herdr at the head of every fire pass and every refill**
(`reconcileSeats`): a hold with no live session under its seat is released,
with a line naming the seat and the bead it was holding. The reconcile is
evidence-only and fails closed — a session listing that could not be read is
not an empty herd, and `--dry-run`, which holds seats it never launched into,
is not reconciled at all. This narrows the map to what §3 already says it
holds; it widens nothing.

*(clarified 2026-09-06, ranger-base-kal4c)* "That seat's settle" above means
the settle of **the bead holding the seat**, never a settle judged for the
seat's name. The strict reading is the one §5's fourth observable requires —
never two live beads per (persona, repo) — but the sentence admitted the loose
one and the implementation took it: `judge` deleted `busy[seat]` by name on
whichever settle it judged next. A settle and the fire pass are different
moments (each leg waits in its own goroutine, and its result sits in a channel
until the loop's next gather), so a stale settle was routinely judged after
the pass had already put a new bead on the seat, and the refill hired into a
working seat. MEASURED 2026-09-06 (dispatch-watch.log, watch pid 94728):
three over-caps in two hours — a one-seat lane at 2/1, a two-seat lane at
3/2, a three-seat lane at 4/3. Fixed and pinned under ranger-base-25cit: a
hold is released only by the settle of the bead it names, and any other
settle leaves it held and says so. A wording clarification, not a design
change; the live statement is 0011 §5, and this note exists because the code
cites this section by number.

**§4 — One throttle, preserved.** All refills originate in the one watch
process. Killing it stops the shop inviting work; no agent-initiated launch
path exists. This property is load-bearing for the operator and is a
ratified invariant of this design, not an implementation accident.

**§5 — Verification observables** (each runs with a control arm before the
ADR is called implemented):
1. idle-to-next per seat (settle→next-launch): baseline measured BEFORE the
   refill ships (slice 1), target ~seconds after; the ranger-base-pfwp
   cadence harness is the instrument.
2. throttle: watch process killed → zero new launches; alive → launches
   observed.
3. spend: epoch spend never exceeds `budget_pass`, including across a Run
   restart mid-epoch.
4. occupancy: never two live beads per (persona, repo) — today's invariant
   re-asserted under rolling refill.

## Consequences

The Run is long-lived (autostart already supervises exactly this). The
flock is taken per-refill rather than once per pass; hold time stays
bounded per-launch (0011 §1) — contention at 11 seats is ASSUMED
negligible, measured by observable 1's harness. 0011's kept-list is
amended: "burst, fire-then-gather, serial launches" is removed; the
busy-key rule (re-denominated), judge-by-bead, the claim-is-the-fence
ordering, and the wait ladder's per-leg mechanics are all KEPT. 0016 stops
being decorative. `--dry-run` semantics unchanged (reports, launches
nothing, unlocked).

## Alternatives rejected

**B — self-serve next (`posse next` run by the agent on settling).** The
operator's initial preference; argued in session and on cpo9. Identical
idle-time win to A, so it had to win elsewhere and did not: the trigger is
a prompt-obeyed ritual executed by an LLM at end-of-context (the settles it
misses — crashed, reaped, wedged, forgot — are exactly the tail needing
pickup); the brake call site moves onto the agent's PATH inside a gated
shell, the environment of this shop's worst measured incidents; the
launcher-flock holder set expands to agent-invoked processes with a
measured 40-minute wedge prior; the one-throttle leaks and demands a new
must-never-fail stop flag; and B needs §2's re-keys anyway. Determinism
decided it: A's trigger set is exhaustive and its ordering replayable from
one process; B's is "the agent behaved." B's honest wins, on record:
smaller build (no herdr plumbing), survives a dead watch loop (redundant
with autostart), matches the pull aesthetic. B remains the named second
step if observable 1 still shows material idle after A ships.

**C — full runner model (per-persona poller daemons).** The CI industry's
endpoint, built for thousands of ephemeral workers under a hosted control
plane. Eleven durable seats and an operator-as-control-plane is not that
scale; C converts the one throttle into N and leaves 0013 §5's account line
homeless. Everything B risks, plus structural loss of §4.

**D — shorten the gather ceiling to ~15m.** The tourniquet: caps the l8u7
incident at ~15m for a constants change, but idle stays polling-quantized
and the barrier remains. Not taken because A subsumes it; it stays
available as interim relief if A's slices stall.
