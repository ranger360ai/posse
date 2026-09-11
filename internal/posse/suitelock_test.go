//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-r3czg — the box-wide suite queue's slot dir is a launch
// observable. Two pins, because the fix has two halves that fail apart: the
// GRANT (a self-sandboxing runtime's rendered line names the dir) and the
// ROW (parity says so, and says so when it does not).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sqTree makes a launch dir that carries the queue — the presence of
// scripts/suite-lock.sh is what suiteLockRoot reads, because that is what
// decides whether anything in this tree will ever ask for a slot.
func sqTree(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 0o644, not 0o755: suiteLockRoot only Stats this path, and a tracked
	// Go file that writes an executable owes the fork lock
	// (execwritesiblings_qa_test.go) for a mode nothing here needs.
	if err := os.WriteFile(filepath.Join(dir, suiteLockScriptRel), []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The grant. A codex seat's `make test-arm*` died on
// `suite-slot.*.lock: Operation not permitted` because the slot dir was on
// no writable root its line named (measured 2026-09-10), while a claude seat
// under the seatbelt took a slot on the same box off the profile's ~/.cache
// grant. This is the line that ends that asymmetry, and both arms are here
// because the grant has to be conditional: every repo would otherwise buy a
// cache root it has no use for.
func TestLaunchWritableRootsNamesTheSuiteQueueSlotDirOnlyWhereTheQueueIs(t *testing.T) {
	// Not parallel: it sets the environment the resolver reads.
	slots := filepath.Join(t.TempDir(), "slotdir")
	t.Setenv("POSSE_SUITE_LOCK_DIR", slots)

	with := sqTree(t, filepath.Join(t.TempDir(), "with"))
	if got := launchWritableRoots(with); !containsString(got, slots) {
		t.Errorf("a tree carrying %s must have the slot dir %s on its launch line, or a caged seat cannot queue: %v",
			suiteLockScriptRel, slots, got)
	}

	// The control, and it is the reason suiteLockRoot takes a dir at all: a
	// repo with no queue asks for no slot, so a grant there is a widening
	// nothing buys. Without this arm the pin above is equally green over an
	// unconditional grant.
	without := filepath.Join(t.TempDir(), "without")
	if err := os.MkdirAll(without, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := launchWritableRoots(without); containsString(got, slots) {
		t.Errorf("a tree with no %s must not buy a cache grant: %v", suiteLockScriptRel, got)
	}
}

// A scrubbed environment answers "" rather than inventing a home — the same
// rule ExpandTilde keeps (ranger-base-a3t1). A grant derived from an
// invented home is a writable root nobody asked for, and it would render as
// a pass on the row below.
func TestSuiteLockDirRefusesToInventAHome(t *testing.T) {
	t.Setenv("POSSE_SUITE_LOCK_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	if got := SuiteLockDir(); got != "" {
		t.Errorf("no HOME must answer \"\", got %q", got)
	}
	tree := sqTree(t, t.TempDir())
	if got := launchWritableRoots(tree); containsString(got, "") {
		t.Errorf("an empty root must never reach the launch line: %v", got)
	}
}

// The row, on the runtime this bead is about. Membership of the roots the
// RENDERED line names — judged against the line and not against
// launchWritableRoots' return value, so a PID's own `command:` that drops
// {deny} is judged too.
func TestSuiteQueueRowOnASelfSandboxingRuntime(t *testing.T) {
	slots := filepath.Join(t.TempDir(), "slotdir")
	t.Setenv("POSSE_SUITE_LOCK_DIR", slots)

	a := NewAppAt(t.TempDir())
	codex, err := a.LoadRuntime("codex")
	if err != nil {
		t.Fatal(err)
	}
	if !codex.SelfSandbox {
		t.Fatal("codex is not self-sandboxing here, so this test judges the wrong arm")
	}
	ag := &AgentFile{Name: "dev", MemoryDir: filepath.Join(a.PersonasDir(), "dev")}
	if err := os.MkdirAll(ag.MemoryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	work := sqTree(t, filepath.Join(t.TempDir(), "work"))

	row := a.CheckParityIn(ag, codex, CageShims, TierStrong, work).Realized[SuiteQueueGate]
	if row.Detail == "" {
		t.Fatal("a tree carrying the queue must get a row — a gap nobody prints is the failure this row exists for")
	}
	if !strings.Contains(row.Detail, AbbrevHome(slots)) || strings.Contains(row.Detail, "NO writable root") {
		t.Errorf("the slot dir is on the line, so the row must say so: %q", row.Detail)
	}
	// The row must never degrade a launch: an ungranted slot dir costs the
	// BOX speed, not the session its suite (scripts/suite-lock.sh degrades,
	// it never refuses), and a refusal over it would render like a missing
	// wall where none is missing (ranger-base-d17a).
	if row.Class != "" {
		t.Errorf("the row is a grant statement, not a wall claim: class %q", row.Class)
	}

	// The pass is not vacuous. Strip the grant back off the rendered line —
	// the exact pre-fix line — and the row must come back naming the dir.
	line := a.renderedLaunchLine(ag, codex, TierStrong, work)
	pre := strings.ReplaceAll(line, " --add-dir "+shellQuote(codexWritableRoot(slots)), "")
	if pre == line {
		t.Fatalf("the rendered line never named %s, so the control measures nothing:\n%s", slots, line)
	}
	roots, readOnly := lineWritableRoots(pre, work)
	if readOnly || anyUnderDir(roots, slots) {
		t.Fatalf("the stripped line still reaches the slot dir: %v", roots)
	}

	// And a tree with no queue gets no row at all — silence, not a line
	// about a file that is not there.
	bare := filepath.Join(t.TempDir(), "bare")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := a.CheckParityIn(ag, codex, CageShims, TierStrong, bare).Realized[SuiteQueueGate]; got.Detail != "" {
		t.Errorf("no queue in this tree, so no row: %q", got.Detail)
	}
}
