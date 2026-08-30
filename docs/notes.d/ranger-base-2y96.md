## An unwritable ledger is a cap of zero that reads as room

*(ranger-base-2y96)*

`readOverflowCount` asked the overflow ledger one question — *how many beads
went to this pool in the last seven days* — and treated the answer as the
whole precondition for spending. It is only half of it. The other half is
the line the move owes **after** it launches, and nothing checked that the
line could be written until it had already failed.

So a `state/overflow.log` that reads clean and refuses every write let
dispatch spend forever. With `plan_guard_overflow: grok`,
`plan_guard_overflow_cap: 1` and the ledger mode `0444`: the pass reads an
empty file as `0/1 in 7d`, announces room, launches the bead, then warns
`overflow ledger not written for a-1 (permission denied) — the 7d count will
be short by one` and exits. The file is still empty. The next pass reads
`0/1` again. Two sequential passes launched two beads under cap 1 and
recorded neither; a hundred passes launch a hundred. The cap was not
loosened, it was never armed — and the only line saying so is a warning
printed *after* the money was spent, on a pool posse has no meter for.

The fix is the precondition, taken where the reading is taken.
`readOverflowCount` now also calls `App.OverflowAppendable()`, and an
unwritable ledger fails closed exactly as an unreadable one does: `overflow
off this pass`, on-meter beads park on the ordinary tripped-guard line,
off-meter beads still launch (ADR 0013 §3). It heals itself the moment the
mode is fixed, which is the property the pre-overflow behaviour always had.

Three things decided on the way:

- **The probe is an open, never the mode bits.** `0444` is a promise about a
  uid: root keeps its write, and an ACL can take one away from a mode that
  looks fine (ranger-base-c00). `ledgerAppendable` opens the file the way
  `appendLedger` will and closes it, having written nothing.
- **No `O_CREATE`.** The obvious probe — the exact open `appendLedger` does
  — leaves an empty `overflow.log` behind on every tripped pass, including a
  `--dry-run` one, and `TestOverflowDryRun` plus the grok-pool guard both
  pin that a bead which never launched writes no ledger line. When there is
  no ledger yet, what has to be writable is the directory the first append
  will create it in, so the probe creates and removes a temp file in
  `StateDir` instead. Mutating the open back to `O_CREATE` reds both.
- **Count first, probe second.** A ledger that is both corrupt and
  unwritable is named by the fault an operator has to fix first, and it
  keeps `TestQAOverflowUnreadableLedgerDisablesThePass` — whose hostile
  ledger is a *directory*, so it fails both checks — reporting `unreadable`.

What this does **not** retire is `overflowUnlogged` (ranger-base-af98). An
open that succeeds is not a write that will: a full disk fails at
`WriteString`, which no probe short of writing a line can see. That append
still warns and still carries the difference into the re-read, so the
in-pass arithmetic stays honest; the probe only closes the case where the
failure was knowable before anything was spent.

Pins: `TestQAOverflowRefusesAReadableButUnwritableLedger` (the repro,
un-skipped, now also asserting each pass's stderr names the ledger it cannot
write and its bead parks — zero launches is otherwise indistinguishable from
a fixture that never reached the guard) and `TestLedgerAppendable`, which
takes the four arms directly: absent ledger, writable ledger, `0444` ledger,
absent ledger under a `0555` `StateDir`. The two hostile arms probe with a
real open first and `t.Skip` if the process is root, so the repro cannot
pass by not being realized. Mutated the probe away entirely (2 unrecorded
launches under cap 1, the bead's own number), the directory branch away (the
`0555` arm reds), and `O_CREATE` back in (the absent-ledger and dry-run arms
red).
