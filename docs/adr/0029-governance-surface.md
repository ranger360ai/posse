# ADR 0029 — Shared governance observations and explicit pause intent

*Status: accepted; simplified 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · descriptive consolidation, no required runtime removal · amended 2026-10-09 for G12-G15 (bead ranger-base-wmaf9, from github.com/ranger360ai/posse/issues/3).*

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
| G3 / forgotten park (`parked:<id>`) | The same bd read: a question or risk bead parked with NO end date, past `attn_parked_age`, default 14d. The clock is `updated_at`, which is when the park happened; a DATED park is re-surfaced by its own date and this horizon never reaches one (ranger-base-pm5zo) | LANE; URGENT when holding ready work |
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
| G11 / `launcher-behind:<depth>` | The running launcher's own build stamp counted against the branch of the configured checkout that holds it, past `launcher_behind_max` (default 16); the key carries the doubling step, so a deepening lag re-prompts at 16, 32, 64 and nowhere between. An unreadable stamp raises nothing and is not a partial view | LANE |
| G12 / `hook-wall:<slot>:<repo>` | One row per degraded L3 slot in the repos `beads_visibility:` declares, from `SweepHookWallIdentity` — the sweep with ADR 0023's behavior half unasked; a skipped repo (absent, not git, managed hooks path per ADR 0052 D1) is never a finding. The detail is the sweep's own line, and the queue repo's carries one clause more | LANE |
| G13 / `queue-not-a-repo` · `queue-unmarked` | Two config facts that make the launcher's commit of the store of record's projection fail at every close: `queue_repo:` is not a checkout; or it is unmarked in `beads_visibility:`, which is public, which is the one state where the commit guard's beads-jsonl scan runs over it. Not the projection's dirtiness, and not a `beads:` store that legitimately lives outside the queue repo — both are ordinary states | LANE |
| G14 / `memory-unlanded:<persona>` | A persona memory dir holding lines no commit holds, with no live session for that persona — the landing runs at a kill (`memoryland.go`), so this is a landing that already ran and left them, or was refused. Owner: the personas checkout, read in one `git status` for the whole dir | LANE |
| G15 / `session-degraded:<name>` | A live, non-foreign session whose meta carries `degraded:` — the gates its wall does not realize, which `posse ls` has marked ⚠️degraded since the parity check shipped. Reported, never alarmed: consent was given once with `--allow-degraded` and is re-armed by every relaunch from the meta | LANE |
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

G11 is the second row added under that bar (bead ranger-base-y13h7, from
ranger-base-6pnab). Its predicate is `LauncherLag.Behind >= launcher_behind_max`
with the reading known; its owner is the running binary's build stamp, counted
in whichever configured checkout holds that commit (`launcherlag.go`); its
scope is the instance's `beads:` checkouts and deliberately not the process's
working directory, because the condition is about the fleet this instance
dispatches rather than the directory a command was typed in; its class is LANE,
on the backup-stale rule — a stale launcher stops nothing, it keeps running
with the defects its own repo fixed. The reading itself is older (ADR 0029 is
not its design; `launcherlag.go` and bead ranger-base-z3hx6 are) and it was
printed on every status view and at every doubling of a watch pass for a month
without any condition reading it: for four days `posse status` printed a lag
sentence ending "only installing closes it" and `nothing needs a human` two
lines beneath it, in the same view, while the binary reached 101 commits
behind. A READING IS NOT A CONTROL AND NEITHER IS THIS ROW — it gates nothing,
declines no launch and installs nothing; installing over a binary that is
dispatching a live fleet remains the operator's (guardrail 3), and the row's
only claim is that a human is needed. The threshold is argued from this box's
own install history rather than inherited, in
`docs/notes.d/ranger-base-y13h7.md` and in the constant's own doc comment: 13
reconstructed install episodes with a median depth of 48, against two traced
costs at depths 34 and 93 — so the box's cadence cannot bound it, and 16 is the
largest doubling step below 21, the shallowest depth at which a fix the fleet
needed was already on main.

G12 through G15 are the next four rows added under that bar (bead
ranger-base-wmaf9, from github.com/ranger360ai/posse/issues/3 — operator,
work-box shakedown ranger-base-x4g3h). They share one defect and it is this
page's own: each is a fact some other surface in this harness already printed,
in a view nobody was reading, while `posse status` printed `nothing needs a
human` the same minute. The all-clear is `GovReport`'s answer for an EMPTY
set, so a fact that raises no condition is a fact the surface actively denies.
All four are LANE on the backup-stale and G11 rule — URGENT means the shop is
stopped, and none of these stops it; LANE still exits `posse status` non-zero,
draws in the cockpit's GOVERNANCE block and ends the all-clear, which is the
whole of what the bead asked for.

- **G12** — predicate: `SweepHookWall` reports a degraded slot. Owner: the
  hooks dir of each repo `beads_visibility:` declares, which is the same list
  `scripts/verify-hook-freshness.sh` walks; scope is that list and
  deliberately not "every repo on the box". The sweep itself is older (bead
  ranger-base-ixv4) and fires at `posse promote`'s epilogue and once per watch
  loop — both one-shot, both scrolled past. The row asks ADR 0023's IDENTITY
  half only: MEASURED 2026-10-09, darwin 25.4.0, four declared repos, the pair
  costs 3.41/3.54/3.62s a sweep against 375/396/393ms for identity alone,
  because the behavior half execs the render once per repo. What the drop
  gives up is named at `l3AskIdentity`: behavior catches a RENDERER
  regression, which is a property of the binary and identical in every repo,
  and is already asked at every launch and once per watch loop. One row per
  SLOT, keyed by slot and repo, because two slots are two facts that heal
  separately.
- **G13** — predicate and owner in `queuejsonl.go` `QueueReady`, beside the
  commit it is about. The two causes are the ones that make the NEXT close
  fail too. "The projection differs from HEAD" is deliberately NOT one, and
  the reason is measured: `bd sync` re-exports the jsonl from any session and
  the launcher commits it only at a close it judges, so on 2026-10-09 this
  instance's queue `issues.jsonl` was dirty at a moment nothing had been
  refused. Nor is "a configured store that does not resolve inside
  `queue_repo:`", which this reading shipped with for one afternoon and which
  a live `posse status` measured away: the launcher does skip those closes,
  but a `beads:` entry with its own store is the ordinary multi-project shape
  — on this instance it named a client repo whose beads were never meant to
  live in posse's queue — and this ADR's §4 reference moves THE STORE OF
  RECORD, not every store an instance reads. `queue-unmarked` is the cause
  that needs its chain said out loud:
  unmarked is PUBLIC (fail closed, `visibility.go`), and the commit guard's
  check 0 — the beads-jsonl visibility scan — runs in public repos only, so an
  unmarked queue repo is exactly the state in which the launcher's own commit
  of the store of record is scanned as a publication and refused. It is also
  ADR 0015 §4's cutover step 5, performed once by hand and recorded nowhere
  else.
- **G14** — predicate: a persona memory dir holds changes no commit holds AND
  no live session carries that persona. The second half is what keeps it a
  condition rather than noise: ADR 0015 §5 leaves the memory WRITE live and
  ungated, so a working persona's dirty ORDERS.md is this system functioning,
  and only a persona with nothing running has a landing that already ran and
  left the lines. Both of `LandPersonaMemory`'s refusal arms reach this row —
  the credential-shape hold, which is level-triggered and refuses identically
  forever, and the git failure, which is self-healing at the next kill and so
  resolves itself off the row. Owner: the personas checkout, read in ONE `git
  status` for the whole dir (MEASURED 2026-10-09: 38/35/34ms against
  194/201ms for a call per persona over 11 personas, same answer), with a pin
  that the sweep and the per-persona read agree.
- **G15** — predicate: a live, non-foreign session whose meta carries
  `degraded:`. Owner: the session meta, already in the listing this check
  takes, so the row costs nothing. REPORTED, NEVER ALARMED, on G8's shape: the
  shop is not stopped and a human already said yes. What makes it a condition
  anyway is that the consent does not stay where it was given — a relaunch
  inherits it from the meta (`relaunch.go`), so one `--allow-degraded` typed
  days ago keeps re-arming itself for as long as the session is rebuilt. The
  detail names the GATES, because the remedy depends on which one it is. It
  can name the same hook G12 does — parity probes the L3 slots — and that
  overlap is deliberate: G12 is a REPO's wall, true whether a session is
  there, cleared by an install; G15 is one SESSION's waiver, cleared by ending
  or relaunching that session once the wall holds. Two keys, so one healing
  does not silence the other.

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

A park with no end date is re-surfaced by a horizon, not by its date, and
that is a widening of G3 rather than a tenth row (ranger-base-pm5zo). The
condition is the same one G3 already owns — a question or risk bead nobody has
answered — and only the clock differs: `updated_at`, which is when the park
happened, because the date the park would otherwise be read from is the field
one store class discards (ranger-base-bwp7h, ranger-base-nkjjg). MEASURED
2026-10-03, bd 0.50.3: `bd ready` excludes a park whose date is four weeks
past on both store classes, so this surface is the only thing that re-surfaces
any park at all, and before the horizon an indefinite one was silent forever.
The row carries its own key (`parked:<id>`) because the pulse fingerprints
keys and the two conditions have different remedies; it reuses G3's URGENT
rule unchanged, because a park holding beads out of `bd ready` stops the shop
the same way. `attn_parked_age:`, default 14 days, argued against every park
horizon the shop has expressed in `docs/notes.d/ranger-base-pm5zo.md`.

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
| Issue #3 / ranger-base-wmaf9 (2026-10-08) | G12-G15: a degraded wall, a queue that cannot take the projection, unlanded persona memory and a degraded session are conditions, not readings somebody else prints |

Prior tables and evidence: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0029-governance-surface.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
