# posse — the Ranger work-system harness (herdr-native)

**posse** is the business harness of the Ranger work system: it
knows who your agents are (personas), what environment they run in (env sets),
how they launch (recipes), and what they should work on (beads). It is built
on shared substrates rather than competing with them:

```
posse        the harness (bespoke, this repo)
             personas · env sets · recipes · dispatch · cockpit
  │
  ├─ beads   work substrate — dependency-aware task graph, agent mail,
  │          project memory        github.com/gastownhall/beads (bd)
  │
  ├─ herdr   presentation & oversight — workspaces, live agent state
  │          (working/blocked/idle)         herdr.dev
  │
  └─ agent runtimes — claude code, codex, … (interchangeable labor)
```

Sessions are herdr workspaces (`posse new/attach/kill`), work is submitted
through the session's detected agent (`posse prompt --wait`), and ready work
comes from the repo's beads database (`posse ready`). The cockpit
(`posse cockpit`) is a herdr plugin pane: sessions sorted blocked-first, with
the ready queue beneath. See [DIRECTION.md](DIRECTION.md) for the
architecture and [NOTES.md](NOTES.md) for how it works.

Persona design credits the [DISCOVER framework](https://discover-framework.ai/): the Persona Intent Document ([ADR 0001](docs/adr/0001-persona-intent-documents.md)) takes its name and its persona · intent · tools · guardrails · metrics binding from that framework's Specify artifact.

The original Ghostty + tmux session manager (bash + Go, launcher TUI, 2×2
grid) lives on the **tmux-reference** branch, kept working as the reference
implementation.

## Requirements

- [herdr](https://herdr.dev) ≥ 0.8 with its server running
- [beads](https://github.com/gastownhall/beads) (`bd`) for the work graph —
  **0.49.1 exactly**; anything from 0.51 up — brew's `beads` included —
  does not read `.beads/beads.db` at all
- Go ≥ 1.26 to build (`make build`); one Go dependency (`golang.org/x/term`)

Neither substrate ships with posse and neither is optional — `posse new`
dies on its first call without herdr. [INSTALL.md §1](INSTALL.md) is where to
get both, pins and reasons included.

## Quick start

Standing up a **new instance** from scratch — build, `RHQ_HOME`, crew,
queue, first dispatch — is [INSTALL.md](INSTALL.md). The short form:

```sh
make build                       # dev build of the working tree → bin/posse-go
make install                     # clean build of HEAD, then promote → ~/.local/bin/posse
export PATH="$HOME/.local/bin:$PATH"   # ← where the line above wrote it
posse init                       # seed $RHQ_HOME (default ~/.config/posse) from the
                                 # examples: examples/ beside the binary when there
                                 # is one, else the copy embedded at build time
mkdir -p ~/code/myproj           # --dir must exist; point it at a project of yours
posse new myproj --dir ~/code/myproj --cmd claude
posse list                       # live agent state per session
posse prompt myproj "fix the failing test" --wait
make link-plugin                 # register the cockpit with herdr (runs the installed posse)
```

`make install` writes to `~/.local/bin` (`BINDIR=…` overrides), which is on no
default macOS or Linux `PATH` — Debian's `.profile` prepends it only when the
directory already existed at login, and `make install` creates it mid-session.
Skip that export and `make install` exits 0 with `installed: …/posse` and the
next command is `posse: command not found`; the target says so on stderr when
it happens. Put the line in your shell's rc file, not just this shell.

Without a checkout the binary installs from the module path — and lands in a
directory your shell does not search:

```sh
go install github.com/ranger360ai/posse/cmd/posse@latest
export PATH="$(go env GOPATH)/bin:$PATH"   # ← where the line above wrote it
posse init
```

`go install` writes to `$GOBIN`, or to `$(go env GOPATH)/bin` when `GOBIN` is
unset — normally `~/go/bin`, which is on no default macOS or Linux `PATH`.
Skip that second line and the very next command is `zsh: command not found:
posse`, with the install itself having exited 0. Put it in your shell's rc
file, not just the current shell. That binary carries the seed tree embedded,
so `posse init` needs no repo beside it. `@latest` resolves to the newest
release tag — currently `v0.3.0` — which trails `main`, so what installs is
the tag, not the tree whose README you are reading. A known stamping bug
makes that build's `posse version` still report `0.3.0+dev`; `go version -m
$(command -v posse)` shows the module version that actually installed. `make
install` stays the path for a fleet, because its build has a commit to name.

`posse version` prints `0.3.0+<sha>[-dirty]` for a build made here, and the
cockpit header shows the same, so "which build is live" is one glance. `make
build` never touches the live binary; only `make install` does, and that
target is denied to fleet personas in `.claude/settings.json` — a human
promotes.

Personas share this checkout, so the working tree usually holds somebody's
unfinished edits. `make install` therefore never builds the working tree: it
checks HEAD out into a throwaway `git worktree`, builds there, and stamps that
sha — so the promoted binary is always a commit you can name, and never
carries (or fails on) an in-flight edit. Uncommitted paths are listed on
stderr when this happens; commit them and re-run if they belong in the build.
`make release` does the same build without promoting, and `BINDIR=…` overrides
the install location. Outside a git repo the build refuses rather than produce
an unidentifiable binary.

**The prose has the same promotion step** (ADR 0015). `posse promote <dir>`
copies the constitution — `agents/`, `config.yaml`, `recipes/`, `skills/` —
out of a repo **at a commit** into `$RHQ_HOME`, and records `{source, sha,
sha256 per file}` in `promoted.json` beside it. Every launch re-hashes the
promoted set against that manifest: a dispatched session refuses on a
mismatch, an interactive one warns DEGRADED, and re-promoting clears it. So an
edited PID is a draft until somebody ratifies it, and what gets ratified is a
diff — promote prints `git diff <last promoted>..HEAD` over those four paths
before writing anything (`--dry-run` prints it and stops). Like `make
install`, it is a human's: every shipped PID denies `Bash(posse promote:*)`
and promote refuses under a persona env marker. It never touches `envs/`
(gitignored secret values — no commit to promote from), `state/`, or
`personas/`.
