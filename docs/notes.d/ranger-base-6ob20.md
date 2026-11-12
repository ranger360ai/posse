# The rest of codex's and grok's CLI-written home members, carved or private (ranger-base-6ob20)

*2026-11-12 · dinesh · follows ranger-base-zvgql; finding ranger-base-74pqi*

## What changed

The zvgql carve now covers the home members that the CLI writes itself and
that a later CLI runs or obeys. The same rules apply: the carve happens only
when the launching runtime's state grant reaches the home, it follows
`CODEX_HOME` / `GROK_HOME`, directories are denied whole, and files are
seeded empty. Code: `internal/posse/wallexecconfig.go`.

| CLI | dirs | files |
|---|---|---|
| grok | `bin/ bundled/ bundles/ memory-v2/` | `channel-switch.json hooks-paths sandbox.toml` |
| codex | `.tmp/ memories/` | |

The grok census is grok's own. Its sandbox doc (`docs/user-guide/18-sandbox.md`,
"Direct global write protection", grok 1.0.77) write-denies `bundled/`,
`bundles/`, `hooks-paths` and `sandbox.toml` for the reason this wall carves
them. `bin/` and `channel-switch.json` are the managed install and its
channel. `memory-v2/` is injected into later sessions when memory is on.

**codex's `tmp/` and `shell_snapshots/` are per-process state, so they get a
different shape.** On bwrap each seat gets an empty private `--tmpfs` over
both. It is mounted after the home grant, and its deny is taken back out of
the carve so that no read-only bind from the host covers it again
(`bwrapRender`, `PrivateBind` with an empty `Src`). On seatbelt, which cannot
mount, both are carved read-only.

**Every grok seat's line now carries `GROK_DISABLE_AUTOUPDATER='1'`**
(`Runtime.LineEnv`, rendered by `RuntimeLineEnv` beside the home pin). This
is grok's own documented switch (`14-headless-mode.md`).

## Measurements

MEASURED 2026-11-12 on the D1 host, codex 0.167.2 and grok 1.0.77. The zvgql
method: scratch homes outside `/tmp`, posse's own `bwrapRender` argv, and a
local fake model endpoint, with no login and no billed turn.

- **codex exec, bwrap render** (private tmpfs, `.tmp/` and `memories/`
  read-only): the turn completes. codex makes its `tmp/arg0-linux` alias dir
  inside the private tmpfs and prints no alias warning. The host's `tmp/`
  and `shell_snapshots/` stay empty.
- **codex TUI, bwrap render**: one typed turn gets the fake reply. The only
  startup warning is the one about `-c` overrides needing embedded mode.
- **codex exec and TUI, seatbelt reading** (the same render with `tmp/` and
  `shell_snapshots/` swapped to read-only binds): both complete the turn.
  codex prints `could not create PATH aliases: Read-only file system` and
  proceeds. This was measured under bwrap as a stand-in. A darwin seatbelt
  run is ASSUMED to behave the same way, because the refusal is the same
  EROFS/EPERM-on-create class.
- **grok -p** with every new member read-only completes the turn and leaves
  each seeded member byte-identical (pinned). The `channel-switch.json` seed
  is `{}` and grok accepts it.
- **grok as a managed install**: the binary was copied to `$GROK_HOME/bin/grok`
  and run from there with `bin/` read-only. `-p` completed both with and
  without `GROK_DISABLE_AUTOUPDATER=1`, and every new member stayed
  byte-identical. There is no managed install on this host, so this is a
  stand-in for one. A real updater run against a read-only `bin/` is
  UNMEASURED.

## Pins

`internal/posse/wallruntimeconfig_qa_test.go`, `-run WallRuntimeConfig`:

- render, both mechanisms: every new member is in the measured-must list.
  codex's `tmp/` and `shell_snapshots/` are directory denies at seatbelt. At
  bwrap they are a `--tmpfs` after the home grant, with no host bind of
  either. Never for a grok or claude seat. `CODEX_HOME` moves them.
- live wall: under the rendered wall, files another process made in the
  host's `$CODEX_HOME/tmp` and `shell_snapshots/` are not visible. The seat
  can write its own there, and nothing it writes or moves reaches the host.
- live CLI: `codex exec` completes a turn with `tmp/` and `shell_snapshots/`
  read-only (the seatbelt reading). The existing turn pin now covers every
  new member for both CLIs.
- line: grok's wrapped line carries the updater switch after the home pin,
  and the shell hands it to the command. claude's and codex's lines do not.

Control: with the lists reverted, the tmpfs swap disabled and the line env
removed, four pins go red (24 failure lines).

## Costs

- On a box whose home lacks them, the launcher creates the absent dirs empty
  before it mounts over them (`~/.grok/bin`, `bundled`, `bundles`,
  `memory-v2`, `~/.codex/.tmp`, `memories`), as it already does for every
  absent directory deny. grok creates empty `bundled/` and `bundles/` itself
  under its own sandbox. An empty `~/.grok/bin` does not make a grok run
  from elsewhere count as a managed install, because grok's updater keys on
  the running binary's path.
- A caged grok cannot download or update its built-in bundles. A caged codex
  cannot refresh its curated marketplace clone.

## Not closed here

- ranger-base-qyq59
