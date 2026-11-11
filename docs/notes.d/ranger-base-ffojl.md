# ranger-base-ffojl — `exec.LookPath("git")` is the bare name, not an escape from it

`internal/treepins/fixturegitpin_qa_test.go` censuses the test corpus for a
fixture git child that will meet the seat's own L1 shim and be refused
(`hookskip:config-env`, ADR 0079, ranger-base-r3pri — the defect
ranger-base-xa2e3 closed at ten sites). ranger-base-l0bep made its binary half
read two spellings: the inline literal `exec.Command("git", …)`, and an
identifier resolving one hop to that literal (`gitBin := "git"`). It reads a
**call** as compliant, which is right for the prescribed fix
(`gitBin := gitOutsideGates(t)`) and wrong for exactly one call:

```go
gitBin, err := exec.LookPath("git")
cmd := exec.Command(gitBin, …)
```

## The decision

**Yes — `exec.LookPath("git")` is a bare-name binding for this pin's purposes**,
and reading it as one is not a widening of the rule's subject but the
recognition of it.

`os/exec` resolves a name with no path separator by calling `LookPath` on it.
So `exec.Command("git", …)` and `gitBin, _ := exec.LookPath("git")` followed by
`exec.Command(gitBin, …)` are the **same operation against the same live
`$PATH`**, differing only in whether the answer is stored in a variable first.
MEASURED 2026-11-10, this box (linux 7.2.15, go1.27.3), from a gated crew seat:

| expression | value |
|---|---|
| `exec.LookPath("git")` | `<home>/state/gates/<persona>/bin/git` |
| `exec.Command("git").Path` | `<home>/state/gates/<persona>/bin/git` |

Byte-identical, and both the seat's own L1 shim. The value is an absolute path,
which is why it reads as compliant; the identity is the gate, which is the
whole subject of the pin.

What the bead cautioned against is a **different** change — "a call is a bare
name unless it is `gitOutsideGates`" — which inverts the rule's default and
would read every unresolvable binary as a violation (ranger-base-l0bep measured
71 such sites, almost all function parameters). That is not what landed. What
landed matches **one** call, on the literal `"git"`, bound to a local
identifier the call then execs, and nothing else. Six of the nine arm-2 cases
this bead added are the boundaries it must not cross.

The narrow reading also carries a nudge the other two spellings do not. The
LookPath result *looks* like compliance from every angle a reader has: it is
absolute, it was computed rather than typed, and one site in this tree
documents the shape as the house form for exactly that —
`internal/posse/hooksredirect_qa_test.go`'s `hrFix` field comment reads
`gitBin string // git by ABSOLUTE path — ADR 0052 M2's own form`. An author who
needs "git by absolute path" and has no `gitOutsideGates` in their package is
led to the wrong absolute path by the tree's own prose. `gitOutsideGates` exists
because LookPath is not safe here: it walks `posse.PathOutsideGates("")` by hand
and takes the first git in no gates dir (`internal/treepins/gitoutsidegates_test.go`,
`cmd/posse/gateschain_qa_test.go`).

## MEASURED — the class, reproduced

gwart, 2026-11-10, this box, on the merge of 4f797308 and ranger-base-l0bep's
234bae82, by an independent AST probe (`fgpScan`'s machinery with an
`isLookPathGit` accept test, the env verdict and the TestMain immunity
derivation unchanged):

| | |
|---|---|
| `exec.LookPath("git")` call sites in the test corpus | 114 |
| of those, binding the **binary's own** LHS slot | 9 |
| of those, bound only to `_` (the availability probe) | 105 |
| git children exec'd through one of the 9 | 2 |
| of those, handed an env that unpins git's config scope | 2 |
| of those, in a package **not** immune and not stripping PATH | **0** |

Identical to the bead's own reading. The two are
`internal/posse/commitwallowner_qa_test.go:448` and
`internal/posse/hooksredirect_qa_test.go:654`, both in `internal/posse`, whose
TestMain steps the whole binary out from behind the gates (`herdr_test.go`,
rangerhq-8sd). **Nothing was red and nothing needed converting.** A census of
this must bind the binary's slot: a grep, or a census that accepts any named
identifier anywhere in the LHS, reports 114 for a population of 9, because
`err` in `if _, err := exec.LookPath("git")` is a named identifier.

## MEASURED — the gap, end to end

Same box and day. `internal/treepins/policypins_qa_test.go`'s `ppRepo` — one of
the ten sites ranger-base-xa2e3 converted — mutated from
`gitBin := gitOutsideGates(t)` to `gitBin, lpErr := exec.LookPath("git")`, md5
checked before and after and restored byte-identical:

| what ran | result |
|---|---|
| the policy pins the fixture protects | **FAIL** — `refused by posse gate: git -C /tmp/… commit -qm qa fixture -- . — GIT_CONFIG_GLOBAL, HOME or XDG_CONFIG_HOME changes the hook config git sees … (rule: hookskip:config-env, ranger-base-r3pri)`, at four tests |
| the census as ranger-base-l0bep left it | **ok** 0.424s, 66 / 39 / 39 — the site is not even counted |
| the census with this bead's arm | **FAIL** 0.202s, 69 / 42 / 41, names `policypins_qa_test.go:117 (ppRepo)` and prescribes the binding |

So the shape is not a theoretical gap: it reproduces ranger-base-xa2e3's
original refusal verbatim, and the guard meant to catch that refusal read the
broken tree as clean.

## MEASURED — the counts move, by exactly two

The bead predicted the totals would **not** move, reasoning that the two
LookPath children were already counted as sites through their `.Env`. They were
not. Same corpus, 775 test files, two readings a minute apart with the site
lists diffed:

| census | files / sites / unpinning / immune |
|---|---|
| ranger-base-l0bep's | 775 / 66 / 39 / 39 |
| with this bead's arm | 775 / **68** / **41** / **41** |

The diff of the two site lists is exactly the two lines named above and nothing
else — which is what rules out the n-from-1 assignment form having also widened
the **env** half it feeds.

The ordering is the reason: **the binary half gates the env half.** A child
whose binary is not read as a bare name never enters `kids`, so its `.Env` is
never looked at and it is not a site at all. Anything that widens the binary
half moves these counts; anything that widens only the env half cannot. Worth
keeping, because this is the second bead in a row to reason about the counts
from the env half alone.

The floors were left where they are. They sit at roughly half the live counts
on purpose — to catch a walk that has stopped reading the tree, not to pin an
exact census — and a floor raised to the live number reds on every unrelated
bead that converts a fixture.

## What changed

- `fgpLookPathGit` — a new accept test for `exec.LookPath("git")` and nothing
  else: one argument, the literal `"git"`, the `exec` package qualifier.
- `fgpBareGit` applies **two** accept tests to each local assignment, the
  literal first (so a name bound both ways is reported as the plainer of the
  two), and returns whether the hit came through LookPath.
- The local-assignment map records the **n-from-1** form (`v, err := f()`), for
  `Lhs[0]` only. Without this the binding never enters the map and the
  identifier resolves to nothing however the accept test reads it — the first
  of the two misses, and sufficient on its own. `Lhs[0]` only because that is
  the slot the value lands in for every `v, err := f()` in this corpus, and
  mapping `err` to `f()` would be a false statement about what that name holds.
- Arm 1 gets its own sentence for this shape. The ordinary prescription reads as
  already done at a LookPath site — the call *does* exec a variable and the
  variable *does* hold an absolute path — so the message says which absolute
  path is wrong and why, before naming the binding to change.
- Arm 2 grows six cases: the shape itself, and five boundaries — the
  availability probe (binds no binary), another binary's LookPath, a LookPath of
  a variable, an n-from-1 binding to an arbitrary call, and a LookPath value
  carried through a struct field. Twenty cases.

Each of the two misses was mutated out separately and the new case reds on
either alone, with the live census falling back to 66 / 39 / 39 both times — so
both halves of the change are load-bearing and the new case is not vacuous.

## Not claimed

- That any site in the tree is affected today. 0 live, MEASURED twice above.
  This closes a guard before the class reaches it.
- That `exec.LookPath` is read anywhere but in that one hop, on the literal
  `"git"`, bound to a local identifier the call then execs. Three shapes are
  deliberately outside it, all in `internal/posse` and immune: the availability
  probe (105 of 114 calls), `exec.LookPath(someVar)`, and a LookPath value
  carried into a struct field and exec'd as `f.gitBin` —
  `hooksredirect_qa_test.go`'s `hrFix`, which is two hops and a selector, past
  the depth **both** halves of the verdict stop at. Arm 2 pins all three as
  `want: false`, so raising the depth later is a deliberate flip and not a
  surprise.
- That the broader reading was declined on its merits as a reading. It was
  declined because it inverts this rule's default over sites nobody has
  measured as broken, in a tree where `cmd/posse`'s PATH leak is a live defect
  *pinned green* by the operator triage ruling of 2026-09-04
  (`gateschainpathleak_qa_test.go`). The narrow reading needs no such ruling: it
  reds nothing that was green, which is measured and not assumed.

## A note on the base

This bead's subject is the accept test ranger-base-l0bep added, and that commit
(234bae82) was not on `main` when this session was dispatched — jian-yang's
branch was inside the launcher's grace window, thirty seconds ahead of this
worktree's base. It was **merged** into this session's branch rather than
re-authored, so its commit, author and bead id stay intact and `main` takes it
exactly once. Every measurement above was taken on that merge.
