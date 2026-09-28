# ranger-base-8eqaa — typed prompt delivery to a pane herdr labels but does not detect

*Measurements behind `internal/posse/reportedagent.go` and the reported-agent
arm of `internal/posse/promptready.go`. Sits under ADR 0013 §2 (delivery) and
ADR 0060 D5 (the `/` hazard), whose snapshot paragraph points here.*

## The shape

herdr's `agent prompt` verb addresses only the agent kinds herdr itself
detects. A pane labelled through `herdr pane report-agent` — herdr's
documented "integrate your own agent" route, and the route every herdr
PLUGIN takes — is labelled, listed and stateful, and `agent prompt` refuses
it anyway. That is what `posse prompt` met on a Bob session under the
MartinLoeper/herdr-bob plugin (ranger-base-p8afi):

```
posse: herdr: agent w2KD:p1 is not an active named agent (agent_not_ready)
```

…in state `done` AND in state `idle`, and identically from `herdr agent
prompt` by hand. The plugin was reporting correctly the whole time.

## MEASURED 2026-09-28 · this box · herdr 0.9.1 (client and server)

Reproduced with no Bob and no plugin at all, against a scratch workspace and
a label of no significance — which is the point: the failure is a property of
the REPORT, not of that runtime. Recipe, verbatim; the workspace was closed
afterwards (`herdr workspace close w2KJ`).

```
$ herdr workspace create --label posse-8eqaa-probe --no-focus --cwd /tmp
  → root_pane w2KJ:p1, agent_status "unknown"

$ herdr pane report-agent --source posse-probe --agent fakebob --state idle w2KJ:p1
  → (no output, exit 0)

$ herdr agent get w2KJ:p1
  {"agent":{"agent":"fakebob","agent_status":"idle","pane_id":"w2KJ:p1",…}}

$ herdr agent explain w2KJ:p1 --json
  {"error":{"code":"agent_explain_unavailable",
            "message":"agent target w2KJ:p1 does not have a detected agent label"}}

$ herdr agent prompt w2KJ:p1 "echo hello"
  {"error":{"code":"agent_not_ready",
            "message":"agent w2KJ:p1 is not an active named agent"}}

$ herdr agent list
  … fakebob w2KJ:p1 idle   (agent_session: ABSENT — see below)

$ herdr agent explain --file <empty> --agent fakebob --json
  {"agent":"fakebob","fallback_reason":"unknown_agent","manifest_version":null,…}
$ herdr agent explain --file <empty> --agent claude  --json
  {"agent":"claude","cached_remote_version":"2026.09.11.1",…}   (a manifest)

$ herdr pane send-text w2KJ:p1 "echo POSSE_8EQAA_LANDED"
$ herdr pane send-keys w2KJ:p1 enter
$ herdr pane read w2KJ:p1 --format text
  ➜  tmp echo POSSE_8EQAA_LANDED
  POSSE_8EQAA_LANDED
  ➜  tmp
```

And the two waits the delivery leans on, each with a `report-agent` fired
from a second shell two seconds in:

```
$ herdr agent wait w2KJ:p1 --until working --timeout 5000
  {"agent":{"agent":"fakebob","agent_status":"working",…}}      2.02s
$ herdr agent wait w2KJ:p1 --until idle --until done --until blocked --timeout 8000
  {"agent":{"agent":"fakebob","agent_status":"done",…}}         2.02s
```

Both answer in the `result.agent.agent_status` shape `agentStatusFromResult`
already reads, so a caller's judgement of the settle is unchanged by the
route. Note the second one: a reported working→idle lands as `done`, matching
ADR 0060's Context 3.

## Why the discriminator is the LABEL, not the session source

The obvious-looking discriminator is `agent_session.source`, which reads
`herdr:claude` / `herdr:codex` on a detected pane. It does not work: a
reported pane has **no `agent_session` at all** unless the reporter also
passed `--agent-session-id`, so the field is absent on both a plugin that
does not report one and a pane herdr simply has no session for. Measured
above (`agent list` row for `fakebob`: no `agent_session` key).

What does work is the pair posse already asks elsewhere: the pane's agent
LABEL (`agent get`), and whether herdr has a detection manifest for that
label (`AgentManifest`, the same reading `RuntimeGaps` takes, which resolves
an `aliases = […]` route too). A label herdr has been asked about and does
not know was put there by somebody else.

## What the route probe costs

Two herdr calls per decision, MEASURED 2026-09-28 on this box, 3 runs each:
`agent get` ~11ms, the `agent explain --file --agent` manifest probe ~65ms.
A hand `posse prompt` takes the decision twice — once in the gate, once in
the delivery — so it pays ~150ms it did not pay before, against a call that
then waits on a CLI. Nothing is cached, deliberately: a memo keyed on the
label would answer "herdr does not know this one" for the life of a
`dispatch --watch` process, which is exactly the answer that should change
the day herdr learns the kind.

## The number in the delivery is herdr's, not ours

`herdr agent prompt --help` on 0.9.1 states the contract the typed route
mirrors, verbatim:

> If the agent is already blocked, submission is rejected with agent_blocked
> before any input is sent. When an accepted submission starts from another
> non-working state, --wait requires an observed working or blocked state
> within 5000ms; otherwise it returns agent_prompt_stalled. A caller timeout
> that expires first returns timeout. It then matches idle, done, or blocked
> by default, or any exact --until state. It does not track turns: if the
> agent is already working, that active turn's completion may match.

So `herdrPromptStallMS` is quoted rather than chosen, and every clause above
has a line in `sendTextPrompt`. The reason to mirror it rather than invent
something simpler is that dispatch already branches on these codes:
`agent_prompt_stalled` and `agent_blocked` mean "the prompt never reached the
agent" and hand the claim back, while `timeout` means herdr took the text and
stopped watching and the bead stays claimed (rangerhq-1z0). A route that
settled on silence, or that reported a bare timeout for a turn that never
started, would hand those branches a different meaning for the same code.

## What is NOT here

- **No runtime name.** Nothing in the implementation reads a runtime, and no
  test names bob. ADR 0060 rejected a report-agent adapter partly as "two
  mechanisms, both keyed on one runtime's name" (the ADR 0017 §3
  shadow-predicate class); the discriminator here is a property of the pane,
  asked of herdr, so a template runtime whose detection arrives from another
  plugin tomorrow is delivered to without a line changing. The day herdr
  compiles the kind in, `AgentManifest` answers `known` and the route stops
  being taken — nothing to prune, which is also why D2's tripwire pin needs
  no amendment.
- **No reporter.** posse still runs no watcher and keeps no state current
  between passes — the "daemon in costume" ADR 0060 declined to build. The
  plugin is the operator's, installed by hand.
- **No read-back.** `ConfirmSubmitted` rests on `agent explain`, which
  refuses a reported pane, so it returns "" and warns nothing. That is the
  concession it already makes for an unreadable diagnostic, not a new gap —
  and it is why the `/` guard is a refusal on this route rather than a
  warning after the fact: on this route nothing else would ever say so.

## The companion upstream ask

Drafted, unsent, the operator's to file:
`etc/herdr/agent-detection/upstream-agent-prompt-reported.md`. It asks herdr
for `agent prompt` to accept a reported pane on the same terms it accepts a
detected one, and — as the smaller version of the same ask — for a code that
distinguishes "this kind cannot be addressed by this verb, ever" from "this
agent is mid-boot, try again", which today are both `agent_not_ready` and
need opposite responses. The reproduction in it needs no agent CLI at all,
which is why it was worth writing generically.

## Left for somebody else

- The launch row still refuses a DISPATCHED session on a runtime herdr has
  no manifest for (`RuntimeGaps`, detection, blocking). That is ADR 0060 D1
  and ADR 0013 §1, and it is untouched here: what this bead fixes is the
  HAND path (`posse prompt`, `posse cockpit`'s `p`, the pulse nudge) on an
  interactively launched session, plus dispatch's delivery for the day the
  launch row opens. Whether a plugin report is enough to lift the launch
  refusal is a design question, not a delivery one.
- `blocked` never fires from the herdr-bob plugin on Bob's approval dialog
  (ranger-base-p8afi, finding 1, upstream). Until it does, a blocked Bob pane
  reports `working` and the delivery's `agent_blocked` arm is unreachable on
  that runtime — correct, and waiting on somebody else's fix.
