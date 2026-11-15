## A launch share for one label's class of work (ranger-base-j11mf)

ADR 0011 §5 (amended 2026-11-14) is the decision. This fragment holds the
measurements it rests on, the config lines for the operator to promote, and
what was validated through the consumer before the code exists.

### What is contended

MEASURED 2026-11-14 on the always-on host, over the instance's
`state/dispatch-watch.log` from 2026-11-10 18:53 (watch armed) to
2026-11-14 22:00 (30,965 lines; `-n 6`, `dispatch_epoch` 1h,
both dollar caps set):

- Epochs that had at least one pass: 101. Epochs that hit the launch cap: 66.
  Exhaustion lines: 434.
- Launches refused by the dollar brake (`budget: … not dispatched`): 0.

Pass headers carry a time and no date, so the epoch count dates each pass
from the last `armed` line plus day rollover. ASSUMED accurate to ±1 epoch per
restart. So the attempt count is what the two classes contend for, and the
share splits only that count.

### Config lines (operator promotes; inert until the code bead lands)

The shape is below. The instance's values are on ranger-base-j11mf, because
live guard values are instance-ops content (ADR 0024 D1).

```yaml
launch_share_label: project   # beads carrying this label are the project class
launch_share: <k>             # attempts per dispatch_epoch reserved for that class,
                              # k < autostart_max_beads; lent when it has nothing launchable
```

The instance's `k` is ASSUMED. It is the operator's dial, and nothing here
measures the project queue's demand, because no bead carried the label when
this was written.

### Validated through the consumer, not a YAML parse

A scratch RHQ_HOME held the live config plus the two lines above (instance values), with its
`agents`, `envs` and `runtimes` symlinked to the live home.

- `posse systemd` (stdout render only, never written) from that home printed
  `ExecStart=… dispatch --watch 5m -n 6 --resume`. That is byte-identical to
  the live home's render, so the added lines disturb no existing flat-YAML
  reader.
- `posse dispatch --dry-run --no-reap -n 6` from that home, cwd the shared
  checkout, exit 0: "6 bead(s) would be dispatched", with the same routes as
  live. Nothing in the queue carries `project` yet, so this proves only that
  the draft config loads. Under the decided rule the result is the same,
  because an empty project class lends its allotment.
- Routing invariance: 0 of 11 instance PIDs and 0 of the 9 shipped example
  PIDs carry `project` (grep of `^labels:`). Under §4 a label that no PID
  carries adds no candidate and anchors no head. This is MEASURED statically
  and does not need a run.

What could NOT be validated today is the share itself. No binary reads the
two keys yet. The code bead's done-when therefore includes the dry-run line
that prints both allotments from this exact config. The operator promotes
only after that line reads right.
