package treepins

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotesFragmentIndexIsCurrent(t *testing.T) {
	if out, err := exec.Command("python3", "scripts/notes-index.py", "--check").CombinedOutput(); err != nil {
		t.Fatalf("notes index: %v\n%s", err, out)
	}
}

func TestNotesFragmentIndexDetectsMissingEntries(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(check bool) ([]byte, error) {
		args := []string{"scripts/notes-index.py", "--directory", dir}
		if check {
			args = append(args, "--check")
		}
		return exec.Command("python3", args...).CombinedOutput()
	}
	write("earlier.md", "## Earlier heading\n2026-08-01\n")
	write("recent.md", "# Recent heading\n2026-09-02\n")
	write("undated.md", "## No date\n")
	if out, err := run(false); err != nil {
		t.Fatalf("generate: %v\n%s", err, out)
	}
	if out, err := run(true); err != nil {
		t.Fatalf("fresh index: %v\n%s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	last := -1
	for _, want := range []string{"## 2026-09", "[recent](recent.md) — Recent heading — 2026-09-02", "## 2026-08", "[earlier](earlier.md) — Earlier heading — 2026-08-01", "## Undated", "[undated](undated.md) — No date"} {
		at := strings.Index(s, want)
		if at <= last {
			t.Fatalf("missing or out of order: %q\n%s", want, s)
		}
		last = at
	}
	write("new.md", "# Newly added fragment\n")
	if out, err := run(true); err == nil || !strings.Contains(string(out), "stale index") {
		t.Fatalf("missing fragment must fail: %v\n%s", err, out)
	}
	if out, err := run(false); err != nil {
		t.Fatalf("regenerate: %v\n%s", err, out)
	}
	if out, err := run(true); err != nil {
		t.Fatalf("regenerated index: %v\n%s", err, out)
	}
}
