# The judge takes the seat's socket wall; the ADR 0097 venue explicitly does not (ranger-base-i92tj)

ADR 0088 D3.5-D3.7 implemented, and ADR 0097 D1 amended. 2026-11-14, D1 host
(linux 7.2.15-1-omarchy x86_64, 16 cores), bubblewrap 0.13.1, Landlock ABI 10,
go1.27.3, branched at b5a6f5c3.

The bead is `vuln-private`; this fragment describes what the change SHUTS,
which AGENTS.md "Landing the plane" allows, and names no residual.

## 1. The four cages, measured

One nested `bwrap --ro-bind / / /bin/true` — which is `bwrapApplyProbeArgv`,
the thing the nest helper execs — inside each of four cages. The outer bwrap
is D17's base (`--die-with-parent --ro-bind / / --dev-bind /dev /dev
--unshare-user --unshare-net`); "step" means the socket wall is on the line
after bwrap's own `--`.

| cage | step | nested bwrap |
|---|---|---|
| D17 base, no `--proc` — a seat, and the judge | no | `bwrap: setting up uid map: Read-only file system`, rc=1 |
| the same | **yes** | the same, byte for byte, rc=1 |
| base + `--unshare-pid --proc /proc` — the venue | no | **nests, rc=0** |
| the venue's shape | **yes** | `bwrap: Failed to make / slave: Operation not permitted`, rc=1 |

And the step alone, uncaged: `posse wall-unix -- bwrap --ro-bind / /
/bin/true` → `Failed to make / slave`, rc=1. The control, uncaged with no
step → rc=0. The step is transparent to everything else: `posse wall-unix --
/bin/sh -c 'echo …; exit 7'` through D17's base printed its line and exited
7.

**Rows 1 and 2 are the answer to the bead's item 5**, and the answer is
better than the bead expected. The refusal the nest helper sees in the judge
does not change at all — not just its meaning, its WORDS. D17's render leaves
`--proc` out on purpose, so `/proc` inside a seat or a judge is the host's
under `--ro-bind / /`, and bwrap dies writing its own `uid_map` long before
it reaches a mount. The Landlock refusal exists only where the step meets a
WRITABLE `/proc`, which is row 4 and which is the one combination ADR 0097 D1
now forbids. Either way `bwrapNestDecision` abstains outside the venue: it is
a function of whether a bwrap started and never of why not, which is the
property and not an accident of these strings — a gate that read the kernel's
wording would turn a new refusal into a red on every caged seat the day a
layer changed.

That also made one sentence in `liveBwrapNestRefusal`'s doc comment wrong,
and it is corrected: it said that once the Landlock layer landed "the kernel
refuses the mount instead, and the two questions answer differently for
good". Rows 1 and 2 say otherwise.

**Row 4 is the trap, priced.** It is what the venue would have done had the
step ridden in the shared argv: the baseline refuses, so under ADR 0097 D4
every one of the 32 members FAILs and D3's body answers exit 3, at every
landing.

D1's subtraction pin is two different facts about this, and both are MEASURED:

- **On the trap it would have stayed GREEN.** The step appears in both argvs,
  so the multiset difference is empty in both directions, and the pin reads
  the two argvs and never what runs in them. That is why the "no `wall-unix`
  verb in the venue" row exists.
- **On the step typed CORRECTLY it went RED**, because its removal half read
  "nothing removed" — the first run of the amended code reddened it with
  `the venue's argv DROPS [-- <self> wall-unix]`. So the amendment is to D1's
  WORDS and not only to the code: a pin that reds on the right change is a pin
  somebody will delete.

## 2. What the render does now

`boxLandedCage`'s bwrap arm, `kind == landedCageJudge` only:

```
bwrap <args> --unshare-net --setenv … --ro-bind <self> <self> \
  -- <self> wall-unix -- <judge>
```

- `self` is `wallWatchBin()` — the LAUNCHER's binary, the same resolution the
  wall-watch sidecar uses — and never a path out of the tree under judgement.
  Landing reviews nothing (ADR 0088 D3), so a step resolved from that tree
  would let the code being judged pick its own wall.
- The `--ro-bind <self> <self>` is load-bearing and was found by a pin, not by
  reasoning: `TestLiveLandedCheckCageReadsNoSecret` execs the cage it renders,
  the test binary standing in for the launcher lives under `/tmp`, and
  `--bind <run>/tmp /tmp` put it out of reach — `bwrap: execvp
  …/posse.test: No such file or directory`, exit 1, a judge that never ran.
  The same would be true of a launcher under `/run` or under a runtime's state
  root (`landedJudgeHides`). It goes after every mask so nothing above can
  take it away, and it adds no read surface: `--ro-bind / /` already gives the
  judge every byte of the host no mask hid.
- `internal/posse`'s `TestMain` gained an `IsWallUnix` arm beside its
  `IsWallWatch` and `IsCageLaunch` ones, for the same reason those exist: in
  this package `wallWatchBin()` resolves to the test binary, so without it the
  step reached the `RHQ_FAKE_HERDR` arm and answered `fake bd: unhandled
  wall-unix -- …`. Keyed on argv, so it cannot depend on an environment the
  judge's own allowlist strips.

Below ABI 9 the step is not rendered (ADR 0080 D31) and all three verdict
lines carry `WallUnixDegraded`'s sentence — green, red, and the one that says
there is no verdict. The judge has no persona and so no `posse gates` line of
its own; those three lines are the only place its wall is ever reported.

## 3. The mutants, one change at a time

Driven over `TestQALandedCheckJudgeIsNoStrongerThanASeat`,
`TestQALandedCheckVerdictSaysWhenThereIsNoSocketWall`,
`TestQAWallVenueArgvIsTheJudgesPlusADR0097sThree`,
`TestQAWallVenueGateRuleIsADR0097D4s` and
`TestLiveLandedCheckCageReadsNoSecret`.

| mutant | result |
|---|---|
| the judge takes no step (`if false && …`) | RED: the tail row, and the venue's subtraction |
| the step rides into the venue (the `kind` guard dropped) | RED: the subtraction, and the "no `wall-unix` verb" row |
| the step typed in FRONT of bwrap in the venue | RED: both of the same two rows, the second naming `argv[1]` |
| the bind-back dropped | RED: the live cage pin (`execvp`), and the tail row |
| the degraded sentence returns `""` | RED: 12 subtests, over all three verdicts × four ABI values |
| `self` taken from the tree under judgement | RED: the live cage pin, and the tail row |

**The degraded-sentence mutant survived the first time**, and that is the
entry worth keeping. The arm in
`TestQALandedCheckJudgeIsNoStrongerThanASeat` that reads it is inside an
`else` taken only on a host below ABI 9, so on this fleet it is unreachable —
the sentence was DECIDED and never checked for EMISSION, which is
ranger-base-ig79v's finding 2(b) one mechanism over.
`TestQALandedCheckVerdictSaysWhenThereIsNoSocketWall` is what closes it:
`wallUnixABI` is a var, so the host answer is a parameter, and the pin reads
the RUNNER's own output buffer through `landedChecker.once` rather than the
helper's return.

## 4. Doors

No new Makefile door. Every pin here is a `-run`-reachable test in arm 1 of
`internal/posse` that reads no tree, so it is not in the class AGENTS.md's
"A `-run` filter cannot reach a tree-wide pin" describes.
