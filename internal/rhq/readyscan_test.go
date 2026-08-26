package rhq

// The ready scan's failures are facts, not silence (rangerhq-llse). bd
// failing in a repo means that repo's queue is UNKNOWN; folding it into an
// empty result is what let a --watch loop print "0 dispatched" for hours
// while a dozen beads sat ready.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// Configured repos that do not exist are failed scans too. Dropping them
// before ReadyAll sees them used to turn an all-missing list into the empty
// string sentinel, which made bd inherit the caller's unrelated cwd.
func TestPassRefusesMissingConfiguredReposInsteadOfScanningCWD(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	root := t.TempDir()
	missingA := filepath.Join(root, "missing-a")
	missingB := filepath.Join(root, "missing-b")
	scanConfig(t, b.App, missingA, missingB)

	n, err := d.Run("", "", 0)
	if n != 0 {
		t.Errorf("nothing is dispatchable from missing repos: %d", n)
	}
	if err == nil || !strings.Contains(err.Error(), "unknown, not empty") {
		t.Fatalf("missing configured repos must fail the pass as an unknown queue: %v\n%s", err, dispatcherOut(d))
	}
	out := dispatcherOut(d)
	for _, path := range []string{missingA, missingB} {
		if !strings.Contains(out, path) {
			t.Errorf("the failed scan must name %s:\n%s", path, out)
		}
	}
	if strings.Contains(out, "no ready work") {
		t.Errorf("missing configured repos must never fall through to cwd:\n%s", out)
	}
}

// A missing entry beside a readable repo must stay visible. Filtering it out
// made a partial queue look complete even though work in one repo was unknown.
func TestReadyAllReportsMissingRepoBesideReadableOne(t *testing.T) {
	b, _ := newTestBackend(t)
	exe, _ := os.Executable()
	bd := Bd{Bin: exe}

	good := scanRepo(t, `[{"id":"a-1","title":"one"}]`)
	missing := filepath.Join(t.TempDir(), "missing")
	scanConfig(t, b.App, good, missing)

	issues, failed := bd.ReadyAll(b.App, "")
	if len(issues) != 1 || issues[0].ID != "a-1" {
		t.Errorf("the readable repo still reports its work: %+v", issues)
	}
	if len(failed) != 1 {
		t.Fatalf("want the missing repo reported once, got %v", failed)
	}
	var se ScanError
	if !errors.As(failed[0], &se) || se.Dir != missing {
		t.Errorf("the missing repo scan must name its configured path: %v", failed[0])
	}
}

// A readable empty repo beside an unreadable one is not "no ready work":
// the queue in the failed repo is unknown, so the pass fails instead of
// reporting the one repo it could see as the whole picture.
func TestPassEmptyReadableRepoBesideFailedScanIsUnknown(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	scanConfig(t, b.App, scanRepo(t, `[]`), scanRepo(t, ""))

	n, err := d.Run("", "", 0)
	if n != 0 {
		t.Errorf("nothing is dispatchable: %d", n)
	}
	if err == nil || !strings.Contains(err.Error(), "unknown, not empty") {
		t.Fatalf("an empty readable repo does not make the failed one empty: %v\n%s", err, dispatcherOut(d))
	}
	if strings.Contains(dispatcherOut(d), "no ready work") {
		t.Errorf("a mixed empty+failed scan must not read as an empty queue:\n%s", dispatcherOut(d))
	}
}

// --watch's job after a scan that failed everywhere: name the pass error
// and keep looping. The pre-llse behaviour was to print "no ready work"
// and look idle.
func TestWatchNamesAFailedScanAndKeepsLooping(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	scanConfig(t, b.App, scanRepo(t, ""), scanRepo(t, ""))

	const wantPasses = 2
	tap := newPassTap(wantPasses)
	d.Out = tap

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-tap.reached:
			cancel()
		case <-ctx.Done():
		}
	}()
	done := make(chan int, 1)
	go func() { p, _ := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 40*time.Millisecond); done <- p }()

	var passes int
	select {
	case passes = <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("watch never returned, though cancel fired on pass %d's header:\n%s", wantPasses, tap.String())
	}
	out := tap.String()
	if passes < wantPasses {
		t.Fatalf("watch must keep looping after a pass error, got %d:\n%s", passes, out)
	}
	if strings.Count(out, "✗ pass failed:") < wantPasses {
		t.Errorf("every failed pass must be named:\n%s", out)
	}
	if !strings.Contains(out, "unknown, not empty") {
		t.Errorf("the pass error must say the queue is unknown:\n%s", out)
	}
	if strings.Contains(out, "no ready work") {
		t.Errorf("a failed scan must never read as an empty queue:\n%s", out)
	}
}

func TestBeadsDirsUsesCWDOnlyWhenKeyIsAbsent(t *testing.T) {
	b, _ := newTestBackend(t)
	if err := os.WriteFile(b.App.ConfigPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if dirs := b.App.BeadsDirs(); len(dirs) != 1 || dirs[0] != "" {
		t.Fatalf("an absent beads key must use cwd, got %q", dirs)
	}

	if err := os.WriteFile(b.App.ConfigPath, []byte("beads: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirs := b.App.BeadsDirs(); len(dirs) != 0 {
		t.Fatalf("an explicitly empty beads list must name no repos, got %q", dirs)
	}
}

// UnresolvedDirs is for the callers that walk BeadsDirs without reaching bd
// (ranger-base-vlrp): they get no chdir failure to learn from, so a path that
// is not there has to be spotted here or not at all. The cwd sentinel always
// resolves, and a file where a repo should be is just as unresolvable as a
// path that does not exist.
func TestUnresolvedDirsNamesOnlyWhatIsNotThere(t *testing.T) {
	good := t.TempDir()
	gone := filepath.Join(t.TempDir(), "projA")
	file := filepath.Join(t.TempDir(), "notarepo")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	failed := UnresolvedDirs([]string{"", good, gone, file})
	if len(failed) != 2 {
		t.Fatalf("want the two unresolvable paths, got %v", failed)
	}
	var se ScanError
	if !errors.As(failed[0], &se) || se.Dir != gone {
		t.Errorf("want a ScanError naming %s, got %v", gone, failed[0])
	}
	if !strings.Contains(failed[0].Error(), "no such directory") {
		t.Errorf("want the missing path said plainly: %v", failed[0])
	}
	if !strings.Contains(failed[1].Error(), "not a directory") {
		t.Errorf("want a file distinguished from a missing path: %v", failed[1])
	}
}
