## The safe form cannot introduce a NEW file (rangerhq-4pbt)

`Bash(git commit unless --)` demands a pathspec, and a pathspec only matches
a file git already has an **index entry** for. So the one form the wall
permits cannot land a file that does not exist in the index yet. gilfoyle hit
it live, landing rangerhq-8rtf's audit script:

```
$ git commit -F - -- NOTES.md Makefile scripts/audit-silent-reverts.sh scripts/silent-reverts.allow
error: pathspec 'scripts/audit-silent-reverts.sh' did not match any file(s) known to git
error: pathspec 'scripts/silent-reverts.allow' did not match any file(s) known to git
exit 1
```

**It is a gate-UX problem, and the dead end is the risk, not the error.** git
refuses *before* either wall runs, so neither layer gets to say anything — a
persona following the refusal's own `safe form:` line gets git's message and
no route out of it. The obvious next reach is `git add` + `git commit`, which
the wall refuses a second time **with the same line that just failed them**.
Two refusals and no way through is how ef8d35f's private `GIT_INDEX_FILE`
recipe gets reinvented (rangerhq-2f5r recorded it; rangerhq-8rtf is it
silently reverting a landed P1 fix for 3h52m).

**The route, measured end to end (git 2.39.3, macOS 25.4; used for a713e37
and pinned in `commitwall_qa_test.go`):**

```sh
$ git add -- <the new paths>
$ git commit -F - -- <all your paths>
```

| form | result |
|---|---|
| `git commit -F - -- <untracked>` | `did not match any file(s) known to git`, exit 1, no hook reached |
| `git add -- <new>` then `git commit -F - -- <new> <old>` | both land; another persona's staged entry untouched |
| shared index afterwards | clean for the committer — the path-limited commit refreshes it for the newly tracked file too |
| bare `git add` (no pathspec) | `Nothing specified, nothing added.`, exit 0 — a no-op, not a sweep |
| `git add -A` | stages **every** persona's new and modified file into the shared index |

So the add is scoped with `--` or it is the shared-index write the wall exists
to prevent.

**Why not `git add -N`, which also makes the pathspec match.** It looks like
it removes the residual below, and under the wall it buys nothing. Measured,
same box, a new file plus another persona's staged entry, per sweeping form:

| sweeping form | untracked | after `git add --` | after `git add -N --` | wall |
|---|---|---|---|---|
| `git commit -m` (unqualified) | not taken | **taken** | not taken | refused |
| `git commit -i -m x -- <theirs>` | not taken | **taken** | not taken | refused (rangerhq-ojnw) |
| `git commit -a` | not taken | **taken** | **taken**, full content | refused |
| `git commit -m x -- .` | not taken | **taken** | **taken**, full content | **passes** |

`-N` helps against exactly the three forms the wall already refuses and
against none of the one it lets through — the `.` pathspec named in the
refusal, which only rangerhq-09o2 closes. It also converts an untracked file,
which `-a` and `.` skip outright, into one they take. A less-known flag that
buys nothing under the wall is not the form to teach, so the prescription is
the plain scoped add.

**The residual, stated rather than closed.** Between the `git add` and the
commit your entry is in the shared index, open to another persona's
unqualified commit — rangerhq-nyqj exactly. It is narrow only because the wall
refuses that form for personas and the operator is exempt; keep the two
commands adjacent and name the new paths in the commit. rangerhq-09o2's
per-session worktrees remove the shared index and the whole question.

**Where it is said now.** Both layers name the route, because they fail in
different trees: L3 (`sharedIndexBody`) carries the full form with the
residual, and L1 carries two lines from `qualifierPrereqs` — keyed by command
and subcommand like `qualifierSpoilers`, which is the one seam where a
git-specific fact reaches the general shim grammar. L1 is not redundant: in a
session worktree the L3 wall stands down by design (rangerhq-09o2), so the
shim is the only layer that speaks, and the dead end is identical there.
