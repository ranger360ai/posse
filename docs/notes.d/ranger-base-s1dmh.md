## Documenting a key the seed's own census forbids documenting (ranger-base-s1dmh)

Found on the work-box shakedown 2026-10-07 (ranger-base-x4g3h leg 1): `posse
status` raised the LANE row `no backup of the store of record on this box …
(config backup_max_age: 12h00m)`, and the only prose in the shipped tree that
named any backup key was `docs/adr/0036-posse-backup.md`. `examples/config.yaml`
— the file `posse init` copies verbatim into every fresh instance — carried
none of them, and neither did INSTALL.md. A row with a remedy nobody can look
up.

Written 2026-10-09.

### `posse help` already had the keys; the two files an operator reads did not

Worth recording because it narrows the gap rather than widening it. The big
help text (`cmd/posse/main.go`, the `posse backup` block) documents all five
`backup_*` keys WITH their defaults, in the same per-key form it uses for
`attn_*` and `verify_box_max_age:`. What it is missing is `queue_repo:`, named
in its prose and not as a `config` line. And `posse backup --help` does not
reach that text: the verb's own flag loop takes `--help` as an unknown flag and
dies with the usage line, which is what the bead saw (MEASURED 2026-10-09:
`posse backup --help` and `posse backup status --help` both print the usage
line and exit 1). So the undocumented surface was the SHIPPED FILES, which is
where this bead landed, and the help text is a third place that already agrees
with them. The `--help` half is filed as **ranger-base-cse63**, not fixed
here: it is a CLI change and this bead's deliverable is the two files.

### The block is in the governance region, not beside the queue keys

`examples/config.yaml` gained a `backups of the store of record` block between
G11 (`launcher_behind_max:`) and the autostart section. That is where it
belongs because `backup-stale` is a governance row (ADR 0029, LANE, no G id)
and reads like its neighbours: the prose says what arms the reading, the
commented keys carry the defaults, and the remedy is in the same paragraph as
the condition.

INSTALL.md §9 ("The work repo and its queue") gained the operator's half. The
one fact that paragraph exists for: **the row's subject is `queue_repo:`, and a
cold instance does not have one.** Until the queue is cut over into its own
tree (ADR 0015 §4) the key is unset, `posse backup` refuses, and the right
state is every backup key absent. Pointing `queue_repo:` at the work repo to
make the verb run would be a different decision about what the store of record
is — ADR 0036's 2026-10-09 amendment (ranger-base-0q7rp) says in as many words
that it is not taken — so the doc says so instead of leaving a deployer to
guess it.

### The seed's documented-default census decides HOW each key may be documented

`internal/posse/seedconfig_test.go` is not a formality here: it reads the seed
and holds every documented value against the constant the tree resolves it
through. Five keys, four different answers, all mechanical:

| key | how it may be documented | why |
|---|---|---|
| `backup_keep: 3` | a value, COMPARED | pairs with `DefaultBackupKeep` through the `BackupKeep` body-form reader |
| `backup_min_free_mb: 384` | a value, ADMITTED | already in `seedNumberDefaultNotAConstant`: `BackupMinFree` answers BYTES (`DefaultBackupMinFreeMB << 20`), so no one number can be compared |
| `backup_interval: 24h` | a value, REGISTERED | presence is the arm switch, so there is no default for a line to drift from — the `autostart_interval` shape, one surface over |
| `backup_max_age:` | **no value at all** | `seedDurationDefaultNotAConstant`'s stronger half: its default is a RULE (2x the interval, else 48h), and a key in that register must stay undocumented because a documented line would be a claim the pin cannot hold |
| `backup_dir:`, `queue_repo:` | a value, unheld | string-valued; neither discriminator enumerates them |

So the block names `backup_max_age:` in prose and in backticks — never as a
commented `key: value` line, which is the one spelling `seedDocumentedValue`
counts. The fifth key gets its default stated as the rule it is.

MEASURED 2026-10-09, before and after: with `backup_interval: 24h` documented
and nothing else changed, `TestSeedConfigDocumentedDurationDefaultsAreTheConstants`
failed by name at `examples/config.yaml:874 documents backup_interval as 24h,
and this census never reached that key` — the pin's own third way out is the
register row, and that is what it got.

### What moved in the census, and the floors that had to move with it

- `seedDocumentedDurationOutsideTheCensus` gains `backup_interval` (third
  member). Its row says what the other two say and one thing they cannot: the
  key IS read through `CfgGet`, but inside `LoadBackupConfig`, which answers a
  `BackupConfig` and an error rather than a duration — so widening the key
  source would not have reached it either.
- `seedNumberDefaultNotAConstant["backup_min_free_mb"]` ended
  "Undocumented in the seed today, so nothing is admitted by this row". It is
  documented now, so the row says what it admits and where the run logs it.
- The number half's population paragraph said ELEVEN documented lines, three of
  four pairings documented, and `backup_keep`/`backup_min_free_mb` as "tree
  keys the seed says nothing about". MEASURED 2026-10-09 after this change: 13
  duration-valued and 13 bare-number commented lines, four pairings compared.
  The witness floors (`compared < 3` → 4, `len(documented) < 11` → 13,
  `len(durationLines) < 10` → 13) move with the population, which is what makes
  them catch a documented line that LEFT.

That last bullet is the reason this bead touched a test file at all. A doc
change here is a change to a surface three pins hold, and leaving the counts
behind would have been the exact stale-prose defect `seedconfig_test.go`'s own
header is about.

### Verified

`make seed-check` (all seven pins), `make fmt-check`, `make notes-check`,
`make tree-check` whole, and `make test` (all three arms). Nothing in
`internal/posse` or `cmd/posse` changed behaviour: the diff is
`examples/config.yaml`, `INSTALL.md`, `CHANGELOG.md`, this fragment, the notes
index, and the census registers and prose above.
