# The home config file leaves the seatbelt write set when the line names the config dir (ranger-base-ntnst)

*2026-11-11 · gilfoyle · ADR 0090 D2, the seatbelt half of
ranger-base-nv3el*

## What changed

On the sandbox-exec mechanism, a claude seat whose line types
`CLAUDE_CONFIG_DIR` no longer gets `$HOME/.claude.json` or its
atomic-write siblings in its write set. That seat writes
`$CLAUDE_CONFIG_DIR/.claude.json`, which is inside the config dir it is
already granted, so the home file was a grant with no writer behind it.
This is ADR 0080 D19's rule, which bwrap has followed since
ranger-base-tc41o.

Code: `SeatbeltStateDirs` / `seatbeltClaudeStateDirs` in
`internal/posse/seatbelt.go`. The four seatbelt readers that used to take
`rt.StateDirs` raw now take it from there: the profile render and the
`posse gates` report (`wall.go`), the reach probe (`reachability.go`), and
D1's MCP-file refusal (`mcpconfig.go`). One list, so the profile and the
checks that judge it cannot disagree.

The comment above `SeatbeltSiblings` that rejected
`CLAUDE_CONFIG_DIR=$HOME/.claude` is rewritten. Its premise was that the
variable would move the operator's file; ADR 0077 D1 already types it.

## The arms

| launcher env | seat's config file | home file in the seatbelt write set |
|---|---|---|
| `CLAUDE_CONFIG_DIR` unset | `$HOME/.claude.json` | yes, with siblings (ranger-base-cypy1) |
| set, any dir but `$HOME` | `<dir>/.claude.json` | **no** (D2) |
| set to `$HOME` itself | `$HOME/.claude.json` | yes: dropping it would lose the seat's writes |

The last row is a narrowing beyond D2's letter. bwrap does not need it,
because its config-dir grant is then the whole home.

Unset is not a refusal here, unlike bwrap: sandbox-exec can grant a file
plus its sibling namespace, which a bind mount cannot (probe B8).

## Pins

`internal/posse/seatbeltclaudeconfigdir_qa_test.go`, arm 1. It renders
the real sandbox-exec profile through `renderWallOn` from any host. It
checks the variable-set arm, the unset control (without it the first pin
passes on a render that never granted the file), the MCP write set
agreeing with the profile, and the pure arms. MEASURED 2026-11-11 on the
Linux build host: with `SeatbeltStateDirs` mutated to return the
declaration, the render pin and the MCP pin both go red.

## Not done

The live half: run a caged claude seat under the rendered profile on
darwin and see a write to `$HOME/.claude.json` refused while one to
`$CLAUDE_CONFIG_DIR/.claude.json` lands. No darwin host was available to
this seat, so that check belongs to the verify lane.

The seatbelt write set still grants the declared `~/.claude`, not the
directory `CLAUDE_CONFIG_DIR` names. On this box they are the same path.
On a box where they differ, the seat's config writes were outside the
grant before this change too.
