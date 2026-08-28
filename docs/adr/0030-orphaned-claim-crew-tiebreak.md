# ADR 0030 — Orphaned claims: a bead nobody visibly runs defers to its assignee's crew session

*Status: accepted 2026-08-28 · owner: architect · ranger-base-vn3o,
discovered from ranger-base-adb7 · implementation: ranger-base-um9a*

## Context

ADR 0008 shields the operator's conversations; ranger-base-adb7 taught
that the crew MARK protects the session and only the run RECORD (ADR
0011 §3 `bead:`, which `posse prompt` stamps) protects the bead. One
route is left, and it writes no record by definition: the operator opens
a crew session and types the work straight into the pane. The session is
shielded; the bead is not; the next pass builds a fleet twin and the two
agents race — the measured adb7 shape, where the twin ran the bead to
close out from under the operator.

Why this is not a bug to fix with one more record: bd claims are
deliberately durable — leases without expiry (ADR 0011's named
divergence), recovery from a dead runner is redispatch. So an
in_progress bead with no live session holding it under any name (run
record, Dial F name, pre-Dial-F slot) is genuinely ambiguous: a crashed
fleet run that recovery should relaunch, or the operator's hand-work
that must be left alone. The typed route stamps nothing, so no record
can break that tie. Something else has to, or the twin stays.

## Decision

**1. The tiebreak.** At the exact recovery moment — an in_progress bead
assigned to this persona that no live session holds under any name —
dispatch asks one more question before creating anything: does the
assignee have a live crew session in this bead's repo? If yes, the bead
**parks**: one visible line, no session created, no prompt, no claim,
same line under `--dry-run`, and `--resume` does not override (the same
clause crew already carries). Both launchers obey — the pass and the
cockpit's `d`. The line names the two releases: stamp the true holder
(`posse prompt` it with the bead) or free the persona (`posse crew
<name> --off`, kill, or finish the conversation).

The rule's shape matters more than its letter: **presence is consulted
only at ambiguity, never against a record.** A record naming a live
holder wins in both directions exactly as today (crew skip, holder
join); the crew-session walk runs only when every record has answered
"nobody", which is the one state the typed route and a crash share.

**2. What stands — ADR 0008 §2 moves one notch, not one section.**
Ready beads of that persona dispatch normally while the operator chats:
an unclaimed bead is free by the queue's contract (ADR 0011: bd is the
queue, atomically claimed), and parking a persona's whole lane for a
conversation is what §2 exists to prevent. In_progress beads with a
live holder keep today's behavior. The one accepted cost: while the
operator talks to a persona, crash-recovery of that persona's orphaned
claims in that repo waits — visible every pass, bounded by the
conversation, one keypress to release. A park is a wait the operator
can see; a twin is corruption they discover later. The asymmetry is the
whole decision.

**3. The claim is the operator's shield.** Hand-working a bead means
claiming it first — any tool; the claim itself now protects it — and
`posse prompt` when the text allows, which stamps the record and turns
the coarse park into a precise hold. A bead typed at while unclaimed is
still twinnable: accepted, the same clause as §1's "presses `o`",
because "unclaimed = free" is the contract everything else routes by.
(In the measured incident the coordinator did claim first; the flow
this blesses is the flow that already happens. ASSUMED: that shape
generalizes.)

## Consequences

- `fireLoop` (dispatch.go, after the holder join comes back empty with
  a nil run record): the crew-session walk, park line, `continue` —
  before PromptGrace, reported identically in `--dry-run`. `LaunchBead`:
  the same question where its name loop resolves no live session.
- Backend helper: first live session with `Crew`, `Agent == persona`,
  `Checkout() == dir`. Reap is already safe: autoReapPass skips crew.
- ADR 0008 §2 gains a pointer amendment; NOTES.md's *posse crew* bullet
  gains one sentence. ~50 lines + tests; one bead (ranger-base-um9a).
- Tests: the typed-route repro red at HEAD on both legs (fresh and
  `--resume`); the accepted cost pinned as deliberate (crashed run
  parks during a chat); the §2 pin (ready bead dispatches during a
  chat); LaunchBead's refusal; mutation-checked per arm.

## Alternatives rejected

- **adb7's option (a): skip every bead of a persona with a crew
  session.** Reverses §2's reason to exist — one conversation stalls
  the fleet's copy of the persona. 0008 already decided this; nothing
  new was learned that unlearns it.
- **A launcher-evidence discriminator** (Dial F worktree or branch
  `bead:` exists → it was posse's run, recover through the park). The
  clever one, and I wanted it: it recovers crashes promptly even
  mid-conversation. Rejected because it re-opens the twin on the
  take-over shape — fleet started the bead, the operator killed the
  session and hand-typed the rest; the worktree survives its session
  (that is its job, ranger-base-nurl), so the "evidence" outlives the
  intent. The costs are asymmetric: the discriminator converts visible
  waits into occasional corruption. No.
- **Auto-stamp at claim time** (`posse claim --as P` finds P's unique
  crew session and writes `bead:`). Inference relocated, not removed —
  and a wrong pointer is worse than none: it redirects the holder join
  and the reap ledger. If parked lines prove annoying, an *explicit*
  `posse claim --session <name>` is a bead of its own, not this
  decision.
- **Pane keystroke detection.** Reaffirmed out of scope (0008 §4):
  herdr sees processes, not intent, and scraping pane text is a new
  substrate dependency bought to avoid one visible line.
- **The session name in bd at claim.** Run facts live in posse's meta
  (ADR 0011 §3); a second store for them re-mints the pairwise
  disagreement class 0011 was written to close.
- **A timer on the park** (expire the hold, then recover). Expiry's
  failure mode is redelivery of work that may still be running — the
  exact twin this decides away, and the same reason §1 rejected the
  crew timer. bd claims stay leases-without-expiry; so does this.
