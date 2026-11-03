package treepins

// QA pins for ranger-base-ctath — scripts/release-artifacts.sh is the ONLY
// thing in this tree that stamps a release identity, and it has to keep
// stamping it.
//
// THE SILENT DIRECTION. posse's wall-freshness reading is defined against a
// local checkout, so on a release install it read "UNKNOWN whether this
// binary carries the wall renders and the launch-time checks that landed"
// permanently — a brew user has no checkout and never will. The fix is one
// extra `-X` on the release build line: internal/posse.ReleaseTag, which
// internal/posse/releasestamp.go turns into a third, non-UNKNOWN state. Drop
// that flag and the defect comes straight back, invisibly: every Go test in
// the tree passes (ReleaseTag's default is "" and every test binary is
// already unstamped), the release builds, installs and runs, and the only
// place it shows is `posse gates | head -1` on a box nobody here owns. That
// is the one direction no unit arm can hold, which is why it is held against
// the script's own text.
//
// AND IT HAS TO BE THE RIGHT SYMBOL. `go build -ldflags "-X <symbol>=<v>"`
// on a symbol that does not exist is SILENT — exit 0, variable keeps its
// default (cagestale_test.go measured it for the Build stamp) — so the
// script is compared against internal/posse.ReleaseTagSymbol rather than
// against a second copy of the string. The other half of that loop is
// executable and lives in cmd/posse/gatesrelease_qa_test.go, which builds
// through that constant and reads the state back off `posse gates`: together
// they say the constant names a real symbol AND the script stamps that
// constant.
//
// Named files, no enumeration: this reads scripts/release-artifacts.sh and
// three other build-line files by name, so it is not in the tree-wide door
// class (treewidedoor_qa_test.go) and needs no Makefile door.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// releaseArtifactsScript is the script that cuts a release, read from the
// tree. Not a fixture: what is pinned is what CI runs on a tag.
func releaseArtifactsScript(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("scripts/release-artifacts.sh")
	if err != nil {
		t.Fatalf("cannot read the release build script: %v", err)
	}
	return string(body)
}

// The release build stamps the tag, through the symbol the Go side names,
// with the tag the rest of the script already validated against
// internal/posse.Version.
func TestQATheReleaseBuildStampsTheReleaseTag(t *testing.T) {
	body := releaseArtifactsScript(t)
	want := "-X " + posse.ReleaseTagSymbol + "=$VERSION"
	if !strings.Contains(body, want) {
		t.Fatalf("scripts/release-artifacts.sh does not carry %q — without it every release install reads \"UNKNOWN whether this binary carries the wall renders … that landed\" forever, with every test in this tree green (ranger-base-ctath)", want)
	}
	// On the same line as the Build stamp, because they are one identity: a
	// tag on a binary that names no commit is refused by readRelease, so a
	// release that stamped only the tag would reach the UNKNOWN arm anyway
	// — and would do it after shipping.
	var found bool
	for _, ln := range strings.Split(body, "\n") {
		if strings.Contains(ln, want) {
			found = true
			if !strings.Contains(ln, "-X github.com/ranger360ai/posse/internal/posse.Build=") {
				t.Errorf("the release tag is stamped without the sha beside it, which readRelease refuses:\n%s", strings.TrimSpace(ln))
			}
		}
	}
	if !found {
		t.Fatal("the stamp was matched across the whole file but on no single line")
	}
	// $VERSION is the tag the script checked against internal/posse.Version
	// (its own "THE VERSION IS THE CODE'S, NOT THE TAG'S" preflight), and
	// that check is what makes the stamp believable at all — readRelease
	// refuses a tag that disagrees with the compiled-in version.
	if !strings.Contains(body, `if [ "$src_version" != "$bare" ]; then`) {
		t.Error("the tag/source version preflight is gone, so $VERSION is no longer known to agree with internal/posse.Version")
	}
}

// NOTHING ELSE MAY STAMP IT. The release state's whole guarantee is that a
// dev build cannot reach it, and that guarantee is the absence of this flag
// from every other build line — a `make build` that stamped it would let a
// clean checkout build at any commit matching Version claim to be the
// published release.
//
// A FLOOR, and read as one: these are the three other build lines the tree
// has today, named rather than enumerated. The full census would be a
// tree-wide enumeration needing its own Makefile door, which is not worth
// buying for this direction — a stamp added to another build line is a
// visible, deliberate edit to a line somebody is already reading, while the
// direction above (the flag quietly going missing) is the one that ships.
func TestQANoOtherBuildLineStampsTheReleaseTag(t *testing.T) {
	for _, path := range []string{
		"Makefile",                    // `make build` / the LDFLAGS var
		"scripts/clean-build.sh",      // `make release`, a throwaway worktree of HEAD
		"internal/posse/cagestale.go", // the cage's inner binary
		".github/workflows/ci.yml",    // every branch build CI makes
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("cannot read %s: %v", path, err)
			continue
		}
		if strings.Contains(string(body), posse.ReleaseTagSymbol) {
			t.Errorf("%s names %s — only scripts/release-artifacts.sh may stamp it, or a dev build can claim to be a published release (ranger-base-ctath)", path, posse.ReleaseTagSymbol)
		}
	}
}

// ─── the value that reaches the linker flag ───────────────────────────────────

// $VERSION is now interpolated into the `-ldflags` STRING, where go splits on
// spaces and `v1 -X evil=1` would become a second linker flag. The script's
// own `case $VERSION in v[0-9]*)` was a PREFIX check — the exact shape
// release.yml's guard was hardened away from (ranger-base-u9at) — so it is
// anchored now, and this arm runs the real script over the alphabet that
// defeated the prefix form.
//
// REFUSALS ONLY, plus one control that gets past the shape check and dies at
// the next one: an accepted version builds four binaries and eight artifacts,
// which is bottle_qa_test.go's job and not a guard pin's. Every case here
// exits before the script writes anything at all — the version check sits
// above the --out guard, the worktree and the build.
func TestQAReleaseVersionGuardRefusesEveryNonVersionString(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "release-artifacts.sh")
	for _, c := range []struct {
		version string
		refused bool
	}{
		// The injections the prefix check let through, and the one that
		// matters most here: a space makes a second -X.
		{"v1 -X github.com/ranger360ai/posse/internal/posse.ReleaseTag=v9.9.9", true},
		{"v0.3.0$(id)", true},
		{"v1`id`", true},
		{"v9;curl evil", true},
		{"v1.0&&id", true},
		{"v1.0|id", true},
		{"v0.3.0\nid", true},
		// Shapes that are not versions.
		{"v1.0_", true},
		{"v1.", true},
		{"v1..2", true},
		{"v1.0-rc1", true},
		{"1.0", true},
		{"main", true},
		{"", true}, // --version with an empty value: the arg parser's own refusal
		// THE CONTROL. A well-formed version gets past the shape check and
		// is stopped by the tag/source preflight instead — which is what
		// proves the arms above are refused by SHAPE and not by the script
		// refusing everything.
		{"v9.9.9", false},
	} {
		t.Run(strconv.Quote(c.version), func(t *testing.T) {
			cmd := exec.Command(script, "--version", c.version,
				"--rev", "HEAD", "--out", filepath.Join(t.TempDir(), "dist"))
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			got := string(out)
			shape := strings.Contains(got, "--version must look like vX.Y.Z") ||
				strings.Contains(got, "--version needs a tag")
			if c.refused {
				if err == nil {
					t.Fatalf("%q was accepted — it reaches the -ldflags string:\n%s", c.version, got)
				}
				if !shape {
					t.Errorf("%q was refused for some other reason, so the shape guard is not what stopped it:\n%s", c.version, got)
				}
				return
			}
			if shape {
				t.Fatalf("the control %q was refused by the shape guard, so every arm above proves nothing:\n%s", c.version, got)
			}
			if !strings.Contains(got, "tag/source version mismatch") {
				t.Errorf("the control got past the shape guard and did not reach the preflight:\n%s", got)
			}
		})
	}
}
