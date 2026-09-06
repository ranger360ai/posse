//go:build posse_arm2

package posse

// QA pins for ranger-base-6prtx — ADR 0052 D2, "the realizer is a hooks dir
// posse owns, per session, aimed by the session env".
//
// WHAT IS BEING HELD. On a managed hooks path D1 made posse stop writing;
// this bead is what it does INSTEAD. The launcher renders
// <StateDir>/hooks/<session>/ — posse's members under `posse-<slot>`, one
// dispatcher per slot in {posse's slots} ∪ {every executable in the managed
// dir} — and puts git's own config-in-env form in the session's environment
// so every git in that session dispatches from there and execs the
// employer's hook afterwards.
//
// THE FOUR FACTS THE DESIGN RESTS ON (ADR 0052 M1–M4, measured on the host
// 2026-09-02 and re-measured here on whatever git runs the suite):
//
//	M1  the env form outranks a config file's core.hooksPath
//	M2  ours runs, then the managed hook, under a real `git commit`
//	M3  an environment without the redirect runs the managed hooks ALONE —
//	    the employer's control is never fewer hooks than today
//	M4  a slot the redirect dir lacks is SKIPPED, which is the whole reason
//	    the dispatcher set is a union and not posse's two
//
// HOW ORDER IS MEASURED. One append-only log, written by both halves of a
// dispatcher. The employer's hooks append their own name; the posse MEMBER
// is replaced, after the render, by a stub that appends its own — which is
// the only way a canary can say "ours ran first" without changing what the
// real render is. The real render is pinned separately, by identity (it is
// byte-for-byte CommitGuardHook's, which is what D3's probe will compare)
// and by a commit that actually passes through it.
//
// Every dispatcher pin carries its wrong arm: delete the dispatcher and the
// employer's hook stops running (M4), drop the redirect vars and only the
// employer's hooks run (M3), make our member refuse and the employer's hook
// is never reached and git refuses (ours first, exit final).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the fixture: a managed box in miniature ─────────────────────────────────

type hrFix struct {
	t       *testing.T
	a       *App
	repo    string
	managed string
	log     string
	hooks   string // the rendered session hooks dir, once rendered
	base    []string
	gitBin  string // git by ABSOLUTE path — ADR 0052 M2's own form
}

// hrFixture builds a repo whose dispatch path is an unwritable directory
// outside it, holding two employer hooks that append their own name to one
// log. core.hooksPath is written into the repo's own config rather than a
// global file, for one reason: the classification runs IN THIS PROCESS
// (managedHooksDir shells out to git with the test binary's environment) and
// a t.Setenv here would reach every parallel test in the package. It also
// makes the precedence claim stronger, not weaker — local is the
// highest-precedence config FILE, so an env form that outranks it outranks
// the global spelling the managed box actually uses (ADR 0052 M1).
func hrFixture(t *testing.T) *hrFix {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	f := &hrFix{
		gitBin:  gitBin,
		t:       t,
		repo:    filepath.Join(root, "checkout"),
		managed: filepath.Join(root, "managed-hooks"),
		log:     filepath.Join(root, "canary.log"),
	}
	home := filepath.Join(root, "home")
	for _, d := range []string{f.repo, f.managed, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.a = &App{Home: home, StateDir: filepath.Join(home, "state")}
	if out, err := exec.Command("git", "-C", f.repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	f.base = []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		// The box's own git configuration is not part of this measurement.
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"RHQ_HOME=" + home}
	f.write("f", "x")
	f.git(nil, "add", "f")
	if out, err := f.git(nil, "commit", "-qm", "init"); err != nil {
		t.Fatalf("init commit: %v %s", err, out)
	}
	// The employer's hooks, then the wall around them.
	for _, slot := range []string{"pre-commit", "prepare-commit-msg"} {
		f.managedHook(slot)
	}
	qaGit(t, f.repo, "config", "core.hooksPath", f.managed)
	mhpLock(t, f.managed)
	return f
}

// managedHook plants one employer hook that appends its own name to the log.
func (f *hrFix) managedHook(slot string) {
	f.t.Helper()
	body := fmt.Sprintf("#!/bin/sh\nprintf 'managed-%s\\n' >> %s\nexit 0\n", slot, f.log)
	if err := os.WriteFile(filepath.Join(f.managed, slot), []byte(body), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

// unlocked runs one fixture edit with the managed directory's write bit back
// on. The lock is the fixture's whole point, so it goes straight back —
// mhpLock's own cleanup then finds it exactly as it left it.
func (f *hrFix) unlocked(edit func()) {
	f.t.Helper()
	if err := os.Chmod(f.managed, 0o755); err != nil {
		f.t.Fatal(err)
	}
	edit()
	if err := os.Chmod(f.managed, 0o555); err != nil {
		f.t.Fatal(err)
	}
}

func (f *hrFix) write(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.repo, name), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// git runs the git BINARY BY ITS ABSOLUTE PATH, which is ADR 0052 M2's own
// form and the reason the redirect is config-borne rather than a PATH shim:
// a wall that only reached the `git` a session's PATH resolves would be
// walked past by anything spelling it out.
func (f *hrFix) git(env []string, args ...string) (string, error) {
	f.t.Helper()
	cmd := exec.Command(f.gitBin, append([]string{"-C", f.repo}, args...)...)
	cmd.Env = append(append([]string(nil), f.base...), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// render is what the launcher does: classify, render, and hand back the
// environment that aims git at the result.
func (f *hrFix) render(wantPrePush bool) (*hooksRedirect, []string) {
	f.t.Helper()
	m, err := managedHooksDir(f.repo)
	if err != nil {
		f.t.Fatal(err)
	}
	if !m.Managed {
		f.t.Fatalf("fixture is not a managed hooks path: %+v", m)
	}
	r, err := f.a.RenderSessionHooks("s1", f.repo, m, wantPrePush)
	if err != nil {
		f.t.Fatal(err)
	}
	f.hooks = r.Dir
	var env []string
	for _, v := range gitConfigHooksPathVars(nil, r.Dir) {
		env = append(env, v.Key+"="+v.Value)
	}
	return r, env
}

// stubMember replaces a rendered member with a canary of the given exit
// status, so the LOG can say whether ours ran and in what order. The real
// render's bytes are pinned by TestQARedirectMembersAreTheInstallRenders.
func (f *hrFix) stubMember(slot string, exit int) {
	f.t.Helper()
	body := fmt.Sprintf("#!/bin/sh\nprintf 'posse-%s\\n' >> %s\nexit %d\n", slot, f.log, exit)
	if err := os.WriteFile(filepath.Join(f.hooks, "posse-"+slot), []byte(body), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *hrFix) canaries() []string {
	f.t.Helper()
	b, err := os.ReadFile(f.log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return strings.Fields(strings.TrimSpace(string(b)))
}

// ─── M1: the env form outranks the config file ───────────────────────────────

// The premise everything below rests on, measured on the git that is running
// this suite rather than quoted from the ADR: with core.hooksPath set in the
// repo's own config — the highest-precedence FILE — the config-in-env form
// still decides where git dispatches from. Both arms, so the pin cannot pass
// on a fixture that never named the managed dir in the first place.
func TestQARedirectEnvOutranksTheConfiguredHooksPath(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	_, env := f.render(false)

	out, err := f.git(nil, "rev-parse", "--git-path", "hooks")
	if err != nil {
		t.Fatalf("rev-parse: %v %s", err, out)
	}
	if got := strings.TrimSpace(out); resolvedPath(got) != resolvedPath(f.managed) {
		t.Fatalf("without the redirect git dispatches from %q, not the managed dir %q — the fixture is not the shape this measures", got, f.managed)
	}
	out, err = f.git(env, "rev-parse", "--git-path", "hooks")
	if err != nil {
		t.Fatalf("rev-parse under the redirect: %v %s", err, out)
	}
	if got := strings.TrimSpace(out); resolvedPath(got) != resolvedPath(f.hooks) {
		t.Fatalf("under the redirect git dispatches from %q, want the session dir %q", got, f.hooks)
	}
}

// ─── M2: ours first, then the employer's, under a real commit ────────────────

// The whole claim in one run: a commit made under the session env runs
// posse's member and THEN the managed prepare-commit-msg, in that order, with
// git's own argv. The managed pre-commit runs too — that is the union arm,
// M4's half, and it is a slot posse has no member for at all.
func TestQARedirectRunsOursThenTheManagedHookOnACommit(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(false)
	f.stubMember("prepare-commit-msg", 0)

	f.write("f", "y")
	if out, err := f.git(f.redirectEnv(), "commit", "-qm", "x", "--", "f"); err != nil {
		t.Fatalf("commit under the redirect: %v %s", err, out)
	}
	want := []string{"managed-pre-commit", "posse-prepare-commit-msg", "managed-prepare-commit-msg"}
	if got := f.canaries(); !equalStrings(got, want) {
		t.Fatalf("hooks ran %v, want %v", got, want)
	}
}

// The same commit with the REAL member in place, unstubbed: a path-limited
// commit passes posse's own wall and reaches the employer's hook. The pin
// above measures ORDER through a stub; this one measures that the wall being
// ordered is the one that actually ships — a redirect whose member refused
// every commit would be a session nobody could work in.
func TestQAARealCommitPassesTheRenderedWallAndReachesTheManagedHook(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	_, env := f.render(false)

	f.write("f", "y")
	if out, err := f.git(env, "commit", "-qm", "a note", "--", "f"); err != nil {
		t.Fatalf("a path-limited commit was refused under the rendered wall: %v %s", err, out)
	}
	want := []string{"managed-pre-commit", "managed-prepare-commit-msg"}
	if got := f.canaries(); !equalStrings(got, want) {
		t.Fatalf("hooks ran %v, want %v", got, want)
	}
}

// The M3 arm of the same fixture: the SAME commit with the redirect shed from
// the environment runs the employer's hooks alone. Posse's wall is
// env-borne and says so (ADR 0052 §4) — what it must never do is take the
// employer's hooks with it.
func TestQAWithoutTheRedirectEnvOnlyTheManagedHooksRun(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(false)
	f.stubMember("prepare-commit-msg", 0)

	f.write("f", "y")
	if out, err := f.git(nil, "commit", "-qm", "x", "--", "f"); err != nil {
		t.Fatalf("commit without the redirect: %v %s", err, out)
	}
	want := []string{"managed-pre-commit", "managed-prepare-commit-msg"}
	if got := f.canaries(); !equalStrings(got, want) {
		t.Fatalf("hooks ran %v, want the managed pair alone %v", got, want)
	}
}

// Ours first and its exit is FINAL: a member that refuses stops the commit
// and the employer's hook behind it is never reached. This is what makes the
// forward safe without fingerprinting what it forwards into — the managed
// hook can only ever refuse more than posse does, never less.
func TestQAAMemberThatRefusesStopsTheCommitBeforeTheManagedHook(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(false)
	f.stubMember("prepare-commit-msg", 1)

	f.write("f", "y")
	out, err := f.git(f.redirectEnv(), "commit", "-qm", "x", "--", "f")
	if err == nil {
		t.Fatalf("git committed anyway after the member refused:\n%s", out)
	}
	want := []string{"managed-pre-commit", "posse-prepare-commit-msg"}
	if got := f.canaries(); !equalStrings(got, want) {
		t.Fatalf("hooks ran %v, want ours refusing before the managed one %v", got, want)
	}
}

// ─── M4: the union, and what a missing dispatcher costs ──────────────────────

// The wrong arm for the union: delete the dispatcher for a slot posse has no
// member for, and the employer's hook stops running. Same fixture, same
// commit, one file removed — so "the managed pre-commit ran" above is a fact
// about the dispatcher and not about git finding the hook some other way.
// (This is also the state D3's forward-completeness arm degrades on.)
func TestQAADeletedDispatcherSkipsTheManagedHookEntirely(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	r, env := f.render(false)
	if !containsString(r.Slots, "pre-commit") {
		t.Fatalf("pre-commit was not forwarded at all: %v", r.Slots)
	}
	if err := os.Remove(filepath.Join(f.hooks, "pre-commit")); err != nil {
		t.Fatal(err)
	}
	f.stubMember("prepare-commit-msg", 0)

	f.write("f", "y")
	if out, err := f.git(env, "commit", "-qm", "x", "--", "f"); err != nil {
		t.Fatalf("commit under the redirect: %v %s", err, out)
	}
	if got := f.canaries(); containsString(got, "managed-pre-commit") {
		t.Fatalf("the managed pre-commit ran with no dispatcher for it: %v", got)
	}
}

// A managed hook that appears AFTER the render is not forwarded until the
// next launch — the bound ADR 0052 names in its consequences. Both arms, on
// one fixture: the new hook does not run under this session's redirect, and
// a re-render (what the next launch does) picks it up. Pinned because it is
// the honest reach of this design and not an accident: a reader who believes
// the forward is live would not re-launch.
func TestQAAManagedHookAddedAfterTheRenderIsNotForwardedUntilTheNextOne(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	r, env := f.render(false)
	if containsString(r.Slots, "post-commit") {
		t.Fatal("fixture already forwards post-commit")
	}
	f.unlocked(func() { f.managedHook("post-commit") })

	f.write("f", "y")
	if out, err := f.git(env, "commit", "-qm", "x", "--", "f"); err != nil {
		t.Fatalf("commit under the redirect: %v %s", err, out)
	}
	if got := f.canaries(); containsString(got, "managed-post-commit") {
		t.Fatalf("a hook added after the render was forwarded anyway: %v", got)
	}

	r2, env2 := f.render(false)
	if !containsString(r2.Slots, "post-commit") {
		t.Fatalf("the re-render did not pick the new hook up: %v", r2.Slots)
	}
	f.write("f", "z")
	if out, err := f.git(env2, "commit", "-qm", "x", "--", "f"); err != nil {
		t.Fatalf("commit after the re-render: %v %s", err, out)
	}
	if got := f.canaries(); !containsString(got, "managed-post-commit") {
		t.Fatalf("the re-rendered dispatcher did not forward: %v", got)
	}
}

// ─── the render itself ───────────────────────────────────────────────────────

// The members are the bytes install-hooks would have written — the same
// call, the same visibility mark and the same ADR 0024 D2 identity literals,
// resolved off hookRepo. D3's identity probe compares the file against
// exactly this render, so a divergence here is a launch that degrades on its
// own wall.
func TestQARedirectMembersAreTheInstallRenders(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	r, _ := f.render(true)

	visibility, _ := f.a.BeadsVisibility(hookRepo(f.repo))
	identity, err := DeriveIdentityLiterals(hookRepo(f.repo))
	if err != nil {
		t.Fatal(err)
	}
	for slot, want := range map[string]string{
		"prepare-commit-msg": CommitGuardHook(visibility, f.a.OpsPatternSet(), identity...),
		"pre-push":           PrePushHook,
	} {
		p := filepath.Join(r.Dir, "posse-"+slot)
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("member %s: %v", slot, err)
		}
		if string(got) != want {
			t.Errorf("%s is not the install render", p)
		}
		st, err := os.Stat(p)
		if err != nil || st.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable (%v)", p, err)
		}
	}
	// pre-push only when the PID denies git push, exactly as an ordinary
	// install decides it.
	r2, _ := f.render(false)
	if _, err := os.Stat(filepath.Join(r2.Dir, "posse-pre-push")); err == nil {
		t.Error("posse-pre-push was rendered for a PID that does not deny git push")
	}
}

// The dispatcher's shape, byte for byte: the chain form with the neighbour
// spelled as the managed dir's ABSOLUTE path (it is not a sibling of ours,
// so `$d/` would name the wrong directory), ours run first with its exit
// honoured, pre-push's member kept off the ref list on stdin — and, for a
// slot posse has no member for, the forward alone.
func TestQARedirectDispatcherShape(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.unlocked(func() { f.managedHook("pre-push") })
	r, _ := f.render(true)

	for _, slot := range []string{"prepare-commit-msg", "pre-push"} {
		b, err := os.ReadFile(filepath.Join(r.Dir, slot))
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		neighbour := filepath.Join(f.managed, slot)
		for _, want := range []string{
			`"$d/posse-` + slot + `"`,
			"|| exit $?",
			`[ -x "` + neighbour + `" ] || exit 0`,
			"exec \"" + neighbour + "\" \"$@\"\n",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s dispatcher lacks %q:\n%s", slot, want, body)
			}
		}
		if strings.Contains(body, `exec "$d/`) {
			t.Errorf("%s dispatcher forwards to a SIBLING, not the managed dir:\n%s", slot, body)
		}
		if got := strings.Contains(body, "</dev/null"); got != (slot == "pre-push") {
			t.Errorf("%s dispatcher stdin handling is wrong:\n%s", slot, body)
		}
	}
	// A slot with no member of ours: the forward alone, and nothing that
	// would exec a member that is not there.
	b, err := os.ReadFile(filepath.Join(r.Dir, "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if strings.Contains(body, "posse-pre-commit") {
		t.Errorf("the memberless dispatcher runs a member that was never rendered:\n%s", body)
	}
	want := fmt.Sprintf("#!/bin/sh\n[ -x %[1]q ] || exit 0\nexec %[1]q \"$@\"\n", filepath.Join(f.managed, "pre-commit"))
	if body != want {
		t.Errorf("memberless dispatcher is\n%s\nwant\n%s", body, want)
	}
}

// What is NOT forwarded is printed. A managed directory holds more than
// hooks — a README, a config dir, a helper nobody made executable — and a
// render that stepped over them silently would be a wall whose completeness
// nobody can read.
func TestQARenderNamesWhatItDidNotForward(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.unlocked(func() {
		if err := os.WriteFile(filepath.Join(f.managed, "README"), []byte("hi\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(f.managed, "helpers"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(f.managed, "nowhere"), filepath.Join(f.managed, "dangling")); err != nil {
			t.Fatal(err)
		}
		// An executable posse would overwrite with a member of its own.
		if err := os.WriteFile(filepath.Join(f.managed, "posse-prepare-commit-msg"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	r, _ := f.render(false)

	if containsString(r.Slots, "README") || containsString(r.Slots, "helpers") || containsString(r.Slots, "dangling") {
		t.Errorf("a non-hook entry was forwarded: %v", r.Slots)
	}
	skipped := strings.Join(r.Skipped, "\n")
	for _, want := range []string{"README", "helpers", "dangling", "posse-prepare-commit-msg"} {
		if !strings.Contains(skipped, want) {
			t.Errorf("%s is not named in the skip list:\n%s", want, skipped)
		}
	}
	// And the member it collided with is still ours.
	b, err := os.ReadFile(filepath.Join(r.Dir, "posse-prepare-commit-msg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), sharedIndexMarker) {
		t.Error("a managed entry overwrote posse's own member")
	}
}

// The render is fresh every launch, and it never writes in the employer's
// directory — the D1 promise, held again by the code that came to replace
// the install.
func TestQARenderIsFreshAndTouchesNothingManaged(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	before := mhpSnapshot(t, f.managed)
	r, _ := f.render(false)
	stale := filepath.Join(r.Dir, "stale-from-a-previous-launch")
	if err := os.WriteFile(stale, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := f.a.RenderSessionHooks("s1", f.repo, mustManaged(t, f.repo), false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a re-render left a file no render would have written")
	}
	if after := mhpSnapshot(t, f.managed); after != before {
		t.Errorf("the render wrote into the managed directory:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	// And the session's dir goes when the session does.
	f.a.RemoveSessionHooks("s1")
	if _, err := os.Stat(r.Dir); !os.IsNotExist(err) {
		t.Errorf("the session hooks dir survived the session (%v)", err)
	}
}

// A session name this may not build a path from renders nothing and removes
// nothing — the guard that keeps RemoveSessionHooks's RemoveAll off a path
// assembled out of an unvalidated name.
func TestQASessionHooksDirRefusesANameItCannotBuildAPathFrom(t *testing.T) {
	t.Parallel()
	a := &App{Home: t.TempDir(), StateDir: filepath.Join(t.TempDir(), "state")}
	for _, bad := range []string{"", ".", "..", "../../etc", "a/b"} {
		if got := a.SessionHooksDir(bad); got != "" {
			t.Errorf("SessionHooksDir(%q) = %q, want no path at all", bad, got)
		}
	}
	if got := a.SessionHooksDir("s1"); got != filepath.Join(a.StateDir, "hooks", "s1") {
		t.Errorf("SessionHooksDir(s1) = %q", got)
	}
}

// ─── the env form ────────────────────────────────────────────────────────────

// The redirect APPENDS. An operator whose environment already carries
// GIT_CONFIG_COUNT entries keeps them: posse's pair lands at the next free
// index, the count is raised by one, and BOTH settings apply — measured by
// running git under the combined environment, not by reading the strings
// back.
func TestQARedirectAppendsToAnExistingGitConfigCount(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	r, _ := f.render(false)

	existing := []EnvVar{
		{"GIT_CONFIG_COUNT", "1"},
		{"GIT_CONFIG_KEY_0", "user.name"},
		{"GIT_CONFIG_VALUE_0", "already-here"},
	}
	got := gitConfigHooksPathVars(existing, r.Dir)
	want := []EnvVar{
		{"GIT_CONFIG_COUNT", "2"},
		{"GIT_CONFIG_KEY_1", "core.hooksPath"},
		{"GIT_CONFIG_VALUE_1", r.Dir},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	var env []string
	for _, v := range append(existing, got...) {
		env = append(env, v.Key+"="+v.Value)
	}
	out, err := f.git(env, "config", "user.name")
	if err != nil || strings.TrimSpace(out) != "already-here" {
		t.Errorf("the operator's own entry did not survive: %q %v", out, err)
	}
	out, err = f.git(env, "rev-parse", "--git-path", "hooks")
	if err != nil || resolvedPath(strings.TrimSpace(out)) != resolvedPath(r.Dir) {
		t.Errorf("posse's entry did not apply: %q %v", out, err)
	}
}

// With nothing in the environment the pair is index 0 and the count is 1;
// a count that is not a positive number is treated as none, because git
// refuses to run at all on a bogus one and there is nothing there to keep.
func TestQARedirectEnvIndexesFromWhatIsAlreadyThere(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ count, wantCount, wantIdx string }{
		{"", "1", "0"},
		{"0", "1", "0"},
		{"3", "4", "3"},
		{"nonsense", "1", "0"},
		{"-2", "1", "0"},
	} {
		var vars []EnvVar
		if c.count != "" {
			vars = []EnvVar{{"GIT_CONFIG_COUNT", c.count}}
		}
		got := gitConfigHooksPathVars(vars, "/d")
		if got[0].Value != c.wantCount || got[1].Key != "GIT_CONFIG_KEY_"+c.wantIdx || got[2].Key != "GIT_CONFIG_VALUE_"+c.wantIdx {
			t.Errorf("count %q → %v, want count %s at index %s", c.count, got, c.wantCount, c.wantIdx)
		}
	}
}

// The count is taken from the launch vars ALONE, never from the launcher's
// own environment (ranger-base-buvq4, escaped from ranger-base-6prtx). The
// pane is the herdr daemon's child and the vars are the only channel from
// the launcher into it (docs/notes.d/ranger-base-ok1x.md), so a count read
// from os.Getenv names indices the session never receives: git there fails
// on EVERY command — `missing config key GIT_CONFIG_KEY_0`, rc 128 — the
// wall included. Measured by running git under what the session actually
// gets, the launcher carrying a count of its own; and the wrong arm of the
// same fact, a launcher count beside a vars count, indexes off the vars.
//
// Not parallel: t.Setenv is the fixture.
func TestQARedirectEnvIgnoresTheLaunchersOwnGitConfigCount(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_VALUE_0", "the-operator")

	hooks := t.TempDir()
	got := gitConfigHooksPathVars(nil, hooks)
	if got[0].Value != "1" || got[1].Key != "GIT_CONFIG_KEY_0" {
		t.Errorf("the launcher's own count leaked into the session's index: %v", got)
	}

	// What the session sees: no GIT_CONFIG_* of the launcher's, plus exactly
	// the vars posse handed CreateWorkspace.
	repo := t.TempDir()
	run := func(env []string, args ...string) (string, error) {
		cmd := exec.Command(gitBin, args...)
		cmd.Dir = repo
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	base := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null"}
	if out, err := run(base, "init", "-q", "."); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	session := append([]string{}, base...)
	for _, v := range got {
		session = append(session, v.Key+"="+v.Value)
	}
	out, err := run(session, "rev-parse", "--git-path", "hooks")
	if err != nil {
		t.Fatalf("git in the session is fatal, the wall included: %v\n%s", err, out)
	}
	if resolvedPath(out) != resolvedPath(hooks) {
		t.Errorf("the redirect did not apply: git dispatches from %q, the render is at %q", out, hooks)
	}

	// A count that IS in the vars — an env set of the operator's own — is
	// still what posse appends after, whatever the launcher carries.
	vars := []EnvVar{{"GIT_CONFIG_COUNT", "2"}}
	if got := gitConfigHooksPathVars(vars, hooks); got[0].Value != "3" || got[1].Key != "GIT_CONFIG_KEY_2" {
		t.Errorf("a vars count did not win over the launcher's: %v", got)
	}
}

// ─── the launch ──────────────────────────────────────────────────────────────

// The launcher's half: a launch into a managed repo renders the dir, says
// what it dispatched and into what, and hands the session the env that aims
// git at it.
func TestQALaunchIntoAManagedRepoRendersTheRedirectAndCarriesItInTheEnv(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	managed := filepath.Join(t.TempDir(), "managed-hooks")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managed, "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	qaGit(t, repo, "config", "core.hooksPath", managed)
	mhpLock(t, managed)

	warn := warnBuf(t, b)
	p, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger", AllowDegraded: true})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	hooks := b.App.SessionHooksDir("s1")
	for _, want := range []string{"posse-prepare-commit-msg", "prepare-commit-msg", "pre-commit"} {
		if _, err := os.Stat(filepath.Join(hooks, want)); err != nil {
			t.Errorf("the launch did not render %s: %v", want, err)
		}
	}
	if !strings.Contains(warn.String(), "L3 rendered at") {
		t.Errorf("the launch did not say what it rendered:\n%s", warn.String())
	}
	env := map[string]string{}
	for _, v := range p.Vars {
		env[v.Key] = v.Value
	}
	if env["GIT_CONFIG_COUNT"] == "" || env["GIT_CONFIG_KEY_0"] != "core.hooksPath" || env["GIT_CONFIG_VALUE_0"] != hooks {
		t.Errorf("the launch vars do not aim git at the rendered dir: %v", env)
	}
}

// The wrong arm: an ordinary repo gets no redirect dir and no redirect env —
// every existing pin over .git/hooks keeps its meaning (ADR 0052
// consequences).
func TestQALaunchIntoAnOrdinaryRepoRendersNoRedirect(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	p, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if _, err := os.Stat(b.App.SessionHooksDir("s1")); !os.IsNotExist(err) {
		t.Errorf("an ordinary repo got a session hooks dir (%v)", err)
	}
	for _, v := range p.Vars {
		if strings.HasPrefix(v.Key, "GIT_CONFIG_") {
			t.Errorf("an ordinary repo's launch carries %s", v.Key)
		}
	}
}

// A managed path that is not one line is refused at the launch, before a
// workspace, a render or a record exists (ranger-base-buvq4, escaped from
// ranger-base-6kmkn). git accepts such a core.hooksPath and the dispatcher
// would run it; what posse cannot do is RECORD it — the session meta is a
// flat file whose reader stops at the first newline (ranger-base-ujdg), so
// the path's tail would read back as meta fields of its own, `crew: true`
// among them, on a session that was never crew. The control arm is
// TestQALaunchIntoAManagedRepoRendersTheRedirectAndCarriesItInTheEnv: the
// same launch over a one-line managed path goes through.
func TestQALaunchRefusesAManagedHooksPathThatIsNotOneLine(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	managed := filepath.Join(t.TempDir(), "managed-hooks\ncrew: true")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managed, "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	qaGit(t, repo, "config", "core.hooksPath", managed)
	mhpLock(t, managed)
	// The condition is staged, not assumed: git round-trips the path and
	// posse classifies it managed.
	if m := mustManaged(t, repo); m.Dir != managed {
		t.Fatalf("git did not round-trip the path: %q, want %q", m.Dir, managed)
	}

	err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger", AllowDegraded: true})
	if err == nil || !strings.Contains(err.Error(), "not one line") {
		t.Fatalf("CreateSession = %v, want a refusal naming the path", err)
	}
	if _, err := os.Stat(b.metaPath("s1")); !os.IsNotExist(err) {
		t.Errorf("a refused launch wrote a record (%v)", err)
	}
	if _, err := os.Stat(b.App.SessionHooksDir("s1")); !os.IsNotExist(err) {
		t.Errorf("a refused launch rendered a hooks dir (%v)", err)
	}
	if got, ok := b.readMeta("s1"); ok && got.Crew {
		t.Errorf("the tail of the path became a meta field: crew: true on a session that was never crew")
	}
}

// The dir goes when the session does, on the path that actually retires
// sessions. A stale render of a wall no live session is behind is a
// directory whose freshness nothing re-establishes — and freshness-by-
// construction is the whole reason ADR 0052 renders per session.
func TestQAKillRemovesTheSessionHooksDir(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "s1"})
	hooks := b.App.SessionHooksDir("s1")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := b.KillSession("s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hooks); !os.IsNotExist(err) {
		t.Errorf("the kill left %s behind (%v)", hooks, err)
	}
	// The wrong arm: another session's dir is not this kill's to remove.
	other := b.App.SessionHooksDir("s2")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	mustCreate(t, b, NewSessionOpts{Name: "s3"})
	if err := b.KillSession("s3"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("a kill removed another session's hooks dir: %v", err)
	}
}

// The seatbelt half: the redirect dir is where L3 lives on a managed box, so
// a session must not be able to write it — the same rule, one directory
// over, that keeps a session out of `.git/hooks` and out of state/gates.
func TestQASeatbeltDeniesTheSessionHooksDir(t *testing.T) {
	t.Parallel()
	a := &App{Home: t.TempDir(), StateDir: filepath.Join(t.TempDir(), "state")}
	c := a.SeatbeltCarveOut(&AgentFile{Name: "ranger"}, t.TempDir(), "", nil)
	want := absResolve(filepath.Join(a.StateDir, "hooks"))
	if !containsString(c.Deny, want) {
		t.Errorf("the carve-out does not deny %s:\n%v", want, c.Deny)
	}
}

// ─── small shared helpers ────────────────────────────────────────────────────

func (f *hrFix) redirectEnv() []string {
	f.t.Helper()
	var env []string
	for _, v := range gitConfigHooksPathVars(nil, f.hooks) {
		env = append(env, v.Key+"="+v.Value)
	}
	return env
}

func mustManaged(t *testing.T, repo string) managedHooks {
	t.Helper()
	m, err := managedHooksDir(repo)
	if err != nil || !m.Managed {
		t.Fatalf("managedHooksDir(%s) = %+v, %v", repo, m, err)
	}
	return m
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
