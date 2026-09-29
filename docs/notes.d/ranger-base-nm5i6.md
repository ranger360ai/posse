## One detection reading for a reported pane: what the fold cost and what the fake was hiding (ranger-base-nm5i6)

Build notes for ADR 0061 D3 — 2026-09-28. The decisions are in the ADR and the
reasons are in the code; these are the two things that came out of building it
and belong nowhere else.

### The fold trades calls for the impossibility of disagreement

ranger-base-8eqaa routed a reported pane with `ReportedAgent` *before* the
promptable gate's screen ladder, precisely so it would not spend an `agent
explain` it knew herdr would refuse. That saving is what bought the second
ladder, and the second ladder is what made two readings possible.

They did disagree, structurally: 8eqaa's gate opened on ANY named state, where
the one door also asks whether the pane is still live (D3.2). A stranded label
was promptable through one ladder and refused by the other, in one binary.

So the door asks first, every time, and a reported pane's reading costs four
herdr calls where a detected one costs one:

| call | what it answers | measured |
|---|---|---|
| `agent explain <pane> --json` | refuses — and the refusal IS the route | 0.00s |
| `agent get <pane>` | the label and the reporter's state | 0.00s |
| `agent explain --agent <label> --file …` | does herdr have a manifest for it | 0.00s |
| `pane process-info --pane <pane>` | is the foreground still not the shell | 0.00–0.02s |

All four are the numbers already in ADR 0061 Claims and ranger-base-mx5x9 #5;
nothing new was measured here. At a 250ms poll that is under a tenth of a
second per poll, and the detected route pays none of it — the first call
succeeds and the arm is never entered.

**The order is the pin, not the count** (`reportedagent_qa_test.go`,
TestQAThePromptGateReadsAReportedStateAsPromptable). A build that reads any of
the last three without the first has grown a second reading; one that stops
before the fourth has dropped the liveness half and will type a work prompt
into a shell.

### The fake herdr was answering a shape herdr has never had

This is the finding worth the fragment. The fake's `agent explain <pane>`
returned a normal detection for **every** pane, including one armed with
`reported-agent` — so a test could arm a reported pane, watch posse read it
happily, and be measuring the DETECTED path the whole time. Every pin of the
reported arm would have passed with the arm deleted.

It is now derived from the two levers the reported route already reads — a
label in `reported-agent` that `herdr-kinds` does not name — rather than given
a lever of its own, deliberately: a fake that could be *armed*
reported-but-explainable would model a herdr that has never existed, and the
whole class of bug above would still be reachable.

The general shape, which is not specific to this verb: **a fixture that answers
a request the real thing refuses makes the refusal's own handling
unreachable**, and every pin over it is green for the wrong reason. The fake
had a lever for `pane report-agent` being absent, a lever for `agent get`
failing, and a lever for `agent prompt` refusing — and no model at all of the
one refusal the mechanism is built on.

### What was NOT changed, and why it looked tempting

- **`RelaunchAgent`'s in-place arm still refuses by name on a reported
  runtime** (ADR 0061 D3.4). The liveness read can now tell a stranded reported
  pane from a live one, which is exactly the trigger the ADR names for
  extending that arm — but the arm fires on `agent list` reporting no agent,
  which on this route is the *reporter's* silence and reads identically for a
  live CLI. The trigger is a stranded pane a pass actually meets; nothing has
  met one, so the refusal stands.
- **`blocked` is handed back as given, stale or not.** Refusing a blocked pane
  is delivery's job — `sendTextPrompt` mirrors herdr's own `agent_blocked` code
  so both routes refuse in one place by one rule — and handing a blocker back
  by name costs no keystroke, which is the only hazard D3.2 is about. Pinned
  both ways in `reportedreading_qa_test.go`.
