## The governance surface: the condition set, and what it costs (rangerhq-81y0)

`posse status` answers one question — *does anything need a human right
now?* — and answers it with an exit code, so a script can ask too. The
cockpit draws the same answer as a GOVERNANCE block, and the pulse tick
carries it to the coordinating persona as hints. One function
(`ShopCheck`, internal/rhq/govern.go), three renderings, no fourth store.

Design: the archive's governance-surface ADR §1–2 (its numbers do not
resolve here — see HISTORY.md "ADR numbering"). Two lines from it decide
almost everything below:

- **Facts get computed, decisions get beads.** A condition is a checkable
  fact — computable by any process, twice, with the same answer. It is
  level-triggered and heals by itself, so it is never written down.
  Decisions get `-l question` / `-l risk` beads, which is the existing
  path and stays the only way a governance item enters the queue.
- **The view does not depend on the loop.** Everything is read live from
  the store that owns it. A dead watch loop is a *row* (G7), not a reason
  the view goes quiet. What dies with the loop is delivery.

### The rows, and the store each one asks

| row | condition | store | class |
|---|---|---|---|
| G1 | session blocked on an approval | herdr agent status | LANE |
| G2 | settled-but-holding (the `rangerhq-zom` skip) | bd + herdr | LANE |
| G3 | question/risk bead past `attn_question_age` (4h) | bd | LANE, URGENT if it blocks open work |
| G4 | plan guard skipping past `attn_guard_stuck` (2h) | the guard + the loop's clock | URGENT |
| G5 | guard blind past `plan_guard_blind_max:` | the plan snapshot | URGENT |
| G6 | Dial E stop (day/plan window ≥ 100%) | cost scan vs caps | URGENT |
| G7 | watch loop dead while autostart armed | the loop's flock | URGENT |
| G8 | paused | `state/pause.yaml` | URGENT |
| G9 | ready bead routed to the coordinator | bd + config `coordinator:` | LANE |

Plus two carry-overs from the pulse's own first cut — `unpushed:<repo>:<n>`
and `no-live:<persona>` — which are not G-rows (the table is closed at
nine) and are marked `—`. They stayed because both are things the
coordinating persona owes, and a bead titled "widen" has no business
quietly deleting shipped oversight. `no-live:` is gated on the pulse being
armed: it is a fact about DELIVERY, so on a shop with no pulse it would
make `posse status` non-zero forever for no reason.

### The three things that will bite a later reader

**G4's streak is the watch process's memory, and a fresh shell has none.**
The guard's *reading* is re-taken at view time; the *streak* — how long it
has been tripping — lives in `Dispatcher.guardTrippedSince`, beside
`blindSince`, and dies with the loop. A fresh loop earning a fresh 2h grace
is correct, not a bug. So `posse status` from a terminal reports **no G4**,
ever, and that is deliberate: a guard that trips once is a SKIP —
automatic, self-healing, pure mechanism — and turning every ordinary skip
into an URGENT the moment somebody typed a command would be the surface
crying wolf about the one thing already working.

**G5's blind clock is the shared snapshot, not a variable.**
`plan-usage.json` records when the last successful reading was TAKEN, and
`PlanCache.LastReadAt` reads it. That is what lets a process which is not
the watch loop answer "how long have we been blind" at all — the blind
window is a fact about the instance, and the instance writes it down.

**Unknown is not clear.** A bd scan that fails, a missing bd, a watch-lock
that cannot be probed: each comes back as an error beside whatever was
computed, `posse status` prints it and exits non-zero anyway, and the
cockpit heading says `partial, N store(s) unread`. Reporting an unreadable
store as an all-clear is the exact silence this surface exists to end —
the same rule `posse beads check` keeps.

### What it costs (measured, 2026-08-27)

The archive ADR assumed the widened view's bd reads would come in under 2s
and asked this bead to measure. First cut: **5.6s and 30 bd calls** on a
single-repo instance with 25 open question/risk beads. 24 of those calls
were one `bd dep list --direction=up` per aging question bead, asking
"what points at me" to decide G3's URGENT promotion.

`bd blocked --json` answers the whole graph in ONE call — every blocked
issue with the ids holding it — and it is the better question, because the
row means "this holds work out of the queue" and that is what `blocked`
reports. After the switch: **1.95s and 7 bd calls** (3 lists + 1 blocked +
one `comments` per settled holder), ~1.0s of it bd. The assumption holds,
but only because the per-finding call went away: a view whose cost scales
with the number of findings is a view that gets slowest exactly when the
shop is worst.

Per view, then: one herdr listing, `bd list --status in_progress`, `bd list
--label-any question,risk`, `bd blocked`, `bd ready`, one `git rev-list`
per beads repo, one `bd comments` per settled holder, and — only where the
operator armed them — one shared plan reading and one transcript scan. An
unarmed guard makes no request and Dial E with no cap scans nothing.
