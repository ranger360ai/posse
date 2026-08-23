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
  │          project memory        github.com/steveyegge/beads (bd)
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

The original Ghostty + tmux session manager (bash + Go, launcher TUI, 2×2
grid) lives on the **tmux-reference** branch, kept working as the reference
implementation.

## Requirements

- [herdr](https://herdr.dev) ≥ 0.8 with its server running
- [beads](https://github.com/steveyegge/beads) (`bd`) for the work graph
- Go ≥ 1.26 to build (`make build`); one Go dependency (`golang.org/x/term`)

## Quick start

Standing up a **new instance** from scratch — build, `RHQ_HOME`, crew,
queue, first dispatch — is [INSTALL.md](INSTALL.md). The short form:

```sh
make build                       # dev build of the working tree → bin/posse-go
make install                     # clean build of HEAD, then promote → ~/.local/bin/posse
./bin/posse-go init              # seed $RHQ_HOME (default ~/.config/rhq) from examples/
                                 # — must be the repo build: init finds ../examples
posse new myproj --dir ~/code/myproj --cmd claude
posse list                       # live agent state per session
posse prompt myproj "fix the failing test" --wait
make link-plugin                 # register the cockpit with herdr (runs the installed posse)
```

`posse version` prints `0.3.0+<sha>[-dirty]`, and the cockpit header shows the
same, so "which build is live" is one glance. `make build` never touches the
live binary; only `make install` does, and that target is denied to fleet
personas in `.claude/settings.json` — a human promotes.

Personas share this checkout, so the working tree usually holds somebody's
unfinished edits. `make install` therefore never builds the working tree: it
checks HEAD out into a throwaway `git worktree`, builds there, and stamps that
sha — so the promoted binary is always a commit you can name, and never
carries (or fails on) an in-flight edit. Uncommitted paths are listed on
stderr when this happens; commit them and re-run if they belong in the build.
`make release` does the same build without promoting, and `BINDIR=…` overrides
the install location. Outside a git repo the build refuses rather than produce
an unidentifiable binary.
