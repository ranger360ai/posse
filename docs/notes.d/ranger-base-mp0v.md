## The queue repo's visibility stamp is reconciled and probed on every queue commit (ranger-base-mp0v)

ADR 0023 made L3 honest for the **session** directory: a launch reconciles
both hook slots and then probes them — identity (the file at the dispatch
path is byte-for-byte the render we would write) plus behavior (that render,
exec'd fresh from a temp file, still refuses) — and parity drops any claim
the probe does not earn. That probe asks about exactly one repo: the one the
session starts in.

ADR 0015 §4 created a second repo that receives commits from posse, and
nobody starts a session in it. After the cutover, `.beads/issues.jsonl`
commits happen in `queue_repo:`, so **that** repo's `prepare-commit-msg` is
the one carrying the beads visibility stamp — the control keeping ops-class
content out of a public-marked db. It was installed by one runbook step,
performed once, and nothing looked again: the exact posture ADR 0023 exists
to end, one repo over.

`CommitQueueJSONL` now does for the queue repo what a launch does for the
session dir, at the moment the guarantee is needed:

| slot in `$QUEUE` | before | now |
|---|---|---|
| missing (step 6 skipped) | commits unguarded | written, stamped from config, commit goes through it |
| ours but stale (config re-marked after install) | keeps the old stamp forever | restamped on the next close |
| foreign / neutered | commits unguarded | **no commit**, and the pass says so |

The reconcile is best effort for the reason it is at launch — a legitimate
foreign chain is *expected* to make install refuse — and the probe is the
verdict.

**Why the refusal is the right side of the trade.** Refusing costs the
bead-loss census this close's line (`beadloss.go` IS the git log of the
projection). But an uncommitted projection is recovered by the next close
that finds the wall back up; an unguarded one is disclosed, and disclosure
does not un-happen. `commitQueue` is already "best effort, and never quiet",
so the refusal is a `⚠` line on the pass, never a failed dispatch.

**The stamp-drift half is worth naming separately.** The visibility verdict
is config-driven and, until this, *install-time only* — editing
`beads_visibility:` moves no hook. Everywhere else a persona launch
restamps. The queue repo had nothing that would, so a repo re-marked after
the window would have carried the wrong stamp indefinitely. Reconciling on
the commit path bounds that drift to one close.

Pins: `TestQueueCommitInstallsTheStampItCommitsThrough`,
`TestQueueCommitRefusesThroughANeuteredStamp`,
`TestQueueCommitRestampsASlotConfigHasReMarked`
(`internal/rhq/queuejsonl_test.go`). All three go red when the reconcile is
removed; the neutered one also goes red when the reconcile stays and only
the probe is dropped, which is the arm that separates "installs a hook" from
"will not commit through one it cannot vouch for".
