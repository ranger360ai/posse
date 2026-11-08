# bd init builds the lossy store by default, and what tells the two classes apart from outside (ranger-base-3opof)

INSTALL.md §9's store-creation step, 2026-11-07, plus the measurements the
second half of the bead turned out to need. The bead asked for two things:
`--backend sqlite` in the recipe (landed here) and a `posse status` finding
over a no-db store (refused by ADR 0055 D3; handed to architecture as
ranger-base-p1l2h).

## What `bd init` does on a CGO-free bd

MEASURED 2026-11-07 by the operator during the ADR 0080 bring-up, bd 0.50.3
(`bd25acbc`), on linux and on darwin, where both the pinned release tarball
and the local build are `CGO_ENABLED=0`:

| command | what it builds |
|---|---|
| `bd init` | prints `Note: dolt backend requires CGO (not available in this build).` / `Falling back to JSONL-only mode.`, exit 0, **no `beads.db`** |
| `bd init --backend sqlite --prefix <p>` | the classic store, `metadata database: beads.db`; a create and its read-back land |

bd 0.50.3's default backend is Dolt; Dolt needs CGO; SQLite does not, so the
one backend the binary can actually run is the one you have to ask for. The
fallback is a note on stdout and an exit 0, which is why nothing in a script
notices. The store it leaves is the mode ADR 0073 measured losing 34-36 of
48 concurrent creates while printing `Created issue: <id>` for every one of
them.

INFERRED, not measured here: that the fallback store carries `no-db: true`
in its `config.yaml`. Two reasons, neither a run of `bd init` — the verb is
in every persona PID's deny set (ADR 0015 §3). bd's own binary carries the
config template string `no-db: true  # JSONL-only mode, no SQLite database`
(`strings` over the pinned binary), and a store with neither a database nor
that line is not usable at all, which the next section measures. A store
that merely lacked the line would have failed the operator's read-back.

## Telling the classes apart from outside, without reading bd's config

MEASURED 2026-11-07, bd 0.50.3 `bd25acbc`, darwin 25.6.0, crew seat, every
call `env -u BEADS_DIR bd --no-daemon info --json` with the cwd in the rig.
Two rigs hand-written per ADR 0072 (no `bd init`): a `.beads` holding
`config.yaml` and a one-row `issues.jsonl`, with and without `no-db: true`.
The third row is the live store of record.

| store | `database_path` | `mode` | exit |
|---|---|---|---|
| `no-db: true`, one row | **the repo dir** (`…/rigs/nodb`) | `direct` | 0 |
| `issue-prefix` only, no database | — (`Error: no beads database found`, and the hint "Add 'no-db: true' to .beads/config.yaml") | — | **1** |
| the live SQLite store of record | `…/.beads/beads.db` | `direct` | 0 |

Three things fall out, and they are what ranger-base-p1l2h is built on:

- **`mode` does not discriminate.** Both answering stores say `direct`. A
  reader that keyed on it would call the lossy store healthy.
- **`database_path` does.** On the no-db store it is a DIRECTORY — the
  repo, not a file in `.beads` — so "does `database_path` name an existing
  file" is a one-stat predicate over bd's own answer, which is not a copy
  of bd's config reader (D3's specific objection).
- **An absent database is not the same state as no-db mode.** A store with
  no database and no `no-db: true` refuses every read, loudly, exit 1 — it
  does not fall back. So `beads.db` missing is two different facts, and only
  bd can say which: that is the cheapest-first order the handoff proposes
  (stat first, ask bd only when the file is absent).

Row three also says the healthy case costs nothing to check: the live store
has its database, so a stat answers and no bd runs.

## What is NOT here

No detector landed. ADR 0055 D3 — "No store-class detector, no refusal, no
doctor" — refuses it by name, and its Alternatives-rejected entry rejects
the warn-on-no-db reader specifically. D3 has been amended once before, by
ADR 0074, to admit one store fact (`sqliteStoreFile`) at one site, so the
route exists; it is architecture's to open. INSTALL.md §9 says plainly that
nothing downstream will tell you, and cites D3 for why.
