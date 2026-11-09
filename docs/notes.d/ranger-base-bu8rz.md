# The bound that could not be reached at this shop's batch size (ranger-base-bu8rz)

MEASURED 2026-11-09, darwin 25.6.0 arm64, at main a1b6d6ef. The instruments
are `scripts/chain-depth-census.py` over one shop's `issues.jsonl` and the
verify-after suite's own rig; re-run them, the corpus grows.

Filed from the verify of ranger-base-ewnas (ranger-base-41g0y) as a separate
dispatch-correctness bug, ADR 0006 §6.

## What happened

`verify_chain_max:` (ranger-base-ewnas, commit a1b6d6ef) shipped the day
before this, to stop a verify/fix pair iterating on one artifact without
bound — ~20 rounds on a single config-test pin on another instance's project
crew. It could not fire on the crew this harness actually serves, because
that crew runs `verify_batch: 4`.

One return is the whole mechanism. `verifyChainRounds` counts a hop, reads
the parent verify bead's source markers, and stops when there is not exactly
one:

    hops = append(hops, hop)
    if hop.Source == "" || seen[hop.Source] {
        return hops, true // a batch, or a graph that loops: stop short
    }

`hop.Source` is `""` whenever the parent verify bead answers other than one
close — that is, whenever it is BATCHED. The escalation needs
`len(hops) >= ChainMax`, so at the default 3 it needs two consecutive
single-close verify beads above the close. Above a batch of 1 there are
none, so every walk counts exactly one round however deep the lineage runs.

Neither half of the bound reached anybody. The escalation never fired, and
`verifyChainHead`'s line stuck at `VERIFY ROUND 2 of 3` on a lineage of any
depth, so the sentence that says THIS IS THE LAST ROUND was never printed.

## The measurement

Over one shop's bead store (3,353 beads, 2,639 of them not verify beads),
671 verify beads carry at least one source marker, counted exactly as
`verifySourceIDs` counts them:

    answers 1 close: 320    answers 2: 3    answers 3: 3    answers 4: 345

Per month, single-close (the walk CONTINUES) against batched (it STOPS):

    2026-08   single 191   batched  85
    2026-09   single 125   batched 176
    2026-10   single   4   batched  56
    2026-11   single   0   batched  34

Every close on that queue whose lineage reads 3 rounds or more — eleven of
them, the same eleven `DefaultVerifyChainMax`'s census prices the bound at —
closed between 2026-08-22 and 2026-09-06, while the shop was still filing
single-close verify beads. **Nothing has been reachable by this bound there
since 2026-09-06.**

By execution: at `verify_batch: 4`, a lineage of 3, 6 or 12 batched rounds
files one verify bead, escalates nothing, and prints `VERIFY ROUND 2 of 3`
in every case. The control — the same depth, single-close — escalates. The
rig and the bound both work; what the batch defeats is the count.

## Why not walk the batch

The obvious repair is to walk all N sources of a batched verify bead and take
the deepest. MEASURED the same day over the same store, it is worse than the
hole: a deepest of **17 rounds where the lineage reads 1**, and at
`verify_chain_max: 3` it escalates **202 of 2,639 closes (7.65%)** against the
lineage key's 11 (0.42%).

The 17-deep chain is visibly not one subject but more than a dozen. Reading
its hops by eye, among them: a backup-lock CI-red close, a suite-arm equality
close, a CHANGELOG-paragraph close, a `GIT_CONFIG_SYSTEM` row close, a
seatbelt credential read-deny close, a FIFO-at-`.beads/redirect` close, a
free-space-floor close, a go-build-cache close and a herdr readiness-gate
close — chained only because batching happened to put each consecutive pair
in one verify bead. That is the same failure the per-artifact
PATH key was measured and rejected for under ranger-base-ewnas ("a hot file
is not an artifact"): **a batch is no more an artifact than a hot file is.**
An overcount here is unrecoverable in the direction that matters — the
watermark walks past an escalated close, so nothing re-examines it, and 7.65%
of closes would lose their verification to buy a bound on 0.42%.

## What shipped, and what did not

Shipped (shape (d) of the four the bead named): `verifyChainBound` returns 0
and names the combination on stderr when `verify_batch:` is above 1, so an
operator reads *this bound does not bind at your batch size* instead of
believing in one that cannot fire. Off means off — no walk, no `bd dep list`
per close, no round line — and the close still gets its verify bead, so the
bound going quiet costs no coverage.

**This fixes the record, not the bound.** The loop ranger-base-ewnas was
filed to stop is still unbounded at `verify_batch: 4`; it is now unbounded
and SAID SO rather than unbounded and reported as bounded. The two keys are
an either/or today: batch the filing, or bound the depth.

The bound itself needs provenance the harness does not write. The walk's
ambiguity is one step wide and in one direction: going UP from a bead to its
verify parent is unambiguous (one `discovered-from` edge), and going up from
that verify bead to *which of its N closes this lineage continues through* is
not derivable from anything written today. `fileVerifyBead` comments
`verify filed: <qid>` on each close, which is close→verify; the markers are
verify→{closes}; neither says which close a child bundle answered. Making
that pair recoverable at filing time is shape (c) of the bead's four, it is a
change to ADR 0006 §3's decision, and it went to architecture rather than
being invented here.

## DIVERGED

ADR 0006 §3 says `verify_chain_max` (default 3) bounds the rule's DEPTH, and
that `0` disables the bound. It does not say that a `verify_batch:` above 1
disables it too, which is what now ships. The line that no longer holds is
§3's pricing of the stop — "a batched verify bead ends the walk … so the
count errs short by construction" — which is true, and which prices the
undercount at "one more round" one paragraph earlier in
`verifyChainRounds`. Above a batch of 1 the undercount costs every round
after the first, without bound. The amendment and the real fix are
architecture's.
