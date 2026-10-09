## Three documented couplings nothing compared (ranger-base-c768n)

Filed 2026-10-09 by the verify of ranger-base-qbbzz, out of
ranger-base-x8bv0, ranger-base-x2vt7 and ranger-base-s1dmh. All three were
correct on the day they were found and measured so; what was missing in each
case was a reading that would notice when one half moved. Each needed a pin
whose subject is the TREE, so each needed a Makefile door as well, which is
why they were one piece of work.

Written 2026-10-09.

### 1. `publicDocBase` — the path half was held and the base half was not

`internal/posse/publicdoc.go` composes every URL a detection refusal hands an
operator, and its header argues that the branch name is PINNED and not
avoided: "`blob/main/<path>` is already the house spelling, www/index.html
ships two of them and `internal/treepins/quickstart_test.go` pins them", and
what makes the URL safe is that it is composed from a repo-relative path "so
one pin can stat that path in the tree".

Both halves of that argument were true of the PATH and neither of the BASE.
The existing pin's comparison is `publicDoc(rel) == publicDocBase+rel`, a
tautology over the base: rewrite `detectionDocPath` and it reds; rewrite the
org or the branch inside the const and it does not. MEASURED 2026-10-09 by
ranger-base-qbbzz's verify and re-measured here before the fix landed — two
mutants, both SURVIVED `TestQADetectionDoorCitesAFilingThatExists`:
`ranger360ai` to another org, and `blob/main` to `blob/no-such-branch`. Host,
org, repo and branch — a 404 for every cited page, or a fork's copy of them,
with every pin green.

The fix is the derivation the header already named without using it:
`TestQAPublicDocBaseIsTheSpellingTheSiteShips`
(`internal/posse/publicdocbase_qa_test.go`) reads `www/index.html`, matches
`https://github.com/<org>/<repo>/blob/<ref>/`, requires the site to spell
exactly ONE such base, and holds the const to it. It is a real derivation and
not a second literal: those hrefs are pinned verbatim, INSTALL.md paths and
all, by `internal/treepins/quickstart_test.go:292,321,326`, so a rewrite of
one side reds here and a rewrite of both reds there. MEASURED 2026-10-09:
both survivors above now red, and the clean tree passes.

### 2. The auto-mode carve-out against the record it quotes

`ClaudeAutoModeCarveOut` (`internal/posse/agents.go`) is documented at its own
declaration as "quoted verbatim from ADR 0070 D2". It was verbatim and
nothing held it. MEASURED 2026-10-09: D2's blockquote with `> ` stripped and
the lines joined on a single space is byte-identical to the const — **862
bytes, 856 runes** on both sides, the gap being the em dashes and the
ellipsis. (ranger-base-qbbzz's finding says 856 bytes; it was counting runes.
The pin logs both.)

MEASURED survivor:

```
perl -0777 -pi -e 's/SIBLING terminal panes/sibling terminal panes/' internal/posse/agents.go
```

then the five `automodeallow` pins plus
`TestQAClaudeAutoModeCarveOutMeetsTheRulesBar` → ok.

That pin holds six phrases and each granted verb's last word, which is the
RULES' own bar and the right thing for it to hold. What no reading reached was
drift from the RECORD. The statement is prose read by a classifier (ADR 0070
D3), so the only thing that makes it posse's standing statement rather than a
seat's paraphrase is that it is the cleared bytes.

`TestQAClaudeAutoModeCarveOutIsADR0070D2Verbatim`
(`internal/posse/automodecarveoutadr_qa_test.go`) extracts the blockquote and
compares. The extraction is bounded by the NEXT decision heading on purpose:
an unbounded scan for the first `> ` after D2 would silently read D3's or D4's
quote if D2 ever lost its own, and be green for a reason unrelated to its
subject. MEASURED 2026-10-09, three mutants, all red: the const drifts (the
survivor above), the ADR's blockquote drifts, and the `**D2 —` heading is
renamed.

### 3. `backup_min_free_mb` — a constant in another unit is not a missing constant

ranger-base-s1dmh gave the seed a `backup_*` block, including
`backup_min_free_mb: 384` (`examples/config.yaml:883`). `DefaultBackupMinFreeMB`
is 384. Nothing compared them. MEASURED 2026-10-09, SURVIVED: change the
documented value to 9999 and both number-census pins PASS, while the census
LOGS the line as "a suggested setting and not a claim about a default" in a
sentence that still reads "the seed documents it at 384 since
ranger-base-s1dmh".

The row in `seedNumberDefaultNotAConstant` was reasoning about the census
MECHANISM and not about the fact: `BackupMinFree` returns
`DefaultBackupMinFreeMB << 20`, so the derivation found no single returned
constant and the census — which compares the reader's own answer — had no one
number to compare. But the default IS a constant, in the key's own unit, and
the seed documents it in that unit. What was missing was the SCALE between
them, and the scale is stated in the reader's own return expression.

So the derivation reads it. `seedReturnedDefault` accepts a Default-shaped
identifier either bare or shifted left by an integer literal (capped at
`seedMaxShift` = 52, which is float64's exact-integer line and the same line
that section's header already drew over the carrier); `seedPairing` carries
the shift; `seedNumberAnswerFor` applies it. The comparison stays a read
through the PRODUCTION reader and never a text compare: the documented text
goes into a scratch config, the instance's own method answers in bytes, and
what it is held against is the constant shifted by the shift the parse read.
The rule is shared with the call-site pass, so a scaled default states its
pairing in both shapes or in neither.

MEASURED 2026-10-09, three mutants, all red: the documented value drifts to
9999 (the s1dmh survivor), the constant moves to 512 and the seed does not,
and the reader's scale changes to `<< 10`. Five pairings now (was four), one
register row left (`grok_guard_week`).

Two things that went with it:

- The planted-tree refusal test moved its near-miss one step along.
  `PlantedShifted` was near-miss 5, reported rather than paired; it is a
  PAIRING now, carrying `<<20` in the expectation so a derivation that found
  the right constant and dropped its scale reds. Its replacement is
  `PlantedTwoScales`, which returns one constant both bare and shifted —
  two stated defaults and no single answer, still reported.
- The DURATION half shares the derivation and does NOT apply the scale: a
  duration is already a scaled integer and nothing in the tree returns
  `Default<X> << n` as one (MEASURED 2026-10-09 — `<<` appears over a
  Default-shaped constant at exactly one site in `internal/posse`). A scaled
  duration pairing arriving there would be compared against the unshifted
  constant, so it is a named failure instead, and whoever writes that reader
  decides what the comparison should be.

### The doors

Both new pins take their root from `qibRepoRoot`, so they are members of the
tree-wide class and would otherwise have been unreachable behind a `-run`
filter in a ~950s package (ranger-base-rulbl's class). Both are folded into
`make doc-check`, whose subject is already prose pins over shipped code and
docs, and both are named in `treewidedoor_qa_test.go`'s enumeration. The
register's count sentence went fifty-six to fifty-eight pins at fourteen
doors, re-measured as that file requires: **66-139s** over three warm
`make tree-check` runs (MEASURED 2026-10-09 — 115.3s, 139.0s and 66.3s in
that order, one-minute load average 9.3, 54.2 and 63.1 before the three, one
suite slot held by a sibling seat throughout, so the FASTEST run was the one
at the highest load). The two pins each report 0.00s, below `go test`'s
per-test resolution.

Finding 3 needed no new door: the two pins it changes are already behind
`make seed-check`.
