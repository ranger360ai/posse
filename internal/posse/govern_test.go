package posse

// The governance surface (ADR 0029 §1-2, bead rangerhq-81y0): the G-table
// computed live, and the three renderings of
// that one computation.
//
// Every test here injects the readings that would otherwise touch the
// operator's own machine — the ledger scan (G6) and the plan endpoint
// (G4/G5). A test that reads live state is red per HOUR rather than per
// commit, which is the class ranger-base-rp2y cost a day to.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

// writeBeadsDirs appends a `beads:` list to the app's config. Without it
// BeadsDirs falls back to the process cwd, which is a different test's repo.
func writeBeadsDirs(a *App, dirs []string) {
	var b strings.Builder
	if cur, err := os.ReadFile(a.ConfigPath); err == nil {
		b.Write(cur)
	}
	b.WriteString("beads:\n")
	for _, d := range dirs {
		fmt.Fprintf(&b, "  - %s\n", d)
	}
	os.WriteFile(a.ConfigPath, []byte(b.String()), 0o644)
}

func appendConfig(t *testing.T, a *App, lines string) {
	t.Helper()
	cur, _ := os.ReadFile(a.ConfigPath)
	if err := os.WriteFile(a.ConfigPath, append(cur, []byte(lines)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// shopKeys runs the check and fails on any unreadable store: a test that
// asserts on a PARTIAL set is asserting on an accident.
func shopKeys(t *testing.T, in GovInputs) []string {
	t.Helper()
	set, failed := ShopCheck(in)
	for _, err := range failed {
		t.Fatalf("shop check partial: %v", err)
	}
	return set.Keys()
}

func shopSet(t *testing.T, in GovInputs) GovSet {
	t.Helper()
	set, failed := ShopCheck(in)
	for _, err := range failed {
		t.Fatalf("shop check partial: %v", err)
	}
	return set
}

// govHasID asks whether the set holds a given G-row. Tests ask this; no
// rendering does — the cockpit's one by-name reader wants the ROW (it
// switches on G7's Key), which a bool cannot answer, so the method this
// replaces (GovSet.Has, ranger-base-zyou5) had no caller outside a test.
//
// A carry-over has no row name, so the empty id matches nothing.
func govHasID(s GovSet, id string) bool {
	if id == "" {
		return false
	}
	for _, c := range s {
		if c.ID == id {
			return true
		}
	}
	return false
}

// govRepo is a bd repo the fake bd serves from files: one directory, one
// `beads:` entry, whatever JSON the row under test needs.
//
// Every test here goes through it, including the ones with no bd row to
// assert. Without it BeadsDirs falls back to the process cwd and NewBd
// finds the real binary — so the check would run `bd ready` against
// internal/posse and read whatever the operator's own queue happens to say.
// That is the live-state class (ranger-base-rp2y): red per hour, not per
// commit.
func govRepo(t *testing.T, b *HerdrBackend) string {
	t.Helper()
	dir := t.TempDir()
	writeBeadsDirs(b.App, []string{dir})
	// On the backend's own field rather than RHQ_BD_BIN: the env var is
	// process-wide, so a helper every govern test calls held 53 of them
	// serial for a value the struct already carries (ranger-base-pj87l).
	// GovInputs takes it from b.Bd below, and the reap guard reads the same
	// field (herdrback.go), so nothing here resolves the ambient binary.
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	return dir
}

func writeJSON(t *testing.T, dir, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// sessionAgent gives a NAMED session's workspace a herdr agent in that
// state — setAgentStatus (pulse_delivery_test.go) takes a workspace id, and
// these fixtures know the session name they just created.
func sessionAgent(t *testing.T, fake, session, status string) {
	t.Helper()
	for _, w := range fakeLoadWSFrom(t, fake) {
		if w.Label == session {
			setAgentStatus(t, fake, w.WorkspaceID, status)
			return
		}
	}
	t.Fatalf("no workspace for session %q", session)
}

// seedPlanSnapshot writes the instance's shared plan snapshot as if a
// reading had succeeded at `at`. It is the store G5's blind clock reads —
// the reason a process that is not the watch loop can answer that row.
func seedPlanSnapshot(t *testing.T, a *App, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"at": at, "windows": fakeWindows(10, 10)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.StateDir, "plan-usage.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// spendReport is a ledger reading with no live transcript scan behind it.
func spendReport(now time.Time, usd float64) *CostReport {
	return &CostReport{Beads: []*Segment{{Bead: "spent", Start: now, CostUSD: usd}}}
}

// govIn is a one-shot process's inputs: everything live, no streak, no
// scans it did not ask for.
func govIn(t *testing.T, b *HerdrBackend) GovInputs {
	t.Helper()
	if !yamlHasKey(b.App.ConfigPath, "beads") {
		govRepo(t, b)
	}
	return GovInputs{App: b.App, HB: b, Bd: b.Bd, Now: func() time.Time { return govNow }}
}

// govNow is a fixed clock, so an age assertion is not a stopwatch race.
var govNow = time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

// govSuspended is a GovInputs.Suspended for one witnessed sleep: the box came
// back at `back` having been gone for `slept`, so the hole it left in wall
// time is [back-slept, back] and the answer for a window is the OVERLAP —
// which is the ledger's own arithmetic (suspend.go suspendSpan,
// suspendedSince), written here rather than driven through the real witness so
// a governance pin does not need a running loop to state a suspend.
func govSuspended(back time.Time, slept time.Duration) func(time.Time) time.Duration {
	return func(since time.Time) time.Duration {
		lo, hi := back.Add(-slept), back
		if since.After(lo) {
			lo = since
		}
		if hi.After(lo) {
			return hi.Sub(lo)
		}
		return 0
	}
}

func find(set GovSet, id string) *GovCondition {
	for i := range set {
		if set[i].ID == id {
			return &set[i]
		}
	}
	return nil
}

// ─── G2 · settled-but-holding ────────────────────────────────────────────────

// The zom skip made visible: dispatch declines to re-prompt a bead whose
// holder settled, and until this row nothing told anyone that it had.
func TestGovG2SettledHolder(t *testing.T) {
	b, fake := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
	sessionAgent(t, fake, "developer-x", "idle")
	writeJSON(t, dir, "fake-list.json", []map[string]any{
		{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
	})

	set := shopSet(t, govIn(t, b))
	g := find(set, "G2")
	if g == nil {
		t.Fatalf("no G2 for a settled holder: %v", set.Keys())
	}
	if g.Key != "settled:bd-1" || g.Class != GovLane {
		t.Errorf("G2 = %+v, want key settled:bd-1 class LANE", *g)
	}
}

// A working holder is not this row: the bead is being worked, and saying
// otherwise would make the surface cry at every busy session.
func TestGovG2WorkingHolderIsNotACondition(t *testing.T) {
	b, fake := newTestBackend(t)
	dir := govRepo(t, b)
	writePersona(t, b.App, "developer", "code")
	mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
	sessionAgent(t, fake, "developer-x", "working")
	writeJSON(t, dir, "fake-list.json", []map[string]any{
		{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
	})

	if g := find(shopSet(t, govIn(t, b)), "G2"); g != nil {
		t.Errorf("a working holder must not raise G2: %+v", *g)
	}
}

// No session at all is dispatch's to self-heal (the relaunch on the
// claim-held path), so it is not a governance condition either.
func TestGovG2NoSessionIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list.json", []map[string]any{
		{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
	})
	if g := find(shopSet(t, govIn(t, b)), "G2"); g != nil {
		t.Errorf("an interrupted run is not settled-but-holding: %+v", *g)
	}
}

// The subtype: `BLOCKED:` and `REFUSED:` are strings the harness itself
// injects into every work prompt, so reading the last comment back is
// mechanism. It changes the KEY, because a bead that refused at a risk line
// and one that merely stopped are different conditions to a human.
func TestGovG2SubtypedByLadderPrefix(t *testing.T) {
	for _, tc := range []struct{ text, key string }{
		{"BLOCKED: need the operator's call → bd-9", "settled-blocked:bd-1"},
		{"REFUSED: publishing — needs the operator", "settled-refused:bd-1"},
		{"just a note", "settled:bd-1"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			b, fake := newTestBackend(t)
			dir := govRepo(t, b)
			writePersona(t, b.App, "developer", "code")
			mustCreate(t, b, NewSessionOpts{Name: "developer-x", Agent: "developer", Dir: dir, Bead: "bd-1"})
			sessionAgent(t, fake, "developer-x", "idle")
			writeJSON(t, dir, "fake-list.json", []map[string]any{
				{"id": "bd-1", "status": "in_progress", "assignee": "developer", "title": "held"},
			})
			writeJSON(t, dir, "fake-comments.json", []map[string]any{
				{"id": 1, "text": "an earlier one"},
				{"id": 2, "text": tc.text},
			})
			g := find(shopSet(t, govIn(t, b)), "G2")
			if g == nil || g.Key != tc.key {
				t.Errorf("G2 = %+v, want key %s", g, tc.key)
			}
		})
	}
}

// ─── G3 · an unanswered question ─────────────────────────────────────────────

func TestGovG3QuestionPastItsAge(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		{"id": "bd-q", "status": "open", "title": "ask the operator",
			"labels": []string{"question"}, "created_at": govNow.Add(-5 * time.Hour)},
	})
	g := find(shopSet(t, govIn(t, b)), "G3")
	if g == nil || g.Key != "question:bd-q" {
		t.Fatalf("G3 = %+v, want question:bd-q", g)
	}
	if g.Class != GovLane {
		t.Errorf("a question blocking nothing is LANE, got %s", g.Class)
	}
}

// Under the age it is a question somebody may still be about to answer.
func TestGovG3YoungQuestionIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		{"id": "bd-q", "status": "open", "title": "ask", "labels": []string{"question"},
			"created_at": govNow.Add(-1 * time.Hour)},
	})
	if g := find(shopSet(t, govIn(t, b)), "G3"); g != nil {
		t.Errorf("a 1h-old question is inside the 4h default: %+v", *g)
	}
}

func TestGovG3ConfigurableAge(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	appendConfig(t, b.App, "attn_question_age: 30m\n")
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		{"id": "bd-q", "status": "open", "title": "ask", "labels": []string{"question"},
			"created_at": govNow.Add(-1 * time.Hour)},
	})
	if g := find(shopSet(t, govIn(t, b)), "G3"); g == nil {
		t.Error("attn_question_age: 30m must raise a 1h-old question")
	}
}

// URGENT when it holds work out of `bd ready`: an unanswered question that
// only costs its own bead is a lane stopped, one that dep-blocks others is
// the shop stopped behind a sentence nobody wrote.
func TestGovG3UrgentWhenItBlocksOpenWork(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		{"id": "bd-q", "status": "open", "title": "ask", "labels": []string{"risk"},
			"created_at": govNow.Add(-9 * time.Hour)},
	})
	writeJSON(t, dir, "fake-blocked.json", []map[string]any{
		{"id": "bd-2", "status": "open", "blocked_by": []string{"bd-q"}},
	})
	g := find(shopSet(t, govIn(t, b)), "G3")
	if g == nil || g.Class != GovUrgent {
		t.Fatalf("G3 = %+v, want URGENT", g)
	}
	if !strings.Contains(g.Detail, "blocking 1 bead") {
		t.Errorf("the line must say what it holds: %q", g.Detail)
	}
}

// An in_progress bead is being worked despite the question and a deferred
// one was parked on purpose — neither is work this question stops.
func TestGovG3NonOpenBlockedBeadDoesNotPromote(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		{"id": "bd-q", "status": "open", "title": "ask", "labels": []string{"question"},
			"created_at": govNow.Add(-9 * time.Hour)},
	})
	writeJSON(t, dir, "fake-blocked.json", []map[string]any{
		{"id": "bd-2", "status": "in_progress", "blocked_by": []string{"bd-q"}},
		{"id": "bd-3", "status": "deferred", "blocked_by": []string{"bd-q"}},
	})
	g := find(shopSet(t, govIn(t, b)), "G3")
	if g == nil || g.Class != GovLane {
		t.Errorf("G3 = %+v, want LANE", g)
	}
}

// deferStatuses are the two status strings a dated defer is seen with.
// "open" is one of them — `bd defer --until` does not always move the
// status, so a deferred question bead can read back "open" with a date
// (ranger-base-03ada). "deferred" is kept beside it so the dated arms are
// pinned status-blind in both directions: ONCE THERE IS A DATE, THE DATE
// DECIDES (deferredNow).
var deferStatuses = []string{"open", "deferred"}

// notParkedStatuses are the statuses that are not themselves a park, so a
// bead carrying one and no date at all is silence rather than an answer.
// Deliberately omits "deferred", which IS the park when no date survived —
// its own arm is below (ranger-base-nkjjg).
var notParkedStatuses = []string{"open", "in_progress", "blocked"}

// A defer with a future date is an answer — the answer is a date
// (ranger-base-5aln). `bd list` still returns a deferred bead (unlike `bd
// ready`), so without this the row nags a question the operator already
// parked on purpose.
func TestGovG3DeferredUntilFutureIsNotACondition(t *testing.T) {
	for _, status := range deferStatuses {
		t.Run(status, func(t *testing.T) {
			b, _ := newTestBackend(t)
			dir := govRepo(t, b)
			writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
				{"id": "bd-q", "status": status, "title": "ask", "labels": []string{"question"},
					"created_at": govNow.Add(-9 * time.Hour), "defer_until": govNow.Add(48 * time.Hour)},
			})
			if g := find(shopSet(t, govIn(t, b)), "G3"); g != nil {
				t.Errorf("a defer-until-future is answered, not open: %+v", *g)
			}
		})
	}
}

// Once defer_until is in the past, the park has expired and nobody
// revisited it — that is unanswered again, same as any other aging
// question.
func TestGovG3DeferredUntilPastStillNags(t *testing.T) {
	for _, status := range deferStatuses {
		t.Run(status, func(t *testing.T) {
			b, _ := newTestBackend(t)
			dir := govRepo(t, b)
			writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
				{"id": "bd-q", "status": status, "title": "ask", "labels": []string{"question"},
					"created_at": govNow.Add(-9 * time.Hour), "defer_until": govNow.Add(-1 * time.Hour)},
			})
			g := find(shopSet(t, govIn(t, b)), "G3")
			if g == nil || g.Key != "question:bd-q" {
				t.Fatalf("G3 = %+v, want question:bd-q once the defer date has passed", g)
			}
		})
	}
}

// No date and a status that is not itself a park is silence: the aging
// question still nags. This is the arm that keeps the guard honest — without
// it, a reader that skipped every question bead would pass every other arm
// here.
func TestGovG3NoDeferUntilStillNags(t *testing.T) {
	for _, status := range notParkedStatuses {
		t.Run(status, func(t *testing.T) {
			b, _ := newTestBackend(t)
			dir := govRepo(t, b)
			writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
				{"id": "bd-q", "status": status, "title": "ask", "labels": []string{"question"},
					"created_at": govNow.Add(-9 * time.Hour)},
			})
			g := find(shopSet(t, govIn(t, b)), "G3")
			if g == nil || g.Key != "question:bd-q" {
				t.Fatalf("G3 = %+v, want question:bd-q with no defer date at all", g)
			}
		})
	}
}

// Status "deferred" with NO date is the park bd leaves on a `no-db: true`
// JSONL store, and it is the shape this row was nagging through.
//
// MEASURED 2026-10-03, bd 0.50.3, on a hand-written no-db store and on the
// shop's own ~/src/hcn: `bd defer <id> --until 2026-10-16`, `bd defer <id>
// --until=tomorrow` and `bd update <id> --defer 2026-10-16` each print
// success, set status "deferred", and write no defer_until anywhere — not
// the JSONL record, not `bd list --json`, not `bd show --json`. So the date
// the operator typed does not exist to be read, and a reader that demanded
// one paged the coordinator about four questions she had already parked
// (hcn-4, hcn-7, hcn-11, hcn-12 — ranger-base-nkjjg).
//
// Indefinite is still an answer: `bd defer <id>` with no --until is bd's own
// status-based defer, documented as "deliberately set aside", and "not now"
// is a decision somebody made.
// The updated_at stamp is explicit and FRESH, because it is the park's own
// clock since ranger-base-pm5zo: an indefinite park is quiet for
// attn_parked_age and loud after. Without it this fixture would pass through
// the zero-clock branch instead of the one it means to pin.
func TestGovG3StatusDeferredWithNoDateIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		{"id": "bd-q", "status": "deferred", "title": "ask", "labels": []string{"question"},
			"created_at": govNow.Add(-9 * time.Hour), "updated_at": govNow.Add(-1 * time.Hour)},
	})
	if g := find(shopSet(t, govIn(t, b)), "G3"); g != nil {
		t.Errorf("status deferred with no date is a park with no end date, not silence: %+v", *g)
	}
}

// The reported symptom, end to end: the PULSE LINE. Four parked question
// beads in one store — two dated, two carrying only status "deferred" —
// beside one that nobody parked. The line the coordinator is paged with must
// name the fifth and nothing else; the bead was filed because it read
// `question:q-a; question:q-b; question:q-c; question:q-d` every tick with
// all four already answered (ranger-base-nkjjg).
//
// GovLines over the same GovSet the pulse fingerprints (set.Keys(),
// pulse.go) rather than a re-derivation, so this cannot pass while the
// delivered line differs.
func TestGovPulseLineOmitsParkedQuestions(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	old := govNow.Add(-9 * time.Hour)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		// Parked with a date, status unmoved — the SQLite shape.
		{"id": "q-a", "status": "open", "title": "ask a", "labels": []string{"question"},
			"created_at": old, "defer_until": govNow.Add(6 * 24 * time.Hour)},
		// Parked with a date and the status moved too.
		{"id": "q-b", "status": "deferred", "title": "ask b", "labels": []string{"question"},
			"created_at": old, "defer_until": govNow.Add(13 * 24 * time.Hour)},
		// Parked, date discarded by the store — the no-db JSONL shape, and
		// hcn-4/7/11/12's own. updated_at is the park's own clock since
		// ranger-base-pm5zo and these two are freshly parked, said out loud
		// rather than left to the zero-clock branch.
		{"id": "q-c", "status": "deferred", "title": "ask c", "labels": []string{"question"},
			"created_at": old, "updated_at": govNow.Add(-1 * time.Hour)},
		{"id": "q-d", "status": "deferred", "title": "ask d", "labels": []string{"question"},
			"created_at": old, "updated_at": govNow.Add(-1 * time.Hour)},
		// Nobody parked this one. It is the control: a guard that went
		// quiet about every question would pass without it.
		{"id": "q-e", "status": "open", "title": "ask e", "labels": []string{"question"},
			"created_at": old},
	})
	line := GovLines(shopSet(t, govIn(t, b)))
	if !strings.Contains(line, "question:q-e") {
		t.Errorf("the one unparked question must still page: %q", line)
	}
	for _, id := range []string{"q-a", "q-b", "q-c", "q-d"} {
		if strings.Contains(line, "question:"+id) {
			t.Errorf("%s is parked and must not page: %q", id, line)
		}
	}
}

// ─── G3 · a park with no end date, past its horizon (ranger-base-pm5zo) ──────

// parkDay is one day of the indefinite-park horizon, so an arm below reads in
// the unit DefaultAttnParkedAge is written in.
const parkDay = 24 * time.Hour

// datelessPark is a question bead in the shape one store class leaves behind:
// status "deferred", NO defer_until, and an updated_at that is when the park
// happened (MEASURED 2026-10-03, bd 0.50.3 — `bd defer` stamps it and writes
// no date).
//
// created_at is pinned FAR past the horizon on every arm, including the arms
// that must stay quiet, and that is the fixture's whole point: it is the one
// condition under which the two available clocks disagree. A reader keyed on
// Created passes every arm here if the two are close together, and it is
// wrong — MEASURED 2026-10-03, the five live dateless parks were created
// between 5h28m and 23m before the park that is the thing being dated, and a
// question bead parked, revisited and re-parked is older still.
func datelessPark(id string, parkedAgo time.Duration) map[string]any {
	return map[string]any{
		"id": id, "status": "deferred", "title": "ask " + id, "labels": []string{"question"},
		"created_at": govNow.Add(-99 * parkDay),
		"updated_at": govNow.Add(-parkedAgo),
	}
}

// An indefinite park goes quiet, and then it goes LOUD AGAIN. This is the
// whole bead: ranger-base-nkjjg stopped a dateless park paging every tick,
// and nothing else in the shop re-surfaces one — MEASURED 2026-10-03, bd
// 0.50.3, on both store classes: a defer date four weeks past is still absent
// from `bd ready`. So without this arm "quiet" means "silent forever".
//
// The boundary is pinned on both sides rather than asserted once, because the
// number is the deliverable here (14d, argued at DefaultAttnParkedAge) and an
// off-by-a-day in either direction is the failure this row is trying to avoid
// — early is ranger-base-nkjjg come back, late is a question rotting.
func TestGovG3IndefiniteParkGoesLoudAgainPastItsHorizon(t *testing.T) {
	for _, c := range []struct {
		name      string
		parkedAgo time.Duration
		loud      bool
	}{
		{"a day under", 13 * parkDay, false},
		{"an hour under", 14*parkDay - time.Hour, false},
		{"exactly the horizon", 14 * parkDay, true},
		{"a day over", 15 * parkDay, true},
		{"a month over", 45 * parkDay, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			dir := govRepo(t, b)
			writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{datelessPark("bd-q", c.parkedAgo)})
			g := find(shopSet(t, govIn(t, b)), "G3")
			if !c.loud {
				if g != nil {
					t.Fatalf("a park %s old is inside the 14d default: %+v", c.parkedAgo, *g)
				}
				return
			}
			if g == nil {
				t.Fatal("a park nobody came back to must be loud again — nothing else re-surfaces one")
			}
			// Its own key, because a stale park and a never-answered
			// question are different conditions with different remedies and
			// the pulse fingerprints these.
			if g.Key != "parked:bd-q" {
				t.Errorf("Key = %q, want parked:bd-q", g.Key)
			}
			// Its own sentence. The ordinary row would say "open 32d
			// unanswered" about a bead `bd list` shows ❄ DEFERRED, and the
			// operator would read that as ranger-base-nkjjg come back.
			for _, want := range []string{"bd-q", "no end date", "nobody came back", "ask bd-q"} {
				if !strings.Contains(g.Detail, want) {
					t.Errorf("Detail = %q, must contain %q", g.Detail, want)
				}
			}
			// In days, not BlindFor's hours: a fortnight-and-up age told as
			// "336h00m" is the answer erased.
			if !strings.Contains(g.Detail, fmt.Sprintf("%dd ago", int(c.parkedAgo.Hours()/24))) {
				t.Errorf("Detail = %q, must say how long ago in days", g.Detail)
			}
		})
	}
}

// The horizon must never reach a DATED park, on either side of its date.
// That is this bead's own pin, and it is the thing a reader keyed on "any
// deferred bead that has not been touched in a fortnight" would get wrong: a
// park the operator deliberately set a month out would be reported as
// forgotten at the fortnight, which is a row that cannot be cleared except
// by un-parking the bead.
func TestGovG3DatedParkIsOutOfTheParkHorizonsReach(t *testing.T) {
	for _, c := range []struct {
		name    string
		defer_  time.Duration
		wantKey string
	}{
		// 30 days out and last touched 60 days ago: well past the park
		// horizon, and still answered — by its date.
		{"future date, long untouched", 30 * parkDay, ""},
		// Past its date, so the park expired and nobody revisited it:
		// unanswered again, and it is the ORDINARY row that says so, not
		// the park row. Unchanged from before this bead.
		{"past date, long untouched", -1 * parkDay, "question:bd-q"},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, status := range deferStatuses {
				t.Run(status, func(t *testing.T) {
					b, _ := newTestBackend(t)
					dir := govRepo(t, b)
					writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
						{"id": "bd-q", "status": status, "title": "ask", "labels": []string{"question"},
							"created_at":  govNow.Add(-70 * parkDay),
							"updated_at":  govNow.Add(-60 * parkDay),
							"defer_until": govNow.Add(c.defer_)},
					})
					g := find(shopSet(t, govIn(t, b)), "G3")
					if c.wantKey == "" {
						if g != nil {
							t.Fatalf("the date decides for a dated park, whatever the park horizon says: %+v", *g)
						}
						return
					}
					if g == nil || g.Key != c.wantKey {
						t.Fatalf("G3 = %+v, want %s", g, c.wantKey)
					}
				})
			}
		})
	}
}

// A park with no clock to read stays quiet, matching what the loop does three
// lines above with a zero created_at. A clock that cannot be read does not
// date a park, and a row nobody could ever clear is the defect this row
// exists to avoid.
//
// MEASURED 2026-10-03/04, bd 0.50.3: no live record is in this shape. Every
// record in both of the shop's stores carries updated_at, in the JSONL and in
// `bd list --json`. This arm names the safe direction, not a population.
func TestGovG3IndefiniteParkWithNoClockStaysQuiet(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		{"id": "bd-q", "status": "deferred", "title": "ask", "labels": []string{"question"},
			"created_at": govNow.Add(-99 * parkDay)},
	})
	if g := find(shopSet(t, govIn(t, b)), "G3"); g != nil {
		t.Errorf("no updated_at is no park date, not a forgotten park: %+v", *g)
	}
}

// The horizon is a config key with the same grammar as the other two, and
// zero means every tick — which puts the pre-ranger-base-nkjjg noise back for
// an instance that wants it, deliberately spellable rather than reachable
// only by editing the binary.
func TestGovG3ConfigurableParkedAge(t *testing.T) {
	for _, c := range []struct {
		cfg  string
		loud bool
	}{
		{"attn_parked_age: 48h\n", true},
		{"attn_parked_age: 0\n", true},
		{"attn_parked_age: 30d\n", false}, // not Go duration grammar: a typo
		{"", false},
	} {
		t.Run(strings.TrimSpace(c.cfg), func(t *testing.T) {
			b, _ := newTestBackend(t)
			dir := govRepo(t, b)
			if c.cfg != "" {
				appendConfig(t, b.App, c.cfg)
			}
			writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{datelessPark("bd-q", 72*time.Hour)})
			g := find(shopSet(t, govIn(t, b)), "G3")
			if c.loud && (g == nil || g.Key != "parked:bd-q") {
				t.Fatalf("G3 = %+v, want parked:bd-q under %q", g, c.cfg)
			}
			if !c.loud && g != nil {
				t.Fatalf("a 3d-old park is inside the horizon under %q: %+v", c.cfg, *g)
			}
		})
	}
}

// A forgotten park that dep-blocks work is graded by the same rule the
// unanswered row is graded by, because it is the same fact: the shop is
// stopped behind a sentence nobody wrote, and here nobody even wrote down
// when they meant to come back to it.
func TestGovG3IndefiniteParkIsUrgentWhenItBlocksOpenWork(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{datelessPark("bd-q", 20*parkDay)})
	writeJSON(t, dir, "fake-blocked.json", []map[string]any{
		{"id": "bd-2", "status": "open", "blocked_by": []string{"bd-q"}},
	})
	g := find(shopSet(t, govIn(t, b)), "G3")
	if g == nil || g.Key != "parked:bd-q" || g.Class != GovUrgent {
		t.Fatalf("G3 = %+v, want parked:bd-q URGENT", g)
	}
	if !strings.Contains(g.Detail, "blocking 1 bead") {
		t.Errorf("the line must say what it holds: %q", g.Detail)
	}
}

// End to end, the way the coordinator actually meets this: the PULSE LINE,
// over the same GovSet the pulse fingerprints (set.Keys(), pulse.go) rather
// than a re-derivation, so it cannot pass while the delivered line differs.
//
// One park inside the horizon and one past it. The fresh one is the control —
// a row that went loud about every dateless park would pass without it, and
// that row is exactly what ranger-base-nkjjg was filed to remove.
func TestGovPulseLineNamesOnlyTheForgottenPark(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-list-labeled.json", []map[string]any{
		datelessPark("q-fresh", 2*parkDay),
		datelessPark("q-forgotten", 40*parkDay),
	})
	line := GovLines(shopSet(t, govIn(t, b)))
	if !strings.Contains(line, "parked:q-forgotten") {
		t.Errorf("a park nobody came back to must reach the pulse line: %q", line)
	}
	if strings.Contains(line, "q-fresh") {
		t.Errorf("a park inside its horizon is an answer and must not page: %q", line)
	}
}

// ─── G4/G5 · the plan guard ──────────────────────────────────────────────────

// govGuardCfg arms the guard on the fake adapter's own window names, so
// nothing here depends on the shipped provider's labels.
const govGuardCfg = "plan_guard_burst: 70\nplan_guard_month: 85\n"

// A guard that is tripping and has been for longer than attn_guard_stuck.
// The reading is re-taken here; the STREAK is the watch process's, handed
// in — which is the whole shape of this row.
func TestGovG4GuardStuckPastItsAge(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg)
	in := govIn(t, b)
	in.Plan = &fakePlanReader{windows: fakeWindows(78, 40)}
	in.GuardTrippedSince = govNow.Add(-3 * time.Hour)
	g := find(shopSet(t, in), "G4")
	if g == nil || g.Key != "guard-stuck" || g.Class != GovUrgent {
		t.Fatalf("G4 = %+v, want guard-stuck URGENT", g)
	}
	if !strings.Contains(g.Detail, "burst at 78% > 70%") {
		t.Errorf("the line must name the window that tripped: %q", g.Detail)
	}
}

// Under the age it is a SKIP — automatic, self-healing, pure mechanism —
// and the design is explicit that no condition may latch one of those.
func TestGovG4ShortStreakIsASkipNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg)
	in := govIn(t, b)
	in.Plan = &fakePlanReader{windows: fakeWindows(78, 40)}
	in.GuardTrippedSince = govNow.Add(-10 * time.Minute)
	if g := find(shopSet(t, in), "G4"); g != nil {
		t.Errorf("a 10m skip is mechanism, not a governance condition: %+v", *g)
	}
}

// No streak, no row. A process that is not the watch loop has no streak
// clock, and inventing one from a single reading would turn every ordinary
// skip into an URGENT the moment somebody typed `posse status`.
func TestGovG4NoStreakNoRow(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg)
	in := govIn(t, b)
	in.Plan = &fakePlanReader{windows: fakeWindows(78, 40)}
	if g := find(shopSet(t, in), "G4"); g != nil {
		t.Errorf("a fresh shell has no streak and must report no G4: %+v", *g)
	}
}

// A streak whose reading has since come back under the threshold is over:
// the row is re-taken at view time, so it heals without anyone clearing it.
func TestGovG4HealedReadingClearsIt(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg)
	in := govIn(t, b)
	in.Plan = &fakePlanReader{windows: fakeWindows(12, 40)}
	in.GuardTrippedSince = govNow.Add(-3 * time.Hour)
	if g := find(shopSet(t, in), "G4"); g != nil {
		t.Errorf("a reading under the threshold is not a skip: %+v", *g)
	}
}

// G5: monitoring itself is broken. The blind clock is the SHARED snapshot's
// own timestamp, which is what lets a fresh shell answer this at all.
func TestGovG5GuardBlindPastItsBudget(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 10m\n")
	seedPlanSnapshot(t, b.App, govNow.Add(-45*time.Minute))
	in := govIn(t, b)
	in.Plan = &fakePlanReader{err: Die("usage endpoint: 429")}
	g := find(shopSet(t, in), "G5")
	// The hour bucket is part of the key since ranger-base-lpoui, and 45m
	// blind is the zeroth hour of it. An error with no CLASS appends
	// nothing — this one is Die(), not a *RateLimit, so it is a socket-shaped
	// failure however its sentence reads.
	if g == nil || g.Key != "guard-blind:0h" || g.Class != GovUrgent {
		t.Fatalf("G5 = %+v, want guard-blind:0h URGENT", g)
	}
}

// Inside the budget it is the fail-open case the guard already handles.
func TestGovG5InsideTheBudgetIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 1h\n")
	seedPlanSnapshot(t, b.App, govNow.Add(-5*time.Minute))
	in := govIn(t, b)
	in.Plan = &fakePlanReader{err: Die("usage endpoint: 429")}
	if g := find(shopSet(t, in), "G5"); g != nil {
		t.Errorf("5m blind against a 1h budget is quiet tolerance: %+v", *g)
	}
}

// A blind window the box was ASLEEP for is not a blind guard (ADR 0064
// D1-D2). The row says this shop's monitoring has been down, and a box with
// the lid shut is not a shop with broken monitoring, so the hours it counts
// are awake hours. (The hiring GATE over the same budget keeps WALL age on
// purpose — ADR 0065 — and is not under test here.) This is the 2026-10-01 wake in
// miniature: 45m since the last snapshot, 40m of it a witnessed sleep, 5m of
// awake blindness against a 10m budget.
//
// Both arms, because the discount is the whole content of the row and an arm
// that only ever saw a cleared row would stay green if blindPast returned a
// constant.
//
// MUTATION: drop the `- slept` from blindPast → the first arm reds. Drop the
// `+suspendDiscount(...)` from G5's add → the second reds.
func TestGovG5SuspendedWindowIsNotBlindness(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, govGuardCfg+"plan_guard_blind_max: 10m\n")
	seedPlanSnapshot(t, b.App, govNow.Add(-45*time.Minute))

	// The control first: the same fixture with no witness is the row as it
	// fired on the night this was filed, so the arm below is the subtraction
	// doing the work and not a fixture that was never blind.
	in := govIn(t, b)
	in.Plan = &fakePlanReader{err: Die("usage endpoint: 429")}
	if g := find(shopSet(t, in), "G5"); g == nil || g.Key != "guard-blind:0h" {
		t.Fatalf("premise: a witnessless 45m blind window against a 10m budget must be G5, got %+v", g)
	}

	in.Suspended = govSuspended(govNow, 40*time.Minute)
	if g := find(shopSet(t, in), "G5"); g != nil {
		t.Errorf("5m of awake blindness against a 10m budget is not a condition, whatever the wall says: %+v", *g)
	}

	// And a window the sleep only partly covers still fires — deferred, never
	// lost — naming both numbers, because `pmset -g log` is the reader's next
	// step and the awake age alone cannot be reconciled with the snapshot.
	in.Suspended = govSuspended(govNow, 20*time.Minute)
	g := find(shopSet(t, in), "G5")
	if g == nil || g.Key != "guard-blind:0h" || g.Class != GovUrgent {
		t.Fatalf("25m of awake blindness past a 10m budget is still G5, got %+v", g)
	}
	for _, want := range []string{"blind 25m", "awake for 25m", "20m", "pmset -g log"} {
		if !strings.Contains(g.Detail, want) {
			t.Errorf("a discounted row must name %q: %q", want, g.Detail)
		}
	}
}

// An unarmed guard has no meter, so it can be neither blind nor stuck —
// and it must make no request finding that out.
func TestGovUnarmedGuardReadsNothing(t *testing.T) {
	b, _ := newTestBackend(t)
	f := &fakePlanReader{windows: fakeWindows(99, 99)}
	in := govIn(t, b)
	in.Plan = f
	in.GuardTrippedSince = govNow.Add(-9 * time.Hour)
	set := shopSet(t, in)
	if find(set, "G4") != nil || find(set, "G5") != nil {
		t.Errorf("no plan_guard_<window>: means no meter: %v", set.Keys())
	}
	if f.reads != 0 {
		t.Errorf("an unarmed guard made %d request(s)", f.reads)
	}
}

// ─── G6 · Dial E stop ────────────────────────────────────────────────────────

func TestGovG6BudgetStop(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "budget_day: 10\n")
	in := govIn(t, b)
	in.Spend = func(time.Time) *CostReport { return spendReport(govNow, 12.50) }
	g := find(shopSet(t, in), "G6")
	if g == nil || g.Key != "budget-stop:day" || g.Class != GovUrgent {
		t.Fatalf("G6 = %+v, want budget-stop:day URGENT", g)
	}
}

func TestGovG6UnderTheCapIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "budget_day: 10\n")
	in := govIn(t, b)
	in.Spend = func(time.Time) *CostReport { return spendReport(govNow, 9.99) }
	if g := find(shopSet(t, in), "G6"); g != nil {
		t.Errorf("99%% of the cap is a step-down, not a stop: %+v", *g)
	}
}

// Dormant by default: no cap set means nothing is scanned at all, which is
// what keeps this row free on every instance that has not armed Dial E.
func TestGovG6DormantScansNothing(t *testing.T) {
	b, _ := newTestBackend(t)
	in := govIn(t, b)
	in.Spend = func(time.Time) *CostReport { t.Fatal("no cap set: the ledger must not be scanned"); return nil }
	shopSet(t, in)
}

// ADR 0029 §1 amendment (bead ranger-base-jbmh): once the epoch re-key
// landed (ranger-base-f0y3), G6 reads `budget_pass:` too — denominated in
// the wall-clock epoch, not a dispatcher's own pass.
func TestGovG6EpochWindowTripsOnEpochSpend(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "budget_pass: 10\n")
	in := govIn(t, b)
	in.Spend = func(time.Time) *CostReport { return spendReport(govNow, 12.50) }
	g := find(shopSet(t, in), "G6")
	if g == nil || g.Key != "budget-stop:epoch" || g.Class != GovUrgent {
		t.Fatalf("G6 = %+v, want budget-stop:epoch URGENT", g)
	}
}

// The control arm the bead names: day under cap and epoch over cap must
// still trip G6 — the case a day-only reading is blind to, and the whole
// reason this row costs the second in-memory sum.
func TestGovG6DayUnderCapEpochOverCapStillTrips(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "budget_day: 100\nbudget_pass: 5\n")
	in := govIn(t, b)
	in.Spend = func(time.Time) *CostReport { return spendReport(govNow, 6) }
	g := find(shopSet(t, in), "G6")
	if g == nil || g.Key != "budget-stop:epoch" || g.Class != GovUrgent {
		t.Fatalf("day 6%% under its $100 cap must not hide an epoch at 120%% of its $5 cap: G6 = %+v, want budget-stop:epoch URGENT", g)
	}
}

// An unreadable ledger is not $0 spent (ADR 0018 §3). Under the cap on a
// FLOOR is not an all-clear, so it comes back as a partial set rather than
// as silence.
func TestGovG6UnreadableLedgerIsPartialNotClear(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "budget_day: 10\n")
	in := govIn(t, b)
	in.Spend = func(time.Time) *CostReport {
		rep := spendReport(govNow, 1.00)
		rep.noteUnread(Die("transcript unreadable"))
		return rep
	}
	set, failed := ShopCheck(in)
	if len(failed) == 0 {
		t.Fatalf("an uncountable ledger must not read as clear: %v", set.Keys())
	}
	if find(set, "G6") != nil {
		t.Error("a floor under the cap is not a stop")
	}
}

// ─── G7 · the watch loop is dead ─────────────────────────────────────────────

// The meta-condition, and the ADR's own observable: a killed loop shows G7
// in a status run from a fresh shell. The probe is the kernel's flock, so
// this test holds nothing and the lock is free — which IS the dead case.
func TestGovG7LoopDeadWhileAutostartArmed(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval: 5m\n")
	g := find(shopSet(t, govIn(t, b)), "G7")
	if g == nil || g.Key != "loop-dead" || g.Class != GovUrgent {
		t.Fatalf("G7 = %+v, want loop-dead URGENT", g)
	}
}

// Disarmed autostart has no loop to miss. The row is "the loop that should
// be running is not", never "no loop is running".
func TestGovG7DisarmedAutostartIsNotACondition(t *testing.T) {
	b, _ := newTestBackend(t)
	if g := find(shopSet(t, govIn(t, b)), "G7"); g != nil {
		t.Errorf("no autostart_interval: means nothing is owed a loop: %+v", *g)
	}
}

// A bare `autostart_interval:` is a BROKEN arm, not an armed one: the hook
// refuses it and arms nothing (ranger-base-cxyk), so the row must name the
// empty key rather than report a dead loop under a config that will never
// start one. Gating G7 on presence alone said "autostart is armed" about
// exactly that config (ranger-base-i6h). One subtest per shape the hook's
// cfg() and CfgGet both read as empty — `""` among them since cfg() drops a
// matched pair of double quotes the way yamlClean does (ranger-base-k3yd),
// and null/~ since yamlGetLines maps a cleaned one to the empty string and
// cfg() now does too (ranger-base-fqfw). This is the posse half of that
// pairing; the hook half is TestAutostartNullIntervalIsTheSameBrokenArmAsABareKey.
func TestGovG7BareIntervalIsABrokenArmNotADeadLoop(t *testing.T) {
	for name, line := range map[string]string{
		"bare":         "autostart_interval:\n",
		"whitespace":   "autostart_interval:   \n",
		"comment":      "autostart_interval: # 5m\n",
		"quoted empty": "autostart_interval: \"\"\n",
		"null":         "autostart_interval: null\n",
		"tilde":        "autostart_interval: ~\n",
		"quoted null":  "autostart_interval: \"null\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			appendConfig(t, b.App, line)
			g := find(shopSet(t, govIn(t, b)), "G7")
			if g == nil {
				t.Fatal("a broken arm delivers nothing — G7 must still fire")
			}
			if g.Key != "arm-broken" || g.Class != GovUrgent {
				t.Errorf("G7 = %+v, want arm-broken URGENT", *g)
			}
			if strings.Contains(g.Detail, "is armed") {
				t.Errorf("a refused key is not armed: %q", g.Detail)
			}
			if !strings.Contains(g.Detail, "autostart_interval:") ||
				!strings.Contains(g.Detail, b.App.ConfigPath) ||
				!strings.Contains(g.Detail, "present but empty") {
				t.Errorf("the row must name the key and the file it is in: %q", g.Detail)
			}
		})
	}
}

// The other broken arm: a value the hook cannot read. It used to fall
// through to the lock and be reported as `loop-dead — autostart is armed`,
// which after ranger-base-7rt5 is false twice over: nothing is armed, and
// nothing will be at the next herdr start either. The hook and this row are
// the two surfaces the seed config promises never disagree, so they move
// together.
func TestGovG7MalformedIntervalIsABrokenArmNotADeadLoop(t *testing.T) {
	// `""` is not here because both readers now take the EMPTY arm above on
	// it — one arm, named once. It used to be an empty value to yamlClean and
	// a malformed one to the hook's cfg(), which is the split ranger-base-k3yd
	// closed. A quoted value that is NOT empty is likewise not a fixture here:
	// `"5m"` is 5m to both readers, and arms.
	for _, value := range []string{"banana", "0", "5min", "-5m", "30 s"} {
		t.Run(value, func(t *testing.T) {
			b, _ := newTestBackend(t)
			appendConfig(t, b.App, "autostart_interval: "+value+"\n")
			g := find(shopSet(t, govIn(t, b)), "G7")
			if g == nil {
				t.Fatal("a broken arm delivers nothing — G7 must still fire")
			}
			if g.Key != "arm-broken" || g.Class != GovUrgent {
				t.Errorf("G7 = %+v, want arm-broken URGENT", *g)
			}
			if strings.Contains(g.Detail, "is armed") {
				t.Errorf("a refused value is not armed: %q", g.Detail)
			}
			if !strings.Contains(g.Detail, "autostart_interval:") ||
				!strings.Contains(g.Detail, value) ||
				!strings.Contains(g.Detail, b.App.ConfigPath) {
				t.Errorf("the row must name the key, the value and the file: %q", g.Detail)
			}
		})
	}
}

// The positive control for that table: an interval posse accepts is armed,
// so with no loop up the row is loop-dead — never arm-broken.
func TestGovG7ValidIntervalIsStillADeadLoop(t *testing.T) {
	for _, value := range []string{"5m", "45", "1h30m", "500ms"} {
		t.Run(value, func(t *testing.T) {
			b, _ := newTestBackend(t)
			appendConfig(t, b.App, "autostart_interval: "+value+"\n")
			g := find(shopSet(t, govIn(t, b)), "G7")
			if g == nil || g.Key != "loop-dead" {
				t.Errorf("G7 = %+v, want loop-dead for an armed config with no loop", g)
			}
		})
	}
}

// And the broken arm does not depend on the lock: a live loop clears
// loop-dead, never arm-broken — the next herdr start still refuses the key.
func TestGovG7BrokenArmSurvivesALiveLoop(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval:\n")
	lock, held, err := lockWatch(b.App)
	if err != nil || held {
		t.Fatalf("could not take the watch lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	if g := find(shopSet(t, govIn(t, b)), "G7"); g == nil || g.Key != "arm-broken" {
		t.Errorf("G7 = %+v, want arm-broken even with a loop up", g)
	}
}

// And a loop that IS running clears it — the same lock a second --watch
// refuses on. A live loop writes its own log (watchlog.go), so a fixture
// that holds the lock and writes nothing is not one: that is the loop-mute
// arm below, and the rig has to plant what a real loop plants.
func TestGovG7LiveLoopClearsIt(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval: 5m\n")
	lock, held, err := lockWatch(b.App)
	if err != nil || held {
		t.Fatalf("could not take the watch lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	writeWatchLog(t, b.App, govNow)
	if g := find(shopSet(t, govIn(t, b)), "G7"); g != nil {
		t.Errorf("a live loop must clear G7: %+v", *g)
	}
}

// ─── G7 · the loop is alive and its record is not (ranger-base-n00wn) ────────

// writeWatchLog plants the log a live loop keeps, dated `at`. Dated against
// govNow and never the wall: the row reads this file's AGE, and a fixture
// stamped with the real clock is a fixture in the future of the frozen one.
func writeWatchLog(t *testing.T, a *App, at time.Time) string {
	t.Helper()
	path := WatchLogPath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("── pass 1 · 09:00:00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
	return path
}

// THE BEAD ITSELF, from the other side. On 2026-08-31 18:08 the fleet's log
// stopped and the loop did not: the lock was held, `--watch-status` said
// running, and every surface reported health for three days while every
// retrospective question went unanswerable. Nothing was red because nothing
// read the file's age. This is the row that does.
func TestGovG7LiveLoopWithAStaleLogIsMute(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval: 5m\n")
	lock, held, err := lockWatch(b.App)
	if err != nil || held {
		t.Fatalf("could not take the watch lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	// Three days, the outage's own number — and well past the 85m a 5m/40m
	// loop can legitimately be quiet.
	writeWatchLog(t, b.App, govNow.Add(-72*time.Hour))
	g := find(shopSet(t, govIn(t, b)), "G7")
	if g == nil || g.Key != "loop-mute" || g.Class != GovUrgent {
		t.Fatalf("G7 = %+v, want loop-mute URGENT", g)
	}
	// The line has to name the file, or the operator is told a loop is
	// mute and not where to look.
	if !strings.Contains(g.Detail, "dispatch-watch.log") {
		t.Errorf("the mute row must name the log: %q", g.Detail)
	}
	if !strings.Contains(g.Detail, "72h00m ago") {
		t.Errorf("the mute row must say how stale: %q", g.Detail)
	}
}

// A log that is not there at all is the same fact one shape over: the record
// is not being written. Named separately because the sentence differs — an
// absent file is not a stale one, and telling an operator their log was
// "last written 0s ago" about a file that does not exist would be the
// unreadable-store-reads-as-clear mistake this surface exists to end.
func TestGovG7LiveLoopWithNoLogAtAllIsMute(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval: 5m\n")
	lock, held, err := lockWatch(b.App)
	if err != nil || held {
		t.Fatalf("could not take the watch lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	g := find(shopSet(t, govIn(t, b)), "G7")
	if g == nil || g.Key != "loop-mute" {
		t.Fatalf("G7 = %+v, want loop-mute for a loop with no log", g)
	}
	if !strings.Contains(g.Detail, "no log at") {
		t.Errorf("the row must say the log is absent, not stale: %q", g.Detail)
	}
}

// A DEAD loop is loop-dead and not loop-mute, however old the log is. The
// keys are the two halves of one row and they must not race: a fleet with no
// loop at all has a stale log by definition, and reporting the symptom over
// the cause would point the operator at a file instead of at the arm.
func TestGovG7DeadLoopIsNotMute(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval: 5m\n")
	writeWatchLog(t, b.App, govNow.Add(-72*time.Hour))
	g := find(shopSet(t, govIn(t, b)), "G7")
	if g == nil || g.Key != "loop-dead" {
		t.Fatalf("G7 = %+v, want loop-dead — the lock is free", g)
	}
}

// A loop QUIET inside its budget is not mute. The threshold is the
// watchdog's own guarantee (WatchLogStaleAfter), so a log written one
// backed-off interval ago — an ordinary idle loop — must clear the row, or
// the fleet's most-believed surface cries wolf every backoff.
func TestGovG7QuietLoopInsideTheBudgetIsNotMute(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval: 5m\n")
	lock, held, err := lockWatch(b.App)
	if err != nil || held {
		t.Fatalf("could not take the watch lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	writeWatchLog(t, b.App, govNow.Add(-41*time.Minute))
	if g := find(shopSet(t, govIn(t, b)), "G7"); g != nil {
		t.Errorf("a loop one backed-off interval quiet is not mute: %+v", *g)
	}
}

// The threshold reads the config that ARMED the loop, both keys. A shop with
// a tight cap is mute sooner than one with a loose cap, and a pin that only
// ever asked the default would stay green if the max-interval term were
// dropped entirely.
func TestGovG7MuteThresholdFollowsTheConfiguredCap(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval: 5m\nautostart_max_interval: 3h\n")
	lock, held, err := lockWatch(b.App)
	if err != nil || held {
		t.Fatalf("could not take the watch lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	// 2h is the discriminating age: inside the 6h05m a 5m/3h loop is
	// allowed, past the 85m a 5m/40m default loop is.
	writeWatchLog(t, b.App, govNow.Add(-2*time.Hour))
	if g := find(shopSet(t, govIn(t, b)), "G7"); g != nil {
		t.Errorf("a 3h cap tolerates a 2h quiet: %+v", *g)
	}
	// The control: the same age under the default cap IS mute, so the arm
	// above is the cap doing the work and not the age being small.
	c, _ := newTestBackend(t)
	appendConfig(t, c.App, "autostart_interval: 5m\n")
	lock2, held2, err := lockWatch(c.App)
	if err != nil || held2 {
		t.Fatalf("could not take the watch lock: held=%v err=%v", held2, err)
	}
	defer lock2.Release()
	writeWatchLog(t, c.App, govNow.Add(-2*time.Hour))
	if g := find(shopSet(t, govIn(t, c)), "G7"); g == nil || g.Key != "loop-mute" {
		t.Fatalf("the same 2h quiet under the default cap must be mute, got %+v", g)
	}
}

// A log the box was ASLEEP past its budget for is not a mute loop (ADR 0064
// D1-D2). The threshold is the watchdog's own guarantee and the watchdog's
// clock is Go monotonic, which on darwin does not advance across a suspend
// (suspend.go's head) — so a wall age compared to it was two quantities, and
// on 2026-10-01 a box that slept 5h30m woke to `loop-mute` URGENT and a loop
// that had done nothing wrong.
//
// Same key, same class, same threshold: what moved is the reading.
//
// MUTATION: drop the `- slept` from watchLogRow → the second arm reds. Drop
// the suspendDiscount from its detail → the third reds.
func TestGovG7SuspendedWindowIsNotAMuteLoop(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "autostart_interval: 5m\n")
	lock, held, err := lockWatch(b.App)
	if err != nil || held {
		t.Fatalf("could not take the watch lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	// The incident's own shape at the stale-log fixture's age: a live loop, a
	// log 72h untouched, and a 5m/40m arm whose budget is 85m.
	writeWatchLog(t, b.App, govNow.Add(-72*time.Hour))

	// The control: no witness, the row as it has always fired.
	in := govIn(t, b)
	if g := find(shopSet(t, in), "G7"); g == nil || g.Key != "loop-mute" {
		t.Fatalf("premise: a witnessless 72h quiet must be loop-mute, got %+v", g)
	}

	// The whole window asleep: zero awake quiet, so there is nothing to
	// report. The loop's next pass is still due on the interval it was
	// waiting out, because its own timers did not advance either.
	in.Suspended = govSuspended(govNow, 72*time.Hour)
	if g := find(shopSet(t, in), "G7"); g != nil {
		t.Errorf("a loop that was frozen with the box is not a loop that stopped writing: %+v", *g)
	}

	// Part of it awake, and still past the budget: the row fires. The signal
	// is deferred by a sleep and never hidden by one — which is the whole
	// reason this is a subtraction and not a wake grace.
	in.Suspended = govSuspended(govNow, 70*time.Hour)
	g := find(shopSet(t, in), "G7")
	if g == nil || g.Key != "loop-mute" || g.Class != GovUrgent {
		t.Fatalf("2h of awake quiet past an 85m budget is still loop-mute, got %+v", g)
	}
	// The wall age stays, because that is what the file's mtime says and a
	// row an operator cannot reconcile with `ls -l` is worse than no row;
	// the awake age and the discounted span arrive beside it.
	for _, want := range []string{"written 72h00m ago", "awake for 2h00m", "70h00m", "pmset -g log"} {
		if !strings.Contains(g.Detail, want) {
			t.Errorf("a discounted mute row must name %q: %q", want, g.Detail)
		}
	}
}

// ─── the suspend reading itself ──────────────────────────────────────────────

// The reading is clipped to the window it is about, and the clip is load
// bearing rather than decoration: the ledger's `now` is the WITNESS's pair
// seam (suspend.go, deliberately not d.now()) while the subtraction's is
// in.now(), so the two ends of this window come from two clocks. A sleep
// recorded by the witness goroutine between the moment ShopCheck took `now`
// and the moment the row is built has a span ending AFTER that now, and every
// fixture in this package freezes one clock and not the other. Unclipped, a
// discount longer than the window is a negative age — which BlindFor renders
// as "0s" and a threshold compare reads as a cleared row, so the row would be
// cleared by arithmetic rather than by the box having been asleep.
//
// MUTATION: delete the `slept > window` clip → the second arm reds. Delete the
// nil guard → the first panics.
// No t.Parallel: this reads govNow, the file's clock, and cmd/testparallel
// does not clear that package var for a parallel test (`go run
// ./cmd/testparallel ./internal/posse extra` names it). The alternative was
// clearing govNow in parallelOK, which is a statement about every test in this
// file and not about this one — and the run is 0.00s, so the slot buys nothing.
func TestGovSuspendedReadingIsClippedToItsWindow(t *testing.T) {
	since := govNow.Add(-time.Hour)
	in := GovInputs{Now: func() time.Time { return govNow }}

	if got := in.suspended(since, govNow); got != 0 {
		t.Errorf("no witness must read as no discount, got %s — a process that did not see the sleep may not "+
			"subtract one (ADR 0064 D4)", got)
	}

	in.Suspended = func(time.Time) time.Duration { return 9 * time.Hour }
	if got, want := in.suspended(since, govNow), time.Hour; got != want {
		t.Errorf("a 9h discount inside a 1h window read as %s, want %s clipped: an age minus this is the AWAKE "+
			"age and must land in [0, age]", got, want)
	}

	in.Suspended = func(time.Time) time.Duration { return -time.Hour }
	if got := in.suspended(since, govNow); got != 0 {
		t.Errorf("a negative discount read as %s: nothing adds to a wall age, because a backward clock step is "+
			"reported and never corrected (ADR 0064 D6)", got)
	}

	in.Suspended = func(time.Time) time.Duration { return time.Hour }
	if got := in.suspended(time.Time{}, govNow); got != 0 {
		t.Errorf("a zero since is no window and read as %s: the caller with no timestamp to measure from — an "+
			"absent log, a guard that never had a reading — has no age to subtract from either", got)
	}
}

// ─── the suspend reading's two ends (ADR 0064 D2, D4) ────────────────────────

// The pulse wires the witness in; a one-shot process does not, and the
// absence is the design rather than an omission (ADR 0064 D4). Pinned by
// READING the struct, because a comment saying "deliberately absent" is green
// in both worlds — the same way GuardTrippedSince's absence is pinned here.
//
// `posse status` and the cockpit are separate processes with no ledger to ask,
// so their wall-clock rows keep the wall reading: the exposure is one witness
// tick of `loop-mute` after a wake and `guard-blind` until the first
// successful plan read, both bounded, measured and accepted in D4. The store
// that would close it is named there and waits on a bead nobody has filed.
func TestGovStatusInputsCarriesNeitherWatchProcessReading(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	in := StatusInputs(b.App, b, new(bytes.Buffer))
	if in.Suspended != nil {
		t.Error("StatusInputs carries a suspend witness: a one-shot process did not see the sleep, and a " +
			"reading invented from no ledger would subtract a suspend that is not known to have happened " +
			"(ADR 0064 D4 — the wall reading is the accepted cost, and a store is the first build)")
	}
	if !in.GuardTrippedSince.IsZero() {
		t.Error("StatusInputs carries a guard streak: a fresh shell has no streak and must report no G4")
	}
}

// The other end: the pulse's inputs DO carry it, which is what makes the
// subtraction reachable at all. The witness runs in this process and
// pulseOnce has already read the pair for this tick (ADR 0064 D3), so a
// govInputs that left the field nil would be a ledger nobody asks.
//
// MUTATION: delete `Suspended: d.suspendedSince` from govInputs → this reds.
func TestPulseGovInputsCarryTheSuspendWitness(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	in := d.govInputs(PulseConfig{Armed: true, Persona: "coordinator"})
	if in.Suspended == nil {
		t.Fatal("the pulse's inputs carry no suspend witness: the ledger is in this process and nothing else " +
			"can subtract it, so every wall-clock row after a long sleep is a fault the shop did not have " +
			"(ADR 0064 D2)")
	}
	// Wired to the LEDGER and not to a zero: a field set to a func that
	// always answers zero would pass the nil check and subtract nothing.
	back := govNow
	d.mu.Lock()
	d.suspends = []suspendSpan{{back: back, slept: time.Hour}}
	d.mu.Unlock()
	if got := in.Suspended(back.Add(-2 * time.Hour)); got != time.Hour {
		t.Errorf("the pulse's Suspended answered %s for a window holding a 1h span: it is not reading the "+
			"witness's ledger", got)
	}
}

// ─── G8 · paused ─────────────────────────────────────────────────────────────

// Reported, never alarmed — and the line names the pauser and the why,
// which is the whole reason the file shape makes both mandatory.
func TestGovG8Paused(t *testing.T) {
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.StateDir, 0o755)
	os.WriteFile(PausePath(b.App), []byte("by: coordinator\nat: 2026-08-27T09:00:00Z\nwhy: waiting on the operator\n"), 0o644)
	g := find(shopSet(t, govIn(t, b)), "G8")
	if g == nil || g.Key != "paused" || g.Class != GovUrgent {
		t.Fatalf("G8 = %+v, want paused URGENT", g)
	}
	if !strings.Contains(g.Detail, "coordinator") || !strings.Contains(g.Detail, "waiting on the operator") {
		t.Errorf("the pause line must name who and why: %q", g.Detail)
	}
}

// A pause file that parses to nothing is still a pause: somebody put it
// there, and declining to see it because a field is missing would be the
// surface deciding a malformed stop is no stop.
func TestGovG8MalformedPauseIsStillAPause(t *testing.T) {
	b, _ := newTestBackend(t)
	os.MkdirAll(b.App.StateDir, 0o755)
	os.WriteFile(PausePath(b.App), []byte("# nothing parseable\n"), 0o644)
	if g := find(shopSet(t, govIn(t, b)), "G8"); g == nil {
		t.Error("a pause file with no fields is still a pause")
	}
}

// ─── G9 · ready work routed to the coordinator ───────────────────────────────

func TestGovG9ReadyBeadOnTheCoordinator(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	appendConfig(t, b.App, "coordinator: coordinator\n")
	writeJSON(t, dir, "fake-ready.json", []map[string]any{
		{"id": "bd-c", "status": "open", "assignee": "coordinator", "title": "triage"},
		{"id": "bd-d", "status": "open", "assignee": "developer", "title": "code"},
	})
	set := shopSet(t, govIn(t, b))
	g := find(set, "G9")
	if g == nil || g.Key != "coordinator:bd-c" || g.Class != GovLane {
		t.Fatalf("G9 = %+v, want coordinator:bd-c LANE", g)
	}
	for _, k := range set.Keys() {
		if k == "coordinator:bd-d" {
			t.Error("a bead on a real lane is not G9")
		}
	}
}

// The refusal it mirrors compares IDENTITY, not the string: `Coordinator` and
// `./coordinator` reach the same PID, and a G9 that compared strings would go
// quiet on exactly the spellings that walk past dispatch's refusal.
func TestGovG9MatchesTheRefusalsIdentityCompare(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	appendConfig(t, b.App, "coordinator: coordinator\n")
	writeJSON(t, dir, "fake-ready.json", []map[string]any{
		{"id": "bd-c", "status": "open", "assignee": "Coordinator", "title": "triage"},
	})
	if g := find(shopSet(t, govIn(t, b)), "G9"); g == nil {
		t.Error("a case-shifted coordinator name is still the coordinator")
	}
}

// No coordinator configured is the pre-0033 shop: dispatch refuses nobody,
// so nothing is stuck.
func TestGovG9NoCoordinatorNoRow(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	writeJSON(t, dir, "fake-ready.json", []map[string]any{
		{"id": "bd-c", "status": "open", "assignee": "coordinator", "title": "triage"},
	})
	if g := find(shopSet(t, govIn(t, b)), "G9"); g != nil {
		t.Errorf("no coordinator: means no G9: %+v", *g)
	}
}

// ─── partial sets ────────────────────────────────────────────────────────────

// A store that could not be read makes the set PARTIAL, and a partial set
// is not an all-clear. This is the property `posse status`'s exit code and
// the cockpit's "partial" heading both rest on.
func TestGovUnreadableStoreIsNotAnAllClear(t *testing.T) {
	b, _ := newTestBackend(t)
	dir := govRepo(t, b)
	os.WriteFile(filepath.Join(dir, "fake-ready-fail"), nil, 0o644)
	appendConfig(t, b.App, "coordinator: coordinator\n")
	set, failed := ShopCheck(govIn(t, b))
	if len(failed) == 0 {
		t.Fatalf("a bd scan that failed must come back as an error, got set=%v", set.Keys())
	}
}

// Missing bd is the same shape: three rows are bd's, and reporting them as
// clear because the binary is absent is the silence this surface ends.
func TestGovMissingBdIsUnknownNotClear(t *testing.T) {
	b, _ := newTestBackend(t)
	writeBeadsDirs(b.App, []string{t.TempDir()})
	in := govIn(t, b)
	in.Bd = Bd{Bin: "/nonexistent/bd"}
	_, failed := ShopCheck(in)
	if len(failed) == 0 {
		t.Error("no bd means G2/G3/G9 are unknown, not clear")
	}
}

// ─── the streak clock (G4's half that lives in the watch process) ────────────

// The clock starts on the first tripped pass, survives the ones after it,
// and is cleared by the first pass that does not trip. That last arm is the
// one that matters: a streak nothing clears turns a guard that recovered
// into a permanent URGENT.
func TestGuardStreakStartsAndClears(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	at := govNow
	d.Now = func() time.Time { return at }

	d.planTrip = "plan burst at 78% > 70%"
	d.noteGuardStreak()
	first := d.guardStreak()
	if first.IsZero() {
		t.Fatal("a tripped pass must start the streak")
	}

	at = at.Add(30 * time.Minute)
	d.noteGuardStreak()
	if got := d.guardStreak(); !got.Equal(first) {
		t.Errorf("a second tripped pass must not restart the clock: %v vs %v", got, first)
	}

	d.planTrip = ""
	d.noteGuardStreak()
	if got := d.guardStreak(); !got.IsZero() {
		t.Errorf("a pass that did not trip must clear the streak, got %v", got)
	}
}

// ─── the renderings ──────────────────────────────────────────────────────────

// One computation, three renderings — and the fingerprint is the KEYS, so a
// condition changing class or wording never re-prompts the coordinator.
func TestGovFingerprintIsKeysOnly(t *testing.T) {
	t.Parallel()
	a := GovSet{{ID: "G1", Class: GovLane, Key: "blocked:x", Detail: "one wording"}}
	c := GovSet{{ID: "G1", Class: GovUrgent, Key: "blocked:x", Detail: "another wording entirely"}}
	if a.Fingerprint() != c.Fingerprint() {
		t.Errorf("the fingerprint must not move on detail/class: %q vs %q", a.Fingerprint(), c.Fingerprint())
	}
	if GovLines(a) != "blocked:x" {
		t.Errorf("GovLines = %q", GovLines(a))
	}
}

// URGENT first: the split is the only ranking the set has, and a human
// reading the top three lines must be reading the three that stopped the
// shop.
func TestGovReportUrgentFirst(t *testing.T) {
	t.Parallel()
	set := GovSet{
		{ID: "G1", Class: GovLane, Key: "blocked:a", Detail: "lane a"},
		{ID: "G7", Class: GovUrgent, Key: "loop-dead", Detail: "urgent one"},
	}
	var buf bytes.Buffer
	GovReport(&buf, set, nil)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "URGENT") {
		t.Errorf("report:\n%s", buf.String())
	}
	if GovSummary(set) != "1 URGENT · 1 LANE" {
		t.Errorf("summary = %q", GovSummary(set))
	}
}

// An empty set with an unread store must not print the all-clear.
func TestGovReportEmptyButPartial(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	GovReport(&buf, nil, []error{Die("bd: database is locked")})
	if strings.Contains(buf.String(), "nothing needs a human") {
		t.Errorf("a partial read is not an all-clear:\n%s", buf.String())
	}
	var clear bytes.Buffer
	GovReport(&clear, nil, nil)
	if !strings.Contains(clear.String(), "nothing needs a human") {
		t.Errorf("a clean read says so:\n%s", clear.String())
	}
}

// A carry-over has no G-row and must not be given one. Not because the
// enumeration is closed (ADR 0029's 2026-09-05 simplification retired that,
// and G10 landed under the bar it set instead) but because a row name is a
// promise that the ADR's table describes the condition.
func TestGovCarryOverHasNoRowName(t *testing.T) {
	t.Parallel()
	if got := (GovCondition{Key: "no-live:coordinator"}).Row(); got != "—" {
		t.Errorf("Row() = %q, want the em dash", got)
	}
}
