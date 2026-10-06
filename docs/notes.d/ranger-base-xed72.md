## A second copy of the pin count, in the page nobody's bead edits (ranger-base-xed72)

2026-10-05. `TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` holds the
tree-wide pin and door counts in one place and says why in its own failure
text: *"the count of this class is said in one place on purpose, because two
places drift apart and the reader cannot tell which is stale."* AGENTS.md
carried a second copy anyway — "Forty-nine pins across the two, behind
fourteen doors" — and it was already four short of the pinned sentence
(fifty-three) at HEAD `6b330c13`, through at least two beads that doored a
pin, corrected the sentence in `treewidedoor_qa_test.go`, and had no reason
to read a crew-wide page. ranger-base-p9qve, in flight, makes it six apart.

### What was chosen, and why not the other rung

The bead offered two shapes: AGENTS.md stops naming a count, or arm 4 takes
AGENTS.md as a second head comment and holds the two numerals equal. The
second is strictly more machinery for a worse page: a seat reads standing
orders to learn that the doors exist and what they cost, and the SIZE of the
class changes nothing it does. A number nobody acts on is a number nobody
checks. So the page states no count — it names
`internal/treepins/treewidedoor_qa_test.go` instead — and arm 4 holds three
things about that bullet rather than one:

- the fenced door block equals `make tree-check`'s prerequisites, membership
  **and order** — the same promise ranger-base-r5546 gave the package
  README's table, now a third page out;
- the bullet carries **no** count of this class: `twdProseCount` matches a
  spelled numeral (or digits) before `pins`/`doors`, plus the `all <n>` form
  the door count was written in a second time ("`make tree-check` is all
  fourteen"). Its vocabulary is derived from `twdSpelled`, so the rule and
  the derivation cannot disagree about what a count looks like;
- the bullet still **names the file** that does carry the count, so a reader
  who loses the pointer is not left hunting.

Two prose edits fell out of the no-count rule, both honest: "5.1s at four
pins when ranger-base-ik44f wrote this line" became "5.1s over a small
fraction of today's class", and "Two doors are worth typing on their own"
became "Two of them" (the block is directly above it). The rule is scoped to
that one bullet, not the page: AGENTS.md counts its own things truthfully
elsewhere — "121 pattern kills from 63 seats", "599 of 608 commits" — and a
page-wide census would red on prose that is true.

### Verified

MEASURED 2026-10-05, this worktree, three mutations against the real page,
each restored from a scratchpad copy and the clean arm re-run green after:

| mutation | arm 4 |
|---|---|
| swap the `adr-check` and `corpus-check` rows in the block | FAIL — "lists the doors as … and `make tree-check`'s prerequisites are …" |
| add "Fifty-three pins across the two, behind fourteen doors." | FAIL — names both numerals it found |
| rewrite both `treewidedoor_qa_test.go` mentions to "that register" | FAIL — the pointer is gone |

`make tree-check` green over the result.

### Left standing

ADR 0067 carries the door count twice more — D1's "`check` … the name of
fourteen Makefile doors" and the dictionary row's "the fourteen Makefile
doors" — both live, both held by nobody. Filed rather than fixed here: it is
a shipped ADR, a different page, and the same class of defect. The arm count
in AGENTS.md ("three binaries", "all three arms") is structural — `make test`
names `test-arm1`, `-arm2`, `-arm3` — and the censuses are dated readings
with re-runnable scripts, not derived facts, so neither is this defect.
