## A hot pool cannot be its own overflow (ranger-base-ay0h)

`plan_guard_overflow: claude` — the runtime the plan guard meters — was
accepted as an overflow target. With `plan_guard_5h: 70` and the window at
78%, dispatch printed `plan 5h at 78% > 70% — overflow claude, 0/1 in 7d`,
launched the bead on `claude` with `[claude ← overflow]`, and wrote a claude
line to `overflow.log`. Every rung of ADR 0010 §2 passed honestly: the bead
was on the guarded meter, the tier was not `strong`, the PID had no
`overflow: false`, parity on the target was clean (it is the *same* runtime),
and the cap had room. The ladder worked; its target was the thing it exists
to spare.

The reading is what makes the move a judgement (§5's whole argument): the
guard read *this* pool and said it is over threshold, so "send the work here
instead" is not a judgement on a second pool, it is the guard cancelling
itself — with a ledger entry recording the cancellation as an overflow.

Two guards, because the invariant belongs to the value and the diagnosis
belongs to the operator:

- `PlanGuardOverflow` refuses `plan_guard_overflow: <guarded runtime>` and
  names it on stderr, exactly like the missing-cap arm. On-meter beads then
  park on the ordinary tripped-guard line, `— skipped`, and nothing is
  ledgered. Off is not enough on its own: silent-off reads as an ordinary
  tripped guard, and a mistyped target would never be found.
- `Overflow.On()` carries the rule too, so an `Overflow` built any other way
  cannot move a bead either. It already refused a capless runtime for the
  same reason.

Pins: `TestQAOverflowTargetCannotBeTheGuardedRuntime` (the pass: zero
launches, empty ledger, the target named on stderr), the `GuardedRuntime` row
of `TestPlanGuardOverflowConfig`, and `TestOverflowOn`. Each guard was
mutated out separately — dropping only `On()`'s copy leaves the QA pass green
through the config reader, which is why the third pin exists.
