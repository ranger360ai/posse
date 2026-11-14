# The bullet and the diff marker are the same byte (ranger-base-gi6ge)

One finding from ranger-base-uc338's verify of ranger-base-ckcly's close, in
ADR 0024 D2 check 4 — the vuln-private residual scan. 2026-11-14, this box
(linux 7.2.15 x86_64, go1.27.3).

## What failed

ranger-base-ckcly made a line that opens no block of its own continue the
LIST ITEM above it whatever its indent — markdown's lazy continuation.
`vulnPrivateListItem` names four bullet spellings as list items: `-`, `*`,
`+` and a number. The fix reached three of them.

`+` is both a markdown bullet marker and the unified-diff marker check 4's
CONTENT arm exists to discount, and at column 0 they are the same byte. The
discount sat inside the line readers (`vulnPrivateContent`, one
`TrimPrefix` called from `vulnPrivateIndent`, `vulnPrivateListItem`,
`vulnPrivateOpensUnit` and `vulnPrivateUnits`), so it ran on the MESSAGE
arm's lines too — where a `+` is a bullet and nothing else. A message line
`+ <id>` lost its `+` before anything could read a bullet there, read as
prose at indent 1, carried no lazy continuation, and the description below
it bought a unit of its own.

MEASURED end-to-end through `posse gates vuln-private-scan` built from the
tree at `fb20f8dc`, one open vuln-private id, description on the next line
(rc 1 = refused, rc 0 = silent):

| text | rc |
|---|---|
| `- <id>\n<text>` | 1 |
| `* <id>\n<text>` | 1 |
| `1. <id>\n<text>` | 1 |
| `+ <id>\n  <text>` (indented) | 1 |
| **`+ <id>\n<text>`** | **0** |
| `++ <id>\n+<text>` (content arm) | 1 |

One space is the whole of the difference between the fourth row and the
fifth, and the fifth is the shape the rule exists for. The exposure is the
MESSAGE arm, which is the arm ranger-base-ckcly's own finding 2 argued
matters more: every line the content arm reads carries a diff `+`, so a
markdown `+` bullet arrives there as `++ <id>`, keeps one `+` after the
discount and is read correctly — and git normalizes a commit message not at
all.

It was none of `vulnPrivateUnits`' three declared residuals. Markdown reads
`+ <id>` as a list item, so there IS one above the description, and the code
says so at `vulnPrivateListItem`'s own `case`.

## The fix: the discount moves to the arm boundary

`vulnPrivateUndiff` strips one leading `+` per line ONCE, over the whole
text, and only for the content arm. Below it nothing knows what a diff is:
`vulnPrivateIndent`, `vulnPrivateListItem`, `vulnPrivateOpensUnit` and
`vulnPrivateUnits` read plain markdown, which is what lets a `+` be a
bullet at all. The unit text loses the marker along with the readers, and
that costs nothing: `vulnPrivateBare` asks only about letters.

**The arm is a parameter and never a reading of the text.** `VulnPrivateArm`
is `RunGatesVulnPrivateScan`'s third argument, and `gates.go` renders one
arm each — `vulnPrivateChecks(" --message")` for `vulnPrivateSource`'s
message half, `vulnPrivateChecks("")` for `vulnPrivateContentCheck`. The
text cannot say which it is: "every line begins with `+`" is true of the
content arm by construction and can be true of a commit message by
accident, and it is the accident that would be read wrong.

**The content arm is the ZERO, and that is about skew rather than taste.** A
rendered hook is a copy of the binary's render, and only `posse gates
install-hooks` and a session create re-render one (`hookfresh.go`) — so an
installed hook older than the posse on PATH passes no arm at all. With the
content arm as the zero, that window behaves exactly as the tree behaved
before this fix: the message-arm hole stays open until the hook is
re-rendered, and nothing that worked stops working. With the message arm as
the zero it would be the CONTENT arm that silently lost its continuation
reading in the same window — a new hole opened by a fix, in the arm that was
never wrong.

**The alternative was dropping `+` from `vulnPrivateListItem`**, and the
`++ <id>` rows are what it would have cost: a markdown `+` bullet staged in
a file is a bullet, and the content arm has to keep reading it as one. That
mutant reds three pins (below).

## The sentence that hid it, rewritten

`vulnPrivateContent`'s comment reasoned the risk away: a message line that
genuinely begins with a `+` "loses a column of indent, which can only make a
unit narrower." It loses a column of indent AND its list-item identity, and
since ranger-base-ckcly the list-item identity is what carries the
continuation — narrower by exactly the line that holds the description. The
understatement is in `vulnPrivateUndiff`'s comment now, quoted, with what it
got wrong.

`vulnPrivateUnits`' third residual also gained the sentence
ranger-base-uc338's shape census earned it: a blockquoted bullet
(`> - <id>` then `> <text>`) falls INSIDE that residual rather than beside
it, because nothing here reads `>` as anything, so `> - <id>` is prose and
the line under it has no list item above it.

## Measured

Mutants through `go test -overlay`, the tracked tree never modified (md5
confirmed unchanged after the battery):

| mutant | pin that reds |
|---|---|
| discount the message arm too (= the defect) | `…ReadsAPlusBulletInTheMessage` |
| never discount (content arm loses its marker) | `…ReadsTheContentArmsDiffMarker` |
| drop `+` from `vulnPrivateListItem` | all three of the above plus `…FlagsADescriptionOnTheContinuationLine` |
| the message arm stops spelling `--message` | `TestVulnPrivateChecksSpellsTheArmPerArm` |
| the content arm spells `--message` | the same |

Cost census, both binaries (the tree at `fb20f8dc` and this one), over every
tracked markdown file that names an open vuln-private id at all — 7 of 715 —
in both arms: **2 (file, id) pairs flagged, identically by both, 0 newly
flagged and 0 lost.** The content arm is provably unchanged (the same
`TrimPrefix` per line, relocated; `vulnPrivateBare` reads letters, so the
marker's absence from the unit text changes nothing), and the census says
the message arm's widening costs this tree nothing either.

The `+`-prefixed rows of
`TestVulnPrivateHitsFlagsADescriptionOnTheContinuationLine` moved out of it
and into `TestRunGatesVulnPrivateScanReadsTheContentArmsDiffMarker`: that
table is `vulnPrivateHits`, which now sees markdown and nothing else, and
the diff marker belongs where the only reader of it is.

The 2 pairs are pre-existing on `main` and the detail is on
ranger-base-gi6ge, where the audience is already the right one: the bead ids
are below.

## Not closed here

- ranger-base-bfla6
- ranger-base-w6ip0
