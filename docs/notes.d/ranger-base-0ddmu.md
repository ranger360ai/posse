# The other direction of the wall-clock-age class: a premise that is fresh (ranger-base-0ddmu)

MEASURED 2026-11-06 on this box (darwin 25.6.0), at `main` 24ab7bbf. One live
finding from ranger-base-rrklg's verification of four closes
(docs/notes.d/ranger-base-rrklg.md §5 has the record and the other three).

`cmd/posse/planstale_qa_test.go`,
`TestQAStatusQuietWhereTheLineWouldLie/fresh_reading`, seeded a plan-usage
reading one minute old, launched the binary, and asserted it printed nothing
about blindness. "One minute old" is not a number the row cares about: it is
the row's way of saying *fresher than `PlanUsageStaleAfterDefault`*, which is
two hours (`internal/posse/planstale.go`). That is a wall-clock deadline
stated in one process and read in another, the class ranger-base-gt0q9
measured, and the row carried no guard.

The repro is one character of the fixture — the seed's age read `-3h` instead
of `-1m`, seeding the state a 2h+ sleep leaves behind and changing nothing
else:

```
--- FAIL: TestQAStatusQuietWhereTheLineWouldLie/fresh_reading (1.41s)
    planstale_qa_test.go:134: fresh reading must print nothing about blindness:
    ...
    plan meter BLIND 3h00m: last reading 2026-11-06T11:14Z ... 1 consecutive failed reads
```

The binary is right in that output and the fixture is what expired.

## Why ranger-base-dsuk7's bracket did not already cover it

The two pins that bead fixed seed PAST the threshold and assert the age the
binary renders, so their exposure is an age read off the wrong clock, and the
answer is a bracket of every age the reading honestly had (`agedAgesOf`) with
a skip beside it. `seedAgedQuietSnapshot`'s own comment names the reason a
jump cannot hurt them in the other direction: an age only ever gets older, so
nothing can take a reading seeded `-3h` back under two hours.

This row is the mirror. It seeds UNDER the threshold and asserts SILENCE, so
there is no age to bracket — and a jump of over two hours takes its premise
over the line, after which the binary correctly prints the BLIND line and the
pin reds accusing the code. Same class, opposite sign, and the bracket half of
the machinery has nothing to do here. `skipIfTheBoxSleptMidTest` is the whole
fix.

So "an exact age with no bracket" was never the definition of the class, and a
census that looks for age literals cannot find this row — it has none. The
discriminator is the one ranger-base-w4enh states: whether ONE clock reads
both sides of the premise.

## The fix

`freshSeedAge` (one minute) names what the row seeds, and the reference is
taken inside the subtest rather than in the table literal, which is evaluated
once when the table is built. `.UTC()` is applied to the value written to the
store and to nothing else: it strips the monotonic reading, and
`skipIfTheBoxSleptMidTest` fatals on a reference carrying only one clock,
because `wallJump` of such a time is identically zero — a guard that looks
installed and asserts nothing. The guard is called after the launch and before
the assertion, since the jump breaks the premise while the binary is reading
it.

The sibling `no meter guard` row is immune and is left alone: it seeds a fixed
calendar date, no clock can move a date, and with no meter guard configured
nothing is ruling on the snapshot at any age. The subtest reads the difference
off the table — a row whose `reading` is the zero time is seeded live.

The guard's tolerance is one minute, against this row's roughly two hours of
margin, so it skips conservatively: a wall jump between a minute and two hours
retires a run that would still have passed. That is the cheap direction, and
parameterizing the tolerance per call site would be machinery bought with
nothing measured. What matters is the other direction, and it holds — a real
staleness regression is not a wall jump, so it still reds (arm 3).

## Three arms, each run

- The guard is reached and fires, from the live row only: tolerance
  temporarily negative, `fresh reading` SKIPs with the measured jump printed
  (`gained 0s`, the ordinary case) and `no meter guard` passes untouched.
- The guard is not vacuous: the call site temporarily handed `ref.UTC()`, and
  the test fails with the refusal above rather than silently comparing one
  clock to itself.
- The assertion still catches what it is for, and the skip does not mask it:
  the `-3h` repro, run with the guard installed, FAILS naming
  `plan meter BLIND 3h00m` — it does not skip.

## Scope

Reading the two remaining out-of-process `cmd/posse` fixtures that state a
premise the same way — `runtimeavail_qa_test.go`'s `seedRuntimeCatalog` and
`backupsurface_qa_test.go`'s `backupHome`/`plantArchive` — is
ranger-base-w4enh's census and is not done here. Both are leads, not
findings. What this bead corrects is w4enh's scope sentence: `cmd/posse` was
not done, and the row above is the counterexample.
