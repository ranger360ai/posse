package rhq

// QA pins for the silent-revert detector (scripts/audit-silent-reverts.sh,
// rangerhq-8rtf, verified under rangerhq-jkhb).
//
// Four claims:
//
//   1. The detector's own --self-test proves the detector FIRES, which is the
//      only thing that separates "the audit ran" from "the audit works". But
//      `make test` runs the script with --quiet only, so --self-test had no
//      trigger — it was a line in the Makefile comment, i.e. a thing to
//      remember, which is the objection rangerhq-2f5r raised in the first
//      place. It runs here now.
//
//   2. The detector covers the ADD-ONLY half of its own mechanism
//      (rangerhq-ypn1, fixed): when the change landed from a private index
//      consisted of NEW files, the shared index has no entry for them, so the
//      next commit from that index DELETES them rather than rolling content
//      back. The scan used to skip deletions, so that half scored clean and
//      exited 0. It now treats absence as a state a path can be rolled back
//      to, and both halves flag. This test is what says so.
//
//   3. The harness that proves 1 and 2 is itself proof against a dead fixture
//      (ranger-base-z4vx). Its rig guard was dead code — errexit is suppressed
//      for the left operand of `||` — and its NEGATIVE control asserted an
//      ABSENCE and nothing else, so the control reported a pass over a fixture
//      that was never built. The guard takes the plant's status on its own
//      line now, and every arm demands a positive witness that it scanned the
//      commits the plant means to build. The last two tests here are the two
//      escapes, one each.
//
//   4. The move exception asks git for rename similarity rather than exact blob
//      identity (ranger-base-en75), at a threshold that is chosen and written
//      down rather than inherited. That is a false-NEGATIVE widening, so the
//      four pins at the end of this file are one per mutation, and no two of
//      them red the same self-test arm.
//
// Self-contained on purpose (own helpers, own fixture): they must survive
// whatever the next persona does to the script's neighbours.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// srScript resolves the audit script, skipping if this checkout does not carry
// it (a tarball, a worktree pruned for a build).
func srScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../scripts/audit-silent-reverts.sh")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("audit script not present: %v", err)
	}
	return p
}

// srEnv is the caller's environment with the two variables that would make
// these tests depend on WHO runs them removed: RHQ_PERSONA arms the commit
// wall, and an inherited GIT_INDEX_FILE would point the fixture's git at
// somebody else's index.
func srEnv() []string {
	var e []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RHQ_PERSONA=") || strings.HasPrefix(kv, "GIT_INDEX_FILE=") {
			continue
		}
		e = append(e, kv)
	}
	return append(e,
		"GIT_AUTHOR_NAME=qa", "GIT_AUTHOR_EMAIL=qa@t",
		"GIT_COMMITTER_NAME=qa", "GIT_COMMITTER_EMAIL=qa@t")
}

// srGit runs one git command in dir and fails the test if it does not.
// extra is appended to the environment (this is how the private-index commit
// is spelled).
func srGit(t *testing.T, dir string, extra []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(srEnv(), extra...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// srAudit runs the audit script with cwd=dir and returns its output and exit
// code. The script cds to the toplevel of whatever repo dir sits in, so dir
// selects the repo under audit.
func srAudit(t *testing.T, script, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(script, args...)
	cmd.Dir = dir
	cmd.Env = srEnv()
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s: %v\n%s", script, err, out)
	}
	return string(out), code
}

// TestSilentRevertSelfTestStillFires gives --self-test a trigger. A clean
// `scripts/audit-silent-reverts.sh --quiet` in `make test` proves only that the
// script ran; this proves the detector still flags the rangerhq-8rtf mechanism
// when it is planted in front of it.
func TestSilentRevertSelfTestStillFires(t *testing.T) {
	script := srScript(t)
	if err := exec.Command("git", "rev-parse", "--show-toplevel").Run(); err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	out, code := srAudit(t, script, ".", "--self-test")
	if code != 0 || !strings.Contains(out, "self-test PASS") {
		t.Fatalf("detector self-test did not fire: exit %d\n%s", code, out)
	}
}

// srPlantAddOnlyRevert builds rangerhq-8rtf's mechanism in a throwaway repo,
// with one difference from the incident: the change that lands is a NEW file
// rather than an edit to an existing one. Returns the repo path.
func srPlantAddOnlyRevert(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	srGit(t, repo, nil, "init", "-q", ".")
	write("existing.go", "v1\n")
	write("other.txt", "o\n")
	srGit(t, repo, nil, "add", "-A")
	srGit(t, repo, nil, "commit", "-qm", "base")

	// The fix: one new file, landed from a PRIVATE index. HEAD gets it; the
	// shared .git/index never hears about it.
	write("newpin_test.go", "package x // the regression pin\n")
	priv := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	srGit(t, repo, priv, "read-tree", "HEAD")
	srGit(t, repo, priv, "add", "--", "newpin_test.go")
	srGit(t, repo, priv, "commit", "-qm", "the fix: add newpin_test.go")

	// The next commit taken from the shared index — a bd sync. It writes a
	// tree with no newpin_test.go in it, so the fix is deleted, silently.
	write("other.txt", "synced\n")
	srGit(t, repo, nil, "add", "other.txt")
	srGit(t, repo, nil, "commit", "-qm", "bd sync: batch")

	tree := exec.Command("git", "ls-tree", "--name-only", "HEAD")
	tree.Dir = repo
	tree.Env = srEnv()
	out, err := tree.CombinedOutput()
	if err != nil {
		t.Fatalf("ls-tree: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "newpin_test.go") {
		t.Fatalf("rig did not reproduce the revert; HEAD still has the pin:\n%s", out)
	}
	return repo
}

// TestAuditFlagsAddOnlySilentRevert is rangerhq-ypn1. The commit under test
// undid a landed change from the stale shared index, which is precisely the
// class the audit exists for — but because the landed change was an ADD, the
// undo is a deletion, and scan() used to skip deletions on the rationale that
// "a removal is visible in review". rangerhq-8rtf is the disproof of that
// rationale: nobody reviewed dcca7b5. Unskipped when ypn1 closed; it fails
// again the moment the deletion rule is weakened.
func TestAuditFlagsAddOnlySilentRevert(t *testing.T) {
	script := srScript(t)
	repo := srPlantAddOnlyRevert(t)
	out, code := srAudit(t, script, repo, "HEAD")
	if code != 1 {
		t.Fatalf("add-only silent revert not flagged: exit %d, want 1\n%s", code, out)
	}
}

// TestAuditFlagsAddOnlySilentRevertIsStillTheMechanism guards the FIXTURE, not
// the defect, so the pin above cannot rot into a claim about a rig that stopped
// reproducing: if the plant ever stops building the three commits it means to
// build, the pin above would pass for the wrong reason and this one fails.
func TestAuditFlagsAddOnlySilentRevertIsStillTheMechanism(t *testing.T) {
	script := srScript(t)
	repo := srPlantAddOnlyRevert(t)
	out, _ := srAudit(t, script, repo, "HEAD")
	if !strings.Contains(out, "scanned 3 commits") {
		t.Fatalf("fixture no longer scans the 3 planted commits:\n%s", out)
	}
}

// srDeadMoveRig writes a copy of the audit script whose move rig — the one the
// NEGATIVE control reads — has been mutated to build nothing, and returns its
// path together with the (non-repo) directory to run it from. inject is the
// line spliced in at the top of plant_move: `return 2` is a rig that FAILS,
// `return 0` is a rig that reports success and plants nothing. Those are two
// different escapes and self_test() answers them with two different guards.
func srDeadMoveRig(t *testing.T, inject string) (mutant, dir string) {
	t.Helper()
	script := srScript(t)
	src, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("read %s: %v", script, err)
	}
	// Guard the mutation itself: if the function is renamed, these pins must
	// say so rather than quietly measure nothing.
	const head = "plant_move() {\n"
	if !strings.Contains(string(src), head) {
		t.Fatalf("plant_move() not found in %s; this pin's mutation no longer applies", script)
	}
	broken := strings.Replace(string(src), head, head+inject, 1)

	dir = t.TempDir()
	mutant = filepath.Join(dir, "audit-mutant.sh")
	if err := os.WriteFile(mutant, []byte(broken), 0o755); err != nil {
		t.Fatalf("write mutant: %v", err)
	}
	return mutant, dir
}

// TestSilentRevertSelfTestRigFailureIsNotAPass is ranger-base-z4vx. The
// self-test's three arms all depend on a fixture the script plants first, and
// the harness that was supposed to notice a plant that did not build —
//
//	( set -e; "plant_$shape" >/dev/null 2>&1; pwd > "$d/$shape" ) || {
//	    echo "self-test: $shape rig did not reproduce the mechanism"; return 2; }
//
// — could not fire: errexit is suppressed for the left operand of `||` and the
// suppression is inherited into the subshell, so a plant returning non-zero
// did not abort it and `pwd` wrote the script's own toplevel as the fixture
// path. The two positive arms fail safe (they want n>=1 and get 0). The
// NEGATIVE control does not: it wants n==0, which a repo nobody planted
// satisfies, so it reported "a plain move is not flagged" having looked at
// nothing — the exact class it was added to prevent.
//
// This plants that failure (the move rig FAILS) and asserts the harness does
// not answer with a pass. It is not hypothetical: any deny rule, sandbox or
// git change that breaks `git commit -qm` inside plant_repo lands here — a
// persona whose gate refuses `git commit` without `--` sees all three rigs
// build nothing.
func TestSilentRevertSelfTestRigFailureIsNotAPass(t *testing.T) {
	mutant, dir := srDeadMoveRig(t, "  return 2   # ranger-base-z4vx: this rig builds nothing\n")
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with a rig that builds nothing:\n%s", out)
	}
	if strings.Contains(out, "a plain move is not flagged") {
		t.Fatalf("negative control reported a pass for a fixture that was never built:\n%s", out)
	}
}

// TestSilentRevertNegativeControlHasAPositiveWitness is the other half of
// ranger-base-z4vx, and it is the half a working rig guard does NOT cover. A
// plant can report success and still plant nothing — the class
// TestAuditFlagsAddOnlySilentRevertIsStillTheMechanism guards for the Go
// fixture. No exit status catches that, so the control cannot rest on an
// absence alone: it has to show it looked at something. self_test() makes
// every arm demand the scan report the commit count the plant means to build,
// and this pin is what says so. Mutating the move rig to `return 0` is a false
// PASS with exit 0 before that witness and a FAIL with exit 1 after — measured
// both directions.
func TestSilentRevertNegativeControlHasAPositiveWitness(t *testing.T) {
	mutant, dir := srDeadMoveRig(t, "  return 0   # ranger-base-z4vx: reports success, plants nothing\n")
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with a rig that reported success and planted nothing:\n%s", out)
	}
	if strings.Contains(out, "a plain move is not flagged") {
		t.Fatalf("negative control reported a pass without a witness that it scanned anything:\n%s", out)
	}
}

// --- ranger-base-hhcu: the detector's verdict must not depend on the awk -----
//
// ci.yml gave two answers for the same 422 commits on the same tree:
// ubuntu-latest flagged b26975f (cmd/posse/cockpit.go "-> content of 1fdf9da"),
// macos-latest did not, and the linux verdict was the wrong one — the three
// states of that path are three distinct blobs. Two of them ABBREVIATE to
// `6e51571` and `6e44262`, both valid scientific notation; a field, and every
// element split out of one, is a STRNUM, and awk compares two strnums
// numerically. Both overflow to +inf, so a coercing awk called them equal.
// Measured 2026-08-29 over this repo's own history: gawk 5.3.2 flags b26975f,
// mawk 1.3.4, busybox awk and darwin's BWK awk 20200816 do not — ubuntu-latest
// runs gawk, and that is the entire split. It matters in the other direction
// too: the same coercion can HIDE a real rollback, so on a coercing awk a clean
// run was worth less than it read.
//
// The script answers it in two independent layers — raw_log's --no-abbrev and
// the `""` in states_awk's capture — so ONE outcome assertion would be green
// with either one gone. These three pins are one per layer plus the arm that
// carries them, because an arm nothing pins is an arm that can be deleted.

// srMutateScript writes a copy of the audit script with old replaced by new,
// and returns it with a (non-repo) directory to run it from. It fails if old
// is not present, so a rename cannot turn these pins into no-ops.
func srMutateScript(t *testing.T, old, new string) (mutant, dir string) {
	t.Helper()
	script := srScript(t)
	src, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("read %s: %v", script, err)
	}
	if strings.Count(string(src), old) != 1 {
		t.Fatalf("mutation target %q appears %d times in %s, want 1; this pin no longer applies",
			old, strings.Count(string(src), old), script)
	}
	dir = t.TempDir()
	mutant = filepath.Join(dir, "audit-mutant.sh")
	if err := os.WriteFile(mutant, []byte(strings.Replace(string(src), old, new, 1)), 0o755); err != nil {
		t.Fatalf("write mutant: %v", err)
	}
	return mutant, dir
}

// TestSilentRevertSelfTestHasTheStrnumArm pins the arm itself. Deleting it
// leaves every other test in this file green, because they all read an exit
// status the deleted arm no longer contributes to.
func TestSilentRevertSelfTestHasTheStrnumArm(t *testing.T) {
	script := srScript(t)
	if err := exec.Command("git", "rev-parse", "--show-toplevel").Run(); err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	out, code := srAudit(t, script, ".", "--self-test")
	if code != 0 {
		t.Fatalf("self-test failed: exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"self-test PASS: a <digit>e<digits> blob id is not a silent revert",
		"self-test PASS: raw_log emits full 40-hex blob ids",
		"self-test PASS: states_awk compares ids as strings, not numbers",
		"self-test PASS: the strnum rig does fire on a real repeat",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("self-test no longer reports %q:\n%s", want, out)
		}
	}
}

// TestAuditStringComparisonHasItsOwnPin is layer two. Dropping the `""` that
// makes states_awk's captured ids plain strings must red the self-test — and on
// EVERY platform, which is why that arm reads a synthetic stream whose two ids
// are `0000100` and `00001e2`: both are the number 100 to all four awks
// measured, no overflow required, so the pin does not itself depend on running
// under gawk. Fed the fixture's real ids instead, this mutation is invisible on
// darwin, mawk and busybox — the blind spot that let the defect ship.
func TestAuditStringComparisonHasItsOwnPin(t *testing.T) {
	mutant, dir := srMutateScript(t,
		`esrc[ne]=m[3] ""; edst[ne]=m[4] ""; est[ne]=m[5]`,
		`esrc[ne]=m[3]; edst[ne]=m[4]; est[ne]=m[5]`)
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with states_awk comparing blob ids as numbers:\n%s", out)
	}
	if !strings.Contains(out, "states_awk coerced two distinct blob ids to one number") {
		t.Fatalf("the strnum arm did not name the defect it exists for:\n%s", out)
	}
}

// TestAuditFullBlobIdsHaveTheirOwnPin is layer one. git's default abbreviation
// is 7 hex, short enough that a blob id lands on the <digit>e<digits> shape
// about once in 270; at 40 hex the whole id has to cooperate. Dropping
// --no-abbrev must red the self-test on its own.
func TestAuditFullBlobIdsHaveTheirOwnPin(t *testing.T) {
	mutant, dir := srMutateScript(t, "--raw --no-abbrev \\", "--raw \\")
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with raw_log emitting abbreviated blob ids:\n%s", out)
	}
	if !strings.Contains(out, "raw_log emitted") {
		t.Fatalf("the abbreviation arm did not name the defect it exists for:\n%s", out)
	}
}

// --- ranger-base-en75: the move exception asks git, not the blob id ----------
//
// The move exception used to be exact-blob: a deletion was excused only when
// the IDENTICAL blob appeared at another path in the same commit. A rename that
// also EDITS the file is a different blob, so the exception could not see it and
// the deletion half was reported as a silent revert. Three commits in ~460 paid
// a triage line for that (631bda7, e82338c, 2eae58a) and the rate was not
// falling. raw_log now asks git to pair renames at a chosen 50% similarity and
// states_awk decomposes the R line into the two entries it stands for.
//
// This is a false-NEGATIVE widening, so it gets four pins rather than one
// outcome assertion, and they are NOT interchangeable — each mutation below
// reds a DIFFERENT arm, which is the whole reason the self-test grew three arms
// and not just the obvious one:
//
//	--find-renames -> --no-renames    reds the rename-that-edits arm
//	threshold 50% -> 75%              reds the rename-that-edits arm
//	moved=emoved[i] -> moved=0        reds the rename-that-edits arm
//	the R branch -> if (0)            reds the RE-LAND arm, and nothing else
//
// That last row is the escape worth naming. Deleting states_awk's R handling
// makes a rename INVISIBLE — the line parses as one path literally named
// "src<tab>dst" and no deletion is ever recorded — and invisible is
// indistinguishable from excused to an arm that only asserts silence. So the
// rename-that-edits arm stays green over it. The re-land arm is the pin that
// branch actually has: it asserts that the DESTINATION of a rename is still
// compared against every state its path has held.

// TestSilentRevertSelfTestHasTheRenameArms pins the three arms themselves.
// Deleting any of them leaves every other test in this file green, because they
// all read an exit status a deleted arm no longer contributes to.
func TestSilentRevertSelfTestHasTheRenameArms(t *testing.T) {
	script := srScript(t)
	if err := exec.Command("git", "rev-parse", "--show-toplevel").Run(); err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	out, code := srAudit(t, script, ".", "--self-test")
	if code != 0 {
		t.Fatalf("self-test failed: exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"self-test PASS: a rename that also EDITS is not flagged",
		"self-test PASS: a deletion plus an UNRELATED add in one commit still fires",
		"self-test PASS: a re-land through a rename is still caught",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("self-test no longer reports %q:\n%s", want, out)
		}
	}
}

// TestAuditRenameDetectionHasItsOwnPin is the mechanism: raw_log must ask git to
// pair renames at all. Putting --no-renames back is the pre-fix behaviour, and
// it must red the rename-that-edits arm rather than quietly cost another triage
// line.
func TestAuditRenameDetectionHasItsOwnPin(t *testing.T) {
	mutant, dir := srMutateScript(t,
		"--find-renames=50% -l0 --raw --no-abbrev",
		"--no-renames --raw --no-abbrev")
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with raw_log asking git for no renames:\n%s", out)
	}
	if !strings.Contains(out, "a rename that also edits was flagged as a silent revert") {
		t.Fatalf("the rename arm did not name the defect it exists for:\n%s", out)
	}
}

// TestAuditRenameThresholdIsLoadBearing pins the NUMBER. 50% is chosen, not
// git's default inherited by accident: the two live strikes measure R097 and
// R060, so 60% is the tightest value that clears both — zero margin, and the
// next rename+edit at 55% buys the triage line back. The self-test fixture is
// deliberately built at R065, the shape of the R060 strike rather than the easy
// R097 one, so raising the threshold to 75% reds it. If somebody wants a
// different number they get to change this pin, which is the point.
func TestAuditRenameThresholdIsLoadBearing(t *testing.T) {
	mutant, dir := srMutateScript(t,
		"--find-renames=50% -l0 --raw --no-abbrev",
		"--find-renames=75% -l0 --raw --no-abbrev")
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with the rename threshold raised to 75%%:\n%s", out)
	}
	if !strings.Contains(out, "a rename that also edits was flagged as a silent revert") {
		t.Fatalf("the rename arm did not notice the threshold change:\n%s", out)
	}
}

// TestAuditRenameSourceSuppressionHasItsOwnPin is the excusing half. The source
// path of an R leaves the tree, and flush() must read the per-entry moved flag
// the parse sets rather than deciding for itself from the blob ids — which is
// what the exact-blob rule did, and what could not see an edited rename.
func TestAuditRenameSourceSuppressionHasItsOwnPin(t *testing.T) {
	mutant, dir := srMutateScript(t, "moved=emoved[i]", "moved=0")
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with the rename source no longer excused:\n%s", out)
	}
	if !strings.Contains(out, "a rename that also edits was flagged as a silent revert") {
		t.Fatalf("the rename arm did not name the defect it exists for:\n%s", out)
	}
}

// TestAuditRenameDestinationIsStillCompared is the pin states_awk's R branch
// actually has, and the reason the self-test grew a third arm. Neutering the
// branch makes every rename invisible: the raw line parses as one path named
// "src<tab>dst", no deletion is recorded, and the rename-that-edits arm — which
// asserts nothing but silence — stays GREEN. Only the re-land arm, which
// asserts that a path returning to an older blob VIA a rename destination is
// still flagged, can tell excused from invisible. Measured both directions.
func TestAuditRenameDestinationIsStillCompared(t *testing.T) {
	mutant, dir := srMutateScript(t, "if (m[5] ~ /^[RC]/) {", "if (0) {")
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with states_awk blind to rename lines:\n%s", out)
	}
	if !strings.Contains(out, "a re-land through a rename was not caught") {
		t.Fatalf("the re-land arm did not name the defect it exists for:\n%s", out)
	}
	if strings.Contains(out, "a rename that also edits was flagged as a silent revert") {
		t.Fatalf("expected the rename-that-edits arm to stay green over this mutation "+
			"(invisible reads as excused); it did not, so this pin's rationale is stale:\n%s", out)
	}
}
