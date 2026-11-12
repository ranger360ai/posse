# The landed-check filer's two unmeasured arms: a bool that read three facts as one, and the row whose dedupe is the record (ranger-base-w9w37)

ADR 0088 D5's last sentence is the invariant: "One open bead per red stretch
replaces several seats each filing a P1 for the same exit code, which happened
with ranger-base-4v4r6." The filer (`internal/posse/landedfile.go`,
ranger-base-ddgfr) kept that invariant across the five rows of D5's table and
broke it on both paths the table does not describe — what the filer does when a
bd call or the record write FAILS. ADR 0073 measured those failures as the norm
under concurrency rather than the exception, and every bead this rule files is a
P1 in the `-l code` lane, so a duplicate is a dispatched seat, a worktree and a
suite run.

Found by verify bead ranger-base-9rbfq over ranger-base-ddgfr's close; both
repros MEASURED 2026-11-11 on this Linux box, go1.27.3, at main ff7130a7.
Dated 2026-11-12.

## Three facts, one bool

`commentStretch` returned "did I comment", and the red→red arm read that as
"does this stretch have a bead". The two differ in three places:

| the store said | the bool | what it means |
|---|---|---|
| no bead carries this stretch's marker | false | the degenerate half of D5's red→red row: file |
| the READ failed (`bd list`: database is locked) | false | nothing is known about this stretch's bead |
| the bead is open, `bd comments add` failed | false | this stretch HAS a bead — the line above read it |

So a locked database or a failed comment filed a twin P1 whose body said "the
bead for this red stretch is not open any more" — unknown in the second case
and false in the third, which the filer had disproved one line earlier.
MEASURED: two open beads, `q-1 main red at <tree>` and `q-2 main red at
<tree'>`, with q-1 open the whole time. The twins were also unassigned (the
degenerate arm has no green tree on record, so its window is the one landing it
judged), so "one ATTRIBUTED bead" failed with the count.

The fix is a named tri-state, `landedStretchState` — unknown, none, open — and
only `none` may be answered by filing. An unknown is re-asked by the next
landing for free; the ⚠ is the record meanwhile.

## The green→red row's dedupe is the RECORD, and it was not consulted

`recordLandedCheck` is best effort by contract (ADR 0088 D4) and the filer ran
unconditionally after it. A `git config` write that cannot take
`.git/config.lock` — a shape AGENTS.md and ADR 0059 D3 both already name on
this box, and any concurrent `git config` on a shared checkout is one — leaves
the record naming the stale GREEN tree. Every later red landing then reads
green→red again, and `file`'s green→red arm went straight to `fileRed` without
looking for anything. MEASURED, three red landings with the lock held across
each judge: three open P1s, one per landing, which is the ranger-base-4v4r6
incident itself.

**Why the marker cannot cover this row**, which is the half the fix had to get
right rather than the half it had to type. The other three rows dedupe on the
marker in the store, and the obvious symmetry — have green→red consult it too,
and comment when a bead is open — is wrong: a filed bead stays OPEN after its
stretch ends, because closing it is the fixer's job (D5 row 4). So "an open
bead carries this marker" cannot tell a NEW stretch from a repeat reading of
this one, and a filer that deduped on it would silence the second genuine red
stretch of every repo forever. The record is the only thing that distinguishes
them, which is exactly why `openStretchBead`'s own doctrine is that a green→red
transition files whatever is already open.

So the record's write result is passed to the filer, and a verdict that could
not be recorded files nothing and says so. The next landing re-judges the same
tree — what D4's cache does everywhere else — and files when the record lands.

**The trade, stated.** The gate costs the bead one landing, and while the
record cannot be written at all it costs the bead entirely: a red main under a
stranded `.git/config.lock` gets a ⚠ in the pass's log and no P1. That is the
cheaper failure of the two available. One P1 per landing is what D5 exists to
prevent, and the price is paid in seats; the ⚠ is in the same log the verdict
itself is printed to.

**A residual, not fixed here.** The three COMMENT rows are deliberately not
gated — their dedupe is the marker, which the record has no part in — and D5's
red→green row therefore comments "green at <tree>" once per landing while the
record cannot be written, instead of once. That is duplicate prose on one bead
a human reads, and no seat: left alone rather than gated, because gating it
would drop the "main went green" line the fixer is waiting for.

## Making a `git config` write fail in a test, and what else breaks when you do

Planting `.git/config.lock` is the whole of it: `git config <key> <value>` then
exits non-zero with `error: could not lock config file .git/config: File
exists`. MEASURED on this box with that lock held, everything else a judged
landing runs goes through untouched — `git worktree add --detach`, `git add`,
`git commit`, `git worktree remove --force`, `git log`, `git cat-file`.

One fixture habit does not: `commitIn` (worktree_helpers_test.go) writes
`user.email` and `user.name` on every call, so a landing made while the lock is
held fails in the helper rather than in the code under test.
`ldfLandUnderLock` is the same commit without those two writes — `wtRepo` has
set the identity already.

## The pins

`internal/posse/landedfile_test.go` (arm 2), three arms, 3.4s together, each
measured red against the pre-fix code and green after:

- `TestARedWhoseStretchBeadCouldNotBeReadFilesNoTwin` — pre-fix: 2 open beads.
- `TestARedWhoseCommentFailedFilesNoTwinOverABeadItReadAsOpen` — pre-fix: 2
  open beads, and the log carrying "not open any more" about an open bead.
- `TestAVerdictThatCouldNotBeRecordedFilesNothingUntilTheRecordLands` — pre-fix:
  one bead at the first landing and one per landing after. It also pins the
  other half, which is what keeps the gate honest: when the lock frees, the
  same red tree files exactly one bead, bounded by the green tree the record
  finally names, and the landing after that comments on it.
