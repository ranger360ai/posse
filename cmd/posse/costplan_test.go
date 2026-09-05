package main

// `posse cost --plan` (rangerhq-p3z): the plan's own rate windows without
// the transcript scan — the reading a fleet persona asks for through
// Bash(posse:*), and the one the operator's shell script used to fetch by
// hand. Hermetic: the reading comes off a seeded snapshot, so no test here
// touches the keychain or the endpoint.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
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

// armGuard writes a config with a meter threshold in it, which is what
// makes this home one that ASKS the endpoint at all: with no
// `plan_guard_<window>:` set, the plan meter is quiet and no surface —
// cockpit, status or cost — sends a request (planquiet.go,
// ranger-base-4rfw1). Every test below whose subject is the request path or
// the cooldown needs the guard armed to have a subject.
func armGuard(t *testing.T, home string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("plan_guard_5h: 70\n"), 0o644); err != nil {
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
//
// And HOME alone does not hold it. The scan roots at ClaudeConfigDirIn
// (trust.go), which answers $CLAUDE_CONFIG_DIR FIRST and only falls to
// $HOME/.claude when that is unset — and every posse-dispatched seat
// exports it, so on the box where this suite actually runs, os.Environ()
// carried the operator's own ~/.claude straight past the sandbox HOME.
// Measured, one variable apart, before this row existed: 0 live rows in
// 0.06s without it, 29 live rows in 33s with it, and a mutant that dropped
// the footer printed 43 rows of the operator's real attribution into test
// output. The 540x is also the apparatus slowness ranger-base-nmab1's
// bracket was written to survive (ranger-base-t7hgi, from -838su).
// Named, not emptied: ClaudeConfigDirIn treats empty as unset and would
// land on the sandbox HOME anyway, but the claude runtime's own `??`
// treats empty as the empty string, and a row that only works because
// posse and the runtime disagree is a row that reads as a mistake.
func planEnvAt(home, url string) []string {
	return append(os.Environ(),
		"HOME="+filepath.Join(home, "h"),
		"CLAUDE_CONFIG_DIR="+filepath.Join(home, "h", ".claude"),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
		"RHQ_PLAN_USAGE_URL="+url,
	)
}

// The CLAUDE_CONFIG_DIR row above is the whole of the sandbox on the box
// this suite actually runs on, and nothing that USES planEnvAt reds when it
// is deleted — every test here keeps passing, just slowly and against the
// operator's real ledger. So it is pinned here, directly, against a fixture
// ledger instead of the operator's.
//
// The env this test's own process carries names a config dir with one
// planted transcript in it, exactly as a dispatched seat's does. Two arms
// one variable apart:
//
//	RIGHT  planEnv(home)                          -> the canary must not print
//	WRONG  planEnv(home) + CLAUDE_CONFIG_DIR=leak -> the canary MUST print
//
// The wrong arm is not decoration: without it a fixture the scanner quietly
// ignores — a record shape it does not parse, a canary spelled two ways —
// would make the right arm green for the wrong reason, and this pin would
// assert nothing on any box. With it, deleting the row from planEnvAt reds
// the right arm and leaves the wrong one green (ranger-base-t7hgi).
func TestCostSandboxHoldsAgainstAnInheritedClaudeConfigDir(t *testing.T) {
	const canary = "leak-canary-t7hgi"
	bin := buildRhq(t)
	home := t.TempDir()
	leak := plantTranscript(t, canary)

	// What a posse-dispatched seat exports. planEnvAt appends after
	// os.Environ(), and exec keeps the LAST spelling of a name, so the
	// sandbox row is what the child must see.
	t.Setenv("CLAUDE_CONFIG_DIR", leak)

	stdout, stderr, code := runPosse(t, bin, planEnv(home), "cost")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout+stderr, canary) {
		t.Errorf("the scan escaped the sandbox through CLAUDE_CONFIG_DIR and read %s:\n%s", leak, stdout)
	}

	// The control: the same binary, the same fixture, the sandbox row
	// overridden. A green right arm means something only because this one
	// finds the canary.
	stdout, stderr, code = runPosse(t, bin, append(planEnv(home), "CLAUDE_CONFIG_DIR="+leak), "cost")
	if code != 0 {
		t.Fatalf("control arm: exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, canary) {
		t.Fatalf("the control arm found nothing, so the arm above measured nothing: the fixture at %s is not one the scan reads\n%s", leak, stdout)
	}
}

// plantTranscript writes a config dir holding one transcript the cost scan
// segments: the work-prompt line that opens a bead segment (cost.go's
// workPromptRe) and one assistant record with usage, which is what makes
// the segment survive the len(s.Msgs) > 0 filter and reach the report.
func plantTranscript(t *testing.T, bead string) string {
	t.Helper()
	dir := t.TempDir()
	proj := filepath.Join(dir, "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	lines := []string{
		`{"type":"user","timestamp":"` + now + `","message":{"content":"Work beads issue ` + bead + `: fixture"}}`,
		`{"type":"assistant","timestamp":"` + now + `","message":{"id":"m1","model":"claude-opus-5","usage":{"input_tokens":10,"output_tokens":20}}}`,
	}
	if err := os.WriteFile(filepath.Join(proj, "s.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
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
	armGuard(t, home)
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

// agedSeed is how old the seeded reading is at the instant it is written:
// mid-"3m" bucket, so a run that finishes inside 30s renders exactly one
// answer and the bracket in runAgedPlan has exactly one member.
const agedSeed = 3*time.Minute + 30*time.Second

// costFooterNote is what the full report bolts on, and the only thing it
// is allowed to bolt on.
const costFooterNote = " (the plan's own rate limits — the real budget; dollars above are API-equivalent)"

// runAgedPlan seeds the shared snapshot agedSeed old, runs the binary once,
// and returns its stdout together with every age the reading honestly had
// while that process was alive.
//
// The seed happens HERE, per run, rather than once for the whole test: the
// binary's clock starts moving at the write, so bracketing one launch is
// the tightest honest statement the test can make about what age that
// launch could have printed.
func runAgedPlan(t *testing.T, bin, home string, args ...string) (string, []string) {
	t.Helper()
	at := time.Now().Add(-agedSeed)
	seedPlan(t, home, map[string]any{
		"at":      at.UTC().Format(time.RFC3339Nano),
		"windows": []map[string]any{{"name": "5h", "pct": 42}, {"name": "7d", "pct": 61}},
	})
	stdout, stderr, code := runPosse(t, bin, planEnv(home), args...)
	if code != 0 {
		t.Fatalf("posse %s: exit %d, stderr %q", strings.Join(args, " "), code, stderr)
	}
	// BlindFor is minute-resolution below an hour, so the ages this run
	// could have printed are one per whole minute the bracket spans: from
	// agedSeed (the reading's age when the process was launched) to its age
	// once the process had certainly exited.
	var ages []string
	for m := int(agedSeed.Minutes()); m <= int(time.Since(at).Minutes()); m++ {
		ages = append(ages, posse.BlindFor(time.Duration(m)*time.Minute))
	}
	return stdout, ages
}

// oneRendering holds one surface's line to the shared template: the same
// bytes for the same reading, plus whatever that surface bolts on.
func oneRendering(t *testing.T, what, got string, ages []string, note string) {
	t.Helper()
	var want []string
	for _, age := range ages {
		w := fmt.Sprintf("plan windows: 5h 42%% · 7d 61%%, read %s ago%s", age, note)
		if got == w {
			return
		}
		want = append(want, fmt.Sprintf("%q", w))
	}
	t.Errorf("%s is not the one rendering:\n got  %q\n want %s", what, got, strings.Join(want, "\n      or "))
}

// The footer of the full report and `--plan` are one rendering with one
// parenthetical bolted on (PlanCache.Line, rangerhq-p3z) — a persona greps
// the same bytes either way. Pinned by running both commands over an aged
// snapshot and holding each of them to ONE template: two renderings that
// drift apart is the failure this consolidation exists to make impossible,
// and until this test the footer could be deleted outright with the suite
// still green.
//
// The age is the one part of that template the test cannot state outright,
// and it is why this pin used to red the suite under load
// (ranger-base-nmab1). The two commands are two processes with two clocks;
// the old fixture seeded the reading 30s from a bucket edge and compared
// the two outputs to each other, which holds only while the apparatus runs
// faster than 30s — and one measured pair of runs took 48s on a loaded box,
// rendering "3m" and then "4m". A wider margin buys minutes and loses them
// again as the suite grows (2.4x in four days, ranger-base-pj87l), so this
// brackets instead of guessing: each run gets its own fresh seed, and every
// age BlindFor could honestly have printed between that seed and the
// process exiting is an accepted answer. An unloaded box offers one; a
// loaded one offers two, and both are true of the same code. Everything
// else stays exact — a different age format, a fabricated number, a missing
// age, a footer that grew its own copy of any of it, all still fail, and so
// does a missing footer. The age's own formatting is pinned against a
// frozen clock in internal/posse (TestPlanCacheLineSaysHowOldTheReadingIs);
// what is left for the binary to prove is that both surfaces print it.
func TestCostPlanAndTheCostFooterAreOneRendering(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()

	// Aged, not fresh: the age suffix is part of the rendering, and a
	// footer that grew its own copy of it is exactly the drift.
	plan, ages := runAgedPlan(t, bin, home, "cost", "--plan")
	oneRendering(t, "cost --plan", strings.TrimSuffix(plan, "\n"), ages, "")

	full, ages := runAgedPlan(t, bin, home, "cost")
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
	oneRendering(t, "the cost footer", footer, ages, costFooterNote)
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
	armGuard(t, home)
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

// ─── the codex hint (ADR 0034 D3) ────────────────────────────────────────────

// seedCodexHint writes one rollout under the sandbox HOME carrying a
// rate_limits reading, the way codex itself appends it. planEnvAt puts HOME
// at <home>/h, so this is the same directory the binary will look under —
// and no test here can reach the operator's own ~/.codex.
func seedCodexHint(t *testing.T, home string, at time.Time, pct float64, resets time.Time) {
	t.Helper()
	h := filepath.Join(home, "h")
	dir := filepath.Join(h, ".codex", "sessions", at.Format("2006"), at.Format("01"), at.Format("02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"timestamp":"%s","type":"event_msg","payload":{"type":"token_count",`+
		`"rate_limits":{"primary":{"used_percent":%g,"window_minutes":10080,"resets_at":%d},`+
		`"secondary":null,"credits":{"has_credits":false,"unlimited":false,"balance":"0"}}}}`+"\n",
		at.Format(time.RFC3339Nano), pct, resets.Unix())
	name := "rollout-" + at.Format("2006-01-02T15-04-05") + "-fixture.jsonl"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Both meters, in the order the header uses: the guard's reading, then the
// hint — and the hint carries its age unconditionally, which is what makes
// it readable as the snapshot it is. 3m30s sits in the middle of the "3m"
// bucket so the run cannot straddle its edge.
func TestCostPlanPrintsTheCodexHintWithItsAge(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	now := time.Now().UTC()
	seedPlan(t, home, map[string]any{
		"at":      now.Format(time.RFC3339Nano),
		"windows": []map[string]any{{"name": "5h", "pct": 42}, {"name": "7d", "pct": 61}},
	})
	seedCodexHint(t, home, now.Add(-3*time.Minute-30*time.Second), 62, now.Add(24*time.Hour))

	stdout, stderr, code := runPosse(t, bin, planEnv(home), "cost", "--plan")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if want := "plan windows: 5h 42% · 7d 61%\ncodex 7d 62%, as of 3m ago\n"; stdout != want {
		t.Errorf("stdout = %q, want exactly %q", stdout, want)
	}
}

// Past its own resets_at the percent is about a window that has rolled over,
// and it is never printed — the one place this display knows something the
// reading does not, used only to withhold a stale number.
func TestCostPlanShowsAResetCodexWindowAsReset(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	now := time.Now().UTC()
	seedPlan(t, home, map[string]any{
		"at":      now.Format(time.RFC3339Nano),
		"windows": []map[string]any{{"name": "5h", "pct": 42}, {"name": "7d", "pct": 61}},
	})
	seedCodexHint(t, home, now.Add(-3*time.Minute-30*time.Second), 96, now.Add(-time.Minute))

	stdout, stderr, code := runPosse(t, bin, planEnv(home), "cost", "--plan")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "96%") {
		t.Errorf("a window past its reset must not show its percent, got %q", stdout)
	}
	if want := "codex 7d reset, as of 3m ago\n"; !strings.HasSuffix(stdout, want) {
		t.Errorf("stdout = %q, want it to end with %q", stdout, want)
	}
}

// The two readings are independent in both directions. The hint is not
// suppressed by the guard's read failing — it has its own store, and showing
// an operator less than the box knows is the failure this line exists to
// prevent — and it does not rescue the exit status either: --plan's contract
// is the guard's reading, so unreadable is still a failed command.
func TestCostPlanHintSurvivesAnUnreadableGuardReadingWithoutRescuingIt(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	armGuard(t, home)
	now := time.Now().UTC()
	seedPlan(t, home, map[string]any{
		"at":       now.Add(-time.Hour).Format(time.RFC3339Nano),
		"windows":  []map[string]any{{"name": "5h", "pct": 42}, {"name": "7d", "pct": 61}},
		"retry_at": now.Add(30 * time.Minute).Format(time.RFC3339Nano),
	})
	seedCodexHint(t, home, now.Add(-3*time.Minute-30*time.Second), 62, now.Add(24*time.Hour))

	stdout, stderr, code := runPosse(t, bin, planEnv(home), "cost", "--plan")
	if code != 1 {
		t.Fatalf("an unreadable guard reading must still exit 1, got %d (stdout %q, stderr %q)", code, stdout, stderr)
	}
	if want := "codex 7d 62%, as of 3m ago\n"; stdout != want {
		t.Errorf("stdout = %q, want exactly the hint %q", stdout, want)
	}
	if !strings.Contains(stderr, "rate-limited") {
		t.Errorf("stderr must still say why the guard's reading failed: %q", stderr)
	}
}

// The full report is unchanged: ADR 0034 D3 names the header and `--plan`,
// and `posse cost` bare stays the dollars plus the guard's own footer. A
// hint that leaked in here would also break the one-rendering pin above.
func TestCostFooterDoesNotGrowTheCodexHint(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	now := time.Now().UTC()
	seedPlan(t, home, map[string]any{
		"at":      now.Format(time.RFC3339Nano),
		"windows": []map[string]any{{"name": "5h", "pct": 42}, {"name": "7d", "pct": 61}},
	})
	seedCodexHint(t, home, now.Add(-3*time.Minute-30*time.Second), 62, now.Add(24*time.Hour))

	stdout, stderr, code := runPosse(t, bin, planEnv(home), "cost")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "codex 7d") {
		t.Errorf("the full report grew a codex line:\n%s", stdout)
	}
}
