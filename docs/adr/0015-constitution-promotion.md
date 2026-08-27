# ADR 0015 — Promote the constitution: three trees, three write policies, one ratification step

*Status: proposed 2026-08-26 · owner: architect · ratified 2026-08-26
(ranger-base-ap2x) · amended 2026-08-26: §7 moves `envs/` out of the
promoted set (ranger-base-h56a) · amended 2026-08-26: §3's clean gate
is the promoted paths, not the whole tree (ranger-base-yb9j, built in
ranger-base-o943) · amended 2026-08-27: verification item 5 verified
live, marker dropped, cwd-elsewhere boundary clause added
(ranger-base-4v1d) · amended 2026-08-27: §3's invariant re-attributed —
the commit read, not the clean gate, keeps bytes == SHA
(ranger-base-znma via ranger-base-70ci; the set half is open in
ranger-base-70ry) · execution rides with the rhq
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
   `skills/`. Rare, deliberate, should be attributable. *(As first
   written this list included `envs/` — wrongly: the same repo's
   `.gitignore` line 3 forbids env values from ever being committed,
   so "promote from a commit" was unsatisfiable for them by design.
   Found by ranger-base-h56a; ruled in §7.)*
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
| constitution repo (the instance tree) | PIDs, config, recipes, skills — plus its own docs/ and scripts/ | ordinary project work — **drafting is open to personas** | `posse promote` (new) |
| runtime home `~/.config/posse` | promoted constitution copy · `state/` · `envs/` (secrets, §7) · `personas/` (memory) | see §2/§5/§7; never "promoted", never draft | n/a — it *is* the live surface |
| queue repo (new, §4) | `.beads` store of record | every session, always, via redirect | n/a — data, not prose |

The split the operator asked for is therefore mostly **not a repo
move**: posse is already separate, the constitution repo already
exists. What moves is the queue out (§4) and the live surface off the
symlink (§2).

**2. The home becomes the promoted copy.** `~/.config/posse` — the
path the binary already prefers (**MEASURED**, `app.go`) — is created
as a *real directory*, not a symlink. `agents/`, `config.yaml`,
`recipes/`, `skills/` under it — the **promoted set** — are written
**only** by `posse promote` (`envs/` is deliberately not in this set;
§7). The `~/.config/rhq` symlink dies with the rhq
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

- **Preconditions** *(amended 2026-08-26, ranger-base-yb9j — as first
  ratified this read "the working tree is clean", whole-tree)*: the
  promoted ref is a commit, and the **promoted paths** — `agents/`,
  `config.yaml`, `recipes/`, `skills/` — are clean: `git status
  --porcelain --ignored=matching` over those pathspecs is empty, so a
  promoted path that is dirty *or gitignored* is a hard refusal (an
  ignored path has no commit to promote from — the h56a `envs/` shape,
  generalized to catch the next path that grows it). Anything else
  dirty in the repo is a warning naming the paths, never a block.
  Whole-tree clean is unsatisfiable by this ADR's own carve-outs:
  `.beads` (§4 — bd rewrites `issues.jsonl` continuously until the
  queue moves out, and the redirect file lives here after) and
  `personas/` (§5 — memory, appended at session end, deliberately
  unpromoted) are dirty in ordinary operation, and neither is prose a
  promote puts in force; a gate that cannot be passed on the day of
  the window gets bypassed with a flag, which is worse.
  *(Re-attributed 2026-08-27, ranger-base-znma via ranger-base-70ci —
  as amended by yb9j this bullet ended by crediting the gate with the
  invariant: "the promoted bytes equal the bytes at the recorded SHA".
  Wrong, and znma is the proof: `git update-index --skip-worktree` or
  `--assume-unchanged` makes `git status --porcelain
  --ignored=matching` report a promoted path clean while its
  working-tree bytes differ from the blob. The gate enumerates what
  git will report; it cannot cover what git has been told not to
  report. It stays for the honest question it CAN answer — an
  uncommitted edit git does report is a hard refusal naming the path,
  because silently promoting the older committed bytes would be its
  own kind of lie. The invariant itself lives in the Mechanism bullet
  below.)*
- **Mechanism** *(added 2026-08-27, ranger-base-znma)*: promote does
  not read the constitution working tree's bytes at all.
  `promotedAtCommit` (`internal/rhq/promote.go`) reads the blobs at
  the recorded SHA — `git ls-tree -r -z` for the oids and modes, one
  `git cat-file --batch` for the bytes — and the manifest's sha256 is
  taken over those bytes. *This*, not the clean gate, keeps the
  property 5na asked for: the promoted bytes equal the bytes at the
  recorded SHA, so uncommitted prose can never be in force. The
  invariant is structural, not gated. Two facts the mechanism carries:
  a promoted file's mode at the home is git's — 0644, or 0755 for a
  committed exec bit; git records no other — and `git archive` is
  explicitly *not* the tool, because export-subst and eol filters
  would rewrite bytes the manifest attests to. **Scope, said
  plainly** (ranger-base-echz): this makes the promoted *bytes* equal
  the bytes at the SHA; it does not yet make the promoted *set* equal
  the set at the SHA. `promotePathspecs` still stats the working tree
  to decide which promoted paths to read, so a promote can record a
  full SHA over a strict subset of it — one `git sparse-checkout` in
  the constitution repo takes ratified prose out of force under a SHA
  that still carries it, manifest born matching. That half is
  ranger-base-70ry; until it lands, §3's invariant claims per-file
  bytes, not set completeness.
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

**7. Env sets are state, not law** *(amended in 2026-08-26,
ranger-base-h56a — as first ratified, §1/§2 put `envs/` in the promoted
set)*. `envs/` leaves the promoted set, the manifest, and the launch
verify. It lives at the runtime home as operator-owned, machine-local
secret state — the same class as `state/`, gated like §5's memory is:
by who can write it, not by ratification.

Why this is the correct classification and not a coverage concession,
each point **MEASURED** 2026-08-26:

- Git is *forbidden* to hold env values — the constitution repo's own
  `.gitignore` line 3, with a comment stating the policy ("Secrets
  never enter git, even in a private repo"). "The promoted ref is a
  commit" and "ratify the diff" are therefore unsatisfiable for
  `envs/` *by design*, and the design that forbids it is right.
- Env sets are runtime-mutable, not drafted prose: `WriteEnvSet` /
  `EnsureEnvSet` / `DeleteEnvSet` (`envs.go`) edit them **at the
  home**, via the TUI and `$EDITOR`. Had the launch verify hashed
  them, every routine credential edit would have broken the manifest
  and refused dispatch until a re-promote that cannot even see the
  values.
- The controls the promoted set gets from the manifest, `envs/`
  already has in kind: `TightenEnvPerms` re-asserts 0700/0600 on
  **every** env read, printing what drifted (`envs.go`,
  rangerhq-f2b); and the seatbelt writable set includes `state/` but
  never `envs/` (`seatbelt.go:122`), so for caged sessions the same
  wall that covers the promoted copy covers the secrets.
- `posse init` already seeds `envs/` from embedded examples with the
  modes set (`init.go`) — the fresh-box path never needed promote.

Two operational clauses o943 builds either way:

- **Promote never creates, copies, or touches `home/envs`.** A copy
  path that widens 0600 publishes tokens to every process of this
  user; the cheapest copy that cannot widen modes is the one that
  does not exist.
- **Promote warns on a dangling `default_env`**: after copying
  `config.yaml`, if `default_env:` names an env set absent from
  `home/envs`, say so (names only, never values). This is the
  tripwire for exactly the failure h56a found — an instance coming up
  on the far side of the window with no env sets and no error.

The cutover consequence, in the runbook not just here: today the four
live env files exist only because the symlink makes the constitution
repo *be* the home. After §2 nothing carries them. The window gains an
explicit carry step — copy with modes preserved, verify, then delete
the originals from the constitution working tree so live tokens stop
sitting in a repo sessions get dispatched into
(`docs/runbooks/home-cutover.md`).

What is priced away: "the promoted set is the constitution" was
exactly true and now has one named exclusion beside §5's, and the
manifest covers one fewer directory. The successor design for the
values themselves — provider seam, `posse refresh`, expiry — is
ranger-base-x6ic, and this ruling feeds it: the credential store is
rooted at the home, not the constitution.

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
the queue. One window, two moves, zero moves later. The home half of
the window includes the §7 env carry (`docs/runbooks/home-cutover.md`)
— it is a step in the runbook *before* the window opens, not a
discovery inside it. Nothing in this
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
- **`envs/` promoted with a non-git anchor** (h56a shape b): promote
  hashes the constitution working tree's env files into the manifest,
  no SHA. Keeps verify coverage on paper; in practice it makes the
  constitution *working tree* the permanent store of secret values —
  the exact dual-home confusion the cutover ends — and every
  legitimate home-side edit (`WriteEnvSet` is a live TUI path,
  **MEASURED**) becomes a dispatch refusal until an operator
  re-promote. A verify that fires on routine correct behaviour trains
  everyone to ignore it.
- **A tracked names-only envs MANIFEST** (h56a shape c): diffable
  structure in git, values 600 and local. Honestly attractive, and
  rejected on price, **ASSUMED** not measured: the binding structure
  it would track already exists in tracked files (`config.yaml
  default_env:`, recipe env references, `CageCred` naming
  `container.env` in code), so it is a second copy that drifts; and
  ranger-base-x6ic is about to redesign the value store behind a
  provider seam — landing a manifest format in the same window as the
  design that may obsolete it violates the operator's one-migration-
  per-window rule. If x6ic wants it, x6ic adds it.
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
4. `posse promote` with a dirty **promoted path**: refused, names the
   paths; with a gitignored promoted path: refused; with only `.beads`
   or `personas/` dirty: proceeds and prints the not-blocking note
   naming them. Under a persona env marker: refused. From a PID:
   `Bash(posse promote:*)` denied at L1 (shim refusal logged).
5. Under seatbelt with cwd elsewhere: write to `home/agents` refused;
   write to own `home/personas/<self>` allowed; write to another
   persona's memory dir refused. *(Verified live 2026-08-27, twice
   independently: at the close of ranger-base-cpyb — e9320bc, the §3
   profile change — and on a fresh probe home outside every temp root,
   ranger-base-0djg, including §5's symlink shape under both
   spellings. Pinned hermetically in
   `internal/rhq/seatbeltconstitution_qa_test.go`.)*
   **Boundary — "cwd elsewhere" is load-bearing, not incidental**: a
   `cage: seatbelt` PID that does *not* deny Edit/Write, dispatched
   with cwd = the repo the home is symlinked into, gets cwd whole and
   with it the constitution writable (**MEASURED** 2026-08-27,
   ranger-base-h15). `posse gates` names every such grant, but the
   verdict is print-only — nothing on the launch path consults it.
   The live fleet is clean today only because the sole seatbelt
   persona denies Edit/Write. The fix for that mode is h15's trailing
   deny, not this item; after §2 the exposed surface shrinks to
   sessions dispatched into the constitution repo, where drafts are
   writable by design and §5's surviving `personas/` symlink is the
   remaining non-draft content in cwd's reach.
6. Queue after the move: `bd ready` / `bd comments add` / `bd close`
   work from a project worktree, from the constitution repo, and from
   a session worktree — all through redirects; the constitution
   repo's `.git` is in no session's writable set (`posse gates`
   prints the set). A bead close produces a jsonl commit in the queue
   repo and no push. *(Unverified until the cutover rehearsal, §4.)*
7. Envs after the window (§7): `~/.config/posse/envs` is 0700, each
   `.env` 0600, and `posse` lists the same four set names the old home
   had; `~/src/ranger-base/rhq/envs/` holds no `.env` files;
   `home/envs` appears in **no** entry of the manifest; corrupting an
   env file does *not* trip the launch verify (it is out of scope by
   design); with `default_env:` naming a missing set, promote prints
   the warning. *(Unverified until o943 is built and the window runs.)*

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
| env values are gitignored in the constitution repo, tracked=0 on-disk=4 | **MEASURED** 2026-08-26 (`git check-ignore -v`, h56a) |
| env sets are runtime-mutable at the home (TUI/`$EDITOR` write paths) | **MEASURED** (`envs.go` WriteEnvSet/EnsureEnvSet/DeleteEnvSet) |
| every env read re-asserts 0700/0600 | **MEASURED** (`envs.go` TightenEnvPerms, called from EnvSetVars) |
| seatbelt writable set includes `state/` but never `envs/` | **MEASURED** (`seatbelt.go:122`) |
| `posse init` seeds envs from embedded examples with modes set | **MEASURED** (`init.go`) |
| the names-only envs manifest would only duplicate tracked bindings | **ASSUMED** — priced in Alternatives; x6ic re-opens it if wrong |
| whole-tree clean is unsatisfiable in the live constitution repo (`.beads/issues.jsonl` and `personas/…/ORDERS.md` dirty in ordinary operation) | **MEASURED** 2026-08-26 (`git status --porcelain`, the hour o943 was built; recorded on ranger-base-yb9j) |
| verification item 5 (cwd elsewhere: constitution refused, own memory allowed, another's refused) under the built §3 profile | **MEASURED** 2026-08-27, twice (cpyb close under sandbox-exec; 0djg fresh probe home) · pinned in `seatbeltconstitution_qa_test.go` (7 tests: 5 from cpyb, 2 from a86ec3f) |
| a seatbelt PID not denying Edit/Write, cwd covering the home, holds the constitution writable; the `posse gates` verdict is consulted by nothing on the launch path | **MEASURED** 2026-08-27 (ranger-base-h15, laurie's probe; sole caller of ConstitutionGrants is `cmd/posse/main.go:960`) |
| `update-index --skip-worktree` (and `--assume-unchanged`) defeats the §3 clean gate: status reports the promoted path clean while its working-tree bytes differ from the blob | **MEASURED** 2026-08-27 (ranger-base-znma repro) |
| promote reads blobs at the SHA (`promotedAtCommit`: `ls-tree -r -z` + one `cat-file --batch`, which applies no smudge, no eol, no export-subst); the manifest sha256 is over those bytes | **MEASURED** 2026-08-27 (`internal/rhq/promote.go`, znma fix; runbook `docs/runbooks/home-cutover.md` agrees) |
| the promoted SET is still decided by a working-tree `os.Stat` (`promotePathspecs`); a sparse-checkout shrinks the set under a full SHA with the manifest born matching | **MEASURED** 2026-08-27 (ranger-base-echz hermetic repro → ranger-base-70ry, P1 in progress) |
