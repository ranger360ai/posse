# ADR 0038 — The cage denies the repo's persistent git identity: `.git/config`, the hook slots, and a worktree's pointer files

*Status: accepted 2026-08-31 · owner: architect · decides ADR 0023
non-goal 3 (flz7 arm a) · extends ADR 0014 §3's trailing-deny slot ·
from ranger-base-7w8g0, on ranger-base-j5s0's measured table*

## Context

Where a session can reach `.git/config` today: a main-checkout session
is granted cwd whole; a PID that denies Edit/Write is granted `cwd/.git`
whole (seatbelt.go, SeatbeltWritable); and when `.beads/redirect` points
at another repo, `beadsGitDirs` grants that repo's `.git` whole. A
worktree session's narrowed grant (ranger-base-m2wf) already excludes
`<common>/config`, but only by omission — no deny stands there if a
grant ever widens.

What a writable config buys: **persistent code execution outside the
cage**. `core.hooksPath`, `core.fsmonitor`, `filter.*.clean`, an alias —
each is a command a *later, unsandboxed* git runs: the operator's daily
git in `~/src/<repo>`, and the launcher's own `git()` calls — including
`git -C <worktree> rebase` at land time (worktree.go:790), which runs
*inside the session's worktree*. That last one extends the surface: a
worktree's `.git` pointer file, and `gitdir`/`commondir`/`config.worktree`
in its own git dir (granted whole), select *which* config and hooks that
unsandboxed git reads. Redirect `commondir` at a session-writable fake
and the config deny is walked around for exactly the git the launcher
runs.

The measured cost of denying all of it (ranger-base-j5s0's table):
**nothing found**. The only in-cage-reachable config/hooks writers are
bd verbs already L1-denied crew-wide; posse's own stamping (`recordBead`
writing `branch.*.posseBase/.posseBead`) runs in the unsandboxed
launcher, untouched by any profile. And the door is genuinely open
today: plain `git config core.hooksPath …` is not a bd verb — no PID
denies that spelling, so where `.git` is writable, any session can plant
the persistent redirect right now.

The hooks dirs are already trailing-denied (`sessionHooksDirs`). ADR
0023 non-goal 3 held that deny to be unsound *alone* — redirect
`core.hooksPath` to a writable dir and plant there. This ADR supplies
the other half.

## Decision

One rule: **the persistent state that tells a later git which code to
run is no session's to write.** Enumerated, at the artifact level, in
`SeatbeltCarveOut.Deny` (last-match-wins, ADR 0014 §3):

1. **The config files, asked of git.** A helper beside `sessionHooksDirs`
   resolves `git rev-parse --git-path config` — asked, never derived,
   the hooksDir doctrine (gates.go:1403) — for both repos a session
   writes: cwd's, and the store of record's when a redirect points
   elsewhere. Deny that file **and its `config.lock` sibling** as
   subpaths (a subpath naming a file denies the file — MEASURED,
   SeatbeltCarveOut doc). Denying the lock too makes the refusal land at
   lock creation: no half-written attempt, and no stray lock in shared
   state — the packed-refs.lock discipline (ranger-base-msex).

2. **A worktree's identity chain, by literal**: `<worktree>/.git` (the
   pointer FILE), and `gitdir`, `commondir`, `config.worktree` in the
   session's own git dir. Write-once at worktree creation; only
   `git worktree move`/`repair` rewrite them, which no session
   legitimately runs. Cost **ASSUMED** zero — not in j5s0's table — so
   implementation measures by execution (a full commit/checkout cycle
   in-cage, and the launcher's rebase path) and drops-and-records any
   literal a legitimate writer turns out to need.

3. **Two payoffs, priced separately.** This deny closes the *persistent*
   redirect: a planted `core.hooksPath` (or fsmonitor, filter, alias)
   surviving the invocation to be read by the operator's git, the next
   launch's probe, or the launcher's land-time git. It does **not**
   touch the *transient* `git -c core.hooksPath=` escape — no file is
   written, there is nothing for a file deny to deny — which stays
   accepted cooperative-class per ADR 0002/0025. Nobody may cite this
   ADR as "the redirect is closed"; the persistent form is.

4. **The L4 twin.** `CageMounts` mounts the common dir read-write whole
   and states the gap. Narrow it the same way: `:ro` file binds of
   `<common>/config` and `<common>/hooks` *after* the common-dir mount
   (later mount wins, the existing carve-out pattern). Bind sources
   exist, so the source-must-exist constraint that blocked narrowing
   the ref grant does not bite here. `cage: shims` has no file boundary
   and stays stated as such (ADR 0002).

5. **The deny is `writable:`-proof, deliberately.** It sits below every
   grant, so a PID extra overlapping it loses (deny-wins, ADR 0001). A
   future legitimate in-cage config writer is a code change with this
   ADR reopened, not a PID knob. `posse gates` prints each entry as an
   `x` line, so the wall stays readable.

No change to `sessionHooksDirs`, `recordBead`, or L1.

## Alternatives rejected

- **Narrow the grant instead of trailing-denying.** The allow block
  cannot say "this directory minus one file", and deny-before-allow
  leaks (MEASURED, ADR 0014 §3). The trailing block is the only slot.
- **Key-granular deny** (only `core.hooksPath`, keep `user.*` open).
  SBPL is path-granular; and no in-cage writer needs partial access, so
  the finer wall would cost complexity to protect nobody.
- **Move posse's stamps out of `.git/config` so the whole file could be
  locked read-only.** The clever one. chmod is a uid promise, not a
  wall; the launcher legitimately writes those stanzas; and the deny
  costs nothing, so relocating the stamps buys nothing.
- **Rely on the L1 bd-verb denies.** Cooperative, argv-shaped, and they
  never covered plain `git config` — the door this closes.
- **Fold in the transient `-c` escape.** Cannot: use-time flag, no
  bytes on disk. Named residual so the under-delivery is loud, not
  quiet (ADR 0025).

## Consequences

- flz7 arm a is decided: hooks-deny + config-deny + identity chain make
  planting containment sound at L2 for every path a later unsandboxed
  git reads. The existing hooks deny stops being the half ADR 0023
  called unsound alone.
- Residual, unchanged: plantings from before the cage existed or from
  other tools, and `cage: shims` sessions. ADR 0023's identity check
  remains the witness; this tier is containment, not the witness.
- `renameSeal` picks up the new paths' ancestors automatically — `.git`
  in a main checkout already gets a seal from the hooks deny.

## Verification (runnable; every refusal pin gets a control arm that
shows the same write landing with the deny removed — the rig must be
shown able to fail)

1. Per session shape (main checkout · worktree · deniesFiles ·
   redirect): in-cage `git config core.hooksPath /tmp/x` → rc≠0,
   config byte-identical, **no stray `config.lock`**.
2. Non-git spellings refused: shell redirect, python open, `mv` onto
   `config`.
3. Session life stays green under the deny: add/commit, checkout,
   bd claim/comment/close staging `.beads` — the zero-cost claim
   measured by execution, not asserted.
4. Identity chain: writes to `<worktree>/.git`, `gitdir`, `commondir`,
   `config.worktree` refused; launcher-side `git -C <worktree>
   rev-parse`/`rebase` unaffected.
5. L4: the file `:ro` binds measured on the container engine — write
   through the rw common mount refused on exactly those two paths.
