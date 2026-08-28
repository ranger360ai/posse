## The count belongs in bd, not in a fifth store (ranger-base-9hm)

ranger-base-9hm asked for one thing and proposed a mechanism for it. The
thing shipped; the mechanism did not, and the substitution is the only part
worth writing down.

**The ask.** `--resume` re-prompts an in_progress bead whose persona settled
idle without closing it. Right the first time — ranger-base-f0g measured
three agents that had finished and simply never said so, and one nudge
cleared all three. Wrong the second time: the same bead settling open again
is a standing disagreement between the agent (believes it is done) and the
bead (says in_progress), and the same prompt every pass is an infinite
polite retry. So: on the second one, stop and file a question for the
operator, dep-added to the stuck bead so it leaves `bd ready`.

**The proposed mechanism.** "Dispatch keeps no cross-pass state today;
`$StateDir` already holds `overflow.log` and `dispatch-watch.pid`, so a
small resume ledger beside them is in the existing idiom."

### Why the ledger is the wrong half of that idiom

`$StateDir` does hold append-only logs, and one of them —
`seat-cadence.log` — was written three days earlier with a long comment
explaining why it was *allowed* to exist:

> It DECIDES NOTHING. No guard reads this ledger, no launch is refused on
> it, no cap counts it … That is also what keeps it from being the "one more
> small store" ADR 0011 warns about: a store that is never read back into a
> decision adds no way for two stores to disagree about a fact.

A resume ledger fails exactly that test. It would be the first file in
`$StateDir` whose contents decide whether a bead is dispatched — which puts
it squarely in ADR 0011's own incident class, *dispatch infers cross-store
facts from single-store snapshots*. And the fact being counted is not a fact
about dispatch at all. "Has this bead settled open before, with the bead
saying what it says now" is a fact **about the bead**, and bd is where the
bead's facts live. Counting it in bd is one store. Counting it in
`$StateDir` is two stores that can disagree about one bead, plus a clearing
rule ("cleared when the bead closes or its status changes") that only exists
because the count was put somewhere the status change cannot reach it.

In bd there is no clearing rule to write. The marker carries the status it
counted against — `settled open [in_progress]: …` — so a bead whose status
moved is a different disagreement and starts its own count, for free, and a
bead that closes is never dispatched again.

The comment earns its keep twice more: it is the evidence the next human
reading that bead needs, and the re-prompted persona reads its own bead's
comments in the work prompt, so the second nudge arrives with the first one
attached to it.

### The failure directions, chosen

Every write here fails toward re-prompting and away from escalating, which
is the safe direction for a mechanism whose whole job is to spend a human's
attention:

- `bd comments` unreadable → not counted, one more nudge, said on stderr.
- the marker comment fails to write → the next settle-open counts as the
  first, one more nudge.
- `bd create` fails → no escalation this pass, and `--resume` re-prompts as
  it did before.

### Idempotence is keyed on the title, not on a second write

gilfoyle flagged idempotence as "the part that will bite", and the obvious
key — comment `escalated: <qid>` on the stuck bead after the create returns
— is the one that has already bitten this repo. bd 0.49.1's create is not
atomic (ranger-base-muoo): against a parent whose dependency closure is
tangled the daemon commits the issue and then outruns the client's socket
timeout, so bd exits 1, prints no id, and any dedupe keyed on the id it did
not return files again every pass. That cost 33 duplicate P1 beads in one
afternoon.

So the dedupe is the escalation's own **title**, written by bd in the same
breath as the issue: at most one OPEN `-l question` bead whose title opens
`settled open twice: <id>`. An orphaned create still dedupes. It is
verifyafter.go's marker discipline, one field over.

Two consequences fall out and both are wanted. An escalation the operator
answers and **closes** re-arms the rung — a bead that sticks again after a
human looked at it is news again. And an escalation whose *blocking edge*
never landed (the other half bd can lose on its own) is retried rather than
re-filed, because that edge is the only thing actually stopping the loop.

### The stop is read back, never assumed

`bd dep add` exiting 0 is not the edge being there — `Bd.Claim` learned that
lesson first (bd refuses a claim on stdout-empty exit 0). So the block is
verified by reading `dep list` back, and a pass that could not stop the loop
says so on stderr instead of printing a stop it did not make. The fake bd
grew a `fake-dep-add-fail` marker for the control arm: exit 0, nothing wrong
on the wire, no edge in the graph.

### Measured while building this

`newTestBackend` does not give a test a temp `$HOME`, and
`DefaultWorktreeRoot()` reads `$HOME` — so a test that cuts a session tree
cuts it under the **operator's live `~/.posse/worktrees`**. Found by doing
it. Any test in `internal/rhq` that touches a worktree needs
`t.Setenv("HOME", t.TempDir())` of its own; the same class as the live-ledger
and live-socket reads (ranger-base-rp2y, ranger-base-ouf9).
