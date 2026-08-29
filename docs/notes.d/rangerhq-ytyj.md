## A 401 is a credential condition, not blind weather (rangerhq-ytyj)

The plan guard read every non-200 as one state — no reading — and printed
the status inside blind-time accounting: `plan guard: blind 40m (usage
endpoint returned 401 Unauthorized) — pass skipped` (2026-08-22). Every word
true; nothing in it says what to do, and nobody was told until the operator
read a log.

What landed, and the line it must not cross: **ADR 0018 §2 still holds.**
Park-vs-degrade reads no diagnosis string, and none of this changes a
threshold, a clock or a verdict. Three layers move, all of them
observability:

- `AuthFailure` beside `RateLimit` in `planusage.go` — the harness's type,
  the adapter's status line. **401 and 403 are one class and two
  sentences**: 401 is stale and an interactive refresh clears it; 403 is a
  credential that was never entitled to plan windows, and its sentence may
  never contain "refresh" (ADR 0019 D2 as amended — refreshing a setup token
  produces the same 403 forever). Anything branching reads `Code`, never the
  sentence.
- Every line that prints the read error now names the class for free — the
  dispatch park line, the fail-open stderr line, `state/plan-usage.log`,
  `posse status`. That is why the fork needed no branch in `blindFork`.
- G5 keeps its row (ADR 0029's table is closed at nine) and forks its
  **key**: `guard-credential:401` vs `guard-blind`.

**The key is the deliverable, not the detail.** `pulsePromptText` joins
`set.Keys()` — the pulse's prompt carries keys and never details — so a
coordinator woken by a credential outage was being handed `guard-blind` and
sent looking for an endpoint problem. Changing the key is what changes what
the coordinator is told, and the fingerprint it feeds is what makes it
exactly one pulse per stretch.

**Why the blind budget still gates it**, against the bead's own "first 401"
reading: a 401 *does* self-heal without a human — `claude` refreshes its own
OAuth token on its next launch and posse's next read succeeds. Quiet
tolerance is for precisely that. And a row raised while the shop is still
dispatching would make G5's URGENT ("the shop is stopped") a lie for the
first ten minutes. So the class is named in the log from the first 401, and
the human is raised when the fleet actually parks — which is what
`plan_guard_blind_max:` means. If minutes still feel long, the knob is the
lever, and `ranger-base-4i4e` (revert the 24h stopgap) is where that lives.

Two things this deliberately did NOT do: the cockpit header's class
(`planRead` carries flags, not an error) and the `unreadable`/keychain class
are `rangerhq-pwpx`'s, which is blocked on `rangerhq-q65q`; and
`modelavail.go`'s own 401 is out of scope there too.
