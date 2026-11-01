## The apply probe resolved its own binary twice, and the second answer was a permission (ranger-base-npqem)

2026-11-01. The fourth surface of ranger-base-d0nyb's class — *a probe for
a name a shipped `protocolCmd`/`protocolBin` site execs, reading THIS
process's gated `$PATH`* — and the one whose wrong answer is **permissive
and silent**.

`sandboxApplyRefusal()` is how every check posse performs *by applying a
seatbelt profile* asks whether it may apply one at all (ranger-base-heur).
Its live reader gated on `SeatbeltAvailable()`, an `exec.LookPath` over this
process's PATH, and then exec'd the same bare name through `protocolCmd`,
which resolves against ADR 0077 D3's **gates-free** PATH and refuses a bare
name nothing out there answers (ranger-base-cwsi8).

**MEASURED** 2026-11-01 (darwin 25.6.0, the filing seat's ranger-base-d0nyb
worktree at main `0861d88d`), with `PATH` = one gates bin dir holding a
`sandbox-exec` shim plus one ordinary dir holding none:

| call | answer |
|---|---|
| `SeatbeltAvailable()` | `true` — it found the shim |
| `protocolCmd(…).CombinedOutput()` | out `""`, err `sandbox-exec: executable file not found outside posse's gates dirs …` |
| `isSandboxApplyRefusal("")` | `false` — it needs `sandbox_apply` |
| `sandboxApplyWhy` | `""` — **"this process MAY apply a profile"** |

The out-of-band `err` is discarded at that call site *by design*: only an
apply refusal is ours to abstain on, and a host problem should be reported
by the check that meets it rather than disappear into an abstention. So
nothing said a word, the abstention never fired, and `reachability.go`
believed a measurement that never happened — which is exactly what
ranger-base-heur was filed for, one layer further out.

### Two surfaces, two directions

The direction here is **not** herdr's or bd's, which is why this was its own
bead.

- **`SeatbeltAvailable()` keeps the gated lookup.** Its binary is not one of
  posse's own children: it is the `sandbox-exec` `startPlanned` types into a
  herdr pane (`PaneRun`), which runs in herdr's process tree carrying the
  *seat's* PATH. The gated lookup is the one that matches how that name will
  be resolved, and a gates-free probe would answer a question neither
  `AvailableCages[CageSeatbelt]` nor the launch gate asked.
- **The apply probe resolves its own binary.** `sandboxApplyProbe` now calls
  `protocolBin("sandbox-exec")` before the exec, and a refusal is an
  abstention of its own: **"could not ask" is not "may apply"**. It no longer
  reads `SeatbeltAvailable()` as a licence to believe its exec will resolve.

That is the whole shape of the fix: two probes, two different binaries, each
resolved the way the thing it predicts is actually run.

### `""` is a permission

The contract `sandboxApplyRefusal` used to state was *"`""` when this process
may apply a profile, and the kernel's own words when it may not"* — a
two-answer function with no room for *don't know*. It is now *"`""` when this
process was MEASURED able to apply, otherwise a sentence saying why that was
not established"*, and only a measurement may produce the empty string.

The row's prose moved with it. `recordReachUnmeasured` said *"this posse
process **may not** apply a seatbelt profile (`<why>`)"*, which is false of
a probe that could not be run; it now says *"is not known to be able to"*.
`why` still carries the kernel's own words whenever they exist, so the
stronger fact is in the row exactly where it is true. `sbSkipUnlessSandbox-
able`'s skip line took the same edit, for the same reason.

### The memo had to be split to be testable

`liveSandboxApplyRefusal` memoizes in a package-level `sync.Once`, and a
`sync.Once` cannot be re-armed: by the time any test calls it, whatever
called it first in the binary has cached the box's own answer. So the asking
is its own function (`sandboxApplyProbe`) and the memo is three lines around
it. The pins drive the asking on a fixture PATH.

### Not firing on this box

Same margin as every other member of the class: `/usr/bin/sandbox-exec` is
on PATH outside every gates dir and no PID here denies it, so nothing renders
a shim for the gated lookup to find. ranger-base-zfljf (open, P1) is the kind
of change that removes such a PATH entry.

### Pins

`internal/posse/seatbeltapplygatesfreepath_test.go`, arm 1. Four arms on the
probe and two on the host question, every arm with its own positive control,
and none of them behind `sbSkipUnlessSandboxable` — every `sandbox-exec` they
reach is a planted script on a fixture PATH, never the kernel, so there is no
apply to be refused and no host grant any of them needs.

MUTATION-CHECKED 2026-11-01 (darwin 25.6.0, this worktree at main
`ea4b5d08`):

- dropping the `protocolBin` guard from `sandboxApplyProbe` — the code as it
  stood before this bead — reds arm (a) alone, with `sandboxApplyProbe() =
  ""`. The other three probe arms and both host arms stay green, which is the
  point of having them: each is a way the probe could have been made to
  abstain for the wrong reason.
- routing `SeatbeltAvailable` through `protocolBin` — the one-line "fix" the
  class invites — reds `TestSeatbeltAvailableStaysTheGatedHostQuestion`'s
  first arm, and reds arm (a) at its **fixture guard** rather than at its
  claim: with the host question answering no, that arm's probe returns `""`
  one line in, which is the same string the defect produced, so the guard
  reports *"fixture did not satisfy the gated lookup"* instead of passing over
  a measurement it never made (ranger-base-z4vx's rule).
