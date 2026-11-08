# The bwrap render, as built: three things D17 did not settle (ranger-base-qoamy)

ADR 0080 D17, the pure render from the L2 lists to a bwrap argv, built
2026-11-08 as `BwrapArgs` in `internal/posse/bwrap.go`, pinned by
`internal/posse/bwrapargs_qa_test.go` (arm 1, 16 tests, 0.5s). No launch
wiring: nothing calls it, which is D18's.

The five shapes went in as D17 lists them. What follows is what the build had
to settle, in descending order of how much it matters to whoever picks up D18
or D19/D20.

## 1. Mount order IS SBPL's last-match-wins, and the gates dir is the proof

The thing that makes one tier with two mechanisms honest is that the two
mechanisms resolve a conflict the same way. At L2 the profile is
`(allow …)` then `(deny …)` then `(allow …)` and SBPL takes the LAST match.
At this mechanism there is no last-match rule — there is a mount table, and a
later mount covers an earlier one. Those turn out to be the same rule as long
as the groups render in D17's order, and the proof is the persona's own gates
dir, which is in all three lists at once:

- it is a write GRANT (`SeatbeltWritable` adds `gatesDir`);
- `state/gates`, its parent, is a DENY (`SeatbeltCarveOut` item 2);
- its `refusals.log` and `shell.log` are KEEP literals.

Under SBPL the deny beats the grant and the two literals beat the deny, so a
caged session cannot write its own shims or its own `seatbelt.sb` and can
append to its two logs. Rendering grant → deny → Keep reproduces exactly
that: `--bind <gatesDir>`, then `--ro-bind <state/gates>` over it, then
`--bind` on each log.

Worth writing down because the first assertion I wrote had it backwards — it
read the deny arriving after the grant as the grant being buried by mistake,
which is the one reading under which the session gets its shims back. The pin
that says it is `TestBwrapArgsRendersTheTiersOwnLists`, and it is the only arm
driven from `SeatbeltWritable`/`SeatbeltCarveOut` rather than from a
hand-built list, for that reason.

## 2. DIVERGED from D17's letter on a missing deny, and kept to its reason

D17 shape 2: "If `d` is missing and its nearest existing ancestor is inside a
write grant, the launcher makes it as an empty directory first. Otherwise
nothing can create it and no mount is rendered."

The rule and its stated reason disagree on one shape, and this fleet has it:
**a write grant that does not exist yet.** On a fresh home `a.StateDir` is a
grant and `a.StateDir/gates` is a deny. If the state dir is not there, the
deny's nearest EXISTING ancestor is the home — which no grant covers — so the
letter renders no mount. But the grant one line above is about to be created
and bound read-write, so the session CAN make `state/gates` and write in it,
behind a wall that reported success.

`BwrapArgs` asks the reason directly instead: is the deny path inside a write
grant. That is never narrower than the letter (a grant covering an ancestor
covers the descendant too) and it covers the prep-created grants as well.
`TestBwrapArgsPrepsAMissingDenyInsideAMissingGrant` is the arm;
MUTATION-CHECKED, implementing the letter reds it alone.

In practice `RenderSeatbelt` does `MkdirAll(gatesDir)` before rendering, so
the shape would not have appeared at the shipped call site today. That is the
argument for fixing it now rather than for leaving it: the hole is one
reordering away and nothing would have said so.

## 3. Two error cases D17 does not name

- **"Could not ask" refuses the render.** Three of the five shapes stat the
  tree, and the tempting spelling — `os.Stat`, test `os.IsNotExist`, treat
  everything else as "not there" — turns an unreadable path into a missing
  one. For a deny, missing means no mount at all, so a `chmod` on a parent
  directory would quietly remove a wall. Same safe failure direction as
  `bwrapApplyProbe`, and for the same reason: the value of the answer is that
  a seat runs behind it. `bwrapStatRefusal` names the path and the LIST it
  came from, because which list it is decides whether the fix is a PID, a
  home or a mount.
- **A DIRECTORY at a Keep path refuses.** Keep is two literal FILES. A
  directory at one of those names would bind a whole tree back open inside
  the deny the carve-out just closed — a wider wall than anything asked for,
  and one no reader could spot in the argv. Fail-closed, which is the tier's
  own direction.

## 4. The residual D18 inherits: darwin literals become host writes

`SeatbeltWritable` adds four paths as unconditional literals that only exist
on darwin — `/private/tmp`, and `Library/Caches`, `Library/Logs` and
`Library/Application Support/go/telemetry/local` under the home. At L2 a
grant for a path that is not there costs nothing: the profile grants it and
nothing is written.

Under this mechanism a missing grant is not free. bwrap cannot bind a source
that does not exist, so D17's answer is that the launcher `mkdir -p`s it
first — and the launcher runs OUTSIDE the namespace, as the seat's user. So
on Linux the prep list for every seat would carry

    mkdir -p /private/tmp

at the filesystem root, which an unprivileged launcher cannot make, plus
three junk directories in the seat's home. The first is a refused launch; the
three are writes posse has no business making.

Not measurable on darwin, where all four exist and the render asks for
nothing. Named here rather than filed, because it is one predicted defect
with two possible owners and the choice is the next bead's:

- narrow the four literals to darwin in `SeatbeltWritable` (and give the
  Linux home its XDG equivalents), which fixes it for both mechanisms; or
- have D18's launcher refuse a prep step it cannot make, and say which.

The second alone leaves the wall announcing grants it does not have. The
first is the smaller diff and the one I would take.

## 5. Two inputs with no bwrap spelling, and one list that renders nothing

`BwrapArgs` takes neither of `SeatbeltProfile`'s other two arguments, and
`Seal` renders nothing. All three are decisions:

- **siblings** (`SeatbeltSiblings`). An SBPL regex grants the scratch
  namespace beside a granted FILE; a mount cannot name a namespace. D19 makes
  it moot by moving claude's config into the whole-dir grant its
  `CLAUDE_CONFIG_DIR` already points at.
- **createOnly** (`sessionRefDirs`). A mount cannot say "making this path is
  allowed, writing inside it is not". D20 replaces it with a bind of the
  ref's parent directory plus a keep file.
- **Seal.** At L2 the seal exists because a subpath deny is a statement about
  a PATH, so `mv a b` carries the denied tree out from under its own deny. A
  mount is a statement about a mount POINT, and probes B3 and B4 measured
  that renaming a mountpoint, or carrying it away by renaming an ancestor, is
  refused. `TestBwrapArgsRendersNothingForSeal` asserts the render is
  byte-identical with the seal and without it — stronger than grepping for
  the sealed path, because the seal's directories are also grants and their
  names are in the argv either way.

A renderer that ignores one of its four inputs is otherwise
indistinguishable from a renderer that forgot it, which is why each of the
three is said in the code as well as here.

## 6. For D18: the render has to join `WallRenderSources`

`internal/posse/bwrap.go` is deliberately NOT in `WallRenderSources` yet. The
list is "paths whose commits can change a wall THIS BINARY RENDERS", and
today nothing launches behind this one, so counting it would inflate the one
number whose value is that it is specific (`hookfresh.go`'s own exclusion is
the same argument). The day `RenderBwrap` writes `bwrap.args` with a
`do not edit` header, internal/treepins' derived half requires the file to be
named there, so the pin will force it. Said here so it reads as a deferral
and not as an oversight.
