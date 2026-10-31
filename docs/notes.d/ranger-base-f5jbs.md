# `posse runtime drift` — giving skillsVerifiedDrift an exit code

ranger-base-f5jbs, 2026-10-31. Discovered from ranger-base-lf6it §7-8: bob
self-updated 2.0.5 → 2.0.7 and the tree found out because monica's pulse read
an mtime, while the detector that should have tripped was already in the
binary, already saying exactly the right sentence.

## The shape of the defect

`skillsVerifiedDrift(rt)` (internal/posse/parity.go) compares a runtime's
`skills_verified:` against `ProbeCLIVersion`'s live read of `<exe> --version`.
It had two readers before this bead, and both are DISPLAY:

- `parity.go`'s Realized detail — printed at launch, exits 0.
- `runtimecheck.go`'s `skillsRow` — printed by `posse runtime check <rt>`,
  exits 0 over a drift.

Both are correct: ADR 0007 §6c reports and never refuses a LAUNCH, and the
rejected alternative there is an `--allow-degraded` wall at every CLI upgrade.
What was missing is that nothing on a clock read the fact, so the line sat
there right and unread. A detector that only prints is on no clock.

## What landed

`posse runtime drift` — a third reader of the same seam, box-wide, with an
exit code. internal/posse/skillsdrift.go: `App.SkillsDrift()` walks
`ListRuntimes()` (built-ins AND `runtimes/*.yaml`, since a template profile
may declare `skills_cwd:` + `skills_verified:` too), and
`App.ReportSkillsDrift(w)` prints one row per profile and returns
scripts/verify-box.sh's per-check status:

```
0  every profile that could be measured matches its skills_verified:
1  a FINDING: a drift, or a profile posse cannot read at all
2  NOTHING WAS MEASURED — no profile both declares a skills_verified: and
   has a CLI here that answers --version. Not a pass.
```

`Drift` is the single decider — `skillsVerifiedDrift`'s own sentence, not a
re-comparison — so this surface and the two display readers cannot disagree
about what counts as drift. `Declared`/`Installed` sit beside it for the
reader and are never compared.

Three things it deliberately does NOT red on:

- an ADR 0007 §6b UNPROVEN root (`skills_cwd:` with no `skills_verified:`).
  That is a different finding and already a loud one — a `skills:` PID there
  degrades the launch — and there is no version to compare. Reddening it
  would make this clock permanently red on a box behaving as declared.
- a flag-surface profile (claude's shape): no CLI version is the subject of a
  measurement there.
- a launch. §6c is untouched; this is a gate surface beside it.

And one thing it DOES red on that is not a drift: a profile whose yaml
refuses to load. `posse runtimes` already reds there; this reds for its own
reason, which is that a refused profile may declare a `skills_verified:`
nobody can reach, and exiting 0 over one is green over a profile the walk
never saw.

### Why not a shell row over the existing output

Grepping `posse runtime check <rt>` for `drift:` is the forked-matcher shape
ranger-base-s8b4g rewrote three `verify-*-pin.sh` scripts to remove: a
matcher that answers nothing (signalled, EPIPE, cannot exec under load) is
indistinguishable from "no drift", prints green and exits 0 — the exact
failure this bead exists to close. `posse runtime check` has no `--json`, so
the exit code had to come from the Go side that already holds the fact.

## MEASURED 2026-10-31, this box, from the new surface's first run

| profile | `skills_verified:` | installed | verdict |
|---|---|---|---|
| claude | (none — flag surface) | 2.1.314 (Claude Code) | not this clock's subject |
| codex | `codex-cli 0.153.4` | `codex-cli 0.153.4` | clean |
| grok | `grok 1.0.5` | `grok 1.0.52 (0920c8169e27) [stable]` | **DRIFT** |
| bob | `2.0.5` | `2.0.7` | **DRIFT** |

Exit 1, two findings, 3 of 4 profiles measured.

**grok was not known to be drifting.** ranger-base-lf6it was about bob; the
first run of this surface found grok one minor version and one build-hash
format away from the line its discovery root was measured on
(ranger-base-65rc measured `grok 1.0.5`). That is the whole argument for the
surface covering every profile on the same mechanism rather than bob alone —
it was filed to catch the next bob and it caught a grok nobody had looked at.

Both re-measurements are a qa-lane job, not this bead's. grok's is filed as
**ranger-base-9f2o6**; bob's is the operator's half of ranger-base-lf6it,
already open as ranger-base-zneyv (turn bob's self-updater off, or rule that
bob floats). The remedy for a
drift is to prove the discovery root is still read on the installed CLI
(`TestQALiveSkillDiscoveryPerRuntime`, `RHQ_PARITY_LIVE=1`) and then update
`skills_verified:` to the `--version` line that was measured against — never
to bump the declaration to match. A declaration edited to silence this clock
is a measurement nobody took.

A note on grok's declared form: `grok 1.0.5` is not the shape grok's
`--version` prints today (`grok 1.0.52 (0920c8169e27) [stable]`), so the
re-measurement should record the verbatim first stdout line, which is what
`ProbeCLIVersion` reads.

## Not on a clock yet

The surface exists and exits; nothing schedules it. `scripts/verify-box.sh`'s
ROSTER needs a `verify-skills-drift` row whose recipe is
`posse runtime drift`, plus the `boxcheck_qa_test.go` /
`verifybox_qa_test.go` rows the roster census pins. That is a devops-lane
change and is filed as **ranger-base-gf0ln**. The three-way exit status above
was chosen to be exactly that script's per-check contract, so the row is the
recipe and nothing else — and the row will be RED on this box from the day it
lands until the two re-measurements below are done, which is the check
working.

## Tests

- `internal/posse/skillsdrift_test.go` (arm 1) — the walk, over the
  `skillsDriftVersion` seam. Four arms: clean vs NOTHING MEASURED (the pair
  the exit code exists to separate), a profile that will not load, an
  UNPROVEN root that must not red, and the roster coverage line. The seam
  fixtures key on the exe NAME (`driftOnly`), because this box's own
  built-ins are in `ListRuntimes`' walk and a seam answering one version for
  every exe would let whatever CLI is installed here decide the statuses.
- `cmd/posse/skillsdrift_test.go` — the exit code through the real process,
  which is what a roster row consumes. Hermetic by PATH: each arm runs with
  PATH pointing at a scratch bin dir alone, so the built-ins answer no
  `--version` and a planted `driftcli` is the only subject.

MUTATION-CHECKED: dropping the `measured == 0` arm from
`ReportSkillsDrift` (so a walk that read nothing returns 0) reds both the
unit arm and the process arm, each naming the profile it could not measure.
