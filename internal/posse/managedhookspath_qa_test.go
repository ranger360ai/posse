//go:build posse_arm2

package posse

// QA pins for ranger-base-mhrta — ADR 0052 D1, "classify before touching".
//
// THE DEFECT. A managed box points every git on it at one absolute,
// root-owned hooks directory outside every repo. `posse gates install-hooks`
// there answered `open <dir>/pre-push: permission denied`: installHook
// swallows the read error on a missing slot and falls straight through to
// os.WriteFile, so the FIRST thing posse did in the employer's directory was
// try to write in it. Session create made the same call best-effort and the
// hook-wall sweep reported the employer's two slots as posse's own wall gone
// stale, prescribing `posse gates install-hooks` — the one write this ADR
// says not to attempt.
//
// WHAT EACH ARM IS FOR. The classification has three legs, and a pin that
// only ever shows the managed verdict cannot tell which leg (or none) is
// carrying it. So each leg gets a two-arm pin over ONE fixture, changing
// exactly the fact that leg reads:
//
//   - MODE is the write leg: the same directory, outside the repo, absolute,
//     0555 vs 0755. Managed, then not — and the 0755 arm must reach the write
//     it would have made, or the managed arm's "nothing was written" is a
//     claim about a fixture that could not write anyway.
//   - SPELLING is the absolute leg: the same locked directory named
//     `../managed-hooks` and named in full. Not managed, then managed.
//   - LOCATION is the outside-the-repo leg: a locked directory INSIDE the
//     repo, named absolutely — the git dir's own hooks, and one in the
//     worktree. Absolute and unwritable both hold, so only this leg can
//     answer; not managed, and installHook still fails there in errno's own
//     words, the behaviour an operator who chmod'ed their own repo has
//     always had. (git's default answer is RELATIVE, so a `.git/hooks`
//     locked without a `core.hooksPath` is decided by the spelling leg and
//     measures this one not at all — mutation-checked.)
//
// The three callers are pinned where they report rather than where they
// classify: the sweep records a skip carrying the line instead of two
// degraded slots, and a launch into a managed repo writes nothing and says
// so. `posse gates install-hooks` itself is pinned end to end, on the
// GIT_CONFIG_GLOBAL spelling the managed box uses, in
// cmd/posse/managedhooks_qa_test.go.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// mhpFixture is the managed box in miniature: a repo, and a hooks directory
// OUTSIDE it holding one employer hook. Neither is aimed at the other yet —
// each pin does that itself, because which spelling is used is the thing two
// of them measure.
func mhpFixture(t *testing.T) (repo, managed string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	repo = filepath.Join(root, "checkout")
	managed = filepath.Join(root, "managed-hooks")
	for _, d := range []string{repo, managed} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	// The employer's own hook. It is here so the fixture is the shape the
	// ADR describes rather than an empty directory, and so a write posse
	// makes shows up beside something rather than in an empty listing.
	if err := os.WriteFile(filepath.Join(managed, "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo, managed
}

// mhpLock takes the write bit off dir and puts it back before t.TempDir's own
// cleanup runs (LIFO), which cannot unlink through a read-only directory.
func mhpLock(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: a mode bit is not a wall for uid 0, so this fixture cannot be built")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	// Measured, not assumed: if this uid CAN write here the pin below is
	// asserting nothing, and the mode bits alone do not say so on every
	// filesystem.
	probe := filepath.Join(dir, ".mhp-fixture-probe")
	if err := os.WriteFile(probe, []byte("x"), 0o600); err == nil {
		os.Remove(probe)
		t.Skipf("%s is writable at mode 0555 — no managed path to classify here", dir)
	}
}

// mhpSnapshot is every byte of the directory posse must not touch: the names,
// sizes, modes and mtimes of what is in it, and the directory's OWN mtime —
// which is what a file created and removed again would move.
func mhpSnapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&b, ". %v %d\n", st.Mode(), st.ModTime().UnixNano())
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		fi, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s %d %v %d\n", e.Name(), fi.Size(), fi.Mode(), fi.ModTime().UnixNano())
	}
	return b.String()
}

// ─── leg 1: the write probe ──────────────────────────────────────────────────

// The managed arm. An absolute hooks directory outside the repo that this uid
// cannot create a file in is MANAGED: the verdict names the owner and the
// mode, both installers refuse with that verdict rather than with errno's
// account of it, and nothing in the directory moves.
func TestQAManagedHooksPathIsClassifiedAndNothingIsWritten(t *testing.T) {
	t.Parallel()
	repo, managed := mhpFixture(t)
	qaGit(t, repo, "config", "core.hooksPath", managed)
	mhpLock(t, managed)

	m, err := managedHooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Managed {
		t.Fatalf("managedHooksDir(%s) = %+v — an unwritable absolute dir outside the repo is the whole class", repo, m)
	}
	if m.Dir != managed {
		t.Errorf("verdict names %q; git dispatches from %q", m.Dir, managed)
	}
	if want := strconv.Itoa(os.Geteuid()); m.Owner != want {
		t.Errorf("owner %q, want this uid %q — the line has to say WHOSE directory it is", m.Owner, want)
	}
	if m.Mode != "0555" {
		t.Errorf("mode %q, want 0555", m.Mode)
	}
	want := "L3: managed hooks path " + AbbrevHome(managed) + " (owner " + m.Owner + ", mode 0555)" +
		" — posse's wall is not installed there; realized by session redirect (ADR 0052)"
	if m.line() != want {
		t.Errorf("report line is\n  %s\nwant\n  %s", m.line(), want)
	}

	before := mhpSnapshot(t, managed)
	a := &App{}
	_, pushErr := InstallPrePushHook(repo)
	_, _, _, guardErr := a.InstallCommitGuardHook(repo)
	for slot, err := range map[string]error{"pre-push": pushErr, "prepare-commit-msg": guardErr} {
		if err == nil {
			t.Fatalf("install of %s claimed to succeed in a directory posse cannot write", slot)
		}
		var mhe managedHooksError
		if !errors.As(err, &mhe) {
			t.Errorf("install of %s failed with %v — want the typed managed verdict, not the create's error", slot, err)
		}
		if strings.Contains(err.Error(), "permission denied") {
			t.Errorf("install of %s still reports errno: %v", slot, err)
		}
	}
	if after := mhpSnapshot(t, managed); after != before {
		t.Errorf("the managed directory moved:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	// Nor anywhere else: an install that "helpfully" wrote into .git/hooks
	// would be a wall git does not dispatch (ranger-base-flz7) dressed up as
	// a success.
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", slot)); err == nil {
			t.Errorf("%s was written into .git/hooks, which git does not dispatch from here", slot)
		}
	}
}

// The wrong arm, and the one that says the arm above measured something: the
// SAME directory, same place, write bit ON. Not managed, and both installs
// land their renders in it — so "nothing was written" up there is a fact
// about the classification and not about a fixture that could not be written
// to in the first place.
func TestQAWritableForeignHooksPathIsNotManaged(t *testing.T) {
	t.Parallel()
	repo, foreign := mhpFixture(t)
	qaGit(t, repo, "config", "core.hooksPath", foreign)

	m, err := managedHooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if m.Managed {
		t.Fatalf("a WRITABLE absolute hooks dir outside the repo was called managed: %+v", m)
	}
	a := &App{}
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatalf("install into a writable foreign hooks path: %v", err)
	}
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatalf("install into a writable foreign hooks path: %v", err)
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		if _, err := os.Stat(filepath.Join(foreign, slot)); err != nil {
			t.Errorf("%s was not written where git dispatches: %v", slot, err)
		}
	}
}

// ─── leg 2: the absolute spelling ────────────────────────────────────────────

// One locked directory, two spellings of it. A RELATIVE core.hooksPath is a
// path inside the operator's own tree as far as this classification goes —
// git resolves it against the worktree, and posse keeps every behaviour it
// has there — while the same directory named in full is managed. Two arms
// over one fixture, so the discriminator is the spelling and nothing else.
func TestQARelativeCoreHooksPathIsNotManagedButTheAbsoluteSpellingIs(t *testing.T) {
	t.Parallel()
	repo, managed := mhpFixture(t)
	qaGit(t, repo, "config", "core.hooksPath", "../managed-hooks")
	mhpLock(t, managed)

	rel, err := managedHooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Managed {
		t.Errorf("a relative core.hooksPath was classified managed: %+v", rel)
	}
	if got := resolvedPath(rel.Dir); got != resolvedPath(managed) {
		t.Fatalf("the relative arm resolved to %q, not the locked directory %q — the two arms are not the same fixture", got, managed)
	}

	qaGit(t, repo, "config", "core.hooksPath", managed)
	abs, err := managedHooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !abs.Managed {
		t.Fatalf("the absolute spelling of the same locked directory is not managed either: %+v — this pin measures nothing", abs)
	}
}

// ─── leg 3: outside the repo ─────────────────────────────────────────────────

// A repo whose OWN hooks directory the operator locked is not somebody else's
// wall. Both places inside the repo that git can be aimed at — the git dir's
// `.git/hooks` and a directory in the worktree — stay unmanaged even when
// they are absolute AND unwritable, so this leg is measured on its own and
// not by the relative-spelling leg above: an absolute `core.hooksPath` is
// what both arms carry, which is exactly the fact leg 2 no longer decides.
//
// installHook then fails there as it always has — in errno's words, with no
// ADR 0052 line — because the remedy is the operator's own chmod, not a
// session redirect.
func TestQAAnUnwritableHooksDirInsideTheRepoIsNotManaged(t *testing.T) {
	t.Parallel()
	repo, _ := mhpFixture(t)
	inRepo := filepath.Join(repo, "myhooks")
	if err := os.MkdirAll(inRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitDirHooks, err := hooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	mhpLock(t, gitDirHooks)
	mhpLock(t, inRepo)

	for _, hooks := range []string{gitDirHooks, inRepo} {
		if !filepath.IsAbs(hooks) {
			t.Fatalf("fixture bug: %s is not absolute, so this arm cannot reach the location leg", hooks)
		}
		qaGit(t, repo, "config", "core.hooksPath", hooks)
		m, err := managedHooksDir(repo)
		if err != nil {
			t.Fatal(err)
		}
		if m.Managed {
			t.Errorf("%s is inside the repo and was classified as a managed path: %+v", hooks, m)
		}
		_, err = InstallPrePushHook(repo)
		if err == nil {
			t.Fatalf("install into a locked %s claimed to succeed", hooks)
		}
		var mhe managedHooksError
		if errors.As(err, &mhe) {
			t.Errorf("install into a locked %s returned the managed verdict: %v", hooks, err)
		}
		if !errors.Is(err, os.ErrPermission) {
			t.Errorf("install into a locked %s returned %v, want the permission error it always returned", hooks, err)
		}
	}
}

// ─── caller: the hook-wall sweep ─────────────────────────────────────────────

// mhpSweepApp declares one repo in config and returns the app that sweeps it.
func mhpSweepApp(t *testing.T, repo, visibility string) *App {
	t.Helper()
	home := t.TempDir()
	a := NewAppAt(home)
	if err := os.WriteFile(a.ConfigPath, []byte("beads_visibility:\n  "+repo+": "+visibility+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return a
}

// A declared repo that dispatches from a managed path is a SKIP carrying the
// managed line, never two degraded slots: the employer's hooks are not
// posse's wall gone stale, and the remedy the degraded line prints —
// `posse gates install-hooks` — is the write ADR 0052 refuses to attempt
// there. The sweep must not turn into a standing instruction to try it.
func TestQAHookWallSweepSkipsAManagedRepoRatherThanCallingItDegraded(t *testing.T) {
	t.Parallel()
	repo, managed := mhpFixture(t)
	qaGit(t, repo, "config", "core.hooksPath", managed)
	mhpLock(t, managed)
	a := mhpSweepApp(t, repo, VisibilityPrivate)

	s := a.SweepHookWall()
	if s.Declared != 1 || s.Managed != 1 || s.Measured != 0 || s.Findings != 0 {
		t.Fatalf("sweep counts %+v — want 1 declared, 1 managed, 0 measured, 0 findings", s)
	}
	if len(s.Repos) != 1 {
		t.Fatalf("sweep reported %d repos", len(s.Repos))
	}
	r := s.Repos[0]
	if !r.Managed || len(r.Degraded) != 0 {
		t.Fatalf("managed repo reported as %+v — want a skip with no degraded slots", r)
	}
	if !strings.Contains(r.Skip, "managed hooks path") || !strings.Contains(r.Skip, "ADR 0052") {
		t.Errorf("skip reason is %q, want the managed line", r.Skip)
	}

	var b strings.Builder
	if a.ReportHookWall(&b, "pin") {
		t.Error("a managed repo was reported as a finding")
	}
	out := b.String()
	if !strings.Contains(out, r.Skip) {
		t.Errorf("the report does not carry the managed line:\n%s", out)
	}
	if strings.Contains(out, "install-hooks") {
		t.Errorf("the report still prescribes a write into the managed directory:\n%s", out)
	}
	if after := mhpSnapshot(t, managed); !strings.Contains(after, "pre-commit") || strings.Contains(after, "pre-push") {
		t.Errorf("the sweep left something in the managed directory:\n%s", after)
	}
}

// The wrong arm: the same repo aimed at a WRITABLE directory with no posse
// render in it is exactly what the sweep exists to find, and it still says
// so — with the prescription. Without this, the pin above would pass on a
// sweep that had simply gone quiet.
func TestQAHookWallSweepStillFindsAWritableForeignHooksPath(t *testing.T) {
	t.Parallel()
	repo, foreign := mhpFixture(t)
	qaGit(t, repo, "config", "core.hooksPath", foreign)
	a := mhpSweepApp(t, repo, VisibilityPrivate)

	s := a.SweepHookWall()
	if s.Managed != 0 || s.Measured != 1 || s.Findings != 1 {
		t.Fatalf("sweep counts %+v — want 0 managed, 1 measured, 1 finding", s)
	}
	var b strings.Builder
	if !a.ReportHookWall(&b, "pin") {
		t.Fatalf("the sweep found nothing in a repo with no wall at all:\n%s", b.String())
	}
	if out := b.String(); !strings.Contains(out, "install-hooks") {
		t.Errorf("the finding does not print the remedy:\n%s", out)
	}
}

// ─── caller: session create ──────────────────────────────────────────────────

// A launch into a managed repo installs NOTHING — not the two renders, not a
// chain — and says on the launch what it did instead. The launch itself is
// still refused here: the wall is realized by the session hooks dir (ADR 0052
// D2, ranger-base-6prtx) and the probe learns to read it (D3,
// ranger-base-6kmkn); until those land the parity check reads the employer's
// slots as foreign and degrades, which is the honest answer for a build that
// has D1 alone. What this pin holds is the part that is D1's: no write, and a
// launch that says so.
func TestQALaunchIntoAManagedRepoWritesNothingAndSaysSo(t *testing.T) {
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
	before := mhpSnapshot(t, managed)

	warn := warnBuf(t, b)
	b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}) //nolint:errcheck // a D1-only build still degrades on the probe; the writes are the claim
	got := warn.String()
	if !strings.Contains(got, "managed hooks path") || !strings.Contains(got, "ADR 0052") {
		t.Errorf("the launch did not say why it installed nothing:\n%s", got)
	}
	if strings.Contains(got, "WRONG before this launch") {
		t.Errorf("the launch reported pre-heal drift over hooks it never re-stamped:\n%s", got)
	}
	if after := mhpSnapshot(t, managed); after != before {
		t.Errorf("the launch wrote into the managed directory:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// The wrong arm: an ordinary repo still gets both renders installed at
// launch, and no managed line is printed. A launcher that had simply stopped
// installing would pass the pin above.
func TestQALaunchIntoAnOrdinaryRepoStillInstallsTheWall(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"}); err != nil {
		t.Fatalf("launch into an ordinary repo was refused: %v", err)
	}
	if strings.Contains(warn.String(), "managed hooks path") {
		t.Errorf("an ordinary repo was called managed:\n%s", warn.String())
	}
	if !CommitGuardHookInstalled(repo) {
		t.Error("the launch did not install the commit guard")
	}
}
