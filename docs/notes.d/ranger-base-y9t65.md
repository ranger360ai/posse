# The reaper on a timer, and the four classes it had never heard of (ranger-base-y9t65)

*2026-11-11 · gilfoyle · from ranger-base-l23nb (the operator's accepted-risk
ruling on /tmp) · builds on ranger-base-32bdv (the reaper), ranger-base-b27pn
(the producer it was pointed at) and ADR 0089 (why its gates are shaped the
way they are)*

The operator accepted the /tmp risk and named the mitigation: run
`scripts/reap-test-tmp.sh` on a timer. Two things had to be true before a
timer was worth arming — the reaper had to reach the leaks that actually
happened, and it had to watch the number that actually ran out.

## What /tmp was holding

MEASURED 2026-11-11 08:4x, linux 7.2.15, /tmp a 14G tmpfs mounted
`usrquota` with `nr_inodes=1048576`, 103504 inodes used (10%) and 3.9G of
bytes (29%):

| top-level class | dirs | inodes |
|---|---|---|
| `claude-1001` (one root) | 1 | **44826** |
| `tmp.XXXXXXXXXX` | 9137 | 42261 |
| `posse-cache-fence-*` | 3 | 5485 |
| `Test*<digits>` | 49 | 3457 |
| `go-build<digits>` | 13 | 2644 |
| `node-compile-cache` (one root) | 1 | 2118 |
| `posse-race-*` | 164 | 974 |
| `posse-coldexec-*` | 27 | 27 |
| `posse-configdir-fence-*`, `posse-l3-probe-*` | 12 | 25 |
| `posse-testhome-*` | **0** | 0 |

The reaper took exactly one of those classes, and it was the one with
nothing left in it — ranger-base-l23nb's reclaim had already cleared it an
hour earlier. Everything that caused the near-outage was invisible to it.

Two numbers are worth saying out loud. The first is that `df -h` read **29%**
through the whole incident: the thing that ran out was the inode table, and
the byte gauge never moved. The second is `claude-1001` — the harness's
per-session scratchpad root is 43% of everything /tmp holds, larger than the
9137-dir leak that caused the outage.

## The classes, and the grace each one gets

`scripts/reap-test-tmp.sh` now carries a table rather than one glob.

| class | glob | grace | why that number |
|---|---|---|---|
| `posse-testhome` | `posse-testhome-*` | 60m | the original, unchanged from 32bdv |
| `posse-scratch` | `posse-*` | 120m | ours by name; 2x the longest `-timeout` in this tree (60m, the coldexec arms) |
| `go-build` | `go-build[0-9]*` | 120m | Go's `$WORK`; a running test binary's `/proc/<pid>/exe` names it |
| `go-testtmp` | `Test*[0-9]` | 120m | `t.TempDir()` with `TMPDIR=/tmp`, bounded by the same timeout |
| `mktemp` | `tmp.??????????` | 1440m | mktemp(1)'s DEFAULT template — a namespace we do not own |

**The graces are the gate; the holder scan is a best-effort negative.** ADR
0089 measured why: Go's `os.Setenv` does not rewrite `/proc/<pid>/environ`, so
a test binary that pointed `HOME` at a dir it made is invisible to the scan.
A shell's `d=$(mktemp -d)` is invisible one step over — the path lives in a
shell variable, which is not argv, not cwd, and not an fd. So each grace is
argued against the longest run that could still be holding one of that
class's dirs, and the scan only ever makes the answer more conservative.

ADR 0089's "Alternatives rejected" says a longer grace narrows the window
and does not close it, which is correct and is the reason the 24h `mktemp`
grace is written down as a cost rather than a design. For `posse-testhome-*`
the answer is the creator's flock (ADR 0089 D1-D2, ranger-base-z056i); the
`flock -n` step goes in the candidate filter this bead left marked for it.
No other class has a creator we can make take a lock, so for them the grace
is all there is.

`tmp.*` is the one that really wants the lock and will never get it. It is
mktemp(1)'s default name, so an operator's hand-made scratch dir in a
terminal left open is in that namespace too, with no bound at all. Hence 24
hours, and hence the pressure arm below, which is the only thing that
actually reaches a burst. ranger-base-i78k9 gets the six remaining bare
`mktemp -d` roots in `scripts/` out of that namespace.

## What the scan can see now

Widened from 32bdv's four named variables (`HOME`/`TMPDIR`/`RHQ_HOME`/
`RHQ_QA_HOLD_HOME`) to **every** value in the environ, plus `cmdline`, plus
`exe`, plus the cwd and fds it already read. Every path found is folded onto
the one top-level entry under a root that would contain it, since the
candidate set is `-maxdepth 1`.

Both additions earn their place on this box. A shell script's scratch dir is
not in its own environment, but the path it hands a child as an argument is
in that child's `cmdline`; and a running `go test` binary lives inside
`$WORK`, so its own `exe` names the `go-build*` dir. On the live box the
widened scan holds `/tmp/posse-coldexec.rWikxs` — a doored arm in flight —
which the old scan did not see at all.

MEASURED by a four-channel rig (scratchpad `reap-rig.sh`; every fixture
backdated three days, so age never explains a keep):

| fixture | held through | ARM 1, holder alive | ARM 2, holder killed |
|---|---|---|---|
| `posse-testhome-111` | `HOME` in a child's environ | HELD | collectable |
| `tmp.AAAAAAAAAA` | the holder's argv | HELD | collectable |
| `go-build999` | the holder's cwd | HELD | collectable |
| `TestHeldByAnFd123` | an open fd | HELD | collectable |
| `posse-race-dead`, `tmp.BBBBBBBBBB`, `go-build111`, `TestNobodyHoldsThis999` | nothing | collectable | collectable |
| `tmp.CCCCCCCCCC` | nothing, but inside its grace | kept | kept |
| `claude-1001`, `node-compile-cache`, `some-random-dir` | matched by no glob | kept | kept |

The first run of that rig had `tmp.AAAAAAAAAA` collectable in both arms. That
was the rig, not the script: `sh -c 'sleep 300' /dev/null "$path"` execs
`sleep` directly and the extra argv goes with it. A loop body instead of a
bare `sleep` keeps the process, and the channel then held. Worth recording
because an argv holder that quietly exec's itself away is exactly how this
channel would read as working while testing nothing.

## Inodes, not just bytes

Every run prints `df` for bytes and for inodes, before and after, and the
arms fire on whichever of the two is worse:

- at or above `--pressure-at` (70%), every class's grace drops to
  `--pressure-min-age` (60m) for that run, and the run says so. An explicit
  `--min-age` is the caller's number and is never overridden.
- if it is STILL at or above `--bead-at` (85%) after an `--apply`, a P1 bead
  is filed on the devops lane — at most once per `--bead-cooldown` hours,
  because a 15-minute timer filing a P1 every tick is a worse outage than
  the one it is reporting.

**The bead arm cannot just `cd` into the posse checkout.** On this instance
that checkout's `.beads` is a one-line `redirect` at a queue repo's store, and in
no-db mode the tracker resolves `$BEADS_DIR` else `$cwd/.beads` and reads no
redirect at all (ADR 0055) — so a chdir there with the variable shed would
file into the redirect stub, invisibly. Handing it a store-selecting flag
instead is the one thing AGENTS.md forbids (github issue 9). So the arm
resolves the store, then names the repository that OWNS it by
`scripts/prune-bd-relates-to.sh`'s three conditions, and refuses rather than
guesses.

MEASURED against a `cp -a` copy of the real store in a scratch repo pair of
the same shape (scratchpad `bead-arm-happy.sh`; nothing touched the live
store):

| arm | expected | result |
|---|---|---|
| over the line, from the redirecting checkout | one row in the OWNER, a stamp, a read-back | `filed ranger-base-r56gv`, rows 33 → 34 |
| again, inside the cooldown | nothing | `one was filed within the last 24h`, 34 → 34 |
| `--no-bead` | nothing | `--no-bead: not filing`, 34 → 34 |
| under the line (`--bead-at 99`) | nothing | silent, 34 → 34 |
| a checkout whose `.beads` resolves nowhere | refuse | refused by name |
| a `.beads` in a directory that is not a git toplevel | refuse | refused by name |

## The two classes it refuses, and why

- **`/tmp/claude-1001`**, 44826 inodes. Both gates fail in the unsafe
  direction. A Claude Code session is not a process: it idles between turns
  with nothing on this box referencing its scratchpad and resumes hours
  later, so the scan cannot tell a dormant live session from a dead one —
  and the dir's mtime does not move while it waits, so neither can the
  grace. It is also the harness's directory, not posse's; nothing here knows
  which session ids are live. ranger-base-4arlo carries it, with the one
  liveness signal that looks real (the session's own transcript under
  `~/.claude/projects`, whose mtime does move every turn).
- **`/tmp/node-compile-cache`**, 2118 inodes. A cache with a stable name, not
  a leak. It regenerates, it does not accumulate, and taking it buys the
  inode table nothing and costs a recompile.

Both are named in the script's header with these reasons, so the next person
to count inodes does not rediscover them.

## The timer

`scripts/reap-timer-unit.sh` renders `posse-reap-tmp.service` and
`posse-reap-tmp.timer` and **never enables them**, which is `posse systemd`'s
contract and ADR 0080 D5's rule, kept for the same reason. It is a shell
script and not a `posse` verb because nothing about it tracks a home:
`posse systemd` refuses outright unless a home says `autostart_via: systemd`,
and this timer is one per BOX, because the thing it sweeps is one shared
filesystem.

**Why 15 minutes.** Not from taste, and not from the steady leak rate.

- The reap costs **2.1s** wall over 9689 top-level entries and 61 processes
  (MEASURED 2026-11-11; the pre-widening script was 1.75s over the same box).
- The steady rate does not set the cadence. MEASURED over /tmp's own mtimes:
  465-928 `tmp.*` dirs per hour, ~4.6 inodes each, so 2100-4300 inodes/h
  against 945k free — 220 to 450 hours of headroom. A daily sweep would hold
  that.
- A BURST sets it. ranger-base-b27pn priced ONE arm-2 run at 310 dirs and
  12200 inodes, and five seats run suites concurrently here, so the worst
  case is ~183000 inodes/h. From the 70% pressure line (734k) to a full
  table (1048576) is 314k inodes: **103 minutes**.
- During a burst what matters is how soon the pressure arm notices and drops
  every grace to 60m, because nothing else reaches dirs that young. 15
  minutes puts six or seven checks inside that 103-minute window, at 2.1s
  each — a duty cycle of 0.23%.

**Why it is off the dispatch loop**, which took four things and not one:

- Its own unit, called from nothing, with no `After=`/`Requires=` on the
  dispatch units. A reap needs no multiplexer, which is the whole reason the
  dispatch unit is ordered behind one.
- `RandomizedDelaySec=2min` is load-bearing, not politeness. The two homes
  here pass at 5m (ranger) and 3m (weller), and **15 is a multiple of both**
  — an unrandomized 15-minute timer would start on a dispatch tick every
  single time it fired, which is the one thing the bead asked against.
- `Nice=10` and `IOSchedulingClass=idle`, so the overlaps that do happen
  yield to the pass and to anybody's suite.
- `Type=oneshot` with no `Restart=`. A failed reap has failed and the next
  one is 15 minutes away; a sweep that retries on a shared filesystem is how
  a bad run becomes a bad hour.

`systemd-analyze --user verify` exits 0 on both rendered units with no
warnings.

**The renderer refuses `--write` from a session worktree.** Every path it
renders is resolved from where the script sits, which is right for an
operator running it out of the shared checkout and wrong for a dispatched
session:
the worktree is retired once its bead closes, and the unit would outlive the
directory it names and fail every interval with `No such file or directory`.
Rendering to stdout from a worktree still works, because that is how it gets
reviewed.

**ExecStart names the script in the checkout, not a promoted copy.** So a
landing that changes `scripts/reap-test-tmp.sh` changes what the timer runs,
with no promote step — unlike the dispatch unit, which names a promoted
`~/.local/bin/posse`. That is the trade, taken deliberately: a frozen copy
under `~/.local/libexec` would need a promote verb nobody has and would go
stale silently, which on a sweep tool means the box quietly keeps running the
version whose gates were wrong. `--script PATH` renders against a frozen copy
for an operator who wants the other trade.

## What the operator types

Nothing is written and nothing is enabled. After this branch lands on `main`:

```
cd <the shared posse checkout>
./scripts/reap-timer-unit.sh                     # read the two units
./scripts/reap-test-tmp.sh --root /tmp           # dry run: what it would take
./scripts/reap-timer-unit.sh --write             # put them in ~/.config/systemd/user
systemctl --user daemon-reload
systemctl --user enable --now posse-reap-tmp.timer
systemctl --user list-timers posse-reap-tmp.timer
systemctl --user start posse-reap-tmp.service    # read the first run back
journalctl --user -u posse-reap-tmp.service -n 40 --no-pager
```

Blast radius, every 15 minutes once enabled: an `rm -rf` of top-level
directories under `/tmp` that match one of the five class globs, are owned by
uid posse, are named by no live process, and are older than their class's
grace. It never descends, it never touches a file, and it writes at most one
bead a day.

The dry run against the live box at the time of writing: **2602 dirs, 1342M,
21139 inodes** collectable, in 2.1s.

## Stood down

`systemctl --user disable --now posse-reap-tmp.timer`. The script's default
is unchanged — a dry run, with `--apply` the operator's.
