package posse

// ranger-base-eupf: a PID that denies the binary the RUNTIME reads and
// writes its own credential with shims the runtime, not the persona.
//
// THE DEFECT. The gates dir is prepended on the TYPED line (ADR 0002 §3), so
// it leads the PATH of the runtime process itself and of everything that
// process spawns — not only of the persona's shells. claude's credential
// path execs `security` by BARE NAME, so under the crew's Bash(security:*)
// the runtime's own read resolved to this persona's refusal shim. MEASURED
// in a live session 2026-08-29: `claude -p` answers "Not logged in", and the
// same line with the gates dir stripped off PATH answers normally. The gate
// logs carried 875 such refusals across the crew since 2026-08-24 and
// nothing anywhere said whose they were.
//
// THE FIX PINNED HERE is the launch saying it: a launch whose PID shims the
// chosen runtime's credential binary warns, naming the rule, the binary and
// BOTH consequences — the logged-out session now, and the refresh that
// cannot write back at token expiry. A warning and not a refusal, because
// every crew PID carries this deny today.
//
// Why not the narrowed carve-out the bead sketched as its other arm: the
// credential WRITE goes primarily through `security -i`, that binary's stdin
// batch mode (measured on the darwin release artifact, claude 2.1.258). An
// argv matcher cannot see a command that arrives on stdin, so a shim that
// let the write through to keep refresh alive would be no deny at all, and
// one that let only the reads through would fix the logged-out symptom and
// leave expiry — the half nobody has ever watched — exactly where it is.
// That is a decision about the wall, not a matcher detail.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cgcRuntime is a runtime declaring cmd as its credential binary.
func cgcRuntime(name, cmd string) *Runtime { return &Runtime{Name: name, CredBin: cmd} }

// cgcBinDir is a shim dir that exists but holds nothing — resolveOutside
// needs a directory to exclude, not a rendered gate.
func cgcBinDir(t *testing.T) string {
	t.Helper()
	d := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

// cgcRealBin is a command that exists on this box outside the gates, so the
// collision test's "would a shim actually stand in front of a binary" arm is
// answered by the platform rather than assumed. `sh` is the one command this
// package already requires everywhere.
func cgcRealBin(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	return "sh"
}

// The positive arm: a PID denying the runtime's credential binary collides,
// and the collision names the PID's own rule verbatim.
func TestQACredGateCollisionNamesTheRuleThatShimsTheRuntimesCredentialBinary(t *testing.T) {
	bin := cgcRealBin(t)
	rt := cgcRuntime("testcli", bin)

	got := CredGateCollision(rt, []string{"Edit", "Bash(" + bin + ":*)"}, cgcBinDir(t))
	if got != "Bash("+bin+":*)" {
		t.Fatalf("the collision must name the PID's rule verbatim, got %q", got)
	}
}

// A whole-binary rule is the one that refuses every form, so it is the rule
// reported even when the PID also carries a narrower one written first — a
// launch warning that quoted `Bash(sh -c:*)` while `Bash(sh)` was what
// refused the runtime would send the reader to the wrong line of the PID.
func TestQACredGateCollisionPrefersTheWholeBinaryRule(t *testing.T) {
	bin := cgcRealBin(t)
	rt := cgcRuntime("testcli", bin)

	got := CredGateCollision(rt, []string{"Bash(" + bin + " -c:*)", "Bash(" + bin + ")"}, cgcBinDir(t))
	if got != "Bash("+bin+")" {
		t.Fatalf("want the whole-binary rule, got %q", got)
	}
}

// Three silences, each of which a warning that fired anyway would turn into
// noise on every launch: a PID that denies something else, a runtime that
// declares no credential binary, and a deny over a binary this box does not
// have — the platform arm, which is what keeps a `security` rule from
// warning on a host whose runtime reads a file instead.
func TestQACredGateCollisionIsSilentWhenThereIsNothingToCollide(t *testing.T) {
	bin := cgcRealBin(t)
	binDir := cgcBinDir(t)
	deny := []string{"Bash(" + bin + ":*)"}

	for _, c := range []struct {
		name string
		rt   *Runtime
		deny []string
	}{
		{"the PID denies another binary", cgcRuntime("testcli", bin), []string{"Bash(git push:*)", "Edit"}},
		{"the runtime declares no credential binary", cgcRuntime("testcli", ""), deny},
		{"no runtime at all", nil, deny},
		{"the binary is not on this box", cgcRuntime("testcli", "no-such-binary-anywhere-eupf"), []string{"Bash(no-such-binary-anywhere-eupf:*)"}},
	} {
		if got := CredGateCollision(c.rt, c.deny, binDir); got != "" {
			t.Errorf("%s: want no collision, got %q", c.name, got)
		}
	}
}

// The warning's substance. A line that says only "this session is logged
// out" is the half-truth this bead exists to kill: a reader whose session
// works reads it as the all-clear, and the half that has never been watched
// is the refresh at token expiry.
func TestQACredGateWarningNamesBothConsequencesNotJustTheLoggedOutOne(t *testing.T) {
	rt := cgcRuntime("testcli", "keybin")
	got := CredGateWarning("ranger", rt, "Bash(keybin:*)")

	for _, want := range []string{"ranger", "testcli", "Bash(keybin:*)", "keybin", "expiry", "write back"} {
		if !strings.Contains(got, want) {
			t.Errorf("the launch warning never names %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "WRITES") {
		t.Errorf("the warning must say the runtime WRITES through this binary too — a read-only reading is what makes the expiry half invisible:\n%s", got)
	}
}

// The consumer: an ordinary launch, on the default runtime, under a PID that
// carries the crew's real deny. This is the arm that fails if the detector
// is right and nothing calls it.
func TestQALaunchWarnsWhenThePIDShimsTheRuntimesCredentialBinary(t *testing.T) {
	b, _ := newTestBackend(t)
	a := b.App
	rt, err := a.LoadRuntime(DefaultRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CredBin == "" {
		t.Skipf("%s declares no credential binary", rt.Name)
	}
	if _, err := exec.LookPath(rt.CredBin); err != nil {
		t.Skipf("%s is not on this box", rt.CredBin)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndeny: [Bash("+rt.CredBin+":*)]\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("the launch was refused rather than warned: %v", err)
	}
	got := warn.String()
	if !strings.Contains(got, "Bash("+rt.CredBin+":*)") || !strings.Contains(got, "expiry") {
		t.Fatalf("a launch whose PID shims %s's own credential binary said nothing about it:\n%s", rt.Name, got)
	}
}

// The control on that consumer: the same launch, same runtime, a PID that
// does not carry the deny, and the line is not printed. Without this arm a
// warning that fired on every launch would pass the pin above.
func TestQALaunchIsQuietWhenThePIDDoesNotShimTheCredentialBinary(t *testing.T) {
	b, _ := newTestBackend(t)
	a := b.App
	rt, err := a.LoadRuntime(DefaultRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CredBin == "" {
		t.Skipf("%s declares no credential binary", rt.Name)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndeny: [Bash(git push:*)]\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatal(err)
	}
	if got := warn.String(); strings.Contains(got, rt.CredBin) {
		t.Fatalf("a PID that does not deny %s must not draw the credential-gate warning:\n%s", rt.CredBin, got)
	}
}

// The declaration itself is the measurement's only home, so it is pinned
// where a reader looks for it: the default runtime names the binary its own
// credential path execs. A build that drops it turns every launch quiet
// again with nothing failing.
func TestQADefaultRuntimeDeclaresItsCredentialBinary(t *testing.T) {
	b, _ := newTestBackend(t)
	rt, err := b.App.LoadRuntime(DefaultRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CredBin != "security" {
		t.Fatalf("%s declares CredBin %q — measured on the darwin release artifact it is `security` (ranger-base-eupf)", rt.Name, rt.CredBin)
	}
}
