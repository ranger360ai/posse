# ranger-base-v3tt4 — a merge-back whose conflict was a pinned COUNT

Landed 2026-10-31, git 2.50.1 / macOS 25.6.0, go 1.26.10.

`ranger-base-8wxbn` closed with three commits on
`posse/gwart-posse-ranger-base-8wxbn` (`3ae6e885`) and `main` moved on
underneath it. `docs/notes.d/ranger-base-xea2y.md` is the procedure and it
held again without amendment; `docs/notes.d/ranger-base-2a7qa.md` is the
previous one, whose lesson was a stale arm count that nothing pinned. This
one is that lesson's other half: the count here IS pinned, so the pin caught
it — and what the pin cannot supply is the thing the merge then owes, which
is a fresh measurement.

## The conflict

`git merge-tree --write-tree --merge-base=<base> main <work>` named two
files, both of them the same question in two spellings:

```
CONFLICT (content): Merge conflict in Makefile
CONFLICT (content): Merge conflict in internal/treepins/treewidedoor_qa_test.go
```

Both sides doored tree-wide pins, which is one edit to the door variables and
one to `treewidedoor_qa_test.go`'s head comment:

- `main` (`9fd5c854`, ranger-base-0ckgm) added TWO —
  `TestQANoTestShellsRealBdWithoutStoppingItsDaemon` to `$(QA_CORPUS_PINS)`
  and `TestQANoTreeWalkingHelperIsHandedATreePathThroughAParameter` to
  `$(QA_REGISTER_PINS)` — with a narrative block each.
- the work commits (ranger-base-8wxbn) added TWELVE across four door
  variables, by widening the derivation rather than pinning a spelling.

So the Makefile resolution is a plain UNION per variable, taking each
main-only name back to the position its own side gave it, and the head
comment's resolution is both narrative blocks, in landing order.

## The part a union cannot resolve

The head comment carries exactly one live claim of the class's size —
`<spelled> pins and <spelled> doors`, derived from the Makefile by
`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` (arm 4), which fails
on two matches as hard as on a wrong one. Neither side's sentence is right
after the merge: `main` said sixty-four, the branch said seventy-four, and
the merged tree is **seventy-six** (62 + 2 + 12). The arm is what says so,
and taking either side's text wholesale reds it — which is the merge-back
shape 2a7qa went looking for and did not have.

The timing in that sentence is prose and nothing holds it, which is the
trap. Each side's reading was taken over a class the merged tree does not
have, so neither is a reading OF what landed, and keeping one would attach a
real measurement to a size it was never taken at. The sentence belongs to
the merge: three warm runs here, at the merged count, attributed to this
bead. The branch's reading is kept for the one thing it still measures —
what the twelve new pins COST, since they are the same twelve.

The same sentence is written a second time in AGENTS.md, where the pin
forbids a pin count but says nothing about the timing, so that copy was
updated by hand from the same three runs.

## Numbers

`make tree-check` at seventy-six pins and fourteen doors: 57.6s, 57.0s and
57.7s over three warm runs, at a one-minute load average of 31.41, 13.93 and
10.10 before the three, with one suite slot held by a sibling seat
throughout.

A 0.7s spread across a threefold move in the load average — the tightest
reading this command has produced, and the fourth in a row to say the spread
is not ordered by that number, this time from the side where the box moved
and the wall did not.

## Verified

`make fmt-check`, the register and notes arms under one `-run` filter
(`QAEveryTreeWidePinHasADoor|QATheTreeWideDoorsReportRealDrift|QAMakeTestOpensTheTreeWideDoors|QAEveryTreepinsDoorFilterNamesItsPins|QATheHeadCommentsPinAndDoorCountsAreTheMakefiles|QATheTreepinsEnumerationRuleMatchesWhatItClaims|QAOneRepoRootHelperInTheTestPackage|QANoTreeWalkingHelperIsHandedATreePathThroughAParameter|TreePinsRunFromModuleRoot|NotesFragmentIndexIsCurrent`,
ok 50.8s), `make tree-check` three times, the xea2y both-directions reading
over the conflicted file, and `make test` whole.
