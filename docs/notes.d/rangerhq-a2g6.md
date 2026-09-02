## PAUSE: a stop a human writes, and the oversight it does not stop (rangerhq-a2g6)

`posse pause "<why>"` stops the shop dispatching; `posse resume` lifts it.
Between them, every launch path declines with one line naming the pauser
and the reason. Design: ADR 0029 §3 (docs/adr/0029-governance-surface.md);
§1–2 shipped the read half (G8, rangerhq-81y0), this bead is the write
half and the gate.

Three verbs, kept distinct, and the whole point of the bead is that they
stay distinct:

| verb | who | scope | heals |
|---|---|---|---|
| SKIP | the mechanism | this pass | itself, next pass |
| PAUSE | the operator or the coordinator | until told otherwise | never — a human lifts it |
| STOP | the operator | the loop, autostart, the install | a human reinstalls |

### What the code is

- **internal/rhq/pause.go** — `PauseActor` (who may), `WritePause` /
  `ClearPause` (the file), `PauseLine` (the words). govern.go keeps the
  read half, because a pause is G8 and G8 is a governance row.
- **internal/rhq/dispatch.go** — the gate, in two places: the top of
  `Run` (watch, hand-typed, autostart — every pass) and the top of
  `LaunchBead` (the cockpit's `d`, the one launcher that does not go
  through `Run`).
- **cmd/posse/main.go** — the two commands, and the help entry.

### Five decisions taken in code

**The gate goes ahead of the load guard, not beside the plan guard.**
§3 says "checks it first (alongside planGuard, one read under the fire
loop's entry)". Both readings cost one syscall and neither forks, so
ordering is about which stop gets NAMED: a paused shop that answered with
"load guard: loadavg 112" would be the surface crediting the machine for a
human's decision. Pause reads first, and a paused pass says so.

**The read is not the decline** (corrected by ranger-base-171f). The pause
file is read at the top of `Run` for the reason above; the pass declines
where §3 puts it, at the fire loop's entry, below the epilogue. As first
shipped these were one statement — the read returned from the whole pass —
so *below the gate* was true of the source order and false of the control
flow, and everything named in the next paragraph silently stopped for the
length of the pause. Two statements now: `paused := ReadPause(...)` and its
line at the top, `if paused.Present && !d.DryRun { return 0, nil }` just
above the ready scan.

**Between the read and the decline: the reap, the land sweep, the guard
readings, `credentialExpiry`, verify-after, the bead-loss census.** They
are the pass's epilogue for work that ALREADY ran, and a pause is a stop on
spending, not an instruction to abandon what the shop is holding. A pause
is meant to be the safe alternative to `kill` and gets held for hours, and
for those hours a paused shop that stopped reaping regrew exactly the
per-bead session graveyard ranger-base-v674 was cut to drain, left closed
beads' trees unlanded, filed no verify beads (ADR 0006 §3 — oversight, not
spend), and froze G4's streak clock at whatever it read when the pause
landed. The load guard below the read is the one reading that still returns
from the whole pass, epilogue included, because on a fork-starved box the
epilogue's own readings hang — pause never had that reason. The pulse
goroutine never enters `Run` at all — it is started by `Watch` and ticks on
its own clock. `TestAPausedShopStillPulses` pins the pulse (gating
`pulseOnce` on the pause file is the mutation that kills it) and
`TestAPausedPassStillRunsTheEpilogue` pins the epilogue: one rig run twice,
differing in nothing but the pause file, asserting the reap and the filed
verify bead directly in both arms.

The autostart hook is deliberately not gated either: a paused shop that
refused to *start* its watch loop would be a paused shop with no pulse,
which is the `kill` this verb exists to be an alternative to. The loop
starts, every pass declines with its one line, and the ticker delivers.

**`--dry-run` reports and gets out of the way**, the load guard's carve
for the load guard's reason: a dry pass launches nothing, and the one
command someone types to ask *what would happen if I resumed* must not be
the one that goes quiet.

**The why is round-tripped, not escaped.** `why:` is free prose read back
through the flat-YAML reader every pass uses (`YamlGet`), and that reader
is line-based and treats whitespace + `#` as a comment. Two halves:

- The writer flattens whitespace, newlines included. That is not
  cosmetic — it is what stops `pause "x\nby: someone-else"` from writing
  the `by:` field.
- What flattening cannot fix, the writer *reports*: `pause "rollout #3
  broke the meter"` stores the whole line, the reader gives back
  `rollout`, and the command says so on stderr naming what it stored.
  A stop must never fail over its own formatting, so this is a warning
  and never a refusal — but a why that comes back shorter than it was
  typed is the surface quietly editing the reason the shop stopped, and
  it does not get to do that silently.

Inventing an escaping dialect for one field was the alternative, and it
would have made state/pause.yaml a file only posse can read.

### Two rules the tests hold

- **No auto-PAUSE.** `TestNoMechanismEverWritesThePauseFile` runs a pass
  the load guard skips and asserts no pause file appears. Latching a
  transient reading into a durable stop trades a self-healing skip for a
  flapping meter parking the shop overnight (§3, and the ADR's own
  rejected alternative).
- **A second pause keeps the first.** Overwriting would move `at:`
  forward and lose the reason the shop actually stopped for. `resume`,
  then `pause` again, is how a why changes.

### Observables (for the verify bead)

- `posse pause "x"` then a hand-typed `posse dispatch`: zero sessions
  launched, one line naming pauser and reason, and no ready scan — but the
  epilogue's own `bd` reads still happen (ranger-base-171f: this line first
  read "and no `bd` forked", which is how the pass came to skip its
  epilogue and how a seven-mutation sweep failed to notice).
- The cockpit's `d` on a paused shop: refused, same words.
- `posse status`: G8, URGENT, the same line.
- A paused shop with a blocked session still pulses the coordinator.
- `posse resume` on an unpaused shop: exit 0, "not paused".
