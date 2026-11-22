# The abstention reaches the members a census could not see (ranger-base-jqdm2)

2026-11-22, linux 7.2.15 x86_64, go1.27.3, bwrap 0.13.1, 16 cores. The wall
check red at tree f38b3f2688e2, and it is the ranger-base-evx07 class one
route over from where ranger-base-k2quj closed it.

## WHAT RED, AND WHY IT IS NOT THE LANDING IT WAS CHARGED TO

```
FAIL TestLiveWallWatchKillsASeatItsBwrapLeftRunning:
  wallwatchlive_test.go:580: watcher exit = 137, want 7 (the seat's own):
  NOTHING — so no refusal of its own: bwrap ended before it reported a child,
  and without a word
```

137 is 128+9 and `NOTHING` is `wwOneLine`'s answer for an empty pane. That is
ranger-base-evx07's signature exactly — the seat said nothing and a
supervisor's signal ended it — and ADR 0088 D5's ratchet charged it to the
newest landing in the window, 648a32d6 (ranger-base-jhmm7), which added one
QA test under `internal/treepins` and compiles no watcher at all.

**The watcher cannot have been the killer, and that is readable rather than
assumed.** `runWallWatch` kills a seat on four paths and every one of them
writes a line to the pane FIRST (`refuse`/`respond`, ranger-base-r9slg,
ranger-base-pfcci). The pane was empty, and the arm had already read both pid
files, so the seat had been released. What is left is bwrap handing back the
status of a sandbox child something outside ended, or bwrap itself being
signalled — both of which arrive as a silent 137.

## THE ESCAPE: A MEMBER WHOSE SEAT IS RUN BY PRODUCTION CODE

ranger-base-k2quj routed `bgSeatAbstainIfOutside` through all 33 bwrap seat
runs in the member files that have one, read from source as the literal
`exec.Command("bwrap", …)` **in a test**. Seven members do not run bwrap from
a test at all: they call `runWallWatch`, which forks bwrap in production code,
reads the `WaitStatus` itself and hands the test an `int`.

| file | members | how its seat runs |
|---|---|---|
| `wallwatchlive_test.go` | 4 (now 5) | `runWallWatch` in-process |
| `wallwatchtty_linux_test.go` | 2 | the same, through `launchOn` |
| `wallwatchbackstopwiring_linux_test.go` | 1 | the same |

So the census could not see them, there was no `*exec.ExitError` for the
existing door to take, and every one of those members compares the watcher's
code to a number it expected — `want 1`, `want 7`, `want 137`. k2quj's own
closing sentence is this bead: *"only the git-refs fixture is converted… the
same silent kill can still red them."* It took one day.

## THE FIX

One criterion, one NOTHING MEASURED line, two doors — because the two call
shapes cannot share one function and must not have two rules.

- `bgSeatStatusFromOutside(code, signalled, out)` (bwrapgitrefslive_test.go)
  is evx07's conjunction over the two numbers a status reduces to.
  `bgSeatEndedFromOutside(err, out)` is now an adapter over it, so the
  decision has exactly one body.
- `bgSeatAbstainAccount(t, what, account)` is the `t.Skipf` itself, split out
  so the ADR 0097 D3 sentence is spelled once whatever built the account.
- `wwAbstainIfOutside(t, what, code, said)` (wallwatchlive_test.go) is the
  watcher-shaped door, and `wwSeatAccount` its account. An `*exec.ExitError`
  cannot be synthesized to borrow the other door — `os.ProcessState` keeps its
  status unexported — which is why there are two doors and not two criteria.
- Routed at **8 sites** in the three files, each before any arm reads the
  code: `launchOn`'s early-exit fatal, `wwWaitKilled` (which covers its five
  callers), both arms and the launch deadline of
  `TestLiveWallWatchKillsASeatItsBwrapLeftRunning`,
  `TestLiveWallWatchPassesTheSeatsOwnExitCode`,
  `TestLiveWallWatchRefusesToReleaseASeatMissingAWallAtLaunch`, and
  `TestLiveWallWatchKillsASeatWhoseBackstopRecordCannotBeWritten`.

Placement is load-bearing: `t.Skipf` returns from the test, so the routing
being ahead of the arms is what keeps a box kill from being recorded as a
reading about the wall — in the control arm of the 6ec6n pin the next two arms
assert that a finished seat's leftover was NOT killed and that no verdict was
written, and a box that killed the whole seat satisfies neither.

**The silence half is what keeps arm 1 honest.** That arm KILLS its own bwrap
and wants 137 — and it gets 137 with the watcher's `KILLED seat seat-o` line
in the pane, because the leader was alive when its bwrap died. A silent 137 is
the other thing. Nothing else discriminates them.

One polarity difference from evx07's half, worth knowing: these members have
no FALSE PASS direction. `walled`'s refusal arms (`if ok { fail }`) read a
silent kill as the wall refusing; every watcher arm is an equality against an
expected code, so an outside kill red all seven and passed none.

## MEASURED

- **The premise, live and new**:
  `TestLiveWallWatchReadsAnOutsideKillAsNothingMeasured` — the seat's leader
  SIGKILLed from outside a live watcher, through the rendered argv, reads
  `the watcher exited 137, and the pane holds NOTHING …`, and the control (a
  seat that exits 128 on its own, nothing signalled) reads 128 and stays RED.
  That is the one thing a unit test cannot say: that the watcher passes an
  outside kill through without a word. It PASSES rather than abstaining,
  deliberately — a member that always skipped would hold the check at exit 3
  forever (ADR 0097 D3) — and it is a member, so it measures where D1's venue
  measures.
- **The census**: 56 members in 24 files, 33 literal bwrap seat runs, 4
  variable-program member tests, **9 watcher-run status sites**, 1
  variable-argv watcher test. Floors at 6 and 1.
- **The rule bites**: 8 mutants over the REAL tree, one per routing call
  commented out, each planted alone with the tree asserted clean between
  arms — all 8 red, and the two sibling `t.Run` arms of the 6ec6n pin red
  SEPARATELY, which is why the owner of a site is the innermost function and
  not the test (the first draft attributed to the test, and deleting either
  arm's routing left it green).
- **The venue, with the fix and the new member in it**: `venue+check` rc=0 in
  116s at load 4.40, `wall check: green, 50 pins measured` — 49 before.
- **Not reproducible by repetition**, as evx07 found: `venue+check` rc=0 in
  118s at load 9.99, `wall check: green, 49 pins measured`; the one member at
  `-count=1` ×20 uncaged, all green. The red is the box, not the render.
- **The sender is still not named.** evx07 established that from a seat it
  cannot be (Linux records no signal sender; the journal, auditd and an eBPF
  `signal_generate` probe all need privilege a seat does not have), ruled out
  the load guard by predicate, both OOM families by counter and a sibling
  member by namespace inode and by the venue's fresh pid namespace, and left
  the question as an operator-side reading. Nothing here re-opens that:
  ranger-base-p6yrb is the open bead beside it.

## THE RULE, AND THE TWO SPELLINGS IT NEEDED

`TestQAEveryWallMemberSeatRunRoutesTheAbstention` grew a third arm.

A watcher-run "seat run" is not one call. These fixtures run the watcher in a
`go func()` that SENDS its status down a channel and read it somewhere else —
a `t.Run` arm, a helper taking the channel, a non-blocking select in the
launch loop. So the rule reads two things:

1. a `runWallWatch` call whose `Bwrap:` field is a `[]string` literal
   beginning with `"bwrap"` — a LIVE seat — unless its value is handed to a
   channel, in which case it is a producer and the receive is the site;
2. a receive from an identifier this file makes a `chan int`.

Both resolutions are deliberately crude and say so at the function: the
`Bwrap` key is searched for anywhere under the call's arguments rather than at
a fixed path, and a channel's type is resolved by NAME within the file,
because `c := <-code` cannot be typed from an AST alone. The alternative was a
rule that reads no receive at all — which is the hole this bead came through,
since the only site arm 1 could ever have seen is `launchOn`'s, and every
member that red reads its status somewhere else.

**THE ESCAPE IS READ ON THE TEST AND THE ROUTER ON THE FUNCTION**, and that
asymmetry was found by the rule redding this bead's own live pin in
`make tree-check` after everything else was green. A pin whose subject is the
criterion grades it in its ARMS and may carry the status out to them through a
transport closure — and it must not route, because it is itself the outside
killer and would then abstain on every run (the `grokleader` row is the same
family, one table down). So `wsaWatcherCriterion`/`wsaCriterion` are looked
for anywhere in the top-level test, while `wwAbstainIfOutside` has to be in
the innermost function that reads the status, which is what keeps one arm of a
member from losing its routing behind a sibling that keeps it. Both directions
are planted controls, and the 8 mutants were re-run after the change: all 8
still red.

Three `/bin/false` fixtures in the tty file are READ and correctly found to be
nobody's seat, exactly as `exec.Command("git", …)` is in arm 1. One fixture
passes its bwrap argv as a variable and is invisible; `wsaWatcherSiteRegister`
names it EXEMPT with the reason, and the arm reds on an unnamed variable-argv
site and on a stale row — the same shape, and the same reason, as
`wsaVariableCommandRegister` beside it.

## WHAT ELSE LANDED, AND WHAT DID NOT

- **ranger-base-w6kke's fix is in this commit**, not folded in silently: its
  deliverable was "pass the `wwSaid` to `wwWaitKilled` and append
  `wwOneLine(said.String())` to that `Errorf` and to its 10s timeout
  `Fatalf`; five call sites; `launchOn` would return it", and the routing
  needed the same words one line earlier. `launch`/`launchOn` return the pane
  now and both `wwWaitKilled` messages carry it. Said on that bead; its owner
  closes it.
- Not closed here: ranger-base-p6yrb.
