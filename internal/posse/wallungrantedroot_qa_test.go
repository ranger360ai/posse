package posse

// QA pins for ranger-base-ydw7p — the fixture root every wall pin needs, the
// seat that cannot provide it, and the run that may not read `ok` over either.
//
// THE DEFECT, MEASURED 2026-11-21 from a security seat on this box (linux
// 7.2.15) while verifying ranger-base-xab99's fix. `wallUngrantedDir`
// (wall_helpers_test.go) cut its fixture root under `/var/tmp`, which is the
// one directory on both hosts that no blanket temp grant covers — and which a
// seat's own bwrap wall makes read-only, `--ro-bind / /` being read-only over
// everything it does not bind back. So the helper answered `mkdir:
// read-only file system`, skipped, and `go test -run 'TestQAKeepLeaf…'`
// printed `ok` with 4 of that bead's 5 pins having measured NOTHING. 77 call
// sites take their root from that helper and every one of them is in the same
// position from such a seat — and they are the ADR 0100 wall pins the
// security lane is asked to re-run when it verifies a fix. ranger-base-oejcn
// closed on a code read plus one executed pin because of it.
//
// TWO HALVES, and the second is the one that generalises:
//
//  1. A DECLARED ROOT. envUngrantedFixtureRoot (prephostfence.go) names a
//     directory the seat can write, held to the same rule the `/var/tmp` arm
//     applies to itself, and it becomes a prep-host fence root so that the
//     fixtures' own prep steps are not refused by the fence instead.
//  2. AN ABSTENTION IS NEVER A PASS (ADR 0097 D5), kept for the run a person
//     types rather than only for the venue. Every `NOTHING MEASURED` the
//     helper can answer with is counted, and an undeclared count decides the
//     exit status — because `go test` without `-v` discards what a PASSING
//     package's binary writes to stdout and stderr, so a summary line is
//     read by nobody in the one run shape this defect appears in (MEASURED
//     2026-11-21, go1.27.3: TestMain's own writes are relayed in full the
//     moment the binary exits nonzero, and not before).
//
// WHAT IS STILL OPEN, said here because this file is where a reader will look
// for it: the rule holds a declared root clear of the three BLANKET grants,
// which are the grants that are the same for every render. A grant some
// render derives from a host input that does not move with the fixture's
// $HOME could still cover such a root, and no rule here can see that — the
// `/var/tmp` arm has the same residual and has always had it. The fence's own
// head comment (prephostfence.go) names the two inputs of that kind this
// package knows about.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Pin: the declaration rule — which values are a root, which are a declared
// dark run, and which are refused. Driven over ungrantedFixtureRoot itself,
// which is pure of the testing package precisely so this can be a table.
//
// EVERY REFUSAL ARM IS A REFUSAL AND NOT A SKIP, which is the bead's own
// lesson: a seat that declared a root asked to measure these pins, so a
// value it cannot use has to say so where the typist can see it.
func TestQAUngrantedFixtureRootRefusesEveryUnusableDeclaration(t *testing.T) {
	// No t.Parallel: sets the variable under test.
	granted := t.TempDir() // under TMPDIR else /tmp, which is the point
	file := filepath.Join(granted, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The accepted arm needs an absolute, existing directory outside all
	// three blanket grants, on any host this ever runs on. `/` is the one
	// path that is always all four — the rule is about the PATH and says
	// nothing about whether a fixture would be sensible there — and the
	// repo's own checkout is the realistic second, read from the cwd `go
	// test` gives this package and used only when this box's checkout is
	// not itself under a temp root.
	accepted := []string{"/"}
	if cwd, err := os.Getwd(); err == nil && ungrantedGrantRefusal(absResolve(cwd)) == "" {
		accepted = append(accepted, cwd)
	}
	for _, dir := range accepted {
		t.Setenv(envUngrantedFixtureRoot, dir)
		root, dark, refusal := ungrantedFixtureRoot()
		if root != dir || dark || refusal != "" {
			t.Errorf("%s=%s: got root=%q dark=%v refusal=%q, want the directory itself", envUngrantedFixtureRoot, dir, root, dark, refusal)
		}
	}

	for _, c := range []struct {
		name  string
		value string
		want  string // a fragment the refusal must carry
	}{
		{"relative", "var/tmp/here", "absolute"},
		{"absent", filepath.Join(granted, "no-such-dir"), "cannot read"},
		{"a file", file, "not a directory"},
		{"under the blanket temp grant", granted, "unconditionally"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(envUngrantedFixtureRoot, c.value)
			root, dark, refusal := ungrantedFixtureRoot()
			if refusal == "" {
				t.Fatalf("%s=%s was accepted (root=%q dark=%v) — a declaration this helper cannot cut a fixture under must refuse", envUngrantedFixtureRoot, c.value, root, dark)
			}
			if root != "" || dark {
				t.Errorf("a refused declaration also answered root=%q dark=%v: exactly one of the three is ever non-zero", root, dark)
			}
			if !strings.Contains(refusal, c.want) {
				t.Errorf("the refusal does not say %q: %s", c.want, refusal)
			}
		})
	}

	// And the two values that are not a path at all.
	t.Run("unset", func(t *testing.T) {
		t.Setenv(envUngrantedFixtureRoot, "")
		if root, dark, refusal := ungrantedFixtureRoot(); root != "" || dark || refusal != "" {
			t.Errorf("unset must declare nothing, got root=%q dark=%v refusal=%q", root, dark, refusal)
		}
	})
	t.Run("declared dark", func(t *testing.T) {
		t.Setenv(envUngrantedFixtureRoot, ungrantedRootDark)
		root, dark, refusal := ungrantedFixtureRoot()
		if !dark || root != "" || refusal != "" {
			t.Errorf("%s=%s must be the declared dark run, got root=%q dark=%v refusal=%q", envUngrantedFixtureRoot, ungrantedRootDark, root, dark, refusal)
		}
	})
}

// Pin: a USABLE declaration is where the fixture is actually cut, and it costs
// no abstention.
//
// THE PARENT COMES FROM THE HELPER ITSELF, which is the only honest way to
// get one: a directory outside every blanket temp grant is exactly what this
// host may not have, and on a seat where it does not, this pin abstains
// through the same counted path as the 77 — which reds the run, which is the
// other half of this bead working.
func TestQAADeclaredUngrantedRootIsWhereTheFixtureIsCut(t *testing.T) {
	// No t.Parallel: sets the variable under test.
	parent := wallUngrantedDir(t)
	silent0, declared0 := wallUngrantedCounts()
	t.Setenv(envUngrantedFixtureRoot, parent)
	d := wallUngrantedDir(t)
	if !underDir(parent, d) {
		t.Fatalf("with %s=%s the fixture root came back as %s, which is not under it", envUngrantedFixtureRoot, parent, d)
	}
	if st, err := os.Stat(d); err != nil || !st.IsDir() {
		t.Fatalf("the fixture root %s is not a directory: %v", d, err)
	}
	// The reaper's namespace, which is what makes a root a killed run leaves
	// behind collectable at all (internal/treepins/temproot_qa_test.go). The
	// declared arm cuts its root by a spelling that census cannot read, so
	// this is the only thing holding the prefix on it.
	if base := filepath.Base(d); !strings.HasPrefix(base, "posse-ungranted-") {
		t.Errorf("the fixture root is named %s — a root outside the `posse-` namespace is matched by no class of scripts/reap-test-tmp.sh, at any age", base)
	}
	if silent, declared := wallUngrantedCounts(); silent != silent0 || declared != declared0 {
		t.Errorf("a usable declaration counted an abstention: silent %d->%d, declared %d->%d", silent0, silent, declared0, declared)
	}
}

// Pin: a declared root is a prep-host fence root, and a refused one is not.
//
// WHY IT MATTERS, and it is not symmetry: these fixtures' prep steps write on
// the HOST at paths the render chose, under the fixture $HOME, which is
// INSIDE this root. A seat that moved the root and not the fence would watch
// every wall pin red at a refusal naming the fence — prephostfence.go's
// sentence, about a test writing into somebody's live area — for a reason
// that has nothing to do with the wall it came to measure.
//
// Over prepHostFenceRootList and never FencePrepHost: the live fence is
// package state that every parallel wall pin is reading, and a test that set
// it would be racing the thing it measures (prepHostFenceWhy).
func TestQAADeclaredUngrantedRootIsAPrepHostFenceRoot(t *testing.T) {
	t.Parallel()
	const home, declared = "/fixtures/home", "/elsewhere/ungranted"
	roots := prepHostFenceRootList(home, declared)
	for _, want := range []string{home, declared, os.TempDir(), "/var/tmp"} {
		if !slices.Contains(roots, want) {
			t.Errorf("the fence roots %v do not name %s", roots, want)
		}
	}
	// And the step under it is admitted, which is the fact the list exists
	// for. The keep-out is "" here: whether a path under the passwd home
	// also needs that arm is TestQATheBwrapPrepHostFenceIsUpAndRefuses...'s
	// subject, and this pin is about the root being declared at all.
	if why := prepHostFenceWhy(roots, "", filepath.Join(declared, "posse-ungranted-1", "home", ".ssh")); why != "" {
		t.Errorf("a prep step inside the declared root is refused: %s", why)
	}
	// A declaration the rule refuses widens nothing — FencePrepHost passes
	// "" for one, and a fence widened to a path no fixture can be cut under
	// is a fence widened for nothing.
	if roots := prepHostFenceRootList(home, ""); slices.Contains(roots, "") {
		t.Errorf("an undeclared root left an empty entry in the fence roots %v, which underDir reads as the process's cwd", roots)
	}
}

// Pin: the STAND-DOWN root is a prep-host fence root too, and it is one in
// its own right rather than by way of os.TempDir().
//
// The pin above's argument, for the other fixture root (ranger-base-shf97).
// `wallStandDownDir` cuts its home under `bwrapTmpMount` because the renders
// those fixtures drive must SUCCEED — and those renders' prep steps then
// write on the HOST at paths under that home, which is what the fence judges.
// os.TempDir() names `/tmp` only while TMPDIR is unset or itself under it, so
// before this entry existed the coverage was the same host accident
// wallStandDownDir exists to escape: MEASURED 2026-11-22 with
// `TMPDIR=/var/tmp/… go test -run TestQAWallSeedProjectConfig
// ./internal/posse`, where the two arms newly cut on a stand-down root read a
// refusal naming the FENCE — the step at `/tmp/posse-standdown-…/home/.cache/
// posse` under none of the declared roots — in place of the wall-root refusal
// they had just been fixed out of.
//
// Over prepHostFenceRootList and never FencePrepHost, for the pin above's
// reason: the live fence is package state every parallel wall pin reads.
func TestQATheStandDownFixtureRootIsAPrepHostFenceRoot(t *testing.T) {
	t.Parallel()
	const home = "/fixtures/home"
	roots := prepHostFenceRootList(home, "")
	if !slices.Contains(roots, bwrapTmpMount) {
		t.Errorf("the fence roots %v do not name %s, which is where wallStandDownDir cuts every fixture whose render must succeed — so on a box whose TMPDIR is outside %s their prep steps are refused by the fence instead", roots, bwrapTmpMount, bwrapTmpMount)
	}
	// And it is NAMED rather than inherited, which is the half that was
	// missing: the os.TempDir() entry is dropped here, so what admits the
	// step below is the stand-down entry alone. That models the seat the
	// defect appears on — TMPDIR outside /tmp — without setting TMPDIR in a
	// process where every parallel test's t.TempDir() reads it.
	// ONE occurrence and not every match, because on a box whose TMPDIR is
	// unset os.TempDir() IS bwrapTmpMount: dropping both would take the
	// named entry away too and this arm would red over the accident it is
	// measuring the absence of.
	named, dropped := []string{}, false
	for _, r := range roots {
		if r == os.TempDir() && !dropped {
			dropped = true
			continue
		}
		named = append(named, r)
	}
	if !dropped {
		t.Fatalf("the fence roots %v name no os.TempDir() entry (%s) — this arm models a box where that entry is not /tmp and there is nothing here to drop", roots, os.TempDir())
	}
	step := filepath.Join(bwrapTmpMount, "posse-standdown-1", "home", ".cache", "posse")
	if why := prepHostFenceWhy(named, "", step); why != "" {
		t.Errorf("with os.TempDir() out of the list, a prep step under a stand-down fixture home is refused: %s — so the coverage was that entry's and a seat whose TMPDIR is outside %s reads this refusal instead of the wall", why, bwrapTmpMount)
	}
}

// Pin: the verdict. An undeclared abstention decides the exit status; a
// declared one is reported and changes nothing; a run that was already red
// stays red with its own status.
func TestQAAnUndeclaredWallFixtureAbstentionIsNotAPass(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name               string
		code               int
		silent, declared   int
		wantCode           int
		wantLine, wantGone []string
	}{
		{
			name: "nothing abstained", code: 0, wantCode: 0, wantGone: []string{"ABSTAINED"},
		},
		{
			name: "one silent abstention over a green run", code: 0, silent: 1, wantCode: 1,
			wantLine: []string{"ABSTAINED", "measured NOTHING", "never a pass", envUngrantedFixtureRoot, "ranger-base-ydw7p"},
		},
		{
			name: "silent abstentions over an already red run", code: 2, silent: 18, wantCode: 2,
			wantLine: []string{"18 test(s)", "18 of them undeclared"},
		},
		{
			name: "declared", code: 0, declared: 77, wantCode: 0,
			wantLine: []string{"77 test(s)", "all 77 declared", ungrantedRootDark},
			wantGone: []string{"never a pass"},
		},
		{
			name: "declared and silent together", code: 0, silent: 1, declared: 2, wantCode: 1,
			wantLine: []string{"3 test(s)", "1 of them undeclared"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			code, line := wallUngrantedVerdict(c.code, c.silent, c.declared, "the reason")
			if code != c.wantCode {
				t.Errorf("exit status %d, want %d (line: %s)", code, c.wantCode, line)
			}
			if c.silent+c.declared == 0 && line != "" {
				t.Errorf("a run with no abstention printed %q", line)
			}
			for _, want := range c.wantLine {
				if !strings.Contains(line, want) {
					t.Errorf("the line does not say %q: %s", want, line)
				}
			}
			for _, gone := range c.wantGone {
				if strings.Contains(line, gone) {
					t.Errorf("the line says %q when it must not: %s", gone, line)
				}
			}
		})
	}
}

// Pin: the counting is WIRED — the helper's abstention goes through the
// counter, and a declared dark run abstains without reddening the run.
//
// Driven through the helper rather than asserted over the counter, because
// the rot this file is about is an abstention that nothing counts: a reason
// added to wallUngrantedDir tomorrow as a bare t.Skip would leave every arm
// above green and the defect exactly where it was.
func TestQATheWallFixtureHelperCountsADeclaredDarkAbstention(t *testing.T) {
	// No t.Parallel: sets the variable under test.
	t.Setenv(envUngrantedFixtureRoot, ungrantedRootDark)
	silent0, declared0 := wallUngrantedCounts()
	returned := false
	if ok := t.Run("the caller", func(t *testing.T) {
		wallUngrantedDir(t)
		returned = true
	}); !ok {
		t.Fatal("the caller FAILED where a declared dark run must make it SKIP")
	}
	if returned {
		t.Error("wallUngrantedDir handed back a fixture root under a declared dark run")
	}
	silent, declared := wallUngrantedCounts()
	if declared != declared0+1 {
		t.Errorf("the declared abstention count went %d -> %d, want one more: an abstention nothing counts is the defect this bead is about", declared0, declared)
	}
	if silent != silent0 {
		t.Errorf("a DECLARED abstention was counted as a silent one (%d -> %d), which would red every run that probes this", silent0, silent)
	}
	// And the verdict over that state says so without redding the run.
	if code, line := wallUngrantedVerdict(0, 0, declared, "the reason"); code != 0 || !strings.Contains(line, ungrantedRootDark) {
		t.Errorf("a declared dark run answered exit %d (%s)", code, line)
	}
}
