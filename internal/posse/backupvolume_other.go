//go:build !darwin && !linux

package posse

import "runtime"

// volumeOf has no reading to take on a platform nobody has written one for,
// and this is the one guard in the harness that must fail CLOSED when it
// cannot read: `posse backup` refuses a target it cannot certify is on this
// box (ADR 0036 §3, as the 2026-09-01 sub-ruling narrowed it). A load guard
// that cannot read the load fails open and launches; a refusal that cannot
// read the volume and proceeds anyway is not a refusal.
func volumeOf(path string) (volume, error) {
	return volume{}, Die("no local-volume reader for %s — posse backup cannot certify %s is on this box", runtime.GOOS, AbbrevHome(path))
}
