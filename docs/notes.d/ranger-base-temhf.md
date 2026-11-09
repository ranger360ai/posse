# Seven tests that asserted a host, and the seam that made them unfixable (ranger-base-temhf)

ci.yml red on `main` at 3f256af5, 2026-11-09T05:07:39Z
([run 42565490062](https://github.com/ranger360ai/posse/actions/runs/42565490062)):
all three **ubuntu** arms red, all three **macos** arms green. Seven tests,
none of them about the wall. Fixed 2026-11-09; the cause is c20771bb
(ranger-base-djl3c, ADR 0080 D18 — type the bwrap wall), the only commit in
`0ea8248f..3f256af5` that touches `internal/posse`.

## What broke

D18 made `cage: seatbelt` **two mechanisms**: `sandbox-exec -f <prof>` on
darwin, `3<args <abs bwrap> --args 3 --` on Linux (`RenderWall`, `wall.go`).
Before it, both launch call sites typed `SeatbeltPrefix` whatever the host, so
every test in the package could say "seatbelt" and mean `sandbox-exec` and be
right on the only host that ever ran the arm green.

The seven split three ways, and the third is the one worth remembering:

| | tests | what they assert |
|---|---|---|
| 1 | `TestQADeclaredStateDirReachesTheProfileALaunchRenders`, `TestQAUndeclaredStateDirIsNotGrantedByALaunch` | open `state/gates/<p>/seatbelt.sb` **by name** — a file no Linux launch writes |
| 2 | `TestSeatbeltProfileAndLaunch`, `TestSelfSandboxSkipsTheSeatbeltWrapAtLaunch` | grep the launch line for the literal `sandbox-exec`, over a line that carried the bwrap wrap verbatim |
| 3 | `TestQARelaunchRefusesAnEnvSetThatMovesTheCredentialDirAfterTheLaunch`, `TestExamplePIDsBindTheSeededSkill`, `TestQARenderedProfileForAClaudeLaunchCarriesNoOtherRuntimesStateDir` | launch a **claude** seat at `cage: seatbelt`, which ADR 0080 D19 REFUSES at the bwrap mechanism with `CLAUDE_CONFIG_DIR` unset |

Class 3 is not a stale string. `TestMain` sets `CLAUDE_CONFIG_DIR=""` for the
whole binary, deliberately and correctly — a surviving value sends every
locator at the OPERATOR's live store, and this box exports one from a
managed-settings file. On darwin that empty value costs nothing, because SBPL
grants claude's config FILE by path and has nothing to refuse. On Linux a
mount cannot grant a file, so D19 demands the variable be set and absolute and
refuses the launch when it is not — correctly, with the one line of host
procedure in the message. Three tests that merely needed a claude seat to come
up hit a refusal that was about the host and not about them.

## Why `make test` could not catch it, which is the actual finding

D18 shipped careful bwrap pins and **every one of them runs on darwin**:
`bwrapResolves`, `bwrapApplyRefusal` and `bwrapBin` are package vars so a pin
can answer the three host measurements, and `renderWallOn` takes the mechanism
as a parameter so the switch itself is drivable from either side.

It stops at the render. A **LAUNCH** asks `WallMechanism()`, which read
`runtime.GOOS` with no seam — so no pin could put `planLaunch` or
`CreateSession` behind the other mechanism. And the launch paths are precisely
where the two mechanisms differ in what they **refuse**: D19 is a refusal on
one host and silence on the other. `bwraplaunchline_qa_test.go`'s own comment
says D19's refusal "reaches the LAUNCH, which is the thing ranger-base-tc41o
could not pin" — and then pins it at `renderWallOn`, because there was no
other door.

`make test-linux` is the other reader and it does not run here either: Docker
is abandoned on this box by operator ruling (ranger-base-6mz7). So ci.yml's
ubuntu arms were the ONLY thing on the planet that ran these seven tests at
the Linux mechanism, and they run after the landing, on main, where a red
costs attribution rather than a rejected branch.

**`wallMechanismHost`** (`wall.go`) is the fourth such var and closes it:
`runtime.GOOS`, never written in production, written by
`wallMechanismForTest`. With it, `TestQALaunchAtTheOtherHostsWallMechanism`
and `TestQAClaudeAtTheBwrapMechanismIsRefusedAtTheLaunchAndNotOnDarwin`
(`walllaunchmechanism_qa_test.go`, arm 1) drive a real launch at **both**
mechanisms from either host: the line carries that mechanism's wrap in front
of the runtime command and names no other mechanism's binary, the artifact is
written and carries the launching runtime's declaration, and D19's refusal
reaches the launch on one mechanism and not the other.

MEASURED 2026-11-09, darwin/arm64, with `wallMechanismHost` pinned to `linux`
and `bwrapBin` stubbed: before the fix, six of the seven reproduce the CI
failures with the same messages (the seventh, parity's codex arm, is masked
behind an earlier failure here — see below); after it, all seven pass at
either mechanism. Mutation: pointing `RenderWall` at `WallSandboxExec`
unconditionally reds both new pins' bwrap subtests and neither darwin subtest.

## What the fixes are, and the one line they all follow

A test whose subject is **the wall** asks this host which mechanism it has and
reads that artifact (`wallWrapForTest`, `wall_helpers_test.go` — built from
`SeatbeltPrefix`/`BwrapPrefix`, so a pin compares the line against the one
spelling of the prefix and not a second copy). A test whose subject is **one
mechanism's own vocabulary** says so and skips elsewhere
(`sandboxExecWallOnly`). Neither spelling is a grep for a binary name.

- classes 1 and 2 take the artifact and the wrap from the host. The
  assertions did not need an arm each: a grant is a path in both renderings —
  a `(subpath "…")` line on one, a `--bind` argv pair on the other — so a
  declaration reaching the wall, and its absence, are readable as text either
  way.
- `TestQARenderedProfileForAClaudeLaunchCarriesNoOtherRuntimesStateDir`
  asserts SBPL lines, so it is darwin's reading and skips on a mount host.
  The bwrap reading of the same narrowing is `TestBwrapStateDirs*`, every arm
  of which already runs on darwin, so the skip gives up a duplicate.
- the other two of class 3 get `claudeConfigDirForTest` — the host procedure
  D19 itself prescribes, applied to the test box. **Per test and not in
  `TestMain`**, which is where it looks like it belongs:
  `ClaudeConfigDirIn` returns the VARIABLE whenever it is in force, whatever
  home it is handed, so a process-wide value would make every test that
  resolves a config dir under its own fixture home resolve to this one
  instead. It does not weaken the refusal
  `credentialdirrelaunch_qa_test.go` is about, either:
  `credentialDirEnvSetRefusal` reads the env set's NAMES and never a value,
  so an env set exporting the variable is refused whatever the launcher
  resolved.

## Two things found while reproducing, neither reported by CI

**The ubuntu run created `/opt/shared` on the runner.** `parity_test`'s
fixture PID declared `writable: [scratch, /opt/shared]`. On darwin that is
free — SBPL grants a path whether or not it is there, and nothing writes it.
At the bwrap mechanism it is not free: bwrap binds only a source that exists,
so the launcher `mkdir -p`s every missing write grant first, outside the
namespace and as the seat's user (`RenderBwrap`, prep shape 1). On this box
that launch refuses with `mkdir /opt/shared: permission denied`; on the runner
`/opt` is writable, so it succeeded **silently** and the test went on to red
three other lines. The fixture names a path under its own tempdir now — still
absolute, still outside the session tree, which is all the arm was ever about.
The general shape is worth carrying into D21: at this mechanism an absolute
`writable:` in a PID is a directory posse will CREATE on the host.

**A bwrap launch creates `<cwd>/.beads`, which changes a later launch's
reachability row.** The same prep makes the store of record, because
`SeatbeltWritable` grants it. In `TestSeatbeltProfileAndLaunch` the grok
seatbelt launch therefore materialized `<repo>/.beads`, and the codex@shims
launch three arms later — same `repo`, a PID that denies bare `Edit` and
`Write`, so codex renders `-s read-only` — then had a REAL store behind a
read-only sandbox where it previously had none, so ADR 0013 §4's row went from
"nothing to reach" to unrealized and refused the launch. That arm's subject is
the cage shortfall being a declared difference rather than a degradation, so
it gets a tree of its own now. The row over a real store is its own pin; what
this cost was a third failure inside one test on one platform, which is how it
looked like the wall was broken three ways.

## Where the gate's reader sits

Nothing in this bead changes that: a darwin green plus a darwin-driven Linux
arm is not ci.yml's ubuntu arms, and the two new pins do not claim to be.
`SeatbeltWritable` still reads `runtime.GOOS` for the home-cache grants and
for `/private/tmp`, so a mechanism-pinned render is not a byte-exact Linux
render — which paths the wall covers is `homeCacheGrants`' own pin, and which
wall a launch puts up is the new one. What moved is that the class of failure
that cost this red — a launch assertion that is secretly about one host — is
now reachable before the landing instead of only after it.
