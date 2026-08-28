## The gate says WHERE to run it (ranger-base-kn99)

The operator's own terminal is a persona pane. The `!` prefix runs in the
*current* session, whose PATH leads with `state/gates/<persona>/bin`, so during
the ranger-base-okbr outage the crew's keychain tripwire refused **his**
credential read:

    refused by posse gate: security find-generic-password -s Claude Code-credentials -w
    (deny: Bash(security:*))

and stopped there. The rule was named; the way out was not. It cost a round
trip in the middle of a stopped shop. A new class: ranger-base-r64 and
ranger-base-ypf5 are *posse's own* tooling refused by the persona shim — this
is the human whose credentials they are.

**No escape was added.** "Open a terminal outside posse" is a complete
workaround and it preserves the tripwire exactly (ranger-base-khu); the
absolute-path route stays posse's own (ypf5), and a persona reaching for it to
answer a question its PID denies is routing round its own cage. What changed is
`RHQ_GATE_HINT` — a slot the shim already rendered and left empty — which now
carries, under the rule:

      this shell is dinesh's pane: posse's gate dir leads its PATH, so
      every shell in it is gated and a persona has no way past that.
      operator: run security in a terminal outside posse.

Naming the pane is the whole trick: it says why a shell the operator thinks of
as his own refused him, and it tells the *persona* reading the same stderr that
there is nothing here for it.

**A table, not a blanket** (`whereHints`, gates.go) — the judgment the bead left
open. The line is only honest where the refused reader might BE the operator.
`security` is a tripwire on the crew: nothing in a pane should read the
keychain, and the operator reads it constantly. `Bash(git push:*)` is the
opposite — a control on an action that is the launcher's by design, where
"run it outside posse" reads as an escape. One line per entry, deliberately:
each entry is a judgment someone made.

`ruleHint` now composes rather than choosing: a negative rule's `safe form:`
line first (the answer for whoever typed it), the where lines after (the answer
only if that was the operator). The multi-line hint stays one shell assignment —
the newlines sit inside the single quotes `shQuote` puts round it.

Pinned in `internal/rhq/gates_test.go`:
`TestRefusalNamesWhereTheCommandDoesRun` drives the REAL rendered shims with the
operator's measured argv, and carries the negative control — `git push` and an
unqualified `git commit` are refused (the witness: without it an absence
measures nothing) and must NOT be pointed outside posse, while `commit` keeps
its safe form. Stubs stand in for both real binaries, so an escape leaks to a
file instead of pushing. `TestRuleHintComposesSafeFormAndWhere` pins the
composition and its ordering. Four mutations, each verified red: the table
emptied, the table made unconditional (only the negative control catches it),
the where line dropped from `ruleHint`, and `persona` not wired through the
call site (a constant name renders, and only the shim-level pin catches it).
