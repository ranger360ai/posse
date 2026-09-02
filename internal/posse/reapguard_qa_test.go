package posse

// QA pins for the ADR 0013 §4 reap guard (ranger-base-6jz): a session whose
// bead is still in_progress and whose cwd is dirty is not killed.
//
// The board every one of these is played on is the near-miss's own
// (ranger-base-0fb): a SHARED checkout, so there is no session branch and
// KillSessionAndLand's existing "a tree still holding work is kept" refusal
// never fires — the kill is a plain workspace close, and the 353 uncommitted
// lines go with it. The guard is what stands between those two facts.
//
// Both arms must fire, and each test below turns exactly one of them off.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reapRepo is the shared checkout a dispatched session works in: a real git
// repo whose fake-bd canned answers are COMMITTED, so the only dirt in it is
// dirt a test made.
func reapRepo(t *testing.T, b *HerdrBackend, show string) string {
	t.Helper()
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")
	if show != "" {
		commitIn(t, repo, "fake-show.json", show, "seed: the bead's status")
	}
	// The guard reads the store of record with the backend's own bd, which
	// the fleet's NewHerdrBackend fills in; a test backend is built bare.
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	return repo
}

// reapSession is a dispatched session in the shared checkout: it carries the
// bead pointer dispatch stamps at launch, and nothing else that matters here.
func reapSession(t *testing.T, b *HerdrBackend, name, repo, bead string) *HerdrMeta {
	t.Helper()
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: repo, Cmd: "true", Bead: bead}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta(name)
	if !ok {
		t.Fatalf("no meta for %s", name)
	}
	if m.Bead != bead {
		t.Fatalf("the session did not record its bead: %q", m.Bead)
	}
	return m
}

// dirty leaves work in the tree that no commit holds — the 353 lines, in
// miniature.
func dirty(t *testing.T, repo string) {
	t.Helper()
	write(t, filepath.Join(repo, "work.go"), "// 353 lines nobody committed\n")
}

func stillAlive(t *testing.T, b *HerdrBackend, name string) {
	t.Helper()
	if _, ok := b.readMeta(name); !ok {
		t.Errorf("%s was killed after all: its meta is gone", name)
	}
	if _, err := b.Resolve(name); err != nil {
		t.Errorf("%s was killed after all: %v", name, err)
	}
}

// The whole bead, in one board: open work, uncommitted tree, a reap.
func TestReapGuardRefusesADirtyTreeUnderAnOpenBead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := reapRepo(t, b, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	reapSession(t, b, "ranger-repo-a-1", repo, "a-1")
	dirty(t, repo)

	_, err := b.KillSessionAndLand("ranger-repo-a-1")
	if err == nil {
		t.Fatal("a session holding an open bead over an uncommitted tree was killed")
	}
	// It has to name WHY, and both arms of the why: a refusal the operator
	// cannot act on is a refusal they will --force out of habit.
	for _, want := range []string{"NOT killed", "a-1", "in_progress", "work.go", "--force"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
	stillAlive(t, b, "ranger-repo-a-1")
	if _, err := os.Stat(filepath.Join(repo, "work.go")); err != nil {
		t.Errorf("the refused kill destroyed the work anyway: %v", err)
	}
}

// Arm one off: the bead is open, and the persona committed. Committed work
// survives a kill, so there is nothing here to refuse over — a bookkeeping
// skip is gather's line to print and --resume's to retry, not a reason to
// keep a workspace alive forever.
func TestReapGuardLetsACleanTreeGoWithAnOpenBead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := reapRepo(t, b, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	reapSession(t, b, "ranger-repo-a-1", repo, "a-1")
	commitIn(t, repo, "work.go", "// committed\n", "a-1: the work")

	if _, err := b.KillSessionAndLand("ranger-repo-a-1"); err != nil {
		t.Fatalf("a clean tree must still be reapable: %v", err)
	}
	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("the session survived a kill it should have taken")
	}
}

// Arm two off: the tree is dirty and the bead is closed. That is the
// operator's own scratch in their own checkout, and none of the harness's
// business.
func TestReapGuardLetsADirtyTreeGoWhenTheBeadIsClosed(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := reapRepo(t, b, `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	reapSession(t, b, "ranger-repo-a-1", repo, "a-1")
	dirty(t, repo)

	if _, err := b.KillSessionAndLand("ranger-repo-a-1"); err != nil {
		t.Fatalf("a closed bead is not a reason to keep a session: %v", err)
	}
	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("the session survived a kill it should have taken")
	}
}

// No pointer, no question: an interactive session, a crew session, a recipe,
// and every session from before the pointer existed kill exactly as they did.
func TestReapGuardIsSilentOnASessionWithNoBead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := reapRepo(t, b, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	if err := b.CreateSession(NewSessionOpts{Name: "crew", Dir: repo, Cmd: "true", Crew: true}); err != nil {
		t.Fatal(err)
	}
	dirty(t, repo)

	if _, err := b.KillSessionAndLand("crew"); err != nil {
		t.Fatalf("a session with no bead of its own must kill unguarded: %v", err)
	}
}

// Ignorance inside the pair fails CLOSED. A bead pointer, a dirty tree, and
// a store that will not say whether the work is finished is the exact state
// a kill must not resolve by guessing — the cost of a wrong refusal is one
// --force, and of a wrong kill is work with no other copy.
func TestReapGuardRefusesWhenBdCannotSayWhetherTheBeadIsFinished(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := reapRepo(t, b, "") // no canned answer: bd resolves nothing
	reapSession(t, b, "ranger-repo-a-1", repo, "a-1")
	dirty(t, repo)

	_, err := b.KillSessionAndLand("ranger-repo-a-1")
	if err == nil {
		t.Fatal("an unreadable store of record let a dirty session be reaped")
	}
	if !strings.Contains(err.Error(), "could not say") || !strings.Contains(err.Error(), "a-1") {
		t.Errorf("the refusal must name what could not be read:\n%v", err)
	}
	stillAlive(t, b, "ranger-repo-a-1")
}

// --force is the operator saying they have read the refusal. It stands the
// guard down and nothing else: the landing's own refusals are untouched.
func TestForceKillTakesTheDirtyOpenSession(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := reapRepo(t, b, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	reapSession(t, b, "ranger-repo-a-1", repo, "a-1")
	dirty(t, repo)

	if _, err := b.ForceKillSessionAndLand("ranger-repo-a-1"); err != nil {
		t.Fatalf("--force must kill it anyway: %v", err)
	}
	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("--force did not kill the session")
	}
	// Forcing the reap is not permission to destroy the tree's contents:
	// the files are still there for whoever comes looking.
	if _, err := os.Stat(filepath.Join(repo, "work.go")); err != nil {
		t.Errorf("--force deleted uncommitted work: %v", err)
	}
}

// The refresh's kill is a kill. `--no-land` reaches it with the tree exactly
// as the agent left it, which is the reap the ADR names; the landing turn is
// the cure, and a session that lands properly refreshes as it always did.
func TestReapGuardRefusesARefreshThatWouldReapOpenWork(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := reapRepo(t, b, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	reapSession(t, b, "ranger-repo-a-1", repo, "a-1")
	dirty(t, repo)

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "ranger-repo-a-1", NoLand: true})
	if err == nil {
		t.Fatal("a refresh reaped a session still holding an open bead over uncommitted work")
	}
	for _, want := range []string{"NOT closed", "a-1", "work.go", "--force"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
	stillAlive(t, b, "ranger-repo-a-1")

	// And the operator's way through, once they have looked.
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "ranger-repo-a-1", NoLand: true, Force: true}); err != nil {
		t.Fatalf("--force must refresh it anyway: %v", err)
	}
}

// The pointer the guard reads is written by dispatch, not derived from the
// session name: an id is not recoverable from `<persona>-<repo>-<id>` when
// both halves may hold dashes, and a derivation that guesses is a store that
// can disagree with the bead (ADR 0011).
func TestDispatchRecordsTheBeadOnTheSessionItLaunches(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	if n, err := d.Run("", "", 0); err != nil || n != 1 {
		t.Fatalf("dispatched %d, err=%v:\n%s", n, err, dispatcherOut(d))
	}
	m, ok := b.readMeta(SessionForBead("ranger", repo, "a-1"))
	if !ok {
		t.Fatalf("no meta for the dispatched session:\n%s", dispatcherOut(d))
	}
	if m.Bead != "a-1" {
		t.Errorf("the dispatched session records bead %q, want a-1 — the reap guard has nothing to ask about", m.Bead)
	}
}
