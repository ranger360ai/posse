//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-lcode: a dispatched launch's own diagnostic lines
// go to the LAUNCHER's stream, never to the process's stderr.
//
// THE DEFECT, MEASURED 2026-10-03 (docs/notes.d/ranger-base-knux2.md).
// planLaunch says everything it has to say through b.warn, whose writer is
// HerdrBackend.Warn else os.Stderr — and Warn was assigned nowhere in
// non-test code, so it was always os.Stderr. The watch loop does not write
// its record through stderr: it opens dispatch-watch.log itself and tees
// d.Out and d.Err into it (watchlog.go). The running loop on this box —
// pid 52273, `posse dispatch --watch 3m ... --resume` — had fd 0, 1 AND 2 on
// /dev/null, so every one of those lines from every dispatched launch was
// discarded.
//
// The cost is the retrospective record. At 00:15:59 the wall reported
// ~/src/posse's prepare-commit-msg stale; at ~00:28:18 the same loop created
// three sessions from that repo, each of which silently re-stamped it; the
// next wall simply showed the repo absent. The line that exists so that does
// not happen "SILENTLY" — "found the L3 prepare-commit-msg wall WRONG before
// this launch just silently re-stamped it" — appears ZERO times in the log
// the loop wrote, and once in the generation an operator had piped by hand
// with `2>&1`. The alarm fired only when somebody happened to redirect
// stderr into the record.
//
// THE FIX. NewSessionOpts.Warn: the launcher hands its own writer down, and
// the dispatcher hands Dispatcher.launchWarns() — the same stream, mutex and
// stamp as the "creating session" line the warning annotates.
//
// Not a gate: the re-stamp itself is correct and wanted, and every arm below
// asserts the launch still healed the wall it reported on.

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// lwsStaleWall: a worktree-capable repo declared in config, with a whole,
// current, marker-bearing PUBLIC prepare-commit-msg render planted where
// config says private — the drift shape launchhookpreheal_qa_test.go pins
// the wording of, and the one the incident was in.
func lwsStaleWall(t *testing.T, b *HerdrBackend, ready, show string) (repo, hook string) {
	t.Helper()
	a := b.App
	repo = wtRepo(t)
	write(t, filepath.Join(repo, "fake-ready.json"), ready)
	write(t, filepath.Join(repo, "fake-show.json"), show)
	write(t, a.ConfigPath, "beads:\n  - "+repo+"\nbeads_visibility:\n  "+repo+": "+VisibilityPrivate+"\n")
	hook = hwsHook(t, repo, "prepare-commit-msg")
	if err := WriteExecutable(hook, []byte(CommitGuardHook(VisibilityPublic, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.BeadsVisibility(repo); v != VisibilityPrivate {
		t.Fatalf("fixture config did not take: visibility is %q", v)
	}
	return repo, hook
}

// The bead's own shape, end to end: a dispatch pass launches into a repo
// whose wall has drifted, and the finding lands in the stream the loop tees
// its record from — not on the stderr that loop has on /dev/null.
func TestQADispatchLaunchWarningsLandInTheLoopsRecordNotOnStderr(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo, hook := lwsStaleWall(t, b, `[{"id":"g-1","title":"t","labels":["go"]}]`, `[{"id":"g-1","status":"closed"}]`)
	agentPerLaunch(t, fake)

	// b.Warn is where the backend writes when nobody hands it a writer, and
	// in production that is os.Stderr — the /dev/null the incident was. A
	// line in here is a line the loop's record would not have.
	stderrish := warnBuf(t, b)

	n, _ := d.Run("", "", 0)
	out := dispatcherOut(d)
	if n != 1 {
		t.Fatalf("the pass launched nothing, so this test asserts nothing: n=%d\n%s", n, out)
	}
	if !strings.Contains(out, "WRONG before this launch") {
		t.Errorf("the pre-heal finding is not in the pass's own stream, which is the only thing the watch log tees:\n%s", out)
	}
	if !strings.Contains(out, hook) {
		t.Errorf("the finding in the pass's stream does not name the file to fix (%s):\n%s", hook, out)
	}
	if got := stderrish.String(); strings.Contains(got, "WRONG before this launch") {
		t.Errorf("the finding went to the backend's writer — os.Stderr in production, /dev/null under the loop:\n%s", got)
	}
	// The scope note on the bead, asserted: this is about the record, not
	// about a gate. The wall is still healed.
	if post := b.App.probeL3Hooks(repo, false); !post.CommitGuard {
		t.Errorf("the dispatched launch did not re-stamp the wall it reported: %s", post.CommitGuardDegraded)
	}
}

// The other line the bead names, on the same stream and from the same
// launch: `--allow-degraded`'s "launches DEGRADED". It is a second b.warn
// site in planLaunch, so it is the arm that fails if the fix is made at one
// call site instead of at the launch's writer.
func TestQADispatchDegradedLaunchLineLandsInTheLoopsRecord(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.AllowDegraded = true
	write(t, filepath.Join(b.App.AgentsDir, "security.md"),
		"---\nname: security\nlabels: [security]\ndeny: [Edit, Write, Bash(git push:*)]\n---\nYou are security.\n")
	lwsStaleWall(t, b, `[{"id":"s-1","title":"t","labels":["security"]}]`, `[{"id":"s-1","status":"closed"}]`)
	agentPerLaunch(t, fake)

	stderrish := warnBuf(t, b)
	n, _ := d.Run("", "", 0)
	out := dispatcherOut(d)
	if n != 1 {
		t.Fatalf("--allow-degraded must launch: n=%d\n%s", n, out)
	}
	if !strings.Contains(out, "launches DEGRADED") {
		t.Errorf("the DEGRADED line is not in the pass's own stream:\n%s", out)
	}
	if got := stderrish.String(); strings.Contains(got, "launches DEGRADED") {
		t.Errorf("the DEGRADED line went to the backend's writer, which the loop does not read:\n%s", got)
	}
}

// The seam itself, under the launch path rather than under a pass: a launcher
// that hands a writer is the one that gets the lines, and a launcher that
// hands none still gets the backend's — which is what keeps `posse new`'s
// warnings on the terminal the operator typed at.
func TestQALaunchWarningsFollowTheWriterTheLauncherHandsDown(t *testing.T) {
	t.Parallel()
	b, repo := lhpWorktreeFixture(t, VisibilityPrivate)
	a := b.App
	hook := hwsHook(t, repo, "prepare-commit-msg")
	if err := WriteExecutable(hook, []byte(CommitGuardHook(VisibilityPublic, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}
	backend := warnBuf(t, b)

	var mine strings.Builder
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger", Worktree: true, Warn: &mine}); err != nil {
		t.Fatalf("launch was refused rather than self-healed: %v", err)
	}
	if got := mine.String(); !strings.Contains(got, "WRONG before this launch") {
		t.Fatalf("the launcher's own writer did not get the finding:\n%s", got)
	}
	if got := backend.String(); strings.Contains(got, "WRONG") {
		t.Errorf("a launch that named its writer still wrote the backend's:\n%s", got)
	}

	// The control, and the launch `posse new` makes: no writer named, so the
	// backend's answers — os.Stderr in production, the operator's terminal.
	if err := WriteExecutable(hook, []byte(CommitGuardHook(VisibilityPublic, a.OpsPatternSet())), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: repo, Agent: "ranger", Worktree: true}); err != nil {
		t.Fatalf("control launch was refused: %v", err)
	}
	if got := backend.String(); !strings.Contains(got, "WRONG before this launch") {
		t.Errorf("a launch that named no writer must still reach the backend's:\n%s", got)
	}
}

// launchWarns is a dispatcher writer and has to behave like one: through
// d.printf, so a launch line cannot land halfway through a gather's (ADR
// 0028 §1) and the watchdog counts it as a sign of life (LastWrite) — and
// through Progress when the caller's Out cannot carry a line at all, which
// is the cockpit, whose Out is io.Discard.
func TestQALaunchWarnsStampTheLoopAndReachACockpitThroughProgress(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)

	if !d.LastWrite().IsZero() {
		t.Fatalf("premise: a fresh dispatcher has already written at %v", d.LastWrite())
	}
	if _, err := io.WriteString(d.launchWarns(), "posse: s1 launches DEGRADED\n"); err != nil {
		t.Fatal(err)
	}
	if got := dispatcherOut(d); !strings.Contains(got, "launches DEGRADED") {
		t.Errorf("a launch warning did not reach Out:\n%s", got)
	}
	if d.LastWrite().IsZero() {
		t.Error("a launch warning did not stamp LastWrite — a launch IS the pass, and the watchdog reads silence")
	}

	// The cockpit's shape: Out discarded, Progress set after the dispatcher
	// was built — so the writer has to read both at write time.
	var lines []string
	d2 := newTestDispatcher(t, b)
	d2.Progress = func(line string) { lines = append(lines, line) }
	if _, err := io.WriteString(d2.launchWarns(), "posse: s1 launches DEGRADED\n"); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "launches DEGRADED") {
		t.Errorf("a launch warning did not reach Progress as one line: %q", lines)
	}
	if got := dispatcherOut(d2); got != "" {
		t.Errorf("Progress is set because Out cannot carry the line; it was written anyway:\n%s", got)
	}
}
