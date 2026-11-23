# ranger-base-4yefx — a seat that read `working` for 41.6h over a closed bead

A dispatched seat whose bead had closed held its lane for ~41.6 hours while
every store posse reads said `working`, and `posse kill` refused it twice —
at its default 10m and again at `--timeout 20m` — with a sentence carrying no
evidence. Two asks, both answered here: a seat in that state should not read
as working forever, and the kill refusal should say that its working signal
is background tasks and not an edit in flight.

## The incident, from the launcher's own log

MEASURED 2026-11-23 from `~/homes/ranger/state/dispatch-watch.log` on this
instance (`multiplexer: rex`), over the whole window:

| | |
|---|---|
| bead `ranger-base-tvokd` closed | 2026-11-21T09:22:47-05:00 (`bd show --json`) |
| seat `<persona>-<repo>-ranger-base-tvokd` retired, by hand | ~2026-11-23T03:00 |
| held after its bead closed | ~41.6h |
| gather legs reading `working` | 16, from 15m0s to 4h0m15s, where `WaitCeiling` took the "claim kept, not judged this pass" arm and stopped judging it |
| land-sweep "workspace … is alive" lines | 252 |
| refill lines reporting that persona's lane busy on it | 53 |
| **lines of any kind naming a reap** | **0** |

The seat's own state came from the runtime: on Rex the reading is claude's
OSC 7501 `program_status` record (ADR 0076 §3, `rexReading`), and every gather
leg's refusal said `last reading: working`. The tree was clean and held no
commits, which is why `posse kill --no-land` retired it without loss once the
operator reached for that flag.

Nothing was broken. `settledForReap`'s `return false // working, blocked:
somebody is in there` held the lane, correctly by its own terms, on every
sweep for 41.6h — and said nothing, which is the shape `ranger-base-fytno`
and `ranger-base-ktiik` already ruled against for this sweep's other standing
declines.

## Why `working` was true and meant nothing

MEASURED 2026-11-23 03:15-03:29 on this box (linux, `/usr/bin/rex`, claude in
a live dispatched seat), by one seat sampling **its own** block every 20s
across its own turn boundary with a declared background task running —
`ranger-base-r5131`'s method, 36 samples, 22 in-turn then 14 post-turn:

| reading | in-turn | post-turn |
|---|---|---|
| root record `state` | `working` (22/22) | **`working` (14/14)** — never idle, never done |
| root record `msg` | the current tool call, moving every call | `1 shell still running` (14/14) |
| root record `updated_at` | moves with each call | **frozen** at the turn's end (one distinct value) |
| footer | `⏵⏵ auto mode on · 1 shell · esc to interrupt · ↓ to manage` | `⏵⏵ auto mode on · 1 shell · ↓ to manage` |
| `esc to interrupt` | 22/22 | **0/14** |
| `· 1 shell ·` | 22/22 | 14/14 |

So a claude seat that ends its turn with **one** background task live reports
`working` with no end. That is tvokd's 41.6h, reproduced on a live seat in
twelve minutes. The hint switched once, at sample 23, and never flapped back.

A child record per background task is also published now
(`{"id":"bcfq60zq6","state":"working","title":"Start the seat sampler in the
background", …}`) — those titles are what the operator read off tvokd's pane.
`rexRootRecord` selects the record whose id is empty, so posse reads the root
one and ignores these.

### What this corrects

`capturedHold`'s doc (panework.go) records, MEASURED 2026-11-10 on claude
2.1.322, that `program_status` "holds ONE root record across the whole settled
window — `state=done`, owner claude — and no child record". Three of those
four facts have moved: the state is `working`, the `msg` now carries the task
summary, and child records exist. The footer reading that doc argues for is
the one that did not move, which is also the argument for having built it
there. Both measurements are now dated side by side in that comment.

## Which discriminator the fix spends, and why not the other three

| candidate | why not |
|---|---|
| root `msg` ("1 shell still running") | prose, and Rex-only. herdr has no such field, so a reading built on it would be off on half the instances posse supports — and matching on a message is the shape this repo refuses everywhere else (`HerdrAPIError`'s own doc). |
| child records | present **during** a turn too (sample 21 carried both), so they say nothing about whether one ended. |
| root `updated_at` age | the strongest of the four — a `working` record whose state has not changed in 41h is unambiguous — but Rex-only, and `MuxReading` has no timestamp field, so spending it means a new occupant-specific reading on the `Mux` interface. More than this bead needs; the readings it would serve are worth their own bead if one ever needs them. |
| **footer `esc to interrupt`** | **chosen.** Cross-occupant (ADR 0076 §2's `Read`, which both have), the region and the anchor posse already parses (`captureFooter` + `backgroundWorkIn`), claude's own words, and measured twice a quarter apart — 2026-09-02's seven live panes and today's 36 samples. |

## What landed

**Ask 2 — the refusal says what it was looking at** (`landrefusal.go`). Both
facts that answer "working at what?" were available at the line that refused
and thrown away: the reading `MuxWaitTimeout.Last` carries, which
`ranger-base-hcig1` put there for this exact caller ("a launcher that abandons
one of them owes the operator the difference"), and the pane's own `PaneHold`.
`landThePlane` now returns what it was looking at, and each caller says it:

```
NOT killed: <name> is still working after 10m0s — let it finish and kill it
again (or --timeout 20m0s, or --no-land to close it without the turn)
  the turn was waiting for it to go idle before typing the landing prompt
  the wait ended with last reading `working`: program_status state=working
    app=claude-code msg=3 shells still running updated_at=2026-11-21T09:23:…
  its pane is holding 3 shells still running
  background work claude carries across a turn boundary is not an edit in
    flight — if that is all this seat is holding, no --timeout settles it
  a kill would take nothing uncommitted: ~/.posse/worktrees/posse/<name> is
    clean and posse/<name> holds nothing main does not — so `--no-land` skips
    only the agent's own wrap-up (its standing orders are committed either way)
```

It decides nothing: the refusal still refuses on the same evidence, every
clause is best-effort, and a refusal that could read nothing prints exactly
the sentence it printed before. The kill's last clause is the one only a kill
can add — `residueHolds` asked as a question — and it is what turns
"`--no-land` is available" into "`--no-land` loses nothing", which is the
judgment the operator made by hand with `git status` after the second refusal.

**Ask 1 — the sweep stops reading `working` as "somebody is in there"**
(`reapworking.go`). A fourth arm on the end-of-pass auto-reap, the narrowest
conjunction that makes the reading stop being an answer:

| | |
|---|---|
| class | `reapFleetClosed` only — dispatch's own per-bead session, no crew mark, carrying the `bead:` pointer |
| the store of record | that bead reads **closed** (read fresh in `reapWhy`, and read once) |
| the occupant | `working` only; `blocked` and `""` keep exactly the behaviour they had |
| the screen | the footer names live background work **and** carries no `esc to interrupt`, off ONE hold at one instant |
| the composer | holds nothing unsent |
| the records | `residueHolds` empty — a kill takes nothing |
| a grace | `reap_working_after:`, default 1h, `off` disarms |

**Why not at the reading instead.** The screen could have been used to call
the seat settled, which would route it into the existing idle/done arm in one
line. `panework.go`'s standing rule refuses that and is right: the reading is
only ever allowed to say "this settle is not one", never to invent one. A
screen that can manufacture a settled state can close a bead and judge a turn
nobody saw. In this arm nothing is judged — the bead is already closed by the
store of record, there is no turn left to judge, and the only question is
whether the workspace may go.

**What it is willing to be wrong about:** claude drawing no interrupt hint
during a turn. Then it reaps a live turn — the same consequence
`settledForReap`'s `""` arm already accepts on a thinner reading, over a bead
the store calls closed and a tree that provably holds nothing. What it cannot
be wrong about is the direction every gap fails in: an unreadable screen, a
herdr that will not explain, a footer with no vocabulary in it and a failed
capture all answer the zero hold, whose `Work` is `""`, and this arm needs
`Work` non-empty.

**And it says itself**, which is half the bead: every pass the arm declines to
act prints one line naming what it read and the command that ends it now. The
41.6h of silence is the condition that line exists for.

## The grace is a dial and not a measurement

`reap_working_after:` is 1h on `DefaultUnpointedReapAfter`'s argument, not on
a reading of the fleet (no number in this family ever was — `ranger-base-f6lk`
says so for the other two). What the grace buys here is not doubt about the
bead (the store of record has spoken) or the tree (`residueHolds` has): it is
"not racing a seat somebody is about to use", which is exactly what the
unpointed hour is priced for. The crew grace is longer because it covers a
*conversation's* gaps, and a crew-marked seat is not this population — it
reaches `reapCrewClosed` and pays four hours.

It is measured against `residueIdle` (the later of the launch and the last
work prompt), the only clock any record here carries and the one both arms
beside it use. On a dispatched bead that is the launch: tvokd was launched at
08:24 and its bead closed at 09:22, so the hour was already spent before the
close and this arm would have taken the seat on the first sweep past it.
