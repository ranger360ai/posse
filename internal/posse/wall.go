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
// host says it is. Before ADR 0080 D18 (ranger-base-djl3c) both of
// `wallRendered`'s call sites answered the first two questions and then
// typed `SeatbeltPrefix` — `sandbox-exec` and nothing else — so the table
// fill reads all three (seatbeltTierProvidedOn) and a Linux host with bwrap
// and a passing probe was REFUSED at parity.go's "cage %s is not available
// on this host/build" rather than handed a seatbelt row over a binary it
// does not have. D18 TYPED it: `RenderWall` below is the one place the
// mechanism is chosen, `WallPrefix` is what goes on the line, and
// wallMechanismRendered now names WallBwrap too. Three questions and not
// two, because collapsing this one into availability would make the apply
// probe answer for a render, and collapsing it into the NAME would print
// bare `seatbelt` on a Linux host whose kernel refuses the namespace — the
// exact reading the paragraph above keeps apart. The question stays asked:
// a GOOS whose mechanism posse cannot render still claims no tier, which is
// what keeps the next mechanism from shipping as a name alone.

import (
	"io"
	"os"
	"runtime"
	"strconv"
)

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
func WallMechanism() string { return wallMechanismOn(wallMechanismHost) }

// wallMechanismHost is the GOOS WallMechanism answers for. `runtime.GOOS`,
// and a var for the reason bwrapResolves, bwrapApplyRefusal and bwrapBin are
// vars (bwrap.go): the arm it selects is unreachable on the box that builds
// it, so a pin answers it to drive the other host's render from here.
//
// IT IS THE FOURTH SUCH VAR AND THE ONE THE OTHER THREE NEEDED. Those three
// make the bwrap RENDER drivable from darwin, and `renderWallOn`'s mechanism
// parameter makes the switch drivable — but a LAUNCH asks WallMechanism(),
// so no pin could put a launch behind the other mechanism, and the launch
// paths are where the two mechanisms differ in what they REFUSE: ADR 0080
// D19 refuses a claude seat here that darwin walls without comment. That gap
// is how ranger-base-djl3c landed `make test` green on darwin and red on
// every ubuntu arm of ci.yml — seven tests, three arms, main's only gate
// (ranger-base-temhf). TestQALaunchAtTheOtherHostsWallMechanism is what the
// var is for; production never writes it, and a write in production would be
// a seat walled by one mechanism and reported under another.
var wallMechanismHost = runtime.GOOS

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
// BOTH MECHANISMS SINCE ADR 0080 D18 (ranger-base-djl3c), and the membership
// of this set is the membership of RenderWall's switch below: sandbox-exec
// renders a profile and types `sandbox-exec -f <prof>`, bwrap writes
// `bwrap.args` and types `3<args <abs bwrap> --args 3 --`. The two changed
// together in one commit, which is the invariant
// TestQAWallTableClaimsNoTierThisBuildCannotType holds — a mechanism named
// here with no arm in RenderWall is a tier posse announces and cannot put
// up, and an arm here with the name withheld is a wall posse renders and
// refuses to launch behind.
//
// IT IS STILL A SEPARATE QUESTION from availability and from the name, and
// the reason it stays one outlives bwrap: `RenderWall` refuses the mechanism
// it has no arm for, and this is what keeps that refusal from being the
// place a host first hears about it. A third mechanism — a Landlock
// ruleset, a FreeBSD jail — is a name in wallMechanismOn and an arm here,
// and until it has both the tier is simply not offered there.
func wallMechanismRendered(m string) bool {
	return m == WallSandboxExec || m == WallBwrap
}

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

// ─── D18: TYPING THE WALL ───────────────────────────────────────────────────

// WallRender is one launch's rendered L2 wall: which mechanism put it up,
// the artifact posse wrote that an operator can go and read, and the prefix
// that goes between the PATH assignment and the runtime command.
//
// ONE STRUCT SO THE THREE CANNOT DISAGREE. The prefix names the artifact —
// `-f <prof>` on one mechanism, `3<<args>` on the other — so a caller that
// rendered one and reported the other would print a wall nobody is behind.
// Parity's reachability probe is exactly such a caller: its row names the
// artifact it judged (reachability.go), and it judges by RUNNING the prefix.
type WallRender struct {
	// Mechanism is WallSandboxExec or WallBwrap, never "": RenderWall
	// refuses a GOOS with no arm rather than returning an empty wall.
	Mechanism string
	// Artifact is the file the prefix reads — the rendered `seatbelt.sb`,
	// or the NUL-separated `bwrap.args`. It is what RUNS, which is why
	// bwrap's readable twin (`bwrap.txt`) is not it.
	Artifact string
	// Prefix is typed verbatim in front of the runtime command, trailing
	// space included, and is what every reader of this wall should run
	// rather than recompose.
	Prefix string
}

// RenderWall renders this launch's L2 wall and returns what posse wrote and
// what it will type. THE ONE PLACE THE MECHANISM IS CHOSEN (ADR 0080 D18):
// every caller that used to render a profile and type `SeatbeltPrefix`
// itself asks this instead, so there is a single switch between the tier's
// two mechanisms and no call site that can answer it differently.
//
// It is the third of D16's three questions made concrete — see the file
// comment. `wallRendered` (seatbelt.go) decides WHETHER a launch gets a wall
// and this decides WHAT it gets; the two are kept apart because the first is
// a predicate about the tier and the runtime, and this one writes files and
// can fail.
//
// IT WRITES, AND SO DID WHAT IT REPLACES. `RenderSeatbelt` has written the
// `.sb` at this point in a launch since the tier existed; the bwrap arm
// writes two files and also makes the host-side prep the namespace cannot
// (RenderBwrap, bwrap.go). Callers that only want to READ the wall — a
// report, a probe — still come through here, because a wall judged from
// anything but the artifact that runs is the class reachability.go's own
// file comment refuses.
//
// A MECHANISM WITH NO ARM REFUSES, naming it and the host. Unreachable from
// a launch — `wallRendered` requires AvailableCages[CageSeatbelt], which
// seatbeltTierProvidedOn only fills for a mechanism wallMechanismRendered
// admits, which is this switch's own membership — and spelled anyway,
// because the alternative return is an empty prefix, which is a launch with
// no wall and nothing said.
func (a *App) RenderWall(ag *AgentFile, rt *Runtime, cwd string) (WallRender, error) {
	return a.renderWallOn(WallMechanism(), ag, rt, cwd)
}

// renderWallOn is that render for a NAMED mechanism, so a pin can drive the
// arm this host does not have — the rule every host-shaped decision in ADR
// 0080 follows (wallMechanismOn, seatbeltTierAvailableOn,
// credentialReadDenyLiterals). It is the whole reason the bwrap arm is
// readable from darwin at all: the render is pure of the host except for the
// tree it stats, so a fixture tree is a fixture seat.
//
// The mechanism is a PARAMETER and never a preference. Production has
// exactly one caller and it passes WallMechanism(); a second production
// caller choosing its own would be a seat walled by one mechanism and
// reported under another.
func (a *App) renderWallOn(mech string, ag *AgentFile, rt *Runtime, cwd string) (WallRender, error) {
	switch m := mech; m {
	case WallSandboxExec:
		var stateDirs []string
		if rt != nil {
			stateDirs = rt.StateDirs
		}
		prof, err := a.RenderSeatbelt(ag, cwd, stateDirs...)
		if err != nil {
			return WallRender{}, err
		}
		// SBPL has no env of its own, so the seat's cache env (ranger-base-
		// olr0g) rides in front of sandbox-exec as `/usr/bin/env K=V …`: an
		// executable, so it reads the same after an `exec` or an assignment
		// list as sandbox-exec itself did. bwrap's arm sets it in the argv.
		home, _ := os.UserHomeDir()
		return WallRender{Mechanism: m, Artifact: prof, Prefix: seatCacheEnvPrefix(seatCacheEnv(SeatCacheDir(home, cwd))) + SeatbeltPrefix(prof)}, nil
	case WallBwrap:
		args, err := a.RenderBwrap(ag, rt, cwd)
		if err != nil {
			return WallRender{}, err
		}
		bin, err := bwrapBin()
		if err != nil {
			return WallRender{}, err
		}
		return WallRender{Mechanism: m, Artifact: args, Prefix: BwrapPrefix(bin, args)}, nil
	}
	return WallRender{}, Die("cage %s: posse has no render for wall mechanism %q (this host is %s, and posse renders %s and %s and nothing else), so there is no L2 write wall to type here (ADR 0080 D16/D18)",
		CageSeatbelt, mech, runtime.GOOS, WallSandboxExec, WallBwrap)
}

// WallPrefix is what the two launch call sites want out of RenderWall: the
// prefix, with the wall rendered as a side effect. It replaced the
// `RenderSeatbelt` + `SeatbeltPrefix` pair that both of wallRendered's call
// sites spelled inline (herdrback.go, the create and the relaunch path).
//
// ONE FUNCTION FOR THE SAME REASON `wallRendered` IS ONE PREDICATE: spelled
// twice, the second spelling is a copy that can drift, and the drift is
// silent in the direction that matters — a launch line typing a binary for
// the wall it did not render.
func (a *App) WallPrefix(ag *AgentFile, rt *Runtime, cwd string) (string, error) {
	w, err := a.RenderWall(ag, rt, cwd)
	if err != nil {
		return "", err
	}
	return w.Prefix, nil
}

// WallReport prints the wall `posse gates <persona>` reads, for whichever
// mechanism realizes the tier here. On darwin it IS SeatbeltReport, byte for
// byte; on Linux it is BwrapReport, which prints the same markers over the
// same lists (ADR 0080 D18).
//
// The runtime and not its state dirs, because the bwrap arm needs the
// runtime itself: claude's state grant at that mechanism is the directory
// its CLAUDE_CONFIG_DIR names and not its declared `state_dir:`
// (BwrapStateDirs, D19), and that substitution is keyed on rt.Name.
func (a *App) WallReport(ag *AgentFile, rt *Runtime, cwd string, out io.Writer) error {
	if WallMechanism() == WallBwrap {
		return a.BwrapReport(ag, rt, cwd, out)
	}
	var stateDirs []string
	if rt != nil {
		stateDirs = rt.StateDirs
	}
	return a.SeatbeltReport(ag, cwd, out, stateDirs...)
}

// wallApplyRefusal is "can THIS PROCESS put up the wall it is about to type",
// for whichever mechanism realizes the tier here: "" when it was measured
// able to, else the kernel's own words. The third of D16's three questions
// asked about the PROCESS rather than about the host — a posse running inside
// a caged session can type the prefix and cannot apply it, and a probe that
// judges a wall by running it has to know which of those it just measured
// (ADR 0013 §4, ranger-base-heur).
//
// Mechanism-keyed through renderWallOn's own parameter shape rather than
// through GOOS, so the arm this host does not have is drivable from the other
// one (wallMechanismOn, seatbeltTierAvailableOn, credentialReadDenyLiterals).
// Both halves are memoized for the process's lifetime by their owners
// (sandboxApplyRefusal in seatbelt.go, bwrapApplyRefusal in bwrap.go), so
// asking is free after the first time.
func wallApplyRefusal() string { return wallApplyRefusalOn(WallMechanism()) }

func wallApplyRefusalOn(mech string) string {
	switch mech {
	case WallSandboxExec:
		return sandboxApplyRefusal()
	case WallBwrap:
		return bwrapApplyRefusal()
	}
	// The same refusal RenderWall gives, as a sentence a row can print: a
	// mechanism with no arm is a wall nothing puts up, so nothing can be
	// measured about it either.
	return "this build renders no wall for mechanism " + strconv.Quote(mech) + " (" + runtime.GOOS + "), so there is nothing to apply (ADR 0080 D16/D18)"
}
