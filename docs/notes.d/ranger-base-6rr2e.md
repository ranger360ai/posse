# The same class one node over: a git CHILD bound by a declaration (ranger-base-6rr2e)

Discovered from ranger-base-o57rc, which closed the other half of
ranger-base-tfqlr finding 2. That half was the BINARY identifier: the
fixture-git pin's one-hop `local` map read `*ast.AssignStmt` alone, so
`var gitBin = "git"` resolved to nothing where `gitBin := "git"` reds. It
reads `var`/`const` ValueSpecs now.

This is the same defect at the walk one node over — the one that collects the
git CHILDREN a function exec'd (the `kids` map). It read `*ast.AssignStmt`
alone too, so a child spelled

	var cmd = exec.Command(gitBin, "-C", repo, "commit", …)

entered no `kids` entry. That is worse than the binary miss, because
`len(kids) == 0` skips the function WHOLE: the `.Env` half is never looked at,
and the binary half never runs however well o57rc had just taught it to
resolve the name.

MEASURED 2026-11-11, this box (linux 7.2.15 x86_64, go1.27.3, git 2.56.0),
from the gwart-posse-ranger-base-6rr2e worktree. Every mutated file was
restored byte-identical (md5 before/after) and the pin's own file reads md5
`9f4410229c0658469e800eb86b774757` after both mutant arms below.

## The live-site population: 0, and now MEASURED rather than assumed

The bead carried the 0 as ASSUMED, NOT MEASURED, and said to run the census
before deciding. Run, over the same corpus the pin censuses (779 `_test.go`
files, the same walk and the same `exec.Command`/`exec.CommandContext`
recognizer):

| exec child bound by…              | sites |
|-----------------------------------|-------|
| an assignment                     |   362 |
| an in-function var/const DECL     |     0 |

So this closes before the class reaches it rather than naming a live site —
exactly what ranger-base-l0bep's binary hop did, and why the arm-1 floors did
not need moving. The one `var cmd = exec.CommandContext(ctx, "gh", "api")` a
grep finds in this tree is inside a raw-string fixture in
`internal/treepins/execwaitdelayholes_qa_test.go`, which is a different pin's
synthetic source and is not Go the walk parses. (That pin met the same class
and answered it the other way, fail-closed: its own CONTROL case reads "the
binding is invisible to the scan, so it reads as chained".)

Arm 1's live counts are unchanged across the fix, which is the same statement
read from the other end: 779 test files, 68 bound bare-name git children, 41
unpinning git's config scope, 41 of those excused by their package's TestMain
— identical before and after, `ok 0.201s` / `ok 0.474s`.

## Judged in both directions

Two accept tests were added for the declaration spelling of a binding, plus
the compliant form of the same spelling as the boundary. Each mutant is the
shipped reading of ONE walk restored to `*ast.AssignStmt` alone
(`gd.Tok != token.VAR` → `token.IMPORT`), with the other walk left fixed:

| arm                                   | the child walk | the `local` map | result |
|---------------------------------------|----------------|-----------------|--------|
| shipped                               | reads decls    | reads decls     | ok 0.474s, 23 cases green |
| mutant 1 (this bead's walk reverted)  | AssignStmt     | reads decls     | FAIL — 2 red |
| mutant 2 (o57rc's arm reverted)       | reads decls    | AssignStmt      | FAIL — 1 red |

Mutant 1 reds `a git child bound by an in-function `var` declaration` (the
child walk isolated: the binary is the inline literal, so nothing else can
account for it) and `both the binary and the child bound by `var`
declarations`. Mutant 2 reds only the second of those — which is the point of
having it: ranger-base-o57rc measured its arm at a live site and left no
accept test, so that case is the only guard over it, and the table is the
evidence that it is one.

The third case, `the fix spelled as declarations is still compliant`, is
green under every arm. It asserts an absence, so it cannot red under a
mutant that narrows the reading; its job is the other direction — the day a
converted site is written `var gitBin = gitOutsideGates(t)`, a widening that
cannot say no here would report it as a violation.

## What was not done

`const` is NOT read at the child node, where the `local` map reads it. A
constant expression cannot hold an `*exec.Cmd`, so `const cmd =
exec.Command(…)` does not compile and the arm would be dead code wearing the
look of symmetry. At the binary node `const gitBin = "git"` compiles and is
the defect, which is why that map has it.

The child walk stays PAIRWISE, where the `local` map reads n-from-1. The third
spelling of a BINARY is `gitBin, err := exec.LookPath("git")`
(ranger-base-ffojl), and no `exec.Command` call binds a child and an error;
taking `Lhs[0]` of an arbitrary n-from-1 call at this node would record a
child the function never exec'd.

The third walk — the `.Env` LHS — needs no declaration arm at all: `var
cmd.Env = …` is not Go, so a write to a field is always an assignment. The
pin's NOT CLAIMED paragraph says so now, so the next reader at this file does
not re-derive it.
