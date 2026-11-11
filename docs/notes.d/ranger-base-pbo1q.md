## The cold-first-exec injector, as built, and what it measured on the way (ranger-base-pbo1q)

2026-11-10 — ADR 0085 D2-D6 built, on Linux 7.2.15 x86_64, go1.26.5. From
ranger-base-gdzu5 (the design) and ranger-base-aru4c (the census this had to
reproduce without its two hand-made readings).

The mechanism and the argument live in the code:
`internal/posse/coldexec_test.go` (the injector, the shim and the one-way
reasons for each), `internal/posse/execwrite.go` (the seam and the
test-caller rule), `internal/posse/coldexecinject_test.go` (six pins), the
`coldexec-check` block in the Makefile (the door and why it queues), and one
bullet at the end of AGENTS.md. This fragment holds what is not derivable
from any of them: the acceptance measurement, the nine mutations, and the
four places the build decided something ADR 0085 left open.

### The acceptance measurement, which is the whole point of the bead

ADR 0085's consequences name one test as ground truth:
`TestRunOnceDoesNotLockOnARead` (`internal/posse/bdwritelock_test.go`), whose
2s bound over the first exec of a planted `bd` sat BELOW darwin's measured
2.17s worst case, and whose fatal reads "a read verb waited on a write lock
held by somebody else" — the defect under test, reported over code that is
fine. ranger-base-aru4c fixed it with a `--warm` pre-flight. MEASURED here,
with that pre-flight reverted in the working tree:

| run | verdict |
|---|---|
| `go test -run TestRunOnceDoesNotLockOnARead` (no injection) | **ok, 0.010s** |
| `make coldexec-check RUN='^TestRunOnceDoesNotLockOnARead$'` | **FAIL, 2.00s**, `bdwritelock_test.go:486: a read verb waited on a write lock held by somebody else` |
| the same door with the pre-flight restored | PASS, 2.52s, `1 plants rewritten` |

The first row is the hole this bead exists for, stated as a number: a silent
revert of aru4c's fix is green on this box in ten milliseconds, and would
have flaked only on ci.yml's macos-latest leg and on the laptop. The third
row is the other half — the test pays the injected 2.5s in the pre-flight,
outside its own deadline, and the bound then covers a warm re-exec, which is
the only thing a number can honestly bound.

And the stay-green half of the same consequence list, all MEASURED under
injection at D=2.5s, all PASS: `TestProbeSessionExeRefusesWhatItCannotName`
(7.84s — paid inside `mux.Start`, which is aru4c's adjudication rule 1
arriving as behaviour), both `ProbeOpts{Timeout: 2 * time.Second}` sites
(`TestRuntimeProbeRecordsTheBinaryTheSessionResolved` 7.09s,
`TestRuntimeProbeAsksTheCredentialPreconditionTooBeforeItLaunches` 2.53s),
`watchpid_test.go`'s `liveProcess` through
`TestWatchStampsAndClearsPidfile` and `TestAutostartIgnoresARecycledPid`
(rule 2: `sh <path>` execs /bin/sh and never the plant), and the warmed
`hangingBin`/`hangingGit` families
(`TestQAHungBdChildIsSignalledAndNamed`, `TestQAHungHerdrChildIsKilledAndNamed`,
`TestQAHungHerdrControlCallIsKilledAndNamed`, `TestQAHungGitChildIsSignalledAndNamed`,
`TestQAHungGitPatchIDPairIsSignalled`). That arm-3 slice rewrote **20
plants** and passed **2** through as non-scripts, so the greens are greens
over real injection and not over an injector that did nothing.

### Nine mutations, nine distinct witnesses

The pins are an absence claim about a timing class, which is the shape that
goes green by ceasing to measure. Each mutation was applied alone and
reverted:

| mutation | witness that fired |
|---|---|
| drop the hook call in `WriteExecutable` | `a plant written from a _test.go file kept its own shebang` |
| drop the `callerIsTestFile` rule | `copySkillFile's bytes were rewritten` |
| the shim sleeps on every exec | `a re-exec of the same path took 1.506s, as long as the first` |
| the shim sleeps never | `the first exec … returned in 5.8ms under a 1.5s injected delay` |
| a constant nonce | `re-planting the same path reused the nonce …` |
| `rewrite` not idempotent | `already-rewritten bytes were rewritten again, burying one shim inside another` |
| the shim forks instead of `syscall.Exec` | `the body ran as pid 224991 and the parent started pid 224977` |
| the marker is created at WRITE time | `the marker … existed before any exec` **and** ``sh <plant>` created the first-exec marker`` |
| the injected line is not a `#` comment | ``sh <plant>` printed "…: line 2: posse-coldexec: command not found\nran\n"`` |

The last two are the ones worth keeping: both are real alternative designs,
and both are caught by the controls rather than by a clock.

### Four things the build decided that the ADR left open

**1. The nonce IS the marker path, so the shim needs no environment.** D2
says `<markerdir>/<nonce>`; the injected line carries the whole path. The
alternative was an environment variable naming the marker dir, and a test
that scrubs its child's environment would then have produced a shim that
silently delayed nothing. The delay itself still comes from the environment
and falls back to D when it did not arrive, which is correct by
construction: a rewritten file exists only because injection was on, so the
question is never whether to sleep but how long.

**2. `rewrite` is idempotent, which D2 does not mention and the pins need.**
A pin plants bytes it rewrote itself and still writes them through
`WriteExecutable` — the one door — so under the door's own run those bytes
arrive at the hook a second time. Keying idempotence on the CONTENT (line 2
already carries the marker) and not on the path keeps D2's "a fresh nonce per
write makes a rewritten file cold again": re-planting a file's ORIGINAL
content is not this case and gets a new nonce.

**3. The interpreter argv is rebuilt per platform.** Linux hands a shebang
interpreter at most one argument — everything after the first word arrives
as a single argv entry — and darwin splits the line into words. The shim
reproduces whichever rule it is running under, so `#!/usr/bin/env python3 -u`
reaches the interpreter the way it would have with no injector in the way.

**4. The known artifact the count does NOT cover: an interpreter whose
comment character is not `#`.** D5 counts the plants with no `#!` at all.
The injected second line is a `#` comment, which is right for sh, python,
perl, ruby and awk and is a syntax error for `#!/usr/bin/env node`. Nothing
in this tree plants one today. If the full run under ranger-base-kiwvn finds
one, it is an injector artifact and the injector is the place to fix it — the
ADR's own disposition for the `$0`-and-line-numbers class, of which this is
one more member.

### The door queues by naming a package TREE, not by a new rule

D6 asks for suite slots like `make test`. `scripts/suite-lock.sh`'s
`suite_lock_wanted` deliberately does NOT queue `go test ./internal/posse` —
that is the arm 1 run a person does while thinking, and its own comment says
reversing it is the architect's call. So the unfiltered arm 1 line is spelled
`./internal/posse/...`: the package has no subpackages, so the two select the
same one arm, and a package-tree pattern is what the rule already queues.
MEASURED through `suite_lock_wanted` directly — all three unfiltered lines
queue, the `RUN=` form takes no slot.

### Prices

`make coldexec-check RUN=<one test>` is seconds: 12.3s for the six injector
pins across all three arms, 24.3s for the fifteen-test acceptance slice.
The unfiltered three-arm run is NOT priced here — that is ranger-base-kiwvn,
which also triages its reds.
