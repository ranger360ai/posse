# ADR 0079 — the hook-skip arm judges git's global scope against the launcher's resolved global files, for HOME, XDG_CONFIG_HOME and GIT_CONFIG_GLOBAL alike

*Status: accepted 2026-11-04 (ranger-base-7yx3s, from ranger-base-z8kym item 3) · owner: architect · replaces the GIT_CONFIG_GLOBAL half of `posse_hs_scope` (hookskip.go, ranger-base-r3pri); the SYSTEM half stands · builds in the code bead named on ranger-base-7yx3s (dinesh)*

## Context

z8kym item 3 asks that a moved `HOME` / `XDG_CONFIG_HOME` be "judged by
content, the same way the GIT_CONFIG_GLOBAL arm already does". That arm
compares the hook keys git sees with the variable set against the same call
with it unset. `HOME` has no unset baseline, so the arm needs a REFERENCE:
which global-scope files count as "not moved". Three facts decide it.

1. **ADR 0077 D1 has already answered this for the host tier.** "`HOME` is
   not pinned: ASSUMED equal, since both the launcher and the server run as
   the same user." And obdnk (gitglobalscope.go) already resolves claude's
   `GIT_CONFIG_GLOBAL` pin from the launcher's HOME through
   `gitGlobalScopeCandidates`. The shop has one reference for global scope.
   The shim just doesn't use it.
2. **The shipped GIT_CONFIG_GLOBAL arm is unsound once HOME moves too.** Its
   baseline is computed under the call's own (moved) HOME. MEASURED
   2026-11-04, git 2.50.1 (Apple Git-155), scratch HOME holding
   `core.hooksPath=/managed`:
   - `HOME=evil GIT_CONFIG_GLOBAL=evil/.gitconfig git commit`: the arm says
     ALLOWED, and git sees no hooksPath. Same for `HOME=empty
     GIT_CONFIG_GLOBAL=/dev/null`. That is a bypass at rc 0.
   - `HOME=empty GIT_CONFIG_GLOBAL=<the pinned file>`, which is a claude seat
     plus an unrelated HOME move: the arm says REFUSED, though git sees
     `/managed` and runs the hook. That is an over-refusal.
3. **`git config --global <read>` is not what a commit reads.** MEASURED,
   same git: with both `~/.config/git/config` and `~/.gitconfig` present,
   `--global --get-regexp` and `--global --list` list only `~/.gitconfig`.
   The all-scope `--show-scope` listing shows both files as `global`, and a
   commit reads both. So the shipped compare can't see an XDG-file hook at
   all. githookparity.go already reads the right way (`--show-scope`, all
   scopes).

## Decision

**D1 — The reference is the launcher's global-scope candidate list, resolved
at render by `gitGlobalScopeCandidates` and baked into the shim.** This is
the same function and environment the obdnk pin uses, so the claude pin and
the shim's reference can't drift apart. The render records:
- the candidate files, in git's order;
- the launcher's `HOME` and `XDG_CONFIG_HOME`, each with whether it was set
  (unset ≠ empty);
- obdnk's pin value, when there is one.

The environment it resolves from is injectable on `App`, with nil meaning
`os.Environ()`. QA tests are `t.Parallel()` and can't `t.Setenv`, and this is
the fix for "every commit would be judged moved" in hookskip_qa_test. The
fixture passes `hsEnv(fx)`, so its scratch HOME is the launcher's.

**D2 — One global compare covers all three names.** It replaces the
GIT_CONFIG_GLOBAL arm on a hook-running verb.
- *have* is the call's own `git config --show-scope --get-regexp
  <hookSkipRegexp>`, keeping only `global` lines. Filter in pure sh
  (`case "$l" in global"<TAB>"*`), not with a second binary.
- *want* is `git config --includes --file <c> --get-regexp <hookSkipRegexp>`
  for each baked candidate in order, concatenated. A missing file
  contributes nothing.
- Equal → allowed; different → `hookskip:config-env`.

MEASURED equal, in a repo and outside one, over two files plus an
`[include]` and an `[includeIf "gitdir:…"]`. Use `-z` on both sides if a
value can carry a newline.

**D3 — Gate the compare on a string test, so the ordinary call runs no
extra git.** The compare runs only when one of these holds:
- `GIT_CONFIG_GLOBAL` is set and differs from the baked pin value (or there
  is no pin);
- `GIT_CONFIG_GLOBAL` is unset and `HOME` or `XDG_CONFIG_HOME` differs from
  the baked value in value or in set/unset.

A pinned `GIT_CONFIG_GLOBAL` with HOME moved skips the compare, because git
ignores HOME there. Cost when it fires: 1 + (number of candidates) execs,
~40 ms MEASURED for two files (10 pairs in 0.78 s, this box).

**D4 — Fail direction.** No real git at render fails closed, as today. No
candidates (a launcher with no HOME) bakes an empty reference: a seat whose
global scope carries hook keys is then refused, and a seat whose global
scope has none is allowed, which is right because that launcher's git reads
no global hook either.

## Consequences

- The z8kym HOME leg, an XDG move, `env -u HOME`, and both measured
  GIT_CONFIG_GLOBAL bypasses are refused. A HOME pointed at an identical
  config, or at one with no hook keys on a box with none (every hermetic test
  in this repo), is allowed.
- On obdnk's two-file DEGRADED box, a typed or settings-injected HOME or XDG
  move on a hook verb is now caught by the shim. Settings `env` inlets
  outside git are still obdnk's to name.
- **Container tier:** the cage HOME differs from the baked one, so the
  compare fires on every hook verb there. The baked host paths are unmounted
  and the cage HOME is seeded with no git config, so both sides are empty
  and the commit is allowed. ASSUMED that the cage carries this shim at all;
  either way nothing is over-refused.
- Two launches of one persona from launchers with different HOMEs would race
  on the shared shim file. This is ASSUMED away by the same ADR 0077 D1
  assumption, and named here so it isn't rediscovered.
- Still cooperative (ADR 0025): `/usr/bin/git`, `env -i` and a hand-edited
  reference file are past it by class.
- **Exit hatch:** the arm is render-time data plus ~15 lines of sh in
  `posse_hs_env`, and it holds no state. Reverting means restoring the
  unset-baseline arm.

## Alternatives rejected

- **The passwd home (`~$USER`).** It's a second notion of home that nothing
  else in posse uses. It equals the launcher's HOME on the host anyway,
  differs from every test fixture, and can't be injected per test.
- **Typing `POSSE_HS_HOME="$HOME"` on the seat line.** A reference the seat
  can reassign is no reference: `POSSE_HS_HOME=evil HOME=evil git commit`
  costs the same one word. It would also edit the ADR 0077 line and
  muxrex.go's parse for nothing.
- **Refusing any HOME/XDG change on a hook verb.** Over-refuses every test in
  this repo that sets HOME, which is exactly what z8kym item 3 said not to
  do.
- **Keeping the unset-baseline arm and adding a HOME arm beside it.** The
  baseline is the measured bypass (Context 2) and `--global` the measured
  blindness (Context 3). A second arm would inherit both.
- **The one I wanted: bake a content snapshot of the launcher's hook keys
  instead of file paths.** That's zero execs on the reference side. But it
  goes stale on the first legitimate `git config --global` edit after
  launch, refusing every commit until relaunch, and it copies the
  operator's config into a shim file. Paths re-read live track the
  operator; a snapshot doesn't.
- **Reading the parent shell's environment.** Rejected in ADR 0077 for the
  same reasons (setuid `ps`, unmeasured sysctl from a cage), and
  `export HOME=x` defeats it anyway.

## Pins owed (execution, through a rendered shim, claude and codex)

Each pin protects the hook-skip refusal on a hook verb. Delete one and a
seat skips the managed hook at rc 0.

1. `HOME=<dir lacking the managed hooksPath> git commit` → refused, hook not
   fired.
2. Managed config in the XDG file; `XDG_CONFIG_HOME=<other> git commit` →
   refused.
3. `env -u HOME git commit` → refused.
4. `HOME=evil GIT_CONFIG_GLOBAL=evil/.gitconfig` and `HOME=empty
   GIT_CONFIG_GLOBAL=/dev/null` → refused (Context 2).
5. Two-file fixture, `GIT_CONFIG_GLOBAL=<~/.gitconfig>` hiding the XDG file's
   hook → refused (Context 3).
6. HOME pointed at an identical config → allowed and fired. Pinned
   GIT_CONFIG_GLOBAL plus `HOME=<empty>` → allowed and fired.
7. The plain commit with HOME, XDG and GIT_CONFIG_GLOBAL as rendered execs
   no `git config` (D3), read through the leak canary the env arm's call
   site already uses.
