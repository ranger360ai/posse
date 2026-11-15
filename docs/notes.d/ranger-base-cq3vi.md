## Check 4 reads source, and the word "Residual" is not a description (ranger-base-cq3vi)

ADR 0024 D2 check 4 — the commit-time wall for "an open `vuln-private`
residual is named by bead id and nothing else" — diffed the staged markdown
alone. A residual described in a `.go` comment, or in a string the binary
prints, never reached the judge; that is how ranger-base-a8qfg's sites
cleared the commit gate twice and were caught only at the push gate
(finding F2 of ranger-base-g2a6d).

The content arm now diffs every staged path except `.beads/*.jsonl`
(`vulnPrivateContentPathspec`, gates.go). The jsonl stays out because it is
the bead's own record and bd flushes it into every commit, so refusing it
removes nothing.

Widening alone would have broken this tree's own convention for naming a
residual in source. `// Residual: <id>.` was refused, because the bareness
test refused any letter, and the label is letters (MEASURED on the bead: five of
eight sites red on 2026-11-14). `vulnPrivateBare` now admits whole words
from a closed list, `residual` and `residuals`, and nothing else:
`// Residual: <id>.` passes, while `// Residual mask: <id>`, `residualmask`
and a printed `(NO MOUNT; <id>)` are still refused.

Still true: the arm reads ADDED lines only, so lines already on `main` red
only when someone edits them. A Go comment is prose to the unit reader, so
a description on the comment line BELOW a bare id is a separate unit and
passes. That is the "prose run" residual vulnprivate.go already states for
markdown, and it applies to source as well.
