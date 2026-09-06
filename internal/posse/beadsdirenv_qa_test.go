//go:build posse_arm3

package posse

import (
	"path/filepath"
	"strings"
	"testing"
)

// ADR 0055 D5, the offline half: the store of record rides the session env.
//
// WHY THESE PINS EXIST. bd in no-db mode resolves `$BEADS_DIR` else
// `$cwd/.beads` on both the read and the write-back and never calls
// FindBeadsDir — so it consults neither the `.beads/redirect` posse seeds
// nor the worktree's main repo (measured 2026-09-04 on bd 0.50.3;
// worktree.go's WHAT WAS MEASURED block). A bead filed from a session
// worktree then lands in that worktree's own issues.jsonl while `bd where`
// names the main store, and no READ can tell the two apart, because the
// worktree's checked-out jsonl carries the main rows by construction. The
// container tier's inner bd is always `--no-db` (CageBdFlags), so that tier
// has the fork on every store class.
//
// The fix is one line in planLaunch, and what these pin is that the line is
// THERE, on every launch shape, valued at the same directory the other three
// consumers of this `dir` already resolve. They are offline by construction:
// no bd is run, and what is asserted is the assembled launch env (p.Vars),
// the rendered cage argv and the rendered writable set. The live arms — a
// real no-db create landing in the main store — are the sibling bead's
// (ranger-base-e3ima, worktreelive_test.go under RHQ_LIVE_BD).

// bdeVars indexes an assembled launch env by name.
func bdeVars(vars []EnvVar) map[string]string {
	env := map[string]string{}
	for _, v := range vars {
		env[v.Key] = v.Value
	}
	return env
}

// bdeBackend is a launcher with one persona, on its own temp home.
func bdeBackend(t *testing.T) *HerdrBackend {
	t.Helper()
	b, _ := newTestBackend(t)
	write(t, filepath.Join(b.App.AgentsDir, "ranger.md"), "---\nname: ranger\ndescription: test\n---\nYou are ranger.\n")
	return b
}

// The dispatch shape, which is the one the fork was measured in: a session
// worktree of a repo whose `.beads` is an ADR 0012 D3-C redirect into
// another repo. The launch env must name the REDIRECT TARGET — the main
// store — and must not name the worktree's own materialized `.beads`, which
// is precisely the directory a no-db bd writes to when nothing tells it
// otherwise.
func TestQADispatchLaunchEnvNamesTheStoreOfRecordNotTheWorktreeBeads(t *testing.T) {
	t.Parallel()
	b := bdeBackend(t)
	store := blRepo(t)
	work := wtRepo(t)
	write(t, filepath.Join(work, beadsDirName, "issues.jsonl"), "")
	target := filepath.Join(store, beadsDirName)
	blRedirect(t, work, target)

	p, err := b.planLaunch(NewSessionOpts{Name: "d1", Dir: work, Agent: "ranger", Worktree: true, Bead: "ranger-base-8eywa"})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	// CONTROL: a launch that made no worktree would be pinning the ordinary
	// checkout, where bd's own resolution never had the problem.
	if p.Dir == work || p.Repo != work {
		t.Fatalf("no session worktree was made — this test asserts nothing: Dir=%s Repo=%s", p.Dir, p.Repo)
	}
	if got := beadsHome(p.Dir); got != target {
		t.Fatalf("the seeded worktree redirect does not reach the store: beadsHome(%s) = %q, want %q", p.Dir, got, target)
	}

	env := bdeVars(p.Vars)
	if got := env["BEADS_DIR"]; got != target {
		t.Errorf("the dispatch launch env must name the store of record: BEADS_DIR = %q, want %q", got, target)
	}
	// The wrong arm named: the worktree's own .beads is where the fork
	// lands, and it EXISTS here, so a value that merely "points at a beads
	// dir" is not the pin.
	if got := env["BEADS_DIR"]; got == filepath.Join(p.Dir, beadsDirName) {
		t.Errorf("BEADS_DIR names the worktree's own .beads (%q) — that IS the fork, not the fix", got)
	}
}

// Crew, and a session with no persona at all. The variable is a fact about
// the DIRECTORY, not about the persona, which is why it is appended outside
// planLaunch's `ag != nil` block: move it inside and this test's second arm
// goes red while every other arm stays green.
func TestQACrewAndPersonalessLaunchesCarryBeadsDirToo(t *testing.T) {
	t.Parallel()
	b := bdeBackend(t)
	work := blRepo(t)
	want := filepath.Join(work, beadsDirName)

	p, err := b.planLaunch(NewSessionOpts{Name: "c1", Dir: work, Agent: "ranger", Crew: true, ByHand: true})
	if err != nil {
		t.Fatalf("crew launch: %v", err)
	}
	if got := bdeVars(p.Vars)["BEADS_DIR"]; got != want {
		t.Errorf("a crew session's env must name its store: BEADS_DIR = %q, want %q", got, want)
	}

	bare, err := b.planLaunch(NewSessionOpts{Name: "c2", Dir: work, Cmd: "sh", ByHand: true})
	if err != nil {
		t.Fatalf("persona-less launch: %v", err)
	}
	if got := bdeVars(bare.Vars)["BEADS_DIR"]; got != want {
		t.Errorf("a session with no persona must carry it too (the variable is the dir's, not the persona's): BEADS_DIR = %q, want %q", got, want)
	}
}

// D2: a repo with no `.beads` gets NO variable — the same decline
// seedBeadsRedirect makes, for the same reason. There is nothing to keep
// unforked, and where bd creates a store on its first write is bd's
// business, in the directory bd chooses. Setting it unconditionally would
// hand bd a path that does not exist and take that choice away.
func TestQALaunchIntoARepoWithNoBeadsCarriesNoBeadsDir(t *testing.T) {
	t.Parallel()
	b := bdeBackend(t)
	work := wtRepo(t) // git repo, no .beads
	p, err := b.planLaunch(NewSessionOpts{Name: "n1", Dir: work, Agent: "ranger", Worktree: true, Bead: "ranger-base-8eywa"})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if got, ok := bdeVars(p.Vars)["BEADS_DIR"]; ok {
		t.Errorf("a repo with no .beads must get no BEADS_DIR at all, got %q", got)
	}
	// CONTROL: the same launcher DOES set it once a .beads is there, so the
	// absence above is the guard and not a launch that never reached the line.
	write(t, filepath.Join(work, beadsDirName, "issues.jsonl"), "")
	p2, err := b.planLaunch(NewSessionOpts{Name: "n2", Dir: work, Agent: "ranger", Worktree: true, Bead: "ranger-base-8eywa"})
	if err != nil {
		t.Fatalf("control launch: %v", err)
	}
	if got := bdeVars(p2.Vars)["BEADS_DIR"]; got == "" {
		t.Errorf("CONTROL: with a .beads present the launch must carry BEADS_DIR — the absence above proves nothing otherwise")
	}
}

// The container render (D1's second half). The name crosses the boundary by
// NAME — the engine's `-e NAME` takes the value from the pane's own env — so
// what is pinned here is that CageEnvNames names it unconditionally, not
// only when the launch's own `vars` happen to carry it. That matters on the
// relaunch path, where `vars` is the env SETS and nothing else.
func TestQAContainerRenderForwardsBeadsDirByName(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		t.Fatal(err)
	}
	ag := cageAgent(t, a, "")

	// Named with no vars at all: the relaunch shape.
	if !containsString(CageEnvNames(nil), "BEADS_DIR") {
		t.Errorf("BEADS_DIR must cross the boundary by name even when vars carry nothing: %v", CageEnvNames(nil))
	}
	store := blRepo(t)
	work := blRepo(t)
	target := filepath.Join(store, beadsDirName)
	blRedirect(t, work, target)

	ms := a.CageMounts(ag, e, work, "s1")
	argv := e.RenderArgv(CageRender{Name: "posse-s1", Image: "img:tag", Workdir: work,
		Inner: []string{"sh", "-c", "exec claude"}, Mounts: ms, Env: CageEnvNames(nil)})
	if !argvHas(argv, "-e", "BEADS_DIR") {
		t.Errorf("the rendered container argv does not forward BEADS_DIR:\n%q", argv)
	}
	// The value is never on the typed line — the whole point of by-name
	// forwarding — so a render that spelled the path out is a different bug.
	for _, x := range argv {
		if strings.Contains(x, "BEADS_DIR=") {
			t.Errorf("the value must not appear on the typed line: %q", x)
		}
	}
}

// ONE RESOLVER, FOUR CONSUMERS (D1). The seatbelt writable set, the cage
// mount and the launch env all answer this `dir` with the same directory,
// and bd's own resolution is now the fourth reader of that answer. This is
// the pin that reds if a later change gives any one of them its own
// resolution — the failure class ranger-base-rhw and ranger-base-0fb both
// were: a grant and a mount over a directory bd never opens.
func TestQABeadsDirAgreesWithTheSeatbeltGrantAndTheCageMount(t *testing.T) {
	t.Parallel()
	b := bdeBackend(t)
	store := blRepo(t)
	work := wtRepo(t)
	write(t, filepath.Join(work, beadsDirName, "issues.jsonl"), "")
	target := filepath.Join(store, beadsDirName)
	blRedirect(t, work, target)

	p, err := b.planLaunch(NewSessionOpts{Name: "a1", Dir: work, Agent: "ranger", Worktree: true, Bead: "ranger-base-8eywa"})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	env := bdeVars(p.Vars)["BEADS_DIR"]
	if env == "" {
		t.Fatal("no BEADS_DIR in the launch env — the agreement below would be vacuous")
	}

	ag, err := b.App.LoadAgent("ranger")
	if err != nil {
		t.Fatal(err)
	}
	// The seatbelt's answer, for the SAME dir the launch resolved.
	if w := b.App.SeatbeltWritable(ag, p.Dir, t.TempDir()); !sbHas(w, env) {
		t.Errorf("the seatbelt writable set does not grant BEADS_DIR (%s):\n%s", env, strings.Join(w, "\n"))
	}
	// The cage's answer. `underDir(dir, home)` is false here — the store is
	// in another repo — so it is the redirect carve-out that must name it.
	e, err := b.App.LoadEngine("docker")
	if err != nil {
		t.Fatal(err)
	}
	//
	// Compared as absResolve, the way underDir and sbHas compare — the same
	// DIRECTORY, not the same string. The two spellings differ only in this
	// fixture: every path here is a t.TempDir(), i.e. behind macOS's
	// /var -> /private/var symlink, and cageOverlay resolves its carve-out
	// while the launch env keeps bd's own spelling of the answer (the
	// literal redirect content). On the box the fleet runs on, absResolve is
	// the identity for both `~/src/<repo>/.beads` and the worktree root
	// (checked 2026-09-04), so the strings agree there too.
	var mounted bool
	for _, m := range b.App.CageMounts(ag, e, p.Dir, "a1") {
		if absResolve(m.Src) == absResolve(env) && m.Src == m.Dst && !m.RO {
			mounted = true
		}
	}
	if !mounted {
		t.Errorf("the cage does not mount BEADS_DIR (%s) read-write: %+v", env, b.App.CageMounts(ag, e, p.Dir, "a1"))
	}
	// And the third: what the launch env says IS the redirect target, not
	// some other directory all three happen to agree about.
	if env != target {
		t.Errorf("all three agree on the wrong directory: %q, want the redirect target %q", env, target)
	}
}
