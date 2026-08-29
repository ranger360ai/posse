## The disk runs out and the suite blames eighty tests (ranger-base-krra)

Filed from ranger-base-7qwm, which was a flake hunt that ran into something
else. `make test` came back exit 2 with ~80 reds in `internal/rhq`, every one
of them the same line:

    --- FAIL: TestWatchPidRoundTrip (0.00s)
        testing.go:1426: TempDir: mkdir /var/folders/dy/.../T/TestWatchPid...:
            no space left on device

MEASURED 2026-08-29, ~15:10-15:40 local, at that moment:

    /dev/disk3s1s1              460Gi  418Gi used  231Mi avail  100%
    ~/Library/Caches/go-build   41G
    /var/folders/dy/.../T       6.1G, 21965 entries, 670 leftover Test* dirs
                                (oldest 2026-08-27)

`posse` and `cmd/posse` were green in the same run — they had already
finished. Minutes later, and not by anyone in that session, the build cache
was cleared mid-build and a `go test` in that window failed to BUILD with a
shape that reads like a broken toolchain and is not one:

    /opt/homebrew/Cellar/go/1.26.5/libexec/src/bytes/iter.go:8:2:
        package iter is not in std
    FAIL github.com/ranger360ai/posse/internal/rhq [setup failed]

Afterwards: go-build 137M, 40Gi free, `go build ./...` clean, `make test`
green. Nothing in the product was wrong at any point.

### Why it needed a bead and not a `rm`

**The red never names the disk where a reader looks.** Through the house
filter (`grep -E '^(---|ok|FAIL)'`) it is a list of unrelated test names —
worktree, watch, dispatch, merge — and reads exactly like a broken change.
The word `disk` appears only inside a `t.Fatal` message, below the fold, and
only if you scroll to one of the eighty.

**One cause, eighty symptoms, by construction.** `t.TempDir()` calls
`t.Fatal` on ENOSPC, so a single full filesystem is reported once per test
that wanted a temp dir. The count is a property of how many tests use
`t.TempDir`, not of how broken anything is.

**The sibling red already had a guard and this one had none.**
ranger-base-2ggb put `-timeout 25m` on the recipe and ranger-base-7xla put
`scripts/test-times.sh` in front of it, so a suite that runs out of CLOCK now
says so in a block instead of a goroutine dump. A suite that runs out of DISK
said nothing.

**And it is self-inflicted and recurring.** The build cache reached 41G
unattended on the box that runs every session's suite, and 670 leaked `Test*`
dirs going back two days say the ENOSPC had been eating cleanups for a while.

### What landed

The same two moments as the clock's guard, in the same script.

**Before the packages run**, one line, because the cheapest time to learn the
box is full is not ten minutes in:

    test-times: DISK: 34664 MB free on /System/Volumes/Data — t.TempDir and the go build cache

Two directories decide whether a run fits: `TMPDIR`, where every `t.TempDir()`
lands, and `GOCACHE`, which grows for the whole run. On this box they are the
same APFS volume and it prints once; they are not required to be, so it asks
`df` rather than assuming, and prints one line per distinct filesystem.
`GOCACHE` comes from the go binary the run will actually use (`$(GOBIN)`, not
necessarily the `go` on `PATH`) and is skipped unless the answer names a
directory that exists — which is also what makes it safe under a stub in
`--self-test`.

**After a run whose log carries `no space left on device`**, a block that
names the cause, says those reds belong to the box, and gives the three
places the space goes (`df -h "$TMPDIR"`, `du -sh "$(go env GOCACHE)"`,
`ls -d "$TMPDIR"/Test*`).

**A second block for the same hour's other face.** `package iter is not in
std` from a working toolchain sends a reader to reinstall go; it arrives as
`[setup failed]`, which is a BUILD error and so names no test at all. The
block quotes the offending line and says: re-run before touching the
toolchain.

### The floor is measured, and is a warning

`DISK_FLOOR_MB` defaults to **5120**. A full green `go test -timeout 25m
./...` on this box (darwin 25.4.0, go1.26, 2026-08-29, `df -kP` sampled
across the whole 830s run) took the filesystem behind `TMPDIR` from 35489 MB
free to a low of 34586 MB:

| | |
|---|---|
| consumed by one run (peak) | **903 MB** |
| of which go build cache | +686 MB (5116 → 5802 MB) |
| of which `TMPDIR` churn | +77 MB |
| `internal/rhq` this run | 829.5s |

The box is shared and other sessions write the same volume, so 903 MB is an
upper reading of one run, not a clean-room one. 5120 MB is that peak times
~5.7, and the multiplier is the two things one run cannot measure: this box
routinely has two or three sessions' suites running at once on the same
volume, and a cache that has just been cleared has to regrow this repo's
working set (5.8 GB, measured above) — which is exactly the state a
disk-pressure cleanup leaves behind, so the run *after* the cleanup is the one
most likely to be short.

**It warns and never fails.** A floor is a claim about a box, and a red that
belongs to the box is the whole thing this script exists to prevent, not one
to introduce — the same rule that keeps it from failing on a wall clock.

**And it deletes nothing.** `go clean -cache` slows every concurrent session
and deleting from `$TMPDIR` can take a live test's TempDir out from under it,
so what to clear on a shared box stays the operator's call. The bead's third
option — a cache cap or a scheduled clean — is not taken here and is still
open; a disk alarm is what a persona can honestly ship on its own.

### How it was verified

Fixtures prove you typed them right and nothing else, so both shapes were
produced on a real filesystem: an 8 MB HFS+ disk image mounted as `TMPDIR`,
with a three-test package whose tests are named after the ones in the incident.

- Filled to 788 KiB free, a test that WRITES into its TempDir fails with
  `write …/blob: no space left on device`.
- Filled to **0** blocks free, `t.TempDir()` itself fails with
  `testing.go:1426: TempDir: mkdir …: no space left on device` — the incident's
  line, `testing.go:1426` included, so the fixture is faithful.
- Both real logs replayed through the wrapper fire the block and report
  `3 failure(s)` / `3 failing test(s) in total`.
- The split-filesystem path is real, not theoretical: with `TMPDIR` on the
  image and `GOCACHE` on the internal volume the preflight printed two lines
  and warned on the one below the floor.

One trap found on the way and worth writing down: **`cmd/go` hands the test
binary `GOTMPDIR` as its `TMPDIR`.** Setting `GOTMPDIR=/tmp` (to keep the
link off the tiny volume) silently moved `t.TempDir()` to `/tmp` as well, and
the "full disk" run went green. The only way to have a roomy build and a
cramped test is `go test -c` first, then run the binary with the `TMPDIR` you
want.

`scripts/test-times.sh --self-test` grew arms for all of it, and each was
mutation-checked — preflight removed, `explain_disk` removed, `explain_disk`
firing unconditionally, the count hardcoded, the floor comparison replaced by
`true`, the `df` reading replaced by a constant, the exit status swallowed.
Every mutation reds at least one arm; none was green over a broken guard.

`suitedisk_qa_test.go` holds the two things the shell cannot hold for itself:
that `make test` still runs the suite THROUGH the wrapper (without which every
explainer is dead code, and which nothing else pinned — `suitetimeout_qa_test.go`
reads the `-timeout` on that line and stays green if the wrapper is dropped
from in front of it), and that the wrapper still passes its own `--self-test`,
by arm name, from inside `go test ./...` rather than only from `make
verify-test-times`.
