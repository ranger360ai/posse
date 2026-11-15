# ranger-base-05aa1 — four git verbs commit without saying `commit`, and the L1 wall now reads them

`Bash(git commit unless --)` is seeded into every PID by ADR 0001, on the
rationale that "L3 is a repo hook that exists only where something installed
it, while a typed-line deny reaches every repo and runtime". It matches the
verb `commit`. Four git verbs write a commit without the word `commit`
anywhere in their argv, take the whole index, and cannot be path-limited at
all — git answers `fatal: cannot do a partial commit during a merge` — so for
those four there was no qualifier to demand and no refusal if you skipped it.

holden found it while verifying ranger-base-4d15g and left the first thing to
settle as a question: **does the L3 `prepare-commit-msg` gate backstop these
four?** It does not. It backstops one.

## MEASURED 2026-11-15, this box

git 2.56.0, Linux 7.2.15-1-omarchy, worktree
`posse/jian-yang-posse-ranger-base-05aa1`. Each verb was conflicted in a
scratch repo, the conflict resolved and staged, and then ANOTHER actor's
unrelated file (`other.txt`, touched by nothing the operation brings) staged
into the same index during the conflict window, before `--continue`:

| typed | no hook | `posse gates install-hooks` |
|---|---|---|
| `git merge --continue` | exit 0, **swept other.txt** | exit 0, **swept other.txt** |
| `git cherry-pick --continue` | exit 0, **swept other.txt** | exit 0, **swept other.txt** |
| `git revert --continue` | exit 0, **swept other.txt** | exit 1, refused |
| `git rebase --continue` | exit 0, **swept other.txt** | exit 0, **swept other.txt** |
| `git commit -m <msg>` | refused at L1 | refused at L1 |

The three the hook waves through are not an oversight there: `sharedIndexBody`
exempts `MERGE_HEAD`, `CHERRY_PICK_HEAD` and `rebase-merge`/`rebase-apply` on
their own stated merits — "is there a safe form to point at?", and git refuses
a pathspec mid-merge and mid-cherry-pick, while a rebase has commits left to
replay — and those exemptions cover `--continue` exactly as they cover a typed
`git commit`. `REVERT_HEAD` is refused there because a pathspec DOES work
mid-revert, which is also why AGENTS.md prescribes the two-step revert.

So the gap is not L1-only, and it is not L3's to close either. It is the L1
half's, which is what ADR 0001 seeded the deny for in the first place:
ranger-core has no non-sample hooks installed, so L1 was the whole wall there.

### The hole is the conflict window and nothing wider

The CLEAN forms need no wall. MEASURED the same day, same box, with
`other.txt` staged before the verb was typed:

| typed | result |
|---|---|
| `git merge --no-ff <branch>` | exit 2, `error: Your local changes to the following files would be overwritten by merge: other.txt` |
| `git cherry-pick <sha>` | exit 128, `error: your local changes would be overwritten by cherry-pick.` |
| `git revert <sha>` | exit 128, `error: your local changes would be overwritten by revert.` |

git refuses to START any of them over a dirty index, so there is no window for
them to sweep. What remains is a clean verb racing a `git add` between git's
own index check and its commit — bounded by milliseconds, unwalled, and named
as residual 2 below rather than closed.

### The abbreviation boundary

MEASURED one prefix at a time against the real git, the way a spoiler's
`LongMin` is (ranger-base-90y3c). git 2.56.0:

| verb | shortest prefix that resolves | what the one below says |
|---|---|---|
| `merge` | `--con` | `ambiguous option: co (could be --commit or --continue)` |
| `rebase` | `--con` | `ambiguous option: co (could be --committer-date-is-author-date or --continue)` |
| `cherry-pick` | `--continue` | the usage line — its sequencer options are not abbreviated at all |
| `revert` | `--continue` | the usage line, same |

## What landed

`renderContinueWall` in `internal/posse/gates.go`, an arm of the git shim
between the hook-skip env call and the sequencer audit — in front of the
audit on purpose, since that one runs these same verbs as a child and a
command this arm refuses must not reach git for its own effect.

It refuses `<verb> <any resolving prefix of --continue>` for the four verbs,
**only where the index is shared**: `rev-parse --path-format=absolute
--git-dir` against `--git-common-dir`, the same discriminator the L3 hook
uses. A linked worktree stands down, and that is load-bearing rather than
polite — ranger-base-4d15g's own merge-back ran in `~/src/ranger-core-piqdz`,
which is a linked worktree of `~/src/ranger-core` (verified), so a wall that
refused there would be the defect instead of the fix.

**It realizes prose rather than inventing policy.** In the shared checkout all
four are already out of bounds for a persona: AGENTS.md "Landing the plane"
says the launcher merges, says never `rebase` there, and prescribes the
two-step revert. The refusal names that prose as the way through, per verb,
and `--abort` is never refused.

It rides the PID's own negative `git commit` rule — the arm is that wall's
other half, not a second wall, so a PID without the commit deny is not given
one here by the back door.

### Why not gilfoyle's staged-set check

The bead's other shape was the hand-check gilfoyle wrote up: compare
`git diff --cached --name-only HEAD` against the merge-base diff and refuse
only a staged set that is wider. It is the right check for a human to run and
the wrong thing to put in a shim: "theirs" is spelled four different ways
(`MERGE_HEAD`, `CHERRY_PICK_HEAD^..CHERRY_PICK_HEAD`, `REVERT_HEAD`, and for a
rebase an internal `rebase-merge/stopped-sha`), so it is four mechanisms in sh
over git's private state, to decide a question the prose has already decided.
The shared/worktree split answers the same risk with one read git documents.

### Pins

`internal/posse/continuewall_qa_test.go`:

- `TestQAContinueWallNamesEveryGitVerbThatCanContinue` — membership is
  DERIVED, not listed: every verb in `git --list-cmds=main` whose `-h` usage
  mentions `--continue` must be in the table, or be `am`, which the hook-skip
  arm already refuses outright for a stronger reason (it commits with neither
  pre-commit nor commit-msg run — ADR 0052). git 2.56.0 reports exactly
  `am cherry-pick merge rebase revert`.
- `TestQAContinueWallMinimaAreGitsBoundary` — each `Min` is asked of the real
  git: every prefix from it up resolves, the one below does not.
- `TestQAContinueWallRefusesTheFourVerbsInASharedCheckout` — two halves, like
  `TestQACommitWallL1AbbreviationDoesNotSweepRealIndex`: the premise unguarded
  (each verb really does sweep) and the same window through the shim, with
  HEAD unmoved, the other persona's entry still staged, a `refusals.log` line,
  and the abbreviation and `git -C <repo>` spellings refused too.
- `TestQAContinueWallLeavesTheOrdinaryFormsAlone` — a clean merge is not this
  wall's business, and the commit wall's own two verdicts are unchanged.
- `TestQAContinueWallStandsDownInALinkedWorktree` — all four land there.
- `TestQAContinueWallRidesTheCommitDeny` — rendered with the commit deny and
  not without, and never for a command that is not git.

`TestQAPIDDenyAliasLookupIsTheOnlyExec` is amended: a continue-wall refusal
reaches git for two read-only `rev-parse` calls, the adjudication and never
the command's own effect, and a verb the wall does not match costs no read.

Both mutations were checked: dropping the arm reds all four shared-checkout
subtests, and making the refusal unconditional reds all four worktree
subtests.

## Residuals

1. An alias reaches the verb without spelling it (`alias.cp = cherry-pick`),
   the inherited hole this file's preamble and `renderSequencerAudit` both
   name for `push`.
2. A clean merge in a shared checkout still makes an unwalled commit. It
   cannot sweep — git would have refused to start — but a persona who stages
   between git's index check and its commit loses that race.
3. Cooperative, ADR 0025: an absolute `/usr/bin/git` or an emptied PATH is
   past this by class, as it is past every L1 shim.
4. The L3 hook's three exemptions are unchanged and still right on their own
   terms; what changed is that L1 now covers them in a shared checkout, hook
   or no hook.
