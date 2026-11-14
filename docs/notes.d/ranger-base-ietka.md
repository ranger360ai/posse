# A closed-set pin whose failure message crashed on the one direction it exists for (ranger-base-ietka)

`internal/posse/remedyclosedset_test.go` was written by ranger-base-mxbq0
because "the ADD-a-promise direction has no reader at all": a clause
appended to a refusal's remedy sentence is a promise a refused writer
performs, and an added promise displaces no phrase that a
`remedyPromises`-style pin greps for, so only a pin closed over the WHOLE
text can see one. The reader landed. Its failure MESSAGE — the half that
tells the author to go measure the clause they just wrote — panicked on
exactly that direction.

2026-11-14, this box (linux 7.2.15 x86_64, go1.27.3, git 2.56.0), branched
at e208dba9.

## The defect

```go
i := remedyDiffAt(a, b)
for i > 0 && !utf8.RuneStart(a[i]) {
```

`remedyDiffAt` answers `min(len(a), len(b))` when one text is a prefix of
the other — correct, and the whole of it. The message calls
`remedyTail(got, want)` and `remedyTail(want, got)`, so an APPENDED clause
makes `want` a prefix of `got` and the second call indexes
`want[len(want)]`:

    panic: runtime error: index out of range [316] with length 316
    remedyclosedset_test.go:126 remedyTail
    remedyclosedset_test.go:101 remedyIsClosed
    TestQARemedySentencesAreClosedSets

A drop and an in-place edit report fine; only the append crashes. And the
panic takes the package BINARY down, so every other test in the run is lost
with it — a reader meant to say "go to the rows" said nothing and reddened
everything.

316 is the whole of `VulnPrivateWayThrough` as it stood at `e3b41b30^`,
which dates the crash: the append that found it was ranger-base-ed07e's
own, the exemption clause that sentence now ends with. The first edit of a
remedy after this file landed hit the bug, and the reward for doing what
the message asks was a stack trace.

## The fix

One bound, in the loop the panic was in:

```go
for i > 0 && i < len(a) && !utf8.RuneStart(a[i]) {
```

Load-bearing, not defensive. `len(a)` is always a rune boundary, so there is
nothing for the backup to do there; the tail is then empty, which is the
honest answer — `a` ran out, nothing of it is left to print — and the
message already carries both whole texts above the zoom.

## The door

`TestQARemedyClosedSetFailureReportsEveryDirectionOfDrift`, over all four
directions a remedy can drift: appended, dropped, edited in place, and a
divergence INSIDE a multibyte rune (the other half of the same loop — every
one of these sentences carries an em dash, so a byte clip would print half
a rune).

It asserts the MESSAGE and not just the helper, which is why
`remedyClosedSetFailure` is now a function returning a string rather than an
inline `t.Errorf`: there is no way to read a `t.Errorf` back, so the part of
a pin that a human actually reads had no reader of its own. That is this
file's own subject one level down.

Re-measured end to end with the clause appended again through `go test
-overlay`, the tracked tree unmodified: one named FAIL, exit 1, the diff at
byte 613 and `want: ""` — where before the bound it was a panic, exit 2, and
no test named.

## Not closed here

Nothing. Scope was one test helper; no shipped code changed.
