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
`internal/posse/dispatch.go`), because `default_known_agent_idle_fallback`
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

It explains against the manifests **in the checkout**, staged into a
throwaway `XDG_CONFIG_HOME`, so a rule you break fails before anyone
installs it and no install is needed to run it (ranger-base-53w1 — it used
to read `~/.config/herdr/agent-detection`, which made it unable to fail a
committed change). The installed copy is compared to the checkout and
reported; `--check-install`, which `make install-detection` passes, turns a
mismatch into a non-zero exit.

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
manifest as a **blocking gap** and exits 1 — unless the profile declares
`detection: reported`, in which case it is a named degrade and the check
exits 0 (below). It asks herdr the way herdr
resolves it (`agent explain`), so a CLI reached through an `aliases` entry
counts as recognized — a check that only read the compiled kind list would
tell an operator who aliased correctly that their CLI is undetectable.

A green `posse runtime check` is not a launched session. Nothing here
proves the contract end to end; that gate still wants a human to stand up a
pane (ranger-base-nlya).

---

## Detection by report — when you do not author a manifest at all

*ADR 0061. Two keys, and the one case where a missing manifest is not a
missing anything.*

Everything above assumes the pane's label comes from **herdr**: argv0 plus
a manifest. There is a second route, and it is herdr's own documented
"integrate your own agent" one — an outside authority states the label:

```sh
herdr pane report-agent --source <who> --agent <label> --state working <pane>
```

Every herdr **plugin** takes that route. The `MartinLoeper/herdr-bob`
plugin labels Bob panes this way today, from its own watcher. If your CLI
is labelled by something like that, you author no TOML: the label already
arrives, and what posse needs to be told is that it does.

```yaml
# runtimes/<name>.yaml
detection: reported
detection_why: herdr-bob plugin MartinLoeper/herdr-bob, installed 2026-09-28
```

`detection:` is `herdr` (the default — everything above) or `reported`.
`detection_why:` is **required** with `reported` and names the authority;
the plugin id and its install date is the honest form. It is not
documentation — it is the sentence posse prints when the label never
arrives, and "no authority labelled the pane" is only actionable beside the
name of the authority that was supposed to.

Both keys are **instance facts**: whether *this box* installed a reporter is
a measurement of the box, so they overlay onto a built-in
(`runtimes/bob.yaml`) and no built-in ships `reported` — `bob` says it has
no detection until upstream compiles the kind in.

### When to declare it

When all three hold:

1. herdr has no manifest for your argv0 — `posse runtime check <name>` says
   so, and authoring one is not the plan.
2. Something else labels the pane through `pane report-agent`, and you have
   watched it do so: `herdr agent get <pane>` names your label while the CLI
   is running.
3. Your herdr has the verb. It arrives in **0.9.0** — `herdr --version`.
   Declaring `reported` on 0.8.2 makes a bead-carrying launch **refuse by
   name**, because nothing can report and the launch could only wait out
   `startup_wait`.

If herdr already detects your argv0, the declaration is **inert** and
`runtime check` says so: herdr's own detection wins, and the key should be
dropped. That is what the day upstream ships your kind looks like.

### What it costs (and what it does not)

`detection: reported` lifts the launch refusal. It does not make the pane
as readable as a detected one, and `runtime check` reports the difference as
a non-blocking degrade rather than a clean row:

- **Every state posse acts on is the reporter's word.** herdr reads no
  screen for such a pane — `agent explain` answers
  `agent_explain_unavailable` — so there is no matched rule behind
  `working` and no visible chrome behind `idle`.
- **`blocked` is whatever the reporter can see**, which may be nothing. On
  herdr-bob today it is nothing: moot for a dispatched seat under
  `--auto-approve`, real for a session you launched by hand.
- **A reported label outlives its process.** herdr ties it to the pane, not
  to the process, and the herdr-bob watcher releases only when the pane
  closes — so a CLI that exits to its shell keeps reading `idle`. posse
  guards that with herdr's own process reading (ADR 0061 D3), and upstream
  has two asks open (D4).
- **The launch says it once.** A launch onto a reported runtime prints one
  line naming the authority, before anything reads a state off the session.
- **`posse runtime probe <name>` runs here** — that is why the gap does not
  block. It is what measures what detection on this runtime actually reads.

### When no label arrives

The launch's failure line does not say "check the session" and does not
offer a larger `startup_wait:`. The observable was an authority labelling
the pane, and an absent label says nothing about the CLI — it may be up and
working with nobody having reported it. First remedy:

```sh
herdr plugin list      # is that authority installed, enabled, and watching?
```

An *enabled* plugin is not a *running* one; herdr-bob's watcher needs a hand
start today.
