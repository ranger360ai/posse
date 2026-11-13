//go:build !linux

package posse

import "runtime"

// procStartOf has no reading to take on a platform with no `/proc`, and the
// refusal is the whole point: runtimesession.go's fence is what tells a
// session row written for THIS process from one left behind by a dead
// process that held the same pid, and without it the join would read another
// seat's submits as this seat's echo.
//
// So this answers "not available", the echo reading stays off, and a composer
// on the capture route is judged exactly as it was before ranger-base-qzujz —
// which is ADR 0013's unknown-is-not-no in the direction that costs a reading
// rather than an invented hold. A platform that grows a start-time reading
// gets a file of its own — on darwin the place to look is the process table
// `cmd/checkorphans` already reads through `sysctl kern.proc` — and it is NOT
// written here, because claude's own `procStart` has been measured against
// the kernel on linux alone (2026-11-10, six live processes, exact string
// matches) and a fence compared against an unmeasured spelling is a fence
// that passes for the wrong reason.
func procStartOf(pid int) (string, error) {
	return "", Die("no process start time reading on %s, so a claude session row cannot be fenced against pid reuse (procstart_other.go)", runtime.GOOS)
}

// procCwdOf has no `/proc` to read either. Nothing reaches it off linux
// today — procStartOf refuses first and the join stops there — and it
// answers "not available" rather than a guess so that stays true if that
// order ever changes.
func procCwdOf(pid int) (string, error) {
	return "", Die("no process working directory reading on %s (procstart_other.go)", runtime.GOOS)
}
