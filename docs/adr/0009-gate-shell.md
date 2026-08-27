# ADR 0009 — The gate shell: L1 survives a runtime that re-execs a login shell

*Status: accepted 2026-08-18 · owner: architect · amends ADR 0002 §3–4 ·
amended 2026-08-18 (§1 guard form, §4 rc files), 2026-08-27 (§1 REAL
must resolve outside every gates dir — ranger-base-f0ay)*

> Restated from the private archive of the instance this harness was
> developed in; incident citations reference that instance's history.
> Persona names are restated as roles; command examples use role-named
> example personas.

## Context

ADR 0002 put the L1 shim dir on the **typed command line**
(`PATH=<gates>/bin:"$PATH" <cmd>`) because macOS `path_helper` reorders a
PATH set in the workspace env. That holds only while the runtime runs
commands in the shell it inherited. Grok 1.0.5 does not: it starts the
operator's **login** shell, captures its state (PATH, env, functions,
aliases) and replays that snapshot into a fresh shell per command — so
`path_helper` runs, `/usr/bin` lands ahead of the gates dir, and in a
live developer-persona grok session `command -v git` answered
`/usr/bin/git`. Codex has the same trap and a knob
(`allow_login_shell=false`); grok has none that works. Today
`Runtime.LoginShellPATH` marks grok and `CheckParity` counts no
`Bash(…)` deny as realized there except `git push` (L3 hook), so any
other shell deny degrades on grok. That is honest, but it means "the wall
is ours on every runtime" (ADR 0002) is false on one of three.

Verified on the development box (2026-08-18, headless probes; argv
logged by a wrapper installed as `$SHELL`):

| runtime | shell resolved from | how it invokes the shell |
|---|---|---|
| grok 1.0.5 | `$SHELL` (`GROK_SHELL` also read; dialect picked by **basename**: `zsh`/`bash`) | login capture `zsh -lc 'source ~/.zshrc; printf PATH; env -0'` and per command `zsh -c '<replay snapshot>' -- '<user cmd>'` (bash: `-O extglob -c … -- …`) |
| claude | `$SHELL` | `zsh -c -l '<snapshot script; eval cmd>'`; its snapshot restores the process PATH, so the typed prefix already wins |
| codex 0.147 | not called at all under `-c allow_login_shell=false` | `/bin/bash -c` directly |

A wrapper named `zsh` that prepends the gates dir to `PATH` **inside every
`-c` string** made grok resolve `git` — and a subprocess `sh -c "git
--version"` — to the shim; claude kept working; codex never invoked it.

## Decision

**1. L1 gains a second rendered artifact: the gate shell.** At every
launch, next to the shims, the launcher renders
`RHQ_HOME/state/gates/<persona>/shell/<basename>` — a POSIX `sh` script
whose only job is `exec <real shell> "$@"` after guarding PATH:

- `<real shell>` = `$SHELL` if its basename is `bash` or `zsh` **and it
  is not itself a rendered gate wrapper**, else the first `zsh`/`bash`
  found on PATH outside every gates dir, else `/bin/sh`; the wrapper is
  **named with that basename** so a runtime that picks its snapshot
  dialect from the name (grok does) picks right.
  **A wrapper's `REAL` must resolve outside every gates dir**, and a
  render that cannot satisfy that is refused rather than written. A
  wrapper is installed as `shell/zsh`, so it has a shell's basename and
  stats like one: while `<real shell>` was chosen by name and stat alone,
  any render running under a gated context — dispatch's own relaunch
  pass, measured — captured the *calling* persona's wrapper as `REAL`,
  and wrappers chained persona-to-persona. On 2026-08-27 the chain closed
  into a two-node cycle: every spawn entering it `exec`-looped, prepending
  its guard to the `-c` string each hop until `E2BIG` ~40 minutes later,
  and every Bash call in every session hung with zero output for two
  hours. The refusal is at both arms of the resolution and again at the
  render (ranger-base-f0ay).
- It walks argv the way a shell does (leading `-x`/`+x` words are options;
  `-o/-O/+o/+O/--rcfile/--init-file` consume a value; `--` ends them). If
  a `-c` was seen, the **first operand is the command string** — Claude's
  `-c -l STR`, grok's `-lc STR` and `-c STR -- CMD` all land here — and
  the guard is prepended to it. If the operand after that (argv0) is
  `--`, the next word is a runtime's *user-command slot* (grok's shape):
  the guard is prepended there too, so it runs **after** the snapshot
  replay, where an rc file that re-prepended a dir cannot outrank it.
- The guard is one line, valid in bash and zsh, and it asserts
  **precedence, not presence**:
  `case "$PATH:" in "<bin>":*) ;; *) PATH="<bin>:$PATH";; esac; export PATH;`
  This ADR first wrote the obvious "idempotent" spelling — `case ":$PATH:"
  in *":<bin>:"*)`, is the dir *anywhere* on PATH — and it did not hold
  on the runtime it was written for: the typed line has
  already put the gates dir on PATH, so `path_helper` (via
  `/etc/zprofile`, which runs before any `-c` string) **re-orders** it
  below `/usr/bin` rather than dropping it. A presence test then sees
  "present" and does nothing — a no-op exactly when it is needed. Live on
  grok 1.0.5 the wrapper rendered, was invoked on the predicted argv, and
  `command -v git` still answered `/usr/bin/git`. The precedence test
  costs one duplicate entry in PATH; lookup takes the first. Do not
  "simplify" it back: any "already set?" guard that runs after a reorder
  has this trap.
  When the guard fires in the user-command slot it appends one line to
  `gates/<persona>/shell.log` — that means the replayed PATH did not have
  the gates dir *first* (rc or `path_helper` reordering), which is worth
  seeing.
- Anything else (interactive `-l -i`, a script path, no `-c`) passes
  through untouched. A mis-parse is a **loud** failure (the persona's
  shell breaks), never a silent bypass — the property we want.

**2. The typed line sets `SHELL` on every runtime.** `GatePrefix` becomes
`PATH=<bin>:"$PATH" SHELL=<gate shell> GROK_SHELL=<gate shell> <cmd>`.
Uniform, not per-runtime: claude is unaffected (verified), codex never
calls it (verified), grok is fixed, and a *future* runtime or version
that starts snapshotting a login shell inherits the fix instead of a
silent regression. `Runtime.LoginShellPATH` and its `CheckParity`
special case go away; L1 is counted on every runtime again. Exit hatch:
`Runtime.NoGateShell` (and `gate_shell: false` in a template-only
`runtimes/<name>.yaml`) drops the two vars for a runtime that chokes on
a wrapper — the parity check then falls back to unrealized for `Bash(…)`
denies there, exactly as today's grok.

**3. What this does not claim.** L1's known holes stand (`/usr/bin/git`,
`command -p`, `PATH=/usr/bin:$PATH git`, `env -i`); L3 remains the wall
for `git push` and the seatbelt/container tiers for the rest. New
residual: on a *persistent-shell* backend (grok has one; 1.0.5 defaults
to a shell per command) only the login-capture guard applies, so an rc
that prepends a dir holding a denied verb *after* our guard could still
win — `shell.log` stays silent then, and QA's live check is the
detector. Grok's `login_shell_capture` overlay, `GROK_LOGIN_ENV`, and an
upstream `allow_login_shell` are no longer needed.

**4. The persona's own rc files run gated — intended, no carve-out.** A
consequence §1 did not name: the guard is prepended *ahead of* the login
capture's `source ~/.zshrc`, so every command an rc file runs in a
persona session resolves through the shims. Observed: a probe PID
denying `rm -rf` refused and logged oh-my-zsh's `rm -rf
~/.oh-my-zsh/log/update.lock`. Decision: keep it. The rc runs under the
persona's identity, in the persona's session; whatever it does, the
persona did. A carve-out ("commands from the rc are trusted") would also
be a bypass channel — nothing stops a model from writing `git push` into
`~/.zshrc` and letting the next login capture run it ungated — and the
wrapper cannot tell an rc-issued command from any other without matching
the CLI's capture string, i.e. the per-runtime knowledge this ADR
exists to avoid. Two things change and are written down:

- **A `refusals.log` line means "this persona's session ran it", not "the
  model typed it".** The command text is in the line; an rc-shaped one
  (`update.lock`, a `brew`/`nvm` bootstrap) is the operator's rc, not the
  model. Reading the log as intent needs that distinction.
- **An rc that runs a denied verb degrades under the persona.** The shim
  refuses and exits non-zero; login shells do not run `set -e`, so the rc
  continues and the session survives — the verb's side effect simply does
  not happen (oh-my-zsh keeps its stale lock; harmless). An rc that
  *depends* on a denied verb's output is broken for that persona; the fix
  is in the rc, not the gate: guard the line with
  `[ -z "$RHQ_GATES_DIR" ] && …`. Denies are chosen per PID and the
  operator's rc is one file; that asymmetry is why the carve-out is not
  worth its bypass.

## Consequences

- `gates.go`: `renderGateShell` (+ real-shell resolution, ~40 lines of
  script as a Go string), `GatePrefix(binDir, shell string)`,
  `RenderGates` returns the shell path; `RHQ_GATES_DIR` unchanged.
- `runtime.go`: drop `LoginShellPATH`; add `NoGateShell`; grok's
  realizer notes drop the "L1 does not hold" caveat.
- `parity.go`: drop the login-shell branch; `Bash(…)` denies realize by
  L1 on any runtime without `NoGateShell`.
- Tests: a table test drives the rendered script with a fake real shell
  that prints argv — cases: `-c -l S`, `-lc S`, `-c S -- C`,
  `-O extglob -c S -- C pfx`, `-l -i`, `-c -- S`, `script a b`,
  `-o errexit -c S -- y`, `--login -c S`, `-ic S` (the probe's own set).
- NOTES.md: rewrite "L1 does not hold on grok" and the runtime table;
  the codex `allow_login_shell` note gains "…and every runtime now gets
  the gate shell besides". The security posture judgment reads this ADR:
  the wall is ours on every runtime again, by construction rather than
  by knob.
- `RelaunchAgent` re-types the same wrapped line, so nothing new to do.
- The architect's standing orders: the "put PATH on the typed line"
  lesson gets its second half — a runtime that re-execs a login shell
  needs the shell itself to be ours.

## Alternatives rejected

- **Ask upstream for `allow_login_shell` / make the config overlay work.**
  Even if it landed, it is a per-runtime knob we would
  have to know about for each CLI and version — the very policy-engine
  shape ADR 0002 rejected. The gate shell is one artifact, ours.
- **Accept the degrade.** Honest, but leaves every non-`git
  push` shell deny unrealized on grok forever, for a fix that is forty
  lines of `sh`.
- **Denied verbs as shell functions in the snapshot.**
  Grok's replay does restore functions, but functions do not reach
  subprocesses (`sh -c "git push"`, make, scripts) — PATH does; verified
  the subprocess case hits the shim with the PATH guard.
- **`ZDOTDIR` / `--rcfile` indirection** to inject after the rc files.
  zsh-only (bash login shells have no equivalent), and rc files that
  reference `$ZDOTDIR` themselves break. The `-c` guard is
  dialect-independent and needs no rc file of ours.
- **Making PATH `readonly` before the rc runs.** Kills every legitimate
  rc prepend (`~/.grok/bin`, brew) for the persona; too blunt.
- **A grok-only `SHELL=` typed by a runtime flag** (the boring version).
  Cheaper today; but the flag is exactly the kind of per-runtime knowledge
  that drifted silently last time. Uniform costs nothing on claude/codex
  (verified) and turns "which runtimes need it" from a table we maintain
  into a property of the launch.
- **Rewriting only grok's user-command slot** (skip the `-c` guard).
  Guaranteed-last, but tied to one CLI's calling convention; the `-c`
  guard is what fixes the snapshot for any shape, the slot guard is the
  belt on top.
- **Exempting the login capture from the `-c` guard** (so rc files run
  ungated; §4). Bypass channel via a model-edited rc, needs the CLI's
  capture string to recognise, and drops the only guard that applies on
  a persistent-shell backend (§3). Rejected; rc lines that need a denied
  verb opt out with `[ -z "$RHQ_GATES_DIR" ]`.

## Verification (QA's checklist, additions to ADR 0002's)

5. `posse new --agent developer --runtime grok`: inside, `command -v git` →
   `…/gates/developer/bin/git`; `sh -c 'git push --dry-run'` prints the
   gate refusal and lands in `refusals.log`; `/usr/bin/git push` is still
   refused by the pre-push hook. `posse list` shows the persona on grok
   clean, not degraded, for a PID whose deny includes a non-`git push`
   shell verb.
6. Same `command -v git` on claude and codex sessions still answers the
   gates path (no regression); `gates/<p>/shell/<basename>` exists and
   `shell.log` is absent (no file) after a normal session.
7. Repeat 5 after any grok/claude self-update — the shapes in the table
   above are what the wrapper recognises.
8. On a PID denying a verb the operator's rc uses (e.g. `rm -rf` with
   oh-my-zsh), a fresh grok session's `refusals.log` shows the rc line
   and the session still works — that is §4 holding, not a bug. A
   `refusals.log` line whose command is not rc-shaped is the model.
