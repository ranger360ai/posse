# The launch-line tests red on the git-global snapshot, not on the line (ranger-base-nycra)

Three tests in `internal/posse` went red together, intermittently, naming a
rendered claude launch line and the whole `--settings` payload beside it:
`TestQAFleetLaunchLineDisablesAutoMemory`, `TestLegacyAgentUnchanged` and
`TestFleetSettingsSurviveRendering`. Nothing was wrong with the line. The only
differing token in two 1.4 KB payloads was `GIT_CONFIG_GLOBAL`, and a test
binary's own `TestMain` was what moved it.

Found on ranger-base-hgcmw and deliberately not fixed there (strict scope: a
half-fix would have left the class unfiled). Filed from it, fixed here.

## The mechanism

The `GIT_CONFIG_GLOBAL` row obdnk pins is not a constant. `gitGlobalPinValue`
(gitglobalscope.go) names the read-only snapshot of this HOME's resolved global
file when one is on disk and verifies, and the operator's own file when none is
— ranger-base-161t2's rule, so a seat can never write the file it reads as
global scope. A LAUNCH prepares that snapshot (`PrepareGitGlobalSnapshot`); a
READER never writes one (ranger-base-qd9ei). So the row every reader of the
settings pin renders — `posse gates`, the parity report, a status line, every
launch-line test in the package — is a **fact about disk state**, not a pure
function of the code.

`TestMain` gives the whole test binary ONE temp HOME (`/tmp/posse-testhome-<n>`,
ranger-base-32bdv). Two tests in that binary are `t.Parallel()` AND call
`PrepareGitGlobalSnapshot()` under that shared HOME:

- `TestQATheInletPinCoversTheGitConfigFamilyOrSaysWhyNot` (inletpingitconfig_qa_test.go)
- `TestQAInletPinCarriesEveryMeasuredNameAtItsMeasuredValue` (inletpin_qa_test.go)

Each flips, mid-parallel-batch and for every sibling still running, what the row
reads:

```
<temphome>/.gitconfig
  →  <temphome>/.cache/posse-wall/git-global/e3b0c44298fc1c149afb/read-only-snapshot.gitconfig
```

(`e3b0c44298fc1c149afb` is the sha of the empty string: the temp HOME has no
`.gitconfig`, and a missing global file snapshots as empty bytes.)

The three victims each compare a RENDERED launch line against a payload
computed by a SECOND call — `ClaudeFleetSettingsJSON(...)` or
`credentialDirPinJSON()` — so the pin is read twice, and the two reads can
straddle the flip. Which read came first is why `got` and `want` swap roles
between tests in a single run: `TestLegacyAgentUnchanged` computes `want`
before it renders, `TestFleetSettingsSurviveRendering` after.

The tests the same bead ADDED are immune, and that is not luck: they parse the
payload off the rendered line (`claudePayloadOf`, memorygrant_qa_test.go) and
assert on THAT, so they read the pin once.

Why the serial tests are not the flippers: every other caller of
`PrepareGitGlobalSnapshot` in this package arrives through
`globalScopeFixture`, which does `t.Setenv("HOME", t.TempDir())` — so it is
ineligible for `t.Parallel`, runs in the serial phase that Go completes before
it releases any parallel test, and writes under a home of its own. The two
above are the only parallel writers of shared disk state here.

## The cost of the shape, not of the bug

A red here prints the launch line and 1.4 KB of payload and says "legacy
rendering changed" or "lost fleet settings". It reads as a launch-line defect,
so it sends the next reader to the template and the renderer. It cost two full
arm-3 `make test` runs on ranger-base-hgcmw before anyone diffed the two
payloads token by token. And a red in `internal/posse` fails the whole package,
so `make test` exits 2 for every seat on the box.

## MEASURED

2026-11-21, this box (linux/amd64, 16 cores), one `go test -c -tags posse_arm3`
binary run directly, `-test.count=1 -test.parallel=8`, loadavg 6-7:

| arms | red |
|---|---|
| the three victims + the two parallel preparers, before the fix | **2/20** |
| the three victims alone (no preparer in the filter) | 0/20 |
| the three victims + the two preparers + the new pin, after the fix | **0/30** |

2026-11-20 (ranger-base-hgcmw, over a wider filter): 2/5 then 1/5 runs red with
the default `-parallel`, 0/5 at `-parallel 1`. It also fired once inside a full
`make test` arm 3 — one test of the three, not all — so it was never only a
filtered-run artifact.

## The fix

`TestMain` prepares the snapshot once, before any test runs, so the binary
starts in the state a launch leaves behind. That is a four-line change plus its
reasoning. The two parallel prepares then write bytes that are already there,
`gitGlobalSnapshot` skips its rewrite, and the row cannot move.

Two shapes were rejected:

- **Caching the pin read in production.** The snapshot path is
  content-addressed on the global file's bytes, so a cache would keep naming a
  snapshot of a gitconfig the operator has since edited — and would break both
  self-heals the path has (a content tamper's, and ranger-base-r9slg's
  mode-only tamper's, each of which is "the next launch over it heals it"). The
  read is honestly disk-dependent; it is the TEST that was in a state no launch
  is ever in.
- **Rewriting the three victims to a single read.** It would work, and for
  `TestQAFleetLaunchLineDisablesAutoMemory` it is nearly free. But
  `TestLegacyAgentUnchanged`'s subject IS its byte-exact want ("nothing else was
  added to a pre-PID render"), and `TestFleetSettingsSurviveRendering`'s is the
  payload reaching the line verbatim, quotes and braces intact. Parsing the
  payload off the line to compare it weakens exactly the two pins whose strength
  is that they do not.

The fallback is stable too, which is why the fix is not fatal when it fails: a
box whose global file carries an `[include]` section, or which has two global
files, renders the fallback for EVERY read. `TestMain` prints one line naming
the bead so a reader of a degraded row knows why, and carries on.

## The pin

`TestTheTempHomesGitGlobalRowDoesNotMoveUnderAParallelLaunch`
(testhome_qa_test.go, untagged so it is built in all three arms). Its subject is
**stability, not preparedness** — it reads the row, does what a parallel sibling
does, and reads it again — which is what makes it portable to a box that cannot
snapshot at all, where both reads are the fallback and the pin is still true.
A second arm asserts two reads of the whole payload agree, so the next row that
starts moving reds where the mechanism is rather than inside a launch-line pin
whose message names the template.

It carries no `t.Parallel()` on purpose, and that is the whole of what makes it
deterministic rather than a second flake: Go releases a package's parallel tests
only once every serial test has run, so in the serial phase the only thing that
can have prepared this HOME's snapshot is `TestMain`. It is registered in
`cmd/testparallel/main.go`'s serial map with that reason, which is the gate that
asks.

MUTATION RUN: with `PrepareGitGlobalSnapshot()` taken out of `TestMain`, the pin
reds 3/3 and prints both paths.

## Residual

A future parallel test that writes shared state under the temp HOME reopens the
class for whatever reads it, and nothing enumerates that. The comment on
`TestMain`'s HOME paragraph used to claim "nothing under it is shared state a
test can see"; it now names the two fixtures that are, and says that anything
else a test leaves there is state its siblings can read. Not filed: there is no
mechanism to propose yet, and the general form ("no parallel test may write
under $HOME") has no spelling a static filter can check.
