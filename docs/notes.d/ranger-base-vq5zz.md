# ranger-base-vq5zz — the last good reading keeps gating

*2026-09-10 · code lane · amends ADR 0010 §5's table · code
`internal/posse/dispatch.go` (`blindGuard`, `staleGate`),
`internal/posse/blindheadroom.go` · pins
`internal/posse/blindgrace_test.go` (arm 2)*

## What was wrong

The plan guard's blind window had two rules stacked in the wrong order.

`plan_guard_blind_max:` (default 10m) is quiet tolerance: under it a blind
unattended pass printed one stderr line and ran. Past it, `blindFork` asked
the meter's last successful reading whether it licensed the pass at all
(`PlanBlindRefusal`, added 2026-08-31 for ranger-base-c3vqe's nineteen blind
hours).

So the refusal was reachable **only past the grace**. For the first ten
minutes of every blind stretch, a reading the guard had been skipping on
gated nothing.

MEASURED 2026-09-07 (pass 77, watch pid 63181, binary 0.4.0+062b765e). The
guard had read over the operator's weekly threshold since pass 66 and
skipped every hire for eleven passes. The twelfth probe answered 429. The
same pass printed

	plan guard: usage endpoint rate-limited, not asking again for 57m —
	pass not gated

and hired four seats. `posse status` had the condition as URGENT G5 ("plan
guard blind — monitoring itself is broken"); the hire path did not consult
it. The operator refreshed the credential that evening and confirmed the
last reading had been real, so those were hires past the brake in fact, not
only in reading.

This is the 2026-08-31 shape one rung earlier: blind for MINUTES rather than
hours, and the last good reading was already above the brake.

## The change

One reordering, no new rule and no new knob. `blindGuard` asks
`PlanBlindRefusal` **before** the grace, the arming check and the ledger
scan; `blindFork` no longer asks it at all and is now only ever reached for
a reading that left room or for no reading. Under the refusal an unattended
pass sets the same per-bead park reason it has always set, so every on-meter
candidate prints a park line of the shape `plan guard: blind <age> (<the
429>), last reading <window> at <pct> is over <the threshold key the
operator would edit>, read <age> ago — skipped`, and nothing is claimed. The
refusal text itself is unchanged from 2026-08-31; the fixture numbers behind
it are ADR 0018's published pair, not this instance's.

Two things deliberately kept:

- **Attended still fails open.** A hand-run pass has the witness the whole
  blind window is premised on (ADR 0018 §1, pinned by
  `TestHeadroomRuleIsUnattendedOnly`). What it gains is the number: the
  stale reading and its age now ride the `pass not gated` line, because a
  witness told "rate-limited" and not "over your weekly threshold six
  minutes ago" is watching the wrong one.
- **The `plan_guard_blind_max:` hatch (set to zero) still means never fail
  closed**, this brake included. It is the one way to hire against a stale braking reading and
  it is a sentence the operator wrote in their own config; widening the rule
  into the hatch is a decision, not a diff, and is pinned as such
  (`TestBlindGraceHatchIsUntouched`).

`PlanBlindRefusal`'s clause now ends at the reading's age. "a dollar cap is
not a brake on the plan window" moved to the call site and is appended only
when Dial E is armed — the refusal now also holds passes on shops that armed
no cap, where that sentence would name a brake nobody set.

## What was verified, and how

`go test -tags posse_arm2 -run 'Blind|Plan|Headroom|Degrade|Guard'` green
(82.7s), the same filter green on arms 1 and 3, and the full `make test`
recorded on the bead.

Mutation-checked per pin rather than per count: each mutation below was
applied to a clean tree, the named test run, the tree restored and asserted
clean.

| Mutation | Red, and only these |
|---|---|
| `staleGate` returns `""` — the defect itself, restored | the incident pin, the no-caps pin, the attended pin — and, run against the whole blind suite, the three 2026-08-31 pins too, because the same edit is what removed the refusal from `blindFork` |
| drop the hatch check in `staleGate` | `TestBlindGraceHatchIsUntouched`, `TestBlindMaxZeroIsUntouchedByTheHeadroomRule` |
| drop `d.Unattended` from the park condition | `TestBlindGraceAttendedRunsButNamesTheStaleReading`, `TestHeadroomRuleIsUnattendedOnly` |
| append the dollar-cap sentence unconditionally | `TestBlindGraceParkWithNoCapsSaysNothingAboutDollars` |
| refuse on any reading, not on the rule's verdict | `TestBlindInsideTheGraceRunsWhenTheLastReadingHadRoom`, `TestBlindStillDegradesWhenTheLastReadingHadHeadroom` |
| refuse on NO reading (park on ignorance, the 2026-08-26 shape) | `TestBlindInsideTheGraceWithNoReadingEverIsUnchanged`, `TestBlindWithNoReadingEverIsUnchanged` |

The last two are separate mutations on purpose: a rule that refuses on any
reading and a rule that refuses on none are different wrongs, and one
mutation cannot red both controls — the no-reading arm returns before the
verdict is computed.

The controls matter as much as the incident arm: without them the change
reads as "park whenever blind", which is the 2026-08-26 outage (a measured
hour of zero dispatch on a machine that had no reading at all) coming back.

## Not done here

- **G5 still fires only past `plan_guard_blind_max:`** (`govern.go`
  `blindPast`). The incident's coordinator row was correct and timely; what
  was missing was the hire path consulting the reading, which is what this
  bead fixed. A G5 that also fires inside the grace when the last reading
  refuses would be a governance-surface change, not a guard one.
- **The cooldown is untouched.** Shortening the 429 backoff was named on the
  bead as the thing not to do; nothing here reads or writes it.
