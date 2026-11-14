# Nine live pins nested a bwrap, and two of them said the box had no bwrap (ranger-base-pvkhn)

ADR 0096 D2-D4 implemented. 2026-11-14, D1 host (linux 7.2.15 x86_64, 16
cores), bubblewrap 0.13.1, go1.27.3, git 2.56.0, branched at f91d4ba0.

## 1. What the class turned out to be

ADR 0096 D3 names three pins. A census of `exec.Command("bwrap"` and
`exec.CommandContext(…, "bwrap")` over `internal/` found **eleven test files
that exec a bwrap**, in two states, and no file outside `internal/posse`:

- **Four gates asked GOOS and `LookPath("bwrap")` and nothing else**, so
  inside a caged seat the pin FAILED: `bgLiveSkip`
  (`bwrapgitrefslive_test.go`, 6 arms), `bhLiveFixture`
  (`bwraphide_qa_test.go`, 3), `btLive` (`bwraptmp_qa_test.go`, 2),
  `wwLiveFixture` (`wallwatchlive_test.go`, 3) and
  `TestLiveBwrapPushDeniedSeatReadsNoConfiguredSSHKey`
  (`sshidentity_qa_test.go`). `wwLiveFixture` is the one the ADR's "three
  pins" did not name: its seats are real bwraps started by `runWallWatch`
  from the test process, so the exec is one level out and a grep for
  `exec.Command("bwrap"` in that file finds nothing.
- **Seven asked `liveBwrapApplyRefusal()` third**, so they skipped — saying
  `NOTHING MEASURED: bwrap: setting up uid map: Read-only file system`.
  That is the AVAILABILITY sentence (`AvailableCages`' "does this host
  provide the tier"), and read on its own it says this box has no working
  bwrap, which is false and reads to a tidying bead as a host to fix.

So the split the ADR cares about was not three-versus-the-rest. It was
"fails" versus "abstains for the wrong reason", and the second class was the
larger one.

## 2. The gate

`bwrapNestSkip` in `internal/posse/wall_helpers_test.go` — untagged, so all
three arms compile it, beside the other helpers for tests whose subject is a
launch behind the L2 wall. One `sync.Once` of its own, `bwrapApplyProbeArgv`
for D17's base mount so that argv keeps one spelling, and a **bare `bwrap`
on the ambient PATH** rather than `protocolCmd`'s gates-free resolution:
every pin it gates types `exec.Command("bwrap", …)` and gates on
`exec.LookPath("bwrap")`, so a probe resolving the other PATH could be green
in front of a red exec.

MEASURED, same binary, `-test.run TestLive` over arm 1, once uncaged and
once inside D17's base plus `--unshare-user` with `/tmp` and `/var/tmp`
bound rw (the only two paths the fixtures write):

| run | PASS | SKIP | FAIL | skips carrying the D3 sentence |
|---|---|---|---|---|
| uncaged (the control) | 35 | 6 | 0 | 0 |
| inside D17's base | 1 | 40 | 0 | 31 |

All three pins the ADR names PASS uncaged and skip inside the wall with

```
NOTHING MEASURED: this process is inside a wall that forbids nesting (ADR 0096 D1): bwrap: setting up uid map: Read-only file system
```

The one PASS inside the wall is
`TestLiveGrokLineStartsNoCompatMCPOrHook`, which execs `sh -c` and no bwrap:
it is live about grok's leader and not about the wall, so it is outside this
class and correctly not gated. The nine skips that carry a different reason
are docker, `RHQ_LIVE_RUNTIME`, sandbox-exec-only, and three that read the
availability probe through SHIPPED code (`reachability.go`, `wall.go`,
`landedcheck.go`) — which is D3's point: those three are the question
`AvailableCages` asks, and they keep their own sentence.

**The venue is the residual.** On this fleet every dispatched seat is caged
and the landed-check judge does not run the arms (ADR 0088 D6), so after
this change the class measures on an uncaged operator run and nowhere else.
ADR 0096's Consequences files that as ranger-base-m2ii4; nothing here solves
it.

## 3. The seat this was measured from was not caged

The bead says "MEASURE this from a caged seat; this bead's own dispatch is
one". It is not: `RHQ_CAGE=shims`, `posse gates dinesh` prints "at cage
shims", `$HOME` is writable from the seat and `bwrap --ro-bind / /
/bin/true` starts. So the caged column above is a test BINARY run inside
D17's base by hand, which is the same process state the gate reads — a
process under `--ro-bind / /` with the host's procfs — and not a posse
launch. What it does not exercise is the launcher, the prep or
`RenderBwrap`. The pins' own gates are what this bead changes, and they run
in the test process.

## 4. `--bind /proc /proc` is the same mount, so the never-list is a pair

`TestBwrapArgsNeverUnsharesWhatTheSeatNeeds` gained an arm over any of
bwrap 0.13.1's ten bind spellings whose DESTINATION is `/proc` — the
destination, because that is what the seat then reads. ADR 0096's table
measured arms B and D as identical mountinfo lines on the same device
(`0:24`), so a never-list naming only the flag would pass the rejected mount
spelled the other way, and the `--bind` spelling is the one a later bead
would reach for (bwrap's fallback to a host bind is a build behaviour — the
same reason ranger-base-racfc spells `--unshare-user` explicitly).

MEASURED, two mutants planted in `bwrapArgsInto`'s base argv, each restored:
`--proc /proc` reds the flag arm (`bwrapargs_qa_test.go:172`) and
`--bind /proc /proc` reds the new pair arm (`:199`). Neither spelling was
caught before, and the pin is green restored.

## 5. T-nest, and why the probe could not be run whole from here

`docs/adr/0080-bwrap-tier.probe.sh` gained the row, EXPECT `refused`, with
bwrap's stderr on a detail line under it rather than in the cell (the B5
lesson, ranger-base-yca4g: `refused` is the claim about whether, the
kernel's words are the claim about why).

MEASURED 2026-11-14, uncaged, the row's own three lines behind the probe's
preamble:

```
T-nest bwrap inside the D17 base                   refused    refused
   T-nest detail (not a row): exit=1 stderr=bwrap: setting up uid map: Read-only file system
```

The same cell and the same words with `--unshare-user` added, so the row
reads the shipped wall although `$BASE` leaves that flag to the bwrap build.

The probe could not be run end to end from this seat: T2 types
`git … commit -q --allow-empty -m root`, every crew PID denies
`Bash(git commit unless --)`, and the shim refused it — so T2 onwards
(T-nest among them) never executed under `sh docs/adr/…`. The probe's header
says to run it as the posse user, which the operator is; a crew seat cannot.
T-nest depends on nothing T1-T5 builds, which is what made the row
measurable on its own.
