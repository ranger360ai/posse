//go:build posse_arm3

package posse

// ranger-base-3ys1f, from ranger-base-wjfnp: `posse probe`'s typed-delivery
// arm read agent_prompt_stalled as "the prompt was not delivered" and, by
// setting SettleWhy, skipped awaitProbeTurn — the one reading that would
// have found the canary the turn goes on to write.
//
// Three arms on one fixture, because a one-sided pin here is worthless in
// both directions: a change that fell through for EVERY delivery error
// passes an arm-1-only pin, and a change that went back to stopping on a
// stall passes an arm-3-only one. The fixture differs between the arms by
// the herdr error code and by whether the pane's turn writes the witness
// file — nothing else.
//
// Driven through production RuntimeProbe, not through evalProbe, because the
// defect is the WIRING and not the contract: a pin that composed a
// probeReading with StallWhy set and asked evalProbe about it would have
// stayed green over the arm that never reaches evalProbe at all
// (ranger-base-mmvrh's lesson, which runtimeprobe.go's own
// probeNoAgentWhy doc states for the same reason).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// probeStallHerdr is a herdr that accepts the launch line for real and then
// answers `agent prompt` with the code this arm is about. turnRuns is the
// slow seat: the turn starts after herdr's window closed, runs command 1 of
// the probe prompt and writes the witness file — which is exactly what the
// incident looked like from outside.
//
// It logs every argv it is handed, because "did the probe go on to read the
// pane for the turn" is the actual question and awaitProbeTurn's wait is the
// only `agent wait` in the file that asks for idle and done WITHOUT blocked.
func probeStallHerdr(t *testing.T, srvDir, code, turnRuns string) (Herdr, string) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "argv.log")
	return fakeProbeHerdr(t, `printf '%s\n' "$*" >> `+log+`
case "$1 $2" in
"workspace create") echo '{"id":"f","result":{"workspace":{"workspace_id":"w1"},"root_pane":{"pane_id":"w1:p1"}}}' ;;
"pane run") PATH="`+srvDir+`:$PATH" /bin/sh -c "$4" >/dev/null 2>&1
	echo '{"id":"f","result":{"type":"pane_run"}}' ;;
"agent list") echo '{"id":"f","result":{"agents":[{"agent":"carol","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]}}' ;;
"agent prompt") `+turnRuns+`
	echo '{"id":"f","error":{"code":"`+code+`","message":"agent prompt produced no observed working or blocked state within 5000 ms; current status is idle"}}' ;;
"agent wait") echo '{"id":"f","result":{"agent":{"agent_status":"idle"}}}' ;;
"agent explain") echo '{"id":"f","result":{"state":"idle","matched_rule":{"id":"live_prompt_box"},"visible_idle":true}}' ;;
"pane read") echo "fake pane" ;;
*) echo '{"id":"f","result":{}}' ;;
esac`), log
}

// probeTurnWasRead reports whether awaitProbeTurn ran: its wait is the only
// one in runtimeprobe.go asking for idle and done and not for blocked (the
// promptable wait above it asks for all three), so the argv says which.
func probeTurnWasRead(t *testing.T, log string) bool {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the fake herdr logged no argv at all, so this pin measures nothing: %v", err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "agent wait ") &&
			strings.Contains(l, "--until idle --until done") && !strings.Contains(l, "--until blocked") {
			return true
		}
	}
	return false
}

// obs3 is observable 3 — "unattended-turn", the one a stalled prompt used to
// fail by assumption. Found by name rather than by index, so a reordering of
// evalProbe cannot make this pin read a different observable.
func obs3(t *testing.T, rec *ProbeRecord) ProbeObservable {
	t.Helper()
	for _, o := range rec.Observables {
		if o.Name == "unattended-turn" {
			return o
		}
	}
	t.Fatalf("the record carries no unattended-turn observable: %+v", rec.Observables)
	return ProbeObservable{}
}

func TestQAProbeStalledPromptIsReadForTheTurnAndNotCalledUndelivered(t *testing.T) {
	t.Parallel()
	// The turn the stall says nothing about. `1. ` is the probe prompt's
	// first command — `command -v <canary> > <whereFile> 2>&1` — and running
	// it is what a seat whose first screen change took longer than five
	// seconds does a moment later.
	const theTurnRuns = `sh -c "$(printf '%s\n' "$4" | sed -n 's/^1\. //p')" >/dev/null 2>&1 || :`
	const theTurnRunsNothing = `:`

	for _, tc := range []struct {
		name, code, turn string
		// pass: observable 3 is green, which can only happen if the probe
		// read the pane for the turn.
		pass bool
		// read: the probe reached awaitProbeTurn at all.
		read bool
		// want / never: substrings observable 3's sentence must and must
		// not carry.
		want, never string
		// screen: what Run must print about the stall, or "" when the
		// delivery error is not a stall and nothing may be said about one.
		//
		// The RECORD is not the only place a stall can be lost
		// (ranger-base-4w7rk, verifying this close). On the passing arm
		// observable 3 deliberately says nothing — the turn is its own
		// evidence — so the screen is the ONLY place the fact survives
		// that herdr never saw the turn start, and until this column
		// landed the whole `fmt.Fprintf` could be deleted with all three
		// arms green (MEASURED 2026-10-03).
		screen string
	}{
		{
			// The incident. herdr typed the prompt, the turn started late,
			// and the probe measures it instead of recording a runtime that
			// works as one whose prompt does not land.
			name: "a stall whose turn then arrives is a pass",
			code: "agent_prompt_stalled", turn: theTurnRuns,
			pass: true, read: true,
			never:  "not delivered",
			screen: "saw no turn start inside its own 5s window — reading the pane for the turn anyway",
		},
		{
			// The stall is not forgotten either: a probe that fails anyway
			// owes the operator the fact that herdr never saw the turn
			// start, because the record carries nothing but these sentences.
			name: "a stall whose turn never arrives still fails, saying what herdr did and did not see",
			code: "agent_prompt_stalled", turn: theTurnRunsNothing,
			pass: false, read: true,
			want: "saw no turn start inside its own 5s window", never: "not delivered",
			screen: "saw no turn start inside its own 5s window — reading the pane for the turn anyway",
		},
		{
			// The wrong arm, and the reason the fix is not "fall through on
			// every error". agent_not_ready is herdr declining to address
			// the pane — nothing was sent, there is no turn to wait for, and
			// burning the budget on one would be patience for a prompt that
			// does not exist.
			//
			// The turn is set to RUN here on purpose, which cannot happen in
			// the field: it is what makes `pass: false` evidence rather than
			// a tautology. The witness file is there, so a fall-through
			// would find it and this arm would go green — the verdict can
			// only stay red by not looking.
			name: "agent_not_ready still means nothing was sent",
			code: "agent_not_ready", turn: theTurnRuns,
			pass: false, read: false,
			want: "the prompt was not delivered",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, rt := probeParityApp(t)
			dir := t.TempDir()
			srv := filepath.Join(dir, "srvbin") // the herdr daemon's PATH
			probeFakeCLI(t, srv, probeFixtureExe, "carol 2.0")
			if rt.PromptMode() == PromptArgv {
				t.Fatal("carol must deliver its prompt by typing, or this file measures the wrong arm")
			}

			h, log := probeStallHerdr(t, srv, tc.code, tc.turn)
			var screen bytes.Buffer
			rec, err := a.RuntimeProbe(rt, h, ProbeOpts{Timeout: 2 * time.Second, Out: &screen})
			if err != nil {
				t.Fatalf("the probe could not run: %v", err)
			}

			o := obs3(t, rec)
			if o.OK != tc.pass {
				t.Errorf("observable 3 is %v, want %v — %s", o.OK, tc.pass, o.Detail)
			}
			if tc.want != "" && !strings.Contains(o.Detail, tc.want) {
				t.Errorf("observable 3 must say %q:\n%s", tc.want, o.Detail)
			}
			if tc.never != "" && strings.Contains(o.Detail, tc.never) {
				t.Errorf("observable 3 must not say %q — herdr accepted the submission, so the text WAS typed:\n%s", tc.never, o.Detail)
			}
			if got := probeTurnWasRead(t, log); got != tc.read {
				t.Errorf("the probe read the pane for the turn: %v, want %v (argv log %s)", got, tc.read, log)
			}
			// What the operator watching the probe is told. Asserted both
			// ways: a stall is said out loud on every arm it happened on,
			// and a delivery error that is NOT a stall may not be dressed
			// as one — without that second half the column passes on a
			// line printed unconditionally.
			const stallSaid = "saw no turn start inside its own 5s window"
			switch got := screen.String(); {
			case tc.screen != "" && !strings.Contains(got, tc.screen):
				t.Errorf("the probe printed nothing saying %q — on the passing arm this is the only record left that herdr never saw the turn start:\n%s", tc.screen, got)
			case tc.screen == "" && strings.Contains(got, stallSaid):
				t.Errorf("the probe said herdr stalled on a %s, which is a refusal made before any input was sent:\n%s", tc.code, got)
			}
		})
	}
}
