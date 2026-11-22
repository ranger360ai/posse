# Five macos arm-1 reds, and the three darwin path shapes a linux box can plant (ranger-base-zbmxc)

2026-11-21, from a linux seat (Linux 7.2.15 x86_64, go1.27.3, 16 cores). No
darwin box was touched here. The reds were READ off ci.yml's macos arm 1 —
run 44974829306 (b2509fba), job 137062263947, "test (macos-latest, 1)" — and
then **reproduced on this box, line for line, by planting the shape**. That
is the part of this bead worth keeping: four of the five rows are not
"darwin-only" in any way a linux suite cannot reach, and the fifth is not a
path spelling at all.

The bead filed eight rows. Three were already fixed at 7863b9a3
(ranger-base-aurb8, a production raw/resolved key — the comparison, not the
pins). These five are the rest.

## the three shapes, and what each one is worth

A darwin red of this family is almost never "darwin"; it is one of three
facts about paths, and **every one of them can be planted here**:

| shape | darwin's spelling | plant it with | what it catches |
|---|---|---|---|
| a temp root behind a symlink | `/var/folders/…` → `/private/var/folders/…` | `TMPDIR=<link to a real dir>` | a raw `t.TempDir()` compared against anything resolved |
| a temp root that is GRANTED but is not `/tmp` | `$TMPDIR`, which `seatbeltTempGrants` grants beside `/private/tmp` and `/tmp` | `TMPDIR=/var/tmp/<dir>` | a fixture relying on a stand-down keyed to `/tmp` itself |
| a home prefix behind a symlink | `/home` → `/System/Volumes/Data/home` | a planted `<root>/home` → `<root>/Data/home` | a path literal in a pure-predicate pin |

`TMPDIR` is enough for the first two because `t.TempDir()` reads
`os.TempDir()` and `SeatbeltWritable` reads `os.Getenv("TMPDIR")` at render
time (seatbelt.go:1311) — so one environment variable moves the fixture AND
the grant set together, which is exactly what darwin does.

MEASURED here, each row red before the fix under its own shape and green
after, with the pre-fix message matching the macos log:

```
TMPDIR=<link>        upgrade_test.go:489      want cwd=<link>/…, got cwd=<real>/…
TMPDIR=<link>        codexwritable_test.go:727  …resolves to …, so no symlink component reaches the line
TMPDIR=/var/tmp/…    bwrapstatedirs_qa_test.go:376  the declaration's refusal does not name the config file: bwrap
                                                    cannot grant write on /var/tmp/…: it contains the per-seat
                                                    wall root /var/tmp/…/.cache/posse-wall
planted /home link   wallseatwall_qa_test.go:61   SeatWallRoot = "<root>/Data/home/someone/.cache/posse-wall"
```

All 63 arms of the five touched files are green natively, under
`TMPDIR=<a link>`, under `TMPDIR=/var/tmp/<dir>`, and under a TMPDIR that is
both at once (darwin's actual shape).

## 1 — the cwd a shell cannot misspell (upgrade_test.go)

`TestUpgradeBuildHandsCleanBuildTheRealSourceCheckout` runs the real
`upgradeBuild` against a script that echoes `$PWD`, and compared it against
the raw `t.TempDir()`. A cwd is a path the KERNEL answers, so the shell can
only ever report the real one — there is no spelling in which that assertion
could hold on darwin. The script says `$(pwd -P)` now and the test compares
`absResolve(wt)`: both sides resolved, and the same question asked twice
rather than two questions.

## 2 — a pure-predicate pin over a literal that darwin resolves (wallseatwall_qa_test.go)

`SeatWallRoot` composes from the RESOLVED home, deliberately and with ADR
0100 D2(a)'s reason at the function: the grant a render is judged against is
resolved too. The pin asserted `filepath.Join(home, ".cache", "posse-wall")`
over `const home = "/home/someone"` — a literal that is an ordinary
non-existent path here and an autofs link on darwin. Fixed by resolving the
home once in the pin and reading every expectation off that.

The containment arm below it moved from `insideDir` to `underDir` in the same
breath. That is hardening and not a reported row: the claim is that the wall
root and the seat cache root do not CONTAIN one another, which is a question
about two directories, and `insideDir` (whose split from `underDir` exists so
one caller can resolve its two sides differently) would let a home behind a
link answer "disjoint" by spelling. The planted-link mutant could not red it
— `posse-seat` and `posse-wall` are disjoint under either spelling — so it is
recorded as unwitnessed rather than claimed.

## 3 — a fixture precondition that was the production predicate's proxy (codexwritable_test.go)

`TestCodexLaunchRefusesAWritableRootCodexWouldRefuse` plants a dangling link
on the launch line and asserted the premise as
`codexWritableRoot(slots) == slots`. That is the same question as "a symlink
component reaches the line" only on a host where nothing ABOVE the dangling
link is a link too. On darwin `/var` is, so the rendered root differed for a
reason that had nothing to do with the fixture and the precondition `Fatalf`d
over a premise that held.

It asks `symlinkComponent(slots) == dangling` now — which is what
`refusedWritableRoot` itself asks, so the fixture is checked by the
production predicate the arms below depend on rather than by a proxy for it.

**And the row behind that Fatalf**, which the bead could not see because the
fixture stopped first: arm 1 asserted the refusal names `AbbrevHome(slots)`,
the RAW root, while `codexReachRow` prints the token it read off the line —
the rendered, resolved one. Fixed in the same place.

## 4 — a stand-down keyed to `/tmp`, in a fixture that is only under `/tmp` on linux (bwrapstatedirs_qa_test.go)

`bwrapRefuseWallRootGrant` stands down where the home is itself under `/tmp`,
and its comment says why and says every fixture in the package relies on it:
`/tmp` is granted unconditionally, so a home inside it is in the shared
directory already and there is no seat for the wall root to protect.

On linux `t.TempDir()` IS under `/tmp` and the stand-down was free. On darwin
it is `$TMPDIR` under `/var/folders/…` — **granted too, but not `/tmp`** — so
the refusal did not stand down, the blanket temp grant contains a wall root
composed from a home inside it, and D19's control arm read ADR 0100's
containment refusal instead of the config-file refusal it was written for.

The fix is in `bwsHome`, so every arm in the file takes its home from one
place for `wckFixture`'s reason: `os.MkdirTemp(bwrapTmpMount, "posse-bwsd-")`
— under `/tmp` on both hosts, where `underDir` resolves `/tmp` to
`/private/tmp` on darwin and the stand-down bites. The parent as the package
const and the `posse-` prefix are the temp-root census's
(internal/treepins/temproot_qa_test.go), the same two tokens `btLive` spells
next door.

**Neither of the bead's two suggested fixes was taken.** Widening the
stand-down to "any granted temp root" would weaken a refusal on linux, where
`os.TempDir()` need not be `/tmp`, to make a darwin row green; skipping the
render off linux would retire the pin on the only host that can run it. The
fixture was the thing that was wrong.

## 5 — not a spelling at all: the venue step does not exist on darwin

`TestQATheVenueCallSiteDiffersFromTheJudgesInTheKindAndTheChildOnly` read
`rendered 1 cage(s), want 2`, and the bead filed it UNDIAGNOSED with a
caveat: if the venue cage were MISSING on darwin rather than mis-spelled,
ADR 0097 D2's venue would run uncaged there, and the row was to be refiled
`-l security` at P1. It is neither.

**It is ranger-base-jyqk4 family 1's residual, one file over.** ADR 0097 D2
asks three questions, and the SECOND is "is this host's wall a bwrap" — if it
is not, the step DOES NOT EXIST (ADR 0096: no darwin change). jyqk4 answered
that with the `landedChecker.wallMechFn` seam and set it in `wckFixture`,
whose comment says it is there and not in each arm "so an arm written
tomorrow cannot forget it". An arm in ANOTHER FILE forgot it: this one builds
its checker with `newLandedChecker` directly.

MEASURED by driving the mechanism the other way here
(`wallMechanismForTest(t, "darwin")`): with the seam the arm is green, and
without it the exact macos red, over a judge that had answered **green** —

```
✓ …/003 green at bc4bd3e51d88 in 0.0s (scripts/posse-landed-check, ADR 0088 D1)
· …/003: wall check not run — this host realizes cage seatbelt with
  sandbox-exec, and ADR 0097 D1's venue is a bwrap (ADR 0096: no darwin
  change), so the step does not exist here
```

So the earlier reading on the bead — "most likely the judge answered
not-green on a spelling" — is ruled out: the judge was green and the step
declined one question later, on the host and not on the tree. And the
write-set spellings the bead noticed in that one cage (`/var/…` beside
`/private/var/…`) were the `%+v` of the JUDGE's call, which is a dump and not
a finding.

**Not a security exposure, confirmed twice over.** `measure` gets the venue's
argv only from `c.cageFn()`, a cage that will not render returns
`landUnanswered`, and there is no uncaged arm — so a missing cage is a
missing MEASUREMENT and never an uncaged venue. Here the step was not reached
at all.

That log line is the whole diagnosis, and the arm had been throwing it away
into a discarded `strings.Builder`. It keeps the writer and prints it on
failure now, which is the other half of the fix.

## not closed here

- **ranger-base-akwrv** — the CLASS of row 5 is pinned by nothing. Twice now
  an arm whose subject is what the venue DOES has read the one line that
  declines it. A source census over internal/posse's tests ("ships
  `wallCheckScript` and builds a `landedChecker`" implies "takes
  `wckFixture`, or sets `wallMechFn`") has the TREE as its subject, so it
  belongs in internal/treepins, where ADR 0098 D4's complement door gives it
  a door for free. Two members today, both correct.
- **The macos leg's own verdict.** Every row here is fixed and witnessed by a
  planted shape, not by darwin: no `sandbox-exec` ran, and nothing in this
  fragment claims a macos pass. The bead's done-condition is the next
  macos-latest arm 1 leg, which this seat cannot run and may not push — the
  same shape ranger-base-jyqk4 and ranger-base-i36bf closed under.
