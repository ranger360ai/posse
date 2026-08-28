package rhq

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
		filepath.Join(f.common, "packed-refs"),
		filepath.Join(f.common, "refs", "heads"), // the directory, or main comes back with it
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
	prof := sbRenderProfile(t, name, SeatbeltProfile("developer-2", w, SeatbeltCarveOut{}))
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
	if !SeatbeltAvailable() {
		t.Skip("no sandbox-exec on this host")
	}
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
	if !SeatbeltAvailable() {
		t.Skip("no sandbox-exec on this host")
	}
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
