package posse

// ranger-base-9hm — a bead that settles open a SECOND time is escalated,
// not re-prompted forever.
//
// The mechanism is pinned twice over on purpose. The unit pins call
// noteSettleOpen directly, because that is the only way to say "the second
// one escalates and the first one does not" without running two whole
// passes; the wiring pin runs a real `d.Run` and asserts the comment lands,
// because a rung nothing calls is a rung that does not exist
// (ranger-base-71ki: a pin named for a caller it does not call).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// settleRepo: a repo whose one bead is claimed, in_progress, and NOT closed
// — the disagreement this whole file is about.
func settleRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	write(t, filepath.Join(repo, "fake-ready.json"),
		`[{"id":"a-1","title":"the thing","labels":["go"]}]`)
	write(t, filepath.Join(repo, "fake-show.json"),
		`[{"id":"a-1","title":"the thing","status":"in_progress","assignee":"ranger"}]`)
	return repo
}

// settleDispatcher: a resuming, non-dry dispatcher — the loop shape f0g made
// the autostart default and therefore the only one that can spin.
func settleDispatcher(t *testing.T, b *HerdrBackend) (*Dispatcher, *strings.Builder) {
	t.Helper()
	d := newTestDispatcher(t, b)
	d.Resume = true
	var errb strings.Builder
	d.Err = &errb
	return d, &errb
}

func settlePending(repo, session string) *pendingBead {
	return &pendingBead{
		is:      RepoIssue{BdIssue: BdIssue{ID: "a-1", Title: "the thing"}, Dir: repo},
		persona: "ranger",
		session: session,
	}
}

func readComments(t *testing.T, repo string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "fake-comments.json"))
	if err != nil {
		return nil
	}
	var cs []map[string]any
	if err := json.Unmarshal(b, &cs); err != nil {
		t.Fatalf("fake-comments.json is not JSON: %v\n%s", err, b)
	}
	return cs
}

func settleEscalations(t *testing.T, repo string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "fake-list.json"))
	if err != nil {
		return nil
	}
	var list []map[string]any
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatalf("fake-list.json is not JSON: %v\n%s", err, b)
	}
	var out []map[string]any
	for _, is := range list {
		title, _ := is["title"].(string)
		if settleStuckSource(title) == "a-1" {
			out = append(out, is)
		}
	}
	return out
}

// ─── the markers ─────────────────────────────────────────────────────────────

// Both markers are read back by a later pass, so both must refuse text the
// harness did not write — a persona's own comment must never be counted as
// a settle-open, and a persona's own question bead must never dedupe an
// escalation away.
func TestSettleMarkersRoundTripAndRefuseForeignText(t *testing.T) {
	if got := settleOpenStatus(settleOpenComment("in_progress", "ranger-posse-a-1", "idle")); got != "in_progress" {
		t.Errorf("settle-open status round-trip = %q, want in_progress", got)
	}
	for _, txt := range []string{
		"",
		"settled open: no status at all",
		"settled open [unterminated",
		"I settled open [in_progress]: not at the head",
		"ASSUMED: the bead settled open [in_progress]: nope",
	} {
		if got := settleOpenStatus(txt); got != "" {
			t.Errorf("settleOpenStatus(%q) = %q, want no marker", txt, got)
		}
	}

	if got := settleStuckSource(settleStuckTitle("ranger-base-9hm", "developer-2", "in_progress")); got != "ranger-base-9hm" {
		t.Errorf("stuck source round-trip = %q, want ranger-base-9hm", got)
	}
	for _, title := range []string{
		"",
		"settled open twice: ",
		"settled open twice: !!! not a bead id",
		"why did it settle open twice: a-1 — asking",
	} {
		if got := settleStuckSource(title); got != "" {
			t.Errorf("settleStuckSource(%q) = %q, want no marker", title, got)
		}
	}
}

// ─── the count ───────────────────────────────────────────────────────────────

// The first settle-open is a nudge that got lost (f0g: three measured cases,
// one re-prompt cleared all three). It records the fact and escalates
// nothing.
func TestFirstSettleOpenRecordsAndDoesNotEscalate(t *testing.T) {
	b, _ := newTestBackend(t)
	d, errb := settleDispatcher(t, b)
	repo := settleRepo(t)

	d.noteSettleOpen(settlePending(repo, "ranger-posse-a-1"), "idle", "in_progress")

	cs := readComments(t, repo)
	if len(cs) != 1 {
		t.Fatalf("want exactly one settle-open comment, got %d: %v", len(cs), cs)
	}
	text, _ := cs[0]["text"].(string)
	if settleOpenStatus(text) != "in_progress" {
		t.Errorf("the comment does not carry the bead status it counts against: %q", text)
	}
	if !strings.Contains(text, "ranger-posse-a-1") || !strings.Contains(text, `"idle"`) {
		t.Errorf("the comment must name the session and what it settled as: %q", text)
	}
	if q := settleEscalations(t, repo); len(q) != 0 {
		t.Fatalf("the FIRST settle-open escalated: %v", q)
	}
	if errb.String() != "" {
		t.Errorf("unexpected stderr: %s", errb)
	}
}

// The second one is a standing disagreement, and the whole rung: one
// question bead for the operator, and the stuck bead blocked on it so
// `bd ready` — which is what a pass selects from — stops offering it.
func TestSecondSettleOpenEscalatesAndBlocksTheBead(t *testing.T) {
	b, fake := newTestBackend(t)
	d, errb := settleDispatcher(t, b)
	repo := settleRepo(t)
	appendConfig(t, b.App, "operator: coordinator\n")

	p := settlePending(repo, "ranger-posse-a-1")
	d.noteSettleOpen(p, "idle", "in_progress")
	d.noteSettleOpen(p, "idle", "in_progress")

	qs := settleEscalations(t, repo)
	if len(qs) != 1 {
		t.Fatalf("want exactly one escalation, got %d: %v", len(qs), qs)
	}
	q := qs[0]
	if ls, _ := q["labels"].([]any); len(ls) != 1 || ls[0] != SettleQuestionLabel {
		t.Errorf("the escalation is not the operator's question: labels=%v", q["labels"])
	}
	desc, _ := q["description"].(string)
	for _, want := range []string{"a-1", "ranger-posse-a-1", `settled "idle"`, `says "in_progress"`, "ranger-base-9hm"} {
		if !strings.Contains(desc, want) {
			t.Errorf("the escalation body does not carry %q:\n%s", want, desc)
		}
	}

	qid, _ := q["id"].(string)
	log := bdCalls(t, fake)
	if !strings.Contains(log, "dep add a-1 "+qid) {
		t.Fatalf("the stuck bead was not blocked on %s:\n%s", qid, log)
	}
	if !strings.Contains(log, "--actor "+VerifyActor+" create") {
		t.Errorf("the escalation was not filed as the harness:\n%s", log)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "escalated to "+qid) ||
		!strings.Contains(out, "blocked on "+qid) {
		t.Errorf("the pass did not report the escalation and the block:\n%s", out)
	}
	if errb.String() != "" {
		t.Errorf("unexpected stderr: %s", errb)
	}
}

// ranger-base-23oo — the escalation is filed WITHOUT a `discovered-from`
// edge, because bd will not carry that edge and the block between the same
// pair. The block is the deliverable, so the provenance goes in the body and
// in a comment on the stuck bead, which is fileMergeBlocked's idiom for the
// neighbouring reason.
//
// The pin that matters is the ABSENCE of `--deps` on the create: with it the
// stuck bead stays in `bd ready` and --resume re-prompts it forever, which
// is the exact loop the rung exists to stop. HOW bd fails to take the block
// is a property of the store and not of its version (ranger-base-lpz0o) — a
// SQLite beads.db refuses the `dep add`, a `no-db: true` store accepts it and
// answers `bd ready` with the bead anyway — so this pins the absence, never
// the refusal.
func TestSettleEscalationCarriesProvenanceWithoutTheEdgeThatCostsTheBlock(t *testing.T) {
	b, fake := newTestBackend(t)
	d, errb := settleDispatcher(t, b)
	repo := settleRepo(t)

	p := settlePending(repo, "ranger-posse-a-1")
	d.noteSettleOpen(p, "idle", "in_progress")
	d.noteSettleOpen(p, "idle", "in_progress")

	qs := settleEscalations(t, repo)
	if len(qs) != 1 {
		t.Fatalf("want exactly one escalation, got %d: %v", len(qs), qs)
	}
	qid, _ := qs[0]["id"].(string)

	for _, line := range strings.Split(bdCalls(t, fake), "\n") {
		if strings.Contains(line, "create "+settleStuckPrefix) && strings.Contains(line, "--deps") {
			t.Fatalf("the escalation was filed WITH the edge that costs it the block:\n%s", line)
		}
	}
	if desc, _ := qs[0]["description"].(string); !strings.Contains(desc, discoveredFromMarkerPrefix+"a-1") {
		t.Errorf("the escalation body does not carry its provenance line:\n%s", desc)
	}

	// The other half: the stuck bead names the escalation, and that comment
	// must never be counted as a settle-open by the next pass.
	var breadcrumb string
	for _, c := range readComments(t, repo) {
		if txt, _ := c["text"].(string); strings.Contains(txt, "escalated to "+qid) {
			breadcrumb = txt
		}
	}
	if breadcrumb == "" {
		t.Fatalf("the stuck bead carries no pointer to %s: %v", qid, readComments(t, repo))
	}
	if settleOpenStatus(breadcrumb) != "" {
		t.Errorf("the breadcrumb reads as a settle-open marker and would inflate the count:\n%s", breadcrumb)
	}
	if errb.String() != "" {
		t.Errorf("unexpected stderr: %s", errb)
	}
}

// The fake bd's own pin, and the second half of ranger-base-23oo: ten green
// pins missed the refusal because the fake granted every `dep add`. Real bd
// on a SQLite beads.db — the operator's queue, and what this fake models —
// refuses one whose blocker already carries an edge back to the issue — ANY
// type, `discovered-from` included — and the control is the same add against
// a blocker created without one. A fake that cannot fail this test cannot
// pin escalateSettleOpen either.
//
// The refusal is the SQLite store's, not bd's (ranger-base-lpz0o): the same
// 0.50.3 binary against a `no-db: true` store accepts the add and then
// answers `bd ready` with the blocked bead. That is why nothing outside this
// file asserts the refusal, and why Bd.Ready subtracts `bd blocked` instead
// of trusting either store to have excluded it.
func TestFakeBdRefusesTheDepAddCycleRealBdRefuses(t *testing.T) {
	_, fake := newTestBackend(t)
	_ = fake
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}
	repo := t.TempDir()

	qid, err := bd.Create(repo, BdNew{Title: "q", Deps: []string{"discovered-from:a-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := bd.DepAdd(repo, "a-1", qid, "posse"); err == nil {
		t.Fatalf("the fake granted a dep add real bd refuses as a cycle (a-1 → %s → ... → a-1)", qid)
	} else if !strings.Contains(err.Error(), "would create a cycle") {
		t.Errorf("the refusal is not bd's: %v", err)
	}
	if deps, err := bd.DepList(repo, "a-1"); err != nil || len(deps) != 0 {
		t.Errorf("a refused add left an edge in the graph: %v %v", deps, err)
	}

	// CONTROL: no discovered-from edge, same add, and it lands.
	q2, err := bd.Create(repo, BdNew{Title: "q2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := bd.DepAdd(repo, "a-1", q2, "posse"); err != nil {
		t.Fatalf("the control add was refused too — the fake refuses everything, which pins nothing: %v", err)
	}
	deps, err := bd.DepList(repo, "a-1")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, dep := range deps {
		if dep.ID == q2 {
			found = true
		}
	}
	if !found {
		t.Errorf("the control add reported success and wrote no edge: %v", deps)
	}
}

// Idempotence — the part devops flagged as the one that would bite. One
// question bead per stuck bead, not one per pass, and the dedupe is the
// escalation's TITLE rather than any second write (ranger-base-muoo: bd's
// create commits and then times out, so the id is lost while the bead is
// not).
func TestSettleEscalationIsOncePerBeadNotOncePerPass(t *testing.T) {
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	repo := settleRepo(t)

	p := settlePending(repo, "ranger-posse-a-1")
	for i := 0; i < 4; i++ {
		d.noteSettleOpen(p, "idle", "in_progress")
	}
	if qs := settleEscalations(t, repo); len(qs) != 1 {
		t.Fatalf("four settle-opens filed %d escalations, want 1: %v", len(qs), qs)
	}
	if n := strings.Count(bdCalls(t, fake), "create settled open twice"); n != 1 {
		t.Fatalf("bd create was called %d times for one stuck bead:\n%s", n, bdCalls(t, fake))
	}
	if out := dispatcherOut(d); !strings.Contains(out, "already escalated to") {
		t.Errorf("a later pass must say the escalation already exists:\n%s", out)
	}
}

// An escalation whose create landed but whose blocking edge did not is the
// one half bd can lose on its own, and it is the half that keeps the loop
// spinning. A later pass retries the edge instead of filing a second
// question — and reads the graph back rather than trusting the exit code.
func TestSettleEscalationRetriesTheMissingBlock(t *testing.T) {
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	repo := settleRepo(t)

	p := settlePending(repo, "ranger-posse-a-1")
	d.noteSettleOpen(p, "idle", "in_progress")
	d.noteSettleOpen(p, "idle", "in_progress")
	qid, _ := settleEscalations(t, repo)[0]["id"].(string)

	// The edge is gone — a create that committed and then timed out, or an
	// operator who cleared it. The bead is dispatchable again and sticks.
	write(t, filepath.Join(repo, "fake-deps.json"), `[]`)
	d.Out = &strings.Builder{}
	d.noteSettleOpen(p, "idle", "in_progress")

	if qs := settleEscalations(t, repo); len(qs) != 1 {
		t.Fatalf("the retry filed a second escalation: %v", qs)
	}
	if n := strings.Count(bdCalls(t, fake), "dep add a-1 "+qid); n != 2 {
		t.Fatalf("the missing edge was not re-added (%d dep adds):\n%s", n, bdCalls(t, fake))
	}
	if out := dispatcherOut(d); !strings.Contains(out, "blocked on "+qid) {
		t.Errorf("the retry did not report the bead blocked again:\n%s", out)
	}
}

// A block that did NOT land is the loop still spinning, and the pass owes
// that out loud rather than reporting a stop it did not make. The read is
// what decides: `bd dep add` exits 0 here and the graph still has no edge.
func TestSettleBlockThatDidNotLandIsNamedOnStderr(t *testing.T) {
	b, _ := newTestBackend(t)
	d, errb := settleDispatcher(t, b)
	repo := settleRepo(t)
	write(t, filepath.Join(repo, "fake-dep-add-fail"), "")

	d.blockOnEscalation(RepoIssue{BdIssue: BdIssue{ID: "a-1"}, Dir: repo}, "q-9")

	if !strings.Contains(errb.String(), "NOT blocked on q-9") ||
		!strings.Contains(errb.String(), "re-prompts it") {
		t.Fatalf("an edge that did not land must be reported:\n%s", errb)
	}
	if out := dispatcherOut(d); strings.Contains(out, "blocked on q-9") {
		t.Errorf("the pass claimed a stop it did not make:\n%s", out)
	}
}

// "Cleared when its status changes": the disagreement is between two named
// states, so a bead the operator reopened (or a persona blocked) starts its
// own count rather than escalating on its first settle under the new one.
func TestSettleOpenCountIsPerBeadStatus(t *testing.T) {
	b, _ := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	repo := settleRepo(t)

	p := settlePending(repo, "ranger-posse-a-1")
	d.noteSettleOpen(p, "idle", "in_progress")
	d.noteSettleOpen(p, "idle", "open")

	if qs := settleEscalations(t, repo); len(qs) != 0 {
		t.Fatalf("a settle under a DIFFERENT bead status escalated: %v", qs)
	}
	if cs := readComments(t, repo); len(cs) != 2 {
		t.Fatalf("want one count per status, got %d comments: %v", len(cs), cs)
	}
	d.noteSettleOpen(p, "idle", "open")
	if qs := settleEscalations(t, repo); len(qs) != 1 {
		t.Fatalf("the SECOND settle under the new status did not escalate: %v", qs)
	}
}

// Without --resume nothing re-prompts: the next pass prints "stopped on
// purpose?" and moves on, which is already a bounded answer. Escalating
// there would file question beads for a loop that is not running. --dry-run
// acted on nothing and must leave no state a later pass counts.
func TestSettleOpenWritesNothingWithoutResumeOrUnderDryRun(t *testing.T) {
	for _, c := range []struct {
		name        string
		resume, dry bool
		wantWrites  bool
	}{
		{name: "no --resume", resume: false, dry: false},
		{name: "--dry-run", resume: true, dry: true},
		{name: "--resume", resume: true, dry: false, wantWrites: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			d, _ := settleDispatcher(t, b)
			d.Resume, d.DryRun = c.resume, c.dry
			repo := settleRepo(t)

			d.noteSettleOpen(settlePending(repo, "ranger-posse-a-1"), "idle", "in_progress")

			if got := len(readComments(t, repo)) > 0; got != c.wantWrites {
				t.Fatalf("wrote=%v, want %v: %v", got, c.wantWrites, readComments(t, repo))
			}
		})
	}
}

// ─── what the operator reads ─────────────────────────────────────────────────

// ranger-base-1cc sat on 353 uncommitted lines in its tree, and that — not
// the open bead — is what made this urgent rather than untidy. It is the
// first fact a human deciding whether to kill the session needs.
func TestSettleEscalationNamesUncommittedWorkInTheTree(t *testing.T) {
	// The session tree below is cut under $HOME (DefaultWorktreeRoot), which
	// newTestBackend makes a temp one — without that it lands in the
	// OPERATOR's live ~/.posse/worktrees (ranger-base-9hm, -gvrh).
	b, _ := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	repo := wtRepo(t)
	tr, err := b.App.EnsureSessionTree(repo, "ranger-posse-a-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.writeMeta(&HerdrMeta{Name: "ranger-posse-a-1", Workspace: "w1",
		Repo: repo, Dir: tr.Path, Branch: tr.Branch}); err != nil {
		t.Fatal(err)
	}

	if got := d.settleTreeLines("ranger-posse-a-1"); !strings.Contains(got, "uncommitted: none") {
		t.Fatalf("a clean tree must say so, not go silent:\n%s", got)
	}
	write(t, filepath.Join(tr.Path, "unsaved.go"), "package x // 353 lines of it\n")
	got := d.settleTreeLines("ranger-posse-a-1")
	if !strings.Contains(got, "unsaved.go") || !strings.Contains(got, "A SESSION REAP WOULD DESTROY") {
		t.Fatalf("uncommitted work is the fact that made this urgent:\n%s", got)
	}
	if !strings.Contains(got, tr.Branch) {
		t.Errorf("the operator cannot find the tree without its branch:\n%s", got)
	}

	// A session with no meta at all is "nobody looked", never "nothing is
	// there" — the two are different facts to somebody deciding to kill it.
	if got := d.settleTreeLines("no-such-session"); !strings.Contains(got, "NOT accounted for") {
		t.Errorf("an unreadable session must not read as a clean one:\n%s", got)
	}
}

// ─── the wiring ──────────────────────────────────────────────────────────────

// A real resuming pass over a bead that settles without closing reaches the
// rung. Without this the unit pins above are all about a function nothing
// calls (ranger-base-71ki).
func TestDispatchPassCountsASettleOpen(t *testing.T) {
	b, fake := newTestBackend(t)
	d, _ := settleDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := settleRepo(t)
	planConfig(t, b.App, repo, "")
	idleClaude(t, fake)
	agentPerLaunch(t, fake)

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, `settled "idle" but issue is "in_progress"`) {
		t.Fatalf("the pass did not reach a settle-without-close:\n%s", out)
	}
	cs := readComments(t, repo)
	if len(cs) != 1 {
		t.Fatalf("the pass recorded %d settle-opens, want 1: %v", len(cs), cs)
	}
	if text, _ := cs[0]["text"].(string); settleOpenStatus(text) != "in_progress" {
		t.Fatalf("the pass's comment is not the marker a later pass counts: %q", text)
	}
	if qs := settleEscalations(t, repo); len(qs) != 0 {
		t.Fatalf("one pass escalated: %v", qs)
	}
}
