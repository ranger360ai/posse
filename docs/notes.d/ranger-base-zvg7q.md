# The pin that only had positional parameters on darwin (ranger-base-zvg7q)

ci.yml went red on `main` at 894120a7 (2026-11-04T06:33:43Z, run
41786341017) — one job of six, `test (ubuntu-latest, 1)`, one test:

```
--- FAIL: TestQAHookSkipRefusesTheSystemScopeHideDirection (0.03s)
    hookskipnosystem_qa_test.go:112: GIT_CONFIG_SYSTEM to /dev/null hid the managed hook and was NOT refused …
    …four legs, each with an EMPTY output…
    hookskipnosystem_qa_test.go:123: a refusal must write a refusals.log line: … no such file or directory
```

`test (macos-latest, 1)` of the same run was green, and so was every arm-1 run
of that pin on this box. The commit that shipped the pin is the commit that
reddened the gate, under the one gate a commit on main ever passes.

## `. file arg` is not portable, and it fails SILENTLY

The pin rigged the arm by dot-sourcing it with the git call as arguments:

```go
cmd := exec.Command("sh", "-c", ". "+shQuote(shim)+" commit -q -m x -- f")
```

MEASURED 2026-11-04, same script into two shells:

| `/bin/sh` is | `$#` inside the sourced file | `$*` |
|---|---|---|
| bash 3.2.57 (darwin) | 6 | `commit -q -m x -- f` |
| dash (darwin 26.7.1, Apple dash-16) | 0 | empty |

POSIX leaves arguments to `.` unspecified; bash sets the positional parameters
for the duration of the source, dash ignores them — with no diagnostic and exit
0. Ubuntu runners' `/bin/sh` is dash; darwin's is bash. So on linux the arm ran
with `$# = 0`, `posse_hs_check` returned at its own first line
(`[ $# -gt 0 ] || return 0`), the env arm was never reached, and all four hide
legs read "not refused" with nothing on stderr — which is exactly the empty
`out:` after each colon in the CI log.

Reproduced here byte for byte with a `sh` → `/bin/dash` symlink ahead of PATH.

## The arm was never broken on either shell

Rendered with a `#!/bin/dash` shebang and exec'd directly, the shipped arm
refuses all four hide spellings by name, allows all three controls, and writes
exactly four `(hookskip: hookskip:config-env)` lines to the refusals log. The
production shim is `#!/bin/sh` plus the arm, invoked through PATH as `git …`, so
its `"$@"` is real argv on every shell — the defect was in the rig alone. **A
red gate like this costs attribution, not correctness** (the bead's own words):
894120a7's arm was fine, and the next commit to land on a red main would have
had nothing to read.

## The fix, and why not `set --`

The rig now renders what `renderShim` renders — `#!/bin/sh` + the arm — writes
it with `WriteExecutable`, and execs it with the git call as its argv:

```go
arm := "#!/bin/sh\n" + renderHookSkipArm(fake, "", filepath.Join(dir, "hooks")) + renderHookSkipEnvCall()
cmd := exec.Command(shim, "commit", "-q", "-m", "x", "--", "f")
```

`sh -c 'set -- commit …; . arm.sh'` would also have been dash-safe, but it keeps
a shell builtin between the test and the thing under test. Exec'ing a shebang
script is what a seat's PATH does to the shim, so the rig is now the production
form and cannot diverge from it by shell again.

## Still discriminating, both directions

- scope comparison forced to always-equal (`[ "x" = "x" ]`) → all four hide legs
  red, as under ranger-base-tmb7t.
- `GIT_CONFIG_NOSYSTEM`'s git-false spellings (`0`, `false`, `no`, `off`) no
  longer counted as off → `CONTROL FAILED: GIT_CONFIG_NOSYSTEM=0 is off was
  refused`. The controls bite, and the refusal text they print is proof the rig
  now has output at all.

## The class, and why it gets no census pin

"A test rig that dot-sources a script with arguments" is the class. A grep of
every `.go` under `internal/` and `cmd/` for a dot-source found three sites and
only this one passed arguments (`paneline.go` and `paneline_test.go` source
without any; `internal/treepins/suitelock_qa_test.go` names `bash` explicitly
and passes none). A class of one, in a file whose head comment now carries the
trap, is not worth a tree-wide pin and the Makefile door it would owe. If a
second site ever appears, that is the moment for one.

## Verified

- `go test -run TestQAHookSkip ./internal/posse` green, and green with
  `sh` → dash ahead of PATH.
- `make fmt-check`, `make tree-check` green (2:45 at a one-minute load average
  of 7.04, with the five- and fifteen-minute averages at 31 and 79 — the box was
  coming down off a storm).
- Docker is abandoned on this box (operator ruling, ranger-base-6mz7), so
  `make test-linux` cannot run here; the dash reproduction is the linux
  evidence, and ci.yml on the next push is the confirmation.
