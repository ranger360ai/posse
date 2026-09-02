package posse

// ranger-base-tc2pp — ADR 0041 §1–§2: a close that leaves uncommitted paths
// in its session tree is written ON THE BEAD, once, and routed back to the
// closer as one bead.
//
// The pins are laid out the way settleopen_test.go lays its own out, and for
// the same reason (ranger-base-71ki: a rung named for a caller it does not
// call). The markers are pinned as text, because they are the dedupe keys and
// a key that drifts is a duplicate every pass; then each of the three SITES
// is driven for real — the judged close, the sweep, and the kill's landing —
// because the whole finding is that all three of them saw the dirt and none
// of them wrote it anywhere a reader would look.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// closedDirtyMarkers counts what the bead carries: how many comments open
// with the §1 marker, from every site between them.
func closedDirtyMarkers(t *testing.T, repo string) int {
	t.Helper()
	n := 0
	for _, c := range readComments(t, repo) {
		if txt, _ := c["text"].(string); strings.HasPrefix(txt, closedDirtyPrefix) {
			n++
		}
	}
	return n
}

// closedDirtyBeads is every bead filed with the §2 title for this bead id.
// Read out of the store the fake keeps rather than off the create's exit
// code, for Bd.DepAdd's reason: what the graph HOLDS is the fact, and a
// create that timed out having committed the issue is exactly the shape the
// dedupe exists for.
func closedDirtyBeads(t *testing.T, repo, id string) []map[string]any {
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
		if title, _ := is["title"].(string); strings.HasPrefix(title, closedDirtyTitlePrefix+id+" ") {
			out = append(out, is)
		}
	}
	return out
}

// ─── the markers ─────────────────────────────────────────────────────────────

// Both keys are read back by a later pass and by two other sites, so both
// must refuse text the harness did not write: a persona's own comment must
// never suppress the correction, and a persona's own bead must never dedupe
// the handoff away.
func TestClosedDirtyMarkersAreAtTheHeadAndRefuseForeignText(t *testing.T) {
	t.Parallel()
	tr := &SessionTree{Repo: "/r", Path: "/w/t", Branch: "posse/ranger-r-a-1", Base: "main"}
	o := MergeOutcome{Branch: tr.Branch, Base: "main", Commits: 0, Dirty: []string{"NOTES.md", "a_test.go"}}
	got := closedDirtyComment(tr, o)

	for _, want := range []string{
		closedDirtyPrefix + "2 path(s)]: ", "NOTES.md a_test.go", "in /w/t",
		"0 commit(s) on posse/ranger-r-a-1", "nothing carries these",
		"committed or discarded", "posse worktrees",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the §1 comment must carry %q:\n%s", want, got)
		}
	}
	if !closedDirtyNoted([]BdComment{{Text: got}}) {
		t.Errorf("the comment this writes is not the one it reads back:\n%s", got)
	}
	for _, txt := range []string{
		"",
		"closed dirty: no bracket",
		"I left it closed dirty [2 path(s)]: not at the head",
		"NOTE: the tree is closed dirty [1 path(s)]",
	} {
		if closedDirtyNoted([]BdComment{{Text: txt}}) {
			t.Errorf("%q must not read as the harness's own marker", txt)
		}
	}

	title := closedDirtyTitle("a-1", 2, "posse/ranger-r-a-1")
	if !strings.HasPrefix(title, closedDirtyTitlePrefix+"a-1 ") {
		t.Errorf("the bead id must sit at a fixed offset in the title: %q", title)
	}
	for _, want := range []string{"2 uncommitted path(s)", "posse/ranger-r-a-1"} {
		if !strings.Contains(title, want) {
			t.Errorf("the §2 title must carry %q: %q", want, title)
		}
	}
}

// ─── §1, over two of the three sites (ADR 0041 Verification 2) ───────────────

// closedDirtyStrand is the incident's shape: a bead the store calls closed,
// a session tree holding a commit that cannot land (the base moved under it)
// AND one path no commit holds. The blocked merge is what keeps the sweep
// looking at this tree on every later pass — a tree with nothing ahead of its
// base is skipped before it is read — so it is what makes "mergeBack THEN the
// sweep" a two-site question at all.
func closedDirtyStrand(t *testing.T) (*Dispatcher, string, string) {
	t.Helper()
	return wtqaPassWithWork(t, func(repo, tree string) {
		commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
		write(t, filepath.Join(tree, "forgotten.txt"), "never committed\n")
	})
}

// The headline, and the correction that was missing when ranger-base-yeg1
// closed: the dirty set is under the close comment, on the bead, where the
// next reader copies from — and it is there exactly once however many
// launcher moments read that tree.
func TestClosedDirtyIsWrittenOnTheBeadOnceAcrossTheSites(t *testing.T) {
	t.Parallel()
	d, repo, tree := closedDirtyStrand(t)

	if n := closedDirtyMarkers(t, repo); n != 1 {
		t.Fatalf("the judged close wrote %d `%s` comments, want 1:\n%s", n, closedDirtyPrefix, dispatcherOut(d))
	}
	txt := ""
	for _, c := range readComments(t, repo) {
		if s, _ := c["text"].(string); strings.HasPrefix(s, closedDirtyPrefix) {
			txt = s
		}
	}
	for _, want := range []string{"forgotten.txt", "1 path(s)", "posse/"} {
		if !strings.Contains(txt, want) {
			t.Errorf("the comment must name %q:\n%s", want, txt)
		}
	}

	// …and now the SWEEP, which reads the same tree on every pass forever.
	// Nothing left to dispatch: this is the pass that lands what nobody
	// watched (landsweep.go).
	write(t, filepath.Join(repo, "fake-ready.json"), `[]`)
	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)
	if !strings.Contains(out, "closed, and this part did not land") {
		t.Fatalf("the sweep never reached this tree, so it pins nothing:\n%s", out)
	}
	if n := closedDirtyMarkers(t, repo); n != 1 {
		t.Errorf("after mergeBack and the sweep the bead carries %d `%s` comments, want 1:\n%s",
			n, closedDirtyPrefix, out)
	}
	// §4, both halves: the close is the persona's write and the paths are
	// the persona's to ship. The launcher touches neither.
	// The argv shape only: the handoff's own description says the launcher
	// does NOT reopen the bead, and a substring scan for the word would find
	// that sentence and call the pin green over its own prose.
	if log := bdCalls(t, fakeDirOf(t)); strings.Contains(log, "update a-1 --status open") {
		t.Errorf("the launcher reopened the bead:\n%s", log)
	}
	if st := mustGit(t, tree, "status", "--porcelain"); !strings.Contains(st, "forgotten.txt") {
		t.Errorf("the launcher committed the paths for the closer: %q", st)
	}
	if _, err := os.Stat(filepath.Join(tree, "forgotten.txt")); err != nil {
		t.Errorf("the uncommitted work was destroyed: %v", err)
	}
}

// ─── §2, the handoff (ADR 0041 Verification 3) ───────────────────────────────

// One dirty close, one bead: assigned to the closer, in the code lane, P1,
// with the provenance that survives a create bd commits and then fails on
// (verifyMarkerPrefix) — and a second pass over the same tree files none.
func TestClosedDirtyFilesOneBeadForTheCloserAndOnlyOne(t *testing.T) {
	t.Parallel()
	d, repo, _ := closedDirtyStrand(t)

	filed := closedDirtyBeads(t, repo, "a-1")
	if len(filed) != 1 {
		t.Fatalf("want exactly one closed-dirty handoff, got %d:\n%s", len(filed), dispatcherOut(d))
	}
	desc, _ := filed[0]["description"].(string)
	for _, want := range []string{
		"forgotten.txt", "posse/", discoveredFromMarkerPrefix + "a-1",
		"COMMIT them under a-1", "DISCARD them", "posse kill",
	} {
		if !strings.Contains(desc, want) {
			t.Errorf("the handoff must name %q:\n%s", want, desc)
		}
	}
	log := bdCalls(t, fakeDirOf(t))
	if !strings.Contains(log, "create "+closedDirtyTitlePrefix+"a-1 ") {
		t.Fatalf("no closed-dirty create in the bd log:\n%s", log)
	}
	for _, want := range []string{"-a ranger", "-l code", "--deps discovered-from:a-1", "-p 1"} {
		if !strings.Contains(log, want) {
			t.Errorf("the handoff was filed without %q:\n%s", want, log)
		}
	}

	// The sweep runs every pass, and a bead per pass is the objection
	// landsweep.go's header raises. The open title is the answer.
	write(t, filepath.Join(repo, "fake-ready.json"), `[]`)
	d2 := newTestDispatcher(t, d.HB)
	dispatcherErr(t, d2)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if again := closedDirtyBeads(t, repo, "a-1"); len(again) != 1 {
		t.Errorf("a second pass filed %d handoffs, want the first one only:\n%s", len(again), dispatcherOut(d2))
	}
}

// The other arm, and the whole of §5: eight of the twelve commit-less closes
// measured were correct — design, question and verify closes produce no
// commit — so an empty branch is not a finding and a clean tree writes
// nothing at all.
func TestACleanCloseWritesNothingOnTheBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "closed with no commit on posse/") {
		t.Fatalf("the fixture is not the zero-commit close it claims to be:\n%s", out)
	}
	if n := closedDirtyMarkers(t, repo); n != 0 {
		t.Errorf("a clean tree got %d `%s` comment(s):\n%s", n, closedDirtyPrefix, out)
	}
	if filed := closedDirtyBeads(t, repo, "a-1"); len(filed) != 0 {
		t.Errorf("a clean close filed %d handoff(s) at its closer:\n%s", len(filed), out)
	}
}

// The belt inside the rung, which the three sites' own `if len(o.Dirty) > 0`
// blocks otherwise hide: without a pin that calls it directly, deleting the
// guard changes nothing anywhere and the line is unreachable. A clean tree
// must not so much as ASK bd a question — the read is what a dedupe costs,
// and the ordinary close is clean.
func TestNoteClosedDirtyAsksBdNothingOnACleanTree(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	before := bdCalls(t, fake)
	tr := &SessionTree{Repo: t.TempDir(), Path: "/w/t", Branch: "posse/ranger-r-a-1", Base: "main"}
	quiet := func(string, ...any) {}
	noteClosedDirty(Bd{Bin: exe}, tr.Repo, "a-1", "ranger", tr,
		MergeOutcome{Branch: tr.Branch, Base: "main", Commits: 3, Merged: true}, quiet, quiet)
	if after := bdCalls(t, fake); after != before {
		t.Errorf("a clean tree ran bd:\n%s", strings.TrimPrefix(after, before))
	}
}

// ─── §1 at the third site: the kill's landing ────────────────────────────────

// closedDirtyKillSession builds a dispatched session with its OWN worktree,
// no pass having judged anything: the sweep cannot see a tree with nothing
// ahead of its base, so this is where a close nobody watched and nobody
// committed is finally read.
func closedDirtyKillSession(t *testing.T, status string) (*HerdrBackend, string, string) {
	t.Helper()
	b, _ := newTestBackend(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b.Bd = Bd{Bin: exe}
	repo := wtqaRepo(t, b.App, `[]`, `[{"id":"a-1","status":"`+status+`","assignee":"ranger"}]`)
	name := SessionForBead("ranger", repo, "a-1")
	if err := b.CreateSession(NewSessionOpts{Name: name, Dir: repo, Cmd: "true", Bead: "a-1", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta(name)
	if !ok || m.Branch == "" {
		t.Fatalf("the session has no worktree of its own: %+v", m)
	}
	write(t, filepath.Join(m.Dir, "forgotten.txt"), "never committed\n")
	return b, repo, name
}

func TestKillLandingWritesTheDirtySetOnAClosedBead(t *testing.T) {
	t.Parallel()
	b, repo, name := closedDirtyKillSession(t, "closed")

	if _, err := b.ForceKillSessionAndLand(name); err != nil {
		t.Fatal(err)
	}
	if n := closedDirtyMarkers(t, repo); n != 1 {
		t.Fatalf("the kill's landing wrote %d `%s` comments, want 1", n, closedDirtyPrefix)
	}
	if filed := closedDirtyBeads(t, repo, "a-1"); len(filed) != 1 {
		t.Errorf("the kill's landing filed %d handoffs, want 1", len(filed))
	}
}

// The wrong arm: a kill can take an OPEN bead's session — the reap guard
// refuses that pair only as far as --force — and a persona's work in
// progress is not a close that did not land. §1 is about a CLOSED bead.
func TestKillLandingSaysNothingWhenTheBeadIsStillOpen(t *testing.T) {
	t.Parallel()
	b, repo, name := closedDirtyKillSession(t, "in_progress")

	if _, err := b.ForceKillSessionAndLand(name); err != nil {
		t.Fatal(err)
	}
	if n := closedDirtyMarkers(t, repo); n != 0 {
		t.Errorf("an open bead was told its own tree is dirty %d time(s)", n)
	}
	if filed := closedDirtyBeads(t, repo, "a-1"); len(filed) != 0 {
		t.Errorf("an open bead's working tree filed %d handoff(s)", len(filed))
	}
}

// ─── §1 at the second site, on its own ───────────────────────────────────────

// The pin above asserts ONE comment after both sites and would stay green
// over a sweep wired to nothing, which is the shape that makes a control
// green over the escape it is meant to catch. This is the other half: the
// incident's own board (landsweep_test.go's nurlStranded — a close NOBODY
// watched, so mergeBack never ran on it at all) plus one uncommitted path.
// If the sweep does not write it here, nothing ever does.
func TestTheSweepWritesTheDirtySetOnACloseNobodyWatched(t *testing.T) {
	t.Parallel()
	d, repo, tr := nurlStranded(t, "closed", true)
	// The closer, in the store of record's own words: bd records no close
	// actor, so the assignee that held the bead is the honest answer to who
	// this goes back to (verifyCloser), and a dispatched bead always has one
	// — the claim is the fence every launch passes through.
	write(t, filepath.Join(repo, "fake-show.json"), `[{"id":"a-1","status":"closed","assignee":"ranger"}]`)
	write(t, filepath.Join(tr.Path, "forgotten.txt"), "never committed\n")
	// The base moves under the branch, so the merge cannot finish and the
	// tree is still there to be read on this pass and every later one.
	commitIn(t, repo, "fix.txt", "the operator's line\n", "main: conflicting")
	dispatcherErr(t, d)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n := closedDirtyMarkers(t, repo); n != 1 {
		t.Fatalf("the sweep wrote %d `%s` comments over a close nobody watched, want 1:\n%s",
			n, closedDirtyPrefix, out)
	}
	if filed := closedDirtyBeads(t, repo, "a-1"); len(filed) != 1 {
		t.Fatalf("the sweep filed %d handoffs, want 1:\n%s", len(filed), out)
	}
	if !strings.Contains(bdCalls(t, fakeDirOf(t)), "-a ranger") {
		t.Errorf("the sweep filed the handoff at nobody — the closer is the bead's assignee")
	}
}
