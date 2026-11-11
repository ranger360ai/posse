# ranger-base-l0bep — the fixture-git census reads the BINARY one hop too

`internal/treepins/fixturegitpin_qa_test.go` censuses the tree's test corpus
for a fixture git child that will meet the seat's own L1 shim and be refused
(`hookskip:config-env`, ADR 0079, ranger-base-r3pri — the defect
ranger-base-xa2e3 closed at ten sites). Its subject has two halves, and until
this bead they were read to different depths:

- the child's **`.Env`** was resolved **one hop** — through a local slice,
  through an `append` over a same-package helper, through a replacement slice
  — because all three converted sites needed it;
- the child's **binary** was matched as a **string literal only**
  (`!fgpStringLit(bin, "git")`).

That asymmetry is a gap, and specifically a gap in the shape the pin
**mandates**. The prescribed fix is `gitBin := gitOutsideGates(t)` then
`exec.Command(gitBin, …)`, so every converted site now execs a *variable*. An
author editing one of those sites, or copying the pattern into a package with
no `gitOutsideGates` helper, writes `gitBin := "git"` and is one token from
the defect with nothing to tell them — while the pin's floor ("0 elsewhere")
reads to the next person as "the class is closed".

## MEASURED

holden, 2026-11-10, this box (linux 7.2.15, go1.27.3, git 2.56.0), from a
gated crew seat at 10d046f9, mutating `policypins_qa_test.go`'s `ppRepo`
(ranger-base-b079k):

| arm | shape | census | the tests it protects |
|---|---|---|---|
| A | `exec.Command("git", …)` (the pre-fix revert) | **FAIL** 0.168s, names the site | red |
| B | `gitBin := "git"`, call unchanged | **ok** 0.187s | red, with ranger-base-xa2e3's original refusal |

Arm B is the pin green over its own subject.

jian-yang, 2026-11-10, same box, at 6365a923, with the hop in place and the
same mutation re-run (md5 of the fixture checked before and after, restored
byte-identical):

| arm | census | message |
|---|---|---|
| A (inline literal) | **FAIL** 0.195s, 67 / 40 / 39 | prescribes the call: `gitBin := gitOutsideGates(t)`, then exec that |
| B (local binding) | **FAIL** 0.193s, 67 / 40 / 39 | prescribes the **binding**: `gitBin := gitOutsideGates(t)` in place of the bare name |
| unmutated | **ok** 0.189s, 773 / 66 / 39 / 39 | — |

Both counts move by exactly the one mutated child, as arm A did for holden.

## What changed

`fgpBareGit` resolves an `*ast.Ident` first argument through the assignments
to that name **in the exec'ing function**, applying the **same**
`fgpStringLit` accept test the inline spelling gets. Depth and rule are the
env half's, deliberately: one hop, local only. The local-assignment map is
collected *before* the children now, because the binary half reads it too and
Go does not require the binding to precede the use in source order.

`fgpSite` carries the identifier it came through (`via`), so the failure names
the binding rather than prescribing an indirection the site already has.

Arm 2 grew three shapes — the blind spot itself (`want: true`), an absolute
path in a local binding (`want: false`, the hop must not over-match), and a
bare-name binding in a *sibling* function (`want: false`, the hop is local as
the env hop is). Fourteen cases, all green.

## Not claimed

- That any site in the tree is affected today. The counts are unchanged
  (66 / 39 / 39); the indirected spelling exists nowhere here.
- That more than one hop is read — a function parameter, a package-level var,
  a second identifier. Same depth as `.Env`, same reason: a hop further starts
  reading other packages' helpers and nothing in the tree needs it. What the
  one hop buys is **symmetry**, so neither half of the subject can be restored
  by moving a token out of the call.
- That the pin was ever *wrong* about what it measured. Its head comment named
  the literal, so this was an incompleteness, not a false statement.

## Companion measurement: the third spelling (→ ranger-base-ffojl)

The hop reads an identifier that resolves to the bare **literal**. It reads a
**call** as compliant, which is right for the prescribed
`gitBin := gitOutsideGates(t)` and wrong for exactly one call:

```go
gitBin, err := exec.LookPath("git")
cmd := exec.Command(gitBin, …)
```

`exec.LookPath` resolves against the **live** `$PATH` of the calling process,
which on a gated seat leads with that seat's gates bin dir — so it returns the
absolute path *of the shim*. The value is an absolute path (hence compliant);
the identity is the gate. `gitOutsideGates` exists because LookPath is not
safe here: it walks `posse.PathOutsideGates("")` by hand
(`internal/treepins/gitoutsidegates_test.go:17`,
`cmd/posse/gateschain_qa_test.go:113`).

`fgpScan` misses it twice, either miss sufficient on its own:

1. `gitBin, err := exec.LookPath("git")` is a 2-from-1 `AssignStmt`, and the
   local-assignment map is built under `len(as.Lhs) == len(as.Rhs)` — so the
   binding never enters it;
2. entered, the RHS is an `*ast.CallExpr`, and the accept test is
   `fgpStringLit`.

MEASURED 2026-11-10, this box (linux 7.2.15, go1.27.3), at 6365a923, by an AST
census over the same 773-file corpus — `fgpScan`'s own machinery with an
`isLookPathGit` accept test swapped in for `fgpStringLit`, the env verdict and
the TestMain immunity derivation unchanged:

| | |
|---|---|
| `exec.LookPath("git")` bindings in the test corpus | 9 |
| git children exec'd through one | 2 |
| of those, handed an env that unpins git's config scope | 2 |
| of those, in a package **not** immune and not stripping PATH | **0** |

The two are `internal/posse/commitwallowner_qa_test.go:448` and
`internal/posse/hooksredirect_qa_test.go:654`, both in `internal/posse`, whose
TestMain steps the whole binary out from behind the gates (`herdr_test.go`,
rangerhq-8sd). Correctly excused today, for the same reason the census's other
39 sites are. **Nothing is red and nothing needs converting.**

Not folded into this bead: reading a call as a bare name inverts the rule's
default, and widening an enforcement line over sites nobody has measured as
broken — in a tree where cmd/posse's PATH leak is a live defect *pinned green*
by the operator triage ruling of 2026-09-04
(`gateschainpathleak_qa_test.go`) — wants a decision rather than a diff from
whoever noticed. Filed as **ranger-base-ffojl** (P3, 0 live sites) with the
narrow change, the caution and the done-when.

A census of this must bind the LHS: ~115 occurrences of `LookPath("git")`
match a grep, and 106 are the availability probe
`if _, err := exec.LookPath("git"); err != nil { t.Skip(…) }`, which binds to
`_` and names no binary. A grep reports 115 for a population of 9.
