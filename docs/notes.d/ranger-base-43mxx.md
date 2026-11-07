# A proven-dead Rex seat is destroyed, not only forgotten (ranger-base-43mxx)

2026-11-06. ADR 0080 D10's build. On a non-herdr occupant, `reclaim` now
calls `Close` on the recorded session id under the launcher lock, after the
re-proof and before the `os.Remove`. A destroy that fails keeps the record
and the next pass retries; the server saying it does not know the id counts
as already destroyed. The herdr path is unchanged, to the byte.

The leak it closes is one nobody can see from either end. posse's listing is
clean — the record is gone, no session is withheld, no warning prints. Rex's
listing is clean too — the session is there, like any other. The only place
the two disagree is the server's session count, and that is the number every
home on the server is spending.

## 1. What the husk is, and what it costs

A Rex seat ends by its block closing: posse opens a block with a command and
no `--keep-open`, so when the program and the shell exit the block closes and
the session stays on the server holding no terminal block
(MEASURED 2026-10-31, ADR 0076's §5 correction). `RexMux.List()` drops a
session with no terminal block, which is exactly what makes §5's liveness
rule fire at all — and then ADR 0011 §2's prune proves death from that
absence and unlinks the meta. The session survives both.

MEASURED 2026-11-06, darwin/arm64, rex CLI 0.0.1793377904, against a
**scratch** server (`server run --listen <a private socket> --restore=false
--state-dir <scratch> --max-sessions 3`, with `HOME` pointed at a scratch
dir so the server's own startup lock stayed out of the operator's config;
the operator's live server was never addressed — every call passed
`-S <that socket> --autostart=false`):

| probe | answer |
|---|---|
| `new husk1 --cwd /tmp -- /bin/sh -c 'exit 0'` | created; `ls --json` lists the session, `block ls -s <it>` lists **zero** blocks |
| three such sessions, then a fourth `new` | `409 conflict`: "server already has its maximum of 3 sessions; destroy one before creating another" |
| `kill <one husk>` | exit 0; the session leaves `ls`, and the next `new` is accepted |
| `kill <the same id again>` | exit 3, stderr `Error: session target "session:01a1b3ee-…" not found` |
| `kill 00000000-0000-0000-0000-000000000000` | exit 3, the same sentence |
| `kill notasession` | exit 3, the same sentence |

So three facts, each of which the build needs: a husk is real and holds a
`--max-sessions` slot (default 64, shared by every home — ADR 0080 D2); a
destroy frees that slot; and an id the server does not hold is refused with a
sentence, on stderr, with exit 3 and nothing on stdout, identically for an
unknown id, a non-id and an id destroyed a moment earlier.

## 2. The refusal is read by TEXT, not by exit code

rex reports failure as plain text with a nonzero exit — there is no envelope
and no code to branch on (rex.go's head comment, item 2). Exit 3 is the same
code a missing BLOCK answers with (`Error: block target "…" not found`,
MEASURED 2026-10-31), so the exit says "the call failed" and only the
sentence says what about it. `rexSessionGone` matches the two halves the id
is interpolated between — `session target "` and `" not found` — and nothing
else: another refusal, a binary that is not there, an answer that would not
parse, and a blown deadline are all "not this sentence", and the caller keeps
the record on every one of them.

The fake rex had to be fixed before any of this was testable: `fakeRexKill`
answered exit 0 for every id, including ids the fake did not hold, so the
one branch that reads a refusal had no way to be reached from the suite. It
now answers the measured sentence with exit 3 for an id it does not hold.

## 3. Why the reading is a second question and not an idempotent `Close`

The obvious spelling is to let `RexMux.Close` swallow `not found` and call
the seat ended. It is wrong, and the reason is the other two callers. `posse
kill` and a relaunch's `closeRecorded` both reach `Close` holding a seat
their own listing has just answered for, and both unlink the record on a
successful close. A `Close` that swallowed `not found` would read a
MISAIMED destroy — against an id this home never held — as a seat
successfully ended, and the record would go with it.

So `Close` keeps its contract ("ends the seat") and the reading is a separate
question, asked only where the absence has already been proven: an optional
interface, `MuxCloseGone`, reached by a type assertion the way ADR 0076 §4's
herdr-only verbs are. A failed assertion is "not available" and never "no" —
an occupant that cannot read its own refusal keeps the record, which on a
destroy is the recoverable direction.

## 4. The order, and why it is the whole of the fix

`destroy` then `unlink`, both under ADR 0011 §1's launcher lock, with the
proof re-taken inside it:

- The record is the only thing that names the session id. A pass that
  unlinks first has nothing left to destroy with — and the leak then has no
  record of itself anywhere, on either side.
- A destroy that FAILS keeps the record. Unlinking after a destroy that did
  not land is the leak made permanent, and a kept record is what the next
  pass retries from. It costs one file and one warning line that names the
  seat, the server's refusal and D10.
- The destroy is inside the lock because reclaim's whole shape is that the
  act is taken under the lock with the evidence re-read there. A destroy
  outside it would act on a listing taken before it.

## 5. What this is not

It is not the reaper ADR 0076's §5 correction rejected. That reaper
destroyed a session when its blocks exited, so that `session.list` would
mean liveness — a second writer to every session's lifetime, racing `List()`
in the window between the exit and the destroy, with immortal seats coming
back whenever it crashed. This destroy runs *after* liveness has been proven
by the occupant's own `List()`, by the one home whose meta names the id. A
server-wide husk sweep is rejected for the same reason D10 rejects it:
ownership is the meta, so only the owner destroys.

And herdr needs nothing here. A dead herdr workspace is gone, so there is no
husk — and herdr re-issues workspace ids across a server restart or handoff
(rangerhq-6bg7), so the id in a proven-dead herdr record is precisely the one
a stranger's workspace may be holding now. `workspace close` on it would
close that stranger. The gate is the record's own `MuxName()`, and by the
time `destroyRecordedSeat` is reached `cannotAnswerFor` has already refused
every record whose mux or server is not this pass's, so the record and the
occupant agree.

## 6. One hole left open, filed and not fixed

`reclaim` is not the only line that unlinks a record on proven death.
`clearDeadMeta` (relaunch.go) does it too, under the same lock and through
the same predicate (`mustNotOrphan` -> `idEvidence` -> `seatEvidence`), on
the relaunch path where the occupant's listing does not hold the recorded
seat. On a Rex instance that unlinks the record of a husk and leaves the
husk, which is this same leak one caller over. It is NOT fixed here, because
what a failed destroy should cost a *relaunch* is a decision D10 does not
make — reclaim can keep a file and move on, where a relaunch would have to
refuse or proceed — and that is a deliverable of its own. Filed as
ranger-base-qywy1.

## 7. The pins

`internal/posse/reclaimdestroy_qa_test.go`, arm 1 of the suite. Every arm is
about an order or a silence, because nothing in this bug is visible in a
green run:

1. `TestQAAProvenDeadNonHerdrSeatIsDestroyedBeforeItsRecordIsUnlinked` — the
   witness is taken INSIDE the destroy, because the ordering claim is about
   an instant: both orders end with the record gone and the seat closed. The
   stub's `Close` stats the meta path (it is still there) and tries
   `tryLockLaunches` (it cannot take it — flock is per open file
   description, so a second open from this very process conflicts with the
   one reclaim holds).
2. `TestQAAFailedDestroyKeepsTheRecordAndRetries` — the record survives, the
   warning names the seat and the server's own words, a second pass attempts
   the destroy again, and then the failing arm is removed and the record IS
   unlinked. Without that last control every assertion above is satisfied by
   a prune that stopped pruning.
3. `TestQAARexRefusalThatTheServerDoesNotKnowTheIdCountsAsDestroyed` —
   end-to-end over `RexMux` and the fake rex, two rows: the fake holds no
   such session and answers the measured sentence (record unlinked), and the
   `rex-fail` lever makes `kill` answer a DIFFERENT refusal with the same
   exit code and the same shape (record kept).
   `TestQAARexSessionGoneReadsOnlyRexsOwnNotFound` pins the sentence itself,
   including the neighbouring `block target "…" not found`, which says
   nothing about a session. None of those rows pins the FAKE's own answer
   for an unknown id, and the mutation run is what said so — each row ends
   with the record unlinked whether the kill was refused or succeeded. That
   is `TestQAATheFakeRexRefusesAKillOfAnIdItDoesNotHold`, which also walks
   the refusal through `MuxCloseGone` as the interface rather than as the
   method, since the prune's assertion is on the interface.
4. `TestQAAThePruneNeverDestroysAHerdrWorkspace` — over the real herdr
   occupant, reading its calls log, with a control that the record really
   was pruned. A stub occupant cannot reach this arm at all: herdr's half of
   the prune is its own per-id query behind §4's door, so a stub would
   refuse for that reason and pass a "no close" assertion while measuring
   nothing. That is the first shape this test was written in, and it was red
   for exactly that reason.

## 8. Every pass is asked again, over a lock flake that is not this bead

Each arm drives listing passes through `pruneUntil` rather than one
`b.Sessions()`, because reclaim's answer to a contended launcher lock is to
spare the file and let the next quiet pass prune (ADR 0011 §1) — and this
package's own concurrency produces that contention. MEASURED 2026-11-06: a
throwaway test doing nothing but `tryLockLaunches` / `Release` five times
over its own `newTestBackend` home, with nothing of this bead in it, failed
with `a launcher is running` under `-count=5` beside four other tests. flock
is per open file description and a child between fork and exec still holds a
reference to the one the parent just closed, and this package forks its
fakes constantly. In production that reads as one spared file and the
documented "the next quiet pass prunes it", which is why it is a note and
not a bead.

## 9. The mutation table

MUTATION-CHECKED 2026-11-06, each mutant applied to a clean tree and the
tree asserted clean again after. Pin names abbreviated.

| mutant | reds |
|---|---|
| the destroy moved AFTER the `os.Remove` | ordering, kept-on-failure, the rex rows |
| the destroy removed entirely | ordering, kept-on-failure, the rex rows |
| a failed destroy returns "" (proceeds to the unlink) | kept-on-failure, the rex rows |
| the `MuxName() == MuxHerdr` gate removed | herdr-untouched |
| `rexSessionGone` matches any error | the sentence table, the rex rows |
| `rexSessionGone` matches nothing | the sentence table, the rex rows |
| the destroy aimed at `Seat{Session:}` without the pane | ordering |
| the `MuxCloseGone` reading ignored | the rex rows |
| `fakeRexKill` back to exit 0 for every id | the fake's own refusal |
| the `IsHerdrHang` guard dropped from `rexSessionGone` | **nothing — declared** |

The last one is a belt and is said to be one in both the predicate and the
test arm: a hang's text is its argv and its wait, so the two substrings
refuse it with no help from the guard. It is worn because every predicate of
this shape in the package wears it, and because the distinction it names is
the one a rewrite of this predicate — keying on the exit code, say, where a
killed child and a refusal are the same nonzero — would get wrong.
