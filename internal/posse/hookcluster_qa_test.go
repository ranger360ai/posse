package posse

// THE L3 COMMIT HOOK READS THE STAGED SET WITH MOVE DETECTION OFF, AND
// CLASSIFIES ADDED ENTRIES (ranger-base-dmsbu, folding ranger-base-60azj,
// ranger-base-qdxe, ranger-base-jex3 and ranger-base-4b1z4).
//
// Five defects, one shape between them: every arm of this hook asks git what
// is staged, and git's answer changes under rename detection — ON by default
// since 2.9. A move prints only its DESTINATION to --name-only, pairs a
// source and a destination inside one pathspec into a single R100 entry that
// '^A' never matches, and makes --diff-filter=A print nothing at all. Four
// walls read that answer and believed it.
//
//	check 1 (docs genre)      docs/ -> docs/ move into an unlisted genre
//	                          committed clean; the public tree gained the
//	                          docs/rca/ ADR 0024 D1 says it must never have.
//	check 2 (ops prose)       git pathspecs are case-sensitive, so '*.md'
//	                          never saw x.MD or x.markdown — a different
//	                          defect, same arm, folded here.
//	check 3 (identity)        the literals were grepped over ADDED LINES
//	                          only, so a filename carrying the operator, and
//	                          a pure move that yields NO plus lines at all,
//	                          both landed.
//	constitution arm          a move OUT of the class was invisible: the PID
//	                          gone from the constitution repo at exit 0.
//	constitution marker       the class detector stat'd the WORKTREE, which
//	                          a persona owns — rm -rf it, never stage it, and
//	                          the promoted set drops out of the class.
//
// EVERY PIN HERE IS MUTATION-CHECKED, per alternative, because a green pin
// over a wall that never had the hole measures nothing (probe-needs-a-
// failing-wrong-arm). What each mutant is and what it reds is recorded at
// each pin; the runs are on ranger-base-dmsbu.
//
// The siblings these sit beside: TestDocsGenreAndProseGuardHook and
// TestIdentityLiteralGuardHook (visibility_test.go) own the non-move shapes
// of checks 1-3, and constitutionwall_qa_test.go owns the constitution arm's
// class. This file owns what those miss, and borrows their helpers rather
// than re-scaffolding: commitWallRepo / constitutionWallRepo / stageAt /
// assertConstitutionRefusal.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the shared scaffolding for the visibility arm's three checks ─────────

// visWall is a public- and a private-stamped scratch repo with the REAL
// rendered hook installed in each, in the shape TestIdentityLiteralGuardHook
// uses. $HOME is this process's for the duration because
// DeriveIdentityLiterals reads it directly (AbbrevHome): without that the
// abbrev and absolute instance paths are the same string, dedupe drops one,
// and the tilde-form pin below has nothing to measure.
type visWall struct {
	pub, priv, gates, home string
	instance               string
	git                    func(repo string, env []string, args ...string) (string, error)
	persona                []string
	identity               []IdentityLiteral
}

func newVisWall(t *testing.T) *visWall { return newVisWallNamed(t, "instance") }

// newVisWallNamed is newVisWall with the instance directory's NAME under the
// caller's control, which is the only handle a test has on what the derived
// literals actually contain — the username and the git email are the box's.
func newVisWallNamed(t *testing.T, instanceDir string) *visWall {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	w := &visWall{
		home:     home,
		gates:    t.TempDir(),
		pub:      filepath.Join(home, "pub"),
		priv:     filepath.Join(home, "priv"),
		instance: filepath.Join(home, instanceDir),
	}
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+w.pub+": public\n  "+w.priv+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: cfg}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	w.git = func(repo string, env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	w.persona = []string{"RHQ_PERSONA=tester", "RHQ_GATES_DIR=" + w.gates}
	for _, repo := range []string{w.pub, w.priv} {
		write(t, filepath.Join(repo, ".beads", "redirect"), filepath.Join(w.instance, ".beads")+"\n")
		if out, err := w.git(repo, nil, "init", "-q", "-b", "main"); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
			t.Fatal(err)
		}
	}
	w.identity = testIdentity(t, w.pub)
	return w
}

// literal digs one derived class's value out, and fails the FIXTURE if the
// box did not produce it — a pin that silently measured an empty literal
// would be green over any wall at all.
func (w *visWall) literal(t *testing.T, class string) string {
	t.Helper()
	for _, l := range w.identity {
		if l.Class == class {
			return l.Value
		}
	}
	t.Fatalf("fixture premise: this box must derive a %q literal, got %+v", class, w.identity)
	return ""
}

// stage writes body at rel and stages exactly that path.
func (w *visWall) stage(t *testing.T, repo, rel, body string) {
	t.Helper()
	write(t, filepath.Join(repo, filepath.FromSlash(rel)), body)
	if out, err := w.git(repo, nil, "add", "--", rel); err != nil {
		t.Fatalf("git add %s: %v %s", rel, err, out)
	}
}

// plant commits a path with the hook bypassed outright. It is only ever used
// to build HISTORY a pin then measures against — a file that is already in
// the tree, which by construction cannot be staged through the wall that is
// under test. Using the override for this instead would put OVERRIDDEN lines
// in refusals.log that the log assertions would then have to explain away.
func (w *visWall) plant(t *testing.T, repo, rel, body string) {
	t.Helper()
	w.stage(t, repo, rel, body)
	if out, err := w.git(repo, nil, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "planted", "--", rel); err != nil {
		t.Fatalf("planting %s: %v %s", rel, err, out)
	}
}

func (w *visWall) log(t *testing.T) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(w.gates, "refusals.log"))
	return string(b)
}

// ─── check 1: a move into an unlisted genre (ranger-base-60azj) ───────────

// ADR 0024 D1's stated consequence is that the public tree has no docs/rca/,
// docs/incidents/ or docs/postmortems/ — not that new files under docs/ are
// A entries, which was only the mechanism check 1 was written with. Git
// pairs a source and a destination that are BOTH inside the 'docs/*'
// pathspec into one R100 entry, and '^A' never matched it, so `git mv
// docs/adr/x.md docs/rca/x.md` committed clean while the identical file
// added fresh was refused. The asymmetry is what hid it for a release: a
// source OUTSIDE docs/ is hidden by the pathspec, git degrades the pair to
// an A, and the wall fired correctly.
//
// MUTATION-CHECKED: with --no-renames removed from check 1's listing this
// pin goes RED (the commit lands), and so does the drift pin at the bottom
// of this file; nothing else here moves — so it measures the flag and not
// the genre rule.
func TestQADocsGenreWallSeesAMoveIntoAnUnlistedGenre(t *testing.T) {
	w := newVisWall(t)

	// A committed, clean, allowlisted doc — big enough that git's 50%
	// similarity gate is not what the pin depends on, and byte-identical
	// across the move so git pairs it at R100 whatever its size.
	body := strings.Repeat("a line of perfectly public prose\n", 40)
	w.plant(t, w.pub, "docs/adr/0001-a.md", body)

	// git mv will not create the destination's parent.
	if err := os.MkdirAll(filepath.Join(w.pub, "docs", "rca"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := w.git(w.pub, nil, "mv", "docs/adr/0001-a.md", "docs/rca/0001-a.md"); err != nil {
		t.Fatalf("git mv: %v %s", err, out)
	}
	// The fixture premise, asserted rather than assumed: git really did
	// report this as a rename. A fixture git declined to pair would make the
	// pin green for the wrong reason.
	if ns, _ := w.git(w.pub, nil, "diff", "--cached", "--name-status", "HEAD", "--", "docs/*"); !strings.HasPrefix(ns, "R") {
		t.Fatalf("fixture premise: git must report this move as a rename, got %q", ns)
	}

	out, err := w.git(w.pub, w.persona, "commit", "-m", "move it", "--", "docs/adr/0001-a.md", "docs/rca/0001-a.md")
	if err == nil {
		t.Fatalf("a move into an unlisted genre must be refused, not committed:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: a new docs/ file outside the public genre allowlist",
		"docs/rca/0001-a.md",
		"(genre: rca)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q:\n%s", want, out)
		}
	}
	if o, _ := w.git(w.pub, nil, "log", "--oneline", "-1", "--name-only"); strings.Contains(o, "docs/rca/") {
		t.Errorf("nothing may have landed:\n%s", o)
	}

	// The move's SOURCE is not the refusal's business: a move that lands
	// inside the allowlist is still an ordinary commit.
	if err := os.MkdirAll(filepath.Join(w.pub, "docs", "notes.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := w.git(w.pub, nil, "mv", "docs/rca/0001-a.md", "docs/notes.d/0001-a.md"); err != nil {
		t.Fatalf("git mv: %v %s", err, out)
	}
	// docs/rca/ is out of the pathspec now: the second mv took its index
	// entry with it, and git validates a pathspec list all-or-nothing.
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "move it back", "--",
		"docs/adr/0001-a.md", "docs/notes.d/0001-a.md"); err != nil {
		t.Errorf("a move into an ALLOWLISTED genre must still commit: %v\n%s", err, out)
	}
}

// A move is a new entry, and a MODIFICATION is not: an existing file under
// an unlisted genre — one that predates the genre allowlist, or arrived by
// the override — stays editable. This is check 1's own rule ("a MODIFIED
// existing file already cleared this the day it was added"), and it is the
// thing --no-renames must not quietly change: with the flag on, a rename is
// reported as a delete plus an add, and nothing else moves.
func TestQADocsGenreWallStillTakesAModificationInAnUnlistedGenre(t *testing.T) {
	w := newVisWall(t)
	w.plant(t, w.pub, "docs/rca/legacy.md", "already here\n")
	w.stage(t, w.pub, "docs/rca/legacy.md", "already here\nand edited\n")
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "edit", "--", "docs/rca/legacy.md"); err != nil {
		t.Errorf("check 1 is added-entries only; a modification must commit: %v\n%s", err, out)
	}
	// And a DELETION of one is not an added entry either.
	if out, err := w.git(w.pub, nil, "rm", "-q", "--", "docs/rca/legacy.md"); err != nil {
		t.Fatalf("git rm: %v %s", err, out)
	}
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "remove", "--", "docs/rca/legacy.md"); err != nil {
		t.Errorf("a deletion carries a path away; it must commit: %v\n%s", err, out)
	}
}

// ─── check 2: markdown spellings (ranger-base-4b1z4) ──────────────────────

// Git pathspec matching is case-sensitive, so the wall's '*.md' did not
// match .MD — nor .markdown — and `git diff --cached -U0 HEAD -- '*.md'`
// came back EMPTY for those files: check 2 never saw them at all. One
// character carried ops-class content into a public tree, and on macOS's
// default case-insensitive filesystem the two spellings are the same file to
// everything except git.
//
// The body is identical in every arm, so the only variable is the extension.
//
// MUTATION-CHECKED: MarkdownPathspecs back to []string{"*.md"} reds the .MD
// and .markdown arms and leaves the .md arm green — the case-sensitivity is
// what is measured, not the pattern list.
func TestQAMarkdownScanOwnsEveryMarkdownSpelling(t *testing.T) {
	w := newVisWall(t)
	const opsBody = "# a note\n\nthe pilot cost $715/wk to run.\n"

	for _, rel := range []string{
		// One basename per arm, not one file in four spellings: on macOS's
		// default case-insensitive filesystem x.md and x.MD ARE the same
		// file, so a shared stem would measure the filesystem instead of
		// the pathspec.
		"docs/adr/0009-a.md",       // the positive control: this one always worked
		"docs/adr/0009-b.MD",       // ranger-base-4b1z4's THROUGH arm
		"docs/adr/0009-c.markdown", // its other THROUGH arm
		"docs/adr/0009-d.Markdown", // mixed case, the same rule
	} {
		t.Run(rel, func(t *testing.T) {
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
			// Unstage, so each arm measures its own extension alone.
			if out, err := w.git(w.pub, nil, "rm", "-q", "--cached", "--", rel); err != nil {
				t.Fatalf("git rm --cached: %v %s", err, out)
			}
		})
	}

	// The list is a decision and not a wildcard: a spelling nothing writes
	// is still not markdown to this wall, and the identical body in a .txt
	// commits clean. That is check 2's documented scope (check 3 is what
	// covers non-markdown), and the arm that would go red if someone
	// "fixed" the case-sensitivity by dropping the extension test entirely.
	w.stage(t, w.pub, "notes.txt", opsBody)
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", "notes.txt"); err != nil {
		t.Errorf("check 2 is markdown-only: a .txt must commit: %v\n%s", err, out)
	}
}

// The rendered hook must carry the Go list and nothing hand-spelled beside
// it: one place the way OpsPatterns is (ranger-base-4b1z4's own ask).
func TestQAMarkdownPathspecsAreRenderedFromTheGoList(t *testing.T) {
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	for _, p := range MarkdownPathspecs {
		if !strings.Contains(render, "'"+p+"'") {
			t.Errorf("the rendered hook must carry the pathspec %q from MarkdownPathspecs", p)
		}
	}
	if strings.Contains(render, "-- '*.md'") {
		t.Errorf("a bare case-sensitive '*.md' pathspec is back in the hook — ranger-base-4b1z4")
	}
}

// ─── check 3: identity literals in an ADDED PATH (ranger-base-dmsbu) ──────

// DONE WHEN (a) and (b): a filename is exactly where an operator-shaped
// artifact puts the operator, and "ADDED lines" was the mechanism check 3
// was written with rather than its intent. Two shapes committed clean with
// the content arm alone — a new file under an ALLOWLISTED genre whose
// content is spotless, and a pure move of an already-clean file, which
// yields no plus lines at all, +++ header included.
//
// MUTATION-CHECKED, per alternative:
//   - drop the whole added-paths arm (posse_ipaths=”): both subtests go
//     red, and so do the tilde and non-ASCII pins below.
//   - drop --no-renames from its listing: the MOVE subtest goes red (the
//     commit lands) and the new-file subtest stays green. That is the pin
//     ranger-base-dmsbu names explicitly, because with detection ON
//     --diff-filter=A prints NOTHING for a pure move.
//   - drop --diff-filter=A: nothing reds here, and
//     TestQAIdentityPathArmIsAddedEntriesOnly reds instead.
func TestQAIdentityLiteralGuardScansAddedPaths(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")

	t.Run("a new file whose PATH carries the username, content clean", func(t *testing.T) {
		// docs/runbooks/ is allowlisted, so check 1 is not what refuses
		// this; the content carries nothing identifying, so neither is the
		// content half of check 3.
		rel := "docs/runbooks/" + username + ".md"
		w.stage(t, w.pub, rel, "# a runbook\n\nnothing identifying in here at all.\n")
		out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
		if err == nil {
			t.Fatalf("an identity literal in a staged PATH must be refused:\n%s", out)
		}
		for _, want := range []string{
			"refused by posse gate: an operator identity literal in a staged PATH",
			"the FILENAME, not its content",
			"username:", // the class
			rel,         // the path itself
			"ADR 0024 D2 check 3",
			"For a PATH: name the file",
			VisibilityOverrideEnv + "=" + VisibilityOverrideValue,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the path refusal must carry %q:\n%s", want, out)
			}
		}
		// It must NOT read as a content hit — that is the whole reason this
		// arm has its own wording (ranger-base-dmsbu MECHANICS).
		if strings.Contains(out, "matched in the staged additions:") {
			t.Errorf("a PATH hit must not be reported as a content hit:\n%s", out)
		}

		before := strings.Count(w.log(t), "identity literal scan")
		if out, err := w.git(w.pub, append(w.persona, VisibilityOverrideEnv+"="+VisibilityOverrideValue),
			"commit", "-m", "x", "--", rel); err != nil || !strings.Contains(out, "OVERRIDDEN") {
			t.Fatalf("the operator's override must pass, and say so: %v\n%s", err, out)
		}
		if after := strings.Count(w.log(t), "identity literal scan"); after != before+1 {
			t.Errorf("the override must log exactly one line, got %d new:\n%s", after-before, w.log(t))
		}
		if !strings.Contains(w.log(t), "identity literal scan [prepare-commit-msg hook] (public repo, staged path)") {
			t.Errorf("the refusal must be logged, and say it was a path:\n%s", w.log(t))
		}

		// The same commit in a private-stamped repository lands clean.
		w.stage(t, w.priv, rel, "# a runbook\n\nnothing identifying in here at all.\n")
		if out, err := w.git(w.priv, w.persona, "commit", "-m", "x", "--", rel); err != nil {
			t.Errorf("a private repo must take the same path: %v\n%s", err, out)
		}
	})

	t.Run("a pure move to a path carrying the username", func(t *testing.T) {
		// 40 lines, byte-identical across the move: git pairs it R100 and
		// the diff has ZERO plus lines, so the content arm cannot see it.
		w.plant(t, w.pub, "docs/runbooks/deploy.md", strings.Repeat("a public line\n", 40))
		dst := username + ".txt"
		if out, err := w.git(w.pub, nil, "mv", "docs/runbooks/deploy.md", dst); err != nil {
			t.Fatalf("git mv: %v %s", err, out)
		}
		// Both fixture premises, asserted: git reports a rename, and the
		// added-lines reader this arm exists to supplement sees NOTHING.
		if ns, _ := w.git(w.pub, nil, "diff", "--cached", "--name-status", "HEAD"); !strings.HasPrefix(ns, "R") {
			t.Fatalf("fixture premise: git must report this move as a rename, got %q", ns)
		}
		if d, _ := w.git(w.pub, nil, "diff", "--cached", "-U0", "HEAD"); strings.Contains(d, "\n+") {
			t.Fatalf("fixture premise: a pure move must yield no added lines, got:\n%s", d)
		}
		out, err := w.git(w.pub, w.persona, "commit", "-m", "move", "--", "docs/runbooks/deploy.md", dst)
		if err == nil {
			t.Fatalf("a move to a path carrying the username must be refused:\n%s", out)
		}
		if !strings.Contains(out, "an operator identity literal in a staged PATH") || !strings.Contains(out, dst) {
			t.Errorf("the refusal must name the move's destination:\n%s", out)
		}
	})
}

// DONE WHEN (c): the arm is added ENTRIES, exactly as check 1 is added
// files. A path already in history cleared this the day it arrived, and a
// deletion carries a path AWAY — refusing that would be the wrong direction,
// and the lint is not a purge.
//
// MUTATION-CHECKED: dropping --diff-filter=A from the listing reds both
// halves (the modification and the deletion are refused), which is the only
// pin that flag has.
func TestQAIdentityPathArmIsAddedEntriesOnly(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	rel := "docs/runbooks/" + username + "-legacy.md"
	w.plant(t, w.pub, rel, "already in history\n")

	w.stage(t, w.pub, rel, "already in history\nand edited, cleanly\n")
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "edit", "--", rel); err != nil {
		t.Errorf("modifying an already-committed identity-bearing path must commit: %v\n%s", err, out)
	}
	if out, err := w.git(w.pub, nil, "rm", "-q", "--", rel); err != nil {
		t.Fatalf("git rm: %v %s", err, out)
	}
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "remove", "--", rel); err != nil {
		t.Errorf("a deletion carries the path away; it must commit: %v\n%s", err, out)
	}
}

// DONE WHEN (d): the instance path's TILDE form is a literal like any other
// — the shape a mis-quoted `~` makes on disk is a real directory called `~`
// — and the bead-id-prefix non-trip property (ranger-base-gk6e) has to hold
// for paths too, not only for content.
//
// MUTATION-CHECKED: with DeriveIdentityLiterals no longer adding the
// AbbrevHome form — so only the absolute instance path is rendered — this
// pin goes red, together with the non-ASCII pin below, which reads the same
// literal.
func TestQAIdentityPathArmSeesTheTildeForm(t *testing.T) {
	w := newVisWall(t)
	abbrev := w.literal(t, "instance-path")
	if !strings.HasPrefix(abbrev, "~/") {
		t.Fatalf("fixture premise: the abbrev instance path must be a tilde form, got %q", abbrev)
	}

	// A staged path carrying the tilde form verbatim — `backup/~/instance/x`
	// is what a shell that did not expand the tilde leaves behind.
	rel := "backup/" + abbrev + "/notes.md"
	w.stage(t, w.pub, rel, "clean content\n")
	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
	if err == nil {
		t.Fatalf("the instance path's tilde form in a staged path must be refused:\n%s", out)
	}
	if !strings.Contains(out, "instance-path:") || !strings.Contains(out, "an operator identity literal in a staged PATH") {
		t.Errorf("the refusal must name the instance-path class:\n%s", out)
	}
}

// The other half of DONE WHEN (d), and a NEGATIVE control: the wall must not
// over-fire. The rendered ERE is the WHOLE literal, slashes and tilde
// included (identityLiteralERE), so a path that is nothing but a bead id is
// a substring miss even on a box whose instance directory IS that bead id.
// This is TestIdentityLiteralDoesNotTripOnABareBeadID's property, carried
// over to the path arm ranger-base-dmsbu added.
//
// MUTATION-CHECKED: with identityLiteralERE loosened to the literal's last
// path segment — the plausible wrong implementation, "match the instance
// directory's name" — this pin goes red, and so does the content twin. The
// fixture directory is named exactly the bead id for that reason: an
// instance-<id> name would leave the mutant alive and the pin measuring
// nothing.
func TestQAIdentityPathArmDoesNotTripOnABareBeadID(t *testing.T) {
	w := newVisWallNamed(t, "ranger-base-gk6e")
	if abs := w.literal(t, "instance-path-abs"); !strings.HasSuffix(abs, "/ranger-base-gk6e") {
		t.Fatalf("fixture premise: the instance path must END in the bead id, got %q", abs)
	}
	const rel = "docs/notes.d/ranger-base-gk6e.md"
	w.stage(t, w.pub, rel, "a note about the bead\n")
	if out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel); err != nil {
		t.Errorf("a path that is a bare bead id must not trip the instance-path literal: %v\n%s", err, out)
	}
}

// core.quotePath=false is load-bearing on the path arm and nothing else in
// this file measures it: git's default C-quotes every non-ASCII byte in a
// path as an octal escape, and the rendered literal is the raw UTF-8 the
// derivation read off the box. The escaped spelling does not match it, so
// with quotePath at its default an instance path carrying one non-ASCII
// character walks a staged path straight past the wall.
//
// The instance directory's name is the only handle a test has on the CONTENT
// of a derived literal — the username and the git email belong to the box.
//
// RESIDUAL, deliberately not pinned because it cannot be closed at this
// layer: git C-quotes a path holding a double quote, a backslash or a
// control byte whatever quotePath says (ranger-base-qg0k8). Only a literal
// that itself carries one of those bytes could be missed that way, and none
// of check 3's three sources produces one.
//
// MUTATION-CHECKED: dropping -c core.quotePath=false from the listing reds
// this pin and nothing else in this file.
func TestQAIdentityPathArmMatchesANonASCIILiteralRaw(t *testing.T) {
	w := newVisWallNamed(t, "inst\u00e4nce")
	w.plant(t, w.pub, "seed.txt", "seed\n") // a HEAD to diff the premise against
	abbrev := w.literal(t, "instance-path")
	if !strings.Contains(abbrev, "\u00e4") {
		t.Fatalf("fixture premise: the instance path must carry a non-ASCII byte, got %q", abbrev)
	}
	rel := "backup/" + abbrev + "/notes.md"
	w.stage(t, w.pub, rel, "clean content\n")
	// The premise, asserted: git's DEFAULT really does escape this path, so
	// the pin is measuring the flag and not a path git prints raw anyway.
	if out, _ := w.git(w.pub, nil, "diff", "--cached", "--name-only", "--diff-filter=A", "HEAD"); !strings.Contains(out, "\\303\\244") {
		t.Fatalf("fixture premise: git must octal-escape this path by default, got %q", out)
	}
	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
	if err == nil {
		t.Fatalf("a non-ASCII identity literal in a staged path must be refused:\n%s", out)
	}
	if !strings.Contains(out, "an operator identity literal in a staged PATH") {
		t.Errorf("the refusal must come from the path arm:\n%s", out)
	}
}

// DONE WHEN (e), as a standing control rather than a one-shot measurement
// (state-close-has-a-shelf-life): this box's own identity literals must not
// appear in any TRACKED PATH of this repo. The content twin —
// TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned — has a
// dispositioned list because the queue cutover put the shared queue repo's
// conventional path in three files. There is no such list here, and there
// should never need to be one: a path is renameable.
func TestQAIdentityLiteralsNeverAppearInATrackedPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not inside a git checkout")
	}
	repo := strings.TrimSpace(string(top))
	identity, err := DeriveIdentityLiterals(hookRepo(repo))
	if err != nil {
		t.Fatal(err)
	}
	if len(identity) == 0 {
		t.Skip("this box derives no identity literals — nothing to measure")
	}
	out, err := exec.Command("git", "-C", repo, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files did not run: %v", err)
	}
	paths := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(paths) < 100 {
		t.Fatalf("census premise: git ls-files returned %d paths, which is not this repo", len(paths))
	}
	for _, lit := range identity {
		for _, p := range paths {
			if p != "" && strings.Contains(p, lit.Value) {
				t.Errorf("identity literal (%s = %q) appears in the tracked PATH %s — rename it", lit.Class, lit.Value, p)
			}
		}
	}
	t.Logf("censused %d tracked paths against %d identity literals", len(paths), len(identity))
}

// ─── the constitution arm (ranger-base-qdxe, ranger-base-jex3) ────────────

// ranger-base-qdxe: the arm read `git diff --cached --name-only -z` with
// rename detection ON, and --name-only prints only a rename's DESTINATION.
// So a move of a class path to a NON-class path showed the wall nothing in
// the class and committed at exit 0 — `git ls-tree -r HEAD -- rhq/` empty,
// the PID gone from the constitution repo. It needs no `git mv`: detection
// pairs any staged delete with a similar staged add, so copy-then-remove
// does it too.
//
// MUTATION-CHECKED: with --no-renames removed from the constitution arm's
// listing both subtests go red (the commits land), and every pin in
// constitutionwall_qa_test.go stays green — which is exactly the gap: the
// class was fully pinned and the READER was not.
func TestQAConstitutionWallRefusesAMoveOutOfTheClass(t *testing.T) {
	t.Run("a promoted path in the constitution repo", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		rel := ConstitutionRepoMarker + "/developer.md"
		stageAt(t, repo, git, nil, rel, strings.Repeat("deny rules\n", 40))
		if out, err := git(nil, "commit", "-qm", "operator installs it", "--", rel); err != nil {
			t.Fatalf("fixture commit: %v %s", err, out)
		}
		if out, err := git(nil, "mv", rel, "notes.md"); err != nil {
			t.Fatalf("git mv: %v %s", err, out)
		}
		if ns, _ := git(nil, "diff", "--cached", "--name-status", "HEAD"); !strings.HasPrefix(ns, "R") {
			t.Fatalf("fixture premise: git must report this move as a rename, got %q", ns)
		}
		out, err := git(persona, "commit", "-m", "move it", "--", rel, "notes.md")
		assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
		if tree, _ := git(nil, "ls-tree", "-r", "--name-only", "HEAD", "--", ConstitutionSourceDir); !strings.Contains(tree, rel) {
			t.Errorf("the class path must still be in HEAD, got %q", tree)
		}
	})

	// The settings file is the every-repo half of the class (ranger-base-az93):
	// a persona that can DELETE it un-fences itself as surely as one that can
	// edit it, and this route worked in every hooked repo, not only the
	// constitution one.
	t.Run("the settings file in an ordinary repo", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, false)
		stageAt(t, repo, git, nil, ClaudeProjectConfig, strings.Repeat("{\"deny\": \"rules\"}\n", 40))
		if out, err := git(nil, "commit", "-qm", "operator installs it", "--", ClaudeProjectConfig); err != nil {
			t.Fatalf("fixture commit: %v %s", err, out)
		}
		if out, err := git(nil, "mv", ClaudeProjectConfig, "settings-draft.json"); err != nil {
			t.Fatalf("git mv: %v %s", err, out)
		}
		out, err := git(persona, "commit", "-m", "draft", "--", ClaudeProjectConfig, "settings-draft.json")
		assertConstitutionRefusal(t, out, err, ClaudeProjectConfig, gatesDirOf(persona))
	})

	// It is the RENAME that was invisible, not the move: copy-then-remove
	// reaches the same place because detection pairs any similar staged
	// delete with a staged add, and it is what an attacker types when `git
	// mv` is the thing being watched.
	t.Run("copy then remove, no git mv", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		rel := ConstitutionRepoMarker + "/developer.md"
		body := strings.Repeat("deny rules\n", 40)
		stageAt(t, repo, git, nil, rel, body)
		if out, err := git(nil, "commit", "-qm", "operator installs it", "--", rel); err != nil {
			t.Fatalf("fixture commit: %v %s", err, out)
		}
		write(t, filepath.Join(repo, "copy.md"), body)
		if err := os.Remove(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
		if out, err := git(nil, "add", "--", "copy.md", rel); err != nil {
			t.Fatalf("git add: %v %s", err, out)
		}
		out, err := git(persona, "commit", "-m", "tidy", "--", "copy.md", rel)
		assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
	})
}

// ranger-base-jex3: the class detector was `[ -d "$top/rhq/agents" ]`
// against the WORKING TREE, and a persona owns its own working tree. `rm -rf
// rhq/agents` — never staged, so nothing on the branch records it — dropped
// the promoted set out of the class and the identical refused commit landed.
// Replacing the directory with a FILE did the same, since -d is false either
// way. The belt (MergeSessionWork, which reads the main checkout) held, which
// is why it was P2; the L3 arm did not.
//
// The two beads compound, which is why they are fixed together: renaming the
// LAST file out of rhq/agents removes the directory, so the marker defeat
// then holds for every later commit in that checkout with no rm -rf typed.
//
// MUTATION-CHECKED: with the base-tree arm removed from the OR both attack
// subtests go red (the commits land) and the genesis subtest stays green —
// so the pin measures the new arm and not the marker rule.
func TestQAConstitutionWallReadsTheMarkerFromTheBaseTreeToo(t *testing.T) {
	// The class member the attack is after: config.yaml, which az93 is
	// about and which is in the class only when the marker says so.
	plantMarker := func(t *testing.T, repo string, git func(env []string, args ...string) (string, error)) {
		t.Helper()
		rel := ConstitutionRepoMarker + "/developer.md"
		stageAt(t, repo, git, nil, rel, "the law\n")
		if out, err := git(nil, "commit", "-qm", "operator installs the law", "--", rel); err != nil {
			t.Fatalf("fixture commit: %v %s", err, out)
		}
	}

	t.Run("the marker directory removed from the worktree", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		plantMarker(t, repo, git)
		// The control first: with the marker on disk, this is refused.
		stageAt(t, repo, git, nil, "rhq/config.yaml", "drafted\n")
		out, err := git(persona, "commit", "-m", "edit the law", "--", "rhq/config.yaml")
		assertConstitutionRefusal(t, out, err, "rhq/config.yaml", gatesDirOf(persona))

		// The attack: remove the marker from the WORKING TREE only. Nothing
		// is staged, so the branch records nothing — and the base tree still
		// carries it.
		if err := os.RemoveAll(filepath.Join(repo, filepath.FromSlash(ConstitutionRepoMarker))); err != nil {
			t.Fatal(err)
		}
		if st, _ := git(nil, "status", "--porcelain", "--", ConstitutionRepoMarker); strings.Contains(st, "D  ") {
			t.Fatalf("fixture premise: the removal must be unstaged, got %q", st)
		}
		out, err = git(persona, "commit", "-m", "edit the law", "--", "rhq/config.yaml")
		assertConstitutionRefusal(t, out, err, "rhq/config.yaml", gatesDirOf(persona))
	})

	// -d is false for a FILE too, which is the second spelling of the same
	// defeat; the base arm asks for a TREE, so it answers this one as well.
	t.Run("the marker directory replaced by a file", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, true)
		plantMarker(t, repo, git)
		marker := filepath.Join(repo, filepath.FromSlash(ConstitutionRepoMarker))
		if err := os.RemoveAll(marker); err != nil {
			t.Fatal(err)
		}
		write(t, marker, "not a directory\n")
		stageAt(t, repo, git, nil, "rhq/recipes/x.md", "drafted\n")
		out, err := git(persona, "commit", "-m", "edit the law", "--", "rhq/recipes/x.md")
		assertConstitutionRefusal(t, out, err, "rhq/recipes/x.md", gatesDirOf(persona))
	})

	// The direction the worktree arm was there for, unchanged: at repo
	// GENESIS the base tree is empty, so only the dir on disk can say this
	// is a constitution — and it does.
	t.Run("repo genesis, no HEAD, marker on disk only", func(t *testing.T) {
		repo, git, persona := commitWallRepo(t)
		if err := os.MkdirAll(filepath.Join(repo, ConstitutionRepoMarker), 0o755); err != nil {
			t.Fatal(err)
		}
		stageAt(t, repo, git, nil, "rhq/config.yaml", "drafted\n")
		out, err := git(persona, "commit", "-m", "first", "--", "rhq/config.yaml")
		assertConstitutionRefusal(t, out, err, "rhq/config.yaml", gatesDirOf(persona))
	})

	// And the negative arm, so the OR did not simply widen the class into
	// every repo: a repo that has the marker NEITHER on disk NOR in its base
	// tree still takes a promoted-set path. (The settings files stay refused
	// there — that is class item b, and TestQAConstitutionWallScopesThe...
	// owns it.)
	t.Run("no marker anywhere", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, false)
		stageAt(t, repo, git, nil, "rhq/config.yaml", "drafted\n")
		if out, err := git(persona, "commit", "-m", "not the law here", "--", "rhq/config.yaml"); err != nil {
			t.Errorf("a repo that is not the constitution must take rhq/config.yaml: %v\n%s", err, out)
		}
	})
}

// ─── the drift pin the whole cluster is really about ──────────────────────

// Every reader in this hook that asks git for staged PATH NAMES must disable
// move detection. Four of them did not, at four different times, and each
// one was found by a separate escape rather than by the last one's fix —
// which is what makes this a property worth pinning rather than four
// comments (ranger-base-x9xbk, pp7k1, 60azj, qdxe, dmsbu).
//
// The CONTENT readers (-U0, checks 0, 2 and 3's first arm) deliberately keep
// detection ON: with it off a pure move re-presents the whole file as added
// lines, and re-scanning content that is already in history is the thing
// check 1's "already cleared this the day it was added" rule rejects.
//
// MUTATION-CHECKED: removing --no-renames from any one of the four name
// readers reds this pin naming that line.
func TestQAHookReadersAllDisableMoveDetection(t *testing.T) {
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{},
		IdentityLiteral{Class: "username", Value: "someone"})
	var names, content int
	for _, line := range strings.Split(render, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "diff --cached") {
			continue
		}
		isName := strings.Contains(trimmed, "--name-only") || strings.Contains(trimmed, "--name-status")
		if !isName {
			if !strings.Contains(trimmed, "-U0") {
				t.Errorf("a staged reader that is neither a name reader nor a -U0 content reader — classify it here: %s", trimmed)
			}
			content++
			continue
		}
		names++
		if !strings.Contains(trimmed, "--no-renames") {
			t.Errorf("a staged-name reader without --no-renames: a move prints only its destination and the arm goes blind:\n  %s", trimmed)
		}
	}
	// A census that counted nothing must not read as a clean one.
	if names < 5 {
		t.Errorf("expected at least 5 staged-name readers in the hook (check 1, check 3's path arm, the constitution arm, the shared-index reader, the NOTES.md arm), found %d", names)
	}
	if content < 3 {
		t.Errorf("expected the three -U0 content readers (checks 0, 2, 3), found %d", content)
	}
}
