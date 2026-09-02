## What keeps a session out of the auto-reap (ranger-base-kftx)

The end-of-pass sweep (`internal/rhq/autoreap.go`) reaps a session when the
store of record calls its bead **closed** and **nobody is working in it**.
The second half was measured short: on the live fleet 2026-08-27, five
sessions had a finished agent over a closed bead and `posse dispatch
--dry-run` named two.

**Nobody is working in it** now means one of two readings of herdr's status,
not one:

| herdr status | reading | reaped |
|---|---|---|
| `idle` / `done` | a settled agent | yes |
| `working` / `blocked` | somebody is in there — `blocked` is a persona *waiting* | no |
| `""` past `RelaunchGrace` | the CLI exited: a bare shell, the operator's "dead shells" | yes |
| `""` inside `RelaunchGrace` | detection is blind for the first seconds of a launch | no |

`Sessions()` reports a status only where herdr's `agent list` shows an agent
in the workspace, so `""` is *no agent detected* — which is the same reading
for a CLI that has died and one that has not finished starting. posse already
had one answer for that ambiguity and this reuses it rather than coining a
second: `RelaunchAgent` refuses to re-type into a session younger than
`RelaunchGrace`, and dispatch's own `else if s.Status == ""` arm relaunches
past it.

**RelaunchGrace, not StartupWait** — the two were split by ranger-base-ze9p
precisely here. `StartupWait` is the pass's *detection patience* and tests
shorten it to stay fast; `RelaunchGrace` is "how long a starting CLI may stay
invisible to detection", measured against a session's real age, and nothing
that shortens a test may shorten it. In production both are 45s.

### The population is the pointer, and the crew mark fires first

The sweep's population is sessions carrying a `bead:` pointer, never sessions
whose *name* ends in something that looks like a bead id: `sessionSanitizeRe`
folds `.` into `-`, so a session name is a lossy encoding of an id and cannot
be inverted back into one.

The measurement above read the three unswept sessions as hand-launched, and
that attribution is wrong in a way worth keeping: **every hand path marks the
session CREW** — `posse new` (`cmd/posse/main.go`, `case "new"`), `posse
up`/`local`, and every recipe — and ADR 0008 puts a crew session outside every
sweep one arm *earlier* than the pointer test. So `posse new --bead <id>`
would buy nothing: the pointer would sit on a session the sweep already
skips. The supported way to hand a session to dispatch is `posse crew --off`,
after which dispatch's own `NoteBead` stamps the pointer the moment it
resumes a bead into it.

What actually reaches the pointer arm is a meta written by a binary from
before `bead:` landed (4793e00, 2026-08-26 — the three measured sessions
carried no `repo:`/`branch:` either, which the worktree launch has stamped
since 32ccff0 the day before). The fleet runs the *installed* binary, so
those keep appearing until the operator promotes. `posse ls` now tags them
`🏷️no-bead` (`NoBeadTag`, `UnpointedBeadSession`) so a session outside the
sweep is visible rather than silent — the silence is what read as a broken
reaper and cost the hand-reaps.

---

**Superseded in part by ranger-base-f6lk (2026-09-02).** Option (b) — tag the
boundary and leave the reap to the operator — was not enough: one such session
sat idle 12h+ and was hand-reaped. The sweep now has an *unpointed arm* that
takes these on age (`reap_unpointed_after:`, default 1h) and a provably empty
tree, with no bead read at all. Everything above still holds: the pointer is
still never inferred from the name, and the crew mark still fires one arm
earlier — for a session the OPERATOR made. See `ranger-base-f6lk.md`.
