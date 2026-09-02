package posse

// A move of NOTES.md out of the shared checkout is a FAMILY of forms, and
// ranger-base-x9xbk pinned one of them: `git mv` followed by a path-limited
// commit (TestQANotesGuardRefusesAMoveOutOfNotes). The arm that refuses all
// of them is one read — the staged set taken with move detection off — so
// the other forms rest on the same flag with nothing holding them.
//
// Measured while verifying that close (ranger-base-fok87, 2026-09-01): with
// the flag taken back out, THREE of the four forms below land — the move
// into a subdirectory, the move made with a copy and a removal rather than
// `git mv`, and the move folded into `--amend`. Only `git commit -a` is
// caught, and by a different arm (`git commit -a or -i`), which is why this
// file does not pin WHICH arm answers that one. Git pairs a removal with an
// add on content similarity alone: it does not care whether `git mv` staged
// the pair, so "not using git mv" was never an escape route and one flag
// closes the family.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bigNotesWithAnUncommittedLine seeds a NOTES.md large enough for git to
// pair the move (the halves are reported separately below the 50% mark, and
// the pre-fix arm refused those already — a small fixture measures nothing),
// then leaves another persona's uncommitted line on disk: the thing a move
// must not carry away.
func bigNotesWithAnUncommittedLine(t *testing.T, repo string, git func(env []string, args ...string) (string, error)) {
	t.Helper()
	var body strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&body, "line %d\n", i)
	}
	notes := filepath.Join(repo, "NOTES.md")
	if err := os.WriteFile(notes, []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	// Past the wall, which refuses this commit on its own merits.
	if out, err := git(nil, "-c", "core.hooksPath=/dev/null", "commit", "-qam", "seed a realistic NOTES.md"); err != nil {
		t.Fatalf("seed commit (hooks off): %v %s", err, out)
	}
	if err := os.WriteFile(notes, []byte(body.String()+"PERSONA-A-UNCOMMITTED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestQANotesGuardRefusesEveryShapeOfMove(t *testing.T) {
	const notesArm = "refused by posse gate: a commit changing NOTES.md in the shared checkout"
	for _, tc := range []struct {
		name string
		// pinArm is false for the one form another arm may answer first.
		pinArm bool
		do     func(t *testing.T, repo string, git func([]string, ...string) (string, error), persona []string) (string, error)
	}{{
		name: "a move committed with -a, no pathspec",
		do: func(t *testing.T, repo string, git func([]string, ...string) (string, error), persona []string) (string, error) {
			if out, err := git(nil, "mv", "NOTES.md", "ARCHIVE.md"); err != nil {
				t.Fatalf("git mv: %v %s", err, out)
			}
			return git(persona, "commit", "-am", "B: archive the notes")
		},
	}, {
		name:   "a move into a subdirectory",
		pinArm: true,
		do: func(t *testing.T, repo string, git func([]string, ...string) (string, error), persona []string) (string, error) {
			// Not docs/: a new file there is refused by the public-genre
			// arm instead, and the arm under test never runs.
			if err := os.MkdirAll(filepath.Join(repo, "attic"), 0o755); err != nil {
				t.Fatal(err)
			}
			if out, err := git(nil, "mv", "NOTES.md", "attic/ARCHIVE.md"); err != nil {
				t.Fatalf("git mv: %v %s", err, out)
			}
			return git(persona, "commit", "-m", "B: archive the notes", "--", "NOTES.md", "attic/ARCHIVE.md")
		},
	}, {
		name:   "a move staged as a copy and a removal, no git mv",
		pinArm: true,
		do: func(t *testing.T, repo string, git func([]string, ...string) (string, error), persona []string) (string, error) {
			b, err := os.ReadFile(filepath.Join(repo, "NOTES.md"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, "ARCHIVE.md"), b, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(repo, "NOTES.md")); err != nil {
				t.Fatal(err)
			}
			if out, err := git(nil, "add", "-A", "--", "NOTES.md", "ARCHIVE.md"); err != nil {
				t.Fatalf("git add: %v %s", err, out)
			}
			return git(persona, "commit", "-m", "B: archive the notes", "--", "NOTES.md", "ARCHIVE.md")
		},
	}, {
		name:   "a move folded into an amend",
		pinArm: true,
		do: func(t *testing.T, repo string, git func([]string, ...string) (string, error), persona []string) (string, error) {
			if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("base\nmine\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := git(persona, "commit", "-m", "other", "--", "other.txt"); err != nil {
				t.Fatalf("fixture commit: %v %s", err, out)
			}
			if out, err := git(nil, "mv", "NOTES.md", "ARCHIVE.md"); err != nil {
				t.Fatalf("git mv: %v %s", err, out)
			}
			return git(persona, "commit", "--amend", "-m", "amend", "--", "NOTES.md", "ARCHIVE.md")
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			repo, git, persona := notesGuardRepo(t)
			bigNotesWithAnUncommittedLine(t, repo, git)

			out, err := tc.do(t, repo, git, persona)
			if err == nil {
				stat, _ := git(nil, "show", "--stat", "--format=", "HEAD")
				t.Fatalf("a move of NOTES.md must be refused in every form, this one landed:\n%s\nHEAD:\n%s", out, stat)
			}
			if tc.pinArm && !strings.Contains(out, notesArm) {
				t.Errorf("the NOTES.md arm is what must refuse this form (another arm answering first would leave the move unheld the day that arm moves), got:\n%s", out)
			}
			// The refusal changes nothing: NOTES.md is still what HEAD carries.
			if names, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); strings.Contains(names, "ARCHIVE.md") {
				t.Errorf("HEAD must not carry the move after the refusal:\n%s", names)
			}
		})
	}
}
