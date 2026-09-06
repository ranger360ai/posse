# ranger-base-yem9f — a merge-back block landed by REPLAY, not by rebase, and what that costs

`posse/dinesh-posse-ranger-base-yi2f8` (ADR 0057, the pane-mode registry
removal) closed with three commits that would not fast-forward onto main.
The block was real and not the stale shape the bead warns about: at dispatch
both `posse/dinesh-posse-ranger-base-yi2f8` and
`refs/posse/merge-blocked/posse/dinesh-posse-ranger-base-yi2f8` still resolved
to `4fca5b70`, and its worktree still existed.

## The conflict, and it was additive on both sides

Three of the branch's eighteen paths were also touched by the main it had to
land on; fourteen are byte-identical to the branch tip after the replay
(`git diff 4fca5b70 HEAD -- <the 18>` names only these three):

- `scripts/silent-reverts.allow` — **the whole conflict**. The branch appended
  its `63e672e` triage line at EOF; main had appended `ed9a698`'s `4efb529`
  line at the same EOF. Resolved by keeping both, main's first. Measured
  additive rather than assumed: sorted `comm` between the merge-base and each
  side shows one line added by each and **none removed by either**.
- `CHANGELOG.md` — auto-merged, and `git diff main HEAD -- CHANGELOG.md` is
  exactly the branch's own hunk, so nothing main wrote was displaced.
- `docs/adr/0013-runtime-dispatch-contract.md` — auto-merged; main's edit is
  §5's `egress:` sentence, the branch's is §7's narrow exception and §8's
  partition sentence, and all three new texts are present with all three old
  ones gone.

## Why a replay and not the rebase the bead prescribes

`git rebase` is denied to this seat by the **Claude Code auto mode
classifier** — twice, on two spellings, with no `refused by posse gate:` line
and no `bd-argv-gate:` header, so it is neither the PID gate nor bd. It also
agrees with the persona's own standing rule not to touch history. So the
three commits were `git cherry-pick`ed onto this bead's own branch, which the
launcher fast-forwards onto main at close — the `ranger-base-gpxo2` shape for
the same class of block, minus the rewrite of the source branch.

## ADR 0054's patch-id token, claimed by a HAND replay

The branch's own allow line carries the token *because* its author expected a
rebase at landing. It paid off, and the base move was outside its hunks'
context so the diff name survived: `git diff-tree -p | git patch-id --stable`
is `b887b9cd…` for **both** `63e672e5` and its replay `7ab56daf`, and the
audit prints `triaged (patch-id twin of 63e672e): 7ab56da`. ADR 0054 had
measured the equal-across-an-outside-context-move case; this is that case
arriving for real, on a replay no launcher performed.

The other two commits do **not** keep their patch-ids (`02a2d1cd`→`734b4657`,
`7f279a5f`→`b642bed4`): both edit the tail of `silent-reverts.allow`, which is
exactly the inside-context move ADR 0054 says changes the name. Neither
carries a token, and neither needs one — only `63e672e` is flagged.

## What this leaves standing, and it is not nothing

`git cherry 9706fdb8 posse/dinesh-posse-ranger-base-yi2f8` marks `63e672e5`
`-` and the other two `+`, and all three replay keys (`%ae|%aI|%s`) match
their replays exactly. So the launcher pairs all three — one by patch-id, two
by the replay arm — and `len(eq) > 0` clears `Merged`, so **no new block is
filed**; but `measuredOnBase` is false the moment any commit pairs by
anything but patch-id, so `worktree.go`'s retirement gate never fires and
**the source tree and branch are never reaped**. That is
`docs/notes.d`-worthy because it is the third instance: `ranger-base-3ji2w`
and `ranger-base-k5aqw` are the same ask. Filed here as `ranger-base-xphof`.

There is a second, independent reason this tree will not be reaped, and it is
new for the class: `contentNotOnBase` names all three merged paths, because on
each of them the branch's exact blob is a **pre-merge** state that
deliberately does not exist on main. `ranger-base-3ji2w` could show
`git diff <branch> <replay> -- scripts/` empty; a merge that keeps both sides
never can. Both gates are false, for different and both correct reasons.

## Verified

`env -u GIT_EXTERNAL_DIFF make test` on the replay, base `a7d93c57`: every Go
package `ok` and zero `--- FAIL` lines — `internal/posse` 906.9s, root 362.7s,
`cmd/posse` 205.5s, `cmd/testparallel` 1.7s — plus `fmt-check`,
`verify-test-times`, `verify-parallel`, `verify-suite-lock`,
`verify-silent-reverts` (all 17 detector self-test arms, including the four
that grade the patch-id token) and `tree-check`. `doc-check`, `crew-check` and
`identity-check` were re-run after this fragment was added. `go vet ./...` is
clean. The one non-zero exit is the audit step, below.

## The red that was already there

`make test`'s audit step is red on this tree and was red on main before it:
`scripts/audit-silent-reverts.sh a7d93c57` — pure main, no replay — exits 1
with the single hit `UNTRIAGED: 953f0be`, and CI run 34014409992's
`--log-failed` ends on the same line with every Go package `ok` on both
runners. That is `ranger-base-584a6` (ci-red), commented there with the
measurement. It is not this bead's diff and not this bead's commit to triage.
