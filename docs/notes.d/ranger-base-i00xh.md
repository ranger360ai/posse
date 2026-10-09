## What ADR 0071's check actually costs a pass: 0 extra bd calls on a live pass, 2.7s worst case on a quiet one (ranger-base-i00xh)

ADR 0071 makes an empty bd `--json` list a QUESTION: every list reader asks
`bd info --json` for the store's own `issue_count` before it serves `[]` as
an answer. The record's Consequences carried the price as ASSUMED — "some
multiple of ~0.5s, under five seconds on this instance" — and asked the
first code bead to measure the multiple. This is that measurement.

### Method

MEASURED 2026-10-09, `bd version 0.50.3 (bd25acbc)`, darwin 25.4.0, from a
`cage: seatbelt` crew seat, on this box with the live instance's own
configuration (two beads repos: the queue repo of record, reached through a
`.beads/redirect`, and `~/src/hcn`).

`posse dispatch --dry-run`, built from this worktree with the check in it,
with `RHQ_BD_BIN` pointed at a pass-through shim that appends one line per
call — exit status, whether stdout was `[]`/`null`/empty, byte count, argv —
and then execs the seat's own gate shim, so the PID's deny set stayed in
force for every call. The dry pass files no bead, runs no CI watch, takes no
launcher lock and launches nothing (`dispatch.go`, the `!d.DryRun` guards),
so this is a read-only diagnostic of the pass's read pattern. Two passes,
the second with `--persona jared`, which changes routing and not the queries.

### What one pass reads

Both passes: **14 bd calls**, byte-identical in shape.

| calls | verb | reader |
|---|---|---|
| 6 | `show <id> --json` | the worktree sweep, one per live worktree |
| 2 | `list --all --json --limit 0` | `ListAll` — the lost-bead sweep, once per repo |
| 2 | `ready --json --limit 0` | `Ready`, once per repo |
| 2 | `blocked --json` | `Blocked`, `Ready`'s cross-check (NOT one of the five) |
| 2 | `list --status in_progress --json --limit 0` | `InProgress`, once per repo |

**Empty list reads: 0. `info` calls fired: 0. Added tax: 0s.** The three
calls whose stdout was empty were `~/src/hcn`'s, all exit 1 (`sqlite3:
unable to open database file` — that store is not openable from this seat's
cage). A failing read returns on `run`'s error path and never reaches the
check, which is the point: an error was already loud.

So on a pass that finds work, the check costs nothing at all, and the reason
is structural rather than lucky: `bd ready --json` is read WHOLE and routed
in Go, so the per-persona filtering that would empty it never reaches bd.

### The worst case, which is the number the ADR wanted

Three of the four list-class reads per repo go through the five readers.
`list --all` over a live store is never empty — a store whose `--all` is
empty while its census holds rows IS the defect, so that read costing an
`info` is the check working. That leaves **two reads per beads repo that can
honestly come back empty on a quiet shop**: `ready` (nothing unblocked) and
`list --status in_progress` (nobody holding anything). On this instance's two
repos that is **4 `info` calls**, and `bd info --json` against the live store
of record costs 0.49s / 0.63s / 0.54s over three runs (same measurement the
ADR's Context took, re-taken here): **≈2.0-2.7s on the quietest pass this
instance can have**, against a `--watch` cadence in minutes.

Under the ADR's five-second line, so the memoization alternative
(ADR 0071, "Memoize healthy per store for the process lifetime") stays
rejected and is NOT filed. It would buy at most those 2.7s and would go
blind on a store that breaks mid-run until a restart nobody schedules.
