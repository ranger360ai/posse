## A path-limited commit never reads the index, so the `git add` in front of it is not the suspect (ranger-base-nor)

The bead reported three occurrences (monica, 2026-08-22): `git add
.beads/issues.jsonl && git commit -m … -- .beads/issues.jsonl` printing `no
changes added to commit` with the file still `MM` in status, and an identical
retry seconds later committing 1–2 lines. Its hypothesis was that the beads
daemon rewrote the JSONL between the add and the commit, so the pathspec
commit compared against content the add had not seen.

**The hypothesis cannot be right, and one measurement settles it** (git
2.39.3, macOS 25.4, 2026-08-29). A path-limited commit builds its tree from
HEAD plus the **working tree** versions of the paths it names. What is staged
for those paths is not consulted at all:

```
stage v2, then write v3 to the worktree, then
$ git commit -m x -- .beads/issues.jsonl
$ git show HEAD:.beads/issues.jsonl        ->  v3-worktree
```

So the add contributes nothing to the commit, and no rewrite "between the add
and the commit" can empty the commit by invalidating the add. `no changes
added to commit` means exactly one thing: **at commit time the working tree
already matched HEAD for that path.**

**The state that produces the whole report, reproduced verbatim.** Index
holding a blob that is not HEAD's, working tree matching HEAD, other work in
flight:

```
$ git status --porcelain
MM .beads/issues.jsonl
 M NOTES.md
$ git add .beads/issues.jsonl && git commit -m 'bd: sync' -- .beads/issues.jsonl
no changes added to commit (use "git add" and/or "git commit -a")     exit 1
```

That state is not exotic — it is what bd's own `pre-commit` hook leaves behind
in any repo where the blessed form runs, measured one bead earlier
(rangerhq-be7k): the hook's `git add` reaches the temporary index the commit
is built from and never the real one, so the previous path-limited commit
carried the flushed projection into HEAD and left the pre-flush blob staged.
The tree was already in git; the retry succeeded because 1–2 genuinely new
rows had landed by then. The daemon is in the story only as the thing that
eventually writes those rows, and it is behind, not ahead: measured on the
live store, `bd comments add` returns with the JSONL byte-identical, and
`bd sync --flush-only` is what changes it — the flush is synchronous.

**Where the defect actually was.** Not in the recipe, which ADR 0015 §4
retired: after the queue cutover the projection lives in the queue repo and
the launcher commits it (`CommitQueueJSONL`). The same shape had moved in
there, and it had teeth, because that function gated its commit on

```go
git status --porcelain -- <paths>     // "is there anything?"
git commit -m msg      -- <paths>     // "…is there anything HERE?"
```

Two different questions. `status` also reports an index that differs from
HEAD, which is precisely the `MM` state above — so in it the gate said
"dirty", the commit that followed exited 1 with nothing to commit, and a
close whose projection was already in git was reported to the operator as a
launcher failure. The fix is to ask the question the commit will ask,
`git diff HEAD -- <paths>`, which is empty exactly when the commit would have
nothing to do. The path-limited form is untouched; it was never the problem.
Pinned by `TestQueueCommitAsksTheQuestionTheCommitWillAsk`, whose fixture
asserts it built the `MM` state before it asserts the skip.

**The rule this leaves.** `git status` is not a preview of a path-limited
commit, and neither is `git diff`. Only `git diff HEAD -- <paths>` is —
AGENTS.md already says so for reading the state be7k leaves; this is the
same fact spelled as a precondition rather than as a diagnosis.
