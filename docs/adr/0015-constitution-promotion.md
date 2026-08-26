# ADR 0015 — Promote the constitution: three trees, three write policies, one ratification step

*Status: proposed 2026-08-26 · owner: architect · awaiting operator
ratification (ranger-base-yqpx) · execution rides with the rhq
retirement (ranger-base-3rv9, operator-ruled 2026-08-25) · informs 0002
§3, 0012 D3-C, 0014 §3*

> The operator asked for the constitution to be clearly separated from
> project work. The instance tree currently holds three kinds of thing
> with three incompatible write policies, and the resulting cage
> question kept coming out unanswerable. This ADR answers it by moving
> the gate off the write and onto the *taking-effect*.

## Context

Three layers nest, and each has a different blast radius when edited:

| layer | what an edit changes | gate between edit and effect today |
|---|---|---|
| posse (mechanism) | what a wall *is* — gates, seatbelt, parity | `make install`: fleet runs the installed binary via `plugin/bin/posse`; install is human-run and persona-denied (**MEASURED**, Makefile + settings) |
| constitution (PIDs, config, recipes, skills, envs) | the prose injected into every future session | **none**: `~/.config/rhq` is a symlink into the instance repo, one inode (**MEASURED**); a saved PID is in force at the next launch, uncommitted edits included (ranger-base-5na) |
| project work | one repo's contents | ordinary review |

The more powerful layer has the stronger gate; the constitution has
none. That inversion — not a missing sandbox — is the actual gap.
Under seatbelt a session whose cwd is elsewhere already cannot write
the instance tree (**MEASURED**, ranger-base-h15); the one hole is a
session dispatched *into* it, where cwd covers the PID dir
(ranger-base-6ne). And seven of eight personas run at shims, where no
file wall exists or is coming (ADR 0002 §3) — so any answer built on
denying the write is prose for most of the fleet.

The instance tree also mixes three write policies in one repo:

1. **Constitution** — `rhq/agents`, `config.yaml`, `recipes/`,
   `skills/`, `envs/`. Rare, deliberate, should be attributable.
2. **Runtime state** — `.beads` (the queue, store of record for every
   repo the crew touches, reached by `.beads/redirect`) and
   `rhq/state`. The write-hottest paths in the system; deniable to
   no one. The seatbelt grant for the redirect target also grants the
   instance repo's *git dirs* to every session (**MEASURED**,
   `seatbelt.go` redirect grant, ranger-base-rhw).
3. **Tooling and docs** — `scripts/`, `docs/`. Ordinary project work.

You cannot make one tree read-only while (2) lives inside it; a
carve-out list is a deny-list maintained forever (ranger-base-h15 is
the surgical form). The operator's external reference (Yegge, *Fences,
Not Sandboxes*) names the missing lifecycle step exactly: rules are
*proposed, ratified, then built* before they take effect — his shop
gates rule changes the way we already gate the binary and do not gate
the rules. Our precedent half already exists (beads as case law,
ORDERS, ADRs); the ratification step and the authority envelope are
what this ADR adds.

## Decision

**1. Three trees, three write policies.** The classification is by
*taking-effect path*, not by directory sentiment:

| tree | contents | write policy | in force via |
|---|---|---|---|
| mechanism repo (posse) | source | ordinary project work | `make install` (exists) |
| constitution repo (the instance tree) | PIDs, config, recipes, skills, envs — plus its own docs/ and scripts/ | ordinary project work — **drafting is open to personas** | `posse promote` (new) |
| runtime home `~/.config/posse` | promoted constitution copy · `state/` · `personas/` (memory) | see §2/§5; never "promoted", never draft | n/a — it *is* the live surface |
| queue repo (new, §4) | `.beads` store of record | every session, always, via redirect | n/a — data, not prose |

The split the operator asked for is therefore mostly **not a repo
move**: posse is already separate, the constitution repo already
exists. What moves is the queue out (§4) and the live surface off the
symlink (§2).

**2. The home becomes the promoted copy.** `~/.config/posse` — the
path the binary already prefers (**MEASURED**, `app.go`) — is created
as a *real directory*, not a symlink. `agents/`, `config.yaml`,
`recipes/`, `skills/`, `envs/` under it are written **only** by
`posse promote`. The `~/.config/rhq` symlink dies with the rhq
retirement; its replacement is the first promote. This is the "one
path move" the ride-together ruling requires. `App` paths do not
change shape — `AgentsDir` is still `home/agents`; what changes is
what writes it.

Consequence, structural rather than carved: a session dispatched into
the constitution repo edits *drafts*. Nothing it writes reaches any
session until promotion. The 6ne hole closes without denying the
write, and "constitution session" stops being a special thing to mark
(the old Q3): there are drafting sessions, which are ordinary, and
there is promotion, which is the operator's.

**3. `posse promote`.** Operator-run, like `make install`, and fenced
the same way twice:

- **Preconditions**: the constitution repo's working tree is clean and
  the promoted ref is a commit — uncommitted prose can never be in
  force, which is the attributability 5na asked for.
- **Manifest**: promote records `{source repo path, git SHA, sha256
  per promoted file}` under the home *beside* the promoted copy (not
  under `state/`, which stays session-writable). It prints
  `git diff <last-promoted>..HEAD -- <constitution paths>` so the
  operator ratifies a diff, not a vibe.
- **Launch verify** (absorbs the shape of ranger-base-5na): every
  launch hashes the promoted set against the manifest. Dispatch
  **refuses** on mismatch; an interactive launch warns DEGRADED. The
  fix for a mismatch is one command (re-promote), so fail-closed is
  cheap. At shims this is detection, not prevention — said plainly:
  promotion is a fence in Yegge's sense, and the wall that does exist
  (seatbelt never grants the home's constitution area; only
  `home/state` and the persona's own memory dir) covers the promoted
  copy for every caged session.
- **Fence, spelled twice** (ORDERS rule): every crew PID gains
  `deny: [Bash(posse promote:*)]` — an L1-realizable named verb on all
  runtimes — and `promote` itself refuses to run when the persona env
  marker is set. Both are politeness against a determined session;
  the manifest check is what notices.

**4. The queue gets its own tree.** A new repo (proposed
`~/src/ranger-queue`; name is the operator's to veto at ratification)
holds the `.beads` store of record. Every repo the crew touches —
the constitution repo now included — reaches it via the existing
`.beads/redirect` mechanism (ADR 0012 D3-C; the redirect grant at L2
and the codex/L4 equivalents already handle a store outside cwd,
**MEASURED** in the posse→instance direction).

Why move it at all: while the queue lives in the constitution repo,
every session in the fleet holds a write grant into that repo's
`.beads` *and its `.git` dirs* (the redirect grant needs them for the
jsonl commit path). After the move, **no session has any write grant
into the constitution repo unless dispatched into it** — the property
becomes checkable by reading the writable set, not by auditing
carve-outs. It also takes the queue-flush commits out of the
governance history, so `promote`'s diff and the repo's log are pure
constitution.

Two facts that bound the implementation, one measured, one to
measure:

- `bd sync` exports jsonl but **does not commit** (**MEASURED**,
  `--help`; `--full` commits but also pushes). In a dedicated queue
  repo nobody's pre-commit hook rides along, so the **launcher**
  commits the jsonl in the queue repo at each close-merge (it already
  owns a git moment there). Never an automatic push.
- Store relocation itself — stop daemon, move dir, rewrite redirects,
  restart — is **ASSUMED** to be clean; the implementation bead
  measures it on a copy before the cutover window, and checks bd's
  current flag surface first (`--no-db`, sync config) rather than
  designing around remembered limits.

Issue ids keep the existing prefix; the daemon already refuses foreign
prefixes, and an id-prefix rename is destructive and explicitly out of
scope here.

**5. Memory is not law.** `home/personas/` (ORDERS) is excluded from
promotion and the manifest, and remains a symlink into the
constitution repo's `personas/` — the one symlink that survives.
Rationale: ORDERS is persona-authored memory whose write loop
(append a lesson at session end) dies under a ratification gate, its
injection is scoped to its own persona, seatbelt already grants only
the session's *own* memory dir, and the repo symlink keeps the git
attributability we have today. This is an accepted, named piece of
unratified prose injection — scoped, versioned, and per-persona — not
an oversight.

**6. What this dissolves elsewhere** (their owners' beads, named not
edited): ranger-base-h15 shrinks to its gates/hooks half — the
constitution part of its trailing deny becomes structural.
ranger-base-5na's digest check becomes §3's launch verify, with the
manifest as the trust anchor it lacked. ranger-base-8hd §2 becomes
decidable: crew-wide seatbelt is worth deciding for project sessions,
where it is already correct, and is *not* the fix for constitution
edits — the fence is. ranger-base-6ne is a documented mode, closed by
construction rather than by wall.

**Deferred as amendments** (do not hold the retirement for them):
Q5 — whether the reporting path (`parity`, `posse gates`) is held to a
verification standard outside posse (ranger-base-3c3 is the live
case); Q6 — whether a posse-development session needs the live
constitution at all (with §2 it reads testdata by default; nothing
special is mounted).

## Sequencing

Rides with ranger-base-3rv9, per the operator's ruling. The
retirement step "retire the `~/.config/rhq` symlink" becomes "first
`posse promote` creates `~/.config/posse`", and the same window moves
the queue. One window, two moves, zero moves later. Nothing in this
ADR blocks the parallel beads already running (dk5, w1b, g7lt).

## Alternatives rejected

- **Carve-outs in one tree as the policy** (h15 generalized). A
  deny-list that must be maintained forever and is wrong the first
  time someone adds a directory; also unenforceable at shims, where
  most of the fleet runs. h15 survives as hardening for the gate
  artifacts, not as the answer.
- **Constitution read-only; operator edits only.** Contradicted by
  practice the same week it would have shipped — the operator
  explicitly directed persona PID edits. Drafting is legitimate;
  what was missing was the step between draft and force.
- **Git-merge-as-ratification** — the clever one, and the one I
  wanted: personas commit to branches, the operator's merge to main
  *is* ratification, the home symlink points at the checkout. Rejected
  because the symlink makes the *working tree* live regardless of
  branch discipline: an uncommitted edit is in force, a dirty checkout
  is a dirty constitution, and a session dispatched into the repo
  still holds the live tree writable. Merge is review of history;
  promote is deployment of state. The manifest gives the launch
  something to check; a branch policy gives it nothing.
- **Queue stays in the constitution repo.** Zero move cost, and the
  redirect grant is already surgical. Rejected because it leaves every
  session a write grant into the constitution repo's `.git`, keeps
  governance history drowned in queue flushes, and makes "no grants
  into the constitution repo" impossible to state. Priced honestly:
  the move costs a daemon cutover and a new committer for the jsonl
  (both bounded, §4); staying costs an audit forever.
- **Queue unversioned in the runtime home.** Simplest layout; loses
  the jsonl git history that is the queue's recovery story. The store
  of record deserves a journal with history.
- **Crew-wide seatbelt as the answer.** A sandbox where the incident
  class needs a lifecycle: it does not reach the dispatched-into case,
  does not exist at shims, and pins the runtime choice. Its real,
  separate value for project sessions is 8hd's call, unblocked by §6.
- **A second OS user / ACLs on the constitution.** Same-user is the
  operating model; macOS ACL maintenance would be a fourth policy
  regime; and detection (manifest) plus a promotion step buys the
  same assurance without fighting the platform.

## Verification (QA's checklist)

1. After first promote: `~/.config/posse` is a directory, not a
   symlink; `readlink` on it fails; manifest present and naming the
   promoted SHA.
2. Edit a PID in the constitution repo, do not promote, relaunch: the
   session's injected PID is unchanged. Promote: it changes.
3. Corrupt one byte of `home/agents/<any>.md`: dispatch refuses with
   the mismatch named; interactive launch prints DEGRADED; re-promote
   clears it.
4. `posse promote` on a dirty constitution tree: refused, names the
   dirty paths. Under a persona env marker: refused. From a PID:
   `Bash(posse promote:*)` denied at L1 (shim refusal logged).
5. Under seatbelt with cwd elsewhere: write to `home/agents` refused;
   write to own `home/personas/<self>` allowed; write to another
   persona's memory dir refused. *(Unverified until built: needs the
   §3 profile change removing the legacy hardcoded state grant.)*
6. Queue after the move: `bd ready` / `bd comments add` / `bd close`
   work from a project worktree, from the constitution repo, and from
   a session worktree — all through redirects; the constitution
   repo's `.git` is in no session's writable set (`posse gates`
   prints the set). A bead close produces a jsonl commit in the queue
   repo and no push. *(Unverified until the cutover rehearsal, §4.)*

## Measured versus assumed

| claim | status |
|---|---|
| `~/.config/rhq` → instance repo, one inode | **MEASURED** 2026-08-26 (`ls -la`) |
| binary promotion gate exists and is persona-denied | **MEASURED** (Makefile, install target + comment) |
| `~/.config/posse` preferred by the binary when present | **MEASURED** (`app.go` home resolution) |
| seatbelt: cwd-elsewhere session cannot write the instance tree | **MEASURED** (ranger-base-h15 probe, 2026-08-24) |
| redirect grant adds store-of-record dir **and its git dirs** to every seatbelt session | **MEASURED** (`seatbelt.go`, ranger-base-rhw) |
| legacy hardcoded `.config/rhq/state` grant exists and must move to derived home state | **MEASURED** (`seatbelt.go` grant list) |
| `bd sync` does not commit; `--full` commits and pushes | **MEASURED** (`bd sync --help`) |
| store relocation via redirect rewrite + daemon restart is clean | **ASSUMED** — implementation bead rehearses on a copy first |
| launcher-side jsonl commit at close-merge is a natural hook point | **ASSUMED** — bead confirms against the close path before building |
| launch-time hashing of the promoted set is negligible | **ASSUMED** (dozens of small files; measure in the bead, it bounds the refusal path) |
