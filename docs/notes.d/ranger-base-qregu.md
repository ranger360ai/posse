# Four closes verified, and the coupling neither seat could see (ranger-base-qregu)

Verify of ranger-base-s64a5, ranger-base-n8wek, ranger-base-quzcd and
ranger-base-in8dr. Three verified; ranger-base-in8dr did not, and what it left
behind reds `make test` for every seat on the box.

Every reading below was taken 2026-11-11 from this bead's worktree at
HEAD == main **ef842dbe**, clean tree, linux 7.2.15 x86_64,
`go version go1.27.3 linux/amd64`, one-minute loadavg 4-8 with slot 1 held by a
sibling seat throughout.

## The live red, and its route

`make test` exits **2 in 31.9s**, before a single suite arm runs. The whole of
it is one pin:

| command | reading |
|---|---|
| `go test ./internal/treepins -run TestQAColdexecArmRegisterIsNotStale` | FAIL 0.24s, both rows named |
| `make corpus-check` | Error 1, exit 2, 6.6s |
| `make -k tree-check` | exit 2, 46.4s, and the ONLY red in all sixteen doors |
| `make test` | exit 2, 31.9s |

Route: `QA_CORPUS_PINS` (Makefile:713) -> `corpus-check` (Makefile:974) ->
`tree-check` (Makefile:1030) -> a prerequisite of `test` (Makefile:330) and
`test-arm1` (Makefile:339). A green tree and a non-zero exit with no FAIL line
that names a defect in the code — AGENTS.md's gofmt shape, and
ranger-base-quzcd's own shape five hours earlier on the same morning.

`cxaUncoveredPlanters` (internal/treepins/coldexecarms_qa_test.go:138-141)
waives `cmd/posse` and `internal/treepins`, and ranger-base-in8dr gave each of
them a `coldexec-check` arm at 99cc5324. Filed: ranger-base-r5tnw (code lane).

## The mutation set over ranger-base-s64a5's three pins

Six arms, each applied, run, reverted, and the tree proved clean after
(md5 of both files identical to the control at the end). `PINS` is
`TestQAEveryPlantingPackageHasAColdexecArm|TestQAColdexecArmRegisterIsNotStale`.

| mutant | reading |
|---|---|
| control, unmutated HEAD | register FAILs on both rows, 0.25s |
| **both of in8dr's new arms deleted from the recipe** | **`ok` 0.254s — the arms are pinned by nothing today** |
| the two stale register rows deleted (the fix) | `ok` 0.248s — nothing else is wrong |
| rows deleted AND `./cmd/posse` arm removed | derivation reds naming cmd/posse, :366 |
| a FOURTH planting package (`cmd/coldexec` given a plant call site) | derivation reds naming cmd/coldexec, :368 |
| one arm's `$(COLDEXEC_BIN)` stripped | derivation reds with the SUPERVISOR message, :345, not the coverage one |
| `cxaPlantWriter` renamed so the matcher reads nothing | both doored pins red through the site floor with the floor's own message, :336 |

Row 2 is the finding and row 3 is the fix. Rows 4-7 are
ranger-base-s64a5's own four mutants re-run by a second hand, all killed, each
for the right reason: the pins are sound and the register is the mechanism
working as written on a tree no seat ran.

The harness itself is not kept: it lived in the session scratchpad and is seven
`sed -i` edits with a `git checkout --` between them. Rebuild it from the table
rather than looking for it.

## Why neither seat could see it

ranger-base-s64a5's bead said the coupling out loud: "Whoever lands second
deletes the matching row from cxaUncoveredPlanters; the red names the row and
says so." Nobody landed second in a tree holding both halves.

| bead | base | landed | held the other half? |
|---|---|---|---|
| ranger-base-s64a5 (gwart) | 7c6ad42e | 03:32 | no — in8dr's arms did not exist yet |
| ranger-base-in8dr (dinesh) | 79ef83df | 04:38 | no — coldexecarms_qa_test.go did not exist in it |

So dinesh's `make tree-check rc=0` and `make test rc=0 in 643s` were both
honest on dinesh's own tree and both are false on the merged one. The
launcher's replay gate cannot see it either, by design rather than omission:
`scripts/posse-land-check` is `go build ./...` plus four `go vet` runs and
ADR 0087 D6 states nothing in it executes a program the branch wrote.
`grep -n -i rebase AGENTS.md` returns two bullets, neither of which asks a seat
to rebase onto main and re-run the doors before closing. The class is filed as
ranger-base-oe9ja (architecture lane) with four unmeasured candidate answers
and no pick.

## The plant census: three numbers, one tree

Worth writing down because all three are correct about different questions and
two shipped documents use the wrong one.

| number | what it counts | where |
|---|---|---|
| **331** (261/53/17) | `WriteExecutable` CALL SITES, parsed | coldexecarms_qa_test.go's head comment, MEASURED by the pin |
| 345 (274/54/17) | `grep -o 'WriteExecutable('` in dinesh's tree at c80a4c9b | AGENTS.md and ADR 0085 D6, both of which call them call sites |
| 354 (274/63/17) | the same grep at ef842dbe | nowhere — s64a5's control file added nine quoted mentions |

331 + the two `func WriteExecutable` declarations + twelve quoted mentions =
345, which is gwart's reconciliation and holds. The nine the grep gained are
`cxaPlantWriter`'s synthetic fixtures in the control. A grep of the name was
never the call-site count and is now also tree-dependent, which is exactly why
the pin parses.

## What verified, and how

- **ranger-base-n8wek / ranger-base-quzcd** (one line,
  scripts/silent-reverts.allow, 5cb33d70). `--quiet` exits **0** over 2480
  commits, 0.4s; delete the line and it exits **1** naming f1f9a2f, restored
  byte-identical. `--self-test` PASSes every arm including "the triage print
  strips the patch-id token" and "the token on an ANCESTOR's line is inert".
  `git diff-tree -p f1f9a2f5 | git patch-id --stable` is
  `f55481f9ce644001218a43406fdb4fe27b6660ed`, the token on the line (`git show
  | git patch-id` is a different value, as gilfoyle's comment warns). One line
  for the sha, reason opens with `Benign` and not a bare 40-hex word, and the
  non-quiet print quotes it from that first word. The deletion's own
  correctness re-checked rather than taken on the line's word: the six
  `TestQAWholeTreeDeny` pins PASS, the named successor
  (seatbeltdenygitgrant_qa_test.go:247-258) really does drive both store
  shapes with a `t.Fatalf` guard that the redirect was followed, and the
  retired file's name survives in exactly the two notes fragments the line
  claims.
- **ranger-base-s64a5**: the mutation table above, plus `make register-check`
  green (19.5s), so both new pins have their door and the count moved with
  them.
- **ranger-base-in8dr**: five arms, all five spelled `/...` and all five
  carrying a literal `-timeout 60m`; `*/...` is what `suite_lock_wanted` reads
  (scripts/suite-lock.sh:224), so all five queue;
  `TestQANoEntryPointRunsGoTestOnTheDefaultTimeout` green, which is the pin
  that refused the make-variable spelling; `cmd/coldexec/doc.go` says five arms
  and names them; ranger-base-orgvt filed with its `discovered-from` edge
  confirmed. **Not re-measured:** the 674.8s / 1581.8s doored walls — a
  quarter-hour of box for numbers ranger-base-xpxo6 owns, and the
  internal/treepins reading now predates 535 lines of new test file in that
  package.
