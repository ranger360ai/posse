## Path-scoped writes at L4: the overlays, measured (ranger-base-yu5)

ADR 0014 §4 shipped with its central mechanism marked **ASSUMED** — "the
Docker probe was not run this session" — and named this bead's done-when as
that probe. It is now `docs/adr/0014-path-scoped-writes.probe.sh`: seven
probes, each with the control arm that has the overlay taken away, run
2026-08-29 on macOS 26.4.1 / Docker Desktop engine 29.0.1 (VirtioFS). All
seven answered their expect line, so nothing DIVERGED and the tier really
holds the rules the matrix has been printing since ranger-base-4ks.

What the engine actually does, as against what the ADR guessed:

- A bind of a directory lands over a bind of its parent in **both**
  directions — `:ro` over read-write *and* read-write over `:ro`.
- The ordering rule is **destination depth, not list order**. The overlay
  listed *before* the repo it sits on gives the same answers (probe 6). So
  "later mount wins" was the right answer for the wrong reason, and a
  renderer that relied on emitting parent-first would have been relying on
  nothing.
- A `:ro` overlay whose **source does not exist** still denies: the engine
  creates the source in the writable parent, and `touch` in it is refused
  (probe 7). That matters because a rule is about a PATH — `mkdir docs/adr`
  is exactly what a persona does next — and a Stat-guarded deny would have
  been a wall that disappears when the directory has not been made yet.

### Two things the renderer had to be built around

**deny-wins cannot be delivered by order here.** At L2 a `writable:` extra
inside a denied subtree loses because SBPL takes the last match and the
trailing deny block is below every grant. At L4 that extra is *deeper* than
the deny containing it, so depth-sorting makes it **win** — the exact
inversion of ADR 0001. `cage.go` therefore **drops** such an extra rather
than trying to out-order it. Same rule, opposite mechanics, and it is why
`posse agent check`'s warning about the pair is now true at both tiers.

There is a *second* pair with a different answer, added with ADR 0038
decision 4a (ranger-base-672zt): an extra naming the **same destination** as
a deny the code places — `writable: [.git/hooks]` against the `:ro` bind of
the hooks dir. Dropping cannot deliver it, because the extra is not inside
the deny, it *is* the deny's path, and `cageOverlay` answers a
same-destination overlay by EDITING the existing mount's mode (two binds on
one destination is an engine error). So whichever pass runs second wins, and
`cageGitIdentityBinds` runs **after** `cagePathScopedOverlays` for that
reason alone. Order delivers deny-wins for this pair and cannot for the
other one; both mechanics are load-bearing and neither generalizes.

**An overlay must be spelled the way the mount it lands on is spelled.**
This one is silent and was caught by mutating the code under the pins, not
by reading it. A session dispatched through a symlinked parent (`/tmp/x` on
macOS, really `/private/tmp/x`) mounts the repo at the path it was *given*;
inside the container there is no symlink to follow. An overlay resolved for
the host lands at a destination nothing mounts — the bind succeeds, `posse
cage` prints it, `posse gates` says `✓ L4 :ro overlay`, and the denied
subtree is writable at the only path the persona can reach it by. So
`cageCovering` finds the deepest mount containing the resolved path and
re-spells the overlay in *that mount's* words, source and destination both.

### The carve-outs on a `:ro` repo

`.beads` was already there (rangerhq-3nxk). `.git` is now beside it, which
is ADR 0014 §4 answering the same question the `.beads` clause answered: L2
grants a write-denying PID `cwd/.git` for index refresh and git's own locks,
and a tier that took it away would be enforcing more than the gate. In an
ordinary repo that is a directory inside the repo mount and an overlay says
it. In a **worktree** `.git` is a FILE and the index, HEAD and objects all
live in the common dir, which is a mount of its own.

### The worktree common dir is `:ro`, and narrower than L2 (ranger-base-t4f1)

ranger-base-yu5 shipped that common dir read-write WHOLE, which was **wider
than L2** and said so here. ranger-base-6q5e assessed what the width bought
an adversary at the one tier whose job is containing one: `refs/heads/main`
(an `update-ref` is not a push, so L1's shim never sees it, L3's pre-push
never fires, and the launcher's own ff fires no hook either), `packed-refs`
wholesale, `config` — where a planted `core.hooksPath` **dodges** the
hooks-`:ro` overlay of ranger-base-3c3/h15 rather than being stopped by it,
because moving the slot beats freezing it — and other sessions'
`worktrees/<name>`, their HEAD and index included.

What ships now is **narrower than L2 rather than wider**: the common dir
`:ro`, with read-write overlays of `worktrees/<own>`, `objects` and `logs`,
and **no ref write at all**. L2's grant is those three plus the session's own
`refs/heads/<branch>` and the `.lock` git renames onto it (ranger-base-m2wf),
plus that ref's parent directory for CREATION only (ranger-base-uuze — the
branch carries a slash, so git must `mkdir refs/heads/posse`, and `git gc`
prunes it as soon as a pack empties it). The ref half is not expressible as a
mount list and no amount of care makes it one: a bind mount's source must
EXIST, git creates the `.lock` at commit time, a pre-created `.lock` fails
every commit with "File exists", and the `rename(2)` that commits a ref update
cannot replace a bind mountpoint. Lock-then-rename and file-granular binds
fight by construction.

**So the session is launched on a DETACHED HEAD instead** (`PrepareSessionHead`
in worktree.go, called from `planLaunch` once the tier is resolved), and the
launcher splices the work back with `git -C <tree> branch -f <branch> HEAD`
at close — the exact command `landed()` has always printed for humans, run
under the launcher lock before `MergeSessionWork`'s guards. Detaching is what
buys the narrowing, and it is measured rather than reasoned: under exactly
those three writable regions the same commit lands detached and **fails on the
branch**, at `cannot lock ref 'HEAD': Unable to create
refs/heads/posse/s-1.lock` (2026-09-05, git 2.50.1, arm A4 of
`docs/adr/0014-l4-worktree-narrowing.probe.sh`).

Three consequences worth keeping:

- The splice runs only for a session posse RECORDED as detached
  (`branch.<b>.posseDetached`, git config on the branch beside `posseBase` and
  `posseBead`, so it survives a kill that removes the meta). Without that
  record it would silence the ranger-base-dybv guard — an off-branch HEAD is
  designed here and an anomaly everywhere else, and only the record tells them
  apart. It is fast-forward only: `branch -f` has no ancestry check, and a
  branch tip the tree's HEAD does not reach is work it would delete.
- `worktree list --porcelain` prints `detached` where it would have printed
  `branch refs/heads/…`, so the landing sweep, `posse worktrees` and the merge
  stopped seeing these trees entirely — over exactly the sessions whose work
  is in the tree and not on the branch. `SessionTreesIn` now recovers the name
  from the tree's directory (`SessionTreePath` makes it the session name,
  `SessionBranch` makes the branch from that) and confirms it against the
  repo. Residual, stated: a detached tree whose branch somebody deleted is
  invisible there — git refuses `branch -D` for a branch a worktree has
  CHECKED OUT, and a detached tree has none.
- `<common>/logs` is in the set because L2's is, not because a detached commit
  needs it. Measured (arms A5/A5b): a linked worktree's HEAD reflog is
  per-worktree (`worktrees/<own>/logs/HEAD`), and `<common>/logs/refs/heads/…`
  appears only when a commit moves a SHARED ref — the thing detaching removes.
  It is kept for the in-cage operation that does update one, and the launcher
  `mkdir -p`s it so the rendered mount set does not depend on whether this repo
  has ever written one.
- The launcher makes a second source for the same reason, and this one does
  not exist at all until it does: `worktrees/<own>/config.worktree` (ADR 0038
  decision 4b, ranger-base-p9h9d). The identity chain that selects WHICH
  config and hooks a later git reads gets `:ro` file binds over the
  read-write `worktrees/<own>` overlay — the pointer, `gitdir`, `commondir`
  and that file — but posse never sets `extensions.worktreeConfig`, so `git
  worktree add` writes no `config.worktree` and the deny direction's Stat
  (`cageOverlayFile`) would drop the bind for want of a source. It cannot
  simply bind the absent path: a `-v` of an absent source becomes a host
  DIRECTORY, and a `config.worktree` directory makes every git command in
  that tree fatal (MEASURED, git 2.50.1). **A wall keyed on a config key
  would read a different repo from the wall beside it**, so
  `PrepareSessionHead` creates the file EMPTY instead and the bind is
  unconditional. An empty one is inert in both directions, measured: with
  the extension off git never reads it, with it on there are no keys to
  read. Never truncated on a relaunch — that would be posse deleting the
  operator's own per-worktree config.

This **subsumes ADR 0038 decision 4** for worktrees (folded as
ranger-base-mugt2, operator-confirmed 2026-09-01): that asked for `:ro` file
binds of `<common>/config` and `<common>/hooks` over a read-write common
mount, and both paths are now inside the `:ro` mount and under none of the
three overlays. The ADR's own prose still describes the read-write-whole
mechanism and is stale; amending it is architecture's, filed as a handoff.
`.git/hooks` in an ORDINARY repo is still ranger-base-3c3 / h15.

Residual after the narrowing, stated: `objects` (content injection — inert
until a ref names it, and naming goes through the splice and the launcher's
ff), `logs` (the reflog), and the session's own worktree dir. Same class L2
accepts.

Measurement: the mount SET is pinned in Go
(`TestWorktreeGitCommonDirIsTheGitCarveOut`, cageoverlay_test.go, with the
`logs`-absent arm that shows the Stat guard dropping an overlay), and the
detach/splice round trip in worktree_test.go — fourteen mutants, all killed.
`docs/adr/0014-l4-worktree-narrowing.probe.sh` carries the rest in two parts:
Part A is MEASURED here (uid permissions, not the L4 wall, but it is the half
that is about git rather than about the engine), Part B is the bind-mount arms
and is **UNRUN** — Docker was abandoned on this box on 2026-08-30
(ranger-base-6mz7). Part B's foundation is measured: the seven probes above.

Every commit under a narrowed common dir — L2's `sessionGitGrants` and now
L4's mount set alike — also prints `error: Unable to create
'<common>/packed-refs.lock': Operation not permitted` on stderr and still
succeeds; git takes that lock speculatively on every ref update and falls
back cleanly when refused. Measured again at L4 on 2026-09-05 (arm A3), where
`gc --auto` after such a commit also exits 0 and leaves HEAD intact. DECLARED
rather than fixed (ranger-base-msex):
the tempting createOnly grant was measured and is worse than the noise —
create-only buys the create but not git's own cleanup unlink, so the lock
file is stranded in the shared common dir, which then hard-fails the
operator's own unsandboxed `git gc`/`git pack-refs` until someone notices
and removes it by hand. No session can write it away either. See the doc
comment beside `sessionRefDirs` in `seatbelt.go` for the measurement.

The **redirect target** joins them, unconditionally rather than only on a
`:ro` repo: when `.beads/redirect` puts the store of record in another repo,
`<dir>/.beads` holds a path and nothing else, so the carve-out above mounts
a directory nothing writes and every mutation lands outside the cage — the
L4 shape of the L2 failure ranger-base-rhw measured. The target alone, not
its `.git`: the inner wrapper is `--no-db --no-daemon`, appends JSONL and
never commits, so L2's git grant buys nothing here and would mount a second
repo's history read-write.

### A trap that is not the engine's

Re-running these binds by hand in **zsh**: `"$R:$R:ro"` is not what it looks
like. `:r` is a zsh modifier (strip the extension), so the word becomes
`$R:${R}o` — docker then binds an empty auto-created directory read-WRITE at
a destination one character off, and the probe reads as "the engine ignored
`:ro`". Spell it `"${R}:${R}:ro"`. The probe script is `/bin/sh` and is not
affected; this cost a measurement that briefly looked like a DIVERGED.

