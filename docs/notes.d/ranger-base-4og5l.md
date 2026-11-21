## The vuln-private judge's units stop at a file boundary and read a Go comment's markdown (ranger-base-4og5l)

Check 4's content arm (ADR 0024 D2, `internal/posse/vulnprivate.go`) judges a
bead id by the UNIT it sits in — a line plus the lines that continue it — and
two things it could not see made it refuse text AGENTS.md's own ruling
prescribes. Both are ADR 0068's class: a refusal whose printed remedy ("write
the bare id alone") the writer had already performed.

1. **No file boundary.** The arm's text was
   `git diff --cached -U0 … | grep -a '^+' | grep -av '^+++'` — one stream of
   every staged file's added lines, with every file and hunk boundary grepped
   out of it. So the grouping ran ACROSS files: a list item opened in an early
   file (by path order) kept its unit open over every later file's added lines
   until a blank added line turned up. The shape the ruling prescribes is a
   "Not closed here" section of bare ids, and such a section is usually the
   last thing in its file.
2. **A comment marker opened no block.** `vulnPrivateListItem` reads a bullet
   after leading whitespace and a `/` is not one, so no line of a Go comment
   could open or pop anything, and `//   - <id>` read as a continuation of
   whatever stood above it. Since ranger-base-cq3vi widened this arm from
   markdown to every staged path, that left the only reliable way to name an
   open residual's id in Go source not naming it there.

### The fix

- The `+++ b/<path>` header comes through the pipeline now (check 4 alone —
  every other reader in the hook greps line by line and has nothing to group)
  and `vulnPrivateUndiff` blanks it, path and all: a blank line is already
  the token that closes every open unit. Which lines are blanked is
  byte-identical to which lines the second grep dropped, `^+++`.
- `vulnPrivateUncomment` cuts a leading `//` and ONE following space — go/doc
  /comment's own reading, where `// ` is the gutter and `//\t` opens a code
  block — once, before the grouping, so nothing below it knows what a comment
  is. Only `//`: `#` already opens a heading, `*` and `-` are already bullet
  markers, and no tracked `.go` file in this tree opens a line with `/*`.
- A comment block's EDGE closes every open unit, either way, exactly as the
  file boundary does one level up. Without it the comment reading buys a new
  false positive in place of the one it fixes: `// Not closed here:\n// -
  <id>` ends a block on a list item, so the `func` line below it reads as
  that bullet's lazy continuation.
- `RunGatesVulnPrivateScan` answers a text of nothing but boundaries before
  it queries bd, so a commit that stages only deletions — all headers, no
  added lines, where the shell's `[ -n "$posse_added" ]` used to skip the arm
  — still pays no subprocess.

The three residuals `vulnPrivateUnits` declares deliberate (the separate
paragraph, the nested block, the prose run at column 0) are untouched, and
their pins are unchanged.

### End to end, through the shipped subcommand

MEASURED 2026-11-21, `printf … | posse gates vuln-private-scan` against this
instance's live bd store, one open `vuln-private` id (written `<open id>`
here, because a still-open residual is named by its id and nothing else and
this fragment is public), every line carrying the content arm's `+`:

| stdin | before | after |
|---|---|---|
| `- <open id>` then a later file's `\t// …` line | **rc 1** | rc 0 |
| the same with `+++ b/<path>` between them | **rc 1** | rc 0 |
| `- <open id>` then a later file's `\tcode` line | **rc 1** | **rc 1** |
| the same with `+++ b/<path>` between them | **rc 1** | rc 0 |
| `## Not closed here`, blank, `- <open id>` | rc 0 | rc 0 |
| an earlier bullet, `// a comment`, `//   - <open id>` | **rc 1** | rc 0 |
| the same with `+++ b/<path>` before the comment | **rc 1** | rc 0 |
| `// Not closed here:`, `// - <open id>`, a `func` line | rc 0 | rc 0 |
| `//   - <open id>` then `//     the heap overflow …` | **rc 0** | **rc 1** |
| nothing but `+++ b/<path>` headers | rc 0 | rc 0 |

Three readings in that table are worth naming. Row 3 — a bare id continued
by a later file's line with NO header in the stream — still reds, and that
is the point rather than a residual: the header is what the arm always
emits, so the only stdin that still crosses a boundary is one the live
pipeline cannot produce (row 4 is the live shape). Row 2 is the skew case
read from the other side: an OLD binary handed the header reads it as
content, `++ b/<path>`, and charges it to the bullet. And the last row is a
false NEGATIVE closing — the one escape the unit grouping exists to close,
which `.go` was exempt from for as long as a comment marker opened no block.

### The census

MEASURED 2026-11-21 on this box, git 2.56.0, over the whole tracked tree:
2,115 files (`git ls-files` minus `.beads/*.jsonl`, binary files dropped as
git drops them), 688,033 added lines, and an id set of the 2,652 distinct bd
ids spelled anywhere in that corpus — EVERY one of them read as an open
vuln-private id, which is the worst case and not this instance's live set.
Two readings of the same corpus, each file rendered as if wholly added:
per-file (one judge call per file) and whole-tree (one call over the
concatenation, in path order, with or without the `+++ b/<path>` headers).
Flagged results are compared as (file, id) pairs, because a unit's opener
line moves when the grouping changes and a line-keyed diff reads that as
churn.

| reading | flagged pairs | vs. per-file, same code |
|---|---|---|
| before, per-file | 13,190 | — |
| before, whole-tree, headers grepped out | 13,190 | **3 pairs mis-attributed across a file boundary** |
| after, per-file | 13,169 | — |
| after, whole-tree, headers kept | 13,169 | **0 pairs of difference** |

So the boundary is exact: with the headers, judging the tree as one staged
diff flags what judging each file alone flags, over 688k lines. Without them
the live tree already mis-attributes 3 pairs — this bug caught in the tree
and not only on a fixture.

The per-file delta, which is the comment reading alone: **30 (file, id) pairs
stop being flagged, 9 start.**

The 30 are the point. Every one is a comment block charged together with the
code around it — most often a labelled `// Residual: <id>.` line inside a
struct body, where the struct's own opening line used to take every
tab-indented line below it, there being no blank line in a struct. Three of
the 30 are LIVE open vuln-private ids: the gate was refusing, today, over
text the ruling permits. Their sites are named on the bead and not here,
because a still-open residual is named by its id and nothing else, and a
census row is a path.

The 9 are all one shape: an id on its own line inside a tab-aligned table in
a doc comment, under the text of the row above it. That is the
indented-bare-id rule (ranger-base-68631 finding 1, "the text above an
indented bare id is the same escape upside down") doing in `.go` what it has
always done in markdown, so it is a hole closing rather than ADR 0068's
class, and the remedy — a blank comment line above the id — is performable.
Eight are one file's door tables, `internal/treepins/treewidedoor_qa_test.go`
lines 54, 74, 104, 641, 976, 1014, 1069 and 1111; the ninth is
`internal/posse/worktree.go:96`, a numbered bullet whose id sits mid-sentence
in its continuation and which the old reading let by as a false NEGATIVE.
None of the 9 is a currently open vuln-private id, so the live cost of the
widening is zero.

Two variants were measured and rejected on the same corpus:

| variant | flagged pairs that start | why rejected |
|---|---|---|
| marker replaced by SPACES, not cut | 16 | comment content sits two columns deeper than the code around it, so a struct or func line absorbs the comment block below it |
| marker cut, gutter space KEPT | 10 | prose and a tab-indented line land in the same column, so a bare id tab-indented under a sentence in a comment is not charged to it — the indented-bare-id escape, open in `.go` |

### Method

The census is a throwaway in-package test, not a shipped script:
`vulnPrivateHits` and `vulnPrivateUnits` are unexported, and the shipped
subcommand reads its id set from the live bd store, which a shape census
cannot use. Rebuild it by copying `vulnPrivateHits`' unit loop into an
`internal/posse/*_test.go` that reports the opener line instead of only the
id, reading the corpus from `git ls-files`; run it once in a `git archive
HEAD` copy and once in the worktree, over the SAME corpus (point both at the
same checkout, so the only difference is the judge), and compare (file, id)
pairs. Each arm takes ~60s, most of it compiling internal/posse.
