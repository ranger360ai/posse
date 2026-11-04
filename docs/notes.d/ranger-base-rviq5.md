# The pass that reaps the session the test is about to read (ranger-base-rviq5)

`ranger-base-191em` found one test of a class and filed the rest. The class:
a test fixture marks its bead **closed**, runs a full `d.Run`, and then reads
the session that pass just launched — which the pass's own epilogue sweep may
legitimately have reaped by then.

Nothing in the product is wrong. A per-bead Dial F session whose bead the
store calls closed and whose agent herdr calls idle is exactly what
`autoReapPass` exists to kill (`autoreap.go`, `reapClassOf`), and the sweep
runs as the pass's epilogue. The only thing holding it off the session the
same pass prompted is `promptedRecently`, and `PromptGrace` unset is **30
seconds** against a pass that budgets itself `DefaultGatherWindow` — three
minutes, with a `DefaultPromptWaitMS` leg of fifteen inside it, neither
shortened by `newTestDispatcher`.

MEASURED 2026-11-03 (ranger-base-191em), with the fake herdr's
`prompt-delay-ms` lever holding the gather's `pane prompt --wait` leg at 35s:
the sweep prints `reaped ranger-003-a-1 (bead a-1 closed)` and REMOVES the
record one line before `TestDispatchRelaunchesDeadAgent` reads it. The
discarded `ok` from `m, _ := b.readMeta(session)` then dereferenced the nil as
**SIGSEGV** — and a panic in a parallel package takes the whole arm-2 binary
down, so every other test in the arm reads as failed with no test named.

## The census (MEASURED 2026-11-04, at main 1ac75f2f)

26 test files under `internal/posse` both call `d.Run(` and read a meta,
which is where the bead's "not every one of those is exposed" came from. Per
TEST FUNCTION, the exposure needs all three of a full pass, a CLOSED bead,
and a post-pass read of the session that pass launched. **22 functions run a
pass and then read a meta; 10 of those have a closed bead; 9 are exposed.**

| test | file | what the reap takes from it |
|---|---|---|
| `TestDispatchRelaunchesDeadAgent` | dispatch_qa_test.go | the record, then a nil deref — the incident |
| `TestDispatchTierReachesSession` | dispatch_qa_test.go | the record it reads for `m.Tier` |
| `TestDispatchRefusesDegradedUnlessAllowed` | parity_test.go | both records it reads for `m.Degraded` |
| `TestDispatchRecordsTheBeadOnTheSessionItLaunches` | reapguard_qa_test.go | the record whose `bead:` is the subject |
| `TestDispatchGivesEachPersonaItsOwnTree` | worktree_qa_test.go | both records, and both trees under them |
| `TestDispatchedTreesShareOneBeadsDatabase` | worktree_qa_test.go | the record, and the tree whose redirect is read |
| `TestAutoReapSkipsASessionJustPrompted` | autoreap_test.go | the record its own first assertion says survives |
| `TestAutoReapRetiresAWorktreeSessionsTreeAndBranch` | autoreap_qa_test.go | the record pass 1 must not reap |
| `TestAutoReapCommitsThePersonaMemoryAndSpendsNoTurn` | memoryland_qa_test.go | the session pass 2's reap line is read from |

The last three are the ones worth reading twice: they are tests OF the reap,
and the fact each asserts first — "not on the pass that prompted it" — is
true only inside `PromptGrace`. Under a loaded box they go red over correct
behaviour, naming the guard they exist to hold.

**Not exposed, and why**, since the near misses are the instructive half:

- `TestAPausedPassStillRunsTheEpilogue`, `TestAutoReapKillsAClosedIdleSession`
  and every other `reapCandidate`/`reapSession` fixture — the candidate was
  never prompted, and `promptedRecently` reads a zero `prompted:` stamp as
  "not recent" whatever the grace is (`dispatch.go`). These WANT the reap and
  still get it.
- the twelve functions with an open bead — `reapWhy` reads the bead fresh and
  returns false on anything but `closed`.
- `TestAPausedPassStillLandsAClosedBeadsTree` and the `pause_test.go` pair —
  they assert the reap HAPPENED.

Two more are exposed through a read that is not a `readMeta`, which is why
the fix is the dispatcher's default and not a line per site:
`TestAutoReapNamesAWorktreeItCouldNotRetire` writes into `tr.Path` after pass
1, and `TestClosedBeadLandsOnTheRepoBranch` stats that tree — a reap retires
it.

### The discarded `ok`, separately

A `readMeta` whose second result is thrown away, inside a function that runs
a dispatch pass: **7 sites in 6 functions in 3 files** (dispatch_qa_test.go
×4, parity_test.go ×2, turnoutcome_qa_test.go ×1). Four of the seven
dereference the result on the next line with no nil guard, which is the
segfault; the other three carry an `m == nil ||` that turns the crash into a
confusing `%+v` of `<nil>`. The bead's open/closed split does not matter
here — a record that is gone and a record carrying the wrong field are
different facts, and a test should say which it found.

## The fix

**`newTestDispatcher` carries `PromptGrace = time.Hour`** (herdr_test.go).
`ranger-base-191em` set it in one test and deliberately left the default
alone for scope; the default is where it belongs, because the exposure needs
only a closed bead and a post-pass read — the ordinary shape of a dispatch
test — so a test written tomorrow inherits the safety without knowing any of
this. The grace is what moves, not the pass: shortening `PromptWaitMS` or
`GatherWindow` would be a new timeout that can expire, the same class of
flake one knob over.

It is safe for the tests that want the sweep to fire, and by construction
rather than by audit:

- a never-prompted candidate is not guarded at any grace (above);
- a test that wants a LATER pass to reap says so with
  `agePrompt(t, b, session, d.PromptGrace+time.Minute)`, which reads the
  grace off the dispatcher and moves with it. All five call sites are written
  that way already (`autoreap_test.go`, `autoreap_qa_test.go` ×2,
  `memoryland_qa_test.go`, `reapworktree_qa_test.go`).

`LaunchBead`'s own use of the grace is unchanged in practice: with 30s, every
in-test second prompt was already inside it, so widening the window changes
no test's answer — and the tests that need it off set it explicitly, 22
assignments across 14 files (21 of `0`, one of `time.Nanosecond`).

`ranger-base-191em`'s own `d.PromptGrace = time.Hour` inside
`TestDispatchRelaunchesDeadAgent` is left in place rather than folded into
the default. It is redundant now, and taking it out would be the merge
conflict this bead went out of its way not to create — and it carries that
test's own reasoning, which is the clearest statement of the class anywhere
in the package.

**The seven discarded `ok`s are read**, each with a `t.Fatalf` that names a
missing record as its own fact. `dispatch_qa_test.go`'s site at
`TestDispatchRelaunchesDeadAgent` is taken VERBATIM from
`ranger-base-191em`'s unlanded branch (`5801c132`) rather than re-fixed:
that file is byte-identical between its base and this one, so both branches
now hold identical content there and the merge-back has nothing to conflict
over.

## The pins

`internal/posse/passreaprace_qa_test.go`, arm 1, door `make pkgcensus-check`:

- `TestQATestDispatcherOutlastsItsOwnPass` — behavioural, because the fact is
  a RELATION between two numbers: `newTestDispatcher`'s grace against
  `DefaultGatherWindow` plus one `DefaultPromptWaitMS` leg (18m). A grep for
  `time.Hour` would hold neither number and would stay green on an hour
  against a gather window somebody later raised to two. The inverse arm
  refuses a zero grace, so the pin cannot be satisfied by disabling the
  guard it is about.
- `TestQANoPassRunningTestDiscardsAReadMetaOk` — the census, over the package's
  test files as TEXT, so one pin in arm 1 covers all three arms. "Runs a
  dispatch pass" is derived from the call SHAPE and not from a list of local
  names: `Dispatcher.Run(dir, persona, max)` takes three arguments where this
  package's other `.Run(` take none (`exec.Cmd.Run`, `testing.M.Run`), so a
  receiver other than `t` with a non-empty argument list is the whole
  discriminator. MEASURED 2026-11-04: 370 dispatch calls on eleven receiver
  names (`d` 321, `d2` 38, `d3` 3, and one each of `again`, `cd`, `dOther`,
  `dry`, `first`, `quiet`, `restart`, `second`) against 27 zero-argument calls
  on `cmd`, `c`, `done` and `m`.

Both carry fixture controls over text they own, and the census carries a
population floor (572 test files, 5,326 top-level funcs, 319 pass-running),
because a census whose reading has broken passes for the wrong reason. Both
were verified red: the grace pin names the 30s and the 18m, and the census
names file, line, test and the offending source line.

## Verified

`make fmt-check`, `make tree-check`, both new pins green and both red against
a reintroduced regression, and `make test` whole — arm 1, arm 2, arm 3.
