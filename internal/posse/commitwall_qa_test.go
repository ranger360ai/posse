package posse

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
	return repo, git, []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + gates}
}

// unwalledRepo is commitWallRepo without the hook: a scratch repo where the
// sweeping forms actually RUN, so what a cell below measures is git's answer
// and not the wall's. It has to be its own repo since rangerhq-lt2w dropped
// the operator carve-out — there is no env that gets a sweep past the wall
// in a repo that carries it, which is the point of that bead.
func unwalledRepo(t *testing.T) (string, func(env []string, args ...string) (string, error)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	return repo, func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
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

	// Persona B (developer) writes a half-finished line and stages it.
	if err := os.WriteFile(shared, []byte("base\nDEVELOPER HALF-WRITTEN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git([]string{"RHQ_PERSONA=developer"}, "add", "shared.txt"); err != nil {
		t.Fatalf("stage: %v %s", err, out)
	}

	// Persona A runs the check the refusal prescribes. It is clean.
	if out, _ := git(persona, "diff", "--", "shared.txt"); strings.TrimSpace(out) != "" {
		t.Fatalf("premise gone: `git diff -- shared.txt` is no longer clean over a staged edit:\n%s", out)
	}

	// …and commits with the blessed form. It lands B's line anyway.
	if out, err := git(persona, "commit", "-m", "qa's own message", "--", "shared.txt"); err != nil {
		t.Fatalf("the safe form must still pass the wall: %v %s", err, out)
	}
	out, err := git(nil, "show", "HEAD:shared.txt")
	if err != nil {
		t.Fatalf("git show: %s", out)
	}
	if !strings.Contains(out, "DEVELOPER HALF-WRITTEN") {
		t.Fatalf("premise gone: the path-limited commit no longer takes the on-disk file:\n%s", out)
	}
	// The check that would have caught it, for the same tree.
	if out, _ := git(nil, "diff", "HEAD~1", "--", "shared.txt"); !strings.Contains(out, "DEVELOPER HALF-WRITTEN") {
		t.Errorf("`git diff HEAD -- <paths>` is the form that sees a staged edit; it did not:\n%s", out)
	}
}

// TestQACommitWallPrescribesADiffThatCatchesStagedWork is the escape found
// verifying rangerhq-lvu9's close, filed as ranger-base-erba: the added
// line prescribed `git diff -- <paths>` and called a clean result "what
// makes the safe form actually safe". The sibling test above measures that
// this is false whenever the other persona has staged — the same class of
// over-promise rangerhq-lvu9 was filed to remove, one remove down.
//
// Fixed by wording (ranger-base-erba): the refusal now prescribes
// `git diff HEAD -- <paths>`, which does see a staged edit, and claims only
// what NOTES.md measures — the form bounds the paths, not the content.
func TestQACommitWallPrescribesADiffThatCatchesStagedWork(t *testing.T) {
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

// rangerhq-4pbt: the safe form cannot introduce a NEW file, and until this
// bead the message a persona hit had no route to the one that can.
//
// `unless --` demands a pathspec and a pathspec only matches a file git
// already has an index entry for, so `git commit -F - -- <untracked>` dies
// on git's own `did not match any file(s) known to git` BEFORE either wall
// runs. The next reach is `git add` + `git commit`, refused by the wall
// with the same "safe form" line that just failed them — two refusals and
// no way through, which is how the private GIT_INDEX_FILE recipe of
// rangerhq-8rtf gets reinvented. The three pins below hold the fix at the
// two layers that can speak (L3 hook, L1 shim) and the measurement they
// both rest on.

// TestQACommitWallSafeFormCannotIntroduceANewFile is the premise, measured
// rather than read: git's refusal of an untracked pathspec, and then the
// prescribed two-step run end to end against the real wall. The negative
// control is the second half — `git add -A`, the form the message tells you
// not to use, stages another persona's file, so "scoped, never bare" is a
// measurement and not manners.
func TestQACommitWallSafeFormCannotIntroduceANewFile(t *testing.T) {
	repo, git, persona := commitWallRepo(t)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("old.txt", "base\n")
	git(nil, "add", "old.txt")
	if out, err := git(nil, "commit", "-qm", "init", "--", "old.txt"); err != nil {
		t.Fatalf("fixture commit: %v %s", err, out)
	}

	// The premise: the blessed form, on a file that does not exist in the
	// index yet. git refuses it, and the wall never gets to say anything.
	write("new.txt", "mine\n")
	out, err := git(persona, "commit", "-m", "add new", "--", "new.txt")
	if err == nil {
		t.Fatalf("premise gone: a pathspec now matches an untracked file:\n%s", out)
	}
	if !strings.Contains(out, "did not match any file(s) known to git") {
		t.Fatalf("premise gone: git's own refusal is no longer the pathspec error:\n%s", out)
	}
	if strings.Contains(out, "refused by posse gate") {
		t.Fatalf("premise gone: the wall now speaks here, so the dead end is not the one filed:\n%s", out)
	}

	// Another persona has work staged in the shared index throughout: the
	// route must not sweep it, and `git add -A` must be shown to.
	write("theirs.txt", "theirs\n")
	if out, err := git([]string{"RHQ_PERSONA=developer"}, "add", "--", "theirs.txt"); err != nil {
		t.Fatalf("stage theirs: %v %s", err, out)
	}

	// The route the refusal now names, run as written.
	if out, err := git(persona, "add", "--", "new.txt"); err != nil {
		t.Fatalf("the prescribed add must work: %v %s", err, out)
	}
	write("old.txt", "base\nmine\n")
	if out, err := git(persona, "commit", "-m", "two-step", "--", "new.txt", "old.txt"); err != nil {
		t.Fatalf("the prescribed commit must pass the wall: %v %s", err, out)
	}
	if out, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); strings.Contains(out, "theirs.txt") ||
		!strings.Contains(out, "new.txt") || !strings.Contains(out, "old.txt") {
		t.Errorf("the two-step must land both named paths and nothing of the other persona's:\n%s", out)
	}
	// And it refreshes the shared index for the newly tracked path too:
	// nothing of the committer's is left staged, and the other persona's
	// entry is exactly what is.
	staged, _ := git(nil, "diff", "--cached", "--name-only", "HEAD")
	if strings.TrimSpace(staged) != "theirs.txt" {
		t.Errorf("after the two-step the shared index must hold the other persona's entry and only that, got %q", staged)
	}

	// The negative control, with its own witness: the form the message
	// forbids stages the other persona's file into the shared index.
	if out, err := git(persona, "add", "-A"); err != nil {
		t.Fatalf("git add -A: %v %s", err, out)
	}
	staged, _ = git(nil, "diff", "--cached", "--name-only", "HEAD")
	if !strings.Contains(staged, "theirs.txt") {
		t.Errorf("control measured nothing: `git add -A` must stage the other persona's file, got %q", staged)
	}
}

// TestQACommitWallRefusalNamesTheNewFileRoute is rangerhq-4pbt's DONE WHEN
// at L3, taken from the wall a persona actually hits: the refusal that
// names the safe form must also name what that form presumes and the two
// steps that get a new file in, with the add scoped.
func TestQACommitWallRefusalNamesTheNewFileRoute(t *testing.T) {
	repo, git, persona := commitWallRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "a.txt")
	git(nil, "commit", "-qm", "init", "--", "a.txt")
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "b.txt")

	out, err := git(persona, "commit", "-m", "sweep")
	if err == nil {
		t.Fatalf("an unqualified commit must be refused:\n%s", out)
	}
	for _, want := range []string{
		"a NEW file cannot be committed by that form alone (rangerhq-4pbt)",
		`did not match any file(s) known to git`,
		"git add -- <the new paths>",
		"git commit -F - -- <all your paths>",
		"never a bare 'git add -A' or 'git add .'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q, got:\n%s", want, out)
		}
	}
	// The scoping is the whole point of the line: a route that reads as a
	// bare `git add` is the shared-index write the wall exists to prevent.
	if strings.Contains(out, "  git add\n") || strings.Contains(out, "git add .\n") {
		t.Errorf("the prescribed add must be scoped with `--`, got:\n%s", out)
	}
}

// TestQAL1CommitRefusalNamesTheNewFileRoute is the same DONE WHEN at L1,
// and it is not redundant: in a session worktree the L3 wall stands down by
// design (rangerhq-09o2 — there is no shared index there), so the shim is
// the only layer that speaks, and a persona adding a file there hits the
// identical dead end. Read from the rendered shim, not from the table.
func TestQAL1CommitRefusalNamesTheNewFileRoute(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	_, binDir, _, err := a.RenderGates("qa", []string{"Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(binDir, "git"), "commit", "-m", "x")
	cmd.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
	b, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("an unqualified commit must be refused by the shim:\n%s", b)
	}
	out := string(b)
	for _, want := range []string{
		"safe form: git commit … -- <operand>",
		"a NEW file has no index entry yet, so no pathspec matches it",
		"git add -- <the new paths>",
		"scoped and never bare",
		"rangerhq-4pbt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the L1 refusal must carry %q, got:\n%s", want, out)
		}
	}
	// It is keyed to the qualifier, not printed under every negative rule:
	// a rule with no prerequisite gets the safe form and nothing more.
	if h := ruleHint("qa", "posse", shimRule{Words: []string{"promote"}, Unless: "--"}); strings.Contains(h, "NEW file") {
		t.Errorf("the prerequisite is keyed by command and subcommand, not blanket: %q", h)
	}
}

// TestQANewFileStagingFormsAgainstEverySweeper is why the prescribed first
// step is the plain scoped `git add` and not `git add -N`, which also makes
// the pathspec match and looks like it closes the residual.
//
// The whole 3×4 matrix, measured against the real sweeping forms with
// another persona's entry staged throughout, in a repo carrying no hook —
// since rangerhq-lt2w dropped the operator carve-out there is no env that
// gets a sweep past the wall, and what each cell has to say is git's answer
// and not the wall's. The wall column of the table in
// docs/notes.d/rangerhq-4pbt.md is the sibling pins' subject, not this one's. `-N` differs from the plain add on exactly the forms the
// wall already refuses, and matches it on `commit -- .` — the one hole the
// refusal names and only rangerhq-09o2 closes. It also converts a file that
// `-a` and `.` would skip outright into one they take. A less-known flag
// that buys nothing under the wall is not the form to teach.
func TestQANewFileStagingFormsAgainstEverySweeper(t *testing.T) {
	sweeps := map[string][]string{
		"unqualified": {"commit", "-m", "sweep"},
		"-i":          {"commit", "-i", "-m", "sweep", "--", "theirs.txt"},
		"-a":          {"commit", "-a", "-m", "sweep"},
		"dot":         {"commit", "-m", "sweep", "--", "."},
	}
	// true = the sweeper carried the new file away under its own message.
	want := map[string]map[string]bool{
		"untracked": {"unqualified": false, "-i": false, "-a": false, "dot": false},
		"add":       {"unqualified": true, "-i": true, "-a": true, "dot": true},
		"add -N":    {"unqualified": false, "-i": false, "-a": true, "dot": true},
	}
	staging := map[string][]string{
		"untracked": nil,
		"add":       {"add", "--", "new.txt"},
		"add -N":    {"add", "-N", "--", "new.txt"},
	}
	for stage, want := range want {
		for sweep, wantTaken := range want {
			t.Run(stage+"/"+sweep, func(t *testing.T) {
				repo, git := unwalledRepo(t)
				write := func(n, b string) {
					if err := os.WriteFile(filepath.Join(repo, n), []byte(b), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				write("a.txt", "a\n")
				git(nil, "add", "a.txt")
				if out, err := git(nil, "commit", "-qm", "init", "--", "a.txt"); err != nil {
					t.Fatalf("fixture commit: %v %s", err, out)
				}
				write("new.txt", "mine\n")
				if args := staging[stage]; args != nil {
					if out, err := git(nil, args...); err != nil {
						t.Fatalf("%s: %v %s", stage, err, out)
					}
				}
				write("theirs.txt", "theirs\n")
				if out, err := git(nil, "add", "--", "theirs.txt"); err != nil {
					t.Fatalf("stage theirs: %v %s", err, out)
				}
				if out, err := git(nil, sweeps[sweep]...); err != nil {
					t.Fatalf("the sweep must run — this repo carries no wall: %v %s", err, out)
				}
				// The witness: every sweeper here does take SOMETHING, so a
				// "not taken" cell is a fact about the new file and not a
				// commit that never happened.
				if out, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(out, "theirs.txt") {
					t.Fatalf("control measured nothing: the sweep landed no other persona's file:\n%s", out)
				}
				_, err := git(nil, "show", "HEAD:new.txt")
				if taken := err == nil; taken != wantTaken {
					t.Errorf("%s then `git %s`: new file taken by the sweep = %v, want %v",
						stage, strings.Join(sweeps[sweep], " "), taken, wantTaken)
				}
			})
		}
	}
}
