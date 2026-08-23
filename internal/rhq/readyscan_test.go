package rhq

// The ready scan's failures are facts, not silence (rangerhq-llse). bd
// failing in a repo means that repo's queue is UNKNOWN; folding it into an
// empty result is what let a --watch loop print "0 dispatched" for hours
// while a dozen beads sat ready.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scanRepo is a beads repo serving `ready` from the given JSON, or failing
// the scan outright when ready is "".
func scanRepo(t *testing.T, ready string) string {
	t.Helper()
	dir := t.TempDir()
	if ready == "" {
		os.WriteFile(filepath.Join(dir, "fake-ready-fail"), nil, 0o644)
		return dir
	}
	os.WriteFile(filepath.Join(dir, "fake-ready.json"), []byte(ready), 0o644)
	os.WriteFile(filepath.Join(dir, "fake-show.json"),
		[]byte(`[{"id":"a-1","title":"t","status":"closed"}]`), 0o644)
	return dir
}

func scanConfig(t *testing.T, a *App, dirs ...string) {
	t.Helper()
	cfg := "beads:\n"
	for _, d := range dirs {
		cfg += "  - " + d + "\n"
	}
	os.WriteFile(a.ConfigPath, []byte(cfg), 0o644)
}

// ReadyAll hands its failures back rather than swallowing them, and the
// repos it could read still come through.
func TestReadyAllReportsFailedRepos(t *testing.T) {
	b, _ := newTestBackend(t)
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}

	good := scanRepo(t, `[{"id":"a-1","title":"one"}]`)
	bad := scanRepo(t, "")
	scanConfig(t, b.App, good, bad)

	issues, failed := bd.ReadyAll(b.App, "")
	if len(issues) != 1 || issues[0].ID != "a-1" {
		t.Errorf("the readable repo still reports its work: %+v", issues)
	}
	if len(failed) != 1 {
		t.Fatalf("want the unreadable repo reported once, got %v", failed)
	}
	var se ScanError
	if !errors.As(failed[0], &se) || se.Dir != bad {
		t.Errorf("a scan failure must name its repo: %v", failed[0])
	}
	if !strings.Contains(failed[0].Error(), "database is locked") {
		t.Errorf("a scan failure must carry bd's own words: %v", failed[0])
	}
}

// One repo down out of two: the pass says which, and dispatches the work it
// could actually see. A partial scan is degraded, not fatal.
func TestPassNamesAPartiallyFailedScan(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	good := scanRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`)
	bad := scanRepo(t, "")
	scanConfig(t, b.App, good, bad)
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("one readable repo is enough to run a pass: %v", err)
	}
	if n != 1 {
		t.Errorf("want the readable repo's bead dispatched, got %d\n%s", n, dispatcherOut(d))
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "ready scan failed") || !strings.Contains(out, "database is locked") {
		t.Errorf("the failed repo must be named in the pass output:\n%s", out)
	}
}

// Every repo down: the pass fails loudly instead of printing "no ready
// work" over a scan that never happened. --watch reports the pass error and
// keeps looping — the honest version of what it did silently before.
func TestPassRefusesToCallAFailedScanAnEmptyQueue(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	scanConfig(t, b.App, scanRepo(t, ""), scanRepo(t, ""))

	n, err := d.Run("", "", 0)
	if n != 0 {
		t.Errorf("nothing is dispatchable from a scan that failed: %d", n)
	}
	if err == nil {
		t.Fatalf("a scan that failed everywhere must fail the pass:\n%s", dispatcherOut(d))
	}
	if !strings.Contains(err.Error(), "unknown, not empty") {
		t.Errorf("the pass error must say the queue is unknown: %v", err)
	}
	if strings.Contains(dispatcherOut(d), "no ready work") {
		t.Errorf("a failed scan must never read as an empty queue:\n%s", dispatcherOut(d))
	}
}
