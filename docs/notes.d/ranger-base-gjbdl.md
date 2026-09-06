# ranger-base-gjbdl — the source-remote refusal, measured before it was deleted

ADR 0049's first done-when row asks for a number: **how many source-remote
refusals prevented an unintended transfer**, told apart from the ones that
only delayed a backup. This is that measurement, taken before the code came
out, so the deletion rests on a count rather than on the ADR's assumption.

## The answer

| class | count | where |
|---|---|---|
| refusals that prevented an unintended transfer | **0** | — |
| refusals that only delayed (here, prevented) a backup | **1 instance-class case** | the work instance, `ranger-base-8e31g` |

Zero is not "none found in the logs I had". The prevented-transfer class was
**structurally empty**, and that is the finding:

- `checkQueueRemote` ran inside `RunBackup`, after `CheckBackupTarget`, and
  its only effect on failure was `return res, err` — no archive written.
  It read `git remote` and `git remote get-url` on the source and wrote
  nothing anywhere.
- Backup transfers nothing to transfer. MEASURED at this commit: no `git
  push` call site exists in the shipped binary —
  `grep -rnE '(git|exec\.Command|runGit)\([^)]*"push"' --include='*.go' cmd internal`
  (excluding `_test.go`) is empty. The two `"push"` hits in
  `internal/posse/gates.go` are the deny classifier reading someone else's
  argv, not a push.
- It fenced no other process. A remote on the queue is reachable by any
  hand-typed `git push` and by `bd sync --full` / `bd daemon`; those are
  walled by the PIDs (`TestExampleAgentsArePIDs`), not by this check. A
  refusal here removed a local copy and left the remote exactly as it was.

So the check could only ever cost a backup. It did, once, by design.

## The searched window

`2026-09-01 18:38:52 -0400` — `8ac4384d`, which landed `posse backup` with
the source refusal already in it — through `2026-09-05`, today. The
per-instance `queue_remote:` form landed inside that window at `59fca8ff`,
`2026-09-03 12:57:46 -0400`.

The window is the whole life of the rule: `git tag --contains 8ac4384d` is
empty (tags are `v0.3.0`, `v0.4.0`), so the check never shipped in a
release and ran only where the binary was installed from `main` — this box
and the work box, and no third party's.

## Cases and evidence

**This box: 0 refusals, 4 successful scheduled runs.**

- The config home's `config.yaml`: a `queue_repo:` naming a checkout,
  `backup_interval: 24h`, and no `queue_remote:` — the unset posture, under
  which any remote refuses.
- `git -C <queue_repo> remote -v` prints nothing: zero remotes, so
  `checkQueueRemote` returned at its `len(names) == 0` arm on every run.
- The runs happened and verified: `state/backup/` holds
  `posse-backup-20260903T123706Z`, `-20260904T124703Z`, `-20260905T124703Z`
  with sidecars, and `state/dispatch-watch.log.1` names a fourth,
  `20260902T052114Z`, as pruned. Each is logged `· verified`.
- Zero refusal lines: grep over `state/*.log` for `never grows a remote`,
  `git remote(s)` and `queue_remote` returns nothing.

**The work instance (26cd): the one firing, and it is the delay class.**

`ranger-base-8e31g` (hoover, 2026-09-02) is the whole record: that box's
bead/constitution repo sits on an **employer-approved internal remote** by
operator ruling w9jv (d), so "remote present = refusal" made the on-box
archive and the sanctioned remote mutually exclusive there. The refusal cost
that instance its recovery copy. It prevented no transfer: the remote was
the operator's own, sanctioned, and reachable by the operator's own push
with or without this check.

`queue_remote:` (`59fca8ff`, ADR 0049's first form) existed to buy that
instance its backup back, at the price of a config line per instance. This
bead removes the price instead.

## What this measurement does not cover

- **The work box is not this box.** Its refusals are recorded only as
  8e31g's prose and the posture handoffs (`ranger-base-7yhe3`,
  `ranger-base-2adal`). No log or shell there was read, so its refusal
  *count* is unknown — one instance-class case, not one tick. The direction
  is not in doubt: every tick there refused until the config line landed.
- **No third-party deployments exist to search.** That is a consequence of
  the unreleased window above, not a gap that closes later.
- **The check's own defects are not refusals.** `ranger-base-m6szh` found
  that it read only the first URL of a multi-URL remote, so a sanctioned
  first URL and a second one anywhere else passed. Those are escapes of the
  check, not transfers it prevented, and they are counted in neither column.
