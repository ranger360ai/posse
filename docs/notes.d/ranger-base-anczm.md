# Three guardrails were deletable green because the verb that wires them was unreachable (ranger-base-anczm)

`posse upgrade`'s guardrails were each pinned as a *function* — `upgradeFence`
over its two strings, `upgradeResolve` over a repo, `upgradeRunPlan` over a
plan. Nothing pinned the function that *calls* them, because nothing in the
tree reached `App.CmdUpgrade`: it read the running binary's own build stamp,
and a test binary carries none. `upgrade.go` said so in a comment, which is
how the hole stayed shaped like a design note.

MEASURED 2026-11-13 at `e9a38609`, three one-line mutations applied alone,
`go test -count=1 -run TestUpgrade ./internal/posse`:

| mutation | before | after |
|---|---|---|
| the D2 `install_delegate:` fence call unwired | green | red |
| `if lag.Dirty` → `if false && lag.Dirty` | green | red |
| the D3 box flock removed | green | red |

Three more, found while fixing it and now covered by the same section:

| mutation | after |
|---|---|
| the target guessed as `$HOME/.local/bin/posse` instead of `RunningPosse().Exe` | red |
| the "no source checkout" refusal switched off | red |
| the `<sha>` argument dropped on the way to `upgradeResolve` | red |

CONTROL, so none of the above is a dead filter: replacing `upgradeSwap`'s
`rename(2)` with an in-place `os.WriteFile` reds
`TestUpgradeSwapNeverLeavesTheTargetAbsentOrShort` and the green-path pin —
the same control the finding was filed with.

## The fix is a seam, not a text pin

The two shapes on the table were a source-level pin in `internal/treepins`
asserting the fence call is the verb's first statement, and a seam making the
verb reachable. The text pin was cheaper and would have covered one of the
three mutations: a pin on "statement 1 is the fence call" says nothing about
`if lag.Dirty` or about the flock, so the other two would have needed a text
pin each, and a text pin cannot see a guardrail that is still *called* and no
longer *refuses* (mutation 2 is exactly that shape).

So `upgradeReads` — the three readings of the live box, handed in:

```go
type upgradeReads struct {
	Bin  func() PosseBinary                    // RunningPosse
	Lag  func() LauncherLag                    // (*App).LauncherOverFleet
	Lock func(io.Writer) (*upgradeLock, error) // lockUpgrade
}
```

`CmdUpgrade` binds those three names to the real readings and calls
`cmdUpgrade`, which is the verb. Parameters and not package vars like the
file's three shelling-out steps, because all three call sites are in one
function and this package's suite is parallel — a package var swapped under
one test answers questions no other test asked (`runningPosse`'s own header).
It is dispatch.Lag's and GovInputs.Lag's seam, for dispatch.Lag's reason.

## What every pin in the section asserts

A deleted guardrail changes no output. It changes which *readings* happen
after it — an unwired fence reads the binary, a dropped dirty-stamp refusal
reaches the lock, a removed flock reaches the resolve. So `upgradeSeams`
records the order it was asked in, and each pin states what the verb did
*not* read as well as what it did. That is why mutation 3 reds two tests and
not one: the lock pin loses its reading, and the target pin's stubbed `make
test` fires on a run that got past a lock it never took.

## Not closed here

`CmdUpgrade` itself — the three lines binding the seam's names to
`RunningPosse`, `(*App).LauncherOverFleet` and `lockUpgrade` — is still
reached by nothing. It is the residue every seam of this shape leaves
(dispatch.Lag and GovInputs.Lag leave the same one) and it is the smallest
piece the file can leave unreached; `upgradeReads`' header says so where the
next reader will be standing.
