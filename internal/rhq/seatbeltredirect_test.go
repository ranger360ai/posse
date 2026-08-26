package rhq

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sbHas(w []string, p string) bool {
	want := absResolve(p)
	for _, x := range w {
		if x == want {
			return true
		}
	}
	return false
}

// L2's writable set has to follow .beads/redirect for the same reason
// codex's --add-dir does (ranger-base-0fb, and rhw for this tier): under ADR
// 0012 D3-C the session dir's .beads holds a path, and the database, its
// jsonl and the git dir that lands them are in another repo. Granting
// cwd/.beads there grants a directory bd never writes, and every mutation —
// `bd sync`, `bd export`, the commit of the jsonl — is denied by the
// profile.
func TestSeatbeltFollowsTheBeadsRedirect(t *testing.T) {
	store := blRepo(t)
	work := blRepo(t)
	blRedirect(t, work, filepath.Join(store, beadsDirName))
	gates := t.TempDir()

	denied := &AgentFile{Name: "hoover", Deny: []string{"Edit", "Write"}, MemoryDir: t.TempDir()}
	w := SeatbeltWritable(denied, work, gates)
	for _, want := range []string{
		filepath.Join(store, beadsDirName), // the database, jsonl, socket, lock
		filepath.Join(store, ".git"),       // index.lock for `bd sync`'s commit
		filepath.Join(work, beadsDirName),  // the redirect file's own dir
		filepath.Join(work, ".git"),
	} {
		if !sbHas(w, want) {
			t.Errorf("writable set missing %q:\n%s", want, strings.Join(w, "\n"))
		}
	}
	// The grant is the store of record, not the instance repo: everything
	// else in that tree stays as unwritable as the tier promises.
	if sbHas(w, store) {
		t.Errorf("the redirect target's repo root must not be writable:\n%s", strings.Join(w, "\n"))
	}
	if prof := SeatbeltProfile("hoover", w); !strings.Contains(prof, `(subpath "`+absResolve(filepath.Join(store, beadsDirName))+`")`) {
		t.Errorf("profile must grant the resolved target:\n%s", prof)
	}

	// A persona that may edit the repo is in the same bind: cwd is granted
	// whole and the store of record is still in another tree.
	open := &AgentFile{Name: "dev", MemoryDir: t.TempDir()}
	wo := SeatbeltWritable(open, work, gates)
	for _, want := range []string{work, filepath.Join(store, beadsDirName), filepath.Join(store, ".git")} {
		if !sbHas(wo, want) {
			t.Errorf("writable set missing %q:\n%s", want, strings.Join(wo, "\n"))
		}
	}
}

// The pre-cut-over shape is untouched: with no redirect the beads dir is
// under cwd already, so nothing new is granted and nothing is granted twice.
func TestSeatbeltWithoutARedirectGrantsNothingExtra(t *testing.T) {
	work := blRepo(t)
	gates := t.TempDir()
	ag := &AgentFile{Name: "hoover", Deny: []string{"Edit", "Write"}, MemoryDir: t.TempDir()}
	w := SeatbeltWritable(ag, work, gates)
	var rooted []string
	for _, x := range w {
		if underDir(work, x) {
			rooted = append(rooted, x)
		}
	}
	want := []string{absResolve(filepath.Join(work, beadsDirName)), absResolve(filepath.Join(work, ".git"))}
	if strings.Join(rooted, "\n") != strings.Join(want, "\n") {
		t.Errorf("only .beads and .git may be granted under cwd:\ngot  %v\nwant %v", rooted, want)
	}
}

// A redirect that resolves nowhere leaves bd reading the local .beads, so
// the profile must not grant a directory bd is not using either.
func TestSeatbeltIgnoresADanglingRedirect(t *testing.T) {
	work := blRepo(t)
	gone := filepath.Join(t.TempDir(), "gone", beadsDirName)
	blRedirect(t, work, gone)
	w := SeatbeltWritable(&AgentFile{Name: "hoover", Deny: []string{"Edit", "Write"}}, work, t.TempDir())
	if sbHas(w, gone) || sbHas(w, filepath.Dir(gone)) {
		t.Errorf("a dangling redirect must grant nothing:\n%s", strings.Join(w, "\n"))
	}
}

// A worktree's .git is a file: the dir a commit locks and the dir the hooks
// live in are two paths, and only `git rev-parse` can name them. `bd
// worktree create` writes exactly this shape, so the redirect can land in
// one.
func TestBeadsGitDirsNamesBothWorktreeGitDirs(t *testing.T) {
	store := blRepo(t)
	blCommit(t, store, "seed", blLine("q-1", "open"))
	wt := filepath.Join(t.TempDir(), "wt")
	if out, err := exec.Command("git", "-C", store, "worktree", "add", "-q", wt).CombinedOutput(); err != nil {
		t.Skipf("git worktree add: %v %s", err, out)
	}
	os.MkdirAll(filepath.Join(wt, beadsDirName), 0o755)
	got := beadsGitDirs(filepath.Join(wt, beadsDirName))
	for _, want := range []string{
		filepath.Join(store, ".git", "worktrees", "wt"), // index.lock lands here
		filepath.Join(store, ".git"),                    // hooks and refs
	} {
		if !sbHas(got, want) {
			t.Errorf("beadsGitDirs missing %q: %v", want, got)
		}
	}
}

// A target git cannot answer for still gets the plain grant — a beads dir
// restored from a backup, or one whose repo is not initialized yet.
func TestBeadsGitDirsFallsBackToRepoDotGit(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, beadsDirName)
	os.MkdirAll(home, 0o755)
	got := beadsGitDirs(home)
	if len(got) != 1 || got[0] != filepath.Join(root, ".git") {
		t.Errorf("non-repo target: got %v, want [%s]", got, filepath.Join(root, ".git"))
	}
}

// MEASURED against bd 0.49.1, and it is the one thing beadsHome and bd
// disagree about: bd REFUSES a chain. Point work -> mid -> store and bd
// prints "redirect chains not allowed, ignoring redirect in <mid>/.beads"
// and opens the database in MID, the first hop. beadsHome follows up to
// eight hops and answers STORE. The writable set is then wrong in both
// directions at once — it grants a directory bd never opens and denies the
// one it does — and a caged persona gets the original defect back verbatim:
//
//	bd sync   -> failed to open database: ... <mid>/.beads/beads.db:
//	             operation not permitted
//	bd export -> the same line
//
// Not live in this fleet: seedBeadsRedirect (worktree.go) resolves the main
// checkout's redirect before writing the worktree's, "so a chain is never
// built". The bound in beadsHome exists for a shape posse does not create
// and bd will not read. The same resolver backs the beadloss census, which
// would walk a repo bd is not using — the alarm disarmed without a word.
func TestSeatbeltGrantsTheHopBdActuallyStopsAt(t *testing.T) {
	t.Skip("ranger-base-f5dg: beadsHome follows chains bd refuses; grant is the wrong hop")
	work, mid, store := blRepo(t), blRepo(t), blRepo(t)
	blRedirect(t, work, filepath.Join(mid, beadsDirName))
	blRedirect(t, mid, filepath.Join(store, beadsDirName))

	w := SeatbeltWritable(&AgentFile{Name: "hoover", Deny: []string{"Edit", "Write"}}, work, t.TempDir())
	if !sbHas(w, filepath.Join(mid, beadsDirName)) {
		t.Errorf("bd opens the FIRST hop's database; the profile must grant it:\n%s", strings.Join(w, "\n"))
	}
	if sbHas(w, filepath.Join(store, beadsDirName)) {
		t.Errorf("bd never opens the chain's end; granting it widens the cage for nothing:\n%s", strings.Join(w, "\n"))
	}
}

// A redirect that stays INSIDE cwd but does not name cwd/.beads. The grant
// is guarded by `!underDir(cwd, home)` — "the store of record is not under
// cwd" — but for a persona that denies Edit/Write cwd is NOT granted: only
// cwd/.beads and cwd/.git are. So "under cwd" is the wrong boundary in that
// branch, the target is skipped as already-covered when nothing covers it,
// and bd is denied its own database. Measured with bd 0.49.1 in
// ~/laurie-cage-probe/work, redirect `inner/.beads` (bd resolves the
// relative form against the repo root — verified, it built the db there):
//
//	bd sync   -> failed to open database: ... work/inner/.beads/beads.db:
//	             operation not permitted
//	bd export -> the same line
//
// The open-repo persona is fine here — cwd whole covers it — which is the
// mirror image of the bug this file's first test pins.
func TestSeatbeltGrantsARedirectThatStaysUnderCwd(t *testing.T) {
	t.Skip("ranger-base-f5dg: underDir(cwd) is the wrong boundary when only cwd/.beads is granted")
	work := blRepo(t)
	inner := filepath.Join(work, "inner")
	if err := os.MkdirAll(filepath.Join(inner, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	blRedirect(t, work, filepath.Join(inner, beadsDirName))

	w := SeatbeltWritable(&AgentFile{Name: "hoover", Deny: []string{"Edit", "Write"}}, work, t.TempDir())
	if !sbHas(w, filepath.Join(inner, beadsDirName)) {
		t.Errorf("the redirect target is where bd opens the db, under cwd or not:\n%s", strings.Join(w, "\n"))
	}
}
