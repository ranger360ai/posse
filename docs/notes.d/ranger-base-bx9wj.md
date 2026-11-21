# ranger-base-bx9wj — the live pins' home was under TMPDIR, so ADR 0100's refusals measured only in the venue

ADR 0100's two refusals — `bwrapRefuseUnpinnedSource` (D2(b)) and
`bwrapRefuseUnpinnedDst` (D4(a)) — stand down when `bwrapWallRoot()` is empty,
and it is empty when this uid's home lies under `/tmp`. That is right for
production: such a box grants `/tmp` unconditionally (`seatbeltTempGrants`),
every private source is in the shared directory already, and there is no seat
for the wall root to protect.

internal/posse's `TestMain` makes the binary's HOME with
`os.MkdirTemp("", "posse-testhome-")`, so that home follows `TMPDIR` — and so
the two refusals were keyed on it:

* uncaged, `TMPDIR` unset: HOME is under `/tmp`, and both refusals are
  **dormant** over every fixture in the package.
* in ADR 0097 D1's venue: `TMPDIR` is `<state>/landed-check/run-<n>/tmp`, HOME
  is not under `/tmp`, and both are **live**.

ADR 0097 D6 keeps the uncaged run as the control for the venue. For this
family of rules the two measured different things, so a green control said
nothing and a venue red could not be reproduced by the command the red's own
advice names first. That is how ranger-base-4gyib's red sat on main for four
landings (docs/notes.d/ranger-base-4gyib.md has that bisect).

## What changed

`bsWideGrantPinsTheLooseComponent` — the arm that reads ADR 0100's refusal off
the seat store's wide-grant shape, and the whole of 4gyib's red — ran inside
`TestLiveBwrapSeatStoreRefusesRewritingASharedObject`, which is `t.Parallel`
and so cannot set HOME. It opened with `if bwrapWallRoot() == "" { return }`:
a silent return under every uncaged run.

It has a test of its own now,
`TestLiveBwrapSeatStoreWideGrantPinsTheLooseComponent`, serial, whose fixture
home is `bsUngrantedHome` — a `<root>/home` under `wallUngrantedDir`, which is
`/var/tmp` and which no blanket temp grant covers. The arm's stand-down guard
is a CONTROL FAILURE there rather than a quiet return, so an empty wall root
under such a home reds instead of passing.

Two things came with it:

* **the live half**, which nothing measured before: ADR 0100's answer to a
  loose component is a mount on it (P4, EBUSY), so the rendered seat is handed
  `mv <common>/worktrees …` and must be refused — with the CONTROL being the
  same argv with that one in-place bind cut, handed to `bwrap` directly rather
  than to the render that declines to produce it, where the rename lands.
* **the asymmetry as a pin**,
  `TestQABwrapSeatStoreFixtureHomeStandsADR0100sRulesUpInBothVenues`: the
  binary's home answers `bwrapWallRoot` by the documented rule and nothing
  else (empty exactly when it is under `/tmp`), and the fixture home answers a
  non-empty root under either venue's `TMPDIR` and lies in none of
  `seatbeltTempGrants`. One pin, both venues, no skip.

**Not the whole binary's home**, and the reason is the cost the bead asked to
be noted before choosing: that home is the process's (`TestMain`,
`FenceCLIHomes`), roughly a thousand tests read it, and every fixture keyed on
"a `t.TempDir()` is inside a blanket grant" would flip with it (`sbRoot`'s
stand-down, 123 tests). A `t.Setenv` is the narrow lever and its price is the
one it always charges: the caller cannot be parallel.

## MEASURED 2026-11-21

This box: Linux 7.2.15-1-omarchy, bubblewrap 0.13.1, go1.27.3, 16 cores.

| reading | before | after |
|---|---|---|
| `go test -run '^TestLiveBwrapSeatStore' ./internal/posse`, `TMPDIR` unset | ok | ok 0.268s |
| the same, `TMPDIR=$HOME/tv` (the venue's shape) | ok | ok 0.319s |
| `go test -run '^Test(QA)?Live(Bwrap\|Wall)' ./internal/posse` uncaged | ok | ok 93.4s |
| `bash docs/adr/0097-wall-venue.probe.sh venue+check` | `rc=0`, green, 46 pins | `rc=0 wall=116s load=7.23`, `wall check: green, 47 pins measured` |

**The mutation run, and it is the deliverable.** With both refusals forced to
stand down (`if bwrapWallRoot() == ""` → `if true` in each), the new pin FAILs
**uncaged, with `TMPDIR` unset** — `with the in-place bind of
<common>/worktrees cut from the argv, neither of ADR 0100's arms refuses the
render`. That is the red the uncaged control could not produce before this
landing.

Pointing `bsUngrantedHome` back at a `t.TempDir()` FAILs at the new control
instead: `the wall root is empty for the fixture home …, so ADR 0100's two
refusals stand down here too and this test measures … nothing`.

The venue's two `note:` FAILs are unchanged and pre-existing — both
`wallunixlive_test.go` socket pins over `sun_path`'s 108 bytes, which the
probe's run-dir prefix spends 52-62 of. They print no ADR 0097 D4 marker, so
they are not members: a red for `make test` and not for this check
(ranger-base-vbp5a).
