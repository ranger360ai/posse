# A `no-db: true` beads store drops every defer date (ranger-base-bwp7h)

Discovered from ranger-base-nkjjg, whose subject is posse's readers; this is
the store's own mode, which no reader can reach.

Everything below is MEASURED 2026-10-03 on darwin 25.4.0 with the pinned bd —
`bd version 0.50.3 (bd25acbc)`, `etc/bd/version-pin.toml` — unless it says
ASSUMED. The live store the symptom was found in is a **private** repo's
(`no-db: true`); its bead ids and the two dates are on ranger-base-bwp7h in
the queue store, which is private, and are deliberately not repeated here,
because this repo is stamped `public`.

Re-measure, don't quote: `make verify-nodb-defer`
(`scripts/verify-nodb-defer.py`, `--self-test` for its own arms).

## 1 · The defect, and exactly how wide it is

| | no-db store | store with a database |
|---|---|---|
| `bd defer <id> --until <date>` | status→`deferred`, prints `* Deferred <id>`, **no `defer_until`** | status→`deferred`, `defer_until` written |
| `bd defer <id> --until=tomorrow` | same, no date | — |
| `bd update <id> --defer <date>` | prints `Updated issue: <id>`, status→`deferred`, **no date** | — |
| re-defer a record that HAS a date | reports success, **old date left in place** | — |
| a hand-written `defer_until` in the JSONL | `bd show` prints `Deferred: <date>`; in `bd show --json` and `bd list --json`; **survives a later unrelated bd write** | — |
| a past-dated deferred bead in `bd ready` | absent | absent |

Four separate reads agree on "no date": the `.beads/issues.jsonl` record,
`bd list --json`, `bd show --json`, and `bd show`'s human frame, which prints
the snowflake and `DEFERRED` with no `Deferred:` field after `Updated`.

So it is the **defer writer alone**, in no-db mode. The reader is whole, and
that is what makes the workaround in §4 durable: a date written into the JSONL
by hand is read back by every surface bd has, and `bd comments add` rewrote the
whole file and kept it.

The last row is the one that makes a dateless park permanent. **bd re-surfaces
nothing**: a bead deferred to a date four weeks past is still absent from
`bd ready`, in BOTH store classes. The thing that makes a park loud again is
posse's reader, and it keys on the date and never on the status string —
`BdIssue.DeferUntil` (internal/posse/beads.go), the G3 arm
(internal/posse/govern.go, "once the date is past, the park has expired and
nobody revisited it"), `deferredNow` (internal/posse/interrupted.go). With no
date there is nothing for it to read, so a dated park and an indefinite one are
the same record, and the intended date survives only wherever a human wrote it
down — in this case, in the bead's own comments.

## 2 · There is no version to wait for

Reporting it upstream and "park on the version that fixes it" were two of the
three candidate shapes on the bead. Both are void, and for the same reason:

- 0.50.3 is the **last** 0.50.x. The next release is 0.51.0.
- 0.51.0 is the Dolt-native cleanup. Its own changelog phases: "Phase 5:
  Remove JSONL sync layer", "Phase 6: Remove SQLite backend entirely". The
  mode this defect lives in is deleted there, not fixed.
- MEASURED: the 1.x binary on this box (the unlinked homebrew keg, 1.2.2) has
  no `--no-db` global flag at all, and on a `no-db: true` store it answers
  `defer` with `Error: no beads database found` / `Hint: … 'bd init' to create
  a new database`. It will not read such a store at all.

So an upstream bug report asks for a fix to a code path that is gone, and
"take the store off no-db" and "upgrade to a bd that gets this right" are the
**same migration** — which is the operator's twice over: the pin is theirs
(`etc/bd/version-pin.toml`: "LIFTING THE PIN is the operator's, and 1.x is a
migration, not a version bump"), and 1.x is Dolt.

ASSUMED, not measured: that no 0.50.x patch between the pin and 0.51.0 fixed
the writer. There is no such patch to test — 0.50.3 is the last — so the claim
is about the release list, read from the 1.2.2 changelog, not about a binary
anybody ran.

## 3 · What the check does, and why arm B is asymmetric

`scripts/verify-nodb-defer.py` is the re-measuring artifact, because the
deliverable here is a STATE and the state regenerates: every later `bd defer`
in a no-db store makes another dateless park.

- **Arm A** builds a throwaway no-db store, defers in it, and reads the JSONL
  back. Defect still there → the verdict this check exists to carry. Defect
  **gone** → exit 1, loudly: the pin moved or something that is not the pinned
  binary is answering as `bd`, and §4 plus this file can be retired. Store
  refused outright → the no-db mode itself is gone (§2), also exit 1.
  It canaries on `bd where` before deferring anything, because bd resolves
  `.beads` from the git root and follows one redirect hop — an un-init'd temp
  dir inside a worktree would resolve to the worktree's own store.
- **Arm B** reads the stores named on argv or in `POSSE_NODB_STORES`, and
  fails on a dateless park **only in a no-db store**. In a store with a
  database, `bd defer <id>` with no `--until` is the icebox and means what it
  says — the queue store's own roadmap bead sits there on purpose, and an arm
  that failed it would be noise. In a no-db store the same record is ambiguous
  by construction: a park somebody dated and a park somebody did not are
  byte-identical. A store whose `config.yaml` cannot be read counts as no-db,
  which is the loud direction.
- With no store named, arm B exits **2** and says so. Finding nothing over
  zero stores is the shape that reads green while a failed producer hands the
  check nothing.

`--self-test` runs arm A against a stub bd that drops the date, one that writes
it and one that refuses the store, and requires a different verdict from each;
arm B against planted stores with two dateless parks, with none, with a
database, empty, absent, and with an unreadable mode. A rig that could not come
out both ways would report "defect present" over a bd that had fixed it.

## 4 · Re-parking in a no-db store is a JSONL edit

`bd defer --until` cannot set the date and cannot move one that is already
there, so in a no-db store the date is written into the record:

- insert `"defer_until":"<YYYY-MM-DD>T00:00:00<±offset>"` immediately **before**
  the top-level `"labels"` key — that is where bd's own writer puts it
  (measured against a database-backed store's `issues.jsonl`), and midnight
  local with an explicit offset is the value bd itself writes;
- splice the text, do not re-serialise the file. A `json` round-trip of a
  bd-written JSONL is **not** byte-identical — 13 of 16 lines changed on the
  store measured here, because Go and Python escape differently — so a
  re-serialising edit churns every record it touches;
- then read it back with `bd show <id>` and look for the `Deferred:` field.
  The file is not the proof; what bd says it reads is.

## 5 · Where the rest of this went

- The posse-side reader was ranger-base-nkjjg's (the code lane).
- ranger-base-pm5zo, the third candidate shape, is the complement of that fix:
  once a dateless deferred question bead stops paging, nothing makes it loud
  again — not bd and not posse — and in a no-db store every `bd defer` keeps
  producing one. It has to pick the N in "quiet for N days, then loud" on the
  record; nobody has measured one.
- ranger-base-6ulbg is the operator's, and is both halves of what this lane
  cannot do: the one command that re-enters the dates (the write was refused
  at the session's permission layer — a live store is a shared resource), and
  the ruling on whether the store keeps `no-db: true`, given §2.
