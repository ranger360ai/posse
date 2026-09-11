package posse

// ranger-base-cz2nw: bd RESOLVES PREFIXES. `bd show <id> --json` exits 0 and
// answers about a DIFFERENT issue when <id> is that issue's prefix — bead
// ids on a store are not fixed width, so one full id being another's prefix
// is a shape a store can hold (measured live, bd 0.50.3: `bd show
// ranger-base-uqsl --json` returns ranger-base-uqslx at rc 0). Bd.Show used
// to return issues[0] unchecked, and every caller then acted on the answer
// as if it were about the id it passed — reapWhy (autoreap.go) decides a
// kill on it and prints the POINTER's id as closed, which the store never
// said.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBdShowRefusesAnAnswerAboutADifferentBead pins Bd.Show itself: the fake
// bd's `show` verb resolves an id its store does not hold onto the one id
// that id PREFIXES (herdr_test.go), which is exactly bd's own
// prefix-resolution shape — a live-store rc=0 answer about an issue other
// than the one named.
//
// The fixture is a real prefix pair (a-1 → a-12) rather than the unrelated
// `other-9` it used to be, because that is the live shape this file's
// header measured (`ranger-base-uqsl` answering `ranger-base-uqslx`), and
// because the fake no longer answers an unrelated id at all
// (ranger-base-s92di).
func TestBdShowRefusesAnAnswerAboutADifferentBead(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-12","status":"closed"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := bd.Show(dir, "a-1")
	if err == nil {
		t.Fatal("an answer about a different id must not be accepted")
	}
	if !strings.Contains(err.Error(), "a-1") || !strings.Contains(err.Error(), "a-12") {
		t.Errorf("the error must name both the id asked for and the id answered, got %q", err.Error())
	}

	// Control: the matching id still passes.
	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-1","status":"closed"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	is, err := bd.Show(dir, "a-1")
	if err != nil {
		t.Fatalf("a matching id must still succeed: %v", err)
	}
	if is.Status != "closed" {
		t.Errorf("got status %q, want closed", is.Status)
	}
}

// TestAutoReapDoesNotKillOnAShowAnswerAboutADifferentBead is the consequence
// at the reap arm (autoreap.go's reapWhy): a session pointed at bead "a-1"
// whose store answers `bd show a-1` about "a-12" (a live prefix collision,
// or a stale/deleted pointer another id now prefixes) must NOT be reaped,
// and the refusal must be said — not silently skipped and not acted on
// under the wrong bead's name.
func TestAutoReapDoesNotKillOnAShowAnswerAboutADifferentBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	dir := reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	// Overwrite the candidate's fake-show.json with a store that holds only
	// a bead the session's own pointer (a-1) PREFIXES, so `bd show a-1`
	// answers about a-12 at rc 0.
	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-12","status":"closed"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	idleClaude(t, fake)

	d.autoReapPass(afterRouting)

	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("a session must not be reaped on an answer about a bead it did not ask about")
	}
	out := dispatcherOut(d)
	if strings.Contains(out, "reaped ranger-repo-a-1") {
		t.Errorf("must not report a reap that never should have happened:\n%s", out)
	}
	if !strings.Contains(out, "ranger-repo-a-1") || !strings.Contains(out, "cannot be read") {
		t.Errorf("the refusal must be said, not silent:\n%s", out)
	}
}
