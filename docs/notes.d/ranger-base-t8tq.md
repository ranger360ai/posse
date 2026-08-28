## The pass stopped ending, and three things denominated in it broke (ranger-base-t8tq)

ADR 0028 §1 made the dispatch `Run` long-lived: a seat whose bead settles is
refired inside the same `Run`, so a busy shop's `Run` does not return and
`--watch`'s pass loop does not come round again. MEASURED on a live loop
2026-08-28 (`state/dispatch-watch.log`): one pass ran **7h09m**, and the next
pass line in the log is a bounce, not a second pass. Everything still written
"per pass" quietly became **per process**.

**1 — The reap went silent.** `autoReapPass` ran at `Run` start and as
`Run`'s epilogue. Both are once per process now, so 26 per-bead sessions over
closed beads piled up across the day; bouncing the loop swept all of them in
its first seconds, which is how the sweep code itself was established
healthy. The fix is placement, not predicate: **the sweep now runs
at every settle** in the gather loop — the settle is the event that *makes* a
per-bead session sweepable — and the two old call sites stay for the shapes
that have no settle (a `Run` that starts after a crash, a quiet pass).

**2 — A seat busy at the head of the pass stayed busy all day.** Two halves,
either one enough on its own:

- The `busy` map (ADR 0028 §3) held both this `Run`'s own fires *and*
  everything a fire pass merely *read* about a seat — `personaActive` found
  it working, another launcher had prompted it inside `PromptGrace`, its CLI
  failed twice. Under a one-shot `Run` those readings expired with the pass.
  Under a rolling one they never expired: the lane line printed seven hours
  in still named the seats that were busy at 08:46. Occupancy is now split
  (`seatMap`): the **Run** map holds seats this `Run` fired into, released at
  their settle; a **fire pass** map holds its own readings and dies with the
  call, so every offer re-reads the seat live.
- The refill was narrowed to the persona that had just settled, so a seat
  this `Run` never fired into was never offered work again. ADR 0028 §1 says
  "re-runs the fire path for the freed seat" and rests on "the level-
  triggered tick still sweeps everything" — but that tick is `--watch`'s, and
  `--watch` does not get its loop back while the `Run` is refilling. **The
  settle is now the tick**: the refill runs the whole fire path (still under
  the operator's `--persona`, still refused by live occupancy), so any free
  seat with ready work is offered it within seconds of any settle. That the
  ADR's §1 text needs to say so is filed as an amendment on the architecture
  lane (ranger-base-ad4y).

Result on the day measured: ~90% of the closes were the one seat that kept
settling, while three other seats sat out seven hours with ready beads in
their lanes.

**3 — `-n` changed units under a flag that did not change.** ADR 0028 §2
re-denominated `-n`/`autostart_max_beads:` from per-pass to per-EPOCH
(`dispatch_epoch:`, default 1h). The running loop still carried `-n 6`, an
old per-pass number, so six launch attempts were the whole shop's hourly
ration and the fast seat spent them; the only trace was the exhaustion line,
which the starved seats never reached. `--watch` now prints its ration in the
unit it is spent in, once, at the top of its log:

```
◷ launch cap: -n 40 = 40 launch attempt(s) per 1h0m0s EPOCH, not per pass (ADR 0028 §2; autostart_max_beads:/dispatch_epoch:)
```

`autostart.sh`'s absent-key default is still **3**, which is 3 launches per
hour for the whole shop — a per-pass number wearing the epoch's name. It is a
tuning decision, so it is the operator's to make; the line above is what
makes it visible.

**What was NOT built, and why.** The bead's third ask was "consider a
per-seat fairness bound (max refills per seat per epoch)". Measured against
the log, fairness was not the binding constraint: the starved seats never
lost a race for the ration — they were never offered a seat at all, because
of (2). A quota would have added an invented number on top of a queue nobody
was drawing from. If observation after this lands still shows one lane
crowding others *while they are being offered work*, that is the moment to
size a bound against real numbers.

**Test lever added.** `fake-ready-next.json` in a fixture repo is the fake
bd's SECOND answer, swapped in once the first has been served — a queue that
moves between two `ready` calls of one `Run` (a bead closed and dropped, a
bead filed since). The canned list that answered every call identically made
every mid-`Run` behaviour untestable.
