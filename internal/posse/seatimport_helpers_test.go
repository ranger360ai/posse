package posse

// Fixtures for ranger-base-xcucy's seat-store import, shared by every suite
// arm: seatimport_qa_test.go (arm 1) pins the import, and mergeBlockedCases
// (mergeblocked_test.go, arm 2) drives the refusals it adds.
//
// The seat store is SIMULATED, as the design's probe did (ranger-base-dvw0w):
// the seat's git runs with GIT_OBJECT_DIRECTORY=<own git dir>/seat-objects,
// whose info/alternates names the shared store.

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// siTree is a session tree on a detached HEAD, as every walled seat is
// (ranger-base-pe5g5), with a seat store made and its alternates pointing at
// the shared store.
func siTree(t *testing.T) (a *App, repo string, tr *SessionTree, store string) {
	t.Helper()
	a = wtApp(t)
	repo = wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareSessionHead(tr, true, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	store = siStoreFor(t, tr, "")
	return a, repo, tr, store
}

// siStoreFor makes a seat store for tr — its own (at the name the import
// reads) when at is "", or a store somewhere the import never looks.
func siStoreFor(t *testing.T, tr *SessionTree, at string) string {
	t.Helper()
	dirs := LinkedGitDirs(tr.Path)
	if len(dirs) != 2 {
		t.Fatalf("not a linked worktree: %v", dirs)
	}
	store := at
	if store == "" {
		store = filepath.Join(dirs[0], seatStoreName)
	}
	write(t, filepath.Join(store, "info", "alternates"), filepath.Join(dirs[1], "objects")+"\n")
	mustGit(t, tr.Path, "config", "user.email", "p@example.com")
	mustGit(t, tr.Path, "config", "user.name", "p")
	return store
}

// siSeatGit is git as the walled seat runs it: every object it writes goes
// into the seat store.
func siSeatGit(t *testing.T, dir, store string, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OBJECT_DIRECTORY="+store)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("seat git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// siSeatCommit commits one file the way the seat would.
func siSeatCommit(t *testing.T, tr *SessionTree, store, path, body, msg string) string {
	t.Helper()
	write(t, filepath.Join(tr.Path, path), body)
	siSeatGit(t, tr.Path, store, "", "add", "--", path)
	siSeatGit(t, tr.Path, store, "", "commit", "-q", "-m", msg, "--", path)
	return siSeatGit(t, tr.Path, store, "", "rev-parse", "HEAD")
}

// siHas is whether the SHARED store holds name, asked uncaged.
func siHas(t *testing.T, repo, name string) bool {
	t.Helper()
	_, err := git(repo, "cat-file", "-e", name)
	return err == nil
}

// siPlant writes body as a blob into the seat store and then MOVES its file
// to the loose path of claimed: a forged object, bytes that do not hash to
// their name. Returns the blob's true name.
func siPlant(t *testing.T, tr *SessionTree, store, body, claimed string) string {
	t.Helper()
	truth := siSeatGit(t, tr.Path, store, body, "hash-object", "-w", "--stdin")
	from := filepath.Join(store, truth[:2], truth[2:])
	to := filepath.Join(store, claimed[:2], claimed[2:])
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	return truth
}

// siBadObject plants a loose file that is not an object at all, under a name
// nobody holds: a store that will not import. Returns its path.
func siBadObject(t *testing.T, store, hexPair string) string {
	t.Helper()
	bad := strings.Repeat(hexPair, 20)
	p := filepath.Join(store, bad[:2], bad[2:])
	write(t, p, "not an object")
	return p
}

// siGateFixture is a session tree (attached, no seat store of its own) whose
// branch has two commits that are both in the shared store, while a blob the
// FIRST adds and the second deletes is not: the shape of a store whose import
// did not happen, reached by no guard but the landing gate — `merge
// --ff-only` reads only the tip's tree, so it fast-forwards over this one.
// Returns the missing blob.
func siGateFixture(t *testing.T) (a *App, repo string, tr *SessionTree, blob string) {
	t.Helper()
	a = wtApp(t)
	repo = wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	store := siStoreFor(t, tr, filepath.Join(t.TempDir(), "elsewhere"))
	siSeatCommit(t, tr, store, "gone.txt", "bytes nobody has\n", "s-1: add")
	blob = siSeatGit(t, tr.Path, store, "", "rev-parse", "HEAD:gone.txt")
	siSeatGit(t, tr.Path, store, "", "rm", "-q", "--", "gone.txt")
	write(t, filepath.Join(tr.Path, "keep.txt"), "kept\n")
	siSeatGit(t, tr.Path, store, "", "add", "--", "keep.txt")
	siSeatGit(t, tr.Path, store, "", "commit", "-q", "-m", "s-1: drop", "--", "gone.txt", "keep.txt")
	head := siSeatGit(t, tr.Path, store, "", "rev-parse", "HEAD")
	objects := filepath.Join(LinkedGitDirs(tr.Path)[1], "objects")
	// Everything the seat wrote, into the shared store, except that blob.
	err = filepath.WalkDir(store, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(filepath.Dir(p)) == "info" {
			return err
		}
		name := filepath.Base(filepath.Dir(p)) + d.Name()
		if name == blob {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		write(t, filepath.Join(objects, name[:2], name[2:]), string(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if siHas(t, repo, blob) || !siHas(t, repo, head) || !siHas(t, repo, head+"~1") {
		t.Fatal("CONTROL: the fixture is not 'both commits present, the intermediate blob missing'")
	}
	return a, repo, tr, blob
}

func siHashOf(t *testing.T, repo, body string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "hash-object", "--stdin")
	cmd.Stdin = strings.NewReader(body)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func siWriteAndRead(t *testing.T, repo, body string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "hash-object", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(body)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	name := strings.TrimSpace(string(out))
	got, err := exec.Command("git", "-C", repo, "cat-file", "-p", name).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}
