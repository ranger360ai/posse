# ADR 0006 — Handoff shapes: collaboration as beads, nothing else

*Status: accepted 2026-08-18 · owner: architect · amended 2026-09-01 (§2/§3: the "done when" row is best-effort, ranger-base-ziy47) · amended 2026-09-01 (§1: hand to the lane, not the person — `-a` only for the five-item allowlist; §2 rows and ADR 0005's HANDOFF rung follow, ranger-base-tpc41) · amended 2026-09-02 (§1–§4: every bead carries a class — feature / bug / debt — and a verify close files ONE findings bead; operator ruling, ranger-base-zbd51)*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> Persona names are restated as roles; an instance's own cast is the
> operator's to map onto them.

## Context

Every crew PID has a `## Handoffs` section that says *who* takes from
whom (the developer from the architect, QA from the developer's closes,
security from anyone `-l security`…). None says *what a handoff looks
like* — which bead, which label, which relation — and nothing in dispatch
notices when a handoff that should exist doesn't. The visible symptom: a
QA persona whose raw material is closes is never dispatched, because a
close creates no work; security findings sit untriaged; a design that got
built with a divergence leaves no trace the metrics can count.

ADR 0005 gave every persona the HANDOFF rung: *create a bead for the
other persona, `discovered-from` yours, comment it, continue.* This ADR
decides the **shapes** that rung produces for the four collaborations,
where each kind of message goes, and the one dispatch affordance the
substrate is missing.

Checked: `bd mail` delegates to an external provider (`mail.delegate`,
`BEADS_MAIL_DELEGATE`); with none configured, a persona typing
`bd mail send` reaches nothing.

## Decision

**1. Three channels, by what the message is about — no fourth.**

| the message is… | goes to | why |
|---|---|---|
| about *this* bead (progress, a decision, `ASSUMED:` `BLOCKED:` `REFUSED:` `DIVERGED:`) | `bd comments add <id>` | stays with the work; ADR 0005 Context says "read the comments" |
| *work for someone* (a fix, a verification, a design, a question) | a **new bead**, the lane's label, `--deps discovered-from:<id>` (+ `blocks:` when order matters); **no `-a`** unless the work needs that person — the five cases in the §1 amendment below *(amended 2026-09-01, ranger-base-tpc41; the row read `-a <persona>, their label` before)* | the only thing dispatch routes and `bd ready` shows; dispatch seats it on whichever seat in the lane is free (ADR 0020 §2) |
| a conversation | **nowhere** — `bd mail` is unused | no provider, and mail is invisible to dispatch and cockpit; a chat that matters becomes a comment or a bead |

Collaboration state therefore lives entirely in beads: `discovered-from`
is the handoff edge, `blocks` is the ordering edge, labels route, and
comment prefixes are the greppable events (the metrics catalog counts
them).

*(Amended 2026-09-01 from ranger-base-tpc41. Wording proposed by the
coordinator on ranger-base-kcnc6 and confirmed by the operator on
ranger-base-vzgk9; the rule below is that wording, verbatim.)* **Hand to
the lane, not the person.**

> Hand to the LANE, not the person. File `-l code` (or the lane's label)
> WITHOUT `-a`; dispatch seats the bead on whichever seat in that lane is
> free (ADR 0020, availability-first). Name a person with `-a` only when
> the work needs THAT person: their own session tree, their own close,
> their own ORDERS.md, a ruling they alone can make, or a skill only
> their PID carries; and say which of those it is in the first line of
> the description. A bead that names a person for any other reason is
> re-filed to the lane by the coordinator.

Why the row above had to change: ADR 0020 made a lane a set of seats
and an explicit assignee "a lane of one that never falls through" (§2
there). §1 as written filed every handoff `-a <persona>`, so every
handoff was a lane of one — the second and third seats in a lane could
receive only what the harness filed unassigned (verify-after, ADR 0020
§3), and a named persona's backlog waited on that persona's
availability while peers sat idle. The two ADRs were consistent only
in a one-seat-per-lane shop. The fix is at the filer, because ADR 0020
rejected the fix at dispatch on purpose: silent rerouting hands work to
the wrong actor, and the five cases below are exactly the beads where
rerouting is wrong.

The allowlist, one line each on what "needs that person" means:

1. **their own session tree** — the work is in a worktree, branch or
   index only that persona's session holds (a stranded commit, a dirty
   tree at close, ADR 0041); another seat cannot reach it.
2. **their own close** — the bead is that persona's own bead to close,
   reopen-and-close, or amend the close of (a settle-open, a close
   reason only they can write); a second seat closing it signs a claim
   it did not make.
3. **their own ORDERS.md** — a persona's memory is single-writer, the
   same fact that makes a persona one serial seat (ADR 0020 §4); an
   append or a compaction is theirs.
4. **a ruling they alone can make** — the ADR owner's amendment, the
   coordinator's placement, the operator's decision (`-l question`,
   ADR 0005's ASK rung keeps its `-a <operator>`: the operator is not a
   seat); the deliverable is the judgement, and the judge is named.
5. **a skill only their PID carries** — `skills:` are per PID, so the
   lane's other seats would answer without the reference the work
   needs.

"Say which of those it is in the first line of the description" is
what makes the rule checkable without a parser: the coordinator's groom
(§2, last row) reads that line, and a person-named bead whose first
line names no case is re-filed `-l <lane>` with the `-a` cleared. That
check is a person reading a line, not code — stated as such.

What follows from it, in this record and the ones it touches:

- **§2 rows lose their `-a`s**, stamped in place: the design→build
  beads are `-l code`; a `DIVERGED:` handoff is `-l architecture`, `-a`
  the ADR's owner only when the divergence needs their ruling (case 4);
  the verify bead is `-l qa`, assigned only when config
  `verify_assignee:` pins a seat (ADR 0020 §3 — the code's default was
  already nobody); QA's escape bug is `-l code` with the closed bead's
  id in the description, because a fix is lane work and the closer is
  not on the list; a finding is `-l security`.
- **§4's example rows name lanes.** `examples/agents/*` Hand-to rows
  read `the code lane · -l code · …` instead of a role, and the
  template line reads `bd create "<title>" -l <label> --deps
  discovered-from:<id>` — no `-a`. Take-from rows still name the
  sender's role: they describe where a bead comes from, not who it is
  assigned to.
- **ADR 0005 §2's HANDOFF rung drops `-a <persona>`**, stamped in the
  same commit as this amendment. The ladder the harness renders
  (`EscalationLadder`, `dispatch.go`) followed on the code bead cut
  from this amendment — landed c067486 (ranger-base-uzw11), pinned by
  `TestEscalationLadderHandoffFilesToTheLane`. SPIKE already files
  `-l <runner's lane>`; ASK keeps `-a <operator>` (case 4).
- **Instance PIDs are the operator's to promote** (§4, unchanged). This
  crew's Hand-to rows move from `<developer> · -l code` to `the code
  lane · -l code` when they do.

Alternatives rejected:

- *Keep `-a` and let dispatch fall through to the lane when the
  assignee is busy.* ADR 0020 §2 chose the opposite and said why; a
  bead naming a person for one of the five cases must not be reseated
  on a free peer, and dispatch cannot tell the five from the rest.
- *An assignee spelling for a lane (`-a code`).* bd's assignee is free
  text, so it would parse, and the cockpit's holder join would show a
  seat no persona has. Labels already are the lane (ADR 0020 §1); a
  second vocabulary for the same set is triple-implementing the
  substrate (ADR 0001).
- *The harness strips `-a` at dispatch when the first line names no
  allowlist case.* The clever one: it needs a parser for prose, and a
  wrong strip unroutes one of the five real cases silently, at exactly
  the beads where that is most expensive. The coordinator's groom is
  one person reading one line; automate it when the count of re-files
  says so, not before.
- *Change the PIDs and leave §1 as written.* The record would keep
  saying the thing the lanes contradict, and the PIDs cite this section
  by number.

Measured (live store, 2026-09-01, after the ranger-base-kcnc6 groom
executed): 127 open beads, 100 in the code lane, 5 of those naming a
person — all one developer, filed before the rule; 19 open beads name
anyone at all. Assumed: that the coordinator's groom reads the first
line (process, not code, and nobody has measured the re-file count yet);
that the five cases are the whole list — the operator confirmed the
wording, not a census of beads that needed a person; and the reading of
"their own close" as the persona's own bead, not a bug found in it.

*(Amended 2026-09-02 from ranger-base-zbd51; operator ruling of
2026-09-02, recorded by the coordinator, after a day that filed 111
beads against 86 closes with QA's per-finding filing the largest line.
Two rules, one field.)* **Every bead carries a class, and a verify close
files one bead.**

**The class.** Every bead is one of three — *feature*, *bug*, *debt* —
and the operator tracks them separately (the raw open count rose when
the crew worked well, because reviews the operator commissioned
*discover* debt, and the number moved the wrong way):

| class | means | the field |
|---|---|---|
| **feature** | new capability or behaviour the operator asked for | bd `issue_type` = `feature` (`-t feature`) |
| **bug** | a live defect against a stated rule, or an observed failure | bd `issue_type` = `bug` (`-t bug`) |
| **debt** | pins, comment/doc rot, audit findings, renames, hygiene, records work | the label `debt` (`-l debt`); type stays whatever it is (usually `task`) |

**Which field is authoritative:** the type, then the label, in that order.
A bead whose `issue_type` is `feature` or `bug` *is* that class, whatever
labels it carries; a `debt` label on such a bead is a filing error the
groom clears. A bead whose type is anything else — `task` (what `bd
create` stamps when nobody passes `-t`: 1040 of 1516 beads on 2026-09-01),
`chore`, `epic`, `decision` — is classed by the `debt` label alone, and
without it is **unclassified**. Unclassified is a visible bucket the
scorecard reports beside the three, never an error and never inferred:
a guess here would make the operator's numbers lie in exactly the
direction they were lying before. One reader for the rule, in code,
shared by every surface that reports it (the class helper cut with
ranger-base-dwlb1's scorecard work); no second spelling.

**Who sets it:** the filer that knows, at filing — a class recovered
later is the groom's work, and the groom is a person reading beads.

| filer | sets the class how |
|---|---|
| a persona filing by hand — the HANDOFF rung, a spec, a design, a finding | names it: `-t feature`, `-t bug`, or `-l debt`. The class of *what the bead is*, not of the bead it came from |
| the SPIKE and ASK rungs | inherit the bead they hang off: the ladder renders the parent's own flag (`-t feature` / `-t bug` / `-t task -l debt`; nothing when unclassified) in place of today's `-t task`. A spike or a question is a sub-deliverable of its parent's class, and the persona typing it has nothing more to know |
| the verify-after filer (§3) | inherits the close it verifies; a batch takes its most urgent class in the order bug › feature › debt › unclassified — the same rule that already picks the batch's priority. An unverified feature is still open feature work; a fourth "verify" bucket would hide that |
| design → build beads (§2, first row) | inherit the design bead's class — a design is nearly always `feature`; the architect names `-t bug` when the design fixes a rule the code breaks |
| QA's findings bundle (below) | `-l debt`; the exception bead `-t bug` |
| an audit's findings (the ADR-adherence audit's shapes: DRIFTED / UNREALIZED / UNGOVERNED / adheres-unpinned) | `-l debt` — "audit findings" is in the definition — except a DRIFTED rule whose defect is *live* in one of the three domains below, which is `-t bug` |
| the coordinator recording a ruling | `-t feature`: a ruling is behaviour the operator asked for |
| the groom | applies the field to the existing open set (the product persona's groom bead of 2026-09-02, ranger-base-ppc85, is the backfill); reclassifies with `bd update -t <type>` / `--add-label debt` / `--remove-label debt` |

**One bead per verify close.** The ruling, which the operator believed
was already the rule:

> A verify close files ONE bead carrying all its findings — file:line
> and the bead each escaped from, for every finding — labelled `debt`.
> Only a LIVE defect in money, constitution, or dispatch correctness
> gets its own bead, at P1/P2, named as such. Everything else rides the
> bundle.

The shape, so it can be checked without the ruling in hand:

- **Trigger:** a verify bead (§3) closes with one or more findings. No
  findings → `VERIFIED: <how>` and no bead at all.
- **One bead:** title opens with the verify bead's id and the finding
  count; label = the verified close's lane (`code`, or `devops` when
  the close was `-l devops`; a batch spanning both is `-l code`) plus
  `debt`; `--deps discovered-from:<verify id>`; priority = the most
  urgent finding's (P3 unless one earns more). Description: one line
  per finding — `file:line` · what fails · the bead it escaped from ·
  the repro or the failing test. "No repro, no bug" holds per line: a
  finding without one is a comment, not a line. A bundle that spans a
  second lane is fixed by its lane and the stray line is HANDOFFed by
  the fixer — one hop, and rarer than a second bead per close.
- **Then** QA closes the verify bead `escape`, as today.
- **The exception, whole:** a finding that is a *live* defect — reproduced
  on the installed binary or the promoted constitution now, not a pin
  gap, not a comment — in **money** (spend, caps, the blind-meter park,
  credentials, egress that bills; ADR 0018/0019/0042), **the
  constitution** (the promoted set of ADR 0015 §1 and the gates that
  fence it), or **dispatch correctness** (a bead seated on the wrong
  actor, seated twice, or never seated — dispatch, lanes, verify-after,
  the watermark; ADR 0013/0020, §3 here) files its own `-t bug` bead at
  P1 or P2 with the domain in the title, and is *named* in the bundle
  by id, not restated. Three domains is the whole list; widening it is
  the operator's ruling, not the filer's judgement — "is this one
  serious enough for its own bead?" is the question the ruling removed.

What this changes downstream: §2's build→verify row and §3's filer
trailer now say the bundle, not "a bug bead per close" (the trailer had
also kept `-a <closer>` after the 2026-09-01 amendment struck it — the
code bead cut here retires both); the QA PIDs' Hand-to rows read the
bundle (§4, the instance's to promote); `escapes-caught` still counts
`escape` closes and is unchanged, while `bugs-with-repros` now counts
bundles a developer could reproduce, so its denominator falls — say so
where it is read, do not re-inflate it.

Alternatives rejected:

- *`-t chore` for debt* (bd has the type). It would replace `task` on
  every debt bead rather than add to it, so "untyped" and "debt" would
  compete for one slot, and `chore` reads as small hygiene where the
  definition includes audit findings and records work. The label is
  additive, greppable (`bd list -l debt`), and the word the operator
  used.
- *A label for all three* (`-l feature`, `-l bug`, `-l debt`). One
  vocabulary, but 71 of 153 open beads already carry `-t bug` and the
  type is what `bd create -t` and `--validate` know; a second spelling
  of the same fact is triple-implementing the substrate (ADR 0001).
- *A parent epic per verify close, one child per finding.* The count
  the operator measures is beads created; an epic adds one and removes
  none. The bundle's lines are the children, at zero beads each.
- *Let the harness bundle* — verify-after reads QA's escape beads and
  folds them. The harness files exactly one shape (the verify bead) and
  never closes or re-files a persona's beads (ADR 0013 §4's absence
  rule, pinned by ranger-base-q8dhm); the rule belongs at the filer, and
  the filer is a person reading their own findings.
- *One bundle per lane per verify close.* Honest, but mixed-lane
  verifies are the rare case (assumed, not measured) and the second
  bead is the amplification being cut; the HANDOFF hop covers it.
- *"Any live defect gets its own bead."* The definition of "live and
  serious" is the judgement that produced 1.3 beads per close; three
  named domains make the exception a lookup.
- *Infer the class from the graph or the title* (`discovered-from` a
  `-l qa` bead → debt; "pin" in the title → debt). Right often, wrong
  silently; the same rejection as §3's intent inference, for the same
  reason.

Measured (live store, 2026-09-02, 1701 beads, `--limit 3000`): 153 open
— 71 typed `bug`, 1 `feature`, 3 `chore`, 78 `task`; **0 carry `debt`**,
so every reader of the class is reading the groom's backfill until
ranger-base-ppc85 closes. Today's inflow at the time of writing: 120
created, 88 closed, 10 `escape` closes; the two QA seats created 26 of
the 120. The `bug` type is over-broad as filed — a stale model id in
NOTES.md and a doc's wrong count are typed `bug` today — which is what
the type-wins rule and the groom's `-t task --add-label debt` are for.
Assumed: that mixed-lane verify closes are rare; that a verifier can
tell "live in three domains" from "a pin gap" without a second ruling
(the domains are named by ADR, and a wrong call costs one bead, not a
missed defect — the bundle still carries the line); that inheriting the
close's class onto the verify bead is what the operator's "open by
class" wants to see (a verify bead is transient, and the alternative
was a fourth bucket he did not ask for).

**2. The four shapes.**

| handoff | trigger | shape | closes when |
|---|---|---|---|
| **architect → developer** (design → build) | ADR committed | implementation beads `-l code --deps discovered-from:<design>` *(no `-a`: §1 amendment of 2026-09-01)*, `blocks:` between them for order, ADR path in each description; the design bead closes when the beads exist (not when they're built) | each build bead: on the closer's word + the verify bead (below). A build that must diverge: comment `DIVERGED: <what/why>` on the *build* bead; if it changes the design, HANDOFF `-l architecture` (`-a` the ADR's owner only when their ruling is the deliverable — §1 case 4) |
| **developer/ops → QA** (build → verify) | a bead with a label in config `verify_labels:` (default `code, devops`) is **closed** | one verify bead `verify: <title>` `-l qa --deps discovered-from:<closed id>` (unassigned unless config `verify_assignee:` pins a seat — ADR 0020 §3; §1 amendment of 2026-09-01), description = closer, `close_reason`, commits (`git log --grep <id>`), and the closer's PID "done when" row where one matches — otherwise the whole `## Intents` table, marked unmatched *(§3 amendment of 2026-09-01)* *(at `verify_batch:` N > 1: one bead per N closes — shape in the §3 amendment)* | QA closes it "verified" (comment `VERIFIED: <how>`), or files **one** findings bead — `-l <the close's lane> -l debt --deps discovered-from:<verify id>`, one line per finding with file:line, the escaped-from id and the repro; a live money / constitution / dispatch-correctness defect alone gets its own `-t bug` P1/P2 bead (§1 amendment of 2026-09-02, ranger-base-zbd51; the row read "a bug bead `-l code` per close" before) — and closes theirs `escape`; the closed bead is never reopened by a persona (operator's call) |
| **anyone → security** (finding → triage) | anything that smells like exposure, at any time | bead `-l security --deps discovered-from:<id>` (no `-a` — §1 amendment of 2026-09-01), **priority = severity**: P0 exploitable now · P1 credential/exposure reachable · P2 hardening · P3 note; the security persona never edits — its output is beads: fixes `-l code` / `-l devops`, accepted-risk decisions ASK the operator (`-l risk`, ADR 0005) | fix bead closes → verify shape applies (it's `-l code`); a P0/P1 finding also comments `SECURITY:` on the origin bead so its holder sees it |
| **operator/product grooming** (cadence) | one `-l groom` bead per week assigned to the product persona, filed by the operator or their scheduling automation (posse does not schedule; `--watch` dispatches, it doesn't create) | the product persona re-prioritises, splits, labels (`tier:` per ADR 0003), files `-l architecture` beads where design precedes build, closes with `bd comments add` listing what moved | close = queue is honest for the week; the `queue-honesty` metric reads it |

**3. The one dispatch affordance: verify-after.** Convention alone would
leave the QA persona idle the first time a closer forgets. So the harness
files the verify bead: each dispatch pass (and `posse ready`) scans beads
with a `verify_labels` label closed since the last pass and, if none has
a `-l qa` child `discovered-from` it, creates one as above (comment on
the closed bead: `verify filed: <qid>`). A closer that files it first is
seen and not duplicated. Config `verify_labels:` (empty = off),
`verify_assignee:` (the QA persona). This is config-driven and one rule;
it is not a workflow engine. *(Amended 2026-08-20: a `--dry-run` pass
files nothing — dry-run shows routing without acting, and filing a bead
is acting. The code said so from the start; the record now does too.)*

*(Amended 2026-08-28 from ranger-base-skgs; the test amended 2026-08-30
from ranger-base-5fyg.)* One close is exempt from the scan: one whose
`close_reason` matches the scorecard's reject vocabulary (`invalid`,
`duplicate`, `dup`, `wontfix`, `won't fix`, `not a bug`) **and** that no
commit names (`git log --grep <id>` in the bead's repo). A rejected
close is not a claim about working software, so the verify bead it
would mint has one reachable verdict — "nothing was built" — at a full
QA session's price. Two conditions because each alone lies, and a false
exemption here is unrecoverable: the watermark advances past it, so the
QA session is cancelled, not deferred. The vocabulary is matched as
whole words, plurals included, never substrings ("dedupes",
"invalidation" and "duplicated" are this shop's fix vocabulary, not
rejections) — yet words alone still read "the retry no longer files a
duplicate bead" as a rejection; the commit trail is the one signal git
writes rather than prose the closer typed. Commitlessness alone would
exempt every doc-only or already-working close, which still earn
verification. Where git cannot answer — no checkout, no git — the close
is not exempt: doubt files the bead. Both the skip and a reject-worded
close held in by its commits are named on the pass's stdout. A close
with no reason at all still earns its verify bead — unexplained is not
rejected. The limit is stated where the rule is: a closer who leaves
the rationale in a comment instead of `bd close -r <reason>` is
invisible here, and that half is process, not code. Measured
(ranger-base-l2qv, live store 2026-08-30): of 520 closed
`code`/`devops` closes carrying a reason, five match the vocabulary;
four stay exempt — all genuine "nothing was built" closes — and one, a
P1 fix whose reason opens "verify-after dedupes…", stops being exempt.
That one bead is the whole behaviour change, and it is exactly the
class this shop verifies hardest.

*(Amended 2026-08-27, from ranger-base-f7pk/bh7q.)* Config gains
`verify_batch: N` (default 1) and `verify_batch_age:` (default 24h). At
N=1 the rule is exactly as written above, byte for byte; the seed config
ships the key commented out, so N > 1 is the operator's move
(ranger-base-bah7). At N > 1 one verify bead answers N closes:

- title `verify N closes: <id>, <id>, …` (a single close keeps
  `verify: <title>` unchanged);
- one `discovered-from` edge per close; priority = the batch's most
  urgent close;
- description = N sections, each exactly the block §2 quotes, under one
  trailer; `verify filed: <qid>` is commented back on every close.

The dedupe is per close, not per bead: each section opens with the
harness's own marker line, so a batched orphan from a timed-out create
still answers every close in it next pass. A trailing partial batch is
**held, bounded**: filed only when its oldest close reaches
`verify_batch_age:`. Both obvious alternatives fail — filing every
pass's leftovers makes N a ceiling, not a quantum (most passes see one
close, so `verify_batch: 4` would still file 1:1 and buy nothing), and
holding unbounded means a shop that goes quiet three closes into a
batch of four never verifies those three. Held closes need no new
store: the watermark does not advance past one, so the pending set is
exactly the closes it has not passed.

Batching is not a coverage cut — the same work is verified, in one
session instead of N; what N divides is the *filing* amplification.
Measured (ranger-base-1t7r): half of verify runs file real follow-up
work (`qa → code` = 0.49), so the 1:1 gate put the queue's branching
factor at ρ = 1.14, 90% CI [1.02, 1.25], which grows without bound at
any headcount; N=4 puts ρ at 0.875. The alternative rejected — dropping
`code` from `verify_labels:` — gets the same ρ for free but buys it by
cutting the catch, and the 0-reopens record is what this gate pays for.
§5 stands unchanged: batched or not, the verify bead never holds any
close.

*(Amended 2026-09-01 from ranger-base-ziy47; measured by
ranger-base-kvecg.)* **The "done when" row is best-effort, and "the
bead's intent" is a match, not a field.** §2 wrote *the closer's PID
"done when" row for the bead's intent* as if a bead carried an intent.
It does not: ADR 0001 rejected routing by intent — "`labels:` routes;
`intents:` describes" — so no store holds an edge from a bead to an
intent slug, and the row can only be recovered by matching words. The
rule as implemented (`closerDoneWhen` → `IntentDoneWhen`): the
candidates are the bead's labels plus its bd `issue_type`; a candidate
matches a slug when it equals the slug or one of its hyphen-separated
words, give or take a plural (`bug` → `fix-bugs`). The first row that
matches is the row. Measured (live store, 1516 beads, 2026-09-01):
before the issue-type candidate landed (ranger-base-wogo) 1 of 531
per-close sections carried the row, because `verify_labels` is by
design a persona's catch-all routing label and no slug in either crew
contains `code` or `devops`; after it, bug closes carry the row 21 of
21 and task closes 0 of 27. `task` is what `bd create` stamps when
nobody passes `-t` — 1040 of the 1516 beads, and 307 of the 623 closed
`code`/`devops` beads. A default type names no intent because it
carries no information, so "no match" is the *correct* answer for a
task close, and this section's requirement was unmeetable for the
majority of closes from the day it was written.

Decided:

1. **The row is best-effort content, never required.** It is absent
   when the closer is not a persona on this box, when its PID has no
   `## Intents` table, or when no candidate matches. An absent row is
   not an error and never holds the filing — the same rule §3 already
   applies to the commit trail. The verifier's checklist is still the
   closer's "done when" column (QA's PID says so); this amendment only
   stops promising that the harness can always pick the row for them.
2. **No match quotes the whole table, marked as such.** When the
   closer's table exists and nothing matches, the section carries every
   row, one indented line each under a header that says it is
   unmatched — e.g. `- done when (developer · unmatched; every intent):`
   then `    build-features: …` / `    fix-bugs: …` /
   `    implement-designs: …`. The verifier picks the row by reading
   the close, which is what they do today by opening the PID; the
   fallback moves the text onto the bead so §2's promise — the
   checklist without the PID — holds for every close a persona on this
   box made. It interprets nothing: the table is quoted, not chosen.
   Bounded: the largest table in either crew is five rows, so a batch
   of four costs at most twenty lines. Every cell passes through the
   one-line flattener like the rest of the section, and an indented
   line cannot forge the per-close marker. Cut as a `-l code` bead for
   the developer (named on ranger-base-ziy47).
3. **What does not change.** No PID gains a slug to catch `task`; no
   label/type → intent map exists anywhere; the word-loose match stays
   word-loose. Hatch: the fallback is one branch in one function and
   holds no state — deleting it returns to (1) exactly.

Alternatives rejected here:

- *A slug containing "task" in the developer's table* (cheap). A match
  on the type that means "untyped" is a fabricated signal: every
  default-typed close would quote the same row whatever was built, and
  the verifier would judge against a checklist chosen by absence. It
  also puts the routing vocabulary ADR 0001 rejected inside a document
  about roles.
- *A label/type → intent map*, in config or code. A second vocabulary
  beside labels is triple-implementing the substrate (ADR 0001's own
  words), and bd carries no data to be exact against; the wogo close
  declined to guess it and this amendment agrees.
- *Leave the text as written.* The record would keep requiring what the
  code cannot produce for 56% of sections. A measured gap the record
  denies is worse than an absent line.
- *Infer the intent from the graph* — `discovered-from` a
  `-l architecture` bead → implement-designs, type bug → fix-bugs, else
  build-features. The clever one, and the wrong one: it is right often
  enough to be trusted and wrong silently, and a verifier judging
  against the wrong "done when" is an escape, where one who reads the
  PID is a file read.
- *Warn on stdout per section without a row.* Noise proportional to
  the majority type, with nothing actionable behind it.

Measured: every count above, and that the row's absence changed zero
answers labels already gave (kvecg). Assumed: that the verifier opens
the closer's PID when the row is absent — their PID says the column is
their checklist, nobody has measured whether they read it — and that
five rows is the ceiling any instance's table reaches.

*(Amended 2026-09-02 from ranger-base-zbd51.)* **The filer's trailer says
the bundle, and the verify bead carries its close's class.** Two changes
to the bead this rule mints, both cut as one `-l code` bead
(`verifyafter.go`; pins in `verifyafter_test.go`):

1. **The trailer.** The verify bead's closing instruction read *"file a
   bug bead `-l code -a <closer>` with a repro and close this one
   `escape`"* — per close, and still `-a` the closer after the
   2026-09-01 amendment struck it (MEASURED: `verifyDescription` and
   `verifyGroupDescription`, with the pin at `verifyafter_test.go`
   asserting `-l code -a developer`). It now reads the §1 shape: *for
   any close that does not verify, file ONE findings bead `-l <the
   close's lane> -l debt --deps discovered-from:<this id>`, one line per
   finding (file:line · what fails · the bead it escaped from · the
   repro); a live money / constitution / dispatch-correctness defect
   alone gets its own `-t bug` P1/P2 bead, named in the bundle by id;
   then close this one `escape`.* The batch form says it once for all N
   closes, since the bundle is per verify close, not per close verified.
   The closer's name leaves the trailer entirely — the fix is lane work
   and the closer is not on §1's list.
2. **The class.** `BdNew` gains a `Type` (`-t`) and the filer sets it,
   or adds `debt` to the labels, from the close it verifies through the
   one class helper; a batch takes the most urgent class in the order
   bug › feature › debt › unclassified, chosen in the same loop that
   picks the batch's priority. Unclassified in → unclassified out: the
   filer never manufactures a class the close did not carry.

Hatch: both are text and one field on a bead the harness already
files; nothing new is read, and no state is added. The `-t` flag is
`bd create`'s own (`bug|feature|task|epic|chore|decision`), measured
on bd 0.50.3's `--help`.

**4. PID `## Handoffs` sections say the shape, not just the name.** Each
row becomes `who · label · what the bead must contain` — *who* is a lane
(`the code lane`), a person only for the §1 allowlist *(amended
2026-09-01, ranger-base-tpc41)* — e.g. the developer's: *hand to QA — nothing to do: verify is filed on your close;
hand to security `-l security` P≤1 when a change touches secrets, auth,
or egress.* Recommended rows for the whole example crew ship in
`examples/agents/*`; an instance's own PIDs are the operator's to update.

*(Amended 2026-09-02 from ranger-base-zbd51.)* The recommended QA rows,
as shipped in `examples/agents/qa.md` by this amendment's commit and as
proposed for this instance's two QA PIDs (a staged diff under the
constitution repo's `docs/rca/`, applied and promoted by the operator —
ADR 0015 §2/§3; the operator's bead is named on ranger-base-zbd51):

> Hand to
> - the code lane (the devops lane when the close was `-l devops`) ·
>   `-l code -l debt` · ONE findings bead per verify close (ADR 0006
>   §1): title opens with the verify bead's id and the count; one line
>   per finding — file:line, what fails, the bead it escaped from, the
>   repro or failing test; `--deps discovered-from:<verify id>` — then
>   close yours `escape`. No findings, no bead.
> - the same lane · `-t bug`, P1/P2 · its own bead only for a LIVE
>   defect in money, constitution, or dispatch correctness (ADR 0006
>   §1 names the three): the domain in the title, the repro attached,
>   and the bundle names it by id.
> - the security lane · `-l security` · a break that smells like
>   exposure, not just breakage, with what it reaches.
> - the product lane · `-l product` · an escape that is really a spec
>   gap.

and every PID's `## Handoffs` opening line gains the class: *`bd create
"<title>" -l <label> --deps discovered-from:<id>`, carrying its class
(`-t feature` / `-t bug` / `-l debt`)*. The examples carry it from this
commit; the instance's eleven PIDs carry it when the operator promotes
— nine of them also still carry the `-a <persona>` template line the
2026-09-01 amendment retired, which the same promote can take.

**5. Out of scope, named.** Persona-to-persona chat; approvals or sign-off
gates beyond "closed" (the verify is a bead, not a gate that holds the
close); a scheduler in posse; multi-repo handoffs (a bead lives in the
repo of the work — a handoff to another repo is `bd create` in that
repo's `beads:` dir with the origin id in the description; the `beads:`
list aggregates them already).

## Consequences

- `dispatch.go`/`beads.go`: verify-after — `ListAll` filtered by
  `closed_at > last pass` and label ∈ `verify_labels`, child check via
  `dependents`, `bd create` with the assembled description;
  fixture-tested; last-pass watermark in `RHQ_HOME/state/`.
- ADR 0005's Context makes every shape legible at the receiving end:
  `from:` names the origin bead, `design:` the ADR, and comments carry
  the prefixes.
- The metrics catalog gains the events it needs: `VERIFIED:`/`escape`
  for QA (`escapes-caught`), `DIVERGED:` for
  `designs-implemented-unchanged`, `SECURITY:` for
  `findings-surviving-triage`.
- Instance PIDs need their `## Handoffs` and `## Done` rows updated by
  the operator (the developer's stops saying "hand to QA" — it's
  automatic).
- *(2026-09-02)* The scorecard and pulse line report closes/day, open
  by class and P1/P2 per class through the one class helper
  (ranger-base-dwlb1); the class helper, the filer's trailer and the
  ladder's rendered class flag are the code beads cut from
  ranger-base-zbd51.

## Alternatives rejected

- **`bd mail` for persona-to-persona.** Unconfigured by default, and even
  configured it's a side channel dispatch can't see; the crew already
  has a queue that routes.
- **A `handoff:` frontmatter key or a routing table in config.** The
  edges are already in beads (`discovered-from`, labels, assignee);
  a second vocabulary is triple-implementing the substrate (ADR 0001).
- **Verify as a gate that holds the close** (bead stays open until QA
  passes). Turns every builder's close into a wait and makes `bd ready`
  lie about what's done; a verify *bead* keeps both honest.
- **The closer files the verify bead (convention only).** Silent failure
  mode is an idle QA persona; the harness rule costs one query per pass.
- **A dedicated `security` severity field.** bd has priority; mapping it
  is one line in the security persona's PID and this ADR.
- **posse schedules grooming.** A scheduler is a product; a weekly bead is
  a habit the operator already has.
