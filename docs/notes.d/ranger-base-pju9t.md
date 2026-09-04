## The fix landed and the block came anyway: the launcher binary is not main (ranger-base-pju9t)

`docs/notes.d/ranger-base-avq12.md` closed with a cause and a prediction: the
merge-back block on `posse/gilfoyle-posse-ranger-base-nw9zg` re-files because
`noteMergeBlocked`'s dedupe read only OPEN beads, and closing a block with a
do-not-land verdict is what destroys its own dedupe. That cause was filed as
**ranger-base-j8qmj** and fixed.

This is the **fourth** filing of the same block, at 07:08 on 2026-09-04 —
**one hour and seventeen minutes after the fix landed on `main`**. The
prediction was right and the fix was right, and neither is what this instance
is about.

**The launcher that files these beads is not running `main`.**

```
$ posse --version
posse 0.4.0+c592683 (herdr-native)          # built 2026-09-03 23:54
$ git rev-list --count c592683..main
34
```

Two commits in that gap of 34 each independently answer this branch:

| commit | time | what it does here |
|---|---|---|
| `67effd0` (ranger-base-emgdb) | 04:28 | `equivalentOnBase` gains the REPLAY arm — author, author date, subject — so a commit a hand-resolved rebase landed is accounted for |
| `c3ab918` (ranger-base-j8qmj) | 05:51 | the dedupe reads CLOSED beads, so a standing do-not-land verdict is not re-asked |

Measured against the live repo and the live store, in-package on `main`:

```
equivalentOnBase: 5 pairings for 5 commits          <- not a strand at all
  df434d8 as 55d0e8e (same author, author date and subject)
  51ca0a7 as an equivalent patch on main
  54ed42b as 4ddf15d (same author, author date and subject)
  c663a9a as an equivalent patch on main
  34a27b4 as 6a230eb (same author, author date and subject)
workHeadTime:     2026-09-02T16:49:05-04:00
prior:            ranger-base-emgdb closed 2026-09-04T04:27:36-04:00
VERDICT:          answered, branch has not moved -> NOT RE-FILED
```

Note the last pairing. 4ri4n's reading said `34a27b4` — the branch's shared
base, another bead's commit, never picked — "can never have either kind of
evidence", and concluded that **one superseded commit anywhere in
`base..tip` makes the branch permanently unaccountable**. emgdb's third arm
retires that conclusion: `main` re-landed it as `6a230eb` by rebase, and the
identity pairing sees it. The branch is not permanently unaccountable; it
just needed a launcher built after 04:28.

### What this class costs, and what actually stops it

A fix that lands on `main` changes nothing about the fleet until somebody
installs it. For the eight hours between 23:54 and 07:16 every pass ran the
23:54 launcher, so the seat-hours spent on emgdb and j8qmj bought nothing —
and this bead is a fourth seat spent re-deriving a verdict two landed commits
already held. The gap is not visible from `git log`; it takes one command:

```bash
git rev-list --count "$(posse --version | sed 's/.*+//; s/ .*//')"..main
```

Nonzero means the fleet is running behind its own repo, and every number in
that gap is a fix nobody is getting.

**It closed on its own at 07:16**, eight minutes into reading this bead:
`~/.local/bin/posse` was rebuilt and now reports `+9920e75`, `main`'s tip,
gap 0. The install did not come from this seat and its source was not
determined — a persona's `make install` and the operator's look identical
from here. So the ask filed for the operator (**ranger-base-tvorm**) was
already satisfied before anyone read it, and is closed saying so.

What did NOT close is the reason it took eight hours: nothing tells the fleet
its launcher is behind its own repo. The command above is one comparison and
one line, and no pass prints it. Filed as **ranger-base-z3hx6**.

### Verified end to end, with the binary that now runs the fleet

Not "the code on main would do the right thing" — the installed launcher,
asked the same question the block came from:

```
$ posse worktrees
  ~/.posse/worktrees/posse/gilfoyle-posse-ranger-base-nw9zg → main
    5 commit(s) not on main by sha, replayed onto main as
    df434d8a4c37 as 55d0e8e39b95 (same author, author date and subject);
    54ed42ba2ea4 as 4ddf15d92506 (same author, author date and subject);
    34a27b4d85f3 as 6a230ebcf234 (same author, author date and subject),
    which is an identity match and not a measurement of what the replay
    kept — compare before retiring
```

The line the four blocks were all filed from was `5 commit(s) not on main,
for ranger-base-nw9zg` — a bare strand. It now says where the work is and
declines to call the tree disposable, which is the as19 distinction doing its
job. Two other trees in that same listing (`dinesh-…-uzgkz`, and four more
under `recorded as landed`) print the same shape, so this was never one
branch's problem.

### The verdict on the branch itself is unchanged, for the fourth time

Re-run today, the reading avq12 records is byte-for-byte the same: sixteen
non-blank lines the branch holds that `main` does not, every one of them a
line `main` deliberately replaced — the `rhq`→`posse` rename, the `+++ b/`
header reader **ranger-base-y7i7k** replaced with `diffHeaderPath`, four
digit-shaped Slack fixtures re-spelled with `A`s, and a `Makefile` `.PHONY`
and `test:` line `main` widened with `verify-parallel`.

```bash
B=posse/gilfoyle-posse-ranger-base-nw9zg
for p in $(git diff --name-only "$(git merge-base main $B)..$B"); do
    comm -23 <(git show $B:$p | sort -u) <(git show main:$p | sort -u)
done
```

`bash`, not `sh`, and the fence says so on purpose: `avq12`'s copy is fenced
`sh`, and under macOS's `/bin/sh` the two `<(...)` are a syntax error. Run
this way it prints two lines of *stderr*, so a reader piping it to `wc -l`
gets a plausible small number that is not an answer at all. Sixteen is the
answer, re-measured today.

**Do not land it.** Merge-back is `--ff-only`, so the branch cannot regress
anything; the tree and branch are the operator's to retire. Everything else
about the reading is in `ranger-base-4ri4n.md` and `ranger-base-avq12.md`
and does not need repeating.

### The other thing this bead walked into

`main`'s test build was red the whole time this was being read:
`fakeBdDropClosed` declared twice in `internal/posse/herdr_test.go`, one
copy from `c3ab918` and one from `3075168`, each written on a branch that
could not see the other. Merge-back is ff-only and opens no PR, which
`ci.yml`'s own header already names as the reason nothing is gated before it
lands — so the first thing to build the pair was `main`. Fixed in `510cc16`
by naming them apart. CI has been failing on **every** push well before that,
on unrelated tests; filed as **ranger-base-90y3c**.
