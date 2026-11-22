# The trail's silence is a line, not a word list (ranger-base-q28yf)

verify-after's "built nothing" exemption reads two halves — a close reason
carrying one of scorecard.go's `rejectWords`, AND no commit naming the bead
— and the posse release triage's templated close reason carries none of
those words:

    Closed in the posse release triage (operator approval 2026-11-21):
    polish that does not change what posse does; not release scope. <class>.

So the exemption misses it on its vocabulary half even where its structured
half is satisfied outright, and a verify bead is filed for a close that
landed nothing. Found by the verify of ranger-base-3na17; escaped from the
closes of ranger-base-nq6bv, -hb08p and -zuth8 and nine more like them.

## What was measured

MEASURED 2026-11-22, this box, live store, `bd list --all --status closed
--limit 100000 --json` (4,179 closed beads, up from the 3,984 the finding
read two days earlier):

- **20** closes carry the triage template; **0 of 20** match `rejectRe`.
  Two more mention the triage without the template — one is a hand-written
  variant, one is ranger-base-3na17's own verify close — and neither is a
  thing a word list should try to reach.
- Over the newest **150** `ranger-base-*` closes carrying a `code` or
  `devops` label (the `verify_labels:` default, so this is the population
  verify-after actually sections): **34 (23%)** have no commit naming them
  (`git log --grep <id>`, exit 0, no match). **7** of those 34 are
  triage-templated.

That last pair is the whole decision. The class a word list can reach is a
fifth of the class the commit trail already reads.

## The three shapes, priced

**(a) add the template's phrase to `rejectWords`.** Refused on evidence.
Of the four triage closes ranger-base-3na17 verified, ONE had a wrong stated
class — ranger-base-nq6bv, filed as "Stale comment or test pin", in fact a
behavioural gap in the landed-check judge's bead output — and the only thing
that caught it was the session this would have suppressed. The two callers of
the one vocabulary are not symmetric: the scorecard can afford an over-match
(a cell is off by one), while here an over-match suppresses a control and the
watermark advances past it unrecoverably. It would also file a release-scope
deferral into the scorecard's `Rejected` column, which is a different claim
about the filer than the triage made. And it reaches 7 of 34.

**(b) have the triage write a structured marker.** The same suppression, made
durable instead of fragile. The objection to (a) is not that its signal is
prose, so making the signal robust does not answer it. It also asks a change
of a process this repo does not own: the triage is the operator's, run by
hand, and nothing in this tree produces that string.

**(c) leave it, and accept one verify batch per four triage closes.** Honest,
zero code, and the floor any fix had to beat. `verify_batch:` already damps
the amplifier and the session is demonstrably not valueless.

**What landed beats (c) by one line and no new signal.** The defect is not
that the bead is filed — it is what the bead is filed AS. The section's only
word about a close that landed nothing was the ABSENCE of a commit block,
beside an acceptance pointer (ADR 0006 §4) aimed at the closed bead's own
description, i.e. at work that may never have been done. ranger-base-3na17
had to derive its own question from that, out loud, in its close reason:
"these are deferrals and the verifiable claim was the triage's." That
derivation is now the line, so the next verifier starts where this one
finished.

## What the line may not say

Not "built nothing". 23% is far too many for the empty trail to be one
thing: this shop's own `git log --grep <id>` promise is scoped (ADR 0022 —
unconditional in a session worktree, in the shared checkout only for a file
with one in-flight writer), so a commit that omitted the id renders here
identically to a close that shipped nothing. The line therefore reports the
trail's SILENCE, names the readings, tells the verifier to settle which
first, and says what each one makes verifiable — the close_reason's own
claim where nothing was built, the §4 pointer as usual where something was.
One git call, three answers, and `err != nil` stays a silence for
`gitCommitsFor`'s own reason: "git said no" and "git could not say" cannot
be read alike.

### And it has to name THREE (ranger-base-m0ajs)

This line shipped with two readings and the class has a third: the work
landed in ANOTHER repo of this instance's crew, under a commit that DOES
name the bead, in a checkout this trail never reads. Found by the verify of
this close, which was itself a batch of four closes containing one
(ranger-base-jttvs, commit eda8427 in ranger-core, PR 128).

MEASURED 2026-11-22, this box, same population eight hours on (the newest
150 `ranger-base-*` closes carrying a `code`/`devops` label): **27** have no
commit naming them in posse — the window moved, the share did not — and **3
of those 27 (11%)** are named by a ranger-core commit: ranger-base-jttvs
(eda8427), -oqrat (29adec1), -2qlz2 (3013107). The two-reading line was
rendered for all three.

Why it was worth a line and not a RECORDED note: of the three settling
sources the line named, only `bd comments <id>` answers here. The branch
records are a block `gitBranchLandingFor` renders for a branch cut in THIS
repo for this bead, and ~/src/posse held 11 `branch.<b>.posseBead` records
naming none of that batch, so the pointer was printing for a block that was
not there. And the half the line spent most words on — "Nothing built: the
close_reason is then the whole claim" — sends the verifier to judge prose
when there is a commit, a diff and a test run to read. So the fix is the
third reading, the settling sources ordered by what they can answer, and
the instruction to go verify in that tree; the pin grew four rows (the
count, the third reading, the `bd comments` pointer and that instruction),
each of them mutation-checked red.

## Pins

- `TestVerifySectionReadsAnAnsweredEmptyCommitTrailAsALineNotASilence`
  (verifyafter_test.go, arm 3) — three arms, which are the three answers one
  git call can give. Mutation-checked 2026-11-22, each mutant red in exactly
  the arm claimed: the guard removed (`case len(lines) == 0 || true`) reds
  "a commit names it"; a reading dropped from the line reds "git answers and
  names nothing" naming the reading; the error arm folded in
  (`case err != nil && false`) reds "git cannot answer".
- `TestIsRejectedCloseMatchesWordsNotSubstrings` (scorecard_test.go, arm 2)
  gained the two rows that make this boundary CHOSEN rather than unmeasured.
  Every false row in that table described a FIX, so nothing in it said what
  the vocabulary does with a close that was neither fixed nor rejected. The
  triage template reads false; the same deferral written by hand ("won't fix
  — not release scope") reads TRUE on "won't fix" alone, and that row is
  kept because it is what the refusal costs — the vocabulary cannot tell a
  deferral from a rejection and is no longer being asked to.
