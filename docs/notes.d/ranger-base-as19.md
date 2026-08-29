## Retiring a tree needs a measurement, not a decision (ranger-base-as19)

`ranger-base-g2xf` taught `MergeSessionWork` that ahead-by-sha is not
ahead-by-work, so a cherry-picked branch reports `Merged` with the pairing
named instead of a strand. It deliberately left the destructive half alone:
`RemoveSessionTree` went on asking its own question with
`git rev-list --count <base>..<branch>`, which for exactly that branch is `1`
forever. `posse kill` therefore reported the work landed and then kept the
tree, the landing sweep re-printed the ≡ line every pass because nothing ever
cleared it, and the only escape was the same override that stands down a real
strand's refusal — the two indistinguishable again, one layer down.

The fix is not "trust the equivalence". It is that `equivalentOnBase` answers
with **two kinds of evidence**, and only one of them is about content:

- **patch-id** (`git cherry` says `-`): the base holds a patch with this
  commit's patch-id. A measurement. The branch is the last copy of nothing,
  so deleting it can lose nothing.
- **git's `-x` trailer** (`(cherry picked from commit <sha>)`): a record that
  a human decided this landed as that. It is the only evidence left after a
  hand resolution — and it cannot say whether the resolution kept every hunk.
  A decision, not a measurement.

So each pairing now carries how it was answered (`equiv.byPatch`), the
reporting half prints both (`equivNotes`), and `measuredOnBase` is the
stricter question the destructive half asks: is **every** commit's account a
measurement? Empty is false — nothing accounted for is not proof.

`RemoveSessionTree` asks it before refusing. Measured: the tree and the branch
retire without `--force`, and the branch goes with `-D`, because
`git branch -d` asks *reachability* — the same by-sha question — and would
otherwise just move the refusal one line down. Trailer-only: still kept, but
the refusal now names the pairing and prescribes the exact two commands,
instead of counting shas at an operator who has just been told the work is
already on main.

That split is the honest reading of the risk the bead itself raised. A hand
resolution that drops a hunk leaves the branch holding the only copy of it,
and `landed()` exists precisely to stop a retire on evidence that thin. The
noise is real, but the answer to it is a sentence a human can act on, not a
deletion on somebody's say-so.

Pins (`internal/rhq/worktree_test.go`):
`TestRemoveSessionTreeRetiresOnlyWhatIsMeasuredOnTheBase` — four arms: a clean
`-x` pick, a pick with no trailer (patch-id alone), a hand-resolved pick
(kept, naming the trailer), and real unlanded work (kept, in the unchanged
words). Every arm asserts the fixture is ahead by sha first, or the retiring
arms could pass over a guard that was never reached.
`TestKillRetiresACherryPickedTree` — the same through the kill's own path and
`KillLanding.Line()`, which is where the operator met the bug.

Mutation-checked four ways: the new arm deleted reds both retiring arms;
collapsing the split so the trailer licenses deletion too reds the
hand-resolved arm alone; dropping `-D` reds both retiring arms (`branch -d`
refuses); and making `measuredOnBase` true for an empty account reds the
control.
