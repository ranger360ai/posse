package posse

// ranger-base-n7lod: the sibling of ranger-base-cz2nw's Bd.Show fix. bd
// resolves prefixes the same way on `bd update --claim` and `bd close`
// (measured live, bd 0.50.3: both answered rc=0 about a bead one character
// longer than the id asked for). Bd.Claim's fast path never compared
// issues[0].ID to id, and Bd.Close discarded its response outright — so
// either could silently claim or close the wrong bead. Both now check the
// id the same way Bd.Show does.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBdClaimFastPathRefusesAWonAnswerAboutADifferentBead pins Bd.Claim:
// a `bd update --claim` answer about an id other than the one asked for
// must not be trusted as a win. It must fall through to Bd.Show instead —
// which here answers correctly about the real id, so the claim still
// resolves (as a resumed claim, since Show reports it already in progress
// under this actor), just not via the untrustworthy fast path.
func TestBdClaimFastPathRefusesAWonAnswerAboutADifferentBead(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fake-claim-answer.json"),
		[]byte(`[{"id":"other-9","status":"in_progress","assignee":"ranger"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-1","status":"in_progress","assignee":"ranger"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	resumed, err := bd.Claim(dir, "a-1", "ranger")
	if err != nil {
		t.Fatalf("a mismatched fast-path answer must fall through to Show, not fail outright: %v", err)
	}
	if !resumed {
		t.Error("Show's honest answer (already in_progress, held by this actor) must read as resumed")
	}
}

// TestBdClaimFastPathRefusesAWonAnswerWithNoFallback is the failure shape
// with nothing to fall back on: the fast path answers about the wrong bead
// and Show cannot find the real one either. The claim must be reported as
// an error — never as a silent (false, nil) win on the wrong bead's say-so.
func TestBdClaimFastPathRefusesAWonAnswerWithNoFallback(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fake-claim-answer.json"),
		[]byte(`[{"id":"other-9","status":"in_progress","assignee":"ranger"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := bd.Claim(dir, "a-1", "ranger")
	if err == nil {
		t.Fatal("a claim fast path answering about a different bead, with no honest fallback, must not silently succeed")
	}
}

// TestBdCloseRefusesAnAnswerAboutADifferentBead pins Bd.Close: a `bd close`
// answer about an id other than the one asked for must be reported as an
// error, not discarded as success.
func TestBdCloseRefusesAnAnswerAboutADifferentBead(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fake-close-answer.json"),
		[]byte(`[{"id":"other-9","status":"closed"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := bd.Close(dir, "a-1", "")
	if err == nil {
		t.Fatal("an answer about a different id must not be accepted")
	}
	if !strings.Contains(err.Error(), "a-1") || !strings.Contains(err.Error(), "other-9") {
		t.Errorf("the error must name both the id asked for and the id answered, got %q", err.Error())
	}

	// Control: the matching id still succeeds.
	if err := os.WriteFile(filepath.Join(dir, "fake-close-answer.json"),
		[]byte(`[{"id":"a-1","status":"closed"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := bd.Close(dir, "a-1", ""); err != nil {
		t.Fatalf("a matching id must still succeed: %v", err)
	}
}
