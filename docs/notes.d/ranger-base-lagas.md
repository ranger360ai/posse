# The wall check's body, its three exits, and the arm that crashed on the one case it exists for (ranger-base-lagas)

ADR 0097 D3's body — `scripts/posse-wall-check` — and the two arms that were
added to `docs/adr/0097-wall-venue.probe.sh` so the body can be run in the
wall the ADR measured. This is the ONE place this bead's readings are said;
the script's header deliberately quotes none of them.

## What the body is

`go test -count=1 -json -timeout 15m -run '^Test(QA)?Live(Bwrap|Wall)'
./internal/posse`, with a `bwrap --ro-bind / / /bin/true` baseline in front of
it and a verdict read off ADR 0097 D4's marker line. Membership is counted
from the run and never from the name: the nest gate logs
`wall venue: measuring (ADR 0097)` the moment it lets a pin proceed, so a pin
that then goes dark is still a member and still turns the verdict into exit 3.
That ordering is the whole reason the ADR's `/var/tmp` row — a fixture helper
that took 18 pins dark under a green exit code — is visible at all.

Three exits, as D3's table writes them: 0 green with
`wall check: green, <n> pins measured` as the last line, 1 for a member FAIL
or a tree that does not build, 3 for a baseline that will not nest, zero
members, or a member that abstained. Exit 2 is off that table and is not
reachable from the runner: it is what a caller gets for breaking ADR 0088
D1's no-argument contract, as it is at `scripts/posse-landed-check`.

## MEASURED 2026-11-14

D1 host, Linux 7.2.15, bubblewrap 0.13.1, go1.27.3, 16 cores. The seat is at
cage `shims`, so a bwrap starts from it and the control arm could be run from
the seat directly, as ranger-base-q749c's and ranger-base-pvkhn's readings
were.

| arm | how | result | wall |
|---|---|---|---|
| control, nesting process | `./scripts/posse-wall-check` from the seat | **exit 0**, `wall check: green, 32 pins measured` | 81.8s and 70s warm (load not recorded) |
| **venue** | `bash docs/adr/0097-wall-venue.probe.sh venue+check` | **exit 0**, `wall check: green, 32 pins measured` | 94s, 95s cold GOCACHE (load 9.27, 6.83) |
| **seat-proc** | `… seat-proc+check` | **exit 3**, tail `venue cannot nest: bwrap: setting up uid map: Read-only file system` | 0.017s |
| planted member FAIL | a scratch arm-1 test calling the gate then `t.Error`, control arm | **exit 1**, `FAIL TestLiveWallVenuePlantedRedForLagas: zzz_lagas_plant_test.go:12: planted red: …`, 33 members | 79s |
| planted build error | a scratch arm-1 file calling an undefined function | **exit 1**, tail `internal/posse/zzz_lagas_break_test.go:5:15: undefined: undefinedForLagas` | 1s |
| no bwrap on PATH | a scratch PATH holding only `bash` | **exit 3**, tail `venue cannot nest: bwrap: not found on PATH` | instant |
| no python3 on PATH | a scratch PATH holding `bash` and `bwrap` | **exit 3**, tail `wall check: no answer — no python3 on PATH to read the run`, asked BEFORE the 95s run | instant |
| `make tree-check` | after both plants were removed | green | 75.6s, 52.3s, 64.5s (load 6.74, 5.68, 9.63) |
| `make test` | all three arms | green; arm 1 cmd/posse 121.1s, internal/posse 212.5s, internal/treepins 177.7s; arm 2 76.7s; arm 3 99.2s; silent-revert audit 2697 commits, 0 untriaged | — |

Both plants were scratch and neither is committed. The two venue readings were
taken either side of the parser fix below, the second over the bytes that
landed. Both sit just under the ADR's own 96-104s band for a cold venue run of
the pins ALONE, so what the body adds to that run is inside its noise: the
baseline costs the 17ms the seat-proc row reads, and the parse is one python
pass over the run's few thousand JSON lines. The 32 is the
ADR's count: `go tool test2json` attributes the marker to exactly 32 of the 34
tests the filter selects, the two without it being
`TestLiveWallWitnessHoldsUnderTheRenderedWall` and
`TestLiveWallWitnessProbeDoesNotWriteTheLiveWall`, which ask the AVAILABILITY
probe through shipped code and never nest.

The seat-proc arm is 0.017s because the baseline fails first, which is the
point of putting it first (ADR 0086 D5): without proof that a bwrap starts,
a pin that expects a refusal passes over nothing.

## The arm that crashed, and why it was found by running it

The verdict mapping was then driven over nine synthetic `go test -json`
streams, with the parser extracted from the script and fed from a scratch
driver: green; a member FAIL; a member that SKIPs after the marker (the
`/var/tmp` row); zero members over tests that ran; a filter that selected
nothing at all; a build error with no test events; green members with a
NON-member failure; a member that printed the marker and reported no verdict;
and a member whose marker and failure come from a subtest.

**The abstention arm raised `TypeError: not enough arguments for format
string` and the script exited 1.** The message carried three `%` conversions
and two arguments — a `%s` typed where `/var/tmp` was meant. So the one arm
ADR 0097 D5's "an abstention is never a pass" exists for answered 1 instead of
3, and D5's ratchet would have filed `wall pins red at <tree>` against
whoever landed last over a tree whose pins had merely gone dark. Reading the
code did not find it; running it did. This is the shape ranger-base-ietka
recorded one bead earlier: a failure message that crashes on the one
direction the check exists for.

Two things came out of that, both of them beads rather than prose:

- **ranger-base-zpx1k** — nothing in the suite reads those three exits.
  `make verify-python-syntax` censuses `.py` FILES, so a `python3 -c` block
  inside a shell script is outside its corpus (as it is for `queue-scrub.sh`,
  `suite-lock.sh` and `ceiling-fill.sh`), and a syntax check would not have
  caught a runtime `TypeError` anyway. The bead carries the two shapes a door
  could take and why the choice is not obvious.
- **ranger-base-enhtx** — `TestQANoEntryPointRunsGoTestOnTheDefaultTimeout`
  globs `scripts/*.sh`, and this body is the first extensionless entry point
  under `scripts/` that runs a LIVE `go test`. It carries `-timeout 15m`
  today; nothing holds it there.

## Four decisions in the body that a reader may want the reason for

- **The run goes through a shell variable, not a temp file and not a pipe.**
  The parser needs both halves of the run: the JSON, and `go test`'s own exit
  STATUS, which is the only thing that separates "the tree did not build"
  (exit 1) from "the filter selected nothing" (exit 3). A pipeline hands it
  one or the other, and a temp file wants somewhere writable, which under ADR
  0088 D3 is the landed tree and the build cache and not much else.
- **`GOFLAGS` is scrubbed.** It is on the judge's environment allowlist
  (`landedJudgeEnvKeys`) and `go test` takes flags out of it. A
  `-tags posse_arm2` there excludes `wallvenue_test.go`, the package then
  fails to compile with `undefined: bwrapNestSkip`, and this check reds a tree
  that is fine — a P1 filed against a persona for a variable nobody typed.
  Command-line flags win over `GOFLAGS` for everything the body names, so the
  hazard is entirely in what it does not name.
- **A failure that is not a member's is reported and is not this check's
  red.** Two tests inside D3's filter print no marker. If one of them fails
  while every member passes, the body still exits 0 and says so in a `note:`
  line above the green sentence. Those two reds are every seat's `make test`
  and the judge's business; ADR 0097's subject is the 32 pins that measure
  nowhere else.
- **Both preflight asks happen before the 95s run.** A host with no bwrap is
  exit 3 because D5 already counts one as unanswered; a host with no python3
  is exit 3 for a reason one step further on — the verdict is READ by it, so
  without it a venue run would cost 95s and then exit 127, which is off D3's
  table entirely and is no answer at all (`verify-bd-argv-gate.sh` says the
  same sentence one directory over).
- **A member that printed the marker and reported no verdict is exit 3, not
  1.** The shapes that produce it — a panic that kills the binary, the
  deadline, a venue that died — are what D5 already counts as unanswered, and
  from the JSON they are indistinguishable from one another. The run's tail is
  printed ABOVE the member names in that arm, because the reason is in the
  tail while D3 says the names are what the tail must carry, and the runner
  keeps the LAST few non-empty lines.

## Registration under scripts/, and the two rosters that wanted nothing

`scripts/shell-syntax.sh` takes its corpus from `git ls-files` — every tracked
`*.sh` plus every tracked file whose first line is a shell shebang — so the
body joined it by being `git add`ed, and `make verify-shell-syntax` then reads
`74 shell scripts parse` with it in the corpus (9 self-test arms clean).

`make scripts-check`'s nine pins walk every file under `scripts/` whatever its
name, so the body is in all of their populations and none of them wanted a
row: it registers no trap, and it makes no `mktemp` root or `mktemp` file.
That is the same decision as "a shell variable and not a temp file" above and
not a second one — the body has nowhere to leak because it has nothing to
clean up. Both rosters are two-way (a row the census no longer finds reds as
stale), so adding one "for completeness" would have been a red rather than a
courtesy.

The one door that would have had something to say cannot see the file:
ranger-base-enhtx, above.

## The probe's two new arms

`venue+check` and `seat-proc+check` run the body in place of the pins, with
`POSSE_WALL_VENUE=1` in front of it as a plain assignment rather than a
`--setenv` on the wall. That is deliberate: with the variable set, a refusal
is a FAIL and not a skip (D4), which would change what the `seat-proc` row of
the ADR's table measures. The wall an arm renders is now chosen by the word
BEFORE the first `+`, so `seat-proc+check` gets seat-proc's wall and
`judge+venue` is unchanged. A `+check` arm prints the last four lines of the
body's stderr and no JSON counters, because the body writes its JSON nowhere —
its tail IS its answer (ADR 0088 D1).

D1's own render — `POSSE_WALL_VENUE=1` on the venue's environment allowlist,
through a flag on `boxLandedCage` — is ranger-base-wplhv's, and until it
lands `wallVenueEnvVar` in `internal/posse/wallvenue_test.go` and the
`venue_env` line in this body are the only places that name is spelled.
