package posse

// ranger-base-n7lod: the sibling of ranger-base-cz2nw's Bd.Show fix. bd
// resolves prefixes the same way on `bd update --claim` and `bd close`
// (measured live, bd 0.50.3: both answered rc=0 about a bead one character
// longer than the id asked for). Bd.Claim's fast path never compared
// issues[0].ID to id, and Bd.Close discarded its response outright — so
// either could silently claim or close the wrong bead. Both now check the
// id the same way Bd.Show does.
//
// ranger-base-s92di: that check ran AFTER the write, so the wrong bead was
// already closed by the time the error was raised, and these pins asserted
// only that an error came back. The id is now resolved before the mutating
// verb runs (Bd.requireExactID), and the two pins named …LeavesThePrefixed…
// assert the OTHER bead's state and that bd was never asked to touch it.
// The answer-side checks stay, with pins of their own, for the window the
// preflight cannot close.

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
// and Show, asked afterwards, finds the real bead unclaimed — nobody took
// it. The claim must be reported as an error — never as a silent
// (false, nil) win on the wrong bead's say-so.
//
// a-1 is in fake-show.json so the preflight passes and the fast path is
// actually reached; without it this test would stop at the preflight and
// pin nothing about the fast path (ranger-base-s92di).
func TestBdClaimFastPathRefusesAWonAnswerWithNoFallback(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"one","status":"open","assignee":""}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fake-claim-answer.json"),
		[]byte(`[{"id":"other-9","status":"in_progress","assignee":"ranger"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := bd.Claim(dir, "a-1", "ranger")
	if err == nil {
		t.Fatal("a claim fast path answering about a different bead, with no honest fallback, must not silently succeed")
	}
}

// TestBdCloseLeavesThePrefixedBeadOpen is the finding this file was reopened
// for (ranger-base-s92di, finding 2 of the codex outside-in review): the pin
// below used to assert only that an error came back, which a guard that
// closes the wrong bead and THEN complains satisfies just as well as one
// that never closes it. So this one asserts the store.
//
// a-12 exists and is open; a-1 does not exist and is its prefix. The fake
// resolves the prefix on the WRITE the way bd 0.50.3 does, so if Bd.Close
// lets the call through, a-12 really is closed afterwards and this test
// says so.
func TestBdCloseLeavesThePrefixedBeadOpen(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-12","title":"the real bead","status":"open","assignee":"ranger"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := bd.Close(dir, "a-1", ""); err == nil {
		t.Error("closing an absent id that prefixes an existing one must be refused")
	} else if !strings.Contains(err.Error(), "a-1") || !strings.Contains(err.Error(), "a-12") {
		t.Errorf("the error must name both the id asked for and the id it resolved to, got %q", err.Error())
	}

	// The point of the bead: the OTHER bead survived.
	cur, showErr := bd.Show(dir, "a-12")
	if showErr != nil {
		t.Fatalf("reading the prefixed bead back: %v", showErr)
	}
	if cur.Status != "open" {
		t.Errorf("a-12 must still be open after a refused close of a-1, got %q", cur.Status)
	}

	// And the refusal happened BEFORE the write, not after it: no `close`
	// ever reached bd. Checking the status alone would also pass against a
	// guard that closed a-12 and put it back.
	log, readErr := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if readErr != nil {
		t.Fatalf("reading the bd call log: %v", readErr)
	}
	for _, line := range strings.Split(string(log), "\n") {
		if strings.Contains(line, " close ") || strings.HasPrefix(line, "close ") {
			t.Errorf("bd close must never have run; call log has %q", line)
		}
	}
}

// TestBdCloseRefusesAnAnswerAboutADifferentBead pins the SECOND line of
// defence, which the preflight above does not replace: the id resolves
// exactly, the close runs, and bd answers about a different bead anyway —
// a store that gained a colliding id between the two calls, or a bd whose
// resolution moved. That must be reported as an error, not discarded as
// success.
func TestBdCloseRefusesAnAnswerAboutADifferentBead(t *testing.T) {
	t.Parallel()
	newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	// a-1 exists, so the preflight passes and the close actually runs.
	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"one","status":"open"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
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

// TestBdClaimLeavesThePrefixedBeadAlone is Close's sibling for the claim:
// `posse claim <id>` and the cockpit take the id from the operator's hand
// too, and a claim resolved onto a longer id takes a bead somebody else is
// holding. `bd update --claim` must never run at all.
//
// The assertion is the CALL LOG rather than the store, and the two are not
// interchangeable: the store half is pinned on close above, where the fake
// models the write bd's prefix resolution performs. Here what matters is
// that no claim was attempted, so nothing needs to model what one would
// have done. Neither check Fatals, so a regression shows both halves.
func TestBdClaimLeavesThePrefixedBeadAlone(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-12","title":"the real bead","status":"in_progress","assignee":"someone-else"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := bd.Claim(dir, "a-1", "ranger"); err == nil {
		t.Error("claiming an absent id that prefixes an existing one must be refused")
	} else if !strings.Contains(err.Error(), "a-1") || !strings.Contains(err.Error(), "a-12") {
		t.Errorf("the error must name both the id asked for and the id it resolved to, got %q", err.Error())
	}
	log, readErr := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if readErr != nil {
		t.Fatalf("reading the bd call log: %v", readErr)
	}
	if strings.Contains(string(log), "--claim") {
		t.Errorf("bd update --claim must never have run; call log:\n%s", log)
	}
}

// TestBdUnclaimLeavesThePrefixedBeadAlone is the third mutating verb with
// the guard, and it is here because a guard with no reader is a guard that
// gets deleted. `bd update --status open --assignee ""` on a prefix reopens
// and unassigns a bead somebody else is holding — the worst of the three,
// since it is a claim taken AWAY rather than one wrongly granted, and the
// cockpit's unclaim takes the id from a typed line.
func TestBdUnclaimLeavesThePrefixedBeadAlone(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-12","title":"the real bead","status":"in_progress","assignee":"someone-else"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := bd.Unclaim(dir, "a-1", "", false); err == nil {
		t.Error("unclaiming an absent id that prefixes an existing one must be refused")
	} else if !strings.Contains(err.Error(), "a-1") || !strings.Contains(err.Error(), "a-12") {
		t.Errorf("the error must name both the id asked for and the id it resolved to, got %q", err.Error())
	}
	log, readErr := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if readErr != nil {
		t.Fatalf("reading the bd call log: %v", readErr)
	}
	if strings.Contains(string(log), "--status open") {
		t.Errorf("bd update --status open must never have run; call log:\n%s", log)
	}
}
