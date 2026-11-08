## The fence has one file in it, and the transport is already equal (ranger-base-i0yuw)

ADR 0083 D3 asked for two readings before the cutover window: that bd fails
LOUD against a read-only store, and that a restored backup archive exports
identically. Both were ASSUMED. This is the measurement, taken on COPIES
only — the store of record was never written and never copied.

**MEASURED 2026-11-08**, bd 0.50.3 (`bd25acbc`), posse 0.5.3+7d3446ef,
sqlite3 3.51.0, darwin 25.6.0 arm64, every run as the files' own uid, in a
session scratch dir outside any tracked repo.

### The headline: the fence as D3 spells it also blocks READS

D3 says "the laptop's db files and their `.beads` dir lose write
permission" and the probe expected `bd list` to keep reading. It does not.
A bd store is a **WAL** database (`pragma journal_mode` → `wal`, both on the
fixture bd built and on a live store), and a WAL reader needs to write the
`-wal`/`-shm` sidecars. So:

| the fence covers | writes (`create`/`update`/`comments add`/`close`) | `bd list` |
|---|---|---|
| `beads.db` + `-wal` + `-shm` + the `.beads` dir (D3 as written) | exit 1 · `permission denied`, names the db path | **exit 1** · `unable to open database file` |
| `beads.db` + `-wal` + `-shm`, dir writable | exit 1 · same | **exit 1** · same |
| the `.beads` dir alone, every file writable | exit 1 · `attempt to write a readonly database` | **exit 1** · same |
| **`beads.db` alone** — `-wal`, `-shm` and the dir left alone | exit 1 · `permission denied`, names the db path | **exit 0** |

Narrowed to one variable: `chmod a-w` on **`beads.db-wal` alone** (main db
read-only, `-shm` and the dir writable) is enough to turn `bd list` into
`Error: failed to search issues: sqlite3: unable to open database file`.
That is also why an earlier arm of this probe read green and then red with
no permission change between them — a `-wal` left read-only by a previous
fence survives, because SQLite does not restore a mode on a file it does not
delete. When SQLite *does* recreate the sidecars it makes them 0644 from the
umask and does **not** inherit the main db's mode (3 consecutive runs with
the sidecars deleted and the main db `a-w`: 0644 each time), so fencing the
one file is stable rather than a race.

**So the fence is one `chmod a-w` on `beads.db`, not on "the db files and
their dir".** In that arm:

- `bd create`, `bd update`, `bd comments add`, `bd close`, `bd sync` and
  `bd export` each exit **1** and print
  `Error: failed to open database: failed to enable WAL mode: sqlite3:
  unable to open database file: open <db>: permission denied` — loud, named,
  on stderr, nothing on stdout;
- `bd list` and `bd show` exit **0**;
- a raw `sqlite3 <db> "insert into …"` is refused too —
  `Error: stepping, attempt to write a readonly database (8)`, exit 8. The
  fence is the filesystem's, not an artifact of bd re-setting its WAL pragma,
  which is what would have made it a courtesy rather than a wall.

Nothing was written under any arm: the db's sha256 and mtime are unchanged
after all five write verbs, no new files appear in `.beads`, and after
`chmod u+w` the store's sorted `bd export` is byte-identical to the
pre-fence baseline. Reversible with one `chmod`, as D3 needs for rollback.

### `bd export` is a write-path verb, so the laptop's export comes first

`bd export` opens the store read-write to enable WAL mode, so it is refused
in **every** fence arm, including the one-file fence where `bd list` is
green. D3's "the host's `bd export` of each store equals the laptop's last
export" therefore has an ordering constraint worth stating in the window:
the laptop's export is captured **before** the fence, because after it the
laptop cannot produce one. (`bd export` also skips the staleness check that
`bd list` runs — see below.)

### Transport: equal, and it does not care about the fence

`posse backup --to <scratch>` from a scratch home whose `queue_repo:` names
a scratch queue repo: exit **0**, `· verified`, 5 members — `MANIFEST.json`,
`home/config.yaml`, `queue/beads.db`, `queue/issues.jsonl`,
`queue/queue.bundle`; manifest `excluded: [envs, secrets, state, personas]`.
`posse backup verify --archive <path>` on the result: exit **0**.

It ran green against the store **unfenced**, against the one-file fence, and
against D3's full fence (dir and all three files `a-w`) — exit 0 all three,
so `stop + FENCE → transport` is sound in the order D3 wrote it.

Restoring: `git clone <queue.bundle>` into a new path brings back
`.beads/issues.jsonl`, `.beads/metadata.json` and `.beads/.gitignore` from
the git history; drop the archive's staged `queue/beads.db` beside them.
Sorted `bd export` of the restored store vs. the source's:

- archive taken **unfenced** → identical, sha256 `6d8e9edb…` both sides;
- archive taken under the **full fence** → identical, same sha256;
- `sqlite3 <db> ".backup <out>"` (the private project's route) → identical,
  on two separate fixtures. It also reads a one-file-fenced db happily
  (exit 0).

So the ASSUMED row "a restored archive's export equals the source's" is now
MEASURED, on both routes, and the staged db being a different byte sequence
from the source db (page-level copy) does not matter: the comparison D3 asks
for is the export.

### What bd needs to adopt a restored db in a new path: the db, and one import

With **only** `beads.db` in a brand-new `.beads` — no `metadata.json`, no
`issues.jsonl`, no git repo, no `redirect` — `bd where`, `bd list` and
`bd export` all work and resolve the prefix out of the db's own `config`
table. bd then writes its own `.beads/metadata.json`
(`{"database":"beads.db","jsonl_export":"issues.jsonl"}`), `.local_version`
and `.migration-hint-ts`. Nothing has to be hand-placed.

The one real adoption step appears as soon as the restored `issues.jsonl`
comes along: the first `bd list` exits **1** with
`Error: Database out of sync with JSONL. Run 'bd sync --import-only' to fix.`
The cause is an **mtime**, not a content mismatch — the db's `last_import_time`
metadata row against the jsonl's mtime. Both routes bump it: tar stamps the
extracted file at archive time, and a `cp` stamps it at copy time, while the
db's `last_import_time` is frozen at the source's last `bd sync`. Proof it is
the mtime: `touch -r <source jsonl> <restored jsonl>` makes the same `bd list`
exit 0 with no flag and nothing else changed. The content was never in
question — `jsonl_content_hash` already matched.

Three ways out, measured: `bd sync --import-only` (exit 0, `Import complete:
0 created, 0 updated, 3 unchanged` — it confirms db and jsonl agree rather
than overwriting either, and the post-adoption export is still identical);
`touch -r` to restore the mtime; or `--allow-stale` for a single read. The
first is the one to put in the runbook, and it is a WRITE — so it belongs on
the host's own copy after the restore, never against a fenced store.

### Fixture note: `bd init` is denied, so the store was built another way

Every crew PID denies `Bash(bd init:*)` (ADR 0015 §3) and the argv gate
refuses it by resolved verb, so the probe could not build its fixture the way
the bead's recipe says. Instead: bd's own sqlite schema, read read-only off an
existing store with `sqlite3 .schema` (schema only — no rows, no data, nothing
copied), applied to a fresh file minus the `sqlite_sequence` line sqlite
refuses to be handed, then two `config` rows for a scratch prefix. bd adopted
it without complaint and filled it through ordinary `bd create` /
`comments add` / `dep add` / `close` / `sync`. The one visible difference from
a real `bd init` is a cosmetic `warning: beads.role not configured`. Recorded
because the next probe that needs a throwaway store will hit the same wall.
