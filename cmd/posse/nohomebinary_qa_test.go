package main

// ranger-base-58b5, the operator's condition on the ranger-base-5flf8
// ruling: the `env -i` arm must go RED on the refusal, and must never read a
// cwd-relative .config/posse. The unit pin (internal/posse,
// TestNewAppRefusesAHomeItWouldHaveToInvent) answers what newApp returns;
// this one answers what the shipped binary DOES, which is the thing the
// measurement in the bead was taken on and the only arm that can see the
// exit status, the stderr an operator reads, and the files left behind.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryWithNoHomeRefusesRatherThanRootAtTheCWD(t *testing.T) {
	bin := buildRhq(t)

	// The trap, planted: a cwd carrying exactly the path the empty Join
	// lands in, holding a config no operator installed. Pre-fix the binary
	// read this file and ran the command against it.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".config", "posse"), 0o755); err != nil {
		t.Fatal(err)
	}
	planted := filepath.Join(dir, ".config", "posse", "config.yaml")
	if err := os.WriteFile(planted, []byte("crew: [nobody]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := treeCensusAt(t, dir)

	// `env -i` with PATH alone: no HOME, no RHQ_HOME, no POSSE_HOME. Not
	// HOME="" — an absent name and an empty one are the same to os.Getenv,
	// and absent is the shape a scrubbed environment actually has.
	cmd := exec.Command(bin, "help")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("posse help with no home in the environment exited 0 — it resolved a home from the cwd. Output:\n%s", out)
	}
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
		t.Errorf("want exit 1, got %v", err)
	}
	for _, want := range []string{"$HOME", "RHQ_HOME", "POSSE_HOME"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the refusal must name %s so an operator knows the way through; got:\n%s", want, out)
		}
	}

	// And it read nothing and wrote nothing under the cwd. `posse help`
	// only prints, so the census is about what a home resolved there would
	// have let the NEXT command do; the file count is the part that would
	// catch a seed.
	for path, sum := range treeCensusAt(t, dir) {
		if was, ok := before[path]; !ok {
			t.Errorf("the refused run created %s under the cwd", path)
		} else if was != sum {
			t.Errorf("the refused run rewrote %s under the cwd", path)
		}
	}
	for path := range before {
		if _, ok := treeCensusAt(t, dir)[path]; !ok {
			t.Errorf("the refused run removed %s", path)
		}
	}

	// The control, and it is what makes the arm above about the HOME and
	// not about `env -i`: the same binary, the same scrubbed environment
	// plus one name, in the same cwd carrying the same trap.
	real := t.TempDir()
	ctl := exec.Command(bin, "help")
	ctl.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + real}
	ctl.Dir = dir
	if out, err := ctl.CombinedOutput(); err != nil {
		t.Fatalf("control arm (HOME set) must still work: %v\n%s", err, out)
	}
}

// treeCensusAt is every file under root, by relative path and content.
func treeCensusAt(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
