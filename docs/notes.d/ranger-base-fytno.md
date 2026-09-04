## The reap sweep on a worktree fleet (ranger-base-fytno)

**MEASURED 2026-09-04 12:45Z** (monica, after "i see a lot of seats not being
filled"): `posse list` showed 13 sessions and 3 working. Eight were
`done`/`idle` over CLOSED beads — four held since 23:38–23:50Z the night
before (9h), four since 07:22–08:10Z that morning — every worktree 0-dirty
and 0 ahead of main. The watch running at the time (pid 60921) reaped six
OTHER sessions on cadence in the same passes (`reaped
dinesh-posse-ranger-base-8nsc6 (bead ranger-base-8nsc6 closed)`, pju9t,
han3i, v4zlr, 5im1q, d91mf). Monica killed the eight by hand; nothing to
land; the seats freed.

### What the bead assumed, and why that is not it

The bead read the seat-release line —

```
↺ seat holden-posse released: no session (held ranger-base-v4zlr) — reaped or gone since this Run fired it
```

— as the reaper's own vocabulary, and asked for the candidate set to be
widened from "sessions this Run fired" to "every live session over a closed
bead". **That line is `reconcileSeats`' (`dispatch.go`), not the sweep's**,
and it is about the SEAT map, which is per-Run by design (ranger-base-6swlr's
abstention). The sweep's candidate set was already store-wide: `autoReapPass`
walks `d.HB.Sessions()` — every meta in `state/herdr/` plus every workspace
herdr holds — and its one Run-scoped guard was re-keyed off passes and onto
the persisted run record by ADR 0028 §3. `TestAutoReapSkipsASessionJustPrompted`
has pinned the cross-Run reap since then: a fresh `Dispatcher`, holding none
of the first one's memory, reaps a session the first one fired.
`TestAutoReapTakesASessionAnotherRunFiredAndBounced` now pins it in the
bead's own words — fire under Run A, close, bounce, assert the next pass
takes it — so no future narrowing can land green.

### The two silences that DO produce exactly this symptom

Both were in `autoreap.go`, both fail-closed and mute, and a mute
forever-skip is indistinguishable from a reaper that does not run — which is
what the shop read while the passes came round on time.

**1. The name checks were rendered from the worktree, not the checkout.**
Dispatch names a session `SessionForBead(persona, is.Dir, is.ID)` where
`is.Dir` is the REPO the bead lives in; since rangerhq-09o2 the session's own
`dir:` is its per-bead worktree, whose basename is that name again. So
`SessionFor(agent, s.Dir)` rendered `gwart-gwart-posse-ranger-base-hlv21` and
matched nothing, which had two consequences on a fleet where every dispatched
session has a worktree — that is, all of them:

- the **crew arm** (ranger-base-f6lk) was unreachable. A per-bead session the
  operator stepped into — cockpit `p`, `posse prompt` — was gated on
  `s.Name == SessionForBead(agent, s.Dir, s.Bead)`, which no worktree session
  can satisfy, so it was skipped by every sweep forever. Not reaped late:
  never. That is f6lk's own population, back in the shape that fleet has.
- the **slot guard** was nominal. `<persona>-<repobase>` with a worktree fell
  through the guard meant to protect it and was taken as ordinary residue.

Both now render from `s.Checkout()` — the repo the name was built out of.

**2. An unreadable store looked like an open bead.** `reapWhy` folded
`d.Bd.Show` failing into `is.Status != "closed"`: one `return "", false`, no
line. A session whose bead could not be READ — a stale or missing `.beads` in
its worktree, a bd refusal, a timeout — was therefore skipped on every pass
and named nowhere, which is why 465 `reaped` lines in `dispatch-watch.log`
say not one word about the eight. Fail-closed is unchanged (an unanswerable
store never licenses a kill); the refusal is now SAID, on every pass it is
true, the way `residueHolds` already says its own.

### What is not claimed

Which of these took the eight is not determined: monica's hand-kills removed
the metas and worktrees the question would be asked of, and the log records
only what the sweep DID. What is determined is that both defects are real in
the code at HEAD, both produce precisely "finished, seat held, nothing said",
and after this bead the next occurrence names itself in the log the operator
already reads.

`UnpointedBeadSession` (`herdrback.go`) still renders from `s.Dir` for the
same slot exclusion. Left alone deliberately: a slot session never carries a
worktree, so the two renders agree on every shape that exists, and the
predicate is doc'd as one definition shared with the `🏷️no-bead` tag.

Pins: `internal/posse/reapworktree_qa_test.go` (six, each half a pair —
what the sweep must take and the nearest shape it must refuse). All are
mutation-checked: reverting either fix, or narrowing the candidate set to
`d.lastPrompt`, turns them red.
