# Arm 1 ran no Go test on either platform for nine days, and both reasons were loud (ranger-base-zcj0z)

2026-11-21, Linux 7.2.15 x86_64, go1.27.3, 16 cores, git 2.56.0, from this
bead's worktree. From ranger-base-8z410, which verified four closes and found
this while looking for the venue one of them deferred its acceptance to.
`make test-arm1` carries eleven prerequisites and died at the seventh on
macos-latest and the ninth on ubuntu-latest, so no arm-1 Go test compiled on
either runner — MEASURED on runs 43731869157 (2026-11-15) and 44865299103
(2026-11-21), both legs, zero `ok ` lines in either arm-1 job.

**Both detectors worked.** Neither failure was a check that stayed silent;
each one printed its finding, in full, with its own diagnosis attached, on a
CI leg nobody reads. That is the whole of this bead: arm 1 was already red on
both platforms from 2026-11-11 for other reasons, and a job that is red for
one reason is a job whose next reason arrives unread.

## macos: the hazard verify-shell-syntax exists for, in the file it was written about

`scripts/shell-syntax.sh` exists because `scripts/verify-box.sh` keeps three
tables of English prose — ROSTER, EXCLUDED, UNTARGETED — and each one was a
quoted heredoc inside a command substitution. bash 3.2 scans `$( )` for its
closing paren while tracking quote state, and quoting the heredoc delimiter
does not stop that, so one apostrophe in a reason field opens a quote that
never closes and swallows the rest of the file. ranger-base-g4z8m measured
that in 2026-09 and wrote the sweep.

It happened again. 24bfa8d9 (2026-11-12, ranger-base-lydqn) added
`scripts/reap-test-tmp.sh's claude-scratch gating arms` to the exclusion
table — three apostrophes in one row, so an odd count — and from that commit
on every macos arm-1 job reported:

```
FINDING scripts/verify-box.sh (bash)
  .../scripts/verify-box.sh: line 422: syntax error near unexpected token `;;'
  .../scripts/verify-box.sh: line 422: `    */*) [ ! -x "$root/$1" ] ;;'
make: *** [verify-shell-syntax] Error 1
```

Line 422 is a `case` arm nobody had touched, which is exactly what the sweep's
own closing NOTE tells its reader to expect.

THE FIX is the one that NOTE prescribes: all three tables are
`IFS= read -r -d '' VAR <<'EOF'` now, with a `VAR=${VAR%$'\n'}` after each to
keep the trailing-newline semantics the substitution gave for free. There is
no `$( )` left for the hazard to live in. `--census` output is byte-identical
across the change for all 36 rows except the three whose prose this landing
edited (MEASURED 2026-11-21, `diff` against the same script at HEAD).

AND THE FOUR SCARRED PHRASES HAVE THEIR GRAMMAR BACK, which is what makes
this the regression test rather than a tidy-up: "this repo own seed PIDs",
"the scripts own --self-test", "herdr-bob own rules.json" and "upstream
shipped file" were de-possessified by hand by whoever met this hazard once and
worked around it without writing it down. The exclusion table now carries an
ODD number of apostrophes on purpose. A table that parses with one in it is
the proof.

Two tracked files still carry the shape with an even apostrophe count —
`scripts/reap-timer-unit.sh` and `docs/adr/0096-proc-spellings.probe.sh` — so
the class is still the sweep's, and the sweep is still the only thing that
knows when an even count stops being even. Filed: ranger-base-fol4d.

## ubuntu: a rig that said "I am broken" and threw away the reason

`scripts/reap-test-tmp.sh --self-test`'s finding-4 arms plant a copy of
`tee(1)` at a path ending in `/claude`, hold its stdin open with a fifo, and
assert that a `-p` run is excluded from `claude_procs` while a non-print run
is counted. Both arms assert the fixture is ALIVE at the moment of the census,
because ranger-base-bttuj found the old spelling passing over a fixture that
had already exited.

On ubuntu-latest both arms report the fixture gone, on every run since the rig
landed:

```
  FAIL the '-p' fixture (pid 10812) was already gone when the census ran …
  FAIL control: the non-print fixture (pid 10998) was already gone …
```

It is not a flake and it is not load: the control arm fails too, so argv is
not the variable, and the same rig is green here 8 of 8 (header of that
script) and re-read green 2026-11-21, both arms LIVE, with the shape also
reproduced standalone outside the script. So the cause is something about that
runner — and the one place it could have been read was `2>&1` to `/dev/null`.

NOT DIAGNOSED, AND SAYING SO: three different defects produced the identical
log line. An exec that cannot happen (a `noexec` mount, a donor that
dispatches on `argv[0]`, a loader error) exits 126 and says so on stderr; a
donor that rejected its argv exits 1 or 2; a fixture that read EOF on the fifo
exits 0. This landing does not guess between them. `fake_postmortem` prints
the fixture's exit status, its stderr, the donor path, whether the copy is
executable and whether the fifo is a fifo, so ONE further CI reading names
which it is. Proved by two mutants of the tracked script, run from a copy
(`rejects-argv`, donor swapped to `sleep`; `cannot-exec`, `chmod -x` on the
copy):

```
postmortem: it exited 1 (126 could not exec, 127 not found, 0 stdin EOF, else
  it rejected its argv); donor /usr/bin/sleep; the copy … is executable; the
  fifo is a fifo
  its stderr: …/claude: invalid option -- 'p'

postmortem: it exited 126 (…); donor /usr/bin/tee; the copy … is NOT
  executable; the fifo is a fifo
  its stderr: …/claude: Permission denied
```

TWO THINGS THE FIRST DRAFT GOT WRONG, both caught by those mutants and
neither by reading:

- `reap-test-tmp.sh` is `set -euo pipefail`, so `wait "$pfake"; frc=$?` on an
  exited child ended the self-test silently at rc 1 — which looked exactly
  like the rig failing one arm earlier. Every status in that function is taken
  with `|| true` or inside a condition now, and the comment says why.
- `internal/treepins/ambientcensus_qa_test.go` finds the fixture arms by
  taking the first function in that region which both says `kill` and names a
  planted pid as the arm's END. A `kill -0 "$pkeep"` in the postmortem made
  the postmortem that end, cut both spans short of their `case`, and red
  `TestQAEachFindingFourFixtureArmGradesMembershipOfItsOwnPlantedPid` over a
  diagnostic. The function signals nothing at all now; the exit status already
  carries what the keeper's liveness would have said, since a fixture that
  read EOF exits 0.

## the darwin pin, which cannot be read from a linux box

ranger-base-8z410 finding 2: `TestQAWorktreeCommitLeavesNoStrayPackedRefsLock`
(`internal/posse/seatbeltworktreegit_qa_test.go`) is red on macos from
9193d3b1, and its `t.Fatalf` takes the ADR 0059 D2 battery below it with it.
The red is `wgPack`'s vacuity guard: after `git pack-refs --all` the directory
`<common>/refs/heads/posse` is still there. The arm skips on every box without
`sandbox-exec`, so a macos CI leg is the only reader there is, and this seat is
linux.

The bead's own discriminator was a thing to type on a darwin box: a leftover
`<branch>.lock` in that directory implicates the new `<common>/objects` deny
(a lock a sandboxed git can create and not unlink — a real defect in the
wall); an EMPTY directory implicates git 2.56.1's pack-refs and makes it the
guard's problem. The guard's message named the directory and not its contents,
so it could not be told apart from the only log that prints it.

It prints the contents now (`wgListing`), with the reading to make and both
verdicts spelled out in the refusal itself, so the next macos run answers the
question without a darwin seat. Exercised on all three branches here — empty,
two entries, unreadable — by a probe test in-package that was then removed.
The pin itself is still red on darwin and is NOT fixed here: ranger-base-blkse.

## what is not closed here

- ranger-base-blkse — the darwin red itself, which needs the reading above.
- ranger-base-fol4d — the ubuntu fixture's actual cause, once a CI run has
  printed the postmortem, plus the two files still carrying the heredoc shape.
