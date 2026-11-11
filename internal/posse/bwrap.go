package posse

// Gates L2 on Linux — the seatbelt tier's bwrap mechanism (ADR 0080 D16-D20).
// Three halves, in order: whether this host can put the wall up at all (D16),
// the pure render from the L2 lists to a bwrap argv (D17, with D19's and
// D20's two substitutions), and the launch (D18) — the prep, the two files
// and the prefix.
//
// bubblewrap is the Linux answer to the same question sandbox-exec answers
// on darwin, and D14 picked it off a measured host probe (ranger-base-hft2q):
// one setuid-or-userns binary, no daemon, no image, and a mount namespace
// per seat that an adversarial process inside cannot unmount. The lists it
// renders from are the ones seatbelt.go already produces — SeatbeltWritable,
// SeatbeltCarveOut, credentialReadDenyLiterals, sessionRefDirs are the same
// for both mechanisms. Only the last step, list to wall, differs.
//
// WHERE THE WRITES ARE, and why that is worth saying up front. Everything
// from BwrapArgs down to D20's parity row is pure in SeatbeltProfile's sense
// — it stats the tree and appends strings, writes nothing and forks nothing —
// which is what lets every shape be pinned by a table on darwin. The D18
// section at the bottom is the exception and the only one: `RenderBwrap`
// makes the host-side prep the namespace cannot and writes the argv where a
// launch line can read it. A reader asking "what does a launch do here"
// starts there; a reader asking "what is the wall" starts at BwrapArgs.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
	// PrepSeed creates the path as a FILE holding its format's empty value
	// (seedBody), and only when nothing is there: it never rewrites a file
	// that exists (ranger-base-6akzw).
	PrepSeed PrepKind = "seed"
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
	// --new-session (setsid would detach the TUI from its controlling
	// terminal). The host's /tmp IS replaced, by a bind of the seat's own
	// directory rather than a tmpfs (1c below, ranger-base-7wdb7).
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

	// 1b. The host's control endpoints (ranger-base-ludpp). `--ro-bind / /`
	// stops writes and NOT connect(2) on a socket or a write-open of a FIFO:
	// the kernel's read-only check covers neither. The user runtime dir is
	// where a desktop session keeps exactly those, and several of them act
	// OUTSIDE the cage on a connection's say-so — the session bus and the
	// user service manager, the multiplexer's server, the gpg and ssh
	// agents, the secret store, other seats' sockets.
	//
	// AN ALLOWLIST, NOT A DENYLIST: an empty tmpfs over the whole directory,
	// so an endpoint a host grows next month is hidden without anyone
	// naming it. What a seat needs back is re-bound by a write grant below,
	// which mounts later and so lands on top — and today that is nothing
	// (MEASURED, ranger-base-ludpp: the rex reporter writes to /dev/tty, and
	// claude's own messaging socket could never be created under the
	// read-only base anyway; on the tmpfs it can, privately).
	//
	// The three variables go with it, because an address can name an
	// endpoint OUTSIDE the hidden dir (an abstract socket, an agent socket
	// under /tmp), and a seat has no use for any of them.
	for _, k := range bwrapUnsetEnv {
		argv = append(argv, "--unsetenv", k)
	}
	hidden := shallowestFirst(carve.Hide)
	for _, h := range hidden {
		exists, dir, err := bwrapStat(h)
		if err != nil {
			return nil, nil, bwrapStatRefusal("hide", h, err)
		}
		switch {
		case !exists:
			continue // nothing there to reach
		case !dir:
			return nil, nil, Die("bwrap cannot hide %s: it is not a directory, and an empty tmpfs can only be mounted over one (ranger-base-ludpp)", h)
		}
		argv = append(argv, "--tmpfs", h)
	}

	// 1c. The seat's own /tmp (ranger-base-7wdb7). The host's /tmp is the
	// one directory every walled seat and every uncaged process on the box
	// write to, at one uid: an uncaged `go build` compiles and links there,
	// node keeps its compile cache there, and the X11 and ICE sockets live
	// there. So the render binds the seat's private directory OVER it, and
	// the host's /tmp is in no seat's view — not as a write grant and not as
	// a read either.
	//
	// A BIND, NOT A TMPFS: a seat's temp files outlive nothing on a tmpfs,
	// and the operator reading a dead seat's `posse-test-signal.log` needs
	// them on the host. The directory lives under the seat's cache dir, so
	// the worktree remove that takes the cache takes this too.
	//
	// It joins `hidden` for the rest of the render, so the rule that already
	// holds for the runtime dir holds here: a write grant UNDER /tmp is a
	// named host directory and still mounts on top (shallowest first), and a
	// deny or Keep under /tmp that no grant re-opened renders nothing,
	// because a bind from the host is exactly what this mount exists to keep
	// out. A grant of /tmp ITSELF is what this replaces, so it is dropped
	// from the grants below rather than rendered over it.
	//
	// A source that itself lies under the host's /tmp (a test home) is still
	// private: bwrap opens the source before it mounts, so the seat sees
	// that one directory and nothing beside it.
	if carve.Tmp != "" {
		exists, dir, err := bwrapStat(carve.Tmp)
		if err != nil {
			return nil, nil, bwrapStatRefusal("private tmp", carve.Tmp, err)
		}
		switch {
		case exists && !dir:
			return nil, nil, Die("bwrap cannot bind %s over %s: it is not a directory (ranger-base-7wdb7)", carve.Tmp, bwrapTmpMount)
		case !exists:
			prep = append(prep, PrepStep{PrepMkdirAll, carve.Tmp, "the seat's private /tmp, bound over the host's so no seat writes where uncaged builds stage what they execute"})
		}
		argv = append(argv, "--bind", carve.Tmp, bwrapTmpMount)
		hidden = append(hidden, bwrapTmpMount)
		kept := writable[:0:0]
		for _, w := range writable {
			if absResolve(w) != bwrapTmpMount {
				kept = append(kept, w)
			}
		}
		writable = kept
	}

	// 2. Shape 1: the write grants, shallowest first so a nested grant's
	// mount lands on top of the one that contains it.
	grants := shallowestFirst(writable)
	// reopens answers whether a later mount of p would bind a HOST path back
	// into a hidden directory that no write grant re-opened. A deny or a mask
	// is a `--ro-bind` from the host, and a read-only bind of a socket is
	// still a socket a seat can connect to — so inside a hidden directory
	// the tmpfs is already the stronger statement and the mount is skipped.
	reopens := func(p string) bool { return writeGranted(hidden, p) && !writeGranted(grants, p) }
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
			//
			// THE DENY ARM BELOW NO LONGER GUESSES and this one still does,
			// which is a difference in what the producers can say and not a
			// difference of opinion (ranger-base-ppnds). A file-shaped deny
			// is named by code that knows — `<gitdir>/config.lock` is a
			// lockfile by construction — so the carve-out carries the kind
			// (SeatbeltCarveOut.DenyFile). A file-shaped GRANT arrives from a
			// `state_dir:` declaration, and ADR 0012 D4's key says
			// "directories (and single files)" without saying which: nothing
			// but a stat can tell, and the path is not there. Git's three —
			// the ref leaf, its `.lock` and `packed-refs.lock` — are gone:
			// D20's BwrapWritable takes them out of the set before this loop
			// sees it, which is a producer answering the same question one
			// step earlier. What is left is claude's `~/.claude.json`, D19's
			// to remove, and the exists-as-a-file arm above refuses it loudly
			// on any box that HAS run the CLI. The next file-shaped grant
			// after that reopens this; what closes it is the producer of that
			// grant saying so, either way the deny half and D20 now do.
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
		if reopens(d) {
			continue // hidden already, and the tmpfs is the stronger statement
		}
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
			// A FILE-SHAPED DENY GETS NEITHER, and this is the one place in
			// the render where the carve-out is asked what a path IS rather
			// than the filesystem (ranger-base-ppnds, finding 1 of the verify
			// at ranger-base-vnp01).
			//
			// `mkdir -p` is what this arm did for every absent deny, and
			// several Deny entries are file paths that are absent on every
			// healthy repo: `<gitdir>/config.lock`, `config.worktree`,
			// `config.worktree.lock` (sessionGitConfigFiles,
			// sessionWorktreeIdentityFiles, and the beads store's own git dir
			// through beadsGitDirs). The step runs OUTSIDE the namespace, in
			// the SHARED repo, in the seat's name, and nothing cleans it up —
			// so what the wrong kind costs, MEASURED by hand against a real
			// repo at git 2.50.1 with a green control first (the verify's
			// REPRO (c), ranger-base-ppnds):
			//
			//   config.lock as a DIR          `git config` dies with
			//                                 `error: could not lock config
			//                                 file .git/config: File exists`.
			//                                 commit and status unaffected, so
			//                                 it is found late and far away.
			//   config.worktree as a DIR      inert while
			//                                 extensions.worktreeConfig is
			//                                 unset; with it set — git sets it
			//                                 itself for sparse-checkout —
			//                                 status, config --list and commit
			//                                 all exit 128 at `fatal: unknown
			//                                 error occurred while reading the
			//                                 configuration files`.
			//   config.worktree.lock as a DIR `git config --worktree` refused.
			//
			// A session cannot repair any of it from inside: the path is
			// behind the wall it is denied. And TOUCH is not the other half of
			// the fix — an empty FILE at `config.lock` is the same "File
			// exists" refusal the directory buys.
			//
			// So nothing is rendered and the residual is stated here rather
			// than closed, because closing it needs a mechanism this one does
			// not have (a mount cannot say "this name may not come into
			// being"): the session CAN create these files inside the grant
			// that contains them, and no wall is over them when it does. What
			// it still cannot do is make one take effect. The file each lock
			// would be renamed over — `config`, `config.worktree` when it
			// exists — is itself bound read-only, and B8 measured that a
			// rename cannot cross a mount point, so the refusal lands one step
			// later than at the SBPL mechanism (`could not write config file`
			// where seatbelt says `could not lock`, both arms measured in
			// seatbeltgitidentity_qa_test.go) and the attacker-chosen config
			// reaches the lock and never the config. The costs that remain are
			// a stray lockfile in the shared repo, which an operator's next
			// `git config` trips over, and an inert `config.worktree` that
			// becomes live only if something first writes the extension into a
			// config this wall denies. Both are smaller than a directory the
			// launcher makes unconditionally at every launch, which is what
			// the mkdir did.
			//
			// THE ONE FILE-SHAPED DENY THAT IS MADE is a Seeded one
			// (ranger-base-6akzw): claude's executable config, where an empty
			// file is exactly as inert as none (seedBody) and the residual
			// above is not — a created settings.json is read by every other
			// claude sharing the directory, uncaged ones included. The
			// launcher writes the empty value and the mount goes over it.
			switch {
			case carve.Seeded(d):
				prep = append(prep, PrepStep{PrepSeed, d, "executable config that is not there yet and sits inside a write grant; created empty so the read-only mount can cover it"})
			case carve.FileShaped(d):
				continue
			default:
				prep = append(prep, PrepStep{PrepMkdirAll, d, "a deny that is not there yet and sits inside a write grant, so the session could create it"})
			}
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
		if reopens(f) {
			continue // its deny was skipped above, and a re-allow would bind the host file back in
		}
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

// bwrapUnsetEnv is what the render takes out of a seat's environment
// (ranger-base-ludpp): the session bus address and the two agent addresses.
// Each names an endpoint that acts outside the cage, and an address can sit
// outside the runtime dir the render hides — so hiding the directory is not
// enough by itself. Unsetting a variable that is not set is a no-op in bwrap,
// so the list is rendered unconditionally and the argv does not depend on
// the launcher's own environment.
var bwrapUnsetEnv = []string{"DBUS_SESSION_BUS_ADDRESS", "SSH_AUTH_SOCK", "GPG_AGENT_INFO"}

// bwrapRuntimeDirs is the carve-out's Hide list: the user runtime dir, found
// the two ways a host names it, resolved, and only where it is a directory
// that is there (ranger-base-ludpp).
//
// BOTH SPELLINGS, not one. $XDG_RUNTIME_DIR is the one the desktop session
// exports and the one a seat's programs read, so it is the one that matters
// when it is set. The launcher is not always started from that session — a
// dispatch loop or a cron line may carry no XDG variable at all — and the
// endpoints are in /run/user/<uid> regardless, so that path is hidden too.
// On a normal host the two resolve to one directory and render one mount.
//
// REFUSED, NOT SKIPPED, when the answer would hide something a seat cannot
// run without: a relative value (the spec calls it invalid, and a renderer
// that guessed what it meant would be guessing where the wall goes), `/` or
// a top-level directory, or any directory holding the home. An empty tmpfs
// over those is not a narrower wall, it is a seat with no tree — and a
// launch that fails naming the variable beats one that comes up blind.
func bwrapRuntimeDirs(xdg string, uid int, home string) ([]string, error) {
	cands := []string{filepath.Join("/run/user", strconv.Itoa(uid))}
	if xdg != "" {
		if !filepath.IsAbs(xdg) {
			return nil, Die("bwrap cannot hide XDG_RUNTIME_DIR=%q: it is not an absolute path (the XDG spec calls such a value invalid), and the wall will not guess which directory it meant (ranger-base-ludpp)", xdg)
		}
		cands = append([]string{xdg}, cands...)
	}
	var out []string
	for _, c := range cands {
		r := absResolve(c)
		exists, dir, err := bwrapStat(r)
		if err != nil {
			return nil, bwrapStatRefusal("runtime dir", r, err)
		}
		if !exists {
			continue
		}
		if !dir {
			return nil, Die("bwrap cannot hide the runtime dir %s: it is not a directory (ranger-base-ludpp)", r)
		}
		if strings.Count(filepath.Clean(r), string(filepath.Separator)) < 2 {
			return nil, Die("bwrap will not hide the runtime dir %s: an empty tmpfs over / or a top-level directory leaves a seat with no tree. Point XDG_RUNTIME_DIR at a directory of its own (ranger-base-ludpp)", r)
		}
		if home != "" && underDir(r, absResolve(home)) {
			return nil, Die("bwrap will not hide the runtime dir %s: it holds the home %s, and an empty tmpfs over it leaves a seat with no home. Point XDG_RUNTIME_DIR at a directory of its own (ranger-base-ludpp)", r, home)
		}
		out = append(out, r)
	}
	return dedupeStrings(out), nil
}

// liveBwrapRuntimeDirs is bwrapRuntimeDirs with this process's answers — the
// launcher's, which is the seat's: one OS account (ADR 0080 D1).
func liveBwrapRuntimeDirs() ([]string, error) {
	home, _ := os.UserHomeDir()
	return bwrapRuntimeDirs(os.Getenv("XDG_RUNTIME_DIR"), os.Getuid(), home)
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
//
// IT TAKES THE PID BECAUSE THE SUBSTITUTION IS A GRANT AND NOT A REWRITE
// (ADR 0086 D3). The ref directory is this mechanism's own addition to the
// set — it is derived from cwd and the branch, not read off the list — so a
// PID that was granted no ref would be handed one here by a function that
// never asked whose wall it was rendering. For a PID that denies the whole
// tree there is nothing to swap either way: sessionGitGrantsFor answered
// nothing, so the drop list matches nothing and the only thing left to do is
// not add the parent. Returning early says that in the one place a reader
// asks it, instead of leaving it to be inferred from an empty intersection.
func BwrapWritable(ag *AgentFile, writable []string, cwd string) (set []string, keep []string) {
	if ag != nil && deniesFileWrite(ag.Deny) {
		return writable, nil // ADR 0086 D3: no git grant to swap, and no ref directory to add
	}
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
func (a *App) applyBwrapRefGrant(p *Parity, ag *AgentFile, dir string) {
	if d := bwrapRefGrantDifference(ag, WallMechanism(), p.Cage, a.cageAvailable(p.Cage), dir); d != "" {
		p.DeclaredDifference = append(p.DeclaredDifference, d)
	}
}

// bwrapRefGrantDifference is that row with the host facts in hand, so a
// darwin pin can read the Linux mechanism's answer — which is the only way
// to pin it at all on this box, D18 (ranger-base-djl3c) notwithstanding: what
// that bead typed is the wall, and the row's two facts are still a Linux
// host's.
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
// on. Before D18 (ranger-base-djl3c) that covered every Linux host, because
// wallMechanismRendered admitted sandbox-exec alone and the tier was
// therefore never in the table there; since D18 a Linux host with bwrap and
// a passing probe prints this row.
func bwrapRefGrantDifference(ag *AgentFile, mech, cage string, available bool, dir string) string {
	if mech != WallBwrap || cage != CageSeatbelt || !available || dir == "" {
		return ""
	}
	// ADR 0086 D3: a PID that denies the whole tree is granted no git dir of
	// its session at EITHER mechanism, so there is no difference here to
	// declare — the same silence a detached HEAD gets below, for the same
	// reason. A line printed only on Linux over a wall both mechanisms now
	// render identically would read as a bwrap-specific capability, which is
	// the opposite of what D3 did. What says this wall is real for such a
	// PID is the Edit/Write row's own witness at the launch (D5), and what
	// says where it allows writes at all is the writable set `posse gates`
	// prints above it — on both mechanisms, from one list.
	if ag != nil && deniesFileWrite(ag.Deny) {
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

// ─── D23: PREP CREATES THE SESSION'S OWN FURNITURE, NOTHING ELSE ─────────────

// pidHostWritable is the first of D23's two members on its own: the
// `writable:` extras that name a path OUTSIDE the session tree, resolved
// through pidWritableResolve so that the entry kept out of the set and the
// entry put into it are one spelling by construction and not by inspection.
//
// NAMED FOR THE QUESTION AND NOT FOR A SPELLING, since ranger-base-hoh3q.
// It was `pidAbsoluteWritable` through ranger-base-sg7a7 — docs/notes.d/
// ranger-base-sg7a7.md still names it that, and the parity row below was
// `bwrapAbsoluteWritableDifference` — and the old names were the defect
// said out loud: D23's two classes are decided by LOCATION, and a `..` in a
// relative entry is a host path that no spelling test calls one.
//
// A FUNCTION OF ITS OWN because two readers need exactly this half and
// neither wants the other: the render asks for both members at once, and the
// parity row below is about the PID and must not go asking the filesystem
// about a store of record (ADR 0013 §4's row owns that question).
func pidHostWritable(ag *AgentFile, cwd string) []string {
	if ag == nil {
		return nil
	}
	var out []string
	for _, w := range ag.Writable {
		if !pidWritableIsHostPath(w, cwd) {
			continue
		}
		if p, ok := pidWritableResolve(w, cwd); ok {
			out = append(out, p)
		}
	}
	return dedupeStrings(out)
}

// pidWritableIsHostPath is that test on one declared entry, and it is the
// one spelling of D23's line between the two classes. `posse agent check`
// asks it too, over `ag.Writable` rather than over the resolved list, so the
// lint can name the string the operator typed while the render names the
// path it binds — one rule, two vocabularies, no second predicate.
//
// THE QUESTION IS LOCATION, which is ranger-base-hoh3q's whole finding. It
// used to be `filepath.IsAbs(ExpandTilde(w))`, and that asks whether the
// DECLARED STRING is absolute, where D23 decided something else: "Prep's
// authority is posse's own perimeter: the session tree, ..." So
// `writable: ["../../shared"]` was CLASSED RELATIVE and therefore prepped,
// while pidWritableExtras resolves it with `filepath.Join(cwd, w)` — which
// cleans `..` — into a directory outside the session tree that posse then
// created on the host. MEASURED at this bead's own door
// (TestQABwrapPrepDoesNotCreateAPathOutsideTheSessionTree): all four of
// D23's surfaces were silent on it at once, and at the real spelling cwd is
// `~/.posse/worktrees/posse/<session>`, so `../shared` is a junk directory
// in the operator's worktrees dir that no session retires — bwrapNoCreate's
// own stated reason for counting `~` as absolute — and enough `..` reaches
// `/opt/shared`, the path D23 was written about.
//
// A `..` THAT RESOLVES BACK INSIDE THE TREE IS STILL RELATIVE, which is why
// this asks underDir rather than re-reading the string for a leading `..`:
// `scratch/../scratch` is session furniture and keeps its prep. underDir
// resolves BOTH sides, which it has to — on darwin a fixture cwd under
// `/var/folders/...` and its `/private/var/...` real path are one directory,
// and comparing one resolved side against one unresolved one would class
// every relative extra as a host path and kill the control arm.
//
// TWO ARMS, ONE RULE, and the second is where `posse agent check` stands: it
// has no session dir, box-independently so — the lint stats nothing and
// gives one answer wherever it runs. With no cwd the STRING is the whole of
// what is known, and `filepath.IsLocal` is that half exactly: a cleaned
// relative path that opens with `..` climbs out of whatever tree it is
// joined to, on every host. It is strictly WEAKER than the resolved arm —
// a symlink inside the tree pointing out of it is caught at the launch and
// not by the lint — and that is the direction to err in, because the lint's
// job is to name what the operator typed and the launch's job is to decide
// what posse writes.
func pidWritableIsHostPath(w, cwd string) bool {
	w = ExpandTilde(w)
	if w == "" {
		return false // names nothing, so it names nobody's host path
	}
	if filepath.IsAbs(w) {
		return true
	}
	if cwd == "" {
		return !filepath.IsLocal(w)
	}
	return !underDir(cwd, filepath.Join(cwd, w))
}

// bwrapNoCreate names the write grants the LAUNCHER MAY NOT CREATE, and it
// is the whole of ADR 0080 D23. BwrapWritable's twin one question over: that
// one says which paths this mechanism can GRANT, this one says which of them
// posse may bring into being on the host.
//
// WHY PREP NEEDS A PERIMETER AT ALL. Shape 1's prep exists because bwrap
// binds only a source that exists where sandbox-exec granted paths that did
// not, so a missing grant becomes a `mkdir -p` outside the namespace, in the
// seat's name (BwrapArgs, D17/D18). That is right for the session's own
// furniture and it is not a general licence: posse creating a path turns a
// grant into a host WRITE, and it makes the one PID line mean three
// different things on three hosts. MEASURED (ranger-base-sg7a7, from
// ranger-base-temhf's reproduction): a fixture PID declaring
// `writable: [scratch, /opt/shared]` is a free grant on darwin — SBPL grants
// a path whether or not it is there and nothing writes it — a REFUSED launch
// on this box (`mkdir /opt/shared: permission denied`) and a new directory on
// ci.yml's ubuntu runner, created silently on every red run.
//
// THE TWO MEMBERS, each because the creation is visible to a decision posse
// does not own:
//
//   - A `writable:` EXTRA THAT RESOLVES OUTSIDE THE SESSION TREE. It is a
//     HOST path the operator declared, not one posse derived, and nothing
//     in the session can bring it into being either (pidWritableExtras). An
//     extra that resolves INSIDE the tree keeps its prep: posse made that
//     tree and retires it with the session, and for a persona that denies
//     file writes cwd itself is not granted, so without the mkdir the grant
//     would be dead. `~` counts as outside, for homeCacheGrants' reason one
//     list over — junk directories in the operator's home, created by posse
//     and used by nothing — and so does a RELATIVE entry that climbs out
//     with `..`, which is the same junk directory one `cd` away and was
//     prepped until ranger-base-hoh3q (pidWritableIsHostPath).
//   - THE STORE OF RECORD. Its EXISTENCE is itself read as a fact: ADR 0013
//     §4's reachability row says "nothing to reach (bd creates it on first
//     write)" in its own words, and recordTargets probes only what is there.
//     MEASURED in the same reproduction: a seatbelt launch materialized
//     `<repo>/.beads`, and a launch three arms later in the SAME tree — a
//     codex seat rendering `-s read-only` — then had a real store behind a
//     read-only sandbox where it had none before, so the row went from
//     "nothing to reach" to unrealized and REFUSED that launch. Same tree,
//     same PIDs, a different answer depending on which seat launched first.
//     An empty `.beads` is not inert either: in no-db mode bd resolves
//     $BEADS_DIR else `$cwd/.beads` and reads no redirect at all (ADR 0055),
//     so a directory posse makes there is a second store of record that can
//     take writes nothing reads back. `bd init` is denied to every persona
//     because creating a store is a decision; it is not the launcher's
//     either.
//
// ONE SPELLING OF THE STORE, and it is `<cwd>/.beads`, which is the only one
// that can be ABSENT. SeatbeltWritable grants the store under two names —
// that one for a persona that denies file writes, and beadsHome's resolved
// target where a redirect moves it out of the tree (ADR 0012 D3-C) — but a
// redirect bd FOLLOWS is one whose target beadsRedirectHop has already
// stat'd as a directory, so the resolved target is there by construction and
// an entry for it would be a control with no failing arm. Said here because
// the two-entry version is the obvious reading of the key, and it costs a
// reader nothing to know which half the stat already answered
// (TestQABwrapTheStoreHasOneSpellingThatCanBeAbsent).
//
// A MEMBER THAT IS THERE IS GRANTED EXACTLY AS BEFORE. This list changes
// nothing about the wall over a host that has the directory; what it changes
// is whether posse makes one that nobody asked it to make.
//
// STATED RESIDUAL, not closed: where a redirect names a store in a directory
// that is not a git repository, SeatbeltWritable grants `<root>/.git` and
// prep still makes it. That path is not in this list because in the common
// case `<root>` IS the session tree, where `.git` is the session's own and
// prep creating it is the furniture this decision keeps. The cost is a `.git`
// directory in a non-repo, which the next `git` run there reports; the fix
// for it is a producer that knows which of the two it is naming, the way D20
// answered the ref pair one step earlier.
//
// SEATBELT'S RENDER IS UNTOUCHED, the rule D19 and D20 both keep: narrowing
// SeatbeltWritable instead would take these entries out of the darwin
// profile, where a grant for an absent path costs nothing and the session
// can still create it.
func bwrapNoCreate(ag *AgentFile, cwd string) []string {
	out := pidHostWritable(ag, cwd)
	if cwd != "" {
		out = append(out, absResolve(filepath.Join(cwd, beadsDirName)))
	}
	return dedupeStrings(out)
}

// bwrapBindExisting applies that list to a writable set: a no-create grant
// that is THERE stays and is bound as any other grant, one that is not is
// taken out of the set and returned, so this render binds nothing for it and
// asks the launcher for nothing.
//
// OUT OF THE SET AND NOT A FLAG ON IT, which is D20's own move
// (bwrapGitSwap) and keeps BwrapArgs' five shapes exactly as they are: the
// renderer never learns that this question exists, there is no sixth arm in
// it to get wrong, and its signature does not grow a parameter at thirty
// call sites.
//
// A FILE AT THE PATH STAYS IN THE SET, deliberately. B8's refusal is
// BwrapArgs' and names the path and the remedy; answering it here would be a
// second refusal for one fact, and a quieter one.
//
// THE RE-STAT IS BENIGN. BwrapArgs stats the survivors again and would prep
// one that vanished between the two statements of a single render — a path
// that was on the host when the render began, which is not a path posse
// invented. The alternative is passing this answer down, and the shape of
// what is being kept out is "a directory posse never had a reason to make",
// not "a byte-exact inventory of the filesystem at one instant".
func bwrapBindExisting(set, noCreate []string) (kept, absent []string, err error) {
	if len(noCreate) == 0 {
		return set, nil, nil
	}
	no := map[string]bool{}
	for _, p := range noCreate {
		no[p] = true
	}
	for _, w := range set {
		if w == "" || !no[absResolve(w)] {
			kept = append(kept, w)
			continue
		}
		exists, _, serr := bwrapStat(w)
		if serr != nil {
			return nil, nil, bwrapStatRefusal("write grant posse may not create", w, serr)
		}
		if exists {
			kept = append(kept, w)
			continue
		}
		absent = append(absent, w)
	}
	return kept, absent, nil
}

// bwrapAbsentGrantLine is how an absent no-create grant reads wherever it is
// printed — the readable copy, `posse gates`, and the parity row. One
// sentence and one spelling, because an operator comparing the three is
// comparing the same fact.
func bwrapAbsentGrantLine(p string) string {
	return AbbrevHome(p) + " (declared, NOT granted: it is not on this host, and the launcher does not create a path it did not derive — bwrap binds only a source that exists, so this render makes no mount for it; create the directory outside posse, or declare it relative to the session dir and INSIDE it, ADR 0080 D23)"
}

// bwrapHostWritableDifference is D23's parity row: the `writable:` extras
// this PID named outside the session tree that the launch host does not
// have, said before the launch rather than discovered by a failed write
// inside the seat. (`bwrapAbsoluteWritableDifference` through
// ranger-base-sg7a7, which is the name docs/notes.d/ranger-base-sg7a7.md
// still uses — renamed with the predicate on ranger-base-hoh3q, where the
// class stopped being a spelling.)
//
// A DECLARED DIFFERENCE AND NOT DEGRADATION (ADR 0017 §2), for
// applyBwrapRefGrant's reason in its own words: nothing here is a wall posse
// asked for and did not get. It is a GRANT the mechanism renders narrower
// than darwin's, what it costs is a function the seat loses loudly — a write
// into that path is refused by the base `--ro-bind / /` — and degrading it
// would put a safe-by-design narrowing behind --allow-degraded and refuse
// dispatch for a path that is nobody's gate, which is the reading
// ranger-base-d17a closed.
//
// IT NEEDS NO WRITABLE SET AND NO RUNTIME, which is why this row is cheap
// enough to sit on the launch path: every extra is in the set
// unconditionally (pidWritableExtras), so the PID plus a stat is the whole
// of the question. The store of record's half of D23 is NOT here — ADR 0013
// §4's reachability row is already the reader for "there is no store at this
// launch dir", and it says so in one place (applyRecordReach).
//
// IT TAKES THE LAUNCH DIR, since ranger-base-hoh3q, and that is the one
// thing that stopped being free here. An absolute path is absolute wherever
// you ask, but a RELATIVE extra that climbs out of the session tree is a
// host path whose identity is a function of the dir — so the row that used
// to be directory-independent now reads the same `dir` the rest of
// checkParityIn reads, and says nothing about a relative entry where there
// is no dir to resolve it against (`posse agent check`'s arm, which is the
// lint's surface and not this one).
//
// A STAT THAT FAILS FOR ANY OTHER REASON IS SILENT HERE, which is not the
// render's answer to the same question and is deliberate. The render refuses
// the launch naming the path and the list (bwrapStatRefusal), because a path
// posse could not ask about must not become a path posse decided about. A
// row that said "not on this host" over `permission denied` would be stating
// the one thing that was not measured — and the refusal it would be
// competing with is the louder signal and arrives first.
//
// Mechanism-parameterized so a darwin pin can read the Linux answer, the
// rule every host-shaped decision in this file follows.
func bwrapHostWritableDifference(mech, cage string, available bool, ag *AgentFile, cwd string) string {
	if mech != WallBwrap || cage != CageSeatbelt || !available || ag == nil {
		return ""
	}
	var absent []string
	for _, p := range pidHostWritable(ag, cwd) {
		if exists, _, err := bwrapStat(p); err == nil && !exists {
			absent = append(absent, AbbrevHome(p))
		}
	}
	if len(absent) == 0 {
		return ""
	}
	return fmt.Sprintf("cage %s (%s): this PID's `writable:` names %s outside the session tree and this host does not have %s, so the launch binds nothing there and a write into it is refused by the base read-only bind. The launcher does not create a path it did not derive, where SBPL grants an absent path and lets the session make it. Create the directory outside posse, or declare it relative to the session dir and INSIDE it (ADR 0080 D23)",
		CageSeatbelt, WallBwrap, strings.Join(absent, ", "), plural(len(absent), "that path", "those paths"))
}

// applyBwrapHostGrants puts that row on a parity matrix. It sits in
// CheckParityIn beside applyBwrapRefGrant because it STATS, which is the
// line parity.go's header draws (CheckParity stats nothing and tests rely
// on that) — and since ranger-base-hoh3q it is directory-AWARE for
// applyBwrapRefGrant's reason too: which paths this PID's `writable:` puts
// outside the session tree is a fact about a concrete launch dir as soon as
// one of them is spelled relatively.
func (a *App) applyBwrapHostGrants(p *Parity, ag *AgentFile, dir string) {
	if d := bwrapHostWritableDifference(WallMechanism(), p.Cage, a.cageAvailable(p.Cage), ag, dir); d != "" {
		p.DeclaredDifference = append(p.DeclaredDifference, d)
	}
}

// ─── D18: THE LAUNCH LINE ───────────────────────────────────────────────────

// The two files RenderBwrap writes into the persona's gates dir, beside
// `seatbelt.sb`. One is read by bwrap and one is read by people, and the
// split is deliberate: the argv is NUL-separated because a mount source may
// hold any byte a path may hold, and a file an operator can read has to
// break lines somewhere.
const (
	// bwrapArgsName holds the argv, each element followed by a NUL. This is
	// the ARTIFACT — what `--args 3` reads and what the wall is.
	bwrapArgsName = "bwrap.args"
	// bwrapTextName is the readable twin. Nothing reads it back, which is
	// why it may carry a header and a prep list the argv has no room for.
	bwrapTextName = "bwrap.txt"
)

// RenderBwrap is ADR 0080 D18's half of the launch: it renders this seat's
// wall from the L2 lists, makes on the host what the namespace cannot, writes
// the argv where the launch line will read it, and returns that file's path.
// `RenderSeatbelt`'s twin, called from the same place in a launch through
// RenderWall (wall.go).
//
// THE PREP IS NOT HOUSEKEEPING AND IT IS WHY THIS FUNCTION EXISTS. D17's
// render is pure, and three of its five shapes cannot be realized without a
// write OUTSIDE the namespace first: bwrap binds only a source that exists,
// where sandbox-exec granted paths that did not (resolveExisting). So a
// missing write grant is a `mkdir -p`, a missing deny inside a grant is an
// empty directory the mount goes over, a missing audit log is a `touch`, and
// the ref keep file is a `touch` that renders no mount at all (D20). Every
// one of them is a write posse makes as the seat's user before the wall goes
// up, in the order BwrapArgs emitted them — which is a promise, not a set:
// a grant's mkdir precedes the touch of a file inside it.
//
// THE TWO SUBSTITUTIONS ARE APPLIED HERE AND NOWHERE ELSE, which is what
// keeps the darwin profile untouched (D19 and D20 both say so in their own
// words). `BwrapStateDirs` is the runtime's state as this mechanism can
// grant it — claude's config DIRECTORY instead of its config file — and
// `BwrapWritable` swaps git's ref leaf and `.lock` for the ref's parent
// directory plus a keep file. Narrowing `SeatbeltWritable` or
// `sessionGitGrants` instead would take those entries out of the sandbox-exec
// profile too and reinstate ranger-base-cypy1's lost config write and
// ranger-base-uuze's lost commit on every seatbelt seat on this box.
//
// THE CARVE-OUT IS COMPUTED FROM THE DECLARED SET AND NOT FROM THE SWAPPED
// ONE. `SeatbeltCarveOut` reads the writable set for exactly one thing — the
// rename seal — and `Seal` renders no mount at this mechanism (BwrapArgs step
// 3's note: a mount is a statement about a mount POINT, so there is nothing
// for a rename to walk around). Feeding it the declared set is therefore
// identical in output and true to D16's claim that both mechanisms render
// from the same lists: `SeatbeltWritable`, `SeatbeltCarveOut`,
// `credentialReadDenyLiterals` and `sessionRefDirs` answer once, and only the
// last step differs.
func (a *App) RenderBwrap(ag *AgentFile, rt *Runtime, cwd string) (string, error) {
	args, _, err := a.renderBwrapWall(ag, rt, cwd)
	return args, err
}

// renderBwrapWall is RenderBwrap with the render itself handed back, so that
// the report can print the wall it just WROTE rather than a second one it
// composed. SeatbeltReport renders its profile for the same reason — `posse
// gates` is where an operator reads the wall off the output, and a report
// whose head line names a file that is not there is worse than no report.
func (a *App) renderBwrapWall(ag *AgentFile, rt *Runtime, cwd string) (string, bwrapRendered, error) {
	gatesDir := a.GatesDir(ag.Name)
	if err := os.MkdirAll(gatesDir, 0o755); err != nil {
		return "", bwrapRendered{}, err
	}
	r, err := a.bwrapRender(ag, rt, cwd, gatesDir)
	if err != nil {
		return "", bwrapRendered{}, err
	}
	if err := runBwrapPrep(r.Prep); err != nil {
		return "", bwrapRendered{}, err
	}
	args := filepath.Join(gatesDir, bwrapArgsName)
	if err := os.WriteFile(args, bwrapArgsBytes(r.Argv), 0o644); err != nil {
		return "", bwrapRendered{}, err
	}
	if err := os.WriteFile(filepath.Join(gatesDir, bwrapTextName), []byte(bwrapReadable(ag.Name, args, r.Argv, r.Prep, r.Absent)), 0o644); err != nil {
		return "", bwrapRendered{}, err
	}
	return args, r, nil
}

// bwrapRendered is one render of this mechanism's wall: the lists as this
// mechanism grants them and what D17 made of them. It exists so the report
// and the launch are ONE render — a report composed from the inputs would
// agree with the design while disagreeing with the wall, which is the class
// reachability.go's file comment refuses in the same words.
type bwrapRendered struct {
	// Set is the writable set after D19's and D20's substitutions: what the
	// argv actually binds read-write.
	Set []string
	// Keep is D20's ref keep file — the one render input that binds nothing.
	Keep []string
	// Carve is the carve-out, computed from the DECLARED writable set (see
	// RenderBwrap's own note on why that is identical here).
	Carve SeatbeltCarveOut
	// Absent is D23's answer: the write grants posse may not create that
	// this host does not have, so Argv binds nothing for them and Prep asks
	// for nothing. Kept on the render rather than recomputed, for the reason
	// this struct exists — a report that composed it again would agree with
	// the design while disagreeing with the wall.
	Absent []string
	// Argv and Prep are D17's two outputs.
	Argv []string
	Prep []PrepStep
}

// bwrapRender is the list-to-argv half with no write in it: the two
// substitutions, the carve-out, and D17's render. Split out so that
// RenderBwrap and BwrapReport cannot render two different walls.
func (a *App) bwrapRender(ag *AgentFile, rt *Runtime, cwd, gatesDir string) (bwrapRendered, error) {
	stateDirs, err := BwrapStateDirs(rt)
	if err != nil {
		return bwrapRendered{}, err
	}
	writable := a.SeatbeltWritable(ag, cwd, gatesDir, stateDirs...)
	r := bwrapRendered{Carve: a.SeatbeltCarveOut(ag, cwd, gatesDir, writable, stateDirs...)}
	// The runtime dir's control endpoints (ranger-base-ludpp). Here and not
	// in SeatbeltCarveOut, because seatbelt has no spelling for it.
	if r.Carve.Hide, err = liveBwrapRuntimeDirs(); err != nil {
		return bwrapRendered{}, err
	}
	r.Set, r.Keep = BwrapWritable(ag, writable, cwd)
	// The seat's own /tmp (ranger-base-7wdb7): the host dir BwrapArgs binds
	// over /tmp, and the grants it replaces out of the set, so the report
	// prints the wall the argv builds.
	home, _ := os.UserHomeDir()
	if r.Carve.Tmp = bwrapSeatTmp(home, cwd); r.Carve.Tmp != "" {
		r.Set = bwrapDropTmpGrants(r.Set, os.Getenv("TMPDIR"))
	}
	// D23, after D20's substitution and before the render: what is left of
	// the set is what posse will bind, and a no-create grant that is not
	// there has been taken out of it, so BwrapArgs can ask the launcher for
	// every missing grant it still sees.
	if r.Set, r.Absent, err = bwrapBindExisting(r.Set, bwrapNoCreate(ag, cwd)); err != nil {
		return bwrapRendered{}, err
	}
	if r.Argv, r.Prep, err = BwrapArgs(cwd, r.Set, r.Carve, r.Keep...); err != nil {
		return bwrapRendered{}, err
	}
	// The seat's own caches (ranger-base-olr0g): the dir SeatbeltWritable
	// granted, and the env that points the tools at it, in one render so the
	// two cannot name different dirs. Set inside the namespace, so it holds
	// for every process the wall runs and for nothing outside it.
	for _, v := range seatCacheEnv(SeatCacheDir(home, cwd)) {
		r.Argv = append(r.Argv, "--setenv", v.Key, v.Value)
	}
	// A launcher TMPDIR is a host path, and the seat's temp dir is now its
	// own /tmp: one value for every seat, whatever the launcher carried.
	if r.Carve.Tmp != "" {
		r.Argv = append(r.Argv, "--setenv", "TMPDIR", bwrapTmpMount)
	}
	return r, nil
}

// bwrapArgsBytes is the args file's format: every element followed by a NUL,
// the last one included. That is what `bwrap --args FD` reads, and the probe's
// T7 row writes the same bytes with `printf '%s\0'`.
//
// NO SEPARATOR THAT A PATH MAY CONTAIN. A newline-separated file would be
// readable and would be wrong the first time a mount source held a newline —
// which is a legal byte in a path on both hosts, and which posse does not
// choose: a `writable:` extra and a `state_dir:` come from the PID and from
// the box. The readable copy is a second FILE for that reason rather than a
// second format for this one.
func bwrapArgsBytes(argv []string) []byte {
	var b strings.Builder
	for _, a := range argv {
		b.WriteString(a)
		b.WriteByte(0)
	}
	return []byte(b.String())
}

// runBwrapPrep makes on the host what the namespace cannot, in the order
// BwrapArgs emitted the steps. The launcher is outside the wall, so every
// one of these is a write in the seat's name — which is why each step
// carries its reason and why the readable copy prints them.
//
// IN ORDER AND NOT CONCURRENTLY (PrepStep's own rule): a Keep file's
// directory is a grant one loop up, so its mkdir has just been emitted and
// the touch that follows cannot run first.
//
// A TOUCH NEVER TRUNCATES. The files this creates are the session's own audit
// logs and the ref keep file; BwrapArgs only emits a step for a path that is
// not there, and the open below carries no O_TRUNC, so a re-render against a
// live seat's gates dir cannot empty `refusals.log`.
//
// AN UNKNOWN KIND REFUSES rather than being skipped. A step posse does not
// know how to make is a grant the argv names and the host does not have, and
// bwrap's own failure for that is `Can't find source path` with no reason in
// it — so the refusal is here, where the reason is.
func runBwrapPrep(prep []PrepStep) error {
	for _, s := range prep {
		switch s.Kind {
		case PrepMkdirAll:
			if err := os.MkdirAll(s.Path, 0o755); err != nil {
				return Die("cage %s (%s) cannot prepare %s (%s): %v — the launcher makes this outside the namespace because bwrap binds only a source that exists (ADR 0080 D17/D18)",
					CageSeatbelt, WallBwrap, s.Path, s.Why, err)
			}
		case PrepTouch:
			f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return Die("cage %s (%s) cannot prepare %s (%s): %v — the launcher makes this outside the namespace because bwrap binds only a source that exists (ADR 0080 D17/D18)",
					CageSeatbelt, WallBwrap, s.Path, s.Why, err)
			}
			if err := f.Close(); err != nil {
				return err
			}
		case PrepSeed:
			// O_EXCL: a file that appeared since the render is left exactly
			// as it is — the mount covers whatever is there, and the seed's
			// only job is to make sure SOMETHING is.
			if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
				return Die("cage %s (%s) cannot prepare %s (%s): %v — the launcher makes this outside the namespace because bwrap binds only a source that exists (ADR 0080 D17/D18)",
					CageSeatbelt, WallBwrap, s.Path, s.Why, err)
			}
			f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if os.IsExist(err) {
				continue
			}
			if err != nil {
				return Die("cage %s (%s) cannot prepare %s (%s): %v — the launcher makes this outside the namespace because bwrap binds only a source that exists (ADR 0080 D17/D18)",
					CageSeatbelt, WallBwrap, s.Path, s.Why, err)
			}
			_, werr := f.WriteString(seedBody(s.Path))
			if cerr := f.Close(); werr == nil {
				werr = cerr
			}
			if werr != nil {
				return werr
			}
		default:
			return Die("cage %s (%s): the render asked the launcher for %q on %s, which posse has no step for — refusing rather than binding a source nothing created (ADR 0080 D18)",
				CageSeatbelt, WallBwrap, string(s.Kind), s.Path)
		}
	}
	return nil
}

// BwrapPrefix is the prefix typed between the PATH assignment and the runtime
// command, SeatbeltPrefix's twin at this mechanism (ADR 0080 D18):
//
//	3<'<gates>/bwrap.args' '/usr/bin/bwrap' --args 3 -- <runtime cmd>
//
// WHY A FILE ON AN FD AND NOT THE ARGV. The argv is long — every grant, every
// deny and every mask of a crew seat's wall — and the line posse types has a
// hard limit it is already over (PaneLine spills every persona's line today).
// `--args` moves the whole wall off the line for the price of one redirection,
// and leaves the typed line the same length whatever the PID grants.
//
// A REDIRECTION BEFORE THE COMMAND NAME IS POSIX, and the line already
// assumes a POSIX pane shell: `PATH=…:"$PATH"` is a prefix assignment and the
// gates wrap types `"$(cat …)"`. Rex's seat shell is `realShell`'s answer,
// which is a bash or a zsh or nothing (rexShell, ADR 0077 D2 surface 4), so
// both multiplexers' panes parse it. fd 3 and not 0: stdin is the seat's tty.
//
// THE BINARY IS THE ABSOLUTE PATH RESOLVED AT RENDER, never the bare name.
// The line runs in the multiplexer's process tree carrying the seat's own
// PATH — which LEADS with the persona's gates dir (ADR 0009) and, on a box
// with mise or a user bin dir, holds whatever else that operator installed.
// A bare `bwrap` there is a name a shim can answer, and the shim would stand
// between the seat and its own wall. Resolved here, against the gates-free
// PATH, the wall is the file posse measured.
func BwrapPrefix(bin, args string) string {
	return "3<" + shellQuote(args) + " " + shellQuote(bin) + " --args 3 -- "
}

// bwrapBin resolves the binary BwrapPrefix types. A variable for the reason
// bwrapResolves and bwrapApplyRefusal above are variables: this is the third
// host measurement the mechanism depends on, and the arm it feeds is
// unreachable on the box that builds it, so a pin drives the Linux render
// from darwin by answering it.
var bwrapBin = liveBwrapBin

// liveBwrapBin is what production resolves with. protocolBin and not
// exec.LookPath, for ADR 0077 D3's reason and for BwrapPrefix's own: the
// gates-free PATH is the only lookup that cannot answer with a gate shim,
// and it is the same resolution `bwrapApplyProbe` measured the host with —
// so the binary the availability answer is about is the binary the launch
// line names.
func liveBwrapBin() (string, error) { return protocolBin("bwrap") }

// bwrapReadable is the `bwrap.txt` twin: the same argv, one element per line,
// under a header naming the renderer and the prep the launcher ran. do not
// edit — nothing reads this file back, and a hand-edit would describe a wall
// that is not up.
//
// THE HEADER IS THE POINT OF THE FILE. `RenderedByPosse()` is how a seat
// reading a wall tells "relaunch pending" from "install pending"
// (ranger-base-mmhhc: twelve caged launches read a superseded wall that named
// nobody), and the prep list is the only place an operator can see the writes
// posse made on the host in the seat's name.
//
// ONE ELEMENT PER LINE, UNQUOTED, and the header says so: that is exactly
// `tr '\0' '\n' < bwrap.args`, so a reader comparing the two files is
// comparing the same thing rather than this file's idea of it. A mount source
// holding a newline would break this copy and not the wall, which is the
// right way round — the args file is the artifact.
func bwrapReadable(persona, args string, argv []string, prep []PrepStep, absent []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# posse bwrap wall for %s — %s from the PID at launch; do not edit (ADR 0080 D18)\n", persona, RenderedByPosse())
	fmt.Fprintf(&b, "# The wall that RUNS is %s, NUL-separated. The launch line reads it on fd 3:\n", filepath.Base(args))
	fmt.Fprintf(&b, "#   %s<runtime command>\n", BwrapPrefix("<bwrap>", args))
	fmt.Fprintf(&b, "# Below is that argv one element per line, unquoted — `tr '\\0' '\\n' < %s`.\n", filepath.Base(args))
	fmt.Fprintf(&b, "# Nothing reads THIS file; it is here so the wall can be read.\n")
	if len(prep) > 0 {
		fmt.Fprintf(&b, "#\n# prep the launcher ran on the host, in this order, before the wall went up —\n")
		fmt.Fprintf(&b, "# bwrap binds only a source that exists, and the launcher is outside the namespace:\n")
		for _, s := range prep {
			fmt.Fprintf(&b, "#   %s   (%s)\n", s, s.Why)
		}
	}
	// The other half of the same sentence, and the only place it can be read
	// off the wall: a grant that was declared, is not on this host, and that
	// the launcher did NOT make (D23). Below the prep list, because the two
	// together are the whole of what the launcher did and did not do.
	if len(absent) > 0 {
		fmt.Fprintf(&b, "#\n# declared and NOT granted — no mount below names these, and the launcher\n")
		fmt.Fprintf(&b, "# did not create them (ADR 0080 D23):\n")
		for _, p := range shallowestFirst(absent) {
			fmt.Fprintf(&b, "#   %s\n", bwrapAbsentGrantLine(p))
		}
	}
	for _, a := range argv {
		b.WriteString(a)
		b.WriteString("\n")
	}
	return b.String()
}

// BwrapReport prints this mechanism's wall for `posse gates <persona>`,
// through SeatbeltReport's own markers over the same lists (ADR 0080 D18).
// WallReport (wall.go) is what chooses between the two.
//
// THE MARKERS ARE THE OTHER REPORT'S so an operator reads one wall and not
// two vocabularies: `w` a write grant, `x` a path the deny takes back, `r` a
// file-read mask, and `w (re-allowed)` for the audit logs bound back over the
// deny. Two of seatbelt's markers are absent because the mechanism has no
// such statement to make — `~` (the sibling namespace beside a granted FILE)
// and `+` as "create only" — and `+` is reused for the one thing this
// mechanism has and seatbelt does not: the keep file, which the launcher
// creates and no mount names.
//
// `·` CARRIES TWO STATEMENTS, and both of them are "this entry is in a list
// and renders no mount": the rename seal, which needs none (a mount is a
// statement about a mount POINT), and D23's declared-but-not-granted, which
// has no source to bind. One marker, because an operator scanning the block
// for what IS mounted wants one thing to skip and the parenthetical says
// which of the two it is.
//
// IT READS THE RENDERED ARGV AND THE PREP, not the lists, which is the
// difference that matters for a report whose whole job is to be checkable.
// `posse gates` is where ADR 0015 §2's claim is read off the output, and a
// report composed from the inputs would agree with the design while
// disagreeing with the wall. So the order below is the argv's order, which is
// the wall's semantics: later covers earlier.
func (a *App) BwrapReport(ag *AgentFile, rt *Runtime, cwd string, out io.Writer) error {
	args, r, err := a.renderBwrapWall(ag, rt, cwd)
	if err != nil {
		return err
	}
	set, keep, carve, prep := r.Set, r.Keep, r.Carve, r.Prep
	fmt.Fprintf(out, "  %s rendered for cwd %s (cage %s (%s); the mount set below, later covers earlier):\n",
		AbbrevHome(args), AbbrevHome(cwd), CageSeatbelt, WallBwrap)
	for _, w := range shallowestFirst(set) {
		fmt.Fprintf(out, "    w %s\n", AbbrevHome(w))
	}
	// The one render input that binds nothing, so it gets its own marker
	// rather than a "w" line an operator would read as a grant (D20).
	for _, f := range shallowestFirst(keep) {
		fmt.Fprintf(out, "    + %s (the launcher's keep file — an empty dot-file that stops an outside `git pack-refs --prune` from removing the bound directory above it; bound by nothing)\n", AbbrevHome(f))
	}
	// D23: declared, not granted, and not created. Printed with Seal's
	// marker — the one that already means "this entry is in a list and
	// renders no mount" — because that is exactly what it is.
	for _, p := range shallowestFirst(r.Absent) {
		fmt.Fprintf(out, "    · %s\n", bwrapAbsentGrantLine(p))
	}
	byRule := map[string][]string{}
	for _, d := range pidDeniedSubtrees(ag, cwd) {
		byRule[d.Path] = append(byRule[d.Path], d.Rule)
	}
	for _, p := range shallowestFirst(carve.Deny) {
		if r := byRule[p]; len(r) > 0 {
			fmt.Fprintf(out, "    x %s (read-only bind over the grant above — the PID's %s; ADR 0014 §3)\n", AbbrevHome(p), strings.Join(r, ", "))
			continue
		}
		fmt.Fprintf(out, "    x %s (read-only bind over the grant above; MNT_LOCKED, so a nested namespace can neither unmount it nor remount it rw — probe B6/B6b)\n", AbbrevHome(p))
	}
	// Seal is the one list of the four that renders nothing here, and it is
	// said out loud for the reason BwrapArgs says it: a renderer that
	// ignores an input looks exactly like a renderer that forgot it.
	for _, p := range carve.Seal {
		fmt.Fprintf(out, "    · %s (rename seal — NO mount: a mount is a statement about a mount POINT, so renaming it or an ancestor is refused by the kernel; probe B3/B4)\n", AbbrevHome(p))
	}
	for _, p := range shallowestFirst(carve.Keep) {
		fmt.Fprintf(out, "    w %s (re-allowed after the deny: the session's own audit trail, bound back as a FILE — an append to a bound file is in place)\n", AbbrevHome(p))
	}
	for _, p := range shallowestFirst(carve.DenyRead) {
		fmt.Fprintf(out, "    r %s (/dev/null bound over it — credential-store literal; ranger-base-hw18, ADR 0019 D2)\n", AbbrevHome(p))
	}
	// The prep, which an operator can read nowhere else: these are writes
	// posse makes on the HOST, outside the namespace, in the seat's name.
	for _, s := range prep {
		fmt.Fprintf(out, "    ! %s (prep, run by the launcher before the wall: %s)\n", s, s.Why)
	}
	// No siblings: a mount cannot name the scratch namespace beside a file,
	// and D19 makes it moot by granting claude's config DIRECTORY.
	a.reportConstitutionGrants(ag, out, set, nil, carve.Deny)
	return nil
}
