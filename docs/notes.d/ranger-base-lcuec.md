# An open hole is a bead id, not a paragraph (ranger-base-lcuec)

Operator ruling 2026-11-11. posse is a public repo; the bead graph it is
worked from is not. A security close has two halves and they route
differently:

- **The hole the fix CLOSES** may be described in full — in the commit
  message, in the fragment, in an ADR. It is shut. Describing it is how a
  reader a year from now knows why the code is shaped this way, and it is
  what ADR 0024 D1's genre arm means by public work.
- **A residual still open** is named by bead id and nothing else: no
  description, no path, no mechanism, no reproduction, until that
  residual's own fix lands.

## What it cost to learn

On 2026-11-11 three sandbox fixes landed, each closing real ground and each
carrying a `## Not closed here` section that described the next hole in
enough detail to work from. Every one of those commits then could not be
pushed — the push was held while the described residual was open, and since
each fix's section named the NEXT piece of work, the hold did not converge:
closing one residual landed a commit describing the one after it.

That is the whole mechanism. It is not a judgment call about severity; a
public tree that publishes its open holes cannot be pushed until all of them
are closed, which is never.

## Where the rule is written

- `AGENTS.md`, "Landing the plane" — one bullet, beside the other rules
  about what a commit message carries, because that is the moment a seat is
  deciding what to type.
- `docs/notes.d/README.md`'s preamble — one sentence, for the seat writing a
  fragment. The preamble is generated, so the source is the line list in
  `scripts/notes-index.py`.

The "Not closed here" section is still worth writing. It is what tells the
next seat there is more ground. Write it as a bare `- ranger-base-xxxxx` and
leave the rest on the bead, where the audience is already the right one.

## Not closed here

- The `prepare-commit-msg` wall could flag a `vuln-private` bead id followed
  by descriptive text in a staged message, which would make this a gate
  rather than a convention. Not built here, by the bead's own instruction:
  ranger-base-ihp7y.
- The three fragments already in the tree still carry their descriptions,
  and whether they are edited, left, or handled some other way is the
  operator's call, not a seat's: ranger-base-wuuqi.
