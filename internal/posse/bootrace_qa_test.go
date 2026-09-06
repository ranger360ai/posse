//go:build posse_arm2

package posse

// QA pins for the rangerhq-3hb5 readiness gate (rangerhq-708f).
//
// WHAT I ATTACKED AND WHAT SURVIVED. The gate's premise is stated in
// awaitSettled: "whatever screen the CLI eventually draws, a rule matched
// it", so a MATCHED RULE is taken as proof the CLI has the screen. Not every
// herdr rule reads the screen — `region = "osc_title"` reads the pane title,
// and codex's osc_title_idle is `regex = ['\S']`, any non-empty title at all.
// A login shell writes a title (zsh here writes user@host:cwd at every prompt
// and the command line at preexec), so on paper a codex pane sitting at a
// shell prompt is a SEEN idle and the gate opens over the shell — 3hb5 with
// extra steps. Measured against herdr 0.8.0 and it does not: herdr scopes the
// detection `osc_title` region to the agent's OWN writes. Through the whole
// boot window the region is empty and the state is the guess; it only fills
// when the CLI sets a title (grok at 0.43s, `osc_title=''` before that). The
// one way to see the shell's title inside detection is `pane report-agent`,
// which nothing in posse calls. TestQALiveGateOpensOnAScreenNotAShellPrompt
// keeps that attack runnable, because it is a herdr behaviour, not ours.
//
// WHAT ESCAPED: rangerhq-lhy2, below.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Seen() is positive evidence — a matched rule, or visible chrome — and NOT
// `fallback_reason == ""`. The gate's own close says so explicitly ("so a
// fallback
// herdr renames tomorrow is still not readiness") and nothing pinned it:
// rewriting Seen as `d.FallbackReason == ""` leaves every bootrace_test.go
// case green, because the fake's guess always names a reason. The shape that
// separates them is a guess herdr does not explain — matched_rule null,
// visible_idle false, fallback_reason null — which is readiness under the
// weaker rule and a guess under this one. `fallback_reason` is nullable in
// the wire shape (a seen state carries null), so this is a shape herdr can
// spell, not an invented one.
func TestQASeenDemandsPositiveEvidenceNotTheAbsenceOfAFallback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		json string
		seen bool
	}{
		{"guess herdr did not explain", `{"state":"idle","matched_rule":null,"visible_idle":false,` +
			`"visible_blocker":false,"visible_working":false,"fallback_reason":null}`, false},
		{"guess under a name herdr has not used yet", `{"state":"idle","matched_rule":null,` +
			`"visible_idle":false,"fallback_reason":"some_other_fallback_2027"}`, false},
		// The other direction: a rule matched but herdr still reported a
		// reason. Positive evidence wins — this is a seen screen.
		{"rule matched despite a reason", `{"state":"idle","matched_rule":{"id":"osc_title_idle"},` +
			`"visible_idle":true,"fallback_reason":"default_known_agent_idle_fallback"}`, true},
	} {
		var det AgentDetection
		if err := json.Unmarshal([]byte(tc.json), &det); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if det.Seen() != tc.seen {
			t.Errorf("%s: Seen()=%v want %v (rule %q visible_idle=%v fallback %q)",
				tc.name, det.Seen(), tc.seen, det.Rule.ID, det.VisibleIdle, det.FallbackReason)
		}
	}
}

// The concession — an explain that cannot be read still launches, out loud —
// pinned against the error shape the fleet actually sends. herdr 0.8.0 puts
// its envelope on STDERR with exit 1 (re-probed 2026-08-22 against a scratch
// server: `agent explain w99:p9 --json` → stdout empty, rc 1, stderr
// {"error":{"code":"agent_not_found",…},"id":"cli:agent:explain"}), and the
// fake's default is that envelope on stdout, a shape the fleet never sends.
// bootrace_test.go's case takes the default; this is the same case with
// error-on-stderr armed (the rangerhq-gnd / rangerhq-625 class).
func TestQAExplainErrorOnStderrStillPromptsOutLoud(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 300 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-error"), []byte("internal|no detection for w1:p1"), 0o644)
	os.WriteFile(filepath.Join(fake, "error-on-stderr"), nil, 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || !strings.Contains(calls(t, fake), "agent prompt") {
		t.Fatalf("a broken `agent explain` must not strand the fleet, got n=%d:\n%s", n, dispatcherOut(d))
	}
	if !strings.Contains(dispatcherOut(d), "herdr cannot explain") {
		t.Errorf("prompting without detection must be said out loud:\n%s", dispatcherOut(d))
	}
}

// rangerhq-lhy2. awaitSettled decides at the deadline on `lastErr`, which
// holds only the MOST RECENT explain, so a pane herdr guessed about for the
// entire window is prompted anyway when the last explain fails. Its own
// comment draws the line the other way — "A guess herdr *did* report is
// different: that is a real answer, it says the screen is unrecognized, and
// it fails loudly" — and twenty-two guesses are twenty-two real answers.
// Production makes the window 22 polls wide (StartupWait 45s, Poll 2s) and
// only the last one has to fail; a herdr restart or live handoff mid-launch
// is how it fails (ranger-base-7t4).
//
// WINDOW SIZING — ranger-base-t1aq, and it governs the twin in
// verify_nx85_qa_test.go too. The fixture witness below needs guesses+1 = 3
// explains INSIDE the window, and at 900ms it did not reliably get them: the
// twin was red about 1 run in 10 on the operator's box with "fixture unmet: 2
// explains". The loop here runs to the deadline by construction — herdr only
// ever guesses, so nothing returns early — which makes the window the test's
// whole duration, and makes it the only wall clock left in the fixture now
// that the late error is armed by call count (ranger-base-9mwa).
//
// Measured 2026-08-30, darwin 25.4.0, 8 cpus, this fixture standalone, load
// manufactured with 16 spinners on top of a box already at 15-40:
//
//	load ~16, 900ms window       15 runs   6-8 explains
//	load ~32-65, 900ms window    12 runs   2 and 4, then 6-7 — and the two
//	                                       starved runs took 13s and 14s of
//	                                       wall for a 900ms window
//	load ~32-65, 3s window       12 runs   17-21 explains
//
// So the worst in-window iteration measured costs ~450ms (2 explains in
// 900ms) against ~150ms idle, and 3 of those is ~1.35s. 4s carries what the
// fixture needs at that worst cost with about 3x over, and buys it at ~3s of
// wall per test. Raising this alone would not have been enough while the
// error was still planted by a timer — that race was the ordering, this one
// is only the margin.
func TestQAGuessesForTheWholeWindowAreLostToOneLateExplainError(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := raceRepo(t, b, fake)
	d.StartupWait = 4 * time.Second
	d.Poll = 100 * time.Millisecond
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644) // guess forever
	os.WriteFile(filepath.Join(fake, "error-on-stderr"), nil, 0o644)  // the real 0.8.0 shape
	// The late failure: the first `guesses` explains answer with the guess,
	// every one after that errors, so the LAST explain of the window is the
	// failed one. Armed by call count rather than by a timer — a timer here
	// raced the launch's own setup and made this test red about 1 run in 3
	// (ranger-base-4pjw); see fakeExplainErrorArmed.
	const guesses = 2
	os.WriteFile(filepath.Join(fake, "explain-error-after"), []byte(strconv.Itoa(guesses)), 0o644)
	os.WriteFile(filepath.Join(fake, "explain-error"), []byte("internal|no detection for w1:p1"), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	// The fixture's own witness. Both halves have to have happened — a
	// window that never got past the guesses would pass this test for the
	// wrong reason, since it asserts an absence.
	if explains := strings.Count(log, "agent explain"); explains <= guesses {
		t.Fatalf("fixture unmet: %d explains in the window, so the late error was never served "+
			"(needs more than %d) — the box is too slow for a %s window at %s polls:\n%s",
			explains, guesses, d.StartupWait, d.Poll, dispatcherOut(d))
	}
	if strings.Contains(log, "agent prompt") {
		t.Fatalf("herdr guessed about this pane for the whole window and posse prompted it anyway, "+
			"because the last explain failed (n=%d):\n%s", n, dispatcherOut(d))
	}
}

// The osc_title attack, kept runnable. It asserts the invariant the gate is
// FOR, not the rule it happens to open on: at the instant the gate returns,
// the pane must not still be showing the launching shell's prompt line.
//
//	herdr --session <s> server &                       # scratch, not the fleet
//	export HERDR_SOCKET_PATH=~/.config/herdr/sessions/<s>/herdr.sock
//	herdr workspace create --cwd <scratch> --label qalive --no-focus
//	RHQ_LIVE_SHELL_PANE=<pane> RHQ_LIVE_CMD='grok --permission-mode auto' \
//	  go test ./internal/rhq -run TestQALiveGateOpensOnAScreen -v
//
// The workspace LABEL must be qalive or the yt1p identity fence leaves the
// meta out of the listing and AgentTarget never resolves. RHQ_LIVE_TITLE
// writes an OSC 2 title to the pane first — the relaunch shape, where a dead
// agent's own title is still on the pane. Nothing is prompted and no Enter is
// ever sent: this costs no turn, which matters because grok's consent banner
// ([Opt in], rangerhq-sz7u) shares that screen.
//
// Measured 2026-08-22, grok 1.0.5: gate open at 0.72s on osc_title_idle,
// screen already grok's. Under old-gate semantics the same pane opened at
// 0.50s with no rule at all.
func TestQALiveGateOpensOnAScreenNotAShellPrompt(t *testing.T) {
	t.Parallel()
	pane := os.Getenv("RHQ_LIVE_SHELL_PANE")
	if pane == "" {
		t.Skip("set RHQ_LIVE_SHELL_PANE=<ws:pane> at a shell (+ HERDR_SOCKET_PATH, RHQ_HERDR_BIN) — see the file comment")
	}
	cmd := os.Getenv("RHQ_LIVE_CMD")
	if cmd == "" {
		cmd = "grok --permission-mode auto"
	}
	b := liveBackend(t, pane)

	if title := os.Getenv("RHQ_LIVE_TITLE"); title != "" {
		if err := b.H.PaneRun(pane, `printf '\033]2;`+title+`\007'`); err != nil {
			t.Fatalf("set title: %v", err)
		}
		time.Sleep(700 * time.Millisecond)
	}
	before, err := b.H.PaneRead(pane, 40)
	if err != nil {
		t.Fatalf("pane read %s: %v (is the pane live and at a shell?)", pane, err)
	}
	shell := qa708LastLine(before)
	t.Logf("the shell's prompt line, which the gate must not open over: %q", shell)

	var out strings.Builder
	d := NewDispatcher(b.App, b, &out)
	d.StartupWait = 30 * time.Second
	d.Poll = 200 * time.Millisecond

	start := time.Now()
	if err := b.H.PaneRun(pane, cmd); err != nil {
		t.Fatalf("pane run %s %q: %v", pane, cmd, err)
	}
	deadline := start.Add(d.StartupWait)

	var target string
	for {
		tg, err := b.AgentTarget("qalive")
		if err == nil {
			target = tg
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no agent detected in %s after %s: %v", pane, d.StartupWait, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	status, det, err := d.awaitSettled("qa-708f", "qalive", target, []string{"idle", "done", "blocked"}, deadline, d.StartupWait)
	opened := time.Since(start)
	// The screen at the instant the gate returned — the thing the work
	// prompt would have gone into. Read it before anything else moves.
	screen, rerr := b.H.PaneRead(pane, 40)
	if rerr != nil {
		t.Fatalf("pane read after the gate: %v", rerr)
	}
	t.Logf("the gate opened after %s on %q: rule=%q visible_idle=%v fallback=%q err=%v\nscreen:\n%s",
		opened, status, det.Rule.ID, det.VisibleIdle, det.FallbackReason, err,
		strings.TrimRight(screen, "\n"))
	if err != nil {
		t.Fatalf("the gate refused a pane that booted normally: %v\n%s", err, out.String())
	}
	if status != "blocked" && !det.Seen() {
		// blocked comes straight off the wait leg and carries no rule by
		// design; every other state the gate opens on must be one herdr saw.
		t.Fatalf("the gate opened on a guess after %s: %q (%s)", opened, status, det.FallbackReason)
	}
	if tail := qa708LastLine(screen); strings.HasPrefix(tail, shell) {
		t.Fatalf("the gate opened after %s on rule %q while the pane's last line is still the shell's prompt %q — "+
			"the work prompt goes to the shell, which is rangerhq-3hb5", opened, det.Rule.ID, shell)
	}
}

func qa708LastLine(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimRight(lines[i], " ")
		}
	}
	return ""
}
