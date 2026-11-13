# Only a launch makes what the namespace cannot (ranger-base-qd9ei)

*2026-11-12 · gwart · the residual of ranger-base-6s5mo, out of
ranger-base-atq78*

ranger-base-6s5mo moved the ARTIFACT of every non-launch wall render out of
the persona's live gates dir and stopped the REPORT's own render making the
host-side prep. It left the prep alone in parity's two probes
(`applyRecordReach`, `applyWallWitness`) on purpose: those two render and
then **apply**, bwrap binds only a source that exists, and a probe with no
prep measures a wall that cannot come up and reports the difference as this
host's. Both probes hang off `checkParityIn`, which `posse gates <persona>`
calls once per runtime in the catalog — so a report typed with a HOME that
was not the launcher's still made that HOME's prep paths.

## What that cost, measured

MEASURED 2026-11-12 (the bead, from a bwrap-caged seat): `HOME=<fresh empty
dir> posse gates builder` in a scratch RHQ_HOME created, in that HOME,
`.bob`, `.cache/posse/plan-usage`, `.cache/posse-seat/<session>`,
`.config/go/telemetry/local` and 24 members of `.grok`.

MEASURED 2026-11-12 in this fix's own control — the defect restored
(`parityAsk.prep()` returning `prepMake` for a reader) and the new pin run
through the built binary: **39 paths** in the HOME the command was typed
from, on the run that renders and again on the run that reads. At the fix:
zero, both runs.

And an empty seeded file is not harmless. `~/.grok/config.toml` is seeded
EMPTY, posse's own project-config classifier reads an empty file as
`invalid JSON: unexpected end of JSON input`, and a later launch in that
home is REFUSED over it: ranger-base-f67jx, a P1 red in one arm of
`make test` that no `-run` filter could reproduce, because one test plants
the file and another reads it (docs/notes.d/ranger-base-zibko.md has the
chain, and names `TestQATheAbsoluteWritableRowDegradesNoLaunch` — a
`CheckParityIn` caller, which is to say a reader).

## The rule

**A reader makes no path on the host, and says so when that leaves it
nothing to measure.** `wallWrite` carries three prep answers now instead of
two (`wallPrep`, wall.go):

- a **LAUNCH**: make it. That process *is* the launcher, it writes the same
  paths itself a few hundred lines later (`WallPrefix`, herdrback.go), and
  the probes run before that — so without it a first launch into a fresh
  tree would judge a wall that cannot come up.
- a **REPORT**: skip it. A command whose whole job is to describe the wall
  has no business making it (6s5mo).
- a **READER'S PROBE**: require it. It applies what it renders, so it cannot
  skip; it may not write whatever HOME typed the command, so it does not
  make. Where the wall needs a path this host has not got, the render
  refuses with `wallPrepAbsentError` and the row takes its third answer.

Which parity run this is comes from `parityAsk`, threaded from
`checkParityIn`: `askedByLaunch` from `CreateSession`, `askedByReader`
everywhere else, and the zero value is the reader — a caller that says
nothing writes nothing. Not read off the `red *l3Redirect` parameter, which
is non-nil only from the launcher today: the two are true of the launcher
together and neither implies the other.

### Why this is not the bead's candidate (a)

The bead proposed abstaining on the report path outright, and said it would
cost the operator a row on the one surface they read a wall off. It does not
have to. **Every prep step is emitted only where its path is ABSENT** —
every one of `BwrapArgs`' emission sites is guarded on a stat — so the list's
emptiness is itself a verdict: a render that asked for no prep is a wall
whose every bind source is already on this host, and applying it writes
nothing. So a reader's probe measures wherever the furniture is there and
abstains only where it is not. That biconditional is now pinned in both
directions (`TestQAEveryBwrapPrepStepNamesAPathThatIsNotThere`); a step
emitted over a path that exists would make a reader abstain over nothing,
and an emission site that stops being guarded would make it measure a wall
it cannot put up.

### Why the abstention and not make-measure-remove

The tempting third option is to make the absent paths, probe, and remove
them again — they are empty by construction. It trades a benign extra
directory for a race on shared state: `~/.cache/posse-seat/<cwd>/tmp` is
keyed by the launch DIR and shared by any seat launching there, so a reader
that creates it and removes it 200ms later can pull it out from under a
launch that started in between, and the gates dir's `refusals.log` is
appended to by a live seat's shims. There is no lock over either. The
abstention is the fail-closed answer and it is the one both ADRs already
have a vocabulary for.

### What it costs, measured

MEASURED 2026-11-12, this instance, `posse gates hoover` typed from a
`cage: shims` seat's worktree: **9 rows abstain** — Edit, Write and
`record: store of record`, for each of the three runtimes whose block is at
`cage seatbelt (bwrap)` — each naming
`~/.cache/posse-seat/<this cwd>/tmp` first and 36, 29 and 1 more paths. The
row says which path, that only a launch makes it, that the launch it judges
is unaffected, and what to type instead (`posse new` / `posse relaunch`, or
`posse gates` from the launcher's own environment once one has run).

`posse gates gwart` on this instance loses nothing: `cage_realize_linux:`
realizes seatbelt as shims for that persona, so no wall is rendered and no
probe runs.

The reading those 9 rows gave before was of a wall that was only buildable
because the probe built it, under whatever HOME typed the command. For the
operator's own shell that HOME *is* the launcher's, so the reading was real
— and so were the writes. That is the trade.

**And it takes a documented verify recipe with it.**
docs/notes.d/ranger-base-3fm26.md §4 grades an ADR 0086 D2 close *without
writing a deployed home* by running `posse gates <persona>` over a `cp -a`
copy of it — a caller that legitimately WANTS the probe to prep, which is
why it copies the home first. Those two rows read `NOT WITNESSED` from a
report now (the absent path is keyed by HOME and cwd, and copying the home
does not make it), so that fragment is amended with what a verify seat
should expect: the `⊘ cage_realize` line it was written for is unaffected,
and the witness rows are a LAUNCH's observable again — which is what ADR
0086 D5 calls them. An opt-in `posse gates` that makes the furniture and
measures is a new flag and a new decision about what a reader may write —
this invariant deliberately reopened under an explicit flag — so it is filed
as ranger-base-jg3le rather than built here, with the three candidates and
that recipe as its named caller.

## What it does NOT close

ranger-base-f67jx's class survives for a test — or a command — that also
LAUNCHES, because a launch may prep. MEASURED 2026-11-12, zibko's own repro
(`RHQ_TEST_HOME=<empty dir> go test -count=1 -run
'^TestQATheAbsoluteWritableRowDegradesNoLaunch$' ./internal/posse`): **40
paths in that home at the defect and 40 at the fix**, `.grok/config.toml`
among them both times. That test calls `CheckParityIn` and then
`planLaunch`, and the launch makes everything the reader's probes used to
make. So the fix removes the reader as a SOURCE of the seed and leaves the
launcher, which is the one caller entitled to it. A test that wants an
unseeded home still needs its own HOME (`wtqaHome`), which is what
docs/notes.d/ranger-base-zibko.md's own correction asks for.

## Pins

- `internal/posse/readerprobeprep_qa_test.go` — five arms. The emission
  invariant in both directions; a reader's render refuses with the typed
  error, names the first step and the count, and makes nothing in its HOME
  (control: the identical render on a launch's behalf makes every one of
  them); a reader's render over a host that HAS the whole wall still renders,
  under `reportWallDir`, leaving a planted live artifact alone; and both
  doors — `applyWallWitness` and `applyRecordReach` — abstain with the row
  still realized, nothing degraded and nothing unrealized, each with the
  launch's own call as its control.
- `cmd/posse/gatesreportwrites_qa_test.go` —
  `TestQAGatesReportMakesNoPathInTheHomeItWasTypedFrom`: the bead's repro
  through the built binary, on the run that renders and on the run that
  reads. A whole-tree reading of the HOME and not a list of names, because
  the paths are a function of the catalog this instance ships.
- Updated: `wallwitness_qa_test.go`'s wiring arm drives the LAUNCH's door
  (`checkParityIn(..., askedByLaunch)`) rather than `CheckParityIn`. The two
  used to be the same call; the exported one is the reader's now, and over a
  fixture whose furniture is not on this host a reader abstains correctly —
  which would have red that arm for a reason that is nothing to do with the
  wiring it pins.

CONTROL: the defect restored (`parityAsk.prep()` returning `prepMake` for a
reader) reds the two door arms in internal/posse — naming each `mkdir -p`
and `touch` it made in the fixture HOME — and the binary arm with its 39
paths. `TestQAEveryBwrapPrepStepNamesAPathThatIsNotThere` and the render
arms stay green under that mutant, which is right: they are about the render
and not about who asked.
