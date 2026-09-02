package posse

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
	"time"
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
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

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
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	os.WriteFile(filepath.Join(fake, "agents.json"),
		[]byte(`[{"agent":"claude","agent_status":"done","pane_id":"w1:p1","workspace_id":"w1"}]`), 0o644)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("agent_status done is a settled state too — the session must be reaped")
	}
}

func TestAutoReapKeepsAClosedWorkingSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	workingClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("a session herdr still calls working must not be reaped, however its bead reads")
	}
}

func TestAutoReapKeepsAnOpenIdleSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "in_progress")
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("an idle session whose bead is still open must not be reaped — the persona stopped on it, and gather's own line is what raises that, not the reaper")
	}
}

// ADR 0008's shield over the session the operator MADE — the half of it that
// ranger-base-f6lk left untouched. The name is an operator's, not the one
// dispatch would have rendered for this persona, dir and bead, so no grace
// applies and no age reaches it: the crew arm added by f6lk takes only a
// session dispatch itself created (reapresidue_test.go pins that arm and
// this boundary from the other side).
func TestAutoReapKeepsACrewSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake-show.json"), []byte(`[{"id":"a-1","status":"closed"}]`), 0o644)
	if err := b.CreateSession(NewSessionOpts{Name: "ranger-crew", Dir: dir, Agent: "ranger", Bead: "a-1", Crew: true}); err != nil {
		t.Fatal(err)
	}
	idleClaude(t, fake)
	// Older than any grace in the file, so the pin is the SHAPE and not the
	// clock: a fresh session would survive this sweep either way.
	ageLaunch(t, b, "ranger-crew", 30*24*time.Hour)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-crew"); !ok {
		t.Error("ADR 0008: a crew session the operator made is never reaped, closed bead or not, at any age")
	}
}

// The pre-Dial-F persona slot keeps a bead pointer across a resume
// (NoteBead) but carries no bead suffix of its own — it is the persona's
// reusable session, and never Dial F's to reap.
func TestAutoReapKeepsTheNonPerBeadSlot(t *testing.T) {
	t.Parallel()
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

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta(slot); !ok {
		t.Error("the persona's own repo slot (no bead suffix) must never be reaped")
	}
}

func TestAutoReapDryRunOnlyLists(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("--dry-run must not actually kill anything")
	}
	if !strings.Contains(dispatcherOut(d), "would reap ranger-repo-a-1") {
		t.Errorf("--dry-run must say what it would reap, got:\n%s", dispatcherOut(d))
	}
}

func TestAutoReapOffByConfig(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)
	os.WriteFile(b.App.ConfigPath, []byte("auto_reap: false\n"), 0o644)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("auto_reap: false must turn the sweep off entirely (today's behaviour)")
	}
}

func TestAutoReapOffByFlag(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.NoReap = true
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("--no-reap must turn the sweep off for this pass")
	}
}

// agePrompt moves a session's `Prompted:` record back, which is the only
// way a test can say "and then some time passed": since ADR 0028 §3 the
// reap's prompt guard reads that record (PromptGrace over `promptedRecently`)
// rather than a set of names this pass fired at, so a later PASS is no
// longer automatically a later PROMPT. MarkPrompted refuses to move the
// stamp backwards — it is a high-water mark — so this writes the record.
func agePrompt(t *testing.T, b *HerdrBackend, session string, by time.Duration) {
	t.Helper()
	m, ok := b.readMeta(session)
	if !ok {
		t.Fatalf("no run record for %s to age", session)
	}
	m.Prompted = m.Prompted.Add(-by)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
}

// A bead just prompted is never a reap candidate, however herdr and bd read
// it right now — the settle race that PromptGrace exists for elsewhere. It
// is fair game once that grace has passed.
//
// The guard is PromptGrace over the run record since ADR 0028 §3, so both
// halves are stronger than the pass-scoped set they replace: the same pass
// is covered because it just prompted, and so is a prompt from a launcher
// this dispatcher never shared memory with (the second pass below runs on a
// fresh Dispatcher and still sees it).
func TestAutoReapSkipsASessionJustPrompted(t *testing.T) {
	t.Parallel()
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

	// A fresh dispatcher, standing in for a second launcher with none of
	// this one's memory: inside PromptGrace it must STILL refuse, because
	// the record is what it reads.
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte(`[]`), 0o644)
	dOther := newTestDispatcher(t, b)
	if _, err := dOther.Run("", "", 0); err != nil {
		t.Fatalf("cross-process pass: %v", err)
	}
	if _, ok := b.readMeta(session); !ok {
		t.Errorf("a session prompted seconds ago must survive ANOTHER launcher's sweep too (ADR 0028 §3):\n%s", dispatcherOut(dOther))
	}

	// A later pass, once the grace has passed: fair game.
	agePrompt(t, b, session, d.PromptGrace+time.Minute)
	d2 := newTestDispatcher(t, b)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if _, ok := b.readMeta(session); ok {
		t.Error("the session should have been reaped on the first pass past PromptGrace")
	}
	if !strings.Contains(dispatcherOut(d2), "reaped "+session) {
		t.Errorf("expected a reap line on the later pass, got:\n%s", dispatcherOut(d2))
	}
}

// The steady state is zero ready beads — that pass must sweep too, not
// only the passes that happen to dispatch something.
func TestAutoReapRunsOnAQuietPass(t *testing.T) {
	t.Parallel()
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

// The starvation fix (ranger-base-v674): a real pass with real beads
// gathers for 15m-4h, and every --watch instance on record so far has died
// somewhere in that window, before ever reaching the epilogue reap below.
// Proving the fix needs a pass that fails before it gets there — a
// launch-lock failure (the same shape TestLaunchLockFailureFailsThePass
// forces) is the cheapest way — and checking that a session a PREVIOUS pass
// left closed-idle was still reaped despite this pass's own failure.
func TestAutoReapSweepsAtPassStartEvenWhenThePassLaterFails(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	qaOneBeadRepo(t, b.App)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-0", "a-0", "closed")
	idleClaude(t, fake)

	// A directory at the lock path: fireLoop's O_RDWR on it cannot succeed,
	// so this pass never reaches gather or its own epilogue. The candidate
	// above was created through CreateSession, which now takes the launcher
	// lock itself (rangerhq-3a5t) and so left the lock FILE here — the
	// directory only goes where nothing is.
	os.Remove(LaunchLockPath(b.App))
	if err := os.MkdirAll(LaunchLockPath(b.App), 0o755); err != nil {
		t.Fatal(err)
	}

	d := newTestDispatcher(t, b)
	if _, err := d.Run("", "", 0); err == nil {
		t.Fatal("expected the launch lock failure to fail the pass")
	}
	if _, ok := b.readMeta("ranger-repo-a-0"); ok {
		t.Error("the start-of-pass sweep must reap a previous pass's closed session even when this pass fails before its own epilogue")
	}
	if !strings.Contains(dispatcherOut(d), "reaped ranger-repo-a-0") {
		t.Errorf("expected a reap line before the pass failed, got:\n%s", dispatcherOut(d))
	}
}
