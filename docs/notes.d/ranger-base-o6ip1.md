# Two commit routes past the hook-skip wall: one refused, one disclosed (ranger-base-o6ip1)

Fix for hoover's finding **ranger-base-2lls3**. Two all-porcelain routes put
new content on a branch with the managed `pre-commit` and `commit-msg` hooks
never run, rc 0, nothing refused — and neither was named in
`internal/posse/hookskip.go`'s RESIDUALS, in the parity row, or in the GH#15
Limits paragraph the operator had just posted. One is refused now. The other
cannot be, so it is disclosed, in one wording that a pin executes.

## 1. `git am` is refused (`hookskip:plumbing-commit`)

`git am <patch>` is the stronger of the two: the input is any text file, so no
prior unscanned commit object is needed, and the content never meets
`pre-commit` or `commit-msg`. git runs only `pre-applypatch`. `am` was already
in `hookSkipVerbs` and `hookSkipNoVerifyVerbs`, so `am -n` was refused and the
patch route was not — the hole was the verb, not a flag.

It is refused for the same reason `commit-tree` is (ranger-base-crlty): a
commit no hook saw is the whole point of the verb, and the census found no
shipped use. **Re-censused at this tree**, `git grep -nE '\bgit (-[^ ]+ )*am\b'`:
every hit is `am --abort`/`--quit` prose (AGENTS.md:110, INSTALL.md:1615,
gates.go:1629) or a line in `scripts/silent-reverts.allow` describing a past
commit. No script, Makefile or doc applies a patch with `am`.

**What it costs, named rather than discovered later:** one real past use. That
`silent-reverts.allow` entry records jian-yang applying a patch with `git am`
under ranger-base-vf3pf. The respelling is `git apply <patch>` then
`git commit -- <paths>`, which meets both hooks and is one more line. That is
the trade the refusal makes.

Four forms still run, because AGENTS.md's sequencer paragraph and gates.go's
`sequencerVerbs` comment both tell seats to type them to clear a stalled tree:
`--abort`, `--quit`, `--skip`, `--show-current-patch` (with `=diff`/`=raw`).
`--continue` is NOT among them — continuing an am is what finally writes its
commit.

EXACT spellings, not prefixes (`hookSkipAmAllowed`). The arm refuses `am` by
default, so an unlisted abbreviation like `--abor`, which git itself accepts,
is an over-refusal one respelling away — the accepted cost in every arm of
this file. A prefix glob would fail the other way: `--s*` reaches `--signoff`
and `--scissors`, both of which commit. The scan also stops at `--`, so a
mailbox file named `--abort` buys no pass.

A second `am)` case arm would have been dead code — sh takes the first
matching arm, and the no-verify loop already writes one — so the refusal is
spliced INTO that arm through `hookSkipVerbExtra`, after the no-verify check,
which keeps the more specific reason for `am -n`.

## 2. The sequencer and stash routes are disclosed, not refused

`git stash push` then `git cherry-pick -m 1 stash@{0}` lands uncommitted
working-tree content through two porcelain verbs and no plumbing at all. A
refusal cannot hold it: cherry-picking an existing commit is the ordinary way
to move one, the sequencer runs no `pre-commit` by git's DESIGN, and the ref
write is `update-ref`'s ordinary job.

The old wording was "a commit OBJECT written by plumbing other than
commit-tree (`hash-object -t commit -w`, `fast-import`)". It was narrow twice
over: **plumbing**, when the two worst routes are porcelain, and **object**,
when the sequencer runs no hook whatever the object's provenance. The
replacement, in `HookSkipCommitResidual` (hookskip.go) and the RESIDUALS
paragraph:

> a commit made by anything but `git commit` or `git merge`:
> cherry-pick/revert/rebase of a stash or other unscanned object, `git stash
> create`, `git replace --graft`, and plumbing (`hash-object -t commit -w`,
> `fast-import`), each put on a ref

`HookSkipCommitResidual` is a const and the parity row's `Detail` concatenates
it, so there is ONE copy of that sentence in code rather than two that drift
(the sentence had three copies before this bead, and two of them said
different things). `docs/notes.d/ranger-base-crlty.md` is amended in place for
the same reason.

## Pins

`TestQAHookSkipRefusedOnEveryRuntime` (claude and codex) carries nine new
refused `am` legs and six allowed ones. The refused legs are the spellings the
finding named: plain, `-C .`, a non-hook `-c`, `--config-env`, an alias that
expands to `am` (`pam`, planted past the shim), a flag before the mailbox
(`--3way`), a mailbox after `--`, bare `am`, and `--continue`.

`hsSetup` now builds **one mailbox per leg** (`hsAmPatchCount = 7`), each a
`format-patch` of a commit that adds a file of its own, built past the shim
and rolled back. That is what makes the mutation check real: with one shared
mailbox only the first leg could land and the rest would fail to apply.

Two new pins:

- `TestQAHookSkipAmAbortStillEndsAStalledAm` — the control the whole refusal
  turns on. The `hsAllowed` legs can only show that with no am in progress the
  refusal is GIT's (`fatal: Resolve operation not in progress`, rc 128). This
  one stalls a real am over a conflicting commit, then types `git am --abort`
  through the shim and requires rc 0 and `.git/rebase-apply` gone.
- `TestQAHookSkipCommitResidualRoutesAreMeasured` — the measured-residual row.
  Every other pin in the file asserts a refusal; this one asserts the
  opposite, which is the point: a disclosure nothing executes drifts from the
  wall it describes. Each route must exit 0, not be refused, leave the managed
  `pre-commit` unrun, and put a new commit object on a ref — AND be named in
  `HookSkipCommitResidual`, so the prose and the measurement cannot disagree.

## MEASURED

2026-11-11, Linux, git 2.56.0, `/usr/bin/git` in a scratch repo with the
managed `pre-commit` in global scope (the ADR 0052 shape), then re-measured
through a rendered shim in the pins above.

| leg | rc | hooks that ran | landed |
|---|---|---|---|
| control: `git commit -m x -- f` | 0 | pre-commit (+ the rest) | yes |
| `git am <patch>`, clean tree | 0 | pre-applypatch ONLY | yes |
| `git am <patch>`, tree dirty in another file | 0 | pre-applypatch ONLY | yes |
| `git stash push` + `git cherry-pick -m 1 stash@{0}` | 0 | none | yes, HEAD moved |
| `git stash create` + `git update-ref` | 0 | none | yes, ref is a commit |
| `git replace --graft HEAD` | 0 | none | yes, replace ref |
| `am --abort`/`--quit`/`--skip`/`--show-current-patch`, nothing in progress | 128 | none | no (git's own fatal) |
| stalled am, then `git am --abort` | 0 | none | tree restored, `rebase-apply` gone, the dirty `f` untouched |

**Mutation check**, with `hookSkipVerbExtra` emptied so the arm renders
without the am refusal: of the eleven `am` legs, the seven carrying a mailbox
each exited 0 and MOVED HEAD with the managed `pre-commit` unrun; `am -n` and
`am --no-verify` stayed refused under `hookskip:no-verify`, so the no-verify
arm keeps the more specific reason; bare `am` exited 0 having read the test's
empty stdin and applied nothing, and `am --continue` exited 128 with no am in
progress — those two are spelling pins, not landing witnesses, and the leg
table says so. Mutating `hookSkipAmAllowed` down to one entry reds both the
`--abort` control and the six allowed legs on both runtimes; striking a route
out of `HookSkipCommitResidual` reds the measured-residual pin by name.

## Verified

- `make tree-check` green, 53.4s warm at a one-minute load average of 0.95
  with both suite slots free. It red once on the way: `crew-check` caught a
  crew name in a new test comment (ADR 0012 App.A 5), which is the pin doing
  its job on a fragment of the finding's own prose.
- `make test` green, all three arms, 0 FAIL lines — internal/posse 167.1s,
  cmd/posse 107.5s, internal/treepins 148.4s (arm 1), 52.8s (arm 2), 89.3s
  (arm 3); silent-revert audit 2549 commits, 0 untriaged. Slots free and
  `go run ./cmd/checkorphans` clean afterwards.
- `make coldexec-check RUN='^(TestQAHookSkipAmAbortStillEndsAStalledAm|TestQAHookSkipCommitResidualRoutesAreMeasured|TestQAHookSkipRefusedOnEveryRuntime)$'`
  green, 23.7s over the five arms, 11 cold first execs held 2.5s each. No
  `--warm` pre-flight is needed: these pins plant gate shims and exec them,
  but they assert exit codes and refusal text, never a deadline.

## Not closed here

- **ranger-base-2lls3** stays open. The GH#15 Limits correction is the
  operator's to post — writing under the operator's name is crew line 2 — and
  hoover re-verifies from a seat afterwards. The draft is on that bead; the
  stale copy in `ranger-base-crlty.md` is corrected in place and labelled as
  not live.

Nothing is withheld in the paragraphs above: 2lls3 records the operator's
decision that this arm's residuals are PUBLIC, because the arm is published as
a cooperative wall (ADR 0025) and the whole fix for the sequencer/stash route
IS the disclosure. AGENTS.md's "a hole still open is named by bead id and
nothing else" scopes to a residual under a `vuln-private` bead, which this is
not.
