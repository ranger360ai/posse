//go:build posse_arm3

package posse

// The fixture `posse gates adr-census` is measured over: a walled repo whose
// object store holds one of every shape ADR 0051's predicate has to tell
// apart, built with real git so nothing here is asserted from how it was
// made. The pins that use it are in adrcensus_qa_test.go (the audit) and
// adrcitationgate_qa_test.go (the commit path, which must carry no citation
// check at all).
//
// IT WAS A COMMIT GATE ONCE. Until operator ruling 2026-09-05 this file also
// carried ten cells measuring a prepare-commit-msg arm that refused a staged
// docs/adr line naming a sha off the base branch. ADR 0051's simplification
// removed that arm (ranger-base-bp0yj) and those cells went with it; the
// fixture stayed, because the audit that survives judges exactly the same
// shapes and needs exactly the same objects to judge them over.
//
// WHAT THE FIXTURE HAS TO CONTAIN, and why each one is not decoration:
//
//   - an ancestor of the base branch, and a NON-ancestor with no twin — the
//     two halves of the verdict;
//   - a patch-id TWIN PAIR built the way the launcher builds one (the same
//     content change committed twice from the same parent tree, once on a
//     side branch and once on the base), because the admission this audit
//     grants is exactly that shape, and a second pair so a first-match-only
//     bug has somewhere to show;
//   - a non-ancestor with an EMPTY patch-id beside the repo's own root
//     commit, which is an ancestor with an empty patch-id: two empties
//     compare equal, so this pair is the escape the emptiness guard exists
//     for and the one shape that is free to type once known.
//
// Every one of those claims is re-measured with git at the end of
// newAdrRepo, so a fixture that stops having the shape says so instead of
// passing quietly.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// adrStamp is what a writer actually types: a status line with a sha and a
// bead id in it. One line, so a cell can swap the token and change nothing
// else.
func adrStamp(tok string) string {
	return "# ADR 0999 — a fixture\n\n*Status: accepted (ranger-base-glewr) · landed " + tok + "*\n"
}

// adrRepo is the fixture: a walled repo whose object store holds one of
// every shape the predicate has to tell apart.
type adrRepo struct {
	dir     string
	git     func(env []string, args ...string) (string, error)
	persona []string
	gates   string

	root        string // the repo's ROOT commit: an ancestor whose patch-id is EMPTY
	landed      string // an ordinary ancestor, non-empty patch-id, twin of nothing stale
	stale       string // a non-ancestor with no twin anywhere on the base branch
	twinStale   string // a non-ancestor whose patch-id twin IS on the base branch
	twinLanded  string // that twin
	twin2Stale  string // a second pair, so a table has two rows and a
	twin2Landed string // first-match-only bug has somewhere to show
	emptyStale  string // a non-ancestor with an EMPTY patch-id
}

// newAdrRepo builds the fixture. The twin pairs are made the way the
// launcher makes them for real: the same content change committed twice from
// the same parent tree, once on a side branch and once on the base branch.
// Nothing here is asserted from the shape — every claim the pins rest on is
// re-measured with git at the end.
func newAdrRepo(t *testing.T) *adrRepo {
	t.Helper()
	dir, git, persona := commitWallRepo(t)
	r := &adrRepo{dir: dir, git: git, persona: persona, gates: adrGatesDir(persona)}

	r.root = r.commit(t, "seed.txt", "seed\n", "fixture seed")
	r.landed = r.commit(t, "base.txt", "base\n", "fixture base")

	// A non-ancestor with no twin: content nothing on the base branch has.
	r.branch(t, "side")
	r.stale = r.commit(t, "side.txt", "side\n", "fixture side")
	r.checkout(t, "main")

	r.twinStale, r.twinLanded = r.twinPair(t, "t1", "twin one\n", "side-t1")
	r.twin2Stale, r.twin2Landed = r.twinPair(t, "t2", "twin two\n", "side-t2")

	// A non-ancestor with an EMPTY patch-id: a commit whose tree is its
	// parent's. commit-tree rather than `git commit --allow-empty`, which
	// this session's PID denies in every tree.
	tree, err := git(nil, "rev-parse", "side^{tree}")
	if err != nil {
		t.Fatalf("rev-parse side^{tree}: %v %s", err, tree)
	}
	head, err := git(nil, "rev-parse", "side")
	if err != nil {
		t.Fatalf("rev-parse side: %v %s", err, head)
	}
	empty, err := git(nil, "commit-tree", strings.TrimSpace(tree), "-p", strings.TrimSpace(head), "-m", "no diff of its own")
	if err != nil {
		t.Fatalf("commit-tree: %v %s", err, empty)
	}
	r.emptyStale = r.short(t, strings.TrimSpace(empty))

	// Every claim the cells below rest on, measured here rather than assumed
	// from how the fixture was built.
	for _, a := range []string{r.root, r.landed, r.twinLanded, r.twin2Landed} {
		if _, err := git(nil, "merge-base", "--is-ancestor", a, "refs/heads/main"); err != nil {
			t.Fatalf("fixture: %s must be an ancestor of main", a)
		}
	}
	for _, n := range []string{r.stale, r.twinStale, r.twin2Stale, r.emptyStale} {
		if _, err := git(nil, "merge-base", "--is-ancestor", n, "refs/heads/main"); err == nil {
			t.Fatalf("fixture: %s must NOT be an ancestor of main", n)
		}
	}
	if r.patchID(t, r.twinStale) != r.patchID(t, r.twinLanded) || r.patchID(t, r.twinStale) == "" {
		t.Fatalf("fixture: %s and %s must be a non-empty patch-id pair", r.twinStale, r.twinLanded)
	}
	if r.patchID(t, r.twin2Stale) != r.patchID(t, r.twin2Landed) {
		t.Fatalf("fixture: %s and %s must be a patch-id pair", r.twin2Stale, r.twin2Landed)
	}
	if r.patchID(t, r.stale) == r.patchID(t, r.landed) {
		t.Fatalf("fixture: %s must NOT be a twin of %s", r.stale, r.landed)
	}
	// The trap the empty guard exists for: a root commit and a commit with
	// no diff of its own have the SAME (empty) patch-id, so an unguarded
	// reader admits the second beside the first.
	if r.patchID(t, r.root) != "" || r.patchID(t, r.emptyStale) != "" {
		t.Fatalf("fixture: %s and %s must both have an EMPTY patch-id — that is the escape under test", r.root, r.emptyStale)
	}
	return r
}

func (r *adrRepo) short(t *testing.T, rev string) string {
	t.Helper()
	out, err := r.git(nil, "rev-parse", "--short", rev)
	if err != nil {
		t.Fatalf("rev-parse --short %s: %v %s", rev, err, out)
	}
	return strings.TrimSpace(out)
}

// patchID is the reference predicate's own pair, run out of process the way
// the hook and the census both run it — a fixture that computed this any
// other way would be measuring a different question.
func (r *adrRepo) patchID(t *testing.T, rev string) string {
	t.Helper()
	patch, err := r.git(nil, "diff-tree", "-p", rev)
	if err != nil {
		t.Fatalf("diff-tree -p %s: %v %s", rev, err, patch)
	}
	cmd := exec.Command("git", "-C", r.dir, "patch-id", "--stable")
	cmd.Stdin = strings.NewReader(patch)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("patch-id: %v", err)
	}
	return strings.TrimSpace(strings.SplitN(strings.TrimSpace(string(out)), " ", 2)[0])
}

// commit stages one path and commits it with the message given. THE MESSAGE
// IS A PARAMETER AND NOT DERIVED FROM THE PATH, and that is load-bearing: a
// commit object is content-addressed, so the twin pair below — same tree,
// same parent, same author and committer, same second — collapsed to ONE
// SHA when both halves also carried the same message, and the fixture's
// "stale" and "landed" were literally the same commit (measured; the
// ancestry assertion caught it).
func (r *adrRepo) commit(t *testing.T, rel, body, msg string) string {
	t.Helper()
	adrStage(t, r.dir, r.git, nil, rel, body)
	if out, err := r.git(nil, "commit", "-qm", msg, "--", rel); err != nil {
		t.Fatalf("fixture commit %s: %v %s", rel, err, out)
	}
	return r.short(t, "HEAD")
}

func (r *adrRepo) branch(t *testing.T, name string) {
	t.Helper()
	if out, err := r.git(nil, "checkout", "-q", "-b", name); err != nil {
		t.Fatalf("git checkout -b %s: %v %s", name, err, out)
	}
}

func (r *adrRepo) checkout(t *testing.T, name string) {
	t.Helper()
	if out, err := r.git(nil, "checkout", "-q", name); err != nil {
		t.Fatalf("git checkout %s: %v %s", name, err, out)
	}
}

// twinPair makes the launcher's own shape: one content change committed from
// the same parent tree twice, once on a side branch (the sha a persona would
// read out of its session tree) and once on main (the sha the rebase minted).
func (r *adrRepo) twinPair(t *testing.T, file, body, branch string) (stale, landed string) {
	t.Helper()
	r.branch(t, branch)
	stale = r.commit(t, file+".txt", body, "in a session tree ("+file+")")
	r.checkout(t, "main")
	landed = r.commit(t, file+".txt", body, "rebased and landed ("+file+")")
	return stale, landed
}

func (r *adrRepo) stage(t *testing.T, rel, body string) {
	t.Helper()
	adrStage(t, r.dir, r.git, r.persona, rel, body)
}

// commitPath is the ordinary path-limited commit a persona makes, under a
// persona env (RHQ_PERSONA set), through the real installed commit wall.
func (r *adrRepo) commitPath(t *testing.T, rel string) (string, error) {
	t.Helper()
	return r.git(r.persona, "commit", "-m", "adr", "--", rel)
}

// adrStage writes body at rel (creating parents) and stages exactly that
// path — a new file needs the add before a path-limited commit can match it
// (rangerhq-4pbt), and the add is scoped to the one path.
func adrStage(t *testing.T, repo string, git func(env []string, args ...string) (string, error), env []string, rel, body string) {
	t.Helper()
	full := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := git(env, "add", "--", rel); err != nil {
		t.Fatalf("git add %s: %v %s", rel, err, out)
	}
}

func adrGatesDir(persona []string) string {
	for _, kv := range persona {
		if strings.HasPrefix(kv, "RHQ_GATES_DIR=") {
			return strings.TrimPrefix(kv, "RHQ_GATES_DIR=")
		}
	}
	return ""
}
