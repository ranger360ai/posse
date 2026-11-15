//go:build !posse_arm2 && !posse_arm3

package posse

// The live half of ranger-base-1l1en: a real interactive bash on a real pty,
// typed at the way `rex send` types at a seat, to settle the one question
// launchexec.go's choice rests on — does the pane's shell survive its
// runtime?
//
// It is here and not in panelinelive_test.go because it needs no server and
// no fleet: `bash --norc -i`, a pty, and under two seconds a row. Linux-only
// for the pty ioctls (it borrows wwPty from wallwatchtty_linux_test.go,
// which is linux-only for the same reason and whose subject — what the
// pane's shell reads after a seat dies — is this one's other half).
//
// MEASURED 2026-11-14 (ranger-base-ie4mx, by hand, bash 5.3.20 on this box),
// and these rows are that measurement made repeatable:
//
//	sh -c 'sleep …; exit 3'; exit        shell gone
//	sh -c 'sleep …; kill -INT $$'; exit  SHELL SURVIVED, ran the next line
//	exec sh -c 'sleep …; kill -INT $$'   shell gone
//
// The middle row is why posse does not render a trailing `exit`: an
// interactive bash abandons the rest of a command list when its foreground
// job dies of SIGINT, and SIGINT is what a 0x03 in typed text delivers —
// which the watcher's drain runs with ISIG still on. So the one spelling
// that holds for a runtime killed by a key is exec, and the trailing-exit
// rows here are kept as the negative case rather than deleted: a future
// renderer that swapped exec for `; exit` would pass every static pin in
// this tree and red arm 2 below.

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// lxeMarker is typed AFTER the launch line and is only ever printed by a
// shell that is still reading the terminal. It is split across the typed
// text so the tty's own echo of the command cannot be mistaken for its
// output: `printf 'ALI%s\n' VE` echoes as itself and prints ALIVE.
const (
	lxeMarkerCmd = `printf 'ALI%s\n' VE`
	lxeMarker    = "ALIVE"
)

// lxeShellSurvived types line at an interactive bash on a pty, waits for the
// line's own program to end, then types the marker and reports whether the
// shell ran it. The transcript comes back for the failure message, because
// "the shell was still there" is only useful beside what the terminal said.
func lxeShellSurvived(t *testing.T, line string) (bool, string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("no bash on this host, so the shell under test is not here: %v", err)
	}
	master, slave := wwPty(t)
	cmd := exec.Command(bash, "--norc", "--noprofile", "-i")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	// Setsid + Setctty: the shell has to be a session leader with this pty
	// as its CONTROLLING terminal, or its job control is off and the SIGINT
	// row below measures nothing. Ctty is the child's fd, which is stdin.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "TERM=dumb", "PS1=", "HOME=" + t.TempDir()}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start an interactive bash on the pty: %v", err)
	}
	// ONE pid, killed by pid, and never a pattern: every seat on this box
	// runs a byte-identical `bash --norc -i` (AGENTS.md, "Ending anything").
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	var mu sync.Mutex
	var seen strings.Builder
	go func() {
		b := make([]byte, 4096)
		for {
			n, err := master.Read(b)
			if n > 0 {
				mu.Lock()
				seen.Write(b[:n])
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	read := func() string {
		mu.Lock()
		defer mu.Unlock()
		return seen.String()
	}

	// The shell has to have the terminal before anything is typed at it —
	// the same "a fresh pane is still reading its rc files" problem
	// paneline.go measured, in miniature.
	time.Sleep(300 * time.Millisecond)
	// Write errors are not failures here: a line that replaced the shell and
	// then ended leaves the slave with no reader, and EIO is that working.
	_, _ = master.WriteString(line + "\n")
	time.Sleep(900 * time.Millisecond)
	_, _ = master.WriteString(lxeMarkerCmd + "\n")
	time.Sleep(600 * time.Millisecond)
	out := read()
	if !strings.Contains(out, "LAUNCHED") {
		t.Fatalf("the line under test never ran at all, so nothing here is about the shell outliving it:\n%s\n--- terminal ---\n%s", line, out)
	}
	return strings.Contains(out, lxeMarker), out
}

// lxeRuntime is a stand-in for a seat's runtime: it says it started, waits
// long enough that the launch has certainly begun, and then dies the way the
// row asks. `kill -INT $$` is the whole point — a runtime killed by a key or
// by the watcher, rather than one that exits.
func lxeRuntime(how string) string {
	return `/bin/sh -c 'printf LAUNCHED\\n; sleep 0.2; ` + how + `'`
}

// Arm 1: the exec'd line, over the death that defeats a trailing exit. The
// assignment prefix is in the row on purpose: posse's exec goes ON the
// command word, after the assignments, and this is the live proof that the
// shell is still replaced when it does (the static half is MEASURED in
// launchexec.go's head).
func TestLiveLaunchExecLeavesNoShell(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"kill -INT $$", "exit 3", "kill -TERM $$"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()
			line, wrapped := ExecLaunchLine(`LX='seat' ` + lxeRuntime(how))
			if wrapped {
				t.Fatalf("fixture: this row went through the sh -c fallback, so it is not the shape posse renders: %s", line)
			}
			alive, out := lxeShellSurvived(t, line)
			if alive {
				t.Errorf("the pane's shell outlived a runtime that died by `%s` and ran the next thing typed at the seat — the launch line did not replace it (ranger-base-1l1en):\nline: %s\n--- terminal ---\n%s", how, line, out)
			}
		})
	}
}

// Arm 2: the form this bead rejected, kept as the negative case. If this
// stops reporting a surviving shell, bash's behaviour has changed and the
// WHY in launchexec.go is stale — which is a thing to know, not a thing to
// delete. A green arm 1 with a green-by-accident arm 2 would be a pin that
// cannot tell a working exec from a shell that happens to exit.
func TestLiveATrailingExitIsNotEnoughWhenTheRuntimeIsSignalled(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		how  string
		want bool // does the shell survive?
	}{
		{"kill -INT $$", true},
		{"exit 3", false},
	} {
		t.Run(c.how, func(t *testing.T) {
			t.Parallel()
			line := `LX='seat' ` + lxeRuntime(c.how) + `; exit`
			alive, out := lxeShellSurvived(t, line)
			if alive == c.want {
				return
			}
			if c.want {
				t.Errorf("`cmd; exit` no longer leaves a shell behind when cmd dies of SIGINT — bash 5.3.20 did (MEASURED 2026-11-14), and that measurement is the whole reason posse execs instead. Re-measure and correct launchexec.go rather than deleting this row:\nline: %s\n--- terminal ---\n%s", line, out)
				return
			}
			t.Errorf("`cmd; exit` left a shell behind for a runtime that EXITED, which is the row that used to work — so the row above proves less than it claims:\nline: %s\n--- terminal ---\n%s", line, out)
		})
	}
}

// Arm 3: the spilled form. In production the launch is usually NOT typed —
// every persona's line measured at 1867-2058 bytes against the 1023 limit
// (paneline.go) — so the pane sources a script, and `exec` inside a sourced
// file has to replace the SOURCING shell for any of this to hold there.
// Nothing in the static pins can say that; this is the arm that does.
func TestLiveASourcedLaunchScriptLeavesNoShellEither(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	// Over PaneLineMax so PaneLine spills it, and the padding rides in an
	// assignment so the line stays one simple command.
	cmd := `LX='` + strings.Repeat("x", PaneLineMax) + `' ` + lxeRuntime("kill -INT $$")
	line, err := b.App.PaneLine("lxe", cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, ". ") {
		t.Fatalf("fixture: the line was typed (%d bytes), so this is not the sourced form: %s", len(line), line)
	}
	alive, out := lxeShellSurvived(t, line)
	if alive {
		t.Errorf("the pane sourced its launch script and the sourcing shell is still there reading the terminal (ranger-base-1l1en):\nline: %s\n--- terminal ---\n%s", line, out)
	}
}

// Arm 4: the assignments still reach the runtime across the exec. This is
// the property a misplaced `exec` would have cost silently — a seat launched
// with no gate PATH, no runtime home and no credential dir — and it is worth
// one live row because the static pin can only say where the word went.
func TestLiveTheAssignmentPrefixSurvivesTheExec(t *testing.T) {
	t.Parallel()
	line, wrapped := ExecLaunchLine(`LX_SEAT='carried' /bin/sh -c 'printf LAUNCHED\\n; printf "LX=%s\\n" "$LX_SEAT"'`)
	if wrapped {
		t.Fatalf("fixture went through the sh -c fallback: %s", line)
	}
	_, out := lxeShellSurvived(t, line)
	if !strings.Contains(out, "LX=carried") {
		t.Errorf("the exec'd launch did not carry its assignment prefix into the runtime's environment — a seat would launch with no gate PATH and no credential dir:\nline: %s\n--- terminal ---\n%s", line, out)
	}
}
