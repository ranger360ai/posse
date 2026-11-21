# ranger-base-rh8y0 — the landed loop's pending re-read, and four findings that are not this repo's

Five live findings from the verify of four closes (ranger-base-vcfc1, over
ranger-base-kq88h, ranger-base-soqa1, ranger-base-2qlz2, ranger-base-4gyib).
One of the five is code in this repo. The other four are a red that fixed
itself while this bead was open, and three findings in two OTHER repos — so
what landed here is one pin and three handoffs, and this fragment says which
is which rather than leaving a reader to guess from the diff.

MEASURED 2026-11-21, this worktree, linux 7.2.15, 16 cores.

## What changed: the re-read of `pending` is pinned (F4)

`loop()`'s finishing critical section (internal/posse/landedcheck.go) re-reads
the pending set before it clears `running` and closes `done`. That re-read
arrived with ranger-base-soqa1 and its own comment calls it load-bearing —
"the one thing Schedule's fast path relies on this goroutine for" — and no
test could tell it from its absence.

WHY THE WINDOW EXISTS. Before soqa1 the loop found no repo and finished under
ONE hold of `c.mu`, so there was nothing to re-read. soqa1 releases the lock
to print the dropped-landing lines, which is right and deliberate — `out` is a
writer the caller owns and takes a lock of its own, and holding the pending
set's mutex across it is how two locks become an order to get wrong. The
re-read is what restores the invariant the release gives up.

WHAT THE MUTANT COSTS. `if len(c.pending) > 0` → `if false`: a landing that
arrives while those lines print finds `running` true, adds itself under
`Schedule`'s documented fast path, and is then orphaned by a loop that exits.
An entry in `pending` with no goroutine coming for it, no ◷ line, nothing in
D4's record and nothing in D6.3's quiet count — a tree that landed and was
never looked at, in silence. That is the exact shape ADR 0088 D6.5's amendment
was written to answer, reintroduced one level down.

THE PIN is `TestAStopDropsALandingThatArrivesWhileItIsSayingSo`
(internal/posse/landedinterrupt_test.go, arm 2), the sibling of
`TestAStopDropsThePendingSetAndSaysSo`: that one pins the ORDERING claim the
same block makes, this one the claim about the set itself.

DETERMINISM IS A GATE AND NOT A WIDE WINDOW. `ldiSlowOut`'s header already
says why a sleeping writer is used at all — the first draft of soqa1's drop
was green in isolation and red in the arm-2 suite under load, "the worst shape
a pin can have". A sleep alone still only makes the window probable, so
`ldiGateOut` closes a channel as the FIRST write BEGINS and only then sleeps:
a test that has received from it knows the loop goroutine is inside `c.say`
and therefore provably outside `c.mu`. The 250ms after that is margin, not
mechanism. 12 consecutive runs and 4 `-race` runs green, 0.54s each.

THE STOP IS FIRED BEFORE ANYTHING IS SCHEDULED, because `dropped` is non-empty
only when this launcher is stopping — the stopping path is the one place the
loop provably prints with the lock down. So what the re-read buys here is the
second landing's ◷ line and an empty set, NOT a judgement: behind a fired stop
both tips are D6.6's arm-time sweep to read. The arm asserts that too, in both
directions, so it cannot drift into pinning the opposite of D6.5.

MEASURED, three arms:

| arm | tree | result |
|---|---|---|
| the new pin, baseline | this worktree | ok 0.54s (12x ok, 4x `-race` ok) |
| the new pin, mutant planted | `if false` | FAIL, both assertions, 0.29s |
| the 264 Land/Wall/Judge tests, mutant, pin skipped | arm 2 | ok 1.03s |
| the same filter, mutant | arm 1, default TMPDIR | ok 37.3s (control ok 33.3s) |

The mutant run prints ONE ◷ line where the baseline prints two, which is the
mechanism in one reading: the second landing does not get a line because
nothing came back for it.

A PROBE ARTIFACT WORTH KEEPING. ranger-base-vcfc1's F1 repro names a TMPDIR
under `$HOME` as the cheap door for a bwrap seat-store arm, and it is — but
the same TMPDIR reds four unrelated wall tests in arm 1
(`TestQAWallSeedProjectConfig*`, `TestQAWallWitness*`) on ADR 0100 D1/D2(a):
`$HOME/tmp-*` contains the per-seat wall root, so no write grant over it
renders. Those reds are the probe's and not the tree's. Arm 1 above was
re-read under the default TMPDIR for exactly that reason, and a reader who
moves TMPDIR to the home for one arm should expect them.

## What did not change, and why

**F1 · the seat-store wide-grant arm (internal/posse/bwrapseatstorelive_test.go)
fixed itself while this bead was open.** It was filed as a line rather than a
bead because ranger-base-49rxe already carried the failure, with the part 49rxe
could not do: 49rxe said "no tree was on record green before this red, so the
window could not be bounded", and vcfc1's three arms bound it to one line of
one test — `bsWideGrantRefusedUnpinned`, ranger-base-4gyib's arm, asserting a
refusal ranger-base-dot6w had deliberately removed. Two diagnoses of one red,
eight minutes apart, that no single tree held until the launcher rebased one
onto the other. 49rxe is CLOSED now (57804f5e, "the seat store pin's wide-grant
arm reads the render's own pin") and the arm is gone from the tree. Nothing was
left to build; the bounding is recorded on ranger-base-rh8y0.

**F2, F3 and F5 are in other repos, and handoffs rather than edits.** This bead
ran in a posse worktree, where only commits from that worktree move — so a
change made from here to another repository would reach nothing. Each went out
with its full repro, its ADR 0006 §7 "what behaves differently", and a
done-when, and this page names them by id alone: ranger-base-xd2hw,
ranger-base-ovqjs, ranger-base-p4orr. The repositories they are filed against
are private and this one is public, so what each finding IS stays on its bead,
where the audience is already the right one (AGENTS.md "Landing the plane",
visibility line 4; the hold is ranger-base-zipwe).
