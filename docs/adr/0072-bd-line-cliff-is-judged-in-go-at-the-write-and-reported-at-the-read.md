# ADR 0072 — bd's 64 KB line cliff is judged once, in Go, by bd's own encoder: the shim refuses the crossing write, every failed read names the line, and status warns at 48 KB

*Status: accepted 2026-10-12 (ranger-base-zufew, from
github.com/ranger360ai/posse/issues/14, operator, work box, 2026-10-11) ·
owner: architect · a sibling of ADR 0069 (the bd shim's write-time arm) and
ADR 0071 (the `BdStoreUnreadError` family); reports-never-repairs per ADR
0063/0068/0071 · builds in the code beads named on ranger-base-zufew
(dinesh)*

> A bd store is a JSONL file read through a 64 KB `bufio.Scanner`, and bd
> 0.50.3 can neither delete nor edit a comment. So the write that pushes an
> issue's line over 65,536 bytes is the last point at which anyone holds a
> remedy bd can perform. Posse refuses that write, names the line when the
> store has already gone over, and warns well before the cliff.

## Context (MEASURED 2026-10-12, bd 0.50.3 `bd25acbc`, darwin 25.4.0, crew seat, every call `--no-daemon`, hand-written rigs per ADR 0071 — no `bd init`; the probe is `0072-bd-line-cliff.probe.sh`)

- **The limit is 65,536 bytes per line, newline included.** A no-db rig whose
  row 2 is 65,535 / 65,536 / 65,537 bytes long: `bd list --json` exits 0, 0,
  then **1**: `Error initializing --no-db mode: failed to load issues:
  bufio.Scanner: token too long`.
- **In no-db mode one line makes the whole store unreadable.** That
  includes `show` of a *different* id and `update -d short` on the
  oversized row itself, which is the only shrink bd has. A cage seat's bd is
  always no-db (ADR 0055, `CageBdFlags`), so every container seat goes dark.
- **The crossing write succeeds.** With row 2 at 60,244 bytes in no-db mode,
  `bd comments add t-2 -f body.txt` (a 6,000-byte file) exits 0 and writes a
  66,318-byte line. Every call after that fails.
- **In SQLite mode the failure is silent.** If the census holds an oversized
  line when the DB is first built, the build succeeds (70,244 bytes, 3
  rows). After the census changes (a new row is appended), `bd sync
  --import-only` creates the row at 60 KB but exits 1 at 70 KB (`error
  reading JSONL: bufio.Scanner: token too long`). That is the verb posse's
  own stale-db self-heal fires (`Bd.run`). The staleness check behaves
  differently on each side of the limit. With the census touched minutes
  after the build, under 64 KB `list --json` refused with "Database out of
  sync with JSONL" (exit 1). Over 64 KB it exited 0 with stderr **empty**,
  serving the old 3 rows. This was one manual run per rig. In the probe,
  where the touch comes 2 s after the build, neither rig refuses, so the
  refusal's timing is bd's and was not traced further. Either way the reader
  sees stale rows, exit 0.
- **bd cannot repair it.** `bd comments` has a single subcommand, `add`. Once
  a line is over the limit, the only route back is a hand edit of the jsonl,
  or in SQLite mode `update -d` when the description is what made it large.
- **bd's encoder is Go `encoding/json` with HTML escaping turned off.** A
  6,000-byte comment (1,000 each of `<`, `\n`, `"`, `é` (two bytes), `\t`)
  grew the line by 9,072 bytes. That is 9,000 of escaped text plus about 72
  bytes of comment envelope, and `<` is written raw, not as `<`. A `sh`
  byte count would be off by up to 6× on control characters, would miss Go's
  `�` replacement of invalid UTF-8, and would miss its ` ` escape.
- **This store, today:** 2,963 rows. The largest line is 38,498 bytes
  (ranger-base-3rv9), and 27 lines are over 24 KB. In the six largest rows,
  persona-written comments make up nearly all the bytes. posse's own
  `Bd.Comment` bodies are bounded sentences (ciwatch, settle, prompt-stall,
  verify). The work box was over 64 KB twice, with four more lines between
  41 and 54 KB (issue #14, REPORTED).
- **Costs.** A Go `ReadSlice` scan of the whole 22.9 MB census for every
  line length takes 2.3 / 2.4 ms. `grep -c '^{"id":"<id>",'` over the same
  file takes 80 ms. `posse version` takes 10 ms warm. All 2,963 lines begin
  `{"id":"<id>",`.

## Decision

**D1 — one judge, in Go, written in bd's own encoding.** A new `bdline`
source file beside `beads.go` in the posse package (B1 writes it) holds:
- `BdLineLimit = 65536` (MEASURED).
- `BdLineHeadroom = 2048` (ASSUMED: it covers unflushed DB writes the census
  does not show yet, and bd reordering the fields when it rewrites).
- `BdLineWarn = 49152`.
- `CensusLongLines(path, over)`: line number, id and bytes for every line
  longer than `over`. It reads with `ReadSlice` and has no size limit.
- `ProjectBdWrite(census, argv)`: the line length bd will write. It takes
  the target's current census line, applies the write, and re-encodes with
  `json.Encoder` and `SetEscapeHTML(false)`. A comment is appended as bd's
  comment object, using a worst-case timestamp. A field write
  (`-d/--description`, `--design`, `--notes`, `--acceptance`, `--title`,
  `close -r/--reason`) replaces that field. `create` builds a new row. Text
  comes from argv, or from a regular file named by
  `-f/--file/--body-file`, under the same rules as ADR 0069 D2: never `-`,
  `/dev/*` or `/proc/*`.

A write is refused when the projection exceeds `BdLineLimit −
BdLineHeadroom`. There is one judge, and both callers below use it.

**D2 — write time, persona: a hidden `posse gates bd-line -- <bd argv>`,
called by a new bd-shim arm.** `renderShim` renders the arm for `cmd ==
"bd"` on **every** instance, after the ADR 0069 arm and before the `exec`.
The ceiling arm renders conditionally; this one cannot, because every store
has this limit.
- The arm skips bd's global options, using the shim's existing loop. It
  calls posse only when the verb is `comments`, `update`, `create` or
  `close`, so reads pay nothing.
- It calls the absolute posse path recorded at render time. If that path is
  missing, it prints one stderr line and lets the call through (fail-open).
  A wall that blocks every bd write because posse moved is worse than the
  gap, and G16 still catches what gets past.
- The subcommand finds the store the way bd will: `$BEADS_DIR`, otherwise
  `beadsHome(cwd)`. It does not judge, and exits 0, when the census is
  absent, the id has no exact `{"id":"<id>",` line (a partial id), or the
  text arrives on stdin.
- **Refusal** (exit 1; bd is not run):

  `refused by posse gate: this write would make <id>'s line in <census>
  <N> bytes; bd 0.50.3 cannot read a line over 65536, and it cannot delete
  a comment, so the store would stay unreadable (ADR 0072)`

  It is followed by the remedy: *put the bulk in a committed file or a new
  bead (`bd create … --deps discovered-from:<id>`) and comment the
  pointer.* Unlike ADR 0069 D4, quoting argv here is harmless, but the
  refusal does not quote it anyway: a 60 KB echo would help no one.
  refusals.log gets `<stamp> bd line cliff [bd shim] <id> <N>B session
  <persona>`. There is no override: the writer is the one typing, and the
  remedy can always be carried out.

**D3 — write time, posse itself:** `Bd.Comment`, `SetDescription`,
`Create` and `Close` ask `ProjectBdWrite` in-process before running bd, and
return `BdLineWouldCrossError` instead of writing. These callers already
treat a failed write as best-effort. posse's PATH has no shim, so without
this, posse's own short comment could be the one that pushes a line over.

**D4 — read time: a failed bd call is checked against the census, not
against bd's wording.** When `Bd.run`, `runOnce` or the `sync --import-only`
self-heal exits non-zero, posse calls `CensusLongLines(census,
BdLineLimit)` (about 2 ms). If any line is over the limit, it returns
`BdStoreLineTooLongError{Store, Lines}`, which wraps bd's error and works
with `errors.As`, as `BdStoreUnreadError` does. Its text:

`bd store unreadable: <census> line <n> (<id>) is <N> bytes > 65536 —
bd 0.50.3 reads the jsonl through a 64 KB scanner (ADR 0072); posse does
not repair it — the operator moves the bulk out of that line by hand`

posse does not match bd's `token too long` string, so ADR 0056's single
prose tripwire stays single. Callers are unchanged (ADR 0071 D5): dispatch
already reports "the queue is unknown, not empty".

**D5 — status: G16, from the census, in both store modes.** For each
distinct `beadsHome` over `BeadsDirs()`, `ShopCheck` runs
`CensusLongLines(census, BdLineWarn)`:
- A line over `BdLineLimit` is **GovUrgent**, key `bd-line-cliff:<id>`:
  "<id> is <N> B in <census>: over bd's 65536 line limit — no-db and cage
  reads fail, imports stop silently".
- A line between `BdLineWarn` and `BdLineLimit` is **GovLane**, key
  `bd-line-warn:<id>`: "<id> is <N> B of 65536 — move bulk out before the
  next comment".

G16 is the only signal for the SQLite-mode failure, because that failure
reaches no error path (Context, fourth bullet).

## Consequences

- On every instance, write verbs at a crew seat cost about 15 ms more (a
  10 ms exec plus a 2 ms scan; ASSUMED until B2 measures it). Reads cost
  nothing. A status pass costs about 2 ms per store.
- A bead past about 63.5 KB takes no further comments from the shim, and
  only small fields. That is intended: G16 has been flagging it since 48 KB.
- **Residuals, by name:** text sent on stdin (`-f -`, `/dev/stdin`: 8.6 % of
  writes, ADR 0069 census), `bd edit`, partial ids, the operator's unshimmed
  shell, hand edits, and DB writes not yet flushed to the census, beyond the
  headroom. G16 reports all of them before the cliff. None justifies a
  third mechanism until a G16 row shows one getting through.

## Alternatives rejected

- **A `sh` byte count inside the shim, matching ADR 0069's shape.** It costs
  no posse exec. But D3 already needs the judge in Go, so this would be a
  second implementation of one rule. And `sh` cannot reproduce bd's encoder:
  it is off by up to 6× on control characters, and blind to `�` and
  ` `. Finding the id with `grep` (80 ms) is also slower than the whole
  Go exec plus scan. ADR 0069 rejected a subcommand because it added a second
  regex dialect; here the shell would be the second dialect, so that
  reasoning points the other way.
- **Measure through bd (`bd show <id> --json`, re-marshal).** About 0.3 s per
  write, a second resolver, and it measures the database rather than the
  census line the scanner actually reads.
- **Cap each write instead (refuse any payload over X KB).** No id parsing,
  but issue #14's two crossings came from many small comments piling up, so
  a per-write cap misses the case that actually happened.
- **The clever one: posse rewrites the oversized line itself**, moving the
  largest comments' text to `docs/notes.d/` and leaving a pointer. That makes
  posse a second writer to bd's store, which ADR 0071 D4 rejects, and it
  would choose, unattended, which comment loses its text.
- **Match `token too long` in stderr.** Pinned-binary prose (ADR 0056), and
  the SQLite-mode failure prints nothing anyway. The census tells the same
  story without depending on bd's wording.
- **Make the limits configurable.** The limit belongs to the pinned binary,
  not to the instance. Add a key the day the pin lifts.
- **Lift the pin / patch bd upstream.** The 2026-10-08 standing ruling rules
  this out (ADR 0071).

## Verification (laurie's checklist)

1. `sh docs/adr/0072-bd-line-cliff.probe.sh <scratch>` reproduces the
   boundary (65,536 passes, 65,537 fails), the crossing write exiting 0 and
   then every verb failing, `sync --import-only` failing at 70 KB but not at
   60 KB, and the 9,072-byte growth from escaping. Re-run 2026-10-12: all as
   stated.
2. On a no-db rig at 60 KB, a shimmed `bd comments add t-2 -f 6k.txt` exits
   1 with the D2 refusal and the line is unchanged. The same call with a
   1 KB file passes.
3. `bd comments add <id> -f -` with 6 KB on stdin passes (a residual).
4. `Bd.Comment` from Go on the same rig returns `BdLineWouldCrossError`.
5. `Bd.Ready` over the 70 KB no-db rig returns `BdStoreLineTooLongError`
   naming line 2, `t-2` and 70,244.
6. `posse status` over a rig at 50 KB shows a G16 LANE row; at 70 KB it
   shows URGENT.
7. 20 × `bd list --limit 1` through the rendered shim takes no longer than
   before the arm (reads do not exec posse).
