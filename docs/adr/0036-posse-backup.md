# ADR 0036 — `posse backup`: the queue store's backup is a harness verb

*Status: accepted 2026-08-29 · bead ranger-base-nbcf · richard*

## Context

The queue repo (`queue_repo:`, ADR 0015 §4) is the store of record for
the whole work graph, and by the operator's 2c ruling (ranger-base-xhsb)
it never grows a git remote — which makes "the launcher cannot push the
queue" structural, and makes one disk failure the whole graph and its
journal. A first mechanism shipped as instance ops scripts plus a
launchd plist (ranger-base-hl2p): age-encrypted archives, retention, a
restore drill as the deliverable, a disk-pressure floor. It worked when
run, and its scheduling never got armed — the plist was never installed
(ranger-base-2ndy). The operator's directive supersedes that whole
arrangement: the backup becomes a first-class `posse backup` verb, not a
script + plist + manual rsync, and the scripts are raw material, not a
contract.

Two operator rulings shape the design and are not re-litigated here:
encrypted backup yes, git remote never (2c); and the off-box cadence —
no cloud, no always-attached volume. The operator attaches an external
disk *as needed* and sweeps then, so the freshness model must
distinguish "on-box copy fresh" from "off-box copy as of the last
sweep" without nightly noise (ranger-base-2ndy).

The boundary that decides most of what follows: posse is public, the
instance's ops detail is not. The verb, the mechanism, and its defaults
ship in the binary for any deployer; destinations, intervals, and the
identity live in the instance home (`$RHQ_HOME` config and secrets),
exactly as `queue_repo:` itself already does (queuejsonl.go).

## §1 — Verb surface

One verb, six intents, none of which needs a herdr socket — a backup
tool that requires the fleet to be healthy is a backup tool that is
absent during the incidents it exists for.

| form | does |
|---|---|
| `posse backup` | build one encrypted archive on-box, then run the drill against it. Bare form = the whole scheduled duty; `--no-drill` is the escape, not the default — a schedule that only writes archives is a pile of files nobody has opened (hl2p). |
| `posse backup init` | mint the age identity + recipients under `$RHQ_HOME/secrets/backup/` (§5). Refuses if an identity exists. |
| `posse backup sweep` | copy archives + sidecars to `backup_dest:`, re-hash them there, stamp the sweep (§6). If the destination is not mounted: one line and a distinct exit — an absent disk is the normal state, not an error. |
| `posse backup status` | three facts: age of the newest on-box archive, the last sweep (when, where, what), the last drill verdict. Exit 0 healthy; nonzero only for on-box staleness or a red drill — never for off-box age (§6). |
| `posse backup drill` | restore the newest (or `--archive`) into scratch, run every arm including mutation, delete scratch, record the verdict. |
| `posse backup restore --to DIR` | real recovery: same engine as the drill, keeps the tree, skips the mutation arm, prints what was asserted. |

Everything an operator decides is config, not flags-remembered-nightly:
`backup_interval:` (the arm switch, §4), `backup_dest:`, `backup_keep:`
(on-box, default 3), `backup_keep_dest:` (default 0 = keep all),
`backup_min_free_mb:` (default 384 — derived from the MEASURED ~180MB
transient peak of a full cycle, hl2p), flat autostart_*-style keys.
Flags override per-invocation.

## §2 — The mechanism is absorbed into the binary

The engine is Go in internal/rhq, not a wrapper over the hl2p scripts.
Three reasons, each sufficient: the promote fence ships the binary and
nothing else, so a verb driving scripts ships half of itself; the
scripts live in the instance's private ops tree, which no other deployer
has and a public verb may not depend on; and the scripts already read
posse's own `queue_repo:` key — the knowledge wants to live where the
key does.

External surface after the fold:

- **git** — already a hard dependency of the harness. `git bundle
  --all` for the journal (~12MB vs 276MB loose, MEASURED hl2p).
- **sqlite3 CLI** — one call: the online backup API
  (`.backup` against a read-only source URI) to stage the live db.
  Preflighted; a missing tool is exit 2 naming it. A pure-Go sqlite
  driver was priced and rejected (Alternatives).
- **filippo.io/age (Go library)** — the one new go.mod dependency, for
  encrypt, decrypt, and keygen. Exit hatch, per the dependency rule:
  the age v1 format is an open spec with an independent reference CLI;
  every archive this verb ever writes decrypts with `age -d -i
  <identity>` and no posse binary at all. No state is held hostage.
- **compression is stdlib gzip**; zstd is dropped with the scripts.
  Price: archive grows from ~20MB toward ~25MB (ASSUMED — the bundle,
  the bulk, is already delta-compressed and near-incompressible either
  way; the implementation bead measures it and this line gets updated).

## §3 — Refusals, single-flight, publish, prune

Carried from hl2p verbatim, now enforced by the binary:

- **No remote, enforced not obeyed**: if the source repo has any git
  remote, refuse loudly (the 2c ruling as code). The drill asserts the
  restored tree has none — `git clone <bundle>` creates an origin and
  the engine removes it.
- **Disk floor**: refuse below `backup_min_free_mb:` free at the
  staging volume, saying why — degrade safely, never fill the disk.
- **Unset `queue_repo:`** refuses with ADR 0015 §4's line: the store
  has not moved, there is nothing to back up. Same inertness as the
  jsonl commit path — installing posse arms nothing.

New, because the verb now has two callers (operator-typed and the §4
ticker): a **flock single-flight lock** in the staging dir — ADR 0011
§1's answer, single-writer for ~30 lines and zero new state. Archives
are built under a temp name and **published by rename**, so sweep and
status never see a half-written archive.

Prune is reclamation and follows the discipline (skill:
safe-reclamation): on-box retention keeps the newest `backup_keep:`,
and a candidate is deletable only when a *newer* archive holds a green
drill verdict — the archive just written is never pruned before its own
drill passes, so a run that builds garbage cannot destroy the last good
copy on its way down. The destination is not pruned by default
(`backup_keep_dest: 0`): the sweep only adds, bounded per sweep by
on-box retention, and the operator owns that disk's economics.

## §4 — The clock: a ticker goroutine in the watch loop

Scheduling rides the one standing process the harness already owns —
`posse dispatch --watch` — as a **backupLoop goroutine on its own
ticker**, the pulse pattern exactly (ADR 0027): starts with Watch's
ctx, dies with it, disarmed unless `backup_interval:` is present in
config.

Naming the clock, because ADR 0028 taught that a duty parked "on the
pass" starves when a rolling Run refills for hours (ranger-base-ad4y):
the backup's clock is **its own goroutine's ticker**, the same lifetime
and independence the pulse has — not the pass, not the Run's return,
not the epoch tick. `posse pause` does not stop it: pause stops
dispatching, and the queue still mutates in a paused shop.

Each tick is level-triggered against durable state, so loop restarts
never double-run: if the newest on-box archive is older than
`backup_interval:`, run archive + drill; then, if `backup_dest:` is
set *and mounted*, sweep opportunistically. That last clause is the
cadence ruling made mechanical: the operator attaches the disk, and
the next tick sweeps without anyone typing anything; detached is a
silent skip, not a warning.

launchd is rejected again, for backup's own reasons this time (the
scheduled-dispatch rejection, rangerhq-snd, leaned on herdr, which
backup does not need): the directive is *one* arrangement, not a verb
plus a sidecar plist; the promote fence ships no plist; and a launchd
job fails silently in a log nobody reads — the measured history of the
predecessor is that its plist was never installed at all. A job inside
the watch process is in the operator's field of view; its failure is a
status fact (§6), not a log line.

The trade named honestly: no watch loop, no scheduled backup. The verb
still runs by hand, `status` still tells the truth, and the coupling is
real but aligned — the fleet that mutates the queue is the fleet whose
loop backs it up.

## §5 — Identity and custody

age X25519, asymmetric, exactly as hl2p argued: the box holds the
recipients (public) to write archives, so a compromised backup path
cannot read its own output; the identity stays on-box because the
nightly duty *is* the drill and a drill that cannot decrypt verifies
nothing — and the box already holds the plaintext store of record, so
its holding the identity adds no exposure. The asymmetry protects the
copies at the destination, which is where copies leave custody.

`posse backup init` writes the identity 0600 in a 0700 dir under
`$RHQ_HOME/secrets/backup/` (secrets/ never enters git — ADR 0015,
ranger-base-h56a) and **prints the path, never the key material**: pane
transcripts are captured by the harness, so a secret printed is a
secret in pane history. Off-box custody of the identity is an operator
step init states each time (copy the file into the password manager) —
posse never transmits it anywhere. Re-init refuses; rotation is out of
scope for v1 and refuses with the reason (old archives need the old
identity — rotating without a custody plan orphans every existing
copy). The identity-touching slice gets a security-lane review before
promote (bead cut below).

## §6 — Freshness: two facts, one owner each

The cadence ruling splits freshness into two facts with different
clocks, and each gets exactly one owning store (skill:
single-writer-and-stores):

- **On-box freshness** is owned by the staging dir itself — the newest
  archive's manifest timestamp. No second stamp store to disagree with
  the files. This is the *enforced* fact: older than 2× the interval,
  or a red last-drill verdict, makes `status` exit nonzero and raises a
  ShopCheck condition (ADR 0029 G-table) so the pulse can say so —
  escalation by shop check, not by nightly log line.
- **Off-box recency** is owned by the on-box sweep stamp
  (`state/backup/last-sweep.yaml`: when, destination, archives, their
  hashes verified at the destination after copy). The sweep also drops
  a marker file *at* the destination — a derived hint so that whoever
  holds only the disk knows what it holds; the on-box stamp is the
  authority `status` reads. Off-box age is **reported, never alarmed**:
  "off-box: as of <sweep>, N archives" is information for the operator
  who decides when to attach the disk, and no threshold in posse turns
  it red. That is the ruling verbatim — nightly noise about a
  deliberately-detached volume trains the operator to ignore the one
  status that matters.

## §7 — The archive and the drill

Archive contents (hl2p's shape, kept): `queue.bundle` (git bundle
--all), the worktree tar minus `.git` and bd's volatile runtime files,
the db staged via the sqlite **online backup API** against a read-only
source URI, and a MANIFEST carrying provenance plus every assertion the
drill checks, derived from the *staged copy* only. Cleartext surface:
the filename and a sha256 sidecar — ids, titles, history all inside the
ciphertext.

The API replaces hl2p's cp-db-then-cp-wal deliberately: that order was
safe at an 03:15 slot with the fleet asleep; §4 moves the duty inside
the fleet's working day, where copying db and wal non-atomically under
an active writer risks a torn snapshot (ASSUMED from SQLite's
documented backup guidance; the API is the documented answer and costs
one CLI call). The source is opened read-only and never checkpointed —
the hl2p read-safety rule survives the mechanism swap. A torn
`issues.jsonl` caught mid-export in the worktree tar is tolerable and
stated: the db is the recovery path, the jsonl a derived projection
re-exportable from it.

Drill arms, each independently red, verdict recorded to
`state/backup/last-drill.yaml`:

1. sidecar hash matches (transport) · 2. decrypts with the identity
(custody) · 3. bundle clones, HEAD and refs match the manifest
(history) · 4. restored tree has no remote (the 2c ruling) · 5. the db
holds exactly the manifest's bead ids (the file) · 6. `bd --readonly
--no-daemon` export from the restored tree matches the manifest's set,
leaving no daemon behind (bd can read it) · 7. posse's own bead-loss
census (beadloss.go LostBeads) runs CLEAN against the restored repo —
the public fold of the private census arm: the instrument the drill
wants was already in this binary · 8. **mutation arm**: one flipped
byte in a copy of the archive must make arm 2 fail, every run — a
drill that cannot go red measures nothing.

## Alternatives rejected

- **launchd plist** (the predecessor): §4 — invisible failure, ships
  outside the promote fence, and measured to be the arrangement nobody
  arms.
- **A posse daemon / second standing loop**: the watch loop is the one
  standing clock the harness owns (rangerhq-snd); a second one is a
  second liveness story and a second thing to leave running.
- **Driving the instance scripts from the verb**: a public binary
  exec-ing a private tree fails for every deployer but one.
- **A git remote for the queue**: ruled out (2c) and enforced at run
  time, §3.
- **`bd export` / `bd sync` as the archiver**: writes to the source
  (checkpoints its WAL) — MEASURED under hl2p; the artifact being
  preserved must never be a write target.
- **age/zstd as external CLIs**: neither ships on a stock box; a verb
  that fails until a brew install is the loose arrangement wearing a
  verb. The library fold removes the install; the format keeps the CLI
  as the exit hatch (§2).
- **Alarming on off-box age**: rejected by the cadence ruling; §6.
- **Cloud/object-store destination**: no autonomous spend, and the
  operator ruled no cloud. The `backup_dest:` seam takes any mounted
  path, so nothing forecloses a future operator-provisioned target.

## Consequences

- go.mod gains filippo.io/age (first dep beyond golang.org/x — priced
  in §2, exit hatch stated).
- sqlite3 CLI joins the preflighted tool set for this verb only.
- Every deployer gets the verb; this instance's destinations,
  interval, and identity stay in its home, out of this repo.
- The hl2p scripts, plist, and runbook retire in the instance tree
  once the verb ships (ops bead); archives already written remain
  readable only by the predecessor identity — custody of that identity
  is an operator note on the ops bead, not posse's concern.

## Verification observables (all unverified until built — the probes)

1. `posse backup` against a scratch `queue_repo:` writes archive +
   sidecar; every source file's mtime and size unchanged.
2. Add a remote to the scratch repo → exit 1, line names the ruling.
3. `posse backup drill` → 8/8 green; then corrupt a copy → arm 8 path
   proves the red (run the mutation arm's failure, not just its pass).
4. Watch loop with `backup_interval: 60s` on the scratch repo →
   archive appears within 2 ticks; restart the loop → no second run
   inside the interval (level-trigger holds).
5. Sweep to a scratch dest → hashes re-verified there, stamp written
   both sides; unmount (move the dest away) → `status` exit 0, off-box
   line says "as of", nothing red.
6. `sqlite3` staging under a concurrent bd writer → restored db passes
   arms 5–7 (the §7 swap's reason, exercised).
