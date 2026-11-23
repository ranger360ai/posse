# ranger-base-25otk — the settle gate was handed its own evidence and dropped it

The ruling is ADR 0076 Amendment 12. This file is the measurements behind it
and the mutation battery.

## The census

MEASURED 2026-11-23 (UTC; 2026-11-22 -05:00 local) over the live fleet's two
dispatch pass logs — `state/dispatch-watch.log` and `.log.1`, 66,476 lines,
the earliest `== dispatch --watch armed ==` line 2026-11-10 18:53:28 — read
while the live log was still being appended to, so every count is that
instant's. This is ranger-base-hcig1's census re-read on a log 189 lines
longer; nothing of its finding moved.

| shape | n |
| --- | --- |
| `prompting on its … anyway` — the settle ladder's concession | 857 (583 + 274) |
| `never became promptable` — the settle ladder's refusal | 0 |
| `herdr never saw a screen it recognizes` | 0 |
| `✗ <bead> …` — the launcher's abandonment shape, all reasons | 46 |
| of those, the occupant's own `Wait` running the window out | 5 |

Two readings decide the ruling rather than describe the incident:

- **857 against 0.** The concession is the only way this arm has ever exited
  on this instance. `agent explain` is an ADR 0076 §4 verb this multiplexer
  does not have, so `lastWhy` is permanently empty, the deadline arm takes
  the concession every time, and the launch is decided by whichever reading
  was LAST. Each of those 857 launches spent the runtime's whole
  `startup_wait:` first — 45s for claude on this instance, which declares
  none and so takes `DefaultStartupWait`.
- **The field was there on the first poll of every one of them.**
  `RexMux.Wait` fills `MuxReading.Seen` on every settled answer, and ADR 0076
  §3 calls that field "the Rex form of herdr's `Seen()`". ADR 0013 §1's
  settle row's observable IS `idle`/`done` *Seen()*. `awaitSettled` read
  `res.State` and not `res.Seen`.

The five abandoned launches, from settletimeout.go's census (unchanged):

| bead | window left when the last `Wait` began | last reading |
| --- | --- | --- |
| ranger-base-72z36 | 16.446s | working |
| ranger-base-k0hd6 | 2.203s | working |
| ranger-base-kzbg8 | 32.763s | unknown |
| (two more, below) | 42.948s and 26.541s | unknown, both |

Open beads, so named by id and nothing else (AGENTS.md, "Landing the plane"):

- ranger-base-1mxhx
- ranger-base-al0d0

The first two are the ones this bead is about: they read settled through most
of their window and went `working` at the end of it, so they were promptable
tens of seconds before the launcher threw them away. The three `unknown` ones
are no record at all, or a record whose owner is the block's own shell or is
no longer live in it (§3's three guards) — nothing said a runtime held those
keyboards, and Amendment 12 refuses them exactly as today.

## Two facts about the occupant that shape the fix

MEASURED 2026-11-23 by reading this HEAD:

- **Rex cannot answer a settled state WITHOUT evidence.** `rexReading` maps an
  OSC 7501 state onto the slot's vocabulary in its `Seen` branch and nowhere
  else; all three not-a-state cases read `MuxUnknown`, which `Wait` never
  returns. So on this occupant the ladder's `explain` poll and its §4
  concession are now unreachable, and the concession arm survives for an
  occupant that answers §2's verb with a state and nothing else — the shape
  herdr's own envelope has.
- **Nothing outside `internal/posse/muxrex.go` calls `Mux().State`** (grep
  over the package's non-test sources). That verb is one of §2's ten, not a
  §4 one, so it answers on every occupant — and no ladder asks it. That is
  the residual, filed as ranger-base-q5fee: the readiness gate and the
  delivered wait still take the concession with no occupant reading at all.

## What landed

- `muxSettledGate` (internal/posse/muxsettle.go): the rule, in one place —
  settled AND seen — with the census and the risk argument in its head
  comment.
- `awaitSettled` (internal/posse/dispatch.go) reads it immediately after the
  blocker arm, so a blocker is still handed back by name.
- `AgentDetection.MuxSeen` (internal/posse/herdr.go), a third arm of `Seen()`:
  the evidence has to ride into the detection the gate returns, because that
  detection is the reading that TYPES (ADR 0066 D1 as amended,
  ranger-base-o1aoi) and a stall record saying `seen: false` over a gate
  opened on evidence would be a false record.
- `muxnoverb_qa_test.go`'s arm 4, split: 4a is the accept against the real
  Rex occupant, 4b keeps ranger-base-l32dl's "this site reaches the one
  renderer" against an occupant that can still get there.
- Pins: `internal/posse/muxsettle_qa_test.go`, six arms. Arm 6 enumerates the
  package's own sources, so it is a tree-wide pin living in internal/posse and
  needs a door (ADR 0098, ranger-base-ik44f's class): it joins
  $(QA_PKGCENSUS_PINS), beside the degrade-sentence pin whose subject it
  shares, with the one-way enumeration and the derived count in
  `internal/treepins/treewidedoor_qa_test.go` updated with it. The first
  `make test` of this bead failed on exactly that — `TestQAEveryTreeWidePinHasADoor`,
  which is the census doing the job it exists for.

## Did the pins catch the fix being taken out

Five mutations, planted one at a time against the filtered pins (the battery
is a scratchpad script, not tracked). MEASURED 2026-11-23 on this box:

| mutation | verdict |
| --- | --- |
| the accept arm is removed (the bug as filed) | red — the gate spends the window (arm 2), the settled-then-working seat is abandoned (arm 3), and the real occupant's accept is gone (muxnoverb 4a) |
| the rule reads the evidence and not the state | red — arm 1, on `working` with evidence: the gate would open mid-turn, which is rangerhq-3hb5's race |
| the rule reads the state and not the evidence | red — arm 1 on `idle` without evidence, arm 5 on the herdr envelope, and muxnoverb 4b, which stops conceding and starts opening |
| `Seen()` stops reading the occupant's answer | red — arms 1, 2, 3 and muxnoverb 4a: the gate opens and the record cannot say what opened it |
| the herdr envelope derives evidence from its state | red — arm 5: herdr's `agent wait` would start licensing settles it has no evidence for |

The second and third rows are the two halves of the rule, and each is caught
by the other half's absence. The last row is the blast-radius pin: the arm's
whole safety argument is that herdr cannot reach it.

## The timing

The hermetic reading, from arm 2: **0.02s against a 4s window**, at the
production poll (250ms). Before the fix the same fixture — an occupant
answering `idle` on every call — ran the window out at that poll and then
conceded. The production arithmetic is the same one poll against `rt.Wait()`:
45s per launch on this instance.

## Not closed here

- ranger-base-q5fee, the residual above: the readiness gate and the delivered
  wait.
- ranger-base-hcig1's own arm is untouched. A `Wait` that RUNS OUT still
  carries its last reading for a caller to refuse on and may open nothing
  (Amendment 11 D4, re-stated as Amendment 12 D2) — what changed is only the
  wait that RETURNED.
