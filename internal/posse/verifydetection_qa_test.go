//go:build posse_arm2

package posse

// ranger-base-53w1: scripts/verify-detection.sh replayed its fixtures through
// `herdr agent explain`, which resolves the manifest from the INSTALLED copy
// in ~/.config/herdr/agent-detection. Measured on the bug: deleting the
// `update_menu` rule from etc/herdr/agent-detection/codex.toml and running the
// script reported 9/9 fixtures OK, because the answer came from whatever the
// operator last installed. `make install-detection` copies and then verifies,
// so the thing under test had just been written over the thing it was tested
// against.
//
// The pin is the bead's own scenario, and its wrong arm is what makes it a
// pin: a complete manifest is planted in a scratch XDG_CONFIG_HOME — an
// operator who HAS installed the good copy — while the rule is cut from the
// tree the script reads. The script must fail. Revert the staging in
// verify-detection.sh and this arm goes green at 9/9, which is the bug.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// detectionRig copies the script and the manifests+fixtures it reads into a
// throwaway repo root, so an arm can cut a rule without touching the checkout.
// Returns the rig root; the script lives at <root>/scripts/verify-detection.sh
// and finds its manifests via its own `cd "$(dirname "$0")/.."`.
func detectionRig(t *testing.T) string {
	t.Helper()
	repo := filepath.Join("..", "..")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	cp := func(from, to string, mode os.FileMode) {
		b, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(to, b, mode); err != nil {
			t.Fatal(err)
		}
	}
	cp(filepath.Join(repo, "scripts", "verify-detection.sh"),
		filepath.Join(root, "scripts", "verify-detection.sh"), 0o755)

	src := filepath.Join(repo, "etc", "herdr", "agent-detection")
	dst := filepath.Join(root, "etc", "herdr", "agent-detection")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	tomls := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		tomls++
		cp(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), 0o644)
	}
	if tomls == 0 {
		t.Fatalf("no manifests under %s — the rig would measure nothing", src)
	}
	agents, err := os.ReadDir(filepath.Join(src, "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	fixtures := 0
	for _, a := range agents {
		if !a.IsDir() {
			continue
		}
		fs, err := os.ReadDir(filepath.Join(src, "testdata", a.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range fs {
			if !strings.HasSuffix(f.Name(), ".txt") {
				continue
			}
			fixtures++
			cp(filepath.Join(src, "testdata", a.Name(), f.Name()),
				filepath.Join(dst, "testdata", a.Name(), f.Name()), 0o644)
		}
	}
	if fixtures == 0 {
		t.Fatal("no fixtures copied — an empty rig passes every arm")
	}
	t.Logf("rig: %d manifests, %d fixtures", tomls, fixtures)
	return root
}

// runDetection runs the rig's script with install dir `installed` presented as
// the operator's XDG_CONFIG_HOME.
func runDetection(t *testing.T, root, installed string, args ...string) (out string, code int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(root, "scripts", "verify-detection.sh"), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"HOME="+filepath.Join(root, "home"),
		"XDG_CONFIG_HOME="+installed,
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("verify-detection.sh: %v\n%s", err, b)
		}
		code = exit.ExitCode()
	}
	return string(b), code
}

func TestQAVerifyDetectionFailsACommittedRuleDeletion(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root := detectionRig(t)
	manifest := filepath.Join(root, "etc", "herdr", "agent-detection", "codex.toml")
	good, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}

	// The installed copy an operator would have: complete, rule present. Every
	// arm below sees it, so a run that reads the install cannot fail.
	installed := filepath.Join(root, "installed")
	if err := os.MkdirAll(filepath.Join(installed, "herdr", "agent-detection"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "herdr", "agent-detection", "codex.toml"), good, 0o644); err != nil {
		t.Fatal(err)
	}

	// Control: an untouched tree passes. Without this the failing arm below
	// could be failing for any reason at all.
	out, code := runDetection(t, root, installed)
	if code != 0 || !strings.Contains(out, "fixtures OK") {
		t.Fatalf("intact tree: exit %d, want 0 with all fixtures OK\n%s", code, out)
	}
	if strings.Contains(out, "0/0 fixtures OK") {
		t.Fatalf("intact tree passed with no fixtures — the rig measured nothing\n%s", out)
	}

	// The bead: cut a rule from the TREE, leave the install whole. The script
	// explained against the install, so this reported every fixture OK.
	if err := os.WriteFile(manifest, []byte(cutRule(t, string(good), "update_menu")), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, installed)
	if code == 0 {
		t.Errorf("update_menu deleted from the checkout and verify-detection still passed (exit 0) — "+
			"it is reading the installed manifest, which is ranger-base-53w1\n%s", out)
	}
	if !strings.Contains(out, "blocked-update-menu") || !strings.Contains(out, "FAIL") {
		t.Errorf("the deletion did not fail the fixture written for it\n%s", out)
	}

	// And it must fail for the deletion, not because the rig's install drifted:
	// the install-side note is a note, and the tree is what sets the exit code.
	if err := os.WriteFile(manifest, good, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "herdr", "agent-detection", "codex.toml"),
		[]byte(cutRule(t, string(good), "update_menu")), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, installed)
	if code != 0 {
		t.Errorf("whole tree with a stale INSTALL failed (exit %d) — the install is reported, "+
			"not verified; `make install-detection` is what tests the install\n%s", code, out)
	}
	if !strings.Contains(out, "differs from the checkout") {
		t.Errorf("a stale install was not reported\n%s", out)
	}
}

// The install arm ranger-base-neyn asked for, which explaining against the
// tree would otherwise drop: right after `make install-detection` the two
// copies must be byte-identical, and the target runs the script with
// --check-install so they are compared with an exit code and not a note.
// Without a mode like this, an install that lands somewhere herdr does not
// read is invisible — the fixtures pass, because they no longer touch it.
func TestQAVerifyDetectionCheckInstallFailsAStaleInstall(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root := detectionRig(t)
	manifest := filepath.Join(root, "etc", "herdr", "agent-detection", "codex.toml")
	good, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	det := filepath.Join(root, "installed", "herdr", "agent-detection")
	if err := os.MkdirAll(det, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"codex.toml", "grok.toml"} {
		b, err := os.ReadFile(filepath.Join(root, "etc", "herdr", "agent-detection", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(det, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Control: a fresh install matches, --check-install passes.
	out, code := runDetection(t, root, filepath.Join(root, "installed"), "--check-install")
	if code != 0 {
		t.Fatalf("--check-install on a matching install: exit %d, want 0\n%s", code, out)
	}

	// Drift the INSTALL only — the tree is untouched, so every fixture still
	// passes and only the install comparison can produce the failure.
	if err := os.WriteFile(filepath.Join(det, "codex.toml"),
		[]byte(cutRule(t, string(good), "update_menu")), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, filepath.Join(root, "installed"), "--check-install")
	if code == 0 {
		t.Errorf("--check-install passed with the installed codex.toml out of step with the checkout\n%s", out)
	}
	if !strings.Contains(out, "fixtures OK") {
		t.Errorf("--check-install failed on something other than the install; the tree was untouched\n%s", out)
	}

	// And the same drift is only a note without the flag: `make verify-detection`
	// on a checkout nobody installed is not a red suite.
	if out, code = runDetection(t, root, filepath.Join(root, "installed")); code != 0 {
		t.Errorf("plain run failed on install drift (exit %d) — the exit code is the tree's\n%s", code, out)
	}
}
