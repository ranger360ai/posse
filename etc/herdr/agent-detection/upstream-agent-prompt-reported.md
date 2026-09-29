# herdr: `agent prompt` refuses a pane whose agent was reported through `pane report-agent`

**Not filed.** This is a draft for the operator to send; nothing here has been
published. See `README.md` in this directory. Not a manifest question — it is
about the `agent prompt` verb — but it lives here because this is where posse
keeps the drafts it means to send herdr. (posse-side record: bead
ranger-base-8eqaa, and `docs/notes.d/ranger-base-8eqaa.md` in the posse repo.)

- herdr 0.9.1, client and server (macOS 26.4.1 / darwin 25.4.0)
- reproduced with no agent CLI at all — see the recipe, which uses a scratch
  pane and a label of no significance

## Summary

`pane report-agent` is the documented "integrate your own agent" route, and
0.9's plugin surface makes it the route a plugin takes to be a pane's status
authority. A pane labelled that way is labelled, listed and stateful: `agent
get` and `agent list` describe it, `agent wait --until working` returns on a
reported transition, and a reported working→idle lands as `done`.

**`agent prompt` refuses it anyway**, with `agent_not_ready: "agent <pane> is
not an active named agent"` — in every reported state, `idle` and `done`
included. So an orchestrator can watch such an agent but cannot speak to it
through the API that exists for speaking to agents, and has to fall back to
`pane send-text` + `pane send-keys enter`, reimplementing `agent prompt`'s
blocked-rejection and its 5000ms observed-turn rule by hand to keep the same
error codes.

## Recipe (no agent CLI needed)

```
$ herdr workspace create --label probe --no-focus --cwd /tmp
  → root_pane w2KJ:p1

$ herdr pane report-agent --source probe --agent fakebob --state idle w2KJ:p1
  → exit 0

$ herdr agent get w2KJ:p1
  {"agent":{"agent":"fakebob","agent_status":"idle","pane_id":"w2KJ:p1",…}}

$ herdr agent prompt w2KJ:p1 "echo hello"
  {"error":{"code":"agent_not_ready",
            "message":"agent w2KJ:p1 is not an active named agent"}}

$ herdr pane send-text w2KJ:p1 "echo hello" ; herdr pane send-keys w2KJ:p1 enter
  → lands, every time
```

## The ask

`agent prompt` should accept a pane whose agent label was reported through
`pane report-agent`, on the same terms it accepts a detected one: submit the
text, press Enter, and apply the existing `--wait` contract (reject
`agent_blocked` before sending; require an observed working-or-blocked state
within 5000ms of an accepted submission, else `agent_prompt_stalled`). Every
one of those states is already available for a reported pane through `agent
wait`, so the contract needs no new observable — only the readiness check in
front of the verb needs to admit a reported label.

## A smaller version of the same ask

If accepting a reported pane is not wanted, the refusal should at least be
distinguishable from a detected agent that is genuinely not ready yet. Today
both are `agent_not_ready`, so a caller cannot tell "this kind cannot be
addressed by this verb, ever" from "this agent is mid-boot, try again" — and
those need opposite responses: one is a permanent route change, the other is a
retry. A distinct code (`agent_not_promptable`, say) would be enough.

## Related

`agent explain` refuses the same panes with `agent_explain_unavailable:
"agent target <pane> does not have a detected agent label"`. That one is
arguably correct — there is no manifest and no rule to explain — but it means
a reported pane has no screen reading at all, so any readiness gate built on
`agent explain` is blind to it. Worth saying in the same breath if the
readiness check above is revisited.
