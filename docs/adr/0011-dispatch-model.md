# ADR 0011 — The dispatch model: bd is the queue; the pass gets a lock, a safe prune, and a run record

*Status: accepted 2026-08-20 · amended 2026-08-23 (§1 third holder;
§2 identity arm — restored 2026-08-26, see §2) · amended 2026-09-05 (0020/0028 folded; current selection and bounded lifecycle) · corrected 2026-09-06 (§4 `route_order` is a PID key; §5 the epoch's two restart bounds and 0028 §5's observable as a median claim; App. A's closed residual — ranger-base-yv9uo) · amended 2026-09-06 (§5 a seat hold is released by the settle of the bead holding it, never by a settle judged for the seat's name — ranger-base-kal4c) · owner: architect*

> Restated from the private archive of the instance this harness was
> developed in. The incidents this ADR reasons from happened in that
> instance; they are named here by shape (the lost-claim incident, the
> meta-sweep race…) rather than by tracker id, and persona names are
> restated as roles.

## Context (dated diagnosis, 2026-08-20)

Three passes over dispatch, in the operator's order:

**Adherence.** The code follows the record. ADRs 0002/0003/0005/0006/0008
are implemented as written; 0010 is accepted-not-built and tracked, so
code and record agree through the bead. Where they disagree, it is the
record that drifted (a recurring precedent): dispatch.go's header comment
still narrates the pre-Dial-F `<persona>-<repobase>` find-or-create loop;
DIRECTION.md's loop sketch predates Dial F and the claim ordering; ADR
0006 §3 omitted the dry-run exception NOTES states (amended in this
commit). No case was found where the code should move to match the
record.

**Simplification.** ~19 closed dispatch beads in three days are not one
flaw. The honest split: about half were substrate-dialect discoveries —
bd exits 0 on a lost claim, herdr's timeout JSON arrives on stderr,
claude's onboarding modal reads idle, grok's splash eats the prompt —
probes any design would have needed; and one bug was fixed three times
because the error's actual shape was probed last, not first (standing
lesson: log the argv before designing). The other half is one class, and
the coordinator's hypothesis names it almost right. Precisely:
**dispatch infers cross-store facts from single-store snapshots.** Four
stores — bd (claims), the meta dir (identity), herdr (liveness), the
watch pidfile (loop liveness) — are updated independently and read at
different instants, and every incident in the class is one store's
momentary reading taken as evidence about another store's durable fact:
a herdr wait timeout read as "the claim is invalid" (the thrice-fixed
bug); a listing snapshot read as "the workspace died" (the meta-sweep
race and its siblings); a pidfile read as "the loop lives"; a session
name read as "who holds the bead" (the holder-join incidents). The
concern count (routing, prompting, judging…) is not the disease — those
steps are sequential and have been stable since the parallel-pass work
landed. The stores are.

**The model.** A pass is a synchronous burst; `-n` and an in-memory busy
map are the only concurrency control. Nothing serializes two passes
(the meta-sweep race), a pass and the cockpit's `d` (the PromptGrace
memory is per-process — verified: `lastPrompt` is never persisted), or
the autostart loop and a hand-run pass. The operator asked: do we need a
queue?

## Decision

**No queue. bd is the queue** — dependency-aware, atomically claimed,
aggregated across repos. A second queue store would be a fifth store and
the disagreement class's next member. Claims stay leases-without-expiry:
recovery from a stranded claim is the operator (`--resume`, visible in
the cockpit), because auto-expiry's failure mode is double-dispatch — the
exact damage the original timeout-as-verdict incident did. What dispatch
is missing is three small disciplines, not a new substrate:

**1. One launcher at a time per RHQ_HOME — a kernel lock, not a
pidfile.** `flock(2)` on `state/dispatch-launch.lock`, taken by `Run` for
the fire loop and by
`LaunchBead` for its whole body. Blocking, with one line naming the
holder pid while waiting. Why flock and never a second pidfile: the
pidfile incidents are what a pidfile costs — liveness inferred from a
file whose truth decays; flock's release *is* process death,
kernel-owned, no staleness class. Hold time is bounded by `-n` ×
(create + StartupWait) — low minutes at `-n 3` (**measured** in current
pass output); flock overhead itself **assumed** negligible.

*(Amended 2026-08-23: the enumeration above was written from what
dispatch launches, and there are three holders, not two. The verify-after
sweep (ADR 0006 §3) is the one write a pass makes* before *its fire
loop, and its dedupe is two check-then-act pairs bd cannot make atomic —
read the watermark … write it, and dependents-list … create, with no
create-if-absent to fold the second into one call — so two launchers
starting within the same second both saw a close as new and both filed
for it: the duplicate-verify incident Appendix A cites, repro pinned in
the launch-lock QA tests. The sweep now takes and drops the lock inside
its own call — Run's two acquisitions are sequential, never nested — and
its hold time is a `bd list --all` per repo plus a create per new close,
not the `-n` × (create + StartupWait) bound above, which is the fire
loop's. The invariant is unchanged; only its list of where it applies
was short.)*

**2. Prune must prove death, not infer it** *(folds the meta-sweep-race
bead)*. `Sessions()` may delete a meta only when (a) its `launched:` is
older than a grace (5m — younger is exactly the race window) **and**
(b) a direct per-id workspace query at prune time confirms the workspace
is gone — a listing snapshot is never sufficient evidence. The socket
guards stay in front. Observable for the verify bead: the race's repro
(three concurrent `--persona` passes) deletes zero metas.

*(Amended 2026-08-23: (b) as written answers* liveness*, and the
measurement closed Appendix A's second asterisk the bad way — herdr's id
allocator is `max(live id)+1`, recomputed from the live set at every
server process start, restart and live handoff both, so an id that
answers alive may be a stranger's workspace. The rule gains an* identity
*arm: the answer must also be ours — the workspace's `label` (every
workspace is created with `--label <session name>`) fenced by `gen:`, a
meta field holding dev:inode of the api socket, which herdr recreates at
exactly the moments the allocator resets. Inside one generation ids are
never re-issued, so a label mismatch there is a rename and the session
keeps its name; across generations rename and re-issue leave identical
evidence, so the ambiguous case does nothing to the file. Three paths ask
the predicate, not one: the prune, the create (`mustNotOrphan`), and the
live branch of `Sessions()` — which this section never covered and which
is where the damage actually was: a stale meta whose id a stranger holds
used to be listed over that workspace, and every addressing path
(Resolve, AgentTarget, KillSession, RelaunchAgent) reads that listing.
Two deliberate asymmetries. The fence is NOT a third arm of the socket
guard (`cannotAnswerFor`): `workspace_not_found` stays proof of death in
ANY generation — a workspace never changes its id while it exists, and
the ids that survive a restart are the live ones, unchanged — while a
generation mismatch that kept metas would keep every meta forever after
any restart. And the create refuses rather than repairing itself onto
the label-matched workspace: a repair is only right if the session were
alive under a different id, which cannot happen. "Not mine" never means
"delete the meta" — kept, left out of the listing, reported with the
repair recipe. `gen:` is stamped at create and backfilled only on
positive identity. Tests: `internal/posse/metaidentity_test.go`; promote
gate: `scripts/verify-prune-guard.sh`. This amendment was written
2026-08-23 in the private instance and lost in the private→public
restatement; restored 2026-08-26 from the closing record, mechanisms
re-verified against this tree.)*

**3. The session meta is the run record.** `bead:` identifies the run's
bead and persisted `prompted:` supports cross-process prompt grace. The
prompt reading uses the later of the file and process map; holder lookup
also checks liveness through `Sessions()`. Read run facts from this record,
not from session-name patterns. Preserve the holder/resume, foreign-claim,
done-agent and no-agent exceptions documented in the dated build record below.

Gather waits outside the launch lock; its settle-driven queue/merge writes
and fresh fire path acquire it as §5 specifies. Watch holds its separate
`state/dispatch-watch.lock` for its lifetime; a second watch refuses. Lock
availability and log health are distinct observations (ADR 0029).

**4. Availability-first lanes and serial seats** (folded from 0020).
A lane is the set of personas whose labels intersect the bead's; no lane
registry exists. A recognized explicit assignee is a lane of one and never
falls through. Label matches sort by `route_order:` ascending — a PID
frontmatter key, lower first, default 50 — then persona name; the
configured default persona is the last routing fallback. The coordinator
is excluded on every route (0033). Under the launcher lock,
every fresh launcher, including cockpit `d`, takes the first eligible seat:
not occupied by this Run, not working/blocked elsewhere in the repo, and
not the bead's crew holder. All busy means wait with the lane and reasons
named. `--persona` narrows the eligible set.

A holder is resumed, never reseated: the assignee and live run record head
the lookup. Crew, foreign-holder and prompt-grace refusals remain bead-level
refusals; they do not invite a second seat. A persona remains one serial
fleet seat per repo, because its memory and identity have one writer.
The explicit operator resume exception recorded in 0020 is unchanged.
Assignment for ordinary work happens at launch; verify-after is unassigned
unless the operator sets its pin. Batching and acceptance belong to 0006.
Epoch width is bounded by bead cap, budget at measured cost per bead, and
free seats with ready work; adding a seat does not increase spending authority.

**5. Rolling seats with bounded passes** (folded from 0028). The watch loop
owns in-flight waits, their fan-in, occupied seats and per-slot failure
counts across passes. A pass gathers for the loop's base interval; it
judges completed legs once and carries outstanding legs to the next pass.
It does not wait for the entire in-flight set to drain. A one-shot run
retains its wait behavior. The bounded return lets the watch run plan,
epoch, merge-back, hook, backup and other periodic duties; the watchdog
names an over-budget pass once. The interval is an existing clock, not a
new timeout declaring work dead.

Each observed settle is judged against the bead; merge-back/queue writes
and the complete fresh fire path run under the launcher lock. Refill
offers ready work to **every** eligible free seat, under the operator's
filters. Both bounded passes and settles drive progress; missing an event
can cost latency, never ownership. 0016 owns the decision to remove hints
and keep this reconciliation clock. The reap sweep runs at settles, run
start and epilogue, so it cannot become a process-start-only duty.

Occupancy holds only seats this Run actually fired into. A hold names the
bead it was fired for, and it is released by the judging of **that bead's**
settle — never by a settle judged for the seat's name. The two are different
moments: a leg settles in its own goroutine and its result waits in a channel
until the loop's next gather, and the ordinary fire pass can put a new bead on
the same seat in between; a release keyed on the seat then retires a hold
minutes younger than the settle retiring it, and the refill hires into a
working seat. MEASURED 2026-09-06 (ranger-base-25cit, dispatch-watch.log):
three over-caps in two hours — a one-seat lane at 2/1, a two-seat lane at 3/2,
a three-seat lane at 4/3. A settle judged for a bead that no longer holds the
seat releases nothing, and says so in the log. The other release is positive
liveness reconciliation at every fire pass/refill: a hold whose seat has no
live session is released on evidence, so refusing the by-name release strands
nothing. An unreadable session list keeps holds; dry-run fake launches are not
reconciled. 0028 §5's fourth observable — never two live beads per (persona,
repo) — is the tiebreak for every reading of this paragraph: where a release
rule and that observable disagree, the seat stays held. Other observations
(busy elsewhere, prompt grace, a benched CLI) expire with the fire pass and
are read fresh on the next offer. Claims are never released because a wait
timed out.

`dispatch_epoch:` (default 1h) denominates `budget_pass:` and
`-n`/`autostart_max_beads`. The epoch is wall-clock aligned, so a restart
mid-epoch recomputes the same window rather than opening a fresh one. Its two
bounds are restart-proof to different degrees, and the record says which:
spend survives a restart because it is not stored at all — it is re-derived
from the transcripts against the recomputed epoch start (one fact, one store)
— while the launch-attempt count has no such external store, lives on the
Dispatcher, and a supervised restart restores the full `-n` ration. The brake
that bounds money is per epoch; the brake that bounds blast radius is per
process. Making attempts durable would add the one-more-small-store this
record warns about, for a bound spend authority already covers. An
attempt reached a runtime, even if it failed.
A constitution verification refusal before session creation, claim and
prompt consumes no attempt and stops the fire pass. Plan, load, tier,
uncounted-work and spending brakes retain their existing per-bead/launch
boundaries. All automatic refills originate in the watch process: ending
it stops new automatic invitations. Agents do not acquire a self-launch path.

MEASURED in 0028's dated evidence: the gather barrier held seats behind a
75-minute worker; narrowed refill starved other seats for seven hours;
an unbounded pass delayed periodic duties for over two hours. Retaining
these fixes removes zero runtime mechanisms. The ASSUMED that removing
optional hint latency stayed acceptable is now MEASURED: 0016's done-when
row was taken on ranger-base-4dxpo over a 15h31m loop block at a fixed 3m
cadence — 0 hint-driven wakes in 142 passes, and p95 2m6s (max 2m48s, n=41)
from a seat becoming ready to its next dispatch wherever ready work
existed, all inside one interval. 0028 §5's first observable — idle-to-next
per seat, target "~seconds" — is met at the MEDIAN and not at the tail: the
2026-09 adherence audit (docs/notes.d/adr-adherence-2026-09.md) read the live
watch log as treatment-arm medians of seconds to a few minutes against maxima
of tens of minutes to hours, with about half the windows per Run closed by a
refill. Read that observable as a median claim, not a ceiling. Its report is
emitted once per pass, after the gather (seatidle.go), so what bounds the
reporting latency is the loop's interval, not the Run's lifetime. The
rejected alternatives remain an agent-invoked
next-work ritual (duplicate delivery and a lost central throttle), one
poller per persona (more concurrent owners), and a shorter gather ceiling
with the same barrier. The standard disciplines are single writer,
durable claims and deduplication; a timeout is not proof of failure.

## Lineage

| Was | Here |
|---|---|
| 0020 §§1–4 selection, assignment and serial identity | §4; `route_order` records the current built tiebreak |
| 0020 §5 width and §6 verify fan-in | §4 and §5 epoch lifecycle; 0006 owns batching |
| 0028 §§1–4 refill, epoch, occupancy, one throttle; §5 observables | §5; dated incident and rejected-option evidence remain in 0028 |

Moving these rules removes zero runtime files, keys, states, actors or flags.
Doing nothing preserves duplicate authority. Round-robin adds a cursor;
least-backlog substitutes a proxy for live availability; filing-time rotation
reads availability hours early; per-persona fan-out makes memory multi-writer.
These alternatives remain rejected. Their added maintenance price is ASSUMED;
the incident evidence is retained in the superseded source.

## Dated implementation record (2026-08-20 through 2026-08-27)

Sequencing and future tense below describe the original build; current lifecycle is §§1–5 above.

- Implementation, dependency-ordered, one session each: **the safe
  prune** (§2, fix directions chosen above) → **launch lock** (§1) →
  **run record** (§3, deps on both — it touches both files' territory).
  A doc bead fixes the stale records (dispatch.go header, DIRECTION
  sketch). ADR 0006 amended in this commit.
  *(§3 landed 2026-08-27, rangerhq-o2ki. `bead:` was already there from
  ADR 0013 §4; `prompted:` is new, written by every launcher under §1's
  lock, and `promptedRecently` believes the later of it and the process
  map. Two things §3's prose left open, decided in the build and recorded
  here because a later reader will ask. **Where the record is read from:**
  as a FILE, not through `Sessions()` — "when was this prompted" is the
  record's own content, where a listing answers liveness at the cost of a
  herdr round trip per bead per pass; they can disagree only for a meta
  the listing would drop, and there this reads "prompted recently" over a
  session the caller then declines to prompt, which is the direction every
  guard in dispatch fails in. `RunHolder` still goes through `Sessions()`,
  because the holder join needs liveness as well as the record. **What the
  pass's grace exempts:** the guard is what answers when every other guard
  abstains on a stale reading, so it stands down wherever a launcher is
  deciding rather than missing — a holder join that found the session and
  an operator's `--resume` that answered for it, a row naming another
  actor (the claim answers that, and must be allowed to fail), a session
  herdr reports `done` in, and one herdr detects no agent in at all, which
  is not a lagging status but a crashed CLI the relaunch answers. Pins:
  `internal/posse/runrecord_qa_test.go`, and laurie's pass↔pass repro in
  `launchlock_qa_test.go`, whose skip is gone.)*
- Handoffs: **security** — a pruned meta also deletes the crew mark, so
  a wipe can turn the operator's own conversation back into
  fleet-promptable; that is the data-loss shape of the meta-sweep race,
  assess exposure. *(Assessed 2026-08-21 as rangerhq-ynx8 and fixed
  2026-08-27. The exposure was wider than the crew mark: a meta-less
  workspace `Resolve` finds by label answers EVERY dispatch guard with a
  zero value — no crew, no agent, no run record — and each guard read the
  absence as permission, so the bead was claimed and a work prompt tiered
  and caged for the routed persona was typed into whatever agent that pane
  held. Dispatch now fails closed: `foreignHeld` sits beside `crewHeld` at
  both launchers and `launchSession` carries the backstop, refusing rather
  than reading "foreign" as "no session yet" — creating under a held label
  is the collision, not the fix. The refusal stays in dispatch and NOT in
  `Resolve`/`AgentTarget`, which the operator's own commands need. The
  destructive half is rangerhq-selx; `docs/notes.d/rangerhq-ynx8.md` has
  the full account.)* **ops** — once §1's helper exists, migrate autostart's
  `loop_alive` onto the same flock discipline (kills the pidfile class
  at the root); `dispatch-watch.pid` stays for the husk check's identity
  half. *(Landed 2026-08-26, rangerhq-gir5: `--watch` holds
  `state/dispatch-watch.lock` for its lifetime and a second one refuses;
  `posse dispatch --watch-status` reports the lock on one line and
  `plugin/autostart.sh` reads that line. No `ps` and no pid remains in the
  liveness decision, closing rangerhq-ppy9 and ranger-base-rmc with it. The
  one judgement the flock did not make for us: a probe that cannot be ASKED
  — a posse too old to know the subcommand — is not an answer, and the hook
  stands down on it, keeping ct9/mugy's asymmetry that unarmed is visible
  and double dispatch is not.)*
- Operator's view: unchanged — `posse list` and the cockpit read as today.
  New surface: one line when a second launcher waits, naming the holder.
  *(Amended 2026-08-23: not quite unchanged — `posse ready` files by the
  same rule as the sweep, so it is a lock waiter too and queues behind a
  live fire loop. If that wait is ever judged too expensive, that is a
  design question and comes back here — not a hold-time tweak.)*
- Metric: dispatch bugs per week whose root is a store disagreement —
  expect ~zero for the three named classes; substrate-dialect
  discoveries continue at their own rate and are not this ADR's to fix.

## Alternatives rejected

- **A single-writer dispatcher daemon** (CLI and autostart enqueue; one
  process launches). The clever one — I wanted it: it makes §1 structural
  and gives the queue an API. It also builds an IPC substrate the harness
  then owns (protocol, crash recovery, client fallback), turns
  `posse dispatch` from a command the operator runs and watches into a
  request to something invisible, and re-fights herdr for the "where does
  the loop live" ground autostart already settled. flock buys the
  single-writer property for ~30 lines and zero new state.
- **A work queue with lease/heartbeat/expiry.** Triple-implements bd
  (DIRECTION's standing caution) and adds a fifth store. Heartbeats need
  a beating heart: the persona CLI won't, so dispatch would heartbeat for
  sessions it may have lost sight of — the blind spot unchanged, with
  auto-expiry double-dispatch on top. If beads itself grows leases,
  revisit; this ADR adds no state such a migration would hold hostage.
- **A per-run state machine in a new store.** The right instinct (the run
  record), the wrong home: a new file family beside the meta dir is
  another store to disagree with. The meta file already is per-run under
  Dial F; extend it.
- **Leave it: patch the prune, keep the one-pass rule.** Defensible and
  considered — most recent incidents are substrate dialects, not design.
  Rejected because two of the three race surfaces (autostart vs hand-run,
  cockpit `d` vs pass) are not covered by an operating rule only the
  coordinator follows, and the class analysis predicts new pairwise
  incidents at a steady rate. The fix is three small beads, not a
  rewrite.

## Appendix A — historical prior-art review (2026-08-21; implementation snapshots are not current status)

The operator's nagging feeling was right: every mechanism above has a
fifty-year-old name and a standard answer. This appendix names each one
and checks our fix against it — what we adopted, what we deliberately did
not, and where we are still short. Sources verified live 2026-08-20.
Verdict up front: we reached the standard answers, mostly independently;
the literature adds two corrections (beads filed) and one precise
permission (no fencing token needed — for a reason worth recording
exactly).

**The disease is TOCTOU.** "One store's momentary reading taken as
evidence about another store's durable fact" is time-of-check-to-time-of-
use (CWE-367; Bishop & Dilger 1996): any `if <check> then <act>` where
the checked state is mutable by another actor, and a snapshot — listing,
poll, exit code — is evidence about the instant it was taken, not the
instant you act. The field's answers, strongest first: make check-and-act
atomic; ask at use time; hold a lock across the window; re-validate
before irreversible action; make the act idempotent. The claim path
already had the first two (`bd update --claim` is atomic; the outcome is
read back from the bead, never the exit code); §§1–3 are the other
three, one each.

**§1 is the sysvinit-vs-systemd argument, decided the kernel's way.** A
pidfile is a snapshot of a durable claim — the file outlives the process,
the pid gets recycled — so every pidfile check races the process table.
The standard answer is state the kernel ties to the process's existence:
flock(2)'s lock belongs to the open file description and dies when the
last fd closes, so release *is* death and no staleness class exists
(man7; DDIA ch. 8). What flock does NOT give us, said plainly: it is
**advisory** — only paths that take it are serialized, and the
duplicate-verify incident (verify-after filed before the lock existed to
it; brought inside the perimeter 2026-08-23, §1 amendment) is live
proof that the perimeter, not the mechanism, is where this fails; it
names **no holder** — the stamped pid is a courtesy the code correctly
reads for nothing; it is **single-host, local-filesystem only** — on
NFS/network mounts flock degrades to emulation or a no-op depending on
mount options, so RHQ_HOME's state/ must stay on a local FS; and the
classic fd-inheritance leak (a child holding the lock past the parent's
death) is closed only because Go opens files O_CLOEXEC — a port keeps
that property or loses the invariant.

**Fencing tokens: not needed, because we have no lease — anywhere.**
Kleppmann's "How to do distributed locking" (2016) argues a lock with
*expiry* cannot protect correctness unfenced: a paused holder outlives
its lease and writes after a successor legitimately acquires; the fix is
a monotonic token checked *by the resource* (Chubby's sequencer —
Burrows, OSDI 2006; the lock service cannot protect a resource that does
not participate). The premise is expiry. flock never expires from under
a paused holder — the kernel revokes at death only, and the dead don't
write; bd claims are leases-without-expiry by this ADR's own decision.
We removed the double-holder path instead of fencing it, at the price of
manual recovery — the standard trade, taken knowingly (expiry without
fencing was the original double-dispatch). The honest residual: **manual
recovery is expiry with the operator as the clock.** Claim surgery
(`--resume`, reassignment) while a paused pass is mid-flight recreates
Kleppmann's zombie writer, and bd has no token check to reject the late
write — a real fence is not buildable without the resource
participating. Accepted: the window needs a deliberate operator act plus
a paused process, and the surgery is cockpit-visible. The revisit
trigger already stands — if beads grows leases, fence; §3's run record
is the natural home for the generation number.

**§2 is RCU's grace period plus a hazard-pointer check — with one
asterisk.** The reclamation law: unlink, wait out every actor who could
have seen the record live, and prove death *at reclaim time* — a
retired object is freed only when no announced holder names it (McKenney,
LWN 2007; Michael, TPDS 2004). Not "it looked dead when I scanned";
both. `prunable()` is both: the 5m grace over `launched:` (stamped
before the command is typed, so the record covers its own race window),
and a per-id `WorkspaceAlive` query at prune time — only
`workspace_not_found` is death; error and silence keep the file. The
create side mirrors it (`mustNotOrphan`). The asterisk: the
**unlink itself is still check-then-act** — `os.Remove` acts on the
*path*, which an interleaved create can legitimately rewrite with a
fresh meta between check and delete. Narrowed to milliseconds, not
closed — and a narrowed race is worse to debug, not safer. The
by-construction fix is cheap because §1 exists: delete and meta-write
under the launcher flock. **Closed** (rangerhq-3a5t): the unlink takes
the lock `LOCK_NB` and re-proves death inside it — locking the delete
alone would still act on evidence read outside the lock, which is the
same race one step over — and a contended lock spares the file, which
also makes nesting safe (flock is per open file description, so a pass
already holding it spares rather than deadlocks). `CreateSession` takes
the same lock around `mustNotOrphan` and its `writeMeta`, which finally
puts `posse new` under §1 — mustNotOrphan's own doc named the unlocked
create as the hole it could not close. Relaunch's two meta-destroying
steps (`clearDeadMeta`, `keepRecipe`) were the same shape and were the
residual this entry filed a bead for; both now take the lock too
(`underLaunchLock` in `relaunch.go`, each re-asking its own proof inside —
clearDeadMeta reclaim's pattern, keepRecipe `mustNotOrphan`), so §2's
by-construction fix covers relaunch's paths as well as the prune's unlink and
the create (rangerhq-9jk1/w4h5, recorded 2026-09-06 on
ranger-base-qoh87). On ordering, the reviewed alternative — write the
meta before the workspace exists — inverts badly: a meta naming no workspace is unprunable by construction
here; the current order (workspace → meta → command) plus grace is
right. Second asterisk, closed — **measured false**: herdr's allocator
is `max(live id)+1`, recomputed from the live set at every server
process start, restart and live handoff both (probe re-run on herdr
0.8.0, `scripts/verify-id-recycle.sh`; measurement table in NOTES.md
"Workspace ids recycle across a server process boundary"). So
`WorkspaceAlive(id)` proves a workspace *answers to that id* and nothing
more — the pid-recycling failure arrived exactly as predicted, one
counter over — and not-found is a statement about the id's *present*:
an id dead today can answer alive tomorrow, for a stranger. Fixed by
§2's identity arm (amendment there). What stays true and load-bearing:
not-found at ask time remains proof that the *meta's* workspace is dead
in any generation, because a workspace never changes its id while it
exists — which is exactly why the fence lives beside the death check
instead of vetoing it.

**"bd is the queue" is database-as-queue done the sanctioned way.** The
folklore says never; the field's modern answer is: fine, *if you use the
store's own claim primitive instead of inventing one* — Postgres's docs
name the queue use-case for `SELECT … FOR UPDATE SKIP LOCKED` themselves.
bd's `--claim` is that primitive: atomic claim, lost race is a clean
skip, outcome read from the bead. What we substitute for SKIP LOCKED's
connection-scoped locks is **durable claims** — deliberately, because
connection-scoped release is auto-expiry (Ringer's catalog of homegrown
DB-queue bugs), auto-expiry is redelivery, and redelivery demands the
idempotence we do not have (next paragraph). At beads-per-minute
throughput the DB side of the broker tradeoff wins on both store count
and transactionality; the rejected "work queue with lease/heartbeat/
expiry" is the same conclusion reached from the store-count direction.

**`--wait` is a visibility timeout; the discipline is at-least-once.**
Exactly-once *delivery* is not on the menu (Gray 1978's Generals
Paradox); what exists is at-least-once plus idempotent or deduplicating
receivers (Helland, "Idempotence Is Not a Medical Condition"). SQS's
definition matches ours precisely: the timeout expiring is a statement
about the consumer's *silence*, not the work's failure — NOTES's "a
`--wait` timeout is a check-in, not a verdict" is that sentence in shop
words, and the thrice-fixed bug was the cost of reading it as a verdict.
Prompting an agent is **not idempotent** — a double-prompt derails a
live session, and the duplicate verify beads are the observed shape of
un-deduplicated re-execution. Our dedup state is claims (durable,
survive restarts) plus PromptGrace — which today is per-process memory,
violating the rule that dedup state must outlive the retry horizon and
the receiver's restarts. §3 is the standard fix: `prompted:` moves into
the run record. Until it lands, the coordinator's one-pass rule is the
compensating control, as Decision already says.

**The frame under all of it: single writer, one store of record per
fact.** Thompson's single-writer principle — contention machinery exists
only because multiple writers mutate one resource — is what §1 buys for
~30 lines; the rejected dispatcher daemon was the same principle at the
cost of owning an IPC substrate. Helland's line that data leaving its
store of record "is clearly from the past and not now" is this ADR's
diagnosis in one sentence, and the corollary predicted our history: a
fact readable in N stores can disagree N(N−1)/2 ways, and four stores
gave us exactly the pairwise incident classes the Context section lists.
§3 nominates authorities instead of adding a fifth store. The predictor
going forward: the next incident class arrives with the next store, or
the next write path that skips the lock.

### Appendix sources

- CWE-367 (TOCTOU) — https://cwe.mitre.org/data/definitions/367.html ·
  Bishop & Dilger 1996 —
  https://nob.cs.ucdavis.edu/bishop/papers/1996-compsys/racecond.pdf
- flock(2) — https://man7.org/linux/man-pages/man2/flock.2.html
- Gray & Cheriton, Leases, SOSP 1989 —
  https://web.eecs.umich.edu/~mosharaf/Readings/Leases.pdf · Burrows,
  Chubby, OSDI 2006 —
  https://research.google/pubs/the-chubby-lock-service-for-loosely-coupled-distributed-systems/
  · Kleppmann, "How to do distributed locking", 2016 —
  https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html
  · Sanfilippo's rebuttal — http://antirez.com/news/101 · Kleppmann,
  *DDIA* ch. 8 & Part III — https://dataintensive.net/
- McKenney & Walpole, "What is RCU, Fundamentally?", LWN 2007 —
  https://lwn.net/Articles/262464/ · Michael, Hazard Pointers, IEEE TPDS
  2004 — https://www.cs.otago.ac.nz/cosc440/readings/hazard-pointers.pdf
- PostgreSQL SELECT locking clause (SKIP LOCKED) —
  https://www.postgresql.org/docs/current/sql-select.html · Ringer 2016,
  via Wayback (original dead) —
  https://web.archive.org/web/*/blog.2ndquadrant.com/what-is-select-skip-locked-for-in-postgresql-9-5/
- Gray, "Notes on Data Base Operating Systems", 1978 —
  https://jimgray.azurewebsites.net/papers/dbos.pdf · Helland,
  "Idempotence Is Not a Medical Condition", ACM Queue 2012 — DOI
  10.1145/2181796.2187821 · AWS SQS visibility timeout —
  https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
  · Stripe idempotent requests — https://docs.stripe.com/api/idempotent_requests
- Thompson, "Single Writer Principle", 2011 —
  https://mechanical-sympathy.blogspot.com/2011/09/single-writer-principle.html
  · Helland, "Life beyond Distributed Transactions", CIDR 2007 —
  https://www.cidrdb.org/cidr2007/papers/cidr07p15.pdf · Helland, "Data
  on the Outside versus Data on the Inside", CIDR 2005 —
  https://www.cidrdb.org/cidr2005/papers/P12.pdf
