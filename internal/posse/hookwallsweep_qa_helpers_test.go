package posse

// Helpers lifted out of hookwallsweep_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hwsRepo makes a git repo with one commit and returns its path.
func hwsRepo(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "pin@example.invalid"},
		{"config", "user.name", "pin"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// hwsFixture: a home whose config declares `repos`, each a real git repo with
// posse's two hooks installed from THIS build's renderer. Returns the app and
// the repo paths in the order named.
//
// The reference is the renderer under test, never a checked-in string: a pin
// that compares the sweep against a frozen body would go green the day the
// body changed and the sweep stopped agreeing with it.
func hwsFixture(t *testing.T, vis map[string]string, order ...string) (*App, map[string]string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHQ_HOME", home)

	dirs := map[string]string{}
	var cfg strings.Builder
	cfg.WriteString("beads_visibility:\n")
	for _, name := range order {
		d := hwsRepo(t, root, name)
		dirs[name] = d
		cfg.WriteString("  " + d + ": " + vis[name] + "\n")
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewAppAt(home)
	for _, name := range order {
		if _, _, _, err := a.InstallCommitGuardHook(dirs[name]); err != nil {
			t.Fatalf("install commit guard in %s: %v", name, err)
		}
		if _, err := InstallPrePushHook(dirs[name]); err != nil {
			t.Fatalf("install pre-push in %s: %v", name, err)
		}
	}
	return a, dirs
}

func hwsHook(t *testing.T, repo, slot string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		t.Fatalf("rev-parse hooks in %s: %v", repo, err)
	}
	h := strings.TrimSpace(string(out))
	if !filepath.IsAbs(h) {
		h = filepath.Join(repo, h)
	}
	return filepath.Join(h, slot)
}

func hwsReport(t *testing.T, a *App, where string) (string, bool) {
	t.Helper()
	var b bytes.Buffer
	found := a.ReportHookWall(&b, where)
	return b.String(), found
}
