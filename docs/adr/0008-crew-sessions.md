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

*Amended 2026-08-21 (ADR 0013):* one named exception — the watch loop's
**pulse** may prompt the `pulse_persona` session (config; typically the
coordinating persona) even when crew-marked: idle-only, `Pulse
check`-prefixed, harness-originated via the §1 `RHQ_PERSONA` seam so it
sets no crew mark. Every other session keeps the full shield; the
bead-prompting path is unchanged.

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
