# ranger-base-72z36 — three wall-member accounts reached the bead with 0 to 3 characters, and the fix is three argument swaps

Findings bead of `ranger-base-bzm06`'s verify, and the class is
`ranger-base-bj1hh`'s unchanged: a seat's account is rendered BEHIND an
interpolated path, so `scripts/posse-wall-check`'s `evidence()` spends its 300
characters before the account's own verb and the launcher's P1 bead gets
nothing. `bj1hh` closed it in `internal/posse/bwrapseatstorelive_test.go`. What
it did not close is the same class in two other member files, and in the one
message of the member set that no route of the census key reads.

Every figure below is a LIVE RENDER and not arithmetic over a format string,
MEASURED 2026-11-22 on this box (linux 7.2.15-1-omarchy, go1.27.3, bubblewrap
0.13.1) at tip `7dc3313e`: each row was planted as an unconditional `t.Logf`
of its own format string and its own arguments, the member was run with
`-count=1 -v`, and the line was sliced the way `evidence()` slices it —
`line.strip()`, then `text[:300]`, in runes, with `go test`'s own
`<basename>:<line>: ` prefix INSIDE the 300.

That venue is the one `ranger-base-uu303` records rather than files, at the
size of one bead. It took under ten minutes of box over three files, and it is
what a static census cannot do: the term that blows the budget is a path whose
width is the fixture's.

## The three rows, before and after

| row | message | account's verb starts at | after |
|---|---|---|---|
| 1 | `bwrapgitrefslive_test.go`'s `bgSeatAbstainAccount` | 196 to 592 over 42 renders | a fixed 50 |
| 2 | `wallclaudeprojects_qa_test.go`'s no-transcript `t.Errorf` | 254 of 300 | 115 |
| 3 | `wallclaudeprojectsattack_qa_test.go`'s ended-badly `t.Logf` | 252 and 286 | 79 for both cwds |

**Row 1** is the whole of the abstention sentence, shared by every seat run
that routes `bgSeatAbstainIfOutside`, and `what` — a caller's joined argv, a
whole shell script, or a fixture cwd — was rendered in FRONT of the account.
ADR 0097 D3's exit 3 spends the same 300 as a failure line (`evidence()`'s own
docstring: one rule for a failure line and a skip line). Repro: turn that
`t.Skipf` into a `t.Logf`, force `bgSeatAbstainIfOutside`'s criterion true, and
run `go test -count=1 -v -run TestLiveBwrapSeatStore ./internal/posse`. Before:
42 renders, the account's verb at 196 to 592, **19 of 42 reached the bead with
nothing at all**, 22 arrived cut mid-account, and 27 could not have fitted a
33-character silence account; the worst, 592, from
`bwrapseatstorelive_test.go:475`. After: **a fixed 50 at all 42** — `go test`'s
32-character prefix for that basename plus `NOTHING MEASURED: `'s 18 — every
silence account whole, and the only two renders still truncated carry accounts
of 261 and 334 characters of what the seat ITSELF said, 250 of each arriving,
which no order fits inside 300.

**Row 2** rendered 374 characters with the verb at 254, so 46 of a
120-character account arrived. The 139 characters in front of it are the key
dir: the fixture root, plus claude's own key-dir mangling of the session dir,
which spells that root a SECOND time inside one directory name — so a root 23
characters wider than this run's puts the account's verb past the slice and the
bead gets nothing.

**Row 3** rendered 372 characters with the verb at 252 for one cwd and 406 with
it at 286 for the other: 48 and 14 of a 120-character account, and a silence
account loses 19 of its 33 on the second. The `t.Fatalf` four lines above it
already had the right order — `ranger-base-mklu0` landed that one on purpose —
and this line kept the cwd in front.

## The fix is the order, and that is all it is

Three argument swaps and the prose that follows them: the account first, the
path and the reasoning on line 2, where ADR 0097 D3 says supporting prose costs
nothing. `bgSeatAbstainAccount` has no newline to need, so its account simply
moves ahead of `what`; rows 2 and 3 defer their path to a second line, which is
the shape `ranger-base-mklu0` landed and arm 2 of
`internal/treepins/wallmemberaccount_qa_test.go` grades as correct.

What that buys is not headroom, it is INDEPENDENCE: after the swap, the
account's place is `prefix + a literal this file controls` at every caller
there will ever be, and no fixture root, TMPDIR or argv can move it. Row 2's 46
characters of headroom were never a budget.

## Why nothing red, and what still cannot see this

Arm 6 of `internal/treepins/wallmemberaccount_qa_test.go`
(`TestQAEveryWallMemberAccountFitsInsideTheEvidenceBudget`) counts every
interpolated verb as ZERO by design and says so in its own report: it graded
row 2 at 116 and row 3 at 206, both inside the derived 267, and it cannot see
row 1 at all because that account arrives as a PARAMETER and the parameter
route is not in the key on `main`. The arm is a correct FLOOR; this is its
documented residual, and the three live rows are what the floor cannot reach.

After the landing it still grades 65 account messages over 24 member files with
the same 267-character budget, and every arm of that file is green.

Not closed here:

- `ranger-base-x1ofd`
- `ranger-base-uu303`
