# Jev as a harness component: where posse decides fast, and whether a typed-decision model belongs there (ranger-base-our1e)

Spike, 2026-10-04, one seat, tree `b973acd9`. Recommendation only. Operator
ask: *"look at Jev and provide a deliverable of significant improvements in
harness design for Posse leveraging Jev. We would need to discuss
recommendations."*

Every number below is **MEASURED** (URL fetched and read 2026-10-04, or a
count taken at `b973acd9`), **ASSUMED** (derived, with the derivation shown),
or **UNKNOWN** (looked for, not found). No Jev call was made, no account was
opened, nothing was spent. The draft decision is ADR 0066 (`proposed`); this
fragment is the evidence it stands on.

---

## 1. What Jev is (identity confirmed)

Jev is TypeSafe AI's hosted "System One" model, early access since
2026-09-15. It is not a text generator. A request carries one `state` (text)
and a map of typed `questions`; the answer carries, per question, a typed
value plus a probability distribution and a derived `confidence`, and no
text. Three question types: `noul` (a 0–1 truth value for a statement),
`choice` (pick one of named options, each with a `criteria` description),
`score` (an ordered scale). Endpoint `POST https://api.typesafe.ai/v1/systemone`,
bearer key from the TypeSafe console, Python SDK `typesafe-sdk`; also listed
on OpenRouter as `typesafe/jev-1.13` (32K context, released 2026-09-18).
MEASURED: docs.typesafe.ai introduction, quickstart, confidence and patterns
pages; openrouter.ai/typesafe.

Vendor claims (MEASURED as *claims*, typesafe.ai front page 2026-09-28):
193.6× faster and 444.6× cheaper than LLMs "on System One tasks"
(USD 0.000081 and 0.114 s per decision against USD 0.013880 and 8.566 s);
USD 42 per billion input tokens, that is USD 0.042 per million, output free.
The "zero
hallucinations" headline is explained on the same page as "every decision
includes a confidence estimate", and the FAQ concedes the model "can get
things wrong". The confidence page makes **no calibration claim**: confidence
is "a derived statistic that collapses the distribution into a single 0-to-1
number … 1 when all the probability is on one outcome and 0 when spread
evenly", with the instruction to "start with conservative thresholds, test
with your own data".

LangChain's "Building a Harness with Jev" (Runkle & Lovell, 2026-09-17)
places it at three seams of an agent loop: model routing, tool guardrails
(classify a tool call before it runs), and classification of state. It gives
no latency, accuracy or head-to-head numbers of its own and repeats the
vendor's "up to 200× / 400× on classification tasks".

## 2. What the independent evaluations found

Three pre-registered or shadow evaluations exist, all within ten days of
launch. These are the numbers the recommendation rests on.

| source (MEASURED 2026-10-04) | finding |
|---|---|
| github.com/priorbench/jev — 5,721 calls, 21 experiments, 50 pre-registered predictions, 2026-09-20, USD 0.176 total | 95.9% zero-shot on a 400-item intent set vs 77.2% hand-written keywords vs 66.0% TF-IDF+LR. **Without a "none of these" option, 0 of 30 out-of-scope inputs were flagged, at 0.99 confidence.** Accuracy flat across thresholds 0.50–0.95, then 100% at 0.99 covering 60.2% of traffic. Option order moved answers up to 13 points. Wrong `criteria` text collapsed accuracy to 16.7%; absent text cost 0.8. ~430 ms latency floor via OpenRouter; 800 judgments in one 985 ms call for USD 0.00075; throughput saturates near 11 req/s. "The model always answers." Counting beyond small n unreliable. |
| github.com/ickma2311/jev-baselines-eval — Banking77 (n=208) and CLINC150 (n=200), jev-1.13.0, 2026-09-18 | Jev 0.832 / 0.870 vs gpt-5.4-nano 0.793 / 0.795 vs frontier GPT-5.6 0.875 / 0.915 vs a supervised encoder 0.933 (10,003 labels, 10 ms). Median latency 0.43 s. Error-ranking AUROC 0.853 then 0.734: "neither direction is established". Cascade verdict AMBIGUOUS; 102 items at confidence exactly 1.0, 6 wrong. USD 0.071–0.106 per 1K calls. |
| beri.net shadow eval (phishing, 2,000 synthetic emails) and the dev.to eight-day roundup (2026-09-23/24) | Asked once, "is this phishing?": Jev 62.6% vs Haiku 4.5 81.3%. **Split into five atomic nouls combined by a logistic regression fitted on 1,000 labels: Jev 95.0% vs Haiku 93.2%.** ECE 0.107 on 900 support tickets (4.4× the noise floor); yes/no underconfident, choice/score overconfident; on a task where the answer was not in the state it was right 44.7% of the time at mean probability 0.74. Roundup: 72.5% on a six-model study, "level with mid-price LLMs, 6.5 to 11.5 points behind the frontier"; median server time ~0.1 s; strong on binary English at volume (98.33% on 18,514 emails), weak on many-option multi-class, non-English, long or irrelevant input, and "when the answer does not follow from the input". No outages or rate-limit incidents reported in the window. |

The vendor's own jaggedness page for 1.13 (docs.typesafe.ai/model-jaggedness)
agrees with the evaluators on every point that matters here: "accuracy falls
as the state grows with content unrelated to the decision"; adversarial
content in the state "can move the answer" and is not treated as hostile;
"leans toward the option that comes first"; "does not count reliably"; dates
are read "as text, not as ordered quantities". Its mitigations are the
pattern every evaluator converged on: filter state to what the question
needs, decompose into atomic questions and combine in code, add an explicit
"not stated" option, test with reordered options, keep arithmetic in code.

**Exit hatch (MEASURED, github.com/jaredpalmer/kev):** Kev is an Apache-2.0
family of decision models (0.8B/4B/9B on Qwen3.5, 27B) that implements the
same System One API, so "the Python SDK works against a Kev server
unchanged". Kev-0.8B runs on "any Apple Silicon" at 22.7/16.1 ms; Kev-4B
needs 32 GB on a Mac. Kev-27B reaches 0.851/0.889 against Jev's 0.857 on the
author's new-source set; the small sizes are "within four points of Jev" on
classification, with MMLU 0.73 (9B) against Jev's 0.90 and a single-
temperature calibration. Jev itself is closed-weight and hosted only; no
self-host or data-retention terms are published (UNKNOWN).

## 3. posse's fast decisions, and what makes each one today

Taken from the code at `b973acd9`. posse makes **no in-process model call
to decide anything**: its only outbound HTTP in the loop are two zero-token
GETs to api.anthropic.com (plan usage, model list). Every decision below is
a herdr manifest rule, a Go string or regex match, a bd field match, or a
numeric threshold.

| # | decision | reads | mechanism today | measured failure (bead) | consequence of a wrong answer |
|---|---|---|---|---|---|
| D1 | pane state idle / working / blocked | screen regions | herdr TOML rules (`etc/herdr/agent-detection/{codex,grok}.toml`), herdr-bob `rules.json` | footer reword silently flipped blocked→idle (rangerhq-7ia); bob rules miss all three real blockers and **false-block** on the `/permissions` picker, each miss one codepoint (`→`, a hyphen) (ranger-base-0sa5a, b96nx); splash read as blocked (rangerhq-1xsj) | false blocked hands a claim back; false idle types a prompt into a shell (ranger-base-3p0) |
| D2 | "is this screen a permission dialog" | screen lines | the `blocked` rules above; posse branches only on state + rule id and **never answers a dialog** (`awaitSettled`, G1 row) | same corpus as D1; Bob fires no approval hook so the dialog "can only be read off the pane" (0sa5a) | a missed dialog is a stuck seat reported as working; a phantom one is a false hand-back |
| D3 | unknown screen diagnosis | `agent explain --json` `evaluated_rules[].evidence` | `internal/posse/unrecognized.go`: 0 bytes → "CLI not spoken yet", bytes → "unknown screen"; **diagnosis only, no key pressed** | three distinct screens produced one identical failure line, two needing opposite fixes (ranger-base-3j8) | a slower human triage; nothing acts |
| D4 | composer hold: working / typed / sent / ghost | footer and prompt-box previews, ANSI dim scan, `~/.claude/history.jsonl` | `internal/posse/panework.go`, `internal/posse/ghostbox.go`, `internal/posse/sentline.go`: regex + word lists + SGR-2 scan + echo equality | idle behind its own suite run read as settle-open, re-prompt unsent 4h (ranger-base-htafy); a menu over the composer read as typed text (i6t90); a row that could never go false (wr624) | a missed hold re-prompts over live work; a phantom hold leaves a seat idle |
| D5 | prompt stall verdict {rewait, keep, hand back} | herdr stall code, a second `agent wait`, commit count on the branch | `internal/posse/promptstall.go`: enum switch + `commits > 0` | unclaimed although the work was done, loadavg 94 (ranger-base-uauvn); 3 of 4 stall witnesses named healthy passes (ADR 0063, 0 of 697 unresolved) | ADR 0063: a stalled pass is **reported, never acted on** |
| D6 | did this turn deliver | bd status after settle; claude transcript JSONL | `gather`: closed → land, else settle-open comment then a question bead; `internal/posse/turnfailure.go` `Contains("you've reached your … limit")` | mid-flight allotment refusal after 33 calls, six beads (ranger-base-qcu4c); wrong cwd made the reader blind (f09bw) | a refusal read as a quiet settle-open costs one polite re-prompt; the second files a question (ADR 0013, 9hm) |
| D7 | route a bead to a lane | bd labels, assignee | label overlap sorted by `route_order:`, assignee is a lane of one, coordinator denied (ADR 0033) | the order was alphabetical by accident (2yj5); misfiled lane work is a *filing* error, not a routing one (ADR 0033) | the wrong persona succeeds, or a bead sits |
| D8 | load / plan / blind / budget governors | loadavg, oauth usage %, reading age, $ per epoch | thresholds (`load_guard:` 25, `plan_guard_*`, `plan_guard_blind_max:` 10m, 80%/100% rungs) | 19h blind caught at 96% (c3vqe); a sleeping box read as hung (sqxo1, wj7e9) | hires at the wrong time, or none |
| D9 | verify triage, close-reason class, ladder subtype | bd comments and fields | prefix and word regex (`BLOCKED:`, `REFUSED:`, `rejectRe`), `verify_labels:` rule | false positive once (muoo) | a verify bead not filed, or a scorecard miscount |
| D10 | pulse: is the shop state worth a coordinator turn | G-table fingerprint | fingerprint change or renag interval → a **billed persona turn** (`internal/posse/pulse.go`) | — | the one place posse already spends a model turn on a reading |

Counts at `b973acd9` (MEASURED): 15 screen fixtures under
`etc/herdr/agent-detection/testdata/`, 232–3,969 bytes each, 16,251 bytes in
all; six Bob captures replayed under ranger-base-0sa5a. That is the entire
labelled corpus posse owns for D1–D4.

## 4. Fit, point by point

The test is the evaluators' own: a typed-decision model fits a **short,
English, semantic, few-option** judgment whose wrong answer is **cheap and
reversible**, and misfits a **numeric, long, noisy, or adversarial** state
whose wrong answer **acts**.

| # | fit | why |
|---|---|---|
| D1 | **no, as the reader; maybe, as a shadow** | A terminal screen is the noisy state the vendor warns about, and it contains the agent's own prose, which is unfiltered third-party text in the state ("Allow? (y/n)" quoted in a reply is the obvious injection). A false idle types into a shell. ADR 0060 D2 makes detection herdr's to ship; ADR 0057 forbids any guard branching on display readings. The regex rules are brittle at the codepoint, but their failure is **legible**: 0sa5a found each miss by replaying six captures through the rule. A model's miss is not. |
| D2 | **no** | Same state, and the consequence is a hand-back. The incident class (one codepoint) is a fixture-and-replay problem, already harnessed by `scripts/verify-herdr-bob-rules.sh` (~9 s). |
| D3 | **yes, reported only** | The only decision point whose output is already "diagnosis only, nothing acts" (3j8). A `choice` over {consent banner, version splash, sign-in, update menu, model picker, composer not yet drawn, **none of these**} with confidence, printed beside herdr's `evaluated_rules`, is exactly the triage the coordinator assembled by hand three times. Short state (the regions herdr already extracted, not the screen). Few options. English chrome. |
| D4 | **tie-breaker only** | The ghost reading already has the right posture — "may retire a claim but never license a keystroke" (ghostbox.go). "Is the prompt box showing a menu or typed text" (i6t90) and "is the footer a background task of the agent's own" (htafy) are nouls over ~200 bytes. A second reader that can only move a hold toward *waiting* (never toward *clear*) adds no new way to re-prompt over live work. |
| D5 | **no** | Already reported-never-acted (ADR 0063 D1); the verdict is `commits > 0`, which a model cannot improve. |
| D6 | **yes, for the message class** | "Does this final assistant message say a usage or allotment limit was reached" is a binary English noul over one message, replacing a `Contains` on three literal phrasings that drift with the vendor's wording (qcu4c). Consequence of error: the stall/settle path already in place. |
| D7 | **no** | Routing is by label and must stay legible (ADR 0033). A model could *suggest* a label at filing time, but filing is the crew's act and the suggestion would be advice to a persona, not a harness decision. |
| D8 | **never** | Numbers, dates, ages: the vendor's listed weaknesses, and these governors are the brake (ADR 0063 D2). |
| D9 | **no** | Prefix conventions the crew is instructed to write; a model would only add tolerance for personas disobeying the ladder. |
| D10 | **not a replacement; a possible pre-filter** | The pulse exists so a persona with agency reads the shop. A noul "has anything in this G-table changed in a way that needs a human or coordinator" could suppress a renag — but the renag interval already does that, and the saving is one billed turn per interval (UNKNOWN how many per day are redundant). Not worth a dependency on its own. |

**Cost of running a shadow (ASSUMED, shown):** a herdr region preview is
≤ 4 KB ≈ 1,200 input tokens, so at the published list price a hundred
readings cost well under one US cent; a pass is p50 2m20s (ADR 0063, 713
headers), so the per-day sum is readings-per-pass × passes-per-day × that
unit, an instance arithmetic left to the instance. Added latency is ~0.4 s
per reading via a gateway (MEASURED floor) against herdr's local-ms
`explain`. Kev-0.8B on this box: no spend, ~20 ms, no egress (MEASURED
latency claim, accuracy on this corpus UNKNOWN).

## 5. What would change in the harness, if anything does

The recommendations below are ordered by what they cost the crew to build,
verify and carry (standing order 2026-09-03). The do-nothing option is first
and is priced.

**R0 — Do nothing.** Cost: the D1/D2/D4 incident stream continues at its
current rate. That rate is UNKNOWN as a number: the incidents are counted by
bead (0sa5a, b96nx, 7ia, 1xsj, htafy, i6t90, wr624, 3j8, 3p0 — nine since
2026-09) but the denominator (readings per day) is not recorded anywhere.
This is the finding that matters most: **posse cannot today say how often
its screen readings are wrong**, so it cannot say whether any reader, model
or regex, would be better.

**R1 — A readings log, no model.** Record every D1–D4 reading that ends in a
refusal, a hand-back, a settle-open, a ghost retirement or a hold, with the
region bytes herdr read, under the session tree (where ADR 0041's dirty-tree
record already lives); a `scripts/readings-census.py` turns the log into
rate-per-day and a replay corpus. This is the precondition every evaluator
names ("your labelled log prices being wrong") and it is also the first
measurement the regex rules have ever had. Cost: one writer, one script, disk;
a visibility question (§6). Buys: the denominator for R0, and fixtures for
0sa5a-class replays without a hand-launch.

**R2 — A second reader on D3 and D4, reported only, local first.** A
`Reading{Value, Confidence, Source}` returned beside herdr's verdict in the
unrecognized-screen diagnosis and the composer hold, printed in the failure
line and on the bead, **never read by any guard, launch, hand-back or
keystroke** (the ADR 0057/0063 posture, verbatim). Run it against Kev-0.8B
on the box first: no credential, no spend, no egress, ~20 ms. Jev only if
the R1 corpus shows Kev disagreeing with the labelled truth more than the
regex does, and only with an operator-held key. Measured before adoption by
agreement against R1's labels, with the acceptance bar: the reader must
flag every labelled false-blocked and false-idle in the corpus with a
"none of these" option present, and its own false-blocked rate must be
zero on the idle fixtures. Cost: a Go client for one JSON endpoint, one
config key naming the endpoint (empty = off), a model process the operator
runs, and the per-question `criteria` text maintained like code (16.7% when
wrong). Carries: a fourth `Source` in the reading vocabulary.

**R3 — A noul for the allotment-limit message (D6).** Replace the three
literal `Contains` phrasings with one question over the final assistant
message, through the same client as R2, with the literal match kept as the
fallback when the endpoint is off. Cost: one call site. Buys: tolerance to
the vendor rewording a refusal. Only worth doing if R2's client exists.

**R4 — The seam, if R2 lands.** Today each reading is a bespoke struct
(`AgentDetection`, `PaneHold`, `PaneMode`, `stallVerdict`, `TurnOutcome`).
If a second reader exists for two of them, a shared `Reading` carrying
source and confidence lets the failure line and the bead comment print
"herdr: idle (fallback) · model: sign-in 0.93" uniformly. **Not before R2**:
a reading type with one reader is the second registry the standing order
names as the racing signal.

Not recommended, with reasons: replacing herdr detection (ADR 0060 D2,
upstream's to ship; and a model's miss is illegible where a regex's is a
replayable codepoint); any model reading that licenses a keystroke, a
hand-back, a kill or a hire (0057, 0061, 0063); model-routed beads (0033);
any numeric governor (D8); a pulse pre-filter (D10) as a first use.

## 6. What must not change

- **No autonomous spend.** A Jev call is metered; a key in the watch loop is
  spend on every pass. R2 is specified Kev-first so that the first adoption
  spends nothing; a Jev key is the operator's to create and hold (ADR 0019
  §4 "no metered API credential substitution", §6 "no speculative provider
  configuration"), and the per-call spend it authorises must be ruled once,
  explicitly, with the daily ceiling from §4 written on the ruling.
- **Visibility (hard line 4).** A screen region leaving the box carries the
  operator's bead titles, paths and the agent's prose. Jev publishes no
  retention terms (UNKNOWN); the shadow evaluator's advice is "negotiate zero
  data retention in contract, not per-request flag". Until that exists, only
  a box-local model may read a screen. R1's log stays under the session tree
  for the same reason.
- **Detection is herdr's (ADR 0060 D2); a reported label is identity, not
  liveness (0061 D3); nothing acts on a stall reading (0063 D1); no guard
  branches on a display reading (0057).** Every recommendation above is a
  *report*. If a later ADR wants a model reading to act, it must reopen one
  of those four, and should expect to lose.
- **The stores are the record.** A reading is a comment on the bead or a
  line in the failure output; it is never a new state file posse keeps
  current between passes (0060 D2 "a daemon in costume").
- **The egress contract.** The L4 cage allowlists hosts by CONNECT
  authority; a hosted decision endpoint is a new host on a loop whose only
  egress today is two zero-token GETs. Local-first keeps the egress list
  unchanged.

## 7. Questions for the operator discussion

1. **Spend line.** Is a metered decision model inside the dispatch loop
   acceptable at all under hard line 1, given a once-ruled daily ceiling
   (unit cost ASSUMED in §4)? If no, R2 is Kev-only and Jev is out of
   scope entirely.
2. **Visibility line.** May screen regions leave the box to a vendor with no
   published retention terms? If no (my recommendation), Jev is out of scope
   until terms exist, and R2 is Kev-only by rule, not by preference.
3. **R0 vs R1.** Is the readings log worth one bead now, independent of any
   model? My recommendation: yes; it is the only item here that makes R0's
   cost a number.
4. **Which of R2/R3, if any,** and in what order. My recommendation: R1,
   then R2 on D3 only (the point already "diagnosis only"), Kev-0.8B,
   measured for one month against R1 before D4 is considered.
5. **Who labels.** A corpus needs ground truth; the coordinator assembled it
   by hand on 3j8. Is that the coordinator's standing work, or a `-l qa`
   verify slice per incident?

## Sources (all fetched 2026-10-04)

- https://docs.typesafe.ai/introduction · /introduction/quickstart.md ·
  /confidence.md · /model-jaggedness/jev-1.13.md · /llms.txt
- https://typesafe.ai/ (claims page, dated 2026-09-28)
- https://www.langchain.com/blog/building-a-harness-with-jev (2026-09-17)
- https://openrouter.ai/typesafe (model ids; the per-model pages answered 404
  on fetch; pricing and 32K context taken from search-result snippets and
  the vendor page — MEASURED as snippets, not as the page)
- https://github.com/priorbench/jev (2026-09-20)
- https://github.com/ickma2311/jev-baselines-eval (2026-09-18)
- https://www.beri.net/article/typesafe-jev-typed-decision-model-calibration-decomposition-shadow-eval
- https://dev.to/aws-builders/jev-after-eight-days-of-independent-tests-level-with-mid-price-llms-behind-the-frontier-1c60
- https://github.com/jaredpalmer/kev
