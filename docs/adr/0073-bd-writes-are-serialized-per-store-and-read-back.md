# ADR 0073 — bd writes take one flock per store, and posse reads its own writes back: no-db bd loses concurrent writes and prints success, and the rig shows it

*Status: accepted 2026-10-12 (ranger-base-wnjqs, from
github.com/ranger360ai/posse/issues/13, operator, work box, 2026-10-11) ·
owner: architect · the write-side sibling of ADR 0071 (an answer posse
cannot trust is refused, not served); the lock is ADR 0011 §1's flock
applied to a third resource; the shim arm sits beside ADR 0069 and ADR 0072
D2 · builds in the code beads named on ranger-base-wnjqs (dinesh)*

> The issue's cause is now known: concurrency, not description length. In
> no-db mode bd 0.50.3 reads the whole jsonl, changes it in memory and
> writes the whole file back, with no lock. Two writers that overlap print
> the same sequential id, and the last one to write wins. Both exit 0. That
> is a **lost update**, and the field's answer to a lost update is a single
> writer. Posse gives every bd write on the box one flock per store, through
> the shim for personas and through `Bd` for itself. It then reads its own
> writes back, because some writers never take the lock. Personas are told
> to do the same.

## Context (MEASURED 2026-10-12, bd 0.50.3 `bd25acbc`, darwin 25.4.0, crew seat, every call `--no-daemon`; hand-written rigs per ADR 0072, no `bd init`; 4 writers × 12 writes, each writer in its own linked worktree with `BEADS_DIR` set to the main rig's `.beads` the way a seat is bound (ADR 0055); probe `0073-bd-write-readback.probe.sh`, about 22 s wall, run 4 times)

| arm | no-db store | SQLite store |
|---|---|---|
| serial control, 12 creates | 12/12 land | 12/12 land |
| concurrent creates, exit 0 + `Created issue: <id>` printed | 48/48 | 48/48 |
| …of which the record is at the printed id | **12–14/48** (6 runs: 36, 35, 34, 34, 36, 34 lost) | 48/48 |
| …duplicate printed ids | 34–36 (`t-15` printed by three writers) | 0 |
| concurrent `comments add` on one issue, exit 0 | 48/48 | 48/48 |
| …present afterwards | **17, 23, 12** of 48 | 48, 48, 48 |
| `close` during the comment storm: `✓ Closed t-1: Closed`, exit 0 | status read back `in_progress` in **1 of 3** | closed 3/3 |
| `close` during the create storm | closed 4/4 | closed 4/4 |

- **All three of the issue's shapes are this.** Shape 1 is the duplicate
  id. The issue's `<prefix>-93` is a sequential id, and only no-db mode
  hands those out (the SQLite rig handed out hash ids such as `t-bz3`), so
  the work-box store is no-db. INFERRED, not measured on that box. The
  closed-but-`IN_PROGRESS` bead is a rewrite from a writer that loaded the
  file before the close landed.
- **Length is not the mechanism.** Creates with 20,000-, 60,000- and
  70,000-byte descriptions all landed and printed an id, in both modes. At
  70,000 bytes the no-db store became unreadable, which is ADR 0072's cliff
  and not this issue. Shape 2 (field echoes, no id, exit 0) was **not
  reproduced**. Given the evidence above, it is ASSUMED to be the same lost
  update reached by the human output path.
- **Mixed modes are worse, and posse ships a mixed mode.** Cage seats run
  `bd --no-db` always (`CageBdFlags`), including against a store that the
  host side runs in SQLite mode. In the rig (a SQLite store, 2 writers on
  SQLite and 2 on `--no-db`), all 48 writes exited 0. SQLite never saw the
  24 no-db creates, and 12 of those 24 survived in the jsonl. Then one
  `bd sync --flush-only` from the SQLite side rewrote the jsonl from the
  database, and **0 of the 24** remained. That was one run. The container
  tier cannot be run on this box (cageinner.go), so how often this happens
  live is UNMEASURED.
- **posse's own writers are in the race.** These are `Bd.Create` (verify
  batches, merge-blocked and closed-dirty beads), `Bd.Comment` (close and
  settle comments, ciwatch, prompt-stall), `Bd.Close`, `Bd.SetDescription`,
  `Bd.DepAdd` (settle-open escalation) and `Bd.Unclaim`. Today each trusts
  bd's exit code. `Bd.Claim` is the only one that already reads back
  (rangerhq-kux). Dispatch writes while several seats write, which is
  exactly the rig.

## Decision

**D1 — one flock per store, for every bd write on the box.** The lock file
is `~/.cache/posse/bd-write/<sha256(realpath(store))[:16]>.lock`.
- The store is the one bd will use: `$BEADS_DIR`, otherwise
  `beadsHome(cwd)`. This is ADR 0072 D2's resolution, with the realpath
  taken so that a redirect and a direct spelling share one lock.
- Held by `flock(2)`, in the shape of `launchlock.go` and `suite-lock.sh`.
  The kernel releases it when the holder dies, it creates no new state,
  and it needs no reaper (ADR 0011 §1).
- The lock is taken for every verb that is **not a known read**: `show`,
  `list`, `ready`, `blocked`, `info`, `where`, `comments` with no
  subverb, `dep list`/`tree`, `stats`, `search`, `count`, `version`,
  `help`, `prime`, `onboard`. Classifying this way fails safe: a verb
  nobody listed is serialized, and costs only a wait.
- It takes the lock in **both** store modes. That keeps it clear of
  ADR 0055 D3, which forbids posse from reading bd's mode. In SQLite mode
  it duplicates a lock bd already holds, which costs nothing.
- **The wait is bounded at 30 s** (ASSUMED, the same as `Bd`'s timeout).
  If the wait runs out, the write is refused, exit 1, and bd is not run:
  `refused by posse gate: <store>'s bd write lock was held for 30s — a
  bd write is still running against it; nothing was written (ADR 0073)`.
  A refusal the writer can see is the entire point of the issue, so it
  must not become a silent loss. After it acquires the lock, the holder
  writes its pid and argv[0..2] into the lock file. That is for reading
  only: an flock records no holder (ADR 0059 D3).
- If the lock dir cannot be opened (a seat whose wall does not name
  `~/.cache/posse`, ranger-base-r3czg), posse prints one stderr line and
  runs the write unlocked, the way `suite-lock.sh` does. D3 still runs
  for posse's own writes.

**D2 — the persona half is a shim arm, and it carries ADR 0072's judge.**
For the write verbs, the bd shim's final `exec` becomes
`exec <posse> gates bd-write --real <bd> -- "$@"`. That subcommand:
1. takes D1's lock on an fd without `FD_CLOEXEC`;
2. runs ADR 0072's `ProjectBdWrite` judge **under the lock**, so the
   census line it projects from cannot change under it (closing a TOCTOU
   that 0072's arm has today);
3. calls `syscall.Exec` on bd. bd inherits the lock's open file
   description, so the lock is held exactly as long as bd runs.

This replaces the arm's separate `gates bd-line` call. Reads still pay
nothing. If posse is missing, the arm stays fail-open, as in ADR 0072 D2.

**D3 — posse's own writes take the lock and read the record back.**
`Bd.runOnce` takes D1's lock for write verbs, and so does `Bd.run`'s
`sync --import-only` self-heal, since it is a write. Each write method
then checks what it wrote:

| method | read-back | landed when |
|---|---|---|
| `Create` | `Show(id)` | the id resolves to itself **and** `Title` equals what was filed |
| `Close` | `Show(id)` | `Status == "closed"` |
| `Comment` | `Comments(id)` | a comment with exactly that text (and author, when an actor is set) whose `created_at` is not before the write started, truncated to the second |
| `SetDescription` | `Show(id)` | `Description` equals what was written |
| `DepAdd` | `DepList(id)` | blocker present (an existing edge counts as landed) |
| `Unclaim` | `Show(id)` | `Status == "open"` |

A mismatch returns `BdWriteNotLandedError{Verb, ID, Want, Got}`. If the
read-back itself cannot be done, the method returns
`BdWriteUnverifiedError` wrapping the read's error. Both work with
`errors.As`, in the `BdStoreUnreadError` family. **posse never retries**,
because a write whose read-back failed may still have landed. Callers keep
their existing error paths (ADR 0071 D5). B3 checks each one and names any
that swallow the error. Cost: one bd read per posse write, about 0.3 s
(ADR 0071, MEASURED). posse writes a few times per pass.

**D4 — the persona rule ships in posse, not in one box's ORDERS.**
The work prompt reaches every dispatched seat on every instance, so it gets
one paragraph, added beside Provenance in `dispatch.go`:

*"A bd write's exit 0 and its printed line are not the record. Read every
write back: `bd show <id>` after a create, close or update, `bd comments
<id>` after a comment, `bd dep list <id>` after an edge. If the read-back
is missing the write, look for it by title in `bd list` before you file
again. Re-file only when both reads say it is absent, and never on an
exit code alone."*

The **Done** sections of the seed PIDs (`examples/agents/*.md`) and
`scaffoldPID` get the same rule as one clause: *"then `bd show <id>` reads
`closed`"*. The rule is also added to this repo's AGENTS.md, under "Landing
the plane".

**D5 — the rig is committed and is the acceptance test.**
`0073-bd-write-readback.probe.sh` reproduces the table above. B2's
acceptance is an extra probe arm that routes its writers through
`posse gates bd-write`, and that arm shows **0 lost** in no-db mode over
3 runs.

## Consequences

- With a lock in place, concurrent writes on this box are serialized:
  4 seats × about 0.3 s per write. Contention is ASSUMED negligible at
  about 5 seats. If it ever matters, the 30 s refusal is what makes it
  visible.
- **The lock does not reach** writes the operator types by hand outside a
  shim, cage seats (flock across a Docker Desktop bind mount is ASSUMED
  not to reach host processes), or a second box writing the same store
  through git. D3 and D4 are the backstop for those, and they detect a
  loss rather than prevent one. The mixed-mode erasure is a design
  problem of its own, filed as its own bead (see the bead comment).
- The exit hatch: remove the arm and the `runOnce` lock. Nothing is lost,
  because the lock files hold nothing. The day bd serializes its own no-db
  writes, D1 is redundant and can go. D3 and D4 should stay.
- Reads stay unlocked. Whether a no-db write is an atomic rename, or
  whether a reader can see a half-written file, was **not measured**. If
  it is not atomic, that becomes ADR 0071's empty-answer case and is
  already refused there.

## Alternatives rejected

- **Read-back only, which is what the bead literally asked for.** It turns
  a silent loss into a reported one for posse's writes and prevents
  nothing. A seat whose create was clobbered half a second after its
  read-back still believes in it. With 36 of 48 writes lost, reporting
  alone leaves a store that is mostly loss.
- **Refuse no-db stores, or move every store to SQLite.** That needs a
  mode detector (ADR 0055 D3 refused one). Cage seats are no-db because
  they must be, and the rig shows mixed mode erasing more than no-db alone.
- **bd's daemon as the serializer.** `--no-daemon` is pinned everywhere
  (ADR 0056 D1), and the daemon's socket cannot cross the cage
  (rangerhq-6so).
- **A posse single-writer daemon that every bd write queues through.**
  This is the clever one, and the one I wanted: one owner, no contention,
  and the cage could reach it. It would own a process, its liveness and a
  protocol, and ADR 0011 rejected the same shape for that reason. A
  ~40-line flock buys the single-writer property for every writer that
  can see the file.
- **Put the lock file in `.beads/`.** That is a git-tracked directory. It
  would only help the cage, and only if flock crossed the VM boundary,
  which is ASSUMED it does not. `~/.cache/posse` is where the box's other
  slot locks live.
- **Retry a write that did not land.** If the read-back is wrong (an
  unreadable store, or a lagging import), a retry files a duplicate. Under
  a lost update, a retry races again. posse reports; the writer decides.
- **Only lock the verbs ADR 0072's arm already routes** (`comments`,
  `update`, `create`, `close`). That leaves `dep add`, `reopen`, `label`
  and `sync` in the race. Locking everything except a known read is the
  safe direction.
