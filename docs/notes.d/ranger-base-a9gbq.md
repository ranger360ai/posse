# A platform filename suffix is a declaration, and three lines that were load-bearing and unpinned (ranger-base-a9gbq)

Four findings from ranger-base-0a1jk's verify of ranger-base-tea4e,
ranger-base-enhtx, ranger-base-slece and ranger-base-5ex01. All four closes
were VERIFIED there; these are pin gaps beside those verdicts, not behaviour
defects — the shipped code was right in every one of them and nothing in the
suite said so. One of them was red on a platform.

## 1. The file-set half read the host GOOS and `want` read the build line

`internal/treepins/armtags_qa_test.go`'s arm 6 asks the toolchain which test
files it built for each arm and compares that set against what the census puts
there. `built` came from `go list` on the HOST GOOS; `want` was computed from
each file's `//go:build` line alone (`armPlaces`). A platform FILENAME suffix
leaves the build line untouched, so on a non-Linux host every
`internal/posse/*_linux_test.go` read as "in arm N here and the toolchain does
not build it there".

Three such files landed 2026-11-14 — wallwatchtty_linux_test.go
(ranger-base-heypq, the first of the class and the one this escaped from),
wallwatchbackstoplive_linux_test.go (ranger-base-tea4e) and
wallwatchbackstopwiring_linux_test.go (ranger-base-slece). ci.yml's matrix is
[ubuntu-latest, macos-latest] x [1,2,3] with fail-fast off, and this one test
is the FIRST command of all three arm recipes on both OSes, so three of the
six jobs were going red on the next push.

MEASURED 2026-11-14, linux, at 3d4703e7: `go list` over internal/posse names
371 test files natively and 368 under `GOOS=darwin GOARCH=amd64`, the three
being exactly those files.

**The decision, and it is a decision and not a patch.** The pin's own head
comment treated the filename-suffix case as a defect to CATCH —
ranger-base-6m7rd's mutation control was literally "renaming a shared file to
`*_linux_test.go` fails the file-set half for all three arms" — so the
question was whether a deliberately platform-constrained test file is
representable at all. It is, and the suffix is a DECLARATION like a build tag,
which this census does not second-guess. What it owes its reader is that the
file runs in SOME job, which is the whole of 6m7rd's worry.

So `armPlatformAxis` is a third axis beside `armModeAxis`, holding exactly the
GOOSes ci.yml's matrix runs, and `armPlaces` takes the armFile rather than its
build line because the platform is a fact about the NAME. Every other platform
or architecture suffix is left unrepresented on purpose: such a file reads as
shared, the toolchain builds it in no job of the matrix, and the file-set half
reds for all three arms with the message that names the cause. An arch suffix
is unrepresentable rather than merely unlisted — the matrix's two runners are
amd64 and arm64, so `x_amd64_test.go` runs on one and not the other and "which
jobs build it" stops being a property of the axis.

Arm 4 (the door-pin arm) now reads the whole armFile too: a door pin in a file
whose name constrains it to another platform is the same rot one axis over.

**Arm 8, `TestQAArmPlatformAxisIsThePlatformsCIRuns`**, is what keeps the
representation from being a silencer. Whether a `_<goos>_test.go` file runs in
any job is a property of the MATRIX, not of the census; hardcoding the pair and
asserting nothing would mean that dropping macos-latest makes every
`_darwin_test.go` file go quiet with this census vouching for it. Both
directions, because each is a different mistake.

It is not in the tree-wide class its siblings are in — it reads one tracked
file by a relative literal and enumerates no directory, which is the treepins
rule's own negative case — so no `-run` door can find it. On the base this work
was cut from that left it with no door at all, and adding it to
`$(QA_CORPUS_PINS)` was refused by `make register-check` ("it IS a test in
internal/treepins and is no longer derived as tree-wide"); that Makefile edit
was reverted. On the base it LANDS on it needs no entry: ranger-base-3cpcg's
`make treepins-check` (ADR 0098) is the package minus `$(QA_TREEPINS_EXCLUDED)`'s
twelve priced rows, this arm is in none of them, and that target is on
`make tree-check`'s line. VERIFIED 2026-11-14 on 1d03f9d1. It is the case that
door was named for: "a pin written tomorrow is doored the moment it is
written", and the pin that most needed one was the pin nothing could find.

MUTATION-CHECKED 2026-11-14 at 1d03f9d1 on linux, md5 on both sides of each,
`git status` clean after:

| mutant | reading |
|---|---|
| shared `agents_test.go` -> `agents_windows_test.go` | file-set half reds, 3 `reads as arm N` lines (one per arm) |
| the same file -> `agents_linux_test.go` | green — the fix |
| arm-1-tagged `wallwatchtty_linux_test.go` -> `_windows_test.go` | reds arm 1 alone (its build line puts it in no other arm's `want`) |
| `wallwatchtty_linux_test.go` -> `_darwin_test.go` | silent here, and correctly: the axis takes it out of `want` and the toolchain out of `built`, which is the two readers agreeing. That mutant is caught by the arm the file's own tests are in, on the platform that runs them — which is the inverted repro of the darwin failure, and the evidence the fix makes the macOS job green |
| axis + `"windows": "windows"` | arm 8 reds, second direction |
| axis - `"darwin"` | arm 8 reds, first direction — and the file-set half stays GREEN on this box, which is why arm 8 is not redundant with it |
| ci.yml's `os:` key renamed | arm 8's fatal |

## 2 and 3. Two lines of ranger-base-5ex01's lane census

Both in `labelMatches` (internal/posse/dispatch.go), both specified on their
bead and stated in a code comment, and deleting either left every shipped lane
and route pin green: `go test -tags posse_arm2 ./internal/posse -run
'TestQALane|TestQARoute|TestQABusy|TestQAHeadAnchors'` read ok in 1.449s with
either gone (MEASURED 2026-11-14 at 3d4703e7). The roster each needs is a shape
no fixture there happened to have.

**Finding 2, the label-less PID.** `primary := ag.Labels[0]` indexes the slice
on the line after `if len(ag.Labels) == 0 { continue }`, so without the guard a
roster holding one PID with no `labels:` key panics — `index out of range [0]
with length 0` — on every pass, for every bead with a candidate. The SHIPPED
examples roster has that shape: `examples/agents/architect.md` has no `labels:`
key at all, so the panic was one `posse dispatch` away from any instance that
installed the published roster. `TestQARosterWithALabellessPIDIsNotAPanic`
writes the frontmatter directly rather than through `writePersona`, which
always writes a `labels:` line — the omitted key is the whole subject.

**Finding 3, the coordinator's exclusion from the LANE CENSUS**, which is a
different claim from her exclusion from candidacy. A label's lanes are the
first labels of every non-coordinator PID carrying it (ADR 0011 §4 as amended
2026-11-14), and an anchor rests on a label belonging to exactly one lane.
Counting her does not merely print a different label: she carries a lane's
vocabulary for triage, so a label she shares with one real seat reads as TWO
lanes, that seat's anchor disappears, and the bead goes to whichever unanchored
candidate sorts first — ranger-base-bskno's defect exactly, the one seat that
can do the deliverable stranded outside its own lane.

No lane pin could see the line: they set no `coordinator:` key, so
`isCoordinator` answers false for every name in them.

MEASURED both ways 2026-11-14 at 3d4703e7 over the new fixture's roster —
shipped: `lane=[ops-seat] label=devops`; with the census exclusion dropped and
candidacy left intact: `lane=[aaa-dev zz-arch] label=team`, byte for byte
bskno's characterization table. Both new pins kill their mutant and the four
older pins stay green under both, which is the finding reproduced.

It is one PID edit from live. On this instance's roster the label `ops` is
carried by a business-first PID and by the coordinator, so the coordinator is
the only reason `ops` reads as one lane, and 36 open and closed beads carry it.
No live route moves today, because the harm also needs the unanchored candidate
to sort first and this roster's does not — a name-order accident, not a
property, which is why the line needs a pin and not a note.

## 4. The backstop wrapper bound five values and one was pinned

ranger-base-slece's link 2 (ranger-base-a9gbq's own finding 4, filed on
ranger-base-0a1jk). `App.WallWatchBackstop` is one line binding five values,
with exactly one caller in the tree (guardclock.go) and no test calling it
directly. The pin that landed covers the state dir and only the state dir: its
record's seat is OVER, so the backstop drops the record and returns before any
of the store, the lock path or the clock is read. Three mutants survived the
whole WallWatch family — `go test ./internal/posse -run WallWatch`, ok 1.79s
each over 29 pins (MEASURED 2026-11-14 at 3d4703e7): the lock path suffixed,
`time.Now` replaced by a constant, and guardclock.go's store argument replaced
by nil. The probe is the fifth value and is not mutable in-tree, so the wrapper
was 1 of 5 pinned rather than 1 of 4.

**The fixture, and why it is the shape it is.** All three are read in
`fileWallWatchDetach`, which is only reached on the detach branch — and that
branch KILLS the seat with the production probe. `wallWatchKillWith` SIGKILLs
every process in the record's mount namespace, round after round, so a record
naming this host's namespace would end the box. So the test makes a mount
namespace of its own and puts exactly one expendable process in it: `sleep`
under `CLONE_NEWUSER|CLONE_NEWNS`, which needs no privilege and no bwrap, and
skips when the box refuses one. The namespace is asserted distinct from the
test process's own and asserted to hold exactly that one pid before the record
is written; those two assertions are load-bearing, not decorative — a clone
that quietly failed would hand the kill the host namespace.

The wall comes off without touching a mount: `wallWatchDetached` compares the
record's `Base` mount ids against the table it reads, so a `Base` naming an id
that namespace has never carried IS a detached wall.

All three values are read through `d.guardTick()`, the production call site, so
the store is the argument guardclock.go passes and not one the test chose.

MUTATION-CHECKED 2026-11-14 at 1d03f9d1 on linux, each red with its own
message, each reverted with md5 taken on both sides:

| mutant | reading |
|---|---|
| `wallWatchLockPath()` + `".mutant"` | `nothing at <lock> after the tick` |
| `time.Now` -> `func() time.Time { return time.Time{} }` | `the bead says the wall came off at 0001-01-01T00:00:00Z` |
| guardclock.go's `d.Bd` -> `nil` | a nil-pointer panic in the tick, which is what a real detach does in production |

`XDG_CACHE_HOME` is set per test so the production `wallWatchLockPath()` is a
path this test owns — otherwise its existence afterwards could be another
test's leftover, and mutant (a) would survive the assertion written for it.

## Not closed here

Nothing. Recorded on ranger-base-0a1jk's close instead, per ADR 0006 §6 (a fix
that changes nothing a process does and nothing a pin proves): the `(19)` count
comment at cmd/testparallel/main.go:578 against 20 entries, and three stale
MEASURED numbers in ranger-base-enhtx's corpus prose.
