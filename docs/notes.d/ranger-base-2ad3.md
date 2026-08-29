## The timeout verified by running it, and a second red the run found (ranger-base-2ad3)

This bead is the third of three filed the same day on one invariant, and it is
the one that closes last. It asked for two things and they were never the same
decision:

> (a) name a timeout in the Makefile, which stops the silent red and keeps the
> slowness; (b) find where internal/rhq spends 511s and cut it. […] (a) is a
> one-line change and should probably happen regardless so the failure mode is
> legible; (b) is the actual bead.

**(a) landed elsewhere and is not re-litigated here.** `c866819`
(ranger-base-2ggb) put `-timeout 25m` on the recipe with
`suitetimeout_qa_test.go` holding it above a 15m floor; `32ce0e9`
(ranger-base-7xla) wrapped the recipe in `scripts/test-times.sh` so the
per-package seconds are printed on every run instead of arriving one day as a
panic. The pooled distribution is `docs/notes.d/ranger-base-2ggb.md`, the
reporter's sixteen mutants and the linux/amd64 margin are
`docs/notes.d/ranger-base-7xla.md`. **(b) is `ranger-base-i7fa`**, open with
dinesh, carrying both sessions' measurements and the finding that the blocker
is one helper (`newTestBackend`, 575 call sites) rather than 56 scattered
files.

### THE VERIFICATION, WHICH IS WHAT THIS BEAD ADDS

Read is not run. `make test` at `44a4143`, darwin 25.4.0 / go1.26, 8 cpu,
loadavg 11-16 throughout, `$RHQ_GATES_DIR/bin` stripped from PATH:

    ok  github.com/ranger360ai/posse             106.175s     7% of budget
    ok  github.com/ranger360ai/posse/cmd/posse    71.773s     4% of budget
    ok  github.com/ranger360ai/posse/internal/rhq 636.076s    42% of budget
    real 665.45   user 233.92   sys 259.53
    test-times: 3 package(s) timed, 1 over the 300s line

**636.076s is a new worst reading, and it is past go's default.** Every prior
standalone number sat between 484.6s and 623.2s; this one clears 600s by 6%
*under `./...`, with the two short packages competing for the same cores* —
the exact configuration that produced the 600.8 / 601.0 / 601.1 timeout
panics before the flag. This run was green only because the flag exists. The
25m ceiling is not margin the suite happens to have; on this box it is now the
only reason a clean tree reports clean. Six standalone-class readings in, the
number chosen without this one is still the right number, and the reporter
called the package slow, by name, without failing on a wall clock.

The CPU columns are worth keeping beside i7fa's case: 233.92s user + 259.53s
sys over 665.45s wall is **0.74 of one core on eight**, for the whole suite.
ranger-base-7xla measured 0.67 over a 136.2s slice of internal/rhq alone. Two
sessions, two scopes, the same answer — the wall is `git`, `sandbox-exec` and
the fake-herdr re-exec, not computation. Whatever i7fa does, it is not
competing for CPU it needs.

### AND THE RUN FOUND A SECOND RED, WHICH IS NOT THIS BEAD'S SUBJECT

`make test` still exited 1. Not a test: `scripts/audit-silent-reverts.sh`
reported `bd687f4` untriaged, and had done so at HEAD for every persona since
that commit landed — the same shape this bead is about, a red belonging to
nobody's diff and landing on whoever ran the suite. Triaged rather than
deferred, because a gate that is red for everyone teaches people to read past
it.

It is a false positive by construction. `bd687f4` deletes
`scripts/__pycache__/bd-argv-gate.cpython-314.pyc` — a build artifact
`916b5f9` committed by accident — and in the same commit gitignores
`__pycache__/` and `*.pyc` and passes python `-B`, so the blob is regenerated
by any run and unversionable from now on. Verified at HEAD, not asserted:
`git ls-files scripts/__pycache__/` empty, the directory absent on disk, both
parser and verifier present. The reason is now in `scripts/silent-reverts.allow`,
and both wrong arms were checked — mutate the sha by one character, or drop
the line, and the audit fails again naming `bd687f4`.

One trap recorded there for the next reader: `bd687f4`'s own commit message
credits `73e38c9` with committing the `.pyc`. It did not. `git log
--diff-filter=A` on the path names `916b5f9`, which is what the audit prints
too; chasing the sha in the message finds nothing.
