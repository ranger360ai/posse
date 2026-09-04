# ADR 0056 — `--no-daemon` stays on every posse `bd` argv as the pin's tripwire, not as a speedup

*Status: accepted 2026-09-04 · owner: architect · re-grounds `bdGlobalFlags`
(ranger-base-cwu7) and `CageBdFlags` (ADR 0002 amendment, rangerhq-3nxk) on
the pinned bd 0.50.3 · bead ranger-base-a67nu, discovered from
ranger-base-c201c*

## Context

posse leads every `bd` argv with `--no-daemon`: `bdGlobalFlags` in
`internal/posse/beads.go`, `CageBdFlags` in `internal/posse/cageinner.go`
(beside `--no-db`), `scripts/verify-bd-pin.sh` and
`scripts/queue-cutover.sh`. The doc comment on `bdGlobalFlags` justifies it
as a 12x speedup measured 2026-08-30 on bd 0.49.1: ~5.3s of every ~5.6s
store-touching call was a dial for a daemon that could not start, and the
flag skipped the dial. The same comment frames the stale-db self-heal in
`Bd.run` as the reader-side cost of that trade.

The pin has been 0.50.3 since 2026-09-01 (operator ruling ranger-base-qrh1,
declared in `etc/bd/version-pin.toml`). 0.50.0 deleted the daemon and the
RPC layer outright; `--no-daemon` survives there as a documented deprecated
no-op ("(deprecated) All operations use direct mode"), kept upstream so
existing hooks do not break, and is deleted at 0.51.0 (NOTES, "the ~5.3s
daemon dial is fixed upstream", ranger-base-db04). So the number the comment
argues from no longer holds on the binary the comment is read against, and
the comment is the thing anyone reads to decide whether the flag may go.

MEASURED 2026-09-04 on the pinned 0.50.3, this box, four axes:

- **Time.** Both arms cost the same: five runs each of `list --all --json`,
  with and without the flag, on a no-db JSONL rig and on the fleet's SQLite
  store, 0.42–0.66s either way (ranger-base-a67nu's own measurement).
- **Rows.** The same rows from both arms once the rig names its store
  class (ranger-base-c201c's finding; the bare-JSONL rig that used to
  materialise a store on 0.49.1 answers nothing on 0.50.3).
- **Stderr.** `bd --no-daemon list --json --limit 1` exits 0 with 0 bytes
  on stderr — there is no deprecation warning for `runOnce` to swallow.
- **Staleness.** Two independent rigs, each a `cp -p` of the fleet store
  into a fresh `git init` plus `touch .beads/issues.jsonl`: `list --json`
  refuses with `Database out of sync with JSONL`, rc 1, on stdout, with the
  flag, without it, and in either order, the first call changing nothing
  for the second. On 0.50.3 every mode is direct mode: the flag no longer
  chooses the refusing side because there is no other side.

And the one measurement that decides the page — what a bd past the pin does
with the flag. Homebrew carries beads 1.2.2 as an unlinked keg (the 08-16
outage was `brew` linking it in front of the pin). Run from its keg path in
an isolated scratch store, `bd --no-daemon version` answers `Error: unknown
flag: --no-daemon`, rc 1, and creates no `.beads`; the same binary's plain
`bd version` runs at rc 0. NOTES (ranger-base-db04, 2026-08-27) measured the
same refusal from the 0.51.0 release binary. The refusal is cobra's flag
parser, which runs before any store is opened.

Where that matters: the launcher and cockpit call `bd` from the operator's
PATH, on which `/opt/homebrew/bin` precedes the pinned `~/.local/bin`
(`version-pin.toml`). Persona sessions reach bd through the gate shim,
whose exec line names the pinned binary. `scripts/verify-bd-pin.sh` is the
declared checker and runs when someone runs it; nothing on the launcher's
path runs it per call.

## Decision

**D1. The flag stays, everywhere it is today, and its reason changes.** It
is the pin's tripwire. On 0.50.3 it costs nothing on all four axes above.
On any bd past the pin's line it turns every posse call into `unknown
flag`, rc 1, at the flag parser — before a 1.x binary can migrate a SQLite
store or refuse it in its own words, and without anyone running the
checker. It is the only refusal on the launcher's path with that property,
and it is already there.

**D2. The record leads with D1; the 0.49.1 numbers stay as dated history.**
The `bdGlobalFlags` comment's lead paragraph states D1 and the pinned
version it was measured on. The cwu7 sweep and the p969 measurement stay
verbatim beneath a heading of the form `HISTORY (bd 0.49.1, measured
2026-08-30)`: they were true when measured and they are why the seam exists
on the runner rather than on a list of read verbs; they are no longer the
reason the flag is there. `CageBdFlags` gets the same treatment: its
2026-08-22 measurement was 0.49.1 in the image, the container tier is
unrunnable on this box, and `--no-db` is the measured, load-bearing half
that is untouched by this record.

**D3. The stale-db self-heal is re-grounded, not changed.** `staleDBMessage`
and the import-once-retry-once in `Bd.run` stay as built. What goes is the
framing: on 0.50.3 the refusal is bd's staleness check — the jsonl's mtime
against an import marker — and it is identical with and without the flag.
Its writers are no longer daemon flushes but a `git pull` or merge, bd's
own pre-commit hook rewriting `issues.jsonl`, the launcher's explicit
`Bd.Flush` (0.50.x no longer auto-flushes), another worktree's flush, and a
bare `touch`. The sentence "a live daemon auto-imports … direct mode
refuses instead" describes a side that no longer exists and is retired
into the history block.

**D4. The pins.** `TestBdRunCarriesNoDaemonOnEveryVerb` keeps the seam and
is re-headed: under D1 a verb that builds its own argv is a verb the
tripwire misses, which is the regression the pin catches. The live pin's
timing comparison (`direct ≤ dialed/2`) is retired — both arms are one arm
on 0.50.3 and the comparison reds by construction. What the live pin
asserts is D1's premise: the pinned bd accepts the flag and answers the
same rows with and without it. That file is ranger-base-c201c's while it is
in flight; the retirement is a separate bead that waits on it.

**D5. Lifting the pin cuts the tripwire on purpose.** The `LIFTING THE PIN`
paragraph of `etc/bd/version-pin.toml` names the four spellings —
`bdGlobalFlags`, `CageBdFlags`, `verify-bd-pin.sh`, `queue-cutover.sh` —
as the first step of the migration, so the day every call fails with a
clear message is a runbook line and not a surprise. `verify-bd-pin.sh`'s
`|| bd version` fallback is correct and stays: the checker must reach its
version row; the launcher must not.

## Consequences

- Nothing changes in any argv, in behaviour, or in timing. Two comments in
  `beads.go`, one in `cageinner.go`, one paragraph in `version-pin.toml`,
  and one retired test assertion.
- The tripwire fails closed for bd ≥ 0.51 (MEASURED: 0.51.0, 1.2.2) and is
  blind to 0.49.x, which accepts the flag (MEASURED, cwu7). 0.49.x is the
  previous pin and the checker's version row is the guard for it. A fork
  that keeps the compat flag would also pass; none is on this box.
- The coupling is now explicit. Every posse argv is spelled for the
  ≤ 0.50 line. What that buys is a migration whose first symptom is one
  uniform message from every call; what it costs is that D5's runbook
  line is load-bearing, and a reader who finds the flag without reading
  the comment will still take it for a speedup — which is why the comment
  leads with D1 and not with the history.
- The seam pin protects a no-op on the pinned binary. Its value is the
  tripwire's coverage, and its header says so; a pin whose header still
  said "12x" would be the same trap this bead was filed over.

## Alternatives rejected

1. **Drop the flag everywhere** — the tidy one, and the bead's question
   (2). Priced: 21 files spell it (5 outside tests: `beads.go`,
   `cageinner.go`, comments in `cage.go`, `reachability.go`,
   `beadpulse.go`), fakes keyed on `$1 = --no-daemon` in `bdpin_qa_test.go`
   and the two cockpit tests, both scripts, and a collision with c201c's
   live edit. It buys nothing on 0.50.3 and removes the one refusal that
   fires before a foreign binary reaches the store. On the day it would
   pay, it is one variable inside a migration that already freezes writers.
2. **A runtime version check in `Bd.run`** — the clever one I wanted:
   exec `bd version` once per process, compare to `version-pin.toml`. A
   second checker for one pin, a toml resolver inside the runner, one more
   exec per process, and it passes whatever it did not run against; the
   flag refuses at the parser with no code of ours. `verify-bd-pin.sh`
   remains the explicit checker and the operator's.
3. **A version-conditional flag** — probe once, add the flag only below
   0.51. Makes the tripwire depend on the thing it guards, and invents a
   mechanism for a question with a one-line answer on migration day.
4. **Drop it from the host runner, keep it in the cage** — two spellings of
   one policy, with the exception on the one tier that cannot be measured
   here. One rule, applied everywhere, cut everywhere at once.
5. **Rewrite the 0.49.1 paragraphs to today's numbers** — a measurement
   dated and versioned is history; it stays as written under a heading
   that dates it, and only the lead and the heading move.

## Verification and evidence

MEASURED 2026-09-04, bd 0.50.3 (gate shim → pinned binary), this box: the
four axes in Context; the 1.2.2 keg refusal (`/opt/homebrew/Cellar/beads/
1.2.2/bin/bd`, `BEADS_DIR` pointed at an empty scratch store under a fresh
`git init`, never on PATH). MEASURED earlier and cited: 0.51.0 rejects the
flag and does not read `beads.db` (NOTES, ranger-base-db04); the staleness
refusal and its stdout-vs-stderr spelling per verb (ranger-base-inomb,
2026-09-02); the 0.49.1 numbers (cwu7, p969).

ASSUMED: the container image's bd accepts the flag exactly as the host's
does (the tier is unrunnable on this box; `CageBdFlags` is unchanged
either way); no bd on any fleet box keeps the compat flag past 0.50.x.

The staleness recipe, for the next measurer: copy the store's files with
`cp -p` into `<scratch>/.beads` under its own `git init`, `touch
.beads/issues.jsonl`, run one arm — a jsonl dated into the future is
unhealable, so never date the fixture forward.
