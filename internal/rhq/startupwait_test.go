package rhq

// ranger-base-p84: Runtime.StartupWait was declared, parsed, validated,
// documented and PRINTED by `runtime check` — and dispatch never read it.
// Dispatcher.StartupWait was set once from the DefaultStartupWait constant
// and every launch in a pass waited that long, whatever its own runtime
// declared. runtimecheck_test.go already pins the getter (rt.Wait()); this
// drives dispatch itself, so a disagreement between what `runtime check`
// prints and what a launch actually waits shows up here, not just there.
//
// richard's design note on ranger-base-il14: one Dispatcher fires every
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
	b, _ := newTestBackend(t)
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

	start := time.Now()
	n, err := d.Run("", "", 0)
	elapsed := time.Since(start)
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
	// A generous ceiling, well under the 3s pass default: proof the launch
	// did not actually sit out the default's clock before refusing.
	if elapsed > 1*time.Second {
		t.Errorf("dispatch took %s to refuse — looks like it waited the pass default (%s) instead of slowcli's 80ms", elapsed, d.StartupWait)
	}
}
