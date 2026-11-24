# The first install that adds a G-row rolls back, and the verb is right to be suspicious (ranger-base-i9fq8)

A delegated `posse upgrade` of 0e1cb6c0..65d90a5e ran `make test` green at the
target, swapped the binary, and then rolled itself back on its own ADR 0092 D4
check. Nothing was wrong with the target. What refused it was the row the target
had just added.

## The reading

MEASURED 2026-11-21, 04:11:59 to 04:30:30 EST (America/New_York), this box
(16 cores), by gilfoyle at a claude/standard seat realized at shims, from the
worktree `~/.posse/worktrees/posse/gilfoyle-posse-ranger-base-i9fq8`. The verb
was backgrounded and waited on by pid, per ADR 0092 D3 and AGENTS.md.

```
posse upgrade                      # no args: the default origin/main target
  0e1cb6c -> 65d90a5 (17 commits) · source ~/src/posse · target ~/.local/bin/posse
exit 1 · wall 1111s (18m31s) · installed and rolled back
loadavg at start 9.17 6.76 5.34 · at end 6.90 10.35 10.19
```

| phase | this run | 76esp | jnf2z |
|---|---|---|---|
| suite-slot queue, arm 1 | **0s** | 10s | 146s |
| arm 1 · internal/posse | 232.7s | 244.9s | 214.0s |
| arm 1 · internal/treepins | 172.9s | 156.9s | 154.5s |
| arm 1 · cmd/posse | 139.1s | 122.3s | 119.1s |
| arm 1 · cmd/coldexec | 1.8s | 1.8s | 2.0s |
| arm 1 · cmd/testparallel | 0.3s | 0.2s | 0.2s |
| arm 2 (`-tags posse_arm2`) | 66.3s | 62.6s | 81.5s |
| arm 3 (`-tags posse_arm3`) | 172.6s | 103.7s | 119.1s |
| silent-revert audit | 2817 commits, 0 untriaged | 2800, 0 | 2798, 0 |
| wall | 1111s | 1017s | 1148s |

**This is the first reading with no suite-slot queue in it at all** — slot 1 was
held by a sibling seat from before the launch to after it, and all three arms
took slot 2 on arrival. ranger-base-76esp derived the verb's own work at ~1000s
from two contended points, and predicted a quiet box would price lower. This
reading does not test that prediction and must not be quoted as refuting it: at
1111s with 0s of queue it is 104s ABOVE 76esp's wall-minus-queue, and it is the
only one of the three that also paid a swap, a second
status/gates/render-probe pass and a rollback install after the arms finished.
Arm 3 alone ran 69s longer than 76esp's at a comparable load, so the package
noise that cancelled across those two runs did not cancel here. What the three
readings jointly support is narrow: **the verb costs 17-19 minutes on this box
and the queue is the part anyone can do anything about.**

## What refused it

```
  before: status 15 condition(s), 0 unreadable store(s) · gates 11 persona(s),
          36 not-held row(s), 0 non-zero exit(s) · render-probe 0
  after:  status 15 condition(s), 0 unreadable store(s) · gates 11 persona(s),
          36 not-held row(s), 0 non-zero exit(s) · render-probe 0
  ✗ `posse status` gained 1 LANE G18 row(s) (0 → 1), a row this binary has no
    class for at all — the new binary is reporting a condition this one cannot name
```

G18 is the row `ranger-base-rrgnc` added **inside this very range**, and rrgnc
classified it correctly: `upgradeverify.go` at the target carries
`"G18": upgradeInstallSubject`, commented "G18 appears after EVERY install with a
live dispatch loop, by construction: rename(2) leaves the loop on its old inode.
Classed binary-rendered it would roll back every good install."

The table that was consulted was not that one. D4's comparison is made by the
binary the verb is running out of, so a new row's classification is looked up in
the OLD table, where it does not exist, and `upgradeCondSource`'s unknown-code
rule answers `upgradeBinaryRendered` — deliberately, with its reasoning written
out: "a row the NEW binary printed and this one cannot name is a condition the new
binary invented. The pin exists so that the next bead to add a G-row classifies it
instead of discovering this."

rrgnc did classify it. The pin held. The install still rolled back, because a
classification can only ship in the binary being installed — one install after the
judge needs it. So the general shape, which is the thing worth keeping:

> **The first install that crosses any new G-row rolls back, always. The fix for
> that install cannot be installed through the verb either, since every target
> containing the new row is judged by a table that predates it.**

Verified rather than inferred: `0e1cb6c0:internal/posse/upgradeverify.go` names
G1..G17 and has no G18 key; `d24988c3:` names G1..G18; G18 is the only class that
differs between them (`G43`/`G46`/`G99` in that tree are fixtures in
`upgradeclass_qa_test.go` and `upgradeverify_test.go`). The row was TRUE when it
fired — both live loops were on the old inode after the rename, which is the state
G18 exists to report. Every other D4 arm was identical across the two binaries.

## What this is not

Not a defect in the target, not a flake, and not a suite failure: `make test` was
green at 65d90a5e. Not a reason to run the rollback command by hand either — the
verb caught its own check and restored `~/.local/bin/posse` to 0e1cb6c0, read back
by `posse version`, with `posse.prev` still beside it and no `posse.new` left.

Worth saying plainly because the asymmetry argument behind the unknown-code rule is
right and should survive the fix: a false rollback costs one re-run of the verb and
leaves the fleet on the binary it was already running, with the reason in one line.
A missed rollback leaves a broken binary on a box that is dispatching a live fleet.
The question this reading raises is only **whose table should answer**, not whether
an unnamed row should be waved through.

The diagnosis cost is its own finding: the rollback line names the row but not that
the target already classifies it, so establishing that the install was good took
source archaeology across two trees rather than reading one line of the report.

## Not closed here

- ranger-base-v2xj9 — the crossing rule itself, in the architecture lane.
- ranger-base-rfsy4 — how this range lands on this box, which is the operator's:
  the verb as it stands cannot land any target containing the new row, and the one
  route that keeps every arm of D4 armed is to stop the dispatch loops for one run,
  because `ReadWatchImage` abstains with no loop running and the row then cannot
  appear. Stopping a live dispatcher is the operator's or the coordinator's
  (guardrail 3), so nothing here does it.
