# ranger-base-qei5l — verifying four closes: what the mutants said

Four closes verified adversarially (ranger-base-pr6ms, ranger-base-k2olp,
ranger-base-u66yb, ranger-base-7mfnf). All four did what their beads asked.
Two of them left a guardrail that no test could see, and both gaps are pins
now; one left a false sentence in a measurement record.

Environment for every reading below unless stated otherwise: this box, linux
7.2.15-1-omarchy, 16 cores, go1.27.3, git 2.56.0, 2026-11-22.

## ranger-base-pr6ms — `am --skip` off the hook-skip allowed list

A NO-DIFF close: the change had already landed under ranger-base-t4qez
(2a32fa07, an ancestor of this worktree's HEAD and on `main`). Verified by
putting `--skip` back on `hookSkipAmAllowed` and running the arm.

**Four independent arms die on that one-line mutant**, which is more than the
close claimed:

| arm | what it said |
|---|---|
| `hookskip_qa_test.go:836` | HEAD moved — the form COMMITS with a tail in the mbox |
| `hookskip_qa_test.go:847` | `am --skip` not refused through the shim |
| `hookskip_qa_test.go:850` | HEAD moved through the shim — the refusal must precede git |
| `continuewall_qa_test.go:353` | an independent sweep: the option ADVANCES an am |

The `:836` arm is the one worth having: it is the fixture's own witness, a
REAL stalled two-patch am, so the removal's premise is pinned as a
measurement rather than as prose. The spelling legs at `:631` die too, on
both runtimes.

The one assertion the bead listed and the close did not write
(`.git/rebase-apply` still present after the refused `--skip`) is genuinely
redundant: `hsAmStallWithTail` asserts the dir present at window open, and
the refusal is proven to precede git by a non-zero code with HEAD at
baseline — nothing between those two points can clear the dir.

## ranger-base-k2olp — `--cmd` beside `--agent` refused

Four mutants, graded **un-combined**, each restored before the next:

| mutant | verdict |
|---|---|
| `CheckCmdAgent` never asked | DEAD — 4 message assertions, the `no such agent` arm, the no-state walk |
| refusal downgraded to a warning on stderr (compiles) | DEAD — the `no such agent` arm and the no-state walk only |
| the check reads one flag (`o.Agent` arm dropped) | DEAD — the `--cmd` alone row |
| the `--cmd` help entry loses its refused-beside clause | DEAD — 3 assertions |

The warning mutant confirms the close's own recorded finding: it prints the
same message and exits 1 on the next thing it hits, so only the
`no such agent` arm and the no-state walk can tell a refusal from a warning.

**The coverage claim holds.** Every non-test `NewSessionOpts` construction
site, read: `cmd/posse/main.go:449` and `:2924` (the latter is
`parseNewFlags`, where the check is), `dispatch.go:5842` and `:5951` (Agent,
no Cmd), `herdrback.go:4882` (the recipe path — `r.Command` beside
`r.Agent`, filed as ranger-base-g27p4) and `relaunch.go:309`
(`RecreateOpts`, both from the meta). `~/homes/ranger/recipes/grok-fam.yaml`
really does declare `command: grok` beside `agent: grok`, so leaving the
recipe layer alone was the right call and not an omission.

### The gap: the guard that argument rests on

`relaunch.go` is covered by one `if` — `herdrback.go`'s
`if o.Agent == ""` around `meta.Cmd = o.Cmd` in `startPlanned`. Changing it
to `if true` — so a persona session's meta records the typed line after all
— left every test matching
`Relaunch|Recreate|MetaCmd|CmdAgent|PlainSession|StartPlanned|WriteMeta`
`ok`. A reader census says why: the only two assertions over `m.Cmd` are
`TestRelaunchReplaysPlainCommand`, whose session is persona-LESS and whose
claim is that the cmd IS recorded, and a kept-recipe test comparing
`m2.Cmd` to `m.Cmd`, which reads the same value whichever it is.

With the guard gone, `RecoverCommand` would print
`posse new … --agent … --cmd …` — a line posse now refuses — and a relaunch
would discard the typed line in silence, which is k2olp's own defect one
layer down from the flag it fixed.

Pinned: `internal/posse/metapersonacmd_qa_test.go`. The control (a
persona-LESS session with the same typed line must still record it) is what
makes the empty read evidence; the INVERTED guard kills both arms, which is
how the control is shown able to fail.

## ranger-base-u66yb — the upgrade lock's wait is bounded

The close's pins pass, named, on the tree as it stands:
`TestUpgradeLockPathSitsInThePosseCacheDir`,
`TestUpgradeLockRefusesWithinItsBoundAndNamesTheKernelsHolder`,
`TestUpgradeLockTakesTheLockWhenTheHolderLetsGoInsideTheBound`,
`TestFlockHolderPidReadsTheKernelsTable`. The one `--- SKIP` is
`TestUpgradeLockHolderChild`, the squatter, guarded on
`POSSE_UPGRADE_LOCK_HOLD` and re-exec'd as a child by the parent pin, which
passed — so the child really ran and really took the lock.

### The gap: the bound itself was pinned by nothing

`const UpgradeLockWait = 60 * time.Minute` is this bead's whole deliverable
and its one ASSUMED number. **MEASURED: `1000 * time.Hour` — the defect the
bead was filed on, respelled in one line — left internal/posse arms 1, 2
and 3, `cmd/posse` and `internal/treepins` all `ok`, with gofmt and
`go vet` clean.**

A census settles it without sampling, and this is the stronger half: **no
`*_test.go` in the tree names `UpgradeLockWait` or `upgradeLockPoll`, and
none calls `lockUpgrade`.** All three pins drive `lockUpgradeWithin` with a
bound of their own — 300ms, 1s, 30s. Production reaches the bound only
through `lockUpgrade`, wired in as `upgradeReads.Lock`, which every
verb-level test replaces with a fake. So the value was unobservable by
construction.

This is the class `upgrade_test.go`'s own "the wiring" header
(ranger-base-anczm) was written about — a guardrail pinned as a FUNCTION
while nothing reaches the verb that calls it — one landing later and one
constant over.

Pinned: `internal/posse/upgradelockbound_qa_test.go`, two arms, because the
behavioural pin cannot be written (asserting the production bound by WAITING
for it costs an hour, and `lockUpgrade` takes no bound to shorten):

| arm | mutant | verdict |
|---|---|---|
| the bound is in range | `UpgradeLockWait = 1000h` | DEAD |
| | `upgradeLockPoll = 0` | DEAD |
| `lockUpgrade` hands the bounded form the CONSTANT | a literal in its place | DEAD |
| (positive control) | `lockUpgrade` emptied | DEAD — "0 calls, want exactly 1" |

The range arm refuses only the values that make the verb a different verb: a
floor at ADR 0092's 25-minute planning envelope, so a genuine slow upgrade
still queues rather than being refused as a squatter, and a ceiling at 2h,
past which it is the unbounded wait again.

## ranger-base-7mfnf — the re-plant hazard cannot reach a `#!` script

**The conclusion holds and the central measurement reproduces.** An
independent re-derivation of the two-arm probe, written from this bead's own
fragment (which the close said was "twenty lines to re-derive" — it is):

| arm | bytes | n | ETXTBSY |
|---|---|---|---|
| ELF, Go (positive control) | 1,951,545 | 400 / 2000 | 17.50% / 17.10% |
| `#!/bin/sh` | 17 | 400 / 2000 | **0.00% / 0.00%** |
| ELF, `/bin/true`'s bytes | 43,320 | 400 / 2000 | 0.00% / 0.00% |

and on the REUSED path, which is this bead's own shape (re-plant a path
already exec'd), at n=400: Go ELF 48.00%, `#!/bin/sh` **0.00%**,
`/bin/true` 0.00%. A first draft reusing one path per arm read 93.75% for
the Go ELF at a lower load, which is the close's own 93-95% band.

So: **0 ETXTBSY for a shebang plant in 2,800 iterations across both shapes**,
against a positive control firing at 2-48% through the identical write. The
close's claim is confirmed.

Two refinements, neither of which changes the answer:

1. The rate tracks the SIZE of the text mapping being torn down, not just
   the format: a real, 43KB ELF fired 0 times in 2,800 iterations where a
   1.95MB Go ELF fired constantly. "It is an ELF" is not sufficient for the
   hazard.
2. The rate falls as the box gets busier (17.50% at loadavg 10, 2.00% at
   loadavg 17) — the write is descheduled long enough for the teardown to
   finish.

Spot-checked and confirmed: the three named hits plant shebang scripts
(`execwrite_test.go` plants `#!/bin/sh\necho first\n` then re-plants
`#!/bin/sh\necho 2\n`; `bdargvgate_qa_test.go`'s `mutant.sh` is a mutated
copy of `scripts/bd-argv-gate.sh`, which begins `#!/bin/sh`), and
`gates.go` emits exactly one shebang spelling, `#!/bin/sh`, at all five of
its sites — `PrePushHook` and `commitGuardHead` among them.

### The false sentence: four production sites DO plant genuinely compiled bytes

The close and this bead's fragment both say, in bold, "no `WriteExecutable`
call site in the tree writes genuinely compiled bytes", and name
`copyPromotedSet` as "the one production plant whose bytes do not come from
a template". A census of the 21 non-test call sites says otherwise — five
write bytes read off disk, and four of those read a compiled binary:

| site | bound by | what it plants |
|---|---|---|
| `cage.go:1697` | `os.ReadFile(gopath/bin/linux_<arch>/bd)` | the cross-compiled `bd` ELF `go install` left moments earlier |
| `upgrade.go:904` | `os.ReadFile(target)` | the live posse binary's own bytes, at `<target>.prev` |
| `upgradegate.go:288` | `os.ReadFile(p)` | a binary, at `upgradePrevPath(p)` |
| `upgradeverify.go:1084` | `os.ReadFile(prev)` | `.prev`'s bytes, at `<target>.rollback` |
| `landedcheck.go:1363` | `gitRaw(… cat-file blob …)` | a script blob — not compiled |

**The ANSWER is still none exposed**, and each for its own reason, which is
why this is a correction to the record and not a defect:

- `cage.go:1697`'s plant is a `GOOS=linux` cross-build staged beside a
  Dockerfile. It is exec'd inside the image, from another inode — never on
  the host that wrote it.
- `upgrade.go:904`'s `.prev` is not exec'd by anything in the tree: the
  prescribed rollback is `install -m 0755 <target>.prev <target>`, which
  READS it. (`upgradeSwap`'s own comment says "the rollback command execs
  `<target>.prev`" — that is about the golang/go#22315 direction, a write
  fd a fork inherits, not this bead's.)
- `upgradeverify.go:1084` and `upgradegate.go:365` plant a temp path and
  `os.Rename` it over the target — plant-to-temp-and-rename, which is the
  shape this bead PRESCRIBES, already in use at the one place a real binary
  is swapped.

What the false sentence costs is the forward-looking rule, which is written
as a hypothetical — "**if** this tree ever plants a real compiled binary
through `WriteExecutable` and re-plants it after exec'ing it" — about
something the tree already does at four sites. A reader who trusts the
complement will not look at them. The audit's key, "a path written once in a
process's life is not exposed whatever execs it", is sound for a test and is
the thing to re-examine for a production path: `.prev` and the cage's `bd`
are written once per RUN and live across runs, so their exposure, if it ever
arrives, arrives between invocations and not inside one.

RECORDED rather than filed: the fix changes a measurement record's prose and
neither what a process does nor what a pin proves (ADR 0006 §6).

## Method notes worth keeping

- A `tail -60` over a `go test` run hid two of the three FAIL lines on the
  pr6ms mutant and very nearly scored a killed mutant as a survivor. Grade
  per-test with `-v` and `grep '^--- '`, never a tail.
- The first ETXTBSY probe graded the NEXT iteration's plant as the same
  write as the rewrite, which inflated one arm from 17% to 94%. The two are
  different writes a loop-turn apart and belong in separate arms.
- The scratch-tree run that was to answer "does anything catch the meta
  guard mutant" was void: `git archive | tar -x` leaves no checkout, so five
  tree-wide pins red for want of a git repo — and the UNMUTATED control
  beside it red differently. The census answered it instead.
- An arm-1 test file reaching for an `posse_arm2` helper does not fail at
  its assertion; it fails `go vet` for the whole arm.
  `TestQAEverySuiteArmTypeChecks` caught that here and named the class.
