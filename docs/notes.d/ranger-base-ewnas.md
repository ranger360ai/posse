# A verify/fix pair has no bound on depth (ranger-base-ewnas)

MEASURED 2026-11-09. The instrument is `scripts/chain-depth-census.py`,
which reads one store's `issues.jsonl` and nothing else; re-run it, the
corpus grows.

## What happened

MEASURED 2026-11-08/09 on another instance's project crew — two personas, a
coder and a verifier, on a repo of two config files and their test. Each
verify close filed a `findings (N)` bead against the same test file, the
coder closed it, verify-after filed a verify bead about THAT close, the next
verify found more: about twenty rounds (bash quoting, rsync pattern grammar,
ANSI-C escapes). A coordinator note on one bead did not stop it, because
every follow-up was a new id and the note was on the old one. A human paused
the home.

Nothing in that was a defect. Each finding was real, each fix was a fix,
each verify bead was the rule in ADR 0006 §3 working exactly as written.
What was missing was a number: nobody was counting rounds, so nobody could
see the loop from inside it.

## Why the existing bounds do not reach it

ADR 0006 has bounded this cycle twice, and both bounds are on the FAN-OUT of
one round:

- 2026-09-02 bounded beads PER VERIFY CLOSE — one findings bundle instead of
  one bead per finding. The ADR's own Consequences records that the count
  kept growing, "because the bound was denominated in closes and closes were
  the growing number".
- 2026-09-06 (§6) changed the CHANNEL: a finding whose fix would change
  nothing a process does is RECORDED as a line on the verify bead's close
  comment and files no bead at all. That is the better bound and it is still
  about one round's output.

A chain of LIVE findings, each genuinely live, each filed as one bundle,
walks as far as the two seats will carry it. `verify_batch:` does not help
either: it divides how many closes one verify bead answers, not how many
times a lineage comes back.

## The depth, measured

Over one shop's whole bead store (3,353 beads, 2026-08-12 .. 2026-11-09;
2,634 of them not verify beads), rounds already spent on each close's
lineage, counted by exactly the walk the harness now makes:

| rounds | closes | share |
|---|---|---|
| 0 | 2055 | 78.0% |
| 1 | 527 | 20.0% |
| 2 | 41 | 1.6% |
| 3 | 8 | 0.3% |
| 4 | 3 | 0.1% |

Deepest observed: 4. So 98.0% of closes sit inside one round. A bound of 3
would have escalated 11 closes in 89 days — about one a week for the
operator — against ~20 sessions spent on one artifact in the incident. That
is where `DefaultVerifyChainMax = 3` comes from; it is not a round number
chosen for looking reasonable.

Every lineage this key reads at 3 or more is one subject by eye: the
GIT_CONFIG inlet pin four rounds running (`53y2k → nn161 → m3moe → sv8x4 →
txxgs → txsed → 6z06r → hegxj`), the common-dir pin three.

## The artifact key: measured and rejected

The bead proposed keying the chain on the ARTIFACT — "beads discovered-from
a verify of a bead that was itself a verify finding on the same path" — so a
chain that moved to a different file would start a fresh count. It is the
better idea on its face: it is the artifact that is or is not converging.

The census measures it too (argument 2 is the repo, and the trail is every
commit whose message names a bead the store holds). Two rounds are one chain
when the closes' commit path sets intersect, and a batched verify bead is no
longer ambiguous because the path says which of its closes the bundle
answered — which is the key's whole appeal. It reads **137 closes at 3
rounds or more, 10 of them at 8 or more, deepest 13**, and those chains are
not one artifact. One 11-deep chain runs a cost-provider bead into a
silent-revert bead into three `posse worktrees` beads into four
common-dir-pin beads into a seatbelt bead. They chain because every one of
them touches `internal/posse/dispatch.go` and `NOTES.md`.

A hot file is not an artifact, and in this tree most files are hot. The key
would escalate chains nobody is looping on, and an operator reading one
could not tell which it was.

The lineage key needs no path at all, because at `verify_batch: 1` — the
default, and the incident's shape — a verify bead answers exactly one close,
so "filed from the verify of that close" already means "about that close's
work". Subject continuity comes from the one-source rule for free.

## What shipped

`verify_chain_max:` (default 3, `0` off) in `internal/posse/verifyafter.go`.
Three pieces:

1. `verifyChainRounds` walks up from a pending close: its first
   `discovered-from` parent, if that parent is a `qa` bead, then the one
   close that bead's description markers say it answered, and again. At most
   `limit` `bd dep list` calls, and none at all when the bound is off. The
   LISTING the pass already made answers everything except the edges.
2. At or above the bound, `fileVerifyChainStop` files one `-l question` bead
   for `operator:` instead of the verify bead, and that is the stop: no
   verify bead, no verifier session, no findings bundle, no next close. The
   cycle ends at the one hop the harness owns. Nothing is blocked and
   nothing is reopened — the subject is a close, and there is nothing in the
   queue to block (which is the difference from `settleopen.go`, whose stuck
   bead is live and leaves `bd ready` on an edge).
3. Below the bound, `verifyChainHead` prefixes the verify bead with the
   round it is: `VERIFY ROUND 2 of 3`, and on the last one a sentence saying
   that a close filed from this bead goes to the operator. The harness can
   only stop the round after next; a verifier who knows it is on the last
   round can stop this one. It renders "" for a first-round filing, which is
   78.0% of them, so those descriptions are byte for byte unchanged.

Three properties worth keeping in mind, each the safe direction of a choice
that could have gone the other way:

- **The count errs SHORT.** A batched hop ends the walk rather than guessing
  which of its N closes a bundle answered. A lost `discovered-from` edge
  (bd's non-atomic create, `verifyMarkerPrefix`'s note) is a hop the walk
  cannot see. Both undercount, and an undercount costs one more round where
  an overcount costs an escalation nobody earned.
- **A FLOOR cannot reach the bound.** The walk appends one hop per answered
  `bd dep list` and only runs while `len(hops) < limit`, so a walk that ends
  in a bd failure has counted strictly fewer than `limit`. That is why the
  escalation needs no second condition: `len(hops) >= limit` means every
  edge the count rests on was really read.
- **The dedupe is the escalation's TITLE, over OPEN AND CLOSED beads.** The
  title is written by bd in the same breath as the issue, so an orphaned
  create still dedupes. Closed counts, which is the opposite of
  `settleopen.go`: there, a bead that sticks again after a human looked at
  it is news again, because the bead is still in the queue. Here the subject
  is a close that has already happened and the watermark walks past it once.

## Two gotchas this bead paid for

- **The fake bd's `dep list` was per REPO, not per issue.** One
  `fake-deps.json` answered every id alike, which is enough for a caller
  asking about the one bead it just filed against and wrong for a caller
  that walks a chain — hop 2 reads hop 1's answer. `fake-deps.<id>.json` is
  that id's own answer now, with the per-repo file as the fallback, so every
  fixture written before this reads exactly as it did.
- **A number config key has three registers to join**, or
  `TestSeedConfigDocumentedNumberDefaultsAreTheConstants` reds by name: the
  reader in `seedNumberBodyReaders`, the constant in `seedNumberDefaults`,
  and the measured population floor in the pin's own Fatalf. They are total
  on purpose — a reader the tree has and the table does not would read every
  key that reader owns as undocumented and hold nothing about them.

## What to measure next

ASSUMED, after a month: that the escalation rate stays near one close a week
on a queue of this size, and that an operator's answer is cheaper than the
round it replaces. Read it as the count of `verify chain stopped:` question
beads and how many were answered by filing one more verify bead by hand. If
most were, the bound is one too low.
