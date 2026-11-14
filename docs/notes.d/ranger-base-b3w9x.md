# A method, and then a helper: two places the exec census was not asking (ranger-base-b3w9x)

2026-11-14. `internal/treepins/treewidedoor_qa_test.go`.

`TestQAEveryTrackedProgramATreePinRunsIsDispositioned` holds every tracked
program a pin in `internal/treepins` hands to `exec.Command` to a row saying
what that program reads — because a program's reading is outside Go, and the
register's default is INCLUSION: an enumerating program makes its callers
tree-wide pins, which need a Makefile door. An undispositioned program is a
pin whose real reach may be a tracked directory, with no door and nothing
saying so.

It was asking in two places the answer is not written.

**1. Declarations with a receiver.** `twdParseDir` skipped them:

    if !ok || fn.Recv != nil || fn.Body == nil { continue }

so `execTree` was never computed for a method body, and a rig that execs from
a method — the natural shape — was outside the census. MEASURED in
ranger-base-41l0r: a pin execing `scripts/verify-gate-freshness.sh` from a
method left the two-way arm reporting the row DEAD ("which no test hands to
exec.Command any more") while the undispositioned arm said nothing at all.
Moving the call to a plain function made the dead arm see it. That bead took
the function shape by hand, for its own file only.

**2. The filter down to tests.** Even with the body read, the census then did
`if !f.test { continue }` — and a rig execs through a helper (`gamRun(t, r,
flags...)`, `(*hfRig).run`), so the `exec.Command` is almost never in a test's
own body. The first run of the widened census found one, sitting there since
the pin was written:

    internal/treepins/bdargvgate_qa_test.go:708 parserGate runs scripts/bd-argv-gate.py

`scripts/bd-argv-gate.py` is dispositioned now, and `scripts/bd-argv-gate.sh`'s
existing row was corrected to agree with it: the wrapper delegates to the
parser (`py=${BD_ARGV_GATE_PY:-"$self_dir/bd-argv-gate.py"}`), so the two have
one reading between them, and that reading is not "no directory at all" — it
is the one directory the stdin payload's own `cwd` field names, through
`git status --porcelain` in the close arm (ADR 0041 §3). Every pin in the
package hands that field a fixture tree under `t.TempDir()` or omits it, so no
tracked path is reached; the row says which, so the next reader does not have
to re-derive it.

## What landed

- `twdDecls` is the inclusion rule, one function, called by `twdParseDir` and
  by all three fixture drivers. The skip is part of the rule, so a fixture
  that applied its own would pass over exactly the declarations the tree's
  parse was dropping.
- `twdDeclName` keys a method by its receiver and name — `(*hfRig).run`.
  `run` is a method name several rigs in this package spell, and a bare key
  would let one rig's reading stand for another's and drop every reading but
  the last.
- The census reads every declaration, not only the tests. Over-inclusion is
  the register's standing default: the cost is one row saying what a program
  reads, against a pin whose subject is a tracked directory and whose door
  nobody ever asked for.

## What a method still cannot do, and the fence that says so

A method is a LEAF of the reach closure. `r.run(t)` is a selector on a local
whose type this file does not compute, so the closure — which follows
`*ast.Ident` calls — cannot carry a reading from a method back to the test
that calls it. The exec census does not need that edge, because it asks every
declaration. The enumeration rules do: a method that `WalkDir`s a tracked
directory is read, and its caller is still underived.

That residual is fenced rather than written down. An arm of the same pin
refuses a method with a non-empty `relEnum` and names the fix in its message
(move the reading into a plain function). It is green today — no method in
`internal/treepins` enumerates a tracked directory — which is a fact worth
having asserted rather than assumed.

## Floors

- `MEASURED 2026-11-14`: 53 method declarations in `internal/treepins/*_test.go`,
  206 in `internal/posse/*_test.go`, both read by the same parse. The pin
  fatals under 20 — a fixture cannot hold this half, because it says nothing
  about whether the TREE's parse found a method to apply the rule to, and a
  parse that went back to skipping receivers would make the census vacuous
  while every fixture stayed green. MEASURED by reverting the skip: the floor
  reds with `the parse names 0 declarations with a receiver`.
- Three fixture rows, driven through `twdDecls`/`twdAnalyze`/`twdDeclName`:
  a method's exec is read under its receiver's name, a plain helper's is read
  as it always was, and two rigs' same-named methods do not collide.

## Verified

`go vet ./internal/treepins`, `make fmt-check`, `make tree-check`, and the
register's own arms plus the gate-arm pins green under a `-run` filter
(20.7s). Full `make test` green.
