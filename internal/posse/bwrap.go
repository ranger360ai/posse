package posse

// Gates L2 on Linux — the seatbelt tier's bwrap mechanism (ADR 0080 D16).
// This file holds the availability half: whether this host can put the wall
// up at all. The render is D17/D18's and is not here yet.
//
// bubblewrap is the Linux answer to the same question sandbox-exec answers
// on darwin, and D14 picked it off a measured host probe (ranger-base-hft2q):
// one setuid-or-userns binary, no daemon, no image, and a mount namespace
// per seat that an adversarial process inside cannot unmount. The lists it
// renders from are the ones seatbelt.go already produces — SeatbeltWritable,
// SeatbeltCarveOut, credentialReadDenyLiterals, sessionRefDirs are the same
// for both mechanisms. Only the last step, list to wall, differs.

import (
	"os/exec"
	"strings"
	"sync"
)

// bwrapResolves and bwrapApplyRefusal are the two host measurements the
// availability answer reads, as variables so a pin can drive the Linux
// branch from darwin and both of the probe's answers on a host that only
// gives one (seatbelt.go's sandboxApplyRefusal, for that reason exactly).
var (
	bwrapResolves     = liveBwrapResolves
	bwrapApplyRefusal = liveBwrapApplyRefusal
)

// liveBwrapResolves asks the GATES-FREE PATH, which is where this mechanism
// parts company with SeatbeltAvailable (whose answer is deliberately the
// gated one, ranger-base-npqem).
//
// The rule is the same in both places — a probe asks the question the
// resolution asks — and the resolutions differ. `sandbox-exec` is typed BARE
// into a herdr pane, so it is resolved in herdr's process tree carrying the
// seat's PATH, and the gated lookup is the one that matches. D18 types
// `bwrap` as "the absolute path resolved at render, not bare, so a mise shim
// or a gates-dir shim cannot stand in for it" — and that render runs in
// THIS process. So the PATH this answer must agree with is posse's own
// gates-free one, which makes it protocolAvailable's question exactly
// (ranger-base-c33j6, -g1xnt, -d0nyb are the probes that asked the other
// one). The probe below execs bwrap in this process too, so both halves of
// the availability answer and the render all resolve the same file.
func liveBwrapResolves() bool { return protocolAvailable("bwrap") }

// bwrapAvailableOn reports whether this host can realize the seatbelt tier
// with bwrap: the binary resolves AND the apply probe passed. Both, because
// either alone is a wall posse would announce and not have — a kernel or LSM
// that refuses unprivileged user namespaces (linux-hardened, Ubuntu's
// AppArmor sysctl) leaves the binary exactly where it was and refuses the
// mount.
//
// Short-circuited left to right, so neither measurement is taken off Linux:
// the probe forks, and a darwin box would pay for it on every process start.
func bwrapAvailableOn(goos string) bool {
	return goos == "linux" && bwrapResolves() && bwrapApplyRefusal() == ""
}

// bwrapApplyProbeArgv is what the probe applies: D17's base mount and
// nothing else — a read-only bind of / over itself, and /bin/true inside it.
// The child is constrained in no way that matters and the only thing
// measured is whether the kernel let us make the namespace at all.
//
// Shaped like the thing it predicts, which is the lesson sandbox-exec's
// probe grid taught (ranger-base-xjw9): a lenient probe for a wall that is
// never lenient reports "walled" on a host that refuses every profile posse
// emits. `--ro-bind / /` is the one mount every render of D17 begins with,
// so a host that refuses this refuses all of them.
var bwrapApplyProbeArgv = []string{"--ro-bind", "/", "/", "/bin/true"}

var (
	bwrapApplyOnce sync.Once
	bwrapApplyWhy  string
)

// liveBwrapApplyRefusal asks the kernel, once per process. The answer can
// only change by this box gaining or losing a sysctl, which is not a thing
// that happens inside one posse command, and the alternative is a fork per
// reader of AvailableCages.
//
// The memo and the asking are two functions so that a pin can drive the ask
// directly: by the time any test calls this one, whatever called it first in
// the binary has already cached the box's own answer, and a sync.Once cannot
// be re-armed (ranger-base-npqem's lesson, same shape).
func liveBwrapApplyRefusal() string {
	bwrapApplyOnce.Do(func() { bwrapApplyWhy = bwrapApplyProbe() })
	return bwrapApplyWhy
}

// bwrapApplyProbe is that ask, unmemoized: "" when a mount namespace was
// MEASURED here, and otherwise the sentence saying why that was not
// established.
//
// EVERY FAILURE IS A REFUSAL HERE, and that is the one place this probe
// differs from sandboxApplyProbe rather than twinning it. There, "" is read
// as "this process may apply a profile" — a PERMISSION — so a failure that
// is not an apply refusal has to fall through to "" and be reported by
// whatever check met it. Here "" is read as "this host provides the tier",
// so the safe failure direction is the other one: a missing /bin/true, a
// bwrap too old for these flags and a seccomp refusal are all answers posse
// must not turn into a wall it claims and does not have. "Could not ask" is
// not "the wall is up".
//
// It resolves its own binary gates-free before running it (ADR 0077 D3),
// because this exec IS one of posse's own children. Without that,
// protocolCmd puts a protocolBin refusal on cmd.Err, CombinedOutput comes
// back with an empty out having forked nothing, and the reason the host is
// unavailable would read as the kernel's words when no kernel was asked.
func bwrapApplyProbe() string {
	if _, err := protocolBin("bwrap"); err != nil {
		return "could not ask: " + err.Error()
	}
	out, err := protocolCmd(exec.Command("bwrap", bwrapApplyProbeArgv...)).CombinedOutput()
	if err == nil {
		return ""
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		return s
	}
	return err.Error()
}
