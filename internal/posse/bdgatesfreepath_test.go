//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0077 D2/D3, ranger-base-rzjie — posse's own bd children run on one
// gates-free PATH, and the test that builds it resolves symlinks.
//
// The failure these pin is ranger-base-c33j6's, MEASURED twice on a live
// dispatch from a gated seat: posse takes ADR 0073's per-store write lock,
// execs a bare `bd`, the launching persona's gates bin leads that PATH, and
// that dir's `bd` shim takes the SAME lock on a new open file description.
// posse waited BdWriteLockWait on itself and refused the write. Nothing was
// holding the lock but posse.
//
// MUTATION-CHECKED, each against the one arm it should red:
//   - runOnce exec'ing `b.Bin` again instead of the name `bin()` resolved
//     reds the first test with the live refusal verbatim ("bd write lock was
//     held for 2s — a bd write is still running against it; nothing was
//     written (ADR 0073)"), which is ranger-base-c33j6's line with the
//     fixture's wait in it. Since ranger-base-cwsi8 that mutant does not
//     compile on its own — `bin` goes unused — which is a stronger kill than
//     a red test and the reason the resolution happens in a named variable
//     rather than inline in the exec call. Re-check it by adding `_ = bin`.
//   - bdChildEnv returning env unchanged (no PATH) reds the first test's last
//     arm alone, on the gates dir handed to the child.
//   - bin() handing the bare name back — the fallback ranger-base-cwsi8
//     removed — reds all three arms of TestBdBinUnresolvableOutsideTheGates-
//     IsAnError and nothing else, reproducing to the wall the measurement QA
//     took on the bead: the shim ran with posse's own argv, after 2.013s,
//     refused on posse's own store lock.
//   - gatesPathSplit judging the spelling only, not the resolution, reds the
//     third test's three arms and nothing else.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// D3: the bd child is resolved and run outside every gates dir, so the shim
// is never reached and the write completes.
//
// Red before the fix: Go's own exec resolution of a bare `bd` searches THIS
// process's $PATH — never the cmd.Env a caller sets — whose head here is the
// gates bin dir, and the shim planted there refuses on the lock runOnce is
// holding.
func TestBdRunsOutsideTheLaunchersGatesDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The launching seat's wall: a gates bin dir leading PATH, holding a
	// `bd` that takes the store lock (TestMain's RHQ_FAKE_GATED_BD_STORE
	// arm → fakeGatedBd).
	gatesBin := filepath.Join(t.TempDir(), "state", "gates", "launcher", "bin")
	if err := os.MkdirAll(gatesBin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, filepath.Join(gatesBin, "bd")); err != nil {
		t.Fatal(err)
	}
	mark := filepath.Join(t.TempDir(), "shim-ran")
	t.Setenv("RHQ_FAKE_GATED_BD_STORE", bdWriteStore(dir))
	t.Setenv("RHQ_FAKE_GATED_BD_MARK", mark)

	// The real binary, on an ordinary dir behind it. It reports the $PATH it
	// was handed, which is the other half of D3: setting the child's PATH
	// and not merely resolving past the shim, so that bd's own git hooks —
	// which call `bd` by name — cannot reach the shim either.
	realBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(realBin, 0o755); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(t.TempDir(), "child-path")
	script := "#!/bin/sh\nprintf '%s' \"$PATH\" > " + childPath + "\necho '{}'\n"
	if err := WriteExecutable(filepath.Join(realBin, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", gatesBin+sep+realBin+sep+PathOutsideGates(""))

	b := Bd{Bin: "bd"} // the production spelling: a bare name, resolved by us
	started := time.Now()
	out, err := b.runOnce(dir, "update", "x-1", "--status", "in_progress")
	if err != nil {
		t.Fatalf("a posse bd write must not be answered by the launcher's shim: %v (after %s)", err, time.Since(started).Round(time.Millisecond))
	}
	if got := strings.TrimSpace(string(out)); got != "{}" {
		t.Errorf("stdout = %q, want the real bd's — the write ran, but not through the binary we planted", got)
	}
	if _, err := os.Stat(mark); err == nil {
		b, _ := os.ReadFile(mark)
		t.Errorf("the gates-dir bd shim ran: %q — posse's own children run outside every gates dir (ADR 0077 D3)", b)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	// And the child's own PATH carries no gates dir at all.
	handed, rerr := os.ReadFile(childPath)
	if rerr != nil {
		t.Fatalf("the real bd never ran: %v", rerr)
	}
	for _, p := range filepath.SplitList(string(handed)) {
		if isGatesPath(p) || isGatesPath(resolveLinks(p)) {
			t.Errorf("the bd child was handed a gates dir on its PATH: %s — a hook child of bd's calls `bd` by name and would reach the shim (ADR 0077 D3)", p)
		}
	}
}

// RHQ_BD_BIN keeps precedence: a Bin that names a path is an answer, not a
// search, and D3's resolution must not take it away — every fixture on the
// box points that variable at a binary and expects it run.
//
// Parallel since ranger-base-cwsi8: the arm that licensed the bare-name
// fallback lived here and needed `t.Setenv("PATH", "")`, which this test's
// own claim never did. That arm is now
// TestBdBinUnresolvableOutsideTheGatesIsAnError, which sets PATH and stays
// serial for it; what is left here touches no process-wide state.
func TestBdBinNamedExplicitlyIsNotReResolved(t *testing.T) {
	t.Parallel()
	named := filepath.Join(t.TempDir(), "state", "gates", "p", "bin", "bd")
	if err := os.MkdirAll(filepath.Dir(named), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := (Bd{Bin: named}).bin()
	if err != nil {
		t.Fatalf("bin() = %v, want the named path back", err)
	}
	if got != named {
		t.Errorf("bin() = %q, want the named path %q — even inside a gates dir", got, named)
	}
}

// ranger-base-cwsi8: a bare name nothing outside the gates answers is an
// ERROR, and never the bare name back. This arm used to assert the opposite —
// `bin()` returning "bd" under PATH="" — on the strength of a doc comment
// saying the bare name "goes to exec unresolved so that exec reports it the
// way it always has". It does not: exec resolves a bare name with
// exec.LookPath over THIS process's $PATH, the gated one, so it lands on the
// launcher's `bd` shim. The shim takes ADR 0073's per-store write lock,
// runOnce is already holding it, and posse refuses its own write — which is
// ranger-base-c33j6's measured deadlock, reinstated by the one branch of D3's
// fix that nothing reached.
//
// Three assertions, because the unit answer and its consequence are
// different claims: bin() errors; runOnce refuses without ever running the
// shim; and what it refuses with is the missing binary, not posse's own held
// lock.
//
// MUTATION-CHECKED: restoring `return b.Bin, nil` as the fallback reds all
// three — bin() = "bd", the shim ran with posse's own argv after 2.013s, and
// the refusal is ranger-base-c33j6's lock line verbatim.
func TestBdBinUnresolvableOutsideTheGatesIsAnError(t *testing.T) {
	// A gates bin dir on PATH holding a `bd`, and NOT ONE non-gates element
	// that answers: the production spelling then has nothing to resolve to.
	gatesBin := filepath.Join(t.TempDir(), "state", "gates", "launcher", "bin")
	if err := os.MkdirAll(gatesBin, 0o755); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, filepath.Join(gatesBin, "bd")); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(t.TempDir(), "bin") // an ordinary dir, holding no bd
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", gatesBin+sep+empty)

	if got, err := (Bd{Bin: "bd"}).bin(); err == nil {
		t.Errorf("bin() = %q, nil — want an error: handing a bare name to exec resolves it on the GATED PATH, onto %s (ADR 0077 D3)", got, gatesBin)
	}

	// And the consequence, end to end: a write verb, so runOnce would take
	// ADR 0073's lock the shim then collides with. It must refuse before
	// that, so the shim never runs and the lock is never contended.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	mark := filepath.Join(t.TempDir(), "shim-ran")
	t.Setenv("RHQ_FAKE_GATED_BD_STORE", bdWriteStore(dir))
	t.Setenv("RHQ_FAKE_GATED_BD_MARK", mark)

	started := time.Now()
	out, err := (Bd{Bin: "bd"}).runOnce(dir, "update", "x-1", "--status", "in_progress")
	waited := time.Since(started).Round(time.Millisecond)
	if err == nil {
		t.Errorf("runOnce = %q, nil — want a refusal: there is no bd outside the gates to run", string(out))
	}
	if got, _ := os.ReadFile(mark); got != nil {
		t.Errorf("the gates-dir bd shim ran: %q (after %s) — posse resolved its own bare name onto the launcher's shim and deadlocked on its own store lock (ranger-base-c33j6, ranger-base-cwsi8)", got, waited)
	}
	// The error says which binary it could not find, not that a lock was
	// held: posse refusing its own write is the symptom this removes, and a
	// reader who sees that message again is looking at a regression.
	if err != nil && strings.Contains(err.Error(), "write lock was held") {
		t.Errorf("runOnce refused on the store lock: %v — that is posse waiting on itself, not a missing binary", err)
	}
}

// D2: one function, one test — and that test resolves symlinks, so an alias
// to a gates bin dir is stripped from the PATH posse hands on and named as
// foreign, by the same walk (gatesPathSplit). Before the merge,
// PathOutsideGates' literal match handed this dir to a child while
// ForeignGatesElements, reading the same PATH, called it foreign
// (ranger-base-z51g).
func TestPathOutsideGatesStripsASymlinkAliasToAGatesDir(t *testing.T) {
	home := t.TempDir()
	gatesBin := filepath.Join(home, "state", "gates", "alpha", "bin")
	ownBin := filepath.Join(home, "state", "gates", "mine", "bin")
	for _, d := range []string{gatesBin, ownBin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	scratch := t.TempDir()
	alias := filepath.Join(scratch, "tools") // spells nothing a literal match catches
	if err := os.Symlink(gatesBin, alias); err != nil {
		t.Fatal(err)
	}
	ownAlias := filepath.Join(scratch, "mytools")
	if err := os.Symlink(ownBin, ownAlias); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(scratch, "usr-bin")
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", strings.Join([]string{alias, keep, gatesBin, ownAlias, ownBin}, sep))

	if got := PathOutsideGates(""); got != keep {
		t.Errorf("PathOutsideGates(\"\") = %q, want %q — an element that RESOLVES into a gates dir is dropped however it is spelled (ADR 0077 D2)", got, keep)
	}
	// ADR 0077 D2: the caller's OWN gates dir goes with the rest — behind
	// its alias as well as by name. A seat's walls are its own PID's.
	if got := PathOutsideGates(ownBin); got != keep {
		t.Errorf("PathOutsideGates(own) = %q, want %q — own included", got, keep)
	}
	// The other half of the same walk, unchanged for parity's caller.
	want := []string{alias, gatesBin}
	if got := ForeignGatesElements(ownBin); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ForeignGatesElements(own) = %v, want %v", got, want)
	}
}
