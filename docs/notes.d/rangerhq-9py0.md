## codex's update menu reads `blocked` now (rangerhq-9py0)

The bead asked one question first — does herdr report codex's "Update
available!" menu as `blocked`, or fall through to idle — because the answer
decides whether this is a detection fix or a launch-line fix. **Measured: it
fell through.** `state: idle, rule: none, fallback_reason:
default_known_agent_idle_fallback`, live on codex-cli 0.147.0 with a version
delta armed in an isolated `CODEX_HOME`. Two reasons at once: the menu's footer
is `Press enter to continue`, which no rule in `etc/herdr/agent-detection/codex.toml`
matched, and codex leaves the OSC title blank (`"  "`) while the menu is up, so
`osc_title_idle`'s `\S` did not fire either.

So: a detection fix. `update_menu` (priority 940) names the screen `blocked`,
keyed on the banner plus the numbered `1. Update now` option — not on the
footer, and not on the parenthetical naming the package manager, which is
install-method dependent (`brew upgrade --cask codex` here, npm/bun elsewhere).

**It is a real blocker, not decoration.** That distinction is rangerhq-1xsj's,
and it went the other way for grok: grok's splash buffers typed text into the
composer beneath it, so `idle` was the honest answer there. This one has no
composer beneath. `PROBEGWART` sent to the untouched menu never appeared, and
after the menu was dismissed the composer still showed its placeholder — the
text was **discarded**, not buffered.

**No launch-line silencer exists.** Checked before writing a rule, since
`--disable hooks` is exactly that kind of answer for the sibling screen:
`--disable in_app_updates` (`features.in_app_updates=false`) does NOT suppress
the menu — measured, the menu drew unchanged with the flag on the line — and
the 0.147.0 binary carries no `check_for_updates` / `auto_update` /
`disable_update` / `update_check` config key and no `CODEX_*` env var for it.
`~/.codex/version.json`'s `dismissed_version` remains the only silencer, and it
is the operator's (`internal/rhq.CodexInterstitials`).

**Nothing changed in dispatch, and nothing needed to.** Both ladders already
do the right thing once the state is right:

- typed — `awaitSettled` takes `agent explain`'s `blocked` over `agent wait`'s
  stale `idle` and `awaitAgent` dies by name, instead of burning the whole
  `StartupWait` and reporting "the pane may still be at a shell prompt".
- argv (codex's path today) — `awaitDelivered` sees a matched rule, then
  `gather`'s wait returns `blocked` and prints `⛔ … blocked in <session> —
  intervene`. Claim kept, bead not judged, no Enter pressed anywhere.

**`make verify-detection` cannot fail a committed change.** It replays the
fixtures through the *installed* manifest in `~/.config/herdr/agent-detection/`,
so deleting `update_menu` from the checkout still goes 8/8 until someone runs
`make install-detection`. That is why the pin is a Go test
(`internal/rhq/codexupdatemenu_qa_test.go`): it explains the checkout's
manifest in a temp `XDG_CONFIG_HOME`, and asserts both halves — `blocked` via
`update_menu`, and the same capture back to unrecognized-`idle` with the rule
cut out. Verified red on both counts with the rule removed.

**The probe has a shelf life and it is nearly up.** `posse runtime check codex`
reads `silenced` right now off `dismissed_version 0.149.1 = latest_version
0.149.1`, but latest is already **0.150.1** upstream — the isolated home's
check picked it up mid-session. The operator's `version.json` still says
0.149.1 only because it has not re-checked since 2026-08-25. The menu returns
on this box the moment it does.
