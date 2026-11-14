//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-41l0r — the bd argv gate copy arm of `posse upgrade`
// (ADR 0092 D5) and D6's gate word.
//
// WHAT THIS ARM CAN GET WRONG IN SILENCE, which is the only kind of wrong
// worth pinning in a verb that writes a fence every Bash call on the box
// runs through:
//
//	the destination  it is the file the operator's PreToolUse hook REACHES
//	                 and never a path this tree guesses, so a hook that
//	                 names nothing, names two things, names a path with no
//	                 gate at it, or names a symlink is a refusal that writes
//	                 nothing and prints the settings edit.
//	the write        only when the installed copy differs, both files kept
//	                 as .prev first, each swapped by rename.
//	the judgement    the three behaviour arms, against the file just
//	                 written; a red arm puts BOTH .prev copies back, and a
//	                 reading that measured nothing is a word and not a
//	                 rollback.
//	the blast line   the arm never fails the run. The binary is already
//	                 installed and already judged by D4 when this starts, so
//	                 a gate copy that could not be refreshed is a word on
//	                 D6's last line and not an exit code.
//
// THE BEHAVIOUR READING IS STUBBED WITH A REAL PROBE, not with a canned
// error. gateDenyArm below is verify-gate-freshness's deny arm, in the test:
// it hands the INSTALLED wrapper the payload the script hands it and insists
// the refusal came from the parser. So "a stub parser that fails the deny
// arm rolls both files back" is pinned as that sentence reads — the planted
// source gate really is a broken parser, the probe really asks it, and the
// rollback really follows. That the REAL script answers those three arms
// through the flags this file's seams pass is the other half, and it is
// pinned where the script can be run against a planted gate:
// internal/treepins/gatearmmodes_qa_test.go.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the fixture ─────────────────────────────────────────────────────────────

// gateWorking is a gate that refuses `bd daemon` through the parser's own
// contract — rc 0 with a deny decision on stdout — and stays out of the way
// of everything else. `$1` is a marker so two generations of it differ in
// bytes while behaving identically, which is what the "differs" arm needs.
func gateWorking(marker string) string {
	return "#!/bin/sh\n" +
		"# gate generation " + marker + "\n" +
		"payload=$(cat)\n" +
		"case \"$payload\" in\n" +
		"  *'bd daemon'*) printf '{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\"}}' ;;\n" +
		"esac\n" +
		"exit 0\n"
}

// gateOpen is the failure the deny arm exists to catch: a gate that answers
// every call with "nothing to see", so the fence is open. Byte-different
// from gateWorking, so it is also a copy the arm must want to install.
const gateOpen = "#!/bin/sh\n# the fence is open\ncat >/dev/null\nexit 0\n"

type gateFixture struct {
	wt   string // <sha>'s worktree: scripts/bd-argv-gate.{sh,py}
	dir  string // the installed gate dir, where the hook points
	plan upgradePlan

	probed int // how many times the behaviour reading was taken
}

// newGateFixture plants <sha>'s pair in a worktree and the operator's pair
// in a gate dir, and points the resolution seam at that dir.
func newGateFixture(t *testing.T, srcWrapper, srcParser, instWrapper, instParser string) *gateFixture {
	t.Helper()
	f := &gateFixture{wt: t.TempDir(), dir: t.TempDir()}
	f.plan = upgradePlan{
		Repo:   "/src/posse",
		Target: filepath.Join(t.TempDir(), "posse"),
		To:     "0123456789abcdef0123456789abcdef01234567",
	}
	if err := os.MkdirAll(filepath.Join(f.wt, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	plant := func(path, body string) {
		t.Helper()
		if body == "" {
			return
		}
		if err := WriteExecutable(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	plant(filepath.Join(f.wt, "scripts", upgradeGatePair[0]), srcWrapper)
	plant(filepath.Join(f.wt, "scripts", upgradeGatePair[1]), srcParser)
	plant(f.wrapper(), instWrapper)
	plant(f.parser(), instParser)
	return f
}

func (f *gateFixture) wrapper() string { return filepath.Join(f.dir, upgradeGatePair[0]) }
func (f *gateFixture) parser() string  { return filepath.Join(f.dir, upgradeGatePair[1]) }

// stub points the two seams at this fixture for one test: the resolution
// answers the planted gate dir, and the behaviour reading is the deny arm
// against whatever is installed at the moment it is taken.
func (f *gateFixture) stub(t *testing.T, resolve func() ([]string, []string, string, error)) {
	t.Helper()
	savedW, savedB := upgradeGateWrappers, upgradeGateBehaviour
	t.Cleanup(func() { upgradeGateWrappers, upgradeGateBehaviour = savedW, savedB })
	if resolve == nil {
		resolve = func() ([]string, []string, string, error) {
			return []string{f.wrapper()}, nil, f.wrapper(), nil
		}
	}
	upgradeGateWrappers = func(string) ([]string, []string, string, error) { return resolve() }
	upgradeGateBehaviour = func(w io.Writer, wt, wrapper string) error {
		f.probed++
		fmt.Fprintf(w, "  (deny arm against %s)\n", wrapper)
		return gateDenyArm(t, wrapper)
	}
}

// gateDenyArm is the one behaviour arm that distinguishes a working gate
// from one that is only failing closed: `bd daemon stop` must come back rc 0
// with a deny decision on stdout, which only the PARSER can produce. A
// finding is reported the way the script reports it — exit 1 — because that
// is the number the arm reads.
func gateDenyArm(t *testing.T, wrapper string) error {
	t.Helper()
	cmd := exec.Command(wrapper)
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"bd daemon stop"}}`)
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), "permissionDecision") || !strings.Contains(string(out), "deny") {
		return exitErr(t, 1)
	}
	return nil
}

// reads the two installed files, plus their .prev copies.
func (f *gateFixture) read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

func (f *gateFixture) absent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("%s exists, and this arm wrote nothing", path)
	}
}

// ─── the write ───────────────────────────────────────────────────────────────

func TestUpgradeGateArmInstallsTheCommitsPairWhenTheInstalledCopyDiffers(t *testing.T) {
	src, parser := gateWorking("new"), "parser new\n"
	f := newGateFixture(t, src, parser, gateWorking("old"), "parser old\n")
	f.stub(t, nil)

	var out bytes.Buffer
	if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateInstalled {
		t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateInstalled, out.String())
	}
	if got := f.read(t, f.wrapper()); got != src {
		t.Errorf("the installed wrapper is not the commit's:\n%q", got)
	}
	if got := f.read(t, f.parser()); got != parser {
		t.Errorf("the installed parser is not the commit's:\n%q", got)
	}
	// BOTH .prev copies, which is what makes the rollback a rollback to a
	// combination that actually ran.
	if got := f.read(t, upgradePrevPath(f.wrapper())); got != gateWorking("old") {
		t.Errorf("%s does not hold the wrapper that was installed:\n%q", upgradePrevPath(f.wrapper()), got)
	}
	if got := f.read(t, upgradePrevPath(f.parser())); got != "parser old\n" {
		t.Errorf("%s does not hold the parser that was installed:\n%q", upgradePrevPath(f.parser()), got)
	}
	// The hook execs the wrapper directly, with no interpreter in front, so
	// a copy that is not executable is a box with no gate.
	for _, p := range []string{f.wrapper(), f.parser()} {
		if st, err := os.Stat(p); err != nil || st.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable after the swap: %v %v", p, st, err)
		}
	}
	// rename(2) moves the staging file, so a leftover means something else
	// wrote it.
	f.absent(t, f.wrapper()+".new")
	f.absent(t, f.parser()+".new")
	if f.probed != 1 {
		t.Errorf("the behaviour arms were taken %d times, want once — `installed` is a word that claims they were green", f.probed)
	}
	// The rollback commands are printed BEFORE the swap they might have to
	// undo, for the reason D4 prints the binary's where it does: the reader
	// who needs them most is the one whose next bd call is the one that
	// broke.
	roll := strings.Index(out.String(), upgradeRollbackCmd(f.wrapper()))
	inst := strings.Index(out.String(), "installed: ")
	if roll < 0 {
		t.Errorf("the report carries no rollback command for the wrapper:\n%s", out.String())
	} else if inst < 0 || roll > inst {
		t.Errorf("the rollback lines are not printed before the swap (rollback at %d, installed at %d)", roll, inst)
	}
	if !strings.Contains(out.String(), upgradeRollbackCmd(f.parser())) {
		t.Errorf("the report carries no rollback command for the parser — half a pair is a combination nobody ran:\n%s", out.String())
	}
}

// ADR 0092 D5: "only when the installed copy differs". An arm that rewrites
// a copy it did not need to rewrite is an arm that gives every upgrade a new
// inode for the file the fleet's PreToolUse hook execs, for nothing.
func TestUpgradeGateArmWritesNothingWhenTheInstalledCopyIsAlreadyTheCommits(t *testing.T) {
	src, parser := gateWorking("same"), "parser same\n"
	f := newGateFixture(t, src, parser, src, parser)
	f.stub(t, nil)

	before, err := os.Stat(f.wrapper())
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateUnchanged {
		t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateUnchanged, out.String())
	}
	after, err := os.Stat(f.wrapper())
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("the wrapper was rewritten (%v → %v) although its bytes already matched the commit's", before.ModTime(), after.ModTime())
	}
	f.absent(t, upgradePrevPath(f.wrapper()))
	f.absent(t, upgradePrevPath(f.parser()))
	f.absent(t, f.wrapper()+".new")
	// `unchanged` still CLAIMS the arms were green (ADR 0092 D6), so they
	// have to have been taken.
	if f.probed != 1 {
		t.Errorf("the behaviour arms were taken %d times, want once — an identical copy that does not work is not `unchanged`", f.probed)
	}
	if !strings.Contains(out.String(), "nothing written") {
		t.Errorf("the report does not say nothing was written:\n%s", out.String())
	}
}

// A copy that is already the commit's and does NOT behave is broken under
// its bytes — no python3, a bad mode, a settings entry pointing at something
// unexpected. There is nothing of this run's to put back, so the word says
// the arm could not certify the gate rather than claiming it was green.
func TestUpgradeGateArmRefusesAnIdenticalCopyThatDoesNotBehave(t *testing.T) {
	f := newGateFixture(t, gateOpen, "parser\n", gateOpen, "parser\n")
	f.stub(t, nil)

	var out bytes.Buffer
	if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateRefused {
		t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateRefused, out.String())
	}
	f.absent(t, upgradePrevPath(f.wrapper()))
	if got := f.read(t, f.wrapper()); got != gateOpen {
		t.Errorf("the installed wrapper changed although nothing was written:\n%q", got)
	}
}

// ─── the judgement ───────────────────────────────────────────────────────────

// The bead's second test, as its sentence reads: a stub parser that fails
// the deny arm rolls both files back.
func TestUpgradeGateArmRestoresBothFilesWhenTheDenyArmGoesRed(t *testing.T) {
	live, liveParser := gateWorking("live"), "parser live\n"
	f := newGateFixture(t, gateOpen, "parser broken\n", live, liveParser)
	f.stub(t, nil)

	var out bytes.Buffer
	if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateRefused {
		t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateRefused, out.String())
	}
	// BOTH, not just the wrapper whose behaviour was measured: the pair is
	// one fence.
	if got := f.read(t, f.wrapper()); got != live {
		t.Errorf("the wrapper was left as this run installed it — the fence this box was running is gone:\n%q", got)
	}
	if got := f.read(t, f.parser()); got != liveParser {
		t.Errorf("the parser was left as this run installed it, beside a restored wrapper — that is a combination nobody ran:\n%q", got)
	}
	// And the restored pair still behaves, which is the whole point of
	// putting it back.
	if err := gateDenyArm(t, f.wrapper()); err != nil {
		t.Errorf("the restored gate does not refuse `bd daemon stop`: %v", err)
	}
	// .prev is LEFT WHERE IT IS, as upgradeRollback leaves the binary's, so
	// the commands this report printed stay true for whoever reads it.
	if got := f.read(t, upgradePrevPath(f.wrapper())); got != live {
		t.Errorf("%s no longer holds the wrapper that was live:\n%q", upgradePrevPath(f.wrapper()), got)
	}
	f.absent(t, f.wrapper()+".new")
	f.absent(t, f.parser()+".new")
	if !strings.Contains(out.String(), "restoring") {
		t.Errorf("the report does not say the gate was restored:\n%s", out.String())
	}
}

// A reading that measured nothing is not evidence of a break, and the paths
// that answer 2 are box properties the installed bytes cannot cause — no
// python3 for the wrapper to run, a signalled probe. Rolling back would buy
// nothing and would hide, behind a word that means "I measured nothing",
// which copy is in place.
func TestUpgradeGateArmLeavesTheInstallStandingWhenTheArmsMeasureNothing(t *testing.T) {
	src := gateWorking("new")
	f := newGateFixture(t, src, "parser new\n", gateWorking("old"), "parser old\n")
	f.stub(t, nil)
	savedB := upgradeGateBehaviour
	t.Cleanup(func() { upgradeGateBehaviour = savedB })
	upgradeGateBehaviour = func(w io.Writer, wt, wrapper string) error { return exitErr(t, 2) }

	var out bytes.Buffer
	if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateUnknown {
		t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateUnknown, out.String())
	}
	if got := f.read(t, f.wrapper()); got != src {
		t.Errorf("the arm rolled back on a reading that measured nothing:\n%q", got)
	}
	if !strings.Contains(out.String(), "measured nothing") {
		t.Errorf("the report does not say the arms measured nothing:\n%s", out.String())
	}
	// And the report still names the way back, because the operator reading
	// `unknown` is the one who may need it.
	if !strings.Contains(out.String(), upgradeRollbackCmd(f.wrapper())) {
		t.Errorf("the report carries no rollback command:\n%s", out.String())
	}
}

// ─── the destination ─────────────────────────────────────────────────────────

// The bead's third test: an unresolvable hook path refuses the arm only.
// Every arm of this table writes nothing at all, and each of them is a shape
// the operator's settings can really be in.
func TestUpgradeGateArmRefusesEveryHookPathItCannotRefresh(t *testing.T) {
	for _, c := range []struct {
		name    string
		resolve func(f *gateFixture) func() ([]string, []string, string, error)
		says    string
	}{{
		name: "no PreToolUse hook names the wrapper",
		resolve: func(f *gateFixture) func() ([]string, []string, string, error) {
			return func() ([]string, []string, string, error) {
				return nil, nil, "verify-gate-freshness: no PreToolUse hook in /c names bd-argv-gate.sh — nothing measured", exitErr(t, 2)
			}
		},
		says: "did not resolve",
	}, {
		name: "the hook command names a relative path",
		resolve: func(f *gateFixture) func() ([]string, []string, string, error) {
			return func() ([]string, []string, string, error) {
				return nil, []string{"gate/bd-argv-gate.sh"}, "gate/bd-argv-gate.sh", nil
			}
		},
		says: "not an absolute path",
	}, {
		name: "the settings name two wrappers",
		resolve: func(f *gateFixture) func() ([]string, []string, string, error) {
			return func() ([]string, []string, string, error) {
				return []string{f.wrapper(), "/elsewhere/bd-argv-gate.sh"}, nil, "", nil
			}
		},
		says: "will not choose between them",
	}} {
		t.Run(c.name, func(t *testing.T) {
			f := newGateFixture(t, gateWorking("new"), "parser new\n", gateWorking("old"), "parser old\n")
			f.stub(t, c.resolve(f))
			var out bytes.Buffer
			if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateRefused {
				t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateRefused, out.String())
			}
			if !strings.Contains(out.String(), c.says) {
				t.Errorf("the refusal does not say %q:\n%s", c.says, out.String())
			}
			// NOTHING WRITTEN, and the gate the box was running is the gate
			// it keeps.
			if got := f.read(t, f.wrapper()); got != gateWorking("old") {
				t.Errorf("the installed wrapper changed on a refusal:\n%q", got)
			}
			f.absent(t, upgradePrevPath(f.wrapper()))
			f.absent(t, upgradePrevPath(f.parser()))
			if f.probed != 0 {
				t.Errorf("the behaviour arms were taken %d times on a refusal that resolved no destination", f.probed)
			}
			// Every refusal hands over the settings edit, because the thing
			// that has to change is a file this verb will never write.
			for _, want := range []string{"PreToolUse", "install -m 0755", "writes no settings"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("the refusal does not name %q — the operator cannot act on it:\n%s", want, out.String())
				}
			}
			// And it names no gate directory of its own invention: the
			// resolution is the whole of how this arm knows where to write.
			if strings.Contains(out.String(), ".config/posse/gate") {
				t.Errorf("the refusal names this box's own gate dir, which it did not resolve:\n%s", out.String())
			}
		})
	}
}

// A hook that names a path with nothing at it is D5's "points somewhere
// new": every Bash call on this box is running unfenced, and which directory
// the gate belongs in is a settings question with an operator's answer. An
// arm that answered it would be choosing which calls on this box are
// fenced.
func TestUpgradeGateArmRefusesAHookPathWithNoGateAtIt(t *testing.T) {
	f := newGateFixture(t, gateWorking("new"), "parser new\n", "", "")
	f.stub(t, nil)

	var out bytes.Buffer
	if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateRefused {
		t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateRefused, out.String())
	}
	f.absent(t, f.wrapper())
	f.absent(t, f.parser())
	if !strings.Contains(out.String(), "settings edit") {
		t.Errorf("the refusal does not say this is a settings edit:\n%s", out.String())
	}
	// It knows the directory here, so the install line it prints names it
	// rather than a placeholder.
	if !strings.Contains(out.String(), AbbrevHome(f.dir)) {
		t.Errorf("the refusal does not name the directory the hook points at:\n%s", out.String())
	}
}

// A symlink is somebody's deliberate arrangement — most plausibly into a
// checkout — and a write-beside-then-rename over it replaces that
// arrangement with a copy, silently.
func TestUpgradeGateArmRefusesASymlinkedWrapper(t *testing.T) {
	f := newGateFixture(t, gateWorking("new"), "parser new\n", "", "parser old\n")
	real := filepath.Join(t.TempDir(), "bd-argv-gate.sh")
	if err := WriteExecutable(real, []byte(gateWorking("linked")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, f.wrapper()); err != nil {
		t.Skipf("this filesystem does not take symlinks: %v", err)
	}
	f.stub(t, nil)

	var out bytes.Buffer
	if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateRefused {
		t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateRefused, out.String())
	}
	if got := f.read(t, real); got != gateWorking("linked") {
		t.Errorf("the file the symlink points at was rewritten:\n%q", got)
	}
	st, err := os.Lstat(f.wrapper())
	if err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the wrapper is no longer a symlink: %v %v", st, err)
	}
	f.absent(t, upgradePrevPath(f.wrapper()))
	// The refusal SAYS symlink. Without that arm the not-a-regular-file
	// case catches this one too and the box is equally safe — what is lost
	// is the only thing the operator needs from it, which is why the
	// arrangement they made was not refreshed. MEASURED by mutation
	// 2026-11-14: deleting the symlink arm leaves every other claim in this
	// pin green.
	if !strings.Contains(out.String(), "symlink") {
		t.Errorf("the refusal does not say the wrapper is a symlink, so it reads as a broken gate dir:\n%s", out.String())
	}
}

// A commit whose tree is missing either file is not a commit this arm can
// install from, and saying so beats writing half a gate.
func TestUpgradeGateArmRefusesWhenTheCommitCarriesNoPair(t *testing.T) {
	f := newGateFixture(t, gateWorking("new"), "", gateWorking("old"), "parser old\n")
	f.stub(t, nil)

	var out bytes.Buffer
	if got := upgradeGateArm(&out, f.wt, f.plan); got != upgradeGateRefused {
		t.Fatalf("gate word = %q, want %q:\n%s", got, upgradeGateRefused, out.String())
	}
	if got := f.read(t, f.wrapper()); got != gateWorking("old") {
		t.Errorf("the installed wrapper changed although the commit carries no pair to install:\n%q", got)
	}
	f.absent(t, upgradePrevPath(f.wrapper()))
}

// ─── the seams, as the arm spells them ───────────────────────────────────────

// The resolution reads PATHS off stdout and must not read the script's own
// "nothing measured" line as one — that line is its channel for exactly the
// case this arm has to distinguish, and taking it for a destination would
// turn a refusal into a write.
// NO t.Parallel: this one reads the package var upgradeGateWrappers, which
// every arm pin above swaps for its own fixture (runningPosse's own header,
// applied to the test side).
func TestUpgradeGateWrappersReadsPathsAndNotTheVerifiersProse(t *testing.T) {
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A stand-in verifier that prints what the real one prints in both
	// shapes: one absolute path, and the prose of a box with no gate.
	script := filepath.Join(wt, "scripts", "verify-gate-freshness.sh")
	if err := WriteExecutable(script, []byte("#!/bin/sh\n"+
		"echo /opt/gate/bd-argv-gate.sh\n"+
		"echo 'verify-gate-freshness: no PreToolUse hook in /c names bd-argv-gate.sh (the gate is not installed on this box) — nothing measured'\n"+
		"echo relative/bd-argv-gate.sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	abs, rel, out, err := upgradeGateWrappers(wt)
	if err != nil {
		t.Fatalf("upgradeGateWrappers: %v (%s)", err, out)
	}
	if len(abs) != 1 || abs[0] != "/opt/gate/bd-argv-gate.sh" {
		t.Errorf("absolute wrappers = %v, want just /opt/gate/bd-argv-gate.sh — the prose line names the wrapper too", abs)
	}
	if len(rel) != 1 || rel[0] != "relative/bd-argv-gate.sh" {
		t.Errorf("relative wrappers = %v, want the one line that is not absolute", rel)
	}
	if !strings.Contains(out, "nothing measured") {
		t.Errorf("the verifier's own words did not reach the report: %q", out)
	}
}

// The flags are the contract with the script, and the argv is where a
// rename of either mode goes wrong in silence: the arm would read exit 2
// (the script's usage answer) as `unknown` for ever, on every install.
// internal/treepins runs the real script with these same flags; this pins
// that these are the flags the arm passes.
func TestUpgradeGateArmPassesTheVerifiersTwoReadOnlyModes(t *testing.T) {
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(wt, "argv.log")
	script := filepath.Join(wt, "scripts", "verify-gate-freshness.sh")
	if err := WriteExecutable(script, []byte("#!/bin/sh\necho \"$@\" >> "+log+"\necho /opt/gate/bd-argv-gate.sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := upgradeGateWrappers(wt); err != nil {
		t.Fatalf("upgradeGateWrappers: %v", err)
	}
	if err := upgradeGateBehaviour(io.Discard, wt, "/opt/gate/bd-argv-gate.sh"); err != nil {
		t.Fatalf("upgradeGateBehaviour: %v", err)
	}
	body, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"--print-wrappers",
		"--behaviour-only --wrapper /opt/gate/bd-argv-gate.sh",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the arm never passed %q — it passed:\n%s", want, body)
		}
	}
	// And NOT --warn, which exits 0 whatever it found and would leave the
	// judgement with nothing to read.
	if strings.Contains(string(body), "--warn") {
		t.Errorf("the arm passes --warn, which exits 0 on a finding:\n%s", body)
	}
}

// ─── the wiring: D6's last line, and the binary arm is unaffected ────────────

// The arm reaches the run through upgradeEpilogue, and its word is the gate
// field of the line the delegate posts on the install bead. So the thing
// pinned here is the WIRING: a refused gate arm leaves the upgrade green and
// says so on that line. Nothing else in the suite reaches upgradeGateArm
// from upgradeRunPlan, and without this pin the whole arm could be deleted
// with every test above green (upgradeReads' header, for the three
// guardrails that were deletable that way).
func TestUpgradeRunPlanCarriesTheGateArmsWordAndIsNotFailedByIt(t *testing.T) {
	f := newUpgradeFixture(t)
	stubUpgradeProbe(t)
	built := fakePosse{Stamp: f.c}
	stubUpgradeSteps(t,
		func(w io.Writer, dir string) error { return nil },
		func(w io.Writer, dir, repo, out string) error { return os.WriteFile(out, built.bytes(), 0o644) },
		upgradeEpilogue) // the REAL epilogue, so the gate arm is reached

	// The hook-freshness half of the epilogue runs a script the throwaway
	// worktree does not hold; it is advisory and its error is swallowed,
	// which is the behaviour this pin wants from it.
	savedW, savedB := upgradeGateWrappers, upgradeGateBehaviour
	t.Cleanup(func() { upgradeGateWrappers, upgradeGateBehaviour = savedW, savedB })
	upgradeGateWrappers = func(string) ([]string, []string, string, error) {
		return nil, nil, "verify-gate-freshness: no PreToolUse hook names bd-argv-gate.sh — nothing measured", errors.New("exit status 2")
	}
	upgradeGateBehaviour = func(w io.Writer, wt, wrapper string) error {
		t.Error("the behaviour arms were taken although no destination resolved")
		return nil
	}

	plan, err := upgradeResolve(f.repo, f.target, f.a, "")
	if err != nil {
		t.Fatalf("upgradeResolve: %v", err)
	}
	plan.Repo = f.repo
	var out bytes.Buffer
	if err := upgradeRunPlan(&out, plan); err != nil {
		t.Fatalf("a refused gate arm failed the upgrade: %v\n%s", err, out.String())
	}
	if body, _ := os.ReadFile(f.target); !bytes.Equal(body, built.bytes()) {
		t.Errorf("the binary arm did not stand: the target holds %q", body)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "· gate "+upgradeGateRefused+" ·") {
		t.Errorf("D6's last line does not carry the arm's word:\n  %q", last)
	}
}
