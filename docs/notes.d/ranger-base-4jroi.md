# A count that came through a failed reset is not a pattern (ranger-base-4jroi)

*2026-11-13 · developer · code bead under ADR 0088 D6.3, from
ranger-base-vzn36's verification of ranger-base-lhiy1*

## What was wrong

D6.3's watchdog files one P1 `-l architecture` bead on the second
CONSECUTIVE judged run that returns no answer, and the bead carries both
runs' sentences. The count lives in `posse.landedCheckQuiet`; the sentences
live in memory, because the record is spelled `<tree> <n>` and a sentence is
neither.

An answered run between two silences resets the count and clears the
remembered sentence. When the reset cannot be WRITTEN — one
`.git/config.lock` lost to a concurrent persona, which AGENTS.md and ADR 0059
D3 both name as routine here — the clear still happens and the count still
stands. That is ranger-base-w9w37's rule working as designed: a count whose
write did not land keeps counting rather than files. What was not designed is
the bead the next silence then wrote, which claimed two things that had not
happened:

    The landed check at <repo> returned NO ANSWER on 2 consecutive judged runs
    ...
    What the runs said:
        1. (not recorded: the earlier run was judged by another launcher
           process, which is where this sentence is held)

One launcher judged all three runs, and an answered GREEN run sat between the
two silences. So the pair was not consecutive, and the absent sentence was
not another process's — it was this process's own, cleared two lines above
the failed reset. The reader of a D6.3 bead is architecture, and this one
sent them looking for a launcher restart that never happened.

A restart was the only cause `silentDescription`'s `prev == ""` branch named;
ranger-base-lhiy1 recorded it as an ASSUMED, and ADR 0088 D6.3's residual
said it too. A failed reset is a second way to reach the same branch.

## The fix

`landedChecker.quietUnreset` is set for a repo whose answered run could not
write the reset, and cleared whenever a reset lands or the count reads zero.
`silentDescription` takes it, and over it says so: the count is not claimed
consecutive, and the absent sentence is attributed to the answered run that
cleared it. In memory beside `quietSaid`, and for the same reason — a third
field in the record reads as no record at all
(`TestTheQuietCounterReadsZeroOverAnyShapeThatIsNotItsOwn`) — so a launcher
that restarted carries neither mark nor sentence and gets the restart
wording, which over a restart is true.

Counting from 1 after a failed reset is unchanged, and so is the clear on the
answered run. Both were right; only what the description may CLAIM over them
was wrong.

## Two arms, and what they kill

Both are `-tags posse_arm2` in `internal/posse/landedhealth_test.go`, over
`ldfFixture`, driving real runs through `landedChecker.once` as the rest of
that file does.

`TestASilenceCountedThroughAFailedResetClaimsNoPatternAndNoRestart` walks the
repro: silence, an answered run with `.git/config.lock` planted, the lock
freed, silence. No landing is needed between the answered run and the second
silence because the lock blocks every `git config` write, the verdict's own
included, so D4's cache has nothing to answer from.

`TestASilenceAfterALauncherRestartSaysTheEarlierSentenceIsHeldElsewhere`
holds the other branch, which is what makes the first a distinction rather
than a new constant: a second `landedChecker` over the same repo IS the
restart — the record survives it, the memory does not.

The site was unpinned before this, in both directions. MEASURED 2026-11-13,
one mutant at a time over a clean tree, restored after each; all five are
KILLED by the two arms above, and the third of them survived all ten of
D6.3's existing arms when ranger-base-vzn36 measured it:

| mutant | killed by |
|---|---|
| `silentDescription`'s unreset branch dropped | failed-reset arm |
| `markQuietUnreset(repo, !reset)` dropped | failed-reset arm |
| `rememberQuietSaid(repo, "")` on the answered run dropped | failed-reset arm |
| the unreset wording made unconditional | restart arm |
| the two `prev == ""` causes swapped | both |
