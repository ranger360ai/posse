## The second footprint, and the one that leaves the box (ranger-base-esz16)

ranger-base-et7bs guarded the three live-box pin gates against moving the
Homebrew data they read (`HOMEBREW_NO_AUTO_UPDATE=1`, above the calls) and
called that "the one footprint it does leave". There were two. The other one
does not stay on the machine: with analytics enabled, every `brew` invocation
in those gates POSTs a `command_run` event to Homebrew's InfluxDB.

hoover's ranger-base-w2bha found it and ruled SUPPRESS: the source audience of
a crew-run check is this box, that endpoint is wider, and nothing on the box
rules the wider audience acceptable, so it does not move — crew hard risk line
4, which is the default the line already sets rather than an accepted risk.
This bead applies it.

### MEASURED 2026-10-31, this box (Homebrew 7.0.11-75-gfcdfa04, darwin 25.6.0)

Analytics are **on**:

```
$ git config -f "$(brew --repository)/.git/config" --get homebrew.analyticsdisabled
exit=1          # the key is unset
$ ls ~/.homebrew_analytics_user_uuid
No such file or directory
```

brew's own `Utils::Analytics.disabled?` is
`return true if Homebrew::EnvConfig.no_analytics?` then
`config_true?(:analyticsdisabled)`, so unset means enabled — brew's default.

The suppression is read out of brew's source on this box rather than measured
on the wire, deliberately: confirming the POST by observing it would mean
sending the event this bead exists to stop.
`/opt/homebrew/Library/Homebrew/utils/analytics.rb` guards both the generic
reporter and the one that matters here with the same line:

```ruby
def report_influx(measurement, tags, fields)
  return if not_this_run? || disabled?
...
def report_command_run(command_instance)
  return if not_this_run? || disabled?
```

`disabled?` is true whenever `HOMEBREW_NO_ANALYTICS` is set in the
environment, so a per-process export in front of the gate's brew reads is the
whole of the fix — nothing is written to disk and nothing is asked of brew.

### What changed

- `scripts/verify-grok-pin.sh`, `scripts/verify-codex-pin.sh`,
  `scripts/verify-bd-pin.sh` — the guard line is now
  `export HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1`,
  the spelling `scripts/macos-install-probe.sh:521,817` already used, with the
  reasoning above it in prose.
- `internal/treepins/brewguard_qa_test.go` — the pin's demand is a list now
  (`bgGuards`), not one string: `bgRead` records the line of each setting's
  first runnable `export`, including several on one line, and
  `bgAssertBrewReadsAreGuarded` fails per setting that is missing or that sits
  below the first brew read. Adding a row to `bgGuards` is the whole of adding
  a guard — it is demanded of all three scripts at once and cannot be
  half-applied, which is what the tree looked like between et7bs and this
  bead. The control grew that shape as a fixture (`auto-update only — the
  pre-esz16 tree`, `analytics only`, and the two on separate lines with one
  below the call), and `TestQABrewGuardListIsTheTwoMeasuredSettings` pins the
  list against the head comment that explains each member.

The three arms keep their names
(`TestQA{Grok,Codex,Bd}PinScriptDoesNotLetBrewAutoUpdate`). The pin is the
helper, not the names, and et7bs's fragment cites those spellings.

### Out of scope, deliberately

The box-wide `brew analytics` state is the operator's own machine setting for
his own brew use. No gate runs `brew analytics off`, and nothing here writes
to brew's settings — the fix is one environment variable per gate process.
