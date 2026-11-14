package posse

// THE LANDED TREE IS READ AFTER EVERY LANDING, OUTSIDE THE LOCK (ADR 0088
// D1-D4, from ranger-base-oe9ja). This file is the runner; filing is
// ranger-base-ddgfr's and this slice PRINTS the verdict.
//
// WHAT IT IS FOR. ADR 0087's land check judges the REPLAY arm, inside the
// lock, before the swap. It cannot see the other half of the problem: two
// seats whose halves are each green on their own base, landed one after the
// other, combining on main into a tree neither seat ever had. MEASURED
// 2026-11-11 — ranger-base-s64a5 shipped a stale-register pin,
// ranger-base-in8dr added the arms that fire it, the replay type-checks so
// 0087's gate passed it, and `make test` then exited 2 in 31.9s for every
// seat on the box. The verify lane found it about five hours later, because
// nothing reads main's exit code: ci.yml fires on the operator's push.
//
// SO THE READER IS THE LAUNCHER, AFTER THE FACT. Five differences from
// landcheck.go, and every one of them is a decision in ADR 0088 rather than
// an implementation detail:
//
//  1. It runs on BOTH landing arms, because the fast-forward arm is the one
//     0087 never judges (D1, 0087 residual (1)).
//  2. It runs OUTSIDE the lock and never blocks (D2). The landing has
//     already happened; this buys detection with attribution, in minutes
//     instead of hours, and it cannot strand the fleet.
//  3. It runs CAGED, in a detached worktree of the landed commit (D3).
//     Landing adds no human review, so the code is no more trusted after it
//     lands than before — and this one runs `make`, which is the whole point
//     (the thing that went red was a door, and a door is `go test`). If no
//     cage renders, the answer is "no answer": ⚠ and nothing recorded. It
//     NEVER falls back to running uncaged.
//  4. One run at a time, and runs COALESCE (D4). When a run ends and main
//     has moved, the next run judges the newest tip only.
//  5. The judge is the copy from the last tree recorded GREEN, not the copy
//     in the tree being judged (D1). A landing that weakens the script does
//     not change what judges it.
//
// WHAT IS NOT HERE. D5's attribution — one bead per green-to-red stretch —
// is ranger-base-ddgfr. This records the verdict and prints it, which is
// what makes that bead a reader of a record rather than a second runner.
//
// THE RESIDUALS, STATED. (1) A semantic conflict only a suite arm reaches is
// still found by the next seat's `make test`: the arms are fifteen minutes
// and three suite slots per landing and are deliberately not run (D6).
// (2) Between the landing and the verdict a dispatch can branch from a red
// main. (3) A repo with no `scripts/posse-landed-check` gets nothing.
// (4) `posse kill`'s landing arm schedules nothing — a kill must stay
// instant (herdrback.go says so where it takes the lock without waiting),
// and this runner is joined by the pass that starts it, so a kill that
// scheduled one would either block the cockpit for a minute or leak a caged
// child into a process that is about to exit. A tree landed by a kill is
// read by the next landing into that repo, one tip later. (5) The child
// inherits the launcher's environment with TMPDIR and GOCACHE redirected
// and nothing else (residual: ranger-base-5m5yq), which is ADR 0087 D1's
// rule and the contract's: what a repo's check needs out of the environment is the repo's to decide, and this repo's
// scrubs make's own variables for itself after its author measured a green
// over an empty room (docs/notes.d/ranger-base-r3pqf.md). Posse scrubs
// nothing on a repo's behalf.
//
// WHAT IT COSTS, and the one reading this file quotes, because it is the
// reading that decided whether D3 was affordable at all: **the cage is
// free**. MEASURED 2026-11-11 on the D1 host, judging 436f467c behind
// bwrap — 70s, 71s, 71s at a one-minute load of 4.3-6.4, against 72s for
// the same script over the same shape of detached worktree with no wall at
// all. Every other reading, and the three-entry write set that makes it
// work, is in docs/notes.d/ranger-base-5fond.md, which is the one place they
// are said.

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// landedCheckScript is D1's contract, and it is ADR 0087 D1's verbatim with
// one word changed: no arguments, cwd is the tree to judge, exit 0 is green,
// and the last lines of combined output are the cause. A separate path from
// landCheckScript because the two run under different rules — the land check
// executes nothing the branch wrote and this one runs the tree's own `make`
// — so a repo may ship either, both, or neither.
const landedCheckScript = "scripts/posse-landed-check"

// landedCheckKey is D4's whole record: one repo-level git config entry
// holding `<tree-id> <green|red>`.
//
// KEYED ON THE TREE for landCheckKey's reason — the check reads BYTES, so
// two commits with one tree are one question — and it is what makes a
// coalesced run free when nothing moved: the record already names the tip's
// tree and there is nothing to ask.
//
// ONE KEY AND NOT ONE PER TREE, also landCheckKey's reason: this is a cache
// with one useful entry, and a config section that grew an entry per landing
// would be a store nothing ever prunes.
const landedCheckKey = "posse.landedCheck"

// landedCheckLimit bounds one run, and what it bounds is a HANG and not a
// price (ADR 0088 D6.1, amended by ranger-base-7idk4). Its own constant at
// ten minutes, deliberately NOT generatorTimeout any more: that was one
// number doing two jobs — the price at which the design goes back to
// architecture, and the kill — and D6 derives the judge's body from the
// Makefile on purpose, so the body grows with every door somebody adds. A
// fixed kill over a growing body gets crossed, and on 2026-11-12 it was:
// the judge took 125.3s twice against generatorTimeout's 120s and was
// killed. D3 spells a deadline as SILENCE, so every landing went unjudged,
// D5 never filed, and the only signal the crossing could produce was the
// absence of one.
//
// TEN MINUTES IS 4x THE HIGHEST READING (150s, MEASURED 2026-11-12 uncaged
// over the same tree; the caged readings are in
// docs/notes.d/ranger-base-5fond.md and the 85.1s door-by-door derivation is
// in ADR 0088's amendment). A bound this loose is cheap HERE and nowhere
// else in the landing path, for D2's reason: this judge holds no lock and no
// seat, so a hung one delays the next coalesced judgement by at most one
// landing gap — median 713s over 167 landings, MEASURED the same day.
//
// THE CEILING IS A SECOND NUMBER AND IS NOT THIS ONE (landedCheckCeiling,
// D6.2). Keeping them apart is the fix: a price that is read off answered
// runs can be reported, and a price enforced by a kill can only be reported
// by the silence it causes.
//
// A deadline here is still NOT a red. It is spelled as silence by
// landChildVerdict, which is the whole doctrine this runner shares with the
// land check.
const landedCheckLimit = 10 * time.Minute

// landedCheckCeiling is D6.2's price: the wall at which this design goes
// back to architecture. It is READ off answered runs and never enforced by a
// kill — every verdict line prints its wall, and a run past this number
// still answers.
//
// 240s is chosen for ATTRIBUTION and not for box time. D5 names every
// landing between the tree last recorded green and this red one, and a judge
// that answers inside the p25 landing gap (253s, MEASURED 2026-11-12 over
// 167 landings on main) keeps that window to one landing most of the time
// (ASSUMED: that is the property worth a price review; the gaps are
// MEASURED). The accepted price is the 110-150s this repo's check costs,
// about 52 landings a day, coalesced and outside the lock.
//
// READING IT IS ranger-base-lhiy1's (D6.3): an answered run over this
// ceiling files one architecture bead carrying the wall, which is how the
// next crossing becomes a bead instead of something a verify lane happens to
// find five hours later. This runner prints the wall and compares nothing.
const landedCheckCeiling = 240 * time.Second

// landedCheckWall is how a run's wall is said on a verdict line (D6.2), and
// it is one function so the three lines cannot drift from each other.
// Seconds with one decimal, which is the unit ADR 0088's readings and its
// 240s ceiling are written in: Duration.String says `1m52s` for a 112s run
// and leaves the reader converting before they can compare it to the
// ceiling.
func landedCheckWall(d time.Duration) string { return fmt.Sprintf("%.1fs", d.Seconds()) }

// landedCheckWorkDir is where the launcher puts one run's scratch: the
// detached worktree, the extracted judge, and the TMPDIR the check's own
// children write into. Under the posse state dir rather than the host's
// /tmp, because every one of them has to be inside the cage's write grant
// and the host's /tmp is the one directory every uncaged process on this box
// also writes (ranger-base-7wdb7's reason, one tier down).
func landedCheckWorkDir(a *App) string { return filepath.Join(a.StateDir, "landed-check") }

// ─── D3: THE CAGE ───────────────────────────────────────────────────────────

// landedCage is one rendered cage: the argv that RUNS and the environment it
// changes. A struct and not an *exec.Cmd because the caller owns the
// deadline — landChildVerdict reads `ctx.Err()`, so the child has to be
// built with the context, and a renderer handing back a started-shaped Cmd
// would be the one place a bound could be forgotten.
type landedCage struct {
	// Argv is the whole command, cage binary first. The judge is its last
	// element, so nothing here can exec the judge without the wall in front
	// of it — which is D3's "never falls back to running uncaged" made
	// structural rather than remembered.
	Argv []string
	// Env is the pairs applied OVER the judge's allowlisted environment
	// (landedJudgeEnv) — never over the launcher's own.
	Env []string
}

// landedCageFn renders that cage around one judge over one tree. It is a
// field on the checker for the reason Dispatcher.CIRead is one: the thing
// most worth pinning here is what happens when the cage REFUSES, and the
// only other way to reach that arm is to run the suite on a box with no
// bwrap and no sandbox-exec — which is a test that passes for the wrong
// reason on this box and cannot be written at all on the other.
//
// Production has exactly one implementation and it is boxLandedCage.
type landedCageFn func(a *App, kind landedCageKind, child, tree string, writable []string, env []string) (landedCage, error)

// landedCageKind is ADR 0097 D1's flag, and it is the WHOLE of the venue's
// difference from the judge: one renderer, one set of masks, one layer list,
// with a flag that adds three things to it.
//
// A FLAG AND NOT A SECOND RENDERER, which is the decision and not a style
// note (D1, ranger-base-hy1aj's lesson). Every mask added to the judge later
// — a runtime state root, a socket directory, a credential nobody has
// declared yet — reaches the venue by construction, because there is nothing
// for it to be added to twice. A second renderer is a layer the venue would
// not inherit, and the one it misses is the one nobody notices.
type landedCageKind int

const (
	// landedCageJudge is ADR 0088 D3's cage: the landed check's judge, which
	// runs this repo's `make` doors over a detached worktree.
	landedCageJudge landedCageKind = iota
	// landedCageVenue is ADR 0097 D1's venue: that cage plus
	// `--unshare-pid --proc /proc`, a private /var/tmp, and
	// POSSE_WALL_VENUE=1. It is where the wall's 32 live pins can nest a
	// bwrap of their own, which is the capability it exists for and the one
	// axis on which it is weaker than a seat (D1's third bullet).
	landedCageVenue
)

// String names the kind in a refusal a human reads.
func (k landedCageKind) String() string {
	if k == landedCageVenue {
		return "wall venue"
	}
	return "judge"
}

// boxLandedCage is D3: the box's own cage tier, bwrap on Linux and
// sandbox-exec on darwin, with the run dir (the landed worktree, its TMPDIR
// and its own build cache) writable and nothing else, and no network but
// loopback (ranger-base-4nzpo).
//
// IT GOES THROUGH THE SAME RENDERERS A SEAT'S WALL GOES THROUGH — BwrapArgs
// and SeatbeltProfile — and not through RenderWall, which is a different
// question with the same shape. RenderWall renders a PERSONA's wall: it
// needs an AgentFile for the PID's denies, a Runtime for the state dirs, and
// a gates dir to write the shims into. None of those exist here. What is
// being caged is not a seat and has no persona; it is one program with one
// write grant, and composing a fake AgentFile to reach the persona renderer
// would be a seat posse announces and does not have.
//
// AN UNAVAILABLE MECHANISM REFUSES rather than returning an empty argv. An
// empty prefix is a run with no wall and nothing said, which is the one
// outcome D3 names by hand.
//
// THE VENUE IS THIS CAGE PLUS EXACTLY THREE THINGS (ADR 0097 D1), and the
// `kind` flag is where they are added. They are added at the END of the bwrap
// argv and after the carve-out, so this render's own masks and the base mount
// are untouched by them: `--unshare-pid --proc /proc` (a pid namespace of the
// venue's own and a procfs over it, which is what lets a process inside write
// its own uid_map and nest a bwrap — ADR 0096 arm C), a private /var/tmp (the
// ADR's own fourth row: without it 18 pins go dark under `--ro-bind / /`), and
// POSSE_WALL_VENUE=1 (the only signal that tells a test process it is the
// venue and not a seat — D1, wallvenue_test.go's wallVenueActive).
//
// ON SANDBOX-EXEC THE VENUE REFUSES. The step does not exist on darwin (ADR
// 0096: no darwin change), and a render that quietly handed back the judge's
// profile would be a wall check measuring in a wall that forbids nesting —
// every member abstaining, exit 3, and D5's watchdog filing silence at a host
// that was never asked to answer. The caller checks the mechanism before it
// gets here (wallcheck.go); this refusal is what makes that structural.
func boxLandedCage(a *App, kind landedCageKind, judge, tree string, writable, env []string) (landedCage, error) {
	// THE READ HALF OF THE CARVE IS NOT EMPTY (ranger-base-6b46n). Under
	// `--ro-bind / /`, and under a seatbelt profile that allows every read
	// it does not deny, an empty DenyRead is not "no narrowing" — it is the
	// whole host's read surface, so the judge read every no-passphrase ssh
	// key this box pushes with and every runtime's credential file. The
	// judge is landed code, and D3 cages it because landing reviews nothing,
	// so it gets the masks a seat that is not the pusher gets: every
	// runtime's credential (no runtime owns this child, so none is spared)
	// and ssh's own key list, through the SAME helper both seat renders
	// call, with no PID — and no PID is not the pusher (bwrapMasksSSHKeys).
	var carve SeatbeltCarveOut
	for _, p := range credentialReadDenyLiterals(runtime.GOOS, nil) {
		carve.DenyRead = append(carve.DenyRead, absResolve(p))
	}
	if _, err := withSSHKeyReadDeny(nil, tree, &carve); err != nil {
		return landedCage{}, err
	}
	withRexNodeReadDeny(&carve) // ranger-base-k06gw
	// THE JUDGE IS NEVER STRONGER THAN A SEAT (ranger-base-4nzpo). The
	// run dir is the judge's own directory, the one judge() made and the one
	// the darwin profile is written into.
	run := filepath.Dir(judge)
	switch m := WallMechanism(); m {
	case WallBwrap:
		if !bwrapAvailableOn(runtime.GOOS) {
			return landedCage{}, Die("the landed check cannot run: this host realizes cage %s with %s and %s (ADR 0088 D3), so there is no wall to put it behind",
				CageSeatbelt, m, orElse(bwrapApplyRefusal(), "it does not resolve on the gates-free PATH"))
		}
		// The carve-out's WRITE half is empty on purpose, and it is the one
		// place this render is DELIBERATELY poorer than a seat's. A seat's
		// write denies are about what a PERSONA may reach inside its grants —
		// the rename seal, the runtime dir's control endpoints. This child's
		// grant is a run dir the launcher just created; there is nothing
		// inside it for a write deny to narrow. Its READ
		// half is the masks above, which are about the whole host and not
		// about the grants, and are not optional for the same reason.
		//
		// The host-scoped masks a seat gets through bwrapRender, which this
		// render does not go through (ranger-base-4nzpo): the user runtime
		// dir hidden (ranger-base-ludpp) and a private /tmp
		// (ranger-base-7wdb7) — here the run's own tmp dir, so it goes with
		// the run. The SYSTEM run dir and the runtimes' own state roots go
		// beside them (landedJudgeHides, ranger-base-8ukiy).
		hide, err := landedJudgeHides()
		if err != nil {
			return landedCage{}, err
		}
		carve.Hide = hide
		// The error is discarded on purpose: the home only decides whether
		// it sits under /tmp, and an unknown home cannot (ranger-base-f1rpw).
		home, _ := os.UserHomeDir()
		carve.Tmp = landedCheckTmp(home, run, tree)
		argv, prep, err := BwrapArgs(tree, writable, carve)
		if err != nil {
			return landedCage{}, err
		}
		if kind == landedCageVenue {
			// ADR 0097 D1's three additions, and the one condition the
			// second of them carries: the /var/tmp bind STANDS DOWN WHEN
			// Tmp DOES. That is one condition and not two — carve.Tmp is
			// empty exactly when the home, the run or the tree is itself
			// under /tmp (landedCheckTmp), which is this package's fixture
			// shape and not a production one — and a private /var/tmp over
			// a tree living under one of them would leave the check nothing
			// to chdir into, for the same reason the private /tmp would.
			var mkdir []PrepStep
			argv, mkdir = wallVenueArgv(argv, run, carve.Tmp != "")
			prep = append(prep, mkdir...)
			env = append(append([]string{}, env...), wallVenueEnvVar+"=1")
		}
		// The prep is not housekeeping: bwrap binds only a source that
		// exists, so a write grant that is not there yet (the run's TMPDIR,
		// its private /tmp) is a mkdir the launcher
		// makes OUTSIDE the namespace, in the order BwrapArgs emitted it.
		if err := runBwrapPrep(prep); err != nil {
			return landedCage{}, err
		}
		bin, err := bwrapBin()
		if err != nil {
			return landedCage{}, err
		}
		// No network (ranger-base-4nzpo). Added HERE and not in BwrapArgs,
		// because a seat keeps its egress ("egress is the container tier's")
		// and the judge has no persona that needs any. bwrap brings the new
		// namespace up with loopback, so a check that listens on 127.0.0.1
		// still runs.
		argv = append(argv, landedCheckBwrapNet)
		for _, kv := range env {
			if k, v, ok := strings.Cut(kv, "="); ok {
				argv = append(argv, "--setenv", k, v)
			}
		}
		return landedCage{Argv: append(append([]string{bin}, argv...), "--", judge), Env: env}, nil
	case WallSandboxExec:
		if kind == landedCageVenue {
			// ADR 0097 D1/D2: the step does not exist here. A profile has no
			// spelling for a pid namespace or a mount view, so the "venue"
			// this arm could render would be the judge's wall with a
			// different name — and the wall check inside it would abstain on
			// every member, answer exit 3, and reach a reader as D5's
			// silence rather than as "this host does not do this".
			return landedCage{}, Die("there is no %s on this host: ADR 0097 D1's venue is a bwrap and this host realizes cage %s with %s (ADR 0096: no darwin change), so the wall check's step does not exist here", kind, CageSeatbelt, m)
		}
		if !SeatbeltAvailable() {
			return landedCage{}, Die("the landed check cannot run: this host realizes cage %s with %s and %s (ADR 0088 D3), so there is no wall to put it behind",
				CageSeatbelt, m, orElse(sandboxApplyRefusal(), "it does not resolve"))
		}
		// "landed-check" where a seat's profile names its persona: the
		// header line is the only thing that reads it, and an operator who
		// opens this file needs it to say what the wall is for rather than
		// to name a persona that has nothing to do with it.
		prof := filepath.Join(filepath.Dir(judge), "landed-check.sb")
		// The network rule goes AFTER the whole profile: last match wins in
		// SBPL, so nothing above can re-open what it closes. Seatbelt has no
		// spelling for Hide or Tmp — it mediates operations, not a mount
		// view, and a write to /tmp is already outside the grant.
		if err := os.WriteFile(prof, []byte(SeatbeltProfile("landed-check", writable, nil, carve)+landedCheckSeatbeltNet(run)), 0o644); err != nil {
			return landedCage{}, err
		}
		return landedCage{Argv: []string{"sandbox-exec", "-f", prof, judge}, Env: env}, nil
	}
	return landedCage{}, Die("the landed check cannot run: posse has no wall render for this host (%s), and ADR 0088 D3 never runs it uncaged", runtime.GOOS)
}

// landedCheckBwrapNet is the judge's network on bwrap: a namespace of its own,
// which bwrap brings up with loopback and nothing else (ranger-base-4nzpo).
const landedCheckBwrapNet = "--unshare-net"

// landedCheckSeatbeltNet is the same rule on seatbelt: every network
// operation denied, then loopback IP and unix sockets inside the run dir
// allowed back. The unix-socket half is not decoration — connecting to a
// unix socket is a network operation in SBPL and not a file write, so
// without it the host's agent sockets stay reachable under the write deny.
// SBPL's ip filter takes only `*` or `localhost` as the host, so
// "loopback only" is exact.
//
// ASSUMED, NOT MEASURED: there is no darwin host behind this render
// (ranger-base-bfla6 carries the probe).
func landedCheckSeatbeltNet(run string) string {
	r := sbQuote(absResolve(run))
	return ";; the landed check's network (ranger-base-4nzpo): loopback and the run's own\n;; unix sockets, nothing else. Last, so it outvotes everything above.\n" +
		"(deny network*)\n" +
		"(allow network* (local ip \"localhost:*\"))\n" +
		"(allow network* (remote ip \"localhost:*\"))\n" +
		"(allow network* (local unix-socket (subpath " + r + ")))\n" +
		"(allow network* (remote unix-socket (subpath " + r + ")))\n"
}

// landedCheckTmp is the directory bwrap binds over the judge's /tmp: `tmp`
// in the run dir, the same directory judge() hands the check as TMPDIR. ""
// — the host's /tmp — when the home, the run or the tree is itself under
// /tmp, for bwrapSeatTmp's reason: a private /tmp over them leaves the judge
// nothing to chdir into. No production run is that shape (the run dir is
// under the state dir); this package's fixtures are. An empty run or tree is
// a stand-down, but an empty home is not (ranger-base-f1rpw): no home is no
// home under /tmp, so nothing of the judge's is hidden by a private one. The
// `home != ""` is not the old emptiness test: underDir resolves "" to the
// cwd, so without it an empty home would stand down whenever the launcher
// happens to sit under /tmp.
func landedCheckTmp(home, run, tree string) string {
	if home != "" && underDir(bwrapTmpMount, home) {
		return ""
	}
	for _, p := range []string{run, tree} {
		if p == "" || underDir(bwrapTmpMount, p) {
			return ""
		}
	}
	return landedCheckTmpDir(run)
}

// landedCheckGoCache is the judge's build cache: fresh in every run and
// removed with it (ranger-base-4nzpo). Fresh rather than a persistent
// judge-only cache, because a persistent one would let landing N plant
// objects that decide landing N+1's verdict.
func landedCheckGoCache(run string) string { return filepath.Join(run, "gocache") }

// ─── THE JUDGE'S SOCKET WALL (ranger-base-8ukiy) ────────────────────────────

// landedCheckRunDir is the SYSTEM run dir, hidden whole.
//
// WHY A DIRECTORY AND NOT A LIST OF SOCKETS. `--unshare-net` takes away the
// network namespace and takes away nothing else: a PATHNAME unix socket is
// reached through the mount namespace, so every system daemon's endpoint
// under /run answers a caged process exactly as it answers the operator —
// same uid, same file. MEASURED 2026-11-14 on the D1 host (Linux 7.2.15),
// `find /run -xdev -type s -perm -o+w` outside /run/user: 31 world-writable
// sockets, among them `/run/tailscale/tailscaled.sock` (srw-rw-rw-), the
// dbus system bus, polkit's agent-helper, cups, avahi, ssh-unix-local and
// about 25 systemd varlink/journal endpoints. tailscaled's LocalAPI carries
// a dial endpoint (`tailscale nc`), so a judge allowed to write to it could
// make TCP connections around `--unshare-net` and the judge's "no network"
// would stop being true. Whether this uid was in fact allowed to is a read
// that belongs to the operator and is never probed from a seat; the hide is
// what makes the answer stop mattering here.
//
// A CENSUS OF LIVE SOCKETS WOULD NOT HOLD. A socket bound after the render,
// or unlinked and re-bound, is not covered by a list taken at render time,
// and the landing path renders once per run. An empty tmpfs over the DIRECTORY
// does hold across a re-bind (ranger-base-6rndn's N4), so the list is
// structural and fixed, and a daemon this box grows next month is hidden
// without anyone naming it — bwrapRuntimeDirs' "an allowlist, not a
// denylist" one directory up.
//
// THE JUDGE NEEDS NOTHING IN IT. Its body is this repo's `make` doors over a
// detached worktree (scripts/posse-landed-check), with TMPDIR and GOCACHE in
// the run dir. /run/user is hidden inside it already; `/var/run` is a
// symlink to it and needs no entry of its own. What a hide COULD break is
// name resolution: `/etc/resolv.conf` is a symlink into
// `/run/systemd/resolve` on this host and nsswitch names `systemd` for
// passwd and `resolve` for hosts. Dangling, those degrade rather than fail —
// `files` answers this uid out of /etc/passwd and there is no network to
// resolve for — and MEASURED 2026-11-14 a full judge run of this tree behind
// the hide is green (the reading is on the bead and in
// docs/notes.d/ranger-base-8ukiy.md).
//
// AND IT IS THE VENUE'S ONLY LAYER. ADR 0097 D1 renders the wall venue as
// this cage plus three additions, so the venue inherits this; it cannot take
// ranger-base-6rndn's Landlock step instead, because a Landlock FS domain
// forbids mount and the venue exists to nest.
const landedCheckRunDir = "/run"

// landedJudgeHides is the judge's whole Hide list: the user runtime dir
// (ranger-base-ludpp), the system run dir above, and every registered
// runtime's own state root.
//
// One function because ADR 0097 D1 renders the venue through this same
// renderer: a second producer is a layer the venue would not inherit, which
// is ranger-base-hy1aj's lesson.
func landedJudgeHides() ([]string, error) {
	hide, err := liveBwrapRuntimeDirs()
	if err != nil {
		return nil, err
	}
	hide = append(hide, absResolve(landedCheckRunDir))
	// The error is discarded for landedCheckTmp's reason one call up: the
	// home only decides whether a hide would swallow it, and an unknown home
	// cannot be swallowed.
	home, _ := os.UserHomeDir()
	for _, d := range landedJudgeRuntimeRoots() {
		if err := landedJudgeHideRefusal(d, home); err != nil {
			return nil, err
		}
		exists, dir, err := bwrapStat(d)
		if err != nil {
			return nil, bwrapStatRefusal("runtime state root", d, err)
		}
		// A state entry that names a FILE is skipped and not refused. Every
		// claude install on this box has one — `~/.claude.json` is declared
		// beside `~/.claude` — and a tmpfs can only be mounted over a
		// directory, so refusing would be a judge that never runs on the
		// box it was written for. Nothing is lost: a file is not a directory
		// a CLI can bind a socket in, which is the whole of what this layer
		// is about, and the file's own secrets are already masked by
		// credentialReadDenyLiterals above.
		if exists && !dir {
			continue
		}
		hide = append(hide, d)
	}
	return dedupeStrings(hide), nil
}

// landedJudgeRuntimeRoots is item 2 of the fix: each registered runtime's own
// state root, TAKEN FROM THE REGISTRY and never from a scan.
//
// WHY THE JUDGE GETS THEM AT ALL. A runtime's state root is where its CLI
// binds its control socket — grok's leader socket is `$GROK_HOME/leader.sock`
// (runtime.go) — and that is an endpoint under the read-only home which a
// caged process of the same uid can still connect to, because connecting is
// not a write. The judge belongs to no
// runtime and needs none of them; a seat gets its OWN runtime's state as a
// write grant and is spared this list by being a seat, which this child is
// not.
//
// BOTH SPELLINGS PER RUNTIME, which is credentialReadDenyLiterals' rule in
// this file's terms: the DECLARED built-in literal (`~/.grok`) and the
// directory the CLI's own home variable moves it to when that variable is in
// force. Whatever the CLI wrote before the variable moved it is still sitting
// in the home, and a hide over an absent path costs nothing — the hide loop
// skips what is not there.
//
// THE BUILT-IN LITERAL AND NOT THE OVERLAY, deliberately. `state_dir:` in a
// runtimes/*.yaml REPLACES the built-in list (ADR 0021 D1), but it is posse's
// DECLARATION of where a CLI keeps state and it does not move where the CLI
// actually binds. So this reads builtinRuntimes directly: no App, no yaml
// read at render time, and the same answer on every box.
//
// RESIDUAL, stated: a runtime DECLARED only in a runtimes/*.yaml contributes
// nothing here. Its state root is named by that yaml and nowhere in this
// binary, and reading it would make the judge's wall depend on a file the
// landing path parses at render time. Residuals of this layer are
// ranger-base-1xyso's.
func landedJudgeRuntimeRoots() []string {
	var out []string
	add := func(p string) {
		p = ExpandTilde(p)
		// credentialDenyable's predicate, for its reason: the declared
		// literals are `~/…` and ExpandTilde hands one back unchanged when
		// there is no HOME, which names no directory. absResolve would hang
		// it off the launcher's cwd — a tmpfs over an ordinary directory
		// beside the launcher, and no wall at all over the runtime's state.
		if !filepath.IsAbs(p) {
			return
		}
		out = append(out, absResolve(p))
	}
	for i := range builtinRuntimes {
		// A VALUE COPY and not &builtinRuntimes[i]: cmd/testparallel reads a
		// taken address as a WRITE, and marking this table written taints
		// every parallel test that names it — 27 of them, MEASURED by
		// `make corpus-check` on the pointer form (ranger-base-8ukiy).
		rt := builtinRuntimes[i]
		for _, d := range rt.StateDirs {
			add(d)
		}
		if rt.HomeVar != "" {
			if v, inForce := runtimeHomeOverride(rt.HomeVar); inForce {
				add(v)
			}
		}
	}
	return dedupeStrings(out)
}

// landedJudgeHideRefusal is bwrapRuntimeDirs' guard applied to a runtime's
// state root, and it REFUSES for that function's reason rather than skipping:
// an empty tmpfs over `/`, over a top-level directory, or over a directory
// holding the home is not a narrower wall, it is a judge with no tree — and
// a landing that reads ⚠ naming the variable beats one whose judge dies on
// its own plumbing and reads red. Reachable only from an operator's own
// `CLAUDE_CONFIG_DIR=$HOME`-shaped value; the declared literals cannot be
// either shape.
func landedJudgeHideRefusal(p, home string) error {
	if strings.Count(filepath.Clean(p), string(filepath.Separator)) < 2 {
		return Die("the landed check will not hide the runtime state root %s: an empty tmpfs over / or a top-level directory leaves the judge with no tree. Point the runtime's home variable at a directory of its own (ranger-base-8ukiy)", p)
	}
	if home != "" && underDir(p, absResolve(home)) {
		return Die("the landed check will not hide the runtime state root %s: it holds the home %s, and an empty tmpfs over it leaves the judge with no home, no module cache and no tree. Point the runtime's home variable at a directory of its own (ranger-base-8ukiy)", p, home)
	}
	return nil
}

// landedJudgeEnvKeys is THE allowlist: the only names of the launcher's
// environment the judge is handed (ranger-base-5m5yq). Who the process is
// and how it prints, then where the go toolchain lives and which one it
// selects. Everything else the launcher carries stays out — the judge's
// stderr tail is filed into a bead other seats read, so a token that reached
// its environment would have a way out. A check that needs one more variable
// reds once, and the fix is to name it HERE and nowhere else.
var landedJudgeEnvKeys = []string{
	"PATH", "HOME", "USER", "LOGNAME", "LANG", "LC_ALL", "TZ", "TERM",
	"GOROOT", "GOPATH", "GOMODCACHE", "GOTOOLCHAIN", "GOFLAGS",
}

// landedJudgeEnv is the judge's whole environment: the allowlisted names
// that are set in this process, then `over` (the run's TMPDIR and GOCACHE)
// on top, then protocolEnv for PATH. A name that is unset is left unset
// rather than spelled empty — `HOME=` is a different instruction from no
// HOME at all.
//
// It is built FROM NOTHING and not filtered out of os.Environ(), so a name
// added to the launcher tomorrow is out by construction. The slice starts
// non-nil because protocolEnv reads a nil list as "the whole of
// os.Environ()", which is the one environment this function exists to not
// hand over.
//
// Built here rather than with bwrap's --clearenv because cmd.Env is the one
// lever both mechanisms honour; sandbox-exec has no such flag.
func landedJudgeEnv(over []string) []string {
	env := make([]string, 0, len(landedJudgeEnvKeys)+len(over)+1)
	for _, k := range landedJudgeEnvKeys {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	for _, kv := range over {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env = envPut(env, k, v)
		}
	}
	return protocolEnv(env)
}

// landedCheckWritable is D3's write set, and the whole of it: the run's own
// directory — which holds the detached worktree, the extracted judge, the
// TMPDIR its children write into and the run's own build cache — and the
// worktree's private git directory.
//
// THE WORKTREE'S GIT DIR IS PART OF THE WORKTREE, not part of the repo. A
// linked worktree keeps its own HEAD, index and logs under
// `<repo>/.git/worktrees/<name>`, and a `git status` inside the tree
// refreshes that index. The check reads git history by construction
// (`history-check`, the silent-revert audit — which is WHY D3 says a
// worktree and not a `git archive` export), so a grant that stopped at the
// working tree would leave a check failing on its own plumbing rather than
// on the tree. The bind is that one directory and not the repo's `.git`:
// what stays out is the COMMON dir, which is where `packed-refs.lock` lives
// and where a stranded lock is the operator's to remove and never a seat's
// (AGENTS.md, ADR 0059 D3).
//
// THE BOX'S SHARED BUILD CACHE IS NOT IN IT (ranger-base-4nzpo). It used to
// be, to save a cold build. But it is the cache every uncaged build of this
// box reads, posse's own included, so a grant on it was landed code writing
// into the launcher's next build. The run builds through
// landedCheckGoCache instead, inside the run dir granted above.
func landedCheckWritable(run, tree string) []string {
	w := []string{run}
	// ASKED OF GIT AND NOT SPELLED. A linked worktree's admin directory is
	// `<repo>/.git/worktrees/<name>`, which is neither inside the worktree
	// nor somewhere this package may compute: the repo's own `.git` can be a
	// file, a `commondir` can redirect it, and the name git chose is git's.
	// Spelling it by hand got a directory that was not there, which BwrapArgs
	// then created and bound — a grant for a path nothing reads, and the real
	// one still read-only, both silently (MEASURED 2026-11-11, before this
	// line; the check passed anyway, which is exactly why it was silent).
	if g, err := git(tree, "rev-parse", "--absolute-git-dir"); err == nil && g != "" {
		w = append(w, g)
	}
	return dedupeStrings(w)
}

// goBuildCache is the box's shared build cache, resolved WITHOUT forking
// `go env`. The documented resolution is `$GOCACHE`, else
// `$XDG_CACHE_HOME/go-build`, else `$HOME/.cache/go-build` (and
// `~/Library/Caches/go-build` on darwin), and a fork here would be a second
// child on a path whose whole subject is that exactly one child runs and
// that it runs caged.
//
// It is no longer granted (landedCheckWritable says why). It stays named so
// the cage's pin can say what must NOT be writable, in the same resolution
// the grant used to use.
func goBuildCache() string {
	if c := os.Getenv("GOCACHE"); c != "" {
		return c
	}
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "go-build")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "go-build")
	}
	return filepath.Join(home, ".cache", "go-build")
}

// ─── D4: ONE AT A TIME, COALESCED ───────────────────────────────────────────

// landedTreeName is the detached worktree's directory inside a run dir, and
// `<it>.git` is the private git dir beside it. Named rather than spelled
// twice because the write grant and the `worktree add` have to agree.
const landedTreeName = "tree"

// landedChecker is D2 and D4: the landings say what moved, this runs the
// check, and the two are not the same goroutine.
//
// WHY A SCHEDULER AT ALL, rather than a call at the end of the landing. D2
// says the check runs outside the lock and never blocks, and D4 says at most
// one is in flight and that runs coalesce. A landing sweep lands five trees
// into one repo under one lock; five checks would be five minutes of box for
// one question, and the first four would each judge a tip that was stale
// before the run finished. So a landing SAYS which repo moved and this reads
// the newest tip when it gets there — which is the same answer, once.
//
// IT IS JOINED, and that is not optional on this box. A caged `make` that
// outlives the process that started it is an orphan with no POSSE_KEEP
// marker, which the fleet load guard's kill arm ends on any tick
// (AGENTS.md, "Something you MEANT to leave running"). So the pass that
// schedules is the pass that waits: `Wait` at the end of a dispatch pass,
// and at the end of `posse worktrees --land`.
type landedChecker struct {
	a    *App
	out  io.Writer
	cage landedCageFn
	// bd is D5's filer (landedfile.go) and D6.3's (landedhealth.go). A field
	// and not a call to NewBd at the filing site, so a test drives the fake
	// bd every other filing path in this package is pinned against, and so
	// one checker spends one resolution.
	bd Bd
	// ceiling overrides D6.2's price for a test, and zero means
	// landedCheckCeiling (ceilingOf, landedhealth.go). A seam for the same
	// reason `cage` is one: a 240s wall cannot be reached inside a test, so
	// without it the arm where an answered run is OVER the price — the arm
	// D6.3 exists to file — would have no spelling here either.
	ceiling time.Duration

	mu sync.Mutex
	// pending is the coalescing set: repo path → the branch that moved.
	// A map, so N landings into one repo are one entry however many passes
	// made them, and the run reads the TIP for itself rather than taking a
	// sha captured at schedule time — which is what makes "judge the newest
	// tip only" true rather than hopeful.
	pending map[string]string
	running bool
	// done is closed when the run loop drains and stops. Remade on each
	// start, so a Wait that arrives between two cycles blocks on the live
	// one and never on a channel that is already closed.
	done chan struct{}
	// saidOwnCopy is D1's "said once": the first run with no green tree on
	// record judges with the landed tree's own copy, and says so. Per
	// checker and not per run, because the record stays empty until the
	// first green and the sentence is about the configuration, not the tip —
	// and per RULE AND REPO rather than a bool, because one launcher runs
	// two checks with a record each (ADR 0097 D5, landedRule.memKey).
	saidOwnCopy map[string]bool
	// quietSaid is D6.3's other input, per repo: the sentence the LAST
	// unanswered run gave, so the bead filed on the second consecutive
	// silence can carry both (rememberQuietSaid, landedhealth.go). In
	// memory and not in the record, which D6.3 spells `<tree> <n>`.
	// Keyed by landedRule.memKey and not by the repo alone, like
	// quietUnreset and saidOwnCopy: one launcher judges one repo under two
	// rules, and a map keyed on the repo would have the wall check's last
	// sentence filed as the judge's (ADR 0097 D5).
	quietSaid map[string]string
	// quietUnreset is set for a repo whose counter came through an answered
	// run that could NOT reset it (ranger-base-4jroi): the count then spans
	// a run that answered, so it is no longer a count of CONSECUTIVE
	// silences and the bead filed on the next one may not claim one
	// (silentDescription, landedhealth.go). In memory beside quietSaid and
	// for the same reason — the record is spelled `<tree> <n>` and a third
	// field reads as no record at all
	// (TestTheQuietCounterReadsZeroOverAnyShapeThatIsNotItsOwn) — so a
	// launcher that restarted carries neither, which is exactly the case the
	// other wording of that branch is for.
	quietUnreset map[string]bool
}

// newLandedChecker builds one. `out` must be a writer the caller is willing
// to have written to from another goroutine — a Dispatcher hands its
// serialized `outWriter()`, and the one command path that hands a bare
// writer schedules after its own last line (LandSessionTrees).
func newLandedChecker(a *App, out io.Writer) *landedChecker {
	return &landedChecker{a: a, out: out, cage: boxLandedCage, bd: NewBd(), pending: map[string]string{}}
}

// Schedule says a landing moved `base` in `repo`. It starts the run loop if
// nothing is running and returns immediately — which is D2's "the landing
// has already happened", and the reason the landing path can call it under
// the lock it is about to drop.
func (c *landedChecker) Schedule(repo, base string) {
	if c == nil || repo == "" || base == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending[repo] = base
	if c.running {
		// D4: a run is in flight. It re-drains this map when it finishes and
		// reads the tip then, so this landing is judged by that run's
		// successor and never by a second concurrent one.
		return
	}
	c.running, c.done = true, make(chan struct{})
	go c.loop()
}

// Wait blocks until nothing is in flight. The loop below may start a new
// cycle while this one waits — a landing can arrive from another goroutine —
// so it re-asks rather than waiting on one channel.
func (c *landedChecker) Wait() {
	if c == nil {
		return
	}
	for {
		c.mu.Lock()
		if !c.running {
			c.mu.Unlock()
			return
		}
		done := c.done
		c.mu.Unlock()
		<-done
	}
}

// loop is the one goroutine. It takes one repo at a time out of the pending
// set and judges it, and stops only when the set is empty — so a landing
// that arrives while a check runs is picked up by the same goroutine rather
// than by a second one.
func (c *landedChecker) loop() {
	for {
		c.mu.Lock()
		var repo, base string
		for r, b := range c.pending {
			repo, base = r, b
			break
		}
		if repo == "" {
			c.running = false
			close(c.done)
			c.mu.Unlock()
			return
		}
		delete(c.pending, repo)
		c.mu.Unlock()
		c.once(repo, base)
	}
}

// once is one landed check: read the newest tip, ask whether that tree is
// already on record, and if it is not, judge it caged and record what it
// said.
//
// EVERY REFUSAL IS A ⚠ AND A RECORD THAT IS NOT WRITTEN (D3's bottom row and
// D5's "no answer | any"). Nothing here returns red for a question it could
// not ask, and nothing here records a verdict it did not get — a cache entry
// written over silence would be read by the next run as a judgement.
func (c *landedChecker) once(repo, base string) {
	tip := refSHA(repo, base)
	if tip == "" {
		return // the branch is gone; the next landing into this repo re-asks
	}
	tree := revTreeID(repo, tip)
	if tree == "" {
		c.warn("%s could not be judged: git would not read %s's own tree", AbbrevHome(repo), abbrevSHA(tip))
		return
	}
	// D4's cache, and the common case of a coalesced run: the record already
	// names this tree, so the question has been asked and the answer has not
	// changed — the check reads BYTES and the bytes are the same bytes.
	//
	// Asked BEFORE the opt-in, which is the cheaper order for the case that
	// happens most: a repo that opted in and has not moved costs one git
	// config read, and a repo that never opted in pays the same two reads
	// whichever way round they go.
	//
	// `had`/`prev` are also D5's other input: the record as it reads BEFORE
	// this run is the transition's left-hand column, and — when it is green
	// — the far edge of the window a filed bead names (landedfile.go).
	had, prev := landedRecorded(landedJudgeRule, repo)
	if had == tree && prev != landUnanswered {
		return
	}
	if !landedCheckShipped(landedJudgeRule, repo, tree) {
		return // the repo has not opted in, which is every repo until it does
	}
	run, err := os.MkdirTemp(landedCheckWorkDirEnsured(c.a), "run-")
	if err != nil {
		c.warn("%s could not be judged: the landed check has nowhere to work (%v)", AbbrevHome(repo), err)
		return
	}
	// The worktree and everything beside it are the launcher's to remove.
	// `git worktree remove` first, so the repo's administrative record goes
	// with the directory rather than being left pointing at a path that is
	// gone — and best effort, because a removal that fails is a directory to
	// sweep and never a reason to lose the verdict.
	defer func() {
		_, _ = git(repo, "worktree", "remove", "--force", filepath.Join(run, landedTreeName))
		_ = os.RemoveAll(run)
	}()
	// D6.2: EVERY verdict line prints its wall, including the one that says
	// there is no verdict. Timed around c.judge and not around the whole of
	// once(), because what the ceiling prices is the judge — the worktree,
	// the cage and the check — and not the two git config reads above it or
	// the worktree removal the defer does below.
	//
	// `took` is kept rather than formatted on the spot because D6.3 reads it
	// (landedhealth.go): an answered run over landedCheckCeiling, and a
	// second consecutive unanswered one, each file one architecture bead,
	// and both of those decisions are made with the Duration in hand.
	start := time.Now()
	v, said := c.judge(repo, run, tip, tree)
	took := time.Since(start)
	switch v {
	case landUnanswered:
		c.warn("%s landed at %s and was NOT judged after %s — %s — nothing recorded (ADR 0088 D3: a check that cannot answer never answers red)", AbbrevHome(repo), abbrevSHA(tip), landedCheckWall(took), said)
		// D6.3, and this is the arm it exists for: silence records nothing
		// and files nothing under D5, so the watchdog on the detector is
		// the only thing that can say the judge has stopped answering.
		c.health(landedJudgeRule, repo, tree, v, took, said)
		// ADR 0097 D2: the wall check runs after a judgement that answered
		// GREEN, so this one says it did not run and why. It records
		// nothing and counts nothing — "not run (judge no answer)" is
		// neither a green nor a silence of its own, which is what keeps one
		// broken tree to one filed bead.
		c.wall(repo, base, tip, tree, run, v)
		return
	case landGreen:
		c.say("✓ %s green at %s in %s (%s, ADR 0088 D1)", AbbrevHome(repo), abbrevSHA(tip), landedCheckWall(took), landedCheckScript)
	default:
		// The verdict and the cause, which is the sentence ranger-base-ddgfr
		// turns into a bead. It is printed here whole rather than summarized,
		// because until that bead lands this line IS the record a human reads.
		c.warn("%s is RED at %s in %s — %s — %s (ADR 0088 D1)", AbbrevHome(repo), abbrevSHA(tip), landedCheckWall(took), landedCheckScript, said)
	}
	recorded := true
	if err := recordLandedCheck(landedJudgeRule, repo, tree, v); err != nil {
		// Best effort by contract, for recordLandCheck's reason: a config
		// write that fails leaves the next landing judging this tree again,
		// which is the behaviour before any record existed.
		//
		// It is also D5's whole dedupe for the green→red row, so the filer
		// is TOLD (ranger-base-w9w37): over a record still naming the stale
		// green tree, every later red reads green→red again, and a filer
		// that did not know would file one P1 per landing.
		recorded = false
		c.warn("%s judged %s %s and the verdict could not be recorded (%v)", AbbrevHome(repo), abbrevSHA(tip), v, err)
	}
	// D5, and AFTER the record for the reason landedfile.go's `file` gives:
	// of the two orders, this is the one whose failure costs one bead one
	// landing late instead of two beads for one red stretch.
	c.file(landedJudgeRule, repo, base, tip, tree, had, prev, v, said, recorded)
	// D6.3, after D5 and not instead of it: the two are about different
	// subjects — the tree, and the judge — and an answered run can be both
	// a red main and a crossing of the price. Its own counter resets here,
	// and the ceiling is read off this wall.
	c.health(landedJudgeRule, repo, tree, v, took, said)
	// ADR 0097 D2, LAST: the wall check is a second check after the judge
	// and not part of its body, so the judge's own record, bead and health
	// are written before a venue run that costs about 100s. It runs on this
	// same tree, in this same run dir, inside the defer above — so the
	// worktree it judges is the one the judge judged, byte for byte, and the
	// removal still happens once.
	c.wall(repo, base, tip, tree, run, v)
}

// judge creates the detached worktree, extracts the judge, and runs it
// caged. The three returns of landChildVerdict are its returns: green, red,
// or a sentence saying why there is no answer.
//
// THE LAUNCHER RUNS ONLY GIT HERE (D3). `worktree add` and `cat-file` are
// the whole of what this process executes outside the cage; the judge's own
// first instruction runs behind the wall.
func (c *landedChecker) judge(repo, run, tip, tree string) (landVerdict, string) {
	path := filepath.Join(run, landedTreeName)
	// `--no-checkout` is deliberately NOT passed: the check's subject is the
	// tree's bytes. Detached, so this creates no branch and the repo's own
	// refs are untouched.
	if _, err := git(repo, "worktree", "add", "--detach", path, tip); err != nil {
		return landUnanswered, fmt.Sprintf("a detached worktree of %s could not be created (%s)", abbrevSHA(tip), gitSaid(err))
	}
	judge, from, err := c.extract(landedJudgeRule, repo, run, tree)
	if err != nil {
		return landUnanswered, err.Error()
	}
	tmp := landedCheckTmpDir(run)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return landUnanswered, fmt.Sprintf("the landed check's own temp dir could not be made (%v)", err)
	}
	// TMPDIR inside the run dir rather than the host's /tmp, because the run
	// dir is the grant: the check's children stage and link there, and under
	// D3 nothing outside this directory is writable. GOCACHE beside it, for
	// the same reason and for landedCheckWritable's: a cache of this run's
	// own, gone when the run is.
	gocache := landedCheckGoCache(run)
	if err := os.MkdirAll(gocache, 0o755); err != nil {
		return landUnanswered, fmt.Sprintf("the landed check's own build cache could not be made (%v)", err)
	}
	env := landedRunEnv(run)
	cage, err := c.cageFn()(c.a, landedCageJudge, judge, path, landedCheckWritable(run, path), env)
	if err != nil {
		// D3's one hard rule, and the reason the argv comes out of a
		// renderer instead of being built here: a cage that will not render
		// is the end of the run. There is no uncaged arm to fall through to.
		return landUnanswered, oneLine(err.Error())
	}
	c.sayOwnCopy(landedJudgeRule, repo, from, tree)
	ctx, cancel := context.WithTimeout(context.Background(), landedCheckLimit)
	defer cancel()
	// protocolCmd around the construction and protocolEnv around the
	// environment, which are two halves of ONE rule (ADR 0077 D3): a posse
	// process that dispatches can have another persona's gates bin leading
	// its PATH, so the cage binary is resolved outside every gates dir — and
	// since cmd.Env REPLACES the environment, handing it a list that did not
	// go through protocolEnv would throw that resolution away on the next
	// line while the call site still read as though it had one.
	//
	// THE CHILD'S ENVIRONMENT IS AN ALLOWLIST, not the launcher's
	// (ranger-base-5m5yq): landedJudgeEnvKeys, with TMPDIR and GOCACHE
	// redirected on top. The repo still owns what green means over the
	// environment it is handed — this one scrubs make's own variables for
	// itself (docs/notes.d/ranger-base-r3pqf.md) — but the wall owns what
	// crosses it, and a launcher variable is something crossing.
	cmd := protocolCmd(exec.CommandContext(ctx, cage.Argv[0], cage.Argv[1:]...))
	cmd.Dir = path
	cmd.Env = landedJudgeEnv(cage.Env)
	// landChildVerdict is the binder: it sets the reaping grace beside this
	// deadline, because a deadline alone bounds the CHILD and not this call
	// — and this child has grandchildren by construction (`make`, and `go`
	// under it).
	return landChildVerdict(ctx, cmd, landedCheckLimit)
}

// sayOwnCopy is ADR 0088 D1's "said once", per RULE: the first run with no
// green tree on that rule's record judges with the landed tree's own copy,
// and says so.
//
// SAID AT THE CALL SITE AND NOT WHERE THE COPY IS EXTRACTED: a run whose cage
// refused never judged anything, and burning the once on it would mean the
// first run that really ran said nothing about judging itself. The fallback
// is the configuration speaking, not this tip — there is no green tree on
// record to take the copy from, so the tree being judged is judging itself,
// and a landing that weakened the script would have weakened its own gate. It
// stays true until the first green lands, which is why once per process is
// the right number.
//
// PER RULE AND NOT PER CHECKER (ADR 0097 D5): one launcher runs both checks,
// and a single bool would let the judge's sentence spend the wall check's.
func (c *landedChecker) sayOwnCopy(r landedRule, repo, from, tree string) {
	if from != tree || c.saidOwnCopy[r.memKey(repo)] {
		return
	}
	if c.saidOwnCopy == nil {
		c.saidOwnCopy = map[string]bool{}
	}
	c.saidOwnCopy[r.memKey(repo)] = true
	c.say("· %s is judged by its OWN %s — no tree is recorded green in %s yet (%s)", AbbrevHome(repo), r.Script, r.Key, r.RunADR)
}

// cageFn is the renderer this checker uses, defaulting to the box's. A
// method rather than a nil-check at the call site so a checker built as a
// zero value still runs the real thing.
func (c *landedChecker) cageFn() landedCageFn {
	if c.cage == nil {
		return boxLandedCage
	}
	return c.cage
}

// extractJudge is D1's "the copy that runs comes from the last tree recorded
// green, not from the tree being judged". It reads the blob out of git and
// writes it where the cage can exec it — beside the worktree and not INSIDE
// it, because a judge written into the tree it judges would be a file the
// check can see in its own `git status` and a byte the next run's tree id
// does not account for.
//
// THE RECORD IS ONE ENTRY, SO "LAST GREEN" IS THE RECORD WHEN IT IS GREEN.
// D4 spells the record `<tree-id> <green|red>`, which holds the last tree
// JUDGED and not a history, so during a red stretch there is no green tree
// on record to take the judge from. That is read as D1's documented absent
// case — the landed tree's own copy, said once — rather than as a reason to
// coin a second config key: the alternative is a judge that gets older every
// landing until somebody fixes main, and the red stretch is exactly when the
// script is most likely to be what is being fixed.
//
// A recorded tree that no longer has the script (it was added after that
// tree, or the record predates the opt-in) falls back the same way, because
// "the judge is not there" is not a statement about the tree being judged.
func (c *landedChecker) extract(r landedRule, repo, run, tree string) (path, from string, err error) {
	from = tree
	if had, v := landedRecorded(r, repo); v == landGreen && had != "" {
		from = had
	}
	// gitRaw and not git: a judge is BYTES, and git() trims its output. A
	// trimmed shell script usually still runs, which is exactly what makes
	// that the wrong tool here — the one thing this gate may not do is judge
	// with a copy that is not the copy on record.
	body, gerr := gitRaw(repo, "cat-file", "blob", from+":"+r.Script)
	if gerr != nil && from != tree {
		from = tree
		body, gerr = gitRaw(repo, "cat-file", "blob", from+":"+r.Script)
	}
	if gerr != nil {
		return "", "", fmt.Errorf("%s could not be read out of %s (%s)", r.Script, abbrevSHA(from), gitSaid(gerr))
	}
	path = filepath.Join(run, r.Extract)
	// 0o755: the blob's own mode is not asked. landCheckShipped's reasoning
	// one file over applies here with the sides swapped — there the execute
	// bit is left to exec so a missing one is loud; here the bytes come out
	// of a git blob into a file posse creates, so posse is the one deciding
	// the mode and a mode that cannot exec would be posse refusing its own
	// write.
	if werr := WriteExecutable(path, body, 0o755); werr != nil {
		return "", "", fmt.Errorf("%s could not be written for the cage to exec (%v)", r.Script, werr)
	}
	return path, from, nil
}

// landedCheckShipped reports whether a TREE opts in, which is D1's "a repo
// opts in by shipping an executable at scripts/posse-landed-check" asked of
// the thing being judged.
//
// ASKED OF THE TREE AND NOT OF THE CHECKOUT, which is the one place this
// differs from landCheckShipped. The land check runs the BASE CHECKOUT's
// copy, so a stat of the working tree is the same question; this one runs
// out of git and judges a worktree the launcher is about to create, and the
// operator's checkout can be mid-rebase, dirty, or on another branch
// entirely. `cat-file -e` is the question git answers about a tree.
// It is also ADR 0097 D2's opt-in for the wall check, asked of the same tree
// with that rule's own script: a repo ships one, both or neither.
func landedCheckShipped(r landedRule, repo, tree string) bool {
	_, err := git(repo, "cat-file", "-e", tree+":"+r.Script)
	return err == nil
}

// recordLandedCheck writes the verdict for one tree. Best effort by
// contract, and silence is never written: landUnanswered has no spelling in
// the record because the next run must read "nothing on record" and ask
// again, not "unanswered" and wonder.
func recordLandedCheck(r landedRule, repo, tree string, v landVerdict) error {
	if tree == "" || v == landUnanswered {
		return nil
	}
	_, err := git(repo, "config", r.Key, tree+" "+v.String())
	return err
}

// landedRecorded reads it back. A record in any other shape is no
// record, which is landCheckRecorded's rule and for its reason: this is one
// line of git config a human can edit, and a half-parsed verdict is the one
// thing that must not be read as green.
// It takes the RULE and not the key, so that a reader of a call site sees
// which check's record is being read — the two are one line of git config
// apiece and a bare key at a call site is the kind of argument that gets
// copied into the wrong one (ADR 0097 D5).
func landedRecorded(r landedRule, repo string) (string, landVerdict) {
	out, err := git(repo, "config", "--get", r.Key)
	if err != nil {
		return "", landUnanswered
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return "", landUnanswered
	}
	switch f[1] {
	case landGreen.String():
		return f[0], landGreen
	case landRed.String():
		return f[0], landRed
	}
	return "", landUnanswered
}

// landedCheckWorkDirEnsured is the work dir, made if it is not there. A
// failure to make it is not reported here: MkdirTemp inside it fails next
// with a sentence naming the path, and two sentences for one missing
// directory is the same finding twice.
func landedCheckWorkDirEnsured(a *App) string {
	d := landedCheckWorkDir(a)
	_ = os.MkdirAll(d, 0o755)
	return d
}

// say and warn are the only two writers. Both go through the checker's own
// writer, which the constructor's doc says must tolerate a write from this
// goroutine.
func (c *landedChecker) say(format string, a ...any) {
	fmt.Fprintf(c.out, strings.TrimRight(format, "\n")+"\n", a...)
}

func (c *landedChecker) warn(format string, a ...any) {
	c.say("⚠ "+strings.TrimPrefix(format, "⚠ "), a...)
}
