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

// ghRepo builds a checkout ReadCI will accept — a git repo with a github.com
// origin and the workflow file — and points the fake gh's answers at it.
func ghRepo(t *testing.T, workflow string, runs ...string) (dir, ghbin string) {
	t.Helper()
	dir = t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if _, err := git(dir, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	run("init", "-b", "main")
	run("remote", "add", "origin", "https://github.com/ranger360ai/posse.git")
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
	dir := t.TempDir()
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

// Green again: the bead is told which run cleared the gate, it is NOT closed
// (ADR 0013 §4), and nothing is said about it twice.
func TestCIWatchSaysOnItsOwnBeadWhenTheGateIsGreenAgain(t *testing.T) {
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
	if !strings.Contains(out, "said on q-1") {
		t.Errorf("said %q", out)
	}
	// ADR 0013 §4: the harness never closes a bead. The green half is a
	// comment naming the run that cleared the gate, and the persona holding
	// it closes it — TestNoBdCloseVerbReachableFromDispatch enforces the
	// reachability rule that says so, and this is its local half.
	if cwCount(t, "close") != 0 {
		t.Errorf("ci-watch closed a bead: %v", cwBdCalls(t))
	}
	body, _ := os.ReadFile(filepath.Join(repo, "fake-comments.json"))
	if !strings.Contains(string(body), "0c0607b0") || !strings.Contains(string(body), ciClearedPrefix) {
		t.Errorf("the clearing comment does not name the run that cleared it: %s", body)
	}
	// A second green pass says nothing: the bead already carries the
	// clearing comment.
	if n, out, _ := cwRun(t, a, bd); n != 0 || cwSay(out) != "" {
		t.Errorf("a second green pass acted %d and said %q", n, cwSay(out))
	}
	// A red after that green is a NEW episode and gets its own bead, even
	// though the cleared one is STILL OPEN — the bead outlives its episode
	// because the harness cannot close it, so the dedupe has to step over
	// it or the crew never hears about the second red.
	a.CIRead = func(CIQuery) CIState { return redState(1) }
	if n, _, _ := cwRun(t, a, bd); n != 1 {
		t.Errorf("a red following a green filed %d beads, want 1", n)
	}
	if got := cwCount(t, "create"); got != 2 {
		t.Errorf("%d creates over two episodes, want 2", got)
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
	// Three episodes, each cleared and none closed.
	for i := 0; i < 3; i++ {
		a.CIRead = func(CIQuery) CIState { return redState(1) }
		if n, _, e := cwRun(t, a, bd); n != 1 {
			t.Fatalf("episode %d file: %d (%s)", i, n, e)
		}
		a.CIRead = func(CIQuery) CIState { return greenState() }
		if n, _, e := cwRun(t, a, bd); n != 1 {
			t.Fatalf("episode %d clear: %d (%s)", i, n, e)
		}
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
