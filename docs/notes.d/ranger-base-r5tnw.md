# The waiver that outlived its cause, on a tree no seat ran (ranger-base-r5tnw)

Two rows deleted from `cxaUncoveredPlanters`
(internal/treepins/coldexecarms_qa_test.go). That is the whole change; the
rest of this fragment is why a two-line deletion was a P1 and what the
deletion had to prove.

## What was red

`make test` exited 2 for every seat on `main`, before a single suite arm ran,
over a tree with no FAIL line anywhere else. The route:

    coldexecarms_qa_test.go (TestQAColdexecArmRegisterIsNotStale)
      -> Makefile QA_CORPUS_PINS -> corpus-check -> tree-check
      -> a prerequisite of `test` and `test-arm1`

`cxaUncoveredPlanters` waived `cmd/posse` and `internal/treepins` from needing
a `coldexec-check` arm, each row naming ranger-base-in8dr as the bead that
would cover them. in8dr landed those two arms (99cc5324). The rows stayed, and
the stale-register pin reds the moment a row stops being true — by design.

MEASURED 2026-11-11, this worktree at 57dbf70b, linux 7.2.15 x86_64, go1.27.3:

    go test ./internal/treepins -run 'TestQAColdexecArmRegisterIsNotStale'
      -> FAIL 0.278s, both rows named, coldexecarms_qa_test.go:408

## Why the deletion is a fix and not a silencing

While a row sits in the register, `TestQAEveryPlantingPackageHasAColdexecArm`
EXEMPTS that package from needing an arm — so the two arms in8dr landed were
pinned by nothing. ranger-base-qregu measured that: delete both recipe lines
and the doored pins read `ok`. Deleting the rows is what re-arms the
derivation over them, so the deletion was mutation-checked in that direction,
not merely re-run.

MEASURED 2026-11-11, rows deleted, one arm removed per mutant, Makefile
restored and md5-compared byte-identical after each:

| mutant | result |
|---|---|
| none (the fix) | `ok 0.261s` — both pins and the control green |
| `./cmd/posse/...` arm removed | FAIL, `cmd/posse plants an executable and make coldexec-check has no arm reaching it`, coldexecarms_qa_test.go:379 |
| `./internal/treepins/...` arm removed | FAIL, same message naming `internal/treepins`, coldexecarms_qa_test.go:379 |

Each new arm is now held by the derivation on its own. The register is empty,
and empty is it working rather than it being unused.

## Why no seat saw the red before main

ranger-base-s64a5 shipped the register deliberately coupled to in8dr and said
so. Neither seat could see the coupling fire: s64a5's worktree was based at
7c6ad42e and landed 03:32, in8dr's at 79ef83df and landed 04:38, so
`coldexecarms_qa_test.go` did not exist in the tree in8dr ran `make tree-check`
and `make test` over. Both of in8dr's readings were honest on in8dr's tree and
false on the merged one. The launcher's replay gate cannot see it either, by
design: `scripts/posse-land-check` runs `go build` and `go vet` only, and
nothing in it executes a program the branch wrote (ADR 0087 D6,
docs/notes.d/ranger-base-pn621.md).

So the coupling worked — it named the package, the arm and the one-line fix —
and it still cost every seat on the box a red `make test`, because what nothing
reads between two concurrent closes is the MERGED tree. That class is the
architecture lane's, filed as ranger-base-oe9ja. The prose in
`coldexecarms_qa_test.go` that predicted in8dr's seat would see the red is
corrected to say it did not, and why.

## Verified

All 2026-11-11, this worktree, loadavg 1.8-3.6, both suite slots held by
sibling seats throughout (dinesh/ranger-base-xpxo6, gwart/ranger-base-4kars):

- `go test ./internal/treepins -run 'TestQAColdexecArm|TestQAEveryPlantingPackageHasAColdexecArm'` — `ok 0.261s`
- `make corpus-check` — exit 0, 6.0s
- `make fmt-check` — exit 0
- `make -k tree-check` — exit 0, 53.2s, all sixteen doors green (it was the only red in all sixteen before this)
- `scripts/posse-land-check` — green in 4s, 5 checks
- `make test` — arm 1 green (internal/posse 103.3s, cmd/posse 121.2s,
  internal/treepins 134.4s, cmd/coldexec, cmd/testparallel, all ok, so both
  cold-exec arms ran). Arms 2 and 3 could not be read on /tmp: four reds in
  arm 2 and six in arm 3, every one of them a `Disk quota exceeded` write and
  none related to each other. /tmp here is a 14G tmpfs mounted `usrquota`, and
  every seat runs as the same uid, so that per-user quota is ONE box-wide
  budget three concurrent suites spend together — `df` reported 2.8-3.2G free
  on the device throughout. A re-run queued 553s for a slot and died the same
  way. Filed as ranger-base-akk9f, with the second half of the finding:
  test-times.sh's disk explainer greps the literal `no space left on device`,
  so the EDQUOT spelling printed no cause block at all, which is exactly the
  class ranger-base-krra built it for.

  The reading, composed, with the deviation declared:
  arm 2 with TMPDIR on /home (930G ext4, no quota) — `ok 71.1s`, so the quota
  was the whole of those reds. Arm 3 the same way — green but for exactly two
  tests, `TestQAGrantStillReachesTheGitConfigInEveryShape/deniesFiles` and
  `TestSeatbeltWritableReachesGoTelemetryCountersAndNotTheModeSwitch`, both
  artifacts of the moved root and not of this change: the pins build their
  grant set in a hermetic environment while `t.TempDir()` still lands under
  the OUTER TMPDIR, so the fixture falls outside every granted root (the
  printed set ends in a bare `/tmp`; the file is under /home). The product
  honours the variable — seatbelt.go:709 adds `$TMPDIR` when non-empty — so
  this is the two tests' assumption, not a defect. Run on the house default
  they pass: `go test -tags posse_arm3 -run <those two> ./internal/posse`,
  `ok 0.022s`.

  So: every arm green, with arms 2 and 3 read on a moved temp root and those
  two pins read back on the house one. Not the same thing as a plain
  `make test` green on this box, and said that way on purpose. `/tmp` was
  never touched — the trees holding the space were a minute old, so they were
  live sibling suites, and test-times.sh is right that clearing is not a
  seat's call. `go run ./cmd/checkorphans` clean after every run.
