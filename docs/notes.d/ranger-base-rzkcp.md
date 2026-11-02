# The install that stales every hook on the box said nothing about it (ranger-base-rzkcp)

Bead: ranger-base-rzkcp, from ranger-base-j0imz. Date: 2026-11-02.
Class: devops. Lane: gilfoyle.

## The defect

The L3 commit-wall hook bodies are compiled into the posse binary, so every
installed hook on this box is a COPY that was correct when it was written.
Two things re-render one: `posse gates install-hooks <repo>`, typed by hand,
and a session create — which refreshes the COMMON hooks dir of the repo the
session was cut from and no other (ranger-base-ixv4). A repo that never holds
a session is re-rendered by nothing at all.

So `make install` is the single moment every hook on the box goes stale: the
new binary lands and, in that instant, each installed hook is a copy of an
older render. The target said nothing about it. Its epilogue ran only
`scripts/verify-gate-freshness.sh --warn`, which measures the operator's bd
argv gate copy and asks nothing about the hook wall (MEASURED 2026-11-02
against Makefile:76 and that script). The sweep that DOES detect hook drift —
`SweepHookWall`, internal/posse/hookfresh.go — fires at `posse promote`'s
epilogue and at the `dispatch --watch` preamble, neither of which is the
install.

The window, MEASURED on ranger-base-j0imz from mtimes, 2026-11-01:

| time  | event |
|---|---|
| 21:22 | `make install` promotes the binary carrying the ranger-base-uvcj8 thirteen-rune fold |
| 21:30 | the security lane reads all four hooked repos stale — still the ranger-base-g55ah-era two-rune table |
| 21:33 | a hand-typed `posse gates install-hooks` repairs three of them |

Eleven minutes with the ranger-base-l717c bypass live in every hooked repo
and the fix sitting in the binary the install had just promoted. Nothing on
the box would have said so; the security lane went looking.

## The decision

Option 1 of the bead: DETECT at install, repair nothing.
`scripts/verify-hook-freshness.sh` already existed (ranger-base-8zki) with a
`make verify-hook-freshness` target and a place on `verify-box`'s roster —
what it did not have was a site that fires without someone deciding to check.
It is now the last line of the `install:` epilogue.

Repair at install (option 2) was not taken. It would rewrite the hooks of
checkouts other sessions share, and ranger-base-ixv4's stance is that such a
rewrite is a change someone should type. A finding prints the
`posse gates install-hooks <repo>` line per stale repo instead. That stance is
unchanged by this bead, which is why this is one Makefile line and not a new
mechanism.

## Why POSSE is bound rather than resolved

```make
@POSSE='$(BINDIR)/posse' scripts/verify-hook-freshness.sh --quiet || true
```

The script's reference is not a checked-in string: it is a fresh
`posse gates install-hooks` render from `$POSSE`, taken per repo under a
`core.hooksPath` redirect into its own tmpdir. That is what keeps the control
from drifting away from the renderer — and it is also what makes the binary it
renders from the whole question. A reference rendered by the binary this target
just REPLACED matches the hooks that binary wrote, so every hook the install
staled reads `fresh`: a green at exactly the moment the check exists to fire.

`command -v posse` cannot be trusted to be that binary. MEASURED 2026-11-02 on
this box, `which -a posse` answers two paths, and the first is not the
installed one:

```
~/.config/posse/state/gates/<persona>/bin/posse   # the persona gate shim
~/.local/bin/posse                                # $(BINDIR)/posse
```

and `$(BINDIR)` need not be on PATH at all — `scripts/path-warning.sh`, one
line above in the same epilogue, exists for that case.

`--quiet` keeps a clean box to one verdict line; every finding names its own
repo (`finding "$short: ..."`, eight call sites) and is restated with the
prescription at the end of the stream whatever the flag. `|| true` because a
stale hook is not a reason to fail an install that already happened — the
binary is in place and the operator needs the line, not an exit code.

## Measured

- The epilogue command, run against the live box 2026-11-02 with
  `POSSE=~/.local/bin/posse` and installed posse 0.5.3+8dab9487:
  `verify-hook-freshness: fresh — 4 repo(s) match this binary's render, stamps
  agree with config`. One line. 12.45s over four hooked repos, against a
  `release` build that costs minutes.
- `make -n install` expands the line to
  `POSSE='<$(BINDIR) absolute>/posse' scripts/verify-hook-freshness.sh --quiet || true`,
  after `install -m 0755 bin/posse-release <$(BINDIR) absolute>/posse` — make
  expands `$(BINDIR)` to an absolute path, so the binding names the file the
  line above wrote and not a relative guess.

## The pins, and the one a grep could not have made

internal/treepins/hookfreshness_qa_test.go, three arms. Arm 1 reads the
`install:` recipe: the script is named, `POSSE=` is bound ahead of it,
`$(BINDIR)/posse` is what it names, the line runs AFTER the write that
installs that binary, and it ends `|| true`. Arms 2 and 3 RUN the script with
the flags read off that recipe line.

Arms 2 and 3 exist because the failure this wiring invites is silent. The
script's argument parser is a strict `case "${1:-}"` that answers an unknown
flag with a usage line and exit 2 — and the recipe's `|| true` swallows
exactly that. A recipe that copied `--warn` off the verify-gate-freshness line
beside it would print `usage: verify-hook-freshness.sh [--quiet]`, measure
nothing, and leave the install green. No grep over either file can see that;
a run can. Arm 3 is the bead's DONE clause: with those same flags a stale repo
is still a finding that names the repo and prints the install-hooks line —
the thing a quieting flag is able to take away, since it drops every per-repo
header and leaves the findings to stand on their own.

Four negative controls, each MEASURED red 2026-11-02 and then restored:

| injected | arm that caught it |
|---|---|
| the `POSSE=` binding dropped from the recipe line | 1 — "comes from whatever PATH resolves" |
| the recipe line removed entirely | 1 — "`install:` does not run scripts/verify-hook-freshness.sh" |
| `--quiet` swapped for `--warn` | 2 — "which this script does not accept", with the usage line |
| the STALE finding stops naming `$short` | 3 — the operator is told nothing about which checkout to type in |

`r.run` is variadic now (`run(t, args...)`); every existing arm passes no flag
and is unchanged.

## What this does not close

Nothing re-renders a stale hook automatically, by design. The remaining
exposure is the same one ranger-base-ixv4 accepted: between a `make install`
and the operator typing the printed command, the hooks on this box are behind
the binary. What changed is that the install now says so, in the same breath
as promoting the binary that caused it.
