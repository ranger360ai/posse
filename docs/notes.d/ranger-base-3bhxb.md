# The park census has one base, and the carrier annotation is witnessed

ranger-base-3bhxb, the two `-l devops` findings of ranger-base-5pkix's bundle
(from ranger-base-j7ekm). Both turn on `scripts/park-carriers.sh` and on
`docs/runbooks/push-hold-park.md`, which is why they did not land with that
bundle's four code findings.

## F3 — two records, two bases

The park procedure had two records prescribing different bases for the same
census step: the runbook's step 3 ran `scripts/park-carriers.sh` bare, taking
the script's default (`main`), and the push delegate's standing orders — which
live outside this repository and are the whole of ranger-base-y1mgf's close —
ran it with `PARK_CARRIERS_BASE=origin/main`.

The base is not a detail. The held set is `<park> --not <base>`, so a base
behind the commit step 2 reset to pulls commits that are legitimately on the
base branch into the held set, and then every live ref that reaches one of
them reads as a carrier. A hold IS a refused push, so the base branch is ahead
of `origin/main` whenever a landing has been made and not pushed, and the
reset point is ahead of it too: the remote-tracking ref is behind in exactly
the case the page is for.

MEASURED 2026-11-22, git 2.56.0, in a scratch repository built to the hold's
shape — `origin/main` two landings behind the base branch, one held commit,
one real carrier branch cut after it, one sibling branch cut from the reset
point:

| base | held set | live carriers named | exit |
|---|---|---|---|
| `main` — the script's default | 1: the held commit | 1: the real carrier | 1 |
| `origin/main` | 3: the held commit and two landings | 3: the real carrier, the base branch itself, and the sibling | 1 |

Two of the three rows the second run named are false, and the two runs are
otherwise indistinguishable — same exit status, same row shape, the sibling's
row carrying a correct worktree annotation of its own. The only thing that
separates them is the base, which the script prints on every line it writes.

**NOT OBSERVED IN THE WILD**, and said plainly: all 370 `origin/main` reflog
entries on this box are one-per-landing pushes, so the base branch and
`origin/main` have been equal at every park so far — they are equal as this
lands — and the delegate's first live run named exactly one carrier,
correctly. This is a latent disagreement with a measured cost, not a defect
anybody has paid for.

**Decided: the default is the base, and the override is not a knob for the
procedure.** The runbook's step 3 now says so, prices the wrong base with the
table above, and tells its reader to read the printed base before the exit
status. `scripts/park-carriers.sh`'s `base=` line carries the same reason
where a reader of `PARK_CARRIERS_BASE` actually lands.

**What this repository cannot hold, and the handoff that follows from it.**
The second record is a persona memory file outside this tree, so no pin here
can read it — `internal/treepins/parknamespace_qa_test.go` arm 2 holds the
procedure page and nothing else can. A record that duplicates a prescription
can drift again; a record that CITES one cannot. So the handoff asks the push
delegate for the one-line edit as a citation of step 3 rather than a corrected
copy of its command, which is the only form of the fix that stays fixed. That
edit is hers because it is her own memory (ADR 0006 §1).

## F4 — the annotation nothing ran

`worktree_of` produces `[checked out at <path>]`, the one field on a carrier
row a reader ACTS on: step 4 of the runbook and the script's own header both
turn on it, because a ref a live seat holds cannot be moved at all. Neither
`--self-test` arm ran its producer. The hermetic fixture built its carrier
branch in a scratch repo and never checked it out in a second worktree, so the
annotation was never asked for and never missed.

MEASURED 2026-11-22 by ranger-base-j7ekm, two blindings, `--self-test` after
each, both green:

| blinding | before | after |
|---|---|---|
| the `[checked out at $path]` echo deleted | SELF-TEST OK, 2 arms green | KILLED |
| `worktree_of` matched against a branch name that cannot exist | SELF-TEST OK, 2 arms green | KILLED |

The "after" column is this bead, re-run over copies of the mutated script with
an unmutated control green in the same run. For contrast, six other mutants of
the same script were killed by the original two arms — `refs/heads` dropped
from the scanned namespaces; a live carrier classified as a posse record;
`--not <base>` dropped from the held set; the ancestry test inverted; the
carrier rows counted but not printed; `report_carriers` returning 0 instead of
1 — so the door was a good one and this was its one hole. The mechanism itself
worked: F3's live run printed the annotation for all five of its rows with
correct paths.

**The fix is a third arm**, between the two that exist: `git worktree add` the
carrier branch in the scratch repo, re-run the census, and assert the carrier's
row names that directory. It asserts the trailing path component and not
byte-equality with what `mktemp` printed, because git records a worktree path
as IT resolves it and a box whose temp root is a symlink (darwin's `/tmp` ->
`/private/tmp`) hands back a different string. The worktree is removed before
the third arm's branch delete, since git refuses to delete a branch some
worktree has checked out.

Hermetic as before: a scratch repo under a fresh temp root, nothing on the box
read or touched, removed on the pass and on every fail path rather than from
an EXIT trap.

## Not closed here

- ranger-base-nxxn6
