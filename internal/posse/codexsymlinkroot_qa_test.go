//go:build posse_arm2

package posse

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
//
// All three arms need a box that can APPLY a sandbox, which a posse persona
// whose PID says `cage: seatbelt` is not — macOS refuses to nest sandbox_apply
// — so the preflight below skips the whole test there (ranger-base-ejva).
func TestQACodexAcceptsOnlyAResolvedWritableRoot(t *testing.T) {
	t.Parallel()
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

	// 0. the preflight: can this box apply a sandbox at all? The root is
	// plainly legal — `real` resolved, by EvalSymlinks rather than by the
	// production helper, so a broken codexWritableRoot cannot make the
	// preflight lie — and it is not the root the arms below judge. A
	// failure here is therefore never "codex refused this root"; under a
	// nested cage it is sandbox-exec's own, and arms 2 and 3 would Fatal on
	// it and read as posse being broken (ranger-base-ejva).
	legal, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := run(legal, "echo applied"); codexSandboxUnappliable(out, err) {
		t.Skipf("this box cannot apply a sandbox at all, so codex's verdict on any root is unmeasurable here (a caged persona: macOS refuses to nest sandbox_apply):\nerr=%v out=%s", err, out)
	} else if err != nil || !strings.Contains(out, "applied") {
		t.Fatalf("codex could not run under a plainly-legal writable root %s, so nothing below measures what it claims:\nerr=%v out=%s", legal, err, out)
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

// codexSandboxUnappliable separates "this box could not apply a sandbox" from
// "codex refused the root it was handed" — the distinction ranger-base-ejva is
// about. sandbox-exec prints its own line and exits 71 when the kernel refuses
// (a nested cage); codex's refusal of a root is exit 1 and codex prose, emitted
// BEFORE any sandbox is applied, which is why arm 1 keeps measuring even inside
// a cage. A DENIED WRITE inside a working sandbox also says "Operation not
// permitted" — that is arm 3's bug and must never skip — so the discriminating
// half is the `sandbox_apply:` prefix, not the errno text.
func codexSandboxUnappliable(out string, err error) bool {
	return err != nil && strings.Contains(out, "sandbox_apply: Operation not permitted")
}

// The wrong arms of that skip. Every fixture below is a transcript measured on
// 2026-08-29 (codex-cli 0.150.1, macOS 25.4, inside a caged persona session), not
// a guess: a skip that fires on codex's genuine refusal would swallow the bug
// the live test exists for.
func TestCodexSandboxUnappliableSkipsOnlyANestedCage(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		out  string
		code int
		want bool
	}{
		{
			// `sandbox-exec -f p.sb /bin/echo nested` inside a seatbelt.
			name: "nested cage",
			out:  "sandbox-exec: sandbox_apply: Operation not permitted\n",
			code: 71,
			want: true,
		},
		{
			// arm 1's control: codex refusing a root, before any sandbox.
			name: "codex refuses the root",
			out:  "Error: writable root /a/b/personas/developer contains symlink component /a/b/personas; symlinked writable roots are not supported\n",
			code: 1,
			want: false,
		},
		{
			// arm 3's bug: the sandbox applied fine and denied the write.
			// Says "Operation not permitted" too — `/bin/sh -c 'echo x >>
			// /System/nope'`, measured — so a substring match on the errno
			// alone would skip over exactly what the test is for.
			name: "the sandbox denied the write",
			out:  "/bin/sh: /System/nope: Operation not permitted\n",
			code: 1,
			want: false,
		},
		{
			// A run that SUCCEEDED is never a skip, whatever it printed.
			name: "success",
			out:  "sandbox-exec: sandbox_apply: Operation not permitted\n",
			code: 0,
			want: false,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := exec.Command("/bin/sh", "-c", fmt.Sprintf("exit %d", c.code)).Run()
			if (err != nil) != (c.code != 0) {
				t.Fatalf("fixture: exit %d produced err=%v", c.code, err)
			}
			if got := codexSandboxUnappliable(c.out, err); got != c.want {
				t.Errorf("codexSandboxUnappliable(%q, %v) = %v, want %v", c.out, err, got, c.want)
			}
		})
	}
}
