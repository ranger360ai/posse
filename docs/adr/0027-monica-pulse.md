# ADR 0027 — The monica pulse: a shop-check ticker inside the watch loop

*Status: accepted 2026-08-21 (design on rangerhq-wxd) · owner: architect ·
amends 0008 §2 (crew carve-out) · implemented rangerhq-4ish (sensing,
d9fa52f) + rangerhq-44w1 (delivery, 18cb114)*

> **Numbering note.** This design was cited as "ADR 0013" in bead comments
> (rangerhq-wxd/4ish/44w1/0l6t) and in the pulse code, but the file was
> never committed and 0013 belongs to the runtime dispatch contract
> (0013-argv-prompt-probe.md is that ADR's probe companion, the 0002/0009
> convention — not a free slot). Committed 2026-08-27 as 0027
> (ranger-base-r63j); the pulse-family code comments were renumbered in the
> same commit. An "ADR 0013" citation elsewhere in the tree means the
> dispatch contract, not this file.

## Context

An unattended `dispatch --watch` fleet can degrade silently: a session
sits blocked, closed beads pile up unpushed, or the coordinating persona's
session is simply gone — and nothing notices until the operator returns.
The incident that motivated this happened in exactly the window where the
fleet was armed dry (`autostart_dry_run`): oversight was needed most
precisely when dispatch itself was doing least. Judgment about what to
*do* already lives in the coordinating persona's PID (typically monica);
what was missing is a cheap, spend-free way to tell her session that the
shop needs looking at.

## Decision

The pulse is a goroutine ticker inside the dispatch `--watch` process —
no second loop, no new process. It starts with the watch loop and dies
with its ctx. A hand-typed pass never pulses: like `Unattended`
(watch.go), only a timer with no human witness needs a shop check running
behind it.

**§1 — Condition set (sensing).** Every tick computes a cheap shop check:

- (a) `blocked:<session>` — any live session whose agent herdr reports
  blocked
- (b) `unpushed:<repo>:<n>` — unpushed commits (`@{u}..HEAD`) on a config
  `beads:` repo; no upstream reads as no condition, never an error
- (c) `no-live:<persona>` — no live session for `pulse_persona`

Sensing touches herdr and local git only — **never bd, never the plan
endpoint**: this runs off a timer with no human watching it spend, so its
failure mode must be silence, never a false alarm or a bill.

**§2 — Ticker, fingerprint, arm switch.** The sorted condition set joins
into a fingerprint persisted to `state/pulse.yaml` (machine-local,
gitignored; one file, one writer — the tick goroutine). The arm switch is
the presence of `pulse_interval:` in config.yaml (typical 2m),
**independent of `autostart_dry_run`** — a dry-armed fleet still gets
oversight, which is the incident's exact window. Unset family = disarmed
default, not a misconfiguration.

**§3 — Delivery: level-triggered hint, renag backoff.** On a non-empty
set that is *due* — a fingerprint not yet prompted, or an unchanged one
whose renag interval has elapsed — the pulse prompts `pulse_persona`'s
live session (default `monica`), **idle-only**, with a fixed
`Pulse check:`-prefixed one-liner. The prompt carries **no authority**:
it names the observed conditions as hints to re-verify against live state
(rhq list, git, bd) — the stores stay the record, the PID stays the
judgment. While a condition set persists unchanged, re-prompts back off
by doubling: `pulse_renag` (default 30m) → ×2 per repeat → capped at
`pulse_renag_max` (default 4h). A cleared set resets the clock; the next
non-empty set is a fresh prompt even with an identical fingerprint.

**§4 — Delivery discipline and the crew seam.** Only an
actually-delivered prompt advances the bookkeeping
(`prompted_fingerprint`/`prompted_at`/`renag_interval` in
`state/pulse.yaml`); a skip (session working/blocked/no agent) or an
undeliverable tick (no live session — condition (c) already says so)
leaves it untouched, so the same set is retried next tick rather than
gated behind renag for a prompt that never went out. Undeliverable
**creates nothing** — the pulse never spawns a session. Delivery targets
the session directly via herdr `AgentPrompt`, not `posse prompt` or
`personaActive`/`crewHeld`: this is the one named exception to ADR 0008
§2 (amended there) — the pulse may reach a crew-marked session, and
because it rides the harness-originated `RHQ_PERSONA` seam and calls no
`MarkCrew`, it sets no crew mark. Every other session keeps the full
shield.

## Consequences

- `internal/rhq/pulse.go` + `watch.go` hook; `pulse_*` config family
  (autostart_* flat-YAML style); `state/pulse.yaml`.
- A second watch loop's state writes interleave last-writer-wins —
  accepted: one loop per queue is the invariant in practice (ADR 0011
  §1), and a stale fingerprint costs one missed pulse, not a wrong one.
- Arming is the operator's: set `pulse_interval:` (rangerhq-0l6t carries
  the live-verify pass and the arm question).
- The prompt's usefulness is bounded by the target PID — the pulse tells
  monica *that*, never *what*; a PID with no standing intents gets a hint
  it cannot use.

## Alternatives rejected

- **A standalone `[[startup]]` loop** — the ct9/ppy9 class of failure:
  a second long-lived process to leak, wedge, and version-skew against
  the watch loop it duplicates. One loop per queue.
- **herdr x4e events as the delivery channel** — kept as a latency
  upgrade later (comment left on x4e), rejected as the foundation:
  edge-triggered events miss whatever happened while nobody listened;
  the level-triggered fingerprint re-derives truth every tick.
- **Folding approvals/judgment into dispatch** — judgment lives in the
  PID, not in Go; the harness senses and delivers, the persona decides.
- **Blind heartbeat (prompt every N minutes regardless)** — spend with
  no signal, and it trains the recipient to ignore the channel.
- **Fact-carrying prompts (the pulse asserts state)** — Two-Generals
  territory: by the time the prompt is read the fact may be stale, and a
  prompt that *asserts* gets acted on without re-verification. Hints
  only; the stores stay the record.
