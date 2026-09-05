# ADR 0049 — Local backup does not validate a source remote

*Status: accepted 2026-09-05 · ADR simplification, operator ruling 2026-09-05 · source-remote precondition removed from the decision; implementation deferred.*

## Decision

Remove `queue_remote` and the source-remote validation from local backups.
A source queue may have no remote, one or multiple remotes, or differing
fetch/push URLs without preventing a recovery copy. Do not read those URLs
to authorize a backup or replace exact matching with another allowlist.

[ADR 0036](0036-posse-backup.md) retains the local-target prohibition, disk
floor, single-flight, read-safe staging, verified publication, retention and
schedule. Backup itself makes no network transfer. Removing an incidental
source-config alarm does not permit a remote destination or a queue push.
Instance ownership and publication policy remain under
[ADR 0015](0015-constitution-promotion.md) and the separate visibility/data-ceiling rules.

Archive construction stays the same: the queue bundle contains history/refs,
not `.git/config`; `queue/` has no remote stanza. Promoted `home/config.yaml`
is copied as source content even if an old unused `queue_remote` remains
there. No migration deletes an operator's config or rewrites its remotes.
Drop the backup-status posture line instead of suggesting that an ignored
key still sanctions or refuses anything. Arming and freshness remain governed
by backup keys, never this obsolete one.

## Deferred deletion and acceptance

Delete `QueueRemote`, `checkQueueRemote`, its URL-list formatting if unused,
`BackupRemoteLine`, the call in `RunBackup`, and status/config/help references.
Concrete surfaces are `internal/posse/backup.go`, `cmd/posse/main.go` and
associated tests/examples. Price: roughly 2–4 source files plus text/tests;
one key and remote-acceptability branches; no new store, actor or flag.
Update rejection tests into successful local-backup cases while keeping all
target-refusal and archive-scope checks. No machinery changes in this session.

First done-when row: **number of source-remote refusals that prevented an
unintended transfer; distinguish these from refusals that only delayed a
backup.** Record the searched window, events and evidence for any prevented
transfer. The earlier source-policy mismatch is measured; prevented network
harm is ASSUMED, and no such count was established by the review.

What breaks if wrong: the harness stops incidentally alerting on an
unexpected source remote. A different process could still push there;
backup's former check did not fence that process. Recovery copies must stay
available when source configuration is wrong. Verify no network calls or
target override were introduced and that all source-remote variants still
produce a verified local archive.

## Lineage and rejected alternatives

| Record | Disposition |
|---|---|
| 0049, 2026-09-02 and multi-URL amendment | Per-instance source allowlist archived, pending removal |
| Operator ruling 2026-09-05 | Local backup no longer gates on source remote configuration |

Reject a boolean sanction, normalized-URL matcher or command override: each
keeps an admission check unrelated to the local copy's effects. Instance
remote policy still belongs to the instance; this page neither sets it nor
arms backup on any box.

[Prior allowlist and measurements](history/0049-queue-remote-is-an-instance-fact-before-2026-09-05.md).
