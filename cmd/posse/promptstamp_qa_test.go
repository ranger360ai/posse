package main

// QA pin for ranger-base-p51zu's second half, on the one verb that half is
// about: `posse prompt` writes the `prompted:` stamp (ranger-base-56gd0,
// verifying that close).
//
// WHY IT NEEDED A PIN OF ITS OWN. The close added two things: G19, a
// governance row for a bead-carrying seat with no stamp (internal/posse,
// four pins), and the stamp write at this verb, which is the measured
// recovery G19's own row text prints as a remedy (`posse prompt <session>`).
// MEASURED 2026-11-23 — with `hb.MarkPrompted(name, time.Now())` deleted
// from `case "prompt"`, `go test ./cmd/posse` came back ok over the whole
// package (103s, and this is the only package that builds that file) and so
// did `make treepins-check` (89.3s, every tree-wide pin). The reader census
// says why: `MarkPrompted` has exactly two call sites, and the launcher's
// (dispatch.go) is pinned by TestRunRecordPersistsBeadAndPrompted while this
// one was pinned by nothing.
//
// Two readers are wrong about the session when the stamp is missing, and
// both are named here rather than in prose alone: `promptedRecently`, the
// guard that stops a pass typing a second instruction into a seat mid-turn —
// so the operator's own prompt was the one prompt it could not see — and
// G19, which reads a bead-carrying seat with no stamp as a seat that never
// got its work.
//
// IT ASSERTS THE PRODUCER'S OWN RENDERING AND NOT A SUBSTRING. writeMeta
// writes `prompted: <RFC3339Nano, UTC>` and parseLaunched reads it back with
// `time.Parse(time.RFC3339)`, returning the zero time on anything else — so
// a stamp that is present but unparseable is a stamp no reader will ever
// see, and `strings.Contains(..., "prompted:")` would pass over it. The
// value is parsed here with the reader's own layout.
//
// AND IT WITNESSES THE WRITE AND NOT THE FIXTURE: the harness's meta is
// asserted stampless BEFORE the verb runs, which is the state the measured
// incident's seat was in.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// promptStampMeta is the fixture's own record, read as text because
// readMeta is internal/posse's and this is package main — the same way
// every other meta assertion in this package reads one.
func promptStampMeta(t *testing.T, home string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, "state", "herdr", "fresh.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// promptStampValue is the `prompted:` value, or "" when there is no such
// key. A key with an empty value is NOT the same as no key and comes back
// as the empty string either way, so the caller checks presence first.
func promptStampValue(meta string) (string, bool) {
	for _, line := range strings.Split(meta, "\n") {
		if v, ok := strings.CutPrefix(line, "prompted:"); ok {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

func TestQAAHandPromptWritesThePromptedStamp(t *testing.T) {
	home, herdr := promptSubmitHerdr(t, "❯\\n")

	// The state the measured seat was in, and the state this fixture starts
	// in: a record with no stamp under it. Asserted, because a fixture that
	// already carried one would make the arm below pass with the write
	// deleted.
	if v, ok := promptStampValue(promptStampMeta(t, home)); ok {
		t.Fatalf("the fixture already carries prompted: %q — this pin's premise", v)
	}

	before := time.Now().Add(-time.Second).UTC()
	out := runPrompt(t, home, herdr)
	after := time.Now().Add(time.Second).UTC()

	// The prompt has to have TAKEN, because the stamp is written after the
	// result for the crew mark's reason: a failed prompt is not a prompt.
	if !strings.Contains(out, "agent_prompted") {
		t.Fatalf("the prompt never reached the occupant, so a missing stamp would be correct:\n%s", out)
	}

	meta := promptStampMeta(t, home)
	v, ok := promptStampValue(meta)
	if !ok {
		t.Fatalf("`posse prompt` left no prompted: stamp — promptedRecently cannot see the operator's own prompt and G19 still reads this seat as never prompted:\n%s", meta)
	}
	// parseLaunched's own layout. Anything it cannot parse is the zero time
	// to every reader, which is indistinguishable from no stamp at all.
	at, err := time.Parse(time.RFC3339, v)
	if err != nil {
		t.Fatalf("prompted: %q does not parse as RFC3339, so parseLaunched reads it as never: %v", v, err)
	}
	if at.Before(before) || at.After(after) {
		t.Errorf("prompted: %s is outside the window this prompt ran in (%s .. %s) — the stamp is a clock reading and a stale one grants PromptGrace to a prompt nobody sent",
			at, before, after)
	}
	// The record is not otherwise rewritten: the fixture's own fields are
	// what a read-modify-write has to preserve, and this call site holds no
	// launcher lock, which is the risk the close accepted out loud.
	for _, want := range []string{"name: fresh", "workspace: w1", "pane: p1"} {
		if !strings.Contains(meta, want) {
			t.Errorf("the stamp write dropped %q from the record:\n%s", want, meta)
		}
	}
}
