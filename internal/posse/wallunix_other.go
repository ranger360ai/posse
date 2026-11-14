//go:build !linux

package posse

// wallunix_linux.go's twin off Linux. Landlock is a Linux LSM, and the wall
// mechanism off Linux is a seatbelt profile (wallMechanismOn), so neither of
// these is ever reached by a render — but both have to COMPILE and answer,
// because the darwin box is where internal/posse's suite runs and where the
// Linux render is read from (wall.go's wallMechanismHost).

// liveWallUnixABI: there is no Landlock here, and the answer is the same one
// a Linux kernel without CONFIG_SECURITY_LANDLOCK gives — the syscall is
// absent. Negative and not 0, so `posse gates` prints "absent" rather than
// claiming a version nobody read.
func liveWallUnixABI() int { return -1 }

// restrictResolveUnix cannot be reached: RunWallUnix runs inside a bwrap
// cage, and nothing off Linux has one. It refuses rather than returning nil,
// for the reason RunWallUnix itself refuses — a nil here would be a wall
// reported and not applied, which is the one failure direction ADR 0080 D4
// forbids.
func restrictResolveUnix() error {
	return Die("%s: there is no Landlock on this operating system, so this seat's socket wall cannot be applied — and this step should never have been typed, because the wall mechanism here is not %s (ADR 0080 D16)", WallUnixVerb, WallBwrap)
}
