# ADR 0084 — A prompt call that says nothing about delivery is INDETERMINATE: it is read, never spent as a hand-back

*Status: accepted 2026-11-09 (ranger-base-20cef, from ranger-base-2mqp3) ·
owner: architect · amends ADR 0011 (the claim is a lease without expiry —
this names what an unreadable delivery does to it) and ADR 0066 D1 (the
stall-verdict record names its trigger) · supersedes the first-leg half of
`TestQAFirstLegErrorWithNoCodeStillUnclaims`*

## Context

gather hands a bead back (`unclaimAfterPromptFailure`, "the claim was made
but the prompt never reached the agent") on any error from the first leg
that is not herdr's `timeout` code. ranger-base-2mqp3 closed that door for
RE-WAIT legs: an `agent wait` submits nothing, and a re-wait only starts
after a leg said the prompt landed. It left the FIRST leg, `agent prompt
--wait`, handing back on an untyped error, and asked which way that goes.

What an untyped first-leg error can be, read from the code
(herdr.go `capture`, muxrex.go `Prompt`) and herdr 0.9.1's own docs
(`herdr agent prompt --help`, `herdr --skill`, MEASURED 2026-11-09):

- herdr submits "text followed by encoded Enter as one ordered submission"
  server-side, and the caller's `--timeout` "includes submission time". A
  client child that is signalled, hangs past its deadline (`HerdrHangError`)
  or loses its server can die before, during or after that submission. The
  error carries no envelope, so **posse cannot tell which.**
- The leg is up to `PromptWaitMS` (15 min) long. An error late in it can
  arrive after a whole turn has run and settled.
- herdr says only two codes mean nothing was sent: `agent_not_ready` (no
  pane to address) and `agent_blocked` ("rejected … before any input is
  sent"). `agent_prompt_stalled` means the submission was ACCEPTED
  (promptstall.go). Nothing else on the first leg is a statement about
  delivery.

The field has a name for this. A call whose outcome cannot be known is
**indeterminate**: Jepsen records it as `:info`, "allow[ing] that it might
or might not have taken place", and converts it to `:fail` only when the
caller *knows* the value did not change
([tutorial ch. 3](https://raw.githubusercontent.com/jepsen-io/jepsen/main/doc/tutorial/03-client.md),
[docs], verified 2026-11-09). gRPC says the same of DEADLINE_EXCEEDED: it
"may be returned even if the operation has completed successfully"
([status codes](https://grpc.io/docs/guides/status-codes/), [docs], verified
2026-11-09). Prompting an agent is not idempotent and carries no key, so the
indeterminate case cannot be retried. It can only be resolved by reading the
receiver, which is what `judgeStall` already does for a stall.

## Decision

**D1 — Three outcomes, and the hand-back is an allow-list.** A prompt call
is *delivered*, *not delivered*, or *indeterminate*. Only *not delivered*
reaches `unclaimAfterPromptFailure` directly, and it is exactly
`agent_not_ready` and `agent_blocked`. Every other first-leg error is
*indeterminate*: an untyped error, a hang, a signalled child, and any herdr
code not on the list, `server_not_running` included. Adding a code to the
list needs herdr's own words that nothing was sent. The guard is written
as an allow-list (`isNotDelivered(err)`), never as "not timeout".

**D2 — Indeterminate goes through `judgeStall`'s readings, not the status
probe.** A stall is herdr saying "accepted, unobserved". An indeterminate
error says less than that, so it gets at least the second reading a stall
gets. In order:
1. re-ask herdr for a turn (`working|blocked`) on posse's clock (`stallGrace`);
2. ask git whether this session did any work (D3).

Turn under way → rewait. Blocked or anything unreadable → claim kept, not
judged this pass. No turn **and** git answered "no work" → hand back. That
keeps rangerhq-81d's property for the case it was written for: a live,
idle seat that never got the text has no turn and no work, so it is handed
back after one `startup_wait`, not stranded. Under a drain, `stopClaim` still
goes first, as 2mqp3 ordered it.

**D3 — "Did this session work?" reads the tree as well as the branch.**
`committedWork` becomes a work reading (built as `sessionWork`,
ranger-base-x475y). It is *commits ahead of base, OR a
non-empty `git status --porcelain` in the session tree*. Either one means
work happened. The branch count alone was enough for a stall: herdr watched
5s from submission and saw nothing, so a whole turn fitting inside that
window is not plausible. It is not enough here, because the error can come
up to a leg after submission. The 2mqp3 incident's own tree was this shape:
six modified files, one untracked, nothing committed. The widening applies
to stalls too, since it only moves verdicts toward keep. The shared-checkout
and unreadable arms are unchanged: (none, answered) and (?, not answered).

**D4 — The print and the record name their trigger.** `judgeStall` takes the
sentence that says why it is re-reading. For a stall it stays "herdr typed
the prompt into X and saw no turn start inside its own 5s window". For an
indeterminate call it is "the prompt call to X ended without saying whether
the text went in (<err>)". The stall-verdict record (ADR 0066 D1,
`DecisionStallVerdict`, rule `promptstall.go:judgeStall`) keeps its
decision id and rule, because this is the same decision: an unobserved
delivery judged by reading. Its `Verdict` is prefixed `stalled:` or
`indeterminate:` so the census can split the two without a new field.

**D5 — The cockpit's `d` (`LaunchBead`) obeys D1 and keeps the claim.** It
prompts without `--wait` and has no gather loop. *Not delivered* hands back
as now. *Indeterminate* keeps the claim and prints `posse peek <session>`.
The operator who pressed the key is watching. A late hand-back there would
be a second dispatch they did not ask for.

**Recorded in ADR 0011 §"Decision"** (one line, beside "Claims are never
released because a wait timed out"): *nor because a prompt call ended
without saying whether it delivered: that is indeterminate, and it is
resolved by reading the seat (ADR 0084), never by expiry.*

## Consequences

- One `if` changes shape in gather, `judgeStall` gains a trigger argument,
  `committedWork` gains a porcelain read (landed as `sessionWork`, with
  `dirtyPaths` split so that "git would not answer" stays apart from "the
  tree is clean" — ranger-base-x475y), and `LaunchBead`'s guard changes.
  There is no new state, config key or herdr call. The porcelain read runs
  only on the D2 path, once per judged bead.
- `TestQAFirstLegErrorWithNoCodeStillUnclaims` is replaced by pins on D2's
  arms: idle and clean → handed back (81d stays pinned), working → rewait,
  dirty tree → kept, herdr unreadable → kept.
- **The cost of the keep direction, stated plainly:** when the readings
  themselves cannot be taken (herdr gone, git unreadable), a bead whose
  prompt really did not land stays claimed until the operator runs
  `--resume`. ADR 0011 already chose that: a visible stranded claim is
  cheaper than an invisible second seat. MEASURED in 2mqp3, the second seat
  cost a full duplicate of one bead and wrong provenance on one of two
  branches.
- **MEASURED 2026-11-09 (ranger-base-x475y), replacing this bullet's
  ASSUMED:** a posse-prepared session tree is clean at prompt time, on BOTH
  prepare paths — the per-session worktree and the container tier's detached
  HEAD (`PrepareSessionHead`). Zero porcelain lines
  (`--untracked-files=all`, stricter than the reading itself takes) over a
  tree carrying the seeded scaffolding a fresh one gets, and over four
  relaunches alternating the tier. Nothing had to be excluded:
  `seedScaffoldExcludes` (ranger-base-e01op) already hides the launcher's own
  files from git, and the two files the caged prepare writes — `<common>/logs`
  and `<gitdir>/config.worktree` — are inside the git dir where no
  working-tree status can reach them. Pinned, not left as prose, in
  `TestPreparedSessionTreeIsPorcelainCleanOnEveryPreparePath`: were a third
  prepare-time create to leave a file in the tree, this reading would find
  posse's own file and keep every claim forever, which is rangerhq-81d lost
  with nothing printed. A live reading agrees (a dispatched claude seat's own
  tree, uncaged, after a session's bd calls: zero lines). A RESUMED session's
  tree may still be dirty from an earlier turn; that reads as work and keeps
  the claim, which is the safe direction.

## Alternatives rejected

- **Keep handing back (today).** This treats `:info` as `:fail`. When the
  text did land, it is 2mqp3's duplicate dispatch arriving through the first
  leg instead of the re-wait.
- **Straight to the status probe, like a timeout.** The probe's `idle/done`
  arm reads "the prompt plainly landed" and judges a settle. For an
  indeterminate call that sentence is false. A seat that never got the text
  would be judged settled-without-close and kept claimed with nobody working
  it. That is rangerhq-81d with no exit, plus a false settle in the
  seat-idle ledger. A timeout earns that arm because herdr took the text.
  Here nobody knows that.
- **Read herdr's `done` vs `idle` as "a turn ran".** It looks clever.
  herdr 0.9.1's docs say the two differ only by the server's *seen* state:
  "explicit focus commands mark the target seen", and the state is per
  server. An operator's `posse attach` erases the evidence. So does the
  replaced server that is one of the very causes of an indeterminate error.
  Git keeps its evidence across both.
- **Retry the prompt.** The prompt is not idempotent and has no key. A
  second copy typed into a working seat is a second instruction.
- **Hand back and kill the old seat (STONITH).** That makes the expiry safe
  by fencing the holder. But the kill goes through the same herdr that just
  failed to answer, so the kill itself may be indeterminate, and it throws
  away the work it exists to protect.
- **Make the Rex occupant type which side of Enter its error fell on.**
  `RexMux.Prompt` builds delivery from separate steps, so it could know.
  Not cut: D2 is already correct for every such error. The only gain would
  be a faster hand-back for the leading-`/` refusal, and a work prompt never
  starts with `/`. Reopen if Rex grows a step that can fail before Send in
  normal use.
