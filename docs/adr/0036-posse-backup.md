# ADR 0036 — Local queue backups, verified before publication

*Status: accepted and built 2026-09-01; simplified 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · archive, verification, status and periodic schedule retained.*

## Decision

`posse backup` creates one on-box archive of the configured queue store and
promoted constitution home. `posse backup status` reports archives, freshness
and schedule; `posse backup verify` reopens an archive and checks its manifest
and sidecar. Recovery uses ordinary `tar -xzf` into a chosen scratch/recovery
directory and git/sqlite tools. No Herdr socket or healthy fleet is required
for these hand-typed operations.

The archive is plaintext `tar.gz`, mode 0600 inside a 0700 directory. It
contains `queue/queue.bundle` with all journal refs when commits exist, a
database staged through SQLite's backup API via the existing read-safe
`pairCheckReader`, available JSONL projections, and `home/` members from
`BackupHomePaths`: promoted paths and the promote manifest. No queue `.git`
configuration is walked. No-commit queues are backed up without a bundle,
with that absence stated. The manifest records provenance, exclusions and
each staged member's size/hash; it describes staged copies, not a claim of
an atomic snapshot across every source.

Exclude `envs`, `secrets`, `state`, `personas`; skip symlinks and nonregular
members instead of following them outside the named set. The database is the
recovery path if JSONL was copied during export. Never use a writing bd
export/sync or checkpoint on the source as the archiver. The quiet-database
round trip is measured; the original concurrent-live-writer recovery arm
remains unmeasured. Do not describe member verification as a full bd restore
drill or as proof of a no-db store's recovery path.

## Publication and recovery boundaries

Refuse a remote target, including remote filesystem mounts or an unreadable
locality determination. There is no target override. Local-source remote
configuration is a different question owned solely by
[ADR 0049](0049-queue-remote-is-an-instance-fact.md); its removal is approved but deferred.
Backing up never pushes, fetches or transmits a queue, and source policy
does not become permission to transport its archive.

Refuse an unset `queue_repo`, missing required tool, insufficient disk or
failed verification. Use git and sqlite3 with stdlib gzip/tar; no age
dependency. Keep the staging-volume floor `backup_min_free_mb` (default 384),
single-flight flock, temporary output and publish-by-rename. Reopen every
archive before publication: hash/size and completeness must match, with no
unexpected or missing member. Failed output does not become a named archive.
Prune only after a newer verified publication; `backup_keep` defaults to 3.
Do not delete the last good copy because a new backup failed.

The queue half has no remote configuration by construction. Restoring via
`git clone queue.bundle` creates a local-bundle origin; the restorer removes
that local origin. A restored `home/config.yaml` may contain operator facts
already in that source file. Neither archive existence nor its hash proves
that an operator tested full recovery; report the checks actually performed.

## Chosen clock and freshness

Keep the built `backupLoop` goroutine on its own ticker inside Watch. Config
`backup_interval` arms the schedule; absent starts no clock, malformed
disarms with a diagnostic. Check the level once at startup and each tick:
archive when no usable archive exists or its age reaches the interval.
Restarts inside the interval do nothing. This is independent of pass return,
rolling work and epoch changes; PAUSE stops new dispatch, not backups.
Cancellation ends the watch-owned clock. Without a watch there is no scheduled
backup; the hand verb and status still work. This session changes no cadence.

The archive directory owns freshness, using the timestamp in published names;
there is no `last-drill.yaml` or separate success-stamp store. Explicit
`backup_max_age` wins, otherwise use twice a valid schedule interval or 48h
without one. Any backup configuration or existing archive enables the
freshness view. Armed with no usable archive is stale; no configuration and
no archive is inert. Report `backup-stale` as LANE through
[ADR 0029](0029-governance-surface.md), with its existing key and no G id.

Both scheduler and status use `splitBackupsAt`: strictly future timestamps
are undatable, named as such and excluded from the freshness clock, never
deleted or renamed merely for being future. A datable older archive can be
fresh while the future warning remains. If all are future, freshness is
unknown/stale and the scheduled duty can run. Count all archive filenames;
retention continues to count future-stamped files. A filename is a cadence
reading, not re-verification of its contents on every tick.

## Dated rejected plan and evidence

The 2026-09-01 sub-ruling on ranger-base-ay3dr cut the former off-box sweep,
destination keys, age identity/init, encrypted archive dependency, transport
clock and eight-arm drill verb. Their useful per-archive checks folded into
publication/verify. They are not a future backlog. Adding off-box transport
would require a new custody/encryption decision before any transfer.

Built as ranger-base-a0ln0 and ranger-base-zv3y6; future-clock correction
ranger-base-rgv61. Recorded evidence: 1.17GiB loose journal packed to a 30MB
bundle in 12s; source-file preservation, corruption/missing-member rejection,
scheduled creation, restart suppression and pause independence exercised in
scratch fixtures. These are dated measurements, not runs made here. The
unmeasured live-writer recovery arm remains explicit above.

## Consequences and alternatives

No essential backup state, key, actor or flag is removed by this page. Source
validation is priced only in 0049. Reject a private script dependency,
uninstalled sidecar scheduler, separate daemon or a second freshness store.
The preserved limitation is on-box recovery: loss of the entire disk can
lose both source and archive. The operator's local-target ruling owns that
trade; this rewrite does not revive the rejected remote destination.

## Lineage

| Record | Surviving decision |
|---|---|
| 0036 sub-ruling and schedule/future-clock amendments | Built local archive and its one freshness model |
| 0049 | Sole source-remote decision, implementation deferred |
| Operator ruling 2026-09-05 | Retains backup; archives superseded ceremony |

[Prior design, rejected plan and measurements](history/0036-posse-backup-before-2026-09-05.md).
