# A non-blocking fetch claim coalesces the plan-usage endpoint's same-second duplicates (ranger-base-zs686)

2026-11-11. Split from ranger-base-888pk: that bead fixed the ledger's
miscount; this one is the cadence/policy half, over the same read log —
**MEASURED 2026-11-08T05:07Z–2026-11-11T18:24Z, 742 reads, 13 of them 429
(1.8%)** — dispatch 494, cockpit 214, pulse 29, cost 4, status 1 — and
**48 same-second duplicate reads**: two callers whose shared TTL (plancache.go,
rangerhq-tdy8) expired in the same instant, each deciding independently to
ask the endpoint. Reproduced on a second box's own log the same way: two
`cockpit ok` lines at identical second-resolution timestamps, 110 duplicate
seconds out of ~1039 lines — one process polling twice in step with another,
not a storm.

## (1) the shared floor: fixed

plancache.go's snapshot write was deliberately lock-free (last-writer-wins —
the file's own header explains why a *blocking* flock across the HTTP call
would trade the rate problem for a liveness one: every other reader on the
box queued behind one slow request). That reasoning is sound for the WRITE
and says nothing about the decision to fetch at all, which is where the
waste actually is.

Added a second, narrower, non-blocking claim around the fetch only
(`PlanCache.claimFetch`/`releaseFetch`/`awaitFetch`, plancache.go): a
`LOCK_EX|LOCK_NB` flock on `<snapshot>.lock`, held only for the duration of
`r.Read()`. A caller that loses the race never blocks on the lock — it polls
the snapshot for up to `planFetchWaitAttempts*planFetchWaitStep` (~1s) for
the winner's answer, then falls through and asks for itself exactly as
before. So the liveness floor is unchanged (worst case = today's behaviour,
a duplicate request) and the common case — two pollers stale at the same
instant — now costs one request instead of two. No lock is ever held across
the write; `store`'s last-writer-wins is untouched.

Pinned by `TestPlanCacheCoalescesConcurrentFetches` (plancache_test.go): a
gated fake reader holds one caller inside its "network call" long enough for
a second caller to arrive at the same stale instant, reproducing the shape by
construction rather than by timing luck. `make fmt-check`, `go vet
./internal/posse/...`, `make crew-check`, `make tree-check`, and
`go test ./internal/posse/ -run 'TestPlanCache' -race -count=5` all green.
`cmd/testparallel`'s `parallelOK` register got the new test's clearance
(reads `blindT`, same as its siblings).

## (2) the backoff: not touched, and the data says it should not be

The only spell past a single retry (2026-11-11 16:27–17:11Z, backoff
2m→5m→10m→20m, streak 3) recovered unattended inside 44 minutes. That is the
escalation (plancache.go's `planCooldown`) doing exactly what rangerhq-rwwp6
built it to do — the guard goes blind for longer, not forever, and the
ledger brake (ranger-base-888pk, now armed) is what degrades under that
blindness per ADR 0018 §1. Tightening the backoff would shorten a blind
spell that already self-healed; loosening it reopens rangerhq-tdy8's storm.
Leaving it alone is the decision, not a deferral — ASSUMED: the existing
escalation is correct policy and this bead's scope is the duplicate reads,
not the backoff curve.

## (3) the 1.8% rate: not itself a problem

13 of 742 reads over four days, median gaps of 300s+ per caller, one spell
that compounded and recovered on its own. Nothing here suggests the rate is
the hazard; the compounding risk named in the bead — a plan-usage storm
landing on top of everything ELSE that polls the account — is real but is a
property of the account's overall request budget, not of this endpoint's
cadence in isolation, and fixing (1) is what this bead can do about it:
removing 48 wasted requests is pure headroom against exactly that
compounding. ASSUMED: no further cadence change is warranted; reopen if a
future read log shows the rate climbing, not on this one.
