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
- `make test` — the reading is on the bead's close comment
