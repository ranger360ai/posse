# ADR 0041 — A close that leaves uncommitted paths in its session tree is written on the bead and routed back to the closer; an empty branch alone is not a finding

*Status: accepted 2026-09-01 · owner: architect · extends ADR 0006 §3
(closed means it is on main) and the settle-open shape (ranger-base-9hm)
· from ranger-base-k77sk, on ranger-base-fyzqf's measured table*

> ranger-base-yeg1 closed on 2026-08-30 naming four deliverables. Not one
> was committed: the branch's reflog is a single "Created from main". The
> launcher saw it — the pass printed `4 uncommitted path(s) left in …
> (NOTES.md bdpin_qa_test.go scripts/verify-bd-pin.sh
> internal/rhq/bdflushdiscipline_qa_test.go) — not merged`, then `closed
> with no commit … nothing to merge`, and the reap `KEPT` the tree for the
> same reason. Three lines, one log. A day later version-pin.toml carried
> three claims copied from the close comment, all false at HEAD.

## Context

Two stores hold the fact "this bead's work reached main": bd holds the
close, git holds the commits. ADR 0006 §3 asserts their agreement
("closed means it is on main") and the verify pass reads it. The launcher
already measures the disagreement at close time — `MergeSessionWork`
returns `Dirty` (worktree.go dirtyPaths) and `Commits` — but the reading
goes to the pass line and `dispatch-watch.log`, a retrospective
instrument, and to `posse worktrees`, a pull surface. Nothing writes it
where the false claim lives: on the bead, under the close comment that
the next reader copies from. yeg1 carried no `verify_labels:` label, so
no verify bead existed to carry the commit list either.

MEASURED 2026-09-01 over the 49 session branches in ~/src/posse: 12 of
the ~40 closed branches never carried a commit; **8 of those 12 have
clean trees** (design, question and verify closes legitimately produce
no commit) and **4 closed beads left uncommitted paths** — yeg1 (4
paths, an 814-deletion rewrite among them), plta (an 87-line test edit),
i312 (a staged new test file), ihd2 (a stray `calls.log`). So the class
is one close in ten, three of four are real work, and the signal that
separates them from the eight is the dirty set, not the commit count.

The bead asked two questions: should the settle contract require the
paths a close comment cites to exist at the branch tip before the close
is accepted, and is an empty branch at close itself a finding. Answers
below: no, and no — with the thing that does the job instead.

Concepts in play: **store of record** (bd owns the close; git owns the
commits; a fact readable in two stores disagrees, and the reading of
one is a snapshot about the other — single-writer-and-stores.md);
**cooperative vs realized** (ADR 0025 vocabulary for the pre-close
refusal).

## Decision

**1. The dirty set is written on the closed bead, once.** At every
launcher moment that reads a closed bead's tree — `mergeBack` (the judged
close), `landClosedTrees` (the sweep), and the kill/reap landing — when
`Dirty` is non-empty, posse comments on the bead:

    closed dirty [N path(s)]: <paths> in <tree>; <M> commit(s) on <branch> — nothing carries these; the tree is kept until they are committed or discarded (`posse worktrees`)

Actor `posse`, deduped by the `closed dirty [` prefix over the bead's
existing comments, exactly as settle-open dedupes its marker: three
sites and every later pass produce one comment. The count lives on the
bead because it is a fact about the bead (settleopen.go's argument; a
$StateDir ledger would be the fifth store ADR 0011 rejects). `Commits`
is context in the sentence, never a trigger: a clean tree with zero
commits gets no comment.

**2. The dirty close is routed back to the closer as a bead.** Same
shape as the merge-back-blocked handoff (dispatch.go fileMergeBlocked):
title `closed dirty: <id> — N uncommitted path(s) in <branch>`, label
`code` (MergeBlockedLabel), P1, assigned to the closer, `discovered-from:
<id>`, deduped by open title. The description names the tree, the paths,
and the two resolutions — commit them under the bead id in that tree, or
discard them — and says that `posse kill` retires the tree only after.
Filed from the judged close and from the sweep both; the sweep's
"a bead per pass is spam" objection is answered by the title dedupe the
merge-back handoff already has. A stray log file costs the closer one
`git clean`; that price is accepted, because the launcher cannot tell an
814-line rewrite from a scratch file and must not guess (§4 below).

**3. A cooperative pre-close refusal in the session.** The bd argv gate
(scripts/bd-argv-gate.py) refuses `bd close` when the call's cwd is
inside a posse session worktree (`git rev-parse --show-toplevel` under
the worktree root and the checked-out branch under `posse/`) whose
`git status --porcelain` is non-empty. The refusal lists the paths and
both resolutions. It never fires in the shared checkout — there the dirt
belongs to other writers (ADR 0022) — and it is **cooperative** class
(ADR 0025): `git checkout -- .` walks around it, and that is an explicit
act by the one actor who knows whether the paths are work. §1–2 are the
realized belt behind it, operator-side, under the launcher lock.

**4. Two things the launcher does not do.** It does not reopen the bead
(the close is the persona's write in the store of record; a harness
flipping status fights the closer, and a reopened bead re-enters `bd
ready` and dispatches into a *new*, clean tree). It does not commit the
paths (ADR 0022: a commit by someone other than the writer attributes
content nobody chose to ship; yeg1's closer had decided step 1 of that
diff was *not* to land).

**5. An empty branch is not a finding.** Eight of twelve commit-less
closes are correct. The finding is empty-and-dirty, and §1 keys on dirty
alone.

## Consequences

- The correction sits beside the claim: whoever copies a close comment
  into a pin file or a successor bead sees `closed dirty` under it.
- A dirty close costs the shop one P1 bead to the closer; the census
  rate is ~1 in 10 closes, of which ~1 in 4 is scratch.
- `posse worktrees` and the pass lines are unchanged; they gain nothing
  and lose nothing. The log stays retrospective, the bead becomes the
  record.
- Verify-after (ADR 0006 §3) needs no change: the verifier reads the
  bead's comments, and a verify bead whose close carries `closed dirty`
  has its answer before it starts.
- The four standing trees are handled outside this ADR: yeg1's paths are
  carried by ranger-base-dep6x and the standing pin-file decision; plta
  and i312 are laurie's tests and go back to her as a bead; ihd2's
  `calls.log` is a `git clean` for the operator.

## Alternatives rejected

- **Require the close comment's cited paths to exist at the branch tip
  before the close is accepted** (the bead's question). There is no
  accept moment: bd is the store of record for the close (ADR 0011, ADR
  0013 §1 record row), the persona writes it, and posse reads it
  afterwards. A pre-close check would have to extract paths from prose,
  which fails both ways — yeg1's own close comment names
  `etc/bd/version-pin.toml untouched`, and a check that reads that path
  refuses a true sentence. The exact signal already exists unparsed:
  the tree's dirty set. This was the clever one; it measures a
  restatement when the tree is the fact.
- **Treat an empty branch at close as a finding.** Measured false eight
  times in twelve.
- **Reopen the bead.** Single writer on status; and reopen re-dispatches
  into a fresh tree without the paths (§4).
- **Auto-commit the dirt under the bead id.** ADR 0022's incident with
  the launcher as the sweeping writer (§4).
- **A ledger of dirty closes in $StateDir.** settleopen.go's reasoning:
  a fact about the bead, counted anywhere but the bead, is one more store
  that can disagree with it.
- **Keep the pass line and the worktrees listing as the record** (status
  quo). MEASURED: all three lines printed at yeg1's close, and the finding
  still took a day and a false pin file to surface. A log nobody reads at
  the moment the claim is copied is not a record.
- **Make §3 realized** (a bd hook, or a launcher-side close interception).
  bd hooks are L1-denied crew-wide and would fire in the shared checkout
  on other writers' dirt; interception has no seam — posse observes the
  close, it does not mediate it. The realized layer is §1–2.

## Verification

1. Census, expected empty once the four standing trees are resolved:
   `for b in $(git for-each-ref --format='%(refname:short)' refs/heads/posse/); do wt=~/.posse/worktrees/posse/${b#posse/}; [ -d "$wt" ] && n=$(git -C "$wt" status --porcelain | wc -l) && [ "$n" -gt 0 ] && echo "$b $n"; done`,
   joined against `bd list --all --json --limit 3000` for status
   `closed`. MEASURED 2026-09-01: yeg1 4, ihd2 1, i312 1, plta 1.
2. §1 pin: a closed bead whose tree holds one dirty path, driven through
   `mergeBack` then `landClosedTrees`, carries exactly one `closed dirty
   [` comment. Mutation: remove the prefix dedupe → two comments, red.
3. §2 pin: the same close files one bead with the title above,
   `discovered-from`, assignee = closer; a second pass files none; a
   clean close files none.
4. §3 probe (probe the gate directly, as bdargvgate_qa_test.go does):
   `bd close x` with cwd a dirty session worktree → deny JSON naming the
   path; cwd the shared checkout, dirty → silence; cwd a clean session
   worktree → silence.
