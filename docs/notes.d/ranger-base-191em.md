# The flake's one-minute budget was spent on a three-minute pass (ranger-base-191em)

`TestDispatchRelaunchesDeadAgent` went red once in arm 2 of a `make test` run
from another worktree while sibling seats held both suite slots, on
2026-11-03, at `dispatch_qa_test.go:388`:

    relaunch must record the new launch time

and then passed 8 of 8 in isolation. The assertion was

    m, _ := b.readMeta(session)
    if time.Since(m.Launched) > time.Minute { ... }

The stamp was never the problem. `RelaunchAgent` writes `m.Launched =
time.Now()` and checks the write, and every later writer of that record
(`NoteBead`, `MarkPrompted`) re-reads it first, so nothing in a pass can lose
or stale the stamp. What `time.Since(...)` measures is not the stamp but
**everything downstream of it**: the rest of `RelaunchAgent`, `awaitAgent`,
the claim, the typed prompt, and the pass's own gather.

## The two budgets that bound that stretch, and are not shortened

`newTestDispatcher` shortens `StartupWait`, `StatusGrace` and `Poll`. It
shortens neither of the two that bound everything after the launch stamp:

| knob | value in this test | why it is that |
|---|---|---|
| `GatherWindow` | 3m (`DefaultGatherWindow`) | unset, so the default |
| `PromptWaitMS` | 15m (`DefaultPromptWaitMS`) | `NewDispatcher`'s default |

So the pass gives itself three minutes for the stretch the assertion gave
sixty seconds. MEASURED on a quiet box, 2026-11-03: **227–427 ms** over
eleven runs (3 solo, 8 at `-count=8 -parallel 8`). A 140x margin against a
one-minute budget is why the number held for as long as it did, and why the
one time it did not, it named the relaunch.

## Reproduced, with the fake's own load lever

`prompt-delay-ms` in the fake herdr dir is "sleep that long, then settle" —
the fake's own words for a loaded box — and it holds the `pane prompt --wait`
leg the gather is waiting on. Both readings MEASURED 2026-11-03:

- **61000** — the old assertion fails at a 62.0s run, with the stamp correct
  in the record. That is the reported flake, reproduced.
- **35000** — the pass's epilogue sweep prints `reaped ranger-003-a-1 (bead
  a-1 closed)` and REMOVES the record. The discarded `ok` then dereferenced
  the nil one line later: `SIGSEGV`, at `dispatch_qa_test.go:388`. A panic in
  a parallel package takes the **whole arm-2 binary** down, not one test.

The reap is correct Dial F behaviour and nobody's bug. The fixture's bead is
closed (that is what `closed by ranger` asserts) and the session settles, so
the only thing keeping the sweep off it is `promptedRecently` — and
`PromptGrace` is 30s unset, well inside the pass's own 180. Past it the
sweep is right and the test is reading a record the product has correctly
deleted.

## The fix

- `d.PromptGrace = time.Hour` in the test, so the pass cannot reap the
  session it relaunched however long the box makes that pass take. The grace
  is moved rather than the pass made to hurry: a shortened `PromptWaitMS` or
  `GatherWindow` is a new timeout that can expire, which is the same class of
  flake one knob over.
- `m, ok := b.readMeta(session)`, with `!ok` a `t.Fatalf` that says the
  record is gone. A missing record and a stale stamp are different facts that
  reached one line; one of them used to be a segfault.
- the stamp is read against `before := time.Now()`, captured before `d.Run`,
  and not against a wall-clock budget. `m.Launched.Before(before)` is the
  asserted fact exactly — the relaunch replaced the fixture's stamp with one
  of its own — and no downstream budget can move it. The error names both
  readings, because a failure is now one of exactly two things: a stamp an
  hour before `before` is the fixture's, never replaced, and the zero time is
  a record carrying no `launched:` line at all (`parseLaunched` reads an
  absent or unparseable one as the zero time, the inversion
  `TestQADeadPersonaSessionRefusesARecordThatDidNotTakeTheStamp` pins).

Verified: PASS at the same 61s hold that reddened the old assertion and
segfaulted it at 35s.

## The class this is one of

The shape is "a test runs a full `d.Run` over a bead the fixture marks
CLOSED, then reads the dispatched session's record afterwards" — and the
record is one the pass's own epilogue may legitimately have reaped by then.
26 test files in `internal/posse` both call `d.Run(` and read a meta, and the
discarded-`ok` read is common in them. Filed as its own bead rather than
fixed here.
