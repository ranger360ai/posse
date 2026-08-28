## The worktree beads redirect: bd does not read it, and the cage does (ranger-base-vczf)

**This CORRECTS the "Without a redirect the graph DOES fork, silently" bullet
in the rangerhq-09o2 section of NOTES.md, and the matching bullet that used to
head worktree.go's WHAT WAS MEASURED block.** That claim does not hold on bd
0.49.1. Measured 2026-08-28, bd 0.49.1, git 2.39.3, in throwaway repos.

**bd resolves a linked worktree to the MAIN checkout's `.beads` by itself.**
Not "as well as" the redirect — *instead of* it. While the main checkout has a
`.beads`, bd does not read the worktree's own `redirect` at all:

| worktree `.beads` | main checkout `.beads` | `bd where` in the worktree |
|---|---|---|
| absent | database | the main checkout's |
| checked-out `issues.jsonl`, no db | database | the main checkout's |
| absent | *redirect only*, to another repo | the redirect's target (the chain is followed from the MAIN checkout) |
| **`redirect` → another LIVE database** | database | **the main checkout's — the redirect is ignored** |
| `redirect` → another LIVE database | *none* | the redirect's target |

The last two rows are the finding. A *misdirected* redirect in a worktree
changes nothing about which rows come back; bd falls back to reading the local
one only when the main checkout has no `.beads` at all — which is the one
shape `seedBeadsRedirect` declines to write for. `bd where` reports its answer
as "(via redirect from `<worktree>/.beads`)" even when the file says something
else entirely, so that line is bd narrating a hop, not quoting the file.

The "fresh clone" case does not fork either: with a tracked `issues.jsonl` and
no database yet, bd builds `beads.db` in the **main checkout's** `.beads`, and
the worktree reads it.

**So what is the redirect for? posse.** `beadsHome` (beadloss.go) resolves it,
and the seatbelt writable set and the codex launch line are built from what it
answers (ADR 0012 D3-C). With no redirect, `beadsHome(tree)` answers the
worktree's own `.beads` — a directory bd never opens — so a caged persona is
granted the wrong path and denied the right one, and bd reports `failed to
open database: … operation not permitted` out of a resolution that was correct
all along. That is ranger-base-0fb verbatim. The redirect is not belt-and-
braces for the graph on today's bd; it is the cage's only account of where the
store is, and it stays the belt for a bd that loses the worktree resolution.

**What that does to the pin.** `TestLiveWorktreeSharesOneGraph` passed with
`seedBeadsRedirect` suppressed entirely, which is what filed this bead: every
arm it had — main rows visible, no second database, a bead filed from the
worktree landing in the main database, no staleness warning — is bd's doing.
No arm of the form *did the graph fork* can be made to fail for want of the
redirect on this bd, so "plant a misdirection and require the fork to show"
was not available; it was measured and it does not fork.

The two honest arms, both in `internal/rhq/worktreelive_test.go`:

- *Agreement*, in `TestLiveWorktreeSharesOneGraph`: the directory posse seeds,
  the directory `beadsHome` resolves, and the directory `bd where --json`
  names are one. This is the arm that fails without `seedBeadsRedirect` (and
  it fails again if the seeded path is merely wrong, which is the failure mode
  that would otherwise be silent — bd reading the right database while the
  cage grants the wrong one).
- *The environment*, `TestLiveWorktreeBdResolvesTheWorktreeItself`: the two
  bottom rows of the table above, pinned. Its second half is the positive
  witness the first needs — the same planted redirect, over a main checkout
  with no `.beads`, must be honoured, or "the plant changed nothing" would be
  satisfied by a plant bd could not read. If the first arm ever goes red,
  posse's redirect has become load-bearing for the graph too.

Mutation-checked both ways on 2026-08-28: suppress `seedBeadsRedirect` →
`TestLiveWorktreeSharesOneGraph` red (and `TestCodexLaunchLineNamesTheStore-
OfRecord` plus the three `TestSessionTree*Redirect*` unit pins red with it);
make it seed a path that does not resolve → red again;
`TestLiveWorktreeBdResolvesTheWorktreeItself` stays green under both, which is
correct — it pins bd, not us.
