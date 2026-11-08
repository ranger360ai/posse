## The settle refusal had the rule and did not have it (ranger-base-x2grk)

MEASURED 2026-11-08, herdr 0.9.1 darwin-arm64, in the worktree
`posse/dinesh-posse-ranger-base-x2grk`.

ranger-base-x2grk was filed on a reading that is half right and worth
correcting in writing, because the correction is the whole implementation.

### 1. What the bead said, and the part that is not so

The bead's claim: *"The evidence is already in hand and thrown away.
awaitSettled RETURNS that detection — awaitAgent binds it as `gate` — and the
Die at dispatch.go:5828 prints only the status string. The rule id, its
region, and the known-screen set are all available at that line."*

The second half of that is right and is this bead's deliverable. The first
half is not, on the arm the incident went through.

`awaitSettled` has two exits that can answer `blocked`, and they carry
different amounts of evidence:

| exit | reached when | detection returned |
|---|---|---|
| the `Wait` arm | `agent wait` settles on a state that is not `idle`/`done` | `AgentDetection{State: status}` — **no rule** |
| the `explain` arm | `agent wait` said `idle`/`done` and the later `explain` says `blocked` | the full detection, rule and all |

`agent wait` answers with a state and nothing else —
`{"agent":{"agent_status":"blocked"}}`, no `matched_rule`, no region, no
manifest (`AgentWait`, `agentStatusFromResult`). The `until` list dispatch
passes is `idle|done|blocked`, so **a pane that is already blocked settles the
wait immediately and leaves through the first arm**, which is the arm with no
rule on it. The second arm exists for the 30ms case in its own comment — a
fresh claude drawing its trust dialog between the two calls — not for the
screen a launch arrives at.

So nothing was thrown away. The rule was never fetched on that path, and the
old `AgentDetection{State: status}` was honest about it.

### 2. herdr's `matched_rule` carries three fields and posse decoded one

```
$ herdr agent explain --file etc/herdr/agent-detection/testdata/codex/blocked-trust-directory.txt \
    --agent codex --json        # manifests staged into a throwaway XDG_CONFIG_HOME
"matched_rule": {"id":"trust_directory","priority":950,"region":"top_non_empty_lines(20)","state":"blocked"}
```

and the plain form of the same call:

```
rule: trust_directory (region=top_non_empty_lines(20) priority=950)
```

`AgentDetection.Rule` decoded `id` alone. It decodes `region` and `priority`
now, and the refusal prints all three in herdr's own key spelling, so the line
and a `herdr agent explain` run can be compared without translating between
two vocabularies. Not new in 0.9.1: the captured 0.8.0 pair in
`bootrace_test.go` carries both fields already.

### 3. So the fix is one `agent explain` on a terminal arm

`blockerEvidence` (internal/posse/blockedsettle.go) — logPromptHandBack's
bargain in the same words. The arm is terminal, `awaitAgent` dies on the next
line, so the call happens once per refused launch and never in a poll loop,
and a refused launch already costs a session nobody can use. It is not behind
the dry-run guard `d3Evidence` carries: that guard is about a `pane read`, and
an `agent explain` is already made unguarded on every poll of the ladder
directly above this one.

**The decision is not re-taken.** The status returned is `agent wait`'s, byte
for byte; the explain only fills in evidence beside it. A second instant
saying `idle` must not open a gate the first instant closed — that is the
loosening direction, and the ladder only ever lets the second read win the
other way. A disagreement therefore DROPS the evidence rather than reporting
it: a rule about a different screen attached to this status would name the
wrong screen with more confidence than the bare line it replaced.

### 4. The refusal is also a record, and was not one

ADR 0066 D1: *"Every pane-state, dialog, unknown-screen and composer-hold
reading that ends in a refusal, hand-back, settle-open, ghost retirement or
hold is recorded."* A blocked settle wrote **no record at all**, so
`scripts/readings-census.py` counted every launch posse refused on a screen
herdr DID recognize as nothing — the denominator was missing the whole
recognized half of the gate's refusals, which is also why the saving could
never be priced against it. That is an unfilled corner of D1, not a new
decision, and no ADR moves for it.

Two taxonomy calls, both by precedent rather than by invention:

- **D2, not D3.** D3 is "what was herdr looking at when it recognized
  nothing"; this screen is one herdr recognized. It is `logPromptHandBack`'s
  own line one keystroke earlier — `agent_blocked` → D2, "herdr read a dialog
  on the screen and refused" — so `blocked` → D2 and any other non-settle
  state → D1.
- **Refusal, not hand-back.** The bead's title says hand-back; `awaitAgent`
  runs BEFORE `d.claim` in `launchSession`, so there is no claim yet to give
  back. Nothing was sent and nothing was claimed, which is
  `ConsequenceRefusal`'s own definition and what the D3 record on the same
  ladder already uses.

### 5. What was deliberately NOT done

**No `looks_like`, and no pane capture.** ADR 0066 D2 names exactly two
consumers of the known-screen report, and a blocked hand-back is a third —
which the bead already said is an architecture question and not a free
choice. Filed as **ranger-base-jnmx6** (`-l architecture`, assigned richard:
ADR 0066's owner rules its own consumer list, ADR 0006 §1).

It is worth asking rather than obviously no, which is why the bead names the
measurement: the rule the line now carries names the SHAPE of the screen and
not the screen. The claude fullscreen-upsell pane reads `blocked` from
`live_blocked_form` — herdr's GENERIC form rule, which rangerhq-4mzt already
observed matches every form claude ever draws, the trust dialog included. The
known-screen table is the one reader that would say
`claude.fullscreen_upsell`. And it is not free: over herdr's 243-character
previews alone `KnownScreensIn` names 5 of 11 residue cases instead of 11
(ranger-base-qk9tr), so the row is only worth having with a capture beside it,
and the data ceiling (ADR 0050) then applies to bytes this record does not
carry today.

`TestQABlockedSettleRefusalIsRecorded` asserts the absence of both, citing
that bead — so the ruling has a pin to come back and change rather than a
silence to fill.

**Nothing is pressed at the screen** (rangerhq-4mzt, ADR 0013 §2).
`startupscreen_test.go`'s `TestDispatchAnswersNoBlockedScreen` is the pin that
says so and it is untouched; this bead is a diagnostic only.

### 6. The pins, and what each one catches

`internal/posse/blockedsettle_qa_test.go`, arm 3, beside the no-keys pin.
Every arm is about a SILENCE — a diagnostic that goes missing fails nothing:
the launch still refuses, the pass still prints a line, the suite still
passes, and the cost lands on whoever reads the line next and cannot act on
it.

MUTATION-CHECKED, 2026-11-08:

| mutation | reds |
|---|---|
| the `Wait` arm back to `AgentDetection{State: status}` | arms 1 and 2 |
| the `logBlockedRefusal` call removed | arms 2 and 3 |
| the `region=` clause removed from `WhyBlocked` | arm 1 alone |

Arm 1 also asserts the OLD line is gone — `status "blocked")`, the closing
paren hard against the status, is exactly what ranger-base-yk5qb read, and a
fix that appended its clause elsewhere while leaving that shape intact would
satisfy every other assertion in the file.

The fake herdr gained two levers for this, `explain-region` and
`explain-priority`, defaulting to `fake_region` and `1000` so that
`matched_rule` is as complete as a real herdr's on every test that reads it.
