package posse

// The host fence a TEST BINARY puts round runBwrapPrep (ranger-base-bncbk).
//
// WHAT IT IS FOR. The prep is the one part of the bwrap wall that happens
// OUTSIDE it: the launcher is not in the namespace, so every PrepStep is a
// write on the host, made in the seat's name, at a path the RENDER chose.
// That is exactly right for a launch and exactly wrong for a test, because
// the render reads the host — and two of its inputs do not move when a
// fixture sets $HOME:
//
//   - liveSSHHomes' PASSWD half. `user.Current().HomeDir` is read from the
//     passwd database and ignores $HOME, so a test whose fixture home is a
//     t.TempDir() still renders `SSHDir: [<operator's real ~/.ssh>, <the
//     fixture's>]`. MEASURED 2026-11-21 on this box, by dumping the render
//     of readerprobeprep_qa_test.go's own fixture: both are there, and the
//     real one is the one the watch list names.
//   - XDG_STATE_HOME, which rexNodeStateDirs reads straight out of the
//     environment and no test fence unset. (Since ranger-base-3cjzs a
//     `~`- or `$HOME`-prefixed value is expanded against every candidate
//     home, so that half DOES move with a fixture's $HOME — but the fence
//     unsets the variable, so the arm contributes nothing either way.)
//
// Until ranger-base-bncbk neither mattered, because steps 5a and 5b skipped
// an absent directory and emitted no prep at all for a present one. Both now
// mkdir and anchor, so without a fence every test in this package that runs
// runBwrapPrep over a full render would plant `.posse-wall-anchor` in the
// operator's real ~/.ssh and in rex's live tsnet state dir. ranger-base-lin9y
// measured that happening and the files were removed by hand.
//
// TWO HALVES, the clihomefence.go pattern and for its reason — three test
// binaries need this and test code cannot be shared across packages:
//
//  1. THE FIX is FencePrepHost's siblings in FenceCLIHomes: the passwd home
//     is pointed at $HOME, so a fixture that moves $HOME moves it, and
//     XDG_STATE_HOME is unset. That is what makes the renders land inside
//     the fixtures.
//  2. THIS FENCE is the census pin over the fix — a step whose path is
//     outside every declared root REFUSES, naming the step and the roots.
//     It holds the FACT rather than a list of the tests that render: a test
//     added tomorrow, or a producer that starts reading a third host input,
//     reds here rather than writing into somebody's live area. The bead
//     asked for "no test may run runBwrapPrep over a step whose path is
//     outside t.TempDir()" and this is it, widened by exactly the two roots
//     this package's fixtures legitimately use.
//
// IT IS OFF UNTIL A BINARY DECLARES IT, and that is not a hole: a LAUNCH's
// prep must write wherever the render says — that is its whole job — so a
// fence that was on by default would refuse the thing it is modelled on. The
// hard arm is a pin that the fence is UP inside a real test
// (TestQATheBwrapPrepHostFenceIsUpAndRefusesAStepOutsideIt), which is what
// stops this quietly becoming a no-op the way a guard keyed on nothing would.

import (
	"os"
	"os/user"
	"strings"
)

// prepHostFenceRoots is the roots a prep step may write under, or nil in a
// process that declared no fence (a real launcher, and this test binary's
// re-exec arms that run before TestMain's fence call).
var prepHostFenceRoots []string

// prepHostFenceKeepOut is the passwd entry's home, read at fence time and
// never through sshPasswdHome — the seam the fence itself replaces — so that
// this arm cannot be moved by the thing it is checking. "" when passwd cannot
// be read, in which case this arm is simply absent.
var prepHostFenceKeepOut string

// FencePrepHost declares that fence for a test binary: `root` is the binary's
// own temp home, plus the three temp roots this package's fixtures are cut
// from — os.TempDir(), which is what t.TempDir() uses; /var/tmp, which
// wallUngrantedDir uses deliberately because a t.TempDir() is under a blanket
// temp grant and ADR 0097 D1's wall venue puts its private mounts there; and
// bwrapTmpMount, which wallStandDownDir uses just as deliberately for the
// opposite reason. Make that FOUR where this seat declared an ungranted root
// of its own (envUngrantedFixtureRoot, at the foot of this file):
// prepHostFenceRootList is the whole list and says why each is in there.
//
// Called from FenceCLIHomes so that every binary with a fence gets this one
// too: cmd/posse and internal/treepins both render walls through the same
// package, and a fence each of them has to remember is a fence one of them
// will not have (ranger-base-vxir7's lesson about the variable list).
// TWO ARMS, which is sbAssertHomeIsAFixture's shape at this mechanism. The
// ROOTS are the hard arm: they need no passwd database and no privileges, so
// the fence cannot quietly become a no-op. Their one blind spot is a box whose
// real home is itself under a temp root, where "under a fixture root" would be
// true of the thing the fence exists to refuse — so the second arm is a KEEP
// OUT at the passwd home.
//
// AND THE TWO ARE ORDERED BY CONTAINMENT, THE MORE SPECIFIC WINNING
// (ranger-base-cmedf). The keep-out used to refuse whatever the roots said,
// and that is the wrong answer in the one venue where these pins measure at
// all: ADR 0097 D1 runs them inside the landed-check judge's cage, whose
// TMPDIR is `<state>/landed-check/run-<n>/tmp` — under the posse state dir,
// which on this box is under the passwd home (landedCheckWorkDir says why it
// is not under /tmp). So `os.TempDir()` there, and every `t.TempDir()` cut
// from it, is INSIDE the keep-out, and the fence refused the fixtures it was
// widened for: MEASURED 2026-11-21, the wall check red at tree fa8245fcf640
// with `TestQATheBwrapPrepHostFenceIsUpAndRefusesAStepOutsideIt` itself
// reporting "the fence refused a step under t.TempDir()". Reproduced outside
// the venue by `TMPDIR=$HOME/<dir> go test -run <that pin> ./internal/posse`,
// which is the venue's shape and the cheap way back to it.
//
// A root therefore beats the keep-out exactly when it lies STRICTLY inside
// it — a run-scoped scratch directory somebody cut under the home — and the
// keep-out beats every root that contains it or equals it. Both blind spots
// stay closed by the same rule: a real home under /tmp is strictly inside
// that root, so the keep-out still wins there, and a root spelled AS the
// passwd home buys nothing. Nothing outside a declared root is reachable
// either way; what moved is only which of two nested declarations decides a
// path they both cover.
func FencePrepHost(root string) {
	// Only a VALID declaration widens the fence: an unusable one is
	// wallUngrantedDir's own hard failure, and a fence widened to a path
	// nothing can cut a fixture under is a fence widened for nothing.
	declared, _, refusal := ungrantedFixtureRoot()
	if refusal != "" {
		declared = ""
	}
	prepHostFenceRoots = prepHostFenceRootList(root, declared)
	prepHostFenceKeepOut = ""
	if u, err := user.Current(); err == nil {
		prepHostFenceKeepOut = u.HomeDir
	}
}

// prepHostFenceRootList is that root list, PURE, so a pin can read the rule
// without writing the package globals every parallel wall pin is reading
// (prepHostFenceWhy says why nothing in a test may set those).
//
// THE THIRD ENTRY IS THE DECLARED UNGRANTED ROOT (ranger-base-ydw7p). It is a
// fixture root exactly as `/var/tmp` is — `wallUngrantedDir` cuts its fixture
// home under it, and the render then names that home's `.ssh` — so a seat
// that moved the root and not the fence would watch every wall pin red at a
// refusal naming the FENCE instead of the wall it came to measure. "" when
// this seat declared nothing, which is every seat on a box whose /var/tmp is
// writable.
//
// THE STAND-DOWN ROOT IS IN HERE FOR THE SAME REASON AND NOT BY WAY OF
// os.TempDir() (ranger-base-shf97). `wallStandDownDir` cuts its fixture home
// under `bwrapTmpMount` — `/tmp`, the one blanket temp grant both hosts spell
// alike — because the render those fixtures drive must SUCCEED, and that is a
// root os.TempDir() names only while TMPDIR is unset or itself under /tmp. So
// the entry that covered it was the same host accident this fence's own
// fixtures were moved onto /tmp to escape: MEASURED 2026-11-22 (linux
// 7.2.15, go1.27.3, at 648a32d6), with `TMPDIR=/var/tmp/… go test -run
// TestQAWallSeedProjectConfig ./internal/posse` and the two arms taking a
// stand-down root, both reds moved off the render and onto THIS fence —
// "/tmp/posse-standdown-…/home/.cache/posse is under none of
// /var/tmp/…/posse-testhome-… /var/tmp/… /var/tmp" — one layer down from the
// refusal they had just been fixed out of.
//
// It widens the fence by no more than `/var/tmp` already does: both are
// world-shared temp roots that no live area of this box's operator or of rex
// lies in, and the keep-out arm still decides a passwd home STRICTLY inside
// either of them.
//
// THE STAGING-ROOT STEP OF THE STAND-DOWN SHAPE IS A STATED RESIDUAL, AND
// THESE ROOTS ADMIT IT (ADR 0100 D5). A render whose home or cwd lies under
// `/tmp` binds no private /tmp, keeps the blanket temp grants and renders the
// denies under them — `upgradePrivateDenied`'s staging root among them — so
// an uncaged test binary's prep creates `<temp root>/posse-upgrade` on the
// host, admitted here through `os.TempDir()` and through `bwrapTmpMount`.
// That is not a breach of this fence's purpose and NOT a reason to narrow it.
// The fence exists to refuse a step at a path the render took from a host
// input the fixture did not move (the passwd home, `XDG_STATE_HOME`); this
// path comes from the fixture's own shape, lies in a world-shared temp root
// this list admits by declaration, and is byte-for-byte the step a launch in
// that shape makes. Both entries are load bearing for the fixtures above —
// `t.TempDir()` under the first, `wallStandDownDir` under the last — so
// narrowing either would refuse the renders those fixtures drive, which is
// what D5's A9-A11 priced and rejected. The cost is one empty 0755 directory
// per temp root, at the name `posse upgrade` creates for itself anyway:
// docs/notes.d/ranger-base-2q0uy.md measured the class at 37 prep attempts
// in 16 files, all naming that one path.
//
// ORDER IS COSMETIC AND THE DECISION DOES NOT READ IT: prepHostFenceWhy
// unions the roots' coverage and unions the keep-out question over them, so
// what the order decides is only how the refusal sentence reads.
func prepHostFenceRootList(root, declared string) []string {
	var roots []string
	for _, r := range []string{root, declared} {
		if r != "" {
			roots = append(roots, r)
		}
	}
	return append(roots, os.TempDir(), "/var/tmp", bwrapTmpMount)
}

// PrepHostFenceRoots is the fence as it stands, so a pin can read it back and
// say it is up. A fresh slice: a caller cannot widen the fence by editing it.
func PrepHostFenceRoots() []string {
	return append([]string{}, prepHostFenceRoots...)
}

// prepHostFenceRefusal is the sentence saying why this step may not be made,
// or "" when it may. A declared fence that no root covers is the refusal; no
// fence at all is "".
func prepHostFenceRefusal(s PrepStep) string {
	return prepHostFenceWhy(prepHostFenceRoots, prepHostFenceKeepOut, s.Path)
}

// prepHostFenceWhy is that decision with the fence passed in rather than read
// off the package. PURE, because the rule is what needs pinning and the
// globals are what a test cannot move: every wall pin in this package calls
// runBwrapPrep and most of them are `t.Parallel`, so a table test that set
// prepHostFenceRoots would be racing the fence it was measuring. The pin over
// this function drives ADR 0097 D1's venue shape — a TMPDIR under the passwd
// home — which is the case ranger-base-cmedf was.
//
// THE MORE SPECIFIC OF TWO NESTED DECLARATIONS DECIDES, which is the head
// comment's ordering rule: a root strictly inside the keep-out is a scratch
// directory somebody cut under the home and admits what is under it; a root
// that contains the keep-out, or is spelled as it, does not.
func prepHostFenceWhy(roots []string, keepOut, path string) string {
	if len(roots) == 0 {
		return ""
	}
	covered, beatsKeepOut := false, false
	for _, r := range roots {
		if !underDir(r, path) {
			continue
		}
		covered = true
		if keepOut != "" && prepHostFenceInsideKeepOut(keepOut, r) {
			beatsKeepOut = true
		}
	}
	if keepOut != "" && underDir(keepOut, path) && !beatsKeepOut {
		return "the bwrap prep host fence is up (this is a test binary) and " + path +
			" is inside the passwd home " + keepOut +
			", which is the operator's own area and which no root containing it can open: a prep step is a write on the HOST in the seat's name, and ranger-base-lin9y measured a test planting .posse-wall-anchor in the operator's real ~/.ssh this way. Only a root STRICTLY inside that home — a run-scoped scratch dir, which is what ADR 0097 D1's venue TMPDIR is — admits a path under it. The fixture's $HOME does not reach the producer that named this path — prephostfence.go names the two host inputs that do not move with it"
	}
	if !covered {
		return "the bwrap prep host fence is up (this is a test binary) and " + path +
			" is under none of " + strings.Join(roots, " ") +
			": a prep step is a write on the HOST in the seat's name, at a path the render chose, so a step outside every fixture root is this test about to write into somebody's live area — the operator's own ~/.ssh and rex's tsnet state dir are what ranger-base-bncbk measured. Either the fixture's $HOME does not reach the producer that named this path (see prephostfence.go for the two host inputs that do not move with it), or a new producer reads a host input no fence covers"
	}
	return ""
}

// prepHostFenceInsideKeepOut is "root lies strictly inside keepOut". STRICTLY:
// a root spelled as the passwd home itself would otherwise open the whole home,
// which is the one thing this fence exists to refuse.
func prepHostFenceInsideKeepOut(keepOut, root string) bool {
	k, r := absResolve(keepOut), absResolve(root)
	return r != k && insideDir(k, r)
}

// ─── the fixture root itself, and the seat whose wall has no /var/tmp ────────

// envUngrantedFixtureRoot is the variable a seat points at a directory it can
// write and no RENDERED wall grants, to be used instead of `/var/tmp` as the
// root every wall pin's fixture is cut from (`wallUngrantedDir`,
// wall_helpers_test.go).
//
// WHY A VARIABLE AT ALL (ranger-base-ydw7p, found verifying ranger-base-xab99
// from a security seat). `/var/tmp` is the one directory on both hosts that no
// blanket temp grant covers, which is what the helper needs — and it is
// read-only inside a seat's own bwrap wall, which `--ro-bind / /` makes of
// everything it does not bind back. So on the box where these pins matter
// most, a seat running them got `mkdir: read-only file system`, the helper
// skipped, and the package said `ok` over 4 of ranger-base-xab99's 5 pins
// having measured NOTHING. 77 call sites are in that position from such a
// seat, and they are the ADR 0100 wall pins the security lane is asked to
// re-run when it verifies a fix. The out-of-tree answer — unpack the tree
// somewhere else and sed the const — is not available to a seat whose PID
// denies Write.
//
// THE OTHER SHAPE, said here because it is still open and this is not it: the
// LAUNCH could grant every verifying seat such a directory and have `posse
// gates` print it, which costs a launch-line change and is the operator's
// (guardrail 5). This variable needs no launch and no install, so it is what
// a seat behind today's wall can use today.
//
// IT IS NOT A WAY ROUND THE RULE, and ungrantedFixtureRoot holds it to the
// same one the `/var/tmp` arm applies to itself: a root inside `/tmp`,
// `/private/tmp` or `$TMPDIR` is granted by every rendered profile
// unconditionally (seatbeltTempGrants), so a fixture there measures the
// grant and not the PID — ADR 0086's own cautionary measurement. A value
// that breaks the rule is REFUSED and never quietly skipped: a seat that
// declared a root asked to measure these pins.
const envUngrantedFixtureRoot = "POSSE_TEST_UNGRANTED_ROOT"

// ungrantedRootDark is the one value of that variable that is not a path: the
// seat SAYING it has no such directory here. The pins still abstain, and the
// run is allowed to read green — which is what the undeclared abstention is
// not (wallUngrantedVerdict). It is spelled as a word no path can be, so a
// relative path typed by mistake cannot land on it.
const ungrantedRootDark = "none"

// ungrantedBlanketGrants is the write grants a rendered profile carries
// whatever its PID, its persona or its cwd says, as the fixture rule needs to
// read them: `/tmp`, darwin's `/private/tmp` spelling of it, and `$TMPDIR`.
//
// A LITERAL LIST AND NOT seatbeltTempGrants, which is the production spelling
// of the same three. That one is a `var` this package's own fixtures stub to
// nil on a box whose TMPDIR is under /tmp (sbRoot, seatbelt.go) — so a rule
// that read it would be a rule a stub could switch off, and the one thing
// this check may not do is pass a root because something made the grant list
// empty.
func ungrantedBlanketGrants() []string {
	return []string{"/tmp", "/private/tmp", os.Getenv("TMPDIR")}
}

// ungrantedGrantRefusal is "" when p lies outside every blanket temp grant,
// and otherwise the sentence naming the grant that covers it. Both arms of
// the helper go through it, so the declared root and `/var/tmp` are held to
// one rule and cannot drift apart.
func ungrantedGrantRefusal(p string) string {
	for _, g := range ungrantedBlanketGrants() {
		if g != "" && underDir(g, p) {
			return "the fixture root " + p + " lies under " + g +
				", which every rendered profile grants write unconditionally (seatbeltTempGrants), so a fixture inside it is writable under the wall for a reason that has nothing to do with the PID under test"
		}
	}
	return ""
}

// ungrantedFixtureRoot reads envUngrantedFixtureRoot: the declared root,
// whether a dark run was declared, and the refusal when the value cannot be
// used. Exactly one of the three is ever non-zero.
//
// PURE OF THE TESTING PACKAGE, so FencePrepHost above can call it from a
// binary's startup and a pin can drive the rule as a table without a fixture.
// What it validates is what a parent of os.MkdirTemp has to be — absolute (a
// relative root would be read against whatever cwd the caller happens to
// have, and these tests chdir), present, and a directory — plus the grant
// rule above.
func ungrantedFixtureRoot() (root string, dark bool, refusal string) {
	v := os.Getenv(envUngrantedFixtureRoot)
	switch {
	case v == "":
		return "", false, ""
	case v == ungrantedRootDark:
		return "", true, ""
	case !strings.HasPrefix(v, "/"):
		return "", false, v + " is not an absolute path, and a fixture root is read from a binary that chdirs: name the directory in full"
	}
	st, err := os.Stat(v)
	switch {
	case err != nil:
		return "", false, "cannot read " + v + ": " + err.Error() + " — make the directory first (`mkdir -p`), or set " + envUngrantedFixtureRoot + "=" + ungrantedRootDark + " to declare that this seat measures none of the wall pins"
	case !st.IsDir():
		return "", false, v + " is not a directory"
	}
	if why := ungrantedGrantRefusal(absResolve(v)); why != "" {
		return "", false, why
	}
	return v, false, ""
}
