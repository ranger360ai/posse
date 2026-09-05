# ADR 0027 — Watch-owned pulse with delivery-only state

*Status: accepted 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · persistence and repeat simplification approved; code deferred.*

## Decision

Keep one pulse ticker inside the dispatch watch process, cancelled with that
process. No pulse on a hand-typed pass. The presence of `pulse_interval:` arms
it (typically 2m); unset is disarmed. This is independent of
`autostart_dry_run`, because a dry-armed shop still needs oversight. The
operator owns arming; this decision does not change instance configuration.

Every tick computes [ADR 0029's condition view](0029-governance-surface.md). Current
Herdr, bd, git, shared plan snapshot and cost inputs supply the facts. A failed
read is reported as partial; do not turn unavailable data into a condition or
an all-clear. Known conditions can still be delivered. The shared plan reader
keeps its cache/cadence and reads nothing for an unarmed guard. Persisted pulse
state is never an input to status or evidence that the shop is healthy.

For a nonempty set, join its sorted stable condition keys into the fingerprint.
Deliver a fixed `Pulse check:` hint when that fingerprint has not been prompted
or its last successful delivery is at least `pulse_renag` old. Use that one
configured repeat interval, default 30m; remove exponential doubling and
`pulse_renag_max`. An empty computed set clears delivery bookkeeping so
a later identical episode is fresh. Preserve partial-read diagnostics: this
delivery reset does not prove that an unobserved condition cleared.

Persist only `prompted_fingerprint` and `prompted_at` in `state/pulse.yaml`,
using the existing atomic file replacement. Compute observation time,
conditions and current fingerprint in memory each tick. Remove persisted
`at`, `conditions`, `fingerprint` and `renag_interval`; old files can be read
for their two surviving fields without a migration job. The watch owns this
delivery state through its existing single-watch discipline in
[ADR 0011](0011-dispatch-model.md); do not add another writer or lock service.

Target `pulse_persona`, else config `coordinator`. If both are empty, sense
and display but deliver nowhere; do not invent a missing crew member. Deliver
only to a live idle agent: working, blocked, missing-agent or missing-session
targets are skips. Never spawn a session. Only a successful prompt advances
delivery state; skipped or failed attempts remain eligible on the next tick.
An unreadable delivery record or a crash after delivery can cause a repeat;
this is best-effort deduplication, not exactly-once delivery.

Use Herdr `AgentPrompt` directly through the harness-originated persona seam,
without setting a crew mark. This is the one pulse exception in
[ADR 0008](0008-crew-sessions.md); all other crew protections remain. The prompt
has no authority: its recipient rechecks live state and applies its own
standing responsibilities. No model enters the guard or pause decision path.

## Price and deferred acceptance

Remove doubling/default-maximum logic in `internal/posse/pulse.go`, config
surfaces and tests. One config key and four persisted fields disappear,
including three snapshots the reader never consumes. Retain one ticker,
delivery file, repeat key and prompt path; no new flag or actor. The three
snapshot fields and interval state were separately priced in the review.

First done-when row: **fraction of repeat prompts, after the first
unchanged-condition prompt, followed by a corrective operator action before
the next scheduled repeat.** State window, denominator and attribution
limits. The review's 27 logged prompts establish activity, not usefulness.
Check restart deduplication, failed-delivery retry, clear/recur behavior,
partial reads, eligibility and old-file compatibility as separate acceptance.

What breaks if wrong: a long-lived condition repeats more often at the retained
interval, causing interruption or spend; a reset/error mistake suppresses an
undelivered hint. Do not quietly choose a different instance interval to
conceal those costs. The operator may tune the existing interval after seeing
the result.

## Evidence and alternatives

The shared governance sweep replaced the old herdr/git-only boundary on
2026-09-01. Its recorded cost was 757ms average, n=20 (610–844ms), versus an
earlier 32ms narrow check; those are dated observations, not present guarantees.
Retain bounded reads and log partial failures. No separate daemon, unconditional
heartbeat, fact-authoritative prompt, persisted condition mirror or replacement
backoff policy is justified by this simplification.

## Lineage

| Record | Surviving decision |
|---|---|
| 0027 sensing/delivery and 2026-08-28 default amendment | Watch ownership, arming, eligibility and crew exception |
| 0029 | Sole condition-computation authority |
| Operator ruling 2026-09-05 | Fixed repeat interval and delivery-only persistence |

[Prior pulse design and evidence](history/0027-monica-pulse-before-2026-09-05.md).
