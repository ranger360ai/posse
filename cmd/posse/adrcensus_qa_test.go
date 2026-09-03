package main

// ranger-base-gyrko: `posse gates adr-census` is wired through the built
// binary — the census pins in internal/posse measure the predicate; this one
// measures that the verb reaches it, from a subdirectory, with the summary
// on stdout and the script's own verdict as the exit code.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestGatesAdrCensusDispatches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	bin := buildRhq(t)
	repo := t.TempDir()
	env := []string{"PATH=" + posse.PathOutsideGates(""), "HOME=" + repo, "RHQ_HOME=" + filepath.Join(repo, "rhq")}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(env, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")

	// No records: an error, not a clean census.
	cmd := exec.Command(bin, "gates", "adr-census")
	cmd.Dir, cmd.Env = repo, env
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "nothing to judge") {
		t.Errorf("no docs/adr must be an error naming what is missing, got err=%v:\n%s", err, out)
	}

	// One record with an unresolvable token, from a subdirectory: exit 0
	// and a summary that says it judged nothing.
	if err := os.MkdirAll(filepath.Join(repo, "docs", "adr", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docs", "adr", "0001-x.md"), []byte("# x\n\n`deadbee` is prose.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "--", "docs/adr/0001-x.md")
	git("commit", "-qm", "seed", "--", "docs/adr/0001-x.md")
	cmd = exec.Command(bin, "gates", "adr-census")
	cmd.Dir, cmd.Env = filepath.Join(repo, "docs", "adr", "sub"), env
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("adr-census from a subdirectory: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "posse gates adr-census: base main judged 0 distinct tokens: 0 ancestors, 0 admitted by twin, 0 refused") {
		t.Errorf("the summary must reach stdout and say 0 judged:\n%s", out)
	}

	// A refusal is exit 1, from the script's own verdict. The stale sha is a
	// commit on a side branch that main does not have.
	git("checkout", "-q", "-b", "side")
	if err := os.WriteFile(filepath.Join(repo, "side.txt"), []byte("side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "--", "side.txt")
	git("commit", "-qm", "side", "--", "side.txt")
	rev := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD")
	rev.Env = env
	stale, err := rev.Output()
	if err != nil {
		t.Fatal(err)
	}
	git("checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(repo, "docs", "adr", "0001-x.md"), []byte("# x\n\nlanded "+strings.TrimSpace(string(stale))+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(bin, "gates", "adr-census", "docs/adr/0001-x.md")
	cmd.Dir, cmd.Env = repo, env
	out, err = cmd.CombinedOutput()
	var ee *exec.ExitError
	if err == nil || !strings.Contains(string(out), "REFUSE docs/adr/0001-x.md:3 "+strings.TrimSpace(string(stale))) {
		t.Fatalf("a stale sha must be a REFUSE line and exit 1, got err=%v:\n%s", err, out)
	}
	if ok := errors.As(err, &ee); !ok || ee.ExitCode() != 1 {
		t.Errorf("the refusal must exit 1 (the census's own verdict), got %v", err)
	}
}
