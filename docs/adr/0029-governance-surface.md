# ADR 0029 — Shared governance observations and explicit pause intent

*Status: accepted; simplified 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · descriptive consolidation, no required runtime removal.*

## Decision

`ShopCheck` computes the shared view used by status, cockpit, pulse and watch
logs. Facts are read from their owners; durable decisions belong in question
or risk beads. Do not add an attention store or condition registry. Existing
G identifiers and stable fingerprint keys remain; they are not a claim that
the vocabulary is closed at nine or that every process has identical inputs.

| Existing id/key | Observation and scope | Class |
|---|---|---|
| G1 / blocked session | Current Herdr agent status | LANE |
| G2 / settled-but-holding | bd claim joined with current session; protocol comment prefix may subtype it | LANE |
| G3 / aged question or risk | bd creation/status and blocked-work graph; `attn_question_age`, default 4h | LANE; URGENT when holding ready work |
| G4 / sustained guard skips | Current guard plus watch-local `GuardTrippedSince`; `attn_guard_stuck`, default 2h | URGENT |
| G5 / blind guard | Current failed plan observation and guard blind-window state | URGENT |
| G6 / exhausted cap | Cost scan against armed day, epoch and plan caps | URGENT |
| G7 / `arm-broken` | Startup arm parsing: present empty interval is broken | URGENT |
| G7 / `loop-dead` | No owner holds the armed watch's lock | URGENT |
| G7 / `loop-mute` | Live watch lock but log older than the watchdog's maximum healthy silence | URGENT |
| G8 / paused | Explicit `state/pause.yaml` intent | URGENT, reported without an alarm |
| G9 / coordinator-routed work | Ready bead routing against config coordinator, per ADR 0033 | LANE |
| G10 / `verify-box-stale` | Age of the last `scripts/verify-box.sh` verdict in `state/verify-box.yaml` against `verify_box_max_age` (default 26h); never run, and a stamp ahead of the clock, are the same observation | LANE |
| G10 / `verify-box-unmeasured` | A fresh verdict in which every check answered "nothing measured" | LANE |
| G10 / `verify-box:<checks>` | Checks the fresh verdict reports as finding or error; the key names them, and `verify_box_accepted` adds the tracking bead id to the detail without removing the check | LANE |
| G10 / `verify-box-accept-stale:<check>` | A `verify_box_accepted` entry whose check is not red in the fresh verdict | LANE |
| `unpushed:<repo>:<n>` | Local git upstream comparison for configured bead repositories; no upstream yields no finding | Existing carry-over, no G id |
| `no-live:<persona>` | Missing delivery target, only when pulse is armed and a target exists | Existing carry-over, no G id |
| `backup-stale` | Armed backup policy and archive observation under ADR 0036; no archive is stale | LANE, no G id |

The table describes current computations, not a new enum. Key is identity;
changing detail text, age or percentage alone must not make a fresh pulse
episode. New observations need a documented predicate, owner, scope and class,
not a fabricated row to satisfy a row count. Preserve current carry-over
rendering (`—`) and machine keys.

G10 is the first row added under that bar (operator ruling 2026-09-06 on
ranger-base-0x1wc; build ranger-base-jj2ax). Its owner is the state file
`scripts/verify-box.sh` writes, and freshness is load-bearing rather than
incidental: checked-recently-and-clean is the only green, so a schedule that
stopped, a run killed before its verdict, and a box nothing has ever checked
are one observation. A dying run's own output is separately preserved — the
LaunchAgent's `StandardOutPath`/`StandardErrorPath` are `state/verify-box.log`
— because a job that logs only what it finished cannot say what killed it. A
red check tracked by an open bead is named with that bead id on the row and is
not removed from it; automatic bead filing was considered and refused in the
same ruling.

G4's streak is process-local and resets on restart. A fresh status process
cannot infer two hours of skips from one reading and reports no G4. G7 is
several observations of delivery health, not a pidfile assertion: test the
lock/arm/log appropriate to that observation. No observer gets an atomic
cross-store snapshot. An unreadable input yields a partial view, never clear:
status exits nonzero, cockpit says partial, and pulse logs failures alongside
known conditions. Current status does not depend on a healthy watch; delivery
does. A held lock alone does not prove logging or progress.

G6 uses wall-clock-aligned `dispatch_epoch` (default 1h), alongside the day
window, from one scan at the earliest needed floor. Preserve this shipped
cap visibility. The plan decision, including blind behavior and approved
overflow removal, belongs solely to [ADR 0010](0010-plan-guard-overflow.md). Pulse
delivery and repeats belong solely to [ADR 0027](0027-monica-pulse.md). Backup
scope and staleness belong to [ADR 0036](0036-posse-backup.md).

## Stop authority

SKIP is a self-healing mechanism for the current pass: guards, caps, busy and
crew eligibility. A governance observation cannot automatically latch PAUSE.
`posse pause "<why>"` writes explicit `by`, `at`, `why` intent;
`posse resume` removes it. Every dispatch entry checks it before new work.
Already-running work is not retroactively killed, and pulse oversight keeps
ticking. The operator and authorized coordinator may pause; a coordinator
reports its reason upstairs. STOP/disarm/deployment remains the operator's.

The coordinator handles routine work within its PID authority and routes
permission changes, promotions, risk refusals and operator judgments to the
operator as decisions. This page grants no new push or approval authority.
The response objective remains action within a pulse turn and unblock or
escalate within minutes; delivery logs alone do not prove intervention.

## Consequences and alternatives

No condition, config key, actor, store or flag is removed. This page retires
the fictional fixed cardinality and universal process-equivalence claims;
there is no machinery task for those wording corrections. Provider-error pane
parsing is not added; existing session/claim observations and logs carry those
symptoms. Do not create a bead for every self-healing condition or mirror
current facts into a durable attention file. Pause intent is a separate fact
and therefore legitimately durable.

Dated evidence: the original expanded view cost 5.6s/30 bd calls; replacing
per-finding dependency reads with one blocked-graph read measured 1.95s/7
calls (rangerhq-81y0). The 2026-09-03 mute-loop incident justified observing
the log as well as lock ownership. Neither number is a fresh performance
claim. Removing shared computations would make the views diverge; preserving
observation scopes prevents a fresh shell from inventing missing history.

## Lineage

| Record | Surviving decision |
|---|---|
| 0029 and its G4/G6/G7/carry-over amendments | One computed view with explicit observation scopes |
| Operator ruling 2026-09-05 | Existing conditions retained without closed-nine fiction |
| Operator ruling 2026-09-06 (ranger-base-0x1wc) | G10 live-box verdict with a mandatory freshness rule and a named-bead suppression |

Prior tables and evidence: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0029-governance-surface.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
