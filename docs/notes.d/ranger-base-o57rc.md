# Two pins that claimed more than they measured (ranger-base-o57rc)

Both findings come from ranger-base-tfqlr's verify of four closes. Neither was
a defect in shipped behaviour: in both cases the CODE was right and the pin
over it was green for a reason that had nothing to do with the decision the
code documents. Both were found by mutating the shipped source and showing the
mutant is a real behavioural difference, and both fixes are judged the same
way — the new arm must red under the mutant and be green under the shipped
reading, measured in both directions.

MEASURED 2026-11-11, this box (linux 7.2.15 x86_64, go1.27.3), from the
gwart-posse-ranger-base-o57rc worktree. Every mutated file was restored
byte-identical (md5 before/after) and `git status` was clean of it after each
arm.

## 1. The coldexec door's ctime choice now has a witness

`cmd/coldexec/door_linux.go`'s epoch test keys on the CTIME and says why at
the stat: a copied binary keeps its source's mtime, and a `chmod +x` moves
only the ctime, and both of those are plants. Nothing measured it. Mutating
`st.Ctim` to `st.Mtim` in that one comparison left all eleven pins in
`supervisor_linux_test.go` green — `ok 1.535s`, 0 red (tfqlr), confirmed here
— so a later "simplify ctime to mtime" lands green over a door that has lost
half of ADR 0085 D5's artifact class.

It is not an equivalent mutant. Per door, over the two spellings:

| door keyed on   | mtime-preserving copy | an old file that gains +x |
|-----------------|-----------------------|---------------------------|
| ctime (shipped) | delayed               | delayed                   |
| mtime (mutant)  | "old", NOT delayed    | "old", NOT delayed        |

`TestACopiedBinaryPaysD` is the pin whose subject is closest to this and it
cannot see it: it plants with a plain `cp`, which sets a FRESH mtime, so its
plant is cold under either reading.

**ARM 12**, `TestAPlantWhoseMtimePredatesTheRunStillPaysD`, carries both
halves of the comment in one run — `cp -p /bin/true preserved`, and a file
written, backdated with `touch -t 202001010000` and then `chmod +x` — and
asserts each pays D.

    shipped: ok 1.445s, 2 cold execs delayed, both halves cold
    mutant:  FAIL — "preserved took 1ms", "gained took 1ms", 10 execve
             notifications, 0 delayed, exit 3 ("the door delayed NOTHING")

`TestACopiedBinaryPaysD` stayed green on the same mutant run, which is the
pin gap stated as a measurement rather than as a claim.

`cp -p` of a root-owned source exits 0 on this box; the arm does not depend on
it, because the backdated half carries the same subject with no ownership step
and the script's exit is the last plant's.

## 2. The fixture-git one hop reads declarations, not just assignments

`internal/treepins/fixturegitpin_qa_test.go`'s one-hop `local` map resolved an
identifier through `*ast.AssignStmt` only. A `var` declaration inside the
exec'ing function is an `*ast.DeclStmt`, so it was invisible — for BOTH accept
tests, the string literal and `exec.LookPath("git")`.

The pin's own prose claimed that depth ("the assignments to that name in the
exec'ing function") and its NOT CLAIMED paragraph enumerated what was outside
it (a function parameter, a package-level var, a second identifier). An
in-function `var` was in neither list, so the enumeration read as complete and
was not. That is the defect: a pin wrong about what it measures, not a pin
that is narrow on purpose.

MEASURED at `internal/treepins/policypins_qa_test.go`'s `ppRepo` — a real
non-immune site, the one tfqlr used — one spelling per arm, file restored
byte-identical (md5 `5ed71897cc4b79ad5c358bc36d08a679`) between arms:

| spelling at the site                     | before      | after  |
|------------------------------------------|-------------|--------|
| `gitBin := gitOutsideGates(t)` (shipped) | ok          | ok     |
| `gitBin := "git"`                        | FAIL        | FAIL   |
| `var gitBin string; gitBin = "git"`      | FAIL        | FAIL   |
| `var gitBin = "git"`                     | **ok**      | FAIL   |
| `var gitBin, _ = exec.LookPath("git")`   | **ok**      | FAIL   |
| `const gitBin = "git"`                   | (untested)  | FAIL   |

The two `FAIL` rows that were already failing are the positive control: the
rig is demonstrably able to fail at this exact site, so the two `ok` rows were
a hole and not a misaimed probe. The control row is green after, so the fix
did not simply make the site red.

The walk now reads `*ast.DeclStmt` -> `*ast.GenDecl` (`var` or `const`) ->
`*ast.ValueSpec` into the same map, through one `bind` closure shared with the
assignment arm, so the two shapes the assignment arm already had — n names
from n values, and n names from one value, where only the first name is
recorded — are the shapes a declaration gets too, by construction rather than
by a second copy of the rule. `const` is read because it is the same node and
the same defect one token away. The accept tests are untouched: a ValueSpec's
value meets the same `fgpStringLit` and `fgpLookPathGit` the inline spelling
meets. This is the recognizer reaching a spelling its prose claims, NOT the
broader reading ranger-base-ffojl declined.

**No live site, before or after**: 778 test files, 68 bound bare-name git
children, 41 unpinning git's config scope, 41 excused by their package's
TestMain — identical to the counts before the change. Nothing was red and
nothing needed converting, the same posture ranger-base-ffojl was filed with.

## Filed, not fixed: ranger-base-6rr2e

The same class at a different node. The walk that collects the git CHILDREN of
a function (the `kids` map) also reads `*ast.AssignStmt` alone, so
`var cmd = exec.Command(gitBin, …)` is never recorded, `len(kids) == 0`, and
the function is skipped WHOLE — the binary half of the verdict never runs,
however well it now resolves. MEASURED at the same site with a *caught*
`var gitBin = "git"` beside it: `ok`. The positive control for that arm is the
row above it in the table, which fails with the identifier bound exactly that
way, so what the second arm isolates is the child walk and nothing else.

It is filed rather than fixed because it is a second deliverable with its own
census to run (this bead measured no corpus count for that shape) and because
the pin's NOT CLAIMED paragraph now names it either way. The `.Env` LHS walk
is NOT affected: `var cmd.Env = …` is not Go, so a `.Env` write is always an
assignment.
