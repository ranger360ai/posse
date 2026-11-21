//go:build !posse_arm2 && !posse_arm3

package posse

// Arm 1, like every other gates fixture: the precedence pin below reads
// sequencerabort_qa_test.go's fake git, which lives there.
//
// The continue wall (ranger-base-05aa1), the L1 commit wall's other half.
//
// WHAT THE BEAD MEASURED and these pins keep measured: `Bash(git commit
// unless --)` matches the verb `commit`, and four git verbs make a commit
// without ever spelling it. MEASURED 2026-11-15 (git 2.56.0, linux
// 7.2.15-1-omarchy, this box), each verb conflicted, resolved, and ANOTHER
// actor's unrelated file staged during the conflict window before
// `--continue`:
//
//	verb                        no hook        L3 hook installed
//	git merge --continue        SWEPT it       SWEPT it
//	git cherry-pick --continue  SWEPT it       SWEPT it
//	git revert --continue       SWEPT it       refused (exit 1)
//	git rebase --continue       SWEPT it       SWEPT it
//	git commit -m <msg>         refused by L1  refused by L1
//
// So the L3 prepare-commit-msg hook backstops ONE of the four — the bead's
// open question — and the other three are exempt there on their own stated
// merits (sharedIndexBody: git refuses a pathspec mid-merge and
// mid-cherry-pick, and a rebase has commits left to replay).
//
// AMENDED ranger-base-6zw05: the table above is keyed on the VERB, and the
// pins below derived the verb set from git while NOTHING derived the option
// set — `--continue` was hand-spelled, `--skip` appeared nowhere, and
// `git cherry-pick --skip` and `git rebase --skip` went through the armed
// wall unrefused. MEASURED 2026-11-21 (git 2.56.0, same box), the same
// window with a SECOND operation left in the sequence so the skip has
// something to replay:
//
//	typed                     commit made   the other actor's staged file
//	git cherry-pick --skip    YES           DESTROYED (index and disk)
//	git revert --skip         YES           DESTROYED (index and disk)
//	git rebase --skip         YES           DESTROYED (index and disk)
//	git merge --skip          — merge has no --skip
//
// `--skip` is a `reset --hard` and then the rest of the sequence, so it
// deletes that file instead of sweeping it into the commit — and the
// commit it then makes is one the PID's `git commit` rule never sees,
// which is this wall's whole subject. So the option set is asked of git
// too, and pin 2 below is the pin that was missing.
//
// Five pins, in the order a reader wants them:
//
//   - the verb table is the whole class git has (derived from git);
//   - so is each verb's OPTION set: every option git treats as meaningful
//     only mid-operation is walled or stood down with a measured reason;
//   - each walled option's abbreviation minimum is git's own boundary;
//   - the wall refuses in a SHARED checkout and the premise still holds
//     (unguarded, each option really does sweep or destroy), while the
//     stood-down options are not refused;
//   - it stands down in a LINKED WORKTREE, where the index is the seat's.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ─── the fixture ─────────────────────────────────────────────────────────────

// cwFixture is a scratch repo, a runner for the REAL git past the gates, a
// runner for the rendered L1 shim, and the refusals log the shim writes.
type cwFixture struct {
	repo, binDir, log string
	git               func(args ...string) (string, error)
	shim              func(dir string, args ...string) (string, error)
	write             func(dir, name, body string)
}

func cwSetup(t *testing.T) *cwFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	fx := &cwFixture{repo: gitTempDir(t)}
	home := gitTempDir(t)
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	gatesDir, binDir, _, err := a.RenderGates("qa-cw", []string{"Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	fx.binDir, fx.log = binDir, filepath.Join(gatesDir, "refusals.log")

	env := func(dir string) []string {
		return []string{"PATH=" + PathOutsideGates(binDir), "HOME=" + dir,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			// Every advance that is NOT refused has to finish without a
			// terminal: git reuses the message it already has. The sequence
			// editor is named too, for `rebase --edit-todo`, which opens
			// that one and not the message editor.
			"GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true"}
	}
	fx.git = func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Env = env(fx.repo)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	fx.shim = func(dir string, args ...string) (string, error) {
		cmd := exec.Command(filepath.Join(binDir, "git"), args...)
		cmd.Dir = dir
		cmd.Env = env(dir)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	fx.write = func(dir, name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := fx.git("-C", fx.repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	fx.write(fx.repo, "f.txt", "base\n")
	fx.write(fx.repo, "other.txt", "theirs\n")
	fx.git("-C", fx.repo, "add", "--", "f.txt", "other.txt")
	if out, err := fx.git("-C", fx.repo, "commit", "-qm", "base"); err != nil {
		t.Fatalf("base commit: %v %s", err, out)
	}
	return fx
}

// cwConflict leaves dir mid-<verb> with the conflict resolved and another
// actor's unrelated file staged — the shared-checkout window the wall is
// for. It is the same shape for all four verbs: one file conflicts, one
// file is somebody else's.
//
// tail asks for a SECOND operation left in the sequence after the one that
// conflicted (ranger-base-6zw05). It is what `--skip` needs to be measured
// at all: a skip with nothing left to replay only ends the sequence, while
// a skip with one more pick in it RESETS HARD and then commits that pick —
// the unwalled commit this wall is for. `merge` takes no tail: there is one
// merge and git gives it no `--skip`.
func cwConflict(t *testing.T, fx *cwFixture, dir, verb, branch string, tail bool) {
	t.Helper()
	g := func(args ...string) (string, error) {
		return fx.git(append([]string{"-C", dir}, args...)...)
	}
	if tail && verb == "merge" {
		t.Fatalf("cwConflict: merge takes no tail — git merge has no --skip")
	}
	switch verb {
	case "merge", "rebase", "cherry-pick":
		g("checkout", "-q", "-b", branch)
		fx.write(dir, "f.txt", "theirs\n")
		g("commit", "-qm", "theirs", "--", "f.txt")
		pick, _ := g("rev-parse", "HEAD")
		second := ""
		if tail {
			// A pick that cannot conflict with anything, so what a `--skip`
			// does next is a commit and not a second conflict.
			fx.write(dir, "s2.txt", "second\n")
			g("add", "--", "s2.txt")
			g("commit", "-qm", "second-pick", "--", "s2.txt")
			s, _ := g("rev-parse", "HEAD")
			second = strings.TrimSpace(s)
		}
		g("checkout", "-q", "-")
		fx.write(dir, "f.txt", "mine\n")
		g("commit", "-qm", "mine", "--", "f.txt")
		switch verb {
		case "merge":
			g("merge", branch)
		case "cherry-pick":
			args := []string{"cherry-pick", strings.TrimSpace(pick)}
			if second != "" {
				args = append(args, second)
			}
			g(args...)
		case "rebase":
			// Rebase the side branch onto the branch that moved, so the
			// replay conflicts and stops with commits left to apply.
			base, _ := g("rev-parse", "--abbrev-ref", "HEAD")
			g("checkout", "-q", branch)
			g("rebase", strings.TrimSpace(base))
		}
	case "revert":
		fx.write(dir, "f.txt", "one\n")
		g("commit", "-qm", "one", "--", "f.txt")
		target, _ := g("rev-parse", "HEAD")
		second := ""
		if tail {
			// A commit whose revert cannot conflict: it only adds a file,
			// so undoing it only removes one.
			fx.write(dir, "k.txt", "k\n")
			g("add", "--", "k.txt")
			g("commit", "-qm", "ktouch", "--", "k.txt")
			s, _ := g("rev-parse", "HEAD")
			second = strings.TrimSpace(s)
		}
		fx.write(dir, "f.txt", "two\n")
		g("commit", "-qm", "two", "--", "f.txt")
		args := []string{"revert", "--no-edit", strings.TrimSpace(target)}
		if second != "" {
			args = append(args, second)
		}
		g(args...)
	default:
		t.Fatalf("cwConflict: unknown verb %q", verb)
	}
	fx.write(dir, "f.txt", "resolved\n")
	g("add", "--", "f.txt")
	// The other persona, staging into the same index mid-conflict.
	fx.write(dir, "other.txt", "theirs-edited\n")
	g("add", "--", "other.txt")
	if out, _ := g("diff", "--cached", "--name-only", "HEAD"); !strings.Contains(out, "other.txt") {
		t.Fatalf("%s fixture: other.txt must be staged before the advance, got %q", verb, out)
	}
	// The tail is not asserted here. What it is FOR is asserted where it
	// matters, in the premise half below: a `--skip` over a fixture that
	// had no tail makes no commit, and that assertion fails loudly.
}

// cwReachable is every commit dir can reach, from any ref or from HEAD, as
// a set — the before/after reading that answers "did this option make a
// commit?" for an option that may also move HEAD BACKWARDS. `--abort`
// does: it restores the pre-operation HEAD, so "HEAD moved" is not the
// question. A new sha appearing is.
func cwReachable(t *testing.T, fx *cwFixture, dir string) map[string]bool {
	t.Helper()
	out, err := fx.git("-C", dir, "rev-list", "--all", "HEAD")
	if err != nil {
		t.Fatalf("rev-list: %v %s", err, out)
	}
	set := map[string]bool{}
	for _, l := range strings.Fields(out) {
		set[l] = true
	}
	if len(set) == 0 {
		t.Fatalf("rev-list --all HEAD in %s is empty, so the reading is broken, not git", dir)
	}
	return set
}

// cwNewCommits is what cwReachable gained between two readings.
func cwNewCommits(before, after map[string]bool) []string {
	var got []string
	for sha := range after {
		if !before[sha] {
			got = append(got, sha)
		}
	}
	sort.Strings(got)
	return got
}

// cwHeadFiles is the file list of dir's HEAD commit.
func cwHeadFiles(t *testing.T, fx *cwFixture, dir string) string {
	t.Helper()
	out, _ := fx.git("-C", dir, "show", "--pretty=format:", "--name-only", "--no-renames", "HEAD")
	return out
}

// ─── pin 1: the table is the class, derived from git ─────────────────────────

// Membership is DERIVED, not listed: every porcelain verb git itself says
// takes `--continue` must be in continueWallVerbs, or be `am`, which the
// hook-skip arm refuses outright for a different and stronger reason (it
// commits with neither pre-commit nor commit-msg run — ADR 0052,
// hookSkipVerbExtra). A git that grows a fifth one reds here on the day it
// lands rather than the day somebody's work is swept by it.
//
// `am`'s EXCUSE IS THINNER THAN THIS CLAUSE READS, and it is now measured:
// that arm refuses the verb by DEFAULT and passes four forms through
// (hookSkipAmAllowed), one of which is `--skip` — which applies and COMMITS
// whatever is left in the mbox, exactly as `cherry-pick --skip` does.
// MEASURED 2026-11-21 through this box's installed shim. So the assertion
// below is true (the entry exists) while the sentence above it is not quite,
// and this test is deliberately left as it was: the fix belongs to that
// arm's own table, under ranger-base-t4qez, and either answer there — drop
// `--skip` from the allowed list, or give `am` a row in continueWallVerbs —
// makes this clause honest again. ranger-base-6zw05 filed it rather than
// widening into another arm's table.
//
// `<verb> -h` and not `<verb> --continue`: the usage line is read-only and
// exits before the verb does anything, so the census cannot change a repo
// or block on stdin, and it covers a verb whose `--continue` would need an
// operation in progress to say anything at all.
func TestQAContinueWallNamesEveryGitVerbThatCanContinue(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	list, err := exec.Command("git", "--list-cmds=main").Output()
	if err != nil || len(list) == 0 {
		t.Skip("git --list-cmds=main unsupported")
	}
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	var got []string
	for _, verb := range strings.Fields(string(list)) {
		cmd := exec.Command("git", verb, "-h")
		cmd.Stdin = devnull
		out, _ := cmd.CombinedOutput()
		if strings.Contains(string(out), "--continue") {
			got = append(got, verb)
		}
	}
	want := []string{"am"}
	for _, v := range continueWallVerbs {
		want = append(want, v.Verb)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("git's --continue verbs are %q, continueWallVerbs+am is %q.\n"+
			"A verb git grew: add it to continueWallVerbs with its measured Min and its way through.\n"+
			"A verb that left: drop it, and say so where the table is documented.", got, want)
	}
	if _, ok := hookSkipVerbExtra["am"]; !ok {
		t.Error(`am is excused from continueWallVerbs because hookSkipVerbExtra["am"] refuses it; that entry is gone`)
	}
}

// ─── pin 2: the OPTION set is derived from git too ───────────────────────────

// cwLongOptions is every long option `git <verb> -h` lists, as git spells
// it with the `[no-]` infix removed — `--[no-]quit` is `--quit`.
func cwLongOptions(t *testing.T, verb string) []string {
	t.Helper()
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	cmd := exec.Command("git", verb, "-h")
	cmd.Stdin = devnull
	out, _ := cmd.CombinedOutput()
	seen := map[string]bool{}
	var got []string
	for _, m := range regexp.MustCompile(`--(\[no-\])?[a-z][a-z0-9-]*`).FindAllString(string(out), -1) {
		o := "--" + strings.TrimPrefix(strings.TrimPrefix(m, "--"), "[no-]")
		if !seen[o] {
			seen[o], got = true, append(got, o)
		}
	}
	if len(got) < 5 {
		t.Fatalf("git %s -h listed %d long options, so the parse is wrong, not git:\n%s", verb, len(got), out)
	}
	return got
}

// cwInProgressOnlyMsg is git's family of "you asked me to carry on
// something that is not happening" refusals, MEASURED 2026-11-21 on git
// 2.56.0 over all four verbs: `fatal: no rebase in progress`,
// `error: no cherry-pick or revert in progress`, `error: no revert in
// progress`, `fatal: There is no merge in progress (MERGE_HEAD missing).`
// and — the one that is not "in progress" at all —
// `fatal: There is no merge to abort (MERGE_HEAD missing).`
//
// A reworded message would SHRINK the class this matcher derives, which is
// the dangerous direction for a derivation. It cannot shrink quietly: the
// pin compares the class to Advance ∪ Stood for equality, so an option that
// falls out of the class reds the pin just as loudly as one that falls in.
var cwInProgressOnlyMsg = regexp.MustCompile(`(?i)\bno [a-z-]+( or [a-z-]+)? (in progress|to abort)\b`)

// cwInProgressOnly answers "is this option meaningful ONLY while an
// operation is in progress?" by asking git, in a fresh repo where nothing
// is in progress. Two answers mean yes: one of the refusals above, or
// NOTHING AT ALL, which is what `merge --quit`, `cherry-pick --quit` and
// `revert --quit` do — they are silent no-ops outside an operation (also
// MEASURED, and the reason the matcher alone is not enough). Every other
// option of all four verbs answered with the usage line, "requires a
// value", "No remote for the current branch", "There is no tracking
// information", or by doing the verb's ordinary work.
//
// One fresh repo per probe, because a probe may MUTATE: `git rebase --root`
// in this fixture rebases and succeeds, and a repo left mid-anything would
// make every later answer in that repo a lie.
func cwInProgressOnly(t *testing.T, verb, opt string) bool {
	t.Helper()
	repo := gitTempDir(t)
	env := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		// An option that would open an editor must not block on one.
		"GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true"}
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env, cmd.Stdin = env, devnull
		out, _ := cmd.CombinedOutput()
		return string(out)
	}
	if out := run("init", "-q", "-b", "main"); strings.TrimSpace(out) != "" {
		t.Fatalf("git init: %s", out)
	}
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "--", "f.txt")
	if out := run("commit", "-qm", "base", "--", "f.txt"); strings.Contains(out, "fatal") {
		t.Fatalf("base commit: %s", out)
	}
	out := run(verb, opt)
	return cwInProgressOnlyMsg.MatchString(out) || strings.TrimSpace(out) == ""
}

// The hole ranger-base-6zw05 found was not a verb git grew — it was an
// OPTION nobody derived. So this is pin 1's shape one level down: for each
// verb, git's own in-progress-only class must be exactly Advance ∪ Stood.
// An option git adds to that class lands in neither and reds here on the
// day it lands; an option that leaves reds too, so the stated reason goes
// with it.
//
// Advance is "it carries the operation forward, so it COMMITS" and Stood is
// "it commits nothing, measured" — which is the only thing that excuses an
// option from a wall whose subject is a commit the PID's `git commit` rule
// never sees. The split is a DECISION and the pin does not re-derive it;
// what it refuses to allow is an option in the class that nobody decided
// about.
func TestQAContinueWallOptionSetIsGitsInProgressClass(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, v := range continueWallVerbs {
		t.Run(v.Verb, func(t *testing.T) {
			t.Parallel()
			var class []string
			for _, o := range cwLongOptions(t, v.Verb) {
				if cwInProgressOnly(t, v.Verb, o) {
					class = append(class, o)
				}
			}
			var named []string
			for _, o := range v.Advance {
				named = append(named, o.Long)
			}
			for _, s := range v.Stood {
				named = append(named, s.Long)
				if strings.TrimSpace(s.Why) == "" {
					t.Errorf("%s %s stands down with no reason: the reason is the whole content of the decision", v.Verb, s.Long)
				}
			}
			sort.Strings(class)
			sort.Strings(named)
			if strings.Join(class, " ") != strings.Join(named, " ") {
				t.Errorf("git %s's in-progress-only options are %q, the table names %q.\n"+
					"An option git grew: put it in Advance with its measured Min if it carries the operation\n"+
					"forward (it then COMMITS), or in Stood with the measured reason it commits nothing.\n"+
					"An option that left git: drop its row and say so where the table is documented.",
					v.Verb, class, named)
			}
			if len(v.Advance) == 0 {
				t.Errorf("%s is in the wall's table and walls no option", v.Verb)
			}
		})
	}
}

// ─── pin 3: the abbreviation minima are git's own boundary ───────────────────

// Each walled option's Min must be the SHORTEST prefix of it that git
// resolves: every prefix from Min up must parse, and the one below it must
// not. An over-short Min renders arms git refuses itself; a too-long one is
// a hole one keystroke wide. Asked of the real git one prefix at a time,
// the way a spoiler's LongMin is (ranger-base-90y3c).
//
// With no operation in progress, git's own "no <verb> in progress" IS the
// proof the option parsed — an unparsed one answers "ambiguous option",
// "unknown option" or the usage line.
func TestQAContinueWallMinimaAreGitsBoundary(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := gitTempDir(t)
	env := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = env
		out, _ := cmd.CombinedOutput()
		return string(out)
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	resolves := func(verb, p string) bool {
		o := run(verb, p)
		return !strings.Contains(o, "ambiguous option") && !strings.Contains(o, "unknown option") &&
			!strings.Contains(o, "unknown switch") && !strings.Contains(o, "usage:")
	}
	for _, v := range continueWallVerbs {
		for _, o := range v.Advance {
			if !strings.HasPrefix(o.Long, o.Min) {
				t.Errorf("%s: Min %q is not a prefix of %q", v.Verb, o.Min, o.Long)
				continue
			}
			for _, p := range longArms(o.Long, o.Min) {
				if !resolves(v.Verb, p) {
					t.Errorf("git %s %s: does not resolve, so Min %q is too short — the shim renders an arm git refuses itself:\n%s",
						v.Verb, p, o.Min, run(v.Verb, p))
				}
			}
			if short := o.Min[:len(o.Min)-1]; short != "--" && resolves(v.Verb, short) {
				t.Errorf("git %s %s: resolves, so Min %q is too long and the wall has a hole one keystroke wide",
					v.Verb, short, o.Min)
			}
		}
	}
}

// ─── pin 4: refused in a shared checkout, and the premise still holds ────────

// Three halves, the first two like
// TestQACommitWallL1AbbreviationDoesNotSweepRealIndex: the premise
// unguarded (each walled option really does take or destroy the other
// persona's staged file, so this dies loudly if git ever stops), the same
// window through the shim, and then the stood-down options, which must NOT
// be refused.
//
// The premise differs by option and is asserted per option rather than
// generically, because the two harms are not the same thing (MEASURED
// 2026-11-21): `--continue` SWEEPS the other persona's staged file into its
// commit, while `--skip` resets hard — destroying it in the index and on
// disk — and then commits what is left to replay. Both are a commit the
// PID's `git commit` rule never sees, which is what this wall refuses.
func TestQAContinueWallRefusesTheFourVerbsInASharedCheckout(t *testing.T) {
	t.Parallel()
	for _, v := range continueWallVerbs {
		t.Run(v.Verb, func(t *testing.T) {
			t.Parallel()
			for _, o := range v.Advance {
				t.Run(o.Long, func(t *testing.T) {
					t.Parallel()
					tail := o.Long == "--skip"

					// Half one: unguarded, the real git does the harm.
					premise := cwSetup(t)
					cwConflict(t, premise, premise.repo, v.Verb, "side", tail)
					before := cwReachable(t, premise, premise.repo)
					if out, err := premise.git("-C", premise.repo, v.Verb, o.Long); err != nil {
						t.Fatalf("premise: git %s %s must finish: %v %s", v.Verb, o.Long, err, out)
					}
					if made := cwNewCommits(before, cwReachable(t, premise, premise.repo)); len(made) != 1 {
						t.Fatalf("premise: git %s %s must make a commit — that is what this wall is about — new commits: %q",
							v.Verb, o.Long, made)
					}
					files := cwHeadFiles(t, premise, premise.repo)
					switch o.Long {
					case "--continue":
						if !strings.Contains(files, "other.txt") {
							t.Fatalf("premise: git %s --continue must SWEEP other.txt, committed %q", v.Verb, files)
						}
					case "--skip":
						// It deletes rather than sweeps, so the proof is
						// that their edit is gone from the index AND from
						// the disk, and is not in the commit either.
						if staged, _ := premise.git("-C", premise.repo, "diff", "--cached", "--name-only", "HEAD"); strings.Contains(staged, "other.txt") {
							t.Fatalf("premise: git %s --skip must clear the index, other.txt still staged: %q", v.Verb, staged)
						}
						b, err := os.ReadFile(filepath.Join(premise.repo, "other.txt"))
						if err != nil || strings.Contains(string(b), "theirs-edited") {
							t.Fatalf("premise: git %s --skip must DESTROY the other persona's edit on disk, read %q (%v)", v.Verb, b, err)
						}
						if strings.Contains(files, "other.txt") {
							t.Fatalf("premise: git %s --skip resets hard first, so other.txt cannot be in its commit, got %q", v.Verb, files)
						}
					default:
						t.Fatalf("no premise written for %s %s — write one before walling it", v.Verb, o.Long)
					}

					// Half two: the same window, through the shim.
					fx := cwSetup(t)
					cwConflict(t, fx, fx.repo, v.Verb, "side", tail)
					head, _ := fx.git("-C", fx.repo, "rev-parse", "HEAD")
					out, err := fx.shim(fx.repo, v.Verb, o.Long)
					if err == nil || !strings.Contains(out, "refused by posse gate") ||
						!strings.Contains(out, "rule: commitwall:sequencer-continue") {
						t.Fatalf("git %s %s must be refused in a shared checkout: %v %s", v.Verb, o.Long, err, out)
					}
					// The refusal names the option git spells, not just the verb.
					if !strings.Contains(out, "git "+v.Verb+" "+o.Long+" ADVANCES") {
						t.Errorf("the refusal must name %s %s: %s", v.Verb, o.Long, out)
					}
					if now, _ := fx.git("-C", fx.repo, "rev-parse", "HEAD"); now != head {
						t.Errorf("refused %s %s moved HEAD: %q -> %q", v.Verb, o.Long, head, now)
					}
					if staged, _ := fx.git("-C", fx.repo, "diff", "--cached", "--name-only", "HEAD"); !strings.Contains(staged, "other.txt") {
						t.Errorf("the other persona's staged entry must survive the refusal, got %q", staged)
					}
					if b, _ := os.ReadFile(fx.log); !strings.Contains(string(b), "(commitwall: sequencer-continue)") {
						t.Errorf("refusals.log has no line for %s %s:\n%s", v.Verb, o.Long, b)
					}
					// The abbreviation git resolves is refused the same way.
					if out, err := fx.shim(fx.repo, v.Verb, o.Min); err == nil || !strings.Contains(out, "rule: commitwall:sequencer-continue") {
						t.Errorf("git %s %s (git's own minimum) must be refused too: %v %s", v.Verb, o.Min, err, out)
					}
					// And so is the form reached through a leading global option.
					if out, err := fx.shim(fx.repo, "-C", fx.repo, v.Verb, o.Long); err == nil ||
						!strings.Contains(out, "rule: commitwall:sequencer-continue") {
						t.Errorf("git -C <repo> %s %s must be refused: %v %s", v.Verb, o.Long, err, out)
					}
				})
			}

			// Half three: the stood-down options are not refused, and the
			// REASON they are not is asked of git.
			//
			// Not refused: the wall reads argv and the git dir and never
			// asks whether anything is in progress, so a repo with nothing
			// in progress is the whole window for that half — one fixture
			// for all of them, and git's own "no <op> in progress" is what
			// these print there.
			t.Run("stand-down", func(t *testing.T) {
				t.Parallel()
				clean := cwSetup(t)
				for _, s := range v.Stood {
					out, _ := clean.shim(clean.repo, v.Verb, s.Long)
					if strings.Contains(out, "commitwall:sequencer-continue") {
						t.Errorf("git %s %s stands down (%s) and must not be refused: %s", v.Verb, s.Long, s.Why, out)
					}
				}
				// And the reason, in the window that matters: mid-conflict,
				// unguarded, each one makes NO commit. This is the half
				// that keeps the Advance/Stood split honest — an option
				// moved out of Advance to quiet the wall reds here, because
				// git makes a commit from it, and "it commits nothing" is
				// the only thing that excuses an option from a wall whose
				// subject is an unwalled commit.
				//
				// WITH THE TAIL, and that is not a detail: a mutant that
				// moved `rebase --skip` into Stood with a false reason
				// passed this half while it used a tail-less fixture,
				// because a skip with nothing left to replay only ends the
				// sequence. The window has to be as strong as the one the
				// Advance half measures in, or the mutant is invisible.
				for _, s := range v.Stood {
					t.Run(s.Long, func(t *testing.T) {
						t.Parallel()
						fx := cwSetup(t)
						cwConflict(t, fx, fx.repo, v.Verb, "side", v.Verb != "merge")
						before := cwReachable(t, fx, fx.repo)
						if out, err := fx.git("-C", fx.repo, v.Verb, s.Long); err != nil {
							t.Fatalf("git %s %s must run mid-conflict: %v %s", v.Verb, s.Long, err, out)
						}
						if made := cwNewCommits(before, cwReachable(t, fx, fx.repo)); len(made) > 0 {
							t.Errorf("git %s %s made %q: it belongs in Advance, not Stood (%s)", v.Verb, s.Long, made, s.Why)
						}
					})
				}
			})

			// The way through the refusal names is NOT refused, and for
			// `--abort` that is asserted in the window it is prescribed
			// for, mid-conflict, where it is also what the refusal's second
			// line tells the reader to type.
			t.Run("abort-mid-conflict", func(t *testing.T) {
				t.Parallel()
				fx := cwSetup(t)
				cwConflict(t, fx, fx.repo, v.Verb, "side", false)
				if out, err := fx.shim(fx.repo, v.Verb, "--abort"); err != nil || strings.Contains(out, "commitwall:sequencer-continue") {
					t.Errorf("git %s --abort must still run mid-conflict: %v %s", v.Verb, err, out)
				}
			})
		})
	}
}

// The verbs are untouched without `--continue`, and a path-limited commit
// is still the way through: the wall must cost nothing to the forms the
// commit wall already permits.
func TestQAContinueWallLeavesTheOrdinaryFormsAlone(t *testing.T) {
	t.Parallel()
	fx := cwSetup(t)
	// A clean merge needs no wall: git refuses to START one with anything
	// staged ("your local changes would be overwritten"), so there is no
	// window for it to sweep — the premise the wall's narrowness rests on.
	fx.git("-C", fx.repo, "checkout", "-q", "-b", "side")
	fx.write(fx.repo, "s.txt", "side\n")
	fx.git("-C", fx.repo, "add", "--", "s.txt")
	fx.git("-C", fx.repo, "commit", "-qm", "side", "--", "s.txt")
	fx.git("-C", fx.repo, "checkout", "-q", "main")
	fx.write(fx.repo, "other.txt", "theirs-edited\n")
	fx.git("-C", fx.repo, "add", "--", "other.txt")
	out, err := fx.shim(fx.repo, "merge", "--no-ff", "--no-edit", "side")
	if err == nil {
		t.Errorf("premise: a clean merge over a staged file must be refused BY GIT, not land: %s", out)
	}
	if strings.Contains(out, "refused by posse gate") {
		t.Errorf("a clean `git merge` is not this wall's business: %s", out)
	}
	// Unstage, and the same merge runs through the shim untouched.
	fx.git("-C", fx.repo, "restore", "--staged", "--worktree", "--", "other.txt")
	if out, err := fx.shim(fx.repo, "merge", "--no-ff", "--no-edit", "side"); err != nil {
		t.Errorf("a clean merge must run through the shim: %v %s", err, out)
	}
	// And the commit wall's own safe form is unchanged.
	fx.write(fx.repo, "f.txt", "edited\n")
	if out, err := fx.shim(fx.repo, "commit", "-qm", "safe", "--", "f.txt"); err != nil {
		t.Errorf("the path-limited commit must still pass: %v %s", err, out)
	}
	if out, err := fx.shim(fx.repo, "commit", "-m", "sweep"); err == nil || !strings.Contains(out, "deny: Bash(git commit unless --)") {
		t.Errorf("the unqualified commit must still be refused under its own rule: %v %s", err, out)
	}
}

// ─── pin 5: a linked worktree stands down ────────────────────────────────────

// A linked worktree's index is the seat's own — nothing of anyone else's
// can be in it — so the wall must not fire there. This is not a nicety: a
// dispatched seat finishing a merge-back in its own worktree is the
// ordinary case (ranger-base-4d15g did exactly that, in a linked worktree
// of ranger-core), and a wall that refuses it would be the defect rather
// than the fix.
func TestQAContinueWallStandsDownInALinkedWorktree(t *testing.T) {
	t.Parallel()
	for _, v := range continueWallVerbs {
		t.Run(v.Verb, func(t *testing.T) {
			t.Parallel()
			for _, o := range v.Advance {
				t.Run(o.Long, func(t *testing.T) {
					t.Parallel()
					fx := cwSetup(t)
					wt := filepath.Join(gitTempDir(t), "wt")
					if out, err := fx.git("-C", fx.repo, "worktree", "add", "-q", "-b", "wt-"+v.Verb+o.Long, wt); err != nil {
						t.Fatalf("worktree add: %v %s", err, out)
					}
					// The discriminator, asserted rather than assumed.
					own, _ := fx.git("-C", wt, "rev-parse", "--path-format=absolute", "--git-dir")
					common, _ := fx.git("-C", wt, "rev-parse", "--path-format=absolute", "--git-common-dir")
					if strings.TrimSpace(own) == strings.TrimSpace(common) {
						t.Fatalf("fixture: %s is not a linked worktree (git-dir == git-common-dir == %q)", wt, own)
					}
					cwConflict(t, fx, wt, v.Verb, "wtside", o.Long == "--skip")
					head, _ := fx.git("-C", wt, "rev-parse", "HEAD")
					if out, err := fx.shim(wt, v.Verb, o.Long); err != nil || strings.Contains(out, "refused by posse gate") {
						t.Fatalf("git %s %s must stand down in a linked worktree: %v %s", v.Verb, o.Long, err, out)
					}
					if now, _ := fx.git("-C", wt, "rev-parse", "HEAD"); now == head {
						t.Errorf("git %s %s stood down but made no commit (%s)", v.Verb, o.Long, head)
					}
				})
			}
		})
	}
}

// ─── precedence over the sequencer audit ─────────────────────────────────────

// The arm sits in FRONT of renderSequencerAudit, which runs these same verbs
// as a CHILD: a command this wall refuses must not reach git for its own
// effect. So in a SHARED checkout a `cherry-pick --continue` is refused here
// and the audit never sees it; in a linked worktree — where ranger-base-71g2f's
// defect actually lives — the wall stands down and the audit is unchanged,
// which is what TestQASequencerVerbExitingZeroWithoutEndingIsRefused's two
// `--continue` rows now assert.
// Not t.Parallel: sequencerShim sets PATH with t.Setenv, as the other
// readers of that fixture do.
func TestQAContinueWallShadowsTheSequencerAuditInASharedCheckout(t *testing.T) {
	shim, gatesDir, gitDir := sequencerShim(t)
	out, errs, code := runSequencerShim(t, shim, gitDir, "CHERRY_PICK_HEAD", "0", "cherry-pick", "--continue")
	if code != 1 || !strings.Contains(errs, "rule: commitwall:sequencer-continue") {
		t.Fatalf("shared checkout: want the continue wall's refusal, got code=%d err=%q", code, errs)
	}
	if strings.Contains(out, "real git") {
		t.Errorf("a refused --continue must not reach git for its own effect: out=%q", out)
	}
	if strings.Contains(errs, "ranger-base-71g2f") {
		t.Errorf("the audit must not have run: %q", errs)
	}
	b, _ := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	if strings.Contains(string(b), "(alarm: ranger-base-71g2f)") {
		t.Errorf("the audit must log nothing for a command refused in front of it:\n%s", b)
	}
}

// ─── the arm rides the PID's own commit deny ─────────────────────────────────

// The continue wall IS the commit wall's other half, so it is rendered when
// and only when the PID carries the negative `git commit` rule. A PID that
// does not carry the commit wall is not given one here by the back door —
// and ADR 0001's seeded rule is what makes the arm reach every crew seat.
func TestQAContinueWallRidesTheCommitDeny(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		deny []string
		want bool
	}{
		{"the seeded commit wall", []string{"Bash(git commit unless --)"}, true},
		{"commit wall beside others", []string{"Bash(git push:*)", "Bash(git commit unless --)"}, true},
		{"a git shim with no commit wall", []string{"Bash(git push:*)"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			rules := ParseShimRules(c.deny)["git"]
			body := renderContinueWall("git", "/usr/bin/git", "/bin/date", rules)
			if got := body != ""; got != c.want {
				t.Errorf("deny %v: arm rendered=%v, want %v", c.deny, got, c.want)
			}
		})
	}
	// Not git, and no resolved real binary: the arm has to ask git which git
	// dir it is in, and a best-effort PATH search has no name to run.
	rules := ParseShimRules([]string{"Bash(git commit unless --)"})["git"]
	if body := renderContinueWall("bd", "/usr/bin/bd", "/bin/date", rules); body != "" {
		t.Errorf("the arm is git's alone, got:\n%s", body)
	}
	if body := renderContinueWall("git", "", "/bin/date", rules); body != "" {
		t.Errorf("no real binary resolved: the arm must render nothing, got:\n%s", body)
	}
}
