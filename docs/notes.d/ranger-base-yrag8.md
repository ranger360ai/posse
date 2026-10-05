# Replaying a merge-back whose conflict was a COUNT, and the red neither side could see (ranger-base-yrag8)

ranger-base-hrf47 (71ce45e9) and ranger-base-nnnf1 (f354346c) were written six
seconds apart and each added one row to the same enumeration in
`internal/treepins/treewidedoor_qa_test.go`. nnnf1 landed; hrf47's merge was
refused. The textual conflict was two hunks of prose. The resolution was not
textual, and neither was the thing the replay found.

## The conflict was prose; the resolution was arithmetic

`git merge-tree --write-tree` auto-merged the enumeration itself — nnnf1's
`doc-check` row and hrf47's `scripts-check` row are twenty rows apart, so both
survived with no marker. What conflicted was the count sentence underneath, and
there the merged text is in NEITHER side:

| | pins the sentence claims |
|---|---|
| before either bead | fifty-one |
| main (nnnf1 landed) | fifty-two |
| hrf47's branch | fifty-two |
| the merge | **fifty-three** |

Both sides incremented from fifty-one and both wrote "fifty-two", so a textual
merge of two identical words is clean and wrong. `TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles`
(arm 4) holds that numeral to the Makefile's own door variables, which is what
caught it — `make register-check` is the 23.7s door, and it is worth typing
BEFORE the seconds are re-measured, because it is the arm that says what the
class now is. The same applies to the seconds: both sides had re-measured
`make tree-check` as that comment's rule requires, and both readings were of a
class with one row, not two.

**MEASURED 2026-10-04, this box.** `make tree-check` over the merged class at
fifty-three: **51s, 46s, 48s** warm, both suite slots free throughout
(`scripts/suite-lock.sh --status`), one-minute load average 4.4 before the
first and 9.1, 7.5, 6.6 after each. nnnf1 read 53-75s and hrf47 read
55.34/99.78/69.69s, each at fifty-two and each beside somebody — so the
largest class yet reads at the quiet END of every range before it. That is the
third independent confirmation of the same conclusion the file already carried:
the spread is the box, not the class and not the cache.

## The red neither side's suite could see

nnnf1's new pin — `TestQAEveryNotesFragmentCitationResolves`, the whole point
of that bead — requires every `docs/notes.d/` citation in tracked markdown to
resolve to a fragment the tree holds. hrf47's fragment cited a
`docs/notes.d/` fragment for ranger-base-rulbl.

There has never been one, in any branch: that bead wrote no fragment, and its
prose is the AGENTS.md bullet "A `-run` filter cannot reach a tree-wide pin".
Every one of the twenty-two other references to it in the tree names the BEAD.
hrf47's two were the only ones naming a path, and nnnf1's ruling is already
written for this case — *to name a fragment that is not there, name the bead* —
so the fragment and the `scripts-check` comment now do.

**This fragment had to obey the same rule to exist.** Its first draft quoted
the dangling path twice, in prose explaining that the path was dangling, and
the pin redded on it at lines 46 and 65: a citation is a citation, and there is
no exemption register by design (nnnf1). So a write-up about a bad pointer
cannot reproduce the pointer. That is the rule working, not a gap in it — but
it means the failure output below is quoted with the path elided.

**Neither bead's suite was capable of reporting this.** hrf47's full green run
predates the pin that catches it. nnnf1's full green run predates the file that
trips it. The two are green separately and red together, and the only place
that fact exists is the merge:

    make register-check
    --- FAIL: TestQATheTreeWideDoorsReportRealDrift
        `make doc-check` failed on a clean copy of this tree:
        docs/notes.d/ranger-base-hrf47.md:44 cites <the rulbl fragment>,
        which is not a file in this tree

So a merge-back is not finished when the markers are gone and it is not
finished when the replayed branch's own verification is re-run. It is finished
when the MERGED tree's doors are green — here `make register-check` first,
because its drift arm runs the other thirteen doors over a clean copy, then
`make tree-check` and the suite. A replay that only restores the branch's text
ships a tree neither bead ever had.

## Verified

- `make fmt-check`, `make notes-check` — green.
- `make register-check` 23.673s — arm 4 confirms fifty-three and fourteen, the
  two-way check accepts the third `twdDoorHolders` holder, and the drift arm
  (which runs every other door over a clean copy) is green, which is the arm
  that found the citation red.
- `make tree-check` all fourteen doors, three warm runs, exit 0 each: 51s, 46s,
  48s.
- `make test` exit 0, no FAIL line, all three arms: arm 1 internal/posse
  769.729s, internal/treepins 813.660s (the unfiltered run that was the only
  thing catching hrf47's original red), cmd/posse 580.142s; arm 2 325.569s;
  arm 3 347.831s; silent-revert audit 2008 commits, 0 untriaged.
- `go run ./cmd/checkorphans` clean, in a later Bash call than the backgrounded
  suite, which carried `POSSE_KEEP=ranger-base-yrag8` while it ran.
- The twin commit keeps 71ce45e9's author, AUTHOR DATE (20:41:57) and subject,
  so `git log --grep ranger-base-hrf47` finds it and no later launcher pass
  re-files the block.
