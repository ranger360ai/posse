package posse

// ADR 0014 §4 at the mount layer — ranger-base-yu5. `parity.go` has printed
// `L4 :ro overlay (…)` for a subtree glob since ranger-base-4ks and nothing
// rendered one; `cage.go` mounted the repo `:ro` for a bare Edit/Write deny
// and read `writable:` not at all. These are the mount lists, without an
// engine: the live half is TestLiveCageOverlayShapes (RHQ_LIVE_DOCKER=1) and
// `docs/adr/0014-path-scoped-writes.probe.sh`, which is where the claim that
// an overlapping bind works at all is MEASURED rather than asserted.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// overlayRepo is a scratch repo with the three directories the two shapes
// talk about, and a `.beads`/`.git` so the carve-outs have something to
// carve. Everything the mounts name has to EXIST — a bind of an absent
// source is a different measurement (see the absent-subtree case below).
func overlayRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"internal", "docs/adr", "docs/rca", ".beads", ".git"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return absResolve(dir)
}

// mountAt is the mount whose destination is p, and whether there is one.
// Destination rather than source because that is what the container sees
// and what the engine deduplicates on; compared RESOLVED on both sides so
// these cases are about the mode, and the one case that is about the
// spelling says so itself (TestOverlayIsSpelledTheWayTheRepoMountIs).
func mountAt(ms []CageMount, p string) (CageMount, bool) {
	for _, m := range ms {
		if absResolve(m.Dst) == absResolve(p) {
			return m, true
		}
	}
	return CageMount{}, false
}

// wantMode asserts one destination is mounted at one mode. The two failures
// it separates are different bugs: a missing mount is a rule with no wall,
// and a mount at the wrong mode is a wall pointing the wrong way.
func wantMode(t *testing.T, ms []CageMount, p string, ro bool, what string) {
	t.Helper()
	m, ok := mountAt(ms, p)
	if !ok {
		t.Errorf("%s: nothing is mounted at %s:\n%s", what, p, showMounts(ms))
		return
	}
	if m.RO != ro {
		t.Errorf("%s: %s is mounted ro=%v, want ro=%v (%s)", what, p, m.RO, ro, m.Why)
	}
}

func wantNoMount(t *testing.T, ms []CageMount, p string, what string) {
	t.Helper()
	if m, ok := mountAt(ms, p); ok {
		t.Errorf("%s: %s must not be mounted at all, got %+v", what, p, m)
	}
}

func showMounts(ms []CageMount) string {
	var b strings.Builder
	for _, m := range ms {
		mode := "rw"
		if m.RO {
			mode = "ro"
		}
		b.WriteString("  " + mode + " " + m.Dst + "\n")
	}
	return b.String()
}

// The deny-list shape (ADR 0014 §4, first bullet): the repo stays writable
// and the denied subtree is overlaid `:ro`. This is the shape the developer
// PID has — `Edit(docs/adr/**)` — and the whole point is that the REST of
// the repo is still writable, which is why the bare rule's `:ro` repo is
// not an answer to it (a wall bigger than the gate is a different gate).
func TestDenyListShapeOverlaysTheSubtreeReadOnly(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit(docs/adr/**), Write(docs/adr/**)]\n")
	ms := a.CageMounts(ag, e, dir, "s1")

	wantMode(t, ms, dir, false, "deny-list")
	wantMode(t, ms, filepath.Join(dir, "docs/adr"), true, "deny-list")
	wantNoMount(t, ms, filepath.Join(dir, "internal"), "deny-list")
	wantNoMount(t, ms, filepath.Join(dir, "docs/rca"), "deny-list")
	// A path-scoped deny is NOT the bare rule: the repo mount must not have
	// quietly become read-only, which is the over-enforcement ADR 0014 §2
	// refuses to count as realization.
	if m, _ := mountAt(ms, dir); m.RO {
		t.Errorf("a path-scoped deny must leave the repo writable: %+v", m)
	}
	// And the engine's read-only spelling is what actually renders it —
	// parity claims a mount flag, so the flag is the assertion.
	argv := e.RenderArgv(CageRender{Image: "img", Workdir: dir, Mounts: ms, Inner: []string{"x"}})
	adr := filepath.Join(dir, "docs/adr")
	if !argvHas(argv, "-v", adr+":"+adr+":ro") {
		t.Errorf("the overlay is the mount flag, not a claim: %q", argv)
	}
	if !argvHas(argv, "-v", dir+":"+dir) {
		t.Errorf("the repo itself rides in read-write: %q", argv)
	}
}

// The allow-list shape (second bullet): repo `:ro`, and each `writable:`
// extra comes back read-write over it. Before this bead L4 read `writable:`
// not at all — the key was honoured at L2 and silently dropped one tier up,
// which is a grant the matrix printed and the mount never made.
func TestAllowListShapeOverlaysWritableExtras(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\nwritable: [docs/adr]\n")
	ms := a.CageMounts(ag, e, dir, "s1")

	wantMode(t, ms, dir, true, "allow-list")
	wantMode(t, ms, filepath.Join(dir, "docs/adr"), false, "allow-list")
	wantNoMount(t, ms, filepath.Join(dir, "docs/rca"), "allow-list")
	// The two carve-outs a `:ro` repo always carries (ADR 0014 §4): the
	// store, so claim/comment/close survive the tier, and the git dir, so
	// the session can refresh an index. Both are L2's, and matching L2 is
	// the ADR's own instruction.
	wantMode(t, ms, filepath.Join(dir, ".beads"), false, "allow-list")
	wantMode(t, ms, filepath.Join(dir, ".git"), false, "allow-list")

	argv := e.RenderArgv(CageRender{Image: "img", Workdir: dir, Mounts: ms, Inner: []string{"x"}})
	adr := filepath.Join(dir, "docs/adr")
	if !argvHas(argv, "-v", dir+":"+dir+":ro") || !argvHas(argv, "-v", adr+":"+adr) {
		t.Errorf("repo :ro with a read-write extra over it: %q", argv)
	}
}

// deny-wins (ADR 0001) at a tier where ORDER cannot deliver it. At L2 the
// trailing deny block is below every grant and SBPL takes the last match;
// here the engine sorts binds by destination depth, so an extra INSIDE a
// denied subtree is deeper than the deny and would win. It is dropped
// instead, and the pair `posse agent check` warns about is a pair that
// really grants nothing.
func TestWritableExtraInsideADeniedSubtreeIsDropped(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit, Write, Edit(docs/**)]\nwritable: [docs/adr]\n")
	ms := a.CageMounts(ag, e, dir, "s1")

	wantMode(t, ms, dir, true, "deny-wins")
	wantNoMount(t, ms, filepath.Join(dir, "docs/adr"), "deny-wins")

	// The composition that must still work, and is the reason the allow
	// pass runs first: the extra is the region, the deny is inside it. The
	// deeper mount wins, so `docs` is writable and `docs/adr` is not.
	ag2 := cageAgent(t, a, "cage: container\ndeny: [Edit, Write, Edit(docs/adr/**)]\nwritable: [docs]\n")
	ms2 := a.CageMounts(ag2, e, dir, "s1")
	wantMode(t, ms2, dir, true, "extra containing a deny")
	wantMode(t, ms2, filepath.Join(dir, "docs"), false, "extra containing a deny")
	wantMode(t, ms2, filepath.Join(dir, "docs/adr"), true, "extra containing a deny")
}

// The rules that name no subtree, at the tier that would otherwise be the
// place to guess. `Edit(**)` is the bare rule spelled long (ADR 0014 §1) and
// is realized by the repo mount, not by an overlay; `Edit(**/*.md)` is a
// file filter that no wall we have expresses, and emitting a `:ro` overlay
// of `docs/adr` for it would be the mount enforcing a rule the matrix says
// nobody holds.
func TestBareAndFileFilterSpellingsMountNoOverlay(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)

	long := a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit(**), Write(**)]\n"), e, dir, "s1")
	wantMode(t, long, dir, true, "Edit(**)")
	if len(long) != len(a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\n"), e, dir, "s1")) {
		t.Errorf("Edit(**) must mount exactly what Edit mounts:\n%s", showMounts(long))
	}

	filt := a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit(**/*.md), Write(docs/adr/**/*.md)]\n"), e, dir, "s1")
	wantMode(t, filt, dir, false, "file filter")
	wantNoMount(t, filt, filepath.Join(dir, "docs/adr"), "file filter")
	wantNoMount(t, filt, filepath.Join(dir, "docs"), "file filter")

	// A PID with only the bare rule is unchanged (ADR 0014 verification 5):
	// same `:ro` repo, plus the carve-outs, and no overlay invented for it.
	bare := a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\n"), e, dir, "s1")
	wantMode(t, bare, dir, true, "bare")
	wantMode(t, bare, filepath.Join(dir, ".beads"), false, "bare")
	wantMode(t, bare, filepath.Join(dir, ".git"), false, "bare")
	wantNoMount(t, bare, filepath.Join(dir, "docs/adr"), "bare")
}

// Two edges of the deny direction that decide whether the overlay is a wall
// or a decoration:
//
// A denied subtree that does not exist yet still gets the overlay. The rule
// is about a PATH, and `mkdir docs/adr && touch docs/adr/x` is exactly what
// a persona does next; the engine creates the source in the writable parent
// that put the bind there (measured, the probe script) and the deny holds.
//
// A denied subtree that no mount covers gets NOTHING. It is already
// unwritable — the cage cannot see it — and binding it `:ro` to look
// thorough would hand over read access the boundary had refused.
func TestDeniedSubtreeAbsentIsStillDeniedAndInvisibleIsNotMounted(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)

	absent := a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit(docs/future/**)]\n"), e, dir, "s1")
	wantMode(t, absent, filepath.Join(dir, "docs/future"), true, "absent subtree")
	if _, err := os.Stat(filepath.Join(dir, "docs/future")); err == nil {
		t.Errorf("rendering the mount list must not create anything on the host")
	}

	outside := absResolve(t.TempDir())
	away := a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit("+outside+"/**)]\n"), e, dir, "s1")
	wantNoMount(t, away, outside, "subtree outside every mount")
}

// A denied subtree that IS one of the other mounts flips that mount rather
// than joining the list beside it: two binds with the same destination is
// an engine error, so an overlay of a path already mounted has to be an
// edit. The memory dir is the one a PID can plausibly name.
func TestOverlayOfAnExistingMountFlipsItRatherThanDuplicating(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit({memory}/**)]\n")
	if err := ag.EnsureMemoryDir(); err != nil {
		t.Fatal(err)
	}
	// The PID names the resolved directory; `{memory}` is not a glob syntax.
	ag = cageAgent(t, a, "cage: container\ndeny: [Edit("+ag.MemoryDir+"/**)]\n")
	if err := ag.EnsureMemoryDir(); err != nil {
		t.Fatal(err)
	}
	ms := a.CageMounts(ag, e, dir, "s1")

	wantMode(t, ms, ag.MemoryDir, true, "memory denied")
	seen := 0
	for _, m := range ms {
		if absResolve(m.Dst) == absResolve(ag.MemoryDir) {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("a destination may appear exactly once, saw it %d times:\n%s", seen, showMounts(ms))
	}
}

// The invariant behind the one above, over every shape this bead renders:
// no destination twice, ever. `docker run` refuses a duplicate mount point,
// so a list that carries one is a launch that does not happen.
func TestNoShapeRendersADuplicateMountDestination(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)
	for _, front := range []string{
		"cage: container\n",
		"cage: container\ndeny: [Edit, Write]\n",
		"cage: container\ndeny: [Edit(docs/adr/**), Write(docs/adr/**)]\n",
		"cage: container\ndeny: [Edit, Write]\nwritable: [docs/adr, docs/rca, .beads]\n",
		"cage: container\ndeny: [Edit, Write, Edit(docs/adr/**)]\nwritable: [docs, docs/adr]\n",
		"cage: container\ndeny: [Edit(docs/**), Edit(docs/adr/**), Write(docs/**)]\n",
	} {
		ag := cageAgent(t, a, front)
		if err := ag.EnsureMemoryDir(); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, m := range a.CageMounts(ag, e, dir, "s1") {
			if seen[m.Dst] {
				t.Errorf("%q renders %s twice:\n%s", front, m.Dst, showMounts(a.CageMounts(ag, e, dir, "s1")))
			}
			seen[m.Dst] = true
		}
	}
}

// `writable:` is one key with one reader (ranger-base-4ks's lesson pointed
// at the allow side): the set L2 puts in the profile and the set L4 mounts
// come from the same function, so an extra cannot be honoured at one tier
// and dropped at the other. The `~` and the relative form are both here
// because both are what a PID actually writes.
func TestWritableIsReadTheSameWayByBothWalls(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)
	home, _ := os.UserHomeDir()
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\nwritable: [docs/adr, ~/.cache]\n")
	if err := ag.EnsureMemoryDir(); err != nil {
		t.Fatal(err)
	}
	extras := pidWritableExtras(ag, dir)
	want := []string{absResolve(filepath.Join(dir, "docs/adr")), absResolve(filepath.Join(home, ".cache"))}
	if len(extras) != 2 || extras[0] != want[0] || extras[1] != want[1] {
		t.Fatalf("writable: resolves to %v, want %v", extras, want)
	}
	// The profile reads the same list...
	prof := a.SeatbeltWritable(ag, dir, a.GatesDir("p"))
	for _, w := range want {
		found := false
		for _, p := range prof {
			if p == w {
				found = true
			}
		}
		if !found {
			t.Errorf("L2's writable set dropped the extra %s: %v", w, prof)
		}
	}
	// ...and so does the mount list, for the extra that is a real directory.
	wantMode(t, a.CageMounts(ag, e, dir, "s1"), filepath.Join(dir, "docs/adr"), false, "writable at L4")
	// A relative extra with no session dir to join is dropped rather than
	// guessed at — the same answer Resolve gives a relative subtree glob.
	if got := pidWritableExtras(ag, ""); len(got) != 1 || got[0] != want[1] {
		t.Errorf("no session dir: only the absolute extra survives, got %v", got)
	}
}

// A read-write overlay whose source is not a directory is not mounted. The
// engine would CREATE it, which is nothing on a read-write parent and the
// mountpoint-creation failure rangerhq-6so measured inside a `:ro` one —
// and either way it is a grant of a path the operator did not make.
func TestWritableExtraThatIsNotADirectoryIsNotMounted(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "NOTES"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\nwritable: [docs/nope, NOTES]\n")
	ms := a.CageMounts(ag, e, dir, "s1")
	wantNoMount(t, ms, filepath.Join(dir, "docs/nope"), "absent extra")
	wantNoMount(t, ms, filepath.Join(dir, "NOTES"), "extra that is a file")
	// And the repo is still `:ro`, so nothing was quietly opened instead.
	wantMode(t, ms, dir, true, "absent extra")

	// The same guard on the other branch — an extra OUTSIDE every mount,
	// which is a whole new bind rather than an overlay. Here the engine
	// would create the source on the host, so the launch would silently
	// make a directory the operator never wrote and hand it over writable.
	away := absResolve(t.TempDir())
	if err := os.WriteFile(filepath.Join(away, "file"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\nwritable: ["+
		filepath.Join(away, "nope")+", "+filepath.Join(away, "file")+", "+away+"]\n"), e, dir, "s1")
	wantNoMount(t, out, filepath.Join(away, "nope"), "absent extra outside the repo")
	wantNoMount(t, out, filepath.Join(away, "file"), "extra outside the repo that is a file")
	wantMode(t, out, away, false, "extra outside the repo that is a directory")
}

// The store of record when a redirect puts it in another repo (ADR 0012
// D3-C): `<dir>/.beads` holds a path and nothing else, so the carve-out
// mounts a directory nothing writes and every mutation lands outside the
// cage — the L4 shape of the L2 failure ranger-base-rhw measured. The
// target is mounted read-write and its repo is NOT: the inner wrapper
// appends JSONL and never commits.
func TestRedirectedBeadStoreCrossesTheBoundaryReadWrite(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	dir := overlayRepo(t)
	store := absResolve(t.TempDir())
	target := filepath.Join(store, ".beads")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".beads", "redirect"), []byte(target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, front := range []string{"cage: container\ndeny: [Edit, Write]\n", "cage: container\n"} {
		ms := a.CageMounts(cageAgent(t, a, front), e, dir, "s1")
		wantMode(t, ms, target, false, "redirect target ("+front+")")
		wantNoMount(t, ms, store, "the instance repo itself")
		wantNoMount(t, ms, filepath.Join(store, ".git"), "the instance repo's git dir")
	}
}

// The `.git` carve-out in the shape a worktree has it, NARROWED
// (ranger-base-t4f1, closing ranger-base-6q5e). `<dir>/.git` there is a FILE
// on the repo mount, so the overlay that answers an ordinary repo answers
// nothing: the index, HEAD and objects are all in the common dir, which is a
// mount of its own.
//
// Mounted whole read-write — which is what ranger-base-yu5 shipped — a caged
// persona could move `refs/heads/main`, rewrite `packed-refs`, move
// `core.hooksPath` in `config` (dodging the hooks-:ro overlay of
// ranger-base-3c3/h15 rather than being stopped by it) and edit another
// session's `worktrees/<name>`. So it is `:ro`, with read-write overlays of
// the three regions a DETACHED-HEAD commit actually writes — the same set
// sessionGitGrants names at L2 minus the ref pair, because the launcher
// detaches HEAD (PrepareSessionHead) and splices at close instead.
//
// This test measures the MOUNT LIST. That the engine honours a read-write
// bind over a `:ro` parent is measured in
// docs/adr/0014-path-scoped-writes.probe.sh (7/7, ranger-base-yu5), and this
// bead's own arms are in docs/adr/0014-l4-worktree-narrowing.probe.sh.
func TestWorktreeGitCommonDirIsTheGitCarveOut(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	main := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir, c.Env = dir, append(os.Environ(), "PATH="+PathOutsideGates(""))
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, b)
		}
	}
	run(main, "init", "-b", "main")
	run(main, "config", "user.email", "t@example.com")
	run(main, "config", "user.name", "t")
	os.WriteFile(filepath.Join(main, "f"), []byte("x\n"), 0o644)
	run(main, "add", "f")
	run(main, "commit", "-m", "one")
	// Resolved before git sees it: mounts are same-path in and out, and a
	// fixture mixing /tmp with its /private/tmp real path would be testing
	// the symlink rather than the carve-out.
	wt := filepath.Join(absResolve(t.TempDir()), "wt")
	run(main, "worktree", "add", wt)

	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	hooks, err := hooksDir(wt)
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Dir(hooks)
	own := LinkedGitDirs(wt)
	if len(own) != 2 {
		t.Fatalf("a linked worktree has two git dirs, got %v", own)
	}
	// `logs/` is made by the launcher (PrepareSessionHead) because a
	// read-write overlay of an absent source is dropped by cageOverlay's Stat
	// guard. Made here for the same reason; the arm that measures the drop is
	// at the bottom of this test.
	if err := os.MkdirAll(filepath.Join(common, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, front := range []string{"cage: container\ndeny: [Edit, Write]\n", "cage: container\n"} {
		ms := a.CageMounts(cageAgent(t, a, front), e, wt, "s1")
		wantMode(t, ms, common, true, "worktree git common dir ("+front+")")
		wantMode(t, ms, own[0], false, "the session's own per-worktree git dir")
		wantMode(t, ms, filepath.Join(common, "objects"), false, "objects")
		wantMode(t, ms, filepath.Join(common, "logs"), false, "logs")
		// The four the narrowing exists for: each is inside the `:ro` common
		// mount and under NO overlay, so the deepest bind covering it is the
		// read-only one. A mount of its own here — in either mode — would be
		// the bug, so the assertion is that nothing lands on them at all.
		for _, p := range []string{"config", "hooks", "packed-refs", "refs"} {
			wantNoMount(t, ms, filepath.Join(common, p), "<common>/"+p+" stays under the :ro mount ("+front+")")
		}
		// Another session's tree, which is the sibling of the one overlay
		// that IS granted — the case depth-ordering makes possible and the
		// reason the grant is `worktrees/<own>` and not `worktrees`.
		wantNoMount(t, ms, filepath.Join(common, "worktrees"), "worktrees/ itself")
		wantNoMount(t, ms, filepath.Join(common, "worktrees", "someone-else"), "another session's per-worktree dir")
		// And `<wt>/.git` is a file, so nothing tried to overlay it — a bind
		// of a non-directory is the grant this must not invent.
		wantNoMount(t, ms, filepath.Join(wt, ".git"), "the worktree's .git pointer file")
	}
	wantMode(t, a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\n"), e, wt, "s1"), wt, true, "worktree repo")

	// The wall removed: with no `logs/` on disk the read-write overlay is
	// DROPPED (cageOverlay's Stat guard), which is what the launcher's
	// mkdir exists to prevent — and it is the arm that proves the three
	// overlays above are read off the tree rather than spelled from a list.
	if err := os.RemoveAll(filepath.Join(common, "logs")); err != nil {
		t.Fatal(err)
	}
	ms := a.CageMounts(cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\n"), e, wt, "s1")
	wantNoMount(t, ms, filepath.Join(common, "logs"), "logs with no source on disk")
}

// The overlay is spelled the way the mount it lands on is spelled, not the
// way the host resolves it. A session dispatched through a symlinked parent
// — `/tmp/x` on macOS, which is really `/private/tmp/x` — mounts the repo at
// the path it was given, and inside the container there is no symlink to
// follow. An overlay resolved for the host would land at a destination
// nothing mounts: the bind would succeed, `posse cage` would print it, the
// matrix would say `L4 :ro overlay`, and the denied subtree would be
// writable at the only path the persona can reach it by.
func TestOverlayIsSpelledTheWayTheRepoMountIs(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	e, _ := a.LoadEngine("fake")
	real := overlayRepo(t)
	link := filepath.Join(t.TempDir(), "repo")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if absResolve(link) == link {
		t.Fatalf("the fixture must be a symlink the resolver actually moves: %s", link)
	}
	ag := cageAgent(t, a, "cage: container\ndeny: [Edit(docs/adr/**)]\n")
	ms := a.CageMounts(ag, e, link, "s1")

	if ms[0].Dst != link {
		t.Fatalf("the repo mounts at the path the session was given: %+v", ms[0])
	}
	m, ok := mountAt(ms, filepath.Join(link, "docs/adr"))
	if !ok || !m.RO {
		t.Fatalf("the denied subtree must still be overlaid :ro:\n%s", showMounts(ms))
	}
	if want := filepath.Join(link, "docs/adr"); m.Dst != want {
		t.Errorf("the overlay lands at %s, which is not inside the repo mount %s — want %s", m.Dst, ms[0].Dst, want)
	}
	// Same path in and out, like every other mount: the source follows the
	// covering mount's spelling rather than the resolver's, and the host is
	// what follows the symlink. The alternative — a resolved source under an
	// unresolved destination — would make one bind disagree with the bind it
	// sits on about which tree this is.
	if m.Src != m.Dst {
		t.Errorf("the overlay is same-path in and out: %+v", m)
	}
	// And an allow-direction overlay is spelled the same way.
	ag2 := cageAgent(t, a, "cage: container\ndeny: [Edit, Write]\nwritable: [docs/adr]\n")
	w, ok := mountAt(a.CageMounts(ag2, e, link, "s1"), filepath.Join(link, "docs/adr"))
	if !ok || w.RO || w.Dst != filepath.Join(link, "docs/adr") {
		t.Errorf("a writable: extra lands inside the repo mount too: %+v", w)
	}
}
