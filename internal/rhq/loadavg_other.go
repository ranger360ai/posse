//go:build !darwin && !linux

package rhq

import "runtime"

// sysLoad1 has no reading to take on a platform nobody has written one for.
// The guard treats that as unreadable and fails open, one line — see
// App.LoadHigh.
func sysLoad1() (float64, error) {
	return 0, Die("no load-average reader for %s", runtime.GOOS)
}
