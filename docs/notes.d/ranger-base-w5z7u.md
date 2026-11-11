## The coldexec arms could not fit in the box, and the box has a number (ranger-base-w5z7u)

Filed from ranger-base-4kars, whose five-arm census tripped it.

`make coldexec-check`'s five arms, run back to back on 2026-11-11 from a seat
with `TMPDIR` unset, red with `disk quota exceeded` — on the TESTS and not on
the door. What the seat read was a hundred unrelated red test names and no
statement about the class the door exists to measure. MEASURED that day (linux
7.2.15 x86_64, go1.27.3, D=2.5s), per arm, FAILs and the lines naming a space
or quota error:

| arm | FAILs | quota/space lines |
|---|---|---|
| 1 (internal/posse) | 4 | 2 |
| 2 (`-tags posse_arm2`) | 2 | 7 |
| 3 (`-tags posse_arm3`) | 1 | 0 |
| cmd/posse | 112 | 122 |
| internal/treepins | 12 | 15 |

The cmd/posse arm held **34 cold first execs against the 231** ranger-base-in8dr
measured for the same arm the same day: its tests died before they could plant,
so the door's own number went with them. That is ADR 0085 D5's case exactly —
a run that is not covered and does not say so.

### The number that binds is a quota, and it can be read

ranger-base-akk9f established the half of this that matters: `/tmp` here is a
14G tmpfs mounted `usrquota`, the quota is per UID, every seat on this box runs
as the same user, so it is ONE BOX-WIDE budget that concurrent suites spend
together while `df` reports a device with room throughout. akk9f left the
usage/limit reading out of scope — "there is no `quota` binary on this box and
a usage/limit reading is a bigger question than a caveat".

There does not need to be a binary. **`quotactl_fd(2)`** (Linux 5.14) takes an
fd of the MOUNT rather than a block device — which is the whole reason it works
here, since a tmpfs has no device — and the kernel lets an unprivileged caller
read its OWN uid's quota. MEASURED 2026-11-11, uid 1001 on `/tmp`:

    USRQUOTA /tmp uid=1001: curspace=9320214528 (8888 MB)
                            bhard=11397897 blocks (11130 MB)   -> 2242 MB left

against `df`'s 5026 MB free on the device at the same instant. Two numbers, and
the smaller one is the one a write hits. `/home` answers `ENOSYS` — btrfs with
no quota — which is the reading "the device is the only limit" and not "your
share is spent", and the two must not be confused: `quotaUnmeasured` is -1 and
never 0, or the door would refuse on every box that runs no quota at all.

### The share moves under you, and that is the point

MEASURED 2026-11-11, **fourteen minutes apart**, with this seat writing nothing
to `/tmp`:

| time | `posse-testhome-*` trees | share left |
|---|---|---|
| 08:46 | 428 (5.48 GB, 5.26 GB of it in three) | 2242 MB |
| 09:00 | 1 | 7425 MB |

Whether those three trees were live sibling suites or residue is precisely what
a seat cannot decide — a tree's ctime says when it was made, not whether its
run is alive — so this fragment does not claim they were abandoned, and the
door does not clear anything. What it does is READ the share at the moment the
arm starts. (The earlier reading is why the bead's description calls the trees
"accumulated"; the second reading is why that word is not in the fix.)

### The arms' own peak

Each arm run doored with a PRIVATE `TMPDIR` on `/var/tmp` — a quota-free 974G
btrfs here — and that directory sampled with `du -sk` every 15s, so the number
is THIS arm's footprint and not the box's. MEASURED 2026-11-11 between
08:43 and 09:30, with both box-wide suite slots held by these runs for most
of it and the one-minute load average reading 2.2-4.1 at the four points it
was sampled:

| arm | wall | cold execs held | notifications | **peak $TMPDIR** | quota/space errors |
|---|---|---|---|---|---|
| 1 `./internal/posse/...` | 955.7s | 706 | 131,235 | 58 MB | 0 |
| 2 `-tags posse_arm2` | 470.1s | 321 | 115,360 | 27 MB | 0 |
| 3 `-tags posse_arm3` | 566.1s | 598 | 84,479 | 44 MB | 0 |
| `./cmd/posse/...` | 644s | 226 | 8,368 | **381 MB** | 0 |
| `./internal/treepins/...` | 1618.4s | 609 | 32,949 | 61 MB | 0 |

(Wall is `go test`'s own package time where the arm printed one, and the
wrapper's otherwise. The `du -sk` sampler ran every 15s, so a spike shorter
than that is not in these numbers — the figures are a floor on the peak and
not a ceiling.)

Every arm came back with **zero** quota or space errors, against the 122 and
15 the cmd/posse and treepins arms produced on `/tmp` the same day — so the
arms are affordable on this box and what they are not is affordable on what
was left of the tmpfs. The two that reproduce ranger-base-in8dr's readings do
so closely: cmd/posse 226 cold execs against its 231 and treepins 609 against
608, treepins' wall 1618.4s against 1581.8s (2.3%).

**The largest arm's peak is 381 MB, and it is cmd/posse rather than any
internal/posse arm** — which is the opposite of what the `posse-testhome-*`
trees suggested, and worth saying because it is what a reader would guess
wrong. cmd/posse's tests copy the working tree per test; internal/posse's
three arms share ONE test home and peak at 27-58 MB between them.

Three of the five exited nonzero, every red one of the class in "Why the arms
were NOT simply given a TMPDIR with room" below plus
`TestQAWaitLegErrorReachesThePassOutputOnEveryMultiplexer`, which is
**ranger-base-s14uz** and reproduced here under the door. None of the five
red on a write.

So the arms are not what fills a 14G tmpfs; they are what cannot fit in what is
left of it. `COLDEXEC_TMP_FLOOR_MB` is **2048**, which is 5.4x that 381 MB —
derived the way `scripts/test-times.sh`'s `DISK_FLOOR_MB` is (903 MB measured,
5120 set, 5.7x) and deliberately a slightly smaller multiple, because that
floor's margin covers a regrown go build cache as well as concurrent suites
and this one does not have to: `GOCACHE` here is `~/.cache/go-build`, on
`/home`, which has no quota and 888 GB. What is left is the concurrency, and
that part is a claim about a shared box rather than a measurement — which is
why it is one number in one place and overridable.

### What landed

- **`cmd/coldexec` reads the room** (`room_linux.go`): `-min-temp-mb N` reads
  the binding number for `$TMPDIR` — the quota share where the filesystem has
  one, free space otherwise, whichever is smaller — prints it, and refuses with
  **exit 2** below the floor. The same refusal the door already gives an
  architecture its filter does not name or a kernel with no user notification,
  and for the same reason (ADR 0085 D5). It is off when nothing asks for it, so
  the supervisor's other callers are untouched.
- **Each arm takes its own temp directory and drops it**: `$(COLDEXEC_TMP)`
  mkdirs one UNDER `$TMPDIR` — under `/tmp` when that is unset — and a trap
  clears it however the arm ends. internal/posse's `TestMain` removes its home
  after `m.Run()`, so a run killed by a timeout, a panic or a signal does not;
  this means the arms stop contributing to that. A seat that has already moved
  its `TMPDIR` keeps it: the arm nests inside rather than overriding.
- **A `RUN=` filter drops the floor**, for the same reason it takes no suite
  slot: the floor is what a whole arm needs, and refusing the one form a bead
  types all day on a short box is a false refusal and the fastest way to teach
  a seat to pass `-min-temp-mb 0`.
- Pins: `TestQAEveryColdexecArmReadsTheRoomAndTakesItsOwnTempDir`
  (internal/treepins, `make corpus-check`) holds both halves on every arm line
  AND holds the two Makefile variables to still saying what they say — a pin
  that only checked the arms NAME them is one that `COLDEXEC_ROOM :=` emptied
  to nothing passes. `cmd/coldexec/room_linux_test.go` holds the refusal (exit
  2, nothing ran, and the number on the screen), the check being off by
  default, the reading being taken at `$TMPDIR`, and the two readings'
  precedence.

### Why the arms were NOT simply given a TMPDIR with room

Because that buys the room and costs a class, and the class is other people's
pins. MEASURED 2026-11-11, deterministic over two runs each, with `TMPDIR` on
`/var/tmp` (neither `/tmp` nor under `$HOME`, so neither of akk9f's two
workaround classes applies):

    arm 3  TestQAGrantStillReachesTheGitConfigInEveryShape/deniesFiles
           "premise gone: <fixture>/003/repo/.git/config is in no grant,
            so denying it measures nothing" — and the grant set it prints
            ends in a bare /tmp
    arm 3  TestSeatbeltWritableReachesGoTelemetryCountersAndNotTheModeSwitch
    arm 1  TestQARecordReachAbstainsBeforeItRendersTheProfile
    arm 1  TestQARecordReachAbstainsWhenThisProcessMayNotApplyAProfile

All four pass with `TMPDIR` anywhere under `/tmp`, including a fresh
`mktemp -d /tmp/...` — which is why the fix nests there and buys no space.
`seatbeltWritable` adds a bare `/tmp` to every rendered profile
unconditionally, and these pins put their fixture in a `t.TempDir()` under the
OUTER `TMPDIR`; where that is `/tmp`, the premise they need is supplied by the
environment rather than by the test, and the assertion is vacuous. akk9f ruled
that "NOT a product bug", which is right about the product and is the reason
it is filed as a TEST defect: **ranger-base-u8xk4**.

### Running it today

The floor refuses on a box whose share is already spent, which is the feature
and is also what a seat will meet first. `make coldexec-check RUN=<filter>` is
unaffected; an unfiltered run on a short box prints the two numbers and stops
in milliseconds instead of at the end of an hour.
