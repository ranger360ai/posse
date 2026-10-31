# ADR 0077 — a seat's environment is typed, not inherited: what posse reads to decide about a seat, posse writes onto it, and its own children run outside every gates dir

*Status: accepted 2026-10-31 (ranger-base-u6c3u, from ranger-base-c33j6) · owner: architect · fills the gap ADR 0076 §2 left (the slot is the client; this says whose ENVIRONMENT the seat has) · realizes ADR 0073's "through `Bd` for itself" · builds in the code beads named on ranger-base-u6c3u (dinesh)*

## Context

A seat is a child of the multiplexer SERVER (herdr's, or Rex's), not of the
posse process that asked for it. Its environment has three layers, and posse
can read only two of them:

| layer | who sets it | can posse read it? |
|---|---|---|
| base | the server, frozen at the moment some process autostarted it | **no** |
| overlay | posse, at create (`workspace create --env`, Rex's `/usr/bin/env K=V <shell> -i`) | yes — posse wrote it |
| line prefix | posse, on the typed runtime line (`GatePrefix`) | yes — posse wrote it |

MEASURED 2026-10-31 (ranger-base-c33j6; Rex CLI 0.0.1793377904, claude
2.1.316, posse a4afaa5a; the notes fragment on ranger-base-c33j6's own
branch): a probe session read `CLAUDE_CONFIG_DIR` and `RHQ_HOME` UNSET where
the launching seat had both set, and found a gates dir mid-PATH that the
launching process did not put there — the server had started hours earlier
from another seat, and donated that seat's PATH to every session it would
ever open. Three surfaces read the launcher's environment as though it were
the seat's:

1. **Trust seed.** `SeedClaudeTrust` writes the file `ClaudeConfigFile()`
   resolves from the LAUNCHER's `CLAUDE_CONFIG_DIR`; the seat resolved
   without it, read `~/.claude.json`, and opened on the trust dialog. Shedding
   the var so posse targets `~/.claude.json` instead fails in a
   `cage: seatbelt` launcher (`.posse-seed-*` temp file in `$HOME`: operation
   not permitted).
2. **Parity.** `ForeignGatesElements` checks the launcher's PATH, and the
   refusal says "relaunch with a clean PATH" — advice about a process that is
   not the seat's.
3. **bd claim.** posse takes ADR 0073's store lock, execs `bd` by PATH, the
   launcher's gates dir leads that PATH, and its `bd` shim takes the same lock
   on a new open file description: posse waits 30 s on itself and refuses,
   every dispatch from a gated seat (MEASURED twice).

And a fourth found while reading for this record:

4. **Rex shell.** `rexShell()` (muxrex.go) hands the Rex seat `$SHELL`. In a
   gated launcher `$SHELL` is that persona's gate shell (MEASURED in this seat:
   `.../gates/richard/shell/zsh`), so every Rex seat dispatched from a gated
   seat runs inside the LAUNCHER persona's gate wrapper. `realShell` in
   gates.go already refuses exactly this candidate (ranger-base-f0ay); rexShell
   does not call it.

The two claude config files on this box are both live: `~/.claude.json`
(1,376 projects) and `~/.claude/.claude.json` (696), both with an account.
MEASURED 2026-10-31. Which one a seat reads is decided by one variable and is
not an accident to paper over.

## Decision

**D1 — What posse reads to decide about a seat, posse types onto the seat.**
The base layer is unreadable, so no launch decision may depend on it. Every
environment variable posse reads from its own process to decide something
about a seat is written onto that seat with the value posse used, on the
typed runtime line, in both arms:

- set in posse's environment → `K=<that value>` on the line;
- unset → `/usr/bin/env -u K` on the line (unset, not empty: claude's
  directory resolver reads an empty `CLAUDE_CONFIG_DIR` as a cwd-relative
  path; trust.go `ClaudeConfigDirIn` documents why).

On the LINE and not in the overlay, because herdr's `--env` can set a
variable but not unset one, and the line is re-rendered by the same code on
every relaunch. Today that is `CLAUDE_CONFIG_DIR` for claude (trust file and
credential dir — one var pins both resolutions, since both are functions of
that var, `$HOME` and the filesystem) and `CODEX_HOME` / `GROK_HOME` for their
runtimes (interstitial.go reads them). `HOME` is not pinned: ASSUMED equal,
since both the launcher and the server run as the same user on the host, and
the container tier has its own home (`SeedCageHome`) and is out of scope.

Consequence for surface 1: the seat reads the file posse seeded, whichever
it is. A launcher that has `CLAUDE_CONFIG_DIR` set (every claude seat)
seeds `$CLAUDE_CONFIG_DIR/.claude.json`, which a seatbelt seat CAN write
(MEASURED, c33j6's first attempt). A caged launcher with the var unset still
cannot write `$HOME`; that refusal stays, and its message names the cage and
the two ways out (set `CLAUDE_CONFIG_DIR`, or dispatch from an uncaged shell)
instead of a bare `operation not permitted`.

**D2 — The seat's base PATH is typed too: the launcher's PATH outside every
gates dir.** The overlay carries `PATH=<posse's PATH with every element that
resolves into any gates dir removed>`, so `GatePrefix`'s `"$PATH"` expands
over a base posse wrote rather than the server's. "Resolves into" is
`ForeignGatesElements`' symlink-resolving test, not `PathOutsideGates`'
spelling test, and the two become one function. The launcher's own gates dir
is foreign to the hired seat and is removed with the rest: a seat's walls are
its own PID's (ADR 0017), and the launcher's shims behind them were a parity
violation the waiver hid, not extra safety.

Consequence for surface 2: parity's foreign-gates row is satisfied by
construction, so it is replaced by an assertion over the PATH posse types
(which can only fail on a bug in the strip) and "relaunch with a clean PATH"
is deleted. Surface 4 is the same rule for `SHELL`: `rexShell` uses
`realShell`, which already refuses a gate wrapper.

**D3 — posse's own children run with that same gates-free PATH.** `bd`,
and every other binary posse execs as part of its own protocol, gets
`PATH=<D2's gates-free PATH>` in `cmd.Env`, and its binary is resolved
against it. The walls bind what a persona TYPES (ADR 0009); posse's calls
are posse's, and ADR 0073 already serializes posse's bd writes in Go —
its own text says the lock is taken "through the shim for personas and
through `Bd` for itself". Setting the child's PATH, not only resolving the
binary outside, is deliberate: bd's own git hooks call `bd` by name, and a
hook child inheriting the launcher's PATH would reach the shim and deadlock
the same way (ASSUMED — the measured deadlock was the direct call).
`RHQ_BD_BIN` keeps precedence. Scope: `Bd` now (measured); the other exec
sites are a census bead, each judged on whether it is protocol or a
persona's command.

## Consequences

- A seat's environment stops depending on which process happened to start
  the server. Nothing needs a server restart to be correct.
- On herdr homes too, the seat's base PATH becomes the launcher's (cleaned)
  rather than the herdr server's. A launcher with a thin PATH (a launchd
  `dispatch --watch`) hands seats a thin base; the interactive shell's rc
  files still run on top. ASSUMED harmless; the live verify reads it.
- The seat runs with whichever claude state the launcher would, so a
  dispatch from inside a claude seat and one from an operator shell can open
  seats on different claude config files. That is the launcher's choice made
  visible, not a new divergence: it already exists on this box.
- Exit hatch: everything here is three render sites (line prefix, overlay
  PATH, `Bd` env). Reverting is deleting them; no state is held.

## Alternatives rejected

- **Seed both candidate files.** Writes a file nobody named (trust.go's own
  rule), and in a cage one of the two writes fails anyway — it trades a
  wrong file for a partial write.
- **Detect the disagreement and stand down loudly.** Cannot be built: the
  disagreement is with the server's environment, which posse cannot read.
  A detector would have to guess, and guessing is how surface 1 happened.
- **Read the server's environment** (`ps -E` / `KERN_PROCARGS2` on the
  server pid). Measures the base at one instant; `/bin/ps` is setuid and a
  caged seat cannot exec it (AGENTS.md, ranger-base-yxmwx), and whether the
  sysctl route reads another process's env from inside a cage is unmeasured.
  Even when it works, it learns a value posse would then have to override
  anyway. Typing it is cheaper and total.
- **Restart the multiplexer server from a clean environment on each
  dispatch.** Kills every other seat on that server. Rejected outright.
- **Strip gates dirs in-shell on the typed line instead of typing the base
  PATH** (the PRE guard's spelling test, extended). Keeps the server's PATH
  for everything else and still cannot see a symlink alias — the case
  `ForeignGatesElements` exists for. Also costs bytes on a line Rex cuts at
  1,024 (the overlay rides argv and has no such cut).
- **Make the shim's lock re-entrant** (posse tells the shim, by env, that an
  ancestor holds the store lock). The clever one. Any process can set an
  env var, so it becomes a way for a persona to skip ADR 0073's
  serialization; passing the open descriptor instead works but couples the
  shim to posse's fd layout. D3 removes the second lock taker instead of
  teaching it to recognise the first.
- **Keep "relaunch with a clean PATH" and document it.** Advice to a human
  about a third process, and the hazard it names is the one D2 removes.
