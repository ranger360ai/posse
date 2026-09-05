package posse

// ADR 0023 Decision 2 rests on WHERE the launcher's own render sits between
// the write and the exec. The exec happens in the launcher's UNSANDBOXED
// context at every launch, and $TMPDIR is writable by every caged session on
// this box at the same uid (ADR 0002's seatbelt writable set), so a render
// dropped straight into $TMPDIR under an enumerable name at 0755 leaves a
// window on the exact escalation the ADR's Context set out to remove: a
// caged session getting its bytes run outside the cage. Filed as
// ranger-base-t5vh, a hardening ask — not a demonstrated race.
//
// The pin below observes the file the exec ACTUALLY receives, at the moment
// it receives it, rather than the writer in isolation: a fake `sh` on PATH
// stands in for the probe shell and MOVES the whole scratch directory aside
// before returning, so the test owns it — modes, names and contents exactly
// as of exec time — after execOwnRenders' RemoveAll has run over a path that
// is no longer there.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The renders the L3 probe execs must be unreachable by name and unwritable
// by anything but the launcher: mode 0700 inside a 0700 directory, never
// loose in the shared $TMPDIR. Flip either back to 0755, or write the render
// at the top of $TMPDIR again, and this reds.
func TestL3ProbeRendersAreNotLooseInTheSharedTempDir(t *testing.T) {
	repo, _ := qaHookRepo(t)

	// Every directory is built before $TMPDIR moves, so t.TempDir is not
	// answering the variable this test is about to change.
	base := t.TempDir()
	tmp := filepath.Join(base, "tmp")
	rec := filepath.Join(base, "rec")
	bin := filepath.Join(base, "bin")
	for _, d := range []string{tmp, rec, bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// The stand-in probe shell. argv is `-c <script> <arg0> <push> <commit>
	// <msg>`, so the three files are $4 $5 $6. It records them, moves the
	// directory holding them out of reach of the caller's RemoveAll, and
	// exits 0 so both slots read OK — this test is about the files, not the
	// verdict.
	shim := "#!/bin/sh\n" +
		"printf '%s\\n%s\\n%s\\n' \"$4\" \"$5\" \"$6\" > " + filepath.Join(rec, "paths") + "\n" +
		"mv \"$(dirname \"$5\")\" " + filepath.Join(rec, "scratch") + "\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "sh"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TMPDIR", tmp)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	commitRender := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	push, commit := execOwnRenders(repo, true, commitRender)
	if !push || !commit {
		t.Fatalf("the shim exits 0; both slots must read OK (push=%v commit=%v) — the exec did not happen as expected", push, commit)
	}

	raw, err := os.ReadFile(filepath.Join(rec, "paths"))
	if err != nil {
		t.Fatalf("the shim did not run: %v", err)
	}
	paths := strings.Fields(string(raw))
	if len(paths) != 3 {
		t.Fatalf("want three argv paths, got %q", paths)
	}

	// Not loose in $TMPDIR: each file sits one level down, in a directory of
	// its own, and that directory is what the shim moved aside.
	for _, p := range paths {
		if filepath.Dir(p) == tmp {
			t.Errorf("render/message file is loose in the shared temp dir: %s", p)
		}
		if d := filepath.Dir(p); filepath.Dir(d) != tmp || !strings.HasPrefix(filepath.Base(d), "posse-l3-probe-") {
			t.Errorf("want a private posse-l3-probe-* directory under %s, got %s", tmp, p)
		}
	}

	scratch := filepath.Join(rec, "scratch")
	di, err := os.Stat(scratch)
	if err != nil {
		t.Fatalf("the shim could not move the scratch directory aside: %v", err)
	}
	if !di.IsDir() {
		t.Fatalf("%s is not a directory", scratch)
	}
	if got := di.Mode().Perm(); got != 0o700 {
		t.Errorf("scratch directory mode is %04o, want 0700 — the render names are enumerable", got)
	}

	// The two exec'd renders: 0700, and still byte-exact posse renders (this
	// is the half of Decision 2 that says which bytes run).
	for name, want := range map[string]string{
		"pre-push":           PrePushHook,
		"prepare-commit-msg": commitRender,
	} {
		p := filepath.Join(scratch, name)
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got := fi.Mode().Perm(); got != 0o700 {
			t.Errorf("%s mode is %04o, want 0700 — the bytes the launcher execs unsandboxed are reachable beyond its own uid", name, got)
		}
		body, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if string(body) != want {
			t.Errorf("%s is not posse's own render for this launch", name)
		}
	}

	// The message file is handed to the commit render, never exec'd.
	if fi, err := os.Stat(filepath.Join(scratch, "commit-msg")); err != nil {
		t.Errorf("commit-msg: %v", err)
	} else if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("commit-msg mode is %04o, want 0600", got)
	}
}

// The scratch directory is this launch's alone and does not outlive the
// probe: nothing is left behind in $TMPDIR for a later reader to find.
func TestL3ProbeLeavesNothingInTheTempDir(t *testing.T) {
	repo, _ := qaHookRepo(t)
	tmp := filepath.Join(t.TempDir(), "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)

	execOwnRenders(repo, true, CommitGuardHook(VisibilityPublic, OpsPatternSet{}))

	left, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range left {
		t.Errorf("the probe left %s behind in the shared temp dir", e.Name())
	}
}
