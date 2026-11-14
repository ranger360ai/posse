package posse

// Helpers for tests whose subject is a LAUNCH behind the L2 wall, shared by
// all three arms because the tests that need them are spread over all three
// (ranger-base-temhf).
//
// WHAT THEY ARE FOR. `cage: seatbelt` names a tier and not a binary, and the
// binary is a host fact: `sandbox-exec` on darwin, `bwrap` on Linux
// (WallMechanism, wall.go). A test that reads `state/gates/<p>/seatbelt.sb`
// or greps the launch line for `sandbox-exec` is therefore asserting DARWIN's
// mechanism, and on the Linux arms of ci.yml it reads a file that was never
// written. Seven tests did exactly that when ADR 0080 D18 typed the bwrap
// wall: darwin green, every ubuntu arm red, over main's only gate.
//
// So the rule these helpers keep: a test whose subject is the WALL asks this
// host which mechanism it has and reads that artifact; a test whose subject
// is one mechanism's own vocabulary (an SBPL line, a mount) says so and skips
// elsewhere. Neither spelling is a grep for a binary name.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// wallArtifactName is the file this host's mechanism renders into a persona's
// gates dir — the two names RenderWall's two arms write (wall.go). Derived
// from the mechanism and not from a GOOS test of its own: a third mechanism
// is one arm in RenderWall and one case here, and nothing in between.
func wallArtifactName(mech string) string {
	switch mech {
	case WallSandboxExec:
		return "seatbelt.sb"
	case WallBwrap:
		return bwrapArgsName
	}
	return ""
}

// wallWrapForTest is the wrap this host's mechanism types in front of the
// runtime command for a persona, and the artifact that wrap reads. It is
// built from the same two functions the launcher types — SeatbeltPrefix and
// BwrapPrefix — so a pin comparing a rendered launch line against it is
// comparing the line to the ONE spelling of the prefix and not to a second
// copy of it.
//
// It does NOT render the wall: the artifact need not exist yet, and a helper
// that wrote one would make every caller's "was the wall rendered" assertion
// answer itself. The caller stats the path when that is its subject.
func wallWrapForTest(t *testing.T, gatesDir string) (wrap, artifact string) {
	t.Helper()
	mech := WallMechanism()
	name := wallArtifactName(mech)
	if name == "" {
		t.Fatalf("this host (%s) has no wall mechanism, so there is no wrap to expect — a test asserting one must say so and skip", mech)
	}
	artifact = filepath.Join(gatesDir, name)
	switch mech {
	case WallSandboxExec:
		return SeatbeltPrefix(artifact), artifact
	default:
		bin, err := bwrapBin()
		if err != nil {
			t.Fatalf("this host's wall mechanism is %s and the binary does not resolve, so no launch here renders a wall: %v", mech, err)
		}
		return BwrapPrefix(bin, artifact), artifact
	}
}

// wallLaunchWrapForTest is what a LAUNCH types for that wall: the same as
// wallWrapForTest at sandbox-exec, and at bwrap the `posse wall-watch`
// sidecar in front of the binary (ranger-base-ihnnl) — built from
// WallWatchPrefix, the one spelling the render types.
func wallLaunchWrapForTest(t *testing.T, gatesDir, seat, dir string) (wrap, artifact string) {
	t.Helper()
	wrap, artifact = wallWrapForTest(t, gatesDir)
	if WallMechanism() != WallBwrap {
		return wrap, artifact
	}
	bin, err := bwrapBin()
	if err != nil {
		t.Fatal(err)
	}
	self, err := wallWatchBin()
	if err != nil {
		t.Fatal(err)
	}
	return WallWatchPrefix(self, filepath.Join(gatesDir, bwrapMountsName), seat, dir, bin, artifact), artifact
}

// ─── ADR 0096 D3: a live pin that nests a bwrap abstains loudly ─────────────

// bwrapNestSkipPrefix is the ONE sentence D3 mandates, as a const so every
// pin prints the same words and a reader greps one string. The reason is
// named in it on purpose: a skip that does not say why reads as coverage
// (ADR 0096, Alternatives rejected).
const bwrapNestSkipPrefix = "NOTHING MEASURED: this process is inside a wall that forbids nesting (ADR 0096 D1): "

var (
	bwrapNestOnce sync.Once
	bwrapNestWhy  string
)

// liveBwrapNestRefusal asks, ONCE PER PROCESS, whether a bwrap starts from
// here: "" when one did, and bwrap's own stderr when it did not.
//
// SEPARATE FROM liveBwrapApplyRefusal ON PURPOSE (ADR 0096 D3), and that is
// the whole point of this helper rather than a convenience. That function
// answers AvailableCages' question — "does this HOST provide the seatbelt
// tier" — and "" there means posse may announce the wall. Here "" means "a
// nested namespace was MEASURED from this process". The two share a probe
// today only because one exec happens to answer both; they must never share a
// meaning, because a host that provides the tier perfectly well still refuses
// a nested bwrap to every process already inside a seat. D17's render leaves
// `--proc` out, so inside a seat `/proc` is the host's procfs under the
// read-only base, a nested user namespace cannot write its own `uid_map`, and
// every nested bwrap fails before it reaches a mount (ADR 0096's table, arm
// A). Once the Landlock layer of ranger-base-6rndn lands the kernel refuses
// the mount instead, and the two questions answer differently for good. A pin
// printing the availability sentence would then tell its reader this box has
// no bwrap, which is false.
//
// ITS OWN sync.Once and not that function's, for the re-arming reason
// liveBwrapApplyRefusal states: by the time any pin asks, whatever called
// AvailableCages first in this binary has already cached the host answer, and
// a sync.Once cannot be re-armed (ranger-base-npqem's lesson, same shape).
//
// MEASURED 2026-11-14, D1 host, Linux 7.2.15, bubblewrap 0.13.1: uncaged this
// returns "", and from a process inside D17's base plus `--unshare-user` it
// returns `bwrap: setting up uid map: Read-only file system`.
func liveBwrapNestRefusal() string {
	bwrapNestOnce.Do(func() { bwrapNestWhy = bwrapNestProbe() })
	return bwrapNestWhy
}

// bwrapNestProbe is that ask, unmemoized.
//
// bwrapApplyProbeArgv, so D17's base mount keeps ONE spelling in this
// package: `--ro-bind / /` is the mount every render of D17 begins with, so a
// process that cannot make THAT namespace cannot run any arm of any live pin
// here, whatever else the pin's own argv asks for.
//
// BARE `bwrap` AND NOT protocolCmd's gates-free resolution, which is the one
// place this differs from bwrapApplyProbe rather than twinning it. The
// question there is posse's — "which file would a launch exec" (ADR 0077 D3,
// because that exec IS one of posse's own children). The question here is
// each pin's own: "will the exec I am about to make start". Every live pin in
// this package types `exec.Command("bwrap", …)` on the ambient PATH and gates
// on `exec.LookPath("bwrap")`, so this probe has to resolve the same way or a
// green gate could precede a red exec. A shim standing in for `bwrap` makes
// this probe refuse, and refusing is the right direction: the pin's own exec
// would have met that same shim.
//
// EVERY FAILURE IS AN ABSTENTION, as at bwrapApplyProbe and for the opposite
// reason to sandboxApplyProbe's: a missing /bin/true or a bwrap too old for
// these flags must not be turned into a measurement nobody took. "Could not
// ask" is not "a namespace was made".
func bwrapNestProbe() string {
	out, err := exec.Command("bwrap", bwrapApplyProbeArgv...).CombinedOutput()
	if err == nil {
		return ""
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		return s
	}
	return err.Error()
}

// bwrapNestSkip is THE GATE: every live pin in this package that execs a
// bwrap calls it, after its own GOOS and `LookPath("bwrap")` gates — which is
// what lets this one skip the fork off Linux — and before it renders
// anything.
//
// IT NEVER FAILS. On this fleet every dispatched seat is caged, so inside one
// the answer is no for a reason that is the wall WORKING, and a red that is
// not a regression teaches people to rerun until green (ADR 0096,
// Alternatives rejected). The pins it gates therefore measure only on an
// uncaged run, which is a pin with no scheduled venue — filed as
// ranger-base-m2ii4 and not solved here.
//
// THE SENTENCE IS PINNED BY NOTHING, deliberately, and this is the one place
// that says so. A pin would have to read ADR 0096 D3's own prose to be worth
// more than a second copy of the literal, and a test in THIS package that
// reads the tree joins the class that needs a Makefile door of its own
// (AGENTS.md, "A `-run` filter cannot reach a tree-wide pin"). What stands in
// its place is a measurement on the bead and in
// docs/notes.d/ranger-base-pvkhn.md: the same binary run uncaged and inside
// D17's base, with every skip line quoted.
func bwrapNestSkip(t *testing.T) {
	t.Helper()
	if why := liveBwrapNestRefusal(); why != "" {
		t.Skip(bwrapNestSkipPrefix + why)
	}
}

// sandboxExecWallOnly skips a test whose assertions are sandbox-exec's own
// vocabulary — an SBPL line, a `(subpath …)` grant, an atomic-write sibling
// regex — on a host whose mechanism is something else. The bwrap readings of
// those same claims are their own pins (bwrapstatedirs_qa_test.go,
// bwraplaunchline_qa_test.go), every arm of which runs on darwin too, so the
// skip gives up a duplicate and never the claim.
//
// NOTHING MEASURED and not a pass, in the spelling this package uses for the
// third answer everywhere else (recordReachUnmeasured, reachability.go).
func sandboxExecWallOnly(t *testing.T) {
	t.Helper()
	if m := WallMechanism(); m != WallSandboxExec {
		t.Skip("NOTHING MEASURED: this host's wall mechanism is " + m + ", whose reading of this claim is its own pin (ADR 0080 D16/D18)")
	}
}

// claudeConfigDirForTest satisfies ADR 0080 D19 for a test that LAUNCHES a
// claude seat at `cage: seatbelt`, and answers the directory it named.
//
// At the bwrap mechanism such a launch is REFUSED unless CLAUDE_CONFIG_DIR is
// set and absolute in the launcher (bwrapClaudeStateDirs, bwrap.go), because
// unset the runtime resolves its config to the `$HOME/.claude.json` FILE,
// which a mount cannot grant — the wall would go up, report success and lose
// every config write. TestMain clears the variable for this whole binary, so
// on Linux every claude seat in the suite hit that refusal: three of the
// seven tests that reddened ci.yml were this and nothing else.
//
// PER TEST AND NOT IN TestMain, which is where it looks like it belongs.
// ClaudeConfigDirIn returns the VARIABLE whenever it is in force, whatever
// home it is handed (trust.go) — so a process-wide value would make every
// test that resolves a config dir under its own fixture home resolve to this
// one instead. TestMain's empty value is deliberate for that reason and
// stays.
//
// It is the host procedure D19's own refusal prescribes, applied to the test
// box: one absolute directory, in the launcher's environment, which is where
// the wall is rendered from.
// gitTempDir and not t.TempDir, because its callers launch a seat in a tree
// a `git init` has run in and the cleanup of such a root has to tolerate a
// concurrent writer (commitwall_qa_test.go) — held by
// TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot.
func claudeConfigDirForTest(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(gitTempDir(t), ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ClaudeConfigDirVar, dir)
	return dir
}

// wallMechanismForTest pins the GOOS WallMechanism answers for, so a pin can
// put a real LAUNCH behind the other host's mechanism (wallMechanismHost,
// wall.go). The bwrap arm also needs bwrapBin answered, which bwlStubBin
// does.
//
// Serial callers only, for seatbeltForTest's reason: this writes a
// package-level var that CheckParity and every launch path read.
func wallMechanismForTest(t *testing.T, goos string) {
	t.Helper()
	had := wallMechanismHost
	wallMechanismHost = goos
	t.Cleanup(func() { wallMechanismHost = had })
}

// wallUngrantedDir is a directory NO L2 wall grants — the precondition of
// every test whose subject is a wall REFUSING a write, and the one thing a
// fixture under `t.TempDir()` cannot be.
//
// `SeatbeltWritable` adds TMPDIR and `/tmp` unconditionally, and
// `/private/tmp` on darwin (seatbelt.go), so a fixture inside any of them is
// writable under the wall for a reason that has nothing to do with the PID's
// rule. ADR 0086's own measurement is the cautionary tale: a code seat
// rendered its wall over a scratch repo under `/tmp` and its "move main" row
// measured nothing.
//
// `/var/tmp` AND NOT sbRoot, which says the same thing for the sandbox pins
// and SKIPS wherever TMPDIR resolves under `/tmp` — every Linux box,
// including the only host ADR 0086's defect was reachable on. `/var/tmp` is
// granted by nothing on either host: `/tmp` and `/private/tmp` are different
// directories. The grant list is checked against the answer rather than
// assumed, because a box that symlinked `/var/tmp` into `/tmp` would hand
// every caller a granted root and every refusal would be measuring the other
// fixture.
//
// os.MkdirTemp with a tolerant removal and not t.TempDir(): callers `git
// init` in here, and the cleanup of such a root has to tolerate a concurrent
// writer (commitwall_qa_test.go, held by
// TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot).
func wallUngrantedDir(t *testing.T) string {
	t.Helper()
	// Named `varTmpRoot` and not `base`, which internal/posse binds to a
	// different literal seven times over in systemdunit_test.go: the
	// tree-wide temp-root census (internal/treepins/temproot_qa_test.go)
	// resolves a parent name across the whole package and DROPS a name the
	// package binds twice rather than guessing, so under the old spelling
	// this root was scored out of that census's scope entirely
	// (ranger-base-qbg7c). One token, and it is the difference between the
	// census reading this line and not reading it.
	const varTmpRoot = "/var/tmp"
	if st, err := os.Stat(varTmpRoot); err != nil || !st.IsDir() {
		t.Skip("NOTHING MEASURED: no " + varTmpRoot + " on this host, so there is no fixture root outside every blanket temp grant")
	}
	d, err := os.MkdirTemp(varTmpRoot, "posse-ungranted-")
	if err != nil {
		t.Skip("NOTHING MEASURED: cannot make a fixture root under " + varTmpRoot + ": " + err.Error())
	}
	t.Cleanup(func() { removeAllTolerant(t, d) })
	d = absResolve(d)
	for _, g := range []string{"/tmp", "/private/tmp", os.Getenv("TMPDIR")} {
		if g != "" && underDir(g, d) {
			t.Skip("NOTHING MEASURED: the fixture root " + d + " lies under " + g + ", which the writable set grants unconditionally")
		}
	}
	return d
}
