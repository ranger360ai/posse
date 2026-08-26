# INSTALL — standing up a new RangerHQ instance, cold

This is the runbook. Follow it top to bottom on a machine that has never
run posse. It assumes you have never seen any existing instance, and it
never asks you to copy a file out of one.

**Where you end up:** a working instance — its own crew, its own queue, its
own environment — with one real bead dispatched to a persona and closed by
it.

**The split you are installing on both sides of:**

| | what it is | where it lives |
|---|---|---|
| **the harness** | `posse` — the binary, the cockpit plugin, the dispatch loop. Carries no secrets, no crew, no engine choice. | this repo, built once per machine |
| **an instance** | an `RHQ_HOME` directory — config, crew (personas), env sets, launch profiles, skills, state — plus the bd repos it serves | a directory you create in step 4 |

One harness serves N instances on a machine. Everything you configure
below is instance data; **no step in this runbook edits posse source.**
If you find one that does, that is a bug in the harness, not a step —
file it (`bd create ... -l code`) and say so.

Conventions in this document:

- `$` lines are commands to run; the line under them is what to check.
- `<angle brackets>` are yours to fill in.
- Every step ends with a **Verify** you can run. If a Verify fails, do not
  continue — the next step will fail in a less obvious place.

---

## 1. Prerequisites

Install these first. Versions matter; two of them are pinned on purpose.

| tool | version | why pinned | get it |
|---|---|---|---|
| **Go** | ≥ 1.26 | builds `posse` | `brew install go` / your distro |
| **herdr** | ≥ 0.8 | the presentation layer; posse talks to its CLI/socket | typed below |
| **bd** (beads) | **0.49.1 exactly** | 1.2.x replaced the SQLite backend with embedded Dolt and does not read `.beads/beads.db` at all — a 1.2 binary silently forks your queue. See NOTES.md, *beads (bd) substrate*. | typed below |
| **git** | any current | bd stores the queue in a git repo | — |
| an **agent CLI** | at least one | the labor. `claude`, `codex`, or `grok`. | vendor |

If you install `grok`, pin it: the fleet runs **1.0.5** and grok
self-updates by default (`etc/grok/version-pin.toml`, `make
verify-grok-pin`). An upgrade is a security re-audit, not a version bump.

**herdr** installs with its own script:

```sh
$ curl -fsSL https://herdr.dev/install.sh | sh
```

**bd** is a release binary from the beads repo,
github.com/gastownhall/beads (formerly `steveyegge/beads`; the old URL
redirects there). Do **not** `brew install beads` — that is 1.2.x, the
wrong binary per the pin above. Take the pinned tarball, which carries
`bd` at its root (`checksums.txt` sits beside the assets on the release
page if you want to verify the download):

```sh
$ os=$(uname -s | tr '[:upper:]' '[:lower:]'); arch=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
$ mkdir -p ~/.local/bin
$ curl -fsSL "https://github.com/gastownhall/beads/releases/download/v0.49.1/beads_0.49.1_${os}_${arch}.tar.gz" | tar xzf - -C ~/.local/bin bd
```

Both land in `~/.local/bin`, which is on no default PATH — the herdr
installer warns about this and then the warning scrolls away. Put it on
PATH now, and in your shell profile (`~/.zshrc`), or the Verify below
dies `command not found` with both tools correctly installed:

```sh
$ export PATH="$HOME/.local/bin:$PATH"
```

```sh
$ go version && herdr --version && bd version && git --version
```
**Verify:** Go ≥ 1.26, herdr ≥ 0.8.0, `bd version 0.49.1`. If `bd version`
says 1.2.x, stop and install 0.49.1 before going further — the rest of this
runbook will appear to work and will not.

Start the herdr server if it is not running: `herdr` with no arguments
launches or attaches its persistent TUI session, and `herdr server` runs
the server headless — installing herdr does not start it, so on a fresh
machine this step is yours, not optional.

```sh
$ herdr status server
```
**Verify:** a running server. posse cannot create a session without one.

---

## 2. Get the harness and build it

There are two ways in. **Homebrew** if you only want to run posse; **a
checkout** if you want to change it — steps 3 and 9 below assume a checkout,
and so does every persona that lands work.

The Homebrew route is **three commands, not one** — the middle one is the
part every published version of this page got wrong until 2026-08-24:

```sh
$ brew tap ranger360ai/tap                       # clone the tap
$ brew trust --formula ranger360ai/tap/posse     # read the next paragraph before running this
$ brew install ranger360ai/tap/posse             # a release binary, no Go needed
```
**Verify:** `posse version` prints `0.3.0+<sha>`, where the sha is the
commit the release was cut from, and `which posse` answers
`/opt/homebrew/bin/posse` (`/home/linuxbrew/.linuxbrew/bin/posse` on Linux).
Check the second one: if you have ever run `make install` from a checkout on
this machine, `~/.local/bin/posse` is earlier on `$PATH` and will answer
`posse version` for you, which makes a broken brew install look fine —
that PATH ambiguity is what produced ranger-base-253. Then skip to step 4 —
there is nothing to promote, and `posse init` seeds an instance from the
examples embedded in the binary (ADR 0012 D5). The cockpit plugin still
wants the checkout, so come back to step 3 when you want it.

**Why the trust line exists, and exactly what it grants.** Homebrew 6.x will
not load formulae from a third-party tap until you say so; until then
`brew tap-info ranger360ai/tap` reads `Untrusted` and brew, in its own
words, "is currently ignoring formulae, casks and commands from these taps
because tap trust is required". `brew trust --formula
ranger360ai/tap/posse` grants **this one formula and nothing else**. The
grant is one string appended to `~/.homebrew/trust.json` (or
`$XDG_CONFIG_HOME/homebrew/trust.json` if that is set) — read the file
afterwards and you can see the whole of what you gave. It is
non-interactive: no prompt, no password, no network; it prints `Trusted
formula: ranger360ai/tap/posse` and exits 0, and prints `Already trusted
formula: …` if it was already there. `brew untrust --formula
ranger360ai/tap/posse` takes it back.

Do **not** reach for the whole-tap form `brew trust ranger360ai/tap` to make
an error go away. Whole-tap trust covers every current *and future* formula,
cask and command from that tap — a standing grant on a repository we can
change later, which is far more than installing one binary is worth. Brew
recommends the narrow form for exactly this reason: "Prefer trusting only
the specific formulae, casks or commands you need."

And do not take our word for it. A page that tells you to "trust" something
without saying what it grants is the shape of a supply-chain lure, and ours
should be checked the same way you'd check a stranger's: `brew trust --help`
and <https://docs.brew.sh/Tap-Trust> describe this command, and the trust
file is plain JSON you can read before and after.

If you skip the trust line, brew names its own fix and you can run it then
instead — this is a refusal to *load* our formula, not a broken machine:

```
Error: Refusing to load formula ranger360ai/tap/posse from untrusted tap ranger360ai/tap.
Run `brew trust --formula ranger360ai/tap/posse` or `brew trust ranger360ai/tap` to trust it.
```

Depending on your brew version you may not see that error at all: naming a
formula in full on the command line (`ranger360ai/tap/posse`, not `posse`)
can be taken as the grant itself, in which case `brew install` trusts that
one formula for you and the explicit line above prints `Already trusted`.
Measured both ways on 6.0.19 — the short name `brew install posse` after a
tap *is* refused. Run the trust line regardless: it is a no-op when it is
not needed, and when it is needed it is the difference between installing
and staring at an error.

If brew answers `Error: Failure while executing tap` or `Repository not
found`, **the tap is not published yet** — it exists only once a release has
been cut, and the error never says so. Nothing on your machine is broken and
there is nothing here to debug: take the checkout path below, which needs Go
and nothing else. (Maintainers: `docs/runbooks/release.md`.)

Homebrew installs the binary and **nothing else**: herdr and the pinned bd
in step 1 are still yours to install, and `brew install beads` is still the
wrong one. The formula says so in its caveats.

```sh
$ git clone <posse remote> ~/src/posse
$ cd ~/src/posse
$ make build
```

`make build` produces `bin/posse-go` — a dev build of the working tree. It
never touches the live binary.

```sh
$ ./bin/posse-go version
```
**Verify:** `0.3.0+<sha>` (a `-dirty` suffix just means the tree has
uncommitted edits; on a fresh clone it will not).

---

## 3. Promote the binary and register the cockpit

`make install` is the *promotion* step, and it is deliberately different
from `make build`: it checks HEAD out into a throwaway `git worktree`,
builds there, and installs that — so the binary on your PATH is always a
commit you can name, never somebody's in-flight edit.

```sh
$ make install                 # → ~/.local/bin/posse   (BINDIR=… to override)
$ export PATH="$HOME/.local/bin:$PATH"   # step 1 already did this; harmless twice
$ posse version
```
**Verify:** the target prints `installed: …/posse`, the promoted sha, and the
version, and `posse version` — the one PATH finds, not `./bin/posse-go` —
prints the sha it just promoted. Two warnings can appear on stderr instead,
and both are yours to fix before going on, because the plugin and every later
step run the binary PATH finds:

- `…/.local/bin is not in your PATH` — the export above is missing from this
  shell. Run it, and put it in `~/.zshrc` too. Skip it and `posse init` in
  step 4 is `command not found` with the install having exited 0
  (ranger-base-88m).
- `PATH resolves posse to …` — some *other* posse (a brew install, an older
  checkout) is earlier on `$PATH` and will answer for the binary you just
  promoted. Reorder `$PATH` or remove the stale one.

`install` also drops an `rhq` symlink beside the binary, and `link-plugin` a
second one in `plugin/bin/`. Both are **transition mechanics** for instances
that predate the rename (rangerhq-tyay): the command is `posse`, and `rhq` is
only there so standing orders, permission allowlists and recorded session
recipes written under the old name keep resolving on the day the binary
changes. Nothing new should be written against it, and it is scheduled for
removal — on a fresh install you can delete both links and lose nothing. One
consequence while it exists: a PID rule naming the harness is matched on the
typed word, so `Bash(posse …)` does not cover `rhq …`. Spell such a rule both
ways, or retire the links first.

From outside a checkout the same binary installs with:

```sh
$ go install github.com/ranger360ai/posse/cmd/posse@latest
```

That build carries the seed tree (`examples/`) embedded, so `posse init`
works with no repo beside it. `@latest` installs the newest release tag —
currently `v0.3.0`, which trails `main` — and until ranger-base-bzu lands its
`posse version` still reports `0.3.0+dev`; `go version -m $(command -v
posse)` shows the module version that actually installed. It is not the
promotion path a fleet should use: the tag lags, and the fleet needs a build
stamped with the exact commit. Prefer `make install`.

```sh
$ make link-plugin
$ herdr plugin list
```
**Verify:** `posse.cockpit (posse) enabled [local:<your checkout>/plugin]`.

`link-plugin` symlinks `plugin/bin/posse` → the *installed* binary, so the
cockpit popup can never run an unpromoted build.

Bind the cockpit to a key. Add to `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+g"          # herdr's prefix is ctrl+b by default → ctrl+b g
type = "popup"
command = "posse cockpit"
width = "75%"
height = "75%"
```

```sh
$ herdr server reload-config
```
**Verify:** `ctrl+b g` opens a pane titled 🤠 posse. (It will be empty
until you have sessions and beads — that is correct here.)

**Agent-detection overrides — do not skip this if you will run codex or
grok.** herdr's stock detection reports some screens that are *holding the
keyboard* as idle, so a dispatched prompt gets typed into a modal instead
of the composer, silently:

```sh
$ make install-detection
```
**Verify:** the target prints one `installed:` line per override and then
runs `verify-detection` without complaint.

Everything in this section is **machine-global** and shared by every
instance on the machine: one binary, one plugin registration, one detection
manifest set. That is by design (ADR 0015 D5) — version skew between two
instances on one machine is impossible by construction.

---

## 4. Create the instance

An instance is a directory. Pick where it lives and **export `RHQ_HOME`**;
everything `posse` does is relative to it. With `RHQ_HOME` unset, posse uses
`~/.config/posse` — fine for your only instance, wrong the moment there are
two.

Existing installs are left alone. If `~/.config/posse` does not exist but
`~/.config/rhq` does, posse keeps using `~/.config/rhq` and prints a notice;
it does not move, copy, or create anything there. That remains your home until
you move it yourself. If both directories exist, `~/.config/posse` wins.

A good home is a directory inside a *private* repo of your own, so your
config and crew are versioned:

```sh
$ mkdir -p ~/src/<your-instance-repo> && cd ~/src/<your-instance-repo> && git init
$ export RHQ_HOME=~/src/<your-instance-repo>/posse
```

Put that export in your shell profile now (`~/.zshrc`), or every new
terminal will address the wrong instance.

Seed it. Any posse binary can do this — the example instance is embedded in
the binary at build time (ADR 0012 D5), so a release binary on a laptop with
no checkout seeds an instance the same as a dev build:

```sh
$ posse init
```

**Verify:** `initialized <your RHQ_HOME> (seed: embedded)`. Run out of a
checkout — `./bin/posse-go init` — and the seed line names the on-disk
`examples/` instead: that directory wins when it is beside the binary, so
edits to the examples take effect without a rebuild.

What `init` created, and what it did not:

```
$RHQ_HOME/
  config.yaml     ← from examples/config.yaml, fully commented
  agents/         ← nine generic personas from examples/agents/
  recipes/        ← three example recipes
  envs/           ← two example env sets   (dir 0700, files 0600)
  skills/         ← empty; the skills registry
  state/          ← empty; machine-local, never commit it
  promoted.json   ← the manifest: sha256 per promoted file, marked `seeded`
```

Not created, and you make them yourself when you need them:

- `runtimes/` — instance launch profiles (step 8)
- `personas/` — per-persona private memory; materialized at first launch,
  each seeded with an `ORDERS.md`

`init` **never overwrites**. Re-running it is safe and fills in anything
missing; it will not undo your edits.

If your instance home is inside a git repo, commit everything except
`state/`:

```sh
$ cd ~/src/<your-instance-repo>
$ echo 'rhq/state/' >> .gitignore
$ git add -A && git commit -m 'posse: seed instance from posse examples'
```

**Or keep the two apart, which is what ADR 0015 recommends once the
instance has personas editing their own prose.** Draft the constitution —
`agents/`, `config.yaml`, `recipes/`, `skills/` — in a repo, and put it in
force with `posse promote <that dir>`, which copies it into `$RHQ_HOME` from
a **commit** and records `{source, sha, sha256 per file}` in
`promoted.json` beside it. Every launch re-hashes the promoted set against
that manifest: a dispatched session refuses on a mismatch, an interactive
one warns DEGRADED, and `posse promote` clears it. Uncommitted prose is then
never in force, and what you ratify is a diff — promote prints
`git diff <last promoted>..HEAD` over those four paths before it writes
anything (`--dry-run` prints it and stops).

Promote never creates, copies or touches `envs/` (gitignored secret values
— there is no commit to promote them from, and a copy that widens 0600
publishes tokens), `state/` (machine-local) or `personas/` (persona memory,
which is scoped, versioned, and deliberately not ratified). `posse init`
already seeds `envs/` with its modes, and every env read re-asserts them.

`posse init` writes a manifest too, marked `seeded`: a fresh install
verifies clean without ever having promoted anything, and a home that never
had a manifest is simply not checked.

---

## 5. config.yaml — the minimum that matters

`$RHQ_HOME/config.yaml` arrives fully commented; read it once. It is a
**flat YAML subset** — scalars, one-level maps, lists. No nesting, no
anchors, no multi-line strings. Unknown keys are ignored.

For a working instance you need these. Everything else has a defensible
default:

```yaml
# Where the TUI's directory picker looks (roots + their immediate subdirs).
dirs:
  - ~/src

# The queue. `posse ready`, the cockpit and dispatch aggregate these repos'
# bd databases. A missing or unreadable repo is named as a failed scan.
beads:
  - ~/src/<your-work-repo>

# Who gets the ASK question beads a persona files when it needs a human.
# Unset means those beads land unassigned, where nobody sees them.
operator: <your-bd-actor-name>

# Launch defaults for personas that name none.
default_runtime: claude          # claude | codex | grok | runtimes/<name>.yaml
default_tier: standard           # strong | standard | fast  (ADR 0003)
```

Two more you will want before you arm anything unattended — set them once
you are past your first dispatch:

```yaml
# Skip a dispatch pass above these percentages of your Claude plan's
# rolling windows, so the fleet leaves headroom for you.
plan_guard_5h: 70
plan_guard_7d: 85
```

Leave `autostart_interval:` **commented out** for now. Its presence is the
arm switch for the unattended dispatch loop, and you are not ready to arm
one (step 12).

```sh
$ posse envs && posse agents
```
**Verify:** both list something and neither errors. If `posse` reports the
wrong paths, `RHQ_HOME` is not exported in this shell.

---

## 6. Env sets

An **env set** is a named `KEY=VALUE` file under `$RHQ_HOME/envs/`,
injected per-session. `init` seeded two examples; they are examples, not
config.

```sh
$ posse env edit default        # opens $EDITOR; created if missing
$ posse envs
```
**Verify:** `posse envs` prints set names and **key names only** — never
values. That masking is the point; check it holds for anything you add.

Rules:

- `envs/` is `0700` and its files `0600`. Keep it that way.
- Plain `KEY=VALUE` lines, passed verbatim. No shell expansion, no quotes
  stripped.
- **Secrets live here or in a secret manager — never in `config.yaml`,
  never in a PID, never in a bead comment.** Refer to a credential by name
  and location; that is what this file is for.
- `default_env:` in config applies to plain sessions with no `--env-file`.
  **Persona sessions get only the sets their PID names in `envs:`**, plus
  any explicit `--env-file`. A persona that needs an env set must say so.

Work-specific secrets and the keychain question are deliberately **out of
scope here** — that is M2, and it waits on its own ADR.

---

## 7. The crew

A **persona** is three things bound to one name:

1. a bd **assignee** (`BD_ACTOR`, injected at launch) — durable identity;
2. `$RHQ_HOME/agents/<name>.md`, its **Persona Intent Document** — flat-YAML
   frontmatter plus a markdown body that *is* the prompt (ADR 0001);
3. `$RHQ_HOME/personas/<name>/` — private memory, seeded with `ORDERS.md`
   at first launch.

`init` gave you nine generic personas (`architect`, `developer`, `devops`,
`product`, `qa`, `reviewer`, `security`, `business-manager`, `ranger`).
**They are a starting point to edit, not a crew.** Rename them, delete the
ones you do not want, write the bodies in your own words.

```sh
$ posse agent edit developer            # edit a seeded one
$ posse agent new <name>                # scaffold a new one (opens $EDITOR)
```

The frontmatter keys that do work:

| key | what it does |
|---|---|
| `name`, `description` | identity and the listing line |
| `labels:` | **dispatch routing** — bead labels this persona picks up |
| `runtime:` | which launch profile (step 8); default `claude` |
| `tier:` / `tier_floor:` | model tier, and the tier below which it refuses to run (ADR 0003) |
| `intents:` | intent slugs; the `## Intents` table's "done when" cell is what a reviewer checks a closed bead against |
| `allow:` / `deny:` | permission rules — `deny:` is enforced by the wall, not by politeness |
| `envs:` | env sets this persona's sessions receive |
| `skills:` | skills to bind (names under `$RHQ_HOME/skills/`) — **declared means required**: a runtime that cannot materialize them refuses the launch |
| `cage:`, `writable:`, `egress:` | minimum wall tier (ADR 0002 §5) |
| `command:` | escape hatch — a hand-written launch template. Prefer `runtime:`. |

**How a bead reaches a persona** (dispatch, in order):

1. the bead's **assignee**, if it names a persona;
2. the first persona whose `labels:` overlap the bead's labels;
3. config `default_persona:`, if you set one.

Unroutable beads are reported and skipped. If nothing picks up your work,
this list is why.

The one name none of the three ever reaches is config `coordinator:` — the
instance's exception handler (ADR 0018). Dispatch refuses to hire her by any
path: assigned to her is unroutable and says so, her PID is skipped in the
label loop, and naming her as `default_persona:` is reported as the config
error it is. No flag overrides it. Coordinator authority — session direction
and `git push` — exists only in a session you opened yourself; you reach her
through her crew session, which dispatch already leaves alone. Leave the key
unset and none of this applies.

Lint the crew:

```sh
$ posse agent check --all
```
**Verify:** `N persona(s) match the PID contract`, exit 0. It checks the
ADR 0001 section order, that `skills:` names resolve, and warns when a PID
has no `## Work prompt` section. Fix findings before dispatching — a PID
that fails the lint launches, it just does not do what you think it does.

Inspect what the wall will actually enforce for one of them:

```sh
$ posse gates <persona>
```
**Verify:** for each runtime × cage tier, either `all gates realized` or a
`DEGRADED` list. A launch refuses on `DEGRADED` unless you pass
`--allow-degraded`; dispatch never degrades on its own. Reading this
*before* your first dispatch saves you a confusing refusal later.

**Path-scoped writes** (ADR 0014). `deny: [Edit, Write]` is still the
whole-repo wall (the reviewer/security skeletons). A parametrized rule
`Edit(docs/adr/**)` / `Write(docs/adr/**)` is a **subtree file-write
deny** — that directory cannot be written, the rest of the repo can. The
wall that realizes it is `cage: seatbelt` (OS-enforced trailing deny) or
`cage: container` (a `:ro` overlay of the directory), not a Claude hook
and not `--disallowedTools`. `sed -i` walks past the hook; it does not
walk past the seatbelt. Globs that are not a directory prefix
(`**/*.md`) stay unrealized — say so with `posse agent check`. The
allow-list dual is the existing `writable:` key: `deny: [Edit, Write]`
plus `writable: [docs/adr]` means *only* that directory (and `.beads` /
`.git`, so bd still works). Codex `-s read-only` realizes the *bare*
deny and over-enforces a scoped one, so a path-scoped PID on codex needs
the container tier. Until the grammar lands in `posse gates`, a
parametrized rule still prints as `runtime-native only` and refuses the
launch — do not put one on a PID you need to dispatch this week.

---

## 8. A launch profile of your own

The three built-in runtimes (`claude`, `codex`, `grok`) each carry a
command template *and* a native realizer that turns a PID's `allow:`/`deny:`
into that CLI's own flags. **Your instance can define its own**, which is
how an engine that posse has never heard of gets used without touching
harness source.

`runtimes/` does not exist yet. Make it:

```sh
$ mkdir -p $RHQ_HOME/runtimes
$ $EDITOR $RHQ_HOME/runtimes/<profile>.yaml
```

A profile is flat YAML. Only `command:` is required:

```yaml
# The command template. Placeholders posse expands:
#   {file}    the PID file — always inside a flag's value, never a bare
#             positional (usually `--flag="$(cat {file})"`); see 1 below
#   {memory}  the persona's private memory dir
#   {model}   rendered from model_flag: + model_<tier>: (see caveat 4)
#   {skills}  rendered from skills_flag:
#   {allow} {deny}   ← always EMPTY here; template profiles have no realizer
command: <cli> --some-unattended-flag --rules="$(cat {file})"

# Optional:
# model_flag: --model          # the flag {model} renders with
# model_strong: <model-id>     # per-tier model ids (ADR 0003)
# model_standard: <model-id>
# model_fast: <model-id>
# skills_flag: --plugin-dir    # this CLI's skill-surface flag; absent means
#                              # it has none, and a PID with skills: cannot
#                              # launch on this profile
# cage_cred: <ENV_VAR>         # the credential a *containerised* session of
#                              # this runtime authenticates with; absent means
#                              # `cage: container` refuses here
# gate_shell: false            # only if this CLI chokes on a wrapper named as
#                              # its SHELL (ADR 0009 §2). Costly — read it first.
#
# --- the dispatch contract (ADR 0013). Every one of these is optional, and
#     leaving it out is a DECLARATION too: see `posse runtime check` below ---
# prompt: typed               # how dispatch delivers the work prompt.
#                              # `typed` (the default): create the session,
#                              # wait for a promptable screen, claim, then
#                              # type. `argv`: append the prompt file to the
#                              # launch line, so no screen is the delivery
#                              # channel. Only declare `argv` once you have
#                              # PROVED your CLI takes a positional prompt
#                              # into an *interactive* session.
# startup_wait: 45s           # how long a launch may take to reach a
#                              # promptable screen. MEASURE it; do not guess.
# record: untrusted           # the default. `trusted` says you have MEASURED
# record_why: <what you saw>  # a dispatched session of this CLI closing its
#                              # bead. Until then dispatch still launches, but
#                              # a session that settles with the bead still
#                              # open is never ✓ and gets re-prompted.
# native_rules: [AGENTS.md]   # rulebook files this CLI discovers and loads by
#                              # itself, ahead of anything posse types. Posse
#                              # rewrites none of them — declaring them is how
#                              # `runtime check` can name the other voice in
#                              # the session.
```

Four things about template profiles that will bite you if nobody says them:

1. **The PID rides in a flag's *value*, never as a positional — and glue
   it with `=` unless you have proved the separated form works.** Every
   PID opens with `---` (ADR 0001 frontmatter), and a clap-based parser
   reads *any* argument beginning with `-` as an option. So
   `<cli> --some-unattended-flag "$(cat {file})"` dies with
   `error: unexpected argument '---…'` before a session ever exists — and
   on grok so does the *separated* flag form, `--rules "$(cat {file})"`;
   only `--rules="$(cat {file})"` binds. The three built-ins are the three
   dialects that work: claude `--append-system-prompt "$(cat {file})"`,
   codex `-c developer_instructions="$(cat {file})"`, grok
   `--rules="$(cat {file})"`. **Probe your CLI before trusting it** — this
   costs no API turn, because the parser fails or the help prints first:

   ```sh
   $ printf -- '---\nname: x\n---\nhello\n' > /tmp/p.md
   $ <cli> <your flags> --your-flag="$(cat /tmp/p.md)" --help
   ```

   Help text means the parser bound the PID. `error: unexpected argument
   '---` means it ate it — and had you launched for real, the persona
   would have come up with no instructions at all.

2. **`{allow}` and `{deny}` render to nothing.** There is no realizer, so
   the CLI's own polite refusals do not exist and *every* gate goes to the
   wall (the L1 shims, and the L3 pre-push hook). This is safe by
   construction — it is strictly more enforcement, not less — but do not
   expect the CLI to refuse anything on its own.
3. **posse does not know this CLI's unattended flag, so it cannot add one.**
   The built-ins get theirs guaranteed at launch; a template profile does
   not. **Your `command:` must name it itself** (codex `-a never`, grok
   `--permission-mode auto`, claude `--permission-mode auto`). Omit it and
   your session sits forever on a dialog nobody is watching.
4. **`model_flag:` always renders with a space** before the id —
   `-c model= 'gpt-5-codex'`, which is not what a glued dialect parses.
   If your CLI wants `-c model=<id>` or `--model=<id>`, **omit
   `model_flag:`/`model_<tier>:` entirely and hardcode the model in
   `command:`**. You lose per-tier model mapping on that profile. Harness
   bug, filed: **rangerhq-5p0d**.

A worked example — the local `codex` CLI redeclared as instance data,
rather than used as the built-in. This is exactly the shape that proves
the config path for an engine posse does not ship a realizer for:

```yaml
# $RHQ_HOME/runtimes/codex-local.yaml
# Model is hardcoded rather than mapped, because codex's dialect is glued
# (-c model=<id>) and model_flag: cannot express that — see caveat 4.
# `-a never` is codex's unattended flag; posse will not add it here.
command: codex -c model=<model-id> -a never --disable hooks -c allow_login_shell=false -c developer_instructions="$(cat {file})"
```

A second in a different dialect — `grok`, which unlike codex maps tiers
fine (`--model` takes a separated value) but needs the glued `=` on
`--rules`, the trap in caveat 1:

```yaml
# $RHQ_HOME/runtimes/grok-local.yaml
# `--permission-mode auto` is grok's unattended flag; posse will not add it here.
# `--rules=` must be glued — the separated form reads the PID's leading
# `---` as a flag and refuses to start (caveat 1).
command: grok --permission-mode auto --rules="$(cat {file})"
model_flag: --model
model_strong: <model-id>
model_standard: <model-id>
model_fast: <model-id>
```

```sh
$ posse runtimes
```
**Verify:** your profile appears after the built-ins, marked
`template-only (gates go to the wall)`, with the template printed back.

Then read the contract grid — this is the onboarding surface, and it is
the one command that tells you what your new profile has *not* declared:

```sh
$ posse runtime check <profile>
```
**Verify:** six stages — `launch / promptable / work / record / settle /
account` — each with who declared it and what a missing one costs (ADR
0013 §1). A profile with nothing but `command:` is **dispatchable and
loud**: typed delivery on the default 45s wait, `record: untrusted`,
`UNCOUNTED`, `UNMAPPED` tiers, and every row naming the yaml key that would
change it. That is the intended reading, not a failure — unknown starts
noisy, never silent and never forbidden.

Two rows are worth acting on before you dispatch anything:

- **launch** says whether herdr recognizes your CLI's argv0. If it does
  not, herdr cannot see `working` or any settled state on it, so the wait
  ladder is guessing and dispatch is blind. Fix that first.
- **promptable** lists this runtime's **instance interstitials** — the
  first-run dialogs that make a fresh pane un-promptable, and the config
  key *you* set to silence each. See §10 below.

Point at least one persona at it — set `runtime: <profile>` in its PID:

```sh
$ posse agents
```
**Verify:** that persona's line reads `[<profile>/<tier>]`.

---

## 9. The work repo and its queue

Work lives in beads, in a git repo. Create the repo first, then the queue.

```sh
$ mkdir -p ~/src/<your-work-repo> && cd ~/src/<your-work-repo>
$ git init
$ bd init
```

`bd init` creates `.beads/` (a SQLite database plus `issues.jsonl`, its
git-tracked projection) and derives the issue prefix from the directory
name unless `.beads/config.yaml` sets `issue-prefix:`. Rename the directory
*before* `bd init` if you care what your bead ids look like — renaming the
prefix afterwards is a maintenance verb.

Then wire the three git-side pieces, and verify each rather than assuming
`bd init` did it:

```sh
$ bd hooks install
$ ls .git/hooks
```
**Verify:** `pre-commit` (flushes pending bd changes into `issues.jsonl`
before the commit) and `post-merge` (imports after a pull) are present.
Without these, the database and the git-tracked JSONL drift apart.

`bd hooks install` also plants shims in two slots the posse L3 gates want —
`pre-push` and `prepare-commit-msg` — and it plants them *silently*, over
whatever is already there. Run it **before** `posse gates install-hooks`,
never after; the block below resolves the collision.

```sh
$ grep beads .gitattributes ; git config --get merge.beads.driver
```
**Verify:** `.beads/issues.jsonl merge=beads` and a driver command
(`bd merge %A %O %A %B`). If the driver is missing, two clones' beads
conflict as raw text:

```sh
$ git config merge.beads.name 'bd JSONL merge driver'
$ git config merge.beads.driver 'bd merge %A %O %A %B'
```

Give the repo an `AGENTS.md` so a persona landing in it knows the queue
exists:

```sh
$ bd onboard >> AGENTS.md
```

**Then reconcile that file with the crew's guardrails, before any persona
reads it.** `bd init` writes an `AGENTS.md` carrying a "Landing the Plane"
section that orders the reader to push — "Work is NOT complete until `git
push` succeeds", "NEVER stop before pushing", "If push fails, resolve and
retry until it succeeds". Every seeded persona in `examples/agents/` denies
`Bash(git push:*)`, and the `pre-push` gate installed below enforces that
deny in the shell. So the file dispatch hands a persona as orientation
orders the one thing the wall refuses, and orders it retried. This is not
theoretical: on the first dispatch of a cold instance a persona spent a turn
pushing into the gate, logged at
`$RHQ_HOME/state/gates/<persona>/refusals.log` (rangerhq-cmfj).

Cut the section out and say who pushes instead:

```sh
$ grep -n "Landing the Plane" AGENTS.md
$ awk '/^## /{s = ($0 ~ /^## Landing the Plane/)}
       !s { if (NF==0) { b = b "\n"; next } printf "%s%s\n", b, $0; b = "" }' \
    AGENTS.md > AGENTS.md.new && mv AGENTS.md.new AGENTS.md
$ cat >> AGENTS.md <<'EOF'

## Landing the plane

- Close the bead, and commit **naming your own paths** (`git commit -F - --
  <paths>`) — every persona shares this checkout and its index, so an
  unqualified commit takes whatever another persona has staged.
- `bd sync`, so `.beads/issues.jsonl` matches the database.
- **Never push. The operator pushes.** Every persona's PID denies
  `Bash(git push:*)` and this repo's `pre-push` gate refuses it, so a push
  is a refused turn, not a landing. Work is complete when it is committed
  locally and the bead is closed.
EOF
```

The `awk` drops everything from `## Landing the Plane` to the next `##`
heading (or end of file), leaves the rest alone, and holds back blank lines
until a non-blank follows one — so the cut leaves no trailing blank for the
appended section to double up on. If `grep` found nothing, skip it and just
append the section.

**Verify:**

```sh
$ grep -n -i "push" AGENTS.md
```
Every surviving mention must say the operator pushes. A line that still
orders the *reader* to push means the orientation still contradicts the wall.

Two copies of that mandate exist and only one is a file. The other is `bd
prime` — the session-start context `bd hooks install` auto-injects — whose
own checklist ends `git push (push to remote)` / "**NEVER skip this.** Work
is not done until pushed." It comes out of the `bd` binary; nothing in the
repo can trim it. That is why the dispatch work prompt states the precedence
in its own voice ("Your PID's guardrails override any push/deploy
instruction in repo docs") and why the section above says it a second time
where the persona is looking. Re-check both after any `bd` upgrade or a
second `bd onboard` — bd is the source of both copies and neither edit
survives being regenerated.

Install the L3 gates — a `pre-push` hook that refuses `git push` in any
persona session whose PID denies it, and a `prepare-commit-msg` hook that
refuses a commit which does not name its own paths (every persona shares
this checkout and its index, so an unqualified commit takes whatever
another persona has staged).

That same `prepare-commit-msg` hook carries the beads **visibility guard**:
in a repo config `beads_visibility:` does not mark `private`, it refuses a
commit that adds instance-ops content — dollar figures, plan names, live
`plan_guard_*`/`budget_*`/`autostart_*` values, credential locations — to
`.beads/*.jsonl`. Unmarked is public, so mark your private beads repos
before you hook them (NOTES.md, *Privacy model*), or the first `bd sync`
carrying a cost figure is a refusal. `posse gates install-hooks` prints
which way it stamped each repo.

Both gates want a slot `bd hooks install` has already taken, and neither
tool yields: `posse gates install-hooks` refuses to overwrite a hook that is
not its own **and exits non-zero**, so its second hook never installs at
all; `bd hooks install` overwrites without asking. Running the two commands
in either order therefore leaves this repo with *no* posse gate
(rangerhq-f2p5). Install them under separate filenames and put a chain in
the real slot:

```sh
$ cd ~/src/<your-work-repo>
$ mv .git/hooks/pre-push           .git/hooks/bd-pre-push
$ mv .git/hooks/prepare-commit-msg .git/hooks/bd-prepare-commit-msg
$ posse gates install-hooks ~/src/<your-work-repo>
$ mv .git/hooks/pre-push           .git/hooks/posse-pre-push
$ mv .git/hooks/prepare-commit-msg .git/hooks/posse-prepare-commit-msg
$ cat > .git/hooks/pre-push <<'EOF'
#!/bin/sh
d=$(dirname "$0")
"$d/posse-pre-push" "$@" </dev/null || exit $?
exec "$d/bd-pre-push" "$@"
EOF
$ cat > .git/hooks/prepare-commit-msg <<'EOF'
#!/bin/sh
d=$(dirname "$0")
"$d/posse-prepare-commit-msg" "$@" || exit $?
exec "$d/bd-prepare-commit-msg" "$@"
EOF
$ chmod +x .git/hooks/pre-push .git/hooks/prepare-commit-msg
```

Copy that chain exactly. The gate runs **as its own process, and its exit
status is checked** — it is never appended to, in either slot. Appending is
not a chain: the gate's refusal is an `exit`, so a hook pasted after it
never runs at all, and worse, until rangerhq-0g1c an appended line
*discarded* the refusal — the hook printed `refused by posse gate` and exited
**0** while git pushed, so the operator's own check said the wall held
(rangerhq-kk6e). The gates now exit explicitly on every path, but the
dispatcher above is still the only form that runs both hooks. The
`</dev/null` keeps the gate off the ref list git feeds on stdin; `exec`
hands that stdin, untouched, to bd's shim, which reads it.

`posse gates install-hooks` prints this same chain, with the slot and paths
filled in, whenever it finds a hook that is not its own.

**Verify — by running the hooks, not by reading them:**

```sh
$ RHQ_PERSONA=probe RHQ_TOOLS_DENY='Bash(git push:*)' \
    sh -c 'printf "refs/heads/main a refs/heads/main b\n" | .git/hooks/pre-push origin x'; echo $?
$ t=$(mktemp); RHQ_PERSONA=probe .git/hooks/prepare-commit-msg "$t"; echo $?; rm -f "$t"
$ t=$(mktemp); env -u RHQ_PERSONA -u RHQ_TOOLS_DENY .git/hooks/prepare-commit-msg "$t" message; echo $?; rm -f "$t"
```
The first two must print `refused by posse gate: …` and exit **1**. The third
must print no refusal and exit **0**: the commit guard keys on
`RHQ_PERSONA`, so **your own commits in that tree are unaffected**. Inside a
persona session the safe form is `git commit -F - -- <paths>`. A gate that
prints its refusal but exits 0 is not installed — re-read the chain.

Persona launch runs the first two behavioral probes itself (both slots in
one shell invocation). Ownership markers still decide whether posse may
replace a hook, but they are never enforcement evidence: a working foreign
chain passes, while a slot that does not exit 1 is reported as `DEGRADED` and
the session refuses unless the operator explicitly allows degradation.

Re-running `bd hooks install` — after a `bd` upgrade, or in a second clone
— overwrites both slots and takes the chain with them. Run the three probes
again after any bd upgrade. Session create installs the two gates too, but
only into an empty or already-posse slot — which is why it leaves an intact
chain alone, and also why it cannot build one. In a repo where `bd hooks
install` got there first, both slots are bd's, so every install it attempts
refuses; and because session create makes them best-effort and discards the
error, it installs **nothing and says nothing**. That is the state
rangerhq-f2p5 was filed about — no push gate, no commit guard — reached
silently, by dispatch.
So the block above is not optional for a repo you dispatch into: it is the
only thing that puts the gates there. Until rangerhq-mgdk lands, run it, and
run the three probes, in every clone.

Health check, then wire it into the instance:

```sh
$ bd doctor
$ bd create "smoke: prove dispatch reaches a persona" -t task -p 2 -l <a label your persona claims>
```

Add the repo to `beads:` in `$RHQ_HOME/config.yaml` (step 5) if it is not
there already, then:

```sh
$ posse ready
```
**Verify:** your bead is listed, with the repo path on the right. If the
list is empty, either `beads:` does not name this repo or the bead's labels
match no persona — `posse ready` shows the queue, `posse dispatch --dry-run`
(next step) shows the routing.

> The first `posse ready` against a repo also *seeds* the verify-after
> watermark (`state/verify-after.<repo>`). That first sweep files nothing;
> from then on, closing a bead labelled `code` or `devops` earns an
> automatic `verify:` bead for your QA persona (ADR 0006 §3, config
> `verify_labels:` / `verify_assignee:`). Set `verify_labels: []` to turn
> it off.

---

## 10. First launch, by hand

Before dispatching anything, prove one persona can start at all.

```sh
$ posse new smoke --dir ~/src/<your-work-repo> --agent <persona>
$ posse list
```
**Verify:** the session appears with a live agent state
(`working`/`idle`/`blocked`) — not `unknown`. `unknown` means herdr did not
detect the CLI: check `make install-detection` (step 3) and that the CLI is
on PATH.

```sh
$ posse prompt smoke "say hello and stop" --wait
$ posse peek smoke 20
```
**Verify:** the agent answered. This session is marked **crew** (👤) —
yours, and dispatch will leave it alone.

```sh
$ posse kill smoke
```

A launch that refuses with a `DEGRADED` gate list is the wall doing its
job: either raise the cage tier, relax the PID, or launch with
`--allow-degraded` knowingly. Do not reach for `--allow-degraded` to get
past your first launch — read `posse gates <persona>` and fix the cause.

### Instance interstitials — the first-run dialogs only you can answer

**Read this before your first dispatch.** A CLI's first run in a fresh pane
draws a dialog: a consent banner, an update menu, a splash. herdr does not
recognize those screens, so a dispatched session sits there un-promptable
until its startup wait runs out, and the pass gets nothing (ADR 0013 §2;
measured in `ranger-base-3j8`). Claude's is a different shape and posse
answers it for you — see *the one posse answers*, below.

Posse **names these keys and never writes them** — with the single declared
exception below — and that is deliberate in both directions:

- one of the answers is a **privacy** decision about your own repositories;
- one of the *defaults* **mutates your machine**.

So nothing in posse blind-sends Enter at a fresh pane, and a dialog whose
default action mutates the machine is a **launch refuse** until your own
config silences it. `posse runtime check <name>` prints each one with its
key, its file, and whether it is already silenced on this box.

| runtime | screen | key | you do |
|---|---|---|---|
| grok | `Help improve Grok  [Opt out] [Opt in]` consent banner | `[privacy] privacy_banner_acked` in `~/.grok/config.toml` | click **[Opt out]** once, in your own grok session. **Never [Opt in]** — it lets xAI retain prompts and traces from sessions working in your private repos. Grok records only that you answered, not which way. |
| grok | New worktree / Resume session / Quit startup menu | `[cli] auto_update = false`, `maximum_version` in `~/.grok/config.toml` | already handled by the fleet pin, declared in `etc/grok/version-pin.toml`. `make verify-grok-pin` asserts it; NOTES.md *"grok substrate"* is the runbook for lifting it. |
| codex | `Update available! → 1. Update now  2. Skip  3. Skip until next version` | `dismissed_version` in `~/.codex/version.json` | pick **3. Skip until next version**: arrow **Down** twice, *verify the caret moved*, **then** Enter. The default-selected option is `1. Update now`, which runs `brew upgrade --cask codex` — an unreviewed roll-forward of a pinned tool. |
| claude | `Quick safety check: Is this a project you created or one you trust?` | `projects["<session dir>"].hasTrustDialogAccepted` in `~/.claude.json` | **nothing — the launch seeds it**, per session directory, because this one fires in every new directory and has no flag to answer it with. See below. |

The codex dismissal is good for **one release**: the menu returns as soon
as `latest_version` moves past `dismissed_version`. `posse runtime check
codex` prints both numbers, so you can see when it is due again.

**The one posse answers for you: claude's directory trust.** Claude asks
*"Quick safety check: Is this a project you created or one you trust?"* the
first time it runs **in a given directory** — so, unlike the rows above,
there is no answer you can give once. Every new repo, worktree, container
HOME and scratch dir asks again, and claude offers no flag and no settings
key to answer it on the launch line (measured on 2.1.241). The launch
therefore writes the key the CLI itself documents,
`projects["<session dir>"].hasTrustDialogAccepted`, into your
`~/.claude.json` — merged into the file, never rewritten from a template,
only for the directory it is launching in, and only when that directory is
not already trusted. It is the same grant posse already types on codex's
line. A `~/.claude.json` posse cannot parse **refuses the launch** rather
than being replaced; run `claude` in that directory once and accept the
dialog by hand if you would rather answer it yourself.

`posse runtime check claude` prints this row too, and its probe tells you
whether the directory you are standing in is already trusted.

---

## 11. First dispatch — the bead that proves the install

Dry run first. It routes and reports and launches nothing:

```sh
$ posse dispatch --dry-run --dir ~/src/<your-work-repo>
```
**Verify:** your smoke bead is shown routed to a persona, with the tier and
the rule that decided it. `unroutable` means no persona's `labels:` overlap
the bead's — fix the PID or the bead's labels, not the harness.

Then for real, one bead, bounded:

```sh
$ posse dispatch --dir ~/src/<your-work-repo> -n 1
```

What happens: a fresh session per bead named
`<persona>-<repo>-<beadid>`, with `BD_ACTOR`/`RHQ_RUNTIME`/`RHQ_TIER` and
the persona's env sets injected; an atomic claim; then the assembled work
prompt with `--wait`. The pass reports per bead: `✓` closed, `⛔` the agent
blocked, `◑` settled with the bead still open (go read the session).

```sh
$ bd show <bead-id>
$ posse list
```
**Verify — this is the install's acceptance test:** the bead is `closed`,
its `Assignee` is your persona, and it has a close comment the persona
wrote. A persona closed a real bead in a fresh instance; the install works.

Sessions of finished beads are left idle for you to reap (`posse kill`, or
`x` in the cockpit). They cost nothing and do not block the next pass.

A session whose bead is *not* finished is a different matter: `posse kill`
**refuses** one whose bead is still `in_progress` while its working
directory holds uncommitted work, and names both halves (ADR 0013 §4).
Look before you reap — `posse attach <session>` — then let it commit and
close, or `posse kill <session> --force` once you have read the refusal.

Useful while you watch: `posse peek <session>`, `posse cockpit` (or `ctrl+b g`),
`posse scorecard`, `posse cost`.

---

## 12. Arming the unattended loop (only when you trust the above)

Scheduled dispatch is `posse dispatch --watch` running in a herdr workspace,
armed once per herdr server start by the cockpit plugin's `[[startup]]`
hook. It is **disarmed** until `autostart_interval:` appears in your
config — the presence of that key is the arm switch. Only the default herdr
server may arm the fleet loop; named-session servers stand down even though
herdr's plugin registry is global.

The hook resolves which config that is exactly the way `posse` does (§4):
`RHQ_HOME` if set, else `~/.config/posse`, else an existing `~/.config/rhq`.
It exports the result, so the workspace it arms — and the dispatch loop
inside it — runs out of the same instance the arm decision was read from.

```yaml
autostart_interval: 5m       # ← the arm switch
autostart_max_beads: 3       # -n per pass. A cap is ALWAYS applied (default 3);
                             # 0 means unbounded, and only by saying so.
autostart_dry_run: true      # start here: passes route and report, dispatch nothing
# autostart_resume: false    # defaults ON — see below
# autostart_session: dispatch
# autostart_dir: ~
```

Arm it dry first, watch a few passes, then set `autostart_dry_run: false`.

`autostart_resume:` is the one key here that **defaults on**. Without it the
loop passes `--resume`, so a bead whose persona settled idle without closing
it gets re-prompted on the next pass. The alternative is the line
`◑ <id> settled "done" but issue is "in_progress" — review <session>`
scrolling past in a log addressed to the operator who armed this loop
precisely so they would not have to watch it. Set it to `false` if you want
that warning and nothing else.

**Set `plan_guard_5h:` / `plan_guard_7d:` before you arm anything.** They
are what keep an unattended loop off your plan's rate windows; under
`--watch` the guard also fails *closed* after `plan_guard_blind_max:` (10m
default) with no successful reading, so passes park rather than run blind.
Arming without the guard is arming a token loop nobody is watching.

Restart the herdr server (or run `plugin/autostart.sh` by hand) to arm.
Log: `$RHQ_HOME/state/dispatch-watch.log`.

---

## 13. Two instances on one machine

Supported by design, with named gaps. The model (ADR 0015): **one harness
install, one herdr server, N instances.** An instance is an `RHQ_HOME` plus
the bd repos its `beads:` lists.

Two invariants you must respect — the harness does not enforce them:

1. **A repo is served by exactly one instance.** The `beads:` sets of two
   instances are disjoint, and so are their `dirs:` work repos. Two
   instances dispatching one repo double-files verify beads, makes crew
   marks invisible across the boundary, and hard-refuses the second
   skills binder.
2. **Every herdr workspace has exactly one owning instance**, and the
   ownership record is that instance's `state/herdr/<name>.yaml` meta file
   — never a label pattern.

What to set in the second instance's `config.yaml`:

```yaml
instance: <short-name>       # DESIGNED, NOT YET IMPLEMENTED — see gaps below
beads:   [ ... ]             # disjoint from the other instance's
dirs:    [ ... ]             # disjoint
autostart_session: <name>    # distinct, if it ever arms
```

**Autostart is single-instance per herdr server, and that is deliberate.**
The `[[startup]]` hook runs once with the server's environment; whichever
`RHQ_HOME` that environment names is the armed instance. The second
instance runs its loop by hand, in a session it owns:

```sh
$ RHQ_HOME=<second home> posse dispatch --watch 5m -n 3
```

### Known gaps — read these before running two instances

As of this writing, coexistence is **not yet clean**. Four holes are
identified, designed, and open (a fifth — the seatbelt's hardcoded
`~/.config/rhq/state` grant — is closed: the profile derives the state
grant from the App's home, so a second `RHQ_HOME` gets its own state dir
and no grant into the default instance's; rangerhq-qfzr, ranger-base-cpyb):

| gap | what actually happens | bead |
|---|---|---|
| `RHQ_HOME` is not injected into sessions | a persona running `posse` **inside its own session** resolves the herdr server's env or the default-home lookup (`~/.config/posse`, then an existing `~/.config/rhq`) — the *wrong* instance's config, queue and skills, silently | rangerhq-ysly |
| labels carry no instance id | `instance:` is designed but **not implemented** — setting it today does nothing. Identical session names collide on the shared herdr: the second instance's create fails "already exists" | rangerhq-ouf9 |
| destructive paths do not refuse foreign workspaces | `posse kill` and cockpit `x` will close a workspace owned by the *other* instance; reachable today via the autostart hook's kill-and-replace | rangerhq-selx |
| `BeadsDirs()` falls back to `[""]` | with no surviving `beads:` entry, bd runs in the caller's cwd — silent wrong-queue dispatch | rangerhq-wmrb |

Until `ysly` and `ouf9` land, run two instances with your eyes open: give
sessions manifestly different names, do not let either instance's autostart
hook run while the other has live sessions, and treat any cross-talk you
hit as a harness bug worth filing — it is cheaper found here than on a work
machine.

Accepted and *not* fixed (ADR 0015 D5): the posse binary, the plugin
registration, the detection manifests and the default cage image tag are
machine-global. So are the plan-guard token and the transcript pile that
feeds `posse cost` — those are *account*-scoped, so two armed fleets consume
one budget and the caps become conservative, not wrong.

---

## 14. Troubleshooting

| symptom | cause | fix |
|---|---|---|
| `examples/ not found next to this binary (~/.local/examples)` | ran the installed `posse init` | run `./bin/posse-go init` from the repo checkout |
| `posse` writes to the wrong place | `RHQ_HOME` not exported in this shell | export it; put it in your shell profile |
| `posse list` shows `unknown` instead of an agent state | herdr did not detect the CLI | `make install-detection`; check the CLI is on PATH |
| launch refuses with a `DEGRADED` list | the wall cannot realize a PID gate on this runtime × cage | `posse gates <persona>` and fix the cause; `--allow-degraded` only knowingly |
| a persona with `skills:` refuses to launch | the runtime has no skill surface (template profile with no `skills_flag:`) | add `skills_flag:`, or drop the binding |
| session sits forever on a permission dialog | template runtime whose `command:` names no unattended flag | add the CLI's own unattended flag to `command:` (step 8) |
| `-c model= 'x'` in the launch line | `model_flag:` renders with a space | hardcode the model in `command:` — rangerhq-5p0d |
| `bd list` → "no beads database found" | a bd 1.2.x binary is on PATH | install 0.49.1; 1.2 does not read `.beads/beads.db` |
| bead never dispatches | no persona's `labels:` overlap it, or it is labelled `question` | `posse dispatch --dry-run`; `question` beads are for the operator and are never routed |
| `posse new <name>` → "already exists" | a workspace with that name is live — possibly another instance's | `posse list`; see §13 |

---

## 15. What this runbook does not cover

Deliberately out of scope, and not because they were forgotten:

- **Work secrets and the keychain question.** Parked on its own ADR.
- **A work-authorized inference engine.** Step 8 proves the *config path*
  for a novel engine; which engine, and in what form (CLI vs API-only), is
  a separate decision.
- **Cutting a release.** Step 2's `brew install` and the GitHub Release it
  pulls from are produced by `docs/runbooks/release.md` (rangerhq-i0n0) —
  the maintainer's procedure, not the deployer's. `homebrew-core` is
  deliberately not chased at launch.
- **Client-data and visibility rules** beyond the crew-wide line that
  instance and client work stays in its own private repo and archive
  content never leaves the machine.

## Where to read next

- `README.md` — the stack, and the build/promote model
- `DIRECTION.md` — why the harness is shaped this way
- `NOTES.md` — how it works under the hood; the substrate pins and their
  reasons live here
- `docs/adr/` — the decisions. 0001 personas · 0002 runtimes and gates ·
  0003 model tiering · 0005 work prompts · 0006 handoff shapes ·
  0007 skills binding · 0011 dispatch · 0012 harness/instance boundary ·
  0015 constitution promotion
