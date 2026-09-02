package posse

// ranger-base-p969: a --no-daemon reader (the whole fleet, since
// bdGlobalFlags carries it on every call — beads.go) can find issues.jsonl
// newer than the database it resolved to and refuse with bd's own
// "Database out of sync with JSONL" rather than importing, even when the
// import that would follow changes nothing (bead evidence: repeated `sync
// --import-only` right after a failure reported 0 created / 0 updated).
// `run` now treats that one message as recoverable — import
// once, retry once — instead of failing the caller outright.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunSelfHealsAStaleDB pins the happy path the bug asked for: a caller
// that would have failed on the stale-db refusal instead gets its answer,
// and gets it by importing and retrying rather than by silently swallowing
// the failure — the fixture only serves the ready list after the sync
// case clears its marker, so a bug that skipped the import straight to a
// blind retry would still read the marker and fail the same way twice.
func TestRunSelfHealsAStaleDB(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := scanRepo(t, `[{"id":"a-1","title":"one"}]`)
	os.WriteFile(filepath.Join(dir, "fake-ready-stale"), nil, 0o644)
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	issues, err := bd.Ready(dir, "")
	if err != nil {
		t.Fatalf("Ready should self-heal past the stale-db refusal, got: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != "a-1" {
		t.Errorf("want the healed repo's own ready list, got %+v", issues)
	}

	log, err := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if err != nil {
		t.Fatalf("the fake bd was never called: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(log)), "\n")
	// The healing sequence is the first three; the fourth is Bd.Ready's own
	// cross-check, which every Ready pays now that `bd ready` is not the
	// definition of unblocked (ranger-base-lpz0o). What is pinned here is
	// still the healing — one import, one retry, never a blind second read
	// of the same refusal — and the tail is named so a Ready that stopped
	// cross-checking fails here too.
	if len(lines) != 4 {
		t.Fatalf("want ready (fails), sync --import-only (heals), ready (retry), blocked (the cross-check) — got %v", lines)
	}
	if !strings.Contains(lines[0], "ready") || !strings.Contains(lines[1], "sync --import-only") ||
		!strings.Contains(lines[2], "ready") || !strings.Contains(lines[3], "blocked") {
		t.Errorf("want [ready, sync --import-only, ready, blocked], got %v", lines)
	}
}

// TestReadyAllSelfHealsPerRepo is the shape the bug actually hit: the
// dispatch pass's aggregated scan, one repo stale and one not. The healthy
// repo must not need healing (no second bd-calls.log entry for it), and the
// stale one must come back in the same result rather than as a ScanError.
func TestReadyAllSelfHealsPerRepo(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}

	healthy := scanRepo(t, `[{"id":"a-1","title":"one"}]`)
	stale := scanRepo(t, `[{"id":"b-1","title":"two"}]`)
	os.WriteFile(filepath.Join(stale, "fake-ready-stale"), nil, 0o644)
	scanConfig(t, b.App, healthy, stale)

	issues, failed := bd.ReadyAll(b.App, "")
	if len(failed) != 0 {
		t.Fatalf("a stale db that heals is not a scan failure: %v", failed)
	}
	ids := map[string]bool{}
	for _, is := range issues {
		ids[is.ID] = true
	}
	if !ids["a-1"] || !ids["b-1"] {
		t.Errorf("want both repos' work, got %+v", issues)
	}
}

// TestRunDoesNotLoopOnAPersistentStaleDB is the safety rail: if the import
// bd actually runs does NOT clear the staleness (the "keep" marker — a real
// problem, not the timestamp/marker false positive this fix targets), `run`
// reports the bd error it actually got back rather than retrying forever or
// inventing a different one.
func TestRunDoesNotLoopOnAPersistentStaleDB(t *testing.T) {
	t.Parallel()
	_, fake := newTestBackend(t)
	bd := Bd{Bin: fakeBinFor(t, "bd")}
	dir := scanRepo(t, `[{"id":"a-1","title":"one"}]`)
	os.WriteFile(filepath.Join(dir, "fake-ready-stale"), []byte("keep"), 0o644)
	os.Remove(filepath.Join(fake, "bd-calls.log"))

	_, err := bd.Ready(dir, "")
	if err == nil {
		t.Fatal("a db still out of sync after the import must still fail the call")
	}
	if !strings.Contains(err.Error(), "Database out of sync with JSONL") {
		t.Errorf("want bd's own words preserved, got: %v", err)
	}

	log, err := os.ReadFile(filepath.Join(fake, "bd-calls.log"))
	if err != nil {
		t.Fatalf("the fake bd was never called: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(log)), "\n")
	if len(lines) != 3 {
		t.Fatalf("want exactly one retry — ready, sync --import-only, ready — not a loop; got %v", lines)
	}
}
