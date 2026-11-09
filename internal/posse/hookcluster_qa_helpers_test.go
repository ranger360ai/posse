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
// DeriveIdentityLiterals reads it directly (canonicalAbbrevHome, ADR 0082
// D2): without that the abbrev and absolute instance paths are the same
// string, dedupe drops one, and the tilde-form pin below has nothing to
// measure.
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

// qaFixtureUsername is the account name every visWall pin derives under.
// It is PINNED rather than read off the box, and the reason is measured
// (ranger-base-c7i8t, from ranger-base-sydnc): the username was the one
// check-3 source whose value belonged to the host, so a whole cluster of
// pins staged a literal no fixture controlled. Under an account named for
// the product — ADR 0080 D2's dedicated `posse` host — 29 of the 142
// commit-wall pins reddened because the literal matched `package posse` and
// `internal/posse/...` everywhere. The shipped-name exclusion reds the same
// cluster the OTHER way, on `w.literal(t, "username")` finding no username
// class at all to stage: MEASURED 2026-11-09, 16 of them, with this pin
// removed and the exclusion in place (docs/notes.d/ranger-base-c7i8t.md has
// the five configurations). Pinned, neither happens and the pins say the
// same thing on every box.
//
// It is not one of posse's shipped names and must never become one: those
// are exactly what DeriveIdentityLiterals skips (isShippedPosseName), so a
// fixture named from that set would derive nothing. It is the same spelling
// the hand-built username literals in this suite already use.
const qaFixtureUsername = "qa-fixture-operator"

// qaPinUsername points the username seam (currentUsername) at name for the
// duration of t. Never from a parallel test: the seam is a package
// variable, which is the same constraint t.Setenv already puts on every
// visWall caller.
func qaPinUsername(t *testing.T, name string) {
	t.Helper()
	old := currentUsername
	currentUsername = func() (string, error) { return name, nil }
	t.Cleanup(func() { currentUsername = old })
}

func newVisWall(t *testing.T) *visWall { return newVisWallNamed(t, "instance") }

// newVisWallNamed is newVisWall with the instance directory's NAME under the
// caller's control. It and the pinned username (qaFixtureUsername) are the
// handles a test has on what the derived literals actually contain; the git
// e-mail is still the box's.
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
	return newVisWallAs(t, qaFixtureUsername, instanceDir, extraConfig)
}

// newVisWallAs is newVisWallCfg deriving under a CHOSEN account name — the
// handle the shipped-name exclusion needs (ranger-base-c7i8t): the hook is
// rendered during the install below, so the username has to be pinned
// before it and not after. Every other constructor in this file reaches it
// with qaFixtureUsername.
func newVisWallAs(t *testing.T, username, instanceDir, extraConfig string) *visWall {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	qaPinUsername(t, username)
	home := gitTempDir(t)
	t.Setenv("HOME", home)
	w := &visWall{
		home:     home,
		gates:    gitTempDir(t),
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
	// The instance store EXISTS on disk, which ADR 0082 D1 made load-bearing
	// for a fixture: canonicalPath renders the kernel's name for a directory
	// it can OPEN, and falls back to resolvedPath's string for one it cannot
	// — so an instance directory that was never created renders the
	// unresolved t.TempDir() spelling while $HOME renders the resolved one,
	// the prefix test fails, and the tilde literal dedupes away. Three pins
	// in this cluster read that literal by name.
	if err := os.MkdirAll(filepath.Join(w.instance, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
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
