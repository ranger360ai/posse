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
	"slices"
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
	// PrepMkdirPrivate is PrepMkdirAll at mode 0700: the two mount points
	// whose OWNER would have made them private, and which this launcher now
	// makes first so that the mask covers the name before anything writes a
	// credential at it (ranger-base-bncbk — ~/.ssh at step 5a, rex's tsnet
	// state dir at step 5b).
	//
	// A SECOND KIND AND NOT A MODE FIELD, for PrepKind's own stated reason:
	// these print, and `mkdir -p -m 700 ~/.ssh` is a line an operator can
	// type and compare. A mode on PrepStep would be a number in a struct
	// that the readable copy would have to learn to render.
	//
	// 0700 IS NOT A HARDENING CHOICE, it is the mode the directory would
	// have had. ssh-keygen mkdirs ~/.ssh 0700, and rex's tsnet node does
	// `os.MkdirAll(rootPath, 0700)` (tailscale.com v1.104.4,
	// tsnet/tsnet.go:863 — the version /usr/bin/rex links, read 2026-11-21),
	// which is also how its missing parents are made. So a posse-made
	// directory at either name is indistinguishable from the one it
	// preempts, parents included — which is the whole of why creating it is
	// a cost the security lane could accept.
	PrepMkdirPrivate PrepKind = "mkdir -p -m 700"
	// PrepTouch creates the path as an empty FILE.
	PrepTouch PrepKind = "touch"
	// PrepSeed creates the path as a FILE holding its format's empty value
	// (seedBody), and only when nothing is there: it never rewrites a file
	// that exists (ranger-base-6akzw).
	PrepSeed PrepKind = "seed"
	// PrepMaskPoint creates the path as an EMPTY 0600 file when it is absent,
	// and leaves anything already there alone: the mount point of a /dev/null
	// mask (SeatbeltCarveOut.Masked, ranger-base-l751c), and both ends of a
	// private FILE bind (PrivateBind.File, ranger-base-a544g).
	PrepMaskPoint PrepKind = "mask point"
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
//
// A STEP IS EMITTED ONLY WHERE ITS PATH IS ABSENT, and that is relied on
// twice over. runBwrapPrep says why it matters for a `touch` (it can never
// truncate a live seat's `refusals.log`), and `prepRequire` reads the list's
// EMPTINESS as a verdict: a render that asked for no step is a wall whose
// every bind source is already on this host, so a reader may apply it
// without writing anything (wallPrep, wall.go — ranger-base-qd9ei). Every
// emission site below is guarded on a stat; one that stops being is a probe
// that silently starts measuring a wall it cannot put up. The pin is
// TestQAEveryBwrapPrepStepNamesAPathThatIsNotThere, and it holds the FACT
// rather than a count of the sites — of each step, that its path is not
// there; and of the same render after the prep has run, that it asks for
// nothing.
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
	var watch []WallWatchMount
	return bwrapArgsInto(&watch, cwd, writable, carve, refKeep...)
}

// bwrapArgsWatched is BwrapArgs with its third output: every mount it put
// over a name that is a WALL — Hide (the runtime dir's tmpfs and the seat's
// /tmp), Private, Masked, Deny and ReadMask (step 5's ssh dir tmpfs, its
// credential-store tmpfs and what is left of its /dev/null credential
// binds) — in render order (ranger-base-0qpsw D1, the read masks added by
// ranger-base-w6ip0's decision as ranger-base-y4cm6, the store tmpfs by
// ranger-base-a8qfg).
// `posse wall-watch` holds the seat to that list: a host rename or unlink
// over any of those names detaches the mount in every namespace, and the
// seat would then read and write the host's object at that name.
//
// A READ MASK IS A WALL AND ITS COST IS ACCEPTED. A credential file that a
// host tool refreshes by atomic rename detaches the /dev/null over it, so
// every live seat that masks that credential is killed at each refresh.
// MEASURED 2026-11-13 by stat only: claude's credentials file has birth and
// mtime 1 ms apart after a refresh, so the refresher does write a new
// inode.
//
// SINCE ranger-base-a8qfg the three sibling stores are masked as
// DIRECTORIES (step 5c), and a refresh inside a masked directory is no
// mount event at all. Residual: ranger-base-1c0r6. The ssh tmpfs is a
// directory too, and the keys under it render no mask of their own.
//
// THE OPENINGS ARE NOT IN IT. A write grant, a Keep file, an ssh allow bind
// re-OPEN something, so a detached one leaves the seat with less than it
// was given and never more — the wall underneath is still there. Recording
// them would turn an operator's `rm -rf` of a stale cache into a P1. That
// is why the config and known_hosts bound back UNDER the ssh tmpfs are not
// watched while the tmpfs itself is: the tmpfs is the wall, those two are
// the holes deliberately left in it.
//
// The list is taken HERE, beside the argv it describes, and not re-derived
// from the argv later: a destination in the argv does not say whether it
// is a wall or an opening, and the render is the one place that knows.
//
// SO THE TAXONOMY ABOVE IS NOT WHAT HOLDS THE LIST TOTAL, and a reader
// should not take it for one (ranger-base-mxbq0 finding 2): a kind added to
// this render with no watch() call would leave both paragraphs reading
// complete. What holds it is a pin that derives the expectation from the
// ARGV — every destination this function mounts over is either in the watch
// list or in an opening set computed from the INPUTS, and every watched path
// is a destination the argv carries. It can do what this function cannot
// because it holds both ends: the render sees only paths, the pin was handed
// the lists they came from. TestWallWatchEveryRenderedMountIsAWallOrANamed
// Opening in wallwatch_test.go, with TestWallWatchFixturesCarryEveryCarveOut
// Field holding its inputs total over SeatbeltCarveOut. A new mount shape
// here reds one of the two; adding it to the taxonomy above reds nothing.
func bwrapArgsWatched(cwd string, writable []string, carve SeatbeltCarveOut, refKeep ...string) ([]string, []PrepStep, []WallWatchMount, error) {
	var watch []WallWatchMount
	argv, prep, err := bwrapArgsInto(&watch, cwd, writable, carve, refKeep...)
	if err != nil {
		return nil, nil, nil, err
	}
	return argv, prep, watch, nil
}

func bwrapArgsInto(watchp *[]WallWatchMount, cwd string, writable []string, carve SeatbeltCarveOut, refKeep ...string) ([]string, []PrepStep, error) {
	watch := func(kind, p string) { *watchp = append(*watchp, WallWatchMount{Kind: kind, Path: p}) }
	// 1. The base. --die-with-parent so a seat's wall dies with the pane
	// that holds it; --ro-bind / / because the default is to deny writes
	// everywhere, which at this mechanism is spelled as a read-only mount of
	// the whole tree; --dev-bind /dev /dev for seatbelt's own
	// (regex #"^/dev/") grant, which every runtime needs for its tty.
	//
	// What is deliberately PRESENT: --unshare-user, explicitly and not left
	// to whatever bwrap decides for itself (ranger-base-racfc). The seat must
	// sit in a child user namespace, because the /proc/<host pid>/root guard
	// (ranger-base-di8da) depends on it, and a guard whose premise is an
	// implicit choice of the bwrap build is a guard nobody can read off the
	// render. No --uid/--gid: the identity map stays, so the seat's own uid
	// is the one it owns files as.
	//
	// What is deliberately absent, each because the seat needs the host's
	// own (D17, and Alternatives rejected): --unshare-pid (a seat kills its
	// own pids and `go run ./cmd/checkorphans` reads the box's process
	// table), --unshare-net (egress is the container tier's), --proc,
	// --new-session (setsid would detach the TUI from its controlling
	// terminal). The host's /tmp IS replaced, by a bind of the seat's own
	// directory rather than a tmpfs (1c below, ranger-base-7wdb7).
	//
	// --proc HAS A SECOND REASON SINCE ADR 0096 D2, and that ADR is where a
	// bead reaching for it has to argue. D17 left it out to keep the host's
	// pid namespace; ADR 0096 priced putting it back and decided the render
	// does not change, so the absence is now a rejected proposal and not only
	// an omission. The consequence to read off THIS line: because /proc stays
	// the host's procfs under `--ro-bind / /`, a nested user namespace cannot
	// write its own uid_map, so nothing forked inside a seat can nest a bwrap
	// or an `unshare -r` (ADR 0096 D1, its table arm A). A runtime's own
	// sandbox is therefore a declared absence here — the bwrap wall is the
	// file wall — and the live pins that exec a bwrap abstain inside a seat
	// rather than failing (D3, bwrapNestSkip in wallvenue_test.go).
	//
	// --bind /proc /proc IS THE SAME ABSENCE, spelled the way a later bead
	// would more likely spell it. With the pid namespace shared, bwrap
	// 0.13.1's --proc /proc mounts no new procfs: it produces a read-write
	// recursive bind of the host's, the same device and byte-identical
	// mountinfo lines (ADR 0096's table, arms B and D). Either spelling opens
	// five /proc/<pid> knobs on same-uid HOST processes to every seat, one of
	// which (coredump_filter) decides what another process's crash writes
	// out. TestBwrapArgsNeverUnsharesWhatTheSeatNeeds holds both.
	argv := []string{
		"--die-with-parent",
		"--ro-bind", "/", "/",
		"--dev-bind", "/dev", "/dev",
		"--unshare-user",
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
	// THE LIST IS MORE THAN THE RUNTIME DIR (ranger-base-s7tkc): the host's
	// crash stores ride it too — systemd-coredump's directory and the
	// journals, where a same-uid peer's core and environment sit readable to
	// every seat. Same loop, same rule: absent is skipped, a directory gets
	// an empty tmpfs (liveBwrapRuntimeDirs, bwrapCrashStoreDirs).
	//
	// The three variables go with it, because an address can name an
	// endpoint OUTSIDE the hidden dir (an abstract socket, an agent socket
	// under /tmp), and a seat has no use for any of them.
	for _, k := range bwrapUnsetEnv {
		argv = append(argv, "--unsetenv", k)
	}
	hidden := shallowestFirst(carve.Hide)
	var hideMounted []string // the Hide entries 1b actually mounted (5c's skip)
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
		watch(WallWatchHide, h)
		hideMounted = append(hideMounted, h)
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
	//
	// THE SOURCE GOES THROUGH THE COMPONENT WALK, not through bwrapStat
	// (ADR 0100 D1, the non-race half of ranger-base-p47q6). bwrapStat is
	// os.Stat and os.Stat FOLLOWS: while this source lived under the seat's
	// own cache dir — a path the seat is granted by name — a seat that
	// replaced it with a link had the next launch bind the link's target
	// read-write over /tmp, with no window to hit and no mkdir of its own to
	// give it away (MEASURED, ranger-base-p47q6). The source is under the
	// seat's WALL dir now, which no render grants, and the walk is the same
	// one every private bind's source gets: a link at the source refuses the
	// render and emits NO prep step, because the prep runs in the uncaged
	// launcher and a link here would aim its 0755 mkdir.
	if carve.Tmp != "" {
		exists, fi, err := bwrapSrcWalk(carve.Tmp, bwrapSeatTmpRoot(carve.Tmp), bwrapTmpMount)
		if err != nil {
			return nil, nil, err
		}
		dir := exists && fi.IsDir()
		switch {
		case exists && !dir:
			return nil, nil, Die("bwrap cannot bind %s over %s: it is not a directory (a link, a file or a special file is there) — remove it and relaunch (ranger-base-7wdb7, ADR 0100 D1)", carve.Tmp, bwrapTmpMount)
		case !exists:
			prep = append(prep, PrepStep{PrepMkdirAll, carve.Tmp, "the seat's private /tmp, bound over the host's so no seat writes where uncaged builds stage what they execute"})
		}
		argv = append(argv, "--bind", carve.Tmp, bwrapTmpMount)
		watch(WallWatchHide, bwrapTmpMount)
		hidden = append(hidden, bwrapTmpMount)
		hideMounted = append(hideMounted, bwrapTmpMount)
		kept := writable[:0:0]
		for _, w := range writable {
			if absResolve(w) != bwrapTmpMount {
				kept = append(kept, w)
			}
		}
		writable = kept
	}

	// 2a. NO WRITE GRANT LIES AT OR ABOVE THE WALL ROOT (ADR 0100 D2(a)).
	// Asked of the set this render is about to BIND — after 1c took the
	// grants the private /tmp replaces — and asked by every render in this
	// package, because every one of them comes through here: bwrapRender,
	// the landed-check judge and the wall venue.
	if err := bwrapRefuseWallRootGrant(writable); err != nil {
		return nil, nil, err
	}

	// 2. Shape 1: the write grants, shallowest first so a nested grant's
	// mount lands on top of the one that contains it — plus the directories
	// between a nested grant and the grant above it, bound in place so the
	// mount pins them (bwrapGrantMounts, ADR 0100 D2(b)).
	grants := shallowestFirst(writable)
	// reopens answers whether a later mount of p would bind a HOST path back
	// into a hidden directory that no write grant re-opened. A deny or a mask
	// is a `--ro-bind` from the host, and a read-only bind of a socket is
	// still a socket a seat can connect to — so inside a hidden directory
	// the tmpfs is already the stronger statement and the mount is skipped.
	reopens := func(p string) bool { return writeGranted(hidden, p) && !writeGranted(grants, p) }
	for _, m := range bwrapGrantMounts(grants) {
		p := m.Path
		exists, dir, err := bwrapStat(p)
		if err != nil {
			return nil, nil, bwrapStatRefusal(m.what(), p, err)
		}
		if m.Pin {
			// A PIN is a directory of its grant's own, so the two arms
			// below are not its arms. A file at that name means the grant
			// under it cannot exist and cannot be made — the mkdir in the
			// grant's own arm would fail at the launcher, after the render
			// said yes — so it is refused here, where the message can say
			// which grant asked for it.
			switch {
			case exists && !dir:
				return nil, nil, Die("bwrap cannot pin %s for the write grant %s below it: it is not a directory, so there is no grant under it to pin and no mkdir that could make one (ADR 0100 D2(b)). Narrow the grant rather than widening the wall", p, m.For)
			case !exists:
				prep = append(prep, PrepStep{PrepMkdirAll, p, "a directory between a write grant and the grant that contains it; bwrap binds it in place so the mount pins the grant below it (ADR 0100 D2(b))"})
			}
			argv = append(argv, "--bind", p, p)
			continue
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

	// 2d. WHERE THE CARVE-OUT'S OWN PINS GO, and that is the whole of what
	// this mark is: the pins themselves are composed at step 5d, when every
	// mount is known, and spliced back in HERE (bwrapCarvePins,
	// ranger-base-a59w7). Two indices and not a third list, because what
	// matters about them is their POSITION and a reader who finds them at
	// the end would have to reconstruct it.
	//
	// AFTER THE GRANTS AND BEFORE EVERY OTHER MOUNT OF THIS RENDER. That is
	// the ordering argument, and it has three halves:
	//
	//   - a pin cannot shadow the statement it exists for. A deny is a
	//     `--ro-bind` and a mask is a `/dev/null` MOUNTED INSIDE the
	//     directory the pin binds, so a pin emitted after one would cover
	//     the name again with the host's own directory and take the wall
	//     off. Everything from 2a-bis down lands on top of these.
	//   - nor any OTHER wall it happens to contain. 2c's private members are
	//     mounts over paths inside the same grants, and a pin above one of
	//     them emitted after it would bury a wall the seat's siblings are
	//     held out by. Here it lands first and the private mount lands on
	//     top, as every narrower statement in this render does.
	//   - and it cannot widen anything, wherever it lands. Every path in the
	//     list lies STRICTLY INSIDE a write grant, and that grant was bound
	//     in place one loop up — so the host's directory at that path is
	//     already read-write in this render and the pin adds no byte of
	//     reach. The only mounts BEFORE it are step 1's base and the 1b/1c
	//     hides, and a hide inside a write grant is already shadowed by that
	//     grant's own in-place bind at step 2, so a pin under one shows the
	//     seat nothing the grant did not.
	//
	// The prep mark is the same decision at the launcher: a pin whose
	// directory is not there yet needs its `mkdir -p` to run before the
	// mask point or keep file the launcher creates INSIDE it, and
	// PrepMaskPoint and PrepTouch make no parents of their own.
	carveAt, carvePrepAt := len(argv), len(prep)

	// 2a-bis. THE PRIVATE /tmp SOURCE, PINNED WHERE IT LIES INSIDE A GRANT
	// (ADR 0100 D2(b)). A seat's source is under the wall dir and inside no
	// grant, so this renders nothing for it. The LANDED-CHECK judge and the
	// WALL VENUE are the shape it exists for: their private /tmp is `tmp`
	// inside the run dir, and the run dir is the one thing they grant — so
	// the component `<run>/tmp` is strictly inside a write grant and is no
	// mount of theirs, which is the predicate's definition of loose. ADR
	// 0100 asserted those two renders passed D2(b) as they stood; MEASURED
	// here while building it, they did not, and the ADR's own remedy for a
	// miss is this one: an in-place bind, never an exemption.
	//
	// IT WIDENS NOTHING, which is why it is conditional on the grant rather
	// than emitted always: a read-write bind in place makes the path
	// writable, and the only paths this fires for are ones a grant already
	// made writable. Where no grant covers the source (every seat render)
	// the same line would be handing out a write the render never meant.
	//
	// IT IS NOT WATCHED. A `posse wall-watch` entry exists to kill a seat
	// whose wall came off; this mount is no wall over the seat's reach —
	// the directory is writable either way — and its one job is to refuse a
	// RENAME of the name from inside the namespace, which affects the NEXT
	// render and not this run. The host can still rename it (a holder gets
	// EBUSY, a non-holder does not — ADR 0100's Context, MEASURED), and
	// killing the victim would not undo that.
	if carve.Tmp != "" && bwrapStrictlyInsideGrant(carve.Tmp, grants) {
		argv = append(argv, "--bind", carve.Tmp, carve.Tmp)
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

	// 2c. The seat's private copies of shared members (ranger-base-5632o;
	// claudePrivateSessionState says which and why). Each one is a bind of a
	// per-seat host dir OVER a directory inside a write grant. It comes after
	// the grants, so it lands on top of the grant that holds it, and before
	// the denies, which name other members and never these.
	//
	// claude's projects/ rides the same slot with binds IN PLACE (Src ==
	// Dst; claudeProjectsBinds, ranger-base-ajkqx): read-only over the grant,
	// the seat's own key dir read-write on top, its memory/ read-only on top
	// of that. List order is mount order, so each narrows the one before.
	//
	// THE DESTINATION IS LSTAT'D, NOT STAT'D. It sits in a dir every seat can
	// write, so a symlink there could have been planted by another seat, and
	// bwrap would mount on wherever it points. A destination that is not a
	// real directory refuses the render. That lets a hostile seat stop the
	// next launch, which is loud. Following the link would hand it a mount
	// point of its choosing, which is silent.
	//
	// OUTSIDE A GRANT IT RENDERS NOTHING. The read-only base already refuses
	// every write there, and a grant can drop out of the set before this
	// point (D23: a no-create grant this host does not have). That check
	// comes AFTER the lstat, because writeGranted resolves its argument, and
	// a link there would resolve outside the grant and be skipped, not
	// refused.
	// FOUR ARMS, ONE WATCH (ranger-base-4tsa7). bwrapPrivateArgs renders the
	// entry — the File bind, the dir bound in place, the empty tmpfs, the dir
	// copy — and the `watch` call is made HERE, once, over whatever it
	// returned, because every one of those mounts is a wall and the arm that
	// made it does not change that. bwrapPrivateArgs' head comment has what
	// the per-arm watch cost us and why an arm that renders nothing says so
	// with a bool.
	for _, pb := range carve.Private {
		add, steps, ok, err := bwrapPrivateArgs(pb, grants)
		if err != nil {
			return nil, nil, err
		}
		prep = append(prep, steps...)
		if !ok {
			continue
		}
		argv = append(argv, add...)
		watch(WallWatchPrivate, pb.Dst)
	}

	// 2e. AND WHERE THE MOUNTS THOSE PINS ARE FOR BEGIN (ranger-base-a59w7).
	// Step 5d reads its candidates out of the argv from here down, which is
	// the deny, keep and mask producers and nothing above them.
	//
	// WHY NOT SIMPLY EVERY MOUNT AFTER THE GRANTS. The two producers between
	// 2d's mark and this one each pin their own loose component already and
	// are named for doing it — 2a-bis for the private /tmp source, and
	// runtimeEphemeralDirParents, which bwrapRender hands in as an in-place
	// Private entry, for the CLI homes' ephemeral dirs. A general list that
	// swallowed them would leave two decisions realized in a place neither
	// one's own pin can see, and the pin that says the ephemeral parent is
	// bound in place works by taking that entry OUT of the carve-out and
	// asking for the refusal (TestQAWallPinnedDstTheCensusMissParentIsBound
	// InPlace): a list that re-emitted it would answer that pin green with
	// the producer gone. Retiring either one in favour of this list is a
	// bead of its own, not a side effect of the one that needed the denies
	// pinned. The SPLICE stays at 2d all the same, because where these
	// mounts land is a different question from which of them is loose.
	denyAt := len(argv)

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
		if carve.Masked(d) {
			// A MASKED deny is covered with /dev/null, never with itself
			// (ranger-base-l751c, claudeManagedCache): what is at the name
			// is an answer a later reader obeys, so the seat must neither
			// write it nor read a copy someone else planted there. The
			// source is the host's /dev/null, which always exists.
			//
			// The NAME must be a regular file or nothing. bwrap follows a
			// symlink at the destination, so a mount "over" a planted link
			// lands on its target and leaves the name a seat can rename
			// over; a directory cannot take a file mount at all. Both are
			// refused, in the tier's fail-closed direction.
			fi, err := os.Lstat(d)
			switch {
			case err == nil && !fi.Mode().IsRegular():
				return nil, nil, Die("bwrap cannot mask %s: it is a %s, not a file, and a mount over it would not cover the name (ranger-base-l751c). Remove it on the host; claude recreates it as a file", d, fi.Mode().Type())
			case err != nil && !os.IsNotExist(err):
				return nil, nil, bwrapStatRefusal("mask", d, err)
			case err != nil:
				if !writeGranted(grants, d) {
					continue // nothing inside the wall can bring this path into being
				}
				// bwrap would make the mount point itself, 0444, and the
				// host keeps it. claude rewrites these caches in place
				// (O_TRUNC, no rename), so a 0444 file would stop the
				// operator's own claude caching for good. An empty 0600
				// file is what claude itself would create, and reads as
				// no stored copy (MEASURED, claudeManagedCache).
				prep = append(prep, PrepStep{PrepMaskPoint, d, "a cached server answer that is not there yet and sits inside a write grant; created empty (no answer) so /dev/null can be mounted over the name"})
			}
			argv = append(argv, "--ro-bind", "/dev/null", d)
			watch(WallWatchMasked, d)
			continue
		}
		exists, _, err := bwrapStat(d)
		if err != nil {
			return nil, nil, bwrapStatRefusal("deny", d, err)
		}
		if exists && !writeGranted(grants, d) && !grantInside(grants, d) {
			// OUTSIDE EVERY GRANT IT RENDERS NOTHING, the private-member
			// arm's rule for that arm's reason (ranger-base-166va): the
			// read-only base already refuses every write there, so a
			// read-only bind of the name over itself adds no wall — and it
			// adds a mount over a NAME, which a host rename detaches. The
			// shared repo's `<common>/config` is the case that found it: a
			// linked worktree's seat is granted objects, logs, its own
			// worktree dir and its ref dir and never the common dir's top,
			// yet every hire, reap and land check rewrites that config by
			// lock-and-rename, the deny's mount came off, and wall-watch
			// killed a seat that could not have written the file either way.
			//
			// BOTH DIRECTIONS ARE ASKED. A deny that CONTAINS a grant
			// (a deny over a dir whose child an earlier bind opened) is
			// still rendered: its mount lands on top and closes that child,
			// which is last-match-wins and the narrowing the carve-out
			// means, and dropping it would widen the wall.
			continue
		}
		// AND THE OTHER SHAPE THE SKIP ABOVE LEAVES: a `.git/config` deny
		// that a write grant CONTAINS (ADR 0095 D1). The arm above covers
		// the linked worktree, where no grant reaches the common dir's top
		// and nothing is rendered. In a main checkout the grant is the cwd
		// and it holds the whole `.git`, so the same deny is a mount inside
		// a writable directory — and every host writer in ADR 0095's table
		// replaces that file by rename, which detaches the mount and makes
		// `posse wall-watch` kill the seat (ranger-base-0qpsw D2). The kill
		// is CORRECT here, because after the detach the seat can write
		// `.git/config` itself and `core.hooksPath`, `core.fsmonitor` and
		// `core.sshCommand` in it are commands a later unsandboxed git runs.
		// So there is nothing to repair in the render: a seat in this shape
		// dies at the next hire, reap or landing check in its own repo, and
		// the remedy is a tree whose grant does not contain the file.
		//
		// REFUSED AT THE RENDER AND NOT AT A LAUNCH PATH, which is D1's own
		// sentence: `posse new`, a recipe, dispatch's detached-HEAD fallback
		// and whatever calls this next all meet it at one place, in the
		// tier's fail-closed direction, rather than each growing a check of
		// its own.
		//
		// THE SESSION DIR'S OWN GIT DIR, and that half of the key is the
		// ADR's Context sentence verbatim — "the seat's grant is the cwd,
		// which contains the whole `.git`" — rather than any grant that
		// happens to lie above some repository. MEASURED 2026-11-14
		// (ranger-base-hi2lt): keyed on containment alone this arm also
		// refused five renders whose seat is in a LINKED worktree, because
		// `bwrapSeatTmp` answers "" when the home or the session dir is
		// itself under /tmp — every fixture in this package is — and with no
		// private /tmp the render keeps the blanket `/tmp` write grant, which
		// then contains the main checkout's `.git/config` too. That refusal
		// is not WRONG (through that grant the seat really can write the
		// file), but it is not this shape, and no production launch has it:
		// homes and worktrees live under the home, so `carve.Tmp` is set and
		// `bwrapDropTmpGrants` takes /tmp out of the set. A main checkout
		// whose `.git` is writable through some OTHER grant — a PID
		// `writable:` extra naming the repo root — is the residual, stated
		// rather than closed, and bwrapGitConfigRepo is where it joins.
		//
		// AND THE WRITABILITY IS ASKED OF THE SEAT'S OWN GRANTS, for the
		// same measurement read from the other end. A grant strictly ABOVE
		// the session dir is not the cwd grant D1 is about, and taking it as
		// one refuses a shape that is fine in production: a whole-tree-deny
		// PID in a main checkout holds no grant on the cwd at all (ADR 0086
		// D3 — only `<cwd>/.beads`), so nothing of its own can write
		// `.git/config` and there is no wall to take off; under the fixture
		// /tmp grant above, the blanket entry said otherwise and refused it
		// (TestQAWallWitnessTakesTheRowAwayWhereTheWallAllowsTheCreate). So
		// the question is put to the grants at or under the cwd, which is
		// what "the seat's grant is the cwd" means and all a main-checkout
		// seat has.
		own := grants[:0:0]
		for _, g := range grants {
			if underDir(cwd, g) {
				own = append(own, g)
			}
		}
		if exists && writeGranted(own, d) && bwrapGitConfigRepo(d) == absResolve(cwd) {
			return nil, nil, Die("bwrap cannot wall %s inside a writable %s: every host git config write (each dispatch hire and reap in this repo, the landing check, the operator's own `git config` / `branch -D`) replaces that file by rename, which takes the wall off, and wall-watch then kills the seat (ranger-base-0qpsw D2, ADR 0095). Run this seat in a linked worktree: dispatch it, or launch it from one", d, filepath.Dir(d))
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
			// (A Masked deny never reaches this arm: its empty mount point
			// is made above, ranger-base-l751c.)
			//
			// THE ONE FILE-SHAPED DENY THAT IS MADE HERE is a Seeded one
			// (ranger-base-6akzw): claude's executable config, where an empty
			// file is as inert as none to the CLI that reads it (seedBody) and
			// the residual above is not — a created settings.json is read by
			// every other claude sharing the directory, uncaged ones included.
			// The launcher writes the empty value and the mount goes over it.
			// It was NOT inert to posse's own project-config classifier when a
			// session starts in the home these steps write to
			// (SeatbeltCarveOut.Seeded's second paragraph, ranger-base-f67jx),
			// and the mkdir arm below had the same exposure. Neither is fixed
			// here: ADR 0091 D1 took the classifier's side of it instead, so
			// nothing in this step list changed and nothing in it needs to
			// (ProjectConfigTrust, parity.go).
			switch {
			case carve.Seeded(d):
				prep = append(prep, PrepStep{PrepSeed, d, "executable config that is not there yet and sits inside a write grant; created empty so the read-only mount can cover it"})
			case carve.FileShaped(d):
				continue
			default:
				prep = append(prep, PrepStep{PrepMkdirAll, d, "a deny that is not there yet and sits inside a write grant, so the session could create it"})
			}
		}
		// AN ANCHORED DENY IS NEVER EMPTY ON THE HOST (ranger-base-v84cn).
		// A host rmdir of the empty directory under this bind takes the
		// bind off the seat, and wall-watch then kills it. The seed refuses
		// that rmdir (ENOTEMPTY), whether the dir was made above or was
		// already there. It is O_EXCL, so an anchor that is already there is
		// left alone. A name that is not a directory gets no anchor: the
		// seed could not land, and nothing rmdirs a file.
		//
		// The `!exists || dir` guard this used to carry read its answer out
		// of bwrapStat, which follows a link; bwrapAnchorStep lstats
		// (ranger-base-ua625).
		if carve.Anchored(d) {
			step, ok, err := bwrapAnchorStep(d, "an empty file inside a read-only deny directory that a host process removes when it finds it empty; a host rmdir of the dir would take the mount off the seat")
			if err != nil {
				return nil, nil, err
			}
			if ok {
				prep = append(prep, step)
			}
		}
		argv = append(argv, "--ro-bind", d, d)
		watch(WallWatchDeny, d)
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
	// A MISSING ONE RENDERS NOTHING, which was ADR 0080's snapshot-at-launch
	// residual: another runtime's credential file created AFTER the seat
	// started was readable by it, with no mount for `posse wall-watch` to
	// see. 5c CLOSES THAT for every sibling credential STORE by masking the
	// directory instead (ranger-base-a8qfg). Residual: ranger-base-1c0r6.
	// The report names a mask that renders nothing. The seat still cannot write at
	// any of them, because nothing grants that runtime's state dir.
	//
	// 5a AND 5b CLOSE IT FOR THE OTHER TWO DIRECTORY MASKS
	// (ranger-base-bncbk): ~/.ssh and a read-masked state dir are made when
	// they are absent, so the whole of shape 5 is now "the mask covers the
	// name before anything writes a credential at it", with the single
	// exception the 5c loop states for a store inside a write grant. The
	// snapshot residual survives only at the FILE masks one loop down, where
	// a mount needs a name that exists and posse will not invent another
	// runtime's credential file.
	//
	// B5 measured that no byte of the credential reached the seat, and
	// measured it by an error rather than by the clean EOF this shape is
	// stated in terms of; ranger-base-yca4g is the re-run that reads which
	// error. Nothing here depends on which it is — the claim this render
	// makes is "the read gets no bytes".
	//
	// 5a. ~/.ssh first, as an ALLOWLIST (ranger-base-12ggv): an empty tmpfs
	// over the directory, then ssh's config and known_hosts bound back
	// read-only. The key list in DenyRead is ssh's own resolution, so a key
	// no config names was readable; under the tmpfs nothing in the directory
	// is, named or not. After the grants and denies so nothing mounted
	// earlier lands on top of it, and the shape is 1b's, at a read wall.
	//
	// A FILE at ~/.ssh refuses the render (a tmpfs covers only a directory,
	// and a mask that silently renders nothing is the failure this tier
	// refuses). One inside a directory the seat already sees empty (1b/1c)
	// renders nothing: the hide is the stronger statement, and a mount there
	// would make its mount point on the host. An ABSENT one is MADE — the
	// `!exists` arm below says why, and it is the larger half of
	// ranger-base-bncbk.
	var readHidden []string
	for _, d := range shallowestFirst(carve.SSHDir) {
		if reopens(d) {
			continue
		}
		exists, dir, err := bwrapStat(d)
		if err != nil {
			return nil, nil, bwrapStatRefusal("ssh dir", d, err)
		}
		switch {
		case exists && !dir:
			return nil, nil, Die("bwrap cannot hide %s: it is not a directory, and an empty tmpfs can only be mounted over one (ranger-base-12ggv)", d)
		case !exists:
			// MADE WHEN IT IS ABSENT, and until ranger-base-bncbk this arm
			// skipped instead — the hole, and the larger half of that bead.
			// The old reason was "a seat cannot create it either — nothing
			// grants its parent", which is true and beside the point: the
			// HOST creates it. Fresh box, no ~/.ssh, seats running, the
			// operator runs ssh-keygen to set up a push key; ssh-keygen
			// mkdirs ~/.ssh and writes id_ed25519, and every live bwrap seat
			// then reads the private key through `--ro-bind / /`. No mount
			// came off, so `posse wall-watch` says nothing, and the exposure
			// lasts until each seat ends. It is ADR 0080's
			// snapshot-at-launch residual at this arm, and it is closed the
			// way 5c closed it for the credential stores: make the mount
			// point, so a key written during the seat's life lands INSIDE
			// the tmpfs where no seat sees it.
			//
			// MEASURED 2026-11-21 in the wall venue (linux 7.2.15, bwrap
			// 0.13.1), both directions, by
			// TestLiveBwrapAbsentSSHDirMasksAKeyTheHostWritesAfterLaunch: a
			// seat launched with no ~/.ssh, the host writing the key only
			// after the seat said its namespace was up. With this arm
			// skipping, the seat reads the key in full; with it making the
			// directory, no bytes — and the pin's unmasked control arm reads
			// the key, so the green is this wall and not a probe that could
			// never read.
			//
			// 0700 because that is the mode ssh-keygen gives a ~/.ssh it
			// makes itself, so this is not a directory the operator can tell
			// from the one it preempts (PrepMkdirPrivate). ssh ignores the
			// anchor beside it: it reads the names it resolves, and
			// StrictModes checks modes and not extra names.
			prep = append(prep, PrepStep{PrepMkdirPrivate, d, "the ssh directory this seat reads nothing in, which is not there yet; made empty 0700 — ssh-keygen's own mode — so the mask covers the name before a key exists at it and a key written during this seat's life lands inside the tmpfs"})
		}
		// AND IT IS ANCHORED, which is 5c's call and step 3's measurement
		// (ranger-base-bncbk, deciding ranger-base-lin9y). This arm leaves
		// its host directory empty now — the case above makes it so, and an
		// already-EMPTY ~/.ssh was always in the shape — so it is in the
		// class exactly: a mount point this launcher leaves empty is one
		// host rmdir from detaching. The detach was already loud here
		// (watch(WallWatchReadMask) below is a wall-watch kill plus a P1
		// ROTATE bead); the anchor turns detect-and-kill into prevent, for
		// the price of one empty dotfile in the operator's ~/.ssh, and the
		// security lane accepted that price on lin9y. A host `rm -rf ~/.ssh`
		// of a NON-empty directory is outside the anchor class and wall-watch
		// remains the control for it, as it does for every other arm.
		astep, aok, aerr := bwrapAnchorStep(d, "an empty file inside the ssh directory this render covers with a tmpfs and leaves empty on the host; a host rmdir of an empty ~/.ssh would take that mask off the seat and show it a key written after launch")
		if aerr != nil {
			return nil, nil, aerr
		}
		if aok {
			prep = append(prep, astep)
		}
		argv = append(argv, "--tmpfs", d)
		watch(WallWatchReadMask, d)
		readHidden = append(readHidden, d)
		for _, f := range carve.sshDirAllowed(d) {
			exists, dir, err := bwrapStat(f)
			if err != nil {
				return nil, nil, bwrapStatRefusal("ssh allow", f, err)
			}
			if !exists || dir {
				continue // a directory at that name is not a file ssh reads, and binding it would reopen a tree
			}
			argv = append(argv, "--ro-bind", f, f)
		}
	}
	// 5b. DenyReadDir (ranger-base-k06gw): 5a's tmpfs with nothing bound
	// back, refused on the same terms.
	//
	// AN ABSENT ONE IS MADE ONLY WHERE ITS PRODUCER SAYS SO
	// (carve.MakesDenyReadDir, ranger-base-bncbk) — which is 5a's new rule
	// with the one difference this list forces. The mask covers every path
	// that resolves, on every GOOS, because a mask wider than its runtime's
	// own resolution is the safe direction; rexNodeStateDirs therefore lists
	// both OSes' XDG defaults under both homes, and posse must not mkdir
	// ~/Library/Application Support/rex/tsnet on linux. So the native path
	// is created and anchored, and the rest skip when absent: nothing on
	// this OS writes there.
	//
	// The hole this closes is 5a's, one runtime along: rex's tsnet node
	// writes the tailnet identity on its FIRST start, and a rex that first
	// started during a seat's life wrote it at a name no mount covered and
	// no watch had ever seen.
	for _, d := range shallowestFirst(carve.DenyReadDir) {
		if reopens(d) || writeGranted(readHidden, d) {
			continue
		}
		exists, dir, err := bwrapStat(d)
		if err != nil {
			return nil, nil, bwrapStatRefusal("read-deny dir", d, err)
		}
		switch {
		case exists && !dir:
			return nil, nil, Die("bwrap cannot hide %s: it is not a directory, and an empty tmpfs can only be mounted over one (ranger-base-k06gw)", d)
		case !exists && !carve.MakesDenyReadDir(d):
			continue // another OS's default, or another home's: nothing here writes it
		case !exists:
			// 0700 and its parents with it, which is what rex's own tsnet
			// node does to this very path (PrepMkdirPrivate names the line).
			// An empty 0700 directory is what tsnet expects to find: it
			// MkdirAll's the dir and then Stats it for IsDir, and reads no
			// directory listing of it at all, so a pre-existing empty one
			// plus the anchor beside it is inert to the node that fills it.
			prep = append(prep, PrepStep{PrepMkdirPrivate, d, "the state dir this runtime writes a credential into on its first start, which is not there yet; made empty 0700 — the runtime's own mode — so the mask covers the name before the credential exists at it"})
		}
		astep, aok, aerr := bwrapAnchorStep(d, "an empty file inside a read-masked state dir this render covers with a tmpfs and leaves empty on the host; a host rmdir of the empty dir would take that mask off the seat and show it a credential written after launch")
		if aerr != nil {
			return nil, nil, aerr
		}
		if aok {
			prep = append(prep, astep)
		}
		argv = append(argv, "--tmpfs", d)
		watch(WallWatchReadMask, d)
		readHidden = append(readHidden, d)
	}
	// 5c. Another runtime's credential STORE, as a DIRECTORY
	// (ranger-base-a8qfg): an empty tmpfs over it with
	// credentialStoreAllowlist bound back read-only. 5a's shape at the same
	// wall, for the two ways a /dev/null over the credential FILE fails open
	// — a file that is not there at launch gets no mount, and a refresh by
	// atomic rename detaches one that is. SeatbeltCarveOut.ReadMaskDir has
	// both, and says why claude's own credential keeps the file mask.
	//
	// THE STORE IS MADE WHEN IT IS ABSENT, which the file half refuses to do
	// and for a reason that does not reach here: creating another runtime's
	// credential FILE would be posse writing into a store that is not its
	// business, while an empty DIRECTORY is the mount point this wall needs
	// and holds no byte of anybody's. It is also inert to every reader posse
	// has: each of them joins a name under the home and stats it, so an
	// empty store answers ENOENT exactly as an absent one does.
	//
	// A WRITE GRANT THAT REACHES THE STORE RENDERS NO MASK, and the file
	// mask one loop down is what is left. An empty tmpfs over a grant is a
	// seat writing where nothing survives the seat — B8's lost-write
	// failure with the grant in place, which is worse than the wall it
	// would be putting up — and a read wall inside a write grant is theatre
	// anyway: the breach is the grant, which is what
	// credentialReadDenyLiterals already says about a declared state dir
	// wide enough to contain a sibling's store. So this is the `reopens`
	// skip with a second reason, not a refusal: skipping leaves the status
	// quo (a /dev/null over the credential file, with ADR 0080's two holes),
	// where refusing would stop every launch on that box. The report names
	// a store that got no mount, in those words.
	//
	// BOTH DIRECTIONS, because either one is that mistake: a grant AT or
	// ABOVE the store would be buried by the tmpfs, and a grant INSIDE the
	// store would be buried too — the tmpfs mounts after it.
	//
	// THE HIDE SKIP ASKS WHAT 1b MOUNTED, not what it was handed
	// (ranger-base-miyv2). `hidden` holds every Hide entry, an absent one
	// too, and since ranger-base-8ukiy the landed-check judge hides each
	// runtime's state root — which for grok and bob IS the store. An absent
	// root renders no tmpfs, so `reopens` alone would skip this mask over
	// the one store it exists for: a login after launch, at a name nothing
	// covered.
	var storeMasked []string
	for _, d := range shallowestFirst(carve.ReadMaskDir) {
		if (writeGranted(hideMounted, d) && !writeGranted(grants, d)) || writeGranted(grants, d) || grantInside(grants, d) {
			continue
		}
		exists, dir, err := bwrapStat(d)
		if err != nil {
			return nil, nil, bwrapStatRefusal("credential store", d, err)
		}
		switch {
		case exists && !dir:
			return nil, nil, Die("bwrap cannot mask the credential store %s: it is not a directory, and an empty tmpfs can only be mounted over one (ranger-base-a8qfg)", d)
		case !exists:
			prep = append(prep, PrepStep{PrepMkdirAll, d, "another runtime's credential store that is not there yet; made empty so the mask has a directory to cover and a first login during this seat's life lands inside it"})
		}
		// AND THE MASK IS ANCHORED, for step 3's reason and on step 3's
		// measurement (ranger-base-ua625 finding 1). Unlike 5a and 5b this
		// arm leaves its host directory EMPTY — the `!exists` case above
		// makes it so deliberately, so that a first login during the seat's
		// life lands inside the mask — which puts it in the class exactly:
		// a mount point this launcher leaves empty is one host rmdir from
		// detaching. MEASURED 2026-11-14: the tmpfs detaches as the ro-bind
		// does, mountinfo 1 -> 0, and the seat then READS a file the host
		// wrote at that name after launch, which here is that login.
		// `~/.bob` was in this shape on this box when the finding was filed
		// — present, zero entries, masked for every claude seat. The anchor
		// sits under the tmpfs, so no seat ever sees it.
		astep, aok, aerr := bwrapAnchorStep(d, "an empty file inside another runtime's credential store, which this render leaves empty under a tmpfs; a host rmdir of the empty store would take that mask off the seat and show it a login made after launch")
		if aerr != nil {
			return nil, nil, aerr
		}
		if aok {
			prep = append(prep, astep)
		}
		argv = append(argv, "--tmpfs", d)
		watch(WallWatchReadMask, d)
		storeMasked = append(storeMasked, d)
		// The allow arm stats and does not Lstat, which is 5a's choice and
		// not a different one: a link at one of these names would be bound
		// through to its target, and the only writer at these names is the
		// operator whose home it is — the seat has no grant there and is
		// behind this very tmpfs. The Masked arm Lstats because the name it
		// covers IS inside a seat's grant (ranger-base-l751c).
		for _, m := range carve.readMaskDirAllowed(d) {
			exists, dir, err := bwrapStat(m.path)
			if err != nil {
				return nil, nil, bwrapStatRefusal("credential store allow", m.path, err)
			}
			if !exists || dir != m.dir {
				continue // not there, or not the shape posse reads at that name (5a's rule)
			}
			argv = append(argv, "--ro-bind", m.path, m.path)
		}
	}
	for _, f := range shallowestFirst(carve.DenyRead) {
		if writeGranted(readHidden, f) || writeGranted(storeMasked, f) {
			continue // under the tmpfs already, and a /dev/null mount would only make its name
		}
		exists, _, err := bwrapStat(f)
		if err != nil {
			return nil, nil, bwrapStatRefusal("read deny", f, err)
		}
		if !exists {
			continue
		}
		argv = append(argv, "--ro-bind", "/dev/null", f)
		watch(WallWatchReadMask, f)
	}
	// 5d. THE CARVE-OUT'S PINS: every directory between a write grant and a
	// deny, mask, keep or re-allow this render mounts below it, bound in
	// place so the mount pins the name (bwrapCarvePins, ranger-base-a59w7).
	// Composed here, where every mount of the render is known and a
	// component another step already mounts over costs nothing; spliced in
	// at 2d's mark, where its ordering argument is written.
	//
	// IT IS SHAPE 1's REMEDY AT THE OTHER PRODUCER. Step 2 binds the
	// directories between nested write GRANTS (bwrapGrantMounts,
	// ranger-base-dot6w); these are the same directories above the
	// carve-out's own mounts, which that list cannot reach because the
	// directory above a deny holds no grant of its own. A deny renders an
	// in-place `--ro-bind` and a mask renders `/dev/null` over the name, so
	// both arms of step 6 ask about those ancestors — and five of this
	// package's own pins were refusing a render for one, which is a seat
	// that does not launch.
	//
	// NOT WATCHED, for the reason the grant pins and step 2a-bis are not: a
	// `posse wall-watch` entry exists to kill a seat whose WALL came off,
	// and this mount is no wall over the seat's reach — the directory is
	// writable through the grant either way. Its one job is to refuse a
	// RENAME of the name from inside the namespace, which is about the next
	// render and not this run. No fixture renders one today (the census is 0,
	// TestQAWallCarvePinsCensusOverEveryRender), so the first render that
	// grows a pin reds TestWallWatchEveryRenderedMountIsAWallOrANamedOpening
	// with an unclassified destination — and the answer is an entry in
	// wwOpenings, beside the write grants, for the reason in this paragraph.
	pins := bwrapCarvePins(argv[:denyAt], argv[denyAt:], grants)
	if len(pins) > 0 {
		var pinArgv []string
		var pinPrep []PrepStep
		for _, p := range pins {
			// LSTAT'D, NOT STAT'D, which is 2c's rule for 2c's reason and
			// the one thing this arm cannot inherit from shape 1's. A pin's
			// path is by construction a directory the seat may write — that
			// is the whole of why it needs pinning — so a LINK at that name
			// is one a seat or its leftover planted, and bwrap follows a
			// link at a bind's source and at its destination alike. Binding
			// it in place would resolve BOTH ends to the link's target and
			// mount that target read-write at its own name, which for a
			// target outside the grant is a write this render never meant:
			// the very attack D2(b) refuses, let through by the mount that
			// was supposed to refuse it. So a link here refuses the render,
			// in the tier's fail-closed direction, and the refusal is the
			// answer the operator can act on (remove the plant). The same
			// question at shape 1's grant pins is open; ADR 0100's D2(b)
			// amendment is where its bead id is kept.
			fi, err := os.Lstat(p)
			exists, dir := err == nil, err == nil && fi.IsDir()
			switch {
			case err != nil && !os.IsNotExist(err):
				return nil, nil, bwrapStatRefusal("carve-out pin", p, err)
			case exists && !dir:
				// A file, a link or a special file at that name means the
				// thing this render mounts below it can neither be there nor
				// be made as itself — the launcher's own mkdir would fail,
				// or bwrap would resolve the name elsewhere, after the
				// render said yes.
				// fileTypeName and not Mode().Type(), whose String() reads
				// `L---------` and tells an operator holding a seat that
				// will not launch nothing at all.
				return nil, nil, Die("bwrap cannot pin %s for the deny or mask %s this render mounts below it: it is %s and not a directory, so there is nothing under it to mount over, and a link there would make this pin bind whatever it points at, read-write, at its own name (ADR 0100 D1/D2(b)/D4(a)). Remove it on the host, or narrow the write grant that contains it — never exempt it", p, bwrapCarvePinFor(argv[denyAt:], p), fileTypeName(fi.Mode()))
			case !exists:
				pinPrep = append(pinPrep, PrepStep{PrepMkdirAll, p, "a directory between a write grant and a deny or mask this render mounts below it; bwrap binds it in place so the mount pins the name (ADR 0100 D2(b)/D4(a))"})
			}
			pinArgv = append(pinArgv, "--bind", p, p)
		}
		argv = slices.Insert(argv, carveAt, pinArgv...)
		prep = slices.Insert(prep, carvePrepAt, pinPrep...)
	}

	// 6. EVERY BIND SOURCE IS PINNED IN THIS RENDER (ADR 0100 D2(b)) — the
	// in-place binds of shape 1's own write grants included, because bwrap
	// resolves a grant's path at mount(2) as surely as it resolves a private
	// source's (ranger-base-dw2du) — AND SO IS EVERY DESTINATION THAT
	// DIFFERS FROM ITS SOURCE (D4(a)).
	// Last, because both are asked of the finished argv — the mounts are
	// what pin a component, and a step above could still have added one.
	// Here and not in each producer, for the reason step 2a is here: this is
	// where bwrapRender, the landed-check judge and the wall venue meet, so
	// a render that grows a fourth caller is covered by writing nothing.
	//
	// TWO CALLS AND ONE PREDICATE. The two arms are different attacks — a
	// source is redirected before this render's own mount happens, a
	// destination is swapped at any moment of the victim's session — so each
	// gets the refusal that names its own mechanism and its own fix. What
	// they must not get is two readings of "pinned" (bwrapPinned's head
	// comment).
	if err := bwrapRefuseUnpinnedSource(argv, grants); err != nil {
		return nil, nil, err
	}
	if err := bwrapRefuseUnpinnedDst(argv, grants); err != nil {
		return nil, nil, err
	}
	return argv, prep, nil
}

// bwrapPrivateArgs renders ONE carve.Private entry: the mount options to
// append, the prep steps the launcher must run before the seat starts, and
// whether it rendered a mount at all. Shape 2c's call site says which arm
// each field selects and why each lstat is an lstat.
//
// IT IS A FUNCTION SO THAT THE WATCH IS NOT (ranger-base-4tsa7). Every arm
// here mounts over pb.Dst and every one of those mounts is a wall, so the
// one `watch(WallWatchPrivate, pb.Dst)` is made by the caller over whatever
// this returned. While each arm made its own, an arm could be added without
// one and nothing said so: ranger-base-6ob20's empty tmpfs arm was MEASURED
// on 2026-11-14 to be watched by no pin — deleting its watch call left `-run
// TestWallWatch` green, where deleting any of the other three red two tests,
// because the fixture that drives those pins rendered three arms and that
// was not one of them. A fixture entry fixes the instance; having no per-arm
// watch to forget is what fixes the class, and a FIFTH arm added below is
// watched by writing no new line at all.
//
// `ok` IS FALSE FOR THE ONE THING THAT IS NOT A MOUNT: a destination inside
// no write grant renders nothing (D23 and the call site's last paragraph).
// It is a third return rather than an empty slice because "rendered nothing"
// and "rendered something" is exactly the distinction the caller's watch
// call hangs on, and a nil slice that means one of them by convention is how
// the per-arm watch got forgotten in the first place.
func bwrapPrivateArgs(pb PrivateBind, grants []string) ([]string, []PrepStep, bool, error) {
	var prep []PrepStep
	bind := "--bind"
	if pb.ReadOnly {
		bind = "--ro-bind"
	}
	if pb.File {
		// A FILE bind (ranger-base-a544g): the same lstat discipline, for
		// a regular file. Both names are lstat'd, the source too: it sits
		// in the seat's own cache dir, which that seat can write, so a
		// link there is one the seat chose, and following it would bind
		// any file the uid can write into the next launch's namespace.
		fi, err := os.Lstat(pb.Dst)
		switch {
		case err == nil && !fi.Mode().IsRegular():
			return nil, nil, false, Die("bwrap cannot bind a private file over %s: it is not a regular file (a link, a dir or a special file is there, and every seat can write its parent) — remove it and relaunch (ranger-base-a544g)", pb.Dst)
		case err != nil && !os.IsNotExist(err):
			return nil, nil, false, bwrapStatRefusal("private member", pb.Dst, err)
		}
		if !writeGranted(grants, pb.Dst) {
			return nil, nil, false, nil
		}
		if err != nil {
			prep = append(prep, PrepStep{PrepMaskPoint, pb.Dst, "the shared file a private copy is mounted over; created empty at 0600 because a mount needs a name to cover"})
		}
		sexists, sfi, serr := bwrapPrivateSrcStat(pb)
		switch {
		case serr != nil:
			return nil, nil, false, serr
		case sexists && !sfi.Mode().IsRegular():
			return nil, nil, false, Die("bwrap cannot bind %s over %s: it is not a regular file (a link, a dir or a special file is there, and the seat can write its own cache dir) — remove it and relaunch (ranger-base-a544g)", pb.Src, pb.Dst)
		case !sexists:
			prep = append(prep,
				PrepStep{PrepMkdirAll, filepath.Dir(pb.Src), "the seat's private copy of shared claude state lives here"},
				PrepStep{PrepMaskPoint, pb.Src, "the seat's private copy of a file every claude seat shares, created empty at 0600 as claude creates it"})
		}
		return []string{bind, pb.Src, pb.Dst}, prep, true, nil
	}
	fi, err := os.Lstat(pb.Dst)
	switch {
	case err == nil && !fi.IsDir():
		return nil, nil, false, Die("bwrap cannot bind a private dir over %s: it is not a directory (a link or a file is there, and every seat can write its parent) — remove it and relaunch (ranger-base-5632o)", pb.Dst)
	case err != nil && !os.IsNotExist(err):
		return nil, nil, false, bwrapStatRefusal("private member", pb.Dst, err)
	}
	if !writeGranted(grants, pb.Dst) {
		return nil, nil, false, nil
	}
	var out []string
	switch {
	case pb.Src == pb.Dst:
		// In place (ranger-base-ajkqx): the lstat above is the source's
		// too, and an absent one is made once, here.
		if err != nil {
			prep = append(prep, PrepStep{PrepMkdirAll, pb.Dst, "a dir of the shared config that is re-bound in place to narrow or reopen the grant over it; a mount needs a directory to cover"})
		}
		out = []string{bind, pb.Dst, pb.Dst}
	default:
		if err != nil {
			prep = append(prep, PrepStep{PrepMkdirAll, pb.Dst, "the shared member a private copy is mounted over; a mount needs a directory to cover"})
		}
		if pb.Src == "" {
			// An empty tmpfs: nothing on the host to stat or make, and
			// nothing a host process wrote at Dst is visible under it
			// (ranger-base-6ob20).
			out = []string{"--tmpfs", pb.Dst}
			break
		}
		// THE SOURCE IS LSTAT'D AT EVERY COMPONENT, NOT STAT'D AT THE LAST
		// (ranger-base-wsgus). It sat under a bwrapStat — os.Stat, which
		// follows — so a seat that replaced its own <seat
		// wall>/claude/<member> with a link had the next launch bind that
		// link's target read-write at <cfg>/<member>, after the grants and
		// past the deny and read-mask steps, which name the target's own
		// path and not this alias.
		sexists, sfi, serr := bwrapPrivateSrcStat(pb)
		switch {
		case serr != nil:
			return nil, nil, false, serr
		case sexists && !sfi.IsDir():
			return nil, nil, false, Die("bwrap cannot bind %s over %s: it is not a directory (a link or a file is there, and the seat can write its own cache dir) — remove it and relaunch (ranger-base-5632o, ranger-base-wsgus)", pb.Src, pb.Dst)
		case !sexists:
			// The reason is the ARM's and not one member's: this branch
			// makes the source dir of every private dir copy, and since ADR
			// 0100 D1 that set is claude's shared members — which claude
			// runs ahead of each Bash command — and the seat's own object
			// store. A sentence naming only the first was read, in a prep
			// list, over the second (`posse gates <persona> --cage
			// seatbelt`, 2026-11-15).
			prep = append(prep, PrepStep{PrepMkdirAll, pb.Src, "the seat's own private copy of a directory it would otherwise share, under the per-seat wall dir: claude's state members, which it runs ahead of each Bash command, and this seat's object store"})
		}
		out = []string{bind, pb.Src, pb.Dst}
	}
	// THE TWO ARMS THAT MOUNT SOMETHING ELSE OVER pb.Dst ARE ANCHORED
	// (ranger-base-ua625 finding 1). A private copy and an empty tmpfs both
	// send the seat's writes somewhere that is not this name, so the
	// launcher leaves the host directory EMPTY for the seat's whole life —
	// and the `err != nil` mkdir above makes an absent one from nothing.
	// MEASURED 2026-11-15 (the evidence on ranger-base-dtzgt): the six host
	// dirs this render binds private copies over were all ABSENT before a
	// launch and recreated empty by the prep in the same millisecond as the
	// anchored deny dirs, and the same host rmdir that takes an empty
	// mcp-skill-archives off would take one of these off. After that detach
	// the seat's session, history and file-history writes land in the shared
	// host dir every other seat and the operator's own claude read, and its
	// reads see theirs.
	//
	// THE IN-PLACE ARM IS NOT IN THE CLASS and that is the whole of the
	// `Src != Dst`. A dir bound over ITSELF is the host's own live
	// directory, re-bound to narrow or reopen the grant over it: the seat
	// writes through to that name, so the launcher neither empties it nor
	// keeps it empty, and an anchor there is posse's file in a directory
	// the operator's own claude enumerates (claude's `projects/`, the live
	// member of this arm). It is 5a's rule at the private loop, and
	// ranger-base-ua625's own first draft anchored it — which a live pin
	// caught as "the real claude changed projects/", the change being
	// posse's.
	//
	// One call for the two arms that are, and not one per arm, for the
	// reason the single watch call is the caller's: a per-arm anchor is a
	// line the fifth arm forgets. The File arm returns above and gets none —
	// nothing rmdirs a file.
	if pb.Src != pb.Dst {
		step, ok, aerr := bwrapAnchorStep(pb.Dst, "an empty file inside a host directory this render mounts a private copy or an empty tmpfs over, and leaves empty; a host rmdir of it would take that mount off the seat and put the seat's writes in the shared dir")
		if aerr != nil {
			return nil, nil, false, aerr
		}
		if ok {
			prep = append(prep, step)
		}
	}
	return out, prep, true, nil
}

// bwrapPrivateSrcStat is the one filesystem question asked of a private
// bind's SOURCE, and it asks it of EVERY component rather than of the name
// (ranger-base-wsgus): it answers whether pb.Src is there and what it is,
// having lstat'd each component from pb.SrcRoot down and refused a link
// anywhere along the way. The LAST component is handed back unjudged, because
// the arms want different things of it — a regular file for the File bind, a
// directory for the dir copy — and each already has the message that says so.
//
// A SOURCE UNDER THE SEAT'S OWN CACHE DIR IS A PATH THE SEAT CAN WRITE, which
// is the whole reason the question is this shape. <SeatCacheDir>/claude was
// in the seat's write set (seatbelt.go's add(SeatCacheDir)), so the seat
// could remove that directory and plant a link to anything its uid can
// write. ADR 0100 D1 moved every such source to the seat's wall dir, which
// no render grants; this walk is what still refuses a link planted there by
// anything that is not a walled seat.
// Checking the last component alone let it through twice over: the dir arm's
// os.Stat followed a link at the member itself, and a link at the PARENT
// made <seat>/claude/history.jsonl lstat as the target's own regular file, so
// the File arm's own lstat passed and bwrap bound a file of the seat's
// choosing read-write inside the next launch's namespace.
//
// ABSENT IS NOT A REFUSAL, and it stops the walk: nothing can exist under a
// component that is not there, so the first ENOENT answers for the whole
// path, and the arms turn that into the PrepStep that makes it. Those steps
// run in the UNCAGED launcher (PrepMkdirAll, PrepMaskPoint), which is why
// they are emitted only on this walk's verdict and never on os.Stat's: a
// link here aimed the launcher's own 0755 mkdir and 0600 create at a path
// the seat picked.
//
// NOTHING AT OR ABOVE SrcRoot IS INSPECTED — PrivateBind's doc says why that
// is the trust line, and why the containment test is lexical.
//
// WHAT THE WALK ANSWERS FOR IS THE INSTANT IT RAN IN. bwrap resolves the
// source itself, at mount, and the launcher's prep steps resolve it again
// when they run. That is why the walk is not the guarantee: ADR 0100 D1 is,
// and it is the one that holds the source where no walled seat can write a
// component of it between this instant and bwrap's.
func bwrapPrivateSrcStat(pb PrivateBind) (exists bool, fi os.FileInfo, err error) {
	if pb.Src == "" {
		// Reachable only from the FILE arm: an empty Src is the dir shapes'
		// tmpfs, and that arm returns before this call.
		return false, nil, Die("bwrap cannot bind a private file over %s: the render named no source for it (ranger-base-wsgus)", pb.Dst)
	}
	return bwrapSrcWalk(pb.Src, pb.SrcRoot, pb.Dst)
}

// bwrapSrcWalk is that walk over any one source, so that the private binds
// and the private /tmp ask the SAME question of their sources (ADR 0100 D1).
// src is the path about to be bound, root the deepest directory this render
// trusts above it, dst the destination a refusal has to name.
//
// THE /tmp SOURCE CAME THROUGH bwrapStat UNTIL ADR 0100, and that was this
// class's one member needing no race at all: os.Stat FOLLOWS, so a seat that
// replaced its own `<cache>/tmp` with a link had the next launch bind the
// link's target read-write over /tmp, past every grant and every deny, which
// name the target's own path and not this alias (MEASURED, ranger-base-p47q6).
// One function and not two readings of "is the source there", for the reason
// bwrapPrivateArgs is one function: an arm that asks its own question is an
// arm that can be written without this one.
func bwrapSrcWalk(src, root, dst string) (exists bool, fi os.FileInfo, err error) {
	if root == "" {
		return false, nil, Die("bwrap cannot bind %s over %s: the render names no trusted root for it, so there is no component of the source this render may trust, and anything that can write a component plants a link at it (ranger-base-wsgus, ADR 0100 D1)", src, dst)
	}
	rootClean, srcClean := filepath.Clean(root), filepath.Clean(src)
	rel, rerr := filepath.Rel(rootClean, srcClean)
	if rerr != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil, Die("bwrap cannot bind %s over %s: its source is not under the %s this render trusts, so no walk of its components says anything about it (ranger-base-wsgus)", src, dst, root)
	}
	comps := strings.Split(rel, string(filepath.Separator))
	// i == -1 is the root itself: lstat'd, because a link AT it is a link in
	// the path too, and its own parents are the launcher's to spell.
	p := rootClean
	for i := -1; i < len(comps); i++ {
		if i >= 0 {
			p = filepath.Join(p, comps[i])
		}
		cfi, cerr := os.Lstat(p)
		switch {
		case os.IsNotExist(cerr):
			return false, nil, nil
		case cerr != nil:
			return false, nil, bwrapStatRefusal("private source", p, cerr)
		}
		if i == len(comps)-1 {
			return true, cfi, nil
		}
		if cfi.Mode()&os.ModeSymlink != 0 {
			return false, nil, Die("bwrap cannot bind %s over %s: %s is a symlink, so following it would bind a path this render never judged read-write inside the seat's namespace. Remove it and relaunch (ranger-base-wsgus)", src, dst, p)
		}
		if !cfi.IsDir() {
			return false, nil, Die("bwrap cannot bind %s over %s: %s is not a directory, so the source is not the path it is spelled as. Remove it and relaunch (ranger-base-wsgus)", src, dst, p)
		}
	}
	// Unreachable: the loop returns at i == len(comps)-1, and comps is never
	// empty (an empty rel is the "." the containment test above refused).
	return false, nil, Die("bwrap cannot bind %s over %s: its source has no component under %s (ranger-base-wsgus)", src, dst, root)
}

// bwrapRefuseWallRootGrant is ADR 0100 D2(a): a render is refused outright
// when any write grant OVERLAPS the wall root in either direction — at or
// above it, or inside it. It is the invariant that makes the wall dir a wall
// — every bind source that differs from its destination lives under that
// root (D1), and a grant that reaches it hands a seat a COMPONENT of those
// sources, which is this whole class of hole (ranger-base-p47q6). The ones
// it would hand over are not only this seat's: the root holds every seat's
// wall dir, so the refusal is box-wide and not a statement about the render
// it refuses.
//
// BOTH DIRECTIONS, and the second one is ranger-base-dw2du's (the review's
// re-read of this decision). "At or above" was the whole rule while the
// attack in view was a grant that holds a COMPONENT of a source. A grant
// INSIDE the root is neither at nor above it and is the same hole arrived at
// from below: a `writable:` extra naming
// `<wall root>/<another seat's key>/claude/shell-snapshots` is a grant on
// that seat's private bind SOURCE, and what the seat sources from there runs
// ahead of every Bash command it makes — a walled-to-walled lateral move
// with no rename and no race in it at all. The negative case is why this
// reds nothing legitimate: a seat's own private destinations are grants
// (`<cfg>/…`), but its own private SOURCES are not granted by any render —
// they are bound, which is the whole of D1 — so no shipped render has a
// grant under this root to lose (MEASURED 2026-11-21, over all five argvs
// this package renders: 0 of 32 grants lie inside the root).
//
// A REFUSAL AND NOT A NARROWING, for the reason the ADR 0095 arm above gives
// in the same words: a seat launched behind a wall that silently dropped a
// grant it was promised fails later and somewhere else. The remedy is the
// PID's own list — a `writable:` extra naming the home or `~/.cache` — and
// the refusal names it.
//
// THE GRANT SIDE IS RESOLVED HERE, which is the one re-resolution BwrapArgs
// makes, and it is deliberate (ADR 0100 D2(a)). Every producer in this
// package already absResolves what it emits, so for them this is idempotent;
// what it buys is the entry that arrives as a LINK from somewhere else — a
// `writable:` extra a seat planted at a path of its own for its next launch
// — because only the resolved spelling says where the bind would land. The
// wall root is composed, from the one resolved home, and the comparison is
// insideDir and never underDir: underDir would resolve the root too, and a
// resolution of the root follows whatever is planted at the END of it, which
// is to say it judges the plant's target instead of the plant. The inside
// arm reads the same two sides the same two ways — the root composed, the
// grant resolved — so an extra spelled as a link INTO the root is caught by
// the one re-resolution the outside arm already makes.
//
// IT STANDS DOWN WHERE THE HOME IS ITSELF UNDER /tmp, which is bwrapSeatTmp's
// own stand-down and the same box shape: /tmp is granted unconditionally
// (SeatbeltWritable) and a home inside it is in the shared directory
// already, so there is no seat on such a box for the wall root to protect
// and no grant this refusal could usefully take away. No production launch
// is that shape — homes and worktrees live under the home — and every
// fixture in this package is, which is why the pins for this rule put their
// home under wallUngrantedDir (the same reading ranger-base-hi2lt wrote into
// the ADR 0095 arm above).
func bwrapRefuseWallRootGrant(writable []string) error {
	root := bwrapWallRoot()
	if root == "" {
		return nil
	}
	for _, g := range writable {
		gr := absResolve(g)
		switch {
		case insideDir(gr, root):
			return Die("bwrap cannot grant write on %s: it contains the per-seat wall root %s, where every bind source of every walled seat on this box lies — a seat with that grant renames a component of another seat's source and bwrap mounts what it points at, read-write, inside that seat's namespace (ADR 0100 D1/D2(a)). Narrow the grant: name the directory the seat needs, not a parent of the wall root", g, root)
		case insideDir(root, gr):
			return Die("bwrap cannot grant write on %s: it lies inside the per-seat wall root %s, which holds the bind SOURCES of every walled seat on this box — a seat with that grant writes another seat's private source directly, and for claude's shell snapshots and session env what is written there runs ahead of every command that seat makes (ADR 0100 D1/D2(a)). Narrow the grant: no seat's own sources are granted to it either, which is what makes them a wall", g, root)
		}
	}
	return nil
}

// bwrapWallRoot is SeatWallRoot for the box this render runs on, or "" where
// the rule above stands down. The home is read the same way bwrapRender
// reads it for the sources themselves, so the root a grant is judged against
// and the root the sources are composed under cannot be two directories.
func bwrapWallRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || underDir(bwrapTmpMount, home) {
		return ""
	}
	return SeatWallRoot(home)
}

// ─── ADR 0100 D2(b): A PATH IS PINNED IN A RENDER ───────────────────────────

// bwrapPinned is D2(b)'s predicate, and it is ONE function on purpose: ADR
// 0100 D4(a) asks the same question of every DESTINATION that differs from
// its source, and two implementations of "pinned" is two answers about one
// wall.
//
//	A path P is PINNED in a render R when every component of P that lies
//	STRICTLY INSIDE a write grant of R is itself a mount destination of R.
//
// It returns the shallowest component that is not, or ("", true).
//
// WHAT IT MEANS, in one sentence: nothing inside R's namespace can change
// what P names. To create, rename or unlink a component C you need write on
// C's PARENT, which is what "strictly inside a write grant" says — a
// component equal to a grant is not strictly inside it, because its own
// parent is outside. And a component that is a mount destination cannot be
// renamed by the namespace that holds that mount: MEASURED, ADR 0100's P4
// and P4b, EBUSY, alias included.
//
// WHY A SOURCE NEEDS IT. bwrap resolves a bind source at mount time and
// follows links; this render's own walk (bwrapSrcWalk) judged it an instant
// earlier. A seat — or any leftover of one — that can swap a component
// between the two instants redirects the bind, and it needs no timing: it
// toggles the component in a loop. D1 answers the class by moving every
// source under the wall dir, where NO component is inside any grant; this
// predicate is what says so of each render rather than of the design, and
// what catches a source put back. It is asked of every bind source and not
// only of those that differ from their destination: an in-place bind's path
// is re-resolved at mount(2) too, so a loose ANCESTOR of a write grant
// redirects that grant (ranger-base-dw2du).
//
// THE GRANT SIDE IS COMPARED BOTH AS WRITTEN AND RESOLVED, and the union is
// deliberate: either spelling reaching the component is a seat that can
// write it, and a predicate whose job is to refuse must not be narrowed by a
// host whose home is behind a symlink. The component side stays COMPOSED —
// resolving it would follow a link a seat planted and judge the link's
// target, which is D2(a)'s insideDir/underDir distinction one level down.
//
// RESIDUAL, stated and not closed: "write grant" here is R's grant list,
// the paths R binds read-write IN PLACE. A read-write bind whose source
// DIFFERS from its destination also exposes its source host-side — so a
// component of P lying inside another bind's rw source is seat-writable and
// this predicate does not say so. No render in this package has that shape
// today (the only aliased rw sources are the wall members themselves, and
// nothing binds their parents), and a bead that adds one widens this
// function rather than exempting itself.
func bwrapPinned(p string, grants, dsts []string) (string, bool) {
	for _, c := range bwrapPathComponents(p) {
		if !bwrapStrictlyInsideGrant(c, grants) || bwrapIsMountDst(c, dsts) {
			continue
		}
		return c, false
	}
	return "", true
}

// bwrapPathComponents is p's components, shallowest first and p itself last:
// `/a/b/c` is `/`, `/a`, `/a/b`, `/a/b/c`. Shallowest first so a refusal
// names the OUTERMOST loose component, which is the one whose fix pins
// everything below it.
func bwrapPathComponents(p string) []string {
	p = filepath.Clean(p)
	var up []string
	for {
		up = append(up, p)
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	out := make([]string, 0, len(up))
	for i := len(up) - 1; i >= 0; i-- {
		out = append(out, up[i])
	}
	return out
}

// bwrapStrictlyInsideGrant is "c's parent is writable in this render".
func bwrapStrictlyInsideGrant(c string, grants []string) bool {
	for _, g := range grants {
		for _, gs := range []string{g, absResolve(g)} {
			if gs != c && insideDir(gs, c) {
				return true
			}
		}
	}
	return false
}

// bwrapGrantMount is one entry of shape 1's mount list: a write grant, or a
// directory bound in place only to PIN a grant nested below it (Pin, with
// For naming that grant for the refusal's message).
type bwrapGrantMount struct {
	Path string
	Pin  bool
	For  string
}

// what names the kind in a stat refusal, so a pin's failure does not read as
// a grant the operator never declared.
func (m bwrapGrantMount) what() string {
	if m.Pin {
		return "write grant pin"
	}
	return "write grant"
}

// bwrapGrantMounts is shape 1's mount list: every write grant, plus every
// directory STRICTLY BETWEEN a grant and the grant that contains it —
// shallowest first, so each mount lands on top of the one that contains it
// and the deepest lands last.
//
// IT IS D2(b)'s OWN REMEDY, GENERALIZED (ADR 0100 D2(b) as amended by
// ranger-base-dw2du, and D4(a) step 2a-bis's shape). Every write grant is
// bound IN PLACE, so a grant's own name is always a mount of the render and
// what the predicate asks of it is only about its ANCESTORS: a grant one
// component inside another has that outer grant for its parent and is
// pinned. What reds is a nest TWO or more components deep with nothing bound
// between — the loose component is a mount of nothing inside a directory the
// seat may write, so a leftover of that very seat renames it, and bwrap
// resolves the grant's own string to what the name points at instead,
// read-write, at the next launch and with no race to win. dw2du's census
// found no render in that state and its head comment said so; the shape
// EXISTS — a store of record inside the same repo grants `<common>` whole,
// which puts `<common>/worktrees/<seat>` two components inside the
// `<common>` grant with `worktrees` loose between them — and a render that
// builds it was refused outright (ranger-base-dot6w). This is the fix that
// decision prescribes for a miss and the only one it allows: bind the loose
// component in place, never exempt it.
//
// EMITTED BY THE RENDER THAT ALREADY GRANTS IT, so it widens nothing: every
// path it names lies strictly inside a write grant, which has already made
// it read-write, and a path inside no grant produces no entry at all. That
// is the same condition step 2a-bis carries for the private /tmp source and
// runtimeEphemeralDirParents for the CLI homes — the two instances this
// generalizes, and they stay where they are because neither is a grant: one
// is a bind SOURCE and the other a tmpfs DESTINATION, and this list is read
// only for the grants.
//
// KEYED TO THE GRANT SET AND NOT TO THE WHOLE RENDER'S MOUNTS, which is one
// notch wider than bwrapPinned asks. A later mount over a loose component
// would pin it just as well, and the predicate at step 6 reads it that way —
// but this list is rendered FIRST, before any of those mounts is composed,
// and a pin computed from mounts that land after it would be a pin whose
// correctness depends on which step appends what. Where a later tmpfs or
// private bind does cover one of these paths it simply lands on top, as
// every narrower statement in this render does; the in-place bind under it
// changes nothing, because in place is the host's own directory read-write
// and the grant above it already said so.
//
// IT IS NOT WATCHED, for step 2a-bis's reason: this mount is no wall over
// the seat's reach — the directory is writable either way — and its one job
// is to refuse a RENAME from inside the namespace, which is about the NEXT
// render and not this run.
func bwrapGrantMounts(grants []string) []bwrapGrantMount {
	isGrant := func(c string) bool {
		for _, g := range grants {
			if g == c || absResolve(g) == absResolve(c) {
				return true
			}
		}
		return false
	}
	pinFor := map[string]string{}
	var pins []string
	for _, g := range grants {
		for _, c := range bwrapPathComponents(g) {
			if isGrant(c) || !bwrapStrictlyInsideGrant(c, grants) {
				continue
			}
			if _, seen := pinFor[c]; seen {
				continue
			}
			pinFor[c] = g
			pins = append(pins, c)
		}
	}
	out := make([]bwrapGrantMount, 0, len(grants)+len(pins))
	for _, p := range shallowestFirst(append(append([]string{}, grants...), pins...)) {
		g, pin := pinFor[p]
		out = append(out, bwrapGrantMount{Path: p, Pin: pin, For: g})
	}
	return out
}

// bwrapCarvePins is shape 1's remedy at the CARVE-OUT's producers: the
// directories a render must bind IN PLACE so that the denies, masks, keep
// files and re-allows it mounts below them are pinned. `head` is the render
// as far as the deny producer (step 2e's mark), `tail` the denies, keeps and
// masks from there down, and the answer is the ancestors — shallowest first,
// each named once — of every path `tail` mounts that lie strictly inside a
// write grant and are a mount destination of neither half.
//
// THE SPLIT IS WHICH MOUNTS ARE ASKED ABOUT, not where the answer goes:
// step 2e says why the two producers above that mark keep their own pins,
// and step 2d why these land before every one of them.
//
// WHY THE CARVE-OUT NEEDS ITS OWN LIST. bwrapGrantMounts pins the
// directories between nested write GRANTS, which is every loose component a
// grant's own in-place bind can have (ranger-base-dot6w). A deny is an
// in-place `--ro-bind` and a mask is a `/dev/null` over a name, and both
// have ancestors too: where one lies TWO or more components inside a write
// grant with no mount between — `<repo>` granted and the deny
// `<repo>/.git/config.lock`, so `<repo>/.git` is loose; a CLI home grant
// above a masked `<home>/codex/auth.json`, so `<home>/codex` is loose —
// nothing in the grant set names that directory and the grant list cannot
// reach it. Step 6 then refuses the whole render. MEASURED 2026-11-21: five
// of this package's own pins, under a home outside the shared `/tmp`, and it
// fails CLOSED — a seat on a box in that configuration does not launch,
// which is this rule costing a launch and never a hole in it
// (ranger-base-a59w7).
//
// IT WIDENS NOTHING, which is why it is conditional on the grant and not
// emitted always: a read-write bind in place makes its path writable, and
// every path here lies STRICTLY INSIDE a grant that has already made it
// read-write. A deny or mask inside no grant produces no entry at all —
// which is most of them, since the masks a seat is not granted lie under the
// read-only base.
//
// ANCESTORS ONLY, never the mounted path itself. A path this render mounts
// over is a mount destination of that very mount, so it is never loose; what
// the predicate asks of it is only about the directories ABOVE it. The one
// path that can be loose on its own account is a bind SOURCE that differs
// from its destination, and that one is NOT pinned here — D1's remedy for it
// is to put the source under the per-seat wall dir, where no component lies
// inside any grant, and step 6 still refuses it by name.
//
// KEYED TO THE GRANT SET for bwrapGrantMounts' reason, and asked of the
// WHOLE argv for what counts as already pinned: a component another step
// mounts over — the private member binds of 2c, a credential store's tmpfs,
// an ssh dir's — needs nothing from this list, and reading those off both
// halves is what keeps the list to the directories no other statement
// covers. The two cases are not symmetric because the question is not: a
// mount that lands LATER still pins a name against a rename, which is all
// this list is for.
func bwrapCarvePins(head, tail, grants []string) []string {
	ms := bwrapMounts(tail)
	dsts := append(bwrapMountDsts(bwrapMounts(head)), bwrapMountDsts(ms)...)
	var pins []string
	for _, m := range ms {
		for _, p := range []string{m.Src, m.Dst} {
			if p == "" {
				continue // a --tmpfs has no source at all
			}
			for _, c := range bwrapPathComponents(p) {
				if c == p || bwrapIsMountDst(c, dsts) || !bwrapStrictlyInsideGrant(c, grants) {
					continue
				}
				pins = append(pins, c)
			}
		}
	}
	// shallowestFirst dedupes, which is what makes two denies under one
	// loose directory render one pin.
	return shallowestFirst(pins)
}

// bwrapCarvePinFor names the first path `tail` mounts below the pin `p`, so
// that a refusal about `p` can say what asked for it. For the message only:
// a pin is emitted for every such path at once, and which one the sentence
// names changes nothing about the fix.
func bwrapCarvePinFor(tail []string, p string) string {
	for _, m := range bwrapMounts(tail) {
		if m.Dst != "" && insideDir(p, m.Dst) {
			return m.Dst
		}
	}
	return "(nothing — a pin with no mount below it is a bug in bwrapCarvePins)"
}

// bwrapIsMountDst is "this render mounts something at exactly c".
func bwrapIsMountDst(c string, dsts []string) bool {
	for _, d := range dsts {
		if d == c || absResolve(d) == absResolve(c) {
			return true
		}
	}
	return false
}

// bwrapMount is one mount the rendered argv makes. Src is "" for a mount
// with no host source — a `--tmpfs`, a `--proc` — and for the FD forms,
// whose first operand is a file descriptor and not a path.
type bwrapMount struct{ Src, Dst string }

// bwrapMountVerb is one mount-making bwrap option as a READER of a rendered
// argv has to know it: how many operands it takes, which of them is a
// filesystem SOURCE, and which is the mount DESTINATION.
type bwrapMountVerb struct {
	Args int // operands the option takes
	Src  int // operand index of the source, -1 for an option that has none
	Dst  int // operand index of the destination
}

// bwrapMountVerbs is THE ONE READING of bwrap's mount-making argv grammar in
// this package (ranger-base-r9slg, escaped from ranger-base-ltmyh). It was a
// hand-copied four-verb switch in each of two readers — bwrapMounts and
// bwrapMountedAt — with nothing holding the two equal to each other, and
// nothing holding either equal to what the renders emit or to bwrap's own
// grammar. `--proc` was in neither, although the wall venue types one, so
// `/proc` was a mount destination the D2(b)/D4(a) predicate could not see;
// and a producer typing `--bind-try` would have gone unjudged by that
// predicate and invisible to the report, with every pin green.
//
// TOTAL OVER WHAT THIS PACKAGE MAY TYPE, held there by a census rather than
// by a reviewer: TestQAEveryBwrapMountVerbThisPackageTypesIsRead reds on a
// mount-making option that appears in these sources and not in this table.
// So a bead that wants a new spelling adds it HERE, where both readers get
// it at once.
//
// THE FD FORMS carry Src -1 deliberately: their first operand is a file
// descriptor, so there is no path for ADR 0100 D2(b) to judge, while their
// destination is a mount like any other and D4(a) must still see it.
//
// THE OVERLAY FAMILY IS DELIBERATELY ABSENT — `--overlay`, `--overlay-src`,
// `--ro-overlay`, `--tmp-overlay`. An overlay's sources are a LIST built up
// by preceding `--overlay-src` options, so "the source of this mount" is not
// one operand and ADR 0100's predicate has no reading of it yet. Nothing in
// this package emits one, and the census above is what makes that a decision
// somebody takes rather than a gap: type an overlay and it reds, naming this
// paragraph.
var bwrapMountVerbs = map[string]bwrapMountVerb{
	"--bind":         {Args: 2, Src: 0, Dst: 1},
	"--bind-try":     {Args: 2, Src: 0, Dst: 1},
	"--ro-bind":      {Args: 2, Src: 0, Dst: 1},
	"--ro-bind-try":  {Args: 2, Src: 0, Dst: 1},
	"--dev-bind":     {Args: 2, Src: 0, Dst: 1},
	"--dev-bind-try": {Args: 2, Src: 0, Dst: 1},
	"--bind-fd":      {Args: 2, Src: -1, Dst: 1},
	"--ro-bind-fd":   {Args: 2, Src: -1, Dst: 1},
	"--bind-data":    {Args: 2, Src: -1, Dst: 1},
	"--ro-bind-data": {Args: 2, Src: -1, Dst: 1},
	"--proc":         {Args: 1, Src: -1, Dst: 0},
	"--dev":          {Args: 1, Src: -1, Dst: 0},
	"--tmpfs":        {Args: 1, Src: -1, Dst: 0},
	"--mqueue":       {Args: 1, Src: -1, Dst: 0},
}

// bwrapMounts reads them off the argv rather than off the lists that
// produced it, for bwrapMountedAt's reason below: a list entry says what the
// design asked for and the argv says what the wall is.
//
// An option whose operands do not all fit in the argv is NOT a mount: bwrap
// would refuse the line itself, and inventing a mount out of the tail would
// make a reader of a truncated argv answer about a path that is not there.
func bwrapMounts(argv []string) []bwrapMount {
	var out []bwrapMount
	for i, a := range argv {
		v, ok := bwrapMountVerbs[a]
		if !ok || i+v.Args >= len(argv) {
			continue
		}
		m := bwrapMount{Dst: argv[i+1+v.Dst]}
		if v.Src >= 0 {
			m.Src = argv[i+1+v.Src]
		}
		out = append(out, m)
	}
	return out
}

// bwrapMountDsts is just the destinations of those, which is the set
// bwrapPinned asks about.
func bwrapMountDsts(ms []bwrapMount) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Dst)
	}
	return out
}

// bwrapRefuseUnpinnedSource is D2(b) over a rendered argv: EVERY bind
// source must be PINNED in the render that binds it, else the render is
// refused. Called at the end of BwrapArgs, where all three of this package's
// renders meet, and again by the venue over the mounts it adds after that
// (boxLandedCage).
//
// EVERY SOURCE, IN-PLACE GRANTS INCLUDED, and that second half is
// ranger-base-dw2du's (the review's re-read of this decision). The first
// text of this rule read "every source that DIFFERS from its destination",
// which left the in-place binds — shape 1's `--bind p p` for each write
// grant, and every in-place `--ro-bind` — judged by neither arm: the
// destination arm (bwrapRefuseUnpinnedDst) skips them for the same
// equality. They are not safe by being in place. bwrap resolves the SAME
// STRING again at mount(2), so a component of a grant path that a walled
// seat can rename is togglable between a directory and a link to anywhere —
// and the render's own absResolve, made an instant earlier, is exactly the
// check-then-use this ADR rejects A2 for. A grant whose loose component gets
// pointed at the wall root is a read-write mount of every seat's private
// sources at the grant's name, at the NEXT launch, with no race to win; the
// attacker is the seat's own leftover, so it needs no sibling.
//
// NO MODE DISTINCTION, which is one notch wider than the re-read asked for
// ("every rw bind"): an in-place `--ro-bind` redirected the same way hands
// out the same tree read-only, which for private history, shell snapshots
// and the seat store is a confidentiality break, and a rule with a mode
// exemption is a rule with an exemption to maintain. MEASURED 2026-11-21
// over all five argvs this package renders: 150 in-place binds, every one
// pinned, so the wider reading costs nothing today.
//
// THE QUESTION THE RE-READ LEFT ASSUMED — whether any render has a grant
// nested two or more components inside another with a non-mount component
// between them — was answered "no" by that census, and the answer was about
// the five argvs it censused and not about the shape (ranger-base-dot6w). A
// SIXTH exists: a store of record inside the same repo grants `<common>`
// whole, which puts `<common>/worktrees/<seat>` two components inside that
// grant with `worktrees` loose between them, and the render of a committing
// seat in a linked worktree was refused outright — a seat that does not
// launch at all, which is how this rule fails and not a hole. Shape 1 emits
// the in-place bind of such a component now (bwrapGrantMounts), so what the
// census reads is a render that PINS the nest rather than one that has
// none: the 150 above are unchanged by that list, because no shipped render
// carries a nest for it to pin.
//
// A REFUSAL AND NOT A WARNING, in the tier's own direction: a source whose
// components a seat can swap is a mount of the seat's choosing, read-write,
// inside somebody's namespace — and the two shapes it has caught are the
// shared object store (ADR 0099's whole subject) and claude's shell
// snapshots (code the victim runs ahead of every Bash command).
//
// WHAT PASSES, because it is worth knowing which legitimate shapes this
// rule had to let through:
//
//   - every source under the seat's wall dir, trivially: no component of it
//     lies inside any grant at all (D1, and D2(a) keeps that true);
//   - the seat store's SIDE bind, whose source `<common>/objects` is inside
//     a `<common>` grant whenever a store of record in the same repo grants
//     that dir whole — it passes because that exact path is the store's own
//     mount destination, which is to say the thing that makes it unswappable
//     is the mount the render already makes (ADR 0100 D2(b));
//   - `/dev/null` over a masked file: `/dev` is a mount destination and
//     nothing grants it;
//   - every IN-PLACE bind, including each write grant's own `--bind p p`: a
//     path is always a mount destination of the mount it is the destination
//     of, so what this arm asks of them is only about their ANCESTORS — a
//     grant equal to a grant is not strictly inside it, and a grant nested
//     one component inside another has its parent for the outer grant
//     itself. What reds is a nest TWO or more components deep with no mount
//     between, and no render here HAS one loose: where a grant is nested
//     that deep, shape 1 binds the directories between in place and the
//     mount is what pins them (bwrapGrantMounts, ranger-base-dot6w), and
//     where a DENY or a mask is, step 5d does the same for the directories
//     above it (bwrapCarvePins, ranger-base-a59w7) — five of this package's
//     pins were refused renders for one of those before it existed.
//
// IT STANDS DOWN WHERE THE WALL ROOT DOES (bwrapWallRoot): on a box whose
// home is itself under the shared /tmp there is no ungranted place for a
// source to live, so every private source would be unpinned and every
// render refused. No production launch is that shape — homes and worktrees
// live under the home — and every fixture in this package is, which is why
// the pins for this rule put their home under wallUngrantedDir.
func bwrapRefuseUnpinnedSource(argv, grants []string) error {
	if bwrapWallRoot() == "" {
		return nil
	}
	ms := bwrapMounts(argv)
	dsts := bwrapMountDsts(ms)
	for _, m := range ms {
		if m.Src == "" {
			continue
		}
		loose, ok := bwrapPinned(m.Src, grants, dsts)
		if ok {
			continue
		}
		if m.Src == m.Dst {
			return Die("bwrap cannot bind %s in place: %s lies inside a write grant of this render and is no mount of it, so a walled seat — or anything this seat leaves behind — renames it and bwrap resolves this very path to what it points at instead, read-write, at the next launch and with no race to win (ADR 0100 D1/D2(b)). Bind %s in place too, so the mount pins it, or narrow the grant that contains it — never exempt it", m.Src, loose, loose)
		}
		return Die("bwrap cannot bind %s over %s: %s lies inside a write grant of this render and is no mount of it, so a walled seat — or anything this seat leaves behind — renames it and bwrap binds what it points at instead, at the next launch and with no race to win (ADR 0100 D1/D2(b)). Put the source under the per-seat wall dir %s, or bind %s in place so the mount pins it", m.Src, m.Dst, loose, bwrapWallRoot(), loose)
	}
	return nil
}

// grantInside says whether any write grant lies at or under d — the other
// direction of writeGranted, for a mount over d that would cover it: a tmpfs
// at d buries a grant under d as surely as a grant above d buries the tmpfs.
func grantInside(grants []string, d string) bool {
	for _, g := range grants {
		if underDir(d, g) {
			return true
		}
	}
	return false
}

// bwrapRefuseUnpinnedDst is ADR 0100 D4(a) over a rendered argv: EVERY
// DESTINATION that differs from its source must be PINNED in the render that
// mounts over it, else the render is refused. Same predicate as the source
// arm one function up — D2(b)'s, and one function on purpose, because two
// implementations of "pinned" is two answers about one wall.
//
// IT IS A DIFFERENT ATTACK FROM THE SOURCE ARM AND NOT A RACE (ADR 0100's
// Context, MEASURED 2026-11-15 by the dst-rename-probe on this box, bwrap
// 0.13.1). A namespace that holds NO mount on `<cfg>/<member>` renames it
// away mid-session and creates a new one; both steps succeed, the victim's
// mount stays alive on the renamed dentry, and the victim's next open of
// `<cfg>/<member>/…` reads the PLANTED file. A namespace that holds the same
// bind gets EBUSY for the same mv. So the swap is a standing rename at any
// moment of the victim's session, not a window at launch — which is why a
// launch-time or post-mount verify cannot close it (A3, rejected on that
// measurement) and why the answer is that every namespace which can write
// the parent holds the child as a mount.
//
// WHAT THAT BUYS, in the one case that matters most: for claude's
// shell-snapshots and session-env the victim runs sibling-written code ahead
// of every Bash command, so an unpinned private destination is a
// walled-to-walled lateral move. The box-wide half of the same decision is
// D4(b) — the private set follows the GRANT and not the runtime
// (claudePrivateSessionState's gate), so a sibling that can write `<cfg>`
// holds every member as a mount whatever runtime it runs.
//
// WHAT PASSES, MEASURED 2026-11-15 by running this predicate over all five
// rendered argvs this package makes (the census ADR 0100 D4(a) asked for;
// the numbers and the one miss are in the ADR's own D4(a) amendment):
//
//   - `<cfg>/<member>` and `<cfg>/history.jsonl`: the member IS the mount,
//     and `<cfg>` itself is the grant rather than something strictly inside
//     one, so nothing above it is loose either;
//   - `<cfg>/projects/<key>` and its `memory/`: in-place binds, so they are
//     not in scope at all, and `projects` is itself a mount of the render;
//   - `/tmp`: `/` is no grant of anyone;
//   - `<common>/objects` and `<own>/seat-objects-base`: their components
//     inside a grant are mounts, and the rest sit under the read-only `/`;
//   - codex's `tmp/` and `shell_snapshots/`: direct children of the granted
//     codex home, each its own tmpfs, so each is the mount that pins it. ADR
//     0100 listed these as needing a census; the census says they pass.
//   - every `/dev/null` read mask and every credential-store tmpfs: the
//     masked name is the mount, and the stores a seat is not granted lie
//     strictly inside no grant. A store a grant DOES reach renders no tmpfs
//     (5c skips it) and keeps the file mask, which leaves the store
//     directory itself between the grant and that mask — step 5d binds it in
//     place (bwrapCarvePins, ranger-base-a59w7). It is the same remedy as
//     the miss below and arrived the same way: by a pin of this package
//     being a refused render.
//
// THE ONE MISS has a bead of its own, and it is named in ONE place — ADR
// 0100's own D4(a) amendment, by bead id and nothing else, no path and no
// mechanism, because the surface it lies on is an open residual (operator
// ruling 2026-11-11). Not here as well: a public record that names an open
// hole in two places has two places to correct, and the id reads the same in
// either, so the ADR keeps it. Its SHAPE was this rule's own, which is why
// the census found it and why no more than the rule is needed to describe it
// — a private
// destination whose parent lay strictly inside a write grant and was a mount
// of nothing, so a leftover of that very seat could rename the parent away
// and plant its own directory at that name. Fixed the way this ADR
// prescribes and the source arm was fixed (step 2a-bis): an IN-PLACE bind of
// the loose parent, emitted by the render that already grants it
// (runtimeEphemeralDirParents), never an exemption here.
//
// IT STANDS DOWN WHERE THE SOURCE ARM DOES, for the same reason and read off
// the same helper: on a box whose home is under the shared `/tmp` there is
// no ungranted place for anything to live.
func bwrapRefuseUnpinnedDst(argv, grants []string) error {
	if bwrapWallRoot() == "" {
		return nil
	}
	ms := bwrapMounts(argv)
	dsts := bwrapMountDsts(ms)
	for _, m := range ms {
		if m.Src == m.Dst {
			continue
		}
		loose, ok := bwrapPinned(m.Dst, grants, dsts)
		if ok {
			continue
		}
		src := m.Src
		if src == "" {
			src = "an empty tmpfs"
		}
		return Die("bwrap cannot mount %s over %s: %s lies inside a write grant of this render and is no mount of it, so a walled seat — or anything this seat leaves behind — renames it mid-session, plants its own directory at that name, and this seat reads and runs what is in it while its own mount rides the renamed one (ADR 0100 D4(a), MEASURED). Bind %s in place so the mount refuses that rename, never exempt it", src, m.Dst, loose, loose)
	}
	return nil
}

// bwrapGitConfigRepo answers the repository whose `<repo>/.git/config` a deny
// path IS, or "" for every other path — ADR 0095 D1's key, read off the render
// and nothing else.
//
// OFF THE PATH AND NOT OFF GIT, for the reason D1 gives for keying the whole
// refusal to the render: the caller is handed lists of paths and one cwd, and
// a deny can belong to a repo that is not the cwd's (sessionGitConfigFiles
// also names the beads store of record's, when a redirect puts it in another
// repo). A `git rev-parse` per deny entry would make the refusal depend on
// which repo the renderer happened to be standing in, and would ask the
// filesystem a question whose answer the carve-out already carries.
//
// KEYED TO `.git/config` BECAUSE THAT IS WHAT THE TABLE MEASURED (ADR 0095's
// Context): a git dir under another name, or a `config.worktree`, is not a
// file any host writer posse runs was measured replacing. A Deny inside a
// grant on some other file has the same mechanism and is Not decided here —
// if wall-watch ever files one, this is the function its name joins.
//
// IT ANSWERS THE REPOSITORY and not a bool, because the caller compares that
// answer to the SESSION DIR: a deny is this shape only when the git dir is
// the cwd's own (the call site says why, and what keying on containment alone
// was measured to refuse besides).
func bwrapGitConfigRepo(d string) string {
	gitDir := filepath.Dir(d)
	if filepath.Base(d) != "config" || filepath.Base(gitDir) != ".git" {
		return ""
	}
	return filepath.Dir(gitDir)
}

// bwrapDirMaskOver answers which of step 5's DIRECTORY masks covers p — the
// ssh dir allowlist, a read-deny dir or a credential store — with the bead
// that shape is from, or "" when none does. A mask whose own mount the
// render SKIPPED covers nothing, so the argv decides and not the list: that
// is the same test the render makes before it skips a /dev/null over p, read
// back from the wall rather than from the design, so the two cannot say
// different things about one path.
func bwrapDirMaskOver(argv []string, carve SeatbeltCarveOut, p string) (string, string) {
	for _, d := range carve.SSHDir {
		if underDir(d, p) && bwrapMountedAt(argv, d) {
			return d, "ranger-base-12ggv"
		}
	}
	for _, d := range carve.DenyReadDir {
		if underDir(d, p) && bwrapMountedAt(argv, d) {
			return d, "ranger-base-k06gw"
		}
	}
	for _, d := range carve.ReadMaskDir {
		if underDir(d, p) && bwrapMountedAt(argv, d) {
			return d, "ranger-base-a8qfg"
		}
	}
	return "", ""
}

// bwrapMountedAt reports whether the RENDERED argv puts a mount at p. The
// report asks the argv and not the lists, for BwrapReport's reason: a list
// entry says what the design asked for and the argv says what the wall is.
// Destinations only — a bind's SOURCE is not a mount point — which is what
// bwrapMountVerbs' Dst index says per option.
//
// THROUGH bwrapMounts AND NOT A SWITCH OF ITS OWN (ranger-base-r9slg): this
// was the second hand-copy of that grammar, and the two could disagree about
// which options make a mount in the direction nothing would notice — a verb
// one reader knows and the other does not is a path the wall report calls
// unmounted while the D2(b) predicate calls it pinned, or the reverse.
func bwrapMountedAt(argv []string, p string) bool {
	for _, m := range bwrapMounts(argv) {
		if m.Dst == p {
			return true
		}
	}
	return false
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

// bwrapAnchorStep is the anchor this render keeps inside a host directory it
// is about to mount over: an empty file, so a host `rmdir` of that directory
// is refused with ENOTEMPTY and cannot take the mount off a live seat.
//
// ONE FUNCTION FOR THE WHOLE CLASS (ranger-base-ua625 finding 1). The class
// is "a mount point THIS LAUNCHER leaves EMPTY on the host", and it has five
// members with nothing in common but that shape: an Anchored deny directory
// (step 3), a sibling runtime's credential store under the read-mask tmpfs
// (step 5c), a private bind over a dir of claude's shared state (step 2), and
// — since ranger-base-bncbk — ~/.ssh (step 5a) and a read-masked state dir
// (step 5b).
// The first was anchored by a list of seven names under ranger-base-v84cn and
// the rest were not, because the list was the thing that generalized and the
// shape was never written down. Every one of the five calls this, so a SIXTH
// member is anchored by calling it and not by joining a table — the lesson
// the watch list already learned in bwrapArgsWatched's head comment, and
// TestQAWallAnchorCoversEveryWatchedDirectoryMountOrPricesIt is what reds
// when a new arm joins neither.
//
// THERE IS NO EDGE LEFT: THE CLASS IS EVERY TMPFS OVER A HOST DIRECTORY
// (ranger-base-bncbk). It used to be drawn at "a mount point THIS LAUNCHER
// leaves empty", which excluded steps 5a and 5b — they mounted over a
// directory the host made and the host filled, and they skipped an absent
// one, so posse neither created nor emptied them and a seed there would have
// been posse writing into the operator's own area. That edge did not survive
// its own residual: an ~/.ssh or state dir ABSENT at launch got no mount and
// no watch, so a credential the host wrote during the seat's life was
// readable, silently, until the seat ended. Both arms make their directory
// now, which puts them in the class on the old definition — and leaves the
// definition with nothing to exclude. Every arm that mounts a tmpfs or a bind
// over a host DIRECTORY calls this function; the only paths that get no
// anchor are the ones nothing can rmdir (a file) and the ones the seat writes
// through (a dir bound over itself, and the Hide mounts), both priced in
// TestQAWallAnchorCoversEveryWatchedDirectoryMountOrPricesIt.
//
// AN ANCHOR THAT IS ALREADY THERE EMITS NO STEP. PrepStep's rule is that a
// step names a path that is NOT there — prepRequire reads the list's
// emptiness as "this host already has the whole wall" — and until
// ranger-base-bncbk this function broke that rule for a directory whose
// anchor had survived an earlier launch, and the cost was that
// applyRecordReach and applyWallWitness abstained on every box whose wall had
// been up once. Nothing measured it, because the one pin that holds the rule
// (TestQAEveryBwrapPrepStepNamesAPathThatIsNotThere) runs over a fixture
// whose every credential store sits inside a blanket /tmp grant and so
// renders no anchor at all; 5a put a real one in that render's way, which is
// how it was found. THE PIN HAS A SECOND ARM NOW
// (TestQAEveryBwrapPrepStepOverAnchoredMountsNamesAPathThatIsNotThere,
// ranger-base-k45vt): the same invariant over a claude render under a root no
// temp grant reaches, so the private copies, the anchored deny dirs and the
// credential stores are graded too, with a floor per class — the fix and the
// arm that would have caught it landed a bead apart.
//
// LSTAT AND NOT STAT, which is the other half of the same finding. The deny
// arm reached here through bwrapStat — os.Stat, which FOLLOWS — so a link
// standing at an Anchored deny name answered `dir` for its target and the
// seed wrote <target>/.posse-wall-anchor, a stray name at a path posse did
// not choose. The anchored names are inside a seat's write grant by the same
// test the Masked arm lstats for (ranger-base-l751c), and a seat cannot plant
// the link while the mount is up — a rename cannot cross a mount point — so
// the window is after a detach, which is also a kill. A link or a file at the
// name therefore gets no anchor and no refusal: nothing rmdirs either of
// them, so there is no mount to lose, and the arms that care what the name is
// have already said so in their own words.
//
// THE COST IS A STRAY EMPTY DOTFILE in a directory posse did not create, and
// it is paid in a sibling runtime's config dir, in claude's own state dirs,
// and — since ranger-base-bncbk — in the operator's ~/.ssh and in rex's tsnet
// state dir. It is the cost ranger-base-v84cn already accepted for claude's
// swept dirs, the security lane accepted the last two on ranger-base-lin9y,
// and it buys at each of them what it bought there: the member that is NOT
// inert to its reader — a directory something loads code from — is excluded
// by its producer and never reaches this function
// (SeatbeltCarveOut.Anchored's last paragraph). ssh is inert to it by the
// same test: it opens the names it resolves, and StrictModes checks modes
// rather than extra names.
func bwrapAnchorStep(d, why string) (PrepStep, bool, error) {
	fi, err := os.Lstat(d)
	switch {
	case err != nil && !os.IsNotExist(err):
		return PrepStep{}, false, bwrapStatRefusal("anchor", d, err)
	case err == nil && !fi.IsDir():
		return PrepStep{}, false, nil
	}
	anchor := filepath.Join(d, wallAnchorName)
	if _, err := os.Lstat(anchor); err == nil {
		return PrepStep{}, false, nil // already anchored by an earlier launch
	} else if !os.IsNotExist(err) {
		return PrepStep{}, false, bwrapStatRefusal("anchor", anchor, err)
	}
	return PrepStep{PrepSeed, anchor, why}, true, nil
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

// liveBwrapRuntimeDirs is the whole of the carve-out's Hide list with this
// process's answers — the launcher's, which is the seat's: one OS account
// (ADR 0080 D1). It is bwrapRuntimeDirs plus bwrapCrashStoreDirs as they are
// on this host (ranger-base-s7tkc).
func liveBwrapRuntimeDirs() ([]string, error) {
	home, _ := os.UserHomeDir()
	out, err := bwrapRuntimeDirs(os.Getenv("XDG_RUNTIME_DIR"), os.Getuid(), home)
	if err != nil {
		return nil, err
	}
	return dedupeStrings(append(out, bwrapPresentDirs(bwrapCrashStoreDirs)...)), nil
}

// bwrapCrashStoreDirs is where a host keeps what a crash wrote out
// (ranger-base-s7tkc, fix (a) for ranger-base-al0d0): systemd-coredump's
// store, whose cores carry an ACL granting the crashed process's own uid
// read, and the persistent and volatile journals, which carry each crash's
// COREDUMP_ENVIRON and which coredumpctl and journalctl read as FILES. Every
// seat and every host process share one uid, so without this a seat reads a
// same-uid peer's memory and environment off a crash that already happened.
//
// HIDDEN, NOT DENIED: an empty tmpfs over each, by the 1b loop, like the
// runtime dir. Logging FROM a seat is untouched: journald's input socket is
// /run/systemd/journal/socket, under none of these.
var bwrapCrashStoreDirs = []string{
	"/var/lib/systemd/coredump",
	"/var/log/journal",
	"/run/log/journal",
}

// bwrapPresentDirs is cands resolved, keeping only what is a directory on
// this host. SKIPPED, NOT REFUSED, on every other answer — unlike the
// runtime dir, nothing a seat runs needs these, so there is no wrong guess
// to refuse: a host without systemd-coredump, or with a volatile-only
// journal, has nothing there to read; a name that is a file is not a store
// this list knows; and a stat this uid is refused is a directory the seat,
// at the same uid, cannot reach either.
func bwrapPresentDirs(cands []string) []string {
	var out []string
	for _, c := range cands {
		r := absResolve(c)
		if exists, dir, err := bwrapStat(r); err == nil && exists && dir {
			out = append(out, r)
		}
	}
	return out
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
	// either (sessionGitGrants), and every walled worktree session — this
	// tier included since ranger-base-pe5g5 — is detached by the launcher on
	// purpose, before this row is asked (PrepareSessionHead). Nothing to say.
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
	// bwrapMountsName is the walls among the argv's mounts, which `posse
	// wall-watch` holds the seat to (ranger-base-0qpsw D1; wallwatch.go has
	// the format).
	bwrapMountsName = "bwrap.mounts"
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
	args, _, err := a.renderBwrapWall(ag, rt, cwd, wallWrite{})
	return args, err
}

// renderBwrapWall is RenderBwrap with the render itself handed back, so that
// the report can print the wall it just WROTE rather than a second one it
// composed. SeatbeltReport renders its profile for the same reason — `posse
// gates` is where an operator reads the wall off the output, and a report
// whose head line names a file that is not there is worse than no report.
//
// `w` SAYS WHICH OF THE RENDERS THIS IS (wallWrite, wall.go). The render
// below is identical in all of them — the persona's LIVE gates dir is an
// input to it either way, so the mount set a report prints and a probe
// judges is the set a launch would grant — and what `w` chooses is where the
// two files land and what becomes of the prep: made (a launch), skipped (a
// report), or REQUIRED to be there already (a reader's probe, which applies
// the argv and may not write a caller's HOME — ranger-base-qd9ei). Only a launch writes
// `a.GatesDir(ag.Name)`: a report that rewrote it left a running seat's git
// refused by its own shim (ranger-base-atq78/6s5mo, and see wallWrite's own
// header for the measurement).
func (a *App) renderBwrapWall(ag *AgentFile, rt *Runtime, cwd string, w wallWrite) (string, bwrapRendered, error) {
	gatesDir := a.GatesDir(ag.Name)
	dest := w.Dir
	if w.live() {
		dest = gatesDir
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return "", bwrapRendered{}, err
		}
	}
	r, err := a.bwrapRender(ag, rt, cwd, gatesDir)
	if err != nil {
		return "", bwrapRendered{}, err
	}
	switch w.Prep {
	case prepMake:
		if err := runBwrapPrep(r.Prep); err != nil {
			return "", bwrapRendered{}, err
		}
	case prepRequire:
		// A READER'S PROBE (ranger-base-qd9ei). It applies what it renders,
		// so it cannot skip the prep the way a report does — and it may not
		// make it, because the paths are whatever HOME typed the command.
		// Every step is emitted only where its path is ABSENT, so a step at
		// all is the answer: this wall cannot come up without a write only a
		// launch may make, and the probe's job is to say it measured nothing.
		if len(r.Prep) > 0 {
			return "", bwrapRendered{}, &wallPrepAbsentError{Mechanism: WallBwrap, Step: r.Prep[0], N: len(r.Prep)}
		}
	}
	args := filepath.Join(dest, bwrapArgsName)
	if err := os.WriteFile(args, bwrapArgsBytes(r.Argv), 0o644); err != nil {
		return "", bwrapRendered{}, err
	}
	if err := os.WriteFile(filepath.Join(dest, bwrapTextName), []byte(bwrapReadable(ag.Name, args, r.Argv, r.Prep, r.Absent)), 0o644); err != nil {
		return "", bwrapRendered{}, err
	}
	// The watcher's list, beside the argv it describes and written by the
	// same render (ranger-base-0qpsw D1). After the prep, so a mount point
	// the launcher just made resolves like every other one.
	if err := os.WriteFile(filepath.Join(dest, bwrapMountsName), wallWatchMountsBytes(r.Watch), 0o644); err != nil {
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
	// Watch is the walls among Argv's mounts (bwrapArgsWatched), which the
	// launch hands `posse wall-watch` as the mounts file (ranger-base-0qpsw
	// D1).
	Watch []WallWatchMount
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
	// The runtime dir's control endpoints (ranger-base-ludpp), and the
	// host's crash stores (ranger-base-s7tkc). Here and not in
	// SeatbeltCarveOut, because seatbelt has no spelling for it.
	if r.Carve.Hide, err = liveBwrapRuntimeDirs(); err != nil {
		return bwrapRendered{}, err
	}
	// The ssh keys a seat that is not the pusher must not read
	// (ranger-base-ighvi): read masks like the runtime credentials, so they
	// ride DenyRead and BwrapArgs step 5 renders them. RenderSeatbelt adds
	// the same list through the same helper (ranger-base-7m3up).
	if _, err := withSSHKeyReadDeny(ag, cwd, &r.Carve); err != nil {
		return bwrapRendered{}, err
	}
	withRexNodeReadDeny(&r.Carve)
	// The sibling runtimes' credential STORES, masked as directories
	// (ranger-base-a8qfg). Here and not in SeatbeltCarveOut for Hide's
	// reason: seatbelt has no need of the list, because an SBPL literal deny
	// covers a path that is not there and a /dev/null bind cannot — the whole
	// of why this field exists. Same stateDirs the credential FILE denies
	// were computed from one call above, so the two walls cannot disagree
	// about which store this seat owns.
	r.Carve.ReadMaskDir = credentialStoreReadMaskDirs(stateDirs)
	r.Set, r.Keep = BwrapWritable(ag, writable, cwd)
	// The seat's own /tmp (ranger-base-7wdb7): the host dir BwrapArgs binds
	// over /tmp, and the grants it replaces out of the set, so the report
	// prints the wall the argv builds.
	home, _ := os.UserHomeDir()
	if r.Carve.Tmp = bwrapSeatTmp(home, cwd); r.Carve.Tmp != "" {
		r.Set = bwrapDropTmpGrants(r.Set, os.Getenv("TMPDIR"))
	}
	// claude's shell snapshots and session-env, one private copy per seat
	// (ranger-base-5632o). Here and not in SeatbeltCarveOut for the reason
	// Tmp is: a seatbelt profile cannot bind.
	//
	// THE WRITE SET AND NOT THE RUNTIME'S STATE GRANTS (ADR 0100 D4(b), THE
	// PRIVATE SET FOLLOWS THE GRANT). The question this gate answers is
	// "can a seat behind THIS render write `<cfg>`", because that is who can
	// rename a member of it out from under a live sibling (D4's Context,
	// MEASURED). The launching runtime's state grants are a different
	// question and a narrower one: a codex or grok PID carrying a
	// `writable:` extra over claude's config dir reaches every member of
	// that table, and under the old gate rendered not one of the binds — so
	// it could swap a claude seat's shell-snapshots directory, which is code
	// the victim sources ahead of every Bash command, and got EBUSY nowhere.
	//
	// r.Set HERE IS THE SAME ANSWER AS THE FINAL SET, and the difference is
	// worth naming rather than leaving to be re-derived. What comes between
	// this line and the render is D23's bwrapBindExisting, which only
	// REMOVES a no-create grant this host does not have. So r.Set is a
	// superset of what BwrapArgs will bind — and for a member whose grant
	// was removed, BwrapArgs step 2c renders nothing at all (its
	// `writeGranted(grants, pb.Dst)` arm, which emits no mount and no prep).
	// Gating on the superset and rendering on the final set cannot disagree;
	// gating LATER would mean composing this table after D23 and prepending
	// it, for an identical argv.
	//
	// claudePrivateStateGate is the one condition on that: where this
	// mechanism's wall stands down the gate reads what it read before D4(b),
	// and its own head comment says why a grant-keyed table on such a box
	// would be a mount with no EBUSY behind it.
	r.Carve.Private = claudePrivateSessionState(home, cwd, claudePrivateStateGate(r.Set, runtimeStateGrants(stateDirs)))
	// projects/ read-only but for the seat's own key dir, and memory/
	// read-only everywhere (ranger-base-ajkqx). After the session state, in
	// the same step, for the same reason: a seatbelt profile cannot bind.
	r.Carve.Private = append(r.Carve.Private, claudeProjectsBinds(home, runtimeStateGrants(stateDirs), a.claudeOwnSessionTree(cwd))...)
	// The seat's own object store over `<common>/objects`, and the shared
	// store read-only at a side path (ranger-base-21i98, ADR 0099 D1). A
	// private bind for the same reason as the two above — seatbelt cannot
	// bind — and in this step because it lands after every write grant, so
	// it is the deepest mount over that name even when a store of record in
	// the same repo grants `<common>` whole.
	//
	// The same `home` the private state and the private /tmp were composed
	// from, because the store's source is the third member of that one wall
	// dir (ADR 0100 D1) and a second reading of the home would be a second
	// directory. It is the one private producer that REFUSES rather than
	// rendering nothing when the home is unknown: rendering nothing here
	// leaves the seat the shared object store read-write.
	storeBinds, err := seatStoreBinds(ag, home, cwd)
	if err != nil {
		return bwrapRendered{}, err
	}
	r.Carve.Private = append(r.Carve.Private, storeBinds...)
	// ...which is what SeatbeltCarveOut's Narrow says in SBPL; the binds
	// above are this mechanism's spelling of it (ranger-base-r2uxm).
	r.Carve.Narrow = nil
	// sessions/ and paste-cache/ are private binds here
	// (claudeSessionStateDirNames), history.jsonl a private file bind; the
	// read-and-write deny is seatbelt's spelling (ranger-base-yaha3,
	// ranger-base-wj93u).
	r.Carve.DenyAll = nil
	// The sockets a seat may not connect to are this mechanism's own
	// question and it answers them with a mount and a Linux-only wall, not
	// with a path rule: grok's daemon/run is a private tmpfs in the step
	// below. DenyConnect is the seatbelt spelling, like Narrow and DenyAll
	// above.
	// Residual: ranger-base-l2gk9.
	r.Carve.DenyConnect = nil
	// codex's per-process tmp/ and shell_snapshots/, and grok's daemon/run/
	// (ranger-base-76mcd): a private empty tmpfs
	// each (ranger-base-6ob20). SeatbeltCarveOut denied them, which is the
	// seatbelt reading; here the deny comes back out, because the deny step
	// mounts after this one and its read-only bind FROM THE HOST would cover
	// the tmpfs with the very dir it exists to keep out. Same home as the
	// deny was spelled against, so the two cannot name different dirs.
	if eph := runtimeEphemeralDirs(wallHome(), runtimeStateGrants(stateDirs)); len(eph) > 0 {
		kept := r.Carve.Deny[:0:0]
		for _, d := range r.Carve.Deny {
			if !containsString(eph, d) {
				kept = append(kept, d)
			}
		}
		r.Carve.Deny = kept
		// THE LOOSE PARENTS FIRST, BOUND IN PLACE (ADR 0100 D4(a), the one
		// miss its census found; bwrapRefuseUnpinnedDst' head comment names
		// that miss's bead by id and nothing else, which is where it is
		// named).
		// Where a member name has a parent in it, that parent lies strictly
		// inside the granted CLI home and was a mount of nothing, so a
		// leftover of this seat renames it and plants its own directory
		// under the name this seat's CLI opens. An in-place read-write bind
		// makes it a mount, which is what refuses that rename;
		// runtimeEphemeralDirParents' head comment has why this list is
		// empty for a CLI whose members are direct children, why it grants
		// nothing, and why it must come BEFORE the tmpfs members below
		// rather than after them.
		for _, p := range runtimeEphemeralDirParents(wallHome(), runtimeStateGrants(stateDirs)) {
			r.Carve.Private = append(r.Carve.Private, PrivateBind{Src: p, Dst: p})
		}
		for _, d := range eph {
			r.Carve.Private = append(r.Carve.Private, PrivateBind{Dst: d})
		}
	}
	// claude's debug/ (ranger-base-ntt1w) comes out of Deny for the same
	// reason and in the same direction, but its private bind is already in
	// the list above — it is a member of claudeSessionStateDirNames, so the
	// seat KEEPS its own debug log on the host rather than getting an empty
	// tmpfs. Only the deny has to go: the deny step mounts after the private
	// one, and its read-only bind from the host would put the shared
	// directory back over the seat's copy. Spelled against wallHome(), the
	// home SeatbeltCarveOut denied it under, so the two cannot name different
	// dirs.
	if dbg := claudeDebugDir(wallHome(), runtimeStateGrants(stateDirs)); dbg != "" {
		kept := r.Carve.Deny[:0:0]
		for _, d := range r.Carve.Deny {
			if d != dbg {
				kept = append(kept, d)
			}
		}
		r.Carve.Deny = kept
	}
	// D23, after D20's substitution and before the render: what is left of
	// the set is what posse will bind, and a no-create grant that is not
	// there has been taken out of it, so BwrapArgs can ask the launcher for
	// every missing grant it still sees.
	if r.Set, r.Absent, err = bwrapBindExisting(r.Set, bwrapNoCreate(ag, cwd)); err != nil {
		return bwrapRendered{}, err
	}
	if r.Argv, r.Prep, r.Watch, err = bwrapArgsWatched(cwd, r.Set, r.Carve, r.Keep...); err != nil {
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
		// A TEST BINARY'S OWN FENCE, and nothing in a launcher
		// (prephostfence.go, ranger-base-bncbk): steps 5a and 5b now mkdir
		// and anchor, and two of the render's host inputs do not move when a
		// fixture sets $HOME — so a test that renders a whole wall is one
		// unfenced input away from planting a file in the operator's ~/.ssh.
		if why := prepHostFenceRefusal(s); why != "" {
			return Die("cage %s (%s) refuses to prepare %s (%s): %s",
				CageSeatbelt, WallBwrap, s.Path, s.Why, why)
		}
		switch s.Kind {
		case PrepMkdirAll:
			if err := os.MkdirAll(s.Path, 0o755); err != nil {
				return Die("cage %s (%s) cannot prepare %s (%s): %v — the launcher makes this outside the namespace because bwrap binds only a source that exists (ADR 0080 D17/D18)",
					CageSeatbelt, WallBwrap, s.Path, s.Why, err)
			}
		case PrepMkdirPrivate:
			// 0700, and the parents too: see PrepMkdirPrivate. MkdirAll is
			// a no-op over a directory that is already there and leaves its
			// mode alone, which is the right answer for a name whose owner
			// got there first.
			if err := os.MkdirAll(s.Path, 0o700); err != nil {
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
			err := seedFileWhole(s.Path, seedBody(s.Path))
			if os.IsExist(err) {
				continue
			}
			if err != nil {
				return Die("cage %s (%s) cannot prepare %s (%s): %v — the launcher makes this outside the namespace because bwrap binds only a source that exists (ADR 0080 D17/D18)",
					CageSeatbelt, WallBwrap, s.Path, s.Why, err)
			}
		case PrepMaskPoint:
			// O_EXCL for PrepSeed's reason: whatever appeared since the
			// render is what the mask covers. 0600 and empty, as claude
			// creates these itself.
			f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if os.IsExist(err) {
				continue
			}
			if err != nil {
				return Die("cage %s (%s) cannot prepare %s (%s): %v — the launcher makes this outside the namespace because a mount needs a name to cover (ADR 0080 D17/D18)",
					CageSeatbelt, WallBwrap, s.Path, s.Why, err)
			}
			if err := f.Close(); err != nil {
				return err
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
// `·` CARRIES THREE STATEMENTS, and every one of them is "this entry is in a
// list and renders no mount": the rename seal, which needs none (a mount is a
// statement about a mount POINT); D23's declared-but-not-granted, which has
// no source to bind; and a credential read mask whose file is not there,
// which has no name to cover (ranger-base-a8qfg). One marker, because an
// operator scanning the block for what IS mounted wants one thing to skip and
// the parenthetical says which of the three it is. The third one is the only
// one of the three that is a HOLE, and its parenthetical says so.
//
// IT READS THE RENDERED ARGV AND THE PREP, not the lists, which is the
// difference that matters for a report whose whole job is to be checkable.
// `posse gates` is where ADR 0015 §2's claim is read off the output, and a
// report composed from the inputs would agree with the design while
// disagreeing with the wall. So the order below is the argv's order, which is
// the wall's semantics: later covers earlier.
func (a *App) BwrapReport(ag *AgentFile, rt *Runtime, cwd string, out io.Writer) error {
	// A REPORT, so: rendered into this process's read-only-by-intent dir and
	// with no prep run (wallWrite, wall.go). The head line still names a file
	// that is there — which is why this report renders at all — and the live
	// wall one line below is named and untouched (ranger-base-6s5mo).
	w, err := a.wallWriteToRead(ag.Name, prepSkip)
	if err != nil {
		return err
	}
	args, r, err := a.renderBwrapWall(ag, rt, cwd, w)
	if err != nil {
		return err
	}
	set, keep, carve, prep := r.Set, r.Keep, r.Carve, r.Prep
	fmt.Fprintf(out, "  %s rendered for cwd %s (cage %s (%s); the mount set below, later covers earlier):\n",
		AbbrevHome(args), AbbrevHome(cwd), CageSeatbelt, WallBwrap)
	fmt.Fprintf(out, "    (a REPORT copy, rendered from this shell's environment and with NO prep run. The wall a launch reads is %s and this command did not write it — only a launch does, so a reader cannot replace what a running seat is behind; ranger-base-6s5mo)\n",
		AbbrevHome(filepath.Join(a.GatesDir(ag.Name), bwrapArgsName)))
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
		if carve.Masked(p) {
			fmt.Fprintf(out, "    x %s (/dev/null bound over it, never seeded — a cached server answer claude obeys; ranger-base-l751c)\n", AbbrevHome(p))
			continue
		}
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
	// A READ MASK IS REPORTED AS THE ARGV RENDERED IT, which for this list is
	// three different statements and used to be one (ranger-base-a8qfg). A
	// /dev/null mask needs a file to cover, so a literal whose file is not
	// there renders NOTHING — and this line said "/dev/null bound over it"
	// about it anyway, which is the one sentence an operator reads to check
	// ADR 0080's "parity names each missing file". A literal under a
	// directory mask renders nothing either, and is covered all the same.
	for _, p := range shallowestFirst(carve.DenyRead) {
		d, bead := bwrapDirMaskOver(r.Argv, carve, p)
		switch {
		case d != "":
			fmt.Fprintf(out, "    r %s (NO mount of its own — the empty tmpfs over %s/ covers the name whether or not anything is at it; %s)\n", AbbrevHome(p), AbbrevHome(d), bead)
		case !bwrapMountedAt(r.Argv, p):
			fmt.Fprintf(out, "    · %s (NO MOUNT; ranger-base-1c0r6)\n", AbbrevHome(p))
		default:
			fmt.Fprintf(out, "    r %s (/dev/null bound over it — credential-store literal; ranger-base-hw18, ADR 0019 D2)\n", AbbrevHome(p))
		}
	}
	for _, d := range shallowestFirst(carve.SSHDir) {
		fmt.Fprintf(out, "    r %s/ (empty tmpfs over it with %s bound back read-only — this seat is not the pusher; ranger-base-12ggv)\n", AbbrevHome(d), strings.Join(sshDirAllowlist, " and "))
	}
	for _, d := range shallowestFirst(carve.DenyReadDir) {
		fmt.Fprintf(out, "    r %s/ (empty tmpfs over it, nothing bound back — the rex tsnet node's state, a tailnet credential no seat reads; ranger-base-k06gw)\n", AbbrevHome(d))
	}
	for _, d := range shallowestFirst(carve.ReadMaskDir) {
		if !bwrapMountedAt(r.Argv, d) {
			fmt.Fprintf(out, "    · %s/ (NO mount: this credential store sits inside a directory the seat already sees empty, or inside a write grant — and an empty tmpfs over a grant is a seat writing where nothing survives the seat. The credential FILE above keeps its own mask, so what is left is ADR 0080's snapshot-at-launch residual; ranger-base-a8qfg)\n", AbbrevHome(d))
			continue
		}
		fmt.Fprintf(out, "    r %s/ (empty tmpfs over it with %s bound back read-only where they exist — another runtime's credential store, masked as a DIRECTORY so a credential created after this seat launched is covered too; ranger-base-a8qfg, ADR 0019 D2)\n", AbbrevHome(d), strings.Join(credentialStoreAllowlistNames(), ", "))
	}
	// The prep, which an operator can read nowhere else: these are writes
	// posse makes on the HOST, outside the namespace, in the seat's name —
	// at a LAUNCH. This report ran none of them, and says so, because the
	// alternative is a reader making them from whatever environment happens
	// to be in the shell (ranger-base-6s5mo).
	for _, s := range prep {
		fmt.Fprintf(out, "    ! %s (prep the launcher runs before the wall, NOT run by this report: %s)\n", s, s.Why)
	}
	// The socket wall (ranger-base-6rndn), AND IT CARRIES NO MARKER. Every
	// marker in this report is a statement about a MOUNT, and this layer
	// renders none: it is a word on the launch line. A sixth marker would be
	// a sixth thing an operator has to learn, in the report whose whole point
	// is that it speaks the other mechanism's vocabulary — so this is a
	// sentence, indented with the rest, the way the head note above it is.
	//
	// BOTH ARMS PRINT. An operator reading a wall has to be able to tell the
	// half that is up from the half that is not, and the degraded arm is the
	// one that must never be silence — a seat whose socket wall is absent can
	// still connect(2) to any host pathname socket in its view. The degraded
	// sentence leads with its own loud word and needs no marker to be found.
	if d := WallUnixDegraded(WallBwrap); d != "" {
		fmt.Fprintf(out, "    %s\n", d)
	} else {
		fmt.Fprintf(out, "    socket wall: `%s` on the line inside the cage — a Landlock domain handling RESOLVE_UNIX, so connect(2) and sendmsg(2) to every host pathname socket in this seat's view are EACCES, including one an uncaged process binds after launch (ADR 0080 D27). No allow rule for that right: MEASURED, a seat needs none. The domain also handles REFER and grants it on `/`, which is Landlock's only way to say \"leave rename(2) and link(2) alone\" — handled-and-granted is a no-op, while handling no refer right at all would make every cross-directory rename and link EXDEV for everything in this seat (ranger-base-p5i2c). The same domain scopes SIGNAL (LANDLOCK_SCOPE_SIGNAL): a process in this seat can signal only what it forked, so kill(2) to the launcher, another seat or any host process is EPERM, signal 0 included, and the launcher's signals to the seat are unaffected (ranger-base-e5un1). The mounts above are unchanged by it.\n", WallUnixVerb)
	}
	// No siblings: a mount cannot name the scratch namespace beside a file,
	// and D19 makes it moot by granting claude's config DIRECTORY.
	a.reportConstitutionGrants(ag, out, set, nil, carve.Deny)
	return nil
}
