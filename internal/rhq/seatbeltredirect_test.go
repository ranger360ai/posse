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
