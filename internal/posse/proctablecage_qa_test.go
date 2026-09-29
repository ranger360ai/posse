//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-yxmwx — the post-background check AGENTS.md mandates
// has to be runnable by the seats that need it, which are the caged ones.
//
// Claim: on darwin SysSelfOrphans reads the process table with no fork at all,
// so it answers from inside a seatbelt sandbox, where an exec of `/bin/ps` is
// refused because that binary is setuid root.
//
// Both arms are about the ROUTE, not the rows: what this box has running is
// whatever it has, and a clean box is expected to come back empty. Arm 1 is
// cheap and hermetic (an empty PATH is a fork nobody can complete), arm 2 is
// the reported bug itself under a real sandbox, with a control so it cannot
// pass by proving nothing.

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// The minimal profile that reproduces it: seatbelt refuses a setuid exec even
// under `(allow default)`, so nothing about the persona wall is needed to pin
// this — and nothing about the persona wall can quietly fix it either.
const qaSeatbeltAllowAll = "(version 1)(allow default)"

// Arm 1. PATH empty is every fork/exec of a bare command name failing at
// lookup, which is the cheapest possible stand-in for a cage that refuses the
// exec — and it needs no sandbox, so it runs everywhere this is built. If
// somebody reintroduces the `ps` route on darwin, this reds in milliseconds.
func TestQASelfCheckTableTakesNoForkOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the setuid-ps refusal and the sysctl route are both darwin's")
	}
	t.Setenv("PATH", "")
	leaks, err := SysSelfOrphans()
	if err != nil {
		t.Fatalf("SysSelfOrphans must not need anything on PATH: %v", err)
	}
	for _, p := range leaks {
		if p.PPID != 1 || p.Age < SelfCheckMinAge {
			t.Errorf("a returned leak must satisfy its own predicate: %+v", p)
		}
	}
}

// Arm 2. The bug as reported: `go run ./cmd/checkorphans` from a seatbelt seat
// answered "fork/exec /bin/ps: operation not permitted", exit 2, leak status
// unknown. This runs the check inside the same kind of sandbox, in a child
// because a sandbox is a property of a process and cannot be entered from
// inside one.
func TestQASelfCheckAnswersInsideASeatbelt(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" {
		t.Skip("seatbelt is darwin's")
	}
	sandboxExec, err := exec.LookPath("sandbox-exec")
	if err != nil {
		t.Skipf("no sandbox-exec on this box: %v", err)
	}
	// The control. If a setuid exec is NOT refused here — a future macOS, or
	// a cage that will not nest another sandbox — then the child below would
	// pass whichever route it took, and a green would mean nothing.
	if out, err := exec.Command(sandboxExec, "-p", qaSeatbeltAllowAll, "/bin/ps", "-axo", "pid=").Output(); err == nil && len(out) > 0 {
		t.Skip("this box's sandbox does not refuse the setuid exec of /bin/ps — nothing left to pin here")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(sandboxExec, "-p", qaSeatbeltAllowAll, exe, "-test.run=TestQASelfCheckSandboxedChild$", "-test.v")
	cmd.Env = append(qaEnvWithoutFakeSubstrate(), "RHQ_QA_SELFCHECK_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the self-check must answer from inside a sandbox (%v):\n%s", err, out)
	}
	// A child that skipped is a child that never ran the check: the env
	// aliasing that costs a caged-launch test its point (cagehomelock).
	if strings.Contains(string(out), "--- SKIP") {
		t.Fatalf("the child skipped rather than reading the table:\n%s", out)
	}
}

// qaEnvWithoutFakeSubstrate is this binary's environment minus the switch
// TestMain sets on itself: with RHQ_FAKE_HERDR inherited, a re-exec of the
// test binary becomes the fake bd and answers "unhandled -test.run=…" instead
// of running anything (the same aliasing trustlock_qa_test.go's qaSeederEnv
// strips, spelled here because that helper lives in another arm).
func qaEnvWithoutFakeSubstrate() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RHQ_FAKE_HERDR=") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

// The child of TestQASelfCheckAnswersInsideASeatbelt, run under sandbox-exec.
// It also proves the control's other half from inside: the `ps` this route
// replaced really is refused in here.
func TestQASelfCheckSandboxedChild(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_QA_SELFCHECK_CHILD") == "" {
		t.Skip("child of TestQASelfCheckAnswersInsideASeatbelt")
	}
	if _, err := exec.Command("ps", "-axo", "pid=").Output(); err == nil {
		t.Fatal("this child is not sandboxed the way the parent thinks: `ps` ran")
	}
	if _, err := SysSelfOrphans(); err != nil {
		t.Fatalf("SysSelfOrphans from inside the sandbox: %v", err)
	}
}
