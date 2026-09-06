//go:build posse_arm2

package posse

// QA pins for the ADR 0013 §1 settle stage's DECLARED half (ranger-base-02zr):
// whether posse can read what a runtime's own first turn did, and what a pass
// prints when it cannot.
//
// MEASURED, and the reason the seam exists: the same fixture and the same
// stubbed turn outcome, driven through production `Dispatcher.Run` on all
// three built-in runtimes, asked the reader once on claude and NEVER on codex
// or grok — because gather keyed the read on `p.runtime == DefaultRuntime`, a
// runtime NAME standing in for the dimension "is this runtime's turn outcome
// readable" (ADR 0017 §3). So an exhausted account on codex/grok printed as an
// ordinary settle-without-close, and on 2026-08-28 grok's account really was
// answering `402 Payment Required` while a pass called it a settle.
//
// The consumer is what these drive — production Run over more than one runtime
// declaration — because the getter agreeing with itself is not the defect.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runtimePersona is a PID pinned to one runtime, which is the only thing that
// varies across the arms below.
func runtimePersona(t *testing.T, a *App, name, labels, runtime string) {
	t.Helper()
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: test\nlabels: " + labels + "\nruntime: " + runtime + "\n---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readableRuntime is a template-only runtime that DECLARES the reader claude
// uses. It is the arm that a name-keyed implementation cannot pass: it is not
// claude, and it must be read anyway.
func readableRuntime(t *testing.T, a *App, name string) {
	t.Helper()
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "command: " + name + " --sys {file}\nturn_outcome: " + TurnOutcomeClaudeTranscript + "\n"
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const qaAllotmentRefusal = "You've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model."

// The pin the bead was filed on. One fixture, one stubbed turn outcome, four
// runtime declarations: what changes across the arms is the DECLARATION, and
// the pass must key on that and nothing else.
func TestQAParityAccountRefusalIsNamedOnEveryRuntime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		runtime  string
		template bool // template-only yaml this test writes, not a built-in
		readable bool // declares a turn_outcome: adapter
	}{
		{runtime: "claude", readable: true},
		{runtime: "codex"},
		{runtime: "grok", readable: true}, // grok-session-store, ranger-base-fc8go
		{runtime: "mycli", template: true, readable: true},
	}
	for _, c := range cases {
		t.Run(c.runtime, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			if c.template {
				readableRuntime(t, b.App, c.runtime)
			}
			runtimePersona(t, b.App, "ranger", "[go]", c.runtime)
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
			idleClaude(t, fake)

			asked, askedCwd := 0, ""
			d.TurnOutcome = func(cwd, bead string, since time.Time) (TurnOutcome, bool) {
				asked++
				askedCwd = cwd
				if bead != "a-1" || since.IsZero() {
					t.Fatalf("turn outcome lookup = %q %q %v", cwd, bead, since)
				}
				return TurnOutcome{Message: qaAllotmentRefusal}, true
			}

			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			session := SessionForBead("ranger", repo, "a-1")

			if c.readable {
				// Asked, stopped, and named: the account refused the turn, so
				// no work ran and re-prompting the same tier buys nothing.
				if asked != 1 {
					t.Errorf("a declared reader must be asked exactly once, asked %dx:\n%s", asked, out)
				}
				for _, want := range []string{"⛔ a-1", c.runtime + " refused the first turn", qaAllotmentRefusal, "no work ran"} {
					if !strings.Contains(out, want) {
						t.Errorf("the refusal line must carry %q:\n%s", want, out)
					}
				}
				if strings.Contains(out, "◑ a-1") {
					t.Errorf("a refused turn was printed as an ordinary settle:\n%s", out)
				}
				// The session carries it too, so `posse list` does not show a
				// dead account as a healthy session.
				m, ok := b.readMeta(session)
				if !ok || m.TurnFailure != qaAllotmentRefusal {
					t.Fatalf("turn failure not recorded in session meta: %+v", m)
				}
				// WHICH directory the reader was asked about is the session's
				// own cwd and not the repo (ranger-base-f09bw). It reads as
				// the repo here only because qaRepo is not a git repo, so
				// this launch made no tree — asserting the repo literal would
				// pin that coincidence, and did until this bead. The pin that
				// drives a real tree through Run is
				// TestQAWorktreeDispatchAsksAboutTheSessionsTreeNotTheRepo.
				if askedCwd != m.Dir {
					t.Errorf("the reader was asked about %q, want the session's own cwd %q", askedCwd, m.Dir)
				}
				return
			}

			// Blind arm. Nothing is read — a runtime that declares no reader
			// gets no reading, injected stub or not — and the line SAYS the
			// fact is missing rather than offering the one explanation that
			// happens to fit a healthy CLI.
			if asked != 0 {
				t.Errorf("a runtime declaring no reader must not be read, asked %dx:\n%s", asked, out)
			}
			if !strings.Contains(out, "◑ a-1") {
				t.Errorf("a settle-without-close must still be reported:\n%s", out)
			}
			for _, want := range []string{
				"posse reads no turn outcome on " + c.runtime,
				"an account that refused the turn settles exactly like this",
				"posse peek " + session,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("the settle line must say what posse cannot see (%q):\n%s", want, out)
				}
			}
			if strings.Contains(out, "refused the first turn") {
				t.Errorf("a blind runtime cannot claim to have read a refusal:\n%s", out)
			}
			if m, _ := b.readMeta(session); m.TurnFailure != "" {
				t.Errorf("a blind runtime marked a turn failure it never observed: %+v", m)
			}
		})
	}
}

// The bead ranger-base-qcu4c was filed on: "no work ran" is a claim about the
// turn, not a synonym for "refused", and a turn can be refused after it has
// been running for a minute and a half. Driven through production Run so the
// pin is on the LINE an operator reads, not on the reader that feeds it.
//
// The fixture is the box's own counterexample: 1 of the 7 refusals in this
// machine's grok history carries a full usage object — 6 model calls, 5,571
// output tokens, 85.6s of API time — from a session that may well have edited
// files and commented on the bead before the account went out from under it.
// The old line told its operator nothing happened and to relaunch.
func TestQARefusalAfterWorkDoesNotClaimNoWorkRan(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		outcome TurnOutcome
		// what the line must say, and what it must not
		want    []string
		notWant []string
	}{{
		// The claude shape and 6 of grok's 7: the account refused before it
		// served anything, so the flat claim is true and stays.
		name:    "refused before serving",
		outcome: TurnOutcome{Message: qaAllotmentRefusal},
		want:    []string{"refused the first turn", "no work ran", "relaunch "},
		notWant: []string{"mid-flight", "check the worktree"},
	}, {
		name:    "refused mid-flight",
		outcome: TurnOutcome{Message: qaAllotmentRefusal, ModelCalls: 6, OutputTokens: 5571},
		// The numbers are on the line because "work may exist" without them
		// is a hedge an operator cannot act on: six model calls is a session
		// to go and look at, and the phrasing must survive somebody deciding
		// the line reads better without them.
		want:    []string{"refused the turn mid-flight", "6 model calls", "5571 output tokens", "posse peek ", "check the worktree"},
		notWant: []string{"no work ran", "refused the first turn"},
	}, {
		// One call, one token: singular, and still work. The arm exists
		// because a plural-only line is the kind of thing that reads fine
		// until the day it prints "1 model calls".
		name:    "refused after a single call",
		outcome: TurnOutcome{Message: qaAllotmentRefusal, ModelCalls: 1, OutputTokens: 1},
		want:    []string{"refused the turn mid-flight", "(1 model call, 1 output token)"},
		notWant: []string{"no work ran", "1 model calls", "1 output tokens"},
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			writePersona(t, b.App, "ranger", "[go]")
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
			idleClaude(t, fake)
			d.TurnOutcome = func(string, string, time.Time) (TurnOutcome, bool) { return c.outcome, true }

			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			for _, want := range append([]string{"⛔ a-1", qaAllotmentRefusal}, c.want...) {
				if !strings.Contains(out, want) {
					t.Errorf("the refusal line must carry %q:\n%s", want, out)
				}
			}
			for _, no := range c.notWant {
				if strings.Contains(out, no) {
					t.Errorf("the refusal line must not carry %q:\n%s", no, out)
				}
			}
			// Both arms are refusals: the bead still stops and the session is
			// still marked, whatever the line says about how far it got.
			if strings.Contains(out, "\u25d1 a-1") {
				t.Errorf("a refused turn was printed as an ordinary settle:\n%s", out)
			}
			session := SessionForBead("ranger", repo, "a-1")
			if m, ok := b.readMeta(session); !ok || m.TurnFailure != qaAllotmentRefusal {
				t.Errorf("turn failure not recorded in session meta: %+v", m)
			}
		})
	}
}

// codex is both `record: untrusted` and turn-outcome blind, and the two say
// different things: one is the declared degrade (nothing was lost, --resume
// retries), the other is a fact posse does not have. The line carries both,
// in one parenthesis, and neither swallows the other.
func TestQABlindAndUntrustedBothLandOnTheSettleLine(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	runtimePersona(t, b.App, "ranger", "[go]", "codex")
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	for _, want := range []string{
		"codex is record: untrusted",
		"--resume re-prompts it",
		"posse reads no turn outcome on codex",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the settle line must carry %q:\n%s", want, out)
		}
	}
	// One parenthesis, both clauses — a line with two trailing parentheses is
	// two answers to one question and reads as an afterthought.
	if strings.Count(out, "(codex is record: untrusted") != 1 || strings.Contains(out, ") (") {
		t.Errorf("the clauses must join into one parenthesis:\n%s", out)
	}

	// The other half: `record: trusted` does not buy a reader. A trusted
	// runtime with no turn_outcome: gets the blindness and NOT the
	// reassurance, because a trusted runtime that stopped closing its beads
	// is the record-skip signal, not a footnote (ADR 0013 §4).
	//
	// A template runtime rather than a built-in, and that is the arm's whole
	// robustness: grok used to be the fixture for this — trusted AND blind —
	// and stopped being one the day it got a reader (ranger-base-fc8go), so
	// the pin was measuring a coincidence of the built-in table rather than
	// the independence of two declarations. Written this way it survives
	// codex getting a reader too.
	b2, fake2 := newTestBackend(t)
	d2 := newTestDispatcher(t, b2)
	if err := os.MkdirAll(b2.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "command: trustedblind {file}\nrecord: trusted\nrecord_why: a dispatched session of it was measured closing its bead\n"
	if err := os.WriteFile(filepath.Join(b2.App.RuntimesDir(), "trustedblind.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runtimePersona(t, b2.App, "ranger", "[go]", "trustedblind")
	qaRepo(t, b2.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out2 := dispatcherOut(d2)
	if !strings.Contains(out2, "posse reads no turn outcome on trustedblind") {
		t.Errorf("a trusted runtime is still blind and must say so:\n%s", out2)
	}
	if strings.Contains(out2, "record: untrusted") {
		t.Errorf("trustedblind is record: trusted — no degrade clause belongs on its line:\n%s", out2)
	}
}

// grok is the runtime that used to carry both facts the test above splits —
// `record: trusted` and turn-outcome blind — and since ranger-base-fc8go it
// carries neither: its settle line is the plain disagreement, with no clause
// about anything posse cannot see. The arm exists because the blindness
// clause going quiet on a runtime is exactly what nobody notices.
func TestQAGrokSettleCarriesNoBlindnessClause(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	runtimePersona(t, b.App, "ranger", "[go]", "grok")
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "◑ a-1") {
		t.Fatalf("a settle-without-close must still be reported:\n%s", out)
	}
	if strings.Contains(out, "posse reads no turn outcome on grok") {
		t.Errorf("grok declares a reader now — the blindness clause is a lie:\n%s", out)
	}
	if strings.Contains(out, "record: untrusted") {
		t.Errorf("grok is record: trusted — no degrade clause belongs on its line:\n%s", out)
	}
}

// A runtime WITH a reader gets no blindness clause on an ordinary settle: the
// pass looked, the turn was answered, and the disagreement is the bead's.
func TestQAReadableRuntimeSettleCarriesNoBlindnessClause(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)
	d.TurnOutcome = func(string, string, time.Time) (TurnOutcome, bool) { return TurnOutcome{}, true }

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "◑ a-1") {
		t.Fatalf("want the settle-without-close line:\n%s", out)
	}
	if strings.Contains(out, "reads no turn outcome") {
		t.Errorf("claude declares a reader — the line must not claim blindness:\n%s", out)
	}
}

// The pin ranger-base-1mei was filed on: a declared reader that looked and
// found nothing (observed=false — transcript not readable yet, not "the
// turn was healthy") must not print byte-identical to the readable-and-
// healthy arm above. Before this fix neither arm carried a clause.
func TestQAUnobservedTurnOutcomeSettleLineIsNamed(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
	idleClaude(t, fake)
	asked := 0
	d.TurnOutcome = func(string, string, time.Time) (TurnOutcome, bool) {
		asked++
		return TurnOutcome{}, false
	}

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	session := SessionForBead("ranger", repo, "a-1")
	if asked != 1 {
		t.Errorf("a declared reader must be asked exactly once, asked %dx:\n%s", asked, out)
	}
	if !strings.Contains(out, "◑ a-1") {
		t.Fatalf("want the settle-without-close line:\n%s", out)
	}
	for _, want := range []string{
		"posse looked for a turn outcome on claude and found none this pass",
		"an account that refused the turn can settle exactly like this",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the settle line must say the read came back unobserved (%q):\n%s", want, out)
		}
	}
	if strings.Contains(out, "reads no turn outcome") {
		t.Errorf("claude declares a reader — this is not the blind case:\n%s", out)
	}
	if strings.Contains(out, "refused the first turn") {
		t.Errorf("observed=false is not a refusal:\n%s", out)
	}
	if m, _ := b.readMeta(session); m.TurnFailure != "" {
		t.Errorf("an unobserved read must not mark a turn failure: %+v", m)
	}
}

// The declaration itself: absent is blind, a registered name reads, and a name
// no reader implements REFUSES at load rather than degrading quietly — the
// same rule `record: trused` gets (ADR 0013 §1).
func TestQATurnOutcomeDeclarationIsRegistryKeyed(t *testing.T) {
	t.Parallel()
	a := checkApp(t)

	if rt := writeRuntime(t, a, "bare", "command: bare {file}\n"); rt.ReadsTurnOutcome() {
		t.Error("an undeclared runtime must be blind, not assumed readable")
	}
	rt := writeRuntime(t, a, "mycli", "command: mycli {file}\nturn_outcome: "+TurnOutcomeClaudeTranscript+"\n")
	if !rt.ReadsTurnOutcome() || rt.TurnOutcomeAdapter != TurnOutcomeClaudeTranscript {
		t.Errorf("a declared adapter must resolve to its reader: %+v", rt.TurnOutcomeAdapter)
	}
	if TurnOutcomeReaderFor(rt) == nil {
		t.Error("the registry must hand back the reader it registered")
	}

	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "typo.yaml"),
		[]byte("command: typo {file}\nturn_outcome: claude-transcripts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.LoadRuntime("typo")
	if err == nil {
		t.Fatal("a turn_outcome: no reader implements must refuse at load")
	}
	for _, want := range []string{"turn_outcome:", "claude-transcripts", TurnOutcomeClaudeTranscript} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q: %v", want, err)
		}
	}
}

// The built-in table, which is where the fleet's answer actually lives: claude
// reads its own transcript and grok its own session store (ranger-base-fc8go,
// over the artifact docs/adr/0013-turn-outcome-refusal-probe.md captured).
// codex declares none — reachable in principle (ranger-base-xaev), but its
// refusal artifact has never been captured, and ADR 0013 §1's promotion rule
// is that a reader waits for the capture rather than guessing the shape.
func TestQABuiltinTurnOutcomeDeclarations(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	want := map[string]bool{"claude": true, "codex": false, "grok": true}
	for name, readable := range want {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatal(err)
		}
		if rt.ReadsTurnOutcome() != readable {
			t.Errorf("%s reads turn outcome = %v, want %v (adapter %q)", name, rt.ReadsTurnOutcome(), readable, rt.TurnOutcomeAdapter)
		}
	}
	// The two readers are not one reader wearing two names: grok's refusal is
	// not in a transcript at all, so a built-in pointed at the wrong key would
	// read nothing and say nothing while looking exactly like this.
	grok, err := a.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	if grok.TurnOutcomeAdapter != TurnOutcomeGrokSessionStore {
		t.Errorf("grok declares turn_outcome: %q, want %q", grok.TurnOutcomeAdapter, TurnOutcomeGrokSessionStore)
	}
}

// `posse runtime check` is how a runtime is onboarded, so the blindness is a
// row there too — with the key that changes it.
func TestQARuntimeCheckSettleRowNamesTheTurnOutcomeReader(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	read := settleRow(claude)
	if !strings.Contains(strings.Join(read.note, "\n"), "turn outcome: READ by the "+TurnOutcomeClaudeTranscript) {
		t.Errorf("a readable runtime's settle row must name its adapter: %+v", read.note)
	}
	blind := settleRow(writeRuntime(t, a, "bare", "command: bare {file}\n"))
	joined := strings.Join(blind.note, "\n")
	for _, want := range []string{"turn outcome: NOT READ", "SAME ◑ line", "turn_outcome: " + TurnOutcomeClaudeTranscript} {
		if !strings.Contains(joined, want) {
			t.Errorf("a blind runtime's settle row must carry %q: %+v", want, blind.note)
		}
	}
}
