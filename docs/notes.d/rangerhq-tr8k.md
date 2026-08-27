## Runtime preflight: three more declarable keys, and a check with a verdict (rangerhq-tr8k)

ADR 0012 D4's second code track. `runtimes/<name>.yaml` gains three keys,
`posse runtime check <name>` gains a preflight and an exit status, and
authoring the herdr manifest — D4's "hardest requirement" — gains a page.

| key | shape | what it says |
|---|---|---|
| `state_dir:` | one path, or a list; absolute or `~`-prefixed | where this CLI keeps its own state. Joins the L2 seatbelt's writable set |
| `env_required:` | list of NAMES | variables a session here cannot work without. Checked at launch preflight; a missing one **refuses** |
| `interstitial_<name>:` | one-level map: `screen: where: key: silence: danger:` | a first-run screen and the operator-owned key that silences it |

**`state_dir:`.** `~/.claude ~/.claude.json ~/.codex ~/.grok` were a literal
in `SeatbeltWritable`, so a third-party CLI under `cage: seatbelt` got a
READ-ONLY state dir with nothing anywhere saying so — it re-runs its
first-run flow every launch, or dies on a config write. The built-ins now
declare their own dirs (`Runtime.StateDirs`) and the profile builder takes
the union of those plus the launching runtime's. The union, not just the
launching runtime's: that is what the literal granted, and narrowing it is a
separate decision with its own blast radius. A relative path refuses — the
session cwd is already writable, so the only thing a relative path can do is
grant a directory under the tree and leave the real state dir read-only.

**`env_required:`.** The Bedrock shape: claude installed, on PATH,
correctly declared, and every launch a dead pane because `AWS_REGION` was
not in the session env. `planLaunch` checks it once the env sets are
resolved — an operator's own exported name counts, and present-but-empty is
missing. **Names only, and it is enforced**: a `FOO=bar` entry refuses at
load, because a list an operator can put a value in is a list whose contents
end up in a terminal.

**`interstitial_<name>:` — dismissals, never keystrokes.** The bead asked
for `startup_screens: {<herdr rule id>: [keys]}`, replacing a Go map in
`dispatch.go`. That map no longer exists: rangerhq-6723 retired
`startupScreenDismissals` the same week, and hoover's rangerhq-4mzt ruling is
that no drawn dialog is the launcher's to answer. So what became declarable
is the half that survives — the *dismissal*, meaning the file and the key the
operator sets — with no keystroke anywhere in it. A key family rather than a
list because flat-YAML has no list-of-maps; `plan_guard_<window>:` is the same
shape. A declared entry carries **no probe**: posse cannot read an unknown
CLI's config, and a guessed probe answers "not silenced" for a screen the
operator silenced years ago. `Seeded` is likewise not declarable — that is
posse *writing* the operator's config, argued to a standstill in
rangerhq-w4uf.

`Interstitial.Danger` is still documentation: no launch path reads it. The
grid used to promise "LAUNCH REFUSE until silenced", which the code did not
make good on; it now says what actually happens, and the refuse is
`ranger-base-a9y9`.

### `posse runtime check <name>` exits 1 on a blocking gap

The grid still prints whole. Under it, the preflight — each gap **by name**
(`exe`, `detection`, `yaml`, `env_required`, `interstitial`) with the split
between a blocking gap and a named degrade:

- **exe** — argv0 is not on PATH. A launch then opens a pane that prints
  "command not found" and sits at a shell, which herdr reads as a shell.
- **detection** — no herdr manifest for that argv0.
- **yaml** — a key nothing reads. A *launch* warns and proceeds (the file is
  the operator's own config root); `check` does not, because `skils_flag:` is
  the answer an onboarder came for.
- **env_required** — a declared name not set here.
- **interstitial** — a probe that says NOT silenced. Non-blocking today.

Detection is asked through `Herdr.AgentManifest` (`agent explain --file
<empty> --agent <argv0>`), not through the compiled kind list, because
**MEASURED on herdr 0.8.0, 2026-08-27**: a standalone
`~/.config/herdr/agent-detection/<newname>.toml` is ignored outright —
`unknown_agent`, null manifest, not listed by `server agent-manifests` — while
`--agent grok-build`, an alias in our own `grok.toml`, resolves to grok's
manifest and matches its rules. An `aliases = [...]` entry is therefore the
only route a CLI herdr was not built with has to detection at all, and a
check that read only the kind list would tell an operator who aliased
correctly that their CLI is undetectable.

`docs/runbooks/agent-detection-manifest.md` is the authoring page: that
measurement first, then the two routes that work, rule anatomy, the priority
ladder and what not to key on. A herdr that cannot be asked is UNKNOWN — a
non-blocking gap — never a wrong "no".
