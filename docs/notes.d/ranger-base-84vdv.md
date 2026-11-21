# A wall pin that trusted an exit code at a path the wall had already moved (ranger-base-84vdv)

2026-11-21, Linux 7.2.15-1-omarchy x86_64, bubblewrap 0.13.1, go1.27.3, git
2.56.0, 16 cores, from this bead's worktree at `399d321f`. The wall-check's
red at tree `66e7fbf9a255`, one FAILed member:
`TestLiveBwrapSeatStoreRefusesRewritingASharedObject`, first failure line
`bwrapseatstorelive_test.go:248: with <common> granted whole: a committing
seat could rename over …/objects/c7/94d1a5b057035e9b5774f196919e6a58961d96`.

**It was not a wall regression, and nothing in the landing window touched it.**
The window held one landing, ranger-base-nqw8v, which added a test file and two
notes files and no production line. The pin reds about one run in 256 on any
tree, and the shared object it is about was never touched in any of them.

## What the arm actually measured

The pin types four commands at the shared object's file — unlink, chmod,
rename over, rewrite in place — at two paths, under two renders, and reads the
exit code as the verdict:

    for _, p := range []string{file, side} { … if ok, out := f.in(…); ok { t.Errorf("could %s %s") } }

`side` is the shared store bound read-only at `<common>/worktrees/<seat>/seat-objects-base`,
so there the path names the shared object inside the seat and the exit code is
a verdict. `file` is `<common>/objects/<xx>/<rest>`, and the whole of ADR 0099
D1 is that `<common>/objects` is covered by the seat's OWN store — so inside
the seat that path is a name in the seat's store, the shared object is not
there, and whether a write succeeds says nothing about the wall. It says one
thing only: whether the seat's store already holds a directory named for the
sha's first two characters.

`printf evil > <file>.new && mv -f <file>.new <file>` needs that directory and
nothing else. With it, the rename lands in the seat's store and exits 0; the
shared object's bytes, its mode and what the operator's `cat-file` serves are
all unchanged, which is the design working exactly as written.

## Where the directory comes from, and why it is one run in 256

The pin's own shape-1 tail writes two objects into the seat's store: a
`hash-object` of a fixed string (`945d0a…`, so always `94`) and a
`git commit --allow-empty`, whose sha is derived from the commit timestamp and
is therefore uniform. MEASURED, five consecutive runs of this fixture, the
store's two-character directories after shape 1:

    [55 94]   [94 c0]   [02 94]   [94 c6]   [94 c2]

The honest object is `c794d1a5…`. When the commit's sha happens to start `c7`
— about one run in 256 — `<store>/c7` exists for the rest of the run and the
real-name arms start exiting 0. That is the red at `66e7fbf9a255`, and it is
why shape 1 was clean there and shape 2 was not: the directory did not exist
until shape 1's commit made it.

With `<store>/c7` planted deliberately, the pre-fix pin reds seven rows in
two shapes, three classes of them:

- `could rename over` / `could rewrite in place` — the exit code read as a
  verdict at a path the store bind covers.
- `could unlink` in shape 2 — the file the shape-1 rename left in the seat's
  store.
- `the seat cannot read history through its store's alternates` with git's
  `inflate: data stream error` — the `evil` the pin itself planted in the seat
  store under a SHARED object's sha, which shadows the alternates for that
  name. The pin was contaminating its own fixture.

The venue's own run red three of those seven and not all of them, which is the
same fact from the other side: there `c7` did not exist until shape 1's commit
made it, so shape 1's rows were clean and the member red shape 2's rename-over,
rewrite-in-place and read-through. The wall-check prints one first-failure line
per member (`evidence()`), so the bead carried the first of those three and
none of the context.

## What landed

`bwrapseatstorelive_test.go`, the pin only. No production line changed.

- The `<xx>` directory is PLANTED in the fixture, so the real-name arms take
  their success path on every run instead of one in 256. The condition is made
  rather than waited for.
- The verdict moves to where this file's own header always said it was —
  "EVERY REFUSAL IS READ BACK FROM OUTSIDE". `bsShared` is the shared object as
  the operator sees it (a digest of its file's bytes, its size, its mode, and
  what `cat-file` serves), read before the arms and again after every one of
  them, so a row that moves it reds naming its own command instead of the test
  reporting one anonymous line at the end.
- The exit code stays a verdict at the SIDE path, where it names the shared
  object, and is explicitly not one at the real name.
- `clearSeatCopy` puts the seat's store back after each real-name row, so the
  read-through arm measures the alternates and not the pin's own plant.
- A positive row per shape says what the real-name arms' exit code used to be
  read as: at that name the write is REDIRECTED into the seat's own store, not
  refused — and it is also what makes the plant load-bearing rather than
  decoration.

## The mutation run, and the one arm of it that was green

Five arms, each planted alone with the tree asserted clean between them:

| mutant | reading |
|---|---|
| the `<xx>` plant removed | positive row reds in both shapes |
| `refused: true` back on the real name | reds in both shapes with the venue's own words — the red at `66e7fbf9a255`, deterministic |
| `clearSeatCopy` a no-op | read-through row reds in both shapes on the pin's own plant |
| `seatStoreBinds`' store bind replaced by an in-place READ-WRITE bind of `<common>/objects` | the per-shape control reds and every read-back reds, each naming its command |
| the same mutant, with the control written as a `cat` of the file compared to the honest TEXT | **GREEN** |

The last row is the finding worth keeping. The first draft of the per-shape
control asked whether the seat could read the honest bytes at the real name —
and a loose object's file is a zlib stream, so `cat` there serves compressed
bytes and the comparison could never be true. It was a line that reads like a
control, in the right place, over the right question, and it could not red
under the ONE render it exists to catch. It digests the file now, and the
digest the seat computes is compared with the one read from outside; a second
control in the pre-store render says the seat can compute it at all, so a seat
image without `sha256sum` cannot quietly turn that row back into decoration.

## Verification

- `go test -run TestLiveBwrapSeatStore ./internal/posse`: green, and green at
  `-count=5` over the family.
- `go test -run '^Test(QA)?Live(Bwrap|Wall)' ./internal/posse`: green, 98.0s.
- `bash docs/adr/0097-wall-venue.probe.sh venue+check` at this tree: rc=0,
  123s at loadavg 19.28, `wall check: green, 47 pins measured` — still 47,
  which is this bead correctly adding no member.
