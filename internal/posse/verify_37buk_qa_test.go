package posse

// QA pins written verifying four closes under ranger-base-37buk
// (ranger-base-p6no, ranger-base-qxwd, ranger-base-gs9r, ranger-base-qmjc).
// Each one is a gap the shipped pins left, found by mutating the fix and
// watching the suite stay green.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ─── ranger-base-92n5p: the launcher still wedges on a FIFO ──────────────────

// ranger-base-gs9r taught l3Identity and identityMatch to stat before they
// read, and probeL3Hooks does return over a FIFO at the dispatch path now.
// The launch does not: planLaunch calls InstallCommitGuardHook on the very
// next line (herdrback.go), installHook reads the same slot with no type
// check, and os.ReadFile on a FIFO with no writer never returns. One mkfifo
// against a checkout the fleet launches into is still a launch that hangs
// with nothing printed — gs9r's own WHY IT MATTERS, one function further
// down.
//
// Un-skip when ranger-base-92n5p lands. Every arm below carries its own
// control in the same rig, so a BLOCKED verdict is the file type and not the
// harness — and the controls hold on both sides of the fix.
func TestQAAFifoAtTheDispatchPathMustNotWedgeTheLaunch(t *testing.T) {
	t.Skip("ranger-base-92n5p: installHook and hookInstalled read <hooks>/<slot> before any type check — the launch blocks forever on a FIFO")
	for _, mode := range []os.FileMode{0o644, 0o755} {
		a, repo := fifoLaunchRig(t)
		if !returnsWithin(t, 30*time.Second, func() { a.probeL3Hooks(repo, true) }) {
			t.Fatalf("CONTROL: probeL3Hooks blocked with no FIFO planted — the rig, not the code")
		}
		plantFifo(t, repo, "prepare-commit-msg", mode)
		if !returnsWithin(t, 30*time.Second, func() { a.probeL3Hooks(repo, true) }) {
			t.Errorf("probeL3Hooks blocked on a %04o FIFO (ranger-base-gs9r's own fix)", mode)
		}
		if !returnsWithin(t, 30*time.Second, func() { a.InstallCommitGuardHook(repo) }) {
			t.Errorf("InstallCommitGuardHook blocked on a %04o FIFO at the dispatch path", mode)
		}
		if !returnsWithin(t, 30*time.Second, func() {
			hookInstalled(repo, "prepare-commit-msg", sharedIndexMarker, legacySharedIndexMarker)
		}) {
			t.Errorf("hookInstalled blocked on a %04o FIFO at the dispatch path", mode)
		}
	}

	// The pre-push slot is the same read through the same function.
	a, repo := fifoLaunchRig(t)
	plantFifo(t, repo, "pre-push", 0o755)
	if !returnsWithin(t, 30*time.Second, func() { InstallPrePushHook(repo) }) {
		t.Errorf("InstallPrePushHook blocked on a FIFO at the dispatch path")
	}

	// And the chained member behind our own dispatcher: probeL3Hooks does ask
	// isRegularFile there, installHook does not.
	a, repo = fifoLaunchRig(t)
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"),
		[]byte(chainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(hooks, "posse-prepare-commit-msg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !returnsWithin(t, 30*time.Second, func() { a.probeL3Hooks(repo, true) }) {
		t.Errorf("probeL3Hooks blocked on a FIFO at the chained posse-<slot> member")
	}
	if !returnsWithin(t, 30*time.Second, func() { a.InstallCommitGuardHook(repo) }) {
		t.Errorf("InstallCommitGuardHook blocked on a FIFO at the chained posse-<slot> member")
	}

	// End to end, the symptom the bead is named for: a launch that never
	// returns. The control launch first, over the same fixture with an
	// ordinary slot, so a blocked launch below is the FIFO.
	b, clean := lhpFixture(t, VisibilityPrivate)
	if !returnsWithin(t, 90*time.Second, func() {
		b.planLaunch(NewSessionOpts{Name: "s0", Dir: clean, Agent: "ranger"})
	}) {
		t.Fatalf("CONTROL: planLaunch blocked over an ordinary hook wall — the rig, not the code")
	}
	b2, wedged := lhpFixture(t, VisibilityPrivate)
	if err := os.Remove(filepath.Join(wedged, ".git", "hooks", "prepare-commit-msg")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	plantFifo(t, wedged, "prepare-commit-msg", 0o644)
	if !returnsWithin(t, 60*time.Second, func() {
		b2.planLaunch(NewSessionOpts{Name: "s1", Dir: wedged, Agent: "ranger"})
	}) {
		t.Errorf("planLaunch blocked forever on a 0644 FIFO at the dispatch path — the launcher still wedges")
	}
}

// fifoLaunchRig is a home and a git checkout with an empty beads_visibility:
// map — the smallest thing probeL3Hooks and installHook both answer for.
func fifoLaunchRig(t *testing.T) (*App, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHQ_HOME", home)
	t.Setenv(EnvPersona, "")
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads_visibility:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := hwsRepo(t, root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	return NewAppAt(home), repo
}

func plantFifo(t *testing.T, repo, slot string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(repo, ".git", "hooks", slot)
	_ = os.Remove(p)
	if err := syscall.Mkfifo(p, uint32(mode)); err != nil {
		t.Fatalf("mkfifo %s: %v", p, err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	// The fixture's own witness, and it holds on both sides of the fix: a
	// pin over a plain file would pass a broken build.
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("fixture at %s is not a FIFO: mode %v (%v)", p, fi.Mode(), err)
	}
}

// returnsWithin reports whether f returned before d elapsed. A blocked f is
// left running — it is parked on a read that never completes, which is the
// thing under test.
func returnsWithin(t *testing.T, d time.Duration, f func()) bool {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); f() }()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// ─── ranger-base-p6no: the resume row the shipped pin does not reach ─────────

// ADR 0020 §2 (amended): an in_progress bead WITH an assignee is a lane of
// one, and `d` there is a resume that never asks the availability question —
// "in_progress assigned, holder idle, persona busy on another bead → still
// resumes the holder (behaviour preserved)".
//
// TestLaunchBeadResumesAnIdleHolderEvenWhenThePersonaIsBusyElsewhere pins
// that row over a bead its own fixture has already launched, so the bead
// carries a RUN RECORD and the unassigned arm's RunHolder walk answers with
// the same persona. Measured: deleting the assignee branch outright
// (`if false {`) leaves every LaunchBead pin green. Without a run record the
// two arms part company — the availability walk sees the persona working
// another bead and refuses lane-busy — and that is the row here.
func TestQALaunchBeadResumesAnAssignedHolderWithNoRunRecord(t *testing.T) {
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "developer", "[code]")
	repo := t.TempDir()
	agentPerLaunch(t, fake)

	d1 := newTestDispatcher(t, b)
	seed := RepoIssue{BdIssue: BdIssue{ID: "seed", Title: "s", Assignee: "developer"}, Dir: repo}
	if _, err := d1.LaunchBead(seed); err != nil {
		t.Fatal(err)
	}
	fakeAgentStatuses(t, fake, map[string]string{
		SessionForBead("developer", repo, "seed"): "working",
	})

	// a-9 has never been launched: no run record, no live holder session.
	d2 := newTestDispatcher(t, b)
	d2.PromptGrace = 0
	is := RepoIssue{BdIssue: BdIssue{ID: "a-9", Title: "t", Status: "in_progress", Assignee: "developer"}, Dir: repo}
	session, err := d2.LaunchBead(is)
	if err != nil {
		t.Fatalf("an in_progress bead assigned to a persona busy elsewhere, with no run record, must still resume its holder rather than ask the availability question: %v", err)
	}
	if want := SessionForBead("developer", repo, "a-9"); session != want {
		t.Errorf("want the holder %s, got %s", want, session)
	}
}

// ─── ranger-base-qxwd: the stamp pin that does not measure the stamp ─────────

// qxwd's DONE WHEN wanted a repository whose installed stamp disagrees with
// config named by a command that does not require standing in it, "with a
// regression test for BOTH directions". SweepHookWall does that — but
// TestHookWallSweepCatchesAStampThatDisagreesWithConfig does not measure it:
// its fixture plants CommitGuardHook(visibility, set) with NO identity
// literals, while the installer and probeL3Hooks both render
// CommitGuardHook(visibility, set, DeriveIdentityLiterals(...)...). The
// planted body therefore differs from the reference whatever the stamp says.
// Measured two ways: planting the CORRECT visibility (still no literals) is
// reported as a finding just the same, and a mutant that ignores config
// outright (`visibility = VisibilityPrivate` in probeL3Hooks) leaves that
// test green.
//
// So this one plants the installer's own render — literals included — and
// varies ONLY the stamp, in both directions, each against a same-render
// control that must come back clean.
func TestQAHookWallSweepStampDriftBothDirectionsOverTheInstallersOwnRender(t *testing.T) {
	a, dirs := hwsFixture(t, map[string]string{"priv": VisibilityPrivate}, "priv")
	repo := dirs["priv"]
	slot := hwsHook(t, repo, "prepare-commit-msg")
	id, err := DeriveIdentityLiterals(hookRepo(repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(id) == 0 {
		t.Fatal("fixture derives no identity literals — the arms below would differ for the shipped test's reason, not for the stamp")
	}
	plant := func(vis string) {
		t.Helper()
		if err := os.WriteFile(slot, []byte(CommitGuardHook(vis, a.OpsPatternSet(), id...)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	setConfig := func(vis string) {
		t.Helper()
		if err := os.WriteFile(a.ConfigPath, []byte("beads_visibility:\n  "+repo+": "+vis+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if v, _ := a.BeadsVisibility(repo); v != vis {
			t.Fatalf("fixture config did not take: visibility is %q, want %q", v, vis)
		}
	}

	for _, c := range []struct{ config, stamp string }{
		// The loud direction: an ops-class refusal where config says none.
		{VisibilityPrivate, VisibilityPublic},
		// The silent one, and the one that matters more: the guard is off in
		// a repository the operator has since declared public.
		{VisibilityPublic, VisibilityPrivate},
	} {
		setConfig(c.config)

		// The control first: the same render, the stamp config asks for.
		// Without it a sweep that reports every planted hook would pass the
		// arm below for the wrong reason — which is exactly what happened.
		plant(c.config)
		if out, found := hwsReport(t, a, "pin"); found {
			t.Fatalf("CONTROL: config %s carrying the %s render was reported as drifted:\n%s", c.config, c.config, out)
		}

		plant(c.stamp)
		out, found := hwsReport(t, a, "pin")
		if !found {
			t.Errorf("config %s carrying a %s-stamped render passed the sweep — only the stamp differs:\n%s", c.config, c.stamp, out)
		}
		if !strings.Contains(out, repo) || !strings.Contains(out, "ours but stale") {
			t.Errorf("config %s / stamp %s: the finding does not name the repository and why:\n%s", c.config, c.stamp, out)
		}
	}
}
