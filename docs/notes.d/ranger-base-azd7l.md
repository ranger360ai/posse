## `cd ""` was a silent no-op until bash 5.3, and a dead guard woke up (ranger-base-azd7l)

main's only gate went red at 2026-11-06T22:47:16Z and stayed red for three
commits. Nothing in the tree had changed: `ubuntu-latest` moved.

### 1. What was measured

| | last green | first red |
|---|---|---|
| run | [42298066234](https://github.com/ranger360ai/posse/actions/runs/42298066234) | [42307299630](https://github.com/ranger360ai/posse/actions/runs/42307299630) |
| sha | 340bd09c, 2026-11-06T21:36:03Z | bbb507df, 2026-11-06T22:47:16Z |
| runner image | ubuntu-24.04, 20261101.360.1 | ubuntu-26.04, 20261101.186.1 |
| bash | 5.2.21(1)-release | 5.3.9(1)-release |

MEASURED 2026-11-06 from the run logs and from each image's readme in
`actions/runner-images` (`gh api -H 'Accept: application/vnd.github.raw'
'repos/actions/runner-images/contents/images/ubuntu/Ubuntu2604-Readme.md?ref=ubuntu26/20261101.186'`).
The three reds are 42307299630, 42310301976 and 42315652161; in each, one job
of six failed — `test (ubuntu-latest, 2)`, the `posse_arm2` arm — with the same
12 tests, all in `internal/posse/silentrevert_qa_test.go`, each printing

```
fatal: not a git repository (or any of the parent directories): .git
/tmp/TestAudit…/001/audit-mutant.sh: line 177: cd: null directory
```

The other two ubuntu jobs ran the same bash 5.3.9 and passed, and so did all
three darwin jobs.

### 2. The mechanism

`scripts/audit-silent-reverts.sh` opened with

```sh
cd "$(git rev-parse --show-toplevel)" || exit 2
```

which reads git's OUTPUT and not its STATUS. Outside a checkout the
substitution is empty, so the line is `cd ""` — and `cd ""` was a silent no-op
in every bash through 5.2, which means the `|| exit 2` beside it could not
fire on any shell this shop had ever run it on. It was dead code from the day
it was written. bash 5.3 makes `cd ""` an error ("cd: null directory"), and the
guard woke up.

What it then guarded was the half of the script that needs no checkout at all.
`--self-test` plants its own throwaway repos (`mktemp -d`), reads nothing
relative to the toplevel, and is the whole subject of 16 pins in
`silentrevert_qa_test.go`. Twelve of those run a MUTATED copy of the script
from a plain `t.TempDir()` (`srMutateScript` returns that temp dir as the cwd),
so on bash 5.3 every one of them died at line 177 before planting anything:
each got a non-zero exit — which is what the mutation is supposed to produce —
and no mention of the defect it exists for, so each failed on its second
assertion. The four pins that run the UNMUTATED script pass `"."` as the cwd
and stayed green, which is why only 12 of 16 moved, and why `make
verify-silent-reverts` (run from the repo root by every arm) stayed green too.

### 3. Reproducing it on a box with no bash 5.3

This box has bash 3.2.57 only. The behaviour that matters is one builtin's exit
status, and a shell FUNCTION overrides a builtin — exported in bash's own
mangled form, it is inherited by the script under test, whose environment the
Go pins pass through (`srEnv`):

```sh
env 'BASH_FUNC_cd%%=() { if [ "$#" -ge 1 ] && [ -z "$1" ]; then echo "cd: null directory" >&2; return 1; fi; builtin cd "$@"; }' \
  go test -count=1 -tags posse_arm2 -run 'TestAudit|TestSilentRevert' ./internal/posse
```

MEASURED 2026-11-06, darwin, bash 3.2.57: before the fix that command reds the
same 12 tests at the same line numbers CI named, and `go test` without it is
green. After the fix both are green. (zsh refuses to export the name, so the
`env` prefix is load-bearing; `export -f cd` from zsh exports nothing.)

### 4. The fix

The toplevel is resolved by git's exit status, and required only by the half
that reads a checkout:

```sh
TOPLEVEL=
if TOPLEVEL=$(git rev-parse --show-toplevel 2>/dev/null); then
  cd "$TOPLEVEL" || exit 2
fi
…
[ -n "$TOPLEVEL" ] || { echo "audit-silent-reverts: not a git repository …" >&2; exit 2; }
audit "${1:-HEAD}"
```

No new mechanism: `clean-build.sh`, `release-artifacts.sh`, `release-notes.sh`
and `prune-bd-relates-to.sh` already ask this way. A grep over `scripts/` finds
this was the ONLY site reading a `cd` off a command substitution, which is why
the pin for it lives beside the script instead of in a tree-wide census.

### 5. The three pins, and which of them can fail where

- `TestAuditSelfTestNeedsNoCheckout` — `--self-test` from a cwd outside any
  checkout exits 0 and reaches its summary line. This is the property the 12
  mutant arms assumed and none of them stated. On bash ≤ 5.2 the OLD spelling
  passes it too, which is exactly how this shipped.
- `TestAuditOutsideACheckoutSaysSo` — the audit half refuses in the script's
  own words. The status alone proves nothing: the old spelling also exited 2
  from there, by limping on until git's own `fatal: not a git repository`
  stopped it twice (MEASURED 2026-11-06, darwin, bash 3.2.57). Shown able to
  fail against the old script on this box.
- `TestAuditResolvesTheToplevelByStatus` — the shape, over CODE lines only
  (the script's comment quotes the defect on purpose). The only one of the
  three that reds on any bash, and the one a darwin seat can be stopped by.
  Shown able to fail against the old script on this box.

### 6. What generalizes

A guard whose firing depends on the shell rather than on the script is not a
guard, and a platform that cannot fire it cannot tell you so. The portable form
is to take the command's status; the portable way to CHECK a status-dependent
guard, when the platform that fires it is somewhere else, is to override the
builtin in the environment of the thing under test.
