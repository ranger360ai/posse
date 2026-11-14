## The gate that refused the branch's own bead was a binary that predates the exemption (ranger-base-li3up)

2026-11-14

ranger-base-li3up was filed as a defect in check 4's own-bead exemption: in a
session worktree whose `posseBead` record read the bead, the
`prepare-commit-msg` gate refused a commit whose subject named that same
bead, and named it as the hit. AGENTS.md "Landing the plane" says the
commit-time check drops the one id the branch is recorded for, so the report
was exactly right about the contradiction.

**There is no defect in the tree.** The exemption works, on the reported
shape, in a real session worktree, through a real `git commit`. What judged
that commit was an installed binary built before the exemption existed. This
fragment is the A/B that says so, and the answer to the open question the
bead left behind.

The root cause is already read in full one bead over —
`docs/notes.d/ranger-base-v81vo.md`, which measured the same installed
binary, priced the install and handed off the category gap. Read that for the
box state and the category work; this is the symptom side, and the two do not
restate each other.

### 1. The A/B, MEASURED

MEASURED 2026-11-14, this seat, worktree `posse/dinesh-posse-ranger-base-li3up`
at `b6baef66`, linux.

| reading | value |
|---|---|
| installed `posse --version` | `0.5.3+7d0f8c31 (herdr-native)` |
| its `vcs.revision` / `vcs.time` (`go version -m`) | `7d0f8c31…196e` / `2026-11-14T01:39:54Z`, `vcs.modified=false` |
| `7d0f8c31` is an ancestor of `e3b41b30` (the `ranger-base-ed07e` landing) | yes |
| `vulnPrivateOwnBead` occurrences in `internal/posse/vulnprivate.go` at `7d0f8c31` | 0 |
| this tree's own build | `0.5.3+b6baef6` |

The fixture is a one-commit repo, a linked worktree on `posse/<session>` with
`branch.posse/<session>.posseBead` recorded, and a stub `bd` reached through
`RHQ_BD_BIN` answering the scan's one `list --json` query with two open
`vuln-private` rows. The message is the one the bead reports, verbatim
subject, with the other id as a bare bullet under "Not closed here".

Piping that message into `posse gates vuln-private-scan --message` with the
worktree as cwd — which is what the hook does, see §3:

| binary | rc | output |
|---|---|---|
| installed `0.5.3+7d0f8c31` | 1 | one line, naming the committing tree's OWN bead |
| this tree's `0.5.3+b6baef6` | 0 | empty |

And through an actual `git commit` in that worktree, under a
`prepare-commit-msg` hook mirroring `vulnPrivateChecks`' one line: **refused
under the installed binary, succeeded under this tree's build.** Same tree,
same message, same hook, same stub store; the binary is the whole of the
difference.

### 2. Three controls, so "clean" cannot mean "the query failed"

Same fixture, this tree's build, the only variable being the message or the
tree:

| case | rc | names |
|---|---|---|
| own bead's own subject (the reported shape) | 0 | — |
| ANOTHER open residual named with text, beside the own bead's subject | 1 | the other id, and only it |
| the own bead's subject, run from the fixture's MAIN checkout (no record) | 1 | the own bead |

So the exemption is one id wide and not a waiver, the live query is answering
rather than erroring into silence, and a tree that answers for no bead still
gets nothing — `ranger-base-n2veh`'s strip and `ranger-base-ed07e`'s
no-record arm, both holding. The installed binary flags the own bead in every
one of those cases, which is the bead's symptom and both of its halves: when
another residual was named too, it printed two lines and the own bead was one
of them.

### 3. The open question on the bead, answered

The bead's comment left one thing to measure: whether `vulnPrivateOwnBead`
answered the record at all in that tree, it being a git call against the
directory the hook is handed. It does, and the environment a commit has is
not the environment the existing pins run in — so the question was worth
asking.

MEASURED, from inside a `prepare-commit-msg` hook in a linked worktree:

| what the hook is handed | value |
|---|---|
| cwd | the worktree's top level |
| `GIT_DIR` | `<repo>/.git/worktrees/<session>`, absolute |
| `GIT_INDEX_FILE` | `<that>/next-index-<pid>.lock` |
| `GIT_PREFIX` | empty |
| `git -C . symbolic-ref --short HEAD` | `posse/<session>`, rc 0 |
| `git -C . config --get branch.posse/<session>.posseBead` | the bead, rc 0 |
| `git -C . rev-parse --git-dir` | the absolute admin dir |

Both reads the exemption makes answer correctly under that environment. An
ambient `GIT_DIR` is not overridden by `git -C`, so it is a real input to
both of them — but the branch config lives in the shared config the linked
worktree reads anyway, and the admin dir the detached fallback parses is
exactly what `GIT_DIR` names.

**No pin was added for it, deliberately.** `git()` builds its children
through `protocolEnv`, which replaces `PATH` and inherits everything else
from the process, so a test could only carry that environment by setting it
process-wide — and `internal/posse` runs its tests in parallel, where a
process-wide `GIT_DIR` would reach every other test's git calls. That is a
flake generator standing in for a guard against a git behaviour change that
no posse code can cause. The reading above is the durable form.

### 4. What this bead did not change, and who has the rest

- The install is `ranger-base-yj80h` (P1, the operator's — `posse upgrade` is
  fenced on an `install_delegate:` key this instance does not set). Note the
  floor: that bead asks for `2839ad8e` or later, which PREDATES `e3b41b30`,
  so its literal minimum would not carry this exemption. An install of the
  current `main` cures both.
- The reason nothing on the box said so is `ranger-base-m1obc` — a third
  `lagCategory` row for a gate the binary judges at COMMIT time, which
  `ranger-base-v81vo` measured and handed off.
- One surface neither of those covers: the refusal itself names no binary.
  `posse gates <persona>` prints the lag, and `m1obc` is making it name this
  category — but a committer refused inside the lag window is reading a
  refusal, and that text, and the rendered hook body behind it, carry no
  stamp of what judged them. Filed from here as `ranger-base-g1jqt`, which
  also asks whether `m1obc`'s row should own it as a fourth surface instead.

### 5. The workaround on the record was the only route available

The bead reports the commit landing with the id alone on its own line, and
calls that the bare-id shape `ranger-base-ed07e` set out to end. It was the
only route a dispatched seat had. The installed hook body is the same lag as
the judge: as rendered on this box it spells `posse gates vuln-private-scan`
with no `--message` at either call site (pre-`ranger-base-gi6ge`), and its
message-arm way-through does not carry the exemption sentence at all —
`grep -c "One id is never flagged here"` over
`~/src/posse/.git/hooks/prepare-commit-msg` reads 0. So the remedy did not
mislead; it had nothing to say about an exemption the binary behind it did
not have. The documented override is operator-typed and never passed by
dispatch, which leaves rewriting the message, which is what happened.
