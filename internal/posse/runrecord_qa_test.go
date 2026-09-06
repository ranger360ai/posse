//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0011 §3: the session meta is the run record. Three claims, one per
// half of the acceptance criterion — `bead:`/`prompted:` are persisted, the
// grace they carry holds across processes, and the holder join is a lookup
// in that record rather than an inference from a session name.
//
// The pass↔pass half of the grace is pinned next door, in
// launchlock_qa_test.go :: TestTwoPassesDoNotDoubleClaimOneBead (qa's
// repro, rangerhq-o2ki). These pin the pair the ADR names in prose — "the
// cockpit's `d` and a running pass cannot see each other's prompts" — in
// both directions, because they are different code paths: `d` is LaunchBead
// and a pass is fireLoop, and only one of the two ever consulted the grace.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaRunRecordPass dispatches the one bead of a fresh one-bead repo and
// returns the repo, the session that got it, and the bead as the cockpit
// would hand it to LaunchBead: a row read BEFORE the pass claimed it, which
// is the stale reading every guard but this one abstains on.
func qaRunRecordPass(t *testing.T, b *HerdrBackend, fake string) (string, string, RepoIssue) {
	t.Helper()
	agentPerLaunch(t, fake)
	repo := qaOneBeadRepo(t, b.App)
	d := newTestDispatcher(t, b)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	session := SessionForBead("alpha", repo, "a-1")
	if !strings.Contains(calls(t, fake), "workspace create --label "+session) {
		t.Fatalf("the first pass created no session for a-1:\n%s", dispatcherOut(d))
	}
	return repo, session, RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"}}, Dir: repo}
}

// The record is on disk, not in a process. `bead:` says which bead the
// session was created for and `prompted:` when its work prompt was sent —
// the two fields ADR 0011 §3 adds, and the reason anything below can read
// what another launcher did.
func TestRunRecordPersistsBeadAndPrompted(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	_, session, _ := qaRunRecordPass(t, b, fake)

	raw, err := os.ReadFile(filepath.Join(b.App.StateDir, "herdr", session+".yaml"))
	if err != nil {
		t.Fatalf("no run record for %s: %v", session, err)
	}
	if !strings.Contains(string(raw), "bead: a-1\n") {
		t.Errorf("the record does not say which bead it is a run of:\n%s", raw)
	}
	if !strings.Contains(string(raw), "prompted: ") {
		t.Errorf("the record does not say when it was prompted:\n%s", raw)
	}

	m, ok := b.readMeta(session)
	if !ok {
		t.Fatalf("readMeta(%s) failed", session)
	}
	// Parsed, not merely present: a `prompted:` that reads back as the zero
	// time is a line in a file, not a fact, and every reader of it would
	// silently fall through to the per-process memory this replaces.
	if m.Prompted.IsZero() {
		t.Errorf("prompted: does not parse back: %+v", m)
	}
	// The prompt follows the launch — they are different moments, and the
	// grace is measured from the later one. A session resumed without being
	// relaunched moves only this field.
	if m.Prompted.Before(m.Launched) {
		t.Errorf("prompted %v is before launched %v", m.Prompted, m.Launched)
	}
}

// The ADR's own example: a pass prompts, and the cockpit's `d` — a
// different process, so an empty `lastPrompt` — must refuse inside the
// grace. Before the record this refusal simply did not happen: `d` looked
// at herdr, saw the freshly launched agent reported idle, and prompted the
// same bead into the same session a second time.
func TestCockpitResumeRefusesAPassesFreshPrompt(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	_, session, is := qaRunRecordPass(t, b, fake)

	before := strings.Count(calls(t, fake), "agent prompt ")
	d2 := newTestDispatcher(t, b) // a second process's memory: empty
	d2.Resume = true
	got, err := d2.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "prompted") {
		t.Errorf("`d` inside the grace was not refused: session=%q err=%v", got, err)
	}
	if n := strings.Count(calls(t, fake), "agent prompt ") - before; n != 0 {
		t.Errorf("`d` sent %d more prompt(s) to %s inside the grace", n, session)
	}
}

// The other direction, and the one the lock alone could not close: a pass
// whose ready list predates another launcher's claim. Every guard fireLoop
// had reads a store that launcher has not moved yet — `busy` is per-pass,
// `personaActive` reads the fresh agent as idle, the in_progress check reads
// the stale row — so the record is the only store that can answer, and the
// second pass must skip rather than claim and prompt a bead already running.
func TestSecondPassSkipsABeadAnotherLauncherJustPrompted(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	repo, session, _ := qaRunRecordPass(t, b, fake)

	// The stale reading, restored by hand: `Run` gathers its ready list
	// BEFORE fireLoop takes the launcher lock (ADR 0011 §1), so the pass
	// that waits fires from rows the holder had not claimed yet. Left as
	// the first pass left it, the row would read in_progress and the
	// held-bead guard would answer — the right answer for the wrong reason,
	// and the reason is what is under test. So the bead's own row goes back
	// to open, and so does the fake's claim state, which is what `ready`
	// overlays onto that row.
	os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"a-1","title":"t","labels":["go"]}]`), 0o644)
	stale, _ := filepath.Glob(filepath.Join(fake, "bd-state-*.json"))
	if len(stale) == 0 {
		t.Fatal("the first pass recorded no claim state — this test would pass vacuously")
	}
	for _, p := range stale {
		os.Remove(p)
	}

	before := strings.Count(calls(t, fake), "agent prompt ")
	beforeClaims := strings.Count(bdCalls(t, fake), "--claim")
	d2 := newTestDispatcher(t, b)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)
	if !strings.Contains(out, "was prompted") {
		t.Errorf("the second pass did not name the fresh prompt as its reason:\n%s", out)
	}
	if n := strings.Count(calls(t, fake), "agent prompt ") - before; n != 0 {
		t.Errorf("%s was prompted %d more time(s) by a pass firing from a stale list", session, n)
	}
	if n := strings.Count(bdCalls(t, fake), "--claim") - beforeClaims; n != 0 {
		t.Errorf("the bead was claimed %d more time(s)", n)
	}
}

// The holder join as a LOOKUP (ADR 0011 §3). The holder here wears a name
// that matches NEITHER pattern the join used to walk — not the bead's Dial
// F name, not the pre-Dial-F slot — so a name-pattern join sees an unheld
// bead and launches a twin beside a working agent, which is the lwx/v330
// class. Its record says `bead: a-1`, and that is what dispatch wrote.
func TestHolderJoinReadsTheRecordNotTheName(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := t.TempDir()

	const holder = "renamed-by-hand" // neither SessionForBead nor SessionFor
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"working","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)
	mustCreate(t, b, NewSessionOpts{Name: holder, Dir: repo, Agent: "ranger", Bead: "a-1"})
	ws := fakeLoadWSFrom(t, fake)
	for i := range ws {
		ws[i].AgentStatus = "working"
	}
	saveWSTo(t, fake, ws)

	if s, ok := b.RunHolder(repo, "ranger", "a-1"); !ok || s.Name != holder {
		t.Fatalf("RunHolder did not find the record's session: %v %v", s, ok)
	}
	// Another persona's session is not this join's answer, whatever the
	// record says about the bead: prompting it would be a reassignment.
	if s, ok := b.RunHolder(repo, "scout", "a-1"); ok {
		t.Errorf("RunHolder crossed personas: %+v", s)
	}
	// And a bead id is only unique inside its repo.
	if s, ok := b.RunHolder(t.TempDir(), "ranger", "a-1"); ok {
		t.Errorf("RunHolder crossed repos: %+v", s)
	}

	is := RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "t", Labels: []string{"go"},
		Status: "in_progress", Assignee: "ranger"}, Dir: repo}
	d.Resume = true
	session, err := d.LaunchBead(is)
	if err == nil || !strings.Contains(err.Error(), "working") {
		t.Errorf("the working holder was not found by its record: session=%q err=%v", session, err)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create --label "+SessionForBead("ranger", repo, "a-1")) {
		t.Errorf("a twin was created beside the record's holder:\n%s", log)
	}
}
