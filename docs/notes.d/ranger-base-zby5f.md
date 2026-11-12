# Two reds from one landing: a dependency no distro has, and a landmark that moved (ranger-base-zby5f)

e5b3d160 (ranger-base-ihp7y, ADR 0024 D2 check 4) landed green on its own
subject and red on two pins' subjects — four failures across arms 2 and 3,
one landing, two unrelated causes. MEASURED 2026-11-12, this box (linux
7.2.15 x86_64), tree 8ce0f0ce..9d8e97a9: deterministic, 0.02s, every run.
Arm 1 was green throughout.

## Red 1, arm 2: HOOK_DEPS has no answer for `posse`

`TestHookDepsNamesEveryCommandTheRenderedHooksCall` derives the commands
the three rendered hooks call and fails when `scripts/cleanroom.sh`'s
`HOOK_DEPS` disagrees in either direction. Check 4's judge is a bare PATH
lookup of this repo's own binary — `printf '%s' "$posse_added" | posse
gates vuln-private-scan` — so the census grew a name the list does not
have, and the pin said so.

Neither direction of the obvious fix is free:

- **Adding `posse` to HOOK_DEPS** would be one line, and then the
  clean-room probe — which walks that list inside a DISTRO IMAGE and
  reports a `MISSING` as a finding rather than a setup step — would report
  every distro dirty forever. A finding nobody can act on, in the one list
  whose whole job is to produce findings.
- **Exempting it in the test** is the `git` precedent (`git` is what RUNS
  the hooks and is deliberately absent), but `posse` is unlike `git` in the
  way that matters to this file: it really is resolved through PATH.

The decision: exempt, because the hook already answers its own absence.
`vulnPrivateChecks` renders every call inside `if command -v posse
>/dev/null 2>&1`, and a box without the binary skips the arm — the same
fail-open answer as an unreachable bd (ADR 0068's lockout lesson). A distro
without `posse` is therefore not a distro the hooks break on, which is the
only question the probe asks.

**The exemption's premise is measured, not asserted.** An exemption by name
covers whatever is named that way later, including an UNGUARDED call — a
hard dependency on a command the probe has been told not to look for, which
is exactly the drift this file exists to catch. So the pin now also counts
one `command -v posse` test per run of the binary in each rendered hook (2
and 2 today, at check 4's content arm and its message arm).

Counted over the RENDER, and by the pipeline's shape rather than through
`shellCommandWords`: the scanner returns each name once at its first
occurrence, and here that first occurrence is the `command -v posse` test
itself (`command -v` leaves the name in a command position), so it can say
THAT the binary is named and never how often it is run. A call written in a
shape the count cannot see reds the pin on purpose.

MUTATION-CHECKED (`go test -overlay`, worktree untouched): the guard
replaced by `if true` reds it with 2 runs behind 0 tests; the judge reached
without its pipeline reds it with 0 runs behind 2 tests.

## Red 2, arm 3: three pins locating an arm by a banner that was renamed

Check 3's message-arm banner is a landmark. e5b3d160 shared that arm with
check 4 (`identityGuardCheck`'s new `extraMessage` parameter, one message
reader for every source that scans the message) and renamed the head from
`check 3, third arm: the commit MESSAGE` to `check 3 (+ check 4, same
reader)`. It updated `dataceiling_qa_test.go` with the rename and left
three arm-3 pins on the old spelling:
`TestQACheckThreeMessageArmRenders`,
`TestQACheckThreeMessageArmIsSeenIdenticallyByTheL3Probe` and
`TestQACeilingBannerDoesNotDenyCheckThreesMessageArm`.

**The rename is load-bearing, so the pins moved and the banner did not.**
Check 4 is unconditional — an open `vuln-private` bead is a live fact about
this repo, not a derived identity literal or a configured pattern — so the
shared banner now renders even when check 3 derives and configures nothing,
and `TestQADataCeilingRendersAboveTheGateUnderEveryStamp` asserts `third
arm: the commit MESSAGE` (still the ceiling's own wording) ABSENT from
exactly that render. Restoring "third arm" to check 3's head would make
that absence assertion unsatisfiable.

The third pin is the one that earned its keep: it is an absence assertion
with a control, and the control is what reported the breakage — "the
rendered hook is missing ... so the absence assertion above measured
nothing". Without the control, the claim it holds (no wall's prose may tell
a refused writer that the other wall does not scan the message) would have
gone silently unchecked.

Two further things were wrong in the same block and are fixed with it. PIN
(s)'s last assertion — "an empty check 3 and an empty ceiling must render
no message arm at all" — had become false in substance, not only in
wording: check 4's arm renders there now. It asserts what an empty check 3
must still cost (none of check 3's own three arms) and, as the other half,
that the reader they share IS present for check 4.

MUTATION-CHECKED (`go test -overlay`, worktree untouched): `extraMessage`
dropped from `identityGuardCheck`'s empty-set early return reds the second
half; `twoArmScan` rendered unconditionally reds the first.

## What the class costs, and where it is written down

A render-prose landmark retyped in four files has no mechanism holding the
copies together, and this repo prefers it that way — a pin that reads the
render is the pin that catches a render change, and "a pin over source
prose ages into a grep" (`ceilingarmclaim_qa_test.go`). The cost is that a
deliberate rename is a four-file edit and the next person has to find the
other three. `checkThreeMessageHead`'s own comment in `gates.go` now names
all four pins and says why the wording is what it is, so the next rename
starts from the list instead of from a red suite.
