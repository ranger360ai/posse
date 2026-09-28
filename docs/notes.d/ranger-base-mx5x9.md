## Reported and detected labels do not expire the same way (ranger-base-mx5x9)

The two upstream asks ADR 0061 D4 names were drafted under this bead. Before
drafting them the record's facts were re-measured independently, because a
filing upstream is read by people who will run its recipe, and one of the
asks rests on a comparison the ADR marked **ASSUMED 3** ("herdr drops a
*detected* label when argv0 leaves the pane; not re-measured here"). It is
measured now, and it holds — with one wrinkle.

MEASURED 2026-09-28, herdr 0.9.1 client and server, darwin 25.4.0, two scratch
workspaces (`--no-focus`, closed afterwards), **no agent CLI, no plugin and no
Bob involved**.

### 1. herdr detects from `argv0`, so the whole comparison needs no agent CLI

`(exec -a codex sleep 25)` in a scratch pane — `argv0` `codex`, executable
`sleep` — is read by herdr as `agent: codex, agent_status: idle`.
`pane process-info` reports both fields separately (`argv0 "codex"`, `name
"sleep"`), which is what makes the substitution visible rather than a guess.
This is the cheap rig for anything about label lifecycle: no credentials, no
network, no turn.

It is also, read the other way, the reason ADR 0061 rejected "liveness by
matching argv against the label" — the label's own source is argv0, so argv is
not independent evidence about it. Nothing here changes that decision; the
shell-pid comparison D3.2 uses is still the one that needs no name.

### 2. A detected label IS released when the process leaves the foreground

After the 25s exits, with the pane at its shell prompt:

| verb | answer |
|---|---|
| `pane get` | `agent: null` (`agent_status: idle`, then `unknown`) |
| `agent get` | `agent codex, idle` for a few seconds, **then** `agent_not_found` |
| `agent list` | the pane is not listed |
| `agent explain` | `agent_not_found` |
| `agent wait` | `agent_not_found` |

So ASSUMED 3 is now MEASURED and true. The wrinkle is the second row: `agent
get` lags `pane get` by a few seconds, during which four verbs say the agent is
gone and `agent get` still describes it. Nothing posse does depends on that
window — the detected route's gate is `Seen()` over `agent explain`, which is
already in the `agent_not_found` column — but it is in the herdr filing, because
a caller that polls `agent get` alone would read a state for a pane that has no
agent.

### 3. A reported label is released by nothing

Same box, same hour. A pane reported `working`, running `sleep 6`, read ~10
minutes after that sleep exited: `agent get` and `pane get` both still
`fakeqa`, and the state moved only when the reporter moved it. `revision` went
2 → 3 across the process's death and `state_change_seq` did not — herdr re-read
the pane and did not reconsider the state, so this is a value with no expiry
rather than a cache that catches up.

The sharpest single read: in the same JSON object that said `agent_status:
working`, herdr's own `terminal_title` had already reverted from `sleep 6` to
the shell's prompt string. herdr holds the contradiction in one row.

A reported `--state unknown` keeps the label and is deliberately not settled
(`agent wait --until idle --until done --until blocked` → timeout), so it is a
safe thing for a reporter to say when it loses track — which is what makes it
the cheap half of the plugin ask.

### 4. `pane report-agent` takes four fields and surfaces none

`--source`, `--agent-session-id`, `--agent-session-path` and `--message` are all
accepted (exit 0) and none of them appears in `agent get`, `agent list` or
`pane get`. So ADR 0061 D4(b) is narrower than "add a source field": the source
is already in the write API, and the ask is to expose what herdr is already
given. Until it does, "this label was reported" is inferable only from `agent
explain` answering `agent_explain_unavailable` — an error code used as a type
discriminator, which is exactly the shape 8eqaa's discriminator has.

### 5. `process-info` already carries the signal, at 0.01–0.02s

`foreground_process_group_id` sits beside `shell_pid` in the same read
(`/usr/bin/time -p`, three runs: 0.01, 0.02, 0.01 — the ADR's 0.00 rounded from
a different `time`). So D3.2's guard costs what the ADR says it costs, and the
herdr-side ask ("scope the label to the foreground process group") is asking for
a comparison against a field herdr already fills.

### What did not change

D3.2 stands exactly as accepted: posse guards this on its side, and after either
upstream fix it is a redundant read that costs a hundredth of a second and stays.
Nothing measured here moves a decision — fact 1 of the record is confirmed, the
one assumption it leaned on is discharged, and the asks got narrower.

### The drafts

Both live beside the existing herdr-bob findings in the instance's private tree,
because they describe one box's install: `ranger-base/docs/drafts/
herdr-bob-stale-label-release.md` (ask a, to the plugin) and
`ranger-base/docs/drafts/herdr-reported-label-scope-and-source.md` (ask b, to
herdr). Neither is filed — sending them is the operator's. The third,
`agent prompt` refusing a reported pane, was already drafted under 8eqaa and
lives with the detection drafts here:
`etc/herdr/agent-detection/upstream-agent-prompt-reported.md`.
