# ADR 0096 — Nesting is not a property of the bwrap seat wall

*Status: accepted 2026-11-14 (ranger-base-rwet4) · owner: architect · amends
[ADR 0080](0080-posse-on-an-always-on-linux-host-rex-first.md) D14's B7
bullet and the B7 entry in its "Measured versus assumed" block · leaves D17's
render as it is*

## Context

ADR 0080 D14 says "a runtime's own bwrap must still nest inside ours (probe
B7)", and its measured block says the 2026-11-08 host run retired that
assumption. The run did not measure the wall posse ships. B7 ran on
`0080-linux-cage.probe.sh`'s `$BW`, which carries `--proc /proc`. D17's
render deliberately leaves `--proc` out. So inside a seat, `/proc` is the
host's procfs under the read-only `--ro-bind / /`. A nested user namespace
cannot write its own `uid_map`, and every nested bwrap or `unshare -r` fails
before it reaches a mount. This was MEASURED 2026-11-13 inside a live caged
seat, in the in-flight change listed under *In flight* below. Three of the
wall's own live pins nest a bwrap, so inside a caged seat one of them FAILS
and two report `NOTHING MEASURED`.

Separately, that in-flight change puts the runtime of a bwrap seat under a
Landlock filesystem domain. The kernel refuses `mount(2)` to any process
under such a domain, whichever right it handles (ADR 0080 D13 records the
same property). Once that change lands, nothing forked from the runtime can
nest a bwrap, whatever `/proc` is.

**Measured.** 2026-11-14, D1 host, Linux 7.2.15, bubblewrap 0.13.1,
`ptrace_scope=1`, run uncaged by the architect. The outer wall is D17's base
plus `--unshare-user`, one grant and one deny inside it. Four `/proc`
spellings were tried:

| arm | nested `bwrap` / `unshare -rm` | host pids seen; `kill -0` same-uid host peer | `/proc/<host pid>/root` | deny: umount / remount rw from a nested ns |
|---|---|---|---|---|
| A: shipped (no `/proc` flag) | refused: `setting up uid map: Read-only file system` | ~400; allowed | EACCES | not reachable (no nested ns) |
| B: `--proc /proc` | allowed | ~400; allowed | EACCES | refused |
| C: `--unshare-pid --proc /proc` | allowed | 6; ESRCH | ENOENT | refused |
| D: `--bind /proc /proc` | allowed | ~400; allowed | EACCES | refused |

- **B and D are the same mount.** If the pid namespace is shared,
  bwrap 0.13.1's `--proc /proc` does not mount a new procfs. It produces a
  read-write recursive bind of the host's procfs (the same device, `0:24`,
  with the `binfmt_misc` submounts). The mountinfo lines are identical in
  both arms.
- **What read-write host `/proc` opens.** Under the read-only base, nothing
  under a host process's `/proc/<pid>` is writable. Under B or D, a seat
  WROTE five knobs of a throwaway same-uid host process: `oom_score_adj`,
  `coredump_filter`, `clear_refs`, `sched` and `autogroup`. The kernel
  refused the rest: `mem`, `comm`, `loginuid`, `timerslack_ns`,
  `uid_map`/`setgroups`, `timens_offsets`, every `/proc/sys` write tried
  (including `ns_last_pid`, which is mode 0666), `binfmt_misc/register` and
  `sysrq-trigger`. The `/proc/<host pid>/root` guard (ranger-base-rfyth)
  holds in every arm.
- **C costs D17's reasons**, as D17 predicted. A seat would no longer see
  or signal its host pids.

The probe is `docs/adr/0096-proc-spellings.probe.sh`. It runs uncaged, makes
no repo writes, and prints every cell above.

## Decision

**D1. Nesting is not a property of the bwrap seat wall.** A runtime's own
sandbox does not start inside a bwrap seat, and posse does not claim it
does. Once the in-flight Landlock layer lands, it cannot start. B7 stays a
true row about the probe's `$BW` and about bwrap as a mechanism. It is no
longer a property of the tier. Claude's own `/sandbox` on a bwrap seat is
therefore a declared absence. The bwrap wall is the file wall, and posse
renders nothing that relies on the runtime's own sandbox.

**D2. The render does not change.** No `--proc`, no `--bind /proc /proc`,
no `--unshare-pid`. `TestBwrapArgsNeverUnsharesWhatTheSeatNeeds` keeps
`--proc` on its never-list, and its comment gains this ADR as the second
reason.

**D3. A live pin that nests a bwrap abstains loudly, for one named reason,
and never fails for it.** One test helper asks the question once per
process: does `bwrap --ro-bind / / true` start from here? Every live pin
that execs a bwrap calls it. Today that is
`TestLiveBwrapGitRefsCommitsOnTheSessionBranch` and its siblings behind
`bgLiveSkip`, `TestLiveWallClaudeProjectsRefusesPeerPlants` and
`TestLiveWallStrictMCPStartsOnlyTheLineServers` (renamed into
[ADR 0097](0097-the-walls-live-pins-measure-in-a-nesting-venue-after-each-landing.md)
D4's filter under ranger-base-q749c; it was
`TestLiveStrictMCPStartsOnlyTheLineServersUnderTheRenderedWall` when this
record was written). When the
answer is no, the pin skips with
`NOTHING MEASURED: this process is inside a wall that forbids nesting (ADR 0096 D1): <bwrap's stderr>`.
The helper is separate from `liveBwrapApplyRefusal`, which answers "does
this host provide the tier" for `AvailableCages`. The two questions share
a probe today. They must not share a meaning, because a Landlock-only
refusal would answer them differently. `make test` is then green inside a
caged seat on this account.

**D4. The probe stops saying B7 about the wrong wall.**
`0080-bwrap-tier.probe.sh` (whose `BASE` is D17's base) gains a row
`T-nest  bwrap inside the D17 base` with EXPECT `refused` and a detail line
carrying bwrap's stderr. `0080-linux-cage.probe.sh`'s B7 comment says that
its `$BW` is not the shipped wall and points here. The row records what the
shipped wall does, and so the next "nesting works" claim has to change an
EXPECT cell in public.

## Consequences

- **The wall's live pins measure only where nothing walls the process
  running them.** On D1 every seat is caged, and the landed-check judge
  renders through `BwrapArgs` (and does not run the arms anyway, ADR 0088
  D6). So after D3, the three pins measure on an uncaged operator run and
  nowhere else. That is a pin with no scheduled place to measure. It is
  filed, not solved here, as ranger-base-m2ii4: a required, caged venue
  for those pins. Implementation of D2–D4 is ranger-base-pvkhn.
  *Answered by [ADR 0097](0097-the-walls-live-pins-measure-in-a-nesting-venue-after-each-landing.md)
  (2026-11-14). It also corrects the count: 32 pins fail to nest on a
  seat's `/proc`, not three, and D3's helper must reach all of them.
  When ADR 0097 was written, the in-flight layer had been held off `main`.
  D1 still holds on the render alone (arm A).*
- **Same-uid host processes keep their `/proc` knobs closed to seats.**
  That property was accidental before. It is now recorded and pinned by
  D2's never-list.
- **No darwin change.** Nested `sandbox_apply` was already refused there
  (ADR 0080 D21's history).

## Alternatives rejected

- **`--proc /proc` (the bead's shape 1), or the same mount spelled
  `--bind /proc /proc`.** This was the clever one. It restores B7 and all
  three pins and keeps host pids, and the measured widening is small:
  five resource and diagnostic knobs on same-uid processes. Raising
  `oom_score_adj` or resetting `sched` is weaker than `kill(2)`, which the
  seat already has against the same set. It loses for two reasons. First,
  it buys nesting only until the in-flight Landlock layer lands, and then
  it is a widening that buys nothing. Second, `coredump_filter` is not a
  resource knob: it decides what a crash writes out of another process.
  Opening it to every seat is a confidentiality question, and a widening
  granted for a window is the wrong place to answer that. The `--bind` spelling would also have been the one to pick
  over `--proc`: bwrap's fallback to a host bind is a build behaviour, the
  same reason ranger-base-racfc spells `--unshare-user` explicitly.
- **`--unshare-pid --proc /proc`.** It nests, but a seat can no longer
  signal its own host-side pids, and `checkorphans` reads only the seat's
  namespace. D17's rejection stands as measured (arm C).
- **Exempt the runtime's sandbox from the Landlock layer.** There is no
  such spelling. The topology hook reads whether a filesystem domain
  exists, not which rights it handles. A domain over the runtime covers
  everything it forks.
- **Leave the pins as they were: one FAIL, two quiet skips.** A red that is
  not a regression teaches people to rerun until green. A skip that does
  not say why reads as coverage.

## In flight

- ranger-base-6rndn

## Exit hatch

If a later bead needs nesting, it has to make two changes together: drop
the Landlock layer, or find a kernel spelling that does not forbid mounts,
and add D's `--bind /proc /proc` with its five knobs re-priced. D4's
EXPECT cell is what that bead flips. Nothing here holds state.
