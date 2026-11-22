# The git-refs wall pin threw away the exit status before the message was written (ranger-base-axt1f)

2026-11-21. Second reading of the class
`docs/notes.d/ranger-base-a688i.md` named, at a different member, five hours
later. Not reproducible, and this fix does not claim to have closed it; what
it closes is the reason nobody could tell.

## What arrived

`scripts/posse-wall-check` read RED at `516f07893b6c` (tree `86196ee42f96`),
1 of 47 members, and the whole of the evidence was:

    FAIL TestLiveBwrapGitRefsCommitsAfterAnOutsidePackRefs:
    bwrapgitrefslive_test.go:477: the seat's commit failed after an outside
    pack-refs --all --prune:

A symptom, a colon, and nothing after it. Line 477 is T4's one fatal: a
seat started inside the rendered wall, held at a ready file while
`git pack-refs --all --prune` ran in the operator's checkout, then released
to `git commit`. The commit did not exit 0.

## Why line 2 was nowhere, again

`evidence()` reports a failed member by the first `file.go:NN:` line of its
output that is not the venue marker, 300 characters, and **nothing after it**
(ADR 0097 D3); the launcher puts that one line on the bead. The fatal's
format string was `…--prune:\n%s`, so the seat's words were on line 2 and
were dropped in transit. Both halves work as designed — a per-member tail
would drown the verdict — so the budget is one line and it is the MEMBER's
job to spend it on a cause.

## And the status was gone even from line 2

This member had a second hole that `ranger-base-a688i`'s did not, and it is
the one that mattered here. `bgLive.walled` and `bgLive.packUnderALiveSeat`
both reduced the seat's error to `err == nil` **before** any message was
formatted, and returned only the combined output. So the exit status never
existed as text anywhere: a `git` that exited 128 at `cannot lock ref` and a
seat ended from outside produced byte-identical reports, and the only thing
that could have discriminated them was a tail that would have been truncated
anyway.

`exit status 128` sends the next reader to this render. `signal: killed`
sends them to the box, and it is a real candidate on a 16-core host running
six other seats — this repo's standing orders carry a census of eleven
confirmed cross-seat pattern kills (AGENTS.md, "Ending anything"). Nothing
in the report could tell those two apart.

## What changed

`internal/posse/bwrapgitrefslive_test.go` only, plus the pins.

`bgSeatAccount(err, out)` is what both seat runners return now: the exit
status, folded, then the seat's words folded onto the same line. Every
`:\n%s` in the file became `: %s`, so no format string in it carries a
newline at all.

The fold is `wwOneLine`'s (`wallwatchlive_test.go`, ranger-base-a688i) and
not a second spelling of it — two foldings of "put this on one line" are two
answers waiting to disagree about what a member's line may contain. What is
NOT borrowed is its silence case: `wwOneLine` answers a wordless bwrap in
wall-watch's own terms, and here a seat that said nothing means something
else, so each caller keeps its own account of silence and shares the fold.
A silent seat reads as a finding — "no git error and no bwrap refusal, which
is the shape of a seat ended from outside rather than one the wall turned
away" — because that absence, beside the status, is the discriminator.

`bwrapgitrefsaccount_qa_test.go` holds the two pins. Neither needs bwrap, a
venue or a fixture, and neither is named like a member
(`^Test(QA)?Live(Bwrap|Wall)`): they measure no wall. Each names its one
file and enumerates nothing, so neither is in internal/treepins'
one-package-census class and neither needs a Makefile door.

## MEASURED 2026-11-21 (linux 7.2.15, bwrap 0.13.1, go1.27.3, 16 cores)

Reproduction attempts, all green:

- `bash docs/adr/0097-wall-venue.probe.sh venue+check` at the red tree,
  before any change: rc=0 in 115s at load 10.91,
  `wall check: green, 47 pins measured`
- the whole member set in D1's venue, 6 rounds: rc=0 at 127s, 97s, 93s, 93s,
  99s and 105s, at loads of 13.05, 14.13, 5.97, 3.71, 2.83 and 6.99, 0 FAIL
  lines in any of them
- `^TestLiveBwrapGitRefs` in that venue at `-count=400` — 2,800 executions
  of the seven git-refs pins, T4 and its control among them: `ok` in 49.3s,
  0 FAIL

So the red is not reproducible by repetition, by the member set, or in the
venue.

Where the untruncated output would have been, and was not: the venue's run
dir, `~/homes/ranger/state/landed-check/run-463411134`, is cut fresh per run
and was already gone. `~/homes/ranger/state/dispatch-watch.log` holds the
same truncated line, the venue's full argv beside it, and — checked, because
`signal: killed` was a live hypothesis — no load-guard kill, no orphan reap
and no `posse kill` anywhere in that pass.

One honest gap in the reproduction: the probe's `venue` arm is D1's wall as
ADR 0097 documents it, while the launcher's is the judge's cage PLUS D1's
flag — the same namespaces, private `/tmp` and `/var/tmp` and
`POSSE_WALL_VENUE=1`, with tmpfs masks over the runtime CLI homes, `/run`,
`/run/user/<uid>` and `~/.ssh` that the probe's arm does not carry. The
git-refs fixture builds its repo under `TMPDIR` and reads none of those, so
the difference is not in anything this member touches; it is still a
difference, and the bead's own "What to do" names the probe as the venue to
reproduce in.

Not this window's green-to-red either. The window holds one landing,
`516f07893b6c` (ranger-base-z56s9, mine): a `PR_GET_PDEATHSIG` read moved
to package init, an `AT_SECURE` row read from `/proc/self/auxv`, and
diagnostics on one wall-unix arm's red path. It touches no git grant, no
render and no fixture this member uses, and its one added `init` is a single
`prctl`. ADR 0088 D5 assigns by position, not by cause, and this is the
positional case.

## Shown able to fail, seven mutants

One at a time, the two files restored byte-identically between arms, each run
over `-run 'TestQABwrapGitRefs|TestLiveBwrapGitRefs'`. All seven red; every
one of them left the live arms themselves green.

- T4's fatal reverted to `--prune:\n%s`, byte-for-byte the line this bead was
  filed on
- `bgLive.walled`'s return reverted to `strings.TrimSpace(string(out))`
- `bgLive.packUnderALiveSeat`'s return reverted the same way
- `bgSeatAccount` folding with its own `strings.Fields` join instead of
  `wwOneLine`
- `bgSeatAccount` returning the words without the status
- `bgSeatAccount` returning the raw output after a newline
- the source pin's verb set reduced to a name nothing matches (its control)

The fourth mutant in that list is why the call-site arm reads every
two-value RETURN of each runner rather than asking whether the body calls
`bgSeatAccount` somewhere: the first draft asked the looser question, and a
mutant that left `packUnderALiveSeat`'s 30-second deadline fatal calling it
while taking it out of the return — the normal path, the one every arm reads
— passed with every live arm green.

## The class, third time

`docs/notes.d/ranger-base-a688i.md` states it: a wall-pin member's failure
message gets ONE line, and a first line that names a symptom and defers the
reason to a captured buffer reaches the bead empty. This bead adds the half
that fragment could not have known, because its own member had no such
value to lose:

**a runner that reduces its child's error to a boolean has destroyed the one
row that discriminates, and no amount of line budget brings it back.** The
shape to look for is `err == nil` as a return value where the error itself is
never rendered. The words a child wrote are a different fact from the status
it exited with, and only the second one is always there.

`ranger-base-w6kke` (filed by ranger-base-a688i) is the nearest remaining
instance of the line-budget half.
