//go:build !linux

package posse

// wallunix_linux.go's twin off Linux. Landlock is a Linux LSM, and the wall
// mechanism off Linux is a seatbelt profile (wallMechanismOn), so neither of
// these is ever reached by a render — but both have to COMPILE and answer,
// because the darwin box is where internal/posse's suite runs and where the
// Linux render is read from (wall.go's wallMechanismHost).

import "os"

// liveWallUnixABI: there is no Landlock here, and the answer is the same one
// a Linux kernel without CONFIG_SECURITY_LANDLOCK gives — the syscall is
// absent. Negative and not 0, so `posse gates` prints "absent" rather than
// claiming a version nobody read.
func liveWallUnixABI() int { return -1 }

// liveWallUnixTask: `gettid` is a Linux call and the thread group leader is a
// Linux idea, so the answer here is the pair that says "nothing to refuse",
// and it is the honest one rather than a fail-open: the step is unreachable
// off Linux, because liveRestrictWallUnix below refuses unconditionally one
// statement later, and no seat here is ever opened by this file.
//
// IT HAS TO ANSWER "LEADER" AND NOT "NOT LEADER", for the reason the whole
// seam exists: RunWallUnix's leader refusal is read from darwin by stubbing
// wallUnixTask, so the arms BELOW it — the apply refusal and the no-runtime
// refusal — must be reachable here with the var unstubbed. A twin that
// answered "not leader" would make every darwin pin of this function read one
// sentence, the wrong one.
func liveWallUnixTask() (tid, pid int) { p := os.Getpid(); return p, p }

// liveArmWallUnixPdeathsig: PR_SET_PDEATHSIG is a Linux prctl. Unreachable
// for liveRestrictWallUnix's reason below, and it refuses rather than
// answering nil for the same one.
func liveArmWallUnixPdeathsig() error {
	return Die("%s: there is no PR_SET_PDEATHSIG on this operating system, and this step should never have been typed (ADR 0080 D16)", WallUnixVerb)
}

// liveRestrictWallUnix cannot be reached: RunWallUnix runs inside a bwrap
// cage, and nothing off Linux has one. It refuses rather than returning nil,
// for the reason RunWallUnix itself refuses — a nil here would be a wall
// reported and not applied, which is the one failure direction ADR 0080 D4
// forbids.
func liveRestrictWallUnix(int) error {
	return Die("%s: there is no Landlock on this operating system, so this seat's socket wall cannot be applied — and this step should never have been typed, because the wall mechanism here is not %s (ADR 0080 D16)", WallUnixVerb, WallBwrap)
}
