# Three pin gaps from the ranger-base-bfbx8 verify, and one seam that had to move for the third

ranger-base-ze7xz, 2026-11-15. The posse half of a six-finding verify
(ranger-base-bfbx8, over the closes ranger-base-me10w, ranger-base-wjnfr,
ranger-base-ig79v, ranger-base-h5iae). All three are pin gaps found by
mutation rather than behaviour defects: nothing on main misbehaved, and every
mutant below left the package it lives in green WHOLE before these pins
landed.

Environment for every reading here: Linux 7.2.15-1-omarchy, 16 cores,
go1.27.3, this worktree, warm build cache. Mutants planted one literal
replacement at a time over an otherwise unmodified tree by a script run by
path, each file's md5 asserted before the plant and after the restore, and
`git status --porcelain` carrying nothing but the intended edits at the end.

The ranger-core half of the same verify (findings 1-3) is NOT here and did not
land with this: see "The other half went to another repo" at the foot.

## Finding 4 — the pulse reminder's cursor is the feed's MAX SEQ, not its entry count

`pulse.go`'s typed fallback arms the coordinator at the feed's newest entry:

```go
arm = PulseWaitCommand(pulseFeedCursor(PulseFeedPath(d.App)))
```

MEASURED (ranger-base-bfbx8): replacing that reading with the feed's entry
COUNT —

```go
arm = PulseWaitCommand(len(pulseFeedEntries(PulseFeedPath(d.App))))
```

— left `go test -tags posse_arm3 -count=1 ./internal/posse` ok 133.129s, rc=0.

**Why neither existing arm could see it, and it is the same reason both
times:** the two numbers were EQUAL in every fixture that types a reminder.
`TestPulseTypesAndRemindsWhenNoWaiterIsArmed` composes its text over an empty
feed (count 0, max seq 0), and `TestPulseReminderCursorIsTheFeedsNewestEntryNotZero`
— the pin ranger-base-wjnfr added for the `--after 0` reading, which verifies
and re-killed its own two mutants here — drives exactly one tick into the feed
(count 1, max seq 1). Nothing else in the tree typed a reminder over a feed
whose count and max seq differ.

The real feed differs by construction. `PulseFeedLines` is 500 and an append
keeps the newest 500 entries, so a long-lived feed holds 501..1000: count 500,
max seq 1000. The count reading then pastes `--after 500` where the oldest
kept entry is 501, and pulse.go's own note says what that gets — "a `--after`
older than the oldest kept entry gets `gap`". The coordinator's first wake is
handed every kept entry, up to 500, instead of this delivery's line alone.
That is the consequence `--after 0` had, which is the whole of
ranger-base-wjnfr, arriving by the other reading of the same file.

**The pin:** `TestPulseReminderCursorIsTheFeedsMaxSeqNotItsEntryCount`
(pulseswitch_test.go). It SEEDS the feed with one entry at seq 7 before tick
one, so after the tick the feed holds two entries whose max seq is 8 — a
seeded line and not seven ticks, because seven ticks would put the count at 7
and the max at 7 again. The want is computed from `pulseFeedCursor`, and the
count, the constant 0 and the off-by-one are each ruled out by name, so one
feed shape discriminates all four readings. `wantFeed` is deliberately not
used: it asserts `seq == i+1` over the whole feed, which is true of a feed
starting at 1 and is exactly what this fixture must not do.

It takes the waiter's flock like its sibling, so it carries no `t.Parallel`
and has an entry in cmd/testparallel's serial map. That entry is load-bearing,
checked both ways: with it, `go run ./cmd/testparallel ./internal/posse check`
is clean over 3332 eligible tests and exits 0; without it the same command
exits 1 naming exactly this test.

## Finding 5 — the nest gate's RULE and its ACTION were each pinned, and the wiring between them was not

ranger-base-ig79v split the gate into `bwrapNestDecision` (the rule, pinned by
wallvenue_qa_test.go arm 1) and `bwrapNestAct` (the action, pinned by arm 7
through a `nestSink` recorder). Its close declared a residual:
"bwrapNestSkip's argument plumbing (t.Name(), liveBwrapNestRefusal(),
wallVenueActive(t))".

The residual was larger than that. The LINE that composes the two halves was
reachable by no test, and two mutants inside `bwrapNestSkip`, neither touching
either half, each left `go test -count=1 ./internal/posse` green WHOLE
(MEASURED, ranger-base-bfbx8):

| mutant | result |
|---|---|
| `bwrapNestAct(t, act, "")` — the marker DECIDED and never SAID | ok 193.036s, rc=0 |
| `bwrapNestAct(t, nestAbstain, msg)` — every member SKIPS whatever was decided | ok 137.750s, rc=0 |

Either is ADR 0097 finding 2(b) verbatim, one level up: 32 members, zero
marker lines, a venue run green, silent and uncountable, which
`scripts/posse-wall-check` answers exit 3 for and ADR 0097 D5's watchdog files
as "wall check silent" rather than as the wall regression the run existed to
find. That script has landed (ranger-base-lagas), so the consequence is live.

**The fix is the same move ig79v made one level down:** the composition is now
`bwrapNestSkipTo(s nestSink, name, refusal string, venue bool)`, which takes
the sink and the three facts as arguments, and `bwrapNestSkip(t)` is the three
reads plus a call to it. Arm 8
(`TestQAWallVenueGateWiresTheRuleToTheAction`) drives it with a
`nestRecorder` over the four rows a seat or the tree can take, asserting the
sink AND the words — and the words are computed from `bwrapNestDecision`
rather than written out, because a hand-written want would be a third copy of
D3's sentence and would pass over a composition that swapped its arguments.
It also asserts TWO `Helper` calls, `bwrapNestSkipTo`'s and `bwrapNestAct`'s,
because all three frames have to be marked or the marker line names a line of
wallvenue_test.go where the member's own belongs.

The pin's name stays outside `wallVenueMemberPattern`, like the four existing
arms, so a venue run does not count it a member of the class it pins.

Both mutants now red, and arm 8 is the only FAIL in either run (MEASURED
2026-11-15, `go test -count=1 -v -run TestQAWallVenue ./internal/posse`, rc=1
both times) — which is the same reading from the other side: arms 1 and 7
still cannot see them.

A note for whoever mutates this next: after the split, the ONE-TOKEN
spellings of those two mutants do not compile — `act` or `msg` is then
declared and not used inside `bwrapNestSkipTo`. The semantic mutant has to
drop the matching binding too (`act, _ :=` / `_, msg :=`), which is what the
readings above are. A build failure is a stronger wall than a red test, but it
is a property of the local variables and not of the pin, so the pin is what
these readings are about.

**Residual, and it is now exactly what ig79v's close claimed it was:** the
three argument READS in `bwrapNestSkip`. That cannot be shrunk from inside
this package — a pin calling `bwrapNestSkip` is either outside D3's member
pattern (and gets `t.Fatal`) or is itself counted a venue member, and a
`*testing.T`'s log is not readable in process.

## Finding 6 — the RESOLVE in ADR 0095 D1's key

```go
if exists && writeGranted(own, d) && bwrapGitConfigRepo(d) == absResolve(cwd) {
```

ranger-base-ig79v's finding 1 closed the question of whether the second
conjunct fires on the right rows. This is the other term in the same
comparison, and the two sides are resolved differently on purpose:
`bwrapGitConfigRepo` resolves NOTHING — it is `filepath.Dir(filepath.Dir(d))`
over a deny path the carve-out has already `absResolve`'d (seatbelt.go's deny
loop) — while the cwd arrives unresolved, because `BwrapArgs` is handed the
session dir as its caller spells it. So the `absResolve` on the right is what
makes the key a question about the DIRECTORY rather than about its spelling.

MEASURED (ranger-base-bfbx8): dropping it — `bwrapGitConfigRepo(d) == cwd` —
left `go test -count=1 ./internal/posse` ok 204.335s and `./cmd/posse` ok
131.092s, both packages green WHOLE. Every existing arm spells its cwd the way
its fixture tree spells it, so the resolve decides nothing in any of them:
`bwTree` hands back an already-resolved root (its own comment says why), and a
resolved path resolves to itself.

**What the miss costs, in the expensive direction.** With the resolve gone, a
cwd reached through a symlink makes the two sides unequal, the refusal does not
fire, the render puts a `--ro-bind` wall over a `.git/config` the seat can
write through its own cwd grant, and the first host `git config` in that repo
replaces the file by rename, takes the wall off and wall-watch kills the seat
(ranger-base-0qpsw D2). A seat killed mid-run, not a launch refused — the more
expensive of the two directions ranger-base-hi2lt priced.

**The pin:** `TestBwrapArgsGitConfigRefusalResolvesTheSessionDirsSpelling`
(bwrapargs_qa_test.go), beside the three existing
`TestBwrapArgsGitConfigRefusal*` arms. It plants its own symlink, because
`t.TempDir()` carries none on this box and that is the whole reason no
existing arm reaches the term. A symlink and not a `..` spelling:
`filepath.Abs` already cleans a lexical path, so an uncleaned spelling is
caught by the `Abs` half and says nothing about the `EvalSymlinks` half.

Three fixture claims are asserted before the row leans on them — the cwd is
not the repo's own spelling, `absResolve` closes the gap, and
`bwrapGitConfigRepo` really does leave its side unresolved — and the control is
a NESTED repository's config under the same symlinked cwd, which must stay
unrefused both before and after the resolve, so the row cannot pass by a
symlinked cwd refusing everything. `underDir` resolves both of its arguments,
so neither the `own` filter nor `writeGranted` can be what the row reads.

**Not claimed: a production miss.** Whether the deny path's spelling carries a
symlink on this box was not measured. What is measured is that no pin in
either package could tell the resolved comparison from the unresolved one, and
that it can now.

No behaviour changed in bwrap.go. The asymmetry between the two sides is
stated here and in the pin's own comment rather than closed: resolving the
left side too would WIDEN the refusal, and a wider refusal is a seat that
never launches — the direction ranger-base-hi2lt measured costing seven renders
in this package. If it should be closed, that is ADR 0095 D1's key and so the
architecture lane's.

## The other half went to another repo

Findings 1-3 of the same verify are three missing pins in
`davidstacy/ranger-core`'s `src/ft/pipeline.py` (the paged stored-article
query's `SKIP $skip`, the loop's `len(articles) < limit` condition, and the
backlog warning's `not exhausted` guard). ranger-base-ze7xz routed them "on
`jian-yang/represent-stored-articles`, pre-merge, rather than in a follow-up"
— and that routing fact expired before the bead was worked: PR #119 merged as
`0a9d571` under the operator's 2026-11-14 merge-queue ruling
(ranger-base-6gwys, closed), and `0a9d571` is in `origin/master`. There is no
open PR to add them to, and this session's venue is the posse worktree.

They are filed as ranger-base-lgrhq — their own `-l code -l debt` bead against
ranger-core's master, with all three probes carried over verbatim, a DONE WHEN,
and the verification that all three sites are still spelled on master exactly
as the readings were taken (`SKIP $skip` at 385, the loop's condition at 508,
the warning's guard at 546). Nothing of findings 1-3 landed in this commit.
