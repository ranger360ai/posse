# ADR 0067 — The harness's house words are a posse dictionary in the shape of ASD-STE100's; the standard itself is not adopted

*Status: accepted 2026-10-04 — operator ruling on ranger-base-w6zpo, the six sub-points at their recommended defaults (no STE registration; dictionary lives in this appendix; ARGUED is a label; WIP gets a not-approved row; the three line rules apply to new lines, the existing clause is a P3 bead; Vale later) · amended 2026-10-04 (ranger-base-mdg3d: appendix seeded in Part 2 row shape — labels, house words, not-approved words, line rules) · owner: architect · source
bead ranger-base-4wwy5 (spike, recommendation only) · evidence, the surface
map and the prior-art table in `docs/notes.d/ranger-base-4wwy5.md` · sits
beside ADR 0005 (the rung words it seeds from), ADR 0006 §7 (no prose pin
without a process), ADR 0022 (one writer per file) and ADR 0040 (one current
statement, amended in place)*

## Context

The operator asked whether ASD-STE100, the aerospace controlled language,
could clarify the English the harness runs on: standing orders, PIDs, the work
prompt, settle and pulse lines, bead titles and close comments, ADR prose.
Four English failures were named from one week. Measured (notes §2), they are
one dictionary failure — `WIP` in two commit subjects on `main`, correct by
the closer's orders and unfinished-looking to everyone else, because the word
has no agreed meaning here — one lineage failure (ADR 0019's rule stood after
its reason died, fixed by amending both), and two that are not language (a
reader that took a UI suggestion for typed text; a pulse line with no
timestamp).

STE has two parts. Part 1 is 53 writing rules — 20 words per procedural
sentence, one instruction per sentence, active voice, no `-ing` verbs. Part 2
is a dictionary of about 900 words, one meaning and one part of speech each,
with every rejected synonym listed beside its approved replacement, and
"technical names" admitted only from the organisation's own glossary. Posse
has no glossary. It does have a working controlled vocabulary with no written
home: the rung words NOTE, ASSUME, SPIKE, ASK, HANDOFF, REFUSE and the labels
MEASURED, ASSUMED, UNKNOWN, VERIFIED appear in 371 comments since 2026-09-27
with their one meaning each. The incidents are the words outside it.

## Decision

**D1. ASD-STE100 is not adopted** — neither its rules nor its dictionary, on
any surface. Its approved meanings collide with the shop's central verbs
(`close` is `bd close`; `check` is a verb and the name of fourteen Makefile
doors); its sentence caps would cut what the shop values in a close comment;
and the dictionary may not be reproduced, so nothing in a public tree can
check against it.

**D2. Posse keeps a dictionary in STE's Part 2 shape, as the appendix of this
record.** One term, one part of speech, one meaning, the file or record that
defines it, and the not-approved synonyms with their replacement. It is
amended in place like every ADR (0040); a coinage is one row, filed by the
bead that coins it. No new docs genre and no second index: the genre allowlist
stays as it is.

**D3. The dictionary governs the instruction surfaces, not the argument
surfaces.** It applies to ADR Decision sections, bead titles, the first line of
a close comment, PID `deny:`/Handoffs rows, and every line the binary writes.
Standing orders, NOTES, notes.d fragments, ADR Context and Alternatives, and
persona voice are out of scope: their reader needs the argument, and the two
agent skills that apply STE to model prose both warn that it "deletes
persuasion by design".

**D4. Every outcome word in a close comment carries a label.** "Clean",
"green", "done", "fixed" stand alone nowhere; they are MEASURED, ASSUMED,
ARGUED, VERIFIED or LANDED with what was run or reasoned. ARGUED joins the
label set with the meaning "concluded from reading the code, not from running
it" (coined on ranger-base-26y03, adopted by the verify lane on
ranger-base-ugf48).

**D5. Three rules for lines the binary writes that a human may read later**
(settle lines, pulse lines, refusal and warning lines) — the one surface whose
readers match STE's: someone with no way to ask back.
1. One fact per line. An instruction, if any, is its own clause after the
   fact, condition first.
2. State words are dictionary words only: settled, landed, parked, blocked,
   refused — never a synonym.
3. A line that can be read later than it is printed carries its timestamp.
A new line over 25 words is split. These are checked where the line is
rendered, by the function's own unit test, which names its process; no
tree-wide pin reads prose for this (ADR 0006 §7).

**D6. No mechanical gate over the dictionary yet.** A prose linter (Vale) is
the realisation if one is ever wanted — its `substitution` rule is exactly the
not-approved → approved row — and it is deferred until the appendix has enough
rows to need a machine. Exit hatch: the rows are a table; any script reads it.

## Consequences

- Price: this record and one row per coinage. ASSUMED: under an hour to seed,
  minutes per row after. No runtime key, flag, store or tool.
- The verify lane's checklist gains D4 and D5 for the closes it reads.
- ADR 0005's rung words acquire a second home; the appendix cites 0005 rather
  than restating the rung's procedure, so there is one writer of the meaning.
- Ruled 2026-10-04 (ranger-base-w6zpo, the six sub-points of notes §6): no
  registration for the PDF; `WIP` has its not-approved row (appendix); the one
  measured D5 offender (`turnOutcomeClause` in `internal/posse/dispatch.go`,
  36 words, fact and instruction joined by a dash) is the P3 code bead
  ranger-base-zt45t; Vale, if ever, as a warning-level door (D6).

## Alternatives rejected

- **Do nothing.** Price MEASURED this week: a verify pass explaining `WIP`, an
  architect session re-reasoning ADR 0019, a label coined ad hoc. Cheap; it is
  the bar. Rejected because the working vocabulary already exists and only its
  written home is missing — D2 costs less than one more such pass.
- **Adopt STE's Part 1 rules on standing orders and ADRs.** Median sentence in
  this persona's orders is 31 words; 74% exceed the cap. A rewrite of 160k
  words of ADR prose and the orders of every persona, ASSUMED weeks, against
  zero incidents a length or voice rule would have caught.
- **A tree-wide pin reading surfaces against the dictionary.** Against STE's:
  dead on licence. Against posse's: ADR 0006 §7's question has no answer
  ("the ADR would use a synonym" is not a process), and it reds every ADR
  commit that uses one — ADR 0040's generated-index objection in another
  costume.
- **A `docs/glossary.md`.** Refused by the public-docs genre allowlist unless
  a code change admits the genre; a second writer on every coinage.
- **Attempto-style controlled English for the work prompt.** ACE's guarantee
  — one reading per sentence — comes from a parser that refuses the rest. An
  LLM has no parser to refuse it; the guarantee does not transfer.
- **A Vale gate from day one.** A new tool on every seat for a dictionary of
  thirty rows. Deferred, not refused (D6).

## Appendix — the posse dictionary

This is the dictionary D2 keeps, in ASD-STE100 Part 2's row shape: one term,
one part of speech, one meaning, the record that defines it, and the
not-approved synonyms beside their replacement. Seeded 2026-10-04 by
ranger-base-mdg3d from the words already in use (notes §2: 371 comments since
2026-09-27); every coinage after it is one row, filed by the bead that coins
it, in the same commit as the coining. A "defined in" cell names the one
writer of the meaning — this appendix restates, it does not redefine. First
uses are MEASURED 2026-10-04 over the queue store's `issues.jsonl` (titles,
descriptions, comments; transcripts not read), so a word may be older than
its row says.

### A1. Outcome labels

The words D4 requires on every outcome in a close. Upper case, one per
claim, followed by what was run or reasoned.

| term | pos | meaning | defined in · first on the record | not approved → use |
|---|---|---|---|---|
| MEASURED | label | the number or fact came from a command that was run, and the claim carries its date and environment | ADR 0026 · rangerhq-53p (2026-08-18) | *clean*, *green*, *passes* standing alone → MEASURED + the command and its exit |
| ASSUMED | label | priced or reasoned, not run; the claim says so and names what would measure it | ADR 0026; the ASSUME rung's comment prefix, ADR 0005 · rangerhq-9te (2026-08-18) | *probably*, *should*, *expected to* as an outcome → ASSUMED + why |
| UNKNOWN | label | looked for and not found; the loud state, never a quiet default | ADR 0017 ("nobody measured it") · ranger-base-5b5 (2026-08-23) | *n/a*, *unclear*, *not applicable* for a fact nobody fetched → UNKNOWN + where you looked |
| ARGUED | label | concluded from reading the code, not from running it | this record, D4 · coined on ranger-base-26y03 ("on the record as an argument and not as a measurement"; its follow-up ranger-base-z9ct5 is titled "argued, not measured"), adopted by the verify lane on ranger-base-ugf48 | *cannot reach*, *clean* for an arm that did not run, *by construction* → ARGUED + the reading |
| VERIFIED | label | the verify lane re-ran the close's own acceptance, as written on the closed bead, and it held; the close comment reads `VERIFIED: <how>` | ADR 0006 (verify close) · rangerhq-beb (2026-08-17) | *done*, *fixed*, *confirmed* standing alone → VERIFIED + how, or the label that is true |
| LANDED | label | the commit is an ancestor of `main` (`git merge-base --is-ancestor`); a matching subject in `git log --grep` does not prove it | AGENTS.md "Landing the plane"; ADR 0051 (a citation is not a landing) · rangerhq-9fv (2026-08-20) | *merged*, *pushed*, *in* for a session branch → LANDED, or "committed on `posse/<session>`, not landed" |

### A2. House words

Rung words, states, and the nouns the harness is built from. Two of these
collide with STE's approved meanings, which is half of why D1 does not adopt
the standard: in this shop they mean what the rows say.

| term | pos | meaning | defined in | not approved → use |
|---|---|---|---|---|
| close | verb | `bd close <id>`: the bead's own acceptance is met and the close comment carries the labels above; nothing else is "closed" (STE's *close* — shut — is not this word) | AGENTS.md; ADR 0006 | *finish*, *complete*, *wrap up* a bead → close; *done* as the verb → close, with VERIFIED or MEASURED on the comment |
| check | verb; name | as a verb, run a door or a pin and read it; as a name, the fourteen Makefile doors are spelled `make <x>-check` and `make tree-check` runs them all. The noun form is a door's spelling only | AGENTS.md "Landing the plane", `make tree-check`; Makefile | *a check* for a pin or a test → pin, door; *checked* for a claim nobody ran → ARGUED or MEASURED |
| NOTE, ASSUME, SPIKE, ASK, HANDOFF, REFUSE | rung | the escalation rungs of the work prompt, in that order of cost; each has one comment prefix and one stop/continue | ADR 0005 (the procedure); ADR 0026 (SPIKE) | *escalate*, *flag*, *raise* without a rung → the rung's name |
| BLOCKED | rung outcome | a question bead holds this bead out of `bd ready` until it is answered; comment `BLOCKED: <need> → <qid>` | ADR 0005 (ASK) | *waiting on*, *stuck*, *pending* → BLOCKED + the question bead |
| REFUSED | rung outcome | a hard risk line or an unrealizable gate stopped the work; comment `REFUSED: <line> — <what would be needed>` | ADR 0005 (REFUSE) | *declined*, *won't*, *skipped* for a risk line → REFUSED + the line |
| DIVERGED | comment class | a build that had to leave its design, recorded on the builder's own bead before the code is written, naming the ADR | developer PID (`examples/agents/developer.md`); ADR 0013 | *deviated*, *changed the plan*, *adapted* → DIVERGED: + what and why |
| land, landed | verb | the launcher fast-forwards `posse/<session>` onto `main` when the bead closes; nothing else lands | AGENTS.md "Landing the plane" | *merge*, *merged* for a session branch → land, landed; *landed* for a commit that merely exists → LANDED only after the ancestor check |
| settle, settled | verb; adjective | the herdr status is `idle` or `done` (`settledStatus`, `internal/posse/govern.go`); a *settle line* is the dispatcher's report that a seat has | ADR 0013 | *idle* as the reading of a seat in a report → settled (idle is one of the two herdr statuses, not the reading); *finished*, *exited*, *quiet* → settled |
| park, parked | verb | a bead deferred (`bd defer`); a dated park re-surfaces at its date, a dateless one after `attn_parked_age` (14 days) | ADR 0029 (G3 parked row); ranger-base-nkjjg, ranger-base-pm5zo | *deferred* as the state word in a line the binary writes → parked; *snoozed*, *shelved*, *on hold* → parked |
| retire, retired | verb | a landed worktree whose session is proven gone is removed, tip kept at `refs/posse/retired/<branch>` | ADR 0058 | *cleaned up*, *deleted*, *pruned* for a worktree → retired |
| pin, tree-wide pin | noun | a test whose fixture is the repository tree and which names the process it protects and the decision it guards | ADR 0006 §7; ADR 0051 | *check*, *guard*, *lint* for such a test → pin |
| door | noun | a Makefile target that runs one tree-wide pin under a `-run` filter, or the tool the pin wraps (`make fmt-check`, `make notes-check`) | AGENTS.md, `make tree-check` | *target*, *shortcut*, *check* for one of these → door |
| gate | noun | a refusal realised at commit, push or launch: a git hook or a PATH shim; a gate refuses, it never edits | ADR 0002; ADR 0009 | *hook* for the refusal (a hook is the mechanism, not the gate) → gate; *block*, *guard* → gate |
| cage | noun | the process sandbox a seat runs in: seatbelt, or the L4 container | ADR 0059; ADR 0002 | *sandbox* in a line the binary writes → cage; *jail*, *container* for the seatbelt case → cage |
| seat | noun | one launched session: a pane, a worktree, a PID | ADR 0013 | *session* when the pane or PID is meant, *agent*, *worker* → seat |
| pass | noun | one iteration of the dispatch watch loop | ADR 0063 | *tick*, *cycle*, *round* for the dispatch loop → pass (the pulse has ticks) |
| lane | noun | the label a persona takes work from; work is handed to a lane, not a person | ADR 0006 §1 | *team*, *queue*, *the <name> lane* for a person → lane, by label |
| class | noun | feature, bug, debt or unclassified (`ClassCensus`, `internal/posse/beadpulse.go`); carried by `-t` or `-l debt` on the bead | ADR 0006 (verify census and the HANDOFF rung); ADR 0005 | *type*, *kind*, *category* for this → class |
| checkpoint commit | noun | a commit made before a mutation so the prior state is recoverable, with the bead id and the word *checkpoint* in its subject; it lands and is never squashed | this record (D2, A3) | *WIP* → checkpoint; *save point*, *snapshot*, *stash* → checkpoint commit |

### A3. Not-approved words

STE's Part 2 lists a rejected word on its own row with the approved
replacement. These are the words the appendix exists for: each has been
read two ways on a surface D3 governs.

| not approved | where it was read | use instead | source |
|---|---|---|---|
| **WIP** | a commit subject on `main` (`73b9b1e4`, `b13ab874`, ranger-base-pm5zo, 2026-10-03: pre-mutation checkpoints the closer's orders require, read by everyone else as unfinished work) | *checkpoint* + the bead id: `<id>: checkpoint — <what is about to change>`; `main` keeps the subject forever | ranger-base-pm5zo; ranger-base-w6zpo (ruling, sub-point 4) |
| **clean, green, done, fixed** standing alone | a close comment ("arms 2 and 3 clean" on ranger-base-26y03, where one arm was run and two were reasoned) | the label that is true — MEASURED, ARGUED, VERIFIED or LANDED — with what was run or read | ranger-base-26y03; ranger-base-z9ct5; D4 |
| **merged** for a session branch | close comments and settle lines | *landed* (the launcher merges; nobody else does), or LANDED after the ancestor check | AGENTS.md; A2 |
| **idle** as the reading of a seat | a report or settle line | *settled* — idle is one of the two herdr statuses the reading is made from | `settledStatus`, `internal/posse/govern.go`; ADR 0013 |
| **check** as a noun for a pin or test | bead titles, close comments | *pin*, or *door* for the Makefile target; `-check` is the door's spelling only | A2; ADR 0006 §7 |
| **deferred** as a state word in a line the binary writes | a G3 row or pulse line | *parked* (the bd status is `deferred`; the dictionary word for the state is parked) | ADR 0029; A2 |

### A4. Rules for lines the binary writes

D5, as the appendix's rules section, for settle lines, pulse lines, refusal
and warning lines — any line a human may read later than it is printed, with
no way to ask back.

1. **One fact per line.** An instruction, if any, is its own clause after the
   fact, condition first ("if X, do Y"), never joined to the fact by a dash.
2. **State words are dictionary words.** A state in such a line is *settled*,
   *landed*, *parked*, *blocked*, *refused* or another A2 row, spelled as the
   row spells it; never a synonym, and never the raw status string when the
   dictionary has a word for it (A3: *idle*, *deferred*).
3. **A line that can be read later carries its timestamp.** The timestamp is
   the line's own, not the pass's, in the format its neighbours already use.

A new line over 25 words is split. The rules bind lines written after
2026-10-04; the one measured offender before it is ranger-base-zt45t
(`turnOutcomeClause`, 36 words). Each rule is checked where the line is
rendered, by the rendering function's own unit test, which names its process
(ADR 0006 §7); no tree-wide pin reads prose for this, and this appendix is
not a fixture.
