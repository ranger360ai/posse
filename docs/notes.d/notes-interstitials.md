## Instance interstitials: the keys posse names, and the one it writes (ADR 0013 §2)

Every CLI draws something on its first run in a fresh pane — a consent
banner, an update menu, a splash. herdr recognizes none of those screens,
so a *dispatched* session lands on one, reports `idle` /
`default_known_agent_idle_fallback`, never becomes promptable, and burns
its startup wait for nothing. Measured on both non-claude runtimes in one
evening (`ranger-base-3j8`), and the layer that fixes it is not the clever
one.

**Three layers, cheapest first (ADR 0013 §2).**

1. **Sidestep.** Deliver the work prompt on the launch line (`prompt:
   argv`), so the screen is not the delivery channel at all. Landed for
   grok and codex (`ranger-base-cl7`/`ranger-base-dg5`); see *Argv
   delivery* above. This layer does not make the banner go away — it
   makes it stop being in the way.
2. **Instance config.** Operator-owned facts posse **documents and never
   writes**. This section.
3. **Declared keystrokes.** Last resort, keyed on a herdr *rule id*
   (today: none — grok's splash was the only entry, retired in
   `rangerhq-6723` once detection stopped calling it a blocker). Any
   future table presses each key once, **Enter is not in the table**,
   and `TestDispatchPathPressesNoKeys` is edited in the same commit
   that revives it (ADR 0013 §2, amended ranger-base-xqft).

**Why layer 2 is a document and not a `os.WriteFile`.** Two of the three
answers are not a harness's to give:

- grok's `[Opt in]` lets xAI retain prompts and traces from sessions
  working in the operator's *private* repos. That is a visibility line
  (crew guardrail 4), and the privacy-preserving answer — `[Opt out]` — is
  a click, not a config posse can write on the operator's behalf.
- codex's update menu has `1. Update now` **default-selected**, and it runs
  `brew upgrade --cask codex`. Enter on arrival is an unreviewed
  roll-forward of a pinned tool, through a Homebrew this box has had broken
  (`rangerhq-y5on`), and the pinning precedent is `rangerhq-y7jr`.

Hence the rule, and it is the whole of layer 2: **a first-run dialog whose
default action mutates the machine is a launch refuse until that config
silences it, and nothing blind-sends Enter.** The coordinator's
string-match Escape watchdog is a stopgap, not the architecture.

**What enforces it, since `ranger-base-9r33`.** For eleven months that rule
was a sentence in three documents and nothing in the code: `Interstitial.Danger`
was read by `posse runtime check` and by nobody on a launch path, so codex
launched onto the menu the contract said must gate it. Three surfaces now
agree, all reading `DangerUnsilenced` (`internal/posse/interstitial.go`):

- **dispatch refuses above the claim.** `launchSession` asks before it
  creates a session, so a refused bead is never claimed and no workspace is
  made — the argv path claims *first*, so a refusal raised any later would
  hand back a bead it had already taken. The refusal is a persona/runtime
  failure by the §2 busy-key split, so the slot is benched for the pass
  rather than the refusal being printed once per bead.
- **every other launch path refuses from `planLaunch`** — the cockpit's `d`
  on a session it must create, a recipe, a relaunch — *if it carries a
  bead*. An **interactive** launch warns `DEGRADED` and proceeds. That is
  ADR 0015 §3's asymmetry, and here it is not merely analogous: the remedy
  for codex's menu is to **answer it**, in a codex session, so a posse that
  refused interactive launches too would have walled off the only way to
  clear its own refusal. It is also the escape hatch, and the reason there
  is no config key for one.
- **`posse runtime check` reports it as a BLOCKING gap**, and `posse runtime
  probe` refuses to probe — the probe launches a scratch session, which
  would meet the same screen.

**A refusal is a reading, never ignorance.** The probes are tri-state
(`Silence`: silenced / not silenced / **unknown**), and only the middle one
refuses. posse cannot read `~/.codex/version.json` on a box codex has never
checked for a release on, and a refusal whose own words are "cannot tell
whether the update menu is silenced" walls a box for something nobody
measured. The screen is not unguarded meanwhile: herdr names it `blocked`
by its own `update_menu` rule, so a launch that *does* meet it fails by name
instead of being typed into.

**A declared screen with `danger:` refuses too** (ranger-base-vbp3). An
interstitial declared in a `runtimes/<name>.yaml` has no probe at all, and
9r33 read that as "declaring a screen documents it and never walls the
declarer's own launches" — which made the whole rule unreachable for the
only runtimes that can newly meet it. The built-ins are measured, claude's
screen is `Seeded` and codex/grok deliver by argv, so the **first
typed-delivery runtime with a machine-mutating dialog is by construction a
declared one**, and it was dispatched onto that dialog while the grid
printed LAUNCH REFUSE about it. `danger:` is not posse guessing at a config
it cannot parse — it is the operator's own written statement that the
default action mutates their machine, so refusing on it is still a reading.
Declaring it is choosing the wall; a declared screen **without** `danger:`
walls nothing. The refusal does not lift by silencing the screen, because
posse cannot read the key: what lifts it is dropping `danger:` from the
profile, and the refusal line and the grid both say so.

**The keys, per runtime.** `posse runtime check <name>` prints each with
its file and whether it is silenced on this box — read-only probes, which
is the only thing posse does to these files.

| runtime | screen | key | who silences it, and how |
|---|---|---|---|
| grok | `Help improve Grok  [Opt out] [Opt in]` consent banner above the composer | `[privacy] privacy_banner_acked` in `~/.grok/config.toml` | the **operator**, clicking `[Opt out]` once in their own grok session. The value is an RFC3339 stamp, not a bool, and it records only *that* the banner was answered — never which way. In 1.0.5 the consent RPC has no server handler, so even an accidental `[Opt in]` cannot persist; that defense is version-verified and evaporates the day xAI ships the handler (`rangerhq-sz7u`). |
| grok | New worktree / Resume session / Quit startup menu, plus the changelog line | `[cli] auto_update = false`, `maximum_version` in `~/.grok/config.toml` | **already applied** — the fleet pin, declared in `etc/grok/version-pin.toml`, kills the update check *and* the shared leader's mid-life self-update. `make verify-grok-pin`; runbook in *grok substrate* above. |
| codex | `Update available! → 1. Update now  2. Skip  3. Skip until next version` | `check_for_update_on_startup = false` in `~/.codex/config.toml` (declared in `etc/codex/version-pin.toml`) | **nothing — already handled by the fleet pin**, which stops the menu being drawn at all. `make verify-codex-pin` asserts it, together with the `brew pin --cask codex` that makes `1. Update now` *fail* rather than upgrade. Without the pin there are two silences and both expire: picking **3. Skip until next version** (arrow **Down** twice, *verify the caret moved*, **then** Enter), which lasts exactly one release, and being at the latest release already, which lasts until the next one ships (ranger-base-cohw). |
| claude | `Quick safety check: Is this a project you created or one you trust?` — full screen, `1. Yes, I trust this folder / 2. No, exit`, footed `Enter to confirm · Esc to cancel` | `projects["<session dir>"].hasTrustDialogAccepted` in `~/.claude.json` (or `$CLAUDE_CONFIG_DIR/.claude.json`, or the config dir's `.config.json` where that exists) | **the LAUNCH**, per session directory — the one exception, below. |

**The codex dismissal has a shelf life; the fleet pin does not
(ranger-base-poj5).** `dismissed_version` silences one release: the menu
returns the moment `latest_version` moves past it. That is why `runtime check`
prints both numbers rather than a bare yes — a probe whose answer expires
should say when. It is also why the probe reads
`check_for_update_on_startup = false` *first*: with the startup check off,
what `version.json` says is not a reading about any screen the operator will
meet, and on 2026-08-30 the expired dismissal alone refused every codex
dispatch on this box. codex has no version-ceiling key to pin with — the
declaration, the measurement and the runbook for lifting it are in
`docs/notes.d/ranger-base-poj5.md`.

**A box already at the latest release is silenced too, and reading only
`version.json` never noticed (ranger-base-cohw).** Those two fields answer
*did the operator dismiss THIS release*, not *will the menu draw*: codex
offers an update only when a release **newer than the running one** exists,
so an operator who **updated** instead of dismissing read un-silenced
forever. MEASURED 2026-08-29 on codex-cli 0.150.1 against
`{"latest_version":"0.150.1","dismissed_version":"0.149.1"}` — the probe said
"the menu is back" while a peeked launch pane carried no `Update available`
and herdr read both live codex sessions `idle` rather than `blocked` on its
own `update_menu` rule. ADR 0013 §2 turns that reading into a launch refuse,
so the most up-to-date box was the one that could not launch. The probe now
asks codex what it is running (`codex --version`, the same reader the parity
drift check uses) as a **third** arm, after the two that cost no subprocess.
It stays honest the other way too: a codex that is not there, or will not say
what it is, reads UNKNOWN and refuses nothing.

**The one key the launch writes, and why it is not the same kind of key
(rangerhq-w4uf).** Claude's directory-trust dialog is not a first-*run*
dialog at all — it is a first-run-*here* dialog. It fires per session
directory, so the fleet's long-lived checkouts never show it and every new
repo, worktree, container HOME and scratch dir draws it again. There is
nothing an operator can answer once: MEASURED on claude 2.1.241, `claude
--help` names no flag for it, `claude project` manages only `purge`,
`--settings` takes no such key, and the single env that skips the check
(`CLAUDE_CODE_SANDBOXED`) claims a confinement a shims-tier launch does not
have. The CLI itself names the supported non-interactive path, in the error
it prints when it drops a project's hooks for want of trust: *"Run Claude
Code interactively here once and accept the trust dialog, or set
projects[<dir>].hasTrustDialogAccepted: true in ~/.claude.json"*.

So the launch seeds it (`SeedClaudeTrust`, `internal/posse/trust.go`), which
is the same grant posse already types on codex's line (`-c
"projects={\"$PWD\"={trust_level=\"trusted\"}}"`, ADR 0002 §4) and the
same one `SeedCageHome` already writes into a container HOME. What it does
NOT do is bend the layer-2 rule: it is scoped to the one directory posse
launched in, merged into the operator's file and never rewritten from a
template, skipped entirely when the directory is already trusted, and it
refuses the launch rather than replacing a config it cannot parse. Nothing
here presses a key: the seed is written before the line is typed.

Measured on the pane, four herdr scratch panes, no API turn:

- fresh dir, real config → the modal, and `herdr agent explain` reports
  **`blocked`** (`live_blocked_form`), not idle. So dispatch does not type
  a work prompt into the menu the way stock detection let it type into
  codex's *Hooks need review* — it waits out its patience and the session
  never becomes promptable. (Consistent with *Dispatch primitives* above:
  `agent wait` can settle `idle` a beat before the dialog is drawn, which
  is why explain wins.)
- same config dir, `hasTrustDialogAccepted: true` seeded → straight to a
  live composer, `herdr` `idle`, `live_prompt_box`.
- the key alone leaves claude's "Welcome back!" project panel drawn above
  the composer — harmless (pane read `idle`, composer live), silenced
  anyway with `hasCompletedProjectOnboarding`, because "harmless splash" is
  what grok's startup menu was called too.

**What the grant hands the session dir, and the keyed launch check.**
REMEASURED 2026-08-27 on claude 2.1.247 and re-run unchanged on
2.1.241/.245/.246 (ranger-base-i0s8), because the first telling of this
paragraph was wrong in its detail and the current claude docs say the
opposite shape. What the shipped binary does:

- **Directory trust gates two keys, and they are permission keys.**
  `permissions.allow` and `permissions.additionalDirectories` out of
  `.claude/settings.json` are dropped while the workspace is untrusted,
  and claude says so on stderr ("Ignoring 1 permissions.allow entry from
  .claude/settings.json: this workspace has not been trusted…"). The debug
  line quoted here before — "Dropped N project-scoped … entries — workspace
  not yet trusted" — is real, but its template is dynamic and those two
  permission keys are its only call sites in the bundle. It never said
  `hooks`. That misquote, not a version drift, is the whole disagreement:
  2.1.241 measured today behaves exactly like 2.1.247.
- **`hooks` are gated one layer down, at execution, and only where the
  dialog is a live screen.** The binary's own line is "Skipping <event>
  hook execution — workspace trust not accepted". Interactive and
  untrusted, the session parks on the trust dialog and a project
  SessionStart hook never runs; interactive and trusted — what the seed
  buys — it runs. Headless does not hold that line: `claude -p` in an
  untrusted directory runs the project's hooks, in the same run that drops
  that file's `permissions.allow`, and writes no trust entry. So the seed
  is the enabling act for repo hooks in a posse LAUNCH, which is
  interactive; it is not what gates hooks in general.
- **`mcpServers` under `.claude/settings.json` is inert** on these builds:
  never listed by `claude mcp list`, never named in a trusted session's
  debug log, never spawned. The live project-MCP channel is `.mcp.json`,
  and it sits behind its own "⏸ Pending approval" gate that reads
  identically trusted and untrusted.
- **`.claude/settings.local.json` is the same channel, not a safer one**
  (measured 2026-08-28 on 2.1.251, a fresh `CLAUDE_CONFIG_DIR` per arm and
  `ANTHROPIC_BASE_URL` on a dead port, no API turn — rangerhq-9u8). A
  `SessionStart` hook declared only there ran before the first turn and
  before the CLI resolved credentials at all: the run ended on "Not logged
  in - Please run /login" with the hook's witness file already written,
  while the same dirs with no settings file ran nothing. The fleet's own
  `--settings` JSON suppresses none of it — `--settings` adds a source,
  hooks merge across sources, and there is no flag posse can type that
  closes this channel. The trust gate above is one early return per hook
  *event*, taken before any source is consulted, so it covers this file
  identically. Gitignored is not a security property: whoever can write the
  repo can write that path, and `git status` will not show it afterwards.
  Hence both files are in the check (ADR 0002 amendment 2026-08-28).

None of that moves the launch check, and it makes it *more* load-bearing:
the check fires on settings content, so it does not depend on which of
claude's gates is holding, or on the docs and the binary agreeing next
release. Claude declares those settings paths with
`ProjectConfigKeys: [hooks, mcpServers]`: presence of either key degrades
before trust is seeded, while this repo's permission-only file stays clean.
Naming `mcpServers` is deliberately conservative — a key claude ignores
today is a key it may honor tomorrow. An existing file that cannot be
decoded as a top-level JSON object also degrades; failing open there would
turn a classification error into a claim that no executable channel exists.

**Trust keys on the repo root, and worktrees inherit it.** Measured in the
same pass: with only a repo root carrying `hasTrustDialogAccepted`, a
subdirectory of it and a `git worktree add` linked worktree of it both
opened on a live composer, an untrusted sibling repo drew the dialog, and
claude wrote no new `projects` entry for either. The fleet's per-worktree
seed entries are belt, not load-bearing.

**What posse does NOT do, so nobody re-proposes it:** pre-answer the other
runtimes' dialogs from the harness (write `privacy_banner_acked`, write
`version.json`). Rejected in ADR 0013 — coding-data consent is a visibility
line and update-skip is the operator's pin. Posse documents those keys, and
writes no answer the operator could have given once.

