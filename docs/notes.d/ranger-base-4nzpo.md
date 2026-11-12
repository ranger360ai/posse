# The landed-check judge gets a seat's host masks, no network, and a build cache of its own run (ranger-base-4nzpo)

2026-11-11. The wall half of the design on ranger-base-hy1aj (D1, D2, D4).
Builds on ranger-base-6b46n's read masks.

## What was open, and is closed here

`boxLandedCage` (internal/posse/landedcheck.go) renders the judge's cage
through `BwrapArgs` and `SeatbeltProfile`, not through `RenderWall`, because
the judge has no persona. That kept it out of the fake-seat problem, and it
also kept it out of every host-scoped mask a seat gets in `bwrapRender`. The
judge is landed code, run on every landing, and it had:

- **the user runtime dir in view** (ranger-base-ludpp's mask): the session
  bus, the user service manager, and the agent sockets. A same-uid client of
  the service manager can start a unit, and that unit runs uncaged.
- **the host's `/tmp`** (ranger-base-7wdb7's mask): X11 and ICE sockets, and
  every uncaged process's scratch.
- **the host's network namespace**: egress, loopback services, abstract
  unix sockets.
- **a write grant on the box's shared Go build cache**, which ADR 0088 D3
  granted on purpose to save a cold build. ranger-base-olr0g measured that an
  archive planted there runs inside the next uncaged build that reads it,
  posse's own included. So landed code could reach the launcher's next build.

The rule that was missing: **the judge is never stronger than a seat**, and
since no persona owns the judge, it keeps none of a seat's egress either.

## The fix

- bwrap: `carve.Hide = liveBwrapRuntimeDirs()`, and `carve.Tmp` is `tmp` in
  the run dir (`landedCheckTmp`, the same dir `judge()` hands the check as
  `TMPDIR`). `--unshare-net` is appended at the judge's call site only, so
  `BwrapArgs` keeps its seat semantics. bwrap brings the new namespace up
  with loopback, so a check that listens on 127.0.0.1 still runs.
- `landedCheckTmp` stands down (host `/tmp`, as before) when the home, the
  run dir or the tree is under `/tmp`, for `bwrapSeatTmp`'s reason: a private
  `/tmp` over them leaves nothing to chdir into. No production run is that
  shape. The run dir is under the state dir.
- seatbelt: `landedCheckSeatbeltNet(run)` is appended after the whole
  profile, so it is the last match: `(deny network*)`, then loopback IP and
  unix sockets under the run dir allowed back. The unix-socket half matters
  because a unix-socket connect is a network operation in SBPL and not a
  file write. Seatbelt has no spelling for Hide or Tmp, and a `/tmp` write is
  already outside the grant.
- `landedCheckWritable` no longer grants `goBuildCache()`. `judge()` sets
  `GOCACHE=<run>/gocache`, made fresh per run and removed with the run. Fresh
  and not a persistent judge-only cache, because a persistent one lets
  landing N plant objects that decide landing N+1's verdict.

This reverses ADR 0088 D3's "and so is the shared build cache". The public
restatement of D3 is ranger-base-ze6ie.

## The pin

`TestQALandedCheckJudgeIsNoStrongerThanASeat`
(internal/posse/landedjudgecage_qa_test.go, arm 1). It drives the real call
site, `landedChecker.judge`, over a real repo with the box's own renderer,
and refuses after the render so nothing runs. On bwrap it asserts
`--unshare-net`, a `--tmpfs` over each runtime dir (the fixture sets its own
`XDG_RUNTIME_DIR` so the row is never vacuous), `/tmp` bound to `<run>/tmp`,
`GOCACHE=<run>/gocache` in the env and in the argv, and no write grant
covering `goBuildCache()`. On seatbelt it asserts the profile ends with the
network rule. `TestQALandedCheckSeatbeltNetRule` reads the darwin rule on any
host, and `TestQALandedCheckTmp` pins the stand-down.

## Verified

MEASURED 2026-11-11, this Linux box, bwrap, go1.27.3:

- Mutation-red, one at a time, each red on its own assertion and not on a
  compile error: drop `--unshare-net`; drop `carve.Hide`; set `carve.Tmp` to
  ""; put `goBuildCache()` back in the write set; change `GOCACHE`.
- `scripts/posse-landed-check` judged posse 5816bb8c behind the new cage,
  with the run dir and HOME outside `/tmp` so the private `/tmp` was on:
  **green in 76s**, at a one-minute load of 2.2. The design measured 70-78s by
  hand-built bwrap. The rendered argv had `--tmpfs /run/user/1001`,
  `--bind <run>/tmp /tmp`, `--unshare-net`, and `GOCACHE=<run>/gocache`.
- `TestLiveLandedCheckCageReadsNoSecret` (6b46n's) stays green with the
  private `/tmp` on, because its fixture is under `/var/tmp`.

ASSUMED, not measured: the darwin rule. There is no darwin host here, and
the render is all that is pinned. The probe is on ranger-base-bfla6.

## Not closed here

- ranger-base-5m5yq
- ranger-base-bfla6
