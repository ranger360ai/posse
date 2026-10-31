# Two more grok project-config channels, and the one entry no key can describe (ranger-base-2zkcy)

ranger-base-l12j9 declared grok's repo→box project-config channel. Verifying
it live (ranger-base-xfy5s, same day) found **two paths its list does not
name**, both reaching a seat exactly the way `.grok/hooks` does — a persona
commits the file in its worktree, the launcher fast-forwards it onto `main`,
and every later grok seat in an inherited-trust worktree reads it before any
turn and before any PID gate:

    .grok/lsp.json     LSP server definitions grok SPAWNS under folder trust
    .claude/plugins    grok's documented equivalent of .grok/plugins

Same severity and the same reason as l12j9: P1, an exec channel that is
reachable, contained by the cage, and **empty today** in all six trusted roots
(census 2026-10-31).

## MEASURED 2026-10-31, grok 1.0.52 (0920c8169e27) [stable], darwin 25.6.0

By hoover under ranger-base-xfy5s, on the ranger-base-1g8gf rig: a fixture
`$HOME` whose `trusted_folders.toml` named only the trusted arm, identical repo
content in both arms, a junk value in grok's own API-credential env var, every
`*_PROXY` and lowercase twin at `127.0.0.1:9`, telemetry off. The log shows only
`shell.turn.inference_retry` with `Connection refused` through the tunnel, so
nothing left the box. Every command was `touch <marker>`, and the marker is the
reading.

| entry | trusted | untrusted |
|---|---|---|
| `.grok/lsp.json` | **SPAWNED** in a headless session, no file edited, no model response | nothing |
| `.claude/plugins/<p>/` | folder trust alone: loaded as "project, enabled", **NOT** run | nothing |
| `.claude/plugins/<p>/` | name shadows an **installed + trusted** plugin: hook **RAN**, MCP **SPAWNED** | nothing |
| `.grok/plugins/<p>/` | identical to `.claude/plugins` on every arm above | nothing |

The plugin body was `hooks/hooks.json` + `.mcp.json` + `plugin.json`. A trusted
folder with a name matching no grant loaded it as "project, enabled" and ran
nothing; a project `.grok/config.toml` `[plugins] enabled=[...] paths=[<in-repo
dir>]` **still** did not run it; where the name shadowed an installed and
trusted plugin the repo's copy **replaced the operator's** in grok's own
listing.

**Positive control**, so "nothing ran" is the trust gate and not a broken
fixture: the same plugin shape installed with `grok plugin install <path>
--trust` into the fixture `$HOME` RAN and SPAWNED.

`grok inspect` lists `.grok/lsp.json` as *"LSP Servers ... project"*. Grok's
docs call it project-scoped config and say nothing about trust (*"Having an
lsp.json file is enough for passive diagnostics"*).

**Docs drift, no action:** a hand-placed `~/.grok/plugins/<p>` is documented as
auto-trusted and on 1.0.52 did **not** run.

**NOT measured, noted here so the next reader does not re-derive the list:**
`.grok/agents/*.md` frontmatter `mcpServers`/`hooks` (fire when an agent is
invoked mid-session, not at session start), `.grok/workflows/*.rhai`
(model/user-invoked), `.grok/sandbox.toml` (profiles chosen by `--sandbox`; not
an exec channel).

## Live and latent, which is the difference between the two entries

`.grok/lsp.json` needs **folder trust and nothing else**, which makes it the
most reachable entry on grok's whole list.

Both `plugins/` trees need an installed, trusted plugin **name to shadow**.
This box has none today — no `~/.grok/plugins`, no installed-plugins registry —
so that half is **latent**, and the first `grok plugin install --trust` an
operator runs opens it. Declared now rather than then: the declaration is what
makes the channel visible, and nothing announces that install.

## The key handling, and why the smaller shape was the right one

`grokProjectConfigKeys` is `hooks, mcpServers`, and `projectConfigTrustFile`
parses every declared entry as JSON once keys are set. A `.grok/lsp.json` has
**neither key** — its top level is server NAMES, each carrying the `command` —
so as a plain keyed entry it parses fine, matches nothing, and the launch reads
**CLEAN** over a live channel. That is a *wrong verdict*, not the
right-verdict-wrong-reason l12j9 accepted for `.grok/config.toml` (TOML read by
a JSON classifier, degrading as "invalid JSON").

The finding offered two fixes: the **per-entry keys** l12j9 deferred as P3, or
a narrower whole-file exception for that one entry. The second is what landed,
as `Runtime.ProjectConfigWholeFile` — entries the key narrowing does not reach,
checked on presence, which is the predicate every entry had before claude's
keys narrowed any of them. Why it is the smaller one:

- one field and one clause in `projectConfigTrustFile`, against per-entry key
  lists that would change the field's type, the yaml loader, the grid row, the
  refusal message and every runtime that declares keys;
- it moves this check in the **tightening** direction only. The narrowing is
  the one thing here that LOOSENS a safety check (`project_config_keys:` is
  the one declarable key that does), so an exemption from it can only ever
  degrade more launches, never fewer;
- **built-in only.** It is not declarable from a yaml, so no onboarder can use
  it to argue a channel *out* of the narrowing — the inverse of the direction
  `project_config_keys:` already allows;
- nothing else on the list needs it. `.grok/config.toml` and the three
  directory entries already degrade, by a reason that names the wrong thing
  and a verdict that is right. Per-entry keys remain l12j9's P3, now with this
  correctness case recorded against them.

`.claude/plugins` needs none of that: it is a directory, and it degrades as
*"a directory, not a regular file"* — the same fail-closed arm `.grok/hooks`
and `.grok/plugins` take.

## Pins

- `internal/posse/grokprojectconfig_qa_test.go` (arm 2) — the two entries join
  the per-entry fixture table (arm 2 refuses a declared entry this table does
  not cover, and asserts the table walks the list **in order**, because
  `ProjectConfigTrust` names the FIRST offender); arm 1 spells the declaration
  and the exemption, and refuses an exempt entry that is not on the declared
  list at all. **Arm 6** is `.grok/lsp.json`'s own:
  - the fixture body carries neither key — asserted, not left to the reader,
    since a body that named one would pass through the key match and measure
    nothing;
  - presence alone is the finding, and the launch refuses naming the file;
  - **the falsifier**: the same runtime with `ProjectConfigWholeFile` set to
    nil reads the same directory CLEAN. That is the state before this bead, and
    it is the only arm that notices the exemption going empty while everything
    else passes by some other route;
  - the exemption is per ENTRY: the same body at a keyed entry (`.mcp.json`)
    stays clean, so nothing has quietly turned grok's declaration back into a
    whole-file predicate;
  - the grid prints the exception beside the keys it does not reach.
- `internal/posse/runtimecheck_test.go` — grok's `project_cfg` row must name
  the entry the narrowing does not reach. An exemption an onboarder cannot read
  off `posse runtime check` is a check nobody can account for (ADR 0017 §2).
- `internal/posse/runtimefields_qa_test.go` (arm 3) — the ADR 0017 §3 field
  audit: `ProjectConfigWholeFile` is **consumed**, by `parity.go` and
  `runtimecheck.go`.
- `internal/treepins/grokprojectconfigclean_qa_test.go` — the same claim over
  this repo's real files, now over the longer list. `.grok/lsp.json` is the
  entry that would bite hardest if it appeared here: exempt from the narrowing,
  so its mere presence would degrade every grok seat in this checkout.

MUTATION-CHECKED. Emptying `grokProjectConfigWholeFile` reds arm 1, arm 6 and
arm 2's `grok-lsp-json` subtest plus the grid pin; dropping `.claude/plugins`
from the list reds arm 1 and arm 2's totality check.

## Re-verification

The same rule l12j9 set: **on any grok bump, re-run the table** — this one and
l12j9's, in full, with that version's `$HOME` fixture. Two rows are expected to
move. `.cursor/hooks.json` is l12j9's dormant one. The two `plugins/` rows are
this bead's: their reading changes the day an operator runs `grok plugin
install --trust`, and that is a change to the BOX and not to grok, so no
version bump announces it.

Fix verification comes back to the security lane (`-l security`,
ranger-base-xfy5s's lane).
