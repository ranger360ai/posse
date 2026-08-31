//go:build linux

package posse

import (
	"os"
	"strconv"
	"strings"
)

// sysLoad1 on Linux is the first field of /proc/loadavg. A file read, not a
// fork, for the reason spelled out in loadavg_darwin.go.
func sysLoad1() (float64, error) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0, Die("/proc/loadavg is empty")
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, Die("/proc/loadavg: %q is not a load average", f[0])
	}
	return v, nil
}
