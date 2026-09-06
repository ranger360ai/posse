package posse

// ranger-base-2asm5, filed while fixing ranger-base-xw51s one function
// down, and deliberately not fixed there: different trigger, different
// line.
//
// THE SWALLOW, and why it is the same one twice. SourceBuildStamp asked
// `git status --porcelain` under `if err == nil && st != ""`, so an
// UNREADABLE status took the same branch a CLEAN one takes: the function
// fell through to `return stamp` and named the commit alone. A dirty tree
// whose status could not be read therefore stamped byte-identically to a
// genuinely clean build of that commit, cageAge read CURRENT, and a stale
// image passed for this source with nothing printed anywhere — the false
// CURRENT cagestale.go's own header says it exists to prevent.
// ranger-base-xw51s fixed the collision one function down (every unreadable
// dirty tree hashed to the digest of nothing, so two dirty trees compared
// equal); this is the same defect in the other direction — the unreadable
// dirty tree colliding with the clean commit — and the same answer: "",
// which is the UNCLEAR verdict cageAge already carries for "one of them
// could not be read at all".
//
// THE FIXTURE, which is the part ranger-base-xw51s owed and could not
// find. That bead measured two candidates on git 2.50.1 / darwin 25.4.0 and
// neither worked: an unreadable untracked DIRECTORY makes status WARN and
// exit 0, and an unreadable tracked FILE fails the diff but not the status.
// A third candidate, measured here, does: an invalid value for a `status.*`
// config key makes every `git status` die at config-parse time (rc 128)
// while `git diff HEAD` still renders a full patch. That asymmetry is what
// this pin needs — a fixture that broke BOTH halves (a corrupt `.git/index`
// does, also measured) would leave the arms below unable to say WHICH read
// failure they caught, and the dirtyIdent arm would be a re-run of
// extdiff_qa_test.go's arm 2 rather than the status branch that bead left
// unpinned.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// badStatusValue is planted as a `status.*` value git must reject. Spelled
// once and asserted in the failure text below, so the fixture is known to
// have failed for OUR reason and not for some other thing about the repo.
const badStatusValue = "ranger-base-2asm5-not-a-mode"

// plantUnreadableStatus makes every `git status` in repo fail while leaving
// `git diff HEAD` working, and proves all three of those before any arm
// leans on it: a rig that cannot be shown to fail — or to fail in only the
// half it claims — measures nothing.
func plantUnreadableStatus(t *testing.T, repo string) {
	t.Helper()
	run := func(args ...string) ([]byte, error) {
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		c.Env = append(os.Environ(), "PATH="+PathOutsideGates(""))
		return c.CombinedOutput()
	}
	if out, err := run("config", "status.showUntrackedFiles", badStatusValue); err != nil {
		t.Fatalf("git config status.showUntrackedFiles: %v\n%s", err, out)
	}
	// The witness, on SourceBuildStamp's own spelling…
	out, err := run("status", "--porcelain")
	if err == nil {
		t.Fatalf("fixture is inert: `git status --porcelain` succeeded with status.showUntrackedFiles=%s\n%s", badStatusValue, out)
	}
	if !strings.Contains(string(out), badStatusValue) {
		t.Fatalf("status failed for some other reason than the planted value:\n%s", out)
	}
	// …and on dirtyIdent's, which passes --untracked-files=all explicitly:
	// a flag that overrode the broken config would leave that arm measuring
	// nothing.
	if out, err := run("status", "--porcelain", "--untracked-files=all", "-z", "--"); err == nil {
		t.Fatalf("fixture is inert for dirtyIdent's spelling: `git status --porcelain --untracked-files=all -z` succeeded\n%s", out)
	}
	// The half that must SURVIVE. Without this the arms below cannot say
	// which read they caught.
	if out, err := run("diff", "--no-ext-diff", "HEAD", "--"); err != nil {
		t.Fatalf("the fixture broke `git diff HEAD` too, so it cannot isolate the status read: %v\n%s", err, out)
	} else if !strings.Contains(string(out), "diff --git") {
		t.Fatalf("`git diff HEAD` rendered no patch, so the tree is not dirty and the fixture is wrong before the arm starts:\n%s", out)
	}
}

// ARM 1, the false CURRENT itself: a dirty tree whose status cannot be read
// must not stamp as the clean build of its commit.
func TestQACageStaleDoesNotStampAnUnreadableStatusAsClean(t *testing.T) {
	t.Parallel()
	src := tempGitTree(t)

	// What a genuinely CLEAN build of this commit is called — the string
	// the swallow returned, derived from the fixture rather than spelled,
	// so the arm cannot pass by comparing against the wrong constant.
	clean, cleanVersion := SourceBuildStamp(src), SourceBuildVersion(src)
	if clean == "" || strings.Contains(clean, "-dirty-") {
		t.Fatalf("clean tree stamped %q — the fixture is wrong before the arm starts", clean)
	}

	if err := os.WriteFile(filepath.Join(src, "f"), []byte("x\ndirty edit A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirty := SourceBuildStamp(src); !strings.Contains(dirty, "-dirty-") {
		t.Fatalf("dirty tree stamped %q, want a -dirty- ident — the fixture is wrong before the arm starts", dirty)
	}

	plantUnreadableStatus(t, src)

	got := SourceBuildStamp(src)
	if got == clean {
		t.Errorf("a dirty tree whose status could not be read stamped as the CLEAN build of its commit (%q) — cageAge reads that as CURRENT against an image built from the commit", got)
	}
	if got != "" {
		t.Errorf("SourceBuildStamp = %q, want \"\" (the UNCLEAR verdict for a read that failed)", got)
	}
	// End to end, in the words the operator sees: an image built from the
	// clean commit, against a source that is dirty but unreadable.
	if age := cageAge("img", "this source", cleanVersion, SourceBuildVersion(src)); age.State != CageImageUnclear {
		t.Errorf("image at the clean commit vs an unreadable dirty source is %q, want %q — %s", age.State, CageImageUnclear, age)
	}
}

// ARM 2, the branch ranger-base-xw51s added and could not pin: dirtyIdent's
// OWN status call. Its diff half is pinned by extdiff_qa_test.go's arm 2;
// under this fixture the diff succeeds, so a non-empty ident here could
// only be one hashed without the untracked half — which is the collision
// that bead removed, arriving through the other call.
func TestQACageDirtyIdentIsUnclearWhenTheStatusCannotBeRead(t *testing.T) {
	t.Parallel()
	src := tempGitTree(t)
	if err := os.WriteFile(filepath.Join(src, "f"), []byte("x\ndirty edit A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if id := dirtyIdent(src); id == "" {
		t.Fatal("no ident for a readable dirty tree — the fixture is wrong before the arm starts")
	}

	plantUnreadableStatus(t, src)

	if id := dirtyIdent(src); id != "" {
		t.Errorf("dirtyIdent fingerprinted a tree whose status it could not read: %q", id)
	}
}
