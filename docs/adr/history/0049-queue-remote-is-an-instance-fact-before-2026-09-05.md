# Historical snapshot — not current policy

Archived 2026-09-05. Current decision: [ADR 0049](../0049-queue-remote-is-an-instance-fact.md).

# ADR 0049 — The queue's sanctioned remote is an instance fact: `queue_remote:` names it, and `posse backup` refuses every other

*Status: accepted 2026-09-02 (ranger-base-8e31g, from ranger-base-3i035) ·
owner: architect · amends ADR 0036 §3 · builds in the code beads named on
8e31g · number: 0043–0045 stay pre-named by ADR 0040 §2; per 0040 §3.1 this
file takes the next number no bead has claimed.*

## Context

ADR 0036 §3 as built (`internal/posse/backup.go`, bead ranger-base-a0ln0)
refuses to back up a queue repo that has ANY git remote, and the refusal
names the operator's 2c ruling. That ruling (ranger-base-xhsb, 2026-08-29)
was an answer about one repo on one box: the personal instance's own
queue repo never grows a remote. The binary ships it as a law of
every instance. **MEASURED:** the source check is `git remote` non-empty →
`Die`, with no input but the repo's state; the refusal's own text cites
"ADR 0015 §4", and 0015 §4 states no such rule (grep: 0015's only `remote`
is a rejected-alternative line about manifests). The rule's record is
xhsb and 0036's Context.

Three days later the operator ruled the opposite for the work instance
(ranger-base-w9jv (d), 2026-09-02): its bead/constitution repo lives on an
employer-approved internal remote. Whether the operator's words also
included "plus `posse backup` on-box" is recorded once (3i035's
description) and absent twice (w9jv's close reason, 26cd) — noted here
because the design must not depend on it, and it does not: the form below
is opt-in per instance. Meanwhile hoover's posture (private tree,
`docs/runbooks/work-instance-visibility.md` §4) holds an interim rule —
every `backup_*` key unset on the work box — which this record keeps
valid.

The deployer angle is the sharper one: posse is public, every `git clone`
mints an `origin`, and a verb that refuses any repo with a remote is a
verb most deployers cannot run at all. One instance's policy is riding in
the binary as everyone's.

## Decision

**D1 — one key, beside the one it qualifies.** `queue_remote:` in
`$RHQ_HOME/config.yaml`, flat like `queue_repo:`. Its value is the remote's
URL exactly as `git remote get-url <name>` prints it. Unset, empty, `~` or
`null` is the 2c posture unchanged: any remote on the queue refuses, with
today's words, the cite corrected (xhsb, not 0015 §4), and the key named as
the sanctioned way out.

**D2 — set, it sanctions one place.** With the key set, the queue repo may
hold exactly one remote, whose fetch URL and push URL both equal the
declared string, and then the source check passes. Everything else still
refuses, and the refusal prints what was declared and what was found: a
second remote of any name, a fetch URL that differs, a push URL that points
elsewhere (`git remote get-url --push` answers the push URL and falls back
to the fetch URL when none is set — **MEASURED** 2026-09-02, git 2.50.1).
*(Amended 2026-09-04, ranger-base-m6szh: and a remote carrying a SECOND
url. `get-url` lists only the first by default — git-remote(1) — while
`remote.<name>.url` uses "the first for fetching, and all for pushing"
— git-config(1) — so a remote whose first url was the declared one and
whose second was anywhere else printed the declared URL on both single
reads and passed, and every operator push landed at both. The check reads
`--all` on both sides and requires each to be the declared URL and nothing
else; the refusal prints every URL found. **MEASURED** 2026-09-03/04, git
2.50.1.)*

A queue with NO remote passes under either posture: the key sanctions, it
does not require.

**D3 — what the key is not.** Not a backup key: it is not in `backupKeys`,
arms no freshness reading, starts no clock — an instance that declares its
remote and no `backup_*` key has not asked for backups, and installing
posse still arms nothing (0036 §6). Not an override on the verb:
`BackupOpts` stays `Dir, Now, Out`, so `TestBackupHasNoOverride` stays
green, and the ticker needs no new argument because the fact is config the
verb reads. Not a name: `origin` is what clone mints everywhere; the ruling
names a place, so the key holds the place.

**D4 — the queue half of the archive stays remote-free on every
instance.** The archive's `queue/` holds the bundle, the staged db and the
jsonl projections (**MEASURED**, `stageBackup`): the bundle carries objects
and refs, not config, and the queue's `.git` is never walked, so no
`.git/config` and no remote stanza enters. The manifest does not record
the remote. The cut drill's arm 4 ("restored tree has no remote") is
therefore true by construction, and the restore 0036 §3 describes — `git
clone queue.bundle` minting an origin that names the bundle, which the
restorer removes — is unchanged. The one member that DOES carry the URL is
`home/config.yaml`, because `config.yaml` is a promoted path
(`PromotedPaths`, **MEASURED**) and the declaration lives there: that is
the operator's own line, already in the archive before this record, and a
restore that brings it back brings the sanction back with it, which is the
right order. The pin is the split: no byte of the declared URL under
`queue/`, and under `home/` only in `config.yaml`.

**D5 — "cannot push" becomes "never pushes", and here is what carries
it.** With a remote present the structural guarantee in `queuejsonl.go`
("the queue repo is created with no remote at all") is per-instance. What
holds on every instance: the binary invokes no `git push` (**MEASURED**:
grep over `internal/posse` finds `push` only in the gate's deny tables and
their tests); every shipped PID denies `git push` (pinned crew-wide,
`TestExampleAgentsArePIDs`); and the bd verbs that push — `bd sync
--full`, `bd daemon` — are denied on all nine shipped PIDs (**MEASURED**
9/9 in `examples/agents`, ADR 0015 §3's u9ud amendment; observed, not
pinned — the pin is a line in the code bead). The push is the operator's,
typed by hand, and ADR 0036's "the launcher cannot push the queue" reads
"the harness never pushes it" from here on. The comment in
`queuejsonl.go` says so once the code bead lands.

**D6 — one surface line.** `posse backup status` prints the posture:
`remote · none declared (config queue_remote: unset) — any remote refuses`
or `remote · <url> (config queue_remote:) — the operator pushes; posse
never does`. The watch loop's per-tick "scheduled archive failed" line
already carries a refusal's reason (0036 §4, level-triggered), so an
armed instance whose remote is undeclared reads stale AND says why on
every tick — the right reading, not a defect: the operator asked for
backups and is getting none.

**D7 — the work instance's line is the operator's, off this tree.** The
posture's interim rule stays valid until the operator sets the key. Setting
`queue_remote:` to the URL recorded on 26cd, and a `backup_interval:` beside
it, is an install step on that box, recorded there and nowhere public.

## Alternatives rejected

- **Keep the refusal; a remote-hosted instance is remote-backed only** (the
  bead's option 2). The remote holds the jsonl and history as of the
  operator's last hand push; the sqlite db and whatever the constitution
  home holds outside git are never in it (**ASSUMED** the db is untracked
  in the work queue as it is here). It leaves the deployer defect in
  place, and it hard-codes a ruling the operator has already reversed for
  one instance.
- **A boolean `queue_remote_allowed:`.** Sanctions ANY remote, including
  the personal GitHub f85 forbids on the work box. The ruling named a
  place; a boolean cannot.
- **Match by remote name.** Clone mints `origin` on every box; a name
  sanctions a shape, not a place.
- **Match by host, or normalise URL spellings** (`git@h:o/r.git` vs
  `ssh://git@h/o/r.git`, trailing `.git`). Cheaper to type, and a
  normaliser is a spelling table that loses to the next spelling. Exact
  string, both strings printed on refusal, the fix is a paste from `git
  remote get-url`. **ASSUMED** this costs one paste per instance, ever.
- **A flag on the verb.** Pinned out already (`TestBackupHasNoOverride`),
  and the ticker could not pass it — a per-invocation sanction of a
  standing fact is the plist nobody installed, wearing a flag.
- **Read the sanction from the private posture doc.** A public binary
  reading a private tree fails for every deployer but one (0036 §2's
  argument, verbatim).
- **Drop the source refusal.** Loses the one place the harness notices a
  personal queue growing a remote by accident (`bd daemon --auto-push`,
  `bd sync --full`), which is the failure 2c exists for.
- **Record the remote in the archive manifest.** Puts the URL into every
  archive for provenance nobody asked for, and breaks D4.

## Consequences

- No config change on the personal instance, and no behaviour change: an
  absent key is today's path, so every existing pin stays green
  (`TestBackupRefusesAQueueRepoWithARemote`,
  `TestBackupLoopSurvivesARefusal`, `TestBackupHasNoOverride`).
- The refusal's text changes: cite corrected, key named. Pins that match
  on `remote` keep matching.
- ADR 0036 §3 and its sub-ruling table row carry a dated pointer here.
- hoover's posture §4 replaces its interim rule with the key once the code
  lands (bead cut to hoover, dep on the code bead).

## Verification observables

Each is one pin in `internal/posse`; the mutation named is the one that
must red it.

1. Key unset, remote present → refused, and the line names `queue_remote:`
   (existing pin, wording widened). Mutation: drop the key from the line.
2. Key = U, one remote with fetch and push both U → archive written.
   Mutation: compare against the remote NAME.
3. Key = U, fetch URL V ≠ U → refused, line contains both U and V.
   Mutation: flip the equality.
4. Key = U, remote U plus a second remote → refused. Mutation: check only
   the first remote.
5. Key = U, fetch U and push V → refused. Mutation: read only the fetch
   URL.
6. Key = U, no remote → accepted (the key sanctions, it does not require).
   Mutation: require the remote.
7. Config holding only `queue_remote:` → `BackupConfigured()` false and
   `posse backup status` says nothing is armed. Mutation: add the key to
   `backupKeys`.
8. `BackupOpts` fields still exactly `Dir, Now, Out` (existing pin).
9. An archive taken under posture 2, every member read back: no byte of
   U under `queue/`, and under `home/` only in `config.yaml` (D4).
   Mutation: write the remote into the manifest.
11. Every PID in `examples/agents` denies `bd sync --full` and `bd daemon`
    (D5), asserted beside the `git push` assertion in
    `TestExampleAgentsArePIDs`. Mutation: drop one deny from one file.
10. The `status` line renders both postures (D6), pinned in
    `cmd/posse/backupsurface_qa_test.go` beside the schedule line.
