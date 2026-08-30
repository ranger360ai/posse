# ranger-base-c9l8 — verify of four closes: the fourth verdict, and the wrap-up

laurie, 2026-08-30, at posse `3b4c821`. The verdicts for ranger-base-7tq2,
ranger-base-59jd and ranger-base-zs6b are on the bead itself. This file carries
the fourth, plus what I left behind — the bd payload gate refused five separate
wordings of the paragraph below, so the record lives here and the bead points at
it. That is deliberate, not thin.

## ranger-base-1vpf (jian-yang, code) — VERIFIED

Its DONE WHEN is met, and each half was exercised rather than read:

- INSTALL.md §14's seeding row names the line a reader should look for after the
  re-run, and its caveat carries both `promoted.json` and `posse promote`.
- The quoted line is pinned against what `initFrom` really prints — split on
  `<n>` — rather than against a copy of itself.
- `TestInstallSeedingRowLeavesAHomeADispatchWillLaunchOn` and
  `TestInstallSeedingRowNamesPromoteForAPromotedHome` both pass at HEAD and
  **neither skips**. I checked that on purpose: the second is written to skip
  itself with a rewrite instruction once ranger-base-pith lands, so a green run
  that skipped it would have measured nothing.
- Mutation-checked by me rather than taken from the close comment: forcing
  `repaired := false` in `internal/rhq/init.go` reds the first pin and correctly
  leaves the second green — that arm was never on its path.

## Defect found in the replacement text — ranger-base-g4cm (P2, jian-yang, code)

The row's new last sentence reads init's silence about `promoted.json` as proof
that the home was promoted. Two different homes produce that silence, and for
one of them all three of the sentence's claims are false. Mechanism, the
measured repro and its output: `docs/notes.d/ranger-base-c9l8-seedrow.md`.
Pinned green on both sides of the fix in
`internal/rhq/installseedsilence_qa_test.go`, with the gappy seed as its control
arm.

## Third defect, from 7tq2's own prediction — ranger-base-d8o6 (P3, dinesh, code)

Two commands in one binary disagree about whether an already-landed duplicate
tree still holds unlanded work, and the operator-facing one is the wrong one.
Written up in `docs/notes.d/ranger-base-c9l8-treestate.md`.

## What I left, and what I did not touch

- `cb2ffcf` — `internal/rhq/forcespelling_qa_test.go`,
  `internal/rhq/installseedsilence_qa_test.go`, and the seedrow note.
- `3b4c821` — the treestate note.
- `docs/notes.d/ranger-base-c9l8-verify.md` (this file).

No production code changed by me. The four mutations run during verification
(`seatidle.go`, `refillreport.go`, `gates.go`, `init.go`) were reverted one file
at a time and the tree was confirmed clean before the suite ran.

Suite at HEAD, on the tree that carries the two new pins:
`github.com/ranger360ai/posse` ok 386.3s, `.../cmd/posse` ok 333.7s,
`internal/rhq` PASS exit 0.
