//go:build posse_arm3

package posse

// The LIVE pin for the transcript ceiling — the measurement itself, kept
// re-runnable rather than transcribed (ranger-base-ac22n):
//
//	RHQ_LIVE_CLAUDE=1 go test -tags posse_arm3 ./internal/posse -run TestLiveClaudeTranscriptCeiling -v
//
// A green suite over the launch-line pins proves the variable is SPELLED on
// the line. It proves nothing about whether the runtime honours it, which is
// the whole claim — and the three levers transcript.go names as losers all
// looked, from the line, exactly as connected as this one. So the arms here
// run the real binary and read the real transcript root.
//
// IT SPENDS NOTHING, and the shape that guarantees it is the control arm's
// own readout. `ANTHROPIC_BASE_URL` points at a dead loopback port, so no
// request leaves the box; the transcript is written at session
// MATERIALIZATION, before the first API call and before auth, which is why
// the control arm still produces a file (MEASURED 2026-11-08, claude 2.1.319:
// both arms printed "Not logged in · Please run /login" and exited 1, and the
// control arm's transcript was there anyway).
//
// WHY `-p` AND NOT AN INTERACTIVE SEAT, said plainly because a dispatched
// seat IS interactive. An interactive arm needs a live credential to reach a
// turn at all: with none, claude opens the login screen and materializes
// nothing, so the arm cannot discriminate (MEASURED 2026-11-08, through a pty
// — no transcript in either arm, which is a rig that would pass over a broken
// fix). A credentialed interactive arm is the operator's to run, and the
// reason it is not needed for THIS claim is the resolver itself: the
// suppression is read by `isTranscriptPersistenceDisabled`, whose four arms
// are a test-env flag, the `--no-session-persistence` flag, this variable and
// a nested-session marker — no print or TTY condition anywhere, and the
// transcript writer's own methods are gated on it (transcript.go quotes it).
//
// IT WRITES INTO THE OPERATOR'S LIVE TRANSCRIPT PILE, which is not a
// shortcut either — it is the finding. This box's root-owned
// managed-settings.json pins CLAUDE_CONFIG_DIR at policy scope, the last
// source in the runtime's settings fold, so there is no scratch transcript
// root to run in: an exported value moves the `.claude.json` and leaves the
// transcript where it was (MEASURED, transcript.go lever 3). The control arm
// therefore lands ONE file in a project directory keyed to this test's own
// temp cwd, and the cleanup removes exactly that directory — matched on this
// test's own name in the mangled key, never on a before/after diff of the
// root, because sibling seats are launching into that root the whole time.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// tcLiveRoot is the transcript root as THE BINARY UNDER TEST resolves it,
// asked with the same environment the arms below run under.
//
// Asked rather than derived, for two reasons that agree. The package's
// TestMain sandboxes HOME, so posse's own resolvers (ClaudeConfigDirIn, which
// cost.go and turnfailure.go join "projects" onto) would answer the test
// home and this pin would measure an empty directory and skip — which is
// what it did before this function asked. And the answer is the one fact
// this whole bead turned on: on a box whose root-owned policy file pins the
// config dir, the root is NOT a function of the environment posse can see
// (transcript.go lever 3), so the only honest resolver is the runtime's own.
// `auth status` runs with no network turn and no trust prompt.
func tcLiveRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("claude", "auth", "status", "--json")
	cmd.Env = os.Environ()
	out, _ := cmd.Output() // exits 1 when not logged in; the JSON is still there
	var m struct {
		ProjectsDirectory string `json:"projectsDirectory"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Skipf("claude auth status --json: unparseable answer (%v) — the transcript root cannot be resolved from the binary, and deriving it from a sandboxed HOME would measure an empty directory:\n%s", err, out)
	}
	if m.ProjectsDirectory == "" {
		t.Skip("claude reports no projectsDirectory (it reads null under a flag-scope CLAUDE_CONFIG_DIR) — nothing here can say where a transcript would land")
	}
	return m.ProjectsDirectory
}

// tcLiveDir is a cwd with no symlink ambiguity left in it, because the
// project key is the cwd STRING mangled character by character: on darwin
// t.TempDir() hands back a /var/... path whose canonical form is
// /private/var/..., and the two mangle to two different keys. Resolving
// first means claudeProjectDir here and the runtime's own mangling agree.
func tcLiveDir(t *testing.T, name string) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// tcLiveRun starts one claude `-p` session with cwd `dir` and `extra` in its
// environment, and reports the transcripts that landed in dir's project
// directory. A non-zero exit is expected and ignored: with a dead endpoint
// the session cannot complete a turn, and the question here is only what it
// left on disk.
func tcLiveRun(t *testing.T, root, dir string, extra ...string) []string {
	t.Helper()
	cmd := exec.Command("claude", "-p", "say hi")
	cmd.Dir = dir
	// A dead loopback port, so the run cannot reach the API and cannot spend.
	cmd.Env = append(append(os.Environ(), "ANTHROPIC_BASE_URL=http://127.0.0.1:1"), extra...)
	out, _ := cmd.CombinedOutput()
	t.Logf("claude -p in %s with %v: %s", dir, extra, strings.TrimSpace(string(out)))

	proj := filepath.Join(root, claudeProjectDir(dir))
	ents, err := os.ReadDir(proj)
	if err != nil {
		return nil // no project directory at all is the strongest "no transcript"
	}
	var got []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			got = append(got, filepath.Join(proj, e.Name()))
		}
	}
	return got
}

// tcLiveCleanup removes the project directory this test caused, and refuses
// to remove anything whose mangled name does not carry this test's own name —
// the root is shared with every live seat on the box and a wrong rm here is
// another session's record.
func tcLiveCleanup(t *testing.T, root, dir string) {
	t.Helper()
	key := claudeProjectDir(dir)
	if !strings.Contains(key, "TestLiveClaudeTranscriptCeiling") {
		t.Fatalf("refusing to clean %q: the mangled key does not name this test, so it may not be this test's directory", key)
	}
	if err := os.RemoveAll(filepath.Join(root, key)); err != nil {
		t.Errorf("cleanup: %v", err)
	}
}

func TestLiveClaudeTranscriptCeilingStopsTheTranscript(t *testing.T) {
	// Parallel-safe: no t.Setenv, and every path it writes is keyed to its
	// own temp cwd, so a sibling test in this binary cannot collide with it
	// in the shared transcript root.
	t.Parallel()
	if os.Getenv("RHQ_LIVE_CLAUDE") == "" {
		t.Skip("set RHQ_LIVE_CLAUDE=1 (shells out to the real claude and writes one file into this box's transcript root)")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("no claude on PATH")
	}
	root := tcLiveRoot(t)
	if _, err := os.Stat(root); err != nil {
		t.Skipf("no transcript root at %s: nothing on this box writes transcripts, so neither arm below means anything", root)
	}

	// ARM 1 — THE CONTROL, and the arm that makes the pin discriminating. If
	// this one comes back empty the rig measured nothing: a run that never
	// writes a transcript cannot show a suppression working.
	control := tcLiveDir(t, "control")
	t.Cleanup(func() { tcLiveCleanup(t, root, control) })
	if got := tcLiveRun(t, root, control); len(got) == 0 {
		t.Fatalf("CONTROL wrote no transcript under %s — this rig cannot see a suppression, so arm 2's green would mean nothing. Has the transcript root moved, or did the session fail before materializing?",
			filepath.Join(root, claudeProjectDir(control)))
	}

	// ARM 2 — the pin. Same binary, same shape of cwd, same dead endpoint;
	// one name added.
	pinned := tcLiveDir(t, "pinned")
	t.Cleanup(func() { tcLiveCleanup(t, root, pinned) })
	if got := tcLiveRun(t, root, pinned, ClaudeTranscriptOffVar+"="+ClaudeTranscriptOffValue); len(got) != 0 {
		t.Errorf("%s=%s left %d transcript(s) on disk: %v\nADR 0050's third channel is open on a repo marked %s: %s.",
			ClaudeTranscriptOffVar, ClaudeTranscriptOffValue, len(got), got, TranscriptsConfigKey, TranscriptsOff)
	}

	// ARM 3 — existing transcripts survive, which is the half the opening
	// bead was most afraid of: the lever it proposed (`cleanupPeriodDays: 0`)
	// was documented as deleting every transcript on the box at startup. The
	// control arm's file is the witness, and it is still there after arm 2
	// ran.
	proj := filepath.Join(root, claudeProjectDir(control))
	ents, err := os.ReadDir(proj)
	if err != nil || len(ents) == 0 {
		t.Errorf("the control arm's transcript directory is gone after the pinned run (%s, err=%v): the suppression deleted an existing record instead of declining to write a new one", proj, err)
	}
}
