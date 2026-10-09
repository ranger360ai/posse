# ADR 0071 — An empty `--json` answer is not an empty queue until bd's own count says so: the wrapper asks `bd info` and the census, and refuses the read instead of serving `[]`

*Status: accepted 2026-10-09 (ranger-base-3xmt6, from ranger-base-cwsjw's
verify of ranger-base-a5st4; the state is github.com/ranger360ai/posse/issues/5,
operator, work-box shakedown) · owner: architect · sits under ADR 0055 (D3
refused a MODE detector; this record is the count check D3 did not rule on,
and 0055's Status line now says so), ADR 0063 and ADR 0068 (report where the
reader cannot repair) and rangerhq-llse (a repo the scan could not read has an
unknown queue, not an empty one) · builds in the code beads named on
ranger-base-3xmt6 (dinesh)*

> On the pinned bd 0.50.3 a store whose database holds ZERO issues over a
> `.beads/issues.jsonl` that holds N answers every `--json` read verb with `[]`,
> exit 0, nothing on either stream. Posse's readers all go through one wrapper,
> and that wrapper now treats an empty list as a QUESTION: it asks bd its own
> `issue_count`, and if that is zero while the census file carries a record,
> the read returns an error naming the operator's one repair. Every caller
> already carries that error honestly. Nothing is repaired by posse.

## Context (MEASURED 2026-10-09, bd 0.50.3 `bd25acbc`, darwin 25.4.0, from a seatbelt seat, every call `--no-daemon`, unless marked)

- **The state and its bytes.** holden's five-step rig, re-run here in a
  scratch repo: an uncommitted two-row jsonl read once (builds `beads.db`
  with 0 rows, prints nothing, exit 0), then committed, then read again. The
  human `bd list` prints `Found 0 issues in database but 2 in git.
  Importing...` and `Warning: auto-import failed: ... attempt to write a
  readonly database` on stderr, exit 0. `bd list --json`, `bd list --all
  --json --limit 0`, `bd ready --json --limit 0` and `bd blocked --json` each
  print `[]`, exit 0, stderr EMPTY. `docs/notes.d/ranger-base-a5st4.md` has the
  eight-fixture table that reaches this state and the two ways in: a read over
  an uncommitted jsonl, or a read-only import over an existing zero-row file.
- **bd carries the discriminator itself.** `bd info --json` over the same
  store answers `"issue_count": 0`, `"mode": "direct"`, exit 0. Over the live
  store of record it answers 2948. Over a `no-db: true` store (scratch, one
  committed row) it answers `"issue_count": 1` with `database_path` naming the
  DIRECTORY — in no-db mode bd counts the jsonl itself, so bd's count and the
  file can never disagree there and this record's check cannot fire on that
  class. Cost on the live store, three runs: 0.49s, 0.54s, 0.68s, against
  0.31-0.32s for `ready --json --limit 0`. (`info` was already on the persona
  argv gate's allow-list: this measurement was typed at a crew seat.)
- **bd's count and the census differ by small numbers on a healthy store,
  by construction.** The live jsonl holds 2950 distinct ids and `bd list
  --all --json` lists 2948: three jsonl rows carry `status: tombstone`, which
  `--all` does not list, and one id (`ranger-base-zrff`) is in bd and not yet
  in the jsonl — 0.50.x does not auto-flush. So an EQUALITY check between the
  two is wrong on the very first store it meets. Zero against non-zero is the
  only reading that is a defect by construction: a database holding no issue
  at all over a projection holding one is a database that never imported.
- **Posse cannot repair it with the self-heal it already has.** `Bd.run`
  fires `bd sync --import-only` on bd's staleness refusal and retries once.
  Over this state that verb exits 1: `import failed: database not
  initialized: issue_prefix config is missing (run 'bd init --prefix
  <prefix>' first)` — measured over the committed-jsonl fixture and the
  uncommitted one alike, 0 rows after. `bd import -i .beads/issues.jsonl` is
  the repair (the operator's, issue #5; REPORTED, not re-measured here — the
  verb is in every persona PID's deny set and this seat cannot type it), and
  so is deleting the zero-row file, which a5st4 ranks worse because the
  rebuild reads `git HEAD` and drops uncommitted rows.
- **Posse already reads the census, and already compares it to bd.** The
  lost-bead sweep (`beadloss.go`) walks the git history of `issues.jsonl` and
  reports every id a commit dropped that bd can no longer resolve: "the alarm
  the substrate does not ring". The ceiling (ADR 0068) and the queue commit
  (`queuejsonl.go`) read the same file. This record is that alarm one row
  over: the sweep catches ids that LEFT the census; this catches bd losing
  every id while the census kept them — which the sweep cannot see, because
  nothing was dropped from git.
- **What ADR 0055 D3 refused, exactly.** "No store-class detector, no
  refusal, no doctor": posse does not read `config.yaml` for `no-db`, grep
  argv for `--no-db` or judge `BD_NO_DB` — a copy of bd's own mode reader,
  covering one door of four, for a fix that was mode-independent. The bead
  read D3 as refusing any second reader of the store. It refused a reader of
  bd's CONFIGURATION to detect a MODE. The check below reads bd's ANSWER and
  the file posse already treats as the census, to detect an ABSENCE; D3 is
  untouched, and its Status line now names this record so the next reader does
  not have to re-derive the boundary.
- **Every reader already has an error path that says the honest thing.**
  `ReadyAll` returns `ScanError`s and dispatch prints `✗ ready scan failed`
  per repo and dies `ready scan failed in all N beads repo(s) — the queue is
  unknown, not empty` when nothing was readable; `posse ready` dies the same
  way; governance collects the `ScanError`; retire's `beadStatuses.err` keeps
  "unreadable" apart from "no such bead" by name. The silent shape exists only
  because `[]` never reaches those paths.

## Decision

1. **One list helper, and an empty list is a question.** Every reader of a
   bd issue LIST — `ListAll`, `Ready`, `InProgress`, `OpenLabeledAny`,
   `AllLabeledAny` — goes through one unexported helper in `beads.go` that
   runs the verb, parses the rows, and when the parsed list is EMPTY asks
   `b.emptyIsReadable(dir)` before returning `nil, nil`. Non-empty answers are
   returned exactly as today; this record adds no cost to a read that found
   anything.

2. **The check is two reads, cheapest first, and compares against zero
   only.**
   - The census: `beadsHome(dir)/issues.jsonl`. If the file is absent, or
     holds no line beginning `{`, the store has no census and the empty
     answer is honest — a brand-new store before its first bead, or a repo
     that is not a queue. Read to the first record and stop: the live file is
     22.7 MB and the question is "at least one", never "how many". No fork.
   - bd's own count: `bd info --json`, field `issue_count`, through the same
     `runOnce` as every other verb (same `--no-daemon`, same `BEADS_DIR`
     binding, same deadline). `issue_count > 0` means the graph is there and
     this list was empty on its own terms: return `nil, nil`. `issue_count ==
     0` over a census that holds a record is the defect: return the error.
   - `bd info` failing (non-zero exit, missing field, unparseable) is the
     store being unreadable by a second route and is returned as THAT error,
     never swallowed into `nil, nil`: the whole of this record is that an
     unanswerable question does not read as "empty".
   - Never equality, never a threshold. The 2950-vs-2948 measurement above is
     a healthy store, and tombstones and unflushed writes make the two counts
     drift on every busy day. A reader that tried to reconcile them would be
     the second resolver the field warns about (single-writer-and-stores: a
     fact in two stores can disagree, so one of them is a hint). bd's count
     is the authority; the census is consulted only for "is there anything
     at all".

3. **The error is typed, and it carries the repair verbatim.**
   `BdStoreUnreadError{Dir, Store string}`, `errors.As`-able like
   `BdHangError`, whose `Error()` reads, in one line: the store directory, that
   bd holds 0 issues over a census that carries rows, that this is bd 0.50.3's
   zero-row database (a read over an uncommitted jsonl, or a read-only import
   that failed — ADR 0071), and the operator's one command from the repo that
   owns the store: `bd import -i .beads/issues.jsonl`. The sentence names the
   REPO (`beadsHome` resolved through the redirect), because on this instance
   the store of record is reached through one and the repo a persona is
   sitting in is not the repo that owns it (AGENTS.md, "Reading ANOTHER repo's
   queue").

4. **Report, never repair.** Posse does not run `bd import`, does not delete
   `beads.db`, and does not extend `run`'s stale-db self-heal to this message:
   the measured `sync --import-only` fails over the state, the verb that works
   is denied to every persona and reserved to the operator by the 2026-10-08
   ruling on a5st4, and the two repairs differ in what they keep (working tree
   versus `git HEAD`), which is a choice with the operator's uncommitted rows
   on one side of it. ADR 0063's shape: a pass that cannot read its queue says
   so every pass and hires nobody; `--watch` reports a pass error and keeps
   its cadence, and the next pass after the import reads the queue again
   with no restart. No memo of a healthy answer (see Alternatives): the check
   is re-asked on every empty read so the repair clears it immediately.

5. **No caller changes.** `ReadyAll`, `posse ready`, the dispatch pass, the
   refill, governance G9, retire's listing and the lost-bead sweep keep the
   error paths they have; the sweep's line `bead-loss check: <err>` and
   dispatch's `✗ ready scan failed: <err>` will both name the same store on
   the same pass, and that is two readers agreeing, not a duplicate to
   suppress. `InProgressAll` keeps swallowing errors, as its own comment says
   it must (a display, never the queue) — the loud line is `ReadyAll`'s on the
   same cockpit render.

6. **Pins.** Offline, through the suite's fake bd (`fakeBd`, herdr_test.go),
   which gains an `info` case serving `fake-info.json` from its working
   directory and, with no such file, `{"issue_count": 1}` — so every existing
   fixture keeps meaning what it meant, and a repo with no `issues.jsonl`
   never reaches `info` at all:
   - `Ready` over `fake-ready.json` = `[]`, a census holding one record, and
     `fake-info.json` carrying `issue_count: 0` returns `BdStoreUnreadError`
     naming the store; the same through `ReadyAll` lands in `failed` and
     dispatch prints the unknown-queue line, not `no ready work`.
   - the same fixture with `issue_count: 1` returns `nil, nil`; with no census
     file and `issue_count: 0` returns `nil, nil` and `bd-calls.log` shows no
     `info` call.
   - `fake-info.json` holding `{"error": "x"}` with exit 1 returns an error,
     not `nil, nil`.
   - a non-empty `fake-ready.json` makes no `info` call (`bd-calls.log`).
   Live, under `RHQ_LIVE_BD=1`, holden's rig built in `t.TempDir()`: `Bd.Ready`
   over the zero-row store returns `BdStoreUnreadError`; over the same rig
   with the jsonl committed BEFORE the first read (a5st4 row one, the case
   that works) it returns the two rows. The repair arm is not pinned — the
   verb is denied at every seat that would run the suite — and the error's
   command text is pinned by the offline arm instead.

## Consequences

- An empty queue costs one more bd call per empty list read, ~0.5s on the
  live store (MEASURED above), and nothing when the list was non-empty or the
  repo has no census. A quiet dispatch pass reads several lists that are
  empty by design (`InProgress`, labeled reads for verify-after), so the pass
  tax is some multiple of that — ASSUMED under five seconds on this instance,
  against a watch cadence in minutes. The first code bead measures the count
  of empty list reads on one quiet pass and writes it on the bead; the memo
  below is filed only if that number makes the tax matter.
- The cockpit and the loop say `queue unknown` over a store the operator has
  to import by hand, where they said `no ready work`. That is the whole
  change in behaviour, and it is the one the bead asked for.
- The stale-past-zero class (a database holding two rows under a HEAD with
  three, a5st4 finding 4) is NOT detected here, on purpose: zero is the only
  count that is a defect without a reconciliation, and the measurement above
  shows a healthy store two rows apart. That class stays with the lost-bead
  sweep's domain and with bd's own flush discipline.
- bd's `Try manually:` line still steers a persona at a denied verb. The
  error text here names the repair as the OPERATOR's, which is what INSTALL
  §14 already says; the code bead adds the posse-side sentence to that row.

## Alternatives rejected

- **Nothing: document and stop** (ADR 0055's floor, again). The state was
  reached by the operator on the work box, found by two people a day apart,
  and every reader posse ships printed "empty" over it. A defect only a human
  with a terminal and the human verb can see is the silent shape rangerhq-llse
  already ruled against.
- **Run the human form beside the JSON one and read its stderr for
  `auto-import failed`.** One extra bd call, same cost as `info` — and it sees
  only ONE of the two zero-row fixtures: over an uncommitted jsonl the human
  form prints nothing at all (a5st4 rows four and five). And it reads a
  warning sentence of a pinned binary, which is the pattern ADR 0056 keeps to
  one tripwire. `info` is a documented verb with a documented field, whose
  own help says it exists to "debug issues where bd is using an unexpected
  database".
- **Count the census and compare to bd's count.** Wrong on the first store
  it meets (2950 vs 2948, healthy). Tombstones and unflushed writes are not a
  defect; a reconciler over them is a second resolver.
- **Read `beads.db` with sqlite.** A reader of bd's DATABASE, which is the
  store of record itself; the notes fragment did this from a seat to MEASURE,
  and that is where it stays. `info` is bd answering the same question
  through its own front door.
- **Repair: extend `run`'s self-heal to fire `sync --import-only`.**
  MEASURED exit 1 over the state; it cannot. **Repair: run `bd import -i`
  from posse's own process.** The operator's cockpit process is not under a
  persona PID, so it could — and it would decide, unattended, between a
  working-tree import and a HEAD rebuild with the operator's uncommitted rows
  at stake, over a state the 2026-10-08 ruling reserved to the operator's own
  hand. ADR 0063 and ADR 0068 both land on report-and-name-the-command for a
  remedy the reader should not perform.
- **Memoize "healthy" per store for the process lifetime** (one `info` per
  store per `--watch` run). Cheaper by the multiple above, and it goes blind
  on exactly the store that breaks mid-run (a `beads.db` deleted and rebuilt
  over an uncommitted jsonl) until a restart nobody schedules. Not worth it at
  0.5s until the per-pass count says otherwise; filed then, with the number.
- **Do it in the lost-bead sweep** (one more finding: "every id lost").
  The sweep runs once per pass and WARNS; the queue is read by five callers on
  their own schedule and acts. A warning line at the head of the pass followed
  by `no ready work` is the silent shape with a preamble. The wrapper is where
  every reader already goes, and the error is what every reader already
  handles.
- **A `posse doctor` / `posse beads check` arm instead of the read path.**
  Useful, and `posse beads check` already exists for the sweep's findings —
  but a check the operator types after noticing is the same lag as the human
  `bd list`. The read path is where the defect lies to the loop.
- **Lift the bd pin.** No bd fixes this and keeps the JSONL/SQLite layers
  (`etc/bd/version-pin.toml`); fixing it upstream and migrating off it are the
  same move, and the 2026-10-08 ruling says nothing goes upstream.
- **The clever one: make `Bd.Ready` read the jsonl itself when bd answers
  empty, and serve the rows.** Posse as a second query engine over bd's
  projection — no blocked set, no dep graph, no status semantics, rows that
  may be unflushed or tombstoned — serving a queue bd itself cannot see, and
  then hiring on it. Every bead claimed from that path would be a write bd
  refuses (`create` exits 1 over the state). One resolver (ADR 0055's last
  rejected row, for the same reason).

## Verification and evidence

MEASURED 2026-10-09 as the Context says: the five-step rig and every `--json`
verb's bytes over it (`[]`, 0, empty stderr), `bd info --json` over the broken
store (0), the live store of record (2948), a `no-db: true` scratch store (1,
`database_path` the directory), `bd sync --import-only` over both zero-row
fixtures (exit 1, `issue_prefix config is missing`, 0 rows after), `bd info`
timing (0.49/0.54/0.68s) against `ready` (0.31/0.32s), and the 2950-vs-2948
gap resolved by id (`comm` over `bd list --all --json` ids and the jsonl's:
three tombstones the listing hides, one unflushed id). Cost of the census
probe: a 4 KB `head` over the 22.7 MB file, unmeasurable against a fork.

REPORTED, not measured here: `bd import -i .beads/issues.jsonl` repairs the
state (issue #5, the operator; a5st4 records the verb's deny status at every
persona seat, which is why this seat could not re-run it).

ASSUMED: the per-pass count of empty list reads on a quiet pass (the first
code bead measures it); that no caller of the five readers treats
`nil, nil` and an error differently in a way this record did not read —
every call site named in Context was read, and each carries an error path.
