# ranger-base-mhl7h — a deadline branch that kills a nil Process reports nothing at all

`TestLiveWallClaudeProjectsRefusesPeerPlants`
(`internal/posse/wallclaudeprojects_qa_test.go`) ran the real claude seat as
`go func() { out, cerr = cmd.CombinedOutput(); close(done) }()` and killed it
from the 90s deadline branch of a `select`. `cmd.Process` is written by
`cmd.Start`, which `CombinedOutput` calls inside that goroutine, so the field
the deadline branch read was written by the goroutine and read by the test: if
the deadline won that race, `os.(*Process).signal` derefed nil.

What that costs is not a flaked test. A panic takes the whole test binary
down, so `scripts/posse-wall-check` gets NO member line — the run reads as a
tree that would not run rather than as one member's finding, which is the
distinction ADR 0097 D3 is about, and a worse answer than the `t.Fatalf` the
branch was about to print.

## MEASURED 2026-11-22 (linux 7.2.15-1-omarchy, go1.27.3, 16 cores)

The mutant is the bead's own: plant `0 * time.Second` for the 90s deadline, so
the branch fires every run. Both variants under
`go test ./internal/posse -count=1 -run '^TestLiveWallClaudeProjectsRefusesPeerPlants$'`:

| variant | what the run reported |
|---|---|
| before (`CombinedOutput` in the goroutine) | `panic: runtime error: invalid memory address or nil pointer dereference`, `[signal SIGSEGV … addr=0x28]`, `os.(*Process).signal(…) wallclaudeprojects_qa_test.go:360`, and no member line |
| after (`Start` here, `Wait` in the goroutine) | `wallclaudeprojects_qa_test.go:395: claude -p under the wall did not end in 90s:` — a `file.go:NN:`-shaped member line, which is what the wall check reads |

Both rows were read on `c1f4bae1`, the parent this work was written against.
The replay below moved that line to `:416` and changed its tail to
`ranger-base-8ck6d`'s `; it said %s` fold; neither is what the mutant
discriminates on, which is a `file.go:NN:`-shaped line existing at all.

So the mutant discriminates, and the member line is what survives the
deadline. Unmutated, the arm is green and the real seat still reports its own
account: `exit status 1, and the seat said "API Error: Connection refused …"`
(1.42s; the dead endpoint at `127.0.0.1:1` is the fixture's, so nothing leaves
the box and no turn is billed).

## Why neither of the two shapes the bead offered

The bead framed the fix as a choice between guarding the Kill and handing the
goroutine an `exec.CommandContext`. Both were measured against and neither was
taken.

- **`if cmd.Process != nil`** answers the panic and not the race. When the
  guard skips the Kill, the `<-done` under it blocks on a seat the goroutine
  is about to `Start` — which is the hang the deadline exists to break,
  arriving later as the 25m binary timeout. Same loss of the member line, one
  mechanism further out.
- **`exec.CommandContext` whose cancel the deadline calls** closes `done` as
  it kills, so both `select` cases are ready and Go picks one at random.
  Half the time a 90s deadline that really elapsed would route through
  `bgSeatAbstainIfOutside` and abstain `NOTHING MEASURED` over a finding —
  which the `case <-done` comment in that test already rules out in words
  ("a 90s deadline that really elapsed is a finding, not an absence of one"),
  because the criterion cannot tell this process's kill from the box's.

What was taken instead is the shape every other seat run in this package
already has — `Start` on the test's own goroutine, `Wait` in the other one, a
`strings.Builder` bound to both streams. `bgLive.packUnderALiveSeat`
(`internal/posse/bwrapgitrefslive_test.go`) is the closest: deadline branch,
`Kill`, `Wait`, `bgSeatAccount` and all. `cmd.Process` is then set before
anything can read it, there is no second writer of it, and the nil is gone
rather than guarded. One builder for both streams, interface-equal, so
`os/exec` gives the child one pipe and the streams cannot interleave —
`CombinedOutput`'s own arrangement; it is complete once `Wait` has returned,
which is what `done` says, and nothing reads it before that.

## The class is two sites, not one

MEASURED 2026-11-22 over the whole tree: every `go func` within six lines of a
`cmd.Run`/`cmd.Output`/`cmd.CombinedOutput`, read against the 31
`Process.Kill`/`Process.Signal` call sites. Every other kill site `Start`s on
its own goroutine before anything can read `cmd.Process` — the remaining
`go func` hits are string fixtures inside pins
(`internal/treepins/racearm_qa_test.go`,
`internal/treepins/execwaitdelayholes_qa_test.go`,
`internal/posse/watchoutlive_qa_test.go`) or a fork-window helper that kills
nothing (`internal/posse/execwrite_test.go`). One live second member:
`TestLiveWallClaudeProjectsANonOwnCwdRunsTheRealClaudeAndWritesNoTranscript`
in `internal/posse/wallclaudeprojectsattack_qa_test.go`, same two lines and
the same 90s deadline. It is filed as `ranger-base-eheo3`, not folded in here:
that file is another member, and under strict scope a second member's fix is
its own close. The judgement above is settled, so that bead carries the shape
rather than the choice. A class of two needs no pin. It landed first, as
`ab20ad09`, so both members of the class are on main as of this fragment.

## The replay (ranger-base-2uvui)

This work was written against `c1f4bae1` and did not fast-forward: `main` had
meanwhile taken `ranger-base-8ck6d` (`eae701cb`), whose own change is inside
this test's deadline branch. So the merge-back block was a real conflict and
one hunk wide, and it is the one this fragment's last section had predicted
would not be one.

**They do overlap, by one token.** `8ck6d` reads the seat's pane twice to
compose its one-line finding, and it reads it as `string(out)` — the
`[]byte` that `CombinedOutput` returned in the goroutine, which is exactly the
variable this work removes. The resolution keeps `8ck6d`'s block whole and
substitutes `outBuf.String()` for both reads; nothing else of either change
moved, because the rest of them really are in different places — this one is
the `Start`/`Wait` block above the `select`, that one is the format string
inside the deadline branch. The prediction was right about the blocks and
wrong about the accessor they share.

Replayed per the block's own recipe — `git merge-tree` against the original's
parent, one replay commit for the one original, carrying its author identity,
author date and subject — so `equivalentOnBase` can pair it. No rebase and no
cherry-pick, so no sequencer marker to strand.

MEASURED 2026-11-22 on the replayed tree (same box: linux 7.2.15-1-omarchy,
go1.27.3, 16 cores), the bead's own `0 * time.Second` mutant re-read because
the resolution changed the line the mutant is read on:

| variant | what the run reported |
|---|---|
| mutant, replayed tree | `wallclaudeprojects_qa_test.go:416: claude -p under the wall did not end in 90s; it said NOTHING in 90s — so it hung before its first word, and the deadline is the whole finding` — one FAIL, no panic (0.97s) |
| unmutated, replayed tree | PASS, 1.25s, and the seat still reports its own account: `exit status 1, and the seat said "API Error: Connection refused …"` |

So the two changes compose: the member line exists at all because of this
work, and it says what it says because of `8ck6d`'s. At `0s` the seat has not
written a word yet, which is why the mutant reads the silence arm rather than
a `wwOneLine` of the pane — the live 90s case reads the other one.
