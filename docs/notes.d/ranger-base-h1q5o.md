# ranger-base-h1q5o — a merge-back whose conflict was the same pinned COUNT, twice over

Landed 2026-10-31, git 2.50.1 / macOS 25.6.0, go 1.26.10.

`ranger-base-na4rm` closed with one commit on
`posse/gwart-posse-ranger-base-na4rm` (`d59bc965`) and `main` moved on
underneath it by four commits. `docs/notes.d/ranger-base-xea2y.md` is the
procedure and it held again without amendment;
`docs/notes.d/ranger-base-v3tt4.md` is the previous one of this exact shape —
a conflict in the pinned class count — and its lesson held too, which is the
point of this entry. **The count conflict in `treewidedoor_qa_test.go`'s head
comment is the standing shape of a merge-back in this repo, not an accident:**
this is the fourth of them — `ranger-base-yrag8` (2026-10-04),
`ranger-base-3e68k` and `ranger-base-v3tt4` (both 2026-10-31) are the others,
and 3e68k's entry predicted it in as many words ("the shape every future bead
that doors a pin will hit"). Every bead that doors a tree-wide pin edits one
sentence, so two of them in the same hour collide there by construction.

## The conflict

`git merge-tree --write-tree --merge-base=<base> main <work>` named exactly
one file:

```
CONFLICT (content): Merge conflict in internal/treepins/treewidedoor_qa_test.go
Auto-merging Makefile
Auto-merging docs/notes.d/README.md
Auto-merging internal/posse/runtimehome_qa_test.go
Auto-merging internal/posse/shelfpidlint_qa_test.go
```

Narrower than v3tt4's, which conflicted in the Makefile as well. Both sides
doored pins, but into DISJOINT door variables — `main` (`66a8e9d4`,
ranger-base-8yv7z) added eight names to `$(QA_SEED_PINS)` and renamed one in
`$(QA_CORPUS_PINS)`; the work commit added three new door variables and three
new `tree-check` prerequisites — so git resolved the Makefile itself. What it
could not resolve is the one sentence both sides rewrote, and the conflicted
region was exactly that sentence plus the narrative block above it.

## What a union cannot resolve, again

The head comment carries exactly one live claim of the class's size —
`<spelled> pins and <spelled> doors`, derived from the Makefile by
`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` (arm 4), which fails
on two matches as hard as on a wrong one. `main` said ninety-four pins and
fourteen doors; the branch said one hundred and one and seventeen; the merged
tree is **one hundred and nine and seventeen** (86 + 8 + 15, and 14 + 3).
Neither side's sentence is right after the merge, so taking either wholesale
reds arm 4 — and the arm is what SAYS so, which is the whole reason this
class of merge-back is cheap to land correctly.

Two traps the pin cannot catch, both of them prose:

- **A sentence that counts the class a second time reds the pin as hard as a
  wrong count.** Writing "the class has grown by twenty-six pins and three
  doors since the 81-89s below" into the resolution matches
  `twdCountClaim`'s `<num> pins and <num> doors` skeleton, so arm 4 fatals on
  TWO claims. The fix is to say it in the other order — "three doors and
  twenty-six pins have joined the class" — which is the same fact and not a
  claim about the current size. Worth knowing before you type it: the trap is
  in the resolution, not in the sides.
- **Superlatives in the entries BELOW go stale when a faster or quieter
  reading is prepended.** `ranger-base-nt0pi`'s entry said "the quietest box
  this command has been read on" and this bead's three runs were quieter
  (3.25-4.56 against 5.62), so that clause now reads "until the entry above".
  `ranger-base-na4rm`'s said "the largest class this command has ever run",
  which its own merge-back made false; it now says what it still measures.

## The reading the merge owes

Each side's timing was taken over a class the merged tree does not have, so
neither is a reading OF what landed. The sentence belongs to the merge: three
warm runs here, at the merged count, attributed to this bead. Both sides'
readings are kept for what they still measure — what their own pins and doors
cost — with the class each was read at named.

`make tree-check` at one hundred and nine pins and seventeen doors: **56s,
55s and 55s** over three warm runs, at a one-minute load average of 4.56,
3.70 and 3.25 before the three, with no suite slot held by anyone. 72s on the
run before them, with the two tagged `pkgcensus` binaries cold.

A **1s spread** — the second-tightest reading this command has produced
(`ranger-base-v3tt4`'s 0.7s stands), read on the quietest box it has been
seen on and over the largest class it has run. The class has taken on
twenty-six more pins and three more doors since `ranger-base-3e68k` read
81-89s, and the command is twenty-five seconds FASTER, which is the fifth
independent reading of the same conclusion: the wall does not track the size
of the class.

**And the LEVEL does show, which four readings in a row had left open.** The
standing advice was ninety seconds "when the box is steady, whatever level it
is steady at", read off 3e68k's steady 25-28. Five readings now exist at a
known load, and they separate by level and not only by movement: every one
taken under a load of 16 is 55-62s (this bead at 3-4, na4rm at 6-15, v3tt4 at
10-31) and the steady 25-28 one is 81-89s. So AGENTS.md now prices the
command at a minute on a quiet box and ninety seconds on a busy but steady
one, and still two and a half minutes when the load is MOVING, which is the
only part of the old advice any reading has ever contradicted.

## Verified

`make fmt-check`; arm 4 of the count pin after each edit to the sentence
(`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles`, which is also what
caught the second-claim trap above); `make notes-check` after this fragment;
`make tree-check` four times (one of them the cold run above); the xea2y
both-directions reading over the conflicted file — the merged
`treewidedoor_qa_test.go` against the work commit's and against main's, so
nothing of either side was dropped (every line of main's that the merge drops
is one `ranger-base-na4rm` deliberately rewrote: the `twdDoors` table, which
gains an `arm` field and three rows, `twdSpelled`'s ceiling of 99,
`twdCountClaim`'s one-word numeral, `twdProseCount`'s 101 alternatives and
the renamed pin); and `make test` whole, green at MAKE_EXIT=0 over the three
arms — 401.8s/487.7s arm 1 (internal/posse, internal/treepins), 435.8s arm 2,
371.6s arm 3, silent-revert audit 2195 commits 0 untriaged, and
`go run ./cmd/checkorphans` clean after it.
