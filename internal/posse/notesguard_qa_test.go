package posse

// The NOTES.md guard (ADR 0022 §3, ranger-base-hokh): the shared-index
// wall's own next-index-<pid> exemption treats a genuine path-limited
// commit as safe, and for NOTES.md that is exactly the failure ADR 0022
// closes — the file has no single writer in this tree (ranger-base-yuwy,
// 808da1b). These pins are the ADR's own Verification checklist (§Verification
// 1-5), each taken from the wall itself rather than from the source.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// notesGuardRepo is commitWallRepo's shape, but with NOTES.md already in
// history BEFORE the guard is installed: the guard refuses any commit that
// changes NOTES.md in the shared checkout, form included, so seeding the
// fixture has to happen before the wall exists to refuse it.
func notesGuardRepo(t *testing.T) (repo string, git func(env []string, args ...string) (string, error), persona []string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	gates := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git = func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if err := os.WriteFile(filepath.Join(repo, "NOTES.md"), []byte("# base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(nil, "add", "NOTES.md", "other.txt"); err != nil {
		t.Fatalf("fixture add: %v %s", err, out)
	}
	if out, err := git(nil, "commit", "-qm", "init"); err != nil {
		t.Fatalf("fixture commit (no wall yet): %v %s", err, out)
	}
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	return repo, git, []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + gates}
}

// dirty stages a same-line-count edit to NOTES.md and other.txt so both are
// dirty AND stageable with a plain `git add -- <path>` before each commit
// under test.
func dirtyNotesFixture(t *testing.T, repo string, git func(env []string, args ...string) (string, error)) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "NOTES.md"), []byte("# base\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("base\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Verification 1: shared checkout, RHQ_PERSONA set, NOTES.md and another
// file dirty — `git commit -- NOTES.md` is refused, naming both routes;
// `git commit -- <other>` lands, NOTES.md untouched and still dirty.
func TestQANotesGuardRefusesPathLimitedCommitInSharedCheckout(t *testing.T) {
	t.Parallel()
	repo, git, persona := notesGuardRepo(t)
	dirtyNotesFixture(t, repo, git)

	out, err := git(persona, "commit", "-m", "notes", "--", "NOTES.md")
	if err == nil {
		t.Fatalf("a path-limited commit of NOTES.md must be refused:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: a commit changing NOTES.md in the shared checkout",
		"docs/notes.d/<bead-id>.md",
		"session worktree",
		"ranger-base-yuwy",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q, got:\n%s", want, out)
		}
	}

	if out, err = git(persona, "commit", "-m", "other", "--", "other.txt"); err != nil {
		t.Fatalf("a path-limited commit of a different file must still land: %v %s", err, out)
	}
	if names, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); strings.Contains(names, "NOTES.md") {
		t.Fatalf("the other.txt commit must not have taken NOTES.md:\n%s", names)
	}
	if diff, _ := git(nil, "diff", "HEAD", "--", "NOTES.md"); strings.TrimSpace(diff) == "" {
		t.Fatalf("NOTES.md must still be dirty after the other commit")
	}
}

// Verification 2: the wall is unkeyed since rangerhq-lt2w — the operator's
// own shared-checkout commit is refused identically.
func TestQANotesGuardUnkeyedRefusesOperatorToo(t *testing.T) {
	t.Parallel()
	repo, git, _ := notesGuardRepo(t)
	dirtyNotesFixture(t, repo, git)

	out, err := git([]string{"RHQ_GATES_DIR=" + t.TempDir()}, "commit", "-m", "notes", "--", "NOTES.md")
	if err == nil {
		t.Fatalf("an operator commit (no RHQ_PERSONA) touching NOTES.md must be refused too:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: a commit changing NOTES.md in the shared checkout",
		"session operator", // RHQ_PERSONA unset reads "operator", not a blank
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q, got:\n%s", want, out)
		}
	}
}

// Verification 3: a session worktree has no shared index — git-dir !=
// git-common-dir exits the guard before the NOTES.md arm ever runs, so the
// same commit lands there.
func TestQANotesGuardStandsDownInSessionWorktree(t *testing.T) {
	t.Parallel()
	repo, git, persona := notesGuardRepo(t)
	wt := filepath.Join(t.TempDir(), "session")
	if out, err := git(nil, "worktree", "add", "-q", "-b", "posse/session", wt); err != nil {
		t.Fatalf("git worktree add: %v %s", err, out)
	}
	wtGit := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
		cmd.Env = append(append([]string(nil),
			"PATH="+PathOutsideGates(""), "HOME="+repo,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if err := os.WriteFile(filepath.Join(wt, "NOTES.md"), []byte("# base\nworktree edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := wtGit(persona, "commit", "-m", "notes from worktree", "--", "NOTES.md"); err != nil {
		t.Fatalf("NOTES.md commit from a session worktree must land: %v %s", err, out)
	}
}

// Verification 4: `git commit --amend -- NOTES.md` in the shared checkout as
// a persona is refused — amend takes a pathspec, and the arm reads the
// commit's own to-be-committed set, not its form.
func TestQANotesGuardRefusesAmend(t *testing.T) {
	t.Parallel()
	repo, git, persona := notesGuardRepo(t)
	// A prior commit unrelated to NOTES.md to amend against.
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("base\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(persona, "commit", "-m", "other", "--", "other.txt"); err != nil {
		t.Fatalf("fixture commit: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "NOTES.md"), []byte("# base\namended\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := git(persona, "commit", "--amend", "-m", "amend", "--", "NOTES.md")
	if err == nil {
		t.Fatalf("an amend touching NOTES.md must be refused:\n%s", out)
	}
	if !strings.Contains(out, "refused by posse gate: a commit changing NOTES.md in the shared checkout") {
		t.Errorf("the refusal must fire on amend too, got:\n%s", out)
	}
}

// Verification 5: a fragment commit — `git commit -- docs/notes.d/<id>.md`
// — is the route the refusal names, and it lands: `git log --grep <id>`
// finds it, the provenance promise ADR 0022 re-scopes rather than breaks.
func TestQANotesGuardAllowsFragmentCommit(t *testing.T) {
	t.Parallel()
	repo, git, persona := notesGuardRepo(t)
	fragDir := filepath.Join(repo, "docs", "notes.d")
	if err := os.MkdirAll(fragDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fragPath := filepath.Join(fragDir, "ranger-base-hokh.md")
	if err := os.WriteFile(fragPath, []byte("# ranger-base-hokh\n\nnotes fragment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(persona, "add", "--", "docs/notes.d/ranger-base-hokh.md"); err != nil {
		t.Fatalf("stage fragment: %v %s", err, out)
	}
	if out, err := git(persona, "commit", "-m", "ranger-base-hokh: notes fragment", "--", "docs/notes.d/ranger-base-hokh.md"); err != nil {
		t.Fatalf("a fragment commit must land: %v %s", err, out)
	}
	if out, _ := git(nil, "log", "--grep", "ranger-base-hokh", "--oneline"); !strings.Contains(out, "notes fragment") {
		t.Errorf("git log --grep must find the fragment commit, got:\n%s", out)
	}
}

// ranger-base-x9xbk: a MOVE of NOTES.md is a change to NOTES.md, and it has
// to be refused like an edit or a removal. Rename detection is on by default
// (git 2.9+ reads an unset diff.renames as true) and `--name-only` prints
// only the DESTINATION for a detected move, so the arm's exact-string test
// saw nothing and `git mv NOTES.md ARCHIVE.md` committed — carrying another
// persona's uncommitted lines out of the shared checkout under the mover's
// message, which is the ADR 0022 sweep itself.
//
// The fixture is seeded here rather than taken from notesGuardRepo on
// purpose: git only PAIRS the removal with the add at 50% similarity or
// better, and the similarity that counts is the one in this commit's own
// next-index — HEAD plus the named paths as they are on disk, persona A's
// uncommitted line included. Measured on the rig below: 200 lines scores
// R098 and only ARCHIVE.md is named, while at 3 lines and at 1 git reports
// the add and the delete separately and NOTES.md is in the list with no flag
// at all, so the pre-fix arm refused those and a small fixture measures
// nothing. A realistic file size is what makes the blind spot reachable.
func TestQANotesGuardRefusesAMoveOutOfNotes(t *testing.T) {
	t.Parallel()
	repo, git, persona := notesGuardRepo(t)

	var body strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&body, "line %d\n", i)
	}
	notes := filepath.Join(repo, "NOTES.md")
	if err := os.WriteFile(notes, []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	// Past the wall, which refuses this commit on its own merits — the same
	// way notesGuardRepo seeds NOTES.md before the guard exists.
	if out, err := git(nil, "-c", "core.hooksPath=/dev/null", "commit", "-qam", "seed a realistic NOTES.md"); err != nil {
		t.Fatalf("seed commit (hooks off): %v %s", err, out)
	}

	// Persona A's half-written edit, on disk and uncommitted: the thing a
	// move must not carry away.
	if err := os.WriteFile(notes, []byte(body.String()+"PERSONA-A-UNCOMMITTED\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Persona B moves it and commits path-limited — the one form the rest of
	// this wall treats as safe.
	if out, err := git(nil, "mv", "NOTES.md", "ARCHIVE.md"); err != nil {
		t.Fatalf("git mv: %v %s", err, out)
	}
	// Rig check, and it has to read the index the HOOK reads. `git mv` alone
	// stages an EXACT rename (R100: the blob it moves is the one already in
	// the index, not the edited file on disk), so the shared index pairs the
	// move at any fixture size and would answer this question with a yes it
	// has not earned. The commit under test builds its own next-index — HEAD
	// plus the named paths as they are ON DISK, persona A's line included —
	// and that is where the similarity is scored. Built the same way here:
	// measured, a 200-line fixture scores R098 and the default read names
	// only ARCHIVE.md, while at 3 lines and at 1 git reports the add and the
	// delete separately and NOTES.md is in the list even without the flag.
	// Shrink the fixture and this fires rather than going quietly green over
	// a move that was never blind.
	probe := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "next-index-probe")}
	if out, err := git(probe, "read-tree", "HEAD"); err != nil {
		t.Fatalf("rig check read-tree: %v %s", err, out)
	}
	if out, err := git(probe, "add", "--all", "--", "NOTES.md", "ARCHIVE.md"); err != nil {
		t.Fatalf("rig check add: %v %s", err, out)
	}
	if out, _ := git(probe, "diff", "--cached", "--name-only", "HEAD"); strings.Contains(out, "NOTES.md") {
		t.Fatalf("rig check: this commit's own to-be-committed set must PAIR the move, "+
			"so the default --name-only names only the destination; a set already "+
			"naming NOTES.md is the low-similarity case, which was never blind:\n%s", out)
	}
	if out, _ := git(probe, "diff", "--cached", "--name-only", "--no-renames", "HEAD"); !strings.Contains(out, "NOTES.md") {
		t.Fatalf("rig check: --no-renames must report the removal separately:\n%s", out)
	}

	out, err := git(persona, "commit", "-m", "B: archive the notes", "--", "NOTES.md", "ARCHIVE.md")
	if err == nil {
		landed, _ := git(nil, "show", "HEAD:ARCHIVE.md")
		t.Fatalf("a staged move of NOTES.md must be refused, the commit landed:\n%s\n"+
			"swept persona A's uncommitted line: %v", out, strings.Contains(landed, "PERSONA-A-UNCOMMITTED"))
	}
	if !strings.Contains(out, "refused by posse gate: a commit changing NOTES.md in the shared checkout") {
		t.Errorf("the refusal must fire on a move too, got:\n%s", out)
	}
	// The refusal changes nothing: NOTES.md is still what HEAD carries.
	if names, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); strings.Contains(names, "ARCHIVE.md") {
		t.Errorf("HEAD must not carry the move after the refusal:\n%s", names)
	}
}
