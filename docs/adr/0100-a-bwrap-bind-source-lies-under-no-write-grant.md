# ADR 0100 — A bwrap bind source lies under no write grant, and a private destination is a mount wherever it can be written

*Status: accepted 2026-11-15 (architect sign-off ranger-base-6w88m) · owner:
architect · extends ADR 0080 D17's render and ADR 0099 D1's seat store; their
other lines stand · fix: ranger-base-5f5kb (D1's claude state and private
/tmp, D2(a), D3), ranger-base-ltmyh (D1's seat store and its two-arm import,
D2(b), which amended D2(b)'s own last paragraph on a measurement),
ranger-base-gn050 (D4, which amended D4(a) with its census and D4(b) with a
stand-down), ranger-base-dw2du (D2(a)'s second direction, D2(b) over the
in-place binds, and D4(b)/(c)'s wording, from the re-read of D2 and D4)*

## Context

bwrap resolves every mount SOURCE at mount time and follows links (MEASURED
P1). The render's lstat walk (ranger-base-wsgus) judges the source a moment
earlier. Any process that can write a COMPONENT of the source in between can
redirect the bind. That includes a leftover of the seat itself, which needs
no timing: it can toggle the component in a loop. The census of the class,
keyed on the invariant "a source that differs from its destination lies under
a write grant", finds four members:

- claude's private session-state dirs and its private history.jsonl, under
  `<SeatCacheDir>/claude`;
- the seat /tmp, `<SeatCacheDir>/tmp`. This one is NOT a race: the render
  checked it with os.Stat, which follows links, so a planted link is mounted
  rw at /tmp (MEASURED, ranger-base-p47q6);
- the seat store, `<own git dir>/seat-objects`, bound rw at
  `<common>/objects` (ADR 0099 D1). A launch that loses the race gets the
  SHARED object store rw, which is the write ADR 0099 exists to remove; the
  rate is ASSUMED, by the same toggle argument.

The DESTINATION side is a different attack, and it is not a race. MEASURED
2026-11-15, this box (Linux 7.2.15, bubblewrap 0.13.1), by a scratch probe
quoted on ranger-base-6w88m:

- A victim namespace binds a private dir over `cfg/m`.
- A namespace that holds no mount on `cfg/m` renames it away mid-session and
  creates a new `cfg/m`. Both steps succeed.
- The victim's next open of `cfg/m/who` reads the PLANTED file. Its mount is
  still alive, but on the renamed dentry.
- A namespace that holds the same bind gets EBUSY for the same mv.

So any walled seat that can write a private destination's parent, and does
not hold that destination as a mount, can swap it at any time in the victim's
session. For claude's shell-snapshots and session-env, a win means the victim
runs sibling-written code ahead of every Bash command: a walled-to-walled
lateral move. A check at launch (bwrap's final-link refusal, or a post-mount
verify) cannot close this, because the swap can come after the check.

## Decision

**D1. NO SOURCE THAT DIFFERS FROM ITS DESTINATION LIES WHERE A WALLED SEAT
CAN WRITE A COMPONENT OF IT.** All four members move to a per-seat WALL dir,
`SeatWallDir = <resolved home>/.cache/posse-wall/<the SeatCacheDir key>/`,
under a wall ROOT that no render grants (see D2). The new paths are
`<wall>/claude/…`, `<wall>/tmp` and `<wall>/seat-objects`, and SrcRoot is
`<wall>`. The seat still reads and writes the contents through the mount at
Dst. It cannot create, rename or unlink any component of the source, because
every component is under the read-only `/`. The walk stays: Tmp goes through
the component walk (`bwrapSrcWalk`) and never through `bwrapStat` again. The
same move applies to the L4 cage's seat-store mount (seatStoreCageMounts).
The launcher writes `<wall>/origin` (the resolved tree path and the common
dir) when it creates the dir. That file is trustworthy because no seat writes
under the wall root.

NO MIGRATION, with one exception. The old `<SeatCacheDir>/claude` and
`<SeatCacheDir>/tmp` are left as inert junk. Moving them would be a rename
through seat-writable components by the uncaged launcher, which is this hole
once more. Each seat's private history, debug logs and /tmp start empty once
(ASSUMED acceptable).

THE SEAT STORE IS THE EXCEPTION, because it holds committed work:

- The first launch that renders the wall store IMPORTS the legacy
  `<own>/seat-objects` first, through openSeatStore (ADR 0099 D3's os.Root
  path, which refuses a link that leaves the parent). Before that import, the
  seat's own branch tip may be readable only there.
- Every importer has TWO arms. The wall arm finds the store from admin dir →
  tree → SeatWallDir; for orphans, it sweeps the wall root for origin files
  that name this common dir and whose tree is gone. The legacy arm is today's
  seatStoreAt path, unchanged.
- The legacy arm stays until a census reads no admin dir holding a
  seat-objects. Its removal is a later bead, not this one
  (ranger-base-tbg3h).
- Retire, reap and prune run in this order: import both arms, then `git
  worktree remove`/prune, then RemoveAll the wall dir. An import that fails
  keeps the wall dir and refuses the removal, as pruneWorktrees already does.

**D2. TWO INVARIANTS, checked over EVERY bwrap argv the package renders:**
bwrapRender, the landed-check judge and the wall venue.

(a) Box-wide: refuse any write grant that OVERLAPS the wall ROOT — at or
above it, or inside it. The grant side is compared RESOLVED (absResolve),
because that is what the bind will expose. A seat could plant a link at a
grant path of its own for its next launch, and only the resolved grant shows
where it points. The wall root and every source are compared as COMPOSED,
from the one resolved home. Compare with insideDir, never underDir: underDir
resolves both sides, so it would follow a planted source link and judge its
target.

AMENDED 2026-11-21 (ranger-base-dw2du, from the re-read of this decision).
BOTH DIRECTIONS, and the second is new: "at or above" was the whole rule
while the attack in view was a grant that holds a COMPONENT of a source. A
grant INSIDE the root is neither at nor above it and is the same hole
arrived at from below — a grant on another seat's wall dir, or on a
directory within one, is a write grant on that seat's private bind SOURCE,
and for a member a seat reads before every command it runs that is code
execution in the victim with no rename and no race in it. Refuse when
`insideDir(wallRoot, G)` OR `insideDir(G, wallRoot)`, G resolved and the
root composed, which is the same two sides the first direction already
compares.

IT REDS NOTHING LEGITIMATE, and the negative case is why: a seat's own
private DESTINATIONS are grants, but its own private SOURCES are granted by
no render — they are bound, which is the whole of D1 — so no render has a
grant under this root to lose. MEASURED 2026-11-21 over all five argvs this
package renders: 0 of 32 grants lie inside the root, while every render has
sources that do. Both halves are pinned together, because either alone is
vacuous.

(b) Per render: a path P is PINNED in render R when every component of P that
lies strictly inside a write grant of R is itself a mount destination of R.
EVERY BIND SOURCE must be pinned, the in-place binds of R's own write grants
included. The old Tmp, claude and seat-store sources all fail this, which is
the pin. The seat store's side bind passes legitimately: its source
`<common>/objects` is inside a `<common>` grant but is the store's own mount.

AMENDED 2026-11-21 (ranger-base-dw2du, the same re-read). The first text of
this rule said "every source that DIFFERS from its destination", which left
the IN-PLACE binds — a write grant's own `--bind p p`, and every in-place
`--ro-bind` — judged by neither arm of this decision, since D4(a) skips the
same equality. They are not safe for being in place: bwrap resolves the same
string again at mount(2), so a component of a grant path that a walled seat
can rename is togglable between a directory and a link to anywhere, and the
render's own absResolve an instant earlier is the check-then-use this record
rejects A2 for. A grant whose loose component is pointed at the wall root is
a read-write mount of every seat's private sources at the grant's own name,
at the NEXT launch and with no race to win; the attacker is the seat's own
leftover, so it needs no sibling. The leftover holds the old render's mounts,
so every pinned component answers EBUSY.

NO MODE DISTINCTION, which is one notch wider than the re-read asked for
("every rw bind"): an in-place `--ro-bind` redirected the same way hands out
the same tree read-only, which for private history, shell snapshots and the
seat store is a confidentiality break, and a rule with a mode exemption is a
rule with an exemption to maintain. MEASURED 2026-11-21 over all five argvs:
150 in-place binds, every one pinned, so the wider reading costs nothing
today. That measurement also answers the one thing the re-read left ASSUMED
— whether any render has a grant nested two or more components inside
another with a non-mount component between them — with no. What this rule
asks of an in-place bind is only about its ANCESTORS: the path itself is the
destination of the mount that binds it, and a grant one component inside
another has that outer grant for its parent, so the shape that reds is a nest
two deep with nothing bound between; the pin is a render carrying one. A miss is fixed the way D4(a) prescribes: bind the loose
component in place, or narrow the grant above it, never exempt it.

AMENDED 2026-11-21 (ranger-base-dot6w): THE NEST EXISTS, AND THE RENDER PINS
IT RATHER THAN BEING REFUSED. The "with no" above is a reading of the five
argvs that census builds and not a property of the shape, and a sixth shape
was already in this tree: a store of record inside the same repository grants
`<common>` whole, which puts the seat's own `<common>/worktrees/<seat>` two
components inside that grant with `worktrees` loose between them, while
`<common>/objects` and `<common>/logs` are one component inside it and so
pinned by their own mounts. MEASURED 2026-11-21 in ADR 0097 D1's venue (Linux
7.2.15, bwrap 0.13.1): the live seat-store pin's second arm renders that shape
and BwrapArgs refused the whole render — 1 of 46 wall-pin members red, a
committing seat in a linked worktree that does not launch at all on a box in
that configuration. It fails CLOSED, so it is this rule costing a launch and
never a hole in it.

THE FIX IS THIS DECISION'S OWN REMEDY, GENERALIZED, and it lands where shape 1
already was: for every write grant, bind IN PLACE every directory strictly
between it and the grant that contains it, shallowest first so the deeper
mount lands last (`bwrapGrantMounts`, BwrapArgs step 2). It widens nothing —
every such directory lies strictly inside a grant that has already made it
read-write — and it is emitted by the render that grants it, which is step
2a-bis's rule for the private /tmp source and `runtimeEphemeralDirParents`'
for the CLI homes, said once more for the grants themselves. MEASURED
2026-11-21 after it landed: the same 150 in-place binds over the same five
argvs (claude 55, codex 31, grok 50, judge 7, venue 7), so no shipped render
carries a nest for it to pin and the list is inert until one does; the venue
reads 46 of 46 green.

AMENDED 2026-11-21 (ranger-base-a59w7): AND THE SAME AT THE CARVE-OUT'S
PRODUCERS, which the paragraph above generalized the GRANT producer only for.
A DENY is an in-place `--ro-bind` and a read MASK is a `/dev/null` over a
name, and both have ancestors: where one lies two or more components inside a
write grant with no mount between — `<repo>` granted with the deny
`<repo>/.git/config.lock`, so `<repo>/.git` is a mount of nothing; a grant
above a CLI home with that home's credential store masked, so the store
directory is — the grant set names that directory nowhere and
`bwrapGrantMounts` cannot reach it. MEASURED 2026-11-21: five of
internal/posse's own pins refused a render for exactly that, under a home
outside the shared `/tmp`, and none of them is a live wall pin, which is why
ADR 0097 D1's venue read green over all five. Same remedy once more, at the
step where those mounts are made: `bwrapCarvePins` (BwrapArgs step 5d) binds
in place every directory between a write grant and a deny, keep or mask the
render mounts below it, and the list is SPLICED IN at step 2d's mark — after
the grants, before every other mount — because a pin emitted after the
`/dev/null` or the read-only bind it exists for would cover that name with
the host's own directory again, and a pin above one of 2c's private members
would bury a wall. It asks only about the mounts from the deny producer down:
step 2a-bis and `runtimeEphemeralDirParents` each pin their own loose
component and are pinned for doing so, and a list that re-emitted theirs
would answer those pins green with their producers gone. CENSUS, MEASURED
2026-11-21 after it landed: 0 carve-out pins over all five argvs (claude,
codex, grok, judge, venue), so the list is inert until a shipped render has
such a nest — the shapes that have one are a main-checkout seat with a
`.git/config*` deny, which ADR 0095 D1 refuses one line earlier for a reason
of its own, and a grant above a CLI home.

AND A LINK AT A PIN'S OWN NAME REFUSES THE RENDER, which is this remedy's own
edge and is decided here rather than left to the producer. A pin's path is by
construction a directory the seat may write — that is the whole reason it
needs pinning — so a link at that name is one a seat or its leftover planted,
and bwrap follows a link at a bind's source and at its destination alike.
Binding it in place resolves BOTH ends to the target and mounts that target
read-write at its own name: the attack (b) refuses, let through by the mount
meant to refuse it, with the component now a mount destination so that this
rule says nothing. Following the link and judging its target is not the other
answer — the seat repoints it between the render and `mount(2)`, which is the
check-then-use this record rejects A2 for. So the pin loop `os.Lstat`s and
refuses, in the tier's fail-closed direction. The same question at the grant
pins of D2(b)'s first remedy is open, and this record names it where it names
every open residual — by bead id alone, in "Not closed here" below, with the
detail on the bead where the audience is already the right one.

The landed-check and venue sources lie under `StateDir/landed-check/run-*`,
which no walled seat writes. They pass D2(a) and the pin says so.

AMENDED 2026-11-15 (ranger-base-ltmyh, the slice that built the predicate):
they did NOT pass D2(b), and this ADR's first text said they did. MEASURED
by running the predicate over the two rendered argvs: the judge's private
/tmp is `tmp` INSIDE the run dir, the run dir is the one thing either render
grants, so that component lies strictly inside a write grant and is a mount
of neither render — loose, by (b)'s own definition above. The venue's
private /var/tmp is the same shape (`var-tmp` in the same run dir). "No
walled seat writes the run dir" is an argument about WHO can write it, which
is true and is not what the predicate asks; a rule that holds for everything
but two renders is a rule with an exemption to maintain, and this ADR
refuses exemptions for the destination arm in D4(a)'s own words.

THE FIX IS D4(a)'s, applied to a source: an IN-PLACE BIND of the loose
component, emitted by the render that already grants it. It costs one mount
in each of those two renders and grants nothing — in place is read-write
over a path the run-dir grant has already made read-write — and it makes the
component a mount destination, which is what refuses the rename (P4/P4b). It
is CONDITIONAL on the grant: where no grant covers the source, as in every
seat render, the same line would hand out a write nobody asked for. A seat's
own sources need none of it, because they lie under the wall dir and inside
no grant at all.

**D3. PREP STAYS IN THE LAUNCHER, unchanged in shape.** Its source-side paths
now have no component a walled seat can write. Its destination-side paths are
covered by D4.

**D4. A PRIVATE DESTINATION IS A MOUNT IN EVERY WALLED NAMESPACE THAT CAN
WRITE ITS PARENT.**

(a) Per render: every destination that differs from its source is PINNED in R
(the D2(b) predicate, one function). This covers the seat's own namespace and
its leftovers. `<cfg>/<member>` passes, because the member is the mount.
`<cfg>/projects/<key>` passes, because projects is an in-place ro bind.
`<common>/objects` and `<own>/seat-objects-base` pass: their components
inside a grant are mounts, and outside a grant they sit under the ro `/`.
That answers the review's "the store's own side bind" case without an
exemption. A miss is fixed by an in-place bind of the parent, never by an
exemption: grok's `daemon/run` is ASSUMED to miss, and codex's tmp and
shell_snapshots need a census.

AMENDED 2026-11-20 (ranger-base-gn050, the slice that built the destination
arm). THE CENSUS IS RUN, and it is a pin rather than a paragraph:
`TestQAWallPinnedDstEveryRenderPinsEveryDestination` runs this predicate over
every destination of all FIVE argvs this package renders — the seat render for
each runtime in the catalog, the landed-check judge and the wall venue — so a
bead that adds a private destination is censused by writing nothing.

MEASURED 2026-11-20 on this box (Linux 7.2.15, the shipped producers): ONE
miss. Its bead is named here by id and nothing else — no path, no mechanism
— because the surface it lies on is an open residual and that is how a
public record names an open one (operator ruling 2026-11-11):

- ranger-base-qyq59

Its SHAPE needs no more than this rule to state, which is why the census
found it: a private destination whose parent lay strictly inside a write
grant and was a mount of nothing. Fixed as this decision prescribes — an
in-place read-write bind of the loose parent, emitted by the render that
already grants it (`runtimeEphemeralDirParents`), ordered BEFORE the tmpfs
so it cannot cover it, and conditional on the grant so it widens nothing.

codex's `tmp/` and `shell_snapshots/`, which this ADR left to the census,
PASS: each is a direct child of the granted codex home and is its own tmpfs,
so each is the mount that pins it. Everything else passes as written —
`<cfg>/<member>`, `<cfg>/history.jsonl`, `<cfg>/projects/<key>` (in place, so
not in scope), `/tmp`, `<common>/objects`, `<own>/seat-objects-base`, and
every `/dev/null` read mask and credential-store tmpfs, whose masked name is
itself the mount.

ONE WORDING FIX from the re-read (ranger-base-hfocr), adopted: (b) says each
member is PINNED and not merely bound, because a grant of an ANCESTOR of
`<cfg>` renames `<cfg>` itself. That is what the predicate asks, so (a) and
(b) are one question over one function, and the census reads it that way.

(b) Box-wide: THE PRIVATE SET FOLLOWS THE GRANT, NOT THE RUNTIME. Any bwrap
render whose final write set reaches a runtime config dir that has a private
table makes every member of that table PINNED in that render, whichever
runtime the seat runs. Today the only such table is claude's:
claudeSessionStateDirNames plus history.jsonl.
claudePrivateSessionState's gate moves from the runtime's state grants to the
render's final write set. A sibling that can write `<cfg>` therefore holds
every member as a mount, and gets EBUSY.

PINNED AND NOT MERELY BOUND (ranger-base-dw2du, the re-read's wording fix,
adopted). The two readings part when the grant is an ANCESTOR of `<cfg>`
rather than `<cfg>` itself: a mount on each member does not stop a rename of
`<cfg>`. For the default home D2(a) already refuses the two ancestors that
matter, `~` and `~/.cache`; a `CLAUDE_CONFIG_DIR` pointed somewhere else has
no such cover, and there the predicate is what says so. It is D2(b)'s
predicate and the same function, so (a) and (b) are one question asked of two
lists.

AMENDED 2026-11-20 (ranger-base-gn050): the gate STANDS DOWN where this
mechanism's wall does, off the same helper and for the same reason D2(b)'s
refusal does (`bwrapWallRoot`). On a box whose home is itself under the
shared `/tmp` there is no ungranted place for a bind source to live, so
nothing is pinned by anything and both refusals above are disabled — and
there EVERY render's write set reaches `<cfg>` through the blanket `/tmp`
grant, so a grant-keyed table would hand every seat a private table whose
sources that seat can swap itself: a mount with no EBUSY behind it. Where the
wall is not real the gate reads what it read before this decision, which
changes no behaviour there at all. No production launch is that shape and
every fixture in internal/posse is, which is why the pins for the
grant-keyed half put their home outside every blanket temp grant.

The SEATBELT readings of the same members (claudeDebugDir,
claudeSessionsRegistry, claudePromptHistory, and claudeCacheAndCleanupMarker
since ranger-base-mbcbv) keep the runtime gate, and that is not an oversight
to tidy: this decision's whole subject is the mount that gives EBUSY, SCOPE scopes the guarantee to walled seats, and a profile has
neither. claudeProjectsBinds keeps it too — its binds are in place, so it has
no destination of its own to swap.

(c) For the seat-store destination `<common>/objects`, the writers of
`<common>` are of two kinds. Linked seats of the repo hold it as their own
mount, (a). The other kind already writes the shared store by grant: an
unlinked seat in the main checkout, or a shims seat. A swap gives them
nothing they lack (ADR 0099's own scope).

READ 2026-11-21 (ranger-base-dw2du), and the list stays at two. The re-read
of this decision named a THIRD kind — a seat linked in another repository
whose store of record is this one, which was granted this repository's git
dirs whole and so held no mount of its own over `<common>/objects`. That was
true when the re-read was written and it is not true now: the grant itself
was removed (ranger-base-kfrpv), and ADR 0099 carries a dated note saying
D1's session-repo scope is complete and is not to be reopened for the store
repo (ranger-base-gdgbb). So the class has no members, and adding it here
would have reintroduced as a premise of this record exactly the sentence
that landing struck from five others. If a seat is ever granted write into
another repository's git again, this is the paragraph that gains its third
kind.

(d) bwrap's final-link refusal (MEASURED P3) is check-then-use: 0.13.1 has no
move_mount, and path mount(2) follows a final link. It is kept as the refusal
a hand-made link gets. Nothing relies on it.

**SCOPE.** The guarantee is for WALLED seats (bwrap, L3/L4). A seat at `cage:
shims`, and a seat degraded from seatbelt to shims on linux, is a same-uid
crew process with no write wall. It reaches every wall dir, every
SeatCacheDir, every `<cfg>/<member>` and every gates dir. It is NOT the
operator. MEASURED 2026-11-15 from this instance's session records: 5 live
seats at shims, 3 degraded seatbelt→shims, 2 walled. That exposure predates
this record, and this record neither closes nor widens it.

## Consequences

- No launch-line change, no new entry step, no ADR 0080 D18 amendment.
- RESIDUAL, stale walls. A wall is fixed at launch. A walled seat launched
  before this landing (or before a member joined the private table) does not
  hold the new mounts. Until it relaunches it can still swap a
  `<cfg>/<member>` in a newer sibling, and its own sources still sit under
  its SeatCacheDir. The residual closes when every walled seat whose launch
  predates the landing has relaunched. Those seats can be listed from the
  launch times in the instance's session records against the landing commit.

  LISTED 2026-11-20 for D4's landing (ranger-base-gn050), from this
  instance's session records: ONE walled seat live — `cage: seatbelt`, which
  on Linux is this mechanism — against six at `cage: shims`, four of those
  degraded from seatbelt. Its launch predates the landing, so by the rule
  above it is stale; its wall is nonetheless BYTE-IDENTICAL to what this
  landing renders, because it is a claude seat, and the two mounts D4 adds
  are another runtime's in-place parent bind (the remedy the census miss
  above took) and claude's table for a NON-claude render whose grant reaches
  `<cfg>`. Read off its live `bwrap.args`: the private table is there, and
  the other runtimes' homes are masks rather than grants. No live PID on
  this instance declares a `writable:`
  extra at all, so D4(b)'s widening changes no render here today — it is the
  wall standing ready for the first PID that declares one. So the stale-wall
  residual for THIS landing is empty, which is a reading of this instance's
  roster and not a property of the decision: a seat of another runtime, or
  an extra-carrying seat, launched before the next landing is stale in the
  way the bullet describes.
- RESIDUAL, a home under the shared `/tmp`. D2(a) stands down there
  (`bwrapWallRoot`), for the reason the private /tmp already stands down
  (`bwrapSeatTmp`): `/tmp` is granted unconditionally, a home inside it is in
  the shared directory already, and there is no seat on such a box for the
  wall root to protect. No production launch is that shape — homes and
  worktrees live under the home — and every fixture of internal/posse is,
  which is why the pins for D2(a) place their home outside every blanket temp
  grant and would otherwise measure the stand-down.
- Every reader that composes the private paths recomposes from SeatWallDir:
  claudePrivateHistoryPath, claudePrivateSessionRowPath, the herdr backend's
  seat home, SeatTmpDir/bwrapSeatTmp, the seat store layout and the
  importers. As a side effect, the uncaged readers stop reading through
  seat-writable components. So does the launcher's own write of the store's
  info/alternates.
- Exit hatch: the wall dir holds only the seat's own state. Deleting it costs
  the D1 "start empty" bullet, plus a store that must be imported first. The
  importers do that.

## Alternatives rejected

**A1. posse holds the objects:** openat2(RESOLVE_NO_SYMLINKS|RESOLVE_BENEATH)
plus --bind-fd from an uncaged entry step. It WORKS (MEASURED P2/Q2/Q3).
Rejected: it races the writer instead of removing it, and it costs an ADR
0080 D18 amendment, an fd protocol and an *at copy of prep. It also does
nothing for the destination, which is not a launch race (Context). Revisit if
a source must live where the seat can write it by name.

**A2. Re-check just before the line is typed.** Rejected: the toggle loop
makes the width of the window irrelevant.

**A3. An in-cage post-mount verify of each destination's dev/ino.** Rejected
for BOTH arms, and for the destination arm now on a measurement and not on
cost. The destination swap is a rename that a sibling can make at any moment
of the victim's session (Context, MEASURED), so a launch-time check verifies
a state that can change a second later. It would also have to become
unconditional (wall-unix is typed only at Landlock ABI >= 6).

**A4. A read-only in-place bind of `<SeatCacheDir>/claude`** (the clever one).
Rejected: it is a mount whose only job is to guard a path that never needed a
grant.

**A5. --unshare-pid to end leftovers.** Rejected by ADR 0096, and it would
not help: the Tmp plant needs no leftover, and neither does the destination
swap.

**A6. Do not grant `<cfg>` whole:** grant only its shared members in place, so
no seat can rename a child of `<cfg>`. Rejected for now. claude writes new
top-level files and is ASSUMED to rename-replace files such as its
credentials in `<cfg>` itself. That would break every claude seat. D4(b) gets
the same EBUSY without that bet. Revisit if a census of claude's `<cfg>`
top-level writes shows none.

**A7. The review's box-wide wording** ("a render that grants a dir rw binds
every private Dst inside it that ANY current render binds"). Adopted in
substance as D4(b), but keyed on a static per-config-dir table, not on "any
current render". A render cannot read its siblings' renders, and a stale one
would make the rule undecidable at render time.

**A8. An in-cage read of the mount's own root instead of pinning the grants**
(`/proc/self/mountinfo` field 4, refusing any mount rooted at or under the
wall root). Offered by the re-read as the FALLBACK if pinning every grant
redded legitimate shapes, and worth recording for the reason it was offered:
after the mount, a bind's root no longer changes, so on the SOURCE arm this
is not check-then-use the way A3 is on the destination arm. Not adopted and
not priced, because the measurement that would have called for it came back
empty (D2(b)'s 2026-11-21 amendment: 150 in-place binds, none loose). It is
where to go if a future PID's grants cannot be pinned without refusing a
shape the shop needs.

## MEASURED vs ASSUMED

MEASURED (2026-11-15, this box, Linux 7.2.15, bubblewrap 0.13.1):

- P1 a source link is followed.
- P2/Q2/Q3 --bind-fd pins a dir, a file and a renamed-away dir.
- P3 a final destination link is refused.
- Q1 an intermediate destination link is followed.
- P4/P4b a mountpoint gives EBUSY in the namespace that holds it, alias
  included.
- P4c a bind source can be renamed in-namespace.
- The Tmp link renders and mounts its target rw.
- (2026-11-15, ranger-base-ltmyh) the landed-check judge's and the wall
  venue's private temp sources were NOT pinned in their own renders, which
  D2(b)'s first text asserted they were. Read off the two rendered argvs by
  the shipped predicate; the amendment under D2(b) has the shape and the
  fix.
- (2026-11-15, ranger-base-ltmyh) with the store under the wall dir, a
  walled seat can neither write nor rename the store's host path, its wall
  dir, or any component of either, and reaches the contents only at
  `<common>/objects`. Read inside a real bwrap from the shipped render
  (TestLiveBwrapSeatStoreHostPathIsUnreachableInTheWall).
- A namespace without the mount renames a live private destination
  mid-session, the victim's name resolves to the planted dir, and a namespace
  with the mount gets EBUSY.
- The seat-store bind shape, the live seat tiers (5 shims, 3 degraded, 2
  walled), and that the gates dir is read-only in-seat.
- (2026-11-20, ranger-base-gn050) THE D4(a) CENSUS over all five rendered
  argvs: one miss, whose bead the amendment under D4(a) names by id and
  nothing else; codex's `tmp/` and `shell_snapshots/` pass; everything else
  passes as D4(a) said it would. That amendment has the readings and the
  remedy.
- (2026-11-20, ranger-base-gn050) a namespace holding `<cfg>/shell-snapshots`
  as a mount from the SHIPPED render cannot rename it (EBUSY), and the same
  render with the private binds taken out — what a non-claude seat granted
  `<cfg>` got before D4(b) — renames it and plants its own directory at that
  name. Against a real bwrap
  (TestLiveWallPinnedDstAHolderCannotRenameAPrivateMember).

- (2026-11-21, ranger-base-dw2du) D2(b) RUN OVER THE IN-PLACE BINDS of all
  five rendered argvs: 150 of them (claude 55, codex 31, grok 50, the judge
  7, the venue 7), read at da79a784, and every one PINNED, so widening the
  rule to them refuses
  no shape this box renders — and no render has the nest the re-read left
  ASSUMED, a grant two or more components inside another with nothing bound
  between. A render carrying one is refused by name, measured by mutation
  (restoring the `Src == Dst` skip reds that pin alone).
- (2026-11-21, ranger-base-dot6w) THAT NEST DOES EXIST, in a shape the census
  above does not build: a store of record inside the same repository, which
  grants `<common>` whole. MEASURED in ADR 0097 D1's venue: the render
  refused, 1 of 46 wall-pin members red. Shape 1 binds the directories
  between a nested grant and the grant above it in place now
  (`bwrapGrantMounts`), and the census reads the SAME 150 in-place binds over
  the same five argvs — the list pins nothing in a shipped render — with the
  venue at 46 of 46. Measured by mutation four ways: dropping the pin arm,
  restoring the `Src == Dst` skip, emitting the pins after the grants instead
  of shallowest-first, and dropping the condition that the pin lie inside a
  grant, each reds a named pin.
- (2026-11-21, ranger-base-a59w7) THE SAME SHAPE OVER A DENY OR A READ MASK,
  which the grant-keyed list cannot reach because the directory above a deny
  holds no grant of its own. MEASURED under a home outside the shared `/tmp`:
  five of internal/posse's own pins refused a render for a loose component
  above a deny or a credential-store mask, and none of the five is a live
  wall pin, so the venue read green over all of them — a filtered run with a
  moved TMPDIR is what says it. `bwrapCarvePins` (BwrapArgs step 5d) binds
  those directories in place, spliced in at step 2d's mark so that no pin
  lands on top of the wall it exists for, and the CENSUS of what it emits
  over the five argvs is 0: no shipped render carries such a nest.
- (2026-11-21, ranger-base-dw2du) D2(a)'s second direction over the same
  five: 0 of 32 grants lie INSIDE the wall root, while every render has bind
  sources that do. Measured by mutation too: dropping the new arm reds the
  three grant spellings written for it — another seat's wall dir, a
  directory within one, and a path of this seat's own that links into the
  root — and nothing else.

ASSUMED:

- the seat-store toggle win rate;
- that losing private history/debug/tmp once is acceptable;
- A6's premise about claude's top-level `<cfg>` writes.

## Not closed here

- ranger-base-tbg3h
- ranger-base-qyq59
- ranger-base-k45vt
- ranger-base-pg251
