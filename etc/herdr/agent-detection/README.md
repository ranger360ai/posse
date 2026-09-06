# herdr agent-detection overrides

herdr decides whether a pane is `idle`, `working`, or `blocked` from a
per-agent TOML manifest. Manifests come from three places, in order:

| source | path | notes |
|---|---|---|
| local override | `~/.config/herdr/agent-detection/<agent>.toml` | wins over both; survives `herdr update` |
| remote | `~/.local/state/herdr/agent-detection/remote/<agent>.toml` | fetched from herdr.dev in the background |
| bundled | inside the herdr binary | used when the cached remote is older |

herdr picks the **highest version**, not the nearest source — so "remote"
is not automatically the live one. On this box the cached remote `grok`
manifest is `2026.07.16.1` and herdr ignores it in favour of the bundled
`2026.07.16.2`. Check before you fork anything; `source_kind` in
`herdr server agent-manifests --json` is the answer, and a `warning` field
says so out loud.

`reload-agent-manifests` is **per server**. Each `herdr --session <name>
server` holds its own cache, so `make install-detection` (which talks to
the default socket) does not reach a scratch server — re-run the reload
with `HERDR_SOCKET_PATH` pointed at it.

`herdr server agent-manifests --json` reports which one answered.
`herdr server reload-agent-manifests` re-reads the override directory.

Authoring a manifest for a CLI herdr has never heard of — as opposed to
forking one of these — is `docs/runbooks/agent-detection-manifest.md`. Read
its first section before you start: a standalone manifest for a new agent id
is ignored outright on herdr 0.8.0 (measured, rangerhq-tr8k).

## grok.toml (rangerhq-37c, re-decided in rangerhq-1xsj)

A fresh `grok` pane opens on a **startup screen**, not a bare composer: the
New worktree / Resume session / Quit menu, a `<version> is here!` changelog
line, and the "Help improve Grok" consent banner. Upstream matches no rule
there and falls through to `default_known_agent_idle_fallback`.

Our override adds one rule, `startup_splash`, which reports that screen
**`idle`** — the same answer the fallback gives, but as a named,
fixture-pinned rule instead of a guess, so `verify-detection` fails the day
grok turns the splash into a real modal.

It reported `blocked` from 37c until rangerhq-1xsj, on the belief that the
splash holds the keyboard. **It does not.** Measured on live grok 1.0.5
panes, no Enter ever sent:

| probe | result |
|---|---|
| type text at an untouched splash | lands in the composer at once, and the **first keystroke undraws the menu, changelog line and version footer** |
| press Enter after it (rangerhq-7sbo) | the turn submits: idle → working → done |
| press Esc | focus moves to the composer; nothing is undrawn |
| grok's own OSC title, splash up | `grok` — its **idle** signal, the whole time |

So the screen is decoration over a live composer, `blocked` contradicted the
agent's own self-report, and it was a blocked state that no key posse is
willing to press could ever clear — which is not what blocked means.

### What the blocker never covered

The prompt loss behind 37c and rangerhq-5on is real and reproduces. It is
the **boot race**, not this screen. Timeline of a dispatch-shaped launch
(`pane run grok …`, work prompt sent at t=0, sampled continuously):

```
0.05s  text sent
0.10s  echoed on the SHELL's prompt line — grok has not exec'd
0.20s  herdr: agent=grok, state=idle, rule=none
       (default_known_agent_idle_fallback, over a shell prompt)
0.39s  grok clears the screen, OSC title -> "grok"; text swallowed into its
       input buffer, composer empty
0.80s  the splash is finally drawn        <- the only window this rule sees
0.86s  buffered text drains into the composer, splash undraws
```

`dispatch`'s settle wait (`AgentWait` on `idle|done|blocked`) is satisfied at
**0.20s**, 0.6s before the splash exists. So `blocked` on this rule never
fired in the launch path at all, and could not have: the screen at the
dangerous moment belongs to the shell, and no rule of ours can match it.
That hole was rangerhq-3hb5. It is closed in the launcher's readiness gate
(`awaitSettled`, `internal/posse/dispatch.go`), and that fix leans on this rule
existing:
`agent explain` distinguishes a **seen** idle (`visible_idle: true`,
`matched_rule` set) from herdr's **guess** (`visible_idle: false`,
`fallback_reason: default_known_agent_idle_fallback`), so a gate that
demands a seen idle accepts the splash — whose composer does take the
prompt — and rejects the 0.20s window. Delete this rule and that fix loses
its footing on grok. Retiring the now-unreachable launcher special case is
rangerhq-6723.

### Priority 1105, not 1250

An idle rule must lose to every blocker. Left at 37c's 1250 this rule would
outrank `option_dialog_blocked` (1200) and the permission rules
(1190/1180) — verified: the real splash capture with a
`┃ 1 (●) Yes, proceed` dialog appended reads **`idle`/startup_splash** at
1250 and **`blocked`/option_dialog_blocked** at 1105. A permission dialog's
footer matches none of the rule's `not` guards, so the guards would not have
saved it. 1105 also sits just above `osc_title_idle` (1100), so a text-only
fixture and a live pane (which has the OSC title) give the same answer.

### The anchors

Keyed on the startup **menu** plus `Grok Build <version>`, with `not` guards
on the live-composer hint footers. At narrow widths Grok Build is a footer;
at production width it sits inside the boxed logo after braille art. The
secondary regex is deliberately not line-anchored so both layouts resolve to
the same named rule (ranger-base-z6n).

The footer's `[channel]` tag is **optional**: the same box renders
`Grok Build  1.0.5 [stable]` on one pane and a bare `Grok Build  1.0.5` on
the next — 7sbo saw the bare form, 1xsj saw the tagged form twice the next
day, so the tag is a **race** (grok resolves its channel asynchronously),
not a per-machine variant. Requiring it drops the rule at random. Both
variants are pinned: `testdata/grok/idle-startup-splash{,-plain-footer}.txt`.
A third, live 2026-08-25 capture
(`idle-startup-splash-no-consent-banner.txt`) has the tagged footer, extra
changelog/tip lines, and **no** consent banner — the banner is not always
drawn; the menu + Grok Build anchor still must match. The production-width
boxed capture is `testdata/grok/idle-startup-splash-wide-boxed.txt`, pinned
twice: `internal/posse/splashwide_qa_test.go` and `verify-detection` both run
it against the manifest in *this checkout* — a committed fix proves itself
before deployment. verify-detection used to explain against the *installed*
manifest, which made it unable to fail a committed change at all
(ranger-base-53w1); the install arm ranger-base-neyn wanted — an override that
drifts back below `2026.07.16.105` while production misses the wide splash —
is now a byte comparison against the checkout, reported on every run and
*failed* by `make install-detection`'s own `--check-install` run.

`verify-detection` requires those splash fixtures to resolve to rule id
`startup_splash`, not just state `idle`. After rangerhq-1xsj the state is
the same as herdr's fallback, so a state-only check is vacuous: deleting
the rule still reads idle (rangerhq-uglc).

**Deliberately not keyed on the consent banner.** That banner survives into
a live, prompt-accepting composer; keying on it would strand every grok pane
forever. `testdata/grok/idle-composer-with-consent-banner.txt` pins it.
The live splash above pins the other direction: a splash with no banner.

`startupScreenDismissals` in `internal/posse/dispatch.go` pressed Esc at this
rule id when a pane reported blocked. **Retired in rangerhq-6723**: the state
change above made that branch unreachable, and dispatch now answers *no*
blocked screen at all — a blocker is the operator's, always (rangerhq-4mzt).
The id is still load-bearing for `verify-detection` (above), which pins the
splash fixtures to this rule id and not merely to state `idle`; it is no
longer load-bearing for dispatch.

Do not send **Enter** to an untouched grok splash: `[Opt in]` for
coding-data retention is on that screen — rangerhq-sz7u. The Enter measured
above was pressed with text already in the composer and the footer reading
`Enter:send`, which is a different state from the menu. `Esc` is the safe
key.

### Re-checking a bundled fork

`grok.toml` is forked from the manifest **bundled inside the herdr
binary**, because that is what was live (see the precedence note above).
A bundled manifest is invisible once our override shadows it, so
`verify-detection` watches the *herdr binary version* recorded as
`# posse_bundled_from_herdr` and warns when herdr moves. To re-derive
the base:

```sh
off=$(grep -abo 'id = "grok"' "$(command -v herdr)" | head -1 | cut -d: -f1)
dd if="$(command -v herdr)" bs=1 skip="$off" count=6000 2>/dev/null > /tmp/grok-bundled.txt
# trim at the next `id = "<agent>"`, then diff against etc/herdr/agent-detection/grok.toml
```

Use `dd`, not `strings` — `strings` mangles the box-drawing and braille
characters the rules match on.

## codex.toml (rangerhq-7ia)

Stock herdr reports codex's startup **"Hooks need review"** dialog as `idle`:
no rule matches and detection falls through to
`default_known_agent_idle_fallback`. That is the dangerous direction — a prompt
dispatched to an "idle" pane is typed *into the dialog* instead of the composer,
and nothing errors. The same hole covers every codex modal whose footer reads
`Press enter to confirm or esc to go back` (`/model`, `/approvals`, …); upstream
only matches the `esc to cancel` wording of the same footer.

The same fallthrough hides a second, more expensive screen: codex's
**"Update available!"** release menu (rangerhq-9py0), drawn on a version delta
before the composer exists and footed `Press enter to continue` — a footer
neither the upstream rules nor `live_strong_blocker` match, over a pane whose
OSC title is blank while the menu is up, so `osc_title_idle` does not fire
either. Its default-selected option is `1. Update now`, which runs a package
upgrade of the operator's pinned tooling.

Our override forks upstream `2026.08.09.1` and adds exactly five things — see
the diff against `~/.local/state/herdr/agent-detection/remote/codex.toml`:

- **`hooks_review`** (priority 960) matches the dialog's own text in the top
  region, mirroring `trust_directory`. Keyed on the dialog rather than the
  shared footer so a footer reword cannot silently turn it back into `idle`.
- **`live_strong_blocker`** gains the `esc to go back` footer wording, which
  generalises to codex modals we have not met yet.
- **`update_menu`** (priority 940) matches the release menu on its banner plus
  its numbered `1. Update now` option — not on the footer, and not on the
  parenthetical naming the package manager, which differs per install method.
  `blocked` is measured, not assumed: text sent to the untouched menu is
  **discarded**, where grok's `startup_splash` buffers it into the composer
  beneath (rangerhq-1xsj). Nothing may answer this screen — see
  `Interstitial.Danger` in `internal/posse/interstitial.go`.
- **`signin_menu`** (priority 930) and **`signin_api_key`** (920) name the two
  screens a codex with no usable credentials draws instead of a composer
  (ranger-base-n6s2u). Same fallthrough, same `Press enter to continue`
  footer; the measurement that earns `blocked`, the region depth and what a
  dispatched seat does about it are in "The sign-in screens" below.

This does not change the fleet's posture. `posse`'s codex template still passes
`--disable hooks` (`internal/posse.CodexFleetFlags`) because the cage is ours, not
the runtime's plugins'. The override is what protects **operator-started** codex
panes, which get no such flag.

### Re-audited against codex 0.153.4 (ranger-base-femsg, 2026-09-06)

`make verify-detection` replays *recorded* fixtures: it proves the rules still
parse and still decide, never that a new build still draws the screens they
were cut from. The fixtures above were captured on 0.147.0. When the fleet pin
moved to 0.153.4 each screen was re-captured live — one fresh `CODEX_HOME` per
arm, `tmux capture-pane`, then `herdr agent explain --file` with this
checkout's manifests staged into a throwaway `XDG_CONFIG_HOME`, exactly what
`verify-detection.sh` does — and all five resolved to the same rules:
`update_menu`, `trust_directory`, `hooks_review`, `live_strong_blocker` (the
`/model` picker, whose model list has changed entirely and whose footer has
not) and the idle composer.

`trust_directory` had no fixture at all, so its live 0.153.4 capture is now
`testdata/codex/blocked-trust-directory.txt` (path elided). It measures the
rule: deleting the rule from `codex.toml` turns that fixture from
`blocked`/`trust_directory` into `idle`/`none`.

**The sixth screen had no rule.** A codex whose credentials are missing or
expired draws a **sign-in** menu at startup — `1. Sign in with ChatGPT` /
`2. Sign in with Device Code` / `3. Provide your own API key`, footed
`Press enter to continue`, with no composer beneath it — and `herdr agent
explain` read it **`idle`, rule `none`**. It was the same fallthrough as the
two screens this override was written for, and it was not a 0.153.4
regression: nobody had captured that screen before. It was filed separately
and closed by the next section, which is where the rangerhq-1xsj
discard-vs-buffer measurement it was waiting on lives — `blocked` has to be
earned, not assumed.

### The sign-in screens (ranger-base-n6s2u, 2026-09-06)

Two rules, `signin_menu` (priority 930) and `signin_api_key` (920), and three
fixtures. Both screens belong to one flow: a codex with no usable credentials
never reaches a composer at all, it lands on the sign-in menu, and the menu's
`3. Provide your own API key` opens an API-key field. Before these rules both
read `idle` / rule `none` / `default_known_agent_idle_fallback`.

**`blocked` is measured, three ways.** codex-cli 0.153.4, a fresh empty
`CODEX_HOME`, tmux panes at 120 / 80 / 60 / 40 columns, no Enter ever sent:

| probe | result |
|---|---|
| type text at the untouched menu | **discarded** — no echo anywhere, and the API-key field reached afterwards came up holding its placeholder, not the text |
| the same `send-keys` into that field | echoes at once — the control that says the row above is a measurement, not a blind rig |
| press Esc at the menu | nothing; the menu stays. Esc out of the API-key field returns *to* the menu |
| the same launch with credentials present | reaches `Ask Codex to do anything` immediately — so there is no composer under the menu, there is a menu instead of one |
| press a bare digit | **activates that option, with no Enter** — `3` jumped straight to the API-key field |

So this is not grok's `startup_splash`, which buffers a prompt into the live
composer beneath and is therefore honestly `idle` (rangerhq-1xsj). There is
nothing beneath. And the last row is the sharp end: a dispatched prompt
carrying a `1` would start the ChatGPT browser sign-in flow, a `2` a
device-code request, and in the API-key field Enter would store the prompt
text as a credential, in the file the screen itself names.

**What a dispatched seat should do: refuse, by name.** Nothing may answer
these screens — signing in is identity and money (crew guardrail 1), and ADR
0013 §2 makes an interstitial whose default action mutates the machine a
launch refuse rather than something to type through. Both ladders in
`internal/posse/dispatch.go` already do the right thing once the state is
right — read there, not run, since a live dispatch rig needs a scratch herdr
server:

- **argv, which is codex's path today.** `awaitDelivered` returns the moment
  detection is *seen* rather than burning the whole startup wait on a guess
  and printing "herdr never recognized a screen there"; `gather`'s wait then
  comes back `blocked` and the pass prints `⛔ <bead> blocked in <session> —
  intervene (posse attach <session>)`, claim kept, nothing pressed.
- **typed.** `awaitSettled` asks for `idle|done|blocked` and hands a blocker
  straight back for the caller to refuse by name, which is `awaitAgent` dying
  `never settled idle (status "blocked")` instead of typing into the menu.

The working/blocked session guard also stops reading the pane as a free one.
The pre-launch refuse is also declared now (`CodexInterstitials`,
`internal/posse/interstitial.go`, ranger-base-d1r4x): a probe over
`codexHome()` reads whether codex's own credential file is present (ADR
0019 — presence only, never the value), which lets `posse runtime check
codex` say a box's codex cannot authenticate *before* it is dispatched onto
the sign-in menu, rather than only after. The answer either way is an
operator signing that box's codex in; a persona files it and stops.

**Keyed on the numbered options, never the footer.** `Press enter to continue`
is `update_menu`'s footer too, and no upstream rule matches it — a reword must
not be able to return this screen to `idle`, which is why `hooks_review` is
keyed on its own text as well. `signin_api_key` is keyed on its heading plus
the field's own words; its `Press enter to save` / `Press esc to go back`
footer is two lines, so `live_strong_blocker`'s single-line
`press enter to confirm or esc to go back` never reached it.

**`region = "top_non_empty_lines(24)"`, not the siblings' 20**, and the number
is measured rather than picked: codex draws this menu under a 15-line ASCII
logo, so `2. Sign in with Device Code` is the 21st non-empty line on a
120-column pane and the 23rd on a 60-column one, where the prose above it
wraps. `testdata/codex/blocked-signin-narrow.txt` is that 60-column pane and
is committed for exactly this — at region 22 it fails while the wide capture
still passes. At 40 columns codex drops the logo and the whole screen is 15
lines. `internal/posse/codexsignin_qa_test.go` mutates the region at that
edge, cuts each `all` clause with a positive witness, and holds the inversion
every rule here needs: delete `signin_menu` (or `signin_api_key`) and its
fixtures go `blocked` → `idle`/`none`.

**Not captured, deliberately:** the device-code screen behind option 2. It
requests a real device code from OpenAI, and no reading is worth a network
round trip nobody asked for. It is the one screen of this flow with no rule
and no fixture.

## Working on this

```sh
make install-detection    # install + reload + verify
make verify-detection     # verify only
```

`scripts/verify-detection.sh` replays the real pane snapshots in
`testdata/<agent>/` through `herdr agent explain --file`. Every `*.toml` in
this directory is picked up automatically; filenames encode the expected
state (`blocked-…`, `idle-…`). Capture new ones with `herdr pane read
<pane> --source detection`.

**It explains against the manifests in this checkout, not the installed
ones.** The script copies `*.toml` into a throwaway `XDG_CONFIG_HOME` and
points herdr at that, and fails any fixture whose answer came from another
file — so no install is needed to run it, and a rule you delete here fails
here. Before ranger-base-53w1 it explained against
`~/.config/herdr/agent-detection`, so the fixtures reported OK for anyone who
had ever run `make install-detection`; the install then being copied over
first hid it completely.

The installed copy is *reported*, not verified: each agent gets an
`install: matches the checkout` / `differs …` line, and the exit code stays
the tree's. `--check-install` turns a mismatch into a failure — it is only
honest immediately after installing, which is where `make install-detection`
wires it. Both directions are pinned in
`internal/posse/verifydetection_qa_test.go`.

Snapshots are **text only**, so they cannot carry the OSC title/progress
regions. Rules keyed on those (`osc_progress_working`, `osc_title_*`)
cannot be pinned this way and have to be checked against a live pane —
which is why there is no `working-…` grok fixture. Check those with
`herdr agent explain <pane>` during a real turn.

## Retiring this override

Overrides shadow upstream **permanently and silently** — that is the hazard.
`verify-detection` reads `# posse_forked_from` out of each `<agent>.toml`
and warns whenever the cached remote manifest has moved past our fork point;
for a fork taken from a *bundled* manifest it also warns when the herdr binary
itself moves past `# posse_bundled_from_herdr`. When either fires, for that
agent:

1. Save the current upstream: the cached remote
   (`~/.local/state/herdr/agent-detection/remote/<agent>.toml`) **or**, if the
   bundled one is the live source, re-extract it with the `dd` recipe above.
2. Replace `etc/herdr/agent-detection/<agent>.toml` with the upstream copy
   saved in step 1 — that is the copy verify-detection stages — and run
   `make verify-detection`. (Moving the file aside instead removes it from
   the run: the script builds its agent list from the `*.toml` files in
   `etc/`, so an absent manifest means no manifest to stage and no fixtures
   replayed against anything, upstream included — it now fails outright,
   flagged as a missing manifest for an agent whose fixtures still exist,
   rather than passing vacuously as it once did (ranger-base-j66o). Neither
   outcome answers the question this step asks.)
3. If the fixtures still pass **against that upstream copy**, upstream fixed
   it — delete `etc/herdr/agent-detection/<agent>.toml` and
   `~/.config/herdr/agent-detection/<agent>.toml`. Drop the make targets only
   once the last override is gone.
4. If not, re-fork the new upstream version and re-apply our rules.

`upstream-report.md` in this directory is the write-up to send to herdr if the
gap is still open; it has not been filed — filing it is the operator's call.
