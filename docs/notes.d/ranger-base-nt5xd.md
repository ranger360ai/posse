# A row the judge cannot name is classed by the binary that printed it (ranger-base-nt5xd)

ADR 0092 D4 rules 5 and 6, built on top of rule 4's `posse upgrade
--classes` (ranger-base-e6yct). The mechanism is in
`internal/posse/upgradeverify.go` — `upgradeParseClasses`,
`upgradeTargetClassExcuses`, `upgradeClassCrossing`, `upgradeClassNote` and
the gained-row loop of `upgradeVerdict` — and the pins are in
`internal/posse/upgradeverify_test.go`. This page holds the measurement and
the two decisions the ADR does not make for you; the code holds the claim.

## The defect, in one sentence

The judge is the table compiled into the binary RUNNING the verb, which is
the OLD one, so a G-row added inside the range is a row the judge cannot
name — and the unknown-code rule classes it binary-rendered, which rolls
back. MEASURED 2026-11-21 (ranger-base-i9fq8): `0e1cb6c0..65d90a5e` rolled
back on one LANE G18 row, a row added in that range and classed
install-subject by the binary that printed it. The fix for such a crossing
cannot be installed either, because every target that holds it is judged by
the table that predates it. That is what makes it a wall and not one bad
install.

The field's answer to the old-reader/new-writer case is that the writer's
data describes itself, and rule 4 is that description. Rule 5 reads it for
exactly the rows the reader cannot name itself, which is also what keeps the
candidate from voting on its own install: a row whose existence is the
target's own speech alone is a row it could have escaped by not printing.

## MEASURED 2026-11-21, this box (linux 7.2.15-1-omarchy, uid 1001)

Eleven one-change mutants of `upgradeverify.go`, applied one at a time with
the tree asserted clean between arms, each run over the six new pins plus the
two older ones the change touches (`-run` filter, 0.6s clean). **Every mutant
red, each at the pin whose name is the behaviour:**

| mutant | red at |
|---|---|
| `upgradeClassExcuses` returns false (excuses nothing) | the word pin, rule 5's two excusing arms, 17 of 19 crossing rows |
| `upgradeClassExcuses` returns true (excuses everything) | the word pin, rule 5's `binary` arm, the crossing row G17 |
| the `!ClassesAsked` guard dropped, so silence reads as a lookup | rule 5's not-asked arm |
| the crossing line never printed | rule 6's disagreeing arm |
| the crossing line printed when the two tables agree | rule 6's agreeing arm |
| the target excuses a row the judge NAMES | rule 6's G7 arm |
| the `--classes` reading never taken | rule 5's two excusing arms, rule 6's G7 arm |
| the note computed and not printed | rule 5's two excusing arms |
| a row answered twice resolved first-wins | the parser pin's three duplicate arms |
| `asked` read off a well-formed line instead of the mark | the parser pin's two malformed-line arms |
| rule 5 excuses nothing at the call site | rule 5's excusing arms, 17 of 19 crossing rows |

The two crossing rows that survive the first and last mutants are G7 and
G17 — the two the real table classes binary-rendered, which trip either way.
That is the pin reading the table rather than a constant, and it is why the
battery runs both directions of every predicate.

`make fmt-check`, `make crew-check`, `make pkgcensus-check` and
`make verify-parallel` green; the whole `Upgrade|Install|Condition|Gov`
filter green at 8.9s.

## Two decisions the ADR leaves open

**A row answered twice is DROPPED, not resolved by order.** The printer
ranges one map, so a well-formed build cannot produce it; a reader still has
to answer. First-wins is right for `shop binary` and wrong for `binary
shop`, and last-wins is the reverse, so neither order is a rule. A dropped
row reads as one the target did not name, which trips exactly as a silence
does — the safe direction, since the only thing an answer can buy a row here
is an excuse.

**The note goes on the STAND path only**, which is `upgradeRenderNote`'s
placement and `upgradeHookWallLine`'s, for their reason: a note on a run
about to be undone describes a binary that will not be there. It is printed
rather than kept silent because such a row is the one line of the block
whose verdict did not come from the judge, and the delegate posts that block
on the install bead.

## One cost outside the verb

`TestUpgradeEveryRowCrossingIsClassedByTheTargetsAnswer` deletes one entry of
`upgradeConditionSource` at a time, because "a judge whose table predates the
row" is this binary's table minus that entry and there is no other way to be
that binary. That makes it the tree's only writer of the table, so
`cmd/testparallel`'s gate flagged the five PARALLEL tests that range it — the
census, rule 4's two, the value pin, and rule 5's new reader — four of which
were green before this bead and none of which is racy. They are cleared in
`parallelOK` with that pin's serialness as the argument: Go resumes paused
parallel tests only once the sequential pass is done, so a serial writer
never overlaps a parallel reader (the same clearance kind as `calls
sandboxApplyRefusal`). The consequence worth knowing: the ABSENCE of
`t.Parallel` on that one test is now load-bearing for five others, and a
later test that writes the table in parallel breaks the argument without
reddening those lines.

The alternative was a pin over the three CLASS values instead of the
nineteen rows, which needs no write — and it was rejected because a crossing
is a row, the measured case was a row (G18), and a pin whose subtest is named
`G18` ought to be about G18.

## What the pins do not assert

- That the published word for a row agrees with that row's class. That is
  `TestUpgradeClassesIsTheWholeTableAndNothingElse`, one file over, and the
  crossing pin reads the real renderer rather than restating it.
- Anything about the BEFORE reading's partition. It is taken for symmetry and
  never consulted; rule 5 is one-sided on purpose, because the judge is this
  binary and the target may only describe what this binary cannot name.
- That a binary from before rule 4 refuses the flag harmlessly. That is
  MEASURED in the ADR and pinned in `cmd/posse`; the fake here reproduces the
  measured shape (no stdout, a refusal on stderr, exit 1) and nothing more.

## The bootstrap stays open

A box whose installed binary predates this build judges every crossing by the
old rule, because the code that reads the target's answer lives in the JUDGE.
So the first install that crosses a new G-row on such a box still needs one
operator `make install`, exactly as ADR 0092 D4's own paragraph says. Nothing
in this bead changes that, and nothing in it can.
