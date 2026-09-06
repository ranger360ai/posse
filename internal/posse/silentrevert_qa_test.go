//go:build posse_arm3

package posse

// QA pins for the silent-revert detector (scripts/audit-silent-reverts.sh,
// rangerhq-8rtf, verified under rangerhq-jkhb).
//
// Five claims:
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
//   5. A triage line survives the launcher's rebase (ADR 0054). The line names
//      the sha the audit printed in a SESSION tree; the launcher rebases that
//      tree at landing and mints another sha for the same diff, so on main the
//      line names a commit no ref reaches and the landed twin is UNTRIAGED —
//      measured 2026-09-04, e8c5e4e's line against its landed self c8adbcc,
//      with main's only gate red on it for three consecutive runs. A line may
//      now carry the diff's patch-id beside the sha, and it claims ONE commit:
//      the oldest flagged one carrying that diff, and only when the line's own
//      sha did not land here. This is a widening, so the pins at the end of
//      this file are again one per mutation, each naming the arm it reds.
//
// Self-contained on purpose (own helpers, own fixture): they must survive
// whatever the next persona does to the script's neighbours.

import (
	"fmt"
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
//
// It asserts BOTH a name list and an arm floor, because they fail differently
// and neither covers the other (ranger-base-am5q1). rc is set only on an
// explicit FAIL, so an arm that stops being EMITTED leaves rc 0 — the old
// assertion here (rc 0 plus the substring "self-test PASS" anywhere) survived
// deleting 16 of the 17 arms. The names below are the four the census found
// pinned by NOTHING: the other thirteen are named in
// TestSilentRevertSelfTestHasTheStrnumArm, ...HasTheRenameArms and
// ...HasThePatchIdArms, and a name pin is what catches a RENAMED arm. The
// floor is what catches a DELETED one, including arms added after this line
// was written that nobody thought to name — the same drift
// TestPIDDenySetSelfTestPasses guards with the same shape. It is a floor and
// not an equality so that adding an arm is not itself a red; raise it when you
// add one.
func TestSilentRevertSelfTestStillFires(t *testing.T) {
	t.Parallel()
	script := srScript(t)
	if err := exec.Command("git", "rev-parse", "--show-toplevel").Run(); err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	out, code := srAudit(t, script, ".", "--self-test")
	if code != 0 || !strings.Contains(out, "self-test PASS") {
		t.Fatalf("detector self-test did not fire: exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"self-test PASS: detector flags the modify half of the rangerhq-8rtf mechanism",
		"self-test PASS: detector flags the addonly half of the rangerhq-8rtf mechanism",
		"self-test PASS: a plain move is not flagged",
		"self-test PASS: detector flags the rangerhq-8rtf mechanism",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("self-test no longer reports %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "self-test PASS: "); n < 17 {
		t.Fatalf("self-test reported only %d passing arms, want >= 17:\n%s", n, out)
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// --- ADR 0054: the triage line survives the launcher's rebase ---------------
//
// A persona reads a silent-revert hit in its own session tree and writes the
// sha the audit printed into scripts/silent-reverts.allow. That sha is the
// SESSION tree's. The launcher lands the tree with merge --ff-only and rebases
// first when main has moved, which mints a new sha for the same diff, so on
// main the line names a commit no ref reaches, the landed twin is UNTRIAGED,
// and `make test` goes red on a hit that was read and explained. Measured
// 2026-09-04: e8c5e4e is on zero refs, c8adbcc is its landed self, same author,
// same second, same patch-id 77e50340…, and main's only gate had failed on that
// one hit for three consecutive runs.
//
// The fix is D1's optional second token — the diff's patch-id — plus three
// restrictions that keep it from becoming the pattern the allow file's header
// refuses: the token speaks only for a line whose sha did NOT land here (D2),
// it claims the OLDEST flagged commit carrying that diff and no other (D3),
// and the UNTRIAGED hint prints the whole line to paste so nobody has to know
// any of it (D4).
//
// Six self-test arms carry those claims, and the seven pins below are one per
// mutation. No two red the same arm, which is the property that makes them
// pins rather than seven copies of one exit status:
//
//	the token never matches                 reds twin (and the strip, and one-claim)
//	the line's sha is never an ancestor     reds inert, and NOTHING else
//	any 40-hex token matches                reds mismatch
//	the one-claim guard is removed          reds one-claim, and nothing else
//	the hint drops the patch-id             reds the hint arm, and nothing else
//	the triage print keeps the token        reds the strip arm, and nothing else
//
// The last row is the one worth naming. Stripping the token out of the printed
// reason is the whole reason the token can be OPTIONAL — a reason may not begin
// with 40 hex — and no arm that only reads an exit status can see it, because a
// triage that prints its reason badly still triages.

// TestSilentRevertSelfTestHasThePatchIdArms pins the six arms themselves.
// Deleting any of them leaves every other test in this file green, because they
// all read an exit status a deleted arm no longer contributes to.
func TestSilentRevertSelfTestHasThePatchIdArms(t *testing.T) {
	t.Parallel()
	script := srScript(t)
	if err := exec.Command("git", "rev-parse", "--show-toplevel").Run(); err != nil {
		t.Skipf("not a git checkout: %v", err)
	}
	out, code := srAudit(t, script, ".", "--self-test")
	if code != 0 {
		t.Fatalf("self-test failed: exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"self-test PASS: a line whose sha did not land triages its patch-id twin",
		"self-test PASS: the triage print strips the patch-id token",
		"self-test PASS: the token on an ANCESTOR's line is inert",
		"self-test PASS: a token that is not this commit's patch-id triages nothing",
		"self-test PASS: the UNTRIAGED hint carries the commit's real patch-id",
		"self-test PASS: one patch-id claims one commit; the second twin stays UNTRIAGED",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("self-test no longer reports %q:\n%s", want, out)
		}
	}
}

// TestAuditPatchIdTwinArmHasItsOwnPin is the mechanism: a line's token has to be
// compared against the flagged commit's patch-id at all. Break the comparison
// and the arm is the pre-ADR behaviour — a rebased twin reads as untriaged and
// the gate is red on a hit somebody already explained.
func TestAuditPatchIdTwinArmHasItsOwnPin(t *testing.T) {
	t.Parallel()
	mutant, dir := srMutateScript(t,
		`$1 !~ /^#/ && ($2 "") == (p "") { print $1 }`,
		`$1 !~ /^#/ && ($2 "") == ("never") { print $1 }`)
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with no line ever matching a patch-id:\n%s", out)
	}
	if !strings.Contains(out, "the patch-id twin arm did not triage the landed twin") {
		t.Fatalf("the twin arm did not name the defect it exists for:\n%s", out)
	}
}

// TestAuditTwinArmRefusesALandedLine is D2, and it is the restriction that
// keeps the token from being a pattern. A line whose sha IS an ancestor of the
// scanned tip has already said what it came to say; letting its token speak
// besides would excuse the NEXT commit with that diff, which is exactly the
// incident's own shape (a second stale-index revert of the same fix has the
// triaged revert's diff). Only the inert arm can see this: the twin arm stays
// green over the mutation, because a predicate that admits everything admits
// the true case too. Measured both directions.
func TestAuditTwinArmRefusesALandedLine(t *testing.T) {
	t.Parallel()
	mutant, dir := srMutateScript(t, `line_sha_landed "$s" "$tip" && continue`, `false && continue`)
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with every allow line treated as un-landed:\n%s", out)
	}
	if !strings.Contains(out, "an ancestor's token triaged another commit's diff") {
		t.Fatalf("the inert arm did not name the defect it exists for:\n%s", out)
	}
	if !strings.Contains(out, "self-test PASS: a line whose sha did not land triages its patch-id twin") {
		t.Fatalf("expected the twin arm to stay green over this mutation (a predicate that "+
			"admits everything admits the true case too); it did not, so this pin's rationale is stale:\n%s", out)
	}
}

// TestAuditTwinArmComparesTheToken is the wrong arm for the one above: the
// widening must be refused by the COMPARISON and not merely by the shape of the
// token. Accepting any 40-hex second field turns the allow file into a list of
// shas that excuse whatever is flagged next, which is alternative (d1) the ADR
// rejected outright.
func TestAuditTwinArmComparesTheToken(t *testing.T) {
	t.Parallel()
	mutant, dir := srMutateScript(t, `($2 "") == (p "")`, `($2 "") ~ /^[0-9a-f]{40}$/`)
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with any 40-hex token claiming any commit:\n%s", out)
	}
	if !strings.Contains(out, "the twin arm fired on a patch-id that does not match") {
		t.Fatalf("the mismatch arm did not name the defect it exists for:\n%s", out)
	}
}

// TestAuditPatchIdClaimsOneCommit is D3, and it is the whole difference between
// this and a pattern: a glob excuses every future commit that deletes a path,
// a token excuses the one commit whose diff the writer read, once. Remove the
// guard that spends a token and the second twin — a second stale-index revert
// of the same fix, the incident's own shape — is excused unread.
func TestAuditPatchIdClaimsOneCommit(t *testing.T) {
	t.Parallel()
	mutant, dir := srMutateScript(t, `*" $pid "*) ;;`, `*" NEVER-SPENT "*) ;;`)
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with one token claiming every commit carrying its diff:\n%s", out)
	}
	if !strings.Contains(out, "one token left 0 commit(s) untriaged") {
		t.Fatalf("the one-claim arm did not name the defect it exists for:\n%s", out)
	}
}

// TestAuditUntriagedHintCarriesThePatchId is D4, and ADR 0054 Verification 2.
// The teaching is the UNTRIAGED line: it prints the line to paste, patch-id
// included, EVERY time, so nobody is asked to know the recipe or to guess
// whether their commit will be rebased. Without it the writer has to know that
// the token exists, which is the state alternative (e) was rejected for.
func TestAuditUntriagedHintCarriesThePatchId(t *testing.T) {
	t.Parallel()
	mutant, dir := srMutateScript(t,
		`printf '               %s %s <reason>\n' "$sha" "$pid"`,
		`printf '               %s <reason>\n' "$sha"`)
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with the hint printing no patch-id:\n%s", out)
	}
	if !strings.Contains(out, "the UNTRIAGED hint did not carry the commit's patch-id") {
		t.Fatalf("the hint arm did not name the defect it exists for:\n%s", out)
	}
}

// TestAuditTriagePrintStripsTheToken is D1's other half, and the one an exit
// status cannot see: a triage that prints its reason badly still triages. The
// token is optional precisely because the print strips it — drop the strip and
// every reason on a tokened line reads "77e50340…8 the launcher rebased this
// one", which is how a grammar quietly stops being one.
func TestAuditTriagePrintStripsTheToken(t *testing.T) {
	t.Parallel()
	mutant, dir := srMutateScript(t,
		`if ($1 ~ /^[0-9a-f]{40}$/) sub($1" *", ""); print`,
		`print`)
	out, code := srAudit(t, mutant, dir, "--self-test")
	if code == 0 {
		t.Fatalf("self-test exited 0 with the triage print keeping the patch-id token:\n%s", out)
	}
	if !strings.Contains(out, "the triage print did not strip the patch-id token") {
		t.Fatalf("the strip arm did not name the defect it exists for:\n%s", out)
	}
}

// srAuditWithGitLog runs the audit over dir with a `git` shim first on PATH that
// appends every invocation's argv to a log, then returns the audit's output,
// its exit code and the log's contents. Counting the calls is the only honest
// way to measure the ADR's cost clause: "the arm runs only when there is an
// untriaged hit, so a clean run makes zero extra git calls."
func srAuditWithGitLog(t *testing.T, script, dir string, args ...string) (string, int, string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	shimDir := t.TempDir()
	logPath := filepath.Join(shimDir, "git-argv.log")
	shim := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %q\nexec %q \"$@\"\n", logPath, real)
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(shim), 0o755); err != nil {
		t.Fatalf("write git shim: %v", err)
	}
	cmd := exec.Command(script, args...)
	cmd.Dir = dir
	cmd.Env = append(srEnv(), "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s: %v\n%s", script, err, out)
	}
	logged, rerr := os.ReadFile(logPath)
	if rerr != nil && !os.IsNotExist(rerr) {
		t.Fatalf("read git log: %v", rerr)
	}
	// The shim has to have been used at all, or "zero patch-id calls" is a fact
	// about a log nobody wrote (the absence-without-a-witness class this file
	// already carries two pins for).
	if !strings.Contains(string(logged), "rev-parse") {
		t.Fatalf("the git shim was never called; PATH did not take:\n%s", logged)
	}
	return string(out), code, string(logged)
}

// TestAuditPatchIdArmCostsNothingOnACleanRun is ADR 0054's cost clause, and it
// is the reason the arm sits behind `if [ "$untriaged" -gt 0 ]` rather than
// being folded into the triage loop. `make test` runs this script with --quiet
// over the full history on every seat and on both CI runners; a patch-id per
// flagged commit is ~100ms each and every one of them is wasted on a run where
// the sha match already cleared the file. Two arms, because "no patch-id was
// computed" over a rig that never had one to compute is not a measurement:
// the same fixture with its allow line removed must compute one.
func TestAuditPatchIdArmCostsNothingOnACleanRun(t *testing.T) {
	t.Parallel()
	script := srScript(t)
	repo := srPlantAddOnlyRevert(t)

	// The wrong arm first: nothing is triaged, so the arm runs and the hint
	// needs a patch-id. If this does not fire, the arm below measures nothing.
	out, code, calls := srAuditWithGitLog(t, script, repo, "HEAD")
	if code != 1 {
		t.Fatalf("untriaged fixture did not exit 1: exit %d\n%s", code, out)
	}
	if !strings.Contains(calls, "patch-id") {
		t.Fatalf("the arm computed no patch-id over an UNTRIAGED hit, so the clean-run "+
			"arm below proves nothing:\n%s\ngit calls:\n%s", out, calls)
	}

	// Now triage the flagged commit by SHA — the six-of-eight case, a line
	// written after landing — and the arm must not run at all.
	sha := ""
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, "UNTRIAGED: "); i >= 0 {
			sha = strings.Fields(line[i+len("UNTRIAGED: "):])[0]
			break
		}
	}
	if sha == "" {
		t.Fatalf("no UNTRIAGED sha to triage in:\n%s", out)
	}
	if err := os.MkdirAll(filepath.Join(repo, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	allow := filepath.Join(repo, "scripts", "silent-reverts.allow")
	if err := os.WriteFile(allow, []byte(sha+" benign, and read: the sha landed here\n"), 0o644); err != nil {
		t.Fatalf("write allow: %v", err)
	}
	out, code, calls = srAuditWithGitLog(t, script, repo, "HEAD")
	if code != 0 {
		t.Fatalf("sha-triaged fixture did not exit 0: exit %d\n%s", code, out)
	}
	if strings.Contains(calls, "patch-id") {
		t.Fatalf("the patch-id arm ran on a clean scan; ADR 0054's cost clause says it must "+
			"not:\n%s\ngit calls:\n%s", out, calls)
	}
}
