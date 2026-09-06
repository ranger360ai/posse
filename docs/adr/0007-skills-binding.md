# ADR 0007 — Skills binding: the PID declares, launch materializes per runtime

*Status: accepted 2026-08-18 · owner: architect · verified 2026-08-18
(rangerhq-1qd, codex-cli 0.147.0 and grok 1.0.5: the `-c` and `--agent`
candidates are dead ends; `<cwd>/.agents/skills/` is the only surface
either CLI has, and it is the same one — §2) · amended 2026-08-28
(rangerhq-3zr: a bound `SKILL.md` with no `description:` binds to nothing
on codex and is a finding — §2) · amended 2026-09-05 (ranger-base-65rc:
the claude plugin tree COPIES each skill in rather than symlinking to it
— §2) · amended 2026-09-06 (Context: the two superseded runtime rows
collapse into their verified answers — ranger-base-mppjc)*

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
| `runtimes/*.yaml` | optional `skills_flag:` (`--foo %s` given the rendered dir); absent → unrealizable | |

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
