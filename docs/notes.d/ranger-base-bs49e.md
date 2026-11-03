## The re-measurement did not clear the clock, and that is the honest outcome (ranger-base-bs49e)

ADR 0007 §6c reports a `skills_verified:` drift and never refuses, and the
remedy it names is a re-measurement. ranger-base-6r1rf shipped bob's first
verified cwd root reading `2.0.5` onto a box whose installed bob was already
`2.0.7`, so this bead is that remedy: the operator re-ran the same three
discovery arms on the installed 2.0.7, all three LISTED, and
`SkillsVerified` moved `2.0.5` → `"2.0.7"`.

It is still in drift. Sixty seconds after that probe exited, bob replaced
itself again — `2.0.9`, and under a different npm package name. This fragment
is why the field does not follow, and what the next probe has to establish
that this one did not.

The arm-by-arm reading lives beside the first one, in
`docs/notes.d/ranger-base-6r1rf.md` ("Re-run on 2.0.7"), because a second
probe of the same question belongs in the same table. Everything below is the
part that is this bead's own.

## 1. What this box did, to the minute

Every reading is MEASURED 2026-10-31 on this box (darwin 25.6.0), from bob's
own logs under `~/.bob/logs/shell`, from `ls -la`, and from a static read of
the shipped bundle. `bob` itself was run exactly once, as `bob --version`,
which ranger-base-lf6it §6 measured to be a read: no session log, no settings
write, no update chain (that hangs off the TUI's init, not off the flag).

```
09:37        ~/.bob/settings/settings.json written: bobShell.autoUpdate = false
09:38:15     a bob chat start — migrateLegacyBobshell FAILS, blocked by a
             running instance (LegacyMigrationBlockedError ... 23267)
09:38:49     THE PROBE. bob 2.0.7 / commit e1a2a9f87. /skills listing read,
             no turn sent. The legacy-bridge migration runs this time:
             backup at ~/.bob/settings/bobshell-migration/2.0.7/bridge-backup,
             worker copied out of bobshell/dist
09:39:45     last line of that session
09:39        /opt/homebrew/lib/node_modules/@ibm/bob/ created;
             /opt/homebrew/bin/bob repointed to ../lib/node_modules/@ibm/bob/dist/bob.js;
             /opt/homebrew/lib/node_modules/bobshell/ GONE
later        bob --version -> 2.0.9 / commit: 9690f20b0
             npm ls -g: @ibm/bob@2.0.9; package.json name @ibm/bob, version 2.0.9
```

So the probe measured 2.0.7 and the box now runs 2.0.9.

**The 2.0.7 reading is not spoiled by that, for the reason ranger-base-lf6it
§1 established for the 2.0.5 one:** node reads `dist/bob.js` once at startup
and runs it from memory, so replacing the file underneath a live process does
not reload it. The process that drew the `/skills` listing was 2.0.7's code,
and the operator's own `bob --version` at the head of the session says 2.0.7.
This is now the **second consecutive** probe of this question overtaken by
bob's own replacement, and the second one whose measurement survives it.

## 2. `autoUpdate: false` was already in force. A MIGRATION moved the version anyway.

This is the part that would mislead a reader who knows only that
ranger-base-zneyv closed.

`~/.bob/settings/settings.json` has carried
`"bobShell": {"lastRunVersion": "2.0.7", "autoUpdate": false}` since its
**09:37** mtime — a minute *before* the probe. The version still moved.

It moved by the `bobshell` → `@ibm/bob` package **migration**, not by the
update check. ranger-base-lf6it §5 predicted exactly this ("A package rename
is armed, and it fires on the next quiet `bob chat`"), and
ranger-base-zneyv's own blast-radius paragraph excluded it from the key's
reach in as many words: *"it does not stop the pending `bobshell` → `@ibm/bob`
package rename, which is a migration and not an update."* Both were right.
What is new is only that it fired, and what it cost: a version move the one
control on this box does not gate.

**The off-switch is therefore still unproven, which is what the operator's own
comment on this bead said** ("the log carries no explicit 'Skipping update
check' line, so the off-switch is NOT yet proven — note it, do not claim it").
What this bead adds is *why no line answered it*: the 09:38:49 log carries the
migration chain and **no update-check line of either kind** — neither
`Skipping update check` nor a registry read. The check sits *after* the
migration in bob's startup chain (lf6it §3), and this is the start on which
the migration finally ran.

Two things follow, and only two:

- The next `bob chat` is the first clean test of the key, because the
  migration cannot fire twice — the legacy package is gone.
- Until that test, nothing on this box refuses to move bob. `verify-skills-drift`
  reporting the drift is the whole of the control, and reporting is what §6c
  chose.

## 3. The roots are unchanged on 2.0.9 — statically, which settles one arm and not the others

MEASURED 2026-10-31 by a static read of
`/opt/homebrew/lib/node_modules/@ibm/bob/dist/bob.js` (15,640,134 bytes,
`@ibm/bob` 2.0.9), the same kind of read ADR 0007's Evidence used on 2.0.5 and
with the same caveat — the minified names move between builds, the string
literals do not:

```
NA=".bob"   tJ="plugins"

xds=[posix.join(NA,"skills"), posix.join(NA,tJ,"*","skills"),
     posix.join(".agents","skills"), posix.join(".claude","skills")]

Sds=Cmr.flatMap(e=>[join(e,"skills"), join(e,tJ,"*/skills")])
  where Cmr=[join(homedir(),".bob"), join(homedir(),".agents"),
             join(homedir(),".claude")]
```

Recipe, one match for both sets: `grep -o -E 'xds=\[.{200}' <that file>`.
The ADR's 2.0.5 spellings `rXo=` and `nXo=Afr.flatMap` are `xds=` and
`Sds=Cmr.flatMap` here, exactly the drift the ADR's recipe warns about.

Read out: workspace roots `.bob/skills`, `.bob/plugins/*/skills`,
`.agents/skills`, `.claude/skills`; global roots `~/.bob`, `~/.agents`,
`~/.claude` × `{skills, plugins/*/skills}`. **Identical to the 2.0.5 read.**
So `BobSkillsRoot` is still a workspace discovery root on 2.0.9, and all six
`bobSkillsGlobalRoots` entries still hold.

**What a static read cannot answer, and why the next probe is still needed.**
A root set is not a behaviour. Whether a `<name>` entry *under* that root
which is a symlink to a dir outside the workspace gets followed is a
readdir/stat question, and that arm IS the binding — posse renders this root
as symlinks into `RHQ_HOME/skills` (§6d). Nor does a root set say a
discovered skill is actually **LISTED** by `/skills` rather than merely
watched. ranger-base-lf6it §2 drew the same line off bob's own log diff; this
is the same line drawn off the bundle.

## 4. Why the field is 2.0.7 and not 2.0.9

Because §6's subject is the release whose **live discovery was measured**, and
nobody has measured 2.0.9's. Declaring 2.0.9 would put a version in the grid
that no probe produced — the §6b failure with the louder label filed off, and
the opposite of the rule ranger-base-6r1rf set when it held the field at 2.0.5
against an installed 2.0.7.

The cost is visible and bounded: `posse runtime check bob` and a launch's
Realized detail both carry

```
drift: skills_verified: "2.0.7" but bob reports "2.0.9" — re-measure live
discovery and update skills_verified: (ADR 0007 §6c)
```

and `posse runtime drift` — `scripts/verify-box.sh`'s `verify-skills-drift`
row — keeps exiting nonzero. It was already exiting nonzero for the 2.0.5 →
2.0.7 drift this bead was filed to clear, so nothing regressed; what changed
is that the remaining question is now one bead wide instead of three arms
wide. And the blast radius of a red row here is documentation, not dispatch:
bob is still not launchable (no herdr 0.8.2 `bob` kind, `posse runtime check
bob` exits 1 by name), so no `skills:` PID on bob can reach a session to be
degraded.

## 5. The package rename, for anyone re-running a static read

`/opt/homebrew/lib/node_modules/bobshell/dist/bob.js` **does not exist on this
box any more.** Eleven tracked files still name it; every one of them is a
dated measurement and stays as written, because rewriting a 2026-10-12 reading
to a path that did not exist then would be a lie. Two places were changed
instead, the two a reader follows to *re-run* something:

- ADR 0007's Evidence entry, which carries the grep recipe, now names the
  current path and the current minified spellings beside the originals.
- `bobSkillsGlobalRoots`' comment in `internal/posse/skills.go`, which is the
  declaration that static read produced.

No code anywhere reads that path — `grep -rn 'lib/node_modules'` over `*.go`,
`*.sh` and `*.py` finds two comments and nothing executable — so the rename
broke no check. `bob --version` and `command -v bob` are the readings that
survive the next one (ranger-base-lf6it §5).

## Changes

- `internal/posse/runtime.go`: bob's built-in `SkillsVerified` moves
  `"2.0.5"` → `"2.0.7"`, and the comment above it carries the re-measurement,
  why the value is not the installed one, the 09:39 replacement and the
  migration that caused it. `SkillsGlobalRoots`' paragraph gains the 2.0.9
  re-read.
- `internal/posse/skills.go`: `BobSkillsRoot`'s comment names both live
  probes; `bobSkillsGlobalRoots`' comment names the current bundle path, the
  2.0.9 re-read and the moved minified names.
- `internal/posse/bobruntime_qa_test.go`: the bob-grid pin's version anchor
  moves to `(verified 2.0.7)`, and its comment says the field holds at what a
  live probe produced, so drift against an installed 2.0.9 is on purpose.
- `internal/posse/parity.go`: the worked example of a bare `--version` line
  tracks the declaration it illustrates.
- `docs/adr/0007-skills-binding.md`: both tables' bob rows, §6a's built-in
  list, §6c's worked example, the status line, the Evidence static-read entry
  (+ the 2.0.9 re-read, the new path, the new minified names), the RESOLVED
  entry (+ the 2.0.7 re-run) and a new MEASURED entry for the 2.0.9 drift and
  why a re-measurement did not clear it.
- `docs/notes.d/ranger-base-6r1rf.md`: the 2.0.7 arm table beside the 2.0.5
  one, and its "filed, not fixed" item marked run.

## Filed, not fixed

- ~~**ranger-base-0q45n**~~ **RUN 2026-11-03** (operator, `-l question`,
  `-a dave` — the probe needs a browser sign-in and a writable `~/.bob`, which
  a `cage: seatbelt` seat has neither of) — re-run arm b and the `/skills`
  listing on 2.0.9, and read the off-switch for free on the same start. §3
  above narrows it from three arms to one plus the listing; §2 says what to
  grep the log for.
  **Both came back.** All three arms LISTED on the installed 2.0.9 and the
  grep §2 specified hit: `Skipping update check. Disabled by user setting or
  policy`. So the clock this fragment's title says was not cleared IS cleared
  — `SkillsVerified` is `"2.0.9"`, matching the installed CLI, and §2's "the
  off-switch is therefore still unproven" is superseded. `docs/notes.d/ranger-base-0q45n.md`.

## Verified

MEASURED 2026-10-31, this box:

- `go build ./...`, `make fmt-check`, `make tree-check`, `python3
  scripts/notes-index.py --check`.
- The focused pins over what changed, each confirmed to have RUN under `-v`
  rather than passed as a filter matching nothing.
- `go run ./cmd/checkorphans` — clean.

The commit message and the bead comment carry the full command list and the
timings.

What is NOT verified here, and cannot be from this seat: bob's live discovery
on the installed 2.0.9. That is ranger-base-0q45n.
