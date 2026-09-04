# A page-currency census that reads bd

ranger-base-tii7l, 2026-09-04, from ranger-base-ur2eo (itself from the verify
bundle ranger-base-2fl9a). Ships `scripts/page-currency-census.py`. The
method, the arms and the limits are in that file's module docstring; this page
is the part that is about the recurrence rather than about the script.

## The recurrence, which is the whole argument

`docs/adr/0019-credential-architecture.md` was swept for page currency three
times in one day.

| sweep | bead | fixed | instrument | left behind |
|---|---|---|---|---|
| 1 | ranger-base-vxbfm | 6 passages | reading | — |
| 2 | ranger-base-vfx8g | 3 citations | `grep -nE "ranger-base-[a-z0-9]+[^)]{0,60}open"` | 5 live sites |
| 3 | ranger-base-ur2eo | 5 passages | reading | — |

Sweep 2 wrote a completeness sentence beside its fix — that the page carried
no open citation naming a closed bead — and that sentence was **true as
written**. Its census returns exactly 2 at the sha carrying the five, and both
are the ones it fixed. The sentence and the census agreed with each other and
both were blind, which is the failure mode worth naming: an instrument that
cannot see a class certifies the class absent, and nobody re-reads a page
that has just been certified.

What that grep can see is one shape: the word `open`, **after** the id, on
**one** line, with no `)` in between. The five it could not see were spelled
"waits behind its number as", "blocked on hs0dl", "runs only after", "is asked
only if B measures dead", and "after hs0dl measures B dead". Two of those put
the claim before the id; one straddles a hard wrap; none of them is the word
`open`.

## Numbers

All measured 2026-09-04 at 0022e4d, which is the page's state in git as this
lands — so the control arm is one command and reproduces.

- ADR 0019: 104 mentions of 41 ids, 38 closed. **13 candidate windows**, and
  every one of ur2eo's five sites is among them. The old grep: 2.
- Without the whitespace snap: 14. The extra one is line 952, where the raw
  ±200 slice ends mid-word and "never opened" reads as "never open".
- Whole doc tree (187 files, 647 ids, 616 closed): 146 windows in ~5 s.

## What it cannot do, stated rather than discovered

Two limits, both measured, and together they are why this is an instrument
with no failing exit code rather than a gate.

**The residue.** Over a *corrected* page the census still returns 5, and all 5
are one passage: the status block's own record of the corrections, which has
to talk about beads being open in order to say they were not. A word-matching
instrument reading a page that discusses openness cannot tell that from a live
claim — the distinguishing fact is the sentence's mood, not its vocabulary.
Wire this to an exit code and the cheapest way to green is to delete a true
sentence.

**The luck.** Four of ur2eo's five spellings are in the phrase list and are
found by design. The fifth is not, and is reported only because an unrelated
"open" happens to sit inside its window; delete `open` from the list and that
site vanishes along with four others, and the arm reads 7. Reword one sentence
and the census goes blind to a live site while still printing twelve others.
No phrase list closes this — a bare `after` would catch it and takes the page
from 13 windows to 22. **The list is a net, not a proof**, and any future
sentence claiming a page is clean should cite the count, not the absence.

## The three decisions the bead said nobody had made

1. **Where it lives** — `scripts/`, beside the three censuses already there,
   not a `posse gates` mode. It reads docs and the bead store, which is
   nothing `posse gates` is about.
2. **Whether it reads bd** — live by default, because the caller is a person
   asking about right now; behind a `--status-json` / `--emit-status` seam so
   nothing hermetic ever shells out. `--self-test` uses only the seam plus a
   planted fake `bd`, and runs under `env -i PATH=/usr/bin:/bin`.
3. **Gate or instrument** — instrument, and not "for now". There is no
   `--strict` to add later without answering the residue first.

## The defect the instrument had, found by its own arithmetic

`bd show ranger-base-fm4p` returns a row whose id is `rangerhq-fm4p` — the
retired pre-publication prefix — status **closed**. The first cut keyed the
result by the RETURNED id, so the requested one came back `unknown`; an
`unknown` id is skipped; and two closed beads' windows were therefore never
read. That is precisely the miss this instrument exists to prevent, and it
was in the instrument.

Nothing announced it. It was found because `--emit-status` wrote 649 keys for
647 ids asked about, and two more keys than questions is not a thing a
lookup should ever produce. The fix: a request left unmatched after a batch
is re-asked alone, where one row is unambiguous, and the rename is reported
beside the status so a reader knows the page cites a retired id. Neither of
the two carried a claim shape, so no live site was hiding behind it — but the
blind spot was real and is now an arm.

The general form is worth keeping: **an output that does not balance is a
finding.** A census that answers one question per id should return exactly
one answer per id, and any instrument with that shape can be made to check
its own arithmetic for free.

## The rig, and the three arms that could not fail

40 arms, 29 mutants of the script run against them, 28 dead and one
equivalent. Three of the 28 only started dying after the rig itself was
fixed, and the defect was identical each time: **a fixture derived from the
constant it was grading**.

- `"z " * (WINDOW // 2 + 40)` moved with `WINDOW`, so `WINDOW 200 -> 2000`
  left the window-bound arm green.
- `range(CHUNK * 2 + 7)` moved with `CHUNK`, so `CHUNK 50 -> 1000` left the
  batching arm green at one call.
- Five spellings on one small page sit inside each other's ±200 windows, so
  every window matched every phrase and `len(hits) == 5` was counting
  *mentions*. A phrase deleted from the list did not move it.

The fix in all three is a literal, and the literals are load-bearing. The
first two also mean the shipped `WINDOW = 200` and `CHUNK = 50` are now
blessed by the rig: changing either is a change to what the instrument claims
and the arms say so.

## Byproduct

The tree run lists 5 ids the store does not have. Two are the documented
placeholders (`ranger-base-abcd` in AGENTS.md, `ranger-base-xxxx` in
HISTORY.md); the other three name beads the store no longer carries, two of
them the subject of their own `docs/notes.d/` pages. That is a different
defect from this bead's and is only listed, never counted as a hit — an id
the store has never heard of is reported `unknown` and never as "not closed",
because `bd show` drops an unknown id from its JSON array, puts the error on
stderr, and still exits 0.

This page is itself in the residue class, and deliberately so: run the census
over it and it returns 3 windows, because a page about beads reading as
pending has to say "pending" to say anything at all. If that ever becomes a
gate, this is the first file it fails on.
