# The wall-pin one-line budget, censused over the member set instead of one file at a time (ranger-base-s9h52)

2026-11-21. Three beads hit the same shape in one day — `ranger-base-a688i`,
`ranger-base-w6kke`, `ranger-base-axt1f` — and each fix pinned ONE NAMED
FILE, which is exactly why nothing read the other members. This is the
census, the rule it settled on, and the three remaining instances it found.

## The budget, and the two ways to spend it on nothing

`scripts/posse-wall-check` reports a failed member by `evidence()`: the first
`file.go:NN:` line of that test's output which is not the venue marker,
truncated to 300 characters, and **nothing after it** (ADR 0097 D3). The
launcher puts that one line on a P1 bead. So a member's budget is one line,
and it is the member's job to spend it on a cause.

- `ranger-base-a688i`: a format string ending `:\n%s` whose `%s` is the only
  account of WHY. The bead read `the seat ended before it ran (exit 1):` and
  stopped there.
- `ranger-base-axt1f`: a runner that reduces its child's error to
  `err == nil` before any message is formatted. The exit status then exists
  as text nowhere, so `exit status 128` (the wall or the command) and
  `signal: killed` (the box) report identically, and **no line budget brings
  that difference back.**

## The decision the bead was filed to make

ranger-base-s9h52 asked for a priced exemption table or a narrower rule, and
said the bead had to say which. **It is a narrower rule, and there is no
exemption table anywhere in the landing.** Here is why, with the numbers.

The wide rule is ranger-base-axt1f's own, which that bead applied to its one
file: no format string in a member file carries a newline. MEASURED
2026-11-21 over the member set (linux 7.2.15, go1.27.3): **785 testing format
calls in 24 member files, 204 of them carrying a newline.** Fixing 204
messages in 24 live-wall files is not a census, and pricing 204 rows is a
register that cannot be read.

Narrowing by PROSE SHAPE does not converge either. The sharpest shape rule I
could state — the newline defers to something, and the first line carries no
format verb at all, so every measured value in the message is below the fold
— reads **44**. That is better, and it is still wrong: those 44 mix the class
with messages whose first line is a complete claim about an artifact the
reader can go and re-read (a render's argv, a verdict file, a bead's text).
Deciding which of the 44 is which is a judgement about someone else's prose,
44 times, and every row of it drifts.

So the census narrows by **subject** instead. It reads the functions that
launch a bwrap SEAT, and the messages that carry a seat's account. That is
the one place where the deferred value is the only account there will ever be
— a child's status and words, unreproducible after the fact — and it is what
both beads above actually hit. A message whose tail is an artifact the reader
can re-read is out of scope and needs no row saying so, because the rule
never reaches it.

## MEASURED 2026-11-21 (linux 7.2.15, go1.27.3, 16 cores, this tree)

`before` is this bead's base, `2a32fa07`. `after` is the landed tree, which
is that base rebased onto a main that had meanwhile landed
`ranger-base-xxtmo` and `ranger-base-5oiz4` — the second of which touches a
member file (`wallwatchbackstoplive_linux_test.go`) and so moves two of these
rows on its own. Both columns are read by the same program over the tree they
name.

| reading | before | after |
|---|---|---|
| member files (`^Test(QA)?Live(Bwrap\|Wall)`) | 24 | 24 |
| bwrap-launching functions in them | 30 | 30 |
| …rendering no child error at all | **3** | 0 |
| messages carrying a seat account after a newline | **19** | 0 |
| testing format calls in those files | 785 | 790 |
| …carrying a newline (the wide rule's reading) | 204 | 184 |
| …deferring with a constant first line (the shape rule's reading) | 44 | 34 |

The last three rows are not a before/after of this fix alone: 786 and 184 of
them are this bead's own reading, and main's two landings in the window added
four more format calls, none of which carries a newline. They are here
because they are the numbers the rule decision rests on, not because this
bead moved them.

The three:

- `bsLive.in` (`internal/posse/bwrapseatstorelive_test.go`) and `srLive.in`
  (`internal/posse/storerepogitwalllive_test.go`) — byte-identical copies of
  the two runners ranger-base-axt1f fixed, `return err == nil,
  strings.TrimSpace(string(out))`, between them feeding 19 messages that put
  that output on line 2.
- `TestLiveWallClaudeProjectsANonOwnCwdRunsTheRealClaudeAndWritesNoTranscript`
  (`internal/posse/wallclaudeprojectsattack_qa_test.go`) — a third copy the
  first two beads could not have seen, because it is not a runner at all:
  `go func() { out, _ = cmd.CombinedOutput(); close(done) }()` blanks the
  seat's error inside a goroutine. That pin asserts on the projects/ tree and
  not on the exit, so a claude the wall refused outright and a claude that
  ran reported identically, and the tree arm passes either way.

## What changed

Both runners return THREE results now: `(ok, out, account)`. The raw output
stays because callers READ it — a sha, a digest line, bwrap's `busy` — and
`bgSeatAccount`'s account is what they SAY. Keeping them as two results is
what lets both be true at once: one return expression, no branch, and no
caller choosing between them. A two-result runner answering "the output if it
ran, the account if it did not" was the first draft and it is the shape that
made `bsLive.in` unfixable without this split.

`bgSeatAccount` is reused and not respelled, for the reason
ranger-base-axt1f's fragment gives about `wwOneLine`: two spellings of "put
this on one line" are two answers waiting to disagree. Its silence sentence
lost the word `git` — these two fixtures run `touch` and `sh -c` beside git,
and "no git error" would have been a false statement in a P1 about a `touch`.

`internal/treepins/wallmemberaccount_qa_test.go` is the census, four arms:

1. every bwrap-launching function in a member file renders the error its
   Cmd waiter returned;
2. every message carrying a seat account carries it in a format string with
   no newline in it — not "the account is before the first newline", because
   everything after that newline is dropped in transit anyway, so a message
   that needs a tail is a message whose tail the bead will not get;
   **SUPERSEDED: that reading is the one ranger-base-momix reversed.** The
   rule is positional now — the account before the first newline — and its
   fragment carries the measurement that decided it: the key here read 3 of
   24 member files, and widening it put four messages under this rule whose
   account was in FRONT of the newline, which is the shape ADR 0097 D3
   prescribes. There are five arms, not four;
3. both rules driven over planted source with their own negative cases,
   through the same functions the tree arms use;
4. the member pattern is READ out of `scripts/posse-wall-check`
   (`member_pattern='…'`) and never retyped, so the census cannot disagree
   with the set it is a census of.

It enumerates a discovered file set (a `Glob` of `internal/posse/*_test.go`),
so it is in internal/treepins' one-package-census class and is doored by
`make treepins-check` — the complement door, the package minus the priced
table in `treewidedoor_qa_test.go`'s `twdTreepinsExcluded` (ADR 0098 D1). It
is in no such table, so it needed no Makefile edit and cannot go undoored
(ADR 0098 D4). That is the second half of the bead's premise answered: the
bead expected a hand-written door, and ADR 0098 had already made one
unnecessary.

## Shown able to fail: nine mutants, one at a time

Script in the scratchpad; every touched file restored from a byte-identical
copy between arms, and the pin re-read green afterwards with all five files
`sha256`-identical to their pre-run state.

| mutant | killed by |
|---|---|
| `bsLive.in`'s account reverted to the trimmed output | arm 1 |
| `srLive.in`'s account reverted to the trimmed output | arm 1 |
| one account message defers to line 2 again | arm 2 |
| the claude seat's error blanked in its goroutine again | arm 1 |
| the wall check's `member_pattern` line renamed | the pattern read (all three tree arms) |
| the waiter set emptied | arm 1's control + arm 3 |
| the seat-launch key no longer names `bwrap` | arm 1 and arm 2 controls + arm 3 |
| arm 2's one hop binds nothing | arm 2's control + arm 3 |
| the format reader stops reading `+` concatenations | arm 3 |

The last four are the controls, and they are there because this is an absence
pin: a census that found no member files, no seat launches, or no accounts
would pass having read nothing. Both directions are the bug, so each arm
carries a floor with its 2026-11-21 reading written beside it.

## The instrument paid off inside this session, and the number is 137

`ranger-base-axt1f`'s fragment says of its red: "not reproducible, and its
cause is not recoverable from any record this shop keeps." While verifying
THIS bead, the same member red again in the same venue — and this time the
one line that reaches a bead carried its own cause:

    FAIL TestLiveBwrapGitRefsCommitsAfterAnOutsidePackRefs:
    bwrapgitrefslive_test.go:522: the seat's commit failed after an outside
    pack-refs --all --prune: exit status 137, and the seat said NOTHING — so
    that status is the whole account: no error from the command and no bwrap
    refusal, which is the shape of a seat ended from outside rather than one
    the wall turned away

`137` is `128 + 9`: the seat was **SIGKILLed from outside**. That is the
discrimination ranger-base-axt1f named as the whole point of the fix and
could not demonstrate, because its own red predated the fix. Compare what the
same failure said before: `the seat's commit failed after an outside
pack-refs --all --prune:` and nothing. One of those sends the next reader to
the box; the other sends them nowhere.

MEASURED 2026-11-21 (linux 7.2.15, bwrap 0.13.1, go1.27.3, 16 cores):

- the red: one `venue+check` run, `go test` exited 1 in 127s, 1 of 48
  members. A sibling seat (`gwart-…-aurb8`) took a suite slot during that
  run and the one-minute load went from 3.62 to 12.40.
- the immediately following `venue+check`: rc=0 in 122s at load 5.68,
  `wall check: green, 48 pins measured`.
- `^TestLiveBwrapGitRefs` at `-count=100` — 700 executions of the seven
  git-refs pins, T4 and its control among them: `ok` in 9.1s, 0 FAIL.
- not memory: no OOM line in `dmesg`, 19G available.
- not the load guard and not a reap of this session:
  `~/homes/ranger/state/dispatch-watch.log` holds only
  `reaped <session> (bead … closed)` rows for OTHER seats in the window, and
  `$TMPDIR/posse-test-signal.log` is hours stale, so nothing signalled a
  suite wrapper here.

So it is still not reproducible by repetition, and this bead does not claim to
have closed it. What is new is that the report now names which KIND of failure
it was, and it points at the box under concurrent load rather than at this
render. Filed as its own bead, because a seat killed from outside while the
wall is the subject is a defect with a condition attached and not a note.

## What this census does NOT reach, and why that is not a gap to widen

The key is a bwrap seat launch plus an **error value**. The wall-watch
members (`wallwatchlive_test.go`, `wallwatchtty_linux_test.go`,
`wallwatchbackstopwiring_linux_test.go`) start a seat through
`runWallWatch`, which returns an `int` exit code and no error at all — so
there is no error to destroy, and the status is already a measured value
their messages carry on the first line (`exit = %d, want 1`). Checked, not
assumed: none of those files holds a function this census's launch key
matches, and none holds a seat account.

`ranger-base-w6kke` is the remaining instance in that half and it stays open.
It is a different defect from either of the two this census reads:
`wwWaitKilled` does not TAKE the watcher's words, so an exit of 2 — a refusal
whose words are in the fixture's buffer — cannot be rendered by any message
however it is formatted. That is a helper signature and a second file, which
is why ranger-base-a688i filed it rather than fixing it, and why widening
this key to cover it would not have found it.

Widening the key to "any child whose failure is the subject" is what puts the
204-message reading back, so it is not done.
