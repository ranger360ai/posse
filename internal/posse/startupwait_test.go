package posse

// ranger-base-p84: Runtime.StartupWait was declared, parsed, validated,
// documented and PRINTED by `runtime check` — and dispatch never read it.
// Dispatcher.StartupWait was set once from the DefaultStartupWait constant
// and every launch in a pass waited that long, whatever its own runtime
// declared. runtimecheck_test.go already pins the getter (rt.Wait()); this
// drives dispatch itself, so a disagreement between what `runtime check`
// prints and what a launch actually waits shows up here, not just there.
//
// architect's design note on ranger-base-il14: one Dispatcher fires every
// runtime a pass touches, so the patience has to move per-launch, not live
// on the Dispatcher alone. Wired via Dispatcher.runtimeWait (dispatch.go).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A launch on a runtime with no declared startup_wait: still gets the
// pass's own default — the field is a fallback, not dead weight.
func TestDispatchUndeclaredStartupWaitUsesThePassDefault(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.StartupWait = 90 * time.Millisecond
	writePersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	// agents.json absent → herdr never sees an agent in the launched pane.

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 || !strings.Contains(out, "no agent detected") {
		t.Fatalf("want no-agent failure, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "90ms") {
		t.Errorf("an undeclared startup_wait must fall back to the pass default (90ms):\n%s", out)
	}
}

// The bead's own specimen: a runtime declaring its own startup_wait: must
// be what dispatch actually waits, not the pass's default — a pass mixing
// personas on different runtimes is wrong for whichever one this ignores.
func TestDispatchUsesTheLaunchedRuntimesDeclaredStartupWait(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// The pass default is deliberately large and round so it cannot be
	// mistaken for the runtime's own number below.
	d.StartupWait = 3 * time.Second
	d.Poll = 10 * time.Millisecond

	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.RuntimesDir(), "slowcli.yaml"),
		[]byte("command: slowcli --sys \"$(cat {file})\"\nstartup_wait: 80ms\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: slowcli\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	// agents.json absent → herdr never sees an agent in the launched pane,
	// so the launch runs out its startup wait and refuses by name.

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 || !strings.Contains(out, "no agent detected") {
		t.Fatalf("want no-agent failure, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "80ms") {
		t.Errorf("the deadline dispatch actually used must name slowcli's declared startup_wait (80ms), not the pass default (%s):\n%s", d.StartupWait, out)
	}
	if strings.Contains(out, d.StartupWait.String()) {
		t.Errorf("the refusal must not name the pass default (%s) when the runtime declared its own:\n%s", d.StartupWait, out)
	}
	// Proof the launch did not sit out the default's clock before refusing.
	// A wall-clock ceiling was the obvious way to say that and it was the
	// wrong one: this pass costs ~365ms of fake-herdr/bd forks before it
	// waits for anything, so a 1s ceiling left ~600ms of margin over a cost
	// that grows with machine load, and the test false-failed 6 of 20 runs
	// at loadavg ~24, every failure just over the ceiling and none near 3s
	// (ranger-base-2jl5, the same class as rangerhq-g6lx/3ig1).
	//
	// The detection loop asks herdr `agent list` once per d.Poll until its
	// deadline (awaitTarget → HB.AgentTarget → H.Agents), so the number of
	// asks is a COUNTABLE difference where the elapsed time was a racy one:
	// an 80ms deadline at Poll 10ms admits at most 8 asks on top of the
	// pass's own fixed handful, a 3s one admits 300. Load can only ever
	// LOWER a count — a slower box fits fewer polls into the same window —
	// so unlike a duration this cannot drift up into its ceiling. Measured
	// on darwin/arm64, 8 cores, this test alone (agent-list calls):
	//
	//	                     idle box   loadavg ~13-32
	//	80ms, as shipped     14-16      10-12
	//	3s (deadline forced
	//	from d.StartupWait)  188-192    90-104
	//
	// 30 sits above the ceiling the shipped path can structurally reach
	// (12 fixed + 8 polls) and a third of the way to the wrong path's floor
	// under the worst load measured. A count over it is not a timing flake:
	// either the deadline grew, or this pass grew new herdr calls — and the
	// second reds every box identically instead of the loaded ones.
	const maxAgentAsks = 30
	if asks := countCalls(t, fake, "agent list"); asks > maxAgentAsks {
		t.Errorf("dispatch asked herdr for an agent %d times (ceiling %d) — looks like it waited the pass default (%s) instead of slowcli's 80ms:\n%s",
			asks, maxAgentAsks, d.StartupWait, calls(t, fake))
	}
}

// countCalls is how many of the fake herdr's logged calls begin with the
// given argv prefix. Whole words from the start of the line, so a prefix
// cannot be counted inside a longer subcommand or inside a persona command
// the launch typed.
func countCalls(t *testing.T, fake, prefix string) int {
	t.Helper()
	n := 0
	for _, line := range strings.Split(calls(t, fake), "\n") {
		if line == prefix || strings.HasPrefix(line, prefix+" ") {
			n++
		}
	}
	return n
}

// The argv launch site, which the two tests above do not reach. Both
// launchSession and launchWithPrompt pass d.runtimeWait(runtime) through,
// but only the typed one is exercised there, and "the other call is the
// same expression" is the reading-not-measuring this bead exists against.
//
// A note in one QA logbook had this arm down as undetectable by
// construction, on the ground that `runtime check` refuses a runtime
// declaring both prompt: argv and startup_wait:. It does not. That rule is
// an assertion over the two BUILT-IN argv runtimes (runtimecheck_test.go:
// codex and grok must carry no startup_wait:), and nothing in LoadRuntime
// cross-checks the two keys, so a declared runtime carries both and reaches
// the argv path with its own number. Which is the more useful answer: an
// instance CAN put a startup_wait: on an argv profile, so dispatch had
// better honour it there too.
func TestDispatchArgvPathUsesTheLaunchedRuntimesDeclaredStartupWait(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.StartupWait = 3 * time.Second
	d.Poll = 10 * time.Millisecond

	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.RuntimesDir(), "slowargv.yaml"),
		[]byte("command: slowargv --rules=\"$(cat {file})\"\nprompt: argv\nstartup_wait: 80ms\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The profile really is on the argv ladder — otherwise this test is a
	// second copy of the typed one wearing a different runtime name.
	rt, err := b.App.LoadRuntime("slowargv")
	if err != nil {
		t.Fatal(err)
	}
	if rt.PromptMode() != PromptArgv {
		t.Fatalf("slowargv is on the %s ladder, so this pin does not reach launchWithPrompt", rt.PromptMode())
	}
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: slowargv\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	// agents.json absent → herdr never sees an agent, so awaitDelivered runs
	// its startup wait out and the refusal names the patience it waited.

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 || !strings.Contains(out, "no agent detected") {
		t.Fatalf("want no-agent failure, got n=%d:\n%s", n, out)
	}
	if !strings.Contains(out, "work prompt on the launch line") {
		t.Fatalf("this launch did not take the argv path, so it pins the wrong call site:\n%s", out)
	}
	if !strings.Contains(out, "80ms") {
		t.Errorf("the argv launch must wait slowargv's declared startup_wait (80ms), not the pass default (%s):\n%s", d.StartupWait, out)
	}
	if strings.Contains(out, d.StartupWait.String()) {
		t.Errorf("the refusal must not name the pass default (%s) when the runtime declared its own:\n%s", d.StartupWait, out)
	}
	// Same countable proof as the typed pin above, and the same ceiling —
	// see its comment for why a wall-clock bound is the racy way to say this.
	const maxAgentAsks = 30
	if asks := countCalls(t, fake, "agent list"); asks > maxAgentAsks {
		t.Errorf("the argv launch asked herdr for an agent %d times (ceiling %d) — looks like it waited the pass default (%s) instead of slowargv's 80ms:\n%s",
			asks, maxAgentAsks, d.StartupWait, calls(t, fake))
	}
}
