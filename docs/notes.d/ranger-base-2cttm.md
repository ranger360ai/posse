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
`WaitDelay` dropped, and after **~0.4s** with it set.

**CORRECTED 2026-10-31 (ranger-base-0npto).** This paragraph used to end "A
ONE-command body (`sleep 120` alone) is bounded either way, because `sh -c`
execs a lone command and the process the context kills IS the sleeper." That
does not reproduce, and it was never about this fixture — the fixture is a
script FILE, not `sh -c`, and darwin's /bin/sh forks the lone command rather
than exec'ing it, so that body leaks a grandchild on the pipes like any
other. RE-MEASURED on this box (darwin 25.6.0), a 200ms limit over a 25s
sleep, five runs each of five bodies — `sleep` alone, echo-then-sleep,
echo-then-background-sleep-then-sleep, background-sleep-then-wait,
echo-then-background-sleep-then-wait — **all 25 runs with `WaitDelay`
dropped waited the full 25s** (25.01-25.12s), and all 25 with it set
returned in 0.20-0.41s. Read it as: no body behind a SHELL is bounded by the
context alone. Whoever builds the next WaitDelay fixture from this note
should build it as `echo <line>; sleep N & wait`, which is what
`TestReadCLIVersionGivesUpOnACLIThatWillNotAnswer`'s arm 2 is now — the `&`
cannot be exec'd away by any shell, so the grandchild is there by
construction rather than by the shell's choice.

**AND NOT AT 200ms (ranger-base-metqp, from ranger-base-grj2i).** Every
reading above needs a box quiet enough to start a `/bin/sh` inside the
limit, which the boxes they were taken on were — a 25s wait is proof the
sleeper ran. A loaded one does not: MEASURED 2026-10-31 at loadavg 5.9/21.1,
with the sleeper touching a witness file as its FIRST act, a 200ms limit
started it 0 times in 6 and 800ms 3 times in 6, while 1600ms and 3s started
it 6/6. Below that floor the child is killed before it forks, so there is no
grandchild, nothing holds the pipes, and the fixture returns at the limit
whether `WaitDelay` is set or not — it measures the scheduler. That is
exactly how `TestReadCLIVersionGivesUpOnACLIThatWillNotAnswer` came to pass
10 runs of 10 over a tree with `cmd.WaitDelay = limit` DELETED. It runs at a
3s limit now and `t.Fatalf`s on the missing witness. So the next fixture
built from this note wants both: the `& wait` body above, AND a positive
control the script writes before it forks, under a limit above process
startup under load. The limit is the part that goes stale; the control is
what reds instead of passing when it does.

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
whole call on its own. The ruling rests on `ps` itself and on there being no
SHELL between posse and it — not on the fixture, whose "passing arm" the
correction above withdraws.

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

(SUPERSEDED by `ranger-base-tpy6q`, from the verify bead `ranger-base-rc63f`:
"inside the site's own function or in a function that encloses it" resolved
per NAME per FUNCTION, and three shapes read as satisfied with no grace at the
site — two children on one identifier, a site in a closure that shadows the
name, and a cmd handed to a binder in the parameter it never touches. The scan
resolves per SITE now: same identifier, the same innermost function exactly,
and after the site but before the next child on that identifier. No shipped
site's reading changed, the `FuncLit` one included.)

The walk spells `"cmd"` and `"internal"` as literals at the two `WalkDir`
calls rather than ranging over a slice. That is not style: the tree-wide
class is DERIVED from a `WalkDir` whose root is a literal relative path
(`treewidedoor_qa_test.go`, `twdFunc.relEnum`), so a root arriving as a
variable would be a pin whose membership nothing can see and whose door
nothing would demand.

(SUPERSEDED by `ranger-base-tpy6q`: the reason stands and the roots do not.
Two literal roots left a repository-root `.go` file outside a census whose
sentence is "every bounded child", so the walk is one literal `"."` now.)

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
