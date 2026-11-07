# Verifying four closes: the destroy's PLACE was the unpinned half (ranger-base-8rg06)

Verification of `ranger-base-azd7l`, `ranger-base-8gh58`, `ranger-base-q0np6`
and `ranger-base-43mxx`, 2026-11-07, darwin 25.6.0, at `main` = `ceba0c90`.
Three verify; one does not, because its work is not on `main`.

## 1. ranger-base-43mxx — the mutation table missed one half of its own claim

`destroyRecordedSeat` carries the ordering claim in its own words:

> it goes HERE and nowhere else in this function: the record is the only thing
> that names the seat's id, so a pass that unlinks first has nothing left to
> destroy with, and one that destroys **before the re-proof** destroys on
> evidence read outside the lock.

The close's ten mutants cover the first half (`destroy moved AFTER the
os.Remove`, `destroy removed entirely`). Nothing covered the second.

MEASURED 2026-11-07, eight mutants against `main`, each applied to a clean
tree, `go vet` per mutant, `go test -run
'Prune|Seat|Kill|Relaunch|Retire|Mux|Rex|Reclaim|Meta|Destroy|Husk|Session'
./internal/posse` (244 tests, ~78s unmutated):

| mutant | arms that red |
|---|---|
| the destroy removed entirely | ordering, keep, rex rows |
| the destroy moved AFTER the `os.Remove` | ordering, keep, rex rows |
| the `MuxName() == MuxHerdr` gate removed | herdr-untouched + 5 prune tests |
| the gate reads `m.Mux`, not `MuxName()` | herdr-untouched + the same 5 |
| `!ok \|\|` — "not available" read as "no" | keep |
| **the destroy moved ABOVE the `prunable` re-proof** | **NOTHING** |
| **the destroy moved above every check in `reclaim`** | **NOTHING** |
| the `IsHerdrHang` guard dropped | NOTHING — and the close DECLARES this |

The first survivor was then run against the **whole** arm-1 `internal/posse`
package (1,885 tests, `go test -c` and five name-prefix chunks from the
package dir) to be sure the 244-test filter was not what spared it.

The gate-reads-the-field mutant is worth keeping: every record written before
the `mux:` key existed has `Mux == ""`, so a gate on the field would call
`workspace close` on the whole pre-existing herdr population. `MuxName()` is
what makes that read as herdr, and the suite does pin the difference.

### What the survivor costs

`reclaim` is reached on evidence read OUTSIDE the launcher lock (rangerhq-3a5t,
and `reclaim`'s own doc), so between that reading and the unlink a create can
have put the seat back. A destroy taken before the re-proof ends the session
the re-proof was about to spare — and `prunable`'s refusal then KEEPS the
record, so posse goes on listing a seat it has just killed. That is the
opposite failure from the one D10 fixes, with the same cause.

Nothing could red it because no arm in the package asserts that a destroy did
NOT happen on a non-herdr occupant: `grep -rn 'stub.closes' internal/posse`
finds three assertions and all three want `> 0`. The one negative-destroy arm
is the herdr one, and herdr never reaches this call.

### The pin, and why its fixture is not a call count

`TestQAATheDestroyIsNotTakenOnEvidenceTheReProofWouldSpare`
(`internal/posse/reclaimdestroy_qa_test.go`). `reclaimRaceMux.List()` re-grows
the seat exactly while the launcher lock is HELD — flock is per open file
description, so a second `open`+`LOCK_NB` from this process conflicts with the
one `reclaim` holds, which is the idiom arm 1 reads from the other side. The
outer pass's listings are taken outside the lock and show the seat absent; the
listing `reclaim` takes inside it shows it back.

A fixture that re-grew the seat on the Nth `List()` would stop aiming at
`reclaim` the moment a listing was added or removed anywhere in the pass, and
would then pass by never reaching the branch at all.

It carries its own positive control in the same test — the same rig with the
lever off destroys the seat and unlinks the record — because the raced row's
claim is an absence. MEASURED: green at `ceba0c90` (`-count=5`), red under both
survivors naming the live seat, and under `destroy removed entirely` the raced
row stays green while the CONTROL row fails, which is each row doing its own
job.

The lock lever has one flake, and the loop is why: the flake
`pruneUntil`'s doc measures makes `tryLockLaunches` answer "held" when nobody
holds it, which through this `List()` re-grows the seat for a listing taken
outside `reclaim` — the outer pass then spares the record itself and never
reaches the branch. That costs the WITNESS and never the assertion (a pass that
did not reach `reclaim` took no destroy), so the arm asks the pass again until
`seatEvidence`'s own `seat <id> is alive` appears, and fails if 20 passes never
produce it.

## 2. ranger-base-azd7l — the gate itself is the evidence

`DONE WHEN: ci.yml is green on main again.` MEASURED: run 42335477524 at
`cb389a12`, **all six jobs success**, `ubuntu-26.04` — the image that broke it.
`git log 307948b2..cb389a12` is exactly one commit, the fix, so the attribution
is clean: four reds (`bbb507df`, `32f9998a`, `474337e8`, `307948b2`) then green.
`28b0196e` and `cb389a12` have the same `patch-id --stable` (`bea4c293`), so
the close's sha landed as a rebase.

The three new pins PASS here with no SKIP, and five mutants each red their own
arm: the defective spelling reintroduced as a CODE line, the source pin's
sentinel broken, the refusal block deleted, its words changed, and the toplevel
required in front of `--self-test`. The last one matters: the close says
`SelfTestNeedsNoCheckout` can only fail on bash 5.3 or newer, and on this box's
bash 3.2.57 it fails as soon as a guard is put in front of `--self-test`, which
is the regression it is actually there for.

The fix also holds in a **bare** repository, which neither the bead nor the note
asks about: MEASURED, `git rev-parse --show-toplevel` exits 128 there, so
`TOPLEVEL` stays empty, no `cd ""` is reached on any bash, and the audit refuses
in the script's own words.

## 3. ranger-base-q0np6 — a diverge that is better evidenced than the premise

The fragment landed byte-identical: the blob at `754f1b05`, at `a17f6b45` and at
HEAD is `4b9dd5eb`. (The two commits' patch-ids differ only because the rebase
picked up another fragment's line in the generated `README.md` index.)

Re-checked, not taken on trust:

- the writer — `herdrback.go:2788-2791` is `InstallPrePushHook(dir)` +
  `a.InstallCommitGuardHook(dir)`; `hooksDir(dir)` is `gitPath(dir, "hooks")`,
  whose own doc records the common-dir answer for a linked worktree; and
  `InstallCommitGuardHook` renders `CommitGuardHook(visibility,
  a.OpsPatternSet(), identity...)` — the LAUNCHING home's.
- the cause — both scratch homes' `config.yaml` still exist and carry **zero**
  `data_ceiling_patterns` keys, the box's own carries it, and
  `l3guardgap_qa_test.go:54-60` pins that the ceiling banner is the one banner
  absent for a home that configured none. That is G12's sentence.
- the exoneration's static half — the only `install-hooks` call in
  `internal/treepins` is `hookfreshness_qa_test.go:152`'s rig;
  `verify-hook-freshness.sh:325-329` exits 2 unless git's own `--git-path hooks`
  IS its redirect dir; `protocolEnv` (`protocolchild.go:151`) rewrites `PATH`
  and nothing else.

NOT re-measured here: the 723.438s `treepins` watcher run, and the 1897-vs-1137
render delta — re-taking that one means writing the operator's real repo's
shared hook, which is the defect under verification.

## 4. ranger-base-8gh58 — the work is not on `main`

`internal/posse/codexrex.go` is absent from `main`. `68266f54` and `fae7e566`
are not ancestors of it, and neither are `0g6zm`'s replays `0816bfe4` /
`e67014fd`. Already filed as `ranger-base-0g6zm` (P1, in progress), so nothing
is filed again here.

The live acceptance the bead stayed open for IS evidenced on `main`
independently of the branch: `ranger-base-w4enh` (assignee `codexherdr`) and
`ranger-base-7vh7y` (assignee `codexrex`) are both closed, with commits
`3bc1a6db` and `b14420ef` on `main`. Two codex seats dispatched, settled and
closed their own beads.

Verified by reading the pinned work commit, which is all a seat can do for code
that is not in its tree: 15 pins in `codexrex_test.go`; the D8 byte-identity pin
compares against a **hand-typed** `const want` rather than deriving its fixture
from the subject; `RexRoute`/`RexVerified` carry both a yaml-unreachable pin and
an overlay-preservation pin; and the tty pin's arm 1 (`SelfSandbox` declared, no
seatbelt rendered) asks a real question on this box — only arm 2 skips.

One clause of the done-when is declared unmet by the closer and stays that way:
the first record's exact arrival time against the 45s `startup_wait` was not
recovered.
