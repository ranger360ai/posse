# Two instruments that said yes without measuring: arm 6's coin flip and the census's empty denominator (ranger-base-7c2py)

Both findings of ranger-base-8h6u2's verify, and they are the same defect in
two languages: a check whose *subject* was sound and whose *apparatus* could
answer without reading anything. Neither fix changes a production path.

Dated 2026-11-21.

## Finding 1 — `scripts/gotest.sh` arm 6 killed its mutant 2 of 5 runs

Arm 6 is the arm that keeps `plant_over`'s bounded retry alive: a write over a
live executable is refused (ETXTBSY), and the retry waits it out. It is
advertised in `internal/treepins/gotestreuse_qa_test.go` as "the deterministic
half of the same refusal" and as "the arm that can fail", because the bug it
fixes showed once at a load average of 14 and a re-run in isolation was green.

It was a coin flip. The arm removed the holder's marker — which is what lets
the holder process exit — and *then* called `plant_over`, so whether the
plant's first attempt was refused at all depended on whether the holder had
already exited and the kernel released its text image. Both outcomes happen
here.

MEASURED 2026-11-21, this box (linux, 16 cores, bash 5), over copies of the
script; the tracked file was never the subject:

| arm (loadavg 13.6-16.0) | reading |
|---|---|
| pre-fix control, `PLANT_TRIES` at success, 5 runs | 9, 1, 9, 1, 9 |
| pre-fix, one-attempt mutant, 5 runs | SURVIVED, KILLED, SURVIVED, SURVIVED, KILLED (2/5) |

A `1` is a run on which the retry was never exercised, and on those runs the
mutant that deletes the retry passes the arm written to catch it. A green
suite licensed the removal of the retry on a majority of runs.

THE FIX IS THE ORDER. The plant goes first, in the background, while the
holder still holds its text image; the marker comes off only once the plant
has said (on its own stderr, into a file) that it is waiting. The first
attempt is then refused by construction, and the arm asserts `PLANT_TRIES >= 2`
rather than merely asserting that the plant eventually landed — a plant that
succeeded on its first attempt now *reds*, because it measured nothing.

MEASURED after the fix, three batteries of five runs each — two at loadavg
5.5-10.6 and the third at 20.5-21.8, above the load the original flake was
first seen at:

| arm | reading |
|---|---|
| fixed control | 15/15 green, `PLANT_TRIES` 9 or 10 every run (one 9, at the high load) |
| fixed, one-attempt mutant | 15/15 KILLED, "after 1 tries" every run |

The try count stopped moving between 1 and 9, which is the whole claim: the
arm no longer has a run on which it measures nothing.

Three things the shape costs, each of which bit a draft:

- **`plant_over`'s answer comes back through files.** It sets `PLANT_ERR` and
  `PLANT_TRIES` by dynamic scope for its caller, and a background subshell
  cannot set a variable in the shell that forked it. The try count *is* the
  measurement, so it is written to a file and read back.
- **The wait for the plant's notice watches for its exit too.** The
  one-attempt mutant returns before printing any notice, so a wait on the
  notice alone would sit for the whole ten-second budget on the one run that
  matters. Watching the pid is not enough on its own either: a child that has
  exited but not been waited on still answers `kill -0`, so the signal is the
  plant's own last write.
- **`set -euo pipefail`.** The try count is validated numeric before `-lt`
  reads it (a `[ x -lt 2 ]` is a bash error, and under `set -e` that takes the
  whole self-test down instead of reddening one arm), and every status a
  diagnostic reads is guarded.

Both arm labels are unchanged, so `gotestreuse_qa_test.go` arm 2 — which
requires each label by text — needed no edit.

## Finding 2 — the census all-cleared on a path, with an empty denominator

`scripts/store-git-write-census.py` is the whole reason a store repo's `.git`
grant went to zero rather than being narrowed, and its close says "re-run it;
the corpus grows". `--store` wants the store repo's directory NAME: it is
matched as a path COMPONENT, because transcripts spell the repo three
different ways. Handed a path it matched no component at all, so every git
verb fell out of the classification — reads and writes alike — and the script
printed

    ANSWER: no seat session has ever typed a git write aimed at <path>.

at exit 0, over a corpus of nothing. Its own header rule 4 already states the
rule its code did not enforce: "READ VERBS ARE THE DENOMINATOR. The answer 'no
seat writes it' is worth nothing without '167 seat commands name it and every
git verb among them is a read'."

MEASURED 2026-11-21, this box:

| `--store` spelling | reading |
|---|---|
| the full path, at 645 transcripts | 111 seat commands, **no git verbs at all**, ANSWER: all-clear, exit 0 |
| the directory name, at 645 | 200 seat commands, 53 read verbs, WRITE verbs NONE, exit 0 |
| the directory name, re-read at 689 | 220 seat commands, the same eight read verbs, WRITE verbs NONE, exit 0 |

The first row is the hole: an answer printed at exit 0 over nothing. The name
rows are the reading, and the second of them reproduces the close this census
validates — the corpus grew, the verb set did not, and the write set is still
empty. The instrument was what was not sound. (The path row is the pre-fix
reading, taken on the verify bead; the fix refuses that spelling, so it is not
re-measurable here.)

TWO REFUSALS, because the script states two rules and enforced neither:

1. a `--store` carrying a path separator is refused at exit 2, naming the
   spelling that works;
2. an empty read denominator refuses the all-clear and exits 2 with
   `ANSWER: NOT PROVEN`, rather than printing an all-clear nobody measured.

`2` is new and means "the census did not answer" — distinct from `0`
(all-clear) and `1` (seat writes found), and documented in the header's own
EXIT STATUS block. The two are deliberately redundant: with the argument check
removed, a path still exits 2, because rule 4 catches it at the answer.

The denominator is the READ VERB set and not the command count. A `--store`
that matches no path component still matches as a plain substring of the
command line, so the command count looked healthy — 111 of them — while both
verb sets were empty. That is how the finding presented, and counting
commands would not have caught it.

AND THE SELF-TEST GAINED THE HALF IT WAS MISSING. All nine of its arms were
green over both holes, because every one of them feeds the classifier
directly: the one thing the self-test never ran was the script. Five
`instrument_arms` now run `report()` and `main()` whole over a planted
corpus — a corpus of seat reads answers all-clear at 0, one with a write exits
1, one that names the store with no git verb aimed at it refuses at 2, a path
is refused before it reads anything, and the same corpus spelled as a NAME
answers all-clear, which is the blindness control that makes the two refusals
mean something.

MEASURED 2026-11-21 over copies: with either refusal removed the instrument
arms go 4/5 and the self-test exits 1, naming the removed one. The classifier
arms stay 9/9 in both mutants, which is the point.
