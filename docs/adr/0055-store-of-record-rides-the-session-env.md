# ADR 0055 — The store of record rides the session env: `BEADS_DIR` from `beadsHome`, so the one-graph clause stops depending on bd's mode

*Status: accepted 2026-09-04 · owner: architect · amends ADR 0012 D3-C (a
fourth reader of the redirect: bd itself, through the env) · bead
ranger-base-yijws, discovered from ranger-base-9lrzx*

## Context

rangerhq-09o2's beads clause says a persona in its session worktree reads
and writes the SAME work graph as the main checkout. `worktree.go`'s header
and `worktreelive_test.go` pin that on bd 0.49.1 in database mode, where bd
resolves a linked worktree to the main checkout's `.beads` by itself and
posse's seeded `.beads/redirect` is read by posse (the cage, the seatbelt
writable set, the codex launch line) and not by bd.

The clause is false whenever bd runs in **no-db mode**. Measured 2026-09-04
on bd 0.50.3 (ranger-base-9lrzx by dinesh, re-run for this record in a
scratch repo under `$HOME` with real linked worktrees):

- `bd where` from the worktree answers the main checkout's `.beads` and
  names the redirect (`redirected_from` in `--json`). The create that
  follows lands in the WORKTREE's own `issues.jsonl`, with an id minted
  from the main store's prefix. `bd list` from the worktree shows both
  rows; from the main checkout, only its own. The fork is invisible to a
  read because the worktree's checked-out jsonl carries the main rows by
  construction; only a write tells them apart.
- The same with the worktree carrying no redirect at all: bd's worktree
  resolution still answers the main `.beads`, and the write still lands in
  the worktree's checked-out `.beads/issues.jsonl`.
- The mechanism is in bd's source, not in a version: `cmd/bd/nodb.go`
  (read side) and `main.go`'s `PersistentPostRun` (write-back) resolve
  `$BEADS_DIR`, else `$cwd/.beads`, and never call `FindBeadsDir`, so
  neither the redirect nor the worktree's main repo is consulted. Read in
  0.49.1's source; behaviour measured on 0.50.3. `$cwd`, not the repo root:
  a no-db `bd` run from a subdirectory says "no .beads directory found".

No-db mode has four doors, and posse opens one of them itself:

1. `no-db: true` in the resolved store's `config.yaml` (the store class the
   bead was filed about).
2. `--no-db` on the command line. **The container tier's inner `bd` wrapper
   always passes it** (`CageBdFlags`, cageinner.go, measured 2026-08-22 as
   the only pair that works through the mount). So on that tier the fork
   is not a store-class accident; it is the shipped configuration, on
   EVERY store class. Measured here at the bd level (a `--no-db` create from
   a worktree over a database-class main lands in the worktree jsonl);
   unmeasured in situ, because the container tier cannot run on this box.
3. `BD_NO_DB=true` in the environment (measured: it flips a plain `bd
   create` into no-db mode with nothing in `config.yaml` to see).
4. A bd built without CGO falling back to no-db with a note on stdout
   (the bead's claim; ASSUMED, not re-measured).

A store that merely lacks a database is NOT a door: bd 0.50.3 builds a
SQLite `beads.db` over a bare jsonl on the first plain read (measured), so
"jsonl only" is a transient state, not a mode.

Also measured, and it is the lever: with `BEADS_DIR=<main .beads>` in the
environment, the no-db create from the worktree lands in the MAIN store and
`bd list` from the worktree reads it — on the `no-db: true` store and on the
`--no-db` invocation over a database-class store alike.

## Decision

1. **Every session posse launches carries `BEADS_DIR` in its environment,
   valued at `beadsHome(dir)`** — the store of record posse already
   resolves for the seatbelt writable set, the cage mount and the codex
   launch line (ADR 0012 D3-C). It is set in `planLaunch` beside
   `BD_ACTOR` (herdrback.go), for dispatch and crew sessions on every
   runtime, and `CageEnvNames` forwards it by name into the container. One
   resolver, four consumers; the grant, the mount, the launch line and
   bd's own resolution now come from the same answer.

2. **Set only when the resolved directory exists.** A repo with no `.beads`
   gets no `BEADS_DIR`, for the reason `seedBeadsRedirect` declines to
   write for it: there is nothing to keep unforked, and bd creating a store
   on first write is bd's business, in the directory bd chooses.

3. **No store-class detector, no refusal, no doctor.** Posse does not read
   `config.yaml` for `no-db`, does not grep argv for `--no-db`, and does
   not judge `BD_NO_DB`. The fix is mode-independent, so a detector would
   only add a reader of bd's configuration to posse — a copy of bd's own
   `isNoDbModeConfigured`, covering one door of four, that goes stale on
   its own schedule.

4. **The record says which bd it holds for.** `worktree.go`'s "WHAT WAS
   MEASURED" block gains the no-db bullet above, dated bd 0.50.3, and its
   sentence "bd 0.49.1 does not read the redirect" is qualified to database
   mode. `CageBdFlags`' comment names the consequence of its own flag and
   the env var that closes it. The persona-facing note (AGENTS.md's env
   list) states that `BEADS_DIR` is set, what it names, and that a persona
   which genuinely needs another repo's graph unsets it for that call.

5. **Pins.** Offline: the assembled launch env for a dispatch, a crew
   session and the container render carries `BEADS_DIR` equal to
   `beadsHome(dir)`, and carries none when the repo has no `.beads`. Live
   (`RHQ_LIVE_BD=1`): the no-db fork pin lands with two arms — without
   `BEADS_DIR` the row is in the worktree jsonl (the pin shown able to
   fail), with it the row is in the main store — and the database-class
   pin runs its "filed from the worktree" arm once more with `BEADS_DIR`
   set, so the env is shown not to move the class that already worked.

## Consequences

- The one-graph clause becomes a property of the launch, not of bd's mode
  or the store's class. The container tier's caged `bd` reaches the store
  of record from a session worktree, which the bd-level measurement says it
  does not today.
- A seat's `bd` is pinned to the seat's store for the session, even after
  a `cd` into another repo. A seat works one bead in one repo, and the
  store of record is one per instance, so this is the intended reading;
  the escape is `env -u BEADS_DIR bd …` for the call that needs it. bd's
  redirect detection still runs against the repo-local `.beads` with
  `BEADS_DIR` pre-set (bd-wayc3 in bd's source), so a store reached by
  redirect keeps reporting itself as redirected.
- A store of record under one of bd's unsafe prefixes (`/etc /usr /var
  /root /System /Library /bin /sbin /opt /private`) makes every `bd` call
  in the session refuse, loudly, on its first line: "BEADS_DIR points to
  unsafe location". Today the same store works in database mode. No
  instance repo lives under one (ASSUMED; `~/src` on this box), session
  trees are already refused outside `$HOME` (`WorktreeRoot`), and a loud
  refusal that names the variable is preferred to a launch-time copy of
  bd's prefix list. The day an instance keeps a repo under `/opt`, the fix
  is a config key the instance declares, not a law of the binary.
- The variable is an environment fact and shares that class's exits: a
  runtime `settings.json` env of the same name outranks the launch env
  (measured elsewhere, settings-env-outranks-launcher-env; none exists
  today, and the offline pin reads the assembled env, not the settings),
  and `env -i` sheds it exactly as it sheds `RHQ_PERSONA` (ADR 0025).
- On the database class, bd reaches the same directory through its
  `BEADS_DIR` branch instead of its worktree branch. The write path was
  measured to land in the main store; that the plain-database arm is
  byte-for-byte unchanged is what the re-run pin in D5 establishes.
- The worktree's checked-out `issues.jsonl` stays where it is, inert, for
  the reason worktree.go already gives: dirtying a tree the persona commits
  from is the worse bug. With `BEADS_DIR` set, no-db mode never opens it.

## Alternatives rejected

- **Document it in worktree.go and stop** (the bead's floor). A fork that
  no read can see is found after the loss, and the note is read then. And
  the container tier ships the class, so "easy to acquire by accident"
  understates it.
- **Read `config.yaml` for `no-db: true` at `EnsureSessionTree` and warn
  or refuse** (the bead's ceiling). Catches one door of four — not the one
  posse opens — and copies bd's reader into posse; the 2026-09-04 lesson
  (ranger-base-9307c) is that a property of the reader written into a rule
  names a hole. There is also no `posse doctor` verb to put it in.
- **Drop `--no-db` from the cage wrapper.** Measured 2026-08-22 as the only
  pair that works through the mount; SQLite does not cross the boundary.
- **Delete, back-date or symlink the worktree's `.beads`.** Rejected in
  worktree.go already: every form dirties the tree the persona commits
  from, and a symlink is resolved by bd's `CanonicalizePath` anyway.
- **Use bd's own `bd worktree create`.** Writes a relative redirect that
  mis-resolves by one component (worktree.go header), and creates the tree
  itself, which is posse's.
- **A per-call `bd` shim that injects the variable.** Same value in a
  second place, and a shim reaches only PATH lookups; the env reaches every
  `bd` a script forks, and the cage already has one wrapper.
- **The clever one: make posse the resolver** (a `posse beads where` every
  wrapper consults). A second resolver for a question bd already answers
  from one env var, with the seatbelt and cage still on the first.

## Verification and evidence

MEASURED 2026-09-04, bd 0.50.3, git worktrees under `$HOME`, every call
`--no-daemon`, no `bd init` (store planted by hand; the PID denies the
verb): the no-db fork with and without a redirect; `bd where --json`
naming `redirected_from` while the write lands locally; `BEADS_DIR`
routing the no-db write to the main store on both the `no-db: true` store
and the `--no-db` invocation; `BD_NO_DB=true` as a third door; a plain read
over a bare jsonl building `beads.db`. Read in bd 0.49.1's source:
`nodb.go` and `PersistentPostRun` resolve `BEADS_DIR` else `$cwd/.beads`;
`unsafePrefixes`; `GetRedirectInfo`'s pre-set-`BEADS_DIR` branch.

MEASURED by ranger-base-9lrzx/yijws (dinesh): the database class holds
every one-graph finding on 0.50.3 (the operator's queue is that class).

MEASURED by ranger-base-e3ima (dinesh), which is D5's live half and turns
one of the assumptions below: `worktreelive_test.go` under `RHQ_LIVE_BD=1`
against real linked worktrees, bd 0.50.3, every call `--no-daemon`. All
four cells of D5 item 1 pass — both no-db doors (`no-db: true` in the
store, and `--no-db` over a database-class store), each with the fork arm
and the `BEADS_DIR` arm — and each arm is shown able to fail by mutation
(drop the variable; point it at the worktree; give the fork arm the
variable; drop the `--no-db` flag). The database-class arm re-run under
`BEADS_DIR` (D5 item 2) passes: same rows, same directory by `bd where`,
no staleness warning, no database of the worktree's own. The value in
every arm is `beadsHome(<the session tree>)`, the resolver planLaunch
calls, not a path spelled out in the test (D5 item 3).

Also measured there, and it is why the test guards its own env value: with
`BEADS_DIR` pointed at the WORKTREE's `.beads`, the database-class arm
still lands in the main store — bd's redirect detection runs against a
pre-set `BEADS_DIR` (bd-wayc3, above) and posse's seeded redirect hands it
back. Only the no-db cells catch a wrong value, because no-db mode reads no
redirect at all.

And the boundary check does not bite a temp store: `isPathInSafeBoundary`
allows anything under the resolved `os.TempDir()` before it consults
`unsafePrefixes`, so a `BEADS_DIR` under macOS's `/var/folders` is accepted
even though `/private` is on the list (read in bd 0.49.1's
`internal/beads/context.go`; measured on 0.50.3 for `where` and `create`).
The escape holds only while bd and the test agree on `$TMPDIR`.

ASSUMED: the CGO-less fallback door; no instance store under an unsafe
prefix; the container tier's in-situ behaviour (unrunnable on this
box; the bd-level arm stands in).
