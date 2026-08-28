## The merge-back handoff reads the graph back (ranger-base-eul1)

`fileMergeBlocked` (dispatch.go) is the third of the harness's own
`bd create` calls, and it was the last one still trusting the exit code.

The hazard is the one `verifyMarkerPrefix` measured: against a parent whose
dependency closure can reach a symmetric `relates-to` pair, bd 0.49.1
COMMITS the issue and then fails on the `--deps` edge (ranger-base-muoo,
ranger-base-pkqn). A caller that reads `err != nil` as "nothing happened"
then says the handoff was not filed while a P1 assigned to the persona is
sitting in the queue — edgeless, unprovenanced, and with nothing that will
ever retry it. The operator goes looking for a bead that exists.

Three things follow, and they are the shape every harness-side create
wants:

**The dedupe key may not live in the edge.** `discovered-from` is the one
field of a create bd can commit the issue without, so nothing that must be
found again may be stored only there. verify-after keeps it in the
description (`verifyMarkerPrefix`); settle-open keeps it in the title
(`settleStuckTitle`). This rung keeps it in both: `mergeBlockedTitle` is
branch+base, which names the merge exactly because a branch is cut per bead
(`SessionForBead`), and the description carries a `discovered-from: <id>`
line for a human. The bead's close also gets a comment naming the filed id
— a pointer that survives an edge that never landed.

**The graph decides what the pass reports, not the exit code.** On a create
error the rung asks the labeled listing whether the bead is there:
present → "filed <id> … WITHOUT its discovered-from:<parent> edge"; absent →
the old "could not file". Both are honest and they are different facts. The
same principle as `blockOnEscalation` (settleopen.go) and `Bd.Claim`.

**The read is the dedupe, never the handoff.** A listing that will not
answer is reported and the create runs anyway: a duplicate handoff is
visible, a missing one is not.

The pins are in `worktree_qa_test.go`, and the negative arm is the point —
`fake-create-hard-fail` (herdr_test.go) is bd failing having committed
NOTHING, which a read-back that reported "filed" off any error would get
wrong. It carries a positive witness that the create was attempted and the
store really holds no bead, because an assertion of pure absence is
otherwise satisfied by a pass that never got that far.
