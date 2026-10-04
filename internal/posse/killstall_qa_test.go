//go:build posse_arm2

package posse

// A stalled landing prompt must not close the workspace on the KILL path
// either — ranger-base-z5lmj, verifying the ranger-base-wjfnp close.
//
// The fix lives in landThePlane, which has two reachable callers, and the
// bead's own census named both: `posse relaunch` (relaunch.go) and
// `posse kill --land` (herdrback.go, KillOpts{Land: true}). The close pinned
// the first. This is the second, and it is the caller where the loss is not
// recoverable: a relaunch that walked through the stall closed the workspace
// and built a replacement, while a kill closes the workspace, removes the
// meta and drops the pane line — so the landing turn being discarded is the
// ORDERS.md lessons and the work-in-progress commit of a session that will
// not exist afterwards to be asked again.
//
// It lives in arm 2 rather than beside the relaunch arm in arm 1 because the
// kill-landing fixture does (memoryRepo/appendOrders, memoryland_qa_test.go):
//
//	go test -tags posse_arm2 -run TestQAStalledKillLandingPrompt ./internal/posse
//
// Same three codes as the relaunch arm, for the same reason — the fix is a
// line about TAXONOMY, so a pin that only watched the stall would pass on
// code that had stopped killing for every reason.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestQAStalledKillLandingPromptDoesNotCloseTheWorkspace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		code      string
		promptErr string // the fake's "code|message" lever
		closes    bool   // what the kill is allowed to do to the workspace
		wantErr   string // a fragment of the refusal; empty for the arm that proceeds
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
			repo := memoryRepo(t, b)
			devSession(t, b, "s1")
			m1, ok := b.readMeta("s1")
			if !ok {
				t.Fatal("no meta for s1")
			}
			// Memory no commit holds, which is what gates the turn at all
			// (killAndLand reads MemoryDirtyPaths before it lands).
			appendOrders(t, repo, "dev", "- a lesson the landing turn is for.\n")
			write(t, filepath.Join(fake, "prompt-error"), tc.promptErr)

			var out strings.Builder
			_, err := b.KillSessionAndLandOpts("s1", KillOpts{Land: true, Out: &out})

			// The control, whichever way the arm goes: the landing prompt
			// was reached. Without it this pin passes on a kill that never
			// took a turn.
			if !strings.Contains(calls(t, fake), "agent prompt") {
				t.Fatalf("%s: the landing prompt was never sent, so nothing here is about it:\n%s", tc.code, calls(t, fake))
			}

			_, hadMeta := b.readMeta("s1")
			closed := strings.Contains(calls(t, fake), "workspace close "+m1.Workspace)
			if closed != tc.closes {
				t.Errorf("%s: workspace closed = %v, want %v\n%s\n%s", tc.code, closed, tc.closes, out.String(), calls(t, fake))
			}

			if tc.closes {
				if err != nil {
					t.Fatalf("%s: a code that means nothing was sent must still kill: %v\n%s", tc.code, err, out.String())
				}
				if hadMeta {
					t.Errorf("%s: a kill that closed the workspace must remove the meta", tc.code)
				}
				return
			}

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("%s: want a refusal naming %q, got %v\n%s", tc.code, tc.wantErr, err, out.String())
			}
			if !hadMeta {
				t.Errorf("%s: a refused kill must leave the session's record where it was", tc.code)
			}
			// The whole point: the turn that was typed is still running in
			// a pane the operator still has.
			if !strings.Contains(err.Error(), "--no-land") {
				t.Errorf("%s: the refusal must name the way through:\n%v", tc.code, err)
			}
		})
	}
}
