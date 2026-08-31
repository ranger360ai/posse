package posse

// Hermetic pins on the live runtime walk itself (ranger-base-nlya).
//
// runtimewalk_live_test.go spends real tokens on a real account. Four
// things about it therefore have to hold without anybody running it:
//
//   - it never runs unasked. A guard that got deleted would put a session
//     launch and a paid turn inside `make test`.
//   - the cell that separates an exhausted ACCOUNT from a broken RUNTIME
//     classifies both directions correctly. That distinction is the one
//     the architect scored as its own dimension, and getting it backwards
//     either direction is what cost the shop a morning on 2026-08-26
//     (ranger-base-nlya's third open question).
//   - the same separation holds for a session that came up and could not
//     LOG IN, which the pre-launch probe cannot see because it runs in a
//     different environment.
//   - the launching persona's own environment does not reach the session
//     under test. A walk that measures a runtime wearing somebody else's
//     gates, deny list and session credentials measures nothing.

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// The grok stderr measured 2026-08-28 on this box, verbatim: exit 1 and
// this text, which is what an exhausted allotment looks like from the
// outside.
const grokExhaustedStderr = `Internal error: {
  "message": "API error (status 402 Payment Required): Grok Build usage balance exhausted",
  "http_status": 402
}
Error: Internal error: {
  "message": "API error (status 402 Payment Required): Grok Build usage balance exhausted",
  "http_status": 402
}`

func TestQAWalkAccountProbeSeparatesAnUnpaidBillFromABrokenRuntime(t *testing.T) {
	for _, c := range []struct {
		name             string
		out              string
		err              error
		alive, exhausted bool
	}{
		{name: "grok's real 402", out: grokExhaustedStderr, err: errors.New("exit status 1"), exhausted: true},
		{name: "codex answered", out: "OK\n", alive: true},
		// The ordering pin. A live account whose ANSWER contains one of
		// the exhaustion words is alive; reading it as exhausted would be
		// the same confusion this cell exists to end, pointed the other
		// way, and it would make the walk refuse to run on a healthy box.
		{name: "a served turn that says quota", out: "OK — your quota is fine\n", alive: true},
		{name: "a served turn that says rate limit", out: "OK (no rate limit hit)\n", alive: true},
		// Neither cleared: this is UNKNOWN(failing), and the walk says so
		// rather than blaming either the account or the CLI.
		{name: "the CLI is not installed", out: "", err: errors.New("exec: \"grok\": executable file not found in $PATH")},
		{name: "exit 0 with no answer", out: "\n"},
		{name: "a crash", out: "panic: runtime error\n", err: errors.New("signal: abort trap")},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := classifyAccount(c.out, c.err)
			if got.alive != c.alive || got.exhausted != c.exhausted {
				t.Errorf("classifyAccount = alive:%v exhausted:%v, want alive:%v exhausted:%v (line %q)",
					got.alive, got.exhausted, c.alive, c.exhausted, got.line)
			}
			if got.line == "" && c.out != "" {
				t.Error("every verdict has to carry the provider's own words as evidence")
			}
		})
	}
}

// The pane line claude printed on 2026-08-28 when the walk launched it
// inside another claude's session env, verbatim from `herdr pane read`.
const claudeExpiredPane = `⏺ Please run /login · API Error: 401 OAuth
  access token has expired. Re-authenticate
  to continue.`

func TestQAWalkReadsAnUnauthenticatedSessionAsTheAccountNotTheRuntime(t *testing.T) {
	if got := walkAuthFailure(claudeExpiredPane); got == "" {
		t.Error("a pane that says the token expired must not be scored as the runtime failing to record")
	}
	// The other direction, which is the one that would make the cell
	// useless: an ordinary settled pane must NOT read as an auth failure,
	// or every record miss becomes UNKNOWN and codex's declared degrade
	// stops being visible at all.
	ordinary := "⏺ Added the comment and closed the bead.\n\n❯\n  ⏵⏵ auto mode on"
	if got := walkAuthFailure(ordinary); got != "" {
		t.Errorf("an ordinary pane read as an auth failure: %q", got)
	}
}

// The launching persona's own environment must not reach the session under
// test. Each name below cost a live run before it was stripped.
func TestQAWalkSessionDoesNotInheritTheLaunchingPersona(t *testing.T) {
	for _, kv := range []string{
		"HERDR_PANE_ID=w1:p1", "CLAUDECODE=1", "CLAUDE_CODE_SESSION_ID=abc",
		"RHQ_PERSONA=some-persona", "RHQ_TOOLS_DENY=Bash(git push:*)", "RHQ_GATES_DIR=/x/gates",
		"RHQ_HOME=/x/home", "RHQ_RUNTIME=claude",
	} {
		if !walkPolluted(kv) {
			t.Errorf("%s would be inherited by the launched session", kv)
		}
	}
	for _, kv := range []string{"HOME=/Users/x", "PATH=/usr/bin", "RHQ_HERDR_BIN=/x/herdr", "RHQ_BD_BIN=/x/bd"} {
		if walkPolluted(kv) {
			t.Errorf("%s is not pollution and must survive", kv)
		}
	}
	got := walkCleanPath("/Users/x/.config/rhq/state/gates/some-persona/bin:/usr/bin:/bin")
	if got != "/usr/bin:/bin" {
		t.Errorf("the launching persona's gate shims survived PATH cleaning: %q", got)
	}
}

// The teardown cell must not cry wolf. This box runs the whole fleet, so a
// bd daemon appearing while the walk runs is usually somebody else's — and
// a cell that blamed the walk for it would be ignored by the third run.
// Classification is by cwd and by cwd alone (ranger-base-42mv's rule).
func TestQAWalkBlamesOnlyTheDaemonsItStarted(t *testing.T) {
	store := "/tmp/fixture/beads"
	cwds := map[string]string{
		"11": "/Users/x/src/ranger-base/.beads", // the canonical queue's
		"22": store,                             // the walk's own
		"33": "",                                // unreadable
	}
	cwdOf := func(pid string) string { return cwds[pid] }

	if v, _, ours := walkClassifyDaemons([]string{"11"}, cwdOf, store); v != "" || ours != "" {
		t.Errorf("another repo's daemon was blamed on the walk: %s (kill %q)", v, ours)
	}
	v, ev, ours := walkClassifyDaemons([]string{"11", "22"}, cwdOf, store)
	if v != walkBroken || ours != "22" {
		t.Errorf("a daemon on the fixture store is the walk's leak: got %s / kill %q (%s)", v, ours, ev)
	}
	if v, _, ours := walkClassifyDaemons([]string{"33"}, cwdOf, store); v != walkUnknown || ours != "" {
		t.Errorf("a daemon nobody can attribute is UNKNOWN and is not killed: got %s / kill %q", v, ours)
	}
	if v, _, _ := walkClassifyDaemons(nil, cwdOf, store); v != "" {
		t.Errorf("no new daemon is no finding: got %s", v)
	}
}

// The walk is the only test in this repo that can spend money, and the one
// line between it and `make test` is its env guard. This pin is a source
// assertion on purpose — the behaviour it wants is "does not run", which a
// test cannot observe by running it — so it names the two halves that have
// to be in the same function and would each be a separate way to lose the
// guard.
func TestQARuntimeWalkNeverRunsUnasked(t *testing.T) {
	src, err := os.ReadFile("runtimewalk_live_test.go")
	if err != nil {
		t.Fatal(err)
	}
	body, ok := funcBody(string(src), "func TestLiveRuntimeContractWalk(t *testing.T) {")
	if !ok {
		t.Fatal("TestLiveRuntimeContractWalk is gone — if it was renamed, this pin has to follow it")
	}
	guard, _, _ := strings.Cut(body, "sheet := newWalkSheet")
	if !strings.Contains(guard, `os.Getenv("RHQ_LIVE_RUNTIME")`) {
		t.Error("the walk must read RHQ_LIVE_RUNTIME before it does anything else")
	}
	if !strings.Contains(guard, "t.Skip") {
		t.Error("an unset RHQ_LIVE_RUNTIME must SKIP: without that, `make test` launches a session and buys a turn")
	}
	// And the guard has to be the first thing, not merely present: a probe
	// placed after the fixture would already have started a herdr server
	// and copied a bd store by the time it fired.
	if i, j := strings.Index(body, "RHQ_LIVE_RUNTIME"), strings.Index(body, "walkFixture("); i < 0 || (j >= 0 && j < i) {
		t.Error("the env guard must come before the fixture is built")
	}
}

// funcBody returns the source between a function's opening line and the
// first line that is exactly "}" — enough to read one top-level function
// without parsing Go.
func funcBody(src, decl string) (string, bool) {
	i := strings.Index(src, decl)
	if i < 0 {
		return "", false
	}
	rest := src[i+len(decl):]
	if j := strings.Index(rest, "\n}\n"); j >= 0 {
		return rest[:j], true
	}
	return rest, true
}
