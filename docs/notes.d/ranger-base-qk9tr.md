# ADR 0066 D3 offline: the unknown-screen reader's input is 243 characters, and that — not the reader — is where the diagnosis fails (ranger-base-qk9tr)

Spike, 2026-10-04, one seat, tree `093cfb1f` (dinesh's readings log landed
the same day). Recommendation only. No model was called, no account opened,
nothing spent, no new egress. Every number is **MEASURED** on this box
(macOS 25.4.0, herdr 0.9.1, python 3) with the command that produced it, or
**ASSUMED** with the derivation shown, or **UNKNOWN**.

The question the bead asked, in four parts: define the typed D3 question;
evaluate candidate readers offline against the corpus, the current rules as
baseline; report accuracy, calibration, latency and cost; recommend whether
to wire a reader as a REPORT beside herdr's verdict (ADR 0066 D2).

---

## 1. The corpus, and what it is not yet

**There are no real readings.** The readings log (ADR 0066 D1,
`internal/posse/readingslog.go`) landed on `main` at `4b471776` today; the
fleet's installed posse is `0.5.0+193c8782`, and `git merge-base
--is-ancestor 193c8782 4b471776` says that build predates the log. A walk of
every session tree's git dir on the box finds zero `posse-readings.jsonl`.
Nothing accrues until the operator's next refresh; my PID denies `posse
refresh`, so this is stated, not fixed.

The bead was gated on "a corpus of real readings". The corpus MECHANISM
exists and the dependency closed; the corpus is empty. Rather than return
the bead untouched, the evaluation below runs over the labelled screens posse
does own, states that it is a stand-in, and is written so the same scorer
runs unchanged over the real corpus (`scripts/d3-reader-eval.py --corpus`,
reading `scripts/readings-census.py --export-corpus`'s output).

**The stand-in: 15 labelled screens** under
`etc/herdr/agent-detection/testdata/{codex,grok}/`, 232–3,969 bytes, each a
real `herdr pane read --source detection` capture labelled by filename
(`verify-detection.sh` pins them). Nine known screens appear on them. The six
Bob captures under `upstream/bob/` are excluded on purpose: Bob is on the
REPORTED route (ADR 0061 D3.3), herdr refuses `agent explain` for it
(`fallback_reason: unknown_agent`, MEASURED), and a D3 record there carries
**no region bytes at all** — so no second reader of any kind has anything to
read for 1 of posse's 4 runtimes. That is a finding, not a gap in the
harness.

**Perturbations.** D3 fires only when herdr matched nothing; on an
unperturbed fixture the rules match by pin. So each screen is also replayed
under the classes in which a known screen has actually stopped matching:

| class | what it does to the capture | the incident it reproduces |
|---|---|---|
| `caret` | the selected-row glyph swapped (`›`→`→`, `❯`→`>`, `> 1.`→`→ 1.`) | ranger-base-0sa5a: three misses, each one codepoint |
| `footer` | the footer hint reworded (`Press enter to continue`→`Press Enter to proceed`, `esc to go back`→`Esc to go back`, `Ctrl+.:shortcuts`→`Ctrl+.:help`) | rangerhq-7ia: a footer reword flipped blocked→idle |
| `depth` | five non-empty rows above the content | ranger-base-k987u: the logo is 15 or 16 rows and the rules' windows carry one row of headroom |
| `heading` | the screen's own heading reworded (`Update available!`→`A newer Codex is available`, …) | **ASSUMED** — none of the nine incidents had this shape; included because it is the one class a heading reader cannot survive by construction |

## 2. The typed question (part 1 of the bead)

A System One `choice`, printed by `scripts/d3-reader-eval.py --question`:

- **state**: the region previews of one D3 record, joined — never the whole
  pane (ADR 0066 D3 as written; see §5 for why that clause does not survive).
- **question**: "Which known first-run screen is this terminal showing?"
- **options**, criteria = the screen's own heading: `codex.update_menu`,
  `codex.signin_menu`, `codex.signin_api_key`, `codex.hooks_review`,
  `codex.trust_directory`, `codex.model_picker`, `grok.startup_splash`,
  `grok.consent_banner`, and **`none_of_these`** last.

The option set is derived from the fixtures, not invented: every screen posse
owns a capture of. Two sources already name these screens and they do not
agree: the Go interstitial registry (`interstitial.go`, 5 of the 8 — it means
"operator-silenceable first-run screen") and herdr's manifests (7 of the 8,
by rule id; the consent banner is deliberately unanchored). Which one a
shipped reader lives on is flagged on the code bead rather than decided here.

**A `choice` returns one option.** 3 of the 15 captures show two true screens
at once (the splash with the consent banner over it). A reader that returns
a SET has no such limit; the question above cannot. This is a structural
mark against the typed-choice shape for D3 specifically, independent of any
model's accuracy.

## 3. Readers (part 2)

| reader | what it is | input | ran? |
|---|---|---|---|
| **rules** (baseline) | herdr's TOML manifests, replayed with `herdr agent explain --file`, manifests staged from THIS checkout (the `verify-detection.sh` rule) | the whole capture | yes, 67 calls |
| **keywords** | a deterministic reader: which known screens' headings (lower-cased, whitespace-collapsed) are present | twice — the 243-character region previews a D3 record holds, and the full capture | yes |
| **model** (Jev hosted / Kev local) | the `choice` in §2 | the previews | **no** — any live call is an operator spend and credential ruling (ADR 0066 D4, ADR 0019 §4/§6); Kev is "a model process the operator runs" and is not installed |

The keyword reader is the *simplest shape that could beat the baseline* on
the incident classes (standing order 2026-09-03): the incidents were in the
rules' STRUCTURE — a regex anchor, a footer clause, a region window — never
in the heading text, so a reader that keys on the heading alone should
survive exactly those classes. The experiment tests that.

## 4. Results (part 3)

`python3 scripts/d3-reader-eval.py`, 2026-10-04, 2.1 s wall, verbatim
summary:

```
cases: 67  errors: 0
rules reader: 67 herdr calls, 28.7 ms mean wall each
perturb  cases  rule names it  D3 fires  residue  kw prev on residue  kw full on residue  kw prev FP  kw prev exact
none     15     13             1         1        1/1                 1/1                 0           6/15
caret    13     10             2         2        2/2                 2/2                 0           6/13
footer   10     8              2         2        1/2                 2/2                 0           4/10
depth    15     7              7         7        2/7                 7/7                 0           6/15
heading  14     1              11        13       0/13                0/13                0           0/14
```

*residue* = a known screen is on the pane and herdr's rule did not name it:
the only cases D3 has anything to say about. Scoring a second reader anywhere
else credits it for the first reader's work.

**Latency and cost.** rules: 28.7 ms mean per `explain` (MEASURED, 67
calls; already paid at D3 time, the JSON is in hand). keywords: 68.6 µs mean,
184 µs max (MEASURED, `time.perf_counter`), no spend, no egress. model:
UNKNOWN on this corpus; MEASURED elsewhere and cited in ADR 0066's context,
~0.43 s and under one US cent per hundred calls via a gateway (Jev), ~20 ms
(Kev-0.8B). **Calibration**: the keyword reader has no confidence to
calibrate — it is a set of exact matches; the vendor's confidence is
distribution shape with no calibration claim (ADR 0066 context); nothing
here changes either statement.

**Per-case facts worth naming** (the full table is the script's output):

- The three measured classes reproduce. `caret` drops `trust_directory`
  (its regex is `\A> You are in`) — 0sa5a's shape. `footer` drops the model
  picker (`live_strong_blocker` is footer-keyed) and the consent-banner
  composer (`prompt_hints_idle` is footer-keyed) — 7ia's shape. `depth`
  drops **all five sign-in captures** and the trust dialog: five rows is
  past the one row of headroom `signin_menu`'s 25-line window carries
  (codex.toml's own note). The rule authors chose that headroom from the
  measured logo distribution; a five-row exceedance is my synthetic, not an
  incident, so no bead is filed on it — but it is the largest D3 producer in
  this corpus, and the next logo art is the thing that would make it real.
- `heading` defeats both readers almost totally, as designed (rules 1/14 —
  the generic footer rule still calls hooks-review `blocked`, right state,
  wrong name). This is the whole of the model's territory on D3.
- `codex/idle-composer` reads D3 under `--file` because a file capture has
  no OSC title (herdr's `osc_title_idle` is what makes a live composer
  `Seen`). A harness artifact; it would not fire live. Stated so nobody
  reads the row as an incident.

## 5. The finding: the input is 243 characters

`herdr agent explain --json` carries a `region_preview` of each region
**capped at 243 characters** (MEASURED on every region of every fixture;
`-v` does not lengthen it), and `ReadingEvidenceOf` copies exactly that
preview into the D3 record (`Truncated: true`). Every top-anchored region
(`top_non_empty_lines(20|25)`, `after_last_prompt_marker`, `whole_recent`)
begins at the top of the screen, and on codex that is 15–16 rows of ASCII
logo. So for **9 of the 15 captures, unperturbed, the screen's own heading
is outside every preview the record holds** — the five sign-in screens, the
model picker, and the splash's consent banner on three of four splashes.

The consequence is in the table: over the full capture the heading reader
names **11/11** residue cases on the three measured classes with **0**
false positives on the idle screens; over the previews, **5/11**. The reader
did not fail on the other six. It was handed logo art. A model handed the
same record is handed the same art, and no `criteria` text fixes that.

ADR 0066 D3 said "over the regions herdr already extracted (never the whole
screen)". The regions herdr extracts ARE the whole screen (`whole_recent` on
`blocked-signin.txt` is 1,053 bytes, the entire capture); what herdr EMITS
is a 243-character preview of each. The clause cited an artifact without
measuring its size, and the size decides the design. It is reversed in the
amendment (§7).

## 6. What it costs (part 4), do-nothing first

| | what the crew builds, tests, carries | buys |
|---|---|---|
| **R0 do nothing** | nothing | D3 stays `WhatHerdrSaw`'s 72-character preview per region; a human names the screen — 3j8 measured that at a hand-launch and a `posse peek` per screen. Rate UNKNOWN until the log runs. |
| **R1 the D3 record carries a fixture-shaped capture** | one `PaneRead(target, paneModeReadLines)` on the three D3 sites, refusal/hold path only — the call `permissionmode.go` already makes and the source the fixtures were captured from; ceiling-redacted like every region; one new region name; the `fits in a page` single-write claim re-measured (fixture max 3,969 bytes + six previews may exceed 4,096) | every D3 record is a candidate `testdata/<agent>/<state>-<what>.txt` — and capture → fixture → rule is how **every** real D3 was closed (3j8→37c/1xsj, 9py0, n6s2u, 7ia). The export already writes regions as files `herdr agent explain --file` reads. ASSUMED cost per D3 ≈ one `explain`, ~30 ms; not measured here (no live pane in this seat's scope). |
| **R2 the heading reader as the D3 REPORT** | one Go reader (set-valued), a marker phrase per known screen, a pin over the 15 fixtures with `d3-reader-eval.py`'s `EXPECTED` sets, a pin that the Go table and the script's `OPTIONS` agree; one line in `WhatHerdrSaw` and one field on the record; **no config key, no network, no credential**; the registry-vs-rule-id placement choice (§2) | 11/11 on the measured residue over the capture, 0 FP, ~70 µs. Legible and replayable at the codepoint — the property ADR 0066 keeps the decision on. Requires R1. |
| **R3 a model shadow** | ADR 0066's consequences as written: a JSON client, a config key, a model process (Kev) or two operator rulings (Jev), `criteria` under review | the `heading` class only — 0/13 for the keyword reader there by construction, UNKNOWN for a model, and **zero incidents** of that shape in the record. The question and `--corpus` scorer are committed so a ruling runs it unchanged. |

## 7. Recommendation

1. **R1 now.** This is an amendment to D1's record, the architect's own, and
   it is worth having whether or not any reader follows: today's D3 record
   reproduces nothing for the screens most likely to produce one. Code bead
   **ranger-base-76gc4** (dinesh).
2. **R2 as the D3 report — the operator's ruling**, because wiring a reader
   beside herdr's verdict is what the bead reserved to a ruling. Question
   bead **ranger-base-gy3io** with the three options priced; code bead
   **ranger-base-6uokf** (dinesh) filed dep-blocked on it and on 76gc4, so
   the ruling is a one-step unblock or a won't-do close.
3. **R3 not now.** Revisit only when `scripts/d3-reader-eval.py --corpus`
   over real D3 records shows a labelled residue the heading reader misses.
   The spend and visibility lines of ADR 0066 D4 are unchanged and untouched
   — nothing here needed them.
4. **ranger-base-o1aoi** (architecture, richard): dinesh's §4 finding that a
   false IDLE is invisible to the consequence-keyed log is a D1 ruling to
   make with the rate in hand, not before — filed as a reminder gated on 14
   days of census.

## 8. Standing-order check

Simplest way first: the model was the question asked, and the answer is that
a heading match over the right bytes does the measured job, so the model is
priced and parked rather than built. Reuse before invention: the option set
is the fixtures', the markers are the screens' own words, the capture call
is one posse already makes. Every number above is labelled; the one ASSUMED
class is the one the recommendation leans away from.
