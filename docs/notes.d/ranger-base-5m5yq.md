# The landed-check judge's environment is an allowlist (ranger-base-5m5yq)

The judge used to get the launcher's whole environment, with `TMPDIR` and
`GOCACHE` redirected (`protocolEnv(append(os.Environ(), cage.Env...))`). That
was stated as a residual in [ranger-base-5fond.md](ranger-base-5fond.md), on
the reasoning that the repo owns what "green" means. The repo still does, over
the environment it is HANDED. What crosses the wall is the wall's decision,
and the judge is landed code that runs on every landing as no persona. Its
output tail is filed into a bead that other seats read.

## What changed

`landedChecker.judge` sets `cmd.Env = landedJudgeEnv(cage.Env)`. The
environment is built from nothing:

- the names in `landedJudgeEnvKeys`, each only if it is set in the launcher:
  `PATH HOME USER LOGNAME LANG LC_ALL TZ TERM GOROOT GOPATH GOMODCACHE
  GOTOOLCHAIN GOFLAGS`;
- then the cage's pairs (`TMPDIR`, `GOCACHE`) on top;
- then `protocolEnv`, which sets PATH to the gates-free one.

`landedJudgeEnvKeys` is the one list. A check that needs one more variable
reds once, and the fix is to name it there. The env is built in `cmd.Env`
rather than with bwrap's `--clearenv` because `cmd.Env` works on both
mechanisms. `sandbox-exec` has no `--clearenv`.

ASSUMED: the design (D3 of the ranger-base-hy1aj decision) lists the first
eight names without the "only if set" it gives the go ones. All thirteen are
taken only when set, because an unset `HOME` handed over as `HOME=` is a
different instruction, not the same one.

The list starts non-nil on purpose: `protocolEnv(nil)` means "all of
`os.Environ()`". That is the one environment this function exists not to hand
over.

## Pin

`TestQALandedCheckJudgeEnvIsTheAllowlist` (`internal/posse/landedjudgecage_qa_test.go`,
arm 1) drives `landedChecker.judge` with a judge that writes its own `env` to
`$TMPDIR`. It plants `FOO_TOKEN` in the test's environment. It asserts that
`FOO_TOKEN` is absent, that no name falls outside the allowlist (plus
`TMPDIR`, `GOCACHE`, and the `PWD SHLVL OLDPWD _` that `/bin/sh` sets for
itself), and that `TMPDIR`, `GOCACHE`, `PATH` and an allowlisted `GOFLAGS`
carry the values they should. It reads what the child got, not what a helper
returns.

Mutation-red, MEASURED 2026-11-12 with `go test -overlay`, tree untouched:

| mutant | result |
|---|---|
| call site back to `protocolEnv(append(os.Environ(), cage.Env...))` | red: `SHELL`, `RHQ_HOME`, … cross |
| list starts nil (so `protocolEnv` falls back to `os.Environ()`) | red |
| the cage's pairs not applied | red |

## Live reading

MEASURED 2026-11-12, D1 host (Linux 7.2.15-1-omarchy), this worktree judging
`3d2f0ea2`, through the real bwrap cage:
`RHQ_LIVE_LANDEDCHECK=1 go test -tags posse_arm2 ./internal/posse -run TestLiveLandedCheck`
read **green in 1m46s**. The one-minute load was 4.84 at the start and 12.52
at the end. That is slower than ranger-base-4nzpo's 76s at a load of 2.2, and
still under D6's 120s ceiling. The design's hand-built `--clearenv` run, with
five variables, read 70s at a load of 4.0. The allowlist adds nothing to the
check's work. The gap tracks the box's load, not the env: one reading, not a
series.
