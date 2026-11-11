# The caged write set, narrowed to what a seat writes and not what runs later (ranger-base-6akzw)

*2026-11-11 · gilfoyle · the fix half of ranger-base-y438o (hoover's finding,
hoover re-verifies) · touches ADR 0080 D17/D19 and the seatbelt mechanism
through the shared producers*

y438o measured that the caged write set reached three places that hold code
or config run LATER and outside the wall — by other seats, the launcher, or
the operator. Both mechanisms read the same producers (`SeatbeltWritable`,
`SeatbeltCarveOut`), so each change below lands at bwrap and seatbelt alike.

## What changed

| was writable | now | where |
|---|---|---|
| `~/.local/share` (XDG_DATA_HOME: mise installs and wrappers, systemd user units, desktop entries) | not granted | `homeCacheGrants` |
| posse's whole state dir (launch lines, session records, skills trees, backups, dispatcher files, prompts) | not granted; the seat's own gates dir and its two audit logs stay | `SeatbeltWritable` |
| claude config dir's executable members: `settings.json`, `settings.local.json`, `CLAUDE.md`, `skills/`, `plugins/`, `agents/`, `commands/`, `hooks/`, `output-styles/`, `rules/` | read-only over the config-dir grant; absent ones are created empty first so a mount can cover them | `claudeExecConfig`, `SeatbeltCarveOut`, `BwrapArgs` (new prep step `seed`) |
| the launcher's own write of a spilled launch line opened the name | temp file + rename, so nothing planted at the name is followed | `PaneLine` |

The launch-line race closes by the second row: no caged seat can write
`state/launch/` at all. The rename is the half that holds for a writer no
wall covers.

## Measurements

- MEASURED 2026-11-11, D1 host, claude through mise's wrapper, run under
  bwrap with `settings.json`, `skills/`, `plugins/` and `~/.local/share`
  bound read-only: `claude -p` exits 0 with its answer, and its debug log
  shows no refused write. User-scope paths the CLI stat'd or loaded:
  `settings.json`, `agents/`, `commands/`, `output-styles/`, `skills/`,
  `plugins/` — those, plus the documented `CLAUDE.md`, `rules/`, `hooks/`, are
  the carve.
- hoover's census said `plugins/` cannot go read-only whole because the CLI
  writes `known_marketplaces.json` and a cache file there. It writes them; the
  turn above did not need them. ASSUMED that the interactive marketplace
  refresh fails quietly too — one print-mode turn is not a seat's lifecycle,
  and y438o's live-seat re-verify is the reading that settles it.
- Census of `filepath.Join(a.StateDir, …)` over `cmd/` and `internal/`: every
  writer of a state-dir member is the launcher or the dispatcher. The grant
  was the old `~/.config/rhq/state` literal made home-relative
  (ranger-base-cpyb), with no measured seat writer behind it.
- `~/.local/share` files modified in the two days before: the operator's
  shell (zoxide), systemd, mise itself, and a desktop entry claude writes
  best-effort at startup.

## Pins

`internal/posse/wallexecconfig_qa_test.go`. The live pin hands posse's own
render to the real bwrap and tries a write at every row of y438o's table
(18 refused) and at the writes claude needs (7 allowed: history, projects/,
the `.claude.json` atomic replace, a new state member, the seat's audit log,
its tree, `~/.cache`). Green with the fix; with the three old grants put back
the file reports 31 failures.

## Not closed here

- `.claude.json` holds MCP server commands and must stay writable (the CLI
  replaces it every turn). Codex and grok config are the same class,
  unmeasured. A design, not a carve: ranger-base-nv3el.
- `~/.cache` and `~/.npm` still hold content later builds and tools execute
  (Go's build cache, npx packages). Severity call and design:
  ranger-base-olr0g.
