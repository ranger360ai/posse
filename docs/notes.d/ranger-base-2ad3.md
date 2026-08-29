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

### A SECOND READING, AND THE MARGIN NOBODY HAD MEASURED

Run 1 was taken at `44a4143`; two commits landed on main while it ran, so it
was re-run at the rebased HEAD `8237db9` with the box under real fleet load
(loadavg 14 rising to 29 — roughly double run 1's):

                       run 1 (load 11-16)   run 2 (load 14-29)    delta
    internal/rhq            636.076s            748.969s         +17.7%
    posse                   106.175s            176.011s         +65.8%
    cmd/posse                71.773s            107.187s         +49.3%
    make test, wall          665.45s            761.76s          +14.5%
    user + sys               493.45s            489.27s           -0.8%

**The CPU is invariant.** Same tree, same work, double the load, and the
user+sys total moves by less than one percent while the wall moves 15-18%.
That is the "internal/rhq is not CPU-bound" claim measured against a control
rather than inferred from one slice — and 493s of CPU over 665s of wall on
eight cores is 0.74 of a single core, agreeing with ranger-base-7xla's 0.67
over a 136.2s slice.

Two things follow, and the second is the one worth keeping:

- **748.969s is the new worst reading**, 25% past go's 600s default. Two runs,
  two regimes, both would have been timeout panics without `-timeout 25m`.
- **The ceiling is robust to load, and that is the point.** At double the
  loadavg the slowest package reached 49% of the 25m budget. Load costs ~18%;
  it does not cost 150%. So what will eventually trip this ceiling is not the
  box's mood — it is *growth*, which is exactly the thing `test-times.sh`'s
  300s line was put there to watch and the thing a bigger `-timeout` would
  hide. 25m is not under-specified and does not want raising again.

### THE RED IN RUN 2 IS ranger-base-ci9e, AND IT IS WORSE THAN FILED

Run 2's `internal/rhq` was FAIL, not timeout, on one test:
`TestQALateExplainErrorStillFailsLoudlyNamingTheGuess` (1.69s). That is
`ranger-base-ci9e`, open with dinesh, which already diagnoses the mechanism:
`verify_nx85_qa_test.go:41` builds an ordering out of three wall clocks, and a
fixed 700ms sleep does not slow down when the dispatcher's window does.

The bead's own title says *"deterministically red on any box ~3x slower than
this one"*, and its evidence is emulated linux/amd64. **That understates the
exposure.** Measured here at `8237db9`, native darwin, no emulation, loadavg
~21, `go test ./internal/rhq -run ExplainError -count=3 -v`:

    FAIL 1.69s    PASS 1.20s    PASS 1.27s

Three consecutive runs of one binary. It is not a slow-box property: on this
box, under the fleet load this box actually carries, it is an intermittent at
roughly one in three, and the boundary sits near 1.5s (every pass observed
0.78-1.38s, every fail at or above 1.69s). An intermittent is worse than a
deterministic red — it lands on a random diff and is green on the re-run that
would have investigated it. Recorded on ci9e.

Which is the whole thesis of this bead arriving twice in one afternoon: a red
that belongs to the box and not to the commit. The timeout was one instance
and is closed; `bd687f4` was a second and is triaged; this is a third and it
already has a home.
