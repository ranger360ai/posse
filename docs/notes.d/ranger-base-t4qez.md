# ranger-base-t4qez — `git am --skip` was on a list named "commits nothing", and it commits

`hookSkipAmAllowed` (internal/posse/hookskip.go) is the hook-skip arm's
exception list: the arm refuses `git am` by DEFAULT — `am` commits a patch
FILE with only `pre-applypatch` run, never `pre-commit`, never `commit-msg`,
and not posse's own L3 `prepare-commit-msg` (ADR 0052, ranger-base-o6ip1) —
and lets four exact spellings through, documented as forms that each "ENDS or
READS an am already in progress and **commits nothing**".

The clause was true of three of them. `git am --skip` does not end an am: it
is a `reset --hard` of the patch it stopped on followed by the REST OF THE
MBOX, exactly as `cherry-pick --skip` is (ranger-base-6zw05's table), so it
commits whenever a patch is left to apply. The arm's own comment carried the
reasoning that should have caught it — "`--continue`/`--resolved` are
deliberately absent ... continuing an am is what finally writes its commit" —
and skipping a patch continues the am too.

Found by ranger-base-6zw05 (dinesh) as residual 2 and filed rather than fixed:
it is another arm's table.

## Why neither arm's pins saw it

1. **The hook-skip legs ask the wrong question in the wrong window.**
   `hsAllowed` asserts that each allowed form is NOT REFUSED; nothing asked
   whether an allowed form commits — the property the list is named for. And
   no widening of those legs could have caught it: with no am in progress git
   itself exits 128 on all four (`fatal: Resolve operation not in progress`),
   so in that window every form trivially commits nothing and the one that
   does commit is indistinguishable from the three that cannot.
2. **The continue wall excused `am` on the strength of that list.**
   `TestQAContinueWallNamesEveryGitVerbThatCanContinue` derives its verb set
   from git and excuses `am` by asserting only that `hookSkipVerbExtra["am"]`
   EXISTS. It does — it refuses the VERB, while passing the ADVANCING option
   through. So the wall whose whole subject is "an option that advances an
   operation commits" did not cover `am` at all, on the strength of an arm
   that let the advancing option by.

## MEASURED

### 2026-11-21, dinesh, this box's INSTALLED shim (on the bead)

posse 0.5.3+00601dfe, `<gates>/bin/git`, git 2.56.0, Linux 7.2.15-1-omarchy,
one fresh `mktemp -d` non-linked checkout, two patches from `format-patch`
with the first conflicting, another actor's unrelated `other.txt` staged into
the shared index during the conflict window:

| typed | refused | rc | commits | HEAD subject | `other.txt` after |
|---|---|---|---|---|---|
| `git am --skip` (shim) | NO | 0 | 2 → 3 | `patch-two-clean` | GONE from index AND disk |
| `git am --skip` (real) | — | 0 | 2 → 3 | `patch-two-clean` | GONE from index AND disk |

The destruction is not this arm's business and is not fixed here: `--abort`
does the same and is the route the refusal itself names. That class —
`reset --hard`, `checkout -f`, `restore`, `clean -fd`, `--abort` — is
ranger-base-hkp3s, decided there.

### 2026-11-21, gwart, the shim rendered from THIS tree

git 2.56.0, Linux 7.2.15-1-omarchy, worktree
`posse/gwart-posse-ranger-base-t4qez`, through
`TestQAHookSkipAmAllowedFormsCommitNothing`'s fixture (a rendered claude
gates dir, the managed `pre-commit` in global scope, an am of two mailboxes
stopped on a conflicting first with a clean second still queued):

| typed, in a window with a tail | refused | rc | HEAD | managed `pre-commit` |
|---|---|---|---|---|
| `git am --abort` | no | 0 | unmoved | unrun |
| `git am --quit` | no | 0 | unmoved | unrun |
| `git am --show-current-patch` | no | 0 | unmoved | unrun |
| `git am --skip`, before the fix | NO | 0 | **moved to the queued patch** | **unrun** |
| `git am --skip`, after the fix | yes, `hookskip:plumbing-commit` | 1 | unmoved | unrun |
| `git am --skip`, past the shim | — | 0 | moved to the queued patch | unrun |

The last row is the premise, kept as a live leg rather than as prose: if the
real git ever stops committing there, the fixture has lost its tail and every
row above it witnesses nothing.

## The fix: (a), drop the entry

The bead offered two shapes. (a) drop `--skip` from `hookSkipAmAllowed`;
(b) give `am` a row in `continueWallVerbs`. (a), and the reason is reach, not
effort: the continue wall stands down in a linked worktree, because the index
there is the seat's own — and `am`'s hole is not about a shared index. It is
ADR 0052's: a commit the employer's managed scanner never saw, which is just
as unscanned in a dispatched seat's own worktree. (b) would have walled the
one place where `am` is least dangerous and stood down everywhere else, and
cost two arms saying overlapping things about one verb.

Three edits, one line of behaviour:

- `internal/posse/hookskip.go` — `hookSkipAmAllowed` is
  `{"--abort", "--quit", "--show-current-patch"}`. The list is EXACT
  spellings, not prefixes, so the removal needed no prefix work. The arm's
  refusal text (`hookSkipVerbExtra["am"]`) and the file's rule table no
  longer name `--skip` as a form that still runs.
- `internal/posse/hookskip_qa_test.go` — `am --skip` moved from `hsAllowed`
  to `hsRefused` (`hookskip:plumbing-commit`), and
  `TestQAHookSkipAmAllowedFormsCommitNothing` added: for every form the list
  calls safe, in a window with a tail, rc 0 AND HEAD unmoved AND the managed
  hook unrun — plus the two `--skip` rows above. This is the pin the bead
  asked for, and the shape generalizes: a table that calls a form safe
  because it commits nothing must be measured where committing is possible.
- `internal/posse/continuewall_qa_test.go` — `am`'s excuse is two facts now,
  not one: the `hookSkipVerbExtra["am"]` entry exists, AND nothing on
  `hookSkipAmAllowed` advances an am (`--continue`, `--resolved`, `--skip`).
  A form added back there reds the wall's own pin instead of quietly
  reopening the hole, which is what the old single-fact assertion allowed.

### The cost, and who pays it

A seat mid-am with a patch it wants to skip. The respelling is `git am
--abort`, then `git apply` plus a path-limited `git commit` — which is what
this arm already prescribes for `am` itself — or the same am in a session
worktree. Nobody was told to type `am --skip`: every `am` hit in AGENTS.md,
INSTALL.md and the ADR 0052 lineage is `--abort`/`--quit` prose
(AGENTS.md:148, INSTALL.md:1704), so no prose needed respelling and no
house advice is now refused. `TestQAHookSkipAmAbortStillEndsAStalledAm` is
still the control that `--abort` works against a real stalled am.

## Mutation check

`hookSkipAmAllowed` with `"--skip"` put back, this tree, 2026-11-21:

- `TestQAHookSkipAmAllowedFormsCommitNothing` — red on three lines: HEAD
  moved under the allowed-forms loop, the shim leg exited 0 instead of
  refusing, and HEAD moved through the shim.
- `TestQAContinueWallNamesEveryGitVerbThatCanContinue` — red:
  "hookSkipAmAllowed passes --skip through, and that option ADVANCES an am".
- `TestQAHookSkipRefusedOnEveryRuntime` — red on the `codex` and `claude`
  arms: the `am --skip` leg's refusal is git's `fatal: Resolve operation not
  in progress` and not this arm's, and `refusals.log` is one line short
  (124 against 125).

Restored, all three green.

## Not closed here

- ranger-base-hkp3s — destruction without a commit, decided in that lane.
- ranger-base-6zw05's residual 2 is this bead and is closed by it. Its
  residuals 1, 3 and 4 are unchanged.
