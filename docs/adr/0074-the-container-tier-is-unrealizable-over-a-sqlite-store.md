# ADR 0074 — The container tier is unrealizable over a SQLite store: a caged `--no-db` write is never imported and the next host flush erases it, on every write, with no race

*Status: accepted 2026-10-12 (ranger-base-4jlcn, from ADR 0073's mixed-mode
arm) · owner: architect · amends ADR 0055 D3 at one site (a single store
fact read at container launch, and nothing else); corrects ADR 0073
Consequences (read-back does not see this loss) and the 2026-08-22 note in
`cageinner.go`/`cage.go` ("the host's daemon imports it") · builds in the
code bead named on ranger-base-4jlcn (dinesh)*

> ADR 0073 filed this as a race. It is not one. A SQLite-mode bd that runs
> `--no-daemon`, which is how posse pins every call, never takes in a row
> another process wrote into the jsonl. Its next export rewrites the jsonl
> from the database, and the row is gone. One caged write, with nothing else
> running, is lost every time. The importer the cage relied on was bd's
> daemon, and bd 0.50 deleted it (ADR 0056). So posse treats "container tier
> over a store that holds a SQLite database" as a gate it cannot realize.
> That goes through parity like any other such gate: dispatch refuses, and a
> crew launch needs `--allow-degraded`.

## Context (MEASURED 2026-10-12, bd 0.50.3 `bd25acbc`, darwin 25.4.0, crew seat, every call `--no-daemon`; hand-written rig per ADR 0072 with `.beads` committed; probe `0074-cage-nodb-into-sqlite.probe.sh`, 6 runs, identical every time)

| step, one writer, no concurrency | result |
|---|---|
| A. `--no-db create n1` (the cage seat's argv, `CageBdFlags`) | exit 0, row in the jsonl |
| B. host SQLite `list` 2 s later | **does not see n1**, and the jsonl still holds it |
| C. host SQLite `create s1` | does not see n1; the jsonl is not yet rewritten |
| D. host `sync --flush-only` | **n1 gone from the jsonl**, s1 present |
| E. `--no-db create n2`, then host `sync --import-only` | SQLite sees n2 |
| F. host create plus flush after that import | n2 and s2 both survive |

- **The loss is deterministic, not a race.** ADR 0073's mixed arm lost
  24 of 24 and read as a concurrency loss. Step B shows that the SQLite side
  never imports a foreign jsonl write on its own, whatever the timing. So
  every caged write into a SQLite store lasts only until the next host
  flush. bd's pre-commit hook flushes (rangerhq-be7k), so on a working box
  that is the next commit in any tree. That the hook's flush is
  byte-for-byte the D export is ASSUMED.
- **A read-back cannot see it.** The caged seat reads the jsonl, which holds
  the row until the flush. So ADR 0073's D3/D4 read-back passes, and the
  erasure comes later. For this class, ADR 0073's "detects rather than
  prevents" is wrong: nothing detects it.
- **Why it once worked.** The 2026-08-22 measurement (bd 0.49.1) had the
  host *daemon* import caged writes on its next read. bd 0.50 deleted the
  daemon, and `--no-daemon` is now a pinned tripwire (ADR 0056). That
  upgrade removed the importer, and nothing failed when it did.
- **Live exposure today: none** (MEASURED 2026-10-12, this box). Of the
  11 persona PIDs under `~/.config/posse/agents/`, 7 set `cage: seatbelt`
  and 4 set no cage, which defaults to `shims` (`DefaultCage`). None sets
  `cage: container`, and none declares `egress:`, which would imply it
  (`agents.go`). Every
  seed in `examples/agents/` is `cage: seatbelt`. `docker info` fails, so
  the tier would degrade here anyway. The store of record
  (`ranger-queue/.beads`) is SQLite: `beads.db` is present and `no-db` is
  commented out. Its seats are seatbelt, and their `bd` is the gate shim,
  which carries no `--no-db`. The class needs a container seat **and** a
  SQLite store. A store `bd init` writes today is `no-db: true`
  (dispatch.go), so the class reaches only older stores like this one. It is
  latent, and it loses 100% of writes the first time anyone enables the
  tier.

## Decision

**D1. Over a SQLite store, the container tier is an unrealizable gate.**
When the launch resolves `cage: container`, CheckParity reads one fact
about `beadsHome(dir)`: does the store hold a SQLite database file? That
means `metadata.json`'s `"database"` name if the file exists, else
`beads.db`. If it does, a Degraded row names the cause:

*"container tier: <store> holds a SQLite database (<file>); a caged bd is
`--no-db`, and its writes are erased by the host's next flush (ADR 0074).
Run this persona at `cage: seatbelt`, or the operator moves the store to
no-db."*

After that, the existing machinery decides: dispatch refuses, and a crew
launch needs `--allow-degraded`, which marks the session degraded and
launches on the host (ADR 0002 §4–5). No new refusal path.

**D2. The amendment to ADR 0055 D3 is this one read.** D3 refused a
mode detector because its fix did not depend on the mode, so a detector
added a stale copy of bd's reader and bought nothing. Here the failure does
depend on the mode, and the cost of not knowing is every write. The read
is a file's presence, not bd's config logic. It is made at one site (the
container parity row) and judges nothing else. A stale `beads.db` left
inside a no-db store gets a false refusal, and the remedy is to delete the
file. That loses nothing, and it is the operator's call.

**D3. The record is corrected.** `cage.go`'s "the host's daemon imports
it" and `cageinner.go`'s HISTORY note get a dated line saying the importer
is gone since bd 0.50 and pointing to this ADR. ADR 0073's Consequences
bullet is amended here, in this commit.

## Consequences

- The tier now fails visibly instead of silently. No live seat is affected
  today (MEASURED above).
- A container persona on this instance needs either the queue moved to
  no-db or a seatbelt seat. Moving the queue is a store migration and an
  operator decision, so it is not taken here. It would also put every seat
  into ADR 0073's no-db lost-update class, where D1's flock is the remedy.
- Exit hatch: delete the parity row. The day bd imports foreign jsonl
  writes in SQLite mode, or the cage bd can run SQLite over the mount, the
  probe's step B reads `sees n1=1` and the row goes. The probe is the test
  for that.
- Unmeasured: whether a caged no-db write crosses a Docker Desktop bind
  mount as an ordinary file write. That was measured on 0.49.1 and ASSUMED
  unchanged. The container tier cannot run on this box.

## Alternatives rejected

- **Import before every host flush** (the bead's third option). The probe's
  E/F show it works when it runs. But the flushes are bd's pre-commit hook,
  every `bd sync`, and anything the operator types, and posse owns none of
  those. A caged write that lands between the import and the export is
  still erased. The tier would then "mostly work", which is worse than
  refusing, because a read-back cannot see what it loses.
- **Give the cage bd the store's own mode.** That means SQLite across a
  Docker Desktop bind mount. It was measured failing on 2026-08-22 (ADR
  0055, Alternatives), and SQLite's locking over a shared or virtualized
  filesystem is the standard way to corrupt a database.
- **Proxy caged writes to a host-side writer.** That is a daemon, with a
  process, its liveness and a protocol. ADR 0011 and ADR 0073 rejected the
  same shape.
- **A session-private store for the cage, merged into the store of record
  at settle.** This is the clever one, and the one I wanted: no
  shared-file contention at all. But posse would own a merge of bd's
  records: no-db sequential ids collide with the host's, the cage reads a
  stale queue, and comments and closes on shared beads conflict. That is
  bd's job, done badly.
- **Document it and refuse nothing.** Exposure is zero today. But the first
  instance to turn the tier on loses every write while each exits 0 and
  reads back fine. That is issue #13's class with no remedy at all. A
  parity row is cheap, and the instance that hits it can act on it.
- **Detect "the store is SQLite" by asking `bd info --json`.** Tried: in
  no-db mode `database_path` is the directory, and in SQLite mode it is
  the file, so it is a path-suffix heuristic. On a fresh SQLite rig, the
  first call did not even print JSON. A file's presence is the plainer
  reading of the same fact.
