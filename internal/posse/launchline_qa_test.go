package posse

// QA pins for the account stage's per-launch record (ranger-base-pjoy,
// item 2 of ADR 0013 §5 / ADR 0017 §3).
//
// The defect: both of dispatch's create lines printed persona, dir and tier
// and NOT the runtime, so a pass that sent beads to three runtimes read as
// three identical launches. The end-of-pass account line could say "codex:
// sent 1 bead(s) this pass" with nothing above it saying WHICH bead — the
// only per-launch record of where the spend went was the session meta, and
// a test that wanted the runtime had to read it back off the resolved PID
// (accountstage_qa_test.go says so in its own comment).
//
// There are two create lines because there are two delivery paths (ADR 0013
// §2): claude is PromptTyped and prints from launchSession; codex and grok
// are PromptArgv and print from launchWithPrompt. A fix to one is not a fix
// to the other, so both arms run a real pass.

import (
	"os"
	"strings"
	"testing"
)

func TestQAEveryCreateLineNamesTheRuntime(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		runtime string
		// path is the create line this runtime's prompt mode reaches, named
		// so a green arm cannot be one path measured twice.
		path string
	}{
		{"claude", "typed"},
		{"codex", "argv"},
	} {
		t.Run(c.runtime, func(t *testing.T) {
			f := uncountedPass(t, "", `[{"id":"a-1","title":"t","labels":["go"]}]`, "ranger")
			// uncountedPass writes a codex PID; retarget it (the same
			// rewrite accountstage_qa_test.go makes, for the same reason).
			pid := strings.Replace(codexPID("ranger"), "runtime: codex", "runtime: "+c.runtime, 1)
			if err := os.WriteFile(f.b.App.AgentsDir+"/ranger.md", []byte(pid), 0o644); err != nil {
				t.Fatal(err)
			}
			// The rig must be shown able to launch at all before its output
			// is read as evidence about launching.
			n, err := f.d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(f.d)
			if n != 1 {
				t.Fatalf("the bead must launch, got n=%d:\n%s", n, out)
			}
			// Positive witness that this arm really ran on the runtime it
			// names — without it a silent PID-rewrite failure leaves both
			// arms measuring codex, and the claude arm passes for the wrong
			// reason.
			ag, err := f.b.App.LoadAgent("ranger")
			if err != nil || ag.Runtime != c.runtime {
				t.Fatalf("the persona dispatch ran is on %q, want %q (%v)", ag.Runtime, c.runtime, err)
			}
			rt, err := f.b.App.LoadRuntime(c.runtime)
			if err != nil {
				t.Fatal(err)
			}
			// ...and that this arm reached the create line under test. A
			// typed runtime that started printing the argv line would
			// otherwise satisfy the runtime assertion below while leaving
			// the other path unmeasured for good.
			argv := strings.Contains(out, "work prompt on the launch line")
			if want := rt.PromptMode() == PromptArgv; argv != want {
				t.Fatalf("%s is the %s path; argv line present = %v, want %v:\n%s", c.runtime, c.path, argv, want, out)
			}

			line := createLineOf(t, out)
			if !strings.Contains(line, c.runtime+"/") {
				t.Errorf("the create line must name the runtime this launch went to:\n%s", line)
			}
			// The tier half is the DISPLAY tier and rides in the same field,
			// so the line and the work prompt it delivers (`runtime/tier:`)
			// and herdr's listing (`@runtime/tier`) all say one thing.
			tier, _ := f.b.App.BeadTier("", BdIssue{}, ag)
			if !strings.Contains(line, c.runtime+"/"+f.b.App.DisplayTier(c.runtime, tier)) {
				t.Errorf("create line must carry runtime/%s: %q", f.b.App.DisplayTier(c.runtime, tier), line)
			}
			// The negative arm: a hardcoded name, or a tag built from the
			// wrong variable, would put SOME runtime on every line.
			for _, other := range []string{"claude", "codex", "grok"} {
				if other != c.runtime && strings.Contains(line, other+"/") {
					t.Errorf("the create line names %s for a launch that went to %s: %q", other, c.runtime, line)
				}
			}
		})
	}
}

// createLineOf returns the one "creating session" line from a pass, and
// fails if there is not exactly one — a pin that greps a whole transcript
// can be satisfied by any line in it.
func createLineOf(t *testing.T, out string) string {
	t.Helper()
	var found []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "creating session ") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly 1 create line, got %d:\n%s", len(found), out)
	}
	return found[0]
}
