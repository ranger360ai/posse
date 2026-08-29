## `make test`, not `go test ./...` — the suite outgrew go's default timeout (ranger-base-2ggb)

`go test`'s `-timeout` defaults to **10m per package**, and `internal/rhq`
spends most of it. Every darwin reading anyone took on 2026-08-29 —
`-count=1`, go1.26.5 darwin/arm64, three sessions, three beads:

| run | wall | source |
|---|---|---|
| `go test ./internal/rhq -timeout 25m` | 484.6s | this bead, loadavg ≈ 11 |
| ” | 509.6s | ranger-base-2ggb's own reading |
| ” | 510.0s | ranger-base-2ad3 |
| ” | 549.3s | ranger-base-7xla |
| ” | **623.2s** | ranger-base-2ad3, rebased tree — **already past the default, alone** |
| `go test ./...` | 491.7s ok | ranger-base-7xla, at 2d8ccc9 |
| `make test` (`-timeout 25m`) | 537.1s ok | this bead, after the fix |
| `go test ./...` | **FAIL 600.8s** | ranger-base-7xla |
| `go test ./...` | **FAIL 601.0s** | ranger-base-2ggb |
| `go test ./...` | **FAIL 601.1s** | ranger-base-2ad3 |

The 601s figures are not assertions. `./...` builds and runs the three
packages **concurrently** (`-p` defaults to GOMAXPROCS), so the two short
packages — 88.1s and 73.3s — spend their run competing with the long one for
the same cores, and the long one crosses a ceiling it clears by 19% when left
alone. Add a persona or two verifying at the same time and it crosses
reliably. The 623.2s standalone reading says the margin is gone even without
that. Off darwin the exposure inverts: `golang:1.26` on native linux/arm64
runs the package in **112.3s**, nowhere near — but `make test-linux
PLATFORM=linux/amd64`, emulated on an arm mac, is over 600s **every time**
(ranger-base-7xla).

**It is also the least legible red in the repo.** A timeout is a panic, not a
failed assertion, and the house filter (`go test ./... | grep -E
'^(---|ok|FAIL)'`) throws the panic trace away — what is left is a bare
`FAIL … 601.010s` with no test named at all. It reads as the diff's fault.
Same class as ranger-base-w4fb one level up: a false red that lands on
whoever ran the full suite to verify something unrelated, worst exactly when
several people run it at once, and it costs `suite-green-on-close`.

### There is no long pole to cut instead

The bead's middle option was to find the disproportionate test. There isn't
one. `-json` over a full pass (484.6s, 1442 top-level tests, 58 skipped):

| slice | time | share |
|---|---|---|
| slowest test (`TestQAWorktreeGrantRefusesSharedGitStateUnderSandboxExec`) | 10.3s | 2% |
| top 10 tests | 69.4s | 14% |
| top 50 | 170.5s | 35% |
| top 200 | 319.5s | 66% |
| all 1442 | 483.8s | 100% |

Mean 0.34s, and the tail *is* the runtime. Deleting the ten slowest tests
outright buys 69s and leaves the package still inside its ceiling. Getting to
a package that clears the default with room would mean moving something like
the slowest two hundred tests behind a build tag — which is not "splitting
off the slow tests", it is putting a seventh of the suite behind a flag
nobody types, and the flag was what we were trying to avoid.

The one structural lever is real and out of scope: the package is a single
serial stream with **no `t.Parallel()` anywhere**, and it is not CPU-bound —
`/usr/bin/time` over a 136.2s slice shows 44.2s user + 47.5s sys, so about
two-thirds of a core while the box has eight. Most of the wall is spent
waiting on subprocesses (git, `sandbox-exec`, the test binary re-exec'd as a
fake herdr). Parallelism would help a lot and cannot be had cheaply: 55 of
191 test files call `t.Setenv`/`t.Chdir`, which panic under `t.Parallel`,
and the package shares process-wide state (PATH, `RHQ_FAKE_DIR`, cwd) by
design. That is a design bead, not a timeout fix.

### What shipped

`make test` runs `go test -timeout 25m ./...`. 25m clears the worst darwin
reading (623.2s) by 2.4x — a box half this one's speed still passes — and it
is 25m rather than 20m because `make test-linux` runs this same target and
its `PLATFORM=linux/amd64` arm is over 600s every time; 20m would have left
that rehearsal thin. It does not pretend the package got faster, it stops a
busy machine from being reported as a broken change. Everything already
routes through this target — `.github/workflows/{ci,release}.yml` run
`make test`, `scripts/test-linux.sh`'s gate is `go vet ./... && make test` —
so CI and the Linux container inherit it. The cost is that a genuine deadlock
now takes 25m to surface instead of 10m; a hang that needs a *faster* answer
than that is a hang worth `-timeout` on the command line.

`suitetimeout_qa_test.go` pins it in three arms, because the flag can be lost
three ways: the recipe drops it (arm 1), the recipe keeps it with a value
below the measurement — a `-timeout 8m` reads as a decision and is worse than
the default it replaced (arm 1's floor, 15m); or a NEW entry point invokes
`go test` itself and silently inherits the default again (arm 2, a corpus
sweep of the Makefile, `scripts/*.sh` and `.github/workflows/*.yml`). Arm 3
is the detector's own: both other arms assert *absence* over files that
discuss `go test ./...` in prose, so a `goTestArgs` blind to the Makefile's
`$(GOBIN) test` spelling, or an `isComment` that swallowed the recipe, would
leave the whole thing green over a suite running on the default. All three
mutation-checked — flag removed, flag set to `8m`, and a bare `go test ./...`
appended to an unrelated script — each fails the arm it belongs to and only
that arm.

### Three beads, one invariant

This landed under ranger-base-2ggb, but **ranger-base-2ad3** (gilfoyle) and
**ranger-base-7xla** (laurie → gilfoyle) are the same finding, filed the same
day from three different sessions — which is itself the point: everybody who
ran the full suite that day met it. The measurements above are pooled from all
three. What the flag does not close is 2ad3's option (b): *find where
internal/rhq spends its 500-620s and cut it.* This bead answers the first half
of that question with a No — there is no long pole, the tail is the runtime —
and names the only lever that would move it (in-package parallelism, blocked
by `t.Setenv`). Whoever picks that up starts from the distribution above, not
from a stopwatch.
