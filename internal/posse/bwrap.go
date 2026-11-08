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
//
// TWO HALVES, and the second one starts at BwrapArgs below. The availability
// half asks whether this host can put the wall up at all; the render half
// (D17, ranger-base-qoamy) is the pure function from those lists to the argv
// that puts it up. Writing that argv to a file and typing it onto a pane is
// D18's and is not here yet: nothing in this file writes anything, which is
// what lets every shape be pinned by a table on darwin.

import (
	"os"
	"os/exec"
	"sort"
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

// ─── D17: THE RENDER ────────────────────────────────────────────────────────

// PrepKind names what the LAUNCHER does to a path before it execs bwrap,
// spelled as the shell verb an operator would type. These print — parity and
// `posse gates` read them back out of a render nobody launched yet — and a
// step an operator cannot reproduce by hand is a step nobody can check.
type PrepKind string

const (
	// PrepMkdirAll makes the path, and every missing parent, as a directory.
	PrepMkdirAll PrepKind = "mkdir -p"
	// PrepTouch creates the path as an empty FILE.
	PrepTouch PrepKind = "touch"
)

// PrepStep is one of those actions beside the reason the render asked for
// it. The pair travels together for pidDeny's reason: a path with no reason
// beside it is a line an operator cannot check against the design, and this
// list is the one part of the wall that happens OUTSIDE it — the launcher is
// not in the namespace, so every step here is a write on the host made in
// the seat's name.
//
// IT IS AN ORDERED LIST AND NOT A SET. BwrapArgs emits the steps in the
// order the shapes render, so a grant's mkdir always precedes the touch of a
// Keep file inside it; a launcher that reorders them, or runs them
// concurrently, breaks that and nothing says so.
type PrepStep struct {
	Kind PrepKind
	Path string
	Why  string
}

// String renders a step as the command it is.
func (s PrepStep) String() string { return string(s.Kind) + " " + s.Path }

// BwrapArgs is the pure render of ADR 0080 D17: the L2 lists that
// seatbelt.go already computes, turned into the bwrap argv that realizes
// them, plus the prep the launcher must do on the host first.
//
// SeatbeltProfile's twin, and deliberately shaped like it — same inputs,
// same no-side-effects rule, one call per launch — so that the two
// mechanisms of one tier (WallMechanism, wall.go) cannot drift into
// answering different questions. What it does NOT take is the two arguments
// SeatbeltProfile has and bwrap has no spelling for:
//
//   - siblings (SeatbeltSiblings). An SBPL regex grants the scratch names
//     beside a granted FILE; a mount cannot name a namespace. It needs none:
//     D19 moves claude's config to the whole-dir grant its CLAUDE_CONFIG_DIR
//     already points at, and a directory grant covers every sibling name the
//     CLI ever invents.
//   - createOnly (sessionRefDirs). A mount cannot say "making this path is
//     allowed, writing inside it is not". D20 replaces that grant with a
//     bind of the ref's PARENT directory plus a keep file.
//
// THE ORDER IS THE SEMANTICS. bwrap applies mounts in argv order and a later
// mount covers an earlier one, so this is not a rendering preference the way
// SBPL's block order is: moving two lines changes which wall the seat gets.
// Base first, then the write grants, then the denies over them, then the
// Keep files back over the denies, then the read masks.
//
// IT IS PURE AND IT STILL STATS THE TREE. "Pure" here is the SeatbeltProfile
// sense — it writes nothing, forks nothing and memoizes nothing — and it is
// not the stronger sense, because three of the five shapes cannot be
// rendered without asking the filesystem a question: bwrap can only bind a
// source that EXISTS, and seatbelt granted paths that did not
// (resolveExisting). So every path that is not there is either a prep step
// or a mount that is not rendered, and which one is a fact about the box.
// That is what the fixture trees in the table tests are for.
//
// EVERY PATH ARRIVES RESOLVED and nothing here re-resolves one, same as
// SeatbeltProfile: SeatbeltWritable and SeatbeltCarveOut absResolve every
// element they emit, and a renderer that resolved a second time would be a
// second place for the spelling of a grant to be decided. cwd is the one
// exception — it is the raw session dir every caller passes around, so
// --chdir resolves it here, and inside the namespace the real path is the
// one that exists.
//
// "COULD NOT ASK" IS NOT AN ANSWER, in either direction: a stat that fails
// for any reason other than "not there" refuses the whole render naming the
// path, rather than guessing the shape of a path the wall is about to be
// built out of. Same safe failure direction as bwrapApplyProbe above, and
// for the same reason — the value of this answer is that a seat runs behind
// it.
func BwrapArgs(cwd string, writable []string, carve SeatbeltCarveOut) ([]string, []PrepStep, error) {
	// 1. The base. --die-with-parent so a seat's wall dies with the pane
	// that holds it; --ro-bind / / because the default is to deny writes
	// everywhere, which at this mechanism is spelled as a read-only mount of
	// the whole tree; --dev-bind /dev /dev for seatbelt's own
	// (regex #"^/dev/") grant, which every runtime needs for its tty.
	//
	// What is deliberately absent, each because the seat needs the host's
	// own (D17, and Alternatives rejected): --unshare-pid (a seat kills its
	// own pids and `go run ./cmd/checkorphans` reads the box's process
	// table), --unshare-net (egress is the container tier's), --proc,
	// --tmpfs /tmp (the multiplexer socket and the shared TMPDIR paths live
	// there), --new-session (setsid would detach the TUI from its
	// controlling terminal).
	argv := []string{
		"--die-with-parent",
		"--ro-bind", "/", "/",
		"--dev-bind", "/dev", "/dev",
	}
	// A render with no session dir — `posse gates` asked about a persona and
	// not a launch — omits the flag rather than passing it an empty string,
	// which bwrap would read as a chdir to "" and refuse.
	if cwd != "" {
		argv = append(argv, "--chdir", absResolve(cwd))
	}
	var prep []PrepStep

	// 2. Shape 1: the write grants, shallowest first so a nested grant's
	// mount lands on top of the one that contains it.
	grants := shallowestFirst(writable)
	for _, p := range grants {
		exists, dir, err := bwrapStat(p)
		if err != nil {
			return nil, nil, bwrapStatRefusal("write grant", p, err)
		}
		switch {
		case exists && !dir:
			// B8, MEASURED on the host probe: an atomic replace of a BOUND
			// file is refused — the rename cannot cross the mount point — so
			// a writer that replaces its file instead of appending to it
			// loses every write and says nothing. That is the ranger-base-cypy1
			// failure with the grant in place, which is worse than no grant,
			// so the render refuses instead of rendering a wall with a note
			// beside it.
			//
			// The two entries in today's writable set that reach here are
			// the two D19 and D20 remove upstream: claude's ~/.claude.json
			// (D19 folds it into the CLAUDE_CONFIG_DIR grant) and git's ref
			// leaf, its .lock sibling and packed-refs.lock (D20 binds the
			// ref's parent directory instead). This error is what makes
			// those two beads prerequisites of a launch rather than
			// improvements to one.
			return nil, nil, Die("bwrap cannot grant write on %s: it is not a directory, and binding a file refuses the atomic replace its writer makes (ADR 0080 D17, probe B8). Narrow the writable set — claude's config file is ADR 0080 D19's and git's ref pair is D20's — rather than widening the wall", p)
		case !exists:
			// seatbelt granted paths that did not exist yet and bwrap
			// cannot bind a source that is not there, so the launcher makes
			// it — outside the namespace, before the wall goes up.
			//
			// AS A DIRECTORY, because a path that is not there cannot be
			// told apart from a file (SeatbeltSiblings says the same thing
			// about the same question), and a directory is the grant the
			// tier means: every path in this set is one a session writes
			// INTO. A grant that is going to be a file is D19's to remove,
			// and until it is, a box that has never run the CLI gets a
			// directory where its config was going to go.
			prep = append(prep, PrepStep{PrepMkdirAll, p, "a write grant that is not there yet; bwrap cannot bind a source that does not exist"})
		}
		argv = append(argv, "--bind", p, p)
	}

	// 3. Shape 2: the carve-out's denies, read-only over the grants above
	// them. This is last-match-wins at the other mechanism: SBPL puts the
	// deny block after the allow block, bwrap puts the ro-bind after the
	// bind, and either way the narrow statement is the later one.
	//
	// MNT_LOCKED is what makes it a wall rather than a suggestion: B6 and
	// B6b MEASURED that a nested user namespace can neither unmount the
	// read-only bind nor remount it rw.
	for _, d := range shallowestFirst(carve.Deny) {
		exists, _, err := bwrapStat(d)
		if err != nil {
			return nil, nil, bwrapStatRefusal("deny", d, err)
		}
		if !exists {
			// A deny on a path that is not there is not nothing: if the
			// session can CREATE it, an unmounted name is a hole that opens
			// the first time it does. So the launcher makes it as an empty
			// directory and the mount goes over that.
			//
			// DIVERGED from D17's letter, which asks whether "its nearest
			// existing ancestor is inside a write grant", and kept to D17's
			// stated reason, which is "otherwise nothing can create it". The
			// two disagree on exactly one shape, and it is a shape this
			// fleet has: a write grant that does not exist yet. On a fresh
			// home a.StateDir is a grant and a.StateDir/gates is a deny, and
			// the deny's nearest EXISTING ancestor is then the home, which
			// no grant covers — so the letter renders no mount while the
			// grant one line above is about to be created and bound rw, and
			// the session can make state/gates and write in it. Asking
			// whether the deny is inside a write grant answers the reason
			// directly, is never narrower (a grant covers an ancestor only
			// by covering the descendant too), and covers the prep-created
			// grants as well.
			if !writeGranted(grants, d) {
				continue // nothing inside the wall can bring this path into being
			}
			prep = append(prep, PrepStep{PrepMkdirAll, d, "a deny that is not there yet and sits inside a write grant, so the session could create it"})
		}
		argv = append(argv, "--ro-bind", d, d)
	}

	// Seal RENDERS NOTHING, and that is a decision with two measurements
	// under it rather than an omission (B3 and B4, both halves). At L2 the
	// seal exists because a subpath deny is a statement about a PATH, so
	// `mv rhq rhq2` carries the denied tree out from under its own deny.
	// A mount is a statement about a mount POINT: renaming a mountpoint is
	// refused, and so is renaming an ancestor to carry it away. The wall is
	// in the kernel's mount table, not in the profile's spelling of a path,
	// so there is nothing for a rename to walk around.
	//
	// Said here, where a reader looking for the fourth list will look,
	// because "the renderer ignores one of its four inputs" is otherwise
	// indistinguishable from a renderer that forgot it.

	// 4. Keep: the session's own audit trail, bound back rw over the deny
	// that closed the gates dir. --bind and not --ro-bind, and a FILE: an
	// append to a bound file is an in-place write, which is the one thing
	// B8 does not touch (B8 is a rename).
	for _, f := range shallowestFirst(carve.Keep) {
		exists, dir, err := bwrapStat(f)
		if err != nil {
			return nil, nil, bwrapStatRefusal("keep", f, err)
		}
		switch {
		case exists && dir:
			// A Keep entry is a literal re-allow inside a deny, and at L2 a
			// literal names one file. A DIRECTORY at that name would bind a
			// whole tree back open inside the deny the carve-out just
			// closed, which is a wider wall than anything asked for — so
			// refuse, in the tier's own fail-closed direction, rather than
			// render a widening nobody can read off the argv.
			return nil, nil, Die("bwrap cannot re-allow %s inside the deny above it: it is a directory, and binding it would hand the session back the tree that deny closed (ADR 0080 D17; the Keep list is the two audit-log FILES of SeatbeltCarveOut)", f)
		case !exists:
			// Under sandbox-exec these two are created on first append from
			// INSIDE the cage — a literal allow under a denied subpath
			// creates and appends (measured, SeatbeltCarveOut). A bind has
			// no such move: the source has to be there, so the launcher
			// makes it.
			prep = append(prep, PrepStep{PrepTouch, f, "the session's own audit log, which must exist before it can be bound back over the deny"})
		}
		argv = append(argv, "--bind", f, f)
	}

	// 5. Shape 5: the credential read masks. /dev/null over the file, so a
	// read gets EOF instead of bytes.
	//
	// A MISSING ONE RENDERS NOTHING, and the residual is stated rather than
	// closed (Consequences): a credential mask is a snapshot at launch, so
	// another runtime's credential file created AFTER the seat started is
	// readable by it. Parity names each missing file. The seat still cannot
	// write there, because nothing grants that runtime's state dir.
	//
	// B5 measured that no byte of the credential reached the seat, and
	// measured it by an error rather than by the clean EOF this shape is
	// stated in terms of; ranger-base-yca4g is the re-run that reads which
	// error. Nothing here depends on which it is — the claim this render
	// makes is "the read gets no bytes".
	for _, f := range shallowestFirst(carve.DenyRead) {
		exists, _, err := bwrapStat(f)
		if err != nil {
			return nil, nil, bwrapStatRefusal("read deny", f, err)
		}
		if !exists {
			continue
		}
		argv = append(argv, "--ro-bind", "/dev/null", f)
	}
	return argv, prep, nil
}

// bwrapStat is the one filesystem question the shapes above ask, with "could
// not ask" kept apart from "not there" — the distinction the render's safe
// failure direction is built on, and the one os.Stat's single error return
// collapses at every call site that only tests os.IsNotExist.
func bwrapStat(p string) (exists, dir bool, err error) {
	fi, err := os.Stat(p)
	switch {
	case err == nil:
		return true, fi.IsDir(), nil
	case os.IsNotExist(err):
		return false, false, nil
	}
	return false, false, err
}

// bwrapStatRefusal is that refusal, naming the shape as well as the path: a
// path posse cannot stat is a box problem, and which LIST it came from is
// what tells an operator whether the fix is a PID, a home or a mount.
func bwrapStatRefusal(shape, p string, err error) error {
	return Die("bwrap render: cannot read the %s path %s: %v — a wall is not rendered over a path posse could not ask about (ADR 0080 D17)", shape, p, err)
}

// shallowestFirst is the mount order D17 fixes, and the whole of what it has
// to buy: bwrap applies mounts in argv order and a later mount covers an
// earlier one, so for any two paths where one CONTAINS the other, the
// container must be mounted first or the nested mount is buried under it.
//
// A plain lexicographic sort of cleaned absolute paths gives exactly that —
// an ancestor is a prefix of its descendants, so it always sorts before them
// — and gives it without a depth count, which would leave unrelated paths of
// equal depth in whatever order the producer happened to append them. Two
// renders of one launch have to be the same argv, because D18 writes it to a
// file an operator reads and a relaunch re-renders.
//
// Dedupe first, for the same reason SeatbeltProfile skips an empty entry:
// this is a renderer, and a duplicated grant is a duplicated mount that
// changes nothing and is read as two decisions.
func shallowestFirst(in []string) []string {
	out := dedupeStrings(in)
	sort.Strings(out)
	return out
}
