package posse

// ranger-base-esa0j (audit finding 13): four runtime-visible arms the record
// ratifies and nothing asserted. Each arm names the mutations that red it.
//
// Arm 1 (ADR 0025 §3's push-effect note) lives in parityorder_test.go, where
// the bead asked for it.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ─── arm 2: the shims-tier relaunch carries the L1 prefix ────────────────────

// This arm reads rawCalls (launchlock_test.go) — the fake herdr's call log
// VERBATIM — and not the house helper `calls`, which collapses every gate
// prefix to "GATES " before any assertion sees it. `calls`'s own guard
// (`!gatePrefixRe.Match(b) && strings.Contains(b, ":\"$PATH\" ")`) cannot
// fire when the prefix is missing ENTIRELY, because the second half is then
// false too — so every existing relaunch test is blind to exactly the
// regression this arm is about.

// ADR 0002 §3's L1 wall on the OTHER path that types a persona line.
// RelaunchAgent (herdrback.go) re-renders and re-types the whole command
// into a live pane after a persona's CLI died; at `cage: shims` the wall is
// the typed prefix and nothing else — no seatbelt, no container. Only the
// CONTAINER shape of a relaunch was pinned (TestCagedRelaunchRetypesTheLauncher),
// and that shape takes a different branch: WrapInCage, not WrapWithGates.
//
// A relaunch that dropped the prefix would revive the persona with its whole
// deny list unrealized, into a pane the operator is already watching, with
// nothing said. The launch and the relaunch are asserted to render the SAME
// prefix, which is the property that survives a refactor of either.
//
// MUTATIONS RUN (each reds one of the two tests here): drop `GatePrefix(binDir, typed) +` in
// WrapWithGates; return `""` for `typed` unconditionally there (kills the
// SHELL/GROK_SHELL half only); swap RelaunchAgent's `WrapWithGates` arm for a
// bare `cmd = inner`.
func TestQAShimsRelaunchRetypesTheGatePrefix(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	os.MkdirAll(a.AgentsDir, 0o755)
	// No `cage:` line at all — the default tier, which is where L1 is the
	// whole wall. A deny is present so the shims have something to refuse.
	os.WriteFile(filepath.Join(a.AgentsDir, "plain.md"),
		[]byte("---\nname: plain\ndescription: test\ndeny: [Bash(git push:*)]\n---\nYou are plain.\n"), 0o644)

	ag, err := a.LoadAgent("plain")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "pl", Agent: "plain", Dir: dir})
	m, ok := b.readMeta("pl")
	if !ok {
		t.Fatal("no session meta for pl")
	}
	if m.Cage == CageContainer {
		t.Fatalf("fixture: this arm is about the shims tier, got cage %q", m.Cage)
	}
	// The line lands in one of TWO places and which one is a fact about its
	// LENGTH (paneline.go): over PaneLineMax it is spilled to
	// state/launch/<session>.sh and the pane types `. <script>`, so the
	// prefix this pin is about is IN THE SCRIPT rather than in calls.log.
	// Both stages are therefore read as "what was typed, plus what the
	// script held at that moment" — the script is rewritten at each launch,
	// so it has to be captured before the relaunch overwrites it
	// (ranger-base-rq83c moved this fixture across that cliff).
	launched := rawCalls(t, fake) + spilled(t, a, "pl")

	// Past the grace, and with no agent in the pane: the two guards
	// RelaunchAgent refuses on.
	m.Launched = time.Now().Add(-time.Hour)
	b.writeMeta(m)
	os.Remove(filepath.Join(fake, "agents.json"))
	if ok, err := b.RelaunchAgent("pl", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	typedNow := rawCalls(t, fake)
	retyped := strings.TrimPrefix(typedNow, strings.TrimSuffix(launched, spilled(t, a, "pl"))) + spilled(t, a, "pl")
	if !strings.Contains(typedNow, "pane run ") || strings.Count(typedNow, "pane run ") < 2 {
		t.Fatalf("fixture: the relaunch typed nothing new:\n%s", typedNow)
	}

	// The prefix itself, on the retyped line — ADR 0002 §3's PATH= and ADR
	// 0009 §2's SHELL=/GROK_SHELL=, all three pointing inside RHQ_HOME. The
	// paths come from the same renderer the launch used rather than being
	// spelled here, so a moved gates dir is not this pin's failure.
	_, bin, shell, err := a.RenderGates("plain", ag.Deny)
	if err != nil {
		t.Fatal(err)
	}
	if shell == "" {
		t.Fatal("fixture: claude renders a gate shell; with none the SHELL= arms below measure nothing")
	}
	want := GatePrefix(bin, shell)
	if !strings.Contains(retyped, "PATH="+shQuote(bin)+`:"$PATH" `) {
		t.Errorf("the relaunch must re-type ADR 0002 §3's PATH= prefix:\n%s", retyped)
	}
	for _, v := range []string{"SHELL=", "GROK_SHELL="} {
		if !strings.Contains(retyped, v+shQuote(shell)+" ") {
			t.Errorf("the relaunch must re-type ADR 0009 §2's %s at the gate shell:\n%s", v, retyped)
		}
	}
	// Whole and in one piece, in the launch's own spelling: a prefix
	// assembled differently on this path is a second thing to keep right.
	if !strings.Contains(retyped, want) {
		t.Errorf("relaunch prefix must be the launch's, verbatim:\nwant %q\n%s", want, retyped)
	}
	if !strings.Contains(launched, want) {
		t.Errorf("fixture: the LAUNCH did not carry it either, so the line above measured nothing:\n%s", launched)
	}
	// And the shims tier really is the shims tier: no cage launcher on the
	// line, so this is the WrapWithGates arm and not the caged one.
	if strings.Contains(retyped, CageLaunchFlag) {
		t.Errorf("fixture: this relaunch took the container branch:\n%s", retyped)
	}
}

// ─── arm 3: ADR 0028 §4 — one throttle ───────────────────────────────────────

// "All refills originate in the one watch process. Killing it stops the shop
// inviting work; no agent-initiated launch path exists." A ratified
// invariant, not an implementation accident — and the observable (§5.2,
// "watch process killed → zero new launches") had no witness: every refill
// test in this package sets `d.Refill = true` on its own dispatcher, which is
// exactly the thing production is not allowed to do anywhere but Watch.
//
// Two halves, because neither alone is the invariant. The census answers
// "who can arm it"; the behavioural arm answers "and when the loop dies, does
// it stop".
//
// MUTATIONS RUN: add a second `d.Refill = true` in dispatch.go → census reds;
// move `d.refire(` behind a second call site → census reds; delete the
// `d.stopping()` early return in the gather's judge (passcarry.go) → the
// behavioural arm reds.
func TestQAOneThrottleOnlyWatchArmsTheRefill(t *testing.T) {
	t.Parallel()
	// Every non-test .go file in this package, as bytes. `go test` runs
	// with the package dir as cwd, so "." is internal/posse.
	var src []struct{ name, body string }
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		src = append(src, struct{ name, body string }{n, string(b)})
	}
	if len(src) < 20 {
		t.Fatalf("fixture: only %d source files found — the census below would pass by looking at nothing", len(src))
	}

	// The setter. `Refill` is the field Run reads to decide it may refire;
	// anything that can set it true is a second throttle.
	setter := regexp.MustCompile(`\.Refill\s*=\s*true`)
	var setters []string
	for _, f := range src {
		for range setter.FindAllString(f.body, -1) {
			setters = append(setters, f.name)
		}
	}
	if len(setters) != 1 || setters[0] != "watch.go" {
		t.Errorf("ADR 0028 §4: exactly one place may arm the refill, and it is Watch's loop — found %v", setters)
	}

	// The caller. refire is the refill; a call site outside Run's own gather
	// loop is a launch path Watch does not own.
	caller := regexp.MustCompile(`\bd\.refire\(`)
	var callers []string
	for _, f := range src {
		for range caller.FindAllString(f.body, -1) {
			callers = append(callers, f.name)
		}
	}
	// passcarry.go since ranger-base-3ryit: the gather that owns this call
	// moved out of Run with the carry, and `judge` is reached from nowhere
	// but Run's own gather round. The invariant is unchanged and so is
	// everything below it — one caller, behind the Refill gate, in the same
	// function, on the pass's own goroutine.
	if len(callers) != 1 || callers[0] != "passcarry.go" {
		t.Fatalf("ADR 0028 §4: refire has exactly one caller, in the pass's own gather — found %v", callers)
	}
	// …and that one call site is behind the Refill gate, in the same
	// function. A refire reachable with Refill unset is a one-shot pass that
	// refills, which is the invariant read backwards.
	held := ""
	for _, f := range src {
		if f.name == callers[0] {
			held = f.body
		}
	}
	at := strings.Index(held, "d.refire(")
	if at < 0 {
		t.Fatalf("fixture: %s was censused for the call and does not contain it", callers[0])
	}
	gate := strings.LastIndex(held[:at], "if !d.Refill {")
	fn := strings.LastIndex(held[:at], "\nfunc ")
	if gate < 0 || gate < fn {
		t.Errorf("the refire call must sit behind `if !d.Refill {` in the same function (refire@%d gate@%d func@%d)", at, gate, fn)
	}
}

// The behavioural half: the loop is gone, so nothing fires. A settle that
// would have refilled a freed seat instead leaves the queue alone — the
// stopCtx is what carries "the watch process is stopping" into a Run that
// has already started, and it is the only thing that does.
//
// The control arm is the same fixture with a LIVE context: same two beads,
// same seat, same settle. Without it a Run that launched nothing for an
// unrelated reason would read as the throttle working.
func TestQAOneThrottleADeadWatchLoopRefillsNothing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		dead  bool
		want2 bool
	}{
		{"live loop refills the freed seat", false, true},
		{"killed loop refills nothing", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			writePersona(t, b.App, "ranger", "[go]")
			repo := qaRepo(t, b.App,
				`[{"id":"a-1","title":"t","labels":["go"]}]`,
				`[{"id":"a-1","status":"closed"},{"id":"a-2","status":"closed"}]`)
			write(t, filepath.Join(repo, "fake-ready-next.json"), `[{"id":"a-2","title":"u","labels":["go"]}]`)
			agentPerLaunch(t, fake)
			d.Refill = true
			d.PromptGrace = 0
			ctx, cancel := context.WithCancel(context.Background())
			d.stopCtx = ctx
			if tc.dead {
				cancel() // the watch loop is gone; this Run is its tail
			} else {
				defer cancel()
			}

			if _, err := d.Run("", "", 0); err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			// The first bead is the fixture's witness: it fires either way,
			// because it is the Run's own fire pass and not a refill.
			if !strings.Contains(out, "· a-1            creating session") {
				t.Fatalf("fixture: the Run never fired at all, so the arm below measures nothing:\n%s", out)
			}
			got := strings.Contains(out, "· a-2            creating session")
			if got != tc.want2 {
				t.Errorf("a-2 launched = %v, want %v — ADR 0028 §4: killing the watch loop stops the shop inviting work:\n%s", got, tc.want2, out)
			}
		})
	}
}

// ─── arm 4: where the skills exclude is written, and for which path ──────────

// gitAt runs one git command in dir and fails the test if it errors.
func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

// skillsApp is an App with two skills installed and nothing else.
func skillsApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	if err := os.MkdirAll(a.SkillsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	mkSkill(t, a.SkillsDir(), "dataviz")
	return a
}

// ADR 0007: the session dir is hidden from git through `.git/info/exclude` —
// the operator's `.gitignore` is never touched. The union rule and the
// idempotent write ARE pinned (TestRenderAgentsSkills), and that test names
// `.git/info/exclude` directly; what nothing measured is the two pieces of
// git arithmetic excludeFromGit exists to get right, and both of them decide
// which file is written and what goes in it:
//
//   - `--show-prefix`, so a session started in a SUBDIRECTORY excludes its
//     own path and not a namesake elsewhere in the tree. The header says
//     `filepath.Rel` against `--show-toplevel` was wrong here "wherever a
//     path component is a symlink (on macOS, every /var/… temp dir)" — which
//     is exactly what t.TempDir() hands this test, so the symlink case is
//     the case being run, not a case being described.
//   - `--git-common-dir`, so a LINKED WORKTREE writes to the main repo's
//     info/exclude. That is not a nicety: a worktree's own .git is a file,
//     `.git/info/exclude` under it is a path git never reads, and every
//     persona in this shop works from a linked worktree.
//
// MUTATIONS RUN (each reds one of the two tests here): `--show-prefix` → `--show-toplevel`
// with a filepath.Rel; drop the prefix from the pattern entirely; drop the
// leading "/" anchor; `--git-common-dir` → `--git-dir`.
func TestQASkillsExcludeIsAnchoredAtTheRepoRootAndSharedByWorktrees(t *testing.T) {
	t.Parallel()
	a := skillsApp(t)

	// ── a session started below the repo root ──
	repo := t.TempDir()
	gitAt(t, repo, "init", "-q")
	sub := filepath.Join(repo, "services", "api")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RenderAgentsSkills(sub, "developer", []string{"dataviz"}); err != nil {
		t.Fatal(err)
	}
	ex := filepath.Join(repo, ".git", "info", "exclude")
	body, err := os.ReadFile(ex)
	if err != nil {
		t.Fatalf("the exclude must be written to the repo's own info/exclude: %v", err)
	}
	const want = "/services/api/" + AgentsSkillsPath + "/"
	if !strings.Contains(string(body), "\n"+want+"\n") {
		t.Errorf("a session below the root must exclude ITS path, anchored at the root — want %q:\n%s", want, body)
	}
	// The pattern is the whole point, so measure it the way git does rather
	// than by reading the file: the dir is invisible from the root.
	if out := gitAt(t, repo, "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Errorf("the session dir must stay out of git status: %q", out)
	}
	// …and an unanchored or prefix-less pattern would also have swallowed a
	// namesake elsewhere in the tree, which is the failure the arithmetic
	// exists to prevent. This one is the operator's, and must still show.
	other := filepath.Join(repo, "vendor", AgentsSkillsPath)
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "keep.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := gitAt(t, repo, "status", "--porcelain"); !strings.Contains(out, "vendor/") {
		t.Errorf("only the session's own path is excluded; a namesake elsewhere is the operator's file: %q", out)
	}

	// ── a linked worktree ──
	// git only ever reads the COMMON dir's info/exclude, so a pattern written
	// under the worktree's own .git would hide nothing at all.
	if err := os.WriteFile(filepath.Join(repo, "seed"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, repo, "-c", "user.email=t@t", "-c", "user.name=t", "add", "seed")
	gitAt(t, repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "seed")
	wt := filepath.Join(t.TempDir(), "wt")
	gitAt(t, repo, "worktree", "add", "-q", "--detach", wt)
	before, _ := os.ReadFile(ex)
	if _, err := a.RenderAgentsSkills(wt, "developer", []string{"dataviz"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(ex)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) <= len(before) {
		t.Errorf("a linked worktree must write to the MAIN repo's info/exclude (--git-common-dir):\nbefore %d bytes, after %d", len(before), len(after))
	}
	if !strings.Contains(string(after), "\n/"+AgentsSkillsPath+"/\n") {
		t.Errorf("the worktree's session dir is at ITS root, so the pattern carries no prefix:\n%s", after)
	}
	if out := gitAt(t, wt, "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Errorf("the worktree's session dir must stay out of ITS git status: %q", out)
	}
	// The operator's file, still untouched on both.
	for _, d := range []string{repo, wt} {
		if b, _ := os.ReadFile(filepath.Join(d, ".gitignore")); len(b) != 0 {
			t.Errorf("%s: the repo's own .gitignore must never be written", d)
		}
	}

	// ── not a repo at all ──
	// Best effort: two git calls fail, nothing is written, the binding still
	// happens. A refusal here would make `posse new` in a plain directory
	// fail over a hygiene step.
	plain := t.TempDir()
	if dir, err := a.RenderAgentsSkills(plain, "developer", []string{"dataviz"}); err != nil || dir == "" {
		t.Errorf("a dir that is not a repo has nothing to pollute and must still bind: %q %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(plain, "dataviz", "SKILL.md")); err == nil {
		t.Error("the links go under .agents/skills, not the dir itself")
	}
	if _, err := os.Stat(filepath.Join(plain, AgentsSkillsPath, "dataviz", "SKILL.md")); err != nil {
		t.Errorf("the skill must be bound even with no repo to exclude it from: %v", err)
	}
}

// ─── arm 5: the two pool brakes dispatch orders ──────────────────────────────

// beadLine is the one report line this pass printed for a bead, without the
// pass-level lines around it: an assertion about what one bead's own line
// does or does not say must not be satisfied by a pass-level line.
func beadLine(t *testing.T, out, id string) string {
	t.Helper()
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, " "+id+" ") || strings.HasSuffix(ln, " "+id) {
			return ln
		}
	}
	t.Fatalf("no report line for bead %s:\n%s", id, out)
	return ""
}

// The pair dispatch.go orders the record's way, grokPoolSkip before
// uncountedSkip, with the comment "the bead cap below is the stand-in for
// the ABSENCE of one". They cannot both fire: grokPoolSkip answers for
// `grok` alone, and `uncountedFor` returns nil for a priced runtime before
// it reads the key at all (ADR 0013 §5's law). So the ordering that IS the
// record's is the one no launch can observe.
//
// This is a live-defect pin in the ORDERS sense: green today because it
// asserts the hole, red the day grok stops being priced or the cap comes
// back — which is exactly when the ordering starts to matter.
//
// MUTATION RUN: `func (grokCost) Prices() bool { return false }` → red.
func TestQATheOrderedBrakePairCannotBothFire(t *testing.T) {
	// The meter armed and OVER threshold: 80% of the pool against 70%. That
	// is the positive control the second half rests on — an unarmed meter
	// returns "" for every runtime, and would let "answers for grok alone"
	// pass while measuring nothing.
	f := trippedPass(t, "grok_guard_week: 70\n"+grokPoolCfg+"uncounted_cap_"+GrokPoolRuntime+": 1\n",
		parkPID, `["go","tier:standard"]`)
	d := f.d
	d.Now = func() time.Time { return grokPoolNow }
	home := grokPoolHome(t)
	at := grokPoolLastReset.Add(time.Hour)
	grokPoolSession(t, home, "s1",
		grokPoolUser(at, "Work beads issue ranger-base-esa0j (t)")+
			grokPoolTurn(at, "p-s1", usdTicks(40)))
	// The bead cap's own ledger, seeded PAST `uncounted_cap_grok: 1`. If the
	// priced-runtime law ever stops short-circuiting, this is a cap with a
	// count over it, and the brake fires.
	if err := os.MkdirAll(f.b.App.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.b.App.UncountedLogPath(),
		[]byte(LedgerEntry{grokPoolNow.Add(-2 * time.Hour), GrokPoolRuntime, "old", "ranger"}.line()), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := f.b.App.UncountedCount(GrokPoolRuntime, grokPoolNow); err != nil || n != 1 {
		t.Fatalf("fixture: the uncounted ledger must hold one grok launch in the window, got %d %v", n, err)
	}

	rt, err := f.b.App.LoadRuntime(GrokPoolRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if !rt.CostPriced() {
		t.Fatalf("%s is no longer priced — uncounted_cap_%s is live again, and the two brakes dispatch.go orders can now both fire on one bead. Reinstate an ordering assertion here (ADR 0013 §5).", GrokPoolRuntime, GrokPoolRuntime)
	}
	// The mechanism: uncountedFor returns nil for a priced runtime BEFORE it
	// reads the key (ADR 0013 §5's law), so the pool never gets an account
	// state for the cap to be compared against.
	if p := d.uncountedFor(GrokPoolRuntime); p != nil {
		t.Errorf("a priced runtime gets no uncounted pool at all: %+v", p)
	}
	// …and the consequence, over a cap the ledger has already reached.
	if line, kind := d.uncountedSkip(GrokPoolRuntime); line != "" || kind != "" {
		t.Errorf("a priced runtime's uncounted cap cannot fire, cap reached or not: %q / %q", line, kind)
	}
	// The first brake, meanwhile, is live — and answers for grok alone, so no
	// other runtime can bring a reading to the pair either.
	if s := d.grokPoolSkip(GrokPoolRuntime); !strings.Contains(s, "grok_guard_week: 70%") {
		t.Fatalf("fixture: the pool meter is not armed and over threshold, so the arms below measure nothing: %q", s)
	}
	for _, other := range []string{"claude", "codex"} {
		if s := d.grokPoolSkip(other); s != "" {
			t.Errorf("the pool meter answers for %s alone; %s got %q", GrokPoolRuntime, other, s)
		}
	}
}

// spilled is the launch script's body for a session, or "" when the line
// fit and was typed instead. Which of the two happened is a length fact and
// no pin here is about the length, so both readings are concatenated and
// the assertions look at the text (ranger-base-rq83c).
func spilled(t *testing.T, a *App, session string) string {
	t.Helper()
	body, err := os.ReadFile(a.LaunchScript(session))
	if err != nil {
		return ""
	}
	return string(body)
}
