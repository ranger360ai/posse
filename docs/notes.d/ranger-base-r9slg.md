# ranger-base-r9slg — twelve findings the release triage dropped

Twelve live findings from the verify of four release-triage drains
(ranger-base-iw783). The SCOPE half of those closes — "not release scope" —
was the operator's call and is not touched here. The LIVENESS half was the
factual claim that did not hold, and ADR 0006 §6's test is one question:
"would the fix change what a process does or what a pin proves?" Every line
below answers yes.

Three were production code or printed output. One was a residual the drain
unasked rather than an escape. Two were shipped doc sentences held mandatory
by a pin. The other six were arms and sets that no run tested, each one
measured by a mutant that left the whole suite green.

## What changed, by finding

**1 · the walled relaunch detaches before it renders** (herdrback.go).
`RelaunchAgent` renders the L2/L3 wall — `wallRendered` then `WallPrefix` —
and never asked `PrepareSessionHead`, so a walled seat whose tree was ATTACHED
when the re-type ran got a git grant naming a `refs/heads` entry and ran
behind that file for the rest of its life. Two states arrive there attached
and this binary relaunches both: a seat launched by a posse from before
ranger-base-pe5g5, and a seat that ran `git checkout <branch>` in its own tree
— the move the work prompt warns against in words. The call goes where the
create's does and for the identical reason: `WallPrefix` and `WrapInCage` both
read the tree's HEAD. The tree is rebuilt from the meta, which is the only
record of it that path has. One `caged` local now serves the trust seed, the
detach and the cage wrap, so the three cannot describe different sessions.
MEASURED: with the call removed, `TestQAAWalledRelaunchRendersNoBranchRef`
reds on all three arms and prints the grant.

**2 · the git-global snapshot's mode is verified and healed**
(gitglobalscope.go). A mode-only tamper — bytes untouched, lock directory in
place, 0444 turned to 0644 — was the one tamper posse neither detected nor
repaired: `gitGlobalSnapshotVerify` read the lock and the bytes and nothing
else, and the chmod lived only on the rewrite path, which a content-identical
snapshot skips. MEASURED 2026-11-20 at 9193d3b1: launch 2 returned nil over a
0644 snapshot, no DEGRADED line, every reader still naming it, and `git config
--global --edit` under the rendered row rewriting it at rc 0 (`git var
GIT_AUTHOR_IDENT` then answering "Author identity unknown"). The verify reads
the mode now, so every READER degrades to the operator's own file; the launch
chmods a content-identical snapshot back, so the mode heals where the content
always did. `Lstat` and `IsRegular`, so a symlink planted at the name is the
same tamper and is caught.

**3 · the launch row names grok's unrepairable case** (runtimecheck.go).
`posse runtime check` reported the strict-MCP door's unrepairable case —
claude behind a first word posse cannot see past — and said nothing about the
identical case one door over, so a grok line wrapped the same way carried no
`--no-leader`, read the operator's config.toml, and with `[cli] use_leader =
true` handed its tool calls to a leader the seat's cage does not hold, while
the grid printed a clean row. Every pin was green over it BY DESIGN:
`EnsureNoLeaderLine` leaving a line it cannot read alone is intended and
pinned that way, so the door's own pins cannot see the silence. The door's
refusal to guess is right; a row is the fix for a silence. The two readings of
"this line names that program" are one helper now (`lineRunsExe`).

**4, 5 · the seat store's no-wall arms** (seatstore_qa_test.go). A launch that
can name no per-seat wall dir is the one state in which the store cannot be
placed where ADR 0100 D1 requires. `seatStoreBinds` and `prepareSeatStoreAt`
refuse; `seatStoreCageMounts` cannot (it renders a mount list and returns no
error) so it fails closed, mounting the shared store read-only over its own
name. Nothing drove any of the three: `seatStoreNoWall` appeared in no test
file in the tree, and both shipped mount pins compose `wall` from a real
`os.UserHomeDir()`/`SeatWallDir`, so `l.Store` was never `""` in a test.

**6 · the wall venue's own D2(b)/D4(a) checks** (wallvenueargv_qa_test.go).
The venue's additions go on after `BwrapArgs`, so its own source and
destination steps never saw them and `boxLandedCage` asks both again. Deleting
those three lines left the suite green, and the shipped render cannot make
them fire: `wallVenueArgv` emits the /var/tmp source's in-place pin under
exactly `bwrapStrictlyInsideGrant(src, grants)`, the complement of the only
condition under which the check could refuse. The one shape that could in
principle — a source two components inside a grant with no mount between —
the render cannot produce either, because the venue's source is a SIBLING of
the judge's private /tmp source under the same run dir: any grant that leaves
one two deep leaves the other two deep, and `BwrapArgs`' own step refuses
first. Keeping the check is right (it is there for the next venue addition);
what it lacked is the fixture, which is now the shipped argv as control plus
one loose post-`BwrapArgs` mount per spelling.

**7, 8 · the verdict's unreadable-file row and its write order**
(wallwatch.go). `readWallWatchVerdicts`' "SAID AND NOT SKIPPED" arm became a
`continue` with the suite green, so the comment two lines above it — "the one
thing a reader of this surface must not be told is absent" — was a sentence no
run tested. And `wallWatchSaid.say` writes the durable file first and the pane
second; swapping the two left the suite green, although reversed a watcher
that dies between the writes leaves the line in a scrollback that is about to
close and nowhere else, which is the whole failure ranger-base-pfcci exists to
close. One pin for both call sites, because the order is in `say` and neither
site has its own copy. A DIRECTORY at a verdict's name is the unreadable shape
every uid sees, root included, with a chmod-0000 file beside it off root.

**9 · bwrap's mount grammar is one table** (bwrap.go). The grammar was a
hand-copied four-verb switch in each of two readers — `bwrapMounts`, which ADR
0100 D2(b)/D4(a) judge a render through, and `bwrapMountedAt`, which the wall
report reads — with nothing holding the two equal to each other and nothing
holding either equal to what the renders type. `--proc` was in NEITHER
although the wall venue types one, so /proc was a mount destination the
pinned-path predicate could not see; and a producer appending `--bind-try`
rendered a source no D2(b) step judged, invisible to the report as well.
`bwrapMountVerbs` is the one reading now, per option: operand count, which is
the source and which the destination. The overlay family is deliberately
absent and says so — an overlay's sources are a list built from preceding
`--overlay-src` options, and the predicate has no reading of that. The census
that keeps the table total over what these sources spell carries
pkgcensus-check's door.

**10 · a refused seat's reason goes to disk** (wallwatch_linux.go). The
watcher's pre-release refusals printed to the pane and nowhere else, and since
ranger-base-1l1en that pane closes with the watcher: a seat refused on one of
those lines left the operator a seat that vanished with the reason nowhere.
ranger-base-pfcci scoped itself to the respond path's say() lines and the
backstop's equivalent, so this was residual. All of them go through one
`refuse` closure now — verdict first, pane second — and the verdict path is
composed from the mounts file and this pid, so it is settled at entry, ahead
of the first refusal. Eleven lines and not the nine the bead listed: the two
`os.Pipe` failures are refusals in the same sense.

**11 · the landing copy says the HEAD a walled seat actually has** (AGENTS.md,
INSTALL.md §9). Both shipped copies told a dispatched session it works "on a
branch `posse/<session>`", and since ranger-base-pe5g5 a walled seat does not.
The work prompt was corrected when that slice landed; the two copies were not,
so a walled seat's standing orders contradicted its own work prompt — and
telling a seat its commits are "on" the branch invites the `git checkout
<branch>` that is one of finding 1's two states.
`landingcopy_qa_test.go`'s `sharedLandingClaims` pinned the stale literal in
BOTH copies, so leaving the pin alone is what kept the sentence mandatory.

**12 · the walled launch line is read for its exec too**
(launchexec_qa_test.go). No pin rendered a walled launch and asserted the exec
lands on its command word, although that is the shape every seat on this host
gets: arm 1's PID names no cage, so its lines carry no wall and no `3<`. The
pin reads whichever prefix the host renders — `WallWatchPrefix` here,
`BwrapPrefix` without the watcher, `SeatbeltPrefix` on darwin — because the
subject is the LINE and not any one producer.

## Prose corrected where the code moved

- `gitglobalscope.go`'s head block and `docs/notes.d/ranger-base-161t2.md`
  both said git's config lock means "every write … refuses at rc 255". True of
  the eight lock-taking spellings; not true of `--edit`, which takes no lock
  and is refused by the MODE alone.
- `docs/notes.d/ranger-base-pe5g5.md` said the producers' attached arm "now
  only serves a walled launch into a linked worktree that is not a posse
  session tree". It also served a walled RELAUNCH of one, which is finding 1.

## What is NOT closed here

- The call site at `landedcheck.go`'s venue arm is still held by nothing: no
  fixture can red it, for the reason finding 6 gives above, and the fixture
  pin that landed asserts the check's behaviour over the venue's argv rather
  than the presence of the call. A source census would close it; the finding
  did not ask for one and the two guards' complementarity is now written down
  where the next reader will find it.
