//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-ksfk6 — the arm ranger-base-wtavx's probe pin could
// not make.
//
// ADR 0062 D3 asks for two things that are only jointly satisfiable by an
// exemption: the preflight's `pid` gap must BLOCK (`runtime check`'s exit code
// is the onboarding gate, and a runtime delivering no PID really cannot take
// dispatched work), and `posse runtime probe` must still run on it, because the
// probe renders a persona line and is the surface the instance side measures
// the channel that LIFTS the dispatch refusal with. runtimeprobe.go carries it
// as `g.Blocking && g.Name != PIDChannelGapName`.
//
// THE ESCAPE, MEASURED 2026-10-03 under ranger-base-ksfk6: deleting that `&& g.Name != …`
// left TestQAPIDChannelGapBlocksTheGridAndNotTheProbe GREEN. Its fixture
// builds a Herdr whose Bin does not exist, and RuntimeProbe's `!h.Available()`
// refusal sits ABOVE the blocking-gap loop the exemption lives in — so the
// loop is never reached, the exemption is never evaluated, and the arm's
// `!Contains(err, "cannot be probed: pid")` is true of a probe that stopped
// three statements earlier. The shipped arm's own comment reasons from that
// shadowing ("the probe refuses here for a different reason — no herdr ... and
// that is the point") and the reasoning inverts: a refusal before the loop
// does not prove the loop is kind, it proves the loop was not asked.
//
// So this arm gives the fixture a herdr that IS Available, and then asserts
// the PAIR: no blocking gap refused the probe at all, AND the probe got far
// enough to invoke herdr — without that second half, "the error does not name
// the pid gap" is satisfied by every earlier exit, which is exactly how the
// shipped arm passes over the mutant.
import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQAProbeIsNotRefusedByThePIDChannelGapWithAHerdrPresent(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// The same reserved argv0 spelling the shipped arm uses: a name nothing on
	// this box resolves, so no arm here can reach a real CLI. It carries
	// neither {file} nor {mode}, which is the whole fixture.
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "carol.yaml"),
		[]byte("command: posse-wtavx-no-such-exe --auto-approve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime("carol")
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.PIDChannels(rt.Command)) != 0 {
		t.Fatalf("fixture no longer delivers no PID — NOTHING MEASURED: %v", rt.PIDChannels(rt.Command))
	}

	// THE SHADOW CONTROL. This is the shipped arm's fixture, and it must stop
	// at the herdr check — above the loop. It is asserted here so that the day
	// RuntimeProbe reorders those two, this file says so instead of quietly
	// becoming a duplicate of an arm that has regained its teeth.
	noHerdr := Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}
	if noHerdr.Available() {
		t.Fatal("the no-herdr control resolved a herdr — it would reach the gap loop and stop being a control")
	}
	_, errNo := a.RuntimeProbe(rt, noHerdr, ProbeOpts{Timeout: time.Second})
	if errNo == nil || !strings.Contains(errNo.Error(), "herdr is not on PATH") {
		t.Errorf("the no-herdr fixture no longer stops above the blocking-gap loop, so a pin written against it can no longer be assumed blind to the exemption — re-read TestQAPIDChannelGapBlocksTheGridAndNotTheProbe: %v", errNo)
	}

	// THE ASSERTION. A herdr that exists and fails every call: Available() is
	// true, so the blocking-gap loop is reached, and nothing past it can
	// succeed — which is what makes the error readable.
	bin := filepath.Join(t.TempDir(), "herdr")
	if err := WriteExecutable(bin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := Herdr{Bin: bin}
	if !h.Available() {
		t.Fatal("the fake herdr is not Available — this arm would measure nothing")
	}
	_, err = a.RuntimeProbe(rt, h, ProbeOpts{Timeout: time.Second})
	if err == nil {
		t.Fatal("this arm needs the probe to stop somewhere past the gap loop, so the sentence can be read")
	}
	if strings.Contains(err.Error(), "cannot be probed: ") {
		t.Errorf("a blocking gap refused the probe — the `pid` gap must be exempt BY NAME, because the probe is how the channel that lifts the dispatch refusal gets measured, and a probe that refused here could never measure its way out (ADR 0062 D3): %v", err)
	}
	// The other half of the pair: proof the loop was REACHED and cleared,
	// without which the assertion above is true of every earlier exit — the
	// defect this file exists for.
	if !strings.Contains(err.Error(), "workspace create") {
		t.Errorf("the probe did not reach herdr, so clearing the gap loop was never shown; this arm measured nothing: %v", err)
	}
}
