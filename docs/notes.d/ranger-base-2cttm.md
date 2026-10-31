# A context deadline is half a bound: the WaitDelay census, the ruling, and the pin (ranger-base-2cttm)

2026-10-31. ranger-base-fnn1o gave `readCLIVersionAt` a deadline and noticed,
while doing it, that the deadline is only half of one. This bead is the rest
of the tree: the census, a ruling on the sites that do not need it, and a pin
so the rule stops being prose.

## The mechanism

`os/exec`'s `Wait` does not return until the goroutines copying the child's
stdout and stderr are done. A GRANDCHILD that inherited those pipes keeps
them open after the child is killed, so a context deadline bounds the
CHILD's life and not the call. `cmd.WaitDelay` is what makes `Wait` give up
on the copy. `herdrWaitDelay`'s doc (internal/posse/herdr.go) has said so
since herdr's own hang bead.

MEASURED 2026-10-31, ranger-base-fnn1o's fixture, darwin 25.6.0, inside
internal/posse's test binary: a 200ms context over `#!/bin/sh` + `echo
<line>` + `sleep 120` returned from `cmd.Output()` after **120.16s** with
`WaitDelay` dropped, and after **~0.4s** with it set. A ONE-command body
(`sleep 120` alone) is bounded either way, because `sh -c` execs a lone
command and the process the context kills IS the sleeper.

Not observed in the wild. Nothing here is a filed hang; the whole bead is
read off the code path, the same way fnn1o was.

## The census

Fifteen `exec.CommandContext` sites in non-test Go under `cmd/` and
`internal/`, MEASURED 2026-10-31 over 156 non-test `.go` files. Five named a
grace; ten did not.

The bead's own list said eleven unset, and **two of those were already
covered**: `internal/posse/worktree.go:2966` and `:2967` (the `git log | git
patch-id` pipe pair) pass their children to `bindGitDeadline`, which sets
`cmd.WaitDelay = gitKillGrace` — the same helper the bead credited at
`githang.go:339`. The list was built by grepping for `WaitDelay` near a call
site, and a grace set through a helper is invisible to that. It is the reason
the pin derives its binders instead of listing them.

So: nine genuinely unset, five fixed, three ruled exempt, one correction.

### Fixed

| site | child | grace |
|---|---|---|
| `ciwatch.go` ×3 | `gh api`, `gh run list`, `gh run view --log-failed` | `ciKillGrace` = 5s |
| `generatedindex.go` | `python3 scripts/notes-index.py` | `generatorKillGrace` = 5s |
| `skillsdirtrust.go` | `grok inspect` | `grokInspectTimeout` = 5s |

Five seconds is `herdrWaitDelay`'s number, quoted rather than re-coined: it
is generous because reaching it at all means a descendant outlived the kill.

`grok inspect` is the one worth naming on its own. It sits in FRONT of a
launch, grok carries a self-updater, and "prints a line, then leaves a forked
child on its pipes" is precisely the fixture arm above. It now takes one
duration for the context and the WaitDelay both, `readCLIVersionWithin`'s
rule, which bounds the whole call at twice it.

`python3` shells nothing today. The grace is set anyway, because "this script
does not subprocess" is a property of a file in another language that no
reader of the Go can check.

### Ruled exempt: `ps`

The three `ps` sites — the load guard's CPU census and its argv read
(`loadguard.go`), and the kill arm's verify re-read (`loadguardkill.go`) —
need no WaitDelay, and that is a ruling rather than an omission. `ps` reads
the kernel's process table and exits. It forks nothing, so the process the
context kills IS the only holder of the pipe, and the deadline bounds the
whole call on its own. This is the same shape as the fixture's passing arm:
one command, no grandchild, bounded either way.

All three are also CHAINED — `exec.CommandContext(...).Output()` — so they
have no variable to set a WaitDelay on. That is fine for the same reason, and
the ruling is what licenses the spelling.

## The pin

`internal/treepins/execwaitdelay_qa_test.go`,
`TestQAEveryBoundedChildNamesItsWaitDelay`, doored at `make corpus-check`.

Prose did not hold this: the rule is written out in three doc comments and
was honoured at five of fifteen sites. Nothing compared the rule with the
code, so the next site was always one landing away.

Three decisions in it are worth reading:

1. **The rule is unconditional over `exec.CommandContext`**, which is
   narrower than the hazard on purpose. The hazard is capture — `Wait` blocks
   on copy goroutines, which `os/exec` starts only for a Stdout/Stderr that
   is not an `*os.File` — but "does this site capture" is a second analysis,
   and the one that can be wrong while looking green. A WaitDelay on a site
   that captures nothing starts no goroutine and bounds nothing that was not
   already bounded, so the unconditional rule is free; a capture detector
   that missed one writer would not be.

2. **The exemption roster is keyed by BINARY, not by file and line.** The
   question WaitDelay answers is "can this child leave a descendant holding
   the pipes", which is a fact about the program being run. A per-line roster
   would go stale on every move of a call that did not change its answer, and
   would say nothing about the next site that runs the same binary. One row
   today: `ps`.

3. **The binders are derived, never listed.** A function qualifies when it
   takes an `*exec.Cmd` parameter and assigns `WaitDelay` on it —
   `bindGitDeadline` is the one, and its own doc says why it exists ("one
   choke point and no call site that can forget the clock"). A roster of
   binders here would be the second place that fact is written and the one
   that goes stale. It is also the hole the bead's hand census fell into.

Satisfaction is scoped to the site's enclosing FUNCTION, not its file:
`ciwatch.go` has three children all called `cmd`, so a file-wide match would
read one function's grace as another's. The scan accepts a satisfier inside
the site's own function or in a function that encloses it (the `grok inspect`
site lives in a `FuncLit` assigned to a package var).

The walk spells `"cmd"` and `"internal"` as literals at the two `WalkDir`
calls rather than ranging over a slice. That is not style: the tree-wide
class is DERIVED from a `WalkDir` whose root is a literal relative path
(`treewidedoor_qa_test.go`, `twdFunc.relEnum`), so a root arriving as a
variable would be a pin whose membership nothing can see and whose door
nothing would demand.

Its two-way control, `TestQATheWaitDelayCensusCanStillSayNo`, is deliberately
NOT doored. It reads planted fixtures rather than the tree, so nothing
derives it and no unrelated bead can red it — it reds only when the reader
itself changes, and whoever changes the reader is in that file.
`twdDoorHolders` is not the place for it: that list's own doc says a pin put
there to dodge the two-way check is one "whose membership nothing derived and
whose door nothing justified".

MEASURED at this bead's commit: 156 non-test `.go` files, 15 sites, 12 naming
a grace, 3 exempt, 0 bare, 1 derived binder. Negative control run against the
live tree — one `cmd.WaitDelay = ciKillGrace` removed from `ciwatch.go`, the
pin named `ciwatch.go:1033` and failed, the line restored.

## What is NOT here

No per-site behavioural arm. The three `gh` readers and the generator take
their deadline from a package const (30s, 10s, 10s, 2m), so an arm that
watched one expire would cost that wall, and the house answer to that — pass
the limit in, as `patchIDsVerbatimWithin` does — would be three new seams for
a mechanism already pinned twice. The behaviour lives at
`TestReadCLIVersionGivesUpOnACLIThatWillNotAnswer`
(internal/posse/runtimeprobe_test.go), which runs the fixture above through a
real child under a 200ms deadline and fails if the call outlives deadline
plus grace, and at `watchhang_qa_test.go`'s herdr arm. This bead's pin is the
census; those two are the proof the census is about something.
