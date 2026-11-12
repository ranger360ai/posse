# The CLI-home fence, derived from the resolvers and checked by asking them (ranger-base-vxir7)

2026-11-12. Closes the class behind ranger-base-u3kft (internal/posse) and
ranger-base-6sddu (cmd/posse): three test binaries each fenced the runtimes'
credential and config dirs by spelling variable NAMES in their TestMain, and
the credential locator honoured a name none of them spelled.

## What changed

- `internal/posse/clihomefence.go` (shipped, exported because cmd/posse is
  package main and test code cannot cross packages):
  - `CLIHomeOverrideVars()` — `credentialDirVars` plus `CodexHomeVar`,
    `GrokHomeVar`. The resolvers' own list, a fresh slice each call.
  - `FenceCLIHomes(root, rows)` — UNSETS every one of them (never `""`:
    an empty `CLAUDE_SECURESTORAGE_CONFIG_DIR` shadows `CLAUDE_CONFIG_DIR`
    and resolves to the home), applies the binary's own rows, then calls
    `CLIHomeFenceEscapes`.
  - `CLIHomeFenceEscapes(root)` — asks `CredentialsFile`,
    `ClaudeConfigDirIn`, `ClaudeConfigFile`, `codexHomeIn`, `grokHomeIn`
    and names each whose answer is not under root (worktree.go's
    symlink-resolving `pathUnder`). The error names resolvers and variable
    NAMES in force, never values.
- The three TestMains call it. internal/posse fences into its temp HOME with
  no rows (its old `""` rows for the three runtime homes are now unsets) and
  exits 1 rather than panicking, so it can remove the temp HOME it made.
  cmd/posse and internal/treepins keep the operator's HOME by design, so
  they now point `CODEX_HOME` and `GROK_HOME` into the fence as well as
  `CLAUDE_CONFIG_DIR` — before this, both binaries' codex and grok homes
  resolved to the operator's own `~/.codex` and `~/.grok`.
- Four cmd/posse tests reached codex through the `$HOME` fallback of a
  sandbox HOME and now name it: `planEnvAt` carries a `CODEX_HOME` row and
  `TestCockpitCodexHintComesFromTheRolloutsOnDisk` sets it with t.Setenv.
- The source-reading pins (`credentialfencecheck_test.go` in cmd/posse and
  internal/treepins, arms 2 and 3 of `credseamfence_qa_test.go`) are gone:
  they read TestMain's literals, and TestMain no longer has any. Their
  replacement is not CI-blind either — each binary's pin puts every honoured
  variable INTO force itself (t.Setenv to a scratch dir, then to `""`),
  requires the outcome check to see it, and requires the fence to take it
  out again.

## Verified

MEASURED 2026-11-12 on linux/amd64, with `CLAUDE_SECURESTORAGE_CONFIG_DIR`
exported to a scratch dir: all three binaries ran a filtered set clean;
with `CLIHomeOverrideVars` mutated to drop that one name, all three stopped
at TestMain (`CLI-home fence: CredentialsFile answer outside the fence …;
runtime-home variable names in force: … CLAUDE_SECURESTORAGE_CONFIG_DIR …`),
internal/posse with exit 1, the other two with a panic.

## Not covered

- bob's home has no override variable and resolves `$HOME/.bob`; in
  cmd/posse and internal/treepins that is the operator's. No variable, so
  nothing for this fence to unset — out of scope here.
- `credentialFileCandidates` lists the home's own credentials file
  unconditionally, by design (it is the wall's input, not a resolver).
