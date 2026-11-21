# ADR 0059 D2's thirteen arms are green on darwin in their post-5blmt spelling (ranger-base-m8msi)

2026-11-21, read off GitHub Actions `ci` from this bead's linux worktree; no
darwin box was touched. From ranger-base-blkse, which fixed the `t.Fatalf`
standing in front of these arms. The arms skip wherever there is no
`sandbox-exec`, so a macos CI leg is their only venue and READING one is the
whole of the work.

## the legs

MEASURED, macos-latest = image `macos-26-arm64` 20261117.0516.1, Darwin
25.6.0 arm64, git 2.56.1, go1.26.10, `GOTOOLCHAIN=local`:

| run | sha | arm 2 | arm 3 |
|---|---|---|---|
| 44921131540 | 22690b7a (blkse's own landing) | green | green |
| 44926213762 | 56854819 | green | green |
| 44927635225 | 6557d59b | green | green |

Green here means `TestQAWorktreeCommitLeavesNoStrayPackedRefsLock` and its
thirteen subtests — D2.1 `wgSeqArms` (6), D2.3 `wgRunPackedRefRewrite` (1),
D2.4 `wgAliasArms` (6) — plus blkse's new
`TestQAWgPackPrunesTheRefDirectoryAfterASeatStoreCommit`, in all six jobs.
The file carries no build tag, so it is in all three arms; macos arm 1 never
reaches it, dying in `tree-check` before the Go test runs.

## reading a green off a leg that prints nothing

`ci.yml` runs `make test-arm<N>`, which runs `go test` with no `-v`, so a
PASS and a SKIP print identically: nothing. Absence from the `--- FAIL:`
list is therefore two readings, not one, and the bead's question — "have
these arms RUN on darwin" — is the one absence does not answer. Two facts
close it, and a later reader of any CI leg can use the same pair:

1. **The arms cannot pass vacuously.** Each carries its own control and
   fails when the control measures nothing: `wgRunSeqArm` `t.Fatalf`s when
   the prep left no sequencer state and `t.Errorf`s when the control never
   asked for `packed-refs.lock` or left no blocker; `wgRunPackedRefRewrite`
   requires the refusal to land at `packed-refs.new`. A green is a measured
   green or a red, never a shrug.
2. **A positive witness that the skip gate did not fire.** The only skip in
   the battery is `sbSkipUnlessSandboxable`, which is `SeatbeltAvailable()`
   (darwin plus `exec.LookPath`) and `sandboxApplyRefusal()` — and the
   second is a `sync.Once`, cached for the test binary's whole life
   (`internal/posse/seatbelt.go`). So any one test in the same process that
   ran past that gate licenses every other. In all six jobs above
   `TestQACarveOutRefusesUnderSandboxExecAndTheControlDoesNot` — whose first
   statement is that same gate — is in the FAIL list. A FAIL is a run.
   The battery ran.

## what the same legs also say, and does not belong to this bead

The macos legs are red for other reasons, and the reds divide by window:

- `TestQAMirrorIsAFourthForcePushSpellingThatOnlyTheVerbRuleCloses`, arm 3.
  Green at edfe7a79 (run 43731869157, 2026-11-15), red from 9193d3b1 (run
  44856440140) onward. The rendered gate shim does not PARSE under darwin's
  `/bin/sh`: `line 403: syntax error near unexpected token ';;'`, quoting the
  `-C)` line of `renderSeatObjectEnv`. Filed as ranger-base-i36bf with the
  construct and the two candidate fixes.
- `TestQACarveOutRefusesUnderSandboxExecAndTheControlDoesNot`, arms 2 and 3.
  Green at 9193d3b1, red by 22690b7a; thirteen landings in that window, not
  bisected here. Its `the record stage: the store's git` arm reads
  `REFUSED under the carve-out, want ALLOWED` and its control reads
  `the CONTROL refused it too`, which is the shape a shim that refuses
  everything makes — so the shim above is the first thing to rule out, not a
  conclusion.
- Already red at edfe7a79, so older than any of this work:
  `TestQAKernelGrantsOwnInstanceStateAndRefusesTheOthers` and
  `TestSeatbeltProfileAndLaunch` (arm 3), and the arm-2 wall-check family
  (`TestEveryVerdictLineNamesTheLauncherThatRanTheCheck` and six siblings),
  whose verdict lines say `no cage renders on this host (fixture)`.

Filed as ranger-base-jyqk4. Both filings are reads of a public CI log on a
public repo and name no hole that is not already in the log.

## the one ASSUMED this retires

ADR 0059's `MEASURED vs ASSUMED` section assumed "that git 2.55 (the
runners) keeps the tempfile shape for `packed-refs.new`". D2.3 is green on
the runners at **git 2.56.1**, and its assertion is that the refusal lands
at `packed-refs.new` by name — so the shape is measured there now, one minor
version past the one the ADR guessed about.
