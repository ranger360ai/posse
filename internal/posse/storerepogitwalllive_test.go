//go:build !posse_arm2 && !posse_arm3

package posse

// Live pins for ranger-base-kfrpv: under a REAL bwrap wall, rendered by the
// shipped producers, a seat whose store of record is in another repository
// writes that store's `.beads` and nothing of that repo's `.git`.
//
//	go test ./internal/posse -run TestLiveBwrapStoreRepoGit -v
//
// The render-level arms are in storerepogitwall_qa_test.go; this is the one
// that reads the wall itself, because the defect was VERIFIED that way — on
// a live seat, from /proc/self/mountinfo and access(W_OK), not from a set.
//
// IT HAS TO RUN ON AN sbRoot FIXTURE, and that is not a detail: the property
// here is an ABSENT grant, and the bwrap render grants `/tmp` and TMPDIR
// wholesale. MEASURED while writing this file, on a plain `t.TempDir()`
// store: under the FIXED render a seat unlinked an object of the store
// repo's history, rewrote it in place, wrote a new one and planted
// `objects/info/alternates` — all four through the blanket temp grant, with
// every producer answering correctly. `sbRoot` takes that grant out of the
// render for the duration of the test (sbNoBlanketTempGrants), which is what
// makes an absent grant measurable at all.
//
// EVERY REFUSAL IS READ BACK FROM OUTSIDE, for its neighbours' reason
// (bwrapseatstorelive_test.go): a row that trusts the exit code measures the
// shell, and what the hole was about is whether the object file, the ref and
// the alternates file are still what the operator's own git serves.
//
// THE CONTROL IS THE PRE-FIX RENDER, the same idiom and the same reason:
// every row below runs a second time under the set this bead removed — the
// shipped set plus `beadsGitDirs(store)` — and must SUCCEED there. A wall
// that refuses because the fixture is unwritable, or because the command was
// malformed, would otherwise read exactly like the wall this bead puts up.
// Each row is run on a FRESH store repo, so the control's own success cannot
// leave the next row's fixture changed.
//
// Gated like its neighbours (bgLiveSkip): Linux, bwrap and git, and not from
// inside a seat (ADR 0096 D1). No `t.Parallel`: sbRoot calls t.Setenv.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// srLive is one walled session tree whose `.beads/redirect` names a SECOND
// repository's store — the shape ADR 0012 D3-C puts every store on this
// instance in, and the shape the hole was verified in.
type srLive struct {
	f          wgFixture
	store      string // the store repo's root
	home       string // <store>/.beads, the store of record
	gitDir     string // <store>/.git — its git dir AND its common dir
	other      string // a commit object in the store repo that main does NOT name
	argv       []string
	argvBefore []string // + beadsGitDirs(home): the render this bead replaced
}

// srLiveFixture builds it from the one worktree fixture this package already
// has (wgFixtureIn over sbRoot), so this wall is rendered from the same tree
// shape every other seatbelt/bwrap pin reads.
func srLiveFixture(t *testing.T) srLive {
	t.Helper()
	bgLiveSkip(t)
	root := sbRoot(t)
	f := srLive{f: wgFixtureIn(t, root)}

	f.store = sbMkdir(t, filepath.Join(root, "store"))
	sbGitInit(t, f.store)
	mustGit(t, f.store, "config", "user.email", "t@example.com")
	mustGit(t, f.store, "config", "user.name", "t")
	f.home = absResolve(sbMkdir(t, filepath.Join(f.store, beadsDirName)))
	f.gitDir = absResolve(filepath.Join(f.store, ".git"))
	sbWrite(t, filepath.Join(f.home, "issues.jsonl"), "{\"id\":\"q-1\"}\n")
	mustGit(t, f.store, "add", "-A", f.home)
	mustGit(t, f.store, "commit", "-q", "-m", "seed the store", "--", f.home)
	// A commit object main does not name, minted with `commit-tree` so no
	// ref moves: `update-ref refs/heads/main` is what the ref row tries,
	// and git refuses a branch pointed at anything but a commit — so
	// without this the CONTROL fails on git's own rule and the row cannot
	// be told from the wall.
	f.other = strings.TrimSpace(mustGit(t, f.store, "commit-tree", "-m", "a seat-built commit",
		strings.TrimSpace(mustGit(t, f.store, "rev-parse", "HEAD^{tree}"))))

	sbMkdir(t, filepath.Join(f.f.tree, beadsDirName))
	sbWrite(t, filepath.Join(f.f.tree, beadsDirName, beadsRedirect), f.home+"\n")
	if got := beadsHome(f.f.tree); got != f.home {
		t.Fatalf("the redirect does not reach the store, so every row here measures nothing: beadsHome = %q, want %q", got, f.home)
	}

	// The seat's own object store, so this is the wall a dispatched seat
	// really gets (ADR 0099 D1) and not a reduced one.
	if err := prepareSeatStore(&SessionTree{Path: f.f.tree, Repo: f.f.repo}); err != nil {
		t.Fatalf("the walled prepare refused the fixture's tree: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("no home for the fixture, so no seat wall dir and no store: %v", err)
	}
	binds, err := seatStoreBinds(f.f.ag, home, f.f.tree)
	if err != nil {
		t.Fatal(err)
	}

	set := f.f.a.SeatbeltWritable(f.f.ag, f.f.tree, f.f.gates)
	// Non-vacuity, before a single row runs: the producer the grant came
	// from still names this repo's git dir, so a wall that refuses below is
	// refusing something that used to be granted.
	if !bgHas(beadsGitDirs(f.home), f.gitDir) {
		t.Fatalf("beadsGitDirs no longer names %s for this store, so these rows are not over ranger-base-kfrpv's defect: %v", f.gitDir, beadsGitDirs(f.home))
	}
	f.argv = f.render(t, set, binds)
	f.argvBefore = f.render(t, append(append([]string{}, set...), beadsGitDirs(f.home)...), binds)
	return f
}

func (f srLive) render(t *testing.T, set []string, binds []PrivateBind) []string {
	t.Helper()
	bset, keep := BwrapWritable(f.f.ag, set, f.f.tree)
	argv, prep, err := BwrapArgs(f.f.tree, bset, SeatbeltCarveOut{Private: binds}, keep...)
	if err != nil {
		t.Fatalf("BwrapArgs refused this render: %v", err)
	}
	// runBwrapPrep and not a copy of its switch, for bsLive.render's reason:
	// a reduced launcher measures a wall the launcher does not put up.
	if err := runBwrapPrep(prep); err != nil {
		t.Fatalf("prep: %v", err)
	}
	return argv
}

func (f srLive) in(t *testing.T, argv []string, stdin string, arg ...string) (bool, string) {
	t.Helper()
	cmd := exec.Command("bwrap", append(append([]string{}, argv...), arg...)...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}

// storeGit is a git invocation aimed at the STORE repo, with an identity so
// a commit inside the wall is never refused for want of one.
func (f srLive) storeGit(arg ...string) []string {
	return append([]string{"git", "-c", "user.name=p", "-c", "user.email=p@example.com", "-C", f.store}, arg...)
}

// THE GRANT THAT MUST SURVIVE: a seat records in the store of record. Its
// own row, because a wall that takes this with it is the failure
// ranger-base-rhw and ranger-base-0fb are both about, and it would make
// every refusal below true for the wrong reason.
func TestLiveBwrapStoreRepoRecordStageStillReaches(t *testing.T) {
	f := srLiveFixture(t)
	jsonl := filepath.Join(f.home, "issues.jsonl")
	if ok, out := f.in(t, f.argv, "", "sh", "-c", "echo '{\"id\":\"q-2\"}' >> "+jsonl); !ok {
		t.Fatalf("the seat cannot append to the store of record, so the wall this bead adds costs the record stage:\n%s", out)
	}
	if b, err := os.ReadFile(jsonl); err != nil || !strings.Contains(string(b), "q-2") {
		t.Fatalf("the seat's append did not reach the host's store: %q %v", string(b), err)
	}
	if ok, out := f.in(t, f.argv, "", "sh", "-c", "touch "+filepath.Join(f.home, "beads.db")); !ok {
		t.Errorf("the seat cannot write the store's database:\n%s", out)
	}
}

// …and nothing of that repo's git: one row per thing the verified hole made
// writable, each refused under the shipped render, succeeding under the
// pre-fix one, and read back from outside the wall.
func TestLiveBwrapStoreRepoGitIsReadOnlyToTheSeat(t *testing.T) {
	rows := []struct {
		name string
		// arg is built from the fixture, since each row gets a fresh one.
		arg   func(f srLive) []string
		stdin string
	}{
		// ADR 0099 D1's own shape, for the store repo: the object files an
		// uncaged reader trusts by name, because git never re-hashes on read.
		{name: "unlink an object the store's history names",
			arg: func(f srLive) []string { return []string{"rm", "-f", "--", f.objectFile(nil)} }},
		{name: "rewrite that object in place",
			arg: func(f srLive) []string {
				p := f.objectFile(nil)
				return []string{"sh", "-c", "chmod u+w " + p + " && printf evil > " + p}
			}},
		{name: "write a NEW object into the store repo",
			stdin: "a seat-chosen object\n",
			arg:   func(f srLive) []string { return f.storeGit("hash-object", "-w", "--stdin") }},
		// The ref the operator's next push sends.
		{name: "move refs/heads/main",
			arg: func(f srLive) []string { return f.storeGit("update-ref", "refs/heads/main", f.other) }},
		// The two that redirect an UNCAGED reader's resolution.
		{name: "plant objects/info/alternates",
			arg: func(f srLive) []string {
				return []string{"sh", "-c", "printf /tmp > " + filepath.Join(f.gitDir, "objects", "info", "alternates")}
			}},
		{name: "plant a refs/replace entry",
			arg: func(f srLive) []string { return f.storeGit("update-ref", "refs/replace/"+f.headSha(), f.other) }},
		// And the whole of what the grant was for.
		{name: "commit in the store repo",
			arg: func(f srLive) []string { return f.storeGit("commit", "-q", "--allow-empty", "-m", "a seat commit") }},
	}

	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			f := srLiveFixture(t)
			objFile := f.objectFile(t)
			objBefore, err := os.ReadFile(objFile)
			if err != nil {
				t.Fatalf("the store repo's tree object is not a loose file, so this row measures nothing: %v", err)
			}
			head := f.headSha()
			alternates := filepath.Join(f.gitDir, "objects", "info", "alternates")

			if ok, out := f.in(t, f.argv, r.stdin, r.arg(f)...); ok {
				t.Errorf("a seat could %s — that is the hole ranger-base-kfrpv closes:\n%s", r.name, out)
			}
			// Read back from outside: the object the operator's git serves,
			// the ref it reads, and the two resolution files.
			if got, err := os.ReadFile(objFile); err != nil || string(got) != string(objBefore) {
				t.Errorf("the store repo's object changed under the wall (%v)", err)
			}
			if got := f.headSha(); got != head {
				t.Errorf("the store repo's HEAD moved under the wall: %s -> %s", head[:12], got[:12])
			}
			if _, err := os.Stat(alternates); err == nil {
				t.Error("objects/info/alternates exists — an uncaged git would read objects from a path the seat chose")
			}
			if ents, err := os.ReadDir(filepath.Join(f.gitDir, "refs", "replace")); err == nil && len(ents) > 0 {
				t.Errorf("refs/replace is not empty — an uncaged `git log` would serve substituted content: %d entry(ies)", len(ents))
			}
			if got := mustGit(t, f.store, "cat-file", "-t", f.treeSha()); got != "tree" {
				t.Errorf("the operator's cat-file no longer serves the tree object: %q", got)
			}

			// THE CONTROL, on the SAME fixture and last, because it really
			// does the thing: under the render this bead replaced the row
			// succeeds, so the refusal above is this wall and not the
			// fixture or a malformed command.
			if ok, out := f.in(t, f.argvBefore, r.stdin, r.arg(f)...); !ok {
				t.Errorf("CONTROL: the pre-fix render refused it too, so this row cannot be told from an unwritable fixture:\n%s", out)
			}
		})
	}
}

// objectFile is the loose file of the store repo's HEAD tree — an object its
// history names, which is the class ADR 0099 D1 is about. t may be nil,
// where the caller only needs the path.
func (f srLive) objectFile(t *testing.T) string {
	sha := f.treeSha()
	return filepath.Join(f.gitDir, "objects", sha[:2], sha[2:])
}

func (f srLive) treeSha() string {
	out, err := git(f.store, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (f srLive) headSha() string {
	out, err := git(f.store, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
