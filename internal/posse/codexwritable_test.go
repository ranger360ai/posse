//go:build !posse_arm2 && !posse_arm3

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
	t.Parallel()
	r := realizeCodex(nil, nil, "/m/personas/developer", "/elsewhere/ranger-base/.beads")
	want := "-s workspace-write --add-dir '/m/personas/developer' --add-dir '/elsewhere/ranger-base/.beads'"
	if r.Deny != want {
		t.Fatalf("codex sandbox flags:\n got %q\nwant %q", r.Deny, want)
	}
}

// read-only never takes --add-dir: codex exits on it (rangerhq-5oi).
func TestCodexReadOnlyStillRefusesAddDir(t *testing.T) {
	t.Parallel()
	r := realizeCodex(nil, []string{"Edit", "Write"}, "/m", "/elsewhere/.beads")
	if r.Deny != "-s read-only" {
		t.Fatalf("read-only must carry no --add-dir, got %q", r.Deny)
	}
}

// The same path named twice must not produce two flags.
func TestCodexWritableDedupes(t *testing.T) {
	t.Parallel()
	r := realizeCodex(nil, nil, "/same", "/same")
	if r.Deny != "-s workspace-write --add-dir '/same'" {
		t.Fatalf("dedupe failed: %q", r.Deny)
	}
}

// Runtimes posse cages itself must be untouched by the new parameter.
func TestClaudeAndGrokIgnoreWritable(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	b, fake := newTestBackend(t)
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nruntime: codex\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	// ~/.config/posse/personas -> $CONSTITUTION/<dir>/personas, the shape
	// the constitution tree made on 2026-08-28.
	constitution := filepath.Join(t.TempDir(), ConstitutionSourceDir, "personas")
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
	t.Parallel()
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

	// THE STORE'S GIT DIRS WERE THE THIRD GRANT AND ARE NOT NAMED ANY MORE
	// (ranger-base-kfrpv). They rode this line because "`bd sync` COMMITS
	// the JSONL in that repo" (ranger-base-rhw, ranger-base-xqwr) — and
	// --add-dir is directory-granular, so naming them handed a
	// self-caging runtime a second repo's whole history, refs, hooks and
	// config. bd does not commit there: the launcher does, uncaged (ADR
	// 0015 §4). The negative arm below is what pins it, and
	// `beadsGitDirs(target)` is still asked so an arm over an empty answer
	// fails rather than passes.
	storeGits := beadsGitDirs(target)
	if len(storeGits) == 0 {
		t.Fatalf("no git dirs resolved for the store at %s — the negative assertion below would be empty", target)
	}

	// A line this long spills to the launch script, so the typed line is
	// the call log and that script together (paneline.go).
	body, _ := os.ReadFile(b.App.LaunchScript("crew"))
	log := calls(t, fake) + "\n" + string(body)
	// The store of record, and the git dirs that hold this tree's index and
	// the repo's objects: outside the workspace, denied unless named. The
	// STORE repo's git dirs are the one thing that must NOT be named.
	// Through codexWritableRoot, because that is what the line carries since
	// ranger-base-c02a: on macOS every one of these fixtures sits under
	// t.TempDir(), i.e. behind the /var -> /private/var symlink, and codex
	// refuses a root with a symlink component at command-run time. The
	// resolution itself is pinned by TestCodexResolvesASymlinkedWritableRoot
	// and the launch-line pin below it — on a box where t.TempDir() has no
	// symlink component this wrapper is the identity.
	for _, want := range append([]string{target}, gits...) {
		if !strings.Contains(log, "--add-dir "+shellQuote(codexWritableRoot(want))) {
			t.Errorf("codex launch does not name %s writable:\n%s", want, log)
		}
	}
	for _, deny := range storeGits {
		if strings.Contains(log, "--add-dir "+shellQuote(codexWritableRoot(deny))) {
			t.Errorf("codex launch names the STORE repo's git dir %s writable — nothing in a seat commits there, and --add-dir cannot name less than the directory (ranger-base-kfrpv):\n%s", deny, log)
		}
	}
}

// resolvedTypedLine, which used to live here, is in herdr_test.go now,
// beside the other readers of the fake herdr's log (ranger-base-zb476). It
// was moved rather than copied for the same reason paneRunLines was: this
// file is arm 1 only, and the spill resolver is what makes a relaunch pin in
// any arm readable. Reading ONE line is still what matters here — see the
// relaunch pin below.

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
	t.Parallel()
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
		t.Fatalf("no git dirs resolved for the store at %s — the negative assertion below would be empty", target)
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
	for _, want := range append([]string{target}, gits...) {
		if !strings.Contains(line, "--add-dir "+shellQuote(codexWritableRoot(want))) {
			t.Errorf("the relaunch line does not name %s writable:\n%s", want, line)
		}
	}
	// And the STORE repo's git dirs on neither line (ranger-base-kfrpv):
	// the relaunch renders its own roots, so it is its own chance to put
	// back a grant the launch no longer makes.
	for _, deny := range storeGits {
		if strings.Contains(line, "--add-dir "+shellQuote(codexWritableRoot(deny))) {
			t.Errorf("the relaunch line names the STORE repo's git dir %s writable (ranger-base-kfrpv):\n%s", deny, line)
		}
	}
}

// launchWritableRoots NEVER grants a git dir of the store's repo — with a
// redirect or without one (ranger-base-kfrpv). It granted them when the
// redirect moved the store out of the session dir, on the reason that `bd
// sync` committed the JSONL there; nothing in a seat commits there and the
// launcher does, uncaged (ADR 0015 §4), so the grant is gone and this is
// the arm that keeps it gone. --add-dir is directory-granular, so what it
// handed over was a second repo's whole history, refs, hooks and config.
//
// THE NO-REDIRECT HALF IS THE OLDER PIN AND IT STAYS. With the store inside
// the tree, beadsGitDirs' first entry is <dir>/.git, which in a linked
// worktree is a FILE, not a directory, so the line used to carry a root
// that is not even a root. MEASURED while verifying ranger-base-xqwr's
// close (ranger-base-ecdp): deleting `!underDir(dir, home)` from
// launchWritableRoots left the whole internal/rhq package green, because
// both of that bead's own pins judged the redirect shape. The condition is
// gone now — the whole call is — and both halves of this arm are the
// regression guard over the day someone writes it back.
func TestLaunchWritableRootsNeverGrantsTheStoreRepoGitDirs(t *testing.T) {
	t.Parallel()
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

	// REDIRECT: the store moves to another repo, and its git dirs still do
	// not ride the line (ranger-base-kfrpv). The store DIRECTORY does, in
	// every shape, which is ranger-base-0fb's own property and the thing a
	// seat really writes.
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
		if containsString(moved, g) {
			t.Errorf("a store outside the session dir must carry NONE of its repo's git dirs — nothing in a seat commits there, and --add-dir cannot name less than the directory (ranger-base-kfrpv): %s in %v", g, moved)
		}
	}
	if !containsString(moved, beadsHome(tree)) {
		t.Errorf("the store of record is granted in every shape (ranger-base-0fb): %v", moved)
	}
	if containsString(moved, own) {
		t.Errorf("the worktree's own .git FILE is never a root, redirect or not: %v", moved)
	}
}

// ─── the residual (ranger-base-k62e) ─────────────────────────────────────────

// codexWritableRoot resolves the longest EXISTING prefix and re-joins the
// rest, so a component EvalSymlinks cannot resolve — a DANGLING link, a loop
// — is walked past and re-joined verbatim, and the rendered root still
// carries the symlink component codex refuses. Rendering it is the right
// CONTENT decision (dropping it is ranger-base-0fb's silent cage, pinned by
// TestCodexRendersAWritableRootThatDoesNotExistYet just above); this is the
// VOLUME the bead is about, so both halves are pinned here: the root still
// renders, AND the line is now readable as one codex will refuse.
//
// MEASURED 2026-09-05, codex-cli 0.150.1: a dangling root is refused exactly
// like the resolvable symlink ranger-base-c02a measured, as the final
// component or not, and one bad root refuses the WHOLE set (exit 1) before
// any sandbox is applied.
func TestCodexRefusesADanglingSymlinkWritableRoot(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	dangling := filepath.Join(d, "dangle")
	if err := os.Symlink(filepath.Join(d, "gone"), dangling); err != nil {
		t.Fatal(err)
	}
	// The fixture is the shape the bug needs: a symlink that EXISTS as a link
	// and resolves to nothing. Without this witness both arms below could
	// pass over a path that was never a symlink at all.
	if st, err := os.Lstat(dangling); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("fixture: %s must be a symlink: %v", dangling, err)
	}
	if _, err := os.Stat(dangling); err == nil {
		t.Fatalf("fixture: %s must dangle", dangling)
	}

	// What the line carries is the root AFTER codexWritableRoot, and on
	// macOS that is not the spelling above: t.TempDir() is itself behind
	// /var -> /private/var, which resolves. The dangling link is what
	// survives it, and that survival is the bug.
	rendered := codexWritableRoot(dangling)
	if st, err := os.Lstat(rendered); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("fixture: the rendered root %s must still BE the symlink, or there is no residual to pin: %v", rendered, err)
	}

	// Half one: the root still renders, unresolved and undropped.
	line := realizeCodex(nil, nil, "", dangling).Deny
	if !strings.Contains(line, "--add-dir "+shellQuote(rendered)) {
		t.Fatalf("the fallback must render the root as typed, never drop it: %s", line)
	}
	// Half two: and the line is readable as one codex refuses, naming the
	// component codex's own error names.
	root, comp := refusedWritableRoot(line)
	if root != rendered || comp != rendered {
		t.Errorf("refusedWritableRoot(%s) = %q, %q; want %q, %q", line, root, comp, rendered, rendered)
	}

	// A root UNDER the dangling link: codex names the link, not the root
	// (measured, both arms of the same run).
	under := codexWritableRoot(filepath.Join(dangling, "sub"))
	root, comp = refusedWritableRoot(realizeCodex(nil, nil, "", under).Deny)
	if root != under || comp != rendered {
		t.Errorf("a root under a dangling link: got %q, %q; want %q, %q", root, comp, under, rendered)
	}

	// A LOOP is the same class and not a hypothetical variant: EvalSymlinks
	// fails, Lstat says symlink, resolveExisting walks past it.
	a, b := filepath.Join(d, "a"), filepath.Join(d, "b")
	if err := os.Symlink(b, a); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, b); err != nil {
		t.Fatal(err)
	}
	if _, err := filepath.EvalSymlinks(a); err == nil {
		t.Fatalf("fixture: %s must not resolve", a)
	}
	loop := codexWritableRoot(a)
	if root, comp = refusedWritableRoot(realizeCodex(nil, nil, "", a).Deny); root != loop || comp != loop {
		t.Errorf("a symlink loop: got %q, %q; want %q, %q", root, comp, loop, loop)
	}

	// TWO symlink components, which only a line posse did not render can
	// carry (a rendered root has exactly one — everything above the failing
	// link is replaced by its real path, and nothing below it exists). codex
	// names the DEEPEST, measured 2026-09-05 on codex-cli 0.150.1 over
	// l1/sub -> …/l1, l1/l2 -> …/l1/l2 and l1/d -> …/l1/d, so this names the
	// same file codex will.
	l1 := filepath.Join(d, "l1")
	if err := os.Symlink(d, l1); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(l1, "dangle")
	for _, c := range []string{l1, deep} {
		if st, err := os.Lstat(c); err != nil || st.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("fixture: %s must be a symlink, or the two-component arm is a one-component arm: %v", c, err)
		}
	}
	if root, comp = refusedWritableRoot("codex -s workspace-write --add-dir " + shellQuote(deep)); root != deep || comp != deep {
		t.Errorf("two symlink components: got %q, %q; want %q, %q (codex names the deepest)", root, comp, deep, deep)
	}
}

// The control, and it is c02a's own shape: a root whose symlink RESOLVES is
// rendered real by codexWritableRoot, so there is nothing left on the line
// to refuse. Without this arm the check above passes just as well when it
// refuses every codex launch on the box — which is the failure mode of a
// refusal, and the one that would cost more than the bug.
func TestCodexAcceptsTheRootsItResolves(t *testing.T) {
	t.Parallel()
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
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("fixture: %s must be a symlink, or this control controls nothing: %v", link, err)
	}

	// The memory dir through the link, a writable: root through it, and the
	// resolved path itself — the three shapes realizeCodex renders.
	for _, line := range []string{
		realizeCodex(nil, nil, symlinked).Deny,
		realizeCodex(nil, nil, "", symlinked).Deny,
		realizeCodex(nil, nil, "", developer).Deny,
	} {
		if root, comp := refusedWritableRoot(line); root != "" {
			t.Errorf("a resolvable root must not be refused: %s named %q (component %q)", line, root, comp)
		}
	}

	// A root that does not exist AT ALL is not refused either: nothing on it
	// is a symlink, and codex is given it for the same reason posse renders
	// it — the launch materializes it later (the memory dir, a store's git
	// dirs).
	if root, _ := refusedWritableRoot(realizeCodex(nil, nil, "", "/no-such-root-ranger-base-k62e/.beads").Deny); root != "" {
		t.Errorf("an unborn root has no symlink component to refuse, got %q", root)
	}
	// And -s read-only carries no roots at all, so there is nothing to judge.
	if root, _ := refusedWritableRoot(realizeCodex(nil, []string{"Edit", "Write"}, symlinked).Deny); root != "" {
		t.Errorf("read-only names no writable root, got %q", root)
	}
}

// The refusal is the RUNTIME's, not the flag's: claude takes --add-dir too
// and does not care what the path is made of, so a claude line over the same
// root must launch. Drop the SelfSandbox gate and this is the test that
// notices — the alternative is refusing every claude session on a box with
// one dangling link anywhere near its memory dir.
func TestOnlyASelfSandboxingRuntimeRefusesASymlinkedRoot(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	dangling := filepath.Join(d, "dangle")
	if err := os.Symlink(filepath.Join(d, "gone"), dangling); err != nil {
		t.Fatal(err)
	}
	line := "codex -s workspace-write --add-dir " + shellQuote(dangling)
	codex := &Runtime{Name: "codex", SelfSandbox: true}
	err := writableRootRefusal("ranger", codex, line)
	if err == nil {
		t.Fatal("a self-sandboxing runtime must refuse a root it cannot use")
	}
	// The message has to carry the two paths an operator needs to act, and
	// the component is the one to repair.
	for _, want := range []string{AbbrevHome(dangling), "ranger", "codex"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q: %v", want, err)
		}
	}
	if e := writableRootRefusal("ranger", &Runtime{Name: "claude"}, line); e != nil {
		t.Errorf("claude does not refuse a symlinked --add-dir; refusing its launch is a bug, not a wall: %v", e)
	}
	if e := writableRootRefusal("ranger", nil, line); e != nil {
		t.Errorf("no runtime, no refusal: %v", e)
	}
}

// The LINE, in the shape dispatch really launches it — the lesson the four
// pins above this section were written under (ranger-base-0fb): a check that
// only ever sees realizeCodex's return value is green over a launch site
// that hands it a different root.
//
// The reachable shape, and it is not the memory dir: EnsureMemoryDir runs
// first and MkdirAll REFUSES a dangling component (measured 2026-09-05 —
// `mkdir …: file exists`, both for a dangling parent and a dangling final
// component), so that route is already loud, if obscurely.
//
// THE FIXTURE IS THE SUITE SLOT DIR, and it used to be the STORE repo's
// `.git` (ranger-base-kfrpv). That root is not on the line any more — no
// tier grants the store repo's git — and of what launchWritableRoots still
// names, the store directory and this tree's git dirs must all Stat as
// directories before they are named at all, so the slot dir is the one root
// that legitimately NEED NOT EXIST: `suiteLockRoot` answers
// `POSSE_SUITE_LOCK_DIR` (else `~/.cache/posse`) for any tree carrying
// scripts/suite-lock.sh, materialized by nothing, so a dangling component
// there rides onto the line untouched. An absent root is rendered on
// purpose (dropping one is ranger-base-0fb's silent cage); a root whose
// first missing component is a LINK is the one codex refuses.
//
// The slot dir's own row never degrades a launch (suitelock.go's header),
// so what refuses below is the writable-root check and the record-reach
// row, which is this test's subject and not the queue's.
func TestCodexLaunchRefusesAWritableRootCodexWouldRefuse(t *testing.T) {
	b, fake := newTestBackend(t)
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nruntime: codex\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}

	store := t.TempDir()
	target := filepath.Join(store, beadsDirName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	// The slot dir, behind a link to nothing. POSSE_SUITE_LOCK_DIR is
	// suiteLockRoot's own override, so this is the production resolver
	// answering a path the box really could be pointed at.
	slotHome := t.TempDir()
	dangling := filepath.Join(slotHome, "dangle")
	if err := os.Symlink(filepath.Join(slotHome, "moved-away"), dangling); err != nil {
		t.Fatal(err)
	}
	slots := filepath.Join(dangling, "slots")
	t.Setenv("POSSE_SUITE_LOCK_DIR", slots)

	work := wtRepo(t)
	// suiteLockRoot answers only for a tree that carries the queue script,
	// and the session runs in a WORKTREE of this repo — so the script has
	// to be committed, not just written.
	write(t, filepath.Join(work, suiteLockScriptRel), "#!/bin/sh\nexit 0\n")
	mustGit(t, work, "add", suiteLockScriptRel)
	mustGit(t, work, "commit", "-q", "-m", "queue script")
	if err := os.MkdirAll(filepath.Join(work, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	blRedirect(t, work, target)

	// The fixture really puts a dangling link on the line: the redirect
	// reaches the store, and the slot dir launchWritableRoots names has a
	// component that resolves to nothing.
	if h := beadsHome(work); h != target {
		t.Fatalf("the redirect does not reach the store: beadsHome = %q, want %q", h, target)
	}
	if !containsString(launchWritableRoots(work), slots) {
		t.Fatalf("%s is not among the launch's writable roots %v — the arms below would measure nothing", slots, launchWritableRoots(work))
	}
	if st, err := os.Lstat(dangling); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("fixture: %s must be a symlink: %v", dangling, err)
	}
	// …and it must be the component codexWritableRoot cannot resolve, so
	// the root really arrives at the line with a link in it.
	if got := codexWritableRoot(slots); got != slots {
		t.Fatalf("fixture: %s resolves to %s, so no symlink component reaches the line", slots, got)
	}
	// The store repo's git dir is on NO line any more, which is why it is
	// not this fixture's dangling root (ranger-base-kfrpv).
	if containsString(launchWritableRoots(work), filepath.Join(store, ".git")) {
		t.Fatalf("the store repo's git dir is back on the launch line (ranger-base-kfrpv): %v", launchWritableRoots(work))
	}

	// Arm 1: the ordinary launch. It never opens — and the message names
	// the root and the component, which is the whole difference from
	// ranger-base-c02a, where codex named them at command-run time inside a
	// session nobody reads.
	err := b.CreateSession(NewSessionOpts{Name: "crew", Agent: "ranger", Dir: work, Worktree: true})
	if err == nil {
		t.Fatal("a codex launch whose line names a root codex refuses must not happen: every command in that session fails")
	}
	if !strings.Contains(err.Error(), AbbrevHome(slots)) {
		t.Errorf("the refusal must name the root: %v", err)
	}
	if !strings.Contains(err.Error(), "unreachable along with everything else") {
		t.Errorf("the reachability row is what refuses an ordinary launch, and it must say the store is unreachable: %v", err)
	}

	// Arm 2: --allow-degraded waives the reachability row — every other gate
	// it waives is a wall posse could not realize, and this one is a session
	// that runs nothing. So the refusal has to hold past the waiver, which
	// is why it is a launch refusal (PIDVoided's shape) and not a row.
	err = b.CreateSession(NewSessionOpts{Name: "crew2", Agent: "ranger", Dir: work, Worktree: true, AllowDegraded: true})
	if err == nil {
		t.Fatal("--allow-degraded must not buy a session in which no command can run")
	}
	if !strings.Contains(err.Error(), "fail every single command it runs") {
		t.Errorf("past the waiver the launch refusal is what must speak: %v", err)
	}

	// The control, same fixture with the link repaired: the root resolves,
	// the launch happens, and the line names the REAL slot dir. Without it
	// both arms above pass just as well over a check that refuses every
	// codex launch there is.
	if err := os.MkdirAll(filepath.Join(slotHome, "moved-away"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustCreate(t, b, NewSessionOpts{Name: "crew3", Agent: "ranger", Dir: work, Worktree: true})
	body, _ := os.ReadFile(b.App.LaunchScript("crew3"))
	log := calls(t, fake) + "\n" + string(body)
	if !strings.Contains(log, "-s workspace-write") {
		t.Fatalf("not a codex sandbox line, so the assertion below means nothing:\n%s", log)
	}
	if !strings.Contains(log, "--add-dir "+shellQuote(codexWritableRoot(slots))) {
		t.Errorf("the repaired root must ride resolved (%s):\n%s", codexWritableRoot(slots), log)
	}
}
