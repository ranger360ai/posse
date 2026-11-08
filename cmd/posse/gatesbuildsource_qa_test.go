package main

// QA pin for ranger-base-eiwhk — `posse gates <persona>` on a box whose only
// bead repo is a project repo, which is the command and the box the bead was
// filed from.
//
// WHY THROUGH THE BUILT BINARY, twice over, and both reasons are
// gatesrelease_qa_test.go's. The usual one first: the call lives in the
// `gates` case in main.go and a unit test of the reading cannot tell whether
// anything calls it (wallrendererwired_qa_test.go's own note) — and this
// surface's reading composed its OWN copy of the candidate list until this
// bead, so the fix landing on `posse status` and not here is exactly the
// half-landing this arm exists to catch.
//
// The reason particular to this bead is the STAMP. The answer is reachable
// only through `-X <internal/posse.BuildSourceSymbol>=<dir>`, and `-X` on a
// symbol that does not exist is SILENT — no warning, exit 0, the variable
// keeps its default (cagestale_test.go measured that for the Build stamp). So
// renaming the Go variable and leaving the Makefile and
// scripts/clean-build.sh alone would ship a binary that reads UNKNOWN again
// with every unit arm in the tree green. This arm stamps through the CONSTANT
// those two lines are pinned against (internal/treepins/buildsource_qa_test.go)
// and reads the line back off the real command.
//
// THE RIG IS THE BEAD'S BOX. roGatesRun runs with cwd in a scratch dir and an
// RHQ_HOME that names no `beads:`, so no candidate directory is a posse
// checkout at all — which is the instance the bead was measured on: the
// checkout the install came from is on disk and in no list the reading reads.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// buildSourceRig is a scratch posse checkout on `main` holding one commit,
// plus the short sha a binary built there would carry. The same shape
// lagRig(0) makes, kept separate because this arm wants the directory to be
// the ONLY way to the answer and so must not share a fixture that plants it
// in `beads:`.
func buildSourceRig(t *testing.T) (repo, stamp string) {
	t.Helper()
	repo = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	// The module declaration is half of what makes this checkout "this
	// binary's" — the reading counts nowhere else (launcherlag.go
	// isPosseCheckout) — and the repo's own path in the seed makes two rigs
	// built in the same second two HISTORIES rather than one sha.
	if err := os.WriteFile(filepath.Join(repo, "go.mod"),
		[]byte("module github.com/ranger360ai/posse\n\ngo 1.26.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte(repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "--", "go.mod", "seed.txt")
	// Path-limited: the gates dir is on this suite's PATH and the git shim
	// there refuses an unqualified commit whatever tree it is in
	// (`deny: Bash(git commit unless --)`).
	git("commit", "-q", "-m", "seed", "--", "go.mod", "seed.txt")
	return repo, git("rev-parse", "--short=7", "HEAD")
}

// buildWithSource builds the command stamped the way `make install` stamps
// it: the sha it was cut from, and the directory it was built in — the second
// through posse.BuildSourceSymbol, so the symbol itself is under test. An
// empty source stamps nothing, which is the control and is also what every
// other build in the world carries.
func buildWithSource(t *testing.T, stamp, source string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	ldflags := "-X github.com/ranger360ai/posse/internal/posse.Build=" + stamp
	if source != "" {
		ldflags += " -X " + posse.BuildSourceSymbol + "=" + source
	}
	out, err := exec.Command("go", "build", "-ldflags", ldflags,
		"-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// The bead, end to end, with its own control. Same rig, same command, same
// stamp — the only difference is whether the binary knows where it was built,
// and that difference is the whole fix.
func TestQAGatesOnABoxWhoseQueueIsNotPosseDoesNotReadUnknownForever(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo, stamp := buildSourceRig(t)

	// THE CONTROL FIRST, because it is what makes the arm below mean
	// anything: unstamped, this is the line the bead quoted.
	out, code := roGatesRun(t, buildWithSource(t, stamp, ""), roGatesHome(t))
	if code != 0 {
		t.Fatalf("posse gates builder exited %d:\n%s", code, out)
	}
	if line := gatesFirstLine(t, out); !strings.Contains(line, "UNKNOWN") {
		t.Fatalf("the control is not the bead's reading, so this fixture proves nothing:\n%s", line)
	}

	out, code = roGatesRun(t, buildWithSource(t, stamp, repo), roGatesHome(t))
	if code != 0 {
		t.Fatalf("posse gates builder exited %d:\n%s", code, out)
	}
	line := gatesFirstLine(t, out)
	if strings.Contains(line, "UNKNOWN") {
		t.Fatalf("the installed binary still reads UNKNOWN on the line the bead quoted — either the stamp did not land (a `-X` on a symbol that does not exist is silent, so check posse.BuildSourceSymbol) or `posse gates` is reading a candidate list of its own:\n%s", line)
	}
	// It found the rig, and says so: the whole value of this reading is that
	// it names the checkout it counted in, so an operator can re-run it.
	if !strings.Contains(line, repo) {
		t.Fatalf("the line does not name the checkout the binary was built in (%s):\n%s", repo, line)
	}
	// The rig IS the tip, so this is the current-and-say-so arm. A reading
	// that answered "behind" here would mean it resolved somewhere else.
	if !strings.Contains(line, "is the tip of main") {
		t.Fatalf("want the tip reading against the rig:\n%s", line)
	}
	// And it still moves no exit code, whatever it found: this is a reading,
	// and installing over a binary that is dispatching a live fleet is the
	// operator's (guardrail 3).
	if code != 0 {
		t.Errorf("the reading moved the exit code to %d", code)
	}
}

// A STAMP THAT NO LONGER NAMES A POSSE CHECKOUT reads UNKNOWN again, and says
// which of its candidates it rejected and why. This is the arm that keeps the
// stamp a candidate rather than a claim: the directory is stamped, it exists,
// and it is not posse — so the reading abstains instead of counting there.
func TestQAGatesRefusesABuildSourceThatIsNotAPosseCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	_, stamp := buildSourceRig(t)
	notPosse := t.TempDir()

	out, code := roGatesRun(t, buildWithSource(t, stamp, notPosse), roGatesHome(t))
	if code != 0 {
		t.Fatalf("posse gates builder exited %d:\n%s", code, out)
	}
	line := gatesFirstLine(t, out)
	if !strings.Contains(line, "UNKNOWN") {
		t.Fatalf("a stamped directory that is not a posse checkout was read as one:\n%s", line)
	}
	// Named, not silently dropped. An operator whose checkout has moved
	// needs to see that the binary looked there and what was wrong with it,
	// or the stamp is a mechanism with no observable failure.
	if !strings.Contains(line, notPosse) || !strings.Contains(line, "not a git checkout") {
		t.Fatalf("the abstention does not name the stamped directory it rejected (%s):\n%s", notPosse, line)
	}
}
