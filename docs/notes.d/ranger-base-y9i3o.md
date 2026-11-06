# Four conjuncts of the ADR 0079 D2 compare now carry a witness (ranger-base-y9i3o)

MEASURED 2026-11-06 on this box (darwin 25.6.0, git 2.50.1 Apple Git-155), in
gwart's worktree at `main` 8c75672f. From ranger-base-pr6p8's findings 1-4
(docs/notes.d/ranger-base-pr6p8.md §1): four conjuncts of
`renderHookSkipGlobalScope` (hookskip.go, ADR 0079 D2) could each be deleted
with the whole shipped `TestQAHookSkip` filter staying green. Three new pins
close that, and ADR 0079 pin 8 is amended to say what it actually measures.

## What the pins are

| ADR 0079 pin | conjunct | shipped as |
|---|---|---|
| 13 | the have listing's `[ $? -le 1 ] \|\| return 1`, the want SYSTEM read's `[ $? -le 1 ] \|\| exit 1`, and each want CANDIDATE read's | `TestQAHookSkipGlobalCompareChecksEveryReadRC` |
| 14 | `--includes` on the want SYSTEM read | `TestQAHookSkipSystemIncludeIsFollowedOnTheWantSide` |
| 15 | `--includes` on the want CANDIDATE read | `TestQAHookSkipGlobalIncludeIsFollowedOnTheWantSide` |

All three live in `internal/posse/hookskipwitness_qa_test.go`, arm 1. The
scratch dirs are not the tree, so none is a tree-wide pin and none owes the
Makefile a door.

## The mutation matrix

Each arm applied to `internal/posse/hookskip.go` alone, md5-checked, built,
and run against `go test -count=1 -run 'TestQAHookSkip' ./internal/posse -v`
— 17 tests, 20 real subtest legs, 0 skips. Unmutated: **ok, 11.704s**.

| arm | mutation | md5 | red |
|---|---|---|---|
| 0 | none | `58b3881b84e477eef876f9095c601ddb` | — (ok) |
| 1 | delete the have listing's rc check | `0a875d30bd1c540d780084d33d35a651` | pin 13, leg "the have listing's git failed" |
| 2 | the want SYSTEM read's rc check → `;;\nesac` | `90d7998c9d03f3b74fa41b2c07fe113c` | pin 13, leg "the want SYSTEM read's git failed" |
| 3 | drop `--includes` from the want SYSTEM read | `0635be73096ed85bad48f747e2d9c65e` | pin 14 |
| 4 | drop `--includes` from the want CANDIDATE read | `158c1c58debc7746dd4d6e4f152a82e4` | pin 15 |

**Every arm reds exactly one new pin and no shipped pin.** That is both
directions of the claim: each new pin is the only witness its conjunct has,
and pr6p8's finding — that the shipped filter does not see these four —
reproduces at this commit rather than being taken on report.

## Why pin 13 is rendered over a stub git and not real git

Real git cannot be made to fail exactly one of the compare's reads. The two
sides read the same files and differ only by `HOME`/`XDG_CONFIG_HOME`, so:

- A value real git dies on dies on **both** sides. `--type=path` exits 128 on
  a `~nosuchuser` whatever HOME is, so the have listing fails too and its own
  rc check refuses first — which is why pr6p8's finding 1 could not be
  witnessed by extending the `~user` fixture.
- Routing the failure through an include does not separate them either:
  `include.` is in `hookSkipRegexp`, so an `include.path` is itself a key the
  have listing prints, and the mutated want side (which contributes no
  `system` lines at all) then differs from the have side and refuses anyway.
- The one route that does separate them is a config file git cannot parse:
  MEASURED, a malformed global file exits **128 with stdout empty on BOTH the
  `--type=path` and the `--name-only` read**, so the entry counts agree at 0
  and the have side reads `""`. That needs the want side to read `""` too —
  i.e. a launcher with no hook keys in global or system scope — and it also
  kills the `git commit` itself, so the witness would rest on which rule
  names the refusal rather than on whether the call is allowed.

So pin 13 renders the shim over a stub git that prints nothing and returns a
chosen rc per read, as ADR 0079 pin 7 does, and carries its own baseline leg:
with every read agreeing, the same moved HOME is **allowed**. Without that
leg a refusal would say nothing about which conjunct caused it.

## What pin 8 was green on

ADR 0079 pin 8 (`TestQAHookSkipMovedHomeFollowsNoTildeInclude`) is cited as
the include pin. Arm 4 above leaves it green: what refuses its fixture is the
`~` in its `include.path` expanding differently under the two HOMEs, not the
include being followed. Pin 15's fixture spells the include path ABSOLUTE, so
both sides read the same included file, agree, and the call is **allowed with
the managed hook firing** — which is the only shape that needs the flag,
since dropping it can only ever make the want side read less. Pin 8's
comment and the ADR entry now say so; the tilde legs stay as they were.

## Severity, unchanged

Nothing here is a known hole, and this stays debt. The reachability reasoning
is pr6p8's and is reasoning, not measurement: the have listing's rc check
looks subsumed by the entry count two lines below it for every input either
of us could construct, and the two `--includes` are fail-closed — dropping
one only makes the want side read LESS than the have side, which is
over-refusal. What is now true is that a refactor that thins any of the four
reds a pin.
