# grok's project-config channel: eight entries, declared on a measurement (ranger-base-l12j9)

grok's built-in declared **no** `ProjectConfig` until this bead. That is the
state `runtimecheck.go`'s `projectConfigRow` calls *the silent one*:
`ProjectConfigTrust` (parity.go) skips a channel nobody declared, so an
unguarded repo→box channel read as a **CLEAN launch**. A claude seat over a
repo's `.claude/settings.json` hooks degraded; a grok seat over the **same
file** said nothing. That asymmetry is the ADR 0017 §2 / ADR 0002 hole
ranger-base-1g8gf found, and closing it is the whole of this bead.

## MEASURED 2026-10-31, grok 1.0.52 (0920c8169e27), darwin 25.6.0

By hoover under ranger-base-1g8gf. A fixture `$HOME` whose
`trusted_folders.toml` named only the trusted arm; identical repo content in
both arms; a junk value in grok's own API-credential env var (the name is
spelled in Go and not in markdown — see `BobCageCred`'s comment for the rule)
and every `*_PROXY` pointed at `127.0.0.1:9`, so the session initialised and
every model request died locally (log:
`api.x.ai/v1/responses ... tunnel ... Connection refused` — nothing left the
box). Each hook and MCP command was `touch <marker>`, and the marker is the
reading.

| file | key / entry | trusted | untrusted |
|---|---|---|---|
| `.claude/settings.json` | `hooks` | **RAN** (SessionStart, UserPromptSubmit) | nothing |
| `.claude/settings.local.json` | `hooks` | **RAN** (same two) | nothing |
| `.claude/settings*.json` | `permissions` | loaded (2 + 1) | 0 loaded |
| `.claude/settings.json` | `mcpServers` | NOT read — not listed, not spawned | — |
| `.mcp.json` | `mcpServers` | **SPAWNED** | not spawned\* |
| `.grok/config.toml` | `[mcp_servers]` | **SPAWNED** | not spawned\* |
| `.grok/hooks/*.json` | `hooks` | **RAN** | nothing |
| `.cursor/mcp.json` | `mcpServers` | **SPAWNED** | not spawned\* |
| `.cursor/hooks.json` | `hooks` | loaded, **REFUSED** by 1.0.52 | nothing |
| `.grok/plugins/` | (directory) | **NOT MEASURED** — ASSUMED | — |

\* `grok inspect` **LISTS** the untrusted repo's MCP servers anyway. That
listing is display only: nothing was spawned. Do not read inspect's MCP
section as the trust verdict.

`.cursor/hooks.json`'s refusal is in grok's own words: *"this Cursor hook
won't run until Grok reads Cursor hook replies"* — dormant, and declared
anyway, because the file is already being read and the refusal is a version's
rather than a design's. `.grok/plugins/` is **ASSUMED** off grok's own docs
(`09-plugins.md`: a project plugin there requires trust and carries hooks, MCP
and skills), not off a marker.

Stop/SessionEnd did not fire because the turn failed. That narrows nothing —
SessionStart alone runs code before any turn.

**Worktree inheritance, MEASURED:** a `git worktree add` of a trusted repo at a
path **outside** the trusted folder reads `Project trusted: yes` and runs its
`.grok/hooks`. So every posse worktree of every trusted checkout inherits the
grant — six trusted roots on this box, which is every repo the fleet dispatches
into — and posse sets no `GROK_FOLDER_TRUST` and no `[folder_trust]` anywhere.
It is the
same resolve-through-the-git-root reading ranger-base-kwbh1 took for the cwd
skills root.

## The declaration

`grokProjectConfig` / `grokProjectConfigKeys` in
`internal/posse/runtime.go`, on the `Name: "grok"` row:

    ProjectConfig: .claude/settings.json, .claude/settings.local.json,
                   .mcp.json, .cursor/mcp.json, .cursor/hooks.json,
                   .grok/config.toml, .grok/hooks, .grok/plugins
    ProjectConfigKeys: hooks, mcpServers

Four things about those two lines that are decisions and not transcription.

**`permissions` is NOT a key**, and it is the one addition that would look like
tightening. Grok loads a repo's `permissions` under trust (the table above: 2 +
1 against 0), and it still does not belong: a permission grant is not an exec
channel, claude's declaration excludes it for that reason, and posse's **own**
`.claude/settings.json` is permissions-only — so adding it would degrade every
crew grok seat in this repo rather than refusing an attack.

**`.mcp.json` IS inside this detector here.** ADR 0002's line that it "retains
its separate runtime approval boundary and is outside this detector" is
**claude's** reading, not a general one: claude keeps that file behind its own
"Pending approval" gate, which reads identically trusted and untrusted
(trust.go's measurement). Grok has no such gate — 1.0.52 spawned the server
under folder trust alone — so the reason for the exclusion is absent and the
file is a channel like the rest.

**The check does not consult grok's folder trust**, though grok's built-in
already carries a probe for it (`GrokSkillsDirTrust`, ADR 0007 §6f). An
untrusted directory does close this channel, but trust resolves through the git
root and every tree posse opens inherits it, so the conservative reading is the
accurate one here too — and ADR 0002 §4 wants a file the launch cannot prove
safe to **degrade**, not a launch that argues about it.

**One key list over mixed file types**, with a misleading reason accepted.
`ProjectConfigKeys` is per runtime and `projectConfigTrustFile` parses every
declared entry as JSON once keys are set, so `.grok/config.toml` degrades as
*invalid JSON* and `.grok/hooks` / `.grok/plugins` degrade as *a directory, not
a regular file*. Both are the **right verdict** — any such file or tree is a
channel; `config.toml` alone contributes `mcp_servers`, permissions and plugins
— reached by a reason that names the wrong thing. Accepted as is (the finding's
own design note). If the message misleads in practice, per-entry keys are a P3
follow-up.

## LAUNCH DIR — confirmed, and where it ends

Grok reads `<project>` as every directory from the git root down to the cwd;
`ProjectConfigTrust` checks **one**. Complete for every launch posse makes
today, read off the code rather than assumed:

- a **dispatched** session's dir is the session tree's root —
  `herdrback.go` sets `dir` from `SessionTree.Path` *before* anything
  path-scoped reads it ("the gates, the hooks, the parity probe, the skills
  render, the seatbelt profile and claude's directory trust are all
  path-scoped") — and a worktree root is that worktree's own git toplevel;
- a **crew** session's dir is the checkout root, which is the toplevel too.

The one shape outside that is a hand-typed `posse new --dir <subdir>`, where a
config file in an ancestor inside the same repo sits above the dir this check
stats. That is the reach **every** runtime's list has, not grok's, and this
declaration neither widens nor narrows it.

## WHY P1 AND NOT P0 — and what reachability means here

Census 2026-10-31: every config file in the six trusted roots carries only
`permissions`. No hooks, no MCP, no `.grok/` or `.cursor/` files. The channel
was open on every grok launch and **empty**.

What makes it reachable: a persona commits `.grok/hooks/x.json`, or a `hooks`
key in `.claude/settings.json`, in its worktree; the launcher fast-forwards it
onto `main` at close; every later grok seat runs it at SessionStart, before the
work prompt and before any PID gate, with the seat's whole env, inside the
cage. After this bead that launch **degrades** and names the file.

Worth saying plainly, because this bead does not close it: the **commit** wall
is narrower than the launch check is now. `ConstitutionClassIn` (promote.go,
rendered into sh by `constitutionGuardBody`) is "the two claude settings files
in every repo, plus the constitution paths in a constitution repo" — so a
persona committing `.claude/settings.json` is refused, and a persona committing
`.grok/hooks/x.json`, `.mcp.json` or `.cursor/hooks.json` is not. Filed, not
fixed here.

## Pins

- `internal/posse/grokprojectconfig_qa_test.go` (arm 2) — five arms: the
  declaration (including the absent `permissions`); a fixture **per entry**
  degrading a grok launch, asserted on the **finding** so a key match is told
  apart from the fail-closed classification; a permissions-only settings file
  staying clean; the PID opt-in and `--allow-degraded` clearing it (and `tier
  fast` not being waivable); and the asymmetry itself — one `hooks` file, which
  claude **and** grok must both degrade on, with codex as the control.
- `internal/treepins/grokprojectconfigclean_qa_test.go` — the same claim over
  **this repo's real files**: posse's own `.claude/settings.json` keeps a clean
  launch on all four built-ins, it is still the shape that makes that a fact
  (readable JSON, `permissions`, neither exec key), and it is still the only
  one of grok's eight entries this checkout carries. Eight paths read **by
  name**, nothing enumerated, so it is outside the tree-wide pin class and
  needs no Makefile door.
- `internal/posse/runtimecheck_test.go` — grok was the grid's "none" arm, which
  is precisely what this bead found. The arm now asserts the eight-entry keyed
  row; the "none" reading is still held, over a template-only yaml, by
  `TestUndeclaredDimensionsAreLoudNotBlank` — a runtime that really has
  nothing.

## Re-verification

A future grok version re-runs the table above, in full, with that version's
`$HOME` fixture. `.cursor/hooks.json` is the row expected to change — the
release that implements the Cursor reply path opens that channel with no edit
to the declaration. `.grok/plugins/` is the row still owed a marker.
