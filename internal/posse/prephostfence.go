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
// own temp home, plus the two temp roots this package's fixtures are cut
// from — os.TempDir(), which is what t.TempDir() uses, and /var/tmp, which
// wallUngrantedDir uses deliberately because a t.TempDir() is under a blanket
// temp grant and ADR 0097 D1's wall venue puts its private mounts there.
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
	roots := []string{os.TempDir(), "/var/tmp"}
	if root != "" {
		roots = append([]string{root}, roots...)
	}
	prepHostFenceRoots = roots
	prepHostFenceKeepOut = ""
	if u, err := user.Current(); err == nil {
		prepHostFenceKeepOut = u.HomeDir
	}
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
