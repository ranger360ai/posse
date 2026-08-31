//go:build darwin

package posse

import (
	"encoding/binary"

	"golang.org/x/sys/unix"
)

// sysLoad1 on darwin is `sysctl -n vm.loadavg` without the fork — which is
// the whole point. The condition this guard exists to detect is fork
// starvation (ranger-base-innx), so a guard that shelled out for its reading
// would hang on exactly the load it was trying to measure.
//
// vm.loadavg is a struct loadavg: three fixed_t (uint32) averages followed by
// a long fscale to divide them by. On 64-bit darwin that is 12 bytes, 4 bytes
// of padding, then an 8-byte scale; the 32-bit layout has no padding. Any
// other length is a kernel this code has not been read against, and a guard
// that cannot read its number says so rather than inventing one.
func sysLoad1() (float64, error) {
	raw, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return 0, err
	}
	var scale float64
	switch len(raw) {
	case 24:
		scale = float64(binary.NativeEndian.Uint64(raw[16:24]))
	case 16:
		scale = float64(binary.NativeEndian.Uint32(raw[12:16]))
	default:
		return 0, Die("sysctl vm.loadavg returned %d bytes, not a struct loadavg", len(raw))
	}
	if scale <= 0 {
		return 0, Die("sysctl vm.loadavg reports fscale %g", scale)
	}
	return float64(binary.NativeEndian.Uint32(raw[0:4])) / scale, nil
}
