# ADR 0051 — "Landed" is cited by bead id; a sha in a record is a measurement against main, never a session tree's

*Status: accepted 2026-09-02 (ranger-base-xzg0x, from ranger-base-a3zvb
Finding 2) · owner: architect · binds every record under `docs/adr/`
forward; amends nothing in place — the twelve stale stamps are a sweep bead ·
number: per ADR 0040 §3.1 this file takes the next number no bead has
claimed (0043–0045 stay pre-named).*

## Context

A persona closes a build bead and writes "landed `c067486`
(ranger-base-uzw11)" into the ADR that asked for it. The only sha it can
know is its own session tree's. The launcher lands that tree with
`merge --ff-only`, and when main has moved since the tree was cut it
**rebases first** (`worktree.go`, `MergeSessionWork`, the `o.Rebased` arm),
which mints a new sha. The stamp then names an object that exists in this
clone's object store, sits on no ref once the branch is retired, is not an
ancestor of main, and is prunable by git's default gc expiry.

**MEASURED 2026-09-02 at 53e1bec** (recipe in ranger-base-xzg0x, rerun here):
of 39 seven-hex tokens in `docs/adr/*.md`, 32 resolve to a commit; 20 are
ancestors of HEAD, 11 are on no ref at all, 1 (`c9a4cdd`) is on a ref but
not on main. Every one of the 12 has a **patch-id-equivalent commit on
main** (`git cherry HEAD <sha> <sha>^` prints `-` for all twelve), so no
work was lost — only the name of it. The landed twins:

| stale | landed | stale | landed | stale | landed |
|---|---|---|---|---|---|
| c067486 | 37c1a5e | a98ed0e | 521d3db | b69c07a | b58c957 |
| d9ed77c | f746ba5 | b065c07 | 950c984 | 074f661 | 5fbb28c |
| a86ec3f | eb8f716 | 18cb114 | 8a2a58a | d505f2c | ae7b08f |
| d9fa52f | db1a042 | d6022fc | 1e9b2ba | c9a4cdd | 1e9b2ba |

**MEASURED, the base rate:** `dispatch-watch.log` (through 2026-08-31)
holds 134 landings, 48 of them "rebased and fast-forwarded" — 36%. The
stamp census says 12 of 32 — 37%. The two agree: a sha copied out of a
session tree goes stale a third of the time, by construction, and nothing
about the writer's care changes that. Two of the twelve were noticed in a
week (uzw11, by a verify); ten were not — a convention only a verify
catches is a convention with a 5-in-6 miss rate.

**MEASURED, the resolver that survives a rebase:** of the last 600 commits
on main, 583 carry a bead id in their message (97%); the 17 that do not are
operator commits, which never pass through the launcher and so never
re-sha. A rebase rewrites the sha and keeps the message, so `git log --grep
<bead id>` answers on every box the change reaches, forever.

## Decision

**D1 — the citation of record for a landed change is the bead id.** A
record says "landed (ranger-base-uzw11)" or "built on ranger-base-t8tq";
the resolver is `git log --grep <id>`. When a bead's commits are several
and one is meant, quote the commit subject beside the id — the subject
survives the rebase, the sha does not.

**D2 — a sha may appear in a record only as a measurement against the
repo's branch.** The writer ran `git merge-base --is-ancestor <sha> main`
(readable from any session worktree — `refs/heads/main` resolves there)
and it said yes, at the HEAD the record's status line dates. That is the
shape a verify or an audit already uses (ORDERS 4wxko: "never bd show"),
and it is the only shape in which a sha is a fact rather than a guess. By
construction no commit a persona made in its own session tree passes this
test until the launcher has landed it, so a build close cites the bead id
and nothing else. A verbatim quote of tool output that carries a session
sha stays a quote; the landed twin goes beside it in editorial brackets
(the 1d8bk shape), not in place of it.

**D3 — the twelve are re-stamped from the table above, sha for sha,** the
bead id kept where it already stands. One code-lane bead, closable in a
session, verified by the census recipe returning 0 non-ancestors. This
keeps `git show` working for the reader who has the record open; the
alternative — strip the shas and leave the ids — throws away a resolver
that works today for nothing.

**D4 — the rule is enforced where it is broken: the writer's commit.** The
prepare-commit-msg hook that already scans the ADDED lines of staged
markdown (`gates.go`, the visibility guard's check 2 scope, ADR 0024 D2 /
0048 D2) gains a sibling check over the ADDED lines of staged `docs/adr/`:
every 7–40 hex token that `git cat-file -e <tok>^{commit}` resolves
locally AND `git merge-base --is-ancestor <tok> <base>` refuses is a
refusal, naming the token and this rule. The base is the branch the main
checkout has checked out (`git --git-dir=$(git rev-parse --git-common-dir)
symbolic-ref -q HEAD`); when that is detached the check judges nothing and
says so — a gate that cannot find its base does not guess (0019's
composite: no fallthrough on a read failure). Tokens that do not resolve
here are passed: they are prose, or another repo's, and this hook cannot
judge them. Cost, MEASURED: cat-file plus merge-base is ~50 ms per token.
The refusal text is the teaching — a sentence in a record does not reach a
writer at the moment they paste a sha; a refusal does (ORDERS zbd51: a diff
lands, a sentence does not).

## Alternatives rejected

- **(b) the launcher writes the landed sha.** It is the only actor that
  knows it, and `mergeBack` holds the bead id and the bd handle at the
  moment. But it cannot edit the ADR line — it does not know which one —
  so the write would be a bead comment, a second store for a fact git
  already holds under `git log --grep`, costing one bd write per close
  under the launcher lock (the lock-storm class, bd-fingerprint-mismatch).
  It fixes the record for nobody who opens the record. Rejected; if a
  second hop is ever wanted, this is the shape, and the cost is one line.
- **(c) re-stamp by hand when a verify catches one.** The standing cost is
  the ten nobody caught. Rejected by measurement.
- **A suite pin that walks `docs/adr` and asserts every sha is an ancestor
  of HEAD.** In the writer's worktree the sha IS an ancestor of HEAD; the
  pin goes red on main, for the next stranger who runs the suite
  (suite-red-that-is-not-the-diff). The red must belong to the writer, so
  the check lives in their commit hook, not in the suite. A read-only
  census script is fine and is the sweep's verify; a pin is not.
- **Strip every sha, keep only ids.** Loses `git show` for twenty stamps
  that resolve today, and the measured-against-main shape D2 keeps is the
  one audits need. Rejected.
- **Let the hook skip blockquotes and fenced blocks.** The 1d8bk ruling
  refused this escape for paths and the reason holds for shas: the quote
  stays, the resolvable twin goes beside it in brackets. A skip would also
  be the first place a writer learns to put a stamp.

## Consequences

- Records stop asserting a sha they cannot know. The 36% goes to 0 for new
  stamps, at the cost of one refusal per paste and the bracket habit.
- The twelve become resolvable on every clone. Until the sweep lands they
  are still resolvable HERE by patch-id (`git cherry`), which is how the
  table above was made; that path dies at the first gc that prunes them
  (git default `gc.pruneExpire` two weeks — ASSUMED from the default, this
  box has no override set; the 08-27 objects reach it around 09-10).
- The hook's base read adds one dependency on the main checkout's
  `symbolic-ref`; the exit hatch is the detached arm, which judges nothing.
- ADR 0006 §3's verify shape and 0040's audit tables are unchanged: they
  already cite by ancestry. What changes is what a build close may write.

## Verification

- The census recipe (ranger-base-xzg0x) at the sweep's landed HEAD returns
  0 non-ancestors among resolving tokens. MEASURED at 53e1bec: 12.
- The hook refuses a staged `docs/adr` line adding a sha that resolves and
  is not on main, and passes the same line with the sha replaced by its
  landed twin; passes a token that does not resolve; refuses nothing when
  the main checkout is detached, and prints that it judged nothing.
- Base rate after 30 days of closes: new stamps that are non-ancestors of
  main = 0, against a prior of one in three.

## Measured versus assumed

MEASURED: the 39/32/20/11/1 census; twelve patch-id twins; 48/134 rebased
landings; 583/600 messages carry an id; per-token hook cost; `refs/heads/main`
and the common-dir `symbolic-ref` resolve from a session worktree.
ASSUMED: gc expiry is git's default on this box; the 17 id-less commits are
all the operator's (not walked, inferred from the launcher's path); the
refusal changes writer behaviour rather than teaching the override.
