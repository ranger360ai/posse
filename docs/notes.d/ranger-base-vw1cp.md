## What fills a tmpfs is inodes, and nothing was reading that column (ranger-base-vw1cp)

Filed from ranger-base-xpxo6, which lost arm-3 runs to it, and narrowed by
ranger-base-akk9f, which had already landed the EDQUOT half of the explainer
from the other direction. This bead owned two halves: the inode reading the
preflight did not take, and the temp HOME the suite leaves behind. The second
half turned out not to be what fills the table.

### The reading

MEASURED 2026-11-11 (linux 7.2.15, go1.27.3), at the moment four arm-2 tests
red on writes:

    df -i /tmp   1041640 of 1048576 inodes used, 100%, 6936 free
    df -h /tmp   80% used, 2.8G available

/tmp here is `tmpfs … size=14247372k,nr_inodes=1048576,…,usrquota`: a fixed
table of one million inodes over 14 GB, one inode per 14 KB, and a suite's
fixtures are nothing like that big. So this filesystem runs out of inodes
long before it runs out of bytes, and `scripts/test-times.sh`'s preflight —
whose whole job is to say "a red from this run may belong to the box" — ran
`df -kP` only (lines 185, 190, 1003), printed a number above its floor, and
let the arm walk into a wall it had no column for.

`df -iP` is on every box this script runs on and needs nothing installed,
which is what makes this a floor and not the quota probe akk9f left undone
for want of a `quota` binary.

**Read BY HEADER NAME, because the column order is not portable.** On linux
the `-i` table is Inodes/IUsed/IFree/IUse%/mount and IFree is field 4; on
darwin `-i` APPENDS iused/ifree to the BYTE table, so field 4 there is
Available kilobytes and ifree is field 7. Field 4 on both would print 2.8
million "free inodes" on a mac that are really 2.8 GB — and a wrong number
reads exactly like a right one, which is worse than no line. `LC_ALL=C`
because df translates its header. Table-driven over five real shapes:

    darwin APFS header      -> 4692187160
    linux tmpfs header      -> 6936
    btrfs `0 0 0 -`         -> (no reading)
    a df with no -i support -> (no reading)
    a translated header     -> (no reading)

The no-reading branch is not hypothetical on this box: `df -iP /home`, where
GOCACHE lives, prints `0 0 0 -`, because a filesystem that allocates inodes
as it goes has no table to be at the end of. A floor over that 0 would warn
on every run about a table that does not exist, so ifree and iused both zero
is read as "no reading" and the DISK line says so in words.

### The floor is measured

What one run NEEDS, not what it leaves. MEASURED 2026-11-11, each arm run
with `TMPDIR` pointed at a fresh empty dir so the count is that run's alone
(loadavg 2-5):

    go test ./...                  (arm 1)   8567 peak,    20 left
    go test -tags posse_arm2 ./internal/posse  12484 peak, 12205 left,  91.3s
    go test -tags posse_arm3 ./internal/posse   2955 peak,     7 left, 109.0s

The arms running one after another does not make `make test` the largest of
those: nearly all of arm 2's reading is still there when it exits, so arm 3
starts on top of it. One full run's high water mark is ~15200 inodes and
~12200 are still there when it returns. `DISK_FLOOR_INODES=65536` is that
times four — two or three concurrent suites, which this box routinely has and
which under akk9f's shared budget spend one table, plus the build cache's
regrowth where GOCACHE shares the filesystem (here it does not; elsewhere it
will).

**A warning and never a failure.** The bead asked for a failed preflight.
This script's charter is that a red belonging to the box is the bug it exists
to explain rather than to introduce, a floor is a claim about a box, and
akk9f had just declined to red on a quota for the same reason — so an inode
floor that exits non-zero would be the one thing this file refuses to do. The
warning is the loud part, and it names what holds a table.

**The ENOSPC explainer reads both columns now too.** A full inode table is
reported ENOSPC, and that block's advice was `df -h` and `du` on GOCACHE —
which answer that the box has room. That is exactly akk9f's failure one errno
over: an explainer wrong about its own box teaches the reader to stop reading
it.

### What actually filled the table, and it was not the temp homes

The bead's title says the suite leaks /tmp test homes until the inode table is
full. The first half is true and the second is not, MEASURED 2026-11-11 by
`du --inodes` over every top-level entry of /tmp — 1040109 inodes accounted:

    974889  (94%)  24679 leaked tmp.XXXXXXXXXX dirs
     41045         one session's own scratch tree
      7647  (0.7%)   427 posse-testhome-* trees (5.2G of bytes, this bead's
                       own earlier reading, over 425 of them)
      5485             3 posse-cache-fence-* trees
      2899            14 go-buildNNN dirs

The 427 temp homes the bead was filed for are 0.7% of the table. The producer
is `scripts/audit-silent-reverts.sh --self-test`, which plants its fixtures
with five bare `mktemp -d` calls and removes none: 17 dirs and 697 inodes per
run, MEASURED, and internal/posse's silentrevert pins run it 19 times under
`-tags posse_arm2` — so one arm-2 run leaves 310 dirs and 12205 inodes. The
leaked dirs are in exactly that script's three shapes (15905 holding `r`, 7347
holding `index`, 1439 holding its 11-shape corpus), in the ratio one run
leaves, and an mtime histogram shows exact multiples of 310 per hour.
Filed as ranger-base-b27pn, not fixed here: different file, its own 17-arm
self-test.

Two things that reading corrects for anyone who repeats it:

- **`go test` does NOT redirect a test binary's TMPDIR.** MEASURED on this
  box: the binary, and a child's `mktemp -d`, both land in the OUTER TMPDIR.
  akk9f's note says cmd/go hands the test binary GOTMPDIR as its own TMPDIR;
  whatever that measured, an outer `TMPDIR` is not overridden here, which is
  why the pins' fixtures reach the real /tmp — and why moving `TMPDIR` moved
  all of it for these measurements.
- **the temp homes leak from a GREEN run**, not from killed ones only. A
  green arm-2 or arm-3 run leaves TWO, each holding the TestMain fixture and
  nothing else, because this package re-execs its own test binary and the
  holder children exist to be `child.Process.Kill()`ed — SIGKILL, which no
  handler can see. So the signal handler the bead offered cannot work. The
  design question is ranger-base-glogz, with both shapes priced.

What landed here for that half is what a process can do for itself:
`herdr_test.go`'s removal is checked, loud on stderr when it fails, and
reached from the two setup failures that used to exit after making the
directory. It does not touch either measured shape, and the comment on it
says so.

### Nothing on the box can tell residue from a live tree

Still true after this bead, and it is why clearing /tmp stays the operator's
call: nothing inside a `posse-testhome-*` names the worktree, the pid or the
run that made it. A stamp file is the input a reaper would need, and inventing
it without the reaper is half a mechanism — so it is in ranger-base-glogz and
not in this diff.
