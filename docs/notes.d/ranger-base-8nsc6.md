## A merge-back block filed by a pre-fix binary is answered by the binary, not by a rebase (ranger-base-8nsc6)

`merge-back blocked: posse/dinesh-posse-ranger-base-uzgkz does not land on
main` was filed on 2026-09-03 against a real rebase conflict. It named the
right conflict and the wrong conclusion: all three commits were already on
main, and the conflict was the replay of one of them meeting its own landed
copy. The fix (67effd0, ranger-base-emgdb, landed 2026-09-04 04:26) added
`equivalentOnBase`'s third arm; the bead predates it. Nothing here was code
work.

**The accounting, all three arms in one branch** — the reason this branch is
the one that needed the third:

| branch commit | on main as | arm |
|---|---|---|
| c9a4cdd | 1e9b2ba | patch-id twin (`git cherry` `-`) |
| 1ed7820 | b7da69a | patch-id twin |
| 34a27b4 | 6a230eb | author + AUTHOR date + subject (replay) |

The launcher's own rebase wrote no `-x` trailer, and the by-hand resolution
moved the patch, so for 34a27b4 the first two arms are both blind. That is
the 36%-of-landings case ADR 0051 measured, not a corner.

**Verified against the live branch with the in-force binary**
(`posse 0.4.0+9920e75`), rather than from the source alone:

```
posse worktrees
  …/posse/dinesh-posse-ranger-base-uzgkz → main  ·  3 commit(s) not on main
  by sha, replayed onto main as 34a27b4d85f3 as 6a230ebcf234 (same author,
  author date and subject), which is an identity match and not a measurement
  of what the replay kept — compare before retiring
```

An in-package probe calling `equivalentOnBase(repo, "main", branch)` returns
the three pairings above with `measuredOnBase = false`. `MergeSessionWork`
asks that same question before it rebases (worktree.go), so `Blocked()` is
now false for this branch and this bead cannot be re-filed by a pass; the
closed-bead dedupe (c3ab918, ranger-base-j8qmj) keeps it closed while the
branch does not move, and its tip has not moved since 2026-09-02.

**What "compare before retiring" is asking for, done.** The identity arm is
deliberately not a measurement, so the tool refuses to grade the resolution
and hands the comparison to a human. Here it is: `34a27b4` vs `6a230eb`
differ in eighteen diff lines — one blob index, and four fixture literals in
`internal/posse/memoryland_credshapes_qa_test.go` whose digit runs the
resolution re-spelled as repeated letters (main had scrubbed that shape on
top). `internal/posse/memoryland.go`, the whole of the shipped change, is
byte-identical across the two. So the replay kept every hunk that ships, and
nothing on this branch is the last copy of anything. **Verdict: DO-NOT-LAND.**
No delete licence follows — `measuredOnBase` is still false, the branch is
kept, and retiring it stays a human's act.

**The lesson for the next one of these.** A merge-back bead carries the
reason its filer computed, not a standing fact, and the filer may since have
been fixed. Order the reading: (1) `posse version` and whether the arm you
need is in that commit; (2) the per-commit accounting above, never
`main..branch` as a whole — main moves on, so a line-level `comm` of the
branch's added lines against main reports dozens of "branch-only" lines that
are simply later refactors (fifteen commits touched `gates.go` and
`visibility.go` after this branch landed); (3) only then a pick. Two of the
three arms of the accounting are one `git cherry` call.
