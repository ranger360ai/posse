# The self-test that filled the inode table, and the trap that was one line (ranger-base-b27pn)

2026-11-11, Linux 7.2.15 x86_64, go1.27.3, 16 cores, from this bead's
worktree. From ranger-base-vw1cp, which went looking for what fills /tmp's
inode table and found this instead of the leak its own title names.

`scripts/audit-silent-reverts.sh --self-test` plants seven fixture repos and
four more for the ADR 0054 triage arms, each in a bare `mktemp -d`, and removed
none of them: no trap, no `rm`. The fix is the one
`scripts/test-times.sh`'s own `self_test` already had — one scratch root, one
`trap 'rm -rf …' EXIT`, every fixture dir a child of it — and it is 28 lines of
which 23 are comment.

## The bill was the pins', not the script's

A script that leaks into the TMPDIR of whoever ran it is a nuisance. What made
this a P1 is that the caller is a test binary and nobody redirects it:

- `internal/posse/silentrevert_qa_test.go` (build tag `posse_arm2`) runs
  `--self-test` 19 times, through `srAudit`, whose `srEnv()` passes TMPDIR
  straight through.
- `go test` does NOT give a test binary its own TMPDIR. MEASURED on this box,
  go1.27.3 linux: the binary, and a child's `mktemp -d`, both land in the OUTER
  TMPDIR. So every one of those 19 runs leaked into the real /tmp.

MEASURED here, one run with TMPDIR pointed at a fresh empty dir:

    TMPDIR=<fresh> bash scripts/audit-silent-reverts.sh --self-test
    -> exit 0, 17 top-level dirs, 697 inodes left behind

and over the whole pin set in that file (`-run 'SilentRevert|Audit'`, all 25
tests, `-tags posse_arm2`), with HEAD's script and TMPDIR pointed at a fresh
empty dir:

    before: 310 top-level entries, 12200 inodes   (all 310 `tmp.XXXXXXXXXX`)
    after:  0 entries, 0 inodes

The before figures are the bead's 310 and 12205 for a whole arm-2 run, within
five inodes, which is the other thing this measurement says: **those 25 tests
are the entire producer.** The two other `_test.go` files that name this script
read its source or run `test-times.sh`, neither of which plants anything.

ranger-base-vw1cp's census of /tmp put 974889 inodes in 24679 such dirs — 94%
of a tmpfs mounted `nr_inodes=1048576`, in exactly the three shapes this script
plants, with an mtime histogram showing exact multiples of 310 per hour. By the
ratios, ~1439 self-test runs' worth of corpus dirs, i.e. ~80 arm-2 runs.

## The box was in the failure state while this was being fixed

MEASURED 2026-11-11 07:2x local, before any of this landed:

    df -i /tmp -> 1048576 inodes, 1042372 used, 6204 free, IUse% 100%

and the first `go test` of this session died on it:

    /usr/lib/go/pkg/tool/linux_amd64/link: mapping output file failed:
        disk quota exceeded

That is the cost clause, observed rather than argued: a full inode table stops
a seat's suite at the LINKER, with an errno that names a quota and not a count,
so the next reader looks for a per-user quota (ranger-base-akk9f's errno) and
not for 24679 abandoned fixture repos. Every measurement above, and the full
`make test`, was therefore taken with TMPDIR relocated — stated because it is a
departure from how the suite normally runs, not because it changes what was
measured.

**And relocating TMPDIR is its own trap — a trap this repo had already
measured, written down, and arranged to print, and which still cost this bead
two suite runs.** The record is `docs/notes.d/ranger-base-akk9f.md`, dated the
same day, and `scripts/test-times.sh`'s own disk explainer (lines ~455-487),
which prints it to any seat whose run breaks on disk. Both halves of what I
then rediscovered the hard way:

- **TMPDIR and not GOTMPDIR** (ranger-base-krra). `cmd/go` hands the test
  binary GOTMPDIR as its own TMPDIR, so setting TMPDIR alone moves the build
  dir and every `t.TempDir` together, and setting GOTMPDIR is the trap. I set
  both, which is why run 2's failures name the GOTMPDIR path and not the
  TMPDIR one.
- **A relocated root reds a class of pins that has nothing to do with the
  diff**, for two reasons: the fixture lands outside every granted root (a bare
  /tmp is in every rendered seatbelt), or the surface abbreviates `$HOME` to
  `~` while the assertion wants the absolute path. The handling is written
  down too: re-run those on the house default, where they pass.

So: run 1 put TMPDIR under `$HOME` and red 11 tests in 3 packages; run 2 moved
to `/var/tmp` and red 5, all of the granted-root half; one `go test -run` of
those 5 on the house default was ok in 0.161s, which is the prescribed reading
and is what says they are the relocation's artefact. akk9f names 7 of the 11.
The other 4 are the same class and are recorded on ranger-base-sjrjj — the
bead I filed from the reds before reading the record, then withdrew as a
duplicate, which is the part of this worth remembering: the repo had the answer
and I did not look. The tell is worth seeing once, because the message denies
the thing it is printing — secondstore_qa_test.go:78:

    the line is missing "/home/posse/.cache/.../003/.beads":
        second store: ~/.cache/.../003/.beads holds beads.db, ...

## Two arms, because "the dir is empty" is an absence

`TestAuditSelfTestLeavesNoTempDirs` (arm 2) reads a TMPDIR it hands the script
and requires nothing left in it. On its own that is the class this file already
carries two pins against (ranger-base-z4vx): a reading of zero over fixtures
that went somewhere the pin cannot see is indistinguishable from a reading of
zero over fixtures that were cleaned up.

So the WRONG ARM runs first, on a mutant whose `trap … EXIT` line is replaced
by `:`, and requires the leak BACK. That is what says the fixtures really land
in the TMPDIR this pin reads, and that the clean reading is the trap's doing.

The pin was shown to red on the real regression rather than only on the mutant:
restoring ONE of the five sites to a bare `mktemp -d` (`plant_numeric`'s) reds
it over a self-test that still exits 0 with every one of its own arms passing —
which is exactly the shape that let this ship.

## Verified

- `--self-test` output byte-identical before and after (`diff`, clean).
- Both failure paths still clean up and still exit right: the mutant that makes
  `plant_move` return 2 exits 2 and leaves nothing; the one that makes it
  return 0 exits 1 and leaves nothing.
- Eight concurrent `--self-test` runs on a healthy temp root: 8/8 exit 0,
  0 dirs left. The same eight of HEAD's script: 8/8 exit 0, **136 dirs left**,
  which is 8 x 17 exactly.
- `make fmt-check`, `make verify-shell-syntax` (67 scripts parse), `make
  tree-check` (50.9s at loadavg 3.7, both suite slots free).
- `make coldexec-check COLDEXEC_RUN="-run 'TestAuditSelfTestLeavesNoTempDirs'"`
  — all five arms green; the arm-2 leg holds 2 cold first execs at 2.5s each
  (the mutant copy and the script's own) and needs no `--warm` pre-flight.
- **The suite, with TMPDIR relocated and the relocation's artefacts re-read on
  the house default, which is what akk9f prescribes.** `make test` reds in arm
  1 and so never reaches arms 2 and 3; the arms were therefore run separately.
  - arm 1: 3 reds — `TestQATheRenderedProfileNamesTheMovedFileForEitherVariable`,
    `TestQARecordReachAbstainsBeforeItRendersTheProfile`,
    `TestQARecordReachAbstainsWhenThisProcessMayNotApplyAProfile`, all three in
    akk9f's named 8. On the house default: ok 0.031s.
  - arm 2 (**where this change lives**): `make test-arm2` green in full, exit 0,
    internal/posse ok 53.2s.
  - arm 3: 2 reds — `TestQAGrantStillReachesTheGitConfigInEveryShape` and
    `TestSeatbeltWritableReachesGoTelemetryCountersAndNotTheModeSwitch`, which
    are akk9f's "arm-3 pair" verbatim. On the house default: ok 0.021s.

## What this bead cost the box, which is the thing to not repeat

Arm 2 could not be read on the house default at all while this was being fixed,
and partway through neither could anything else: /tmp reached **0** free
inodes and the harness itself stopped being able to write a tool result.

The last of those inodes were mine. The eight-way concurrency CONTROL above —
HEAD's script, to show the flake was not the diff's — was typed without
relocating TMPDIR, so it planted its 136 dirs in the real /tmp, on a box
already at 99%. The irony is the useful part: the control measured the leak by
committing it. 702 inodes came back by deleting this session's own scratchpad
measurement dirs, which is all that could honestly be reclaimed.

The 136 are still there, with the other ~24679. At two minutes old they are
indistinguishable from a live sibling seat's fixtures — same three shapes, same
`tmp.XXXXXXXXXX` name, no holder identity to ask — and deleting one of those
reds that seat's suite. So: not swept, and the sweep stays the operator's
(ranger-base-vw1cp owns /tmp). **Relocate TMPDIR for anything that runs this
self-test, including the control that proves a failure is not yours.**
