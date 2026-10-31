# ranger-base-2a7qa — a merge-back replay whose conflict was real code

Landed 2026-10-31, git 2.50.1 / macOS 25.6.0, go 1.26.10.

`ranger-base-0ckgm` closed with one commit on
`posse/jian-yang-posse-ranger-base-0ckgm` (`0fef3d46`) and `main` moved on
underneath it. The launcher's replay conflicted, aborted, and filed this
bead. `docs/notes.d/ranger-base-xea2y.md` is the procedure and it held
without amendment; what is worth keeping here is that this conflict was NOT
the generated-file hunk xea2y's was, so the step xea2y spends one paragraph
on — "resolve whatever came out carrying conflict markers" — was the whole of
the work.

## The conflict

`git merge-tree --write-tree --merge-base=<base> main <work>` named exactly
one file:

```
Auto-merging internal/treepins/bddaemonleak_qa_test.go
CONFLICT (content): Merge conflict in internal/treepins/bddaemonleak_qa_test.go
```

`docs/notes.d/README.md` — the hunk xea2y says will recur on every merge-back
that carries a fragment, and which is most of them — auto-merged clean this
time, because `main` had landed no fragment since the base.

Both sides rewrote the same scan, for unrelated reasons:

- `main` (`3b2a49e2`, ranger-base-q82lc) made the CLEARING half parse: the
  two excuse tokens are read out of each carrying file's string literals
  rather than its bytes. `files` became `map[string][]string`, the walk
  collects literals beside call sites and skips a file with no call, and a
  fourth control arm plants each token as prose and asserts the file is still
  flagged.
- the work commit (ranger-base-0ckgm) made the scan's ROOT derivable:
  `scanRealBdCallSites()` takes no root and spells `"."` at the walk, the
  fixture arms call a new `scanBdCallSitesUnder(root)`, and both share one
  `bdScanVisitor`.

So the resolution is neither side's text: `bdScanVisitor` carries q82lc's
literal collection and its skip, both scans carry the new map type, and
q82lc's new arm calls `scanBdCallSitesUnder` — the fixture-rooted scan the
work commit split out — because calling `scanRealBdCallSites` with a root is
the spelling 0ckgm exists to remove.

## What made it checkable

The reading xea2y prescribes for a file both sides edited, run in both
directions over the three blobs `merge-tree` prints for the conflicted path
(stage 1/2/3 = base/main/work):

```
diff <(git cat-file blob <stage-3>) <merged>   # must be exactly main's change
diff <(git cat-file blob <stage-2>) <merged>   # must be exactly the work's
```

Both came out clean to the line, with the only additions beyond either side's
own diff being the three the merge itself forces: the new arm's call site, the
`bdScanVisitor` doc sentence about literals, and the arm COUNT. That count is
prose in three places and all three were four short of the truth — this
file's head comment, `treewidedoor_qa_test.go`'s door row, and
`docs/notes.d/ranger-base-0ckgm.md` all said "three control arms" because
q82lc's arm did not exist when they were written. Nothing pins it, so a
merge-back that adds an arm is exactly the shape that leaves it stale.

## The amend trap, in your own worktree

AGENTS.md forbids `--amend` in the SHARED checkout and this is a session
worktree, so an amend to correct a mangled message is allowed here — but the
PID deny is not scoped to the shared checkout. A bare `git commit --amend -F
<msg>` is refused by the PATH shim like any other unqualified commit, and the
refusal prints the deny guidance rather than a git error, so the commit it
would have fixed is still there with its old message. The form that runs is
the same path-limited one every other commit here takes:

```
git commit --amend -F <msg> --date=<the author date> -- <the same paths>
```

`--amend` with a pathspec rebuilds `HEAD^`'s tree with those paths taken from
the working tree, so naming every path the commit touched leaves the tree
byte-identical — checked, `git rev-parse HEAD^{tree}` before and after.
`--amend` ignores `GIT_AUTHOR_DATE`, which is why the author date is passed as
`--date`.

## Verified

`make fmt-check`, `scripts/notes-index.py --check`, `make tree-check`, the
treepins pins and arms this touched under one `-run` filter
(`QABdDaemonGuard|QANoTestShellsRealBd|QANoTreeWalkingHelperIsHandedATreePath|QAEveryTreeWidePinHasADoor|QATheTreeWideDoorsReportRealDrift|QAMakeTestOpensTheTreeWideDoors|QAEveryTreepinsDoorFilterNamesItsPins|QATheTreepinsEnumerationRuleMatchesWhatItClaims|QATheHeadCommentsPinAndDoorCountsAreTheMakefiles|NotesFragmentIndexIsCurrent|QAOneRepoRootHelperInTheTestPackage`,
ok 26.9s, the live census reading 9 real-bd call sites in 7 files), and
`make test` whole.
