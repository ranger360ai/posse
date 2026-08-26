package rhq

// End-to-end tests (over the fake herdr/bd) for the end-of-pass auto-reap
// (rangerhq-us8): dispatch.go's own doc says a finished bead's session is
// "left idle for the operator or --watch to reap" — these pin the predicate
// that now applies automatically, and the guards that keep it from being
// the reap guard's own near-miss all over again.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reapCandidate is a per-bead session in a plain (non-worktree) checkout of
// its own, with a `bd show` answer a test can flip independently of every
// other session in the same test.
func reapCandidate(t *testing.T, b *HerdrBackend, name, bead, beadStatus string) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"`+bead+`","status":"`+beadStatus+`"}]`), 0o644)
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: dir, Agent: "ranger", Bead: bead}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAutoReapKillsAClosedIdleSession(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)

	d.autoReapPass(nil)

	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("a session whose bead is closed and whose agent is idle must be reaped")
	}
	if !strings.Contains(dispatcherOut(d), "reaped ranger-repo-a-1 (bead a-1 closed)") {
		t.Errorf("reap must say what it did, got:\n%s", dispatcherOut(d))
	}
}

// "done" is the other settled state the fire loop itself treats as
// finished (AgentWait's own idle/done/blocked triad) — it must reap too.
func TestAutoReapKillsAClosedDoneSession(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"done","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)

	d.autoReapPass(nil)

	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("agent_status done is a settled state too — the session must be reaped")
	}
}

func TestAutoReapKeepsAClosedWorkingSession(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	workingClaude(t, fake)

	d.autoReapPass(nil)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("a session herdr still calls working must not be reaped, however its bead reads")
	}
}

func TestAutoReapKeepsAnOpenIdleSession(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "in_progress")
	idleClaude(t, fake)

	d.autoReapPass(nil)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("an idle session whose bead is still open must not be reaped — the persona stopped on it, and gather's own line is what raises that, not the reaper")
	}
}

func TestAutoReapKeepsACrewSession(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake-show.json"), []byte(`[{"id":"a-1","status":"closed"}]`), 0o644)
	if err := b.CreateSession(NewSessionOpts{Name: "ranger-crew", Dir: dir, Agent: "ranger", Bead: "a-1", Crew: true}); err != nil {
		t.Fatal(err)
	}
	idleClaude(t, fake)

	d.autoReapPass(nil)

	if _, ok := b.readMeta("ranger-crew"); !ok {
		t.Error("ADR 0008: a crew session is never reaped, closed bead or not")
	}
}

// The pre-Dial-F persona slot keeps a bead pointer across a resume
// (NoteBead) but carries no bead suffix of its own — it is the persona's
// reusable session, and never Dial F's to reap.
func TestAutoReapKeepsTheNonPerBeadSlot(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake-show.json"), []byte(`[{"id":"a-1","status":"closed"}]`), 0o644)
	slot := SessionFor("ranger", dir)
	if err := b.CreateSession(NewSessionOpts{Name: slot, Dir: dir, Agent: "ranger", Bead: "a-1"}); err != nil {
		t.Fatal(err)
	}
	idleClaude(t, fake)

	d.autoReapPass(nil)

	if _, ok := b.readMeta(slot); !ok {
		t.Error("the persona's own repo slot (no bead suffix) must never be reaped")
	}
}

func TestAutoReapDryRunOnlyLists(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)

	d.autoReapPass(nil)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("--dry-run must not actually kill anything")
	}
	if !strings.Contains(dispatcherOut(d), "would reap ranger-repo-a-1") {
		t.Errorf("--dry-run must say what it would reap, got:\n%s", dispatcherOut(d))
	}
}

func TestAutoReapOffByConfig(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)
	os.WriteFile(b.App.ConfigPath, []byte("auto_reap: false\n"), 0o644)

	d.autoReapPass(nil)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("auto_reap: false must turn the sweep off entirely (today's behaviour)")
	}
}

func TestAutoReapOffByFlag(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.NoReap = true
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)

	d.autoReapPass(nil)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("--no-reap must turn the sweep off for this pass")
	}
}

// A bead this pass itself just prompted is never a reap candidate on that
// same pass, however herdr and bd read it right now — the settle race that
// PromptGrace exists for elsewhere. It is fair game on the pass after,
// once it is no longer this pass's own prompt.
func TestAutoReapSkipsASessionThisPassJustPrompted(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	if n, err := d.Run("", "", 0); err != nil || n != 1 {
		t.Fatalf("dispatched %d, err=%v:\n%s", n, err, dispatcherOut(d))
	}
	session := SessionForBead("ranger", repo, "a-1")
	if _, ok := b.readMeta(session); !ok {
		t.Fatal("the session this pass just prompted must still exist right after that same pass")
	}
	if strings.Contains(dispatcherOut(d), "reaped "+session) {
		t.Errorf("a session this pass just prompted must not be reaped in the same pass:\n%s", dispatcherOut(d))
	}

	// A later pass: the bead is closed and no longer ready, and this
	// session was not this pass's own prompt — it is fair game now.
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(`[]`), 0o644)
	d2 := newTestDispatcher(t, b)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if _, ok := b.readMeta(session); ok {
		t.Error("the session should have been reaped on the pass after the one that prompted it")
	}
	if !strings.Contains(dispatcherOut(d2), "reaped "+session) {
		t.Errorf("expected a reap line on the later pass, got:\n%s", dispatcherOut(d2))
	}
}

// The steady state is zero ready beads — that pass must sweep too, not
// only the passes that happen to dispatch something.
func TestAutoReapRunsOnAQuietPass(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[]`, "")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("a pass with no ready work must still sweep closed-idle sessions")
	}
	if !strings.Contains(dispatcherOut(d), "reaped ranger-repo-a-1") {
		t.Errorf("expected a reap line on the quiet pass, got:\n%s", dispatcherOut(d))
	}
}
