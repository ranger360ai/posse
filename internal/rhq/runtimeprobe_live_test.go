package rhq

// The live half of `posse runtime probe` — laurie's ADR 0017 checklist,
// items 1 and 2, run against real CLIs in real panes. It costs one model
// turn per runtime, so it is opt-in:
//
//	RHQ_LIVE_PROBE=codex go test ./internal/rhq -run TestLiveRuntimeProbe -v
//	RHQ_LIVE_PROBE=codex RHQ_LIVE_PROBE_FAKE=1 go test ./internal/rhq -run TestLiveRuntimeProbe -v
//
// RHQ_LIVE_PROBE names an INSTALLED CLI, which the test redeclares as a
// template-only profile in its own RHQ_HOME — checklist item 2, "codex
// redeclared as a template profile", and the M1 criterion-3 flow end to end.
//
// RHQ_LIVE_PROBE_FAKE wraps that same CLI in a shim that hardcodes
// `/bin/zsh -l` for its child commands — checklist item 1, the SILENT case
// (b). The probe must FAIL naming observable 1, because path_helper demotes
// the gates dir below /usr/bin in a login shell and the L1 shim never runs.
// That arm is the whole reason this file exists: a probe whose wrong arm
// passes measures nothing, and observable 1's wrong arm cannot be built out
// of anything but a real login shell on a real box.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveRuntimeProbe(t *testing.T) {
	name := os.Getenv("RHQ_LIVE_PROBE")
	if name == "" {
		t.Skip("set RHQ_LIVE_PROBE=<installed CLI, e.g. codex> (+ HERDR_SOCKET_PATH / RHQ_HERDR_BIN for a scratch herdr) — see the file comment")
	}
	h := NewHerdr()
	if !h.Available() {
		t.Skip("no herdr on PATH — observable 4 cannot be read")
	}
	real, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is not installed: %v", name, err)
	}

	home := t.TempDir()
	a := NewAppAt(home)
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// The line is the built-in's, borrowed: the point of the exercise is
	// that the SAME CLI, declared as a template rather than compiled in,
	// gets no free wall claim until it is probed.
	builtin, err := a.LoadRuntime(name)
	if err != nil {
		t.Fatal(err)
	}
	command := builtin.Command
	fake := os.Getenv("RHQ_LIVE_PROBE_FAKE") != ""
	if fake {
		// Checklist item 1: a CLI that hardcodes its login shell. The shim
		// is what a third-party CLI's own exec would be — it re-execs
		// `/bin/zsh -l`, which is where path_helper reorders PATH and puts
		// /usr/bin in front of whatever the launcher prepended.
		dir := t.TempDir()
		shim := filepath.Join(dir, name)
		body := "#!/bin/sh\n# a CLI that ignores $SHELL — ADR 0009's silent case (b)\nSHELL=/bin/zsh GROK_SHELL=/bin/zsh exec " + real + " \"$@\"\n"
		if err := os.WriteFile(shim, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	profile := "command: " + command + "\n"
	if p := builtin.PromptMode(); p == PromptArgv {
		profile += "prompt: argv\n"
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), name+".yaml"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	// LoadRuntime returns a BUILT-IN before it stats the yaml, so the
	// profile has to be loaded under a name no built-in owns — which is
	// also how an operator onboards a re-pointed CLI (ADR 0017 §3 branch b).
	tmpl := name + "-tmpl"
	if err := os.Rename(filepath.Join(a.RuntimesDir(), name+".yaml"), filepath.Join(a.RuntimesDir(), tmpl+".yaml")); err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Builtin {
		t.Fatalf("%s must load as template-only", tmpl)
	}

	// Before: the wall claim is assumed, and a PID with a shell-verb deny
	// degrades on it.
	dev := loadTestAgent(t, "---\nname: dev\ndeny: [Bash(rm -rf /)]\n---\nYou are dev.\n")
	if p := a.CheckParity(dev, rt, CageShims, TierStrong); len(p.Degraded) == 0 {
		t.Fatalf("an unprobed template profile must degrade a Bash deny: %+v", p)
	}

	rec, err := a.RuntimeProbe(rt, h, ProbeOpts{Timeout: 5 * time.Minute, Out: os.Stderr})
	if err != nil {
		t.Fatalf("probe could not run: %v", err)
	}
	for _, o := range rec.Observables {
		t.Logf("observable %d %s ok=%v — %s", o.N, o.Name, o.OK, o.Detail)
	}

	if fake {
		// The wrong arm. It must fail, and it must fail on observable 1
		// specifically: a probe that failed for any other reason (the CLI
		// did not start, herdr did not see it) proves nothing about the
		// demotion this arm exists to catch.
		if rec.Passed() {
			t.Fatal("a CLI that hardcodes /bin/zsh -l must FAIL the probe — observable 1 is the whole reason it exists")
		}
		for _, o := range rec.Observables {
			if o.N == 1 && o.OK {
				t.Errorf("observable 1 passed under the login-shell shim: %s", o.Detail)
			}
			if o.N != 1 && !o.OK {
				t.Errorf("only observable 1 should fail in this arm; %d %s also failed: %s — the probe failed for the wrong reason", o.N, o.Name, o.Detail)
			}
		}
		if p := a.CheckParity(dev, rt, CageShims, TierStrong); len(p.Degraded) == 0 {
			t.Error("a failed probe must leave the launch degraded")
		}
		return
	}

	if !rec.Passed() {
		t.Fatalf("%s redeclared as a template profile must pass all four observables: %v", name, rec.Failures())
	}
	// After: the same PID on the same profile is clean, and the record says
	// which binary was measured.
	if p := a.CheckParity(dev, rt, CageShims, TierStrong); len(p.Degraded) != 0 {
		t.Errorf("a passing probe must clear the degradation: %+v", p.Degraded)
	}
	if rec.CLIPath == "" {
		t.Error("the record must name the binary it measured")
	}

	// Checklist item 3: edit the recorded version and the claim goes back to
	// assumed, with `runtime check` calling for a re-probe.
	if rec.Version == "" {
		t.Logf("%s prints no version — the drift arm cannot be exercised here", rt.Exe())
		return
	}
	rec.Version = rec.Version + "-stale"
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
	st := a.ProbeState(rt)
	if st.Current || !st.Drift || !strings.Contains(st.Why, "re-run") {
		t.Errorf("a stale recorded version must call for a re-probe: %+v", st)
	}
	if p := a.CheckParity(dev, rt, CageShims, TierStrong); len(p.Degraded) == 0 {
		t.Error("version drift puts the Bash claim back to assumed")
	}
}
