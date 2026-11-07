# The wait-leg error with no sink, and the discriminator that did not exist (ranger-base-aj79z)

2026-11-07. Filed out of ranger-base-6qede's verify of four closes — the one
live finding of the four; escaped from ranger-base-74tkp.

## What failed

`gather` (internal/posse/dispatch.go) drops out of its wait loop at
`if r.err == nil`, so **every** line of the loop below that point runs with a
non-nil wait-leg error. Two guards consume one, and both carry
`&& !p.delivered`:

| guard | what it does | reachable on `delivered` |
|---|---|---|
| `IsHerdrCode(r.err, "agent_prompt_stalled") && !p.delivered` | second reading, then keep or re-wait | no |
| `!IsHerdrCode(r.err, "timeout") && !p.delivered` | `logPromptHandBack` + unclaim | no |

`delivered` is ADR 0013 §2 — the prompt rode the launch line, so a wait
failure says nothing about whether it landed and the claim is never handed
back on one. That is correct and is not in question. The consequence was that
on `prompt: argv` (codex, which is the only runtime ranger-base-74tkp is
about) the leg error had **zero** sinks: no `printf`, no `logReading`, no
`logPromptHandBack`, in any arm of the status switch that follows. A seat whose
every leg failed for a whole turn printed exactly what a healthy one printed.

It was survivable while the progress line printed once per LEG — 295 lines in
~3m30s is what got ranger-base-74tkp filed at all. `DefaultProgressEvery` is a
minute now, 84x quieter, and the pacing is correct and pinned: this is the
diagnostic that was traded for it, not a defect in the trade.

## And a committed claim rested on it

docs/notes.d/ranger-base-74tkp.md's "Not done" section left the live Rex cause
unidentified (two authorized codex turns spent, ranger-base-x7ojx) and said
the route back was open: "the discriminator is in the pass output: a leg that
ran out names its own deadline, and a leg that could not read names the rex
call."

**MEASURED FALSE 2026-11-07**, darwin 25.6.0, over ranger-base-74tkp's own
fixture. Neither half could print in the regime that produces the busy-wait:

- the loop goes round again only when `statusAfterTimeout` answers `working`,
  which means the status read SUCCEEDED, so the deadline half never reached
  any output at all;
- the "names the rex call" half reached the output only through the switch's
  `default` arm (`statusPhrase`), which fires when `statusAfterTimeout` also
  fails — and which returns. A pass that printed a rex call had run one leg,
  not 295.

That section is corrected in place.

## The fix

One paced line, printed **before** the status probe:

```go
if d.sayLegError(p) {
    d.printf("◷ %-14s wait leg ended: %v\n", p.is.ID, r.err)
}
```

Three decisions worth the words:

- **Before the probe, not inside an arm of it.** The leg error is a fact about
  the leg, not about what the probe then says. The busy-wait regime is the
  `working` arm, but `blocked`, the `WaitCeiling` arm and the `default` arm
  dropped it too — `statusPhrase` names the PROBE's error and never this one.
  One printf covers all four.
- **Its own clock (`p.saidLegErr`), not `saidWorking`.** Both lines print in
  the same iteration, so a shared timestamp means whichever printed first
  suppresses the other for a whole `ProgressEvery` — and this one prints
  first, so sharing would cost the progress line in exactly the regime where
  the agent IS working. `TestQAWaitLegErrorDoesNotSuppressTheProgressLine`
  pins it; under the sharing mutant `waitLegGather`'s own guard fails first.
- **`%v`, not a code.** `HerdrAPIError.Error()` renders `herdr: <message>
  (<code>)` and both occupants return it (`RexMux.Wait` builds one by hand for
  its own blown deadline), while a rex call that failed is a raw error naming
  the call. Those two ARE the discriminator; a code alone would collapse them,
  since both run-out paths are code `timeout`.

A timeout is NOT filtered out, deliberately. "A `--wait` that ran out of time
is not a failed prompt" is true of the verdict and says nothing about
legibility: the run-out message is one half of the promised discriminator —
on rex it carries the `until` set, the pane and the deadline — so suppressing
it would have left the corrected note as false as the original.

## MEASURED output

2026-11-07, darwin 25.6.0, `waitLegGather` with `LegFloor` 400ms and
`ProgressEvery` 0 (one line per leg), identical five-line shape on both
occupants:

```
◷ a-1   wait leg ended: herdr: timed out waiting for agent status (timeout)
◷ a-1   still working after 0s — waiting again
...
◷ a-1   still working in ranger-qa-a-1 after 2s — claim kept, not judged this pass
```

```
◷ a-1   wait leg ended: herdr: no idle|done|blocked state in block:fake-0001 within 1ms (last reading: working) (timeout)
◷ a-1   still working after 0s — waiting again
...
```

Before this bead both of those passes printed the progress lines and nothing
else — the two probes the finding named (`no idle|done|blocked state in`,
`timed out waiting for agent status`) were false on both occupants.

The `herdr:` prefix on the rex line is the envelope's, not a claim about the
occupant: `RexMux` reuses `HerdrAPIError` on purpose (muxrex.go — it carries
that route's codes verbatim rather than coining its own), and the pane in the
message is what says who spoke. Noted rather than changed: `Error()` is shared
by every herdr call site in the tree.

## Pins

internal/posse/waitlegpace_qa_test.go, arm 1, in the shape the pacing pins
already use — one bounded `gather` per occupant, driven directly, asserting
the same thing on both:

| pin | holds |
|---|---|
| `TestQAWaitLegErrorReachesThePassOutputOnEveryMultiplexer` | the error text and its code reach the output, each occupant on its own half of the discriminator |
| `TestQAWaitLegErrorLineHasItsOwnIntervalOnEveryMultiplexer` | and it is paced by `ProgressEvery`, not per leg — at least one, at most `ceiling/every + 2` |
| `TestQAWaitLegErrorDoesNotSuppressTheProgressLine` | the two clocks are separate: exactly one of each inside one `ProgressEvery` |

MEASURED 2026-11-07, darwin 25.6.0, each mutant reddening its own pin on BOTH
occupants:

| mutant | herdr | rex | pin that reddens |
|---|---|---|---|
| no sink at all (the pre-fix state) | 0 lines | 0 lines | all three; the "reaches the output" pin names each missing probe |
| `sayLegError` reads `saidWorking` | progress line gone | progress line gone | `waitLegGather`'s own guard, before any assertion |
| `sayLegError` always true | 75 lines / 2s | 33 lines / 2s | the interval pin (allowed 7) |

With the fix: 1 and 1 inside one `ProgressEvery`, and 5 and 5 at
`ProgressEvery` 0 over a 400ms floor.

## Not in scope

Identifying the live Rex cause, which still costs a billed codex turn. This
bead is what makes the next occurrence legible enough to diagnose without one.
