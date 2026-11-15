# Three pin gaps from the ranger-base-lng6p verify, and the one that made a whole invariant unfalsifiable

ranger-base-lng6p, 2026-11-15. A four-close verify (ranger-base-5f5kb,
ranger-base-5mtwr, ranger-base-00req, ranger-base-n01lc). All four closes
verified: every DO item landed, every claimed pin exists and runs, and nothing
on main misbehaved. The three findings below are PIN gaps in the set that
landed with ranger-base-5f5kb, found by mutation, and all three are closed by
the pins this fragment lands beside.

Environment for every reading: Linux 7.2.15-1-omarchy, 16 cores, go1.27.3,
bubblewrap 0.13.1, this worktree, warm build cache. Mutants were planted one
literal replacement at a time over an otherwise unmodified tree by a script
run by path, each file's sha256 asserted against baseline after every restore,
and the battery's own `finally` restoring all three files before it printed a
summary.

## The control, measured by hand before any of it

The refusal ranger-base-5f5kb landed is pinned over a real property of this
host, and it is worth knowing it was re-measured rather than read: handed the
argv the OLD render emitted for a symlinked private `/tmp` source, bubblewrap
0.13.1 follows the link and the seat's write lands in the link's target on the
host. A full `bwrap --ro-bind / / --dev-bind /dev /dev --unshare-user --bind
<src> /tmp sh -c 'echo > /tmp/planted'` costs **3 ms** here, which is also why
the live pin reporting `0.00s` is a real exec and not a vacuous pass — a thing
worth checking before calling a fast live arm empty.

## Gap 1 — the wall root's own resolution was unmeasured

`SeatWallRoot` composes from the RESOLVED home on purpose: the grant side of
ADR 0100 D2(a) is `absResolve`'d, and only the resolved spelling says where a
bind would land. Nothing measured that resolution.

MEASURED by mutation — `filepath.Join(absResolve(home), seatWallRootRel)` ->
`filepath.Join(home, seatWallRootRel)`: every pin in the package stayed green.
The four D2(a) arms each set HOME to a real directory under the ungranted
fixture root, where `absResolve` is the identity, so all four hold with the
resolution dropped.

A home reached through a SYMLINK is the case that separates them, and it
separates them in the direction that matters: with the root composed from the
unresolved home and the grant still resolved, the two sides are spelled in
different trees, `insideDir` answers false for every grant, and D2(a) refuses
NOTHING on such a box — box-wide, since the root holds every seat's wall dir.
Not reachable from this instance's home today (`/home/posse` resolves to
itself, MEASURED); reachable wherever a home sits under a symlinked volume.

Pin: `TestQAWallRootGrantRefusesAHomeSpelledThroughASymlink`, which also
refuses to measure nothing — it fails the fixture outright if the two
spellings of the wall root have not diverged.

## Gap 2 — the wall dir's removal at retire was unpinned

ADR 0100 D1 gives the retire order, and `RemoveSessionTree` writes it. Nothing
measured that the wall dir goes at all — nor that the seat CACHE dir does,
unpinned since the cache dir was introduced.

MEASURED by mutation — `os.RemoveAll(seatWall)` -> `_ = seatWall`: nothing in
the package reds.

Why it was unreachable before: `RemoveSessionTree` composes both dirs from
`os.UserHomeDir()`, so a fixture that leaves HOME alone names a path under the
operator's real `~/.cache` keyed by a hash of a temp dir — a path that never
existed, whose `RemoveAll` is a no-op either way. Setting HOME is the whole of
what makes the observable visible, and it is why none of the eleven existing
`RemoveSessionTree` fixtures saw it.

Pin: `TestQAWallSeatWallDirGoesWithTheRetiredTree`, which plants the one file
the private `/tmp` bind exists to keep on the host and asserts both per-seat
dirs are gone after the retire. What it does NOT cover, said rather than
implied: the ORDER D1 asks for. A mutant that removes the wall dir EARLY as
well as late survives this pin and every other in the package. The order is
inert until the seat's own object store moves into the wall dir, which is when
a pin on it can observe something.

## Gap 3 — the stand-down made D2(a) unfalsifiable, and this is the one that matters

D2(a) stands down where the launcher's home is itself under the shared `/tmp`,
by design and for a documented reason. Every D2(a) arm therefore reads the
rule through a guard that SKIPS when `bwrapWallRoot()` answers `""`.

The cost is that the guard cannot tell "inert by design" from "inert because
the rule is gone". MEASURED by mutation — `bwrapWallRoot`'s
`return SeatWallRoot(home)` -> `return ""`: D2(a) is disabled box-wide and
**not one test in internal/posse reds**. All four landed arms, and both arms
added for the gaps above, skip quietly; `go test` exits 0 over a seat wall
that refuses nothing at all. This is the only one of the five mutants in this
battery that survived the pin set as it stood after gaps 1 and 2 were closed.

Pin: `TestQAWallRootStandDownIsNotInForceForAnUngrantedHome`, which asserts
the PREMISE the other arms skip on — for a home under no blanket temp grant,
the stand-down is not in force and `bwrapWallRoot` names the same root
`SeatWallRoot` does. The fixture helper has already skipped if this host's own
ungranted root is inside a blanket temp grant, which is the one honest reason
for such a home to stand the rule down; past it, an empty answer is the rule
being gone. Re-measured after landing: the same mutant now reds, and reds only
this arm.

The shape generalises past this ADR. **A pin that reads its subject through a
stand-down guard measures nothing when the subject is removed, and a suite of
such pins is green over an absent invariant.** The remedy is cheap and is one
arm: assert the premise, for a fixture built so the premise must hold.

## What the landed pin set already caught

Two further mutants were aimed at the arms ranger-base-5f5kb landed, and both
died, so this fragment is not a complaint about that set:

- the D2(a) containment test reversed (`insideDir(gr, root)` ->
  `insideDir(root, gr)`), which would admit exactly the grants the rule exists
  to refuse: red in five arms, four of them the landed ones.
- the component walk stopping at its own trusted root (`for i := -1` ->
  `for i := 0`), so a link planted AT the root is no longer seen: red in the
  landed link arm and in the private-source arm of the claude state pins.

## Recorded, no fix landed here

- `internal/posse/bwrap.go:433` — the private `/tmp` step's head comment still
  says "The directory lives under the seat's cache dir, so the worktree remove
  that takes the cache takes this too". It lives under the seat's WALL dir now
  and the removal that takes it is the wall dir's, which the same comment
  block says correctly twenty lines further down. Still on main past the
  seat-store slice.
- ADR 0100's "Not closed here" section names two residuals. Two more were
  filed against its own D2/D4 text before it landed and are not named there,
  so a reader of the record cannot tell that one of its decisions is knowingly
  one review behind.

## Not closed here

- ranger-base-dw2du
- ranger-base-julaf
- ranger-base-gn050
- ranger-base-rpyn0
- ranger-base-shft5
