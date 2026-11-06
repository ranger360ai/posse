# ranger-base-rrklg — verifying four closes

QA record for the batched verify of ranger-base-7czd6, ranger-base-c1pqo,
ranger-base-jmkjb and ranger-base-dsuk7. Everything below was RUN, on
darwin 25.6.0, git 2.50.1 (Apple Git-155), at main `24ab7bbf`, on
2026-11-06. Where a reading is a number, the load average it was taken at
is beside it.

## 1. The hook-skip read arm past its pinned spellings (ranger-base-c1pqo)

ranger-base-c1pqo taught `posse_hs_readverb` to skip the leading options
before `remote`'s subcommand and took `-v`/`--verbose` out of
`hookSkipReadSubverbs`. Its shipped legs carry the seven spellings the bead
measured. The skip's claim is that it does not depend on that list, so the
spellings the bead did NOT name are the question.

MEASURED first against **plain git, no shim**, in a scratch clone with a
stale remote-tracking ref — `git remote -h` prints the surface, and `-v,
--[no-]verbose  be verbose; must be placed before a subcommand` is the ONLY
top-level option `remote` has:

| argv | rc | stale tracking ref |
|---|---|---|
| `remote --no-verbose prune origin` | 0 | **DELETED** |
| `remote --no-verb prune origin` | 0 | **DELETED** |
| `remote -v -v -v prune origin` | 0 | **DELETED** |
| `remote -v update --prune` | 0 | **DELETED** |
| `remote --no-verbose` | 0 | survived (a read) |
| `remote -v set-url origin /nowhere` | 0 | survived (a config write) |
| `worktree -v list` | 129 | `unknown switch 'v'` |

So `--no-verbose` and its abbreviations are the half of git's own
`--[no-]verbose` spelling that no shipped leg reaches, and `worktree` takes
no option before its subverb — which is what makes the skip safe there.

Then through the rendered shim, on claude AND codex:
`internal/posse/verify_rrklg_qa_test.go`,
`TestQAHookSkipOptionSkipHoldsPastItsPinnedSpellings` — eight refused legs
(the four ref-updating spellings above, `remote -v set-url`,
`remote --no-verbose remove origin`, and two with a global git option
between the override and the verb) and five allowed (`remote --no-verbose`,
`--no-verb`, `-v -v get-url`, `--no-verbose show -n`,
`--no-pager remote -v get-url`). **GREEN at 24ab7bbf**, 2.06s and 2.44s.

Shown able to fail, both directions, against the PRE-FIX file
(`git show fc72606f:internal/posse/hookskip.go`): `remote -v update
--prune`, `remote -v -v -v prune origin` and `remote -v set-url` each
**exited 0**, and the three `--no-verbose` read legs were falsely refused
under `hookskip:config-override`.

### The rendered function across shells

The close claimed the rendered `posse_hs_readverb` gives the same answer
under `/bin/sh` (bash 3.2) and `/bin/dash`. Re-run over the 17 argv shapes
above, with the function dumped straight out of `renderHookSkipReadVerb`:
**`/bin/sh` and `/bin/dash` agree on all 17.**

`/bin/zsh` does NOT — it answers REFUSE where the other two answer READ for
every `remote`/`worktree` subverb arm, `worktree list` included. Unreachable
and harmless, said here so nobody re-derives it: the shim's own first line
is `#!/bin/sh` (`renderShim`, internal/posse/gates.go:1518), so zsh never
interprets it, and the divergence is fail-CLOSED — zsh refuses MORE, which
is the accepted over-refusal class the RESIDUALS paragraph already names.

The first attempt at this reading was a probe that could not fail: the
function was dumped with `go test` and no `-v`, which suppresses a passing
test's stdout, so the driver sourced an empty file and printed REFUSE for
all 17 shapes under every shell — including `status`, which is a plain read
verb. The positive control is that `status` reads READ.

## 2. The c1pqo mutants

Two, each run, against the landed pins
(`TestQAHookSkipRefusedOnEveryRuntime`,
`TestQAHookSkipReadVerbsAreNotHookVerbs`):

- **The whole pre-fix file** (`fc72606f`, whose only delta to main in that
  file is the three-line fix). Both pins red on both runtimes: `-v prune`,
  `--verbose prune`, `-v -v prune`, `-v update` and the `prm` alias all
  exited 0; `-vv get-url origin` was falsely refused; the table test named
  `-v` and `--verbose` as options in the subverb table and named the `"$@"`
  call site. `-vv prune` and `--verb prune` stayed refused under the mutant,
  which is what the bead already said ("safe, by accident").
- **Per entry**: `"prune"` added to `hookSkipReadSubverbs["remote"]` with
  the fix otherwise intact. Red, naming all seven prune legs. So the table
  is load-bearing row by row and not only as a whole.

## 3. The ADR 0079 tilde hole, confirmed by execution (ranger-base-7czd6)

ranger-base-7czd6's closer confirmed the finding by READING
`renderHookSkipGlobalScope`. Confirmed here by RUNNING it: the pin filed
with the finding (`internal/posse/verify_h0hjx_qa_test.go` at `627197ff`,
`TestQAHookSkipTildeHooksPathIsJudgedByRawText`, which ships green
asserting the hole) compiles against main unchanged and **passes on claude
and codex** at 24ab7bbf — so all three of its arms hold there:

- CONTROL 1 — a plain commit through `core.hooksPath = ~/managed-hooks`
  fires the managed hook once, so the reading below is true of something.
- CONTROL 2 — a HOME whose config drops the key IS refused under
  `hookskip:config-env`, so "allowed" is a verdict and not a dead arm.
- THE BOUNDARY — all five `git config --global core.hooksPath` spellings
  stay refused, which is what prices the finding at P2 rather than P1.
- THE HOLE — HOME at a byte-identical `.gitconfig` with its own
  `managed-hooks` dir: rc 0, managed hook fired 0 times, the seat's own
  pre-commit ran once, no refusal printed.

The pin is on `posse/holden-posse-ranger-base-h0hjx`, which is not an
ancestor of main. It was borrowed for this reading and removed again;
ranger-base-87zme's done-when names it with its branch, so nothing here is
stale, but the fix lane has to bring it across.

## 4. The merge-back's content (ranger-base-jmkjb)

`2f1d59aa` on main carries `160d6bd9`'s author identity, AUTHOR date
(2026-11-04 06:31:25 -0500) and subject byte for byte, so the pair holds and
no later pass re-files the block. The merge-blocked ref is gone.

Content checked line by line rather than by ancestry: of the 49 non-blank
lines `160d6bd9` ADDED across its five files, 48 are present verbatim in
main's copy of the same file. The single exception is one prose line of
`hookskip.go`'s RESIDUALS paragraph — exactly the hand-resolved three-way
merge the close disclosed. Read on main, the resolution keeps all three
claims: ranger-base-e09l0's `split_cmdline` sentence verbatim,
ranger-base-d4bk4's `renderAliasDeny` clause appended to it, and
ranger-base-32ll0's two new residuals with "A moved HOME / XDG_CONFIG_HOME
is no longer a residual (ADR 0079)". No test reads that paragraph —
`grep -rn 'RESIDUALS' --include='*_test.go'` is empty.

## 5. The wall-clock-age class is not finished in cmd/posse (ranger-base-dsuk7)

ranger-base-dsuk7's two pins verify: `TestQAStatusSaysTheMeterIsQuiet` and
`TestQAStatusSaysTheGuardIsUnarmedOverAnOldReading` both bracket through
`agedAgesOf` and skip through `skipIfTheBoxSleptMidTest`, green in 2.55s and
2.04s, and an empty bracket errors rather than passing, so the bracket
cannot go vacuous.

What does NOT hold is the close's census claim, repeated in
ranger-base-w4enh as "cmd/posse is done".
`cmd/posse/planstale_qa_test.go:124`,
`TestQAStatusQuietWhereTheLineWouldLie/fresh_reading`, seeds the plan-usage
reading one minute old, launches the binary and asserts it prints nothing
about blindness. The premise is a WALL deadline — fresher than
`PlanUsageStaleAfterDefault` (2h) — read by another process, and it carries
no skip guard. The narrow claim the close actually typed is true (the pin
uses `runStatus`, not `runPosse`, and carries no exact-age literal); the
broad one is not.

MEASURED by seeding the state a 2h+ sleep leaves behind — the row's age
changed from one minute to three hours, nothing else:

```
--- FAIL: TestQAStatusQuietWhereTheLineWouldLie/fresh_reading (1.41s)
    planstale_qa_test.go:134: fresh reading must print nothing about blindness:
    ...
    plan meter BLIND 3h00m: last reading 2026-11-06T11:14Z ... 1 consecutive failed reads
```

The binary is right in that output and the fixture is what expired, which is
the signature ranger-base-gt0q9 measured (this box slept 2h51m02s inside one
cmd/posse test). The guard it needs is in the same package already.

Two more out-of-process fixtures in `cmd/posse` state a premise the same way
and are unread leads, not findings: `runtimeavail_qa_test.go`'s
`seedRuntimeCatalog` (`at` seeded `age` old, plus a `retry_at` cooldown) and
`backupsurface_qa_test.go`'s `backupHome`/`plantArchive` (the archive's age
is its NAME's stamp, read against a retention window). The discriminator is
ranger-base-w4enh's: whether one clock reads both sides.
