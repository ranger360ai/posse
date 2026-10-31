package posse

// Helpers lifted out of constitutionwall_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// constitutionClassSpec is the HAND-SPELLED half of the class, so these pins
// have something to measure the code AGAINST. Adding a path to PromotedPaths
// widens the wall automatically and reds the spec pin; the fix is to add it
// here too, having decided it belongs.
//
// It is a half and not the whole class since ranger-base-t03st, and the
// split is by reason rather than by convenience. What is spelled here is
// what someone DECIDED path by path: ADR 0015 §1's promoted set, plus
// az93's two claude settings files, whose reason is the deny list they carry
// and not the exec channel they are. The rest of the every-repo half —
// grok's, codex's and bob's project-config channels, and `.claude/plugins` —
// is DERIVED from the built-in runtimes' own declarations
// (ConstitutionEveryRepoPaths, promote.go), so there is nothing to decide
// here: a runtime that declares a repo→box exec channel is a channel the
// wall refuses, and spelling those out again would be the third copy of a
// list the bead exists to stop copying.
//
// The behavioural matrices below iterate THIS list, which is why it is worth
// keeping finite. The derived half's membership is pinned exactly
// (TestQAConstitutionClassIsTheSpecifiedSet, which reads the runtime table as
// its other source, and
// TestQAConstitutionWallRenderNamesEveryBuiltinProjectConfig), and its
// BEHAVIOUR is measured over ranger-base-ihda8's own two shapes
// (TestQAConstitutionWallRefusesAnotherRuntimesProjectConfig) rather than
// over a 16-member cross product of git fixtures that would say the same
// thing sixteen times.
var constitutionClassSpec = []string{
	ConstitutionSourceDir + "/agents",
	ConstitutionSourceDir + "/config.yaml",
	ConstitutionSourceDir + "/recipes",
	// Added 2026-09-01 by ADR 0039 D2 (ranger-base-ight8), which is the
	// "having decided it belongs" this comment block asks for: the runtime
	// overlay is read at every launch and is now promoted prose.
	ConstitutionSourceDir + "/runtimes",
	ConstitutionSourceDir + "/skills",
	ConstitutionSourceDir + "/envs",
	".claude/settings.json",
	".claude/settings.local.json",
}

// constitutionWallRepo is commitWallRepo plus the marker that makes it the
// constitution repo: a top-level ConstitutionRepoMarker directory. The
// commits are ordinary path-limited ones, which is the whole point — this
// arm has to refuse the form the shared-index arm blesses.
func constitutionWallRepo(t *testing.T, constitution bool) (repo string, git func(env []string, args ...string) (string, error), persona []string) {
	t.Helper()
	repo, git, persona = commitWallRepo(t)
	if constitution {
		if err := os.MkdirAll(filepath.Join(repo, ConstitutionRepoMarker), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A HEAD to diff against. The no-HEAD case has its own pin below.
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "--", "seed.txt")
	if out, err := git(nil, "commit", "-qm", "seed", "--", "seed.txt"); err != nil {
		t.Fatalf("fixture commit: %v %s", err, out)
	}
	return repo, git, persona
}

// stageAt writes body at rel (creating parents) and stages exactly that path.
// A NEW file needs the add before the path-limited commit can match it
// (rangerhq-4pbt), and the add is scoped to the one path.
func stageAt(t *testing.T, repo string, git func(env []string, args ...string) (string, error), env []string, rel, body string) {
	t.Helper()
	full := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(env, "add", "--", rel); err != nil {
		t.Fatalf("git add %s: %v %s", rel, err, out)
	}
}

// constitutionRefusalMarks are the words the arm's refusal must carry: what
// it is, the ADR that rules it, the az93 route, and the promise that the
// hook left the tree alone.
var constitutionRefusalMarks = []string{
	"refused by posse gate: a persona commit touching the constitution",
	"ADR 0015 §2/§3",
	"posse promote",
	"ranger-base-az93",
	"the way through — stage the intended diff, the operator applies it",
	"Nothing here has been reset, unstaged or cleaned up",
}

// assertConstitutionRefusal is the whole verdict in one place: refused, by
// THIS arm (not the shared-index one), naming the path, and logged.
func assertConstitutionRefusal(t *testing.T, out string, err error, wantPath, gatesDir string) {
	t.Helper()
	if err == nil {
		t.Fatalf("a persona commit touching %s must be refused:\n%s", wantPath, out)
	}
	if strings.Contains(out, "This working tree's .git/index is shared") {
		t.Fatalf("refused by the SHARED-INDEX arm, not the constitution arm — the pin is measuring the wrong wall:\n%s", out)
	}
	for _, want := range append(append([]string(nil), constitutionRefusalMarks...), wantPath) {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q, got:\n%s", want, out)
		}
	}
	if gatesDir == "" {
		return
	}
	log, err := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	if err != nil || !strings.Contains(string(log), "constitution path in a persona commit") {
		t.Errorf("the refusal must be recorded in refusals.log, got %q (%v)", string(log), err)
	}
}

// gatesDirOf digs RHQ_GATES_DIR back out of the persona env commitWallRepo
// hands over, so the log assertion does not need a second constructor.
func gatesDirOf(env []string) string {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "RHQ_GATES_DIR="); ok {
			return v
		}
	}
	return ""
}
