# An exemption that drops an id from the SCAN also drops it from the STRIP (ranger-base-4fx5x)

*2026-11-14 · holden · verifying ranger-base-ed07e, ranger-base-blzkq,
ranger-base-omdr9 and ranger-base-41l0r*

Three of the four closes verify on their own acceptance. One has a narrow
regression at the boundary of the thing it added, and it is a shape worth
naming because the mistake is structural rather than careless: **an
exemption written as "take this id out of the list" silently changes a
second thing, because the list was doing two jobs.**

## The class

`vulnPrivateHits(text, ids)` (internal/posse/vulnprivate.go) uses `ids` for
two different purposes in the same pass:

1. **presence** — which ids appear in this unit of text, i.e. which ids are
   candidates to flag; and
2. **stripping** — every id present is removed from the unit before
   `vulnPrivateBare` asks whether what is LEFT carries a letter.

Its own doc comment states the invariant job 2 buys:

> Every id present in a unit is stripped from it before the bareness test
> runs, so `- ranger-base-a, ranger-base-b` reads as bare for both (two bare
> ids sharing one bullet …)

ranger-base-ed07e's exemption removes the committing tree's own bead id from
`ids` — which is right for job 1 and wrong for job 2. The own id is now left
in the text while the bareness test runs, so its twelve-plus letters are read
as a description of whatever OTHER open id shares the unit.

MEASURED 2026-11-14, this tree at 025d2e58, through the shipped
`RunGatesVulnPrivateScan` in a real session worktree whose branch record
names the own bead, with both beads open and labelled `vuln-private`:

| the unit | before ed07e | after ed07e |
|---|---|---|
| `- <own>` | clean | clean |
| `- <other>` | clean | clean |
| `- <other> the mechanism` | flags other | flags other |
| `- <own>`<br>`- <other>` (two bullets) | clean | clean |
| `- <own>, <other>` (ONE bullet) | clean | **flags other** |
| `<own> <other>` (one subject line) | clean | **flags other** |

Both controls were live in that run: `- <other> the mechanism` flagged, and
`<own>: the fix this commit carries` stayed clean.

## Why it is worth a bead rather than a shrug

The base rate is low — AGENTS.md prescribes one bare id per bullet, and that
shape is clean. What makes it a defect and not a rough edge is the REMEDY.
The refusal prints:

> One id is never flagged here: the bead this tree's branch is recorded for
> … So a flagged id names ANOTHER hole

and the way-through sentence tells the writer to write the bare id alone. In
this shape the writer has already done both: the unit contains nothing but
bare ids, and the flagged id names no hole in it. That is a refusal with no
remedy the writer can perform over text the rule does not bind — the ADR 0068
class, and the same class as ranger-base-68631 finding 5, which this very
file's comments cite as the cost to avoid.

## The ask of the next exemption

**When you exempt an id from a check, ask which of the check's uses of that
id you meant to exempt.** Here the fix keeps one list for presence and
another for stripping: flag only non-own ids, strip every open id. The
doc-comment invariant then holds again as written, and
`TestVulnPrivateHitsAcceptsEveryDecorationTheRemedyPromises`' promise is kept
in the one case the exemption reaches.

Filed as ranger-base-4fx5x's findings bundle; not fixed here (QA verifies,
the code lane fixes).

## Two things that did NOT turn out to be defects

Both were worth the measurement, and the record is the point.

**The predicted starvation did not happen.** ranger-base-blzkq's own handoff
ranger-base-a5hvg warned that B's repaired registry lane would put 86-138
records ahead of `github_release` under a prefix-truncation cap whose fix is
unpushed, and that repairing the registry by starving the launch stream is a
trade rather than a repair. monica's post-deploy evidence (20 of 20 recent
Events carrying `source_registry`) is exactly what that starvation would also
look like, so it was worth separating. MEASURED 2026-11-14 off the public
`ranger360.ai/api/events/summary` histogram, approvals by day: 11-13 launch
0 / publication 25; **11-14 launch 8, publication 25, plus 24 events in types
that lane had never produced** (other 12, partnership 11, leadership_change
1), total 57 against 25. `launch` went UP, to its highest since 11-02, and
the arXiv lane is untouched. The cap was not binding at that volume.

**A prefix collision between two open vuln-private ids would mis-flag, and
cannot today.** `vulnPrivateHits` tests presence with `strings.Contains` and
strips with `ReplaceAll`, so an id that is a strict prefix of another is
found inside it and stripped out of it, leaving the suffix's letters behind.
It is pre-existing, not ed07e's, and the base rate is zero: MEASURED
2026-11-14, 25 open `vuln-private` beads, all five-character suffixes, no
colliding pair. Thirteen ids in the store are shorter than five characters,
so the shape is reachable in principle. Named here and nowhere else, because
nothing is broken.

## What verifying these four cost, and what it bought

Independent re-measurement, not re-reading, is what moved every verdict here.

- ranger-base-omdr9's nine-plant mutation battery was re-run from scratch
  against its own branch: nine plants, nine RED, green control, tree restored
  (`git status` clean). Its load-bearing arXiv numbers re-measured against
  export.arxiv.org and every one reproduced — 25/25 identical ids between the
  main query's newest 25 and the registry query's 25; `totalResults` 488,661;
  4,221 for the bead's literal 11-day window; **0** for a trailing-24h
  window; 10,234 in a trailing 30 days (~341/day against the bead's ~342);
  the newest indexed entry 39.2h behind wall clock. 25/25 identical ids with
  and without a 30-day window, which is the measurement that refutes fix
  shape (a).
- My OWN first probe of those numbers was the thing that was wrong, and in a
  way worth recording: joining the window with `+AND+` and no parentheses
  reads as `cat:A OR cat:B OR (cat:C AND date)`, so the ORs dominate and
  every window answered ~426,000 — three plausible-looking numbers that
  refuted nothing. **Parenthesize the category clause**, or the window is
  decoration twice over.
- ranger-base-ed07e's regression was found by asking what else the exempted
  id had been doing, and confirmed end-to-end through the shipped judge with
  two live controls, not by reading the diff and nodding.
