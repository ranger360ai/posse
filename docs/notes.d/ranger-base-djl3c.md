# The bwrap launch line, as built: four surfaces, not two (ranger-base-djl3c)

ADR 0080 D18 — the launch line, identical on herdr and Rex panes — built
2026-11-08. `RenderBwrap` and `BwrapPrefix` in `internal/posse/bwrap.go`'s D18
section; `WallRender`, `RenderWall`, `WallPrefix` and `WallReport` in
`internal/posse/wall.go`; pinned by `internal/posse/bwraplaunchline_qa_test.go`
(arm 1, darwin). The line is D18's letter unchanged:

    3<'<state>/gates/<p>/bwrap.args' '<abs bwrap>' --args 3 -- <runtime cmd>

What follows is what the build had to settle, in descending order of how much
it matters to whoever picks up D21 or reads a parity row on the D1 host.

## 1. The letter named two call sites. There were four surfaces

D18 says "both call sites that prefix today (herdrback.go, the create and
relaunch paths) change together, through one `WallPrefix(rt)` function". Both
did. But `SeatbeltPrefix`'s slot is not the only place this build decides what
the L2 wall IS, and the other two were silent in different directions.

**`reachability.go` (the third).** ADR 0013 §4's record-reach row judges the
cage BEHAVIORALLY, by running a create-and-remove inside each target under the
artifact the launch will run. Its seatbelt arm opens with
`if !SeatbeltAvailable() { return }` — darwin and `sandbox-exec` on PATH — so
the day the tier became available on Linux, every bwrap seat would have got
**no row at all**: not a pass, not a finding, not the file's own third answer
("NOT MEASURED"), but an absence. That is the exact shape the file's own
comment refuses — "a check that measured nothing and a check that measured a
pass must not print the same line" — and it would have landed on the one gate
whose failure mode is that nothing observes it (a caged session that cannot
reach the bead store claims, comments and closes nothing while parity prints
all gates realized).

So the arm is split in two, keyed on `WallMechanism()`, and the darwin arm is
byte-identical to what it was. The bwrap arm is shaped measurement for
measurement: `bwrapApplyRefusal()` where the other asks
`sandboxApplyRefusal()`, `RenderWall` where it renders a profile,
`wallReachRow` where it calls `seatbeltReachRow`. Two arms and not one
parameterized arm because every one of those measurements has a different
NAME at this mechanism, and the row an operator reads has to say which thing
to go and fix — a profile's trailing deny and a mount the kernel locked are
not the same errand.

One difference is deliberate and is an improvement: the bwrap probe runs the
**typed prefix** under `/bin/sh`, not a hand-built argv with the args file on
an fd. The wall at this mechanism is spelled as a redirection before the
command name, and a probe that exec'd `bwrap` directly would measure the
MOUNTS and say nothing about the line posse types. The darwin arm execs
`sandbox-exec` directly and was left alone, because changing a load-bearing
probe to gain symmetry is not a trade this bead was asked to make.

**`posse gates` (the fourth).** `SeatbeltReport` prints SBPL's vocabulary —
`(subpath …)`, the sibling regex, create-only ref dirs, the trailing deny that
beats every grant above it. On a bwrap host those are four statements the
mechanism cannot make. `WallReport` dispatches; `BwrapReport` prints the same
MARKERS (`w`, `x`, `r`) over the same lists, drops `~`, reuses `+` for the one
thing this mechanism has and seatbelt does not (D20's keep file, which binds
nothing), and adds `!` for the prep — the writes posse makes on the HOST, in
the seat's name, which an operator can read nowhere else. `SeatbeltReport`'s
signature is untouched and its nine test call sites with it; the one
production call site in `cmd/posse/main.go` now passes the runtime, because
the bwrap arm needs `rt.Name` (D19's substitution is keyed on it).

The constitution block — ADR 0015 §2's claim checked against the grant set
rather than restated — is now `reportConstitutionGrants`, shared. It is the
one part of a report that HAD to be shared: the rest is a mechanism's own
spelling, but that block is a property of the grant SET, and the set is the
same list on both. Two copies would be two deny-lists to keep complete, which
is the exact failure the block exists to replace.

## 2. `WallRender` is one struct because the three cannot disagree

`RenderWall` returns mechanism · artifact · prefix together. The prefix NAMES
the artifact — `-f <prof>` on one mechanism, `3<<args>` on the other — so a
caller that rendered one and reported the other prints a wall nobody is
behind. Parity's reach row is exactly such a caller: its detail line says
which artifact it judged, and it judges by running the prefix. The pin is
`strings.Contains(w.Prefix, shellQuote(w.Artifact))` for both mechanisms.

`renderWallOn(mech, …)` is the parameterized half, following this ADR's own
rule for every host-shaped decision (`wallMechanismOn`,
`seatbeltTierAvailableOn`, `credentialReadDenyLiterals`). It is the whole
reason the bwrap arm is readable from darwin.

## 3. The third question stays a separate predicate, and the hazard moved

`wallMechanismRendered` now admits both mechanisms, which is the one line D16's
as-built note said D18 would flip. It would have been tempting to delete it
instead: with both mechanisms rendered, the conjunct in
`seatbeltTierProvidedOn` is true wherever availability is.

It stays, and `TestQAWallTableClaimsNoTierThisBuildCannotType` now holds the
biconditional rather than the hold: **every mechanism `wallMechanismOn` can
NAME is one `RenderWall` has an arm for.** The hazard ranger-base-pl3ck found
has not gone away, it has moved — from "bwrap is available and untyped" to
"the next mechanism is named and untyped" — and it is the same hazard with the
same silence: `AvailableCages` claiming a tier, parity reporting L2 rules
realized, and a launch line typing a binary the host does not have. A
Landlock ruleset or a FreeBSD jail added as a name in `wallMechanismOn` reds
that arm now, where before D18 nothing would have asked.

## 4. D17's residual, closed the way qoamy §4 recommended

`SeatbeltWritable` carried four darwin paths as unconditional literals.
`docs/notes.d/ranger-base-qoamy.md` §4 named the choice and left it to this
bead: narrow them per GOOS, or have the launcher refuse a prep step it cannot
make. The first, as recommended — the second alone leaves the wall announcing
grants it does not have.

- `homeCacheGrants(goos)` is the three under the home. `Library/Caches` and
  `Library/Logs` are darwin's own and need no Linux equivalent (`.cache`
  already is it); the Go toolchain's telemetry counters move to their
  `os.UserConfigDir()` spelling, `.config/go/telemetry/local`, and the REASON
  travels with them — `local` and never its parent, because `telemetry/mode`
  beside it is the only thing that decides whether the box UPLOADS those
  counters (crew guardrail 4).
- `/private/tmp` is guarded on darwin. The cost of leaving it was not a junk
  directory but a **refused launch**: on Linux there is no `/private`, so the
  prep is a `mkdir -p` at the filesystem root that an unprivileged launcher
  cannot make.

**The darwin render is byte-identical**, and that is measurable rather than
argued: `add` absResolves, `/tmp` IS `/private/tmp` on darwin, and
`dedupeStrings` folded the two already — so the fourth literal was redundant
on the host it was written for. The darwin list's ORDER is kept as it was,
because a grant set is rendered in list order into the allow block and
reordering it would rewrite every seat's `seatbelt.sb` for nothing.

## 5. Two small things with victims

- **A prep `touch` never truncates.** The launcher creates the session's two
  audit logs so the Keep binds can go back over the deny the carve-out made.
  `os.Create` is the obvious spelling and it would empty a live seat's
  `refusals.log` at every relaunch — and the only reader who would notice is
  whoever goes looking for a refusal that was recorded and is gone. The open
  carries no `O_TRUNC`, and the pin writes a refusal into the log, renders
  twice, and reads it back.
- **An unknown `PrepKind` refuses.** A step posse cannot make is a grant the
  argv names and the host does not have, and bwrap's own failure for that is
  `Can't find source path` with no reason in it. The refusal is in
  `runBwrapPrep`, where the reason still exists.

## 6. `bwrapBin` is a variable, and the census pin that caught the rename

`bwrapBin` joins `bwrapResolves` and `bwrapApplyRefusal` as a package
variable, for their reason: without it the bwrap arm of `RenderWall` cannot be
reached on the box that builds it, and an untested arm that puts up an L2
write wall is the one answer nobody may be handed on a guess.

And one pin earned its keep without being aimed at this bead.
`TestQAEveryLaunchPathThatRendersASeatbeltRefusesACredentialDirEnvSet` is a
module-wide AST census keyed on the call name `RenderSeatbelt`; the moment
both launch sites moved to `WallPrefix` it found **zero** launch paths and
failed on its own floor ("fewer than two is watching a path that has left").
It keys on `cdmWallRenders` now — `WallPrefix`, `RenderWall`,
`RenderSeatbelt`, `RenderBwrap` — every entry point and not the top one,
because a path reaching for a mechanism's own renderer directly owes the same
refusal. That floor is the only reason the census did not quietly stop
checking anything.

## What is NOT measured here

The real binary. Every arm runs on darwin, which is what D17's purity buys,
and the SHELL half of this line is measured there against a stand-in `bwrap`
that copies fd 3 and execs its own tail: the redirection, the quoting of both
paths (the fixture puts a space in every directory), the fd number and the
`--` separator. What the D1 host adds is the real `bwrap`'s acceptance of the
argv and the kernel's honouring of the mounts — probe T7
(`docs/adr/0080-bwrap-tier.probe.sh`) and
`internal/posse/bwrapgitrefslive_test.go`, which skips off Linux.
