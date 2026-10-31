# bob 2.0.5 → 2.0.9: what a bundle read re-measures for free, and what stays as-of-2.0.5 (ranger-base-kf9ou)

ranger-base-lf6it found bob self-updating 2.0.5 → 2.0.7 under the
ranger-base-lfk7t probe, and counted the cost: the tree names bob 2.0.5 **99
times across 31 tracked files**, every one of them a measurement, none of them
about the binary this box runs. ranger-base-zneyv took the decision to the
operator and the operator pinned it. This fragment is the re-check.

Every reading below is from this box on **2026-10-31**, and every command was a
read: the installed bundle at `/opt/homebrew/lib/node_modules/@ibm/bob/dist/bob.js`,
bob's own session logs under `~/.bob/logs/shell`, and `bob --version`, which
ranger-base-lf6it §6 measured as writing no log and moving no file. **No model
turn, no spend, no `bob chat`.**

**Landed after ranger-base-bs49e, and read with it.** This bead and
ranger-base-bs49e ran the same morning in different worktrees and read the same
bundle without each other; bs49e landed first, and the merge-back
(ranger-base-s85fx) kept its text where the two overlap. Everything below is
what THIS bead measured on 2026-10-31, unamended — but two sentences about the
CURRENT state were overtaken while it was in flight, and both are marked
`[bs49e]` where they appear: the operator did re-run the live `/skills` probe,
on the installed 2.0.7 at 09:38, so `SkillsVerified` is `"2.0.7"` and not
`"2.0.5"`, and the remaining live arm is filed as **ranger-base-0q45n**. The
reasoning this bead gives for not advancing the field off a bundle read is the
reasoning bs49e's 2.0.7 applies; only the number differs.

## 1. The version is 2.0.9, not 2.0.7, and the pin is not what held it

MEASURED: `bob --version` prints `2.0.9` / `commit: 9690f20b0`.
`/opt/homebrew/lib/node_modules/bobshell` is **gone**; `@ibm/bob` 2.0.9 is in
its place, package dir mtime `09:39` local, `package.json` `releaseDate`
`2026-10-23T11:10:21Z`. So **the `bobshell` → `@ibm/bob` rename that
ranger-base-lf6it §5 called armed has fired**, and every check keyed on the old
npm name or the old path — including the path monica's pulse read the mtime of
— is now stale rather than half-stale.

The timeline, from the log mtimes and the logs' own lines (bob logs in UTC, this
box is UTC-4; bd's comment stamps on ranger-base-zneyv are UTC too, which is why
`13:37` there and `09:37` here are the same minute):

```
09:37:02 local  ~/.bob/settings/settings.json mtime — autoUpdate:false lands
09:38:15 local  bob chat in the operator's bobprobe/ws; migrateLegacyBobshell:
                failed — "blocked by running instances: 23267"
09:38:49 local  a second one; migrateLegacyBobshell: backup created, worker
                copied to ~/.bob/settings/bobshell-migration/2.0.7/worker.mjs
09:39    local  @ibm/bob 2.0.9 on disk
```

**The pin held the update check and the legacy migration went around it.** That
is not a surprise the pin failed at — ranger-base-zneyv's own blast radius said
so in advance ("it does not stop the pending `bobshell` → `@ibm/bob` package
rename, see below, which is a migration and not an update") — but it is worth
writing down that the rename's worker installs `@ibm/bob@latest`, so the
migration is also an upgrade, and the first `bob chat` after the pin moved the
binary two patch versions.

**The route is closed and cannot fire again.** MEASURED off the 2.0.9 bundle:
`dist/bobshell-legacy-migration-worker.js` is absent (the bundle ships
`dist/bobshell-release-update-worker.js` instead), and
`migrateLegacyBobshell`, `LegacyMigrationBlockedError` and
`bobshell-legacy-migration-worker` appear **0 times** in `dist/bob.js`.

### The pin's own verification criterion is not checkable as written

ranger-base-zneyv's VERIFIED WILL MEAN was "that session's log carrying
`Skipping update check. Disabled by user setting or policy`". MEASURED: the
module `bob-shell:update-check` logs **zero lines in all nine bob session logs
on this box** — before the pin as well as after. The 00:26:38 install that
started this whole thread is itself invisible in the 00:25 log: no `update`,
`registry`, `npm install` or `new version` line anywhere in it. The skip is a
`.debug()` call on a logger that is not reaching these files.

What does assert the pin is a read of the live bundle. The gate, de-minified:

```js
var REGISTRY = "https://registry.npmjs.org", PKG = "@ibm/bob";
// ... fetch(`${REGISTRY}/${encodeURIComponent(PKG)}/latest`) ...
var log = La("bob-shell:update-check");
async function checkForUpdate() {
  if (!Vn.getUserConfig()?.values.bobShell.autoUpdate)
    return log.debug("Skipping update check. Disabled by user setting or policy"), null;
  /* ... compare, showInfo("Bob Shell update available! …") ... */
}
```

Same config key (`bobShell.autoUpdate`), same message, same registry host,
`UpdateMode` still named as the policy, and the package it fetches is now
`@ibm/bob`. `~/.bob/settings/settings.json` reads `autoUpdate: false` with the
hooks, `licenseConsent` and `approval` keys intact.

And one reading that is a measurement in its own right: `lastRunVersion` is
still `"2.0.7"`. bob writes that key on the first start of a new version
(`if (last !== current) await config.updateAt("bobShell.lastRunVersion", current)`),
so **no 2.0.9 session has ever started on this box**. Every live reading of
2.0.9 is still unmade, and that is the line between the two halves below.

## 2. Re-measured on 2.0.9 (static bundle read, free)

The 2.0.5 recipe said the minified names move between builds and the string
literals do not; that held — every cluster below was found by its literals.

| claim, and where posse keeps it | 2.0.9 |
|---|---|
| `bobSkillsGlobalRoots` (skills.go): `~/.bob`, `~/.agents`, `~/.claude` × {`skills`, `plugins/*/skills`} | **unchanged.** `GLOBAL=[~/.bob,~/.agents,~/.claude]`, `GLOBAL.flatMap(d=>[join(d,"skills"), join(d,"plugins","*/skills")])` |
| workspace skill roots (ADR 0007 §6a, runtime.go:323): `.bob/skills`, `.bob/plugins/*/skills`, `.agents/skills`, `.claude/skills` | **unchanged**, same four, same order |
| `BobSkillsRoot = ".bob/plugins/posse/skills"` is a root bob reads | **still inside `.bob/plugins/*/skills`** — the declaration's static half holds. The LIVE half is §3 |
| persona-mode file glob (ADR 0062 D1, personamode.go): `.bob/{custom_modes.yaml,plugins/*/custom_modes.yaml}` | **unchanged.** `WS="{custom_modes.yaml,plugins/*/custom_modes.yaml}"`, and the global one is `"{settings/custom_modes.yaml,plugins/*/custom_modes.yaml}"` — the asymmetry posse already knew |
| mode slug alphabet `^[a-zA-Z0-9-]+$` | **unchanged**, verbatim in the zod schema ("Slug must contain only letters numbers and dashes") |
| an omitted `groups` is a mode with NO tools | **unchanged.** `groups: GROUPS.optional()` with no default, and the workspace loader iterates `for (const g of m.groups || [])` |
| `command` → `execute` group aliasing | **unchanged**, both in the loader and in the import path |
| `hidden` is a schema field | **unchanged**, `hidden: z.boolean().optional()`; `hiddenFromUser` still a different field |
| `--mode <slug>` on `chat`, default `agent` | **unchanged** |
| the four flags on `BobCommand` — `--accept-license`, `--trust`, `--auto-approve`, `-w/--workspace` | **all four still `bob chat` options.** `--auto-approve` is chat's own; the other three are the shared `chat`/`run` set |
| `Fallback: "Agent Mode"`, `Footer: "%s Mode"` (the D2 pane reading) | **unchanged, and now read off the composing code rather than off a pane.** The footer renders `<mode.name>` then a dim `" Mode"` then, under `--auto-approve`, `" (auto-approve)"`; the built-in agent mode is `{id:"agent", name:"Agent"}`. "Agent Mode" is **not** a literal in the bundle — it is composed — which is why grepping for it answers 0 and means nothing |
| `FallbackSentence: "Unknown mode"` | **unchanged**, verbatim: `Unknown mode "{{modeId}}". Falling back to "agent" mode.` |
| no per-launch model on `chat`/`run` (runtime.go, cmd/posse/tierdisplay_test.go) | **unchanged, and now with the mechanism named**: `-m/--model`, `--provider` and `--reasoning-effort` are added only when `isDev()` is true, and `isDev()` is `FLAG && process.env.BOB_DEV_KEY === <literal>`. Tiers stay unmapped |
| `BobCageCred`, the container credential env NAME, quoted from the CLI's own refusal (spelled in Go and deliberately nowhere in markdown — the ops-residue scan reads a credential env name in prose as a finding, which is why this row describes it rather than quoting it) | **unchanged**: the refusal is still verbatim, `Bob API key is required.` followed by the sentence naming that variable, and the bundle's seven occurrences of the name are the same gateway/dev/support family ranger-base-9k9ff counted |
| `bobHomeIn` (interstitial.go): the credential store is a fixed `~/.bob/settings/auth-secrets.json`, no home override | **unchanged** |
| the license interstitial's quote, "if you have reviewed the license" | **unchanged**, in the sandbox branch of the license screen, which also still names `--accept-license` |
| the sign-in interstitial: `Complete sign-in in your browser…`, `Press ESC or Ctrl+C to exit` | **both unchanged**, verbatim |
| the top-level program's parser shape (ADR 0060 D3's order reasoning, bobruntime_qa_test.go) | `.enablePositionalOptions().allowExcessArguments().allowUnknownOption()` — **unchanged**, so the ORDER reasoning still holds. One change: **`chat` now calls `allowExcessArguments(false)` for itself** |

### The herdr-bob detection fork still keys on strings 2.0.9 draws

`etc/herdr/herdr-bob/rules.json` is posse's fork of the plugin's rules, and its
two measured rules are pairs of line-anchored patterns. The captures in
`etc/herdr/agent-detection/upstream/bob/` are 2.0.5 TUI scrollback and cannot be
re-taken without a live session — but the *strings they key on* are bob's own,
and those are a bundle read. All present in 2.0.9:

| keyed on | rule | 2.0.9 |
|---|---|---|
| `Execute Command` (heading) + `Approve Once` (option row) | `bob_execute_command` | present; the approval dialog's two rows are still `Approve Once` / `Reject` |
| `Complete sign-in in your browser` + `Press ESC or Ctrl+C to exit` | `bob_signin` | both present, verbatim |
| `/permissions — View and change this folder's trust level` | the FALSE POSITIVE posse's fork narrows away | **still there**, `{name:"permissions", altNames:["trust"], description:"View and change this folder's trust level"}` — so posse's override is still the right file and still needed |
| `do you trust` + `this folder` | `trust_folder_prompt`, previously UNFIXTURED | **now corroborated statically**: the screen's title is `Do you trust this folder?`, description "Trusting a folder allows Bob Shell to execute commands it suggests…", footer `Press ESC or CTRL+C to exit`. The rule's positive side was upstream's guess; on 2.0.9 it matches bob's own words |
| `licen[sc]e` + `accept`/`agree` | `license_prompt`, previously UNFIXTURED | **now corroborated statically**: title `IBM License Agreement`, footer `Press y to accept or n to decline · Enter to open the selected file` |
| `esc to interrupt` (a `none:` guard) | three rules | absent from the bundle, as it is absent from all six captures — unchanged, and it has never fired |

Note the trust screen's footer is `Press ESC or CTRL+C to exit`, uppercase
`CTRL`, where the sign-in screen's is `Ctrl+C`. Matching is case-insensitive so
both satisfy `bob_signin`'s second pattern — but `bob_signin` requires the
sign-in line too, which the trust screen does not draw, so the pair holds and
the trust screen resolves at `trust_folder_prompt`. The two-line anchor is doing
exactly the work it was written for.

## 3. Still as-of-2.0.5 — and why each one costs something to re-measure

| claim | why it stays |
|---|---|
| **skills discovery, live** (ADR 0007 §6 Evidence, `SkillsVerified: "2.0.5"`): that bob LISTS a skill at `.bob/plugins/posse/skills/<name>/SKILL.md` in `/skills`, as a real dir AND as a symlink out of the workspace | needs a TUI and a browser sign-in. **ranger-base-bs49e** is already filed with the operator for exactly this; ranger-base-lf6it §2 narrowed it to the symlink arm and the listing itself. `SkillsVerified` is not advanced off a bundle read, so the drift row keeps firing. `[bs49e]` the field reads `"2.0.7"` and the row `drift: skills_verified: "2.0.7" but bob reports "2.0.9"`, because the operator re-ran the probe on the installed 2.0.7 at 09:38 — one hour before this bead read the binary as 2.0.9. The live 2.0.9 arm is **ranger-base-0q45n** |
| **`rules_precedence: pid`** (runtime.go, ranger-base-4mrmc) | a billed model turn — 0.024704 Bobcoins under the operator's authorization on ranger-base-2vr1j. The artifact survives (bob task `77a20a1a8b35b4a1b9b50d95fde15712` in `~/.bob/db/bob.db`) but it is a 2.0.5 reply |
| **`hidden` hides nothing** (ranger-base-4mrmc F1) | read off that same turn's stored prompt. The schema field is still there on 2.0.9; what it does is a reading of a rendered prompt |
| **top-level `-p` is swallowed and delivers nothing** (ranger-base-5jjtn) | the measurement was a pane watched at 5/10/20/30/45s. **Corroborated, not re-measured**: `initialPrompt`, `args.prompt` and `args?.prompt` appear **0 times** in the 2.0.9 bundle, and the chat executor hands the whole args object to the TUI without a first-message path. That is a reason not to re-probe, not a substitute for one — and `DO NOT PUT -p BACK` is unaffected either way |
| **the six TUI captures** in `etc/herdr/agent-detection/upstream/bob/` | pane scrollback of a live session. §2's table is what can be said without one, and it says the rules still match |
| **the ~20s to a signed-in composer** (`StartupWait` deliberately unset) | a single reading, not a measurement, on 2.0.5 and still |

## 4. One declaration came back measured FALSE, and it is fixed here

`bobNativeRules` (runtime.go) declared:

```go
{"AGENTS.md", "CLAUDE.md",
 ".bob/rules-agent/AGENTS.md", ".bob/rules-plan/AGENTS.md", ".bob/rules-ask/AGENTS.md"}
```

2.0.9's RuleLoader, de-minified from the `La("RuleLoader")` cluster:

```js
const BOB = ".bob", COMMON = "rules", MODE_PREFIX = "rules-", AGENTS = "AGENTS.md";
const SKIP = new Set([".DS_Store","Thumbs.db",".gitkeep",".gitignore",".bobignore"]);
const MAX_DEPTH = 5;
async function loadRules(wsRoot, fs) {
  const wsBob = join(wsRoot, BOB), globalBob = join(homedir(), BOB);
  const dirs = await bobDirs(wsBob);            // [wsBob, ...wsBob/plugins/<each>]
  const [agentsMd, wsCommon, wsMode, global] = await Promise.all([
    readIfExists(join(wsRoot, AGENTS), fs),     // the ONLY single file
    Promise.all(dirs.map(d => readTree(join(d, COMMON), fs))),
    Promise.all(dirs.map(d => readModeDirs(d, fs))),   // any dir named rules-<slug>
    loadGlobal(globalBob, fs),                  // the same two arms under ~/.bob
  ]);
  /* … */
}
```

Three things fall out, and none of them is `CLAUDE.md`:

- **`CLAUDE.md` is not read.** `AGENTS` is the only root filename, and all three
  `CLAUDE.md` occurrences in the bundle are prose inside bob's built-in
  "author an AGENTS.md" skill, telling the MODEL to fold Claude Code's and
  Cursor's rules into the AGENTS.md it writes. That is bob advising an author,
  not bob loading a file. A grep for the string finds it; a grep for the loader
  does not.
- **The mode arm is not three fixed paths.** `readModeDirs` takes every
  directory whose name starts with `rules-` and uses the remainder as the mode
  slug, so it is `rules-<any slug>`, not `rules-{agent,plan,ask}` — which
  matters here more than it would elsewhere, because posse's own PID channel
  launches `--mode posse-<persona>`, and `.bob/rules-posse-<persona>/` is
  therefore a rulebook directory for posse's own sessions.
- **There is a common arm and a plugins arm and a global arm, and the
  declaration named none of them.** `.bob/rules/`, `.bob/plugins/*/rules{,-<slug>}/`
  and the same four under `~/.bob` are all read, every filename in them, up to
  five directories deep, following symlinks, skipping only the five names in
  `SKIP`.

Whether that is drift since 2.0.5 or an incomplete reading at 2.0.5 **cannot be
determined on this box**: the 2.0.5 bundle is gone (the rename removed
`node_modules/bobshell`), and the migration left only a `state.json` and the
8.6 MB `worker.mjs` under `~/.bob/settings/bobshell-migration/2.0.7/`, not a
bundle to diff. The honest form is the one taken: the declaration now says what
2.0.9 was MEASURED to read, dated, with the method named.

`NativeRules` is a display field (`fcDisplay`, rendered by `runtimecheck.go` as
the `rulebooks` grid row) and bob is deliberately not in
`qmRulebookTruth` — that table's contract is "what
`TestQALiveNativeRulesDiscovery` measured", and nobody has spent a bob turn on
it — so this correction moves one printed row and no behaviour. The row's pin in
`bobruntime_qa_test.go` moves with it, which is the point of having one.

## Filed, not fixed

- **ranger-base-bs49e** (operator, `-l question`) — the three discovery arms,
  live. §3 is why it is still the only route to `SkillsVerified`, and it now has
  one less thing to check: not the `Skipping update check` log line (§1).
  `[bs49e]` CLOSED: the operator ran it on the installed 2.0.7 at 09:38 that
  day, all three arms listed, and `SkillsVerified` moved to `"2.0.7"`. The same
  wall for 2.0.9 is **ranger-base-0q45n**, which §2 narrows to the symlink arm
  and the listing — never the roots.
- **ranger-base-f5jbs** (`-l code`) — the exit-code surface for
  `skillsVerifiedDrift`. Unchanged by any of this, and §1 is a second argument
  for it: the version moved twice in one day, both times without anybody typing
  an upgrade, and both times the tree learned of it from an mtime.
