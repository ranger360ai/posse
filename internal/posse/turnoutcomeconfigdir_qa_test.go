package posse

// The regression pin for ranger-base-r2s9l finding 1, filed PARKED from the
// verify bead ranger-base-onx3x and unparked here by the fix.
//
// WHAT IT PINS. ranger-base-yqdov named TWO readers of claude's transcript
// store as its cost: "Two readers, not one: FindClaudeTurnOutcome goes
// through the same TranscriptFiles, so under an override every dispatched
// turn also read as unobserved." cost.go's transcriptFiles was fixed
// (ClaudeConfigDirIn, cost.go:484) and is pinned by
// TestClaudeTranscriptsFollowClaudeConfigDir. The turn-outcome half was not:
// claudeTranscripts (turnfailure.go) joined home/.claude/projects directly
// and turnfailure.go named ClaudeConfigDirIn nowhere.
//
// The locator is a SEPARATE one on purpose (ranger-base-f09bw: it must name
// one session's own store exactly, where TranscriptFiles substring-matches
// for `posse cost --project`), so the fix is claudeTranscripts asking
// ClaudeConfigDirIn for its ROOT — not claudeTranscripts calling
// TranscriptFiles, which would take the substring match with it.
//
// It did not fire on this box the day it was found — the launch pins
// CLAUDE_CONFIG_DIR to the home's own .claude (credentialdirpin) and the
// operator's managed-settings.json pins it to the same, so the hardcoded
// join coincided with the resolver — which is what this pin is for: it
// costs the moment either pin names a different directory, which is
// precisely what ranger-base-yqdov's bead is about an operator being free
// to do.

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQATurnOutcomeLocatorFollowsClaudeConfigDir(t *testing.T) {

	since := time.Now().Add(-time.Minute)
	at := since.Add(time.Second).UTC().Format(time.RFC3339Nano)

	// A dispatched session's cwd is its worktree, which is the shape the
	// locator is actually asked about (ranger-base-f09bw). Spelled by the
	// test rather than by calling claudeProjectDir, so a mangling that
	// agrees with itself does not pass this by construction.
	const cwd = "/Users/example/.posse/worktrees/posse/ranger-posse-a-1"
	const project = "-Users-example--posse-worktrees-posse-ranger-posse-a-1"

	plant := func(t *testing.T, root string) {
		t.Helper()
		dir := filepath.Join(root, "projects", project)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTranscript(t, dir, "session.jsonl",
			`{"type":"user","timestamp":"`+at+`","message":{"content":"Work beads issue ranger-base-r2s9l: do the work"}}`,
			`{"type":"assistant","timestamp":"`+at+`","message":{"model":"claude-opus-5","content":[{"type":"text","text":"done"}],"usage":{"output_tokens":40}}}`,
		)
	}

	// The control arm. Without it a red below would only say the rig cannot
	// find a transcript anywhere (ranger-base: "probe needs a failing wrong
	// arm" — and its converse, a wrong arm that can pass).
	t.Run("control: the home's own .claude, no override", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		unsetenvForTest(t, "CLAUDE_CONFIG_DIR")
		plant(t, filepath.Join(home, ".claude"))

		if _, observed := FindClaudeTurnOutcome(cwd, "ranger-base-r2s9l", since); !observed {
			t.Fatal("CONTROL FAILED: the reader did not observe a turn in the home's own .claude, so this rig measures nothing about the override")
		}
	})

	// The arm that costs something: $HOME holds no .claude at all, so a
	// locator that ignores the override finds nothing and reports no error —
	// which the settle line prints as "looked for a turn outcome and found
	// none this pass" and which leaves a previous pass's turn-failure marker
	// uncleared.
	t.Run("moved: the store is under $CLAUDE_CONFIG_DIR", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		moved := filepath.Join(t.TempDir(), "claude-elsewhere")
		t.Setenv("CLAUDE_CONFIG_DIR", moved)
		plant(t, moved)

		// The cost reader, which ranger-base-yqdov DID fix, is asked in the
		// same run and about the same store: the two readers of one store
		// disagreeing inside one binary is the fact this pins, not merely
		// "the turn reader came back empty".
		files, errs := claudeCost{}.Transcripts("")
		if len(errs) != 0 || len(files) != 1 {
			t.Fatalf("CONTROL FAILED: the cost locator should list exactly the planted transcript under the override; got %d file(s), errs %v", len(files), errs)
		}

		if _, observed := FindClaudeTurnOutcome(cwd, "ranger-base-r2s9l", since); !observed {
			t.Errorf("FindClaudeTurnOutcome observed nothing under $CLAUDE_CONFIG_DIR=%s, while the cost locator listed the transcript sitting there: posse's two readers of claude's store disagree inside one binary, and ranger-base-yqdov named both", moved)
		}
	})
}
