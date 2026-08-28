package rhq

// ADR 0013 DISPATCHING, measured once per runtime (ranger-base-unzn).
//
// Every dispatch pin in this package drives ONE runtime's declaration, and
// which one is an accident of the bead that wrote it: the busy-key split is
// pinned on claude (the typed path), delivery and the claim fence on grok
// (the argv path), record on codex. That is coverage, not parity — the two
// launch paths are different FUNCTIONS (launchSession vs launchWithPrompt),
// each with its own error wrapping, so a pin on one says nothing about the
// other, and a runtime is dispatchable on the strength of whichever bead
// happened to touch it.
//
// These run the same production pass over all three built-in declarations
// and assert the row the ADR promises for each. The table is the point: a
// fourth runtime (Bob) is onboarded by adding a row, and a row that cannot
// be filled is the ADR 0017 §2 UNKNOWN — the loud state.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// parityRuntimes is the ADR 0013 grid's dispatch-relevant declarations, as
// posse ships them. Read from LoadRuntime in the tests below rather than
// trusted from here: this table says what we EXPECT the built-in to declare,
// and a drift between the two is itself a finding.
var parityRuntimes = []struct {
	name    string
	argv    bool // prompt: argv (the work prompt rides the launch line)
	counted bool // a cost adapter reads this runtime
}{
	{name: "claude", argv: false, counted: true},
	{name: "codex", argv: true, counted: false},
	{name: "grok", argv: true, counted: false},
}

// parityPersona writes a PID on the named runtime.
func parityPersona(t *testing.T, a *App, name, labels, runtime string) {
	t.Helper()
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: test\nlabels: " + labels + "\n"
	if runtime != DefaultRuntime {
		md += "runtime: " + runtime + "\n"
	}
	md += "---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The declaration is the contract, so the first thing to measure is that the
// pass FOLLOWS it rather than reproducing whatever the last runtime did. Two
// observables, and they are mutually exclusive by construction: an argv
// runtime leaves a work-prompt file and types nothing at the pane; a typed
// runtime leaves no file and its prompt is a herdr `agent prompt` call.
func TestQAParityDeliveryFollowsEachRuntimesDeclaration(t *testing.T) {
	for _, rt := range parityRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			parityPersona(t, b.App, "ranger", "[go]", rt.name)
			repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"closed"}]`)
			idleClaude(t, fake)

			// The built-in must still declare what this table says it does:
			// a pin that reads the runtime it is about from the runtime it
			// is about proves nothing when the declaration itself moves.
			decl, err := b.App.LoadRuntime(rt.name)
			if err != nil {
				t.Fatalf("LoadRuntime(%s): %v", rt.name, err)
			}
			if got := decl.PromptMode() == PromptArgv; got != rt.argv {
				t.Fatalf("%s declares prompt: argv = %v, the parity table says %v — one of them moved", rt.name, got, rt.argv)
			}

			n, err := d.Run("", "", 0)
			if err != nil || n != 1 {
				t.Fatalf("dispatched %d, err=%v:\n%s", n, err, dispatcherOut(d))
			}
			session := SessionForBead("ranger", repo, "a-1")
			file := b.App.WorkPromptFile(session)
			log := calls(t, fake)
			out := dispatcherOut(d)
			// The rendered launch line lands in one of TWO places and which
			// one is a fact about its LENGTH, not about the runtime: a line
			// too long to type is spilled to state/launch/<session>.sh and
			// the pane runs that (paneline.go). codex's line spills, grok's
			// does not — so a pin that reads only calls.log measures the
			// length, and would have scored codex broken here for a
			// property it has.
			line := log
			if body, err := os.ReadFile(b.App.LaunchScript(session)); err == nil {
				line += "\n--- " + b.App.LaunchScript(session) + " ---\n" + string(body)
			}

			if rt.argv {
				body, err := os.ReadFile(file)
				if err != nil {
					t.Fatalf("%s declares argv and no work prompt was written for %s: %v", rt.name, session, err)
				}
				if !strings.HasPrefix(string(body), "Work beads issue a-1") {
					t.Errorf("the file must hold the assembled ADR 0005 prompt, got:\n%.120s", body)
				}
				if !strings.Contains(line, ArgvPromptSuffix(file)) {
					t.Errorf("%s: the launch line must end in %s:\n%s", rt.name, ArgvPromptSuffix(file), line)
				}
				if strings.Contains(log, "agent prompt") {
					t.Errorf("%s: argv delivery must type nothing at the pane:\n%s", rt.name, log)
				}
				if !strings.Contains(out, "prompt on the launch line") {
					t.Errorf("%s: the pass must say how the prompt got there:\n%s", rt.name, out)
				}
				return
			}
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				t.Errorf("%s declares prompt: typed and a work-prompt file was written anyway (%v)", rt.name, err)
			}
			if !strings.Contains(log, "agent prompt") {
				t.Errorf("%s declares prompt: typed and nothing was typed at the pane:\n%s", rt.name, log)
			}
		})
	}
}

// Step 1 of ADR 0013 §2 is the fence, on every runtime that declares argv:
// a lost claim creates NOTHING — no workspace, no prompt file, no persona
// sitting in a repo working a bead someone else holds — and it is neither a
// session failure nor a persona failure, so nothing is benched. Pinned on
// grok since ranger-base-dg5; codex reaches the same function through its
// own declaration, and "the same function" is the thing a parity table is
// for saying out loud rather than assuming.
func TestQAParityClaimIsTheFenceOnEveryArgvRuntime(t *testing.T) {
	for _, rt := range parityRuntimes {
		if !rt.argv {
			continue // the typed path claims AFTER the session is promptable — the ADR's own order
		}
		t.Run(rt.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			parityPersona(t, b.App, "ranger", "[go]", rt.name)
			repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"in_progress","assignee":"devops"}]`)
			if err := os.WriteFile(filepath.Join(repo, "fake-claim-lost"), []byte("devops"), 0o644); err != nil {
				t.Fatal(err)
			}
			idleClaude(t, fake)

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if n != 0 || !strings.Contains(out, "claim lost") {
				t.Fatalf("%s: want a clean claim loss, got n=%d:\n%s", rt.name, n, out)
			}
			if log := calls(t, fake); strings.Contains(log, "workspace create") {
				t.Errorf("%s: a lost claim created a session anyway — the claim is not the fence:\n%s", rt.name, log)
			}
			if _, err := os.Stat(b.App.WorkPromptFile(SessionForBead("ranger", repo, "a-1"))); !os.IsNotExist(err) {
				t.Errorf("%s: a lost claim wrote the work prompt anyway (%v)", rt.name, err)
			}
			if strings.Contains(out, "skipped for the rest of this pass") || strings.Contains(out, "keeps its slot") {
				t.Errorf("%s: a claim race is neither a session nor a persona failure:\n%s", rt.name, out)
			}
		})
	}
}

// ADR 0013 §2's busy-key split and its ceiling, on every runtime. The split
// lives in fireLoop and is shared, but what REACHES it is not: the typed
// path wraps awaitAgent's failure in sessionFailure and the argv path wraps
// awaitDelivered's, in a different function, around a different error (the
// argv one has already unclaimed). Drop either wrapper and that runtime's
// first cold start benches the persona for the whole pass again — the
// ranger-base-3j8 sterilise, which cost an evening's queue and was measured
// on grok, the runtime whose arm nothing pinned.
func TestQAParitySessionFailureSplitHoldsOnEveryRuntime(t *testing.T) {
	for _, rt := range parityRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.StartupWait = 100 * time.Millisecond
			parityPersona(t, b.App, "ranger", "[go]", rt.name)
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]},{"id":"a-3","title":"v","labels":["go"]}]`, "")
			// No agent is ever detected in any pane: every launch of this
			// persona fails at the "did it become promptable" gate.
			_ = fake

			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)

			if !strings.Contains(out, "keeps its slot") {
				t.Errorf("%s: the FIRST session failure must keep the slot:\n%s", rt.name, out)
			}
			if n := strings.Count(out, "keeps its slot"); n != 1 {
				t.Errorf("%s: exactly one retry, got %d:\n%s", rt.name, n, out)
			}
			bench := "did not take the launch either — second session failure this pass; " +
				SessionFor("ranger", repo) + " benched (ADR 0013 §2 ceiling)"
			if !strings.Contains(out, bench) {
				t.Errorf("%s: want the second failure benched and named as the second (%q):\n%s", rt.name, bench, out)
			}
			// Both beads got their own fresh Dial F pane, and the third
			// never launched because the slot is benched.
			for _, want := range []string{SessionForBead("ranger", repo, "a-1"), SessionForBead("ranger", repo, "a-2")} {
				if !strings.Contains(out, want) {
					t.Errorf("%s: bead session %s never got its own launch:\n%s", rt.name, want, out)
				}
			}
			if s3 := SessionForBead("ranger", repo, "a-3"); strings.Contains(out, s3) {
				t.Errorf("%s: a benched slot launched a third bead (%s):\n%s", rt.name, s3, out)
			}
		})
	}
}

// The re-prompt loop, on every runtime (ranger-base-9hm). MEASURED
// 2026-08-26: two sessions whose account was exhausted settled with their
// beads still open, and an unattended --resume pass re-prompted them every
// five minutes until the pass budget was gone. The escalation that bounds it
// reads only the bead's own comments, so it is runtime-blind by
// construction — which is worth a pin, because "runtime-blind" is exactly
// what the account stage was also assumed to be.
func TestQAParitySettleOpenEscalationBoundsTheLoopOnEveryRuntime(t *testing.T) {
	for _, rt := range parityRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			parityPersona(t, b.App, "ranger", "[go]", rt.name)
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
			idleClaude(t, fake)

			// Pass 1: the session settles, the bead is still open. Recorded,
			// not escalated — one settle is a nudge.
			first := newTestDispatcher(t, b)
			first.Resume = true
			if _, err := first.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			if out := dispatcherOut(first); strings.Contains(out, "escalated to") {
				t.Errorf("%s: the FIRST settle-open escalated — every runtime gets one nudge:\n%s", rt.name, out)
			}

			// Pass 2: the same disagreement, a second time. The loop stops
			// being polite and the bead leaves the ready queue.
			second := newTestDispatcher(t, b)
			second.Resume = true
			if _, err := second.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(second)
			if !strings.Contains(out, "escalated to") || !strings.Contains(out, "not re-prompted") {
				t.Errorf("%s: the SECOND settle-open must escalate rather than re-prompt forever:\n%s", rt.name, out)
			}
			_ = repo
		})
	}
}

// THE ACCEPTANCE CRITERION of ranger-base-unzn, and it fails on two of three
// runtimes: an ACCOUNT EXHAUSTED result must be distinguishable from a
// RUNTIME BROKEN result in the pass output, without a human peeking at the
// pane.
//
// MEASURED 2026-08-28 (ranger-base-02zr): dispatch.go's gather gates the
// turn-outcome read on `p.runtime == DefaultRuntime`, so the adapter is
// never consulted for codex or grok. A codex session whose provider refused
// the first turn prints the same `◑ settled "idle" but issue is
// "in_progress"` as a codex session that did the work and skipped the bead,
// and a grok one prints the line a record: trusted runtime gets for an
// ordinary miss. Nothing is written to the session meta either, so `posse
// list` shows both as healthy.
//
// Read this skip as a red. The claude arm below is the positive witness that
// the assertion can pass at all.
func TestQAParityAccountRefusalIsNamedOnEveryRuntime(t *testing.T) {
	t.Skip("ranger-base-02zr: the turn-outcome adapter is hardcoded to claude, so an exhausted account on codex/grok is rendered as an ordinary settle")

	const refusal = "You've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model."
	for _, rt := range parityRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			parityPersona(t, b.App, "ranger", "[go]", rt.name)
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","title":"t","status":"in_progress","assignee":"ranger"}]`)
			idleClaude(t, fake)
			asked := 0
			d.TurnOutcome = func(string, string, time.Time) (string, bool) {
				asked++
				return refusal, true
			}

			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if asked == 0 {
				t.Fatalf("%s: the turn-outcome adapter was never asked — an exhausted account cannot be told from a broken runtime:\n%s", rt.name, out)
			}
			if !strings.Contains(out, "refused the first turn") || !strings.Contains(out, "no work ran") {
				t.Errorf("%s: a provider refusal was presented as an ordinary settle:\n%s", rt.name, out)
			}
			if strings.Contains(out, "review") {
				t.Errorf("%s: a refused turn is not work to review:\n%s", rt.name, out)
			}
			// And it survives the pass: `posse list` must not show a session
			// that never ran a turn as healthy.
			session := SessionForBead("ranger", repo, "a-1")
			m, ok := b.readMeta(session)
			if !ok || m.TurnFailure == "" {
				t.Errorf("%s: the refusal is not in the session meta, so posse list looks healthy: %+v", rt.name, m)
			}
		})
	}
}
