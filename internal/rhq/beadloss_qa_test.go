package rhq

// QA pins for the bead-loss alarm (rangerhq-b33n, verifying the close
// of rangerhq-fuom). The mechanism's whole contract is that a bead cannot
// leave silently, so every pin here is a FALSE NEGATIVE: a real loss the
// census or the ledger says nothing about. Helpers are private to this file
// (qbl*) so the pins survive whatever happens to beadloss_test.go.

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

func qblLine(id, status string) string {
	return `{"id":"` + id + `","title":"verify: ` + id + `","status":"` + status +
		`","priority":2,"issue_type":"task","assignee":"qa"}`
}

func qblRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	qblGit(t, repo, "init", "-q")
	qblGit(t, repo, "config", "user.email", "t@example.com")
	qblGit(t, repo, "config", "user.name", "t")
	return repo
}

func qblGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}

// qblCommit writes the JSONL as exactly these lines and commits it.
func qblCommit(t *testing.T, repo, msg string, lines ...string) {
	t.Helper()
	qblWrite(t, repo, lines...)
	qblGit(t, repo, "add", "-A")
	qblGit(t, repo, "commit", "-q", "-m", msg)
}

func qblWrite(t *testing.T, repo string, lines ...string) {
	t.Helper()
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(repo, beadsDirName, beadsJSONL), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// qblLive is what the fake bd answers `list --all` with — the store of record.
func qblLive(t *testing.T, repo string, ids ...string) {
	t.Helper()
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, qblLine(id, "open"))
	}
	if err := os.WriteFile(filepath.Join(repo, "fake-list.json"),
		[]byte("["+strings.Join(parts, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func qblLost(t *testing.T, repo string) []LostBead {
	t.Helper()
	lost, err := LostBeads(testBd(t), repo)
	if err != nil {
		t.Fatalf("LostBeads: %v", err)
	}
	return lost
}

// The ledger records a deletion; it must not exempt the id for the life of
// the repo. `bd import` is the operator's documented next step for the three
// ids already recorded here — restore one and lose it again and the alarm
// that exists to catch exactly this never rings (rangerhq-6he5).
func TestLedgerDoesNotExemptALaterLoss(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommit(t, repo, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblCommit(t, repo, "q-2 gone", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")

	first := qblLost(t, repo)
	if len(first) != 1 {
		t.Fatalf("setup: want the first loss, got %+v", first)
	}
	if err := RecordDeletions(repo, "owned", "qa", first, time.Now()); err != nil {
		t.Fatal(err)
	}

	// The operator restores it — bd resolves it again, so it is not lost.
	qblCommit(t, repo, "q-2 restored", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblLive(t, repo, "q-1", "q-2")
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("a bead bd resolves is not lost: %+v", l)
	}

	// And it vanishes again, silently, exactly as it did the first time.
	qblCommit(t, repo, "q-2 gone again", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	again := qblLost(t, repo)
	if len(again) != 1 || again[0].ID != "q-2" {
		t.Fatalf("a loss after the record that silenced it is a new loss: got %+v", again)
	}
}

// `git log -p` prints no diff for a merge commit, so a line the merge
// resolution dropped never becomes a `-{…}` census entry. issues.jsonl is a
// machine-generated, one-line-per-bead file that several writers touch at
// once — taking one side wholesale is how a bead leaves in a merge commit
// (rangerhq-boco).
func TestCensusSeesABeadDroppedByAMergeCommit(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommit(t, repo, "base", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblGit(t, repo, "checkout", "-q", "-b", "side")
	qblCommit(t, repo, "side adds q-3", qblLine("q-1", "open"), qblLine("q-2", "open"), qblLine("q-3", "open"))
	qblGit(t, repo, "checkout", "-q", "-")
	qblCommit(t, repo, "main closes q-1", qblLine("q-1", "closed"), qblLine("q-2", "open"))
	// Conflict resolved by taking one side wholesale: q-2's line is dropped,
	// and the drop exists only in the merge commit.
	exec.Command("git", "-C", repo, "merge", "--no-commit", "--no-ff", "side").Run()
	qblWrite(t, repo, qblLine("q-1", "closed"), qblLine("q-3", "open"))
	qblGit(t, repo, "add", "-A")
	qblGit(t, repo, "commit", "-q", "-m", "merge resolution drops q-2")
	qblLive(t, repo, "q-1", "q-3")

	lost := qblLost(t, repo)
	if len(lost) != 1 || lost[0].ID != "q-2" {
		t.Fatalf("a bead the merge commit dropped is still a loss: got %+v", lost)
	}
}

// The census scanner's error is never read, so a line over its 8MB cap ends
// the walk mid-history and the census comes back partial — with no error and
// no finding, which is the one thing this mechanism may never do. Latent
// today (the longest line this repo ever committed is ~27KB) but it is a
// silent path inside the alarm (rangerhq-boco).
func TestCensusDoesNotTruncateSilentlyOnAnOversizedLine(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	huge := `{"id":"q-big","title":"` + strings.Repeat("x", 9*1024*1024) + `","status":"open"}`
	qblCommit(t, repo, "three and a monster", qblLine("q-1", "open"), qblLine("q-2", "open"), huge)
	qblCommit(t, repo, "q-2 gone", qblLine("q-1", "open"), huge)
	// The monster leaves last, so its line is the first thing the
	// newest-first walk meets — everything older is behind it.
	qblCommit(t, repo, "monster gone", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")

	// Either the walk copes with the line, or it says it could not — what
	// it may not do is come back empty and calm.
	lost, err := LostBeads(testBd(t), repo)
	for _, lb := range lost {
		if lb.ID == "q-2" {
			return
		}
	}
	if err == nil {
		t.Fatalf("an oversized line buried an older loss with no error at all: lost=%+v", lost)
	}
}

// The alarm's whole value is that a dispatch pass rings it. Nothing pinned
// the wiring — WarnLostBeads had one direct-call test and no pass-level one,
// so deleting the block in Dispatcher.Run left the suite green. This asserts
// the three properties the block claims: it runs, it runs under --dry-run
// (where verify-after deliberately does not), and it does not gate the pass.
func TestDispatchPassRingsTheBeadLossAlarmUnderDryRun(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")

	repo := qblRepo(t)
	qblCommit(t, repo, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblCommit(t, repo, "q-2 gone", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	if err := os.WriteFile(filepath.Join(repo, "fake-ready.json"),
		[]byte(`[{"id":"q-1","title":"t","labels":["go"]}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var errb strings.Builder
	d.Err = &errb
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("the alarm must never gate the pass: %v", err)
	}
	if !strings.Contains(errb.String(), "bead-loss: q-2") {
		t.Errorf("a dispatch pass must name what the census lost, got:\n%s", errb.String())
	}
	if n != 1 {
		t.Errorf("the pass still dispatches its ready work, got n=%d\n%s", n, dispatcherOut(d))
	}
}

// ---------------------------------------------------------------------------
// The ledger after rangerhq-6he5 (verify bead rangerhq-vfzl). A record owns
// one removal, so one id legitimately carries one record per removal and
// .beads/deleted.jsonl becomes a real history rather than a set of exempt
// ids. These pin what that history has to mean.

// qblRecord owns whatever the check currently finds.
func qblRecord(t *testing.T, repo string) {
	t.Helper()
	lost := qblLost(t, repo)
	if len(lost) == 0 {
		t.Fatal("setup: nothing to record")
	}
	if err := RecordDeletions(repo, "owned", "qa", lost, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func qblLedgerLines(t *testing.T, repo string) []string {
	t.Helper()
	b, err := os.ReadFile(beadsPath(repo, beadsDeleted))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func qblWriteLedger(t *testing.T, repo string, lines []string) {
	t.Helper()
	if err := os.WriteFile(beadsPath(repo, beadsDeleted), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func qblRedirect(t *testing.T, repo, target string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, beadsDirName, beadsRedirect), []byte(target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// One cycle is what rangerhq-6he5 was filed about; the contract is every
// cycle. `bd import` is a repeatable operator move, so lose/restore/lose can
// run any number of times, and the ledger has to keep saying "this removal is
// owned, that one is not" — which a reader that collapses the file to one
// answer per id can only do while the newest answer happens to be last.
func TestLedgerOwnsEachLossNotTheID(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommit(t, repo, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblLive(t, repo, "q-1", "q-2")

	seen := map[string]bool{}
	for cycle := 1; cycle <= 3; cycle++ {
		qblCommit(t, repo, fmt.Sprintf("q-2 gone %d", cycle), qblLine("q-1", "open"))
		qblLive(t, repo, "q-1")
		lost := qblLost(t, repo)
		if len(lost) != 1 || lost[0].ID != "q-2" {
			t.Fatalf("cycle %d: a loss after the record that silenced the last one is a new loss: %+v", cycle, lost)
		}
		if seen[lost[0].Commit] {
			t.Fatalf("cycle %d: the census must name THIS removal, not the one already owned: %s", cycle, lost[0].Commit)
		}
		seen[lost[0].Commit] = true

		qblRecord(t, repo)
		if l := qblLost(t, repo); len(l) != 0 {
			t.Fatalf("cycle %d: the record just written must silence this removal: %+v", cycle, l)
		}

		qblCommit(t, repo, fmt.Sprintf("q-2 restored %d", cycle), qblLine("q-1", "open"), qblLine("q-2", "open"))
		qblLive(t, repo, "q-1", "q-2")
		if l := qblLost(t, repo); len(l) != 0 {
			t.Fatalf("cycle %d: a bead bd resolves is not lost: %+v", cycle, l)
		}
	}
	// The ledger is the audit trail, not a set of ids: three removals, three
	// records, each naming a different commit.
	if lines := qblLedgerLines(t, repo); len(lines) != 3 {
		t.Errorf("want one record per removal, got %d lines: %v", len(lines), lines)
	}
	if len(seen) != 3 {
		t.Errorf("three removals must be three commits: %v", seen)
	}
}

// The ledger is append-only by contract (TestRecordDeletionsAppends) and now
// carries several records per id, but ReadDeletionLedger keeps only the LAST
// line for an id — so the order of an append-only file that git merges and
// rebases replay decides the verdict. Reversing the lines removes no
// information from an audit trail; it must not turn an owned deletion into a
// finding (rangerhq-fknq).
func TestLedgerLineOrderDoesNotDecideTheVerdict(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommit(t, repo, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblLive(t, repo, "q-1", "q-2")
	for cycle := 1; cycle <= 2; cycle++ {
		qblCommit(t, repo, fmt.Sprintf("q-2 gone %d", cycle), qblLine("q-1", "open"))
		qblLive(t, repo, "q-1")
		qblRecord(t, repo)
		if cycle == 1 {
			qblCommit(t, repo, "q-2 restored", qblLine("q-1", "open"), qblLine("q-2", "open"))
			qblLive(t, repo, "q-1", "q-2")
		}
	}
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("setup: both removals are owned: %+v", l)
	}
	lines := qblLedgerLines(t, repo)
	if len(lines) != 2 {
		t.Fatalf("setup: want one record per removal, got %v", lines)
	}

	qblWriteLedger(t, repo, []string{lines[1], lines[0]})
	if l := qblLost(t, repo); len(l) != 0 {
		t.Errorf("the same records in a different order must give the same verdict: %+v", l)
	}
}

// The `Commit == ""` arm exempts an id the way the ledger did before
// rangerhq-6he5, and last-line-wins hands it the whole answer for that id: one
// record written by a posse that predates the field, appended after any number
// of modern ones, and a real unaccounted loss is silent again. That is 6he5
// itself, reached through the file its fix already writes (rangerhq-fknq).
func TestACommitlessRecordDoesNotReExemptTheID(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommit(t, repo, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblCommit(t, repo, "q-2 gone", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	qblRecord(t, repo)
	qblCommit(t, repo, "q-2 restored", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblLive(t, repo, "q-1", "q-2")
	qblCommit(t, repo, "q-2 gone again", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	if l := qblLost(t, repo); len(l) != 1 {
		t.Fatalf("setup: the second removal is unaccounted for: %+v", l)
	}

	lines := qblLedgerLines(t, repo)
	var legacy DeletionRecord
	if err := json.Unmarshal([]byte(lines[0]), &legacy); err != nil {
		t.Fatal(err)
	}
	legacy.Commit = "" // what a posse older than dc2bc16 writes
	b, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	qblWriteLedger(t, repo, append(lines, string(b)))
	if l := qblLost(t, repo); len(l) != 1 {
		t.Errorf("no record here names the removal the census found, so it stays a finding: %+v", l)
	}

	// Control: the same file, that one line moved off the end. If this half
	// alarms and the half above does not, the verdict came from the order.
	qblWriteLedger(t, repo, append([]string{string(b)}, lines...))
	if l := qblLost(t, repo); len(l) != 1 {
		t.Fatalf("control: an unaccounted loss is a finding: %+v", l)
	}
}

// Census, ledger and git all hang off beadsHome, so under a redirect the
// commit a record names was read in one repo and compared against a census
// walked in another. Keying on the sha is only sound while those are the same
// repo — the D3-C shape is production (rangerhq-92bv), so pin the whole cycle
// through the hop, not just the first silence.
func TestLedgerOwnsALaterLossThroughARedirect(t *testing.T) {
	newTestBackend(t)
	store := qblRepo(t) // the instance repo: database, census and ledger
	work := qblRepo(t)  // the `beads:` entry: a redirect and nothing else
	qblRedirect(t, work, filepath.Join(store, beadsDirName))

	qblCommit(t, store, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblCommit(t, store, "q-2 gone", qblLine("q-1", "open"))
	qblLive(t, work, "q-1")
	qblRecord(t, work)
	if l := qblLost(t, work); len(l) != 0 {
		t.Fatalf("the ledger must silence it through the redirect: %+v", l)
	}

	qblCommit(t, store, "q-2 restored", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblLive(t, work, "q-1", "q-2")
	if l := qblLost(t, work); len(l) != 0 {
		t.Fatalf("a bead bd resolves is not lost: %+v", l)
	}
	qblCommit(t, store, "q-2 gone again", qblLine("q-1", "open"))
	qblLive(t, work, "q-1")
	again := qblLost(t, work)
	if len(again) != 1 || again[0].ID != "q-2" {
		t.Fatalf("a second loss through the redirect is a new loss: %+v", again)
	}
}

// The D3-C alarm is a dispatch-pass line, not a helper return. The unit
// test (TestLostBeadsFollowsTheBeadsRedirect) calls WarnLostBeads directly;
// the pass-level pin (TestDispatchPassRingsTheBeadLossAlarmUnderDryRun)
// uses a repo that tracks its own jsonl. Together they still go green if
// Dispatcher.Run walks dir/.beads itself. This is the cut-over shape:
// beads: names a working copy whose .beads holds only a redirect.
func TestDispatchPassRingsBeadLossThroughRedirect(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	writePersona(t, b.App, "ranger", "[go]")

	store := qblRepo(t)
	qblCommit(t, store, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblCommit(t, store, "q-2 gone", qblLine("q-1", "open"))

	work := qblRepo(t)
	qblRedirect(t, work, filepath.Join(store, beadsDirName))
	qblLive(t, work, "q-1")
	if err := os.WriteFile(filepath.Join(work, "fake-ready.json"),
		[]byte(`[{"id":"q-1","title":"t","labels":["go"]}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+work+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var errb strings.Builder
	d.Err = &errb
	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("the alarm must never gate the pass: %v", err)
	}
	if !strings.Contains(errb.String(), "bead-loss: q-2") {
		t.Errorf("a dispatch pass over a redirected beads: dir must name the store's loss, got:\n%s", errb.String())
	}
	if n != 1 {
		t.Errorf("the pass still dispatches its ready work, got n=%d\n%s", n, dispatcherOut(d))
	}
}

// gitBead runs from inside the resolved .beads so a target that is not at
// its repo root still has a cwd-relative pathspec. The unit tests all put
// .beads at the root; this is the case that justified the cwd choice.
func TestLostBeadsFollowsRedirectIntoNestedBeadsDir(t *testing.T) {
	newTestBackend(t)
	store := qblRepo(t)
	nested := filepath.Join(store, "extra", beadsDirName)
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(lines ...string) {
		t.Helper()
		body := ""
		for _, l := range lines {
			body += l + "\n"
		}
		if err := os.WriteFile(filepath.Join(nested, beadsJSONL), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		qblGit(t, store, "add", "-A")
	}
	write(qblLine("n-1", "open"), qblLine("n-2", "open"))
	qblGit(t, store, "commit", "-q", "-m", "nested two")
	write(qblLine("n-1", "open"))
	qblGit(t, store, "commit", "-q", "-m", "nested drop n-2")

	work := qblRepo(t)
	qblRedirect(t, work, nested)
	qblLive(t, work, "n-1")
	lost := qblLost(t, work)
	if len(lost) != 1 || lost[0].ID != "n-2" {
		t.Fatalf("census must walk extra/.beads from inside it: %+v", lost)
	}
}

func TestBeadsHomeDoesNotHangOnARedirectCycle(t *testing.T) {
	a := qblRepo(t)
	b := qblRepo(t)
	qblRedirect(t, a, filepath.Join(b, beadsDirName))
	qblRedirect(t, b, filepath.Join(a, beadsDirName))
	done := make(chan string, 1)
	go func() { done <- beadsHome(a) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a redirect cycle hung beadsHome")
	}
}

func TestBeadsHomeDoesNotExpandTildeInRedirect(t *testing.T) {
	repo := qblRepo(t)
	qblRedirect(t, repo, "~/no-such-beads")
	got := beadsHome(repo)
	want := filepath.Join(repo, beadsDirName)
	if got != want {
		t.Fatalf("a ~ redirect is dangling for bd and for us: got %q want %q", got, want)
	}
}

// bd 0.49.1 refuses a second hop ("redirect chains not allowed, ignoring
// redirect in <first-hop>/.beads") and reads the first target as a normal
// .beads dir — measured against the real binary in ranger-base-7kw: with a
// jsonl in mid, `bd list` from work answers mid's bead and creates
// mid/beads.db; with none, it errors "no beads database found" and store is
// untouched either way. beadsHome used to follow up to 8 hops, so the census
// walked a store bd is not using and ListAll against real bd errored. D3-C
// is one hop and is not this.
func TestBeadsHomeDoesNotFollowARedirectChain(t *testing.T) {
	store := qblRepo(t)
	mid := qblRepo(t)
	work := qblRepo(t)
	qblRedirect(t, mid, filepath.Join(store, beadsDirName))
	qblRedirect(t, work, filepath.Join(mid, beadsDirName))
	got := beadsHome(work)
	want := filepath.Join(mid, beadsDirName)
	if got != want {
		t.Fatalf("bd ignores a redirect in the target; beadsHome followed to %s, want first hop %s", got, want)
	}
}

// A deletion the operator RECORDED is owned: the ledger exempts it by naming
// the commit that dropped it. But the census keeps, per id, the most recent
// commit that removed the line, and --diff-merges=first-parent (rangerhq-boco)
// made a merge's net diff a removal entry too — so merging the branch that
// dropped the bead re-attributes the drop from the side commit to the merge
// commit. While the exemption was sha equality that un-recorded the deletion
// and it alarmed forever; recording it again silenced it only by appending a
// second ledger line for one deletion, which is the input rangerhq-fknq is
// filed about. sameRemoval is the fix: one removal, two shas
// (ranger-base-ntsz).
// The other half of sameRemoval. Accepting "no re-addition between them"
// alone exempts an id whose record was written on a line of history the
// census never walked: the range from an unmerged branch's commit to a drop
// on main contains no re-addition either — the id was added before the fork
// — so a real, unowned loss on main goes silent, which is the one thing this
// mechanism may never do. Ancestry is what makes the ledger's record a claim
// about THIS history (ranger-base-ntsz).
func TestALedgerRecordOffThisHistoryOwnsNothing(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommit(t, repo, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblGit(t, repo, "branch", "-M", "main")

	// A branch drops q-2 and owns it there. The branch is never merged.
	qblGit(t, repo, "checkout", "-q", "-b", "side")
	qblCommit(t, repo, "side drops q-2", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	qblRecord(t, repo)

	// main loses q-2 on its own. Nothing owns THAT removal, and the census
	// walking main never sees the side commit at all.
	qblGit(t, repo, "checkout", "-q", "main")
	qblCommit(t, repo, "q-2 vanishes on main", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")

	lost := qblLost(t, repo)
	if len(lost) != 1 || lost[0].ID != "q-2" {
		t.Fatalf("a record from an unmerged branch owns no removal on main: got %+v", lost)
	}
}

func TestLedgerRecordSurvivesAMergeOfTheDroppingCommit(t *testing.T) {
	newTestBackend(t)
	repo := qblRepo(t)
	qblCommit(t, repo, "two", qblLine("q-1", "open"), qblLine("q-2", "open"))
	qblGit(t, repo, "branch", "-M", "main")

	// The bead is dropped on a branch, on purpose, and recorded there.
	qblGit(t, repo, "checkout", "-q", "-b", "side")
	qblCommit(t, repo, "side drops q-2", qblLine("q-1", "open"))
	qblLive(t, repo, "q-1")
	qblRecord(t, repo)
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("setup: a recorded deletion is owned, got %+v", l)
	}

	// The branch merges. Nothing about the deletion changed — only which
	// commit git's first-parent diff blames for it.
	qblGit(t, repo, "checkout", "-q", "main")
	qblGit(t, repo, "merge", "-q", "--no-ff", "--no-edit", "side")
	qblLive(t, repo, "q-1")
	if l := qblLost(t, repo); len(l) != 0 {
		t.Fatalf("a merge of the branch that dropped it must not un-record an owned deletion: got %+v", l)
	}

	// And the ledger still holds one record for one deletion.
	if lines := qblLedgerLines(t, repo); len(lines) != 1 {
		t.Errorf("one deletion, one record: got %d lines: %v", len(lines), lines)
	}
}
