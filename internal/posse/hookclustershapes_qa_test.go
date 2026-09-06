//go:build !posse_arm2 && !posse_arm3

package posse

// THE SHAPES THE L3 HOOK CLUSTER'S OWN PINS DO NOT REACH (QA, verify
// bead ranger-base-bki3i, verifying the four closes folded into
// ranger-base-dmsbu: ranger-base-60azj, ranger-base-4b1z4,
// ranger-base-qdxe, ranger-base-jex3).
//
// Each of those four is fixed and each has a pin. What was pinned is ONE
// shape per defect — one file moved, one extension, one `git mv`, one `rm
// -rf`. The defects were all "git's answer about the staged set changes with
// the form of the change", and a form is not one command: a directory move,
// a move that also edits, a pure deletion, a rename INTO the class, a path
// with a space or a non-ASCII byte in it, and a staged marker removal all
// ask the same arm the same question a different way. These are those ways,
// measured against the REAL rendered hook and a real `git commit`.
//
// MUTATION-CHECKED as a set (recorded on ranger-base-bki3i):
//   - --no-renames out of check 1's listing        reds every arm of the
//                                                  docs-move pin
//   - MarkdownPathspecs back to []string{"*.md"}   reds the case arms and
//                                                  the move-that-edits pin
//   - --no-renames out of the constitution arm     reds every move-out arm
//                                                  and the staged-removal pin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkdirIn is git mv's missing half: it will not create a destination parent.
func mkdirIn(t *testing.T, repo, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
}

// assertDocsGenreRefusal is check 1's verdict: refused, by check 1, naming
// the destination path.
func assertDocsGenreRefusal(t *testing.T, out string, err error, wantPath string) {
	t.Helper()
	if err == nil {
		t.Fatalf("a move into an unlisted genre must be refused, not committed:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: a new docs/ file outside the public genre allowlist",
		wantPath,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q:\n%s", want, out)
		}
	}
}

// ─── check 1: the move shapes beyond one file (ranger-base-60azj) ─────────

func TestQADocsGenreWallSeesEveryMoveShape(t *testing.T) {
	const clean = "a line of perfectly public prose\n"
	body := strings.Repeat(clean, 40)

	// A DIRECTORY move is the shape someone reorganising types, and it is
	// several renames in one commit rather than one.
	t.Run("a whole directory moved into an unlisted genre", func(t *testing.T) {
		w := newVisWall(t)
		w.plant(t, w.pub, "docs/adr/a.md", body)
		w.plant(t, w.pub, "docs/adr/b.md", strings.Repeat("other public prose\n", 40))
		if o, err := w.git(w.pub, nil, "mv", "docs/adr", "docs/incidents"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		out, err := w.git(w.pub, w.persona, "commit", "-m", "reorganise", "--", "docs")
		assertDocsGenreRefusal(t, out, err, "(genre: incidents)")
	})

	// A move that also EDITS the file is an R below 100%, and it is what
	// "move the write-up and tidy it" actually produces.
	t.Run("a move that also modifies the file", func(t *testing.T) {
		w := newVisWall(t)
		w.plant(t, w.pub, "docs/adr/c.md", body)
		mkdirIn(t, w.pub, "docs/postmortems")
		if o, err := w.git(w.pub, nil, "mv", "docs/adr/c.md", "docs/postmortems/c.md"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		write(t, filepath.Join(w.pub, "docs/postmortems/c.md"),
			strings.Repeat(clean, 30)+strings.Repeat("a quarter of it rewritten, still public\n", 10))
		if o, err := w.git(w.pub, nil, "add", "--", "docs/postmortems/c.md"); err != nil {
			t.Fatalf("git add: %v %s", err, o)
		}
		// The fixture premise: git must still PAIR these two, or the arm is
		// measuring the genre rule and not the flag that hid it.
		if ns, _ := w.git(w.pub, nil, "diff", "--cached", "--name-status", "HEAD", "--", "docs/*"); !strings.HasPrefix(ns, "R") {
			t.Fatalf("fixture premise: git must report this move as a rename, got %q", ns)
		}
		out, err := w.git(w.pub, w.persona, "commit", "-m", "move and tidy", "--", "docs/adr/c.md", "docs/postmortems/c.md")
		assertDocsGenreRefusal(t, out, err, "(genre: postmortems)")
	})

	// Check 1 keys on the PATH, so the destination need not be markdown —
	// the genre is what ADR 0024 D1 names, not the extension.
	t.Run("a non-markdown file moved into an unlisted genre", func(t *testing.T) {
		w := newVisWall(t)
		w.plant(t, w.pub, "docs/adr/d.txt", body)
		mkdirIn(t, w.pub, "docs/rca")
		if o, err := w.git(w.pub, nil, "mv", "docs/adr/d.txt", "docs/rca/d.txt"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		out, err := w.git(w.pub, w.persona, "commit", "-m", "move", "--", "docs/adr/d.txt", "docs/rca/d.txt")
		assertDocsGenreRefusal(t, out, err, "(genre: rca)")
	})

	// The genre is the FIRST segment under docs/, however deep the rest goes.
	t.Run("a move deep under an unlisted genre", func(t *testing.T) {
		w := newVisWall(t)
		w.plant(t, w.pub, "docs/adr/e.md", body)
		mkdirIn(t, w.pub, "docs/rca/2026/09")
		if o, err := w.git(w.pub, nil, "mv", "docs/adr/e.md", "docs/rca/2026/09/e.md"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		out, err := w.git(w.pub, w.persona, "commit", "-m", "file it", "--", "docs/adr/e.md", "docs/rca/2026/09/e.md")
		assertDocsGenreRefusal(t, out, err, "(genre: rca)")
	})

	// A move to docs/ itself has no genre at all, and the refusal has to say
	// so rather than name an empty one.
	t.Run("a move to a bare path directly under docs", func(t *testing.T) {
		w := newVisWall(t)
		w.plant(t, w.pub, "docs/adr/f.md", body)
		if o, err := w.git(w.pub, nil, "mv", "docs/adr/f.md", "docs/f.md"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		out, err := w.git(w.pub, w.persona, "commit", "-m", "loose", "--", "docs/adr/f.md", "docs/f.md")
		assertDocsGenreRefusal(t, out, err, "no subdirectory")
	})

	// A NON-ASCII destination still does not get through. It is asserted as
	// a refusal and nothing more on purpose: check 1's listing does not pass
	// core.quotePath=false, so git C-quotes this path, the leading quote
	// stops it matching `docs/*/*`, and the refusal names the genre WRONGLY
	// (ranger-base-k2ohx — fail-closed, and its own bead). The invariant
	// this pin owns is the one ADR 0024 D1 states: the file does not land.
	t.Run("a move to a non-ASCII destination path", func(t *testing.T) {
		w := newVisWall(t)
		w.plant(t, w.pub, "docs/adr/g.md", body)
		mkdirIn(t, w.pub, "docs/rca")
		if o, err := w.git(w.pub, nil, "mv", "docs/adr/g.md", "docs/rca/ünlisted.md"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		out, err := w.git(w.pub, w.persona, "commit", "-m", "move", "--", "docs/adr/g.md", "docs/rca/ünlisted.md")
		if err == nil {
			t.Fatalf("a move into an unlisted genre must be refused whatever the path's bytes:\n%s", out)
		}
		if o, _ := w.git(w.pub, nil, "log", "--oneline", "-1", "--name-only"); strings.Contains(o, "docs/rca") {
			t.Errorf("nothing may have landed:\n%s", o)
		}
	})
}

// ─── check 2: the case arms, and a move that edits (ranger-base-4b1z4) ────

// The shipped pin owns .md/.MD/.markdown/.Markdown. These are the spellings
// between them: an extension is not two states, and ':(icase)' either holds
// for the whole string or the list is a guess.
func TestQAMarkdownScanOwnsEveryCaseOfTheListedSpellings(t *testing.T) {
	const opsBody = "# a note\n\nthe pilot cost $715/wk to run.\n"
	for _, rel := range []string{
		"docs/adr/a.mD",
		"docs/adr/b.MARKDOWN",
		"docs/adr/c.MarkDown",
	} {
		t.Run(rel, func(t *testing.T) {
			w := newVisWall(t)
			w.stage(t, w.pub, rel, opsBody)
			out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
			if err == nil {
				t.Fatalf("the same bytes must be refused whichever case the extension is written in:\n%s", out)
			}
			for _, want := range []string{
				"refused by posse gate: ops-class content in staged markdown in a public repo",
				"cost:", "$715/wk",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("the refusal must carry %q:\n%s", want, out)
				}
			}
		})
	}
}

// The two defects at once: check 2's CONTENT reader keeps rename detection
// ON deliberately (a pure move re-presenting the whole file as added lines
// would re-scan history), so a move that also adds an ops line shows only
// that line — and the destination is spelled .MD, which is what walked
// through before. Neither half may hide the other.
func TestQAMarkdownScanSeesAnOpsLineAddedOnAMove(t *testing.T) {
	w := newVisWall(t)
	w.plant(t, w.pub, "docs/adr/clean.md", strings.Repeat("public prose\n", 40))
	if o, err := w.git(w.pub, nil, "mv", "docs/adr/clean.md", "docs/adr/renamed.MD"); err != nil {
		t.Fatalf("git mv: %v %s", err, o)
	}
	write(t, filepath.Join(w.pub, "docs/adr/renamed.MD"),
		strings.Repeat("public prose\n", 40)+"the pilot cost $715/wk to run.\n")
	if o, err := w.git(w.pub, nil, "add", "--", "docs/adr/renamed.MD"); err != nil {
		t.Fatalf("git add: %v %s", err, o)
	}
	out, err := w.git(w.pub, w.persona, "commit", "-m", "move and add", "--", "docs/adr/clean.md", "docs/adr/renamed.MD")
	if err == nil {
		t.Fatalf("an ops line added on a move into a .MD name must be refused:\n%s", out)
	}
	if !strings.Contains(out, "$715/wk") {
		t.Errorf("refused, but not by check 2:\n%s", out)
	}
}

// ─── the constitution arm: every way OUT of the class (ranger-base-qdxe) ──

// The shipped pin owns `git mv` of a promoted path, `git mv` of the settings
// file, and copy-then-remove. A class member leaves by more doors than
// those, and the arm's answer has to be the same at each: a pure deletion (a
// persona that can delete the file un-fences itself, ranger-base-az93), a
// directory move, the LOCAL settings file, a move INTO the class, and paths
// whose bytes the -z reader and the IFS loop have to survive.
func TestQAConstitutionWallRefusesEveryMoveOutShape(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("deny rules\n", 40)

	t.Run("a pure deletion of a class path", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		rel := ConstitutionRepoMarker + "/developer.md"
		other := ConstitutionRepoMarker + "/keeper.md"
		stageAt(t, repo, git, nil, rel, body)
		stageAt(t, repo, git, nil, other, "kept\n")
		if o, err := git(nil, "commit", "-qm", "operator installs it", "--", rel, other); err != nil {
			t.Fatalf("fixture commit: %v %s", err, o)
		}
		if o, err := git(nil, "rm", "-q", "--", rel); err != nil {
			t.Fatalf("git rm: %v %s", err, o)
		}
		out, err := git(persona, "commit", "-m", "drop it", "--", rel)
		assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
	})

	t.Run("the whole class directory moved out", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		rel := ConstitutionRepoMarker + "/developer.md"
		stageAt(t, repo, git, nil, rel, body)
		if o, err := git(nil, "commit", "-qm", "operator installs it", "--", rel); err != nil {
			t.Fatalf("fixture commit: %v %s", err, o)
		}
		if o, err := git(nil, "mv", ConstitutionRepoMarker, "drafts"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		out, err := git(persona, "commit", "-m", "reorganise", "--", ConstitutionRepoMarker, "drafts")
		assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
	})

	// The other direction is the same class question: a file RENAMED INTO
	// the class is a persona writing the law under a new name.
	t.Run("an outside file moved INTO the class", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, false)
		stageAt(t, repo, git, nil, "draft.json", body)
		if o, err := git(nil, "commit", "-qm", "a draft", "--", "draft.json"); err != nil {
			t.Fatalf("fixture commit: %v %s", err, o)
		}
		mkdirIn(t, repo, filepath.Dir(filepath.FromSlash(ClaudeProjectConfig)))
		if o, err := git(nil, "mv", "draft.json", ClaudeProjectConfig); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		out, err := git(persona, "commit", "-m", "install it", "--", "draft.json", ClaudeProjectConfig)
		assertConstitutionRefusal(t, out, err, ClaudeProjectConfig, gatesDirOf(persona))
	})

	t.Run("the local settings file moved out", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, false)
		stageAt(t, repo, git, nil, ClaudeProjectConfigLocal, body)
		if o, err := git(nil, "commit", "-qm", "operator installs it", "--", ClaudeProjectConfigLocal); err != nil {
			t.Fatalf("fixture commit: %v %s", err, o)
		}
		if o, err := git(nil, "mv", ClaudeProjectConfigLocal, "local-draft.json"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		out, err := git(persona, "commit", "-m", "draft", "--", ClaudeProjectConfigLocal, "local-draft.json")
		assertConstitutionRefusal(t, out, err, ClaudeProjectConfigLocal, gatesDirOf(persona))
	})

	// The reader is -z and diff.relative=false so no path is C-quoted, and
	// the loop splits on a newline IFS under `set -f` so a space or a glob
	// character stays part of the path. Both are load-bearing at exactly
	// this join and neither had an arm.
	for name, rel := range map[string]string{
		"a class path carrying a non-ASCII byte": ConstitutionRepoMarker + "/entwickler-ü.md",
		"a class path carrying a space":          ConstitutionRepoMarker + "/dev eloper.md",
		"a class path carrying a glob character": ConstitutionRepoMarker + "/dev[0-9].md",
	} {
		t.Run(name+" moved out", func(t *testing.T) {
			repo, git, persona := constitutionWallRepo(t, true)
			stageAt(t, repo, git, nil, rel, body)
			if o, err := git(nil, "commit", "-qm", "operator installs it", "--", rel); err != nil {
				t.Fatalf("fixture commit: %v %s", err, o)
			}
			if o, err := git(nil, "mv", rel, "notes.md"); err != nil {
				t.Fatalf("git mv: %v %s", err, o)
			}
			out, err := git(persona, "commit", "-m", "move it", "--", rel, "notes.md")
			assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
		})
	}

	// And the class did not widen into everything that merely starts with a
	// class path's name: the `case` matches the member or the member plus a
	// slash, and a sibling directory is neither.
	t.Run("a path that only has a class path as a prefix is outside it", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		rel := ConstitutionRepoMarker + "x/foo.md"
		stageAt(t, repo, git, nil, rel, body)
		if o, err := git(nil, "commit", "-qm", "a sibling", "--", rel); err != nil {
			t.Fatalf("fixture commit: %v %s", err, o)
		}
		if o, err := git(nil, "mv", rel, "bar.md"); err != nil {
			t.Fatalf("git mv: %v %s", err, o)
		}
		if out, err := git(persona, "commit", "-m", "move it", "--", rel, "bar.md"); err != nil {
			t.Errorf("%s is not in the class and must commit: %v\n%s", rel, err, out)
		}
	})
}

// ─── the marker, staged and swapped (ranger-base-jex3) ────────────────────

// The shipped pin owns the UNSTAGED worktree removal, the marker replaced by
// a file, genesis, and no marker anywhere. These are the shapes where the
// removal is real work rather than a dodge: staged in the same commit, or
// committed on its own. Both are killed by removing the base-tree arm from
// the marker test, which is what ranger-base-jex3 added — with the marker
// gone from the worktree AND from the index, only the base tree still says
// this is a constitution.
//
// The third arm measures the OTHER half: `-d` follows a symlink, so swapping
// the marker directory for a link to an empty one leaves the worktree test
// answering true. It survives the base-arm mutant on purpose — a defeat of
// the worktree test would have to beat this too.
func TestQAConstitutionMarkerSurvivesAStagedRemoval(t *testing.T) {
	t.Parallel()
	plantLaw := func(t *testing.T, repo string, git func(env []string, args ...string) (string, error)) {
		t.Helper()
		rel := ConstitutionRepoMarker + "/developer.md"
		stageAt(t, repo, git, nil, rel, "the law\n")
		if o, err := git(nil, "commit", "-qm", "the law", "--", rel); err != nil {
			t.Fatalf("fixture commit: %v %s", err, o)
		}
	}

	t.Run("the marker removal staged alongside the class edit", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		plantLaw(t, repo, git)
		if o, err := git(nil, "rm", "-rq", "--", ConstitutionRepoMarker); err != nil {
			t.Fatalf("git rm -r: %v %s", err, o)
		}
		stageAt(t, repo, git, nil, ConstitutionSourceDir+"/config.yaml", "drafted\n")
		out, err := git(persona, "commit", "-m", "tidy", "--", ConstitutionRepoMarker, ConstitutionSourceDir+"/config.yaml")
		assertConstitutionRefusal(t, out, err, ConstitutionSourceDir+"/config.yaml", gatesDirOf(persona))
	})

	// The two-commit version of the same idea — remove the marker first,
	// edit the law next — needs its first commit to land, and it does not:
	// the marker's own contents are class paths.
	t.Run("removing the marker as its own commit", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		plantLaw(t, repo, git)
		if o, err := git(nil, "rm", "-rq", "--", ConstitutionRepoMarker); err != nil {
			t.Fatalf("git rm -r: %v %s", err, o)
		}
		out, err := git(persona, "commit", "-m", "tidy", "--", ConstitutionRepoMarker)
		assertConstitutionRefusal(t, out, err, ConstitutionRepoMarker+"/developer.md", gatesDirOf(persona))
	})

	t.Run("the marker replaced by a symlink to an empty directory", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		plantLaw(t, repo, git)
		marker := filepath.Join(repo, filepath.FromSlash(ConstitutionRepoMarker))
		if err := os.RemoveAll(marker); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(t.TempDir(), marker); err != nil {
			t.Fatal(err)
		}
		stageAt(t, repo, git, nil, ConstitutionSourceDir+"/config.yaml", "drafted\n")
		out, err := git(persona, "commit", "-m", "edit the law", "--", ConstitutionSourceDir+"/config.yaml")
		assertConstitutionRefusal(t, out, err, ConstitutionSourceDir+"/config.yaml", gatesDirOf(persona))
	})
}

// `--no-verify` bypasses pre-commit and commit-msg. This wall is a
// prepare-commit-msg hook, and the whole L3 tier rests on that not being the
// same list — stated everywhere, measured nowhere until here.
func TestQACommitWallIsNotSkippedByNoVerify(t *testing.T) {
	t.Parallel()
	repo, git, persona := constitutionWallRepo(t, true)
	stageAt(t, repo, git, nil, ConstitutionSourceDir+"/config.yaml", "drafted\n")
	out, err := git(persona, "commit", "--no-verify", "-m", "edit the law", "--", ConstitutionSourceDir+"/config.yaml")
	assertConstitutionRefusal(t, out, err, ConstitutionSourceDir+"/config.yaml", gatesDirOf(persona))
}
