# Historical snapshot — not current policy

Archived 2026-09-05. Current decision: [ADR 0051](../0051-landed-is-a-bead-id.md).

# ADR 0051 — "Landed" is cited by bead id; a sha in a record is a measurement against main, never a session tree's

*Status: accepted 2026-09-02 (ranger-base-xzg0x, from ranger-base-a3zvb
Finding 2) · owner: architect · binds every record under `docs/adr/`
forward; amends nothing in place — the twelve stale stamps are a sweep bead ·
number: per ADR 0040 §3.1 this file takes the next number no bead has
claimed (0043–0045 stay pre-named) · amended 2026-09-03 (D2/D3/D4 and
Verification 1, new D5: a record ABOUT stale shas — this one — carries the
landed twin in the same record, and the hook and the census admit exactly that;
ranger-base-mlfie, from the D3 sweep ranger-base-0ujt7).*

## Context

A persona closes a build bead and writes "landed `c067486`
(ranger-base-uzw11)" *[the session sha; landed as `37c1a5e`]* into the ADR
that asked for it. The only sha it can know is its own session tree's. The launcher lands that tree with
`merge --ff-only`, and when main has moved since the tree was cut it
**rebases first** (`worktree.go`, `MergeSessionWork`, the `o.Rebased` arm),
which mints a new sha. The stamp then names an object that exists in this
clone's object store, sits on no ref once the branch is retired, is not an
ancestor of main, and is prunable by git's default gc expiry.

**MEASURED 2026-09-02 at 53e1bec** (recipe in ranger-base-xzg0x, rerun here):
of 39 seven-hex tokens in `docs/adr/*.md`, 32 resolve to a commit; 20 are
ancestors of HEAD, 11 are on no ref at all, 1 (`c9a4cdd` *[landed as
`1e9b2ba`]*) is on a ref but not on main. Every one of the 12 has a
**patch-id-equivalent commit on main** (`git cherry HEAD <sha> <sha>^`
prints `-` for all twelve), so no
work was lost — only the name of it. The landed twins:

| stale | landed | stale | landed | stale | landed |
|---|---|---|---|---|---|
| c067486 | 37c1a5e | a98ed0e | 521d3db | b69c07a | b58c957 |
| d9ed77c | f746ba5 | b065c07 | 950c984 | 074f661 | 5fbb28c |
| a86ec3f | eb8f716 | 18cb114 | 8a2a58a | d505f2c | ae7b08f |
| d9fa52f | db1a042 | d6022fc | 1e9b2ba | c9a4cdd | 1e9b2ba |

*(amended 2026-09-03)* The stale column is this record's content and stays;
every row carries the twin beside it, which is the D5 shape, so the table is
judged by the hook and the census and passes. The two prose mentions above
carry their twins in editorial brackets for the same reason.

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
(the 1d8bk shape), not in place of it. *(amended 2026-09-03)* The bracket
is not a courtesy, it is the shape the gate checks: D5 says what "beside"
means.

**D5 — a record whose subject is a stale sha carries the landed twin in the
same record, and that is the only exemption the hook and the census know.**
A census, an incident writeup, a table of what went stale, this file: the
stale sha IS the content and cannot be re-stamped (the sweep bead measured
it — re-stamping the table above would leave its last row reading
`1e9b2ba` four times). Such a record is not excluded from the check; it
passes it, because each stale sha has its twin beside it. Mechanically, per
record: tokens that resolve are split into ancestors of the base and
non-ancestors; a non-ancestor is admitted when an ancestor anywhere in the
same file has the same patch-id (`git diff-tree -p <sha> | git patch-id
--stable`), and refused otherwise, naming it. The twin must be a twin — an
unrelated ancestor beside a stale sha does not admit it (MEASURED: the
census refuses `c067486` beside main's HEAD and admits it beside
`37c1a5e`). That is what keeps D2's construction: a sha minted in a session
tree has no twin on main until the launcher lands it, so no line a build
close writes can pass by decoration — the only way through is still the
bead id. The radius is the record, not the line, and that was measured,
not chosen: the line form refused all three bracketed prose mentions this
amendment wrote, because 76-column prose wraps the bracket onto the next
line; a paragraph radius needs a markdown block parser inside a shell hook.
The radius carries no safety — the twin's nonexistence before landing
carries all of it — so the widest radius the hook can compute in one grep
is the right one. Adjacency stays D2's style rule for the reader, not the
gate's demand.

**D3 — the twelve are re-stamped from the table above, sha for sha,** the
bead id kept where it already stands. One code-lane bead, closable in a
session, verified by the census returning 0 refusals under the D5 predicate
(`posse gates adr-census`, which prints judged / admitted-by-twin /
refused so that a zero over a pruned object store reads as "nothing
judged", not as clean; *editorial, 2026-09-03, ranger-base-gyrko: the
census was `scripts/adr-sha-census.sh` until the mode landed*). This keeps
`git show` working for the reader who has the record open; the alternative
— strip the shas and leave the ids — throws away a resolver that works
today for nothing. *(amended 2026-09-03: the
criterion read "0 non-ancestors", which this file makes unreachable by
construction — the sweep landed at ranger-base-0ujt7 with the recipe still
printing 12, all twelve sourced from the table above. Under D5, MEASURED at
ffec279: 58 distinct tokens judged over docs/adr, 46 ancestors, 12 admitted
by twin, 0 refused — and 0 on this file as it stood on main, because the
table's twins already sit in the record; the brackets on Context's two
prose mentions are D2's shape for the reader, not what the gate demanded.)*

**D4 — the rule is enforced where it is broken: the writer's commit.** The
prepare-commit-msg hook that already scans the ADDED lines of staged
markdown (`gates.go`, the visibility guard's check 2 scope, ADR 0024 D2 /
0048 D2) gains a sibling check over the ADDED lines of staged `docs/adr/`:
every 7–40 hex token that `git cat-file -e <tok>^{commit}` resolves
locally AND `git merge-base --is-ancestor <tok> <base>` refuses is a
refusal, naming the token and this rule — *(amended 2026-09-03)* unless an
ancestor somewhere in the same staged file is its patch-id twin (D5): the
non-ancestors come from the added lines, the candidate twins from the whole
staged blob, and a match is the bracket shape and passes. The patch-id pair
is computed only when a file holds both a non-ancestor and an ancestor, so
the common case pays nothing extra (MEASURED: ~60 ms per pair). The census is
this same predicate run over every line of every `docs/adr` file instead of
the staged added lines — one predicate, two line sources, so the gate and
the sweep's verify cannot disagree about what is exempt; a second copy of
the rule in prose or in a script is the thing this amendment exists to
prevent, and the reference script under `scripts/` is retired when the
hook's own text can be run in census mode *(done: `posse gates adr-census`
renders the hook's predicate from the same Go function, ranger-base-gyrko,
2026-09-03)*. The base is the branch the main
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
- *(amended 2026-09-03, the shapes priced for a record about stale shas)*
  **An explicit marker the hook and census both honour** (a comment, a
  strikethrough, a `stale:` prefix). Free to type, so it is the first thing a
  refused writer learns, and it says nothing a reader can resolve. Rejected.
  **Name this file (and successors) in both.** The next census or incident
  table needs an ADR amendment before it can be written, and the named file
  becomes the one place a stale sha is never judged — the opposite of what a
  record about staleness wants. Rejected. **Any ancestor in the record admits
  a stale one** (the weak form of D5). One paste of main's HEAD beside a
  session sha passes, and a build close can do that from its own tree; the
  patch-id form cannot be passed before landing, which is D2's construction.
  Rejected by the fixture measurement in D5. **The line as the radius.**
  Refused the first three bracketed sentences written under it (wrapping),
  and the paragraph form costs a block parser in a hook. Rejected by
  running it. **Exclude this file from the census recipe only.** Leaves the hook refusing every future edit to the
  table's region, and the two disagree about what is exempt. Rejected.

## Consequences

- Records stop asserting a sha they cannot know. The 36% goes to 0 for new
  stamps, at the cost of one refusal per paste and the bracket habit.
- The twelve become resolvable on every clone. Until the sweep lands they
  are still resolvable HERE by patch-id (`git cherry`), which is how the
  table above was made; that path dies at the first gc that prunes them
  (git default `gc.pruneExpire` two weeks — ASSUMED from the default, this
  box has no override set; the 08-27 objects reach it around 09-10).
  *(amended 2026-09-03)* After that prune the twelve stop resolving and the
  table is prose to the hook and the census alike: "12 admitted by twin"
  becomes "0 judged" in the census summary, and a fresh clone reads the same.
  That is D4's stated limit, not a hole in D5 — the record still resolves by
  its landed column.
- The hook's base read adds one dependency on the main checkout's
  `symbolic-ref`; the exit hatch is the detached arm, which judges nothing.
- ADR 0006 §3's verify shape and 0040's audit tables are unchanged: they
  already cite by ancestry. What changes is what a build close may write.

## Verification

- *(amended 2026-09-03; editorial 2026-09-03, ranger-base-gyrko: the
  recipe was `sh scripts/adr-sha-census.sh` until census mode landed)*
  `posse gates adr-census` at the repo's branch prints 0 refused, and its
  summary line shows a non-zero judged count. MEASURED at 53e1bec under the
  old criterion: 12; at ffec279 under D5: 0 refused with 58 distinct tokens
  judged, 12 of them admitted by twin; at 57f1185 the retired script and
  the mode printed the same summary (58 judged, 46 ancestors, 12 admitted
  by twin, 0 refused) and the same twelve ADMITTED lines. The old
  criterion — 0 non-ancestors among resolving tokens — is unreachable while
  this file states its evidence and is withdrawn.
- The hook refuses a staged `docs/adr` line adding a sha that resolves and
  is not on main, and passes the same line with the sha replaced by its
  landed twin; passes a token that does not resolve; refuses nothing when
  the main checkout is detached, and prints that it judged nothing.
  *(amended 2026-09-03)* And: passes a line adding a stale sha when its
  patch-id twin is anywhere in the staged file; refuses the same line when
  the file holds an ancestor that is not its twin and no twin; re-adding
  this file's table region passes. The hook and the census agree on the whole fixture set (a two-way pin — a line the
  hook refuses the census refuses, and the reverse).
- Base rate after 30 days of closes: new stamps that are non-ancestors of
  main = 0, against a prior of one in three.

## Measured versus assumed

MEASURED: the 39/32/20/11/1 census; twelve patch-id twins; 48/134 rebased
landings; 583/600 messages carry an id; per-token hook cost; `refs/heads/main`
and the common-dir `symbolic-ref` resolve from a session worktree.
ASSUMED: gc expiry is git's default on this box; the 17 id-less commits are
all the operator's (not walked, inferred from the launcher's path); the
refusal changes writer behaviour rather than teaching the override.

*Amendment claims (2026-09-03, ranger-base-mlfie).* MEASURED: the census at
ffec279 prints 12 `NOT ON MAIN`, all from this file (the sweep's own
finding, rerun); all twelve table pairs are patch-id equal (`git patch-id
--stable` per pair; `git cherry` prints `-` for each); the predicate over
docs/adr under the line radius refuses the 2 prose mentions and nothing
else, and under the record radius judges 58 distinct tokens, admits 12 and
refuses 0, before and after the bracket edits; the fixture refuses a stale sha beside
main's HEAD and admits it with its twin two paragraphs away; the line radius
refused three freshly bracketed sentences; ~60 ms per patch-id pair; the
whole census over docs/adr runs in ~7 s. MEASURED (2026-09-03, ranger-base-gyrko): the census is the hook's own
predicate text rendered from one Go function, and the two-way pin in
Verification holds over the whole fixture set — what this sentence ASSUMED
of the reference script until census mode landed; a record that holds a
twin table admits its twelve anywhere in itself, which is a reader's puzzle
at worst, never a false citation.
