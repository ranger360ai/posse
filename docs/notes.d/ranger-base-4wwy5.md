# ASD-STE100 for the harness's own English: what the standard mandates, where posse's English failed this week, and what a controlled language would buy (ranger-base-4wwy5)

Spike, 2026-10-04, one seat, tree `b60b777e`. Operator ask (Dave): *"investigation
of ASD-STE100. I am intrigued by this old standard of simplified technical
english and wonder if it could help clarify our harness especially when
english must be used."* Recommendation only: nothing in ORDERS, PIDs, ADRs or
the binary was rewritten here.

Every claim is **MEASURED** (a URL fetched and read on 2026-10-04, or a command
run at `b60b777e` on this box), **ASSUMED** (priced, not run), or **UNKNOWN**
(looked for, not found). The standard's own PDF was **not** opened: it is free
but requires a registration form (§1.4), which is the operator's call. Everything
about the rules below comes from the publisher's public pages, two papers by
STEMG members, and an MIT-licensed paraphrase of the rule list.

The one-paragraph answer: **do not adopt ASD-STE100; adopt its dictionary's
shape.** The four English failures the operator named are, on inspection, one
dictionary failure (a word with no agreed meaning), one lineage failure (a rule
whose reason died while its text stood), and two that are not language at all
(a reader that cannot tell typed text from a suggestion; a line printed without
a timestamp). The shop already runs a controlled vocabulary where it matters —
the rung words and MEASURED/ASSUMED — and it propagated into 371 comments this
week without drift. The gap is that the vocabulary is nowhere written down as a
dictionary, so a word outside it (`WIP`, "clean") has whatever meaning the
writer had in mind. The draft decision is ADR 0067 (proposed).

## 1. What ASD-STE100 actually mandates

### 1.1 Identity and status (MEASURED, asd-ste100.org, 2026-10-04)

| fact | value | source |
|---|---|---|
| name | "ASD-STE100 Simplified Technical English — Standard for Technical Documentation"; a registered EU trade mark (No. 017966390) and copyright of ASD, Brussels | [home page](https://www.asd-ste100.org/) |
| current issue | Issue 9, 2025-01-15; issues come "usually three years"; Issue 10 scheduled January 2028 | [FAQ](https://www.asd-ste100.org/STE_faq.html) |
| what it is, in its own words | "an international standard to write technical documentation in a controlled natural language. STE has two parts: a set of writing rules (part 1) and a controlled dictionary (part 2)" | FAQ |
| origin | AEA (European airlines) asked AECMA in 1979 for one controlled language all manufacturers could share; working group formed 1983-06-30; first Guide 1986; Specification from 2005; "international standard" from Issue 9 | FAQ, "Who created STE?" |
| maintainer | the ASD Simplified Technical English Maintenance Group (STEMG), a working group of ASD; national Support Teams (STEST) | FAQ; Zambrini & Chiarello 2025 |
| intended readers | "to make aircraft maintenance documentation easier to understand for readers with only a basic command of English" | home page |
| intended texts | "procedural and descriptive texts" of maintenance documentation; "It is not intended for general-purpose writing, such as international correspondence" | FAQ, "Who needs to write in STE?" |
| what the writer needs | "a C1 proficiency level in English … is recommended"; "STE was created for the maximum benefit of the reader. This does not necessarily mean that it is simple to write" | FAQ, "Is STE simple to write?" |
| readers need no training | "It is not necessary to know STE rules to enjoy the benefits of reading documents written in STE" | FAQ |
| tools | optional; "no tool can replace the standard itself"; ASD endorses no checker | FAQ |

### 1.2 Part 1 — the writing rules

53 rules in nine sections. Issue 9 added no rule and "refined" 31 of the 53
(MEASURED: Zambrini, tcworld, February 2025). The section list and the rules
quoted below are from an MIT-licensed paraphrase for software text
([aminblg/simpleenglish, rule-catalog.md](https://raw.githubusercontent.com/aminblg/simpleenglish/main/skills/simple-english/references/rule-catalog.md),
fetched 2026-10-04), cross-checked against the publisher's FAQ and Wikipedia
where they overlap. The official wording is **UNKNOWN** to this spike.

| section | rules | the rules that would matter here |
|---|---|---|
| 1 Words | 1.1–1.14 | approved words only, each with its one approved meaning and part of speech; "technical nouns" and "technical verbs" from the company's own glossary are allowed outside the dictionary |
| 2 Multi-word nouns | 2.1–2.2 | noun clusters of three words or fewer |
| 3 Verbs | 3.1–3.7 | approved tenses; no `-ing` verb forms ("usually not permitted" — FAQ); active voice |
| 4 Sentences | 4.1–4.5 | one topic per sentence; condition before instruction ("If hot oil touches your skin, injuries can occur" — FAQ) |
| 5 Procedural writing | 5.1–5.5 | imperative ("Install the component", never "must be installed"); **one instruction per sentence** unless two actions are simultaneous; **≤ 20 words** per sentence |
| 6 Descriptive writing | 6.1–6.6 | **≤ 25 words** per sentence; **≤ 6 sentences** per paragraph; passive only when the agent is unknown |
| 7 Safety instructions | 7.1–7.3 | a warning starts with a clear command or condition |
| 8 Punctuation and word count | 8.1–8.7 | the counts above, plus list and punctuation shapes |
| 9 Writing practices | 9.1–9.4, GR-1..8 | general practices |

### 1.3 Part 2 — the dictionary's shape (the part worth copying)

MEASURED from the FAQ and Zambrini & Chiarello (MDTT 2026, CC BY 4.0):

- About **900 approved words**, "selected because they were simple and easy to
  recognize"; criteria "simplicity, and frequency of use". American spelling,
  Merriam-Webster as the reference.
- **One word, one meaning, one part of speech.** The FAQ's own examples: *fall*
  means "to move down by the force of gravity", never "to decrease"; *about*
  means "concerned with", never "approximately"; **`check` is approved only as a
  noun** ("do a check"), not as a verb; *close* (Wikipedia) has two approved
  senses, moving together and operating a circuit breaker, never "close a
  meeting".
- **One synonym kept, the rest listed as not approved with the replacement**:
  "STE uses *start* instead of *begin*, *commence*, *initiate*, or *originate*".
  Approved words are printed in capitals, unapproved in lower case with the
  approved alternative beside them.
- **Technical names escape the dictionary** only when they come from "official
  documentation, engineering drawings, company glossaries, or terminology
  databases" — i.e. STE assumes the organisation HAS a glossary, and the
  dictionary governs everything that is not in it.
- The dictionary is managed deliberately across editions: "selective retention,
  consolidation, and limited removal of dictionary entries, supported by
  definitional refinement, restriction tightening" (Zambrini & Chiarello 2026).
  A change form exists for proposing a word; approval lands in the next issue,
  never immediately.
- **Reproduction**: the one published derivative found ships "no dictionary
  content, because the standard forbids reproduction without written authority
  from ASD" (SimpleEnglish README). UNKNOWN whether that is the PDF's exact
  notice; it is consistent with the trade-mark and copyright notice on every
  page of the publisher's site. This kills one enforcement option outright (§4).

### 1.4 Obtaining it (what registering requires)

MEASURED 2026-10-04, [STE_downloads.html](https://www.asd-ste100.org/STE_downloads.html)
→ a Google Form, "Request your copy of ASD-STE100": "Complete this form to
download your official free copy of ASD-STE100 Issue 9." Fields: name and
surname (required), country (required), company or organisation (required),
field of activity (required), "New STE user?" (required), company website
(optional), email for STEMG news (optional), and a consent box (required) whose
text says the details "are for record and statistical purposes only and will
not be used or disclosed in any form to anyone". No payment. PDF only. The
older request page (Issue 8, 2021) asked for an emailed form; it is still live
and stale. **Registering is the operator's call** (a form carrying a name and
organisation is a visibility move under the hard risk lines); this spike did
not need the PDF and nothing below depends on it.

### 1.5 The publisher's own position on LLMs

The STEMG white paper "ASD-STE100 Simplified Technical English and Artificial
Intelligence" (MEASURED, [PDF](https://www.asd-ste100.org/assets/files/WhitePaper-ASD-STE100_and_AI.pdf),
3 pages, fetched 2026-10-04): LLMs "can compromise controlled natural language
discipline, reduce traceability of content decisions, and introduce undetected
inaccuracies"; "Automated checks for STE compliance, but currently with varying
levels of accuracy"; "Accountability: Responsibility remains with the human
author or organization". The downloads page states the point this shop would
recognise as its own: **"Plausibility must not be confused with verified
compliance."**

## 2. Posse's English surfaces, measured, with the week's incident on each

Sentence counts split on `.!?` with code spans replaced by one token; MEASURED
at `b60b777e`. STE's caps are 20 words (procedure) and 25 (description).

| surface | writer → reader | size | sentences > 20 / > 25 words | incident |
|---|---|---|---|---|
| **standing orders** (`ORDERS.md`, this persona's) | the persona and the operator → the persona's model at every launch | 13,147 words, 407 sentences, median **31** words, p90 53 | 74% / 64% | ranger-base-pm5zo (2026-10-03): two commits reached `main` with `WIP —` subjects (`73b9b1e4`, `b13ab874`). The closer's comment: "they are the pre-mutation checkpoints my orders require … I did not squash them — rewriting history is the one thing my standing orders flatly forbid." Two rules in one persona's orders, both obeyed; the word `WIP` has no agreed meaning in this shop, so on `main` it reads as unfinished work. **A dictionary failure.** |
| **PIDs** (`examples/agents/*.md`, nine shipped) | the operator and the architect → the model | 8,398 words; architect.md 50 sentences, median 18 | 40% / 28% | none this week. The `deny:` block is already a controlled language: `Bash(pkill:*)` has one reading, enforced by a shim. The prose above it is the part that explains three times. |
| **work prompt** (`internal/posse/dispatch.go`, the escalation block) | the binary → the model | 37 sentences, median 15, max **66** | 35% / — | none this week. The rung words — NOTE, ASSUME, SPIKE, ASK, HANDOFF, REFUSE — are one word, one meaning, upper case, and the shape propagates: of 371 comments written since 2026-09-27, MEASURED appears in 128, VERIFIED 108, NOTE 55, ASSUMED 35, SPIKE 15, UNKNOWN 10, ASK 10, HANDOFF 10, BLOCKED 5, REFUSED/REFUSE 3, DIVERGED 1. **The controlled vocabulary already works where it exists.** |
| **settle and pulse lines** (`dispatch.go` `turnOutcomeClause`, `beadpulse.go` `Line()`) | the binary → the operator and the coordinator's model, often hours later | the settle clause is one 36-word sentence joining a fact and an instruction with an em-dash | — | ranger-base-sqxo1 (2026-10-03): "It reads as 5h45m because pulse lines carry no timestamp; only pass headers do." ranger-base-l51p1 (**2026-09-10**, not this week): the dispatcher's settle text was sent, Claude Code then drew a suggestion ("keep waiting, then commit and close") and the pulse read that ghost text as an unsent prompt. **Neither is a wording failure**: one is a missing field, one is a reader that cannot tell typed from suggested. |
| **bead titles and close comments** | personas → the dispatcher (which routes on labels, not titles), the verify lane, the operator | 2,844 titles, median 17 words, p90 28, max 53 | 34% / 16% | the operator's "close reason read two ways": **UNKNOWN which bead** — no comment this week says so in those words. Nearest MEASURED case: ranger-base-26y03 closed with arms 2 and 3 "clean", and its own closer filed ranger-base-z9ct5 titled "closed with them argued, not measured" because "clean" without a label is two claims. ARGUED is not in the shop's label set; the closer coined it, and the verify lane (ranger-base-ugf48, laurie) adopted it unprompted. |
| **ADR prose** (66 records) | the architect; amended by any lane | 162,587 words | not measured | ADR 0019 said "Posse does not add Unicode NFC normalization"; ranger-base-d88rp (2026-10-03) added NFC for the trust key; the rule's text stood while its stated reason ("Go's standard library has no NFC") died, in the ADR and in two code comments, until ranger-base-4ch00 reversed the decline the same day. **A lineage failure, not a vocabulary one**: the fix was amending the reason beside the rule (ADR 0040 amendment-in-place), which no language standard does. In the same ten days ADR 0062 was edited by 7 beads, ADR 0060 by 7, ADR 0013 by 4: the protection there is the one-writer rule (ADR 0022). |
| **runbooks** (`docs/runbooks/`, 6 files) | personas → the operator | 16,373 words | not measured | none found |
| **the binary's refusal and warning lines** | the binary → the persona's model and the operator | 17,822 string literals of 40+ characters in non-test Go under `cmd/` and `internal/` | not measured | ranger-base-lcode (this week): a warning line "writes to os.Stderr, which is /dev/null under a watch loop" — delivery, not wording |

What STE's dictionary would collide with, MEASURED on `AGENTS.md` alone: 66
`-ing` words in 3,969 (1.7%); 19 passive constructions; `check` used as a verb
(STE: noun only); `close` as the shop's central verb (`bd close`; STE approves
two physical senses). Fourteen Makefile doors are named `*-check`. The house
nouns — pin, door, gate, wall, cage, seat, pass, lane, class, land, settle,
park, retire — are "technical names" in STE's sense, legal only if a company
glossary defines them. **There is no glossary**: `grep -ril glossary docs
DIRECTION.md NOTES.md AGENTS.md` finds none, and the only "vocabulary" hits are
ADRs refusing to invent a second one.

## 3. Per surface: adopt STE rules / a posse dictionary in STE's shape / leave alone

| surface | call | reason |
|---|---|---|
| standing orders, NOTES, notes.d, ADR Context/Consequences/Alternatives | **leave alone** | the reader is an LLM and the operator; the value is the argument and the story ("when a rule here seems arbitrary, the story is there"). STE's FAQ: "not intended for general-purpose writing". The two agent skills that apply STE to model output both warn the same way: "it deletes persuasion by design" (Hermes `simple-english`), "Keep your voice for your blog" (SimpleEnglish). A 31-word median sentence has not been the failure; an undefined word has. |
| PIDs | **leave alone**, except the dictionary applies to the words they use for shop mechanisms | persona voice is part of the design (ADR 0001); the enforceable part is already `deny:` rules |
| ADR **Decision** sections | **dictionary** | a Decision is the one place a rule's text is read by a pin or a persona as an instruction. Use the approved term (land, settle, pin, door) and label every price MEASURED or ASSUMED — already the architect's orders; the dictionary makes the term list findable |
| bead titles, close comments | **dictionary** — labels, not length | titles here are long because the title is the finding; a 20-word cap would cost what the shop values (a close comment "itemized enough to reconstruct the lost commit"). The failure shape is a bare "clean"/"done"/"fixed": every outcome word carries MEASURED / ASSUMED / ARGUED / VERIFIED / LANDED |
| lines the binary writes that a human reads later (settle, pulse, refusal, warning) | **three STE rules**, as a writing rule for the code that renders them | these are the one surface whose readers match STE's: a tired operator at 02:00 and a coordinator model with no way to ask back. (1) One fact per line; an instruction, if any, is its own clause after the fact, condition first. (2) Approved state words only (settled, landed, parked, blocked, refused), never a synonym. (3) A line that may be read later than it is printed carries its timestamp (sqxo1). Length: a settle or pulse line over 25 words splits (ASSUMED benefit; `turnOutcomeClause` at 36 words is the test case). |
| work prompt | **leave alone**; the rung words join the dictionary as its first entries | it is already the shop's best controlled text and its 66-word sentence is the HANDOFF rung's provenance paragraph, which is a procedure the model follows correctly |
| ASD-STE100 itself (Part 1 rules or Part 2 dictionary) on any surface | **reject** | the dictionary's approved meanings collide with the shop's central verbs; C1-writer discipline over 160k words of ADR prose would cost weeks (ASSUMED) against zero incidents this week that a length or voice rule would have caught; the dictionary may not be reproduced, so a checker against it cannot live in a public tree |

## 4. Enforcement options, priced

| option | what it is | price | call |
|---|---|---|---|
| **do nothing** | keep the rung words as oral tradition | 0. Cost of the status quo this week: one verify pass that had to explain `WIP` (rkva0), one architect session to re-reason ADR 0019 (4ch00), one coined label (ARGUED) that happened to be adopted | the baseline every other row must beat |
| **a tree-wide pin that reads a surface against a dictionary** | `internal/treepins` test over `docs/adr` / `examples/agents` / `ORDERS.md` | against STE's dictionary: **dead on licence** — the dictionary cannot be committed. Against posse's own: ADR 0006 §7 asks what process behaves differently if the pin is deleted; "the ADR would use a synonym" is the answer that section names as not an answer. And it reds every ADR commit that uses an unapproved word — the generated-index objection of ADR 0040 in another costume | **reject** |
| **a lint in the landing lane** | Vale (MEASURED, docs.vale.sh: a Go CLI; YAML rules; "block and inline code are ignored by default"; a `substitution` rule is exactly STE's not-approved → approved shape; `error` severity "will cause CI builds to fail"; packages exist for the Google and Microsoft guides, write-good, proselint). Not on this box (`command -v vale`: none) | a new tool on every seat, a `.vale.ini`, one YAML file per rule, a `make prose-check` door outside `tree-check` at warning level. Exit hatch: the rules are a table; a 30-line script reads the same table. ASSUMED: one session to stand up, seconds per run | **defer** — worth it only once the dictionary has enough not-approved rows to be worth a machine; the dictionary comes first |
| **writing guide only** | the dictionary and the three line rules live in an ADR; applied at review (the verify lane's checklist is the ADR's "done when") | one ADR; one row per coinage after. No new genre: a `docs/glossary.md` would be refused by the public-docs genre allowlist (`PublicDocsGenres` = adr, runbooks, notes.d, probes; "admitting a genre to the public tree is a reviewed code change") — an ADR is already the home for "the current statement of a decision, amended in place" (ADR 0040) | **adopt** — ADR 0067 (proposed) |

For the binary's lines the enforcement is cheaper than any of the above: each
rendering function already has a unit test, and a shape assertion on a
function's output is an ordinary test that names its process (the line the
coordinator reads), not a prose pin.

## 5. Prior art: controlled languages for instructions and for prompts

| name | type | what it controls | how enforced | lesson for posse | source (fetched 2026-10-04) |
|---|---|---|---|---|---|
| **ASD-STE100** | human-oriented CNL | 53 rules + ~900-word dictionary, one meaning per word, company glossary for technical names | writer training (C1), optional checkers, none endorsed | the dictionary shape; the glossary assumption; "not for general-purpose writing" | [asd-ste100.org FAQ](https://www.asd-ste100.org/STE_faq.html) |
| Caterpillar Technical English, IBM Easy English | human-oriented CNLs | same family: "Keep sentences short", "Only use dictionary-approved words", "Use only the active voice" | in-house | the class exists beyond aerospace; all are for maintenance/product docs | [Wikipedia, Controlled natural language](https://en.wikipedia.org/wiki/Controlled_natural_language) |
| **Attempto Controlled English (ACE)** | machine-oriented CNL; "a rich subset of standard English designed to serve as knowledge representation language"; last updated 2025-02-28 | fixed function words, user lexicon of content words; ambiguity removed by construction or by **deterministic interpretation rules** ("a relative clause always modifies the immediately preceding noun phrase"; "and binds stronger than or"); translates to first-order logic | a parser (APE) rejects what it cannot read | the other end of the spectrum: every sentence has exactly one reading because the reader is a parser. A prompt to an LLM has no parser to refuse it, so ACE's guarantee does not transfer | [attempto.ifi.uzh.ch](https://attempto.ifi.uzh.ch/site/description/), [ACE in a nutshell](https://attempto.ifi.uzh.ch/site/docs/ace_nutshell.html) |
| **Plain Writing Act of 2010** (US) | law | "writing that is clear, concise, well-organized, and follows other best practices appropriate to the subject or field and intended audience"; covered documents only; regulations excluded | **none**: "There shall be no judicial review of compliance or noncompliance" | a standard that names its audience and carries no gate is a writing guide; it still moved an industry | [govinfo PLAW-111publ274](https://www.govinfo.gov/content/pkg/PLAW-111publ274/html/PLAW-111publ274.htm) |
| Google developer documentation style guide | style guide + word list | voice, tense, person, a word list | none; "this guide contains guidelines, not rules … Depart from it when doing so improves your content" | a word list with no gate is how most shops do it | [developers.google.com/style](https://developers.google.com/style/) |
| Microsoft Writing Style Guide | style guide + A–Z | "Use bigger ideas, fewer words", "Write like you speak", "Get to the point fast", "Revise weak writing" | none | the opposite voice to STE (contractions, friendliness) — proof that "clear" is audience-relative | [learn.microsoft.com/style-guide](https://learn.microsoft.com/en-us/style-guide/top-10-tips-style-voice) |
| **Vale** | prose linter | YAML rules: substitution, existence, occurrence; ignores code; packages for the two guides above | CI, exit code by severity | the one mechanism that would realise a dictionary as a gate; a Go binary like the rest of this shop | [docs.vale.sh](https://docs.vale.sh/) |
| Anthropic, prompting best practices | prompt-engineering guide | "Claude responds well to clear, explicit instructions"; XML tags "reduce misinterpretation"; **"Golden rule: Show your prompt to a colleague with minimal context on the task and ask them to follow it. If they'd be confused, Claude will be too."** | none | the test posse already runs in costume: a work prompt is read by a seat with no context but the bead | [platform.claude.com](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices) |
| SimpleEnglish (aminblg), MIT | an agent skill that makes a model WRITE in STE; a 53-rule paraphrase; a mechanical linter; "no dictionary content, because the standard forbids reproduction" | its own linter: "81.3% fewer linter violations across 9 Claude models (144 generations)"; the authors' caveat: it "measure[s] rule obedience, not what a reader sees" | the linter | STE-as-output-style for a model is live practice; the measured number measures the measurer | [README](https://raw.githubusercontent.com/aminblg/simpleenglish/main/README.md) |
| Hermes Agent `simple-english` skill | same idea, Nous Research | "Do not apply it to marketing copy, blog voice, or brand writing — it deletes persuasion by design"; "no tool can guarantee STE compliance" | none | the "leave alone" column above, said by someone who tried | [hermes-agent docs](https://hermes-agent.nousresearch.com/docs/user-guide/skills/optional/creative/creative-simple-english) |
| STEMG white paper on AI (2026) | the publisher's position | "Plausibility must not be confused with verified compliance"; responsibility "remains with the human author" | — | the MEASURED/ASSUMED label is the shop's version of this sentence | [PDF](https://www.asd-ste100.org/assets/files/WhitePaper-ASD-STE100_and_AI.pdf) |
| Zambrini & Chiarello, MDTT 2025 and 2026 (CC BY 4.0) | STEMG members on Issue 9's terminology and on the dictionary's forty-year lexical management | — | — | a dictionary is maintained by "selective retention, consolidation, and limited removal"; a proposed word waits for the next issue | [Vol-3990/short24](https://ceur-ws.org/Vol-3990/short24.pdf), [Vol-4219/paper4](https://ceur-ws.org/Vol-4219/paper4.pdf) |

UNKNOWN: any published measurement that a controlled language improves an
LLM's instruction-following on long operational prompts. The two papers found
measure a model's *compliance when writing* STE, not a model's *comprehension
when reading* it. The STEMG's own FAQ claims STE helps "neural machine
translation engines, or Large Language Models" translate — a claim about
translation, offered without a number.

## 6. Questions for the operator discussion

Filed as one question bead, ranger-base-w6zpo, so the discussion has a home; one decision — accept,
amend or reject ADR 0067 — with these sub-points to rule on:

1. **Register for the free PDF?** Requires name, country, organisation, field of
   activity and a consent box (§1.4). Nothing in the recommendation needs it.
   Worth it only if the operator wants to read Part 2's entry format first-hand
   before seeding posse's own; the format is visible in public sources already.
2. **Where the dictionary lives.** ADR 0067's own appendix (recommended: no new
   genre, no second writer, amended in place) versus a `docs/glossary.md`, which
   needs the genre allowlist widened by a code change and would be a second
   writer on every coinage.
3. **Is ARGUED a label?** The close of ranger-base-26y03 coined it and the
   verify lane adopted it. The draft says yes, beside MEASURED and ASSUMED, with
   the meaning "reasoned from the code, not run".
4. **Does `WIP` get a not-approved row?** The draft says yes: a checkpoint
   commit is named by its bead id and the word "checkpoint", never `WIP`, since
   `main` carries it forever.
5. **The three line rules for binary output**: accept as a writing rule on new
   lines only, or also file the one measured offender (`turnOutcomeClause`, 36
   words, instruction joined to fact by an em-dash) as a code bead now.
6. **Vale later, or never.** The draft defers it. If the operator wants a gate
   from day one, the price is in §4 and the exit hatch is the table.

## Where this landed

- `docs/adr/0067-house-words-are-a-posse-dictionary-in-the-shape-of-ste.md` —
  proposed, operator discussion pending; carries the seed dictionary and the
  three line rules.
- This note. No ORDERS, PID, ADR body or binary string was changed.
