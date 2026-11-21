# The first delegated `posse upgrade`: 19m08s, no rollback (ranger-base-jnf2z)

ADR 0092's Consequences left one number ASSUMED — "15-25 min on a quiet box,
to be measured on the first real run" — and D4 left the rollback triggers
built but never fired in anger. This is that run. It is the first install of
posse on this box that did not go through a human's `make install`.

## The reading

MEASURED 2026-11-20, 23:08:51 to 23:27:59 EST (America/New_York), this box
(16 cores), by gilfoyle at a claude/standard seat realized at shims, from the
worktree `~/.posse/worktrees/posse/gilfoyle-posse-ranger-base-jnf2z`. The verb
was run in the background and waited on by pid, per ADR 0092 D3 and AGENTS.md.

```
posse upgrade                      # no args: the default origin/main target
  edfe7a7 -> 3c0e3c5 (2 commits) · source ~/src/posse · target ~/.local/bin/posse
exit 0 · wall 1148s (19m08s)
loadavg at start 11.98 8.44 7.89 · at end 6.79 9.88 9.78
```

**19m08s, inside the ASSUMED 15-25 min envelope**, against
ranger-base-zhw0h's 11m57s FLOOR for a green `make test` alone. So the
envelope holds and the floor is where it said it was; what the extra ~7m
bought is named below, and most of it is not the verb's own work.

Where the wall went, from the run's own output:

| phase | reading |
|---|---|
| private copy (`git init` + fetch by full sha, D3.2 as amended by p8ymi) | ~1s, consistent with p8ymi's 1.1s |
| tree-wide doors (`make test`'s prerequisites: fmt, crew, treepins et al.) | ~83s, squarely in AGENTS.md's 83-92s warm band |
| **suite-slot queue, arm 1** | **146s** — both slots were held by sibling seats at launch |
| arm 1 (`./...` + gates + silent-revert audit) | internal/posse 214.0s, internal/treepins 154.5s, cmd/posse 119.1s, cmd/coldexec 2.0s, cmd/testparallel 0.2s |
| arm 2 (`-tags posse_arm2`) | internal/posse 81.5s, slot taken immediately |
| arm 3 (`-tags posse_arm3`) | internal/posse 119.1s, slot taken immediately |
| silent-revert audit | 2798 commits, 0 untriaged |
| `clean-build.sh` at 3c0e3c5 | built `~/.local/bin/posse.new` |
| snapshot, swap, verify, epilogue | the readings below |

The queue is the entry worth keeping: **146s of a 1148s wall, 13%, was this
verb waiting on two other seats' suites**, and it is charged to the install
rather than to them. The box was not quiet — loadavg 11.98 at launch, both
suite slots held — so this is a reading of the verb under contention, which is
the condition an install will usually meet. A quiet-box reading is still
unmeasured, and would be expected near the bottom of the envelope.

## The D4 outcome: no rollback, and nothing newly red

Quoting the verb's own two lines, which are the condition D4 judges on:

```
  before: status 8 condition(s), 0 unreadable store(s) · gates 11 persona(s), 36 not-held row(s), 0 non-zero exit(s) · render-probe 0
  after:  status 8 condition(s), 0 unreadable store(s) · gates 11 persona(s), 36 not-held row(s), 0 non-zero exit(s) · render-probe 0
  ✓ nothing the binary renders is newly red — the install stands
```

Every count identical across the two binaries, so **no trigger fired and
there is no condition line to quote**. The three arms, each read through the
NEW binary after the swap: it execs and `posse version` names `3c0e3c5e`; no
`posse gates <p>` that exited 0 now exits non-zero and the not-held count did
not grow; `posse status` gained no URGENT line and no row in a
binary-rendered code. `render-probe` answered **0** on both sides with the
`render-probe:` mark present, so D4 amendment 3's "not asked" reading — which
would have demanded a 0 from the new binary — never came into it.

Verified by execution afterwards, independently of the verb's own counts:
`~/.local/bin/posse --version` is `0.5.3+3c0e3c5e` and
`~/.local/bin/posse.prev` is `0.5.3+edfe7a79`, so one level of rollback is in
place as D3.4 promises; `posse.new` is gone, which is the `rename(2)` of D3.5
having happened; `posse gates gilfoyle` exits 0; `posse gates render-probe`
exits 0 with its mark. `go run ./cmd/checkorphans` reads clean.

The D5 gate arm reported `unchanged` and wrote nothing: the copy at
`~/.config/posse/gate` was already 3c0e3c5's, and
`verify-gate-freshness --behaviour-only` held all three arms against it (`bd
list` allowed, `bd daemon stop` denied by the parser, `go test` untouched).
`verify-hook-freshness` read fresh, 4 repos matching this binary's render — so
this range changed neither the gate pair nor the hook render, and the G12
sequence D7 designs for was not exercised either.

## The D6 line, verbatim

```
upgraded edfe7a7..3c0e3c5 (2 commits) · gate unchanged · rollback: install -m 0755 ~/.local/bin/posse.prev ~/.local/bin/posse
```

Four words were decided for the gate field at acceptance; this run produced
`unchanged`, the first of them, and it meant what it says.

## G11 deepened by a good install — the exclusion, live

While the suite ran, dinesh's ranger-base-hgcmw landed and was pushed:
`origin/main` moved `3c0e3c5e -> 783cdeb9`. So the install that stood up
3c0e3c5e finished with `posse status` reading

```
launcher · 0.5.3+3c0e3c5e is 1 commit(s) behind main in ~/src/posse
```

which is a G11 row that the install itself opened. ADR 0092's "Measured
versus assumed" excludes G11 from the triggers for exactly this reason — "it
is ABOUT which binary is installed, so a good install can deepen it when
origin/main moved further forward than the install did while the suite ran" —
and that sentence was written from a census, not from a run. This run is the
live case: had G11 been a trigger, the first delegated install would have
rolled itself back for succeeding. The second install is ranger-base-8vws2.

One LANE row also appeared between the two readings (a G9 for
ranger-base-3a9i9, assigned to a non-lane). G9 is excluded as a code that
moves on its own, and the before/after counts were taken seconds apart so it
fell outside them entirely — but it is the shape the exclusion set exists for.

## What this does not measure

- A quiet-box wall. 146s of this one is queue, and the rest ran against two
  sibling suites and a loadavg near 12.
- Any D4 trigger firing. No rollback happened, so the triggers are still
  exercised only by their pins, not by a live red. The condition-line
  quotation this bead asked for is therefore absent on purpose: there was no
  condition.
- The gate arm's `installed` or `refused` words, or the G12 re-render
  sequence. This range touched neither the gate pair nor the hook render.
