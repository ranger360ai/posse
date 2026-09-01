# ADR 0029 — The governance surface: facts get computed, decisions get beads, and pause is a human speech act

*Status: accepted 2026-08-21 in the archive (its bead rangerhq-e37c,
architect; amended there 2026-08-22 for G9) · restated 2026-08-27, bead
ranger-base-evva · §1–2 implemented here as rangerhq-81y0 (govern.go,
`posse status`, the cockpit GOVERNANCE block, the pulse tick rewired);
§3 is rangerhq-a2g6, G7's flock probe is rangerhq-mgvx · G6 epoch
amendment 2026-08-27, bead ranger-base-jbmh · G7 arm-reading amendment
2026-08-29, bead ranger-base-yznr · ADR 0036 tenth-row question answered
2026-09-01, bead ranger-base-a0ln0 (the table stays closed at nine)*

> Restated from the private archive of the instance this harness was
> developed in (its governance-surface ADR, bead rangerhq-e37c); incident
> citations reference that instance's history. Persona names are restated
> as roles; measurements from that instance are restated as rationale,
> not quoted. In the archive this document carries a number that resolves
> to a different ADR here — see HISTORY.md "ADR numbering" for why
> cross-repo references go by title, never by number. Two amendments
> decided in code on rangerhq-81y0 are folded in below, each marked
> *(amended here)*, and §1–2's measured cost replaces the archive's
> assumption in Consequences.

## Context

The operator, thinking aloud: dispatch should distribute
deterministically (it does — orderBeads, Route, `-n`, the busy key, the
launch flock), and governance should be the thing that pauses the shop
when tokens run out or a human is needed — "not an agent." He is right,
and most of the stopping machinery exists: the plan guard (armed 70/85),
the blind window, Dial E, ADR 0010 overflow, question beads never
dispatched, dep-blocked beads waiting.

What does not exist is the ESCALATION SURFACE. When an *agent* stops — a
tool-approval dialog in its pane, a `REFUSED:` at a risk line, a stalled
prompt, a provider 529 — that fact lives in a terminal pane, a pass log,
sometimes a bead comment. Nothing enumerates it and nothing raises it.
Twice in a single day in the archive instance, the coordinator's
blocked-time-to-intervention ran to multiple hours, because the shop's
only escalation path was a human deciding to look. The pulse ADR —
restated here as ADR 0027 — built the delivery: a clock and a prompt
over three conditions. This ADR defines the full condition set, where it
lives, and who may stop what.

## Decision

**1. "Needs a human" is a closed, enumerated set of predicates, each
with one store of record.** A governance condition is a checkable fact —
computable by any process, twice, with the same answer — never a
judgement. The set, and the honest split between URGENT (the shop is
stopped) and LANE (one bead or session is stopped, the rest flows):

| id | condition | predicate (store of record) | class |
|---|---|---|---|
| G1 | session blocked on an approval | herdr agent status `blocked` | LANE |
| G2 | settled-but-holding: bead `in_progress`, its session's agent settled | bd status + herdr status, subtyped by the last comment's ladder prefix — `BLOCKED:` / `REFUSED:` are protocol strings the harness itself injects (dispatch.go's ladder), so reading them is mechanism | LANE |
| G3 | question bead open past `attn_question_age` (default 4h) | bd: `-l question` / `-l risk`, status + created_at | LANE; URGENT if it dep-blocks ready work |
| G4 | plan guard skipping, sustained past `attn_guard_stuck` (default 2h) | the guard's own reading, re-taken at view time; streak from the watch process's clock | URGENT (self-healing; urgent only when sustained) |
| G5 | guard blind past `plan_guard_blind_max` | planusage fetch fails now | URGENT — monitoring itself is broken |
| G6 | Dial E stop / budget ≥ 100% | cost scan vs caps | URGENT for spend, heals at the window |
| G7 | watch loop dead while autostart armed, or the arm itself broken | the loop's flock, and config.yaml's arm reading (a bare `autostart_interval:` is a refused arm, not an armed one) | URGENT — the meta-condition: no other condition gets delivered |
| G8 | paused (§3) | `state/pause.yaml` | URGENT by intent — reported, never alarmed |
| G9 | ready bead routed to the coordinator, whom dispatch refuses to hire (ADR 0033, restating the archive's coordinator-is-not-a-lane ADR) | bd assignee/labels + config `coordinator:` | LANE |

Provider errors (529 storms) are not a ninth row: their observable shape
*is* G1/G2 plus watch-log lines, and a predicate that parses pane text
for provider names would be a judgement in costume.
Dead-session-on-claimed-bead is also not a row: dispatch already
self-heals it (relaunch on the claim-held path); only the settled case
(G2) needs a human.

*(amended here)* **The reported set is the nine G-rows plus two
carry-overs from ADR 0027 §1** — `unpushed:<repo>:<n>` and
`no-live:<persona>` — which are not G-rows (the table is closed at nine)
and render with class `—`. They stayed because both are things the
coordinating persona owes, and a widening has no business quietly
deleting shipped oversight. `no-live:` is gated on the pulse being
armed: it is a fact about DELIVERY, so on a shop with no pulse it would
hold `posse status` non-zero forever for no reason.

*(amended here, 2026-08-27, bead ranger-base-jbmh)* **G6 carries the
epoch window once the rolling-seats epoch re-key lands** (ADR 0028 §2;
its S3 slice, ranger-base-f0y3). G6's reading was day-plus-plan only,
and the reason was honest when written: `budget_pass:` measured spend
from the moment a pass began — a clock only the dispatching process
held, which fails this section's own test (computable by any process,
twice, with the same answer). The re-key dissolves that reason: the
window becomes wall-clock-aligned (`dispatch_epoch:`, default 1h), so
its start and its spend are the same scan-since-a-time the day window
already runs. Two facts decide it:

- **Honesty forces it; opportunity alone would not have.** After the
  re-key, an epoch cap at 100% is a reason dispatch launches nothing —
  exactly G6's condition — and the epoch is almost always the *tightest*
  armed dollar window (default 1h against the day). A `posse status`
  reporting only the day window answers "why is nothing launching" with
  "budget fine" while the shop is stopped: blind to the very stop G6
  exists to name. "Unknown is never clear" applies to a window the
  surface declines to read, not only to a store that failed.
- **The feared cost is dead on the shipped scan shape** (MEASURED from
  the code's structure, not timed: `budget()` in dispatch.go already
  feeds both windows from ONE transcript scan, floor = the earlier of
  local midnight and the window start; window totals are in-memory sums
  over the same report — `PassTotal`'s shape). dialE armed already scans
  from midnight, so the epoch total is a second sum over a report it
  already holds: zero additional scans on the operator keystroke. One
  carve: an epoch configured longer than a day moves the scan floor to
  the epoch start — the same floor rule dispatch applies to a pass that
  opened before midnight.

Effective when the re-key lands; until then there is no epoch to read
and dialE's day-only reading is the correct rendering of the caps that
exist. The dialE change rides its own `-l code` bead, dep-blocked on
the re-key slice, and replaces dialE's interim comment with a pointer
here.

*(amended 2026-09-01, bead ranger-base-a0ln0 — answering the tenth-row
question ADR 0040 sent here)* **The table stays closed at nine; ADR
0036's stale-backup condition ships as a third carry-over.** ADR 0036 §6
says on-box backup staleness "raises a ShopCheck condition (ADR 0029
G-table)", which reads as a request for a G10. It is not a conflict: 0036
asked for the fact to reach the surface, not for it to be numbered — so
this record's closed enumeration wins, and the condition takes the shape
this section already defines for one that is not a G-row. Key
`backup-stale`, no id, rendered `—`, beside `unpushed:` and `no-live:`.

Its class is **LANE**. URGENT is defined above as "the shop is stopped",
and a stale backup stops nothing; giving the one class that means
stop-everything to an overdue duty is how a surface stops being read.
LANE still makes `posse status` exit non-zero and still draws in the
cockpit's GOVERNANCE block, which is what 0036 §6 asked for.

Armed is the gate, and armed is generous: any `backup_*` key in config,
or an archive already on disk from a hand-typed run. An instance with
neither reports nothing, the inertness rule `queue_repo:` keeps (ADR
0015 §4). An instance that is armed and holds NO archive reports the
condition — that is 0036's own Context, the arrangement configured and
never run, and it is the one state an age-only check would call clear.

*(amended 2026-08-29, bead ranger-base-yznr — descriptive accuracy, not
a design change)* **G7's "armed" is the startup hook's reading, not key
presence.** A present-but-empty `autostart_interval:` is a broken arm —
plugin/autostart.sh refuses it by name and exits 1 rather than arming
anything (ranger-base-cxyk) — so G7 also fires off the config, key
`arm-broken`, without consulting the flock (nothing is armed to be
running); a valued key keeps the flock probe and key `loop-dead`. The
table stays closed at nine: the fact is the same one — nothing is
delivering, and nothing will at the next herdr start — only the cause
differs, and it differs by KEY, which is what the delivery fingerprint
moves on. Decided in code on ranger-base-i6h; the same amendment retires
the row's pidfile+argv husk parenthetical, the flock probe having
shipped (rangerhq-mgvx).

**2. The surface is a computed view, rendered three ways — not a store.**
Every G-row above is a fact already owned by exactly one store (herdr,
bd, the plan endpoint, the kernel's flock, the pause file). A durable
"attention file" the loop writes each pass would be a snapshot of other
stores' facts — the fifth store, ADR 0011's disease, honest only while
the loop that writes it is healthy. So:

- **One function** (`ShopCheck`, the pulse's condition set from ADR 0027
  §1, widened to the table above) computes the set live from the stores.
- **Three renderings, one computation:** `posse status` — a command and
  a cockpit section — for any human on demand, exit code non-zero when
  the set is non-empty (scriptable); the **pulse tick** for delivery to
  the coordinator (ADR 0027 unchanged: hints, edge-suppressed, renag
  backoff); **log lines** in the watch log for audit and for the
  blocked-time-to-intervention metric. `pulse.yaml` remains dedup state
  for delivery, never a record anyone reads for truth.
- **Honesty when the loop is dead:** the view does not depend on the
  loop — `posse status` reads the stores directly and reports G7 itself,
  via the arm reading and the flock probe (release *is* death, no
  staleness class). What
  dies with the loop is *delivery* only, stated plainly: a dead loop
  pulses nobody, and the residual witness is the operator's glance at
  the cockpit header — which is ADR 0027's own arming premise ("no
  watch, no pulse; a drained shop's witness is the operator").

*(amended here)* **The widening deliberately breaks ADR 0027 §1's
"never bd, never the plan endpoint" boundary.** G2/G3/G9 are bd facts
and G5/G6 are meter facts; a surface that cannot see them is blind to
most of what actually stops a shop. What the boundary protected —
silence off a timer nobody is watching, never a false alarm and never a
bill — survives as a rule instead of an abstinence: an unreadable store
is logged and the tick moves on, and in the on-demand renderings the
same failure is printed beside whatever was computed ("unknown is never
clear" — `posse status` exits non-zero, the cockpit heading says
partial). The cost of the added reads is bounded and was measured, not
assumed — see Consequences.

The line that decides every future case: **facts get computed, decisions
get beads.** Conditions are level-triggered and heal (an approval gets
clicked, a window cools) — they are computed, never stored. Decisions —
a question, a risk acceptance, a REFUSE needing an operator call — are
durable work items and are already bead-shaped (`-l question`,
`-l risk`); that stays the only way a governance item enters the queue.

**3. SKIP is the mechanism's; PAUSE is a human's; STOP is the
operator's.** Three verbs, kept distinct:

- **SKIP** — this pass only, keep polling. The guard, the blind window,
  Dial E, the busy key, crew-held. Automatic, self-healing, pure
  mechanism. **No condition may auto-PAUSE**: latching a transient
  reading into a durable stop that needs a human to clear trades a
  self-healing skip for a flapping meter parking the shop overnight.
  When the constraint is tokens, the mechanical answers are already
  chosen: skip (guard) now, overflow (ADR 0010) when armed — pausing
  is not the token response.
- **PAUSE** — stop dispatching until told otherwise. `posse pause
  "<why>"` writes `state/pause.yaml` (`by:`, `at:`, `why:`); `posse
  resume` removes it. Every pass — watch, hand-typed, cockpit `d` —
  checks it first (alongside planGuard, one read under the fire-loop's
  entry) and declines with one line naming the pauser and the reason.
  The file is a legitimate new store: pause intent is a *new fact* with
  a single writer, not a copy of another store's fact. A pass in flight
  finishes first (same contract as ctrl-c). **PAUSE stops spend, not
  oversight:** the pulse goroutine keeps ticking, so a paused shop
  still escalates blocked sessions and aging questions — this is what
  distinguishes it from killing the loop, and why the coordinator
  should reach for it instead of `kill`. Who may pause: the
  **operator**, and the **coordinator** — strictly gentler than the
  authority the coordinator's PID already grants ("stop the watch
  rather than let it hire overnight"); every coordinator pause is
  reported upstairs with its why.
- **STOP** — the loop killed, autostart disarmed, `make uninstall`.
  Operator only; it is his promotion/deploy key wearing its off switch.

**4. The coordinator is the exception handler, not the scheduler.** The
loop schedules (deterministically); the surface detects (mechanically);
the coordinator judges. What that role owes, made explicit:

- **SLA:** act within one pulse turn of delivery; unblock-or-escalate
  within minutes — the `blocked-time-to-intervention` metric, now
  computable from the pulse log against herdr state changes instead of
  from apology.
- **Clears alone:** routine tool approvals (per the PID's never-approve
  list), stranded claims (`--resume`, reassignment), crew pushes,
  PAUSE/resume with a recorded why.
- **Must reach the operator:** his keys (promotions, permission files,
  judgment calls), every `REFUSED:` at a risk line, `-l risk` and
  `-l question` beads, and any pause — as options + recommendation on a
  bead, per the coordinator's PID.
- **How the coordinator is told:** the pulse (ADR 0027), unchanged in
  mechanism — this ADR only widens its condition set to §1's table. The
  prompt carries hints, never authority; the stores stay the record; a
  lost prompt costs latency, never truth.

**5. The mechanism/judgement line, drawn once.** Mechanism (no model in
the loop, ever): the predicates, the guard and blind window, Dial E and
the overflow cap, the pause-file gate, the pulse's clock and dedup —
every budget decision lives here, by design. Judgement (a persona with a
PID, memory, metrics): clearing approvals, answering questions, deciding
to pause, choosing what reaches the operator. Delivery from mechanism to
judgement is a lossy hint by construction, so governance never depends
on a model choosing to care — if the coordinator never wakes, the guard
still skips, the pause file still gates, the cap still caps.

**ADR 0011, checked:** this stays inside it. The watch process gains a
wider *view* and no new *authority*: bd is still the queue, launches
still serialize on the flock, `posse dispatch` is still a command a
human runs and watches, and `posse status` is a read. The pause gate is
enforced by whichever process runs a pass — no daemon owns it. Arming
the existing watcher is untouched by this ADR and not required by it:
the surface works dry-armed, exactly as ADR 0027 §2 chose for the pulse.

## Consequences

- Implementation, dependency-ordered, one session each: **§1–2**
  condition set + `posse status` + cockpit section, extending the
  pulse's ShopCheck — **shipped here as rangerhq-81y0** → **§3**
  pause/resume + the pass gate (rangerhq-a2g6, independent) → the infra
  persona: G7's flock probe wired into status/cockpit alongside the
  standing loop_alive migration, and a scratch-herdr verification that
  the surface stays honest with the loop killed (rangerhq-mgvx).
- Config: `attn_question_age` (4h), `attn_guard_stuck` (2h) — code
  defaults, keys optional; no new arming key. `state/pause.yaml` is
  state, not config.
- Observables for the verify beads, predicted now: `posse status` exits
  non-zero within one tick of a session going blocked; a killed watch
  loop shows G7 in `posse status` run from a fresh shell; `posse pause`
  then a hand-typed `posse dispatch` launches zero sessions and names
  the pauser; a paused shop with a blocked session still pulses the
  coordinator.
- Costs *(measured here, replacing the archive's assumption)*: the
  archive assumed the widened view's bd reads come in ≤2s and asked the
  §1–2 bead to measure. First cut: **5.6s / 30 bd calls** on a
  single-repo instance with 25 open question/risk beads — 24 of them a
  per-finding `bd dep list --direction=up` deciding G3's URGENT
  promotion. The assumption holds only after that per-finding call was
  replaced by one `bd blocked --json` (the whole blocked graph in one
  call, and the better question — the row means "this holds work out of
  the queue," which is what `blocked` reports): **1.95s / 7 bd calls**,
  ~1.0s of it bd. The durable lesson: a view whose cost scales with the
  number of findings gets slowest exactly when the shop is worst.
  Details: docs/notes.d/rangerhq-81y0.md.
- G4's streak lives in the watch process's memory like blindSince —
  lost on restart, accepted: a fresh loop earning a fresh 2h grace is
  correct, not a bug. Decided in code on rangerhq-81y0: a fresh shell
  has no streak either, so `posse status` from a terminal never reports
  G4 — a guard that trips once is a SKIP, and promoting every ordinary
  skip to URGENT the moment somebody typed a command would be the
  surface crying wolf about the one thing already working.
- The coordinator's ORDERS gain the §4 SLA when the pulse arms; the PID
  needs no edit — every §4 authority is already granted there.
- Metrics: `blocked-time-to-intervention` computed from the pulse log
  (target: first delivery within `pulse_interval`, cleared or escalated
  within the coordinator's "minutes not hours"); pauses with a recorded
  why: 100% (the file shape makes the why mandatory).

## Alternatives rejected

- **Attention beads** (`-l attention` per condition). Durable and
  auditable — and wrong on both axes that matter: conditions are
  level-triggered facts that heal by themselves, so every filed bead
  needs a closer (a GC/safe-reclamation problem the archive crew had
  just paid for), and each one is a second writable copy of a fact
  herdr or bd already owns — the fifth store again. bd is the queue;
  polluting it with weather reports makes `bd ready` lie. Decisions
  keep their beads; that is the existing `-l question` path, kept.
- **A durable attention file the loop writes each pass.** The literal
  "durable file" candidate. It is a snapshot pretending to be a store
  of record, exactly as honest as the loop is alive — it fails the
  design's own test ("nothing that only works while the thing being
  monitored is healthy"). The append-only *log* survives (audit is
  history, not truth); the state file does not.
- **An OS notification as the escalation channel.** Visible without
  polling, yes — but sent by the same process being monitored (dead
  loop, dead channel), unauditable (no record it fired or was seen),
  and a second delivery path to maintain beside the pulse. If the
  cockpit-glance residual ever proves insufficient, revisit as a
  renderer of `posse status`, never as the surface.
- **A governance agent** (the operator's own instinct said no, and he
  is right). ADR 0027 already rejected folding approvals into
  mechanism; this is the mirror image — folding the *stopping* into a
  model. A pause decided by an LLM is a judgement where a checkable
  fact is required; §5's line exists so the budget and the gate never
  depend on a model choosing to care.
- **Auto-PAUSE on urgent conditions** (the clever one — the architect
  wanted G5, guard-blind, to latch a durable pause). But the skip
  already stops spend while blind, heals on the first good reading, and
  costs nothing; a latch converts that into a manual-resume outage on
  every OAuth hiccup, and its real content — "someone should look" — is
  precisely what the pulse and G5's row already deliver. If a future
  condition genuinely demands a latch, it must arrive as its own ADR
  with the flap analysis this one declined to hand-wave.
