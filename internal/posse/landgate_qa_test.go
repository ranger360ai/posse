//go:build posse_arm3

package posse

// ranger-base-pqque: `posse kill` landed a session's branch onto the repo's
// branch while the bead was in_progress and dep-blocked on an operator
// question — the landing being precisely the act the seat had refused to
// perform and filed the question about.
//
// Every pin here is a PAIR, because a gate that refuses everything and a
// gate that refuses nothing both satisfy a one-sided assertion: each keep is
// measured against the same fixture with the bead CLOSED, where the identical
// branch lands and the tree goes. The base is deliberately left where the
// session's branch was cut from, so the landing being refused is one that
// would otherwise SUCCEED — a fixture whose merge could not have happened
// anyway would pin nothing at all.

import (
	"os"
	"strings"
	"testing"
)

// landGateSession is the live shape: a dispatched session with its own
// worktree, one commit of its own on its branch, and a bead in the store the
// fixture describes. The tree is CLEAN, which is what makes this the kill the
// bead reports rather than one the reap guard refuses first (reapguard.go):
// the seat committed its work and stopped.
func landGateSession(t *testing.T, show string) (*HerdrBackend, string, string, *SessionTree, string) {
	t.Helper()
	b, _ := newTestBackend(t)
	// fakeBinFor and not os.Executable(): the child reads its fake-state
	// directory off argv[0], so a bd invoked as the test binary itself logs
	// into the shared build directory.
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	repo := wtqaRepo(t, b.App, `[]`, show)
	name := SessionForBead("ranger", repo, "a-1")
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: repo, Cmd: "true", Bead: "a-1", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta(name)
	if !ok || m.Branch == "" {
		t.Fatalf("the session has no worktree of its own: %+v", m)
	}
	commitIn(t, m.Dir, "fix.txt", "the persona's work\n", "a-1: the fix")
	tr := SessionTreeOf(m)
	if tr == nil {
		t.Fatalf("no session tree off the meta: %+v", m)
	}
	// Both halves of the premise, or the pins below are green over a fixture
	// that could not have landed and would not have been refused.
	if dirty := dirtyPaths(tr.Path); len(dirty) > 0 {
		t.Fatalf("fixture: the tree is dirty (%v), so the REAP guard answers first and this gate is never asked", dirty)
	}
	if n, ok := unlandedCount(tr); !ok || n != 1 {
		t.Fatalf("fixture: %s holds %d commit(s) the base does not (readable=%v), want 1", tr.Branch, n, ok)
	}
	return b, repo, name, tr, mustGit(t, repo, "rev-parse", "refs/heads/"+tr.Branch)
}

// landGateState reports what became of the tree: whether the base took the
// session's commit, and whether the worktree and branch are still there to
// hold it.
//
// Asked of the recorded TIP and not of `<base>..<branch>`, because a landing
// deletes the branch — so the count the fixture checks is unanswerable on
// exactly the arm where the landing happened, and reads the same as "kept"
// (ranger-base-pqque, first draft of this file).
func landGateState(t *testing.T, repo, tip string, tr *SessionTree) (onBase, treeThere, branchThere bool) {
	t.Helper()
	_, err := git(repo, "merge-base", "--is-ancestor", tip, tr.Base)
	_, statErr := os.Stat(tr.Path)
	return err == nil, statErr == nil, branchExists(repo, tr.Branch)
}

// blockedOn plants the question bead in the listing `bd dep list <id>`
// answers from: the ASK rung's own shape (settleopen.go files exactly this
// edge — `bd dep add <stuck> <question>` — to take a bead out of `bd ready`
// until the operator answers).
func blockedOn(t *testing.T, repo, id, status string) {
	t.Helper()
	writeJSON(t, repo, "fake-deps.json", []map[string]any{{
		"id": id, "title": "may this land?", "status": status,
		"issue_type": "bug", "labels": []string{"question"},
		"dependency_type": "blocks",
	}})
}

// THE HEADLINE, and the reported incident exactly: in_progress, blocked on an
// unanswered question, one commit on the branch, and a kill that frees the
// seat. The seat goes; the landing does not happen; the sentence names the
// bead, its status and the question that is holding it.
func TestQAKillDoesNotLandABranchWhoseBeadIsBlockedOnAQuestion(t *testing.T) {
	t.Parallel()
	b, repo, name, tr, tip := landGateSession(t, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	blockedOn(t, repo, "q-1", "open")

	l, err := b.KillSessionAndLand(name)
	if err != nil {
		t.Fatalf("the kill itself was refused (%v) — the seat is the thing a kill must always free", err)
	}
	onBase, treeThere, branchThere := landGateState(t, repo, tip, tr)
	if onBase {
		t.Errorf("the kill landed %s onto %s while a-1 was in_progress and blocked on q-1 — the landing IS the gated act", tr.Branch, tr.Base)
	}
	if !treeThere || !branchThere {
		t.Errorf("the work was not landed and the tree (%v) / branch (%v) did not survive to hold it", treeThere, branchThere)
	}
	// The kill still did its job: a seat that cannot be freed is a worse
	// bug than the one this gate fixes.
	if _, ok := b.readMeta(name); ok {
		t.Errorf("%s still has a session meta — the kill did not free the seat", name)
	}
	for _, want := range []string{"a-1 is in_progress", "blocked on q-1", "not landed"} {
		if !strings.Contains(l.Kept, want) {
			t.Errorf("the keep does not say %q:\n%s", want, l.Kept)
		}
	}
	// Said out loud, not only recorded on the struct: `posse kill` prints
	// these lines and nothing else reports this.
	if ln := l.Line(); !strings.Contains(ln, "KEPT: a-1 is in_progress") {
		t.Errorf("the kill's printed line does not carry the keep:\n%s", ln)
	}
}

// THE CONTROL for every keep in this file: the same fixture over a CLOSED
// bead lands and retires. Without it each pin above is payable by a rig that
// could never have landed anything.
func TestQAKillLandsTheSameBranchOnceTheBeadIsClosed(t *testing.T) {
	t.Parallel()
	b, repo, name, tr, tip := landGateSession(t, `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	blockedOn(t, repo, "q-1", "closed")

	l, err := b.KillSessionAndLand(name)
	if err != nil {
		t.Fatal(err)
	}
	onBase, treeThere, branchThere := landGateState(t, repo, tip, tr)
	if !onBase {
		t.Fatalf("a closed bead's branch did not reach %s — this fixture cannot land at all, so the keeps it controls measure nothing (kept: %s)", tr.Base, l.Kept)
	}
	if treeThere || branchThere {
		t.Errorf("the landing did not retire the tree (%v) / branch (%v)", treeThere, branchThere)
	}
	if l.Kept != "" {
		t.Errorf("a landed kill still reported a keep: %s", l.Kept)
	}
}

// --force is the operator saying they have read the REAP refusal — that they
// have looked at that session's unfinished work (ADR 0013 §4). It stands that
// guard down "and nothing else ... not the tree's contents, which are left
// where they are", and where the branch goes is a decision about the repo, not
// about the session.
func TestQAForceKillStillDoesNotLandAnUnclosedBeadsBranch(t *testing.T) {
	t.Parallel()
	b, repo, name, tr, tip := landGateSession(t, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)

	l, err := b.ForceKillSessionAndLand(name)
	if err != nil {
		t.Fatal(err)
	}
	if onBase, _, _ := landGateState(t, repo, tip, tr); onBase {
		t.Errorf("--force landed an in_progress bead's branch onto %s", tr.Base)
	}
	if !strings.Contains(l.Kept, "a-1 is in_progress") {
		t.Errorf("the keep does not name the bead and its status:\n%s", l.Kept)
	}
	// No question bead in this fixture, so the sentence must not invent one.
	if strings.Contains(l.Kept, "blocked on") {
		t.Errorf("an unblocked bead's keep claims a blocker:\n%s", l.Kept)
	}
}

// The blocker half's own wrong arm: an ANSWERED question is not a decision
// the operator still owes, and a keep that names it sends them to a closed
// bead to read a verdict that is already in. The bead's status is what holds
// this tree, and the sentence says only that.
func TestQAKillsKeepDoesNotNameAnAlreadyAnsweredQuestion(t *testing.T) {
	t.Parallel()
	b, repo, name, tr, tip := landGateSession(t, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	blockedOn(t, repo, "q-1", "closed")

	l, err := b.KillSessionAndLand(name)
	if err != nil {
		t.Fatal(err)
	}
	if onBase, _, _ := landGateState(t, repo, tip, tr); onBase {
		t.Fatalf("fixture: the branch landed, so there is no keep here to read (kept: %s)", l.Kept)
	}
	if !strings.Contains(l.Kept, "a-1 is in_progress") {
		t.Errorf("the keep does not name the bead and its status:\n%s", l.Kept)
	}
	if strings.Contains(l.Kept, "q-1") {
		t.Errorf("the keep sends the operator to a question that is already answered:\n%s", l.Kept)
	}
}

// A store that cannot answer is not a store that said yes — the same
// fail-closed reading the reap guard and RemoveSessionTree already make of
// the same pair.
func TestQAKillDoesNotLandWhenBdCannotSayWhetherTheBeadIsClosed(t *testing.T) {
	t.Parallel()
	b, repo, name, tr, tip := landGateSession(t, `[]`)

	l, err := b.ForceKillSessionAndLand(name)
	if err != nil {
		t.Fatal(err)
	}
	if onBase, _, _ := landGateState(t, repo, tip, tr); onBase {
		t.Errorf("an unanswerable store licensed a landing onto %s", tr.Base)
	}
	if !strings.Contains(l.Kept, "bd could not say whether a-1 is finished") {
		t.Errorf("the keep does not say the store could not answer:\n%s", l.Kept)
	}
}

// THE ARM THAT KEEPS THE CLEANUP WORKING. A tree with nothing ahead of its
// base is not a landing, so there is no act here to gate — and refusing over
// one would strand every finished session's empty tree forever, which is the
// population the auto-reaper's widened arms reap (autoreap.go: residueHolds
// lets them through only when the tree holds nothing at all).
func TestQAKillRetiresAnEmptyTreeWhateverTheBeadSays(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	repo := wtqaRepo(t, b.App, `[]`, `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	name := SessionForBead("ranger", repo, "a-1")
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: repo, Cmd: "true", Bead: "a-1", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, _ := b.readMeta(name)
	tr := SessionTreeOf(m)
	if n, ok := unlandedCount(tr); !ok || n != 0 {
		t.Fatalf("fixture: %s already holds %d commit(s) (readable=%v) — this arm is about a tree with nothing to land", tr.Branch, n, ok)
	}

	l, err := b.ForceKillSessionAndLand(name)
	if err != nil {
		t.Fatal(err)
	}
	_, treeThere, branchThere := landGateState(t, repo, tr.Base, tr)
	if treeThere || branchThere {
		t.Errorf("an empty tree was kept over an open bead (tree=%v branch=%v, kept: %s) — nothing was at risk and nothing else will ever remove it",
			treeThere, branchThere, l.Kept)
	}
}
