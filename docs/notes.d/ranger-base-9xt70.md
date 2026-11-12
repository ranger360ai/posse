# Four arms for distinctions four closes introduced and nobody asked about (ranger-base-9xt70)

*2026-11-12 · laurie · verify bead over the closes of ranger-base-ajkqx,
ranger-base-r2uxm, ranger-base-fswhs and ranger-base-w9w37; adds two test
files and changes no production code*

## What this is

The four closes verify. Their own mutation lists reproduce: 13 of 14 mutants
I aimed at the three wall beads died, and all five of the call sites the
landed-check fix touched died at their own site and nowhere else. This
fragment is the part that was NOT already covered — three conditions the
fixes introduce, each of which survived a mutant, and the hostile shapes the
live wall pin does not try.

The class is the one ORDERS.md names: **a condition or distinction a fix
introduces is itself an arm, and usually the unpinned one.** All three
survivors below are the OVER-grant or NOT-gated half of a rule whose
UNDER-grant half is pinned hard.

## The three mutants that survived, and what each one would cost

**1. `claudeOwnSessionTree` without its linked-worktree test**
(internal/posse/wallclaudeprojects.go). Dropping `LinkedGitDirs(real) == nil`
left every arm-1 pin in the `Wall|Seatbelt|Bwrap|Gates|Claude|Private`
filter green — 78s of tests, MEASURED 2026-11-12. The three refusals the
close pins are a main checkout, a plain dir in `$HOME` and a subdir of the
tree, and ALL THREE are refused by the path-shape test (two components under
the worktree root) before the worktree test is reached. So the guard that
says "and it must actually be a linked worktree" had no fixture of its own,
although it is the one deciding whether a directory gets `projects/<key>/`
reopened read-write.

`TestQAWallClaudeProjectsOwnTreeNeedsALinkedWorktreeAndNotJustThePathShape`
(arm 1) is that fixture, three ways: a plain dir at the accepted path shape,
a STANDALONE repository there (git answers `--git-dir`, so only the
git-dir/common-dir difference tells it from a worktree), and a tree whose
`.git` file names a worktree registration that is gone — the shape a
removed-but-not-cleaned session leaves. It asserts the refusal AND its
consequence at both renders: one read-only bind and no reopen, no `allow`
row in the SBPL narrow list.

**2. `posse gates` printing the narrow rules.** The ranger-base-r2uxm close
says it does. Emptying the loop in `SeatbeltReport` left the same 78s filter
green. `TestQAWallClaudeProjectsSeatbeltReportPrintsTheNarrowRules` asks the
report for the three rows, in order, with the reopen marked `w` and the
denies `x` — and asks a non-own cwd for the absence of the `w` row, so the
arm cannot pass over a report that prints nothing.

**3. The landed-check COMMENT rows gated on the record.** The
ranger-base-w9w37 fix gates the green→red FILING on whether
`recordLandedCheck` wrote, and deliberately leaves the three comment rows
ungated; the close states the trade and the ranger-base-w9w37 fragment
carries it as a residual. Applying the same gate to the red→red arm, and
separately to the red→green arm, left all 217 tests of the arm-2
`Landed|landed|Stretch|Red|Green|Verdict|Judge|File|Twin|Record` filter
green.

Gating either one costs something D5 owes a red stretch, and only while a
`git config` write cannot take `.git/config.lock`, which is when nobody is
reading carefully:

- the red→red comment naming the newest red tree, which is where the verdict
  is written once the warn has scrolled;
- the DEGENERATE filing on that same row — a stretch whose bead a fixer
  closed mid-red has no bead, and red→red is what puts one back. Gated, a red
  main with a closed bead and a held lock would have no open attributed bead
  and nothing saying so: D5's last sentence failing in the other direction
  from the one ranger-base-w9w37 fixed;
- the red→green `green at <tree>` comment, which is the line the fixer is
  waiting for.

`TestTheCommentRowsAreNotGatedOnTheRecordTheFilingRowIs` (arm 2) is those
three rows with the lock held from the first red onward, each re-checking
that the record really did stay stale, so none of it is read over a write
that succeeded. The gate the fix DID add is not in this arm's path: dropping
it leaves this test green and reds
`TestAVerdictThatCouldNotBeRecordedFilesNothingUntilTheRecordLands`, which
is where it belongs.

## The write verbs the live wall pin does not try

The ranger-base-ajkqx live pin tries twelve plant shapes: append,
replace-by-rename, directory move, a new file, and a symlink in the TREE.
`TestLiveWallClaudeProjectsRefusesTheWriteVerbsThePinDoesNotTry` is the rest
of what a shell has over a file it can see, under the same rendered wall and
the real bwrap. MEASURED 2026-11-12, bwrap 0.13.1, all thirteen refused with
the peer's bytes AND mtime unchanged:

| shape | answer |
|---|---|
| `rm` a peer transcript / memory file | EROFS |
| in-place truncate (`: >`) of either | EROFS |
| `chmod` the peer transcript, then write | EROFS at the chmod |
| `touch` the peer transcript | EROFS |
| `mkdir` a `memory/` in a peer key that has none | EROFS |
| **hardlink a peer transcript into the own key dir, then write it** | **EXDEV** |
| symlink from the own key dir to a peer transcript, then write it | EROFS |
| reopen a read descriptor via `/proc/self/fd/N` for write | EROFS |
| rename the own key dir over a peer's | EROFS |
| write at the root of `projects/` | EROFS |
| write inside the own key's `memory/` under a new name | EROFS |

The hardlink is the one worth keeping. A read-only bind mount is a property
of the MOUNT and not of the inode, so a link made from the read-only
`projects/` into the read-write own-key dir would leave the peer's inode
writable under a name the wall allows — and `/proc/sys/fs/protected_hardlinks`
does not help, because every seat is one uid and owns the file. What refuses
it is that the two paths are different MOUNTS of one filesystem, which
`do_linkat` answers `EXDEV` before any permission question is asked. That is
a property of the in-place-bind shape ranger-base-ajkqx chose, and it is why
the shape is safe rather than merely untried: a single `--ro-bind` with a
`chmod`-style narrowing inside it would not have this.

The arm's first act is a write to the seat's OWN key dir, fatal if refused,
so the thirteen refusals cannot be the refusals of a wall that reopened
nothing.

## Recorded, not filed

Two rendered sentences that are now wrong about half their subjects, both
from ranger-base-fswhs widening `claudeSessionStateDirNames` from two
members to four. Neither changes what any process does and no pin reads
either, so both are recorded on ranger-base-9xt70's close comment rather
than filed (ADR 0006 §6).

- `internal/posse/bwrap.go:519` — the prep step's reason reads "a member
  every claude seat shares, which claude runs ahead of each Bash command".
  True of `shell-snapshots/` and `session-env/`, false of `backups/` and
  `file-history/`, which claude runs at no point. It is printed twice per
  member in operator-facing output: `posse gates` (`bwrap.go:1975`) and the
  rendered launch line's comment block (`bwrap.go:1876`).
- `internal/posse/wallclaudesessionstate.go:5` — "Two members of claude's
  config directory are executed by claude ITSELF" now heads a list of four.
  The paragraph the close added says plainly that nothing runs the new two,
  so the file is internally honest; the opening sentence is what over-claims.

## Also read, and not findings

- **`SeatbeltCarveOut.Empty()` has no production caller.** Dropping
  ranger-base-r2uxm's `len(c.Narrow) == 0` term from it survives, and
  correctly: `SeatbeltProfile` gates each block on its own `len(...)` now, so
  the predicate the sibling pin
  (`TestEmptyCarveOutIsNotEmptyWithOnlyDenyRead`) was written for is read by
  nothing. The widening is consistency, not a gate, and the missing sibling
  pin would pin a dead function.
- **`carve.Keep` renders AFTER the narrow rules**, inside the carve-out
  block, as an `allow`. So the narrow block's comment — "the carve-out below
  still beats all" — holds for the carve-out's `Deny` and not for its
  `Keep`: a `Keep` literal under `projects/` would outvote the deny. Its one
  writer is `filepath.Join(gatesDir, f)` and the gates dir is never under
  the claude config dir, so nothing reaches it today.
- **`claudeProjectDir` collides by construction**, because it maps every
  non-alphanumeric byte to `-`: `<root>/a-b/c` and `<root>/a/b-c` are one
  key. Both are legal session-worktree paths, so two seats could share a
  reopened key dir. It is not reachable past the operator's keys — every
  reopen is under `<WorktreeRoot>/`, and `-home-posse` and
  `-home-posse-src-posse` are not — and the key scheme is claude's own, so
  posse cannot pick a different one and still find the dir claude writes.
  What the collision would buy is a sibling seat's transcript, which the
  design already treats as the seat's own untrusted output.
- **Claude 2.1.322's own protected list for that dir is now covered whole.**
  The design records it as `["shell-snapshots","session-env","projects","file-history","backups"]`:
  ranger-base-5632o took the first two, ranger-base-ajkqx the third, and
  ranger-base-fswhs the last two.

## Not closed here

- ranger-base-r2uxm's live darwin half. The render pin shows the rule's ORDER
  and the mutants show it is load-bearing; whether the kernel honours an
  `allow` nested inside a `deny` of a parent subpath is `ASSUMED` on the bead
  and this box is Linux. The verify lane has no darwin host either, so it is
  stated and not claimed — the same way ranger-base-3clx6 left
  ranger-base-ntnst's. The equivalent rule at the mechanism this host HAS is
  measured above, under the real bwrap.
