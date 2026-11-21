# The ubuntu donor was never dispatching on argv[0], and two green mutants in the arm-time sweep (ranger-base-x78s5)

2026-11-21, Linux 7.2.15 x86_64, go1.27.3, 16 cores, from this bead's
worktree. The findings bundle for verify bead ranger-base-tvokd: four live
findings over four closes, three of them worked here and one (finding 2) on
its own bead, ranger-base-pdcf6.

Three of the four were a recipe that looked right and was not — including two
of my own. Each section below says what did not work as well as what did,
because the cost of this bead was spent there and the next reader should not
pay it twice.

## Finding 1 — the ubuntu half of ranger-base-fol4d (P1, devops)

`exec -a "$teename"` did not fix it. Runs **44903910882** (fol4d's own landing,
3363785c) and **44908392250** (one landing later, 5da1bc54), ubuntu-latest
arm 1, print the PRE-fix line verbatim for both finding-4 arms:

```
postmortem: it exited 1 (… or dispatched on argv[0]); donor /usr/bin/tee,
  run with argv[0] tee; the copy … is executable; the fifo is a fifo
its stderr: coreutils: unknown program 'claude'
```

**That reading rules out the donor fol4d's fix was written for.** An ELF
multi-call coreutils run under `exec -a tee` dispatches to tee and the fixture
lives; it cannot print `unknown program 'claude'`. What can is a donor the
kernel execs through a `#!` LINE — the shape a coreutils built
`--enable-single-binary-shebangs` installs each utility as — because a shebang
exec REBUILDS argv from the interpreter and the script path, discarding
`exec -a` entirely, and the multi-call interpreter then takes its program name
from the copy's own basename. The identical pre/post stderr is the evidence:
under that mechanism the fix is a no-op by construction.

MEASURED here, first local reproduction of that CI job. A faithful mutant of
the shape — a Go ELF that answers `coreutils: unknown program '<basename of
the script path>'` for anything but `tee`, reached through a `#!` stub named
`tee` first on PATH (both in this bead's scratchpad, `fakecoreutils.go` and
`stubbin/tee`):

| script | donor | reading |
|---|---|---|
| tracked, pre-fix (3363785c…bfc068fa) | the `#!` stub | **2 arms FAILED**, `exited 1`, `coreutils: unknown program 'claude'`, byte for byte the CI line |
| tracked, post-fix (this bead) | the `#!` stub first on PATH | all arms pass — it never looks at tee |

And the two facts the production counter reads, under a `#!` donor copied to
`.../claude` and exec'd `-a tee` (holden measured this first on the bead; this
is the re-reading):

```
/proc/<pid>/exe     -> /usr/bin/bash          (the INTERPRETER, so `*/claude` never matches)
/proc/<pid>/cmdline -> /usr/bin/bash .../claude -p   (exec -a discarded)
```

So **no argv[0] spelling can fix this** and the donor is the thing to change.
It is now `${BASH:-$(command -v bash)}` — the shell this script is already
running, which is the one candidate whose shape the script cannot be wrong
about. `bash -p` is privileged mode and a bash with no script operand reads
its commands from stdin, so the four facts the arms need are the four `tee -p`
gave (MEASURED, the two donors side by side):

```
donor /usr/bin/bash: args '-p'    ALIVE exe .../claude  cmdline [bash -p]  comm claude
donor /usr/bin/bash: args none    ALIVE exe .../claude  cmdline [bash]     comm claude
donor /usr/bin/tee:  args '-p'    ALIVE exe .../claude  cmdline [tee -p]   comm claude
donor /usr/bin/tee:  args none    ALIVE exe .../claude  cmdline [tee]      comm claude
```

A `#!` donor is REFUSED and not worked around. The reachable case is the
fallback — a box where `$BASH` is empty and PATH's `bash` is a wrapper SCRIPT,
which some are — and falling back to another system utility is how this bug
was written in the first place: the donor would be chosen for its name again
and the next distro's build of it is the next nine days.

**The postmortem prints observations now, not intentions.** `run with argv[0]
tee` was a reprint of the variable the rig INTENDED, which is why two CI
readings after the fix could not say whether the exec had taken. start_fake
reads `/proc/<pid>/exe` and `/proc/<pid>/cmdline` off the live process at +1s
and the postmortem prints them, with the donor's own first two bytes beside
them. On the passing path those read `exe=<the copy> cmdline=[bash -p ]` /
`[bash ]` (MEASURED through an instrumented copy). The cmdline is read with
`read -r -d ''` and not a `tr '\0' ' '` — selftestforkarm_qa_test.go red the
first draft for exactly that, and it is right to.

Four controls, all MEASURED here:

| control | mutant | reading |
|---|---|---|
| A | donor is a `#!` stub | `FAIL the donor … is a '#!' stub and not a binary`, 1 arm FAILED — refused, nothing planted |
| B | A, with the refusal neutered | both arms `FAIL the fixture … is live, but /proc/<pid>/exe reads /usr/bin/bash` — the vacuity guard naming the one state in which the `-p` arm would have passed over a process the counter cannot see |
| C | the `#!` stub `tee` first on PATH, tracked script | all arms pass |
| D | donor `/usr/bin/cat`, which rejects `-p` | the `-p` arm FAILs with `its stderr: cat: invalid option -- 'p'` and the new `as the kernel saw it at +1s` line; the no-arg control still passes |

`scripts/reap-selftest-ambient.sh` carried the same tee donor and no `exec -a`
at all, so on such a box every arrival it plants would be invisible to the
counter it is perturbing — a detector reading GREEN over the defect it exists
to find. Same donor derivation, same refusal. Re-read: `--runs 1`, 11 ambient
arrivals, 1 of 1 green.

Reading left for the next CI run: whether ubuntu-latest's `/usr/bin/tee` really
is a `#!` stub. That is ASSUMED for that runner and MEASURED for the mechanism
— and it no longer gates the fix, because the rig does not use tee on any box.

**SETTLED, and FALSIFIED, on ranger-base-x0ses** (2026-11-21, the first
ubuntu-latest arm-1 job carrying this fix: run 44923103475, job 136903195755,
both arms green and no postmortem). That runner's `/usr/bin/tee` is a
**symlink**, not a `#!` stub: ubuntu-latest is Ubuntu 26.04.1 LTS now, where
`coreutils` is an Architecture:all metapackage over
`coreutils-from-uutils | coreutils-from-gnu` and every utility on PATH is a
symlink into one multi-call binary. `exec -a` was a no-op because uutils'
dispatcher takes the utility from the kernel's exec path and ignores `argv[0]`
when the basenames disagree — deliberately, to stop that exact spoofing. The
conclusion above is unchanged and so is the fix; only the mechanism was wrong.
docs/notes.d/ranger-base-x0ses.md has every reading.

## Findings 3 and 4 — two unpinned narrowings in landedarm.go (nits, code)

`scheduleInheritedTips` derives its repos as `BeadsDirs()` → `dirFilter`
narrowing → `mainCheckoutsOf`, and landedarm.go calls the pair load-bearing in
one sentence. Deleting either was GREEN over all five arms of
landedarm_qa_test.go, because every arm drives `d.Watch(ctx, "", …)` and
configures one repo by its own root.

**Finding 3, the dirFilter.** Two configured repos, each shipping its own check
into its own log, and `Watch` driven with `--dir` naming one of them: the
filtered repo is judged and the other is not, in the same pass. Mutant
(`_ = dirFilter`) judges both and reds on the log this arm reads as empty.
MEASURED both ways.

**Finding 4, `mainCheckoutsOf`, and this took three shapes.** Two readings look
like they kill the bare `range dirs` mutant and do not:

1. *The bead's own recipe* — configure a SUBDIRECTORY of the repo alone, on the
   reading that the mutant's judge would find no `scripts/posse-landed-check`
   relative to it. It finds one: a judge handed `<repo>/docs` extracts the
   check from `<branch>:scripts/posse-landed-check`, which git resolves against
   the repository ROOT and not the cwd, then judges the same tip and records
   through `git config` in the same repo. One run over the right tip, every
   reading identical. **Mutant green.**
2. *The same repo configured twice*, by its root and by that subdirectory,
   asserting one run for the DEDUPE half of `mainCheckoutsOf`. D4's cache
   absorbs it: the second judge reads the record the first one just wrote — the
   same `git config`, both spellings — and returns. **Mutant green.**

What reds is a reading over a TREE. The configured dir is a linked worktree on
an ordinary branch (`sidecar`), moved off main by one commit; `MainCheckout`
resolves it to the repo and the judge runs over MAIN's tip, while the mutant
schedules the worktree at its own branch and judges a tree no landing lands
into:

```
the main checkout of a configured worktree (…/sidecar) was not judged at arm:
[{Version:v1 Tip:85021cac…}] (want one run over 5aa26bc81c11, main's tip)
```

`posse/…` is the prefix that would make that worktree a SESSION tree
(`SessionTreesIn`), so an ordinary branch name is what keeps this file's
premise intact — no session worktree, so no landing can happen in any of these
passes and every run in the log is the arm's. That was holden's reason for
preferring a subdirectory, and it survives the change of shape.

Both arms green at HEAD, red under their own mutant, three consecutive green
runs of all seven arms.

## Finding 2 — ranger-base-pdcf6, a fail-open in check 4 (P2, security lane)

A prefix test cannot find a file header in this stream: an added line whose own
text begins `++` ARRIVES as `+++`, and one beginning `+++ b/` arrives as
`++++ b/`. ranger-base-4og5l's comment named the collision and reasoned it away
as "read exactly as it was read before"; it was not — the grep it replaced
DROPPED such a line, leaving the lines around it adjacent so a continuation
still continued its id, where blanking CLOSES the unit.

The fix is hoover's, by POSITION: the pipeline keeps the `diff --git ` line as
well (no added line can begin with it, because every added line carries the
`+`), and the judge blanks that line plus a `+++ ` line IMMEDIATELY AFTER one.
Everything else, `+++` lines included, takes the one-`+` discount and reads as
content. In the filtered stream those two are adjacent for every file git
emits — `index`, `--- a/…` and `@@` are all dropped by the grep — and a file
with no `+++` line at all (a pure rename, a mode change, an empty new file) is
why the second blanking is conditional.

MEASURED through `RunGatesVulnPrivateScan`, content arm, one open id:

| row | reading |
|---|---|
| bare id, `++ a content line`, TAB + description | **HIT** (was clean: the fail-open) |
| bare id, `+++ b/bin/bob-watch`, TAB + description | **HIT** (was clean) |
| control: the same two lines, no `++` line | HIT |
| a real `diff --git` + `+++` boundary between them | clean — 4og5l's fix intact |
| a file whose header has no `+++` line, then a file with content | HIT — the conditional blanking does not eat the next file |
| the compliant bare id alone | clean |

The pre-fix column is hoover's and holden's, measured twice on the landed tree
(bfc068fa) and recorded on ranger-base-pdcf6.

Every fixture in that pin now renders through one helper, `vpFile`, which
writes what the pipeline actually feeds: `diff --git`, the `+++` header, then
`+`-marked added lines. Four readings across two pins wrote a bare
`+++ b/<path>` and were therefore writing CONTENT; each read as a hit under
the new rule — fail-closed, and now spelled the way the stream is.

**End to end, through real git.** `TestVulnPrivateContentArmReadsEveryStagedFileButTheBeadsJsonl`
already staged files into a real repo and ran the arm's diff; it ran a
hand-written `grep -a '^+'` beside the arm instead of the arm's own. The grep
is now built from `vulnPrivateDiffHeader` and the render is asserted to carry
that exact string, so the test cannot measure a pipeline the hook does not
have — and a staged `docs/y.md` holding the collision shape is judged through
`RunGatesVulnPrivateScan` at the end: real git output, the arm's grep, the
real judge, HIT.

The mutant control: restore the one-line prefix test
(`case strings.HasPrefix(line, "+++")`) and both collision rows and that
end-to-end arm go red, with the boundary rows still green. MEASURED here.

HOOK SKEW: an installed hook older than the binary feeds no `diff --git`
lines, so nothing is blanked and the grouping falls back to the pre-4og5l
reading — fail-closed for these rows and fail-open across file boundaries,
which is the state this tree was in before either fix. Hooks are re-rendered
by `posse gates install-hooks` and by a session create, both from the binary
that is running, so this window closes when the fix is installed.
