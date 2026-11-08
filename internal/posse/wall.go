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
// THREE QUESTIONS, AND THEY ARE NOT THE SAME QUESTION. WallMechanism below
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
//
// AND THE THIRD: CAN POSSE TYPE IT (ranger-base-pl3ck). A mechanism this
// build has no render for is a wall nothing puts up, however available the
// host says it is — `wallRendered`'s two call sites answer the first two
// questions and then type `SeatbeltPrefix`, which is `sandbox-exec` and
// nothing else. So the table fill reads all three (seatbeltTierProvidedOn),
// and a Linux host with bwrap and a passing probe is REFUSED at
// parity.go's "cage %s is not available on this host/build" rather than
// handed a seatbelt row over a binary it does not have. ADR 0080 D18
// (ranger-base-djl3c) is what TYPES bwrap's wall — `bwrap.args` written and
// the prefix put in SeatbeltPrefix's slot at both call sites; D17's pure
// argv render is not the same thing and does not lift this. The day D18
// lands, wallMechanismRendered names WallBwrap too and this host gains the
// tier in one line. Three questions and not two, because collapsing this one into
// availability would make the apply probe answer for a render, and
// collapsing it into the NAME would print bare `seatbelt` on the Linux host
// again — the exact reading the paragraph above keeps apart.

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
// "what makes this tier available" has one function to read — and it is
// seatbeltTierProvidedOn, which is availability AND a render, not
// availability alone (ranger-base-pl3ck).
func init() {
	if seatbeltTierProvidedOn(runtime.GOOS) {
		AvailableCages[CageSeatbelt] = true
	}
}

// seatbeltTierProvidedOn is the whole question the table fill asks: this host
// can realize the L2 wall, AND posse has a render to type for the mechanism
// that would realize it here. Both halves, in the one function init calls, so
// a pin can drive either from the other host.
//
// Parameterized on GOOS for seatbeltTierAvailableOn's reason, and it is the
// GOOS that decides the mechanism, so the second half is pure.
func seatbeltTierProvidedOn(goos string) bool {
	return wallMechanismRendered(wallMechanismOn(goos)) && seatbeltTierAvailableOn(goos)
}

// wallMechanismRendered reports whether this build can TYPE the wall for a
// mechanism — the third question in the file comment, and the one nothing
// between the availability table and the launch line used to ask.
//
// Today it is sandbox-exec alone: `RenderSeatbelt` writes a sandbox-exec
// profile and `SeatbeltPrefix` types `sandbox-exec -f <prof>`, both
// unconditionally, at both of wallRendered's call sites. The question is
// about TYPING the wall and not about rendering an argv: ADR 0080 D17's
// `BwrapArgs` is a pure function with no call site on a launch line, so it
// does not make this true. D18 (ranger-base-djl3c) replaces both
// SeatbeltPrefix call sites; this returns true for WallBwrap in the same
// commit that does, and the pin beside it reds if one lands without the
// other.
func wallMechanismRendered(m string) bool { return m == WallSandboxExec }

// seatbeltTierAvailableOn reports whether this HOST can realize the L2
// write wall, by whichever mechanism this GOOS uses. A host question and not
// a process one, for the reason SeatbeltAvailable's comment gives: what it
// gates is a session posse is about to launch, whose wall binary is typed
// into a herdr pane and runs in herdr's process tree.
//
// HALF of what the table fill asks, and not all of it: a host that can is not
// a host posse can type the wall on. seatbeltTierProvidedOn above is the
// whole question, and this one is deliberately left answering only its own
// (ranger-base-pl3ck).
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
