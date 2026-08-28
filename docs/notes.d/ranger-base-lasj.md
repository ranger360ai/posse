## A corrupt ledger line is an unknown week, not a zero (ranger-base-lasj)

`countLedger` skipped any line it could not parse, with the reasoning that
"the file is ours to write and a corrupt line is not a launch anyone can
date". True about the date; wrong about the launch. Skipping reads the line
as *no launch happened*, which is the one fact a torn or hand-edited record
cannot give you — so with `plan_guard_overflow_cap: 1` and an
`overflow.log` holding a single grok line whose timestamp lost its seconds
(`2026-08-26T12:00 grok prior-1 ranger`), dispatch printed `0/1 in 7d` and
spent the cap a second time. Unknown history had become permission to spend.

Now a line that is not a ledger entry — short of four fields, or a `fields[0]`
that is not RFC3339 — returns an error naming the line number, and the count
is unknown. Both callers already had the honest answer wired for the
`os.Open` failure and now reach it for this one too: `overThreshold` turns
overflow off for the pass (`overflow ledger ~/… unreadable (line 1 is not
dated: …) — overflow off this pass`) and on-meter beads park on the ordinary
tripped-guard line; `uncountedSkip` refuses the launch, because an armed cap
over a ledger nobody can count is the unarmed case wearing the armed case's
clothes.

Two edges decided on the way:

- **A corrupt line for another pool poisons this pool's count too.** The
  corruption may have eaten the runtime field; a file that cannot be parsed
  cannot be partitioned by a field read out of it.
- **Whole-blank lines are still skipped, and only those.** `appendLedger`
  writes one newline-terminated line per `WriteString`, so a torn write
  leaves a prefix with the newline missing — never an empty line. An empty
  line carries no record to lose, and treating it as corruption would strand
  a ledger on a stray keystroke with no way back except deleting the evidence.

Pins: `TestQAOverflowCorruptTargetLedgerLineFailsClosed` (the repro, plus the
stderr witness that zero launches came from the fail-closed path and not from
a fixture that never reached the guard) and
`TestLedgerCorruptLineIsUnknownNotZero`, which reads the shape contract
through **both** ledgers — the fix is in the function they share, so a pin on
the overflow caller alone would leave `uncounted_cap_<runtime>:` uncovered.
Mutated the two `return`s back to `continue`: both go red, the well-formed
and blank-line rows stay green.
