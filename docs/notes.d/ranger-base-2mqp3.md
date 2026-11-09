# A failed wait is not a failed prompt (ranger-base-2mqp3)

**2026-11-09.** Two seats worked one bead, each committed the same change on
its own branch, and the only defect was one word in a guard.

## What happened

From the record on ranger-base-bu8rz, same day:

- **13:57** — a dispatch pass commented on that bead: *"posse dispatch
  unclaimed this bead: the prompt to `jian-yang-posse-ranger-base-bu8rz`
  failed — `herdr agent wait w36W:p1 --until idle --until done --until
  blocked --timeout 900000`: signal: interrupt."* The bead went open and
  unassigned and was re-dispatched to a second persona.
- **14:20** — the FIRST seat commented on the same bead with a full
  implementation and a commit, `e7bcf63e`. It had been working the whole 23
  minutes and had seen the reassignment happen under it: its comment reads
  *"When this session started, `bd show` read `Assignee: jian-yang`. It now
  reads `Assignee: dinesh`."*
- The second seat found that tree holding uncommitted work with no commit on
  its branch and reasonably read it as an interrupted session about to be
  retired with the tree. It adopted the work, verified it, and committed it
  as `c2ed8024`.

Two branches, one bead, byte-identical single commits on the same base. Only
one can be fast-forwarded onto `main`, the bead's close only triggers the
assignee's, and the provenance in whichever does not land is wrong —
`c2ed8024`'s own message states that the tree it adopted from "would have
been retired", which the 14:20 comment then falsified.

## The mechanism

`signal: interrupt` on `herdr agent wait` says the WAIT was interrupted. It
is not evidence about the agent, and it CANNOT be evidence about delivery,
because `agent wait` submits nothing. The prompt had gone in fifteen minutes
earlier and the pass had already watched it land: a re-wait only starts after
a leg came back `timeout` over an agent the pass's own status probe read
`working`, or after `judgeStall` found a turn under way.

What let it through is the shape of gather's hand-back guard:

```go
if !IsHerdrCode(r.err, "timeout") && !p.delivered {   // before
```

A deny-list of one, reading a contract that is an allow-list —
`unclaimAfterPromptFailure` is "the claim was made but the prompt never
reached the agent", and the codes that say that are `agent_not_ready`,
`agent_blocked` and (after a second reading) `agent_prompt_stalled`. Every
error with no herdr envelope behind it — a signalled child, a replaced
server, a hang — fell through and was spent as a failed prompt. `delivered`
was the only exemption, and it means "the prompt rode the launch line" (ADR
0013 §2), which is false on every `prompt: typed` runtime including claude.

A bd claim is a lease WITHOUT expiry by ADR 0011's own decision, precisely
because expiry without a fencing token the resource checks is
double-dispatch. Unclaiming a live seat on a wait error is that expiry,
reintroduced through an exec failure.

## The fix

Two changes in `gather`, both narrowings of one `if`:

1. **`pendingBead.rewaited`** — set by `rewait()`, it is the typed path's half
   of what `delivered` already states for the launch line. Once a leg has
   come back and been re-waited, delivery is settled: the leg is an `agent
   wait` (submits nothing) and it only started because an earlier leg said
   the prompt landed. Either reason alone is sufficient; the field's doc
   carries both.
2. **The drain moved above the hand-back.** A SIGINT reaches the whole
   process group, so the signal that stops a watch loop also kills the `herdr
   agent wait` child it is holding, and that child's death arrives in gather
   as exactly the error above. `stopClaim`'s own doc already stated the rule
   — "a drain is not evidence about the agent, and the agent is still
   working" — but the check sat one line BELOW the unclaim, where its comment
   called itself "defensive rather than load-bearing". True of the timed-out
   leg it was written for, since the select guarantees the exit; false of
   every other error.

An error past either guard now reaches the status probe, which is the only
thing that ever said whether work was happening (rangerhq-1z0). The claim is
kept and the loop goes round, paced by `LegFloor` (ranger-base-74tkp).

**rangerhq-81d is intact and that is the point of the design**: the same
untyped error on the FIRST leg still hands the bead back, because nothing yet
says the prompt landed. The discriminator is WHICH LEG failed, never which
error it was.

> **REFINED by [ADR 0084](../adr/0084-a-prompt-call-with-no-answer-about-delivery-is-indeterminate-and-is-read-not-spent.md)**
> (ranger-base-20cef, built as ranger-base-ng8wz), which answers the
> question this bead left open and narrows the sentence above rather than
> overturning it. The discriminator is still which leg failed — a re-wait
> leg's error is not evidence about delivery at all, and that half is
> unchanged — but a FIRST leg's untyped error is not evidence *either*: herdr
> submits the text server-side and the client child can die before, during
> or after that submission with no envelope to say which, so the call is
> INDETERMINATE. "Still hands the bead back" is therefore now "is READ before
> it can be spent": judgeStall's two readings, exactly as a stall, and only
> "no turn in the grace AND no work in this session's branch or tree"
> reaches the unclaim. 81d's own case — a live, idle seat with a clean tree —
> still goes back after one `startup_wait`, which is the arm that keeps it.
> The hand-back is an allow-list now (`isNotDelivered`: `agent_not_ready`
> and `agent_blocked`, herdr's own words that nothing was sent) and must
> never be written as "not a timeout" again.
> `TestQAFirstLegErrorWithNoCodeStillUnclaims` was replaced in place by
> `TestQAFirstLegErrorWithNoCodeIsJudgedNotSpent`; D2's full set of arms is
> `internal/posse/indeterminateprompt_qa_test.go`.

## What this supersedes

`TestDispatchRewaitServerGoneHandsTheBeadBack` (now
`…KeepsTheClaim`) pinned the old behaviour for `server_not_running` — the
code a replaced server answers every leg with after `herdr update --handoff`
(MEASURED on real herdr 0.8.0 under ranger-base-7t4 / ranger-base-kcr). Its
own comment described that hand-back as *"the rangerhq-khc failure mode
arriving through the one door the khc fix does not cover"*, with the
mitigation pushed out to the upgrade runbook's "wait for the in-flight
prompts to drain". It was a characterization of a defect, not a decision, and
it said so. Same door, same fix: that bead is now kept-claim too, and the
runbook's `grep server_not_running` post-flight names beads to WATCH rather
than beads to re-claim by hand.

The last paragraph of `docs/notes.d/ranger-base-74tkp.md` carries a pointer
here, since its closing sentence ("On the typed path only the `timeout` code
reaches the same arm") was the defect stated as a fact.

## Pins

`internal/posse/rewaiterrorclaim_qa_test.go`, arm 1, three arms of one guard:

| pin | leg | verdict |
|---|---|---|
| `TestQARewaitLegErrorWithNoCodeKeepsTheClaim` | re-wait, through a real leg | claim kept |
| `TestQAFirstLegErrorWithNoCodeStillUnclaims` | first | handed back (81d) — replaced by `…IsJudgedNotSpent`, read then judged (ADR 0084) |
| `TestQADrainDoesNotUnclaimOnAKilledWaitChild` | first, loop stopping | claim kept |

The first is driven through a real leg rather than a hand-fed result so that
it also pins `rewait()` writing the flag — a fix that guarded on the field but
never set it would pass the hand-fed arms and fail that one. Both the re-wait
and the drain pin were confirmed to go red with their half of the fix
reverted; the error they print is the incident's own
(`herdr agent wait … : signal: interrupt — unclaimed`).

The fake herdr gained one lever for this: **`wait-crash`**, an `agent wait`
that exits nonzero with nothing on stdout and nothing on stderr. That is the
shape `wait-error` cannot make — it always writes an envelope — and it is the
shape of the incident, since `HerdrCodeOf` of such an error is `""` and every
guard that branches on a code reads it as "not that code".
