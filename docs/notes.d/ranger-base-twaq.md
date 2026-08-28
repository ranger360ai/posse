# ranger-base-twaq — the availability mark had to survive a refresh

`posse relaunch` recreates a session from its meta, and the meta records
the pair the launch **fell to**, not the pair the PID asked for. So the
recreate asked `TierPreflight` about `claude/standard`, found it
available, fell nowhere — and re-derived `fallback:` as empty on a
session that was still running the substitute. Four things went quiet at
once: the meta line, `posse list`'s `⤵️fallback` tag, the relaunch
receipt's `FALLBACK:` clause, and `dispatch.effectiveTier`, which
answers only for a meta that records a fallback and so handed the work
prompt the bead's *resolved* tier — `strong` — for a session running
opus. dispatch.go names that one itself: "a header naming a model the
session is not running … the exact lie this preflight exists to kill."

The fix carries the line instead of re-deriving it (`NewSessionOpts.
Fallback`, set only by `RecreateOpts`), **conditioned on the launching
pair still differing from the PID's own**. That condition is the point:
the mark is a statement about a divergence, so it lasts exactly as long
as the divergence. Drop the condition and an operator who edits `tier:`
down to what the session really runs keeps a line saying "tier strong
wants claude-fable-5" forever.

Two things this is not. It does not move the pair — a session degraded
during an outage still stays on the substitute until it is created
afresh (ADR 0003 §3's trade, argued elsewhere). And it is not the
crash-restart path: `RelaunchAgent` re-types into the live pane and
writes the meta back, so the mark always rode through there. Only the
recreate re-derived.

Pinning note: three arms, three mutations, each checked to fail —
carry removed (both carry pins red), condition dropped (the negative
pin red), and the condition's runtime half dropped (the runtime-hop pin
red). The tier half alone looks sufficient until a `tier_fallback:`
runtime hop, which diverges by runtime at the *same* tier.
