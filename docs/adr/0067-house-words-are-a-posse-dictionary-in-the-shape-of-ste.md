# ADR 0067 — The harness's house words are a posse dictionary in the shape of ASD-STE100's; the standard itself is not adopted

*Status: accepted 2026-10-04 — operator ruling on ranger-base-w6zpo, the six sub-points at their recommended defaults (no STE registration; dictionary lives in this appendix; ARGUED is a label; WIP gets a not-approved row; the three line rules apply to new lines, the existing clause is a P3 bead; Vale later) · owner: architect · source
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
- Open to the operator (notes §6): register for the free PDF or not; whether
  `WIP` gets its not-approved row; whether the one measured D5 offender
  (`turnOutcomeClause` in `internal/posse/dispatch.go`, 36 words, fact and
  instruction joined by a dash) is filed as a code bead now or left to its next
  edit.

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

## Appendix — the posse dictionary (seed)

Approved term · part of speech · meaning · where defined. A not-approved
synonym is listed with its replacement. Rows marked *owner to confirm* are
the architect's reading and want one line from the owning record.

| term | pos | meaning | defined in |
|---|---|---|---|
| MEASURED | label | the number or fact came from a command run, with date and environment | ADR 0026 |
| ASSUMED | label | priced or reasoned, not run; says so out loud | ADR 0026 |
| UNKNOWN | label | looked for and not found | ADR 0026 |
| ARGUED | label | concluded from reading the code, not from running it | this record, D4 |
| VERIFIED | label | the verify lane re-ran the close's acceptance and it held | ADR 0006 §3 |
| LANDED | label | the commit is an ancestor of `main` (`git merge-base --is-ancestor`) | AGENTS.md, "Landing the plane" |
| NOTE, ASSUME, SPIKE, ASK, HANDOFF, REFUSE | rung | the escalation rungs of the work prompt, in that order of cost | ADR 0005 |
| BLOCKED | rung outcome | a question bead holds this bead out of `bd ready` | ADR 0005 |
| DIVERGED | comment class | a build that had to leave its design, on its own bead | ADR 0006 §2 |
| land, landed | verb | the launcher fast-forwards `posse/<session>` onto `main` when the bead closes; nothing else "lands" | AGENTS.md |
| settled, settle | adjective, verb | the herdr status is `idle` or `done` (`settledStatus`, `internal/posse/govern.go`); a *settle line* is the dispatcher's report of one | ADR 0013 |
| park, parked | verb | a bead deferred with a `defer_until`; re-surfaces at that date | ranger-base-pm5zo, ranger-base-nkjjg |
| retire, retired | verb | a landed worktree whose session is gone is removed | ADR 0058 |
| pin, tree-wide pin | noun | a test whose fixture is the repository tree and which cites the decision it guards | ADR 0006 §7, ADR 0051 |
| door | noun | a Makefile target that runs one tree-wide pin under a `-run` filter | AGENTS.md, `make tree-check` |
| gate | noun | a refusal realised at commit, push or launch: a git hook or a PATH shim | ADR 0002, ADR 0009 |
| cage | noun | the process sandbox a seat runs in (seatbelt or L4) | ADR 0059 |
| seat | noun | one launched session: a pane, a worktree, a PID | ADR 0013 |
| pass | noun | one iteration of the dispatch watch loop | ADR 0063 |
| lane | noun | the label a persona takes work from; work is handed to a lane, not a person | ADR 0006 §1 |
| class | noun | feature, bug, debt or unclassified (`ClassCensus`, `internal/posse/beadpulse.go`) | ADR 0006 §3 (verify census) *owner to confirm* |
| checkpoint commit | noun | a commit made before a mutation so the prior state is recoverable; named by its bead id and the word checkpoint | this record |

Not approved: **WIP** (use *checkpoint* with the bead id; `main` keeps the
subject forever) · **clean, green, done, fixed** standing alone in a close
(carry MEASURED / ARGUED / VERIFIED / LANDED and what was run) · **merged**
for a session branch (use *landed*; the launcher merges, nobody else) ·
**idle** for a settled seat in a report (use *settled*; idle is one of the two
herdr statuses, not the reading) · **check** as a noun for a pin (use *pin* or
*door*; `-check` is the door's spelling only).
