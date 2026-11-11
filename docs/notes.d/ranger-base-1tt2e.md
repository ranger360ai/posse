## Five pins that could not fail for their own subject (ranger-base-1tt2e)

2026-11-10 — the code half of ranger-base-s3a9g's six live findings, built on
tree fc4f9ecb (Linux 7.2.15 x86_64, go1.27.3). The findings were MEASURED by
holden at 765d6f75 and are re-measured here, at the tree they land on, the
same way: plant the exact wrong code, run the pin named for it, restore,
check the file's md5 against the reading taken before the plant.

Five of the six are test-only. The sixth finding on that bead — the landing
that nobody compiles (`MergeSessionWork`) — is NOT here: it needs a gate
inside the launcher lock and a surface to configure it, which is a design
call and not a defect to patch, so it went to the architecture lane as
ranger-base-qk7w6.

### What each pin could not see, and what it sees now

| pin | could not fail for | the plant that was green | fixed by |
|---|---|---|---|
| `TestQAAHerdrThatFailedDoesNotFallBackToTheCapture` | a herdr that FAILED, which is the arm's whole name | `panework.go` PaneHolding → a bare `return b.capturedHold(target)` | the error is ARMED (`explain-error` in the fake's dir), not provoked with an unknown pane |
| `TestQAACaptureComposerIsBetweenTheLastTwoRules` | the LAST two rules, as against the first two | `capturedcomposer.go` → `if isFooterRule(l) && topRule < 0 {` | a case with THREE rules and the box between the last two |
| `TestQAARexAgentSessionRefusesARuntimeThatIsNotClaude` | the runtime-name guard | `runtimesession.go` → `if false {` in place of the name guard | the row carries `ownProcStart(t)`, so the procStart fence cannot refuse first |
| `TestQAARuntimeOwnerAnswersExactlyWhenTheReadingIsSeen` | one of the three §3 causes its own doc comment enumerates | `muxrex.go` → `return RexOwner{}, nil` for a record with no owner | a fifth row with a nil Owner (`rexSeedOwnedBy` takes a pointer now) |
| `TestQAAWrappedBoxEchoesTheLineThatWasSubmitted` | `collapseScreenWrap`, the one thing this route's comparison does that herdr's does not | `capturedcomposer.go` → `return s` | a case where the two sides differ only in whitespace runs |
| `TestQAAClaudeSessionRowIsFencedAgainstPidReuse` | the `start == ""` disjunct of the pid-reuse fence | `runtimesession.go` → `if row.ProcStart != start {` | a row with no procStart read against a kernel reading nobody could take |

### Why each one was green, which is the part worth keeping

Three of the five were green for the same reason: **the fixture reached the
guard under test through a cheaper refusal, and the cheaper refusal's answer
was indistinguishable from the right one.**

- The fake herdr's `agent explain` has an idle fallback and so explains ANY
  pane. `Seat{Pane: "no-such-pane:0"}` therefore exercised the SUCCESS path,
  and the arm's assertion — no regions, not waiting — is what a success over
  a screen with no regions answers too. The sibling arm in
  `confirmsubmitted_qa_test.go:86` had already met this and armed the error
  instead; its comment says why. The arm that was named for a herdr FAILURE
  was the one arm in the family that never saw one.
- The codex row carried `"procStart": "1"` against the live kernel value for
  `os.Getpid()`, so `claudeSessionIn`'s fence refused before the name guard
  was reached — and the fence's own message contains the word `claude`, which
  is what the arm's second assertion checks for. Two assertions, both
  satisfied by the wrong guard.
- `captureComposer` rejoins wrapped lines with SINGLE spaces, so by the time
  a reading reaches `collapseScreenWrap` there is nothing left to collapse
  unless the SUBMITTED side carries a run. No fixture's did.

The other two are plain absences: a table with four rows for three causes,
and eight cases that each plant exactly two rules.

### The measurement

Each plant is unique in its file (the mutator refuses a target that is not),
and the pin named for it was run alone. Restores checked by md5 against the
reading taken before any plant:

```
e910a9ec8ce6e465608980e64e8b3723  internal/posse/panework.go
e0e3c28c70d1466b516127c9932793b3  internal/posse/capturedcomposer.go
a5ff850fc3dfebec9c345e8c15c0cf04  internal/posse/runtimesession.go
0c92464d0f856b291d4ca31508347530  internal/posse/muxrex.go
```

| plant | the pin's verdict |
|---|---|
| `return b.capturedHold(target)` | **FAIL** — `a herdr explain that failed produced a reading ({Work:1 shell …})` |
| `isFooterRule(l) && topRule < 0` | **FAIL** — `three rules: the composer is between the LAST two: captureComposer = "", want "this prompt is sitting unsent in the box"` |
| `if false {` for the name guard | **FAIL** — `paneAgentSession answered "0226278e…" for a block held by codex` |
| `return RexOwner{}, nil` | **FAIL** — `State().Seen = false but RuntimeOwner err = <nil>` |
| `return s` for the collapse | **FAIL** — `capturedEcho("commit  the  bead and  close it", "commit  the  bead \t and  close it") = false` |
| `if row.ProcStart != start {` | **FAIL** — `claudeSessionIn answered "0226278e…" where it must refuse` |

Every one of the six reds the pin it was written against and nothing else was
touched to make it. The two directions each defect would have gone if it ever
regressed are in the arms' own comments; the one that is defensive rather
than live today — the empty `start` — says so in its case comment, and is
pinned because the whole echo reading rests on that fence.

### One shape, for whoever writes the next pin in this family

A QA arm whose fixture reaches the guard it is named for **through another
guard** is an arm that passes on the wrong refusal, and nothing in a green
suite says so. The family's own files already carried the antidote twice —
`confirmsubmitted_qa_test.go`'s armed `explain-error`, and
`runtimesession_qa_test.go`'s `ownProcStart(t)` — and the arms that went
without them were written beside the ones that used them. The cheap check is
not "does this arm pass" but "name the line that must break for it to fail,
and break it".
