# ranger-base-vyz61 — the one figure in that file no arm prints, and how to re-derive it

Landed 2026-11-22, linux 7.2.15-1-omarchy, go1.27.3, at tip `3c296c07`.

`internal/treepins/wallmemberaccount_qa_test.go`'s head comment paragraph *"AND
THE TERM IS STILL WITNESSED HERMETICALLY AND NOT BY THE TREE"* still charged
*"those eight 27 to 41"* and read *"nine of the 65 rather than eight"*.
`ranger-base-x1ofd` (`0687d302`) added arm 2's fourth account route, which made
the graded set 66 and the helper-rendered set 9; it updated every other figure
in the file and left this paragraph alone. Fourth instance in this file of the
class `ranger-base-yem73` was filed for — a figure carried across the landing
that falsified it.

## Why this one was the one left

`ranger-base-yem73` already fixed the durable half for the two figures that
*are* readable off a run: arms 2 and 6 LIST their sets by file and line in
their own `Logf`, so the deferred set and the helper-rendered set are a `-v`
run away and no paragraph has to be the record of one.

The sloppier reading's count is not in that class. It is the count a mutant
reads — arm 7's hermetic case, over planted source — so no arm's output
contains it at any tip, and the only honest way to state it is to re-derive it.
That is what went stale: the figure with no printed counterpart.

## The battery, four runs, one mutant at a time

A throwaway script, not committed and not needed to re-run this: for each
mutant, patch the file, `go test -run '<arm 6>|<arm 7>' -v ./internal/treepins`,
then restore from a pristine copy and assert it is byte-equal. The mutations
are the whole of it:

| | mutation | reads |
|---|---|---|
| base | none | the tree as it stands |
| M1 | `wmaRenderedInHelper` returns `false` always | is arm 6 still green at the other charge |
| M1b | `wmaChargedPrefix` always charges `wmaLinePrefix`, listing preserved | each helper-rendered message's head at its OWN position |
| M2 | `wmaFuncSpans` drops the `FuncLit` span and the `FuncLit` bail | the sloppier reading's count, and which message it adds |

`M1` loses arm 6's list (the `m.helper` flag is what the arm lists on), which
is why `M1b` exists: the same charge with the branch disabled instead of the
reading, so the `Logf` still names the nine and prints each head at its own
position. The per-message charge is then `44 - (head_base - head_M1b)`.

## Output

base — arm 6 and arm 7 green:

    24 member file(s), 66 account message(s) graded against
    scripts/posse-wall-check's 300 characters, 9 of them rendered inside a
    t.Helper() and charged `wallwatchbackstopwiring_linux_test.go:NNNN: ` (44)
    for the caller go test prints instead of their own position:
    [bwrapgitrefslive_test.go:485 (in bgSeatAbstainAccount, head 62)
     bwrapgitrefslive_test.go:867 (in packUnderALiveSeat, head 91)
     wallclaudeprojectsattack_qa_test.go:386 (in a func literal, head 97)
     wallclaudeprojectsattack_qa_test.go:403 (in a func literal, head 82)
     wallwatchlive_test.go:139 (in launchOn, head 105)
     wallwatchlive_test.go:143 (in launchOn, head 98)
     wallwatchlive_test.go:300 (in wwWaitKilled, head 115)
     wallwatchlive_test.go:303 (in wwWaitKilled, head 124)
     wallwatchlive_test.go:604 (in a func literal, head 117)]

M1 — arm 6 **green**, arm 7 red on two rows (`MARKED FUNC`, `CLOSURE THAT
MARKS ITSELF`), and arm 6's list reads `0 of 66`. So the term is still
witnessed hermetically and not by the tree, one route after
`ranger-base-dxaxt`'s mutant 8 said so.

M1b — arm 6 **green**, arm 7 red on the charge row, and the nine heads drop to
48, 77, 94, 79, 88, 81, 98, 107, 100. Against base that is, per message:

| message | own position | decision |
|---|---|---|
| `bwrapgitrefslive_test.go:485` (the parameter route's own) | 30 | 44 |
| `bwrapgitrefslive_test.go:867` | 30 | 44 |
| `wallclaudeprojectsattack_qa_test.go:386` | 41 | 44 |
| `wallclaudeprojectsattack_qa_test.go:403` | 41 | 44 |
| `wallwatchlive_test.go:139` `:143` `:300` `:303` `:604` | 27 | 44 |

**27 to 41 against 44** — the range the paragraph already carried, unmoved by
`x1ofd`'s route: its message charges 30 at its own position, inside the range
the other eight already spanned. So the route widened the count and not the
arithmetic, which is why it landed with no arithmetic of its own.

M2 — arm 6 **green**, arm 7 red on three rows (`CLOSURE INSIDE A HELPER`,
`CLOSURE THAT MARKS ITSELF`, `BESIDE THAT CLOSURE`), and arm 6's list reads
**10 of 66**: the nine, plus `wallwatchlive_test.go:614` and nothing else —
adds one, drops none. It is the `t.Errorf` in
`TestLiveWallWatchReadsAnOutsideKillAsNothingMeasured`'s body, written beside
the `run` closure that calls `t.Helper()`, and go test decorates it with its
own file and line. Under M2 the attack file's two closures and
`wallwatchlive:604` are also re-attributed to their enclosing `TestLive…`
FuncDecl, which the `in …` half of the list shows and which is the same error
reading the other way.

So: **nine of 66, charged 27 to 41 where the decision charges 44; the sloppier
reading reads ten.**

## What the edit does besides the numbers

The pair of counts was typed TWICE — in that paragraph and in `wmaFuncSpan`'s
own docstring — and both copies were stale, which is the mechanism rather than
the accident. The counts and the re-derivation route are now said once, in the
head comment under *"AND THE SLOPPIER READING'S COUNT IS PRINTED BY NOTHING"*;
the docstring states the distinction and that the sloppier reading reads one
message more, with no count of its own. `wallwatchlive_test.go:614` is likewise
now typed in one place only — arm 7's `BESIDE THAT CLOSURE` row, which is the
thing that reds if the distinction breaks — where the head comment used to name
it too.

## Not closed here

- `wallwatchlive_test.go:614` in that arm 7 row is still a member line number
  typed by hand, so it is the same class as this bead at a smaller size: a
  landing that moves that line leaves the row's prose naming the wrong one
  while the row itself stays green, because what it grades is planted source.
  Deriving it would mean the hermetic arm reading the member set, which is the
  thing arm 7 exists not to do.
