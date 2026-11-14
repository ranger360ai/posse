//go:build posse_arm2

package posse

// A close whose commit the push gate holds carries the back-pointer: the sha,
// the ref it is parked at, and the bead the hold was filed as. Filed as
// ranger-base-yc3i3
//
// Laid out the way closeddirty_test.go lays its own out, and for the same
// reason: the marker is the dedupe key, so it is pinned as TEXT first, and
// then the census and the write are driven for real over a git repo and the
// fake store — because the whole finding is that the facts were all on
// record (a parked ref, a closed bead, an open hold naming the sha) and
// nothing joined them.
//
// NO REAL BEAD ID IS FIXTURE DATA HERE. The ids in the leading-id table are
// made up and resolve to nothing in any store — `ranger-base-zzzz1` and the
// short `a-N` the rest of this package uses. A real, open `vuln-private` id
// as fixture data is its own defect, and it is the one the push gate held a
// commit over on 2026-11-14; the hold is why this file says so out loud.
//
// MUTATION-CHECKED, control green before and after, source md5 restored:
//
//	mutant                                          red
//	census not scoped to live branches              the census pin: work a live branch carries is named
//	nothing counts as landed                        the census pin: the re-landed bead is named
//	base read whole-message, not subject            the census pin: the body-named bead goes silent
//	key not sorted                                  the marker pin
//	dedupe ignores the sha set                      the marker pin: a second copy is swallowed
//	a closed bead counts as naming the sha          the hold pin
//	an open bead is commented on too                the open-bead pin
//	the id shape guard dropped                      the leading-id pin

import (
	"path/filepath"
	"strings"
	"testing"
)

// parkedWorkMarkers counts the comments on a bead that open with this
// mechanism's marker, from every pass between them.
func parkedWorkMarkers(t *testing.T, repo, id string) int {
	t.Helper()
	n := 0
	for _, c := range readComments(t, repo) {
		if txt, _ := c["text"].(string); strings.HasPrefix(txt, parkedWorkPrefix) {
			if got, _ := c["issue_id"].(string); got == id || id == "" {
				n++
			}
		}
	}
	return n
}

// parkedWorkTexts is the text of those comments, for the pins that read what
// the line actually says.
func parkedWorkTexts(t *testing.T, repo string) []string {
	t.Helper()
	var out []string
	for _, c := range readComments(t, repo) {
		if txt, _ := c["text"].(string); strings.HasPrefix(txt, parkedWorkPrefix) {
			out = append(out, txt)
		}
	}
	return out
}

// parkCommit makes one commit on a branch of its own, parks its tip under
// refs/posse/retired the way the push gate's hold does, and leaves the base
// where it was. keepBranch leaves the branch standing, which is the one case
// the census must stay silent about: work a live branch still carries is the
// landing sweep's, not this one's.
func parkCommit(t *testing.T, repo, name, subject string, keepBranch bool) string {
	t.Helper()
	mustGit(t, repo, "checkout", "-q", "-b", name, "main")
	commitIn(t, repo, name+".txt", subject+"\n", subject)
	sha := mustGit(t, repo, "rev-parse", "HEAD")
	mustGit(t, repo, "checkout", "-q", "main")
	mustGit(t, repo, "update-ref", retiredTipPrefix+name+"-"+sha[:8], sha)
	if !keepBranch {
		mustGit(t, repo, "branch", "-qD", name)
	}
	return sha
}

// ─── the marker ──────────────────────────────────────────────────────────────

// The key is read back by every later pass, so it must refuse text the
// harness did not write — a persona's own comment must never suppress the
// correction — and it must be keyed on the sha SET, so a second parked copy
// of the same bead's work is a line and not a silence.
func TestParkedWorkMarkerIsAtTheHeadAndKeyedOnTheShaSet(t *testing.T) {
	t.Parallel()

	key := parkedWorkKey([]string{"deadbeef"})
	if key != parkedWorkPrefix+"deadbeef]" {
		t.Errorf("the one-sha key must be %q: %q", parkedWorkPrefix+"deadbeef]", key)
	}
	// Sorted, because git's walk order is not a promise: two spellings of
	// one set must be one key or the same line is filed twice.
	if a, b := parkedWorkKey([]string{"bbbbbbbb", "aaaaaaaa"}), parkedWorkKey([]string{"aaaaaaaa", "bbbbbbbb"}); a != b {
		t.Errorf("the key must not depend on git's order: %q vs %q", a, b)
	}

	cs := []BdComment{{Text: parkedWorkKey([]string{"deadbeef"}) + ": the rest of the line"}}
	if !parkedWorkNoted(cs, parkedWorkKey([]string{"deadbeef"})) {
		t.Errorf("the comment this writes is not the one it reads back: %q", cs[0].Text)
	}
	// A DIFFERENT set is a different fact and must not be deduped away by
	// the line already there.
	if parkedWorkNoted(cs, parkedWorkKey([]string{"deadbeef", "f00dface"})) {
		t.Errorf("a new parked copy must not be swallowed by the line about the old one")
	}
	for _, txt := range []string{
		"",
		"work parked: no bracket",
		"I think the work parked [deadbeef] somewhere",
		"NOTE: work parked [deadbeef]",
	} {
		if parkedWorkNoted([]BdComment{{Text: txt}}, parkedWorkKey([]string{"deadbeef"})) {
			t.Errorf("%q must not read as the harness's own marker", txt)
		}
	}
}

// The join is the bead id at the head of the subject, in both house
// spellings — `<id>: <text>` and the bare `<id>` a vuln-private commit
// message is — and nothing else may be mistaken for one.
func TestParkedLeadingIDReadsBothHouseSpellingsAndNothingElse(t *testing.T) {
	t.Parallel()
	for subject, want := range map[string]string{
		"ranger-base-zzzz1: the back-pointer":    "ranger-base-zzzz1",
		"ranger-base-zzzz1":                      "ranger-base-zzzz1",
		"  ranger-base-zzzz1: leading space":     "ranger-base-zzzz1",
		"a-1: short":                             "a-1",
		"rangerhq-zzz":                           "rangerhq-zzz",
		"Merge branch 'x' into main":             "",
		"fix: a conventional-commits subject":    "",
		"":                                       "",
		"WIP":                                    "",
		"ranger base zzzz1: spaces are not a id": "",
	} {
		if got := parkedLeadingID(subject); got != want {
			t.Errorf("parkedLeadingID(%q) = %q, want %q", subject, got, want)
		}
	}
}

// ─── the census ──────────────────────────────────────────────────────────────

// Six shapes over one repo, because every one of them is on the box today
// and four of them must be silent. The pin is the WHOLE set rather than a
// containment check: a census that is right about the parked commit and
// wrong about the others files a line on a bead whose work is in the tree,
// which is the one failure worse than the silence it replaces.
func TestParkedCommitsSeesOnlyWorkNoLiveBranchCarries(t *testing.T) {
	t.Parallel()
	repo := wtRepo(t)

	// (1) landed: on main, so not parked at all.
	commitIn(t, repo, "landed.txt", "landed\n", "a-1: landed work")
	// (2) held: parked, no branch carries it — the class.
	held := parkCommit(t, repo, "a-2", "a-2: held work", false)
	// (3) in flight: parked AND a live branch carries it.
	parkCommit(t, repo, "a-3", "a-3: still in flight", true)
	// (4) re-landed under another sha: parked, and the base's own log heads
	// a subject with the same id.
	parkCommit(t, repo, "a-4", "a-4: first try", false)
	commitIn(t, repo, "relanded.txt", "again\n", "a-4: second try")
	// (5) no bead id in the subject at all.
	parkCommit(t, repo, "nobead", "Merge branch 'x' into main", false)
	// (6) held, and named at the head of a LINE in another commit's body —
	// which is the shape a whole-message `--grep` reads as landed. MEASURED
	// on this repo 2026-11-14: a parked security fix whose id the base's own
	// source names, in prose about the cost that fix removes, so the
	// message-wide reading went silent on exactly the bead the mechanism is
	// for. The subject is the record; the body is prose.
	bodyNamed := parkCommit(t, repo, "a-5", "a-5: held too", false)
	commitIn(t, repo, "prose.txt", "prose\n",
		"a-6: a bead that mentions another\n\na-5 is the shape that removes this cost, and it is not in the tree.\n")

	cs, err := parkedCommits(repo, "main")
	if err != nil {
		t.Fatalf("parkedCommits: %v", err)
	}
	got := map[string]string{}
	for _, c := range cs {
		got[c.ID] = c.Ref
	}
	want := map[string]string{
		"a-2": retiredTipPrefix + "a-2-" + held[:8],
		"a-5": retiredTipPrefix + "a-5-" + bodyNamed[:8],
	}
	if len(got) != len(want) || len(cs) != len(want) {
		t.Fatalf("the census must name exactly %d parked commit(s), got %d: %v", len(want), len(cs), got)
	}
	for id, ref := range want {
		if got[id] != ref {
			t.Errorf("%s must be named as parked at %q, got %q", id, ref, got[id])
		}
	}

	// A detached checkout has no base a landing could have reached, so there
	// is no question to ask and the answer is not an alarm.
	if cs, err := parkedCommits(repo, ""); err != nil || cs != nil {
		t.Errorf("no base is no question: %v, %v", cs, err)
	}
}

// A repo with no parked refs at all costs one git child and answers nothing,
// which is the ordinary pass in every repo but this one.
func TestParkedCommitsIsSilentWithNothingParked(t *testing.T) {
	t.Parallel()
	repo := wtRepo(t)
	commitIn(t, repo, "a.txt", "a\n", "a-1: work")
	cs, err := parkedCommits(repo, "main")
	if err != nil {
		t.Fatalf("parkedCommits: %v", err)
	}
	if len(cs) != 0 {
		t.Errorf("nothing is parked, so nothing may be named: %v", cs)
	}
}

// ─── the write ───────────────────────────────────────────────────────────────

// parkedWorkRepo is the incident's shape: a closed bead whose commit is
// parked off main, an OPEN bead whose text names that sha (the hold the push
// gate filed), and a second closed bead that is not in the census at all.
func parkedWorkRepo(t *testing.T) (*App, Bd, string, string) {
	t.Helper()
	repo := wtRepo(t)
	commitIn(t, repo, "landed.txt", "landed\n", "a-1: landed work")
	held := parkCommit(t, repo, "a-2", "a-2: held work", false)
	write(t, filepath.Join(repo, "fake-show.json"), `[
	  {"id":"a-1","title":"landed work","status":"closed"},
	  {"id":"a-2","title":"held work","status":"closed"}
	]`)
	write(t, filepath.Join(repo, "fake-list.json"), `[
	  {"id":"a-9","title":"operator: publish a-2's work now or hold it (push gate held `+held[:8]+`)","status":"open"},
	  {"id":"a-8","title":"something else entirely","status":"open"}
	]`)
	return wtApp(t), Bd{Bin: fakeBinFor(t, "bd")}, repo, held
}

// §1 over two passes: the closed bead gets the line, it carries the sha, the
// ref and the hold, and the second pass adds nothing.
func TestNoteParkedWorkNamesTheShaTheRefAndTheHoldOnceOverTwoPasses(t *testing.T) {
	t.Parallel()
	a, bd, repo, held := parkedWorkRepo(t)
	var out, errw strings.Builder

	if n := a.NoteParkedWork(bd, []string{repo}, &out, &errw); n != 1 {
		t.Fatalf("the first pass must note exactly one bead, got %d\nout: %s\nerr: %s", n, out.String(), errw.String())
	}
	texts := parkedWorkTexts(t, repo)
	if len(texts) != 1 {
		t.Fatalf("one comment, got %d: %v", len(texts), texts)
	}
	for _, want := range []string{
		parkedWorkKey([]string{held[:8]}),
		"CLOSED",
		held[:8],
		retiredTipPrefix + "a-2-" + held[:8],
		"named by open bead(s): a-9",
		"ADR 0006 §2",
	} {
		if !strings.Contains(texts[0], want) {
			t.Errorf("the back-pointer must carry %q:\n%s", want, texts[0])
		}
	}
	// The bead that is not in the census, and the open bead that holds the
	// decision, are both left alone.
	for _, no := range []string{"a-1", "a-8", "a-9"} {
		if strings.Contains(texts[0], "\n"+no) {
			t.Errorf("the line must be about a-2 alone: %q appears\n%s", no, texts[0])
		}
	}

	if n := a.NoteParkedWork(bd, []string{repo}, &out, &errw); n != 0 {
		t.Errorf("the second pass must add nothing, noted %d\nerr: %s", n, errw.String())
	}
	if got := parkedWorkMarkers(t, repo, "a-2"); got != 1 {
		t.Errorf("two passes left %d marker(s) on the bead, want 1", got)
	}
}

// An OPEN bead's parked work is the landing sweep's business and is already
// in the queue; the defect this closes is a bead that reads as SHIPPED.
func TestNoteParkedWorkLeavesAnOpenBeadAlone(t *testing.T) {
	t.Parallel()
	a, bd, repo, held := parkedWorkRepo(t)
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-2","title":"held work","status":"open"}]`)
	var out, errw strings.Builder
	if n := a.NoteParkedWork(bd, []string{repo}, &out, &errw); n != 0 {
		t.Errorf("an open bead must not be commented on, noted %d\nout: %s", n, out.String())
	}
	if got := parkedWorkMarkers(t, repo, ""); got != 0 {
		t.Errorf("an open bead carries no back-pointer, got %d comment(s)", got)
	}
	_ = held
}

// The hold is the point of the line, so its ABSENCE is said out loud rather
// than left as a blank: a parked sha no open bead names is the shape that
// escaped twice before any of this was filed.
func TestNoteParkedWorkSaysSoWhenNoOpenBeadHoldsTheSha(t *testing.T) {
	t.Parallel()
	a, bd, repo, _ := parkedWorkRepo(t)
	write(t, filepath.Join(repo, "fake-list.json"), `[{"id":"a-8","title":"something else entirely","status":"open"}]`)
	var out, errw strings.Builder
	if n := a.NoteParkedWork(bd, []string{repo}, &out, &errw); n != 1 {
		t.Fatalf("the bead must still be noted, got %d\nerr: %s", n, errw.String())
	}
	texts := parkedWorkTexts(t, repo)
	if len(texts) != 1 || !strings.Contains(texts[0], "named by NO OPEN BEAD") {
		t.Errorf("an unheld parked sha must say so:\n%v", texts)
	}
}

// A CLOSED bead naming the sha is not a hold: a hold that is answered is
// answered, and reading one as live is how the 2026-11-13 reading of this
// class lost the ask in the first place (a decision written on a bead that
// was already closed left the queue with it).
func TestNoteParkedWorkDoesNotTakeAClosedBeadForTheHold(t *testing.T) {
	t.Parallel()
	a, bd, repo, held := parkedWorkRepo(t)
	write(t, filepath.Join(repo, "fake-list.json"), `[
	  {"id":"a-7","title":"the hold, answered and closed (push gate held `+held[:8]+`)","status":"closed"}
	]`)
	var out, errw strings.Builder
	if n := a.NoteParkedWork(bd, []string{repo}, &out, &errw); n != 1 {
		t.Fatalf("the bead must still be noted, got %d\nerr: %s", n, errw.String())
	}
	texts := parkedWorkTexts(t, repo)
	if len(texts) != 1 {
		t.Fatalf("one comment, got %d: %v", len(texts), texts)
	}
	if strings.Contains(texts[0], "a-7") {
		t.Errorf("a closed bead must not be reported as the live hold:\n%s", texts[0])
	}
	if !strings.Contains(texts[0], "named by NO OPEN BEAD") {
		t.Errorf("with no open bead naming the sha, the line must say so:\n%s", texts[0])
	}
}

// A graph that will not answer the dedupe read must not cost a closed bead
// the correction that belongs under its close comment (commentClosedDirty's
// rule): the pass says so and writes anyway, because a duplicate is visible
// and a missing line is not.
func TestNoteParkedWorkCommentsAnywayWhenTheDedupeReadFails(t *testing.T) {
	t.Parallel()
	a, bd, repo, _ := parkedWorkRepo(t)
	write(t, filepath.Join(repo, "fake-comments-read-fail"), "")
	var out, errw strings.Builder
	// The write's own read-back fails too with this marker in place, so the
	// bead is not counted as noted — what is pinned here is that the pass
	// said why, out loud, and did not go quiet.
	a.NoteParkedWork(bd, []string{repo}, &out, &errw)
	if !strings.Contains(errw.String(), "commenting anyway") {
		t.Errorf("a failed dedupe read must be said out loud:\nout: %s\nerr: %s", out.String(), errw.String())
	}
}
