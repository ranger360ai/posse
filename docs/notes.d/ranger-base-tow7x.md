# The coldexec register was already clear; what reds `tree-check` now is the box (ranger-base-tow7x)

2026-11-11, Linux 7.2.15 x86_64, go1.27.3, 16 cores, from this bead's
worktree at HEAD 04c881a6 (= `main`), clean tree, loadavg 2-4.

Filed from ranger-base-4kars, which hit `make tree-check` red at
`corpus-check`. **The defect this bead names was fixed before the bead's tree
was cut**: ranger-base-r5tnw deleted the two stale rows in 537cd425, and that
commit is an ancestor of `main`. `cxaUncoveredPlanters` is empty,
`coldexec-check` carries all five arms (Makefile:1219-1223), and both pins
read `ok 0.245s`. No code change was needed and none was made.

## The derivation really is re-armed, measured here and not inherited

r5tnw measured this at ef842dbe; re-measured on this tree because the whole
point of the deletion is that the derivation now holds those arms up.
Mutation: delete Makefile:1222-1223 (the `./cmd/posse` and
`./internal/treepins` arms), nothing else.

    go test ./internal/treepins -count=1 \
      -run '^TestQAEveryPlantingPackageHasAColdexecArm$'
    -> FAIL 0.206s, naming BOTH packages, coldexecarms_qa_test.go:366 and :379,
       each with its call-site count (cmd/posse 17, internal/treepins 53) and
       the recipe line to add.

`git checkout -- Makefile` after; md5 `3a0831035d023ebadce371069a549d52`
before and after, `git status` clean. So an arm silently lost from
`coldexec-check` reds today, which is what finding 2 of r5tnw was about.

## `tree-check` IS still red here, for a reason with no code in it

MEASURED, same tree, `make -k tree-check`, exit 2 in 33s. Four doors red —
`pkgcensus-arm2-check`, `pkgcensus-arm3-check`, `corpus-check`,
`register-check` — and **every one of the four is the same one error**:

    link: mapping output file failed: disk quota exceeded
    compile: writing output: write $WORK/b132/_pkg_.a: disk quota exceeded
    write /tmp/TestQAParallelClearance...../004/ciwatch_test.go: disk quota exceeded

`register-check`'s is the most misleading of the four, because the pin
rephrases it as drift: `TestQATheTreeWideDoorsReportRealDrift` reports
"`make crew-check` failed on a clean copy of this tree — it reports drift that
is not there", and the quoted output underneath is the compiler running out of
inodes.

The cause is not bytes. `/tmp` is a 14G tmpfs with **2.8G free and its inode
table full**: `df -i /tmp` reads 1042105/1048576 used, 6471 free, 100%, against
`nr_inodes=1048576` on the mount line. It is mounted `usrquota`, which is why
the kernel answers `EDQUOT` ("disk quota exceeded") where an out-of-inodes
tmpfs would otherwise say `ENOSPC` — so the error names a quota nobody set and
hides a ceiling somebody did. A 1KiB write probe into a fresh `/tmp` dir
succeeds while the suite's many-file writes fail, which is the same fact from
the other side.

## The same tree, same commands, with `TMPDIR` off the tmpfs

    TMPDIR=/home/posse/<dir> GOTMPDIR=/home/posse/<dir> make -k tree-check
    -> exit 0 in 38s, all sixteen doors green, no FAIL line anywhere.

So `tree-check` on `main` is green and the box is what is red. That
corroborates ranger-base-w5z7u ("a sibling seat's TMPDIR under /home does
not") over the whole door class rather than over `coldexec-check` alone, and
it is the argument for l23nb's recommendation (move `TMPDIR`) rather than for
raising a quota. The leak itself is owned: ranger-base-b27pn (310 mktemp dirs
per arm-2 run, 94% of the inode table) and ranger-base-vw1cp (leaked test
homes; the disk preflight reads only bytes). Nothing was filed from here and
nothing reclaimed — the reclaim is l23nb's, and it is the operator's.

## For the next reader

A red door whose message is about *drift*, *census* or *staleness* is worth
reading to the bottom before believing its subject line. Four doors with four
different subjects said four different things here and all four were `df -i`.
