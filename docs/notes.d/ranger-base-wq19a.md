# ranger-base-wq19a — the launch share, built: charge per class, owner first per reading, lend the rest

ADR 0011 §5 (amended 2026-11-14 by ranger-base-j11mf) is the decision and
`docs/notes.d/ranger-base-j11mf.md` holds the measurements it rests on. This
fragment holds what the code does, the one thing the design did not say, and
the dry-run lines the operator promotes the two keys through — the line
j11mf's own fragment said this bead's done-when would have to produce,
because when it was written no binary read the keys.

Built 2026-11-15. `internal/posse/launchshare.go` is the whole mechanism;
`internal/posse/launchshare_test.go` is the pins.

## Where it lives

- `LaunchShareFor(max)` resolves `launch_share_label:` and `launch_share:`
  against the `-n` they split, and answers either both allotments or ONE
  sentence naming the key that made the split dormant. Dormant arms, in the
  order §5 states them: `-n 0`, label unset, count unset, count not a count
  (`0` and `two` alike), share not BELOW `-n`, and a loaded non-coordinator
  PID carrying the label.
- `fireLoop` is the realization, because it is the one choke point the fire
  pass and every refill both run. Every attempt is charged to the class of
  the bead it was made for, and the returned `shareSpend` is what the caller
  books against the epoch — through `bookSpend`, the one site that moves the
  total and the two class counts together.
- `epochClass` on the Dispatcher, beside `epochAttempts`, rolled by
  `rollEpoch`. Same lifetime, same per-process bound, for the reason
  epoch.go gives at length about `epochAttempts` itself.

## Owner first, then lent, is TWO READINGS OF ONE QUEUE

The shape worth knowing, because it is not the only one that would satisfy
§5's prose. `fireLoop` walks the queue twice:

- **Round 0, the owner's pick.** Queue order. A bead whose class is already
  at its allotment is DEFERRED — not skipped, and nothing is printed, because
  round 1 will very likely take it.
- **Round 1, the lend.** Exactly the beads round 0 deferred, bounded by
  nothing but the epoch's total room.

Two readings rather than a priority sort because each bead must be judged
ONCE: every guard below the class check prints or holds a seat, so a bead
walked twice would have its skip line written twice. The deferral therefore
sits above routing and below the two filters that answer "this is not a
launchable bead at all" (a malformed id, a question for the operator), which
keeps those reporting in queue order.

**What that placement costs, said out loud: the held count is beads in the
class the owner's reading did not JUDGE, not beads that were ready and
waiting.** The class bound is read above routing, so a held bead may well
have been unroutable or on a busy seat. That is the honest number for the
question the line answers — is the allotment the thing standing in the way —
and the per-bead reasons arrive in the next reading's report or the next
pass's. The live run below held 12 and the queue it held them from is full of
unroutable rows, so this is not a hypothetical.

**A class never strands a bead while total room remains.** Within one reading
the only exhaustion that can hold work back is the epoch's own ration, which
is why the one exhaustion line that names a shortfall is `epochRoom`'s, with
the split. A class at its allotment with room left over is a lend, and the
lend happens in the same reading.

## MEASURED through the consumer, 2026-11-15

A scratch `RHQ_HOME` holding the live config plus the two lines, with
`agents`/`envs`/`runtimes`/`recipes` symlinked to the live home — the shape
j11mf validated the config through. `posse dispatch --dry-run --no-reap`,
cwd this worktree, exit 0 each time.

Armed, `launch_share_label: project` / `launch_share: 2`, `-n 6`:

```
◷ launch cap: -n 6 = 6 launch attempt(s) per 1h0m0s EPOCH, not per pass (ADR 0028 §2; autostart_max_beads:/dispatch_epoch:) · launch share: 2 of them reserved for the project class and 4 for the remainder, per EPOCH — an idle class's allotment is lent to the other and never repaid (ADR 0011 §5; launch_share_label:/launch_share:)
```

and the pass under it, abridged to the lines the share wrote. Three of the
four remainder ids are elided and the fourth is this bead's own: two of them
are open residuals under `vuln-private` beads, which AGENTS.md "Landing the
plane" says are named by bead id and nothing else — and a launch line is a
word beside an id, so here they are named by nothing. What the format does
is visible on the two lines that matter.

```
· ranger-base-wq19a → dinesh (assignee) in session … [standard via PID] [class: remainder]
· <three more remainder beads>                       …                  [class: remainder]
· ranger-base-ze7xz → jian-yang (assignee) in session … [standard via PID] [class: project]
· ranger-base-5mtwr → gilfoyle (assignee) in session … [standard via PID] [class: project]
◷ launch share: the remainder class spent its 4 attempt(s) this epoch — 12 more bead(s) in it were held for the lending reading, which launched 0 of them on room the other class left (ADR 0011 §5)
6 bead(s) would be dispatched
```

**That reading is the feature working on live data, and it contradicts one
line of j11mf's fragment** — which recorded that "nothing in the queue
carries `project` yet, so this proves only that the draft config loads". Two
beads carry it now (`ranger-base-ze7xz`, `ranger-base-5mtwr`), both at the
BACK of the queue behind a remainder class deep enough to spend the whole
ration. Unshared, this pass would have launched six remainder beads and
neither project bead; with the share, the remainder is held to its four and
the project class gets its two. The share is not inert on this instance on
the day it landed.

And the dormant arms, from the same home, each at `-n 6` except the first.
`-n 0` is the one an operator is most likely to hit, because it is the one
where the share's own two keys are fine:

```
◷ launch cap: -n 0 — no cap on launch attempts (set one with autostart_max_beads:; the accounting epoch is 1h0m0s) · launch share DORMANT: -n 0 is no bounded ration to split (autostart_max_beads:) — nothing is split (ADR 0011 §5)
◷ launch cap: -n 6 = … · launch share DORMANT: launch_share: 6 is not below -n 6 — it would leave the remainder zero and starve every other class by configuration — nothing is split (ADR 0011 §5)
◷ launch cap: -n 6 = … · launch share DORMANT: launch_share: "two" is not a count of launch attempts — nothing is split (ADR 0011 §5)
```

## Routing is untouched, and the PID arm proves it from the other side

0 of this instance's PIDs carry `project` (j11mf measured it statically; the
armed run above is the dynamic witness — every launch routed by assignee,
and the `project` label appears in no route line). The dormant arm for a PID
that DOES carry the label was exercised in a fixture, and what it showed is
the reason §5 refuses that configuration: with one PID labelled
`[project, code]`, routing seats that PID on every `project` bead and a
second share bead waits with `project lane busy` — a §4 lane, which is the
thing a budget class must never become. Dormanting leaves that §4 behaviour
exactly as it was; arming on top of it would have made the share look like
the cause.

## What is NOT built here

- **No dollar share.** `budget_pass:`/`budget_day:` are not split. §5's own
  measurement is why: 66 of 101 epochs ran out of launch attempts and the
  dollar brake refused 0 launches, so the attempt count is the contended
  resource. The exit hatch is in the record, for the day a budget line first
  refuses.
- **No durability.** A supervised restart hands back the whole split, the
  same way it hands back the whole `-n`.
- **No repayment.** Lending is decided per reading and never repaid, so a
  class that becomes ready after the lend has spent the epoch waits for the
  turn. §5 rules out starving, not waiting.

## Not closed here

- ranger-base-ndini (the operator picks `k`)
- ranger-base-7qtci
