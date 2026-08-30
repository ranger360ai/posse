package posse

// QA pin for ranger-base-42mv: our own suite must not leak bd daemons.
//
// bd 0.49.1 auto-starts a per-database daemon on first use against a database
// and NOTHING ever stops it. A bd call in a throwaway directory therefore
// leaves a process behind that outlives its directory — holding a sqlite
// handle to a database nobody can reach, and (measured 2026-08-25 and again
// 2026-08-28) holding the directory open well enough to defeat t.TempDir's own
// RemoveAll, so it leaks a process AND a directory. Ten accumulated on the
// live box in twelve days that way; two of them were filed in a single evening
// by TestLiveWorktreeSharesOneGraph, which is what turned this from somebody
// else's mess into ours.
//
// The rule this pins is per FILE, because that is the granularity at which
// the two honest answers are legible:
//
//   - `--no-daemon` on the calls — prevention, nothing is ever started. The
//     right answer when the daemon is not part of the claim.
//   - a cleanup that stops what was started — the right answer when a RUNNING
//     daemon IS the claim (liveCageBeadStore in internal/rhq: it imports a
//     newer JSONL before answering, and stopping it first turns the same read
//     into a staleness refusal). `.beads/daemon.pid` beside the fixture's own
//     database is that cleanup's handle, and never `bd daemon stop-all`, which
//     would take the canonical queue's daemon with it.
//
// A file that shells the real bd and does neither is the leak, and is what
// worktreelive_test.go was until 2026-08-28.
//
// This is the prevention half of the bead. The detection half — naming a
// daemon whose working directory is gone or throwaway, on the live box — is
// scripts/verify-bd-pin.sh's cwd layer, pinned in bdpin_qa_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bdCallSite is one `exec.Command("bd", …)` in the tree — a call to the
// operator's real bd, as opposed to the fake ones the hermetic tests write
// onto a stub PATH, which start no daemon and are not this rule's business.
type bdCallSite struct {
	file string
	line int
}

// guardFile is this file, excluded from the scan for the reason given at the
// skip. Kept as a constant so the exclusion is one name, not a path.
const guardFile = "bddaemonleak_qa_test.go"

// scanRealBdCallSites returns every real-bd call site under root, and the
// files that hold them. Errors are returned rather than swallowed: a walk
// that reads nothing must not look like a tree with nothing in it.
func scanRealBdCallSites(root string) (sites []bdCallSite, files map[string]string, err error) {
	files = map[string]string{}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if name := info.Name(); name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// This guard's OWN source is not a caller. Its three matches are a
		// doc comment, the matcher literal below, and the control's planted
		// fixture text — so counting them made the zero-witness unfirable:
		// delete every real bd caller in the tree and the scan still read
		// "3 sites in 1 file" off itself and passed (measured 2026-08-30,
		// ranger-base-athy). A guard whose proof that it measured something
		// is satisfied by reading itself is the shape it exists to catch.
		if filepath.Base(path) == guardFile {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		body := string(b)
		rel, _ := filepath.Rel(root, path)
		for i, line := range strings.Split(body, "\n") {
			if strings.Contains(line, `exec.Command("bd"`) {
				sites = append(sites, bdCallSite{file: rel, line: i + 1})
				files[rel] = body
			}
		}
		return nil
	})
	return sites, files, err
}

// leakyBdTestFiles names the files that shell the real bd and neither pass
// `--no-daemon` nor stop the daemon they started.
func leakyBdTestFiles(files map[string]string) []string {
	var bad []string
	for rel, body := range files {
		if strings.Contains(body, "--no-daemon") || strings.Contains(body, "daemon.pid") {
			continue
		}
		bad = append(bad, rel)
	}
	return bad
}

func TestQANoTestShellsRealBdWithoutStoppingItsDaemon(t *testing.T) {
	sites, files, err := scanRealBdCallSites(".")
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	// The positive witness. An assertion of pure absence is satisfied by a
	// scan that measured nothing — a renamed helper, a walk that hit the
	// wrong root — so say out loud what was actually read, and fail if the
	// answer is "no live bd anywhere", which has not been true since
	// rangerhq-09o2 shipped.
	if len(sites) == 0 {
		t.Fatal("scanned the tree and found no `exec.Command(\"bd\"` at all — this guard is measuring nothing, not passing")
	}
	t.Logf("scanned %d real-bd call sites in %d files", len(sites), len(files))

	if bad := leakyBdTestFiles(files); len(bad) > 0 {
		t.Errorf("these files shell the real bd and leave its daemon running: %s\n"+
			"pass `--no-daemon` on the calls, or stop the daemon in a t.Cleanup by SIGTERMing "+
			"the pid in the fixture's own .beads/daemon.pid — never `bd daemon stop-all`, "+
			"which would take the canonical queue's daemon with it (ranger-base-42mv)", strings.Join(bad, ", "))
	}
}

// The control. The rule above is a rule about text, so it is only worth
// anything if a file that breaks it is actually caught — this plants the
// shape worktreelive_test.go had before the fix and asserts both arms.
func TestQABdDaemonGuardCatchesALeakyFile(t *testing.T) {
	root := t.TempDir()
	leaky := "package x\n\nfunc f() { cmd := exec.Command(\"bd\", \"list\"); _ = cmd }\n"
	if err := os.WriteFile(filepath.Join(root, "leaky_test.go"), []byte(leaky), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err := scanRealBdCallSites(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 {
		t.Fatalf("the planted call site was not seen: %d sites — the scan, not the rule, is what failed", len(sites))
	}
	if bad := leakyBdTestFiles(files); len(bad) != 1 || bad[0] != "leaky_test.go" {
		t.Fatalf("a file that shells real bd with neither guard must be flagged, got %v", bad)
	}

	// …and the other arm, or "flagged" would just mean "flags everything":
	// the same file with the flag is clean.
	fixed := strings.Replace(leaky, `exec.Command("bd", "list")`, `exec.Command("bd", "--no-daemon", "list")`, 1)
	if err := os.WriteFile(filepath.Join(root, "leaky_test.go"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, files, err = scanRealBdCallSites(root)
	if err != nil {
		t.Fatal(err)
	}
	if bad := leakyBdTestFiles(files); len(bad) != 0 {
		t.Fatalf("`--no-daemon` must clear the file, got %v", bad)
	}

	// …and so does stopping the daemon in cleanup, which is the answer for a
	// fixture whose claim needs a running one.
	stopped := strings.Replace(leaky, "_ = cmd", "_ = cmd; _ = \".beads/daemon.pid\"", 1)
	if err := os.WriteFile(filepath.Join(root, "leaky_test.go"), []byte(stopped), 0o644); err != nil {
		t.Fatal(err)
	}
	_, files, err = scanRealBdCallSites(root)
	if err != nil {
		t.Fatal(err)
	}
	if bad := leakyBdTestFiles(files); len(bad) != 0 {
		t.Fatalf("a daemon.pid cleanup must clear the file, got %v", bad)
	}
}

// The zero-witness must be able to FIRE. TestQANoTestShellsRealBd…'s
// `len(sites) == 0` guard exists so an empty PASS cannot masquerade as a
// clean tree — a renamed helper, a walk rooted somewhere wrong. Until
// 2026-08-30 it could not fire at all: the scan counted this file's own
// comment, matcher literal and fixture text as three call sites, so a tree
// with no bd caller whatsoever still reported "3 sites in 1 file" and
// passed. This is the arm that keeps the exclusion honest, and it is a
// without-arm for the control above: that one proves a leaky file is
// caught, this one proves an EMPTY tree is not silently blessed.
func TestQABdDaemonGuardZeroWitnessCanFire(t *testing.T) {
	root := t.TempDir()
	b, err := os.ReadFile(guardFile)
	if err != nil {
		t.Fatal(err)
	}
	// A tree holding only this guard: every real bd caller gone.
	if err := os.WriteFile(filepath.Join(root, guardFile), b, 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err := scanRealBdCallSites(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 || len(files) != 0 {
		t.Fatalf("a tree with no bd caller but this guard must scan to nothing, "+
			"or the zero-witness is satisfied by the guard reading its own source: "+
			"%d sites in %d files", len(sites), len(files))
	}

	// …and the other direction, or "scans to nothing" would just mean the
	// walk is broken: one real caller beside it is still seen.
	caller := "package x\n\nfunc f() { _ = exec.Command(\"bd\", \"list\") }\n"
	if err := os.WriteFile(filepath.Join(root, "other_test.go"), []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, files, err = scanRealBdCallSites(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || len(files) != 1 {
		t.Fatalf("the exclusion must skip only this file, not blind the walk: %d sites in %d files", len(sites), len(files))
	}
	if bad := leakyBdTestFiles(files); len(bad) != 1 || bad[0] != "other_test.go" {
		t.Fatalf("the real caller must still be judged, got %v", bad)
	}
}
