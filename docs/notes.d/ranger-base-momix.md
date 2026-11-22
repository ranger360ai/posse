# ranger-base-momix — the account census refused the shape the house prescribes, and graded 3 of 24 files

Finding 1 of `ranger-base-0b6zo`'s verify (4 closes: `ranger-base-6r6ox`,
`-x5zmk`, `-mklu0`, `-6felj`), which escaped `ranger-base-mklu0`'s close. It
was filed as a DECISION and not a patch — a census rule and the key that reads
the tree with it could not both stand — so what is below is the measurement
that decided it and the mutants that hold the answer.

Findings 2 and 3 of the same bead are in another repository, and are named here
by bead id and nothing else (`ranger-base-momix`, `ranger-base-0b6zo`), per
AGENTS.md, "Landing the plane".

Every reading MEASURED 2026-11-22 on this box (linux 7.2.15-1-omarchy,
go1.27.3, 16 cores) at the landed tip `e5221de6`.

## The two readings, and the measurement that cannot leave both standing

`internal/treepins/wallmemberaccount_qa_test.go` arm 2
(`TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine`) said **NO
NEWLINE AT ALL** in a message carrying a seat's account, and its report said of
a message that had one that "the account is dropped in transit". That is false
when the account is in FRONT of the newline: `scripts/posse-wall-check`'s
`evidence()` carries the first `file.go:NN:` line, and `go test` puts
everything up to the first `\n` on exactly that line. ADR 0097 D3 and
`ranger-base-mklu0`'s own fix shape both prescribe that shape — the account
first, the cwd and the reasoning on line 2, "where D3 says supporting prose
costs nothing" (`docs/notes.d/ranger-base-mklu0.md`).

**The only reason the landing was green is that the key read almost nothing.**
`wmaAccountLocals` recognised an account solely as a local assigned from a
multi-return seat-LAUNCHING `FuncDecl` whose last result is a string.

| | the old key | the key now |
| --- | --- | --- |
| member files carrying an account | 3 of 24 | **8** of 24 |
| account messages read | 32 | **65** |
| by route | 32 runner local | 32 runner local, 8 folded local, 25 inline fold |
| with a newline in them | 0 | **4**, all four with the account in front |

The four are `wallclaudeprojectsattack_qa_test.go:386` — the line
`ranger-base-mklu0` landed on purpose — and `wallunixlive_test.go:237`, `:526`
and `:1130`, whose `the seat exited badly: %s\nargv: bwrap %s` is three more of
the same shape from a different lineage. **Nothing in the member set puts an
account after a newline.** So the old rule was never a strictness the tree was
paying for: it was a rule that could only ever red the shape the house
prescribes, held off by a key too narrow to reach any of the four — while the
three files it did reach folded everything onto one line for a rule the house
does not hold. The rule is now "the account is rendered before the first
newline", which still reds `ranger-base-a688i`'s own shape (a format ending
`:\n%s` whose `%s` is the account) and no longer reds D3's.

`internal/posse/wallclaudeprojectsattack_qa_test.go` is the file where this
class has now been found twice — under `ranger-base-eheo3`'s close and under
`ranger-base-mklu0` — and it contributed nothing to the census: its runner is a
closure with one result, and its account comes from `wwOneLine` and from an
inline `bgSeatAccount(...)` argument. Neither spelling is an `Ident` assigned
from a `FuncDecl`.

## What the key reads now

Three routes, kept apart so each can be floored, which is
`ranger-base-hhps5`'s lesson one arm over — an account this census cannot see
does not lower the count of the ones it can:

- **runner local** — the reading that was there: a local bound to the last
  string result of a seat-launching `FuncDecl`.
- **folded local** — a local bound to a call of an account fold.
- **inline fold** — a fold call handed straight to the verb, binding nothing.

Reading the FOLD rather than the runner is what makes the key independent of
how the seat was launched: a closure, a one-result helper, a bare inline call.
The vocabulary is two names (`bgSeatAccount`, `wwOneLine`) because the package
says it is two — `bwrapgitrefslive_test.go`'s own head comment is the contract,
"THE FOLD IS wwOneLine's AND NOT A SECOND SPELLING OF IT … two foldings of 'put
this on one line' are two answers waiting to disagree about what a member's
line may contain". A typed pair of names is the register this file refuses
everywhere else, so **arm 5**
(`TestQATheWallMemberAccountFoldsAreTheOnesThePackageDeclares`) reads the pair
back out of the member set: each name declared there once as a function
returning exactly one string, and `bgSeatAccount`'s body calling `wwOneLine`. A
drifted list here would not red — it would GRADE LESS, silently, which is the
state this bead found.

**Two narrownesses one layer down, both fixed with it.** The walk was gated on
the file binding an account to a local, which hid every inline fold in a file
that binds none — three of the four deferred-prose messages, three quarters of
what witnesses the rule. And a message whose argument positions cannot be read
off the text (an indexed `%[n]s`, a star width, a variadic spread) is now held
to the stricter rule WITH a report that says why, rather than being counted
with a guessed position. There are none in the member set today, so that arm is
held by arm 3's planted rows alone.

## The mutants

One mutant at a time, each planted into a pristine copy of the five files
below and restored before the next, then
`go test -count=1 -run 'TestQA(EveryWallMemberMessageCarries|TheWallMemberAccount)' ./internal/treepins`
(0.2-0.3s, no suite slot) and a count of the arms' errors. The plants, so this
is re-runnable without the harness that drove it — `P` is the pin, `G` is
`internal/posse/bwrapgitrefslive_test.go`, `W` is `wallwatchlive_test.go`, `U`
is `wallunixlive_test.go`, `A` is `wallclaudeprojectsattack_qa_test.go`, all
under `internal/posse/` but the pin:

```
P1  G  s/the rendered wall: %s/the rendered wall:\n%s/        (line 507's message)
P2  P  s/return argIdx < wmaVerbs(text[:i]), true, text[:i]/return false, true, text[:i]/
P3  P  s/case folded[id.Name]:/case false && folded[id.Name]:/
       s/isCall && wmaAccountFolds[wmaCallee(c)]/isCall && false/
P4  P  drop wmaFoldFunc from wmaAccountFolds
P5  P  s/const wmaFoldFunc = "wwOneLine"/const wmaFoldFunc = "wwOneLineX"/
P6  W  s/func wwOneLine(s string) string {/func wwOneLine(s string) (string, error) {/
P7  G  s/" + wwOneLine(out)$/" + out/                         (bgSeatAccount's last line)
P8  U  s/the seat exited badly: %s\nargv/the seat exited badly: %s argv/g
    A  s/it said %s\ncwd %s/it said %s cwd %s/
```

| | mutant | rc | what red |
| --- | --- | --- | --- |
| P0 | none | 0 | — |
| P1 | a graded account moved behind a newline (`ranger-base-a688i`'s shape) | 1 | arm 2, naming `bwrapgitrefslive_test.go:507` |
| P2 | the rule re-tightened to "no newline at all" | 1 | all four deferred messages AND the witness floor — the decision taken twice |
| P3 | the key narrowed back to the runner route alone | 1 | 10 rows: arm 3's four new key rows, both route floors, the witness floor, both census floors |
| P4 | the fold dropped from the vocabulary | 1 | arm 3, the folded-local floor; read falls 65 → 41, deferred 4 → 3 |
| P5 | the fold renamed out from under the vocabulary | 1 | arm 5's absence arm, arm 5's pair arm, the folded-local floor |
| P6 | the fold given a second result | 1 | arm 5's one-result arm, alone |
| P7 | `bgSeatAccount` stops calling the fold | 1 | arm 5's pair arm, alone |
| P8 | the four deferred messages folded onto one line | 1 | the witness floor, alone |

P2 is the row the decision rests on: re-tightening the rule reds four messages
whose shape ADR 0097 D3 prescribes, and that re-tightening is easy to type —
which is why the witness floor exists. P6 and P7 red one arm each with nothing
else, so arm 5 is not passing on a neighbour's reading.

## Not closed here

- ranger-base-uu303
