package posse

// Helpers lifted out of worktree_qa_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wtqaRepo is a real git repo that is also a fake-bd beads repo: the two
// substrates a dispatched session sits between.
func wtqaRepo(t *testing.T, a *App, ready, show string) string {
	t.Helper()
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "fake-ready.json"), ready)
	if show != "" {
		write(t, filepath.Join(repo, "fake-show.json"), show)
	}
	write(t, a.ConfigPath, "beads:\n  - "+repo+"\n")
	return repo
}

// wtqaHome points $HOME at a temp dir. It used to be how every test in this
// file kept session worktrees out of the operator's real ~/.posse, and it
// held 137 tests serial for it; ADR 0047 now gives that guarantee twice over
// without touching the environment — TestMain hands the binary a temp $HOME,
// and hermetic() puts each App's default worktree root at
// $HOME/worktrees/<t.Name()>. So 48 calls came out (ranger-base-pj87l) and
// the two left are the ones that want a $HOME of their very own for a reason
// the worktree root does not cover: sbRoot, whose sandbox pins create and
// remove operator-owned paths OFF that home (~/.claude.json, ~/Library/...),
// and one init test that is env-tainted anyway.
func wtqaHome(t *testing.T) { t.Helper(); t.Setenv("HOME", t.TempDir()) }

// wtqaPassWithWork runs a pass over one closed bead, having pre-made the
// session's tree and put a commit on its branch — which is what the pass
// finds when a persona really did the work, since EnsureSessionTree is
// idempotent and the pass lands in the tree that is already there.
func wtqaPassWithWork(t *testing.T, extra func(repo, tree string)) (*Dispatcher, string, string) {
	t.Helper()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	// The merge-back rung says half of what it says on errw (a handoff that
	// could not be filed is an error, not a pass line), so the pass's stderr
	// is captured rather than left to leak into the suite's own — where a
	// non-verbose `go test` prints it above some other package's failure.
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")
	if extra != nil {
		extra(repo, tr.Path)
	}
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	return d, repo, tr.Path
}

// mergeBlockedID is the id the fake bd handed the merge-back create, read
// out of the store the create landed in — so the pin names the bead that
// exists rather than the one the id counter happens to be on.
func mergeBlockedID(t *testing.T, repo string) string {
	t.Helper()
	var list []struct {
		ID    string   `json:"id"`
		Title string   `json:"title"`
		Label []string `json:"labels"`
	}
	b, err := os.ReadFile(filepath.Join(repo, "fake-list-labeled.json"))
	if err != nil {
		t.Fatalf("no labeled listing: %v", err)
	}
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	for _, is := range list {
		if strings.HasPrefix(is.Title, "merge-back blocked: ") {
			return is.ID
		}
	}
	t.Fatalf("bd committed no merge-back bead:\n%s", b)
	return ""
}

// wtqaErr is what the pass wrote to errw — the writer dispatcherErr attached
// in wtqaPassWithWork. The merge-back rung reports a handoff it could NOT
// file there, so a pin that reads only d.Out cannot see it.
func wtqaErr(d *Dispatcher) string { return d.Err.(*strings.Builder).String() }
