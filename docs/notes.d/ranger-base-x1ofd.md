# ranger-base-x1ofd — the account key read three routes, the member set used four

`discovered-from: ranger-base-dxaxt`, which read the key while it was adding
the 300-character arm and found the route it does not read. Lane: code, class
debt — the line is correct today and held by nothing, which is
`ranger-base-momix`'s shape one route further on.

Every reading MEASURED 2026-11-22 on this box (linux 7.2.15-1-omarchy,
go1.27.3, 16 cores) at the landed tip `79b7bf11`.

## The route

`internal/treepins/wallmemberaccount_qa_test.go`'s key (`wmaAccountArg`)
recognised a seat's account three ways: a local assigned from a
seat-launching runner (`wmaAccountLocals`), a local assigned from one of the
two folds (`wmaFoldLocals`, `ranger-base-momix`), and an inline fold call.
Both local maps record locals ASSIGNED from something, so neither can ever
hold a parameter name — and the member set's fourth route is exactly that:

    internal/posse/bwrapgitrefslive_test.go:415, bgSeatAbstainAccount

        t.Skipf("NOTHING MEASURED: %s — %s. Nothing in this run read the
                 wall: …", what, account)

`account` is that function's own second parameter, bound by no `:=` anywhere
in the file. It is the ONE spelling of ADR 0097 D3's exit 3 — the abstention
every member's seat run routes through (`ranger-base-jqdm2`,
`ranger-base-evx07`) — and D3 reads an abstention's line with the same rule as
a failure's, by `evidence()`'s own docstring, so its account is spent out of
the same 300. Arm 2 graded it not at all.

## What the route reads

With the route added: **24 member files, 8 carrying an account, 66 account
messages — 32 runner local, 8 folded local, 25 inline fold, 1 parameter.**
That one is `bwrapgitrefslive_test.go:415` and nothing else; the count was 65
before, and no file joined the set.

The route is read at the CALL SITE (`wmaParamAccounts`) and not off the
parameter's name: a call of a function declared in the same file, whose
argument at position i is an account by one of the three routes above, makes
that function's i-th parameter an account. Nothing keys on `account`, `acct`
or any other spelling, for the reason `wmaMemberPattern` reads the check
instead of retyping it.

**One hop, and the param map is nil in the recursive position.** An account
that reaches a parameter FROM another parameter is two hops and is not read.
That is also what keeps the reading independent of the order `ast.Inspect`
meets the call sites in: fed back into itself the route stops being one hop
and becomes a fixpoint reached in walk order.

**The parameter's declared type must be `string`**, which is what an account
is by construction (`bgSeatAccount` and `wwOneLine` both return one). This is
a narrowing the tree asked for. Without it the route bound
`wallwatchlive_test.go`'s `code`, an **int**: the three routes above are
file-wide NAME sets, that file folds an account into a local named `got`, and
the same name is its watcher's exit code at `wwSeatAccount(got, said)` — so a
name-only reading propagates one existing over-read a hop further, into a
parameter no account can reach. With the type read, the route binds `account`
(the site) and `out` (already a runner local in that file, so the reading is
unchanged), and nothing else anywhere.

## The both-ways reading — six mutants, one at a time

Driven by a script that applies each mutant to a pristine copy, runs arms 2
and 3, and restores; the whole output is below.

| mutant | verdict | what red |
|---|---|---|
| 0 unmutated | GREEN | — |
| 1 the route dropped from `wmaAccountArg` | RED | the `"parameter"` floor, arm 3's two positive rows |
| 2 the string-type reading dropped | RED | arm 3's non-string row AND its variadic row; **nothing over the tree** |
| 3 the blank reading dropped | RED | arm 3's blank row |
| 4 one hop widened to two | RED | arm 3's second-hop row, in ONE of its two source orders |
| 5 the graded line's account moved behind a newline | RED | arm 2, at `internal/posse/bwrapgitrefslive_test.go:415` |
| 6 mutant 5 again with the route dropped | RED | only mutant 1's rows — **no `:415` at all** |

**Mutants 5 and 6 are the bead.** The line could have lost its account to a
newline at any time and this census would have said nothing; with the route
in, arm 2 names the file and the line.

**Mutant 2 red nothing over the tree**, which is the file's own discipline
(the head comment on the wrapper ceiling, `ranger-base-8752f`): the narrowing
is held hermetically by arm 3, not by a bound no tree reading witnesses.

**Mutant 4 is why there are two second-hop rows.** With one row — the inner
call written first — the mutant red NOTHING: `ast.Inspect` met `inner(t,
acct)` before anything had bound `outer`'s parameter, so the fixture passed by
walk order rather than by the rule, and the one-hop choice was witnessed by
nothing. The second row writes the outer call first and reds. The row that
found this was added because the battery was run, not because the rule was
re-read.

**And a guard came out, not in.** A variadic bail stood in `wmaParamAt` until
its mutant red nothing: a `...string` parameter's type is an `*ast.Ellipsis`
and not an `Ident`, so the string reading already refuses it, at every index
and past it. One line, one witness — arm 3's variadic row now reds under
mutant 2.

## Where that line sits inside the 300 at this tip

The prefix question the bead asked whoever widened the key to decide.
`bgSeatAbstainAccount` calls `t.Helper()`, and so does every function in its
chain (`bgSeatAbstain`, `bgSeatAbstainIfOutside`, `wwAbstainIfOutside`), so
`go test` decorates the line with the first NON-helper frame — the CALLING
member file and its line, never `bwrapgitrefslive_test.go`'s. A prefix term
charging the helper's own basename is the wrong figure.

**Decided: charge the widest basename in the member set**, derived from the
set the way `wmaMemberPattern` derives the set itself — 37 on this date
(`wallwatchbackstopwiring_linux_test.go`), so 44 with a four-digit `:NNNN: `.
It is the strictest honest reading; a narrower one under-reads, and the actual
caller cannot be known statically for a chain with 48 of them. **The rule
stays a floor either way**: a line that fits at its own caller and not at the
widest is still one caller away from being cut.

Against that term, at this tip:

| term | characters |
|---|---|
| prefix, widest member basename + `:NNNN: ` | 44 |
| literal preamble (`NOTHING MEASURED: ` + ` — `) | 21 |
| the account, `exit 0` … `exit status 128` | 205 … 214 |
| left for the caller's `what` | 30 … 21 |

The shortest COMPLETE `what` literal in the member set is 29 characters ("the
seat that ended by itself"), and the 44 of the chain's 48 call sites that open
their `what` with a literal run from there to 78 characters of it plus an
interpolated script or argv. So the route's one
graded line is at **299 of 300 in its single best case** and over it at almost
every caller — cut on the account's literal tail, exactly as
`ranger-base-uu303` measured for the other 32 messages. That tail is one
function and shortening it is `ranger-base-dxaxt`'s decision, not this bead's;
this bead only makes the line visible to a census.

## Not landed here

Arm 6 — the 300-character rule itself — is `ranger-base-dxaxt`'s and was not
in the tree at `79b7bf11`, so the bead's "both arms grade that line" is half
done by construction: arm 2 grades it, and the prefix arm 6 needs is decided
and recorded above rather than implemented in someone else's lane. That bead
and this one both edit `wallmemberaccount_qa_test.go`.

## Not closed here

- ranger-base-dxaxt
- ranger-base-bj1hh

## The battery, verbatim

```
--- mutant 0 unmutated: GREEN (exit 0)
--- mutant 1 the parameter route dropped from wmaAccountArg: RED (exit 1)
      wallmemberaccount_qa_test.go:1420: wmaAccountArg says argument 1 is an account = false, want true
      wallmemberaccount_qa_test.go:1420: wmaAccountArg says argument 0 is an account = false, want true
      wallmemberaccount_qa_test.go:1053: this arm read 0 account(s) by the "parameter" route, want at least 1
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter_of_an_abstention,_from_an_inline_fold_at_the_call_site
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter,_from_a_folded_local_at_the_call_site
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine
--- mutant 2 the string-type reading dropped from wmaParamAt: RED (exit 1)
      wallmemberaccount_qa_test.go:1390: wmaParamAccounts found map[parts:true], want []
      wallmemberaccount_qa_test.go:1390: wmaParamAccounts found map[code:true], want []
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/a_variadic_parameter_binds_nothing
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/a_parameter_that_is_not_a_string_cannot_be_holding_an_account
--- mutant 3 the blank reading dropped from wmaParamAt: RED (exit 1)
      wallmemberaccount_qa_test.go:1390: wmaParamAccounts found map[_:true], want []
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/a_blank_parameter_binds_nothing
--- mutant 4 one hop widened to two (the param map fed back in): RED (exit 1)
      wallmemberaccount_qa_test.go:1393: wmaParamAccounts found map[account:true acct:true], want [acct]
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/an_account_two_hops_from_the_fold_is_not_read,_the_outer_call_first
--- mutant 5 the graded line's account moved behind a newline: RED (exit 1)
      wallmemberaccount_qa_test.go:1020: internal/posse/bwrapgitrefslive_test.go:415: this t.Skipf renders
          the seat's account account AFTER the first newline of its format string …
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine
--- mutant 6 that same line, with the route dropped again (the state before this bead): RED (exit 1)
      wallmemberaccount_qa_test.go:1420: wmaAccountArg says argument 1 is an account = false, want true
      wallmemberaccount_qa_test.go:1420: wmaAccountArg says argument 0 is an account = false, want true
      wallmemberaccount_qa_test.go:1053: this arm read 0 account(s) by the "parameter" route, want at least 1
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter_of_an_abstention,_from_an_inline_fold_at_the_call_site
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter,_from_a_folded_local_at_the_call_site
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine
--- restored
```

Mutant 6's output carries no `bwrapgitrefslive_test.go:415` line. That absence
is the finding.

## What landing it actually took (ranger-base-0v6a3)

This bead's commit did not fast-forward. `main` had meanwhile landed
`ranger-base-bj1hh` and `ranger-base-dxaxt`, and dxaxt refactored the key into
one walk — `wmaAccountMessages`, returning a `wmaMsg` per account message, read
by arm 2 and by arm 6 both — while this bead was still wiring the fourth route
into arm 2's own inline walk. So the two sides edited the same twenty lines for
unrelated reasons and the replay was a real merge, not a markers-only one.
Replayed under `ranger-base-0v6a3` as one commit, MEASURED 2026-11-22 at
`7dc3313e` (same box, same go):

- The route moved to `wmaAccountMessages`, which is the one place the key now
  lives, so arm 2 AND arm 6 read it off the same walk. `wmaParamAt`,
  `wmaParamAccounts`, `wmaAccountArg`'s fourth map and arm 3's eight planted
  rows are unchanged from what this bead wrote.
- Arm 2: 24 member files, 8 carrying an account, **66** messages — 32 runner
  local, 8 folded local, 25 inline fold, 1 parameter — and 4 deferring prose.
  The same figures as at `79b7bf11`.
- Arm 6 now grades that line too, 66 of 66, and it is green: the budget is 267
  after dxaxt's 27-character tail, and the line lands at 84 of 300 charged at
  its own basename.

### The one thing the two tips disagree about

The "WHERE THAT LEAVES THIS LINE" reading above — 270 to 279 spent before one
character of the caller's `what`, the line at 299 of 300 in its best case — was
taken against the **199-character** silence tail that `79b7bf11` still carried.
`ranger-base-dxaxt` cut that tail to 27, so at `7dc3313e` the same line spends
21 of literal preamble and 30 of prefix against a 33-character account: **84 of
300**, with 183 of headroom at its own basename and 202 at the widest. The
reading is kept above as taken, and this is what it reads today.

The prefix decision recorded above — charge the WIDEST basename in the member
set, 37 (`wallwatchbackstopwiring_linux_test.go`), 44 with a four-digit
`:NNNN: ` — is still the decision and is still not implemented: arm 6 charges
`bwrapgitrefslive_test.go:459` its own 30. The under-read is 14 characters
against 202 of headroom, so it hides no red at any of the chain's 52 callers
(41 of `bgSeatAbstainIfOutside` and 11 of `wwAbstainIfOutside` at `7dc3313e`,
against 48 at `79b7bf11`),
which is why the route landed without it. `ranger-base-q41e8` carries the
landing, and `wmaLinePrefix`'s docstring says there that this is the one
message it is wrong about.

### The battery re-run at the replay tip, verbatim

The same six mutants, re-applied at `7dc3313e` where the route lives in
`wmaAccountMessages` rather than in arm 2's own walk — because a resolution that
moves the code moves what each mutant reaches, and the head comment asserts
these reds. One python script under the session scratchpad, run by path, each
edit applied alone and every file restored at the end — `all files restored` is
the script asserting that against its own pristine copies, not a claim.

Mutant 4 reds the "outer call first" row and not the "inner call first" one,
which is why that row has a twin; mutant 2 reds two arm 3 rows and nothing over
the tree. The pair that is the bead: mutant 5 reds arm 2 AT
`internal/posse/bwrapgitrefslive_test.go:459`, and mutant 6 — the same newline
with the route dropped again — reds the route's rows and its floor and names
that line nowhere.

```
--- mutant 0 unmutated: GREEN (exit 0)
--- mutant 1 the parameter route dropped from wmaAccountArg: RED (exit 1)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim (0.00s)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter_of_an_abstention,_from_an_inline_fold_at_the_call_site (0.00s)
      wallmemberaccount_qa_test.go:1736: wmaAccountArg says argument 1 is an account = false, want true — arm 2's parameter route no longer means what this file says it means:
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter,_from_a_folded_local_at_the_call_site (0.00s)
      wallmemberaccount_qa_test.go:1736: wmaAccountArg says argument 0 is an account = false, want true — arm 2's parameter route no longer means what this file says it means:
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine (0.39s)
      wallmemberaccount_qa_test.go:1369: this arm read 0 account(s) by the "parameter" route, want at least 1 (on 2026-11-22: 32 runner local, 8 folded local, 25 inline fold, 1 parameter). That route is in 
      wallmemberaccount_qa_test.go:1382: 24 member file(s), 8 of them carrying a seat account, 65 account message(s) read (32 by a runner local, 8 by a folded local, 25 by an inline fold, 0 by a parameter),
--- mutant 2 the string-type reading dropped from wmaParamAt: RED (exit 1)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim (0.02s)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/a_variadic_parameter_binds_nothing (0.00s)
      wallmemberaccount_qa_test.go:1706: wmaParamAccounts found map[parts:true], want [] — arm 2's parameter route no longer binds what this file says it binds:
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/a_parameter_that_is_not_a_string_cannot_be_holding_an_account (0.00s)
      wallmemberaccount_qa_test.go:1706: wmaParamAccounts found map[code:true], want [] — arm 2's parameter route no longer binds what this file says it binds:
--- mutant 3 the blank reading dropped from wmaParamAt: RED (exit 1)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim (0.00s)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/a_blank_parameter_binds_nothing (0.00s)
      wallmemberaccount_qa_test.go:1709: wmaParamAccounts found map[_:true], want [] — arm 2's parameter route no longer binds what this file says it binds:
--- mutant 4 one hop widened to two (the param map fed back in): RED (exit 1)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim (0.00s)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/an_account_two_hops_from_the_fold_is_not_read,_the_outer_call_first (0.00s)
      wallmemberaccount_qa_test.go:1709: wmaParamAccounts found map[account:true acct:true], want [acct] — arm 2's parameter route no longer binds what this file says it binds:
--- mutant 5 the graded line's own account moved behind a newline: RED (exit 1)
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine (0.34s)
      wallmemberaccount_qa_test.go:1341: internal/posse/bwrapgitrefslive_test.go:459: this t.Skipf renders the seat's account account AFTER the first newline of its format string. `scripts/posse-wall-check`
      wallmemberaccount_qa_test.go:1384: 24 member file(s), 8 of them carrying a seat account, 66 account message(s) read (32 by a runner local, 8 by a folded local, 25 by an inline fold, 1 by a parameter),
--- mutant 6 the same newline mutation WITH the route dropped again: RED (exit 1)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim (0.00s)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter_of_an_abstention,_from_an_inline_fold_at_the_call_site (0.00s)
      wallmemberaccount_qa_test.go:1736: wmaAccountArg says argument 1 is an account = false, want true — arm 2's parameter route no longer means what this file says it means:
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter,_from_a_folded_local_at_the_call_site (0.00s)
      wallmemberaccount_qa_test.go:1736: wmaAccountArg says argument 0 is an account = false, want true — arm 2's parameter route no longer means what this file says it means:
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine (0.43s)
      wallmemberaccount_qa_test.go:1369: this arm read 0 account(s) by the "parameter" route, want at least 1 (on 2026-11-22: 32 runner local, 8 folded local, 25 inline fold, 1 parameter). That route is in 
      wallmemberaccount_qa_test.go:1382: 24 member file(s), 8 of them carrying a seat account, 65 account message(s) read (32 by a runner local, 8 by a folded local, 25 by an inline fold, 0 by a parameter),
all files restored
```

## What landing it took the SECOND time (ranger-base-zr2mf)

The replay above did not fast-forward either, and this is the useful half of
the record: the thing that blocked it was the decision it had deferred. `main`
landed four commits in the window — `ranger-base-p51zu`, `ranger-base-72z36`,
`ranger-base-q41e8`, `ranger-base-hcig1` — and two of them are this bead's own
subject matter:

- **`ranger-base-q41e8` landed the prefix decision recorded above**, for all
  eight helper-rendered messages arm 6 already graded, with
  `wmaRenderedInHelper`, `wmaHelperPrefix` and `wmaChargedPrefix` in the key's
  one walk. So the two sides conflicted on the same twenty lines again, for the
  same reason as the first replay and with the roles reversed: the first replay
  moved the route into dxaxt's walk, and this one drops the route's own
  arithmetic because q41e8 made it void.
- **`ranger-base-72z36` swapped the graded line's own argument order**, from
  `t.Skipf("NOTHING MEASURED: %s — %s. …", what, account)` to `account, what`.
  The account is now **argument 0**, the literal preamble before it is
  `NOTHING MEASURED: ` and nothing else (**18** characters, where it was 21
  with `what` in front), and the line is **:485** where it was :459.

### What the resolution dropped, and why that is the whole of it

Two paragraphs. The head comment's "THE PREFIX FOR A HELPER-RENDERED MESSAGE
… NOT realized here" and its "WHY THE ROUTE LANDS WITHOUT IT", plus
`wmaLinePrefix`'s "this reader keeps the simple rule until that bead lands" —
all three priced an under-read of 14 characters that this tree no longer has.
Nothing of the ROUTE was dropped: `wmaParamAt`, `wmaParamAccounts`,
`wmaAccountArg`'s fourth map, arm 2's parameter floor and arm 3's eight planted
rows are what the first replay wrote. The code conflict was one line on each
side of the same `wmaAccountMessages` preamble — `spans := wmaFuncSpans(f.ast)`
against `param := wmaParamAccounts(f, runner, folded)` — and both are kept.

### The figures re-read at this tip, not carried

MEASURED 2026-11-22 at `814776ed` (linux 7.2.15-1-omarchy, go1.27.3), every one
off the arms' own `Logf` or off a probe that prints what arm 6 charges:

| | at `7dc3313e` (first replay) | at `814776ed` (this one) |
|---|---|---|
| arm 2 | 24 files, 8 carrying, 66 messages (32/8/25/1) | the same |
| arm 6 graded | 66 of 66 | 66 of 66 |
| helper-rendered | not read — q41e8 had not landed | **9** of 66, 8 of them q41e8's |
| the graded line | `bwrapgitrefslive_test.go:459`, charged 30 | **`:485`, charged 44** |
| its preamble / head | 21 / 51 | **18 / 62** |
| widest head in the set | — | **209** at `bwrapgitrefslive_test.go:624` |
| the chain's call sites | 52 | **52** (41 + 11, re-counted) |

So the route's message is the **ninth** helper-rendered one and the
**narrowest** of the nine: 62 against 91, 97, 82, 105, 98, 115, 124 and 117 for
the other eight. The widest head over the whole set is 209 and it is NOT a
helper-rendered message — the same figure and the same line as before this
route landed, which is the measured form of "the route moves no headroom".

### The battery re-run again, seven mutants, verbatim

Re-run because the resolution moved the code a second time, and with one
mutant added: q41e8's reading is now in the walk this route is read off, so the
question "does the 66th message make the TREE witness the prefix term" has an
answer and it is no. One python script under the session scratchpad, run by
path, each edit applied alone, every file restored and `git diff` read back.

```
=== MUTANT 0 unmutated  (meant to red: nothing)
    exit 0, 0 failing test(s)

=== MUTANT 1 route dropped from wmaAccountArg
    exit 1, 4 failing test(s)
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter_of_an_abstention,_from_an_inline_fold_at_the_call_site
      --- FAIL: TestQATheWallMemberAccountRulesMatchWhatTheyClaim/the_account_parameter,_from_a_folded_local_at_the_call_site
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine
      | wallmemberaccount_qa_test.go:1505: this arm read 0 account(s) by the "parameter" route, want at least 1
      | wallmemberaccount_qa_test.go:1518: 24 member file(s), 8 of them carrying a seat account, 65 account message(s) read (32 by a runner local, 8 by a folded local, 25 by an inline fold, 0 by a parameter)

=== MUTANT 2 the string-type reading dropped from wmaParamAt
    exit 1, 3 failing test(s)
      --- FAIL: .../a_variadic_parameter_binds_nothing
      --- FAIL: .../a_parameter_that_is_not_a_string_cannot_be_holding_an_account
      | wmaParamAccounts found map[parts:true], want []
      | wmaParamAccounts found map[code:true], want []

=== MUTANT 3 the blank reading dropped from wmaParamAt
    exit 1, 2 failing test(s)
      --- FAIL: .../a_blank_parameter_binds_nothing
      | wmaParamAccounts found map[_:true], want []

=== MUTANT 4 one hop widened to two
    exit 1, 2 failing test(s)
      --- FAIL: .../an_account_two_hops_from_the_fold_is_not_read,_the_outer_call_first
      | wmaParamAccounts found map[account:true acct:true], want [acct]

=== MUTANT 5 the graded line's own account behind a newline
    exit 1, 1 failing test(s)
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine
      | wallmemberaccount_qa_test.go:1477: internal/posse/bwrapgitrefslive_test.go:485: this t.Skipf renders the seat's account account AFTER the first newline of its format string.
      | wallmemberaccount_qa_test.go:1520: 24 member file(s), 8 of them carrying a seat account, 66 account message(s) read (… 1 by a parameter)

=== MUTANT 6 the same, with the route dropped again  (meant to red: that line nowhere)
    exit 1, 4 failing test(s)
      --- FAIL: .../the_account_parameter_of_an_abstention,_from_an_inline_fold_at_the_call_site
      --- FAIL: .../the_account_parameter,_from_a_folded_local_at_the_call_site
      --- FAIL: TestQAEveryWallMemberMessageCarriesItsSeatAccountOnTheFirstLine
      | wallmemberaccount_qa_test.go:1505: this arm read 0 account(s) by the "parameter" route, want at least 1
      (and bwrapgitrefslive_test.go:485 named nowhere)

=== MUTANT 7 wmaRenderedInHelper answers 'not a helper' everywhere  (NEW)
    exit 1, 1 failing test(s)
      --- FAIL: TestQATheWallMemberBudgetReadersMatchWhatTheyClaim
      | "PLAIN FUNC" / "MARKED FUNC" / "CLOSURE INSIDE A HELPER" / "CLOSURE THAT MARKS ITSELF" / "BESIDE THAT CLOSURE": all five arm 7 rows red
      | arm 6 GREEN — the tree witnesses the prefix term no more with 66 messages than it did with 65

tree restored: (no stray mutation)
```

Mutant 6 is the pair that matters, as it was the first time: the newline at
`:485` is seen by arm 2 only because the route exists, and with the route
dropped again that line is named nowhere. Mutant 7 is the new reading: arm 6 is
green whether or not the nine are recognised as helper-rendered, so the term
stays held in arm 7 over planted source exactly where `ranger-base-q41e8`'s
mutant 8 put it — the 66th message does not change that, and the arm's logged
count is not a floor for the reason arm 6 says there.

### Two figures this landing did NOT fix

Both are on `main` and both predate this replay, so they are filed rather than
fixed here — see `ranger-base-yem73`:

- arm 2's deferred-prose floor names four messages where the arm logs **6**.
- q41e8's head-comment paragraph names `packUnderALiveSeat at :841` and
  `wallclaudeprojectsattack_qa_test.go:389`, which `ranger-base-72z36` had
  already moved to `:867` and `:403` before q41e8 landed — a figure carried
  across the landing that falsified it, which is the error that same paragraph
  warns about one sentence earlier.
