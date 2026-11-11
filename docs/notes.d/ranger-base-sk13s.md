# The L4 git carve-out, withdrawn: three pins the bead did not list, and what the degeneracy looks like (ranger-base-sk13s)

2026-11-11, Linux 7.2.15 x86_64, go1.27.3, from this bead's worktree.
ADR 0086 D8 at the mount layer, built from the design on ranger-base-56w67.
D3's build was ranger-base-8t6j1 one tier down; this is the same rule one
tier up.

## The change is two branches on a predicate that was already there

`CageMounts` opens with `ro := deniesFileWrite(ag.Deny)` — the same
predicate D3's five doors read — so D8 asked no new question of the PID:

1. the `<dir>/.git` read-write overlay is deleted. It only ever existed
   inside the `if ro` block that carries the `.beads` carve-out, so "do not
   mount it when the PID denies the whole tree" and "do not mount it" are
   the same line of code;
2. the `sessionCommonDirWrites` loop is skipped for `ro`, and the common
   dir's `Why` string says which of the two walls a reader of `posse cage`
   is looking at.

`sessionCommonDirWrites` itself is untouched. It answers "the three regions
a detached-HEAD commit in this tree writes", which is a fact about the tree
and not about a PID; the PID is read at the one call site, so that function
stays a reader of git. Kept, per the decision: the `.beads` carve-out, the
store-of-record overlay, `cageGitIdentityBinds`, and `PrepareSessionHead`'s
detach — which now has nothing to splice, and is left alone so there is one
launch path per tier rather than two.

## Eight pins asserted the old grant, and three of them were not on the list

The bead named five. The three it did not are the shape the decision is
about, and they fail for a reason worth writing down: ADR 0038 decision 4
binds `config`, `hooks` and the worktree identity chain `:ro` **over
whatever read-write mount covers them**, and after D8 nothing covers them.
`cageOverlay`'s same-mode rule then appends nothing, so the rendered list
has no mount on those paths at all.

| pin | what flipped |
|---|---|
| `TestAllowListShapeOverlaysWritableExtras` | `.git` → no mount |
| `TestBareAndFileFilterSpellingsMountNoOverlay` | same row, bare spelling |
| `TestWorktreeGitCommonDirIsTheGitCarveOut` | the fronts split: deny gets none of the three, no-deny keeps all three |
| `TestGitIdentityFilesAreReadOnlyInBothWritableShapes` | shape (2): `.git`, `config`, `hooks` → no mount |
| `TestWorktreePointerAndIdentityChainAreReadOnly` | **not listed** — the deny front's `worktrees/<own>` and the three chain files → no mount |
| `TestAbsentWorktreeConfigIsNeitherBoundNorCreated` | **not listed** — the Stat-guard arm needs a front that binds the siblings, so it is the no-deny front's |
| `TestNoGitLockSiblingIsEverBound` | **not listed** — the non-lock entries are not bound on the deny front either; the lock claim is carried by the list-free sweep beside it |
| `TestWritableExtraCannotOpenTheGitIdentityBinds` | `writable: [.git/hooks]` no longer brings `config` under a read-write mount; `writable: [.git]` still does |

The same argument applies to `TestWorktreeGitCommonDirIsTheGitCarveOut`'s
closing arm. It removes `<common>/logs` from the disk to show the overlay is
read off the tree and not spelled from a list — and it asked the DENY front,
which after D8 renders no overlay for a present source either. It asks the
no-deny front now, with a sibling row that is still mounted, which is the
only front where the Stat guard is observable.

**INVISIBLE is the stronger of the two walls**, and that is why none of this
is a hole: a path no mount covers cannot be written, and `cageOverlay`'s
deny direction has refused to bind such a path since ranger-base-yu5 on
exactly that reasoning (binding it `:ro` to look thorough hands over read
access the boundary refused). What D8 does to decision 4 is move two more
shapes into the degeneracy the worktree shape has always been in.

## The new pin is a sweep over the KIND of every mount

`TestNoGitDirIsWritableForAWholeTreeDenyPID` (`cageoverlay_test.go`), which
is ADR 0086 D6's fixture rule applied to this decision: drive the PID in a
real linked worktree AND a main checkout, and judge mounts by kind rather
than by membership of a list. Two directions, because a read-write mount
reaches a path under a git dir from either side of it:

- any mount whose destination is AT or INSIDE either git dir — every
  overlay this decision removes;
- any probe path under a git dir whose DEEPEST covering mount is read-write,
  which is how a main checkout's read-write repo mount reaches `.git`
  without an overlay of its own. The resolver is `cageCovering`, the one the
  renderer decides an overlay's mode by, so the pin asks the engine's
  question and not a looser one.

A path nothing covers is not a complaint, for the reason above. The control
is the no-deny front, asserted to **fail** the same sweep in both shapes:
without it the pin would pass over a renderer that mounted nothing at all,
or over a fixture whose git dir the mount list never reaches.

MUTATION-CHECKED 2026-11-11, both arms, one at a time:

| mutant | result |
|---|---|
| `cageOverlay(ms, <dir>/.git, false, …)` restored inside `if ro` | `main_checkout` red, 8 complaints, each naming the mount and its `Why`; `linked_worktree` green (there `.git` is a FILE and the overlay was always skipped) |
| the `sessionCommonDirWrites` loop run unconditionally | `linked_worktree` red, 11 complaints; `main_checkout` green (no common dir outside the tree) |

Each mutant reds exactly the shape it belongs to, which is also the evidence
that the two arms are two measurements and not one.

## The probe, re-run after the change

`docs/adr/0086-l4-git-carve-out.probe.sh`, written on the design bead when
both halves were hypothetical. Re-run here (2026-11-11, same D1 host, bwrap
0.13.1, git 2.56.0, loadavg 4.5) and its "after D8" arms are now what
`CageMounts` renders:

    LINKED WORKTREE, after D8:  cwd create refused · status ok · log ok
                                hash-object -w refused · commit refused
                                => worktree HEAD unchanged
    MAIN CHECKOUT,  after D8:   cwd create refused · status ok
                                hash-object -w refused
                                update-ref refs/heads/main refused
                                => refs/heads/main unchanged

and its "before D8" arms still reproduce the exposure: the detached commit
lands and `posse/s1` fast-forwards onto it, `refs/heads/main` moves. The
arms it labelled "proposed" are labelled "after D8" for that reason — the
script now describes the shipped code in one half and the history in the
other.

## A live pin has been asking for D8 since before the carve-out existed

`cageinnerliveqa_test.go`'s `TestQALiveCageMountBoundaryIsDeepAndTheNotebookSurvives`
(rangerhq-pafo, QA's adversarial half of rangerhq-6so) drives a PID denying
`Bash(git push:*)`, `Edit` and `Write` against a REAL main checkout
(`liveCageRepo` runs `git init` and commits), and asserts
`gitdir=refused` for `touch ./.git/written` and `gitcommit=refused` — in a
loop whose failure message is "a boundary with a hole in it is not a
boundary".

Until this bead that was false of the shipped renderer: `<dir>/.git` was
mounted READ-WRITE over the `:ro` repo for exactly that PID, and
`cageGitIdentityBinds` puts `:ro` binds on `config` and `hooks` and on
nothing else, so `touch .git/written` had a read-write mount under it. The
pin could only have stayed green by not running — it is `RHQ_LIVE_DOCKER`
gated, this box has no reachable docker socket, and the carve-out
(ranger-base-yu5, ADR 0014 §4) landed after the pin was written
(the older instance's bead-id prefix predates this one's `ranger-base`
prefix; both files arrive in this
repo's history at the same squash, 2026-08-31, so that is the ordering the
ids give and not a diff I could read).

Stated from the MOUNT LIST, not from a run: before this change
`TestGitIdentityFilesAreReadOnlyInBothWritableShapes` asserted
`wantMode(.git, read-write)` for this PID in a main checkout, which is the
same claim the live probe would have met. After D8 both agree, and the live
arm is the one that would prove it on an engine — still unrun here.

## What is NOT judged here

A PID that names a git dir in `writable:`. That is an explicit grant in the
PID's own text, honoured unconditionally at L2 (`pidWritableExtras`,
`seatbelt.go`) and at L4, so the two walls agree; deny-wins over it is
`TestWritableExtraCannotOpenTheGitIdentityBinds`' subject. D8 is about what
the tier hands a PID that asked for nothing.

The engine-side arm also stays off-box, as the decision's own alternative
says: `cageoverlaylive_test.go`'s allow-list and bare-`Edit`/`Write` arms
now want `git=refused` and are UNRUN — this seat cannot reach the docker
socket, and ranger-base-017dx is that venue.

## Two stale comments left behind, filed not fixed (ranger-base-w4lxe)

D3's own build left two comments in L2 tests that describe the grant it
removed: `seatbeltgitidentity_qa_test.go`'s `deniesFiles` shape row ("is
granted `<cwd>/.git` WHOLE instead of cwd — a different route to the same
config file") and `pathscoped_test.go`'s "the repo out, .beads/.git in". The
assertions under both are already post-D3 and pass; only the prose is
stale. Filed as ranger-base-w4lxe rather than fixed here, because they are
the other tier's lane — D3's build bead was ranger-base-8t6j1 — and this
bead's diff is the one that must stay readable.

## Box note

The full suite could not be read from `/tmp` while this ran: the box-wide
tmpfs quota (ranger-base-akk9f) was spent, and the first focused run red
four cage tests on `couldn't write … .lock` and `disk quota exceeded`
before any assertion was reached. Every reading here was taken with
`TMPDIR` under `$HOME`, which that fragment measures as costing a class of
pins elsewhere — see the close comment for which runs were and were not
available.
