# ranger-base-q41e8 — the prefix arm 6 charges, and the file `go test` will print

`ranger-base-x1ofd`'s decision, landed. Arm 6 of
`internal/treepins/wallmemberaccount_qa_test.go`
(`TestQAEveryWallMemberAccountFitsInsideTheEvidenceBudget`) bounds
`prefix + preamble + the account's own fixed width` against
`scripts/posse-wall-check`'s 300 characters (ADR 0097 D3). The prefix term read
`<basename>:<line>: ` off the file each message is WRITTEN in — and `go test`
does not print that file for a message rendered inside a `t.Helper()`:
testing's `decorate()` writes the first NON-helper frame, so the line carries
the CALLING member file and its line.

The charge is now the WIDEST basename in the member set for such a message,
derived from the set the way `wmaMemberPattern` derives the set itself, plus a
four-digit line number. That is `x1ofd`'s decision and this entry does not
re-take it: the actual caller cannot be known statically for a chain with 52
call sites, a narrower charge under-reads, and the widest keeps the rule a
FLOOR — a line that fits at its own caller and not at the widest is one caller
away from being cut.

Every number below is MEASURED 2026-11-22 on this box (linux
7.2.15-1-omarchy, go1.27.3, 16 cores) at tip `7dc3313e`.

## IT WAS NEVER ONE MESSAGE, which is the one reading that changes the bead

`ranger-base-x1ofd` decided this for the ONE line its parameter route adds
(`bgSeatAbstainAccount`'s `NOTHING MEASURED`, ADR 0097 D3's exit 3), on the
reading that nothing arm 6 graded was in such a helper and the term was
therefore inert over the tree. `wmaLinePrefix`'s docstring said the same.

That route is not in this tree — it is `posse/gwart-posse-ranger-base-x1ofd`,
held by `ranger-base-0v6a3` — and **eight of the 65 messages arm 6 already
grades are rendered inside a `t.Helper()` without it**, so the term was charged
to the wrong file eight times over before the route that raised the question
existed:

| message | the frame it is in | own prefix | charged |
|---|---|---|---|
| `wallwatchlive_test.go:139` | `launchOn` (method, `t.Helper()`) | 27 | 44 |
| `wallwatchlive_test.go:143` | `launchOn` | 27 | 44 |
| `wallwatchlive_test.go:300` | `wwWaitKilled` (func, `t.Helper()`) | 27 | 44 |
| `wallwatchlive_test.go:303` | `wwWaitKilled` | 27 | 44 |
| `wallwatchlive_test.go:604` | a closure that calls `t.Helper()` | 27 | 44 |
| `bwrapgitrefslive_test.go:841` | `packUnderALiveSeat` (method, `t.Helper()`) | 30 | 44 |
| `wallclaudeprojectsattack_qa_test.go:386` | a closure that calls `t.Helper()` | 41 | 44 |
| `wallclaudeprojectsattack_qa_test.go:389` | the same closure | 41 | 44 |

The widest basename in the member set is
`wallwatchbackstopwiring_linux_test.go` at 37 runes, so the charge is
`37 + 1 + 4 + 2 = 44`, against 27 to 41 for those eight files' own prefixes:
**the under-read was 3 to 17 characters, 14 at the line `x1ofd` priced.**

**It reds nothing.** The widest head arm 6 charges stays 209 of the
267 a message gets: `bwrapgitrefslive_test.go:598` is not helper-rendered and
is unmoved, and the widest of the eight — `attack:389` — moves from 206 to 209.
58 characters of headroom, before and after.

## The reading, and the one distinction that decided it

"Rendered inside a `t.Helper()`" is read off the INNERMOST function containing
the call — a `FuncDecl` or a `FuncLit`, because a closure is its own frame —
and off whether that function's OWN body calls `Helper()` on any receiver. A
nested closure is not its own parent's body: `t.Helper()` marks the function
that CALLS it, so a closure inside a helper is an ordinary frame unless it
marks itself, and a helper's caller is reached by the CALL stack and not by the
lexical one.

That is not a fine point. The sloppier reading — a `Helper()` call anywhere
under the enclosing `FuncDecl`, closures included — reads **nine** of the 65
where this reads eight, and the ninth is `wallwatchlive_test.go:614`, written
in a test body beside a closure that is a helper. `go test` prints that line at
its own file and line, so charging it the widest basename would over-read it by
17.

## The term is witnessed hermetically, and the tree witnesses NOTHING of it

`ranger-base-dxaxt`'s mutant 8 found that the whole prefix term could be
replaced by a constant 0 with arm 6 still green. The helper half is the same
hole one reading on, in both directions, so both the reading and the arithmetic
are held in arm 7
(`TestQATheWallMemberBudgetReadersMatchWhatTheyClaim`) over planted source.

Nine mutants, one at a time, `go vet` plus arms 1, 2, 3, 6 and 7 after each,
reverted before the next
(the battery is a throwaway; the readings are here):

| # | mutation | arm 7 | arm 6 | what the member tree read |
|---|---|---|---|---|
| M0 | unmutated | PASS | PASS | 8 of 65 helper-rendered, charged 44, widest head 209 |
| M1 | `wmaRenderedInHelper` always answers "not a helper" | **FAIL**, all 5 reading rows | PASS | 0 of 65 — the term silently back to 27-41 |
| M2 | it always answers "a helper" | **FAIL**, the 3 negative rows | PASS | 65 of 65, widest head 223, still inside 267 |
| M3 | `wmaFuncSpans` descends into nested closures | **FAIL**, the `:614` row | PASS | 9 of 65 |
| M4 | `wmaFuncSpans` reads `FuncDecl`s only | **FAIL**, both closure rows | PASS | 5 of 65 |
| M5 | `wmaRenderedInHelper` picks the OUTERMOST function | **FAIL**, both closure rows | PASS | 5 of 65 |
| M6 | `wmaWidestMemberBase` compares whole paths | **FAIL**, the basename row | PASS | charged 38 off `internal/posse/a_middling_name_test.go` |
| M7 | `wmaHelperLineDigits = 3` | **FAIL**, the prefix and charge rows | PASS | charged 43 |
| M8 | the helper arm deleted from `wmaChargedPrefix` (this landing reverted) | **FAIL**, the charge row | PASS | 8 counted, 27-41 charged |

Arms 1, 2 and 3 were green under all nine: the reading is arm 6's and touches
no other key. **Arm 6 was green under all nine**, which is the whole argument
for arm 7 carrying this — a tree that fits at both charges cannot tell them
apart, and the charge is 14 characters of a 267-character budget that the next
preamble edit will spend.

The branch itself is a named reader (`wmaChargedPrefix`) and not two lines
inside arm 6 for the same reason: M8 is the mutation a later reader would
make by hand, and inside the arm it would have red nowhere.

## The adjacent figure in the head comment, which was wrong twice over

The arm-6 paragraph said arm 6 reds nothing "at 34 characters of headroom on
the widest literal preamble in the set (233,
`bwrapseatstorelive_test.go:751`)", while arm 6's own `Logf` read 209 at
`bwrapgitrefslive_test.go:598`. Both halves of the prose figure were wrong:

- **233 was measured on a tree that no longer existed.** It is
  `ranger-base-uu303`'s and `ranger-base-dxaxt`'s reading of a row that
  `ranger-base-bj1hh` then shortened out of the account-carrying set
  altogether — at `7dc3313e` `bwrapseatstorelive_test.go:751` carries no
  account at all, and the file's widest row is `:816` at 169. dxaxt landed
  ON TOP of bj1hh and carried a pre-bj1hh figure across it.
- **It named one quantity and was computed from another.** 267 − 233 = 34 is
  the arithmetic, but 233 was called "the widest literal preamble" while the
  headroom the rule leaves is over `prefix + preamble` — the sum the rule
  bounds. At this tip those are 179 and 209 at the same row.

The sentence now reads 58 characters of headroom on the widest HEAD, 209 at
`bwrapgitrefslive_test.go:598` (30 of prefix plus a 179-character literal
preamble), says which quantity it is, and points the reader at the arm's own
`Logf` rather than at itself. The `Logf` no longer calls that sum "the widest
preamble" either.

## The `go test` rule, read in go1.27.3 rather than remembered

`$(go env GOROOT)/src/testing/testing.go` on this box, MEASURED 2026-11-22:

- the helper-skipping walk is `(*common).frameSkip`, and it returns `frame`
  at `if _, ok := c.helperNames[fnName]; !ok { // Found a frame that wasn't
  inside a helper function`. The set it tests is `helperNames` — function
  NAMES derived from the PCs `t.Helper()` recorded — so a closure, whose name
  is its own (`pkg.TestFoo.func1`), is a separate frame from the function it
  is written in. That is the upstream fact under the closure distinction
  above.
- the formatter is `(*common).callSite`, which is
  `fmt.Sprintf("%s:%d: ", filepath.Base(file), line)` unless `-test.fullpath`
  is set. **This file's prose calls that function `decorate()`** — the older
  upstream name, kept here because `x1ofd`'s decision and two of this file's
  older paragraphs name it; `wmaLinePrefix`'s docstring now names both.
- `scripts/posse-wall-check` runs `go test -count=1 -json -timeout 15m -run
  "$member_pattern" ./internal/posse` and no `-test.fullpath`, so the
  basename is what reaches a bead. A check that added that flag would make the
  whole prefix term — the pre-existing half as much as this one — an
  under-read by the length of every member file's path, and
  `wmaLinePrefix`'s docstring says so now.

## Not derived, and said so in the code

- **Four digits** for the helper-rendered line number, not a reading of the
  member files' own lengths. The longest member file is
  `wallunixlive_test.go` at 1218 lines and three others are past 1000, so a
  caller's line is three or four digits and four is the wider — but this term
  rides with the widest BASENAME, which is already a different file from the
  widest caller, and pairing the two would read as a precision the charge does
  not claim. A member file past 9999 lines is what would make it one short.
- **The first non-helper frame need not be in a member file at all.** A
  non-helper function in another file of `internal/posse`, called from a
  member, would decorate the line with its own basename, which this charge
  under-reads if that basename is wider. No such route is in the member set
  today. The member set is the one subject this census has, and a reading over
  the whole package would be a wider subject than the check's own definition
  of a member — which is what `wmaMemberPattern` exists to refuse.

## Verified

- `go vet ./internal/treepins`, `make fmt-check`.
- `go test ./internal/treepins -run 'TestQAEveryWallMember|TestQATheWallMember'`
  — all eight arms green, arm 6 logging `8 of them rendered inside a
  t.Helper() and charged ...:NNNN: (44)`.
- the nine-mutant battery above, one at a time, each reverted.
- `make treepins-check`, `make tree-check`, `make test`.

## Not closed here

- `ranger-base-0v6a3`
