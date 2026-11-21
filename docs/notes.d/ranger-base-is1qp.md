# The dispatch loops on this binary (ranger-base-is1qp)

The decision is ADR 0092 D8; the mechanism is `internal/posse/boxloops.go`
with `boxloops_linux.go` / `boxloops_other.go` and the pins in
`boxloops_qa_test.go`. This page holds the measurement it rests on and the
alternatives that were weighed and dropped, which do not belong in either.

## The question, as it arrived

`posse upgrade` renames one file, `~/.local/bin/posse`, and rename(2) leaves
every running process on its old inode (ADR 0092 D3.5). That file is box-wide:
every `RHQ_HOME`'s `dispatch --watch` loop execs it, and the loop is what
RENDERS a launch. Every reader of that fact was per-instance —
`ReadWatchImage(a *App)` asks ONE instance's watch lock and reads ONE
instance's `dispatch-watch.pid`, ShopCheck's G18 arm runs per instance, and
the verb runs in the delegate's instance only. So after an install, nothing on
this box said a SIBLING instance's loop was on the old inode.

ranger-base-nqaa1 fixed the READING OF THE REMEDY — `watchScopeSentence`, on
every branch of both renderings — and deliberately did not widen the reading,
because widening meant coining a notion of "the instances on this box" that no
shipped code in this tree had.

## What was measured

2026-11-21, this box, linux 7.2.15-1-omarchy, from a gilfoyle seat at cage
shims, uid 1001. `~/.local/bin/posse` at dev:ino 54:14541392. Walking `/proc`
for every pid whose `exe` link resolves to that path found three:

```
PID       PPID     DEV:INO        DELETED  RHQ_HOME        RECORD   VERDICT
1185112   1140     54:14541392    no       ~/homes/weller  1185112  LOOP
1185185   1140     54:14541392    no       ~/homes/ranger  1185185  LOOP
3285561   3285542  54:14541392    no       ~/homes/ranger  1185185  not a loop
```

argv, for the record and never as a key: the first two are `posse dispatch
--watch 3m -n 2 --resume` and `posse dispatch --watch 5m -n 6 --resume`; the
third is `posse pulse wait --after 494`.

Sibling `exe` readlinks, their stats, their `environ` and the sibling
instance's `state/dispatch-watch.pid` were ALL readable from that seat.

**What moved since the bead was filed**, and it is worth saying because the
bead's own description carries the older numbers: the install inode and both
pids (54:12591441, pids 2066610 and 1991667). An install landed and both loops
restarted at 12:11:54Z onto the new inode. So neither loop was stale when the
box was read, and what this measurement establishes is READABILITY — which is
what the mechanism needs. **A live stale sibling is still unmeasured.** The
fixture pin (`theMeasuredBox`) holds this exact arrangement with the weller
loop left on the pre-install inode, which is the state the census exists to
report; the live arm (`TestLiveBoxCensusReadsThisBox`) holds that the
arrangement is readable, and logs the table so a suite run carries the
measurement rather than this page's word for it.

Reproduce it: `scripts/` holds nothing for this, on purpose — the live arm is
the reproduction, and it runs from `make test`. For a reading outside a suite,
`go test -run TestLiveBoxCensusReadsThisBox ./internal/posse/ -v`.

The class the third row belongs to is recurrent, not a one-off: two separate
runs of the live arm, minutes apart, each found a different transient `posse`
child of a seat under the ranger instance (pids 3285561 and 594507). So the
filter is exercised on this box on essentially every reading, which is the
best thing that can be said about a filter.

## The measurement that was MISSING, and what it cost

The reading above was taken from a `cage shims` seat. A shims seat is not in a
child user namespace, and that turned out to be the whole of what the
measurement did not cover.

MEASURED 2026-11-21 by hoover, from a live seat on this box whose Bash runs in
a child user namespace (`/proc/self/uid_map` = `1001 1001 1`), uid 1001,
against the ranger dispatch loop (pid 1185185, uid 1001, recorded in its own
home's `dispatch-watch.pid`):

```
ls /proc                      -> 460 pids        (so boxPids succeeds)
readlink /proc/1185185/exe    -> EACCES
stat -L  /proc/1185185/exe    -> EACCES
cat      /proc/1185185/environ-> EACCES
a child the seat started itself -> readable
```

Same-uid but cross-userns is refused by `cap_ptrace_access_check`, which
compares user namespaces and not just uids (the ranger-base-rfyth mechanism).

**So two claims in the first version of this work were wrong**, and both in
the direction that costs a reader:

1. *"The census survives a cage by ruling rather than by luck"* — citing
   bwrap's deliberately-absent `--unshare-pid` and the host procfs under
   `--ro-bind / /`. True, and not enough: that opens the pid **list**, not the
   **links**.
2. *"A process of another uid is invisible here … ASSUMED harmless on this
   box, where every instance's loop runs as the one account."* The uid was the
   wrong fence to be reassured by. On a caged reader the "harmless" case was
   every sibling loop on the box.

And the consequence was the defect this census exists to prevent, one level
in: `procExe` answers the same `ok=false` for a pid that exited and for a link
the kernel refused, so the first version dropped both silently, found no
candidates, and rendered **"no dispatch loop on this box runs
~/.local/bin/posse — nothing a rename of it left behind"**. A false all-clear
about loops that may still be rendering pre-install walls — the one sentence
the file says must never be printable. It is filed as ranger-base-rph5z and
fixed before any of this landed on main.

I could not reproduce it myself: `unshare -U -r` to read a sibling's `/proc`
from a child userns was refused by this seat's permission coordinator as
unauthorized access, and that is not worth working around. So the EACCES
reading stays hoover's, attributed, and what is verified here is the consumer
behaviour through the `ExeOf` seam — which is what the seam is for — plus
three mutants, below.

## The one finding that shaped the mechanism

**The third row is a live false positive, not a hypothetical one.** A seat's
own `posse pulse wait --after 494` child runs the same image, from the same
path, under the same `RHQ_HOME` as the ranger loop. Keyed on the exe link
alone — which is the only key the census has for finding candidates at all —
it is indistinguishable from a dispatch loop.

What separates them is the per-instance record, read as a FILTER rather than
as an enumerator: the process names its own instance from its `environ`, and
that instance's `dispatch-watch.pid` either names the process back or does
not. Enumerating records is what needs a homes root; filtering pids `/proc`
already produced does not. That inversion is the whole of why this needed no
config key.

## Why "the instances on this box" was refused

The bead asked for that notion and the answer is that it is the wrong
derivation, not merely an absent one.

- `RHQ_HOME` is an environment variable. Nothing constrains it to live under
  one root, so a declared-instance enumeration UNDER-reports a loop whose home
  is elsewhere — which is exactly the hole.
- It OVER-reports: an instance running no loop has nothing a rename could have
  left behind. `~/homes` holds `ranger` and `weller` today, and only one of
  them has pulse state at all.
- It needs a config key the operator would have to maintain for a reading to
  stay correct, which is a worse trade than deriving the set from the file the
  install actually renames.

## Alternatives weighed and dropped

- **Reuse `sysProcTable`** (loadguard.go), which already walks the box. It is
  two bounded `ps` reads giving pid, ppid, %CPU, etime, comm and argv. `ps`
  cannot report the (dev, inode) of a process's image at all, and argv is the
  one key AGENTS.md ("Ending anything") forbids as a target key because every
  seat's is byte-identical. So the census reads `/proc` directly and forks
  nothing — which is also what lets it run from a caged seat.
- **Make it a G-row (box-wide G18, or a G19).** Dropped on coherence and not
  on cost. Every G-row is per-instance, and a ShopCheck tick runs per
  instance, so a box-wide row would be raised once per instance that happens
  to have pulse state and silent in every instance that has none — one
  condition, both duplicated and missed, depending on which instances tick.
  `posse upgrade` is the one command whose subject is box-wide because it is
  the one that does the rename.
- **Signal 0 for liveness** instead of "`/proc/<pid>` is there". Under
  `LANDLOCK_SCOPE_SIGNAL` a caged seat's signal 0 to a process it did not fork
  is EPERM whether that process is alive or dead (AGENTS.md,
  ranger-base-e5un1), so it would read wrong from exactly the seats that most
  need the census.
- **A darwin route.** The process table `cmd/checkorphans` reads through
  `sysctl kern.proc.all` carries comm and argv and neither an image inode nor
  an environment. A real route needs `proc_pidinfo`'s PROC_PIDVNODEPATHINFO
  and a `KERN_PROCARGS2` parse, neither measured from inside a seatbelt
  sandbox. `boxloops_other.go` refuses and says why, on
  `procstart_other.go`'s rule — an abstention costs a line in a report, while
  a census compared against an unmeasured spelling passes for the wrong
  reason.
- **Replace the per-instance line with this one.** Dropped: the per-instance
  reading is STRONGER for the one loop it reads — a lock the kernel holds,
  against a pid record a process names itself by — and it is the loop the
  delegate's next seat comes through. Both print, each stating its own reach.

## The second round: the fix that made the caveat useless

The first fix for ranger-base-rph5z counted every same-uid process whose exe
was refused. Run against this box, the live arm then read **5 unreadable
(partial=true)** from an ordinary `cage shims` seat — so the caveat was on
permanently, which is the failure the code comment next to it warns about in so
many words: a caveat nobody can act on is a caveat nobody reads, and an
always-on caveat would have hidden the false all-clear it exists to prevent.

MEASURED 2026-11-21, this box, from that seat (`uid_map` = `0 0 4294967295`,
userns `user:[4026531837]`), what those refusals actually were:

```
pid 1140     dir_uid=1001  comm=systemd        exe=EACCES (errno 13)
pid 1142     dir_uid=1001  comm=(sd-pam)       exe=EACCES (errno 13)
pid 2682725  dir_uid=1001  comm=sshd-session   exe=EACCES (errno 13)
pid 2931892  dir_uid=1001  comm=gpg-agent      exe=EACCES (errno 13)
pid 1185185  dir_uid=1001  comm=posse          exe=/home/posse/.local/bin/posse
```

Four non-dumpable system processes of the same account, none of which could
ever be a dispatch loop — and note `dir_uid=1001`, so `/proc/<pid>` ownership
does NOT separate them, which is what the first fix keyed on.

`/proc/<pid>/comm` does, at no cost: world-readable, no ptrace check, so it
answers where `exe` does not. A refusal counts when comm names this binary, or
when comm could not be read either; it is excluded when comm names another
program. With that in, the same live arm reads **0 unreadable (partial=false)**
and the all-clear on this box is honest again.

**Why this is not the argv matching AGENTS.md forbids.** That rule is about
choosing a TARGET by a pattern every seat's process shares, and the cost is
killing someone else's work. comm here decides one thing — whether a process
we were REFUSED is worth saying we could not read — and a loop is still
confirmed by nothing but its own instance's record. A hostile
`prctl(PR_SET_NAME)` can inflate the count and make a complete reading claim it
is partial; that is a denial of clarity, not a false all-clear, which is the
direction everything here is built to fall in.

## The five mutants

Each new fence was removed and its own pin watched go red, because a fence
whose pin has never failed is a fence nobody has tested.

| mutant | what was removed | result |
|---|---|---|
| 1 | the `Lstat`/`IsRegular` check on another instance's record | `TestQABoxRecordReadRefusesANonRegularFileAndDoesNotHang` blocked on the FIFO and failed on its 10s deadline |
| 2 | the unreadable-candidate count | `…NeverRendersAnAllClearWhenItWasRefusedAReadOfOurOwnProcesses` red: "census counted 0" |
| 3 | the home validation at the portable chokepoint | `…RendersNoTextFromAMalformedHome` red, and it printed the injection verbatim: the peer's text on its own line in the middle of the sentence the install delegate pastes onto a bead |
| 4 | the refusal predicate, forced to false | three pins red, including "census counted 0" over two refused loops |
| 5 | the comm narrowing removed | `…DoesNotCountRefusalsThatCouldNotBeALoopOnThisBinary` red both ways: "counted 4" where 0 is right, and "counted 2" where 1 is |

Mutant 3 is the one worth keeping in mind. The rendered line was:

```
dispatch (box): 1 dispatch loop on this box, 1 still on the image the rename replaced — /homes/ranger
PLEASE RUN THIS INSTEAD pid 4242 STALE, runs inode 54:12591441; launches those render …
```

The environ-fence census (`internal/treepins/environread_qa_test.go`) was
mutated three ways too — a second reader, a widened return, a second lifted
key. The second reader and the second key each red their arm against the real
tree; the widened return does not **compile** against the real callers, so
that arm is proved hermetically instead, by a self-test that runs the
predicates over source it writes itself. A census only ever fed a green tree
is a census nobody has seen say no.

## What this reader touches that a security reader should know about

`boxHomeOf` reads `/proc/<pid>/environ` — another process's whole
environment, which on this box carries credentials. One variable leaves that
function and it is a path (`RHQ_HOME`); the return type is one string rather
than a map on purpose, the raw bytes are never logged, rendered or attached to
a `BoxProc`, and the census line an operator pastes on an install bead cannot
carry them. The alternative keys were worse on this axis, not better: argv is
forbidden for a different reason and is equally sensitive, and there is no
other way for a process to tell a reader which instance it belongs to. Filed
for hoover's read on its own bead rather than asserted here — and that read
came back with two P2 defects (ranger-base-rph5z above, and ranger-base-cwboz,
which is why the value is now validated before it is rendered or opened and
why the fence is a census rather than a comment). What survives as a residual:
a same-uid process can still plant a well-formed home holding a regular record
that names its own pid, and so present itself as a loop. Read-only effects,
and it is the uid boundary doing the work rather than this reading.

## Five things that bit during the build, all worth keeping

**The reader is a subject of its own census.** `posse upgrade` runs FROM the
path it replaces, so the verb's own process is a candidate by the census's own
key, and after its swap its own image is the old inode. Unskipped, every
single run would report the reader as a stale process on this binary — true,
and useless, because it is about to exit and renders no launch. `boxProbe.Self`
is the skip, and it is deliberately narrow: a transient CHILD of the verb is
still reported, among the unattributed, because a reader that silently drops
processes it thinks it recognises is the shape this file is a correction of.

**A path that holds nothing had to be called stale.** The first predicate
here read `Image != "" && onDisk != "" && Image != onDisk` — which looks
careful and is the wrong answer, because it makes an absent file "nothing to
compare against" rather than "not the image this loop runs". `WatchImage.Stale`
had it right all along (`Image != "" && Image != OnDisk`), and the divergence
would have surfaced as the one sentence this reading must never print: "all
already on the installed image", over a path with no installed image on it.
Mid-swap and the `install(1)` unlink/write window (ADR 0092 Context) are the
states that reach it. Both predicates are now the same shape, a pin asserts
the census's answer and `WatchImage.Stale`'s answer over the same facts, and
the rendering says the path is empty.

**A fixture that reads `/proc` right after `exec.Cmd.Start` reads the wrong
process.** This one turned `make test` red, and the failing test was
`watchimage_qa_test.go`'s two-instance fixture — written under
ranger-base-nqaa1, this bead's parent, so a defect of the same lineage rather
than of this change.

`Start` returns once the child's exec has been reported, but `/proc/<pid>/exe`
is not reliably the new image by the time the parent is next scheduled.
MEASURED 2026-11-21: 2 failures in 20 runs of these fixtures, each reading the
forked-but-not-yet-settled child as the TEST BINARY
(`/tmp/go-build…/posse.test`) a fifth of a second into the test — and then
failing an assertion about a rename that had worked perfectly, which is the
worst shape a flake can take. It is the same window `cmd/checkorphans` refuses
to judge a process inside of (AGENTS.md, "old enough not to be a fork/exec
teardown window"); this package had simply never waited for it.

Underneath it, a second unsound assumption in the same fixtures: the stand-in
loop slept 60 seconds inside a package that ran 262.7s on the failing pass, so
on a loaded box it could exit mid-test and leave the reading pointing at
whatever took its pid. `watchpid.go`'s own `pidAlive` comment says why that is
never safe to build on — "a recycled pid is alive and is a stranger" — and the
fixtures were keying identity on liveness.

The remedy is one `rrgncStartLoop` that does not return until `/proc` says the
child really runs the fixture's binary, a stand-in that outlives the package
timeout rather than a minute of it, and an identity check before every
reading, so a child that dies mid-test arrives as "your fixture died" and
never as a verdict about staleness. 40 consecutive runs clean, against 2 in 20
before. The product code under test is unchanged: a fixture may re-establish
its own precondition, and that is the whole of what this does.

A smaller note on my own diagnosis, kept because the wrong guess is instructive:
the first guard I wrote blamed pid REUSE and said so in its failure message.
The reproduction said otherwise — the child was 0.19s old, far too young for
that — and the message was corrected rather than left to mislead the next
reader with a plausible-sounding cause.

**Naming a local `home` in internal/posse reddened a file this work never
touched — twice.** `internal/treepins`' temp-root census resolves an
identifier across the WHOLE package, so every literal assigned to `home`
anywhere in internal/posse joins the set of parent directories that
`os.MkdirTemp(home, "abbrev-probe-")` in `app_test.go` could be writing under.
Two of my own lines widened that set: a `home := t.TempDir()` in the FIFO pin,
and an `ok, home = false, ""` in the census loop — the second put `""` in
there and the error read "home, which this package binds to 7 different
literals — "" among them", pointing at `app_test.go:209`.

It took a bisect to believe it, because the failing file is one this bead
never opened: green with round 1 of this work in the tree, red with round 2,
and the round-2 diff touches neither `app_test.go` nor the census. The census
is right to work that way — a prefix nobody can resolve is a prefix nobody can
check, and that is the whole point of a temp-root census on a box whose /tmp
hit 100% of its inodes. The cheap half of keeping it resolvable is not reusing
its names for a local: `stateHome` and `declaredHome` here, with a comment at
each saying why, because "rename this variable" looks like style and is not.

**`os.UserHomeDir()` in a live arm of internal/posse skips silently.** This
package's `TestMain` replaces `HOME` with a per-tree temp home
(herdr_test.go), so the obvious spelling found no installed posse and the live
arm skipped with a message that read exactly like a CI runner's — on the one
box the arm exists for, on its first run. `operatorHome` is the home this
binary started with and is what `ciwatch_live_test.go` reaches for, for the
same reason. A live arm in this package that looks for anything in the real
home has to use it.
