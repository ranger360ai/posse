# git's refs on the bwrap wall: a dir grant, a dot, and one question left for the host (ranger-base-gi1gi)

ADR 0080 D20, built 2026-11-08 as `BwrapWritable` in
`internal/posse/bwrap.go` beside D19's half, plus `applyBwrapRefGrant`'s
parity row and one line in `checkParityIn` (`internal/posse/parity.go`).
Pinned by `internal/posse/bwrapgitrefs_qa_test.go` (arm 1, 12 tests, ~7s over
real linked worktrees) and by `internal/posse/bwrapgitrefslive_test.go`,
which skips off Linux or without `bwrap`. Nothing calls it yet; the wiring is
D18's (ranger-base-djl3c).

Four things worth carrying forward, in descending order of how much they
matter to whoever picks up D18.

## 1. The dangerous entry is the one that does not exist

The L2 grant for a worktree session names three files in the shared common
dir: the branch ref, the `.lock` git renames onto it, and
`packed-refs.lock`. The ref LEAF is loud at this mechanism — it exists, so
`BwrapArgs` refuses the render and names it (B8: binding a file refuses the
atomic replace its writer makes). The other two do not exist at launch, and
a path that does not exist is not refused by shape 1, it is **prep**:
`mkdir -p`, outside the namespace, in the operator's git dir.

So the failure mode of forgetting either one is a DIRECTORY named
`<ref>.lock` or `packed-refs.lock` in the operator's `.git`, which fails
every commit on that branch, or every `gc` and `pack-refs` in that repo, for
everyone, until a human removes it. That is ADR 0059 D4's reason for
declining the container twin, reached here from the other side.

It is also why the pins assert the prep LIST and not only the argv. Mutation-
checked: leaving `packed-refs.lock` in the set reds five arms, and the one
that names the defect is
`the render asked for 3 writes inside the shared git dir … [mkdir -p …/packed-refs.lock]`.

## 2. The keep file is the one render input that renders no mount

`BwrapArgs`' other four inputs each become a `--bind` or a `--ro-bind`. The
keep file becomes a `PrepStep` and nothing else, which is why it is a
trailing argument of that function rather than a sixth list, and why it is
emitted with the write grants rather than with `carve.Keep`: the prep list's
ORDER is a promise to the launcher, and the keep file's directory is a grant
one loop up, so its `mkdir -p` has just been emitted. Moving the touch to the
head of the list reds exactly one arm, which is the one that says so.

Two refusals went in beside it. A keep file inside no write grant refuses —
the file's whole job is to hold a BOUND directory open, so one over a
directory nothing binds is a prep write that buys nothing and hides a missing
grant. And a directory at the keep path refuses: `pack-refs --prune` removes
an EMPTY directory, so a directory there is either pruned with its parent or
holds something that is not ours.

## 3. The dot is worth more than the ADR said, and T4d was wrong

Three of probe T4's rows are git's behaviour rather than the kernel's, so
they did not need the Linux host. MEASURED 2026-11-08, darwin 25.6.0, git
2.50.1 (Apple Git-155), over a real linked worktree:

| verb | `.posse-keep` | `posse-keep` |
|---|---|---|
| `pack-refs --all --prune` | dir survives, next commit lands | — |
| `for-each-ref` | clean, rc 0 | `warning: ignoring broken ref`, rc 0 |
| `show-ref` | clean, rc 0 | `fatal: bad ref`, **rc 128** |
| `branch` | clean, rc 0 | warning, rc 0 |
| `log --all` | clean, rc 0 | `fatal: bad object`, **rc 128** |
| `gc` | clean, rc 0 | `failed to run repack`, **rc 128** |
| `fsck` | **rc 8**, `badRefName` + `badRefContent` | rc 10 |

Two things follow. The dot is not a tidiness convention: without it the same
empty file breaks `git gc` and `git log --all` **in the operator's
checkout**, which is a worse failure than the prune the file exists to
survive, and it lands on the operator rather than on the seat. And D20's
probe row T4d — "git fsck says nothing of it", EXPECT `clean` — is false:
fsck is the one verb that does say something, and it says it either way,
because a dot-named entry can never be a valid refname. The probe's row now
reads `flagged`, corrected by measurement and not by a rerun, with a second
row for the verb that matters (`gc`) beside it.

The residual is stated in the ADR's Consequences rather than closed: `git
fsck` is not clean in a repo that has hosted a bwrap seat, permanently, one
empty inode per ref directory. The exit, if that noise is ever measured
costing anything, is a `.posse-keep` DIRECTORY — non-empty parent, no
ref-shaped entry at all, invisible to fsck — which is exactly what the render
refuses today because the letter asks for a file.

**The assertion had to go through `CombinedOutput`.** git warns about a broken
ref on **stderr at exit 0**, and the package's `git()` helper returns stdout
and folds stderr into an error it does not construct on success. The first
spelling of this pin read `fsck` through it and passed over a keep file named
WITHOUT its dot — an assertion of absence that measured nothing, found by
mutating the name.

## 4. What is left for the host is narrower than T4e

T4e asks what happens with no keep file when a pack runs under a live seat,
and EXPECTS the commit to be refused. That rests on the prune removing a
directory that is a MOUNTPOINT in the seat's namespace, and `vfs_rmdir`
answers EBUSY for a dentry anything is mounted on. bwrap's `--bind p p`
target resolves, through its own `--ro-bind / /`, to the same dentry as the
host's, so the prune may simply fail and the keep file be belt-and-braces.

Unmeasurable from darwin, and the design holds under either answer — the file
costs an empty inode. So the live pin asserts the COHERENCE of the two
readings instead of the outcome, and logs which answer the host gave:

* the commit fails → D20's reasoning, the keep file is load-bearing;
* the commit lands and the operator's checkout sees the branch move → the
  prune did not take the directory, the keep file is belt-and-braces;
* **the commit exits 0 and the checkout does NOT see the move → fail.** That
  is a ref write into an unlinked directory: exit 0, nothing red, and the
  launcher's later ff-merge finding no commit. It is the only outcome that
  must never be green, and it is the one an exit code alone cannot see — which
  is why every commit arm here reads the branch back from OUTSIDE the wall.

## The parity row, and why it is a declared difference

Two arms, one subject. A top-level branch in a linked worktree gets no ref
grant — the directory to bind would be `refs/heads`, with `refs/heads/main`
in it — so parity says so before the launch, instead of leaving a `cannot
lock ref` at the session's first commit with nothing to connect it to the
wall. Where the grant IS made, the same row states the widening it carries
(the directory holds every peer session's `posse/*` ref), because an operator
checks the design against that line and a line claiming a grant the argv does
not make is worse than no line. So the row reads `keep` back out of
`bwrapGitSwap` rather than re-deciding what was granted.

Declared difference in both arms (ADR 0017 §2): nothing here is a wall posse
asked for and did not get. The slashless arm is NARROWER than the tier's
darwin mechanism, and what it costs is a function the seat loses loudly.
Degrading it would put a safe-by-design refusal behind `--allow-degraded`,
which is the reading ranger-base-d17a closed.

It is also gated on availability, so it prints nothing on this host today and
nothing on the Linux host until D18 lands: a mechanism that cannot put the
wall up grants nothing, and parity already has the sentence to act on there
(`cage seatbelt is not available on this host/build`). The pure half takes
the mechanism, the tier, availability and the dir as arguments for that
reason — it is the only way a darwin box can read the Linux answer at all.
