# Three stated decisions that no pin held (ranger-base-obaim)

MEASURED 2026-11-07, darwin 25.6.0, go1.26.5, at HEAD 88b17e77 (== main).
Filed by ranger-base-2oahh's verify pass over four closes; this fragment is
the fix. None of the three was a defect in today's behaviour — the shipped
code is right about all three. What was missing was the pin, so each survived
a mutant that reversed the decision with every existing pin green.

The class is one class: **the close states a placement, an order or a window
as the design decision IN PROSE, and the prose is the only thing holding it.**
The next refactor lands green and reintroduces exactly what the bead fixed.

No production code changed. Three test files did.

## 1 · the leg error's PLACEMENT (escaped from ranger-base-aj79z)

`dispatch.go:4010` prints the wait leg's own error BEFORE the status probe,
and the code says why in as many words: "the busy-wait regime is the `working`
arm, but `blocked`, the ceiling and the `default` arm each dropped it too".

Why it escaped: all three of ranger-base-aj79z's pins are driven by
`waitLegGather`, whose seat reads WORKING throughout — it hard-requires that
("this gather never reached the re-wait arm"). So all three are satisfied by a
sink that only fires inside `case "working":`.

MEASURED with the sink moved into `case "working":` (immediately before
`if d.sayWorking(p)`): `blocked`, the `default` arm and the WaitCeiling arm
each print no leg error, and `TestQAWaitLegError*` stays green, because earlier
`working` legs printed one.

Pinned by `waitLegGatherEndingOn` + `legErrorBefore` in
`internal/posse/waitlegpace_qa_test.go`: one bounded gather per occupant over a
seat the probe reads as a state gather does not loop on, so the pass ends on its
first leg. Two arms, `blocked` and an unnamed state, each asserting the leg
error is present, carries the OCCUPANT's own text, and is ABOVE the arm's own
line. The mutant reddens both arms on both occupants (4 of 4) and nothing else.

Two fixture notes, both honest asymmetries rather than contrivances:

- On herdr the fake's `wait-error` lever is independent of what the seat reads,
  so the seat reads the probe state from the start.
- On rex the status QUEUE steps: head `working` (not in rewait's until set, so
  RexMux's poller is past its own 1ms deadline when the first sample returns and
  answers `timeout`), then the probe state, which `fakeRexProgramStatus` leaves
  in place forever. That is the production sequence — the leg ran out while the
  agent was working and the seat had moved by the time the probe asked.
- An unnamed record state reaches posse as `MuxUnknown` on rex and verbatim on
  herdr (muxrex.go maps what it does not know, ADR 0076 §3), so the `default`
  arm's assertion is per occupant.

## 2 · the ORDER of the three Rex readings (escaped from ranger-base-nhxdx)

`dispatch.go:5118` and `herdrback.go:2632` read declaration, then version, then
precondition, both carrying "THE ORDER IS THE DECISION, not an optimisation of
it: a grok the key was not measured on falsifies the measurement the
precondition rests on".

Why it escaped: the order is observable only where BOTH readings refuse, and no
fixture produced that. Every precondition arm fixes the version at
`rex_verified` (`grokRexBackend`, and two `t.Fatal` guards say so), and every
version arm seeds a config carrying the line.

MEASURED with the two blocks swapped, over a grok reporting
`grok 1.0.70 (deadbeefcafe)` whose config has `[ui.notifications]` but no
`program_status`:

| | message |
|---|---|
| as landed | `RexVersionRefusal` — names the release read, the release it was measured on, and `re-measure the key and move rex_verified:` |
| inverted | `RexPreconditionRefusal` — "ADD THE LINE ... program_status = \"on\"", and it **never names the version at all** |

So inverted, the operator is sent to edit their config on a release nobody
measured that key on — the admission D13's order exists to refuse — and is told
nothing about the version. The refusals do not compose: each returns, so exactly
one is ever seen.

Pinned by `grokRexBothRefusing` +
`wantGrokVersionRefusalAndNotThePrecondition` in
`internal/posse/grokrex_qa_test.go`, one arm per call site, plus planLaunch's
interactive DEGRADED arm — which carries the same order and is the arm an
operator at the keyboard actually reads. The positive half is the release
(decisive on its own: the precondition's refusal names no version), the negative
half is `ADD THE LINE` (what the operator would act on). Each mutant reddens its
own call site's arm(s) and nothing else.

## 3 · the sparing window is per PASS (escaped from ranger-base-6mpke)

`prunePasses` takes `at := len(warn.String())` at the top of each pass and asks
`!strings.Contains(warn.String()[at:], lockSparedSentence)`. That `[at:]` is the
whole of "a spared pass is not a pass", and widening it to the whole buffer is
the obvious simplification.

Why it escaped: all three existing arms stay green on the widened read. "held
throughout" wants `NOT ONE` and still gets it; "held past any pass count" ends
on cond; the deadline arm holds no lock, so its buffer never carries the
sentence at all.

MEASURED over the shape the bead is about — ONE transient false-busy (the lock
taken, a pass run under it, released: a sibling's fork-to-exec window), then a
free lock and a cond that can never hold:

| | passes | wall | give-up |
|---|---|---|---|
| as landed | 20 | 122ms | "20 of which reached reclaim's own decision" → blames the PRUNE |
| widened | 2,197 | 15.01s | "NOT ONE of which reached ... a box on which that lock is never free" → blames the BOX |

After one spared pass the sentence is in the buffer forever, so `reached` can
never increment again: the reach guard is dead, every call can only end on cond
or the 15s clock, and a genuinely broken prune is reported as a broken box. That
is ranger-base-6mpke's own stated disease — "a message aiming its reader at the
box when its evidence was about the prune" — at 123x the wall.

Pinned by a fourth arm of
`TestQAAThePruneRetryBudgetRidesOutAHeldLaunchLock` ("one spared pass does not
blind the reach guard") in `internal/posse/reclaimdestroy_qa_test.go`, plus a
comment on `at` naming the window as load-bearing — which is what stops the
simplification being made at all. The hold is 50ms against the 5ms gap, so the
first pass is certainly spared; it does NOT need to outlast
`pruneUntilReaches` passes the way the ride-out arm's 300ms does, because the
subject is the first spared pass poisoning every pass after it.

Note the mutant the bead described — replacing `warn.String()[at:]` with
`warn.String()` — does not build (`declared and not used: at`). The honest
mutant deletes the `at :=` line too, and that is the one measured above.

## What bears on what else to look for

ranger-base-2oahh found all three by mutating each stated decision SEPARATELY
rather than together. ranger-base-nhxdx's close mutated both wire sites at once;
mutated separately they are each pinned — so that close's coverage was BETTER
than its own table showed — and the gap was one rung up, in the ORDER of the
readings rather than their presence.
