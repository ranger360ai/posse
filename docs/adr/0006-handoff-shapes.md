# ADR 0006 — Handoffs carry explicit acceptance; verification stays batched

*Status: accepted; simplified 2026-09-05 by operator ruling · inferred-intent removal landed 2026-09-06 (ranger-base-0ezn7) · owner: architect.*

## Context

Doing nothing guesses a bead's intent from labels/type and a closer's PID.
Beads carry no intent field (0001). Keep the measured batching correction,
deduplication and landing evidence; verification uses the task's stated
acceptance. Missing criteria must be visible, not silently invented.

## Decision

**1. Keep three handoff channels.** Progress and decisions about this bead
are comments on it. Work for someone is a new bead, lane label and
`discovered-from` provenance, with blocking dependencies where order matters.
Conversation is not a dispatch channel. File unassigned unless the task
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
on success. On escape, file **one** lane/debt findings bead per verify close,
each line carrying file:line, failure, origin and repro, then close `escape`.
Only a reproduced live money, constitution or dispatch-correctness defect
gets a separate P1/P2 bug, cited by id in the bundle. Personas do not reopen
the original close. Verification never holds a build close hostage.

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

Rejected: doing nothing (guessed policy); removing batching (known queue
failure); a label-to-intent map or graph inference (another vocabulary);
one bug per finding (amplification); branch-record absence as proof of
landing; verify as a close gate. bd remains the only work store and its
existing export is the exit hatch. No runtime edit accompanies this decision.

Historical rulings, measurement windows and alternatives: the page as it stood before this simplification is in git history, `git show c86a6b8:docs/adr/0006-handoff-shapes.md` (the dated copies were dropped by operator ruling 2026-09-05; git history is the record).
are retained separately.

Execution census 2026-09-05: this baseline has `closerDoneWhen` and
`IntentDoneWhen` matching; no unmatched whole-PID-table fallback was found
in verify construction. The removal task deletes the observed inference,
not a hypothetical extra branch. The SIMPLIFY verdict remains applicable.

Executed 2026-09-06 (ranger-base-0ezn7): `closerDoneWhen` and
`closerIntentRows` are gone from `verifyafter.go`, and with their callers
went `IntentDoneWhen`, `intentMatchesLabel`, `IntentRow`, `IntentRows` and
`markdownRows` in `agents.go` — the caller census showed nothing else read
them. The PID's `## Intents` table itself is untouched: `pidcheck` still
requires its header and the frontmatter `intents:` list is unchanged; it is
only the HARNESS that stopped mining it. The section now carries one
acceptance line, which interpolates nothing but the closed bead's own id.
