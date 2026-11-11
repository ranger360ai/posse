## The dispatch path holds a Seat now, not a pane (ranger-base-k0hd6)

2026-11-10 — the debt ranger-base-nfc2b left standing, paid.

`HerdrBackend.AgentTarget` resolved a session's workspace, read the pane out
of it, and returned the pane alone. Everything downstream was a `target
string`, and every verb it addressed went through a one-line helper —
`paneSeat(pane) Seat { return Seat{Pane: pane} }` — that threw the session
half away again at the call site. Thirty-one call sites across twelve files,
every one of them on the live dispatch path.

### What the pane-only Seat cost, and what it did not

Nothing, on herdr. A herdr pane id addresses a pane anywhere on the server,
so `Seat{Pane: p}` is a complete address there and the helper was free.

On Rex it is not. Every block method is under `session.*` — there is no
global block lookup to reach for (`rex api ls`, MEASURED under
ranger-base-nfc2b) — so an occupant handed a Seat with no session resolves
one out of the server's own listing before it can address the block at all.
That resolution is a `session ls` plus a `block ls` per session, MEASURED at
**14 + 16N ms** for N sessions on the server: 62ms over this box's three,
~340ms over twenty (ranger-base-nfc2b, 2026-11-10, this host, rex CLI
0.0.1794069975-g6c6689).

The loop that made it matter is `AwaitPersonaMode`: it reads the pane every
250ms for the length of a launch's startup wait. A per-read resolution puts
that loop's real interval above the one it asks for, and the number grows
with how busy the box is — which is the one thing a poll interval must not
depend on.

### The fix is the resolution, moved up one level

`AgentTarget` returns `Seat{Session, Pane}`, paired from the occupant's own
`List()` row — the one place both ids are already in hand together. The
`target` that then runs through `awaitTarget`, `awaitDelivered`,
`awaitSettled`, `AwaitPromptable`, `PaneHolding`, `ConfirmSubmitted`,
`AwaitPersonaMode`, `blockerEvidence`, `withPaneCapture(Named)`,
`logUnrecognized`, `logBlockedRefusal`, `logPromptHandBack`, `d3Evidence`,
`gateReading` and the `target` fields of `pendingBead` and `launched` is that
Seat. `paneSeat` is gone from `internal/posse` and `cmd/posse`.

Three other callers were already holding both halves and spelling only one:
`fillPaneModes` (`HerdrSession.Seat()`), `sessionHolding` (`HerdrMeta.Seat()`)
and `pulseTarget` (`HerdrSession.Seat()`). The probe's own addressing came
from `awaitProbeAgent`, which knew the workspace it was asking about and
returned the pane; it returns the Seat.

### What did NOT change, and must not

**A half Seat is still legal, and the occupant still answers for one.** ADR
0076's `Seat` doc licenses a caller to build a Seat carrying only the half it
holds, and ranger-base-nfc2b made that correct on Rex by having the occupant
resolve the missing half. That license is not withdrawn — a caller outside
posse's own plumbing may hold one, and the rex pins that assert the occupant
answers for it are untouched. What changed is that posse's dispatch path no
longer *relies* on it: the resolution is paid once, where the pairing is
free, instead of per verb.

So this is not a fix for a bug. It is the reason the next occupant admitted
to the multiplexer slot does not have to answer "what does half a Seat
mean?" before it can be dispatched to.

### Where the pins are

- `TestHerdrAgentTarget` (herdr_test.go) — AgentTarget returns both halves.
- `TestQATheDeliveredWaitConcedesNamingTheRuleAndNotHerdr`
  (muxnoverb_qa_test.go, arm 1) — `awaitDelivered` hands back the whole Seat,
  asserted against a real Rex occupant.
- `TestQAD3TheDeliveredHoldRecordCarriesTheScreen` (d3capture_qa_test.go,
  arm 3) — the same, at the site the D3 capture is taken from.
- `TestQAPersonaModeFooterRefusesAPaneThatFellBack`
  (personamode_qa_test.go, arm 1) — the gate hands the occupant the seat it
  was given, both halves, on every read of the loop. `fakePane` records the
  seats it is handed, which is the only way a fake can pin this.
- `TestPulseTargetEmptyPersonaMatchesNothing` (pulse_delivery_test.go) — the
  pulse's target is the session's whole Seat.

`fakeHerdrSeat` (herdr_test.go) is the fixture seat the rewritten pins pass:
workspace `w1`, pane `w1:p1`, which is what every fake-herdr fixture in the
package seeds.

### How it landed: a replay, not a rebase (ranger-base-pqahc)

The branch was cut at `8f4965e6`, and `ranger-base-nfc2b` landed as
`f6db41f7` while this work was in the tree — so the merge-back was refused
and `ranger-base-pqahc` replayed the one commit onto current `main` in a
fresh worktree. No rebase and no cherry-pick: this seat's PID denies
touching history, sealing a merge needs the unqualified `git commit` the
same PID denies, and a sequencer left set in a session worktree kills the
NEXT path-limited commit with `fatal: cannot do a partial commit during a
cherry-pick` (AGENTS.md, "Landing the plane"). `git merge-tree
--write-tree --merge-base=<the original's own parent>` writes the merged
tree into the object store with no index state at all, and one
`cat-file blob` per path puts it in the tree.

Three conflicts, all predicted on the bead before the replay ran:

- `docs/notes.d/README.md` — generated, so regenerated with
  `python3 scripts/notes-index.py` rather than hand-merged. Both entries
  are in it.
- `internal/posse/herdrback.go`, `fillPaneModes`' default arm — both sides
  moved the same line to `b.Mux().Read(s.Seat(), …)`; nfc2b's new comment
  above it named `paneSeat`, which this bead deletes, so the comment says
  "a pane-only Seat" instead.
- `internal/posse/mux.go`, the `Seat` doc — nfc2b's text kept whole, less
  the parenthesis naming dispatch as the caller that relies on half a
  Seat, which this bead makes false. The paragraph that replaces it sits
  between that one and "A PANE-ONLY SEAT IS THE OCCUPANT'S TO RESOLVE",
  whose "the price of the one above" became "the price of the **licence**
  above": the insertion displaces the referent, and the licence — not the
  new paragraph — is what that price is for.

One commit in, one commit out, carrying the original's author email,
AUTHOR date and subject, because that triple is the only thing left that
pairs a replay with the commit it stands in for (`replayKey`,
`equivalentOnBase`; docs/notes.d/ranger-base-4r8fw.md).
