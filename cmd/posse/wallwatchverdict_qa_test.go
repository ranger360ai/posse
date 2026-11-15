package main

// QA pins for ranger-base-pfcci: the two READERS of a wall-watch verdict.
//
// THE DEFECT. `posse wall-watch` says its verdict on the pane — three
// lines, the last of which exists for the case where the wall came off AND
// bd could not be written, and says so: "this line is the record". Since
// ranger-base-1l1en every launch line is exec'd, so the watcher IS the
// pane's process: when it returns the block closes and the scrollback goes
// with it. On that double-failure path the only copy of the reason went
// with it too.
//
// The fix writes the same lines to a file beside the seat's backstop
// record, which nothing removes (internal/posse/wallwatch.go, "The verdict
// file"). These are the pins that it is READ back — a record nobody
// surfaces is the same silence one step along, which is
// ranger-base-7idk4's lesson and the reason D3's backstop exists at all.
//
// Both arms plant the file rather than detaching a real wall: the writing
// half is pinned where it happens (internal/posse, TestWallWatch… and the
// live TestLiveWallWatch… rows), and what is in question here is whether
// `posse gates` and `posse peek` show an operator a file that is already
// on disk.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// wwvPlant writes a verdict where a watcher of `persona` would have left
// one: <RHQ_HOME>/state/gates/<persona>/wall-watch/<pid>.verdict.
func wwvPlant(t *testing.T, rhq, persona, pid, text string) string {
	t.Helper()
	dir := filepath.Join(rhq, "state", "gates", persona, "wall-watch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, pid+".verdict")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const wwvText = "── posse wall-watch verdict · 2026-11-15T09:00:00Z · seat builder · dir /w/builder · bwrap pid 10 · seat pid 11\n" +
	"KILLED seat builder: the wall over /h/.claude/settings.json came off at 2026-11-15T09:00:00Z (ranger-base-0qpsw D2)\n" +
	"⚠ the record is incomplete: create: bd timed out — this line is the record\n"

func TestQAGatesReportPrintsThisPersonasWallWatchVerdicts(t *testing.T) {
	bin := buildRhq(t)
	rhq := grwHome(t)
	launchHome := t.TempDir()

	// Nothing on disk: said, not skipped. A report that prints nothing
	// where there is nothing reads the same as one whose reader is broken.
	clean, code := grwRun(t, bin, rhq, launchHome)
	if code != 0 {
		t.Fatalf("`posse gates builder` exited %d:\n%s", code, clean)
	}
	if !strings.Contains(clean, "wall-watch: no verdict on disk") {
		t.Errorf("the report does not say the persona has no verdict:\n%s", clean)
	}

	wwvPlant(t, rhq, "builder", "4411", wwvText)
	// Another persona's, which must not appear in builder's report.
	wwvPlant(t, rhq, "stranger", "4412", strings.ReplaceAll(wwvText, "builder", "stranger"))

	got, code := grwRun(t, bin, rhq, launchHome)
	// NOT part of the verdict: a detach that happened is history, and this
	// command's exit says whether the persona is cleared to dispatch now.
	if code != 0 {
		t.Errorf("a verdict on disk changed `posse gates`'s exit to %d; it judges the PID against the shims, not the past:\n%s", code, got)
	}
	for _, want := range []string{
		"wall-watch: 1 verdict(s)",
		filepath.Join("gates", "builder", "wall-watch", "4411.verdict"),
		"KILLED seat builder: the wall over /h/.claude/settings.json came off",
		"this line is the record",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not carry %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "seat stranger") {
		t.Errorf("builder's report carried another persona's verdict:\n%s", got)
	}
}

// `posse peek` is the command an operator types at a seat that has gone,
// and a killed seat has no pane: before this, "no such session" was all it
// said over a verdict sitting on disk.
func TestQAPeekSaysWhereAKilledSeatsReasonWent(t *testing.T) {
	bin := buildRhq(t)
	rhq := t.TempDir()
	// The same rig as TestPeekLinesRefusesBadCount: a herdr binary that is
	// not there, so Resolve fails and peek takes its lost-pane path.
	env := append(os.Environ(),
		"RHQ_HOME="+rhq,
		"RHQ_HERDR_BIN="+filepath.Join(rhq, "herdr-must-not-run"),
		"PATH=",
	)
	run := func() string {
		t.Helper()
		cmd := exec.Command(bin, "peek", "gone")
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("`posse peek gone` with no herdr must fail:\n%s", out)
		}
		return string(out)
	}

	// With no verdict on disk the error is exactly what it was.
	before := run()
	if strings.Contains(before, "wall-watch") {
		t.Errorf("peek mentioned wall-watch with no verdict on disk:\n%s", before)
	}

	wwvPlant(t, rhq, "builder", "4411", wwvText)
	after := run()
	if !strings.Contains(after, "herdr-must-not-run") {
		t.Errorf("the notice replaced the error instead of being added under it:\n%s", after)
	}
	for _, want := range []string{"1 verdict(s) on disk", filepath.Join("gates", "builder", "wall-watch", "4411.verdict"), "posse gates <persona>"} {
		if !strings.Contains(after, want) {
			t.Errorf("peek does not say %q:\n%s", want, after)
		}
	}
	// No persona to narrow by — the session name resolved to nothing — so
	// the verdict's TEXT stays out of it: peek is also what you type at a
	// name you mistyped, and a stranger's old detach must not bury that.
	if strings.Contains(after, "KILLED seat builder") {
		t.Errorf("peek printed a verdict's text for a session it could not resolve:\n%s", after)
	}
}
