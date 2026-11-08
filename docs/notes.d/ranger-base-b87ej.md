# The commit wall says which home rendered it (ranger-base-b87ej)

ADR 0081 D2, implemented 2026-11-07. The `prepare-commit-msg` body now carries
one comment line — `# posse-home: <absolute home>` — and a reader that finds
bytes which are not its own render asks whose they are before it calls them
stale.

## What the old reader said, in its own words

MEASURED 2026-11-07, with the D2 refinement removed from
`commitGuardHomeVerdict` and nothing else changed, over a repo whose installed
guard differs from the reader's render **only** in the home that rendered it:

```
L3 prepare-commit-msg hook — …/.git/hooks/prepare-commit-msg — ours but stale
— run `posse gates install-hooks`; no named guard is missing from the
installed body — the beads visibility, constitution-path, shared-index and
NOTES.md guards are in it, this render's or an older one — so the drift is
elsewhere
```

That is the whole defect in one line. The verdict is wrong (the body is
another home's CURRENT render, not this home's old one), the remedy is wrong
(`install-hooks` from here writes a file the other home's next launch writes
back — the two-writer loop ranger-base-q0np6 measured), and the derived
consequence clause is reduced to telling the operator that it cannot find the
difference it is reporting. With D2 in force the same file reads:

```
L3 prepare-commit-msg hook — …/.git/hooks/prepare-commit-msg — rendered by
home <path>, not by this one: a repo's shared commit wall has one owning home
(ADR 0081 D2), so this is not ours gone stale — re-render it from whichever
home should own it; …
```

## Where the stamp comes from, and why it is not a parameter

`CommitGuardHook` is a method on `*App` now, not a package function with a
`home` argument. The stamp's whole job is to tell a reader which home rendered
the bytes, and that reader cannot check the claim against anything but the
file. A parameter would let a caller render a body stamped with a home that did
not render it — exactly the claim every reader would then believe. On the
receiver it cannot be spelled wrong.

The byte-exactness doctrine is unchanged and now has a third clause: a fixture
that must match what an installer wrote has to pass the same identity literals,
the same ops-pattern set, **and** be rendered from the same home. The ~60
fixtures in this package that only inspect the body's content render from
`qaNoHome` (`&App{}`, no Home, no stamp) and are byte-identical to what they
were; the ones that compare against an install or feed a probe render from the
App under test. One shipped test was installing from `installCommitGuard`'s
homeless App and probing from a home-bearing one
(`TestL3ProbeSeesTheWallFromInsideAWorktree`) — that is the whole class, and it
reds loudly rather than quietly, because the probe reports `l3Stale`.

## The migration, measured on this box

ADR 0081's Consequences predict that every body installed before the ADR reads
as a difference once. MEASURED 2026-11-07, `scripts/verify-hook-freshness.sh`
run from a fresh build against every repo this instance's `beads_visibility:`
declares, four of them:
**4 of 4 findings, all of them `is STALE`, none of them classified by home.** That is the intended reading and not a gap — an
unstamped body is D3 case 4, the one state a launch rewrites without asking, so
calling it another home's would have re-worded every finding on the box on the
day this landed. One `posse gates install-hooks` per repo clears them.

The same script, same build, over a scratch repo stamped by a scratch home X
and read from a scratch home Y that declares the repo identically (so the stamp
is the only line that can differ): one finding, `was rendered by home
<X>, not by this one`, exit 1 — and run from X instead, `fresh`, exit 0. Both
halves of the ADR's DONE WHEN, through the shell reader rather than only
through the Go one.

## What the shell reader does NOT do

It does not normalize the stamp line away. The reference render is taken per
repo from the binary on PATH (that is the control's whole design), so the
stamp it carries IS this run's home, and no home path is spelled in shell
anywhere — a value recomputed there would differ by a symlink or a trailing
element and report every repo on the box as a foreign home's. And a reference
from a `posse` predating D2 carries no stamp at all, which switches the
classification off and leaves the script saying exactly what it said before.

## Still open

D1, D3 and D4 — ownership, the launch's write rule, and `install-hooks`'
refusal — are ranger-base-lh9nj. Until they land, the launch still rewrites a
foreign-stamped wall, so `l3Verdict.reStamped()` reports `l3OtherHome` as
re-stamped and herdrback's "found the wall WRONG before this launch just
silently re-stamped it" warning carries the new line. Narrowing the predicate
ahead of the write would make the launch take the wall in silence.

LANDED 2026-11-07, D1 and D3 (ranger-base-lh9nj): the launch no longer rewrites
a foreign-stamped wall it does not own, and takes ADR 0052 D2's per-session dir
instead. `l3Verdict.reStamped()` is unchanged — it is a fact about the body, and
an OWNING home still re-stamps (D6) — so the narrowing is at the call site,
which asks `sharedCommitWallWrite`. D4 (`install-hooks`) is still open:
ranger-base-hnujh. See docs/notes.d/ranger-base-lh9nj.md.
