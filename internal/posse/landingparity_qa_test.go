package posse

// ranger-base-ewq9 is CLOSED with the decision that landThePlane's
// AgentPrompt call stays BRANCHLESS (d3a3fed): every other AgentPrompt
// caller consults PromptMode() to place a CREATE's work prompt — argv line
// or typed — and landThePlane never creates, so typed delivery is the only
// mechanism there is on every runtime. That decision was never pinned. The
// suite carried only a t.Skip (sessionparity_qa_test.go), parked while the
// bead was open and reasoned from a fix that is no longer coming, so a
// later "consistency" edit adding `if rt.PromptMode() == PromptArgv` to
// landThePlane — the shape the closed bead rejects — would have left the
// suite green and codex and grok sessions relaunching without their
// landing turn.
//
// Filed while verifying that close (ranger-base-enw0n).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// devSessionOn is devSession with the runtime named — the one axis this
// pin varies.
func devSessionOn(t *testing.T, b *HerdrBackend, name, runtime string) {
	t.Helper()
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\ndeny: [Bash(git push:*)]\n---\nYou are dev.\n"), 0o644)
	if err := b.CreateSession(NewSessionOpts{
		Name: name, Agent: "dev", Dir: t.TempDir(), Tier: "standard", Runtime: runtime,
	}); err != nil {
		t.Fatalf("%s: create: %v", runtime, err)
	}
}

func TestQALandingTurnIsTypedOnEveryRuntimePromptArgvIncluded(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		runtime string
		mode    string
	}{
		{"claude", PromptTyped},
		{"codex", PromptArgv},
		{"grok", PromptArgv},
	} {
		t.Run(tc.runtime, func(t *testing.T) {
			b, fake := newTestBackend(t)
			agentPerLaunch(t, fake)
			// The control arm: this runtime really is the delivery class
			// the pin means to exercise. A runtimes/ edit that flipped
			// codex to prompt: typed would otherwise leave this test
			// asserting nothing about argv delivery at all.
			rt, err := b.App.LoadRuntime(tc.runtime)
			if err != nil {
				t.Fatalf("load runtime %s: %v", tc.runtime, err)
			}
			if got := rt.PromptMode(); got != tc.mode {
				t.Fatalf("%s prompt mode = %q, want %q — this pin measures the wrong class now", tc.runtime, got, tc.mode)
			}

			devSessionOn(t, b, "s1", tc.runtime)
			var out strings.Builder
			if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
				t.Fatalf("relaunch on %s: %v\n%s", tc.runtime, err, out.String())
			}
			log := calls(t, fake)
			if !strings.Contains(out.String(), "landing s1") {
				t.Errorf("%s: relaunch did not announce the landing turn:\n%s", tc.runtime, out.String())
			}
			// The turn is DELIVERED, not merely announced: a typed
			// `agent prompt` carrying the landing text.
			if !strings.Contains(log, "agent prompt") {
				t.Errorf("%s (prompt: %s): landThePlane sent no typed prompt — the landing turn is branchless by decision (ranger-base-ewq9):\n%s",
					tc.runtime, tc.mode, log)
			}
			for _, want := range []string{"Land the plane", "ORDERS.md"} {
				if !strings.Contains(log, want) {
					t.Errorf("%s: landing prompt missing %q:\n%s", tc.runtime, want, log)
				}
			}
		})
	}
}
