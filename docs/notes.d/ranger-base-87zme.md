# ranger-base-87zme — the ADR 0079 compare resolves `~` per read

2026-11-06, dinesh. Fix for ranger-base-7czd6 (holden's finding under ranger-base-h0hjx, §3 of
`ranger-base-h0hjx.md`): `core.hooksPath = ~/managed-hooks` in the launcher's
global file read byte-identical under a moved HOME, so the global compare
allowed the commit and git ran `<moved HOME>/managed-hooks/pre-commit`.

## What changed (internal/posse/hookskip.go, `renderHookSkipGlobalScope`)

- Every value read on both sides takes `--type=path`. git expands `~` per
  read, from that read's HOME, so the sides differ when the directory does.
  Values without a `~` read unchanged, so aliases and absolute paths compare
  exactly as before.
- A value read whose git exits past 1 (1 means "no match") refuses. That
  closes a fail-open that `--type=path` would otherwise have added.
- SYSTEM lines are compared too, under the same HOME rule. Cost when the
  compare runs: 3 + candidates execs, up from 2 + candidates.
- The `--name-only` count read is unchanged: it prints keys, not values.

## Measured (2026-11-06, darwin 25.6.0, git 2.50.1 Apple Git-155)

- `git config --type=path --get-regexp` with a `~nosuchuser/x` value: rc 128,
  `fatal: failed to expand user dir`, and NOTHING printed, even for entries
  listed before the bad one. `--name-only` with the same value still lists
  every key at rc 0.
- An alias value spelled `~nosuchuser...` in the launcher's file is enough.
  Without the rc check, a HOME moved to a config with no hook keys reads
  have = "" and want = "" (the failed read printed nothing), so the commit is
  allowed with the managed hooksPath gone. Pinned:
  `TestQAHookSkipUnexpandableTildeFailsClosed`.
- `git config --system` ignores `GIT_CONFIG_NOSYSTEM`, and it reads no
  includes unless given `--includes`. The all-scope listing does follow
  includes.
- `git config --file <missing>` returns rc 1, so a missing candidate still
  contributes nothing.

## System scope (the bead's item 4)

The system arm (`posse_hs_scope`) only runs when GIT_CONFIG_SYSTEM or
GIT_CONFIG_NOSYSTEM is set, and it reads both its sides under the call's HOME.
So a system file carrying `core.hooksPath = ~/x` (or `include.path = ~/x`),
plus a moved HOME with a byte-identical global file, reached no compare at
all. The global compare kept only `global` lines. It now keeps `system` lines
too. Its want side reads `--system` under the launcher's HOME with the call's
GIT_CONFIG_SYSTEM/NOSYSTEM, so the two sides read the same file and differ
only by HOME. The system arm itself is unchanged: its two reads share one
HOME, so `--type=path` would add nothing there.

Pinned by `TestQAHookSkipSystemTildeHooksPathJudgedAgainstLauncher`, through
a shim rendered over a git wrapper that points GIT_CONFIG_SYSTEM at a scratch
file on every exec. That simulates a box whose /etc/gitconfig carries the
line. holden's fixture typed GIT_CONFIG_SYSTEM on the call, which tripped the
system arm. This one sets nothing on the call. The pin also checks that an
absolute system hooksPath under the same moved HOME is still allowed.

## Mutation checks (each run against the fixed tree with one guard removed)

| removed | reds |
|---|---|
| the whole fix (HEAD's hookskip.go) | tilde pin, system pin, unexpandable pin, exec count |
| want candidate rc check | `TestQAHookSkipUnexpandableTildeFailsClosed` |
| `system` lines in have | `TestQAHookSkipSystemTildeHooksPathJudgedAgainstLauncher` |
| `--type=path` on have only | tilde pin's `g-xdg` leg (the bead's item 1) |

## Over-refusal, accepted

A moved HOME is now refused whenever a hook-relevant value in global or
system scope carries a `~` that expands differently. That includes
`include.path = ~/x` with identical included content, which used to be
allowed. It is also refused when such a value cannot be expanded at all.
Only a call that moved HOME, XDG_CONFIG_HOME or GIT_CONFIG_GLOBAL pays this.
The ordinary call never runs the compare (D3).

## Not deployed by this bead

The bead says a wall fix is not deployed until `posse gates install-hooks`
runs. For this arm that is not the step. `install-hooks` writes the repo's
pre-push and prepare-commit-msg hooks (cmd/posse/main.go). The hook-skip arm
lives in the L1 git shim, and `RenderGates` renders that shim fresh at every
seat launch, from the `posse` binary doing the launch. So the fix reaches a
seat when the operator installs a `posse` built from a main that has this
commit, and that seat is (re)launched. Seats already running keep their old
shim until they are relaunched. Installing the binary is a step on a
deployed system, so it is the operator's, and it is not run from here.
