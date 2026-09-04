## The launcher says how far behind it is, and where that line had to go (ranger-base-z3hx6)

`docs/notes.d/ranger-base-pju9t.md` ends by filing this: the fleet ran an
eight-hour-old binary, 34 commits behind `main` by the end, two of those 34
each independently stopping the merge-back block that a fourth seat then
spent a session re-deriving — and nothing anywhere printed the gap.

The reading is `internal/posse/launcherlag.go`. It is the command that note
gives, with the repo argument resolved rather than assumed:

```bash
git rev-list --count "$(posse --version | sed 's/.*+//; s/ .*//')"..main
```

The stamp picks its own checkout out of the repos the instance knows about —
each resolved to its MAIN checkout (`MainCheckout`, so a session worktree
cannot answer with a `main` a persona could move), each required to declare
`module github.com/ranger360ai/posse`, each required to actually hold the
stamp commit. Then one `git rev-list --count`.

### The design question the bead left open, and the measurement that closed it

WHERE it prints was deliberately not decided, with three defensible homes: a
pass banner, a `posse doctor`, and the merge-back handoff's own description.

The obvious one is wrong, and it is wrong by construction rather than by
taste. `posse dispatch --watch` already prints the binary's identity once in
its preamble (`ReportPosseBinary`, ranger-base-39jnl), and the lag looks like
it belongs on the next line. It does not, because **the gap at install time
is zero**:

| binary | commit | committed | installed / built | `rev-list --count <stamp>..main` at that instant |
|---|---|---|---|---|
| the 23:54 build | `c592683` | 23:52:01 | 2026-09-03 23:54 | **0** |
| its replacement | `9920e75` | 06:35:42 | 2026-09-04 07:16:28 | **0** |

A binary is built from the tip and the loop is started right after it, so the
one moment a start-of-loop line speaks is the one moment there is nothing to
say. The next commit after `c592683` landed at 23:55:47 — **1m47s** after the
build — and the other 33 followed over the next eight hours, under a loop that
cannot change its own binary. So the reading lives in the PASS, which is the
only clock still ticking while the binary is not, and `posse status` carries
it for an operator who asks directly. `posse doctor` does not exist, and an
operator-typed command is exactly what nobody typed for eight hours.

### Cadence: doubling

Every pass is ~90 lines over that window, which is how a visible line becomes
an invisible one — the rule the same preamble already keeps for the hook wall
and the stale-plan typo. Once is the banner above, and it is always zero. So
the line is said when the number has DOUBLED since it was last said: 1, 2, 4,
8, 16, 32 — six lines over eight hours, the last of them alarming. A loop that
STARTS behind says so on its first pass. Doubling is this loop's own idiom: a
quiet pass doubles its sleep, for the same reason.

An abstention — a binary that names no commit, no checkout holding the stamp —
is said once in the preamble instead. A reading that cannot be taken must not
render as silence, because silence is what an all-clear looks like.

### It decides nothing

It prints, it warns, and it does not move `posse status`'s exit code
(`possebinary.go`'s rule, one file over). Installing over a binary that is
dispatching a live fleet is a live change and stays the operator's; the bead
put auto-install out of scope in the same words.

### Two things measured on the way that are worth keeping

**A second window opened while the bead was open.** The 07:16 install was 28
commits behind by 09:47 and 29 by 10:0x — `main` moves ~11 commits/hour on
this box — so the class is not a one-off, and the six-line drumbeat would have
been at "16 behind" by mid-morning.

**Two synthetic repos seeded in the same second are one history.** Same tree,
same message, same author and committer timestamps give git the same sha, so a
rig asserting "this checkout does not hold the stamp" was silently asserting
it about a checkout that did. The rig now writes its own path into the seed
commit. Same family as `same-second commits make git log order arbitrary`.

### What was NOT taken

Stamping the lag into `noteMergeBlocked`'s description — the persona-facing
half, and the exact measured cost. The bead's DONE WHEN makes the PASS the
subject, and `noteMergeBlocked` is a package function called from three sites
(`dispatch.go`, `landsweep.go`, `closeddirty.go`'s `HerdrBackend`) of which the
third holds no `App`. Left out on scope, not on merit.
