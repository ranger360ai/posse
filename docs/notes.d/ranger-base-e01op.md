## The launcher's own scaffolding, read as the seat's unlanded work (ranger-base-e01op)

Github issue #10, from the work box on 2026-10-08: a dispatched seat that had
committed everything it wrote still collected a P1 closed-dirty handoff. The
two paths named in it were both written by the LAUNCHER, before the persona
had typed anything — `.beads/redirect` and a `.bob` symlink — and neither was
excluded from git.

### The two holes, MEASURED

MEASURED 2026-10-09, darwin 25.4.0 / APFS, git 2.50.1 (Apple Git-155), over a
one-commit repo plus one linked worktree built by hand.

| fixture | main checkout | linked worktree |
|---|---|---|
| `.bob` a DIRECTORY, `.gitignore` says `.bob/` | clean | — |
| `.bob` a SYMLINK, `.gitignore` says `.bob/` | — | `?? .bob` |
| `.beads/` holding one `redirect`, no `.beads/.gitignore` | — | `?? .beads/redirect` |
| `/.bob` in `.git/worktrees/<name>/info/exclude` | — | `?? .bob` (unchanged) |
| `/.bob` and `/.beads/redirect` in `.git/info/exclude` (COMMON) | clean | clean |

Three findings in that table, and the design rests on all three:

1. **A trailing slash is the bug.** A gitignore pattern that ends in `/`
   matches a directory and nothing else. The main checkout has `.bob` as a
   directory, so the operator's `.bob/` covers it and reads as correct
   forever; the session tree has the same name as a SYMLINK, which that
   pattern does not match. The ignore was never wrong — it was written about
   the only tree the operator looks at.
2. **bd's own `.beads/.gitignore` is not in a session tree.** It covers
   `.beads/redirect` in the main checkout. The `.beads` a fresh worktree has
   is the one `seedBeadsRedirect` creates, and it holds exactly one file: the
   redirect.
3. **A linked worktree has no exclude file of its own.** git reads
   `$GIT_COMMON_DIR/info/exclude` and only that one — `git rev-parse
   --git-path info/exclude` from inside a worktree answers the common path,
   and a pattern written under the worktree's own git dir hid nothing (row 4).
   `excludeFromGit` already got this right for `.agents/skills`; the new
   caller reuses the same arithmetic rather than a second copy of it.

### Why it is fixed at the source and not in the check that reported it

ADR 0041's closed-dirty check is one of five readers of this tree's status:
`posse worktrees`, the reap guard (`reapguard.go`) and `RemoveSessionTree`'s
refusal all read `dirtyPaths`, and the persona reads `git status` with its own
eyes. A scaffolding set subtracted inside the closed-dirty check would still
be dirt in the other three, and the fifth reader is a human who would be told
the tree is clean by a tool while git says it is not. An exclude makes it
clean for all five — and the launcher's own scaffolding is the one thing here
posse is entitled to speak for.

The cost of leaving it was not the one spurious P1. It was that closed-dirty
is the signal that says a close did not land, the one signal that must never
be discounted, and a per-close false positive teaches its reader to discount
it.

### The rule the fix follows: exact, never a class

`seedScaffoldExcludes` (internal/posse/worktree.go) names only a path the tree
ACTUALLY holds as posse's own render: `.beads/redirect` when it is a regular
file, and a declared `worktree_link:` path when it is a SYMLINK. Three things
follow from that, and each is a thing the fix deliberately does not do:

- **A declared path git checked out gets no pattern.** An operator who
  declares a tracked directory by mistake must not then have the seat's NEW
  files under it go invisible to `git status`. That is the silent-loss mirror
  of this bug, and it is worse: a spurious P1 is read by a human, a missing
  untracked file is read by nobody.
- **No `.beads/`.** The issue's own side note suggests excluding the bead
  directory so a `git add -A` salvage cannot commit the queue. Declined here:
  `.beads/issues.jsonl` is TRACKED in repos that are not this one — that is
  what `bd sync` is for — and posse does not get to decide that for them.
  This repo ignores `.beads/` in its own `.gitignore`, which is the
  operator's call and stays the operator's.
- **No glob.** A crashed `seedBeadsRedirect` can leave a `.beads/.redirect-<pid>-<ns>`
  temp behind, and `/.beads/.redirect-*` would hide it. Not written: nobody
  has measured that it happens, and a pattern for scaffolding that does not
  exist is a line in the operator's file that says nothing.

The write is `O_APPEND` with the whole block in one `Write`, and the dedupe is
over the file's own lines. Two launchers seeding two trees of the same repo
reach that file concurrently, and an append under `PIPE_BUF` is not
interleaved; a read-modify-write with a rename would be atomic per writer and
would lose the other's lines — and the operator's own, if they were editing
it.

### The pin

`TestFreshSessionTreeIsPorcelainCleanBeforeTheSeatTypes`
(internal/posse/worktree_test.go, arm 2) builds the reported shape — main
checkout with `.bob` as a directory and `.gitignore` spelling it `.bob/`,
`worktree_link: .bob`, a `.beads` in the repo — and asserts the fresh tree's
`git status --porcelain --untracked-files=all` is empty. Its controls are the
part that makes it a measurement rather than a tautology: the main checkout's
`.bob` is clean (so the operator's ignore is not the bug), the tree really
holds the symlink and the redirect (so an unseeded tree cannot pass), and the
seat's own untracked file still shows afterwards (so the fix did not trade a
spurious P1 for a silent loss). A relaunch arm pins both halves of
idempotence: no second copy appended, and an exclude the operator deleted is
repaired.

`TestWorktreeLinkOverATrackedPathIsNotExcluded` is the tracked-path rule on
its own.

MUTATIONS RUN 2026-10-09, each one red:

| mutation | red |
|---|---|
| drop the `seedScaffoldExcludes` call | PorcelainClean |
| spell the patterns with `excludeFromGit`'s trailing slash | PorcelainClean |
| `--git-common-dir` → `--git-dir` | PorcelainClean |
| exclude the redirect only | PorcelainClean |
| exclude the links only | PorcelainClean |
| exclude every declared link, symlink or not | OverATrackedPath |
