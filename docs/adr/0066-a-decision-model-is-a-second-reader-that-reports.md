# ADR 0066 — A typed-decision model, if posse ever carries one, is a second reader that reports and never acts; the readings log comes first

*Status: proposed — operator discussion pending · owner: architect · source
bead ranger-base-our1e (spike, recommendation only) · sits under ADR 0057
(no guard branches on a display reading), ADR 0060 D2 (detection is herdr's
to ship), ADR 0061 D3 (a label is identity, not liveness) and ADR 0063 D1
(a stall reading is reported, never acted on) · evidence and the full
decision-point map in `docs/notes.d/ranger-base-our1e.md`*

## Context

The operator asked where posse's loop makes fast structured decisions that
TypeSafe AI's Jev — a hosted model that answers typed questions about a
state with probabilities and no text — could make better or cheaper, and
what harness changes that implies. Ten decision points were mapped from
the code at `b973acd9`. None is a model call today; every one is a herdr
rule, a string or regex match, a bd field, or a threshold. The screen
readings (pane state, dialogs, unknown screens, the composer hold) are the
brittle ones: nine incidents since September, each a codepoint, a reworded
footer, or a menu drawn where text was expected. The governors, routing and
comment classes are deterministic and legible and should stay so.

Three independent evaluations within ten days of launch (MEASURED, notes §2)
agree with the vendor's own jaggedness page: Jev is a strong zero-shot
classifier on short English few-option questions (95.9% where keywords hit
77.2%), level with mid-price LLMs and 4–11 points behind the frontier,
~0.43 s through a gateway, under one US cent per hundred calls at the
published list price — and it **always
answers**: 0 of 30 out-of-scope inputs flagged at 0.99 confidence without a
"none of these" option, calibration error 4.4× the noise floor, accuracy
falling as unrelated state grows, adversarial text in the state moving the
answer. A terminal screen is that state, and a false idle types a prompt
into a shell (ranger-base-3p0). Jev is closed-weight and hosted, with no
published retention terms. Kev (Apache-2.0, same API, 0.8B runs on Apple
Silicon at ~20 ms) is the exit hatch, accuracy on our corpus UNKNOWN.

The finding that governs the decision: posse records its incidents by bead
but not its readings by count, so **the error rate of today's readers is
UNKNOWN**, and no reader — model or regex — can be shown better.

## Decision (proposed)

**D1 — The readings log precedes any model.** Every pane-state, dialog,
unknown-screen and composer-hold reading that ends in a refusal, hand-back,
settle-open, ghost retirement or hold is recorded with the region bytes it
read, under the session tree; a census script yields readings per day and a
replay corpus. This is a bead on its own, worth filing whether or not any
model follows.

**D2 — A decision model is a second reader whose output is a report.** Its
reading prints beside herdr's verdict in the failure line and on the bead.
No guard, launch eligibility, hand-back, kill, hire or keystroke reads it.
This restates ADR 0057, 0060 D2, 0061 D3 and 0063 D1 for a new reader and
adds nothing to them; a later record that wants a model reading to act must
reopen one of those four.

**D3 — First seams: the unknown-screen diagnosis, then the composer hold.**
The unknown-screen line is already "diagnosis only" (ranger-base-3j8) and
is the only decision point whose consequence is a slower human. A `choice`
over the known interstitials plus **none of these**, with confidence, over
the regions herdr already extracted (never the whole screen). The composer
hold follows only as a tie-breaker that can move a hold toward *waiting*,
never toward *clear* — the ghost reading's existing posture.

**D4 — Local first; hosted only under two operator rulings.** The reader
runs against a box-local System One server (Kev) with no credential, spend
or egress. Jev is admitted only if (a) the operator rules once on metered
spend in the loop with a daily ceiling written on the ruling, and (b) a
zero-retention term exists, because a screen region leaving the box is a
visibility move (hard line 4). Absent either, Jev is out of scope.

**D5 — Adoption is measured against D1's labels, not against the vendor.**
Bar: the reader flags every labelled false-blocked and false-idle in the
corpus with the none option present, and reads zero of the idle fixtures as
blocked; question `criteria` text is kept and reviewed like code.

**Not decided here, and not recommended:** a shared `Reading` type across
the five bespoke reading structs (only if D3 lands — one reader is not a
seam); a model noul for the allotment-limit message (worth one call site
once a client exists); a pulse pre-filter; anything numeric.

## Consequences

- One new bead now (D1). No code, config, credential or dependency until
  the operator rules on §7 of the notes.
- If D3 lands: one JSON client, one config key naming the endpoint (empty
  is off), a model process the operator runs, and `criteria` text under
  review. ASSUMED full-shadow cost if hosted: about 1,200 input tokens and
  ~0.4 s per reading at the vendor's published list price, the per-day sum
  following from the instance's seat count and pass rate; local: no spend
  and ~20 ms.
- The regex and TOML readers stay the readers. Their failures remain
  replayable at the codepoint, which is the property a model does not have
  and the reason they keep the decision.

## Alternatives rejected

- **Replace herdr detection with the model** — upstream's to ship (0060
  D2); the noisy, adversarial state is the vendor's listed weakness; a miss
  is illegible. Rejected.
- **Let a high-confidence reading act** (type, hand back, kill) — 0 of 30
  out-of-scope inputs were flagged at 0.99; confidence is distribution
  shape, not correctness, by the vendor's own definition. Rejected.
- **Model-routed beads** — routing is by label and legible (0033); a
  suggestion at filing time is advice to a persona, not a harness decision.
  Rejected.
- **Jev first, Kev as fallback** — reverses the spend and visibility
  lines for a speed gain nobody measured a need for. Rejected.
- **Do nothing** — priced in the notes as R0. Rejected only in the narrow
  sense that D1 costs one bead and makes the price a number.

## Verification (the closer's observables, when the operator rules)

- `grep -m1 '^\*Status' docs/adr/0066-*.md` reads `proposed` until ruled.
- A D1 bead exists with the census script named; a D3 bead, if any, names
  the config key and the "empty is off" default.
- No PID carries a TypeSafe or OpenRouter credential line before the §7
  rulings are recorded on a bead.
