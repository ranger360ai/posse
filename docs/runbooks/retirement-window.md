# Runbook — the retirement window: the order the other two run in

*ranger-base-3rv9 step 4 · the driver for `queue-cutover.md` and
`home-cutover.md`, which interleave and cannot be run back to back ·
ordering and preconditions measured 2026-08-27 on ranger-base-6y83 ·
**run for real 2026-08-28**, and corrected from what that run measured —
ranger-base-j2io*

**The window has been executed.** All three files are kept, and are kept
in the imperative, because the sequence is re-runnable: the rollbacks at
the end of both halves walk it backwards, and a second instance walks it
forwards. The `2026-08-27` preconditions table at the foot of this file is
a record of that one run's starting state, not a claim about today's.
Everything marked **MEASURED 2026-08-28** is errata from the live run —
three lines in these runbooks were wrong, and they are corrected in place
rather than appended, so what you read is what to type.

The two halves of the window are written as separate runbooks because
they have separate subjects — the beads store moves repos, the home
leaves the symlink. They do **not** execute separately. `home-cutover.md`
says why in its own preamble: the binary prefers `~/.config/posse` the
moment that directory exists, so the home sequence has to happen inside
the quiet the queue half already asks for. This file owns the single
order, and the gate that has to pass before any of it starts.

It adds no steps of its own. Every step below is a pointer; the detail,
the exact commands and the rollback all stay in the two runbooks.

**Who runs it.** The operator, and not by convention — by construction,
at four separate points:

| step | what refuses a persona | where |
|---|---|---|
| queue 1, 7 | `bd daemon stop` / `start` | denied to personas — **singular spelling only**, see below |
| queue 4 | `bd migrate --update-repo-id` | denied to personas |
| home 3, 8 | `posse promote` | `internal/rhq/promote.go` — refuses under `RHQ_PERSONA` (ADR 0015 §3) |
| pre-window | `make install` | denied to personas |

One of those four is weaker than it reads. The deny is `Bash(bd daemon:*)`,
a prefix match on the typed word, and bd 0.49.1 also answers to `bd
daemons` — `bd daemons list` runs from a persona session today, measured
2026-08-29, and so would `bd daemons stop`/`killall` (filed as
ranger-base-llp1). Queue step 1 now types the plural because that is the
form that works, not because it is fenced. The other three rows hold.

A persona session that reaches this file should stop here and say so.

## The gate: two things before the window, neither in either runbook

**P1 — the build must already have stopped writing the `rhq` alias.**
Done: ranger-base-igup, on `main` at `10cc44f`. Neither `make install`
nor `make link-plugin` writes an `rhq` inode any more, so the two that
predate that change can be deleted in the window and stay deleted. The
retirement of the inodes themselves is step 6 below; the Makefile
comments at `install:` and `link-plugin:` record the measurement that
they have zero consumers.

**P2 — one commit in the constitution, in `rhq/config.yaml` and
`rhq/agents/monica.md`.** Both are **promoted paths**, and promote
refuses on a dirty promoted path, so this cannot be deferred into the
window:

- **add `constitution: ~/src/ranger-base/rhq`** — absent today. Without
  it the constitution path lives only in `promoted.json`, and a home
  rebuilt from scratch does not know where to promote from.
  `home-cutover.md` step 3 asks for it in so many words.
- **four stale command spellings in comments**, at `config.yaml` lines
  **1, 3, 14, 113** — these say `rhq <verb>` where the command is now
  `posse`. Promote copies comments verbatim into the new home, so a
  stale spelling ships. The other `rhq` hits in that file are **repo
  paths and bead ids** (`~/src/rangerhq`, `rangerhq-*`) and are correct
  as they stand — do not sweep them.
- **drop `Bash(rhq:*)` from `rhq/agents/monica.md:9`**, the only PID
  that carries it. After P1 the alias is command-not-found, so it is
  dead text — but it is an explicit *allow* on the spelling that walks
  past the fence step 8 ratifies, written into the file the fence is
  written into.

## The order

Steps 1–3 and 7 are `queue-cutover.md`; 4–6, 8 and 10 are
`home-cutover.md`, and step 4 carries one queue step with it. Both
runbooks' own numbering is kept, so a step here names a step there —
including `home-cutover.md`'s renumbering of its own steps 2–5, which the
window's measured order forced (ranger-base-j2io).

| # | runbook | step |
|---|---|---|
| 0 | — | **F9** (archive `$OLD`) — the operator's, and the one step that ends rollback. It has no ordering dependency on the window. Running it *after* step 9 verifies is the cheaper order; see the constitution's own `0012-cutover.md`. |
| 1 | queue | 1–2 · stop the daemon; quiesce the fleet |
| 2 | queue | 3 · `scripts/queue-cutover.sh` |
| 3 | queue | 4 · `bd migrate --update-repo-id` — **not optional**, and the step to watch: the repo-id mismatch arms bd's git-history backfill to delete issues, and it fails the daemon closed until it is run |
| 4 | home | 1–2 · preconditions; `rm ~/.config/rhq` — a symlink; the tree it points at is untouched. **Before the promote, not after**: promote refuses onto a symlinked home, and until `~/.config/posse` exists the symlink *is* the home (ranger-base-j2io, measured at the window) · then queue **5** · the two config edits — typed in the **constitution**, `rhq/config.yaml`, and committed, so the promote in the next row carries them and the promoted home never drifts |
| 5 | home | 3–5 · promote; carry envs (`umask 077`, `cp -p`); carry state; link personas |
| 6 | — | **the two alias inodes**: `rm ~/.local/bin/rhq` and `rm ~/src/posse/plugin/bin/rhq`. **Before step 8, not after** — while either exists the fence step 8 ratifies is defeated by typing the old spelling, because PID rules match the typed word |
| 7 | queue | 6–8 · install gate hooks in the queue repo (**queue 5's visibility stamp landed at step 4, and must be in force before this**); restart the daemon there and confirm with `bd daemons list`; commit the constitution's staged untracking |
| 8 | home | 8 · `scripts/draft-pid-deny-promote.sh`, read the diff, commit, `posse promote` — the first change put through the step ADR 0015 adds |
| 9 | both | home 6 + queue 9 · **verify** (ADR 0015 verification items 1, 6, 7) |
| 10 | home | 7 · **only after 9 passes** · delete the env values from the constitution tree |

**Rollback is cheap through step 9** and is written out at the end of
both runbooks — nothing is destroyed before then: the queue store is
moved rather than copied, and the constitution's `.beads` untracking is
staged rather than committed. Step 10 ends that, and F9 ends it
entirely.

## Preconditions, measured 2026-08-27

Everything the two runbooks ask for before the window, re-measured on
ranger-base-6y83 after igup landed:

| | |
|---|---|
| installed binary | `~/.local/bin/posse` = `0.3.0+7c16c6b`, carrying promote, the seatbelt, and `queue_repo:` support (`internal/rhq/queuejsonl.go` present at that sha) |
| `posse promote --help` | answers |
| home | `~/.config/posse` does not exist; `~/.config/rhq` is still a **symlink** to `~/src/ranger-base/rhq` |
| env sets | 4 files at `rhq/envs/`, `drwx------` / `-rw-------` — one is empty and is carried anyway, it is a name a recipe may reference |
| alias inodes | both still present, both now build-inert (P1) |
| watch loop | armed; disarm at step 1 |
| queue repo | `~/src/ranger-queue` does not exist — the cutover script refuses if it does |
| `$OLD` | `~/src/rangerhq` present, in sync with its remote, so F9 archives nothing unpushed |

**Not clear as of this writing:** `rhq/config.yaml` is dirty, which
fails promote's clean gate. The gate's pathspec is
`rhq/agents rhq/config.yaml rhq/recipes rhq/skills` and nothing else —
`.beads` and `personas/` are dirty in that repo essentially always and
are reported without blocking (ADR 0015 §4/§5). Check it immediately
before the window, not from this file:

```sh
git -C ~/src/ranger-base status --porcelain -- rhq/agents rhq/config.yaml rhq/recipes rhq/skills
```

That check must come back empty. P2 is a commit against two of those
paths, so P2 is what clears it — but the working tree may also hold
edits from other sessions in the same files, and a path-limited commit
takes working-tree content. Read the diff before committing it: the
commit that opens the window should be one the operator meant to make.
