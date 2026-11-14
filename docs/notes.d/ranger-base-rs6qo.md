# Three arms that were load-bearing and unheld (ranger-base-rs6qo)

2026-11-14. Verifying four closes — ranger-base-lagas, ranger-base-p2i0q,
ranger-base-b3w9x, ranger-base-wplhv. All four hold. What this fragment
records is the three mutants that SURVIVED on the way to saying so, and the
arms that now kill them. Every reading below is MEASURED on this box
(Linux 7.2.15, go1.27.3, 16 cores, `RHQ_CAGE=shims`, so a bwrap starts from
the seat).

## 1. The exec census read every declaration, and nothing said so

`internal/treepins/treewidedoor_qa_test.go`, over ranger-base-b3w9x.

That bead's fix is two widenings of one question: `twdDecls` reads
declarations with a receiver, and the census then reads every declaration
rather than filtering `all` down to `f.test`. The first half has a live floor
(`the parse names %d declarations with a receiver`, 53 on 2026-11-14) and a
fixture arm. The second half had neither.

MEASURED: putting back the one line the bead removed —

    for name, f := range all {
        if !f.test {
            continue
        }

— left **every arm of the pin green**, and a planted
`exec.Command("scripts/suite-lock.sh")` then walked straight through in BOTH
shapes the bead is about, a method on a rig and a plain helper beside it. The
two-way dead-row arm does not notice, because it iterates `all` unfiltered;
the fixture arms do not, because they drive `twdAnalyze` on one declaration at
a time and never the census's own choice of declarations. The live tree's only
program reached that way, `scripts/bd-argv-gate.py` out of `parserGate`, is
dispositioned, so there was nothing left to red.

The census's rule is a function now (`twdUndispositioned`), called by the pin
with the live map and by a new arm with typed source whose only exec is in a
non-test declaration. The arm asserts its own premise: it names a tracked
program that must NOT be in `twdExecDispositions`, and fatals with the fix if
one is added, because a dispositioned plant makes the arm vacuous.

The first run of this mutant scored GREEN for the wrong reason and had to be
re-driven: `perl -0pi` on `var out []string\n\tfor name, f := range all {`
matched an **earlier identical pair** 600 lines up. The second drive inserted
by line number inside the function, and the kill is that one.

## 2. `wall`'s mechanism guard — the question with no table row

`internal/posse/wallcheck.go`, over ranger-base-wplhv.

`landedChecker.wall` asks three questions in order, and its header says so:
did the tree opt in, is this host's wall a bwrap, did the judge answer green.
Both tables drive the first and the third. Nothing drove the second.

MEASURED: deleting the guard outright —

    if m := WallMechanism(); m != WallBwrap { … return }

— left every row of both tables green, both argv pins green, and
`TestQATheWallCheckExecsNothingButTheCagesOwnArgv` green. On darwin that
deletion is a venue render that refuses, counted as a silence, reaching ADR
0097 D5's watchdog as `wall check silent at <repo>` — a bead filed against
nobody, on every landing, at a host that was never asked to answer.

`TestAHostWhoseWallIsNotABwrapRunsNoVenueAndCountsNoSilence` is the row:
`wallMechanismForTest(t, "darwin")` over the production `once` → `wall` path,
asserting the step did not run, recorded nothing, **counted nothing**, filed
nothing in either lane, and said which host decision it was — with the judge's
own recorded green as the positive control, so the arm cannot pass over a run
that never happened.

## 3. The venue's sandbox-exec refusal, read from the wrong host

`internal/posse/landedcheck.go`, over ranger-base-wplhv.

ADR 0097 D1's other half is that the venue REFUSES where the wall is
sandbox-exec. `wallvenueargv_qa_test.go` has that arm, behind
`switch m := WallMechanism()`, so only a darwin host takes it — not this box,
and not any Linux leg of `ci.yml`.

MEASURED: deleting the `if kind == landedCageVenue` block from the
`WallSandboxExec` case left every arm of both argv pins and both tables green.

`TestQATheVenueRefusesOnTheOtherHostsWallMechanism` reads it from here, which
is what `wallMechanismHost` being a var is for (ranger-base-temhf). It asserts
the refusal cites both records — under the mutant the fall-through reaches
`SeatbeltAvailable()` and refuses with ADR 0088 D3's sentence instead, which is
a refusal for a different reason and would otherwise read as a pass — and
carries a control that the JUDGE's refusal behind the same mechanism is the
mechanism's and not something the fixture did.

## What was re-driven and held

- **ranger-base-lagas.** The two `+check` probe arms, independently: `venue+check`
  exit 0, `wall check: green, 32 pins measured`, 91s; `seat-proc+check` exit 3,
  `venue cannot nest: bwrap: setting up uid map: Read-only file system`, 0s. The
  body's verdict map was then driven over **eleven** synthetic `go test -json`
  streams through a fake `go` on PATH — all green, all pass, zero members, a
  member skip (D5's abstention arm, the one that raised `TypeError` in draft), a
  member with no verdict, a build error, a non-member FAIL under green members, a
  subtest-only marker, a reworded marker, fail-beside-skip, and a marker in
  package-level output — every one mapping to D3's table. Criterion 3 was then
  re-read off the REAL producer: a planted failing member (scratch, removed) gave
  exit 1, `1 of 33 member(s)`, named with its first failure line, in 81s.
- **ranger-base-p2i0q.** `make ci` green, 793 passed. The closer's nine plants
  all RED on re-run, tree restored. Four more: a `except` narrowed to
  `ValueError` (so a 404 escapes), the per-run reset relocated to the end of
  `collect()`, and the failure tuple's `registry_id`/`target` swapped — all three
  RED. Dropping the `__init__` initialization SURVIVES and is an equivalent
  mutant: `collect()` resets both lists before any entry is read, and no
  production caller reads them before `collect()`.
  The edge the close's tests do not cross — the REAL collector through the real
  `pipeline_run._record_registry_outcomes` — was measured once by a throwaway
  probe (not committed): a dead target puts the target in
  `pipeline_result.errors` and calls `record_source_error` with the registry id
  and the 403, a live one advances `record_source_success` and files no error.
  Its control is the pre-fix collector at `dbe6c1c`, where both arms FAIL.
- **ranger-base-wplhv.** Three of the eleven argv mutants re-driven and RED:
  `POSSE_WALL_VENUE=0`, the prep mkdir dropped, `--unshare-pid` dropped.
