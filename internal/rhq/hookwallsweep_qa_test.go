package rhq

// QA pins for ranger-base-ixv4 — the hook wall swept across every repo config
// declares, from two places that fire without anyone deciding to check.
//
// THE DEFECT. The L3 hook bodies are compiled into the binary, so every hook
// on a box is a COPY that was correct when it was written. Only a session
// create (which refreshes the COMMON hooks dir of the repo it was cut from,
// and no other) and a typed `posse gates install-hooks` re-render one. So the
// wall exists where sessions launch and goes stale everywhere else — and
// ~/src/ranger-base, which HOLDS the constitution, holds no session. Its
// prepare-commit-msg waved a promoted-class commit through hours after
// ranger-base-ak3e shipped the arm that refuses exactly that. probeL3Hooks
// already asks the right question (ADR 0023: identity at the dispatch path,
// behavior of our own render); it was only ever asked about one repo.
//
// WHAT EACH ARM IS FOR, and why a green one measures something:
//   - FRESH is the wrong-arm control. A sweep that finds a finding everywhere
//     is not a detector; the fixture installs from the renderer under test and
//     must come back clean.
//   - STALE is the regression that shipped: a body that still carries our
//     marker, is still +x and still refuses. A presence check passes it.
//   - STAMP is the half identity would hide if identity were normalized: a
//     PRIVATE repo carrying a PUBLIC-stamped hook leaks ops-class beads. Here
//     the render is built per repo from that repo's own configured visibility,
//     so identity subsumes it — pinned by name so it stays subsumed.
//   - ABSENT-FILE vs FOREIGN: the common real case is a repo the operator
//     declared and never installed into. Naming a foreign hook that is not
//     there sends the reader looking for a file to inspect.
//   - NOTHING MEASURED is the pass earned by looking at nothing.
//   - The two WIRING pins are the whole bead: a control nothing calls is the
//     state this bead was filed about.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// hwsRepo makes a git repo with one commit and returns its path.
func hwsRepo(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "pin@example.invalid"},
		{"config", "user.name", "pin"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// hwsFixture: a home whose config declares `repos`, each a real git repo with
// posse's two hooks installed from THIS build's renderer. Returns the app and
// the repo paths in the order named.
//
// The reference is the renderer under test, never a checked-in string: a pin
// that compares the sweep against a frozen body would go green the day the
// body changed and the sweep stopped agreeing with it.
func hwsFixture(t *testing.T, vis map[string]string, order ...string) (*App, map[string]string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHQ_HOME", home)
	t.Setenv(EnvPersona, "")

	dirs := map[string]string{}
	var cfg strings.Builder
	cfg.WriteString("beads_visibility:\n")
	for _, name := range order {
		d := hwsRepo(t, root, name)
		dirs[name] = d
		cfg.WriteString("  " + d + ": " + vis[name] + "\n")
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewAppAt(home)
	for _, name := range order {
		if _, _, _, err := a.InstallCommitGuardHook(dirs[name]); err != nil {
			t.Fatalf("install commit guard in %s: %v", name, err)
		}
		if _, err := InstallPrePushHook(dirs[name]); err != nil {
			t.Fatalf("install pre-push in %s: %v", name, err)
		}
	}
	return a, dirs
}

func hwsHook(t *testing.T, repo, slot string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		t.Fatalf("rev-parse hooks in %s: %v", repo, err)
	}
	h := strings.TrimSpace(string(out))
	if !filepath.IsAbs(h) {
		h = filepath.Join(repo, h)
	}
	return filepath.Join(h, slot)
}

func hwsReport(t *testing.T, a *App, where string) (string, bool) {
	t.Helper()
	var b bytes.Buffer
	found := a.ReportHookWall(&b, where)
	return b.String(), found
}

// A repo carrying exactly what this build renders is FRESH — the arm that
// makes every finding below mean something. Without it a sweep that always
// finds a finding would pass every other pin here.
func TestHookWallSweepPassesRepoCarryingThisBuildsRender(t *testing.T) {
	a, _ := hwsFixture(t, map[string]string{"pub": VisibilityPublic, "priv": VisibilityPrivate}, "pub", "priv")
	s := a.SweepHookWall()
	if s.Declared != 2 || s.Measured != 2 {
		t.Fatalf("declared=%d measured=%d, want 2/2", s.Declared, s.Measured)
	}
	if s.Findings != 0 {
		t.Fatalf("a freshly installed wall reported %d finding(s):\n%v", s.Findings, s.Repos)
	}
	out, found := hwsReport(t, a, "pin")
	if found {
		t.Fatalf("ReportHookWall reported a finding over a fresh wall:\n%s", out)
	}
	if !strings.Contains(out, "2 repo(s) carry this binary's render") {
		t.Fatalf("clean line missing — a check whose passing is invisible is a check nobody knows ran:\n%s", out)
	}
}

// The regression that shipped: still ours, still +x, still refuses — and three
// days behind the binary. Only byte identity sees it.
func TestHookWallSweepCatchesAStaleBodyThatStillRefuses(t *testing.T) {
	a, dirs := hwsFixture(t, map[string]string{"pub": VisibilityPublic, "priv": VisibilityPrivate}, "pub", "priv")
	p := hwsHook(t, dirs["priv"], "prepare-commit-msg")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Delete one sentence of guidance, exactly the shape of the real
	// staleness: the marker survives, the refusal survives, the advice rots.
	stale := strings.Replace(string(body), "git diff HEAD -- <paths>", "git diff", 1)
	if stale == string(body) {
		t.Skip("render no longer carries the erba sentence this fixture ages")
	}
	if err := os.WriteFile(p, []byte(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if !ownsHook(stale, sharedIndexMarker, legacySharedIndexMarker) {
		t.Fatal("fixture no longer carries our marker — it would be caught as foreign, not as stale")
	}

	out, found := hwsReport(t, a, "pin")
	if !found {
		t.Fatalf("a stale body that still refuses was waved through:\n%s", out)
	}
	if !strings.Contains(out, dirs["priv"]) || !strings.Contains(out, "ours but stale") {
		t.Fatalf("finding does not name the repo and why:\n%s", out)
	}
	if strings.Contains(out, dirs["pub"]) {
		t.Fatalf("the untouched repo was reported too — the sweep is not discriminating:\n%s", out)
	}
}

// pre-push carries no visibility stamp, so nothing about it varies per repo:
// its own arm, pinned separately, because a sweep that only ever looked at
// prepare-commit-msg would pass every pin above.
func TestHookWallSweepCatchesAStalePrePush(t *testing.T) {
	a, dirs := hwsFixture(t, map[string]string{"priv": VisibilityPrivate}, "priv")
	p := hwsHook(t, dirs["priv"], "pre-push")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(body, []byte("\n# aged\n")...), 0o755); err != nil {
		t.Fatal(err)
	}
	out, found := hwsReport(t, a, "pin")
	if !found || !strings.Contains(out, "L3 pre-push hook") || !strings.Contains(out, "ours but stale") {
		t.Fatalf("a stale pre-push was waved through:\n%s", out)
	}
}

// The half identity would hide if identity were normalized over the stamp: a
// private repo carrying a public-stamped hook is the one that leaks.
func TestHookWallSweepCatchesAStampThatDisagreesWithConfig(t *testing.T) {
	a, dirs := hwsFixture(t, map[string]string{"priv": VisibilityPrivate}, "priv")
	p := hwsHook(t, dirs["priv"], "prepare-commit-msg")
	// Plant the PUBLIC render — a whole, current, marker-bearing hook that
	// is stale in exactly one line, the one that decides the exemption.
	if err := os.WriteFile(p, []byte(CommitGuardHook(VisibilityPublic, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.BeadsVisibility(dirs["priv"]); v != VisibilityPrivate {
		t.Fatalf("fixture config did not take: visibility is %q", v)
	}
	out, found := hwsReport(t, a, "pin")
	if !found {
		t.Fatalf("a private repo carrying the public render passed the sweep:\n%s", out)
	}

	// The other direction, and the one that matters more
	// (ranger-base-qxwd): config-public / stamp-private is SILENT and fails
	// toward disclosure — the ops-class guard is simply off in a repo the
	// operator has since declared public. Same compare, so it must be the
	// same finding; a check that only ever saw the loud direction would
	// pass everything above.
	cfg, err := os.ReadFile(a.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	flipped := strings.Replace(string(cfg), ": "+VisibilityPrivate, ": "+VisibilityPublic, 1)
	if flipped == string(cfg) {
		t.Fatal("fixture config carries no private entry to flip")
	}
	if err := os.WriteFile(a.ConfigPath, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.BeadsVisibility(dirs["priv"]); v != VisibilityPublic {
		t.Fatalf("fixture flip did not take: visibility is %q", v)
	}
	if err := os.WriteFile(p, []byte(CommitGuardHook(VisibilityPrivate, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, found := hwsReport(t, a, "pin"); !found {
		t.Fatalf("a private-stamped hook in a repo config now calls public passed the sweep — this is the direction that fails toward disclosure:\n%s", out)
	}
}

// A declared repo nobody ever installed into: the common real case. The line
// must say so rather than name a foreign hook that is not there.
func TestHookWallSweepNamesAnUninstalledSlotAsUninstalled(t *testing.T) {
	a, dirs := hwsFixture(t, map[string]string{"priv": VisibilityPrivate}, "priv")
	if err := os.Remove(hwsHook(t, dirs["priv"], "prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	out, found := hwsReport(t, a, "pin")
	if !found {
		t.Fatalf("a repo with no commit wall at all passed the sweep:\n%s", out)
	}
	if !strings.Contains(out, "no hook installed at all") {
		t.Fatalf("an absent hook was described as something else:\n%s", out)
	}
	if strings.Contains(out, "foreign hook") {
		t.Fatalf("an absent hook was reported as foreign — the reader goes looking for a file that is not there:\n%s", out)
	}
}

// A hook posse did not write is still a finding, and keeps its own wording:
// the remedy differs (install-hooks refuses to overwrite it and prints the
// chain to paste instead).
func TestHookWallSweepReportsAForeignHookAsForeign(t *testing.T) {
	a, dirs := hwsFixture(t, map[string]string{"priv": VisibilityPrivate}, "priv")
	p := hwsHook(t, dirs["priv"], "prepare-commit-msg")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, found := hwsReport(t, a, "pin")
	if !found || !strings.Contains(out, "foreign hook") {
		t.Fatalf("a foreign hook was not reported as foreign:\n%s", out)
	}
}

// Config outliving a checkout is ordinary. It is not evidence about a wall,
// and it must not be reported as one — but with EVERY declared repo gone the
// sweep has measured nothing, and "no findings" would be a pass earned by
// looking at nothing.
func TestHookWallSweepSeparatesAbsentReposFromFindings(t *testing.T) {
	a, dirs := hwsFixture(t, map[string]string{"priv": VisibilityPrivate, "gone": VisibilityPrivate}, "priv", "gone")
	if err := os.RemoveAll(dirs["gone"]); err != nil {
		t.Fatal(err)
	}
	s := a.SweepHookWall()
	if s.Declared != 2 || s.Measured != 1 || s.Findings != 0 {
		t.Fatalf("declared=%d measured=%d findings=%d, want 2/1/0 — an absent checkout is not a stale wall", s.Declared, s.Measured, s.Findings)
	}
	seen := map[string]string{}
	for _, r := range s.Repos {
		seen[r.Config] = r.Skip
	}
	// Config, not Dir: Dir is absResolve'd, and on darwin the temp root is
	// behind a symlink (/var → /private/var), so a comparison against the
	// raw fixture path matches nothing and the assertion is dead. Measured
	// exactly that way — a mutant renaming the skip reason killed no pin.
	if len(seen) != 2 {
		t.Fatalf("sweep reported %d repos, want both declared: %v", len(seen), seen)
	}
	if !strings.Contains(seen[dirs["gone"]], "absent") {
		t.Fatalf("the vanished repo was recorded as %q, not as absent", seen[dirs["gone"]])
	}
	if seen[dirs["priv"]] != "" {
		t.Fatalf("the present repo was skipped: %q", seen[dirs["priv"]])
	}
	out, found := hwsReport(t, a, "pin")
	if found || !strings.Contains(out, "1 repo(s) carry this binary's render") {
		t.Fatalf("absent repo turned into a finding:\n%s", out)
	}

	if err := os.RemoveAll(dirs["priv"]); err != nil {
		t.Fatal(err)
	}
	out, found = hwsReport(t, a, "pin")
	if found {
		t.Fatalf("nothing measured must not report a finding:\n%s", out)
	}
	if !strings.Contains(out, "nothing measured") {
		t.Fatalf("with no repo present the sweep claimed something:\n%s", out)
	}
}

// An instance with no beads_visibility: block has made no claim for this to
// check. A line about it in every promote and every watch loop is noise about
// nothing.
func TestHookWallSweepIsSilentWhenConfigDeclaresNoRepo(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("default_env: default\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewAppAt(home)
	out, found := hwsReport(t, a, "pin")
	if found || out != "" {
		t.Fatalf("sweep spoke over a config that declares no repo: %q", out)
	}
}

// ─── the wiring: a control nothing calls is the state this bead is about ─────

// promote's epilogue. It runs after the promote, beside ADR 0015 §7's dangling
// default_env tripwire and for the same reason: this is the operator's
// constitution touch point, and the constitution repo is the one whose wall
// goes stale first and is noticed last.
//
// Three promotes over one repo, walking it through uninstalled → fresh →
// stale. A pin that only asserted the heading would pass over a sweep wired to
// nothing; each verdict has to be the one the repo's state earns.
func TestPromoteEpilogueSweepsTheHookWall(t *testing.T) {
	a, src, git := promoteFixture(t)
	repo := hwsRepo(t, t.TempDir(), "declared")

	// The declaration travels IN the constitution, because promote copies
	// config.yaml over the home's — a beads_visibility: written straight
	// into the home would be replaced by the promote whose epilogue reads it.
	cfg := filepath.Join(src, "config.yaml")
	body, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, append(body, []byte("beads_visibility:\n  "+repo+": private\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git("add", "-A"); err != nil {
		t.Fatalf("git add: %s", out)
	}
	if out, err := git("commit", "-qm", "declare a guarded repo"); err != nil {
		t.Fatalf("git commit: %s", out)
	}

	run := func() string {
		t.Helper()
		var b bytes.Buffer
		if err := a.CmdPromote(&b, PromoteOpts{Source: src}); err != nil {
			t.Fatalf("promote: %v\n%s", err, b.String())
		}
		if !strings.Contains(b.String(), "hook wall (promote)") {
			t.Fatalf("promote's epilogue does not sweep the hook wall — the control fires nowhere:\n%s", b.String())
		}
		return b.String()
	}

	// 1. Declared and never installed into.
	if out := run(); !strings.Contains(out, repo) || !strings.Contains(out, "no hook installed at all") {
		t.Fatalf("epilogue did not name a declared repo with no wall in it:\n%s", out)
	}

	// 2. Installed from this build's renderer.
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	if out := run(); !strings.Contains(out, "1 repo(s) carry this binary's render") {
		t.Fatalf("epilogue did not clear a freshly installed wall — a sweep that always finds a finding is not a detector:\n%s", out)
	}

	// 3. Aged behind the binary.
	p := hwsHook(t, repo, "pre-push")
	hook, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(hook, []byte("\n# aged\n")...), 0o755); err != nil {
		t.Fatal(err)
	}
	if out := run(); !strings.Contains(out, repo) || !strings.Contains(out, "ours but stale") {
		t.Fatalf("epilogue printed a heading but not the stale repo:\n%s", out)
	}
}

// The watch loop's preamble — once for the life of the loop, beside
// LaunchCapLine. Once and not per pass because the answer can only change when
// the binary does, and a loop IS a binary: a per-pass sweep would re-spawn git
// and sh for every configured repo forever to re-derive an answer that cannot
// have moved.
func TestWatchPreambleSweepsTheHookWallOncePerLoop(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := hwsRepo(t, t.TempDir(), "declared")
	if err := os.WriteFile(b.App.ConfigPath, []byte("beads_visibility:\n  "+repo+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := b.App.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatal(err)
	}
	p := hwsHook(t, repo, "pre-push")
	hook, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(hook, []byte("\n# aged\n")...), 0o755); err != nil {
		t.Fatal(err)
	}

	const wantPasses = 3
	tap := newPassTap(wantPasses)
	d.Out = tap
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 20*time.Millisecond); done <- p }()
	select {
	case <-done:
	// ranger-base-fa55: this deadline is not bounding the loop (3 passes at
	// a 10ms base interval finish in well under a second) — it is bounding
	// the ONE real hook-wall sweep the loop preamble runs, which forks git
	// and sh against a real repo. That cost is ~24s alone and grows with
	// box load (31s observed at load ~35), so a 30s ceiling chosen for an
	// idle machine reds under load without the loop ever actually hanging.
	// 90s matches the margin launchlock_qa_test.go gives its own real
	// cross-process work.
	case <-time.After(90 * time.Second):
		t.Fatalf("watch never returned:\n%s", tap.String())
	}

	s := tap.String()
	if n := strings.Count(s, "hook wall (watch)"); n != 1 {
		t.Fatalf("hook wall swept %d time(s) across >=%d passes, want exactly 1 — the preamble is per loop, not per pass:\n%s", n, wantPasses, s)
	}
	if !strings.Contains(s, repo) || !strings.Contains(s, "ours but stale") {
		t.Fatalf("the watch preamble printed a heading but not the stale repo:\n%s", s)
	}
	if i, j := strings.Index(s, "hook wall (watch)"), strings.Index(s, passHeader); i > j {
		t.Fatalf("the sweep printed after the first pass, not as its preamble:\n%s", s)
	}
}
