# ranger-base-0q7rp — the backup row and the backup verb disagreed about whether a store exists

*Found on the work-box shakedown (ranger-base-x4g3h leg 1, 2026-10-07, posse
0.5.1+5c2948a2): a single-tree instance with `backup_max_age: 12h` written and
`queue_repo:` unset. Fixed 2026-10-09.*

## The contradiction, verbatim

    $ posse status
    LANE  —  no backup of the store of record on this box — ~/.posse/state/backup
             is empty (config backup_max_age: 12h00m)
    $ posse backup
    config queue_repo: is unset — the store has not moved yet (ADR 0015 §4),
    so there is nothing to back up
    $ posse backup status
      schedule · every 6h00m, from the dispatch --watch loop (config backup_interval:)

Three surfaces over one config. The row counts the absence of an archive as a
condition a human must answer and exits `posse status` non-zero; the only
remedy it implies answers that the condition cannot exist; and the schedule
line describes a cadence whose every tick calls that refusal. One of them had
to be wrong for an instance with `queue_repo:` unset.

## Which half moved, and why

Not the verb. ADR 0036's publication boundary decides it in as many words —
"refuse an unset `queue_repo`" — and ADR 0015 §4 is the inertness rule the key
itself keeps: absent means the store of record has not moved into a queue
repo, so there is no store to archive and `posse backup` says so. Teaching the
verb to archive a single-tree store (the beads source's own `.beads`) would be
a new decision about what the store of record IS, not a bug fix.

So the ROW moved. `queue_repo:` unset is not a backup that is late, it is a
duty that does not exist yet, and the row that used to name the empty
directory was a true reading pointing at the wrong instruction.

It stays a condition, and it stays LANE. Armed-with-nothing is the
predecessor's exact failure — an arrangement that was configured and never ran
(ADR 0036 Context, the plist nobody installed) — and an armed backup
arrangement that CANNOT run is that same arrangement with a config key in
front of it. What it now says is the key and the two ways out:

    LANE  —  backups are armed and config queue_repo: is unset — the store has
             not moved yet (ADR 0015 §4), so there is nothing to back up —
             ~/.posse/state/backup holds no archive: set queue_repo:, or remove
             the backup_* keys from config.yaml to disarm this row (config
             backup_max_age: 12h00m)

## One sentence, one place

The sentence lives in `backupNoStoreClause()` (internal/posse/backup.go) and
every surface that ASSERTS the fact reads it from there: `RunBackup`'s
refusal, `GovDetail`, and `BackupFreshness.Line`'s suffix.
`BackupScheduleLine` carries a short form instead — "every tick refuses while
config queue_repo: is unset (ADR 0015 §4)" — because `posse backup status`
prints it directly under the freshness line and the first draft said the whole
sentence twice in two adjacent lines. Same key, same ADR, one consequence
clause of its own. That
is the same shape `FutureClause` took for bead ranger-base-rgv61 and for the
same reason — the whole defect was two readers of one fact rendering it in
words that could drift apart, and a shared sentence is the only form of
agreement a test can pin.

The key did NOT change: still `backup-stale`, still no G id (ADR 0029's table,
ADR 0036 §6 — "its existing key and no G id"). The condition is the same one
— the armed duty is not discharged — and only its reason and its remedy are
new, so a second key would have split one condition across two fingerprints
for the coordinator to correlate.

## What is pinned

- `TestNoStoreOfRecordRowAgreesWithTheVerb`
  (internal/posse/backupfresh_qa_test.go, arm 3): the row's detail contains the
  same sentence `RunBackup` returns on the same instance, names `set
  queue_repo:`, and no longer reports the empty directory as the condition.
  Its control arm writes the key over the same empty directory: the detail goes
  back to naming the archive, and the verb stops refusing for that reason.
- `TestBackupSurfacesAgreeWhenTheStoreHasNotMoved` (cmd/posse, the shipped
  binary): `posse backup status` and `posse backup` on one scratch home, both
  non-zero, both carrying the sentence, with the schedule line saying its ticks
  refuse — and a control home with the key written saying none of it.
- `TestBackupScheduleLineNamesTheKey` gained the no-store arm, and the age rigs
  in backupfresh/backupfuture gained a `queue_repo:`, because a store that has
  moved is the premise of every reading about an archive's age.

## Left alone, deliberately

`backupTick` still calls `RunBackup` on an instance with no store and still
prints the refusal — once per sample, which under `backup_interval: 6h` is
every 45m while no usable archive exists. It is truthful and it agrees with
the verb; silencing it would be a level trigger that stops reporting a level.
Whether that cadence of an unfixable complaint in the watch log is worth
damping is a separate question and is not this bead's.
