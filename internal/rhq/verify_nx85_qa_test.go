package rhq

// What survived the attack on four closes (ranger-base-nx85), kept runnable.
// Each pin below is a question the close's own tests do not ask, found by
// trying to break the claim rather than by reading it:
//
//   rangerhq-lhy2  the close pins that nothing is TYPED when a late explain
//                  error lands on a window of guesses. It does not pin that
//                  the launch then fails LOUDLY naming the guess, which is
//                  the bead's EXPECTED, nor the other ordering (an error
//                  early, herdr back for the rest of the window).
//   ranger-base-ujdg  ADR 0023 stops exec'ing the file git dispatches and
//                  INFERS the wall from identity plus our own render. Every
//                  pin it added is about the inference; none runs real git.
//                  These two do: on a repo the probe certifies, git itself
//                  must refuse — including through the prescribed chain with
//                  a hostile neighbour behind it.
//   rangerhq-oaya  the close pins idempotence by composing
//                  EnsureUnattendedLine with itself; the path that actually
//                  re-plans a recorded line is RelaunchAgent.
//
// And one that did not survive: ranger-base-gs9r, skipped at the bottom.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ─── rangerhq-lhy2 ───────────────────────────────────────────────────────────

// A late explain error over a window of guesses must not merely refrain from
// typing — the launch has herdr's real answer and owes the operator the loud
// failure that names it. TestQAGuessesForTheWholeWindowAreLostToOneLateExplainError
// asserts only the silence; this asserts the diagnosis, so a fix that swapped
// Die() for a quiet return would still be caught.
func TestQALateExplainErrorStillFailsLoudlyNamingTheGuess(t *testing.T) {
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 900 * time.Millisecond
	d.Poll = 100 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644) // guess forever
	os.WriteFile(filepath.Join(fake, "error-on-stderr"), nil, 0o644)  // the real 0.8.0 shape

	done := make(chan struct{})
	go func() {
		time.Sleep(700 * time.Millisecond)
		os.WriteFile(filepath.Join(fake, "explain-error"), []byte("internal|no detection for w1:p1"), 0o644)
		close(done)
	}()
	defer func() { <-done }()

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "never saw a screen it recognizes") {
		t.Errorf("a window of guesses is a real answer and must fail loudly:\n%s", out)
	}
	if !strings.Contains(out, "default_known_agent_idle_fallback") {
		t.Errorf("herdr's own word for the guess is the diagnosis — say it:\n%s", out)
	}
	if strings.Contains(out, "prompting on its") {
		t.Errorf("the cannot-be-read concession fired over a window that WAS read:\n%s", out)
	}
}

// The other ordering, which the close does not cover: explain broken at the
// START of the window, herdr back for the rest of it. lastErr must not
// outlive a later real answer either — this is the arm that keeps the fix
// from being "an error anywhere in the window wins".
func TestQAAnEarlyExplainErrorDoesNotOutliveALaterGuess(t *testing.T) {
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 900 * time.Millisecond
	d.Poll = 100 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644)
	os.WriteFile(filepath.Join(fake, "error-on-stderr"), nil, 0o644)
	os.WriteFile(filepath.Join(fake, "explain-error"), []byte("internal|no detection for w1:p1"), 0o644)

	done := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Remove(filepath.Join(fake, "explain-error")) // herdr comes back
		close(done)
	}()
	defer func() { <-done }()

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("prompted at a screen herdr only ever guessed about:\n%s", dispatcherOut(d))
	}
	if out := dispatcherOut(d); !strings.Contains(out, "never saw a screen it recognizes") {
		t.Errorf("an early error must not survive the guesses that followed it:\n%s", out)
	}
}

// ─── ranger-base-ujdg / ADR 0023 ─────────────────────────────────────────────

// The inference, measured against the thing it is an inference ABOUT. ADR
// 0023's probe never runs the file git dispatches, so "identity holds and our
// render refuses" has to still mean "git refuses". Real git, real hook
// dispatch, a persona's own environment: the unqualified commit must die.
func TestQAAnIdentityCertifiedSlotIsRefusedByRealGit(t *testing.T) {
	repo, _ := qaHookRepo(t)
	qaGit(t, repo, "config", "user.email", "qa@example.invalid")
	qaGit(t, repo, "config", "user.name", "qa")
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}
	if got := a.probeL3Hooks(repo, true); !got.PrePush || !got.CommitGuard {
		t.Fatalf("fixture must certify before the claim can be tested: %+v", got)
	}

	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644)
	qaGit(t, repo, "add", "a.txt")
	cmd := exec.Command("git", "-C", repo, "commit", "-qm", "unqualified")
	cmd.Env = append(os.Environ(), "RHQ_PERSONA=qa", "RHQ_GATES_DIR=")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Errorf("the probe certified the commit guard and real git allowed the unqualified commit:\n%s", out)
	}
}

// The chain form of the same claim, with the part that makes it load-bearing:
// the neighbour behind the dispatcher waves everything through. Identity is
// certified on posse-<slot>, which git only reaches through a dispatcher that
// runs ours FIRST and propagates its exit — so a hostile neighbour changes
// nothing. If chainRender ever stops running ours first, identity would still
// certify and this is what would say so.
func TestQAAChainCertifiedByIdentityRefusesThroughAHostileNeighbour(t *testing.T) {
	repo, hooks := qaHookRepo(t)
	qaGit(t, repo, "config", "user.email", "qa@example.invalid")
	qaGit(t, repo, "config", "user.name", "qa")
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	vis, _ := a.BeadsVisibility(repo)
	os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"),
		[]byte(chainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg")), 0o755)
	os.WriteFile(filepath.Join(hooks, "theirs-prepare-commit-msg"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	os.WriteFile(filepath.Join(hooks, "posse-prepare-commit-msg"), []byte(CommitGuardHook(vis)), 0o755)

	if got := a.probeL3Hooks(repo, false); !got.CommitGuard {
		t.Fatalf("the prescribed chain must certify: %+v (%q)", got, got.CommitGuardDegraded)
	}
	os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644)
	qaGit(t, repo, "add", "a.txt")
	cmd := exec.Command("git", "-C", repo, "commit", "-qm", "unqualified")
	cmd.Env = append(os.Environ(), "RHQ_PERSONA=qa", "RHQ_GATES_DIR=")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Errorf("a certified chain let real git through a pass-through neighbour:\n%s", out)
	}
}

// ranger-base-gs9r, filed from this verify. l3Identity reads the file at the
// dispatch path with os.ReadFile before anything else, and a read of a FIFO
// with no writer never returns — so a `mkfifo` at .git/hooks/<slot> wedges
// every persona launch into that repo, with no timeout and nothing printed.
// MEASURED both sides of the change: at 88a7726^ a non-executable FIFO
// returned in 38ms (it was exec'd, and exec of a non-executable file fails
// fast); at 88a7726 both modes block. The executable case predates the ADR;
// the widening to any mode does not.
func TestQAL3ProbeMustNotBlockOnANonRegularFileAtTheDispatchPath(t *testing.T) {
	t.Skip("ranger-base-gs9r: os.ReadFile on a FIFO at the dispatch path never returns; every launch into the repo hangs")
	repo, hooks := qaHookRepo(t)
	slot := filepath.Join(hooks, "prepare-commit-msg")
	if err := syscall.Mkfifo(slot, 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	a := &App{}
	done := make(chan l3HookProbe, 1)
	go func() { done <- a.probeL3Hooks(repo, false) }()
	select {
	case got := <-done:
		if got.CommitGuard {
			t.Errorf("a FIFO certified: %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("probeL3Hooks blocked 5s on a FIFO at %s — the launch has no timeout around it", slot)
	}
}

// ─── rangerhq-oaya ───────────────────────────────────────────────────────────

// Idempotence where it actually happens. TestNoPersonaModeIsIdempotent
// composes EnsureUnattendedLine with itself; the path that re-plans a
// recorded line is RelaunchAgent -> planLaunch, and the line it re-plans is
// the one that already carries the flag. Asserted on the typed line, not on
// the function.
func TestQANoPersonaRelaunchTypesTheModeExactlyOnce(t *testing.T) {
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "bare", Cmd: "claude", Crew: true})
	if !strings.Contains(calls(t, fake), "claude "+ClaudeFleetFlags) {
		t.Fatalf("fixture: create typed no mode:\n%s", calls(t, fake))
	}
	if _, err := b.RelaunchAgent("bare", 0); err != nil {
		t.Fatalf("relaunch: %v", err)
	}
	log := calls(t, fake)
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "claude") && strings.Count(line, "--permission-mode") > 1 {
			t.Errorf("a re-planned line collected the mode twice: %q", line)
		}
	}
	if !strings.Contains(log, "claude "+ClaudeFleetFlags) {
		t.Errorf("relaunch typed no permission mode:\n%s", log)
	}
}
