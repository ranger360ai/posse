//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-yx73g (ADR 0082 D1+D2) — ONE repo renders ONE
// identity wall, however the caller spelled its path.
//
// WHAT THIS PROTECTS, and it is not the literal: it is the L3 probe's
// ours-vs-stale verdict at launch. Three renderers write this wall —
// `install-hooks` (dir from the operator's argument, or cwd),
// RenderSessionHooks and the L3 probe (both the session dir) — and the probe
// decides "ours" by rendering again and comparing BYTE FOR BYTE (ADR 0023).
// All three already funnel through one derivation, App.commitGuardLiterals,
// but its output still depended on how the caller spelled `dir`: ADR 0024 D2
// check 3's instance-path literal is filepath.Dir of the .beads/redirect
// target, and a RELATIVE redirect is joined onto that spelling as typed. So
// two absolute spellings of one repo rendered two walls that behave
// identically and differ in exactly three lines — MEASURED on
// ranger-base-k9x8n, darwin 25.6.0, `install-hooks <real>/repo` against
// `install-hooks <link>/repo` — and the second reads as "ours but stale" on
// every launch into that repo. That is ranger-base-up22's class reached
// through the path spelling instead of the visibility mark.
//
// BYTE EQUALITY IS THE ASSERTION, not a grep for the path, for the same
// reason the sibling in cmd/posse gives
// (installhooksrelativetarget_qa_test.go, which owns the relative-vs-absolute
// ARGUMENT axis through the real binary; this file owns the two-spellings-of-
// one-absolute-path axis at the derivation). One equality holds the claim for
// every literal the derivation grows later.
//
// WHY THE REDIRECT IS RELATIVE AND WHY IT DOES NOT CLIMB PAST THE SPELLING.
// `../store/.beads`, not `../../store/.beads`: filepath.Clean collapses a
// `..` against the segment in front of it, so a redirect that climbs ABOVE
// the symlinked or case-changed segment erases the very spelling this pin
// varies, and all three renders would agree before the fix as well. The
// fixture asserts that premise rather than trusting it — qcwTargets below
// shows the three naive joins are three distinct strings.
//
// MUTATION-CHECKED, twice, and the two mutants red DIFFERENT arms — which
// is what says the pin is measuring D1 and D2 separately rather than one
// equality twice:
//
//   - Reverting DeriveIdentityLiterals to `repo := filepath.Dir(target)`
//     with AbbrevHome (D2 undone) reds BOTH arms of the wall pin, 6
//     differing lines each — two literals, rendered in each of check 3's
//     three subjects — and leaves
//     TestQACanonicalPathIsTheKernelsSpelling green, since that one calls
//     canonicalPath directly.
//   - Making kernelPath return an error on darwin (D1's platform half
//     undone, so canonicalPath degrades to resolvedPath — exactly ADR
//     0082's rejected option (a)) leaves the SYMLINK arm green and reds the
//     CASE arm, here and in TestQACanonicalPathIsTheKernelsSpelling. That
//     is the ADR's reason for rejecting (a), measured rather than asserted.
//
// Reverting only the canonicalAbbrevHome half — canonicalPath on the repo,
// AbbrevHome on the home — leaves this file green, because both halves are
// canonical in this rig and the `~` form survives. That half is held next
// door, by TestDeriveIdentityLiterals' "the fixture must make them differ"
// premise in visibility_test.go.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// qcwRig is one scratch repo, a symlink to its parent, and the App that
// renders its wall. Every field is a SPELLING of something in it.
type qcwRig struct {
	a         *App
	home      string
	repo      string // <home>/real/Repo — the on-disk spelling
	viaLink   string // <home>/link/Repo — through a symlink to <home>/real
	viaCase   string // <home>/REAL/Repo — the same directory, other case
	wantStore string // <home>/real/store, the canonical instance path
}

func newQCWRig(t *testing.T) *qcwRig {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	// Two things about this one line.
	//
	// gitTempDir and not t.TempDir(), because this rig runs a `git init`:
	// a git tree on a bare temp root reds
	// TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot, whose claim is
	// that the cleanup tolerates a transient concurrent write.
	//
	// And CANONICAL, because the rig's whole subject is which spelling a
	// render picks: a temp root comes back through darwin's
	// /var -> /private/var symlink, so an unresolved one would make the
	// REFERENCE arm differ from the canonical answer too and the pin would
	// be measuring the rig. ADR 0082's Consequences says this in as many
	// words — a fixture that builds an expected literal from a temp dir
	// passes it through canonicalPath, and that is expected, not a
	// divergence.
	root := qaCanonicalPath(t, gitTempDir(t))
	r := &qcwRig{
		home: filepath.Join(root, "home"),
	}
	r.repo = filepath.Join(r.home, "real", "Repo")
	r.viaLink = filepath.Join(r.home, "link", "Repo")
	r.viaCase = filepath.Join(r.home, "REAL", "Repo")
	r.wantStore = filepath.Join(r.home, "real", "store")

	for _, d := range []string{
		filepath.Join(r.repo, ".beads"),
		filepath.Join(r.wantStore, ".beads"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(r.home, "real"), filepath.Join(r.home, "link")); err != nil {
		t.Fatal(err)
	}
	// RELATIVE — bd's own spelling, and the one that made the wall depend
	// on the caller's. See the file header on why it climbs exactly one
	// level.
	write(t, filepath.Join(r.repo, ".beads", beadsRedirect), filepath.Join("..", "store", ".beads")+"\n")

	if out, err := exec.Command("git", "-C", r.repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v %s", r.repo, err, out)
	}
	if out, err := exec.Command("git", "-C", r.repo, "config", "user.email", "t@example.com").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}

	// HOME is this process's for the duration: DeriveIdentityLiterals reads
	// it directly (canonicalAbbrevHome), and without it the ~ form and the
	// absolute form are one string, dedupe drops one, and the pin has one
	// literal less to vary.
	t.Setenv("HOME", r.home)
	cfg := filepath.Join(r.home, "config.yaml")
	// The ON-DISK spelling as the config key, so every arm's visibility
	// stamp is decided by samePath (the COMPARISON) and only the rendered
	// literal is left varying. An arm that fell through to unmarked→public
	// would differ in the stamp as well and the pin could not say which
	// half it had caught.
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+r.repo+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.a = &App{ConfigPath: cfg, Home: r.home}
	return r
}

// render is what the three renderers of this wall all do, through the one
// derivation they share.
func (r *qcwRig) render(t *testing.T, dir string) string {
	t.Helper()
	vis, _ := r.a.BeadsVisibility(hookRepo(dir))
	if vis != VisibilityPrivate {
		t.Fatalf("fixture premise: %s must stamp private, got %q — the pin would then be measuring the stamp", dir, vis)
	}
	lits, err := r.a.commitGuardLiterals(dir)
	if err != nil {
		t.Fatalf("commitGuardLiterals(%s): %v", dir, err)
	}
	return r.a.CommitGuardHook(vis, r.a.OpsPatternSet(), lits...)
}

func TestQAOneRepoRendersOneWallHoweverItsPathIsSpelled(t *testing.T) {
	r := newQCWRig(t)

	// THE PREMISE, asserted: the three spellings are three distinct
	// strings, and the naive join of the relative redirect onto each is
	// three distinct instance paths. Without this the pin could be green
	// over a fixture in which nothing ever varied.
	naive := func(dir string) string {
		return filepath.Dir(filepath.Clean(filepath.Join(dir, "..", "store", ".beads")))
	}
	seen := map[string]string{}
	for _, dir := range []string{r.repo, r.viaLink, r.viaCase} {
		if prev, dup := seen[naive(dir)]; dup {
			t.Fatalf("fixture premise: %s and %s join the relative redirect to the same string %q — the spelling this pin varies was cleaned away", prev, dir, naive(dir))
		}
		seen[naive(dir)] = dir
	}

	want := r.render(t, r.repo)
	// And the reference really does carry the literal, in its canonical
	// spelling, both ways round.
	for _, line := range []string{
		"posse_check 'instance-path' '" + identityLiteralERE("~/real/store") + "'",
		"posse_check 'instance-path-abs' '" + identityLiteralERE(r.wantStore) + "'",
	} {
		if !strings.Contains(want, line) {
			t.Fatalf("fixture premise: the reference wall must carry %s", line)
		}
	}

	for _, c := range []struct {
		name, dir string
		run       bool
	}{
		{"through a symlink to the repo's parent", r.viaLink, true},
		// darwin only, and only where the volume folds case — the axis
		// EvalSymlinks cannot close and F_GETPATH can (ADR 0082 D1). The
		// second conjunct is samePath asked out loud: on a case-INsensitive
		// volume (the APFS default, and what a temp root lands on here)
		// `<home>/REAL/Repo` is the same directory, and on a case-sensitive
		// one it does not exist at all — rendering it anyway would measure a
		// repo with no .beads/redirect in it.
		{"a case-changed spelling of the same directory", r.viaCase, runtime.GOOS == "darwin" && samePath(r.repo, r.viaCase)},
	} {
		if !c.run {
			t.Logf("skipped on this box: %s (%s)", c.name, c.dir)
			continue
		}
		if got := r.render(t, c.dir); got != want {
			t.Errorf("%s rendered a different wall than %s:\n%s", c.dir, r.repo, qcwDiffLines(got, want))
		} else {
			t.Logf("byte-identical: %s", c.name)
		}
	}
}

// qcwNoReach makes dir unopenable by this process for the rest of the test
// and reports whether it managed to. chmod 0 is the cheap local stand-in for
// a cage, and both land on the same branch of canonicalPath — an open() that
// returns an error (ADR 0082 D1 as amended, ranger-base-8j070, which has the
// census of which cage actually loses this open(): none of the three).
//
// The restore is registered IMMEDIATELY and not deferred by the caller,
// because the rig's root is a t.TempDir/gitTempDir: that cleanup was
// registered first and therefore runs last, so a mode-0 directory left
// behind is a cleanup failure rather than a test failure, reported against
// a test that passed.
func qcwNoReach(t *testing.T, dir string) bool {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Log("reach cannot be withdrawn from root — mode bits do not bind uid 0")
		return false
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Logf("cannot chmod %s: %v", dir, err)
		return false
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := kernelPath(dir); !errors.Is(err, fs.ErrPermission) {
		t.Logf("chmod 0 did not deny the open of %s (kernelPath err %v) — nothing to measure here", dir, err)
		_ = os.Chmod(dir, 0o755)
		return false
	}
	return true
}

// TestQACanonicalPathRefusesADirectoryItMayNotName is the amendment to ADR
// 0082 D1's third bullet, on its own: a directory that is THERE and that
// this process may not open has no canonical spelling to render, and
// canonicalPath says so rather than handing back resolvedPath's answer.
//
// WHY THE REFUSAL AND NOT THE FALLBACK. D1 first said the fallback answer
// "is deterministic for a given input, which is all the byte compare needs".
// canonicalPath's input is not the path string alone — the reading is an
// open(), so it is the string PLUS the rendering process's reach. The
// control arm below measures the consequence rather than asserting it:
// with reach withdrawn, resolvedPath still ANSWERS, and for the
// case-changed spelling of the same directory it answers a DIFFERENT
// string. Two renderers with different reach would therefore write two
// walls for one repo and the probe would read the second as "ours but
// stale" on every launch — ranger-base-k9x8n's defect reached through
// privilege instead of through the caller's spelling (ranger-base-g6bs7's
// finding).
//
// DARWIN ONLY, and that is the platform half rather than a thin pin:
// kernelPath off darwin answers errNoKernelPath for EVERY path, so
// canonicalPath never reaches the permission branch there and no reach axis
// exists to lose. D1's non-darwin half IS the fallback.
func TestQACanonicalPathRefusesADirectoryItMayNotName(t *testing.T) {
	// Parallel-safe, unlike its sibling below: this one reads no
	// environment and newQCWRig's t.Setenv("HOME") is the whole reason that
	// one cannot take it.
	t.Parallel()
	if runtime.GOOS != "darwin" {
		t.Skipf("%s has no kernel reading (ADR 0082 D1's non-darwin half IS the fallback), so no reach axis exists to lose", runtime.GOOS)
	}
	root := qaCanonicalPath(t, t.TempDir())
	dir := filepath.Join(root, "instance")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The reference, with reach: the kernel's spelling, and the same answer
	// through a case-changed spelling of the same directory. If this second
	// equality does not hold the volume is case-sensitive and the control
	// arm below has no divergence to find.
	cased := filepath.Join(root, "INSTANCE")
	caseFolds := samePath(dir, cased)
	if got := qaCanonicalPath(t, dir); got != dir {
		t.Fatalf("premise: canonicalPath(%q) = %q with reach", dir, got)
	}
	if caseFolds {
		if got := qaCanonicalPath(t, cased); got != dir {
			t.Fatalf("premise: canonicalPath(%q) = %q with reach, want the on-disk %q", cased, got, dir)
		}
	} else {
		t.Logf("case row skipped: %s is on a case-SENSITIVE volume", root)
	}

	if !qcwNoReach(t, dir) {
		t.Skip("reach could not be withdrawn on this box")
	}

	// THE DECISION: no answer, and the sentinel its readers ask for by
	// errors.Is — the L3 probe has to tell this class from the other
	// derivation error (a literal carrying a single quote), which is a
	// property of the value and identical in every process.
	got, err := canonicalPath(dir)
	if !errors.Is(err, errPathUnnameable) {
		t.Errorf("canonicalPath(%q) with no reach = (%q, %v), want errPathUnnameable", dir, got, err)
	}
	if got != "" {
		t.Errorf("the refusal answered %q as well — a caller that drops the error must get a loudly empty string, not a plausible second spelling", got)
	}
	// And the refusal is reached from UNDER the directory too (an
	// unreadable component above the one being named): the open() is of the
	// path, so every component is in it.
	if _, err := canonicalPath(filepath.Join(dir, "store")); !errors.Is(err, errPathUnnameable) {
		t.Errorf("a path below an unreadable component: err = %v, want errPathUnnameable", err)
	}

	// THE CONTROL, measured and not asserted: this is what D1's original
	// fallback would have rendered, and why it was silent.
	if r := resolvedPath(dir); r == "" {
		t.Errorf("premise: resolvedPath stopped answering with no reach, so the fallback was never the silent second spelling this pin is about")
	}
	if caseFolds {
		if r := resolvedPath(cased); r == dir {
			t.Errorf("premise: resolvedPath(%q) folded the case with no reach, so the two reaches would not have diverged", cased)
		} else {
			t.Logf("the fallback's second spelling, measured: resolvedPath(%q) = %q against the kernel's %q", cased, r, dir)
		}
	}

	// A MISSING path is still the fallback, and keeps being it with the
	// parent unreadable — "nothing is there" and "there and not mine" are
	// the two halves the amendment separates, and only one refuses.
	missing := filepath.Join(root, "no-such-dir")
	if g, err := canonicalPath(missing); err != nil || g != resolvedPath(missing) {
		t.Errorf("a missing path: canonicalPath = (%q, %v), want resolvedPath's %q and no error", g, err, resolvedPath(missing))
	}
}

// TestQAAWallWhoseInstancePathCannotBeNamedIsRefusedNotRespelled is the same
// amendment at the wall, which is the only place it matters — the pin
// ranger-base-g6bs7 asked for, beside the one above it: render the same repo
// over an instance directory that is readable and then is not.
//
// THE THREE ARMS ARE THREE DIFFERENT CLAIMS, and the third is the one the
// finding was about:
//
//  1. The DERIVATION refuses, with canonicalPath's own sentinel, so the two
//     installers and RenderSessionHooks refuse — which they already did for
//     a literal they could not render, and this joins that class.
//  2. The INSTALL refuses, through the real entry point, rather than
//     writing a wall spelled by whichever process happened to render it.
//  3. The L3 PROBE abstains. This is the live half: the probe deliberately
//     drops a derivation error ("an install that hit it already failed
//     loudly"), which is right for the single-quote class and wrong for
//     this one — the install may have HAD the reach the probe lacks. A
//     probe that degraded to no literals would compare a correct installed
//     wall against a render short two lines and call it l3Stale on every
//     launch, which is the whole defect with the silence moved. So the arm
//     asserts l3Unnameable AND, separately, that it is not l3Stale.
//
// ARM 3 IS ASKED THROUGH BOTH SPELLINGS, and the case one is not a bonus
// row — it is the only one that can catch the defect, MEASURED as a
// mutation arm (ranger-base-8j070): with canonicalPath reverted to the
// fallback, a probe asked through the ON-DISK spelling reads l3Held,
// because the fallback's answer for that spelling IS the canonical one and
// nothing moved. That is ranger-base-g6bs7's "latent on this box" in one
// line: reach cannot move a literal the caller already spelled canonically.
// Asked
// through the case-changed spelling of the same directory the same mutant
// reads l3Stale, which is ranger-base-k9x8n's verdict on a wall that is
// perfectly good.
func TestQAAWallWhoseInstancePathCannotBeNamedIsRefusedNotRespelled(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skipf("%s has no kernel reading (ADR 0082 D1's non-darwin half IS the fallback), so no reach axis exists to lose", runtime.GOOS)
	}
	r := newQCWRig(t)

	// WITH REACH: the wall renders, carries both instance-path literals,
	// and installs. That installed file is arm 3's subject — a wall that is
	// genuinely this home's current render, written by a process that could
	// name the instance directory.
	want := r.render(t, r.repo)
	slot, _, _, err := r.a.InstallCommitGuardHook(r.repo)
	if err != nil {
		t.Fatalf("install with reach: %v", err)
	}
	if got := qaReadFile(t, slot); got != want {
		t.Fatalf("premise: the installed slot is not the reference render:\n%s", qcwDiffLines(got, want))
	}
	if p := r.a.probeL3Hooks(r.repo, false); p.CommitGuardVerdict != l3Held {
		t.Fatalf("premise: with reach the probe must hold the slot it just installed, got verdict %v (%s)", p.CommitGuardVerdict, p.CommitGuardDegraded)
	}

	if !qcwNoReach(t, r.wantStore) {
		t.Skip("reach could not be withdrawn on this box")
	}

	// ARM 1 — the derivation.
	if lits, err := r.a.commitGuardLiterals(r.repo); !errors.Is(err, errPathUnnameable) {
		t.Errorf("commitGuardLiterals with no reach to %s = (%d literals, %v), want errPathUnnameable", r.wantStore, len(lits), err)
	}
	// ARM 2 — the install, through the entry point an operator types.
	if _, _, _, err := r.a.InstallCommitGuardHook(r.repo); !errors.Is(err, errPathUnnameable) {
		t.Errorf("InstallCommitGuardHook with no reach = %v, want errPathUnnameable — a refusal, not a wall spelled by this process's privileges", err)
	}
	if got := qaReadFile(t, slot); got != want {
		t.Errorf("the refused install moved the installed wall:\n%s", qcwDiffLines(got, want))
	}
	// ARM 3 — the probe, through both spellings. See the header on why the
	// second row is the one that can catch anything.
	for _, c := range []struct {
		name, dir string
		run       bool
	}{
		{"the on-disk spelling", r.repo, true},
		{"a case-changed spelling of the same directory", r.viaCase, samePath(r.repo, r.viaCase)},
	} {
		if !c.run {
			t.Logf("skipped on this box: %s (%s) — case-SENSITIVE volume", c.name, c.dir)
			continue
		}
		p := r.a.probeL3Hooks(c.dir, false)
		switch {
		case p.CommitGuardVerdict == l3Stale:
			t.Errorf("%s: the probe called a correct installed wall STALE because it could not name the instance directory — ranger-base-k9x8n's \"ours but stale on every launch\", reached through privilege (degraded: %s)", c.name, p.CommitGuardDegraded)
		case p.CommitGuardVerdict != l3Unnameable:
			t.Errorf("%s: probe verdict = %v, want l3Unnameable (degraded: %s)", c.name, p.CommitGuardVerdict, p.CommitGuardDegraded)
		}
		if p.CommitGuard {
			t.Errorf("%s: the probe vouched for a wall it could not render to compare — a privacy wall is not held by a comparison nobody made", c.name)
		}
		if !strings.Contains(p.CommitGuardDegraded, "NOT MEASURED") {
			t.Errorf("%s: the degraded line must say it did not look, got %q", c.name, p.CommitGuardDegraded)
		}
	}
}

// TestQACanonicalPathIsTheKernelsSpelling is D1 on its own, under the wall
// it exists for — so a platform half that stops folding an axis is named
// here rather than discovered as "two bodies of N lines are unequal".
func TestQACanonicalPathIsTheKernelsSpelling(t *testing.T) {
	t.Parallel()
	root := qaCanonicalPath(t, t.TempDir())
	real := filepath.Join(root, "real")
	repo := filepath.Join(real, "Repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	// The reference: canonicalPath of a path with nothing to fold is that
	// path. If this is not so, every row below is unreadable.
	if got := qaCanonicalPath(t, repo); got != repo {
		t.Fatalf("canonicalPath is not idempotent over its own answer: %q -> %q", repo, got)
	}

	// SYMLINK, on every platform — EvalSymlinks closes this axis too, so
	// this row holds for the non-darwin half as well.
	if got := qaCanonicalPath(t, filepath.Join(root, "link", "Repo")); got != repo {
		t.Errorf("a path through a symlink: canonicalPath = %q, want %q", got, repo)
	}

	// CASE, darwin and a case-folding volume only. This is the axis
	// EvalSymlinks does NOT close (it keeps the caller's case) and the whole
	// reason D1 is fcntl(F_GETPATH) and not option (a).
	cased := filepath.Join(real, "REPO")
	switch {
	case runtime.GOOS != "darwin":
		t.Logf("case row skipped: %s has no F_GETPATH half (ADR 0082 D1 ASSUMES case-sensitive volumes there)", runtime.GOOS)
	case !samePath(repo, cased):
		t.Logf("case row skipped: %s is on a case-SENSITIVE volume, where %q is a different directory and keeping it apart is correct", root, cased)
	default:
		if got := qaCanonicalPath(t, cased); got != repo {
			t.Errorf("a case-changed spelling: canonicalPath = %q, want the on-disk %q", got, repo)
		}
		// And the control for that row: EvalSymlinks really does NOT
		// answer it, so the row is measuring F_GETPATH and not a
		// normalization resolvedPath would have done anyway.
		if resolvedPath(cased) == repo {
			t.Errorf("premise: resolvedPath folded the case of %q, so the case row no longer measures the kernel reading", cased)
		}
	}

	// A MISSING PATH falls back to resolvedPath's string answer, which is
	// deterministic — that determinism is all the byte compare needs.
	missing := filepath.Join(repo, "no", "such", "dir")
	if got, want := qaCanonicalPath(t, missing), resolvedPath(missing); got != want {
		t.Errorf("a missing path: canonicalPath = %q, want resolvedPath's %q", got, want)
	}
	if a, b := qaCanonicalPath(t, missing), qaCanonicalPath(t, missing); a != b {
		t.Errorf("a missing path must answer the same twice: %q then %q", a, b)
	}
}

// qcwDiffLines is the differing lines, so a failure names the literal rather
// than the length of the two bodies.
func qcwDiffLines(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	n := len(g)
	if len(w) > n {
		n = len(w)
	}
	// lineAt, not `at`: cmd/testparallel resolves a called NAME against
	// every `at := func(...)` in the package, and one of those reaches the
	// plan-clock package var `blindT`. So a t.Parallel test that wants
	// nothing from this helper but a readable diff is reported as reaching
	// that var and goes UNCLEARED — which is what
	// TestQAOneHomeSpelledTwoWaysRendersOneCommitWall did, and why
	// TestQAParallelClearanceDoesNotWaiveAReasonNobodyCleared reds over a
	// local variable's name. A unique name is the honest fix; a parallelOK
	// line naming blindT would be clearing a reason this test does not have.
	lineAt := func(lines []string, i int) string {
		if i < len(lines) {
			return lines[i]
		}
		return "(no such line)"
	}
	var b strings.Builder
	shown := 0
	for i := 0; i < n && shown < 6; i++ {
		if lineAt(g, i) != lineAt(w, i) {
			fmt.Fprintf(&b, "  line %d\n    got:  %s\n    want: %s\n", i+1, lineAt(g, i), lineAt(w, i))
			shown++
		}
	}
	if b.Len() == 0 {
		return "  (no line differs — the bodies differ only in trailing bytes)\n"
	}
	return b.String()
}
