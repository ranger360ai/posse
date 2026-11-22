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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// wallUnixWrapForTest is the socket-wall step as the render appends it to the
// LAUNCH spelling (ranger-base-6rndn), or "" where there is none — another
// mechanism, or a kernel below Landlock ABI 9. Here rather than inline in
// wallLaunchWrapForTest for this file's own stated reason: a pin comparing a
// launch line against an expectation must compare it to the ONE spelling of
// that line, and the step is part of it now.
//
// NOT in wallWrapForTest, which is `Prefix`: that is the probes' spelling and
// carries no launch-only wrapper, the same rule the wall-watch sidecar
// follows (wall.go's bwrap arm says why).
func wallUnixWrapForTest(t *testing.T) string {
	t.Helper()
	if !WallUnixAvailable() {
		return ""
	}
	self, err := wallWatchBin()
	if err != nil {
		t.Fatalf("this host renders the socket wall and this posse binary does not resolve, so no launch here renders a line at all: %v", err)
	}
	return WallUnixPrefix(self)
}

// wallLaunchWrapForTest is what a LAUNCH types for that wall: at bwrap the
// `posse wall-watch` sidecar in front of the binary (ranger-base-ihnnl),
// and at sandbox-exec the seat's env prefix in front of it — both built
// from the one spelling the render types.
//
// THE SANDBOX-EXEC ARM IS A PREFIX IN FRONT OF A PREFIX, and that is the
// whole of what this arm is for. SBPL has no env of its own, so every
// variable the launcher hands a darwin seat rides in front of
// `sandbox-exec` as `/usr/bin/env K=V …` — the seat's own build and package
// cache (ranger-base-olr0g) and, for a seat the seat-store wall applies to,
// its two object-store variables (ranger-base-5blmt). wall.go's
// WallSandboxExec arm puts both on `WallRender.Prefix` itself, so a pin
// that expected SeatbeltPrefix alone was comparing the line to a prefix the
// launcher stopped typing bare: `GATES <wrap>grok` is `GATES /usr/bin/env
// 'GOCACHE=…' … sandbox-exec -f '…' grok` on a real line, so an assertion
// anchored at GATES was red on every macos leg from olr0g's landing and
// green on every ubuntu one, where those variables go in bwrap's argv
// instead (ranger-base-jyqk4, MEASURED on macos-latest arm 3).
//
// `ag` AND NOT A SEAT NAME, because seatStoreEnv is nil for a PID that
// denies file writes (seatbeltSeatStore reads ag.Deny): the two arms differ
// by exactly those two variables, and a helper handed a name alone would
// have to guess which seat it was.
func wallLaunchWrapForTest(t *testing.T, gatesDir string, ag *AgentFile, dir string) (wrap, artifact string) {
	t.Helper()
	wrap, artifact = wallWrapForTest(t, gatesDir)
	if WallMechanism() != WallBwrap {
		home, _ := os.UserHomeDir()
		env := append(seatCacheEnv(SeatCacheDir(home, dir)), seatStoreEnv(ag, home, dir)...)
		return seatCacheEnvPrefix(env) + wrap, artifact
	}
	seat := ag.Name
	bin, err := bwrapBin()
	if err != nil {
		t.Fatal(err)
	}
	self, err := wallWatchBin()
	if err != nil {
		t.Fatal(err)
	}
	return WallWatchPrefix(self, filepath.Join(gatesDir, bwrapMountsName), seat, dir, bin, artifact) +
		wallUnixWrapForTest(t), artifact
}

// The ADR 0096 D3 nest gate used to live here, untagged. It is in
// wallvenue_test.go now, behind the arm 1 build tag, because ADR 0097 D4
// makes a member in arm 2 or arm 3 a compile error (ranger-base-q749c).

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
	// THE DECLARED ROOT FIRST, when this seat declared one
	// (ranger-base-ydw7p, and prephostfence.go's envUngrantedFixtureRoot
	// carries the argument). Inside a seat's own bwrap wall `/var/tmp` is
	// read-only, so every arm below abstains and a verify reads `ok` over
	// pins that measured nothing; a seat that can write somewhere outside
	// every blanket temp grant says so there and measures them here.
	//
	// ITS FAILURES ARE FAILURES AND NOT ABSTENTIONS, both of them: a seat
	// that declared a root asked to measure these pins, so a value that
	// cannot be used, or a root that cannot be written, is a red naming the
	// variable rather than one more silent skip. That is the whole defect
	// this arm exists for, and re-introducing it here would be the joke.
	declared, dark, refusal := ungrantedFixtureRoot()
	switch {
	case refusal != "":
		t.Fatalf("%s=%q: %s", envUngrantedFixtureRoot, os.Getenv(envUngrantedFixtureRoot), refusal)
	case dark:
		// The one declared abstention: this seat has said it has no such
		// directory, so the skip is counted and the run may still read
		// green (wallUngrantedVerdict).
		wallUngrantedAbstain(t, "a dark run is declared by "+envUngrantedFixtureRoot+"="+ungrantedRootDark+", so no wall pin measures from this seat")
	case declared != "":
		d, err := os.MkdirTemp(declared, "posse-ungranted-")
		if err != nil {
			t.Fatalf("%s=%s: cannot make a fixture root under it: %v — point it at a directory this seat can write, or set it to %s to declare a dark run",
				envUngrantedFixtureRoot, declared, err, ungrantedRootDark)
		}
		t.Cleanup(func() { removeAllTolerant(t, d) })
		// Resolved and re-checked, exactly as the /var/tmp arm is: the
		// grant list is read against the ANSWER and not against the
		// declaration, because a box that symlinked the declared root into
		// `/tmp` would hand every caller a granted root and every refusal
		// below would be measuring the other fixture.
		d = absResolve(d)
		if why := ungrantedGrantRefusal(d); why != "" {
			t.Fatalf("%s=%s: %s", envUngrantedFixtureRoot, declared, why)
		}
		return d
	}
	if st, err := os.Stat(varTmpRoot); err != nil || !st.IsDir() {
		wallUngrantedAbstain(t, "no "+varTmpRoot+" on this host, so there is no fixture root outside every blanket temp grant")
	}
	d, err := os.MkdirTemp(varTmpRoot, "posse-ungranted-")
	if err != nil {
		wallUngrantedAbstain(t, "cannot make a fixture root under "+varTmpRoot+": "+err.Error())
	}
	t.Cleanup(func() { removeAllTolerant(t, d) })
	d = absResolve(d)
	if why := ungrantedGrantRefusal(d); why != "" {
		wallUngrantedAbstain(t, why)
	}
	return d
}

// wallStandDownDir is wallUngrantedDir's OPPOSITE, and it is the fixture root
// for a pin whose subject is a bwrap render that must SUCCEED: a directory
// inside the one blanket temp grant both hosts spell the same way, `/tmp`
// (bwrapTmpMount), which is where the render's own stand-downs are.
//
// WHY A FIXTURE NEEDS ONE. BwrapArgs judges every write grant against the
// per-seat wall root composed from $HOME (bwrapRefuseWallRootGrant) and
// REFUSES a grant that contains it — and a blanket temp grant contains any
// fixture home cut under that temp root, so a pin whose home is a plain
// `t.TempDir()` is asking the render to refuse itself. The refusal stands
// down where the home is under `/tmp`, for the reason its own comment gives:
// `/tmp` is granted unconditionally, a home inside it is in the shared
// directory already, so there is no seat there for a wall root to protect.
// bwrapSeatTmp's private-/tmp mount stands down over the same question.
//
// SO THE HOST IS THE WHOLE OF IT, and that is why this helper exists rather
// than a sentence at each call site. On linux `t.TempDir()` IS under /tmp and
// both stand-downs were free; on darwin it is $TMPDIR under `/var/folders/…`,
// granted just as unconditionally but not `/tmp`, so the containment refusal
// spoke and the arm read it instead of the thing it came to measure. MEASURED
// on macos arm 1 at 2a50bcb5 (run 44990274146, job 137109586827) and
// reproduced here with `TMPDIR=/var/tmp/…`, same lines and same messages
// (ranger-base-1aayx). ranger-base-zbmxc took this same answer one file over
// for the same reason — bwrapstatedirs_qa_test.go's bwsHome — and ruled out
// both alternatives there: widening the stand-down weakens a live refusal on
// linux, and skipping the render off linux retires the pin on the only host
// that runs it.
//
// RESOLVED, because the raw/resolved split is the class this is in: on darwin
// `/tmp` is itself a link to `/private/tmp`, so a fixture handed the raw
// spelling would plant every path in the pin at one spelling and have every
// producer answer at the other.
//
// os.MkdirTemp with a tolerant removal and not t.TempDir(), for
// wallUngrantedDir's reason: callers `git init` in here
// (TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot is the pin). The
// `posse-` prefix is the temp-root census's
// (internal/treepins/temproot_qa_test.go): this root outlives no SIGKILL and
// no -timeout panic, and `posse-*` is the only class scripts/reap-test-tmp.sh
// reaches under /tmp.
func wallStandDownDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp(bwrapTmpMount, "posse-standdown-")
	if err != nil {
		t.Fatalf("cannot make a fixture root under %s: %v", bwrapTmpMount, err)
	}
	t.Cleanup(func() { removeAllTolerant(t, d) })
	d = absResolve(d)
	// The ANSWER and not the spelling, exactly as wallUngrantedDir checks its
	// own: a box whose `/tmp` led somewhere outside the grant would hand every
	// caller a root the stand-downs do not cover, and every arm below would be
	// reading a refusal instead of a render.
	if !underDir(bwrapTmpMount, d) {
		t.Fatalf("%s is not inside %s, so the render's stand-downs do not cover this fixture and every arm over it measures a refusal", d, bwrapTmpMount)
	}
	return d
}

// ─── an abstention here is counted, and an undeclared one is not a pass ─────
//
// ADR 0097 D5's rule — "an abstention is never a pass" — one layer in from
// where that record states it. `scripts/posse-wall-check` keeps it for the
// VENUE, by reading `go test -json` and answering exit 3 over a member that
// went dark; it is the ADR's own `/var/tmp` row, where this very helper took
// 18 pins dark over a green exit code. Nothing kept it for the run a PERSON
// types: `go test -run <the pins> ./internal/posse` from a seat whose wall
// makes /var/tmp read-only printed `ok` over five pins of which one had run
// (ranger-base-oejcn, which closed on that reading plus a code read).
//
// IT HAS TO BE THE EXIT STATUS, and that is MEASURED and not a preference
// (2026-11-21, go1.27.3, linux): `go test` without `-v` DISCARDS everything
// a passing package's binary writes to stdout and stderr, TestMain's own
// writes included — so a summary line printed after m.Run is seen by nobody
// in the one run shape this defect appears in. The same line is relayed in
// full the moment the binary exits nonzero. So the count decides the status,
// and the sentence rides out with it.
//
// A FILTERED RUN IS THE CASE THAT MATTERS, which is why this is not a pin of
// its own. A dedicated test that reds when no root is usable is unselected by
// `-run 'TestQAKeepLeaf…'` and the verify reads `ok` again; a TestMain arm is
// read by every run shape there is.
//
// THE TWO COUNTS ARE SEPARATE AND THAT IS LOAD-BEARING, not bookkeeping. A
// single count plus a "was it declared" flag reads whatever the LAST
// abstention of the run happened to be — and this bead's own pin
// (wallungrantedroot_qa_test.go) declares a dark run for one subtest, so one
// probe would have set that flag for every real abstention after it and the
// mechanism would have reported "declared" forever while failing nothing.
var wallUngranted struct {
	mu       sync.Mutex
	silent   int    // nobody declared these: the defect
	declared int    // the seat said it has no such root here
	why      string // the first reason, which is every reason in practice
}

// wallUngrantedAbstain counts this caller's abstention and skips it. Every
// `NOTHING MEASURED` wallUngrantedDir can answer with goes through here, so
// the count cannot be short by a reason somebody added later.
func wallUngrantedAbstain(t *testing.T, why string) {
	t.Helper()
	_, dark, _ := ungrantedFixtureRoot()
	wallUngranted.mu.Lock()
	if dark {
		wallUngranted.declared++
	} else {
		wallUngranted.silent++
	}
	if wallUngranted.why == "" {
		wallUngranted.why = why
	}
	wallUngranted.mu.Unlock()
	t.Skip("NOTHING MEASURED: " + why)
}

// wallUngrantedVerdict is what TestMain does with that count: the exit status
// the run should carry, and the line that says why. PURE, so the rule is read
// by a pin rather than by an exit code somebody has to reproduce.
//
// A DECLARED DARK RUN KEEPS THE STATUS IT EARNED and still prints the count.
// The seat has said it cannot measure these pins here, which is a different
// thing from a seat that believes it did: the reader of such a run already
// knows, and a red it cannot clear would only teach it to stop reading the
// line.
func wallUngrantedVerdict(code, silent, declared int, why string) (int, string) {
	if silent+declared == 0 {
		return code, ""
	}
	line := fmt.Sprintf("posse wall fixtures: %d test(s) that need a fixture root outside every blanket temp grant ABSTAINED and measured NOTHING (%s)",
		silent+declared, why)
	if silent == 0 {
		return code, line + " — all " + strconv.Itoa(declared) + " declared, by " + envUngrantedFixtureRoot + "=" + ungrantedRootDark + " (ranger-base-ydw7p)"
	}
	if code == 0 {
		code = 1
	}
	return code, line + ", " + strconv.Itoa(silent) + " of them undeclared. That is exit " + strconv.Itoa(code) + " and not `ok`: an abstention is never a pass (ADR 0097 D5). Point " +
		envUngrantedFixtureRoot + " at an existing directory this seat can write that lies outside /tmp, /private/tmp and $TMPDIR — inside a seat's wall `/var/tmp` is read-only, so the place to look is that wall's own write grants, of which `~/.cache/posse` is one on both hosts (homeCacheGrants) — or set it to " +
		ungrantedRootDark + " to declare that this seat measures none of them (ranger-base-ydw7p)"
}

// wallUngrantedReport is the two things TestMain needs: call it after m.Run.
func wallUngrantedReport(code int) (int, string) {
	wallUngranted.mu.Lock()
	defer wallUngranted.mu.Unlock()
	return wallUngrantedVerdict(code, wallUngranted.silent, wallUngranted.declared, wallUngranted.why)
}

// wallUngrantedCounts is the pair as it stands, for the pin that drives an
// abstention and reads the delta.
func wallUngrantedCounts() (silent, declared int) {
	wallUngranted.mu.Lock()
	defer wallUngranted.mu.Unlock()
	return wallUngranted.silent, wallUngranted.declared
}
