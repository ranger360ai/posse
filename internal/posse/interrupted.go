package posse

// The interrupted-run scan (ranger-base-eh1kr): the half of the dispatch
// queue `bd ready` cannot answer.
//
// A pass reads its queue with `bd ready`, and bd's own help says what that
// query is: "Show ready work (open issues with no blockers). Excludes
// in_progress, blocked, deferred, and hooked issues." So an in_progress bead
// NEVER reaches the fire loop from the store, on any store class —
// MEASURED 2026-10-03, bd 0.50.3, both classes, the same argv: a SQLite
// store (a `beads.db` beside its `issues.jsonl`) answers `ready --json
// --limit 0` with open rows only, and a hand-written `no-db: true` JSONL
// store holding one open and one in_progress bead answers with the open one
// alone. ADR 0004 §2 says "today `bd ready` may include them; the cockpit
// filters"; that reading is retired, and this file is the other half.
//
// Which made every in_progress branch of the fire loop dead code as far as
// the store is concerned. The loop holds the whole recovery contract — the
// holder join (ADR 0004 §2), ADR 0008's crew shield, ADR 0030's
// orphaned-claim tiebreak, rangerhq-zom's stopped-on-purpose skip, and
// `--resume`'s override — and the only beads that ever reached it carrying
// `status: in_progress` came from a fixture: the suite's fake bd serves
// `fake-ready.json` whole, in_progress rows included, so every one of those
// branches was green against a queue real bd does not produce.
//
// What that cost (MEASURED 2026-10-03 16:0x, the bead's own evidence): two
// beads sat in_progress with no live session — ranger-base-4mrmc, whose
// holder `posse kill --no-land` retired at 14:5x, and ranger-base-mz8ud,
// whose seat was reaped at 08:5x while the bead was blocked — and dispatch
// said NOTHING about either. Not a relaunch, not a "held by", not a skip:
// the 16:00 pass hired four other beads including a P3 and `posse dispatch
// --dry-run -n 0 --resume` printed no line for either. ADR 0013's landing
// clause promises the opposite — "nothing is stranded by a keep … so
// closing the bead lands it on the next pass" — and the operator's
// workaround was `bd update --status open` by hand, which throws away the
// claim the work was done under.
//
// So the pass reads the claimed list too, and keeps the rows that are an
// INTERRUPTED RUN. The judgment itself stays in the fire loop, which is
// where every one of those guards lives: this scan decides only which
// claimed beads are worth offering it.
//
// Cost: one `bd list --status in_progress` per repo per scan, plus one
// `bd blocked` when that list is non-empty — 0.13-0.17s each against the
// 1551-bead queue db, the grain `Bd.Ready` already pays twice.

import "time"

// interruptedRuns is the claimed half of the queue: the in_progress beads of
// every scanned repo that no live AGENT is working, deduped against the
// ready half the caller already read.
//
// "No live agent" is the usage text's own definition of the population that
// resumes unattended — `--resume`'s help says "default: only interrupted
// runs resume — no live agent" — and it is read through the same holder join
// the fire loop walks (heldSession: the run record first, then the Dial F
// name, then the pre-Dial-F slot), so the scan and the loop cannot disagree
// about who holds a bead. Two readings pass it:
//
//   - no live session under any of the three names: the holder was killed,
//     reaped, or crashed with its workspace. The launch creates the Dial F
//     session and the claim resumes (Bd.Claim: "held by this actor already
//     — an interrupted run").
//   - a live session with NO AGENT in it: the persona's CLI exited and left
//     a bare shell. ADR 0004 §3's "re-prompt the holder, or launch it if
//     gone" relaunches in place, and the loop's own retarget (`if holder !=
//     "" { session = holder }`) is what keeps it from building a twin
//     beside it (ranger-base-6bu).
//
// And under `--resume` only, a third: a holder whose agent herdr calls
// settled (idle or done) — the flag's own population, "re-prompt
// in_progress beads whose persona session is alive and idle". The loop then
// decides whether to type into it (the waiting and ghost-box skips,
// ranger-base-htafy / ranger-base-6o7wm) exactly as it does for a bead the
// store handed it.
//
// A holder herdr calls WORKING is not offered, under either flag: the
// persona is in the middle of the turn, the cockpit's IN PROGRESS section is
// where that bead is read (ADR 0004 §2), and a line per pass per in-flight
// bead would bury the lines that mean something. A SETTLED holder on a pass
// with no `--resume` is the same judgment one rung over, and it is why the
// settled arm is gated on the flag rather than offered always: the
// governance surface's G2 row (`settled:<bead>`, govern.go) is the surface
// that reports a holder which stopped without closing, and a per-pass skip
// line beside it would say the same thing again every pass. What the flag then opts into — one re-prompt of a
// settle-open bead per settle under `--watch` — is rangerhq-zom's own
// accepted cost, in the clause that made the default a skip; see
// docs/notes.d/ranger-base-eh1kr.md, "The accepted cost, named".
//
// Three more rows are dropped, and none of them is an interrupted run:
//
//   - BLOCKED by an unmet dependency, which this subtracts exactly as
//     Bd.Ready does and for the same reason (ranger-base-lpz0o: `bd ready`
//     is not the definition of unblocked on every store class, and
//     `bd blocked` lists in_progress rows — the live queue has one as this
//     was written). Dispatch does not hand a persona blocked work; mz8ud's
//     own dep closed at 15:0x, and from that moment this scan offers it.
//   - DEFERRED past now: a defer is an answer somebody already gave, and
//     the date is the answer (ranger-base-5aln). `bd ready` excludes
//     deferred rows and so does this.
//   - a claim whose assignee is not a lane of ONE at its own holder — no
//     assignee, an assignee that loads no PID, or the coordinator (ADR 0033
//     §2). laneFor is asked rather than CanonAgent so the rule is routing's
//     own: an in_progress bead is claimed work, and re-laning it by LABEL
//     would offer another persona a bead somebody else holds — the claim
//     would lose (ClaimLostError) after a seat and a session had already
//     been spent on it, and a bead the operator claimed by hand would be
//     routed to whoever owns its label.
//
// Failures are this scan's own to report and never the pass's to die on: a
// repo whose claimed list cannot be read has unknown interrupted work, not
// none, and the ready half of its queue is still good (rangerhq-llse's
// reading, one query over).
func (d *Dispatcher) interruptedRuns(dirFilter string, already []RepoIssue) []RepoIssue {
	dirs := d.App.BeadsDirs()
	if dirFilter != "" {
		dirs = []string{dirFilter}
	}
	seen := make(map[string]bool, len(already))
	for _, is := range already {
		seen[is.Dir+"\x00"+is.ID] = true
	}
	var out []RepoIssue
	for _, dir := range dirs {
		held, err := d.Bd.InProgress(dir)
		if err != nil {
			d.printf("✗ claimed scan failed in %s: %v — interrupted runs there are unknown this pass, not absent\n", AbbrevHome(dir), err)
			continue
		}
		if len(held) == 0 {
			continue
		}
		blocked, err := d.Bd.Blocked(dir)
		if err != nil {
			d.printf("✗ claimed scan failed in %s: %v — interrupted runs there are unknown this pass, not absent\n", AbbrevHome(dir), err)
			continue
		}
		stuck := make(map[string]bool, len(blocked))
		for _, r := range blocked {
			stuck[r.ID] = true
		}
		for _, is := range held {
			rep := RepoIssue{BdIssue: is, Dir: dir}
			if seen[dir+"\x00"+is.ID] || stuck[is.ID] || deferredNow(is, d.now()) {
				continue
			}
			persona, ok := d.heldLane(rep)
			if !ok {
				continue
			}
			if !d.interruptedRun(rep, persona) {
				continue
			}
			out = append(out, rep)
		}
	}
	return out
}

// heldLane is "this claimed bead's lane is its own holder, and that holder is
// a persona" — laneFor's assignee arm and nothing else (see
// interruptedRuns).
//
// The canon name must equal the assignee as WRITTEN, not merely resolve to
// it. CanonAgent folds case (rangerhq-c6u6: one persona, one identity), so
// a bead assigned `Ranger` routes to `ranger` — and every in_progress
// branch of the fire loop is gated on `is.Assignee == persona` as a string,
// so a folded name would reach the loop with its holder join, its crew
// shield and ADR 0030's tiebreak all abstaining, and be claimed afresh
// under the other spelling. That is a re-attribution, not a resume. The
// fire loop has always compared the two spellings this way for a READY
// assigned bead; what this clause does is keep the scan from manufacturing
// the case.
func (d *Dispatcher) heldLane(is RepoIssue) (string, bool) {
	if is.Assignee == "" {
		return "", false
	}
	lane := d.laneFor(is)
	if lane.deny != "" || lane.why != "assignee" || len(lane.seats) != 1 {
		return "", false
	}
	if lane.seats[0].name != is.Assignee {
		return "", false
	}
	return lane.seats[0].name, true
}

// interruptedRun is the holder reading, through the fire loop's own join.
func (d *Dispatcher) interruptedRun(is RepoIssue, persona string) bool {
	var runHolder *HerdrSession
	if s, ok := d.HB.RunHolder(is.Dir, persona, is.ID); ok {
		runHolder = s
	}
	holder, status := d.heldSession(runHolder,
		SessionForBead(persona, is.Dir, is.ID),
		SessionFor(persona, is.Dir))
	switch {
	case holder == "", status == "":
		return true // no session, or a session with no agent left in it
	case d.Resume && settledStatus(status):
		return true // the flag's own population
	}
	return false
}

// deferredNow reads DeferUntil and never the status string, on
// ranger-base-5aln's measurement: on bd 0.50.3 a deferred bead reads back
// with a date and whatever status it had.
func deferredNow(is BdIssue, now time.Time) bool {
	return is.DeferUntil != nil && is.DeferUntil.After(now)
}
