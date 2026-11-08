//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-lh9nj — ADR 0081 D1 and D3, "a repo's shared commit
// wall has one owning home; a launch from any other home takes its own wall
// instead of rewriting the shared one".
//
// WHAT WAS WRONG. A session launch ends, on an unmanaged hooks path, with
// `a.InstallCommitGuardHook(dir)` — where `dir`'s hooks dir is the repo's
// COMMON one by design, and `a` is the LAUNCHING process's home. The body is
// a function of that home's config (OpsPatternSet, DeriveGuardValueLiterals,
// DeriveCrewLiterals, BeadsVisibility), so any home on the box rewrote any
// repo's shared wall as a matter of course. MEASURED 2026-11-06
// (ranger-base-q0np6): same binary, same repo, only RHQ_HOME changed, 1897
// lines against 1137 — and two scratch-home launches took ~/src/posse's wall
// at 22:06 and 22:11, after which every reader from the box's own home said
// "ours but stale" and every install from either home put it back. Two
// writers on one file, different inputs, no ordering.
//
// WHAT HOLDS NOW. Ownership is config (D1: `beads_visibility:` is the hook
// wall's repo list, and BeadsVisibility already reports an unlisted repo as
// VisibilityUnmarkedSource). The stamp is the file (D2, ranger-base-b87ej).
// D3 is where they meet: the launch writes the shared slot unless the body
// there is another home's current render AND this home does not own the repo
// — and then it renders its own wall per session instead (ADR 0052 D2's dir,
// forwarding into the shared hooks dir), so the session is held by BOTH walls
// and never by fewer than the owner's.
//
// THE FOUR ARMS OF D3's RULE, each with its wrong arm, because every one of
// them is a case where the launch MUST still write and a predicate that
// over-reached would silently stop arming repos:
//
//	case 1  nothing (or a foreign hook) in the slot   → write
//	case 2  this home owns the repo                   → write (D6: two
//	                                                    owners is visible
//	                                                    misconfiguration,
//	                                                    not a refusal)
//	case 3  the stamp names this home                 → write
//	case 4  the body carries no stamp                 → write (the one-shot
//	                                                    migration of every
//	                                                    body written before
//	                                                    this ADR)
//
// Cases 1, 3 and 4 are the pin that keeps alternative (c) rejected: a
// non-owner that refused every shared write would lose L3 in every repo
// nobody listed — `~/code/...`, a `dirs:` entry, a repo created seconds ago.
// The stamp is what separates "nobody's" from "somebody else's".
//
// MUTATION-CHECKED 2026-11-07, four mutants, each red in a different arm:
//
//	the rule removed (sharedCommitWallWrite always writes)
//	  → the three "leaves the wall alone" arms red, at the BYTES; nothing else
//	    moves, which is what says the other arms are about the states a
//	    launch still writes rather than about the rule
//	ownership dropped (l3OtherHome refused even for the owner)
//	  → TestQAAnOwningHomeStillReStampsAWallAnotherHomeStamped reds; D6's
//	    case would go silent, and this is the mutant a predicate narrowed
//	    inside reStamped() would have been
//	the `.sample` skip removed (forwardableSlots)
//	  → the redirect-exec arm reds on the dispatcher set: `git init`'s
//	    template dir would get a dispatcher and a report line each
//	the pre-heal warning asked of the VERDICT again instead of the write
//	  → the non-owner arm reds on "WRONG before this launch", which is the
//	    launch claiming a re-stamp it did not do

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the fixture ─────────────────────────────────────────────────────────────

// cwoFixture gives a launchable backend and a repo its home either declares
// or does not. The undeclared case writes a `beads_visibility:` that names a
// DIFFERENT repo rather than no list at all: D1 is a question about one repo,
// and an empty config would pass an arm that read "does this home list
// anything".
func cwoFixture(t *testing.T, declared bool) (*HerdrBackend, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	b, _ := newTestBackend(t)
	a := b.App
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"), []byte("---\nname: ranger\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// wtRepo and not hwsRepo: seeded with one commit on `main`, which is
	// what `git worktree add` requires and `git init` alone does not give.
	// Every dispatched launch on this box is Worktree:true, so an arm that
	// could not cut a worktree would be measuring the one shape the fleet
	// never runs.
	repo := wtRepo(t)
	listed := repo
	if !declared {
		listed = hwsRepo(t, gitTempDir(t), "somebody-elses")
	}
	if err := os.WriteFile(a.ConfigPath, []byte("beads_visibility:\n  "+listed+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := a.OwnsCommitWall(repo); got != declared {
		t.Fatalf("fixture premise: OwnsCommitWall(%s) = %v, want %v", repo, got, declared)
	}
	return b, repo
}

// cwoForeignWall installs the commit guard from a SECOND home, so the slot
// holds that home's current render with that home's stamp. Returned with the
// home, because every assertion below is about naming it.
func cwoForeignWall(t *testing.T, repo string, launcher *App) (*App, string) {
	t.Helper()
	x := NewAppAt(t.TempDir())
	if samePath(x.Home, launcher.Home) {
		t.Fatalf("fixture premise: the foreign home and the launching home are one directory (%s)", x.Home)
	}
	slot, _, _, err := x.InstallCommitGuardHook(repo)
	if err != nil {
		t.Fatalf("install from the foreign home: %v", err)
	}
	// The premise the whole file rests on, measured rather than assumed: the
	// launching home reads this body as another home's render, by its stamp.
	if p := launcher.probeL3Hooks(repo, false); p.CommitGuardVerdict != l3OtherHome || p.CommitGuardHome != x.Home {
		t.Fatalf("fixture premise: verdict=%d home=%q, want l3OtherHome (%d) stamped %q — line: %q",
			p.CommitGuardVerdict, p.CommitGuardHome, l3OtherHome, x.Home, p.CommitGuardDegraded)
	}
	return x, slot
}

// ─── D1: ownership is this home's own beads_visibility: list ─────────────────

func TestQAOwnsCommitWallIsThisHomesBeadsVisibilityList(t *testing.T) {
	t.Parallel()
	// Both arms are asserted by cwoFixture's own premise check; this adds
	// the one shape a `beads_visibility:` key can never name.
	owner, repo := cwoFixture(t, true)

	// A LINKED WORKTREE of a declared repo is owned, because the file the
	// rule governs is the repo's common hooks dir and not the tree launched
	// into (hookRepo). Every dispatched launch on this box is that shape, so
	// an owner predicate keyed on the tree would read the whole fleet as
	// non-owning and redirect every seat (ranger-base-up22, one question
	// over).
	tree := filepath.Join(gitTempDir(t), "wt")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", tree, "-b", "wt1").CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v %s", err, out)
	}
	if LinkedGitDirs(tree) == nil {
		t.Fatalf("fixture premise: %s is not a linked worktree, so hookRepo answers it unchanged and this arm measures nothing", tree)
	}
	if !owner.App.OwnsCommitWall(tree) {
		t.Errorf("a worktree of a declared repo is not owned — hookRepo(%s) resolved to something `beads_visibility:` cannot name", tree)
	}

	// And the config is read per repo: the same home does not own a repo it
	// did not list, whatever else is on the list.
	other, unlisted := cwoFixture(t, false)
	if other.App.OwnsCommitWall(unlisted) {
		t.Errorf("an unlisted repo is owned — D1 is `beads_visibility:` and nothing else")
	}
}

// ─── D3: the write rule, asked of the pre-heal probe ─────────────────────────

// THE DONE WHEN, first half. A launch from a home that does not own the repo
// leaves the installed body's BYTES AND MTIME alone — the whole hooks dir,
// in fact — and takes the per-session redirect instead.
func TestQAALaunchFromANonOwningHomeLeavesTheWallsBytesAndMtimeAlone(t *testing.T) {
	t.Parallel()
	b, repo := cwoFixture(t, false)
	a := b.App
	x, slot := cwoForeignWall(t, repo, a)
	before, beforeDir := qaReadFile(t, slot), mhpSnapshot(t, filepath.Dir(slot))

	warn := warnBuf(t, b)
	p, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"})
	if err != nil {
		t.Fatalf("launch into another home's wall was refused rather than redirected: %v", err)
	}

	// The bytes, the mtime, and everything else in that directory. mtime
	// because a same-content rewrite is still a write: it is what two homes
	// with identical configs do to each other, and bytes alone cannot see it.
	if after := qaReadFile(t, slot); after != before {
		t.Errorf("the launch rewrote another home's wall:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if after := mhpSnapshot(t, filepath.Dir(slot)); after != beforeDir {
		t.Errorf("the launch wrote in the shared hooks dir:\nbefore:\n%s\nafter:\n%s", beforeDir, after)
	}

	// The redirect, on the record and in the env. `foreign-owner` and never
	// `redirect`: there is no employer's directory here, and a reader that
	// found the managed word over a repo with no managed path behind it
	// would go looking for one (ADR 0081, Consequences).
	if p.HooksMode != string(redirectForeignOwner) {
		t.Errorf("hooks_mode = %q, want %q", p.HooksMode, redirectForeignOwner)
	}
	if p.ManagedHooks != "" {
		t.Errorf("managed_hooks = %q, want nothing: the forward target is the repo's own hooks dir, and `managed_hooks` is the record's word for an employer's", p.ManagedHooks)
	}
	hooks := a.SessionHooksDir("s1")
	if got := qaEnvValueFor(p.Vars, "core.hooksPath"); got != hooks {
		t.Errorf("the session's git is aimed at %q, want the rendered dir %q", got, hooks)
	}

	// The render itself: this home's wall as a member, and a dispatcher that
	// runs it and then the owner's.
	member := filepath.Join(hooks, "posse-prepare-commit-msg")
	id, _ := a.commitGuardLiterals(repo)
	vis, _ := a.BeadsVisibility(hookRepo(repo))
	if !identityMatch(member, a.CommitGuardHook(vis, a.OpsPatternSet(), id...)) {
		t.Errorf("%s is not this home's own commit-guard render", member)
	}
	if !identityMatch(filepath.Join(hooks, "prepare-commit-msg"), redirectDispatcher("prepare-commit-msg", filepath.Dir(slot), true)) {
		t.Errorf("the dispatcher does not chain this home's member into %s", filepath.Dir(slot))
	}

	// D3's one line, and the two facts only it carries: whose wall it is,
	// and that this launch did not take it.
	got := warn.String()
	for _, want := range []string{AbbrevHome(x.Home), "not rewritten", "ADR 0081 D3", AbbrevHome(hooks)} {
		if !strings.Contains(got, want) {
			t.Errorf("the launch did not say %q:\n%s", want, got)
		}
	}
	// And it did not claim the re-stamp it used to do. The verdict is still
	// `reStamped` — that is a fact about the BODY — so the narrowing has to
	// be at the write, or the launch says it took a wall it left alone.
	if strings.Contains(got, "WRONG before this launch") {
		t.Errorf("the launch claimed it re-stamped a wall it did not write:\n%s", got)
	}
}

// THE DONE WHEN, in the shape the fleet actually launches: Worktree:true, so
// `dir` is a tree posse cuts for the session and the wall is the SHARED
// repo's. Both halves of the rule have to resolve through the common repo
// here — ownership by hookRepo, the forward target by hooksDir — and neither
// is spelled anywhere near the other, so this is the arm that reds if one of
// them starts answering about the session tree.
func TestQAAWorktreeLaunchFromANonOwningHomeLeavesTheSharedWallAlone(t *testing.T) {
	t.Parallel()
	b, repo := cwoFixture(t, false)
	a := b.App
	x, slot := cwoForeignWall(t, repo, a)
	before, beforeDir := qaReadFile(t, slot), mhpSnapshot(t, filepath.Dir(slot))

	warn := warnBuf(t, b)
	p, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger", Worktree: true})
	if err != nil {
		t.Fatalf("worktree launch into another home's wall was refused: %v", err)
	}
	if p.Dir == repo {
		t.Fatalf("fixture premise: the launch ran in the repo itself (%s), not a session tree — this arm measures nothing", p.Dir)
	}
	if after := qaReadFile(t, slot); after != before {
		t.Errorf("a worktree launch rewrote the shared wall:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if after := mhpSnapshot(t, filepath.Dir(slot)); after != beforeDir {
		t.Errorf("a worktree launch wrote in the shared hooks dir:\nbefore:\n%s\nafter:\n%s", beforeDir, after)
	}
	if p.HooksMode != string(redirectForeignOwner) {
		t.Errorf("hooks_mode = %q, want %q", p.HooksMode, redirectForeignOwner)
	}
	// The forward target is the COMMON hooks dir — the one the shared wall
	// is in — and not anything under the session tree. Asked in two steps,
	// because the two spellings of that directory are not one string: git
	// answers `--git-path hooks` RELATIVE in a main checkout (joined against
	// the dir asked, so `/var/...` here) and ABSOLUTE through
	// `--git-common-dir` in a linked worktree, where it comes back with the
	// symlinks resolved (`/private/var/...` on darwin). The render and the
	// probe both take it from the same call, so they agree; a test that
	// spelled it for itself would be pinning the spelling.
	shared, err := hooksDir(p.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(shared, filepath.Dir(slot)) {
		t.Errorf("the session tree's hooks dir is %s, want the shared repo's %s — a forward target under the session tree would hold the session behind nothing", shared, filepath.Dir(slot))
	}
	if !identityMatch(filepath.Join(a.SessionHooksDir("s1"), "prepare-commit-msg"),
		redirectDispatcher("prepare-commit-msg", shared, true)) {
		t.Errorf("the dispatcher does not forward into the shared hooks dir %s", shared)
	}
	if got := warn.String(); !strings.Contains(got, AbbrevHome(x.Home)) || !strings.Contains(got, "not rewritten") {
		t.Errorf("the worktree launch did not name whose wall it left alone:\n%s", got)
	}
}

// D3 case 2, and D6. The owning home still re-stamps a wall another home
// stamped, and still says so: two owners is misconfiguration made visible,
// never a refusal, and the warning is the only thing that says the wall
// moved.
func TestQAAnOwningHomeStillReStampsAWallAnotherHomeStamped(t *testing.T) {
	t.Parallel()
	b, repo := cwoFixture(t, true)
	a := b.App
	x, slot := cwoForeignWall(t, repo, a)
	before := qaReadFile(t, slot)

	warn := warnBuf(t, b)
	p, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"})
	if err != nil {
		t.Fatalf("an owner's launch was refused: %v", err)
	}
	if after := qaReadFile(t, slot); after == before {
		t.Error("the owning home did not re-stamp the wall it owns")
	}
	if post := a.probeL3Hooks(repo, false); post.CommitGuardVerdict != l3Held {
		t.Errorf("the owner's wall is not this home's render after the launch: verdict=%d %s", post.CommitGuardVerdict, post.CommitGuardDegraded)
	}
	if p.HooksMode != "" {
		t.Errorf("an owner's launch took a redirect (hooks_mode=%q) — D3 writes the slot it owns, and ADR 0081 rejected alternative (b) for exactly this reason", p.HooksMode)
	}
	got := warn.String()
	if !strings.Contains(got, "WRONG before this launch") {
		t.Errorf("the owning home re-stamped another home's wall in silence:\n%s", got)
	}
	if !strings.Contains(got, AbbrevHome(x.Home)) {
		t.Errorf("the warning does not name the home whose render it replaced:\n%s", got)
	}
}

// D3 cases 1, 3 and 4 — the three states a NON-OWNER still writes. This is
// alternative (c)'s wrong arm: refuse every shared write from an unmarked
// home and L3 is lost in every repo nobody listed, which is most repos on a
// developer's box.
func TestQAANonOwningHomeStillWritesEveryStateThatIsNotAnotherHomes(t *testing.T) {
	t.Parallel()
	b, repo := cwoFixture(t, false)
	a := b.App
	slot := hwsHook(t, repo, "prepare-commit-msg")
	id := testIdentity(t, repo)

	for _, c := range []struct {
		what  string
		plant func()
	}{
		// Case 1: a repo somebody ran `git init` in a moment ago.
		{"an empty slot", func() {
			if err := os.Remove(slot); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}},
		// Case 3: ours, stamped by us, and stale — the same home's own
		// drift in a repo it never listed.
		{"our own stamp, gone stale", func() {
			if err := WriteExecutable(slot, []byte(a.CommitGuardHook(VisibilityPrivate, a.OpsPatternSet(), id...)), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		// Case 4: the migration body. Every wall on the box was this on the
		// day ADR 0081 landed.
		{"no stamp at all", func() {
			if err := WriteExecutable(slot, []byte(qaNoHome.CommitGuardHook(VisibilityPublic, a.OpsPatternSet(), id...)), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		c.plant()
		warn := warnBuf(t, b)
		p, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"})
		if err != nil {
			t.Fatalf("%s: launch refused: %v", c.what, err)
		}
		if p.HooksMode != "" {
			t.Errorf("%s: the launch redirected (hooks_mode=%q) instead of writing a slot that is not another home's", c.what, p.HooksMode)
		}
		if post := a.probeL3Hooks(repo, false); post.CommitGuardVerdict != l3Held || !post.CommitGuard {
			t.Errorf("%s: the launch did not arm the wall: verdict=%d %s", c.what, post.CommitGuardVerdict, post.CommitGuardDegraded)
		}
		if got := warn.String(); strings.Contains(got, "belongs to home") {
			t.Errorf("%s: reported as another home's wall:\n%s", c.what, got)
		}
	}
}

// The predicate on its own, over the verdicts no fixture above can reach by
// planting a file: a dir that is not a repository, and a foreign hook (which
// installHook refuses anyway — the rule must not turn that refusal into a
// redirect, or a repo holding somebody else's prepare-commit-msg would stop
// being reported as the foreign hook it is).
func TestQASharedCommitWallWriteOverTheVerdictsNoPlantCanReach(t *testing.T) {
	t.Parallel()
	b, repo := cwoFixture(t, false)
	a := b.App
	for _, c := range []struct {
		what  string
		pre   l3HookProbe
		write bool
	}{
		{"not a repository", l3HookProbe{}, true},
		{"a foreign hook", l3HookProbe{Repo: true, CommitGuardVerdict: l3Foreign}, true},
		{"bd's shim with our gate moved aside", l3HookProbe{Repo: true, CommitGuardVerdict: l3Displaced}, true},
		{"our chain dispatcher with the member gone", l3HookProbe{Repo: true, CommitGuardVerdict: l3MemberGone}, true},
		{"our own current render", l3HookProbe{Repo: true, CommitGuardVerdict: l3Held}, true},
		{"ours by marker, stale or unstamped", l3HookProbe{Repo: true, CommitGuardVerdict: l3Stale}, true},
		{"another home's current render", l3HookProbe{Repo: true, CommitGuardVerdict: l3OtherHome, CommitGuardHome: "/opt/other-home"}, false},
	} {
		write, stamp := a.sharedCommitWallWrite(repo, c.pre)
		if write != c.write {
			t.Errorf("%s: write = %v, want %v", c.what, write, c.write)
		}
		if !write && stamp != c.pre.CommitGuardHome {
			t.Errorf("%s: the refusal named home %q, want the stamp %q", c.what, stamp, c.pre.CommitGuardHome)
		}
	}
	// The same table's one config-dependent row: ownership flips the only
	// verdict that is ever refused, and nothing else.
	owner, ownRepo := cwoFixture(t, true)
	pre := l3HookProbe{Repo: true, CommitGuardVerdict: l3OtherHome, CommitGuardHome: "/opt/other-home"}
	if write, _ := owner.App.sharedCommitWallWrite(ownRepo, pre); !write {
		t.Error("the owning home may not write its own repo's wall — D3 case 2")
	}
}

// ─── the thing the redirect is FOR: both walls run ───────────────────────────

// THE DONE WHEN, second half. A commit in the redirected session runs this
// home's wall and then the owner's — ours first, in its own process, with its
// exit final — so the session is held by both and never by fewer than the
// owner's alone. Measured with a real `git commit` through a real git, by
// absolute path (ADR 0052 M2's own form).
func TestQAAForeignOwnerRedirectRunsThisHomesWallAndThenTheOwners(t *testing.T) {
	t.Parallel()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	root := gitTempDir(t)
	repo, home, log := filepath.Join(root, "checkout"), filepath.Join(root, "home"), filepath.Join(root, "canary.log")
	for _, d := range []string{repo, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	env := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		// The box's own git configuration is not part of this measurement.
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull}
	git := func(extra []string, args ...string) (string, error) {
		cmd := exec.Command(gitBin, append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), env...), extra...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := git(nil, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(nil, "add", "f"); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}

	// The OWNER's wall, standing in for another home's render: what it does
	// here is say that it ran. Whose bytes they are is D2's question and is
	// pinned where the stamp is read; what this arm measures is that the
	// redirect still reaches them.
	shared, err := hooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(filepath.Join(shared, "prepare-commit-msg"),
		[]byte("#!/bin/sh\nprintf 'owner\\n' >> "+log+"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := a.RenderSessionHooks("s1", repo, l3Forward{Dir: shared, Reason: redirectForeignOwner}, false)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if r.Reason != redirectForeignOwner || r.Managed != shared {
		t.Fatalf("render reason=%q forward=%q, want %q into %s", r.Reason, r.Managed, redirectForeignOwner, shared)
	}
	// `git init` fills a template dir with `.sample` files, which is why
	// they are counted out rather than given a dispatcher each: git's hook
	// lookup is find_hook(<the exact slot name>), so no slot it dispatches
	// ends in `.sample`.
	if !equalStrings(r.Slots, []string{"prepare-commit-msg"}) {
		t.Errorf("dispatchers rendered for %v, want the one slot git dispatches here", r.Slots)
	}
	if ents, err := os.ReadDir(shared); err == nil {
		samples := 0
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".sample") {
				samples++
			}
		}
		if samples > 0 && !strings.Contains(strings.Join(r.Skipped, " "), ".sample") {
			t.Errorf("%d samples stepped over and not one word said about it: %v", samples, r.Skipped)
		}
	}

	// A canary in place of this home's real member: the render's own bytes
	// are pinned by identity above, and a real wall would refuse this commit
	// rather than report its order.
	if err := WriteExecutable(filepath.Join(r.Dir, "posse-prepare-commit-msg"),
		[]byte("#!/bin/sh\nprintf 'ours\\n' >> "+log+"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var redirect []string
	for _, v := range gitConfigHooksPathVars(nil, r.Dir) {
		redirect = append(redirect, v.Key+"="+v.Value)
	}
	if out, err := git(redirect, "commit", "-qm", "seed", "--", "f"); err != nil {
		t.Fatalf("commit under the redirect: %v %s", err, out)
	}
	if got := qaCanaries(t, log); !equalStrings(got, []string{"ours", "owner"}) {
		t.Errorf("the redirected commit ran %v, want this home's wall and then the owner's", got)
	}

	// The wrong arm, and ADR 0052 M3's own claim one reason over: without
	// the redirect vars the owner's wall runs ALONE. The owner never gets
	// fewer hooks than before this launch existed — `env -i` included.
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "g"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(nil, "add", "g"); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	if out, err := git(nil, "commit", "-qm", "second", "--", "g"); err != nil {
		t.Fatalf("commit without the redirect: %v %s", err, out)
	}
	if got := qaCanaries(t, log); !equalStrings(got, []string{"owner"}) {
		t.Errorf("without the redirect the commit ran %v, want the owner's wall alone", got)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func qaReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func qaCanaries(t *testing.T, log string) []string {
	t.Helper()
	b, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(strings.TrimSpace(string(b)))
}

// qaEnvValueFor digs the value of one git config-in-env KEY out of a
// launch's rendered vars, by its indexed pair rather than by position: the
// redirect APPENDS at the next free index (gitConfigHooksPathVars), so the
// index is not knowable from here.
func qaEnvValueFor(vars []EnvVar, key string) string {
	idx := ""
	for _, v := range vars {
		if strings.HasPrefix(v.Key, "GIT_CONFIG_KEY_") && v.Value == key {
			idx = strings.TrimPrefix(v.Key, "GIT_CONFIG_KEY_")
		}
	}
	if idx == "" {
		return ""
	}
	for _, v := range vars {
		if v.Key == "GIT_CONFIG_VALUE_"+idx {
			return v.Value
		}
	}
	return ""
}

// ─── the container tier: no redirect, and no write either ────────────────────

// At the container tier D2's exclusion applies and there is no redirect: the
// env would cross the boundary and the dir it names is not on the mount list,
// so a core.hooksPath pointing at it would skip every hook in the repo — the
// owner's included, which is the one thing ADR 0052 promises never to do.
//
// So the session runs under the owner's wall ALONE, and the launch says so.
// What it does NOT do is take the wall: the bytes are left exactly as the
// owning home wrote them, and the parity check then reports — on the state
// the repo is honestly in — that this launch's own commit guard is not what
// the slot holds. That refusal is the behaviour change this bead ships at
// that tier, and it is the one the ADR's Consequences call for: an ad-hoc
// home pointed at a real repo can no longer change that repo's wall for
// anyone else, and a scratch home needs a scratch repo.
func TestQAAtTheContainerTierTheWallIsNeitherRewrittenNorRedirected(t *testing.T) {
	t.Parallel()
	b, repo := cwoFixture(t, false)
	a := b.App
	x, slot := cwoForeignWall(t, repo, a)
	before, beforeDir := qaReadFile(t, slot), mhpSnapshot(t, filepath.Dir(slot))

	// A container tier this host really can provide, without touching this
	// process's PATH: an engine whose `command:` is an absolute path to a
	// file that exists (engineOnPath resolves a path with a separator in it
	// directly — cage.go, resolveOutside).
	if err := os.MkdirAll(a.CagesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"), []byte("command: "+fakeBinFor(t, "fake-engine")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := qaReadFile(t, a.ConfigPath)
	if err := os.WriteFile(a.ConfigPath, []byte("default_engine: fake\n"+cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"), []byte("---\nname: ranger\ncage: container\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !a.ContainerAvailable() {
		t.Skip("fixture premise: this build does not read the engine fixture as available")
	}

	warn := warnBuf(t, b)
	_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"})
	if err == nil {
		t.Fatal("a caged launch under another home's wall was not reported as degraded — the session's gates are that home's, not this one's")
	}
	if !strings.Contains(err.Error(), "rendered by home") || !strings.Contains(err.Error(), AbbrevHome(x.Home)) {
		t.Errorf("the refusal does not name whose wall is in the slot:\n%v", err)
	}
	got := warn.String()
	for _, want := range []string{AbbrevHome(x.Home), "not rewritten", "the session runs under it", "ADR 0081 D3"} {
		if !strings.Contains(got, want) {
			t.Errorf("the launch did not say %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "L3 rendered at") {
		t.Errorf("a redirect was rendered at the container tier:\n%s", got)
	}
	if after := qaReadFile(t, slot); after != before {
		t.Errorf("the caged launch rewrote another home's wall:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if after := mhpSnapshot(t, filepath.Dir(slot)); after != beforeDir {
		t.Errorf("the caged launch wrote in the shared hooks dir:\nbefore:\n%s\nafter:\n%s", beforeDir, after)
	}
}
