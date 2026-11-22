# ranger-base-8ck6d: the last four member lines that deferred their cause

`scripts/posse-wall-check` reports a failed member by its first
`file.go:NN:` line and nothing after it — `evidence()`, 300 characters,
ADR 0097 D3 — and the launcher puts that one line on a P1 bead. So a member's
first failure line is its whole budget, and a line that names a symptom and
leaves the reason in a captured buffer reaches the bead empty. That is
ranger-base-a688i's class. ranger-base-w6kke's census found 28 failure
messages in the member set carrying `:\n%s`, carved out 22 whose first line is
already the whole claim, folded 2, and filed the remaining 4 here because they
are not the same shape as a watcher's two lines.

## The four rows, and what each one got

One in `wallclaudeprojects_qa_test.go` and three in `wallunixlive_test.go`,
all of them reading their seat's output inline in a test body.

- `TestLiveWallClaudeProjectsRefusesPeerPlants`, the 90s deadline on the real
  claude seat. A HANG, so there is no exit status to read at all and the pane
  is the only account of how far the seat got. It carries
  `wwOneLine(string(out))` on the first line now.
- `TestLiveWallUnixRefusesAHostSocketBoundAfterTheSeatLaunched`,
  `TestLiveWallUnixScopesSignalsToTheSeat` and
  `TestLiveWallUnixLeavesCrossDirectoryRename`, all three
  `the seat exited badly (%v). Seat output:\n%s\nargv: bwrap %s`. They carry
  `bgSeatAccount(err, out)` — the status, then the seat's words — with `argv`
  left on line 2.

## Asking per row whether the status already discriminates

That is the question the bead was filed with, because these three were not
a688i's shape: their first line already carried the seat's exit status in
`(%v)`, so a bead got a status rather than nothing. The answer for all four is
that it does not discriminate, and it is a measurement rather than a reading
of the prose.

MEASURED 2026-11-22 (linux 7.2.15, bwrap 0.13.1, go1.27.3, 16 cores). The
status at these three rows is `exit status 1` for at least four different
causes: a bwrap that refuses its own setup (`bwrap: Can't find source path …`
→ 1), an unknown bwrap option (→ 1), a seat whose own test fails (the seat is
this test binary re-exec'd, so a failed `go test` → 1), and the fake-bd shape
`wuSeatEnv`'s comment records (`fake bd: unhandled -test.run=…` → 1, which
happened on that pin's first run in both arms). The claude row is worse than
non-injective: THIS process does the killing one line above the fatal, so
`cerr` reads `signal: killed` whatever happened and the status names the
deadline rather than the seat.

So: a status is an account only when it is injective
(docs/notes.d/ranger-base-w6kke.md), and at four causes to one number it is
not. No row was left alone.

## MEASURED 2026-11-22 — the mutants

Six plants, one at a time, by a scratchpad script; md5 taken on both sides of
every plant and equal after each revert (the revert is a `cp` from a copy the
script takes, never `git checkout --`: the edits under test were uncommitted).
Each reading is the first `*_test.go:NN:` line of the run, which is the line
`evidence()` carries:

- row 1, bwrap setup refusal: `the seat exited badly: exit status 1, and the
  seat said "bwrap: Can't find source path /nonexistent-ajkq: No such file or
  directory"`, and in the walled arm `… the seat said "posse: wall-unix:
  cannot resolve the runtime \"--ro-bind\" on this seat's PATH … — so this is
  a seat that will not open, rather than one that opened unwalled"`. Two arms,
  two different causes, both named.
- row 1, the seat's own test failing: `… exit status 1, and the seat said
  "=== RUN TestWallUnixSeatChild wallunixlive_test.go:237: SEAT-RESULT
  connected but this mutant says the seat failed --- FAIL:
  TestWallUnixSeatChild (0.30s) FAIL"`. The SAME status as the plant above and
  only the words tell them apart, which is this bead's whole claim.
- rows 2 and 3, the same setup refusal: the same two lines at
  `wallunixlive_test.go:455` and `:1055`.
- the claude row, a seat that spoke and then hung (`/bin/sh -c 'echo …; sleep
  10'` at a 3s deadline): `claude -p under the wall did not end in 90s; it
  said "the seat got as far as this"`.
- the claude row's silence case, a seat that starts and hangs without speaking
  (`sleep 10` at a 3s deadline): `… it said NOTHING in 90s — so it hung before
  its first word, and the deadline is the whole finding`.

Every one of those first lines is inside `evidence()`'s 300 characters, prefix
included (155-265). Longer seat words will be truncated; what the fold buys is
that the cause is at the FRONT of what survives.

The silence case is read through a hanging seat and NOT through a `0 *
time.Second` deadline, which is the mutant that branch invites. At 0s the
deadline fires before `cmd.CombinedOutput` in the goroutine has reached
`cmd.Start`, so `cmd.Process` is nil and the pre-existing `Kill` panics with
SIGSEGV before any message is formatted. That is filed as ranger-base-mhl7h
(P3, `-l code`, `discovered-from` confirmed by `bd dep list`) and deliberately
not fixed here: it matters because a panic takes the whole test binary down,
so the wall check gets no member line at all rather than one member's finding,
and the fix is a choice between guarding the Kill and moving to
`exec.CommandContext`.

## Why the fold is spelled at the call site and not behind a helper

The three `wallunixlive` rows are byte-identical, so a file-local
`wuSeatExitedBadly(t, err, out, argv)` is the obvious way to keep one
spelling. It would RED the landed census.
`internal/treepins/wallmemberaccount_qa_test.go`'s arm 1 grades, for every
function in a member file that launches a bwrap seat, whether the name bound
to its waiter's error is handed to something that SHOWS it — and the callees
it accepts are `wmaSayers`: the testing format verbs and `bgSeatAccount`. A
helper of this file's own is in neither set, so moving the fatal out of the
launching function would read as an error rendered nowhere, which is
ranger-base-axt1f's hole exactly. The census is the reason for the shape here,
so the comment at the first row says so.

## What still pins none of this

Nothing. Arm 1 of that census grades that the seat's error REACHES a reader,
which the old `(%v)` form also did, so it cannot see the fold; arm 2 reads
only a local bound to a seat runner's last string result, and all four of
these rows read their seat's output inline in a test body. That is
ranger-base-s9h52's decision with its own numbers — the wide rule reads 204 of
785 calls, the sharpest prose rule 44 — and this page is not a request to
re-open it: these four were prose judgements the census declined to automate,
which is why they were a bead. Same standing as ranger-base-w6kke's two
converted lines, and the mechanism to extend if anyone ever wants it is
`callsite_qa_test.go`'s `qcsFuncDecl`.
