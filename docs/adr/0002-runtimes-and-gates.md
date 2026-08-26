# ADR 0002 — Runtimes and gates: a persona launches on any runtime, the cage is not the runtime's

*Status: accepted 2026-08-17 · owner: architect · amended 2026-08-18
(L4 folded from the container-tier spike — §3 L4, §4, §5, Consequences,
Alternatives, Verification 8–11) · amended 2026-08-25 (ADR 0014:
path-scoped Edit/Write, `writable:` at L2 and L4, L4 `:ro` carve-outs) ·
amended 2026-08-26 (Claude directory trust and executable project config)*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> Persona names are restated as roles; command examples use role-named
> example personas.

## Context

ADR 0001 retired `codex.md`/`grok.md` as personas: a persona is the PID
(mindset); the CLI it runs on is labor. It left runtime choice to editing
`command:` — which means every PID's `command:` is claude-shaped, and
`{allow}`/`{deny}` render to `--allowedTools`/`--disallowedTools`, flags
only claude has. Meanwhile the fleet security posture (NOTES.md)
admits what those flags are: friction control, not a boundary.

Operator directive: permission gates must be **model-agnostic** — the
core of this design, not a constraint on it — and a PID whose gates
can't be realized on a runtime must refuse to launch there without
explicit consent. Containers are to be evaluated as an isolation *tier*,
not the fleet path.

Facts checked on the development machine (2026-08-17):

| runtime | prompt injection | native permission surface | herdr detects |
|---|---|---|---|
| `claude` | `--append-system-prompt`, `--add-dir` | `--allowedTools`/`--disallowedTools` (rule syntax, deny wins), settings `sandbox` | yes |
| `codex` | prompt arg (`codex "$(cat file)"`), `--add-dir` (writable dirs) | `-s read-only\|workspace-write\|danger-full-access`, `-a untrusted\|on-request\|never`, own seatbelt for child commands; **no per-verb rules** | yes |
| `grok` 1.0 | `--rules` (appends to system prompt) | `--allow`/`--deny` (compat aliases of claude's rule flags — semantics unverified), `--permission-mode`, `--sandbox <profile>`, `--disable-web-search` | yes |

`macOS 26.4`: `sandbox-exec` present and working (codex uses it under the
hood); Docker installed; Apple `container` not installed.

Measured 2026-08-18 for L4 (the container-tier spike; NOTES.md *Container
tier (L4)*, `docs/adr/0002-container-tier.probe.sh` re-runs it): repo +
`{memory}` bind-mount cleanly (VirtioFS maps root-inside to
operator-outside); cold `go build ./...` 4.84s bind-mounted / 4.49s
container fs / 5.33s host, warm start ~0.2s; the herdr socket round-trips
through a bind mount; herdr identifies a pane by foreground `argv0`, so
bare `docker run` is `agent_not_found`; a `--internal` network + CONNECT
proxy is a real egress boundary and all three CLIs honour `HTTPS_PROXY`;
a caged claude cannot ride the host login's credential file (stale
without the keychain that minted it, and a read-only mount kills the
refresh), so it needs an operator-minted env credential.

Constraints kept from ADR 0001: plain PID files, flat-YAML subset,
`{file}`/`{memory}` semantics, thin harness, backward compatible on the
current binary (mind the unrendered-placeholder trap).

## Decision

**1. Runtime is a named launch profile, not a command string.**
`internal/rhq/runtime.go` holds a table of built-ins — `claude`, `codex`,
`grok` — each with a command template and a *native realizer* (below).
A PID picks one with a new frontmatter key; the launch site can override:

| where | key/flag | precedence |
|---|---|---|
| PID frontmatter | `runtime: codex` (default `claude`) | persona default |
| recipe | `runtime:` | over PID |
| CLI | `posse new --runtime`, `posse dispatch --runtime` | over both |
| config.yaml | `default_runtime:` | fleet default when PID says nothing |

`command:` in a PID stays as the escape hatch, but is now *the template
for that PID's own runtime*: when a launch overrides to a different
runtime, the runtime's built-in template is used and the PID's `command:`
is ignored (it's claude-shaped; that's the whole point). PIDs should drop
`command:` and say `runtime:`; the scaffold does. Exit hatch for a runtime
we don't know: `RHQ_HOME/runtimes/<name>.yaml` (flat-YAML: `command:` only)
defines a template-only runtime with no native realizer — every gate goes
to the wall (§3), which is safe by construction.

Templates (initial; the developer verifies unattended-mode flags per
runtime):

```
claude: claude --append-system-prompt "$(cat {file})" --add-dir {memory} --settings '<fleet>' {allow} {deny}
codex:  codex {deny} -a never --add-dir {memory} "$(cat {file})"
grok:   grok --rules "$(cat {file})" {allow} {deny}
```

`{file}` `{memory}` `{allow}` `{deny}` are the only placeholders. `{allow}`
and `{deny}` are rendered **by the runtime's native realizer**, not by a
fixed flag: claude → `--allowedTools r…`/`--disallowedTools r…`; grok →
`--allow r`/`--deny r` per rule; codex → `-s read-only` when the deny
list covers `Edit`+`Write`, else `-s workspace-write` (codex has no
verb rules; its sandbox modes are what it *can* express). The realizer
returns which rules it realized natively; the rest go to the wall.

**2. Env is runtime-agnostic and unchanged, plus three vars.** `BD_ACTOR`,
`RHQ_PERSONA`, `RHQ_PERSONA_DIR`, `RHQ_TOOLS_ALLOW/DENY` ride the herdr
workspace env exactly as today for every runtime. New: `RHQ_RUNTIME`
(name), `RHQ_GATES_DIR` (§3), `RHQ_CAGE` (tier). Session meta records
`Runtime`, `Cage`, and `Degraded` (§4); the cockpit shows the runtime's
emoji (`config.yaml` `emoji:` already has codex/grok entries) and marks
degraded sessions.

**3. The wall: gates rendered from the PID, outside any runtime.**
`allow:` is friction (what runs unprompted) and stays runtime-native
only. `deny:` is the cage and is realized in layers, cheapest first, all
rendered fresh at every launch into `RHQ_HOME/state/gates/<persona>/`
from the PID (source of truth; nothing hand-edited there survives):

| layer | tier | mechanism | realizes |
|---|---|---|---|
| L0 politeness | any | runtime-native flags (§1) | whatever the runtime says it does — never counted as the wall |
| L1 shims | `shims` (always on) | `gates/<p>/bin/<cmd>` for every `Bash(<cmd> <prefix>:*)`/`Bash(<cmd>)` deny; refuses when argv matches, else `exec`s the real binary resolved at render time; logs refusals to `gates/<p>/refusals.log`; PATH is prepended **on the typed command line** (`PATH=<bin>:$PATH <cmd>`), not in the workspace env, because macOS `path_helper` reorders PATH when the pane shell starts | any deny that is a shell verb, on any runtime |
| L3 hooks | `shims` (always on) | `.git/hooks/pre-push` refusing when `RHQ_TOOLS_DENY` matches `git push`, and `.git/hooks/prepare-commit-msg` refusing a commit that does not name a pathspec when `RHQ_PERSONA` is set — the working tree and its index are shared by every persona, so an unqualified commit takes whatever anyone else has staged (both installed by `posse gates install-hooks` and at session create, marker-commented, never overwrite a foreign hook; after reconciliation, session create executes both slots and requires exit 1 on the gated probes — behavior, not the ownership marker, is what parity counts; the commit guard takes `prepare-commit-msg` because `pre-commit` is bd's and because `--no-verify` skips `pre-commit` but not this slot) | `git push`, and an unqualified `git commit`, even via absolute path / `env -i` / a subprocess |
| L2 seatbelt | `seatbelt` | `sandbox-exec -f gates/<p>/seatbelt.sb` wrapping the rendered command: deny `file-write*` except cwd, `{memory}`, the runtime's state dir (`~/.claude`,`~/.codex`,`~/.grok`), `$TMPDIR`; PID may add `writable:` paths; parametrized `Edit(<glob>)` is a trailing `subpath` deny after that allow (ADR 0014; last match wins) | `Edit`/`Write`-class denies on any runtime — bare and subtree-glob; the only runtime-proof file gate |
| L4 container | `container` | runtime runs inside a container (engine = a command template, `RHQ_HOME/cages/<engine>.yaml`, Docker default) with the repo and `{memory}` mounted and **nothing else of the host**; typed in the pane through an argv0 launcher `state/cages/<p>/bin/<runtime>` so herdr still sees `claude`/`codex`/`grok`; joined to a `--internal` network whose only other member is a CONNECT proxy holding `egress:`; L1 and L3 **re-rendered inside** (below); herdr socket **not mounted unless the PID says `sockets: [herdr]`**; path-scoped writes are `:ro` overlays, and a `:ro` repo always gets `.beads`/`.git` (and `writable:`) as read-write overlays (ADR 0014 §4) | `egress:` (hosts the persona may reach); the mount boundary (`Edit`/`Write` denies by mounting the repo `:ro`, scoped globs by overlay) — the successor of L2, which cannot wrap a container; hostile-input work, untrusted runtimes |

L1 is rendered locally; L3 adds one behavior-probe shell invocation per
persona launch into a repo (both slots in that invocation), on every runtime
— including claude, where `--disallowedTools` becomes the polite refusal in
front of the shim's hard one. A failed slot is a launch degradation, never a
silently discarded install error. **They do not follow a
process into a container by themselves**: a shim `exec`s the real binary
resolved *at render time on the host* (`/opt/homebrew/bin/git` does not
exist in Linux) and the gate shell (ADR 0009) points at the host's zsh.
So at `cage: container` the tiers stay cumulative in what they realize,
not in mechanism: the image carries the Linux `posse` and the
inner command is `posse gates wrap <persona> -- <runtime cmd>`, which
renders `gates/<p>/` *inside* the container — same renderer, same PID,
resolution against the image's PATH and shell — and types the same
`PATH=<bin>:$PATH SHELL=<gate shell>` prefix on the inner line. L3 rides
in on its own: `.git/hooks/pre-push` is on the repo mount, POSIX sh, and
reads only `RHQ_TOOLS_DENY`/`RHQ_GATES_DIR`/`RHQ_PERSONA` — the engine
template forwards every `RHQ_*`/`BD_ACTOR` var (`-e`) and the hook fires
unchanged. L2 does not stack: `sandbox-exec` around `docker run` cages
the docker *client*, not the container; the mount boundary is what
replaces it. The **herdr socket** is a fleet-wide capability — a persona
holding it can prompt or close every other pane — so the cage does not
mount it unless the PID declares `sockets: [herdr]`; dispatch *to* a caged
session never needs it (herdr prompts the pane from the host), only a
persona that itself dispatches does, and such a persona is not the one
you cage. When it is on, session meta records it and the cockpit marks
the cage (`container+herdr`): declared, so not refused — but the parity
claim it costs is stated where the operator can see it. Known holes of L1
(`/usr/bin/git`, `command -p`) are why L3 exists for the one verb that
is a hard risk line; the remaining holes are what the seatbelt/container
tiers are for. `sandbox-exec` is deprecated by Apple but is what codex
itself ships on today; its successor is the container tier.

**4. Enforcement parity: refuse, or degrade out loud.** At launch the
launcher computes, for the chosen (runtime × cage), which PID gates are
realized by *at least one wall layer* (L0 doesn't count). Realization:

| gate | realized by |
|---|---|
| `Bash(cmd …:*)` deny | L1 (any runtime) — plus L3 for `git push` |
| `Edit`/`Write`/`NotebookEdit` deny (bare) | L2 seatbelt; or codex `-s read-only` (native, but OS-enforced, so it counts); at `container`, the repo mounted `:ro` (mount boundary, replaces L2), with `.beads`/`.git` read-write overlays so bd still works (ADR 0014 §4) |
| `Edit(<glob>)` / `Write(<glob>)` / `NotebookEdit(<glob>)` | **subtree file-write deny** (ADR 0014): L2 trailing `subpath` deny; L4 `:ro` overlay of that directory. Unrealized at `shims`. Codex `-s read-only` over-enforces and does **not** count. A hook is never this row. A glob that is not a directory prefix (`**/*.md`) is unrealized by construction |
| `WebFetch`/`WebSearch` deny | claude/grok native flags only → **unrealized** below `container` tier; at `container`, realized only as far as `egress:` goes (the proxy stops unknown hosts, not fetches through an allowed one — say so in the degraded list when both are set) |
| `mcp__*` / other tool-name denies | claude/grok native only → **unrealized** on other runtimes |
| `egress:` list | L4 only — the route, not the env var: `--internal` network + CONNECT proxy; the launcher always adds the runtime's own hosts (claude `api.anthropic.com` + `platform.claude.com`; codex `chatgpt.com` + `ab.chatgpt.com`; grok `cli-chat-proxy.grok.com` + `grok.com`) and the proxy's denials land in `gates/<p>/refusals.log` like L1's |
| `cage:` in PID | minimum tier the PID demands; launching below it is degrading |
| the session *directory*'s own runtime config | not a gate — a hazard the launch names: see the amendment below |

The tiers are cumulative in *gates realized*: `container` realizes
everything `shims` does (L1/L3 rendered inside, §3), plus the mount
boundary, plus `egress:`. If the inner render cannot happen (no Linux
`posse` in the image, `.git/hooks` not on the mount) the shell-verb denies
are **unrealized at that tier** and the launch refuses like any other —
the strongest cage is never allowed to be the one that silently loses
`git push`. Auth is not a gate but it is a precondition: a caged claude
launches only with an operator-minted credential in the env set; until
then `cage: container` refuses with that reason.

If any gate is unrealized, `posse new`/`posse dispatch` **refuse** with the
list, unless `--allow-degraded` is passed; then they launch, print the
list, and record it in session meta so the cockpit shows a degraded cage.
Dispatch never passes `--allow-degraded` on its own. Concretely: the
security persona (`deny: Edit, Write, git push`) on codex at tier `shims`
launches — codex `-s read-only` + shims + hook realize all three; the
same persona on grok at tier `shims` refuses (grok's `--deny` is L0)
unless `cage: seatbelt` or `--allow-degraded`.

**5. PID additions** (all optional; inert on the current binary except
placeholders, of which there are none new):

```yaml
runtime: claude          # claude | codex | grok | <runtimes/*.yaml>
cage: shims              # shims | seatbelt | container — minimum tier
writable:                # L2/L4 allow-list extras (ADR 0014): paths that stay writable when Edit/Write are denied
egress:                  # container: hosts allowed out (implies cage: container)
sockets:                 # container: host sockets passed in; only `herdr` is known — off by default
trust_project_config: true  # allow the runtime to read the session dir's own config (amendment below)
```

## Consequences

- **Launcher** gains `runtime.go` (table + realizers, ~120 lines) and
  `gates.go` (shim/hook/seatbelt renderers + parity check, ~250 lines);
  `RenderCommand` takes a runtime; `CreateSession` writes gates and the
  degraded list. `RelaunchAgent` re-types the same wrapped command, so
  the wall survives a crash restart for free.
- **Dispatch** relies on herdr's kind detection, which already covers
  codex/grok. Answered by the spike: herdr classifies by the pane's
  foreground `argv0`, then screen-scrapes (OSC titles cross the boundary
  verbatim). Bare `docker run` is `agent_not_found`; a launcher
  `state/cages/<p>/bin/<runtime>` — a binary (a `#!/bin/sh` wrapper hands
  herdr `sh`) in its own dir (not `gates/<p>/bin`, whose entries refuse)
  that `exec`s the engine with `argv[0]` reset — restores `claude`/`idle`
  and dispatch.
- **Container tier follow-ons** (each a bead): engine template + image
  with Linux `bd`/`posse` (blocked on the credential question); argv0
  launcher; egress network + proxy; L1/L3 inside + `:ro` mount +
  `sockets:`; engine re-evaluation (ops). Two things the developer must
  carry from the probe: codex on a denied host **retries ~70 times in
  35s then errors** — an `egress:` typo is a retry storm, surface the
  proxy's 403s; and a fresh container has no `~/.claude.json`, so the
  launch seeds trust the way it seeds codex's `trust_level`.
- **Codex specifics** the developer must verify: `shell_environment_policy`
  strips env names matching `*KEY*`/`*TOKEN*`/`*SECRET*` by default —
  env sets with such names need `-c shell_environment_policy.ignore_default_excludes=true`;
  the PID rides as the first user turn (no system-prompt flag verified);
  `-a never` is what unattended needs; nested seatbelt (ours outside,
  codex's inside) intersects fine in principle — test it.
- **Grok specifics**: `--allow/--deny` are compat aliases; whether deny
  wins and `Bash(git push:*)` matches is unverified — irrelevant to
  safety (L0), relevant to politeness. Unattended mode flag TBD.
- **Every claude session in the fleet gets shims + hook.** A persona that
  hits a shim sees `refused by posse gate: git push (deny: Bash(git push:*))`
  — clearer than a permission dialog nobody is watching.
- **`gates/` is state**, under `RHQ_HOME/state/`, not memory: personas
  don't read it, and `refusals.log` is an audit trail the scorecard may
  later count.
- **NOTES.md fleet security posture** gets rewritten: the allowlist is
  friction; the wall is L1–L3; the boundary is L2/L4 when declared.
- **Instance PIDs** migrate `command:` → `runtime: claude` once bead 1
  ships; until then they keep working unchanged.

## Alternatives rejected

- **Translate every deny rule into each runtime's native surface** (the
  clever one). A rule→flag compiler per runtime is a policy engine that
  ages with three CLIs' flag sets and still can't express `git push` on
  codex. Native flags are L0; the wall is ours.
- **Runtime-specific personas / one PID per runtime** — re-litigated and
  re-rejected (ADR 0001): mindset ≠ labor.
- **Wrapper script per runtime reading `RHQ_TOOLS_DENY`** (ADR 0001's
  hatch). Kept as the env export, rejected as the design: a wrapper is a
  fourth CLI-shaped thing to maintain and still shells out to the same
  `git`; the shim is that wrapper, one level lower, runtime-agnostic.
- **PATH prepend in the workspace env.** `path_helper` reorders it;
  discovered while designing, hence the typed-line prefix.
- **Containers as the fleet default.** Rejected, and still rejected —
  but not for the reason first written. The VirtioFS tax was assumed;
  measured, it is ~8% on a cold build of this repo and a warm start is
  0.2s — not an argument. What holds: every session would need a
  credential the operator mints outside the keychain, a
  Linux build of every tool a persona touches, gates rendered twice, and
  either no herdr socket (so crew personas that dispatch cannot run
  there) or a socket that leaks the whole fleet into the cage. Against a
  threat model of a single operator running operator-authored beads,
  L1–L3 already hold the lines that matter. A tier a PID opts into.
- **Mount the host's `gates/<p>/` into the container as-is** (the
  cheap one). The shims `exec` host paths, the gate shell `exec`s the
  host's zsh; both die inside Linux, and a shim that dies is a shell verb
  that is *not refused*. Render where the binaries are.
- **Say `cage: container` realizes `egress:` and the mount and nothing
  else** (the honest-sounding one). It would make the strongest tier the
  only one that loses `git push`, and every caged PID with a shell-verb
  deny would launch degraded or not at all. Cumulative-in-realization is
  the property the parity matrix already promises; keep it true.
- **Mount the herdr socket by default** so caged sessions look exactly
  like host ones. Dispatch to the pane never needed it; only a persona
  that dispatches does, and that persona is the one you do not cage.
- **Claude's own `sandbox` setting as L2.** Runtime-specific by
  definition; grok's `--sandbox` likewise. Both are welcome *inside* the
  seatbelt as L0.
- **A `--runtime` that rewrites the PID's `command:` in place.** Mutating
  the persona file at launch makes the diff a reviewer reads untrue.

## Verification (QA's checklist)

1. `posse new --agent security --runtime codex` launches; inside, `git push`
   prints the gate refusal and appends to `refusals.log`; `touch x` fails.
2. `posse new --agent security --runtime grok` refuses with the unrealized
   list; `--allow-degraded` launches and the cockpit marks it.
3. On claude, `git push` in a developer session is refused by the shim
   before any dialog; `/usr/bin/git push` is refused by the pre-push hook.
4. `posse dispatch` of a `-l code` bead to a codex-runtime PID reaches the
   idle→prompted→done cycle exactly as claude does.

*Added by ADR 0009 (the gate shell), verified live:*

5. `posse new --agent developer --runtime grok`: inside, `command -v git` →
   `…/gates/developer/bin/git`; a `git push --dry-run` **reached through a
   Makefile recipe** prints the gate refusal and lands in `refusals.log`
   (grok's auto mode refuses to run an unknown local script, and its own
   deny rule catches `git push` in any command string it can see — a
   `make <target>` is the shape that reaches L1); `/usr/bin/git push` is
   still refused by the pre-push hook. `posse list` shows the PID on grok
   clean, not degraded, for a PID whose deny includes a non-`git push`
   shell verb — and that verb really refuses in the session, which is what
   makes the parity line honest.
6. Same `command -v git` on claude and codex sessions still answers the
   gates path (no regression); `gates/<persona>/shell/<basename>` exists
   and `gates/<persona>/shell.log` is absent after a normal session.
7. Repeat 5 after any grok/claude self-update — the shapes in ADR 0009's
   table (and NOTES.md's *gate shell* table) are what the wrapper
   recognises. Install an argv logger as `$SHELL` to see them: the wrapper
   is only as good as the argv it was written against.

*Added with the container-tier fold, to run once the container beads land:*

8. `posse new --agent security --cage container` (a PID with `deny: Edit,
   Write, git push` and an `egress:` list): `posse list`/`herdr agent list`
   show the session as `claude`/`idle` (not `agent_not_found`); inside,
   `command -v git` → `/…/gates/security/bin/git`, `git push` prints the
   gate refusal and lands in `refusals.log`, `/usr/bin/git push` is
   refused by the pre-push hook, `touch x` in the repo fails (`:ro`),
   `bd comments add` on the current bead succeeds through `.beads/bd.sock`.
9. Inside the same session: `curl https://example.com` fails (no route,
   no DNS); an allowlisted host answers through the proxy; a denied host
   yields the proxy's 403 *and* a line in `refusals.log`.
10. `ls ~/.config/herdr/herdr.sock` inside → absent; with `sockets:
    [herdr]` in the PID it is present, session meta says so, and the
    cockpit shows the cage marked.
11. A PID with `cage: container` and no credential in its env set
    refuses to launch with the credential-precondition reason; nothing
    is spent.

*Added by the shared-index commit guard:*

13. In any persona session in a repo with the hooks installed: `git add f
    && git commit -m x` and `git commit -am x` are both refused by the
    `prepare-commit-msg` hook, naming `git commit -F - -- <paths>`, and
    land in `refusals.log`; `git commit --no-verify -m x` is refused too;
    `git commit -m x -- f` commits, and another persona's staged entry is
    still in `git diff --cached` afterwards. The same `git commit -m x`
    from the operator's own shell in the same tree commits normally.
14. `git merge`, `git cherry-pick` and `git rebase --continue` in a persona
    session are **not** refused — git forbids a pathspec during those, so a
    refusal would have no safe form to point at.

## Amendment 2026-08-18 — the session directory is an input to the launch

§4's matrix is a statement about a PID on a (runtime × cage × tier). One
thing the launcher must decide is not in that product: **what the runtime
reads out of the directory the session starts in.**

The concrete case is codex. `posse` types
`-c "projects={\"$PWD\"={trust_level=\"trusted\"}}"` because codex's
directory-trust dialog fires per exact path, `-a never` does not suppress
it, and an unattended session otherwise hangs on a dialog nobody is
watching. Verified on codex-cli 0.147.0 (reproduced with a scratch repo
and `codex mcp list`, no API turn), that trust also loads
`$PWD/.codex/config.toml`:

- Every key posse types on the line **beats** the project's value — `-s`,
  `-a`, `developer_instructions`. The sandbox mode and the PID are ours.
- Every key posse does **not** type is the repo's: `[mcp_servers.*]`,
  `notify`, `model_provider(s)` (`env_key` names any session env var as
  the bearer sent to `base_url`), `shell_environment_policy`. codex spawns
  `mcp_servers` and `notify` itself — outside its per-command sandbox,
  with the whole session env, **before any model turn**.

So the channel is a file in a repo getting exec on the box with no model
and no PID in front of it. The security persona's verdict is to **keep
trust as the fleet default** — opt-in would mean codex dispatch never
works, the grant is one exact path and is not persisted, the attacker
needs repo write already, and claude has the same class of channel in
`.claude/settings.json` project hooks, so trust is parity, not a new
floor.

Decision: **the launcher checks for the file and makes the operator
answer.**

1. A runtime may name `ProjectConfig` — a path, relative to the session
   dir, that it reads as configuration because posse typed the flag that
   lets it. `.codex/config.toml` for codex; empty everywhere else,
   including claude (posse types it no trust flag; wiring the same check to
   `.claude/settings.json` hooks is a follow-on, not this decision).
2. `ProjectConfigTrust(rt, ag, dir)` is a `Degraded` entry when that file
   exists — never an `Unrealized` one. Nothing here is a gate that went
   unenforced; like a `cage:` shortfall it says what this launch gives
   away. It therefore inherits §4's whole machine: refuse with the list,
   `--allow-degraded` to launch marked, `degraded:` in meta, `⚠️degraded`
   in `posse list` and the cockpit, and — per ADR 0003 §3 — no waiver at
   tier `fast`.
3. `trust_project_config: true` on the PID is the durable opt-in, for a
   persona whose work *is* the repo's own tooling.
4. `CheckParity` stays dir-independent (a PID × runtime × cage × tier
   statement, statting nothing); `CheckParityIn` is that plus this and the
   launch directory's behavior-probed L3 hook result.
   `CreateSession` uses it, so `RelaunchAgent` re-checks for free — a repo
   that grows a `.codex/` after a clean launch refuses on the next
   relaunch. `posse gates <persona>` computes for the cwd and prints the
   directory it assumed.

Verification (added to the checklist above):

12. In a scratch repo with a `.codex/config.toml` naming an
    `[mcp_servers.probe]`: `codex mcp list` shows `probe` only under the
    trust flag; `posse new --agent developer --runtime codex` in that repo
    refuses and names the file; `--allow-degraded` launches it with
    `degraded:` in meta; the same launch at tier `fast` refuses even with
    `--allow-degraded`; a PID with `trust_project_config: true` launches
    clean.

## Amendment 2026-08-25 — path-scoped writes (ADR 0014)

§4's bare `Edit`/`Write`/`NotebookEdit` row stays. A *parametrized* rule
`Edit(docs/adr/**)` is not a tool-name deny and must not fall through to
the `mcp__*` default. It is a subtree file-write deny, realized by L2
(trailing SBPL deny) and L4 (`:ro` overlay), never by a hook, never by
codex `-s read-only`. `writable:` is the allow-list dual, at both tiers.
L4's `:ro` repo carries L2's `.beads`/`.git` carve-outs as read-write
overlays — the NOTES question about SQLite on a read-only mount is
answered there, not here. Verification is ADR 0014's checklist.

## Amendment 2026-08-26 — Claude directory trust makes project config live

### Context

`SeedClaudeTrust` now writes
`projects[<session dir>].hasTrustDialogAccepted` before every Claude
persona launch that needs it. This is necessary: Claude 2.1.241 has no
trust flag or settings key, and an untrusted fresh directory stops at a
full-screen dialog before dispatch can deliver work. The grant is exact-
directory, merged into the operator's config, and already the supported
non-interactive answer Claude names.

The grant also makes the 2026-08-18 amendment's premise false. In an
untrusted directory Claude drops project `hooks` and `mcpServers` from
`<dir>/.claude/settings.json`; after the launch trusts the directory those
entries load. They are the same repo-to-box executable channel for which
the amendment makes Codex ask at launch. But file presence alone is not
the right Claude predicate: this repository's committed settings file has
only `permissions`, which Claude reads without this grant, and refusing it
would describe no newly unlocked capability.

### Decision

Keep launch-time Claude directory trust, and extend `ProjectConfig` with
an optional content trigger:

1. `Runtime.ProjectConfig` remains the relative path. An empty path means
   no project-config surface. `ProjectConfigKeys` is a list of top-level
   JSON keys; empty means the existing whole-file-presence predicate.
2. Codex stays `ProjectConfig: .codex/config.toml` with no keys. Its check
   is unchanged: any such file is a hit because untyped TOML settings are
   live under trust.
3. Claude becomes `ProjectConfig: .claude/settings.json` with
   `ProjectConfigKeys: [hooks, mcpServers]`. A present top-level key is a
   hit regardless of its value. Posse identifies the channel; it does not
   reimplement Claude's schema or decide whether today's value happens to
   execute.
4. For a keyed JSON check: missing file is clean; a readable top-level
   object with neither key is clean; a matching key is degraded. An
   existing file that is unreadable, invalid JSON, or not a top-level
   object is degraded because the launch cannot prove the executable
   channel absent. The message names the path and either the matched keys
   or the classification failure.
5. The result stays a `Degraded` entry in `CheckParityIn`, with all of §4's
   existing behavior: refusal by default; marked launch under
   `--allow-degraded`; no waiver at tier `fast`; re-check on relaunch; and
   `trust_project_config: true` as the PID's durable opt-in.

This amendment does not scan `.claude/settings.local.json`, which is the
operator's gitignored local scope, or `.mcp.json`, whose project-server
approval is a separate Claude surface. If measurement shows the directory
seed bypasses either boundary, that is a new security finding, not a reason
to turn this detector into a general Claude settings engine.

### Consequences

- This repository's permission-only `.claude/settings.json` does not
  refuse Claude launches. Adding `hooks` or `mcpServers` makes the refusal
  immediate and visible before the runtime line is typed.
- The implementation remains data plus the standard JSON decoder. It owns
  no state and holds none hostage: removing Claude's table entry restores
  the pre-amendment behavior; clearing its keys deliberately falls back to
  whole-file presence.
- `NOTES.md`, `Runtime.ProjectConfig` comments, and the refusal text must
  stop claiming Codex-only or `mcp_servers/notify`-only semantics.

Verification:

13. In scratch directories, a Claude runtime with no settings file and one
    with a permissions-only file are clean; files with top-level `hooks`
    and `mcpServers` each degrade, refuse creation by default, launch marked
    with `--allow-degraded`, cannot be waived at `fast`, and launch clean for
    a PID with `trust_project_config: true`. Existing but unreadable,
    malformed, and non-object JSON each fail closed. The existing Codex
    whole-file tests stay green.

### Alternatives rejected

- **Treat every `.claude/settings.json` as hazardous.** This is simple but
  knowingly false for permission-only files and would refuse this fleet's
  own checkout until every Claude PID opted in.
- **Leave Claude unwired.** The trust seed now enables the exact class of
  project-owned executable channel the Codex decision requires the launch
  to disclose; runtime parity is not a reason to omit the check.
- **Interpret hook/MCP values deeply.** Claude owns that evolving schema.
  Key presence is a stable boundary; a bespoke executable-value evaluator
  would age with the runtime and could fail open.
- **Disable all Claude project settings at launch.** Project permissions
  are an intentional fleet floor (ADR 0001), and removing the whole source
  is a wider policy change than containing the two channels trust unlocks.
