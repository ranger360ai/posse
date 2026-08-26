# ADR 0011 — The dispatch model: bd is the queue; the pass gets a lock, a safe prune, and a run record

*Status: accepted 2026-08-20 · amended 2026-08-23 (§1 third holder;
§2 identity arm — restored 2026-08-26, see §2) · owner: architect*

> Restated from the private archive of the instance this harness was
> developed in. The incidents this ADR reasons from happened in that
> instance; they are named here by shape (the lost-claim incident, the
> meta-sweep race…) rather than by tracker id, and persona names are
> restated as roles.

## Context

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
the fire loop (gather runs unlocked — it only reads and judges) and by
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
positive identity. Tests: `internal/rhq/metaidentity_test.go`; promote
gate: `scripts/verify-prune-guard.sh`. This amendment was written
2026-08-23 in the private instance and lost in the private→public
restatement; restored 2026-08-26 from the closing record, mechanisms
re-verified against this tree.)*

**3. The session meta is the run record.** Dial F already made
session ≈ bead-run; the meta file is the record dispatch wrote and then
declines to trust. Promote it: add `bead:` (the bead the session was
created for) and `prompted:` (persisted, so PromptGrace holds across
processes — today the cockpit's `d` and a running pass cannot see each
other's prompts). Wherever dispatch needs a run fact, it reads the record
it wrote instead of inferring from a name pattern or a snapshot; the
holder join becomes a lookup.

**Explicitly kept:** the pass shape (burst, fire-then-gather, serial
launches), the busy-key rule, judge-by-bead, the wait ladder as landed,
`--watch` backoff. Events-instead-of-polling stays open and orthogonal —
it changes *when* dispatch looks, not *what it trusts*. The
coordinator's one-pass operating rule stands until §1 lands, then
retires.

## Consequences

- Implementation, dependency-ordered, one session each: **the safe
  prune** (§2, fix directions chosen above) → **launch lock** (§1) →
  **run record** (§3, deps on both — it touches both files' territory).
  A doc bead fixes the stale records (dispatch.go header, DIRECTION
  sketch). ADR 0006 amended in this commit.
- Handoffs: **security** — a pruned meta also deletes the crew mark, so
  a wipe can turn the operator's own conversation back into
  fleet-promptable; that is the data-loss shape of the meta-sweep race,
  assess exposure. **ops** — once §1's helper exists, migrate autostart's
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

## Appendix A — the prior art, checked (2026-08-21)

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
under the launcher flock (correction bead filed). On ordering, the
reviewed alternative — write the meta before the workspace exists —
inverts badly: a meta naming no workspace is unprunable by construction
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
