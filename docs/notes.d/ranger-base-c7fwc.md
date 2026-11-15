# ranger-base-c7fwc: the RunWallUnix ABI-floor refusal, pinned

ranger-base-i95x2's finding 1. `RunWallUnix`'s floor refusal
(internal/posse/wallunix.go) was reached by no test, so it could be deleted
with the whole wall-unix pin set green — the one survivor of six mutants aimed
at ranger-base-e5un1's work. The guard itself worked; what was missing was the
pin.

## Why stubbing the ABI alone could not pin it

`RunWallUnix`'s order is LookPath, floor check, apply, `syscall.Exec`. A pin of
the floor check stubs `wallUnixABI` below `WallUnixSignalABIMin` — but the
kernel the pin RUNS on is whatever the box has, and on an ABI 6+ box
`liveRestrictWallUnix(5)` builds `{Scoped: SCOPE_SIGNAL}`, which that kernel
ACCEPTS. So with the floor check deleted the next statement executes: the
mutant applies a signal-scoped Landlock domain to the test binary and then
replaces it with the runtime. It does not red, and it takes the rest of the
test binary with it.

That is why `restrictWallUnix` is now a var over `liveRestrictWallUnix`, the
shape `wallUnixABI` already had and for the same reason one step further on.
The rename is the whole of the production change; the one production caller
still calls the live function.

## MEASURED 2026-11-15, D1 (Linux 7.2.15-1-omarchy, Landlock ABI 10)

`TestQAWallUnixStepRefusesBelowTheSignalABIFloor`, three rows
(`WallUnixSignalABIMin-1`, `1`, `-38` for the syscall absent):

- As landed: `go test -count=1 -run
  'TestQAWallUnix|TestLiveWallUnixScopesSignalsToTheSeat|TestQALandedCheckJudgeTypesNoSocketWallStepBelowTheABIItNeeds'
  ./internal/posse` → ok, 2.958s.
- Against the bead's mutant (`if false && abi < WallUnixSignalABIMin`): all
  three rows FAIL, and each one's first line is the precise fault — `the apply
  was called 1 time(s), so the floor check did not refuse`. The test binary
  survived the run, which is what the apply stub bought.

The pin's command is `true`: it resolves on every PATH, so the LookPath arm
above the floor check cannot be what refuses, and the refusal the pin reads is
the floor's.

## What the pin defends, precisely

Not a security boundary. On a REAL sub-floor kernel the ruleset is rejected and
the apply refuses one line later, so the seat does not open unwalled either
way — that clause is DERIVED from the kernel's trailing-non-zero-field
behaviour and was not measured, because this box is ABI 10. What had no other
holder is the guard's WORDING and its ADR 0080 D4 claim, so the pin reads the
sentence: the floor number, `LANDLOCK_SCOPE_SIGNAL`, `ADR 0080 D4`, and the ABI
text as the step read it.
