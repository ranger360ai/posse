//go:build posse_arm3

package posse

// ranger-base-53w1 fixed WHICH manifest scripts/verify-detection.sh replays
// its fixtures against — the checkout, staged into a throwaway
// XDG_CONFIG_HOME, instead of whatever the operator last installed. These two
// pins are the arms that fix leaves standing, both found while verifying that
// close (ranger-base-u2nq):
//
//   - the run reports "N/N fixtures OK" against no floor at all, and the
//     agents it replays come from the *.toml files present in the tree. Delete
//     an override and its fixtures stop being replayed with it: measured, the
//     whole of etc/herdr/agent-detection/grok.toml removed leaves "4/4
//     fixtures OK", exit 0, and the four startup_splash fixtures rangerhq-uglc
//     added are simply not run. Filed as ranger-base-j66o for the script; this
//     is the arm that keeps the suite from going quiet with it.
//   - the fixture loop checks which file answered, which is what stops a
//     bundled or cached manifest winning on version and putting the run back
//     in the illusion 53w1 was filed to end. Nothing pinned that check:
//     measured, making it inert leaves the package green.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// installComplete presents the rig's own manifests as the operator's
// installed copy: whole, current, and matching the tree. Every arm below runs
// against it, so nothing here can fail merely because an install is stale.
func installComplete(t *testing.T, root string) string {
	t.Helper()
	installed := filepath.Join(root, "installed")
	det := filepath.Join(installed, "herdr", "agent-detection")
	if err := os.MkdirAll(det, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "etc", "herdr", "agent-detection")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(det, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return installed
}

var detectionCount = regexp.MustCompile(`verify-detection: (\d+)/(\d+) fixtures OK`)

// Every fixture that ships must actually be replayed. The script's own
// "N/N fixtures OK" is a count of what it chose to run, not of what exists,
// and its agent list is the tree's *.toml files — so removing an override
// removes its own regression tests and the run still says OK. That is the
// same class 53w1 was filed about (a committed detection change the check
// cannot fail), one remove down, and the count is where the suite can see it.
//
// Reds when an override is deleted while its testdata dir stays: measured by
// moving etc/herdr/agent-detection/grok.toml aside, which takes the run to
// 4/4 and this assertion to "5 fixtures shipped but were not replayed".
func TestQAVerifyDetectionReplaysEveryFixtureItShips(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root := detectionRig(t)
	testdata := filepath.Join(root, "etc", "herdr", "agent-detection", "testdata")

	// What the tree ships, counted independently of the script.
	shipped := 0
	orphans := []string{}
	agents, err := os.ReadDir(testdata)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		if !a.IsDir() {
			continue
		}
		fs, err := os.ReadDir(filepath.Join(testdata, a.Name()))
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, f := range fs {
			if strings.HasSuffix(f.Name(), ".txt") {
				n++
			}
		}
		shipped += n
		manifest := filepath.Join(root, "etc", "herdr", "agent-detection", a.Name()+".toml")
		if _, err := os.Stat(manifest); err != nil && n > 0 {
			orphans = append(orphans, a.Name())
		}
	}
	if shipped == 0 {
		t.Fatal("no fixtures shipped — this pin would pass over an empty tree")
	}

	// An agent with fixtures and no manifest is invisible to the run: the
	// agent list is built from the manifests, so its fixtures are skipped
	// silently and the total drops with them.
	if len(orphans) != 0 {
		t.Errorf("testdata for %v has no manifest in the checkout — those fixtures are never replayed", orphans)
	}

	out, code := runDetection(t, root, installComplete(t, root))
	if code != 0 {
		t.Fatalf("intact tree: exit %d, want 0\n%s", code, out)
	}
	m := detectionCount.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no fixture count in the output — the pin cannot measure anything\n%s", out)
	}
	replayed, err := strconv.Atoi(m[2])
	if err != nil {
		t.Fatal(err)
	}
	if replayed != shipped {
		t.Errorf("%d fixtures shipped, %d replayed — the run counts what it chose to run, "+
			"not what exists, so deleting an override deletes its own tests (ranger-base-j66o)\n%s",
			shipped, replayed, out)
	}
}

// TestQAVerifyDetectionReplaysEveryFixtureItShips above measures the scope
// gap through the suite's own accounting; this pin measures the same gap
// through the SCRIPT's exit code and output, which is what a plain `make
// verify-detection` actually reports. Before ranger-base-j66o's fix, deleting
// grok.toml took the run to 4/4, exit 0, and named nothing.
func TestQAVerifyDetectionFailsAnOverrideDeletion(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root := detectionRig(t)
	installed := installComplete(t, root)

	// Control: the intact tree passes.
	out, code := runDetection(t, root, installed)
	if code != 0 {
		t.Fatalf("intact tree: exit %d, want 0\n%s", code, out)
	}

	manifest := filepath.Join(root, "etc", "herdr", "agent-detection", "grok.toml")
	testdata := filepath.Join(root, "etc", "herdr", "agent-detection", "testdata", "grok")
	fixtures, err := os.ReadDir(testdata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}

	out, code = runDetection(t, root, installed)
	if code == 0 {
		t.Fatalf("grok.toml deleted from the checkout and verify-detection still passed (exit 0)\n%s", out)
	}
	for _, f := range fixtures {
		if !strings.HasSuffix(f.Name(), ".txt") {
			continue
		}
		base := strings.TrimSuffix(f.Name(), ".txt")
		if !strings.Contains(out, base) {
			t.Errorf("deleted override's fixture %s was not named in the output\n%s", base, out)
		}
	}
}

// The staging is only half the fix: herdr resolves a manifest from local
// override, cached remote and bundled-in-binary and picks the HIGHEST version,
// so a staged override can still lose and the run go green against a file
// nobody committed. The script guards that by checking the `manifest:` path
// herdr reports for every fixture, and until this pin nothing measured the
// guard — making it inert left the package green.
//
// The arm: leave the staging alone and make the staged file unreadable as a
// manifest, so herdr answers from the bundled one instead. The run must name
// what answered, not just disagree about a state.
func TestQAVerifyDetectionNamesTheManifestThatAnswered(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("herdr"); err != nil {
		t.Skip("herdr not on PATH")
	}
	root := detectionRig(t)
	installed := installComplete(t, root)

	// Control: the tree answers for itself and every fixture passes.
	out, code := runDetection(t, root, installed)
	if code != 0 || strings.Contains(out, "manifest=") {
		t.Fatalf("intact tree: exit %d, want 0 with no manifest complaint\n%s", code, out)
	}

	manifest := filepath.Join(root, "etc", "herdr", "agent-detection", "codex.toml")
	if err := os.WriteFile(manifest, []byte("this is not a manifest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runDetection(t, root, installed)
	if code == 0 {
		t.Fatalf("the tree's manifest could not be read and the run still passed\n%s", out)
	}
	if !strings.Contains(out, "manifest=") {
		t.Errorf("the run failed without naming which file answered — that check is what keeps a "+
			"bundled or cached manifest from quietly winning on version (ranger-base-53w1)\n%s", out)
	}
}
