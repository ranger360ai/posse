//go:build posse_arm3

package posse

// The GIT_CONFIG_GLOBAL snapshot's MODE is the second half of its wall
// (ranger-base-k2cbh, from the verify of ranger-base-161t2).
//
// The lock DIRECTORY at `<snapshot>.lock` refuses every config write that
// goes through git's O_CREAT|O_EXCL lockfile. MEASURED 2026-11-15, git 2.56.0
// linux, over `--global`, `--add`, `--unset`, `--unset-all`,
// `--remove-section`, `--rename-section`, `--replace-all` and
// `--file <snapshot>`: all rc 255, bytes unmoved.
//
// `git config --global --edit` is the exception. It takes no lock: it hands
// the path to $GIT_EDITOR and the editor writes in place, so the lock
// directory is nowhere in that path and the file's 0444 mode is the whole
// wall. MEASURED the same day: a snapshot WITH its lock directory but at mode
// 0644 is rewritten by `--edit` at rc 0 — user.email emptied, and
// `git var GIT_AUTHOR_IDENT` then answers "Author identity unknown".
//
// Nothing held that mode. The mutant `os.Chmod(tmp.Name(), 0o444)` ->
// `0o644` in gitGlobalSnapshot left all five pins in
// gitglobalsnapshot_qa_test.go, TestQAGitGlobalSnapshotIsUnderNoCagedWriteGrant
// and the whole `-run 'GitGlobal|InletPin|HookSkipGlobal'` family GREEN, while
// the control mutant (never make the lock directory) reds four of them. This
// file is that mutant's door.
//
// Not parallel: the fixture sets HOME, as the rest of this family does.

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// gitEditorThatWrites is a $GIT_EDITOR that truncates the file it is handed.
// git runs the editor through `sh -c "$editor \"$@\""`, so a shell snippet
// needs no planted executable — which also keeps this test out of ADR 0085's
// cold-exec class. $0 is the `--`; $1 is the path git passes.
const gitEditorThatWrites = `/bin/sh -c 'printf "[user]\n\temail = probe@invalid\n" > "$1"' --`

// TestQAGitGlobalSnapshotIsReadOnlyAgainstTheEditorRoute holds the mode, and
// holds what the mode is FOR: `git config --global --edit` under the rendered
// row must not change the snapshot every seat reads as its global scope. The
// control is the same line against the operator's own file, which is the row
// as it stood before ranger-base-161t2 — it succeeds there, so a green arm
// above measures something.
func TestQAGitGlobalSnapshotIsReadOnlyAgainstTheEditorRoute(t *testing.T) {
	_, homeCfg, _ := globalScopeFixture(t, true, false)
	if err := PrepareGitGlobalSnapshot(); err != nil { // as a launch does
		t.Fatal(err)
	}
	row, ok := pinRow("GIT_CONFIG_GLOBAL")
	if !ok || row == homeCfg {
		t.Fatalf("CONTROL FAILED: the row is %q (present %v), not a snapshot of %s", row, ok, homeCfg)
	}
	want, err := os.ReadFile(row)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(row)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o444 {
		t.Errorf("the snapshot %s is mode %#o, want 0444 — the lock directory does not stand in `git config --global --edit`'s way, so the mode is the only thing refusing it (ranger-base-k2cbh)", row, perm)
	}

	out, rc := seatGit(t, row, []string{"GIT_EDITOR=" + gitEditorThatWrites},
		"config", "--global", "--edit")
	if got, _ := os.ReadFile(row); !bytes.Equal(got, want) {
		t.Errorf("`git config --global --edit` rewrote the snapshot under the rendered row (rc %d):\n before %q\n after  %q\n%s", rc, want, got, out)
	}
	if rc == 0 {
		t.Errorf("`git config --global --edit` under the rendered row exited 0 — the editor route must refuse, not land somewhere quietly:\n%s", out)
	}

	// CONTROL: the row as it stood names the operator's own file, and the same
	// editor line rewrites it. Without this, the arm above could be green
	// because the editor never ran.
	opBefore, err := os.ReadFile(homeCfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, rc := seatGit(t, homeCfg, []string{"GIT_EDITOR=" + gitEditorThatWrites},
		"config", "--global", "--edit"); rc != 0 {
		t.Fatalf("CONTROL FAILED: `git config --global --edit` under GIT_CONFIG_GLOBAL=%s exited %d, so the editor never wrote and the arm above measured nothing", homeCfg, rc)
	}
	if opAfter, _ := os.ReadFile(homeCfg); bytes.Equal(opAfter, opBefore) {
		t.Fatal("CONTROL FAILED: the editor line did not change the operator's own file, so the arm above measured nothing")
	}
}

// TestQAAGitGlobalModeOnlyTamperDegradesAndTheNextLaunchHealsIt is the other
// half of the mode (ranger-base-r9slg, from the verify of ranger-base-2zgr3).
//
// The pin above holds that the mode IS 0444 as written. What nothing held is
// what happens when it is not: a mode-only tamper — bytes untouched, lock
// directory in place, 0444 turned to 0644 — was the one tamper posse neither
// saw nor repaired. gitGlobalSnapshotVerify read the lock and the bytes and
// passed; the chmod lived only on gitGlobalSnapshot's rewrite path, which a
// content-identical snapshot skips. MEASURED 2026-11-20 at 9193d3b1: launch 2
// returned nil over a 0644 snapshot, every reader kept naming it, and `git
// config --global --edit` under the rendered row rewrote it at rc 0.
//
// The content tamper is the control for both halves and it is one file over
// (TestQAGitGlobalSnapshotIsRewrittenWhenTampered): it was healed all along,
// which is what made this one's silence hard to see.
//
// MUTATIONS RUN (each reds this test): dropping the mode arm from
// gitGlobalSnapshotVerify; dropping gitGlobalSnapshot's heal arm.
func TestQAAGitGlobalModeOnlyTamperDegradesAndTheNextLaunchHealsIt(t *testing.T) {
	_, homeCfg, _ := globalScopeFixture(t, true, false)
	row := gitGlobalPinOf(t, homeCfg) // prepares, as a launch does
	want, err := os.ReadFile(row)
	if err != nil {
		t.Fatal(err)
	}
	lock := row + ".lock"

	// The tamper: the mode and nothing else.
	if err := os.Chmod(row, 0o644); err != nil {
		t.Fatal(err)
	}

	// EVERY READER degrades. The row falls back to the operator's own file —
	// writable, but the file the operator can see and the one GitGlobalScopeDegraded
	// is about — rather than a snapshot posse can no longer vouch for.
	if got, _ := pinRow("GIT_CONFIG_GLOBAL"); got != homeCfg {
		t.Errorf("over a snapshot at mode 0644 the row is %q, want the fallback %s: a reader is still pointed at a snapshot `git config --global --edit` can rewrite (ranger-base-r9slg)", got, homeCfg)
	}
	if d := GitGlobalScopeDegraded(); !strings.Contains(d, "DEGRADED") || !strings.Contains(d, "ranger-base-161t2") {
		t.Errorf("a mode-only tamper printed no degraded line: %q", d)
	}

	// THE NEXT LAUNCH HEALS IT, the way a content tamper has always been
	// healed — and heals the mode without touching the bytes or the lock.
	if err := PrepareGitGlobalSnapshot(); err != nil {
		t.Fatalf("the launch over a mode-tampered snapshot refused instead of healing it: %v", err)
	}
	fi, err := os.Lstat(row)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o444 {
		t.Errorf("after a launch the snapshot is still mode %#o, want 0444 — the mode-only tamper is not self-healing (ranger-base-r9slg)", perm)
	}
	if got, _ := os.ReadFile(row); !bytes.Equal(got, want) {
		t.Errorf("the heal rewrote the snapshot's bytes: %q, want %q", got, want)
	}
	if fi, err := os.Lstat(lock); err != nil || !fi.IsDir() {
		t.Errorf("the heal lost the lock directory: %v", err)
	}
	if got, _ := pinRow("GIT_CONFIG_GLOBAL"); got != row {
		t.Errorf("after the heal the row is %q, want the snapshot %s again", got, row)
	}
	if d := GitGlobalScopeDegraded(); d != "" {
		t.Errorf("a healed snapshot still prints a degraded line: %s", d)
	}

	// CONTROL, and the reason any of this matters: at 0644 the editor route
	// really is open, so the arms above are about a live hole and not about a
	// mode nobody can use. Measured last because it leaves the snapshot's
	// bytes rewritten, which is a CONTENT tamper and another test's subject.
	if err := os.Chmod(row, 0o644); err != nil {
		t.Fatal(err)
	}
	out, rc := seatGit(t, row, []string{"GIT_EDITOR=" + gitEditorThatWrites},
		"config", "--global", "--edit")
	if got, _ := os.ReadFile(row); rc != 0 || bytes.Equal(got, want) {
		t.Fatalf("CONTROL FAILED: `git config --global --edit` over a 0644 snapshot exited %d and left the bytes %q — the mode is not what refuses that write here, so the arms above measured nothing:\n%s", rc, got, out)
	}
}

// ggsAnswered runs f and reports whether it answered inside d, which is the
// one thing a probe over a FIFO cannot take for granted: a reader that opens
// one O_RDONLY with no writer blocks in the kernel, not in Go, so an
// unbounded call here would hang the whole arm-3 binary until its 25m
// timeout and print no test name. Bounded, the same mutant FAILS by name
// (ranger-base-suozl).
func ggsAnswered(d time.Duration, f func() error) (error, bool) {
	done := make(chan error, 1)
	go func() { done <- f() }()
	select {
	case err := <-done:
		return err, true
	case <-time.After(d):
		return nil, false
	}
}

// TestQAAGitGlobalSnapshotVerifyRefusesWhatIsNotAFileAtThatName is the door
// on gitGlobalSnapshotVerify's REGULAR-FILE arm — the `!fi.Mode().IsRegular()`
// half of the shape check ranger-base-r9slg added beside the mode. Nothing
// drove it: MEASURED 2026-11-21 at 52ce199f (ranger-base-suozl, verifying
// ranger-base-r9slg), deleting that arm from the condition leaves
// TestQAAGitGlobalModeOnlyTamperDegradesAndTheNextLaunchHealsIt and the rest
// of this file GREEN. The close's own sentence for it — "Lstat, not Stat, and
// IsRegular: a symlink planted at the snapshot's name carries mode 0777 and
// would resolve to whatever it points at" — was a claim no run tested.
//
// THE FIFO IS THE ISOLATING SHAPE, and that is the whole reason this arm
// exists rather than another symlink row. A fifo can be made at mode 0444, so
// `fi.Mode().Perm() != gitGlobalSnapshotMode` is FALSE for it and the mode arm
// below cannot answer — the regular-file arm is the only thing left. The
// symlink row is the opposite case and is here as the close's own example:
// on linux a link's own mode is 0777 whatever chmod is typed (chmod follows),
// so there the MODE arm refuses it and this arm is redundant; on darwin,
// where lchmod can set a link to 0444, this arm is again the only one. Both
// rows assert the refusal; only the fifo row asserts WHICH arm made it.
//
// MEASURED the same day, and why the shape check cannot simply be deleted as
// redundant: with a 0444 fifo at that name, `PrepareGitGlobalSnapshot` — the
// launch path — does not answer at all. gitGlobalSnapshot compares the
// snapshot's bytes with `os.ReadFile(snap)` before any shape check, and that
// read blocks. The verify's arm is what keeps every READER (gitGlobalPinValue,
// GitGlobalScopeDegraded) answering over such a plant, which is the only
// reason this test can assert anything after the plant at all.
func TestQAAGitGlobalSnapshotVerifyRefusesWhatIsNotAFileAtThatName(t *testing.T) {
	_, homeCfg, _ := globalScopeFixture(t, true, false)
	row := gitGlobalPinOf(t, homeCfg) // prepares, as a launch does
	want, err := os.ReadFile(row)
	if err != nil {
		t.Fatal(err)
	}
	// POSITIVE CONTROL: the rig can pass. Without it an arm below could be
	// green over a verify that refuses everything, including the snapshot a
	// launch just wrote.
	if err, answered := ggsAnswered(10*time.Second, func() error { return gitGlobalSnapshotVerify(row, want) }); !answered || err != nil {
		t.Fatalf("CONTROL FAILED: the snapshot this launch just wrote does not verify (answered=%v, %v)", answered, err)
	}

	for _, tc := range []struct {
		name string
		// plant leaves something that is not a regular file at row.
		plant func(t *testing.T)
		// modeArmCanAnswer: whether `perm != 0444` is true of the plant, i.e.
		// whether the arm BELOW this one could have made the refusal.
		modeArmCanAnswer bool
	}{
		{"a fifo at the snapshot's own mode", func(t *testing.T) {
			if err := syscall.Mkfifo(row, 0o444); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"a symlink to a file with the same bytes", func(t *testing.T) {
			decoy := filepath.Join(t.TempDir(), "decoy")
			if err := os.WriteFile(decoy, want, 0o444); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(decoy, row); err != nil {
				t.Fatal(err)
			}
		}, runtime.GOOS != "darwin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Tolerant, because the row is the one the row BEFORE this one
			// planted and removed: what each row needs is that nothing is
			// there when it plants, and its own Lstat below is what says the
			// plant landed.
			if err := os.Remove(row); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			defer func() { _ = os.Remove(row) }()
			tc.plant(t)
			fi, err := os.Lstat(row)
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().IsRegular() {
				t.Fatalf("CONTROL FAILED: the plant left a regular file at %s, so this row measures nothing", row)
			}
			if got := fi.Mode().Perm() != gitGlobalSnapshotMode; got != tc.modeArmCanAnswer {
				t.Fatalf("CONTROL FAILED: the plant is mode %#o, so the mode arm can answer = %v, want %v — this row no longer isolates the arm it is about", fi.Mode().Perm(), got, tc.modeArmCanAnswer)
			}
			err, answered := ggsAnswered(10*time.Second, func() error { return gitGlobalSnapshotVerify(row, want) })
			if !answered {
				t.Fatalf("gitGlobalSnapshotVerify did not answer over %s at the snapshot's name: it read the plant instead of refusing its shape, and a reader of the global scope now blocks (ranger-base-suozl)", tc.name)
			}
			if err == nil {
				t.Errorf("gitGlobalSnapshotVerify accepted %s at %s — a snapshot posse cannot vouch for is named to every seat as its global scope (ranger-base-r9slg)", tc.name, row)
			}
			// AND THE READERS SAY SO, which is what the arm is for: the row
			// falls back to the operator's own file and the degraded line
			// names the reason.
			if got, _ := pinRow("GIT_CONFIG_GLOBAL"); got != homeCfg {
				t.Errorf("over %s the row is %q, want the fallback %s", tc.name, got, homeCfg)
			}
			if d := GitGlobalScopeDegraded(); !strings.Contains(d, "DEGRADED") {
				t.Errorf("over %s no degraded line was printed: %q", tc.name, d)
			}
		})
	}
}

// TestQAAGitGlobalSnapshotLaunchAnswersOverWhatIsNotAFileAtThatName is the
// WRITER's half of the shape check (ranger-base-40vo2, from the verify at
// ranger-base-suozl), and the test above is the reader's.
//
// ranger-base-r9slg put the shape check in gitGlobalSnapshotVerify only, so
// every READER of the row refused a plant at the snapshot's name in
// microseconds while gitGlobalSnapshot compared bytes with `os.ReadFile(snap)`
// BEFORE asking what was there. MEASURED 2026-11-21 at 52ce199f: with a 0444
// fifo at `<SeatWallRoot(HOME)>/git-global/<hash>/read-only-snapshot.gitconfig`,
// gitGlobalSnapshotVerify answers at once with "is not a regular file" and
// PrepareGitGlobalSnapshot — the LAUNCH path — does not answer in 5s. It opened
// the fifo O_RDONLY with no writer and blocked in the kernel, so the launch
// never answered and the DEGRADED line that exists for exactly this plant was
// never reached: nothing returned to print it. That is the one shape in this
// family with no line for an operator to read.
//
// TWO ROWS, because "answers" has two honest answers and the bead's fix claim
// is about both. A fifo is REPLACEABLE: the rewrite arm's rename lands over it
// the way it lands over tampered bytes, so the launch heals it and the row
// names a snapshot again. A directory at that name is not replaceable — rename
// refuses it — so the launch must come back with the error and let the caller
// print the degraded line. Hanging is the only outcome ruled out for both.
//
// BOUNDED, through ggsAnswered, for the reason that helper names: an unbounded
// call over the mutant would hang the whole arm-3 binary to its timeout and
// print no test name. The mutant (gitGlobalSnapshotRead -> os.ReadFile) FAILS
// this test by name on the fifo row and leaves the directory row green, which
// is the state the tree was in before this landed.
func TestQAAGitGlobalSnapshotLaunchAnswersOverWhatIsNotAFileAtThatName(t *testing.T) {
	_, homeCfg, _ := globalScopeFixture(t, true, false)
	row := gitGlobalPinOf(t, homeCfg) // prepares, as a launch does
	want, err := os.ReadFile(row)
	if err != nil {
		t.Fatal(err)
	}
	lock := row + ".lock"
	// POSITIVE CONTROL: a launch over the snapshot it just wrote answers nil.
	// Without it, "answered" below could be a function that refuses everything.
	if err, answered := ggsAnswered(10*time.Second, PrepareGitGlobalSnapshot); !answered || err != nil {
		t.Fatalf("CONTROL FAILED: the second launch over an untouched snapshot answered=%v, %v", answered, err)
	}

	for _, tc := range []struct {
		name  string
		plant func(t *testing.T)
		// heals: whether the rewrite arm can land over the plant, i.e. whether
		// the launch is expected to answer nil and leave a snapshot behind.
		heals bool
	}{
		{"a fifo at the snapshot's own mode", func(t *testing.T) {
			if err := syscall.Mkfifo(row, 0o444); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"a directory the rewrite cannot replace", func(t *testing.T) {
			if err := os.Mkdir(row, 0o755); err != nil {
				t.Fatal(err)
			}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.Remove(row); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			tc.plant(t)
			fi, err := os.Lstat(row)
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().IsRegular() {
				t.Fatalf("CONTROL FAILED: the plant left a regular file at %s, so this row measures nothing", row)
			}

			err, answered := ggsAnswered(10*time.Second, PrepareGitGlobalSnapshot)
			if !answered {
				t.Fatalf("PrepareGitGlobalSnapshot did not answer over %s at %s: the launch read the plant instead of asking its shape, so it is blocked in the kernel and no degraded line can be printed (ranger-base-40vo2)", tc.name, row)
			}
			if !tc.heals {
				// The honest answer when posse can neither trust the snapshot
				// nor repair it: the error, which is what makes the caller's
				// DEGRADED line reachable.
				if err == nil {
					t.Errorf("the launch returned nil over %s at %s — it can neither read nor replace that name, so it must say so", tc.name, row)
				}
				if got, _ := pinRow("GIT_CONFIG_GLOBAL"); got != homeCfg {
					t.Errorf("over %s the row is %q, want the fallback %s", tc.name, got, homeCfg)
				}
				if d := GitGlobalScopeDegraded(); !strings.Contains(d, "DEGRADED") {
					t.Errorf("over %s no degraded line was printed: %q", tc.name, d)
				}
				if err := os.Remove(row); err != nil {
					t.Fatal(err)
				}
				return
			}
			// THE HEAL, the same one the mode-only tamper gets: the rewrite
			// arm's rename replaces the plant, and the row names the snapshot.
			if err != nil {
				t.Fatalf("the launch over %s refused instead of healing it: %v", tc.name, err)
			}
			fi, err = os.Lstat(row)
			if err != nil {
				t.Fatal(err)
			}
			if !fi.Mode().IsRegular() {
				t.Errorf("after a launch %s is still at %s, mode %v — the plant is not self-healing", tc.name, row, fi.Mode())
			}
			if perm := fi.Mode().Perm(); perm != gitGlobalSnapshotMode {
				t.Errorf("the healed snapshot is mode %#o, want %#o", perm, gitGlobalSnapshotMode)
			}
			if got, _ := os.ReadFile(row); !bytes.Equal(got, want) {
				t.Errorf("the heal left the wrong bytes at %s: %q, want %q", row, got, want)
			}
			if fi, err := os.Lstat(lock); err != nil || !fi.IsDir() {
				t.Errorf("the heal lost the lock directory: %v", err)
			}
			if got, _ := pinRow("GIT_CONFIG_GLOBAL"); got != row {
				t.Errorf("after the heal the row is %q, want the snapshot %s again", got, row)
			}
			if d := GitGlobalScopeDegraded(); d != "" {
				t.Errorf("a healed snapshot still prints a degraded line: %s", d)
			}
		})
	}
}
