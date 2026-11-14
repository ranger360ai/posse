package posse

// The seat's SOCKET wall: a Landlock domain handling
// LANDLOCK_ACCESS_FS_RESOLVE_UNIX and no other right, applied inside the
// bwrap cage immediately before the runtime is exec'd (ADR 0080, amendment
// "RESOLVE_UNIX closes the connect hole", ranger-base-6rndn; the finding is
// ranger-base-l2gk9).
//
// WHY A SECOND MECHANISM AT ALL, when ADR 0080 D13 rejected Landlock. D13
// rejected it as a REPLACEMENT for the mount namespace, and every word of
// that holds: Landlock is allow-only, so the trailing deny under a grant —
// the whole of what the wall defends — has no Landlock spelling. This is not
// that. The file wall stays the bwrap wall, byte for byte; this layer handles
// exactly one right, and it is a right no mount can express.
//
// THE RIGHT NO MOUNT CAN EXPRESS. `--ro-bind / /` does not stop connect(2) on
// a socket: the kernel's read-only check does not cover it (bwrapArgsInto 1b
// already says so, and that is why the runtime dir gets a tmpfs). A tmpfs
// over a DIRECTORY does hold, and holds across the host's unlink-and-re-bind
// (MEASURED, N4) — so every directory a seat can afford to have emptied is
// already answered by ludpp's hide, and this layer is for the residual that
// is not: a socket inside a WRITE GRANT, which the seat must be able to write
// and therefore cannot have emptied. ranger-base-l2gk9 measured both halves
// of the mount answer failing there — a `--ro-bind /dev/null` over a live
// socket is detached by the host's own unlink, and a socket absent at launch
// has nothing to bind over at all.
//
// MEASURED 2026-11-13, D1 host, Linux 7.2.15-1-omarchy, Landlock ABI 10,
// linux-api-headers 7.3-1 (docs/adr/0080-resolve-unix.probe.sh, every row as
// its EXPECT): handled with no allow rule, connect(2) is EACCES to a socket
// bound before restrict_self (U1) AND to one an unrestricted process binds
// after (U2) — the case nothing else covers; sendmsg(2) with an explicit
// address likewise (U4); the domain survives execve (U5); a socket created
// INSIDE the domain still connects (U3); and ordinary file read, write,
// mkdir and rename are untouched (U11), which is what "the bwrap wall stays
// the file wall" means operationally.
//
// WHAT IT COSTS, AND WHY THAT COST IS ALREADY PAID. A domain handling any
// filesystem right — this one included — forbids mount(2), so a nested user
// plus mount namespace cannot start inside it (U9, U10). That is ADR 0080
// D14's probe B7, "a runtime's own bwrap must still nest inside ours", which
// D13 names as the property Landlock fails. It fails here too, and it costs
// nothing, because B7 does not hold for the wall posse SHIPS: the render
// deliberately omits `--proc` (bwrapArgsInto 1.), so /proc inside a seat is
// the host's, bound read-only by `--ro-bind / /`, and a nested bwrap dies
// writing /proc/self/uid_map long before it reaches a mount. MEASURED
// 2026-11-13 against the rendered args file and against the installed wall of
// a live caged seat: `bwrap: setting up uid map: Read-only file system`, with
// no Landlock anywhere. B7 was measured on a wall carrying `--proc /proc`,
// which is not this one. ranger-base-6rndn's handoff carries that reading;
// nothing here depends on how it is decided, because this layer adds no
// refusal that the shipped wall does not already impose.
//
// NO ALLOW RULE, BECAUSE THE SEAT NEEDS NO HOST SOCKET. MEASURED 2026-11-13
// on D1: of the host's pathname listeners, every one a seat could otherwise
// resolve is either inside the runtime dir the render already hides as a
// tmpfs (the session bus, the gpg and ssh agents, the keyring, p11-kit,
// pulse, pipewire, the user systemd, rex's own server.sock, and the other
// seats' sockets) or belongs to the system and is of no use to a seat
// (/run/systemd/*, /run/dbus, /run/udev, /run/polkit, /run/cups,
// /run/avahi-daemon, /run/tailscale, /run/wpa_supplicant,
// /etc/pacman.d/gnupg). No socket exists anywhere under the home, and
// ranger-base-ludpp had already measured the same list from the other side:
// what a seat needs bound back out of the hidden runtime dir is nothing. The
// one host socket posse ever hands a session is herdr's, and that is the
// container tier's opt-in `sockets:` (CageSocketMounts) which no shipped PID
// declares.
//
// IF AN ALLOW RULE IS EVER NEEDED IT NAMES A DIRECTORY, NEVER A SOCKET.
// MEASURED (U12, U13): a rule on the socket FILE is defeated by exactly the
// move that defeated the mount hide — the host unlinks and re-binds, and the
// rule, which is keyed on the inode it was added for, no longer covers the
// new one (EACCES, so a daemon restart would silently cut the seat off); a
// rule on the DIRECTORY covers a socket bound in it afterwards. A directory
// rule is also the only way to allow a socket that is ABSENT at launch,
// since an O_PATH open of it would be ENOENT and there would be no rule to
// add.

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// WallUnixVerb is argv[1] of the inner step. An entry point and not a catalog
// verb, for WallWatchVerb's reason exactly: the launch line runs it, nobody
// types it, and it reads no config.
const WallUnixVerb = "wall-unix"

// IsWallUnix is how main.go routes to the step: argv[1], as every other
// entry point of this kind is read (IsWallWatch, IsCageLaunch).
func IsWallUnix(argv []string) bool { return len(argv) > 1 && argv[1] == WallUnixVerb }

// WallUnixABIMin is the Landlock ABI that added RESOLVE_UNIX. Below it there
// is no such right to handle, and a ruleset naming it is refused outright by
// the kernel rather than quietly handling nothing — which is the only reason
// a version number appears in posse at all instead of a capability probe.
const WallUnixABIMin = 9

// wallUnixABI is the host's Landlock ABI, or a negative number when the
// syscall is not there at all. A var for the reason bwrapResolves,
// bwrapApplyRefusal and wallMechanismHost are vars (bwrap.go, wall.go): the
// arm it selects is unreachable on the box that builds it, so a pin answers
// it to drive the Linux render from darwin.
var wallUnixABI = liveWallUnixABI

// WallUnixAvailable reports whether this host can realize the socket wall.
func WallUnixAvailable() bool { return wallUnixAvailableFor(WallMechanism()) }

// wallUnixAvailableFor is that answer for an arbitrary MECHANISM, which is
// how the renderer and the report both ask it — and the reason it is the
// mechanism and not the GOOS is renderWallOn's: the mechanism is a parameter
// and never a preference, so a pin on darwin reads the Linux arm by passing
// it, with the ABI stubbed, and production has one caller passing the one
// mechanism it rendered.
//
// IT IS SCOPED TO THE BWRAP MECHANISM and not to Landlock's own availability:
// the step is a word on a bwrap launch line, and there is no line to put it on
// where the wall is a seatbelt profile. The darwin half is its own bead and
// its own mechanism. Short-circuited left to right, so the ABI is not asked
// where no line could carry the answer.
func wallUnixAvailableFor(mech string) bool {
	return mech == WallBwrap && wallUnixABI() >= WallUnixABIMin
}

// WallUnixPrefix is the step itself, appended after bwrap's own `--`: the
// absolute path of THIS posse binary, the verb, and `--` closing it. What
// follows is the runtime command the pane would otherwise have handed bwrap.
//
// THE BINARY IS THE ABSOLUTE PATH RESOLVED AT RENDER, for BwrapPrefix's
// reason and through the same resolution: the line runs carrying the seat's
// PATH, which LEADS with the persona's gates dir, so a bare `posse` there is
// a name a shim can answer — and a shim standing here would stand between
// the seat and its own socket wall. It is the same path the wall-watch
// sidecar is typed with (wallWatchBin), because it is the same binary and one
// resolution cannot disagree with itself.
func WallUnixPrefix(self string) string {
	return shellQuote(self) + " " + WallUnixVerb + " -- "
}

// WallUnixDegraded is the sentence `posse gates` and the wall-watch print
// when the wall is a bwrap wall and this layer is NOT on it — never silence,
// because a host socket reachable from a seat is the l2gk9 class and an
// operator reading a wall should see which of the two halves is up.
//
// Empty when the layer is up, or when the mechanism is not bwrap at all (on
// darwin there is nothing degraded about a seatbelt profile; q9bqu is its
// own bead).
func WallUnixDegraded(mech string) string {
	if mech != WallBwrap || wallUnixAvailableFor(mech) {
		return ""
	}
	return "DEGRADED: no socket wall on this line — this kernel's Landlock ABI is " +
		wallUnixABIText() + " and RESOLVE_UNIX needs " + strconv.Itoa(WallUnixABIMin) +
		", so a process in this seat can connect(2) to any host pathname socket in its view, " +
		"including one an uncaged process binds after launch (ranger-base-l2gk9). " +
		"The file wall is unaffected."
}

// wallUnixABIText is that number as a reader wants it: the version, or the
// plain fact that the syscall is not there, which is a different sentence
// from "version 0".
func wallUnixABIText() string {
	if n := wallUnixABI(); n > 0 {
		return strconv.Itoa(n)
	}
	return "absent (the landlock_create_ruleset syscall is not on this kernel)"
}

// RunWallUnix is the step. It applies the domain to THIS process and then
// becomes the runtime: `syscall.Exec` and not a child, so the seat's pid, its
// controlling terminal and its exit status are the runtime's own, exactly as
// they are when bwrap execs the runtime directly today.
//
// IT REFUSES RATHER THAN EXECS when the domain cannot be applied. The render
// only types this step on a host whose ABI it has already read as sufficient,
// so a failure here is a wall posse promised and does not have — and ADR 0080
// D4's rule is that such a launch refuses rather than opening a seat that
// reads as walled. A refusal costs one seat; a silent exec would put every
// host socket back in reach with the wall report still saying the layer is up.
//
// THE RUNTIME IS RESOLVED ON THE SEAT'S OWN PATH — exec.LookPath and
// deliberately NOT protocolBin. The gates dir LEADS that PATH, and the L1
// shims there are aimed at the runtime process itself (herdrback.go's gate
// prefix): resolving gates-free here would hand the seat the real binary and
// take the shell-verb denies off the one process they most have to cover.
// This step is a wrapper, not a launcher, so it resolves what bwrap's own
// execvp would have resolved.
func RunWallUnix(args []string) error {
	// ONLY WHAT FOLLOWS `--`, and nothing when there is no `--`. Written
	// first as "default to args", which on a line with no `--` made the
	// step's own argv its command: it restricted this process and then
	// exec'd `posse`, so the arm meant to refuse an empty line instead
	// replaced the caller. A wrapper that can mistake its own argv for its
	// payload has no business applying a wall.
	var cmd []string
	for i, a := range args {
		if a == "--" {
			cmd = args[i+1:]
			break
		}
	}
	if len(cmd) == 0 {
		return Die("%s: nothing to exec — the step is `%s -- <command>`, and a launch line that typed it with no command would open a seat with no runtime", WallUnixVerb, WallUnixVerb)
	}
	// Resolved BEFORE the wall goes up, so that every failure this step can
	// detect is reported by an unrestricted process, and the restrict is the
	// last thing that happens before the exec with nothing between them.
	bin, err := exec.LookPath(cmd[0])
	if err != nil {
		return Die("%s: cannot resolve the runtime %q on this seat's PATH (%v) — so this is a seat that will not open, rather than one that opened unwalled", WallUnixVerb, cmd[0], err)
	}
	if err := restrictResolveUnix(); err != nil {
		return Die("%s: this seat's socket wall could not be applied (%v), and posse does not open a seat behind a wall it reported and does not have (ADR 0080 D4). The render typed this step because this kernel's Landlock ABI read as %s; if that has changed under it, `posse gates` says what a launch would render now", WallUnixVerb, err, wallUnixABIText())
	}
	return syscall.Exec(bin, cmd, os.Environ())
}
