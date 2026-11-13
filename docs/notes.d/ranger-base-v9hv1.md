# A merge-back's worst conflict is the one git does not report (ranger-base-v9hv1)

2026-11-12. `posse/gilfoyle-posse-ranger-base-u1z89` would not land: main had
moved two commits and the replay conflicted. The replay itself was routine and
the two worked examples already cover it — docs/notes.d/ranger-base-xea2y.md
for the shape, docs/notes.d/ranger-base-4r8fw.md for the per-commit loop. This
fragment is about the part neither covers, because it is not a conflict.

## A DERIVED COUNT THAT BOTH SIDES BUMP MERGES CLEAN AND LANDS WRONG

`internal/treepins/treewidedoor_qa_test.go`'s head comment carries the
sentence a seat prices `make tree-check` from: "one hundred and forty-N pins
and seventeen doors". The base said forty-one. ranger-base-u1z89 added a
scripts-check pin and wrote forty-two. ranger-base-qbg7c added a corpus-check
pin and wrote forty-two. Both were right about their own tree.

Both sides therefore wrote the SAME WORD on the SAME LINE, so git took it with
no conflict at all, while the two pins beside it merged additively into two
different Makefile variables. The landed tree had 143 pins and a sentence
saying 142 — and nothing about the merge pointed at it. The three files git
DID report (a generated index, two prose paragraphs where each side struck a
different item off one list) were the easy half; this was the one that would
have shipped.

`make register-check` is what said so, in the words the pin exists to say:

    the head comment says "one hundred and forty-two" pins and "seventeen"
    doors; the Makefile has one hundred and forty-three (143) and seventeen (17).

The pin derives the number from the Makefile's door variables, so it could not
agree with the merge's arithmetic. Doors stayed at seventeen because both pins
joined existing doors, which is why only one of the two numbers moved.

**The general shape**: when a merge-back's two sides both edit a value DERIVED
from something they each also added to, textual agreement is not agreement.
git's merge is over text and the invariant is over the tree, so the merge is
silent exactly where both sides did the same arithmetic on different inputs. A
count, a total, a "N of M" census figure and a roster length are all this
class. A conflict is a question git asks you; this is one it does not.

**So: run the DERIVING door after a replay, not just the suite.** On this tree
that is `make tree-check` (`register-check` is one of its seventeen), and the
reason to type it is that no `-run` filter over the files you touched will
reach a pin whose subject is the Makefile. It is the same gap AGENTS.md
records for `make fmt-check`, one class over: a tree-wide pin is not reachable
from the names of the tests you think you changed.

## WHAT ELSE THE REPLAY HAD TO DECIDE

- `docs/notes.d/README.md` is GENERATED. It was regenerated with `python3
  scripts/notes-index.py` and checked with `--check`, never hand-merged — both
  rows land in id order on their own. A hand-merge here would have been two
  rows in whatever order the markers happened to leave them.
- `internal/treepins/mktemproot_qa_test.go`'s "WHAT IT DOES NOT REACH" list:
  each side struck a DIFFERENT item off it. u1z89 struck the `mktemp` with no
  `-d`, which is its own new second arm's subject; qbg7c struck Go's
  os.MkdirTemp and Python's tempfile, which are temproot_qa_test.go's. Both
  strikes hold, so the merged list is the one item both sides left on it. The
  reading to avoid is "take one side": either choice re-opens a hole the other
  side had just closed, and both sides' prose looks complete on its own.
- `treewidedoor_qa_test.go`'s register gained TWO rows, ordered as their doors
  are: u1z89's scripts-check row completes the paragraph above it, then
  qbg7c's corpus-check row.
- u1z89's `THREE WARM READINGS AT ONE HUNDRED AND FORTY-TWO` section is kept
  and relabelled as the last reading taken WITHOUT qbg7c's pin, rather than
  edited to claim a class it never ran at. A dated reading is a measurement;
  re-pointing it at a different tree would make it a guess.

## MEASURED

`make tree-check` at the class of 143, warm, this box (linux), 2026-11-12:
**44s, 49s then 41s** in that order, at a one-minute load average of 4.35,
8.07 and 11.46 over a five-minute 3.42, 4.81 and 6.08, with NEITHER suite slot
held by anyone for any of the three. An 8s band whose fastest run is the one at
the HIGHEST load and whose slowest is the one on the quietest box — the fifth
reading in that file's chain to say the variable is not the average. The two
pins cost 0.45s between them (0.15s for the whole scripts-check door, 0.30s for
qbg7c's census), two orders of magnitude under the band. The readings are in
treewidedoor_qa_test.go's own chain, which is where that file's head comment
says they go.

Also verified by running, after the resolution: `register-check`,
`scripts-check`, `corpus-check`, `notes-check`, `fmt-check`;
`make verify-reap-test-tmp` (24 arms) and `make verify-bd-argv-gate` (19600
spellings, 0 waved through); `scripts/test-times.sh --self-test` and
`scripts/gotest.sh --self-test`; `go test -tags posse_arm3 -run TestAutostart`;
and a full green `make test` (three arms, 0 reds, silent-revert audit 2600
commits / 0 untriaged). The five shell scripts and the notes fragment came
across byte-identical to the original commit, so only the four files git
reported — and the one it did not — were decided by hand.

One note from that suite run, not this bead's work and not a red: test-times
reported /tmp at 4152 MB free, under its own 5120 MB floor, on a box whose
`usrquota` mount means `df` cannot see the share that runs out first
(ranger-base-akk9f). It is the quota this bead's family exists to protect.

Not closed here: ranger-base-2tnbv.
