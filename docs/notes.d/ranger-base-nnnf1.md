## An evidence pointer that resolved to nothing, and the door that reads one now (ranger-base-nnnf1)

A release was cut over a citation that pointed at no file. v0.5.1 (`32a90cde`,
2026-10-04) shipped a CHANGELOG section calling ADR 0062's bob claims MEASURED,
and `main`'s ADR 0062 named its evidence by path — a `docs/notes.d` fragment
that was still on the branch that wrote it. Nothing in the tree was red,
because the citation is prose and the fragment's absence is a missing FILE, and
no door read the two against each other. `ranger-base-aza46` landed that
fragment; `ranger-base-nnnf1` is the gap that let it ship.

### What was measured

A walk over every markdown file in the tree, matching
`docs/notes\.d/[A-Za-z0-9._-]+\.md` and stat-ing each hit. MEASURED 2026-10-04,
this box, over the working tree:

| rev | markdown files | citations | dangling |
|---|---|---|---|
| this worktree before the fix | 320 | 160 | 1 |
| after the fix | 320 | 159 | 0 |

The one dangling citation was `docs/notes.d/ranger-base-l1rjl.md:130`, which
leaned on a fragment `ranger-base-fm23s` never wrote in any branch
(`git log --all --diff-filter=A` over the path is empty) — so the rule
l1rjl's verification section rested on was unreadable. It is restated in
l1rjl now, from that bead's own close: on a base that moves under a finished
suite, re-run the arms the new commits can actually reach rather than the whole
suite again, and say which arms were not re-run.

The counts differ from the ones this bead's description carried (127 at
`ca92fb64`, 131 at `50c84e02`). `ranger-base-lyjbt`'s re-measure said 143 and
149 for the same revisions with a plain scanner, and this pin counts every hit
including repeats on one line, which is a third number again. The dangling
members and their count reproduce exactly in all three readings; only the
totals move, so the pin counts what its own matcher counts and logs it.

### The pin

`internal/posse/notescitation_qa_test.go` —
`TestQAEveryNotesFragmentCitationResolves`, doored as `make doc-check`
(`$(QA_DOC_PINS)`), beside the other prose pin whose subject is a path.
It walks from `qibRepoRoot`, skipping `.git`, `bin`, `dist` and
`node_modules`, and fails on a hit that is not a file — a directory of that
name is not a fragment. Two floors, well under the tree (100 files, 50
citations), because a pin over absence is satisfied by reading nothing.
`TestQANotesCitationCheckCanFail` is the control: it builds its own scratch
tree, so it computes no repo root and is not itself a member of the tree-wide
class.

### Why the two neighbours did not cover it

`internal/treepins/mergeblockedrunbook_qa_test.go` reads ONE function of
`internal/posse/dispatch.go`, narrow on purpose: a tree-wide version over GO
source would need an exemption register first, because internal/posse fixtures
name fragments no tree has. That reasoning stands and is untouched — markdown
has no fixture problem. `make notes-check` is the other near miss: it asserts
`docs/notes.d/README.md` lists every fragment that EXISTS, never that a cited
fragment does.

### Naming a fragment that is not there

There is no exemption register, and prose does not need one: name the BEAD, not
the path. Both sentences in this tree that have to say `ranger-base-fm23s` wrote
no fragment say it that way — including the one above. A register would be the
place to park the next real escape.
