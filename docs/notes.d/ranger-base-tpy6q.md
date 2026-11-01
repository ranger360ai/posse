# The WaitDelay census resolved per name per function, and its corpus stopped at the package directories (ranger-base-tpy6q)

2026-10-31. The two LIVE findings of verify bead `ranger-base-rc63f`, which
verified `ranger-base-2cttm`'s census — `internal/treepins/execwaitdelay_qa_test.go`,
`TestQAEveryBoundedChildNamesItsWaitDelay`, doored at `make corpus-check`.
Neither was an unbounded child: both were the PIN being wrong about what it
measures, which is why they came here as `-l debt` and not as a P1.

## FINDING 1 — the resolver

The census read a site as satisfied when a WaitDelay satisfier with the SAME
IDENTIFIER sat anywhere inside the site's own function, or in a function that
encloses it. Three shapes passed that reading with no grace at the site:

- **(a) two children in one function on one identifier.** One assignment to
  `cmd` satisfied every site in the function bound to `cmd`, on either side
  of it. This is the shape an ordinary landing reaches — a second bounded
  child added to a function that already has one.
- **(b) a site in a closure that SHADOWS the name.** The outer assignment's
  scope encloses the closure, which is the clause that lets a wrapper's grace
  count, and a shadowed `cmd` is a different variable.
- **(c) a binder's other parameter.** `ewdBinders` keyed on the function
  NAME, so a call to a binder marked every identifier argument satisfied —
  `bindIt(nil, cmd)`, where `bindIt` assigns on its first parameter, bound
  `cmd`.

### The margin was one identifier wide, and MEASURED

`internal/posse/worktree.go:2966` and `:2967` are two `exec.CommandContext`
sites in ONE function — the `git log` and `git patch-id` halves of
`patchIDsVerbatimWithin` — censused correctly under the old rule only because
they carry DISTINCT identifiers (`logCmd`, `idCmd`) with a separate
`bindGitDeadline` call on each. MEASURED 2026-10-31 by renaming both to `cmd`
in the shipped tree (`cmd :=` then `cmd =`, which is what that rename
compiles to):

| rule | reading of the renamed tree |
|---|---|
| old, per name per function | GREEN — said nothing about either child |
| narrowed, per site | RED — `internal/posse/worktree.go:2966` |

It names the FIRST site because both `bindGitDeadline` calls sit below both
children: the child the first `cmd` held is overwritten before anything binds
it. Shape (a) was never waiting on a landing that merges two functions.

### The fix: per-site reaching

`ewdScan` resolves a satisfier to a site on three conditions now.

1. **The same identifier.** Unchanged.
2. **The same INNERMOST function, exactly** — not "a function that encloses
   the site", and not "anywhere inside the site's own function" either. A
   closure is a function, so this closes (b) and also refuses a grace set
   inside a closure the site sits outside of. Both directions fail CLOSED.
3. **After the site, and before the next child on that identifier.** A grace
   is written after the child it binds, so the next event on the name after
   the site has to be one; a `cmd = exec.…` below it is a SECOND child, and a
   grace past that point is the second child's. This closes (a).

Source order is the whole of the reaching analysis, which is why 3 fails
closed rather than open: a grace textually above its site — a loop body that
binds the child at the bottom — is reported although it has one. No shipped
site is that shape. Reporting a bound child costs a reader one look; excusing
an unbound one costs a wedged dispatch pass.

(c) is closed by deriving WHICH PARAMETER POSITIONS a binder assigns on,
rather than a bool, from the same body read. Positions are flat and count
unnamed parameters, because a call's argument list is flat. Methods are
skipped: a call to one is a `SelectorExpr` the scan never reads as a binder
call, so registering one could only ever satisfy a same-named plain
function's call. The derivation is still keyed by NAME across the corpus
rather than by package directory — reachable only by a non-binder sharing a
binder's name in another package, and the tree has one binder and no such
twin.

**No shipped site's reading changed.** Before and after, over the same tree:
15 `exec.CommandContext` sites, the same 12 naming a grace (11 by assignment,
1 through `bindGitDeadline`), the same 3 `ps` sites ruled exempt, 0 bare. The
`FuncLit` site `ranger-base-2cttm` cited as the reason for the `encloses`
clause does not need it: dropping the clause changes nothing in the tree.

### Mutation-checked both ways

`internal/treepins/execwaitdelayholes_qa_test.go` was the verify lane's
witness, shipped green asserting the hole, with the instruction to FLIP its
rows rather than delete them. That is what happened, and the rows are LINES
now rather than a count — a per-site rule is a claim about WHICH child is
unbound, and "some site here is unsatisfied" is satisfied by a reader that
reports the wrong one. 17 rows: the 3 flipped, 9 new controls, 5 kept.
MEASURED 2026-10-31, each of the seven ways to put the old reading back:

| mutation | reds |
|---|---|
| the ordering dropped | 3 rows |
| the next-child limit dropped | 1 row |
| scope relaxed to "or a function that encloses the site" | 1 row |
| scope relaxed to "anywhere inside the site's own function" | 1 row |
| the binder's parameter position ignored | 1 row |
| methods registered as binders | 1 row |
| the corpus narrowed back to a package directory | the census's root arm |

The closure-shadow HOLE row as it was measured reds under none of them: two
of the conditions close it, so the row that carries the scope half alone is a
new one — an enclosing grace BELOW a shadowed site, where ordering does not
refuse it. An over-determined row is worth keeping and worth labelling; it is
not worth mistaking for a witness.

## FINDING 2 — the corpus

`ewdGoFiles` walked `cmd/` and `internal/`, so a `.go` file at the repository
ROOT was outside a census whose sentence is "every bounded child". The roots
were spelled as literals for a reason that says nothing about WHICH roots
they are: the tree-wide class is derived from a `WalkDir` over a literal
relative path, so a root arriving as a variable would be a pin whose
membership nothing can see and whose door nothing would demand. One literal
`"."` keeps that and loses the gap. Dot-directories join `node_modules` and
`testdata` in the skip list.

The reading did not change — the tree has exactly one non-test `.go` at the
root (`embed.go`), it runs no child, and the corpus went 159 → 160 files with
the site list identical — and that is exactly why the fix needed an arm of
its own. Nothing else in the file reds when the walk narrows back, so
`TestQAEveryBoundedChildNamesItsWaitDelay` now reads the root directory a
second time (`os.ReadDir(".")`) and reports any non-test `.go` there that the
corpus does not hold. A second enumeration rather than a literal `embed.go`,
so the next root file is covered by it.

`ewdGoFiles` is shared with `TestQAEveryProtocolChildRunsOutsideTheGates`
(`protocolchildpath_qa_test.go`), which widened with it: 160 files, 46
`exec.Command`/`CommandContext` sites, 5 watched `.Env` assignments, 2
binders — same reading, one more file.

## Not here

The other two findings of `ranger-base-rc63f` are live defects in their own
domains and carry their own beads: `ranger-base-y52v6` (security, P2 — the commit
wall's class matched against the staged path string), `ranger-base-7qnag`
(devops, P2 — the installed hook predates the widened wall). Nothing in this
bead touches them.
