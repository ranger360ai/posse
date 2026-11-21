# ranger-base-6zw05 — the continue wall read one option, and `--skip` advances a sequencer too

The L1 continue wall (ranger-base-05aa1) is the realization of every crew
PID's `Bash(git commit unless --)` for the four git verbs that commit without
saying `commit`. It was keyed on the VERB, with one hand-spelled option:
`continueWallVerbs` carried a `Min` that was a prefix of `--continue` and
nothing else, and `TestQAContinueWallNamesEveryGitVerbThatCanContinue` derived
the verb set from git while NOTHING derived the option set. `--skip` appeared
nowhere in `internal/posse/continuewall_qa_test.go`.

`--skip` advances an operation exactly as `--continue` does. So
`git cherry-pick --skip`, `git revert --skip` and `git rebase --skip` went
through the wall unrefused, in a shared checkout, at every seat where it was
armed — which by 2026-11-21 was all of them (the installed launcher is posse
0.5.3+00601dfe, and `9b23517d` is an ancestor of it; laurie reproduced it
through this box's rendered shim, from the verify at ranger-base-suozl).
The defect was the KEY, not the entry.

## MEASURED 2026-11-21, this box

git 2.56.0, Linux 7.2.15-1-omarchy, worktree
`posse/dinesh-posse-ranger-base-6zw05`. One fresh `mktemp -d` repo per row.
Each verb was conflicted, the conflict resolved and staged, ANOTHER actor's
unrelated `other.txt` staged into the same index during the conflict window,
and — for the `--skip` rows — a SECOND operation left in the sequence, so the
skip has something to replay. That tail is the whole measurement: a skip with
nothing left only ends the sequence.

| typed | commit made | the other actor's staged file |
|---|---|---|
| `git merge --continue` | YES | swept into the commit |
| `git cherry-pick --continue` | YES | swept into the commit |
| `git revert --continue` | YES | swept into the commit |
| `git rebase --continue` | YES | swept into the commit |
| `git cherry-pick --skip` | YES | **destroyed, index and disk** |
| `git revert --skip` | YES | **destroyed, index and disk** |
| `git rebase --skip` | YES | **destroyed, index and disk** |
| `git merge --skip` | — | merge has no `--skip` (`unknown option`) |
| `git <verb> --abort` | no | **destroyed, index and disk** |
| `git <verb> --quit` | no | survived, staged, untouched on disk |
| `git rebase --edit-todo` | no | survived, staged, untouched on disk |
| `git rebase --show-current-patch` | no | survived, staged, untouched on disk |

The probes were shell, one fresh repo per row, and they are gone with the
session's scratchpad. The durable copy of the shape they measured is
`cwConflict` in `internal/posse/continuewall_qa_test.go`, whose `tail`
parameter is exactly the "second operation left in the sequence" above: the
premise half of the shared-checkout pin re-measures the seven walled rows of
the first table on every suite run, unguarded, and fails loudly if git ever
stops doing this. The stand-down half re-measures the "commit made" column of
the other rows and not their destruction column — that column is
ranger-base-hkp3s's subject, and this fragment is where the reading for it
lives.

`--skip` is a `reset --hard` followed by the rest of the sequence, so it
cannot sweep that file INTO its commit the way `--continue` does: it deletes
it first and then commits what is left to replay. Both halves are real, and
only one of them is this wall's business. The COMMIT is — it is a commit the
PID's `git commit` rule never sees, which is the rule this arm realizes. The
DESTRUCTION is not, and `--abort` shares it: see the residual below.

### The option set, asked of git

The fix shape the bead offered and this took: the table grows an option set
per verb, derived from git the way the verb set is. The derivation is
behavioural, not prose — for each long option `git <verb> -h` lists, run
`git <verb> <opt>` in a fresh repo where NOTHING is in progress, and read the
answer:

| what git answered | class | examples |
|---|---|---|
| `no <op> in progress` / `There is no merge to abort (MERGE_HEAD missing).` | in-progress-only | `--continue`, `--skip`, `--abort`, `--edit-todo`, `--show-current-patch`, `rebase --quit` |
| nothing at all, exit 0 | in-progress-only | `merge --quit`, `cherry-pick --quit`, `revert --quit` |
| the usage line, `requires a value`, `No remote for the current branch`, `There is no tracking information`, or the verb's ordinary work | ordinary | the other 87 |

The four verbs list 104 long options between them (30 `merge`, 19
`cherry-pick`, 15 `revert`, 40 `rebase`, counted the way the pin parses
`-h`), and 17 are in the class: 3 for `merge`, 4 each for `cherry-pick` and
`revert`, 6 for `rebase`.

The silent arm is not decoration: three verbs' `--quit` is a no-op outside an
operation and says so by saying nothing, so a message matcher alone would
have missed them. And `merge --abort` is why the matcher is not a search for
"in progress": git says `no merge to abort` there.

That class must be exactly `Advance ∪ Stood` for each verb —
`Advance` the options the wall refuses, `Stood` the rest, each with the
MEASURED reason it commits nothing, which is the only thing that excuses an
option from a wall whose subject is an unwalled commit. An option git grows
lands in neither and reds the pin on the day it lands; one that leaves reds
too, so its stated reason goes with it. A rewording of git's messages can
only SHRINK the derived class, and it cannot do that quietly: the pin compares
for equality, so an option falling OUT of the class reds exactly as loudly as
one falling in.

### The abbreviation minima

Asked of the real git one prefix at a time, the way a spoiler's `LongMin` is
(ranger-base-90y3c):

| verb | `--continue` | `--skip` |
|---|---|---|
| `merge` | `--con` (`--co` is ambiguous with `--commit`) | — |
| `cherry-pick` | `--continue`, nothing shorter | `--skip`, nothing shorter |
| `revert` | `--continue`, nothing shorter | `--skip`, nothing shorter |
| `rebase` | `--con` (`--co` ambiguous with `--committer-date-is-author-date`) | `--sk` (`--s` ambiguous with `--strategy`) |

`cherry-pick` and `revert` abbreviate none of their sequencer options; every
shorter prefix answers with the usage line.

## What landed

- `internal/posse/gates.go`: `continueWallVerb` is keyed on the option as well
  as the verb — `Advance []continueWallOpt{Long, Min}` and
  `Stood []continueWallStood{Long, Why}` replace the single `Min`. The
  renderer emits one case arm per option per verb, over every prefix git
  resolves, and records the option git SPELLS in `posse_cw_opt` so the
  refusal's second clause names it (the first clause already echoes the typed
  line). The rule id is unchanged — `commitwall:sequencer-continue` — because
  it is the rule's id and not a description of one option, and `refusals.log`
  lines, AGENTS.md and ADR 0001 all cite it.
- `internal/posse/continuewall_qa_test.go`: the option-set derivation pin, and
  every other pin generalized from "the verb's `--continue`" to "each of the
  verb's walled options".
- `AGENTS.md` ("Landing the plane") and ADR 0001's Consequences bullet for the
  seeded rule, both of which said four verbs and `--continue`.

## The pins, and the mutants that found a hole in them

Five, in the order a reader wants them: the verb set is git's; so is each
verb's option set; each walled option's `Min` is git's own boundary; the wall
refuses in a shared checkout (with the premise re-measured unguarded, per
option, because the two harms differ) while the stood-down options are not
refused and make no commit; and it all stands down in a linked worktree.

Five mutants were run, each applied to `gates.go` by a script that restores
the file from a backup under `trap` (the scripts were a session scratchpad and
are gone; the mutations are the left column, and each is one line of the
table):

| mutant | red |
|---|---|
| `cherry-pick --skip` dropped from the table entirely — the pre-fix state | option-set pin, `cherry-pick` |
| `rebase --skip` moved to `Stood` with a false reason | shared-checkout pin, `rebase/--skip` |
| `rebase --skip` `Min` one keystroke too short (`--s`) | minima pin |
| `merge --abort` pulled into `Advance` | option-set, shared-checkout and linked-worktree pins |
| the table correct but the RENDERER emitting only each verb's first option | shared-checkout pin |

The second mutant **passed on its first run**, and that is worth keeping. The
stand-down half measured "makes no commit" in a tail-LESS conflict window,
where `rebase --skip` really does commit nothing — there is nothing left to
replay — so a `--skip` moved out of `Advance` to quiet the wall was invisible
to the pin written to catch exactly that. The window has to be as strong as
the one the Advance half measures in; it now uses the tail wherever the verb
has one, and the mutant reds.

## Residuals

1. **Destruction without a commit is past this wall by class, and `--abort`
   is in it.** `--abort` wipes another persona's staged entry and their on-disk
   edit exactly as `--skip` does, differing only in making no commit — and it
   is the way out AGENTS.md prescribes and this gate's own refusal names. It is
   one member of a class L1 does not wall anywhere: `reset --hard`,
   `checkout -f`, `restore`, `clean -fd`. Filed as **ranger-base-hkp3s**
   (architecture), with this table, because "wall it" would mean moving the
   prose and the wall together.
2. **The same hole, one verb over: `git am --skip`.** `am` is excused from
   `continueWallVerbs` because the hook-skip arm "refuses it outright" — and
   that arm refuses the VERB by default while passing four forms through
   (`hookSkipAmAllowed`: `--abort`, `--quit`, `--skip`,
   `--show-current-patch`, documented there as forms that "commit nothing").
   MEASURED 2026-11-21 through this box's installed shim, with two patches in
   the mbox and the first conflicting: `git am --skip` was not refused,
   exited 0, applied and COMMITTED the second patch, and reset the other
   actor's staged entry out of the index and off the disk. Filed as
   **ranger-base-t4qez** (code) rather than fixed here — it is another arm's
   table, and the comment on pin 1 says so where a reader of the excuse will
   be standing.
3. ranger-base-05aa1's four residuals are unchanged: an alias reaches the verb
   without spelling it; a CLEAN merge in a shared checkout still makes an
   unwalled commit it cannot sweep with; the shim is cooperative (ADR 0025);
   and the L3 hook's three exemptions still stand on their own terms.
4. The sequencer audit's invariant ("rc 0 and a blocker survives") was checked
   against the two stood-down options that exit 0 with a rebase still in
   progress, `--edit-todo` and `--show-current-patch`: on this box no blocker
   exists during a conflicted non-interactive rebase, so nothing alarms
   (MEASURED through this seat's live shim). The 2026-09-11 darwin reading in
   that arm's comment says `CHERRY_PICK_HEAD` survives there, so the two could
   alarm on a box where the lock is unavailable. Nothing is filed: it is that
   arm's own trade, and the wall's pins do not depend on it — the stand-down
   measurement runs the real git, not the shim.
