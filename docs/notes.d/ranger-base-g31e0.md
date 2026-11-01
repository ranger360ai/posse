# The shape neither half of the derivation reads: a root list that is both (ranger-base-g31e0)

2026-10-31. ranger-base-tw3t7's verify of four closes found two live findings,
and its bead says once why they are one gap: a QA pin in internal/posse that
ranges over a list of relative literals **mixing the package's own directory
with a climb out of it** — `for _, d := range []string{".", "../../cmd/posse"}`
— is derived as tree-wide by **neither** half of the register in
internal/treepins/treewidedoor_qa_test.go, precisely BECAUSE it does both. The
climb fence cannot see the climb; the enumeration seed discards the readable
`"."` because the list also holds the climb. So the pin is outside the class,
gets no Makefile door, and the ~950s arm run is its only route — which is the
whole defect ranger-base-nt0pi, ranger-base-8yv7z and
ranger-base-m94g2/ranger-base-na4rm exist to close.

Three pins were live in that shape. This is what each half now reads, how the
three were dispositioned, and the one decision that is neither finding's.

## The two fixes, one to each half

**The fence** (`twdClimbSet`, ranger-base-8yv7z) bound at an `AssignStmt` or a
`ValueSpec`. `roots := []string{"..", "../../cmd"}` binds the SLICE, and
`climbs()` answers false for a `CompositeLit`, so the elements never entered the
set and the range value was never bound at all. The set now carries a second
map — `list`, a name bound to a `[]string` holding at least one climb that NAMES
a path below the root — and `extend` has a `RangeStmt` arm that binds the range
VALUE from it (`climbingList`). The two maps are separate for the reason
`twdRelEnv` keeps `val` and `list` apart one rule over: ranging a STRING yields
its runes, so a range variable may inherit a climb only from something the rule
knows is a slice of paths.

The jurisdiction line is unmoved because it is held **per element**: an element
is tested by exactly the pair of conditions `bind` applies to a scalar, so
`[]string{"..", "../.."}` — bare roots, naming no path below the root — stays
`twdRootExprs`' and the hidden arm's business, and widening to lists cannot take
`detectionRig`'s nine callers with it (ranger-base-sx2dq).

**The seed** (`twdRelTreeList`, ranger-base-na4rm's third) voided the whole list
on one unreadable element, and its head comment gave the reason: *"a range over
that list walks the element the rule could not read too, so claiming the list
would be claiming a root this file does not know."* That is right in
internal/treepins, where a `../` climb LEAVES the tree. It is wrong in
internal/posse, where a climb is how a test REACHES the tree: the unreadable
element makes the reading MORE tree-wide, and refusing the list threw away the
readable `"."` beside it — the one-package census ranger-base-m94g2 had just
decided is a member on its own.

The fix is scoped to the element KIND and not to the package, which is the one
place this bead went narrower than the finding asked. A **climbing** element is
skipped; every other kind of unreadable element — a variable, a scratch
directory, an absolute path, a helper's return — still voids the list. That
keeps `twdRelTreeList` ONE rule with one caller-independent answer, and it keeps
internal/treepins' own fixture row green (`[]string{"cmd", d}` with
`d := t.TempDir()` is still nothing), which a package-keyed fork would have had
to re-decide. A list of nothing but climbs still reads as nothing.

## Which half takes which site was MEASURED, not preferred

MEASURED 2026-10-31, darwin 25.6.0, go 1.26.10, at main 13bce8c3, with both
fixes in and before any site was touched:

| site | fence | seed |
|---|---|---|
| `internal/posse/visibility_test.go` `[]string{"..", "../../cmd"}` | names it | reads nothing |
| `internal/posse/gatedkeychain_test.go` `[]string{".", "../../cmd"}` | names it | **derives it** |
| `internal/posse/exactmodel_qa_test.go` `[]string{".", "../../cmd/posse"}` | names it | **derives it** |

The posse half of the class went from 56 members to 58 on the seed fix alone,
and to **59** at HEAD once visibility_test.go took the root (50 in
internal/treepins, unchanged — the seed's element rule is the same one rule and
nothing over there is spelled this way). So the two findings really are two: the two mixed lists are derived through
their readable `"."`, and visibility_test.go's is not, because every element of
ITS list climbs and the seed can read none of them.

Which decided the remediation per site, rather than one rule for all three:

- **visibility_test.go took the root from `qibRepoRoot`** — the house fix a
  third time (ranger-base-nt0pi's three, ranger-base-xndgk's five). Nothing
  derives that reading otherwise, so the fence is its only route. It walks
  `filepath.Join(root, "internal")` and `filepath.Join(root, "cmd")`, the same
  population `".."` and `"../../cmd"` had, and reports paths relative to the
  root so its failure message is unchanged. Mutation-checked: a planted
  `RHQ_VISIBILITY_OVERRIDE=i-mean-it` under cmd/posse reds it and names the
  file.
- **The other two are doored where they stand.** A census of internal/posse's
  own sources is a member AS a relative read — that is ranger-base-m94g2's
  decision, and the fifteen it doored were not asked to take a root. Demanding
  one of the mixed spelling alone would treat it more harshly than the
  `"."`-only sibling the house blessed, for the same door.

## The decision that is neither finding's: the fence ARM reads `relEnum`

`TestQAOneRepoRootHelperInTheTestPackage`'s climb arm says *"no rule in this
file can see it, the pin gets no door and nothing says so"*. Once the seed
derives a mixed list, that sentence is FALSE for the two sites above and the
door is in the Makefile. A prohibition that prints a false reason is worse than
one that stays quiet, so the arm now reads `f.climbEnum && !f.root &&
len(f.relEnum) == 0`.

That is not a new idea in the arm: `!f.root` has always been the same
condition for the DIRECT rule — a body that takes the root from the one helper
is derived, so its climb makes nothing invisible. `len(f.relEnum) == 0` is that
condition for the third seed, and it is sound in the same way: `relEnum`
non-empty means the body itself seeds the enumeration reach, so either it is a
member or its callers are, and in both cases arm 2 requires a door. The RULE's
answer is untouched — `twdFixtureClimbs` still reports true for a mixed list,
which is what the witness rows assert — so a future site that climbs with no
readable element beside it is fenced exactly as before.

## The witness closes as a regression, not as a record

`internal/treepins/climbidentholes_qa_test.go` (landed by the verify lane at
3fe66e84) carried four `hole` rows asserting the gap, with the instruction to
flip the want rather than delete the row. Flipped: all four are a YES from at
least one rule now, the two mixed rows from both. The field is `filed` rather
than `hole`, and the floor changed with it — it required four holes, and now
requires four `filed` rows AND that every one of them be a YES. So reverting
either fix cannot be made green by flipping a want back: the floor fails too,
with its own message naming which rule to restore.

The register's own fixture table grew the same rows, because that is the table
the door runs: a range over a named list, a range over an inline list, a mixed
list — all true — and three controls that must stay false (bare roots only, a
`t.TempDir()` list, a range variable that roots no walker).

## Doors

All three went into `$(QA_PKGCENSUS_PINS)`, the untagged one-package-census
door. No new door, and the radius is why it needed saying in the Makefile: that
variable is named for a class that was one package when ranger-base-na4rm
doored it, and these three ask the same never-question one directory further
out — `.` plus cmd/, `.` plus cmd/posse, internal/ plus cmd/. The subject is a
reading of this repo's own Go sources whose file SET is discovered, so a bead
that adds a .go file in that radius reds them, which is this door's subject and
the same key the fifteen are in on. All three are arm 1 or shared, so the
untagged door reaches them with no `-tags`.

0.23s between the three (0.02s, 0.02s and 0.19s, three warm runs agreeing to
the hundredth, at a one-minute load average of 6.00). The class is one hundred
and twelve pins and seventeen doors.

## Mutation-checked, three ways

Each applied to the landed tree, measured, reverted (MEASURED 2026-10-31, after
a green `make test`; the register's own arms are the only observers, so no
sibling suite could see the planting):

1. **the fence's `RangeStmt` arm disabled** reds the witness's four `filed` rows
   AND the register's own fixture table, three rows of it, with
   `TestQAOneRepoRootHelperInTheTestPackage` naming each planted spelling. Both
   tables, because the rule is one and they drive it through the same harness.
2. **the seed's climb skip disabled** reds three arms at once and is the useful
   one: the witness's two mixed rows, the fence arm on the two sites it then
   has to speak about again, and — the one that matters — arm 2, with the posse
   class back to 57 and `$(QA_PKGCENSUS_PINS)` naming two tests *"no longer
   derived as tree-wide ... the door still passes; the pin is back to being a
   package-tree run away."* That is the whole defect printed in its own words.
3. **the fence arm's `len(f.relEnum) == 0` dropped** reds that arm on the two
   doored sites — which is the message that would be FALSE in front of a seat,
   and the reason the guard is there.

## Not a reopen

ranger-base-8yv7z and ranger-base-na4rm both deliver what their descriptions
asked, mutation checks included and re-run by the verify lane; neither is
reopened (ADR 0006 §2). ranger-base-8yv7z's "once the fence reads an identifier
it finds six" was six of nine, and ranger-base-na4rm's own description named and
waived this risk in as many words: *"Not a new rule to be wrong about: the same
code, a second caller."* The second caller wanted one line of it different.
