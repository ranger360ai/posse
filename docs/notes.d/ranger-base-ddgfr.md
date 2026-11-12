# The landed check's filing: what keys the dedupe, and the row D5's table does not have (ranger-base-ddgfr)

ADR 0088 D5 turns the landed check's verdict into beads: one `-t bug` P1
`-l code` per green→red stretch, a comment on the open bead while it stays
red, a comment when it goes green, and nothing at all when the check could
not answer. The runner that produces those verdicts is ranger-base-5fond
(`internal/posse/landedcheck.go`); this is the filer beside it
(`internal/posse/landedfile.go`), and the pins are
`internal/posse/landedfile_test.go` (arm 2, 13 arms, 0.25s).

Dated 2026-11-11.

## The dedupe is a marker in the store, not a second config record

D5's three non-filing rows all need the same thing: *the* open bead for this
red stretch. There were two places to keep that pointer and the choice is
not cosmetic.

D4 already keeps one record, `posse.landedCheck = <tree-id> <green|red>`,
and a bead id could have joined it as a third field or as a second key. What
rules that out is the shape of the process: the filer runs in a goroutine
that the landing pass joins, a `git config` write can fail (it is best
effort by contract, and the runner already warns when it does), and a second
record would have to agree with the first across a window in which the
process can die. Two records that disagree about a red stretch are worse
than none: the one that says "red" sends the next landing looking for a bead
the other one cannot name.

So the open bead is found in the store, by a marker line this rule writes
into the description it later reads back:

```
landed-check: ~/src/posse main (ADR 0088 D5)
```

matched as a WHOLE line, over the open beads in the lane (`bd list
--label-any code`), newest first. It carries the repo as well as the branch
because a beads store here can serve more than one repo — two of this
instance's three stores are reached through a `.beads/redirect` — so "the
open bead" has to be the open bead for this branch of this repo.

Three things that key buys, in the order of what they cost when they are
wrong:

1. **D4's record stays exactly as D4 spells it.** Nothing to keep in step.
2. **It survives a create whose read-back failed.** ADR 0073 B3 has
   `Bd.Create` confirm through a second read, and the interesting failure is
   the one where bd committed the bead and the confirming read did not run:
   exit non-zero, bead in the store, nothing posse can be sure of. A
   config-keyed dedupe would have no id to write and the next red would file
   a twin. The marker is already in the orphan's own description, so the
   next red finds it and comments — pinned by
   `TestAFiledBeadWhoseReadBackFailedIsSaidAndNotFiledTwice`.
3. **It is the house pattern.** verify-after dedupes the same way, out of a
   listing it has already made, and for the same reason
   (`verifiedSources`, verifyafter.go).

## The row D5's table does not have: red→red with no open bead

D5's table is keyed on the transition, and its red→red row says "one comment
on the open bead; no new bead". There is a state that row does not describe:
main is still red, and the stretch's bead is **gone** — a fixer closed it
while main was still red, or the create that was meant to file it lost its
write and its bead with it.

Commenting is impossible and filing nothing would leave a red main with no
open bead at all, which is the one outcome D5's last sentence rules out
("one open bead per red stretch"). So that case files, and the bead says in
its own body that it was filed outside the green→red row and why. It is
bounded: the bead it files is the one the next red comments on, so a close
mid-stretch costs exactly one extra bead and not one per landing.
`TestARedWhoseStretchBeadIsNoLongerOpenFilesOneAgain` is that arm.

## Attribution: the store says what a bead id is, not a regexp

The window is every commit between the tree last recorded green and the
judged tip, newest first, and each line wants a bead and a persona. This
shop's provenance is the subject line — `<bead-id>: <what changed>`, 599 of
608 commits on main — but `fix: a typo` has that shape too, and a filer that
read English as an id would attribute a red main to the word `fix`.

So the prefix is a **candidate** and `bd show` is the answer: the id appears
on the bead only when the store resolves it to itself (`Bd.Show` refuses a
prefix collision for us), and the persona is the assignee it names, else
whoever filed it — `verifyCloser`'s reading of the same question about a
different event. A landing the store cannot attribute is named by commit
alone, and the bead says so rather than printing a word as a bead id
(`TestASubjectWhosePrefixIsNoBeadIdIsNamedByCommitAlone`).

Two bounds, both of which exist because this runs on the pass that **joins**
the checker: 50 landings in the window (past it the list is declared a
floor), and 12 `bd show` calls per filing, because every one of them can
spend `BdTimeout` before it answers. A `bd` that stops answering mid-window
ends the attribution and says so in the bead; it never ends the filing.

## What a green landing costs: nothing

The filer asks the store a question only on a transition. A green main that
stays green makes **zero** bd calls — the record is the whole of that row —
which matters because this is wall on the launcher's own pass.
`TestAGreenLandingSpendsNoBdCall` measures it from the fake's own call log,
with a red at the end of the same arm so a green-blind fixture cannot pass
it.

## No `discovered-from:` edge, deliberately

ADR 0006 §1's edge is a handoff from a BEAD, and what discovered this is a
landing. The window names every bead in it in the description, which is the
provenance that survives — and `--deps` on a create is the write bd loses:
against a tangled closure the daemon commits the issue and outruns the
client, so the create exits 1 with the bead already filed and the edge
missing (ranger-base-muoo). A red main must not become two beads over an
edge nothing reads.

## A fixture finding worth keeping: the fake bd's created row is narrower than bd's

`fakeBdAppendCreated` (herdr_test.go) puts a created bead into
`fake-list.json` with id, title, description, status, labels and a
timestamp. It does **not** record `-t`, `-p` or `-a`, so a row read back
after a create is silent about the type, the priority and the assignee
however they were asked for — a test asserting `is.Priority == 1` over a
created row reads 0 and fails, and one asserting `is.Assignee == ""` passes
for no reason at all.

Those three are pinned here against the argv posse handed bd
(`ldfArgv`/`ldfCreateFields`), which is what decides them in a real store,
with the label pinned through the row beside them. Noted rather than fixed:
widening a fixture every arm of this package shares is its own change, and
this bead's scope is D5.
