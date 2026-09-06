//go:build !posse_arm2 && !posse_arm3

package posse

// QA, rangerhq-40ig — adversarial verification of the shared-index
// commit wall (rangerhq-lmq9). What is pinned here is what I attacked and
// what survived; self-contained (own repo, own fixtures) so it stands
// whatever the next persona does to gates_test.go.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qaCommitRepo is a fresh repo with one commit and a git runner whose env is
// explicit — no RHQ_PERSONA leaks in from the pane the suite runs in, and
// PathOutsideGates drops this session's own shims (rangerhq-8sd).
func qaCommitRepo(t *testing.T) (repo string, git func(env []string, args ...string) (string, error), write func(string, string)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
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
	write = func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "a")
	write("b.txt", "b")
	git(nil, "add", "a.txt", "b.txt")
	if out, err := git(nil, "commit", "-qm", "init"); err != nil {
		t.Fatalf("init commit: %v %s", err, out)
	}
	return repo, git, write
}

// `git commit -i -- <paths>` (--include) is a FOURTH sweeping form: it
// commits the named paths ON TOP OF whatever is already in the shared
// index, and it carries a pathspec, so the L1 rule's qualifier is
// satisfied. Measured (git 2.39.3): it gets .git/index.lock, so the L3
// hook refuses it through its `-a` arm. This test asserts both halves —
// that the form really does sweep, and that the hook stops it.
func TestQACommitWallIncludeFormSweepsAndIsRefused(t *testing.T) {
	t.Parallel()
	// Half one: without the guard, -i takes the other persona's staged file.
	_, git, write := qaCommitRepo(t)
	write("a.txt", "theirs") // another persona's staged work
	git(nil, "add", "a.txt")
	write("b.txt", "mine")
	// The bundled spelling, because that is the one the L1 arm has to read
	// out of a cluster: `-im x` is `-i -m x` to git (measured, 2.39.3).
	if out, err := git(nil, "commit", "-im", "mine", "--", "b.txt"); err != nil {
		t.Fatalf("unguarded -i commit: %v %s", err, out)
	}
	if out, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(out, "a.txt") {
		t.Fatalf("premise: `git commit -im mine -- b.txt` must sweep a.txt, got %q", out)
	}

	// Half two: with the guard installed, the same argv is refused.
	repo2, git2, write2 := qaCommitRepo(t)
	gates := t.TempDir()
	if _, err := installCommitGuard(repo2); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + gates}
	write2("a.txt", "theirs")
	git2(nil, "add", "a.txt")
	write2("b.txt", "mine")
	for _, argv := range [][]string{
		{"commit", "-i", "-m", "x", "--", "b.txt"},
		{"commit", "--include", "-m", "x", "--", "b.txt"},
		{"commit", "-im", "x", "--", "b.txt"},
	} {
		out, err := git2(persona, argv...)
		// The hook has no argv — it discriminates on GIT_INDEX_FILE, and -i
		// gets .git/index.lock, the same arm that catches -a. So the form it
		// names has to cover both; naming this one "git commit -a" sent the
		// reader after a flag that is not on the line (rangerhq-ojnw).
		if err == nil || !strings.Contains(out, "refused by posse gate: git commit -a or -i") {
			t.Errorf("git %s must be refused, named as -a or -i (it sweeps): %v %s", strings.Join(argv, " "), err, out)
		}
	}
	if out, _ := git2(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" {
		t.Errorf("the other persona's staged entry must survive, got %q", out)
	}
}

// A refusal must leave the shared tree exactly as it found it: another
// persona's staged entry intact (content included) and no index.lock or
// next-index-* left behind. A wall that wedges the shared index for the
// whole crew would be worse than the sweep it prevents.
func TestQACommitWallRefusalLeavesSharedTreeIntact(t *testing.T) {
	t.Parallel()
	repo, git, write := qaCommitRepo(t)
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	write("a.txt", "theirs")
	git(nil, "add", "a.txt")
	write("b.txt", "mine")
	for _, argv := range [][]string{
		{"commit", "-m", "x"},
		{"commit", "-am", "x"},
		{"commit", "--amend", "--no-edit"},
		{"commit", "-m", "x", "--"},
	} {
		if out, err := git(persona, argv...); err == nil {
			t.Fatalf("git %s must be refused: %s", strings.Join(argv, " "), out)
		}
		if out, _ := git(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" {
			t.Errorf("after refused `git %s`, staged set changed: %q", strings.Join(argv, " "), out)
		}
		if out, _ := git(nil, "show", ":a.txt"); strings.TrimSpace(out) != "theirs" {
			t.Errorf("after refused `git %s`, staged content changed: %q", strings.Join(argv, " "), out)
		}
		leftovers, _ := filepath.Glob(filepath.Join(repo, ".git", "*.lock"))
		next, _ := filepath.Glob(filepath.Join(repo, ".git", "next-index-*"))
		if len(leftovers)+len(next) != 0 {
			t.Errorf("after refused `git %s`, lock residue: %v %v", strings.Join(argv, " "), leftovers, next)
		}
		if out, err := git(nil, "status", "--short"); err != nil {
			t.Errorf("tree wedged after refused `git %s`: %v %s", strings.Join(argv, " "), err, out)
		}
	}
	// And the way through still works after all that.
	if out, err := git(persona, "commit", "-m", "safe", "--", "b.txt"); err != nil {
		t.Errorf("path-limited commit must still pass: %v %s", err, out)
	}
}

// The L1 half catches --include too (rangerhq-ojnw). It did not: the rule
// asked only for `--` with an operand, and `git commit -i -- <paths>` has
// one while still committing the whole shared index. L3 caught it either
// way — but L1 is the layer that travels with the session into a repo that
// has no hook, which is the whole reason both layers exist.
//
// The bundled spelling is the fifth form and it is not theoretical:
// `git commit -im x -- b.txt` sweeps exactly as `-i` does (measured, git
// 2.39.3, in TestQACommitWallIncludeFormSweepsAndIsRefused's premise arm).
// The other side of that WAS an accepted false positive: `-mi` is the
// message "i" and was refused too. ranger-base-v3cu closed it — the scan
// pairs the value-taking options now (spoiler.ValueOpts) — and what is
// left of the class is one token shape further out: a value option behind
// a boolean in the SAME cluster (`-qmi`), where a glob cannot tell the
// message from the option. That still refuses, and refusing is the side to
// be wrong on: a false positive has a way through; a false negative is the
// wall not being there.
func TestQACommitWallL1IncludeForm(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	os.WriteFile(filepath.Join(realBin, "git"), []byte("#!/bin/sh\necho \"real git $*\"\n"), 0o755)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, binDir, _, err := a.RenderGates("qa", []string{"Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	run := func(argv ...string) (string, int) {
		cmd := exec.Command(filepath.Join(binDir, "git"), argv...)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return string(out), code
	}
	for _, argv := range [][]string{
		{"commit", "-i", "-m", "x", "--", "a.go"},
		{"commit", "--include", "-m", "x", "--", "a.go"},
		{"commit", "-im", "x", "--", "a.go"},
		{"-C", "/tmp", "commit", "-i", "-m", "x", "--", "a.go"},
	} {
		out, code := run(argv...)
		if code != 1 || !strings.Contains(out, "refused by posse gate") {
			t.Errorf("git %s must be refused at L1 (it commits the shared index): %d %s", strings.Join(argv, " "), code, out)
		}
		if !strings.Contains(out, "and without -i/--include") {
			t.Errorf("git %s: the refusal must name the option that spoiled the qualifier, got %q", strings.Join(argv, " "), out)
		}
	}
	// The way through is unchanged, and the scan stops at `--`: past it a
	// word spelled like an option is a path, and `--signoff` is a long
	// option that merely contains the letter.
	for _, argv := range [][]string{
		{"commit", "-m", "x", "--", "a.go"},
		{"commit", "-m", "x", "--", "-i"},
		// An option's VALUE spelled like an option is not an option
		// (ranger-base-v3cu); TestQACommitWallL1OptionValueIsNotAnOption
		// carries the rest of that class and its premise.
		{"commit", "-m", "-i am a message", "--", "a.go"},
		{"commit", "--signoff", "-m", "x", "--", "a.go"},
		{"commit", "--fixup=HEAD", "--", "a.go"},
		{"commit", "--amend", "--no-edit", "--", "a.go"},
	} {
		if out, code := run(argv...); code != 0 || !strings.HasPrefix(out, "real git ") {
			t.Errorf("git %s must pass: %d %s", strings.Join(argv, " "), code, out)
		}
	}
}

// ranger-base-l1at — found verifying rangerhq-ojnw's close (ranger-base-wst3).
// The spoiler table lists `--include` as a literal and renderSpoiled emits a
// `--*) ;;` arm ahead of the cluster pattern, so every UNAMBIGUOUS PREFIX git
// accepts for that option — `--inc`, `--incl`, `--inclu`, `--includ` — falls
// through the wall and sweeps the shared index. Measured live: refused as
// `--include`, through as `--inc`, and the commit took another persona's
// staged file.
//
// Half one is the premise and it is NOT skipped: if git ever stops accepting
// the abbreviation, or stops sweeping with it, the guard arm below is
// pinning a ghost and this half says so first (the TestQA2f5rIncidentFourForms
// shape). Half two is the wall, unskipped by l1at's fix.
func TestQACommitWallL1IncludeAbbreviations(t *testing.T) {
	// The shortest prefix git itself resolves. `--in` and `--i` are
	// ambiguous for `git commit` and git rejects them (measured, 2.50.1),
	// so they are not spoilers and are not listed here.
	abbrevs := []string{"--includ", "--inclu", "--incl", "--inc"}

	// Half one: real git accepts each abbreviation AND it sweeps.
	for _, opt := range abbrevs {
		_, git, write := qaCommitRepo(t)
		write("a.txt", "MINE-EDITED")                         // tracked, edited, not staged
		write("c.txt", "THEIRS")                              // another persona's...
		if out, err := git(nil, "add", "c.txt"); err != nil { // ...staged work
			t.Fatalf("add: %v %s", err, out)
		}
		if out, err := git(nil, "commit", "-m", "x", opt, "--", "a.txt"); err != nil {
			t.Fatalf("premise: git must accept %s as --include: %v %s", opt, err, out)
		}
		out, _ := git(nil, "show", "--name-only", "--format=", "HEAD")
		if !strings.Contains(out, "c.txt") {
			t.Errorf("premise: `git commit %s -- a.txt` must sweep the shared index; it took %q", opt, out)
		}
	}

	// `abbrevs` is a measurement, so it is checked against the parser before
	// the wall is judged against it: a git whose ambiguity boundary moved
	// must not quietly shrink this pin. Asked before the shim is rendered —
	// rendering puts a stub git on PATH, and a stub answers nothing about
	// what the real parser resolves.
	resolved := map[string]bool{}
	for _, p := range qaGitResolves(t, "--include") {
		resolved[p] = true
	}
	if len(resolved) != len(abbrevs)+1 { // the abbreviations, plus the option itself
		t.Errorf("git resolves %d prefixes of --include, this pin lists %d: %v", len(resolved), len(abbrevs)+1, resolved)
	}
	for _, opt := range abbrevs {
		if !resolved[opt] {
			t.Errorf("premise: git no longer resolves %s to --include", opt)
		}
	}

	// Half two: the wall must refuse every spelling git accepts.
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	run := qaRenderCommitShim(t)
	for _, opt := range append(append([]string{}, abbrevs...), "--include") {
		argv := []string{"commit", "-m", "x", opt, "--", "a.go"}
		if out, code := run(argv...); code != 1 || !strings.Contains(out, "refused by posse gate") {
			t.Errorf("git %s must be refused at L1: %d %s", strings.Join(argv, " "), code, out)
		}
	}
	// The way through must not narrow: these are not prefixes of --include.
	// The last two are the over-match this fix could have caused — an
	// abbreviation of a long option that is NOT a spoiler is still a way
	// through, as its full spelling is (`--am` is `--amend`, `--sign` is
	// `--signoff`; measured, git 2.50.1).
	for _, argv := range [][]string{
		{"commit", "-m", "x", "--", "a.go"},
		{"commit", "--signoff", "-m", "x", "--", "a.go"},
		{"commit", "--fixup=HEAD", "--", "a.go"},
		{"commit", "-m", "x", "--", "--inc"},
		{"commit", "--am", "--no-edit", "--", "a.go"},
		{"commit", "--sign", "-m", "x", "--", "a.go"},
	} {
		if out, code := run(argv...); code != 0 || !strings.HasPrefix(out, "real git ") {
			t.Errorf("git %s must pass: %d %s", strings.Join(argv, " "), code, out)
		}
	}
}

// rangerhq-t9by — close of rangerhq-2f5r. The wall lmq9 landed covers THIS
// incident's shared-index half. The four forms devops measured against the
// live hook, driven with the incident's own argv (`git commit -F <file>`,
// not `-m`): B holds staged work throughout.
//
// Half one is the unguarded incident: without the hook, `git add mine &&
// git commit -F msg` captures B's staged file. If git stops sweeping, this
// pin dies and the wall is guarding a ghost. Half two is the wall.
func TestQA2f5rIncidentFourForms(t *testing.T) {
	t.Parallel()
	msg := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msg, []byte("incident\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Half one: the incident, unguarded.
	_, git, write := qaCommitRepo(t)
	write("a.txt", "B-STAGED")
	git(nil, "add", "a.txt")
	write("b.txt", "A-MINE")
	git(nil, "add", "b.txt")
	if out, err := git(nil, "commit", "-F", msg); err != nil {
		t.Fatalf("unguarded incident must land: %v %s", err, out)
	}
	if out, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") {
		t.Fatalf("premise: `git add mine && git commit -F msg` must capture B's staged a.txt, got %q", out)
	}

	// Half two: the same board with the guard. B stays staged through every form.
	repo, git2, write2 := qaCommitRepo(t)
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	write2("a.txt", "B-STAGED")
	git2(nil, "add", "a.txt")
	write2("b.txt", "A-MINE")
	git2(nil, "add", "b.txt")

	stillB := func(after string) {
		t.Helper()
		if out, _ := git2(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" && !strings.Contains(out, "a.txt") {
			t.Errorf("after %s, B's staged entry missing: %q", after, out)
		}
		if out, _ := git2(nil, "show", ":a.txt"); strings.TrimSpace(out) != "B-STAGED" {
			t.Errorf("after %s, B's staged content changed: %q", after, out)
		}
	}

	// Form 1: the incident itself — git add mine then commit -F, no pathspec.
	out, err := git2(persona, "commit", "-F", msg)
	if err == nil || !strings.Contains(out, "refused by posse gate: an unqualified git commit") {
		t.Errorf("form1 (incident argv) must be refused: %v %s", err, out)
	}
	stillB("form1")
	if out, _ := git2(nil, "diff", "--cached", "--name-only"); !strings.Contains(out, "b.txt") {
		t.Errorf("form1 must leave A's staged mine.txt too, got %q", out)
	}

	git2(nil, "reset", "-q", "HEAD", "--", "b.txt")
	write2("b.txt", "A-MINE")

	// Form 2: git commit -a, named as -a.
	if out, err := git2(persona, "commit", "-am", "sweep-all"); err == nil ||
		!strings.Contains(out, "refused by posse gate: git commit -a") {
		t.Errorf("form2 (`git commit -a`) must be refused as -a: %v %s", err, out)
	}
	stillB("form2")

	// Form 3: the blessed form. Commits only mine; B stays in the shared index.
	if out, err := git2(persona, "commit", "-F", msg, "--", "b.txt"); err != nil {
		t.Fatalf("form3 (blessed `git commit -F msg -- b.txt`) must pass: %v %s", err, out)
	}
	if out, _ := git2(nil, "show", "--name-only", "--format=", "HEAD"); strings.TrimSpace(out) != "b.txt" {
		t.Errorf("form3 must commit only b.txt, got %q", out)
	}
	if out, _ := git2(nil, "show", "HEAD:b.txt"); strings.TrimSpace(out) != "A-MINE" {
		t.Errorf("form3 HEAD:b.txt: %q", out)
	}
	if out, _ := git2(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" {
		t.Errorf("form3 must leave B staged, got %q", out)
	}
	stillB("form3")

	// Form 4: the private GIT_INDEX_FILE workaround recorded on 2f5r. Dead.
	idx := filepath.Join(t.TempDir(), "index")
	priv := []string{"RHQ_PERSONA=qa", "GIT_INDEX_FILE=" + idx}
	write2("b.txt", "A-VIA-PRIVATE")
	if out, err := git2(priv, "read-tree", "HEAD"); err != nil {
		t.Fatalf("read-tree private index: %v %s", err, out)
	}
	if out, err := git2(priv, "add", "--", "b.txt"); err != nil {
		t.Fatalf("add private index: %v %s", err, out)
	}
	head, _ := git2(nil, "rev-parse", "HEAD")
	out, err = git2(priv, "commit", "-m", "workaround")
	if err == nil || !strings.Contains(out, "refused by posse gate: a commit from a private GIT_INDEX_FILE") {
		t.Errorf("form4 (private GIT_INDEX_FILE) must be refused as private index: %v %s", err, out)
	}
	if now, _ := git2(nil, "rev-parse", "HEAD"); strings.TrimSpace(now) != strings.TrimSpace(head) {
		t.Errorf("form4 moved HEAD")
	}
	stillB("form4")
}

// rangerhq-2f5r residual, filed rangerhq-lvu9: the blessed form commits the
// WORKING TREE content of the named path, not what you staged. The wall
// closed the shared-index half of the incident; this half rides through
// because the form is correct. Isolation (rangerhq-09o2) is the real fix.
func TestQA2f5rBlessedFormTakesWorkingTree(t *testing.T) {
	t.Parallel()
	repo, git, write := qaCommitRepo(t)
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	persona := []string{"RHQ_PERSONA=qa", "RHQ_GATES_DIR=" + t.TempDir()}
	write("a.txt", "v1\ndeveloper line\nQA HALF-WRITTEN\n")
	msg := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msg, []byte("msg about developer line only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(persona, "commit", "-F", msg, "--", "a.txt"); err != nil {
		t.Fatalf("blessed form must pass: %v %s", err, out)
	}
	body, _ := git(nil, "show", "HEAD:a.txt")
	if !strings.Contains(body, "developer line") || !strings.Contains(body, "QA HALF-WRITTEN") {
		t.Fatalf("residual: named path commits the file on disk; got %q", body)
	}
}

// qaCommitOptsSince names the `git commit` options qualifierSpoilers
// carries that a git OLDER than the one beside them does not have at all,
// with the version that does. Two gits are in play at once — this box runs
// one and ci.yml's two runners run the other — and the table is the UNION
// of them, so a pin that measures against the local git has options it
// cannot reach. An entry here exempts that option from the two arms whose
// subject is "the table names a spelling this git never showed us", and
// from nothing else: wherever the local git DOES have the option, every arm
// runs against it unchanged, and the exemption arm still makes git say the
// option is unknown before it takes the exemption. A typo in the table is
// what those arms exist for, so an entry is a MEASUREMENT with a version on
// it, not a way to quiet one (ranger-base-tiidc, measured 2026-09-05
// against a git 2.55.0 bottle on this box and against Apple Git-155).
var qaCommitOptsSince = map[string]string{
	"-U":                   "2.55.0",
	"--unified":            "2.55.0",
	"--inter-hunk-context": "2.55.0",
}

// qaOptIsAbsentHere reports whether opt is one of those, AND makes this git
// say so: `git commit <opt>` must answer "unknown", so an option the help
// text merely omits — `git commit -h` hides some it still parses, measured
// `--ahead-behind` on 2.50.1 — is not waved through as version drift.
func qaOptIsAbsentHere(t *testing.T, git func(env []string, args ...string) (string, error), opt string) bool {
	t.Helper()
	absent, parsed, ver := qaOptAbsence(git, qaCommitOptsSince, opt)
	if parsed != "" {
		t.Errorf("%s is exempted as a git %s option, but the git on this box does not call it unknown (%q) — it parses it, so the exemption hides a real measurement", opt, ver, parsed)
	}
	return absent
}

// qaOptAbsence is that decision without a *testing.T, so the arm can be
// driven against a fixture and its answer READ rather than only failed on
// (TestQAVersionDriftExemptionRefusesAnOptionThisGitParses). It returns
// whether opt is exempt here, git's own answer when git parses it anyway
// (empty otherwise), and the version the table names.
func qaOptAbsence(git func(env []string, args ...string) (string, error), since map[string]string, opt string) (absent bool, parsed, ver string) {
	ver, ok := since[opt]
	if !ok {
		return false, "", ""
	}
	// --dry-run so nothing writes, opt LAST and no `--` after it: a value
	// option given a `--` swallows it and reaches "nothing to commit",
	// which reads exactly like an option git never parsed.
	out, _ := git([]string{"GIT_EDITOR=true"}, "commit", "--dry-run", "-m", "x", opt)
	if strings.Contains(out, "unknown option") || strings.Contains(out, "unknown switch") {
		return true, "", ver
	}
	return false, strings.TrimSpace(out), ver
}

// The exemption is a record of a version this box cannot reach, not a mute
// button, and the arm that keeps it one is "make git call the option
// unknown". That arm has to be shown able to FIRE, or an entry in
// qaCommitOptsSince would silence a live measurement — and its two callers
// only reach it for an option `git commit -h` did not list, so no arm above
// ever exercises it. `--message` is the witness: every git in play parses
// it, which is exactly the case the exemption must refuse.
func TestQAVersionDriftExemptionRefusesAnOptionThisGitParses(t *testing.T) {
	t.Parallel()
	_, git, _ := qaCommitRepo(t)
	const opt = "--message"
	absent, parsed, ver := qaOptAbsence(git, map[string]string{opt: "9.9.9"}, opt)
	if absent {
		t.Errorf("the exemption accepted %s, which this git parses — a qaCommitOptsSince entry could then mute a real measurement instead of recording a version this box cannot reach", opt)
	}
	// The premise, so a fixture that stopped being an option git parses
	// dies loudly instead of passing this test by accident.
	if !strings.Contains(parsed, "requires a value") {
		t.Errorf("`git commit --dry-run -m x %s` answered %q on this git, not \"requires a value\" — the fixture no longer names an option git parses, so the arm above was not exercised", opt, parsed)
	}
	if ver != "9.9.9" {
		t.Errorf("the fixture version came back %q — qaOptAbsence is not reading the table it was handed", ver)
	}
}

// The table's LongMin is a MEASUREMENT, and a measurement rots. Every long
// spoiler must carry one, it must be a real prefix of its option, and the
// arms the shim renders from it must COVER every abbreviation the git on
// this box resolves. A LongMin that is too long is a hole: git takes
// `--inc`, the shim's arms start at `--incl`, and the option walks past the
// wall (ranger-base-l1at).
//
// It used to demand git's exact boundary — LongMin resolves and one
// character shorter does not — and that is not a property one table can
// have. git's boundary moves with git's option table: measured 2026-09-04
// (ranger-base-90y3c), `git commit --int` IS `--interactive` on git 2.50.1
// and is ambiguous on git 2.55.0, where the shortest is `--intera`. Both
// gits are in play at once here — the box runs one, ci.yml's two runners
// run the other — so equality was a red on one platform or the other on
// every push, and it was the PIN that was wrong: `--int` covers git 2.55's
// `--intera` fine, being shorter.
//
// The other direction is not a hole and is not free-floating either. Arms
// are LITERAL case patterns, one per length (longArms), not prefix
// matches, so an over-short LongMin adds arms for spellings git itself
// refuses as ambiguous — it costs case lines and refuses nothing git would
// have run. So it is reported, with both numbers, rather than failed on.
//
// ValueOpts is walked with Opts (ranger-base-v3cu). The measurement is the
// same one — every abbreviation git resolves needs an arm — and only the
// consequence differs: an uncovered spoiler abbreviation walks past the
// wall, while an uncovered VALUE abbreviation takes its value back into the
// scan and refuses a safe commit. Both are the table having gone stale.
func TestQASpoilerLongMinIsGitsBoundary(t *testing.T) {
	t.Parallel()
	_, git, _ := qaCommitRepo(t)
	here := qaCommitOptions(t, git)
	for key, sp := range qualifierSpoilers {
		if !strings.HasPrefix(key, "git ") {
			continue // another command's parser, another set of rules
		}
		for _, o := range append(append([]string{}, sp.Opts...), sp.ValueOpts...) {
			if !strings.HasPrefix(o, "--") {
				continue
			}
			// An option THIS git does not have has no abbreviations to
			// cover, and qaGitResolves cannot be asked instead: it reports
			// that a prefix resolved, never WHAT it resolved to. On git
			// 2.50.1 `--u` and `--un` resolve cleanly — to
			// `--untracked-files`, which is not this option and is in
			// neither list — so an emptiness test here would read them as
			// abbreviations of `--unified` and demand arms for them, which
			// is the HOLE this pin exists to prevent, rendered by the pin
			// itself (measured, ranger-base-tiidc).
			if !contains(here, o) && qaOptIsAbsentHere(t, git, o) {
				t.Logf("%q long option %s: this git does not have it (git %s does) — its boundary is unmeasurable here, not stale",
					key, o, qaCommitOptsSince[o])
				continue
			}
			min, ok := sp.LongMin[o]
			if !ok {
				t.Errorf("%q long option %s has no LongMin: its abbreviations reach the shim unmatched (ranger-base-l1at, ranger-base-v3cu)", key, o)
				continue
			}
			if min == "" || !strings.HasPrefix(o, min) {
				t.Errorf("%q long option %s: LongMin %q is not a prefix of it, so longArms renders the literal alone and every abbreviation is unmatched", key, o, min)
				continue
			}
			got := qaGitResolves(t, o)
			if len(got) == 0 {
				t.Errorf("%q long option %s: this git resolves NO prefix of it, not even the full spelling — the probe is not measuring git's parser", key, o)
				continue
			}
			// The wall property, and the only one that is a property: every
			// abbreviation THIS git resolves has an arm.
			arms := map[string]bool{}
			for _, a := range longArms(o, min) {
				arms[a] = true
			}
			for _, g := range got {
				if !arms[g] {
					t.Errorf("%q long option %s: git resolves %s and the rendered arms %v do not cover it — LongMin %q is stale", key, o, g, longArms(o, min), min)
				}
			}
			if len(got[0]) > len(min) {
				t.Logf("%q long option %s: LongMin is %s, this git's shortest is %s — covered, with %d arm(s) of slack (git version drift, not a hole)",
					key, o, min, got[0], len(got[0])-len(min))
			}
		}
	}
}

// qaGitResolves asks the real git which prefixes of a `git commit` long
// option it resolves to that option, shortest first, the full spelling
// last. A prefix git will not resolve answers "ambiguous option" or
// "unknown option" and exits 129 before it can commit anything; a resolved
// one reaches "nothing to commit" in a clean repo, so nothing here writes.
func qaGitResolves(t *testing.T, long string) []string {
	t.Helper()
	_, git, _ := qaCommitRepo(t)
	var out []string
	for n := 3; n <= len(long); n++ {
		p := long[:n]
		o, _ := git(nil, "commit", "-m", "x", p, "--", "a.txt")
		if strings.Contains(o, "ambiguous option") || strings.Contains(o, "unknown option") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// qaRenderCommitShim renders the L1 git shim for `Bash(git commit unless --)`
// over a stub git that echoes its argv, and returns a runner for it.
func qaRenderCommitShim(t *testing.T) func(argv ...string) (string, int) {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(realBin, "git"), []byte("#!/bin/sh\necho \"real git $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, binDir, _, err := a.RenderGates("qa", []string{"Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	return func(argv ...string) (string, int) {
		out, err := exec.Command(filepath.Join(binDir, "git"), argv...).CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return string(out), code
	}
}

// The bead's own repro, end to end, with the rendered shim in front of the
// REAL git and no L3 hook in the repo — the case L1 exists for. The stub
// runner above pins the decision; this pins what the decision is worth: the
// other persona's staged entry is still staged afterwards, and HEAD has not
// moved. Half one is the premise, unguarded, so the pin dies loudly if git
// ever stops sweeping instead of quietly guarding a ghost.
func TestQACommitWallL1AbbreviationDoesNotSweepRealIndex(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	// Half one: unguarded, `--inc` really does take the other persona's file.
	_, git, write := qaCommitRepo(t)
	write("a.txt", "theirs")
	git(nil, "add", "a.txt")
	write("b.txt", "mine")
	if out, err := git(nil, "commit", "-m", "x", "--inc", "--", "b.txt"); err != nil {
		t.Fatalf("premise: unguarded `--inc` commit must land: %v %s", err, out)
	}
	if out, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(out, "a.txt") {
		t.Fatalf("premise: `git commit -m x --inc -- b.txt` must sweep a.txt, got %q", out)
	}

	// Half two: the same argv through the shim, over the real git.
	repo, git2, write2 := qaCommitRepo(t)
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	_, binDir, _, err := a.RenderGates("qa", []string{"Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	write2("a.txt", "theirs")
	git2(nil, "add", "a.txt")
	write2("b.txt", "mine")
	head, _ := git2(nil, "rev-parse", "HEAD")
	shim := func(args ...string) (string, error) {
		cmd := exec.Command(filepath.Join(binDir, "git"), append([]string{"-C", repo}, args...)...)
		cmd.Env = []string{"PATH=" + PathOutsideGates(binDir), "HOME=" + repo,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := shim("commit", "-m", "x", "--inc", "--", "b.txt")
	if err == nil || !strings.Contains(out, "refused by posse gate") {
		t.Fatalf("`git commit -m x --inc -- b.txt` must be refused at L1: %v %s", err, out)
	}
	if now, _ := git2(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(now) != "a.txt" {
		t.Errorf("the other persona's staged entry must survive the refusal, got %q", now)
	}
	if now, _ := git2(nil, "rev-parse", "HEAD"); now != head {
		t.Errorf("refused commit moved HEAD: %q -> %q", head, now)
	}
	// And the way through, over the real git, still lands exactly one path.
	if out, err := shim("commit", "-m", "safe", "--", "b.txt"); err != nil {
		t.Fatalf("path-limited commit must still pass: %v %s", err, out)
	}
	if now, _ := git2(nil, "show", "--name-only", "--format=", "HEAD"); strings.TrimSpace(now) != "b.txt" {
		t.Errorf("safe form must commit only b.txt, got %q", now)
	}
}

// The spoiler table is an ALLOW-BY-OMISSION list over an option set GIT
// owns and grows. TestQASpoilerLongMinIsGitsBoundary pins the boundary of
// every entry that exists; nothing pinned that the entry SET was complete,
// which is exactly how `-p`/`--patch` and `--interactive` sat outside it
// (ranger-base-myai). This is the other half of that pin: ask the real git
// for every option `git commit` has, put each one through the incident's
// own shape — another persona's file staged, my own path named after `--` —
// and require the set that sweeps to be EXACTLY the set the table declares.
// A git that grows a sweeping option fails here instead of in someone's
// history, and a table entry that no longer sweeps fails here as noise.
//
// Residual, named rather than measured: each option is tried BARE, which is
// how a boolean is typed. A value-taking option eats the `--` as its value
// instead (measured: none of them sweeps, they commit the named path), so
// what this pins for those is that spelling and not `--opt=<value>`.
func TestQASpoilerTableCoversEveryCommitOption(t *testing.T) {
	t.Parallel()
	_, git, write := qaCommitRepo(t)
	// A second commit, so `--amend` has a parent and its `show --name-only`
	// is a diff rather than a root listing.
	write("base.txt", "base")
	git(nil, "add", "--", "base.txt")
	if out, err := git(nil, "commit", "-qm", "second", "--", "base.txt"); err != nil {
		t.Fatalf("second commit: %v %s", err, out)
	}
	base, _ := git(nil, "rev-parse", "HEAD")
	base = strings.TrimSpace(base)

	declared := map[string]bool{}
	for _, o := range qualifierSpoilers["git commit"].Opts {
		declared[o] = true
	}
	opts := qaCommitOptions(t, git)
	// The parse is the test's own premise: a `git commit -h` this cannot
	// read would yield an empty list and pass vacuously.
	if len(opts) < 40 {
		t.Fatalf("parsed only %d options out of `git commit -h`: %v", len(opts), opts)
	}
	for _, want := range []string{"-i", "--include", "-p", "--patch", "--interactive", "-a", "--all", "-o", "--only", "--no-include"} {
		if !contains(opts, want) {
			t.Fatalf("parse missed %s, so it cannot be trusted to catch a new one: %v", want, opts)
		}
	}

	for _, opt := range opts {
		// mine.txt is my own unstaged edit; other.txt is another persona's
		// staged work, which no form of this commit may take.
		if out, err := git(nil, "reset", "-q", "--hard", base); err != nil {
			t.Fatalf("reset: %v %s", err, out)
		}
		git(nil, "clean", "-qfdx")
		write("b.txt", "MINE-EDITED")
		write("other.txt", "THEIRS")
		if out, err := git(nil, "add", "--", "other.txt"); err != nil {
			t.Fatalf("stage other.txt: %v %s", err, out)
		}
		// GIT_EDITOR so `--edit` cannot hang the suite waiting on a human;
		// no stdin, so the interactive selectors see EOF the way a fleet
		// Bash call does — which is what makes them commit at all.
		git([]string{"GIT_EDITOR=true"}, "commit", "-m", "x", opt, "--", "b.txt")
		head, _ := git(nil, "show", "--name-only", "--format=", "HEAD")
		swept := strings.Contains(head, "other.txt")
		if swept && !declared[opt] {
			t.Errorf("`git commit -m x %s -- b.txt` takes the other persona's staged work (HEAD %q) and %s is not in qualifierSpoilers[\"git commit\"]", opt, strings.Fields(head), opt)
		}
		if !swept && declared[opt] {
			t.Errorf("%s is declared a spoiler but no longer sweeps (HEAD %q): the table is carrying noise", opt, strings.Fields(head))
		}
	}
}

// qaCommitOptions is every option `git commit` names in its own `-h`, the
// full spellings only — the abbreviations on the way to them are
// LongMin's job. `--[no-]x` yields both `--x` and `--no-x`, because both
// are spellings a persona can type and the negation of a safe option is
// not obviously safe.
//
// It reads the option lines (exactly four spaces, then a dash) and takes
// tokens from the left while they still look like an option or an option's
// own argument; the first plain word is the description and ends the line.
func qaCommitOptions(t *testing.T, git func(env []string, args ...string) (string, error)) []string {
	t.Helper()
	help, _ := git(nil, "commit", "-h") // exits 129; the usage text is the point
	var out []string
	seen := map[string]bool{}
	add := func(o string) {
		if o != "-" && o != "--" && !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	cut := func(s string) string {
		if i := strings.IndexAny(s, "[="); i >= 0 {
			return s[:i]
		}
		return s
	}
	for _, line := range strings.Split(help, "\n") {
		if !strings.HasPrefix(line, "    -") {
			continue
		}
		for _, tok := range strings.Fields(line) {
			tok = strings.TrimSuffix(tok, ",")
			if !strings.HasPrefix(tok, "-") {
				// `<file>` and `[(amend|reword):]commit` are the option's
				// own argument; anything else begins the description.
				if strings.HasPrefix(tok, "<") || strings.HasPrefix(tok, "[") {
					continue
				}
				break
			}
			if rest, ok := strings.CutPrefix(tok, "--[no-]"); ok {
				add("--" + cut(rest))
				add("--no-" + cut(rest))
				continue
			}
			add(cut(tok))
		}
	}
	return out
}

// The bead's own repro end to end, with the rendered shim in front of the
// REAL git and no L3 hook — the case L1 exists for. `--patch` is worse than
// `--include` in the way that matters: `--include` at least also commits
// the paths you named, while `--patch` with no TTY commits ONLY the other
// persona's staged entry, exit 0, and leaves your own edit unstaged. Half
// one is the premise, unguarded, so the pin dies loudly if git ever stops
// doing that rather than quietly guarding a ghost (ranger-base-myai).
func TestQACommitWallL1PatchDoesNotSweepRealIndex(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	// Half one: unguarded, `--patch` commits the other persona's file and
	// NOT the path on the command line.
	_, git, write := qaCommitRepo(t)
	write("a.txt", "theirs")
	git(nil, "add", "a.txt")
	write("b.txt", "mine")
	if out, err := git(nil, "commit", "-m", "x", "--patch", "--", "b.txt"); err != nil {
		t.Fatalf("premise: unguarded `--patch` commit must land: %v %s", err, out)
	}
	took, _ := git(nil, "show", "--name-only", "--format=", "HEAD")
	if strings.TrimSpace(took) != "a.txt" {
		t.Fatalf("premise: `git commit -m x --patch -- b.txt` must commit ONLY a.txt, got %q", took)
	}

	// Half two: the same argv through the shim, over the real git.
	repo, git2, write2 := qaCommitRepo(t)
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	_, binDir, _, err := a.RenderGates("qa", []string{"Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	write2("a.txt", "theirs")
	git2(nil, "add", "a.txt")
	write2("b.txt", "mine")
	head, _ := git2(nil, "rev-parse", "HEAD")
	shim := func(args ...string) (string, error) {
		cmd := exec.Command(filepath.Join(binDir, "git"), append([]string{"-C", repo}, args...)...)
		cmd.Env = []string{"PATH=" + PathOutsideGates(binDir), "HOME=" + repo,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	for _, argv := range [][]string{
		{"commit", "-m", "x", "--patch", "--", "b.txt"},
		{"commit", "-m", "x", "--patc", "--", "b.txt"},
		{"commit", "-m", "x", "-p", "--", "b.txt"},
		{"commit", "-pm", "x", "--", "b.txt"},
		{"commit", "-m", "x", "--interactive", "--", "b.txt"},
		{"commit", "-m", "x", "--int", "--", "b.txt"},
	} {
		out, err := shim(argv...)
		if err == nil || !strings.Contains(out, "refused by posse gate") {
			t.Errorf("`git %s` must be refused at L1: %v %s", strings.Join(argv, " "), err, out)
		}
		if now, _ := git2(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(now) != "a.txt" {
			t.Errorf("%v: the other persona's staged entry must survive the refusal, got %q", argv, now)
		}
		if now, _ := git2(nil, "rev-parse", "HEAD"); now != head {
			t.Errorf("%v: refused commit moved HEAD: %q -> %q", argv, head, now)
		}
	}
	// And the way through, over the real git, still lands exactly one path.
	if out, err := shim("commit", "-m", "safe", "--", "b.txt"); err != nil {
		t.Fatalf("path-limited commit must still pass: %v %s", err, out)
	}
	if now, _ := git2(nil, "show", "--name-only", "--format=", "HEAD"); strings.TrimSpace(now) != "b.txt" {
		t.Errorf("safe form must commit only b.txt, got %q", now)
	}
}

// The wall's long-option arms are GENERATED from one measurement per option
// (spoiler.LongMin) by longArms, which walks min..full. Two pins stand
// either side of that generation and neither watches the walk itself:
// TestQASpoilerLongMinIsGitsBoundary checks the measurement against git,
// and the shim pins spell endpoints — `--inc`/`--include`, `--patc`/
// `--patch`, `--int`/`--interactive`. Only --include's ladder is walked in
// full (TestQACommitWallL1IncludeAbbreviations), so the walk is pinned for
// one option and inferred for the rest.
//
// MEASURED (ranger-base-4lwsh, mutating longArms in internal/posse/gates.go):
// returning {min, long} for EVERY option reds TestQACommitWallL1Include-
// Abbreviations; returning {min, long} for `--interactive` ALONE is green
// across the whole package, and `--intera` then walks the wall and commits
// the other persona's staged work. That is the hole this closes — a wall
// narrowed one option at a time is invisible to a pin that spells its
// endpoints.
//
// It asks the real parser for the prefixes rather than listing them, so a
// git whose ambiguity boundary moves cannot shrink the pin quietly, and it
// reads the option set out of qualifierSpoilers rather than a copy — a
// spoiler added tomorrow is walked the same day.
func TestQACommitWallL1RefusesEveryPrefixGitResolves(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	// Scoped to `git commit`: it is the rule the shim below renders. Another
	// git key in the table would need its own rule and its own repro shape,
	// so this fails rather than passing over a key it never measured.
	const key = "git commit"
	sp, ok := qualifierSpoilers[key]
	if !ok {
		t.Fatalf("qualifierSpoilers has no %q — this pin measures nothing", key)
	}

	// Asked BEFORE the shim is rendered: rendering puts a stub git on PATH,
	// and a stub answers nothing about what the real parser resolves.
	prefixes := map[string][]string{}
	total := 0
	for _, o := range sp.Opts {
		if !strings.HasPrefix(o, "--") {
			continue // short options have no abbreviations to walk
		}
		got := qaGitResolves(t, o)
		// The option itself plus at least one abbreviation. A git that
		// resolved nothing would leave every assertion below vacuous.
		if len(got) < 2 {
			t.Fatalf("git resolves %v for %s: fewer than the option itself plus one abbreviation, so this pin would measure nothing", got, o)
		}
		if got[len(got)-1] != o {
			t.Fatalf("git does not resolve %s to itself (%v): the premise is broken", o, got)
		}
		prefixes[o] = got
		total += len(got)
	}
	if total < 10 {
		t.Fatalf("only %d spellings collected across %v — a ladder this short is not the ladder (%v)", total, sp.Opts, prefixes)
	}

	run := qaRenderCommitShim(t)
	for _, o := range sp.Opts {
		for _, p := range prefixes[o] {
			argv := []string{"commit", "-m", "x", p, "--", "a.go"}
			out, code := run(argv...)
			if code != 1 || !strings.Contains(out, "refused by posse gate") {
				t.Errorf("`git %s` — git resolves %s to %s, and %s takes the shared index while carrying a pathspec — must be refused at L1, got %d %s",
					strings.Join(argv, " "), p, o, o, code, out)
			}
		}
	}

	// The control: a pin that refused everything would pass every line
	// above. The safe form and a non-spoiler abbreviation still go through.
	for _, argv := range [][]string{
		{"commit", "-m", "x", "--", "a.go"},
		{"commit", "-m", "x", "--am", "--", "a.go"},
		{"commit", "-m", "x", "--sign", "--", "a.go"},
	} {
		if out, code := run(argv...); code != 0 {
			t.Errorf("`git %s` is not a spoiler and must still pass: %d %s", strings.Join(argv, " "), code, out)
		}
	}
}

// ranger-base-v3cu — the spoiler scan read an option's VALUE as an option.
// `git commit -m '-i am a message' -- a.txt` is path-limited, carries no
// -i, and does not sweep, and the `-*i*` cluster arm matched the MESSAGE
// before the scan reached `--`. It failed CLOSED, so this is a refusal of a
// safe form rather than a way through — and a wall that refuses the one
// form it permits is a wall nobody can work behind.
//
// Half one is the premise, unguarded, against the real git: the argv the
// shim now lets through must really be safe. If git ever starts reading
// that word as an option, this half reds first and the allow row below is
// not quietly widening the wall.
//
// Half two is the wall: the same argv passes the rendered shim, and the
// forms that DO sweep are still refused — the control, without which
// "everything passes" would satisfy every line above.
func TestQACommitWallL1OptionValueIsNotAnOption(t *testing.T) {
	// No t.Parallel: qaRenderCommitShim uses t.Setenv to put the stub git
	// on PATH, and the two are mutually exclusive.

	// Half one. b.txt is my own edit; a.txt is another persona's staged work.
	_, git, write := qaCommitRepo(t)
	write("a.txt", "THEIRS")
	if out, err := git(nil, "add", "--", "a.txt"); err != nil {
		t.Fatalf("stage a.txt: %v %s", err, out)
	}
	write("b.txt", "MINE")
	if out, err := git(nil, "commit", "-m", "-i am a message", "--", "b.txt"); err != nil {
		t.Fatalf("premise: `git commit -m '-i am a message' -- b.txt` must land: %v %s", err, out)
	}
	if msg, _ := git(nil, "log", "-1", "--format=%s"); strings.TrimSpace(msg) != "-i am a message" {
		t.Fatalf("premise: git must read that word as the -m VALUE, got subject %q", strings.TrimSpace(msg))
	}
	if took, _ := git(nil, "show", "--name-only", "--format=", "HEAD"); strings.TrimSpace(took) != "b.txt" {
		t.Fatalf("premise: it must commit ONLY the named path, got %q", strings.Fields(took))
	}
	if staged, _ := git(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(staged) != "a.txt" {
		t.Fatalf("premise: the other persona's staged entry must survive, got %q", strings.Fields(staged))
	}

	// Half two.
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	run := qaRenderCommitShim(t)
	// The bead's row first, then the rest of the class: a value that is
	// spelled like an option, in every form git actually pairs — short
	// separate-word, short GLUED (git takes the rest of the token, so
	// `-mi` is the message "i"), long separate-word, and a long
	// ABBREVIATION, which is a separate-word pair just the same.
	for _, argv := range [][]string{
		{"commit", "-m", "-i am a message", "--", "a.go"},
		{"commit", "-mi", "--", "a.go"},
		{"commit", "-mfix the include", "--", "a.go"},
		{"commit", "-F", "-i-file", "--", "a.go"},
		{"commit", "-t", "-i", "--", "a.go"},
		{"commit", "-C", "-i", "--", "a.go"},
		{"commit", "--message", "-i msg", "--", "a.go"},
		{"commit", "--m", "-i msg", "--", "a.go"},
		{"commit", "--trailer", "-i", "--", "a.go"},
		{"commit", "--squash", "-i", "--", "a.go"},
		// The three git 2.55 added, in the same forms: without them in
		// ValueOpts these are the false positive the table's comment
		// names — a path-limited commit refused, one respelling away from
		// working, on the git both ci.yml runners now carry. The value is
		// really git's and not an option: measured on 2.55,
		// `git commit -m x -U -i -- b.txt` answers "switch `U' expects an
		// integer value", leaves the other persona's staged a.txt staged
		// and HEAD where it was — so letting it past the wall lets nothing
		// past git (ranger-base-tiidc).
		{"commit", "-U", "-i", "--", "a.go"},
		{"commit", "--unified", "-i", "--", "a.go"},
		{"commit", "--uni", "-i", "--", "a.go"},
		{"commit", "--inter-hunk-context", "-i", "--", "a.go"},
		{"commit", "--inter-", "-i", "--", "a.go"},
	} {
		if out, code := run(argv...); code != 0 || !strings.HasPrefix(out, "real git ") {
			t.Errorf("git %s carries no -i/-p option — only a value spelled like one — and must pass: %d %s", strings.Join(argv, " "), code, out)
		}
	}
	// The control. Pairing must not shift past a REAL option: the value
	// options are consumed with their value and the scan goes on, and the
	// two options whose argument is OPTIONAL (-S, -u) take the rest of
	// their own token and never the next word, so they must not pair at all.
	for _, argv := range [][]string{
		{"commit", "-m", "x", "-i", "--", "a.go"},
		{"commit", "-m", "x", "--inc", "--", "a.go"},
		{"commit", "--message", "x", "-p", "--", "a.go"},
		{"commit", "--trailer", "x", "-i", "--", "a.go"},
		{"commit", "-u", "-i", "--", "a.go"},
		{"commit", "-S", "-i", "--", "a.go"},
		{"commit", "--gpg-sign", "-i", "--", "a.go"},
		{"commit", "--untracked-files", "-i", "--", "a.go"},
		{"commit", "-im", "x", "--", "a.go"},
		{"commit", "-U", "3", "-i", "--", "a.go"},
		{"commit", "--unified", "3", "-i", "--", "a.go"},
		{"commit", "--uni", "3", "-i", "--", "a.go"},
		{"commit", "--inter-hunk-context", "3", "-i", "--", "a.go"},
		{"commit", "--inter-", "3", "-i", "--", "a.go"},
	} {
		if out, code := run(argv...); code != 1 || !strings.Contains(out, "refused by posse gate") {
			t.Errorf("git %s carries a real spoiler and must still be refused: %d %s", strings.Join(argv, " "), code, out)
		}
	}
}

// ValueOpts is a MEASUREMENT of git's own parser and the two directions of
// error are not the same size, so this asks git rather than trusting the
// table (ranger-base-v3cu):
//
//   - an option listed that does NOT eat the next word is a HOLE — the scan
//     shifts past whatever follows, and `git commit -u -i -- f` sweeps with
//     the wall silent.
//   - an option MISSING costs a false positive: its value is read as an
//     option and a safe commit is refused, one respelling away from working.
//
// Both are reported. The premise — that the parse found the options at all
// — is checked first, so a `git commit -h` this cannot read fails loudly
// instead of passing vacuously.
func TestQAValueOptsAreGitsRequiredValueOptions(t *testing.T) {
	t.Parallel()
	const key = "git commit"
	sp, ok := qualifierSpoilers[key]
	if !ok {
		t.Fatalf("qualifierSpoilers has no %q — this pin measures nothing", key)
	}
	_, git, _ := qaCommitRepo(t)
	opts := qaCommitOptions(t, git)
	if len(opts) < 40 {
		t.Fatalf("parsed only %d options out of `git commit -h`: %v", len(opts), opts)
	}
	for _, want := range []string{"-m", "--message", "-F", "-u", "-S", "-i"} {
		if !contains(opts, want) {
			t.Fatalf("parse missed %s, so it cannot be trusted to classify the rest: %v", want, opts)
		}
	}

	declared := map[string]bool{}
	for _, o := range sp.ValueOpts {
		declared[o] = true
	}
	// git's own answer: an option whose argument is REQUIRED says so when
	// it is given none, and that is exactly the set that consumes the next
	// word. An OPTIONAL argument (-S, -u) is silent here and must stay out.
	found := 0
	for _, opt := range opts {
		// --dry-run so a spelling that parses cannot write; parse-options
		// runs first, so the value error still wins when both apply.
		out, _ := git([]string{"GIT_EDITOR=true"}, "commit", "--dry-run", opt)
		takesValue := strings.Contains(out, "requires a value")
		if takesValue {
			found++
		}
		if takesValue && !declared[opt] {
			t.Errorf("git says %s requires a value, so its value is the NEXT WORD and the scan reads it as an option: add %s to qualifierSpoilers[%q].ValueOpts", opt, opt, key)
		}
		if !takesValue && declared[opt] {
			t.Errorf("%s is declared in ValueOpts but git does not take a separate value for it (%q): pairing it shifts the scan past a real spoiler — that is a HOLE, not a false positive", opt, strings.TrimSpace(out))
		}
	}
	if found < 10 {
		t.Fatalf("only %d of `git commit`'s options answered \"requires a value\" — the probe is not reaching git's parser", found)
	}
	// …and nothing in the table escaped the loop by not being an option
	// this git names at all: a spelling `git commit -h` does not list is
	// never probed above, so the HOLE check would pass over it in silence.
	for _, o := range sp.ValueOpts {
		if contains(opts, o) {
			continue
		}
		if qaOptIsAbsentHere(t, git, o) {
			t.Logf("ValueOpts names %s and this git does not have it — git %s does, and the table is the union of both. Unprobed here, not drift", o, qaCommitOptsSince[o])
			continue
		}
		t.Errorf("ValueOpts names %s and `git commit -h` does not, so no arm of this pin ever probed it: measure it or drop it", o)
	}
}
