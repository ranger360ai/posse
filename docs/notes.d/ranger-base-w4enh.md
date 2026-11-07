# Seven wall-clock fixture leads, and the three files that crossed clocks (ranger-base-w4enh)

Read 2026-11-06, after ranger-base-0ddmu corrected this census from five
`internal/posse` leads to seven files total. The discriminator was not an age
literal: it was whether the fixture and the assertion read one clock. That
found one member which the original co-occurrence grep could not classify,
and cleared four false leads.

## The five internal/posse files

- `loadguard_test.go` is immune. Its `2h30m` strings render injected `Proc`
  values. The real shell and `ps` children exercise process discovery and
  carry no wall-time fixture into an assertion.
- `govern_test.go` is immune. Every dated premise is derived from the fixed
  `govNow`, which is also handed to the code making the decision. Its one
  `git worktree add` child only constructs a repository fixture.
- `watchhang_qa_test.go` is immune to this class. Its real children are
  bounded by process deadlines measured with `time.Since`; its dated
  dispatcher fixtures use an injected clock, including the accelerated
  watchdog clock. No child writes a wall timestamp which the parent later
  judges as an age or deadline.
- `autostart_test.go` is immune. The real child at the lead is only a dead pid
  source. The live record's `Started: time.Now()` is identity bytes which the
  hook quotes; liveness is decided by the flock, not by that timestamp's age.
- `verifybox_qa_test.go` has one member:
  `TestTheScriptWritesWhatThisReaderParses`. The shipped script writes its
  current wall timestamp in a child process, then the parent calls
  `VerifyBoxFreshness(time.Now(), ...)` and asserts the verdict is fresh. A
  sleep longer than `verify_box_max_age` would make the correctly written
  verdict stale and accuse the producer/reader join. The test now takes a
  monotonic reference before the child and skips on a wall gain over one
  minute before the parent reads freshness. All other age assertions in the
  file use fixed `vbNow()` on both sides.

## The two cmd/posse files

Both contain members.

`runtimeavail_qa_test.go` wrote a model-catalog reading with one `time.Now`,
launched `posse runtimes`, and asserted freshness, a live cooldown, and in
one row the exact text `48h00m ago`. A long sleep could expire either the
lease or cooldown, while an ordinary launch crossing a minute could red the
exact age. `seedRuntimeCatalog` now takes one reference for both timestamps
and returns the reading timestamp with its monotonic component intact. Every
clock-dependent launch runs `skipIfTheBoxSleptMidTest` before its first
assertion, and the 48-hour text uses `agedAgesOf` rather than one exact age.
The unknown-flag row is immune: parsing refuses before any catalog-time
decision.

`backupsurface_qa_test.go` dates archives by a wall timestamp in their names,
then launches the binary to classify those names against freshness and the
current clock. The fresh status row can age past its threshold, and the
future-stamp verify rows can cease to be future. `backupHomeAt` now stamps
all archives from one reference and returns it; `plantArchive` returns its
reference too. Those launches run the same wall-jump guard before asserting.
The rows with no archive timestamp are immune and unchanged.

## Verification

Focused runs passed with a seat-local Go build cache:

```
go test -run 'Test(Runtimes|BackupStatusExitsForOnBoxStalenessOnly|BackupVerifyNamesWhyItCannotPickAnArchive)' ./cmd/posse
go test -run '^TestTheScriptWritesWhatThisReaderParses$' ./internal/posse
```

The first read `ok` in 13.191s and the second in 2.544s. No system sleep was
forced; the guard's clock arithmetic is the already measured
ranger-base-gt0q9 mechanism, reimplemented locally only where package
boundaries require it.
