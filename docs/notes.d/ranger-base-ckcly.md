# One space fewer, and check 4 went quiet again (ranger-base-ckcly)

Two findings from ranger-base-bdjot's verify of ranger-base-68631's close,
both in ADR 0024 D2 check 4 — the vuln-private residual scan. 2026-11-13,
this box (linux 7.2.15 x86_64, go1.27.3, 16 cores).

## Finding 1: the continuation the fix was named after is the one it missed

ranger-base-68631 finding 1 widened check 4's scope from a LINE to a bullet
plus its continuation, because a description written under a bare id is how
anyone writes a "Not closed here" entry with detail. It caught the indented
spelling and missed the one a space to its left:

```
- <open id>\n  <text>    flagged, rc 1
- <open id>\n<text>      SILENT, rc 0
```

Markdown calls the second a *lazy continuation* and reads it as part of the
list item; `vulnPrivateUnits` read it as a block of its own, because its pop
loop dropped every opener sitting at or past the current line's indent, and
a column-0 line is at every opener's indent. The file's own comment said a
BLANK LINE is what closes a unit, so a reader of it would have believed the
shape was charged.

**The fix**: a line that opens no block of its own never pops a LIST item
off the opener stack, whatever its indent. Five lines, no deletion.

**It is the list item alone, and that is the whole of the care in it.** The
obvious spelling — break for any opener at equal indent — also charges the
second line of a WRAPPED HEADING to the heading above it, which markdown
does not do (a heading is a leaf block; the next line opens a new
paragraph). MEASURED over every tracked markdown file, each bd-shaped id in
a file treated as an open vuln-private id:

| grouping | ids flagged, 674 files at the verify sha | newly flagged |
|---|---|---|
| shipped | 5309 | — |
| break for any opener at equal indent | 5311 | **2** |
| break for a LIST item only (landed) | 5309 | **0** |

Both of the two are in `docs/notes.d/ranger-base-nr3qf.md`, which is written

```
### Finding 1: the landed check's order-only filter (escaped from
ranger-base-r3pqf)
```

— a bare id on the wrapped second line of a heading, which is the compliant
shape, and refusing it prints a remedy the writer has already performed.
That is ADR 0068's class, so the narrow rule is the one that landed, and the
wrapped heading is pinned as a control in both directions: it reds under the
wider rule, and the five lazy-continuation rows red on the shipped tree.

**Three residuals stay open, now all three named in
`vulnPrivateUnits`' comment** (the old one said "THE RESIDUAL IS DELIBERATE
AND IT IS THE PARAGRAPH" — definite and singular, with two more ways past
the same scope):

1. the separate paragraph, unchanged and already pinned;
2. a description written as a block of its OWN under the bare id — a nested
   bullet, numbered item or heading — which is bought rather than left,
   because `ranger-base-lcuec.md`'s own section names ANOTHER bead that way;
3. a prose RUN at column 0 with no list item above it. Markdown reads one
   paragraph; this grouping reads two units. MEASURED cost of closing it —
   letting a non-opener continue a non-opener too — is **36 ids newly
   flagged over the 679 tracked markdown files of this tree**, every one
   ordinary prose naming a bead id on its own line. ADR 0068's class again,
   so it stays.

The hit line says CONTINUING and no longer "indented under it": a committer
sent looking for an indent that is not there has been sent to the wrong
place to look, which is the same reasoning that made the message name the
unit rather than the line in the first place.

## Finding 2: row 6 named two constants and one got the pin

ranger-base-68631 finding 6 was filed against both of check 4's remedy
sentences — `VulnPrivateWayThrough` and `VulnPrivateMessageWayThrough` —
over a sentence that promised decorations an eleven-byte whitelist refused.
The close's answer, `TestVulnPrivateHitsAcceptsEveryDecorationTheRemedy
Promises`, reads the first constant and only it. The second had no test
reader anywhere in the tree (reader census 2026-11-13: two comment lines and
one call site in `gates.go`), so it could drift back to exactly the shape
the finding was filed over in silence — and it is the arm that matters more,
by finding 5's own reasoning: git normalizes what it stages and does not
normalize a commit message.

`TestVulnPrivateMessageRemedyPromisesOnlyWhatTheRuleAllows` is the same
shape one constant over — each row a noun the sentence uses and a line that
noun describes, plus the half the sentence refuses (it must still name a
LETTER, because that is the rule `vulnPrivateBare` implements).

MEASURED through `go test -overlay`, tracked tree never modified, every
overlay confirmed to compile with `go vet -overlay`:

| mutant | before | after |
|---|---|---|
| the message remedy re-promises what the rule refuses | GREEN | RED |
| the message remedy reverts to a byte list | GREEN | RED |

## Not closed here

- ranger-base-2nj35 — check 3's two remedy sentences
  (`IdentityWayThrough`, `IdentityMessageWayThrough`) have no test reader
  either, so the same class is live one check over. Filed, not fixed: check
  3's rule is a derived identity-literal match and its remedy makes two
  promises check 4's does not, so it is its own rows and not a second loop
  over these.
