# ADR 0008 — Crew sessions: dispatch never commandeers a session the operator is talking to

*Status: accepted 2026-08-18 · owner: architect*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> Persona names are restated as roles.

## Context

DIRECTION.md: **crew** = personas the operator actively talks to; **fleet**
= background workers grinding the queue. Dispatch can't tell them apart.
It skips sessions herdr shows *working/blocked*, but an *idle* session the
operator just prompted is fair game, and the next `Work beads issue …`
splices into a conversation. Tolerable by hand; not under `--watch`
(landed) or scheduled passes (blocked on this).

Dial F (ADR 0003) narrowed the surface: dispatch now creates
`<persona>-<repo>-<bead>` sessions and prompts only the session it made
for that bead (or the pre-Dial-F `<persona>-<repo>` slot when resuming).
What remains: the operator can step into *any* session (cockpit `enter`,
`p`, `posse prompt`, typing in the pane), and posse has no memory that they
did. `HerdrMeta` already carries per-session facts (`Agent`, `Runtime`,
`Tier`, `Cage`, `Degraded`); the natural place for one more.

## Decision

**1. The marker is `Crew` in the session meta — set by origin, then by
the operator's hands; never by a clock.**

| event | effect |
|---|---|
| `posse new` (any, `--agent` or recipe) | `Crew: true` — the operator made it to talk to it |
| dispatch `CreateSession` | `Crew: false` (fleet) |
| cockpit `p`, `posse prompt <session>` **without** `RHQ_PERSONA` in the caller's env | `Crew: true` — the operator started a conversation. With `RHQ_PERSONA` set, the prompt is a persona's (a coordinating persona orchestrating) and marks nothing |
| cockpit `o` / `posse crew <session> [--off]` | explicit toggle either way |
| cockpit `p`, `posse prompt <session>` where this home holds **no session meta** (a foreign workspace) | nothing recorded — and both paths say so in one line. The shield is the meta, not the prompt, so the operator who just started a conversation there has to be told it did not engage (rangerhq-sk6p) |
| session dies | marker dies with the meta |

Why not "operator prompted within N minutes": a conversation has no
timeout, and expiry's failure mode is exactly the splice we're removing;
stickiness's failure mode is a persona sitting idle — visible in the
cockpit and one keypress to fix. Why not origin only: the operator does
step into fleet sessions, and the definition is "talks to", not "made".
Typing directly into a pane stays invisible to posse (herdr shows
*working* while it lasts, which `personaActive` already respects); the
operator who wants that session kept presses `o` — the cost of no
timer, accepted.

**2. Dispatch treats a crew session as if it did not exist.** Not
prompted, not relaunched, not counted in `personaActive`/`busy` — the
operator talking to a persona must neither be interrupted nor stall the
fleet's copy of that persona. A bead whose own session (or the legacy
slot) is crew is reported `– <id> held by crew session <name>
(operator's) — skipped` in normal and `--dry-run` output and left alone:
**no fleet twin for that bead**; the operator finishes it or releases
the session (`o`) — `--resume` does not override crew, release first.
Other beads of that persona/repo dispatch normally into their own
per-bead sessions — under Dial F the "fleet twin" is automatic and needs
no `-fleet` suffix.

*Amended 2026-08-28 (ranger-base-adb7):* **which session is the bead's is a
lookup, not a name.** The shield above shipped asking herdr for two names —
the Dial F `<persona>-<repo>-<bead>` and the pre-Dial-F slot — so a crew
session the operator made by hand (`posse new jared-staffing`, then the bead
handed to it) held nothing: both names resolved to nothing, the pass read
"no session holds this bead", and `--resume` built a fleet twin that ran the
bead to close out from under the operator's own conversation. The mark
protected the SESSION and left the BEAD open. So the run record (ADR 0011 §3
`bead:`, which `posse prompt` stamps for a hand-dispatch via
`NoteBeadFromPrompt`) heads the name list this shield and the holder join
both read, in the pass exactly as it already did in `LaunchBead`. What is
still uncovered is what §1 already accepts: the operator who types straight
into a pane stamps no record, and presses `o`.

*Amended 2026-08-28 (ADR 0030, ranger-base-vn3o; lands with
ranger-base-um9a):* **an orphaned claim defers to its assignee's crew
session.** The typed route stamps no record, so an in_progress bead no
live session holds under any name is ambiguous — a crashed run to
recover, or the operator's hand-work to leave alone. At that recovery
moment only, a live crew session of the assignee in the bead's repo
parks the bead (visible line, nothing created, `--resume` does not
override) instead of twinning it. Ready beads still dispatch — "other
beads dispatch normally" above now reads: other beads *whose claims are
not orphaned*. The claim is the operator's shield for hand-work; the
record (`posse prompt`) remains the precise one.

*Amended 2026-08-21 (ADR 0027, monica pulse — designed as "0013", file
committed 2026-08-27 under the free number):* one named exception — the watch loop's
**pulse** may prompt the `pulse_persona` session (config; typically the
coordinating persona) even when crew-marked: idle-only, `Pulse
check`-prefixed, harness-originated via the §1 `RHQ_PERSONA` seam so it
sets no crew mark. Every other session keeps the full shield; the
bead-prompting path is unchanged.

*Amended 2026-09-02 (ranger-base-f6lk):* **the mark is worn by two shapes,
and only one of them is a session the operator owns.** §1's table makes a
crew session two ways — `posse new` MAKES a conversation, while cockpit `p`
/ `posse prompt` mark a session **dispatch** made for one bead that the
operator merely stepped into. The shield above did not distinguish them, so
the second sat outside every sweep forever: measured on the fleet 2026-08-29,
two of them (`<persona>-<repo>-ranger-base-3j3t`, `<persona>-<repo>-ranger-base-teau`)
skipped on hundreds of consecutive passes, and the operator reaping such
sessions by hand — the mechanism the auto-reap exists to replace.

So the auto-reap (autoreap.go) may now take a crew-marked session, and only
this one: its name is the name `SessionForBead(persona, dir, bead)` renders
from the session's **own record**, its bead the store of record calls closed,
its agent has settled, no launcher prompted it inside `PromptGrace`, its tree
holds nothing a kill would take (dirty paths, or commits the base does not
hold by measured patch-id **and** content — ranger-base-as19/x8jp), its
persona is not `pulse_persona:`, and it has been untouched for
`reap_crew_after:` (default 4h; `off` restores the permanent skip). Rendering
the name from the record is the only inference here, and it goes from record
to name, never the reverse — a name is a lossy encoding of a bead id
(ranger-base-kftx) and is never read back into one.

The sibling arm this landed with is not this ADR's — a per-bead-named session
with no `bead:` pointer at all (ranger-base-kftx) — but one property of it
belongs here for contrast: that arm fires only at a sweep running **past
routing**, because dispatch reaches a session by name whether or not it
carries a pointer, so an unpointed session at a live bead's name is a seat the
pass is about to reuse. The crew arm needs no such wait: its bead is closed,
and a closed bead is never dispatched again.

What is unchanged is the part §1 argued for: a session the operator MADE is
never swept, at any age, whatever pointer it later carries. That is stronger
than the "longer grace" the bead asked for, and it is why this is not the
timer §1 rejected — the failure mode of a timer was splicing into a
conversation, and nothing here prompts anything. Typing straight into a pane
still stamps no record, and still does not need to: herdr reports it as
`working`, which the sweep refuses one guard earlier.

**3. What shows.** Cockpit rows and `posse list` carry a `crew` tag (`👤`)
after the persona; crew sessions keep their status sort (a blocked crew
session is still blocked-first — the operator wants to see it). Cockpit
footer on a session row gains `o crew/fleet`.

**4. Out of scope.** Advisory-mode gating (ADR 0001's "dispatch does not
read mode today") — same policy surface, different rule; file a bead if
wanted. Detecting keystrokes in a pane. Per-persona "always crew" —
a PID that should never be dispatched simply has no `labels:` and is
never assigned; that already works.

## Consequences

- `HerdrMeta.Crew bool` (+ `HerdrSession.Crew`); set in `CreateSession`
  from `NewSessionOpts.Crew` (`posse new` true, dispatch false); flipped by
  cockpit `p`/`o`, `posse prompt` (env check), `posse crew`. Dispatch:
  `personaActive` skips crew; the held/resume path reports and skips;
  `--dry-run` line. Cockpit/`posse list` tag. NOTES.md *Dispatch primitives*
  gains the rule in one sentence. ~60 lines + tests.
- Scheduled dispatch is unblocked once this lands.
- The coordinating persona's rule (a PID `## How you work` line, the
  operator's edit): give work with `posse dispatch`, talk with
  `posse prompt`; a `👤` session is the operator's.

## Alternatives rejected

- **Timer ("prompted within N minutes").** Above.
- **Origin only.** Above.
- **A `-fleet` twin session per persona.** Dial F already gives every
  bead its own session; a suffix would be a second naming scheme.
- **Herdr-side flag.** herdr sees processes, not intent; posse's meta is
  where posse's facts live (same as `Degraded`).
