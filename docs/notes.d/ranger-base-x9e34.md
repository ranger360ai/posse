# ci-watch — a red gate on `main` files one bead (ranger-base-x9e34)

`internal/posse/ciwatch.go`, wired into the dispatch pass beside verify-after.
Discovered from ranger-base-90y3c ("CI is red on every push to main and has
been for hours, so the only gate a bead branch ever passes is not a gate").

## What was wrong

`ci.yml` is the only gate a commit on `main` ever passes — merge-back
fast-forwards a bead branch onto `main` and opens no pull request, so nothing
runs before it lands. Measured 2026-09-04 over the workflow's whole 300-run
history:

    last green   0c0607b0  2026-08-30T01:23Z
    first red    8d50fed5  2026-08-30T01:53Z
    every run since         red — 191 failures, ~120 commits, 5 days

Nobody noticed for five days, because nothing said so anywhere a person or a
seat looks. The cost is not the reds; it is the **attribution**. While the
gate stands red, a red run says nothing about the commit it is attached to,
so a genuine break — `internal/posse` not building at all, twice, for over an
hour each — is indistinguishable from the standing noise.

## The shape, and the two things it must not do

The design bead named the shape and rejected the alternative: a GitHub
notification setting is not versioned and reaches one account. So: a
dispatch-loop check that files a bead on a red main run and closes it when
main is green again.

**Half of that shape did not survive contact with ADR 0013 §4**, and that is
the most important thing in this file. The close is covered under "What did
not ship" below.

Two failure modes were named up front and both are pinned:

1. **One bead per episode, never one per push.** A mechanism that files per
   red run during the next five-day red is worse than the silence it
   replaces.
2. **An abstention must never read as green.** A gate that HAS a workflow and
   could not be read is said once and out loud; silence is what an all-clear
   looks like.

The second one needed a distinction the first cut did not have, and the suite
supplied it — see "The abstention the suite corrected" below.

## The measurement that settled `cancelled`

`gh run list --workflow=ci.yml --branch main --limit 300`, 262 completed
runs, replayed through the state machine:

| `cancelled` treated as | beads filed over 6.6 days |
|---|---|
| **no verdict (shipped)** | **7** |
| red | 16 |
| green | 13 |

20 of the 262 completed runs were `cancelled`, all from the era when pushes
to `main` shared a concurrency group. A run GitHub stopped is a statement
about the queue, not about the code, so it is skipped and the next run down
answers instead. 7 beads against 196 red runs is the blast radius, measured
rather than argued.

The shortest episode would have been open 15 minutes (red 20:29:09Z, green
20:44:10Z on 2026-08-29), so **no refile cooldown is carried**: a red that
follows a green is a new red, and the previous episode's bead is sitting
beside it in the store carrying the comment that says it is over.

## Decisions

**The dedupe is a store read, not process state.** An open bead carrying
`ci-red` whose description holds `ci-red-gate: <slug> <workflow> <branch>`.
A launcher restart and a second launcher see the same one bead. The marker rather than the label alone, because every repo here
redirects `.beads` to one queue — the ci-red bead for one gate is in the
listing the other gate's dedupe reads, and label-only dedupe would let the
first red gate silence every other one for as long as it stayed red.

**`ciOpenBead` asserts OPEN itself** rather than leaving it to the query.
(SUPERSEDED by `ranger-base-bwrp8`: the assertion moved into
`Bd.OpenLabeledAny`, where the promise is made, and the local guard came out
— a duplicate would have hidden a regression of the general one from the very
test that found this. The measurement below still stands.)
Measured 2026-09-04 on bd 0.50.3: `bd list --label-any <l> --json` drops
closed rows on the shop's SQLite store (391 of 396 `-l qa` beads are closed,
5 come back) and KEEPS them on the `no-db: true` JSONL store `bd init` writes
today. `ciwatch_live_test.go` failed on exactly this before the line existed.
A dedupe that adopted a closed bead would never file again: the gate goes red
for five days and the mechanism sits holding a bead that says the last
episode is over.

**The drumbeat state lives on the bead.** While the gate stays red the streak
is re-said when it has doubled — 1, 2, 4, 8, 16 — so 191 failures earn eight
comments and not 191. `launcherlag.go`'s cadence and `watch.go`'s backoff,
for the same reason both have it. The "last said" number is read back out of
the description and the comments through one parser rather than kept in the
process, because a launcher restart is the ordinary case here (the incident
outlived several) and process state would re-say from 1 on every one.

**The reading is outside the launcher lock; the writes are inside it.** The
lock is needed because the dedupe is a read followed by a create and two
unserialized launchers file two beads for one red (verify-after's own
reason). But the reading is a `gh` child over the network — 2.8-4.2s measured
here — and holding the launcher lock across that would park the fire loop and
freeze the cockpit for those seconds on every pass, including every pass
where the gate is green. An instance whose repos all abstain never touches
the lock at all. Both arms are pinned.

**Not in `posse status`, and that is a measurement.** `posse status` costs
3.87s; one `gh run list` costs 2.8-4.2s, so the reading would roughly double
the latency of the one command an operator waits on — to answer a question
the filed bead already carries. `launcherlag.go` takes the opposite decision
for the opposite reason: it is a local `git rev-list --count` and it has no
bead.

**The pass speaks only when it acts.** A line on the pass that files and the
pass that clears; nothing on the passes in between. The standing condition is
the bead's to carry — a loop that re-announces it every pass for five days is
how a visible line becomes an invisible one.

**Nothing from a commit message rides into a bead.** Shas, run URLs,
conclusions and timestamps only; `displayTitle` is deliberately not among the
fields asked for. Guardrail 4 is satisfied by construction rather than by a
filter.

**verify-after exempts a `ci-red` close that no commit names**
(`verifyafter.go`, beside the rejection exemption). Closing a bead because
the gate cleared itself built nothing for a QA session to look at, and
without the exemption the 7 episodes measured above each cost a second seat
on top of the close. The second signal is the same one the
rejection exemption uses and for the same reason — the label alone must never
suppress a control: a persona who actually fixed CI under the bead leaves
commits naming it, and that close earns its verify bead like any other.

## The abstention the suite corrected

The first cut said every abstention out loud, once per process. That turned
**22 tests red** — every dispatch and plan-guard pin that runs a clean pass
over a `t.TempDir()` and asserts nothing reaches stderr — and they were
right. Two kinds of abstention were being conflated:

| kind | examples | says |
|---|---|---|
| **no gate here** (`CIState.NoGate`) | not a git checkout, no such workflow file, no `github.com` origin, `ci_workflow:` empty | nothing, ever |
| **gate unread** | no `gh`, an unauthenticated `gh`, a network that did not answer, unparseable JSON, no verdict-bearing run in the window | once per process |

A repo with no CI is not a repo whose CI could not be read. Nothing is
hidden, there is no all-clear to mistake, and the fact is true on every pass
forever — so a line about it is noise in the one stream that has to stay
signal. A gate that exists and went unread is the opposite: silence there is
indistinguishable from green, which is the failure this whole file is about.
Both arms are pinned; either alone passes over a mechanism that is uniformly
silent or uniformly loud.

## What did not ship then, and shipped under ranger-base-4gy4i

`ranger-base-x9e34`'s DONE WHEN asks for a bead **closed** when main is green
again. ADR 0013 §4 rejects "harness closes the bead on the agent's behalf" in
as many words — "resume-until-record is the harness's job; `bd close` is the
persona's" — and `absencerules_qa_test.go`'s
`TestNoBdCloseVerbReachableFromDispatch` enforces it by reachability: no `Bd`
close verb may be reachable from any `*Dispatcher` method. Arm 1 has a
register (one row, the operator verb `posse done`); **arm 2 has none**. The
first cut of `ciwatch.go` closed on green and that pin caught it on the full
suite, which is the gate working.

Amending a constitutional pin is not this bead's to do, so the green half is
a COMMENT: `ci-red cleared: <workflow> is green again on <branch> — <sha> at
<t>, <url>` plus "CLOSE IT" and why the harness did not. `ciAlreadyCleared`
reads that comment back, so a cleared bead does not suppress the next red —
one bead per episode still holds; what changes is that a bead outlives its
episode.

**What that costs, measured.** Of the 7 episodes over 6.6 days, six
self-healed (the gate went green under some other commit), so six beads would
be filed, cleared by comment, and left for a persona to close — a dispatched
session that reads one comment and closes. That is the price of the rule as
it stands, and it is not obviously wrong: a person confirming the gate is
green is exactly what §4 wants. Whether §4 admits a narrow exception for a
bead the harness itself filed about a CONDITION and never dispatched — nobody
is graded by a bead nobody claimed, which is the harm §4 names — is asked in
`ranger-base-8fr2j`, with the three candidate rulings and what each would
need. It is deliberately NOT a blocker on this bead: the filing half is the
whole of "nothing tells the crew", and it shipped.

**RULED (Dave, 2026-09-05, on `ranger-base-8fr2j`): candidate (b).** ADR 0013
§4 admits one narrow exception — the harness may close a bead **it filed**
that **no session ever claimed** (status still `open`, no assignee). Built
under `ranger-base-4gy4i`: `ciHolder` is the predicate, read off the bead
rather than remembered; the clearing comment is written FIRST and is
therefore the close comment, so a closed ci-red bead names the run that
answered it and a close that fails leaves exactly the state described above.
A bead a seat holds keeps every word of this section: the comment, the CLOSE
IT, and the close left to the seat. `absencerules_qa_test.go` grew arm 2's
own register — one row, `App.ciClear`, the same shape arm 1 has — and the
verb half of that arm now cuts the registered caller's close edge before it
asks, so a second, unregistered closer is still red. The measured saving is
the six self-healed episodes above: ~6 dispatched sessions a week.

## Verification

- **Live, against the real gate.** `ReadCI` against `ranger360ai/posse` on
  2026-09-04 read red, latest `d3909c27` 12:57:34Z, streak 100 reported as a
  floor (the real streak is 191, past the 100-run window one `gh` page
  covers), since `624f5796`. The floor clause exists for exactly this.
- **Live, end to end, against real bd.** `ciwatch_live_test.go`
  (`RHQ_LIVE_BD=1`): a throwaway `bd init` store and a real `gh`-shaped child
  serving a runs file the test rewrites. Green → nothing; red → one bead;
  four more red passes → still one bead and nothing said; streak 1→3 → one
  comment, and a second pass at the same streak → none; green over a bead a
  seat holds → the clearing comment and NO close; a second red → its own new
  bead; green over a bead nobody claimed → the clearing comment and the
  harness's own close, after which the dedupe steps over it. It has been red for a real
  product defect once — `ciOpenBead` trusting bd's query to drop closed rows
  — which is what makes its green mean anything.
- **The full suite, twice.** The first run is what found both of the design
  errors above: `TestNoBdCloseVerbReachableFromDispatch` (the close) and 22
  dispatch/plan-guard pins (the abstention). Neither was reachable from the
  package's own tests.
- **22 mutants, all killed by a named test** (`go test -overlay`, no golden
  copy in the worktree). Including: cancelled counted as red; the dedupe
  adopting a closed bead; the dedupe matching any gate's marker; the drumbeat
  saying every pass; filing per pass; the streak counting past the leading
  block; the off switch doing nothing; the bead losing its routing label; the
  abstention repeating; closing without the comment that says why; trusting
  `gh`'s order; reading a gate the repo does not have; filing on a failed
  dedupe read; the lock held across the network read; and both arms of the
  verify-after exemption.

Three of those mutants only became killable after the fixtures were fixed,
and each fixture was lying about something real:

- the marker mutant survived a two-repo fixture built from two DISJOINT
  stores, where the marker is never load-bearing — this shop's repos all
  redirect to one store, so the fixture had the wrong shape;
- the closed-bead mutant survived every hermetic pin until the fake bd
  learned to model the store class that KEEPS closed rows
  (`fake-list-keep-closed`);
- the newest-first mutant survived because `fakeBdAppendCreated` stamped no
  `created_at`, so every bead it filed sorted equal and the comparison could
  point either way. Real bd stamps one; the fake does now.

## Config

`ci_workflow:` (default `ci.yml`, present-but-empty = off) and `ci_branch:`
(default: whatever `origin/HEAD` names, then `main`). Documented in
`examples/config.yaml`.

## Also found

`ranger-base-bwrp8` (`-l code`): `Bd.OpenLabeledAny` is open-only because of
the store class it happens to be pointed at, not because of anything its
query asks for. Its own doc comment and `AllLabeledAny` beside it both state
the filter as bd behaviour, and on one of the two store classes bd 0.50.3
ships it is not — measured above. Nothing on this box is affected (the shop
store is SQLite), but governance G3's open question/risk count and the
closed-dirty handoff would both mis-read on a fresh instance, because a fresh
`bd init` writes the other class. ci-watch defends itself locally; the
general fix and its two other readers are that bead's.
