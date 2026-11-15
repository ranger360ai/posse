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
// Four pins, in the order a reader wants them:
//
//   - the table is the whole class git has (derived from git, not listed);
//   - each verb's abbreviation minimum is git's own boundary;
//   - the wall refuses in a SHARED checkout and the premise still holds
//     (unguarded, each verb really does sweep);
//   - it stands down in a LINKED WORKTREE, where the index is the seat's.

import (
	"os"
	"os/exec"
	"path/filepath"
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
			// Every `--continue` that is NOT refused has to finish without a
			// terminal: git reuses the message it already has.
			"GIT_EDITOR=true"}
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
func cwConflict(t *testing.T, fx *cwFixture, dir, verb, branch string) {
	t.Helper()
	g := func(args ...string) (string, error) {
		return fx.git(append([]string{"-C", dir}, args...)...)
	}
	switch verb {
	case "merge", "rebase", "cherry-pick":
		g("checkout", "-q", "-b", branch)
		fx.write(dir, "f.txt", "theirs\n")
		g("commit", "-qm", "theirs", "--", "f.txt")
		pick, _ := g("rev-parse", "HEAD")
		g("checkout", "-q", "-")
		fx.write(dir, "f.txt", "mine\n")
		g("commit", "-qm", "mine", "--", "f.txt")
		switch verb {
		case "merge":
			g("merge", branch)
		case "cherry-pick":
			g("cherry-pick", strings.TrimSpace(pick))
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
		fx.write(dir, "f.txt", "two\n")
		g("commit", "-qm", "two", "--", "f.txt")
		g("revert", "--no-edit", strings.TrimSpace(target))
	default:
		t.Fatalf("cwConflict: unknown verb %q", verb)
	}
	fx.write(dir, "f.txt", "resolved\n")
	g("add", "--", "f.txt")
	// The other persona, staging into the same index mid-conflict.
	fx.write(dir, "other.txt", "theirs-edited\n")
	g("add", "--", "other.txt")
	if out, _ := g("diff", "--cached", "--name-only", "HEAD"); !strings.Contains(out, "other.txt") {
		t.Fatalf("%s fixture: other.txt must be staged before --continue, got %q", verb, out)
	}
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

// ─── pin 2: the abbreviation minima are git's own boundary ───────────────────

// Each verb's Min must be the SHORTEST prefix of `--continue` that git
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
	const long = "--continue"
	for _, v := range continueWallVerbs {
		if !strings.HasPrefix(long, v.Min) {
			t.Errorf("%s: Min %q is not a prefix of %q", v.Verb, v.Min, long)
			continue
		}
		for _, p := range longArms(long, v.Min) {
			if !resolves(v.Verb, p) {
				t.Errorf("git %s %s: does not resolve, so Min %q is too short — the shim renders an arm git refuses itself:\n%s",
					v.Verb, p, v.Min, run(v.Verb, p))
			}
		}
		if short := v.Min[:len(v.Min)-1]; short != "--" && resolves(v.Verb, short) {
			t.Errorf("git %s %s: resolves, so Min %q is too long and the wall has a hole one keystroke wide",
				v.Verb, short, v.Min)
		}
	}
}

// ─── pin 3: refused in a shared checkout, and the premise still holds ────────

// Two halves, like TestQACommitWallL1AbbreviationDoesNotSweepRealIndex: the
// premise unguarded (each verb really does take the other persona's staged
// file, so this dies loudly if git ever stops) and the same window through
// the shim.
func TestQAContinueWallRefusesTheFourVerbsInASharedCheckout(t *testing.T) {
	t.Parallel()
	for _, v := range continueWallVerbs {
		t.Run(v.Verb, func(t *testing.T) {
			t.Parallel()
			// Half one: unguarded, the real git sweeps other.txt.
			premise := cwSetup(t)
			cwConflict(t, premise, premise.repo, v.Verb, "side")
			if out, err := premise.git("-C", premise.repo, v.Verb, "--continue"); err != nil {
				t.Fatalf("premise: git %s --continue must finish: %v %s", v.Verb, err, out)
			}
			if files := cwHeadFiles(t, premise, premise.repo); !strings.Contains(files, "other.txt") {
				t.Fatalf("premise: git %s --continue must sweep other.txt, committed %q", v.Verb, files)
			}

			// Half two: the same window, through the shim.
			fx := cwSetup(t)
			cwConflict(t, fx, fx.repo, v.Verb, "side")
			head, _ := fx.git("-C", fx.repo, "rev-parse", "HEAD")
			out, err := fx.shim(fx.repo, v.Verb, "--continue")
			if err == nil || !strings.Contains(out, "refused by posse gate") ||
				!strings.Contains(out, "rule: commitwall:sequencer-continue") {
				t.Fatalf("git %s --continue must be refused in a shared checkout: %v %s", v.Verb, err, out)
			}
			if now, _ := fx.git("-C", fx.repo, "rev-parse", "HEAD"); now != head {
				t.Errorf("refused %s --continue moved HEAD: %q -> %q", v.Verb, head, now)
			}
			if staged, _ := fx.git("-C", fx.repo, "diff", "--cached", "--name-only", "HEAD"); !strings.Contains(staged, "other.txt") {
				t.Errorf("the other persona's staged entry must survive the refusal, got %q", staged)
			}
			if b, _ := os.ReadFile(fx.log); !strings.Contains(string(b), "(commitwall: sequencer-continue)") {
				t.Errorf("refusals.log has no line for %s --continue:\n%s", v.Verb, b)
			}
			// The abbreviation git resolves is refused the same way.
			if out, err := fx.shim(fx.repo, v.Verb, v.Min); err == nil || !strings.Contains(out, "rule: commitwall:sequencer-continue") {
				t.Errorf("git %s %s (git's own minimum) must be refused too: %v %s", v.Verb, v.Min, err, out)
			}
			// And so is the form reached through a leading global option.
			if out, err := fx.shim(fx.repo, "-C", fx.repo, v.Verb, "--continue"); err == nil ||
				!strings.Contains(out, "rule: commitwall:sequencer-continue") {
				t.Errorf("git -C <repo> %s --continue must be refused: %v %s", v.Verb, err, out)
			}
			// The way through the refusal names is NOT refused, and it is
			// what leaves the other persona's work where it was.
			if out, err := fx.shim(fx.repo, v.Verb, "--abort"); err != nil {
				t.Errorf("git %s --abort must still run: %v %s", v.Verb, err, out)
			}
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

// ─── pin 4: a linked worktree stands down ────────────────────────────────────

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
			fx := cwSetup(t)
			wt := filepath.Join(gitTempDir(t), "wt")
			if out, err := fx.git("-C", fx.repo, "worktree", "add", "-q", "-b", "wt-"+v.Verb, wt); err != nil {
				t.Fatalf("worktree add: %v %s", err, out)
			}
			// The discriminator, asserted rather than assumed.
			own, _ := fx.git("-C", wt, "rev-parse", "--path-format=absolute", "--git-dir")
			common, _ := fx.git("-C", wt, "rev-parse", "--path-format=absolute", "--git-common-dir")
			if strings.TrimSpace(own) == strings.TrimSpace(common) {
				t.Fatalf("fixture: %s is not a linked worktree (git-dir == git-common-dir == %q)", wt, own)
			}
			cwConflict(t, fx, wt, v.Verb, "wtside")
			head, _ := fx.git("-C", wt, "rev-parse", "HEAD")
			if out, err := fx.shim(wt, v.Verb, "--continue"); err != nil || strings.Contains(out, "refused by posse gate") {
				t.Fatalf("git %s --continue must stand down in a linked worktree: %v %s", v.Verb, err, out)
			}
			if now, _ := fx.git("-C", wt, "rev-parse", "HEAD"); now == head {
				t.Errorf("git %s --continue stood down but made no commit (%s)", v.Verb, head)
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
