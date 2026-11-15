package posse

import (
	"os"
	"testing"
)

// gitGlobalPinOf prepares, as a launch does, the read-only snapshot of a
// global file, and returns the GIT_CONFIG_GLOBAL row's value for it — the
// snapshot, never the file itself (ranger-base-161t2). Shared, because the
// pins that hold the row against git's own answer live in more than one arm.
// Fatal when no snapshot can be made: a fixture that cannot snapshot is
// grading the degraded path, and says so where it means to.
func gitGlobalPinOf(t *testing.T, file string) string {
	t.Helper()
	snap, err := gitGlobalSnapshot(file, SeatWallRoot(os.Getenv("HOME")))
	if err != nil {
		t.Fatalf("no read-only snapshot of %s: %v", file, err)
	}
	v, err := gitGlobalPinValue(file)
	if err != nil || v != snap {
		t.Fatalf("gitGlobalPinValue(%s) = %q (%v) after preparing %s — the render does not name the snapshot the launch wrote (ranger-base-161t2)", file, v, err, snap)
	}
	return v
}
