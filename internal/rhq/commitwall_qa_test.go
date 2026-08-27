package rhq

// The commit wall's refusal text, verified against the wall itself
// (ranger-base-k19q, verifying the close of rangerhq-lvu9).
//
// rangerhq-lvu9 was filed because the refusal promised a safety the safe
// form does not have: `git commit -F - -- <paths>` commits the file as it
// is ON DISK, so another persona's in-flight edit to a path you name rides
// into your commit under your message. The close (b537d84) added four lines
// naming that. These pins hold the two halves apart:
//
//   - the refusal still names the in-flight-edit case at all, so a later
//     edit to the hook body cannot quietly drop it again;
//   - the residual it describes is measured, not asserted — the mechanism
//     is git's, no refusal at this layer can close it (only rangerhq-09o2's
//     isolation can), so the measurement is permanent and any claim in the
//     message has to be judged against it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// commitWallRepo is a scratch repo with the real prepare-commit-msg wall
// installed, plus a `git` runner in the shape TestSharedIndexCommitHook
// uses: the operator's env by default, `persona` for a session's.
func commitWallRepo(t *testing.T) (repo string, git func(env []string, args ...string) (string, error), persona []string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	gates := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git = func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	return repo, git, []string{"RHQ_PERSONA=laurie", "RHQ_GATES_DIR=" + gates}
}

// TestQACommitWallRefusalNamesTheInFlightEdit is rangerhq-lvu9's DONE WHEN,
// taken from the wall rather than from the source string: the refusal a
// persona actually sees must say that a named path commits the file as it
// is on disk, and that another persona's edit rides in with it.
func TestQACommitWallRefusalNamesTheInFlightEdit(t *testing.T) {
	repo, git, persona := commitWallRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "shared.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "shared.txt")
	if out, err := git(nil, "commit", "-qm", "init", "--", "shared.txt"); err != nil {
		t.Fatalf("fixture commit: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "shared.txt"), []byte("base\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "shared.txt")

	out, err := git(persona, "commit", "-m", "sweep")
	if err == nil {
		t.Fatalf("an unqualified commit must be refused:\n%s", out)
	}
	for _, want := range []string{
		"safe form: git commit -F - -- <paths>",
		"ON DISK",
		"if another persona is editing it, you commit their",
		"rangerhq-lvu9",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q, got:\n%s", want, out)
		}
	}
}

// TestQACommitWallTakesAnotherPersonasStagedLineUnderACleanDiff is the
// measurement the refusal's advice has to survive.
//
// `git diff -- <paths>` compares the WORKING TREE against the INDEX. When
// the other persona has staged their in-flight edit — rangerhq-2f5r's own
// shape, a persona who ran `git add` and has not committed — index and
// working tree agree, so that diff is empty while their line is still what
// a path-limited commit will take. Measured here end to end: clean diff,
// no refusal, their line in the commit.
//
// This is not a bug in the wall and no refusal at this layer can fix it
// (rangerhq-09o2's isolation is the only answer). It is the fact any
// wording in the refusal is measured against.
func TestQACommitWallTakesAnotherPersonasStagedLineUnderACleanDiff(t *testing.T) {
	repo, git, persona := commitWallRepo(t)
	shared := filepath.Join(repo, "shared.txt")
	if err := os.WriteFile(shared, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "shared.txt")
	if out, err := git(nil, "commit", "-qm", "init", "--", "shared.txt"); err != nil {
		t.Fatalf("fixture commit: %v %s", err, out)
	}

	// Persona B (dinesh) writes a half-finished line and stages it.
	if err := os.WriteFile(shared, []byte("base\nDINESH HALF-WRITTEN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git([]string{"RHQ_PERSONA=dinesh"}, "add", "shared.txt"); err != nil {
		t.Fatalf("stage: %v %s", err, out)
	}

	// Persona A runs the check the refusal prescribes. It is clean.
	if out, _ := git(persona, "diff", "--", "shared.txt"); strings.TrimSpace(out) != "" {
		t.Fatalf("premise gone: `git diff -- shared.txt` is no longer clean over a staged edit:\n%s", out)
	}

	// …and commits with the blessed form. It lands B's line anyway.
	if out, err := git(persona, "commit", "-m", "laurie's own message", "--", "shared.txt"); err != nil {
		t.Fatalf("the safe form must still pass the wall: %v %s", err, out)
	}
	out, err := git(nil, "show", "HEAD:shared.txt")
	if err != nil {
		t.Fatalf("git show: %s", out)
	}
	if !strings.Contains(out, "DINESH HALF-WRITTEN") {
		t.Fatalf("premise gone: the path-limited commit no longer takes the on-disk file:\n%s", out)
	}
	// The check that would have caught it, for the same tree.
	if out, _ := git(nil, "diff", "HEAD~1", "--", "shared.txt"); !strings.Contains(out, "DINESH HALF-WRITTEN") {
		t.Errorf("`git diff HEAD -- <paths>` is the form that sees a staged edit; it did not:\n%s", out)
	}
}

// TestQACommitWallPrescribesADiffThatCatchesStagedWork is the escape found
// verifying rangerhq-lvu9's close, filed as ranger-base-erba: the added
// line prescribes `git diff -- <paths>` and calls a clean result "what
// makes the safe form actually safe". The sibling test above measures that
// this is false whenever the other persona has staged — the same class of
// over-promise rangerhq-lvu9 was filed to remove, one remove down.
//
// Skipped until erba lands: the fix is a wording change (`git diff HEAD --
// <paths>`, which does see a staged edit), not a behaviour change.
func TestQACommitWallPrescribesADiffThatCatchesStagedWork(t *testing.T) {
	t.Skip("ranger-base-erba: the refusal prescribes `git diff -- <paths>`, which is clean over another persona's staged edit")

	repo, git, persona := commitWallRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "a.txt")
	git(nil, "commit", "-qm", "init", "--", "a.txt")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "a.txt")

	out, err := git(persona, "commit", "-m", "sweep")
	if err == nil {
		t.Fatalf("an unqualified commit must be refused:\n%s", out)
	}
	if !strings.Contains(out, "git diff HEAD -- <paths>") {
		t.Errorf("the prescribed check must be one that sees a staged edit, got:\n%s", out)
	}
	if strings.Contains(out, "a clean diff there is what makes the safe form actually safe") {
		t.Errorf("a clean `git diff` is not sufficient — it is empty over a staged edit:\n%s", out)
	}
}
