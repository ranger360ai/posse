//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-d0nyb — a probe answers on the PATH the thing it predicts will
// be RESOLVED on.
//
// THE DEFECT, which is ranger-base-g1xnt's one surface out: posse's own
// children are resolved outside every gates dir (ADR 0077 D3, protocolBin),
// and since ranger-base-cwsi8 a bare name nothing out there answers is an
// ERROR. Four probes still asked exec.LookPath, which searches THIS process's
// $PATH — the gated one. A posse process that dispatches, checks parity or
// renders a wall routinely has another persona's gates bin dir leading that
// PATH (MEASURED 2026-10-31, ranger-base-c33j6: a gates dir mid-PATH that the
// launching process did not put there), and a PID that denies a herdr verb or
// a container engine has a shim rendered in that dir. So the probe found the
// shim, said yes, and the call it cleared then failed at protocolBin: a wrong
// yes followed by a right no, with the plan already made on the yes.
//
// NOT FIRING ON THIS BOX TODAY, which is why this pin is the whole evidence
// and the same margin ranger-base-cwsi8 measured for `bd`: a real herdr and a
// real docker both sit on PATH outside every gates dir and no PID here denies
// either, so nothing renders a shim for a probe to find. The fixture below is
// that box — one gates bin dir that answers the name, and not one ordinary
// element that does.
//
// BOTH DIRECTIONS, because "unavailable" is what a probe reading a PATH it
// cannot resolve anything on reports, and a resolveOutside that answered ""
// for everything would pass every arm of the negative half alone. Each arm
// below runs its own positive control on a PATH whose non-gates element DOES
// hold the binary.
//
// Serial, like TestBdBinUnresolvableOutsideTheGatesIsAnError next door: these
// set $PATH, which is one variable shared by every goroutine in the process.
//
// MUTATION-CHECKED 2026-11-01 (darwin 25.6.0, this worktree), each mutant
// against the arms it should red:
//   - Herdr.Available back to `exec.LookPath(h.Bin)` reds
//     TestHerdrAvailableAgreesWithTheProtocolResolution's negative arms and
//     nothing else; the positive controls stay green, which is the point of
//     having them.
//   - engineOnPath back to `exec.LookPath(e.Binary())` reds all three arms of
//     TestEngineProbesAgreeWithTheProtocolResolution — and only the gated
//     halves of them.
//   - engineOnPath returning false unconditionally reds the three positive
//     controls alone.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shimmedPATH is the box the bead describes: PATH = one gates bin dir holding
// an executable called name, then one ordinary dir holding nothing. It returns
// the gates bin dir and the ordinary one, so an arm can plant the real binary
// in the second half for its positive control.
//
// The gates dir is spelled with a 'gates' path ELEMENT because that is how
// posse spells "ours" (isGatesPath) — a dir merely called gatesy is not one,
// and a fixture that got this wrong would be testing an ordinary PATH.
func shimmedPATH(t *testing.T, name string) (gatesBin, plain string) {
	t.Helper()
	gatesBin = filepath.Join(t.TempDir(), "state", "gates", "launcher", "bin")
	plain = filepath.Join(t.TempDir(), "bin")
	for _, d := range []string{gatesBin, plain} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if !isGatesPath(gatesBin) {
		t.Fatalf("%s is not read as a gates path — the fixture is an ordinary PATH and measures nothing", gatesBin)
	}
	if err := WriteExecutable(filepath.Join(gatesBin, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", gatesBin+string(os.PathListSeparator)+plain)
	return gatesBin, plain
}

// plantReal puts the binary in the ordinary half of the PATH — the positive
// control's one difference from the arm above it.
func plantReal(t *testing.T, plain, name string) {
	t.Helper()
	if err := WriteExecutable(filepath.Join(plain, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// Herdr.Available must answer what capture's protocolCmd would resolve.
//
// The second half is the cost that made this worth fixing rather than leaving
// to fail per call. HasReportAgent, AgentManifest and KnownAgentKinds all
// document "not available is UNKNOWN, never a no" — a launch must not be
// refused because a diagnostic verb went missing. A protocolBin refusal
// arriving inside capture() is not a *HerdrHangError, so HasReportAgent's one
// exemption misses it and the verb reads as ABSENT: present=false, ok=TRUE,
// a confident wrong no. With the probe agreeing up front the caller gets the
// abstention it was written for.
func TestHerdrAvailableAgreesWithTheProtocolResolution(t *testing.T) {
	_, plain := shimmedPATH(t, "herdr")
	h := Herdr{Bin: "herdr"}

	if _, err := protocolBin(h.Bin); err == nil {
		t.Fatal("protocolBin resolved `herdr` outside the gates — the fixture no longer has a gated-only herdr and NOTHING IS MEASURED")
	}
	if h.Available() {
		t.Error("Available() = true where protocolBin refuses: the probe read THIS process's gated $PATH and found the launching persona's herdr shim (ADR 0077 D3, ranger-base-d0nyb)")
	}
	if (HerdrMux{H: h}).Available() {
		t.Error("HerdrMux.Available() = true — runtimeprobe.go's gate reads this one, and it is Herdr.Available verbatim")
	}
	if present, ok := h.HasReportAgent(); ok {
		t.Errorf("HasReportAgent() = (%v, ok=true) — a protocolBin refusal is not clap refusing a subcommand, and this file's callers must read an unaskable herdr as UNKNOWN and never as a no", present)
	}

	// The positive control: the same gates shim still sitting first on PATH,
	// and now a real herdr in the ordinary element behind it.
	plantReal(t, plain, "herdr")
	if got, err := protocolBin(h.Bin); err != nil || filepath.Dir(got) != plain {
		t.Fatalf("protocolBin = %q, %v — want the ordinary dir's herdr; the control cannot measure a yes", got, err)
	}
	if !h.Available() {
		t.Error("Available() = false with a real herdr on the gates-free PATH — the probe now refuses what protocolBin resolves, which is the same disagreement the other way round")
	}
}

// The three engine probes must answer what WrapInCage's protocolBin will.
//
// All three are read BEFORE something expensive or irreversible: parity and
// the launch planner read ContainerAvailable, CageNotReady/CageEngineNotReady
// read cageNoBinary for the sentence an operator acts on, and BuildCageImage
// cross-builds two Linux binaries past its own. A yes here that WrapInCage
// refuses is a plan made on a probe, which is the hazard cage.go's own comment
// at WrapInCage already named and gave the same thin margin.
func TestEngineProbesAgreeWithTheProtocolResolution(t *testing.T) {
	const engine = "posse-fake-cage-engine"
	_, plain := shimmedPATH(t, engine)

	a := cageApp(t)
	if err := os.WriteFile(filepath.Join(a.CagesDir(), "shimmed.yaml"),
		[]byte("command: "+engine+" run {mounts} {env} -w {workdir} {image} {cmd}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_engine: shimmed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		t.Fatal(err)
	}
	if e.Binary() != engine {
		t.Fatalf("engine binary = %q, want %q — the fixture engine is not the one the shim answers for", e.Binary(), engine)
	}
	if _, err := protocolBin(e.Binary()); err == nil {
		t.Fatalf("protocolBin resolved %s outside the gates — NOTHING IS MEASURED", engine)
	}

	// cage.go:ContainerAvailable
	if a.ContainerAvailable() {
		t.Error("ContainerAvailable() = true where protocolBin refuses — the tier reads available and WrapInCage then refuses the launch it cleared (ranger-base-d0nyb)")
	}
	// cage.go:cageNoBinary — and the sentence has to name the narrower PATH,
	// because an operator looking at the shim can see the engine IS on PATH.
	why := a.cageNoBinary(e)
	if why == "" {
		t.Error("cageNoBinary() = \"\" — CageNotReady and CageEngineNotReady both read this for the reason they print")
	} else if !strings.Contains(why, "outside posse's gates dirs") {
		t.Errorf("cageNoBinary() = %q — it must say WHICH path, or it tells an operator a binary they can see on PATH is not on PATH", why)
	}
	// cage.go:BuildCageImage — refused before it stats the Dockerfile, so the
	// cross-build cannot be the thing that discovers this. src is deliberately
	// absent: the error naming the ENGINE and not the checkout is the proof of
	// the order.
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	berr := a.BuildCageImage(filepath.Join(t.TempDir(), "no-such-checkout"), "", devnull)
	if berr == nil {
		t.Fatal("BuildCageImage() = nil on a host with no engine outside the gates")
	}
	if !strings.Contains(berr.Error(), engine) {
		t.Errorf("BuildCageImage refused with %q — want the engine named, which is also the proof the probe ran before the Dockerfile stat and the two cross-builds", berr)
	}

	// The positive controls, all three, with the gates shim still first.
	plantReal(t, plain, engine)
	if got, err := protocolBin(e.Binary()); err != nil || filepath.Dir(got) != plain {
		t.Fatalf("protocolBin = %q, %v — want the ordinary dir's engine; the controls cannot measure a yes", got, err)
	}
	if !a.ContainerAvailable() {
		t.Error("ContainerAvailable() = false with the engine on the gates-free PATH")
	}
	if why := a.cageNoBinary(e); why != "" {
		t.Errorf("cageNoBinary() = %q with the engine on the gates-free PATH", why)
	}
	// Now the refusal must have moved PAST the engine: this host has the
	// binary and no checkout, so the sentence is the Dockerfile's.
	berr = a.BuildCageImage(filepath.Join(t.TempDir(), "no-such-checkout"), "", devnull)
	if berr == nil {
		t.Fatal("BuildCageImage() = nil with no posse checkout at src")
	}
	if strings.Contains(berr.Error(), "on PATH outside posse's gates dirs") {
		t.Errorf("BuildCageImage still refuses on the engine with it on the gates-free PATH: %v", berr)
	}
}
