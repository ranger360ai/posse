# Extracting the multiplexer slot: four places the interface is not ADR 0076 §2's table (ranger-base-csnzn)

2026-10-31. The refactor bead behind ADR 0076 §1 — define `Mux`, implement it
for herdr, route `HerdrBackend`'s `b.H` calls and the fourteen files that
called the client directly through it, change no behaviour.

The design held. Four details did not survive contact with the code exactly
as §2's table spells them, and each is written here rather than left as a
surprise in the next reader's diff. None changes what posse does; the suite is
the proof, and no test was edited for behaviour.

## 1. `Start` is an eleventh method, and it is §2's `Create` row

§2's `Create` row names two herdr commands: `workspace create` + `pane run`.
A single verb that did both cannot stand at either of posse's two call sites.

- `HerdrBackend.startPlanned` types the runtime's line only AFTER the meta
  that records the session is on disk. Fold the line into `Create` and a
  failed meta write leaves a runtime running that posse has no record of —
  the orphan `rangerhq-v52t` and `mustNotOrphan` exist to prevent, and the
  one state the launcher may never write over.
- `HerdrBackend.RestartAgent` types a line into a pane whose shell has been
  settled for hours. There is no create there to fold into.

So `Create(label, cwd, env) Seat` opens the seat and `Start(Seat, line)` runs
the line. The interface carries both; the ten verbs are otherwise §2's.

An occupant that cannot open a seat without its command — Rex, whose blocks
take `{command, cwd}` — has to answer `Start` by other means and say so. That
is the Rex occupant's bead. Noting it here because it is a real constraint
ADR 0076 §2's table does not show: on Rex the meta cannot be written between
the create and the launch, so §5's "posse keeps what it already owns" has to
say what the ordering becomes.

## 2. `MuxReading` has a fifth field: `Seen`

§2 lists `State`, `Kind`, `Owner`, `Evidence`. §3 then says a Rex record's
owner, checked by pid and name and not the block's shell, "is the Rex form of
herdr's `Seen()`: positive evidence that the runtime has the keyboard."

`Seen` is a verdict two occupants reach from facts with no spelling in
common — herdr from its own detection (a matched rule, chrome it can see, or
another authority's word about a live pane), Rex from the record's owner — so
it cannot be derived at a call site from the four fields, and it cannot be
derived from `State`: every settled state and `working` itself are guesses on
a seat nothing has recognized, which is the whole of ADR 0013 §1. It is the
occupant's answer, so it is a field the occupant fills.

## 3. Timeouts are milliseconds, not a `time.Duration`

They are herdr's own contract: `agent wait --timeout <ms>` returns inside
that window or returns a timeout envelope, and `Herdr.callDeadline` reads the
number back out of the argv to size the child deadline it wraps the call in
(ranger-base-wj7e9). A `time.Duration` on the interface would convert at both
ends of every call and could only ever lose a factor of a million.

## 4. §4's list of herdr-only verbs is four verbs short

§4 enumerates the `EvaluatedRules` diagnostics, the readings-log detection
read, `PaneAgentSession`, the detection preflight and the report-agent route.
Four more verbs land herdr-only for reasons §4's own rule covers, each with a
door on `HerdrBackend` whose "not available" answer is an arm the caller
already had:

| verb | door | why it is herdr's |
|---|---|---|
| `workspace list`, raw | `b.workspaces` | the three readers that ask it — `sessionGone`, `holdsRecorded`, `nameWornElsewhere` — are doing herdr's id arithmetic: a workspace id identifies nothing across a server restart or handoff, so telling "ours" from "a stranger wearing our id" needs the label and the server GENERATION (`rangerhq-6bg7`, `notOurWorkspace`). §5 says Rex has no such question — its UUIDs are never reissued, so a seat is dead when its id is missing from the listing. Keeping it here also keeps those three at one herdr call, where `List` would join an `agent list` none of them can use. |
| `workspace get` | `b.workspaceGet` | the per-id query the meta rule's two halves ask, because a listing is a snapshot and absence from one is never death (ADR 0011 §2, `rangerhq-9nso`). Same §5 reason: on Rex the listing IS the answer. |
| `agent explain`, whole | `b.explain` → `HerdrMux.Explain` | §4 names the evaluated rules; in fact every consumer of `agent explain` in this tree reads the `AgentDetection` — `Seen()`, `FallbackReason`, `ReportedNotSeen()`, the composer regions, `WhatHerdrSaw` — so the thin `State(Seat)` reading has no production caller yet and the Rex occupant is its first. `State` is implemented and mapped from the same detection; it is not dead code waiting to rot, it is the verb the second occupant answers. |
| `agent list`, raw | `HerdrMux.Agents` | `posse probe`'s detection loop asks about a workspace it CREATED an instant ago and holds the id of, so the `workspace list` the slot's `List` joins on answers nothing it does not already know. Routing it through `List` was the one change in this bead the suite caught: `TestQAProbeStalledPromptIsReadForTheTurnAndNotCalledUndelivered`'s fake herdr answers `workspace create` and `agent list` and no listing, because the probe never needed one. |
| `Available` | `HerdrMux.Available` | asked once, by `posse probe`, before it will report a detection observable at all. |

The occupant's §4 forwarders keep the client's own method names, so a reader
following a call lands where it always did — except `Explain`, which forwards
`AgentExplain`. ADR 0061 D3's detection reading is allowed two homes, and
`reportedreading_qa_test.go` holds it to them by FUNCTION NAME: a second
`AgentExplain` in the tree is how a second reading starts, and nothing
mechanical can tell a one-line forwarder from a real one. That pin is in arm 3
and is the second thing the suite caught here. The name gave way; the pin did
not.

`posse probe`'s own helpers take the herdr occupant rather than the interface
(`RuntimeProbe(rt, HerdrMux, …)`). The probe's fourth observable IS herdr's
detection, so a probe that could be handed another occupant would be
measuring something it has no rule for.

## What the extraction cost, measured

MEASURED 2026-10-31, this worktree, `sh docs/adr/0076-multiplexer-slot.probe.sh`
part 1: files calling the herdr client fell from **14 to 3** —
`muxherdr.go` (the occupant), `reportedagent.go` (the report-agent route) and
`detection.go` (the detection preflight, now through a `ManifestReader`
interface the bare client also satisfies). The last two are exactly §4's
type-assertion sites, so the census now prints the design.

The probe matches receivers by SPELLING (`.H`, `h`, `herdr*`), which is why
`posse probe`'s local was renamed from `h` to `mux` when its parameter became
the occupant — otherwise two `HerdrMux` method calls read as client calls —
and why the invariant also has a pin now:
`internal/treepins/muxseam_qa_test.go`, which asks which files can HOLD a
`Herdr` rather than who calls one, doored by `make corpus-check`.

One herdr call was added, on purpose, and in exactly one place.
`HerdrBackend.AgentTarget` now reads the slot's `List()` instead of herdr's
`agent list` alone, because "which pane holds a runtime" is a question every
occupant must answer and a herdr-only answer there would make dispatch
herdr-only. That costs one `workspace list` per call. The poll loop that
calls it (`awaitTarget`) already paid both listings through `Resolve`, so the
countable the startup-wait pin reads — `agent list` calls per poll — is
unchanged at two.

The same move in `posse probe`'s detection loop was wrong and the suite said
so; it is in §4's table above. The difference is whether the caller already
knows the seat: `AgentTarget` is handed a NAME and has to find the pane,
which is every occupant's question; `awaitProbeAgent` created the workspace
one function call earlier.
