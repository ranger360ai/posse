package posse

// ci-watch (ranger-base-x9e34, ciwatch.go) over two substrates: the fake bd
// the dispatch suite already uses for the FILING side, and a fake `gh` for
// the READING side, so the argv posse sends GitHub and the JSON it parses
// back are both driven rather than assumed.
//
// The two are deliberately not one test. The reading is a pure function of
// what gh printed; the filing is a state machine over a store, and its whole
// contract is a NEGATIVE — one bead per red episode, not one per push — which
// is only visible over repeated passes.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ─── the fake gh ─────────────────────────────────────────────────────────────

// fakeGh logs its argv to gh-calls.log and serves `run list --json` from
// fake-gh-runs.json, both in the test's fake dir (fakeDir, off argv[0]). A
// `fake-gh-fail` marker there is a gh that exits 1 with a word on stderr —
// no network, no auth, an unreachable GitHub — which must read as an
// abstention and never as "green".
func fakeGh(args []string) int {
	if f, _ := os.OpenFile(filepath.Join(fakeDir(), "gh-calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); f != nil {
		fmt.Fprintln(f, strings.Join(args, " "))
		f.Close()
	}
	if b, err := os.ReadFile(filepath.Join(fakeDir(), "fake-gh-fail")); err == nil {
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			msg = "HTTP 401: Bad credentials"
		}
		fmt.Fprint(os.Stderr, msg)
		return 1
	}
	// `run view <id> --log-failed` is a second call ciCauses makes, answered
	// from a per-id fixture (fake-gh-log-<id>.txt) so distinct runs can
	// answer distinctly. Missing is a run gh could not be asked about —
	// exit 1, the same "could not read" shape fake-gh-fail gives the list
	// call, and ciCauses must treat it the same way: skipped, not fatal.
	if len(args) >= 2 && args[0] == "run" && args[1] == "view" {
		id := ""
		if len(args) >= 3 {
			id = args[2]
		}
		b, err := os.ReadFile(filepath.Join(fakeDir(), "fake-gh-log-"+id+".txt"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "no log fixture for run %s", id)
			return 1
		}
		fmt.Print(string(b))
		return 0
	}
	// `api repos/<slug>/actions/runs/<id>/jobs?...` is the third call, the
	// job-level vet's (ciJobsSayQueue, ranger-base-rdi79), answered from a
	// per-id fixture (fake-gh-jobs-<id>.json) for the same reason the log
	// fixture is per-id. Missing is a page gh could not be asked for —
	// exit 1 — which must leave the run RED rather than set it aside, so
	// every reading test in this file that names a `failure` with no jobs
	// fixture goes on reading red.
	if len(args) >= 2 && args[0] == "api" {
		b, err := os.ReadFile(filepath.Join(fakeDir(), "fake-gh-jobs-"+ciRunID(args[1])+".json"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "no jobs fixture for %s", args[1])
			return 1
		}
		fmt.Print(string(b))
		return 0
	}
	if b, err := os.ReadFile(filepath.Join(fakeDir(), "fake-gh-runs.json")); err == nil {
		fmt.Print(string(b))
	} else {
		fmt.Print("[]")
	}
	return 0
}

// ciRun renders one row of what `gh run list --json` answers with.
func ciRunJSON(sha, status, conclusion string, at time.Time) string {
	return fmt.Sprintf(`{"headSha":%q,"status":%q,"conclusion":%q,"createdAt":%q,"url":"https://github.com/o/n/actions/runs/1"}`,
		sha, status, conclusion, at.UTC().Format(time.RFC3339))
}

// ciRunJSONID is ciRunJSON with its own run id, for the causes tests: every
// row ciRunJSON itself renders shares run id 1, which is fine for the
// streak arithmetic and wrong for anything that asks gh about one run by
// id.
func ciRunJSONID(sha, status, conclusion string, at time.Time, id string) string {
	return fmt.Sprintf(`{"headSha":%q,"status":%q,"conclusion":%q,"createdAt":%q,"url":"https://github.com/o/n/actions/runs/%s"}`,
		sha, status, conclusion, at.UTC().Format(time.RFC3339), id)
}

// writeGhJobs plants the fixture fakeGh serves for the job-level vet's
// `gh api .../runs/<id>/jobs` — the whole envelope, so a test can drive a
// PAGINATED page (total_count past the rows) as easily as a complete one.
func writeGhJobs(t *testing.T, id, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fakeDirOf(t), "fake-gh-jobs-"+id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ciJobJSON is one row of that page: name, conclusion, how many steps it
// executed, and the runner it held. Zero steps and an empty runner is a job
// that never started, which is the whole discriminator.
func ciJobJSON(name, conclusion string, steps int, runner string) string {
	st := make([]string, steps)
	for i := range st {
		st[i] = fmt.Sprintf(`{"number":%d,"conclusion":"success"}`, i+1)
	}
	return fmt.Sprintf(`{"name":%q,"status":"completed","conclusion":%q,"runner_name":%q,"steps":[%s]}`,
		name, conclusion, runner, strings.Join(st, ","))
}

// ciJobsJSON wraps rows in the envelope with an honest total_count.
func ciJobsJSON(jobs ...string) string {
	return fmt.Sprintf(`{"total_count":%d,"jobs":[%s]}`, len(jobs), strings.Join(jobs, ","))
}

// ciPhantomJobsJSON is the incident's own run (37362765576): five greens
// that ran eleven steps each, and one job that never got a runner.
func ciPhantomJobsJSON() string {
	return ciJobsJSON(
		ciJobJSON("test (macos-latest, 1)", "success", 11, "GitHub Actions 1"),
		ciJobJSON("test (macos-latest, 2)", "success", 11, "GitHub Actions 2"),
		ciJobJSON("test (macos-latest, 3)", "success", 11, "GitHub Actions 3"),
		ciJobJSON("test (ubuntu-latest, 1)", "success", 11, "GitHub Actions 4"),
		ciJobJSON("test (ubuntu-latest, 2)", "success", 11, "GitHub Actions 5"),
		ciJobJSON("test (ubuntu-latest, 3)", "cancelled", 0, ""),
	)
}

// ghRepo builds a checkout ReadCI will accept — a git repo with a github.com
// origin and the workflow file — and points the fake gh's answers at it.
func ghRepo(t *testing.T, workflow string, runs ...string) (dir, ghbin string) {
	t.Helper()
	dir = gitTempDir(t)
	run := func(args ...string) {
		t.Helper()
		if _, err := git(dir, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	run("init", "-b", "main")
	run("remote", "add", "origin", "https://github.com/ranger360ai/posse.git")
	// An identity and ONE commit with refs/remotes/origin/main on it,
	// because the freshness guard (ciFreshness) checks the page it was
	// handed against that ref and a checkout without one is an abstention
	// rather than a reading. A real beads repo has the ref; the first cut of
	// this fixture did not, and every reading test in this file went red
	// saying so. The run shas these tests name are still fakes the object
	// store has never heard of, which the guard reads as a page AHEAD of the
	// local view rather than behind it — the freshness pins below are the
	// ones that put real commits under real shas.
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	ciChain(t, dir, 1)
	if workflow != "" {
		if err := os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".github", "workflows", workflow), []byte("name: ci\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ghbin = fakeBinFor(t, "gh")
	body := "[" + strings.Join(runs, ",") + "]"
	if err := os.WriteFile(filepath.Join(fakeDirOf(t), "fake-gh-runs.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, ghbin
}

// ghJobsCalls counts the job-level vet's children. The verb is matched at
// the HEAD of the logged argv line and not as a bare substring: `api` is
// three letters that also sit inside the repo slug and inside the query
// string, and a substring count of it read 0 where the log held three.
func ghJobsCalls(t *testing.T) int {
	t.Helper()
	n := 0
	for _, l := range strings.Split(ghCalls(t), "\n") {
		if strings.HasPrefix(l, "api ") {
			n++
		}
	}
	return n
}

func ghCalls(t *testing.T) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(fakeDirOf(t), "gh-calls.log"))
	return string(b)
}

// ─── the reading ─────────────────────────────────────────────────────────────

// The verdict rule, stated as the table the measurement produced: over
// ci.yml's own 300-run history, skipping `cancelled` files 7 beads where
// counting it red files 16 and counting it green files 13. A run GitHub
// stopped is not a statement about main.
func TestCIVerdictSkipsEverythingThatIsNotAStatementAboutTheBranch(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		status, conclusion string
		red, ok            bool
	}{
		{"completed", "success", false, true},
		{"completed", "failure", true, true},
		{"completed", "timed_out", true, true},
		{"completed", "startup_failure", true, true},
		{"completed", "cancelled", false, false},
		{"completed", "skipped", false, false},
		{"completed", "neutral", false, false},
		{"completed", "action_required", false, false},
		{"in_progress", "", false, false},
		{"queued", "", false, false},
	} {
		red, ok := ciVerdict(c.status, c.conclusion)
		if red != c.red || ok != c.ok {
			t.Errorf("ciVerdict(%q,%q) = (%v,%v), want (%v,%v)", c.status, c.conclusion, red, ok, c.red, c.ok)
		}
	}
}

// ─── the job-level vet (ranger-base-rdi79) ───────────────────────────────────

// THE RULE, as a table over jobs pages. A `failure` is demoted to "not a
// verdict" only on POSITIVE evidence that nothing of it ran, and every other
// answer leaves the run red — the direction that matters, because a
// suppressed genuine red is this file's founding incident (191 reds over
// five days, nobody looked) and leaves no trace anywhere, while a false P1
// leaves a bead behind saying so.
func TestCIJobsSayQueueDemandsPositiveEvidence(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		page string
		want bool
	}{
		// The episode itself: five greens and one job that never got a
		// runner (run 37362765576, 2026-10-05).
		{"the incident's own run", ciPhantomJobsJSON(), true},
		// A real red, which is all 55 failures in the measured window.
		{"a job that failed", ciJobsJSON(
			ciJobJSON("test (ubuntu-latest, 3)", "failure", 11, "GitHub Actions 1"),
			ciJobJSON("test (macos-latest, 1)", "success", 11, "GitHub Actions 2"),
		), false},
		// A real red beside a phantom: something DID fail, so the run is a
		// verdict whatever else is on the page.
		{"a job that failed beside one that never started", ciJobsJSON(
			ciJobJSON("test (ubuntu-latest, 3)", "failure", 11, "GitHub Actions 1"),
			ciJobJSON("test (macos-latest, 1)", "cancelled", 0, ""),
		), false},
		{"a job that timed out", ciJobsJSON(
			ciJobJSON("test", "timed_out", 7, "GitHub Actions 1"),
			ciJobJSON("other", "cancelled", 0, ""),
		), false},
		{"a job that could not start", ciJobsJSON(
			ciJobJSON("test", "startup_failure", 0, ""),
			ciJobJSON("other", "cancelled", 0, ""),
		), false},
		// Cancelled, but it was RUNNING: stopped after doing work is not a
		// job that never started, and this rule does not claim to know what
		// it means.
		{"cancelled with steps behind it", ciJobsJSON(
			ciJobJSON("test (macos-latest, 1)", "success", 11, "GitHub Actions 1"),
			ciJobJSON("test (ubuntu-latest, 3)", "cancelled", 4, "GitHub Actions 2"),
		), false},
		{"cancelled holding a runner", ciJobsJSON(
			ciJobJSON("test (macos-latest, 1)", "success", 11, "GitHub Actions 1"),
			ciJobJSON("test (ubuntu-latest, 3)", "cancelled", 0, "GitHub Actions 2"),
		), false},
		// A skipped job is nobody's cause, and a phantom beside it still
		// demotes.
		{"skipped beside one that never started", ciJobsJSON(
			ciJobJSON("lint", "skipped", 0, ""),
			ciJobJSON("test (ubuntu-latest, 3)", "cancelled", 0, ""),
		), true},
		// The run says failure and no job accounts for it. That is a
		// disagreement with GitHub about its own run, and the reading that
		// does not suppress is the honest one.
		{"a failure no job accounts for", ciJobsJSON(
			ciJobJSON("test (macos-latest, 1)", "success", 11, "GitHub Actions 1"),
		), false},
		{"an empty page", `{"total_count":0,"jobs":[]}`, false},
		{"junk", `{}`, false},
		// Paginated: the jobs this did not get could each be the red one.
		{"a page gh paginated", `{"total_count":7,"jobs":[` +
			ciJobJSON("test (ubuntu-latest, 3)", "cancelled", 0, "") + `]}`, false},
		// Still running, so nothing about it is concluded yet.
		{"a job that has not finished", `{"total_count":2,"jobs":[` +
			ciJobJSON("test (macos-latest, 1)", "success", 11, "GitHub Actions 1") + `,` +
			`{"name":"test (ubuntu-latest, 3)","status":"in_progress","conclusion":null,"runner_name":"","steps":[]}]}`, false},
		{"a conclusion nobody here has seen", ciJobsJSON(
			ciJobJSON("test", "action_required", 0, ""),
		), false},
	} {
		var page ciJobsPage
		if err := json.Unmarshal([]byte(c.page), &page); err != nil {
			t.Fatalf("%s: fixture is not JSON: %v", c.name, err)
		}
		why, got := ciJobsSayQueue(page)
		if got != c.want {
			t.Errorf("%s: ciJobsSayQueue = %v (%q), want %v", c.name, got, why, c.want)
		}
		if got && why == "" {
			t.Errorf("%s: set aside with no evidence stated", c.name)
		}
		if !got && why != "" {
			t.Errorf("%s: not set aside but said %q", c.name, why)
		}
	}
}

// THE EPISODE, end to end through the shipped argv: a page topped by the
// run that filed the false P1 reads GREEN, because the newest run with a
// verdict is the success under it.
func TestReadCISetsAsideAFailedRunWhoseRedJobNeverRan(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 5, 19, 21, 0, 0, time.UTC)
	dir, bin := ghRepo(t, "ci.yml",
		ciRunJSONID("bab60e51", "completed", "failure", base, "9001"),
		ciRunJSONID("f5ccb1a8", "completed", "success", base.Add(-time.Hour), "9000"),
	)
	writeGhJobs(t, "9001", ciPhantomJobsJSON())
	st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	if !st.Known() {
		t.Fatalf("not read: %s", st.Why)
	}
	if st.Red {
		t.Errorf("red off a run whose only non-success job never started — latest %q", st.Latest.Short())
	}
	if st.Latest.Short() != "f5ccb1a8" {
		t.Errorf("latest = %q, want the newest run that actually carries a verdict", st.Latest.Short())
	}
	if len(st.QueueOnly) != 1 || st.QueueOnly[0].Run.Short() != "bab60e51" {
		t.Fatalf("QueueOnly = %+v, want the one run it set aside", st.QueueOnly)
	}
	if why := st.QueueOnly[0].Why; !strings.Contains(why, "never started") || !strings.Contains(why, "test (ubuntu-latest, 3)") {
		t.Errorf("the set-aside reason does not name the job or the evidence: %q", why)
	}
	// A run it set aside is not a verdict, so it is not in the streak it
	// was never part of.
	if st.Streak != 1 {
		t.Errorf("streak = %d, want 1", st.Streak)
	}
	// And the argv is the documented one.
	if got := ghCalls(t); !strings.Contains(got, "api repos/ranger360ai/posse/actions/runs/9001/jobs?per_page=100") {
		t.Errorf("gh argv did not ask the documented jobs query:\n%s", got)
	}
	if got := ghCalls(t); strings.Contains(got, "filter=all") {
		t.Errorf("asked for every attempt's jobs, not the latest run attempt's:\n%s", got)
	}
}

// And a run that really did fail is untouched — which is all 55 failures in
// the measured 300-run window, so the red half of this gate reads exactly
// as it did before the vet existed.
func TestReadCIKeepsAFailedRunWhoseJobActuallyFailed(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 5, 19, 21, 0, 0, time.UTC)
	dir, bin := ghRepo(t, "ci.yml",
		ciRunJSONID("bab60e51", "completed", "failure", base, "9001"),
		ciRunJSONID("f5ccb1a8", "completed", "success", base.Add(-time.Hour), "9000"),
	)
	writeGhJobs(t, "9001", ciJobsJSON(
		ciJobJSON("test (ubuntu-latest, 3)", "failure", 11, "GitHub Actions 1"),
		ciJobJSON("test (macos-latest, 1)", "success", 11, "GitHub Actions 2"),
	))
	st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	if !st.Known() {
		t.Fatalf("not read: %s", st.Why)
	}
	if !st.Red || st.Latest.Short() != "bab60e51" {
		t.Errorf("red=%v latest=%q, want the failure to stand", st.Red, st.Latest.Short())
	}
	if len(st.QueueOnly) != 0 {
		t.Errorf("set aside a real red: %+v", st.QueueOnly)
	}
}

// A jobs page this could not read leaves the run RED. The vet only ever
// demotes on evidence, so no network, no auth and no such run all read as
// the reading this file took before it existed.
func TestReadCIKeepsTheRedWhenTheJobsPageCannotBeRead(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 5, 19, 21, 0, 0, time.UTC)
	for _, c := range []struct{ name, jobs string }{
		{"no answer at all", ""}, // no fixture: the fake gh exits 1
		{"not the JSON asked for", "<html>404</html>"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir, bin := ghRepo(t, "ci.yml",
				ciRunJSONID("bab60e51", "completed", "failure", base, "9001"),
				ciRunJSONID("f5ccb1a8", "completed", "success", base.Add(-time.Hour), "9000"),
			)
			if c.jobs != "" {
				writeGhJobs(t, "9001", c.jobs)
			}
			st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
			if !st.Known() {
				t.Fatalf("not read: %s", st.Why)
			}
			if !st.Red || len(st.QueueOnly) != 0 {
				t.Errorf("red=%v queueOnly=%+v, want the failure to stand", st.Red, st.QueueOnly)
			}
		})
	}
}

// THE COST, which is the reason the vet sits at the head of the page: a
// green pass forks no second child, and neither does a run GitHub stopped —
// the extra call is paid only by a reading that is about to FILE.
func TestReadCIAsksForJobsOnlyWhenAFailureIsAboutToBeTheVerdict(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 5, 19, 21, 0, 0, time.UTC)
	for _, c := range []struct{ name, conclusion string }{
		{"a green pass", "success"},
		{"a run GitHub stopped", "cancelled"},
		{"a run that timed out", "timed_out"},
		{"a run that could not start", "startup_failure"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir, bin := ghRepo(t, "ci.yml",
				ciRunJSONID("bab60e51", "completed", c.conclusion, base, "9001"),
				ciRunJSONID("f5ccb1a8", "completed", "success", base.Add(-time.Hour), "9000"),
			)
			ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
			if n := ghJobsCalls(t); n != 0 {
				t.Errorf("%d jobs calls over %s:\n%s", n, c.name, ghCalls(t))
			}
		})
	}
}

// AND IT IS BOUNDED. A page topped by phantom after phantom is a queue
// broken for everybody, not a fact about this branch, and it must not put
// one gh child per run of the window on a dispatch pass. Past ciJobsVetCap
// the run stands as gh reported it — and the cap counts the CALLS, which is
// what costs, so a page of nothing but phantoms forks exactly as many
// children as a page whose first unvetted run stands.
func TestReadCIVetsAtMostTheCapAndOnlyAtTheHead(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 5, 19, 21, 0, 0, time.UTC)
	var rows []string
	for i := 0; i <= ciJobsVetCap; i++ {
		rows = append(rows, ciRunJSONID(fmt.Sprintf("sha%05d", i), "completed", "failure",
			base.Add(-time.Duration(i)*time.Hour), strconv.Itoa(9000+i)))
	}
	rows = append(rows, ciRunJSONID("green000", "completed", "success", base.Add(-100*time.Hour), "8999"))
	dir, bin := ghRepo(t, "ci.yml", rows...)
	for i := 0; i <= ciJobsVetCap; i++ {
		writeGhJobs(t, strconv.Itoa(9000+i), ciPhantomJobsJSON())
	}
	st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	if !st.Known() {
		t.Fatalf("not read: %s", st.Why)
	}
	if n := ghJobsCalls(t); n != ciJobsVetCap {
		t.Errorf("%d jobs calls, want ciJobsVetCap=%d:\n%s", n, ciJobsVetCap, ghCalls(t))
	}
	if len(st.QueueOnly) != ciJobsVetCap {
		t.Errorf("set aside %d, want ciJobsVetCap=%d", len(st.QueueOnly), ciJobsVetCap)
	}
	if !st.Red {
		t.Errorf("green past the cap: the run that was never vetted must stand as gh reported it")
	}
	if want := fmt.Sprintf("sha%05d", ciJobsVetCap); st.Latest.Short() != want {
		t.Errorf("latest = %q, want %q — the first run past the cap", st.Latest.Short(), want)
	}
}

// TWO PHANTOMS IN A ROW, over the payloads GitHub actually served
// (ranger-base-hxcqe). The tests above drive one demotion and the cap's
// boundary, both from synthetic pages. This is the walk in between, and
// until 2026-10-05 it had never existed: a page topped by two consecutive
// queue-only runs with a real verdict beneath them, which is what an Actions
// OUTAGE produces rather than a one-in-300 phantom.
//
// MEASURED 2026-10-05 from /actions/runs/<id>/jobs?per_page=100 during
// GitHub's "Incident with Actions" (component major_outage, opened
// 19:11:58Z). Both runs carry a `failure` conclusion that no job of theirs
// accounts for, and the starvation is not the same shape twice — which is
// the point of using the real rows: 37372180663 lost every ubuntu job and
// kept both macos ones, 37367869180 kept ubuntu 2 and lost ubuntu 1. A
// fixture that repeated one page twice would not have told them apart.
//
//   - 37372180663 (64f18288, 20:50:06Z): macos 1 and macos 2 success with
//     11 steps; macos 3, ubuntu 1, ubuntu 2, ubuntu 3 all cancelled at zero
//     steps, cut together at 21:05:09Z after 15m03s queued.
//   - 37367869180 (eebe9737, 20:08:21Z): macos 1, macos 2, ubuntu 2 success
//     with 11 steps; macos 3, ubuntu 1, ubuntu 3 cancelled at zero steps,
//     15m03s.
//   - 37362765576 (bab60e51, 19:21:00Z) is the verdict: ranger-base-94grm's
//     own run, success under the default filter once its one starved job was
//     re-run green.
//
// What this pins that the single-demotion test cannot: the loop ADVANCES.
// It must take the second run from the top as the next candidate, ask its
// jobs page too, and land on the third — two gh children, two set-asides in
// page order, and a gate that reads GREEN over two red runs in a row.
func TestReadCIWalksPastTwoConsecutivePhantomsToTheVerdict(t *testing.T) {
	t.Parallel()
	phantom := func(cancelled ...string) map[string]bool {
		m := map[string]bool{}
		for _, n := range cancelled {
			m[n] = true
		}
		return m
	}
	page := func(cancelled map[string]bool, runners map[string]string) string {
		var rows []string
		for _, n := range []string{
			"test (macos-latest, 1)", "test (macos-latest, 2)", "test (macos-latest, 3)",
			"test (ubuntu-latest, 1)", "test (ubuntu-latest, 2)", "test (ubuntu-latest, 3)",
		} {
			if cancelled[n] {
				rows = append(rows, ciJobJSON(n, "cancelled", 0, ""))
				continue
			}
			rows = append(rows, ciJobJSON(n, "success", 11, runners[n]))
		}
		return ciJobsJSON(rows...)
	}

	tip := time.Date(2026, 10, 5, 20, 50, 6, 0, time.UTC)
	dir, bin := ghRepo(t, "ci.yml",
		ciRunJSONID("64f18288", "completed", "failure", tip, "37372180663"),
		ciRunJSONID("eebe9737", "completed", "failure", tip.Add(-41*time.Minute-45*time.Second), "37367869180"),
		ciRunJSONID("bab60e51", "completed", "success", tip.Add(-89*time.Minute-6*time.Second), "37362765576"),
	)
	writeGhJobs(t, "37372180663", page(
		phantom("test (macos-latest, 3)", "test (ubuntu-latest, 1)", "test (ubuntu-latest, 2)", "test (ubuntu-latest, 3)"),
		map[string]string{
			"test (macos-latest, 1)": "GitHub Actions 1000002509",
			"test (macos-latest, 2)": "GitHub Actions 1000002516",
		}))
	writeGhJobs(t, "37367869180", page(
		phantom("test (macos-latest, 3)", "test (ubuntu-latest, 1)", "test (ubuntu-latest, 3)"),
		map[string]string{
			"test (macos-latest, 1)":  "GitHub Actions 1000002497",
			"test (macos-latest, 2)":  "GitHub Actions 1000002496",
			"test (ubuntu-latest, 2)": "GitHub Actions 1000002501",
		}))

	st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	if !st.Known() {
		t.Fatalf("not read: %s", st.Why)
	}
	// The whole point: two red runs at the head of the page, and the gate is
	// not red. A P1 filed here would name a commit nothing was wrong with,
	// twice over.
	if st.Red {
		t.Errorf("red over two runs whose only non-success jobs never started — latest %q", st.Latest.Short())
	}
	if st.Latest.Short() != "bab60e51" {
		t.Errorf("latest = %q, want bab60e51 — the newest run that carries a verdict", st.Latest.Short())
	}
	if len(st.QueueOnly) != 2 {
		t.Fatalf("set aside %d, want 2: the loop did not advance past the first phantom (%+v)", len(st.QueueOnly), st.QueueOnly)
	}
	// In PAGE order, newest first, because the bead's SET ASIDE block prints
	// them in the order it found them and a reader matches them against
	// `gh run list`.
	if got := []string{st.QueueOnly[0].Run.Short(), st.QueueOnly[1].Run.Short()}; got[0] != "64f18288" || got[1] != "eebe9737" {
		t.Errorf("set aside %v, want [64f18288 eebe9737] in page order", got)
	}
	// Each reason is built from its OWN page, so they must not be the same
	// sentence: the tip lost ubuntu 2 and eebe9737 kept it.
	tipWhy, prevWhy := st.QueueOnly[0].Why, st.QueueOnly[1].Why
	if !strings.Contains(tipWhy, "test (ubuntu-latest, 2)") {
		t.Errorf("the tip's reason does not name ubuntu 2, which starved in it: %q", tipWhy)
	}
	if strings.Contains(prevWhy, "test (ubuntu-latest, 2)") {
		t.Errorf("eebe9737's reason names ubuntu 2, which SUCCEEDED in it — the reasons are not read per-run: %q", prevWhy)
	}
	for i, why := range []string{tipWhy, prevWhy} {
		if !strings.Contains(why, "never started") {
			t.Errorf("set aside[%d] states no evidence: %q", i, why)
		}
	}
	// One child per demotion and not one per run of the window, and the last
	// run is never asked about because it is not a failure.
	if n := ghJobsCalls(t); n != 2 {
		t.Errorf("%d jobs calls, want 2:\n%s", n, ghCalls(t))
	}
	if got := ghCalls(t); strings.Contains(got, "37362765576/jobs") {
		t.Errorf("asked the jobs endpoint about a run that already carried a verdict:\n%s", got)
	}
	// A run set aside was never a verdict, so neither phantom is in the
	// streak — and a green gate's streak is the green one.
	if st.Streak != 1 {
		t.Errorf("streak = %d, want 1: a set-aside run counted toward it", st.Streak)
	}
}

// A window holding NOTHING but runs that never ran is the could-not-READ
// abstention, not a green pass: there is no verdict in it either way, and
// the next completed run clears it.
func TestReadCIAbstainsWhenEveryRunItCanSeeNeverRan(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 5, 19, 21, 0, 0, time.UTC)
	dir, bin := ghRepo(t, "ci.yml", ciRunJSONID("bab60e51", "completed", "failure", base, "9001"))
	writeGhJobs(t, "9001", ciPhantomJobsJSON())
	st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	if st.Known() {
		t.Fatalf("read a verdict off a window with none in it: red=%v latest=%q", st.Red, st.Latest.Short())
	}
	if !strings.Contains(st.Why, "set aside") || !strings.Contains(st.Why, "never started") {
		t.Errorf("why = %q, want it to name what it set aside and why", st.Why)
	}
	if st.NoGate {
		t.Error("NoGate: this repo HAS a gate, and a pass that cannot read one says so")
	}
}

// A bead filed while a phantom sits ABOVE the verdict names it, because the
// bead's own Reproduce block sends a seat to a `gh run list` topped by a red
// run the bead is deliberately not about.
func TestCIDescriptionNamesTheRunsItSetAside(t *testing.T) {
	t.Parallel()
	st := redState(1)
	if d := st.Description(); strings.Contains(d, "SET ASIDE") {
		t.Fatalf("an ordinary bead carries the set-aside block:\n%s", d)
	}
	st.QueueOnly = []CIQueueOnly{{
		Run: CIRun{Sha: "bab60e51c0", URL: "https://x/9001", Conclusion: "failure",
			Created: time.Date(2026, 10, 5, 19, 21, 0, 0, time.UTC)},
		Why: "the run says failure but no job failed: test (ubuntu-latest, 3) never started",
	}}
	d := st.Description()
	for _, want := range []string{"SET ASIDE (1)", "bab60e51", "https://x/9001", "never started"} {
		if !strings.Contains(d, want) {
			t.Errorf("description missing %q:\n%s", want, d)
		}
	}
}

// A cancelled run between two failures does not break the streak and does
// not become its own verdict: the incident's own history has 20 of them, and
// counting them either way more than doubles the beads this files.
func TestReadCICountsTheStreakThroughCancelledRuns(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 8, 30, 2, 0, 0, 0, time.UTC)
	dir, bin := ghRepo(t, "ci.yml",
		ciRunJSON("dddddddddd", "completed", "failure", base.Add(3*time.Hour)),
		ciRunJSON("cccccccccc", "completed", "cancelled", base.Add(2*time.Hour)),
		ciRunJSON("bbbbbbbbbb", "completed", "failure", base.Add(1*time.Hour)),
		ciRunJSON("aaaaaaaaaa", "completed", "success", base),
	)
	st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	if !st.Known() {
		t.Fatalf("not read: %s", st.Why)
	}
	if !st.Red {
		t.Fatal("want red")
	}
	if st.Streak != 2 {
		t.Errorf("streak = %d, want 2 (the cancelled run is skipped, not counted and not a break)", st.Streak)
	}
	if st.Latest.Short() != "dddddddd" {
		t.Errorf("latest = %q, want the newest failure", st.Latest.Short())
	}
	if st.Since.Short() != "bbbbbbbb" {
		t.Errorf("since = %q, want the oldest run of the streak", st.Since.Short())
	}
	if st.Slug != "ranger360ai/posse" || st.Branch != "main" {
		t.Errorf("slug/branch = %q/%q", st.Slug, st.Branch)
	}
}

// PriorRedRuns is the red streak a green reading is about to clear — free
// of an extra gh call, since it is the same window ReadCI already scanned
// for Streak and Since (ranger-base-d6zyu finding 3). Newest-first, and it
// stops at the streak boundary rather than walking every red the window
// holds.
func TestReadCIRecordsThePriorRedStreakOnlyWhenItClears(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	dir, bin := ghRepo(t, "ci.yml",
		ciRunJSON("newestgree", "completed", "success", base.Add(4*time.Hour)), // Latest
		ciRunJSON("secondgree", "completed", "success", base.Add(3*time.Hour)),
		ciRunJSON("redrun0002", "completed", "failure", base.Add(2*time.Hour)), // PriorRedRuns[0]
		ciRunJSON("redrun0001", "completed", "failure", base.Add(1*time.Hour)), // PriorRedRuns[1]
		ciRunJSON("redrun0000", "completed", "failure", base),                  // PriorRedRuns[2]
	)
	st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	if !st.Known() || st.Red {
		t.Fatalf("st = %+v, want a known green reading", st)
	}
	if st.Streak != 2 {
		t.Fatalf("streak = %d, want 2", st.Streak)
	}
	if got := len(st.PriorRedRuns); got != 3 {
		t.Fatalf("PriorRedRuns has %d runs, want 3: %+v", got, st.PriorRedRuns)
	}
	wantOrder := []string{"redrun0002", "redrun0001", "redrun0000"}
	for i, want := range wantOrder {
		if got := st.PriorRedRuns[i].Sha; got != want {
			t.Errorf("PriorRedRuns[%d] = %q, want %q (newest-first)", i, got, want)
		}
	}

	// The control: a RED reading has no "prior" streak to report — the one
	// it is IN is Since/Streak's job, and PriorRedRuns must stay empty so a
	// still-red pass never asks ciCauses about anything.
	dir2, bin2 := ghRepo(t, "ci.yml",
		ciRunJSON("stillred01", "completed", "failure", base.Add(2*time.Hour)),
		ciRunJSON("stillred00", "completed", "failure", base.Add(1*time.Hour)),
		ciRunJSON("oldgreen00", "completed", "success", base),
	)
	st2 := ReadCI(CIQuery{Dir: dir2, Workflow: "ci.yml", GhBin: bin2})
	if !st2.Red {
		t.Fatalf("st2 = %+v, want red", st2)
	}
	if len(st2.PriorRedRuns) != 0 {
		t.Errorf("a red reading has PriorRedRuns %+v, want none", st2.PriorRedRuns)
	}
}

// The newest run is the verdict whatever order gh answered in — gh documents
// no order and the whole reading is "the newest".
func TestReadCISortsRatherThanTrustingGhsOrder(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 8, 30, 2, 0, 0, 0, time.UTC)
	dir, bin := ghRepo(t, "ci.yml",
		ciRunJSON("old11111", "completed", "failure", base),
		ciRunJSON("new22222", "completed", "success", base.Add(time.Hour)),
	)
	st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	if !st.Known() {
		t.Fatalf("not read: %s", st.Why)
	}
	if st.Red {
		t.Errorf("red, but the newest run is the success: latest=%q", st.Latest.Short())
	}
}

// The argv is the query the bead's own "Reproduce:" block names. --repo is
// explicit and not left to gh's cwd resolution: a dispatch pass runs from
// wherever the launcher was started.
func TestReadCIAsksTheDocumentedQuery(t *testing.T) {
	t.Parallel()
	dir, bin := ghRepo(t, "ci.yml", ciRunJSON("a1b2c3d4", "completed", "success", time.Now()))
	ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	got := ghCalls(t)
	for _, want := range []string{
		"run list", "--repo ranger360ai/posse", "--workflow ci.yml", "--branch main",
		"--limit 100", "--json conclusion,status,createdAt,headSha,url",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("gh argv %q missing %q", strings.TrimSpace(got), want)
		}
	}
}

// The abstentions, each with the thing that is missing named in the reason —
// and none of them reading as green.
func TestReadCIAbstainsRatherThanGuessing(t *testing.T) {
	t.Parallel()
	t.Run("no workflow file", func(t *testing.T) {
		t.Parallel()
		dir, bin := ghRepo(t, "")
		st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
		if st.Known() || !strings.Contains(st.Why, ".github/workflows/ci.yml") {
			t.Errorf("why = %q", st.Why)
		}
		if !st.NoGate {
			t.Error("a repo with no such workflow is NoGate: there is nothing to read, so nothing to say")
		}
		if c := ghCalls(t); c != "" {
			t.Errorf("forked gh for a repo with no such workflow: %q", c)
		}
	})
	t.Run("workflow off", func(t *testing.T) {
		t.Parallel()
		st := ReadCI(CIQuery{Dir: t.TempDir(), Workflow: ""})
		if st.Known() || !strings.Contains(st.Why, "ci_workflow") {
			t.Errorf("why = %q", st.Why)
		}
		if !st.NoGate {
			t.Error("configured off is NoGate")
		}
	})
	t.Run("not a github remote", func(t *testing.T) {
		t.Parallel()
		dir, bin := ghRepo(t, "ci.yml")
		if _, err := git(dir, "remote", "set-url", "origin", "git@gitlab.com:o/n.git"); err != nil {
			t.Fatal(err)
		}
		st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
		if st.Known() || !strings.Contains(st.Why, "not on github.com") {
			t.Errorf("why = %q", st.Why)
		}
		if !st.NoGate {
			t.Error("a non-github origin is NoGate")
		}
	})
	t.Run("gh fails", func(t *testing.T) {
		t.Parallel()
		dir, bin := ghRepo(t, "ci.yml")
		os.WriteFile(filepath.Join(fakeDirOf(t), "fake-gh-fail"), []byte("HTTP 401: Bad credentials"), 0o644)
		st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
		if st.Known() {
			t.Fatal("a gh that exited 1 must not produce a verdict")
		}
		if st.Red {
			t.Fatal("an abstention must not carry Red")
		}
		if !strings.Contains(st.Why, "Bad credentials") {
			t.Errorf("why = %q, want gh's own words", st.Why)
		}
		if st.NoGate {
			t.Error("a gh that could not answer is NOT NoGate — there is a gate and it went unread, which must be said")
		}
	})
	t.Run("no verdict-bearing run", func(t *testing.T) {
		t.Parallel()
		dir, bin := ghRepo(t, "ci.yml", ciRunJSON("c1", "completed", "cancelled", time.Now()))
		st := ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
		if st.Known() || !strings.Contains(st.Why, "carries a verdict") {
			t.Errorf("why = %q", st.Why)
		}
		if st.NoGate {
			t.Error("a gate with no verdict-bearing run is NOT NoGate — the workflow is there and said nothing usable")
		}
	})
}

// The branch is DERIVED, not assumed: a repo whose default branch is not
// `main` is read on its own branch rather than answered over an empty one.
func TestCIBranchFollowsOriginHEADAndConfig(t *testing.T) {
	t.Parallel()
	dir := gitTempDir(t)
	if _, err := git(dir, "init", "-b", "trunk"); err != nil {
		t.Fatal(err)
	}
	if got := ciBranch(dir, ""); got != "main" {
		t.Errorf("with no origin/HEAD: %q, want the main fallback", got)
	}
	if _, err := git(dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk"); err != nil {
		t.Fatal(err)
	}
	if got := ciBranch(dir, ""); got != "trunk" {
		t.Errorf("origin/HEAD says trunk, got %q", got)
	}
	if got := ciBranch(dir, "release"); got != "release" {
		t.Errorf("config outranks origin/HEAD: got %q", got)
	}
}

func TestGithubSlugSpellings(t *testing.T) {
	t.Parallel()
	for remote, want := range map[string]string{
		"https://github.com/ranger360ai/posse.git": "ranger360ai/posse",
		"https://github.com/ranger360ai/posse":     "ranger360ai/posse",
		"git@github.com:ranger360ai/posse.git":     "ranger360ai/posse",
		"ssh://git@github.com/ranger360ai/posse":   "ranger360ai/posse",
		"https://github.com/ranger360ai/posse/":    "ranger360ai/posse",
	} {
		got, ok := githubSlug(remote)
		if !ok || got != want {
			t.Errorf("githubSlug(%q) = %q,%v; want %q", remote, got, ok, want)
		}
	}
	for _, remote := range []string{"git@gitlab.com:o/n.git", "https://github.example.com/o/n", "", "/local/path"} {
		if got, ok := githubSlug(remote); ok {
			t.Errorf("githubSlug(%q) = %q, want no match", remote, got)
		}
	}
}

// ─── the causes (ranger-base-d6zyu finding 3) ───────────────────────────────

// writeGhLog plants the fixture fakeGh serves for `run view <id> --log-failed`.
func writeGhLog(t *testing.T, id, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fakeDirOf(t), "fake-gh-log-"+id+".txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ciCauses reads one `--- FAIL: Name` per run regardless of what gh prefixed
// the line with, dedupes repeats WITHIN a run (one run failing a test twice
// is still one run's worth of evidence), and counts ACROSS runs.
func TestCICausesAggregatesDedupesAndSortsByCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := fakeBinFor(t, "gh")
	writeGhLog(t, "101", "ubuntu-latest\tgo test\t--- FAIL: TestAlpha (0.01s)\nubuntu-latest\tgo test\t--- FAIL: TestAlpha (0.01s)\n")
	writeGhLog(t, "102", "macos-latest\tgo test\t--- FAIL: TestAlpha (0.02s)\nmacos-latest\tgo test\t--- FAIL: TestBeta (0.03s)\n")
	writeGhLog(t, "103", "ubuntu-latest\tgo test\t--- FAIL: TestBeta (0.01s)\n")

	runs := []CIRun{
		{Sha: "c", URL: "https://github.com/o/n/actions/runs/103"},
		{Sha: "b", URL: "https://github.com/o/n/actions/runs/102"},
		{Sha: "a", URL: "https://github.com/o/n/actions/runs/101"},
	}
	got := ciCauses(dir, bin, "o/n", runs)
	want := []string{"TestAlpha (2 runs)", "TestBeta (2 runs)"}
	if len(got) != len(want) {
		t.Fatalf("ciCauses = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ciCauses[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
	if got := ghCalls(t); strings.Count(got, "run view") != 3 {
		t.Errorf("gh calls = %q, want exactly 3 `run view`s, one per run", got)
	}
}

// A run gh could not be asked about (missing fixture here — a run too old
// for GitHub to still hold logs for, in production) is skipped rather than
// failing the whole reading: best-effort, the same rule NoGate and Why
// already follow in this file.
func TestCICausesSkipsARunItCouldNotRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := fakeBinFor(t, "gh")
	writeGhLog(t, "201", "--- FAIL: TestReachable (0.01s)\n")
	// 202 has no fixture: fakeGh exits 1 for it.
	runs := []CIRun{
		{Sha: "b", URL: "https://github.com/o/n/actions/runs/202"},
		{Sha: "a", URL: "https://github.com/o/n/actions/runs/201"},
	}
	got := ciCauses(dir, bin, "o/n", runs)
	if len(got) != 1 || got[0] != "TestReachable (1 run)" {
		t.Fatalf("ciCauses = %v, want exactly the readable run's cause", got)
	}
}

// No FAIL line anywhere is an abstention, not an empty report — nil, so
// ciClear's caller can tell "nothing to say" from "said nothing".
func TestCICausesReturnsNilWhenNothingFailed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := fakeBinFor(t, "gh")
	writeGhLog(t, "301", "ok  \tinternal/posse\t0.5s\n")
	got := ciCauses(dir, bin, "o/n", []CIRun{{Sha: "a", URL: "https://github.com/o/n/actions/runs/301"}})
	if got != nil {
		t.Errorf("ciCauses = %v, want nil", got)
	}
}

// ciCauseScanCap bounds the cost: a streak longer than the cap is scanned
// only at its newest ciCauseScanCap runs, and the report says so rather
// than silently under-reporting as if that were the whole streak.
func TestCICausesCapsHowManyRunsItAsksGhAbout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := fakeBinFor(t, "gh")
	n := ciCauseScanCap + 3
	runs := make([]CIRun, n)
	for i := 0; i < n; i++ {
		id := strconv.Itoa(400 + i)
		writeGhLog(t, id, fmt.Sprintf("--- FAIL: TestRun%d (0.01s)\n", i))
		runs[i] = CIRun{Sha: strconv.Itoa(i), URL: "https://github.com/o/n/actions/runs/" + id}
	}
	got := ciCauses(dir, bin, "o/n", runs)
	if calls := strings.Count(ghCalls(t), "run view"); calls != ciCauseScanCap {
		t.Errorf("gh calls = %d, want exactly the cap (%d)", calls, ciCauseScanCap)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, fmt.Sprintf("newest %d of %d", ciCauseScanCap, n)) {
		t.Errorf("ciCauses = %v, want a line naming the cap against the true total", got)
	}
	// Only the FIRST (newest) ciCauseScanCap runs' tests appear — the last
	// three (TestRun%d for i >= cap) are past the cap and must be silent.
	for i := ciCauseScanCap; i < n; i++ {
		name := fmt.Sprintf("TestRun%d", i)
		if strings.Contains(joined, name) {
			t.Errorf("ciCauses named %s, which is past the cap: %v", name, got)
		}
	}
}

// runIDFromURL — sorry, ciRunID: the id `gh run view` needs, off the URL
// `gh run list` gave it.
func TestCIRunIDPullsTheNumericIDOffTheURL(t *testing.T) {
	t.Parallel()
	if got := ciRunID("https://github.com/o/n/actions/runs/34067366022"); got != "34067366022" {
		t.Errorf("ciRunID = %q, want the numeric id", got)
	}
	if got := ciRunID("not a url"); got != "" {
		t.Errorf("ciRunID(%q) = %q, want empty", "not a url", got)
	}
}

// ─── the freshness of the reading ────────────────────────────────────────────

// ciChain writes n empty commits into dir's object store, points
// refs/remotes/origin/main at the newest, and returns every sha OLDEST
// FIRST.
//
// commit-tree and update-ref rather than `git commit --allow-empty`: every
// crew PID denies an unqualified commit in every tree (adrcensusrepo_qa_test
// builds its own fixture the same way), and this needs no index, no working
// tree and no hook either way.
func ciChain(t *testing.T, dir string, n int) []string {
	t.Helper()
	tree, err := git(dir, "hash-object", "-t", "tree", "-w", os.DevNull)
	if err != nil {
		t.Fatalf("hash-object: %v %s", err, tree)
	}
	tree = strings.TrimSpace(tree)
	shas := make([]string, 0, n)
	parent := ""
	for i := 0; i < n; i++ {
		args := []string{"commit-tree", tree, "-m", "c" + strconv.Itoa(i)}
		if parent != "" {
			args = append(args, "-p", parent)
		}
		sha, cerr := git(dir, args...)
		if cerr != nil {
			t.Fatalf("commit-tree %d: %v %s", i, cerr, sha)
		}
		parent = strings.TrimSpace(sha)
		shas = append(shas, parent)
	}
	if out, uerr := git(dir, "update-ref", "refs/remotes/origin/main", parent); uerr != nil {
		t.Fatalf("update-ref: %v %s", uerr, out)
	}
	return shas
}

// ciMergeChain builds the smallest chain that can tell `rev-list --count`
// from `rev-list --count --first-parent`, and hands back one sha of each
// kind. side is how many commits the merge brings in from off the
// first-parent chain.
//
//	b0 ─ b1 ──────── M        refs/remotes/origin/main = M
//	 \              /
//	  s1 ─ … ─ sside
//
// onChain is b1, which IS on origin/main's first-parent chain with the merge
// between it and the ref: the plain count is side+1 (M and every s) and the
// --first-parent count is 1 (M alone), because --first-parent drops the
// side-branch commits the merge brought in. offChain is sside, which is not
// on that chain at all and which BOTH spellings answer 2 for.
//
// That pair is the whole point of the fixture (ranger-base-1ump9 F1): "off
// the first-parent chain" is the wrong discriminator to write this arm on —
// it is the on-chain sha the two spellings disagree about. ciChain is a
// LINEAR chain, one -p per commit, so nothing built from it can separate
// them and the --first-parent mutant survived every pin in this file.
//
// commit-tree and update-ref for the same reason ciChain uses them: no
// index, no working tree, no hook, and no unqualified commit.
func ciMergeChain(t *testing.T, dir string, side int) (onChain, offChain string) {
	t.Helper()
	tree, err := git(dir, "hash-object", "-t", "tree", "-w", os.DevNull)
	if err != nil {
		t.Fatalf("hash-object: %v %s", err, tree)
	}
	tree = strings.TrimSpace(tree)
	commit := func(msg string, parents ...string) string {
		t.Helper()
		args := []string{"commit-tree", tree, "-m", msg}
		for _, p := range parents {
			args = append(args, "-p", p)
		}
		sha, cerr := git(dir, args...)
		if cerr != nil {
			t.Fatalf("commit-tree %s: %v %s", msg, cerr, sha)
		}
		return strings.TrimSpace(sha)
	}
	b0 := commit("b0")
	onChain = commit("b1", b0)
	offChain = b0
	for i := 0; i < side; i++ {
		offChain = commit("s"+strconv.Itoa(i+1), offChain)
	}
	merge := commit("merge", onChain, offChain)
	if out, uerr := git(dir, "update-ref", "refs/remotes/origin/main", merge); uerr != nil {
		t.Fatalf("update-ref: %v %s", uerr, out)
	}
	return onChain, offChain
}

// ciSaneBound keeps the three pins below from building a commit per unit of a
// bound somebody raised to make them pass. Every one of their fixtures is as
// deep as ciFreshMaxBehind — a chain for two of them and a side branch for
// the third — so the pin's own cost IS the bound, and a bound this far past
// the census in its doc comment (max 57 legitimate, over 1,357 reconstructed
// instants of this gate's history) is not a bound. Failing here in a second
// beats a suite that hangs building a hundred thousand commits.
func ciSaneBound(t *testing.T) {
	t.Helper()
	if ciFreshMaxBehind > 512 {
		t.Fatalf("ciFreshMaxBehind = %d: a page that far behind is every page, so the guard is off — re-read the census in its doc comment before moving it", ciFreshMaxBehind)
	}
}

// ciPage rewrites what the fake gh will answer with.
func ciPage(t *testing.T, runs ...string) {
	t.Helper()
	body := "[" + strings.Join(runs, ",") + "]"
	if err := os.WriteFile(filepath.Join(fakeDirOf(t), "fake-gh-runs.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// THE READING IS ONLY A READING IF IT IS CURRENT (ciFreshness,
// ranger-base-m46kr). GitHub's run-list index hands out PAST pages: on
// 2026-10-05 this gate filed ranger-base-17jhu at P1 off a page topped
// 2026-09-11, 24 days and 220 commits behind a main that was green in all
// six jobs at its tip.
//
// Both directions are pinned, because only one of them leaves a trace. A
// stale page topped RED files a false bead someone reads; a stale page topped
// GREEN is byte-identical to a clean pass and hides a red main for as long as
// the index does — this file's founding incident, 191 reds over five days.
// Neither may produce a verdict.
func TestReadCIAbstainsOnARunListPageTheIndexServedFromBehind(t *testing.T) {
	t.Parallel()
	ciSaneBound(t)
	dir, bin := ghRepo(t, "ci.yml")
	// Deep enough to sit either side of the bound, and the bound is read
	// from the const rather than spelled again: a pin that restates the
	// number cannot tell anyone the number moved.
	chain := ciChain(t, dir, ciFreshMaxBehind+8)
	behind := func(k int) string { return chain[len(chain)-1-k] }
	read := func() CIState { return ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin}) }

	// The episode as it happened: the page the index served, topped at the
	// 2026-09-11 run whose streak ranger-base-17jhu went on to name.
	stale := time.Date(2026, 9, 11, 8, 59, 43, 0, time.UTC)
	for _, conclusion := range []string{"failure", "success"} {
		ciPage(t,
			ciRunJSON(behind(ciFreshMaxBehind+5), "completed", conclusion, stale),
			ciRunJSON(behind(ciFreshMaxBehind+6), "completed", conclusion, stale.Add(-4*time.Minute)),
		)
		st := read()
		if st.Known() {
			t.Fatalf("a %s page %d commits behind origin/main produced a verdict (red=%v): the index serves past pages and this one is not about the branch it names",
				conclusion, ciFreshMaxBehind+5, st.Red)
		}
		if !strings.Contains(st.Why, "not current") || !strings.Contains(st.Why, strconv.Itoa(ciFreshMaxBehind)) {
			t.Errorf("why = %q, want the staleness and the bound named", st.Why)
		}
		if st.NoGate {
			t.Error("a stale page is NOT NoGate: there is a gate and this pass could not read it, which is the kind that must be SAID (ciAbstain)")
		}
		if st.Latest.Sha != behind(ciFreshMaxBehind+5) {
			t.Errorf("latest = %q; the rejected reading is kept as measured so a reader can re-run the count the Why names", st.Latest.Short())
		}
	}

	// THE BOUNDARY IS THE MEASURED NUMBER, both sides of it. ciFreshMaxBehind
	// carries the census: legitimate readings reach 57 commits behind over
	// 1,357 reconstructed instants of this gate's own history, and the two
	// stale pages actually observed were 150 and 220.
	for _, c := range []struct {
		k     int
		known bool
	}{
		{0, true}, // the tip itself
		// 5 is the census's p99, and it is here as a LITERAL rather than off
		// the const: a run in flight carries no verdict (ciVerdict), so every
		// commit pushed while CI runs sits ahead of the newest run that has
		// one, and a guard strict enough to reject that abstains in the
		// ordinary mid-flight state — which is the sha-equality shape the
		// measurement rejected. Without this row a bound of 0 passes every
		// other row in this test.
		{5, true},
		{ciFreshMaxBehind, true},      // exactly the bound, still a reading
		{ciFreshMaxBehind + 1, false}, // one past it, no longer one
	} {
		ciPage(t, ciRunJSON(behind(c.k), "completed", "failure", stale))
		st := read()
		if st.Known() != c.known {
			t.Errorf("%d commits behind: known = %v, want %v (why: %s)", c.k, st.Known(), c.known, st.Why)
		}
		if c.known && !st.Red {
			t.Errorf("%d commits behind: a red run inside the bound must still read red", c.k)
		}
	}

	// A sha this checkout has never heard of is the page AHEAD of the local
	// view, not behind it — nothing in a dispatch pass fetches, so the ref
	// this counts against can only lag what GitHub has. Every other test in
	// this file rests on it.
	ciPage(t, ciRunJSON("beefbeefbeefbeefbeefbeefbeefbeefbeefbeef", "completed", "failure", stale))
	if st := read(); !st.Known() || !st.Red {
		t.Errorf("a run on a commit this checkout does not have must still read: known=%v why=%q", st.Known(), st.Why)
	}

	// And no local reference point at all is an abstention rather than a
	// reading taken on trust: a checkout that stops fetching would otherwise
	// lose the guard with nothing anywhere saying so.
	if out, err := git(dir, "update-ref", "-d", "refs/remotes/origin/main"); err != nil {
		t.Fatalf("update-ref -d: %v %s", err, out)
	}
	ciPage(t, ciRunJSON(behind(0), "completed", "failure", stale))
	st := read()
	if st.Known() || !strings.Contains(st.Why, "refs/remotes/origin/main") {
		t.Errorf("no remote-tracking ref: known=%v why=%q", st.Known(), st.Why)
	}
	if st.NoGate {
		t.Error("a checkout with no origin/main is not a repo with no gate: the gate is there and went unread")
	}
}

// THE SPELLING OF THE COUNT, which the whole guard rests on: plain
// `rev-list --count` and NOT --first-parent (ciFreshness's own paragraph).
// ranger-base-1ump9 F1: swapping the shipped line for --first-parent reds
// NOTHING else in this file, because every other fixture here is a chain
// ciChain built — one -p per commit — and two spellings of a first-parent
// walk cannot differ on a chain that has only first parents.
//
// Plain never UNDERCOUNTS the distance, and undercounting is the fail-open
// direction this guard exists to refuse: a stale page read as current leaves
// no trace at all, which is this file's founding incident. MEASURED
// 2026-10-05 over this repo's whole first-parent chain (1,985 commits, git
// 2.50.1), 1,254 shas answer differently under the two spellings and plain
// is never the smaller — gaps up to 53 commits, because plain counts the
// side-branch commits a merge brought in and --first-parent does not.
// Against a bound of 64 a gap of 53 is a page whose true distance is outside
// the bound and whose --first-parent distance is inside it.
//
// No sha within the bound disagrees TODAY, for a reason that is a distance
// and not a property: the newest merge on the chain is 730 commits back, so
// the exposure is CONSTRUCTIBLE rather than present and the next `merge main
// into <branch>` before a fast-forward brings it back. This is the fixture
// that constructs it, and the straddle is asserted below rather than assumed
// — see ciFreshness's own paragraph and
// docs/notes.d/ranger-base-m46kr.md for the census.
func TestCIFreshnessCountsEveryCommitBehindAndNotTheFirstParentChainAlone(t *testing.T) {
	t.Parallel()
	ciSaneBound(t)
	dir, bin := ghRepo(t, "ci.yml")
	// One merge bringing in more commits than the bound, which is what puts
	// the two spellings on OPPOSITE sides of it.
	onChain, offChain := ciMergeChain(t, dir, ciFreshMaxBehind+4)
	count := func(sha string, args ...string) int {
		t.Helper()
		args = append(append([]string{"rev-list", "--count"}, args...), sha+"..refs/remotes/origin/main")
		out, err := git(dir, args...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		n, cerr := strconv.Atoi(strings.TrimSpace(out))
		if cerr != nil {
			t.Fatalf("git %v answered %q", args, out)
		}
		return n
	}

	// THE FIXTURE'S SEPARATING PROPERTY IS AN ASSERTION, not a comment: a
	// linear chain of the same depth passes the ReadCI arm below for the
	// wrong reason — plain is over the bound there too — and would pin
	// nothing about the spelling. This is the row that says the fixture can
	// still tell the two apart.
	plain, fp := count(onChain), count(onChain, "--first-parent")
	if !(fp <= ciFreshMaxBehind && plain > ciFreshMaxBehind) {
		t.Fatalf("the fixture no longer straddles the bound: %s is plain %d / --first-parent %d against ciFreshMaxBehind %d, so the arm below cannot tell the two spellings apart",
			onChain[:8], plain, fp, ciFreshMaxBehind)
	}
	// AND "OFF THE FIRST-PARENT CHAIN" IS THE WRONG DISCRIMINATOR to have
	// written the arm on, which is the half of ciFreshness's old paragraph
	// that was wrong about its mechanism: `--first-parent A..B` limits the
	// NEGATIVE traversal to first parents too, so A's own first-parent
	// ancestry is still excluded and the walk is not the whole chain. An
	// off-chain sha one merge below the ref answers the SAME small number
	// under both spellings.
	if p, f := count(offChain), count(offChain, "--first-parent"); p != f {
		t.Errorf("%s is off the first-parent chain and the two spellings answer %d and %d: the disagreement this guard cares about is the ON-chain one", offChain[:8], p, f)
	}

	read := func(sha string) CIState {
		t.Helper()
		ciPage(t, ciRunJSON(sha, "completed", "success", time.Date(2026, 9, 11, 8, 59, 43, 0, time.UTC)))
		return ReadCI(CIQuery{Dir: dir, Workflow: "ci.yml", GhBin: bin})
	}

	// THE KILL. A page topped by a sha that is ON the chain, with a merge
	// between it and the ref, is `plain` commits behind — past the bound, so
	// it is not a reading. Under --first-parent it is `fp` commits behind,
	// inside the bound, and a stale GREEN page would read as an all-clear
	// with nothing anywhere saying so.
	if st := read(onChain); st.Known() {
		t.Errorf("a page %d commits behind origin/main produced a verdict (red=%v, why=%q): the count must be every commit behind, not the %d the first-parent chain alone answers",
			plain, st.Red, st.Why, fp)
	}
	// And the guard has not just been turned up: the off-chain sha, which
	// both spellings put at the same small distance, still reads.
	if st := read(offChain); !st.Known() {
		t.Errorf("a page %d commits behind origin/main abstained (why=%q) — merges do not make a current reading stale", count(offChain), st.Why)
	}
}

// The property that must not regress, over the real pass rather than over
// ReadCI alone: a stale page files NOTHING and says why once, and a current
// red page still files on the FIRST red. The second half is the whole reason
// this mechanism exists, and a freshness guard that bought silence with it
// would be worse than the bead it prevents.
func TestCIWatchFilesNothingOffAStalePageAndStillFilesOffACurrentOne(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	ciSaneBound(t)
	dir, bin := ghRepo(t, "ci.yml")
	chain := ciChain(t, dir, ciFreshMaxBehind+3)
	if err := os.WriteFile(a.ConfigPath, []byte("beads:\n  - "+dir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The real ReadCI, driven through the fake gh — the guard is in the
	// reading, so a pinned CIState would pin nothing about it.
	a.CIRead = func(q CIQuery) CIState { q.GhBin = bin; return ReadCI(q) }
	bd := testBd(t)

	at := time.Date(2026, 9, 11, 8, 59, 43, 0, time.UTC)
	ciPage(t, ciRunJSON(chain[0], "completed", "failure", at))
	acted, out, errs := cwRun(t, a, bd)
	if acted != 0 || cwSay(out) != "" {
		t.Fatalf("a stale page acted %d and said %q, want neither", acted, cwSay(out))
	}
	if n := cwCount(t, "create"); n != 0 {
		t.Fatalf("%d beads filed off a page %d commits behind origin/main, want 0", n, len(chain)-1)
	}
	if !strings.Contains(errs, "ci-watch:") || !strings.Contains(errs, "not current") {
		t.Errorf("stderr = %q, want the could-not-READ notice: silence is what an all-clear looks like", errs)
	}
	// Once per process, like every other abstention here: a condition that
	// recurs must not be re-announced every pass.
	if _, _, errs2 := cwRun(t, a, bd); errs2 != "" {
		t.Errorf("the second pass over the same stale reading said %q", errs2)
	}

	// Current page, same red run: the bead gets filed.
	ciPage(t, ciRunJSON(chain[len(chain)-1], "completed", "failure", at))
	acted, out, errs = cwRun(t, a, bd)
	if acted != 1 {
		t.Fatalf("a CURRENT red page acted %d, want 1 — stdout %q stderr %q", acted, out, errs)
	}
	if !strings.Contains(out, "ci red ·") {
		t.Errorf("the filing pass said %q", out)
	}
	if n := cwCount(t, "create"); n != 1 {
		t.Errorf("%d creates, want 1", n)
	}
}

// THE WHOLE COST OF THE EPISODE, at the pass: no bead, no dispatched
// session, nothing on stderr — and then a real red through the same path
// files one. The vet lives in the READING, so a pinned CIState would pin
// nothing about it (the stale-page pin's rule).
func TestCIWatchFilesNothingOverAFailureWhoseRedJobNeverRanAndStillFilesARealOne(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	dir, bin := ghRepo(t, "ci.yml")
	if err := os.WriteFile(a.ConfigPath, []byte("beads:\n  - "+dir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.CIRead = func(q CIQuery) CIState { q.GhBin = bin; return ReadCI(q) }
	bd := testBd(t)
	at := time.Date(2026, 10, 5, 19, 21, 0, 0, time.UTC)

	// The episode: one `failure` whose only non-success job never started,
	// over a green run. This filed ranger-base-94grm at P1.
	ciPage(t,
		ciRunJSONID("bab60e51", "completed", "failure", at, "9001"),
		ciRunJSONID("f5ccb1a8", "completed", "success", at.Add(-time.Hour), "9000"),
	)
	writeGhJobs(t, "9001", ciPhantomJobsJSON())
	acted, out, errs := cwRun(t, a, bd)
	if acted != 0 || cwSay(out) != "" {
		t.Fatalf("acted %d and said %q over a job that never ran, want neither", acted, cwSay(out))
	}
	if n := cwCount(t, "create"); n != 0 {
		t.Fatalf("%d beads filed over a run nothing failed in, want 0", n)
	}
	if errs != "" {
		// A run with no verdict is skipped, exactly as a cancelled run is,
		// and a skipped run has never printed anything.
		t.Errorf("stderr = %q over a green branch, want silence", errs)
	}

	// Same page, same run id, but the job really failed: the bead is filed.
	writeGhJobs(t, "9001", ciJobsJSON(
		ciJobJSON("test (ubuntu-latest, 3)", "failure", 11, "GitHub Actions 1"),
		ciJobJSON("test (macos-latest, 1)", "success", 11, "GitHub Actions 2"),
	))
	acted, out, errs = cwRun(t, a, bd)
	if acted != 1 {
		t.Fatalf("a REAL red acted %d, want 1 — stdout %q stderr %q", acted, out, errs)
	}
	if n := cwCount(t, "create"); n != 1 {
		t.Errorf("%d creates, want 1", n)
	}
}

// ─── the filing ──────────────────────────────────────────────────────────────

// cwRepo points config `beads:` at a fresh repo and pins the reading to a
// canned state, so the filing side is driven without a network.
func cwRepo(t *testing.T, a *App, cfg ...string) string {
	t.Helper()
	repo := t.TempDir()
	conf := "beads:\n  - " + repo + "\n" + strings.Join(cfg, "\n")
	if err := os.WriteFile(a.ConfigPath, []byte(conf+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func redState(streak int) CIState {
	at := time.Date(2026, 8, 30, 1, 53, 21, 0, time.UTC)
	return CIState{
		Repo: "/r", Slug: "ranger360ai/posse", Workflow: "ci.yml", Branch: "main",
		Red:    true,
		Latest: CIRun{Sha: "d3909c27", URL: "https://x/1", Conclusion: "failure", Created: at.Add(time.Hour)},
		Since:  CIRun{Sha: "8d50fed5", URL: "https://x/0", Conclusion: "failure", Created: at},
		Streak: streak,
	}
}

func greenState() CIState {
	s := redState(1)
	s.Red = false
	s.Latest = CIRun{Sha: "0c0607b0", URL: "https://x/9", Conclusion: "success", Created: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)}
	s.Since = s.Latest
	return s
}

// cwRun runs one pass and returns (acted, stdout, stderr).
func cwRun(t *testing.T, a *App, bd Bd) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	n := a.CIWatch(bd, a.BeadsDirs(), &out, &errb)
	return n, out.String(), errb.String()
}

func cwBdCalls(t *testing.T) []string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(fakeDirOf(t), "bd-calls.log"))
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// cwCount counts bd invocations of a verb. It matches the VERB as it
// appears after the actor flag rather than as a bare substring, because a
// bare "create" also matches the `--json conclusion,status,createdAt,...`
// that ci-watch writes into every bead's Reproduce block — which counted two
// creates where the store held one and made this suite's central invariant
// pass for the wrong reason.
func cwCount(t *testing.T, verb string) int {
	t.Helper()
	n := 0
	for _, l := range cwBdCalls(t) {
		if strings.Contains(l, "--no-daemon --actor "+VerifyActor+" "+verb) || strings.Contains(l, "--no-daemon "+verb) {
			n++
		}
	}
	return n
}

// cwOnlyRepo is the one repo `beads:` names — cwRepo configured it — so a
// test that files through several episodes can reach the listing cwHold
// rewrites without carrying the path around.
func cwOnlyRepo(t *testing.T, a *App) string {
	t.Helper()
	dirs := a.BeadsDirs()
	if len(dirs) != 1 {
		t.Fatalf("config names %d beads repos, want 1", len(dirs))
	}
	return dirs[0]
}

// cwHold puts a bead in the state a DISPATCHED SEAT leaves it in — status
// in_progress, assigned — by rewriting the listing the dedupe reads. The
// fake's own claim state (fakeBdApplyState) rides `ready` and `show`, not
// `list --label-any`, and this mechanism only ever reads the listing; a
// fixture that claimed through `bd update` would leave the row ci-watch
// actually sees untouched and pin nothing.
//
// It is the whole fixture for ADR 0013 §4's exception having an EDGE: an
// unclaimed bead is closed by the harness and a claimed one is not, and only
// a test that can produce a claimed row can tell those apart.
func cwHold(t *testing.T, repo, id, assignee string) {
	t.Helper()
	path := filepath.Join(repo, "fake-list-labeled.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cwHold: %v", err)
	}
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("cwHold: %v\n%s", err, body)
	}
	hit := false
	for _, is := range list {
		if s, _ := is["id"].(string); s == id {
			is["status"], is["assignee"], hit = "in_progress", assignee, true
		}
	}
	if !hit {
		t.Fatalf("cwHold: %s is not in the labeled listing:\n%s", id, body)
	}
	nb, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nb, 0o644); err != nil {
		t.Fatal(err)
	}
}

// cwSay is what ci-watch itself said, with the launcher lock's own waiting
// notice dropped: that line belongs to a lock this package's parallel suite
// shares, not to the mechanism under test.
func cwSay(out string) string {
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "\u23f3") {
			continue
		}
		keep = append(keep, l)
	}
	return strings.Join(keep, "\n")
}

// THE INVARIANT. A red that stays red produces ONE open bead, not one per
// push — a mechanism that files per red run during the next five-day red is
// worse than the silence it replaces. Ten passes over a red gate, one
// create.
func TestCIWatchFilesOneBeadPerEpisodeNotOnePerPass(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	bd := testBd(t)

	acted, out, errs := cwRun(t, a, bd)
	if acted != 1 {
		t.Fatalf("first pass acted %d, want 1 (stderr: %s)", acted, errs)
	}
	if !strings.Contains(out, "ci red ·") || !strings.Contains(out, "filed q-1") {
		t.Errorf("first pass said %q", out)
	}
	for i := 2; i <= 10; i++ {
		if n, out, _ := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
			t.Fatalf("pass %d acted %d and said %q; the open bead is the dedupe", i, n, cwSay(out))
		}
	}
	if n := cwCount(t, "create"); n != 1 {
		t.Errorf("%d creates across ten red passes, want 1", n)
	}
}

// And the pass that files says so exactly once: the standing condition is
// the BEAD's to carry, not a line repeated every pass for five days.
func TestCIWatchPassSpeaksOnlyWhenItActs(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	bd := testBd(t)
	cwRun(t, a, bd)
	for i := 0; i < 3; i++ {
		if _, out, _ := cwRun(t, a, bd); cwSay(out) != "" {
			t.Fatalf("a pass that did nothing printed %q", cwSay(out))
		}
	}
	a.CIRead = func(CIQuery) CIState { return greenState() }
	_, out, _ := cwRun(t, a, bd)
	if !strings.Contains(out, "ci green ·") || !strings.Contains(out, "0c0607b0") {
		t.Errorf("the closing pass said %q, want the run that cleared it", out)
	}
}

// Green again on a bead NO SESSION EVER CLAIMED: the bead is told which run
// cleared the gate and then CLOSED — ADR 0013 §4's one exception, ruled on
// ranger-base-8fr2j. The comment comes first and is therefore the close
// comment, which is what the ruling's DONE WHEN asks for: a closed bead with
// the clearing run on it.
func TestCIWatchClosesTheBeadNoSessionEverClaimed(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(4) }
	bd := testBd(t)
	if n, _, e := cwRun(t, a, bd); n != 1 {
		t.Fatalf("file: %d (%s)", n, e)
	}

	a.CIRead = func(CIQuery) CIState { return greenState() }
	n, out, errs := cwRun(t, a, bd)
	if n != 1 {
		t.Fatalf("close pass acted %d, want 1 (stderr %s)", n, errs)
	}
	if !strings.Contains(out, "q-1") || !strings.Contains(out, "CLOSED") {
		t.Errorf("said %q, want the line to name the bead and the close", out)
	}
	if got := cwCount(t, "close"); got != 1 {
		t.Errorf("%d closes over a bead nobody claimed, want 1: %v", got, cwBdCalls(t))
	}
	// The close comment carries the run that cleared the gate — the whole
	// of what the ruling asks a closed ci-red bead to say.
	body, _ := os.ReadFile(filepath.Join(repo, "fake-comments.json"))
	for _, want := range []string{ciClearedPrefix, "0c0607b0", "https://x/9", "ranger-base-8fr2j"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the close comment is missing %q: %s", want, body)
		}
	}
	// And the close really left the store: a second green pass has no bead
	// to walk, and a NEW red is a NEW bead rather than a comment on a
	// closed one.
	if n, out, _ := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
		t.Errorf("a second green pass acted %d and said %q", n, cwSay(out))
	}
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	if n, _, _ := cwRun(t, a, bd); n != 1 {
		t.Errorf("a red following the closed episode filed %d beads, want 1", n)
	}
	if got := cwCount(t, "create"); got != 2 {
		t.Errorf("%d creates over two episodes, want 2", got)
	}
}

// The clear comment carries the streak's own causes when the reading found
// a prior red streak to ask about — App.CICauses is the seam (the same
// reason CIRead is one), so this drives ciClear without a real gh
// (ranger-base-d6zyu finding 3).
func TestCIWatchClearCommentNamesTheStreaksCauses(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(2) }
	bd := testBd(t)
	if n, _, e := cwRun(t, a, bd); n != 1 {
		t.Fatalf("file: %d (%s)", n, e)
	}

	var gotDir, gotSlug string
	var gotRuns []CIRun
	green := greenState()
	green.PriorRedRuns = []CIRun{
		{Sha: "r1", URL: "https://x/2"},
		{Sha: "r0", URL: "https://x/1"},
	}
	a.CICauses = func(dir, slug string, runs []CIRun) []string {
		gotDir, gotSlug, gotRuns = dir, slug, runs
		return []string{"TestFlaky (2 runs)"}
	}
	a.CIRead = func(CIQuery) CIState { return green }
	if n, _, errs := cwRun(t, a, bd); n != 1 {
		t.Fatalf("close pass acted %d (stderr %s)", n, errs)
	}
	if gotSlug != green.Slug || len(gotRuns) != 2 {
		t.Errorf("CICauses called with slug %q runs %v, want %q and the 2 prior red runs", gotSlug, gotRuns, green.Slug)
	}
	if gotDir == "" {
		t.Errorf("CICauses called with an empty dir")
	}
	body, _ := os.ReadFile(filepath.Join(repo, "fake-comments.json"))
	if !strings.Contains(string(body), "TestFlaky (2 runs)") {
		t.Errorf("clear comment is missing the reported cause: %s", body)
	}
}

// The ordinary clear — no prior red streak in the reading — asks CICauses
// nothing at all, and says nothing about causes. Most clears follow a
// streak of exactly the run that filed the bead (finding 3's own examples
// were 1 and 5 runs), so this is the common path, not the edge.
func TestCIWatchClearAsksNothingWhenThereIsNoPriorStreak(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	bd := testBd(t)
	if n, _, e := cwRun(t, a, bd); n != 1 {
		t.Fatalf("file: %d (%s)", n, e)
	}

	called := false
	a.CICauses = func(dir, slug string, runs []CIRun) []string {
		called = true
		return []string{"should never be asked"}
	}
	a.CIRead = func(CIQuery) CIState { return greenState() } // PriorRedRuns is nil
	if n, _, errs := cwRun(t, a, bd); n != 1 {
		t.Fatalf("close pass acted %d (stderr %s)", n, errs)
	}
	if called {
		t.Errorf("CICauses was called with no prior red streak to ask about")
	}
	body, _ := os.ReadFile(filepath.Join(repo, "fake-comments.json"))
	if strings.Contains(string(body), "CAUSES") {
		t.Errorf("clear comment names causes with nothing to name: %s", body)
	}
}

// THE EDGE, and the arm the exception lives or dies on: the same red-then-
// green episode over a bead a SEAT HOLDS leaves it OPEN with the comment.
//
// ADR 0013 §4's exception is a bead the harness filed that no session ever
// claimed; a bead somebody is working is somebody's record, and closing it
// out from under them is the "harness closes the bead on the agent's behalf"
// case the section rejects in as many words. Flip the guard in ciHolder and
// this goes red; the live tree's other half is
// TestNoBdCloseVerbReachableFromDispatch's arm-2 register.
func TestCIWatchDoesNotCloseTheBeadASeatHolds(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(4) }
	bd := testBd(t)
	if n, _, e := cwRun(t, a, bd); n != 1 {
		t.Fatalf("file: %d (%s)", n, e)
	}
	cwHold(t, repo, "q-1", "devops")

	a.CIRead = func(CIQuery) CIState { return greenState() }
	n, out, errs := cwRun(t, a, bd)
	if n != 1 {
		t.Fatalf("clear pass acted %d, want 1 (stderr %s)", n, errs)
	}
	if cwCount(t, "close") != 0 {
		t.Fatalf("ci-watch closed a bead a seat holds: %v", cwBdCalls(t))
	}
	if !strings.Contains(out, "said on q-1") || !strings.Contains(out, "devops") {
		t.Errorf("said %q, want the line to name who it left the close to", out)
	}
	body, _ := os.ReadFile(filepath.Join(repo, "fake-comments.json"))
	for _, want := range []string{ciClearedPrefix, "0c0607b0", "CLOSE IT", "devops is assigned it (in_progress)"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the clearing comment is missing %q: %s", want, body)
		}
	}
	// The bead outlives its episode, so the rest of the shipped contract
	// still has to hold over it: a second green pass says nothing…
	if n, out, _ := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
		t.Errorf("a second green pass acted %d and said %q", n, cwSay(out))
	}
	// …and a red after that green is a NEW episode with its own bead, even
	// though the cleared one is STILL OPEN — ciAlreadyCleared is what steps
	// the dedupe over it, and without that the crew never hears the second
	// red.
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	if n, _, _ := cwRun(t, a, bd); n != 1 {
		t.Errorf("a red following a green filed %d beads, want 1", n)
	}
	if got := cwCount(t, "create"); got != 2 {
		t.Errorf("%d creates over two episodes, want 2", got)
	}
}

// The same episode over an in_progress bead with NOBODY ASSIGNED, which is
// the arm that makes the STATUS half of the guard load-bearing end to end:
// flip `open.Status != "open"` and the fixture above still fails on the
// assignee, so this is the one that goes red on the status guard alone.
//
// The row is not hypothetical — an in_progress bead nobody is assigned is a
// shape this store produces (orphanedclaimnarrow_qa_test.go names it) — and
// the reading is the same either way: a status that is not `open` means a
// claim happened, and what happened to the assignee afterwards is not this
// guard's business.
func TestCIWatchDoesNotCloseAnInProgressBeadNobodyIsAssigned(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(4) }
	bd := testBd(t)
	if n, _, e := cwRun(t, a, bd); n != 1 {
		t.Fatalf("file: %d (%s)", n, e)
	}
	cwHold(t, repo, "q-1", "")

	a.CIRead = func(CIQuery) CIState { return greenState() }
	if n, out, errs := cwRun(t, a, bd); n != 1 || !strings.Contains(out, "in_progress") {
		t.Fatalf("clear pass acted %d and said %q (%s), want the line to name the status it stopped on", n, out, errs)
	}
	if cwCount(t, "close") != 0 {
		t.Errorf("ci-watch closed an in_progress bead: %v", cwBdCalls(t))
	}
	body, _ := os.ReadFile(filepath.Join(repo, "fake-comments.json"))
	if !strings.Contains(string(body), "CLOSE IT") {
		t.Errorf("the clearing comment does not ask the seat to close it: %s", body)
	}
}

// ciHolder's four corners, because the end-to-end pins above drive two of
// them and a guard is only as narrow as its edges. The claim (ranger-base-
// 8fr2j) is "status still open, never in_progress", and the row carries two
// fields that answer it: Bd.Claim sets BOTH, so either one alone is enough
// to say a bead is somebody's.
//
// The unassigned in_progress row is the corner that kills the mutant of the
// STATUS test on its own, and the assigned open row kills the mutant of the
// ASSIGNEE test on its own — without both, one guard could be deleted and
// every other arm here would stay green.
func TestCIHolderIsReadOffTheBead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		is     BdIssue
		closes bool
	}{
		{"open and unassigned: nobody was ever dispatched onto it", BdIssue{Status: "open"}, true},
		{"in_progress and assigned: a seat holds it", BdIssue{Status: "in_progress", Assignee: "devops"}, false},
		{"in_progress with nobody assigned: a claim happened", BdIssue{Status: "in_progress"}, false},
		{"open but assigned: the operator routed it to somebody", BdIssue{Status: "open", Assignee: "developer"}, false},
		{"blocked: not a status this exception knows", BdIssue{Status: "blocked"}, false},
		{"deferred: an answer somebody already gave", BdIssue{Status: "deferred"}, false},
		{"a status bd does not have today: errs toward the seat", BdIssue{Status: "parked"}, false},
	} {
		held := ciHolder(tc.is)
		if (held == "") != tc.closes {
			t.Errorf("%s: ciHolder(%+v) = %q, want closable=%v", tc.name, tc.is, held, tc.closes)
		}
		if !tc.closes && !strings.Contains(held, tc.is.Status) && !strings.Contains(held, tc.is.Assignee) {
			t.Errorf("%s: the reason %q names neither the status nor the holder, so the bead and stdout cannot say why the close was left to a seat", tc.name, held)
		}
	}
}

// The drumbeat: a five-day red says its number when the number has DOUBLED,
// so 191 failures earn eight comments and not 191.
func TestCIWatchCommentsOnDoublingNotOnEveryPass(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	cwRepo(t, a)
	streak := 1
	a.CIRead = func(CIQuery) CIState { return redState(streak) }
	bd := testBd(t)
	cwRun(t, a, bd) // files at 1

	for _, c := range []struct {
		streak int
		want   int // cumulative drumbeat comments
	}{{1, 0}, {1, 0}, {2, 1}, {3, 1}, {3, 1}, {4, 2}, {7, 2}, {8, 3}, {15, 3}, {16, 4}} {
		streak = c.streak
		cwRun(t, a, bd)
		if got := cwCount(t, "comments add q-1"); got != c.want {
			t.Errorf("at streak %d: %d comments, want %d", c.streak, got, c.want)
		}
	}
}

// The drumbeat's state lives ON THE BEAD, not in this process: a launcher
// restart mid-red must not re-say the number from 1. Same store, a fresh
// App — which is what a restart is.
func TestCIWatchDrumbeatSurvivesALauncherRestart(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	bd := testBd(t)
	cwRun(t, a, bd)
	streak := 8
	a.CIRead = func(CIQuery) CIState { return redState(streak) }
	cwRun(t, a, bd)
	if got := cwCount(t, "comments add q-1"); got != 1 {
		t.Fatalf("setup: %d comments, want 1", got)
	}

	fresh := hermetic(t, NewAppAt(t.TempDir()))
	conf := "beads:\n  - " + repo + "\n"
	if err := os.WriteFile(fresh.ConfigPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh.CIRead = func(CIQuery) CIState { return redState(streak) }
	cwRun(t, fresh, bd)
	if got := cwCount(t, "comments add q-1"); got != 1 {
		t.Errorf("a restarted launcher re-said the number: %d comments, want 1", got)
	}
	// …and the NEXT doubling still lands.
	streak = 16
	cwRun(t, fresh, bd)
	if got := cwCount(t, "comments add q-1"); got != 2 {
		t.Errorf("the restarted launcher missed the next doubling: %d comments, want 2", got)
	}
}

// The marker, not the label alone: an instance watching two repos must not
// let one repo's red suppress the other's bead.
func TestCIWatchDedupesPerGateNotPerLabel(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	one, two := t.TempDir(), t.TempDir()
	conf := "beads:\n  - " + one + "\n  - " + two + "\n"
	if err := os.WriteFile(a.ConfigPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	a.CIRead = func(q CIQuery) CIState {
		st := redState(1)
		if strings.HasPrefix(q.Dir, two) {
			st.Slug = "ranger360ai/other"
		}
		return st
	}
	if n, _, e := cwRun(t, a, testBd(t)); n != 2 {
		t.Fatalf("two repos, two red gates, acted %d (stderr %s)", n, e)
	}
	if n := cwCount(t, "create"); n != 2 {
		t.Errorf("%d creates, want one per gate", n)
	}
}

// A dedupe read that FAILED is not an empty store. Filing on it is exactly
// the one-bead-per-push failure this whole mechanism must not have.
func TestCIWatchFilesNothingWhenTheDedupeReadFails(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	os.WriteFile(filepath.Join(repo, "fake-list-fail"), nil, 0o644)
	a.CIRead = func(CIQuery) CIState { return redState(1) }

	n, out, errs := cwRun(t, a, testBd(t))
	if n != 0 || cwSay(out) != "" {
		t.Errorf("acted %d and said %q over a store it could not read", n, cwSay(out))
	}
	if !strings.Contains(errs, "ci-watch:") {
		t.Errorf("silent about a failed dedupe read: %q", errs)
	}
	if got := cwCount(t, "create"); got != 0 {
		t.Errorf("%d creates on a failed dedupe read", got)
	}
}

// The clearing comment IS the green half's whole state — the bead stays open
// (ADR 0013 §4), so a comment that did not land leaves nothing behind. The
// pass must not report a clearing it did not write, and must try again.
func TestCIWatchSaysNothingWhenTheClearingCommentFails(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	bd := testBd(t)
	cwRun(t, a, bd)

	os.WriteFile(filepath.Join(repo, "fake-comment-fail"), []byte("database is locked"), 0o644)
	a.CIRead = func(CIQuery) CIState { return greenState() }
	n, out, errs := cwRun(t, a, bd)
	if n != 0 || cwSay(out) != "" {
		t.Errorf("acted %d and said %q over a comment that did not land", n, cwSay(out))
	}
	if !strings.Contains(errs, "clear comment") {
		t.Errorf("stderr = %q", errs)
	}
	// …and the next pass tries again, because nothing durable says it was
	// said: the clearing comment IS the state.
	os.Remove(filepath.Join(repo, "fake-comment-fail"))
	if n, _, _ := cwRun(t, a, bd); n != 1 {
		t.Errorf("the retry acted %d, want 1", n)
	}
}

// A reading that could not be taken files nothing, closes nothing, and says
// so ONCE — not every pass for the life of a loop, and not silently, because
// silence is what an all-clear looks like.
func TestCIWatchAbstentionIsSaidOnceAndActsNever(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	a.CIRead = func(CIQuery) CIState {
		return CIState{Why: "gh could not list runs (" + repo + ")"}
	}
	_ = repo
	bd := testBd(t)
	said := 0
	for i := 0; i < 4; i++ {
		n, out, errs := cwRun(t, a, bd)
		if n != 0 || cwSay(out) != "" {
			t.Fatalf("pass %d acted %d and said %q over an abstention", i, n, cwSay(out))
		}
		said += strings.Count(errs, "ci-watch:")
	}
	if said != 1 {
		t.Errorf("the abstention was said %d times, want once", said)
	}
	if got := cwCount(t, "create"); got != 0 {
		t.Errorf("%d creates over an abstention", got)
	}
}

// Config turns it off the way an empty verify_labels: turns verify-after
// off, and off means it does not even take the reading.
func TestCIWatchOffWhenWorkflowIsEmpty(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	cwRepo(t, a, "ci_workflow:")
	read := 0
	a.CIRead = func(CIQuery) CIState { read++; return redState(1) }
	if n, out, errs := cwRun(t, a, testBd(t)); n != 0 || cwSay(out) != "" || errs != "" {
		t.Errorf("acted %d, said %q / %q", n, cwSay(out), errs)
	}
	if read != 0 {
		t.Errorf("took %d readings while configured off", read)
	}
}

// The filed bead is the handoff the persona contract describes: it routes on
// `devops`, it carries the dedupe label and marker, it is a P1 bug, and it
// is attributed to the harness rather than to a persona.
func TestCIWatchFiledBeadCarriesItsContract(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	st := redState(191)
	a.CIRead = func(CIQuery) CIState { return st }
	cwRun(t, a, testBd(t))

	var list []map[string]any
	body, err := os.ReadFile(filepath.Join(repo, "fake-list.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &list); err != nil || len(list) != 1 {
		t.Fatalf("store holds %d rows (%v)", len(list), err)
	}
	desc, _ := list[0]["description"].(string)
	for _, want := range []string{
		ciMarker(st), ciStreakPrefix + "191", "8d50fed5", "d3909c27",
		"gh run list --repo ranger360ai/posse --workflow=ci.yml --branch main",
		"DONE WHEN:",
	} {
		if !strings.Contains(desc, want) {
			t.Errorf("description missing %q:\n%s", want, desc)
		}
	}
	// No commit message rides in: the bead is built from run metadata only.
	call := strings.Join(cwBdCalls(t), "\n")
	for _, want := range []string{"-l " + CIRedLabel + "," + CIRedLane, "-p 1", "-t bug", "--actor " + VerifyActor} {
		if !strings.Contains(call, want) {
			t.Errorf("bd create argv missing %q:\n%s", want, call)
		}
	}
	title, _ := list[0]["title"].(string)
	if !strings.Contains(title, "ci is red on main") {
		t.Errorf("title = %q", title)
	}
}

// The title does not move while the bead is open — the streak, the sha and
// the URL all change under it, and a title that changed would break every
// human's memory of which bead this is.
func TestCITitleIsStableAcrossTheEpisode(t *testing.T) {
	t.Parallel()
	first := redState(1).Title()
	later := redState(191)
	later.Latest = CIRun{Sha: "ffffffff", Created: time.Now()}
	if later.Title() != first {
		t.Errorf("title moved: %q then %q", first, later.Title())
	}
}

func TestCILastStreakReadsTheLargestNumberStated(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]int{
		"":                        0,
		"nothing here":            0,
		ciStreakPrefix + "7 blah": 7,
		"x\n" + ciStreakPrefix + "12 since a\n" + ciStreakPrefix + "4 since b": 12,
		"  " + ciStreakPrefix + "9": 0, // not at the start of a line: not the marker
	} {
		if got := ciLastStreak(text); got != want {
			t.Errorf("ciLastStreak(%q) = %d, want %d", text, got, want)
		}
	}
}

// A store that answers the dedupe query with CLOSED rows must not make
// ci-watch adopt one. Both store classes exist on bd 0.50.3 — the shop's
// SQLite store drops them here, the `no-db: true` JSONL store keeps them
// (measured 2026-09-04, ciwatch_live_test.go, which failed on this before
// ciOpenBeads asserted open for itself). Adopting a closed bead is the
// silent version of the bug this whole mechanism exists to prevent: the
// gate goes red for five days and nothing is filed, because the dedupe is
// holding a bead that says the last episode is over.
//
// The closed bead is SEEDED rather than produced by a close, because
// ci-watch does not close (ADR 0013 §4) — a persona did, which is exactly
// how a closed ci-red bead comes to be in the store.
func TestCIWatchNeverAdoptsAClosedBead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	if err := os.WriteFile(filepath.Join(repo, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	st := redState(1)
	seeded := `[{"id":"x-1","title":"` + st.Title() + `",` +
		`"status":"closed","labels":["` + CIRedLabel + `","` + CIRedLane + `"],` +
		`"description":` + strconv.Quote(ciMarker(st)+"\n"+ciStreakPrefix+"4 consecutive failed run(s)\n") + `}]`
	if err := os.WriteFile(filepath.Join(repo, "fake-list-labeled.json"), []byte(seeded), 0o644); err != nil {
		t.Fatal(err)
	}
	a.CIRead = func(CIQuery) CIState { return st }
	bd := testBd(t)

	n, out, errs := cwRun(t, a, bd)
	if n != 1 {
		t.Fatalf("the red acted %d, want 1 — the dedupe adopted a CLOSED bead (out %q err %q)", n, cwSay(out), errs)
	}
	if got := cwCount(t, "create"); got != 1 {
		t.Errorf("%d creates, want 1", got)
	}
	// The control: the SAME seeded bead, open, does suppress it. Without
	// this arm the assertion above passes over a dedupe that adopts nothing
	// at all.
	b2, _ := newTestBackend(t)
	a2 := b2.App
	repo2 := cwRepo(t, a2)
	if err := os.WriteFile(filepath.Join(repo2, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo2, "fake-list-labeled.json"),
		[]byte(strings.Replace(seeded, `"status":"closed"`, `"status":"open"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	a2.CIRead = func(CIQuery) CIState { return st }
	if n, out, _ := cwRun(t, a2, testBd(t)); n != 0 || cwSay(out) != "" {
		t.Errorf("the OPEN control acted %d and said %q — the dedupe adopts nothing", n, cwSay(out))
	}
}

// ranger-base-jbnug: ci-watch filed duplicate ci-red beads (d3dgd, x11iy) for
// one episode. d3dgd was closed by the fixer's own `bd close` at 05:04:24,
// the run that actually cleared the gate (3f4402a7) went green on GitHub at
// 05:07:46, and x11iy was filed at 05:09:16 — for the identical "since
// d2f13e68" episode — by a pass whose own `gh run list` reading raced the
// fix and was still red. ciOpenBeads only ever sees OPEN beads, so d3dgd's
// close dropped it out of the dedupe before ci-watch's reading caught up,
// and the stale-red pass found nothing open and filed a second bead.
//
// The closed bead is SEEDED with the real streak line (st.Description(), so
// its "since <sha> at <time>" text is the shipped shape ciSinceSha reads)
// and no comments, which is the whole of what makes it look like a fixer's
// own close rather than one ci-watch itself already answered.
func TestCIWatchDoesNotRefileOverAnEpisodeASeatAlreadyClosed(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	if err := os.WriteFile(filepath.Join(repo, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	st := redState(1)
	seeded := `[{"id":"d3dgd","title":"` + st.Title() + `",` +
		`"status":"closed","labels":["` + CIRedLabel + `","` + CIRedLane + `"],` +
		`"description":` + strconv.Quote(st.Description()) + `}]`
	if err := os.WriteFile(filepath.Join(repo, "fake-list-labeled.json"), []byte(seeded), 0o644); err != nil {
		t.Fatal(err)
	}
	// The gate's own read is STILL RED — it raced the fix — so this is
	// exactly the pass that would have filed x11iy.
	a.CIRead = func(CIQuery) CIState { return st }
	bd := testBd(t)

	n, out, errs := cwRun(t, a, bd)
	if n != 0 || cwSay(out) != "" {
		t.Fatalf("a stale-red pass over an episode a seat already closed acted %d and said %q (stderr %s) — this is the duplicate-file bug", n, cwSay(out), errs)
	}
	if got := cwCount(t, "create"); got != 0 {
		t.Errorf("%d creates over an episode already closed by a seat, want 0", got)
	}

	// The control: the SAME closed bead, but ci-watch itself already told it
	// its gate cleared (ciClearedPrefix). That is an ANSWERED episode, and
	// treating it as a reason not to file the NEXT red would be the refile
	// cooldown the header rejects — so this must still file.
	b2, _ := newTestBackend(t)
	a2 := b2.App
	repo2 := cwRepo(t, a2)
	if err := os.WriteFile(filepath.Join(repo2, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo2, "fake-list-labeled.json"), []byte(seeded), 0o644); err != nil {
		t.Fatal(err)
	}
	cleared := `[{"issue_id":"d3dgd","author":"` + VerifyActor + `","text":` +
		strconv.Quote(ciClearedPrefix+"ci.yml is green again on main — 3f4402a7 at 2026-09-07T09:07:46Z, https://x/9.") + `}]`
	if err := os.WriteFile(filepath.Join(repo2, "fake-comments.json"), []byte(cleared), 0o644); err != nil {
		t.Fatal(err)
	}
	a2.CIRead = func(CIQuery) CIState { return st }
	if n, _, e := cwRun(t, a2, testBd(t)); n != 1 {
		t.Fatalf("an episode ci-watch itself already cleared should still let the next red file, acted %d (%s)", n, e)
	}
}

// ranger-base-95mw3, escaped from ranger-base-jbnug's own fix: marker-and-sha
// alone cannot tell jbnug's race (a fixer's close landing seconds ahead of
// this gate's own read catching up to green) from a bead closed WITHOUT the
// gate ever recovering — Since does not move while one streak keeps failing,
// so both leave a closed bead sharing the live episode's Since sha while the
// read is still red. d3dgd itself, named in jbnug's own description, was
// closed "verified locally, ahead of the CI run finishing" — exactly the
// second shape. Unbounded, ciDupeFiled matched it forever and ci-watch never
// filed again for the rest of a genuinely still-red episode.
//
// The fix bounds the suppression to ONE pass: the first still-red pass over
// the closed bead marks it (ciRaceAbstainPrefix) and abstains, same as
// before. A SECOND still-red pass over the same episode reads that mark
// back and files — the grace already had its one chance to turn green (the
// race jbnug's fix targets resolves by the very next read) and did not, so
// this is not a race any more.
func TestCIWatchStaysSilentAfterAPrematureCloseOnAStillRedStreak(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	if err := os.WriteFile(filepath.Join(repo, "fake-list-keep-closed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	st := redState(3)
	seeded := `[{"id":"d3dgd","title":"` + st.Title() + `",` +
		`"status":"closed","labels":["` + CIRedLabel + `","` + CIRedLane + `"],` +
		`"description":` + strconv.Quote(st.Description()) + `}]`
	if err := os.WriteFile(filepath.Join(repo, "fake-list-labeled.json"), []byte(seeded), 0o644); err != nil {
		t.Fatal(err)
	}
	// The gate's own read is still red on every pass — this streak never
	// actually recovered, unlike jbnug's race.
	a.CIRead = func(CIQuery) CIState { return st }
	bd := testBd(t)

	// Pass 1: indistinguishable from jbnug's race on this read alone, so it
	// gets the same benefit of the doubt — abstain, no create.
	if n, out, errs := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
		t.Fatalf("the first still-red pass over a closed bead acted %d and said %q (stderr %s)", n, cwSay(out), errs)
	}
	if got := cwCount(t, "create"); got != 0 {
		t.Fatalf("%d creates after the first still-red pass, want 0", got)
	}
	cs, err := bd.Comments(repo, "d3dgd")
	if err != nil {
		t.Fatal(err)
	}
	if !ciRaceAbstained(cs) {
		t.Fatalf("d3dgd carries no %s comment after the first still-red pass — the grace pass left no mark, so the next pass cannot tell this episode apart from a fresh race", ciRaceAbstainPrefix)
	}

	// Pass 2: still red, same episode, same closed bead — but the grace
	// already had its one chance to resolve and did not. This is the pass
	// that was silenced forever before the fix.
	n, out, errs := cwRun(t, a, bd)
	if n != 1 {
		t.Fatalf("the second still-red pass over the same episode acted %d, want 1 (a genuinely still-red streak must not stay silenced forever) (stderr %s)", n, errs)
	}
	if !strings.Contains(out, "ci red ·") {
		t.Errorf("the second pass said %q, want a filed-bead line", out)
	}
	if got := cwCount(t, "create"); got != 1 {
		t.Errorf("%d creates after the second still-red pass, want 1", got)
	}

	// Pass 3: the ordinary dedupe (ciOpenBeads) now owns this episode
	// through the bead pass 2 just filed — no third create.
	if n, out, errs := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
		t.Errorf("the third still-red pass acted %d and said %q (stderr %s) — the newly filed bead should dedupe it", n, cwSay(out), errs)
	}
	if got := cwCount(t, "create"); got != 1 {
		t.Errorf("%d creates after the third still-red pass, want 1", got)
	}
}

// The marker earns its place only where two gates share ONE store, which is
// this shop's actual shape: every repo's `.beads` redirects to one queue, so
// the ci-red bead for one gate is in the listing the OTHER gate's dedupe
// reads. Label-only dedupe would let the first red gate silence every other
// one for as long as it stayed red.
//
// Two separate temp stores cannot see this — they are disjoint, and a
// label-only dedupe passes over them — so the fixture is a store that
// already holds another gate's open ci-red bead.
func TestCIWatchFilesEvenWhenAnotherGatesCiRedBeadIsOpen(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	other := CIState{Slug: "someone/else", Workflow: "ci.yml", Branch: "main"}
	seeded := `[{"id":"x-9","title":"ci is red on main: ci.yml is failing in someone/else",` +
		`"status":"open","labels":["` + CIRedLabel + `","` + CIRedLane + `"],` +
		`"description":` + strconv.Quote(ciMarker(other)+"\nci-red streak: 3 consecutive failed run(s)\n") + `}]`
	if err := os.WriteFile(filepath.Join(repo, "fake-list-labeled.json"), []byte(seeded), 0o644); err != nil {
		t.Fatal(err)
	}
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	bd := testBd(t)

	if n, _, errs := cwRun(t, a, bd); n != 1 {
		t.Fatalf("acted %d over a store holding ANOTHER gate's ci-red bead, want 1 (%s)", n, errs)
	}
	// …and this gate's own bead now suppresses the next pass, while the
	// other one still does not answer for it.
	if n, out, _ := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
		t.Errorf("the second pass acted %d and said %q", n, cwSay(out))
	}
	if got := cwCount(t, "create"); got != 1 {
		t.Errorf("%d creates, want 1", got)
	}
}

// The mirror: a bead an EARLIER launcher filed for THIS gate is adopted, so
// a restart mid-red does not file a second one. Same fixture, this gate's
// marker.
func TestCIWatchAdoptsTheBeadAnEarlierLauncherFiled(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	st := redState(9)
	seeded := `[{"id":"x-9","title":"` + st.Title() + `",` +
		`"status":"open","labels":["` + CIRedLabel + `","` + CIRedLane + `"],` +
		`"description":` + strconv.Quote(ciMarker(st)+"\n"+ciStreakPrefix+"9 consecutive failed run(s)\n") + `}]`
	if err := os.WriteFile(filepath.Join(repo, "fake-list-labeled.json"), []byte(seeded), 0o644); err != nil {
		t.Fatal(err)
	}
	a.CIRead = func(CIQuery) CIState { return st }
	bd := testBd(t)

	if n, out, _ := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
		t.Errorf("acted %d and said %q over a bead already filed for this gate", n, cwSay(out))
	}
	if got := cwCount(t, "create"); got != 0 {
		t.Errorf("%d creates, want 0", got)
	}
	// The green pass clears the bead it did not file — the mechanism owns
	// the gate, not the create.
	a.CIRead = func(CIQuery) CIState { return greenState() }
	if n, out, errs := cwRun(t, a, bd); n != 1 || !strings.Contains(out, "said on x-9") {
		t.Errorf("the green pass acted %d and said %q (%s)", n, out, errs)
	}
}

// The launcher lock is taken for the WRITES and never held across the
// network read. One `gh run list` costs 2.8-4.2s here, and a pass that held
// the launcher lock for that on every tick would park the fire loop and
// freeze the cockpit for a reading that usually has nothing to say.
//
// Both arms, because only the pair says anything: an instance whose gates
// all abstain returns while another holder has the lock (it never asked for
// it), and an instance with a red gate does NOT — it waits, which is what
// makes the dedupe safe against a second launcher.
func TestCIWatchTakesTheLockForWritesAndNotForTheReading(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	cwRepo(t, a)
	bd := testBd(t)

	held, err := lockLaunches(a, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)

	a.CIRead = func(CIQuery) CIState { return CIState{Why: "gh is not on this box"} }
	go func() { n, _, _ := cwRun(t, a, bd); done <- n }()
	select {
	case n := <-done:
		if n != 0 {
			t.Errorf("the abstaining pass acted %d", n)
		}
	case <-time.After(5 * time.Second):
		held.Release()
		t.Fatal("an all-abstaining pass waited on the launcher lock — it must never ask for it")
	}

	a.CIRead = func(CIQuery) CIState { return redState(1) }
	go func() { n, _, _ := cwRun(t, a, bd); done <- n }()
	select {
	case n := <-done:
		held.Release()
		t.Fatalf("a pass with a bead to file did NOT wait on the launcher lock (acted %d) — two launchers would double-file", n)
	case <-time.After(300 * time.Millisecond):
	}
	held.Release()
	select {
	case n := <-done:
		if n != 1 {
			t.Errorf("after the lock was dropped the pass acted %d, want 1", n)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the pass never finished after the lock was dropped")
	}
}

// A capped streak is a floor at BOTH ends, and must not be rendered as a
// start date. The window is one gh page (100 runs) and the incident's own
// streak was 191, so at the cap Since is the oldest run still INSIDE the
// window and it moves forward in time as more reds pile up behind it —
// "since 2026-09-02" over a red that began 2026-08-30.
func TestCIStreakLineDoesNotDateACappedStreak(t *testing.T) {
	t.Parallel()
	st := redState(ciScanLimit)
	st.Capped = true
	line := st.streakLine()
	if strings.Contains(line, "since") {
		t.Errorf("a capped streak is rendered as a start date: %q", line)
	}
	for _, want := range []string{strconv.Itoa(ciScanLimit) + "+", "floors", st.Since.Short()} {
		if !strings.Contains(line, want) {
			t.Errorf("capped streak line %q missing %q", line, want)
		}
	}
	if ciLastStreak(line) != ciScanLimit {
		t.Errorf("the drumbeat parser cannot read a capped streak line: %q -> %d", line, ciLastStreak(line))
	}
	// The uncapped line is the ordinary case and DOES name the first red.
	plain := redState(3).streakLine()
	if !strings.Contains(plain, "since "+redState(3).Since.Short()) {
		t.Errorf("uncapped streak line %q does not name the first red", plain)
	}
	if !strings.Contains(st.Description(), "red at least") || strings.Contains(st.Description(), "red since  ") {
		t.Errorf("the capped description still calls it a start:\n%s", st.Description())
	}
}

// The dedupe walks NEWEST FIRST past the beads of episodes nobody has closed
// yet. This is the failure the first cut of the rework had and no other test
// here reaches: ci-watch cannot close (ADR 0013 §4), so a cleared bead sits
// in the listing forever, and a dedupe that adopted the OLDEST match would
// find that cleared bead on every pass of the SECOND episode and file
// another bead each time — one per pass, the exact disaster the mechanism
// exists to avoid, arrived at from the other side.
//
// red → green → red → red → red: three passes into the second episode, two
// beads total.
func TestCIWatchDoesNotRefileWhileAnEarlierEpisodesBeadIsStillOpen(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	cwRepo(t, a)
	bd := testBd(t)

	a.CIRead = func(CIQuery) CIState { return redState(1) }
	if n, _, e := cwRun(t, a, bd); n != 1 {
		t.Fatalf("episode 1: %d (%s)", n, e)
	}
	a.CIRead = func(CIQuery) CIState { return greenState() }
	if n, _, e := cwRun(t, a, bd); n != 1 {
		t.Fatalf("clear: %d (%s)", n, e)
	}
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	if n, _, e := cwRun(t, a, bd); n != 1 {
		t.Fatalf("episode 2: %d (%s)", n, e)
	}
	for i := 0; i < 3; i++ {
		if n, out, _ := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
			t.Fatalf("pass %d of episode 2 acted %d and said %q", i, n, cwSay(out))
		}
	}
	if got := cwCount(t, "create"); got != 2 {
		t.Errorf("%d beads over two episodes and five passes, want 2", got)
	}
	// And the drumbeat is beating on the SECOND bead, not the first.
	a.CIRead = func(CIQuery) CIState { return redState(4) }
	cwRun(t, a, bd)
	if cwCount(t, "comments add q-2") != 1 || cwCount(t, "comments add q-1") != 1 {
		t.Errorf("the drumbeat did not land on the live bead: %v", cwBdCalls(t))
	}
}

// A green pass costs ONE comments read however many cleared beads are
// waiting for somebody to close them. ci-watch cannot close (ADR 0013 §4),
// so that pile is unbounded in principle and a walk over all of it every
// pass would be a bd call per episode per pass, forever.
func TestCIWatchGreenPassDoesNotWalkEveryClearedBead(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	cwRepo(t, a)
	bd := testBd(t)
	// Three episodes, each cleared and none closed — which since
	// ranger-base-4gy4i means each one CLAIMED before its gate went green:
	// the harness closes the bead nobody claimed, so the pile this walk is
	// about is the pile of beads seats hold.
	for i := 0; i < 3; i++ {
		a.CIRead = func(CIQuery) CIState { return redState(1) }
		if n, _, e := cwRun(t, a, bd); n != 1 {
			t.Fatalf("episode %d file: %d (%s)", i, n, e)
		}
		cwHold(t, cwOnlyRepo(t, a), "q-"+strconv.Itoa(i+1), "devops")
		a.CIRead = func(CIQuery) CIState { return greenState() }
		if n, _, e := cwRun(t, a, bd); n != 1 {
			t.Fatalf("episode %d clear: %d (%s)", i, n, e)
		}
	}
	if got := cwCount(t, "close"); got != 0 {
		t.Fatalf("%d closes over three beads seats hold, want 0", got)
	}
	if got := cwCount(t, "create"); got != 3 {
		t.Fatalf("%d beads over three episodes, want 3", got)
	}
	before := cwCount(t, "comments")
	if n, out, _ := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
		t.Fatalf("a green pass over three cleared beads acted %d and said %q", n, cwSay(out))
	}
	if got := cwCount(t, "comments") - before; got != 1 {
		t.Errorf("the green pass made %d comments reads over three cleared beads, want 1", got)
	}
}

// A repo with no CI must produce NO stderr, ever — and one whose gate could
// not be READ must produce exactly one line. Both arms, because either alone
// passes over a mechanism that is uniformly silent or uniformly loud.
//
// The silent arm is not a softening of "an abstention must never render as
// an all-clear": a repo with no gate is not a repo whose gate went unread.
// It was measured — saying it turned 22 dispatch and plan-guard pins red,
// every one of them a clean pass over a temp dir asserting that nothing
// reaches stderr, and every one of them right.
func TestCIWatchIsSilentAboutARepoWithNoGateAndLoudAboutOneItCannotRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		st   CIState
		want int
	}{
		{"no gate here", CIState{Why: "~/x has no .github/workflows/ci.yml", NoGate: true}, 0},
		{"gate unread", CIState{Why: "gh could not list runs (HTTP 401)"}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			a := b.App
			cwRepo(t, a)
			a.CIRead = func(CIQuery) CIState { return c.st }
			bd := testBd(t)
			said := 0
			for i := 0; i < 4; i++ {
				n, out, errs := cwRun(t, a, bd)
				if n != 0 || cwSay(out) != "" {
					t.Fatalf("acted %d and said %q", n, cwSay(out))
				}
				said += strings.Count(errs, "ci-watch:")
			}
			if said != c.want {
				t.Errorf("four passes said it %d times, want %d", said, c.want)
			}
			if got := cwCount(t, "create"); got != 0 {
				t.Errorf("%d creates over an abstention", got)
			}
		})
	}
}
