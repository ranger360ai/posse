## bob reads the posse namespace, symlinks and all — and is in drift the day it ships (ranger-base-6r1rf)

ADR 0007 §6 gave bob a posse-owned cwd skill root,
`.bob/plugins/posse/skills/<name>` (§6d), on a STATIC read of bob 2.0.5's
shipped bundle — the string literals in
`/opt/homebrew/lib/node_modules/bobshell/dist/bob.js` name it as one of the
workspace roots. A static read cannot prove a live discovery, so §6b left
bob UNPROVEN: a `skills:` PID on bob degraded the launch and refused without
`--allow-degraded`. This bead is the live probe that answers it, and the two
facts §6's Evidence had ASSUMED: does bob actually list a skill planted
there, and does it follow a symlink out of the workspace?

Both arms matter. posse renders a cwd root as SYMLINKS into
`RHQ_HOME/skills` (§6d's provenance rule — an entry is posse's iff its
target is under `RHQ_HOME/skills`), so the symlink arm is not a nicety: it
is the binding. A real-dir-only result would have forced bob onto the
copy-in shape claude's flag tree uses (`RenderClaudeSkills`, after
ranger-base-65rc measured grok validating a symlinked plugin tree and
surfacing zero skills).

### Method

**MEASURED 2026-10-31 by the operator (Dave), bob 2.0.5 (`commit:
2dc180906`)**, `bob chat --workspace . --trust` in a scratch workspace, read
off the TUI's own `/skills` listing. **No chat turn was sent** — the listing
is local, and hard risk line 1 (no autonomous spending) is why the recipe
stops at `/skills` and exits with Esc.

| arm | fixture | result |
|---|---|---|
| a. real dir | `<ws>/.bob/plugins/posse/skills/probe-a-realdir/SKILL.md` | **LISTED**, location `local` |
| b. symlink out | same path, `probe-b-symlink` → a dir OUTSIDE the workspace | **LISTED**, location `local` |
| c. control | `<ws>/.bob/skills/probe-c-control/SKILL.md` | **LISTED**, location `local` |

All three arms discovered. The control is what makes (a) and (b) readable as
bob's behaviour rather than as a fixture that was wrong: `.bob/skills` is the
one root both boxes already agreed bob reads (§6's rejected-alternatives
entry), so a listing that named only the control would have said the probe
worked and the posse namespace did not.

**Why the operator ran it and not this seat** (ranger-base-lfk7t carries the
recipe verbatim). A `cage: seatbelt` seat cannot run `bob chat` at all, by
two different walls:

1. With the real `~/.bob`, every launch dies before writing any log file —
   `Fatal error: Session store is not initialized`. Direct writes confirm
   the cause: `touch ~/.bob/logs/shell/x` and `touch ~/.bob/db/x` both
   answer `Operation not permitted`. `~/.bob` joins a writable set only when
   posse renders the profile for a bob-runtime LAUNCH (`rt.StateDirs`), and
   bob has no such launch today — `posse runtime check bob` refuses on the
   herdr 0.8.2 detection gap — so no posse-dispatched route grants it either.
2. Redirecting `$HOME` to a writable scratch dir clears that (TaskStore
   initializes) and then demands a fresh browser sign-in a headless seat
   cannot complete.

That is the bead's own stop condition, and it is worth keeping: bob is the
one built-in whose skill surface has no non-interactive reader at all
(`bob --help`: chat, run, mcp, acp — §6's "detect discovery after launch"
rejection), so it cannot join `TestQALiveSkillDiscoveryPerRuntime`
(`RHQ_PARITY_LIVE=1`), which probes codex with `codex debug prompt-input`
and grok with `grok inspect`. bob's verification is a human reading a TUI,
and re-verifying it costs an operator session.

### The version string is BARE, on purpose

`skills_verified:` is compared VERBATIM against `ProbeCLIVersion`'s read
(§6c, `runtimeprobe.go`) — the first non-empty line of `<exe> --version`,
trimmed. The CLIs do not agree on what goes in that line:

```
$ codex --version     ->  codex-cli 0.153.4
$ grok --version      ->  grok 1.0.52 (0920c8169e27) [stable]
$ bob --version       ->  2.0.7
                          commit: e1a2a9f87
```

So bob's field is `"2.0.5"`, not `"bob 2.0.5"`. Writing the runtime's name
into it would read naturally in the grid (`verified bob 2.0.5`) and be
permanent false drift against every bob that ever installs.

### Drift, immediately

MEASURED 2026-10-31, this box: `bob --version` reports **2.0.7** (`commit:
e1a2a9f87`) against the probe's 2.0.5. So bob's declaration is in drift
under §6c the day it ships — exactly codex's position when §6 landed
(`0.147.0` declared, `0.153.4` installed, resolved by re-measurement in
ranger-base-bdias). §6c reports and never refuses, and the remedy it names
is a re-measurement, so:

- the field stays at what was MEASURED (`2.0.5`) — the honest value is the
  one a probe produced, never the one the box happens to have installed;
- `posse runtime check bob`'s skills row and a launch's Realized detail both
  carry the drift line, naming both versions;
- **ranger-base-bs49e** is filed for the operator to re-run the three arms on
  2.0.7, since this cage cannot.

Whether 2.0.5 and 2.0.7 differ here is unknown and deliberately unasserted:
a version is the only thing a cwd-root declaration can honestly key on,
which is the whole of §6.

### Changes

- `internal/posse/runtime.go`: bob's built-in gains
  `SkillsVerified: "2.0.5"`, and the comment above it carries the
  measurement, the two arms, why the value is bare, and the standing drift.
- `internal/posse/skills.go`: `BobSkillsRoot`'s doc comment no longer says
  the root carries no `SkillsVerified`.
- `internal/posse/bobruntime_qa_test.go`: the bob-grid pin's skills want
  moves off `UNPROVEN (never measured live)` to the verified row, and is now
  anchored on the VERSION as well as the root — the failure the second
  substring exists to catch is the field going empty again, which silently
  re-degrades every `skills:` PID on bob while the row still names the right
  path.
- `docs/adr/0007-skills-binding.md`: both tables' bob rows, §6a's built-in
  list (plus the verbatim-comparison rule for the value), §6b's "that is bob
  today" (the rule now binds the next `skills_cwd:` runtime an instance
  declares, not a built-in), the Evidence ASSUMED line → RESOLVED with the
  arm-by-arm reading, a MEASURED line for the installed-2.0.7 drift, and the
  `.bob/skills` rejected-alternative (the fallback is unused rather than
  pending).

### Filed, not fixed

- **ranger-base-bs49e** (operator, `-l question`) — re-run the three arms on
  the installed 2.0.7, §6c's remedy for the drift above.
- **ranger-base-xzgz3** (`-l code,debt`, P3) — the flip makes bob's binding
  realizable and `qmSurfaces` (arm 3's skills table, one row per runtime
  DECLARATION) still has no bob row. It could not have had one before: every
  `cwd` row asserts `len(p.Degraded) == 0`, which bob failed by design while
  it was UNPROVEN. Adding it is not a five-line diff — the `cwd` branch
  hardcodes `AgentsSkillsDir(dir)` and `AgentsSkillsPath`, and the row ends
  in a `mustCreate` for a runtime whose launch refuses by name on the herdr
  detection gap.

No CHANGELOG entry: `## Unreleased` does not exist since v0.5.3, and the
flip changes nothing a user can reach today — bob's own launch still refuses
on the herdr detection gap, so no `skills:` PID on bob could launch to be
degraded in the first place.

### Verified

MEASURED 2026-10-31, this box, all green:

- `go build ./...`, `make fmt-check` (exit 0), `python3
  scripts/notes-index.py --check`.
- Focused pins, each confirmed to have actually RUN under `-v` rather than
  passed as a filter matching nothing: `TestQABobGridDeclaresEveryRow`
  (0.88s — the grid now says both `symlinked at launch and ADDITIVE` and
  `the LINKS are the binding (verified 2.0.5)`),
  `TestQASkillsTreeAtASessionDirThatIsTheHomeRefusesUnderADeclaredGlobalRoot`,
  `TestQABobsSkillsTreeAtTheHomeRefusesUnderItsOwnGlobalRoot`,
  `TestSkillsCwdIsDeclarable`, `TestSkillsCwdPathFormAndVerification`,
  `TestSkillsVerifiedDriftIsReportedNotRefused`,
  `TestSkillsFlagTakesAPrintfForm`.
- The full `make test`: all thirteen `tree-check` doors, then arm 1
  (`./...`: internal/posse 325.0s, internal/treepins 341.1s, cmd/posse
  170.7s, cmd/testparallel 1.8s, plus the silent-revert audit — 2,125
  commits, 0 untriaged), arm 2 (260.9s) and arm 3 (279.4s). Exit 0.
- `go run ./cmd/checkorphans` — clean, after the suite ran backgrounded
  under a declared `POSSE_KEEP=ranger-base-6r1rf`.

What is NOT verified here, and cannot be from this seat: bob's live
discovery on the installed 2.0.7. That is ranger-base-bs49e.
