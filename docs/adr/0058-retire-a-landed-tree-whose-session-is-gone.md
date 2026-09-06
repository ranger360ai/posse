# ADR 0058 — A session tree is retired unattended when its bead is closed, its work is measured on the base, and its session is proven gone; "a human can retire the tree" stops being the answer

*Status: accepted 2026-09-05 · owner: architect · source bead
ranger-base-wo980, from ranger-base-d8o6 · extends ADR 0011 §2 (prune must
prove death) to the session tree, ADR 0006 §3 (closed means it is on main),
ADR 0041 (the dirty close stays) · overturns two code comments that call the
sweep's restraint the point (landsweep.go header; `LandSessionTrees`) and
the NOTES.md line "`--land` … never removes a tree" · prerequisite:
ranger-base-v2rj7 (RemoveSessionTree asks the branch, not the tree's HEAD;
this record makes it a second caller, which is the loss v2rj7 names)*

## Context

The bead asked one question: what evidence does an unattended retire of a
session worktree need, and is there enough of it anywhere? Today the only
non-test caller of `RemoveSessionTree` is the kill's settle path
(herdrback.go), which runs once, at session end. Two surfaces say the rest
is deliberate — the sweep "does not remove a tree (that is `posse kill`'s)",
`--land` "MERGES and never removes … it reads git, so it cannot tell a
tree whose session ended from one a persona is working in right now" — and
every refusal ends "a human can retire the tree". Nobody is given that
instruction; nobody has followed it.

**MEASURED 2026-09-05 in ~/src/posse, 70 session trees standing, 8 with a
live herdr session (`posse list`).** Bead status read fresh from the store;
`ahead` is `rev-list --count main..branch`; `eq` is `git cherry`'s `-`:

| class | trees | what stands there |
|---|---|---|
| bead closed · ahead 0 · clean · no session | 36 | fully landed by fast-forward, or never committed; nothing to lose, nothing retired it |
| bead closed · ahead ≥1 · every commit patch-id equivalent · no session | 2 | the bead's shape (olwk, wr624): the sweep prints `≡ … nothing here is unlanded` for each on EVERY pass — 41 lines each in the current, rotated log alone. *Amended 2026-09-06: fact 2 as written never takes this row — see the amendment below* |
| bead closed · ahead ≥1 · some commit unmeasured (`-x` trailer, replay pair) or unpaired · no session | 13 | the listing says "compare before retiring" — correctly; stays a human's |
| bead closed · dirty · no session | 3 | ADR 0041's closed-dirty class; kept by design |
| bead open · no session | 6 | seats a relaunch will reuse; not this record's |
| no bead record | 4 | ADR 0006's rule: no record accounts for them; stays a human's |
| live session | 8 | the persona's |

So 38 of 70 trees (54%) are dead, landed, clean and closed, and the 36 are
not the bead's premise at all: they landed by FAST-FORWARD, and the tree
stood anyway. The equivalence case is a corner of a class, and the class is
**session gone, work landed**. The trees cost 712M on disk across 71 dirs
(8.5M each), 51 of `posse worktrees`' 89 lines say "nothing unlanded", and
the operator reads three live facts through that.

Why the trees stand: `RemoveSessionTree` runs only inside a kill, and a
kill runs only while herdr still lists the workspace. A herdr restart, a
pane closed in herdr, a `posse kill` that lost the launcher-lock race (the
log's one `KEPT: a launcher is running — not landed` line), or a crash all
take the workspace without the landing, and from then on the tree has no
path to removal: the sweep lands it and `continue`s in silence on the next
pass (`if n == 0 { continue }`, landsweep.go).

The "reads git" objection was true of `--land` when written and is not
true of the sweep. `landClosedTrees` reads the bead fresh from the store
of record, walks live herdr sessions, and holds the launcher lock. That is
more evidence than "reads git" admits, and the field's rule for how much
is enough exists (safe-reclamation.md; this shop met it in ADR 0011 §2):
**proof of death at reclaim time, plus a grace covering actors the scan
could not see**. Not "it looked dead in the listing".

## Decision

**D1 — the retire predicate: four facts, all read at retire time, every
unanswerable one fails closed.** A session tree may be retired unattended
when and only when:

1. *the bead is closed* — `bd show` fresh, the same read the sweep and the
   reap already make (ADR 0011: never cached);
2. *nothing would be lost* — `RemoveSessionTree(t, false)`'s own unforced
   refusal does not fire: clean tree, and either nothing ahead of the base
   or every commit measured by patch-id AND the base holding the branch's
   bytes for every path it touched (ranger-base-as19, x8jp). This is the
   existing destroy predicate, not a new one; ranger-base-v2rj7 must land
   first so it asks `workHead(t)` and a detached tree does not read as
   empty;
3. *the session is proven gone* — ADR 0011 §2's own question asked of the
   tree's session name: herdr does not list the workspace on this server,
   and where a meta still exists, the prune's identity-fenced evidence
   (`prunable`/`idEvidence`) says dead. Alive, foreign, unanswerable, or
   "this server cannot answer for it" all KEEP the tree. No new liveness
   rule is coined; the retire borrows the one the prune already proved;
4. *the grace has passed* — no write to the tree's own git dir (index,
   HEAD, logs under `.git/worktrees/<session>/`) and no commit on the
   branch inside `retire_tree_after:` (default 1h; `off`/`never` disables,
   spelled as the two reap graces are). Denominated in tree WRITES, not in
   time since close: it covers the one actor the board cannot show — a
   process in the tree whose workspace detection blinked, or an operator's
   shell — and a `git status` in the tree resets it, which is the
   fail-safe direction. The checkout directory's own mtime is NOT the
   reading (it does not move on a commit).

**D2 — the site is the landing sweep, under the launcher lock, and the
check is taken again inside it.** `landClosedTrees` visits every closed
tree already; the silent `n == 0` branch is where the 36 sit. After
landing (or finding nothing to land), it asks D1, takes the lock it
already holds lazily, RE-READS facts 2 and 3 under it (reclaim's rule:
evidence read before the lock is the same race one step over), and calls
`RemoveSessionTree(t, false)`. One line per retire (`⌫ <bead> <branch>
retired: <why it was safe>`); a tree kept for the grace prints nothing
(transient, and 36 at once is noise); a tree kept for any OTHER reason
prints on every pass it is true, kftx's rule. `--dry-run` reads and says
"would retire", removing nothing.

**D3 — the operator gets the same predicate on demand, and the listing
stops promising a human.** `posse worktrees --retire` runs D1 over every
tree under one blocking lock (the `--land` shape), prints one line per
tree, and never takes `--force`: force is `RemoveSessionTree`'s existing
override and stays the two-command hand recipe the refusals print. The
listing's "a human can retire the tree" becomes one of: "retirable —
the next pass takes it", "kept: <the D1 fact that failed>", or, for a
tree no record accounts for, the ADR 0006 sentence unchanged.

**D4 — what is never retired unattended, stated so the next reader does
not reopen it:** an open bead's tree (a seat); a tree with no bead record
(ADR 0006 — no record, no act; `--land --force` is the human's word);
a dirty tree (ADR 0041 — its handoff bead is the record, the tree is its
evidence); any commit whose landing is a decision or an inference rather
than a measurement (the `-x` trailer, the replay pair — the 13 above);
the shared checkout (no tree). Each of these prints the sentence it
prints today.

## Alternatives rejected

- **Leave the trees permanent by design and say so in NOTES.md** — the
  bead's "fine answer". Priced: 54% of standing trees are dead and landed,
  a `≡` line per equivalent tree per pass forever, 712M, and an
  operator surface where the live facts are the minority. The restraint
  protects a case (`--land` reading git alone) the sweep is not in.
- **Retire only on measured equivalence** — the bead's framing. Covers 2
  trees of 38. The class is dead-and-landed; equivalence is how ONE
  landing shape reaches it. *(Amended 2026-09-06: as fact 2 stood on
  2026-09-05 it covered 0 of 38 — the amendment below says why and what
  fact 2 now measures.)*
- **Retire inside `posse worktrees --land`.** Its comment is right about
  itself: a human command reading git cannot tell a dead tree from a live
  one and should not learn herdr to try. `--retire` beside it asks the
  full predicate; `--land` stays what it is.
- **Age alone** (git's own `gc.worktreePruneExpire` shape). Age is not
  death: the six open-bead seats are older than most of the 36. The
  standard rebuttal in safe-reclamation.md, verbatim.
- **Retire at kill time only** (status quo, made stricter). The settle
  path cannot see an equivalence that arrives after it ran, and cannot
  run at all for a workspace herdr lost.
- **Half-retire** — delete the branch and keep the tree, or the reverse.
  Same listing row, half the evidence gone.
- **A trash directory for retired trees.** A fifth store (ADR 0011) for
  content the predicate has just measured to be on main byte for byte.
  Nothing is kept because nothing is lost; the refs are the record.
- **The clever one: teach `RemoveSessionTree` to retire itself** on the
  merge's report (the `Equivalent`/`Merged` outcome) so every landing
  site retires. Rejected because the merge outcome is a fact about git,
  and fact 3 is a fact about herdr; a destroy predicate that reads only
  one store is the mistake this record is correcting.

## Consequences

- The `≡` line becomes a one-pass event; the listing shrinks to live
  trees, the unmeasured 13 (with their "compare" sentence), the four
  unrecorded, and ADR 0041's dirty three. *(Amended 2026-09-06: true of
  the `≡` line only once fact 2 carries the whitespace-exact twin below.
  That landed the same day — `baseHoldsBytes` in internal/posse/worktree.go,
  ranger-base-06y60 — so the sentence stands as written; before it, the
  `≡` tree was kept on every pass, correctly, for bytes it is not the last
  copy of.)*
- Two code comments and one NOTES.md sentence stop being true and are
  amended with the build (landsweep.go header, `LandSessionTrees`,
  NOTES.md's `--land` paragraph and lifecycle table row — the NOTES row is
  amended in this record's commit).
- A wrong retire is bounded: fact 2 means the bytes are on main, so the
  worst case is a directory removed under a live process whose next git
  command fails loudly. Fact 3 and fact 4 exist to make that a race of
  two independent misses, not one.
- The reap's `residueHolds` and the kill path are unchanged; a live
  session's tree is still the kill's to land and remove.
- Cost per pass: one `bd show` per closed tree (already paid), one
  herdr query per candidate that passed facts 1–2, a handful of stat(2)s.

## Verification (laurie's checklist)

1. A fixture tree — closed bead, ahead 0, clean, no herdr workspace, git
   dir untouched past the grace — is retired by one sweep pass; worktree
   registration and branch both gone.
2. The same tree with each fact broken in turn is KEPT, and the sentence
   names the fact: bead open; one uncommitted path; one commit unmeasured
   (a `-x` trailer fixture); workspace listed alive; meta the server
   cannot answer for; index touched inside the grace.
3. The v2rj7 shape — detached HEAD, one commit in the tree, branch at
   base — is KEPT with the commit still referenced, on every path that
   reaches `RemoveSessionTree`.
4. `--dry-run` over fixture 1 removes nothing and says "would retire".
5. `posse worktrees --retire` over fixtures 1–2 prints the same verdicts
   the sweep does; `--retire --force` is refused as unknown.
6. The listing over fixture 1 says "retirable", never "a human can
   retire the tree"; over a no-record tree it says the ADR 0006 sentence.

## MEASURED vs ASSUMED

MEASURED: the census table (2026-09-05, ~/src/posse, recipe in the bead's
comment); 8 live sessions; 712M / 71 dirs; 41 `≡` lines per equivalent
tree in the rotated log; one lock-race `KEPT` in that log; 23 reaps, 22
removals — the kill path does retire when it runs.

ASSUMED: the 1h default grace (a policy dial, like the two reap graces; no
tree-write cadence was measured); that an absent meta with no listed
workspace on this server is death — ADR 0011 §2's prune accepted the same
premise, and a second posse home sharing one worktree root would break it
for both; that the git-dir mtime reading is a faithful "last write" on the
box's filesystem (dinesh measures it on the way and says so in the code).

## Amendment 2026-09-06 — fact 2 could not take the tree it was filed from (ranger-base-lwd29, from iz8fx)

**The finding.** Fact 2's second half asks whether the base ever held the
branch's BLOB for each touched path (`contentNotOnBase`, x8jp). A tree
reaches the `≡` row because its landing was not a fast-forward — and every
landing that is not a fast-forward writes a NEW blob whenever the base had
moved the same file elsewhere in the meantime. For an append-heavy file
(CHANGELOG.md, INSTALL.md, NOTES.md) the base has always moved it, so the
branch's blob is never on the base, on any commit, and the row is kept on
every pass — the permanence this record was written to end. The corner
was not small; it was empty. The 2026-09-05 census counted it right and
fact 2 covered 0 of it.

**MEASURED 2026-09-06, ~/src/posse, git 2.50.1.** olwk is the whole row now
(wr624's tree and branch are both gone). Its one commit landed on main as
7ff3e4da by `cherry-pick -x` — not a hand re-landing, the bead's premise:
the trailer is on the landing and `git cherry` says `-`. The blob walk finds
5 of the 7 touched paths in `tip..main` and never CHANGELOG.md or
INSTALL.md; every line the branch added to those two is on main verbatim.
And `git patch-id --verbatim` — the whitespace-EXACT form the default
patch-id is not — gives the branch commit and 7ff3e4da the same id. Over a
scratch repo, the same instrument: a clean pick onto a base that moved the
file outside the hunk's context → `cherry -`, blob never on base, verbatim
EQUAL; a hand landing that re-indented (x8jp's shape), or added trailing
whitespace → `cherry -`, verbatim DIFFERS; the same line under a changed
neighbour, or a dropped hunk → `cherry +` (already unmeasured); a base that
moved inside the three context lines → the pick CONFLICTS (kept by
construction, h6g65); a squash of two commits → `cherry ++`. One call,
`git log -p --no-ext-diff --no-renames <tip>..<base> | git patch-id
--verbatim`, prints an id per non-merge commit of the range and finds the
twin under later edits: 970 ids over olwk's 987-commit range in 5.5s wall.
Live census of the 31 posse branches ahead of main: 30 are unpaired
(D4's human class) and olwk is the only tree any of this reaches.

**D1 fact 2, amended.** "The base holds the branch's bytes" is measured
two ways, and either licenses: for every touched path the branch's blob
was on the base in `tip..base` (today's walk, kept — it is the cheap common
answer and the refusal's per-path pointer), OR every commit ahead has a
whitespace-exact patch-id twin (`git patch-id --verbatim`) among the base's
commits in the same bound. The second closes exactly the hole x8jp named —
`git patch-id` normalises whitespace — with git's own flag for it, and
nothing else: context lines are hashed, so the twin is the same hunks with
the same neighbours, byte for byte; lines the branch never touched are not
its to lose (the rule `contentNotOnBase` already states for paths). A
commit the range form prints no id for (a merge) is unmeasured, and an
older git that rejects `--verbatim` (it is 2.39+, and it cannot be combined
with `--stable`) is an error, and both fail CLOSED — the tree is kept with
the sentence it prints today. Asked in one helper, in `heldByTip` and
`treeHolds` both, and `TestRetireGuardsSeeADetachedTreesWork` keeps their
answers one. The refusal that remains names the commit with no exact twin
and the paths whose bytes differ.

**Alternatives rejected.**
- *Say the corner is out of reach and stand on the 36.* Honest, and it
  leaves the exact shape the source bead was filed about — one `≡` line
  per pass forever — as the one shape the record does not fix, for the
  price of a flag git already has.
- *Replace the blob walk with the verbatim twin.* Buys ~70 lines and a
  single question. Costs a git floor that turns into a silent refusal
  class on an older box, the per-path pointer in the refusal, and a
  rewrite of x8jp's pins under a seat (iz8fx) that is live on the same
  file. Priced, not taken; a later simplification may fold them once the
  twin has run a pass.
- *Per-line containment* (every added line of the range diff present in
  some base blob). Ignores order and placement, and matches a blank line
  or a `}` against any file; the patch-id keeps the hunk whole.
- *A third arm that reads the `-x` trailer.* The trailer is a decision,
  not a measurement (as19); 7ff3e4da carries one AND a measured twin, and
  it is the twin that licenses.

**MEASURED:** everything in the paragraph above, dated. **ASSUMED:** that
the base's edits to an append-heavy file fall outside a hunk's context
often enough for the twin to take the row in the common case — one tree,
one landing, measured; the next `≡` tree measures the rule. Verification
adds to laurie's list: the eight scratch arms above as pins, with the
re-indent and trailing-whitespace arms as the wrong arms that must KEEP.

*Built 2026-09-06 in ranger-base-06y60: `baseHoldsBytes` (with
`verbatimUnpaired` and `patchIDsVerbatim`) in internal/posse/worktree.go,
asked by `heldByTip` and `treeHolds` both; the eight arms are pinned in
internal/posse/verbatimtwin_test.go, plus a ninth for the git floor — a
PATH shim rejecting `--verbatim` keeps the one tree that otherwise
retires. Mutation-checked on the way in: dropping `--verbatim` reds the
re-indent and trailing-whitespace arms and nothing else; removing the
twin arm reds the two that retire on it; reading a merge's missing id as
"nothing to compare" reds the merge arm; failing OPEN on the flag's
absence reds the git-floor arm; writing the twin lookup as a set rather
than a count reds the add/revert/add arm. That last rule is not this
record's — a base holding ONE copy of an id that two commits ahead share
holds one of them, so a twin is consumed by the commit it pairs.*
