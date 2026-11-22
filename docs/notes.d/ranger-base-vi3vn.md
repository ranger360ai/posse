## Four closes verified, and a wall probe's execution count is two pins over (ranger-base-vi3vn)

2026-11-21, this box (linux 7.2.15-1-omarchy x86_64, uid 1001, go1.27.3, git
2.56.0, bwrap 0.13.1, 16 cores). The verify of ranger-base-84vdv,
ranger-base-otoi4, ranger-base-ahlt5 and ranger-base-axt1f. All four hold.
Verified at 86139806, which is main's tip and the first tree to carry all
four — so every reading below is of the combination, not of any closer's own
base.

What is worth keeping is four methods and one recorded finding.

### The finding: five git-refs pins, not seven (ranger-base-axt1f)

`docs/notes.d/ranger-base-axt1f.md:84-85` reads "`^TestLiveBwrapGitRefs` in
that venue at `-count=400` — 2,800 executions of the seven git-refs pins".
There are FIVE functions matching that filter, so the run was 2,000
executions. MEASURED at every tree the close could have meant: the red tree
516f07893b6c, its parent, gilfoyle's own branch tip 54ef6392 and main — five
at each.

RECORDED and not filed (ADR 0006 §6): the substantive claim is that the
member did not reproduce in a large number of executions, and 2,000 carries
it as well as 2,800. No process changes and no pin is wrong about what it
measures.

### A guard over captured output is graded by injecting into the real one

ranger-base-otoi4's close replayed its new guard over text the bead had
captured, and deliberately did not commit that replay — "a pin over text this
bead captured would assert the capture and not the door". The same reasoning
applies to grading it: a battery that re-implements the guard grades a copy.

`runDoor` returns `doorRun` by value, so `r.out` is assignable in the test
body. Appending one hostile line there, immediately above the shipped guard,
runs the COMMITTED guard over chosen input. Eight arms, each planted alone
with the file restored byte-identically between them:

| arm | guard | injected | verdict |
|---|---|---|---|
| 0 | shipped | nothing | GREEN |
| A | shipped | a foreign withdrawal naming no target | GREEN |
| B | pre-otoi4 | the same line | RED |
| C | pre-otoi4 | nothing | GREEN |
| B2 | pre-otoi4 | a withdrawal of one of OUR pids | RED |
| D | shipped | a withdrawal of one of OUR pids | RED |
| E | shipped | a truncation of one of OUR pids | RED |
| F | shipped | one pid's `/plant` lines removed | RED (`len(ours) != 2`) |

A and B are the escape and the fix as a pair, over one byte-identical input.
C is why the red tracked the run queue. B2 says the narrowing dropped no
coverage. F is the close's new premise assertion, and it is live.

The production mutant the arm exists for — `charge{pid: 0, key: key}` at
`door_linux.go:597` — still reds through the narrowed guard, on both
`wantDelayed` and the "paid D" count. So the scope change did not cost the
pin its target, which is the thing a narrowing has to be shown not to do.

### The premise, reproduced with no load

The bead asked for one measurement first: whether a `/bin/sh` fixture under
the supervisor can be withdrawn by something the test does not control. Driven
here with ARM 21's own script plus one UNRELATED plant exec'd by a python3
caller carrying an SA_RESTART handler on SIGURG, nudged by a forked child
polling `/proc/<pid>/exe` — `withdrawalFixture`'s own mechanism pointed at a
process ARM 21 knows nothing about. MEASURED at a one-minute loadavg of 3.46,
no load generated (the standing ruling here is no load testing):

```
coldexec: withdrawn pid 2940612 /proc/2940612/cwd/./other   x59
coldexec: delayed   pid 2940618 /proc/2940618/cwd/./plant    <- ours
coldexec: warm      pid 2940621 /proc/2940621/cwd/./plant    <- ours
coldexec: 70 execve notifications, 2 cold delayed (1 warm, 8 old, 0 absent)
ELAPSED first 602ms / ELAPSED second 7ms
```

59 withdrawals, every one of a pid that is not ours, both of ARM 21's own
charges correct. Re-deriving the shipped guard's own question over that
output: `ours` has exactly 2 members, 0 of the 59 name either, so the old
guard reds on this run and the new one does not. jian-yang's reading was 72/60
at loadavg 4.67; the shape and the conclusion are the same and the counts move
with the nudge loop.

### A mutant is graded on where it landed, not on whether the file changed

Arm B above read GREEN on its first run, which was the battery's fault: the
injection was anchored on `r.wantCode(t, 0)`, which is not unique in that
file, so `str.replace(..., 1)` planted it in an earlier test function. The
battery had asserted that the file CHANGED, which it had. Anchor every mutant
on a string asserted unique in the file (`s.count(old) == 1`), and prefer an
anchor inside the body being mutated.

This is the same shape as my own standing note about a fix into a helper
leaving the call unwitnessed, one layer out: presence is not placement, for a
mutant exactly as for a fix.

### Grade a close's own correction, and measure a count at the tree it was taken on

Two numeric claims, opposite outcomes.

ranger-base-84vdv's close CORRECTED itself from eight reproduced rows to
seven. Driven by taking the pre-fix pin from `70e02fca^` and planting the
`<xx>` directory: exactly SEVEN failure rows — shape 1 rename-over,
rewrite-in-place and read-through; shape 2 unlink, rename-over,
rewrite-in-place and read-through — with the bead's own first failure line
among them, and the shared object's digest, size, mode and `cat-file` output
unchanged in every one. The correction is right, including its breakdown. (An
eighth line matches a naive `file.go:NN:` grep and is the venue marker,
`wall venue: measuring (ADR 0097)` — which is also what says this seat
measures these pins rather than abstaining.)

The same close's "3495 eligible tests" read 3497 at the landed commit
70e02fca, and 3495 exactly at 1acb6fc0 — the branch tip the close was written
on. A landing that replays onto a moved main carries a different tree than the
one the closer measured, so measure a count at THAT tree before calling a
delta an error. +3 test functions landed between the two, which accounts for
the current 3500 exactly.

### A live/source pin pair is graded in two columns, and the green one is the point

ranger-base-axt1f adds a source pin because the call it protects has no
observable a behavioural test can reach. That claim is only as good as the
second column: seven mutants, each red on the source pin AND green on the live
arms.

| mutant | source pin | live arms |
|---|---|---|
| T4's fatal back to `--prune:\n%s` | RED | GREEN |
| `walled` returns `TrimSpace`, not the account | RED | GREEN |
| `packUnderALiveSeat` keeps the deadline fatal's call, drops it from the RETURN | RED | GREEN |
| `bgSeatAccount` folds with its own `Fields` join | RED | GREEN |
| the same, returning words without the status | RED | GREEN |
| the same, returning raw output after a newline | RED | GREEN |
| the pin's own control: verb set matching nothing | RED | — |

The third row is the one that justifies the pin's shape. It is the placement
mutant: a `bgSeatAccount` call survives in the body, at the 30s deadline
fatal, and only the RETURN loses it — so an "is it called in this body" check
passes while every message the file interpolates goes back to naming no exit
status. Reading every two-value RETURN is what catches it. The pin reads 33
testing format calls against its own floor of 10.

### The merge-back, read from both sides (ranger-base-ahlt5)

One original in (e0c7c34d), one commit out (bb2b5c14), no squash. The
`(author email, AUTHOR date, subject)` key sets over the two ranges are
identical, so `equivalentOnBase` pairs it. Per path, the replay's change is
byte-identical to the original's for AGENTS.md, `internal/posse/gates.go`,
`internal/posse/continuewall_qa_test.go` and the notes fragment; only the ADR
and the notes index differ, which is what the close says.

The ADR union is complete from BOTH directions, and that is the reading worth
copying: `git diff bb2b5c14^ bb2b5c14` over the file is exactly 6zw05's two
additions, and `git diff e0c7c34d bb2b5c14` over it is exactly hkp3s's two.
Neither side lost a line, and 6zw05's forward reference — "That is
ranger-base-hkp3s, not this bullet" — is answered by hkp3s's paragraph
immediately below it, which is the order the close argued for.

The close's code claim holds too: `gates.go`'s four `Way` strings still name
`--abort`, so the prose and the code agree today, and ranger-base-2hmfq is
open and owns moving both. Nothing to file.

### What this verification did not do

The full three-arm `make test` was not re-run on the combined tip. Each closer
ran it on their own tip, and ranger-base-ahlt5 ran it over the combined tree
at 63ae11fd, which predates ranger-base-w37g2 and ranger-base-axt1f. Run here
instead, all at 86139806: `make tree-check` green (0 FAIL, internal/treepins
80.4s, `posse-land-check`'s five steps including `GOOS=darwin go vet ./...`),
`bash docs/adr/0097-wall-venue.probe.sh venue+check` rc=0 in 111s at load
6.03 with "wall check: green, 47 pins measured" and 0 FAIL lines,
`go test ./cmd/coldexec` green at `-count=1` and `-count=5`,
`-run TestLiveBwrapSeatStore` green at `-count=40`, `TestQAContinueWall`
green, `make fmt-check`, `make notes-check` and `make verify-parallel` green.
`go run ./cmd/checkorphans` clean, no suite slot held by this seat.

ranger-base-axt1f's underlying red remains unexplained, which its close states
and does not claim to have closed; the structural answer is ranger-base-s9h52.
