package rhq

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScoreIssues(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	at := func(h int) *time.Time { x := t0.Add(time.Duration(h) * time.Hour); return &x }
	issues := []BdIssue{
		{ID: "a", Assignee: "dev", Status: "closed", Created: t0, ClosedAt: at(2)},
		{ID: "b", Assignee: "dev", Status: "closed", Created: t0, ClosedAt: at(10)},
		{ID: "c", Assignee: "dev", Status: "closed", Created: t0, ClosedAt: at(4)},
		{ID: "d", Assignee: "dev", Status: "in_progress"},
		{ID: "e", Assignee: "dev", Status: "open"},
		{ID: "f", Assignee: "dev", Status: "blocked"},
		{ID: "g", Assignee: "other", Status: "closed", CreatedBy: "dev"},
		{ID: "h", Assignee: "other", Status: "closed", CreatedBy: "dev", CloseReason: "Closed as duplicate of x"},
		{ID: "i", Assignee: "", Status: "open", CreatedBy: "dev"},
	}
	s := ScoreIssues("dev", issues, map[string]int{"b": 1})
	want := Score{Persona: "dev", Closed: 3, Reopened: 1, ReposScored: 1, ReposWithHistory: 1, Open: 1, Held: 1, Blocked: 1, AgeAtClose: 4 * time.Hour, Filed: 3, Rejected: 1}
	if s != want {
		t.Errorf("got %+v\nwant %+v", s, want)
	}
	if m := s.Metric("closed-no-reopen"); !strings.Contains(m, "→ 2") {
		t.Errorf("closed-no-reopen: %s", m)
	}
	// A nil reopens map is "no history to read", not "nobody reopened
	// anything": the metric line must say unknown and cap the score, never
	// spend the zero value as a perfect 3 (ranger-base-0tc).
	u := ScoreIssues("dev", issues, nil)
	if u.ReopensKnown() {
		t.Error("nil reopens must score as unknown")
	}
	m := u.Metric("closed-no-reopen")
	if !strings.Contains(m, "3 closed") || !strings.Contains(m, "unknown") || !strings.Contains(m, "≤3") {
		t.Errorf("unknown reopens must read as a ceiling: %s", m)
	}
	if strings.Contains(m, "0 reopened") || strings.Contains(m, "→ 3") {
		t.Errorf("unknown reopens must not print a number: %s", m)
	}
	// Unknown or not, the id stays computed — the scorecard answers it, it
	// just answers with a bound.
	if !MetricComputed("closed-no-reopen") {
		t.Error("closed-no-reopen must count as computed")
	}
	// The PIDs' spelling is canonical; the ADR's original stays an alias
	// (ADR 0001 amendment 2026-08-18).
	for _, id := range []string{"findings-surviving-triage", "findings-survive-triage"} {
		if m := s.Metric(id); !strings.Contains(m, "→ 2") {
			t.Errorf("%s: %s", id, m)
		}
		if !MetricComputed(id) {
			t.Errorf("%s must count as computed", id)
		}
	}
	if m := ScoreIssues("nobody", issues, nil).Metric("findings-surviving-triage"); !strings.Contains(m, "nothing filed") {
		t.Errorf("empty filed: %s", m)
	}
	// A declared id the scorecard cannot answer says what bd would need —
	// never "not in the catalog".
	for id, need := range map[string]string{
		"blocked-honestly":     "dispatch-side outcome",
		"spec-clarity":         "comment scan",
		"suite-green-on-close": "an answerer over what bd shows",
	} {
		m := s.Metric(id)
		if !strings.HasPrefix(m, NotYetComputable) || !strings.Contains(m, need) {
			t.Errorf("%s: %s", id, m)
		}
		if MetricComputed(id) {
			t.Errorf("%s is not computable yet", id)
		}
		if strings.Contains(m, "not in the catalog") {
			t.Errorf("%s: retired wording: %s", id, m)
		}
	}
}

// posse scorecard --catalog: the derived catalog, computable or not, and who
// declares each id. No bd involved.
func TestMetricCatalogReport(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	var out strings.Builder
	if err := a.MetricCatalogReport(&out); err == nil {
		t.Error("no PIDs and no config: must be an error, not an empty table")
	}
	os.MkdirAll(a.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"),
		[]byte("---\nname: developer\nmetrics: [closed-no-reopen, suite-green-on-close]\n---\nYou are developer.\n"), 0o644)
	os.WriteFile(a.ConfigPath, []byte("metric_ids: [escapes-caught]\n"), 0o644)
	if err := a.MetricCatalogReport(&out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"closed-no-reopen", "computed", "developer",
		"suite-green-on-close", "declared",
		"escapes-caught", "config metric_ids:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("catalog missing %q:\n%s", want, got)
		}
	}
	// Sorted, so the listing is stable between runs.
	if i, j := strings.Index(got, "closed-no-reopen"), strings.Index(got, "escapes-caught"); i > j {
		t.Errorf("catalog not sorted:\n%s", got)
	}
}

// Reopens come from the git history of .beads/issues.jsonl: a closed→open
// transition between two commits.
func TestReopensFromGit(t *testing.T) {
	dir := t.TempDir()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	os.MkdirAll(filepath.Join(dir, ".beads"), 0o755)
	write := func(lines ...string) {
		os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		git("add", ".beads/issues.jsonl")
		git("commit", "-q", "-m", "sync")
	}
	if got := ReopensFromGit(dir); got != nil {
		t.Errorf("no history yet must be nil (unknown), got %v", got)
	}
	write(`{"id":"x-1","status":"open"}`, `{"id":"x-2","status":"open"}`)
	write(`{"id":"x-1","status":"closed"}`, `{"id":"x-2","status":"closed"}`)
	write(`{"id":"x-1","status":"in_progress"}`, `{"id":"x-2","status":"closed"}`)
	write(`{"id":"x-1","status":"closed"}`, `{"id":"x-2","status":"closed"}`)
	write(`{"id":"x-1","status":"open"}`, `{"id":"x-2","status":"closed"}`)
	got := ReopensFromGit(dir)
	if got["x-1"] != 2 || got["x-2"] != 0 {
		t.Errorf("want x-1 reopened twice, x-2 never: %v", got)
	}
}

func TestScorecardOutput(t *testing.T) {
	b, _ := newTestBackend(t)
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}
	writePersona(t, b.App, "dev", "[code]")
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\nlabels: [code]\nmetrics: [closed-no-reopen, findings-surviving-triage, suite-green-on-close]\n---\nYou are dev.\n"), 0o644)
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "fake-list.json"), []byte(`[
		{"id":"a-1","status":"closed","assignee":"dev","created_at":"2026-08-01T00:00:00Z","closed_at":"2026-08-01T03:00:00Z"},
		{"id":"a-2","status":"in_progress","assignee":"dev"},
		{"id":"a-3","status":"open","created_by":"dev"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+repo+"\n"), 0o644)

	var out strings.Builder
	if err := b.App.Scorecard(bd, &out, ""); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"persona", "dev", "closed-no-reopen", "1 closed", "findings-surviving-triage", "1 filed",
		"suite-green-on-close", NotYetComputable, "reopened: ?"} {
		if !strings.Contains(s, want) {
			t.Errorf("scorecard missing %q:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "3.0h") {
		t.Errorf("age at close missing:\n%s", s)
	}
}

// The reopen count had the bead-loss census's blindness (rangerhq-92bv),
// by a different route: under ADR 0012 D3-C the one `beads:` entry tracks
// no issues.jsonl of its own, so its history holds fewer than two commits
// and every persona's reopened column reads "?" for the life of the
// instance. Unknown is not a lie, but closed-no-reopen is a crew metric
// and it stops being measurable at cut-over — so this walk has to follow
// .beads/redirect into the repo that actually holds the census.
func TestReopensFromGitFollowsTheBeadsRedirect(t *testing.T) {
	store := blRepo(t)
	blCommit(t, store, "sync", blLine("x-1", "open"), blLine("x-2", "open"))
	blCommit(t, store, "sync", blLine("x-1", "closed"), blLine("x-2", "closed"))
	blCommit(t, store, "sync", blLine("x-1", "open"), blLine("x-2", "closed"))

	work := blRepo(t) // the `beads:` entry: a redirect and no census at all
	if got := ReopensFromGit(work); got != nil {
		t.Fatalf("a repo with no census of its own is unknown, not counted: %v", got)
	}
	blRedirect(t, work, filepath.Join(store, beadsDirName))
	got := ReopensFromGit(work)
	if got == nil {
		t.Fatalf("the reopen count must follow the redirect into %s", store)
	}
	if got["x-1"] != 1 || got["x-2"] != 0 {
		t.Errorf("want x-1 reopened once, x-2 never: %v", got)
	}
}

// Not a copy of the census's fix: that one only ever runs `git log`, whose
// pathspec is cwd-relative, so running from inside .beads was the whole
// hop. This walk also reads blobs, and `git show <rev>:<path>` is
// repo-root-relative — a redirect target whose .beads is not at its repo
// root would resolve to nothing and go quietly back to "?". The blob read
// has to name the path relative to the .beads dir it is standing in.
func TestReopensFromGitReadsBlobsRelativeToTheBeadsDir(t *testing.T) {
	store := blRepo(t)
	beads := filepath.Join(store, "instance", beadsDirName)
	if err := os.MkdirAll(beads, 0o755); err != nil {
		t.Fatal(err)
	}
	commit := func(lines ...string) {
		t.Helper()
		body := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(beads, beadsJSONL), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "sync"}} {
			if out, err := exec.Command("git", append([]string{"-C", store}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
		}
	}
	commit(blLine("y-1", "closed"))
	commit(blLine("y-1", "in_progress"))
	commit(blLine("y-1", "closed"))

	work := blRepo(t)
	blRedirect(t, work, beads)
	got := ReopensFromGit(work)
	if got["y-1"] != 1 {
		t.Errorf("a redirect target whose .beads is below its repo root must still be read: %v", got)
	}
}

// The unit-level hop above is only half of what the bead asked for
// (rangerhq-i6n6): the DONE WHEN is at the command's own level — with
// `beads:` naming a repo whose .beads is a redirect, `posse scorecard`
// must print a reopen COUNT, not the "?" trailer. The column and the
// trailer are driven by reopensKnown (scorecard.go), one nil away from
// each other, so pin the two states of that flag end to end.
func TestScorecardCountsReopensThroughTheBeadsRedirect(t *testing.T) {
	b, _ := newTestBackend(t)
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\nlabels: [code]\nmetrics: [closed-no-reopen]\n---\nYou are dev.\n"), 0o644)

	store := blRepo(t) // the repo that actually tracks the census
	blCommit(t, store, "sync", blLine("x-1", "closed"), blLine("x-2", "closed"))
	blCommit(t, store, "sync", blLine("x-1", "open"), blLine("x-2", "closed"))
	blCommit(t, store, "sync", blLine("x-1", "closed"), blLine("x-2", "closed"))

	work := blRepo(t) // the `beads:` entry: a redirect and no census of its own
	os.WriteFile(filepath.Join(work, "fake-list.json"), []byte(`[
		{"id":"x-1","status":"closed","assignee":"dev"},
		{"id":"x-2","status":"closed","assignee":"dev"}]`), 0o644)
	os.WriteFile(b.App.ConfigPath, []byte("beads:\n  - "+work+"\n"), 0o644)

	card := func() string {
		t.Helper()
		var out strings.Builder
		if err := b.App.Scorecard(bd, &out, "dev"); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	// Before the redirect: honestly unknown, which is the state the bead
	// says the whole instance would be stuck in after cut-over.
	if s := card(); !strings.Contains(s, "reopened: ?") {
		t.Fatalf("a repo with no census of its own must read unknown:\n%s", s)
	}
	// The column and the metric line read the same fact: neither may print
	// a perfect score off the zero value (ranger-base-0tc).
	if s := card(); strings.Contains(s, "0 reopened") || !strings.Contains(s, "reopens unknown") {
		t.Errorf("the closed-no-reopen line must say unknown too:\n%s", s)
	}
	blRedirect(t, work, filepath.Join(store, beadsDirName))
	s := card()
	if strings.Contains(s, "reopened: ?") {
		t.Errorf("the reopen column must follow the redirect into %s:\n%s", store, s)
	}
	// x-1 closed→open once, x-2 never: 2 closed, 1 of them reopened.
	if !strings.Contains(s, "closed-no-reopen") || !strings.Contains(s, "→ 1") {
		t.Errorf("want closed-no-reopen 2 closed → 1 through the hop:\n%s", s)
	}
}

// scorecardRig is a card over a fixed persona and a `beads:` list the test
// writes: one repo per entry, each with its own fake-list.json (and, if the
// test wants it, the fake-list-fail marker that makes bd's own call fail on
// a path that DOES resolve).
func scorecardRig(t *testing.T, dirs ...string) func() (string, error) {
	t.Helper()
	b, _ := newTestBackend(t)
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}
	os.MkdirAll(b.App.AgentsDir, 0o755)
	os.WriteFile(filepath.Join(b.App.AgentsDir, "dev.md"),
		[]byte("---\nname: dev\nlabels: [code]\nmetrics: [closed-no-reopen]\n---\nYou are dev.\n"), 0o644)
	cfg := "beads:\n"
	for _, d := range dirs {
		cfg += "  - " + d + "\n"
	}
	os.WriteFile(b.App.ConfigPath, []byte(cfg), 0o644)
	return func() (string, error) {
		var out strings.Builder
		err := b.App.Scorecard(bd, &out, "dev")
		return out.String(), err
	}
}

// scorecardRepo is one beads repo holding n closed beads for "dev".
func scorecardRepo(t *testing.T, n int, fail bool) string {
	t.Helper()
	dir := t.TempDir()
	var rows []string
	for i := 0; i < n; i++ {
		rows = append(rows, `{"id":"z-`+fmt.Sprint(i)+`","status":"closed","assignee":"dev",`+
			`"created_at":"2026-08-01T00:00:00Z","closed_at":"2026-08-01T01:00:00Z"}`)
	}
	if err := os.WriteFile(filepath.Join(dir, "fake-list.json"), []byte("["+strings.Join(rows, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fail {
		if err := os.WriteFile(filepath.Join(dir, "fake-list-fail"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The bead (ranger-base-ynim): with two configured repos and one the scan
// could not read, the card printed a full, healthy-looking table computed
// from half the data and said nothing about the half it never read. Every
// column here is a real number that is silently short — worse than the
// queue scan's version of this (rangerhq-llse), because a queue that comes
// back empty at least looks like nothing happened.
func TestScorecardNamesTheReposItCouldNotRead(t *testing.T) {
	good, bad := scorecardRepo(t, 2, false), scorecardRepo(t, 5, true)
	card, err := scorecardRig(t, good, bad)()
	if err != nil {
		t.Fatal(err)
	}
	// Named, not merely counted: an operator cannot fix a path the card
	// will not spell.
	if !strings.Contains(card, AbbrevHome(bad)) {
		t.Errorf("the unread repo %s must be named on the card:\n%s", bad, card)
	}
	if strings.Contains(card, "scorecard scan failed: "+AbbrevHome(good)) {
		t.Errorf("the repo that WAS read must not be reported as failed:\n%s", card)
	}
	// How much of the data the numbers came from, next to the numbers.
	if !strings.Contains(card, "scored 1 of 2 configured beads repo(s)") {
		t.Errorf("the card must say how many repos it scored:\n%s", card)
	}
	// And the good repo's data still lands — naming the failure is not an
	// excuse to drop the half that read.
	if !strings.Contains(card, "2 closed") {
		t.Errorf("the readable repo's 2 closed beads must still be scored:\n%s", card)
	}
}

// The silence half. A caveat that fires on a healthy card is a caveat the
// operator learns to skip, and this one has to survive being read.
func TestScorecardSaysNothingWhenEveryRepoReads(t *testing.T) {
	card, err := scorecardRig(t, scorecardRepo(t, 2, false), scorecardRepo(t, 3, false))()
	if err != nil {
		t.Fatal(err)
	}
	for _, no := range []string{"scorecard scan failed", "scored 2 of 2"} {
		if strings.Contains(card, no) {
			t.Errorf("a card that read every repo must not print %q:\n%s", no, card)
		}
	}
	if !strings.Contains(card, "5 closed") {
		t.Errorf("both repos must be summed (2+3 closed):\n%s", card)
	}
}

// repos == 0 already died; what it did not do was say WHICH repos, which
// is the whole of what an operator needs to act on the refusal.
func TestScorecardRefusalNamesEveryUnreadRepo(t *testing.T) {
	a, b := scorecardRepo(t, 1, true), scorecardRepo(t, 1, true)
	card, err := scorecardRig(t, a, b)()
	if err == nil {
		t.Fatalf("a card with no readable repo must refuse, got:\n%s", card)
	}
	for _, dir := range []string{a, b} {
		if !strings.Contains(err.Error(), AbbrevHome(dir)) {
			t.Errorf("the refusal must name %s: %v", dir, err)
		}
	}
	if card != "" {
		t.Errorf("a refusal must print no table at all:\n%s", card)
	}
}

// ranger-base-od6g. The reopen count was "known" if ANY scored repo had a
// readable census history, because addScore OR-ed the flag — so with two
// `beads:` entries and one of them unreadable, the column, the metric line
// and the trailer all printed a sum computed from half the histories as if
// it were the whole count. Same class of overstatement ranger-base-0tc
// fixed for the none case: a number that is short must not render as exact.
func TestPartialReopenHistoryScoresAsAFloor(t *testing.T) {
	closed := func(ids ...string) []BdIssue {
		var out []BdIssue
		for _, id := range ids {
			out = append(out, BdIssue{ID: id, Assignee: "dev", Status: "closed"})
		}
		return out
	}
	read := ScoreIssues("dev", closed("a", "b"), map[string]int{"a": 1})
	blind := ScoreIssues("dev", closed("c"), nil)

	if !read.ReopensKnown() || read.ReopensPartial() {
		t.Errorf("one repo whose history read is known, not a floor: %+v", read)
	}
	if blind.ReopensKnown() || blind.ReopensPartial() {
		t.Errorf("one repo with no history is neither known nor a floor: %+v", blind)
	}
	sum := addScore(read, blind)
	if sum.ReposScored != 2 || sum.ReposWithHistory != 1 {
		t.Fatalf("want history for 1 of 2 scored repos: %+v", sum)
	}
	if sum.ReopensKnown() {
		t.Errorf("a count missing one repo history is not known: %+v", sum)
	}
	if !sum.ReopensPartial() {
		t.Fatalf("a count read from only some repos is a floor: %+v", sum)
	}
	m := sum.Metric("closed-no-reopen")
	for _, want := range []string{"3 closed", "≥1 reopened", "1 of 2", "≤2"} {
		if !strings.Contains(m, want) {
			t.Errorf("the floor line must carry %q: %s", want, m)
		}
	}
	// The two renderings it must never borrow: the exact count of the
	// all-known case, and the no-evidence wording of the none case.
	if strings.Contains(m, "3 closed, 1 reopened") || strings.Contains(m, "unknown") {
		t.Errorf("a floor is neither exact nor no evidence: %s", m)
	}
	// Either end of the range is untouched: every repo blind stays the
	// unknown of ranger-base-0tc, every repo read stays an exact count.
	if m := addScore(blind, blind).Metric("closed-no-reopen"); !strings.Contains(m, "unknown") || strings.Contains(m, "≥") {
		t.Errorf("no history anywhere is still unknown, not a floor: %s", m)
	}
	if m := addScore(read, read).Metric("closed-no-reopen"); !strings.Contains(m, "4 closed, 2 reopened → 2") {
		t.Errorf("every history read is still an exact count: %s", m)
	}
}

// scorecardHistoryRepo is one beads repo that both answers `bd list --all`
// with n closed beads for "dev" and tracks a census whose git history holds
// exactly one reopen (h-0 closed→open→closed).
func scorecardHistoryRepo(t *testing.T, n int) string {
	t.Helper()
	dir := blRepo(t)
	var rows []string
	for i := 0; i < n; i++ {
		rows = append(rows, `{"id":"h-`+fmt.Sprint(i)+`","status":"closed","assignee":"dev"}`)
	}
	if err := os.WriteFile(filepath.Join(dir, "fake-list.json"), []byte("["+strings.Join(rows, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, st := range []string{"closed", "open", "closed"} {
		blCommit(t, dir, "sync", blLine("h-0", st))
	}
	return dir
}

// The command level: the column, the metric line and the trailer are three
// renderings of one fact, so pin all three across the three states — none
// read, some read, all read.
func TestScorecardSaysWhenOnlySomeReposHaveReopenHistory(t *testing.T) {
	row := func(card string) string {
		t.Helper()
		for _, ln := range strings.Split(card, "\n") {
			if strings.HasPrefix(ln, "dev ") {
				return ln
			}
		}
		t.Fatalf("no dev row on the card:\n%s", card)
		return ""
	}
	hist := scorecardHistoryRepo(t, 2)  // 2 closed, 1 reopened, history read
	blind := scorecardRepo(t, 3, false) // 3 closed, no git at all
	card, err := scorecardRig(t, hist, blind)()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(row(card), "≥1") {
		t.Errorf("the reopened column must wear the sign of a floor:\n%s", card)
	}
	for _, want := range []string{"5 closed", "≥1 reopened", "git history for 1 of 2 beads repos",
		"read for 1 of the 2 scored beads repo(s)"} {
		if !strings.Contains(card, want) {
			t.Errorf("the card must say %q:\n%s", want, card)
		}
	}
	// Neither of the other two states may be borrowed: not the "?" of no
	// history at all, and not the exact count of a full read.
	for _, no := range []string{"reopened: ?", "5 closed, 1 reopened"} {
		if strings.Contains(card, no) {
			t.Errorf("a partial read must not render as %q:\n%s", no, card)
		}
	}
	// The control: every scored repo history read, and the caveat is gone —
	// a floor sign on a healthy card is a sign the operator learns to skip.
	full, err := scorecardRig(t, hist, scorecardHistoryRepo(t, 3))()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(full, "5 closed, 2 reopened → 3") {
		t.Errorf("two read histories must sum to an exact count:\n%s", full)
	}
	for _, no := range []string{"≥", "reopened: ?", "scored beads repo(s)"} {
		if strings.Contains(full, no) {
			t.Errorf("a fully read card must not print %q:\n%s", no, full)
		}
	}
}

// The vocabulary, both directions (ranger-base-5fyg). isRejectedClose was
// strings.Contains, so every reject word matched inside a longer ordinary
// word: "dup" inside dedupes/deduplicated/duplicated, "invalid" inside
// invalidate/invalidation/invalidates. It is read by two callers — the
// scorecard's Rejected column and verify-after's exemption — and one of
// them pays for a wrong answer with a suppressed QA session.
//
// The false rows are the live close reasons the bead measured; the true
// rows are what the word list is FOR, and without them a regex that matched
// nothing at all would pass this test.
func TestIsRejectedCloseMatchesWordsNotSubstrings(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   bool
	}{
		// Real fixes, in this shop's own vocabulary.
		{"verify-after dedupes on the description marker bd commits with the bead", false},
		{"Fixed: deduplicated the render", false},
		{"Fixed: cache invalidation now keys on the sha", false},
		{"Fixed: a config write invalidates the cached probe", false},
		{"Fixed: removed the duplicated branch in Route()", false},
		{"the dedupe is keyed on the marker now", false},
		{"Fixed: the guard refuses", false},
		{"Closed", false},
		{"", false},
		// Rejections. The plurals matter: a real rejection reads "closed as
		// duplicates of x", and the substring test caught those too.
		{"duplicate of a-9", true},
		{"Duplicate", true},
		{"dup of a-9", true},
		{"closed as duplicates of a-9", true},
		{"invalid", true},
		{"Closed as wontfix", true},
		{"won't fix — the design changed", true},
		{"not a bug", true},
		// Whole words in a sentence that describes a fix. isRejectedClose
		// says true here and is not wrong to: free text cannot carry this
		// verdict alone, which is why verify-after reads the commit trail
		// beside it rather than trusting this answer on its own.
		{"Fixed: the retry no longer files a duplicate bead", true},
	} {
		if got := isRejectedClose(tc.reason); got != tc.want {
			t.Errorf("isRejectedClose(%q) = %v, want %v", tc.reason, got, tc.want)
		}
	}
}
