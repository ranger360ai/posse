# ranger-base-hcig1 — the settle window's two exits, and what the illegible one cost

MEASURED 2026-11-23 on this box (linux/amd64), over the live fleet's two
dispatch pass logs — `state/dispatch-watch.log` and `.log.1`, 66,287 lines,
the earliest `== dispatch --watch armed ==` line 2026-11-10 18:53:28 and the
latest 2026-11-21 07:11:54, read while the live log was still being
appended to (so every count here is that instant's).

## The census

`✗ <bead> <reason>` is the launcher's abandonment shape. 46 of them, by
reason:

| reason | n |
| --- | --- |
| `no agent detected in <session> after 45s — check the session (posse peek <session>)` | 16 |
| `claude launch refused in <session>: no program_status record from the runtime within …` | 8 |
| `claude launch refused in <session>: claude in <session> ENDED before it reported …` | 7 |
| **`herdr: no idle\|done\|blocked state in block:<uuid> within <n>s (last reading: <state>) (timeout)`** | **5** |
| `refused: claude at cage <tier>, tier strong does not realize every gate` | 5 |
| `claim lost: <operator> holds it` | 4 |
| `herdr: typed the prompt into <block> but no turn started within … (agent_prompt_stalled)` | 1 |

Of the 46, 21 are a bounded wait that ran out **on the seat** rather than on
a cage, a claim or a runtime that ended: the first row and the bold one. They
are the same fact — a window closed with the last reading not settled — and
only the first row's spelling can be acted on. The five:

| bead | window left when the last `Wait` began | last reading |
| --- | --- | --- |
| ranger-base-72z36 | 16.446s | working |
| ranger-base-k0hd6 | 2.203s | working |
| ranger-base-kzbg8 | 32.763s | unknown |
| (two more, below) | 42.948s and 26.541s | unknown, both |

The two the table cannot spell are open beads, so this file names them by id
and nothing else (AGENTS.md, "Landing the plane"):

- ranger-base-1mxhx
- ranger-base-al0d0

The bead that filed this named three of the five and counted the
abandonments as 19; the full census over both logs is 46/21/5. What moved is
the denominator and the completeness of the list, not the finding.

Two counts decide the design rather than describe the incident, same corpus:

- **856** `prompting on its … anyway` lines — the settle ladder's concession,
  which fires when no poll in the window ever produced a readable detection.
- **0** `never became promptable` lines, and 0 `herdr never saw a screen it
  recognizes` — the ladder's refusal arm, which has never fired on this
  instance.

That ratio is what `agent explain` being a §4 verb this instance's
multiplexer does not have looks like from the log: the diagnostic is
permanently unavailable, so `lastWhy` is permanently empty and the deadline
arm concedes every time. A timeout folded into that arm unchanged would
therefore not have been refused at all — it would have PROMPTED a pane whose
last reading was `working`, which is rangerhq-37c's race. That is the whole
reason the timeout is an arm of its own.

## What landed

- `MuxWaitTimeout` (internal/posse/mux.go): the refusal carries the last
  reading on the ERROR, wrapping the `timeout` envelope so `IsHerdrCode`
  still matches it at the four live sites that branch on the code. ADR 0076
  Amendment 11 has the decision and its four parts.
- `RexMux.Wait` returns it (internal/posse/muxrex.go). The RESULT is still
  empty, which is §2's Wait row unchanged.
- `refuseUnsettled` (internal/posse/settletimeout.go): the ladder's refusal,
  with the deadline arm's own sentence and a D1 record under
  `RuleSettleGate`. No pane capture, by the rule `logBlockedRefusal` already
  argued for its own D1 branch.
- Pins: `internal/posse/settletimeout_qa_test.go`, five arms.

## Did the pins catch the fix being taken out

Four mutations, planted one at a time against the filtered pins (the battery
is a scratchpad script, not tracked):

| mutation | verdict |
| --- | --- |
| `awaitSettled` leaves by the occupant's error raw (the bug as filed) | red — the sentence loses the session, the peek and the patience, and no record is written at all |
| the refusal carries no last reading | red — arm 1 on the reading, arms 2-3 on a refusal that cannot name the state |
| `MuxWaitTimeout.Unwrap` removed | red — `IsHerdrCode(err, "timeout")` stops matching, so the ladder never reaches its own arm and the raw error comes out |
| the timeout falls THROUGH into the deadline arm | red — **the gate OPENS, `status "idle"`**, on a seat whose record never settled |

The last row is the measurement the design argument above rests on: with the
fall-through planted, a pane that reported `working` for the whole window is
handed back as a settle for the launcher to type at.

## Not closed here

- ranger-base-p51zu closed the orphan seat's visibility with G19; nothing
  here touches it.
- The larger residual is the one ADR 0076 Amendment 11's last consequence
  names: on a §4-missing instance the ladder spends the whole startup wait
  and decides on whichever reading is last, while `Wait` has been handing it
  a `Seen` it drops on the floor — and §3 calls that field the Rex form of
  herdr's own. Two of the five above had been reading settled for most of
  their window, so that is the change that would stop the abandonments
  rather than make them legible. Filed as ranger-base-25otk.
