## Four closes verified, and the arm a sub-ABI-9 host would have taken (ranger-base-pxprm)

A batched verify over four closes. Three of the four core mechanisms were
re-measured from scratch rather than read, which is what this fragment is for;
the one coverage escape worth a pin is pinned here, and the rest are on the
bead.

## The socket-wall step on the landed-check judge: the render half of D31

`TestQALandedCheckVerdictSaysWhenThereIsNoSocketWall` was added because a
mutant survived: the degraded SENTENCE was decided and never checked for
emission, the arm reading it sitting in an `else` only a host below Landlock
ABI 9 takes. The same disease was one mechanism over and survived the same
battery. `TestQALandedCheckJudgeIsNoStrongerThanASeat` already carries the
right assertion —

```
} else {
        if i := bwIndex(argv, WallUnixVerb); i >= 0 { ... }
```

— but selects it with `if wallUnixAvailableFor(WallBwrap)`, THIS host's real
answer, and this fleet is ABI 10. So the arm is unreachable here and its own
header says so. An arm no run reaches is a mutant nobody runs.

MEASURED 2026-11-14, D1 host, Linux 7.2.15-1-omarchy, bwrap 0.13.1, Landlock
ABI 10: dropping `wallUnixAvailableFor(m)` from the gate in `boxLandedCage`
— so the step is typed on every host, the sub-ABI-9 one included — left
`-run 'Anchor|Bwrap|Wall|Cage|Seatbelt|Landed|Venue'` over `internal/posse`
**green, 88.9s**. Seven other mutants of the same change each died one at a
time: the step never rendered; the step on the venue too; the bind-back of
`self` dropped; `self` taken from the judged tree; the degraded sentence
returned empty; the step's trailing `--` dropped; the step typed in front of
bwrap.

Closed by `TestQALandedCheckJudgeTypesNoSocketWallStepBelowTheABIItNeeds`
(`landedjudgeabistep_qa_test.go`), which stubs `wallUnixABI` — a var for
exactly this — and drives the real `boxLandedCage` at ABI 10, 9, 8 and -38,
reading the step's presence in the argv in BOTH directions plus the verdict
note as its complement. It reds on the gate mutant (the two sub-ABI rows), on
the step-never-rendered mutant (the two available rows) and on the
empty-sentence mutant, so it has a wrong arm each way and is not satisfied by
a render that never types the step at all.

Why it is not cosmetic: `RunWallUnix` refuses rather than execs when the
domain will not apply (ADR 0080 D4/D30), and the render is what keeps it off a
host where it cannot. A step typed below the ABI it needs is a judge that
never runs — every landing answers nothing, forever, while the verdict line
says DEGRADED.

## The anchor, re-measured end to end

Re-measured rather than inherited, bwrap 0.13.1, Linux 7.2.15, scratch dirs,
a live `bwrap` seat waiting on a signal file while the host acts:

| mount point | host `rmdir` | seat's mountinfo rows | seat's write |
|---|---|---|---|
| empty dir under `--ro-bind d d` | rc=0 | 1 → 0 | **lands in the host's dir** |
| same, one empty file inside | `ENOTEMPTY` | 1 → 1 | `Read-only file system` |

So the kills were CORRECT, not false, and the anchor is the whole of the
difference. The same two readings over a `--tmpfs` mount rather than a
`--ro-bind` are identical — rc=0, 1 → 0, and 1 → 1 with a file inside — so the
measurement is about the mount point's emptiness and not about which mount
goes over it.

## Where `posse upgrade`'s private copy stands

MEASURED 2026-11-14, git 2.56.0, scratch repos, the premise and the fix from
the other side:

- A loose blob overwritten in place with different bytes leaves `rev-parse
  HEAD` unchanged and `git worktree add --detach <sha>` checks out the
  substituted file. The premise reproduces.
- `git init` + `git fetch <local path> <full sha>` into an empty repository
  refuses it: `fatal: missing blob object` / `did not send all necessary
  objects`, rc=1. The receiving side re-hashes, and that is the fix.
- A fetch of a sha that is REACHABLE but is no advertised ref tip also
  succeeds (rc=0) with no `uploadpack.allow*` setting, so a tip that moves
  between the plan and the fetch is not a refusal this verb has to explain.
- `--no-replace-objects` is a belt and was measured to be one: with a
  `refs/replace` in force in the source, `cat-file` there serves the
  replacement, and the fetch delivers the TRUE bytes with the flag and
  without it, copying no replace ref either way. A missing pin over that flag
  is therefore a missing pin over a redundancy.

## A number corrected

`docs/notes.d/ranger-base-3iybl.md` reads "113 `merge-back blocked` beads, 1
of them this reason". Re-counted 2026-11-14 over the same
`$BEADS_DIR/issues.jsonl`: **108** beads whose title starts with that phrase,
108 distinct ids, no id appearing twice; the phrase appears on 309 lines of
the store once descriptions and comments are counted. The load-bearing half —
one of them this reason, so a one-off and no AGENTS.md line — is confirmed.
The denominator is five out, and a store that flattens duplicate lines is the
likeliest reason a line count taken that evening read higher than a bead count
taken later.

Everything else in that fragment reproduced exactly: `git checkout HEAD~0`
prints the whole `advice.detachedHead` paragraph (571 bytes) where `-q` prints
zero bytes on both streams; `git status` detached says `HEAD detached at`; a
quiet path-limited commit while detached exits 0, says nothing and moves no
branch; `git status -sb` reads `## HEAD (no branch)` against `## main` where
plain `--porcelain` is byte-identical in both states.

## Not closed here

- ranger-base-ua625
