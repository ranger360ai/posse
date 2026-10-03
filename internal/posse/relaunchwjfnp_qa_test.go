//go:build !posse_arm2 && !posse_arm3

package posse

// A stalled landing prompt preserves the session — ranger-base-wjfnp.
//
// landThePlane separated `timeout` from every other herdr code by name, and
// agent_prompt_stalled fell on the wrong side of that line: herdr ACCEPTED
// the submission, so the landing prompt WAS typed, and the arm that fired
// printed "landing prompt failed … relaunching anyway" and closed the
// workspace mid-landing-turn — the turn that writes ORDERS.md, commits the
// work in progress and comments the beads. A stall is a slow start, which
// under load is the normal case (promptstall.go, ranger-base-uauvn), so the
// arm most likely to fire was the one that discarded the turn.
//
// Three arms on one fixture, because the fix is a line about TAXONOMY and
// not about relaunching less: a pin that only watched the stall would pass
// on code that had stopped relaunching for every reason.
//
//	timeout               the control — the pre-existing stop, still a stop
//	agent_prompt_stalled  the fix — text sent, turn unobserved: a stop
//	agent_not_ready       herdr refused before sending anything: proceeds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQAStalledLandingPromptDoesNotCloseTheSession(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		code string
		// promptErr is the fake's "code|message" lever, spelled with the
		// message herdr 0.9.1 actually returns for this code.
		promptErr string
		// closes is what the relaunch is allowed to do to the workspace.
		closes bool
		// wantErr is a fragment of the refusal the operator reads. Empty
		// for the arm that proceeds.
		wantErr string
	}{
		{
			code:      "timeout",
			promptErr: "timeout|timed out waiting for agent status",
			closes:    false,
			wantErr:   "still working",
		},
		{
			code:      "agent_prompt_stalled",
			promptErr: "agent_prompt_stalled|herdr: agent prompt produced no observed working or blocked state within 5000 ms; current status is idle",
			closes:    false,
			wantErr:   "no turn start inside its own 5s window",
		},
		{
			code:      "agent_not_ready",
			promptErr: "agent_not_ready|herdr will not address that pane",
			closes:    true,
		},
	} {
		t.Run(tc.code, func(t *testing.T) {
			b, fake := newTestBackend(t)
			agentPerLaunch(t, fake)
			devSession(t, b, "s1")
			m1, ok := b.readMeta("s1")
			if !ok {
				t.Fatal("no meta for s1")
			}
			write(t, filepath.Join(fake, "prompt-error"), tc.promptErr)

			var out strings.Builder
			err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})

			// The control that keeps this pin honest whichever way it
			// goes: the landing prompt was reached at all. Every arm
			// below is about what happens AFTER herdr answered it.
			if !strings.Contains(calls(t, fake), "agent prompt") {
				t.Fatalf("%s: the landing prompt was never sent, so nothing here is about it:\n%s", tc.code, calls(t, fake))
			}

			m2, hadMeta := b.readMeta("s1")
			closed := strings.Contains(calls(t, fake), "workspace close "+m1.Workspace)
			if closed != tc.closes {
				t.Errorf("%s: workspace closed = %v, want %v\n%s\n%s", tc.code, closed, tc.closes, out.String(), calls(t, fake))
			}

			if tc.closes {
				if err != nil {
					t.Fatalf("%s: a code that means nothing was sent must still relaunch: %v\n%s", tc.code, err, out.String())
				}
				if !strings.Contains(out.String(), "landing prompt failed") {
					t.Errorf("%s: a refused submission is said out loud:\n%s", tc.code, out.String())
				}
				if !hadMeta || m2.Workspace == m1.Workspace {
					t.Errorf("%s: the session must be recreated: %+v", tc.code, m2)
				}
				return
			}

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("%s: want a refusal naming %q, got %v\n%s", tc.code, tc.wantErr, err, out.String())
			}
			if !hadMeta || m2.Workspace != m1.Workspace {
				t.Errorf("%s: a refused relaunch must leave the session exactly as it was: %+v (was %+v)", tc.code, m2, m1)
			}
			if strings.Contains(out.String(), "relaunching anyway") {
				t.Errorf("%s: an unobserved turn is not a landing that failed to be submitted:\n%s", tc.code, out.String())
			}
		})
	}
}

// The refusal has to be one an operator can act on, and the stall's is not
// the timeout's: herdr owns the 5000ms window and 0.9.1 has no flag for it
// (MEASURED 2026-10-03 on this box: `herdr agent prompt --help` offers
// --wait, --until and --timeout, and --timeout is the CALLER's patience).
// The caller's own sentence — "still working after 10m0s … or --timeout
// 20m0s" — would be printed within five seconds of the prompt and would
// name a flag that cannot widen anything, so landThePlane says this one
// itself.
func TestQAStalledLandingRefusalOffersNoTimeoutItCannotHonour(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	os.WriteFile(filepath.Join(fake, "prompt-error"),
		[]byte("agent_prompt_stalled|herdr: agent prompt produced no observed working or blocked state within 5000 ms; current status is idle"), 0o644)

	var out strings.Builder
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatalf("a stalled landing prompt must refuse:\n%s", out.String())
	}
	if strings.Contains(err.Error(), "--timeout") {
		t.Errorf("the stall refusal must not offer --timeout — herdr's 5s window is not the caller's patience:\n%v", err)
	}
	for _, want := range []string{"--no-land", "was NOT closed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the stall refusal must name %q:\n%v", want, err)
		}
	}
}
