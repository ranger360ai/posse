package main

// QA pin for ranger-base-ctath — `posse gates <persona> | head -1` on a
// RELEASE install, which is the command and the pipe the bead was filed from.
//
// WHY THROUGH THE BUILT BINARY, twice over. The usual reason first: the call
// lives in the `gates` case in main.go and a unit test of the reading cannot
// tell whether anything calls it (wallrendererwired_qa_test.go's own note).
// The reason particular to this bead is the STAMP. The release state is
// reachable only through `-X <internal/posse.ReleaseTagSymbol>=<tag>`, and
// `-X` on a symbol that does not exist is SILENT — no warning, exit 0, the
// variable keeps its default (cagestale_test.go measured that for the Build
// stamp). So renaming the Go variable and leaving
// scripts/release-artifacts.sh alone would ship releases that read UNKNOWN
// again with every unit arm in the tree green. This arm stamps through the
// CONSTANT the script is pinned against and reads the state back off the real
// command: if the constant does not name a real symbol, the stamp lands
// nowhere and the line reads UNKNOWN.
//
// THE RIG IS THE BEAD'S BOX. roGatesRun runs with cwd in a scratch dir and an
// RHQ_HOME that names no `beads:`, so no candidate directory is a posse
// checkout at all — which is exactly a brew install: nothing here holds the
// commit the binary names, and nothing ever will.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// gatesReleaseSha is a stamp no checkout can hold: 40 hex that is not a
// commit in this repository or any other. A short plausible sha would be a
// one-in-billions accident; this one cannot resolve anywhere.
const gatesReleaseSha = "deadbee5deadbee5deadbee5deadbee5deadbee5"

// buildReleaseRhq builds the command stamped the way
// scripts/release-artifacts.sh stamps a release: the sha it was cut from, and
// the tag — the second through posse.ReleaseTagSymbol, so the symbol itself
// is under test.
func buildReleaseRhq(t *testing.T, tag string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "posse")
	out, err := exec.Command("go", "build",
		"-ldflags", "-X github.com/ranger360ai/posse/internal/posse.Build="+gatesReleaseSha+
			" -X "+posse.ReleaseTagSymbol+"="+tag,
		"-o", bin, "github.com/ranger360ai/posse/cmd/posse").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// gatesFirstLine is the wall-renderer line as `| head -1` delivers it.
func gatesFirstLine(t *testing.T, out string) string {
	t.Helper()
	at := strings.Index(out, wrwPrefix)
	if at < 0 {
		t.Fatalf("`posse gates <persona>` printed no wall-renderer line:\n%s", out)
	}
	line := out[at:]
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return line
}

// The bead, end to end: a release install reads its own identity, and the
// word the operator met is gone from the line that `head -1` returns.
func TestQAGatesOnAReleaseInstallDoesNotReadUnknownForever(t *testing.T) {
	tag := "v" + posse.Version
	bin := buildReleaseRhq(t, tag)
	out, code := roGatesRun(t, bin, roGatesHome(t))
	if code != 0 {
		t.Fatalf("posse gates builder exited %d:\n%s", code, out)
	}
	line := gatesFirstLine(t, out)
	if strings.Contains(line, "UNKNOWN") {
		t.Fatalf("a release install still reads UNKNOWN on the line the bead quoted — either the stamp did not land (a `-X` on a symbol that does not exist is silent, so check posse.ReleaseTagSymbol) or the state is not wired:\n%s", line)
	}
	if !strings.Contains(line, "is release "+tag) {
		t.Fatalf("the line does not name the release it is:\n%s", line)
	}
	// It still moves no exit code and still says nothing about being the
	// newest release — the two properties that keep this a reading rather
	// than a control, and keep it off the network on a launch path.
	if !strings.Contains(out, "brew upgrade ranger360ai/tap/posse") {
		t.Errorf("the release reading carries no remedy a release user can run:\n%s", out)
	}
}

// The control arm, and it is the one that makes the arm above mean something:
// the SAME rig, the same command, a stamp that is not this source's release
// — and the permanent-UNKNOWN wording is back, with the disagreement named
// under it. A fix that read "release" for any stamped tag would pass the arm
// above and fail here.
func TestQAGatesRefusesAReleaseStampThatIsNotThisSources(t *testing.T) {
	bin := buildReleaseRhq(t, "v9.9.9")
	out, code := roGatesRun(t, bin, roGatesHome(t))
	if code != 0 {
		t.Fatalf("posse gates builder exited %d:\n%s", code, out)
	}
	line := gatesFirstLine(t, out)
	if !strings.Contains(line, "UNKNOWN") {
		t.Fatalf("a tag that disagrees with the compiled-in version was read as a release:\n%s", line)
	}
	if !strings.Contains(out, "stamped as release v9.9.9") || !strings.Contains(out, "one of the two is wrong") {
		t.Errorf("the refusal was silent, which reads exactly like an ordinary dev build:\n%s", out)
	}
}
