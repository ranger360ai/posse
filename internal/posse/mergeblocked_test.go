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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// ─── the dedupe reads CLOSED beads too (ranger-base-j8qmj) ───────────────────
//
// The defect these pin: the dedupe read OPEN beads only, so CLOSING a block
// was the act that destroyed it. For a branch whose content is already on
// main — superseded, or re-landed by an earlier rescue — closing the block
// do-not-land is the CORRECT outcome, merge-back being ff-only. So the right
// answer destroyed the key, the next pass filed a byte-identical P1 against
// the same untouched branch, and a dispatched seat re-derived the same
// verdict. MEASURED 2026-09-04 over all 1921 beads: 23 merge-back filings
// across 15 branches, 8 of them re-files on 5 branches.

// closedVerdict plants the bead a persona was handed, read, and CLOSED
// do-not-land, in the labeled listing `--label-any code` answers from. NOT in
// fake-list.json, so mergeBlockedBeads keeps counting only what the pass
// itself files.
func closedVerdict(t *testing.T, repo, title string, when time.Time) {
	t.Helper()
	writeJSON(t, repo, "fake-list-labeled.json", []map[string]any{{
		"id": "v-1", "title": title, "status": "closed",
		"labels":    []string{MergeBlockedLabel},
		"closed_at": when.Format(time.RFC3339),
	}})
}

// commitAt is commitIn with the dates stated. These tests compare a branch's
// tip against a verdict's timestamp and `%cI` is second-granular, so a
// fixture that let two commits and a verdict race the same second would be
// measuring the clock. One second is git's granularity, not this suite's
// subject.
func commitAt(t *testing.T, dir, path, body, msg string, when time.Time) {
	t.Helper()
	write(t, filepath.Join(dir, path), body)
	stamp := when.Format(time.RFC3339)
	for _, args := range [][]string{
		{"config", "user.email", "p@example.com"},
		{"config", "user.name", "p"},
		{"add", path},
		{"commit", "-q", "-m", msg, "--", path},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// THE HEADLINE. A block already answered and closed draws nothing, however
// many passes read the branch — the property the open-only dedupe claimed in
// its own comment ("N sweeps over one permanently blocked branch leave one
// bead and one comment") and stopped delivering the moment anyone acted on
// the handoff.
func TestSweepDoesNotRefileABlockThatWasAlreadyClosed(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)
	// The verdict was recorded after the branch's last commit — which is the
	// only order that can happen in the field: the block cannot be filed
	// before the merge was attempted, the merge cannot precede the commit it
	// failed to land, and the close comes after the bead exists.
	closedVerdict(t, repo, mergeBlockedTitle(tr.Branch, "main"), time.Now().Add(time.Minute))

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "did NOT reach") {
		t.Fatalf("fixture: the sweep did not find a blocked merge, so nothing here measures the blocked arm:\n%s", out)
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 0 {
		t.Errorf("a branch whose block was closed do-not-land drew %d fresh handoff(s), want 0 — this is the P1 that costs a dispatched seat:\n%s", n, out)
	}
	if n := mergeBlockedNotes(t, repo); n != 0 {
		t.Errorf("the closed bead gained %d fresh pointers to a handoff nothing filed, want 0", n)
	}
	if !strings.Contains(out, "already answered this block") {
		t.Errorf("the pass did not say it recognised the closed verdict:\n%s", out)
	}
}

// THE WRONG ARM, and the reason the closed read is not simply "closed exists,
// stop". EnsureSessionTree is idempotent by design — "a relaunch, a resume,
// or a second pass over the same bead lands in the tree that already exists"
// — so a reopened bead re-dispatched into its old tree commits onto the same
// branch, and a dedupe that stopped at the closed bead would swallow THAT
// handoff forever. The verdict stands only while the branch it was about has
// not moved.
//
// Both arms are one test on purpose: the first pass is the control that
// proves this fixture can be silenced at all, so the second pass's bead is
// the branch moving and not the fixture failing to dedupe.
func TestSweepRefilesWhenTheBranchMovesAfterTheVerdict(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)
	verdict := time.Now().Add(time.Minute)
	closedVerdict(t, repo, mergeBlockedTitle(tr.Branch, "main"), verdict)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if n := len(mergeBlockedBeads(t, repo)); n != 0 {
		t.Fatalf("control: the first pass filed %d handoff(s) over an unmoved branch, want 0 — the second pass measures nothing:\n%s", n, dispatcherOut(d))
	}
	// The bead was reopened and re-dispatched into the tree it already had,
	// and the persona committed. That is work no verdict has ever read.
	commitAt(t, tr.Path, "more.txt", "the reopened bead's work\n", "a-1: more", verdict.Add(time.Minute))

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
		t.Errorf("a branch that gained a commit after the verdict drew %d handoff(s), want 1 — the closed bead silenced work nobody has read:\n%s", n, out)
	}
	if !strings.Contains(out, "has moved since") {
		t.Errorf("the pass did not say why it filed over a closed verdict:\n%s", out)
	}
}

// The selection, directly: which of a branch's several merge-back beads
// answers. Five branches carry two or three each in the field, so this is not
// a corner.
func TestPriorMergeBlockedPrefersOpenThenTheLatestVerdict(t *testing.T) {
	t.Parallel()
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	repo := t.TempDir()
	title := mergeBlockedTitle("posse/ranger-r-a-1", "main")
	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)
	row := func(id, status string, closed *time.Time, ttl string) map[string]any {
		r := map[string]any{"id": id, "title": ttl, "status": status, "labels": []string{MergeBlockedLabel}}
		if closed != nil {
			r["closed_at"] = closed.Format(time.RFC3339)
		}
		return r
	}

	// Two verdicts and another branch's bead: the freshest verdict for THIS
	// title answers, and the neighbour is not mistaken for it (the title is
	// matched exactly, never as a prefix — openTitledBead's E6).
	writeJSON(t, repo, "fake-list-labeled.json", []map[string]any{
		row("v-old", "closed", &old, title),
		row("v-new", "closed", &recent, title),
		row("other", "closed", &recent, mergeBlockedTitle("posse/ranger-r-a-2", "main")),
	})
	got, err := priorMergeBlocked(bd, repo, title)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "v-new" || got.Open || got.Verdict.Format(time.RFC3339) != recent.Format(time.RFC3339) {
		t.Errorf("the freshest verdict is %+v, want v-new closed at %s", got, recent.Format(time.RFC3339))
	}

	// An OPEN handoff outranks every verdict whatever the dates say: it is
	// still owed, and re-filing beside it is the duplicate the dedupe has
	// always refused.
	writeJSON(t, repo, "fake-list-labeled.json", []map[string]any{
		row("v-new", "closed", &recent, title),
		row("open-1", "in_progress", nil, title),
	})
	got, err = priorMergeBlocked(bd, repo, title)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "open-1" || !got.Open {
		t.Errorf("an open handoff beside a closed verdict read as %+v, want open-1 open", got)
	}
}
