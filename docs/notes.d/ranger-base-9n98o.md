# A hand-zeroed quiet counter carries no caveat, and two fail-closed arms get doors (ranger-base-9n98o)

*2026-11-13 · developer · code bead, from ranger-base-obato's verification of
four closes (ranger-base-sxez7, ranger-base-mdku2, ranger-base-4jroi,
ranger-base-rgyto)*

Three findings, two files. One is a live defect with the same shape as the
defect the close that introduced it was filed to fix; two are arms that
existed in the code and in nothing else.

## 1. The clear was on one of the two paths out of `health` (a defect)

ranger-base-4jroi added `landedChecker.quietUnreset` so that D6.3's silent
bead may not call a count consecutive when it spans an ANSWERED run — the
case where the reset that run owed could not be written. The mark is set on
exactly one path, and before this bead it was cleared on exactly one path
too: inside `healthAnswered`, at `n == 0` and after a reset lands.

There is no `n == 0` on the SILENT path, and the record reaching zero without
an answered run is not hypothetical. `landedQuietRecorded` reads ANY shape it
cannot parse as zero, deliberately and for a stated reason — "this is one
line of git config a human can edit" — so an operator's `--unset`, or a
half-written record, takes the count to zero while the mark stands. The next
crossing of two silences that really were consecutive then filed a bead
saying, verbatim:

    THEY ARE NOT CONSECUTIVE, which is what this count otherwise means: a run
    between them ANSWERED, and the reset that answered run owed the counter
    could not be written (the ⚠ is in the launcher's log).

Both silences were consecutive and nothing answered between them: the judge
IS dead, and the bead told architecture to read it as unreliable. That is
ranger-base-4jroi's own defect class — a D6.3 description claiming something
that did not happen — with the sign flipped, and it was reachable only
because of the mark that close introduced. The close comment and
docs/notes.d/ranger-base-4jroi.md both state the clear covers a hand-zeroed
record ("cleared whenever a reset lands or the count reads zero"); on the
silent path neither was true.

THE FIX is the same reading `healthAnswered` already makes, on the other
path: `healthSilent` clears the mark when the count it was handed reads zero.
A count that reads zero carried nothing over anything, whichever verdict is
being counted. It costs a map delete — no git write, no bd call — which is
the same price the existing clear pays and the reason
`TestAGreenLandingSpendsNoBdCall` is undisturbed. `healthSilent` reads `n`
before it increments, so the test was already in hand.

## 2. and 3. Two live lines that no arm held

Both were found by mutating the shipped code and watching every test the
close in question claims to exercise stay green.

**`healthAnswered`'s `n == 0` clear** (landedhealth.go). Deleting it outright
was GREEN over the five-test filter ranger-base-4jroi's close lists, where
the other four mutants that close names all red. It is live code, not dead:
the answered path with `n == 0` clears a mark the silent path left standing,
which is what finding 1 is about.

**The hoisted presence check's degrade** (parity.go). ranger-base-rgyto
hoisted the `os.Lstat` above the whole-file/keyed branch — the right fix, and
well pinned for the row it MOVED: all four mutants over a dangling symlink
red, a full revert of the hoist included. What the same close carried with it
and named in its own commit message as a widening — "a non-IsNotExist Lstat
error now degrades 'classification failed: unreadable' there too, where it
used to be called clean" — had no arm. Replacing that degrade with
`return ""` — failing OPEN on a project-config entry the launch cannot
classify, in a check whose whole job is to fail closed — was green over
`-run 'TestQAProjectConfigTrust|ProjectConfig'`. The gap predates the hoist
on the keyed branch; the hoist doubled the surface it left unguarded, exactly
as ranger-base-g3pfl doubled the surface of the defect ranger-base-rgyto
fixed.

## The arms

`TestAHandZeroedQuietCounterLeavesNoUnresetCaveatBehind`
(internal/posse/landedhealth_test.go, arm 2). Two rows reaching one crossing
by two routes: a failed reset sets the mark, an operator unsets the counter,
and then either the next silence reads the zeroed record or an answered run
reads it first. Both assert the filed bead claims no pattern it cannot carry.
Each row kills the mutant on its own path and neither kills the other's,
which is what says the subject is the record reading zero rather than either
path's bookkeeping.

`TestQAProjectConfigTrustUnreadablePresenceFailsClosed`
(internal/posse/projectconfigtrust_qa_test.go, arm 2). An unsearchable parent
directory — no root, no fixture beyond a chmod, and a shape a repo can really
ship — over EVERY declared project-config entry of every runtime in the tree,
read off the runtimes rather than spelled, so a runtime that declares a new
entry tomorrow is in the arm the day it lands. 19 entries across claude,
codex, grok and bob today. Asserted per ENTRY, which is the answer the f67jx
census reads, because two entries can share a parent and `ProjectConfigTrust`
returns only the first offender in declared order; and then once at the
launch level, which is the answer the operator sees. It skips under euid 0,
where the mode bits mean nothing.

## MEASURED

2026-11-13, linux 7.2.15 x86_64, go1.27.3, this box, one change at a time,
tree restored after each (`git status` empty afterwards).

| mutant | arm | result |
|---|---|---|
| `healthSilent`'s `n == 0` clear dropped (the fix here) | hand-zero, straight-to-the-next-silence | RED |
| ” | hand-zero, through-an-answered-run | green (the answered run clears it — the distinction) |
| `healthAnswered`'s `n == 0` clear dropped | hand-zero, through-an-answered-run | RED |
| ” | hand-zero, straight-to-the-next-silence | green (same distinction, other way) |
| ” | every other D6.3 arm, including ranger-base-4jroi's two | green — this is finding 2 |
| hoisted presence degrade → `return ""` | fail-closed, all 19 entries | RED |
| ” | `-run 'TestQAProjectConfigTrust\|ProjectConfig'` without this arm | green — this is finding 3 |

The mutation battery is scratchpad-only (AGENTS.md: a battery that mutates
goes in a script file and is run by path); what survives is the table.

## What is not here

`TestASilenceCountedThroughAFailedResetClaimsNoPatternAndNoRestart` and
`TestASilenceAfterALauncherRestartSaysTheEarlierSentenceIsHeldElsewhere` are
untouched. The caveat they pin is right and this bead does not weaken it: a
count that came through a failed reset still says so. What changed is that a
count that came through nothing at all no longer does.
