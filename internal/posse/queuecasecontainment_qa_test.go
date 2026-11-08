//go:build posse_arm3

package posse

// QA pins for ranger-base-oczuq — the queue repo contains its store by
// IDENTITY, not by string.
//
// THE DEFECT. CommitQueueJSONL asks "is the store inside `queue_repo:`?"
// before it commits, and it asked with underDir, which compares the two
// resolved spellings using filepath.Rel. The two spellings are independent:
// `q` is whatever the operator typed under `queue_repo:`, `store` is
// whatever the session's `.beads/redirect` names. MEASURED 2026-11-08
// (go1.26.5 darwin/arm64, APFS case-insensitive, scratch `real/Repo/.beads`):
// with `queue_repo:` spelled `real/REPO`, absResolve keeps both cases —
// EvalSymlinks does not fold case — Rel answers `../Repo/.beads`, and the
// close returned Skipped "<store> is not inside <q>". A skip there is a
// close whose record never reaches the queue repo's git history, which is
// the bead-loss census's only input (beadloss.go). Silent: the pass prints
// the skip and nothing is red.
//
// THE FIX is underSameDir (visibility.go) — the string answer first,
// unchanged, then a climb up the store's ancestors asking samePath, which
// stats. ADR 0082 D4's closing paragraph prescribes exactly that and rules
// out the two alternatives: `QueueRepo()` must not canonicalise (a VALUE
// that feeds comparisons), and underDir's own sides must not either (the
// seatbelt callers' subject is the spelling the sandbox matches).
//
// WHAT EACH PIN IS FOR. The first two are the bead's DONE WHEN, in both
// directions, because an identity walk that answered true unconditionally
// would satisfy the first alone. The third holds underDir's own semantics
// where ~25 grant callers rely on them, so a future pass that "fixes"
// containment in one place reds here instead of widening a sandbox grant.
// The fourth is the sibling-namespace shape identity must not reach
// (ranger-base-cypy1).
//
// MUTATION-CHECKED 2026-11-08: reverting queuejsonl.go's call to underDir
// reds TestQueueCommitLandsThroughACaseChangedQueueRepo alone;
// making underSameDir return underDir's answer reds that one and
// TestUnderSameDirAnswersContainmentByIdentity; making it return true
// reds TestQueueCommitStillRefusesACaseChangedStranger and the
// sibling-namespace arm.
//
// THE SKIP. Every case arm needs a case-INsensitive volume, and whether a
// mount is one is a per-mount question neither statfs nor a filesystem name
// answers (samePath's note) — so each asks the filesystem directly, by
// stat'ing the case-changed spelling and comparing identity, and skips when
// the two are genuinely two directories. On a case-sensitive volume there
// is no defect to measure here.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qcaseRepo is qRepo one directory down, under a name this test spells
// itself — t.TempDir's own leaf is not ours to re-case, and the defect needs
// two spellings of one directory.
func qcaseRepo(t *testing.T, name string) string {
	t.Helper()
	repo := filepath.Join(gitTempDir(t), name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, ".beads", beadsJSONL), `{"id":"q-1","title":"seed"}`+"\n")
	mustGit(t, repo, "add", filepath.Join(".beads", beadsJSONL))
	mustGit(t, repo, "commit", "-q", "-m", "seed")
	return repo
}

// recased is dir with its last element upper-cased, and the witness that the
// result is the SAME directory. It skips rather than fails when it is not:
// on a case-sensitive volume the two spellings are two directories, identity
// is right to keep them apart, and nothing here is measurable.
func recased(t *testing.T, dir string) string {
	t.Helper()
	other := filepath.Join(filepath.Dir(dir), strings.ToUpper(filepath.Base(dir)))
	if other == dir {
		t.Fatalf("fixture name %q has no case to change", filepath.Base(dir))
	}
	a, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Stat(other)
	if err != nil || !os.SameFile(a, b) {
		t.Skipf("%s is on a case-sensitive volume: %q and %q are two directories, so there is no case axis to measure here",
			dir, dir, other)
	}
	return other
}

// The bead's DONE WHEN: a case-changed `queue_repo:` over a store spelled as
// it is on disk commits, and the commit is findable by the bead's id — which
// is the whole point of committing the projection at all.
func TestQueueCommitLandsThroughACaseChangedQueueRepo(t *testing.T) {
	repo := qcaseRepo(t, "Repo")
	recasedRepo := recased(t, repo)
	store := filepath.Join(repo, ".beads") // spelled as on disk
	work := qWork(t, store)
	a := qApp(t, recasedRepo) // queue_repo:, spelled the other way
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	c, err := a.CommitQueueJSONL(NewBd(), work, "beads: q-1 closed by developer")
	if err != nil {
		t.Fatalf("CommitQueueJSONL: %v", err)
	}
	if c.Skipped != "" {
		t.Fatalf("a case-changed queue_repo: skipped the close's record: %q\n"+
			"queue_repo: %s, store %s — os.SameFile says those are one directory\n"+
			"the record never reaches the queue repo's git history, which is the bead-loss census's only input",
			c.Skipped, recasedRepo, store)
	}
	if c.SHA == "" {
		t.Fatalf("nothing was committed: %+v", c)
	}
	if msg := mustGit(t, repo, "log", "-1", "--format=%s"); !strings.Contains(msg, "q-1") {
		t.Errorf("the commit message must name the bead: %q", msg)
	}
	if body := mustGit(t, repo, "show", "HEAD:.beads/"+beadsJSONL); !strings.Contains(body, "closed") {
		t.Errorf("the committed projection is not the post-close one: %q", body)
	}
}

// The other direction, and what makes the identity walk safe to have: a
// store that really is outside the queue repo is still refused when the
// SPELLINGS differ only in case. A predicate that climbed to the volume root
// and answered true, or one that folded case into a prefix test, passes the
// pin above and fails this one.
func TestQueueCommitStillRefusesACaseChangedStranger(t *testing.T) {
	queue := qcaseRepo(t, "Queue")
	recasedQueue := recased(t, queue)
	elsewhere := qcaseRepo(t, "Elsewhere") // stands in for the constitution repo
	store := filepath.Join(elsewhere, ".beads")

	a := qApp(t, recasedQueue)
	before := mustGit(t, elsewhere, "rev-parse", "HEAD")
	closeBead(t, store, `{"id":"q-1","title":"closed"}`)

	c, err := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
	if err != nil {
		t.Fatalf("CommitQueueJSONL: %v", err)
	}
	if c.SHA != "" {
		t.Errorf("it committed in a repo queue_repo: does not name: %+v", c)
	}
	if !strings.Contains(c.Skipped, "not inside") {
		t.Errorf("the skip must say the store is not inside the queue repo, got %q", c.Skipped)
	}
	if after := mustGit(t, elsewhere, "rev-parse", "HEAD"); after != before {
		t.Errorf("HEAD moved in the wrong repo: %s -> %s", before, after)
	}
}

// underSameDir's own arms, including the two it must answer the same way
// underDir does.
func TestUnderSameDirAnswersContainmentByIdentity(t *testing.T) {
	base := gitTempDir(t)
	repo := filepath.Join(base, "Repo")
	store := filepath.Join(repo, ".beads")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	recasedRepo := recased(t, repo)

	if !underSameDir(recasedRepo, store) {
		t.Errorf("underSameDir(%q, %q) = false, want true — os.SameFile says those name one directory",
			recasedRepo, store)
	}
	// The directory itself, not only something inside it: underDir's own
	// contract is "p is dir or inside it", and the climb's first step has to
	// honour the first half.
	if !underSameDir(recasedRepo, repo) {
		t.Errorf("underSameDir(%q, %q) = false — a directory is under itself however it is spelled",
			recasedRepo, repo)
	}
	// And a stranger stays a stranger, case-changed or not: the climb must
	// not walk up past the queue repo and find a shared parent.
	other := filepath.Join(base, "Elsewhere", ".beads")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if underSameDir(recasedRepo, other) {
		t.Errorf("underSameDir(%q, %q) = true — those are two directories under one parent",
			recasedRepo, other)
	}
}

// The sibling-namespace shape identity must not reach, and the reason the
// climb asks about ancestors rather than about prefixes: `<base>.<suffix>`
// is NOT under `<base>` (ranger-base-cypy1, ConstitutionGrants' note), and
// it has no ancestor that is `<base>`, so the identity answer is the string
// answer here. A prefix-matching "fix" to containment would reverse this.
func TestUnderSameDirLeavesTheSiblingNamespaceOutside(t *testing.T) {
	base := gitTempDir(t)
	dir := filepath.Join(base, "config")
	sibling := filepath.Join(base, "config.yaml")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, sibling, "queue_repo: /nowhere\n")

	if underSameDir(dir, sibling) {
		t.Errorf("underSameDir(%q, %q) = true — a sibling is not inside", dir, sibling)
	}
	if underDir(dir, sibling) {
		t.Errorf("underDir(%q, %q) = true — the two predicates must agree here", dir, sibling)
	}
}

// underDir itself stays a STRING comparison. ~25 callers ask it about a
// sandbox grant, a seatbelt literal or a PID's `writable:`, where the
// spelling some other program will match IS the subject (seatbelt.go's note,
// and insideDir beside it). ADR 0082 D4 rejects canonicalising its sides
// explicitly. So this pin asserts the disagreement underSameDir exists to
// close, which is the only thing that keeps a later pass from closing it in
// the wrong place and widening every grant on the box.
func TestUnderDirStaysTheSandboxStringComparison(t *testing.T) {
	base := gitTempDir(t)
	repo := filepath.Join(base, "Repo")
	store := filepath.Join(repo, ".beads")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	recasedRepo := recased(t, repo)

	if underDir(recasedRepo, store) {
		t.Errorf("underDir(%q, %q) = true — it compares resolved spellings, and a grant predicate "+
			"that started folding case would widen every seatbelt grant on a case-insensitive volume "+
			"(ADR 0082 D4). Containment by identity is underSameDir.", recasedRepo, store)
	}
	// Not vacuous: the same predicate says true for the spelling as it is on
	// disk, so the false above is the case axis and not a broken fixture.
	if !underDir(repo, store) {
		t.Fatalf("underDir(%q, %q) = false — this fixture measures nothing", repo, store)
	}
}
