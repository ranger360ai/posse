# A fix into a helper leaves the call unwitnessed (ranger-base-nqw8v)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, go1.27.3, bash 5.3.20, 16 cores,
from this bead's worktree at `5161bc1f`. The verify lane over four closes —
ranger-base-a688i, ranger-base-es4ai, ranger-base-nt5xd, ranger-base-vxz74.
All four verified on substance; three carried a finding. Two of the three are
pins and landed here under this bead, which is what this fragment is about.
The third is in another repository and is on its own bead.

## The class: a fix that moves behaviour into a helper leaves ONE line nobody measures

Two of these four closes have the same shape, and it is a shape worth naming
because it is the shape of a GOOD fix. The behaviour that was wrong gets moved
into a function the pins can call, and that function gets a cheap, fast,
hermetic pin. Every mutant inside the function reds. What nothing reads is the
CALL — whether it is made at all, whether it is handed the right thing, and
whether it is made on the path it was deliberately kept off.

The tell is a pin whose NAME says the caller while its body only exercises the
callee.

MEASURED, one mutant per close, each planted alone with the tree asserted
clean between arms:

- ranger-base-a688i moved a wall-pin member's fatal onto one line by folding
  the watcher's words through `wwOneLine`, and pinned the fold
  (`TestQAWallWatchFixtureFoldsWhatTheWatcherSaidOntoOneLine`). Reverting the
  exit-code fatal to its PRE-FIX text — `t.Fatalf("the seat ended before it
  ran (exit %d):", c)`, byte-for-byte the line the P1 was filed on — left
  arm 1's whole wall-watch family GREEN, that pin included. Three mutants of
  `wwOneLine` red it; the call site is not its subject.
- ranger-base-nt5xd added ADR 0092 D4 rule 5's note and printed it on the
  STAND path only, which is a documented decision with two older siblings
  (D7's hook-wall line, amendment 3's render note): on a run that is about to
  be undone, all three describe a binary that will not be there. Adding any
  one of the three to the ROLLBACK path left the whole upgrade family GREEN.
  Presence was pinned; placement was not, for all three.

`internal/posse/callsite_qa_test.go` is the two pins, in one file with the
class stated once, both source pins because neither call has an observable a
behavioural test can reach: `launchOn`'s fatals fire only when a live bwrap
seat fails to start, which is the case a688i could not reproduce at
`-count=100`, and the three report halves print onto a verdict path whose
rollback arm the unit fixtures reach with no gained row to report.

### Grading a source pin needs the arms that must stay GREEN

Each pin is keyed on a mechanism rather than a sentence — "every `t.Fatalf`
inside `launchOn`'s wait loop", because that loop is where the watcher
goroutine is already running and therefore where there are words to spend the
line on; "the one top-level `if … { … return nil }`", because that is the arm
where the install survived. A pin keyed that way can be over-broad, and the
only way to say it is not is an arm it must NOT red on. Both are in the
battery: a wordless `t.Fatalf` added ABOVE the loop stays green (setup has no
watcher and nothing said), and a line added to the stand path stays green.
Nine arms, seven red at the named pin, two green as required.

## What the budget a one-line fatal is spending actually buys

ranger-base-a688i's premise is `scripts/posse-wall-check`'s `evidence()`: a
failed member is reported by its first `file.go:NN:` line, 300 characters, and
nothing after it (ADR 0097 D3). The close reproduced two of `runWallWatch`'s
four exit-1 causes. The fourth — a wall missing at launch,
`respond(missing, true)` — is the one that writes a multi-line verdict AND
files a bead, so it is the one with the most to say and the only one that
could overrun the budget.

MEASURED 2026-11-21, by appending one unmounted wall to the fixture's watch
list so the baseline is short at launch: the folded fatal is **370
characters**, so `evidence()` does truncate it. What survives inside the 300
is `KILLED seat <seat>: the wall over <path> came off` — the cause, at the
front, which is where the budget is spent. The timestamp and the filed bead id
are what the truncation takes. So the fold holds for the cause the close did
not reproduce, and the residual is that a long enough bwrap preamble on the
same buffer would push a later refusal past 300: the order of the buffer is
what makes the cause legible, not its length.

## A measurement whose parameter is not the one the thing runs with

Not a defect in any code, and recorded here because the shape recurs: a
MEASURED table can be right at a parameter nobody passes. ranger-base-es4ai's
numbers for two weekly runs hold at `--warn-within-days 21` and not at the
default 14 that the scheduled job actually uses, where the second of the two
runs has nothing to report at all and the channel is silent. Reproducing a
measurement means reproducing it at the value the shipped caller supplies —
which for a workflow is the branch taken when its dispatch input is empty.

## Not closed here

- ranger-base-92676

