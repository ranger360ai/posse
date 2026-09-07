//go:build posse_arm3

package posse

// The live half of `posse runtime probe` — the ADR 0032 verification
// checklist, items 1 and 2, run against real CLIs in real panes. It costs one model
// turn per runtime, so it is opt-in:
//
//	RHQ_LIVE_PROBE=codex go test ./internal/posse -run TestLiveRuntimeProbe -v
//	RHQ_LIVE_PROBE=codex RHQ_LIVE_PROBE_FAKE=1 go test ./internal/posse -run TestLiveRuntimeProbe -v
//
// RHQ_LIVE_PROBE names an INSTALLED CLI, which the test redeclares as a
// template-only profile in its own RHQ_HOME — checklist item 2, "codex
// redeclared as a template profile", and the M1 criterion-3 flow end to end.
//
// RHQ_LIVE_PROBE_FAKE re-execs that same CLI THROUGH `/bin/zsh -l` —
// checklist item 1, the SILENT case (b). The probe must FAIL naming
// observable 1, because path_helper demotes the gates dir below /usr/bin in
// a login shell and the L1 shim never runs. That arm is the whole reason
// this file exists: a probe whose wrong arm passes measures nothing, and
// observable 1's wrong arm cannot be built out of anything but a real login
// shell on a real box.
//
// The arm was inert until ranger-base-385x, twice over, and both repairs are
// below. It installed its shim by mutating THIS process's PATH, which the
// pane never sees — the pane is a child of the herdr daemon and inherits its
// environment — so the pane launched the real CLI and the probe passed all
// four observables; the shim is now reached by absolute path, through the
// template profile's own `command:`, where no PATH gets a vote. And it
// worked by setting SHELL/GROK_SHELL, which ADR 0009's measured table says
// codex does not read at all (it runs `/bin/bash -c` directly); the shim now
// re-execs the login shell ITSELF, so the demotion happens before the CLI
// starts and every child inherits it, whatever the CLI reads. Measured on
// this box: `PATH=<dir>:$PATH /bin/zsh -l -c 'command -v uname'` answers
// /usr/bin/uname (2026-09-05).
//
// A wrong arm also owes a witness that its sabotage took effect. Two here,
// both checked before the arm passes judgement on the probe: a marker file
// the shim touches, and the record's own cli_path — which since
// ranger-base-385x is what the SESSION resolved, so it names the shim when
// the shim is what ran. Without them a green probe is ambiguous between "the
// wall held under a login shell" and "the shim never ran", and the red the
// arm printed accused the production probe for the second one.

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
	var shim, ran string
	if fake {
		if _, err := os.Stat("/bin/zsh"); err != nil {
			t.Skipf("no /bin/zsh on this box: %v — the demotion this arm measures is a login shell's", err)
		}
		// Checklist item 1: a CLI that hardcodes its login shell. The shim
		// is what a third-party CLI's own exec would be — it re-execs
		// `/bin/zsh -l`, which is where path_helper reorders PATH and puts
		// /usr/bin in front of whatever the launcher prepended. The CLI it
		// then starts inherits that PATH, so its children do too, whether
		// or not the CLI itself reads $SHELL.
		dir := t.TempDir()
		shim = filepath.Join(dir, name)
		ran = filepath.Join(dir, "shim-ran")
		body := "#!/bin/sh\n" +
			"# ADR 0009's silent case (b): a CLI that re-execs a LOGIN shell.\n" +
			": > " + shim2q(ran) + "\n" +
			"exec /bin/zsh -l -c 'exec \"$0\" \"$@\"' " + shim2q(real) + " \"$@\"\n"
		if err := os.WriteFile(shim, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		// The shim reaches the pane by ABSOLUTE PATH, in the profile the
		// probe launches. Mutating this process's PATH reaches nothing: the
		// pane's PATH is the herdr daemon's (ranger-base-385x). The name is
		// unchanged, so Runtime.Exe() — the basename of the template's first
		// word — is still the CLI's and observable 4 still asks herdr for it.
		command = shim + strings.TrimPrefix(command, strings.Fields(command)[0])
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
	// also how an operator onboards a re-pointed CLI (ADR 0032 §3 branch b).
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
		// FIRST: did the sabotage take effect? Answer this before judging
		// the probe, or a green record is ambiguous between "the wall held
		// under a login shell" and "the shim never ran" — and the arm spent
		// its red accusing the production probe for the second one.
		if _, err := os.Stat(ran); err != nil {
			t.Fatalf("the shim never took effect (no %s): the pane launched something else, so this arm measured nothing about the probe. NOT a probe failure", ran)
		}
		if rec.CLIPath != shim {
			t.Fatalf("the shim never took effect: the session resolved %s, not the shim at %s. NOT a probe failure", rec.CLIPath, shim)
		}
		// SECOND, and only now: the probe's verdict on a runtime whose
		// children resolve in a demoted PATH.
		if rec.Passed() {
			t.Fatal("the probe PASSED under a shim that re-execs /bin/zsh -l — observable 1 is the whole reason it exists")
		}
		byN := map[int]ProbeObservable{}
		for _, o := range rec.Observables {
			byN[o.N] = o
		}
		// 1 and 2 fail together, necessarily: path_helper puts /usr/bin in
		// front of the gates bin dir, so the canary resolves to the real
		// binary — which is both "the shim is behind it" (1) and "nothing
		// refused, so nothing reached refusals.log" (2). An arm that
		// demanded 2 stay green would be demanding a refusal from a shim
		// that never ran.
		if o := byN[1]; o.OK {
			t.Errorf("observable 1 passed under the login-shell shim: %s", o.Detail)
		}
		if o := byN[2]; o.OK {
			t.Errorf("observable 2 passed while the shim was demoted below /usr/bin: %s — a deny that refuses without its shim is a wall nobody can explain", o.Detail)
		}
		// 3 and 4 must HOLD. They are what separates "the demotion was
		// measured" from "the probe failed for the wrong reason": the CLI
		// started, took the turn unattended, and herdr saw it.
		for _, n := range []int{3, 4} {
			if o := byN[n]; !o.OK {
				t.Errorf("observable %d %s failed: %s — the probe failed for the wrong reason, so it says nothing about the demotion", n, o.Name, o.Detail)
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
	if rec.CLIPath == "" || rec.LauncherPath == "" {
		t.Errorf("the record must name the binary the SESSION resolved and the one posse did: %q / %q", rec.CLIPath, rec.LauncherPath)
	}

	// Checklist item 3: edit the recorded version and the claim goes back to
	// assumed, with `runtime check` calling for a re-probe.
	//
	// Only where the two PATHs agree. Where they do not, the version reader
	// can reach only posse's binary and ProbeState says so instead of
	// comparing across two files — the honest answer, and not one this arm
	// can turn into a drift (ranger-base-385x).
	if rec.CLIPath != rec.LauncherPath {
		t.Skipf("the session resolves %s to %s and posse resolves it to %s — version drift is not checkable from outside a pane on this box", rt.Exe(), rec.CLIPath, rec.LauncherPath)
	}
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

// shim2q single-quotes a path for the shim's own /bin/sh body. The paths are
// t.TempDir()'s and a LookPath answer, so this is a seatbelt and not a
// parser.
func shim2q(p string) string { return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'" }
