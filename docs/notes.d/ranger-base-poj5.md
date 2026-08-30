# codex substrate: pinned at 0.150.1 by the cask, and there is no hard ceiling

*ranger-base-poj5 · devops · 2026-08-30 · a NOTES.md fragment (ADR 0022 §2)*

The operator's instruction on rangerhq-iy3y was: *"when you land the gate,
`required_maximum_version` starts at 0.150.1 — the walked version."* That is
grok's mechanism, and codex does not have it. This is what codex has instead,
what was measured to establish that, and what lifting the pin costs.

## codex carries no version-ceiling key at all

Measured on the installed 0.150.1 binary — a 229 MB Mach-O on which `strings`
emits **nothing**, and on which `grep -c` dies with *"Illegal byte sequence"*
unless `LC_ALL=C` is set. Both traps return a zero that looks like an answer,
so the recipe carries a positive control:

```sh
B=$(readlink -f "$(command -v codex)")
for k in required_maximum_version maximum_version minimum_version \
         auto_update model_reasoning_effort; do
  printf '%-28s %s\n' "$k" "$(LC_ALL=C grep -ac -- "$k" "$B")"
done
```

```
required_maximum_version     0
maximum_version              0
minimum_version              0
auto_update                  0
model_reasoning_effort      27   <- the control: the read DOES reach codex's
                                    config-key strings
```

**Never read a zero here without the control.** An earlier sweep reported all
four as zero *including* the control — that was `strings` returning nothing on
this binary, and it would have "confirmed" the same conclusion for the wrong
reason.

So `required_maximum_version` is a grok/xAI mechanism. There is nothing to set
on codex that refuses to *start*, and no version of
`etc/codex/version-pin.toml` can add one.

**ACCEPTED RISK.** A codex above the pin still starts. Everything below
refuses to *move* the binary; nothing refuses to *run* one that got past the
pin by another route — `npm`, `bun`, `pnpm` and the standalone installer are
each a separate codex update channel, and any of them linked ahead of
`/opt/homebrew/bin/codex` leaves the pin asserting a binary nothing runs.
`scripts/verify-codex-pin.sh` prints this risk on every run, green or red, and
its "codex resolves into the pin" row is what catches the other channels.

## What the pin actually is

**1. The cask.** `brew pin --cask codex`. It refuses to *upgrade*, never to
start, so it is `maximum_version`'s shape, not `required_maximum_version`'s.
Measured on Homebrew 6.0.20 with 0.150.1 installed and 0.151.0 in the tap:

| command | before the pin | after the pin |
|---|---|---|
| `brew upgrade --cask codex -n` | `Would upgrade … 0.150.1 -> 0.151.0` | `Error: Not upgrading 1 pinned package`, **rc=1** |
| `brew upgrade -n` | codex in the upgrade list | `==> 1 Pinned cask` |
| `brew outdated --cask` | `codex (0.150.1) != 0.151.0` | `… [pinned at 0.150.1]` |

That rc=1 is why the cask pin also covers codex's *own* updater. On a brew
install codex's update action **is** `brew upgrade --cask codex` — codex says
so itself:

```sh
codex doctor --json   # .checks["updates.status"].details["update action"]
```

so "1. Update now" now fails loudly instead of rolling the fleet forward.
`brew pin` has a documented hole for casks declaring `auto_updates true`
(they can update themselves outside Homebrew); the codex cask declares no such
stanza, which is what makes the pin load-bearing here.

**2. The affordance.** `check_for_update_on_startup = false`, a **top-level**
key in `~/.codex/config.toml`. Appended after a `[table]` header it becomes
that table's key and does nothing, silently.

The startup menu default-selects `1. Update now`; one Enter on it is how this
fleet last moved a codex version with no decision. This key stops the menu
being drawn at all — permanently, where `dismissed_version` silences exactly
one release. Measured in a four-arm tmux rig, `CODEX_HOME` pointed at a
fixture whose `version.json` says dismissed 0.149.1 / latest 0.151.0, i.e. the
menu is due:

| `~/.codex/config.toml` | pane |
|---|---|
| key absent | `✨ Update available! 0.150.1 -> 0.151.0` |
| `check_for_update_on_startup = true` | the menu |
| `unrelated_bogus_key_xyz = false` | the menu |
| `check_for_update_on_startup = false` | **no menu** — codex goes straight on |

The bogus-key arm is not padding: *"any extra key suppresses it"* and *"this
key suppresses it"* are different claims, and only one of them is true. The
absent-key arm is what shows the rig can fail at all.

`internal/rhq` reads this key ahead of `version.json` for the same reason
(`codexUpdateProbe`): with the startup check off, what `version.json` says is
not a reading about any screen the operator will meet. Before that change,
`dismissed_version` alone refused **every** codex dispatch on this box —
`posse runtime probe codex` → *"cannot be probed: interstitial — the menu is
back"* — because the tap had moved to 0.151.0 against a dismissal of 0.149.1.

**3. A rollback target that still exists.** grok keeps every build it has run
in `~/.grok/downloads/`, so rolling back is `grok update --version <old>`. A
Homebrew cask keeps **one**: the Caskroom holds only the installed version,
homebrew-cask carries no version history, and the 0.150.1 tarball is not in
Homebrew's download cache — that cache is keyed on the *current* cask's URL,
which is 0.151.0's. `brew cleanup` after any upgrade takes the only copy on
the box.

So the rollback target is asserted, not assumed: `verify-codex-pin.sh` fails
if `Caskroom/codex/0.150.1` stops existing. Off-box, upstream still serves the
artifact (measured 2026-08-30: HTTP 200, 113348973 bytes); the URL and
homebrew-cask's own sha256 are in `etc/codex/version-pin.toml`, `[rollback]`.

## Runbook: lifting the pin

Lifting is the operator's. `make verify-codex-pin` prints this list whenever
the tap is ahead of the pin, and stays **green** while it does — the tap
moving is not a failure, it is the gate reporting that a re-audit is due.

1. **Fetch the rollback artifact first, not after.** The moment the upgrade
   lands, `brew cleanup` deletes the only copy of the old version on the box.
2. **Re-audit the dispatch contract** (ADR 0013 §1). Every flag on the codex
   launch line is version-verified, not contractual: `-a never`,
   `--disable hooks`, `-c allow_login_shell=false`, the `projects` trust
   grant, and `-c developer_instructions="$(cat …)"` — which is how the work
   prompt is delivered at all. `posse runtime check codex`.
3. **Re-run the four-arm rig** against the new build.
   `check_for_update_on_startup` exists at 0.150.1; a rename retires the
   affordance kill silently, and the "no menu" reading would then be a build
   that simply had not checked yet.
4. **Re-run `make verify-detection`** against the new build's screens
   (`etc/herdr/agent-detection/codex.toml` and its testdata: `update_menu`,
   `model_picker`, `hooks_review`, `idle_composer`).
5. Then, in one change: `brew unpin --cask codex`, upgrade, `brew pin --cask
   codex` again, and raise `posse_pinned_version`, `caskroom_dir`, `url` and
   `sha256` in `etc/codex/version-pin.toml`. `make verify-codex-pin` is the
   check that the new state is the declared one.

## Rolling back

```sh
brew unpin --cask codex          # the pin refuses a downgrade too
brew reinstall --cask codex      # only reaches the CURRENT cask version
```

There is no `brew install codex@<version>`. The supported route back is the
tarball named in `[rollback]`: verify its sha256, unpack it, and put its
`bin/codex` where the cask's symlink points. That is why step 1 above is step
one.
