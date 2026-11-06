# ADR 0079 — the hook-skip arm judges git's global scope against the launcher's resolved global files, for HOME, XDG_CONFIG_HOME and GIT_CONFIG_GLOBAL alike

*Status: accepted 2026-11-04 (ranger-base-7yx3s, from ranger-base-z8kym item 3) · owner: architect · replaces the GIT_CONFIG_GLOBAL half of `posse_hs_scope` (hookskip.go, ranger-base-r3pri); the SYSTEM half stands · builds in the code bead named on ranger-base-7yx3s (dinesh) · amended 2026-11-04 (ranger-base-o6xr6): D2 and D3 now say what ranger-base-32ll0 shipped · amended 2026-11-06 (ranger-base-87zme, from ranger-base-7czd6): D2 resolves HOME-relative values instead of comparing them raw, and it compares system lines too*

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
- *have* is the call's own `git config --show-scope --type=path
  --get-regexp <hookSkipRegexp>`, keeping the `system` and `global` lines.
  Filter in pure sh (`case "$l" in system"<TAB>"*|global"<TAB>"*`), not
  with a second binary.
- *want* is `git config --system --includes --show-scope --type=path
  --get-regexp <hookSkipRegexp>`, then `git config --includes --file <c>
  --type=path --get-regexp <hookSkipRegexp>` for each baked candidate in
  order, each of those lines prefixed `global<TAB>`. A missing file
  contributes nothing. **The want reads run under the launcher's baked
  `HOME` and `XDG_CONFIG_HOME`** (set, or unset, as baked), not the call's.
  An `include.path = ~/x` in the operator's file expands `~` from `HOME`, so
  under the call's moved HOME the include would follow the attacker on both
  sides and compare equal.
- **The compare resolves HOME-relative values. It does not compare them
  raw** (amended, ranger-base-87zme). git expands a `~` in `core.hooksPath`
  when it runs the hook, from that process's HOME. So in a raw compare,
  `core.hooksPath = ~/managed-hooks` read the same text under a moved HOME
  while naming the seat's own directory. That allowed the commit, and the
  seat's hook ran instead of the managed one. MEASURED 2026-11-06 by holden
  (ranger-base-7czd6). `--type=path` on both sides makes git expand the `~`
  per read, from that read's HOME, so the sides differ whenever the
  directory does. A value with no `~` reads unchanged. Both sides must use
  the same `--type`.
- **A read that fails refuses.** Exit 1 means "no match". Anything above 1
  refuses. `--type=path` exits 128 and prints nothing when it cannot expand
  a `~user` (MEASURED 2026-11-06, git 2.50.1). Two empty reads must not
  compare equal.
- **System lines too** (amended, ranger-base-87zme). The system arm runs
  only when GIT_CONFIG_SYSTEM or GIT_CONFIG_NOSYSTEM is set, and it reads
  both its sides under the call's HOME. So a `~` in the system file plus a
  moved HOME reached no compare at all. The want's `--system` read keeps the
  call's GIT_CONFIG_SYSTEM and GIT_CONFIG_NOSYSTEM, so both sides read the
  same file and differ only by HOME. Which file is read stays the system
  arm's question. `--system` ignores GIT_CONFIG_NOSYSTEM, so a truthy one
  skips that read. It needs `--includes` because, unlike the all-scope
  listing, it reads no includes by default.
- *have* is also counted with `--show-scope --name-only --get-regexp` (no
  `--type`: it prints keys, not values). A key can't hold a newline, so the entry count is honest. A listing with more
  lines than entries is refused. That's because a value with a newline can
  forge a `global` line in the non `-z` listing: MEASURED 2026-11-04, git
  2.50.1, a local alias value `a\nglobal\tcore.hookspath /m` prints exactly
  that line. `-z` is out because sh command substitution drops NULs.
- Equal → allowed; different, or a miscount → `hookskip:config-env`.

MEASURED equal, in a repo and outside one, over two files plus an
`[include]` and an `[includeIf "gitdir:…"]`.

**D3 — Gate the compare on a string test, so the ordinary call runs no
extra git.** The compare runs only when one of these holds:
- `GIT_CONFIG_GLOBAL` is set and differs from the baked pin value (or there
  is no pin);
- `GIT_CONFIG_GLOBAL` is unset and `HOME` or `XDG_CONFIG_HOME` differs from
  the baked value in value or in set/unset.

A pinned `GIT_CONFIG_GLOBAL` with HOME or XDG moved still runs the
compare. git ignores HOME when it looks up the global file, but not inside
it: `include.path = ~/x` in the pinned file expands from the call's HOME.
(The first text of this ADR said such a call skips the compare. That was
wrong, and ranger-base-32ll0 shipped the safer gate; ranger-base-o6xr6
amended this text to match.) Only a call that moved HOME or XDG pays for
this, and the ordinary call never does.

Cost when it fires: 3 + (number of candidates) execs (listing, name count,
the system read, one read per candidate). It was 2 + candidates before
ranger-base-87zme added the system read; the ~85 ms below is that shape. MEASURED 2026-11-04, git 2.50.1, this box at a
load of 7-18: 0.81-0.86 s for 10 compares over two candidates, so ~85 ms a
compare. (The first text's ~40 ms was for the 3-exec shape, at an
unrecorded load.)

**D4 — Fail direction.** No real git at render fails closed, as today. No
candidates (a launcher with no HOME) bakes an empty reference: a seat whose
global scope carries hook keys is then refused, and a seat whose global
scope has none is allowed, which is right because that launcher's git reads
no global hook either.

## Consequences

- The z8kym HOME leg, an XDG move, `env -u HOME`, and both measured
  GIT_CONFIG_GLOBAL bypasses are refused. A HOME pointed at an identical
  config is allowed only when every hook-relevant value in global and system
  scope resolves the same under both HOMEs. A `~`-spelled hooksPath,
  include path or hook command does not, so that move is refused (amended,
  ranger-base-87zme; the first text said "an identical config is allowed",
  and that was the hole). A HOME at a config with no hook keys, on a box with
  none (every hermetic test in this repo), is allowed.
- **Over-refusal: a `~` value under a moved HOME.** A moved-HOME hook verb is
  refused when a hook-relevant value carries a `~` that expands differently,
  even when the content it leads to is identical. It is also refused when
  `--type=path` cannot expand the value at all (a `~user` with no such user,
  even in an alias the launcher's own git never expands). Only a call that
  moved HOME, XDG or GIT_CONFIG_GLOBAL pays this, and the ordinary call
  never reaches the compare (D3).
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
- **Over-refusal: a hook-relevant config value that carries a newline**,
  in any scope, makes a moved-HOME/XDG/GIT_CONFIG_GLOBAL hook verb
  unjudgeable, and it is refused (D2's count). It's ASSUMED rare: the
  regexp covers hook keys and aliases, and nothing in this repo writes a
  multi-line one. The ordinary call never reaches the compare (D3).
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
8. (amendment) The pinned file carries `include.path = ~/x`; `HOME=evil`
   with `GIT_CONFIG_GLOBAL=<pin>`, where `evil/x` drops the hooksPath →
   refused. This pins both the D3 gate firing under the pin and the D2 want
   reads running under the launcher's HOME. Shipped as
   `TestQAHookSkipMovedHomeFollowsNoTildeInclude`. **It does not pin the want
   side following the include at all** (amended, ranger-base-y9i3o, from
   ranger-base-pr6p8 finding 4): what refuses that fixture is the `~` in its
   `include.path` expanding differently under the two HOMEs, so dropping
   `--includes` from the want's candidate read leaves it green — MEASURED
   2026-11-06. Pin 15 carries the no-`~` shape that needs the flag.
9. (amendment) `GIT_CONFIG_GLOBAL=/dev/null` plus a local value whose
   newline forges the launcher's `global` hooksPath line → refused. Delete
   D2's count and that call compares equal and skips the hook at rc 0.
   Shipped as `TestQAHookSkipNewlineForgedGlobalLineIsRefused`
   (ranger-base-ava0x), which pins the same forgery through a moved HOME
   beside the `/dev/null` spelling and carries the mutant as an in-test
   witness: the same two legs through a shim whose count test is rewritten
   to `[ 0 -eq 0 ]` must be allowed at rc 0 with the hook unfired. MEASURED
   2026-11-06, git 2.50.1 (Apple Git-155): with D2's count mutated away in
   `renderHookSkipGlobalScope` itself, both legs land at rc 0 with the
   managed hook unread, on claude and on codex.
10. (amendment, ranger-base-87zme) The launcher's file carries
    `core.hooksPath = ~/managed-hooks`; `HOME=evil` at a byte-identical file
    with its own `managed-hooks/` → refused, neither hook runs. And
    `XDG_CONFIG_HOME=<other>` with HOME as launched → allowed and fired,
    which pins that both sides read with the same `--type`. Shipped as
    `TestQAHookSkipTildeHooksPathIsJudgedByRawText`, holden's pin, inverted.
11. (amendment, ranger-base-87zme) The system file carries
    `core.hooksPath = ~/managed-hooks` with a global file that carries no
    hooksPath; `HOME=evil` at a byte-identical global file → refused. The
    same system file spelled absolute → allowed. Shipped as
    `TestQAHookSkipSystemTildeHooksPathJudgedAgainstLauncher`.
12. (amendment, ranger-base-87zme) The launcher's file carries an alias
    value `~nosuchuser-…`; HOME at a config with no hook keys → refused, and
    HOME at a byte-identical config → refused. Drop the want CANDIDATE
    read's exit check and the first leg compares two empty reads as equal and
    skips the hook. The fixture plants its `~user` in a candidate file, so
    the want SYSTEM read's check and the have listing's are separate
    conjuncts this does not reach (pin 13). Shipped as
    `TestQAHookSkipUnexpandableTildeFailsClosed`.
13. (amendment, ranger-base-y9i3o, from ranger-base-pr6p8 findings 1 and 2)
    EVERY read the compare makes has its rc checked: the have listing's, the
    want SYSTEM read's and each want CANDIDATE read's. One read returns 128
    and prints nothing while every other read agrees → refused; with every
    read agreeing the same moved HOME is allowed. Real git cannot be made to
    fail exactly one of them — the two sides read the same files and differ
    only by HOME/XDG_CONFIG_HOME, so a value git dies on dies on both sides,
    and an `include.path` is itself a key the have listing prints — so this
    renders the shim over a stub git with a chosen rc per read, as pin 7
    does. Shipped as `TestQAHookSkipGlobalCompareChecksEveryReadRC`.
14. (amendment, ranger-base-y9i3o, from ranger-base-pr6p8 finding 3) The
    system file's hooksPath sits behind an `include.path` spelled ABSOLUTE,
    the global file carries none, and HOME is moved to a byte-identical
    global file → allowed and fired. Drop `--includes` from the want
    `--system` read and the want side stops at the `include.path` line, the
    sides differ, and that call is refused. Shipped as
    `TestQAHookSkipSystemIncludeIsFollowedOnTheWantSide`.
15. (amendment, ranger-base-y9i3o, from ranger-base-pr6p8 finding 4) The same
    in global scope: the launcher's file is an `include.path` spelled
    ABSOLUTE whose target holds the managed hooksPath, HOME moved to a
    byte-identical top file → allowed and fired. Drop `--includes` from the
    want CANDIDATE read and that call is refused. This is the include pin
    pin 8 was read as being.
