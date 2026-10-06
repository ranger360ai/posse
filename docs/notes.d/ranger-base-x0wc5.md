## The same count, in the record that defines the word (ranger-base-x0wc5)

2026-10-05. ranger-base-xed72 took the second copy of the tree-wide door and
pin counts out of AGENTS.md and held the absence. Its own closing pass named
where the third and fourth copies sat: ADR 0067, twice — D1's *"`check` is a
verb and the name of fourteen Makefile doors"* and the appendix row for
`check`, *"as a name, the fourteen Makefile doors are spelled `make
<x>-check`"*. Both live at `5d80c36e`, both held by nothing, and filed rather
than fixed there because a shipped record is a different page.

Same rung as xed72, for the same reason: neither sentence is about how MANY
doors there are — D1 argues that STE's approved *check* collides with a name
this shop already uses, and the appendix row defines the word's noun form.
The numeral was carrying no weight in either, so both now say *the Makefile
doors*, and the one place a count of this class is written is still
`internal/treepins/treewidedoor_qa_test.go`'s head comment, which arm 4 of
`TestQATheHeadCommentsPinAndDoorCountsAreTheMakefiles` derives from the
Makefile. The record's status line carries the amendment (ADR 0040: one
current statement, amended in place).

### The rule shipped to hold three pages matched the spelling of one

`twdProseCount`, as xed72 wrote it, matched a numeral **immediately** before
`pins` or `doors` — which is how AGENTS.md happened to spell it
("forty-nine pins across the two, behind fourteen doors"). ADR 0067's
spelling puts a word in between, and the rule read straight past it. So a
census written the day before to stop this exact defect would not have
caught either of the two copies the same bead's close had already found.

Widened on two axes, each paid for by a sentence that exists:

- the numeral may sit **up to two plain words** from its noun. Plain words
  only — a gap token carrying `.` or `|` would let a match cross a sentence
  end or a table cell wall, and MEASURED 2026-10-05 a cross-paragraph pair
  ("There are fourteen." / "The doors are these.") does not match;
- the noun must be **plural**. That is what keeps the wider gap honest: the
  same record's `door` row says *"a Makefile target that runs one tree-wide
  pin"*, and ADR 0067 carries "no prose pin", "the D5 pin" and "by the pin"
  within reach of a digit. A gap rule that took the singular reds on four
  true sentences in the file it was written for. A count of a class with one
  member is not a sentence this class has ever written.

The record is read by NAME, like the Makefile, the README and AGENTS.md, and
for the reason that matters more than taste here: a
`filepath.Glob("docs/adr/0067-*.md")` is a rooted enumeration, which is the
key `twdTreeWideTests` derives membership of the tree-wide class from — the
arm that doors the class would have become a member of it.

### Verified

MEASURED 2026-10-05, this worktree, five mutations of the real record, each
restored from a scratchpad copy and the clean arm re-run green after. `-count=1`
on every run: the change is a `.md` file, so `go test` serves a cached `ok`
over an edited tree and the first pass of this table was worthless.

| mutation | arm 4 |
|---|---|
| restore D1's numeral (wrapped across two source lines) | FAIL — `["fourteen Makefile doors"]` |
| restore the appendix row's numeral | FAIL — `["fourteen Makefile doors"]` |
| add "There are 14 Makefile doors." | FAIL — `["14 Makefile doors"]` |
| add "There are 14 live Makefile tree-wide doors." | ok — three words out, past the bound |
| "There are fourteen." / "The doors are these." in two paragraphs | ok — no cross-paragraph reach |

`make tree-check` green over the result.

### Left standing

The three-word gap is a known bound, not an oversight: every spelling either
page has carried sits within two, and a rule that reaches further starts
reading sentences about something else. The WALL figures in AGENTS.md and in
the pin's head comment are still two honest readings from two load
conditions (ranger-base-xed72's bead): unifying them is a decision about
which box state the shipped advice quotes, and nobody has made it.
