## The account ledger learns what the overflow ledger learned

*(ranger-base-ws09)*

`uncountedSkip` asked `uncounted.log` one question — *how many beads went to
this runtime in the last seven days* — and spent the cap on the answer. That
is the same half-precondition ranger-base-2y96 took off `overflow.log` three
days earlier, and its own comment said so: it claimed to keep "the rule the
overflow ledger and Dial E both already keep" at a moment when that rule had
grown a second half and this kept the first.

The shape, with `uncounted_cap_codex: 1` and the ledger mode `0444`: the pass
reads an empty file as `0/1 in 7d`, prints `! account-degraded codex: sent 1
bead(s) this pass, 1/1 in 7d (uncounted_cap_codex:) ... the cap is the
brake`, launches, and only then warns on stderr `uncounted ledger not written
for a-1 (permission denied) — the 7d count will be short by one`. The file is
still empty, so the next pass reads `0/1` again. Two sequential passes over
one `StateDir` launched two beads under cap 1; a hundred launch a hundred.
The cap was never armed, and the only line saying so was printed after the
spending, on a channel posse has no dollar meter for at all.

The fix is 2y96's, taken where this file takes its reading. `uncountedFor`
calls `App.UncountedAppendable()` — `ledgerAppendable` over
`UncountedLogPath()`, the generic half already existed — and `uncountedSkip`
refuses on it beside the unreadable case, with its own skip line and its own
refill kind (`uncounted_cap_<rt>: ledger unwritable`), because inside a
refill only the kind survives and "unreadable" sends an operator to a
different fix than a mode bit.

Three things decided on the way:

- **Only with a cap set.** The count is read every pass because the report
  prints it; the appendability is not, because the brake is the only thing
  that acts on it and the probe touches `StateDir` (it creates and removes a
  temp file when there is no ledger yet). An unset cap is unlimited by
  design, and a pass with nothing to gate on the probe has no business
  running it. ADR 0010 §3 scopes `readOverflowCount`'s the same way.
- **Count first, probe second** — 2y96's order, for its reason: a ledger that
  is both corrupt and unwritable is named by the fault an operator has to fix
  first, and `TestUncountedCapUnreadableLedgerSkips`, whose hostile ledger is
  a *directory*, fails both checks and must keep reporting `unreadable`.
- **`overflowUnlogged` has a twin here now.** It was the sharper half of the
  bead: an append that fails for a reason no open can see — a full disk — was
  warned about once on stderr and then forgotten. `noteUncounted` now counts
  it (`p.Unlogged`) and sets `p.Unappendable`, so the rest of the pass parks
  on that runtime instead of spending a cap against a file that has just
  proved it cannot record the spending, and the pass's account line carries
  the shortfall into the operator's reading: `1/5 in 7d
  (uncounted_cap_codex:), 1 of them never reached ~/…/uncounted.log so the 7d
  count is short by that many from now on`. `p.Used` already carried those
  launches in memory, so the cap still bit inside the pass; what nothing
  carried was the difference the NEXT pass reads.

Pins: `TestQAProbeUncountedUnwritableLedger` (QA's repro, un-skipped
unchanged; its control with a `0644` ledger was passing all along, which is
what makes the 2-under-cap-1 the writability and not the rig),
`TestUncountedCapUnwritableLedgerSkips`,
`TestUncountedUnwritableSkipKindIsItsOwn`,
`TestUncountedUnsetCapLaunchesAndNamesTheShortfall` and
`TestUncountedFailedAppendArmsTheBrake`. Every hostile arm appends for real
first and `t.Skip`s if the process can write a `0444` file, so root cannot
turn a zero-launch pass into a false pass. Mutated: the probe removed (the
QA repro and both skip tests red, 2 unrecorded launches under cap 1 — the
bead's own number), the `noteUncounted` arming removed, and the report's
shortfall clause removed (the last two tests red on each).
