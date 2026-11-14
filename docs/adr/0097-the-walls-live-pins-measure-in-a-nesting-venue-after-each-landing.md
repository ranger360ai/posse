# ADR 0097 — The wall's live pins measure in a nesting venue after each landing

*Status: accepted 2026-11-14 (ranger-base-m2ii4) · owner: architect · answers
[ADR 0096](0096-nesting-is-not-a-property-of-the-bwrap-seat-wall.md)'s first
Consequence · adds a second check beside
[ADR 0088](0088-the-landed-tree-is-read-after-every-landing-outside-the-lock-and-a-red-is-filed-not-blocked.md)'s
judge and reuses its cage, ratchet and watchdog*

## Context

ADR 0096 D3 makes every live pin that nests a bwrap abstain inside a wall
that forbids nesting. Every seat is such a wall. The landed-check judge is
one too, and it does not run the suite arms anyway (ADR 0088 D6). So these
pins measure only when an operator runs `make test` uncaged, and nothing
schedules that. A pin that runs nowhere protects nothing. The question is
where they must measure, and who reds when they did not.

ADR 0096 named three such pins. **There are 32.** Run on a seat's `/proc`,
32 of the 34 tests matching `^Test(QA)?Live(Bwrap|Wall|StrictMCP|GrokLeader|GrokLine)`
fail to nest: 15 FAIL and 17 skip `NOTHING MEASURED`. One of the two that
pass is `TestLiveBwrapGitRefsRefusesTheSharedGitState`, and it passes only
because its bwrap never started, so the refusal it expects reads true. That
is ADR 0086 D5's false pass: a probe that passes when its command fails.

**Measured.** 2026-11-14, D1 host, Linux 7.2.15, bubblewrap 0.13.1, go1.27.3,
16 cores, run uncaged by the architect. Each caged arm gets a fresh run dir
and a fresh `GOCACHE` (so every reading is a cold build, as the judge's are),
`--unshare-net`, and a private `/tmp` and `/var/tmp`. The probe is
`docs/adr/0097-wall-venue.probe.sh`.

| arm | wall | result | wall time (load) |
|---|---|---|---|
| uncaged | none (an operator's `make test`) | 34 pass | 75.0s warm (6.0) |
| seat-proc | D17 base, host `/proc` (ADR 0096 arm A) | 15 FAIL, 17 skip, 2 pass | 42.4s, 45s (5.8, 11.0) |
| **venue** | D17 base + `--unshare-pid --proc /proc` (ADR 0096 arm C) | **34 pass** | 103.7s, 96.3s, 99s cold (13.3, 6.4, 8.1) |
| venue, no private `/var/tmp` | as above | 16 pass, 18 skip | 18.8s |
| judge + venue | `scripts/posse-landed-check`, then the pins, in one venue run | green, 34 pass | 199.8s cold (7.4); the judge's part 113s |

- **No pin needs host pids.** `wallwatch_linux.go` reads `/proc` for the
  seat's mount namespace and the seat's processes. In the venue those are
  the venue's own descendants, and every wall-watch pin passes.
- **Every skip in the fourth row has one cause.** `wallUngrantedDir` makes
  its fixture root under `/var/tmp`, which `--ro-bind / /` makes read-only.
  A helper change unrelated to the wall took 18 pins dark. That is the case
  for counting abstentions rather than reading the exit code.
- **The judge's body is green in the venue.** Nothing in ADR 0088 D6's doors
  needs host pids either.

## Decision

**D1. The venue is the judge's cage plus exactly three things.** These are
`--unshare-pid --proc /proc` (arm C), a private `/var/tmp` bound from the
run dir, and `POSSE_WALL_VENUE=1` added to that run's environment
allowlist. It is rendered by the same function as the judge
(`boxLandedCage`), through a flag, never by a second renderer. A QA pin at
the real call site asserts that the venue's argv, minus the judge's, is
those additions and nothing else. Because of this, every mask added to the
judge later reaches the venue too (ranger-base-hy1aj's lesson). The venue's
argv is printed in the ADR 0088 verdict line.

- It is never weaker than the judge.
- It is stronger than the judge and than a seat on signals: host pids are
  invisible to it, as ADR 0096 arm C measured.
- It is **weaker than a seat on one axis**: it is root in a user namespace
  of its own, which is the capability it exists for. ADR 0096 measured that
  this does not reach the wall's denies (B6 refused in arm C). It is still
  the kernel's unprivileged-userns surface, handed to landed code. Today
  these pins measure only in an uncaged operator run, which has that
  surface and everything else, so the venue narrows the status quo. The
  residuals are listed under *Not closed here*.

**D2. It is a second check after the judge, not part of the judge's body.**
It runs after each judgement that answered **green**, on the same tree, and
under ADR 0088 D4's rules: one in flight, and coalesced to the newest tip.
When the judge reds or does not answer, the wall check records "not run
(judge <verdict>)". That counts as neither green nor silence, so one broken
tree is filed once. On a host whose wall mechanism is not bwrap, the step
does not exist (ADR 0096: no darwin change), and the judge's line says so.

**D3. The repo owns the body: `scripts/posse-wall-check`.** It keeps ADR
0088 D1's contract: no arguments, cwd is the tree, and the copy that runs
comes from this check's last green tree. It has **three exits**:

| exit | meaning | when |
|---|---|---|
| 3 | no answer | the baseline `bwrap --ro-bind / / /bin/true` does not start in the venue (stderr is the tail) |
| 1 | red | any member FAILs |
| 3 | no answer | no member ran, or a member skipped; the tail names each one |
| 0 | green | every member passed; the last line is `wall check: green, <n> pins measured` |

The body runs `go test -count=1 -json -run '^Test(QA)?Live(Bwrap|Wall)' ./internal/posse`.
A **member** is a test whose output carries D4's marker line. Membership is
read from the run and never inferred from the name. The baseline goes first
for the reason ADR 0086 D5 gives: without proof that the wall starts, a pin
that expects a refusal passes over nothing.

**D4. ADR 0096 D3's helper carries the venue contract.** The helper is
the nest helper that ranger-base-pvkhn builds.

- It logs `wall venue: measuring (ADR 0097)` whenever it lets a pin
  proceed.
- When `POSSE_WALL_VENUE=1` and nesting is refused, it fails instead of
  skipping. The baseline has already proved that this venue nests, so a
  refusal here is something the tree did.
- It fails any caller whose name does not match `^Test(QA)?Live(Bwrap|Wall)`.
  The three current outliers are renamed. That way the D3 filter and the
  membership cannot drift apart.
- It lives in an arm 1 file. A member written into arm 2 or arm 3 does not
  compile, so the venue never needs those builds.

Outside the venue, its behaviour is exactly ADR 0096 D3's.

**D5. Who reds: ADR 0088's ratchet and watchdog, keyed separately.**

- **Records.** `posse.wallCheck = <tree> <green|red>` and
  `posse.wallCheckQuiet = <tree> <n>`.
- **Ratchet.** ADR 0088 D5's table applies unchanged. A green-to-red step
  files one `-t bug` P1 `-l code` bead titled "wall pins red at <short
  tree>". It names every landing in the window and is assigned to the
  newest one's persona.
- **Watchdog.** ADR 0088 D6.3's table applies unchanged. Exit 3, a venue
  that will not render, a host with no bwrap or no unprivileged user
  namespace, and a deadline all count as unanswered. The second consecutive
  one files "wall check silent at <repo>" (`-l architecture`), carrying the
  tails. **An abstention is never a pass.** It reaches a reader on the
  second run, never the twentieth.
- **Bounds.** `landedCheckLimit` (10m) bounds a hang, and D6.2's 240s
  ceiling is read off answered runs, with the same over-ceiling bead.

**D6. The uncaged operator run stays the control, and is not the venue.**
`make test` uncaged still runs every member. Nothing requires it.

## Consequences

- The 32 pins have a scheduled place to measure. A wall regression is found
  about 100s after the judge answers green, and it arrives as one
  attributed P1.
- Cost: one cold venue run of about 100s per coalesced green judgement
  (MEASURED 96–104s), outside the lock, holding no seat. ASSUMED: at most
  about 52 a day (ADR 0088 D6.2's landing count).
- A caged seat's `make test` stays green on these pins (ADR 0096 D3). The
  seat abstains, and the venue measures.
- `TestLiveBwrapGitRefsRefusesTheSharedGitState`'s false pass on a seat is
  closed by ADR 0096 D3's helper (it sits behind `bgLiveSkip`). In the
  venue, the baseline and the helper prove the wall starts. A pin whose own
  exec fails for some other reason still reads as a refusal. That is the
  pin's ADR 0086 D5 obligation, and nothing here discharges it.

## Alternatives rejected

- **Fold the pins into the judge's body.** 199.8s against D6.2's 240s
  ceiling, and the judge alone has read up to 150s. It would also mix "red
  for every seat" (D6's subject) with "red nowhere a seat looks". And it
  would give the nesting capability to every judgement, whatever the
  verdict. D2 keeps the cage function shared and the run separate.
- **ci.yml's Linux leg.** It runs at the operator's push, hours after the
  landing, on another kernel, with public logs, and it attributes nothing.
  ASSUMED, not measured here: Ubuntu runners restrict unprivileged user
  namespaces through AppArmor, so bwrap there needs a sysctl change first.
- **A timer, or a scheduled uncaged operator run.** An uncaged run runs
  landed code without a cage (ADR 0088 D3). A timer loses attribution and
  saves nothing, because coalescing already bounds the rate.
- **Run only when a landing touches wall files.** The `/var/tmp` row is the
  counterexample: a fixture helper took 18 pins dark.
- **The helper fails instead of skipping everywhere.** It reds every caged
  seat, which ADR 0096 rejected. D4 fails only in the venue.
- **`--proc` in the seat render.** ADR 0096 D2.

## In flight

- ranger-base-pvkhn (ADR 0096 D3's helper; D4 extends it)
- ranger-base-fcxyz

## Not closed here

- ranger-base-1xyso

## Exit hatch

Delete the step, the script and the two config keys. The pins go back to
measuring only when an operator runs them uncaged, which is today's state.
The venue is one flag on `boxLandedCage`. Nothing holds state beyond the
two keys.
