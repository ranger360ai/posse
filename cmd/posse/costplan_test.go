package main

// `posse cost --plan` (rangerhq-p3z): the plan's own rate windows without
// the transcript scan — the reading a fleet persona asks for through
// Bash(posse:*), and the one the operator's shell script used to fetch by
// hand. Hermetic: the reading comes off a seeded snapshot, so no test here
// touches the keychain or the endpoint.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseCostFlags(t *testing.T) {
	day := time.Date(2026, 8, 20, 0, 0, 0, 0, time.Local)
	for _, c := range []struct {
		name string
		args []string
		want costOpts
		err  string
	}{
		{name: "bare", args: nil},
		{name: "since a day", args: []string{"--since", "2026-08-20"}, want: costOpts{since: day}},
		{name: "project", args: []string{"--project", "posse"}, want: costOpts{project: "posse"}},
		{name: "plan", args: []string{"--plan"}, want: costOpts{plan: true}},
		{name: "plan with a selector", args: []string{"--plan", "--project", "posse"}, err: "--plan takes no other flags"},
		{name: "selector then plan", args: []string{"--since", "2026-08-20", "--plan"}, err: "--plan takes no other flags"},
		{name: "since needs a value", args: []string{"--since"}, err: "--since needs a date"},
		{name: "since needs a date", args: []string{"--since", "yesterday"}, err: "--since:"},
		{name: "unknown", args: []string{"--plans"}, err: "unknown flag: --plans"},
	} {
		got, err := parseCostFlags(c.args)
		switch {
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%s: got (%+v, %v), want error containing %q", c.name, got, err, c.err)
		case c.err == "" && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.err == "" && got != c.want:
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

// seedPlan writes the shared snapshot every posse process reads
// (internal/posse/plancache.go). Written straight to the state dir rather
// than through the library, so the test is exercising the file format the
// binary actually reads.
func seedPlan(t *testing.T, home string, e map[string]any) {
	t.Helper()
	dir := filepath.Join(home, "state")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan-usage.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// planEnv points the binary at a private RHQ_HOME and at an endpoint that
// cannot answer: any test below that produces a reading proves it came off
// the snapshot, and no request left the machine.
func planEnv(home string) []string {
	return planEnvAt(home, "http://127.0.0.1:1/usage")
}

// planEnvAt is planEnv with the endpoint named, for the one test below that
// wants a listener that answers rather than a port that refuses.
//
// HOME is the sandbox too, not just RHQ_HOME: `posse cost`'s transcript
// scan reads $HOME/.claude/projects, and a test that let it reach the
// operator's own would be measuring a machine instead of a build.
func planEnvAt(home, url string) []string {
	return append(os.Environ(),
		"HOME="+filepath.Join(home, "h"),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
		"RHQ_PLAN_USAGE_URL="+url,
	)
}

func runPosse(t *testing.T, bin string, env []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var o, e strings.Builder
	cmd := exec.Command(bin, args...)
	cmd.Env, cmd.Stdout, cmd.Stderr = env, &o, &e
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("posse %s: %v", strings.Join(args, " "), err)
		}
		code = ee.ExitCode()
	}
	return o.String(), e.String(), code
}

// The whole command: one line, the shared reading, nothing else. No
// transcript scan (that is the point — it is what makes this cheap enough
// to call from a guard), and no request to the endpoint.
func TestCostPlanPrintsTheReadingAndNothingElse(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	seedPlan(t, home, map[string]any{
		"at": time.Now().UTC().Format(time.RFC3339Nano),
		// The snapshot is window-shaped since ADR 0012 D4: the names are the
		// adapter's, and this file only asserts that they come back out.
		"windows": []map[string]any{{"name": "5h", "pct": 42}, {"name": "7d", "pct": 61}},
	})

	stdout, stderr, code := runPosse(t, bin, planEnv(home), "cost", "--plan")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := "plan windows: 5h 42% · 7d 61%\n"; stdout != want {
		t.Errorf("stdout = %q, want exactly %q", stdout, want)
	}
}

// Unlike the footer of the full report, --plan may not be silent: the
// reading IS the output, so an unreadable one is a failed command. Here the
// snapshot is stale and carries a live cooldown, which is the one state
// that fails without asking the endpoint again.
func TestCostPlanFailsLoudWhenTheReadingIsUnavailable(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	now := time.Now().UTC()
	seedPlan(t, home, map[string]any{
		"at":       now.Add(-time.Hour).Format(time.RFC3339Nano),
		"windows":  []map[string]any{{"name": "5h", "pct": 42}, {"name": "7d", "pct": 61}},
		"retry_at": now.Add(30 * time.Minute).Format(time.RFC3339Nano),
	})

	stdout, stderr, code := runPosse(t, bin, planEnv(home), "cost", "--plan")
	if code != 1 {
		t.Fatalf("an unreadable reading must exit 1, got %d (stdout %q, stderr %q)", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("nothing goes to stdout when there is no reading, got %q", stdout)
	}
	if want := "rate-limited"; !strings.Contains(stderr, want) {
		t.Errorf("stderr must say why: got %q, want it to contain %q", stderr, want)
	}
}

// The footer of the full report and `--plan` are one rendering with one
// parenthetical bolted on (PlanCache.Line, rangerhq-p3z) — a persona greps
// the same bytes either way. Pinned by running both commands over one
// snapshot and comparing them: two renderings that drift apart is the
// failure this consolidation exists to make impossible, and until this test
// the footer could be deleted outright with the suite still green.
func TestCostPlanAndTheCostFooterAreOneRendering(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	// Aged, not fresh: the age suffix is part of the rendering, and a
	// footer that grew its own copy of it is exactly the drift. 3m30s sits
	// in the middle of the "3m" bucket, so the two runs below cannot
	// straddle its edge.
	seedPlan(t, home, map[string]any{
		"at":      time.Now().UTC().Add(-3*time.Minute - 30*time.Second).Format(time.RFC3339Nano),
		"windows": []map[string]any{{"name": "5h", "pct": 42}, {"name": "7d", "pct": 61}},
	})

	plan, stderr, code := runPosse(t, bin, planEnv(home), "cost", "--plan")
	if code != 0 {
		t.Fatalf("cost --plan: exit %d, stderr %q", code, stderr)
	}
	plan = strings.TrimSuffix(plan, "\n")
	if !strings.HasSuffix(plan, ", read 3m ago") {
		t.Fatalf("the aged reading must carry its age, got %q", plan)
	}

	full, stderr, code := runPosse(t, bin, planEnv(home), "cost")
	if code != 0 {
		t.Fatalf("cost: exit %d, stderr %q", code, stderr)
	}
	var footer string
	for _, l := range strings.Split(full, "\n") {
		if strings.HasPrefix(l, "plan windows: ") {
			footer = l
			break
		}
	}
	if footer == "" {
		t.Fatalf("the full report still ends with the reading; got:\n%s", full)
	}
	if want := plan + " (the plan's own rate limits — the real budget; dollars above are API-equivalent)"; footer != want {
		t.Errorf("the two renderings drifted:\n footer %q\n --plan %q", footer, want)
	}
}

// The request path, once, through the built binary: no snapshot, a listener
// that answers, and the read log naming who asked. The other tests here
// point at a dead port and so can only prove that nothing left the
// machine — this one proves the leaving works, and that `--plan` is
// "cost" in $StateDir/plan-usage.log, which is the file that settles who
// was hammering the endpoint the next time it 429s.
//
// The log line is also the positive witness for the absence asserted at the
// end: a reading fetched under an RHQ_PLAN_USAGE_URL override may not
// become the instance's shared fact (credpin.go rule 5), and "no snapshot
// was written" only means something once something is known to have run.
func TestCostPlanFetchesThroughTheSeamAndNamesItselfInTheReadLog(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"five_hour":{"utilization":7,"resets_at":"2026-08-28T12:00:00Z"},` +
			`"seven_day":{"utilization":13,"resets_at":"2026-09-01T12:00:00Z"}}`))
	}))
	defer srv.Close()

	stdout, stderr, code := runPosse(t, bin, planEnvAt(home, srv.URL+"/usage"), "cost", "--plan")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := "plan windows: 5h 7% · 7d 13%\n"; stdout != want {
		t.Errorf("stdout = %q, want exactly %q", stdout, want)
	}
	if hits != 1 {
		t.Errorf("one command, one request: got %d", hits)
	}

	b, err := os.ReadFile(filepath.Join(home, "state", "plan-usage.log"))
	if err != nil {
		t.Fatalf("every request that leaves the machine is logged: %v", err)
	}
	line := strings.TrimSpace(string(b))
	if !strings.HasSuffix(line, " cost ok") {
		t.Errorf("the read log must name the caller: got %q, want it to end with %q", line, " cost ok")
	}
	if _, err := os.Stat(filepath.Join(home, "state", "plan-usage.json")); err == nil {
		t.Error("a reading fetched under an endpoint override is nobody's shared fact (credpin.go rule 5), but a snapshot was written")
	}
}
