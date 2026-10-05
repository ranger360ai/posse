# A pin on a helper is not a pin on its call sites (ranger-base-cu0zg)

Four closes verified: ranger-base-hrf47, ranger-base-yrag8, ranger-base-76gc4,
ranger-base-r5546. Three verified clean; one has an unpinned half. The reason
the half is unpinned is general enough to be worth writing down, and it is not
a quality problem with the pins that exist — every one of them kills what it
was written to kill.

MEASURED 2026-10-04, this box (darwin 25.4.0, go1.26.5), at main = 2c328504 and
re-read at 65eade40.

## The shape: the helper was extracted FOR the pin, so the pin aimed there

ranger-base-76gc4 put a whole-screen pane capture on four D3 consequence sites.
Two are the hand path (`AwaitPromptable`'s two refusals, which share one
`logUnrecognized` call); two are dispatch's own (`awaitDelivered`'s hold,
`awaitSettled`'s refusal, which share `(*Dispatcher).d3Evidence`).

`d3Evidence` exists for a good reason: a Go argument is evaluated before the
call that would discard it, so wrapping the evidence inline at the two dispatch
sites would make every `--dry-run` hold and refusal fork a `pane read` for a
record `logReading` then refuses to write. Extracting it carries `logReading`'s
dry-run rule one step earlier.

And that extraction is exactly why the pin went to the helper instead of the
sites. Arm 5 of the close's five pins calls `d.d3Evidence` directly — its own
head comment says "Pinned on d3Evidence directly" — because that is where the
dry-run rule now lives. The helper's behaviour is pinned four ways. Its two
call sites are pinned no ways.

Three mutants, all survivors:

| mutant | result |
| --- | --- |
| `dispatch.go:5410` `d.d3Evidence(target, lastGuess)` -> `ReadingEvidenceOf(lastGuess)` | arm 3 `^TestQAD3` ok 1.202s, five pins GREEN |
| `dispatch.go:5625`, same substitution | ok 1.041s, five pins GREEN |
| BOTH at once — `grep -c d3Evidence dispatch.go` answers 0 | arm 1 ok 8.024s, arm 2 ok 16.294s, arm 3 ok 8.394s over 'Reading\|Unrecognized\|Promptable\|Delivered\|Settled\|D3\|Reported\|Herdr' (55 tests in arm 1). Nothing in the tree reds. |

The four mutants the close itself claimed all kill, and for the right reason —
`withPaneCapture` returning `ev` unchanged reds arm 1 on the missing region;
dropping `--source detection` reds arm 1 on the argv; dropping the `Reported`
guard reds arm 2; dropping `d3Evidence`'s `DryRun` return reds arm 5. So this
is not a weak pin set. It is a pin set aimed one level below the deliverable.

**The generalisation.** When a close extracts a helper so that a rule can be
pinned in one place, the extraction moves the pin's SUBJECT from "what happens
at these N sites" to "what this helper does". Those are the same claim only
while every site calls the helper, and nothing was asserting that. Ask of any
close that adds a helper with more than one caller: which mutant deletes a
CALLER, and what reds? Reverting a call site is the cheapest mutant there is
and it is the one a helper-shaped diff makes easy to forget.

Filed as ranger-base-h9925 with the repro and two shapes of failing test (one
arm per consequence, or a source pin over `dispatch.go` holding every
`DecisionUnknownScreen` record literal to taking its `Herdr` from
`d3Evidence`).

## What the other three closes cost to break

Recorded here because a verification nobody can re-run is an opinion.

**ranger-base-hrf47** — the deliverable is a DOOR, so the decisive arm is the
pair, not the single mutant. Adding a `verify-holdenprobe:` target to the
Makefile with no `scripts/verify-box.sh` row reds `make scripts-check` in 0.59s
naming the target; removing `TestQABoxCheckCensusCoversEveryVerifyTarget` from
`QA_SCRIPTS_PINS` and leaving the same mutant planted makes `make scripts-check`
answer **ok 0.333s**. That green is the pre-hrf47 world, and it is what the
bead was filed about: the red existed, and the only thing that could see it was
an unfiltered 589.965s package run. Both halves of the two-way check also kill
— the `twdDoorHolders` row deleted reds `TestQAEveryTreeWidePinHasADoor` on
"named by a door and no longer derived as tree-wide", the door variable
emptied reds it on "registers ... and no Makefile door names it".

**ranger-base-yrag8** — the replay is a true merge, not a re-commit. Author
date (2026-10-04 20:41:57 -0400), subject and all five paths preserved from
71ce45e9; `git log --grep ranger-base-hrf47` finds e0f1274c on main; every
difference between the two commits is nnnf1's content being folded in plus the
merged count. The arithmetic the bead is about is pinned: the head comment's
`fifty-three` edited back to `fifty-two` reds arm 4 naming the Makefile's own
count. And the red "neither bead's suite could report" reproduces — restoring
the dangling `docs/notes.d` path into hrf47's fragment reds
`TestQAEveryNotesFragmentCitationResolves` through `make doc-check` at the
fragment's line.

**ranger-base-r5546** — all four findings' mutants kill, including the three
that were survivors on ranger-base-qr0as. The census repro answers
`0 replayable · 1 truncated · 0 redacted` where main's pre-fix script answered
`1 replayable · 0 redacted`; each of the three census clauses reds on its own;
each of the three never-under-home bounds reds on its own INCLUDING the state
dir, which by outcome alone the home row would have covered — the reason
assertion is what reaches it; the README door table is held on membership and
order (a row dropped, two rows swapped, a row renamed and a bogus fifteenth row
all red arm 4). F4's plant is a real gate and not a sticker: pointed at a bd
line that is never logged, the pass fatals on "the lever never fired", and the
pin ran 10/10 at a one-minute load average of 21.5 — heavier than the run that
reddened it.

Its work is NOT on main (eb130781, pinned at
`refs/posse/merge-blocked/posse/gwart-posse-ranger-base-r5546`), so it was
verified on the merge: main's code plus r5546's four non-conflicting paths runs
green, and the one real conflict — `twdDoorHolders`, where hrf47 added a third
row and r5546 rewrote the arm-4 row's reason — resolves by taking both. That
merged tree passes all fourteen `make tree-check` doors, exit 0, 82.5s at load
12.5. ranger-base-o6ka5 is the bead for that landing and gwart holds it.

## The limit of this verification, stated

No full `make test`. Both suite slots were held by sibling seats for the whole
session (`scripts/suite-lock.sh --status`: dinesh-posse-ranger-base-zt45t and
gwart-posse-ranger-base-o6ka5) and the one-minute load average reached 115.6, so
a third queued suite would have cost the box more than it could have told me:
the three landed closes were each run whole by their closer, and all fourteen
tree-check doors are green at 65eade40. What a full run would add over the
focused runs above is a cross-package interaction, and the one class of those
this round could produce — two beads green apart and red together — is the
thing ranger-base-yrag8 already found and pinned, re-measured here.
