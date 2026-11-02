# The fold's premise gets executed, not only derived (ranger-base-3pm6b)

`constitutionResolveRunes` (`internal/posse/promote.go`) is the commit wall's
model of what a case-insensitive APFS volume RESOLVES an ASCII name to. It is
DERIVED — NFC, `cases.Fold`, NFC, over every rune `0x80..0x10FFFF` — and every
pin over it re-derived it the same way. So the sh sed render, the Go land belt,
`TestQAConstitutionResolveRunesAreTheClosedSet` and the sufficiency pin all
agreed with the table whenever the table agreed with **Go**, whatever the
filesystem did.

That is the shape of three beads in a row, each the same defect at one more
rune's distance:

- **ranger-base-y52v6** — the wall matched staged names by string, and the
  volume resolves. Fixed by making the class resolution-equivalent
  (ranger-base-g55ah), modelled as Go's simple case fold.
- **ranger-base-qu8wn** — APFS is normalization-insensitive too, so two runes
  reach ASCII through NFC and neither arm folded them (U+037E, U+1FEF).
  Fixed by widening the model to NFC + fold.
- **ranger-base-l717c** — APFS's FULL case fold lengthens, so one rune opens
  two letters and a rune->byte table could not say it; `poße/config.yaml`
  landed past both readers. Fixed (ranger-base-uvcj8) by making the table
  carry ASCII STRINGS: thirteen rows.

Each fix widened the MODEL. None of them executed the premise that the model
is what the volume does, so the next reading found the same bead again — this
one, filed by ranger-base-2g820's verify of the fourth close.

## What landed

`TestQAConstitutionResolveTableIsThisVolumesEquivalence`
(`internal/posse/constitutionfold_qa_test.go`, arm 3) reads the subject. Two
halves, and a failure in each means the opposite thing:

- **Forward** — every row of the table opens its ASCII target on this volume,
  by `os.SameFile` and not by existence, because the table names WHICH string.
  A red here says the table is wider than the volume: the wall stays shut (a
  fold wider than the equivalence refuses spellings nobody could use) but the
  table's measurement is stale.
- **Boundary** — nothing OUTSIDE the table opens an ASCII name. A red here
  says the wall is open, which is the landing ranger-base-l717c measured.

The candidate set for the boundary half is the one class that could falsify
the premise and the one a `cases.Fold` derivation cannot produce: runes whose
COMPATIBILITY form (NFKC) reaches ASCII but whose canonical fold does not.
That is Unicode's next-wider standard relation after the one the table models,
and it is where every lookalike spelling of an ASCII name lives — the
fullwidth forms (U+FF53 in place of `s`), the circled, parenthesized and
superscript letters, the roman numerals, the squared abbreviations whose
compatibility form is several ASCII characters.

It subtracts the **shipped table**, never `constitutionResolveDerived()`.
Subtracting the derivation puts every rune the derivation produces outside the
candidate set, so a row DELETED from `constitutionResolveRunes` would leave
the boundary half green over a spelling the volume really does open — measured
as a surviving mutant and fixed before landing (M3 below).

## MEASURED 2026-11-02 — Darwin 25.6, case-insensitive APFS, go1.26.10, golang.org/x/text v0.43.0

    derived (NFC / fold / NFC)          13 runes
    wider   (NFKC / fold / NFC)      1,405 runes
    candidates (wider minus table)   1,392 runes

    all 13 table rows resolve, by identity
    0 of 1,392 candidates resolve
    U+0130 opens nothing (negative control, as in every bead of this chain)

Under half a second for fixtures and probes together; 0.41s for the test
including the two derivations over 1.1M runes.

Fixture shape: the ASCII target is the MIDDLE of `x<target>y`. Several
compatibility targets are not legal names on their own — `.` and `/` both
occur among the 1,392 — and a wrapper gives every target one spelling that is.
Every fixture is written BEFORE any probe, so each probe is asked against the
whole name set: a candidate resolving to some other row's target is still a
spelling that opens an ASCII name.

## The skips, and why they are not decoration

Whether two spellings are one file is a property of the MOUNT, so the pin
skips off darwin and on a case-sensitive volume, each with a line naming the
volume it did not get rather than passing vacuously. That is
ranger-base-99gww's rule (`visibilitypathcase_qa_test.go`), and the detection
is the same: write one file, stat the other case, `os.SameFile`. On a
case-sensitive volume no spelling resolves to another one, so there is nothing
there for the fold to be a model of.

The anti-vacuity guard is a FLOOR on the number of candidates actually probed
(1,000 against the 1,392 measured), not an equality, so a Unicode table bump
that adds or removes a lookalike does not red it — but an empty candidate set
does.

## MUTATION-CHECKED 2026-11-02 (`go test -overlay`, nothing dirtied)

- **M1** — boundary assertion inverted (`err != nil`): reds 1,392 times, one
  line per candidate. The probe can say NO, which is the whole claim.
- **M2** — `{0x00DF, "ss"}` to `{0x00DF, "sss"}`: reds the forward half,
  naming U+00DF. This is the mutant that earns `os.SameFile` over existence —
  `xssy` exists on the volume as another candidate's target, so an
  existence check would have passed.
- **M3** — the `{0x212A, "k"}` row deleted: reds the boundary half, naming the
  KELVIN SIGN as a live escape. **This mutant SURVIVED the first draft**,
  whose candidate set subtracted `constitutionResolveDerived()`; it is why the
  subtraction is keyed on the table.
- **M5** — the candidate derivation emptied: reds the vacuity floor.

## What this does NOT cover

The candidate set is still derived, from a strictly wider model. The arm reds
the day the volume's equivalence widens as far as compatibility equivalence,
and not the day it widens past that. The TOTAL reading — every rune
`0x80..0x10FFFF` against one fixture per graphic ASCII character and per a-z
string of length 2 and 3 — stays ranger-base-l717c's, dated, and an OS bump
re-pays it. ADR 0015 section 3 carries that obligation, narrowed to the
residual, in its 2026-11-02 amendment.

That amendment also corrects the paragraph above it: it still said "the
measured four", which ranger-base-l717c made thirteen. The decision did not
move.
