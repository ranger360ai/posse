package posse

// The bead-loss alarm (rangerhq-fuom), over a real git repo — the census is
// the JSONL's diff history, so there is nothing to fake about it — and the
// same fake bd as the dispatch suite: the live side comes from
// fake-list.json.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// blLine is one issues.jsonl line: bd's export shape, one object per line.
func blLine(id, status string) string {
	return `{"id":"` + id + `","title":"verify: ` + id + `","status":"` + status +
		`","priority":2,"issue_type":"task","assignee":"qa","created_at":"2026-08-19T16:53:35-04:00","updated_at":"2026-08-19T16:53:35-04:00"}`
}

// blList is a `bd list --all` answer holding exactly these ids.
func blList(ids ...string) string {
	var b strings.Builder
	b.WriteString("[")
	for i, id := range ids {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(blLine(id, "open"))
	}
	b.WriteString("]")
	return b.String()
}

// blRepo is a git checkout with a .beads/issues.jsonl and a canned bd.
func blRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".beads"), 0o755)
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return repo
}

// blCommit writes the JSONL as these lines and commits it.
func blCommit(t *testing.T, repo, msg string, lines ...string) {
	t.Helper()
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(repo, beadsDirName, beadsJSONL), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-q", "-m", msg},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

func blSetLive(t *testing.T, repo string, ids ...string) {
	t.Helper()
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(blList(ids...)), 0o644)
}

// The case the check exists for: a bead a commit carried, a later commit
// dropped, and bd can no longer resolve. Nothing in posse deleted it and
// nothing recorded that anything did — the git census is the only witness,
// so the finding has to carry the commit and the bead's own line, which is
// all the provenance left anywhere (rangerhq-fuom).
func TestLostBeadsNamesWhatGitCarriedAndBdLost(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	repo := blRepo(t)
	blCommit(t, repo, "three beads", blLine("q-1", "open"), blLine("q-2", "closed"), blLine("q-3", "open"))
	blCommit(t, repo, "two beads", blLine("q-1", "open"), blLine("q-3", "open"))
	blSetLive(t, repo, "q-1", "q-3")

	lost, err := LostBeads(testBd(t), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(lost) != 1 || lost[0].ID != "q-2" {
		t.Fatalf("want just q-2 lost, got %+v", lost)
	}
	if lost[0].Status != "closed" || lost[0].Title != "verify: q-2" {
		t.Errorf("finding lost the bead's own fields: %+v", lost[0])
	}
	if lost[0].Commit == "" || lost[0].When.IsZero() {
		t.Errorf("finding must name the commit that dropped it: %+v", lost[0])
	}
	if !json.Valid([]byte(lost[0].Record)) || !strings.Contains(lost[0].Record, `"q-2"`) {
		t.Errorf("finding must keep the bead's JSONL line verbatim: %q", lost[0].Record)
	}
}

// A bead can leave the JSONL and come back — an export from a stale database
// followed by a re-import, which is the churn this substrate runs on all day.
// bd is the store of record, so a bead bd still resolves is not lost, however
// the file behaved in between.
func TestLostBeadsIgnoresWhatBdStillResolves(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	repo := blRepo(t)
	blCommit(t, repo, "two", blLine("q-1", "open"), blLine("q-2", "open"))
	blCommit(t, repo, "q-2 momentarily gone", blLine("q-1", "open"))
	blCommit(t, repo, "q-2 back", blLine("q-1", "open"), blLine("q-2", "open"))
	blSetLive(t, repo, "q-1", "q-2")

	lost, err := LostBeads(testBd(t), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(lost) != 0 {
		t.Fatalf("a bead bd resolves is not lost: %+v", lost)
	}
}

// A deletion somebody owns is not a loss. The ledger is the record the
// deletion owes; once it is written the check goes quiet about that id, so
// the alarm stays about the *silent* ones.
func TestRecordedDeletionSilencesTheCheck(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	repo := blRepo(t)
	blCommit(t, repo, "two", blLine("q-1", "open"), blLine("q-2", "open"))
	blCommit(t, repo, "one", blLine("q-1", "open"))
	blSetLive(t, repo, "q-1")

	lost, err := LostBeads(testBd(t), repo)
	if err != nil || len(lost) != 1 {
		t.Fatalf("setup: lost=%+v err=%v", lost, err)
	}
	now := time.Date(2026, 8, 21, 15, 7, 0, 0, time.UTC)
	if err := RecordDeletions(repo, "duplicate of q-1", "coordinator", lost, now); err != nil {
		t.Fatal(err)
	}
	again, err := LostBeads(testBd(t), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("a recorded deletion must not still read as lost: %+v", again)
	}

	// The ledger is also the last copy of the bead: the row is gone from bd
	// and a restore has nothing else to be built from.
	ledger, err := ReadDeletionLedger(repo)
	if err != nil {
		t.Fatal(err)
	}
	recs, ok := ledger["q-2"]
	if !ok || len(recs) != 1 {
		t.Fatalf("ledger lost the id: %+v", ledger)
	}
	rec := recs[0]
	if rec.Reason != "duplicate of q-1" || rec.By != "coordinator" || !rec.At.Equal(now) {
		t.Errorf("ledger must say who and why: %+v", rec)
	}
	// And WHICH removal it owns: without the commit the record exempts the
	// id forever, and a bead restored and lost again leaves in silence
	// (rangerhq-6he5).
	if rec.Commit == "" || rec.Commit != lost[0].Commit {
		t.Errorf("ledger must name the removal it covers: %q want %q", rec.Commit, lost[0].Commit)
	}
	var kept BdIssue
	if err := json.Unmarshal(rec.Record, &kept); err != nil || kept.ID != "q-2" {
		t.Errorf("ledger must keep the bead itself: %s (%v)", rec.Record, err)
	}
}

// Appending, not rewriting: a second recording keeps the first. The ledger is
// an audit trail, and an audit trail that a later write can shorten is not
// one.
func TestRecordDeletionsAppends(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	repo := blRepo(t)
	now := time.Now()
	if err := RecordDeletions(repo, "first", "coordinator", []LostBead{{ID: "q-1", Record: blLine("q-1", "open")}}, now); err != nil {
		t.Fatal(err)
	}
	if err := RecordDeletions(repo, "second", "devops", []LostBead{{ID: "q-2", Record: blLine("q-2", "open")}}, now); err != nil {
		t.Fatal(err)
	}
	ledger, err := ReadDeletionLedger(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) != 2 || len(ledger["q-1"]) != 1 || len(ledger["q-2"]) != 1 ||
		ledger["q-1"][0].Reason != "first" || ledger["q-2"][0].Reason != "second" {
		t.Fatalf("second recording must not shorten the ledger: %+v", ledger)
	}
}

// No git checkout, or a JSONL git has never seen, means no census — and no
// census is not a finding and not an error. This runs on every configured
// repo at the head of a dispatch pass; a repo it cannot look into must not
// stop the fleet or say so every pass.
func TestLostBeadsQuietWithoutACensus(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bare := t.TempDir()
	blSetLive(t, bare)
	lost, err := LostBeads(testBd(t), bare)
	if err != nil || len(lost) != 0 {
		t.Fatalf("a dir with no git census: lost=%+v err=%v", lost, err)
	}

	repo := blRepo(t) // a checkout whose JSONL was never committed
	blSetLive(t, repo)
	lost, err = LostBeads(testBd(t), repo)
	if err != nil || len(lost) != 0 {
		t.Fatalf("a repo with no committed JSONL: lost=%+v err=%v", lost, err)
	}
}

// The dispatch pass says it out loud. bd's auto-import deletes rows and logs
// nothing when it does, so the pass naming the id is the difference between a
// loss found in an afternoon and a loss found never — but a lost bead is
// already lost, so it is a line on stderr and never a gate.
func TestWarnLostBeadsNamesEachOnErrw(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	repo := blRepo(t)
	blCommit(t, repo, "two", blLine("q-1", "open"), blLine("q-2", "open"))
	blCommit(t, repo, "one", blLine("q-1", "open"))
	blSetLive(t, repo, "q-1")

	var errb strings.Builder
	n := b.App.WarnLostBeads(testBd(t), []string{repo}, &errb)
	if n != 1 {
		t.Fatalf("want 1 lost bead named, got %d: %s", n, errb.String())
	}
	line := errb.String()
	if !strings.Contains(line, "bead-loss:") || !strings.Contains(line, "q-2") {
		t.Errorf("the alarm must name the bead: %q", line)
	}
	if !strings.Contains(line, "posse beads check") {
		t.Errorf("the alarm must say what to run next: %q", line)
	}
}

// blRedirect points repo's .beads at target's, the way ADR 0012 D3-C points
// the public working copy at the instance repo's database.
func blRedirect(t *testing.T, repo, target string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, beadsDirName, beadsRedirect), []byte(target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The D3-C shape (rangerhq-92bv): the only `beads:` entry is a working copy
// whose .beads holds a redirect and no tracked jsonl, so its own git history
// carries no census at all. Walking it finds nothing and keeps finding
// nothing — the alarm still runs, still says nothing, and nobody learns it
// stopped watching. The census has to follow the redirect into the repo that
// does track the jsonl, and the ledger has to land there too, because that
// is where git can keep it.
func TestLostBeadsFollowsTheBeadsRedirect(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	store := blRepo(t) // the instance repo: the database and the census
	blCommit(t, store, "two", blLine("q-1", "open"), blLine("q-2", "open"))
	blCommit(t, store, "one", blLine("q-1", "open"))

	work := blRepo(t) // the `beads:` entry: a redirect and nothing else
	blRedirect(t, work, filepath.Join(store, beadsDirName))
	blSetLive(t, work, "q-1")

	lost, err := LostBeads(testBd(t), work)
	if err != nil {
		t.Fatalf("LostBeads: %v", err)
	}
	if len(lost) != 1 || lost[0].ID != "q-2" {
		t.Fatalf("the census must follow the redirect into %s: %+v", store, lost)
	}
	if lost[0].Commit == "" || lost[0].Record == "" {
		t.Errorf("a finding needs the redirect target's provenance: %+v", lost[0])
	}

	var errb strings.Builder
	if n := b.App.WarnLostBeads(testBd(t), []string{work}, &errb); n != 1 {
		t.Fatalf("the pass must name it too, got %d: %s", n, errb.String())
	}
	if !strings.Contains(errb.String(), "q-2") {
		t.Errorf("the alarm must name the bead: %q", errb.String())
	}

	// Owning it writes the ledger where git tracks it — the redirect
	// target's repo, not the working copy, which gitignores .beads whole.
	if err := RecordDeletions(work, "migrated", "operator", lost, time.Now()); err != nil {
		t.Fatalf("RecordDeletions: %v", err)
	}
	if got := DeletionLedgerPath(work); got != filepath.Join(store, beadsDirName, beadsDeleted) {
		t.Errorf("ledger path %q must be in the redirect target", got)
	}
	if _, err := os.Stat(filepath.Join(work, beadsDirName, beadsDeleted)); !os.IsNotExist(err) {
		t.Errorf("nothing may be written into the redirecting copy: %v", err)
	}
	again, err := LostBeads(testBd(t), work)
	if err != nil || len(again) != 0 {
		t.Fatalf("the ledger must silence it through the redirect: %+v %v", again, err)
	}
}

// Which redirect forms resolve, and what happens to one that does not. bd
// accepts an absolute path (what the cut-over runbook writes) and a path
// relative to the repo root (what `bd worktree create` writes); a redirect
// naming something that is not there makes bd warn and read the local
// .beads, so the census must read the local one too — censusing a directory
// bd is not using is the same blindness by a different route.
func TestBeadsRedirectFormsAndFallback(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	store := blRepo(t)
	blCommit(t, store, "two", blLine("q-1", "open"), blLine("q-2", "open"))
	blCommit(t, store, "one", blLine("q-1", "open"))
	storeBeads := filepath.Join(store, beadsDirName)

	work := blRepo(t)
	rel, err := filepath.Rel(work, storeBeads)
	if err != nil {
		t.Fatal(err)
	}
	blRedirect(t, work, rel)
	blSetLive(t, work, "q-1")
	lost, err := LostBeads(testBd(t), work)
	if err != nil || len(lost) != 1 || lost[0].ID != "q-2" {
		t.Fatalf("a repo-root-relative redirect (%s) must resolve: %+v %v", rel, lost, err)
	}

	// A dangling redirect over a repo that has a census of its own: bd falls
	// back to the local .beads, and so does the census.
	local := blRepo(t)
	blCommit(t, local, "two", blLine("p-1", "open"), blLine("p-2", "open"))
	blCommit(t, local, "one", blLine("p-1", "open"))
	blRedirect(t, local, filepath.Join(t.TempDir(), "gone", beadsDirName))
	blSetLive(t, local, "p-1")
	lost, err = LostBeads(testBd(t), local)
	if err != nil || len(lost) != 1 || lost[0].ID != "p-2" {
		t.Fatalf("a redirect that resolves nowhere must census what bd reads: %+v %v", lost, err)
	}

	// And a dir with no redirect is untouched: the pre-cut-over shape.
	if got := beadsHome(local + "/nope"); got != filepath.Join(local, "nope", beadsDirName) {
		t.Errorf("no redirect must mean no hop: %q", got)
	}
}

// The compatibility arm, and the whole of what rangerhq-fknq left of it: a
// ledger an rhq older than dc2bc16 wrote carries no commit on any line, and
// it must go on silencing the deletions it already owns rather than start
// alarming about them. Narrowing the arm to "no record for this id names a
// commit" keeps exactly this case and drops the one where a commit-less line
// rides alongside modern records and re-exempts the id.
func TestACommitlessLedgerStillExempts(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	repo := blRepo(t)
	blCommit(t, repo, "two", blLine("q-1", "open"), blLine("q-2", "open"))
	blCommit(t, repo, "one", blLine("q-1", "open"))
	blSetLive(t, repo, "q-1")
	if lost, err := LostBeads(testBd(t), repo); err != nil || len(lost) != 1 {
		t.Fatalf("setup: lost=%+v err=%v", lost, err)
	}

	// What an rhq older than dc2bc16 writes: no commit field at all.
	legacy, err := json.Marshal(DeletionRecord{
		ID: "q-2", Reason: "duplicate of q-1", By: "coordinator", At: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(beadsPath(repo, beadsDeleted), append(legacy, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := LostBeads(testBd(t), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("a ledger written before the commit field must not start alarming: %+v", again)
	}
}
