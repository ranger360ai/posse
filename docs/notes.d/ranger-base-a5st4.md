## bd builds the FILE, not the GRAPH — and the one repair is denied to every persona (ranger-base-a5st4)

From github.com/ranger360ai/posse issue #5 (operator, work-box shakedown
ranger-base-x4g3h, 2026-10-08): with `.beads/issues.jsonl` tracked and no
`beads.db`, a read verb opens the database read-only, notices `0 in database,
N in git`, attempts the auto-import through that handle and fails `set config:
sqlite3: attempt to write a readonly database`. The operator's workaround was
`bd import -i .beads/issues.jsonl`, a write verb, by hand.

Two things came out of re-measuring it. The reported determinant is **not the
one**, and our own record — "a plain `bd list` builds the db", carried in ADR
0055's Context and in `worktree.go`'s WHAT WAS MEASURED block as *bd 0.50.3
builds a SQLite `beads.db` over a bare jsonl on the first plain read* — is
wrong in a second way that nobody had noticed and that is worse than the
reported one.

**Operator ruling 2026-10-08 on this bead**: nothing goes upstream. We are
pinned at bd 0.50.3 on purpose (the last 0.50.x; 0.51+ is Dolt-native and
removes the JSONL/SQLite layers — ranger-base-6ulbg, `etc/bd/version-pin.toml`),
so a fix there would never reach this pin. posse designs around bd's
behaviour.

### Method

MEASURED 2026-10-09, `bd version 0.50.3 (bd25acbc)`, darwin 25.4.0, from a
`cage: seatbelt` crew seat. Throwaway `git init` repos under the session
scratchpad, one `.beads/issues.jsonl` of two synthetic rows per repo, no `bd
init` (the verb is denied to every persona PID). Every call
`env -u BEADS_DIR bd --no-daemon list` with the cwd **inside** the fixture
repo, which is the only safe way to read another store on this binary
(AGENTS.md, "Reading ANOTHER repo's queue"). Row counts after the fact read
with `sqlite3 <db> 'select count(*) from issues;'`.

| `.beads/issues.jsonl` | `beads.db` before | what the read verb printed | rows in the db after | exit |
|---|---|---|---|---|
| 2 rows, **committed** | absent | `Found 0 issues in database but 2 in git. Importing...` · `Successfully imported 2 issues from git.` · both rows | 2 | 0 |
| 2 rows, committed, plus `config.yaml` | absent | the same | 2 | 0 |
| 3 rows on disk, **2 of them in HEAD** | absent | `imported 2`, two rows | 2 | 0 |
| 2 rows, **gitignored** | absent | **nothing at all** | **0** | 0 |
| 2 rows, **uncommitted, not ignored** | absent | **nothing at all** | **0** | 0 |
| 2 rows, committed | present, **0 issues** | `Importing...` · `Warning: auto-import failed: failed to set issue_prefix from imported issues: set config: sqlite3: attempt to write a readonly database` · `Try manually: git show HEAD:.beads/issues.jsonl \| bd import -i /dev/stdin` | **0** | 0 |
| 3 rows, committed | present, **2 issues** | **nothing**; the two stale rows | 2 | 0 |
| 2 rows, committed | present, **zero-byte file** | `Error: failed to search issues: sqlite3: SQL logic error: no such table: issues` | — | 1 |

### What the table says

1. **"Tracked jsonl and no `beads.db`" is the case that WORKS.** bd creates
   the database on that call, so the handle it imports through is read-write
   and the import succeeds. The reported state cannot have been that: the
   failure needs an **already-existing, schema-valid `beads.db` holding zero
   issues**. A read verb opens an existing database read-only — correctly, it
   is a read — and the auto-import writes through it, so the import dies at
   the first config write (`issue_prefix`). The error text is issue #5's
   verbatim, so this is the same defect, read one fixture over.
   A zero-row `beads.db` is easy to have without knowing: the fourth and
   fifth rows above leave exactly one.

2. **The auto-import source is `git HEAD`, never the working-tree file.** Row
   three is the witness: three rows on disk, two in HEAD, `imported 2`. So the
   rebuild silently drops any uncommitted edit to `.beads/issues.jsonl` —
   which is the state a path-limited commit leaves routinely (AGENTS.md,
   "`MM .beads/issues.jsonl` after a clean commit is not work").

3. **Over an uncommitted jsonl the database is built EMPTY and bd says
   nothing.** `bd list` prints no rows, no import line and no warning, and
   exits 0. Ignored or merely uncommitted makes no difference — the jsonl is
   not in HEAD, so as far as the import is concerned there are `0 in git`, and
   nothing is out of the ordinary to report. **This is the half of our record
   that was wrong all along**, and it has nothing to do with the read-only
   handle: `bd list` builds the FILE. It does not build the GRAPH.

4. **The import fires only at zero rows.** A database with two rows under a
   HEAD with three prints nothing and lists two. Staleness past zero is not
   detected at all, so none of this is a mechanism to rely on for freshness —
   it is a one-shot bootstrap.

### Nothing in it can be detected from `$?`, and under `--json` nothing at all

Measured on the failing fixture (zero-row db, committed jsonl):

| call | stdout | stderr | exit |
|---|---|---|---|
| `bd list` | — | the `Importing...` + `Warning:` pair | **0** |
| `bd ready` | `✨ No open issues` | the same pair | **0** |
| `bd show scr-1` | — | `Error fetching scr-1: no issue found matching "scr-1"` | 1 |
| `bd list --json` | `[]` | **empty** | **0** |
| `bd ready --json` | `[]` | **empty** | **0** |

The `--json` row is the one that matters to posse. The human form at least
puts a warning on stderr; the JSON form suppresses it on **both** streams, so
a programmatic reader of that store — anything going through `Bd.runOnce`, the
cockpit's queue view, `posse dispatch` — is handed an empty graph, exit 0, and
no signal whatsoever. "The queue is empty" and "the queue could not be read"
are the same bytes.

**The posse-side answer is ADR 0071**: every reader of a bd issue LIST goes
through one wrapper, and an empty parsed list is now a QUESTION rather than an
answer — the wrapper asks bd its own `issue_count` (`bd info --json`) and, if
that is zero while this census carries a record, returns `BdStoreUnreadError`
naming the store and the operator's command below. So the `--json` row above no
longer reaches a caller as `[]`; it reaches it as "the queue is unknown, not
empty". Nothing is repaired by posse, for the reason the next section gives.
The live pin over this very rig is `internal/posse/bdemptyunreadlive_test.go`
(ranger-base-dkrwi); the offline arms are `bdemptyunread_qa_test.go`.

### The repair is the operator's, and that is not a style preference

No persona-runnable verb gets out of it. Over the failing state a write verb
does not trigger a writable import either — `bd create` exits 1 with
`database not initialized: issue_prefix config is missing (run 'bd init
--prefix <prefix>' first)` — and both verbs that would fix it,
`Bash(bd init:*)` and `Bash(bd import:*)`, are in the shipped deny set of
every PID (ADR 0015 §3, `scripts/verify-pid-deny-set.sh`). So bd's own
`Try manually:` line steers a persona at a verb the gate refuses.

The operator's one command, from the repo that owns the store:

```
bd import -i .beads/issues.jsonl
```

Deleting the zero-row `beads.db` and letting the next read rebuild it works
too (row one), and is the worse of the two: it rebuilds from HEAD, so any
uncommitted row in the jsonl is dropped by finding (2). Import first.

A persona who meets this says so on the bead and stops — it is `BLOCKED`, not
an empty queue.

### What this corrects

- `docs/adr/0055-store-of-record-rides-the-session-env.md`, Context and
  Verification: the sentence stands as a **door** argument — a store that
  merely lacks a database is still not a no-db door, because bd is in database
  mode either way — and is corrected on what "builds" delivers.
- `internal/posse/worktree.go`, WHAT WAS MEASURED: the same sentence, same
  correction.
- `docs/notes.d/ranger-base-vczf.md`'s "fresh clone" paragraph: true as
  written (tracked jsonl, no database) and now says which fixture it is.
- `INSTALL.md` §14 gains the row, beside the existing `bd list` → `no beads
  database found` row.

Nothing here argues for lifting the pin, for the reason `version-pin.toml`
already gives about the no-db defer defect: there is no bd that fixes this and
still has the layers, so fixing it and migrating off it are the same move.
