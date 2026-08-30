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
	"strconv"
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
// NO WALL CLOCK HERE — ranger-base-9mwa, and before it ranger-base-m8ko.
// This test used to plant explain-error from a goroutine 700ms after the
// body started, which is the fixture commit 0ebdbce (ranger-base-4pjw)
// retired in the OTHER test of the pair and left standing in this one.
//
// Why that red: 700ms raced the launch's own setup. The first `agent
// explain` of a fake-herdr launch lands 293-340ms in on an idle box
// (0ebdbce, 10 runs), so the margin was ~395ms of
// workspace-create/persona/launch forks. When the setup crossed 700ms the
// FIRST explain already errored, awaitSettled's lastWhy stayed "" and the
// concession path prompted — the exact shape the third assertion below says
// is absent, so all three fired at once. Measured at 3583221: 3 of 30 red
// on a box at load 13-16; ranger-base-9mwa reports 11 of 20 on main. The
// timer was never the point — a probe on the old fixture
// logged only 2-3 explains served before it fired, so "after some guesses"
// was a coincidence of scheduling.
//
// THE UNDER-LOAD NUMBERS WERE RE-TAKEN — ranger-base-0qny. 9mwa's loaded
// pair ("20 of 20 red under 8 spinners, 0 of 20 after") came off a rig that
// leaked its own spinners: the arms ran in two blocks, the first block was
// still burning when the second started, so they were measured under 8 and
// 16 while both labels said 8. Re-taken with the three fixtures INTERLEAVED
// inside each repetition — any drift then falls on all three equally — in a
// container pinned to 2 real cpus, 20 reps per arm per level. The three are
// e2b1cfe (the timer), 6ee039e (the countdown as landed) and ff1779e (the 4s
// window), each built with `GOOS=linux go test -c ./internal/rhq/`:
//
//	                              idle   1 spin/cpu   2 spin/cpu
//	timer, 900ms window          11/20     19/20        16/20
//	countdown, 900ms window       0/20      1/20         0/20
//	countdown, 4s window (here)   0/20      1/20         0/20
//
// The direction 9mwa claimed holds, and its own numbers understate it: every
// red of the timer fixture is the concession path, at every load level, and
// the countdown removes it entirely — 0 in 120. What "0 of 20" overstated is
// what is left. The countdown's remaining reds are a different failure
// wearing the same red: "fixture unmet", the witness below reporting the box
// rather than the fixture, which is the defect ranger-base-t1aq then sized
// this window against. It survives that sizing here — one 4s red in 60 — so
// the witness is doing its job, not the window's.
//
// Read the columns, not the rows: only the three arms of one level were
// measured against each other. 16/20 under the heaviest load is not a
// recovery from 19/20 (n=20, and the host under the container quieted from
// loadavg 21 to 9 across that level) — it is the same number.
//
// The lever is the same countdown bootrace_qa_test.go uses — the first
// `guesses` explains answer, every one after that errors, so the LAST
// explain of the window is the failed one by construction. The window is
// still time-driven, so the fixture carries 0ebdbce's witness too: if the
// log holds no more explains than guesses served, the late error was never
// reached and this fails naming the fixture rather than passing on an
// assertion of absence.
//
// Which left the witness reporting the box rather than the fixture: at 900ms
// it was red about 1 run in 10 under load with "fixture unmet: 2 explains"
// (ranger-base-t1aq). The window below is sized off that measurement — see
// the WINDOW SIZING note over the twin in bootrace_qa_test.go, which carries
// the runs and why 4s.
func TestQALateExplainErrorStillFailsLoudlyNamingTheGuess(t *testing.T) {
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 4 * time.Second
	d.Poll = 100 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644) // guess forever
	os.WriteFile(filepath.Join(fake, "error-on-stderr"), nil, 0o644)  // the real 0.8.0 shape
	// The late failure, armed by call count rather than by a timer: see
	// fakeExplainErrorArmed, and the comment above for what the timer cost.
	const guesses = 2
	os.WriteFile(filepath.Join(fake, "explain-error-after"), []byte(strconv.Itoa(guesses)), 0o644)
	os.WriteFile(filepath.Join(fake, "explain-error"), []byte("internal|no detection for w1:p1"), 0o644)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	// The fixture's own witness. Both halves have to have happened: a window
	// that never got past the guesses would satisfy the third assertion
	// below for the wrong reason, since that one asserts an absence.
	if explains := strings.Count(calls(t, fake), "agent explain"); explains <= guesses {
		t.Fatalf("fixture unmet: %d explains in the window, so the late error was never served "+
			"(needs more than %d) — the box is too slow for a %s window at %s polls:\n%s",
			explains, guesses, d.StartupWait, d.Poll, dispatcherOut(d))
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
//
// NO WALL CLOCK HERE — ranger-base-3wc7. This test used to remove
// explain-error from a goroutine 300ms after the body started, which is the
// same wall clock ranger-base-9mwa took out of the twin above, with the
// opposite symptom: it never went red, it just measured nothing. The delete
// raced the launch's own setup and won, so no poll in the window was ever
// served the early error and both assertions below held over a window of
// pure guesses. Measured 2026-08-30 with a throwaway in-package probe that
// counted calls.log at the instant the goroutine fired: 0 explains at 300ms,
// 12 of 12 runs.
//
// The lever is now the inverse of the twin's: explain-error-for errors the
// FIRST `broken` explains and answers every one after that, so the error is
// at the head of the window by construction rather than by scheduling. The
// window is still time-driven, so the fixture carries a witness of its own:
// if the log holds no more explains than the countdown consumed, herdr never
// came back and this fails naming the fixture rather than passing on an
// assertion of absence. Window sized as the twin's — see the WINDOW SIZING
// note over TestQAGuessesForTheWholeWindowAreLostToOneLateExplainError in
// bootrace_qa_test.go, which carries the runs and why 4s.
func TestQAAnEarlyExplainErrorDoesNotOutliveALaterGuess(t *testing.T) {
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 4 * time.Second
	d.Poll = 100 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644) // guess forever
	os.WriteFile(filepath.Join(fake, "error-on-stderr"), nil, 0o644)  // the real 0.8.0 shape
	// herdr away at the head of the window and back for the rest of it,
	// armed by call count rather than by a timer: see fakeExplainErrorArmed,
	// and the comment above for what the timer cost.
	const broken = 2
	os.WriteFile(filepath.Join(fake, "explain-error-for"), []byte(strconv.Itoa(broken)), 0o644)
	os.WriteFile(filepath.Join(fake, "explain-error"), []byte("internal|no detection for w1:p1"), 0o644)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	// The fixture's own witness. Both halves have to have happened: a window
	// that never got past the errors would satisfy the first assertion below
	// for the wrong reason, since that one asserts an absence — and it would
	// take the concession path, which is what the third one is for.
	if explains := strings.Count(calls(t, fake), "agent explain"); explains <= broken {
		t.Fatalf("fixture unmet: %d explains in the window, so herdr never came back "+
			"(needs more than %d) — the box is too slow for a %s window at %s polls:\n%s",
			explains, broken, d.StartupWait, d.Poll, dispatcherOut(d))
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("prompted at a screen herdr only ever guessed about:\n%s", dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "never saw a screen it recognizes") {
		t.Errorf("an early error must not survive the guesses that followed it:\n%s", out)
	}
	if strings.Contains(out, "prompting on its") {
		t.Errorf("the cannot-be-read concession fired over a window that WAS read after the error:\n%s", out)
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
	os.WriteFile(filepath.Join(hooks, "posse-prepare-commit-msg"), []byte(CommitGuardHook(vis, OpsPatternSet{})), 0o755)

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
