# git will not pack a ref it cannot resolve, and a seat's commit is in the seat's store (ranger-base-blkse)

2026-11-21, Linux 7.2.15 x86_64, go1.27.3, 16 cores, git 2.56.0, from this
bead's worktree. From ranger-base-zcj0z, which made the refusal print the
directory's contents so that a macos CI leg could answer a question no linux
seat can reach.

`TestQAWorktreeCommitLeavesNoStrayPackedRefsLock`
(`internal/posse/seatbeltworktreegit_qa_test.go`) was red on macos arms 2 and
3 from 9193d3b1 (ranger-base-5blmt) — arm 1 dies in `tree-check` on that
runner and never reaches this file at all. The red was `wgPack`'s vacuity
guard:
after `git pack-refs --all` the directory `<common>/refs/heads/posse` was
still there, and because it is a `t.Fatalf` the whole ADR 0059 D2 battery
under it — D2.1 `wgSeqArms`, D2.3 `wgRunPackedRefRewrite`, D2.4
`wgAliasArms` — never ran on darwin either.

## the listing, and the third answer

zcj0z's refusal offered two readings. The macos legs printed neither:

```
seatbeltworktreegit_qa_test.go:737: pack-refs left …/repo/.git/refs/heads/posse in place …
    IN THAT DIRECTORY: developer-2-probe
```

MEASURED on runs 44914630327 and 44901378525 (2026-11-21), macos-latest arms
2 and 3, identical in both. `developer-2-probe` with no `.lock` suffix is the
session's own LOOSE REF — not a lock a sandboxed git created and could not
unlink (which would have been a defect in the wall this file pins), and not
an empty directory (which would have been git's pack-refs declining to prune
one).

**git does not pack a ref that does not resolve to an object**, and since
ranger-base-5blmt a darwin seat's commit object lands in the seat's OWN
store, behind the new deny on `<common>/objects`. `wgPack` then ran
`pack-refs` in the repo, uncaged, where that object does not exist yet: the
ref was broken, the loose file stayed, and the directory stayed with it.

MEASURED here, git 2.56.0, linux, with no sandbox at all — a seat commit made
with `GIT_OBJECT_DIRECTORY` pointed at a store of its own, then `pack-refs
--all` in the repo:

```
error: refs/heads/posse/developer-2-probe does not point to a valid object!
pack-refs exit=0
listing: developer-2-probe
```

## the confound the macos log excluded by itself

The bead stated one, honestly: the runner image moved 20261102.0473 to
20261117.0516 and git 2.56.0 to 2.56.1 in the same window. The listing closes
it with no second run. The SIBLING session's branch is `posse/other-probe` —
the same directory, two files, one `pack-refs` — and the sibling's loose ref
is GONE from that listing. git 2.56.1 prunes. What it will not do is pack a
ref it cannot resolve.

## the fix, and the venue it was missing

`wgPack` imports the seat's own store before it packs, which is what every
fleet caller that reads a session tip already does (ranger-base-xcucy:
`spliceDetachedWork`, the landing sweep, `RemoveSessionTree`, each with THE
IMPORT FIRST in its own comment). So the rig now constructs the state the
fleet actually reaches rather than one it never has. The refusal names three
readings instead of two, the third being "the import above did not drain the
store".

AND THE GUARD HAS A READER ON EVERY BOX NOW, which is the half that matters
more than the line that fixed it. The state that red has nothing to do with
`sandbox-exec`: it is the seat's object store, which this package builds on
any platform (`siSeatCommit`, ranger-base-xcucy's fixtures). So
`TestQAWgPackPrunesTheRefDirectoryAfterASeatStoreCommit` measures the guard
here, with the un-imported state as its control — the control's own assertion
is that the listing holds the session's ref and ONLY it, which is the
sibling-pruned discriminator above, pinned. Mutated by removing the import:
the arm reds on linux with `IN THAT DIRECTORY: developer-2-probe`, the macos
line reproduced.

`wgFixtureAnyBox` is the fixture that makes that possible, and it is
`wgDenyFixture`'s body given its own name: `wgNewFixture` takes `sbRoot`'s
root, which SKIPS wherever TMPDIR resolves under /tmp — every Linux box. An
arm that runs no command under a wall does not need that root.

## not closed here

- ADR 0059 D2's three arms, in the spelling they have today. They were green
  on darwin at edfe7a79 (2026-11-15) and the battery dates to a6a4c8b6
  (2026-09-11), so "never measured there" would be wrong — but 9193d3b1
  rewrote every probe in all three to carry the seat-store env prefix and
  red the guard above them in the same commit, so the current spelling has
  had no darwin reading at all. This landing unblocks them; the next macos
  leg is the first. Filed: ranger-base-m8msi.
