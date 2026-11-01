# The commit wall's fold model was two runes short of what APFS resolves (ranger-base-qu8wn)

ranger-base-g55ah made the commit wall's class RESOLUTION-equivalent: the
thing the wall defends is what the filesystem opens, and the staged name is
only a proxy for it. Its model of that resolution was Go's simple case fold —
`strings.EqualFold` in the Go belt, and in the sh arm a sed line folding the
two non-ASCII runes whose `unicode.SimpleFold` orbit reaches ASCII.

ranger-base-hrx86, verifying that close, measured the model short. APFS is
normalization-insensitive as well as case-insensitive, so its equivalence is
wider than any case fold. This bead widens both arms to the measured set,
renames the table for what populates it (`constitutionResolveRunes`, an APFS
equivalence table, not a `SimpleFold` census), and replaces the pin that
could only ever red on a bump to GO's table.

## What APFS resolves — MEASURED 2026-11-01, this box

Darwin 25.6, case-insensitive APFS, over every rune `0x80..0x10FFFF`: write
one file per ASCII graphic character (lower case only, since the volume
collides `A`/`a`), then open each rune by name and read back which ASCII
character it landed on. **Four** single runes open an ASCII-named file:

    U+017F  LATIN SMALL LETTER LONG S  ->  s    case fold
    U+037E  GREEK QUESTION MARK        ->  ;    NFC singleton
    U+1FEF  GREEK VARIA                ->  `    NFC singleton
    U+212A  KELVIN SIGN                ->  k    case fold, and NFC to "K"

The middle two are the ones g55ah's model missed: they reach ASCII through
normalization, not case, so `strings.EqualFold` answers false for both
(measured) and the sh arm's sed carried neither. This re-measures
ranger-base-hrx86's census independently and agrees with it exactly.

The negative control is U+0130 LATIN CAPITAL LETTER I WITH DOT ABOVE, which
does **not** open `i` — without it the probe would only be saying "the volume
matches everything". The whole thing is three lines with no build:

    cd "$(mktemp -d)"
    printf 'control\n' > ';' ; printf 'control\n' > '`' ; printf 'control\n' > i
    cat $';' ; cat $'`' ; cat $'İ'

## Nothing was exploitable, and that was the class's accident

Every one of the live 22 class members is ASCII lower case with neither `;`
nor a backtick in its name (measured off `ConstitutionEveryRepoPaths` +
`ConstitutionRepoPaths` at HEAD), so no spelling reachable through the gap
named a member and there was no bypass of the wall. The defect was that
nothing held that property: the day a class member carried a `;` — a runtime
declaring one in `Runtime.ProjectConfig` is all it would take, and the class
is derived from those declarations since ranger-base-t03st — the wall would
have had a silent hole and no pin would have noticed.

Sufficiency is the TABLE's property now, not the class's.

## The pin that could not see it

`TestQAConstitutionFoldRunesAreTheClosedSet` was framed as the guard that reds
"when a Go Unicode table bump adds one ... without it the sh arm would quietly
reopen". It enumerated `unicode.SimpleFold`, so it could only ever red on a
change to GO's fold table, and it was blind to the larger equivalence the wall
actually defends. It was right about what it computed and wrong about what it
proved.

Its replacement, `TestQAConstitutionResolveRunesAreTheClosedSet`, re-derives
the set from a rune-level model of the volume — NFC first, then case, which is
the order that makes U+212A reach `k` by either road — so a bump to either of
Go's tables reds. The model derives exactly the four the filesystem was
measured to resolve, in 0.09s over every rune.

## Single runes are enough, and that is checkable

The table maps one rune to one ASCII byte, so a SEQUENCE equivalent to a
single ASCII character would be past both arms with no row able to express it.
None exists, and the reason is not an enumeration: no ASCII character has a
canonical decomposition, so its NFD is itself and nothing longer can be
equivalent to it. `TestQAConstitutionResolveIsSingleRuneSufficient` checks
that over all 128 and reds if a future Unicode gives one a decomposition.

## The backtick is a SHELL hazard, and it is why both sides go through printf

The sh arm's fold renders as `s///` expressions inside double quotes. U+1FEF's
ASCII byte is a backtick, and inside double quotes a literal backtick opens a
command substitution. MEASURED 2026-11-01: rendered literally, the hook stops
being shell at all — `sh -n` over it exits 2 with "unexpected EOF while
looking for matching \`", because the substitution swallows the rest of the
file. That half is loud and `TestQAInstalledCommitHookParses` already reds on
it.

`constitutionFoldSed` now renders BOTH sides of every `s///` through
`printf` octal, which keeps the rule "whatever the table says" rather than one
re-reasoned for each new ASCII byte. `&` is a replacement's only sed
metacharacter and no row's byte is one; the closed-set pin says so, so a row
that would need an escape reds before it lands.

What parsing cannot say is that the line RESOLVES, and no real class member
carries a `;` or a backtick, so no git fixture reaches those two rows.
`TestQAConstitutionShFoldResolvesEveryTableRune` runs the rendered line over
a probe built from the table itself — that is the only measurement those two
rows have in the sh arm.

## What the fix touched

- `internal/posse/promote.go` — `constitutionResolveRunes` (4 rows),
  `constitutionFold`/`constitutionFoldEqual`/`constitutionResolveASCII`, and
  `foldPrefix` comparing resolved forms instead of calling
  `strings.EqualFold` directly.
- `internal/posse/gates.go` — `constitutionFoldSed` renders both sides
  through `printf` octal; the `posse_cls_fold` comment names the wider
  equivalence.
- `internal/posse/constitutionfold_qa_test.go` — the closed-set pin replaced,
  plus the single-rune sufficiency pin, the rendered-sh-fold run, a KELVIN and
  a U+0130 row in the matcher table, and
  `TestQAConstitutionClassMatchFoldsNormalizationRunes`: a SYNTHETIC class
  whose member carries both `;` and a backtick, matched through the U+037E and
  U+1FEF spellings through the matcher and the land belt. Synthetic on purpose
  — that is the finding restated, since no real member can exercise it.
- `docs/adr/0015-constitution-promotion.md` §3 — the "which suffices because
  every member is ASCII and its own fold" sentence now says which of the two
  facts it means, and the fold paragraph is amended to APFS's equivalence.
