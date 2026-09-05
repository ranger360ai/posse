package main

// `posse status`'s half of the second-store sweep (ranger-base-dj3k2, ADR
// 0012 D3, the September 2026 adherence audit's finding 6).
//
// The sweep, its truth table and the two sentences are pinned beside the
// code that computes them (internal/posse/secondstore_qa_test.go). What no
// test in that package can reach is whether this COMMAND prints them — the
// half a refactor drops silently, and the half that matters, because a fact
// nothing prints is a fact nobody has.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// statusBeadsDir plants files in a repo's own .beads, creating it first.
func statusBeadsDir(t *testing.T, repo string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(repo, ".beads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The command prints the line, off the files, naming the path and the fix —
// and it stays a READING: the exit code is the one the same shop hands back
// with no second store in it.
func TestQAStatusPrintsTheSecondStoreLine(t *testing.T) {
	bin := buildRhq(t)
	home, repo := t.TempDir(), t.TempDir()
	instance := t.TempDir()
	writeStatusConfig(t, home, repo, "")
	statusBeadsDir(t, instance, nil)

	// The control first, on the same instance: a redirect and no store
	// beside it says nothing. Without it a line printed unconditionally
	// would pass the arm below.
	statusBeadsDir(t, repo, map[string]string{"redirect": filepath.Join(instance, ".beads") + "\n"})
	clean, cleanCode := runStatusCode(t, bin, home)
	if strings.Contains(clean, "second store:") {
		t.Fatalf("a redirect with no store beside it must say nothing:\n%s", clean)
	}

	// …and now the audit's shape.
	statusBeadsDir(t, repo, map[string]string{
		"beads.db":     "SQLite format 3\x00",
		"issues.jsonl": `{"id":"x-1"}` + "\n",
	})
	out, code := runStatusCode(t, bin, home)
	line := ""
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "second store:") {
			if line != "" {
				t.Fatalf("one store is one line:\n%s", out)
			}
			line = ln
		}
	}
	if line == "" {
		t.Fatalf("status printed nothing about the second store:\n%s", out)
	}
	for _, want := range []string{
		filepath.Join(repo, ".beads"), // the path
		"beads.db, issues.jsonl",      // what is in it
		filepath.Join(instance, ".beads"),
		"delete it (ADR 0012 D3)", // the fix
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the line is missing %q:\n%s", want, line)
		}
	}
	// A report, never a refusal: finding a second store may not be what
	// decides this command's exit code (ADR 0012 D3's exit hatch is an `rm`
	// the operator types, and posse deleting a database nobody asked it to
	// delete is the worse incident).
	if code != cleanCode {
		t.Errorf("a second store moved the exit code from %d to %d — it is a reading, not a condition", cleanCode, code)
	}
	// And it is still there afterwards.
	for _, name := range []string{"redirect", "beads.db", "issues.jsonl"} {
		if _, err := os.Stat(filepath.Join(repo, ".beads", name)); err != nil {
			t.Errorf("`posse status` deleted %s: %v", name, err)
		}
	}
}

// The other control, and the one the bead names by hand: a repo whose
// `.beads` holds its own database and NO redirect is every ordinary bd
// repo, this instance's queue included. A check that reported it would fire
// on almost every shop that runs posse.
func TestQAStatusSaysNothingAboutAnOrdinaryLocalStore(t *testing.T) {
	bin := buildRhq(t)
	home, repo := t.TempDir(), t.TempDir()
	writeStatusConfig(t, home, repo, "")
	statusBeadsDir(t, repo, map[string]string{
		"beads.db":     "SQLite format 3\x00",
		"issues.jsonl": `{"id":"x-1"}` + "\n",
	})
	if out, _ := runStatusCode(t, bin, home); strings.Contains(out, "second store:") {
		t.Fatalf("a store of record with no redirect is not a second store:\n%s", out)
	}
}

// runStatusCode is runStatus with the exit code kept, which is what makes
// "a reading, not a condition" assertable.
func runStatusCode(t *testing.T, bin, home string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, "status")
	cmd.Env = statusEnv(t, home)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse status: %v", err)
	}
	return string(out), code
}
