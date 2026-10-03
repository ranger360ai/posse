# A stale hook wall named four guards and had lost one (ranger-base-knux2)

Follow-up to [ranger-base-sqxo1](ranger-base-sqxo1.md), which read the line and
did not capture it. Everything below is MEASURED 2026-10-03 on this box, with
`posse` built from main `58743caf` unless a line names another build.

## The line

`$RHQ_HOME/state/dispatch-watch.log:17532`, in the generation

    == dispatch --watch armed 2026-10-03 00:15:59 · pid 2074 ==
    posse binary · ~/.local/bin/posse · 0.5.0+1b8ffe2d
    hook wall (watch): 3 of 4 repo(s) do NOT carry this binary's render
      $CONSTITUTION
        L3 prepare-commit-msg hook — … — ours but stale — run `posse gates install-hooks`;
        the data ceiling, beads visibility, constitution-path and shared-index guards are not realized
      <the queue repo>     (same slot, same verdict; path elided — ADR 0024 D2)
      ~/src/posse          (same slot, same verdict, at posse-prepare-commit-msg behind the chain)

The count in the bead was right; its DATE was off by one — the generation is
2026-10-03 00:15:59, not 2026-10-02. The fourth repo is `~/src/hcn`. The bead's
caveat asked `managed-hooks` about a fourth repo of its own, one this instance's
private beads tree lives in; that repo is not in `beads_visibility:` at all, so
the wall has never had an opinion about it.

The four walls either side of it, which is where the cause is:

| log line | generation | binary | verdict |
|---|---|---|---|
| 17366 | 2026-10-02 23:11:02 | 0.5.0+1b8ffe2d | 3 repo(s) carry this binary's render |
| 17532 | 2026-10-03 00:15:59 | 0.5.0+1b8ffe2d | **3 of 4 do NOT** |
| 17598 | 2026-10-03 08:32:36 | 0.5.0+1b8ffe2d | 2 of 4 do NOT (posse healed) |
| 17862 | 2026-10-03 09:43:41 | 0.5.0+763de34f | 4 repo(s) carry this binary's render |

## The cause is a config input, not a code change

Rows 1 and 2 are the SAME binary, 65 minutes apart, and three repos crossed
from fresh to stale between them. A render that drifts without the renderer
moving drifted because an INPUT moved: `~/.config/posse/config.yaml`, mtime
2026-10-02 23:39:07, squarely inside the gap.

That config is not version controlled. The pre-edit copy is recoverable
because the operator's backup at `state/backup/posse-backup-20261003T031102Z.tar.gz`
holds `home/config.yaml` with mtime 2026-10-02 23:11:02 — taken at the same
moment as the clean wall in row 1. The edit added three things:

- `~/src/hcn` to `beads:`,
- `~/src/hcn: private` to `beads_visibility:` (which is why row 2 measures four
  repos where row 1 measured three),
- and the part that stales every hook on the box at once: this instance's
  FIRST `data_ceiling_patterns:` block (ADR 0050), one class, `ssn`.

`data_ceiling_patterns:` is instance-wide, so it is in every repo's
`prepare-commit-msg` render. Three repos installed before 23:39:07 went stale
in one step. `~/src/hcn`, whose hooks were first rendered AFTER it, is the
control arm: it is the one of the four that row 2 found fresh.

## Which rules the stale render was grading by

Rendered `$CONSTITUTION`'s `prepare-commit-msg` under the pre-edit config
and under the current one, both under a `core.hooksPath` redirect into a
scratch dir so nothing of the repo's was touched (the mechanism
`scripts/verify-hook-freshness.sh` uses):

```
pre-edit  config   87,837 bytes
current   config  107,642 bytes   md5 0c97b6f7… == the INSTALLED hook, byte for byte
diff              324 lines:  0 removed  ·  324 added  ·  all three data-ceiling arms
```

`~/src/posse`'s `posse-prepare-commit-msg` is the same reading: 0 removed, 324
added, and the current-config render is md5 `60bb6e98…` == the installed file.

Zero removed lines is the whole answer. The stale render carried everything the
current one carries except the data ceiling — the beads visibility guard (line
31 of the stale body), the constitution-path guard (1048), the shared-index
guard (1151) and the NOTES.md guard (1245) are all present and byte-identical.
**It was grading by today's rules minus a data ceiling that did not exist when
it was rendered.**

So the finding line over-claimed three of its four names. That is structural,
not a slip: `l3DegradeLine` (internal/posse/gates.go:5769) prints a consequence
string its caller hands it, and the caller (gates.go:5839) hands the same
literal — "the data ceiling, beads visibility, constitution-path and
shared-index guards are not realized" — for `l3Stale`, `l3Uninstalled`,
`l3Foreign`, `l3MemberGone` and the renderer-regression default alike. True for
a slot with no hook in it; for `l3Stale` the probe is holding both bodies at
that moment and declines to diff them, which is the one verdict where the
answer is computable. Filed as ranger-base-e78c4.

## What slipped through: nothing

Window opens 2026-10-02 23:39:07 for all three. It closes:

- `~/src/posse` — at the session create at ~00:28:18 (the 00:26:04 pass). Every
  launch runs `a.InstallCommitGuardHook(dir)` (herdrback.go:2326), and a
  worktree shares its parent's common hooks dir. ≈49m.
- `$CONSTITUTION` and the queue repo — at a hand-typed
  `install-hooks` at 08:36, four minutes after row 3 reported them; both repos'
  `pre-push` and ranger-base's `prepare-commit-msg` carry that mtime. ≈8h57m.

Every commit in each window, scanned on all three ceiling arms (ADDED lines of
the commit, ADDED paths, the commit MESSAGE) against the `ssn` pattern — and
re-scanned under the widest bound the log can defend, each repo's window run on
to the first wall that reported it FRESH:

```
~/src/posse        23:39:07 → 08:32:36   12 commits   0 hits
$CONSTITUTION  23:39:07 → 09:43:41   10 commits   0 hits
the queue repo     23:39:07 → 09:43:41    3 commits   0 hits
```

And the repo the ceiling was written FOR was never unguarded. `~/src/hcn`'s
hooks postdate the edit; its two commits in the window (23:52:29 and 08:30:40)
were graded by the full render. The exposure window covered three repos the
ceiling was not written for and missed the one it was.

## The managed-hooks caveat was the real answer

The bead suspected `posse gates managed-hooks` answering "not managed" for
every repo was a stale-binary artifact. It is not. Main's build gives the
identical line for all five repos asked, and this box has no global
`core.hooksPath` at all, so ADR 0052's managed path never applies here. "not
managed" is the PRECONDITION for the wall to probe a repo (`SweepHookWall`
skips a managed one before probing), so it agrees with the finding rather than
contradicting it.

## Second finding: the pre-heal report goes to /dev/null under a watch loop

herdrback.go:2328 exists for exactly this incident — a repo in the launch
rotation whose stamp had drifted "self-heals SILENTLY and the operator never
learns the wall was wrong, or for how long". It prints through `b.warn`, whose
writer is `HerdrBackend.Warn` else `os.Stderr`; `Warn` is assigned nowhere in
non-test code. The watch loop writes its own log (`WatchLogPath`,
watchlog.go:48), not through stderr.

MEASURED: the running watch — pid 52273,
`posse dispatch --watch 3m --max-interval 3m -n 4 --resume` — has fd 0, 1 and 2
all on `/dev/null`. The 00:28:18 launches into `~/src/posse` are precisely the
event that line reports, and the log has 0 occurrences of it, against 1 in the
rotated `dispatch-watch.log.1` — in a generation whose stdout is doubled
line-for-line, i.e. a hand `tee` launch with `2>&1`. The control works only
when the operator happened to redirect stderr into the log. Row 3 is what the
operator got instead: `~/src/posse` simply absent from the list, with nothing
saying it had been stale for 49 minutes or who fixed it. Filed as
ranger-base-lcode.
