# ranger-base-rq986 — a merge-back that was overtaken while its own suite ran

Landed 2026-11-01, git 2.50.1 / macOS 25.6.0, go 1.26.10.

`ranger-base-56eln` closed with one commit on
`posse/dinesh-posse-ranger-base-56eln` (`e27b7618`) and `main` moved on under
it, so the launcher filed this block. The replay worked, the suite went
green, and the work still did not land — because by then `main` had landed
another lane's bead doing the same job better. **This entry is about that, not
about the replay.** The replay itself is `docs/notes.d/ranger-base-xea2y.md`
and it held without amendment for the fifth time.

## The sequence, with the times, because the times are the finding

| time | what happened |
| --- | --- |
| 22:58 | `56eln` commits on its branch (`e27b7618`), parent `975d0d82` |
| 23:12 | this bead dispatched; `main` is `13bce8c3`, three commits ahead |
| 23:12-23:40 | replay, four conflicts resolved, `make tree-check` ×3 green |
| 23:54 | **`ranger-base-g31e0` lands on `main` (`5620d3b9`)** — same subject |
| 23:40-00:50 | `make test` red, then fixed, then green. Branch committed |
| 00:52 | `git merge-base --is-ancestor main HEAD` says NO. `main` is four ahead |

Everything between 23:12 and 00:50 was done against a `main` that had stopped
existing at 23:54, and nothing in the session said so. `bd show` on this bead
was read at dispatch. The block's own text warns that a block outlives the
branch it is about and gives a command for it — but the branch was fine; it
was `main` that moved, and there is no equivalent check prescribed for that.

**The check that caught it was incidental.** It was the last line of a
verification sweep, `git merge-base --is-ancestor main HEAD`, typed to confirm
the commit was fast-forwardable. It failed, and only then did `git log
HEAD..main` name the four commits. Had that line not been typed, this bead
would have closed green and the launcher would have filed `rq987` against the
same code.

**So: on a merge-back, re-read `main` after the suite, not only before it.**
A full `make test` here is 20-25 minutes and `make tree-check` ×3 is another
three; this box lands a commit on `main` every few minutes at a busy hour. The
cheap form is `git -C <the repo> rev-parse main` before the commit and
`git merge-base --is-ancestor main HEAD` after it. The second is the one that
cannot be fooled, and it costs nothing.

## What `main` had landed, and why it wins

`ranger-base-g31e0` (jian-yang, `5620d3b9`, 23:54) is the findings bundle for
the same verify bead — `ranger-base-tw3t7` — that produced the witness table
`56eln` tripped over. It does everything `56eln` does and two things more:

- **both halves of the gap.** `56eln` widened the climb fence to read a range
  variable. `g31e0` did that too (`twdClimbSet` is two maps now, val/list,
  with the jurisdiction line held per ELEMENT) **and** fixed the enumeration
  seed, which voided a whole list on the one element it could not read and so
  discarded the readable `"."` beside a climb.
- **per-site remediation decided by measurement rather than by rule.** With
  both halves fixed, two of the three pins are derived through their readable
  `"."` and need no root change at all; only `visibility_test.go`, whose every
  element climbs, takes `qibRepoRoot`. `56eln` had routed all three through
  `qibRepoRoot` on principle.
- **no new door.** `56eln` added an eighteenth, `make source-check` with
  `$(QA_SOURCE_PINS)`. `g31e0` put all three pins into `$(QA_PKGCENSUS_PINS)`
  with the widened radius said in the Makefile, and seventeen doors stands.
- **a stronger witness conversion.** `56eln`'s landing required flipping the
  four `hole` rows' `wantClimb`. `g31e0` converted them from `hole` to `filed`
  and converted the floor with them: it required four holes, it now requires
  four filed rows AND that every one be a YES from a rule, so a revert cannot
  be made green by flipping a want back.

Both arrive at **one hundred and twelve pins**; they differ on the doors,
seventeen against eighteen.

**So the replayed commit was not landed.** Landing it would have re-added a
door the house had just measured its way out of, reverted a per-site decision
to a rule-based one, and replaced a converted floor with a flipped want — all
under a bead whose job was to carry `56eln` across, not to re-open it. The
branch was reset to `main` and the replay discarded. It survives as
`17a69856` for the session and in this entry.

## What of 56eln was NOT on main, and is now

One thing, and it is the half of `56eln`'s finding that `g31e0`'s subject
never covered. `g31e0` is about DERIVATION — which pins the register sees and
which doors they get. `56eln`'s second finding is about the censuses
themselves:

> AND TWO OF THE THREE WERE GREEN OVER NOTHING, live whatever was decided
> about a door: both discarded their `filepath.Walk` error, answered nil on
> the callback's own, and floored no count.

That was still true on `main` at `5620d3b9`, in both
`TestQAKeychainStoreIsWiredToTheAbsoluteBinary` and
`TestVisibilityOverrideIsNeverDispatched` — the second one even after `g31e0`
gave it a correct root, because a correct root and a checked walk are
different repairs. A door makes a census REACHABLE; it does not make it
HONEST. The third,
`TestExactModelHasNoDeclarationSurface`, already had both (`t.Fatal` on
`os.ReadDir` and a `found != 1` floor) and was left alone.

Landed here, in the shape `g31e0`'s decisions leave room for — **the roots are
not touched**, only the error handling and the floor:

- `filepath.Walk`'s return value is captured and fatals, naming the root.
- the callback returns its own `werr` and its `os.ReadFile` error instead of
  `nil`.
- a `goFiles` count with a floor of 50, the shape
  `TestKeychainReadIsExecdFromExactlyOneFile` has carried all along.

MUTATION-CHECKED both ways, 2026-11-01: with `cmd/` renamed, both censuses
fail at `walk ...: no such file or directory — this census is green over
whatever it fails to read`, where before this change they passed in 0.01s over
zero files; with the floor raised to 200, both fail naming **159 non-test .go
files**, which is `56eln`'s own measured number and the population on this
day.

## The three conflicts of the discarded replay, kept because they recur

Worth keeping even though the commit did not land, because the next merge-back
of this class meets them again.

**Four files conflicted, where every earlier member of this class had one or
two.** `main`'s `na4rm` added three doors and `56eln` added one, both after
`execwrite-check`, so every ORDERED list of the doors collided at the same
line: `.PHONY` and `tree-check:` in the Makefile, the table in
`internal/treepins/README.md` and the block in `AGENTS.md` (both held in order
by arm 4's `slices.Equal`), and `twdDoors`. `slices.Equal` means the
resolution must pick ONE insertion point and use it in all four places, and
nothing in the tree says which. **The Makefile's own variable block is the
tiebreak** — it is the one list of the four git resolved by itself. Had it
conflicted too there would be no tiebreak in the tree, and the right move
would be to say so on the bead rather than invent one.

**The count sentence, the fifth time.** `h1q5o`'s two prose traps were both
live and both avoided by its own instructions: the `<n> pins and <n> doors`
skeleton matches a second time if the resolution writes "the class has grown
by N pins and M doors", so that fact goes in the other order; and `h1q5o`'s
"the largest class this command has ever run" went stale the moment a larger
reading was prepended.

**And one conflict with no text in it at all, which only the full suite
found.** `main`'s `tw3t7` added `internal/treepins/climbidentholes_qa_test.go`
— a witness whose four `hole` rows assert that neither half of the derivation
sees these shapes — and `56eln` closed the fence half of exactly that gap.
One subject, zero shared lines: `merge-tree` had nothing to report, every door
was green on both sides, and the first thing that said the two beads had met
was four subtests failing in the arm-1 run. No door could have caught it and
that is not a gap in the register: that file plants fixtures and reads no
tree, so it has no door by `tw3t7`'s own argument, and the ~590s package run
is the only route. **A witness that ships green asserting what a rule reads
today is a tripwire across every branch in flight, and it fires in whichever
one lands second.** That is the idiom working. What it means in practice: when
a merge-back's two sides are about the same rule, a green `make tree-check`
says nothing and the full suite is the only reading — which is the same
conclusion the overtaking above reaches from the other end.

## Verified

`make fmt-check`; the three censuses under `-run` after each edit; the two
mutations above; `make tree-check`; `go run ./cmd/checkorphans` clean after
each backgrounded suite; and `make test` whole, green at MAKE_EXIT=0.

Of the discarded replay: `make tree-check` three times (65s, 64s and 59s at
one hundred and twelve pins and eighteen doors, load 8.96/5.53/6.82, one
sibling suite slot held throughout — not carried into the head comment,
because that class is not what landed), and `make test` twice, red at the
witness and then green.
