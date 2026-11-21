# The ubuntu donor fix holds, and ubuntu-latest's donor is a uutils symlink and never a '#!' stub (ranger-base-x0ses)

2026-11-21. The reading ranger-base-x78s5 could not take, because the CI run
did not exist yet. Fourth bead in the family (ranger-base-8z410 ->
ranger-base-zcj0z -> ranger-base-fol4d -> ranger-base-x78s5 -> here), and the
first one that read a green arm.

## The job

ranger-base-x78s5's commit (bac28fe0) was pushed with ranger-base-pdcf6's
(eb3f34fc) and so has no run of its own: run **44923103475**, head eb3f34fc,
job **136903195755** `test (ubuntu-latest, 1)` is the first ubuntu-latest
arm-1 job carrying it. Log read whole.

## Finding 1's fix is CONFIRMED

Both finding-4 arms print `ok` and there is **no postmortem anywhere in the
job**:

```
ok   claude_procs (0) and the pids it reports (0) agree
ok   a live 'claude -p' run (pid 11092) is excluded from claude_procs
ok   control: the same binary without -p IS counted (pid 11280)
...
reap-test-tmp.sh --self-test: all arms pass
```

`verify-reap-test-tmp` is arm 1's NINTH prerequisite; the tenth
(`verify-wall-check`) printed `posse-wall-check --self-test: 10 arms clean`
and the eleventh (`tree-check`) went on to compile and run Go tests —
`ok github.com/ranger360ai/posse/internal/posse 6.179s` at 16:11:49, the
first Go test an ubuntu arm-1 job has compiled since 2026-11-11. The arm then
ran its own `go test ./...`: `internal/treepins` **ok 262.020s**,
`internal/posse` **FAIL 433.453s** on four tests, none of them the rig's.
So the donor fix is not merely green, it moved the job's failure 11 minutes
downstream into territory nobody had been able to see.

## The ASSUMED claim is FALSIFIED, and the real mechanism is better than the one we wrote

x78s5 left one claim for a runner to settle: that ubuntu-latest's
`/usr/bin/tee` is a `#!` stub of a multi-call coreutils, the shape a
`--enable-single-binary-shebangs` build installs. **It is not. It is a
symlink.** No shell on the runner was needed; the runner's own log, Launchpad
and the two candidate .debs settle it.

MEASURED 2026-11-21, from the job log, `api.launchpad.net`, and
`archive.ubuntu.com`:

| reading | value |
|---|---|
| the runner, from its own `Set up job` block | Ubuntu **26.04.1 LTS**, image `ubuntu-26.04`, version `20261116.202.1` |
| its `coreutils`, from the image's own readme | `9.5-1ubuntu2+0.0.0~ubuntu25` |
| that version, from Launchpad | source **`coreutils-from` 0.0.0~ubuntu25**, series `stonking` |
| that binary package | Architecture `all`, **2.7 kB**, `dep: coreutils-from-uutils \| coreutils-from-gnu` |
| `coreutils-from-uutils_0.0.0~ubuntu25_all.deb`, extracted | `/usr/bin/tee -> ../lib/cargo/bin/coreutils/tee`, `/usr/bin/sleep -> ../lib/cargo/bin/coreutils/sleep` |
| `coreutils-from-gnu_0.0.0~ubuntu25_all.deb`, extracted | `/usr/bin/tee -> gnutee`, `/usr/bin/sleep -> gnusleep` |

So on this runner `coreutils` is an empty metapackage and every utility on
PATH is a **symlink into one multi-call binary**. Neither variant ships a
`#!` line anywhere.

And the reason `exec -a tee` was a no-op is in uutils' source, not in the
kernel's shebang handling (`src/common/validation.rs`, the linux-gnu
`binary_path`, read 2026-11-21):

```rust
/// Get actual binary path from kernel, not argv0, to prevent `env -a` from bypassing
/// AppArmor, SELinux policies on hard-linked binaries
let execfn = rustix::param::linux_execfn();
let argv0 = args.next().unwrap();
// When the names match, keep the kernel path: argv0 may lack the directory after PATH lookup.
if basename(execfn) == basename(argv0) { return exec_path.into(); }
```

and the message the CI postmortem printed is that file's own, byte for byte:

```rust
pub fn not_found(util: &OsStr) -> ExitCode {
    let _ = writeln!(stderr(), "coreutils: unknown program '{}'", util.maybe_quote());
```

`name()` is `file_stem()`, so a copy at `…/claude` asks to be the utility
`claude`, finds none, and exits 1 — **and it ignores `argv[0]` on purpose**,
to stop exactly the kind of spoofing `exec -a` is. That also decides which
variant the runner has: under `coreutils-from-gnu` the donor would be a
plain GNU `tee` that cannot refuse a name, and the rig would have passed.
It did not, so the runner carries the uutils variant.

**x78s5's conclusion stands and its fix stands** — no `argv[0]` spelling can
fix it, the donor is the thing to change, and `${BASH:-$(command -v bash)}`
is green on the runner. Only the recorded *mechanism* was wrong: a symlink to
a multi-call binary that discards `argv[0]` by design, not a shebang exec
that rebuilds it. The `#!` mutant reproduced the CI line because both shapes
end in "the dispatcher does not see the name you asked for".

## The same donor class, in Go, is why two of the four new reds fail

`internal/posse/watchimage_qa_test.go` does what the rig used to do:
`exec.LookPath("sleep")`, copy its bytes to `<root>/posse`, run the copy as a
stand-in dispatch loop. On this runner that copy IS the uutils multi-call
binary, asked to be a utility named `posse`, so it exits 1 within
milliseconds of starting. Both reds are the fixture's own read-time guard
(`rrgncChildStillOurs`) firing after `rrgncCopy` — the guard working exactly
as ranger-base-rph5z's run built it to:

```
watchimage_qa_test.go:159: pid 164890: this fixture's stand-in loop is gone
  before the reading — the assertions below would be about a stranger
watchimage_qa_test.go:390: pid 164816: … (same line)
```

MEASURED here, three readings, Linux 7.2.15-1-omarchy, go1.27.3, 16 cores:

| run | reading |
|---|---|
| the four CI reds, solo, at eb3f34fc | **all PASS**, 2.947s — this box's `sleep` is an ELF GNU coreutils 9.12, first two bytes `177 E` |
| a faithful mutant donor first on PATH as `sleep` (a Go ELF that prints `coreutils: unknown program '<stem>'` and exits 1 for any stem but `sleep`, and sleeps otherwise) | **both watchimage tests FAIL at :159 and :390 with the CI line verbatim** |
| the mutant run as `sleep`, and copied to `posse` | sleeps / `coreutils: unknown program 'posse'`, exit 1 |

The mutant is in this bead's scratchpad (`mutant/fakeuutils.go`) and keys on
`argv[0]`'s basename, which in this fixture's exec is byte-equal to
`AT_EXECFN`'s — the fixture breaks under either dispatch key, which is why
the mutant is faithful without reimplementing the auxv read.

**Census of the rest of the tree, so this is the last site and not the next
one.** Every `exec.LookPath("<a coreutils utility>")` in the tree and every
`command -v` of one in `scripts/`: the only site that runs a utility under a
basename the utility does not own is `watchimage_qa_test.go`.
`queuecutover_undo_qa_test.go`'s `qcFlakyMv` keeps the name `mv` and execs
the real path; `upgradeverify_test.go` runs `pwd` at its own path;
`gates.go`'s `posseSupportsBdWrite` passes `true`'s own path;
`reap-test-tmp.sh`'s `generic_rig` runs `$sleepabs` at its own path (all four
arms of it are green in this job). A symlink would also survive —
uutils resolves a symlinked path through `utility_from_symlink` — but a
symlink is no use to a fixture whose subject is an install renaming over the
file the loop runs.

## The four reds

Not closed here; each is another lane's, filed with the job and the log line:

- ranger-base-f177g — the two watchimage reds above (P1, code).
- ranger-base-ebv8j — `TestLiveWallWatchBackstopFilesAWatcherThatDiedUnderARunningSeat`
  asserts `"pids " + <the bg pid>` as a substring of a bead that renders
  `wallWatchPidList` in probe order; the runner's probe returned two pids and
  the bg one is second, so `pids 166926 166927` does not contain
  `pids 166927`. One line either way, and it passes here because this box's
  probe returned the one pid (P2, code).
- ranger-base-z56s9 (P1, security lane) — the fourth red. Named by id alone.

## What the next reader should take from the family

Four beads, and the thing that cost all four was that the ubuntu leg's own
output was read by nobody: fol4d's close said "NOT CLOSED HERE: nothing" over
a fix that did not work, and two runs printed the pre-fix line verbatim
before a verify bead noticed. The postmortem prints observations now
(`/proc/<pid>/exe`, `/proc/<pid>/cmdline`, the donor's first two bytes), and
this bead needed none of them, because the arms were green. What it needed
instead was the runner's own `Set up job` block — which says what box the
leg is, and which nobody in this family had read.
