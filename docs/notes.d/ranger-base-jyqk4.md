# Four macos-only red families, and the one thing all four have in common (ranger-base-jyqk4)

2026-11-21, from a linux seat (Linux 7.2.15 x86_64, go1.27.3, 16 cores). No
darwin box was touched here. The reds were READ off ci.yml's macos legs and
the windows bisected by reading the landings in them — which is this bead's
whole shape, and ranger-base-m8msi's and ranger-base-i36bf's before it.

MEASURED, macos-latest (image macos-26-arm64 20261117.0516.1, Darwin 25.6.0
arm64, git 2.56.1, go1.26.10). The bead's three runs — 44921131540
(22690b7a), 44926213762 (56854819), 44927635225 (6557d59b) — plus run
**44947866559 (acdaf81f)**, read here and the first leg AFTER
ranger-base-i36bf's git-shim fix landed. Four legs, identical.

## the thing all four families have in common

**Every one of them is a pin whose only venue is a macos leg, asserting a
decision some LINUX-green landing had already changed.** Not one of them is
a defect in the code it covers; all four are stale expectations, and in all
four cases the landing that staled them could not have seen it:

| family | staled by | the pin's venue |
|---|---|---|
| the arm-2 wall-check table (9 tests) | ADR 0097 D2's own host gate | a host whose `WallMechanism()` is bwrap |
| the carve-out control (2 rows) | ranger-base-kfrpv, 1fb4fbff | `sbSkipUnlessSandboxable` — darwin only |
| the instance-state pin | ranger-base-6akzw, 3aae3c5a | `sbSkipUnlessSandboxable` — darwin only |
| `TestSeatbeltProfileAndLaunch`'s wrap | ranger-base-olr0g (+ 5blmt) | the sandbox-exec arm of a mechanism switch |

So the class is "a pin one platform cannot run, over a decision the other
platform's suite ratifies", and a tree with one always-on linux box and a
macos leg nobody reads produces it on a schedule. Three of the four were red
for days to a week; the fourth, below, since the step was written.

## 1 — the arm-2 wall-check table: nine parallel arms measuring the one line that declines them

ADR 0097 D2's second question is "is this host's wall a bwrap", and if it is
not, **the step does not exist** (ADR 0096: no darwin change). `wall()` read
`WallMechanism()` directly, and `wckFixture` stubbed the cage but not the
mechanism — so on a darwin host every arm of D5's table got
`wall check not run — this host realizes cage seatbelt with sandbox-exec …`
and measured nothing it was written for.

Red on **every macos arm-2 leg since the step landed**, green on every ubuntu
one. The bead named seven tests and listed eight; the real fail list is
**nine**, because `TestEveryVerdictLineNamesTheLauncherThatRanTheCheck`'s
three wall arms (landedlauncher_test.go) go through the same fixture and were
not in the bead.

REPRODUCED HERE, which is what made this family cheap: the only darwin fact
in it is `WallMechanism()`, so a six-line untracked `func init() {
wallMechanismHost = "darwin" }` in the package reproduces the macos arm-2
fail list exactly — the same nine tests, the same subtests, including the
log's truncated `…/the_judge` row (an apostrophe in
`the_judge's_cage_refuses`).

The fix is a seam, `landedChecker.wallMechFn`, nil meaning the real one
(`wallMechOf`) — `graceOf`'s and `ceilingOf`'s shape, and the field comment
says why it is not `wallMechanismForTest`: that writes a package var every
launch path reads, so an arm driven by it must be SERIAL, and these eight
are parallel and land real commits on a real main. `wckFixture` sets it, so
an arm written tomorrow cannot forget it.

**And one arm clears it to nil on purpose.**
`TestAHostWhoseWallIsNotABwrapRunsNoVenueAndCountsNoSilence` keeps
`wallMechanismForTest(t, "darwin")` and stays serial, which makes it both the
reading of D2's host gate and the pin on "nil means the real one". MEASURED
against a mutant `wallMechOf` returning `WallBwrap` unconditionally: that arm
reds, `the wall check RAN on a host whose wall is sandbox-exec`. Without it
a mis-wired seam would leave the whole table green on a host where the step
cannot run, which is the hole this fix would otherwise dig one layer down.

## 2 — the carve-out control: a probe whose deny went away with its grant

`<store>/.git` left every tier's write set at **ranger-base-kfrpv**
(1fb4fbff, 2026-11-20) — nothing in a seat commits in the store repo, and
the grant went to zero rather than being narrowed. That bead updated the two
pins in `seatbeltcarveout_qa_test.go` it could SEE red, asserting over the
SET that `<store>/.git`, `/hooks` and `/objects` are now in neither the grant
nor the deny. It could not see the third pin in the same file, because
`TestQACarveOutRefusesUnderSandboxExecAndTheControlDoesNot` is gated by
`sbSkipUnlessSandboxable` and never runs on linux at all. Its two rows read,
on every macos leg since:

```
/plant_an_L3_hook_in_the_store_of_record: the CONTROL refused it too — the probe proves nothing about the deny
/the_record_stage:_the_store's_git:       REFUSED under the carve-out, want ALLOWED
```

Both are correct behaviour. `want bool` had no spelling for it, so the probe
table takes a third answer, `sbUngranted`: refused with the carve-out and
without it. **With a third profile as its control** —
`sbStoreGitGranted`, the control set plus `beadsGitDirs(<store>/.beads)`,
which must ALLOW the same command. That is the same idiom
`storerepogitwalllive_test.go` uses at the bwrap mechanism ("THE CONTROL IS
THE PRE-FIX RENDER"), and without it "refused twice" would also be what a
malformed command or a destroyed fixture reads like.

## 3 — the instance-state pin: a control that could no longer be built

`TestQAKernelGrantsOwnInstanceStateAndRefusesTheOthers` asserted that a
session under `RHQ_HOME=<home2>` writes its own `state/`. **ranger-base-6akzw**
(3aae3c5a) took `<home>/state` out of the caged write set outright — it holds
what the launcher and the dispatcher write and later run or trust outside
every wall, and no member of it ever had a measured seat writer. So on darwin
the pin read both halves of its own failure at once:

```
:46 a session under <tmp>/home2 cannot write its own state dir <tmp>/home2/state
:63 control built nothing: <tmp>/home2/state is in no grant to swap
```

Rewritten over the decision as it stands: the own-state probe is a REFUSAL
now, the cross-instance probe is unchanged, and each gets its own pre-fix
control — 6akzw's grant put back for the first, rangerhq-qfzr's wrong-home
grant for the second, both built by `sbInstanceStateProfile`, which Fatalfs
if the "extra" grant turns out to be in the shipped set already.

**The cost check moved**, and the measurement is why. A seat's own gates dir
is granted and then DENIED by the carve-out — it cannot rewrite even its own
L1 shim — and only `refusals.log` and `shell.log` come back over that deny
through `Keep`. So the "the profile reaches this home at all" witness is an
append to `refusals.log`, not a `touch` in the gates dir, which would have
failed over the deny and looked like a dead fixture.

## 4 — `TestSeatbeltProfileAndLaunch`: a prefix in front of a prefix

SBPL has no env of its own, so every variable the launcher hands a darwin
seat rides in front of `sandbox-exec` as `/usr/bin/env K=V …` — the seat's
own build and package cache (**ranger-base-olr0g**) and, for a seat the
seat-store wall applies to, its two object-store variables (5blmt).
`wall.go`'s `WallSandboxExec` arm puts both on `WallRender.Prefix`; the test
helper `wallWrapForTest` answered `SeatbeltPrefix` alone. The real line is

```
GATES /usr/bin/env 'GOCACHE=…/go-build' 'npm_config_cache=…' 'POSSE_SEAT_CACHE=…' sandbox-exec -f '…/seatbelt.sb' grok -m 'grok-4.6' …
```

and the pin asks for `GATES <wrap>grok` — anchored at `GATES `, which is
exactly what the env prefix sits between. On ubuntu those variables go in
bwrap's argv instead, so the ubuntu arms stayed green over the same
assertion.

`wallLaunchWrapForTest` builds the env prefix now, from the same three
producers the render calls. It takes an `*AgentFile` rather than a seat name,
because `seatStoreEnv` is nil for a PID that denies file writes
(`seatbeltSeatStore` reads `ag.Deny`) — the `security` PID in this fixture is
one, which is why the real line above carries three variables and not five.

Also reproduced locally under the same `wallMechanismHost = "darwin"` pin,
red before and green after; and `TestQALaunchAtTheOtherHostsWallMechanism`
drives the new arm on linux, where it was already green — its `Contains` is
not anchored at the gate prefix, so it is a compile-and-render reader of this
arm and not a door on it. A linux door for the class would have to assert
what sits between `GatePrefix` and the wall prefix, which three producers
decide; it is not in this bead.

## what a linux seat could and could not verify

| | verified here | the reading still owed |
|---|---|---|
| family 1 | the full fail list reproduced and fixed under a `wallMechanismHost = "darwin"` pin, plus a mutant | — |
| family 4 | same pin, red before / green after; both mechanisms green on linux | — |
| family 2 | the set arithmetic: both rows in no shipped grant, in no carve deny, and in the pre-kfrpv grant — i.e. (refused, refused, allowed) | sandbox-exec's own verdict |
| family 3 | every assertion of the rewritten pin as set membership, both controls flipping | sandbox-exec's own verdict |

Families 2 and 3 execute `sandbox-exec`, which a linux box has not got, so
their kernel reading is the next macos leg's — the same shape
ranger-base-i36bf closed under, and the reason this fragment states the set
arithmetic rather than claiming a pass.

## not closed here

Four MORE red families on the same leg, none of them named by any bead and
none of them this one's: **ranger-base-5oiz4**, filed with its windows for
the same reason this bead carried four — so the next reader of a CI log does
not re-derive them. Two are ubuntu-only, two macos; one of the two macos ones
is a single landing wide.

`TestQAMirrorIsAFourthForcePushSpellingThatOnlyTheVerbRuleCloses` — the third
done-condition ranger-base-i36bf left for the next leg — is **green** on
macos arm 3 of run 44947866559. And ranger-base-i36bf was NOT the carve-out
control's cause: both rows are still red on that same post-fix leg, which is
what ruled the suspicion out and sent family 2 to kfrpv.
