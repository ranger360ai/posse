package rhq

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The live half of ranger-base-c02a: what codex actually does with a
// writable root, measured against the CLI rather than against our belief
// about it. `codex sandbox` runs one command under the same sandbox a
// session gets — no model turn, no network, no money.
//
// Three arms, and the first is the control the other two need:
//
//  1. the root as posse used to render it (a symlink COMPONENT) is refused,
//     and refused when the command runs — which is why the failure was
//     invisible: the session comes up, herdr calls it working then idle, and
//     every tool call inside it dies;
//  2. the root as codexWritableRoot renders it is accepted and the command
//     runs;
//  3. a write through the SYMLINKED spelling into the resolved root lands —
//     the measurement that lets {memory} and RHQ_PERSONA_DIR go on naming
//     the path the operator typed while only the sandbox root moves.
//
// Arm 1 is a fact about codex-cli, not about posse: if a later codex stops
// refusing, this skips loudly rather than failing, and the fix stays correct
// either way (a resolved root is accepted in both worlds).
func TestQACodexAcceptsOnlyAResolvedWritableRoot(t *testing.T) {
	bin, err := exec.LookPath("codex")
	if err != nil {
		t.Skip("codex not on PATH")
	}

	real := t.TempDir()
	developer := filepath.Join(real, "personas", "developer")
	if err := os.MkdirAll(developer, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "personas")
	if err := os.Symlink(filepath.Join(real, "personas"), link); err != nil {
		t.Fatal(err)
	}
	symlinked := filepath.Join(link, "developer") // ~/.config/posse/personas/<p> on the broken box

	// CODEX_HOME so the probe reads and writes a temp dir, never the
	// operator's ~/.codex (ranger-base-c02a is not worth a live-state read).
	run := func(root, script string) (string, error) {
		cmd := exec.Command(bin, "sandbox",
			"-c", `sandbox_mode="workspace-write"`,
			"-c", fmt.Sprintf(`sandbox_workspace_write.writable_roots=[%q]`, root),
			"--", "/bin/sh", "-c", script)
		cmd.Dir = real
		cmd.Env = append(os.Environ(), "CODEX_HOME="+t.TempDir())
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. the control: the unresolved root is refused, by name.
	out, err := run(symlinked, "true")
	if err == nil || !strings.Contains(out, "symlink component") {
		t.Skipf("codex no longer refuses a writable root with a symlink component — the fix is still right, but this pin's control is gone:\nerr=%v out=%s", err, out)
	}

	// 2. the fix: what realizeCodex renders now is a root codex takes.
	root := codexWritableRoot(symlinked)
	if root == symlinked {
		t.Fatalf("codexWritableRoot did not resolve %s, so arm 2 repeats arm 1", symlinked)
	}
	if out, err := run(root, "echo ran"); err != nil || !strings.Contains(out, "ran") {
		t.Fatalf("codex refused the resolved root %s — a session on it can run nothing:\nerr=%v out=%s", root, err, out)
	}

	// 3. the resolved grant still covers the symlinked spelling, so nothing
	// else on the launch line has to move.
	probe := filepath.Join(symlinked, "ORDERS.md")
	if out, err := run(root, "echo lesson >> "+shellQuote(probe)); err != nil {
		t.Fatalf("a write through %s into the granted root %s was denied — the persona cannot append to its ORDERS.md:\nerr=%v out=%s", probe, root, err, out)
	}
	if b, err := os.ReadFile(probe); err != nil || !strings.Contains(string(b), "lesson") {
		t.Fatalf("the write did not land at %s: %v %q", probe, err, string(b))
	}
}
