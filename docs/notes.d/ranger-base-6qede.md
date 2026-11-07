# Verifying four closes: two claimed diagnostics, neither reachable (ranger-base-6qede)

Verification of `ranger-base-74tkp`, `ranger-base-0g6zm`, `ranger-base-m928w`
and `ranger-base-t5ein`, 2026-11-07, darwin 25.6.0, from a `cage: seatbelt`
seat, at `main` = `847fbe24`. All four closes meet their own DONE WHEN. Two
carry an escape, and both are the same shape: **a diagnostic the close names
as existing, which nothing can reach.**

## 1. ranger-base-74tkp — the discriminator is not in the pass output

The close deliberately leaves the live Rex cause unidentified, and says the
route back is still open:

> If it is ever worth knowing, the discriminator is in the pass output: a leg
> that ran out names its own deadline (`no idle|done|blocked state in <block>
> within <d>`), and a leg that could not read names the rex call.
> — `docs/notes.d/ranger-base-74tkp.md:116-118`

MEASURED FALSE 2026-11-07, over the close's OWN fixture (`waitLegGather`,
`waitlegpace_qa_test.go`), driving `gather` on both occupants with
`ProgressEvery: 0` so every leg prints:

```
=== rex pass output ===                     === herdr pass output ===
◷ a-1  still working after 0s — waiting again   (identical, 5 lines)
◷ a-1  still working after 1s — waiting again
◷ a-1  still working after 1s — waiting again
◷ a-1  still working after 2s — waiting again
◷ a-1  still working in ranger-qa-a-1 after 2s — claim kept, not judged this pass

contains "no idle|done|blocked state in" : false    (both occupants)
contains "timed out waiting for agent status" : false
```

The rex arm stages the first of the two early returns the close enumerates
(`RexMux.Wait` past its own deadline) and its deadline message never prints.
**Neither half can print in the regime that produces the busy-wait**, and that
is the sharper statement: the loop only goes round again when
`statusAfterTimeout` answers `working`, which means `HB.Resolve` succeeded. The
"names the rex call" half reaches the pass output ONLY through the switch's
`default` arm (`statusPhrase`, `dispatch.go:4018`) — which fires when
`statusAfterTimeout` ALSO fails, and which `return`s. So a pass that prints the
rex call has run one leg, not 295; a pass that runs 295 prints neither.

**Why.** On the `delivered` path a non-nil leg error has **zero sinks**. Both
guards that consume it are `&& !p.delivered` (`dispatch.go:3951`, `:3967`), and
codex is `prompt: argv`, so `delivered` is true for the only runtime this bead
is about (ADR 0013 §2). Between `if r.err == nil` (`:3929`) and the status
switch nothing prints, logs or records it: no `printf`, no `logReading`, no
`logPromptHandBack`.

**What it costs, and it is the bead's own symptom.** Before this close the leg
error was invisible but a flood of progress lines was not — 295 lines in
~3m30s is what got the bead filed. `DefaultProgressEvery` is a minute now, 84×
quieter, so a Rex seat whose every leg fails for a whole turn prints exactly
what a healthy one prints: one line a minute, no error, claim kept. The signal
that exposed the class is gone and nothing replaced it. The pacing itself is
correct and pinned; this is the diagnostic that was traded for it.

### The pacing, verified

Three pins, all green, each mutant redding its own and leaving the others
green (MEASURED 2026-11-07, 2s windows, ceiling 7):

| mutant | poll pin | progress pin | numbers pin |
|---|---|---|---|
| `rewait` takes no floor | **FAIL** 40 herdr / 21 rex | pass | pass |
| `sayWorking` always true | pass | **FAIL** 46 herdr / 18 rex | pass |
| `legAt` stamped at the rewait CALL, not the leg START | **FAIL** 8 herdr | pass | pass |

The third is a shape the close's own table does not list (it names "legAt never
stamped forward", which collapses to the first). It is caught on every run of
three, always by the herdr arm at 8 against 7; the rex arm reported 8 once in
three. Worth knowing for whoever next touches that ceiling: against this shape
the rex arm has one leg of headroom, and since the assertion is a ceiling and
load only lowers the count, tightening the `+2` slack is free.

Production wiring checked rather than assumed: one `Dispatcher` literal in the
tree (inside `NewDispatcher`), both production call sites go through it
(`cmd/posse/main.go:937`, `cmd/posse/cockpit.go:841`), so a real pass carries
`DefaultLegFloor`/`DefaultProgressEvery`. On herdr the floor is a floor under
the INTERVAL and a herdr leg is `PromptWaitMS` of it, so the remainder is
negative and nothing sleeps — byte-identical, as claimed.

## 2. ranger-base-m928w — the "no cage" guard is vacuous

VERIFIED LIVE from a caged seat, which the close says it could not do
("NOT verified live from a caged seat"). This seat is `RHQ_CAGE=seatbelt`
and stages the bead's exact condition with no fixture at all: a listing of
codex's runtime home succeeds, and a stat of the credential file inside it
answers `Operation not permitted`, because posse's own L2 profile denies that
file (`credentialReadDenyLiterals`, ADR 0019 D2 store 3). Paths restated rather
than spelled, for the reason the close's own follow-up commit was needed
(NOTES.md "Privacy model", ADR 0024 D1). `posse runtime check codex` now reads:

> state unknown: cannot read <codex's credential file> (operation not
> permitted) — so this cannot tell whether codex has a credential … this stat
> never reached an answer. … This reader is itself a caged seat —
> RHQ_CAGE=seatbelt, persona <this seat> — so the refused read is a fact about
> THIS SEAT and not about the box

All three DONE WHEN met on the box, not only in a fixture.

**The escape.** `probeCageNote` stands on two facts and the close names both:
`fs.ErrPermission` AND an `RHQ_CAGE` in posse's own environment. Mutating them
separately (the standing reflex: mutate the conditions the close did not):

| mutant | result |
|---|---|
| drop the `RHQ_CAGE` conjunct | **FAIL** — pin 1's cage arm |
| drop `fs.ErrPermission` from `probeCageNote` | **all three pins GREEN** |
| `ENOTDIR` no longer absence | **FAIL** — pin 2 |
| `ENOENT` no longer absence | **FAIL** — pin 1's control arm |

`TestQAProbeReadsENOTDIRAsAbsenceAndClaimsNoCage` is named for that conjunct
and its doc says "a non-permission failure is diagnosed as neither a cage nor
an unknown", but its guard

```go
if strings.Contains(sil.Why, "This reader is itself a caged seat") {
```

can never fire: the fixture stages ENOTDIR, `probeAbsent` returns **true**, the
probe takes the absence arm and never calls `unreadableSilence` or
`probeCageNote` at all. Every non-permission errno that pin can reach is one
already classified as absence, so the half of its name that says "no cage" is
unreachable.

**Fixed here**, because a missing pin over correct production code is a test
addition: `TestQANonPermissionErrnoIsUnknownAndClaimsNoCage` stages ELOOP — a
self-referential symlink as `CODEX_HOME`, so `stat` answers "too many levels of
symbolic links", which is neither absence nor permission and is therefore the
only shape that reaches `probeCageNote` on a non-permission errno. Control
MEASURED both ways: green on the fixed tree, and under the dropped-conjunct
mutant it reds naming the harm while the ENOTDIR pin stays green.

The census the bead asked for is complete and independently confirmed: four
`Silence` producers in `interstitial.go` all branch on `probeAbsent` now, bob's
two read the launch line or nothing on purpose, and claude's two are the
inverted defect on a file a cage CAN read — claude's own top-level config is
readable from this seat, confirmed by stat, so the filing of
`ranger-base-c2k6z` rather than a fix here is right.

### One sentence over-scoped

The ADR amendment grounds the no-refuse verdict partly on a backstop:

> and mzmv's backstop holds, since herdr's `signin_menu` / `signin_api_key`
> rules name that screen `blocked` too
> — `docs/adr/0013-runtime-dispatch-contract.md:501-502`

That holds on herdr and not on a `multiplexer: rex` instance, where ADR 0076
states there are **no screen rules** (`:107`) — and Rex is the instance class
`ranger-base-8gh58` just admitted codex on. The verdict still stands: codex on
Rex is `prompt: argv`, so nothing is ever typed into the sign-in menu and the
`danger:` the row declares cannot fire; what a Rex seat gets instead of
`⛔ intervene` is `◷ claim kept, not judged`. Prose, with no code fix available,
since §3 forbids the rule the sentence leans on.

## 3. ranger-base-0g6zm — verified, nothing dropped

The replay landed as `e760b3e8` and `29af1313`. Checked independently of the
close's own account:

- **All three `equivalentOnBase` arms pair both commits**, so `blockStillStands`
  returns false (`landsweep.go:483`) and no later pass re-files. `git cherry`
  gives `-` for `fae7e566` and `+` for `68266f54`; the `-x` trailer finds
  `29af1313` and `e760b3e8`; `replayKey` (author email, AUTHOR date, subject)
  pairs the same two. `measuredOnBase` is false, so the branch is correctly
  NOT auto-deletable and waits for the operator — it still exists at
  `fae7e566`. The `refs/posse/merge-blocked/…` pin is gone, which is the
  launcher dropping it on the close.
- **Nothing was lost in the merge.** Every line the work's two commits ADDED is
  present on `main` — 246 added lines across the four non-ADR files main also
  touched (`herdrback.go` 81, `dispatch.go` 24, `runtime.go` 140, the generated
  notes index 1), 0 missing; the ADR is the bullet below. The one apparent resurrection is an indentation change, not a
  revert: the work re-indented one `if o.Bead != "" || o.Recreate {` by a tab,
  and all four occurrences are present either way. 10 of the work's 15 paths are
  byte-identical to the original commit; the 5 that differ are exactly the files
  later landings touched (`dispatch.go`, `runtime.go`, `herdrback.go`, the ADR,
  the generated notes index).
- **The one conflict, re-derived from the three blobs.** `difflib` over the ADR
  Status line gives exactly two disjoint insertions and nothing else: the work's
  tty-probe arm at offset 919, and `ranger-base-778y0`'s Amendment 4 clause at
  the tail. Round trip holds byte for byte — `main`'s line minus 778y0's clause
  IS the work's line.
- Commit stats reproduce the originals exactly: 15 files +1715/-62, then 2 files
  +166/-10, with author dates and subjects preserved.

## 4. ranger-base-t5ein — verified, read off the store

- Both measurement beads are CLOSED with the seat's OWN comment (`codexrex` on
  `ranger-base-7vh7y`, `codexherdr` on `ranger-base-w4enh`), and each bead's
  `closed_at` is the same minute as that comment. Both commits are ancestors of
  `main` (`b14420ef`, `3bc1a6db`) and `git log --grep` finds each under its own
  id.
- `Record: RecordTrusted` with a `RecordWhy` that carries the caution the bead
  asked for — both PIDs had a `bd close` instruction in the body. Two mutants,
  each redding `TestBuiltinContractDeclarations` alone: reverting to
  `RecordUntrusted` → "codex record: untrusted want trusted"; blanking
  `RecordWhy` → "codex is trusted with no measurement named". So the promotion
  is pinned and a promotion without provenance cannot land.
- The re-aimed fixtures are the right shape and I confirmed it the hard way:
  under the revert-to-untrusted mutant the four §4 pins and the arm-2 pin stay
  GREEN, because they now declare `runtimes/unmeasured.yaml` instead of naming a
  built-in. They survive the next promotion too.
- Both claims I could falsify, checked: `RecordTrust()` has exactly two
  consumers (`dispatch.go:4357`, `runtimecheck.go:459`), both name-free; and bob
  really cannot be dispatched (`runtime.go:1958` — herdr 0.8.2 has no `bob`
  kind, so a dispatched bob session is `agent_not_found`), so "no built-in is
  both dispatchable and untrusted" is true and the fixtures were moved on a
  sound premise. The arm-3 `runtimefields` audit needed no change and passes.

## What the suite says about the merged tree

Worth recording because nobody had run it: each of the four closes ran `make
test` green on its OWN pre-merge tree, and the launcher then rebased them into
a different one. `ranger-base-t5ein`'s close is the clearest case — its commit
`af4b8483` carries tree `36074639`, and what landed as `847fbe24` carries tree
`2e27b0e0`. So the tree on `main` had never been suited.

MEASURED 2026-11-07, `make test` over `847fbe24` plus this bead's own test
addition, exit read from its own file and not from an `echo`: exit 0, zero FAIL,
all three arms — arm 1 `internal/posse` 501.1s + `internal/treepins` 491.3s +
`cmd/posse` 210.3s, arm 2 414.4s, arm 3 334.3s — silent-revert audit 2300
commits, 0 untriaged, `tree-check` run as its prerequisite.
`ranger-base-6mpke`'s accepted fork/flock flake did not fire.

An earlier run over the same commit WITHOUT the test addition also read exit 0
and zero FAIL (arm 1 542.9s + 586.5s + 329.3s, arm 2 343.9s, arm 3 456.4s), but
see the method note below before quoting it: its tree moved while it ran.

## Two method notes, both of them mine

**The mutation window.** The first `make test` above was launched and then I ran
mutation checks that edited `dispatch.go` and `interstitial.go` while it was
running. That is a tree moving under a suite, and the result could have been red
for my reason. It cannot have been falsely GREEN — every mutant here only
weakens production code, which reds pins rather than hiding failures, and go
keys its build cache by content so no mutated object can be served for clean
source — but the honest reading is that a run like that proves less than it
looks like it proves. The figures recorded above are from the clean re-run, with
no edit in flight.

**Mutate the conditions, not only the switch.** The reflex that found the
`probeCageNote` gap is the one `ranger-base-n3s4s` wrote down: a close
mutation-checks the switch its fix turns on and leaves the CONJUNCTS that bound
the fix unmeasured. Both closes here had a thorough mutation table; both tables
tested the branch and not its guards. `ranger-base-74tkp`'s missed a fourth leg
shape (caught anyway); `ranger-base-m928w`'s missed one of the two facts its own
close names as the function's whole safety property (not caught, and now pinned).
