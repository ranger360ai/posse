# The persona memory dir is granted in the settings payload, not as `--add-dir` (ranger-base-hgcmw)

*2026-11-20 · dinesh · operator bead ranger-base-3a9i9*

## The hole it closes

Claude Code 2.1.328 started gating `--add-dir` behind a trust dialog when the
added directory is inside a git repository nobody has trusted. Posse's claude
template named the persona's memory dir there (`--add-dir {memory}`), and on an
instance whose personas live inside another repo — the one this was measured on
keeps `$POSSE_HOME/personas` as a symlink into `$CONSTITUTION`, because
that is the repo the memory landing commits ORDERS.md to — every seat stopped
at:

```
Adding a new directory: …/posse/personas/<persona>
This directory is part of the repository at …/ranger-base. Trusting it trusts
that whole repository, including its other worktrees and subdirectories.
```

The answer holds for one session, so every launch asked again, and a launch
whose dialog nobody answers misses its task prompt and sits idle: nine seats in
two days (ranger-base-3a9i9). The standing workaround was an operator watcher
answering "Yes, add it" within seconds for exactly that path.

## What was measured

MEASURED 2026-11-20, claude **2.1.330**, linux, this box. Both probes are in
the bead's session scratchpad; the arms are reproducible from the scripts
quoted here.

**Does the dialog fire?** Three interactive arms in a pty, each killed after
30s without answering anything, captures ANSI-stripped and grepped for the
dialog's own words. The target in arms 1 and 2 is one directory inside a
freshly `git init`-ed repo that is neither the cwd's repo nor a trusted one;
arm 3's target is inside the cwd's own (trusted) repo.

| arm | how the dir is named | trust dialog |
|---|---|---|
| 1 | `--add-dir <dir in an untrusted repo>` | **YES** — "Adding a new directory … part of the repository at …" |
| 2 | `--settings '{"permissions":{"additionalDirectories":["<same dir>"]}}'` | no |
| 3 | `--add-dir <dir in the cwd's own trusted repo>` | no |

Arm 3 is why this is not "the flag is bad": what the dialog is about is the
TARGET's trust. `/home/posse/src/posse` is a trusted project in this box's
claude config and `/home/posse/src/ranger-base` is not, which is the whole of
why posse's own worktrees never asked and the persona dir always did.

**Does the payload grant the same reach?** Three print-mode arms with
`permissions.blockWritesOutsideWorkingDirectories` on — with that key set, an
edit outside a WORKING DIRECTORY is refused whatever the permission mode, so
the file on disk is the oracle. Each arm asks for one Edit inside the granted
dir:

| arm | how the dir is named | Edit taken? |
|---|---|---|
| 1 | `--settings … additionalDirectories` | **yes** (file rewritten) |
| 2 | `--add-dir` | **yes** (file rewritten) |
| 3 | neither | no (model answered BLOCKED, file untouched) |

So the two spellings grant the same thing and the difference between them is
only the dialog. (An earlier run of the same three arms without the block key
had all three succeed — `acceptEdits` alone is not a discriminator, and a probe
whose control passes proves nothing.)

**And on the real path, not only a synthetic venue.** The arms above use a
freshly `git init`-ed directory with no symlink in it, and claude takes a
working directory "exactly as it is passed" and follows no link at it, so this
box's actual memory dir — `/home/posse/homes/ranger/personas/dinesh`, whose
`personas` component is the symlink into `$CONSTITUTION` — is its own
measurement. Same date, same version:

- `--add-dir <that path>`: the dialog, verbatim what ranger-base-3a9i9
  reported, naming the RESOLVED path
  (`/home/posse/src/ranger-base/posse/personas/dinesh`) and offering to trust
  `/home/posse/src/ranger-base`. It displays the resolved path because `uGl`
  looks the trust up for `lHo(t) ?? t` while the flag named the link.
- the same path in `permissions.additionalDirectories` of `--settings`: no
  dialog.
- the write reach on that same path, with the block key on: **DONE**, file
  rewritten, through the settings grant; **BLOCKED**, file untouched, with no
  grant. So the grant reaches through the symlinked parent exactly as the flag
  did. (The probe file was a `*.out`, which the memory dir's own ignore seed
  excludes, and it was removed.)

**Why, from the binary.** `uGl` ("untrustedAddedFolder") returns a dialog
descriptor only when the path's trust verdict is `untrusted`, no persisted
trust covers it, and it is not under the session cwd in the same repo; the
startup step that renders it is built from `foldersToTrustForFlag`, over the
`--add-dir` values alone. The CLI's own non-interactive path says the same
thing in words: *"Added folder not trusted: X. Leave out its `--add-dir` or …
start `claude` there once and accept the trust prompt."* A settings-sourced
`additionalDirectories` from `--settings` (claude's `flagSettings` scope) is
merged into the session's working directories with no such gate; only the
`projectSettings` and `localSettings` scopes have one.

## The fix

`permissions.additionalDirectories` in the `{settings}` payload, and no
`--add-dir` on the claude line.

- `ClaudeFleetSettingsJSON(allow []string, memoryDir ...string)` merges the
  dir into the payload's `permissions` object (`applyMemoryGrant`), keeping
  whatever that key already holds. `claudeGrantableDirs` drops empty and
  relative entries — claude drops relative ones itself — and passes the path
  through byte-for-byte, links and all, because claude takes a working
  directory "exactly as it is passed" and follows no link at it. That is the
  same unresolved string `--add-dir` was handed, so the reach does not move.
- A grant with **no** credential pin is a return now rather than a
  fallthrough: a box with no home directory names no credential dirs and
  still names a memory dir, and ORDERS.md is the write that must not start
  asking. The auto-mode carve-out stays on the pinned side of that branch,
  so the documented order of loss is unchanged.
- Both claude templates — `DefaultAgentCommand` and the built-in `claude`
  runtime's `Command` — lost `--add-dir {memory}`. Variadic, trailing, so
  every caller with no dir to name (the pin readers, the probes) renders
  what it rendered before.

Codex is untouched: `realizeCodex` emits its own `--add-dir` as the sandbox's
writable grant, in the one mode where that flag is legal, and codex has no
trust dialog.

## Residual

A PID that spells its OWN `command:` with `--add-dir {memory}` still gets the
dialog. Nothing strips it, deliberately: `Unattended`'s rule for this file is
that an explicit spelling in a hand-written PID beats an implicit one from
posse, so the guarantees here APPEND and never remove. No PID on this instance
and none under `examples/agents/` carries a `command:` at all. Such a line now
carries the grant twice over — once in the payload `{settings}` renders, once
in its own flag — so deleting the flag from it is the whole fix for it.

## Found on the way

`arm3`'s launch-line tests go red on the git-global snapshot rather than on
the line: each compares a rendered line against a payload computed by a
SECOND call, so `settingsPin()` is read twice and `GIT_CONFIG_GLOBAL` can
change between the reads. MEASURED 2/5 then 1/5 parallel runs red against
0/5 serial, and it fired once inside a full `make test` arm 3. It cost two
full suite runs here before the differing token was spotted, because the
output names the launch line and reads as a launch-line defect. Filed as
ranger-base-nycra, not fixed here. This bead's own new pins are immune by
shape — they parse the payload off the rendered line (`claudePayloadOf`)
instead of recomputing it.

## Not closed here

- ranger-base-3a9i9 — the operator's, and it closes when a fresh seat on
  claude latest reaches its task with no dialog and the watcher logs nothing
  for that launch.
