# ranger-base-m8kfv — a merge-back whose whole conflict was one spelled number, replayed three times

Landed 2026-11-01, git 2.50.1 / macOS 25.6.0, go 1.26.10.

`ranger-base-372iq` closed with one commit on
`posse/jian-yang-posse-ranger-base-372iq` (`5d7b9245`, parent `2225b0b0`) and
`main` moved on under it by exactly one commit — `524e3e3b`,
`ranger-base-00fyg`, which built ADR 0078. The launcher's rebase conflicted
and aborted, so it filed this block.

`docs/notes.d/ranger-base-xea2y.md` is the procedure and it held again
without amendment — the sixth time. `docs/notes.d/ranger-base-v3tt4.md` is
the rule this one is an instance of, and it held too. The only thing worth
adding is how SMALL the conflict can be and still be unresolvable by taking a
side.

## The conflict

```
Auto-merging Makefile
Auto-merging internal/treepins/treewidedoor_qa_test.go
CONFLICT (content): Merge conflict in internal/treepins/treewidedoor_qa_test.go
```

One file, one hunk, and the hunk is two lines of prose. Four of the work
commit's six paths came out of `merge-tree --write-tree` **byte-identical to
the work commit** — `internal/posse/{backup,cage,herdr}.go` and
`internal/treepins/protocolchildpath_qa_test.go` — because `00fyg` touched
none of them. So the xea2y both-directions reading (diff the merged file
against the work commit, diff that against the other side's commit) had
nothing to check on the code at all: `git diff --name-only 2225b0b0..main`
and the work commit's path list intersect in exactly `Makefile` and
`treewidedoor_qa_test.go`.

The `Makefile` auto-merged correctly and that is checkable in one command,
because both sides edit the same door variables: `git diff main -- Makefile`
on the merged tree showed ONE changed line, `$(QA_CORPUS_PINS)` with
`TestQAEveryProbeForAProtocolChildsNameGoesThroughProtocolAvailable`
appended — main's own Makefile change still present, the work's added beside
it.

## The number

That left the head comment's one live claim of the class's size, derived from
the Makefile by `TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles`:

- `main` raised it 114 → **119** (ADR 0078 D3's five detection pins).
- the work commit raised it 114 → **115** (its one census arm).
- the merged tree is **120**, and neither side's text says so.

This is v3tt4's finding arriving at its smallest possible size. There, two
sides had added two and twelve pins across four door variables and the union
was real work. Here the union is nothing — one line in the Makefile, two
disjoint narrative blocks in the comment — and the conflict is still not
resolvable by `--ours` or `--theirs`, because the pinned number is **119 + 1**
and that is an arithmetic fact about the merge rather than a line either
parent wrote. `git checkout --theirs` reds the arm; so does `--ours`. The
resolution is main's block with one spelled word replaced.

So the rule generalises past "both sides doored pins": **any merge-back whose
either side touches a door variable owes this arm a fresh resolution, however
small the two diffs are.** A one-line Makefile change was enough.

## The reading the merge owes

v3tt4's other half: each parent's timing was taken over a class the merged
tree does not have, so keeping either attaches a real measurement to a size
it was never read at. `main`'s entry was 90-117s at 119 pins; `372iq`'s was
55-56s at 115, which it had inherited from `ranger-base-h1q5o` unchanged.
Three warm runs here, at 120, on the tree that landed:

| run | wall | 1-min | 5-min | 15-min | suite slots |
| --- | --- | --- | --- | --- | --- |
| 1 | 67.73s | 4.46 | 4.11 | 8.80 | both free |
| 2 | 68.95s | 7.08 | 5.01 | 8.77 | both free |
| 3 | 72.75s | 7.90 | 5.84 | 8.82 | both free |

One warm-up run discarded. A 5.0s spread on a quiet box whose load rose
gently across the three — which puts it where the chain predicts, and makes
it the entry that TESTS the standing advice rather than restating it. "Price
this command at a minute on a quiet box" predicted ~60s; the quiet box said
68-73s. The advice is right about the band and about eight seconds light at
this size, so the floor to quote is seventy now.

**An earlier set of four, taken on the first replay, is not the reading** —
63.88s, 64.43s, 70.70s and 78.75s at one-minute loads of 2.80 to 13.23. They
were read over `524e3e3b` + this work, and `ranger-base-w0w7u` reached main
before the commit landed, which puts one more `internal/posse` test file in
`pkgcensus-check`'s corpus. Same rule as v3tt4's, applied to the merge's own
measurement rather than to its parents': a reading over a class the landed
tree does not have is not a reading of the landed tree. Their one surviving
use is as the second data point for "the level does not order the wall" — the
load fell monotonically 13.23 -> 9.11 -> 5.73 across runs 1-3 of that set and
the SLOWEST was the one at 5.73.

The one-minute load fell monotonically across the three and the wall did not
follow: the **slowest** run is the one at 5.73 with both slots free. That is
the chain's "what the spread tracks is how much the load MOVES" from the
falling side, and it is now the fifth reading in a row to say the level does
not order the wall. A 14.3s spread, between the 1s a quiet steady box gave
(`ranger-base-h1q5o`) and the 84s a sharply moving one did
(`ranger-base-pyl22`) — and the box was shedding this bead's own `make test`
the whole time, which is what "moving" was here.

The pin this bead carried across costs nothing measurable: it is one more arm
over a corpus `corpus-check` already walks (`ewdGoFiles`, non-test `.go`).

## The census numbers did NOT need re-measuring, and that is checkable

`protocolchildpath_qa_test.go`'s head comment carries MEASURED counts — 21
probes, 7 `exec.LookPath` and 14 `resolveOutside`, against 36 exec'd
spellings over 28 files — taken "at main 2225b0b0 + this change". A merge-back
normally owes those the same re-reading the timing gets. This one did not,
and the reason is one line: the arm's corpus is `ewdGoFiles`, which takes
non-test `.go` only, and every file `00fyg` touched outside the Makefile and
the two docs is a `_test.go`. So the corpus is identical on both sides of the
merge and the counts are still readings of the tree that landed.

The arm and its planted-shape partner (`TestQATheProbeCensusCanStillSayNo`,
eight rows) pass on the merged tree, which is the live confirmation.

## Verified

All of this on the THIRD replay's tree, which is what landed; the figures in
brackets are that run. `make fmt-check`; `make notes-check` and `python3 scripts/notes-index.py --check`;
the register arms under `-run`
(`QATheHeadCommentsPinAndDoorCountsAreTheMakefiles|QAEveryTreeWidePinHasADoor|QAEveryTreepinsDoorFilterNamesItsPins|QATheTreepinsEnumerationRuleMatchesWhatItClaims`,
ok 6.5s); the new arm with its planted-shape partner and arms 1-2 of the same
file, ok 1.1s; every `Backup|StageReason|StageBackup|BackupTool` test under
`-tags posse_arm3`, 40 of them green including `w0w7u`'s two new ones, which
is the overlap's behavioural check and not just its textual one;
`make tree-check` three times as tabled plus a discarded warm-up; `make test`
whole, green over all three arms at **0 failures** (arm 1 `cmd/posse` 182.1s,
`internal/posse` 371.5s, `internal/treepins` 400.1s; arm 2 262.0s; arm 3
286.7s), silent-revert audit 2212 commits 0 untriaged; `go run
./cmd/checkorphans` clean.

`make test` was run whole on all three bases and was green on all three; the
second run's figures were arm 1 `cmd/posse` 281.4s / `internal/posse` 484.1s
/ `internal/treepins` 515.0s, arm 2 344.5s, arm 3 339.0s, at 2211 commits
audited. The tree-check readings quoted above and in the head comment are the
SECOND base's, and they stand for the third unchanged: all eight of this
bead's paths are byte-identical across the two, and `ranger-base-uvcj8` added
no pin, no door and no Makefile line, so the class is one hundred and twenty
behind seventeen on both.

`main` was re-read immediately before the commit: `9465b3d7`,
`git merge-base --is-ancestor main HEAD` yes.

## rq986's process finding, arriving on the very next bead

`docs/notes.d/ranger-base-rq986.md` ends by saying: on a merge-back, re-read
`main` AFTER the suite as well as before it, because a full `make test` is
20-25 minutes here and this box lands on main every few minutes at a busy
hour. That is exactly what happened, one bead later.

| time | what happened |
| --- | --- |
| 02:05 | `372iq` commits on its branch (`5d7b9245`), parent `2225b0b0` |
| ~02:10 | this bead dispatched; `main` is `524e3e3b`, one commit ahead |
| 02:12-02:20 | replay, the one count-sentence conflict resolved |
| 02:20-02:48 | `make test` whole, green over all three arms |
| 02:49-02:52 | three `make tree-check` readings, head comment updated |
| 02:54 | replay committed as `32526f3d`; `main` re-read — **still `524e3e3b`** |
| 02:5x | **`ranger-base-w0w7u` lands on main (`cf78f918`)** |
| 02:5x | second commit made (`4098c78a`); the SAME check now says NO |
| 02:5x-03:33 | second replay onto `cf78f918`; three fresh readings; `make test` whole, green |
| 03:33 | re-read immediately before the commit — **`main` is `9465b3d7`** |
| 03:33-04:0x | third replay onto `9465b3d7`; `make test` whole, green again |
| 04:0x | re-read once more — `9465b3d7`. Committed, commented, closed |

So the check was typed four times across this bead and **changed its answer
twice**, the second time in the single minute between a finished suite and
the commit that followed it. The lesson rq986 drew — re-read `main` after the
suite — is right but not sufficient on its own: the window is not "while the
suite runs", it is **every minute between the replay and the moment the
launcher fast-forwards**, and no check a session can type closes it. What
makes it cheap rather than expensive is that the check is one command, so the
discipline that actually works is to re-read `main` immediately before the
last commit, and to treat commit-and-close as one burst rather than two steps
with a measurement between them.

**And the second and third replays cost almost nothing, which is the part
worth planning around.** Both were disjoint from this bead's paths, so
`merge-tree` was clean and every one of the eight replayed blobs came out
byte-identical to the tree the previous suite had already run over. A replay
after a DISJOINT landing is a `reset --hard main` and a `cat-file` loop — no
resolution, no re-measurement, no new numbers. What it does cost is another
25-minute `make test`, because `internal/posse` is one Go package and a
sibling's edit to `gates.go` shares a compilation unit with this bead's
`cage.go` even when no line of either refers to the other. For the third
replay that risk was also checkable directly and came back empty: the new
census arm passes over the new `gates.go` and `promote.go`, and
`ranger-base-uvcj8`'s whole diff contains no `LookPath`, `resolveOutside`,
`protocolCmd`, `protocolBin` or `protocolAvailable` line. The suite was run
anyway, because "no line refers to the other" is an argument and a green
suite is a reading.

Replaying again cost little here, and the reason is worth keeping: `w0w7u`'s
own commit message says it deliberately left this bead's three halves alone
("Writing them again here would turn that commit's one conflicting file into
three (ADR 0022)"), so the second merge-tree was clean and the only overlap
was one file whose two changes sit 300 lines apart. A sibling seat reading the
in-flight block and routing around it is what made the second replay a
`merge-tree` and a both-directions diff instead of a conflict.

The first replay's commits are kept in this worktree's reflog —
`32526f3d` (the replay onto `524e3e3b`) and `4098c78a` (its fourth reading) —
and neither is what landed.

And the check was typed a third time, after the second suite: see Verified.
