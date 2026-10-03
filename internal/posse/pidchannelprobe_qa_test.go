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
	"bytes"
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

// QA pin for ranger-base-dtm44 — the SIBLING finding of the one above, and
// the other half of ADR 0062 D3's exemption.
//
// The exemption is two clauses: the `pid` gap does not refuse the probe
// (pinned above), and "it warns below instead, in the same words an
// interactive launch warns in" — runtimeprobe.go's
// `fmt.Fprintf(out, "  %s\n", PIDChannelDegraded(...))`. THE ESCAPE, MEASURED
// 2026-10-03 by census under ranger-base-ksfk6: deleting that Fprintf left
// every pin in the tree green. `grep -rn PIDChannelDegraded` over the
// package's tests returned nothing, and no test passed a capturable Out to
// ProbeOpts — the only non-nil one was runtimeprobe_live_test.go's os.Stderr,
// which nothing reads back. Every other DEGRADED assertion in the package is
// a different site: detection_qa_test.go, singletreerefresh_qa_test.go, and
// pidchannel_qa_test.go's INTERACTIVE launch, which is the warn this one is
// required to agree with.
//
// The warning is load-bearing for the same reason the exemption is. With
// dispatch refusing by name, the probe is the one surface that still opens a
// pane on such a runtime, and a reader comparing the probe's four observables
// to a real session's has to be told this pane has no persona in it. Silence
// there reads as a normal probe.
//
// ranger-base-dtm44's own description predicted this needed arm 3's
// probeParityApp, on the reading that the Fprintf "sits past `herdr workspace
// create`". MEASURED 2026-10-03, it does not: the warn is printed from the
// rendered template, above CheckCredGate and h.CreateWorkspace, so the arm
// above's fake herdr — present, and failing every call — reaches it. The arm
// below is that fixture with a buffer on Out. Noted on the bead; the finding
// was right and its reason was not.
func TestQAProbeWarnsOnThePIDChannelInTheWordsAnInteractiveLaunchUses(t *testing.T) {
	t.Parallel()

	// A herdr that EXISTS and fails every call: Available() is true, so the
	// probe runs the whole render — including the warn — and then stops at
	// `workspace create`, before anything needs a real CLI.
	bin := filepath.Join(t.TempDir(), "herdr")
	if err := WriteExecutable(bin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := Herdr{Bin: bin}
	if !h.Available() {
		t.Fatal("the fake herdr is not Available — this arm would measure nothing")
	}

	probe := func(t *testing.T, command string) (*Runtime, string, error) {
		t.Helper()
		home := t.TempDir()
		a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
		if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "carol.yaml"), []byte("command: "+command+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rt, err := a.LoadRuntime("carol")
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		_, perr := a.RuntimeProbe(rt, h, ProbeOpts{Timeout: time.Second, Out: &out})
		return rt, out.String(), perr
	}

	// THE ASSERTION. A template carrying neither channel, and the screen must
	// carry the interactive warn VERBATIM — PIDChannelDegraded with the
	// runtime half of the `where`, which is the reading `runtime check`
	// prints, because the scratch PID carries no `command:` of its own.
	rt, screen, err := probe(t, "posse-wtavx-no-such-exe --auto-approve")
	if len(rt.PIDChannels(rt.Command)) != 0 {
		t.Fatalf("fixture no longer delivers no PID — NOTHING MEASURED: %v", rt.PIDChannels(rt.Command))
	}
	if err == nil {
		t.Fatal("this arm needs the probe to stop at `workspace create`, past the warn")
	}
	if !strings.Contains(err.Error(), "workspace create") {
		t.Fatalf("the probe stopped before `workspace create`, so it may have stopped before the warn too and this arm measures nothing: %v", err)
	}
	want := "  " + PIDChannelDegraded(rt, runtimeTemplateWhere(rt)) + "\n"
	if !strings.Contains(screen, want) {
		t.Errorf("the probe opened a pane with no persona in it and said nothing — ADR 0062 D3 exempts the `pid` gap on the promise that the probe %q instead, so a silent probe is the exemption with its other half deleted.\nwant line:\n%s\ngot screen:\n%s", "warns below in the same words an interactive launch warns in", want, screen)
	}
	// The same words, not merely a DEGRADED-shaped sentence: herdrback.go's
	// interactive warn is the one this must agree with, and both read one
	// PIDChannelDegraded so they cannot drift. Asserted so that a second
	// spelling added here has to say why.
	if !strings.Contains(screen, "an interactive launch proceeds because your own keyboard can paste the PID") {
		t.Errorf("the warn is not the interactive one's words:\n%s", screen)
	}

	// THE NEGATIVE CONTROL, and the proof Out is wired at all. A template
	// that DOES deliver a PID must not carry the line — and the canary line
	// must be there, so that "absent" is a reading of a captured screen and
	// not of a buffer nothing ever wrote to.
	okRT, okScreen, okErr := probe(t, "posse-wtavx-no-such-exe --pid {file}")
	if len(okRT.PIDChannels(okRT.Command)) == 0 {
		t.Fatalf("the control fixture delivers no PID either — it controls nothing")
	}
	if okErr == nil || !strings.Contains(okErr.Error(), "workspace create") {
		t.Fatalf("the control did not reach `workspace create`, so passing the warn was never shown: %v", okErr)
	}
	if !strings.Contains(okScreen, "probe carol — canary ") {
		t.Fatalf("no progress line on the control's screen — Out is not captured, so every absence below is vacuous:\n%s", okScreen)
	}
	if strings.Contains(okScreen, "DEGRADED") {
		t.Errorf("a template that delivers a PID was warned about anyway, so the warn is unconditional and says nothing about the channel:\n%s", okScreen)
	}
}
