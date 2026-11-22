# ranger-base-dudu4 — a census for the wall member that reaches no nest gate

A live pin in `internal/posse` that execs a bwrap, sits inside ADR 0097 D3's
`-run` filter by name, and reaches `bwrapNestSkip` through no route at all was
caught by no census. ranger-base-mklu0 found the instance by hand — three pins
in `wallunixlive_test.go` — and this bead is the reading that makes the class
visible.

**What landed here is the census and only the census.** This bead also wrote
its own fix for mklu0's instance, against a tree where that fix had not landed
anywhere; mklu0's landed on `main` first, and the two are the same fix under
different helper names, so the merge-back (ranger-base-aojnj) took main's and
replayed the census onto it. The last section records that and the readings
that confirmed it, because a fragment describing a helper split nobody can
`grep` for is worse than no fragment.

Membership is read off the MARKER and never off the name (ADR 0097 D4), and
the marker's only emitter is the gate. So an ungated member is a pin the venue
counts as nothing: a red in it prints as `note: FAIL <test>` under
`wall check: green, N pins measured` at EXIT 0 and never reaches ADR 0088 D5's
ratchet; a run in which it goes dark is counted by neither D5 arm; and on a
caged seat it FAILs where every gated member SKIPs, which ADR 0096 D3 forbids
outright.

## The depth is measured, not chosen

The rule is "a member that launches a bwrap seat reaches the gate", and the one
number in it is how far "reaches" goes. TWO readings, both MEASURED 2026-11-22
(linux 7.2.15-1-omarchy, go1.27.3, 16 cores), over 767 files and 57 members, by
the census in `internal/treepins/wallnestgate_qa_test.go` run at each depth in
turn — one on the tree the depth was CHOSEN against and one on the tree the
census LANDS on. Those two bases are seven landings apart, not one, so the
comparison is only worth making because every count the census reads is
IDENTICAL across them — 767 files, 24 member files, 57 members, 45 launching,
54 gated — and the single thing that moved is what mklu0's fix moved. That the
rest of those seven touched none of these numbers is itself the reading:
this member set is stable, and a change in it means something.

On `5d11a45f`, the landing base, with that fix in:

| depth | ungated | launching | what the reading says |
|---|---|---|---|
| 1 | 6 | 45 | **all six are FALSE**: `bwrapseatstorelive_test.go` and `storerepogitwalllive_test.go` reach the gate at TWO hops, through `bsLiveFixture`/`srLiveFixture` to `bgLiveSkip` |
| 2 | 0 | 45 | the rule is satisfied by the tree's own spellings |
| 3 | 0 | 45 | identical to depth 2, so the rule is not perched on its own edge |
| 4 | 0 | **54** | the reach has blown out into the package: the launch count goes 45 → 54, so the nine gated members that launch nothing all read as launchers and "launches a seat" has stopped meaning anything |

On `d5527330`, before it — the reading that found the instance:

| depth | ungated | what the reading says |
|---|---|---|
| 1 | 9 | 6 of the 9 are FALSE, the same two files |
| 2 | 3 | `wallunixlive_test.go`'s three — the instance ranger-base-mklu0 found by hand, and nothing else |
| 3 | 3 | identical to depth 2 |
| 4 | 3 | the same blow-out in the launch count |

Depth 2 is therefore the smallest depth the tree's own spellings need, with
depth 3 agreeing and depth 4 measurably over-reading — on either tree, which is
the part worth having two readings for. The census holds both edges
hermetically rather than only logging the number: a planted gate at hop 2 is
reached at depth 2 and not at depth 1, and a planted gate at hop 3 is reached
at neither. That hermetic arm is now the whole of what keeps the depth honest,
because on the landing base the tree arm finds nothing at any depth — a count
of zero cannot distinguish a budget of 2 from a budget of 9.

## The three deliberate non-members need no exemption table

ADR 0098 D4's rule is that a hand-written exemption is a row that drifts, and
this census has none. The three members that emit no marker on purpose read as
non-LAUNCHES, which is a property of their source and not a judgement anyone
has to maintain:

- `TestLiveWallWitnessProbeDoesNotWriteTheLiveWall` and
  `TestLiveWallWitnessHoldsUnderTheRenderedWall` — they RENDER a wall rather
  than exec one, so no `exec.Command` in their reach names bwrap.
- `TestLiveWallUnixStepReArmsAClearedDieWithParent` — its launcher is a plain
  `sh -c '"$@"; :'`, which its own comment explains (what AppArmor's
  transition leaves the step is a task with pdeath_signal 0, and a plain
  fork+exec is that).

That third one is why `wallunixlive_test.go` has two signal gates. As the tree
holds it (ranger-base-mklu0's landing, which is the one on `main`), its sibling
`TestLiveWallUnixScopesSignalsToTheSeat` execs a bwrap and takes the gated
`wuSigLiveSkip`; the pdeathsig pin takes `wuSigStepSkip`, which is the same
floors WITHOUT the nest gate, because a caged seat can measure it exactly as an
uncaged one does and gating it would cost that reading and buy nothing.

## Where the gate sits

The fix for the instance is mklu0's and is on `main`; this bead's independent
fix for the same three pins is not, and the last section says why. As the tree
holds it:

| pin | gate | before |
|---|---|---|
| `TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched` | `wuLiveSkip` | FAIL on a caged seat, no marker |
| `TestLiveWallUnixLeavesCrossDirectoryRename` | `wuLiveSkip` | the same |
| `TestLiveWallUnixScopesSignalsToTheSeat` | `wuSigLiveSkip` | the same |
| `TestLiveWallUnixStepReArmsAClearedDieWithParent` | `wuSigStepSkip` | ungated before and ungated after, deliberately — `scripts/posse-wall-check` names it as one of the three non-members |

The signal half's shared floors are `wuSigHostSkip` (`runtime.GOOS` and
`LookPath("bwrap")`) and `wuSigABISkip` (the `SCOPE_SIGNAL` floor), split
exactly so that `bwrapNestSkip` can sit BETWEEN them for the one caller that
needs it. The gate goes before the ABI floor for mklu0's reason: the gate logs
the marker the moment it lets a pin proceed, so a skip AFTER it is a counted
abstention — exit 3, which ADR 0097 D5's watchdog files as "wall check silent"
— and a skip BEFORE it is a pin the venue never knew it had, which is a smaller
N with nothing to compare it to.

What the census asserts about all four, on the landing base, is the one thing
either fix had to buy: 0 members launch a bwrap seat and reach the gate from
nowhere.

## What this census does not grade

**Order**, and **placement within a member**. The tree has no one order to
read: `bgLiveSkip` puts `LookPath("git")` after the gate, `bwraptmp_qa_test.go`
and `sshidentity_qa_test.go` put the gate at the call site after their own
floors, and every pin's floors are its own judgement about what its body
needs. Grading order is a second rule and needs its own argument (dudu4's
judgement 2, undecided there and here). A gate inside one `t.Run` arm and not
its siblings reads as gated.

**The runtime count** — the other half, and it is closed now, by someone else.
This census closes the SOURCE half of the class: a gate deleted from a
launching member reds. It cannot see a pin that reaches the gate in source and
never gets there at runtime — one that skips for its own reason first. When
this bead ran, `scripts/posse-wall-check` carried no floor on N, so that pin
was a smaller N and a green run; this fragment filed it as ranger-base-c75wa
with three options priced (derive the floor, ratchet on the last green N, or
floor the abstention/measured split).

**c75wa took the first of those three and landed it** (`82b4adc2`, on `main`
before this census): the check derives a floor for n from its own run, and a
selected pin that prints no marker is exit 3 rather than a smaller green N. So
the class is now held at both ends — source here, runtime there — and the two
should stay apart rather than be merged into one pin: this file reads SOURCE
and cannot see a runtime skip, c75wa's reads a RUN and cannot see a deleted
gate. A reader who collapses them will trust whichever is greener.

**The one blind spot in the key**: the gate is read as a CALL NAME, so a gate
handed to a helper as a function VALUE is invisible. That is a row in the
hermetic arm rather than a silence — the fix for it is to name the call, which
every gate in this file's tree does today (`wuSigLiveSkip` calls
`bwrapNestSkip` by name, one hop from its member).

## Where it lives

`internal/treepins/wallnestgate_qa_test.go`, which puts it inside
`make treepins-check`'s complement door — this package minus the priced table
in `treewidedoor_qa_test.go`'s `twdTreepinsExcluded` — so it needs no
hand-written Makefile entry and no register row, and cannot go undoored
(ADR 0098 D1/D4), nothing added to the Makefile and no register row.
`make tree-check` green on the landing base in 152s (MEASURED 2026-11-22,
`5d11a45f`, one-minute load average 13.3 rising to 17.3, one suite slot held by
a sibling seat), with `make treepins-check` timed separately at 121s at a load
of 30.6.

Three readings of the same door across this bead's two bases, because the
spread is worth more than any one of them: 119.5s (`d5527330`, the branch's own
base), 128s with `treepins-check` at 83.5s (`a50aa688`, load 13.6 falling to
9.1, both slots held), and the 152s above. The door's own content barely moved
between them — what moved is the box, which is what every reading of this
command in AGENTS.md also says. Price it at two minutes on a loaded box and
treat anything past that as the load and not the census.

It is a separate file from `wallmemberaccount_qa_test.go`, whose subject is a
member's one LINE on a cause, and it reuses that file's member-file set
(`wmaMemberFiles`, off the check's own `member_pattern`) and launch key
(`wmaSeatLaunches`, the key ranger-base-hhps5 widened) rather than spelling
either a second time. Two spellings of "what a seat launch looks like" would be
two answers about one member set, which is ranger-base-8752f's lesson one
census over.

Census as of this landing: 767 files parsed, 24 member files, 57 members, 45
launching, 54 gated, 0 launching and ungated. The arm logs that line on every
run, so the next reader needs no hand count.

## The merge-back, and why only the census landed

ranger-base-aojnj. This bead's branch was cut from `d5527330` and wrote two
things: the census, and its own fix for mklu0's instance in
`internal/posse/wallunixlive_test.go`. By the time it closed, `main` had moved
on and mklu0's fix for the SAME three pins was one of the landings, so the
replay conflicted in that one file and in nothing else.

**The two fixes are the same fix.** Same gate, same three pins gated, same
fourth pin deliberately left ungated, and `bwrapNestSkip` before the ABI floor
in both for the same stated reason. They differ only in how the shared floors
are factored and therefore in what the helpers are called:

| | this bead's branch | mklu0, on `main` |
|---|---|---|
| host floors | `wuHostSkip`, shared by both halves | `wuSigHostSkip`, signal half only; `wuLiveSkip` inlines its own |
| gated signal gate | `wuSigSeatLiveSkip` (new) | `wuSigLiveSkip` |
| ungated signal gate | `wuSigLiveSkip` | `wuSigStepSkip` (new) |

So the resolution took main's file whole — `git cat-file blob HEAD:<path>` over
the merged tree's conflicted copy — and replayed the census, the notes index
row and this fragment on top. Nothing of the fix was dropped, because nothing
of it was missing; what was dropped is a second spelling of it, which is the
only thing a refactor-shaped tie-break can honestly drop when neither side is
wrong and one of them has landed.

**The census is why this is checkable rather than asserted.** It does not read
helper names — membership comes off the check's own `member_pattern` and the
gate off a call name — so it grades mklu0's factoring exactly as it graded this
bead's, and 0 ungated on `5d11a45f` is the reading that says the fix on `main`
is the fix this bead wrote. Had they NOT been equivalent, the census would have
reddened on the landing base and said which pin. That is the argument for a
census over a hand audit, arriving one bead earlier than expected.

### The base moved twice, and the second time was a visibility hold

Worth recording, because it is the part that was not about this bead's subject
at all. The first replay was prepared on `a50aa688`, which carried
ranger-base-repzh's two commits. monica's push gate then HELD those two — see
ranger-base-8h17w — and local `main` was reset off them to `bd7891ac`, with the
commits parked at `refs/heads/parked/repzh-a50aa688`.

The reason the hold exists is on that bead and stays there, per AGENTS.md's
"a finding in ANOTHER repository is named in this one by bead id and nothing
else" (ranger-base-uymh6, landed at `3dbc8cd7` — two commits below this one on
the very base this fragment is written against). The first draft of this
paragraph described the held text and named its repo, which is exactly the
class the hold was raised for; it reached neither `main` nor the gate, but it
was written, so the rule's own measurement — four fragments in 24 hours — is
not a story about other people.

**That left this branch as a re-entry vector**, which is ranger-base-zlvyk's
whole subject: a branch already cut from the held tip still reaches the held
commits, so the launcher's fast-forward at close would have put them back on
`main` under THIS bead's id — and the bead comment on 8h17w records that
happening once already through the closed repzh seat's branch. So the branch was
re-based onto local `main` (by then `5d11a45f`, five further landings on) and
the held commits are unreachable from it; `git merge-base --is-ancestor` says so
for both, and the parked ref is what means nothing was lost.

Three things are worth keeping from how that was done:

- **There was nothing to rebase.** The replay had not been committed yet, so
  moving the base was `git reset --hard main` over a saved copy of two new
  files plus a regenerated index row — no rebase, no cherry-pick, no sequencer
  marker to leak (AGENTS.md, "a `cherry-pick`/`revert`/`rebase` that exits 0
  has not necessarily ended"), and no history rewritten. The cheapest moment to
  re-base is before the commit exists.
- **Local `main`, not `origin/main`.** 8h17w says to rebase onto `origin/main`,
  and by the time this seat read it `origin/main` was `82b4adc2` while local
  `main` was `5d11a45f`. The launcher fast-forwards onto the repo's branch, so
  local `main` is the base that makes the landing a fast-forward; it also
  already contained the drop. Basing on the remote ref would have been a base
  two landings stale for no gain.
- **The index row is generated, never hand-merged.** `python3
  scripts/notes-index.py` over the new base added exactly this fragment's row
  and did not re-add repzh's, which a hand-carried diff hunk would have.
