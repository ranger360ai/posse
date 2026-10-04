# ADR 0066 — A typed-decision model, if posse ever carries one, is a second reader that reports and never acts; the readings log comes first

*Status: accepted 2026-10-04 — operator ruling A on ranger-base-our1e's recommendations: D1 the readings log (code bead filed), D2 report-only, D3 first seams gated on the corpus (spike bead filed, any model call is an operator spend ruling) · D3 defined, measured offline and amended 2026-10-04 (ranger-base-qk9tr: the input clause reversed, a heading reader recommended as the report, the model parked) · owner: architect · source
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

*Amended 2026-10-04 (ranger-base-qk9tr, spike, offline, no model call;
measurement in `docs/notes.d/ranger-base-qk9tr.md`).* **The question is
defined**: a `choice` over the eight known screens posse owns a capture of,
plus *none of these* (`scripts/d3-reader-eval.py --question`). **The clause
"over the regions herdr already extracted" is reversed by measurement**:
`agent explain` emits a 243-character PREVIEW of each region, that preview is
all a D3 record holds, and for 9 of the 15 labelled screens the heading is
outside it — a reader of any kind fed that record reads logo art. Over the
full capture a deterministic heading reader (the screens' own words, ~70 µs,
no spend, no egress) names 11 of 11 residue cases on the three measured
incident classes (caret codepoint, footer reword, extra rows) with 0 false
positives on the idle screens; over the previews, 5 of 11. So: **(a)** D1's
D3 record carries a fixture-shaped pane capture (`PaneRead`, the source the
fixtures were captured from, ceiling-redacted) — the architect's own D1
amendment, code bead ranger-base-76gc4; **(b)** the recommended D3 report is
that heading reader, SET-valued because 3 of 15 screens show two true screens
and a `choice` returns one — the operator's ruling, ranger-base-gy3io, with
ranger-base-6uokf filed behind it; **(c)** a model on D3 is not recommended
now: its only territory is a reworded heading, a class with zero incidents
in the record. The question and a `--corpus` scorer are committed so a D4
ruling can run it unchanged. The real corpus is empty today — the fleet
binary predates the log — and the composer-hold seam is untouched.

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
- *Amended 2026-10-04 (ranger-base-3xt9y, code): D1 is BUILT —
  `internal/posse/readingslog.go`, ten consequence sites, the data-ceiling
  redaction, and `scripts/readings-census.py` for the per-day census and the
  replay corpus. Three things the decision did not say, now measured and
  written down in `docs/notes.d/ranger-base-3xt9y.md`.* **(a)** The log goes in
  the session tree's own git DIR, not its working tree: `git status
  --porcelain` counts untracked files and six callers act on that count (ADR
  0041's closed-dirty comment, `RemoveSessionTree`, the retire guard, the land
  sweep, the reap guard), so a log one directory over would make every close a
  dirty close — and `lastTreeWrite` skips the log by name, or a tree that ever
  took a reading could never be retired. **(b)** "Never under $HOME" is read as
  "the log's lifetime is the TREE's, not the instance's", because `WorktreeRoot`
  refuses a worktree root outside `$HOME` on purpose; the enforced spelling is
  that a path in the instance's own stores is refused and a session with no
  tree gets NO log rather than a fallback. **(c)** The five consequences are
  five ways of NOT acting, so **a false IDLE is invisible to this log** — it
  types, and there is no refusal to key a record on. rangerhq-7ia, the incident
  in the bead's own measure list, is captured only in the sub-case where the
  reword drops herdr to its idle fallback. Closing that needs a SAMPLED record
  of non-consequential D1 readings, which changes the census's denominator and
  is this record's own call to make; the rate it would be priced against is now
  takeable for the first time.

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

- `grep -m1 '^\*Status' docs/adr/0066-*.md` reads `accepted` (it read
  `proposed` until the 2026-10-04 ruling).
- A D1 bead exists with the census script named; a D3 bead, if any, names
  the config key and the "empty is off" default — or, under the D3
  amendment, names no key at all, because the recommended reader is
  in-process.
- `python3 scripts/d3-reader-eval.py` prints `kw full on residue` equal to
  its denominator on the `caret`, `footer` and `depth` rows and `0` under
  `kw prev FP` (MEASURED 2026-10-04: 2/2, 2/2, 7/7, 0).
- No PID carries a TypeSafe or OpenRouter credential line before the §7
  rulings are recorded on a bead.
