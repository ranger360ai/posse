# ranger-base-bttuj — the same dead-fixture vacuity, one arm up

*2026-11-12 · gilfoyle · `scripts/reap-test-tmp.sh --self-test`, the KEEP/COLLECT rig.*

Every number below is MEASURED on this box (linux 7.2.15 x86_64, uid 1001,
`/tmp` a tmpfs) on 2026-11-12, tree at `ead3e6fe` plus this change.

## The finding this bead was filed over was already fixed

ranger-base-bttuj reported that the `-p` exclusion arm was green with
`claude_could_register` removed, because its fixture — a copy of `sleep(1)`
run as `claude -p 30` — died on `invalid option -- 'p'` before the count.
**ranger-base-0wvu6 closed that at `7725ba75`, hours earlier and in another
lane**, by the shape bttuj proposed: a fixture that survives `-p` (a copy of
`tee(1)`), a membership test against the production counter's own new pid
line instead of a delta of two box-wide censuses, and a `kill -0` liveness
assertion on both arms. docs/notes.d/ranger-base-0wvu6.md has that work.

Re-graded here rather than taken on trust, by three mutants of the shipped
script, each demanded to red the named arm:

| mutation | result |
|---|---|
| `claude_could_register` deleted (`if true`) at :1107 | `FAIL a live 'claude -p' run (pid N) IS in claude_procs`, exit 1 |
| the same guard inverted (`if false`) | `FAIL control: a non-print claude process (pid N) is NOT in claude_procs` |
| the fixture back to a copy of `sleep` | `FAIL the '-p' fixture (pid N) was already gone when the census ran`, both membership arms |

The third row is bttuj's own repro, and under the shipped rig it is now a
reported broken rig rather than a passing exclusion. The bead's finding is
VERIFIED FIXED and nothing in the production reaper needed to change — as
bttuj itself said, `claude_could_register` was always correct.

ranger-base-om4i3, open over the same two arms for the ambient-drift flake,
is covered by the same commit: the arms take one census and ask whether their
own pid is in it, so no sibling seat's launch is in the answer.

## What was still open, and is what this bead landed

bttuj's "CHECKED AND RULED OUT" said the rest of the rig grades, and with
live holders it does. But the census it was checked under is the one thing
the `-p` arm had taught us not to assume. Of the four background holders the
KEEP/COLLECT rig plants, **arm C is vacuous the moment its holder dies**, for
precisely the reason the `-p` arm was.

C registers a live pid with a deliberately WRONG `procStart` and wants its
dir COLLECTED, which grades the `procStart` match at :1125. A **dead** pid is
collected too — by the `[ -d "/proc/$rpid" ] || continue` gate one line
*above* the match C exists to grade. So a dead holder makes C pass whether
that match is there or not:

| procStart match at :1125 | C's holder | C's verdict |
|---|---|---|
| intact | alive (`sleep 60`) | `ok C collected` |
| **deleted** | alive | `FAIL C: want COLLECTED, got KEPT` — grades |
| **deleted** | dies at once (`sleep 0`) | `ok C collected`, **all arms pass, exit 0** |

That last row is the defect: the production fix deleted, the suite green.
Same signature as the `-p` arm, same function, twelve lines up.

A, D and H are fail-safe by direction — each wants KEPT and a dead holder
gets its dir collected, so they red on their own — but they red as
`want KEPT, got COLLECTED`, which reads as a reaper defect when the reaper is
fine.

## The fix

One arm, after the sweep and before any verdict: assert all four holders are
still alive, and report a broken **rig** when one is not. Checked *after* the
sweep because alive on both sides of it is what says alive during it — these
are sleeps, and a sleep does not come back. `kill -0` is a builtin, as an
assertion arm's matcher must be
(`TestQANoAssertionArmDecidesThroughAForkedMatcher`, ranger-base-t07yx).

Graded, same box, same tree:

| mutation | result |
|---|---|
| procStart match deleted + C's holder exits at once | `FAIL arm C's holder (pid N) was already gone when the sweep ran`, exit 1 |
| procStart match deleted, holder alive | `FAIL C: want COLLECTED, got KEPT` — the real grading is not masked |
| holder exits at once, match intact | the broken-rig FAIL, and not a reaper FAIL |

The first row is the mutant that was all-green before this change. The second
matters as much: a liveness gate that swallowed C's own verdict would be the
same mistake in a new coat.

`make verify-reap-test-tmp` green, `make tree-check` green,
`go run ./cmd/checkorphans` clean after every run above.

## The shape worth keeping

Twice now in one function: **an arm whose fixture's death is
indistinguishable from the behaviour it asserts**. It is not a typo class and
reading the arm does not reveal it — both spellings look like they test what
they name, and both are green. What finds it is mutating the production line
the arm exists to grade *and* killing the fixture, and the two mutations have
to be applied together: either alone leaves the arm looking sound.

The directional tell is cheap, though, and is what to look for next time: ask
which way the arm fails if its fixture dies. An arm that wants the fixture's
effect to be PRESENT (A, D, H — "kept") is fail-safe; an arm that wants an
absence (C — "collected", the `-p` arm — "not counted") is not, because death
produces that absence for free.
