# A cleanup trap that owned nothing, because the pick ran in a subshell (ranger-base-m3fl5)

MEASURED 2026-11-12, this box (linux 7.2.15 x86_64), against this instance's
own beads store — the one `.beads/redirect` names — at 68,837,376 bytes.

`scripts/verify-bd-dep-safety.sh` and `scripts/prune-bd-relates-to.sh` each
read the beads db through a `q` helper, and each carried the same `pick_reader`
behind it: open the store `mode=ro` if sqlite can, else copy the db to a
`mktemp -d` and read the copy. The copy was meant to be given back by

```sh
RO_TMPS=""
cleanup_ro() { [ -z "$RO_TMPS" ] || rm -rf $RO_TMPS; }
trap cleanup_ro EXIT
```

and it never was, because of where `pick_reader` ran. `q` picked lazily — `[ -n
"$DB_READ" ] || pick_reader` — and **every caller of `q` is a command
substitution**: `verify-bd-dep-safety.sh:146,163,175,179,182` and
`prune-bd-relates-to.sh:193,250`. A `$(…)` is a subshell, so the pick happened
in a child: `RO_TMP`/`DB_READ` were assigned there and died with it. The
parent's EXIT trap then read an empty `RO_TMPS` and removed nothing — and since
`DB_READ` did not survive either, the memo was gone too and the next query
picked again. One leaked snapshot **per query**, not per run.

The trap was not wrong. It was reading a variable that nothing in the process
that owned it had ever assigned.

## Latent here, and why it was still worth fixing

`/usr/bin/sqlite3` on this box opens the store `mode=ro`, so the fast path wins
and the fallback never runs. It is the branch an older or reduced sqlite3
takes, and the branch a WAL-mode store with no `-shm` file takes on any
sqlite3 — the state the comment above `pick_reader` was written for. Driving it
takes a rig: a `sqlite3` PATH shim that refuses exactly the `file:…?mode=ro`
URI and `exec`s `/usr/bin/sqlite3` for everything else, with `TMPDIR` pointed
at a fresh scratch root.

Under that rig, before and after, with `probes` = how many times the shim saw
the `mode=ro` URI (so, how many times `pick_reader` ran) and `leftovers` =
directories left in `TMPDIR`:

| invocation | probes before | leftovers before | probes after | leftovers after |
|---|---|---|---|---|
| `verify-bd-dep-safety.sh` (no args, 3 queries) | 3 | 3 | 1 | 0 |
| `verify-bd-dep-safety.sh --gate` | 1 | 1 | 1 | 0 |
| `verify-bd-dep-safety.sh <target>` | 1 | 1 | 1 | 0 |
| `prune-bd-relates-to.sh` (dry run) | 1 | 1 | 1 | 0 |
| `prune-bd-relates-to.sh --apply` | 2 | 2 | 2 | 0 |

Each of those directories holds a copy of the db, so at 68.8M the no-arg audit
spent 206M of a shared 14G tmpfs and gave none of it back. `probes` is the
other half of the bug and the reason the cost is per-query: a 3 that should be
a 1 is the lost memo, and it is also two copies of a 68.8M file that nobody
asked for. `--apply` legitimately probes twice — the post-write re-read is a
second, deliberate pick — and now releases the first snapshot before taking the
second.

## The fix

One slot, not a list, and the pick hoisted into the shell that owns the trap:

- `RO_TMP` holds at most one path. `pick_reader` calls `cleanup_ro` first, so a
  re-pick gives the previous copy back before taking another — which is what
  makes `--apply`'s re-read free rather than a second leak.
- `q` no longer picks at all. It fails loudly on an unset `DB_READ` instead,
  because an unpicked reader is a bug in the caller and not a reason to pick
  inside a `$(…)`.
- Each script calls `pick_reader` once, unconditionally, before its first
  query — every branch of both scripts queries — and `prune`'s post-apply
  re-read replaces its `DB_READ=""` with a plain `pick_reader` on the same line
  of reasoning.
- The `rm` is `rm -rf -- "${RO_TMP:?}"`. The `:?` is what makes the expansion
  provably non-empty, which is both the safety and what lets the permission
  coordinator read the command (AGENTS.md, "Shapes that stall the seat").

## Verified by execution

- The rig above, old code (`git show HEAD:`) and new, same shim, same fresh
  `TMPDIR` per run: the table. Output byte-identical between old and new in
  every form; only the leftovers and the probe count moved.
- `--apply` is the one path a dry run never reaches, so it was driven on a
  **throwaway** store: a scratch git repo whose `.beads/beads.db` is a copy of
  the fleet db, satisfying `store_owner`'s three conditions, with one symmetric
  pair planted by two opposite `bd dep add -t relates-to` calls between two ids
  that had no dependency rows at all. Both codes pruned it to 0 remaining rows
  and printed the same report; leftovers 2 → 0. Nothing in the real store was
  written at any point.
- Incidentally MEASURED on that store: pinned bd 0.50.3 stores the type string
  `relates-to`, which is what `prune-bd-relates-to.sh`'s foreign-type guard
  matches on. The fleet store's own symmetric-pair-free `dependencies` table
  carries `related` rows among its types; a symmetric pair of that spelling
  would be refused by that guard as "a type this cannot unlink" rather than
  guessed at, which is the guard working.
- Fast path (no shim, this box's sqlite3): all four forms plus `--apply`, zero
  leftovers, unchanged output — the fast path never copies and still does not.
- `make verify-shell-syntax` (70 scripts parse, 9 self-test arms),
  `make verify-bd-no-relate-pairs`, `make verify-bd-dep-safety`,
  `make prune-bd-relates-to` (dry run on the fleet store): all green.
- `make tree-check`: green twice — 72.6s at a one-minute load average of 17.87
  with one suite slot held by a sibling seat (the scripts alone), then 54.9s at
  a load of 8.30 with this fragment added, which is the reading that counts
  because `corpus-check`, `doc-check`, `adr-check` and `notes-check` all read
  `docs/notes.d/`. Both inside the 55-62s-under-a-load-of-16 band AGENTS.md
  prices the command at; the work here cost it nothing measurable.
- `go run ./cmd/checkorphans`: clean.

## Not closed here

- ranger-base-tbukc — five more `mktemp -d` roots under `scripts/` carry a
  template that matches no class row in `scripts/reap-test-tmp.sh`, so a leak
  out of one of those is never collected rather than quarantined. A sibling of
  this bead under ranger-base-i78k9 and not a residual of this fix: these two
  roots are `posse-*`, and they are now given back by their own trap rather
  than left to any reaper.
