## The third re-measurement is the one that landed, because the same session proved the off-switch (ranger-base-0q45n)

ADR 0007 §6c reports a `skills_verified:` drift and never refuses, and the
remedy it names is a re-measurement. bob had taken that remedy twice and both
times the reading was stale within a day: ranger-base-6r1rf measured 2.0.5
onto a box already running 2.0.7, and ranger-base-bs49e measured 2.0.7 sixty
seconds before bob replaced itself with 2.0.9. This bead is the third run, on
the installed 2.0.9, and it is the first whose number is still true when it
lands — `posse runtime drift` now reports **every measured profile clean**,
which it has not done since §6 shipped.

That did not happen by running the probe faster. It happened because the same
session finally produced the evidence that `bobShell.autoUpdate: false`
(ranger-base-zneyv) is in force, which the two runs before it could not — so
this fragment is as much about §2 below as about the arms.

The arm-by-arm reading lives beside the first two, in
`docs/notes.d/ranger-base-6r1rf.md` ("Re-run on 2.0.9"), because a third probe
of the same question belongs in the same table. Everything below is this
bead's own.

## 1. The run, and what a code seat could check of it

MEASURED 2026-11-03 10:27Z by the operator (Dave), monica-verified: `bob chat
--workspace <probe> --trust` in the same three-arm scratch workspace, read off
the TUI's own `/skills` listing with **no turn sent** (hard risk line 1 — no
autonomous spending — is why the recipe stops at `/skills` and exits with
Esc). `bob --version` → `2.0.9` / `commit: 9690f20b0`. Session log
`~/.bob/logs/shell/bob-shell-20261103T052710.log` (local 05:27, Z 10:27). All
three arms **LISTED**, location `local`, including arm b — the symlink whose
target is outside the workspace, which IS the binding, because posse renders
this root as symlinks into `RHQ_HOME/skills` (§6d).

The probe needs a browser sign-in and a writable `~/.bob`, neither of which a
`cage: seatbelt` seat has — ranger-base-lfk7t §§1-2 measured both walls — so
the TUI reading is the operator's and can only be. What this seat re-read for
itself afterwards, and what each reading is worth:

| reading | this seat measured | worth |
|---|---|---|
| `bob --version` first line | bare `2.0.9`, second line `commit: 9690f20b0` | the declaration's exact value; `bob` prints the number alone where codex prints `codex-cli 0.153.4` |
| `@ibm/bob/dist/bob.js` mtime | `2026-10-31T09:39:50Z` — **unmoved** across the probe, three days later | §2: nothing replaced bob |
| `/opt/homebrew/bin/bob` | still → `../lib/node_modules/@ibm/bob/dist/bob.js` | the migration is spent, not re-armed |
| the `Skipping update check` line | present, 10:27:11.146Z | §2 |
| the probe fixture on disk | `probe-b-symlink` still an absolute symlink out to `outside-target/`; `probe-a-realdir` and `probe-c-control` real dirs with the frontmatter 6r1rf planted | the arms were the arms — the fixture was not quietly flattened between runs |
| the session log's module census | 10 × `SkillDirectoryWatcher`, 12 × `PluginManager`, 1 × `bob-shell:update-check`, **0 migration lines** | §2 and §3 |

The one thing a log cannot corroborate is the listing itself. bob logs the
watcher registrations and **no discovery or listing line at all** — the census
above has nothing between `SkillDirectoryWatcher` and the TUI — so "watched"
and "LISTED" are different facts with different evidence, exactly the
distinction ranger-base-lf6it §2 drew off a log diff and ranger-base-bs49e §3
drew off the bundle. It is why this root costs an operator session and not a
parse, and why no amount of static reading retires the probe.

## 2. The off-switch is PROVEN, and that is what makes this number durable

`bobShell.autoUpdate: false` has been in
`~/.bob/settings/settings.json` since 2026-10-31 09:37 and had never been
observed doing anything. ranger-base-bs49e established *why* no log line
answered for it: the `bobshell` → `@ibm/bob` package MIGRATION sits ahead of
the update check in bob's startup chain, and on the one start that mattered
the migration ran and the check was never reached — the 09:38:49 log carries
the migration chain and **neither** `Skipping update check` nor a registry
read.

That migration is spent. On this run:

```
10:27:11.146Z  bob-shell:update-check  Skipping update check.
                                       Disabled by user setting or policy
```

and the start one minute earlier
(`bob-shell-20261103T052624.log`, 10:26:25.413Z) carries the identical line —
so it is **two consecutive clean starts**, not one, which is more than the
bead asked for. Zero migration lines in either. The bundle's mtime has not
moved since the 09:39 replacement three days earlier.

ranger-base-zneyv's VERIFIED-WILL-MEAN is met. The honest scope of it:

- It gates the **update check**. It does not gate a migration — bs49e measured
  that and zneyv's own blast-radius note said so in advance. There is no
  second migration armed today (the legacy package is gone and its worker is
  absent from the 2.0.9 bundle, ranger-base-kf9ou §1), but "no migration is
  armed" is a reading of today's bundle, not a property of the key.
- It survives bob's own writes to the file that holds it. bob rewrote
  `settings.json` on this very start — `bobShell.lastRunVersion` moved
  `"2.0.7"` → `"2.0.9"` at 10:27:10Z — and left `autoUpdate: false` standing
  beside the `hooks`, `licenseConsent` and `approval` keys. That was not
  guaranteed and is worth knowing, because the key's whole value is that it
  outlives the thing it is switching off.
- `lastRunVersion` is also the record of **which session was this box's first
  2.0.9 start**, and it was the probe. ranger-base-kf9ou §1 read that key as
  `"2.0.7"` and concluded "no 2.0.9 session has ever started on this box";
  that was true when written and this run is what made it false, in the one
  way that produces a measurement rather than a surprise.

## 3. Two facts the earlier runs' logs could have carried and did not

Neither is reachable from a bundle read, and neither was asked for. Both are
MEASURED 2026-11-03 from the probe's own log.

**The ten discovery roots are confirmed LIVE.** Every reading of
`bobSkillsGlobalRoots` until now has been a static parse of minified JS — done
three times, by three beads, agreeing. This log registers a
`SkillDirectoryWatcher` on each root by **absolute path** at startup: the four
workspace roots (`.bob/skills`, `.bob/plugins/*/skills`, `.agents/skills`,
`.claude/skills`) and §6e's six global ones (`~/.bob`, `~/.agents`,
`~/.claude` × `{skills, plugins/*/skills}`). So the running 2.0.9 walks
exactly what the declaration names, and the list is no longer derived from a
parse alone. Note the shape the parse already predicted and the log confirms:
`.agents` and `.claude` get **no** plugins glob at workspace scope, where all
three do at home scope.

**`.bob/plugins/*/skills` is a skill root whether or not the dir is a loadable
plugin.** The probe workspace's `.bob/plugins/posse/` holds no `plugin.json`,
and bob says so four times in this one start:

```
PluginManager  No plugin.json found in <ws>/.bob/plugins/posse, skipping
```

The skill under it **listed anyway**. That matters because it is precisely the
shape `renderSkillsCwd` writes: `<root>/<name>` symlinks and nothing else — no
manifest, unlike the claude flag tree, which gets a
`.claude-plugin/plugin.json`. So posse's bob render is not an analogue of the
probe fixture, it IS the probe fixture, and the one structural difference
between the two surfaces is measured inert on the releases probed. The same
line appears in the 2.0.7 log (4×) and not in the 2.0.5 one, so the reading
covers two of the three releases; nobody had looked.

## 4. What this does NOT establish

- **Anything about the next release.** `skills_verified:` keys on the version,
  and the field will read as drift the next time bob moves — by an update the
  key now demonstrably blocks, or by a migration it never covered. §6c reports
  and never refuses, so the cost of that day is one red
  `verify-skills-drift` row and a fourth probe, not a blocked launch.
- **Anything about another box.** The work box's bob is still unmeasured
  (6r1rf said so first). The off-switch proof is this box's `~/.bob`.
- **Anything about dispatch.** bob is still not launchable — herdr 0.8.2 has
  no `bob` kind, so `posse runtime check bob` exits 1 by name — so no
  `skills:` PID on bob reaches a session either way. The blast radius of this
  field, green or red, is documentation and a verify row.

## Changes

- `internal/posse/runtime.go`: bob's built-in `SkillsVerified` moves `"2.0.7"`
  → `"2.0.9"`. Its comment carries the third probe, the off-switch proof and
  its scope, `lastRunVersion`'s move, and — stated plainly because it is new —
  that the field and the installed binary agree for the first time, as a
  measurement catching up and not a rule change. `SkillsGlobalRoots`' paragraph
  gains §3's live confirmation.
- `internal/posse/skills.go`: `BobSkillsRoot`'s comment names all three live
  probes, sharpens why the listing needs a TUI (bob logs watchers, not
  listings) and carries the no-`plugin.json` reading;
  `bobSkillsGlobalRoots`' comment carries the live watcher confirmation beside
  its static reads.
- `internal/posse/bobruntime_qa_test.go`: the bob-grid pin's version anchor
  moves to `(verified 2.0.9)`, and its comment says why the anchor must not be
  re-derived from `bob --version` if the two part again.
- `internal/posse/skillsrules_parity_qa_test.go`: the `qmSurfaces` bob row's
  as-of comment tracks the probe it names.
- `docs/adr/0007-skills-binding.md`: a new amendment line; both tables' bob
  rows; §6a's built-in list; §6c's bracketed-field example; the Evidence
  RESOLVED entry gains the third run; a new MEASURED entry for this reading,
  the off-switch and §3's two facts.
- `docs/notes.d/ranger-base-6r1rf.md`: the three-version arm table, and its
  bs49e "filed, not fixed" item marked run.
- `docs/notes.d/ranger-base-bs49e.md`: its `0q45n` "filed, not fixed" item
  marked run, saying which of its own conclusions this supersedes (its title's
  "did not clear the clock", and §2's "still unproven").
- `docs/notes.d/ranger-base-kf9ou.md`: three `[0q45n]` bracketed updates in
  that file's own convention — the head note, §1's `lastRunVersion` reading,
  the wall table's live-discovery row and its filed-not-fixed entry. Its dated
  measurements stand as written.

## Filed, not fixed

Nothing. The remaining bob work is unchanged by this and already filed:
**ranger-base-f5jbs** (the exit-code surface for `skillsVerifiedDrift`) and
the herdr detection gap, which is what keeps bob unlaunchable.

## Verified

MEASURED 2026-11-03, this box (darwin 25.6.0), from the worktree
`posse/dinesh-posse-ranger-base-0q45n`:

- `go run ./cmd/posse runtime drift` → **exit 0**, `3 of 4 launch profile(s)
  measured, every one on the version its skill discovery was measured on`,
  with `✓ bob  skills_verified: "2.0.9" and bob reports the same`. This is the
  observable point of the change and the row `scripts/verify-box.sh` schedules.
- `go build ./...`, `make fmt-check`, `make tree-check`.
- The focused pins over what changed, each confirmed to have RUN under `-v`
  rather than passed as a filter matching nothing.
- `go run ./cmd/checkorphans` — clean.

The commit message and the bead comment carry the command list and the
timings.
