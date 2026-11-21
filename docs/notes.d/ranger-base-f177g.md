# The stand-in loop's donor is bash, and a donor picked for its NAME is the bug this family keeps rewriting (ranger-base-f177g)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, go1.27.3, 16 cores, from this
bead's worktree. Fifth bead in the donor family (ranger-base-8z410 →
ranger-base-zcj0z → ranger-base-fol4d → ranger-base-x78s5 →
ranger-base-x0ses → here) and the second SITE of the same class: a fixture
that runs a system utility's bytes under a basename that utility does not
own.

## What was red

`internal/posse/watchimage_qa_test.go`, two tests, in every ubuntu-latest
arm-1 CI job — deterministic there and green on every box whose `sleep` is a
standalone GNU binary, which is why it reached CI at all:

```
watchimage_qa_test.go:159: pid 164890: this fixture's stand-in loop is gone
  before the reading — the assertions below would be about a stranger
    (TestQAG18FiresWhenTheInstallRenamesOverALiveLoop)
watchimage_qa_test.go:390: pid 164816: (the same line)
    (TestQAALoopReadingCoversItsOwnInstanceAndNotItsSiblings)
```

The guard was right and was not the bug: the stand-in loop really was gone.
ranger-base-x0ses read the runner and the two candidate .debs and has the
whole mechanism; the short form is that ubuntu-latest is Ubuntu 26.04 now,
its `coreutils` is a 2.7 kB Architecture:all metapackage over
`coreutils-from-uutils`, and `/usr/bin/sleep` there is a SYMLINK into one
multi-call binary that decides which utility to be from the path the kernel
exec'd it through — that path's file STEM, with `argv[0]` ignored on purpose
(uutils `src/common/validation.rs`: "prevent `env -a` from bypassing
AppArmor, SELinux policies").

These tests' whole subject is an install RENAMING over the file a loop runs,
so the donor's bytes live at `<root>/posse` by construction. That asked the
donor to be a utility called `posse`; it answered `coreutils: unknown program
'posse'` and exited 1 a few milliseconds in. The child was alive long enough
for the start-time settle and the first read-time guard, and died inside the
multi-megabyte copy that follows — which is why both reds landed on a guard
AFTER the install and never before it.

## The donor rule, which is the whole fix

**The donor must run the same whatever its basename is, and it must not be
picked for its NAME.** Picking a system utility for its name is how this bug
was written twice: ranger-base-x78s5 hit it in `scripts/reap-test-tmp.sh`
with `/usr/bin/tee`, and no `argv[0]` spelling could fix it there either. The
next distro's build of the next utility is the next nine days.

So the donor is `bash`, the same answer ranger-base-x78s5 landed one layer
over — a standalone binary that dispatches on nothing, the shell this suite
already runs under, and one that HONORS its argv, which is where the loop's
life goes. There is deliberately no fallback to another utility: a box with
no bash SKIPS and says so, because a fallback would be the same choice made
again under pressure.

Three things changed shape with it:

| | before | after |
|---|---|---|
| the image | `exec.LookPath("sleep")`'s bytes | `rrgncDonor(t)` = bash's bytes, a `#!` wrapper refused |
| the life | `<copy> 1500`, an argv the donor had to interpret as a utility | `<copy> -c 'read -t 1500 -r _'`, an argv bash interprets as a shell |
| the block | a timer | a pipe nobody writes to, so the loop also dies the moment the test process does |

The pipe is strictly tighter than the timer it replaced: an orphan of a
`sleep 1500` outlived its suite by up to the per-package budget, and this one
cannot outlive the process that holds the write end at all. Both bounds are
still there; the timer is now the outer one. It burns no CPU in either shape
(MEASURED: state `S`, `00:00:00` of cpu time at +1s), so a leaked one was
never what the load guard's kill arm is for.

A `#!` wrapper is REFUSED and not worked around, which is ranger-base-x78s5's
ruling carried across: a shebang exec rebuilds argv and leaves
`/proc/<pid>/exe` reading the INTERPRETER, and that link is the one fact every
assertion in the file is taken from.

## The failures print observations now

ranger-base-x0ses's closing lesson was that two CI runs could not say whether
an exec had even taken, because the rig reprinted the variable it MEANT to
pass. The Go fixture was worse than that: it sent the loop's stdout and
stderr to `/dev/null`, so `coreutils: unknown program 'posse'` — the one line
that names the mechanism — was printed by nothing, in either CI run or on any
box. Every failure in the file now carries the donor on PATH, the copy that
was exec'd, both files' first two bytes, the argv, the `/proc/<pid>/exe`
reading and the loop's own captured output:

```
watchimage_qa_test.go:315: pid 3593346: this fixture's stand-in loop is gone before the reading — …
        postmortem, as observed:
          donor   <scratchpad>/mutantbash/bash (first two bytes "\x7fE", so a binary) (chosen by rrgncDonor)
          argv    ["…/001/posse" "-c" "read -t 1500 -r _"]
          /proc/3593346/exe unreadable — the loop is gone or is not ours
          the copy it ran, …/001/posse (first two bytes "\x7fE", so a binary): 2436736 bytes, mode -rwxr-xr-x, 57:211731045
          its own output: "coreutils: unknown program 'posse'"
          a donor that answers "unknown program" there dispatches on its own path's stem and cannot be this fixture's loop — see rrgncDonor
```

## Seven arms, two test binaries, one variable

MEASURED 2026-11-21. Both binaries are `go test -c` of the same tree; the
only difference between A and C is the fixture's donor choice. The mutant is
a faithful stand-in for the runner's `sleep`: a Go ELF that answers
`coreutils: unknown program '<the stem of its own path>'` and exits 1 for any
stem but `sleep`, and sleeps otherwise — planted first on PATH. It keys on
`argv[0]`'s basename, which in this fixture's exec is byte-equal to the
kernel's exec path, so the fixture breaks under either dispatch key without
the mutant reimplementing the auxv read (ranger-base-x0ses measured the same
of its own).

| arm | binary | donor on PATH | reading |
|---|---|---|---|
| A | pre-fix | the mutant as `sleep` | **both FAIL**, the CI line verbatim at the read-time guard |
| B | pre-fix | this box's real GNU `sleep` | both PASS — so the mutant is what reds A, and the box is what hid it |
| C | post-fix | the mutant as `sleep` | **both PASS** — the fix; it never looks at `sleep` |
| D | post-fix | this box's real `sleep` | both PASS, 0.05s |
| E | post-fix | a `#!` stub named `bash` first on PATH | both SKIP, naming the stub and why a shebang donor cannot serve |
| F | post-fix | a PATH with no `bash` at all | both SKIP, naming the absence and that no other donor is taken |
| G | post-fix | the mutant as `bash` — an ELF the chooser accepts and that then refuses to be `posse` | **FAIL** with the postmortem above, which is how a report that only runs on the failure path was seen red |

G is the arm worth keeping: it is the next donor surprise, whatever it turns
out to be, and it arrives as `unknown program` in one read rather than as a
verdict about staleness.

## The cold-exec door, because this fixture is exactly its class

These tests plant an executable and then exec it under a deadline, which is
ADR 0085's class, and the new shape execs TWICE as often as a reader might
assume (a loop, then the negative control's restart). So the door was typed:

```
make coldexec-check RUN='TestQAG18|TestQAALoopReading'
```

MEASURED 2026-11-21, green on all five arms with no `--warm` pre-flight
anywhere. Arm 1: `TestQAG18FiresWhenTheInstallRenamesOverALiveLoop` 5.06s and
`TestQAALoopReadingCoversItsOwnInstanceAndNotItsSiblings` 2.50s, from 4 cold
first execs held 2.5s each. The settle deadline is 10s and the door's hold is
2.5s against the worst 2.17s ADR 0085 measured on macOS, so the deadline has
headroom over the real cost rather than merely over this box's 1.07ms. The
other four arms compile and run nothing under this filter, as expected: these
fixtures are arm 1's.

## Where the class stands

ranger-base-x0ses's census is still the census, and it had already named this
as the last site: every `exec.LookPath("<a coreutils utility>")` in the tree
and every `command -v` of one under `scripts/`. With this landing, no site
runs a utility under a basename the utility does not own —
`queuecutover_undo_qa_test.go`'s `qcFlakyMv` keeps the name `mv`,
`upgradeverify_test.go` runs `pwd` at its own path, `gates.go`'s
`posseSupportsBdWrite` passes `true`'s own path, `boxloops_qa_test.go` holds
a child open with `sleep` at its own path, and `reap-test-tmp.sh`'s
`generic_rig` runs `$sleepabs` at its own path. A symlink at the copy's path
would also survive uutils and is still wrong here, for the reason the subject
gives: an install renaming over the file the loop runs.

The residual is the same class one donor along: nothing in the fixture
MEASURES that its donor runs under a foreign basename — it reads two magic
bytes and reasons about bash. A box whose `bash` is itself an applet of a
multi-call binary would land in exactly this bead again, and the only thing
standing in front of it is that the postmortem now names the mechanism in one
read instead of nine days. The cheap closure, if that box ever shows up, is to
turn the reasoning into a reading: copy the donor to a name no utility owns,
run it `-c 'exit 7'`, and require 7 before any product assertion — one exec,
and it covers both facts the loop needs (a foreign basename, and an honored
`-c`). Not done here because it is insurance against a donor nobody has
measured, and the diagnosis it would print is the diagnosis the postmortem
already prints.

Not closed here:

- ranger-base-bekle — the same file and another lane's. Until this donor fix
  lands that file cannot run green on ubuntu CI at all, so what bekle is
  about could not be observed there.
