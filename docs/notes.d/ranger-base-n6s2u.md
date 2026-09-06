## codex's sign-in screens read `blocked` now (ranger-base-n6s2u)

The sixth codex screen, found by ranger-base-femsg's 0.153.4 re-audit and left
open there: a codex whose credentials are missing or expired draws a **sign-in
menu** at startup instead of a composer, and `herdr agent explain` read it
`state: idle, rule: none, fallback_reason: default_known_agent_idle_fallback`
— the same fallthrough rangerhq-7ia (`hooks_review`) and rangerhq-9py0
(`update_menu`) were written to close. Not a 0.153.4 regression; nobody had
ever captured the screen.

The bead's own order was: **measure discard-vs-buffer first** (rangerhq-1xsj —
`blocked` has to be earned), then write a rule, then prove the fixture
measures it, then say what a dispatched seat does.

### The measurement

codex-cli 0.153.4, one fresh empty `CODEX_HOME` per arm, panes on a scratch
tmux server (`tmux -L cdxsi`) at 120 / 80 / 60 / 40 columns. **No Enter was
ever sent**, and option 2 — device code — was never pressed: it asks OpenAI
for a real code, and no reading is worth an unasked-for network round trip.

| probe | result |
|---|---|
| `zzmeasurezz` typed at the untouched menu | nothing echoes; the screen is unchanged but for the animated logo |
| Esc at the menu | nothing. The menu stays |
| `3` — a **bare digit, no Enter** | activates the option: straight to the API-key entry screen |
| the buffered text, on arrival there | **gone**. The field holds its `Paste or type your API key` placeholder |
| `zzcontrolzz` typed into that field | echoes at once — the control that says the two rows above are readings and not a blind rig |
| Esc from the field | back to the sign-in menu. The menu is the floor of this flow |
| the same launch with credentials present | reaches `Ask Codex to do anything` immediately |

So: **discarded, not buffered**, and there is no composer beneath to buffer
into — the last row is the mirror arm, the same launch differing only in
whether that home carries credentials at all. grok's `startup_splash` went the
other way for exactly the opposite reading (text lands in the live composer,
so `idle` is honest there). `blocked` is earned here three times over, and
the bare-digit row is the sharp end: a dispatched prompt carrying a `1` starts
the ChatGPT browser sign-in flow, a `2` a device-code request.

### The rules

`signin_menu` (priority 930) and `signin_api_key` (920) in
`etc/herdr/agent-detection/codex.toml`, manifest version `2026.08.09.103`.
Fixtures `testdata/codex/blocked-signin.txt` (120 columns),
`blocked-signin-narrow.txt` (60) and `blocked-signin-api-key.txt` (80).

Keyed on two of the **numbered options**, never on the footer: `Press enter to
continue` is `update_menu`'s footer too and no upstream rule matches it, so a
reword must not be able to walk this screen back to `idle`. The API-key screen
is keyed on its heading plus the field's own words; its footer is `Press enter
to save` / `Press esc to go back` on **two** lines, which is why
`live_strong_blocker`'s single-line `press enter to confirm or esc to go back`
never reached it.

**`region = "top_non_empty_lines(24)"` is measured, not picked.** The menu sits
under a 15-line ASCII logo, so `2. Sign in with Device Code` is the 21st
non-empty line at 120 columns and the 23rd at 60, where the prose above wraps.
The siblings' 20 misses it at every width. At 40 columns codex drops the logo
and the whole screen is 15 lines.

### What pins it

`internal/posse/codexsignin_qa_test.go`, three tests, all against the
**checkout's** manifest staged into a temp `XDG_CONFIG_HOME` (`codexExplain`,
shared with `codexupdatemenu_qa_test.go` rather than re-implemented):

- both screens read `blocked` by their own rule ids, and each capture goes
  back to `idle`/`none` with its rule cut — the inversion, without which a
  rule that never fires passes;
- each of `signin_menu`'s two `all` clauses is load-bearing: drop one numbered
  option out of the capture and the rule must stop firing, **and** the same
  trimmed capture must go blocked again once the matching clause is cut from
  the rule. The second half is the witness that the arm failed for the reason
  claimed rather than because the derived capture stopped being the screen;
- the region is mutated at its edge — 20 fails both captures, 22 passes the
  wide one and fails the narrow one, 24 passes both. That is what the narrow
  fixture is committed for, and without it the 24 is a number nobody measured.

`make verify-detection` is 13/13 with the three new fixtures. Unlike
rangerhq-9py0's day, it now replays the checkout rather than the install
(ranger-base-53w1), so it fails a broken rule before anyone deploys one.

### What a dispatched seat does

Nothing types through, and nothing needed to change in dispatch. Read from
`internal/posse/dispatch.go`, not run — a live dispatch rig needs a scratch
herdr server, and a scratch socket is not a sandbox:

- **argv (codex's path).** `awaitDelivered` returns as soon as detection is
  *seen*, so instead of burning the whole startup wait and printing "herdr
  never recognized a screen there", `gather`'s wait comes back `blocked` and
  the pass prints `⛔ <bead> blocked in <session> — intervene (posse attach
  <session>)`. Claim kept, bead not judged, no key pressed.
- **typed.** `awaitSettled` hands a blocker straight back and `awaitAgent`
  dies `never settled idle (status "blocked")`.

The remedy is an operator signing that box's codex in. A persona files it and
stops — signing in is identity and money (crew guardrail 1), and ADR 0013 §2
says an interstitial whose default action mutates the machine is a launch
refuse, never something to answer.

**The gap that leaves.** The *pre-launch* refuse still has nothing to read:
`CodexInterstitials` (`internal/posse/interstitial.go`) declares the update
menu and not this screen, so `posse runtime check codex` says nothing about a
box whose codex cannot authenticate — it is found at launch instead of before
one. The entry, with a credential-presence probe over the home `codexHome()`
already resolves (ADR 0019: presence, mtime and path, never values — and an
API-key environment variable reads *unknown* rather than "not silenced"), is
handed to the code lane as its own bead.

### Not captured

The device-code screen behind option 2, for the reason above. It is the one
screen of this flow with no rule and no fixture.
