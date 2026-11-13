//go:build !linux

package posse

import (
	"fmt"
	"io"
)

// RunWallWatch has nothing to watch off Linux: the seatbelt mechanism
// matches a path at access time and puts up no mount a rename could take
// down (ranger-base-0qpsw), and only the bwrap render types this verb.
func RunWallWatch(argv []string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "posse: %s watches a bwrap seat's mounts, and bwrap is the Linux mechanism — this host has none to watch\n", WallWatchVerb)
	return 2
}

// runWallWatch is the same refusal for the live pin, which skips before it
// gets here on any host but Linux.
func runWallWatch(r wallWatchRun) int { return RunWallWatch(nil, r.Stderr) }
