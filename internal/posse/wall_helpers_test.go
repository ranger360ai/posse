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
	"path/filepath"
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
	const base = "/var/tmp"
	if st, err := os.Stat(base); err != nil || !st.IsDir() {
		t.Skip("NOTHING MEASURED: no " + base + " on this host, so there is no fixture root outside every blanket temp grant")
	}
	d, err := os.MkdirTemp(base, "posse-ungranted-")
	if err != nil {
		t.Skip("NOTHING MEASURED: cannot make a fixture root under " + base + ": " + err.Error())
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
