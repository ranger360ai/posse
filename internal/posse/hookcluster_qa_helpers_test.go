package posse

// Helpers lifted out of hookcluster_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// visWall is a public- and a private-stamped scratch repo with the REAL
// rendered hook installed in each, in the shape TestIdentityLiteralGuardHook
// uses. $HOME is this process's for the duration because
// DeriveIdentityLiterals reads it directly (AbbrevHome): without that the
// abbrev and absolute instance paths are the same string, dedupe drops one,
// and the tilde-form pin below has nothing to measure.
type visWall struct {
	pub, priv, gates, home string
	instance               string
	git                    func(repo string, env []string, args ...string) (string, error)
	// gitIn is git with something on STDIN: the crew's own commit form is
	// `git commit -F - -- <paths>` (AGENTS.md), so a pin over what a wall
	// does to a MESSAGE has to be able to type one the way the crew does.
	gitIn    func(repo string, env []string, stdin string, args ...string) (string, error)
	persona  []string
	identity []IdentityLiteral
}

func newVisWall(t *testing.T) *visWall { return newVisWallNamed(t, "instance") }

// newVisWallNamed is newVisWall with the instance directory's NAME under the
// caller's control, which is the only handle a test has on what the derived
// literals actually contain — the username and the git email are the box's.
func newVisWallNamed(t *testing.T, instanceDir string) *visWall {
	return newVisWallCfg(t, instanceDir, "")
}

// newVisWallCfg appends extraConfig to the scratch config.yaml before the
// hooks are stamped from it — the handle a test needs on the OTHER half of
// what check 3 scans since ADR 0048 D2, this instance's own
// beads_visibility_patterns:. It is appended, not merged: flat-YAML is
// line-oriented and the caller writes whole top-level keys.
func newVisWallCfg(t *testing.T, instanceDir, extraConfig string) *visWall {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	w := &visWall{
		home:     home,
		gates:    t.TempDir(),
		pub:      filepath.Join(home, "pub"),
		priv:     filepath.Join(home, "priv"),
		instance: filepath.Join(home, instanceDir),
	}
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+w.pub+": public\n  "+w.priv+": private\n"+extraConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: cfg}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	w.git = func(repo string, env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	w.gitIn = func(repo string, env []string, stdin string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	w.persona = []string{"RHQ_PERSONA=tester", "RHQ_GATES_DIR=" + w.gates}
	for _, repo := range []string{w.pub, w.priv} {
		write(t, filepath.Join(repo, ".beads", "redirect"), filepath.Join(w.instance, ".beads")+"\n")
		if out, err := w.git(repo, nil, "init", "-q", "-b", "main"); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
			t.Fatal(err)
		}
	}
	w.identity = testIdentity(t, w.pub)
	return w
}

// literal digs one derived class's value out, and fails the FIXTURE if the
// box did not produce it — a pin that silently measured an empty literal
// would be green over any wall at all.
func (w *visWall) literal(t *testing.T, class string) string {
	t.Helper()
	for _, l := range w.identity {
		if l.Class == class {
			return l.Value
		}
	}
	t.Fatalf("fixture premise: this box must derive a %q literal, got %+v", class, w.identity)
	return ""
}

// stage writes body at rel and stages exactly that path.
func (w *visWall) stage(t *testing.T, repo, rel, body string) {
	t.Helper()
	write(t, filepath.Join(repo, filepath.FromSlash(rel)), body)
	if out, err := w.git(repo, nil, "add", "--", rel); err != nil {
		t.Fatalf("git add %s: %v %s", rel, err, out)
	}
}

// plant commits a path with the hook bypassed outright. It is only ever used
// to build HISTORY a pin then measures against — a file that is already in
// the tree, which by construction cannot be staged through the wall that is
// under test. Using the override for this instead would put OVERRIDDEN lines
// in refusals.log that the log assertions would then have to explain away.
func (w *visWall) plant(t *testing.T, repo, rel, body string) {
	t.Helper()
	w.stage(t, repo, rel, body)
	if out, err := w.git(repo, nil, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "planted", "--", rel); err != nil {
		t.Fatalf("planting %s: %v %s", rel, err, out)
	}
}

func (w *visWall) log(t *testing.T) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(w.gates, "refusals.log"))
	return string(b)
}
