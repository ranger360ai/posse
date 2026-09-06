//go:build posse_arm3

package posse

// QA pins for what scripts/queue-cutover.sh's abort trap tells the operator
// to DO, from the verify pass on ranger-base-nzyn (ranger-base-5gnb).
//
// queuecutover_qa_test.go pins the abort message's TEXT. These two run it.
// The distinction is the whole finding: a recovery instruction that is
// present, quoted, and wrong reads exactly like one that works.
//
// ─── ranger-base-iycc: the move-window UNDO overwrites the live store ───────
//
// For stage `move` the trap prints (queue-cutover.sh:150-154) a loop that
// walks every entry of $DST_BEADS — the dotfile glob included — back into
// $SRC_BEADS, then drops the queue repo. But $DST_BEADS does not hold only
// what the mv loop moved: step 2 replays the constitution's `.beads` history
// into the queue repo and checks it out, so before the loop starts
// $DST_BEADS ALREADY holds the last COMMITTED projection (issues.jsonl,
// deleted.jsonl, .gitignore). An abort partway through the loop leaves some
// live files moved and some at home, and the printed UNDO then walks the
// replayed copies home ON TOP of the ones that never left. The live
// projection is replaced by the last commit, every uncommitted bead in it is
// gone, and the UNDO exits 0 saying nothing.
//
// The shipped move-window pin cannot see this. It blocks the move by making
// the store directory mode 0500, so the FIRST rename fails and nothing is
// ever split — $DST_BEADS is still exactly the replay — and it asserts the
// message's text without running it. Two gaps, and each hides the other:
// that fixture cannot produce a partial move, and that assertion cannot
// notice a wrong recovery. The fixture here uses an `mv` that works twice
// and then refuses, which splits the store for real.
//
// Scope: stage `move` only. Stage `redirect` runs after the loop finished,
// so by then $DST_BEADS IS the live store and the same loop is right; the
// runbook's Rollback block (the same loop, documented as such) is likewise
// only used after a COMPLETE cutover.
//
// Fix directions, none of them chosen here: move back only what the loop
// moved (a list written as it goes); or empty $DST_BEADS of the replayed
// projection before the loop starts, so it holds nothing but moved files;
// or have the UNDO refuse to overwrite a file already at home and name the
// ones it skipped.
//
// FIXED, the second way: the queue's working tree is emptied (the index
// kept) between the replay's checkout and the move loop, so $DST_BEADS holds
// nothing but what the loop moved and the UNDO's assumption is true rather
// than merely documented. That keeps the same two-line UNDO right in stage
// move, in stage redirect and in the runbook's Rollback, where the other two
// directions would have had the three diverge. Two consequences, both
// measured: the success path no longer resurrects a file tracked at the last
// commit but deleted from the live store (pinned as
// TestQueueCutoverDoesNotResurrectAFileTheLiveStoreDeleted), and the nesting
// below stops happening, because `mv` only renames a directory INTO another
// when one of the same name is already there.
//
// ─── ranger-base-8izk: a tracked subdirectory nests instead of aborting ─────
//
// The nzyn close recorded, as adjacent and unreachable, that a tracked
// subdirectory under `.beads` would abort the move with "Directory not
// empty" and that the new trap would at least say so. Measured: BSD `mv`
// renames a directory INTO an existing one of the same name, so the live
// subdirectory lands a level deeper, the replayed stale copy keeps the real
// path, and the script exits 0 reporting a clean cutover.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// qcFlakyMv returns a PATH entry holding an `mv` that execs the real one
// `ok` times and then exits 1 — a move window that fails partway through,
// with no cage, no hook and no unwritable directory involved.
func qcFlakyMv(t *testing.T, ok string) string {
	t.Helper()
	real, err := exec.LookPath("mv")
	if err != nil {
		t.Skipf("no mv on PATH: %v", err)
	}
	dir := t.TempDir()
	n := filepath.Join(dir, "calls")
	p := filepath.Join(dir, "mv")
	write(t, p, "#!/bin/sh\n"+
		"c=$(cat '"+n+"' 2>/dev/null || echo 0)\n"+
		"c=$((c+1)); printf '%s' \"$c\" > '"+n+"'\n"+
		"[ \"$c\" -gt "+ok+" ] && { echo \"mv: refused (call $c)\" >&2; exit 1; }\n"+
		"exec '"+real+"' \"$@\"\n")
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// qcUndoBlock is the indented command block the trap prints under "UNDO",
// up to the first line that is not part of it.
func qcUndoBlock(t *testing.T, out string) string {
	t.Helper()
	lines := strings.Split(out, "\n")
	start := -1
	for i, l := range lines {
		if strings.Contains(l, "UNDO") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("the abort printed no UNDO:\n%s", out)
	}
	var b []string
	for _, l := range lines[start:] {
		if strings.TrimSpace(l) == "" || !strings.HasPrefix(l, "    ") {
			break
		}
		b = append(b, strings.TrimSpace(l))
	}
	if len(b) == 0 {
		t.Fatalf("the UNDO block is empty:\n%s", out)
	}
	return strings.Join(b, "\n")
}

func TestQAQueueCutoverMoveUndoDoesNotOverwriteTheLiveStore(t *testing.T) {
	t.Parallel()
	// unskipped by ranger-base-iycc
	if os.Geteuid() == 0 {
		t.Skip("root")
	}
	constitution, _ := qcConstitution(t)
	src := filepath.Join(constitution, ".beads")
	qcDrift(t, constitution) // q-4: live, uncommitted, and the thing to lose

	queue := filepath.Join(t.TempDir(), "queue")
	project := qcWork(t, t.TempDir(), src)
	shim := qcFlakyMv(t, "2")

	out, err := qcRunEnv(t, []string{"PATH=" + shim + string(os.PathListSeparator) + os.Getenv("PATH")},
		constitution, queue, t.TempDir(), []string{project})
	if err == nil {
		t.Fatalf("the fixture did not abort, so this pin measures nothing:\n%s", out)
	}
	if !strings.Contains(out, `stage "move"`) || !strings.Contains(out, "mv: refused") {
		t.Fatalf("something other than a partial move failed:\n%s", out)
	}

	// The witnesses that the half-state is the bead's, not the shipped
	// pin's: something DID move, the drifted projection did NOT, and
	// nothing waiting in the queue under that name already carries the
	// drift — otherwise an overwrite would be invisible here. That last one
	// is deliberately written to hold on BOTH sides of the fix: before it
	// the file is the replayed projection, which lacks q-4; after it there
	// is no such file at all, because the queue's working tree is emptied
	// before the move loop starts.
	dst := filepath.Join(queue, ".beads")
	if _, e := os.Stat(filepath.Join(dst, "beads.db")); e != nil {
		t.Fatalf("nothing moved at all — no partial move, nothing measured: %v", e)
	}
	if _, e := os.Stat(filepath.Join(src, beadsJSONL)); e != nil {
		t.Fatalf("everything moved — no partial move, nothing measured: %v", e)
	}
	if b, e := os.ReadFile(filepath.Join(dst, beadsJSONL)); e == nil && strings.Contains(string(b), "q-4") {
		t.Fatalf("the copy waiting in the queue already carries the drift, so an overwrite would be invisible")
	}

	undo := qcUndoBlock(t, out)
	if b, e := exec.Command("/bin/sh", "-c", undo).CombinedOutput(); e != nil {
		t.Errorf("the printed UNDO does not run: %v\n%s\n%s", e, undo, b)
	}

	home := readFile(t, filepath.Join(src, beadsJSONL))
	if !strings.Contains(home, "q-4") {
		t.Errorf("the UNDO restored the replayed projection over the live one — the uncommitted drift is gone:\n%s", home)
	}
	for _, f := range []string{beadsJSONL, beadsDeleted, "beads.db", ".gitignore"} {
		if _, e := os.Stat(filepath.Join(src, f)); e != nil {
			t.Errorf("the UNDO did not bring %s home: %v", f, e)
		}
	}
	if _, e := os.Stat(queue); e == nil {
		t.Errorf("the UNDO left the queue repo standing at %s", queue)
	}
}

func TestQAQueueCutoverTrackedSubdirDoesNotNestSilently(t *testing.T) {
	t.Parallel()
	// unskipped by ranger-base-8izk: FIXED as a side effect of ranger-base-iycc
	// (326a8dc) — see the FIXED block above.
	constitution, _ := qcConstitution(t)
	src := filepath.Join(constitution, ".beads")
	write(t, filepath.Join(src, "sub", "x"), "tracked\n")
	mustGit(t, constitution, "add", ".beads/sub/x")
	mustGit(t, constitution, "commit", "-q", "-m", "beads: a tracked subdir", "--", ".beads")
	write(t, filepath.Join(src, "sub", "y"), "live only\n")

	queue := filepath.Join(t.TempDir(), "queue")
	out, err := qcRun(t, constitution, queue, t.TempDir(), []string{qcWork(t, t.TempDir(), src)})
	dst := filepath.Join(queue, ".beads")

	// Either it aborts and says so — the close's prediction — or it lands
	// the live subdirectory AT its own path. What it must not do is exit 0
	// having buried it one level down under a stale replayed copy.
	if err != nil {
		if !strings.Contains(out, "ABORTED") {
			t.Errorf("the run failed without naming the half-state:\n%s", out)
		}
		return
	}
	if _, e := os.Stat(filepath.Join(dst, "sub", "sub")); e == nil {
		t.Errorf("the live subdirectory was nested under the replayed one, and the script reported success:\n%s", out)
	}
	if _, e := os.Stat(filepath.Join(dst, "sub", "y")); e != nil {
		t.Errorf("the live subdirectory's content is not at its own path: %v", e)
	}
}
