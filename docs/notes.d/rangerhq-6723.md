## The startup-screen special case is retired (rangerhq-6723)

`internal/rhq/dispatch.go` no longer presses a key at any screen.
`clearStartupScreen`, `startupScreenDismissals` and `startupScreen` are gone,
and `awaitAgent` is four lines: wait for a settled state, refuse anything that
is not `idle`/`done`. `AgentSendKeys` survives in `herdr.go` as a binding with
no caller in dispatch.

**Why it could go.** The table had exactly one entry, grok's `startup_splash`,
and the branch only ran on `status == "blocked"`. rangerhq-1xsj made that rule
report `idle` (manifest 2026.07.16.104), so the branch became unreachable for
its only user. rangerhq-3hb5 had already shown it was unreachable in a stronger
sense: the splash is not drawn until ~0.80s and the readiness gate opens at
~0.20s, so the Esc press never fired in a real launch even when the rule did
say `blocked`. It was covering a hole it stood 0.6s to the right of.

**What `blocked` is still doing in the settle wait.** It stays in
`awaitSettled`'s `until` list, and that is not leftover. It buys a launch that
fails *immediately and by name* on a real blocker instead of sitting out the
whole `StartupWait` behind one — which is how claude's trust dialog is
reported (rangerhq-4mzt). Retiring the dismissal table makes the wait no less
selective; rangerhq-3hb5's seen-state requirement is what made it more.

**The test that stopped testing, and what replaced it.**
`TestStartupScreenRulesExistInTheManifests` asserted that every dismissal rule
id appears in a manifest. It went on passing after 1xsj because it never looked
at the rule's *state* — the rule was still there, it had simply stopped
producing the state the code keyed on. That is the shape to watch for: a
contract test that pins a name against a fix that changed a value.

It was also, per hoover's ruling on rangerhq-4mzt, the executable form of "the
launcher may never answer a drawn dialog" — its two live assertions were *only
Esc* and *only a rule id from posse's own manifests*, the second because
claude's trust dialog matches herdr's generic `live_blocked_form` and one entry
for it would answer every form claude ever draws. Deleting the table leaves
that ruling nothing to constrain, so it is re-pinned two ways in
`startupscreen_test.go`:

- `TestDispatchAnswersNoBlockedScreen` — a permission selector, the generic
  form, *and* `startup_splash` itself all fail the launch loudly with zero
  `agent send-keys`. The screen posse used to answer is now in the list of
  screens it must not.
- `TestDispatchPathPressesNoKeys` — `dispatch.go` names neither
  `AgentSendKeys` nor the retired symbols, so the table cannot come back
  quietly. If ADR 0013 §2 layer 3 ("declared keystrokes") is ever taken up for
  another agent, hoover's two assertions come back with it.

Both were measured against the pre-change `dispatch.go`: swapped `HEAD`'s file
in and the `startup_splash` subtest fails on all three of its assertions while
the other two subtests still pass, which is the right split — those two screens
were never answered.

`etc/herdr/agent-detection/grok.toml`'s id note and the README paragraph both
said the id was load-bearing for dispatch. It is not any more; it is still
load-bearing for `verify-detection`, which pins the splash fixtures to this
rule id rather than to state `idle` (a state-only check went vacuous when the
state became the same one herdr's fallback guesses — rangerhq-uglc). Both files
now say that instead. `make verify-detection`: 7/7.
