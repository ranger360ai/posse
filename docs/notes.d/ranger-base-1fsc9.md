# A safety filter put where nothing could read it (ranger-base-1fsc9)

The verify at ranger-base-gj8m2 filed one finding against `posse upgrade`:
the ADR 0052 D1 filter on D4 amendment 1's repo list — the whole of that
verb's obligation to that ADR — had no reader in the tree in either
direction. 2026-11-14, this box (linux 7.2.15 x86_64, go1.27.3, git 2.56.0,
16 cores), branched at e3b41b30.

Nothing was wrong at HEAD. What was missing was a pin, and the thing worth
keeping is WHY no pin could have been written where the line sat.

## The seam ate the line, and the file had already said so

`cmdUpgrade` fills `plan.Repos` so the report can print `posse gates
install-hooks <repos>`, and it filled it by walking
`SweepHookWallIdentity().Repos` under `if r.Skip == ""`. That filter is ADR
0052 D1 at this site: a managed hooks path is a path posse must never be
told to write, and the remedy line is the one instruction the ADR forbids.
The close comment, the code comment and the ADR all named it as the property
being held.

`upgrade.go` already declared the seam that swallowed it. `upgradeReads`
carries the three readings a pin can supply — the build stamp, the launcher
lag, the box lock — and its own header names what is still unreachable:
`CmdUpgrade`, "which binds these three names to the real readings and does
nothing else — no branch, no refusal". The repo derivation was the one
branch on the wrong side of that line. `SweepHookWallIdentity` is an `App`
method and does not go in through `upgradeReads`, so reaching the loop from
a test needed a real stamp, a real checkout and a real lock; and
`d4Fixture.run` — the one fixture that drives the plan — calls
`upgradeResolve` and then assigns `plan.Repos = f.repos` outright
(`upgradeverify_test.go`), deriving nothing.

So the derivation sat behind a seam the file itself declared total, and the
filter sat inside the derivation. This was not an oversight in
ranger-base-hm4gf's close, which met every line of its own DONE WHEN. It is
a line that was put where nothing can read it.

MEASURED, the mutant that found it (ranger-base-gj8m2, at main 2bb926f3):
`if true` in place of `if r.Skip == ""` was **GREEN** over 77 tests in 2.2s,
and under it a managed checkout and an absent one both land in the
operator's install-hooks line.

## The fix is smaller than what it replaces

`upgradeHookWallRepos(s HookWallSweep) []string` is the filter as a package
function over the sweep value `cmdUpgrade` already holds. `cmdUpgrade` calls
it in place of the inline loop. A `HookWallSweep` is a slice of structs, so
on this side the same filter needs no running binary, no fixture `App` and
no change to the seam at all — the derivation MOVES and nothing is added.
`upgradeReads` keeps the residue it declares; the residue is now the
binding, which is what its header already claimed it was.

The pins are `internal/posse/upgradehookwallrepos_qa_test.go`, arm 1. Three
of them, and the second is the half a reader assumes and so the half that
rots quietly:

- every unmeasured repo is dropped, through each of the three reasons the
  sweep has for not measuring one. The managed arm takes its `Skip` string
  from `managedHooks.line()`, the producer the sweep itself uses
  (`hookfresh.go`), rather than a retyped literal — a pin that invents its
  own string is pinning a repo state the sweep cannot produce.
- every measured repo survives, in config order. A filter that kept only
  the first measured repo, or reversed them, passes the arm above.
- a sweep that measured nothing returns nil, which is the state every
  scratch home in this package is actually in, and is the positive control
  that says the empty answer the first arm asserts is reachable without the
  filter firing at all.

### What the mutants said

Six, one at a time, `perl -0777 -pi` with md5 before and after and the file
restored byte-identical (`md5 3db587bc44e74f2ce8391b1c39e4fbe9` both ends).

| mutant | before | after |
|---|---|---|
| `if true` in place of `if r.Skip == ""` (the filing's own) | **GREEN** | RED |
| `if r.Skip != ""` — only the skipped repos come back | GREEN | RED |
| `if !r.Managed` — the ADR arm held, the other two dropped | GREEN | RED |
| `return` after the first append | GREEN | RED |
| prepend instead of append — config order reversed | GREEN | RED |
| drop the append entirely | GREEN | RED |

6 of 6 killed. The third row is the one that says why the pin asks about all
three skip reasons and not only the managed one: a filter narrowed to ADR
0052's own arm still prints an install-hooks line for a checkout that is not
there.

## Not changed, and not re-filed

`govern.go`'s G12 loop reads the same sweep and is not this shape: it
iterates `HookWallRepo.Findings()`, which is empty for a skipped repo, so
the filter there is the data's and not a predicate of its own. No function
is shared between the two sites.

The other finding on ranger-base-1fsc9 — the unread `ROTATE:` line at
`wallwatch.go` — was a pure test addition and was fixed in-lane by
ranger-base-gj8m2 itself (`internal/posse/verify_gj8m2_qa_test.go`, 9abbf491
on that bead's branch and 9c8052db as it landed). It is recorded here so
nobody looks for it twice.
