# ADR 0054 — A silent-revert triage line survives the launcher's rebase: it carries the diff's patch-id, and the patch-id claims one commit

*Status: accepted 2026-09-04 (ranger-base-h6g65, from ranger-base-ur2eo, from
ranger-base-2fl9a finding 1) · owner: architect · binds
`scripts/audit-silent-reverts.sh` and `scripts/silent-reverts.allow` ·
amends nothing; ADR 0051 D5 is the precedent it copies, not a record it
changes · number: per ADR 0040 §3.1, the next no bead had claimed.*

## Context

A persona reads a silent-revert hit in its own session tree and writes the
sha the audit printed into `scripts/silent-reverts.allow`. That sha is the
session tree's. The launcher lands the tree with `merge --ff-only` and
rebases first when main has moved (`worktree.go`, `MergeSessionWork`),
which mints a new sha for the same diff. On main the line then names a
commit no ref reaches, the landed twin is UNTRIAGED, and `make test`
(Makefile, the audit's `--quiet` line) inside `ci.yml` goes red on a hit
that was read and explained. Three allow lines already warn that this
would happen; on 2026-09-04 it did.

**MEASURED at 7a002e9 (this design's HEAD):**

- `e8c5e4e` was triaged; `c8adbcc` is its landed self. Same author, same
  second, same subject, same patch-id `77e50340…`. `git rev-list --all`
  does not contain `e8c5e4e`: it is on zero refs. Main is red *now*: the
  audit prints `c8adbcc` UNTRIAGED over 1223 commits, and the hand-written
  second line the bead describes sits in ranger-base-ur2eo's unlanded
  session tree (its commit "three escapes from the 2fl9a verify batch"),
  not on main.
- The rebase is the common case, not the exception. `dispatch-watch.log`
  by bead id: 211 landings rebased, 107 fast-forwarded plain — two in
  three. ADR 0051 counted 48 of 134 through 08-31; the rate has risen
  since.
- In-session triage of the persona's own commit: 2 of the 8 lines written
  since 2026-08-29 (18c95a5, e8c5e4e); 1 of those 2 was rebased. The other
  6 were written after landing, by a later bead, and name a landed sha.
- `ci.yml` checks out with `fetch-depth: 0`, which fetches every REF's
  history. An object on no ref is never transferred. So on the runner the
  session sha does not resolve at all, and no lookup that starts from that
  object can run there. On this box the same object dies at gc
  (`gc.pruneExpire` unset, git's default is two weeks — ASSUMED from the
  default; e8c5e4e was minted 09-04, so ~09-18).
- A patch-id survives a rebase when the base moved in another file or in
  the same file outside the hunk's context, and CHANGES when the base
  moved inside the hunk's three context lines (scratch repo, four arms,
  git 2.50.1). Every twin pair in this history is equal — the twelve in
  ADR 0051's table, plus e8c5e4e/c8adbcc and 3102403/d309e2b — so the
  context-line case has not happened here yet, and its failure direction
  is stated below.
- `git rebase` DROPS a commit whose diff the base already carries (skipped
  cherry-pick, measured). So a line's sha can be a non-ancestor whose diff
  landed under another name without any rebase of its own.

The tension is the allow file's own header (ranger-base-ztzd): NOT
PATTERNS, EVER. A sha line is a statement about one commit's intent. The
question is what statement survives the rename the launcher performs,
without becoming a standing statement about future commits.

## Decision

**D1 — the key stays the sha; the line may carry the diff's patch-id
beside it.** Format: `<sha> [<patch-id>] <reason>`, where the patch-id is
the 40-hex first field of `git diff-tree -p <sha> | git patch-id --stable`.
The sha is what a reader resolves with `git show`; the patch-id is what
the launcher cannot rename. The existing fourteen lines are untouched and
valid.

**D2 — the patch-id arm fires only for a line whose sha did not land
here.** For a flagged commit that no line names by sha, the audit computes
its patch-id and looks for a line whose second token equals it AND whose
sha is not an ancestor of the scanned tip (does not resolve, or resolves
and `merge-base --is-ancestor` refuses). A line whose sha IS an ancestor
triages by sha alone and its patch-id is inert. This is the ADR 0051 D5
construction turned around: there, a non-ancestor is admitted when its
twin is in the record; here, the record's non-ancestor admits its twin on
the branch. Same predicate, same reason it is safe — a sha minted in a
session tree names nothing on main until the launcher lands it, and the
line and its commit travel in the same branch, so on main a line that
names a non-ancestor is, by construction, a line whose diff landed in the
same landing (rebased) or was already there (dropped).

**D3 — a patch-id claims exactly one commit, the oldest flagged one that
carries it.** A second flagged commit with the same diff is UNTRIAGED. A
line is a statement about one commit; the patch-id lets the commit be
found under its landed name, it does not let the line stand for the next
commit with that diff. This is the whole difference from a pattern: a
glob excuses every future commit that deletes a path, this excuses the one
commit whose diff the writer read, once.

**D4 — the teaching is the UNTRIAGED line.** Where the audit prints "write
the reason in scripts/silent-reverts.allow", it prints the line to paste:
`<sha> <patch-id> <reason>`, patch-id included, every time. Nobody is asked
to know the recipe or to guess whether their commit will be rebased; the
hint carries the token that survives either way. The allow file's header
gains one paragraph stating D1–D3 in its own vocabulary, beside the NOT
PATTERNS paragraph, and says what the widening is.

**D5 — nothing at landing changes.** The launcher does not write. The
allow file is not rewritten. A persona triaging a landed sha after the
fact (six of eight lines) writes the sha as today; the patch-id is inert
for an ancestor and may be omitted.

**The widening, stated as the header asks it: what does this excuse
unread?** The same diff, landed under a different name, once. On main a
line naming a non-ancestor got there in the same branch as the commit it
names, and that branch landed whole — so the one commit with that diff is
the commit the writer read. The exposure is a line that reaches main by
another route than its commit (a merge-back that carries the allow edit
but not the commit): then the first future commit with that exact diff is
excused unread. The diff must be byte-for-byte the one the writer
triaged, so the shape excused is that triage's own re-land — the 1cc432e
shape, benign by construction — and the revert it re-lands over is still
flagged and untriaged, so the incident still reds the gate. The
context-line hole fails CLOSED: a rebase that moved the hunk's context
changes the patch-id, the arm does not fire, and the result is today's
behaviour (UNTRIAGED, a red gate, a second line). No arm of this design
can excuse a commit the writer did not name a diff for.

## Alternatives rejected

- **(a) resolve at audit time from the allow sha's object** — the bead's
  first candidate, the `adrShaPredicate` shape copied whole. On the venue
  where the gate is red the object does not exist: CI is a fresh clone,
  and an object on no ref is never fetched (MEASURED, `rev-list --all`
  empty for e8c5e4e; `fetch-depth: 0` fetches refs). On this box it dies
  at gc, which ADR 0051's Consequences already states as D5's limit. The
  ADR census can afford that limit because it runs on the box that
  minted the objects; the gate runs on a runner. D1 is (a) with the
  patch-id written into the record at the moment the object still exists
  — which is the only moment the writer has anyway.
- **(b) the landing step rewrites the line to the landed sha.** ADR 0051
  rejected the launcher as a writer for the sibling problem; here it is
  worse, not better: the line lives inside the persona's own commit, so
  the rewrite is an edit to a commit's content mid-rebase, under the
  launcher lock, in the tree `seatbelt.go` already notes runs unsandboxed
  at land time, and a landing that edits content is a landing that can
  conflict. It also rewrites nothing in a shared checkout, where there is
  no rebase and no problem. Rejected.
- **(c) the second line, today's state.** Its measured cost is not a
  line: it is main red for the life of a follow-up bead — red since the
  17:41Z run, still red at 7a002e9, the fix in a tree that has not
  landed — because the persona who read the hit cannot write the landed
  sha (their session is retired before it exists) and the one who can
  did not read it. At two rebases in three and one in-session triage a
  week, that is a red gate every fortnight or so (ASSUMED from 2 of 8
  and 211 of 318). Rejected by measurement.
- **(d1) the patch-id as the ONLY key.** Excuses every commit with that
  diff, forever: a second stale-index revert of the same fix has exactly
  the triaged revert's diff, and that is the incident's own shape
  (dcca7b5's diff is the reverse of ef8d35f's). Fails the header's test
  outright. D3 exists because of this arm. Rejected.
- **(e) a commit-message trailer** (`Silent-revert: <path>: <reason>`)
  on the flagged commit itself. The message survives every rebase (ADR
  0051 MEASURED 583/600) and needs no file. But the hit is only visible
  once the commit exists, and `git commit --amend` is refused by the
  commit wall (`gates.go`: it sweeps without a pathspec), so only an
  author who knew BEFORE committing can write it — the deliberate
  replacement or repair shape, which is all nine lines written since the
  rename — and the accident shape, the one the file exists for (dcca7b5),
  can never take it. Two routes for one
  fact, and a trailer is free to template. Rejected for now; it is the
  shape to revisit if the context-line hole starts costing lines.
- **(f) key by the flagged (path, state) pair.** `D docs/PROBE.md` is the
  glob the header refused, wearing the detector's vocabulary. Rejected.
- **(g) in a session tree, skip commits not on the base branch.** Hides
  the session's own hits from the session's own `make test`, which is
  where the stale-index revert has to be caught. Rejected.
- **(h) make the patch-id mandatory; rewrite the grammar.** Touches
  fourteen lines, five of which name pre-rename history that does not
  resolve here, for nothing the optional form does not give. Rejected.

## Consequences

- An in-session triage is green in the session tree (sha match), green
  on main after a rebase (patch-id match), and green on a fresh clone
  (the patch-id is text in the file; the flagged commit is on main). The
  red-gate-per-rebase class closes, except for the context-line case,
  which fails closed to today's behaviour.
- Cost in the common case is zero: the patch-id arm runs only for a
  flagged commit no sha names, which is the state the gate is red in
  anyway. With a hit: one patch-id (~100 ms MEASURED, nine in 0.9 s) plus
  one `merge-base` per line that carries a patch-id.
- Two lines for one diff (a sha line for the twin and a patch-id line for
  the original) are harmless: sha match is tried first, and D3 leaves the
  patch-id line unclaimed. ranger-base-ur2eo's c8adbcc line and e8c5e4e's
  backfilled patch-id coexist.
- The exit hatch is the token: delete the second field from a line and
  it is a sha line again. No state is held anywhere but the file.
- The allow file still cannot excuse a commit by pattern, and the
  detector still cannot tell a tidy-up from an accident and must not
  learn to. Nothing here teaches it.

## Verification

Laurie's checklist; the verify bead quotes it.

1. `--self-test` gains a `twin` arm and its wrong arms, each over the
   modify plant with a planted allow file (the script `cd`s to the
   toplevel of the repo it is run in, so a fixture repo's own
   `scripts/silent-reverts.allow` is the one read):
   - twin: a line naming a sha that does not resolve, carrying the sync
     commit's patch-id — exit 0, and the triage print names the twin.
   - inert: the same patch-id beside the sha of an ANCESTOR that is not
     the flagged commit — exit 1 (D2).
   - mismatch: a non-resolving sha beside a different patch-id — exit 1.
   - one claim: fix, revert, fix again, revert again (two flagged commits,
     one patch-id) and one line — exactly one UNTRIAGED (D3).
2. The UNTRIAGED hint prints `<sha> <patch-id>` for the flagged commit,
   and the patch-id it prints equals `git diff-tree -p <sha> | git
   patch-id --stable`.
3. Go pins in `internal/posse/silentrevert_qa_test.go`: one per arm above,
   plus the existing fourteen unchanged and green; the rigs are shown able
   to fail (the wrong arms are the witness).
4. Production: at main with e8c5e4e's line carrying `77e50340…`, the audit
   prints 0 untriaged over the full history; with the token removed it
   prints c8adbcc UNTRIAGED again.
5. Base rate after 30 days: in-session triages whose landed twin was
   UNTRIAGED on main = 0, against a prior of 1 in 2.

## Measured versus assumed

MEASURED: e8c5e4e on zero refs, c8adbcc on main, same patch-id, same
author second; main red at 7a002e9 with the second line in an unlanded
tree; 211/107 rebased/plain by bead id; 2 of 8 in-session triages, 1
rebased; `fetch-depth: 0` in ci.yml; patch-id equal across an other-file
and an outside-context base move, changed by an inside-context move;
rebase skips an already-applied diff; nine patch-ids in 0.9 s; the wall
refuses `--amend`; `gc.pruneExpire` unset.
ASSUMED: git's default prune expiry on this box; "a red gate every
fortnight" from a base of two in-session triages; that a line and its
commit always reach main in one landing (a merge-back carrying the allow
edit alone is the stated exposure, not measured to have happened).
