# ADR 0013 §1 / turn_outcome: in-store refusal artifact probe

Measured for `ranger-base-e123`. This is a trace, not fleet code — the
promotion rule (ADR 0013 §1, `internal/rhq/turnfailure.go`'s header) is
that a `turn_outcome:` reader joins the registry only after the refusal
artifact in that runtime's *own* session store is captured and pinned as
a fixture; a reader built over a guessed shape is the
`probe-needs-a-failing-wrong-arm` class of undiscriminating pin.

The question: when an account refuses the first turn, does codex or
grok's own session store record something distinguishable from an
ordinary settled turn — and if so, where does that fact live?

## grok — MEASURED, discriminates

grok's account was exhausted 2026-08-26 through 2026-08-28 (six separate
probes across ranger-base-cl7/unzn/xaev all hit the same 402). One of
those probes — `laurie`'s live-state check on `ranger-base-unzn`,
2026-08-28T21:01:35Z, `grok --leader-socket <scratch> -p "Reply with
exactly: OK"` — left its session store artifact on disk, so this bead
reads it rather than re-spending to reproduce it: the refusal record was
already free. Session dir:
`$GROK_HOME/sessions/<cwd>/<id>/` where `<id>` =
`01a04a2d-8c8b-7811-a6da-edf99f567e7b`.

To know what a *served* turn looks like in the same store for
comparison, this bead ran one live control probe today
(2026-08-31T02:02:05Z, same `-p "Reply with exactly: OK"` shape, scratch
dir, scratch leader-socket): the grok account has recovered since
2026-08-28 and served it (14219 input / 33 output tokens, mostly cached
— the cheapest turn the runtime sells, same as `probeAccount`'s own
docstring promises). `<id>` = `01a0558d-6255-7cc2-9e70-1a734338c794`.
That is the only spend this bead made; it was necessary to get a
same-day, same-shape control arm rather than trusting an assumed
"success looks like X".

**Where the refusal shows up — `events.jsonl` and `updates.jsonl`, NOT
`chat_history.jsonl`.** Unlike claude, which writes the refusal as a
synthetic *assistant* message inside its own transcript
(`turnfailure.go`'s whole mechanism), grok's chat transcript stays
silent: a refused turn's `chat_history.jsonl` ends at the user's
`<user_query>` entry with no `reasoning` or `assistant` record at all
(4 entries; `summary.json`'s `num_chat_messages` agrees: 4 refused vs 6
served — the served run adds a `reasoning` and an `assistant` entry). A
reader that only scans the transcript, the way `FindClaudeTurnOutcome`
scans claude's, would see nothing — an `observed=false`, not a
distinguishing signal.

The refusal is recorded as **structured, typed JSON**, not prose to
pattern-match:

`events.jsonl`, refused (4 lines total, ends here — no `first_token`, no
`streaming_*` phase):

```json
{"ts":"2026-08-28T21:01:35.814Z","type":"turn_started","session_id":"01a04a2d-8c8b-7811-a6da-edf99f567e7b","turn_number":0,"model_id":"grok-4.6","yolo_mode":true,"conversation_message_count":3,"session_relationship":"primary","schema_version":"1.0"}
{"ts":"2026-08-28T21:01:35.992Z","type":"loop_started","loop_index":0}
{"ts":"2026-08-28T21:01:35.993Z","type":"phase_changed","phase":"waiting_for_model"}
{"ts":"2026-08-28T21:01:36.051Z","type":"turn_ended","outcome":"error"}
```

`events.jsonl`, served (29 lines; several `streaming_reasoning` /
`streaming_text` `phase_changed` rows and a `first_token` row elided —
full file in the bead's worktree scratch, not repo-committed):

```json
{"ts":"2026-08-31T02:02:05.664Z","type":"turn_started", … }
{"ts":"2026-08-31T02:02:05.823Z","type":"loop_started","loop_index":0}
{"ts":"2026-08-31T02:02:05.823Z","type":"phase_changed","phase":"waiting_for_model"}
{"ts":"2026-08-31T02:02:06.469Z","type":"first_token"}
… streaming_reasoning / streaming_text …
{"ts":"2026-08-31T02:02:06.900Z","type":"turn_ended","outcome":"completed"}
```

`events.jsonl`'s last line's `outcome` field is `"error"` vs
`"completed"` — a one-field, purpose-built discriminator.

`updates.jsonl`, refused, has two entries a served run never gets:

```json
{"timestamp":1787950896,"method":"_x.ai/session/update","params":{"sessionId":"01a04a2d-8c8b-7811-a6da-edf99f567e7b","update":{"sessionUpdate":"retry_state","type":"failed","error_type":"api","message":"API error (status 402 Payment Required): Grok Build usage balance exhausted"}, …}}
{"timestamp":1787950896,"method":"_x.ai/session/update","params":{"sessionId":"01a04a2d-8c8b-7811-a6da-edf99f567e7b","update":{"sessionUpdate":"turn_completed","prompt_id":"1fc58438-563d-4840-830f-0c430559dbd9","stop_reason":"error","agent_result":"API error (status 402 Payment Required): Grok Build usage balance exhausted"}, …}}
```

vs the served run's `turn_completed`:

```json
{"update":{"sessionUpdate":"turn_completed","prompt_id":"46b2125d-20b8-4781-be2f-bd1e51aa7a9f","stop_reason":"end_turn","usage":{"inputTokens":14219,"outputTokens":33,"totalTokens":14252, … }}}
```

Three independent fields discriminate a refusal, in order of how load-bearing
they'd be for a reader:

1. `updates.jsonl`'s last `turn_completed` update: `stop_reason` is
   `"error"` (refused) vs `"end_turn"` (served) — and a served turn's
   `turn_completed` always carries a `usage` object; a refused one never
   does.
2. A refused turn's `turn_completed` carries `agent_result` — the
   verbatim provider error string (`"API error (status 402 Payment
   Required): Grok Build usage balance exhausted"`), grok's own quoted
   text — where a served turn's `turn_completed` has no `agent_result`
   field at all.
3. An extra `retry_state` update with `"type":"failed"` precedes the
   `turn_completed` on a refusal; nothing named `retry_state` appears on
   a served run.

`system_prompt.txt` (5779 B both), `prompt_context.json`, and
`rewind_points.jsonl` are **not** discriminating — all three are written
before the API call and are byte-identical in shape between the refused
and served arms. `summary.json` differs only indirectly (`num_messages`
4 vs 5, `num_chat_messages` 4 vs 6) and would be a weak, coincidental
signal on its own.

**Verdict: grok's session store discriminates a refusal, cleanly and
structurally** — the artifact to read is `updates.jsonl`'s last
`sessionUpdate":"turn_completed"` record, keying off `stop_reason` (or
equivalently `events.jsonl`'s last `turn_ended.outcome`), with
`agent_result` carrying the message a reader would surface. A reader
bead follows this probe: ranger-base-<see e123's comments for the id>,
`-a dinesh -l code`.

## codex — not captured; stays trigger-shaped

codex's account was alive on 2026-08-28 (`ranger-base-unzn`: `codex exec
--sandbox read-only 'Reply with exactly: OK'` → served, 4162 tokens) and
was not re-probed by this bead: e123's own instructions are explicit —
*"codex's account was alive the same day, so its half is trigger-shaped:
capture when it happens, do not spend to cause it."* There is no way to
force a real account refusal without either spending until the quota
trips or fabricating one, and neither is this bead's to do. codex's half
stays open until an ordinary fleet dispatch pass hits one; when it does,
`~/.codex/sessions/*.jsonl` (the same rollout file
`FindClaudeTurnOutcome`'s codex analogue would read) is the store to
capture from.

## No-spend accounting

One real turn was spent: the control probe above
(`grok --leader-socket <scratch> -p "Reply with exactly: OK"`, ~14.2k
input tokens almost entirely cache reads, 33 output tokens) — needed to
know what "served" looks like in the same store on the same day, rather
than trusting a two-day-old assumption. The refusal artifact itself cost
nothing: it was already on disk from a prior probe, read only. No codex
turn was spent by this bead.
