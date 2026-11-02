# Two rigs that lie about a wall, and the probe that would have caught both (ranger-base-2g820)

Verified 2026-11-02, the four closes ranger-base-m8kfv, ranger-base-pruuq,
ranger-base-7qnag and ranger-base-l717c. All four hold. What is worth keeping
is not the verdict but the two rig traps below: the first produced a false
"the constitution wall is open" reading at this seat, with a clean-looking
control beside it, and the second is the falsification test the fold's whole
chain has never had.

## Trap 1: `git clean -fd` between probe rows deletes the constitution marker

The promoted half of the commit-wall class — `posse/config.yaml`,
`posse/agents`, `posse/recipes`, `posse/runtimes`, `posse/skills`,
`posse/envs` — is in the class only when the repo IS the constitution repo,
and the hook decides that by `[ -d "$top/posse/agents" ]` OR a `posse/agents`
tree in the commit's base (the OR is ranger-base-jex3: the worktree test alone
was a detector a persona could `rm -rf`).

A probe rig that runs one staged commit per row and resets between them —
`git reset --hard $BASE && git clean -qfd` — passes row 1 and then silently
changes the subject. git tracks no empty directory, so an unstaged
`mkdir -p posse/agents` marker is removed by the `clean`, the base tree does
not carry it either, and from row 2 on the promoted set is simply not in the
class. Every `posse/*` row after the first then reads **LANDED** against a
wall that is working correctly.

MEASURED here: rows 1-2 refused, rows 3-6 (`recipes`, `runtimes`, `skills`,
`envs`, all canonical spellings) landed; and `poße/config.yaml` landed while
`poẞe/agents/developer.md` was refused in the same run. That asymmetry is
what makes it convincing rather than obviously broken — `poße/agents/...`
re-CREATES the marker dir as a side effect of staging its own path (APFS
resolves `poße/agents` onto `posse/agents`), so the rows that look like they
exercise the fold hardest are exactly the rows that repair the rig.

- **Commit the marker.** `posse/agents/.keep`, in the base commit, so it
  survives every reset. Then the dir test and the base-tree test both answer
  yes for every row.
- **Assert it per row, not once.** Print the marker's state on each row
  (`[ -d posse/agents ]` and `git cat-file -t "$BASE:posse/agents"`). A rig
  guard that runs once at the top cannot see a rig that degrades at row 2.
- **The canonical spelling is the control that catches this, and a fold probe
  that omits it has no control at all.** I had a negative control (U+0130,
  opens nothing) and a positive control (U+017F, in the table) and both read
  correctly while four rows lied, because both are `.claude/*` — the
  every-repo half, which needs no marker. The control has to be the plain
  ASCII spelling of *the same class member the folded row is attacking*.

## Trap 2: a wall whose premise is derived, and the 40-rune probe for it

`constitutionResolveRunes` (internal/posse/promote.go) is pinned as a closed
set by re-deriving it from NFC -> `cases.Fold` -> NFC over every rune
(`TestQAConstitutionResolveRunesAreTheClosedSet`). That pin reds on a
Go/Unicode table bump. It cannot red on the filesystem changing, and the
filesystem is the subject: three beads in this chain — ranger-base-y52v6,
ranger-base-qu8wn, ranger-base-l717c — were each the same defect, a model of
APFS's name equivalence narrower than the equivalence.

The probe that tests the premise instead of the model, and it is a dozen
lines: take the runes whose COMPATIBILITY (NFKC) form is ASCII but whose full
case fold is NOT, since a `cases.Fold` derivation can never produce them,
write a fixture file per ASCII target, and try to open it by the hostile
spelling.

MEASURED 2026-11-02 (darwin 25.6.0, case-insensitive APFS, main 8dab9487):
40 candidates — fullwidth `U+FF53`/`U+FF33`/`U+FF4B`, roman numeral `U+2170`,
circled `U+2460`, superscript `U+00B2`, the solidus family `U+2044`/`U+2215`/
`U+FF0F`, the full-stop family `U+FF0E`/`U+2024`/`U+FE52`, dotless `U+0131`,
angstrom `U+212B`, `U+1E9B`, `U+0345`, `U+1FBE`, `U+FB29` and 23 more — each
as a bare name and substituted at every position of `config`, `custom`,
`settings` and `posse`: **zero resolve**. All eleven table rows resolve as the
table says; `U+0130` opens nothing. So APFS's equivalence here is case fold
plus normalization and NOT compatibility, which is exactly what the
derivation models — the premise holds, and nothing in the suite says so.
Filed as the live finding on ranger-base-3pm6b.

- **Keyed on what could falsify it, not on what it already holds.** A census
  of "every rune" that keys on case folding re-finds the thirteen rows it was
  derived from. The useful question is which equivalence class the derivation
  *structurally cannot* express.

## The rest of the method, in one line each

- **Both readers of a two-reader wall, mutated separately.** Cutting the nine
  multi-character rows reds the derivation and the landing rows; changing only
  the belt's `b.WriteString(a)` to `a[:1]` reds exactly the four landing rows
  at *"the land belt lets %q through"* with the hook still refusing. Two
  mutants, two readers; the shared table reds both at once and proves neither.
- **A merge-back replay is checked by its REMOVED lines, not its diffstat.**
  For each path, the lines the replay removed minus the lines the original
  commit removed is the silent-revert set. For ranger-base-m8kfv that set was
  two prose lines — the pinned class count the close declared as its only
  conflict — and empty everywhere else.
- **A state close expires in hours on this box, and that is not an escape.**
  ranger-base-7qnag closed on `0.5.3+25686a8c` as "the tip of main, zero lag";
  a third install made it `0.5.3+8dab9487` before I read it, still the tip,
  still zero lag. Re-measure and report the current reading rather than
  scoring the stale number wrong.
