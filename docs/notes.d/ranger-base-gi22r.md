# ranger-base-gi22r — shape 1's grants and pins walk their components instead of stat'ing them

ADR 0100 D2(b)'s remedy for a loose directory is to bind it IN PLACE, and the
amendment that closed the carve-out's half of it added the edge that remedy
has: a link at a pin's own name refuses the render, because an in-place bind
of a link mounts the link's TARGET at that name. `bwrapCarvePins` (BwrapArgs
step 5d) was given that `os.Lstat` under `ranger-base-a59w7` and said in as
many words that shape 1's own arm still stat'd. This is that arm, and the
grant arm beside it, which nobody had asked the question of at all.

## What was open

`BwrapArgs` step 2 — the loop over `bwrapGrantMounts` — decided both of its
arms with `bwrapStat`, which is `os.Stat` and FOLLOWS, and then emitted
`--bind p p`. bwrap resolves `p` at `mount(2)` and follows a link at a bind's
source and at its destination alike, so a link at that name binds the link's
target read-write at `p`'s own name: a directory of the planter's choosing,
inside a write wall that granted it nowhere.

Nothing downstream said so, and the reason is worth keeping:

- `bwrapRefuseUnpinnedSource` (step 6) asks whether every ANCESTOR of a bind
  source is a mount destination of the render. `p` is always its own
  destination, so a link AT `p` satisfies it — and the pin that step 2 emits
  is what makes the component a mount destination in the first place, so the
  render that carries the plant looks MORE pinned than one that does not.
- `bwrapRefuseWallRootGrant` (D2(a)) does resolve a grant, but only against
  the wall root. A target anywhere else passes.

## Measured

MEASURED 2026-11-21, linux 7.2.15, at d24988c3, with a scratch test outside
the tree and `HOME` set off `/tmp` so these rules are live (the package's own
`TestMain` puts `HOME` under `os.TempDir()`, where `/tmp` is granted
unconditionally and the whole class stands down — `bwrapWallRoot`). With
`outer = <proj>/g` and `<proj>/g/a` a symlink to a directory inside no grant:

| render | before |
|---|---|
| `BwrapArgs(<proj>, [g, g/a/b])` | RENDERS, with `--bind g/a g/a` — the pin arm |
| `BwrapArgs(<proj>, [g, g/a])` | RENDERS — the grant arm, which predates the pin |
| either, with the link DANGLING | RENDERS, and asks the launcher for `mkdir -p <proj>/g/a` |

The bwrap half — where the mount actually lands — was NOT re-measured: a
nested bwrap is refused from a walled seat (`setting up uid map: Read-only
file system`). It is ASSUMED from `ranger-base-p47q6`, which measured the same
`os.Stat`-plus-in-place-bind shape against a real bwrap for `/tmp`.

WHO CAN PLANT ONE: any namespace that can write `p`'s PARENT and holds no
mount on `p`. A render that grants only the directory above `p` leaves `p`
loose and writable — that is the first control of
`TestQAWallPinnedSourceBindsATwoDeepNestsLooseComponentInPlace` — so the same
seat relaunched with a nested grant gets the plant's target. A sibling
namespace can plant it too, because a mount refuses a rename only in the
namespace that holds it (ADR 0100's P4/P4b).

## The fix

`bwrapGrantSrcStat` (bwrap.go) is the one question step 2 asks of a grant or a
pin now, and it is two questions with one thing deciding between them:
whether the path lies strictly inside ANOTHER grant of the same render.

- Inside one: walk `p`'s components with `os.Lstat` from the OUTERMOST grant
  that contains it (`bwrapOutermostContainingGrant`, then `bwrapSrcWalk` —
  the same walk the private binds and the private `/tmp` already use, which
  is why this is a call and not a second reading of "is the source there"),
  and refuse a symlink or a non-directory at any component, `p` included.
- Inside none: `bwrapStat`, as before. That is a SCOPE and not an exemption.
  Such a path's own parent is outside every grant, so no namespace this wall
  describes can plant at its name, and what is left is the race D1 already
  prices. Every PIN meets the inside-a-grant condition by construction, so in
  practice the walk arm is the pins plus the nested grants.

Outermost and not deepest, because a walk from the deepest containing grant
would trust whatever lies between that grant and the one above it — which is
exactly the directory this class is about.

### The prep step is the row with nothing to do with the mount

`os.Stat` reads a dangling link as "not there", which is step 2's `!exists`
arm, so the unfixed render asked the launcher for `mkdir -p` at that name.
That prep runs in the UNCAGED launcher (`PrepStep`'s head comment: every step
there is a write on the host made in the seat's name), and `MkdirAll` follows
a link, so the render aimed its own 0755 mkdir at the plant's target. A
`PrepMkdirAll` is emitted on the WALK's absent verdict now and never on
`os.Stat`'s — the same rule step 2a-bis already carried for the private `/tmp`
source, said once more for the grants.

### One message got its shape back

`bwrapSrcWalk`'s "could not ask" refusal named `private source` for whatever
walked it. It takes the shape as an argument now, so a grant's `EACCES` reads
`write grant` and a pin's reads `write grant pin` — `bwrapStatRefusal`'s own
rule, which exists because a path with no shape beside it is a refusal an
operator cannot check against the design.

## Verified

- `TestQAWallPinnedSourceRefuseALinkAtAGrantOrItsPinsOwnName`, both arms
  (the pin arm and the grant arm), three rows each: the control over a real
  directory renders and binds it in place; the plant to a directory outside
  every grant is refused by name, in words, with remove-and-relaunch; the
  DANGLING plant is refused and no prep step names it.
- `TestQAWallPinnedOutermostContainingGrantIsTheWalksRoot` (seven rows) and
  `…ReadsAGrantsResolvedSpelling`, which pin WHICH grant the walk is rooted
  at. As rows because no render arm can see that choice: by the time two
  containing grants differ, the deeper one's own mount has already been
  walked from the shallower, so a wrong root shows up as a refusal that is
  MISSING and never as one that is wrong.
- MUTATION, MEASURED 2026-11-21 (linux 7.2.15, 16 cores, this worktree),
  three mutants, each red in its own row and nowhere else:
  - the walk swapped back to `bwrapStat`: all six rows of the refusal pin
    red, and NOTHING ELSE in a `-run 'Wall|Bwrap'` run over
    `internal/posse` — 145.8s, one FAIL. That run is also the census that
    says the walk refuses no shape this box renders.
  - shallowest swapped for deepest: the two-grant rows red, naming the root
    the walk would have trusted.
  - the resolved spelling dropped from the candidate roots: the symlinked-
    grant row reds with `ok false` — a grant declared through a symlinked
    home would have rooted the walk nowhere, which is a walk that trusts
    every component of it.
- `make test` and `make tree-check` green, with the numbers in the commit
  message.

## Not closed here

Nothing. ADR 0100's residual list is one shorter for this.
