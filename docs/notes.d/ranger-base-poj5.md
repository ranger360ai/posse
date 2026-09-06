# codex substrate: pinned at 0.153.4 by the cask, and there is no hard ceiling

*ranger-base-poj5 · devops · 2026-08-30, pin moved 0.150.1 -> 0.153.4 2026-09-06
(ranger-base-femsg) · a NOTES.md fragment (ADR 0022 §2)*

The operator's instruction on rangerhq-iy3y was: *"when you land the gate,
`required_maximum_version` starts at 0.150.1 — the walked version."* That is
grok's mechanism, and codex does not have it. This is what codex has instead,
what was measured to establish that, and what lifting the pin costs.

## codex carries no version-ceiling key at all

Measured first on the installed 0.150.1 binary — a 229 MB Mach-O on which `strings`
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

Re-run on 0.153.4 (2026-09-06, ranger-base-femsg) when the pin moved: the same
four zeros, control **28**, and the four keys the dispatch contract rests on
all present in the same read — `check_for_update_on_startup` 15,
`allow_login_shell` 16, `developer_instructions` 56, `trust_level` 19. One
thing did change, and it is about the *tool*, not about codex: `strings` emits
nothing on 0.150.1 and **124,257 lines** on 0.153.4 (a 210 MB Mach-O). A
`strings`-based sweep would have been silently blind on the older binary and
would happen to work on this one. Keep the control whichever tool you reach
for; it, not the tool, is what separates the two runs.

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

## The pin moved to 0.153.4 (2026-09-06, ranger-base-femsg)

Not lifted on schedule — **overtaken**. `brew pin --cask codex` was not
applied when this note was written, and on 2026-09-05 at 11:13:50 the cask
upgraded the box to 0.153.4 against a declaration still saying 0.150.1
(ranger-base-k4lza). Worse, the checker did not say so: its tap row read
`brew outdated`, which is silent about a cask that is not *behind* the tap —
see the last section. The operator was given the two honest options on
ranger-base-ydwn1 and ruled **B**: re-audit 0.153.4 and move the pin. 0.150.1
was **not** fetched first, so it now exists only at its own upstream URL,
recorded in ranger-base-ydwn1 and in this file's git history.

Everything in the runbook below was re-run against 0.153.4 before the
declaration moved. Each has a failing arm, because a re-audit whose every
reading is "fine" has not been shown able to say anything else:

| item | reading at 0.153.4 | the arm that fails |
|---|---|---|
| `-a never` | accepted | `-a nevr` → *invalid value … [possible values: on-request, never]* |
| `--disable hooks` | `hooks` flips `stable true` → `false`, and the "Hooks need review" modal stops drawing over an untrusted `hooks.json` | `--disable bogus_feature_xyz` → *Unknown feature flag*; and the same launch **without** `--disable` draws the modal |
| `-c allow_login_shell=false` | recognised by the config schema | `bogus_key_xyz` → *unknown configuration field* |
| `projects.<path>.trust_level="trusted"` | recognised, and the launch reaches the composer with no trust dialog | `trust_lvl` → *unknown configuration field*; `"bogusvalue"` → *unknown variant, expected `trusted` or `untrusted`*; and the same launch without the flag draws the dialog |
| `-c developer_instructions=…` | the text appears in `codex debug prompt-input` | `developer_instructionsXX` → the text appears nowhere |
| `check_for_update_on_startup` | four-arm rig reproduces exactly (below), and the schema accepts the name | the bogus-key arm still draws the menu; a bogus field is rejected by name |
| interstitial detection | five live 0.153.4 screens, each matched by its rule | see below — one screen matched **nothing** |
| rollback artifact | `Caskroom/codex/0.153.4` on disk; the cached tarball's sha256 computed on this box **equals** the cask's declared `35438da1…`; the URL serves HTTP 200 at the same 111,554,884 bytes | — |

The carrier for the schema readings is `codex --strict-config exec`. Pick it
deliberately: `--strict-config` is a **global** flag that most subcommands
*refuse* (`Error: --strict-config is not supported for codex mcp`), and under
zsh an unquoted `$var` holding `"mcp list"` stays one word, so a loop can hand
codex an unknown subcommand, watch config loading fail first, and read that as
the subcommand supporting the flag. It does not. Quote the arguments and check
the bogus arm is rejected by the *same invocation* you are about to trust.

**One finding, and it is not about 0.153.4.** The five screens above are
`update_menu`, `trust_directory`, `hooks_review`, the `/model` picker and the
idle composer; all five still resolve the way the fixtures say. The sixth, the
**sign-in screen** a codex with no usable credentials draws at startup, is a
modal footed *"Press enter to continue"* with no composer beneath it — and
`herdr agent explain` reads it **`idle`, rule `none`**: no rule matches and
detection falls through to `default_known_agent_idle_fallback`. That is the
exact class this override exists to close. It is not a 0.153.4 regression —
nobody had ever captured that screen — and it is filed separately.

**The security read of 0.150.1 → 0.153.4** (eight releases, 2026-08-29 to
2026-09-04) is on ranger-base-femsg. Nothing in it weakens a lever here;
`--disable hooks` was re-measured behaviourally because two of the changes
(#41435, #42110) move bundled cleanup hooks toward being built-ins.

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
3. **Re-run the four-arm rig** against the new build, *and read the key off
   codex's own config schema*. A rename retires the affordance kill silently,
   and the rig alone cannot catch it: a renamed key and a working one both
   give "no menu", because a key codex does not recognise is ignored without
   a word. `codex --strict-config exec` rejects an unrecognised field by
   name — that is the reader, with a bogus key as its wrong arm.
4. **Re-run `make verify-detection`, and then go past it.** That target
   replays *recorded* fixtures against the tree's manifest — it says the rules
   still parse and still decide, not that the new build still draws those
   screens. Capture the new build's screens and run each through
   `herdr agent explain --file <capture> --agent codex` with the checkout's
   manifests staged into a throwaway `XDG_CONFIG_HOME` (what
   `scripts/verify-detection.sh` does), then refresh the fixtures in
   `etc/herdr/agent-detection/testdata/codex/` if the shapes moved.
5. Then, in one change: `brew unpin --cask codex`, upgrade, `brew pin --cask
   codex` again, and raise `posse_pinned_version`, `caskroom_dir`, `url` and
   `sha256` in `etc/codex/version-pin.toml`. `make verify-codex-pin` is the
   check that the new state is the declared one — and `cpPinnedVer` in
   `codexpin_qa_test.go` moves with it, together with `cpPastVer`, which has
   to stay **above** the pin or every "UPSTREAM MOVED" arm in that file turns
   into a silent no-op instead of a failure.

## Rolling back

```sh
brew unpin --cask codex          # the pin refuses a downgrade too
brew reinstall --cask codex      # only reaches the CURRENT cask version
```

There is no `brew install codex@<version>`. The supported route back is the
tarball named in `[rollback]`: verify its sha256, unpack it, and put its
`bin/codex` where the cask's symlink points. That is why step 1 above is step
one.

## The tap row reads `brew info`, not `brew outdated` (ranger-base-k4lza)

One sentence above — "`make verify-codex-pin` prints this list whenever the
tap is ahead of the pin" — was not true in the case that matters most, and
the mechanism is worth stating next to the runbook that relies on it.

The tap row used to read `brew outdated --cask --verbose`, which is silent
about a cask whose installed version is not *behind* the tap. An upgrade past
the pin makes installed *equal* the tap, so exactly when the pin has already
been lost, `brew outdated` names nothing — and the row's
`|| upstream=$want_ver` fallback then filled in the pin's own version. The run
printed the pin as the tap and `== the pin; nothing to re-audit`, suppressing
the whole re-audit list at the moment it was due. Same shape as the forked
matchers of ranger-base-s8b4g: a reader that answered nothing was
indistinguishable from "nothing to re-audit", with a fallback in place of the
fork.

The row now reads `brew info --cask`, which names the tap version whatever is
installed, and there is nothing left to fall back to: a non-zero brew and a
header with no version in it both fail the row. The header is parsed as its
last digit-initial token, which is the tap version in all three shapes brew
writes (`: 0.153.4`, `: 1.15.0 → 1.16.2`, and either with a trailing
`(auto_updates)`). Pinned by `TestQACodexPinTapReadWhenTheBoxIsAlreadyPastThePin`
and `TestQACodexPinUnreadableTapFailsTheRow`.

Two limits of the other rows, while they are being written down:

- `brew pin` pins whatever is *installed*. The `brew cask pin` row asserts
  that the cask is pinned, not which version it holds; the `codex --version`
  row is what carries a mismatch, so read the two together.
- The `config check_for_update` row reads
  `${CODEX_HOME:-$HOME/.codex}/config.toml`, so it measures whichever home
  the *runner* has. Run from a seat with its own home it reports that seat,
  not the box.

The state of this box under all of the above, and which version the fleet
should pin, are in the beads: ranger-base-k4lza and ranger-base-ydwn1.
