# Every claude line ignores the MCP servers a seat can write (ranger-base-m0bfv)

2026-11-11, Linux 7.2.15 x86_64, bwrap on PATH, claude 2.1.322 (mise), from
this bead's worktree. The build half of ADR 0090 D1 and D4, from
ranger-base-nv3el (the design).

## What landed

- `--strict-mcp-config` is in claude's built-in template and in
  `DefaultAgentCommand` (`ClaudeStrictMCPFlag`, runtime.go).
- `EnsureStrictMCPLine` appends it to any line whose first word is `claude`
  and that lacks it. It keys on the LINE, not on the runtime, so a declared
  runtime whose template starts `claude` gets it too. It runs in
  `RenderCommandFor`, next to the unattended and settings-pin doors, and in
  `EnsureUnattendedLine` for the `--cmd` of a no-persona launch. It is its own
  constant and not part of `ClaudeFleetFlags`, because EnsureUnattended keys
  on that string's first word, and a line that types `--permission-mode plan`
  would then lose this flag too.
- `posse runtime check`'s launch row says the flag is on the line. When claude
  runs behind a first word the door cannot see past (`env X=1 claude`), the
  row says `NO --strict-mcp-config` instead.
- `MCPConfigWallRefusal` (mcpconfig.go) refuses a launch or relaunch whose
  line passes `--mcp-config` a file inside the seat's write set. It checks
  against the write set of the mechanism actually in force: sandbox-exec's
  grants plus atomic-write siblings, or bwrap's bound set after D19/D20, minus
  each one's carve-out deny. It checks the path both as typed and with links
  resolved. A value it cannot resolve without a shell (`$VAR`, `$(…)`) is
  refused. Inline JSON names no file and passes.

## Pins

- `TestQAStrictMCPOnEveryClaudeLinePosseCanProduce`: built-in template,
  `DefaultAgentCommand`, every tier of claude, codex, grok and a declared
  `claude` runtime, a PID's own `command:` in five shapes, and the `--cmd`
  line. Each claude line carries the flag exactly once, and no other CLI's
  line carries it.
- `TestQAMCPConfigInsideTheWriteSetIsRefusedAtBothMechanisms`: ten cases at
  each of the two mechanisms.
- `TestLiveStrictMCPStartsOnlyTheLineServersUnderTheRenderedWall`: the live
  join (ADR 0090 D4). It uses posse's bwrap render with `--unshare-net` added
  in front (a narrowing). First control: the plant into the seat's
  `.claude.json` and a repo `.mcp.json` SUCCEEDS. Under the rendered line plus
  one legitimate `--mcp-config` outside the wall, only the legitimate server
  starts. Second control: without the flag, all three planted servers start.
  A write to `$HOME/.claude.json` is refused. MEASURED green twice, 42.5s and
  55.0s.

## allowedTools in .claude.json — MEASURED not read on 2.1.322

The ADR left this UNMEASURED because a static read of the binary was
inconclusive, and the debug log names no rule loading either. So I measured
it by behaviour, with no billed turn. A local fake Messages API answered the
first request with one `Bash` tool call (`touch <marker>`) and the second with
`end_turn`. claude ran in `-p` mode, in the default permission mode, with a
dummy key and `ANTHROPIC_BASE_URL` pointed at the fake. Each arm got a fresh
scratch `CLAUDE_CONFIG_DIR`. All three arms were run twice, once under the
harness temp dir and once under /var/tmp, with the same result both times:

| arm | rule | marker |
|---|---|---|
| control | `--allowedTools 'Bash(touch:*)'` on the line | created |
| `.claude.json` | `projects.<cwd>.allowedTools: ["Bash(touch:*)","Bash"]` | NOT created: "needs approval" |
| none | nothing | NOT created: "needs approval" |

The key was still in the file after each run, so claude neither honours it
nor migrates it. No bead goes to the architecture lane (the bead's step 5 is
conditional on "if it does"). Re-measure this when the claude pin moves.
That is ADR 0090's own instruction for the key list.

## Not done here

- Seatbelt's live arm. The refusal's sandbox-exec write set is pinned from
  this host, but the kernel join needs darwin: hoover's re-verify bead.
- D2 (the seatbelt `~/.claude.json` grant) and D3 (codex/grok) are other
  beads (ntnst, zmk4q per monica's comment on nv3el).
