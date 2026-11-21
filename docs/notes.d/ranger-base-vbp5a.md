# ranger-base-vbp5a — two socket-wall live pins spent sun_path on their own test names

`TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched` and
`TestLiveWallUnixLeavesCrossDirectoryRename` bound their unix socket under
`t.TempDir()`. A `t.TempDir()` name carries the TEST's own name — 73 bytes for
the first of these, before the subtest suffix and the `/001` — so the path
clears AF_UNIX's 108-byte `sun_path` as soon as `TMPDIR` is not a 4-byte
`/tmp`, and the HOST's `net.Listen` is what fails:

    listen unix <TMPDIR>/TestLiveWallUnixRefuses…cont943109368/001/leader.sock:
    bind: invalid argument

ADR 0097 D1's venue sets `TMPDIR` to `<state>/landed-check/run-<10 digits>/tmp`
(62 bytes on this box), and ADR 0096 D3 makes that venue where these live pins
measure. So the two pins measured nothing in the one place they are run after
every landing, while an operator's `make test` — `TMPDIR` unset — read green
over them.

## Worse than a red: the control arm dies with the step arm

Both pins are written as two arms differing in one word of argv, and this
file's header says why: "A connect that fails proves nothing on its own". A
`sun_path` overflow fails before either arm reaches the wall, so the pin could
not even read as a false pass — the control arm reported no `connected` and the
step arm reported no `EACCES`, and `t.Fatalf` fired in the parent both times.

Not a wall-check red, either. Neither pin prints ADR 0097 D4's member marker,
so `scripts/posse-wall-check` reports them in a `note:` line and exits 0: a red
for `make test` and for nobody's gate.

## The fix

`wuSockDir(t, <name>)`, one helper for both pins, returning
`shortTempDir(t)` and the socket path inside it. `shortTempDir` is this
package's existing answer to the same cap (three live fixtures already use it);
its root is a literal `/tmp`, which the venue binds a private tmpfs over, so
the path is ~36 bytes in the venue and out of it.

The length is then ASSERTED rather than skipped over, which is the one
difference from `wallchecklive_test.go`'s `wallLiveSocketBudget`. That budget
guards a run root an operator may pass in, so going dark is the honest verdict
there; here no environment can reach the path any more and the only thing that
could push it over is a rename in this file — a tree bug, which a skip would
hide. `wuSunPath` is `wallLiveSunPath`'s number written a second time because
the two files are in different build arms (arm 1 and arm 2) and neither
compiles with the other.

## MEASURED

2026-11-21, Linux 7.2.15-1-omarchy, bubblewrap 0.13.1, go1.27.3, 16 cores.

| reading | arm | result |
|---|---|---|
| before, uncaged, `TMPDIR` unset | `-run '^TestLiveWallUnix'` | ok |
| before, `TMPDIR` 62 bytes | both pins, 4 subtests | 4 FAIL, all `bind: invalid argument` |
| before, venue | `0097-wall-venue.probe.sh venue+check` | rc=0, 130s, **2 `note: FAIL`** lines, `green, 46 pins measured` |
| after, `TMPDIR` 62 bytes | both pins, 4 subtests | PASS |
| after, uncaged, `TMPDIR` unset | `-run '^TestLiveWallUnix'` | ok |
| after, venue | `venue+check` | rc=0, 116s, **no `note:` lines**, `green, 46 pins measured` |
| after, venue | `venue` | rc=0, 123s, `{'pass': 52}` — no fail, no skip |

The last row is the one that says the pins PASS in the venue rather than skip
into it: the `+check` arm's silence alone cannot tell those apart.

The 62-byte `TMPDIR` is the venue's own shape reproduced outside it
(`/home/posse/.cache/posse-vbp5a/landed-check/run-0123456789/tmp`), which is
the cheap repro — a full venue arm is two minutes and this is two seconds.

Pre-existing: nothing in ranger-base-4gyib's red window touched this file.
