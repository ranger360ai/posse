# ADR 0038 — The cage denies the repo's persistent git identity: `.git/config`, the hook slots, and a worktree's pointer files

*Status: accepted 2026-08-31 · amended 2026-09-02 (ranger-base-xwepd:
decision 1's "no stray lock" clause RETRACTED as measured false; the
`config.worktree.lock` sibling added to decision 2) · amended 2026-09-05
(ranger-base-huhnw: decision 4 restated to the mechanism ranger-base-t4f1
builds — a `:ro` common dir with three read-write overlays and a detached
HEAD, in place of two `:ro` file binds that were never built; verification
item 5 folded into t4f1's engine arm, UNRUN; two L4 residuals named) ·
amended 2026-09-05 (ranger-base-n3ywd: decision 4 gains the two L4
twins those residuals asked for — `:ro` FILE binds of the identity files
whose source the launcher owns, the lock siblings deliberately unbound;
verification item 6) · lands with ranger-base-t4f1 (decision 4's common
dir) and its successors ranger-base-672zt → ranger-base-p9h9d →
ranger-base-017dx (decision 4's file binds and the engine arm) · owner:
architect ·
decides ADR 0023 non-goal 3 (flz7 arm a) · extends ADR 0014 §3's
trailing-deny slot · from ranger-base-7w8g0, on ranger-base-j5s0's
measured table*

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
   SeatbeltCarveOut doc). Denying the lock too moves the refusal to
   **lock creation**: git says "could not lock config file" and nothing
   of the attempted config reaches the disk. Without the sibling it says
   "could not write config file" — it already holds the lock with the
   whole new config in it and fails one step later. That word is the
   difference the entry makes, and it is what the pin asserts.

   This clause used to end "and no stray lock in shared state — the
   packed-refs.lock discipline (ranger-base-msex)". **That half was
   wrong** and is retracted (ranger-base-xwepd): `git config` on git
   2.50.1 removes its own lock when the rename fails, so no stray lock
   is left with the sibling denied *or* allowed — MEASURED in both arms.
   The packed-refs.lock discipline is still why a stray lock would
   matter; it is not what this entry buys.

2. **A worktree's identity chain, by literal**: `<worktree>/.git` (the
   pointer FILE), and `gitdir`, `commondir`, `config.worktree` and its
   `config.worktree.lock` sibling in the session's own git dir. The lock
   sibling is decision 1's reasoning applied where it had been left out
   (ranger-base-xwepd): git writes `config.worktree` through a lockfile
   too, so without it a refused `git config --worktree` landed at "could
   not write" with the attacker's whole file already on disk in the lock
   (MEASURED, git 2.50.1). Write-once at worktree creation; only
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

4. **The L4 twin** (restated 2026-09-05, ranger-base-huhnw). As first
   written this item prescribed two `:ro` file binds of `<common>/config`
   and `<common>/hooks` over a read-write common-dir mount. Neither
   sentence describes a mechanism that ships: the build bead
   (ranger-base-mugt2) was folded into ranger-base-t4f1 (groom
   ranger-base-kcnc6, operator-confirmed 2026-09-01 on ranger-base-vzgk9),
   whose mechanism is stronger and subsumes it. **The decision is t4f1's
   mechanism.** For a worktree session `CageMounts` mounts the git common
   dir `:ro` whole and overlays read-write exactly the three regions a
   detached-HEAD commit writes — `worktrees/<own>`, `objects`, `logs`
   (`sessionCommonDirWrites`, which is L2's `sessionGitGrants` in the
   shape a mount list can say) — and the launcher puts the caged session
   on a **detached HEAD** at the branch tip before launch
   (`PrepareSessionHead`) and splices the work onto the branch at close
   (`spliceDetachedWork`, the `git branch -f` landed() already
   prescribed). `config` and `hooks` are inside the `:ro` mount and under
   none of the three overlays, so this item's outcome holds without naming
   either path — and so do `packed-refs`, `refs/` and every other
   session's `worktrees/<name>`, which the two file binds would have left
   writable. The worktree shape at L4 is thereby *narrower* than L2's
   grant, not wider: no ref write at all.

   Three things the original text had wrong or missing. (a) The
   source-must-exist constraint bites `logs`, not the config/hooks binds:
   a repo that has never moved a shared ref has no `logs/`, a read-write
   overlay of an absent source is Stat-dropped, so the launcher makes it
   before launch. (b) The ref pair L2 grants is not expressible as a mount
   at all — a pre-created `.lock` fails every commit and rename(2) cannot
   replace a bind mountpoint (assessed on ranger-base-6q5e) — and the
   detached HEAD is what makes *not* granting it survivable. (c) This is
   worktree-only by construction: `CageMounts` adds the common-dir mount
   only when the common dir is outside the tree (pinned,
   cageinner_test.go), so the main-checkout shape gets nothing from it —
   see the residual below. `cage: shims` has no file boundary and stays
   stated as such (ADR 0002).

   Lands with ranger-base-t4f1, in flight at this amendment: until it
   lands, `cage.go` mounts the common dir read-write whole with the gap
   stated in its comment — which is the shipped state this item's
   original first sentence described. The bead id is the citation, not a
   session sha (ADR 0051).

   **The two L4 twins the common dir does not supply** (2026-09-05,
   ranger-base-n3ywd, from the residuals ranger-base-huhnw named). One
   mechanism for both, and it is the one this item originally prescribed
   in the wrong place: a **`:ro` FILE bind, placed by `cageOverlay`'s deny
   direction over whatever read-write mount covers the file** — the
   engine sorts binds by destination depth, so the file bind wins over the
   directory it sits in (MEASURED for directories, 7/7,
   `0014-path-scoped-writes.probe.sh`; ASSUMED for a file source until
   arm B4, item 6). Two rules decide which files, and they are the
   whole design:

   *(a) Bind only what the launcher owns and exists.* A `-v` bind whose
   source is absent is not refused by the engine: it **creates the source
   on the host, as a directory** (NOTES, probe 7 — the property the
   directory overlays rely on). For a file that is the wrong artifact in
   the operator's git dir: a `config.lock` directory makes the operator's
   every `git config` fail "could not lock config file: File exists", and a
   `config.worktree` directory makes every git command in that tree fatal
   ("unknown error occurred while reading the configuration files") —
   both MEASURED 2026-09-05, git 2.50.1. So the deny direction takes a
   Stat for FILE sources, the one place it does, and **no `.lock` sibling
   is ever bound**: decisions 1 and 2's lock entries have no L4 twin. What
   that costs is wording, not containment: the refusal lands at the
   rename(2) onto the mountpoint instead of at lock creation, git says
   "could not write config file" and removes its own lock (MEASURED at L2,
   xwepd), and nothing of the attempted config reaches `config`. A stray
   lock left by hand is a nuisance any session under a read-write `.git`
   could already leave, and is not a code-execution route.

   *(b) Same readers as L2.* The paths come from `sessionGitConfigFiles`,
   `sessionHooksDirs` and `sessionWorktreeIdentityFiles` (seatbelt.go),
   minus their lock siblings — asked of git through `gitPath`, never
   joined — because a wall that reads the PID or the repo differently from
   the wall beside it is the classification error ADR 0014 exists to
   prevent (ranger-base-4ks).

   **4a. The main-checkout shape** (ranger-base-672zt): `.git/config` gets
   a `:ro` file bind and `.git/hooks` a `:ro` directory overlay — the
   hooks half ranger-base-3c3/h15 named and cage.go:356 deferred — over
   the read-write repo mount, or over the read-write `.git` carve-out on a
   `:ro` repo (ADR 0014 §4). What a caged main-checkout session
   legitimately writes in `.git` (index, `HEAD`, `ORIG_HEAD`, its refs and
   their locks, `logs`, `objects`, `COMMIT_EDITMSG`, `packed-refs`) is
   untouched. Cost MEASURED zero in this exact shape at L2 (j5s0's table;
   item 3's session-life pins), and L4 adds no writer. The store of record
   behind a redirect needs nothing here: at L4 only its `.beads` is
   mounted, its `.git` is invisible, and the deny direction of
   `cageOverlay` mounts nothing where nothing covers the path — invisible
   is stronger than `:ro`. In the worktree shape the same call resolves to
   `<common>/config` and `<common>/hooks`, finds them under the `:ro`
   common mount, and `cageOverlay`'s same-mode rule appends **no bind** —
   so t4f1's "no mount on `config`/`hooks`" pin stays true and the
   mechanism is shape-agnostic by construction, not by a branch.

   **4b. Decision 2's identity chain** (ranger-base-p9h9d): `:ro` file
   binds of the `<worktree>/.git` pointer file and of `gitdir`,
   `commondir` and `config.worktree` in `worktrees/<own>`, over t4f1's
   read-write overlay of that directory. Why these four: after a whole
   session's life `worktrees/<own>` holds `COMMIT_EDITMSG`, `commondir`,
   `gitdir`, `HEAD`, `index`, `logs`, `ORIG_HEAD`, `refs` (MEASURED), and
   only the pointer, `commondir` and `config.worktree` select which
   config and hooks the launcher's unsandboxed land-time git reads —
   a pointer rewritten at a fake git dir carrying its own `commondir`
   redirects `--git-path hooks` at a planted `core.hooksPath` (MEASURED)
   — while `gitdir` is what `git worktree prune` trusts, so a session
   that rewrites it can have the launcher's prune delete its own git dir.
   `config.worktree` is the one source the launcher has to **make**:
   posse never sets `extensions.worktreeConfig`, no live worktree carries
   the file, and with the extension off git never reads it (MEASURED —
   a planted `core.hooksPath` in it changed nothing; `git config
   --worktree` itself refuses without the extension). So the extension in
   the `:ro` common `config` is what keeps the file inert, and the bind is
   for the repo where the operator turned the extension on. Rather than
   condition the wall on a config key, `PrepareSessionHead` creates an
   EMPTY `config.worktree` beside its `logs/` mkdir before every caged
   launch and the bind is unconditional; an empty file carries no keys
   either way. The pointer file lands a bind only on a read-write repo —
   on a `:ro` one it is already inside the boundary and the same-mode
   rule mounts nothing. Cost MEASURED zero: L2's identity pin already
   shows all five files byte-, inode- and mtime-identical across a
   session's life and the land-time rebase.

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
- **Keep decision 4's two file binds on top of the `:ro` common dir**
  (belt and braces, 2026-09-05). A `:ro` bind of a path the deepest
  covering mount already holds `:ro` is not an addition — `cageOverlay`
  flips the covering mount's mode, and two binds at one destination is an
  engine error — and the pin asserts that NO mount of either mode lands
  on `config`/`hooks`/`packed-refs`/`refs`, because a mount of its own
  there is the shape a future widening would take. The binds would fail
  the pin and buy nothing.
- **Pre-create the lock siblings so they can be bound** (2026-09-05).
  The move that gives `config.worktree` a source, applied to
  `config.lock`. Dead by git's lock protocol: a pre-existing lock fails
  the operator's every `git config` "File exists" (MEASURED), which is the
  denial of service the sibling entry exists to avoid, and rename(2)
  cannot replace a bind mountpoint — the same reason 6q5e rejected
  binding `refs/heads/<branch>.lock`. So the L4 twin of a lock entry is
  no entry, said out loud.
- **Bind `config.worktree` only when `extensions.worktreeConfig` is on,
  and skip the create** (2026-09-05). Fewer files touched, and a wall
  conditional on a key read at render time: turn the extension on in the
  `:ro` common config from the host between render and launch (the
  operator's own git, a tool, a later posse) and the chain reopens with
  `posse gates` still printing the bind. A wall that disappears with a
  config key is the wall probe 7 was written against. One empty file per
  session in a directory posse already owns and prunes is the cheaper
  side.
- **Check the chain at land time instead of binding it** — the launcher
  reads the pointer, `commondir` and `gitdir` back against what it wrote
  at `worktree add` and refuses to run git in a tree whose chain moved
  (2026-09-05). Engine-independent and unsandboxed, so it cannot be fooled
  from inside. But it is a WITNESS, and this ADR's Consequences already
  say what this tier is: containment; ADR 0023 owns the witness. It also
  protects only the launcher's git — the pointer redirect is equally an
  escape for the operator's next `git -C <tree>` — and it lands after the
  session has already written the redirect it then has to explain. Not
  rejected as wrong, rejected as the other lane's: if 0023's identity
  check grows the chain, it should, and nothing here conflicts.
- **Wait for t4f1 to land before amending** (2026-09-05). The record
  would keep prescribing a mechanism the operator retired on 09-01, to
  whoever reads it for the next cut; ADR 0040's `lands with: <bead>`
  status form exists for exactly this gap, and the bead id survives the
  rebase a sha does not (ADR 0051).

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
- L4 (2026-09-05): containment for the *worktree* shape is the `:ro`
  common dir, narrower than L2. The main-checkout shape and the identity
  chain get `:ro` file binds (decision 4a/4b, ranger-base-672zt and
  p9h9d), so the rendered set at L4 names every path L2's deny block
  names EXCEPT the three lock siblings, which have no mount twin. Until
  those two beads land and arm B4 has run somewhere with an engine,
  nobody may cite this ADR as "L4 is closed"; after them, the honest
  sentence is "L4 is closed except at the lock siblings, where the
  refusal lands one step later and leaves nothing behind".
- An L4 file bind is a bind of a source the launcher OWNS: it exists
  before launch, or `PrepareSessionHead` makes it. The deny direction of
  `cageOverlay` never binds an absent file, because the engine would make
  it a directory on the host (decision 4a) — the one asymmetry between a
  file rule and a directory rule at this tier, and the reason the lock
  siblings stop at L2.

## Verification (runnable; every refusal pin gets a control arm that
shows the same write landing with the deny removed — the rig must be
shown able to fail)

One shape cannot have that control arm, and saying so is part of the
claim (measured ranger-base-1fz21, 2026-09-02): in the **worktree**
shape no grant reaches `<common>/config` at all, so the write is refused
with the deny removed too. Items 1 and 2 there grade the omission wall
this ADR's Context names, not the deny — `reachesConfig` in
`seatbeltgitidentity_qa_test.go` asks the production grant which of the
two a row is grading, and asserts it the other way round so the day a
grant widens the row fails and goes back to grading the deny. The other
three shapes carry the real control arm.

1. Per session shape (main checkout · worktree · deniesFiles ·
   redirect): in-cage `git config core.hooksPath /tmp/x` → rc≠0,
   config byte-identical, and the refusal's own words are **"could not
   lock config file"** and not "could not write" — the difference the
   `.lock` sibling makes, graded against a control that takes back out
   only that sibling. "No stray `config.lock`" is asserted in BOTH arms
   as a regression guard; it is NOT the deny's witness, because it is
   green with the sibling gone (ranger-base-xwepd).
2. Non-git spellings refused: shell redirect, python open, `mv` onto
   `config`. The `mv` stages its forged file in the persona's memory
   dir, not the gates dir: `state/gates` is denied in both arms, and a
   probe refused at its own setup exits like one refused at the wall —
   which is how that row asserted nothing in all four shapes until
   ranger-base-1fz21.
3. Session life stays green under the deny: add/commit, checkout,
   bd claim/comment/close staging `.beads` — the zero-cost claim
   measured by execution, not asserted.
4. Identity chain: writes to `<worktree>/.git`, `gitdir`, `commondir`,
   `config.worktree` and `config.worktree.lock` refused; launcher-side
   `git -C <worktree> rev-parse`/`rebase` unaffected. The lock sibling's
   cost is measured with the rest: it stays ABSENT across a whole
   session's life and the launcher's rebase, which is the reading that
   would have to change before the literal could be dropped.
5. L4 (restated 2026-09-05, ranger-base-huhnw): answered by
   ranger-base-t4f1's arms, not separately. "Refused on exactly those two
   paths" is withdrawn — the refused set is the whole common dir minus
   three overlays, so the claim splits in two. The RENDERED set is
   `TestWorktreeGitCommonDirIsTheGitCarveOut`: common `:ro`, the three
   overlays read-write, and no mount of either mode on `config`, `hooks`,
   `packed-refs`, `refs` or another session's `worktrees/<name>` —
   MEASURED at the renderer, both PID shapes. The ENGINE composition is
   `docs/adr/0014-l4-worktree-narrowing.probe.sh` Part B: arm B2 is this
   item (`git config --local`, `core.hooksPath`, `touch <common>/hooks/…`,
   `update-ref`, `packed-refs`, the neighbour's tree — each refused, own
   tree writable, against the control with the common dir read-write
   whole), B1 the positive arm (a detached commit lands under `:ro` plus
   the overlays), B3 `gc --auto` non-fatal on `:ro` `packed-refs`. Part A
   measures the git half of the same arms behind a uid wall on this box
   (MEASURED, git 2.50.1) and is not the L4 measurement. Part B is UNRUN
   and stays so until a container engine exists somewhere — Docker is
   abandoned on this box by operator ruling (ranger-base-6mz7), so the
   venue is any box with an engine, the off-laptop cleanroom the ruling
   defers to, not this rig. Its foundation (a read-write bind lands over a
   `:ro` bind of its parent, depth-ordered, 7/7 Docker 29.0.1/VirtioFS)
   is MEASURED in `0014-path-scoped-writes.probe.sh`; what B adds is that
   composition on real git plumbing at these paths, and until it runs the
   engine half of this item is ASSUMED from that foundation.
6. L4 file binds (2026-09-05, ranger-base-n3ywd). Two halves again. The
   RENDERED set, MEASURED at the renderer in both PID shapes
   (cageoverlay_test.go, ranger-base-672zt and p9h9d): main checkout —
   one `:ro` mount on `.git/config` and one on `.git/hooks`, the `.git`
   carve-out still read-write beneath them; worktree — `:ro` mounts on
   `gitdir`, `commondir`, `config.worktree` under a read-write
   `worktrees/<own>`, the pointer file bound only on a read-write repo,
   and STILL no mount of either mode on `<common>/config` or `hooks`
   (t4f1's pin, unchanged); every shape — no mount whose destination
   ends in `.lock`, derived from the L2 readers so a widened list fails
   the pin (two-way, not a literal list); rendering creates nothing on
   the host, and the reach probe still finds the store writable. The
   launcher half: after `PrepareSessionHead`, `config.worktree` exists,
   is empty, and add/commit/status/rev-parse in the tree succeed with
   the extension off AND on. Every refusal pin gets a wrong arm that
   goes red. The ENGINE composition is arm B4 of
   `0014-l4-worktree-narrowing.probe.sh` Part B (ranger-base-017dx):
   inside the cage, append to a `:ro`-bound `config` → EROFS; `git config
   core.hooksPath` → refused with "could not write config file", the
   file byte-identical, no stray `config.lock` after; `mv` onto it and
   `rm` of it → EBUSY; `commondir` likewise — each against the control
   with the two file binds removed, where every one lands. And the
   negative the design rests on, measured on the engine and not only on
   host git: a file bind with an ABSENT source, recording what the engine
   creates on the host and that git in that tree then fails. UNRUN on
   this box (ranger-base-6mz7); venue as item 5's. Until it runs, "a
   file bind lands over the directory overlay containing it" is ASSUMED
   from the directory measurement, and it is the one claim here whose
   failure would change the design rather than the wording.
