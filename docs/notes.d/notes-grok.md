## grok substrate: pinned at 1.0.5, upgrades are a security re-audit (rangerhq-y7jr)

The fleet's grok shipped configured to replace itself. `~/.grok/config.toml`
had `[cli] auto_update = true` with `installer = "internal"`, and
`which grok` resolves to `~/.grok/bin/grok` → a symlink into
`~/.grok/downloads/` — i.e. **the managed install**, which is exactly the one
the updater is allowed to touch (`stdio auto-update skipped: not the managed
install` is the negative case, and it does not apply here). Three binaries are already
stacked in `downloads/` — the June install, 1.0.0 (Aug 8) and 1.0.5 (Aug 18) —
so this has happened before, unattended.

**The vector is the leader, not the launch.** `grok` runs a long-lived leader
process shared by every session (`cli.use_leader`), and the leader updates
itself *mid-life*: `xai-grok-update/src/auto_update.rs` carries
"Leader auto-update: v<x> installed successfully" and "Leader auto-update:
newer binary already on disk, relaunching without download". So "we only
upgrade when someone runs `grok update`" was never true — a fleet that never
restarts anything can still be on a new binary by lunchtime.

**Why that is a security problem and not a version-hygiene problem.** Nearly
everything the fleet knows about grok is version-VERIFIED, not contractual:

- **Coding-data consent** (rangerhq-sz7u). The defense there is that in 1.0.5
  the consent-record RPC `x.ai/consent/record` has **no server handler** —
  the binary says so ("consent record not sent; no server handler yet";
  "failed to persist consent answer; the notice will re-arm next launch") —
  so even an accidental `[Opt in]` on the startup splash cannot persist. That
  defense evaporates the day xAI ships the handler, and the splash is the
  same screen that eats a dispatched Enter (rangerhq-37c). This is the
  operator's data line.
- **Permission-mode precedence** (rangerhq-vjl). CLI
  `--permission-mode` beats `[ui] permission_mode` in config.toml
  (runtime.go:361-369). That precedence is the only thing keeping fleet grok
  launches off this machine's always-approve config fallback.
- The rule dialect, the `--rules="$(cat …)"` `=` form, and the login-shell
  capture L1 rides on — all of "Grok specifics" above.

An unreviewed roll-forward retires all of it silently, which is the actual
loss: not a broken binary, an un-rechecked one.

**The pin, applied 2026-08-22.** Declared in `etc/grok/version-pin.toml`,
live in `~/.grok/config.toml`:

```toml
[cli]
auto_update = false                    # kills the update check and the leader's self-update
maximum_version = "1.0.5"              # soft ceiling: the updater never INSTALLS above this
required_maximum_version = "1.0.5"     # hard ceiling: grok refuses to START above this
```

`maximum_version` is the belt: a soft bound caps the updater's *target* even if
`auto_update` gets flipped back on by `/settings`, a managed-config push, or a
hand edit, and **soft bounds never block startup**, so it cannot strand a pane.
The declaration is not installed over config.toml — that file also holds the
operator's own preferences and grok rewrites it itself (`dismissed_version`,
`marketplace.*`). `make verify-grok-pin` asserts the live config still matches.

- **Verified by execution, not by reading the config**: `grok update --check
  --json` answers `{"currentVersion":"1.0.5","latestVersion":"1.0.5",
  "updateAvailable":false,"autoUpdate":false,…}`. That `autoUpdate` field is
  grok's own answer about what the updater will do, and it is what the verify
  script checks — the config file is only the second opinion.
- **`grok inspect --json` cannot see any of this.** Its keys stop at
  `grokVersion`/`channel`/`permissions`/…; there is no update or version-policy
  section. `update --check --json` is the only cheap probe.
- **A non-TTY launch skips the update check on its own** ("Non-TTY stderr
  (auto-detected)"), which is why you cannot demonstrate the pin by watching
  `version.json`'s `checked_at` from a script — it would not have moved either
  way. Don't use that as evidence.

**The harder gate, set 2026-08-28 (operator ruling, rangerhq-iy3y).**
`required_maximum_version` is a *hard* bound: grok exits at startup above it,
where `maximum_version` alone only refuses to *install* — a gate with a hinge on
it, since a hand-run `grok update`, a managed-config push from xAI, or
`auto_update` flipped back on in `/settings` can all land a binary past a soft
ceiling and it will still run. The ruling is the same philosophy as the bd pin:
an unreviewed upgrade must refuse to start rather than run silently.

Probed live before it was applied, both arms, and the **config key** gates — not
only the `GROK_*` env override:

```
# required_maximum_version = "1.0.4" in config.toml, binary 1.0.5:
$ grok -p "x" -m no-such-model-xyz
This version of Grok (1.0.5) is newer than the maximum allowed by your organization (1.0.4).
Install an approved version through your organization's approved method …

# required_maximum_version = "1.0.5": starts, and fails later on auth/model —
# past the version gate. Lowering only the SOFT ceiling refuses nothing.
```

**Only the agent path is gated**: `--version`, `update`, `inspect`, `doctor` and
`models` all run normally above the ceiling, so an out-of-range install stays
diagnosable and recoverable (`grok update --version 1.0.5` is allowed above a
ceiling by design, and the old binary is still in `downloads/`). So the failure
is loud, self-describing — grok names the ceiling in the refusal — and about two
minutes to undo.

**Know the blast radius before you raise the binary**: the moment grok is out of
range, every dispatched grok pane *and* the operator's own interactive grok stop
starting, at once. That is the gate working. Recovery is
`grok update --version 1.0.5`, or the symlink rollback below.

Bounds resolve across layers by tightening only (floor takes the highest,
ceiling the lowest), the `GROK_*_VERSION` env overrides can only tighten
further, and an invalid value is ignored rather than blocking startup
("ignoring invalid version bound", `config/resolve/version.rs`). Do **not** set
`minimum_version`/`required_minimum_version`: those are anti-downgrade floors
and would block rolling back to a known-good build.

**`--no-auto-update` exists but is hidden.** It is in grok's own docs
(`14-headless-mode.md`) and absent from `grok --help` in 1.0.5; it is
nonetheless accepted (`grok --no-auto-update --version` → 0, while a genuinely
unknown flag errors). It was not added to `GrokFleetFlags`: it is per-session,
so it would leave the *interactive* operator sessions and the shared leader
unpinned — the config pin covers every entry point, including the leader.
`GROK_DISABLE_AUTOUPDATER=1` is the per-process equivalent.

**Operator runbook — lifting the pin (a deliberate upgrade):**
```
make verify-grok-pin                       # prints the re-audit list when upstream has moved
cp -a ~/.grok/config.toml ~/.grok/config.toml.before   # grok rewrites this file itself
cp -a ~/.grok/downloads/grok-1.0.5-macos-aarch64 /tmp/ # THE rollback binary
# --- the security persona re-audits the new build FIRST: consent-record
#     handler (sz7u), --permission-mode precedence (vjl), rule dialect,
#     splash detection ---
grok update --check --json                 # confirm the target
grok update --version <new>                # explicit; allowed above the ceiling
# then raise ALL THREE in etc/grok/version-pin.toml and ~/.grok/config.toml:
#   posse_pinned_version / maximum_version / required_maximum_version
# (raise required_maximum_version FIRST or grok will not start on the new build)
make verify-grok-pin && make verify-detection
```
Rollback is `ln -sfn ../downloads/grok-1.0.5-macos-aarch64 ~/.grok/bin/grok`
(and `bin/agent`) — unlike `herdr update`, grok keeps the old binaries in
`downloads/`, so there is a rollback target without a re-fetch. Keep it that
way: do not prune `downloads/`. `grok du` reports it as the largest thing in the
grok home and is pure temptation — it only reports, and `grok worktree gc` does
not touch it, so nothing prunes those binaries but a person.

