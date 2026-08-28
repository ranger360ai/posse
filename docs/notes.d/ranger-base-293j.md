## Measuring a barrier from inside the barrier (ranger-base-293j)

ADR 0028 §5 observable 1 asks for idle-to-next per seat — settle → next
launch — measured BEFORE the rolling refill ships, so the "~seconds after"
claim has something to be false against. The instrument is `seatidle.go`
and it is deliberately inert: nothing reads the ledger back into a
decision, so the "before" number is taken by a dispatch that behaves
exactly as it always did, and the new store adds no way for two stores to
disagree about a fact (ADR 0011).

Three things about it are easy to get wrong, and two of them are wrong in
the direction that flatters the change.

**The settle timestamp is stamped in the waiting goroutine, not at the
gather.** A pass gathers its pending beads in launch order. A bead that
settles in three minutes behind one that runs seventy-five is not *read*
for seventy-two more, so `time.Now()` at the gather dates its settle at
the moment the barrier let go — and the barrier is the latency being
measured. Measured that way, the baseline is smaller than the shop, by
exactly the amount ADR 0028 §1 proposes to remove. `promptResult.at` is
the fix and it is two lines; the trap is that both versions compile,
both run, and only one of them can be falsified later.

The one settle that cannot be stamped honestly is the poll after a `--wait`
leg runs out: the settle happened somewhere inside the leg and only its
discovery is datable, so that window is short by up to one leg. Named in
the code because it is the instrument's only systematic bias.

**File order decides which settle is newest, not timestamps.** The ledger
line is RFC3339 — second granularity — and a bead can settle inside the
second it launched in (a stranded pane, a refused first turn). Comparing
those timestamps ties, and a tie read as "unordered" refuses a window that
is perfectly good. The ledger is append-only and a seat is one persona in
one repo, so the lines are already in the order the events happened: ask
the last line what kind it is.

**A settle that does not free the seat is not a window.** `personaActive`
reads *blocked* as the persona being busy, so a bead that settles blocked
keeps its claim and its seat and is waiting on a human. Counting that wait
as dispatch latency puts the operator's response time inside the harness's
number. Blocked settles are therefore not recorded as freeing events, and
a seat freed by hand afterwards falls into the "previous settle not
observed" branch — a named non-figure rather than a wrong one.

That last shape is the general rule the whole file is built on: **unmeasured
is not zero.** Every case the ledger cannot honestly subtract reports why
instead of a duration, and the pass line says how many of its refills were
measured — because a control arm whose gaps are silently filled with the
target value is not a control arm.
