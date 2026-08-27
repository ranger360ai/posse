# Authoring an agent-detection manifest

*ADR 0012 D4 §6 · the hardest requirement in the runtime contract, and the
one that is only partly posse's.*

Everything else about onboarding a third-party CLI is a key in
`runtimes/<name>.yaml`. This is not. herdr decides whether a pane is
`idle`, `working` or `blocked` from a per-agent TOML manifest, and a
runtime herdr cannot name from argv0 is **`agent_not_found`**: dispatch
cannot address the session at all, `working` and every settled state are
guesses, and `posse runtime check <name>` reports it as a blocking gap.

`etc/herdr/agent-detection/README.md` is the reference for posse's two
existing overrides — how they were forked, what they pin, when to retire
them. This page is the other half: authoring one for a CLI herdr has never
heard of.

---

## First, the thing that will cost you the evening

**A standalone manifest for a new agent id is ignored.** herdr's agent
kinds are compiled into the binary — `herdr agent start --help` prints them
as clap `[possible values: …]` — and dropping
`~/.config/herdr/agent-detection/mycli.toml` in beside the others does not
add one.

Measured on herdr 0.8.0, 2026-08-27 (rangerhq-tr8k). Reproduce it in a
minute:

```sh
cat > ~/.config/herdr/agent-detection/mycli.toml <<'EOF'
id = "mycli"
version = "2026.08.27.1"
min_engine_version = 3
updated_at = "2026-08-27T00:00:00Z"

[[rules]]
id = "mycli_idle_probe"
state = "idle"
priority = 1000
region = "whole_recent"
visible_idle = true
any = [ { contains = ["mycli>"] } ]
EOF
herdr server reload-agent-manifests >/dev/null
printf 'mycli> ready\n' > /tmp/pane.txt
herdr agent explain --file /tmp/pane.txt --agent mycli --json
```

```json
{"agent":"mycli","state":"unknown","fallback_reason":"unknown_agent",
 "manifest_source":null,"manifest_version":null,"evaluated_rules":[]}
```

No manifest, no rules evaluated, and `herdr server agent-manifests --json`
does not list it. Remove the file and reload before you go further.

### The two routes that do work

**1. `aliases` on an existing manifest.** A manifest's `aliases = [...]`
resolves foreign labels onto it. Measured the same day: our own
`etc/herdr/agent-detection/grok.toml` carries `aliases = ["grok-build"]`,
and `herdr agent explain --agent grok-build` comes back as `agent: "grok"`
with grok's manifest version and grok's rules matching. So a CLI whose
screens resemble a kind herdr *does* know can be aliased onto it:

```toml
# in the forked manifest of the agent you are aliasing onto
aliases = ["grok-build", "mycli"]
```

The price is real and the README spells it out: an override **shadows
upstream permanently and silently**. You now own that agent's detection for
this box, including every upstream fix you will not receive. Record
`# posse_forked_from` so `scripts/verify-detection.sh` can warn when
upstream moves past your fork point, and read the README's "Retiring this
override" before you take the fork.

Alias only where the screens really are the same shape. An alias onto a
kind whose rules do not match your CLI buys
`default_known_agent_idle_fallback` — a *guess* of `idle` — which is worse
than `unknown`: dispatch prompts into it.

**2. Upstream.** herdr adding the kind is the only route with no shadowing
cost. `etc/herdr/agent-detection/upstream-report.md` is the shape of that
filing; sending one is the operator's call, never a persona's.

---

## Anatomy of a rule

A manifest is a header plus `[[rules]]`. From
`etc/herdr/agent-detection/codex.toml`:

```toml
id = "codex"                       # the agent kind — must be one herdr knows
version = "2026.08.09.101"         # highest version wins across sources
min_engine_version = 3
updated_at = "2026-08-09T00:00:00Z"
# aliases = ["codex-cli"]          # optional; see "The two routes" above

[[rules]]
id = "hooks_review"                # the name that appears in `agent explain`
state = "blocked"                  # idle | working | blocked
priority = 960                     # highest matching rule wins
region = "top_non_empty_lines(20)" # what slice of the pane is matched
visible_blocker = true             # this is a SEEN state, not a fallback guess
all = [                            # all/any/not, each with contains/regex/line_regex
  { contains = ["Hooks need review"] },
  { regex = ['(?s)hooks?\s+(?:is|are)\s+new\s+or\s+changed'] },
]
```

**Regions** seen in the shipped manifests: `whole_recent`,
`top_non_empty_lines(n)`, `bottom_non_empty_lines(n)`,
`after_last_prompt_marker`, `osc_title`, `osc_progress`.

**`visible_*`** is what separates a rule that MATCHED from herdr's guess.
posse's readiness gate demands a seen state (`awaitSettled`,
`internal/rhq/dispatch.go`), because `default_known_agent_idle_fallback`
answers `idle` for any known agent whose screen matched nothing — including
a pane that is still a shell 0.2s into a launch. Set it on every rule you
write.

### The priority ladder

Take the numbers from the manifest you are extending; these are grok's:

| band | rules |
|---|---|
| 1300 | `osc_title_blocked` — the terminal title says blocked |
| 1200–1180 | dialogs and permission prompts (**blockers**) |
| 1170–1150 | working chips, OSC progress |
| 1105 | `startup_splash` (posse's) — an idle rule that must sit *below* every blocker |
| 1100 | `osc_title_idle` |
| 900–500 | weaker blockers, working fallbacks |
| 100 | `prompt_hints_idle` — the composer's own footer |

**An idle rule must lose to every blocker.** posse's `startup_splash`
landed at 1250 first and outranked `option_dialog_blocked` (1200): a real
permission dialog drawn over the splash then read `idle`, and a dispatched
prompt would have been typed into it. Verified both ways before it was
moved to 1105 (rangerhq-1xsj). 1105 also sits just above `osc_title_idle`,
so a text-only fixture and a live pane give the same answer.

### What not to key on

- **Anything that survives into a live composer.** grok's "Help improve
  Grok" consent banner stays on screen while the composer takes input;
  keying a blocker on it would strand every pane forever.
- **The shared footer instead of the dialog.** codex's `hooks_review` is
  keyed on the dialog's own text, not on the
  `Press enter to confirm or esc to go back` footer it shares with `/model`
  and `/approvals`, so a footer reword cannot silently turn the dialog back
  into `idle`.
- **A tag that races.** grok's version footer renders
  `Grok Build 1.0.5 [stable]` on one pane and `Grok Build 1.0.5` on the
  next — the channel resolves asynchronously. Requiring the tag drops the
  rule at random. Pin both variants as fixtures.

---

## Fixtures, and the loop

```sh
herdr pane read <pane> --source detection > \
  etc/herdr/agent-detection/testdata/<agent>/<state>-<what>.txt
make install-detection    # install + reload + verify
make verify-detection     # verify only
```

`scripts/verify-detection.sh` replays every fixture through
`herdr agent explain --file`; the filename encodes the expected state
(`blocked-…`, `idle-…`), and every `*.toml` in the directory is picked up
automatically. Pin the **rule id**, not just the state: after 1xsj our
splash rule reports the same `idle` herdr's fallback would, so a state-only
check passes with the rule deleted (rangerhq-uglc).

Two limits worth knowing before you spend an hour on them:

- **Snapshots are text only.** Rules keyed on `osc_title` / `osc_progress`
  cannot be pinned this way; check those against a live pane with
  `herdr agent explain <pane>` during a real turn.
- **Reload is per server.** Each `herdr --session <name> server` holds its
  own cache, so `make install-detection` does not reach a scratch server —
  re-run the reload with `HERDR_SOCKET_PATH` pointed at it.

And when you fork a *bundled* manifest, extract it with `dd`, never
`strings`: `strings` mangles the box-drawing and braille characters the
rules match on. The recipe is in the README.

---

## Checking your work from posse

```sh
posse runtime check <name>
```

The `launch` row says whether herdr recognizes your argv0 and which
manifest version answered; the preflight at the bottom reports a missing
manifest as a **blocking gap** and exits 1. It asks herdr the way herdr
resolves it (`agent explain`), so a CLI reached through an `aliases` entry
counts as recognized — a check that only read the compiled kind list would
tell an operator who aliased correctly that their CLI is undetectable.

A green `posse runtime check` is not a launched session. Nothing here
proves the contract end to end; that gate still wants a human to stand up a
pane (ranger-base-nlya).
