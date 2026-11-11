# ranger-base-j687x — the typed route had no liveness guard, and the trace is why that is a change

The bead it came from (ranger-base-xdaw9) measured two facts on the way past
and filed this one rather than fixing it:

1. **A reported label OUTLIVES the CLI it describes on this herdr.** MEASURED
   2026-11-11 (herdr 0.9.1, claude 2.1.322, this box): `pane report-agent
   --agent fakebob-xdaw9 --state idle` over a pane running claude; the first
   probe's Enter actuated the folder-trust prompt's `No, exit` and claude
   EXITED; `agent get w1:p1` then still read
   `{"agent":"fakebob-xdaw9","agent_status":"idle"}` over a pane that was a
   bare shell prompt. That is ADR 0061 Claims fact 1 reproduced on a live
   claude, on this herdr.
2. **`pane send-text <pane> "<text>"` + `pane send-keys <pane> enter` into a
   pane at a shell RUNS the text.** The same probe launched claude that way on
   purpose (`cd <dir> && claude --model claude-haiku-5-5`) and the shell
   executed it.

Venue, screens and teardown are in `docs/notes.d/ranger-base-xdaw9.md`; this
bead took no new live measurement and says so.

## What was NOT measured, and was this bead's first job

The bead's own words: "whether any live caller reaches it today. `--now` is
the named route; dispatch's path was not traced for this bead. Trace that
first — if nothing can reach it, this is a comment and not a change."

TRACED 2026-11-11 by reading every caller of `HerdrMux.Prompt` (the only
caller of `Herdr.AgentPrompt`, which is the one door onto the typed route):

| call site | gate in front | reaches the typed route unguarded |
|---|---|---|
| `cmd/posse/main.go` `posse prompt` | `AwaitPromptable`, **skipped by `--now`** | **yes, on `--now`** |
| `cmd/posse/cockpit.go` `promptSession` | `AwaitPromptable`, no ungated key | no |
| `internal/posse/pulse.go` | `AwaitPromptable`, plus its own idle\|done rule | no |
| `internal/posse/dispatch.go:3846` (the pass) | `awaitSettled`/`awaitDelivered`, both on `det.Seen()` | no |
| `internal/posse/dispatch.go:5730` (the cockpit's `d`) | `launchSession` → the same ladders | no |
| `internal/posse/relaunch.go:863` `landThePlane` | **none, on purpose** | **yes** |
| `internal/posse/runtimeprobe.go:901` | `agent wait --until idle\|done\|blocked` only | **yes** |

So three, and the two nobody had named are the interesting ones.
`landThePlane`'s gatelessness is argued in its own comment and the argument is
sound as far as it goes — `AgentTarget` has just resolved a LIVE PANE, so
there is no launch line to choose between. A live pane is not a live CLI,
which is ADR 0061's title. The probe's `agent wait` gate is weaker than it
looks on this route for a reason herdr documents: herdr answers the REPORTED
state, so a stale `idle` settles that wait instantly.

`det.Seen()` is what makes the five gated rows gated — it carries
`!ShellForeground` — so dispatch was never the hole. It is also still a
time-of-check-to-time-of-use reading: the gate reads liveness, and the
keystrokes go in later.

## The fix

`refuseAtShellPrompt` in `internal/posse/reportedagent.go`, called from
`sendTextPrompt` before either keystroke and before the screen refusal beside
it. It asks `paneShellForeground` — ADR 0061 D3.2's existing reading, no new
mechanism — and refuses with `agent_not_ready` so ADR 0084 D1's
`isNotDelivered` hands a dispatch claim back. ADR 0061's Amendment (D5-D7) is
the decision; ADR 0076 Amendment 7 D7 is the argument it borrows.

Two things it does NOT copy from the screen refusal, both deliberate:

- **no scope clause.** The screen refusal asks `agent explain` whose chrome
  the screen is drawn with, because the box it locates is claude's idiom. This
  reading is a pid compared to a pid — no argv matched, no shell list kept, no
  runtime named — so it answers for a stranded Bob exactly as for a stranded
  claude, and a LIVE CLI of any kind has its own pid in the foreground and is
  never refused. The designed case of the typed route is the case the screen
  refusal concedes and this one covers.
- **no second look.** `promptComposerRecheck` exists because a screen can be
  caught mid-redraw. A foreground process group cannot: the reading is "every
  foreground pid IS the pane's own shell pid", which goes false the instant
  anything is launched there. One `process-info` per prompt, MEASURED 0.00s
  (ADR 0061 Claims).

Order matters for the diagnosis and not only for the cost. A pane at a shell
has no CLI on it, so `agent explain` names no claude there and the screen
refusal would concede it and say nothing. Reading liveness first buys the
right sentence for one call, and spends neither the scope `agent explain` nor
the `pane read` behind it.

## The message that had to change with it

The gate's reported refusal ended by offering `--now` — and for the stale
clause that is now a door that answers `agent_not_ready`. `AwaitPromptable`
says so instead, on that clause only: the `unknown`-state clause (ADR 0061
D3.1) keeps the offer, because that is a pane posse cannot judge, which is
what `--now` is for. `posse prompt`'s help line and the `--now` call site say
the same thing in one clause each.

One existing pin asserted the old promise —
`TestQAAStaleReportedLabelRefusesThePromptWithNothingTyped` in
`reportedreading_qa_test.go` required `--now` in that refusal — and now
asserts its absence, naming this bead. That is the only pre-existing
assertion this change inverted.

## Verified

- `internal/posse/promptstalelabel_qa_test.go` (new, `posse_arm3`): the
  refusal in all four watched states with nothing typed; `isNotDelivered`;
  the refusal carries `ReportedNotSeen`'s own clause, asserted as the same
  string rather than a second spelling; the three concessions (an unreadable
  `process-info`, an empty foreground, a missing `shell_pid`) all still
  deliver; a live foreground still delivers; the stranded-Bob pane the screen
  refusal concedes is refused here; the order arms (no screen read before the
  liveness answer, one `process-info`, and the `/`-prefix and `agent_blocked`
  refusals still ahead of it); both halves of the gate's `--now` change;
  delivery through `HerdrMux.Prompt` rather than the method, so the pin is
  about who reaches it.
- `reportedReadingFuncs` in `reportedreading_qa_test.go` names
  `refuseAtShellPrompt`, so the census holds it to "lives in one file, names
  no runtime" with the rest of the reading.
- `make fmt-check`, `make adr-check`, `make tree-check`, `make test` (three
  arms), `go run ./cmd/checkorphans`.

## Filed, not fixed: the same hole on the other occupant

`ranger-base-6pew3`. `RexMux.Prompt` types after the `/` refusal and
`refuseWithoutComposer`, and that predicate opens `owner, err :=
m.RuntimeOwner(s); if err != nil … return nil` — so the one Rex reading that
already distinguishes "the record outlived its writer" and "that was the
block's own shell" (ADR 0076 §3's three guards, `rexOwnerLive` /
`rexOwnerIsShell`) hands its answer to a predicate that concedes on it. On a
rex instance there is nothing upstream either: `HerdrBackend.explain` answers
`muxNotHerdr` off the herdr slot, so the readiness gate takes its
never-answered concession and dispatch's ladders take theirs, and nothing
outside `muxrex.go` calls `Mux().State`. That is this bead's argument with the
bypass being the whole instance rather than one flag, and this box IS a rex
instance. What it needs first is the measurement this bead did not take:
whether a rex block whose CLI exited leaves a `program_status` record at all,
and which of the three cases it lands in.

## What this does not do

It refuses only on LIVENESS. `ReportedNotSeen`'s other clause — a reporter
state nobody watched (`unknown`, or none) — is a WAIT condition in ADR 0061
D3.1 and not a refusal, and the delivery verb has nothing to wait with; a pane
in that state whose foreground is a live CLI has something holding the
keyboard, and one at a shell is caught by this guard anyway. The gate keeps
that clause and keeps offering `--now` for it.
