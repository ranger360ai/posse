package rhq

import (
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
	want := Score{Persona: "dev", Closed: 3, Reopened: 1, Open: 1, Held: 1, Blocked: 1, AgeAtClose: 4 * time.Hour, Filed: 3, Rejected: 1}
	if s != want {
		t.Errorf("got %+v\nwant %+v", s, want)
	}
	if m := s.Metric("closed-no-reopen"); !strings.Contains(m, "→ 2") {
		t.Errorf("closed-no-reopen: %s", m)
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
