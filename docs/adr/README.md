# Decision records — how to read this directory

Each numbered file here is one decision this harness is built on, written as
the *current* statement of that decision. A decision that changes is amended
in place; how it got there lives in the record's own Lineage table and in git,
never in a second competing page (ADR 0040).

This page is a reading guide, not an inventory. It names no record and no
disposition, so nothing on it can go stale when a decision changes — the two
things a newcomer actually needs from a first open of this directory are the
conventions, and the one command that renders the set in force.

## The set in force, right now

Line 3 of every decision record is its `*Status:` line, and that line *is* the
record's disposition. Render the set live rather than reading it off a page:

```sh
grep -m1 '^\*Status' docs/adr/0*.md
```

Four shapes come back:

- **`accepted <date>`** — in force. Clauses after it on the same line are the
  amendments since, each with its date and the bead that made it. An amended
  record still reads as one decision; there is no second record to reconcile.
- **`superseded <date> by ADR NNNN`** — the decision moved. `NNNN` is where it
  lives now, and the pointer resolves in one hop. The body left behind is
  history: read it for why, never for what is true today.
- **`retired` / `premise dead`** — the decision is gone and nothing replaced
  it. The premise it rested on stopped being true.
- **`proposed`** — written down, not yet ruled on. Accepted is not the same as
  shipped either: a record says so in its own words when its code has not
  landed.

A number is permanent. No record here is renumbered or deleted, and a gap in
the numbering is not a reservation — so every citation in this tree, in a
comment, a persona document, a runbook or a bead, resolves to the page it was
written against.

## Not every file here is a decision

`*.probe.sh` files and the `0013-*-probe.md` traces are supplements: the
reproduction, or the transcript, of what was measured for a decision. They
carry no `*Status:` line by design, which is why a trace contributes no line to
the grep above. That line, not the filename, is what tells the two apart — a
record can have `probe` in its name and still be a decision. When a trace
disagrees with a decision record, the record is what is in force and the trace
is what was seen on the day it names.

## Finding the record for a subject

When you know the subject but not the number, start from **ADR 0040**: it
carries the table of policy homes — which record owns which area, and which
records were folded into it. Otherwise you arrive here by citation: a bead's
`design:` path, or a number written into the code or prose that the decision
governs.

## Writing a new one

- Take the next free number by listing this directory at the commit you are
  writing on.
- Line 3 carries the `*Status:` line, in the shapes above.
- Prefer amending the existing home to adding a competing record. When a
  decision does move, leave a dated superseded pointer at the source and a
  Lineage row at the destination, so the one-hop rule keeps holding.
- Old citations are repointed when their code is otherwise edited. Do not
  churn runtime strings, tests or prompts only to change an ADR number.

## Why there is no list of the decisions on this page

ADR 0040 (amended 2026-09-06) rejected a committed index, in each shape it
could take: a hand-written one states every disposition a second time, which
is the assembly amendment-in-place exists to end; a generated and pinned one
reds every ADR commit that did not regenerate it; and either is a second
writer on every ADR change, which conflicts under the one-writer rule
(ADR 0022) on any busy day.

None of that argues against telling a first-time reader how the directory
works, which is all this page does — two outside-in reviews found the
directory elegant for its authors and unreadable on first open
(`docs/notes.d/ranger-base-vuosd.md` §9, `docs/notes.d/ranger-base-b0fsz.md`
§5), and this page is that finding's fix. It carries the grep and the
conventions and no disposition, so it is not the file ADR 0040 refused.
That record's Decision still reads that `docs/adr/README.md` is not written:
that sentence, not this page, is what needs amending, and it is filed for the
architect as ranger-base-yci3t.
