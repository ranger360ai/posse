# Four facts this harness printed and its governance surface denied (ranger-base-wmaf9)

Bead ranger-base-wmaf9, from github.com/ranger360ai/posse/issues/3 (operator,
work-box shakedown ranger-base-x4g3h, 2026-10-08). Design: ADR 0029, amended
2026-10-09 for G12-G15.

## The shape of the defect

`posse status` ends with `nothing needs a human`, and that sentence is
`GovReport`'s answer for an EMPTY condition set — not a judgement anybody
makes, just the rendering of zero rows. So any fact that raises no condition
is a fact the surface actively denies. The four the operator found were each
already computed, already printed, and read by nothing that answers "does
anybody need to do something":

| fact | who already printed it | where that goes |
|---|---|---|
| a foreign/stale L3 `prepare-commit-msg` | `SweepHookWall` (ranger-base-ixv4), `posse gates` | `posse promote`'s epilogue, the watch preamble — one-shot, scrolled past |
| the queue jsonl commit was refused | `dispatch.go commitQueue` | one `⚠` line in a watch log |
| a persona memory landing was refused | `MemoryLanding.Line()` | one line at a `posse kill` |
| a session is running degraded | `posse ls` (`⚠️degraded`) | a listing nobody diffs |

Same class as ranger-base-y13h7 (the launcher-lag sentence printed two lines
above the all-clear, in the same view, for four days) and ranger-base-0q7rp.

## MEASURED 2026-10-09 · darwin 25.4.0 · this box

**The hook wall's two halves.** ADR 0023 asks identity (the file at the
dispatch path is byte-for-byte this build's render) and behavior (that
render, exec'd fresh, still refuses). Over the four repos this instance's
`beads_visibility:` declares:

| ask | three readings |
|---|---|
| identity ∧ behavior (`SweepHookWall`) | 3.41s · 3.54s · 3.62s |
| identity alone (`SweepHookWallIdentity`) | 375ms · 396ms · 393ms |

The behavior half execs the render once per repo and that render runs `git`
over the repo's own index, which is where the 3.5s is. `posse status` pays it
once; the cockpit recomputes its GOVERNANCE block every 30s and the pulse
every two minutes, so the pair is not a price a reading may charge on a
ticker. The drop is named at `l3AskIdentity` rather than hidden: behavior
catches a RENDERER regression — a property of this binary, identical in every
repo — and is already asked at every launch and once per watch loop. Identity
is what catches the per-repo facts (foreign, stale, uninstalled), which are
the ones a governance row is about and the ones nothing else asks on a
schedule.

**The reading itself, and the trap in reading it from a test.** Against a
binary built from THIS worktree all four declared repos came back `l3Stale` —
ours by marker, not by bytes. That is not the box's state, it is the
worktree's: this tree is ahead of `main` (ADR 0069's write-time arm landed the
day before), so its render legitimately disagrees with hooks installed from
`main`. `scripts/verify-hook-freshness.sh`, which renders from the binary on
PATH, answered `fresh — 4 repo(s) match this binary's render, stamps agree
with config` in the same hour. So G12 is SILENT on this box today, which is
the right answer — the drift between a dev build and the installed hooks is
G11's row, not this one. It is also why no pin here may read the live
`beads_visibility:`: a governance pin that did would be red per hour, the
class ranger-base-rp2y cost a day to.

**The memory sweep.** One `git status --porcelain --untracked-files=all -z`
over the whole personas dir, plus one `rev-parse --show-prefix`, against one
`MemoryDirtyPaths` call per persona (11 personas):

| shape | readings | answer |
|---|---|---|
| one sweep, bucketed by path segment | 38ms · 35ms · 35ms | `dinesh: 1 path` |
| a call per persona | 194ms · 201ms | `dinesh: 1 path` |

Same answer, 5x the wall. `TestGovG14SweepAgreesWithThePerPersonaRead` is the
pin that keeps the two readers from drifting — the kill path and the
governance row must never disagree about whether a persona's memory is
landed.

**Why "the projection is dirty" is not G13's predicate.** At 2026-10-09 the
queue repo's `.beads/issues.jsonl` was dirty (`git diff HEAD --name-only`
named it) with nothing refused at all: `bd sync` re-exports the jsonl from any
session and the launcher commits it only at a close it judges, so dirty is the
ordinary state between closes. A row keyed on it would fire almost always and
say nothing about whether anything was wrong. G13 reads the three CONFIG
causes instead — the ones that make the next close fail too and that no amount
of waiting clears.

**The `queue-unmarked` chain**, because it is the one cause with three links:
an unmarked repo is PUBLIC (`visibility.go`, fail closed) → the commit guard's
check 0, the beads-jsonl visibility scan, runs in public repos only
(`gates.go`, guarded on `posse_beads_visibility = public`) → so an unmarked
queue repo is exactly the state in which the launcher's own commit of the
store of record is scanned as a publication and refused. On this instance that
scan would hit: its one `beads_visibility_patterns:` entry is a
pre-publication name whose shape appears in thousands of this store's own bead
ids, so the refusal would be total rather than occasional. Marking it is ADR 0015 §4's
cutover step 5 — performed once by hand, recorded nowhere else, and
unreconstructable from any other surface.

## The third G13 cause, and the live run that removed it

`QueueReady` shipped with a third cause for one afternoon: a `beads:` store
that does not resolve inside `queue_repo:`. The mechanism is real —
`CommitQueueJSONL` returns `Skipped: <store> is not inside <repo>` and the
pass prints `◑ no queue commit` — so every close in such a repo leaves no
record in git. What made it wrong was the first `posse status` run against a
binary carrying it:

```
LANE  G13  the bead store <a client project>/.beads does not resolve inside
           config queue_repo: <the queue repo> — the launcher skips its
           projection at every close in that repo, so those beads reach git
           nowhere
```

(paths generalized; this repo's commit guard refuses an instance path in a
staged line, which is also how I found out the fragment had one.)

The store it named belongs to a client project with its own bd database, by
intent. So the row was a standing instruction to move another project's beads
into posse's queue, which nobody wants and which ADR 0015 §4 never asked for:
§4 moves THE STORE OF RECORD, singular, and says nothing about every other
store an instance reads. A multi-project instance is the ordinary shape. The
cause is gone, the reason is written at `QueueReady`'s site, and
`TestGovG13UnmarkedQueueRepo` asserts no such row appears for a store outside
the queue repo — so the next person to have the same idea meets the pin before
the operator meets the noise.

The same run is why the rest of the rows are trusted: G15 named one real
waiver on this box (a seat running with a project-config trust gate the wall
does not realize), and G12's four rows read `ours but stale` against a
worktree build and are silent against the installed one.

## What is deliberately NOT a row

- **A git failure at a memory landing.** `LandPersonaMemory` takes no launcher
  lock and says why: two kills contending for `index.lock` leave the loser's
  file untouched, which the NEXT kill lands, because the evidence is the file
  itself and nothing consumed it. Self-healing in the direction that matters,
  so it resolves itself off G14 rather than needing a row.
- **A one-off refusal of any kind.** A condition is a level, not an event
  (ADR 0029). Each of the four rows reads a fact that is still true while the
  remedy is outstanding and false the moment it lands.
- **A behavior-half regression as a per-repo row.** See the table above: it is
  one fact about the binary, not N facts about the box.
