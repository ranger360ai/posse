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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
//     bind of the ref's PARENT directory plus a keep file — BwrapWritable
//     below makes that substitution, and refKeep here is the keep file it
//     names.
//
// refKeep IS THE ONE THING THAT RENDERS NO MOUNT (D20, ranger-base-gi1gi).
// Every other argument of this function turns into a `--bind` or a
// `--ro-bind`; a keep file turns into a PrepStep and nothing else. It is a
// file the launcher must leave inside a write grant so that an outside `git
// pack-refs --all --prune` cannot empty and remove the bound directory under
// a live seat — the mount survives the prune, and the seat's next commit then
// writes into a directory that is no longer the one its branch lives in. Not
// carve.Keep, which is the other list of files the launcher creates: a Keep
// entry is a literal re-allow and IS bound, back over the deny above it.
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
func BwrapArgs(cwd string, writable []string, carve SeatbeltCarveOut, refKeep ...string) ([]string, []PrepStep, error) {
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

	// 2b. The keep files, inside the grants above and bound by nothing (D20,
	// ranger-base-gi1gi). Emitted HERE and not with carve.Keep below, because
	// the order of this list is a promise to the launcher (PrepStep): a keep
	// file's directory is one of the grants one loop up, so its mkdir has
	// just been emitted and the touch that follows it cannot run first.
	//
	// OUTSIDE A GRANT IT IS A LIE, so it refuses rather than renders. The
	// file's whole job is to keep a BOUND directory from being pruned; a
	// keep file under no grant names a directory this render never binds, so
	// the prune it is there to survive would take the directory and the seat
	// would be none the wiser — the same shape of silence as a deny that
	// renders no mount.
	for _, f := range shallowestFirst(refKeep) {
		exists, dir, err := bwrapStat(f)
		if err != nil {
			return nil, nil, bwrapStatRefusal("ref keep", f, err)
		}
		if !writeGranted(grants, f) {
			return nil, nil, Die("bwrap cannot keep %s: it is inside no write grant, so this render binds no directory for it to hold open (ADR 0080 D20)", f)
		}
		if exists && dir {
			// A directory at that name is not a keep file: `pack-refs
			// --prune` removes an EMPTY directory, so a directory here is
			// either pruned with its parent or holds something that is not
			// ours — and either way the launcher's touch would fail.
			return nil, nil, Die("bwrap cannot keep %s: it is a directory, and the keep file is an empty FILE whose only job is to make its parent non-empty (ADR 0080 D20)", f)
		}
		if !exists {
			prep = append(prep, PrepStep{PrepTouch, f, "the keep file that stops an outside `git pack-refs --prune` from removing the bound ref directory under a live seat"})
		}
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

// ─── D19: CLAUDE'S STATE GRANT IS ITS CONFIG DIRECTORY ──────────────────────

// BwrapStateDirs is `rt.StateDirs` as THIS mechanism can grant them, and it
// is the whole of ADR 0080 D19. Every other runtime's list passes through
// untouched; claude's is the one that cannot, and it is not a preference.
//
// WHY CLAUDE IS DIFFERENT. Its declaration is two entries (runtime.go), and
// one of them is a FILE: `~/.claude` is the CLI's own tree and
// `~/.claude.json` is where it keeps `projects`, the MCP list and the
// onboarding state. At L2 a file entry works — `(subpath …)` grants it, and
// SeatbeltSiblings grants the `.lock` and `.tmp.<pid>` names the CLI
// actually writes beside it (ranger-base-cypy1). At this mechanism neither
// move exists: a bind of a FILE refuses the atomic replace its writer makes
// (B8, MEASURED), and a mount cannot name a sibling NAMESPACE at all. So
// BwrapArgs refuses a writable file outright, and this function is what
// keeps that refusal from being every claude launch.
//
// WHAT REPLACES IT. MEASURED (D19): claude writes `.claude.json`'s lock and
// temp siblings inside `$CLAUDE_CONFIG_DIR`, and with that variable in force
// the config FILE is in there too (ClaudeConfigFile's own rule, trust.go).
// A whole-DIRECTORY grant therefore covers the file, the lock, the temp and
// every scratch name the CLI ever invents — which is the one thing the
// sibling regex was for. So the config directory joins the list, and the
// declared entry naming `$HOME/.claude.json` drops out.
//
// IT IS THE LAUNCHER'S RESOLVED VALUE AND NOT A SECOND SPELLING. The
// directory is ClaudeConfigDirIn's answer, which is the value ADR 0077 D1
// types onto the seat's own line (RuntimeHomePin) and the value credential.go
// walls — so the wall, the seed and the runtime cannot disagree about which
// directory is in play. That is surface 1's whole lesson
// (ranger-base-c33j6): posse seeded one file, the seat read another, and
// every signal said the launch was healthy.
//
// `~/.claude` STAYS. D19 drops the file entry and not the tree, and keeping
// it matters for a reason outside this function: credentialReadDenyLiterals'
// own() is keyed to the DECLARED state_dir literal, so a list that had lost
// `~/.claude` would read as "not a claude seat" and deny a claude seat its
// own credential on Linux — the one thing a runtime must never be denied
// (ADR 0019). Pinned, because nothing about this function's own output says
// it.
//
// UNSET REFUSES, rather than falling back to `<home>/.claude`. With the
// variable out of force the runtime resolves its config file to
// `$HOME/.claude.json` — outside any directory grant, and a file this
// mechanism cannot bind — so the fallback would be a wall that goes up,
// reports success, and silently loses every config write the seat makes.
// That is ranger-base-cypy1's failure with the grant in place, which is
// worse than no grant. The remedy is one line of host procedure
// (ranger-base-yyq64/offid) and the refusal names it.
func BwrapStateDirs(rt *Runtime) ([]string, error) {
	if rt == nil {
		return nil, nil
	}
	if rt.Name != "claude" {
		return rt.StateDirs, nil
	}
	// $HOME and not os.UserHomeDir(), because this home's only job is to
	// resolve the entry being dropped against the SAME home ExpandTilde
	// expands the surviving entries against one call later
	// (SeatbeltWritable). On unix the two are one value (ClaudeConfigFile's
	// own note, measured); where they could differ, agreeing with the
	// consumer is the property that matters.
	home := os.Getenv("HOME")
	_, inForce := runtimeHomeOverride(ClaudeConfigDirVar)
	return bwrapClaudeStateDirs(rt.StateDirs, ClaudeConfigDirIn(home), inForce, home)
}

// bwrapClaudeStateDirs is that answer with the environment already read, so
// that a pin can drive the one arm the environment cannot produce: a value
// in force that is still RELATIVE.
//
// runtimeHomeOverride makes a relative value absolute at the single read of
// these variables (ranger-base-sczjy), so on any box with a working cwd the
// `in force and relative` arm is unreachable — it survives only where
// os.Getwd itself failed, which is posse's own directory removed under it.
// The check stays because D19's demand is "set AND absolute" and the reason
// is the whole of ADR 0077 D1: the launcher resolves a relative value
// against ITS cwd and the seat resolves the same string against the session
// dir, so the directory walled and the directory written would be two
// directories. A render that cannot name one directory renders nothing.
//
// The declared list is otherwise copied verbatim, order intact. An operator
// who overlaid `state_dir:` for this box keeps it (ADR 0021 D1), and an
// overlaid entry that is a file this mechanism cannot bind still reaches
// BwrapArgs' own refusal with the path named — narrowing it quietly here
// would be a widening nobody could read off the argv.
func bwrapClaudeStateDirs(declared []string, dir string, inForce bool, home string) ([]string, error) {
	if !inForce || !filepath.IsAbs(dir) {
		return nil, bwrapClaudeConfigDirRefusal(dir, inForce)
	}
	// The config directory FIRST: it is claude's state grant at this
	// mechanism, and shallowest-first ordering is BwrapArgs' to impose
	// anyway (shallowestFirst), so this order is for the operator reading
	// `posse gates` rather than for the kernel.
	out := []string{dir}
	drop := ""
	if home != "" {
		drop = absResolve(filepath.Join(home, claudeConfigFileName))
	}
	for _, d := range declared {
		if drop != "" && absResolve(ExpandTilde(d)) == drop {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// bwrapClaudeConfigDirRefusal is D19's refusal, in its two arms. It names
// the variable, what it costs to leave it unset, and the one line of host
// procedure that fixes it for every seat on the box at once — because the
// bare failure this prevents has no error in it at all.
//
// The cage is spelled `seatbelt (bwrap)` from the constants and not through
// CageLabel, which reads the LIVE host: this render is Linux's, and a
// refusal rendered on darwin must still say which mechanism refused.
func bwrapClaudeConfigDirRefusal(dir string, inForce bool) error {
	why := "it is not set there, so the runtime resolves its config file to $HOME/" + claudeConfigFileName +
		" — a FILE, outside every directory grant, and binding one refuses the atomic replace its writer makes (probe B8): the wall would go up, report success, and lose every config write the seat makes"
	if inForce {
		why = "it is set to " + dir + ", which is not absolute — the launcher resolves that against its own directory and the seat against the session's, so the directory walled and the directory written would be two directories (ADR 0077 D1)"
	}
	return Die("claude on cage %s (%s) needs %s set in the launcher: %s. Set it in the home's unit Environment= — one line, and every claude seat on this host then resolves the very directory this wall grants (ADR 0080 D19)",
		CageSeatbelt, WallBwrap, ClaudeConfigDirVar, why)
}

// ─── D20: GIT'S REF WRITES NEED A DIR GRANT ─────────────────────────────────

// bwrapRefKeepName is the file the launcher leaves in the bound ref
// directory. Dot-named on purpose: git skips dot-named entries when it reads
// a loose ref directory (ASSUMED, probe T4c/T4d), so the file that keeps the
// directory alive is not also a ref called `.posse-keep`.
//
// EMPTY, and it has to be: git's loose-ref reader is what decides whether a
// stray file in `refs/heads/<x>/` is a broken ref, and the name is the whole
// of what posse controls there. Nothing reads its contents — not posse, not
// git — so a byte in it would be a claim about a reader that does not exist.
const bwrapRefKeepName = ".posse-keep"

// BwrapWritable is SeatbeltWritable's set as THIS mechanism can grant it,
// and it is the whole of ADR 0080 D20. Every entry passes through untouched
// except the git ones a linked-worktree session gets, which cannot: three of
// them name FILES in the shared common dir, and BwrapArgs refuses a writable
// file outright (B8 — binding one refuses the atomic replace its writer
// makes). This function is what keeps that refusal from being every
// worktree launch, the way BwrapStateDirs keeps it from being every claude
// launch.
//
// WHAT GIT ACTUALLY NEEDS, and why the L2 spelling has no translation. A ref
// update is `<ref>.lock` created, written, and then renamed onto `<ref>`.
// Both of those are writes on the ref's DIRECTORY, not on the ref: at L2
// that is sayable as two subpath grants over the leaf and its `.lock`
// sibling, because SBPL grants a path and the rename is a write to a granted
// name. A mount has no such move — the lock does not exist at launch so
// there is nothing to bind, a pre-created `.lock` fails every commit ("File
// exists"), and a bind mountpoint cannot be replaced by the rename(2) that
// commits the update. So the grant moves up one level, to the directory both
// writes land in: `<common>/refs/heads/posse` for the fleet's
// `posse/<session>` branches.
//
// THE KEEP FILE IS LOAD-BEARING AND IT IS NOT HOUSEKEEPING. `git pack-refs
// --all --prune` — which `git gc` runs on its own at gc.auto, from OUTSIDE
// the wall, in the operator's checkout — writes every loose ref into
// `packed-refs`, deletes the loose files, and then removes the directories
// that just emptied. The bound directory going away under a live seat is
// worse than it going away between launches (which the mkdir prep below
// would simply remake): the mount survives the prune and the seat keeps
// writing into an unlinked inode nothing reads. An empty dot-named file in
// the directory means it is never empty, so the prune leaves it. That is
// ranger-base-uuze's measurement at the other mechanism, where the same gc
// left every live session unable to commit; here it is answered by a file
// instead of by a create-only grant, because create-only is the thing a
// mount cannot say.
//
// A SLASHLESS BRANCH IN A LINKED WORKTREE GETS NO REF GRANT. Its ref sits
// directly in `refs/heads`, so the parent to bind would be `refs/heads`
// itself — `refs/heads/main` with it, and moving main from a dispatched
// session is the capability ranger-base-m2wf was opened to close. The commit
// fails loudly at `cannot lock ref`, and parity says why before the launch
// (bwrapRefGrantDifference below). The fleet's own branches all carry a
// slash, so this costs a hand-made worktree and nothing posse dispatches.
//
// `<common>/packed-refs.lock` IS DROPPED, and the cost is stated rather than
// closed. ADR 0059 D1 grants it at L2 so that git can finish its own cleanup
// after a sequencer verb; granting it here means a writable `<common>`,
// which is a writable `packed-refs`, `HEAD` and `config` — the fail-open
// direction, and `--ro-bind` over the children would be enumerating leaves.
// So bwrap seats carry the container tier's residual (ADR 0059 D4): a
// `cherry-pick`/`revert`/`rebase` that ends by deleting a pseudo-ref exits 0
// and leaves it, and AGENTS.md's git shim is the floor that refuses the next
// partial commit. Dropping it is also the only safe answer at a mount: an
// absent bind source is not refused, it is CREATED — a `packed-refs.lock`
// DIRECTORY in the operator's git dir would kill their every `gc` and
// `pack-refs` permanently.
//
// objects, logs and the worktree's OWN git dir are directories already and
// pass through as dir binds, which is what they are at both mechanisms.
//
// SEATBELT'S RENDER IS UNTOUCHED. sessionGitGrants and sessionRefDirs still
// say exactly what they said, and this is the only thing that knows
// otherwise — the same rule D19 kept for `~/.claude.json`, and for the same
// reason: narrowing the shared producer would take the leaf pair out of the
// darwin profile too and put ranger-base-uuze's lost commit back on every
// seatbelt seat on this box.
func BwrapWritable(writable []string, cwd string) (set []string, keep []string) {
	dirs := LinkedGitDirs(cwd)
	if len(dirs) != 2 {
		// A main checkout: sessionGitGrants named only `.git`, a directory,
		// and there is no common dir of anyone else's to narrow against.
		return writable, nil
	}
	return bwrapGitSwap(writable, dirs[1], repoBranch(cwd))
}

// bwrapGitSwap is that substitution with git already asked, so that a pin
// can drive every branch shape from one fixture: the two questions this
// function needs — which directory is the shared common dir, and what branch
// is checked out — cost a `git rev-parse` each, and a table that had to
// build a worktree per row would measure git's behaviour and not this one's.
//
// It compares RESOLVED paths and returns the caller's own spellings, because
// the set it filters is SeatbeltWritable's, whose every element is already
// absResolve'd, and the entries it drops are rebuilt here from the same two
// inputs sessionGitGrants built them from. A resolution mismatch would be
// silent in the safe-looking direction: the leaf would stay in the set and
// BwrapArgs would refuse the launch, naming the file. The DANGEROUS
// direction is closed by BwrapArgs rather than here — a `.lock` left in the
// set is a path that does not exist, which shape 1 prep would `mkdir -p`
// into a directory that fails every commit, and that is why the leaf pair's
// refusal is spelled there and not as a note.
//
// The order of what survives is the caller's, intact: BwrapArgs imposes
// shallowest-first on the whole set anyway (shallowestFirst), so this order
// is for the operator reading `posse gates` rather than for the kernel. The
// ref directory is appended LAST for the same reason — it is the entry this
// mechanism added, and a reader comparing the two walls should find it where
// it was added.
func bwrapGitSwap(writable []string, common, branch string) (set []string, keep []string) {
	drop := map[string]bool{absResolve(filepath.Join(common, "packed-refs.lock")): true}
	parent := ""
	if branch != "" {
		ref := filepath.Join(common, "refs", "heads", branch)
		drop[absResolve(ref)] = true
		drop[absResolve(ref+".lock")] = true
		// The DEEPEST parent only, and one bind covers the whole chain: for
		// `a/b/c` the launcher's `mkdir -p` makes `refs/heads/a` and
		// `refs/heads/a/b`, the bind lands on `a/b` where both of git's
		// writes go, and the prune cannot reach `a` while `a/b` is in it.
		//
		// The climb-out check is sessionRefDirs', for its reason: a parent
		// that is `refs/heads` itself is the slashless case above, and one
		// that is not under `refs/heads` at all is not a ref this grants.
		base := absResolve(filepath.Join(common, "refs", "heads"))
		if d := absResolve(filepath.Dir(ref)); d != base && underDir(base, d) {
			parent = d
		}
	}
	for _, w := range writable {
		if drop[absResolve(w)] {
			continue
		}
		set = append(set, w)
	}
	if parent != "" {
		set = append(set, parent)
		keep = append(keep, filepath.Join(parent, bwrapRefKeepName))
	}
	return set, keep
}

// applyBwrapRefGrant is D20's parity row, and both of its arms are
// statements about this mechanism's ref grant that an operator cannot read
// off anything else: the session that gets NO ref grant and will fail its
// first commit, and the widening the grant carries where it is made.
//
// A DECLARED DIFFERENCE AND NOT DEGRADATION (ADR 0017 §2), in both arms,
// because nothing here is a wall posse asked for and did not get. The
// slashless arm is a wall NARROWER than the one `cage: seatbelt` means on
// darwin — the mechanism differs, and what it costs is a function the seat
// loses loudly rather than a gate that goes unrealized. Degrading it would
// put a safe-by-design refusal behind --allow-degraded and would refuse
// dispatch for it, which is the reading ranger-base-d17a closed.
//
// Directory-aware, so it is CheckParityIn's and never CheckParity's: both
// facts come from `git rev-parse` in the launch dir.
func (a *App) applyBwrapRefGrant(p *Parity, dir string) {
	if d := bwrapRefGrantDifference(WallMechanism(), p.Cage, a.cageAvailable(p.Cage), dir); d != "" {
		p.DeclaredDifference = append(p.DeclaredDifference, d)
	}
}

// bwrapRefGrantDifference is that row with the host facts in hand, so a
// darwin pin can read the Linux mechanism's answer — which is the only way
// to pin it at all until D18 (ranger-base-djl3c) types the wall and this box
// is a Linux host.
//
// IT READS THE RENDER'S OWN ANSWER rather than re-deciding what the grant
// is: `keep` is empty exactly when bwrapGitSwap granted no ref directory, so
// a note and a wall that disagree is not a state these two can reach. That
// is the one property worth having here — a parity line claiming a grant the
// argv does not make is worse than no line, because it is what an operator
// checks the design against.
//
// AVAILABILITY IS ONE OF THE FACTS. A mechanism that cannot put the wall up
// here grants nothing, and a declared difference about its shape would be a
// claim about a wall nobody has — parity already says `cage seatbelt is not
// available on this host/build` in that case, which is the sentence to act
// on. So on today's Linux host, where D18 has not landed and
// wallMechanismRendered admits sandbox-exec alone, this prints nothing and
// the launch is refused; the day D18 lands it prints.
func bwrapRefGrantDifference(mech, cage string, available bool, dir string) string {
	if mech != WallBwrap || cage != CageSeatbelt || !available || dir == "" {
		return ""
	}
	dirs := LinkedGitDirs(dir)
	if len(dirs) != 2 {
		return "" // a main checkout: cwd's own grant covers .git whole
	}
	// A detached HEAD is not a difference: seatbelt grants no ref there
	// either (sessionGitGrants), and a caged worktree session is detached by
	// the launcher on purpose (PrepareSessionHead). Nothing to say.
	branch := repoBranch(dir)
	if branch == "" {
		return ""
	}
	_, keep := bwrapGitSwap(nil, dirs[1], branch)
	if len(keep) == 0 {
		return fmt.Sprintf("cage %s (%s): branch %s is a top-level ref, so this session gets NO write grant for it — the directory to bind would be %s, and %s is in it. A commit on this branch fails at `cannot lock ref`; the fleet's own branches are posse/<session>, whose parent directory is this session's alone to write (ADR 0080 D20)",
			CageSeatbelt, WallBwrap, branch, AbbrevHome(filepath.Join(dirs[1], "refs", "heads")), filepath.Join("refs", "heads", "main"))
	}
	return fmt.Sprintf("cage %s (%s): the ref grant for %s is the directory %s, because a ref update is a lock-then-rename and a mount cannot grant the leaf — so this session can also move a PEER session's ref in that directory, which the darwin mechanism's leaf grant does not allow. It still cannot move %s or any top-level branch, or write the shared hooks or config (ADR 0080 D20)",
		CageSeatbelt, WallBwrap, branch, AbbrevHome(filepath.Dir(keep[0])), filepath.Join("refs", "heads", "main"))
}
