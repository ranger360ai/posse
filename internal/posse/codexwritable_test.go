package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A codex session's writes are confined to its workspace, so the store of
// record — which ADR 0012 D3-C puts behind a .beads redirect, outside the
// session dir — has to be named or every bd write is denied (ranger-base-0fb).
func TestCodexAddsTheBeadsRedirectTargetAsWritable(t *testing.T) {
	r := realizeCodex(nil, nil, "/m/personas/developer", "/elsewhere/ranger-base/.beads")
	want := "-s workspace-write --add-dir '/m/personas/developer' --add-dir '/elsewhere/ranger-base/.beads'"
	if r.Deny != want {
		t.Fatalf("codex sandbox flags:\n got %q\nwant %q", r.Deny, want)
	}
}

// read-only never takes --add-dir: codex exits on it (rangerhq-5oi).
func TestCodexReadOnlyStillRefusesAddDir(t *testing.T) {
	r := realizeCodex(nil, []string{"Edit", "Write"}, "/m", "/elsewhere/.beads")
	if r.Deny != "-s read-only" {
		t.Fatalf("read-only must carry no --add-dir, got %q", r.Deny)
	}
}

// The same path named twice must not produce two flags.
func TestCodexWritableDedupes(t *testing.T) {
	r := realizeCodex(nil, nil, "/same", "/same")
	if r.Deny != "-s workspace-write --add-dir '/same'" {
		t.Fatalf("dedupe failed: %q", r.Deny)
	}
}

// Runtimes posse cages itself must be untouched by the new parameter.
func TestClaudeAndGrokIgnoreWritable(t *testing.T) {
	c := realizeClaude(nil, nil, "/m", "/elsewhere/.beads")
	g := realizeGrok(nil, nil, "/m", "/elsewhere/.beads")
	for n, got := range map[string]string{"claude": c.Deny, "grok": g.Deny} {
		if containsAddDir(got) {
			t.Fatalf("%s must not emit --add-dir, got %q", n, got)
		}
	}
}

func containsAddDir(s string) bool { return strings.Contains(s, "--add-dir") }

// codex refuses a writable root that has a symlink COMPONENT, and it refuses
// it when a command runs rather than when the session starts — so a posse box
// whose ~/.config/posse/personas is a symlink into the constitution tree
// launched codex sessions that came up, read their prompt, and could then run
// nothing at all, silently (ranger-base-c02a, measured live on codex-cli
// 0.150.1). The root must come out resolved.
//
// The fixture builds its own symlink rather than leaning on t.TempDir()'s:
// /var -> /private/var makes this discriminate on macOS for free, and on
// Linux the wrong arm would render the literal path and the pin would be
// green over the bug.
func TestCodexResolvesASymlinkedWritableRoot(t *testing.T) {
	real := t.TempDir()
	developer := filepath.Join(real, "personas", "developer")
	if err := os.MkdirAll(developer, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "personas")
	if err := os.Symlink(filepath.Join(real, "personas"), link); err != nil {
		t.Fatal(err)
	}
	symlinked := filepath.Join(link, "developer")
	// The fixture really is the shape the bug needs: a path that exists and
	// whose PARENT is the symlink. Without this the arms below could both
	// pass over a fixture codex would have accepted anyway.
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("fixture: %s must be a symlink: %v", link, err)
	}
	if _, err := os.Stat(symlinked); err != nil {
		t.Fatalf("fixture: %s must exist through the link: %v", symlinked, err)
	}

	// The want is developer's own real path: on macOS t.TempDir() is itself
	// behind /var -> /private/var, so `developer` as spelled is not yet what
	// codex will accept.
	want, err := filepath.EvalSymlinks(developer)
	if err != nil {
		t.Fatal(err)
	}
	r := realizeCodex(nil, nil, symlinked)
	if !strings.Contains(r.Deny, "--add-dir "+shellQuote(want)) {
		t.Errorf("the memory root must render resolved (%s), got: %s", want, r.Deny)
	}
	if strings.Contains(r.Deny, link) {
		t.Errorf("a root with a symlink component kills every command in the session; %s is still on the line: %s", link, r.Deny)
	}

	// Same for a root that arrives through `writable`, not just the memory
	// dir: the store of record and the git dirs are real paths on the box
	// this was measured on, which is the only reason they did not break too.
	w := realizeCodex(nil, nil, "", symlinked)
	if !strings.Contains(w.Deny, "--add-dir "+shellQuote(want)) || strings.Contains(w.Deny, link) {
		t.Errorf("a writable: root must resolve the same way, got: %s", w.Deny)
	}

	// And the resolved pair is ONE root, not two.
	if n := strings.Count(realizeCodex(nil, nil, symlinked, developer).Deny, "--add-dir"); n != 1 {
		t.Errorf("the link and its target are the same root; got %d --add-dir flags: %s", n, realizeCodex(nil, nil, symlinked, developer).Deny)
	}
}

// The wrong arm of the resolution: a root that does NOT EXIST YET still
// renders. Dropping it would be the silent cage of ranger-base-0fb — a codex
// session whose store of record is simply not writable and which reports
// nothing about it — and a root is routinely named before the launch
// materializes it (the memory dir, a store's git dirs). Two shapes:
//
//	a symlinked parent that exists  → the parent resolves, the tail rides along
//	nothing on the path exists      → the path renders as itself
func TestCodexRendersAWritableRootThatDoesNotExistYet(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "personas")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	unborn := filepath.Join(link, "developer") // the memory dir before EnsureMemoryDir
	if _, err := os.Stat(unborn); err == nil {
		t.Fatalf("fixture: %s must not exist yet", unborn)
	}
	want, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Join(want, "developer")
	if got := realizeCodex(nil, nil, unborn).Deny; !strings.Contains(got, "--add-dir "+shellQuote(want)) {
		t.Errorf("a root under a symlinked parent that does not exist yet must resolve to %s, got: %s", want, got)
	}

	// Nothing on this path exists, so there is nothing to resolve and the
	// root rides as typed — never dropped.
	gone := "/no-such-root-ranger-base-c02a/.beads"
	if _, err := os.Stat(gone); err == nil {
		t.Fatalf("fixture: %s must not exist", gone)
	}
	if got := realizeCodex(nil, nil, "", gone).Deny; !strings.Contains(got, "--add-dir "+shellQuote(gone)) {
		t.Errorf("an unresolvable root renders as itself, never dropped: %s missing from %s", gone, got)
	}
}

// The line, not the function (ranger-base-0fb's lesson): the box that was
// broken had its personas dir — the parent of every persona's memory dir —
// replaced by a symlink, so pin the launch line rendered over exactly that
// shape.
func TestCodexLaunchLineResolvesASymlinkedPersonasDir(t *testing.T) {
	b, fake := newTestBackend(t)
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nruntime: codex\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	// ~/.config/posse/personas -> ~/src/ranger-base/rhq/personas, the shape
	// the constitution tree made on 2026-08-28.
	constitution := filepath.Join(t.TempDir(), "rhq", "personas")
	if err := os.MkdirAll(constitution, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(constitution, b.App.PersonasDir()); err != nil {
		t.Fatal(err)
	}

	work := wtRepo(t)
	mustCreate(t, b, NewSessionOpts{Name: "crew", Agent: "ranger", Dir: work, Worktree: true})

	body, _ := os.ReadFile(b.App.LaunchScript("crew"))
	log := calls(t, fake) + "\n" + string(body)
	if !strings.Contains(log, "-s workspace-write") {
		t.Fatalf("not a codex sandbox line, so the assertions below mean nothing:\n%s", log)
	}
	mem := filepath.Join(constitution, "ranger")
	if _, err := os.Stat(mem); err != nil {
		t.Fatalf("the launch did not materialize the memory dir through the link: %v", err)
	}
	real, err := filepath.EvalSymlinks(mem)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log, "--add-dir "+shellQuote(real)) {
		t.Errorf("the launch line must name the RESOLVED memory dir %s:\n%s", real, log)
	}
	if strings.Contains(log, "--add-dir "+shellQuote(filepath.Join(b.App.PersonasDir(), "ranger"))) {
		t.Errorf("the line still carries the symlinked memory dir — every command in that session dies:\n%s", log)
	}
}

// The four tests above pin realizeCodex in ISOLATION, which is not where the
// bug was. The bug was one argument at the launch site: drop beadsHome(dir)
// from planLaunch's RenderCommandFor call and every test above stays green
// while dispatched codex sessions go silent again — and it took five of them
// before anyone read the silence as a cage rather than an agent skipping its
// bookkeeping (ranger-base-0fb). So pin the LINE, in the shape dispatch
// really launches it: a session worktree (rangerhq-09o2) of a repo whose
// .beads is an ADR 0012 D3-C redirect.
func TestCodexLaunchLineNamesTheStoreOfRecord(t *testing.T) {
	b, fake := newTestBackend(t) // its own temp $HOME
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nruntime: codex\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}

	store := blRepo(t)
	work := wtRepo(t)
	target := filepath.Join(store, beadsDirName)
	if err := os.MkdirAll(filepath.Join(work, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	blRedirect(t, work, target)

	// No AllowDegraded, and that is half the pin (ranger-base-xqwr): ADR
	// 0013 §4's reachability row judges this very line, and until the
	// store's git dirs rode it the row was unrealized and dispatch — which
	// never allows degradation on its own (ADR 0002 §4) — could not launch
	// codex in the fleet's own shape at all. A create that returns is the
	// row realized.
	mustCreate(t, b, NewSessionOpts{Name: "crew", Agent: "ranger", Dir: work, Worktree: true})

	tree, err := b.App.SessionTreePath(work, "crew")
	if err != nil {
		t.Fatal(err)
	}
	gits := LinkedGitDirs(tree)
	if len(gits) != 2 {
		t.Fatalf("no session worktree was made — the test asserts nothing: LinkedGitDirs(%s) = %v", tree, gits)
	}
	if h := beadsHome(tree); h != target {
		t.Fatalf("the seeded worktree redirect does not reach the store: beadsHome = %q, want %q", h, target)
	}

	// The STORE's git dirs are the third grant, and the one this bead is
	// about: `bd sync` COMMITS the JSONL in that repo, so it takes
	// index.lock there and reads hooks and refs beside it. With .beads
	// granted and these not, the session records nothing for the same
	// reason the pre-23c4e54 seatbelt recorded nothing (ranger-base-rhw).
	storeGits := beadsGitDirs(target)
	if len(storeGits) == 0 {
		t.Fatalf("no git dirs resolved for the store at %s — the assertion below would be empty", target)
	}

	// A line this long spills to the launch script, so the typed line is
	// the call log and that script together (paneline.go).
	body, _ := os.ReadFile(b.App.LaunchScript("crew"))
	log := calls(t, fake) + "\n" + string(body)
	// The store of record, the git dirs that repo's `bd sync` commit locks,
	// and the git dirs that hold this tree's index and the repo's objects:
	// all outside the workspace, all denied unless named.
	// Through codexWritableRoot, because that is what the line carries since
	// ranger-base-c02a: on macOS every one of these fixtures sits under
	// t.TempDir(), i.e. behind the /var -> /private/var symlink, and codex
	// refuses a root with a symlink component at command-run time. The
	// resolution itself is pinned by TestCodexResolvesASymlinkedWritableRoot
	// and the launch-line pin below it — on a box where t.TempDir() has no
	// symlink component this wrapper is the identity.
	for _, want := range append(append([]string{target}, storeGits...), gits...) {
		if !strings.Contains(log, "--add-dir "+shellQuote(codexWritableRoot(want))) {
			t.Errorf("codex launch does not name %s writable:\n%s", want, log)
		}
	}
}

// resolvedTypedLine is what a typed pane line actually runs: the line
// itself, or the body of the script a spilled line sources (paneline.go).
// Reading ONE line matters here — see the relaunch pin below.
func resolvedTypedLine(t *testing.T, ln string) string {
	t.Helper()
	rest, ok := strings.CutPrefix(ln, ". '")
	if !ok {
		return ln
	}
	path, _, ok := strings.Cut(rest, "'")
	if !ok {
		t.Fatalf("a sourced pane line with no script path: %q", ln)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spilled launch script %s: %v", path, err)
	}
	return string(body)
}

// The OTHER site that renders a persona line. RelaunchAgent re-types the
// whole command into the surviving shell of a session whose CLI died, and it
// rendered that line with no writable roots at all — so a codex session
// revived on the unattended path (dispatch.launchSession) came back in the
// exact ranger-base-0fb shape: silent beads, a denied `bd close`, and denied
// commits in its own worktree (ranger-base-qdtw). ADR 0013 §4's reachability
// row does not catch it: that row runs at CheckParity time against the
// LAUNCH line, and a relaunch renders its own.
//
// The pin reads the RELAUNCH's line alone, not the call log. The launch's
// line is in that log naming every root, and the spilled script is rewritten
// per launch — an assertion over either whole would have been green against
// the bug it is here to catch.
func TestCodexRelaunchLineNamesTheStoreOfRecord(t *testing.T) {
	b, fake := newTestBackend(t)
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nruntime: codex\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}

	store := blRepo(t)
	work := wtRepo(t)
	target := filepath.Join(store, beadsDirName)
	if err := os.MkdirAll(filepath.Join(work, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	blRedirect(t, work, target)
	mustCreate(t, b, NewSessionOpts{Name: "crew", Agent: "ranger", Dir: work, Worktree: true})

	tree, err := b.App.SessionTreePath(work, "crew")
	if err != nil {
		t.Fatal(err)
	}
	gits := LinkedGitDirs(tree)
	if len(gits) != 2 {
		t.Fatalf("no session worktree was made — the test asserts nothing: LinkedGitDirs(%s) = %v", tree, gits)
	}
	if h := beadsHome(tree); h != target {
		t.Fatalf("the seeded worktree redirect does not reach the store: beadsHome = %q, want %q", h, target)
	}
	storeGits := beadsGitDirs(target)
	if len(storeGits) == 0 {
		t.Fatalf("no git dirs resolved for the store at %s — the assertion below would be empty", target)
	}

	// The state RelaunchAgent needs: the launch is old enough to be past the
	// grace window, and herdr detects no agent in the workspace any more —
	// a persona whose CLI has gone.
	m, ok := b.readMeta("crew")
	if !ok {
		t.Fatal("no meta for crew")
	}
	if m.Dir != tree {
		t.Fatalf("meta dir is %q, not the session tree %q — the relaunch would resolve roots for the wrong dir", m.Dir, tree)
	}
	m.Launched = m.Launched.Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(fake, "agents.json"))
	if ok, err := b.RelaunchAgent("crew", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}

	lines := paneRunLines(t, fake)
	if len(lines) != 2 {
		t.Fatalf("expected the launch and the relaunch, got %d: %v", len(lines), lines)
	}
	line := resolvedTypedLine(t, lines[1])
	// A witness that we are reading a codex sandbox line at all, so a
	// mis-resolved or empty read fails as itself rather than as this bug.
	if !strings.Contains(line, "-s workspace-write") {
		t.Fatalf("the relaunch line is not a codex sandbox line:\n%s", line)
	}
	for _, want := range append(append([]string{target}, storeGits...), gits...) {
		if !strings.Contains(line, "--add-dir "+shellQuote(codexWritableRoot(want))) {
			t.Errorf("the relaunch line does not name %s writable:\n%s", want, line)
		}
	}
}

// launchWritableRoots grants the STORE repo's git dirs, and it grants them
// ONLY when the redirect moves the store out of the session dir. That
// condition is not decoration. With no redirect, beadsGitDirs' first entry is
// <dir>/.git, which in a linked worktree is a FILE, not a directory — and
// --add-dir is directory-granular, so putting it on the line widens the codex
// cage past the trade ADR 0013 §4 states, over a path that is not even a root.
//
// MEASURED while verifying ranger-base-xqwr's close (ranger-base-ecdp):
// deleting `!underDir(dir, home)` from launchWritableRoots left the whole
// internal/rhq package green. Both of the bead's own pins judge the redirect
// shape, so both keep passing over the over-grant — an inverted arm nothing
// held. This is that arm.
func TestLaunchWritableRootsGrantsTheStoreGitDirsOnlyWhenTheStoreMovedOut(t *testing.T) {
	repo := wtRepo(t)
	tree := filepath.Join(t.TempDir(), "wt")
	mustGit(t, repo, "worktree", "add", "-q", "-b", "sess", tree)
	if got := LinkedGitDirs(tree); len(got) != 2 {
		t.Fatalf("the fixture is not a linked worktree, so neither arm below means anything: LinkedGitDirs(%s) = %v", tree, got)
	}
	own := filepath.Join(tree, ".git")
	if st, err := os.Stat(own); err != nil || st.IsDir() {
		t.Fatalf("a linked worktree's .git must be the FILE this test is about: %v", err)
	}

	// NO REDIRECT: the store is <tree>/.beads, inside the workspace codex
	// writes anyway, so nothing extra is granted for it.
	if err := os.MkdirAll(filepath.Join(tree, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	// The absence below is about a path the resolver really would have
	// produced — without this witness the arm passes by measuring nothing
	// (ranger-base-fm4p).
	if g := beadsGitDirs(beadsHome(tree)); len(g) == 0 || g[0] != own {
		t.Fatalf("positive witness: beadsGitDirs of a store inside the tree must resolve %s first, got %v", own, g)
	}
	plain := launchWritableRoots(tree)
	for _, r := range plain {
		if r == own {
			t.Errorf("no redirect moved the store out, so the launch line must not name the worktree's .git FILE writable: %v", plain)
		}
	}
	if !containsString(plain, beadsHome(tree)) {
		t.Errorf("the store of record is granted in every shape (ranger-base-0fb): %v", plain)
	}

	// REDIRECT: the store moves to another repo, and now its git dirs ride
	// the line — the grant this test's negative arm must not be read as
	// denying (ranger-base-xqwr).
	store := blRepo(t)
	blRedirect(t, tree, filepath.Join(store, beadsDirName))
	if h := beadsHome(tree); h != filepath.Join(store, beadsDirName) {
		t.Fatalf("the redirect does not reach the store: beadsHome = %q", h)
	}
	moved := launchWritableRoots(tree)
	sg := beadsGitDirs(beadsHome(tree))
	if len(sg) == 0 {
		t.Fatal("no git dirs resolved for the moved store — the assertion below would be empty")
	}
	for _, g := range sg {
		if !containsString(moved, g) {
			t.Errorf("a store outside the session dir must carry its git dirs (bd sync commits the JSONL there): %s missing from %v", g, moved)
		}
	}
	if containsString(moved, own) {
		t.Errorf("the worktree's own .git FILE is never a root, redirect or not: %v", moved)
	}
}
