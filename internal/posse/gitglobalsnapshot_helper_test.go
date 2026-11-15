package posse

import "testing"

// gitGlobalPinOf is the GIT_CONFIG_GLOBAL row's value for git's resolved
// global file: its read-only snapshot, never the file itself
// (ranger-base-161t2). Shared, because the pins that hold the row against
// git's own answer live in more than one arm. Fatal when no snapshot can be
// made: a fixture that cannot snapshot is grading the degraded path, and
// says so where it means to.
func gitGlobalPinOf(t *testing.T, file string) string {
	t.Helper()
	v, err := gitGlobalPinValue(file)
	if err != nil {
		t.Fatalf("no read-only snapshot of %s: %v", file, err)
	}
	if v == file {
		t.Fatalf("gitGlobalPinValue(%s) named the file itself, so a seat's `git config --global` writes it (ranger-base-161t2)", file)
	}
	return v
}
