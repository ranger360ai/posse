package main

// QA pin for ranger-base-w79sc — ADR 0082 D4 at the second of the two sites
// it names: the path `posse gates install-hooks` prescribes when it refuses
// another home's wall is spelled the way the kernel names the directory,
// however the operator spelled the target.
//
// WHAT WAS WRONG. The refusal and its remedy printed
// `AbbrevHome(hookRepo(dir))` — the argument as typed — beside the owning
// home taken from the wall's `# posse-home:` stamp, which ADR 0082 D3 had
// already made canonical. So one line showed two paths in two spellings, and
// the `beads_visibility:` key it told the operator to paste depended on which
// symlink they happened to walk in through.
//
// WHY A THIRD SPELLING PIN IN THIS PACKAGE, and what each of the three owns:
//
//	installhooksrelativetarget_qa_test.go  relative vs absolute, at the WALL
//	                                       (byte equality of two renders)
//	commitwallowner_qa_test.go's cwd arm   the refusal names a repo and not
//	                                       `.` — absolute, by substring
//	this file                              the refusal names the repo as the
//	                                       KERNEL spells it — by equality
//
// The middle one asserts absoluteness and must keep doing so: it was written
// while "should this site canonicalise too?" was open (ranger-base-wvoqe),
// and a `.` resolved against the cwd is already absolute without being
// canonical. D4 answered that question yes, so canonicalness is a separate
// claim and this is where it is made.
//
// WHY THE ARGUMENT'S SPELLING STILL DOES NOT ENTER THE RULE, asserted here so
// nobody reads this pin as a widening of D4's own scope: `dir` reaches
// ForeignCommitWallOwner and InstallCommitGuardHook exactly as typed. This
// arm's own control is that the refusal happens AT ALL through the symlinked
// spelling, and that the wall it refused to write is untouched — the same
// two facts the sibling arms assert through the plain spelling.
//
// MUTATION-CHECKED 2026-11-08, darwin 25.6.0: reverting main.go's two
// Fprintf lines to `posse.AbbrevHome` (D4 undone at this site) reds the first
// arm three times over — the repo the refusal names, the key it prescribes,
// and the symlinked spelling still being there — and the second arm once, on
// the byte equality of the two refusals. It leaves all eleven other
// install-hooks arms green, commitwallowner_qa_test.go's cwd arm and
// installhooksrelativetarget_qa_test.go's byte equality among them, which is
// what says those three pins measure three different claims.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// ihrsPrescribedKey captures the `beads_visibility:` key the remedy tells the
// reader to add. Read back out of the output rather than assumed: the whole
// finding is that this one substring was the caller's spelling, and a test
// that only asked whether the output contains SOME canonical path would pass
// on the home stamp, which was canonical before this bead.
var ihrsPrescribedKey = regexp.MustCompile("add `([^`]+): <private\\|public>`")

// ihrsGuardLines is the commit-guard arm's output and nothing else: the
// `not installed: prepare-commit-msg` line and the indented lines under it.
//
// IT IS NOT A TIDINESS FILTER. The pre-push arm above it reports the SLOT IT
// WROTE — `<target>/.git/hooks/pre-push`, derived from the argument by git —
// and that path carries the caller's spelling, correctly: it is a file this
// run created and not a path D4 prescribes for pasting into config, and ADR
// 0082 D4 is scoped to the latter ("a path that a refusal or remedy prints
// for the operator to paste"). MEASURED 2026-11-08: with this fix in place
// that line reads `installed <symlink>/checkout/.git/hooks/pre-push` while
// the refusal under it reads the kernel's spelling, which is the pair this
// helper exists to keep apart. Whether an installed-slot report should be
// canonical too is a different question about a different kind of path, and
// nobody has asked it.
func ihrsGuardLines(s string) string {
	var keep []string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "prepare-commit-msg") || strings.HasPrefix(line, "  ") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

func TestQAInstallHooksRefusalPrescribesTheRepoNotTheSymlinkItWasReachedBy(t *testing.T) {
	t.Parallel()
	bin, owner, other, repo := cwoFixture(t)
	path, before, beforeTime := cwoArm(t, bin, owner, repo)

	// The target, spelled through a symlink to the repo's parent. The link
	// lives in a temp dir of its own rather than inside the fixture root, so
	// nothing under test is reached through a directory that contains a link
	// to itself.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Dir(repo), link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	target := filepath.Join(link, filepath.Base(repo))
	// The premise: the two spellings name one directory and are not one
	// string. Without this the arm would pass on a binary that printed its
	// argument verbatim.
	if resolved, err := filepath.EvalSymlinks(target); err != nil || resolved != repo {
		t.Fatalf("fixture premise: %s resolves to (%q, %v), want the repo %s", target, resolved, err, repo)
	}

	// CanonicalAbbrevPath is the claim, so it is asked of the SAME helper the
	// binary renders through — a second copy of the rule here would be a
	// second implementation to keep in step, in the one place where the two
	// are compared for equality. It reads $HOME, and this process's is not
	// the subprocess's: a repo under either would be abbreviated by one and
	// not the other, and the assertion would be measuring that instead.
	want := posse.CanonicalAbbrevPath(repo)
	if !filepath.IsAbs(want) {
		t.Skipf("fixture premise: this process's $HOME abbreviates the repo to %q, and the subprocess's would not", want)
	}

	out, code := cwoRun(t, bin, other, target)

	// The controls first — the refusal happened, and it refused by leaving
	// the wall alone. D4's rule reads `dir` as typed, and an argument this
	// pin varies must not change the answer.
	if code == 0 {
		t.Errorf("exit 0 — a non-owning home rewrote %s's wall through a symlinked target:\n%s", repo, out)
	}
	if _, after, afterTime := cwoSlot(t, repo); after != before || afterTime != beforeTime {
		t.Errorf("%s moved: bytes equal=%v, mtime %d → %d", path, after == before, beforeTime, afterTime)
	}

	// THE CLAIM, over the commit-guard lines alone (see ihrsGuardLines, which
	// says why the pre-push line is not one of them). Both places: the repo
	// the refusal names, and the key the remedy prescribes.
	guard := ihrsGuardLines(out)
	if !strings.Contains(guard, want) {
		t.Errorf("the refusal does not name the repo as %s, which is how the identity wall in that same repo spells it:\n%s", want, guard)
	}
	key := ihrsPrescribedKey.FindStringSubmatch(guard)
	if key == nil {
		t.Fatalf("the remedy no longer prescribes a `<repo>: <private|public>` key, which is the config line that is the whole of D4's first remedy:\n%s", guard)
	}
	if key[1] != want {
		t.Errorf("the remedy prescribes the key %q, want %q — the spelling the identity wall of that repo already prints, so an operator who greps for one finds one:\n%s", key[1], want, guard)
	}
	// And the spelling the operator typed appears in neither place: both
	// assertions above would pass on a refusal that carried each path twice.
	if strings.Contains(guard, target) {
		t.Errorf("the refusal still carries the typed spelling %s beside the canonical one:\n%s", target, guard)
	}
}

// And the control the three arms above cannot give: the same command, same
// home, same repo, refusing through the PLAIN spelling prints byte-identical
// lines. That is what makes the fix a render and not a second message —
// MEASURED, where ADR 0082's "every spelling prints the same refusal, byte
// for byte" is otherwise only asserted.
func TestQAInstallHooksRefusesIdenticallyThroughEitherSpelling(t *testing.T) {
	t.Parallel()
	bin, owner, other, repo := cwoFixture(t)
	cwoArm(t, bin, owner, repo)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Dir(repo), link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}

	plain, plainCode := cwoRun(t, bin, other, repo)
	viaLink, linkCode := cwoRun(t, bin, other, filepath.Join(link, filepath.Base(repo)))
	if plainCode == 0 || linkCode == 0 {
		t.Fatalf("fixture premise: the refusal must fire both ways, got exit %d and %d:\n%s\n%s", plainCode, linkCode, plain, viaLink)
	}
	if g, l := ihrsGuardLines(plain), ihrsGuardLines(viaLink); g != l {
		t.Errorf("one repo, two spellings, two refusals:\n--- install-hooks %s\n%s\n--- install-hooks <symlink>/%s\n%s", repo, g, filepath.Base(repo), l)
	}
}
