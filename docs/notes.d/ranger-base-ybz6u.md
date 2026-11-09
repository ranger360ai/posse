# `seatbelt (bwrap)` is a label and not a mapping, and the rest of this bead landed twice (ranger-base-ybz6u)

Replayed onto main under `ranger-base-cgmnk`, which is the merge-back block
this fragment is mostly about.

ADR 0080 D18 (`ranger-base-djl3c`, `c20771bb`) landed `make test` green on all
three darwin arms and red on all three Linux ones: seven tests, MEASURED on
ranger360ai/posse run 42565490062, first red in `0ea8248f..3f256af5`, no
product defect among them. `ranger-base-ybz6u` was filed on it with two
questions — the seven regressions, and a DESIGN question about whether the
label `cage seatbelt (bwrap)` was a `cage_realize:` mapping that had landed
undeclared.

**Both questions were answered twice, concurrently, by two sessions of the same
persona.** `ranger-base-temhf` was filed on the same CI red three minutes
earlier and landed as `3135c8ac`; `ranger-base-ybz6u` landed as `bb492fad` on
`posse/jian-yang-posse-ranger-base-ybz6u`, which then could not merge back —
`ranger-base-cgmnk`. Read that merge block before reading its conflict: the
two commits are the same work, not two halves of it.

## The duplication, so a reader of either bead can get to the other

| | `ranger-base-temhf` (on main) | `ranger-base-ybz6u` (`bb492fad`, not on main) |
|---|---|---|
| the seam | `var wallMechanismHost = runtime.GOOS` | `var wallMechanismHere = func() …` |
| the fixtures | `internal/posse/wall_helpers_test.go` | `internal/posse/wallfixture_test.go` |
| the pins | `internal/posse/walllaunchmechanism_qa_test.go` | the same file name, different bodies |
| the seven | fixed | fixed, independently |
| the two side effects | found, filed `ranger-base-sg7a7` | found, recorded |
| the D7 design question | not addressed | **answered and pinned** |

Diagnosis and fix agree line for line in both: every other host-shaped fact in
ADR 0080 is drivable from the other host on purpose — `wallMechanismOn`,
`seatbeltTierAvailableOn`, `renderWallOn` take the host as a parameter and
`bwrapResolves`, `bwrapApplyRefusal`, `bwrapBin` are package variables each
carrying a comment that says so — but `WallMechanism()` was not, and after D18
it was the single fact a LAUNCH's choice of mechanism rested on. Both beads
made it a variable, both wrote mechanism-neutral fixtures composed from the
production renderers, both fixed the same seven tests in their own terms, and
both found the same two side effects of the one thing bwrap does that
sandbox-exec never did (the launcher MAKES every write grant that is not
there yet, `RenderBwrap`'s prep, D17/D18): a `writable:` entry under `/` is a
directory posse creates on the suite's own host, and a seatbelt launch
materializes `<cwd>/.beads`, which moves ADR 0013 §4's record-reach row for a
LATER launch in the same tree. `docs/notes.d/ranger-base-temhf.md` is the
long form of all of it and is the page to read; nothing here repeats it.

So the replay under `ranger-base-cgmnk` carries **one** thing: the design
answer, which `ranger-base-temhf` never asked and main therefore did not hold.
Replaying the rest would have put two seams on the same host fact behind one
`WallMechanism()`, which is a defect and not a merge.

## The design answer, which is the part that was only in this bead

The bead's worry was that `cage seatbelt (bwrap)` is an ADR 0083 D7
`cage_realize_<goos>:` mapping landed without being declared — D7 being an
operator ruling (option C on `ranger-base-nllw7`) that the weaker realization
of a demanded tier is an explicit promoted line which fails closed when
absent. If that were what the label meant, the Linux host cutover would need
a `cage_realize_linux:` entry and every bwrap seat would carry a ratified
waiver.

It is not, and the constitution already says so in two places that agree:

- **ADR 0080 D16** — "the tier keeps its name, `seatbelt`; the mechanism is a
  host fact." `cage: seatbelt` names the L2 path-scoped write wall (ADR 0002
  §3), realized by `sandbox-exec` on darwin and `bwrap` on Linux, the way
  `cage: container` is realized by whichever engine the home names. "A PID
  never names the mechanism." `seatbelt (bwrap)` is `CageLabel`'s output — a
  label and never a value; `Parity.Cage` stays the bare tier.
- **ADR 0083 D7** maps one TIER to another (`seatbelt: shims`), fails closed
  when absent, and marks every session it lowers with a ratified waiver. Its
  own second bullet is "a tier maps to itself where `AvailableCages` has it",
  and D6 option C names the bwrap tier as the thing that ENDS the
  `seatbelt → shims` entry: "the D7 map flips from `shims` to the bwrap tier,
  one promoted line."

**So the host's map at cutover has no `seatbelt` key in it at all** — the tier
realizes itself there. A mechanism maps nothing and waives nothing; a mapping
maps a tier and marks the session. Nothing to change in the code, and the
answer is now pinned rather than reasoned, in the last block of
`TestQALaunchAtTheOtherHostsWallMechanism`
(`internal/posse/walllaunchmechanism_qa_test.go`, arm 1): over the launch that
test has already made at either mechanism, in a home declaring no
`cage_realize_*` key, `CheckParity` answers the bare `CageSeatbelt`, an empty
`Ratified` and an empty `Degraded`.

It reads the claim inside BOTH subtests on purpose, and the comment says why it
is not reading the mechanism: `RealizeCageIn` keys on `runtime.GOOS`, not on
`wallMechanismHost` (`cagerealize.go`), so what the block measures is the
ABSENCE of a mapping — which is the fact D7 and D6 option C are about — and a
mechanism that ever did reach that answer would red here and nowhere else.

MEASURED 2026-11-09, darwin/arm64: green at both mechanisms. Mutation, a
scratch patch and not shipped — appending
`cage_realize_darwin:\n  seatbelt: shims` to the fixture's `config.yaml` reds
both subtests on all three fields at once: `cage="shims"`,
`ratified=[cage_realize: darwin seatbelt→shims (config.yaml — this home was
never promoted)]`. That is also the reading that confirms the `runtime.GOOS`
sentence above: the `darwin` entry fired inside the `bwrap` subtest too.

## The lesson that is not about walls

Two sessions of one persona took one CI red from two beads filed three minutes
apart, and the second one's work is a merge conflict with the first's. Nothing
in either session could see the other: the beads were separate, the worktrees
were separate, and the first commit reached `main` while the second was being
written. What the merge block then reports is a conflict, which reads like a
race over shared lines and is actually a duplicate deliverable — and the
literal instruction on a merge-back block, *replay every original*, is the
wrong move for one. The tell is cheap and worth checking first: `git log
--format='%s' main -5` beside the blocked commit's own subject. Here they
named the same host fact in the same words.
