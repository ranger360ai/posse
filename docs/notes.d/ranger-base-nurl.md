## "Closed means it is on main" now has a sweep behind it (ranger-base-nurl)

`mergeBack` runs where a pass JUDGES a close, and that is not every close. A
bead whose wait ran out keeps its claim and is not judged that pass; a
persona that closes it afterwards closes it in front of nobody, and its
branch is unlanded with nothing left watching. Measured 2026-08-27: four
closed beads at once whose commits were on their session branch and not on
main — one of them a 2172-line governance surface a later bead then built on
and could not find. Nothing was lost, but the invariant the ADR 0006 §3
verify pass reads was false four times over and the only way to see that was
a `git rev-list --count main..<branch>` census by hand.

**The record goes on the BRANCH, not only in the meta.** ADR 0011 §3 asks
for a run record naming the bead a tree belongs to, and `HerdrMeta.Bead`
already is one — but it is the wrong record for this question: a kill and a
`clearDeadMeta` both remove the meta and leave the tree standing
(rangerhq-09o2), so the pointer disappears exactly when the work strands.
`branch.<b>.posseBead` (worktree.go `beadKey`) is kept where
`branch.<b>.posseBase` already lives, for the reason that key's own comment
gives: the one record that survives every path is the one git keeps, and
`git branch -d` takes it with the branch. It is written at every launch into
the tree and moved by `NoteBead`, because the pre-Dial-F slot session is
reused across beads and a stale pointer would have the sweep asking about
the wrong bead.

**The sweep** (`landsweep.go`, `landClosedTrees`) runs at pass start, right
after `autoReapPass` and before routing — a pass that dies in gather (every
`--watch` instance on record has died somewhere in that window,
ranger-base-v674) still lands what the pass before it left. It reads git,
not the session list, which is the whole reason it exists next to the reap:
the reap walks live herdr sessions and cannot see a tree whose session was
killed or whose herdr restarted. Per tree: skip if nothing is unlanded (one
`git rev-list --count`, no bd call); ask the store of record whether the
recorded bead is closed; land it under the launcher lock (taken lazily, on
the first tree that needs it, so an ordinary pass never waits on another
launcher).

**Three things it deliberately does not do.** It never guesses a bead from a
branch name — persona names and repo basenames both contain `-`, so an
unrecorded tree is reported and not landed. It never touches a tree whose
bead is open: that is a persona at work, and landing there is what
`posse worktrees --land` would do and this must not. And it does not FILE a
bead on a blocked merge the way `mergeBack` does: a judged close happens
once, this runs every pass, and a bead per pass over a permanently
conflicted branch is spam. The ⚠ line repeats instead — every pass, which is
what "observable without a human running the census" means here.

**Pins** (`landsweep_test.go`), all six mutation-checked: unwiring the sweep
from `Run` fails two; dropping the closed check fails the open-bead arm;
dropping the "no record" report fails the unrecorded arm; stamping `""` at
launch fails the survives-the-meta arm; deleting the `--dry-run` branch
fails the dry-run arm; dropping `NoteBead`'s mirror fails the slot arm.
