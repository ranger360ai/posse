package posse

// ranger-base-m2wf (from security's posture check, ranger-base-sipu): the L2
// grant for a session worktree named the COMMON git dir — the operator's
// main checkout's `.git`, shared with every other worktree on that repo —
// and named it whole. Measured writable from a live caged session:
// `refs/heads/main`, `hooks/pre-push`, `config`, `packed-refs`.
//
// Two capabilities the tier is supposed to wall came with it. Moving a ref
// is not `git push`, so L1's shim never sees it and L3's pre-push never
// fires; a persona could move the operator's main in the repo it was
// dispatched INTO, around the launcher's ff-merge of its session branch.
// And `hooks/` is shared, so overwriting a slot disarms L3 for the repo and
// every other session on it, persistently.
//
// So this file pins the narrowing, and pins it the way its neighbour
// seatbeltcarveout_qa_test.go pins the carve-out: by EXECUTION under
// sandbox-exec, with the grant AS IT WAS — the two dirs whole — as the
// control. A refusal proves nothing unless the same command succeeds with
// the narrowing removed, and the control's success is also the witness that
// the file it wrote to was really there.
//
// Neither arm carries the carve-out. The carve-out already denies the
// common hooks dir (sessionHooksDirs), so a probe run under it would be
// refused by BOTH arms and would measure nothing about this change: what is
// varied here is the GRANT, and only the grant.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the fixture: a repo, the session's worktree, and a sibling's ────────────

type wgFixture struct {
	a             *App
	repo, tree    string // the operator's checkout, and the session's tree (cwd)
	own, common   string // the per-worktree git dir, and the shared one
	other         string // another session's per-worktree git dir
	gates, branch string
	ag            *AgentFile
	head          string // the tree's HEAD, a sha that is NOT main's
}

func wgNewFixture(t *testing.T) wgFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := sbRoot(t) // HOME elsewhere, TMPDIR a sibling: nothing here is granted by accident
	repo := sbMkdir(t, filepath.Join(root, "repo"))
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	sbWrite(t, filepath.Join(repo, "README.md"), "seed\n")
	mustGit(t, repo, "add", "README.md")
	mustGit(t, repo, "commit", "-q", "-m", "seed")
	mustGit(t, repo, "pack-refs", "--all") // packed-refs exists, as it does in a real checkout

	// The session's tree, on its own branch — the launcher's shape, slash
	// and all, because the branch ref is then a file two directories deep.
	branch := "posse/developer-2-probe"
	tree := filepath.Join(root, "trees", "developer-2")
	mustGit(t, repo, "worktree", "add", "-q", "-b", branch, tree)
	commitIn(t, tree, "work.txt", "session work\n", "session work")
	// A sibling session on the same repo: its index and HEAD live in the
	// common dir too, and are no business of this session's.
	mustGit(t, repo, "worktree", "add", "-q", "-b", "posse/other-probe", filepath.Join(root, "trees", "other"))

	dirs := LinkedGitDirs(tree)
	if len(dirs) != 2 {
		t.Fatalf("no linked worktree was made — the test asserts nothing: LinkedGitDirs(%s) = %v", tree, dirs)
	}
	a := NewAppAt(filepath.Join(root, "home"))
	homeWithConstitution(t, a, "")
	f := wgFixture{
		a: a, repo: repo, tree: tree,
		own: absResolve(dirs[0]), common: absResolve(dirs[1]),
		other:  absResolve(filepath.Join(dirs[1], "worktrees", "other")),
		gates:  sbMkdir(t, a.GatesDir("developer-2")),
		branch: branch,
		head:   mustGit(t, tree, "rev-parse", "HEAD"),
	}
	f.ag = &AgentFile{Name: "developer-2", MemoryDir: sbMkdir(t, filepath.Join(a.Home, "personas", "developer-2"))}
	if f.head == mustGit(t, repo, "rev-parse", "refs/heads/main") {
		t.Fatal("the session branch and main are the same commit — a ref move would not be observable")
	}
	return f
}

func (f wgFixture) writable(t *testing.T) []string {
	t.Helper()
	return f.a.SeatbeltWritable(f.ag, f.tree, f.gates)
}

// wide is the grant as it stood before this bead: everything else the same,
// and the two git dirs named WHOLE. It is the control, so it is built by
// substitution rather than by hand — a control assembled from a different
// list would be measuring two changes at once.
func (f wgFixture) wide(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, w := range f.writable(t) {
		if underDir(f.common, w) {
			continue // objects, logs, the ref pair — and the per-worktree dir, which is under it
		}
		out = append(out, w)
	}
	for _, d := range LinkedGitDirs(f.tree) {
		out = append(out, absResolve(d))
	}
	return out
}

func (f wgFixture) ref(name string) string { return filepath.Join(f.common, "refs", "heads", name) }

// lock is the one entry ADR 0059 D1 adds to the grant: the shared repo's
// packed-refs lock, writable so git can create AND unlink it.
func (f wgFixture) lock() string { return absResolve(filepath.Join(f.common, "packed-refs.lock")) }

// withoutLock is the writable set as it shipped BEFORE ADR 0059 — the same
// list with that one entry taken out. It is how the controls below vary the
// grant and nothing else, and it fatals when there is nothing to remove: a
// control identical to the arm measures nothing, and would go quiet exactly
// when the entry it exists to isolate was dropped.
func (f wgFixture) withoutLock(t *testing.T) []string {
	t.Helper()
	w := f.writable(t)
	var out []string
	for _, p := range w {
		if absResolve(p) == f.lock() {
			continue
		}
		out = append(out, p)
	}
	if len(out) == len(w) {
		t.Fatalf("%s is not in the writable set, so removing it varies nothing — the ADR 0059 grant is gone and every control below is the arm:\n  %s",
			f.lock(), strings.Join(w, "\n  "))
	}
	return out
}

// profile renders a seatbelt for this fixture from a writable set the
// caller chose. RenderSeatbelt is the shipped path and is what the arms
// use; this is for the CONTROL, which needs the same profile minus one
// entry, assembled the way RenderSeatbelt assembles it rather than by hand.
func (f wgFixture) profile(t *testing.T, name string, w []string) string {
	t.Helper()
	carve := f.a.SeatbeltCarveOut(f.ag, f.tree, f.gates, w)
	return sbRenderProfile(t, name, SeatbeltProfile(f.ag.Name, w, nil, carve, sessionRefDirs(f.tree)...))
}

// ─── the writable set ────────────────────────────────────────────────────────

// What the narrowed grant names, and what it stops naming. The list is the
// answer to "what does a commit in a linked worktree write outside its own
// tree", and everything else in the common dir is shared state.
func TestQAWorktreeGrantNamesObjectsLogsAndItsOwnRefOnly(t *testing.T) {
	f := wgNewFixture(t)
	w := f.writable(t)
	for _, p := range []string{
		f.own,                              // its index, HEAD, COMMIT_EDITMSG, locks
		filepath.Join(f.common, "objects"), // the commit it writes
		filepath.Join(f.common, "logs"),    // the reflog of the ref it moves
		f.ref(f.branch),                    // its own branch
		f.ref(f.branch) + ".lock",          // and the lock git renames onto it
		f.lock(),                           // the packed backend's lock (ADR 0059 D1)
		filepath.Join(f.tree, "work.txt"),  // its tree, unchanged by this bead
	} {
		if !sbCovers(w, p) {
			t.Errorf("a session cannot commit in its own tree: %s is in no grant:\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	for _, p := range []string{
		f.common,                         // never the dir itself: that is a rename of anything below it
		f.ref("main"),                    // the operator's branch — the whole point
		filepath.Join(f.common, "hooks"), // L3's shared slots
		filepath.Join(f.common, "hooks", "pre-push"),
		filepath.Join(f.common, "config"),
		filepath.Join(f.common, "packed-refs"),     // the lock above is granted; the file it guards is not
		filepath.Join(f.common, "packed-refs.new"), // nor the tempfile git writes the new content through
		filepath.Join(f.common, "refs", "heads"),   // the directory, or main comes back with it
		filepath.Join(f.common, "refs", "remotes"),
		f.other,                    // another session's index and HEAD
		f.ref("posse/other-probe"), // and its branch
		f.repo,                     // the operator's checkout, which is not this session's tree
	} {
		if sbCovers(w, p) {
			t.Errorf("the grant still reaches shared state: %s is writable:\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	// The control on the control: the OLD grant reached all of it, so the
	// list above is measuring the narrowing and not a fixture that never
	// had those paths. If this ever goes quiet, the test below stops
	// proving anything and this says so first.
	old := f.wide(t)
	for _, p := range []string{f.ref("main"), filepath.Join(f.common, "hooks", "pre-push"), f.other} {
		if !sbCovers(old, p) {
			t.Errorf("premise gone: the pre-m2wf grant did not reach %s either — nothing was narrowed", p)
		}
	}
}

// A main checkout is untouched: `.git` is inside cwd and already granted,
// LinkedGitDirs is empty, and this bead must not have narrowed a session
// that was never widened. Its refs/heads/main IS writable — that posture
// predates the worktree model and is not this bead's to change.
func TestQAMainCheckoutGrantIsUnchanged(t *testing.T) {
	f := wgNewFixture(t)
	if got := sessionGitGrants(f.repo); got != nil {
		t.Errorf("sessionGitGrants(main checkout) = %v, want none — .git is already inside cwd", got)
	}
	w := f.a.SeatbeltWritable(f.ag, f.repo, f.gates)
	if !sbCovers(w, filepath.Join(f.repo, ".git", "refs", "heads", "main")) {
		t.Errorf("a main-checkout dispatch lost its own .git:\n  %s", strings.Join(w, "\n  "))
	}
}

// A detached HEAD has no branch to name. It gets objects and logs and its
// own per-worktree dir, and no ref grant at all — and that is enough,
// because the HEAD a commit moves there is inside the per-worktree dir.
func TestQADetachedWorktreeGetsNoRefGrant(t *testing.T) {
	f := wgNewFixture(t)
	mustGit(t, f.tree, "checkout", "-q", "--detach")
	w := f.a.SeatbeltWritable(f.ag, f.tree, f.gates)
	for _, p := range []string{f.own, filepath.Join(f.common, "objects")} {
		if !sbCovers(w, p) {
			t.Errorf("a detached session cannot write objects: %s is in no grant", p)
		}
	}
	for _, p := range []string{f.ref(f.branch), f.ref("main"), filepath.Join(f.common, "refs")} {
		if sbCovers(w, p) {
			t.Errorf("a detached session has no branch to move; %s must not be granted", p)
		}
	}
}

// ─── the execution half ──────────────────────────────────────────────────────

type wgProbe struct {
	what    string
	sh      func(f wgFixture) string // /bin/sh -c, built against a FRESH fixture
	want    bool                     // true: must still be allowed under the narrowed grant
	witness func(t *testing.T, f wgFixture)
}

// wgRun measures one command under one profile. Unlike sbRun it does not
// treat a high exit code as "not the sandbox": git exits 128 on a refused
// write, which is exactly the measurement here, so the output is carried
// back and reported instead of guessed at.
func wgRun(t *testing.T, profile, sh string) (bool, string) {
	t.Helper()
	sbSkipUnlessSandboxable(t)
	out, err := exec.Command("sandbox-exec", "-f", profile, "/bin/sh", "-c", sh).CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}

// wgTry renders a profile for a FRESH fixture — narrowed or as it was — and
// runs the probe under it. Fresh per arm because the control arm is a real
// write: it moves refs and renames directories, and a probe that ran
// against a fixture an earlier probe mutated is the shape of a green suite
// over no wall at all (measured on ranger-base-h15).
func wgTry(t *testing.T, p wgProbe, narrowed bool) (bool, string) {
	t.Helper()
	f := wgNewFixture(t)
	w, name := f.wide(t), "control.sb"
	if narrowed {
		w, name = f.writable(t), "narrowed.sb"
	}
	prof := sbRenderProfile(t, name, SeatbeltProfile("developer-2", w, nil, SeatbeltCarveOut{}))
	ok, out := wgRun(t, prof, p.sh(f))
	if ok && p.witness != nil {
		p.witness(t, f)
	}
	return ok, out
}

func wgExists(t *testing.T, p string) bool {
	t.Helper()
	_, err := os.Stat(p)
	return err == nil
}

func TestQAWorktreeGrantRefusesSharedGitStateUnderSandboxExec(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	probes := []wgProbe{
		// The bead's first capability: a ref move is not a push. L1 shims
		// `git push`, L3 fires on pre-push, and neither is reached here.
		{"move the operator's main with git update-ref", func(f wgFixture) string {
			return "git -C " + f.tree + " update-ref refs/heads/main " + f.head
		}, false, func(t *testing.T, f wgFixture) {
			if got := mustGit(t, f.repo, "rev-parse", "refs/heads/main"); got != f.head {
				t.Errorf("the control did not actually move main (%s) — it witnesses nothing", got)
			}
		}},
		{"move the operator's main by writing the loose ref", func(f wgFixture) string {
			return "printf '%s\\n' " + f.head + " > " + f.ref("main")
		}, false, func(t *testing.T, f wgFixture) {
			if b, _ := os.ReadFile(f.ref("main")); strings.TrimSpace(string(b)) != f.head {
				t.Errorf("the control did not write the ref file: %q", b)
			}
		}},
		{"cut a branch beside its own", func(f wgFixture) string {
			return "git -C " + f.tree + " branch sneaky " + f.head
		}, false, func(t *testing.T, f wgFixture) {
			if !wgExists(t, f.ref("sneaky")) {
				t.Error("the control did not create the ref — it witnesses nothing")
			}
		}},
		// The second capability: hooks/ is SHARED, so this disarms L3 for
		// the operator's checkout and every other session on the repo.
		{"disarm L3 by overwriting the shared pre-push hook", func(f wgFixture) string {
			return "echo 'exit 0' > " + filepath.Join(f.common, "hooks", "pre-push")
		}, false, func(t *testing.T, f wgFixture) {
			if b, _ := os.ReadFile(filepath.Join(f.common, "hooks", "pre-push")); !strings.Contains(string(b), "exit 0") {
				t.Errorf("the control did not plant the hook: %q", b)
			}
		}},
		{"rewrite the repo's git config", func(f wgFixture) string {
			return "echo '[core]' >> " + filepath.Join(f.common, "config")
		}, false, func(t *testing.T, f wgFixture) {
			if b, _ := os.ReadFile(filepath.Join(f.common, "config")); !strings.Contains(string(b), "[core]") {
				t.Errorf("the control did not append to config: %q", b)
			}
		}},
		{"rewrite packed-refs", func(f wgFixture) string {
			return ": > " + filepath.Join(f.common, "packed-refs")
		}, false, func(t *testing.T, f wgFixture) {
			if b, _ := os.ReadFile(filepath.Join(f.common, "packed-refs")); len(b) != 0 {
				t.Errorf("the control did not truncate packed-refs: %q", b)
			}
		}},
		{"reach another session's per-worktree git dir", func(f wgFixture) string {
			return "touch " + filepath.Join(f.other, "PWNED")
		}, false, func(t *testing.T, f wgFixture) {
			if !wgExists(t, filepath.Join(f.other, "PWNED")) {
				t.Error("the control did not write the sibling's git dir — it witnesses nothing")
			}
		}},
		// The rename escape, from the other side: the narrowed grant names
		// paths INSIDE the common dir and not the dir, so nothing above
		// them can be moved out from under the default deny. No seal is
		// needed for it, and this is why.
		{"rename objects/ out from under the grant", func(f wgFixture) string {
			return "mv " + filepath.Join(f.common, "objects") + " " + filepath.Join(f.common, "objects2")
		}, false, func(t *testing.T, f wgFixture) {
			if !wgExists(t, filepath.Join(f.common, "objects2")) {
				t.Error("the control did not rename objects/ — it witnesses nothing")
			}
		}},

		// And what a session must keep. These are ALLOWED under the narrowed
		// grant too, so they are not controls — they are the cost check.
		{"write a loose object", func(f wgFixture) string {
			return "echo body | git -C " + f.tree + " hash-object -w --stdin"
		}, true, nil},
		{"move its OWN branch ref", func(f wgFixture) string {
			return "git -C " + f.tree + " update-ref refs/heads/" + f.branch + " " + f.head
		}, true, nil},
		{"write its own per-worktree git dir", func(f wgFixture) string {
			return "touch " + filepath.Join(f.own, "probe")
		}, true, nil},
		{"work in its own tree", func(f wgFixture) string {
			return "echo more >> " + filepath.Join(f.tree, "work.txt")
		}, true, nil},
	}
	verb := map[bool]string{true: "ALLOWED", false: "REFUSED"}
	for _, p := range probes {
		t.Run(p.what, func(t *testing.T) {
			got, out := wgTry(t, p, true)
			if got != p.want {
				t.Errorf("%s under the narrowed grant, want %s:\n%s", verb[got], verb[p.want], out)
			}
			if p.want {
				return
			}
			if ok, out := wgTry(t, p, false); !ok {
				t.Errorf("the CONTROL refused it too — the probe proves nothing about the narrowing:\n%s", out)
			}
		})
	}
}

// security's ask on the bead, in one measurement: a plain worktree commit
// stays green under the narrowing — under the REAL rendered profile, carve-
// out and all, not the stripped one the probe table varies. The witness is
// the branch ref, which must have moved to a commit that did not exist when
// the profile was rendered; an exit code alone would be satisfied by a git
// that did nothing.
func TestQAWorktreeCommitStaysGreenUnderTheNarrowedProfile(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := wgNewFixture(t)
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	before := mustGit(t, f.tree, "rev-parse", "HEAD")
	sh := "echo caged >> " + filepath.Join(f.tree, "work.txt") +
		" && git -C " + f.tree + " add work.txt" +
		" && git -C " + f.tree + " commit -q -m 'caged commit' -- work.txt"
	if ok, out := wgRun(t, prof, sh); !ok {
		t.Fatalf("a plain worktree commit is refused under the narrowed profile:\n%s\n\nprofile:\n%s", out, mustRead(t, prof))
	}
	after := mustGit(t, f.tree, "rev-parse", "HEAD")
	if after == before {
		t.Fatalf("git exited 0 but the branch did not move: %s", after)
	}
	if got := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch); got != after {
		t.Errorf("the session branch ref did not follow the commit: %s != %s", got, after)
	}
	if got := mustGit(t, f.repo, "rev-parse", "refs/heads/main"); got == after {
		t.Errorf("the commit landed on the operator's main: %s", got)
	}
	// The same profile, same session, one command later: main is out of
	// reach. The commit above is what makes this a narrowing and not a
	// wall — both halves have to be true at once.
	if ok, _ := wgRun(t, prof, "git -C "+f.tree+" update-ref refs/heads/main "+after); ok {
		t.Errorf("the profile that let the commit through also let main move")
	}
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// ─── the ref's parent directory (ranger-base-uuze) ───────────────────────────
//
// The narrowing above names the branch ref as a LEAF. The fleet's branches
// carry a slash, so that leaf is a file two directories deep and git must
// make `refs/heads/posse` before it can take the lock. Nothing granted it.
// While the directory happens to exist the commit works — which is how the
// narrowing shipped green — and `git pack-refs --all` prunes it the moment
// it empties, which `git gc` does on its own at gc.auto. The fixture above
// never reaches that state: it packs at :61, BEFORE `worktree add` cuts the
// branch at :67, so the directory is created afterwards and never pruned.
//
// wgPack constructs the state the fleet reaches instead, and asserts it was
// really constructed. A rig for "the directory is gone" that quietly leaves
// it there measures nothing, and every sandbox probe below it would pass
// under the defect (ranger-base-h15's shape, again).

func wgPack(t *testing.T, f wgFixture) {
	t.Helper()
	mustGit(t, f.repo, "pack-refs", "--all")
	dir := filepath.Dir(f.ref(f.branch))
	if wgExists(t, dir) {
		t.Fatalf("pack-refs left %s in place — the arm this test needs was not built, so nothing below it is measured", dir)
	}
	if got, head := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch), mustGit(t, f.tree, "rev-parse", "HEAD"); got != head {
		t.Fatalf("the packed ref no longer names the session's commit (%s != %s)", got, head)
	}
}

// The grant, as two lists with two different meanings: the ref's parent is
// creatable and is NOT writable. If it ever joins the writable set instead,
// every sibling session's branch ref comes back with it and the narrowing
// this file pins is undone from the other side.
func TestQAWorktreeRefParentIsGrantedForCreationOnly(t *testing.T) {
	f := wgNewFixture(t)
	parent := filepath.Dir(f.ref(f.branch)) // <common>/refs/heads/posse
	got := sessionRefDirs(f.tree)
	if len(got) != 1 || got[0] != absResolve(parent) {
		t.Fatalf("sessionRefDirs = %v, want [%s]", got, parent)
	}
	w := f.writable(t)
	for _, p := range []string{
		parent,
		filepath.Join(parent, "other-probe"), // the sibling's ref lives in it
		filepath.Join(parent, "sneaky"),
	} {
		if sbCovers(w, p) {
			t.Errorf("the create-only directory leaked into the writable set: %s\n  %s", p, strings.Join(w, "\n  "))
		}
	}
	// And it is spelled as a create, above the carve-out that must still
	// outvote it — a literal allow BELOW the trailing deny would re-open
	// whatever that deny closed.
	prof := SeatbeltProfile("developer-2", w, nil, SeatbeltCarveOut{Deny: []string{f.gates}}, got...)
	line := "  (literal " + sbQuote(absResolve(parent)) + ")\n"
	create := strings.Index(prof, "(allow file-write-create\n")
	if create < 0 || !strings.Contains(prof[create:], line) {
		t.Fatalf("the profile does not grant %s for creation:\n%s", parent, prof)
	}
	if deny := strings.Index(prof, "(deny file-write*\n  (subpath"); deny < 0 || deny < create {
		t.Errorf("the create block renders at %d and the carve-out deny at %d — the deny must come last:\n%s", create, deny, prof)
	}
	if strings.Contains(prof, "(subpath "+sbQuote(absResolve(parent))+")") {
		t.Errorf("the parent directory is granted as a subpath as well — that is every sibling's ref:\n%s", prof)
	}
}

// Nothing to create is nothing granted: a detached HEAD has no branch, a
// main checkout has no common dir of its own, and a slashless branch's ref
// sits directly in refs/heads, which git leaves in place across a pack.
func TestQAWorktreeRefParentIsEmptyWhenThereIsNoDirectoryToMake(t *testing.T) {
	f := wgNewFixture(t)
	if got := sessionRefDirs(f.repo); got != nil {
		t.Errorf("sessionRefDirs(main checkout) = %v, want none", got)
	}
	mustGit(t, f.tree, "checkout", "-q", "-b", "flat")
	if got := sessionRefDirs(f.tree); got != nil {
		t.Errorf("sessionRefDirs(slashless branch) = %v, want none — refs/heads is not this bead's to grant", got)
	}
	mustGit(t, f.tree, "checkout", "-q", "--detach")
	if got := sessionRefDirs(f.tree); got != nil {
		t.Errorf("sessionRefDirs(detached) = %v, want none", got)
	}
}

// The regression, by execution: the same commit TestQAWorktreeCommitStays-
// GreenUnderTheNarrowedProfile makes, on a repo whose refs have been packed
// since the branch was cut. The control arm is the profile as it shipped —
// same fixture, same command, the create block removed and nothing else —
// so a green here is the create grant and not the fixture.
func TestQAWorktreeCommitStaysGreenAfterPackRefs(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	commit := func(f wgFixture) string {
		return "echo caged >> " + filepath.Join(f.tree, "work.txt") +
			" && git -C " + f.tree + " add work.txt" +
			" && git -C " + f.tree + " commit -q -m 'caged commit' -- work.txt"
	}

	// The control first, because it is the claim that this test measures
	// anything at all: without the create grant the commit DIES, and the
	// commit is lost rather than landing somewhere else.
	ctl := wgNewFixture(t)
	wgPack(t, ctl)
	gates := ctl.a.GatesDir(ctl.ag.Name)
	w := ctl.writable(t)
	shipped := sbRenderProfile(t, "shipped.sb", SeatbeltProfile(ctl.ag.Name, w, nil, ctl.a.SeatbeltCarveOut(ctl.ag, ctl.tree, gates, w)))
	if ok, out := wgRun(t, shipped, commit(ctl)); ok {
		t.Errorf("the pre-fix profile committed after a pack — the control proves nothing:\n%s", out)
	} else if !strings.Contains(out, "unable to create directory") {
		t.Errorf("the control failed for some other reason than the missing directory:\n%s", out)
	}
	if got := mustGit(t, ctl.repo, "rev-parse", "refs/heads/"+ctl.branch); got != ctl.head {
		t.Errorf("the refused commit moved the ref anyway: %s", got)
	}

	f := wgNewFixture(t)
	wgPack(t, f)
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	if ok, out := wgRun(t, prof, commit(f)); !ok {
		t.Fatalf("a worktree commit is refused after a pack-refs:\n%s\n\nprofile:\n%s", out, mustRead(t, prof))
	}
	after := mustGit(t, f.tree, "rev-parse", "HEAD")
	if after == f.head {
		t.Fatalf("git exited 0 but the branch did not move: %s", after)
	}
	if got := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch); got != after {
		t.Errorf("the session branch ref did not follow the commit: %s != %s", got, after)
	}

	// What the create grant must NOT have bought back, under the very
	// profile that just committed: the directory is creatable, and nothing
	// in it — or beside it — is writable.
	for _, p := range []wgProbe{
		{what: "the sibling session's ref, inside the same directory", sh: func(f wgFixture) string {
			return "printf '%s\\n' " + after + " > " + f.ref("posse/other-probe")
		}},
		{what: "the sibling's ref via git update-ref", sh: func(f wgFixture) string {
			return "git -C " + f.tree + " update-ref refs/heads/posse/other-probe " + after
		}},
		{what: "a new branch beside its own in the same directory", sh: func(f wgFixture) string {
			return "git -C " + f.tree + " branch posse/sneaky " + after
		}},
		{what: "the operator's main", sh: func(f wgFixture) string {
			return "git -C " + f.tree + " update-ref refs/heads/main " + after
		}},
		{what: "packed-refs", sh: func(f wgFixture) string {
			return ": > " + filepath.Join(f.common, "packed-refs")
		}},
		{what: "the shared pre-push hook", sh: func(f wgFixture) string {
			return "echo 'exit 0' > " + filepath.Join(f.common, "hooks", "pre-push")
		}},
	} {
		if ok, out := wgRun(t, prof, p.sh(f)); ok {
			t.Errorf("the create grant bought back %s:\n%s", p.what, out)
		}
	}
}

// The premise the grant rests on, measured without sandbox-exec so it holds
// in a CAGED session too — where every probe above skips (ranger-base-xjw9)
// and this file would otherwise assert nothing about the defect at all.
//
// The variable is the one directory, and the wall is a read-only
// `refs/heads` rather than a profile: what a create grant buys is exactly
// the mkdir, so refusing the mkdir by any means reproduces the same failure.
// Three arms, one variable — the third is the discriminator, because it has
// the same packed refs as the second and differs only in the directory.
func TestQAWorktreeCommitNeedsTheRefsParentDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a read-only directory is not a wall, so the middle arm cannot fail (see the memory note on chmod)")
	}
	f := wgNewFixture(t)
	heads := filepath.Join(f.common, "refs", "heads")
	commit := func(name string) error {
		if err := os.WriteFile(filepath.Join(f.tree, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := git(f.tree, "add", "--", name); err != nil {
			t.Fatal(err)
		}
		_, err := git(f.tree, "commit", "-q", "-m", name, "--", name)
		return err
	}
	sealed := func(fn func()) {
		if err := os.Chmod(heads, 0o555); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(heads, 0o755)
		fn()
	}

	// 1. the loose ref is there, so is its directory: nothing to create.
	sealed(func() {
		if err := commit("one.txt"); err != nil {
			t.Fatalf("arm 1 was refused with the directory in place — the wall is not measuring the directory: %v", err)
		}
	})
	// 2. the pack prunes it, and the commit dies with it.
	wgPack(t, f)
	before := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch)
	sealed(func() {
		if err := commit("two.txt"); err == nil {
			t.Error("arm 2 committed with refs/heads unwritable and the directory pruned — git did not need to create it, and the grant this bead adds is dead weight")
		}
	})
	if got := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch); got != before {
		t.Errorf("the refused commit moved the ref anyway: %s != %s", got, before)
	}
	// 3. same packed state, the directory back: it commits again.
	if err := os.MkdirAll(filepath.Dir(f.ref(f.branch)), 0o755); err != nil {
		t.Fatal(err)
	}
	sealed(func() {
		if err := commit("three.txt"); err != nil {
			t.Fatalf("arm 3 was refused with the directory recreated — then arm 2 was failing for some other reason: %v", err)
		}
	})
	if got := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch); got == before {
		t.Error("arm 3 exited 0 but the ref did not move")
	}
}

// ─── packed-refs.lock: declared, not granted (ranger-base-msex) ────────────
//
// The second finding on ranger-base-uuze: every commit under the
// narrowed grant prints `error: Unable to create '<common>/packed-refs.lock':
// Operation not permitted` on stderr and then succeeds. The two tests below
// are the measurement that decision rests on, in both directions — the
// accepted behaviour stays green, and the tempting fix is pinned as unsafe
// so it cannot come back quietly.

// The accepted shape: the refusal is noise, not damage. Two commits in a
// row, one of them after a pack — the same commit TestQAWorktreeCommitStays-
// GreenAfterPackRefs makes — and neither leaves anything behind in the
// shared common dir. A stray lock is the failure mode the next test pins;
// this one is the control that the SHIPPED profile does not produce it.
func TestQAWorktreeCommitLeavesNoStrayPackedRefsLock(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := wgNewFixture(t)
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(f.common, "packed-refs.lock")
	commit := func(name string) (bool, string) {
		return wgRun(t, prof, "echo "+name+" >> "+filepath.Join(f.tree, "work.txt")+
			" && git -C "+f.tree+" add work.txt"+
			" && git -C "+f.tree+" commit -q -m "+name+" -- work.txt")
	}
	if ok, out := commit("one"); !ok {
		t.Fatalf("first commit refused:\n%s", out)
	}
	if wgExists(t, lock) {
		t.Fatal("packed-refs.lock survived the first commit — the refusal is no longer self-healing")
	}
	wgPack(t, f)
	if ok, out := commit("two"); !ok {
		t.Fatalf("commit after pack-refs refused:\n%s", out)
	}
	if wgExists(t, lock) {
		t.Fatal("packed-refs.lock survived a commit after pack-refs")
	}

	// ─── ADR 0059 D2: what the write+unlink grant must keep true ──────────
	//
	// The body above is D2.2 and predates the grant: it was the msex
	// control that the SHIPPED profile leaves no stray lock, and it says
	// the same thing about the granted one. What follows is the rest of
	// D2, each item by EXECUTION under the profile a dispatched session
	// really gets (RenderSeatbelt), with the set as it shipped — this one
	// entry short — as the control wherever a control can run the probe.
	for _, arm := range wgSeqArms() {
		t.Run("D2.1 "+arm.what, func(t *testing.T) { wgRunSeqArm(t, arm) })
	}
	t.Run("D2.3 deleting a PACKED ref fails at packed-refs.new and rolls back", wgRunPackedRefRewrite)
	for _, arm := range wgAliasArms() {
		t.Run("D2.4 "+arm.what, func(t *testing.T) { wgRunAliasArm(t, arm) })
	}
}

// ─── D2.1: every sequencer end (ADR 0059's reason for existing) ──────────────
//
// ranger-base-71g2f's finding, which is what D1 buys off: every git verb
// that ends by DELETING a pseudo-ref asks the SHARED repo's
// packed-refs.lock — `cherry-pick`, `revert`, `rebase`, on `--abort`,
// `--quit`, `--continue`, and on a CLEAN run that never conflicted at all.
// Refused, git gives up the delete and exits 0; CHERRY_PICK_HEAD survives,
// and the session's next path-limited commit — the only form its PID allows
// — dies at `fatal: cannot do a partial commit during a cherry-pick`.
//
// So the arm is not "the verb exits 0". It is: exits as git says, says
// nothing about packed-refs.lock, leaves no blocker, leaves no lock. The
// control is the same script under the set one entry short, and it has two
// jobs — witness that the arm really reaches the lock (it names it), and
// witness that the defect is real (a blocker survives). An arm whose
// control goes quiet is measuring the fixture.

type wgSeqArm struct {
	what string
	// prep runs under the SAME profile and its exit code is ignored on
	// purpose: a conflicted cherry-pick exits 1, and that conflict is the
	// state the arm needs.
	prep func(f wgFixture, clean, conflict string) []string
	// steps are the measured commands, in order.
	steps func(f wgFixture, clean, conflict string) []string
}

// wgSequencer gives a fresh fixture what a sequencer arm needs, and nothing
// else: one commit on the operator's main that cherry-picks CLEANLY into
// the session tree (a file the tree has never seen), and a pair — one on
// main, one in the tree — that add the SAME file with different content, so
// the verbs conflict. It is built on top of wgNewFixture rather than beside
// it so the shape stays the fleet's: refs packed before the branch was cut,
// the branch two directories deep, a sibling worktree on the same repo.
func wgSequencer(t *testing.T, f wgFixture) (clean, conflict string) {
	t.Helper()
	commitIn(t, f.repo, "clean.txt", "main line\n", "clean-on-main")
	clean = mustGit(t, f.repo, "rev-parse", "HEAD")
	commitIn(t, f.repo, "conflict.txt", "main says A\n", "conflict-on-main")
	conflict = mustGit(t, f.repo, "rev-parse", "HEAD")
	commitIn(t, f.tree, "conflict.txt", "session says B\n", "session-conflict")
	return clean, conflict
}

// wgBlockers names the sequencer state that makes the session's next
// path-limited commit fail. It lives in the PER-WORKTREE git dir, which the
// session may write — the file it cannot delete is the packed lock in the
// SHARED one, and that is what stops git deleting these.
//
// `AUTO_MERGE` is deliberately NOT in the list, and that is a measurement:
// the ort strategy's scratch ref survives a clean cherry-pick in BOTH arms
// (MEASURED 2026-09-11, darwin 25.4.0, git 2.50.1 — sealed common dir
// leaves `AUTO_MERGE CHERRY_PICK_HEAD`, writable leaves `AUTO_MERGE`), and
// a path-limited commit lands with it sitting there. It blocks nothing and
// it does not vary with this grant, so counting it would make every arm
// below red for a reason that has nothing to do with the lock.
func wgBlockers(t *testing.T, f wgFixture) []string {
	t.Helper()
	var out []string
	for _, n := range []string{"CHERRY_PICK_HEAD", "REVERT_HEAD", "MERGE_HEAD", "sequencer", "rebase-merge", "rebase-apply"} {
		if wgExists(t, filepath.Join(f.own, n)) {
			out = append(out, n)
		}
	}
	return out
}

func wgSeqArms() []wgSeqArm {
	g := func(f wgFixture, rest string) string { return "git -C " + f.tree + " " + rest }
	// The commit afterwards is the cost D1 buys back, so the arm that
	// aborts carries it: under the shipped grant this is where the session
	// actually dies, one command after the verb that exited 0.
	commit := func(f wgFixture) string {
		return "echo after >> " + filepath.Join(f.tree, "work.txt") +
			" && " + g(f, "add work.txt") + " && " + g(f, "commit -q -m after -- work.txt")
	}
	pick := func(f wgFixture, _, conflict string) []string { return []string{g(f, "cherry-pick "+conflict)} }
	return []wgSeqArm{
		{"a clean cherry-pick, which never conflicts at all", nil,
			func(f wgFixture, clean, _ string) []string { return []string{g(f, "cherry-pick "+clean)} }},
		{"conflicted cherry-pick --abort, then the path-limited commit the PID allows", pick,
			func(f wgFixture, _, _ string) []string { return []string{g(f, "cherry-pick --abort"), commit(f)} }},
		{"conflicted cherry-pick --quit", pick,
			func(f wgFixture, _, _ string) []string { return []string{g(f, "cherry-pick --quit")} }},
		{"conflicted cherry-pick, resolved, --continue", pick,
			func(f wgFixture, _, _ string) []string {
				return []string{"echo resolved > " + filepath.Join(f.tree, "conflict.txt") +
					" && " + g(f, "add conflict.txt") + " && GIT_EDITOR=true " + g(f, "cherry-pick --continue")}
			}},
		{"conflicted revert --abort", func(f wgFixture, _, _ string) []string {
			return []string{"echo later >> " + filepath.Join(f.tree, "conflict.txt") +
				" && " + g(f, "add conflict.txt") + " && " + g(f, "commit -q -m later -- conflict.txt"),
				g(f, "revert --no-edit HEAD~1")}
		}, func(f wgFixture, _, _ string) []string { return []string{g(f, "revert --abort")} }},
		{"conflicted rebase --abort", func(f wgFixture, _, conflict string) []string {
			return []string{g(f, "rebase "+conflict)}
		}, func(f wgFixture, _, _ string) []string { return []string{g(f, "rebase --abort")} }},
	}
}

func wgRunSeqArm(t *testing.T, arm wgSeqArm) {
	t.Helper()
	sbSkipUnlessSandboxable(t)

	f := wgNewFixture(t)
	clean, conflict := wgSequencer(t, f)
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	if arm.prep != nil {
		for _, sh := range arm.prep(f, clean, conflict) {
			wgRun(t, prof, sh) // may fail: the conflict IS the setup
		}
		// Prep's exit codes are ignored, so this is what says it worked. An
		// arm that ends a sequencer git never started measures the grant
		// against nothing at all.
		if b := wgBlockers(t, f); len(b) == 0 {
			t.Fatalf("the setup left no sequencer state in %s — there is nothing for this arm's verb to end", f.own)
		}
	}
	for _, sh := range arm.steps(f, clean, conflict) {
		ok, out := wgRun(t, prof, sh)
		if !ok {
			t.Errorf("refused under the ADR 0059 grant, which is what the grant exists to allow:\n  %s\n%s", sh, out)
		}
		if strings.Contains(out, "packed-refs.lock") {
			t.Errorf("the grant is in the set and git still could not have the lock — the entry is not reaching the profile:\n  %s\n%s", sh, out)
		}
	}
	if b := wgBlockers(t, f); len(b) != 0 {
		t.Errorf("the verb exited 0 and left %v in %s — the session's next path-limited commit dies on it, which is the whole defect ADR 0059 D1 closes", b, f.own)
	}
	if wgExists(t, f.lock()) {
		t.Errorf("git created %s and never unlinked it — write WITHOUT unlink is the ranger-base-msex strand, and it kills the operator's gc", f.lock())
	}

	// The control, on its own fresh fixture: the same script under the set
	// as it SHIPPED, one entry short and nothing else varied.
	c := wgNewFixture(t)
	cclean, cconflict := wgSequencer(t, c)
	ctl := c.profile(t, "shipped.sb", c.withoutLock(t))
	var said, last string
	run := func(sh string) {
		_, out := wgRun(t, ctl, sh)
		last = out
		if strings.Contains(out, "packed-refs.lock") {
			said = out
		}
	}
	if arm.prep != nil {
		for _, sh := range arm.prep(c, cclean, cconflict) {
			run(sh)
		}
		if b := wgBlockers(t, c); len(b) == 0 {
			t.Fatalf("the control's setup left no sequencer state in %s — there is nothing for its verb to end either", c.own)
		}
	}
	for _, sh := range arm.steps(c, cclean, cconflict) {
		run(sh)
	}
	if said == "" {
		t.Errorf("the CONTROL never asked for packed-refs.lock — this arm does not reach the grant it is varying, so its green above measures nothing:\n%s", last)
	}
	if b := wgBlockers(t, c); len(b) == 0 {
		t.Errorf("the CONTROL left no sequencer state behind — the defect ADR 0059 prices is not reproducing on this box, and the arm above is proving nothing")
	}
	if wgExists(t, c.lock()) {
		t.Errorf("the control STRANDED a lock it was never granted: %s", c.lock())
	}
}

// ─── D2.3: the rewrite git actually wants ────────────────────────────────────
//
// The lock is a pure lock on git 2.50.1: the new content goes through
// `packed-refs.new` and is renamed over `packed-refs`, and both of those
// stay denied. So a session that gets git to WANT a packed-refs rewrite —
// deleting a ref that exists only in the pack — gets as far as the lock and
// no further: git fails at the tempfile, rolls back, removes its own lock,
// and `packed-refs` is byte-identical. This is also why removing a live
// lock costs at most a lost update between two unsandboxed writers rather
// than a corrupt file, and why the "just skip the packed lock on delete"
// alternative cannot exist — deleting only the loose copy of a packed ref
// resurrects the packed value.
func wgRunPackedRefRewrite(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := wgNewFixture(t)
	wgPack(t, f) // the session's ref now lives ONLY in packed-refs
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	packed := filepath.Join(f.common, "packed-refs")
	before := mustRead(t, packed)
	del := "git -C " + f.repo + " update-ref -d refs/heads/" + f.branch

	ok, out := wgRun(t, prof, del)
	if ok {
		t.Errorf("the grant let a session delete a PACKED ref — packed-refs is rewritable through the lock after all:\n%s", out)
	}
	if !strings.Contains(out, "packed-refs.new") {
		t.Errorf("the refusal did not land at packed-refs.new — if git stopped somewhere else the rollback this arm asserts is a different rollback:\n%s", out)
	}
	if wgExists(t, f.lock()) {
		t.Errorf("git did not remove its own lock after the rollback — the unlink half of D1 is what keeps a failed rewrite from stranding: %s", f.lock())
	}
	if after := mustRead(t, packed); after != before {
		t.Errorf("packed-refs changed under the grant:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if got := mustGit(t, f.repo, "rev-parse", "refs/heads/"+f.branch); got != f.head {
		t.Errorf("the session's packed ref moved anyway: %s != %s", got, f.head)
	}

	// The control: same command, the set one entry short. The refusal is
	// still a refusal — it just lands a step EARLIER, at the lock — and
	// packed-refs survives in both. What D1 changed is where git stops,
	// not whether packed-refs is reachable.
	c := wgNewFixture(t)
	wgPack(t, c)
	ctl := c.profile(t, "shipped.sb", c.withoutLock(t))
	cbefore := mustRead(t, filepath.Join(c.common, "packed-refs"))
	cok, cout := wgRun(t, ctl, "git -C "+c.repo+" update-ref -d refs/heads/"+c.branch)
	if cok {
		t.Errorf("the CONTROL deleted the packed ref — packed-refs was reachable before this grant too, and this arm is not about the grant:\n%s", cout)
	}
	if !strings.Contains(cout, "packed-refs.lock") {
		t.Errorf("the CONTROL did not stop at the lock — then the arm above is not measuring the entry that was added:\n%s", cout)
	}
	if after := mustRead(t, filepath.Join(c.common, "packed-refs")); after != cbefore {
		t.Errorf("the control changed packed-refs:\n--- before\n%s\n--- after\n%s", cbefore, after)
	}
}

// ─── D2.4: the lock name is not a route to anything else ─────────────────────
//
// A writable file name in a directory that is otherwise denied is only as
// narrow as the operations it permits. These are the ways a name becomes a
// write on its neighbours — a hard link, a symlink, a rename — and the
// tempfile the real content goes through. None needs a control: a refusal
// under the grant is the whole claim, and the grant is what is being
// varied, so a control could only show the same refusal for a wider reason.

type wgAliasArm struct {
	what string
	// pre, when set, must SUCCEED before sh is measured. Only one arm has
	// one, and it is why: planting a symlink at the lock name is allowed —
	// the name is granted — and the refusal that matters is the write
	// THROUGH it. Folded into sh with an `&&` the two would be one exit
	// code, and an arm that reported "refused" could be reporting that the
	// symlink was never planted at all.
	pre  func(f wgFixture) string
	sh   func(f wgFixture) string
	want bool
}

func wgAliasArms() []wgAliasArm {
	packed := func(f wgFixture) string { return filepath.Join(f.common, "packed-refs") }
	return []wgAliasArm{
		{what: "a hard link of packed-refs at the lock name", sh: func(f wgFixture) string {
			return "ln " + packed(f) + " " + f.lock()
		}},
		{
			// sandbox-exec resolves the path, so the grant follows the NAME
			// and not the inode a symlink at it points to.
			what: "a write THROUGH a symlink planted at the lock name",
			pre:  func(f wgFixture) string { return "ln -s " + packed(f) + " " + f.lock() },
			sh:   func(f wgFixture) string { return "echo junk >> " + f.lock() },
		},
		{what: "the lock renamed onto packed-refs", sh: func(f wgFixture) string {
			return "touch " + f.lock() + " && mv " + f.lock() + " " + packed(f)
		}},
		{what: "the lock renamed onto refs/heads/main", sh: func(f wgFixture) string {
			return "touch " + f.lock() + " && mv " + f.lock() + " " + f.ref("main")
		}},
		{what: "packed-refs renamed onto the lock name", sh: func(f wgFixture) string {
			return "mv " + packed(f) + " " + f.lock()
		}},
		{what: "a write to packed-refs.new, which is where the content really goes", sh: func(f wgFixture) string {
			return "echo x > " + filepath.Join(f.common, "packed-refs.new")
		}},
	}
}

func wgRunAliasArm(t *testing.T, arm wgAliasArm) {
	t.Helper()
	sbSkipUnlessSandboxable(t)
	f := wgNewFixture(t)
	prof, err := f.a.RenderSeatbelt(f.ag, f.tree)
	if err != nil {
		t.Fatal(err)
	}
	packed := filepath.Join(f.common, "packed-refs")
	before := mustRead(t, packed)
	mainRef := mustGit(t, f.repo, "rev-parse", "refs/heads/main")

	if arm.pre != nil {
		if ok, out := wgRun(t, prof, arm.pre(f)); !ok {
			t.Fatalf("the setup for %q was refused, so the refusal below would be that refusal and not the one this arm is about:\n%s", arm.what, out)
		}
	}
	if ok, out := wgRun(t, prof, arm.sh(f)); ok != arm.want {
		t.Errorf("%s: got %v, want %v — the lock name is a grant on ONE file and this is the list of ways it could stop being one:\n%s",
			arm.what, ok, arm.want, out)
	}
	// Whatever the exit code said, the shared files are the claim.
	if !wgExists(t, packed) {
		t.Fatalf("packed-refs is gone after %q — the grant moved the operator's ref store", arm.what)
	}
	if after := mustRead(t, packed); after != before {
		t.Errorf("packed-refs changed after %q:\n--- before\n%s\n--- after\n%s", arm.what, before, after)
	}
	if got := mustGit(t, f.repo, "rev-parse", "refs/heads/main"); got != mainRef {
		t.Errorf("the operator's main moved after %q: %s != %s", arm.what, got, mainRef)
	}
}

// The rejected shape, kept alive on purpose: a createOnly grant for
// packed-refs.lock — the same shape sessionRefDirs uses for the ref's
// parent directory — looks like the obvious fix for the stderr line above.
// MEASURED and worse than what it silences: create-only buys the create but
// not git's own cleanup unlink, so the lock file itself is stranded in the
// shared common dir. The session's OWN later commits do not notice — same
// as the doc comment on sessionRefDirs's neighbour says, an ordinary
// commit's ref update never needs that lock — so that half is asserted here
// as the reason this candidate looks safe from inside a session. What it
// actually breaks is anyone who DOES need the lock: the operator's own
// unsandboxed `git gc` / `git pack-refs` on the shared repo, hard, with no
// session able to clear it.
//
// This builds the candidate profile that grant would produce and asserts
// both halves, so the temptation cannot be re-added without this test
// failing first.
func TestQAPackedRefsLockCreateGrantIsUnsafe(t *testing.T) {
	sbSkipUnlessSandboxable(t)
	f := wgNewFixture(t)
	// ADR 0059 D1 put the lock in the WRITABLE set, and a subpath grant
	// outvotes the create-only line this test exists to measure: built on
	// today's set the candidate would get the unlink too, leave no stray,
	// and the test would report the hazard gone. So the candidate is built
	// on the set as it shipped BEFORE 0059 — the one entry removed, and
	// nothing else varied. withoutLock fatals if the entry is not there to
	// remove, so this cannot quietly become a test of the shipped set.
	w := f.withoutLock(t)
	lock := f.lock()
	createOnly := append(append([]string{}, sessionRefDirs(f.tree)...), lock)
	carve := f.a.SeatbeltCarveOut(f.ag, f.tree, f.gates, w)
	candidate := sbRenderProfile(t, "candidate.sb", SeatbeltProfile(f.ag.Name, w, nil, carve, createOnly...))
	commit := func(name string) (bool, string) {
		return wgRun(t, candidate, "echo "+name+" >> "+filepath.Join(f.tree, "work.txt")+
			" && git -C "+f.tree+" add work.txt"+
			" && git -C "+f.tree+" commit -q -m "+name+" -- work.txt")
	}
	if ok, out := commit("one"); !ok {
		t.Fatalf("first commit under the candidate grant was refused outright — the candidate must at least reach the strand:\n%s", out)
	}
	if !wgExists(t, lock) {
		t.Fatal("the candidate grant left no stray lock — the hazard this test pins is gone, and the doc comment on sessionRefDirs's neighbour needs revisiting instead of this test")
	}
	// The session's own next commit is NOT where this bites — it still
	// lands, only louder. Pinned so a future reader trusting a green commit
	// here does not mistake it for the candidate being safe.
	if ok, out := commit("two"); !ok {
		t.Fatalf("a second commit was refused outright under the stray lock — that is a DIFFERENT, worse hazard than the one this test measures:\n%s", out)
	} else if !strings.Contains(out, "File exists") {
		t.Errorf("the second commit's noise changed shape — expected the stray-lock message:\n%s", out)
	}
	// The actual hazard: the operator's real gc/pack-refs on the shared
	// repo, unsandboxed, dies as long as the stray file sits there.
	if out, err := exec.Command("git", "-C", f.repo, "pack-refs", "--all").CombinedOutput(); err == nil {
		t.Error("the operator's own unsandboxed pack-refs succeeded despite the stray lock — the shared-repo hazard this test pins is gone")
	} else if !strings.Contains(string(out), "File exists") {
		t.Errorf("the operator's pack-refs failed for some other reason than the stray lock:\n%s", out)
	}
	// gc's EXIT STATUS is not where the hazard lives, and it moved.
	// MEASURED 2026-09-04 (ranger-base-90y3c) over this exact repo shape, a
	// stray packed-refs.lock and nothing else, on both gits in play here:
	//
	//   git 2.50.1 (this box)      gc rc=128  "fatal: failed to run pack-refs"
	//   git 2.55.0 (both runners)  gc rc=0    "error: failed to run pack-refs"
	//
	// Same refusal, same stranded lock, demoted from fatal to error — so
	// `err == nil` red macos-latest on every push while the hazard was
	// exactly as bad as it had ever been. What did NOT move is asserted
	// instead: gc still hits the lock and says so, and gc still leaves it
	// there. Nobody's gc clears another process's lock, and that is what
	// makes the strand permanent — which is the whole claim.
	out, _ := exec.Command("git", "-C", f.repo, "gc").CombinedOutput()
	if !strings.Contains(string(out), "File exists") {
		t.Errorf("the operator's own unsandboxed gc no longer even reaches the stray lock — the shared-repo hazard this test pins is gone:\n%s", out)
	}
	if !wgExists(t, lock) {
		t.Error("gc cleared the stray lock — the strand this test pins is not permanent after all, and the doc comment on sessionRefDirs's neighbour needs revisiting instead of this test")
	}
}

// ─── what an operator reads (ADR 0059 verification 3) ───────────────────────
//
// The grant costs a reader exactly one line, and it has to be the RIGHT
// line. `posse gates` marks create-only grants with `+` and atomic-write
// siblings with `~` precisely so neither is mistaken for a writable path,
// and this entry is neither: create-only is the spelling ranger-base-msex
// measured and rejected — it strands the lock and kills the operator's gc —
// and a sibling grant would hand back `packed-refs.lock.*` names nobody has
// measured. So the assertion is the marker, not just the presence.
//
// It needs no sandbox-exec, which is the point: this is the one arm of ADR
// 0059 a caged session can run, and it is the one an operator actually
// looks at.
func TestQAGatesReportShowsThePackedRefsLockAsOneWriteLine(t *testing.T) {
	f := wgNewFixture(t)
	var buf strings.Builder
	if err := f.a.SeatbeltReport(f.ag, f.tree, &buf); err != nil {
		t.Fatal(err)
	}
	report := buf.String()
	want := "w " + AbbrevHome(f.lock())
	var got int
	for _, line := range strings.Split(report, "\n") {
		line = strings.TrimSpace(line)
		if line == want {
			got++
			continue
		}
		if !strings.Contains(line, "packed-refs.lock") {
			continue
		}
		if strings.HasPrefix(line, "+ ") {
			t.Errorf("the lock is reported as a CREATE-ONLY grant — that is the ranger-base-msex spelling, which strands the lock and kills the operator's gc:\n  %s", line)
		}
		if strings.HasPrefix(line, "~ ") {
			t.Errorf("the lock is reported as an atomic-write SIBLING grant — that would cover packed-refs.lock.* names nothing has measured:\n  %s", line)
		}
	}
	if got != 1 {
		t.Errorf("`posse gates` printed %d lines reading %q, want exactly 1 — ADR 0059 verification 3 is what an operator reads this grant off:\n%s", got, want, report)
	}
	// And it is the LOCK that joined the report, not the file it guards.
	if bad := "w " + AbbrevHome(filepath.Join(f.common, "packed-refs")); strings.Contains(report, bad+"\n") {
		t.Errorf("packed-refs itself is reported writable — that is the grant ranger-base-m2wf narrowed away:\n%s", report)
	}
}

// ─── the premise, without sandbox-exec (ranger-base-xjw9) ────────────────────
//
// Every arm above is a sandbox-exec, and a session that may not apply a
// profile skips all of them — which is every DISPATCHED session on this
// fleet, since a caged persona cannot nest one. The same gap that
// TestQAWorktreeCommitNeedsTheRefsParentDirectory closes for the ref's
// parent directory, closed the same way for this grant: the variable is
// whether git may CREATE that one file in the shared common dir, so
// refusing the create by any means reproduces the failure, and a read-only
// common DIRECTORY refuses it while leaving `objects`, `logs` and
// `refs/heads/...` — which have their own modes — exactly as they were.
//
// Two arms, one variable, on two fresh fixtures, and the first names the
// refusal it got so the wall is witnessed rather than assumed. MEASURED
// 2026-09-11, darwin 25.4.0, git 2.50.1 Apple Git-155, inside a caged
// session where every probe above skipped.
func TestQASequencerNeedsThePackedRefsLock(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a read-only directory is not a wall, so the sealed arm cannot fail (same reason as the refs-parent arm above)")
	}
	// One clean cherry-pick and the path-limited commit after it, run
	// UNSANDBOXED so this holds wherever the suite runs. Returns what git
	// said on the way — stderr included, because the defect is a command
	// that exits 0 and complains.
	arm := func(t *testing.T, sealed bool) (pickOut string, blockers []string, lock bool, commitErr error) {
		t.Helper()
		f := wgNewFixture(t)
		clean, _ := wgSequencer(t, f)
		if sealed {
			if err := os.Chmod(f.common, 0o555); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(f.common, 0o755)
		}
		out, _ := exec.Command("git", "-C", f.tree, "cherry-pick", clean).CombinedOutput()
		pickOut = strings.TrimSpace(string(out))
		blockers = wgBlockers(t, f)
		lock = wgExists(t, f.lock())
		if err := os.WriteFile(filepath.Join(f.tree, "after.txt"), []byte("after\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := git(f.tree, "add", "--", "after.txt"); err != nil {
			t.Fatal(err)
		}
		_, commitErr = git(f.tree, "commit", "-q", "-m", "after", "--", "after.txt")
		return
	}

	// 1. the create refused — the shipped grant's shape. git exits 0, gives
	//    up the pseudo-ref delete, and the next commit dies on what it left.
	out, blockers, lock, err := arm(t, true)
	if !strings.Contains(out, "packed-refs.lock") {
		t.Fatalf("the sealed arm never mentioned packed-refs.lock — the wall is not measuring this file, so neither arm says anything about the grant:\n%s", out)
	}
	if len(blockers) == 0 {
		t.Error("arm 1: a cherry-pick whose packed-refs.lock was refused left no sequencer state — the defect ADR 0059 D1 closes does not reproduce, and the grant is dead weight")
	}
	if lock {
		t.Error("arm 1: a refused create left the lock file behind anyway")
	}
	if err == nil {
		t.Error("arm 1: the path-limited commit after it LANDED — then the refused delete costs a stderr line and nothing more, and ADR 0059's premise is wrong")
	} else if !strings.Contains(err.Error(), "partial commit") {
		t.Errorf("arm 1: the commit failed for some other reason than the stranded pseudo-ref: %v", err)
	}

	// 2. the same create allowed, and the unlink with it: git finishes its
	//    own cleanup, says nothing, leaves nothing, and the commit lands.
	out, blockers, lock, err = arm(t, false)
	if strings.Contains(out, "packed-refs.lock") {
		t.Errorf("arm 2: git still could not have the lock with the directory writable — the two arms differ by something other than this file:\n%s", out)
	}
	if len(blockers) != 0 {
		t.Errorf("arm 2: %v survived a clean cherry-pick that had the lock — then the lock is not what git needed to delete them", blockers)
	}
	if lock {
		t.Error("arm 2: git created the lock and did not unlink it — write-without-unlink is the ranger-base-msex strand, and the grant would be shipping it")
	}
	if err != nil {
		t.Errorf("arm 2: the path-limited commit after a clean cherry-pick was refused: %v", err)
	}
}
