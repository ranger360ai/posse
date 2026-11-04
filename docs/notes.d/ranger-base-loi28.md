# A fixture that scrubs HOME no longer has an empty git global scope (ranger-base-loi28)

MEASURED 2026-11-04 on this box (darwin 25.6.0, git 2.50.1 Apple Git-155),
from a posse-launched seat whose environment carries
`GIT_CONFIG_GLOBAL=~/.gitconfig`, `GIT_CONFIG_SYSTEM=/etc/gitconfig` and an
empty `GIT_CONFIG_PARAMETERS`.

`make test` was red in **every** seat on the box with 11 failures, and all 11
passed when those three variables were shed from the test process:

- `internal/posse` — `TestDeriveIdentityLiteralsSkipsAbsentSourcesSilently`
  (visibility_test.go), "no configured email, no .beads at all — must not
  derive email".
- `internal/treepins` — all ten `TestQAHookFreshness*`
  (hookfreshness_qa_test.go), "prepare-commit-msg is STALE — it does not
  match this binary's render", on a fixture whose whole premise is a fresh
  box. The hook *behaved* correctly throughout
  (`unqualified refused (1) · path-limited allowed (0)`); only the
  byte-identity against the re-render failed.

## The one root cause

Both fixtures made git's global scope empty by pointing `HOME` at a
`t.TempDir()` with no `.gitconfig` in it. That stopped working when
ranger-base-obdnk pinned `GIT_CONFIG_GLOBAL` (and ranger-base-t301k
`GIT_CONFIG_SYSTEM`) to git's resolved global file per launch: **an absolute
`GIT_CONFIG_GLOBAL` REPLACES git's whole global-file lookup**, so git never
consults `$HOME/.gitconfig` at all and `t.Setenv("HOME", …)` displaces
nothing. There is no spelling of `HOME` that empties a scope git is no longer
resolving from `HOME`.

`DeriveIdentityLiterals` then read the operator's real `user.email` inside a
fixture asserting there was none. That literal is an argument to
`CommitGuardHook`, so it was baked into the rendered `prepare-commit-msg` —
and the treepins rig installed its hooks through `append(os.Environ(), …)`
(inheriting the pin) while `hfRig.run` builds the script's environment from
scratch (carrying none of it). Two sides, two different global scopes, so the
reference render legitimately differed from the installed hook. Ten STALE
findings, one cause, none of it a hook defect.

Production is unaffected and this is fixture-only: a seat installing hooks
and the operator later running `scripts/verify-hook-freshness.sh` both
resolve the same `~/.gitconfig`, because the launch pin names exactly the
file git resolved. Only a fixture that builds one side's environment from
scratch sees the two scopes come apart.

## The fix, and the pattern

The pattern was already in the tree — `hsEnv` in
`internal/posse/hookskip_qa_test.go` sheds every `GIT_CONFIG*` beside `HOME`
and `XDG_CONFIG_HOME` for this exact reason. Two helpers now carry it:

- `gitScrubGlobalScope(t, global)` in `internal/posse/commitwall_qa_test.go`
  (untagged, so every arm sees it) — `t.Setenv`s `GIT_CONFIG_GLOBAL` to
  `os.DevNull` or to a scratch file you name, `GIT_CONFIG_SYSTEM` to
  `os.DevNull`, and empties the config-in-env scopes
  (`GIT_CONFIG_PARAMETERS=`, `GIT_CONFIG_COUNT=0`), which outrank both files
  and so are not emptied by shedding the files. MEASURED: under all four,
  `git config --get-all user.email` exits 1 with no output.
- `hfEnv(extra…)` in `internal/treepins/hookfreshness_qa_test.go` —
  `os.Environ()` minus every `GIT_CONFIG*`, which makes an inherited leg
  resolve exactly what `run()`'s from-scratch environment resolves. Used by
  the `install-hooks` leg and `manage`'s dispatch probe. The arms that plant
  `GIT_CONFIG_COUNT`/`KEY_0`/`VALUE_0` deliberately are unaffected: they go
  through `extraEnv`, which `run()` appends last.

**If a fixture's premise is "no global git config", scrubbing `HOME` is not
the way to build it** — name the scopes. Roughly 140 `t.Setenv("HOME", …)`
sites across 50 files in `internal/posse` scrub HOME; only the ones that then
*read git config* are affected, which is why one of them reds and the rest do
not. The next one to grow a git read will red the same way.

Related but not the same thing: ADR 0079 judges a moved `HOME` against the
launcher's resolved global files in the **shim**. This is the test fixtures.
