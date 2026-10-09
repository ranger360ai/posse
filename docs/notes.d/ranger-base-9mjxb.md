# ranger-base-9mjxb — bd's store-selecting flag writes across repositories

github.com/ranger360ai/posse issue 9, filed by the operator from the work box
2026-10-08. On the pinned bd, a call made from a cwd inside repo A that names
repo B's database with bd's explicit store-selecting global flag auto-imports
A's `.beads/issues.jsonl` — discovered from the cwd's git repo — into B's
store. A READ verb writes one repository's records into another repository's
database, silently and beyond the import line. Upstream defect; we are pinned
and will not file there (standing ruling 2026-10-08).

This fragment is the posse-side audit, what it changed, and what it could not
settle.

## The audit — every posse-side bd call site, 2026-10-09 at 03fba176

The question asked of each: does it hand bd a store-selecting flag, or run bd
from a working directory other than the store's own repo?

| call site | verdict |
|---|---|
| `internal/posse/beads.go` `Bd.runOnce` | **clean.** Sets `cmd.Dir` and rebinds `BEADS_DIR` to `beadsHome(dir)` (`bdStoreEnv`, shedding any inherited `BEADS_DIR`/`BEADS_DB` first); `bdGlobalFlags` is `--no-daemon` and nothing else. This is every bd call posse's Go makes. |
| the rendered bd gate shim (`internal/posse/gates.go`) | **clean.** It MATCHES the flag so its verb resolver skips the value instead of reading it as the verb (`globalValueOpts["bd"]`, ranger-base-3bqn), then `exec`s the real bd with the persona's argv unchanged. It adds no flag and removes none. |
| `scripts/bd-argv-gate.py` | **clean**, same shape as the shim: `BD_VALUE_OPTS` is a parser table over an argv a persona typed. |
| `scripts/verify-bd-dep-safety.sh` | **clean.** Resolves the store the same way but reads it with `sqlite3` only; it runs no bd at all. |
| `scripts/queue-cutover.sh` | **clean.** Its bd lines are PRINTED for the operator and never run. |
| **`scripts/prune-bd-relates-to.sh`** | **the one violation.** Three writes — two `comments add` and one `dep unrelate` — made from whatever directory the operator was standing in, each carrying the flag, against a database its own usage block documents `BEADS_DB` as pointing at in another repository. It also passed no `--no-daemon`, so it ran bd's daemon path. |

## What changed

`scripts/prune-bd-relates-to.sh` resolves the repo that OWNS the store and
makes its writes from inside it, with no store flag and with `--no-daemon`.
`store_owner` requires three things of a store's directory, all load-bearing
because a chdir to the wrong directory silently reads a different database
than the SQL above it measured:

1. the database sits in a `.beads` directory, so its parent is the repo;
2. that parent is a git work tree's TOPLEVEL;
3. that repo's own `.beads` resolves BACK to this store, one redirect hop
   included — the condition the redirect shape needs, since a
   `<repo>/.beads/redirect` can name a store in another repo entirely.

Both sides of 2 and 3 are canonicalised with `pwd -P`: on darwin `/tmp/x` and
`/private/tmp/x` are one directory spelled two ways, and a string compare
refuses a store that is fine.

The two failure directions are opposite and both are deliberate. `--apply`
with no namable owner REFUSES (exit 2) before the first write, because the
only way to make those writes without an owning repo is the flag this exists
to avoid. The dry run writes nothing, so it says what it could not name and
prints its plan anyway.

## The pins

- `TestQAPruneRelatesToAppliesFromTheStoresOwnRepo` — `--apply` invoked from a
  DIFFERENT repo: three bd calls, every one with cwd = the store's own repo,
  none carrying the flag, each carrying `--no-daemon`; the owning store loses
  its pair and the invoking repo's store neither gains rows nor loses its own.
  The fake bd resolves its store from its own cwd, exactly as the real one
  does without a flag, so a wrong chdir fails the assertion AND the script's
  own post-apply re-read.
- `TestQAPruneRelatesToRefusesToApplyWithoutAnOwningRepo` — the refusal, its
  timing (zero bd calls, store unchanged), the dry run still working, and the
  redirect arm from condition 3.
- `TestQANoShippedPosseCallHandsBdAStoreSelectingFlag` (`make corpus-check`) —
  the census that says no other call site came back. Two arms: shipped shell,
  python and Makefile lines that run bd as a command word with the flag on
  them, and non-test `.go` under `cmd/` and `internal/` carrying the flag as a
  PARSED string literal against a roster that says why. Plus `bdGlobalFlags`
  by name, because the roster cannot see a flag that arrives as a variable and
  that slice is prepended to every bd call in the process.

Each was seen red before it was believed (2026-10-09): the original script
reds the census on all three of its lines by file and line; the chdir with the
flag still on reds the flag assertion three times; the flag removed with the
chdir gone reds the cwd assertion three times; condition 3 deleted reds the
redirect arm; a planted `"--db"` literal in a non-test Go file reds arm 2; and
a widened `bdGlobalFlags` reds the by-name arm.

## What this could NOT settle — the defect does not reproduce on this box

MEASURED 2026-10-09, bd 0.50.3 (`bd25acbc`), darwin, nine arms, from a cwd in
one synthetic repo against another's database: a read verb, a write verb, the
daemon path and the `--no-daemon` path, a target database with and without a
sibling `issues.jsonl`, a target inside and outside a `.beads` directory, and
both repos carrying real stores copied from a live `bd init` one. **No arm
imported anything.** The `repo_mtimes` table — the one bd 0.50.3 keeps the
auto-import mtimes in, keyed by `repo_path` with a `jsonl_path` beside it —
stayed EMPTY in every fixture, and is empty in all three of this box's live
stores, none of which holds a row whose id prefix belongs to another repo.

**That negative is not evidence the defect is absent, because the CONTROL
failed.** With a store's own `issues.jsonl` holding a row the database did not
and an mtime set a year ahead, a plain read from inside that repo did not
import it either — in a synthetic store and in a real one. So the auto-import
path never engaged in ANY arm, and these readings say nothing about what it
does when it runs. `no-auto-import` is documented as default-off in the
store's own `config.yaml`, so what gates it here is unknown.

The hardening above does not rest on reproducing it. It rests on the operator's
measurement and on a property that is true either way: a bd call made from
inside the repo that owns its store has no second repository in the question.

What would change the answer: finding the trigger that populates
`repo_mtimes`, and re-running the nine arms with it. That is bd's code, not
posse's, and the ruling says we are not filing it.
