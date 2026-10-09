## Two live pins whose control arms stopped firing, for two unrelated reasons (ranger-base-yglkp)

ranger-base-yglkp reports `TestLiveClaudeSettingsPinRefusesACredentialDirRedirect`
(credentialdirpin_live_test.go) and
`TestLiveClaudeFieldPinRefusesAPlantedCommandField` (fieldpin_live_test.go) red
under `RHQ_LIVE_CLAUDE=1` on claude 2.1.295, each on its own CONTROL arm — the
shape that says the rig measured nothing. Both reds reproduced at
`bcd7b0ea`, before the landing that was suspected, so neither was a
regression of a landing. They were also not one problem: the two have nothing
in common but the readout they share.

Everything below is **MEASURED 2026-10-09**, darwin 25.4.0 on the operator's
box, against `claude auth status --json` (no login, no network turn, no trust
prompt) on the five builds present under `~/.local/share/claude/versions`:
2.1.288, 2.1.291, 2.1.292, 2.1.294, 2.1.295. The box carries a root-owned
policy tier: `managed-settings.json` =
`{"env":{"CLAUDE_SECURESTORAGE_CONFIG_DIR":"","CLAUDE_CONFIG_DIR":"<the
operator's ~/.claude>"}}`, plus all three of this repo's drop-ins installed
under `managed-settings.d/` (`10-posse-inlet-pin.json`,
`20-posse-field-pin.json`, `30-posse-hooks.json`).

### 1. credentialdirpin: the first claude run against a fresh config dir reads `loggedIn=false`

The redirect the pin is about **still happens**. What broke is narrower and
entirely inside the rig: the pin's first arm was the first `claude` ever run
against its `t.TempDir()` config directory.

That run finds no `.claude.json` under the configured directory, writes a
fresh one, and says so on **stderr**:

```
Claude configuration file not found at: <dir>/.claude/.claude.json
A backup file exists at: <the operator's ~/.claude/backups/...>
You can manually restore it by running: cp ...
```

Stdout still carries well-formed JSON — which is why the pin did not die at
its "unparseable answer" guard — and that JSON reads `loggedIn=false`
whatever the settings say.

Three fresh directories, three calls each, identical files on disk and
identical settings between calls:

| call | loggedIn |
|---|---|
| 1 | false |
| 2 | true |
| 3 | true |

3/3 trials. Pre-creating `.claude.json` does **not** stand in for the first
run — measured empty, `{}`, and `{"hasCompletedOnboarding":true,"numStartups":3}`,
all three still false on the next call. A throwaway readout does.

That is the whole of the first red. Arm 1 (`CLAUDE_SECURESTORAGE_CONFIG_DIR`
named outright) failed its control; arms 2 and 3, running against the
now-warm directory, passed. **Fix: one throwaway `claudeLoggedIn(t, home)`
before the arms**, with the measurement at the call site. All four arms green
in 2.31s afterwards.

Worth keeping separately: the user-scope settings `env` redirect of
`CLAUDE_SECURESTORAGE_CONFIG_DIR` **wins over the policy tier's `""`** for
this variable once the directory is warm (arm 1 green), while a
`CLAUDE_CONFIG_DIR` set in the real process environment **loses** to the
policy row (measured false where the same value in a user-scope `env` block
reads true). The asymmetry was not chased down; it is recorded because a
future reader of either pin will hit it.

### 2. fieldpin: the attack arm is shadowed by this repo's own policy drop-in

Here the control arm cannot be restored where it was, and the reason is a
guarantee rather than a defect.

`etc/claude/managed-settings.d/20-posse-field-pin.json` is **installed** on
this box (since 2026-09-06), and it carries `apiKeyHelper: ""` at
`policySettings` — the last source in the runtime's left fold over
`userSettings, projectSettings, localSettings, flagSettings, policySettings`.
So nothing below policy can arm an `apiKeyHelper` here any more, and the
pin's project-scope attack arm reads "not in force" with no pin on the line
at all.

Measured, every one of the five builds, config directory warm, both under
this session's environment and under `env -i` with only `HOME`, `PATH`,
`TERM` and `CLAUDE_CONFIG_DIR`:

| apiKeyHelper planted at | non-bare | `--bare` |
|---|---|---|
| user scope (`$CLAUDE_CONFIG_DIR/settings.json`) | not in force | — |
| project scope (`./.claude/settings.json`) | not in force | — |
| flag scope (`--settings`) | not in force | **in force** |

And it is not merely unreported. A real `-p` run with a helper that appends
to a marker file and echoes a fake key left **no marker** at user, project or
flag scope non-bare, printing `Not logged in · Please run /login` each time.
The helper is not resolved and not executed.

`--bare` reaches past the policy row because it resolves this field from the
raw flag source rather than the merged object. 2.1.295's resolver:

```js
function am(){ if(ur()) return me("flagSettings")?.apiKeyHelper;
               if(Wd()) return;
               return (rr()||{}).apiKeyHelper }
```

`ur()` is `CLAUDE_CODE_SIMPLE`/`--bare`; `Wd()` is
`CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST`; `rr()` is the merged settings object
(its other readers in the same module take `availableModels` and
`enforceAvailableModels` off it). `--bare`'s own entry in `claude --help`
says the same thing in prose, restated here rather than excerpted: in minimal
mode Anthropic auth is strictly the API-key environment variable or an
`apiKeyHelper` arriving via `--settings`, and OAuth and the keychain are never
read.

The readout itself is **intact** — a flag-scope helper under `--bare` reads
`loggedIn=true, authMethod=api_key_helper`, and a flag-scope `env` row naming
the API-key variable reads `loggedIn=true, authMethod=api_key` — so nothing
here is the readout having stopped reporting.

#### What the live pin measures now

Two venues, neither allowed to pass silently:

1. **The project-scope arm is kept.** If its control fires (a box with no
   policy-tier field pin), it runs the original precedence measurement. If it
   does not, the test requires an OS-admin settings file on *this* box to name
   `apiKeyHelper` — and skips with that path in the message. A control that
   does not fire with nothing to explain it is still a `Fatalf`.
2. **The type canary moved to `--bare`, and got stronger.** The old canary was
   a one-row payload (`{"apiKeyHelper":"…FLAG…"}`) beside the payload under
   test, so it said the readout worked and nothing about the real payload. The
   new arms plant the helper **inside** the rendered payload — `apiKeyHelper`
   replaced, every other row exactly as the launcher ships it — and assert it
   is in force. One wrong-typed row anywhere in that payload and the runtime
   discards the whole object, so the planted row vanishes with it. Then the
   same payload as shipped must read "not in force", which is the pin, and is
   evidence only because the canary fired.

Both of `ClaudeFleetSettingsJSON(nil)` and `credentialDirPinJSON()` get both
arms; the old test graded the second builder with no canary at all.

Verified by reintroducing the hazard the header exists for: with
fieldpin.go's `statusLine` row mutated back to a bare `""`, **both** canary
arms go red with "the runtime threw the whole object away". Restored, both
green.

#### What this venue no longer claims

Under `--bare`, `flagSettings` is the only source read for this field, so
there is no lower scope for the pin to beat. The precedence half of
ranger-base-i7cy4 is now held by the policy drop-in — which outranks every
scope the old arm could reach, i.e. strictly more than the launch pin could —
and by the merge measurements transcribed in `fieldpin.go`. The launch pin
stays load-bearing on every box without the drop-in installed, which is why
both ship.

### 3. The general shape, for whoever reads the next one of these

Both reds were honest pins doing their job, and neither was about the thing
the failure message guessed at first. The two failure modes are worth naming
because they will recur:

- **A cold scratch directory is a state the runtime has an opinion about.**
  A rig that creates `t.TempDir()` and measures once measures the first-run
  path, not the steady state. One throwaway call is the whole fix, and the
  arm ordering hides it: the arms that run later look fine.
- **A pin can be shadowed by a stronger version of itself.** Installing the
  policy-tier drop-in retired the live pin's attack arm, because the hole it
  probes is now closed above the scope the arm can write. That is success,
  and it reads exactly like breakage. The pin now has to name the file that
  closed it rather than go green or go red.
