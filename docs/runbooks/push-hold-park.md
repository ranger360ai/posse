# RUNBOOK — parking a held commit, and the refs that still carry it

The procedure for a commit that must come off the base branch and must not be
lost: a push the publication gate refuses, a commit whose content turns out to
belong to another audience, anything that has to be held while it is rewritten.
You park it, you move the base off it, and then you find out which other refs
are still pointing at it.

This is the operator's and the push delegate's page, not a seat's. A seat's
part in it is the last section.

The detective half is `scripts/park-carriers.sh` (ranger-base-zlvyk) and the
enforcement half is a guard in the launcher's landing chain
(ranger-base-3s2ig). Both read the ref namespace below, and neither can ask
you what you meant.

---

## The namespace is an enforcement input

A park is a branch ref under one prefix:

```
refs/heads/parked/<name>
```

Exactly that spelling. Two programs consume it today — the census's own glob
and the landing guard's constant — and more may. **A park spelled any other
way is a park neither of them can see.** `refs/parked/<name>`, a tag,
`hold-<name>`, a branch called `parked-<name>` with no slash: the census
prints nothing, the guard refuses nothing, every carrier branch lands exactly
as it would have before either existed, and **nothing reds**. The park's own
author is the only person positioned to notice, and that is why the spelling
is written here rather than left inside the two programs that read it.

posse writes none of these refs. Parking is a hand procedure at the gate, by
decision: it runs once per hold, so the census is a script rather than a verb
(ranger-base-zlvyk, "What was decided"). A hand convention that is also an
enforcement input has to be written where the procedure is, which is this page
(ranger-base-jv2q3). `internal/treepins/parknamespace_qa_test.go` holds this
page and every program that reads the namespace to the one spelling, so a
reader who changes one of them is told about the others.

**The held set of a park is `git rev-list <park> --not <base>`** — not the
park's whole history. A park ref also reaches the base it was cut from, and
everything on that base is on every branch by construction. What makes a
commit held is that the park is the only intended way to reach it.

---

## The procedure

**1. Park the commits before you move anything.** The park ref is what keeps
them reachable; make it first and nothing in the rest of this page can lose
them.

```sh
$ git branch parked/<name> <the tip that carries them>
$ git rev-list parked/<name> --not <base> | wc -l
```

**Verify:** the second command counts the commits you meant to hold, and
`git for-each-ref refs/heads/parked` names your new ref with the prefix
spelled as above.

`<name>` is yours, and the convention in use is
`<bead-id suffix>-<short sha of the held tip>`, so the hold bead is findable
from the ref and two parks of the same bead do not collide.

**2. Move the base off them.**

```sh
$ git reset --keep <the last commit that stays>
```

Blast radius: the base branch of the repository checkout you are standing in,
and the working tree with it. `--keep` rather than `--hard` on purpose — it
aborts rather than discarding a local change to a file it would have to
rewrite, and in a checkout with concurrent writers that refusal is the whole
point. If it aborts, find out whose change that is before going further.

MEASURED 2026-11-22, git 2.56.0, in a scratch repository built to the shape
above: over a clean tree the base moves and the park still holds its commit;
with an uncommitted change to a file the reset would have to rewrite it exits
128 with `error: Entry '<path>' not uptodate. Cannot merge.`, the base does
not move, and the change is still there.

**Verify:** `git log --oneline <base> -1` is the commit you named, and
`git rev-list parked/<name> --not <base>` still counts the held commits — if
that count went to zero, the base did not move off them.

**3. Census the carriers, in the minute after the reset.**

```sh
$ scripts/park-carriers.sh
```

Exit 0 says no landable branch reaches a held commit. Exit 1 names every one
that does, with the worktree holding it, and that is the list the hold is
actually about. The reset removed ONE ref's claim on the held commits; every
other ref already cut from that tip still points straight at them, session
branches included, because a session branch is cut from the base's tip at
dispatch and the park does not go back and move it.

**The carrying set is closed at the moment of the park**, which is what makes
this complete rather than a sample: a seat dispatched afterwards cuts from the
reset base and carries nothing. So the census's answer in that minute is the
whole answer — and it is still correct whenever it is run.

Two classes the census prints and does not count, because neither can put a
held commit back on the base: another park that contains this one, and
posse's own `refs/posse/*` records, which keep the objects reachable so gc
does not prune held content the park's own ref no longer names.

**4. Resolve each carrier.** A branch a live seat has checked out cannot have
its ref moved at all, and the base cannot be reset under a seat that is
mid-flight — which is why each carrier prints with its worktree. The landing
guard is what holds those: it refuses a landing whose `<base>..<branch>` range
reaches a held commit and names the park in the refusal, on every sweep, until
that seat ends and the branch can be dealt with. A standing block for hours is
the gate working.

**5. File the hold bead, and rewrite from there.** The rewrite is a new commit
on a current base, never an amend or a rebase of the held one. What the hold
was about stays on the bead: a public commit message or notes fragment may
describe a hole its fix CLOSES, and a residual still open is named by bead id
and nothing else (AGENTS.md).

**6. Leave the park ref alone while the hold stands.** It is the only intended
way to reach the held commits and it is what both programs read. Deleting it
does not unpublish anything; it only takes the guard's subject away.

---

## Verify the machinery, not just this park

```sh
$ scripts/park-carriers.sh --self-test     # the census says YES with a carrier, NO without
$ git for-each-ref refs/heads/parked       # every park, and the one place the spelling shows
$ make treepins-check                      # includes the pin over this page and the namespace
```

The self-test builds the hold's exact shape in a scratch repo — a commit, a
branch cut from that tip, the park, the reset — and proves the census can
still say both things. It touches nothing on the box.

---

## If you are a seat and your branch was named a carrier

Nothing to do with the ref, and nothing to force. Your branch cannot land
while it reaches a held commit, and that refusal is correct: the launcher's
fast-forward would fail on it, and the replay behind that fast-forward would
put the held commits back on the base under new shas, which is the failure the
guard exists for (ranger-base-3s2ig, measured). Commit the work you want kept,
say on your bead that the branch is a carrier and name the park, and let the
hold bead's owner move it. Do not rebase onto the park, and do not delete it.

---

## If posse should own the namespace instead

A namespace posse writes — `refs/posse/parked/*`, beside the
`refs/posse/merge-blocked/*` the landing chain already writes — would end the
inheritance: the spelling would be one constant in one program, and no
procedure page would have to carry it.

It needs the verb that does the parking in the same change. Moving the
namespace without moving the writer gives the guard a prefix a hand park never
writes, which is strictly worse than today — a guard that reads an empty set
refuses nothing and says so to nobody. That shape was weighed and declined
once, on the ground that parking runs once a hold (ranger-base-zlvyk), and
ranger-base-jv2q3 recorded writing the convention down as the answer instead.
Re-open it with a verb, or not at all.
