# Outside-in review of posse — 2026-09-09, Codex

Reviewed tree: `f97cf1fc`. Reviewer: Richard on Codex, bead
`ranger-base-b0fsz`. Code was read-only; the parallel review was not read.

## Verdicts — the page to read first

**Posse has become substantially better at protecting its own operations,
and substantially larger than its stated job.** The last fourteen days bought
real correctness, recovery and test improvements. They do not establish that
the extra machinery produces more useful work outside the harness. I would
continue supervised use, fix the two queue-boundary defects below before
broadening unattended use, and stop adding autonomous policy until its benefit
is demonstrated on work outside this repository.

1. **FIX — first: bind every beads call to its requested store.** The runner
   changes working directory but inherits `BEADS_DIR`; the aggregator then
   labels returned rows with the directory it intended to query. A caller's
   queue can therefore masquerade as another repository's queue. Reproduced
   with the production runner and the pinned substrate in separate probes.
   `internal/posse/beads.go:202`, `:1007`.
2. **FIX — reject an unresolved exact ID before mutating.** Closing an absent
   ID that prefixes an existing ID closes the latter. Posse detects the wrong
   answer after the write. Its passing regression test proves an error is
   returned, not that the other bead survives. `internal/posse/beads.go:788`;
   `internal/posse/beadsclaimcloseprefix_test.go:72`.
3. **KEEP — explicit ownership and conservative preservation.** Passed lock
   tokens, atomic record replacement, bead-owned seat holds and retained git
   refs are good foundations. Preserve these properties while simplifying.
   `internal/posse/launchlock.go:103`, `herdrback.go:640`, `passcarry.go:269`;
   commit `cbf34dc8`.
4. **FIX — finish bounding subprocess waits.** Herdr and bd have deadlines;
   the common git runner and patch comparison children do not. A stuck git
   operation can still hold landing and its launcher lock indefinitely.
   This is a code-path finding, not a reproduced production hang.
   `internal/posse/worktree.go:259`, `:2241`, `dispatch.go:5013`.
5. **SIMPLIFY — the core and its explanation.** One internal package holds
   117 product files; the four largest core files total 18,076 physical lines.
   The promised small dispatcher and one-file substrate replacement are no
   longer a usable architectural description. Reduce responsibilities before
   attempting a package migration. `DIRECTION.md:58`, `:122`.
6. **KEEP — three test arms, behavioral controls and visible omissions.** The
   partition and actual compiler-file checks pass. They are valuable work;
   mandatory `t.Parallel` eligibility is not evidence of race freedom, and
   neither these checks nor this review certify the full suite.
   `armtags_qa_test.go:497`; `cmd/testparallel/main.go:641`.
7. **DELETE — permanent tests for one-time status wording.** Start with the
   ADR 0036 and 0026 status-line pins, preserving their product behavior tests.
   A historical editorial correction should not become a perpetual compiler
   obligation. `internal/posse/verify_i9dbb_qa_test.go:63`,
   `verify_8dnuy_qa_test.go:36`; `Makefile:494`.
8. **KEEP — the distinction between cooperative gates and enforced walls.**
   I would not ship shims or hooks as hostile-agent containment. The code's
   explicit classes are more credible than counting realized rules.
   `internal/posse/parity.go:62`; `gates.go:3`.
9. **KEEP — the recent removal of autonomous choices; change the progress
   criterion.** Removing paid overflow and model substitution is the best
   direction in the sample. Judge the next fortnight by completed external
   tasks and intervention rate, rather than commit or bead volume.
   Commits `a3852e2f`, `9f438c7b`, `d6185d09`.

## Evidence and consequences

### 1. FIX: directory and queue identity disagree

`Bd.runOnce` sets `cmd.Dir` but never sets `cmd.Env`. Meanwhile,
`planLaunch` deliberately gives sessions `BEADS_DIR` at
`internal/posse/herdrback.go:2494`. A posse command invoked inside such a
session inherits that variable. `ReadyAll` iterates configured directories,
calls `Ready(dir, ...)`, and stamps each result `RepoIssue{..., Dir: dir}`.
`beadsHome` independently resolves the directory's redirect
(`internal/posse/beadloss.go:398`), without considering the inherited variable.
The adapter therefore has two answers to the question of which store it reads.

**MEASURED:** an overlay-only test called production `Bd.Show` with directory
B and a recording executable standing in for bd. The child ran in B but
retained A's `BEADS_DIR`; an assertion that it must not retain A failed.
Separately, pinned `bd version 0.50.3` read synthetic no-db fixtures: with
working directory B and `BEADS_DIR` naming A, `list --json --readonly`
returned A's `probe-a`, exit 0, no stderr. No live queue was changed by either
probe. These establish inheritance and substrate precedence; I did not
dispatch a real job into the wrong repository.

The affected shape is a posse process with an inherited queue binding that
operates on another configured directory. A single-store process whose
inherited value agrees with every requested directory does not exhibit it.
The inferred consequence is mislabeled queue results and actions against the
wrong store; an exact returned bead ID does not authenticate its repository.

**Smallest correction:** make the existing adapter resolve and explicitly bind
the store for each requested directory, consistently with its census and
launch paths. Preserve the session's binding for direct user bd calls; the
fix belongs at posse's cross-repository call boundary. Verify two stores,
redirects, no store, and an inherited contradictory value with actual returned
rows. Do not add another queue service or another persisted registry.

### 2. FIX: a wrong-ID error is too late to protect the other bead

`Bd.Close` invokes `close` at line 789 and compares the returned ID at line
797. `Bd.Claim` similarly sends its mutating command before checking the
answer (`beads.go:619`). The existing close test supplies a canned wrong-ID
response and asserts an error; it does not inspect a store after the call.

**MEASURED:** an isolated JSONL store contained only open `probe-a1`.
`bd --no-db close probe-a --json` returned exit 0 with `probe-a1` closed;
reading the fixture afterward confirmed the persisted status was `closed`.
The requested exact ID did not exist. This reproduces the substrate behavior
the source comment already acknowledges. The product runner's ordering means
its subsequent error cannot prevent or roll back that mutation.

This is relevant to stale IDs as well as typed prefixes. The required
observable is **“requesting absent probe-a leaves probe-a1 open,”** not merely
“Close returns an error.” An exact-ID preflight can reject the ordinary
missing-ID case before the write, but a concurrent deletion between that read
and a prefix-resolving mutation remains a check-then-act race. Keep the
postcondition check, require exact-match semantics at the mutation boundary
for the stronger guarantee, and state the residual honestly if the substrate
cannot provide it. The installed close help exposes no exact-match flag.

### 3. KEEP: ownership is where the architecture is strongest

The launcher lock is a kernel-held file lock, with the actual held token
passed through nested calls. This makes ownership a caller property rather
than a process-global guess (`launchlock.go:103`). `replaceMeta` writes a
temporary file in the metadata directory and renames it over the record
(`herdrback.go:640`); it prevents readers seeing a truncate-and-rewrite gap.
This is an atomic visibility property, not a claim of power-loss durability.

`passcarry.go:269` judges a completed leg and releases a seat only if its
current holder agrees with that leg's bead. Commit `a8380efb` bounded the
gather window so pass duties can run with prompts outstanding; `e2757c02`
replaced torn metadata writes; `0ed7f5b0` landed the holder correction.
These fixes are present in the reviewed ancestry, not just described in ADRs.
Focused lock, metadata and carried-pass tests passed here.

Worktree preservation also has an appropriate failure direction:
`worktree.go:2477` checks before removal, and `cbf34dc8` retains a retired
tip under a git ref when landing was a recorded decision rather than a byte
equivalence proof. Git remains the durable owner of committed work. Preserve
that exit hatch; do not replace it with a database claim that a branch landed.

### 4. FIX: the liveness guarantee stops at the git boundary

`beads.go:202` uses a context deadline and termination grace. Herdr similarly
has typed timeout handling (`herdr.go:233`, `:319`). In contrast,
`worktree.go:259` uses `exec.Command(...).Run()` without a deadline;
`patchIDsVerbatim` starts two unbounded children and waits for both
(`worktree.go:2241`). `dispatch.go:5013` takes the launcher lock before
calling `MergeSessionWork`.

**Code-inspected, not dynamically reproduced:** a git child that never
returns can keep this caller and its lock indefinitely. A watchdog warning
does not release that lock. This matters because configured hooks, filesystem
operations and child processes are outside the dispatcher's own progress
logic. Carry cancellation and a bounded subprocess lifetime into this existing
runner, including its pipe pair; distinguish timed-out mutation from known
failure and re-read git state before retrying. Do not blindly retry a merge
that may already have taken effect. The appropriate timeout value is
**ASSUMED/unmeasured** in this review.

### 5. SIMPLIFY: boundaries exist in prose more than in code

At this HEAD, `internal/posse` is the repository's only internal package:
117 product files and 467 test files share it. Physical lengths are
`gates.go` 5,796, `dispatch.go` 5,489, `herdrback.go` 3,583 and `worktree.go`
3,208 lines. Comments and blank lines are included: these are maintenance
surface measurements, not executable statement counts or a coupling graph.

The useful seams are already there: `Bd`, `Herdr`, the runtime descriptors,
and injected readers on `App` and `Dispatcher`. But `App` also carries model
catalog, CI, CPU and orphan-reaping concerns (`app.go:25`), while Watch owns
several clocks and the core spans routing, spending guards, escalation,
verification, landing and cleanup. Replacing the terminal substrate requires
understanding more than the “one file” promised by `DIRECTION.md:62`.

I would first remove unused policy and duplicated readings, then consolidate
one state transition at a time behind its existing owner. A broad package
split would make this review's uncertainty into implementation risk. The
older dependency-graph measurements in `docs/notes.d/ranger-base-qp1hm.md`
support caution, but I did not rerun that graph and do not adopt its SCC count
as a measurement of this HEAD.

The newcomer burden is measurable: INSTALL is 2,424 lines, NOTES 7,473,
CHANGELOG 2,065, alongside 58 ADR markdown files and 137 note fragments.
The tree explains incidents exceptionally well; it explains the current
minimal system poorly. The entry documents need a short current map of
authoritative stores, writers, launch/settle/landing order, build/test entry
points and remaining manual decisions. Move dated implementation narratives
out of hot-path comments where they obscure that contract. Keep history
available by citation. This is documentation consolidation, not another
status registry that every change must update.

### 6. KEEP: substantial test discipline, with narrower claims

The three-arm partition is real. The census executed here reports
**1,345 / 1,369 / 1,334 tests**, including **522 shared tests in each arm**:
3,004 distinct top-level tests by that census. `TestQAEverySuiteArmTypeChecks`
also asks the toolchain which files it built and vets all three arms; it
passed. `Makefile:272` runs the arms sequentially, and
`.github/workflows/ci.yml` defines a two-platform, three-arm matrix.
The CI definition was inspected; hosted run results were not fetched.

The cost is that an ordinary Go test invocation no longer means the complete
test set. The compiler-file census is a good defense against silent omissions.
The repair chain `e8f62920` → `f97cf1fc`, moving the type-check door and then
restoring the timing wrapper around it, shows how test infrastructure can
create its own integration work. Keep one canonical entry point and stop
adding independently spelled variants of the same contract.

`make verify-parallel` passed with 2,189 eligible tests marked parallel.
Its analyzer explicitly approximates by identifier, ignores receiver identity,
and contains named serial exceptions (`cmd/testparallel/main.go:16`, `:255`).
It is useful enforcement of a chosen scheduling policy; it is not a proof
that shared state is safe. The committed race-arm study
(`docs/notes.d/ranger-base-d0xvw.md`) records failures under instrumentation,
zero reported data races in those runs, and an on-demand recommendation.
There is no race-detector invocation in the inspected default CI/test recipes.
Stabilizing a narrow concurrency-focused race run is a better next test
investment than another source-spelling pin. Its cost here remains unmeasured.

### 7. DELETE: editorial history that has become an execution gate

Two concrete deletion candidates are
`TestQAADR0036StatusLineDoesNotCarryTheRetractedUnbuiltStamp` and
`TestQAADR0026StatusLineDoesNotDeferTheImplementedRung`.
The former forbids particular historical bead/status combinations and checks
that a backup implementation still exists. The latter requires the exact
phrase “implemented in the rendered rung,” alongside checks of its premise.
Both are included in `QA_DOC_PINS` and therefore `tree-check`.

These tests do check something; the objection is what that something costs
to keep. They turn a dated editorial correction into a permanent executable
dependency on a particular account of history. Delete the status-wording
assertions and their membership entries, retaining independent tests that
backup works and the rendered rung has the required behavior. Do not delete
the security, identity-disclosure or behavioral tests merely because they
also inspect text. The benefit sought is less conceptual coupling;
**no suite-speed improvement was measured or is promised** for this deletion.

### 8. KEEP: honest enforcement classes; resist another matcher layer

`parity.go:62` distinguishes enforced from cooperative realization.
`gates.go:3` describes shims and their absolute-path, alias and hook-related
limitations rather than claiming to parse every possible command execution.
The lower-level filesystem and network boundaries are separate mechanisms.
This distinction is a reason to trust the architecture more, provided the
operator-facing claim remains equally qualified.

I would not present a shims-only deployment as a hostile-agent sandbox, or
treat a count of realized deny rules as a containment score. Nor would I
delete the OS boundary because the cooperative layer is bypassable: they
protect different failure classes. Keep the ordinary-path guard small enough
to explain, and require a concrete missed effect before adding another command
parser or option table.

The dependency count also understates operational coupling. `go.mod` is
small, but a functioning installation depends on pinned bd behavior, herdr
state/detection, runtime configuration conventions, git and platform sandbox
behavior. The bd adapter's ready-minus-blocked correction (`beads.go:402`)
and staleness import/retry (`:122`) are evidence of substantial substrate
semantics already absorbed into the harness. Git refs and plain records make
recovery possible; they do not make replacement a trivial shim. I did not
retest live containment or claim this review is a security certification.

### 9. KEEP: removals are compounding; activity alone is not progress

**MEASURED from reachable git history:** the fourteen complete calendar days
`2026-08-26 00:00` through `2026-09-09 00:00`, offset `-04:00`, contain 1,660
commits at this HEAD. The baseline is `32ccff0068d29bdb33cf083a0761898328d92120`,
the last reachable commit before the window. Daily counts range from 3 on
September 8 to 190 on August 29. These are repository activity counts, not
agent productivity or elapsed effort.

| Tracked inventory, physical lines | Baseline | Reviewed HEAD | Change |
|---|---:|---:|---:|
| Non-test Go files | 44 | 123 | +79 |
| Non-test Go lines | 20,574 | 76,138 | 3.70× |
| Go test files | 91 | 571 | +480 |
| Go test lines | 30,499 | 199,309 | 6.53× |
| Markdown files | 43 | 230 | +187 |
| Markdown lines | 10,714 | 44,598 | 4.16× |

775 commits (46.7%) changed only Go tests and/or markdown; 276 changed only
markdown. Nonexclusive counts: 688 touched non-test Go, 1,230 touched Go tests
and 757 touched markdown. NOTES changed in 214 commits, INSTALL in 118,
CHANGELOG in 88 and the Makefile in 48. Classification is mechanical by file
extension; it does not classify intent, usefulness or effort. Merge commits
without ordinary diff output contribute to the total but not a file category.
The August 31 package rename (`9c00e192`) inflates churn statistics; the
endpoint inventories above do not double-count the rename.

What the period bought is tangible: per-session worktree recovery was hardened,
metadata writes became atomic to readers, stale seat releases were corrected,
the pass regained a clock, shared-state fixtures improved, and testing gained
platform/arm checks. Those are investments in keeping work and knowing what
ran. The repeated sequence of abstaining-listing, unreadable-directory,
torn-record and idle-holder fixes in dispatch is also evidence that the
seat-availability decision was scattered across too many paths.

The strongest recent direction is subtraction. `a3852e2f` removes automatic
paid overflow (net 1,584 lines removed across its diff); `9f438c7b` removes
model substitution (net 342 removed). `d6185d09` simplifies pulse policy and
persisted state but adds 354 net lines overall, substantially in evidence and
tests. This last example is why line reduction cannot be the only criterion:
fewer live states can be worthwhile even with more tests.

My judgment is **better and busier, with busyness outrunning demonstrated
product benefit**. The histories explain why individual repairs were made;
they do not demonstrate that all the new machinery was worth creating.
No clean before/after sample of external-task throughput, intervention rate
or lost work was established in this review. For the next fortnight, hold
new autonomous policy steady, fix the queue boundary, and use a fixed sample
of outside tasks to measure completed work, human recovery actions and
retained changes. Do not build another dashboard before collecting that
evidence. The do-nothing alternative for new policy has the lowest immediate
maintenance cost; any throughput cost is **ASSUMED**, not measured here.

## What ran, and what I could not verify

Executed on this tree with `GOCACHE` in a writable scratch directory:

- `make tree-check verify-parallel`: passed, including formatting and all
  listed tree-wide checks. The first attempt with the default cache failed
  on sandbox permissions; that was an environment failure, not a code failure.
- Root-package `-run` selection of
  `TestQAEveryPosseTestFileIsSharedOrInANamedArm`,
  `TestQAMakefileRunsEverySuiteArm`, and `TestQAEverySuiteArmTypeChecks`:
  passed, 3.809s package time, all three type-check subtests named.
- Default-arm selection
  `^(TestLaunchLock.*|TestBdClaim.*|TestBdClose.*|TestWriteMeta.*|TestReadMetaSaysWhenTheFileCarriesNoRecord)$`:
  passed, 2.493s. Repeated with `-tags posse_arm3` for the lock and metadata
  selections residing there: passed, 2.721s. No inference was made from a
  filter matching zero tests; verbose output identified those executed.
- The four `passcarry_qa_test.go` tests for bounded completion, offering a
  free seat work, draining carried legs and waking on a carried settle:
  passed, 4.142s, including their unbounded controls. They used fake backends.
- The production-runner environment probe: expected failing assertion,
  0.607s; an initial probe draft failed compilation due to a missing mode
  argument and supplied no evidence. Corrected probe ran afterward.
- Pinned-bd two-store read and missing-ID close probes: behaviors recorded
  above, using synthetic temporary JSONL stores only.
- `go run ./cmd/checkorphans`: could not inspect the process table because
  the sandbox denied `/bin/ps`; leak status is unknown. All test/tool sessions
  started for this review returned completion, but that is a narrower check.

I did not run `make test`, any full arm, `-race`, hosted CI, the live-container
probes, backup/restore rehearsal, a cold installation, or live runtime sessions.
No real dispatch or session launch was performed. No production-code defect
was fixed in this task. All reported timings are observed package timings,
not a performance baseline.

There is no `docs/rca/` directory at the reviewed HEAD; relevant incident
material was read in commit diffs, ADRs and note fragments instead. The large
committed timing/SCC/incident studies are attributed as prior evidence where
used, not presented as fresh measurements. Release documentation and local
tags were inspected, but installation claims and current hosted release state
were not independently verified. No claim about the deployed binary follows
from this worktree's HEAD.

## Reproduction details for the two leading findings

The substrate probes can be repeated with the pinned binary and the following
isolated fixtures. The first invocation is read-only; the close changes only
the temporary synthetic store. No initialization, daemon or live queue is
needed.

```python
import json, os, pathlib, subprocess, tempfile

root = pathlib.Path(tempfile.mkdtemp(prefix="outside-in-bd-"))
def store(name, issue_id):
    repo = root / name
    directory = repo / ".beads"
    directory.mkdir(parents=True)
    row = dict(id=issue_id, title=name, status="open", priority=2,
               issue_type="task", created_at="2026-09-01T00:00:00Z",
               updated_at="2026-09-01T00:00:00Z")
    (directory / "issues.jsonl").write_text(json.dumps(row) + "\n")
    return repo, directory

a, ad = store("a", "probe-a")
b, bd = store("b", "probe-b")
subprocess.run(["bd", "--no-db", "--readonly", "list", "--json"],
               cwd=b, env=dict(os.environ, BEADS_DIR=str(ad)), check=True)
# Observed: probe-a, despite cwd=b.

c, cd = store("prefix", "probe-a1")
subprocess.run(["bd", "--no-db", "close", "probe-a", "--json"],
               cwd=c, env=dict(os.environ, BEADS_DIR=str(cd)), check=True)
print((cd / "issues.jsonl").read_text())
# Observed: probe-a1 persisted as closed; probe-a never existed.
```

The separate runner probe used Go's `-overlay` to append a temporary test to
`beadsclaimcloseprefix_test.go` without changing its on-disk bytes. The test
set `BEADS_DIR` to one temporary directory, called `Bd{Bin: fake}.Show` with
another, and made `fake` print a JSON issue with title from `BEADS_DIR` and
description from `PWD`. It wrote the executable with
`WriteExecutable(fake, script, 0o700)`. The working directory matched the
requested target, the title remained the caller's store, and the assertion
rejecting that inherited store failed. The exact production lines responsible
are cited in finding 1; neither the overlay nor its fixtures are needed by
the repository after this review.

The inventory uses `git archive` of the baseline and HEAD, counting regular
tracked `.go` and `.md` members with `splitlines()`; `_test.go` selects tests.
Commit categories use `git log --numstat --no-renames` over the stated window.
All line references above refer to `f97cf1fc`.
