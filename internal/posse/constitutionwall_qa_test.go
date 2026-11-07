//go:build posse_arm3

package posse

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

// TestQAConstitutionClassIsTheSpecifiedSet is the join between the class the
// code computes and a reading of it taken from somewhere else. It fails in
// both directions on purpose: a member dropped from the class is the wall
// narrowing, and a member added is a widening nobody wrote down.
//
// The other reading is TWO sources, because the class has two halves with
// two different reasons (ranger-base-t03st):
//
//   - constitutionClassSpec — hand-spelled. The promoted half, which ADR
//     0015 §1 decides, plus az93's two claude settings files, whose reason
//     is the deny list and not the exec channel.
//   - every built-in runtime's own Runtime.ProjectConfig declaration — the
//     repo→box executable channels (ADR 0002). This is the source
//     ConstitutionEveryRepoPaths derives FROM, read here through the
//     runtime table rather than through that function, so dropping a member
//     out of the derivation still reds this.
//
// Compared as a SET: the class is everyRepo-then-repo and the two sources
// here are in neither order, and an order this pin fixed would be an order
// the wall could not change without a bead.
func TestQAConstitutionClassIsTheSpecifiedSet(t *testing.T) {
	t.Parallel()
	want := map[string]bool{}
	for _, m := range constitutionClassSpec {
		want[m] = true
	}
	for i := range builtinRuntimes {
		for _, p := range builtinRuntimes[i].ProjectConfig {
			want[p] = true
		}
	}
	got := map[string]bool{}
	for _, m := range ConstitutionClassIn(constitutionRepoTop(t)) {
		got[m] = true
	}
	for m := range want {
		if !got[m] {
			t.Errorf("the commit wall's class is missing %q — it is specified by constitutionClassSpec or declared by a built-in runtime's ProjectConfig, and the wall narrowed", m)
		}
	}
	for m := range got {
		if !want[m] {
			t.Errorf("the commit wall's class carries %q, which no built-in runtime declares and constitutionClassSpec does not name — a widening nobody wrote down; if it is right, write it down in the same edit", m)
		}
	}
}

// TestQAConstitutionWallRefusesEveryClassMemberUnderAPersona is the bead's
// first DONE WHEN, one subtest per class member per shape. Each member is
// probed BOTH ways — as a file at the exact path and as a file inside it —
// because one rule in the hook covers `<dir>/config.yaml` (a file) and
// `<dir>/agents` (a tree), and a rule that only handled one of them would look
// identical from the source.
//
// ConstitutionRepoMarker is the one member with no "path itself" shape, and
// not by exemption: the marker IS the directory whose presence says this repo
// is the constitution, so in a repo where the class applies at all it cannot
// also be a file. Its tree shape is pinned like everyone else's.
func TestQAConstitutionWallRefusesEveryClassMemberUnderAPersona(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	for _, rel := range []string{
		"docs/notes.d/az93-settings.json", // the prescribed route itself has to work
		ConstitutionSourceDir + "/personas/developer/ORDERS.md",
		ConstitutionSourceDir + "/state/gates/refusals.log",
		"scripts/thing.sh",
		"internal/posse/gates.go",
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
// two halves of the class apart. `<ConstitutionSourceDir>/recipes/…` is only
// the law in the repo
// that HAS a constitution; `.claude/settings.json` carries this session's own
// deny list in every repo it is dispatched into (ranger-base-az93).
//
// The third case is the detector's own edge, measured rather than assumed:
// the marker is ConstitutionRepoMarker EXISTING, so a persona that creates
// that tree in
// an unrelated repo has made the repo answer to the detector and is refused
// on the spot. That is the direction to be wrong in — a fake constitution
// nobody promotes costs a persona one refusal and a differently-named
// directory, while the other reading would let a repo be walked out of the
// class by choosing what to write first.
func TestQAConstitutionWallScopesThePromotedSetToTheConstitutionRepo(t *testing.T) {
	t.Parallel()
	t.Run("no marker: recipes is ordinary work", func(t *testing.T) {
		repo, git, persona := constitutionWallRepo(t, false)
		rel := ConstitutionSourceDir + "/recipes/thing.yaml"
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
	t.Parallel()
	repo, git, persona := commitWallRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, ConstitutionRepoMarker), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := ConstitutionRepoMarker + "/developer.md"
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
	t.Parallel()
	repo, git, persona := constitutionWallRepo(t, true)
	rel := ConstitutionRepoMarker + "/developer.md"
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
	t.Parallel()
	render := qaNoHome.CommitGuardHook(VisibilityPublic, OpsPatternSet{})
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

// TestQAConstitutionWallRenderNamesEveryBuiltinProjectConfig is the same
// property for the every-repo half, which is derived rather than listed
// since ranger-base-t03st: every path a BUILT-IN runtime declares as its
// repo→box executable configuration channel (ADR 0002 "Project
// configuration is a launch input") is in the class and is a whole line of
// the rendered hook.
//
// It reads the runtime table, not ConstitutionEveryRepoPaths, for the reason
// the file's preamble gives about case lists that shrink with the wall: a
// pin generated from the function under test is green the moment the
// function drops a member. The declaration is the independent source — a
// runtime that declares a channel and a wall that does not refuse it is
// exactly ranger-base-ihda8, which this bead closes.
//
// ONLY THE BUILT-INS. ConstitutionEveryRepoPaths says why at length: the
// hook is rendered at install and the land belt runs at close, and the
// `runtimes/<name>.yaml` overlay can differ between those two moments. A
// template runtime's project_config is covered where it is read, at the
// launch check.
func TestQAConstitutionWallRenderNamesEveryBuiltinProjectConfig(t *testing.T) {
	t.Parallel()
	render := qaNoHome.CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	class := ConstitutionClassIn(constitutionRepoTop(t))
	plain := ConstitutionClassIn(t.TempDir())
	for i := range builtinRuntimes {
		rt := builtinRuntimes[i]
		for _, p := range rt.ProjectConfig {
			t.Run(rt.Name+"/"+p, func(t *testing.T) {
				if got := InConstitutionClass(class, p); got != p {
					t.Errorf("%s declares %s as a project-config channel; ConstitutionClassIn puts it under %q, want %q (ranger-base-ihda8)", rt.Name, p, got, p)
				}
				// EVERY hooked repo, not only the constitution's: a grok
				// seat reads its session directory wherever it is
				// dispatched.
				if got := InConstitutionClass(plain, p); got != p {
					t.Errorf("%s's %s is out of the class in a repo that is not the constitution, which is every repo the fleet is dispatched into", rt.Name, p)
				}
				if !strings.Contains(render, "\n"+p+"\n") {
					t.Errorf("the rendered hook must name %q as a whole line of the class; the runtime table and the wall have drifted", p)
				}
			})
		}
	}
}

// TestQAConstitutionWallRefusesAnotherRuntimesProjectConfig is the every-repo
// widening MEASURED through a real commit rather than read off the render,
// and it is ranger-base-ihda8's own two shapes: before this bead both landed
// at exit 0, in any repo, and the launcher fast-forwarded them onto main at
// close.
//
// `.grok/hooks/x.json` is the directory-prefix shape and `.mcp.json` the
// exact-path one, which between them exercise both halves of the hook's one
// `case` rule over a class member that is NOT claude's.
//
// Neither repo carries the constitution marker: the point of this half of the
// class is that it holds in every hooked repo, and a fixture with the marker
// would be satisfied by a wall that only ever had the promoted set.
//
// The CONTROL is the same commit with no RHQ_PERSONA, and it is not
// decoration: ADR 0015 §2/§3 splits drafting from ratification by ACTOR, and
// an operator who cannot write a hook or an MCP server into their own repo
// has had a tool taken away rather than a wall put up.
func TestQAConstitutionWallRefusesAnotherRuntimesProjectConfig(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{".grok/hooks/x.json", ".mcp.json"} {
		t.Run(rel, func(t *testing.T) {
			t.Run("a persona is refused", func(t *testing.T) {
				repo, git, persona := constitutionWallRepo(t, false)
				stageAt(t, repo, git, persona, rel, "{}\n")
				out, err := git(persona, "commit", "-m", "land a channel every later seat reads", "--", rel)
				assertConstitutionRefusal(t, out, err, rel, gatesDirOf(persona))
			})
			t.Run("CONTROL the operator's identical commit lands", func(t *testing.T) {
				repo, git, _ := constitutionWallRepo(t, false)
				stageAt(t, repo, git, nil, rel, "{}\n")
				if out, err := git(nil, "commit", "-m", "operator configures the repo", "--", rel); err != nil {
					t.Fatalf("the operator's own commit of %s must land: %v\n%s", rel, err, out)
				}
				out, err := git(nil, "log", "-1", "--name-only", "--format=")
				if err != nil || !strings.Contains(out, rel) {
					t.Fatalf("the commit must hold %s, got %q (%v)", rel, out, err)
				}
			})
		})
	}
}

// TestQAConstitutionWallSitsAboveTheSharedIndexArm pins the one ordering
// fact the file's comment calls load-bearing. The shared-index arm exits 0
// in a linked worktree and mid-merge; a dispatched worktree is where a
// persona's commits come from, so an arm placed below it would stand down in
// exactly the case ranger-base-7pq0 measured.
func TestQAConstitutionWallSitsAboveTheSharedIndexArm(t *testing.T) {
	t.Parallel()
	render := qaNoHome.CommitGuardHook(VisibilityPublic, OpsPatternSet{})
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
	t.Parallel()
	repo, git, persona := constitutionWallRepo(t, true)
	tree := filepath.Join(t.TempDir(), "wt")
	if out, err := git(nil, "worktree", "add", "-q", "-b", "posse/probe", tree); err != nil {
		t.Skipf("git worktree add: %v %s", err, out)
	}
	wt := func(env []string, args ...string) (string, error) {
		return git(env, append([]string{"-C", tree}, args...)...)
	}
	rel := ConstitutionRepoMarker + "/developer.md"
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
	t.Parallel()
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
	// Both halves of the class, and the derived half from the same source
	// the other pins read it from — the operator reading this paragraph has
	// just hit the refusal and needs the whole list, not the half somebody
	// spelled out by hand (ranger-base-t03st).
	named := append([]string(nil), constitutionClassSpec...)
	for i := range builtinRuntimes {
		named = append(named, builtinRuntimes[i].ProjectConfig...)
	}
	for _, m := range named {
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

// TestQAConstitutionWallHoldsOverHostilePathShapes is the arm's shell read as
// shell: every place a path could stop being a path on its way from `git
// diff` to the `case` arm — a glob character becoming a pattern, a space
// splitting a word, git's own quoting wrapping the name, a leading dash
// reaching an option parser.
//
// The CONTROL is what makes the rest mean anything: an ordinary persona
// commit in the same repo, in the same path-limited form, must still land.
// Without it every row above is satisfied by a wall that refuses everything,
// which is the shape a broken `case` arm actually takes.
//
// WHAT THIS CORPUS MEASURES, mutation-checked rather than claimed
// (2026-08-29, three defences broken one at a time):
//
//	`-z` and the `tr`      LOAD-BEARING. Drop them and git's own path
//	                       quoting wraps the name — the quote, backslash,
//	                       UTF-8 and newline rows go red, and each of those
//	                       is a constitution commit landing.
//	`set -f`               not measured here. Dropping it keeps every row
//	                       green: a staged path that globs expands to files
//	                       under the same class member, so the failure is
//	                       over-matching, which this class cannot turn into
//	                       a hole.
//	IFS narrowed to \n     not measured here either, and for the same
//	                       reason: `<marker>/a b.md` split on the space
//	                       leaves `<marker>/a`, which the prefix arm still
//	                       catches.
//
// Both unmeasured defences stay. They are correct and they cost nothing, and
// "fail-closed for THIS class" is a property of today's class rather than of
// the code — but nothing here proves them, and a later reader must not take
// a green run as if it did.
func TestQAConstitutionWallHoldsOverHostilePathShapes(t *testing.T) {
	t.Parallel()
	repo, git, persona := constitutionWallRepo(t, true)
	for _, name := range []string{
		ConstitutionRepoMarker + "/a b.md",
		ConstitutionRepoMarker + "/a*c.md",
		ConstitutionRepoMarker + "/a[b].md",
		ConstitutionRepoMarker + "/a?b.md",
		ConstitutionRepoMarker + "/a\"b.md",
		ConstitutionRepoMarker + "/a\\b.md",
		ConstitutionRepoMarker + "/é—ü.md",
		ConstitutionRepoMarker + "/a$b.md",
		ConstitutionRepoMarker + "/-x.md",
		ConstitutionRepoMarker + "/a\nb.md",
		ConstitutionRepoMarker + "/nested/deep/x.md",
	} {
		t.Run(strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			full := filepath.Join(repo, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Skipf("this filesystem will not hold %q: %v", name, err)
			}
			if err := os.WriteFile(full, []byte("drafted\n"), 0o644); err != nil {
				t.Skipf("this filesystem will not hold %q: %v", name, err)
			}
			defer os.Remove(full)
			if out, err := git(persona, "add", "--", name); err != nil {
				t.Skipf("git will not stage %q: %v %s", name, err, out)
			}
			defer git(persona, "restore", "--staged", "--", name)
			out, err := git(persona, "commit", "-m", "edit the law", "--", name)
			if err == nil {
				t.Fatalf("a persona commit touching %q landed — the path stopped being a path somewhere in the arm's shell:\n%s", name, out)
			}
			if !strings.Contains(out, "a persona commit touching the constitution") {
				t.Errorf("refused, but not by this arm — %q:\n%s", name, out)
			}
		})
	}
	t.Run("CONTROL ordinary work still lands", func(t *testing.T) {
		// docs/adr/, not a bare docs/note.md: a staged NEW file directly under
		// docs/ with no subdirectory has no genre and check 1 (ADR 0024 D2)
		// refuses it on its own — this control needs a path outside BOTH the
		// constitution class and the docs-genre allowlist's refusal.
		stageAt(t, repo, git, persona, "docs/adr/note.md", "ordinary\n")
		if out, err := git(persona, "commit", "-m", "draft", "--", "docs/adr/note.md"); err != nil {
			t.Fatalf("the wall refuses everything, which makes every row above meaningless: %v\n%s", err, out)
		}
	})
}
