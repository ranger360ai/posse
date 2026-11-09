# ADR 0006 — Handoffs carry explicit acceptance; verification stays batched

*Status: accepted; simplified 2026-09-05 by operator ruling · inferred-intent removal landed 2026-09-06 (ranger-base-0ezn7) · escape doctrine amended 2026-09-06 by operator ruling (§1, §6; ranger-base-0f2zy) · pin write-freeze added 2026-09-10 by operator ruling (§7; ranger-base-7dh59) · verify-chain depth bound added 2026-11-09 (§3; ranger-base-ewnas) · owner: architect.*

## Context

Doing nothing guesses a bead's intent from labels/type and a closer's PID.
Beads carry no intent field (0001). Keep the measured batching correction,
deduplication and landing evidence; verification uses the task's stated
acceptance. Missing criteria must be visible, not silently invented.

## Decision

**1. Keep three handoff channels.** Progress and decisions about this bead
are comments on it. Work for someone is a new bead, lane label and
`discovered-from` provenance, with blocking dependencies where order matters.
Conversation is not a dispatch channel. A recorded finding (§6) is not work
and files nothing. File unassigned unless the task
needs a specific person's session tree, own close, own memory, exclusive
ruling or uniquely bound skill; name the reason in the description's first
line. Selection at launch belongs to 0011 §4. Cross-repo handoffs go to
the work's store, with origin cited; do not manufacture cross-store edges.

**2. Preserve the four shapes.** A committed design hands build slices to
the code lane with the ADR and explicit done-when; its design bead closes
when those slices exist. A divergence is commented on the build and handed
to architecture if it changes the design. A closed configured code/devops
bead enters verify-after below. A security finding is severity-prioritized
work for that lane; P0/P1 findings also name `SECURITY:` on the origin.
Grooming remains an operator-scheduled bead, not a harness scheduler.
PID Handoffs rows say who/lane, label and required content.

**3. Verify-after keeps its current queue guarantees.** On each pass and
`posse ready`, scan configured `verify_labels` closes under the launcher
lock. Empty labels disable the feature; dry-run files nothing. Use the
existing per-repo watermark, per-close section markers and discovered-from
evidence to deduplicate, including an uncertain create that actually landed.
Quote/flatten externally supplied fields so a title or description cannot
forge another close's dedupe marker. Do not change existing marker syntax.

`verify_batch` (default 1) groups closes; `verify_batch_age` (default 24h)
flushes the tail only when its oldest close reaches the limit. Hold the
watermark behind pending closes. A batch has one section and provenance
edge per close, one trailer, most-urgent priority, and a filed comment back
on each close. Respect `verify_assignee` only as an operator pin. Exempt a
whole-word rejected close (`invalid`, `duplicate`/`dup`, `wontfix`/`won't
fix`, `not a bug`, including the existing plural forms) only when a
successful git query finds no commit naming it; a failed query cannot exempt.

**`verify_chain_max` (default 3) bounds the rule's DEPTH** (added 2026-11-09,
ranger-base-ewnas). Count the verify rounds already spent on a close's
LINEAGE: this close filed `discovered-from` a verify bead, that verify bead
answering exactly one close, repeated from that close. At or above the bound,
file no verify bead for that close — file one `-l question` bead for
`operator:` instead, priority the close's own, carrying the chain, the
close's evidence and what there is to decide; comment it back on the close,
distinguishably from `verify filed:`; dedupe it on its own title across OPEN
AND CLOSED question beads, because the subject is a close the watermark
walks past once. Block nothing and reopen nothing: there is nothing in the
queue to block. `0` disables the bound and then no edge is read for it. A
batched verify bead ends the walk rather than guessing which of its closes a
bundle answered, and a dependency read that fails yields a FLOOR that is
named and never escalated on — so the count errs short by construction.
Below the bound the filing is unchanged but for one head line naming the
round, which the last round says is the last. Not the artifact: keying the
chain on the touched path was measured and rejected (Consequences).

**4. Acceptance is explicit.** `closerDoneWhen` label/type-to-intent
selection is REPLACED (landed 2026-09-06, ranger-base-0ezn7), and not by an
unmatched whole-PID-table fallback. Each verification section points at the
closed bead's own description/acceptance as its
checklist, alongside closer, close reason, close time and labels. The
verifier reads that source directly; do not add an acceptance-heading parser
or second task field merely to avoid a bead read. When acceptance is absent,
say it is missing and record the verification limit. No guessed PID row
stands in for it. Preserve batching's safe data quoting if excerpting text.

**5. Citation and landing are different evidence.** Keep commits naming the
id under a header that says they may merely cite it. Beside them, retain the
session-branch block from `branch.<b>.posseBead` and `posseBase`: reached,
not reached, or named reasons a verdict cannot be made. A missing record
prints no invented success; deleting a landed branch also deletes its stamp.
Both are filing-time observations. The verifier rereads current evidence;
the filer does not rewrite a stored description and disturb deduplication.
0051 owns editorial citation conventions, never the landing verdict.

**6. Preserve class and findings rules.** Type `bug` or `feature` wins;
otherwise `debt` labels debt, and absence stays unclassified. Hand filers
classify the work; design/build, spike/question and verify inherit under
the existing class rule. Verify batches take bug, feature, debt, then
unclassified in that order. QA records `VERIFIED: <how>` and files nothing
on success. On escape (amended 2026-09-06 by operator ruling,
ranger-base-0f2zy) each finding takes one of two channels, decided by one
test the verifier applies per finding: **would the fix change what a process
does or what a pin proves?** No: the finding is RECORDED — one line on the
verify bead's close comment, `file:line · what · the close it escaped from`
— and no bead is filed for it. That is the whole class of comment claims,
doc sentences, NOTES and CHANGELOG wording, and a measurement record's
prose. Yes: the finding is LIVE and is filed as before — **one** lane/debt
findings bead per verify close carrying every live finding, each line with
file:line, failure, origin and repro; a pin that is wrong about what it
measures is live. Either way the verify closes `escape`, and a close with
recorded lines and no bead is a complete close. Only a reproduced live
money, constitution or dispatch-correctness defect gets a separate P1/P2
bug, cited by id in the bundle. Personas do not reopen the original close.
Verification never holds a build close hostage.

### §7 — a pin has to name the process it protects

Two outside-in reviews of 2026-09-09 (docs/notes.d/ranger-base-b0fsz.md §6–7,
docs/notes.d/ranger-base-vuosd.md §6) measured the cost of the escape
doctrine's habit: 1,120 `TestQA*` functions, 328 new `*_qa_test.go` files in
fourteen days, a class of tests whose subject is a comment, a Makefile
recipe, an ADR status line or another pin, and one pin that encoded a live
bug as its fixture so the fix broke the test. From this ruling, a new
`*_qa_test.go` whose scanner reads prose — code comments, Makefile text, ADR
or NOTES wording, another test's source — is filed only with the answer to
one question written on the bead that files it: **what process behaves
differently if this pin is deleted?** "The comment would be wrong" is not an
answer; "the launcher would grant a path the profile denies" is. A pin with
no answer is not written; the finding is recorded on the verify bead under
§6 and closes with it. Existing pins are not swept by this section — the
silent-revert audit exists because sweeps are how pins die — they are
retired one at a time, measured, as the census beads name them.

## Consequences and alternatives

MEASURED in the dated evidence: one-to-one verification amplified arrivals
beyond service; batching corrected the filing rate without dropping coverage.
ASSUMED price, and PAID as assumed: two source files plus tests
(`verifyafter.go`, `agents.go` — plus the shipped-example digest table any
example-PID edit requires); the intent-inference branches and their PID
lookup are gone, and no queue store, actor, config key or flag was removed.

MEASURED 2026-09-06 (ranger-base-0ezn7), answering the first done-when row —
how often inferred intent supplied a useful criterion absent from the task's
explicit acceptance. Window and coverage: the operator's whole queue, every
bead in the store, 2229 beads of which 2100 closed, 1048 of them carrying a
`verify_labels:` label, closed 2026-08-12 .. 2026-09-06.

- OUTCOME: the 203 verify beads this rule filed in that window carried the
  inferred row ZERO times, and no close of one cites it. The reader was shown
  able to see the line it counted, on a section rendered by the shipping code.
- COUNTERFACTUAL, the matcher as it stood on the day it was deleted: 379 of
  the 1048 closes would have earned a row — and those 379 rows are TWO
  distinct sentences, 359 of them one sentence. The cell is a property of the
  CLOSER's PID and the bead's TYPE, never of the task, so it cannot carry a
  criterion the task's own acceptance does not own: of those 379 sources, 279
  descriptions already say "test", 153 "cause", 140 "suite"/"green".

So the incremental value of guessed intent is not small, it is structural:
two constants. What it cost was real — operator-written PID prose
interpolated into a description whose per-close dedupe markers are found BY
LINE. Underspecified tasks now lose automatic completion text, as predicted,
and expose that gap: the section says the acceptance is missing and tells the
verifier to record it as the verification's limit.

MEASURED 2026-09-06 (the bead store, UTC, the coordinator's census on
ranger-base-0f2zy), the amplification the §6 amendment cuts: 132 beads
created and 131 closed in one day; 40 of the 132 were verify beads and 52
were debt findings, most of them escapes from those verifies; 45 of the 125
open beads carry `debt`. Over the week 2026-08-30..09-06 product Go grew
45,153 to 73,032 lines and test Go 101,329 to 187,572, with 38 of the day's
59 commit subjects pins, bans, censuses or measurements. The 2026-09-02
bundle rule bounded beads PER VERIFY CLOSE and the count kept growing,
because the bound was denominated in closes and closes were the growing
number: a doc-sentence finding filed as debt earns a fix, a pin over the
fix, a verify of the pin and a finding on the verify's prose. What ends the
loop is the channel, not the count — a finding whose fix runs nothing is a
comment, and a comment is not a bead. ASSUMED, the ruling's own estimate:
the recorded class is about a third of inflow; measure it after a week as
the share of `escape` closes that filed no bead, and the day's create count.

The open debt pile is not mass-closed. Triage rule for the coordinator, one
pass after this amendment lands: close each open `debt` bead whose finding
is of the recorded class with reason `doctrine 2026-09-06, recorded on
<verify bead>` — and add that one line to the verify bead it escaped from
so the record moves rather than vanishes; keep every bead whose fix would
change what a process does or a pin proves.

MEASURED 2026-11-09 (ranger-base-ewnas, scripts/chain-depth-census.py over
one shop's whole bead store: 3,353 beads, 2026-08-12 .. 2026-11-09, 2,634 of
them not verify beads), the depth §3's bound is denominated in — rounds
already spent on a close's lineage, by the shipped walk:

- 0 rounds 2055 (78.0%) · 1 round 527 (20.0%) · 2 rounds 41 (1.6%) · 3
  rounds 8 (0.3%) · 4 rounds 3 (0.1%). Deepest observed: 4. So 98.0% of
  closes sit inside one round, and a bound of 3 would have escalated 11
  closes in 89 days — about one a week — against the ~20 sessions one
  artifact took in the incident that filed this (another instance's project
  crew, two seats, ~20 verify→fix rounds on a test of two config files; a
  coordinator note on one bead did not stop it because each follow-up was a
  new id, and the home was paused by hand).
- THE ARTIFACT KEY, measured and rejected. The same census keyed on the
  touched path instead — two rounds are one chain when the closes' commit
  path sets intersect, with a batched hop disambiguated by that path, which
  is the key's whole appeal — reads 137 closes at 3 rounds or more, 10 at 8
  or more, deepest 13. Those are not one artifact: one 11-deep chain runs a
  cost-provider bead into a silent-revert bead into three `posse worktrees`
  beads into four common-dir-pin beads into a seatbelt bead, chained because
  every one touches `internal/posse/dispatch.go` and `NOTES.md`. A hot file
  is not an artifact and in this tree most files are hot, so the path key
  both over-counts and cannot be read. The lineage key needs no path at all:
  at `verify_batch: 1` — the default, and the incident's shape — a verify
  bead answers one close, so "filed from the verify of that close" already
  means "about that close's work".

ASSUMED, to be measured after a month: that the escalation rate stays near
one close a week on a queue of this size, and that an operator answer is
cheaper than the round it replaces. Read it as the count of `verify chain
stopped:` question beads and how many were answered by filing one more
verify bead by hand — if most were, the bound is one too low.

Rejected: doing nothing (guessed policy); removing batching (known queue
failure); bounding the depth in the VERIFIER's PID instead — a
diminishing-returns instruction is advice at the one hop the harness does
not own, and §6's own history says the channel has to change, not the
count; keying the chain on the artifact (measured above); blocking or
reopening the looping close (§6: verification never holds a build close
hostage, and the close is already closed); a `chain-depth` field or label on
the bead (another vocabulary for what the graph already says); a counter in
$RHQ_HOME/state (0011: a fifth store, and the first one that decides
something — the count is a fact about the beads); a label-to-intent map or graph inference (another vocabulary);
one bug per finding (amplification); branch-record absence as proof of
landing; verify as a close gate; filing recorded-class findings as debt at
a lower priority (same inflow, same loop, one more triage field); a
`recorded` label or a third close reason (a vocabulary for a comment line);
mass-closing the open debt pile unread (the live-class ones are work);
changing the verify seats or the batch size (the seats were not the
amplifier, the finding class was). bd remains the only work store and its
existing export is the exit hatch. The 2026-09-05 decision needed no runtime
edit; the 2026-09-06 amendment changes the rendered verify trailer's
wording and the shipped QA example, and nothing else in the runtime.

Historical rulings, measurement windows and alternatives: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0006-handoff-shapes.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).

Execution census 2026-09-05, at `bfb64fd4` (16:47): that baseline had
`closerDoneWhen` and `IntentDoneWhen` matching, and no unmatched
whole-PID-table fallback in verify construction, so the removal task was
scoped to the observed inference and told not to invent an extra branch.
Amended: `c5edac14` (17:44, ranger-base-29eei) ADDED exactly that fallback —
`closerIntentRows`, called from `verifySection` — 57 minutes after the census
and before `b96a1080`, the page the executor was given. The census therefore
described no baseline the executor ever saw. §4 governs regardless: an
unmatched whole-PID-table fallback is a guessed PID row, so ranger-base-0ezn7
deleted it with the inference (below) rather than preserving it, and that
divergence from its own task text is recorded here. The SIMPLIFY verdict is
unchanged.

Executed 2026-09-06 (ranger-base-0ezn7): `closerDoneWhen` and
`closerIntentRows` are gone from `verifyafter.go`, and with their callers
went `IntentDoneWhen`, `intentMatchesLabel`, `IntentRow`, `IntentRows` and
`markdownRows` in `agents.go` — the caller census showed nothing else read
them. The PID's `## Intents` table itself is untouched: `pidcheck` still
requires its header and the frontmatter `intents:` list is unchanged; it is
only the HARNESS that stopped mining it. The section now carries one
acceptance line, which interpolates nothing but the closed bead's own id.
