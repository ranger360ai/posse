## grok substrate: pinned at 1.0.52 (Homebrew cask `grok-build`), upgrades are a security re-audit (rangerhq-y7jr)

**What this pin governs: the Homebrew cask `grok-build`.** OPERATOR RULING
2026-10-31 (ranger-base-yq1ar, applied under ranger-base-7qey3) — keep the
cask. `which grok` → `/opt/homebrew/bin/grok` →
`/opt/homebrew/Caskroom/grok-build/1.0.52/grok-1.0.52-macos-aarch64`;
`~/.grok/bin/` and `~/.grok/downloads/` do not exist. The two version ceilings
still live in `~/.grok/config.toml`, because grok reads its policy from
`$GROK_HOME` whatever installed the binary, so the pin has two halves now and
`etc/grok/version-pin.toml` declares both. `make verify-grok-pin` asserts all
of it — the live config, the installed cask, the brew pin, the Caskroom
directory, and which tree the binary on PATH actually answers from.

**The history, which is no longer this box.** The fleet's grok shipped
configured to replace itself. `~/.grok/config.toml` had `[cli] auto_update =
true` with `installer = "internal"`, and `which grok` resolved to
`~/.grok/bin/grok` → a symlink into `~/.grok/downloads/` — i.e. **the managed
install**, which is exactly the one the updater is allowed to touch (`stdio
auto-update skipped: not the managed install` is the negative case, and it did
not apply then). Three binaries were stacked in `downloads/` — the June
install, 1.0.0 (Aug 8) and 1.0.5 (Aug 18) — so this had happened before,
unattended. The install route changed under all of that on 2026-10-12
(`brew install --cask grok-build`), which is what ranger-base-bttb1 found and
what the ruling above settled.

**The vector was the leader, not the launch.** `grok` runs a long-lived leader
process shared by every session (`cli.use_leader`), and the leader updates
itself *mid-life*: `xai-grok-update/src/auto_update.rs` carries "Leader
auto-update: v<x> installed successfully" and "Leader auto-update: newer binary
already on disk, relaunching without download". So "we only upgrade when
someone runs `grok update`" was never true — a fleet that never restarts
anything can still be on a new binary by lunchtime.

**The cask weakens that vector and opens a different one.** Weaker: the binary
skips self-update when it is not running from `$GROK_HOME/bin/grok`, so a
cask-run leader does not replace itself. New: `brew upgrade` does, and brew is
not grok — none of the `[cli]` keys can refuse it. Hence the brew half of the
pin, and hence the one thing to keep in mind about every rollback below: a
re-fetch **recreates the managed install**, which puts the original vector back.

**Why that is a security problem and not a version-hygiene problem.** Nearly
everything the fleet knows about grok is version-VERIFIED, not contractual:

- **Coding-data consent** (rangerhq-sz7u; **void from 1.0.52**,
  ranger-base-ppzty / ranger-base-g4294). The 1.0.5 defense was that the
  consent-record RPC `x.ai/consent/record` had no handler. The binary said so
  ("consent record not sent; ..."), so an accidental `[Opt in]` on the startup
  splash could not persist. **That defense is gone: on 1.0.52 consent is
  server-persisted.** MEASURED 2026-10-13 with `strings -a` over
  `/opt/homebrew/Caskroom/grok-build/1.0.52/grok-1.0.52-macos-aarch64`
  (172,237,072 bytes), re-confirmed on the same binary 2026-10-31. The 1.0.5
  "not sent" message appears 0 times. Present:
  - `"/consent/accept"` with header `x-grok-client-identifier`, plus
    `"consent record rejected"` (2 occurrences). The record goes out over HTTP.
  - `x.ai/privacy/setCodingDataRetention` → `"/privacy/coding-data-retention"`
    (2) with field `codingDataRetentionOptOut` (10), and
    `coding_data_retention_opt_out` in the user profile. This is a second,
    account-level path that sz7u never covered. The user-facing door is
    `/privacy`.
  - `"consent gate not armed; failing open"`. The gate is a remote setting.

  **The next re-audit greps for `/consent/accept` and
  `/privacy/coding-data-retention`. Do not look for the 1.0.5 message again.**
  Whether xAI keeps what it receives is not visible from here. Exercising the
  live RPC to find out was refused (hoover, ranger-base-ppzty), and the
  client-side evidence already decides the question fail-closed. The splash
  is the same screen that eats a dispatched Enter (rangerhq-37c), so on 1.0.52
  splash detection is the **only** layer left; before, there were two.
  **This is a SEAT gate, not the version pin's gate, and that distinction is
  the 2026-10-31 ruling's.** The pin moved to 1.0.52 because 1.0.52 is what is
  installed and a pin's job is to make the installed build a reviewed one.
  Before any PID declares `runtime: grok`: prove that a dispatched Enter cannot
  select `[Opt in]` — `make verify-detection` holding **and** the splash's
  default focus read from a pane capture with no key sent, on an account the
  operator designates. **No PID declares `runtime: grok` today** (checked
  2026-10-31 across `examples/agents/` and the live persona dir), which is why
  moving the pin carried no live exposure either way. **Recommended (the
  operator's call):** set the account to Opt out through `/privacy` before any
  grok seat exists. An accidental opt-in is then a visible change from a known
  value instead of a first answer. Data retained between an opt-in and its
  reversal is not recoverable. This is the operator's data line.
- **Permission-mode precedence** (rangerhq-vjl). CLI `--permission-mode` beats
  `[ui] permission_mode` in config.toml (runtime.go:361-369). That precedence is
  the only thing keeping fleet grok launches off this machine's always-approve
  config fallback. Re-verified on 1.0.52, MEASURED 2026-10-13: `grok -p
  "Create a file named probe.txt…" --permission-mode default --max-turns 3` in
  a scratch dir against a config carrying `permission_mode = "always-approve"`
  returned `"stopReason":"cancelled"` and never created the file — the flag
  governed, not the config.
- The rule dialect, the `--rules="$(cat …)"` `=` form, and the login-shell
  capture L1 rides on — all of "Grok specifics" above. Re-verified on 1.0.52,
  MEASURED 2026-10-13: the separated `--rules "$(cat …)"` form still dies with
  `error: unexpected argument '---…'` at zero model cost; the `=` form still
  parses (proved by pairing it with a bogus `-m no-such-model-xyz`, which fails
  later at "unknown model id"); `--deny "Bash(git push:*)"` under
  `--permission-mode auto` refused `git push origin main` against a throwaway
  bare remote, which had zero refs afterwards; and `command -v git` in the same
  session answered the gate shim, not `/usr/bin/git`.
- The **startup-splash detection override**
  (`etc/herdr/agent-detection/grok.toml`, rangerhq-37c/1xsj). Re-verified on
  1.0.52: `make verify-detection` 15/15, all five grok fixtures against the
  `startup_splash` rule. The installed copy at
  `~/.config/herdr/agent-detection/grok.toml` differs from the checkout in
  **comments only** (the ranger-base-k39dz provenance block) — no rule
  changes, checked 2026-10-31 — so the live layer is the one the fixtures
  prove. Pre-existing drift worth a re-diff some day, not blocking: herdr's
  bundled grok manifest has moved 2026.07.16.2 → 2026.10.05.1 since our fork.

An unreviewed roll-forward retires all of it silently, which is the actual
loss: not a broken binary, an un-rechecked one.

**The pin, applied 2026-08-22, raised to 1.0.52 on 2026-10-31.** Declared in
`etc/grok/version-pin.toml`; the `[cli]` half live in `~/.grok/config.toml`,
the `[install]` half in brew's own state:

```toml
[cli]
auto_update = false                     # kills the update check and the leader's self-update
maximum_version = "1.0.52"              # soft ceiling: the updater never INSTALLS above this
required_maximum_version = "1.0.52"     # hard ceiling: grok refuses to START above this

[install]
route = "homebrew-cask"
formula = "grok-build"                  # `brew pin --cask grok-build`
pin_state = "pinned"
caskroom_dir = "Caskroom/grok-build/1.0.52"
```

`maximum_version` is the belt: a soft bound caps the updater's *target* even if
`auto_update` gets flipped back on by `/settings`, a managed-config push, or a
hand edit, and **soft bounds never block startup**, so it cannot strand a pane.
The declaration is not installed over config.toml — that file also holds the
operator's own preferences and grok rewrites it itself (`dismissed_version`,
`marketplace.*`).

**`brew pin` DOES work on casks here, and the ruling that commissioned this
page said it would not.** That was true of older Homebrew. MEASURED 2026-10-31
on Homebrew 7.0.10/7.0.11: `brew pin --cask grok-build` exits 0, `brew list
--pinned` then names it beside `beads` and `codex`, `brew info --cask
--json=v2` reports `"pinned": true, "pinned_version": "1.0.52"`, `brew outdated
--cask --verbose` prints `grok-build (1.0.52) != 1.0.70 [pinned at 1.0.52]`,
and `brew upgrade --cask --greedy grok-build` answers **`Error: Not upgrading 1
pinned package:`**. It is applied. It is still the *second* gate that is
load-bearing: `brew pin` has a documented hole for casks declaring
`auto_updates true` (they can update themselves outside Homebrew), and while
`grok-build` declares no such stanza at 1.0.52, a cask that gained one would
slip the pin quietly. `required_maximum_version` is what refuses to *run* a
build that got past everything.

**Verified by execution, not by reading the config:**

- `grok update --check --json` answers
  `{"currentVersion":"1.0.52","latestVersion":"1.0.52","updateAvailable":false,
  "installer":"internal","channel":"stable","autoUpdate":false,"error":null}`.
  That `autoUpdate` field is grok's own answer about what the updater will do.
- **`latestVersion` is CLAMPED to `maximum_version`, and that cost this pin its
  re-audit gate for two months.** MEASURED 2026-10-31 (ranger-base-7qey3), one
  scratch `$GROK_HOME`, three runs differing only in that key:

  ```
  (no maximum_version)        -> "latestVersion":"1.0.70","updateAvailable":true
  maximum_version = "1.0.52"  -> "latestVersion":"1.0.52","updateAvailable":false
  maximum_version = "1.0.5"   -> "latestVersion":"1.0.5", "updateAvailable":false
  ```

  The last one is *below* the installed binary. `latestVersion` is what grok
  would install, not what upstream published — and since the pin always sets
  `maximum_version` to the pinned version, it equals the pin exactly when the
  pin is intact. `verify-grok-pin.sh` drove "UPSTREAM MOVED" off that field, so
  the block was unreachable by construction; it only ever fired on this box
  because the live config (1.0.52) and the pin file (1.0.5) disagreed. **The
  script reads upstream from the cask tap now** (`brew info --cask grok-build`,
  which no grok config can clamp) and keeps `latestVersion` as a checked row
  against the soft ceiling — a positive control that the updater obeys it.
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
it, since a hand-run `grok update`, a `brew upgrade`, a managed-config push from
xAI, or `auto_update` flipped back on in `/settings` can all land a binary past a
soft ceiling and it will still run. The ruling is the same philosophy as the bd
pin: an unreviewed upgrade must refuse to start rather than run silently.

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
diagnosable and recoverable (`grok update --version <v>` is allowed above a
ceiling by design). So the failure is loud and self-describing — grok names the
ceiling in the refusal.

**Know the blast radius before you raise the binary**: the moment grok is out of
range, every dispatched grok pane *and* the operator's own interactive grok stop
starting, at once. That is the gate working. **"About two minutes to undo" is
retired** — it assumed a cached binary in `downloads/` and a symlink swap, and
neither exists on the cask route. What the undo actually costs is measured in
*Rollback* below.

Bounds resolve across layers by tightening only (floor takes the highest,
ceiling the lowest), the `GROK_*_VERSION` env overrides can only tighten
further, and an invalid value is ignored rather than blocking startup
("ignoring invalid version bound", `config/resolve/version.rs`). Do **not** set
`minimum_version`/`required_minimum_version`: those are anti-downgrade floors
and would block rolling back to a known-good build.

**`--no-auto-update` exists but is hidden.** It is in grok's own docs
(`14-headless-mode.md`) and absent from `grok --help`; it is nonetheless
accepted (`grok --no-auto-update --version` → 0, while a genuinely unknown flag
errors). It was not added to `GrokFleetFlags`: it is per-session, so it would
leave the *interactive* operator sessions and the shared leader unpinned — the
config pin covers every entry point, including the leader.
`GROK_DISABLE_AUTOUPDATER=1` is the per-process equivalent.

### Operator runbook — lifting the pin (a deliberate upgrade)

```
make verify-grok-pin                       # prints the re-audit list when the TAP has moved
cp -a ~/.grok/config.toml ~/.grok/config.toml.before   # grok rewrites this file itself
# Save the rollback artifact FIRST. A cask keeps exactly one version, so the
# moment the upgrade lands the on-disk copy of the pinned build is gone:
cp -a "$(brew --prefix)/Caskroom/grok-build/"*/grok-*-macos-* /tmp/
shasum -a 256 /tmp/grok-*-macos-*           # must match sha256 in etc/grok/version-pin.toml
# --- the security persona re-audits the new build FIRST: consent-record
#     (sz7u; server-persisted from 1.0.52, so the g4294 SEAT gate: a
#     dispatched Enter provably cannot select [Opt in]), --permission-mode
#     precedence (vjl), rule dialect, splash detection ---
brew unpin --cask grok-build
brew upgrade --cask grok-build              # or `brew install --cask grok-build`
brew pin --cask grok-build                  # put the belt back immediately
# then raise ALL THREE in etc/grok/version-pin.toml and ~/.grok/config.toml:
#   posse_pinned_version / maximum_version / required_maximum_version
# (raise required_maximum_version FIRST, or grok will not start on the new build)
# and in etc/grok/version-pin.toml also: caskroom_dir, url, sha256
make verify-grok-pin && make verify-detection
```

### Rollback — measured, and not what the old runbook promised

The symlink rollback this page used to carry
(`ln -sfn ../downloads/grok-1.0.5-macos-aarch64 ~/.grok/bin/grok`) **cannot run
on this box**: `~/.grok/bin/` and `~/.grok/downloads/` do not exist, and nothing
pruned them — the cache was never created, because the updater that fills
`downloads/` is not what installed this binary. Three routes exist; MEASURED
2026-10-31 (ranger-base-7qey3), in this order of preference.

**1. The saved Caskroom copy, if you took one.** This is why the runbook above
saves it before upgrading, and why `caskroom_dir` is asserted by
`make verify-grok-pin` rather than assumed. Copy the binary back into place and
re-pin. Local, no network, and the sha in the pin file is what you check it
against.

**2. Re-download the release artifact.** x.ai serves superseded releases by
version-templated URL — the same route homebrew-cask itself uses. MEASURED
2026-10-31, HTTP HEAD on each plus one full fetch:

```
https://x.ai/cli/grok-1.0.5-macos-aarch64   -> 200, 134,349,648 bytes
https://x.ai/cli/grok-1.0.52-macos-aarch64  -> 200, 172,237,072 bytes
https://x.ai/cli/grok-1.0.70-macos-aarch64  -> 200, 179,063,008 bytes
```

1.0.52's content-length is exactly the size of the installed Caskroom binary.
The 1.0.5 artifact, fetched fresh that day, is
`3dfa7f04fbb5427a8fbead286591543aaecb478b3a0ab222c4329eca1a3b2f86`. So "the
old artifact may be gone" is no longer the worry it was — but nothing
guarantees x.ai keeps doing this, which is why route 1 exists and why the sha
is recorded in the pin file at every lift.

**`grok update --version <old>` is the convenient spelling of this route, and
it works from a cask-run binary** — the "not the managed install" skip applies
to *auto*-update, not to an explicit one. MEASURED 2026-10-31 against a scratch
`$GROK_HOME`, with the live install untouched:

```
$ GROK_HOME=/tmp/probe grok update --version 1.0.5
Installing Grok 1.0.5 (current: 1.0.52)...
  Downloading grok v1.0.5 (macos-aarch64)...
  ✓ grok v1.0.5 installed successfully!
$ ls /tmp/probe/bin /tmp/probe/downloads
bin/grok -> ../downloads/grok-1.0.5-macos-aarch64
downloads/grok-1.0.5-macos-aarch64   (134,349,648 bytes)
```

**Read the catch before using it.** It does not restore the cask and does not
touch the Caskroom. It fetches into `$GROK_HOME` and **recreates the managed
install** — which is (a) a second install brew does not track, so
`brew list --cask --versions` and `make verify-grok-pin`'s cask rows keep
reporting the cask's version, (b) not what PATH resolves to here, so by itself
it changes nothing about what the fleet runs, and (c) **self-updatable again**,
which is the exact vector the cask route had structurally closed. If you use
it, expect `make verify-grok-pin` to say so: the "grok resolves into the pin"
row goes red the moment `~/.grok/bin/grok` wins on PATH, and the moved-box
block prints "A MANAGED INSTALL EXISTS BESIDE THE CASK" with both cautions.

**3. An old cask version — NOT AVAILABLE.** There is no versioned cask to
install. MEASURED 2026-10-31:

```
$ brew info --cask grok-build@1.0.52
Error: Cask 'grok-build@1.0.52' is unavailable: No Cask with this name exists.
$ ls /opt/homebrew/Library/Taps/homebrew/homebrew-cask
No such file or directory
```

homebrew-cask is not cloned on this box (brew reads the JSON API), so there is
not even local tap history to extract an old cask definition from. Recovering
one would mean fetching the old `.rb` out of homebrew-cask's git history on
GitHub and `brew install --cask` from a local file — which only gets you back to
route 2's artifact by a longer path. Don't plan on it.

**So: do not prune the Caskroom directory**, and take the copy before the
upgrade, not after. `grok du` reports the grok home as the largest thing there
and is pure temptation; it only reports, and `grok worktree gc` does not touch
binaries, so nothing prunes them but a person — and on the cask side it is
`brew cleanup` that does.
