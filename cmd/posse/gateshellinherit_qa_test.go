package main

// QA, rangerhq-v553 — the bead's own repro, at the layer it was filed from:
// two PIDs, two `posse gates` runs through the built binary, and the second
// one started the way a hand-launch starts, from inside the first persona's
// gated session. Filed against `posse gates <persona>` rather than against
// realShell because that is where it was seen on the fleet: the mechanism
// spans the renderer AND the environment the launched wrapper inherits, and
// only the command exercises both. Verbs are `ls`/`date` rather than the
// bead's `git push`/`curl` so no arm of it can skip.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/rhq"
)

// qaPID writes a minimal PID denying one verb.
func qaPID(t *testing.T, home, name, deny string) {
	t.Helper()
	dir := filepath.Join(home, "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: " + name + "\ndescription: " + name + "\nruntime: claude\ndeny: [" + deny + "]\n---\n" + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
}

// qaGateShell renders a persona's gates through the binary and returns the
// wrapper and bin dir. shell is what $SHELL is when it runs — the launching
// session's, which is the whole point.
func qaGateShell(t *testing.T, bin, home, persona, shell string) (wrapper, binDir string) {
	t.Helper()
	cmd := exec.Command(bin, "gates", persona)
	cmd.Env = []string{
		"HOME=" + filepath.Join(home, "h"),
		"RHQ_HOME=" + home,
		"PATH=" + rhq.PathOutsideGates(""),
		"SHELL=" + shell,
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("posse gates %s: %v\n%s", persona, err, out)
	}
	shellDir := filepath.Join(home, "state", "gates", persona, "shell")
	ents, err := os.ReadDir(shellDir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("want one rendered wrapper in %s: %v %v", shellDir, ents, err)
	}
	return filepath.Join(shellDir, ents[0].Name()), filepath.Join(home, "state", "gates", persona, "bin")
}

// A session runs the wall of the PID it carries and no other (ADR 0002 §3).
// Launched from another persona's pane, it inherited that pane's PATH — the
// launching persona's shim dir at its head — and the wrapper prepended its
// own without dropping the stranger. For a verb only the LAUNCHING PID
// denies there is no shim of ours in front, so that PID's refusal is what
// the launched session got: a false refuse from a rule it does not carry.
// `posse gates <persona>` reports the launched persona's realized gates and
// says nothing about the other dir being live, so parity is blind to it.
func TestQALaunchedPersonaCarriesOnlyItsOwnGates(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	qaPID(t, home, "alpha", "Bash(ls:*)")
	qaPID(t, home, "beta", "Bash(date:*)")

	// beta launches from a login shell; alpha launches from beta's session,
	// so $SHELL is beta's wrapper (ADR 0009 §2 types it) and PATH leads
	// with beta's shim dir.
	betaShell, betaBin := qaGateShell(t, bin, home, "beta", "/bin/zsh")
	alphaShell, alphaBin := qaGateShell(t, bin, home, "alpha", betaShell)

	run := func(cmd string) string {
		c := exec.Command(alphaShell, "-c", cmd)
		c.Env = []string{
			"PATH=" + betaBin + ":" + rhq.PathOutsideGates(""),
			"RHQ_GATES_DIR=" + filepath.Join(home, "state", "gates", "alpha"),
		}
		out, _ := c.CombinedOutput()
		return string(out)
	}
	if out := run("date +%Y"); strings.Contains(out, "refused by posse gate") {
		t.Errorf("alpha's PID does not deny date; beta's does. A session must not be refused by another PID's rule:\n%s", out)
	}
	if out := run("echo $PATH"); strings.Contains(out, betaBin) {
		t.Errorf("beta's shim dir must not be live in alpha's session:\n%s", out)
	}
	if out := run("echo $PATH"); !strings.HasPrefix(strings.TrimSpace(out), alphaBin+string(os.PathListSeparator)) {
		t.Errorf("alpha's own shim dir must lead its PATH:\n%s", out)
	}
	if out := run("ls /"); !strings.Contains(out, "refused by posse gate: ls /") {
		t.Errorf("alpha's own deny must still bite:\n%s", out)
	}
	// The renderer's half of the same leak (ranger-base-f0ay): a wrapper is
	// never another one's REAL, whatever $SHELL says.
	b, err := os.ReadFile(alphaShell)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "REAL='"+betaShell+"'") {
		t.Errorf("alpha's REAL must not be beta's wrapper:\n%s", b)
	}
}
