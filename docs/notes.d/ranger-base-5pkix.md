# Four code findings from ranger-base-j7ekm's verify (ranger-base-5pkix)

ranger-base-j7ekm graded four closes (ranger-base-c75wa, ranger-base-zlvyk,
ranger-base-y1mgf, ranger-base-jv2q3) and filed six live findings. Every
close's deliverable works; these are the leftovers. Four of them are `-l code`
and landed here. The two `-l devops` ones — the park procedure's two records
prescribing different census bases, and the carrier census's unwitnessed
`[checked out at <path>]` annotation — went to that lane as ranger-base-3bhxb.

Each fix is a pin or an arm, so each one is recorded below as the plant that
was GREEN before it and FAILs after. Every plant was restored and
`git status --porcelain` was read back empty after each battery.

## F1 — the park namespace census read 64 of 67 tracked programs

`internal/treepins/parknamespace_qa_test.go`, arm 1, censuses "every park ref
spelling in a tracked PROGRAM" and admitted a file by `*.sh` or `*.py` alone.
This repository keeps three EXTENSIONLESS bash programs under `scripts/`:
`posse-land-check`, `posse-landed-check`, `posse-wall-check` — the landing
gate, the post-landing judge and the wall check, which is exactly where a park
check in the landing chain would be typed. A park spelled
`refs/parked/<name>` in any of them was invisible to the pin, to the carrier
census and to ranger-base-3s2ig's guard, and the landing those exist to refuse
went through with nothing red: the failure the pin was written to end.

The fix is the house shape, already in the same package —
`trapstatepick_qa_test.go`'s `tsIsShell` admits a program by extension OR by a
`#!` first line, "the way the box's own roster reads it", and 13 of the 15
treepins pins that walk `scripts/` read a shebang. `pnsIsProgram` is that
test; the walk reads the file first and decides afterwards.

MEASURED 2026-11-22, one planted line per file, the three park arms run after
each and the file restored. The five rows are ranger-base-j7ekm's own table:

| plant | before | after |
|---|---|---|
| `scripts/park-carriers.sh` + `planted_wrong_ref="refs/parked/oe884"` | arm 1 FAILS (control) | arm 1 FAILS |
| `internal/posse/landsweep.go` + the same in a Go literal | arm 1 FAILS (control) | arm 1 FAILS |
| `scripts/posse-land-check` + the same | all 3 GREEN | **arm 1 FAILS** |
| `scripts/posse-landed-check` + the same | all 3 GREEN | **arm 1 FAILS** |
| `scripts/posse-wall-check` + the same | all 3 GREEN | **arm 1 FAILS** |

The census reads 67 scripts now against 64, with one file under `scripts/`
(the `.allow` roster, whose lines are English) correctly left out.

Two narrowings are doored, because the finding was the census seeing less than
it reported and neither direction reds on its own:

- the shebang arm dropped from `pnsIsProgram` — arm 3's admission control reds,
  naming the programs it can no longer see;
- an extension test reinstated in the WALK with `pnsIsProgram` untouched — arm
  1 reds, because the files the walk passed over are read again and held to
  `pnsIsProgram`.

Both MEASURED RED 2026-11-22.

## F2 — a wrong spelling split across two Go string literals

The AST walk judged each `*ast.BasicLit` alone, and the short-ref arm accepts
a bare `parked/` component whatever precedes it, so a ref written
concatenated was judged as neither half. MEASURED 2026-11-22:
`internal/posse/landsweep.go` plus `var x = "refs/posse/" + "parked/oe884"`
left all three arms GREEN, where the same spelling in ONE literal reds arm 1.

`pnsConcat` renders a pure `+` expression as the text its literals spell and
the walk judges that as one chunk, not descending into the leaves — so nothing
is counted twice. A MIXED expression (`base + "parked/x"`) falls through to the
per-literal walk and the short form is still accepted there: a namespace
computed at runtime is not a spelling this census can read off the source at
all, and `pnsConcat`'s comment says so out loud with a control row to witness
it.

After: the split plant reads `internal/posse/landsweep.go:751 (Go string
concatenation) spells a park ref "refs/posse/parked/oe884"`, and the
one-literal control still reads `(Go string literal)`.

The control for this one drives `pnsJudgeGoSource` — the walk's own judging,
split out of the tree walk for the purpose — and not `pnsConcat`. A control
over the helper alone stays green when the walk stops calling it, which is
what F2 WAS: the matcher was never the broken half. MEASURED 2026-11-22 with
the walk's concatenation case removed: arm 3 RED on three rows.

## F5 — the wall check's non-member ceiling was held to a number, not a roster

`internal/treepins/wallcheckselftest_qa_test.go`'s arm 6 held the body's
`nonmember_ceiling` to the same number in ADR 0097 and not to the ROSTER that
number is the size of. So the one move the arm exists to catch passed it:
MEASURED 2026-11-22, `nonmember_ceiling=3` -> `4` in
`scripts/posse-wall-check` AND in the ADR, with the roster still naming three
— arm 6 GREEN, the floor lowered by one pin, every door in the tree green.

The arm counts the roster now: the amendment's non-member list must name at
least `ceiling` tests, so a bump has to say WHICH test it is making room for.
Scoped to that paragraph and not to the document, because the ADR names a live
pin in a bullet elsewhere (D2's false-pass row) and a whole-document count
would have read it as a fourth non-member — the same bump would then have
passed a four-name count as well. The anchor is the half of the paragraph's
opening sentence that does not carry the count, since a bump is expected to
rewrite "The three".

MEASURED 2026-11-22, five plants:

| plant | before | after |
|---|---|---|
| both sides 3 -> 4, roster at three | GREEN | **RED** |
| body only | RED | RED |
| ADR only | RED | RED |
| both sides AND a fourth name in the roster | — | GREEN |
| the roster paragraph's anchor removed | — | **RED** |

## F6 — nothing drove the dark arm's placement

`scripts/posse-wall-check`'s floor arm sits AFTER the member arms on purpose,
and the comment at its site says why: a member that FAILed is a red whatever
else the run did, and a member that abstained is the diagnosis a reader can
act on. No `--self-test` arm drove that placement. MEASURED 2026-11-22: the
whole `if len(dark) > ceiling:` block moved above `if skipped or quiet:` left
`--self-test` at 12 arms clean, because no planted stream carried both an
abstaining member and a dark pin at once. Both arms are exit 3, so the cost is
a worse diagnosis and not a wrong verdict — which is why the arm asserts the
SENTENCE and not the status.

The thirteenth arm is that stream. It is built from the `$dark` stream the arm
above it drives, plus one abstaining member and nothing else, so a later edit
that weakens the dark half reds that arm first rather than silently retiring
this one. MEASURED 2026-11-22 with the block moved: `1 of 13 arm(s) FAILED`,
naming both missing lines, and `TestQATheWallCheckSelfTestStillReadsTheThreeExits`
red over it.

Arm 5's mutant count moved with it — it asserts the shape of
ranger-base-h5iae's two-exit mutant and reads the number of `st_arm` calls
wanting 3, which is six now.

## What was NOT touched

- F3 and F4, which are `-l devops` by ranger-base-j7ekm's own per-line
  assignment: the base the park census runs with (one line, and which side
  moves is that lane's call) and a third `--self-test` arm for the carrier
  census's worktree annotation. Filed as ranger-base-3bhxb.
- ADR 0097's prose. F5's fix reads the amendment as it stands; the roster
  paragraph was not rewritten, so the record still says what it said when
  ranger-base-c75wa measured it.
