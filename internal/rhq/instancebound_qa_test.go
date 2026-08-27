package rhq

// QA pin for ADR 0012 App.A 5 (verifying rangerhq-24yt under rangerhq-ikx5).
// A fresh deployer's `go test ./...` must not name the originating instance's
// crew, operator, or home. rangerhq-24yt renamed the suite onto the shipped
// example roles; this pin is the invariant that commit did not encode, and
// that rangerhq-oay walked back the next day.
//
// Tests marked t.Skip pin a filed bug: they encode the expected behavior
// and fail today. Remove the skip when the bead closes.

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func qibRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %s has no go.mod: %v", root, err)
	}
	return root
}

// Assembled so this file itself does not contain the banned spellings.
func qibCrewPattern() *regexp.Regexp {
	names := []string{
		"di" + "nesh",
		"gil" + "foyle",
		"hoo" + "ver",
		"lau" + "rie",
		"jar" + "ed",
		"mon" + "ica",
		"rich" + "ard",
		"erl" + "ich",
		"da" + "ve",
		"david" + "stacy",
	}
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(names, "|") + `)\b`)
}

func TestFixturesNameRolesNotThisCrew(t *testing.T) {
	t.Skip("ranger-base-h6fx: 32 other test files still name the originating instance's crew (223 hits); ranger-base-idq cleared modelavail_test.go")

	root := qibRepoRoot(t)
	re := qibCrewPattern()
	var hits []string
	for _, rel := range []string{"cmd", "internal", "etc"} {
		err := filepath.WalkDir(filepath.Join(root, rel), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			base := d.Name()
			if base == "instancebound_qa_test.go" {
				return nil
			}
			inTestdata := false
			for _, p := range strings.Split(path, string(os.PathSeparator)) {
				if p == "testdata" {
					inTestdata = true
					break
				}
			}
			if !inTestdata && !strings.HasSuffix(base, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relPath, _ := filepath.Rel(root, path)
			for i, line := range bytes.Split(body, []byte("\n")) {
				if loc := re.FindIndex(line); loc != nil {
					hits = append(hits, relPath+":"+strconv.Itoa(i+1)+": "+string(line[loc[0]:loc[1]]))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) > 0 {
		t.Errorf("ADR 0012 App.A 5: fixture names the originating instance (%d hits):\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}
