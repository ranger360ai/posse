# ADR 0085 — The cold-first-exec class is decided by injecting the cost, not by reading the deadline

*Status: accepted 2026-11-10 (ranger-base-gdzu5, from ranger-base-aru4c) ·
owner: architect · builds on ADR 0006 §7 (a pin names the process it
protects) and on the execwrite pins (every executable write goes through
`WriteExecutable`)*

## Context

A test plants a new executable (`WriteExecutable` into a `t.TempDir()`)
and execs it under a deadline of a few hundred ms. macOS checks a newly
written executable on its FIRST exec. MEASURED on darwin 25.6.0
(ranger-base-y9q7l, ranger-base-aru4c): first exec mean 156-714ms, worst
2.17s; re-exec of the same path 6.3-10.9ms. So a small deadline over a
cold exec gives a verdict about process startup, not about the code. The
class has been found three times (hangingBin 2026-09-03, y9q7l, aru4c).
The fix is settled: a `--warm` pre-flight outside the deadline, with both
halves asserted.

aru4c's census found 14 candidate rows. 10 of them turned on a judgement no
matcher made: does this duration bound an `execve` of the planted file at
all? Two readings settled them: (1) a fixture driven through `Herdr` pays
its cold exec inside a synchronous `mux.Start` before any caller's deadline
starts; (2) `sh <path>` execs `/bin/sh`, not the planted file.

New, MEASURED 2026-11-10 on the Linux host (7.2.15, x86_64, loadavg 7.56,
n=40 fresh `#!/bin/sh` scripts): first exec mean 1.07ms, worst 1.29ms;
re-exec mean 0.96ms. **Linux has no cold-exec cost.** The main crew writes
tests here now (ADR 0083), but darwin still runs the suite: ci.yml's
`macos-latest` leg and the laptop, kept as the rollback home. So a new
unwarmed fixture now passes where it is written and flakes only where
nobody is watching. Prose in hangingBin's header cannot catch that. A gate
that runs here can.

## Decision

**D1. The subject is the verdict, not the source.** The class is "a test
whose verdict changes when the first exec of a planted file is slow." That
is decidable by running the test with the first exec made slow. No reading
of the source is needed. Both of aru4c's readings fall out of execution:
a `Herdr`-driven fixture pays the injected cost inside `mux.Start`, outside
its deadline, so it stays green. `sh <path>` never execs the path, so it is
never delayed.

**D2. The cost is injected at `WriteExecutable`, the one door.** The
execwrite pins already force every executable write in the tree through
`WriteExecutable`. So the set of plants is closed by an existing gate and
the injector needs no census of its own. When a test-only hook is set,
`WriteExecutable` rewrites a script whose first line is `#!<interp>` to:

```
#!<shim>
# posse-coldexec <nonce> <original interpreter line>
<original body>
```

The kernel then execs `<shim> <path> <args>`. The shim atomically creates
`<markerdir>/<nonce>` (O_EXCL). If it created it, this is the first exec,
and it sleeps D. Then it `syscall.Exec`s the original interpreter line with
`<path> <args>`, keeping its pid, so pid files and kills still name the
right process. A fresh nonce per write makes a rewritten file cold again.
The marker lives outside the script's directory so tests that list a dir
see no extra file.

**D3. The hook is test-only by construction.** A package variable in
`execwrite.go`, nil by default and assigned only in `herdr_test.go`'s
`TestMain` when `POSSE_COLD_EXEC_DELAY` is set. It applies only when the
immediate caller of `WriteExecutable` is a `_test.go` file
(`runtime.Caller`). Shims that production code writes keep their exact
bytes, so content read-back pins do not red for the wrong reason. A shipped
binary never assigns the hook. The shim is the test binary itself,
detected in `TestMain` by argv[1] naming a file whose line 2 is the
`posse-coldexec` marker. This is the same pattern as `IsCageLaunch`, and it
needs no build step.

**D4. D = 2.5s.** That is above the measured darwin worst case (2.17s). A
deadline that survives 2.5s of cold exec survives darwin. ASSUMED: the
2.17s tail is the true worst. If darwin ever measures worse, raise D. It is
one constant.

**D5. Non-script plants are passed through and counted.** Content without
`#!` (a copied binary) cannot be rewritten. The injector counts these and
the run prints the count, so the gap is a number and not a silence.
ASSUMED small: 124 of the 322 test call sites have `#!` in their literal
text within three lines, and most of the rest pass a variable built from a
script string. Slice 2 measures the real count.

**D6. The door is `make coldexec-check`**, with `RUN=<regex>`.
Unfiltered, it runs internal/posse's three arms under injection and takes
suite slots like `make test`. With `RUN=`, a bead that adds a planted
fixture checks only its own tests, which takes seconds. It is not part of
`tree-check` or `make test`. D adds up to 2.5s once per planted file, and
the full run is priced in slice 2 before anyone proposes running it more
often. Whether CI should run it is the operator's call, because it costs
runner time.

## Consequences

- The class stops depending on someone remembering it. Running the door
  turns the darwin tail into a deterministic red on Linux, with the fatal
  the test already prints.
- Acceptance is measured against aru4c's census, which is the ground truth
  this ADR has to reproduce without the two readings. Under injection,
  `TestRunOnceDoesNotLockOnARead` with its pre-flight reverted must red,
  and green with it. `TestProbeSessionExeRefusesWhatItCannotName` (300ms,
  Herdr), the `ProbeOpts{Timeout: 2s}` sites, `watchpid_test.go`
  `liveProcess` (`sh script`) and every warmed helper (hangingBin,
  hangingGit) must stay green.
- The first full run will red some tests. Each red is either in the class
  (fix it with a `--warm` pre-flight) or an injector artifact (fix the
  injector). Both get recorded. A test that changes `$0`, or reads its own
  line numbers, sees one extra line. That is an artifact, and the injector
  is the place to fix it.
- Exit hatch: the hook is one nil variable and the shim is one argv arm.
  Delete both and nothing else changes. They hold no state outside a
  temporary marker dir.

## Alternatives rejected

- **A static matcher over duration literals near `WriteExecutable`.**
  aru4c's census is the measurement against it: 10 of 14 rows were
  undecidable. It either reds the runtimeprobe/watchpid family or misses
  the two sites that were actually wrong.
- **A pin over the fixture helpers** (every plant goes through a
  registered warming helper, or carries an exemption comment). This was
  the bead's most promising static shape. Across 322 call sites, most of
  them never timed, it becomes an exemption list whose entries are
  comments. ADR 0006 §7 asks what process changes if such a pin is
  deleted, and the answer is none: the comment would be the subject.
- **Prose only** (hangingBin's header plus the notes fragment). That is
  what held the class until now. It was rediscovered three times, and on
  Linux the darwin cost is now invisible to the people writing tests.
- **The clever one: emulate Gatekeeper in the kernel.** Use fanotify
  `FAN_OPEN_EXEC_PERM` to hold the first exec of each new file, or a
  bubblewrap namespace with `/bin/sh` bound to a shim that reads
  `AT_EXECFN` to tell a shebang exec from `sh <path>`. Both leave file
  bytes untouched and catch compiled binaries. But fanotify permission
  events need CAP_SYS_ADMIN in the initial namespace. The namespace shim is
  Linux-only and replaces the shell every test uses, so it would be wrong
  in more places than the problem it solves. Rewriting the shebang at the
  one door costs a line of `$0` drift and the non-script gap (D5). Both
  are counted.
- **Make `WriteExecutable` warm every plant by exec'ing it once.** Running
  arbitrary test fixtures for their side effects is not safe: half of them
  write marks, pid files or exit codes that the test then reads.
