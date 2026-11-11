# ADR 0086 — A cage map lowers the wall, never a PID's rule: hoover launches at `seatbelt (bwrap)` on the host

*Status: accepted 2026-11-11 (ranger-base-6zqdq, from the operator's ruling
on ranger-base-3xh2l, option B) · owner: architect · amends ADR 0083 D7 ·
builds on ADR 0080 D16–D23 (the bwrap mechanism of `seatbelt`) ·
amended 2026-11-11 (ranger-base-56w67): D8 carries D3 to the container
tier, and amends ADR 0014 §4's `.git` carve-out*

## Context

hoover (security, findings only) denies bare `Edit` and `Write`. On the
Linux host its demand, `cage: seatbelt`, is lowered to `shims` by the one
promoted line ADR 0083 D7 added, `cage_realize_linux: seatbelt: shims`.
Parity at shims leaves both rules unrealized ("native flags are politeness"),
and dispatch refuses the launch. The map cannot waive a rule (D7), so every
hoover bead is unlaunchable on the host.

The bead asked four questions. One fact answers most of the first:
**the Linux cage already exists.** ADR 0080 D16–D20 built `seatbelt` at the
`bwrap` mechanism: `--ro-bind / /`, then the L2 writable set bound
read-write, then the carve-outs bound read-only. A whole-tree write deny
was always in that design. `SeatbeltWritable` drops `cwd` for such a PID,
which leaves the tree under the read-only root. What stops hoover is the
map, plus one render bug this bead found by running the real render.

**MEASURED 2026-11-11 on the D1 host** (Linux 7.2.15, bubblewrap 0.13.1,
git 2.56.0, load 18–42). These rows render hoover's real argv, read from
the live home `~/homes/ranger` through `bwrapRender` with no write. cwd was
this session's linked worktree. Each workload ran under
`bwrap --args 3` exactly as the launch line types it:

| row | under hoover's wall | got |
|---|---|---|
| R0 | render in a session worktree | **refused**: `bwrap cannot grant write on <cwd>/.git: it is not a directory` |
| W1, W2 | create a file in cwd; append to a tracked file | refused, `Read-only file system` |
| W13 | append to the home's `config.yaml` | refused |
| W3, W4 | `git status`, `git log` | ok |
| W5, W6, W16 | `bd show`, `bd comments`, `bd comments add` (store of record) | ok (W16 is a real comment on this bead) |
| W7, W12 | write under TMPDIR (`/tmp`), write under `~/.claude` | ok |
| W8, W9 | `go build ./cmd/posse`, `go test -run` one pin | ok |
| W10, W11 | `make fmt-check`, `scripts/suite-lock.sh --status` | ok |
| TC | `make tree-check` (17 doors) | ok, 85s, load 24–42 |
| W14, W15 | `git hash-object -w`, then `commit-tree` on a scratch index | **ok: a findings-only seat minted a commit object** |
| G1–G3 | status, diff, log with **no** git-dir write grant | ok |
| G4 | `hash-object -w` with no git-dir write grant | refused |

W1–W16 and TC ran with R0's file grant filtered out by hand. Nothing else
in the argv was changed.

Parity was read through `CheckParity` over the live home, then over a copy
of its `config.yaml` with the map's two lines removed. With the map,
hoover's realized cage is `shims`, `Edit` and `Write` are degraded, and the
launch refuses. Without the map, hoover, dinesh and laurie each realize
`seatbelt` with nothing degraded. Of the 11 host PIDs, **hoover is the only
one** with a rule shims cannot hold: the only whole-tree write deny, and no
path-scoped write deny anywhere.

## Decision

**D1. No new tier and no second wall.** hoover's cage is `seatbelt` at the
`bwrap` mechanism, the render ADR 0080 built. A "read-only bwrap wall for
Edit/Write PIDs" would be a second render of a wall the first one already
draws: the same lists, with `cwd` already absent.

**D2. A `cage_realize_<goos>:` mapping is declined for a PID it would leave
with a rule unrealized, when this host provides the demanded tier.** The
PID then launches at its demand. This amends ADR 0083 D7, whose words
stand: a map changes the cage and nothing else, and it cannot waive a rule.
D7 then refused such a PID. D2 gives it the tier it asked for.
- The test is per PID and per runtime. The mapping is declined when
  parity at the mapped tier has an unrealized rule that the demanded tier
  realizes, and the demanded tier is in `AvailableCages`. If no such rule
  exists, the map applies exactly as today. If the demanded tier is not
  available, the map applies and the rule refuses, as today.
- A declined mapping is not a waiver: no `Ratified` row, nothing marked
  degraded. Parity prints one `DeclaredDifference` line, which never marks
  a session: `cage_realize: linux seatbelt→shims declined for this PID:
  shims leaves Edit, Write unrealized; launched at seatbelt (bwrap)`.
  `posse status`'s standing map row adds "declined for N PIDs".
- **One resolver.** `RealizeCage(demand)` becomes
  `RealizeCageFor(ag, rt, demand)`. Every door that reads the map calls it:
  CreateSession's launch path (herdrback.go), both halves of `CheckParity`
  (the demand and the relaunch's by-PID reading), and dispatch's
  `CheckTier`. If the launch and parity disagree, the session has a wall
  its record does not name. That is the defect to design out, so the build
  pins that every door reaches the one function.
- It never raises a PID above its demand and never picks a tier the PID did
  not name. It only declines to lower.
- On the host today, D2 moves exactly one seat (hoover) and changes no
  constitution line.

**D3. A PID that denies the whole tree gets no write grant into any git
dir of its session.** `SeatbeltWritable`'s `deniesFiles` arm drops
`<cwd>/.git` and the `sessionGitGrants` set (objects, logs, the ref
directory, the worktree's gitdir). The store of record's own git dirs stay
granted, because bd commits there. This holds on both mechanisms.
- It fixes R0. In a linked worktree `<cwd>/.git` is a FILE. That grant was
  dead on darwin, because the carve-out denies the same path and the last
  match wins. On bwrap it refuses the whole render, so hoover cannot launch
  at bwrap in any dispatched worktree today.
- It closes W14/W15. With objects and the ref directory writable, a seat
  that cannot edit a file can still write a commit by plumbing and move
  `posse/<session>`. The launcher fast-forwards that branch onto `main` when
  the bead closes. In the main checkout the grant is `cwd/.git` whole,
  which reaches `refs/heads/main` (STATIC: seatbelt.go grants the
  directory, and the carve-out denies only hooks and the config chain).
  The Edit/Write deny exists to make a findings-only seat's output
  findings. A wall that lets it land commits keeps the rule's letter and
  loses its point.
- G1–G4 show what it costs: `git status` still works but no longer saves
  its index refresh. A `git fetch` from inside the seat is refused. Neither
  is in hoover's intents.
- **As built (ranger-base-8t6j1, `docs/notes.d/ranger-base-8t6j1.md`).** The
  rule reaches FIVE doors, not one, and four of them answered a question
  about this PID's git writes without reading the PID: `SeatbeltWritable`'s
  arm; the create-only ref directory (`sessionRefDirs`); `BwrapWritable`,
  which does not filter the set but APPENDS the ref's parent directory
  derived from `cwd` and the branch — so with D3 in the producer alone the
  renderer handed the ref grant straight back, `refs/heads/posse` bound
  read-write; the parity ref-grant row, which goes silent here because after
  D3 both mechanisms agree and a Linux-only line would read as a
  bwrap-specific capability; and `recordTargets` (ADR 0013 §4), the one that
  is a GATE — it probed `<cwd>/.git` and refused the launch for the write
  this decision removes, wherever the store of record is the session's own
  tree, which no re-grant-free launch could ever satisfy. All five read one
  predicate, `deniesFileWrite`. R0 was reproduced and cleared against the
  live home, and the live wall arm ran on the D1 host.

**D4. What "read-only" means for a verifier that runs suites.** It means
the mechanism-neutral `SeatbeltWritable` set, after D3. That set is TMPDIR
(`/tmp` on the host), `~/.cache` (GOCACHE and the suite-slot locks under
`~/.cache/posse`), `~/.local/share`, `~/.npm`, Go's telemetry dir, the
claude config dir (ADR 0080 D19), `cwd/.beads` and the store of record, the
home's state dir minus `gates` and `hooks`, and the persona's memory dir.
Nothing is added for hoover. A test that writes into its package directory
fails under this wall, as it did under hoover's seatbelt profile on the
laptop. That is the deny working, not the wall misfiring.

**D5. Parity proves the Edit/Write rows by a witness at the launch, not by
availability.** Today the row reads `L2 seatbelt` because the tier is
available, which is a claim about the host and not about this wall. For a
PID with a whole-tree write deny at `seatbelt`, the launch types the
rendered prefix (`WallRender`, the same door `wallReachProbe` uses) and
runs one create under it: `: > <cwd>/.posse-wall-witness.<nonce>`, and the
same under the session's git objects dir (D3). Both must fail.
- If either create succeeds, the probe removes the file it made, and the
  rule is unrealized with the path named. The launch refuses.
- The row's Detail becomes `L2 seatbelt (<mechanism>), witnessed at
  launch`.
- It is a create and not `test -w`, because `access(2)` under seatbelt is
  not measured. A create is what the deny forbids, on both mechanisms.
- What a witness cannot see: a write the wall allows somewhere it does not
  probe. D3's list is the complete statement of where it allows writes.

**D6. The fixture-git lesson (ranger-base-xa2e3, and ppnds finding 3).**
R0 escaped because no pin ever drove a whole-tree-deny PID in a linked
worktree. Every bwrap fixture with `deniesFiles` was a plain directory or a
main checkout. So:
- every pin for D3 and D5 drives a whole-tree-deny PID in a real linked
  worktree (`wgNewFixture`) and in a main checkout, and asserts the KIND of
  every grant, not only that it lies inside one;
- `bwrapgitrefslive_test.go` gains a whole-tree-deny arm. It runs live on
  Linux with bwrap and skips elsewhere. Under the RENDERED argv it checks
  that a cwd create is refused, `git status` works, and
  `git hash-object -w` is refused;
- git run inside a fixture resolves past the gates, as xa2e3 settled. Under
  this wall the gates dir is bound read-only and still executes. The seat's
  own git still goes through its shim, which is the point.

**D7. The rest of D6 C's flip stays the operator's, and it waits for
evidence.** Removing the `seatbelt: shims` line moves dinesh, gwart,
holden, jian-yang and laurie onto bwrap together. That was always ADR 0083
D6 C's end state. It is one promoted line, and putting the line back is
the rollback. It is filed as an operator question. It is gated on D2, D3
and D5 having landed, and on hoover's first three sessions closing with no
wall finding. Those sessions are the first live evidence at this
mechanism: no seat on this box has run a session at bwrap (MEASURED: the
weller home's two PIDs are `cage: shims`).

**D8. D3 holds at the container tier (L4) too.** For a PID that denies
the whole tree (`deniesFileWrite`, the predicate D3's five doors read),
`CageMounts` mounts no git dir of the session read-write:
- **main checkout:** the `<dir>/.git` read-write carve-out over the `:ro`
  repo is not mounted. `.git` stays under the `:ro` repo mount.
- **linked worktree:** the common dir stays `:ro`, with **none** of the
  `sessionCommonDirWrites` overlays (`worktrees/<own>`, `objects`, `logs`).
- **kept:** the `<dir>/.beads` carve-out, and the store of record's overlay
  when a redirect puts it in another repo. At L4 that overlay was already
  `.beads` only, with no `.git`, because the inner `bd` is no-db and never
  commits. So this tier keeps less than L2 does, and needs no more.
- **unchanged:** every other PID's mounts; the `:ro` binds of
  `cageGitIdentityBinds`, which land on a `:ro` mount and so add nothing
  here; and `PrepareSessionHead`, which still detaches a caged session's
  HEAD. Detaching is harmless here and keeps one launch path per tier.
  The splice at close then finds nothing to move.

The bead behind this asked which half to keep. Its own estimate was that
the linked-worktree exposure was small ("objects + logs + the per-worktree
dir"). **It is not small.** Those three regions are exactly what a
detached commit writes. They are why the narrowing in ranger-base-t4f1
could grant them and still let a seat commit. A seat that cannot edit a
file can still write a blob, stage it with `update-index --cacheinfo`, and
commit on its detached HEAD. At close, `MergeSessionWork` runs
`spliceDetachedWork` *before every guard*. That fast-forwards
`posse/<session>` onto the injected commit, and the landing carries it to
`main`. That is W14/W15, reached through the launcher's own splice
instead of a ref grant. In a main checkout the carve-out is `.git` whole,
and `update-ref refs/heads/main` moves main directly.

MEASURED 2026-11-11, D1 host (Linux 7.2.15, bubblewrap 0.13.1, git
2.56.0, load 0.9), `docs/adr/0086-l4-git-carve-out.probe.sh`. The probe
uses CageMounts' mount list mirrored under bwrap, because this seat cannot
reach the docker socket:

| shape | L4 today | after D8 |
|---|---|---|
| linked worktree, detached | cwd create refused; `hash-object -w` ok; **commit ok**, and `posse/s1` is an ancestor of it (the splice fast-forwards) | `hash-object -w` and commit refused; HEAD unchanged |
| main checkout | cwd create refused; `hash-object -w` ok; **`update-ref refs/heads/main` ok, main moved** | both refused; main unchanged |
| either shape | `git status`, `git log` ok | `git status`, `git log` ok |

The `:ro` repo does not make the carve-out safe. The repo mount realizes
"cannot edit a file", and the carve-out reopened "can land a commit"
underneath it. That is the same gap D3 closed one tier down. A tier stricter
than L2 that is weaker than L2 on the one capability the deny exists to
remove is not stricter.

ADR 0014 §4's sentence "L4's `:ro` repo always carries L2's carve-outs …
`.beads/` and `.git/`" reasoned "match L2". It still does: L2 no longer
grants `.git` to this PID, so matching L2 means `.beads/` alone. The
amendment is in ADR 0014's text, and cage.go's comment stops citing the
L2 grant (the build bead).

## Alternatives rejected

- **Delete the map line now.** This is the boring answer, and the one ADR
  0083 D6 C pointed at. It changes five working seats to help one, with no
  live session at bwrap behind it, and the operator is the only rollback.
  It would also not have worked: R0 refuses hoover's render in every
  session worktree. It stays the end state (D7), not the first step.
- **A narrower read-only bwrap wall for Edit/Write PIDs only** (the bead's
  option b). See D1. `SeatbeltWritable` already drops `cwd` for exactly
  these PIDs. A second render is a copy that drifts.
- **A per-persona exemption key** (`cage_realize_linux_exempt: [hoover]`).
  It puts persona names into a map keyed by tier. It also re-states, by
  hand, what the deny list already says, and a hand copy can disagree with
  it.
- **A `cage_floor:` in the PID**, the twin of `tier_floor:`. This was the
  clever one, and I liked it because it reads at a glance. But it is
  declared where D2 is derived: a PID that grows an `Edit` deny without a
  floor refuses as today, and a floor on a PID that needs no wall pins it
  above the host's map for nothing. Parity already knows which rules need
  which tier.
- **Keep the git grants (skip D3) and fix only R0.** That is the smallest
  diff. It leaves W15: the Edit/Write rows would read "realized" over a
  seat that can land a commit.
- **Landlock.** ADR 0080 D13. Nothing here reopens it.
- **(D8) Keep the L4 carve-out because the repo mount is already `:ro`.**
  This was the bead's other option. The probe refutes it: the `:ro` repo
  does not stop a commit, and the carve-out is the route to one.
- **(D8) Keep the linked-worktree overlays and drop only the main
  checkout's `.git`.** This was the half the bead thought was small, and it
  was the clever one. Detaching HEAD did remove the ref grant. But the
  launcher's own splice puts back the ref write the detach took away, so
  the worktree shape lands the commit too, just one step later.
- **(D8) Keep the overlays and refuse the splice for a whole-tree-deny
  PID.** That would be a gate at close that reads the PID, protecting a
  grant the PID never needed. It would also leave injected objects in the
  operator's store. Not granting the write is one branch; guarding its
  result would be a second door that has to agree with the first.
- **(D8) A witness at L4 like D5's.** The L4 wall is the mount list, and
  `CageMounts` is pure. A pin over its output, in both shapes, asserts what
  a witness would. The engine-side arm stays the off-box one
  (ranger-base-017dx).

## Consequences

- hoover launches on the host at `seatbelt (bwrap)` with no constitution
  change. The other seatbelt PIDs stay at `shims` until D7.
- The cage_realize map now means "lower the wall where nothing the PID
  denies depends on it". `posse status` says when it declined.
- On darwin, D3 also narrows hoover's seatbelt profile: no commit objects,
  no ref writes. If a whole-tree-deny PID ever needs to commit, it does not
  have a whole-tree deny. Its deny list is the place to say so.
- D5 adds one exec under the wall to each such launch. Its cost is ASSUMED
  to be milliseconds. The build reads it.
- Exit hatch: D2 is one decline rule inside one resolver, and removing it
  returns ADR 0083 D7's refusal. D3 is one branch in `SeatbeltWritable`.
  D5 is one row. Nothing holds state.
- D8: a whole-tree-deny seat at L4 can no longer save `git status`'s
  index refresh, `git fetch` or write any shared-ref reflog. That is G1–G4's
  cost one tier up. No PID on this box asks for `cage: container` with a
  whole-tree deny today, so nothing moves on the host. D8 is one branch
  on `deniesFileWrite` in `CageMounts`, and removing it restores the
  carve-out.

## Measured versus assumed

- MEASURED 2026-11-11, D1 host: R0, W1–W16, TC and G1–G4 (table above);
  parity with and without the map for hoover, dinesh and laurie; and the
  11 host PIDs' deny lists. MEASURED with a caveat: a code seat (dinesh)
  rendered and committed under its own wall in a scratch linked worktree.
  Its "move main" row measured nothing, because the scratch repo sat under
  `/tmp`, which the wall binds read-write. Probe T3 (ranger-base-r5m98) is
  the host's reading of that row.
- MEASURED 2026-11-11, D1 host, `docs/adr/0086-l4-git-carve-out.probe.sh`
  (D8's table). The mount list was mirrored under bwrap, not run under
  docker: the probe measures which git writes need which overlay. The
  depth-ordered bind semantics it relies on are ADR 0014's docker
  measurement (7/7).
- STATIC (code read), D8: `MergeSessionWork` splices a detached session's
  work before any guard (worktree.go), so the probe's ancestry check
  stands for the fast-forward. The container reach gate
  (`containerReachRow`) probes only the store, so D8 has no fifth-door
  deadlock at L4. `cageGitIdentityBinds` adds nothing over a `:ro` mount.
- ASSUMED, D8: that `git status` under docker's `:ro` bind behaves as it
  does under bwrap's (EROFS on the opportunistic index write, and the
  write is skipped). It is the same errno from the same syscall.
- STATIC (code read): the main checkout's `cwd/.git` grant reaches
  `refs/heads/main` for a whole-tree-deny PID; the launch, parity and
  dispatch doors reading the map through one function today
  (`RealizeCage`).
- ASSUMED:
  - a full `make test` under hoover's wall. Only `tree-check` and focused
    runs were measured. Three suite slots on a box at load 40 were not
    worth spending on a seat whose intents are audits;
  - D5's cost;
  - that `access(2)` is not a reliable witness under seatbelt, which is
    why D5 uses a create;
  - a real claude turn under the wall. ADR 0080's probe T1 measured the
    config save, and B9 measured `--version`. hoover's first session is
    the first turn, which is why D7 waits for it.
