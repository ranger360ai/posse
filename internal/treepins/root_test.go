package treepins

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// repoRoot finds the module containing the test working directory. TestMain
// installs this directory once, before m.Run, so every existing relative path
// and child command keeps reading the repository rather than the package dir.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above test working directory")
		}
		dir = parent
	}
}

func TestTreePinsRunFromModuleRoot(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil || cwd != root {
		t.Fatalf("working directory = %q, %v; want module root %q", cwd, err, root)
	}
	files, err := filepath.Glob(filepath.Join(root, "*_test.go"))
	if err != nil || len(files) != 0 {
		t.Fatalf("module root must contain no tests: %v, %v", files, err)
	}
}
