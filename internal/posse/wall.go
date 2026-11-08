package posse

// WHICH MECHANISM REALIZES THE L2 WALL ON THIS HOST (ADR 0080 D16).
//
// `cage: seatbelt` is the name of a TIER — "the L2 path-scoped write wall",
// ADR 0002 §3 — and not of a binary. The binary behind it is a host fact:
// `sandbox-exec` on darwin, `bwrap` on Linux, the way `cage: container` is
// realized by whichever engine the home names. A PID never names the
// mechanism, which is the whole point of keeping the tier's name: the
// promoted constitution is read on both hosts (ADR 0083), so one
// `cage: seatbelt` line has to mean the same demand on each, and a per-host
// tier name would mean a per-host constitution.
//
// TWO QUESTIONS, AND THEY ARE NOT THE SAME QUESTION. WallMechanism below
// answers "which mechanism would put the wall up here"; it is pure, reads no
// host beyond GOOS, and never answers "". `AvailableCages[CageSeatbelt]`
// answers "can it", and that one is measured — on Linux by resolving `bwrap`
// and by an apply probe, because a kernel or LSM that refuses unprivileged
// user namespaces leaves the binary on PATH and refuses the mount (bwrap.go).
// They are separated deliberately: a Linux host whose kernel refuses the
// namespace prints `seatbelt (bwrap)` over a block of ✗ rows, because the
// mechanism there IS bwrap and the rows are what say it did not happen.
// Folding availability into the name would print bare `seatbelt` on that
// host, which reads as a tier with no mechanism rather than as a mechanism
// that was refused.

import "runtime"

// The mechanism names. Each is the BINARY that puts the wall up, which is
// what makes it worth printing: an operator reading `seatbelt (bwrap)` off a
// parity row has the name of the thing to go and look at.
const (
	WallSandboxExec = "sandbox-exec"
	WallBwrap       = "bwrap"
)

// WallMechanism names what realizes the seatbelt tier on this host:
// `sandbox-exec`, `bwrap`, or "" on a GOOS with neither. See the file
// comment for why this is a naming question and not an availability one.
func WallMechanism() string { return wallMechanismOn(runtime.GOOS) }

// wallMechanismOn is that answer for an arbitrary GOOS, so a pin can read
// the Linux branch from darwin. Nothing but the OS decides it.
func wallMechanismOn(goos string) string {
	switch goos {
	case "darwin":
		return WallSandboxExec
	case "linux":
		return WallBwrap
	}
	return ""
}

// CageLabel renders a cage tier for a report: the seatbelt tier carries the
// mechanism realizing it here, every other tier is its own name.
//
// A LABEL AND NEVER A VALUE. `Parity.Cage` stays the bare tier, because it
// is compared against `CageSeatbelt`, written into session records and read
// back by `Sessions()`; this is only how a row says it to a reader. The two
// surfaces that print it are Parity.String and, through it, `posse gates`.
func CageLabel(cage string) string {
	if cage != CageSeatbelt {
		return cage
	}
	m := WallMechanism()
	if m == "" {
		return cage
	}
	return cage + " (" + m + ")"
}

// The one place the seatbelt tier's availability is filled in, for both
// mechanisms. Two inits — one per mechanism — would each be correct about
// its own host and neither would be the answer; this way a reader asking
// "what makes this tier available" has one function to read.
func init() {
	if seatbeltTierAvailableOn(runtime.GOOS) {
		AvailableCages[CageSeatbelt] = true
	}
}

// seatbeltTierAvailableOn reports whether this HOST can realize the L2
// write wall, by whichever mechanism this GOOS uses. A host question and not
// a process one, for the reason SeatbeltAvailable's comment gives: what it
// gates is a session posse is about to launch, whose wall binary is typed
// into a herdr pane and runs in herdr's process tree.
//
// Parameterized on GOOS so a pin can drive the Linux branch on darwin
// (ranger-base-eoxo1); the switch has already decided the OS by the time
// each arm runs, so SeatbeltAvailable's own darwin check is redundant here
// and left alone rather than split, since that answer has callers of its own.
func seatbeltTierAvailableOn(goos string) bool {
	switch wallMechanismOn(goos) {
	case WallSandboxExec:
		return SeatbeltAvailable()
	case WallBwrap:
		return bwrapAvailableOn(goos)
	}
	return false
}
