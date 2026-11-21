//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-5blmt (design ranger-base-hxens): darwin's seatbelt seat
// commits into its own object store. Three pins, one per half and one over
// both:
//
//   - the RENDER (any host): for a linked worktree the seatbelt writable set
//     covers neither `<common>/objects` nor `<common>` nor any ancestor of
//     it, the store under the wall dir is granted, and the carve-out's Deny
//     names `<common>/objects` — and with a PID `writable:` extra over an
//     ancestor, the seal names the ancestors that grant made renamable.
//   - the L1 SHIM ARM (any host with git; no sandbox): the env is set for the
//     seat's own repo, and only for it, resolved the way git resolves it.
//   - the WALL under the RENDERED profile (darwin only): a no-env object
//     write is refused, every mutation of the shared store is refused, and a
//     commit through the shim lands in the seat store.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ssFixture is wgNewFixture's worktree, detached the way every walled seat
// is (PrepareSessionHead), with the store's paths named the way the launcher
// names them.
type ssFixture struct {
	wgFixture
	objects, store string
}

func ssNewFixture(t *testing.T) ssFixture {
	t.Helper()
	f := wgNewFixture(t)
	mustGit(t, f.tree, "checkout", "-q", "--detach")
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("no HOME for the fixture: %v", err)
	}
	l, ok := seatStoreForHome(home, f.tree)
	if !ok || l.Store == "" {
		t.Fatalf("no seat store layout for %s — the test asserts nothing", f.tree)
	}
	return ssFixture{wgFixture: f, objects: absResolve(filepath.Join(f.common, "objects")), store: absResolve(l.Store)}
}

func ssHas(list []string, p string) bool {
	for _, x := range list {
		if absResolve(x) == absResolve(p) {
			return true
		}
	}
	return false
}

// ─── the render ──────────────────────────────────────────────────────────────

func TestQASeatbeltSeatStoreRenderDeniesTheSharedStore(t *testing.T) {
	f := ssNewFixture(t)
	// CONTROL: the producer both mechanisms read still grants the shared
	// store, so the swap below is what takes it away — and bwrap, which
	// mounts the store over it, still sees the grant it binds.
	if !sbCovers(f.a.SeatbeltWritable(f.ag, f.tree, f.gates), f.objects) {
		t.Fatalf("premise gone: SeatbeltWritable no longer grants %s, so nothing here measures the swap", f.objects)
	}
	w, c := f.a.seatbeltSets(f.ag, f.tree, f.gates)
	for p := f.objects; ; p = filepath.Dir(p) {
		if writeGranted(w, p) {
			t.Errorf("the seatbelt writable set reaches %s (the shared store, <common> or an ancestor of it):\n  %s", p, strings.Join(w, "\n  "))
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	if !sbCovers(w, f.store) {
		t.Errorf("the seat store %s is in no grant — a seatbelt seat cannot commit:\n  %s", f.store, strings.Join(w, "\n  "))
	}
	if !ssHas(c.Deny, f.objects) {
		t.Errorf("the carve-out does not deny %s explicitly:\n  %s", f.objects, strings.Join(c.Deny, "\n  "))
	}
	// And the profile that runs says both.
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, prof)
	deny := body[strings.LastIndex(body, "(deny file-write*"):]
	if !strings.Contains(deny, sbQuote(f.objects)) {
		t.Errorf("the rendered profile's trailing deny does not name %s:\n%s", f.objects, body)
	}
	if !strings.Contains(body, sbQuote(f.store)) {
		t.Errorf("the rendered profile does not grant the seat store %s:\n%s", f.store, body)
	}
}

// A PID `writable:` extra over an ancestor of the shared store re-grants it.
// The explicit deny takes it back, and the seal makes every ancestor that
// grant made renamable unrenamable — or `mv <common> x` would carry the
// shared store out from under its own deny.
func TestQASeatbeltSeatStoreSealsAnAncestorAPIDExtraGrants(t *testing.T) {
	f := ssNewFixture(t)
	ag := *f.ag
	ag.Writable = []string{f.repo}
	w, c := f.a.seatbeltSets(&ag, f.tree, f.gates)
	if !writeGranted(w, f.common) {
		t.Fatalf("premise gone: the extra %s does not cover %s, so the seal is not under test:\n  %s", f.repo, f.common, strings.Join(w, "\n  "))
	}
	if !ssHas(c.Deny, f.objects) {
		t.Errorf("an ancestor grant is in the set and the shared store is not denied:\n  %s", strings.Join(c.Deny, "\n  "))
	}
	if !ssHas(c.Seal, f.common) {
		t.Errorf("%s is renamable under the extra and the seal does not name it:\n  seal: %s", f.common, strings.Join(c.Seal, "\n  "))
	}
}

// A PID that denies the whole tree has no `objects` grant to swap (ADR 0086
// D3), so it gets no store grant and no launch env either.
func TestQASeatbeltSeatStoreSkipsAWholeTreeDenyPID(t *testing.T) {
	f := ssNewFixture(t)
	ag := *f.ag
	ag.Deny = []string{"Edit", "Write"}
	if !deniesFileWrite(ag.Deny) {
		t.Fatal("premise gone: the fixture PID does not deny the whole tree")
	}
	w, _ := f.a.seatbeltSets(&ag, f.tree, f.gates)
	if sbCovers(w, f.store) {
		t.Errorf("a whole-tree-deny PID was granted a seat store:\n  %s", strings.Join(w, "\n  "))
	}
	home, _ := os.UserHomeDir()
	if env := seatStoreEnv(&ag, home, f.tree); env != nil {
		t.Errorf("a whole-tree-deny PID was handed the seat store env: %v", env)
	}
}

// The launch line carries the two variables the shim reads, on the seatbelt
// mechanism's prefix.
func TestQASeatbeltSeatStoreEnvRidesThePrefix(t *testing.T) {
	f := ssNewFixture(t)
	r, err := f.a.renderWallOn(WallSandboxExec, f.ag, nil, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range []string{seatObjectsEnvVar + "=" + f.store, seatCommonEnvVar + "=" + f.common} {
		if !strings.Contains(r.Prefix, shellQuote(kv)) {
			t.Errorf("the seatbelt prefix does not carry %s:\n  %s", kv, r.Prefix)
		}
	}
}

// ─── the L1 shim arm ─────────────────────────────────────────────────────────

// ssGitEnv pins git's config for the fixture the way the probe lessons say:
// GIT_CONFIG_GLOBAL at a scratch file (HOME alone is not isolation,
// ranger-base-161t2) and XDG_CONFIG_HOME UNSET. Set before RenderGates, so
// the shim's hook-skip reference is this same environment.
func ssGitEnv(t *testing.T, root string) {
	t.Helper()
	g := filepath.Join(root, "gitconfig")
	sbWrite(t, g, "[user]\n\tname = ss\n\temail = ss@example.com\n[init]\n\tdefaultBranch = main\n")
	t.Setenv("GIT_CONFIG_GLOBAL", g)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("XDG_CONFIG_HOME", "")
	os.Unsetenv("XDG_CONFIG_HOME")
}

func ssCount(t *testing.T, objects string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(objects, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

type ssShim struct {
	bin string
	env []string
}

func (s ssShim) run(t *testing.T, dir string, extra []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(s.bin, args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, s.env...), extra...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (s ssShim) must(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := s.run(t, dir, nil, args...)
	if err != nil {
		t.Fatalf("shim git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return out
}

func ssPrepare(t *testing.T, f ssFixture) (ssShim, string) {
	t.Helper()
	root := filepath.Dir(filepath.Dir(f.tree))
	ssGitEnv(t, root)
	if err := prepareSeatStoreAt(&SessionTree{Path: f.tree, Repo: f.repo}, true); err != nil {
		t.Fatal(err)
	}
	if b := mustRead(t, filepath.Join(f.store, "info", "alternates")); strings.TrimSpace(b) != f.objects {
		t.Fatalf("a sideless store's alternates name %q, want the shared store %s", b, f.objects)
	}
	if _, err := os.Lstat(filepath.Join(f.own, seatStoreBaseName)); err == nil {
		t.Errorf("a sideless prepare made the side path %s, which nothing mounts", seatStoreBaseName)
	}
	_, binDir, _, err := f.a.RenderGates(f.ag.Name, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "PATH="+PathOutsideGates(binDir),
		seatObjectsEnvVar+"="+f.store, seatCommonEnvVar+"="+f.common)
	other := filepath.Join(root, "other")
	mustGit(t, root, "init", "-q", other)
	commitIn(t, other, "o.txt", "o\n", "other base")
	return ssShim{bin: filepath.Join(binDir, "git"), env: env}, other
}

func TestQASeatObjectEnvScopesToTheSeatsOwnRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	f := ssNewFixture(t)
	s, other := ssPrepare(t, f)
	before := ssCount(t, f.objects)

	// (1) a detached commit through the shim lands in the seat store.
	sbWrite(t, filepath.Join(f.tree, "y.txt"), "y\n")
	s.must(t, f.tree, "add", "y.txt")
	s.must(t, f.tree, "commit", "-q", "-m", "seat", "--", "y.txt")
	h := s.must(t, f.tree, "rev-parse", "HEAD")
	if _, err := os.Stat(filepath.Join(f.store, h[:2], h[2:])); err != nil {
		t.Errorf("the seat's commit %s is not in the seat store %s", h, f.store)
	}
	if got := ssCount(t, f.objects); got != before {
		t.Errorf("the shared store gained %d files from a seat commit", got-before)
	}
	if !strings.Contains(s.must(t, f.tree, "log", "--format=%s"), "seed") {
		t.Error("base history does not read through the alternate")
	}
	s.must(t, f.tree, "fsck", "--connectivity-only")

	// The env is the seat's for every spelling of its own repo...
	for _, c := range []struct {
		dir  string
		args []string
	}{
		{f.tree, nil},
		{f.repo, []string{"-C", f.tree}},
		{filepath.Dir(f.tree), []string{"-C", "", "-C", filepath.Base(f.tree)}},
		{f.repo, nil}, // the main checkout shares the common dir: the same repo
	} {
		args := append(append([]string{}, c.args...), "rev-parse", "--git-path", "objects")
		got := s.must(t, c.dir, args...)
		if !filepath.IsAbs(got) {
			got = filepath.Join(c.dir, got)
		}
		if absResolve(got) != f.store {
			t.Errorf("git %s from %s resolves objects to %s, want the seat store", strings.Join(args, " "), c.dir, got)
		}
	}
	// ...and never for another repo's, by any route git takes to it.
	otherObjects := absResolve(filepath.Join(other, ".git", "objects"))
	for _, c := range []struct {
		name  string
		extra []string
		args  []string
	}{
		{"-C other", nil, []string{"-C", other}},
		{"-C seat -C other", nil, []string{"-C", f.tree, "-C", other}},
		{"--git-dir other", nil, []string{"--git-dir", filepath.Join(other, ".git")}},
		{"--git-dir=other", nil, []string{"--git-dir=" + filepath.Join(other, ".git")}},
		{"GIT_DIR=other", []string{"GIT_DIR=" + filepath.Join(other, ".git")}, nil},
	} {
		args := append(append([]string{}, c.args...), "rev-parse", "--path-format=absolute", "--git-path", "objects")
		got, err := s.run(t, f.tree, c.extra, args...)
		if err != nil {
			t.Errorf("%s: %v\n%s", c.name, err, got)
			continue
		}
		if absResolve(got) != otherObjects {
			t.Errorf("%s from the seat resolves objects to %s, want the other repo's own %s", c.name, got, otherObjects)
		}
	}

	// (3) a commit in the other repo from inside the seat is whole in its own store.
	sbWrite(t, filepath.Join(other, "p.txt"), "p\n")
	s.must(t, f.tree, "-C", other, "add", "p.txt")
	s.must(t, f.tree, "-C", other, "commit", "-q", "-m", "other", "--", "p.txt")
	mustGit(t, other, "fsck", "--connectivity-only")
	oh := mustGit(t, other, "rev-parse", "HEAD")
	if _, err := os.Stat(filepath.Join(other, ".git", "objects", oh[:2], oh[2:])); err != nil {
		t.Errorf("the other repo's commit %s is not in its own store", oh)
	}

	// clone and init from inside the seat make a repo of their own. The
	// clone's source is the OTHER repo: the seat's own history is reachable
	// through the alternate, so a clone of it would pass with the env set.
	// MEASURED 2026-11-20, git 2.56.0, linux: with GIT_OBJECT_DIRECTORY set,
	// `clone` dies at "trying to write ref ... with nonexistent object" and
	// `init` makes a repo with no objects dir at all.
	cl := filepath.Join(filepath.Dir(other), "cl")
	s.must(t, f.tree, "clone", "-q", other, cl)
	mustGit(t, cl, "fsck", "--connectivity-only")
	ini := filepath.Join(filepath.Dir(other), "ini")
	s.must(t, f.tree, "init", "-q", ini)
	if fi, err := os.Stat(filepath.Join(ini, ".git", "objects")); err != nil || !fi.IsDir() {
		t.Errorf("an init from inside the seat made no objects dir of its own (%v) — the env reached it", err)
	}

	// A CDPATH that matches makes `cd` print; the arm's answer must not
	// carry that text, or the seat's own repo stops matching.
	got, err := s.run(t, filepath.Dir(f.tree), []string{"CDPATH=" + filepath.Dir(f.tree)},
		"-C", filepath.Base(f.tree), "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil || absResolve(got) != f.store {
		t.Errorf("with CDPATH set, -C into the seat resolves objects to %q (%v), want the seat store", got, err)
	}

	// CONTROL: with no launcher variables the arm is inert.
	plain := ssShim{bin: s.bin, env: s.env[:len(s.env)-2]}
	got = plain.must(t, f.tree, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if absResolve(got) != f.objects {
		t.Errorf("with no seat variables the shim still redirected objects to %s", got)
	}
}

// ─── the wall, under the rendered profile (darwin) ───────────────────────────

func ssSandbox(t *testing.T, prof string, env []string, sh string) (bool, string) {
	t.Helper()
	cmd := exec.Command("sandbox-exec", "-f", prof, "/bin/sh", "-c", sh)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}

func TestQASeatbeltSeatStoreWallUnderTheRenderedProfile(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	f := ssNewFixture(t)
	s, _ := ssPrepare(t, f)
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	q := shellQuote
	victims := func() []string {
		var out []string
		_ = filepath.WalkDir(f.objects, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && len(filepath.Base(filepath.Dir(p))) == 2 {
				out = append(out, p)
			}
			return nil
		})
		return out
	}
	v := victims()
	if len(v) < 5 {
		t.Fatalf("SETUP: only %d loose objects in the shared store; the arms below would be vacuous", len(v))
	}

	// (0) a git that bypasses the shim gets no env, and its object write is refused.
	sbWrite(t, filepath.Join(f.tree, "x.txt"), "x\n")
	if ok, out := ssSandbox(t, prof, s.env, "cd "+q(f.tree)+" && "+q(real)+" hash-object -w x.txt"); ok {
		t.Errorf("(0) a no-env object write succeeded under the rendered profile — the deny is not holding:\n%s", out)
	}

	// (2) every mutation of the shared store, each with a CONTROL outside the sandbox.
	before := ssCount(t, f.objects)
	for _, op := range []struct{ name, sh string }{
		{"overwrite", `chmod u+w "$1" && printf z >"$1"`},
		{"unlink", `/bin/rm -f "$1" && [ ! -e "$1" ]`},
		{"chmod", `/bin/chmod 666 "$1"`},
		{"rename", `/bin/mv "$1" "$1.x"`},
		{"hardlink", `/bin/ln "$1" "$1.l"`},
	} {
		ctl := filepath.Join(filepath.Dir(f.tree), "ctl."+op.name)
		b, _ := os.ReadFile(v[0])
		sbWrite(t, ctl, string(b))
		if out, err := exec.Command("/bin/sh", "-c", op.sh, "_", ctl).CombinedOutput(); err != nil {
			t.Errorf("CONTROL %s is dead outside the sandbox (%v: %s) — the arm is vacuous", op.name, err, out)
		}
		if ok, out := ssSandbox(t, prof, s.env, "set -- "+q(v[1])+"; "+op.sh); ok {
			t.Errorf("(2) %s of a shared object was allowed:\n%s", op.name, out)
		}
	}
	for name, sh := range map[string]string{
		"create in objects":            "echo z >" + q(filepath.Join(f.objects, "info", "new")),
		"mkdir in objects":             "mkdir " + q(filepath.Join(f.objects, "zz")),
		"hardlink into a writable dir": "ln " + q(v[2]) + " " + q(filepath.Join(f.own, "escape")),
		"rename out of objects":        "mv " + q(v[3]) + " " + q(filepath.Join(f.own, "moved")),
		"rename of the objects dir":    "mv " + q(f.objects) + " " + q(f.objects+".x"),
		"rename of <common>":           "mv " + q(f.common) + " " + q(f.common+".x"),
	} {
		if ok, out := ssSandbox(t, prof, s.env, sh); ok {
			t.Errorf("(2) %s was allowed:\n%s", name, out)
		}
	}
	if got := ssCount(t, f.objects); got != before {
		t.Errorf("(2) the shared store changed: %d files, was %d", got, before)
	}

	// (1) the same profile lets a commit through the shim land in the seat store.
	h0 := mustGit(t, f.tree, "rev-parse", "HEAD")
	sh := "cd " + q(f.tree) + " && echo w > w.txt && " + q(s.bin) + " add w.txt && " + q(s.bin) + " commit -q -m caged -- w.txt"
	if ok, out := ssSandbox(t, prof, s.env, sh); !ok {
		t.Fatalf("(1) a commit through the shim is refused under the rendered profile:\n%s\n\nprofile:\n%s", out, mustRead(t, prof))
	}
	h := s.must(t, f.tree, "rev-parse", "HEAD")
	if h == h0 {
		t.Fatal("(1) git exited 0 and HEAD did not move")
	}
	if _, err := os.Stat(filepath.Join(f.store, h[:2], h[2:])); err != nil {
		t.Errorf("(1) the caged commit %s is not in the seat store", h)
	}
	if got := ssCount(t, f.objects); got != before {
		t.Errorf("(1) the shared store gained %d files from a caged commit", got-before)
	}
}
