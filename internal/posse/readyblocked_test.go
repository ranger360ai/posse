package posse

// ranger-base-lpz0o — `bd ready` is not the definition of unblocked, so
// Bd.Ready subtracts `bd blocked` and the dispatch pass never fires a bead
// the store itself calls stuck.
//
// The defect these pin, measured 2026-09-01 with one bd 0.50.3 binary
// against two stores: in the store `bd init` writes today (`no-db: true`,
// JSONL only) a `dep add <trigger> <blocker>` over a `discovered-from` edge
// is ACCEPTED and `<trigger>` then comes back from `bd ready` AND from
// `bd blocked` at once; a SQLite `beads.db` refuses the same add outright.
// The block never takes either way — only its loudness varies — so every
// stop built on "the bead leaves bd ready" (the settle-open escalation, the
// SPIKE rung, ASK) was a no-op on the silent half.
//
// These drive the fake, which serves both answers from files, because the
// two-store measurement is not something a hermetic suite can hold. The
// live half is settleescalation_live_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readyBlockedRepo is a fake-bd repo whose `bd ready` answers with ids and
// whose `bd blocked` answers with blocked (bead → blocker) rows.
func readyBlockedRepo(t *testing.T, ready []string, blocked map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	var rows []string
	for _, id := range ready {
		rows = append(rows, `{"id":"`+id+`","title":"`+id+` title","status":"open","labels":["go"]}`)
	}
	write(t, filepath.Join(dir, "fake-ready.json"), "["+strings.Join(rows, ",")+"]")
	if blocked != nil {
		var brows []string
		for id, by := range blocked {
			brows = append(brows, `{"id":"`+id+`","status":"open","blocked_by":["`+by+`"]}`)
		}
		write(t, filepath.Join(dir, "fake-blocked.json"), "["+strings.Join(brows, ",")+"]")
	}
	return dir
}

func readyIDs(t *testing.T, dir string) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	issues, err := Bd{Bin: exe}.Ready(dir, "")
	if err != nil {
		t.Fatalf("Ready: %v", err)
	}
	var ids []string
	for _, is := range issues {
		ids = append(ids, is.ID)
	}
	return ids
}

// THE PIN: a bead both verbs claim is not ready work. Without the
// subtraction this returns both ids and the pass dispatches a-1 forever.
func TestReadyDropsWhatBdBlockedAlsoLists(t *testing.T) {
	newTestBackend(t)
	dir := readyBlockedRepo(t, []string{"a-1", "a-2"}, map[string]string{"a-1": "q-1"})
	if got := readyIDs(t, dir); len(got) != 1 || got[0] != "a-2" {
		t.Fatalf("Ready = %v, want [a-2] — a bead bd blocked lists is not ready work however it got there", got)
	}
}

// CONTROL 1, and the arm that keeps the pin above from passing over a
// filter that drops everything: with nothing blocked, the whole ready set
// survives. `bd blocked` answering `[]` is the ordinary case — it is what
// every repo with no dep edges says — so this is also the shape almost
// every other test in this package runs through.
func TestReadyKeepsEverythingWhenNothingIsBlocked(t *testing.T) {
	newTestBackend(t)
	dir := readyBlockedRepo(t, []string{"a-1", "a-2"}, nil)
	if got := readyIDs(t, dir); len(got) != 2 || got[0] != "a-1" || got[1] != "a-2" {
		t.Fatalf("Ready = %v, want [a-1 a-2]", got)
	}
}

// CONTROL 2: the subtraction is a filter, not a join. `bd blocked` lists
// the whole graph's stuck beads, most of which `bd ready` never offered —
// none of them may appear in the answer, and the order of what is left is
// bd's own (OrderBeads is the pass's, not this method's).
func TestReadyDoesNotInventBeadsBdBlockedNames(t *testing.T) {
	newTestBackend(t)
	dir := readyBlockedRepo(t, []string{"a-2"}, map[string]string{"a-1": "q-1", "a-9": "q-2"})
	if got := readyIDs(t, dir); len(got) != 1 || got[0] != "a-2" {
		t.Fatalf("Ready = %v, want [a-2]", got)
	}
}

// A `bd blocked` that fails makes the repo's queue UNKNOWN, not ready
// (rangerhq-llse). Serving the raw `bd ready` set here would put the silent
// shape back exactly when the store is least trustworthy, and dispatch
// already knows what to do with a scan error: name the repo, and fail the
// pass only when every repo failed.
func TestReadyFailsWhenBlockedCannotBeRead(t *testing.T) {
	newTestBackend(t)
	dir := readyBlockedRepo(t, []string{"a-1"}, nil)
	write(t, filepath.Join(dir, "fake-blocked-fail"), "")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	issues, err := Bd{Bin: exe}.Ready(dir, "")
	if err == nil {
		t.Fatalf("Ready returned %v with no error — an unreadable blocked set is not an empty one", issues)
	}
	if issues != nil {
		t.Errorf("a failed cross-check must return no issues, got %v", issues)
	}
	if !strings.Contains(err.Error(), "database is locked") {
		t.Errorf("the reason bd gave must survive: %v", err)
	}
}

// End to end: the pass itself. A bead in `bd ready` and in `bd blocked` at
// once is not dispatched — which is the whole of the settle-open stop, the
// SPIKE rung and ASK on a store that accepts the cycle instead of refusing
// it.
func TestDispatchDoesNotFireABeadBdBlockedLists(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	idleClaude(t, fake)
	dir := readyBlockedRepo(t, []string{"a-1"}, map[string]string{"a-1": "q-1"})
	write(t, filepath.Join(dir, "fake-show.json"), `[{"id":"a-1","title":"a-1 title","status":"open"}]`)
	write(t, b.App.ConfigPath, "beads:\n  - "+dir+"\n")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("pass: %v\n%s", err, dispatcherOut(d))
	}
	if n != 0 {
		t.Fatalf("dispatched %d — a bead the store calls stuck is not ready work:\n%s", n, dispatcherOut(d))
	}
	if !strings.Contains(dispatcherOut(d), "no ready work") {
		t.Errorf("the pass considered a bead bd blocked lists:\n%s", dispatcherOut(d))
	}
}

// The wrong arm of the one above, and the reason it means anything: the same
// fixture with nothing blocked DOES dispatch a-1. Without this, a pass that
// stopped firing for any other reason — no persona, no idle agent, a repo it
// could not read — would pass the pin above having measured nothing.
func TestDispatchFiresTheSameBeadWhenBdBlockedIsEmpty(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	idleClaude(t, fake)
	dir := readyBlockedRepo(t, []string{"a-1"}, nil)
	write(t, filepath.Join(dir, "fake-show.json"), `[{"id":"a-1","title":"a-1 title","status":"open"}]`)
	write(t, b.App.ConfigPath, "beads:\n  - "+dir+"\n")

	n, err := d.Run("", "", 0)
	if err != nil || n != 1 {
		t.Fatalf("dispatched %d, err=%v — the control fixture must reach a seat:\n%s", n, err, dispatcherOut(d))
	}
}
