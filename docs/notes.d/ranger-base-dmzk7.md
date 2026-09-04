# ranger-base-dmzk7 — the ≡ line asked its evidence

`MergeOutcome.EquivalentNote` is the one merge-back surface that reaches an
operator over an equivalence: `Blocked()` is false on every pairing, no
`merge-back-blocked` bead is filed, and the `≡` line the sweep prints
(`landsweep.go`, `dispatch.go` `mergeBack`, `LandSessionTrees`) is the whole
record. It said **"nothing here is unlanded"** for all three kinds of
evidence undifferentiated, which is a measurement claim made from evidence
that is a measurement in exactly one of the three (`equiv.byPatch`).

Found by ranger-base-9yje2 verifying ranger-base-emgdb (67effd0). That close
revisited the other two sentences for this exact overstatement —
`unmeasuredEvidence` and `unmeasuredClause`, and says so in its own doc
comment — and did not revisit this one. It is the sentence beside the change,
not the change: the pairing itself is correct.

## The shape that made it cost something

The launcher lands v1 of a session's commit, then the session **amends** its
commit. `--amend` keeps the author, the AUTHOR date and the subject, so the
identity pairing still matches while the content no longer does, and the
amended bytes are on no ref but that branch. ranger-base-9yje2 measured it
in-package at main `9920e75` — pairing `replayed:true`, `Merged=true`,
`Blocked()=false`, `contentNotOnBase(repo, "main", branch) -> ["notes.txt"]`
— and that repro is now the fixture of the pin below. The same tree in
the same pass got the honest sentence from `unaccountedFor` — *"an identity
match and not a measurement of what the replay kept; compare before retiring
the tree"*. Two confidences, one piece of evidence, and the confident one won
where it cost the most: a silently dropped strand report.

## What changed

`MergeOutcome` carries `Unmeasured` — `unmeasuredClause`'s words for the
commits no measurement of content accounts for, `""` when every one is a
patch-id twin — and `EquivalentNote` prints the confident sentence only on
`""`. Otherwise it prints what the other two surfaces (`unaccountedFor`,
`treeState`) already print, verbatim, so a human reads one answer whichever
surface reached them.

## What deliberately did NOT change

The bead offered a second shape: ask `contentNotOnBase` before setting
`Merged`, and report a strand when the base does not hold the branch's bytes.
It is not available. `contentNotOnBase` answers the DELETE question — *is
this branch the last copy of these bytes* — and cannot tell the amend above
from the case this whole arm exists for: ranger-base-nw9zg, a rebase whose
conflict a human resolved by keeping both sides, where the branch's bytes are
on main nowhere and the work is nevertheless entirely landed. Gating `Merged`
on it puts back the every-pass P1 over landed work that ranger-base-emgdb
removed, on 36% of landings (ADR 0051). Measured, not supposed: over that
fixture `contentNotOnBase(repo, "main", branch)` returns `["shared.mk"]`
while `main:shared.mk` carries the session's own target. So the sentence
sends a person to read the two commits; it does not guess for them in either
direction.

`Merged`, `Blocked()` and the caller switches are untouched for the same
reason, and so is `measuredOnBase` — autoreap and both `RemoveSessionTree`
arms still require every pairing to be `byPatch`, and nothing here widens
what may be deleted.

## Superseded

`docs/notes.d/ranger-base-emgdb.md` records the live-branch probe printing
`≡ 5 commit(s) … — nothing here is unlanded` for
`posse/gilfoyle-posse-ranger-base-nw9zg`. That pairing is three replays and
two patch-id twins, so from this change on the same branch prints the
`accounted for … compare before retiring the tree` sentence instead. The
measurement in that note stands; the sentence it quotes does not.

## Pins

- `internal/posse/mergebackequivnote_test.go` — the amend fixture (the bead's
  repro), both surfaces asserted to agree, the measured control that keeps
  `"nothing here is unlanded"` reachable, and
  `TestContentNotOnBaseCannotDecideTheReportingHalf`, which pins why the
  other shape is unavailable.
- `internal/posse/mergebackreplay_test.go` — the replay pin's sentence
  assertion, which encoded the defect.
- `internal/posse/worktree_test.go` — the three equivalent arms now expect
  the sentence their own evidence earns.

Mutation-checked both directions with `go test -overlay`: forcing the
confident arm reds all three pins, forcing the unmeasured arm reds the
measured control and two arms of the table.
