# ADR 0040 — Amend existing roots; preserve numbered history

*Status: accepted 2026-09-01; simplified 2026-09-05 by operator ruling · amended 2026-09-06 (ranger-base-x2pbz: no index file, the status line is the index) · owner: architect.*

## Context

Doing nothing leaves a decision assembled from competing paragraphs. The
September 1 consolidation was accepted (ranger-base-ay3dr), despite its
proposed header. Its preference for four new roots is reversed by **ADR
simplification, operator ruling 2026-09-05**. That ruling also approves the
September 5 review's smaller mechanisms, with measurements on deferred code
beads; documentation does not wait for those measurements.

## Decision

Amend the existing policy homes. Fold surviving rules into the homes below,
with a Lineage row at the destination and a dated superseded pointer at the
source. Source bodies are historical. Keep one active statement of each
decision; amend it in place rather than append a competing amendment.

| Policy home | Content folded or governed there |
|---|---|
| 0002 — enforcement | 0009 gate shell, 0023 hook identity, 0025 enforcement classes; 0052 owns managed-hook realization and points here for what realization proves |
| 0003 — model selection | 0039 built-in dial and catalog lease; 0015 owns its promotion portion |
| 0008 — session ownership | 0030 orphaned-claim crew tie-break |
| 0010 — plan guard | 0018 complete blind/headroom/ledger table; 0013 points here for guard policy |
| 0011 — dispatch | 0020 availability-first selection and 0028 bounded passes with rolling seats |
| 0012 — public/private boundary | 0037 venue dimensions versus deployment facts; 0015 owns constitution source and 0055 owns store environment binding |
| 0013 — runtime contract | 0017 equivalence, 0021 overlays, 0032 onboarding; 0057's concrete pane readers are an explicit narrow exception to declarability |
| 0015 — constitution promotion | sole promotion home for runtime overlays from 0021/0039; current directory name from completed 0046 migration |
| 0019 — credentials | 0042 mint-before-runtime seam; 0002 continues to own enforcement |
| 0024 — public visibility | 0048 scan coverage; 0050 remains the distinct all-repository instance data ceiling |
| 0029 — governance facts | observations and their scopes; 0027 owns delivery and repetition |
| 0051 — editorial citations | citation convention and optional audit; 0006 owns verification and landing evidence |
| 0056 — substrate pin | no-daemon is the current compatibility tripwire; older daemon-performance explanations in 0002/0014 are dated history |

Simplify 0003, 0006, 0010, 0012, 0016, 0019, 0026, 0027, 0029,
0034, 0035, 0036, 0040, 0049, 0051 and 0057. Retire 0046's completed
migration premise. Other review KEEP verdicts retain their decisions;
cross-reference corrections and receipt of folded content do not retire them.
The 0034 display remains telemetry; its guard proposals are dropped. Cancel
0035's approved but unbuilt duplicate mode flag under this new ruling.

No numbered file is deleted or renumbered. Existing 0041 and 0042 belong to
their landed decisions, not the proposed replacement roots. The 0043–0045
gap is not a reservation. If a genuinely new consolidated page is needed,
inventory the directory at commit and take the next free number; none is
needed for this execution. Old citations resolve in one hop. Repoint them
when their code is otherwise edited; do not change runtime strings, tests,
or prompts just to change an ADR number.

There is no index file: `docs/adr/README.md` is not written. The directory
listing is the inventory, and line 3 of every decision record, its
`*Status:` line, is that record's disposition; a superseded record's line
names its successor (`superseded <date> by ADR NNNN`), which is the one-hop
pointer this Decision already requires at the source. The set in force is
rendered live, never transcribed:

    grep -m1 '^\*Status' docs/adr/0*.md

A new decision record carries that line on line 3. The probe traces
(`0013-*-probe.md`, `*.probe.sh`) are supplements, not decisions, and carry
none. MEASURED 2026-09-06 at 6f94a99c (ranger-base-x2pbz): 58 numbered
pages, 55 carry the line and all 55 carry it on line 3; the three without
are the 0013 traces; 40 decision records are in force, 15 are superseded or
retired and every one of the 15 names its successor in that form.

Accepted changes to running behavior remain **pending implementation** until
their code beads land. Each removal has one task, priority 2, label `code`,
unassigned, deferred to 2026-09-12. Its first done-when row records the review's
measurement; remaining rows name deletion, retained invariants, and the risk
if wrong. The coordinator lifts the pause after this execution. This record
does not claim the simplified implementation already ships.

## Consequences and alternatives

MEASURED at the September 5 review: 55 numbered ADRs and seven supplements;
the old migration had priced 59 runtime strings and nine test messages to
move. This documentation change removes no runtime mechanisms. ASSUMED:
one policy home reduces reading and maintenance cost; that gain is not a
throughput measurement.

Rejected: doing nothing or adding only an index (keeps contradictory
authority); four new roots (citation churn without a smaller implementation);
one giant constitution (one shared editing bottleneck); renumbering or
deleting old pages (breaks historical citations); treating a proposed or
accepted header as proof that code landed (confuses a decision with state).
History remains in the superseded pages and git. The pre-execution version
of this record retains the previous inventory, cost estimates and rejected
migration plan; those dated counts are not current product facts.

Rejected 2026-09-06 (ranger-base-x2pbz), a committed index in any of three
shapes. Hand-written: it states every disposition a second time, which is
the assembly this record exists to end, and the pre-execution §2 index went
stale in four days. Generated and pinned: the same file, plus a pin that reds
every ADR commit that did not regenerate it. Either way the file is a second
writer on every ADR bead under ADR 0022's one writer per file — MEASURED
2026-09-01 to 09-06: 71 commits changed a status line, and 41 distinct beads
touched `docs/adr` on 2026-09-05 alone — so the index would conflict on the
launcher's fast-forward on any busy day. Grouped by the policy-home table
above: the table names 20 of the 40 records in force, so the other 20 would
need homes invented for a page no reader is pointed at (crew reach a record
by the bead's `design:` path or by number; nothing in AGENTS.md, the PIDs or
the work prompt names a README). Every `.md` under `docs/adr` is a record to
the citation corpus, the retired-package sweep and the SHA census, so an
index would also be the most-cited record in the set.

## Lineage

| Was | Now |
|---|---|
| 0040 §§1–2, 4, old new-root migration accepted on ranger-base-ay3dr | Existing-root disposition above; operator ruling 2026-09-05 reverses the migration preference |
| 0040 §3 numbering, one-hop citations and single policy home | Decision above; stable numbers retained, amendment and folding replace new-root ceremony |
| 0040 §2's concern index, the `docs/adr/README.md` it promised | No index file (Decision above, 2026-09-06); the status line on line 3 of each record is the disposition and the successor pointer, rendered live by the grep there |
