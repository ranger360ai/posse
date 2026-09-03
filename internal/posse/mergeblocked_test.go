package posse

// ranger-base-5nf8m: the merge-back handoff is filed by every site that
// reads a closed bead's blocked merge, not only by the site that judged the
// close.
//
// THE ASYMMETRY THIS PINS. ranger-base-dybv fixed the inference in landed()
// and closed over the reporting with a stated assumption: "All four callers
// (mergeBack, landClosedTrees, LandSessionTrees, KillSessionAndLand) already
// had a loud !Merged arm, so the pass prints the warning and mergeBack files
// the merge-blocked bead with no change to any of them." That was true of
// mergeBack and of nothing else. The sweep is the site that sees the closes
// mergeBack never judges — landsweep.go's header says why — which makes it
// the site most likely to be a strand's ONLY reader, and it printed a ⚠ line
// and filed nothing. ranger-base-aupee is the instance: closed at 861b0e6,
// 134 files that never reached main, no merge-back bead anywhere in the
// store, and a human finding it by hand hours later.
//
// AND WHY THESE ARE ABOUT THE DEDUPE AS MUCH AS THE FILING. The rationale
// that kept the sweep quiet was "a bead per pass over a permanently
// conflicted branch is spam, not a handoff", so "it does not spam" is the
// whole objection being overturned and cannot be left to a comment:
// TestSweepFilesOneMergeBackBeadOverEveryPass is the property, measured over
// two passes of the same blocked tree.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mergeBlockedBeads is every bead filed with the merge-back handoff's title,
// read out of the store the fake keeps rather than off a create's exit code
// (closedDirtyBeads' reason: what the graph HOLDS is the fact).
func mergeBlockedBeads(t *testing.T, repo string) []map[string]any {
	t.Helper()
	var list []map[string]any
	b, err := os.ReadFile(filepath.Join(repo, "fake-list.json"))
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatalf("fake-list.json is not JSON: %v\n%s", err, b)
	}
	var out []map[string]any
	for _, is := range list {
		if title, _ := is["title"].(string); strings.HasPrefix(title, "merge-back blocked: ") {
			out = append(out, is)
		}
	}
	return out
}

// mergeBlockedNotes is how many `merge-back blocked: filed` pointers are on
// the closed bead — the breadcrumb back from the handoff, and the other
// thing N passes must not write N of.
func mergeBlockedNotes(t *testing.T, repo string) int {
	t.Helper()
	n := 0
	for _, c := range readComments(t, repo) {
		if txt, _ := c["text"].(string); strings.HasPrefix(txt, "merge-back blocked: filed ") {
			n++
		}
	}
	return n
}

// nurlBlocked is the incident with the merge made impossible: the sweep's
// board (a close NOBODY watched — landsweep_test.go's nurlStranded, so
// mergeBack never ran on it at all) plus a base that moved under the branch
// with a conflicting edit of the same path. !Merged with a reason, which is
// the arm that used to print and file nothing.
func nurlBlocked(t *testing.T) (*Dispatcher, string, *SessionTree) {
	t.Helper()
	d, repo, tr := nurlStranded(t, "closed", true)
	// The closer, in the store of record's own words: bd records no close
	// actor, so the assignee that held the bead is the honest answer to who
	// the handoff goes back to (verifyCloser).
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
	dispatcherErr(t, d)
	return d, repo, tr
}

// The headline. Without it a closed bead's commits sit on a branch with the
// only record of it in the scrollback of a pass nobody was watching.
func TestSweepFilesTheMergeBackHandoffForACloseNobodyWatched(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	// The fixture's positive witness: an assertion about a bead the blocked
	// arm files is worth nothing if the pass took some other arm.
	if !strings.Contains(out, "did NOT reach") {
		t.Fatalf("fixture: the sweep did not find a blocked merge, so nothing here measures the blocked arm:\n%s", out)
	}
	filed := mergeBlockedBeads(t, repo)
	if len(filed) != 1 {
		t.Fatalf("the sweep filed %d merge-back handoffs over a close nobody watched, want 1:\n%s", len(filed), out)
	}
	// It is about THIS branch — the title is also the dedupe key, and a
	// handoff naming the wrong branch deduplicates the wrong merges away.
	if title, _ := filed[0]["title"].(string); title != mergeBlockedTitle(tr.Branch, "main") {
		t.Errorf("the handoff's title is %q, want %q", title, mergeBlockedTitle(tr.Branch, "main"))
	}
	// And it goes back to the closer, with the provenance edge and the P1
	// the judged close's handoff carries.
	bd := bdCalls(t, fakeDirOf(t))
	for _, want := range []string{"-a ranger", "-l code", "--deps discovered-from:a-1", "-p 1"} {
		if !strings.Contains(bd, want) {
			t.Errorf("the sweep's handoff was filed without %q:\n%s", want, bd)
		}
	}
	if n := mergeBlockedNotes(t, repo); n != 1 {
		t.Errorf("the closed bead carries %d pointers back to its handoff, want 1", n)
	}
}

// THE OBJECTION THE OLD RATIONALE RESTED ON, measured. "This runs every pass,
// and a bead per pass over a permanently conflicted branch is spam" — so the
// property that replaces it has to be pinned directly: N sweeps over one
// blocked tree, one bead and one comment. The dedupe is the handoff's OWN
// title (branch+base, and a branch is cut per bead), which is the same key
// the judged close has always read back.
func TestSweepFilesOneMergeBackBeadOverEveryPass(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 1 {
		t.Fatalf("fixture: the first pass filed %d handoffs, want 1 — the second pass measures nothing", n)
	}
	// A second pass over the same tree: nothing about it has changed, and the
	// branch is still blocked, so the sweep reads it again exactly as the
	// field does every three minutes.
	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)
	if !strings.Contains(out, "did NOT reach") {
		t.Fatalf("fixture: the second pass did not read the blocked tree at all:\n%s", out)
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 1 {
		t.Errorf("two passes over one blocked branch left %d handoffs, want 1 — this is the spam the old rationale refused to file for:\n%s", n, out)
	}
	if n := mergeBlockedNotes(t, repo); n != 1 {
		t.Errorf("two passes left %d pointers on the closed bead, want 1", n)
	}
	if !strings.Contains(out, "already filed") {
		t.Errorf("the second pass did not say it recognised the open handoff:\n%s", out)
	}
	// The tree and its branch are untouched by any of it: this sweep reports
	// and lands, it never repairs.
	if !branchExists(tr.Repo, tr.Branch) {
		t.Errorf("the sweep deleted %s", tr.Branch)
	}
}

// The wrong arm, and the one that makes the pins above worth having: an OPEN
// bead's branch that will not fast-forward is a persona at work, not a
// strand, and a P1 filed at them for it is a false handoff.
func TestSweepFilesNoMergeBackHandoffForABeadThatIsStillOpen(t *testing.T) {
	t.Parallel()
	d, repo, _ := nurlStranded(t, "in_progress", true)
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`)
	commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
	dispatcherErr(t, d)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 0 {
		t.Errorf("an open bead's tree drew %d merge-back handoff(s):\n%s", n, dispatcherOut(d))
	}
}

// ─── the third site: the kill's landing ──────────────────────────────────────

// mergeBlockedKillSession is a dispatched session whose branch holds a commit
// the base has moved out from under. `posse kill` KEEPS that tree, so the
// next pass's sweep would reach the same branch — what this site covers is
// the window where there is no next pass, and it costs nothing because both
// sites dedupe on one title.
func mergeBlockedKillSession(t *testing.T, status string) (*HerdrBackend, string, string) {
	t.Helper()
	b, _ := newTestBackend(t)
	// fakeBinFor and not os.Executable(): the child reads its fake-state
	// directory off argv[0] (fakeDir), so a bd invoked as the test binary
	// itself logs its calls into the shared build directory.
	b.Bd = Bd{Bin: fakeBinFor(t, "bd")}
	repo := wtqaRepo(t, b.App, `[]`, `[{"id":"a-1","status":"`+status+`","assignee":"ranger"}]`)
	name := SessionForBead("ranger", repo, "a-1")
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: repo, Cmd: "true", Bead: "a-1", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta(name)
	if !ok || m.Branch == "" {
		t.Fatalf("the session has no worktree of its own: %+v", m)
	}
	commitIn(t, m.Dir, "fix.txt", "the persona's work\n", "a-1: the fix")
	commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
	return b, repo, name
}

func TestKillLandingFilesTheMergeBackHandoffOnAClosedBead(t *testing.T) {
	t.Parallel()
	b, repo, name := mergeBlockedKillSession(t, "closed")

	l, err := b.ForceKillSessionAndLand(name)
	if err != nil {
		t.Fatal(err)
	}
	if l == nil || l.Kept == "" {
		t.Fatalf("fixture: the kill landed the branch, so there is no blocked merge to file for: %+v", l)
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 1 {
		t.Errorf("the kill's landing filed %d merge-back handoffs, want 1 (kept: %s)", n, l.Kept)
	}
	if n := mergeBlockedNotes(t, repo); n != 1 {
		t.Errorf("the closed bead carries %d pointers back to its handoff, want 1", n)
	}
}

// The kill's wrong arm, for the reason noteUnlandedOnKill states: a kill
// lands OPEN beads too (the reap guard refuses that pair only as far as
// --force), and work in progress that will not fast-forward is not a close
// that did not land.
func TestKillLandingFilesNothingWhenTheBeadIsStillOpen(t *testing.T) {
	t.Parallel()
	b, repo, name := mergeBlockedKillSession(t, "in_progress")

	if _, err := b.ForceKillSessionAndLand(name); err != nil {
		t.Fatal(err)
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 0 {
		t.Errorf("an open bead's blocked branch drew %d merge-back handoff(s)", n)
	}
}

// ─── the guard: !Merged is not the same fact as "the merge said no" ──────────

// MergeOutcome.Blocked, from the kill's shape: noteUnlandedOnKill runs BEFORE
// the error return, deliberately (the dirt is still there to read and after
// the return the reading is gone), so it is handed the ZERO outcome of a
// MergeSessionWork that errored — !Merged, no branch read, no obstacle named.
// A guard of !o.Merged alone would file a P1 whose whole reason line is
// empty, at a persona who cannot act on it, over a merge that never happened.
func TestNoteMergeBlockedFilesNothingForAMergeThatNeverAnswered(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	before := bdCalls(t, fake)
	tr := &SessionTree{Repo: t.TempDir(), Path: "/w/t", Branch: "posse/ranger-r-a-1", Base: "main"}
	quiet := func(string, ...any) {}

	noteMergeBlocked(bd, tr.Repo, "a-1", "ranger", tr,
		MergeOutcome{Branch: tr.Branch, Base: "main"}, quiet, quiet)
	if after := bdCalls(t, fake); after != before {
		t.Errorf("a merge that never answered filed a handoff:\n%s", strings.TrimPrefix(after, before))
	}
	// The failing wrong arm: the same call with a reason DOES file, so what
	// silenced the one above is the guard and not a fixture that could never
	// have filed anything.
	noteMergeBlocked(bd, tr.Repo, "a-1", "ranger", tr,
		MergeOutcome{Branch: tr.Branch, Base: "main", Commits: 2, Reason: "main moved on and replaying conflicts"}, quiet, quiet)
	if after := bdCalls(t, fake); !strings.Contains(strings.TrimPrefix(after, before), "create merge-back blocked") {
		t.Fatalf("the fixture cannot file at all, so the arm above measures nothing:\n%s", after)
	}
}
