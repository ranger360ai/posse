## Relaunch re-proves the name under the lock (ranger-base-rrg2)

`ranger-base-w4h5` put relaunch's destructive tail — `closeRecorded` →
`recreateSession` → `keepRecipe` — under one launcher lock. The preflight
stayed outside it on purpose: `landThePlane` waits up to `DefaultLandTimeout`
(10m), and a launcher lock held that long queues every dispatch pass, every
`posse new` and every listing's prune behind one operator's refresh.

That left one window open. `nameWornElsewhere` proves no *other* workspace
wears this session's name, and it proved it against a `workspace list` read
**before** the landing turn. Ten minutes later the tail takes the lock and
kills the session. Anything that took the name in between — a `posse new`, a
second launcher, a hand-run `herdr workspace create` — is invisible to that
proof and fatal to the one `recreateSession` actually enforces:

```
checked s1: plain shell, dir …
killed s1
s1 was closed but could not be recreated: session 's1' already exists
  its recipe was kept in …/state/herdr/s1.yaml
```

That transcript is the measured wrong arm (delete the guard below and
`TestRelaunchRefusesANameTakenAfterThePreflight` prints exactly it). A
session destroyed for a reason that **was** knowable before anything was
touched — rangerhq-v52t's loss, reached from the other side.

**The fix.** `provenNameTakeable` is the refusal in the words both callers
print, and it is asked twice: once in `RelaunchSession` as the fail-fast, so
a doomed relaunch is refused without first queueing behind a firing pass, and
once at the top of `replace()` — inside the lock, one line **before**
`closeRecorded`. That is `reclaim`'s and `clearDeadMeta`'s pattern (ADR 0011
§1, rangerhq-3a5t) applied to the one obstacle that lives in herdr rather
than in the plan. Refusing there costs nothing: nothing has been destroyed,
so the session is still running and its record is still intact.

**One listing answers the create's whole name question.** `nameFree` has
three arms, and on the far side of the kill only one can fire: the syntax was
proved when the name was first created, and `mustNotOrphan` reads the meta
`closeRecorded` has just removed. What is left is `HasSession`, and a session
row is either this meta's own (excluded by workspace id) or a workspace
wearing the label — which is exactly what `nameWornElsewhere` reads. Under
the lock that reading is decisive in the direction that matters: a create for
this name is either finished, and its workspace is in the listing, or has not
begun.

**The plan is deliberately not re-resolved.** `planLaunch` is a *value*
carried into `recreateSession`, so preflight and create cannot disagree about
it by construction — that is rangerhq-v52t's own rule. A stale plan builds
the session the operator was shown; a stale name proof destroys one.

**Pinning a window, not a call count.** The fixture is a listing lever in the
fake herdr, `unhide-when-locked`: a workspace hidden from `workspace list`
becomes visible the moment the launcher lock is **held**. It is keyed to the
lock and not to "the Nth listing" because a count is a fixture about how many
listings each phase happens to take today, and would go red on any change to
either that has nothing to do with the race. Held is measured the way
`fakeProbeLaunchLock` measures it — from the fake's own process, since flock
is per open file description and the pass under test would always find its
own lock free.

Pins: `TestRelaunchRefusesANameTakenAfterThePreflight` (the session must
still be running and its meta intact when the relaunch refuses; no
`workspace close` in the call log) and
`TestRelaunchStillReplacesTheSessionWhenTheNameIsFreeUnderTheLock`, the
control — the same lever revealing a workspace that does *not* wear the name,
which must still be replaced. Both die when the lever is removed from the
`workspace list` handler, so the rig cannot measure nothing quietly.
