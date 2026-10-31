# ADR 0007 — Skills binding: the PID declares, launch materializes per runtime

*Status: accepted 2026-08-18 · owner: architect · verified 2026-08-18
(rangerhq-1qd, codex-cli 0.147.0 and grok 1.0.5: the `-c` and `--agent`
candidates are dead ends; `<cwd>/.agents/skills/` is the only surface
either CLI has, and it is the same one — §2) · amended 2026-08-28
(rangerhq-3zr: a bound `SKILL.md` with no `description:` binds to nothing
on codex and is a finding — §2) · amended 2026-09-05 (ranger-base-65rc:
the claude plugin tree COPIES each skill in rather than symlinking to it
— §2) · amended 2026-09-06 (Context: the two superseded runtime rows
collapse into their verified answers — ranger-base-mppjc) · amended
2026-10-12 (ranger-base-q1w0h, github issue 12: a cwd root is a declared,
version-verified fact per runtime, or that runtime's skills are UNPROVEN;
bob gets a posse-owned root; the `$HOME` position of ranger-base-q114b is
reversed — §6) · amended 2026-10-31 (ranger-base-6r1rf: bob's root is
MEASURED live and no longer UNPROVEN — §6a, Evidence; re-measured on the
installed 2.0.7 the same day by ranger-base-bs49e, which is what `verified`
names now) · amended 2026-10-31 (ranger-base-kwbh1: a cwd root may
also be a fact per DIRECTORY — grok's is gated on folder trust, declared and
probed rather than seeded — §6f)*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> The runtime-surface facts below were verified on that instance's
> machine at the CLI versions named; re-verify on version bumps.

## Context

DIRECTION.md's fourth pillar: runtimes own skill *loading*; posse owns
the cross-agent, cross-project *binding* — persona X gets these skills
whether it launches as claude or codex. Today a persona's skills are
whatever this user installed into this runtime's global config — a
per-user-per-runtime accident, not part of the persona, and not part of
the diff a reviewer reads.

Facts checked on this machine (2026-08-18):

| runtime | skill artifact | global location | per-session surface |
|---|---|---|---|
| claude | `<name>/SKILL.md` dir | `~/.claude/skills/`, project `.claude/skills/`, plugins' `skills/` | **`--plugin-dir <dir>`** (repeatable, session-only; a dir with `.claude-plugin/plugin.json` + `skills/`). `--add-dir` is *CLAUDE.md dirs* — it does not load skills |
| codex | `<name>/SKILL.md` dir | `~/.codex/skills/` (10 present) | **`<cwd>/.agents/skills/`, and no flag** — the `-c` candidate is a real config table with no root field, verified 2026-08-18 (§2) |
| grok | `<name>/SKILL.md` dir | `~/.grok/skills/` (8 present) | **the same `<cwd>/.agents/skills/`, and no flag** — the `--agent` candidate carries no skill-path field, verified 2026-08-18 (§2) |
| bob | `<name>/SKILL.md` dir | `~/.bob/skills/`, `~/.bob/plugins/*/skills/`, **and `~/.agents/skills/`, `~/.claude/skills/`** | no flag; the workspace's `.bob/skills/`, `.bob/plugins/*/skills/`, `.agents/skills/`, `.claude/skills/` — static read of 2.0.5, re-read unchanged on 2.0.9 (ranger-base-bs49e), and `.bob/plugins/posse/skills/` MEASURED live on 2.0.5 and again on 2.0.7 (§6 Evidence, ranger-base-6r1rf and ranger-base-bs49e) |

*(Amended 2026-09-06, ranger-base-mppjc: the two rows above read
"none verified: candidates …" until this date, and were then followed by a
line saying §2 had answered them — a table of open questions with the
answers one hop away, which is how a reader ends up citing the candidate.
The rows now carry the verified answer; §2 keeps the evidence, the CLI
versions and what was tried. The claude row was verified the same day and
never said otherwise.)*

The artifact is the same everywhere — the Agent Skills `SKILL.md`
directory — so the binding can be one dir of skills and per-runtime
*pointing*, never conversion. The launch path is real code now:
`RenderCommand` (runtime.go) renders placeholders through per-runtime
realizers; `WrapWithGates`/`CheckParity` (gates.go, parity.go) render
per-persona state under `RHQ_HOME/state/` at every launch and refuse a
launch whose declarations can't be realized unless `--allow-degraded`.
Skills bind into exactly that shape.

## Decision

**1. Skills live in `RHQ_HOME/skills/<name>/SKILL.md`; the PID names them.**
One directory of Agent-Skills dirs (real dirs or symlinks to
`~/.claude/skills/x`, a plugin's `skills/x`, a repo — posse doesn't
care). Frontmatter key `skills:` (flat-YAML list of names):

```yaml
skills: [dataviz, code-review]
```

A name resolves to `RHQ_HOME/skills/<name>/SKILL.md` or it is unknown.
posse indexes nothing and copies nothing INTO this dir: `posse skills list`
is `ls`, `posse agent check` is `stat`. (What a launch renders OUT of it is
§2's question, and one of the two shapes there does copy.)

**2. Launch materializes per runtime through a skills realizer**, next
to the tool realizer, and a new template placeholder **`{skills}`**:

| runtime | materialization | `{skills}` renders |
|---|---|---|
| claude | `RHQ_HOME/state/skills/<persona>/claude/` rendered fresh at launch (like gates): `.claude-plugin/plugin.json` `{"name":"posse-<persona>","description":"skills bound by posse"}` and `skills/<name>` — a real directory of files, COPIED from `RHQ_HOME/skills/<name>` (amended 2026-09-05, below) | `--plugin-dir <that dir>` |
| codex | `<session dir>/.agents/skills/<name>` → symlink to `RHQ_HOME/skills/<name>`, excluded via `.git/info/exclude` (verified, rangerhq-1qd) | nothing — there is no flag |
| grok | the same `.agents/skills/<name>` symlinks (verified, rangerhq-1qd) | nothing — there is no flag |
| bob | `<session dir>/.bob/plugins/posse/skills/<name>` → symlink to `RHQ_HOME/skills/<name>`, posse's own namespace (§6d); verified `2.0.7` (§6's live probe, ranger-base-6r1rf, re-run on 2.0.7 by ranger-base-bs49e) | nothing — there is no flag |
| `runtimes/*.yaml` | optional `skills_flag:` (`--foo %s` given the rendered dir), or `skills_cwd: <root>` + `skills_verified: <version>` (§6); absent → unrealizable | |

*Amended 2026-09-05 (ranger-base-65rc): the flag tree binds each skill by
COPYING it in, not by symlinking to it. The symlink made this row's promise —
"what sits inside it is the universal Agent-Skills layout" — a claim about the
reader rather than about the tree, because a CLI that does not follow a link
out of a plugin root finds a `skills/` dir with nothing in it. Measured on
grok 1.0.5, the one non-claude CLI available that reads the claude plugin
shape: `plugin validate` passed, `plugin install --trust` installed one
plugin, and `inspect` reported `Skills (0)`; the same tree copied with
`cp -RL` reported both skills. claude dereferences, so the surface had been
exercised on exactly the CLI that hides the defect — and a `skills_flag:`
runtime whose loader behaves like grok's launched CLEAN, with the binding in
Realized and the persona holding nothing, which is §3's refusal arriving
through the accepted path. The copy is a launch cost paid where the gates
render is (a skill dir is kilobytes), it keeps each file's mode so a skill
that ships a script still runs it, and it closes the same hole in the
container tier, which mounts this tree and not `RHQ_HOME/skills`. The cwd
shape below is unchanged and stays symlinked, deliberately: that dir belongs
to the operator's repo (see the rejected alternative), both CLIs were measured
following links out of it, and the never-clobber rule tells posse's entry from
the operator's by reading a link's target. Pinned hermetically by
`TestQARenderedTreeNeedsNoSymlinkFollowed` — a walk that refuses every symlink
and must still find each bound skill whole — and live by
`TestQALiveSkillDiscoveryPerRuntime`, which installs the rendered tree into a
redirected HOME and asks `grok inspect` what the persona would have had; both
in `internal/posse/skillsrules_parity_qa_test.go`.*

**Verified 2026-08-18 (rangerhq-1qd), codex-cli 0.147.0 and grok 1.0.5.**
The `-c` / `--agent` candidates are both dead ends; the cwd fallback is not
a fallback but the only surface either CLI has, and it is the *same* surface
for both, so one realizer serves them:

- **codex.** `skills` is a real config table (`-c skills=1` → "expected
  struct `SkillsConfig`"), but its only field is `bundled.path`, which
  overrides where the shipped system skills live and adds no root; the
  app-server's `skills/extraRootsSet` JSON-RPC method is not reachable from
  the CLI, and unrecognized `-c` keys are dropped silently. It *does* read
  `<cwd>/.agents/skills/<name>/SKILL.md` (and `<cwd>/.codex/skills/` —
  `.agents` is the vendor-neutral one), follows symlinks out of the repo,
  and keeps reading them under every flag the fleet types (`--disable
  hooks`, `allow_login_shell=false`, the trust table, `-s read-only`).
  Discovery is at the cwd only: it does not climb to the repo root.
- **grok.** `[skills] paths` exists, but only in `config.toml`: the
  `GROK_CONFIG` / `GROK_CONFIG_PATH` overlay — the one config layer a
  launcher can inject — is allowlisted to `models`, `features`, `toolset`
  and `shell_environment_policy` and "cannot add a discovery source"
  (verified: the overlay leaves the skill list unchanged). `--agent <file>`
  carries no skill-path field either: a definition with `skills:` /
  `skill_paths:` parses and binds nothing (checked against a headless
  session's `init` line). It reads `<cwd>/.agents/skills/` as `project`
  scope, and skill discovery deliberately ignores git's ignore rules — so
  `.git/info/exclude` hides the dir from `git status` without hiding it
  from the CLI.

Two consequences of the cwd shape, both accepted: `{skills}` renders
*nothing* on codex and grok (the symlinks are the realization, not a flag),
and the dir belongs to the **repo**, not to the persona — so the launch
adds its own links, never removes another persona's, and refuses rather
than overwrite an entry posse did not write. Union semantics is what §4
already licenses: posse guarantees presence, not absence. One entry is
exempt from the refusal: a **dangling** symlink — a link whose target is
gone — binds nothing and is a relic of a moved or retired home rather
than the operator's work, so the launch replaces it (ranger-base-f6hiy: a
link into the pre-ADR-0015 `~/.config/rhq` survived the cutover and refused
every launch of the persona binding that skill until it was removed by
hand). "Gone" is two errnos and only two: `ENOENT`, a component of the
target path is not there, and `ENOTDIR`, a component of it is an ordinary
file — the retired home archived in place rather than deleted, which cannot
resolve now and cannot resolve later either (ranger-base-epdyv). Every
other failure is about the *path* and not the target — a loop, a parent
this uid cannot traverse, a live target spelled with a trailing slash — so
it is no evidence of a relic and it still refuses.

Empty `skills:` renders nothing (the placeholder vanishes with its
space, like `{allow}`). Env always carries `RHQ_SKILLS_DIR`
(`RHQ_HOME/skills`) and `RHQ_SKILLS` (newline names) — the exit hatch: a
runtime posse can't point can still be told where they are, by the PID
body or a wrapper.

**3. Declared means required — enforcement parity applies.** `skills:` on
a PID is a statement that the persona's work depends on them. If the
chosen runtime's realizer reports unrealizable, `CheckParity` adds
`skills: <names> — <runtime> has no per-session skill surface` to
`Degraded`; the launch refuses unless `--allow-degraded`, and a degraded
session shows it in the cockpit exactly as an unrealized gate does. A
skill the persona would merely *like* stays in the runtime's global
config — that path is not removed, only made irrelevant to the persona.

**4. Binding is additive, not isolating.** The runtime's global skills
still load; posse guarantees presence, not absence. (Isolation —
"this persona sees only its skills" — would need `--setting-sources` and
per-runtime equivalents and is a cage question, ADR 0002's tier, not a
binding question. Named out of scope.)

**5. `posse agent check` lints:** unknown skill name (no `SKILL.md`) is a
finding; a PID whose own `command:` lacks `{skills}` while `skills:` is
non-empty is a finding (ORDERS lesson: never leave a token unrendered,
never silently skip one either — same rule as `{model}`).

*Amended 2026-08-28 (rangerhq-3zr): a third finding of the same kind — a
bound `SKILL.md` whose frontmatter carries no `description:`. `name` and
`description` are the two required Agent-Skills keys, and codex renders
each discovered skill as one `- <name>: <description>` line and drops one
without a description entirely (verified codex-cli 0.147.0), so the name
resolves, the launch materializes the link, parity calls it realized, and
the persona has nothing. claude and grok fall back to the body, which is
what makes it silent: the same binding works on two runtimes out of
three.*

**Cage interplay.** Seatbelt denies writes only — reading
`RHQ_HOME/skills` is fine; the container tier mounts it read-only. Skills
that ship scripts run *inside* the cage like anything else.

**6. A cwd root is a declared, version-verified fact — or the runtime's
skills are UNPROVEN** (amended 2026-10-12, ranger-base-q1w0h, github
issue 12). §2 hard-coded one root, `.agents/skills`, for every runtime with
no flag, and bob inherited it from ADR 0060's recon row without a
measurement. The operator then MEASURED on the work box that the bob there
reads `.bob/skills/<name>/SKILL.md` and that `.agents/skills` was neither
present nor read — so a bob PID naming a skill linked into a path that
CLI may not read and parity called it realized: the grok shape of 65rc,
arriving through a declaration nobody verified. A binding whose surface is
the filesystem fails silently by construction; the only defence is to
say, per runtime and per version, which root was measured.

- **6a. Declaration.** The bool `SkillsCwd` / `skills_cwd: true` becomes a
  root: `skills_cwd: <path relative to the session dir>` (the literal
  `true` keeps meaning `.agents/skills`, so instance overlays parse) and
  `skills_verified: <the CLI's --version line at the live measurement>`.
  The built-ins carry: codex `.agents/skills` verified `codex-cli 0.147.0`
  (moved to `0.153.4` by ranger-base-bdias); grok `.agents/skills` verified
  `grok 1.0.5`; bob `.bob/plugins/posse/skills` verified **`2.0.7`**
  (ranger-base-6r1rf, 2026-10-31 — it read **no verified root** until then;
  moved 2.0.5 → 2.0.7 the same day by ranger-base-bs49e's re-run, 6c's own
  remedy applied to a reading that shipped in drift).
  The value is the CLI's own line, the way that CLI prints it — bob's is the
  bare number, where codex's carries a program name — **minus any trailing
  bracketed field, which is box state and not the release** (ranger-base-3a5z7,
  2026-10-31). grok's is the one that has one: `grok 1.0.52 (0920c8169e27)`,
  where this box's `grok --version` says `... [stable]`. §6c drops that field
  from both sides of its compare, so the declaration is the release and the
  box's own channel cache cannot decide a drift.
- **6b. Unverified is UNPROVEN, and UNPROVEN is §3.** A cwd root with no
  `skills_verified` is unrealizable: `CheckParity` adds `skills: <names> —
  <runtime>'s discovery root <root> is UNPROVEN (never measured live)` and
  the launch refuses unless `--allow-degraded`. That was bob from this
  amendment until 2026-10-31, by design — the honest state, louder than a
  link into a dead path. Every built-in that HAS a cwd root carries a
  verified one now; the rule stands for the next `skills_cwd:` runtime an
  instance declares, and an instance overlay that declares one is exactly
  where it bites.
- **6c. Drift is REPORTED, not refused.** `skills_verified` is compared to
  the installed CLI by the same reader the runtime probe's drift check
  already uses (`ProbeCLIVersion`, runtimeprobe.go); a mismatch is a line in
  `posse runtime check` and at launch naming both versions and the probe to
  re-run. It does not refuse: the remedy is a re-measurement, codex alone
  moved 0.147.0 → 0.153.4 in eight weeks, and a wall at every brew upgrade
  decides only who types `--allow-degraded`. Flag surfaces (`--plugin-dir`,
  `skills_flag:`) carry no `skills_verified`: a removed flag fails the
  launch loudly, which is the property a cwd root lacks.
  **The read is BOUNDED** (ranger-base-fnn1o): `ProbeCLIVersion` gives the
  `--version` child `cliVersionTimeout` (5s) and reads `""` past it, which
  §6c already disposes of — an unknown version is a Skip, never an unchanged
  one. It had no deadline at all until the first UNATTENDED reader arrived:
  `posse runtime drift` on `scripts/verify-box.sh`'s ROSTER
  (ranger-base-gf0ln) runs under launchd, where no second instance starts, so
  a CLI that blocks on `--version` wedges the whole daily run rather than one
  person's launch.
  **The compare is on the RELEASE, not the raw line** (ranger-base-3a5z7):
  `skillsVersionRelease` (parity.go) drops a trailing bracketed field from
  each side first, because one CLI's `--version` line carries a field that
  belongs to the box and not to the binary — grok prints `[stable]` under a
  $HOME whose `~/.grok/version.json` exists and nothing under a fresh one,
  the same 1.0.52 both times (MEASURED 2026-10-31, bisected file by file;
  `docs/notes.d/ranger-base-3a5z7.md`). Comparing raw had no right answer:
  declaring the field reds every container tier and fixture-HOME seat forever,
  dropping it reds the box that runs `scripts/verify-box.sh`, and no
  re-measurement clears either — a permanent false finding standing beside the
  real ones, which is the failure the exit code was built to avoid
  (ranger-base-f5jbs). It is one rule, not a per-runtime parser: codex's
  `codex-cli 0.153.4` and bob's `2.0.7` have no bracketed field, so their
  canonical form is the raw line byte for byte, and a release move still reds
  with the field present or absent — the version and the build hash are both
  ahead of the bracket. What stops being drift is a channel change alone,
  which §6 was never keyed on. A declaration may not carry the field at all
  (`TestNoBuiltinDeclaresASkillsVerifiedCarryingBoxState`), so the rule is
  enforced on both sides and not only on the read.
- **6d. Ownership.** Two kinds of root, declared by which the runtime reads:
  - *posse-owned* — where the runtime reads a glob of namespaces, posse
    writes only inside its own: bob's target is `.bob/plugins/posse/skills/`,
    the namespace ADR 0062 D1 already gave posse's mode file (same exclude
    line, same never-write-at-`GlobalRoot` guard). The repo's own
    `.bob/skills/` and `.agents/skills/` are never touched, so nothing
    shadows the team's skills and nothing of the team's is in posse's dir.
  - *shared* — `.agents/skills` on codex and grok, the only surface they
    have. The §2 rules stand and are now the whole contract: an entry is
    posse's **iff it is a symlink whose target is under `RHQ_HOME/skills`**
    (that target is the provenance, readable with `readlink`); posse adds
    only; it never overwrites an entry that is not posse's (a team skill of
    the same name refuses the launch — that is the collision rule, and it
    runs in the direction that protects the repo); it replaces only a
    dangling entry; it removes nothing. **Cleanup** is the seat's tree: a
    dispatched worktree retires with its links; in a shared checkout they
    persist under union semantics (§4) and that is declared, not leaked.
    A same-named skill in the runtime's *global* root is the runtime's
    precedence to resolve and is out of posse's contract (§4).
- **6e. A home-level render is refused** — the reversal of ranger-base-q114b
  (Consequences). The static read of bob 2.0.5's bundle on this box names
  `~/.bob`, `~/.agents` and `~/.claude` as GLOBAL skill roots, each with
  `skills/` and `plugins/*/skills`. So a cwd render at `$HOME` — by ANY
  runtime — lands in a root bob reads in every session on the box: the
  mode file's case exactly. Each runtime declares its measured global roots
  (`SkillsGlobalRoots`, beside `PersonaModeChannel.GlobalRoot`), and the
  cwd render refuses when its target lies under any declared runtime's
  global root, before writing. The pin that held q114b's position read a
  declaration nobody had made, so it stayed green while the fact flipped;
  its replacement reads the declared roots, and a root reaches that
  declaration only through a measurement with a version on it.

**6f. A cwd root may carry a declared PRECONDITION on the session
  directory — asked of the CLI, never modelled, and never seeded**
  (amended 2026-10-31, ranger-base-kwbh1, from ranger-base-9f2o6). 6a–6c make
  a cwd root a fact per RUNTIME and per CLI VERSION. grok's is also a fact per
  DIRECTORY, and nothing here declared that.

  MEASURED 2026-10-31, grok 1.0.52 (0920c8169e27), darwin 25.6.0, two scratch
  git repos with IDENTICAL content under ONE fixture `$HOME` whose
  `trusted_folders.toml` named only the first, so the arms differ in the trust
  entry alone:

  | | trusted repo | untrusted repo |
  |---|---|---|
  | `Project trusted:` | yes | no |
  | `Skills` | `(1) qm-alpha project` | `(None)` |
  | Permissions `Source` | `<repo>/.claude/settings.json` | `(none)` |
  | Permissions loaded | 2 | 0 |

  So an untrusted session directory reads NONE of the links the launch wrote,
  says nothing about why, and leaves the realized `skills:` binding silently
  empty: §3's refusal arriving through the accepted path, which is
  ranger-base-65rc's defect with a different cause (there the plugin root was
  unreadable; here the root is right and the folder is not trusted). A session
  worktree and a subdirectory both inherit the enclosing checkout's grant, which
  is why every grok launch on this box has had its skills all along, and why
  nobody found this from a red.

  - **Declared.** `SkillsDirTrust` on the Runtime (What · Where · Grant ·
    Probe). Built-in only, with no `runtimes/*.yaml` key: a declaration
    without a probe is a permanent UNKNOWN, UNKNOWN degrades nothing (below),
    so the key would buy an onboarder a sentence that reads like a guarantee
    and changes no launch. It is worth adding the day a second runtime has
    both a measured precondition and a probe posse can write.
  - **Probed by ASKING the CLI.** `grok inspect` — "Show the configuration
    Grok discovers for this directory" — prints `Project trusted: yes|no`, and
    MEASURED on this box: 0.061s, exit 0, no network, `~/.grok` byte-identical
    before and after. Reading `trusted_folders.toml` and deciding here was
    rejected on a measurement and not on taste: in a subdirectory of a linked
    worktree, with no entry for that path and no trusted ancestor of it in the
    table, grok answers `yes` — it resolves the GIT ROOT and then the main
    checkout, which is what the table names — and the key must be the path
    grok RESOLVES (`/private/var/...`, never `/var/...`), where the unresolved
    spelling matches nothing and reads exactly like no entry at all. A second
    implementation of that rule would disagree silently and in the direction
    that matters: reading "trusted" over a folder the CLI will not read.
  - **NOT MET degrades; UNKNOWN never does.** An untrusted directory takes the
    skills row OUT of `Realized` and adds a `Degraded` line, because a
    Realized row over a root the CLI will not read IS the lie 65rc landed on.
    Degraded and never Unrealized, like every other skills arm — nothing here
    is a wall claim. UNKNOWN (no CLI, a cage, a release that reworded its own
    line) leaves the verdict alone and prints the reading: posse refuses on a
    reading, never on ignorance (ranger-base-9r33, the rule DangerUnsilenced's
    UNKNOWN exclusion already settled). §6b outranks §6f — an UNPROVEN root
    has already degraded the launch with the louder sentence.
  - **POSSE DOES NOT SEED THE GRANT**, and that is the decision rather than
    the gap. The three sibling grants are the contrast: codex's `trust_level`
    and bob's `--trust` are ARGV — per directory, session-scoped, visible in
    `ps`, nothing written to the operator's box — and claude's
    `hasTrustDialogAccepted` is written because without it the session parks on
    a modal and never becomes promptable, so that seed buys the LAUNCH and
    claude offers no line to type it on (ADR 0002 §4 amendment, trust.go).
    grok has neither shape: no flag and no overlay (§2 measured the
    GROK_CONFIG overlay allowlisted to four tables, unable to add a discovery
    source), so the only channel is a PERSISTENT, box-wide entry in the
    operator's own file that outlives every session wanting it; the launch it
    would buy is not dead, only skill-less; and the grant is **not scoped to
    skills** — the same two-arm run measured folder trust also deciding
    whether grok reads the repo's OWN `.claude/settings.json` as a permissions
    source, a repo→box authority channel grok's built-in declares nothing
    about (`ProjectConfig` is nil, which `projectConfigRow`'s own `missing:`
    line calls the silent one). Writing a grant on the operator's box that
    opens a channel nobody has measured is not posse's to do before somebody
    measures it; filed as **ranger-base-1g8gf** (P1, security lane). So the
    remedy is the OPERATOR's, once per checkout, and the degrade line says so.
  - **Where it is READ, and where it is not.** The reading is per DIRECTORY,
    so it lives at the dir-aware parity layer beside `ProjectConfigTrust` —
    the launch and `posse gates` in the session dir. `posse runtime check`
    prints the DECLARATION and not a reading, because that grid is a runtime ×
    box statement that never invents a cwd (CheckParity's own rule), and
    `posse runtime drift` is untouched: its subject is the CLI VERSION and its
    exit codes are a box roster row (ADR 0007 §6c, ranger-base-f5jbs), while a
    directory is not a property of the box.

**Evidence** (2026-10-12, this box, Darwin 25.4.0):
- MEASURED (static — a read of the shipped bundle, not a session):
  `/opt/homebrew/lib/node_modules/bobshell/dist/bob.js`, `bob --version`
  2.0.5. Workspace roots `[".bob/skills", ".bob/plugins/*/skills",
  ".agents/skills", ".claude/skills"]` joined under each workspace folder;
  global roots `~/.bob`, `~/.agents`, `~/.claude` × `{skills,
  plugins/*/skills}`. Recipe: `grep -o '.\{150\}rXo=.\{300\}'` and
  `grep -o 'nXo=Afr.flatMap.\{200\}'` over that file; the minified names
  move between builds, the string literals do not.
  **RE-READ 2026-10-31 on 2.0.9 (ranger-base-bs49e): both root sets are
  unchanged. The file's PATH and the minified names are not.** The global
  package migrated `bobshell` → `@ibm/bob` (ranger-base-lf6it §5, fired
  09:39 that day), so the file is
  `/opt/homebrew/lib/node_modules/@ibm/bob/dist/bob.js` and `command -v bob`
  is the reading that survives the next rename. `rXo=` is `xds=` and
  `nXo=Afr.flatMap` is `Sds=Cmr.flatMap`, with `NA=".bob"` and
  `tJ="plugins"`; recipe `grep -o -E 'xds=\[.{200}'`, which prints both sets
  in one match. This is the arm the next probe does NOT have to re-run — a
  static read cannot speak to the readdir/stat behaviour UNDER a root, which
  is arm b and is the binding.
- MEASURED: `~/.agents/skills/distributed-systems` exists on this box, a
  posse link created 2026-09-05 — under 6e it is a global bob skill today.
  It is the operator's home: reported, not removed.
- MEASURED: installed codex-cli 0.153.4 and grok 1.0.5 against verified
  0.147.0 and 1.0.5 — codex is in drift under 6c the day it ships.
- ~~ASSUMED~~ **RESOLVED 2026-10-31 (ranger-base-6r1rf):** that bob reads
  `.bob/plugins/posse/skills/<name>` live and follows a symlink out of it.
  MEASURED by the operator on bob **2.0.5** (`commit: 2dc180906`) — the
  probe needs a browser sign-in and a writable `~/.bob`, neither of which a
  seatbelt seat has (ranger-base-lfk7t). `bob chat --workspace <ws> --trust`
  in a scratch workspace, read off the TUI's own `/skills` listing with no
  turn sent (no spend): a skill at `<ws>/.bob/plugins/posse/skills/<name>/SKILL.md`
  is LISTED as location *local* both as a real dir and as a symlink whose
  target is OUTSIDE the workspace, and so is the control at
  `<ws>/.bob/skills/<name>`. Both arms matter: posse renders this root as
  symlinks into `RHQ_HOME/skills`, so the symlink arm IS the binding and a
  real-dir-only result would have forced the copy-in shape claude's flag
  tree uses. **RE-RUN 2026-10-31 09:38 on the installed bob 2.0.7
  (`commit: e1a2a9f87`, ranger-base-bs49e), 6c's own remedy for the drift
  the first reading shipped in: the same three arms, the same method, all
  three LISTED again.** So bob's built-in declares `SkillsVerified: "2.0.7"`
  (runtime.go) and a `skills:` PID on bob no longer degrades. The work
  box's bob version is still unmeasured and out of this probe's reach, and
  the probe's own box is unstated — what the declaration keys on is the
  VERSION, which is the whole of §6.
  Method and both arm-by-arm readings: `docs/notes.d/ranger-base-6r1rf.md`.
- MEASURED 2026-10-31 (ranger-base-6r1rf), this box: installed bob was
  **2.0.7** (`commit: e1a2a9f87`) against a declaration of 2.0.5, so bob's
  own declaration was in drift under 6c the day it shipped — the codex case
  again, reported and never refused. Re-measurement was filed for the
  operator as ranger-base-bs49e, because this cage cannot run the probe at
  all.
- MEASURED 2026-10-31 (ranger-base-bs49e), this box: **the re-measurement
  did not clear the clock, and 6c's remedy is still the right one.** bob
  replaced itself again at 09:39, sixty seconds after the 2.0.7 probe
  exited — `bob --version` **2.0.9** (`commit: 9690f20b0`), package
  `@ibm/bob` — so a declaration of 2.0.7 reads as drift the day IT ships
  too. This is the second consecutive probe overtaken by bob's own
  replacement (ranger-base-lf6it §1 is the first), and the reason the
  declaration is still honest: node runs `dist/bob.js` from memory, so a
  swap underneath a live process cannot change what that session measured.
  **The field holds at the release a LIVE probe produced, never at what the
  box carries** — that is §6's whole subject, and declaring 2.0.9 here would
  be asserting a discovery nobody has run. What makes this bounded rather
  than a treadmill: the 2.0.9 static re-read above settles the roots, so
  ranger-base-0q45n has only arm b and the listing left, and the move was
  the one-shot `bobshell` → `@ibm/bob` MIGRATION, which
  `bobShell.autoUpdate: false` does not gate (set 09:37, before the probe —
  ranger-base-zneyv said so in its own blast radius) and which cannot fire
  twice.
- MEASURED 2026-10-31 (ranger-base-kwbh1), this box, darwin 25.6.0, grok
  **1.0.52 (0920c8169e27)**: §6f's two-arm table above, plus three readings
  that shaped the mechanism. (1) `grok inspect` costs 0.061s, exits 0, reaches
  no network and leaves `~/.grok` byte-identical (a stat census over every file
  under it, before and after) — cheap enough for the launch path. (2) In
  `<a linked worktree>/.tmp/deep`, with no `trusted_folders.toml` entry for
  that path and no trusted ancestor of it, grok answers `Project trusted: yes`
  and names the WORKTREE as its git root: trust resolves through the git root
  to the main checkout, so one acceptance per repo covers every seat posse
  opens in it — and modelling that rule here would be a second implementation
  that disagrees silently. (3) `grok --version`'s trailing channel suffix is a
  function of `$HOME`, not of the binary — `grok 1.0.52 (0920c8169e27)
  [stable]` under the operator's home and `grok 1.0.52 (0920c8169e27)` under
  any fresh one — so grok's own `skills_verified:` declaration reports §6c
  drift forever on a fresh-`$HOME` seat (a container tier, a second box).
  Filed as ranger-base-3a5z7; §6c reports and never refuses, so the cost is a
  permanently red detective-tier row beside the real findings.
  Method, full output and the mutation checks: `docs/notes.d/ranger-base-kwbh1.md`.
- **RESOLVED 2026-10-31 (ranger-base-3a5z7):** reading (3) above — which file
  under `~/.grok` supplies the channel suffix, and what §6c should do about it.
  Bisected one file at a time into an empty `$HOME/.grok` against the same
  binary, it is **`~/.grok/version.json`**, grok's own update-check cache, here
  `{"version":"1.0.70","stable_version":"1.0.70","checked_at":...}` — a record
  of the channel's newest release and not of the 1.0.52 that printed the line
  (`config.toml` pins `maximum_version = "1.0.52"`, and `config.toml` alone
  produces no suffix). So the field is not merely `$HOME`-dependent but *cache*-
  dependent: deleting that one file changes this box's own string with no binary
  change, and grok rewrites it on its next check. kwbh1 recommended dropping the
  suffix from the declaration alone; MEASURED against `posse runtime drift`
  before the change, that would have moved the permanent red off a container
  tier and onto the box that runs `scripts/verify-box.sh`, where grok was the
  one green cwd-root row — an exact compare has no value that is right for both
  populations. So §6a and §6c above both key on the RELEASE, and both arms are
  now verified end to end on the same binary: grok ticks under the operator's
  HOME (the tick naming both strings) and under a fresh HOME (identical lines).
  Method, the bisect and the mutation checks: `docs/notes.d/ranger-base-3a5z7.md`.
- ASSUMED: whether codex 0.153.4 reads `~/.agents/skills` globally — the
  binary's strings were inconclusive. Re-verified with the codex drift.
- **RESOLVED 2026-10-12 (ranger-base-bdias):** both the drift and the
  ASSUMED line above, by the same re-run. `TestQALiveSkillDiscoveryPerRuntime`
  (RHQ_PARITY_LIVE=1) against the installed 0.153.4 still passes — the cwd
  root and the symlink-out both hold, unchanged from 0.147.0 — so
  `SkillsVerified` moves to `codex-cli 0.153.4` (6c's remedy is a
  re-measurement, not a refusal). And codex 0.153.4 DOES read
  `~/.agents/skills` as a global root: `codex debug prompt-input` with
  `$HOME` pointed at a fixture and the cwd a bare, unrelated repo lists
  `$HOME/.agents/skills` in its own skill-roots table and loads a skill
  planted only there; with a skill at both `$HOME` and the cwd, both are
  listed and both load — union, not override, the same shape 6e already
  gives bob. codex's built-in now declares `SkillsGlobalRoots:
  []string{AgentsSkillsPath}` (runtime.go) and joins bob as an owner of
  this path under the "any runtime" sweep in `refuseSkillsGlobalWrite` —
  which is exactly the fact the old Consequences paragraph below named as
  the one that would red an arm in `skillshomedir_qa_test.go` rather than
  drift quietly; it did, and that arm (ARM 1) is updated to match. grok is
  untouched — out of this bead's scope, still ASSUMED. Full method and
  recipe: `docs/notes.d/ranger-base-bdias.md`.

## Consequences

- `runtime.go`: `Skills func(dir string, names []string) (flag string, ok bool)`
  per built-in; `{skills}` in the three templates. `skills.go` (~80 lines):
  `SkillsDir`, `ResolveSkills`, `RenderClaudeSkills`. `agents.go`:
  `Skills []string`. `parity.go`: one more row. `pidcheck.go`: three
  findings (the third added by rangerhq-3zr). `CreateSession`: render + env. Scaffold gains `skills:` with
  a hint.
- The claude path is buildable today; codex/grok are verify-then-build in
  a separate bead so the first bead doesn't wait on their answers
  (rangerhq-1qd — both landed on the `.agents/skills` surface; `skills.go`
  gained `RenderAgentsSkills` and `runtime.go` a `SkillsCwd` flag for the
  runtimes that discover from the cwd instead of from a flag).
- **REVERSED 2026-10-12 by §6e (ranger-base-q1w0h)** — the paragraph below
  rested on "no CLI reads `~/.agents` as a global root", and bob 2.0.5's own
  bundle reads it. Kept as the record of what was believed and why.
  **A session dir of `$HOME` renders this tree to `~/.agents/skills/`, and
  that is accepted** (ranger-base-q114b, 2026-10-03). `posse new` has no
  worktree option, so its dir is `--dir`, else `default_dir`, else `$HOME`,
  and one interactive launch with no `--dir` reaches it with nothing
  misconfigured — as it already did (the tree was created 2026-09-05). It is
  accepted because discovery is at the cwd only (§2, MEASURED 2026-08-18)
  and no CLI reads `~/.agents` as a global root at all — each of the three
  has its own and it is a different path: `~/.codex/skills`,
  `~/.grok/skills`, `~/.bob/skills` under bob's state dir (ADR 0060)
  — so the tree at the home is read by the cwd-at-home session that asked
  for it, under exactly the §4 contract above: additive, owned by the dir,
  outliving the seat. The contrast is ADR 0062's persona-mode file, which
  shares the `dir` default and *refuses* this input, because its relative
  path lies inside a glob bob reads globally. Both halves are pinned in
  `internal/posse/skillshomedir_qa_test.go`, and the fact that would flip
  this one — a CLI declaring `.agents` as its own config root — reds an arm
  there rather than drifting. A refusal of the dir itself, if it is ever
  wanted, belongs where the dir is resolved (ranger-base-er6mt): four writes
  land inside the session dir — the pre-push and commit-guard hooks, this
  tree, the mode file — and a guard in one of them stops one.
- **(§6f)** `runtime.go`: `SkillsDirTrust` on the Runtime, declared for grok.
  `skillsdirtrust.go` (new): the declaration and reading types, grok's probe
  over `grok inspect`, `applySkillsDirTrust` at the dir-aware parity layer, and
  `skillsGateKey` — one spelling of the skills row's key, because two layers
  write it now and two spellings would make the removal a silent no-op.
  `parity.go`: one call in `checkParityIn`. `runtimecheck.go`: the declaration
  in the skills row. Pins in `skillsdirtrust_qa_test.go` (five mutations
  checked) and a live agreement arm in `skillsrules_parity_qa_test.go`, which
  hands posse's parse the bytes grok actually printed under both fixture homes
  rather than a string the test wrote.
- `posse skills list` (names, and which PIDs bind each) — read-only.
- Instance: the operator populates the instance repo's `RHQ_HOME/skills/`
  (symlinks to what already exists in `~/.claude/skills`, `~/.codex/skills`)
  and adds `skills:` to PIDs that need them; nothing changes for PIDs
  that don't.

## Alternatives rejected

- **Let each runtime's global config handle it.** That is today: skills
  are per-user-per-runtime, invisible in the PID, and differ between two
  machines running the same crew; a persona is a file, its skills should
  be in it.
- **Copy skills into the repo cwd (`.claude/skills`, `.agents/skills`).**
  Pollutes every project a persona touches, leaks between personas sharing
  a repo, and shows in `git status`. Only accepted as codex's *last*
  resort, symlinked and excluded via `.git/info/exclude`.
- **`--add-dir` for claude.** Verified not to load skills (CLAUDE.md dirs).
- **Inline the skills into the PID body.** Loses progressive loading —
  the whole point of the skill mechanism — and bloats every prompt.
- **A per-persona `CODEX_HOME`/`GROK` home overlay.** Would carry auth
  and session state per persona; fragile and a security question in its
  own right. Verify the flag surfaces first; overlays only if nothing
  else exists, and then as a bead of their own.
- **A posse skills registry, index, or marketplace.** Thin harness; the
  directory is the registry.
- **(§6) A session-only tree for every runtime.** Issue 12's first ask, and
  the right shape wherever it exists — it is claude's. codex and grok were
  measured to have no flag, config overlay or agent-file field that adds a
  root (§2); a home overlay is rejected above. Where a runtime reads only
  from the cwd, declared ownership is the remaining honest answer.
- **(§6) bob on `.bob/skills/`.** The one root both boxes agree bob reads,
  and the cheapest flip. Rejected because it is the team's own bob skills
  dir: posse's links would sit among them with provenance only in a link
  target, the exact asymmetry the issue names. It was the fallback if the
  live probe found `.bob/plugins/posse/skills/` unread — under 6d's
  *shared* rules, declared as such. The probe read it on 2026-10-31
  (Evidence, ranger-base-6r1rf), so the fallback is unused; it is the
  answer if a later bob stops listing the posse namespace.
- **(§6) Refuse a launch on version drift.** The clever wall: every
  cwd-root launch proven against today's CLI. Priced at a refusal per
  upgrade per runtime with a remedy (a live probe) most seats cannot run
  themselves; the drift line carries the same information without
  deciding who types the override (6c).
- **(§6f) SEED grok's folder trust at launch**, the way posse seeds claude's
  dialog key and types codex's `trust_level`. The obvious fix, and the one the
  finding asked for first. Rejected for three reasons, in ascending order of
  weight: the channel is a PERSISTENT, box-wide entry in the operator's own
  `~/.grok/trusted_folders.toml` rather than a session-scoped flag, so it
  outlives every session that wanted it; the launch it buys is not a dead one,
  only a skill-less one, where claude's seed is the difference between a
  session and a modal nobody answers; and the grant is **not scoped to
  skills** — the same measurement found it also opening a repo→box permissions
  channel posse declares nothing about (§6f, ranger-base-1g8gf). The first two
  are arguments about shape; the third is the one that decides it, and it
  reopens the day somebody measures what grok's trust buys in full.
- **(§6f) Read `~/.grok/trusted_folders.toml` and decide in posse.** Cheaper
  than a subprocess and the shape the two interstitial probes already use for
  that same file's neighbour. Rejected on a measurement: trust resolves
  through the git root to a linked worktree's MAIN checkout, and the key must
  be the path grok RESOLVES — so the rule is a multi-step resolution, and a
  second implementation of it would disagree with grok silently and in the
  direction that matters (reading "trusted" over a folder grok will not read).
  `grok inspect` is grok's own answer at 0.061s. The two interstitial probes
  stay a file read because each is one unique key with no resolution rule
  (`tomlFlag`, interstitial.go).
- **(§6f) Carry the precondition on `posse runtime drift`'s exit code.** It is
  the surface with a clock and a roster row, which is why the finding named it.
  Rejected because that walk's subject is the CLI VERSION and its exit codes
  are a per-box check (ranger-base-f5jbs): a directory is not a property of the
  box, the walk takes no dir, and giving it one would make `0 of N measured`
  mean two different things. The launch and `posse gates` have the dir.
- **(§6) Detect discovery after launch** (ask the CLI what it loaded).
  bob has no non-interactive listing (`bob --help`: chat, run, mcp, acp)
  and codex none either; it would be a per-runtime pane scrape on every
  launch for a fact that changes only at an upgrade.
