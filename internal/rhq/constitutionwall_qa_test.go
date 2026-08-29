package rhq

// The commit wall's constitution-path arm, measured through the rendered
// hook rather than read off the source string (ranger-base-ak3e, fixing
// ranger-base-7pq0).
//
// ranger-base-7pq0 was filed off a live event: 9dfbbd4 in the constitution
// repo edited all eleven `rhq/agents/*.md` crew PIDs from an uncaged persona
// session and nothing refused it. Every wall that existed checked something
// else — the PID deny list checks a COMMAND, the shared-index arm checks the
// commit's FORM, the land path checks bead and branch state — and the only
// path-CLASS check in the tree was seatbelt's, which is prose at the shim
// tier seven of eight personas run on.
//
// Two traps these pins are shaped around, and neither is hypothetical:
//
//   - THE WRONG ARM PASSES THE TEST. An unqualified persona commit in a
//     walled repo is refused by the shared-index arm whatever it touches, so
//     a pin that only asserted "exit 1" would be green with no constitution
//     arm in the file at all. Every commit below is PATH-LIMITED — the form
//     the shared-index arm exempts — and every assertion is on the
//     constitution refusal's own words, with the shared-index refusal
//     asserted ABSENT.
//   - A CASE LIST THAT SHRINKS WITH THE WALL. The first cut of these pins
//     generated their cases from ConstitutionRepoPaths(), which reads well
//     and measures nothing: dropping a member from the class deletes its own
//     subtest, and the mutation run came back green five times out of five.
//     So the class is WRITTEN OUT below as constitutionClassSpec — the bead's
//     specification, not the code's answer — the behavioural cases iterate
//     the spec, and one pin compares the two lists. Widening the promoted set
//     is then a deliberate two-line edit instead of a silent one.
//
// Mutation-checked per alternative (h6fx, probe-needs-a-failing-wrong-arm):
// dropping any single member from ConstitutionRepoPaths reds that member's
// subtests plus the spec pin, and removing the arm from CommitGuardHook reds
// every pin here — measured 2026-08-29, not asserted.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// constitutionClassSpec is the class ranger-base-ak3e specifies, spelled out
// so these pins have something to measure the code AGAINST. Adding a path to
// PromotedPaths widens the wall automatically and reds the spec pin below;
// the fix is to add it here too, having decided it belongs.
var constitutionClassSpec = []string{
	"rhq/agents",
	"rhq/config.yaml",
	"rhq/recipes",
	"rhq/skills",
	"rhq/envs",
	".claude/settings.json",
	".claude/settings.local.json",
}

// TestQAConstitutionClassIsTheSpecifiedSet is the join between the written
// class and the computed one. It fails in both directions on purpose: a
// member dropped from ConstitutionRepoPaths is the wall narrowing, and a
// member added is a widening nobody wrote down.
func TestQAConstitutionClassIsTheSpecifiedSet(t *testing.T) {
	got := append(ConstitutionRepoPaths(), ClaudeProjectConfig, ClaudeProjectConfigLocal)
	if strings.Join(got, " ") != strings.Join(constitutionClassSpec, " ") {
		t.Errorf("the commit wall's class is\n  %v\nand ranger-base-ak3e specifies\n  %v\n"+
			"— if the promoted set legitimately grew, widen constitutionClassSpec in the same edit",
			got, constitutionClassSpec)
	}
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

// TestQAConstitutionWallRefusesEveryClassMemberUnderAPersona is the bead's
// first DONE WHEN, one subtest per class member per shape. Each member is
// probed BOTH ways — as a file at the exact path and as a file inside it —
// because one rule in the hook covers `rhq/config.yaml` (a file) and
// `rhq/agents` (a tree), and a rule that only handled one of them would look
// identical from the source.
//
// ConstitutionRepoMarker is the one member with no "path itself" shape, and
// not by exemption: the marker IS the directory whose presence says this repo
// is the constitution, so in a repo where the class applies at all it cannot
// also be a file. Its tree shape is pinned like everyone else's.
func TestQAConstitutionWallRefusesEveryClassMemberUnderAPersona(t *testing.T) {
	for _, member := range constitutionClassSpec {
		shapes := []struct{ name, rel string }{{"a file under it", member + "/probe.md"}}
		if member != ConstitutionRepoMarker {
			shapes = append(shapes, struct{ name, rel string }{"the path itself", member})
		}
		for _, shape := range shapes {
			t.Run(member+"/"+shape.name, func(t *testing.T) {
				repo, git, persona := constitutionWallRepo(t, true)
				stageAt(t, repo, git, persona, shape.rel, "drafted\n")
				out, err := git(persona, "commit", "-m", "edit the law", "--", shape.rel)
				assertConstitutionRefusal(t, out, err, shape.rel, gatesDirOf(persona))
			})
		}
	}
}

// TestQAConstitutionWallPassesTheIdenticalCommitUnmarked is the second DONE
// WHEN and the reason the arm keys on RHQ_PERSONA at all: ADR 0015 §2/§3
// splits drafting from ratification BY ACTOR, and the operator is the
// ratifying actor. Same repo, same paths, same form — no marker, it lands.
func TestQAConstitutionWallPassesTheIdenticalCommitUnmarked(t *testing.T) {
	for _, member := range constitutionClassSpec {
		t.Run(member, func(t *testing.T) {
			repo, git, _ := constitutionWallRepo(t, true)
			rel := member + "/probe.md"
			stageAt(t, repo, git, nil, rel, "ratified\n")
			if out, err := git(nil, "commit", "-m", "operator applies it", "--", rel); err != nil {
				t.Fatalf("the operator's identical commit must pass: %v\n%s", err, out)
			}
			out, err := git(nil, "log", "-1", "--name-only", "--format=")
			if err != nil || !strings.Contains(out, rel) {
				t.Fatalf("the commit must hold %s, got %q (%v)", rel, out, err)
			}
		})
	}
}

// TestQAConstitutionWallPassesAPersonaCommitOffTheClass is the third DONE
// WHEN. Drafting is open to personas (ADR 0015 §2); the wall is a path
// class, not a session ban, and a wall that refused ordinary work in the
// constitution repo would be routed around within a day.
func TestQAConstitutionWallPassesAPersonaCommitOffTheClass(t *testing.T) {
	for _, rel := range []string{
		"docs/rca/az93-settings.json", // the prescribed route itself has to work
		"rhq/personas/developer/ORDERS.md",
		"rhq/state/gates/refusals.log",
		"scripts/thing.sh",
		"internal/rhq/gates.go",
	} {
		t.Run(rel, func(t *testing.T) {
			repo, git, persona := constitutionWallRepo(t, true)
			stageAt(t, repo, git, persona, rel, "ordinary work\n")
			if out, err := git(persona, "commit", "-m", "draft", "--", rel); err != nil {
				t.Fatalf("a persona commit touching %s must pass: %v\n%s", rel, err, out)
			}
		})
	}
}

// TestQAConstitutionWallScopesThePromotedSetToTheConstitutionRepo pins the
// two halves of the class apart. `rhq/recipes/…` is only the law in the repo
// that HAS a constitution; `.claude/settings.json` carries this session's own
// deny list in every repo it is dispatched into (ranger-base-az93).
//
// The third case is the detector's own edge, measured rather than assumed:
// the marker is `rhq/agents` EXISTING, so a persona that creates that tree in
// an unrelated repo has made the repo answer to the detector and is refused
// on the spot. That is the direction to be wrong in — a fake constitution
// nobody promotes costs a persona one refusal and a differently-named
// directory, while the other reading would let a repo be walked out of the
// class by choosing what to write first.
func TestQAConstitutionWallScopesThePromotedSetToTheConstitutionRepo(t *testing.T) {
	t.Run("no marker: rhq/recipes is ordinary work", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, false)
		rel := "rhq/recipes/thing.yaml"
		stageAt(t, repo, git, persona, rel, "not a constitution\n")
		if out, err := git(persona, "commit", "-m", "draft", "--", rel); err != nil {
			t.Fatalf("%s must pass in a repo that is not the constitution: %v\n%s", rel, err, out)
		}
	})
	t.Run("no marker: settings.json is still refused", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, false)
		stageAt(t, repo, git, persona, ClaudeProjectConfig, "{}\n")
		out, err := git(persona, "commit", "-m", "unfence myself", "--", ClaudeProjectConfig)
		assertConstitutionRefusal(t, out, err, ClaudeProjectConfig, gatesDirOf(persona))
	})
	t.Run("writing the marker makes the repo answer to the detector", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, false)
		rel := ConstitutionRepoMarker + "/developer.md"
		stageAt(t, repo, git, persona, rel, "a constitution of my own\n")
		out, err := git(persona, "commit", "-m", "seed a constitution", "--", rel)
		assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
	})
}

// TestQAConstitutionWallRefusesOnTheRootCommit is the edge the bead names:
// with no HEAD there is nothing to diff against, so the arm diffs the empty
// tree. A first commit is exactly when a constitution arrives whole, which
// makes this the one commit where getting it wrong loses the most.
func TestQAConstitutionWallRefusesOnTheRootCommit(t *testing.T) {
	repo, git, persona := commitWallRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, ConstitutionRepoMarker), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := "rhq/agents/developer.md"
	stageAt(t, repo, git, persona, rel, "the whole law at once\n")
	out, err := git(persona, "commit", "-m", "seed the constitution", "--", rel)
	assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
}

// TestQAConstitutionWallRefusesADeletion is the shape a "narrow the wall"
// edit reaches for first. `git diff --cached --name-only` reports a removed
// path exactly like a changed one, so deleting a PID is refused on the same
// terms as rewriting it — which is what it should be, since a deleted PID is
// a persona with no fence at the next launch.
func TestQAConstitutionWallRefusesADeletion(t *testing.T) {
	repo, git, persona := constitutionWallRepo(t, true)
	rel := "rhq/agents/developer.md"
	stageAt(t, repo, git, nil, rel, "the law\n")
	if out, err := git(nil, "commit", "-qm", "operator installs it", "--", rel); err != nil {
		t.Fatalf("fixture commit: %v %s", err, out)
	}
	if err := os.Remove(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	if out, err := git(persona, "add", "--", rel); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	out, err := git(persona, "commit", "-m", "delete my own fence", "--", rel)
	assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
}

// TestQAConstitutionWallRenderNamesEveryPromotedPath is the bead's fourth
// DONE WHEN, and it is about promote.go rather than about the hook: the
// class is rendered from PromotedPaths, so an edit that adds a promoted path
// widens the wall in the same commit — and an edit that REMOVES one from the
// render without removing it from the promoted set is caught here.
func TestQAConstitutionWallRenderNamesEveryPromotedPath(t *testing.T) {
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	for _, p := range PromotedPaths {
		want := ConstitutionSourceDir + "/" + p
		if !strings.Contains(render, "\n"+want+"\n") {
			t.Errorf("the rendered hook must name the promoted path %q as a class member; PromotedPaths and the wall have drifted", want)
		}
	}
	for _, want := range []string{
		ConstitutionSourceDir + "/" + ConstitutionEnvsDir,
		ClaudeProjectConfig,
		ClaudeProjectConfigLocal,
		ConstitutionRepoMarker,
	} {
		if !strings.Contains(render, want) {
			t.Errorf("the rendered hook must name %q", want)
		}
	}
}

// TestQAConstitutionWallSitsAboveTheSharedIndexArm pins the one ordering
// fact the file's comment calls load-bearing. The shared-index arm exits 0
// in a linked worktree and mid-merge; a dispatched worktree is where a
// persona's commits come from, so an arm placed below it would stand down in
// exactly the case ranger-base-7pq0 measured.
func TestQAConstitutionWallSitsAboveTheSharedIndexArm(t *testing.T) {
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	con := strings.Index(render, "the constitution-path guard")
	shared := strings.Index(render, "the shared-index guard")
	if con < 0 || shared < 0 {
		t.Fatalf("both arms must be in the render (constitution=%d shared=%d)", con, shared)
	}
	if con > shared {
		t.Errorf("the constitution arm must render ABOVE the shared-index arm, which exits 0 in a linked worktree — a persona's worktree commit would never reach it")
	}
}

// TestQAConstitutionWallRefusesInALinkedWorktree is that ordering fact
// MEASURED rather than read off the render: the arm has to fire in a
// dispatched session's own worktree, which is where every persona commit in
// the fleet is actually typed.
func TestQAConstitutionWallRefusesInALinkedWorktree(t *testing.T) {
	repo, git, persona := constitutionWallRepo(t, true)
	tree := filepath.Join(t.TempDir(), "wt")
	if out, err := git(nil, "worktree", "add", "-q", "-b", "posse/probe", tree); err != nil {
		t.Skipf("git worktree add: %v %s", err, out)
	}
	wt := func(env []string, args ...string) (string, error) {
		return git(env, append([]string{"-C", tree}, args...)...)
	}
	rel := "rhq/agents/developer.md"
	stageAt(t, tree, wt, persona, rel, "drafted in my own tree\n")
	out, err := wt(persona, "commit", "-m", "edit the law", "--", rel)
	assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
	_ = repo
}

// TestQAConstitutionWallInstallDocNamesTheWholeClass is the operator-facing
// half. INSTALL.md spells the class out for a person who has just hit the
// refusal, and a spelled-out list is a list that drifts — the widening that
// promote.go gets for free does not reach a paragraph. So the paragraph is
// read back and measured against the same spec the walls are.
func TestQAConstitutionWallInstallDocNamesTheWholeClass(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(body), "constitution-path guard")
	if start < 0 {
		t.Fatal("INSTALL.md no longer describes the constitution-path guard at all")
	}
	end := strings.Index(string(body)[start:], "\n\nIf this instance holds")
	if end < 0 {
		t.Fatal("the constitution-path guard paragraph has no end — the pin is reading the wrong span")
	}
	para := string(body)[start : start+end]
	for _, m := range constitutionClassSpec {
		if !strings.Contains(para, "`"+m+"`") {
			t.Errorf("INSTALL.md's constitution-path paragraph does not name %q — the wall widened and the doc did not", m)
		}
	}
	for _, want := range []string{"ADR 0015 §2/§3", "posse promote", "ranger-base-az93", "RHQ_PERSONA", ConstitutionRepoMarker} {
		if !strings.Contains(para, want) {
			t.Errorf("INSTALL.md's constitution-path paragraph must carry %q", want)
		}
	}
}
