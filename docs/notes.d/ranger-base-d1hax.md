## A door with two arms needed two classifications, and the roster can only hold a one-line recipe (ranger-base-d1hax)

`ranger-base-bwp7h` landed `verify-nodb-defer` on 2026-10-04 and
`ranger-base-eawjq` merged it back; the landing (`ca92fb64`) was red on main
for the reason the control exists:
`TestQABoxCheckCensusCoversEveryVerifyTarget` found a new `verify-*` target on
neither the ROSTER nor the EXCLUDED table of `scripts/verify-box.sh`. Same
class as `ranger-base-rsgca` the day before. Everything else at that commit
was green, and `main` was held unpushed behind this one pin with the v0.5.1
tag waiting on it.

MEASURED here, 2026-10-04, this worktree at `ca92fb64`:
`go test ./internal/treepins -run TestQABoxCheck` — one failure, that pin, and
no other.

## The classification, and why it took a second target

The bead's instruction was the right call on the substance and does not fit
the roster as the roster is spelled:

- **Arm A asserts THIS MACHINE.** It builds a throwaway `no-db: true` store,
  defers a record in it with the bd this box resolves, and reads the JSONL
  back. The subject is the installed binary, so the answer moves with the pin
  — which is the roster's own membership rule, and the same rule that puts
  `verify-bd-pin` on it. Its exit 1 is the *good* news: the defect is gone and
  every hand-written date, plus the JSONL recipe in
  `docs/notes.d/ranger-base-bwp7h.md` section 4, can be retired. A check that
  stayed green through that would leave the shop editing JSONL for a bug that
  no longer exists.
- **Arm B cannot be on a clock.** It reads only the stores named on argv or in
  `POSSE_NODB_STORES`; nothing in a repo stamped public can name a private
  store, and with none named it exits 2 saying a verdict over zero stores is
  not a pass. And since `ranger-base-bwp7h` section 6 this box has no known
  no-db store at all — the one that had the symptom was taken off the mode —
  so on a clock arm B is not-measured every run, forever.

So the two arms are classified apart. What stopped the obvious edit — a ROSTER
row for `verify-nodb-defer` — is mechanical, and it is a property of two
things written for good reasons:

1. `run_roster()` executes a row with `eval "$cmd"`, and checks
   `[ -x "$root/${cmd%% *}" ]` first, so the row's first word must be a real
   executable path in the repo — `env -u POSSE_NODB_STORES scripts/…` is an
   ERROR row, not a clever one.
2. `TestQABoxCheckRosterCommandsAreTheirTargetsRecipe` (ranger-base-bbl6r
   finding 3) requires the row to BE the target's **one** recipe line, so a
   check cannot report under a target's name while running something else.

`verify-nodb-defer` has two recipe lines, and the second is
`scripts/verify-nodb-defer.py $(if $(POSSE_NODB_STORES),,--arm-a-only)` — a
make function that `eval` would read as command substitution
(`if $(POSSE_NODB_STORES),,--arm-a-only` as a command). Rostering that target
means either loosening the pin or rewriting the door `ranger-base-bwp7h` had
just made able to go green. Neither is worth it.

**`verify-nodb-defer-capability` is the answer**: one target, one recipe line,
`scripts/verify-nodb-defer.py --arm-a-only`, on the roster. The two-arm door
keeps its name, its `--self-test` prelude and its conditional, and is EXCLUDED
with arm B's reason. This is not a new shape — it is exactly
`verify-bd-dep-safety` (the report, a person types it) beside
`verify-bd-no-relate-pairs` (its `--gate`, rostered): one script, two flag
sets, two targets, one of them on the clock.

One edge is written into the Makefile rather than smoothed over: `--arm-a-only`
is REFUSED (exit 2) when a store IS named, so if `POSSE_NODB_STORES` is ever
exported box-wide this row reads "not measured" with its reason printed. The
answer then is to roster arm B too, not to loosen the flag.

## Verified by running it

- `make verify-nodb-defer-capability` — exit 0, `bd version 0.50.3
  (bd25acbc)`, `status='deferred' defer_until=None`: the defect is still
  present, which is this check's clean verdict.
- `RHQ_HOME=<scratch> scripts/verify-box.sh --quiet` — exit 0, **9** checks,
  the new row among them and `ok`. Run under a scratch home on purpose: the
  aggregate WRITES `$RHQ_HOME/state/verify-box.yaml`, the file posse's
  governance row G10 reads, and a verification run has no business dating the
  operator's surface. (`verify-hook-freshness` reads not-measured there for
  the same reason — the scratch home has no `config.yaml`. It is `ok` on the
  real home.)
- `go test ./internal/treepins -run TestQABoxCheck` — ok.
- `make treepins` and `make tree-check` whole before the close.

The report column went `%-26s` to `%-29s` in the five `printf` sites: the new
name is 28 characters and the old width put its verdict 2 columns out. The
state file is unaffected — posse's reader keys on the YAML `checks:` tokens,
not the human column.
