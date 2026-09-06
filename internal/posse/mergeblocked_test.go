//go:build posse_arm2

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
	"fmt"
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

// ─── the block outlives its branch (ranger-base-m3195) ──────────────────────
//
// The defect these pin: the handoff's description asserted "The branch is
// untouched and still holds every commit", which is a claim about the FUTURE
// filed by a pass and read by a seat some unbounded time later. It was false
// at dispatch twice on record — ranger-base-g7br6 and ranger-base-nr3eq were
// both worked against a branch already deleted and a worktree path that no
// longer existed, and in both cases the only commit survived as an object
// reachable from no ref, alive only until the next `gc`. A seat that follows
// a false instruction literally either fails or invents a recovery out of a
// sha it scraped from the block reason.
//
// So the bead stops asserting what it cannot keep true, and posse pins the
// work under a ref of its own so the sha it DOES name is still there when
// the seat arrives.

// mergeBlockedDescription is the one filed handoff's body, read out of the
// store the fake keeps (mergeBlockedBeads' reason: what the graph HOLDS is
// the fact, not what an exit code said).
func mergeBlockedDescription(t *testing.T, repo string) string {
	t.Helper()
	filed := mergeBlockedBeads(t, repo)
	if len(filed) != 1 {
		t.Fatalf("fixture: %d merge-back handoffs filed, want 1 — there is no description to read", len(filed))
	}
	desc, _ := filed[0]["description"].(string)
	if desc == "" {
		t.Fatalf("the filed handoff carries no description at all: %v", filed[0])
	}
	return desc
}

// refsNaming is every ref in the repo that reaches sha — git's own answer to
// "would a gc keep this". Empty is the incident: an object alive only until
// the next prune, with a bead telling a seat to go and get it.
func refsNaming(t *testing.T, repo, sha string) string {
	t.Helper()
	out, err := git(repo, "for-each-ref", "--format=%(refname)", "--contains", sha)
	if err != nil {
		t.Fatalf("for-each-ref --contains %s: %v", sha, err)
	}
	return strings.TrimSpace(out)
}

// retireTreeAndBranch is what took ranger-base-nr3eq's work between the block
// being filed and the bead being worked: the worktree reaped and the branch
// deleted while the handoff stood open. It is written as the two git commands
// rather than through RemoveSessionTree on purpose — every one of that
// function's refusals hands an operator these exact two to run by hand, so
// this is the shape that must survive, whoever typed it.
func retireTreeAndBranch(t *testing.T, tr *SessionTree) {
	t.Helper()
	if _, err := git(tr.Repo, "worktree", "remove", "--force", tr.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := git(tr.Repo, "branch", "-D", tr.Branch); err != nil {
		t.Fatal(err)
	}
	if branchExists(tr.Repo, tr.Branch) {
		t.Fatalf("fixture: %s survived its own deletion", tr.Branch)
	}
}

// The headline. The sentence that was false at dispatch is gone, and what
// replaces it is the check plus the two facts that stay true either way.
func TestMergeBackHandoffNeverPromisesTheBranchWillStillBeThere(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)

	sha, err := git(tr.Repo, "rev-parse", tr.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	desc := mergeBlockedDescription(t, repo)

	// The CLAIM and not the word (ranger-base-eq3ba): whichever o.Reason
	// arm this fixture reached is embedded verbatim above the shelf-life
	// block, so the description carries the same promise its reason does.
	// Every arm is driven in TestMergeBlockedReasonsNeverPromiseTheBranch;
	// this is the one that proves the two are joined end to end.
	if claim := blockedPromise(desc); claim != "" {
		t.Errorf("the handoff still asserts the branch is still there (%q) — the claim that was false at dispatch on g7br6 and nr3eq:\n%s", claim, desc)
	}
	if !strings.Contains(desc, "CHECK THE BRANCH") || !strings.Contains(desc, "rev-parse --verify "+tr.Branch) {
		t.Errorf("the handoff does not tell the seat to read the branch before believing it:\n%s", desc)
	}
	// The handle that survives every way a branch can go, and the ref that
	// keeps it: a sha with no ref on it is the incident, not the fix.
	if !strings.Contains(desc, sha) {
		t.Errorf("the handoff never names %s, so a seat whose branch is gone has nothing to read:\n%s", sha, desc)
	}
	if !strings.Contains(desc, blockedPinRef(tr.Branch)) {
		t.Errorf("the handoff never names the pin %s:\n%s", blockedPinRef(tr.Branch), desc)
	}
}

// ─── the promise, over every arm that can carry it (ranger-base-eq3ba) ──────
//
// The pin above drives ONE of the merge path's refusals — the conflict arm —
// and greps its description for the WORD "untouched". That is the word
// ranger-base-m3195 happened to delete, not the claim it was filed to remove:
// at 3c2fa2a two arms that close never drove still said "%s still holds every
// commit" in a sentence with no "untouched" in it (worktree.go's constitution
// refusal and its replay-exhausted arm), and a third arm the close DID edit
// had nothing holding its wording at all. Planting the word in each of those
// three and re-running the merge suite left it green.
//
// So the claim is asserted instead of the word, and it is asserted over every
// o.Reason spelling in the merge path rather than the one a single fixture
// reaches: noteMergeBlocked embeds whichever one this pass produced
// (dispatch.go), verbatim, in a bead a seat opens some unbounded time later.

// blockedPromise names the claim no o.Reason may make, in any spelling: that
// the branch is still there and still holds the work. It is a claim about the
// FUTURE — true when the pass prints it, unknown by the time the handoff is
// read — and it was false at dispatch twice on record (g7br6, nr3eq).
//
// The report of what the launcher DID is always allowed: "the rebase was
// aborted", "nothing was landed", "nothing here was changed". Those stay true
// forever, which is the whole distinction.
func blockedPromise(reason string) string {
	low := strings.ToLower(reason)
	for _, claim := range []string{"untouched", "still holds", "still there", "still on the branch"} {
		if strings.Contains(low, claim) {
			return claim
		}
	}
	return ""
}

// mergeBlockedReason is the sentence a merge-back bead would embed for this
// fixture. The refusal is the premise: a fixture that landed, or that errored
// before the function decided anything, files no bead and measures no wording
// (MergeOutcome.Blocked's own doc says why both halves are load-bearing).
func mergeBlockedReason(t *testing.T, tr *SessionTree) string {
	t.Helper()
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatalf("fixture: MergeSessionWork errored, so no reason was produced: %v", err)
	}
	if !o.Blocked() {
		t.Fatalf("fixture: this arm must refuse and say why, got %+v", o)
	}
	return o.Reason
}

// wtTreeWithWork is the common half of every fixture below: a repo, a session
// tree, and one commit on the branch — because a branch with nothing ahead of
// its base never reaches a refusal at all.
func wtTreeWithWork(t *testing.T) (string, *SessionTree) {
	t.Helper()
	a := wtApp(t)
	repo := wtRepo(t)
	tr, err := a.EnsureSessionTree(repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "fix.txt", "the work\n", "s-1: the fix")
	return repo, tr
}

// detachTreeHead puts the session's own tree off its branch and commits
// there, which is where a persona's work goes when something detached the
// worktree — the shape landed() exists for (ranger-base-dybv).
func detachTreeHead(t *testing.T, tr *SessionTree) {
	t.Helper()
	if _, err := git(tr.Path, "checkout", "--detach"); err != nil {
		t.Fatal(err)
	}
	commitIn(t, tr.Path, "detached.txt", "off the branch\n", "s-1: off the branch")
}

// mergeBlockedCases is one case per o.Reason spelling the merge path can hand
// noteMergeBlocked. `arm` is the per-case POSITIVE CONTROL: a substring only
// that sentence carries, so a fixture that fell through to some other refusal
// reds instead of passing the claim check for an arm nobody drove. That is
// the failure the pin below had.
//
// Hoisted out of the test (ranger-base-xndgk FINDING 2) because a hand-written
// table with no floor is the ranger-base-ik44f shape one domain over: the
// file's own header promises the claim is asserted "over every o.Reason
// spelling in the merge path", and nothing kept "every" true — a thirteenth
// refusal added tomorrow got no case and nothing said so. The census that
// keeps it true is TestQAEveryMergeRefusalSentenceHasACase
// (mergereasonfloor_qa_test.go), and it reads this table.
func mergeBlockedCases() []mergeBlockedCase {
	return []mergeBlockedCase{
		{
			// worktree.go: the base is not a branch at all.
			name: "the tree has no base branch to land on",
			arm:  "there is no branch for",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				// The reachable shape: a branch cut before the base was
				// recorded, in a checkout that is now detached — so the
				// record is empty AND the fallback has nothing to give.
				mustGit(t, repo, "config", "--unset", baseKey(tr.Branch))
				mustGit(t, repo, "checkout", "--detach")
				// Read back through the sweep's own reader rather than
				// hand-building the struct: Base is derived (baseOf), and a
				// fixture that assigned it would be pinning my arithmetic.
				trees, err := SessionTreesIn([]string{repo})
				if err != nil || len(trees) != 1 {
					t.Fatalf("fixture: %d trees, %v", len(trees), err)
				}
				if trees[0].Base != "" {
					t.Fatalf("fixture: base = %q, want empty — this arm was not reached", trees[0].Base)
				}
				return mergeBlockedReason(t, trees[0])
			},
		},
		{
			// worktree.go, notOnBase: the operator's checkout is the one
			// store on this path the launcher lock does not govern.
			name: "the operator has another branch checked out",
			arm:  "checked out, not",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				mustGit(t, repo, "checkout", "-q", "-b", "sidetrack")
				return mergeBlockedReason(t, tr)
			},
		},
		{
			// The same producer's other spelling. notOnBase is also read
			// AFTER the rebase (worktree.go), so that arm is these two
			// sentences and pinning them here pins it.
			name: "the operator detached the checkout",
			arm:  "has a detached HEAD, so",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				mustGit(t, repo, "checkout", "--detach")
				return mergeBlockedReason(t, tr)
			},
		},
		{
			// constitutionOnBranch's own error return: nothing lands on a
			// diff git could not read. Produced by calling it rather than
			// through MergeSessionWork, because every way to break that one
			// diff also breaks the rev-list above it — which returns an
			// error and files no bead. The sentence is the same sentence.
			name: "git could not read the diff",
			arm:  "so whether",
			reason: func(t *testing.T) string {
				_, tr := wtTreeWithWork(t)
				broken := *tr
				broken.Base = "no-such-base"
				hit, why := constitutionOnBranch(&broken)
				if why == "" {
					t.Fatalf("fixture: the diff answered (%v), so this arm was not reached", hit)
				}
				return why
			},
		},
		{
			// The belt behind the commit wall (ranger-base-ak3e), and
			// FINDING 2 of ranger-base-eq3ba: this arm said "%s still holds
			// every commit" until this test.
			name: "the branch touches the constitution",
			arm:  "it touches the constitution",
			reason: func(t *testing.T) string {
				_, _, tr := constitutionLandTree(t, true)
				commitIn(t, tr.Path, constitutionClassSpec[0]+"/probe.md", "rewritten\n", "s-1: edit the law")
				return mergeBlockedReason(t, tr)
			},
		},
		{
			name: "the tree has uncommitted changes to rebase over",
			arm:  "has uncommitted changes",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				commitIn(t, repo, "other.txt", "meanwhile\n", "main moved")
				write(t, filepath.Join(tr.Path, "wip.txt"), "not committed\n")
				return mergeBlockedReason(t, tr)
			},
		},
		{
			name: "the replay conflicts and is aborted",
			arm:  "the rebase was aborted, so this attempt changed nothing",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				commitIn(t, tr.Path, "clash.txt", "the session's line\n", "s-1: mine")
				commitIn(t, repo, "clash.txt", "the operator's line\n", "main: theirs")
				return mergeBlockedReason(t, tr)
			},
		},
		{
			// ranger-base-5hqa's arm — and the one FINDING 4 found already
			// correct with nothing holding it: the close edited this
			// sentence and pinned a different one.
			name: "the replay never reached a merge",
			arm:  "failed before any merge",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				commitIn(t, repo, "elsewhere.txt", "meanwhile\n", "main: moved on")
				// A pre-rebase hook is the cheapest honest stand-in for the
				// class (a full disk, a lock, a bad object): git exits
				// non-zero before any merge and leaves no rebase state.
				hooks := t.TempDir()
				hook := filepath.Join(hooks, "pre-rebase")
				write(t, hook, "#!/bin/sh\necho 'no space left on device (simulated)' >&2\nexit 1\n")
				if err := os.Chmod(hook, 0o755); err != nil {
					t.Fatal(err)
				}
				mustGit(t, repo, "config", "core.hooksPath", hooks)
				return mergeBlockedReason(t, tr)
			},
		},
		{
			// The replay worked and the fast-forward still would not: the
			// base is where the rebase left it, so this is about the branch
			// and not a lost race (worktree.go).
			name: "the fast-forward refuses after a clean replay",
			arm:  "still would not fast-forward after the rebase",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				commitIn(t, repo, "other.txt", "meanwhile\n", "main moved")
				// An untracked file the merge would overwrite: git refuses
				// the ff without moving the base an inch.
				write(t, filepath.Join(repo, "fix.txt"), "the operator's own copy\n")
				return mergeBlockedReason(t, tr)
			},
		},
		{
			// FINDING 3 of ranger-base-eq3ba: this arm said "%s still holds
			// every commit and the next pass retries" until this test.
			name: "the base moved under every replay",
			arm:  "never held still",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				commitIn(t, repo, "other.txt", "meanwhile\n", "main moved")
				count := raceOnRebase(t, repo, mergeRebaseAttempts)
				reason := mergeBlockedReason(t, tr)
				if got := countIn(t, count); got != mergeRebaseAttempts {
					t.Fatalf("fixture: the rebase ran %d time(s), want %d — the race did not happen", got, mergeRebaseAttempts)
				}
				return reason
			},
		},
		{
			// landed()'s two arms: the work is in the TREE and not on the
			// branch, which is the read ranger-base-dybv put a measurement
			// behind. Both are reasons a merge-back bead embeds.
			name: "the tree's HEAD is off its own branch",
			arm:  "is on neither",
			reason: func(t *testing.T) string {
				_, tr := wtTreeWithWork(t)
				detachTreeHead(t, tr)
				return mergeBlockedReason(t, tr)
			},
		},
		{
			name: "no branch reaches the tree's work at all",
			arm:  "no branch here reaches it",
			reason: func(t *testing.T) string {
				repo, tr := wtTreeWithWork(t)
				detachTreeHead(t, tr)
				mustGit(t, repo, "branch", "-D", tr.Branch)
				return mergeBlockedReason(t, tr)
			},
		},
	}
}

func TestMergeBlockedReasonsNeverPromiseTheBranch(t *testing.T) {
	t.Parallel()
	cases := mergeBlockedCases()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			reason := c.reason(t)
			if !strings.Contains(reason, c.arm) {
				t.Fatalf("fixture: the reason is not this arm's — want %q in:\n%s", c.arm, reason)
			}
			if claim := blockedPromise(reason); claim != "" {
				t.Errorf("this refusal promises the seat that the branch is still there (%q) — a claim about the future, embedded verbatim in a bead read long after the pass that wrote it:\n%s", claim, reason)
			}
		})
	}
}

// And the pin is real, measured the only way that matters: retire the tree
// and delete the branch exactly as the field did, then ask git whether
// anything still reaches the commit.
func TestMergeBackPinKeepsTheWorkReachableAfterTheBranchIsDeleted(t *testing.T) {
	t.Parallel()
	d, _, tr := nurlBlocked(t)

	sha, err := git(tr.Repo, "rev-parse", tr.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if got, err := git(tr.Repo, "rev-parse", "--verify", blockedPinRef(tr.Branch)); err != nil || got != sha {
		t.Fatalf("the pass filed a block and pinned %q at %s, want %s (%v)", got, blockedPinRef(tr.Branch), sha, err)
	}
	retireTreeAndBranch(t, tr)

	if refs := refsNaming(t, tr.Repo, sha); refs != blockedPinRef(tr.Branch) {
		t.Fatalf("after the tree and branch went, %s is reached by %q, want only the pin", sha, refs)
	}
	// THE FAILING WRONG ARM. Drop the pin and the same repo, the same two
	// commands and the same commit leave nothing at all — which is the state
	// ranger-base-g7br6 and ranger-base-nr3eq were both worked in, and the
	// proof that the arm above measured the pin and not the fixture.
	if err := unpinBlockedWork(tr.Repo, tr.Branch); err != nil {
		t.Fatal(err)
	}
	if refs := refsNaming(t, tr.Repo, sha); refs != "" {
		t.Fatalf("without the pin %s is still reached by %q, so the arm above proved nothing", sha, refs)
	}
}

// closeTheBlock marks every merge-back handoff in the labeled listing closed
// — the seat read it, reached a verdict and closed it, which is the moment
// the pin stops being owed to anybody.
func closeTheBlock(t *testing.T, repo string) {
	t.Helper()
	var labeled []map[string]any
	b, err := os.ReadFile(filepath.Join(repo, "fake-list-labeled.json"))
	if err != nil {
		t.Fatalf("fixture: the pass filed nothing into the labeled listing: %v", err)
	}
	if err := json.Unmarshal(b, &labeled); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, row := range labeled {
		if title, _ := row["title"].(string); strings.HasPrefix(title, "merge-back blocked: ") {
			row["status"] = "closed"
			row["closed_at"] = time.Now().Format(time.RFC3339)
			n++
		}
	}
	if n == 0 {
		t.Fatal("fixture: no merge-back handoff in the labeled listing to close")
	}
	writeJSON(t, repo, "fake-list-labeled.json", labeled)
}

// The pin is not forever: a block that has been ANSWERED has no work owed to
// it, so the next pass drops the ref and git can collect again. It has to
// happen at pass start and off the REPO, because by now the tree this branch
// had is gone and nothing that walks session worktrees reaches it.
func TestPassDropsTheMergeBackPinOnceTheBlockIsAnswered(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlBlocked(t)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	pin := blockedPinRef(tr.Branch)
	if _, err := git(tr.Repo, "rev-parse", "--verify", pin); err != nil {
		t.Fatalf("fixture: the first pass left no pin, so the prune below measures nothing: %v", err)
	}
	retireTreeAndBranch(t, tr)
	closeTheBlock(t, repo)

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := git(tr.Repo, "rev-parse", "--verify", pin); err == nil {
		t.Errorf("%s still stands after the block was answered — posse has stopped collecting garbage for a branch nobody is owed", pin)
	}
}

// The wrong arm, and the one that makes the prune safe to have: while the
// block is still OPEN the pin is the only copy of somebody's work, and a
// pass that dropped it would be the incident with an extra step.
func TestPassKeepsTheMergeBackPinWhileTheBlockIsOpen(t *testing.T) {
	t.Parallel()
	d, _, tr := nurlBlocked(t)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	sha, err := git(tr.Repo, "rev-parse", tr.Branch)
	if err != nil {
		t.Fatal(err)
	}
	retireTreeAndBranch(t, tr)

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	pin := blockedPinRef(tr.Branch)
	if got, err := git(tr.Repo, "rev-parse", "--verify", pin); err != nil || got != sha {
		t.Fatalf("a pass dropped %s while its block was still open — %s is now reached by %q (%v)",
			pin, sha, refsNaming(t, tr.Repo, sha), err)
	}
}

// The store's own worst answer: bd will not say. Every pin stands, because
// "the graph is down" is not evidence that somebody's only copy can go.
func TestPinsStandWhenTheStoreWillNotSayWhichBlocksAreOpen(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "p@example.com"}, {"config", "user.name", "p"}} {
		if _, err := git(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	commitIn(t, repo, "fix.txt", "one\n", "root")
	tr := &SessionTree{Repo: repo, Path: repo, Branch: "posse/ranger-r-a-1", Base: "main"}
	sha, pin := pinBlockedWork(tr)
	if sha == "" || pin == "" {
		t.Fatalf("fixture: nothing was pinned (%q, %q), so the prune below measures nothing", sha, pin)
	}
	// The fake's hard-fail marker: `list` exits non-zero, which is what
	// AllLabeledAny hands the prune.
	write(t, filepath.Join(repo, "fake-list-fail"), "")

	var said strings.Builder
	prunePinnedBlocks(bd, repo, func(f string, a ...any) { fmt.Fprintf(&said, f, a...) })
	if _, err := git(repo, "rev-parse", "--verify", pin); err != nil {
		t.Errorf("a store that would not answer cost a live pin its ref: %v\n%s", err, said.String())
	}
	if !strings.Contains(said.String(), "not checked") {
		t.Errorf("the pass said nothing about the pins it could not check: %q", said.String())
	}
}

// The three arms in one place, including the one no fixture above reaches:
// a filing that could not read a head at all. Each says something DIFFERENT
// about how recoverable the work is, and a seat that is told the wrong one
// either hunts for a ref that was never written or walks past a commit it
// still had time to save.
func TestMergeBlockedShelfLifeSaysWhichRecoveryTheSeatHas(t *testing.T) {
	t.Parallel()
	tr := &SessionTree{Repo: "/r", Path: "/w/t", Branch: "posse/ranger-r-a-1", Base: "main"}
	for _, c := range []struct {
		what, sha, pin string
		want, notWant  string
	}{
		{"pinned", "abc123", blockedPinRef(tr.Branch), "pinned it at " + blockedPinRef(tr.Branch), "NOTHING PINNED IT"},
		{"a sha and no pin", "abc123", "", "NOTHING PINNED IT", "pinned it at"},
		{"no head at all", "", "", "no sha to fall back", "NOTHING PINNED IT"},
	} {
		got := mergeBlockedShelfLife(tr, "main", c.sha, c.pin)
		if !strings.Contains(got, "rev-parse --verify "+tr.Branch) {
			t.Errorf("%s: the seat is never told to read the branch:\n%s", c.what, got)
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: want %q in:\n%s", c.what, c.want, got)
		}
		if strings.Contains(got, c.notWant) {
			t.Errorf("%s: says %q, which is another arm's recovery:\n%s", c.what, c.notWant, got)
		}
	}
}

// The migration case, and the one the filing alone does not cover: a block
// filed BEFORE posse pinned anything has the same branch and the same shelf
// life, and it is the oldest open blocks that are nearest to being reaped.
// So the pass pins on the arm that recognises an open handoff too, not only
// on the arm that files one.
func TestPassPinsABlockItAlreadyFiledAndDidNotRefile(t *testing.T) {
	t.Parallel()
	d, _, tr := nurlBlocked(t)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	// The pre-pin world: an open handoff whose branch nothing names but
	// refs/heads.
	if err := unpinBlockedWork(tr.Repo, tr.Branch); err != nil {
		t.Fatal(err)
	}
	sha, err := git(tr.Repo, "rev-parse", tr.Branch)
	if err != nil {
		t.Fatal(err)
	}

	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	// The positive witness: an assertion about the not-re-filed arm is worth
	// nothing if the pass took the filing arm instead and pinned there.
	if out := dispatcherOut(d2); !strings.Contains(out, "already filed") {
		t.Fatalf("fixture: the second pass re-filed rather than recognising the open handoff, so this measures the wrong arm:\n%s", out)
	}
	if got, err := git(tr.Repo, "rev-parse", "--verify", blockedPinRef(tr.Branch)); err != nil || got != sha {
		t.Errorf("a pass that read an open block left its work at %q, want %s pinned (%v)", got, sha, err)
	}
}
