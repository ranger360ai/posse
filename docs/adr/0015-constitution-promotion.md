# ADR 0015 — Promote the constitution: three trees, three write policies, one ratification step

*Status: proposed 2026-08-26 · owner: architect · ratified 2026-08-26
(ranger-base-ap2x) · amended 2026-08-26: §7 moves `envs/` out of the
promoted set (ranger-base-h56a) · amended 2026-08-26: §3's clean gate
is the promoted paths, not the whole tree (ranger-base-yb9j, built in
ranger-base-o943) · amended 2026-08-27: verification item 5 verified
live, marker dropped, cwd-elsewhere boundary clause added
(ranger-base-4v1d) · amended 2026-08-27: §3's invariant re-attributed —
the commit read, not the clean gate, keeps bytes == SHA
(ranger-base-znma via ranger-base-70ci; the set half landed in
ranger-base-70ry) · execution rides with the rhq
retirement (ranger-base-3rv9, operator-ruled 2026-08-25) · amended
2026-08-29: §3's fence widens to bd's destructive/egress verbs
(ranger-base-u9ud, from ranger-base-3bqn) · amended 2026-08-29: §3's
hook denies narrow to install/uninstall — the whole verbs refused
beads' own git hooks and no PID carrying them could commit
(ranger-base-i6do, operator-ruled on ranger-base-y5g7) · amended
2026-08-29: §3's
actor split gains a third and fourth spelling, at the commit and at the
land (ranger-base-ak3e, from ranger-base-7pq0; recorded by
ranger-base-ubtc) · amended 2026-08-31: §3's launch-verify claim
tier-conditioned — an ABSENT or re-stamped anchor defeats detection at
shims, accepted with reasons; anchor-state line added
(ranger-base-zio33, from ranger-base-bejb) · amended 2026-09-01: the
promoted set gains `runtimes/` — the per-key runtime overlay of ADR 0021
is read at every launch and was the one launch-read fact at the home no
manifest attested to (ADR 0039 D2, built in ranger-base-ight8) · amended 2026-09-02: the manifest records WHICH posse wrote it and
WHAT SET that posse walked, and the launch verify leads with the
promoted-set drift rather than a file — an older posse on PATH refused
every dispatched launch for ~90 minutes naming a file that was present
and hash-matched (ranger-base-39jnl) · informs
0002 §3, 0012 D3-C, 0014 §3, 0025 · amended 2026-09-02: the constitution directory in the instance repo is `posse/`, not `rhq/` — the cutover is complete and every historical `rhq/<p>` spelling below now means `posse/<p>` (ADR 0046 retired 2026-09-05; current source constant verified) · amended
2026-09-06: §3's invariant claims the promoted SET as well as the bytes —
ranger-base-70ry landed 2026-08-27 in b348799c and the record had gone on
calling that half open (ranger-base-rowut)*

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

1. **Constitution** — `posse/agents`, `config.yaml`, `recipes/`,
   `skills/`. Rare, deliberate, should be attributable. *(As first
   written this list included `envs/` — wrongly: the same repo's
   `.gitignore` line 3 forbids env values from ever being committed,
   so "promote from a commit" was unsatisfiable for them by design.
   Found by ranger-base-h56a; ruled in §7.)*
2. **Runtime state** — `.beads` (the queue, store of record for every
   repo the crew touches, reached by `.beads/redirect`) and
   `posse/state` (was `rhq/state`). The write-hottest paths in the system; deniable to
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
`recipes/`, `runtimes/`, `skills/` under it — the **promoted set** — are
written **only** by `posse promote` (`envs/` is deliberately not in this
set; §7). Runtime overlays are governed here on the same promotion terms as config: versioned constitution source, promoted home copy, manifest verification. Their merge and validation semantics live in [0013 §8](0013-runtime-dispatch-contract.md); 0021/0039 are lineage, not additional promotion authorities. The `~/.config/rhq` symlink dies with the rhq
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
  `config.yaml`, `recipes/`, `runtimes/` *(added 2026-09-01, ADR 0039
  D2)*, `skills/` — are clean: `git status
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
  `promotedAtCommit` (`internal/posse/promote.go`) reads the blobs at
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
  plainly** (ranger-base-echz, completed by ranger-base-70ry): the
  invariant covers the promoted *bytes* and the promoted *set*, and
  the commit decides both. The set half was the later half:
  `promotePathspecs` used to `os.Stat` each promoted path under the
  working tree and drop the ones the tree did not have, so a promote
  could record a full SHA over a strict subset of it — one
  `git sparse-checkout` in the constitution repo took ratified prose
  out of force under a SHA that still carried it, manifest born
  matching. That is ranger-base-70ry, landed 2026-08-27 in b348799c:
  the stat is gone, the pathspecs are always the whole promoted set,
  and the “nothing to promote” refusal moved to `promotedAtCommit`,
  which asks the commit.
- **Manifest**: promote records `{source repo path, git SHA, sha256
  per promoted file}` under the home *beside* the promoted copy (not
  under `state/`, which stays session-writable). It prints
  `git diff <last-promoted>..HEAD -- <constitution paths>` so the
  operator ratifies a diff, not a vibe.

  **Plus its own writer** *(amended 2026-09-02, ranger-base-39jnl)*:
  `posse` — `VersionString()` of the binary that wrote it — and `set`,
  that binary's `PromotedPaths`. Both are additive at manifest version
  1, so an older posse reads such a manifest exactly as before. They
  exist because the file list alone cannot tell "the home has no
  `runtimes/`" from "the posse that wrote this did not know `runtimes/`
  existed", and that ambiguity is a 90-minute fleet outage: a brew keg
  put a three-day-old release ahead of `~/.local/bin` on PATH, its
  `PromotedPaths` predated `runtimes` joining the set, and the launch
  verify refused every dispatched launch naming `missing
  runtimes/claude.yaml` — a file that was present, readable and
  hash-identical. `VerifyPromoted` now leads its line with the drift
  ("manifest written for promoted set […] by posse X; this binary (Y)
  walks […]") and the path classes follow it. The two directions are
  different findings and read differently: a manifest naming a root
  this binary does not walk can only be an OLDER posse answering, and
  says so; this binary walking *more* than the manifest was written for
  is the ordinary upgrade order and says *that*. The drift is a
  **diagnosis, not a verdict** either way: it never enters `OK()`,
  because refusing on the upgrade direction would take the fleet down
  on every release that widens the set.
- **Launch verify** (absorbs the shape of ranger-base-5na): every
  launch hashes the promoted set against the manifest. Dispatch
  **refuses** on mismatch; an interactive launch warns DEGRADED. The
  fix for a mismatch is one command (re-promote), so fail-closed is
  cheap. At shims this is detection, not prevention — said plainly:
  promotion is a fence in Yegge's sense, and the wall that does exist
  (seatbelt never grants the home's constitution area; only
  `home/state` and the persona's own memory dir) covers the promoted
  copy for every caged session.

  **The anchor itself, tier-conditioned** *(amended 2026-08-31,
  ranger-base-zio33, from ranger-base-bejb — the sentence above was
  honest about modification and silent about the anchor)*. The
  manifest is a trust anchor stored inside the tree it attests, and an
  ABSENT manifest is by design not a mismatch: `ReadPromoteManifest`
  reads a missing file as "never promoted — OK", because every
  pre-0015 home and every `RHQ_HOME` test rig must keep launching
  (pinned, `TestVerifyPromotedIsSilentWithNoManifest`). So the
  detection claim splits by tier, and is now conditioned rather than
  universal:

  - At **seatbelt**, the anchor is walled: `HomeConstitutionPaths`
    includes `promoted.json`, so a caged session can neither modify
    the promoted copy nor remove the thing that would notice.
  - At **shims**, detection covers *modification* and is loud there —
    a changed byte, a truncated manifest, an unpromoted file all
    DEGRADE or refuse (**MEASURED**, ranger-base-bejb cases A/B). A
    session that *deletes* the anchor, or re-stamps it with its own
    tampered file's sha256, silences detection with no output on any
    surface (**MEASURED**, cases C/D, one command each). At shims the
    manifest is a fence against accident and drift; against a session
    that means it, it is a speed bump, and this ADR says so instead
    of fixing it.

  Why accepted rather than fixed: shims applies no sandbox — the
  session holds the operator's uid whole (ADR 0002 §3) — so **every
  candidate location for a second anchor is session-writable in
  exactly the tier the anchor would serve**: operator config, launcher
  state, the constitution repo, a user-launchd path — all the same
  trust domain. A second same-domain anchor converts one `rm` into
  two and adds a consistency obligation whose failure mode is a false
  refusal on the unattended fleet (the ranger-base-h7cd/rgx0 class,
  already paid once when init over-stamped homes). And case D
  subsumes case C: a session that would delete the anchor can instead
  re-stamp it, which no presence rule, doctor line, or same-domain
  second anchor catches. Adversary-proof anchoring requires leaving
  the uid — the shapes Alternatives already rejects. What *is* bought
  (ranger-base-zio33's one mechanism): a read-only **anchor-state
  line** on the operator surfaces that already sweep launch health at
  the watch preamble (beside `ReportHookWall`) — `promoted <sha>
  <date>` / `seeded <date>` / `never promoted — no
  promoted.json` — so an *accidental* deletion (cleanup script,
  botched restore) is visible at the operator's next touch point
  instead of never. It changes no launch behavior, refuses nothing,
  and claims nothing against a deliberate session (a re-stamped home
  reads "promoted").
- **Fence, spelled twice** (ORDERS rule): every crew PID gains
  `deny: [Bash(posse promote:*)]` — an L1-realizable named verb on all
  runtimes — and `promote` itself refuses to run when the persona env
  marker is set. Both are politeness against a determined session;
  the manifest check is what notices — at seatbelt unconditionally, at
  shims only while the anchor stands (the launch-verify bullet's
  tier-conditioning, ranger-base-zio33).
- **The fence widens to bd's destructive and egress verbs** *(amended
  2026-08-29, ranger-base-u9ud, from ranger-base-3bqn/az93)*. This
  bullet is the precedent being reused, not the subject: a shipped PID
  `deny:` set is an ADR-level decision, not a devops close's to edit,
  and the question this amendment answers is the bd analogue of the
  one above — **yes**, the verbs join it.

  Reason it needed an ADR line at all: `Bash(bd <verb>:*)` was
  measured unfaithful before this (ranger-base-az93). A Bash
  permission rule matches a **token prefix** of the typed line, and
  any of bd's four value-taking global options — `--actor --db
  --dolt-auto-commit --lock-timeout`, **MEASURED**: bd 0.49.1 has
  eighteen global options, not the seven first counted — placed before
  the verb moves it out of the prefix: `bd --no-daemon daemon --help`
  ran straight past `Bash(bd daemon:*)`. An allow-list posture
  (`deny: Bash(bd:*)` plus allow the safe verbs) does not fix it
  either — deny beats allow (ADR 0001), so the catch-all swallows every
  allow and kills `bd show`/`bd ready`/`bd close` fleet-wide. What
  fixes it is the mechanism this section already leans on: the L1
  PATH shim (`internal/posse/gates.go`, the same `posse_verb_match`
  machinery the `promote` rule above renders through) skips leading
  global options before matching the verb, and it already carried
  entries for `git` and `posse`. It now carries one for `bd`
  (`globalValueOpts["bd"]`, the four options above), so every
  `Bash(bd <verb>:*)` rule in a PID renders **option-aware** instead of
  best-effort — a reordered spelling resolves the same as the plain
  one, on every runtime the shim reaches (claude, grok, codex — not
  claude alone, unlike a Claude-only hook).

  The rule list (from `bd --help` on 0.49.1; staged and measured
  ranger-base-az93/3bqn):

  `Bash(bd daemon:*)` · `Bash(bd daemons:*)` · `Bash(bd admin:*)` ·
  `Bash(bd delete:*)` · `Bash(bd doctor:*)` ·
  `Bash(bd hook install:*)` · `Bash(bd hook uninstall:*)` ·
  `Bash(bd hooks install:*)` · `Bash(bd hooks uninstall:*)` ·
  `Bash(bd import:*)` · `Bash(bd init:*)` ·
  `Bash(bd migrate:*)` · `Bash(bd rename:*)` ·
  `Bash(bd rename-prefix:*)` · `Bash(bd repair:*)` ·
  `Bash(bd repo:*)` · `Bash(bd federation:*)` ·
  `Bash(bd config set:*)` · `Bash(bd config unset:*)` ·
  `Bash(bd dep relate:*)` · `Bash(bd relate:*)` ·
  `Bash(bd sync --full:*)` · `Bash(bd jira:*)` · `Bash(bd linear:*)` ·
  `Bash(bd setup:*)`

  Shape: daemon lifecycle, delete, doctor, hook(s) install/uninstall,
  import, init, migrate, rename(-prefix), repair, repo, federation,
  config set/unset, and relate rewrite or remove store state; `sync --full`
  is the one `sync` spelling that commits and pushes rather than
  reading; `setup` writes agent instruction files, a second promotion
  surface this ADR did not create; `jira`/`linear` are egress, kept
  denied on their own terms — hard risk line 4 (visibility), per
  hoover on ranger-base-3bqn: `--push` moves the whole private store,
  closed RCAs and credential-location names included, to an external
  tracker, and they stay off any future allow-list even though no
  credential exists to egress with today (`bd config set` is itself
  denied, and the `JIRA_`/`LINEAR_` env vars are absent). This list is
  additive to each PID's existing `Bash(bd:*)` allow — deny wins (ADR
  0001) — not a replacement for it.

  **The two hook rules name a subverb, and must** *(amended
  2026-08-29, ranger-base-i6do; operator-ruled on ranger-base-y5g7
  and executed by the operator's own hand across all eleven PIDs,
  e6518ff)*. They shipped as the whole verbs
  `Bash(bd hook:*)` and `Bash(bd hooks:*)` and had to be narrowed
  within the day. An L1 shim sits on `PATH`, so it is matched by every
  `execve` of `bd` — not only the ones a persona types. beads' own
  installed git hooks exec `bd hook pre-commit`, `bd hook
  post-checkout`, `bd hook post-merge`, `bd hooks run pre-push` and
  `bd hooks run prepare-commit-msg`. The whole-verb rules therefore
  refused beads' hooks, the hooks exited non-zero, and git aborted:
  **eleven PIDs could neither commit nor check out** in any repo where
  bd had installed hooks, which is every posse worktree, since
  worktrees share the main checkout's hooks dir. Measured from inside
  the failure by three personas — the refusals, one per spelling, are
  quoted verbatim in ranger-base-y5g7's comments; it also took
  `git worktree add`, and with it `scripts/release-artifacts.sh`, down
  (recorded on ranger-base-i312, with the stranded-work consequence on
  ranger-base-c7ek). `--no-verify` is not a
  workaround — git runs `prepare-commit-msg` regardless, which is the
  property the constitution wall in that slot relies on, so the flag
  only moves the refusal from the singular rule to the plural one.

  The hazard the whole verb was reaching for is install/uninstall,
  which the four rules above still deny; the run-time slots are a
  closed set git itself defines. Narrowing costs nothing a persona
  could otherwise reach, because the layer that sees what a persona
  *types* is a different one: `.claude/settings.json` and
  `scripts/bd-argv-gate.py` both match a Bash tool call by its command
  **text**, they keep the whole-verb spelling, and the argv gate is an
  allow-list — so it walls every spelling of the typed verb, including
  ones an enumerated L1 list cannot name. L2 covers typing; L1 must
  stay narrow precisely because it also sees what git spawns.

  This is pinned twice, and neither pin is optional. Content:
  `scripts/verify-pid-deny-set.sh` carries the whole-verb spellings in
  a `FORBIDDEN` list, because a PID that keeps them *alongside* the
  narrow four satisfies every presence check and still cannot commit.
  Behavior: `bdhookcommit_qa_test.go` renders each shipped PID's real
  deny set, puts the gates dir **first** on `PATH` in a scratch repo
  carrying beads' hook shims, and requires a path-limited commit and a
  branch checkout to land. That PATH ordering is the whole point —
  every renderer test in `internal/posse` drops the gates dir from the
  child's `PATH`, which is why none of them could see this.

  **Known residual, stated rather than assumed away.** The L1 shim
  renders on `PATH`; it never sees `bd` reached by an absolute path
  (`/Users/.../bd daemon stop`). That half is not this ADR's to close.
  Second residual, accepted with the narrowing above: a persona can now
  *type* `bd hook post-merge`, which imports JSONL over the db, on any
  box where the argv gate is not installed. That is a mutation beads'
  own hooks perform on every pull, and it is the price of the fleet
  being able to commit at all.
  `scripts/bd-argv-gate.{sh,py}` — a PreToolUse hook the operator may
  install, not one posse renders (ADR 0014 §5 unamended) — is the
  answer for that spelling on claude, and its allow-list posture also
  closes a class the PID deny-list structurally cannot: a hidden verb
  (`bd daemons`) no one has enumerated yet. Both layers are
  **cooperative**, never `enforced`, in ADR 0025's vocabulary — held
  in-process, defeated by an emptied environment, an alias, or a
  script that calls bd — never a cage. The wall against a session that
  means it is the L2/L4 boundary, unchanged by this amendment.
- **The fence gains a third and fourth spelling — at the commit and at
  the land** *(amended 2026-08-29, ranger-base-ak3e, from
  ranger-base-7pq0; recorded by ranger-base-ubtc)*. The two spellings
  above fence the `promote` COMMAND, and both are politeness.
  ranger-base-7pq0 measured what that leaves open: 9dfbbd4 committed
  all eleven `rhq/agents/*.md` crew PIDs from an uncaged persona
  session and nothing refused — under shims there was no path-class
  check anywhere, so §2/§3's "drafting is open to personas, promotion
  is the operator's" held at the taking-effect path and not at the
  commit a promote then reads. Two more spellings now sit on the
  commit's own path:

  *Third — the commit wall's constitution-path arm* (ADR 0002 §3, the
  `prepare-commit-msg` slot; mechanism record in
  `constitutionGuardBody`, `internal/posse/gates.go`). When
  `RHQ_PERSONA` is set — the same marker promote's own refusal reads —
  a commit whose to-be-committed set touches `posse/<p>` for each §1
  promoted-set entry plus `posse/envs` (in the constitution repo), or
  `.claude/settings.json` / `.claude/settings.local.json` (in every
  hooked repo — not this ADR's promoted set, but the same class by
  blast radius: az93's deny list lives there, and a persona that can
  commit it un-fences itself), is refused with the paths named and the
  way through prescribed. The class is rendered FROM `PromotedPaths`
  at hook-render time, so widening §1's list widens this wall in the
  same edit. Cooperative class (ADR 0025): `env -i` scrubs the marker
  and the arm stands down, the same residual promote's own env check
  carries — it is a wall against the ordinary path, not a cage.

  *Amended 2026-09-01 (ranger-base-dmsbu, from ranger-base-qdxe and
  ranger-base-jex3, both measured).* Two ways the arm stood down that
  were not residuals but defects, and are closed. **The staged set is
  read with move detection off:** `--name-only` prints only a rename's
  DESTINATION, so a move of a class path OUT of the class showed the arm
  nothing and committed at exit 0 — the PID gone from the constitution
  repo, and the same shape carried `.claude/settings.json` away in every
  hooked repo. It needed no `git mv`; detection pairs any similar staged
  delete with a staged add. **And the class marker is no longer read
  from the persona's own working tree alone:** the detector stat'd
  `posse/agents` on disk (the marker, ADR 0046), which a session owns, so `rm -rf` it — never
  staged, so nothing on the branch recorded it — and the promoted set
  dropped out of the class. It is now read from the working tree OR the
  base tree the commit is diffed against, which keeps repo genesis (no
  HEAD, marker on disk) working unchanged. The land-path belt below
  compensated for both, which is why they were hardening and not a live
  hole in the promoted constitution; the L3 arm now holds on its own.

  *Fourth — the launcher's land-path belt*
  (`MergeSessionWork`, `internal/posse/worktree.go`). The launcher
  refuses to fast-forward a session branch whose merge-base..HEAD
  diff touches the same class (one class, two readers:
  `ConstitutionClassIn`, so belt and wall cannot drift apart). It runs
  operator-side, under the launcher lock, about a branch already
  written — a session cannot scrub an environment it is not in — and
  it repairs nothing: the branch keeps every commit and the refusal
  names the diff and the by-hand landing. Coverage is what crosses a
  land: dispatched worktrees, not shared-checkout commits.

  What this sharpens in §2, said plainly: drafting stays open — the
  working tree stays writable, the refusal resets nothing — but
  *committing the class paths* is now the operator's act too, one step
  earlier than promotion. A persona's constitution draft goes under a
  path outside the class (the az93 route the refusal prints), and the
  operator applies it and promotes. The manifest remains the detector
  at every tier against accident and drift — with the anchor caveat
  the launch-verify bullet now carries (a shims session can delete or
  re-stamp what detects, ranger-base-zio33); these spellings move the
  first refusal from taking-effect to the commit.

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
sitting in a repo sessions get dispatched into (the home-cutover
runbook, moved to the instance tree by ADR 0024 D4 as a
one-deployment procedure).

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

## Lineage

| Was | Here |
|---|---|
| 0021 promotion amendment and 0039 D2 | §§2–3: `runtimes/` is promoted, removed when absent from the source commit, manifested, and fenced from persona ratification |
| 0046 completed rename | This ADR owns the current `posse/` directory contract; 0046 retains the retired cutover evidence |

## Sequencing

Rides with ranger-base-3rv9, per the operator's ruling. The
retirement step "retire the `~/.config/rhq` symlink" becomes "first
`posse promote` creates `~/.config/posse`", and the same window moves
the queue. One window, two moves, zero moves later. The home half of
the window includes the §7 env carry (the home-cutover runbook, moved
to the instance tree by ADR 0024 D4) — it is a step in the runbook
*before* the window opens, not a discovery inside it. Nothing in this
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
- **An external anchor for the manifest** (ranger-base-bejb option 1:
  record "this home was promoted" outside the home — operator config,
  the constitution repo, a launchd-owned path). Rejected 2026-08-31,
  ranger-base-zio33. Every same-uid location is writable by the shims
  sessions it would anchor against, so it moves the problem rather
  than fixing it (bejb's own words) and adds a two-anchor consistency
  obligation whose failure mode is a false refusal on the unattended
  fleet (the h7cd class). The variants that would hold — a root-owned
  path, a second OS user, a signature whose key the session cannot
  read, a remote — are the alternative above plus key management this
  fleet does not have, bought for one file in a tier whose design says
  no wall exists or is coming (ADR 0002 §3), while the same session
  can still edit the promoted PIDs, shadow `posse` on PATH, or scrub
  the env marker. Also rejected, same grounds: **arming absence**
  ("once ever-promoted, a missing manifest refuses") — the arming bit
  is itself same-domain state, the (nil, nil) branch is load-bearing
  three ways (pre-0015 homes, `RHQ_HOME` test rigs, the pinned test),
  and re-stamping (case D) walks past any presence rule anyway. The
  posture is the tier-conditioned claim plus the anchor-state line,
  both in §3.
- **Leave the bd verbs as a hand-maintained `.claude/settings.json`
  deny list** (the status quo this amendment replaces, az93's
  rai0/3bqn work). Claude-only, per-repo rather than per-PID, and
  proven evadable by the same token-prefix gap the shim fix closes —
  keeping the rule there and not in the PID leaves grok and codex
  fenceless and reopens the reordering hole every time the file is
  hand-edited instead of rendered.

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
   `internal/posse/seatbeltconstitution_qa_test.go`.)*
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
   had; `$CONSTITUTION/rhq/envs/` holds no `.env` files;
   `home/envs` appears in **no** entry of the manifest; corrupting an
   env file does *not* trip the launch verify (it is out of scope by
   design); with `default_env:` naming a missing set, promote prints
   the warning. *(Unverified until o943 is built and the window runs.)*
8. Every example PID in `examples/agents/` carries the full bd rule
   list from the amendment above, in addition to its existing
   `Bash(bd:*)` allow. `posse gates <p>` for one of them: each
   `Bash(bd <verb>:*)` rule renders `option-aware` (not
   `best-effort`), and `bd --db /tmp/x daemon stop` is refused the
   same as `bd daemon stop` — the reordered spelling and the plain one
   resolve to the same verb. Pinned in `internal/posse/bdshim_test.go`
   (ranger-base-3bqn: emptying `globalValueOpts["bd"]` fails both the
   refuse arm and, separately, the `--actor daemon show x` pass arm).

   The one rule in the list that names a FLAG rather than a verb,
   `Bash(bd sync --full:*)`, is verified separately and differently: a
   flag has no position, so `bd sync --push --full` and `bd sync
   --dry-run --full` must be refused exactly as `bd sync --full` is,
   `--full=true` with them. Both ran past the rule until
   ranger-base-vct2; the shim now matches the flag anywhere in `sync`'s
   own arguments, and `posse gates <p>` renders that rule
   `option-aware, flag anywhere in the segment`. Pinned in the same
   file, both ways — the refuse arms above, and the pass arms where the
   word only looks like the flag (`bd sync -m --full` is a commit
   message, `bd sync -- --full` an operand).

9. The anchor-state line (§3, ranger-base-zio33): with a promoted
   home, the watch preamble prints `constitution: promoted <sha>
   <date>`; on a freshly seeded home, `constitution: seeded <date>`;
   after `rm $RHQ_HOME/promoted.json`, `constitution: never promoted —
   no promoted.json` — and the launch behavior of all three states is
   unchanged (absence still launches; item 3's mismatch still
   DEGRADEs/refuses). *(Built 2026-09-05, ranger-base-xevp7:
   internal/posse/anchorstate.go, called once from the watch preamble
   beside `ReportHookWall`. Pinned in
   internal/posse/anchorstate_qa_test.go — the bytes of each state,
   exactly one such line on the preamble, and both halves of the
   "changes nothing" clause: absence verifies OK while the line says
   `never promoted`, and a tampered home still fails VerifyPromoted
   while the line says `promoted`. The C/D silences themselves are
   MEASURED — ranger-base-bejb repro at 12ce5be.)*

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
| verification item 5 (cwd elsewhere: constitution refused, own memory allowed, another's refused) under the built §3 profile | **MEASURED** 2026-08-27, twice (cpyb close under sandbox-exec; 0djg fresh probe home) · pinned in `seatbeltconstitution_qa_test.go` (7 tests: 5 from cpyb, 2 from eb8f716) |
| a seatbelt PID not denying Edit/Write, cwd covering the home, holds the constitution writable; the `posse gates` verdict is consulted by nothing on the launch path | **MEASURED** 2026-08-27 (ranger-base-h15, laurie's probe; sole caller of ConstitutionGrants is `cmd/posse/main.go:960`) |
| `update-index --skip-worktree` (and `--assume-unchanged`) defeats the §3 clean gate: status reports the promoted path clean while its working-tree bytes differ from the blob | **MEASURED** 2026-08-27 (ranger-base-znma repro) |
| promote reads blobs at the SHA (`promotedAtCommit`: `ls-tree -r -z` + one `cat-file --batch`, which applies no smudge, no eol, no export-subst); the manifest sha256 is over those bytes | **MEASURED** 2026-08-27 (`internal/posse/promote.go`, znma fix; the home-cutover runbook — moved to the instance tree by ADR 0024 D4 — agrees) |
| the promoted SET was decided by a working-tree `os.Stat` (`promotePathspecs`); a sparse-checkout shrank the set under a full SHA with the manifest born matching | **MEASURED** 2026-08-27 (ranger-base-echz hermetic repro → ranger-base-70ry) · FIXED 2026-08-27 in b348799c; the commit decides the set, pinned in `promotegate_qa_test.go` |
| pre-ak3e, a persona session could commit the entire promoted set with nothing refusing (9dfbbd4: all eleven crew PIDs) | **MEASURED** 2026-08-29 (ranger-base-7pq0, verified at HEAD) |
| the commit wall's constitution arm refuses each class member persona-marked, passes the identical commit unmarked and a persona commit off the class; the land belt refuses per class member, mutation-checked | **MEASURED** 2026-08-29 (ranger-base-ak3e pins: `internal/posse/constitutionwall_qa_test.go`, `internal/posse/constitutionland_qa_test.go`) |
| a shims session silences the launch verify by deleting `promoted.json` or re-stamping it with its tampered file's sha256, no output on any surface; a truncated manifest is loud | **MEASURED** 2026-08-28 (ranger-base-bejb repro at 12ce5be, cases A–D) |
| 10 of the 11 live PIDs declare no `cage:` and run at shims; only hoover declares `cage: seatbelt` | **MEASURED** 2026-08-28 (ranger-base-bejb, `~/.config/rhq/agents`) |
| every same-uid location for a second anchor is session-writable at shims | by definition of the tier (ADR 0002 §3: shims applies no sandbox, so file access is the uid's); the exceptions are the out-of-uid shapes Alternatives rejects |
