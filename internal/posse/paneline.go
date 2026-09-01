package posse

// How long a line posse may type into a pane (rangerhq-ybec).
//
// A launch is a command typed into a shell, and `herdr pane run` puts it
// through the pane's tty exactly as a keyboard would. A freshly created
// workspace has not started its ZLE yet — the shell is still reading its rc
// files — so the tty is in CANONICAL mode, and the line discipline's
// per-line buffer is MAX_CANON, 1024 bytes including the newline that would
// submit it (sys/syslimits.h). Over that, the head is echoed raw by the tty,
// the tail sits in the buffer without its newline, and nothing ever runs;
// the next thing typed is appended to the leftover.
//
// Measured on this host (herdr 0.8.0, macOS 25.4, zsh), a single line typed
// into the pane `herdr workspace create` just returned:
//
//	1023 bytes  3/3 ran        1024 bytes  0/3 ran
//	1500 bytes  0/3 ran        (the same 1500 bytes, 3s later: 3/3)
//
// Waiting is not the fix, twice over. `herdr pane process-info` reports a
// shell_pid from the first sample and a foreground group that is the shell
// alone within 0.03-0.35s, and a line typed the instant that predicate goes
// true is still lost 5/5 — there is no observable for "ZLE has the tty".
// And a settled pane is not unbounded either: re-measured 2026-08-27 on a
// pane three seconds old, 24000 bytes runs 3/3 and 28000 bytes runs 0/3.
// (2026-08-25 put that bound at 16000/20000; the fresh-pane cliff above is
// MAX_CANON and has not moved, but the settled bound is not a documented
// constant and drifts — what holds is that one is there.) A line long
// enough is lost no matter how long anyone waits.
//
// So the line stays short instead. Over the limit it is written to
// state/launch/<session>.sh and the pane runs `. <path>` — sourced, not
// `sh <path>`, so the tty's foreground process is the runtime itself and
// not a shell holding it: `. ` reproduces typing the line, which is what
// every other part of posse (herdr's argv0 detection, RelaunchAgent typing
// into the surviving shell, a kill) already rests on. Verified live: a
// 20000-byte line spilled this way runs 3/3 on a pane one instant old, and
// `pane process-info` shows the launched process directly under the pane's
// shell, exactly as when the line is typed whole.
//
// rangerhq-9fv met this first with the container tier's ~1.6KB engine line
// and worked around it there; this is the same move made once, for every
// launch, so the next thing that grows a line — another deny rule, a longer
// --settings, more mounts — does not walk back into it.

import (
	"os"
	"path/filepath"
	"strings"
)

// PaneLineMax is the longest command posse types into a pane directly. The
// measured cliff is 1024 bytes of line plus its newline; this is the last
// length that survives it.
const PaneLineMax = 1023

// LaunchDir holds the spilled launch lines, one file per session.
func (a *App) LaunchDir() string { return filepath.Join(a.StateDir, "launch") }

// LaunchScript is where a session's line lands when it is too long to type.
func (a *App) LaunchScript(session string) string {
	return filepath.Join(a.LaunchDir(), session+".sh")
}

// PaneLine returns the line to type for cmd: cmd itself when it fits, else
// a `. <script>` that runs it. A line that fits is typed verbatim, because
// the pane's scrollback and herdr's log are where an operator reads what a
// session was launched with — so the typed line is the one worth keeping,
// and the spill is what the limit costs.
//
// Typing is the exceptional path now, not the usual one. The headroom a
// crew line once had is spent: measured 2026-09-01, every persona's line
// rendered to 1867-2058 bytes against the 1023 limit, so all seven spilled.
// A populated state/launch/ is therefore the expected state, not a symptom
// — it is this fallback working. That is one host on one day, not an
// invariant; NOTES.md ("The launch line is typed, so it has a limit") keeps
// the accounting of what spent the headroom, and is the copy to correct
// when it moves again.
//
// The script is rewritten at every launch and removed the moment a line
// fits again: a rendering left behind that nothing runs is how the next
// debugging session gets misled (the same care cagelauncher.go takes with
// its superseded launch.sh).
func (a *App) PaneLine(session, cmd string) (string, error) {
	script := a.LaunchScript(session)
	if len(cmd) <= PaneLineMax {
		if err := os.Remove(script); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return cmd, nil
	}
	if err := os.MkdirAll(a.LaunchDir(), 0o755); err != nil {
		return "", err
	}
	// 0600: the line is rendered from the PID and carries no credential
	// (env crosses a launch as names, never values), but a PID's own
	// `command:` is the one template posse did not write, and this file is
	// not the place to widen whatever it put there.
	body := "# posse launch line for session " + oneLine(session) + " — rendered at launch, do not edit.\n" +
		"# The pane sources this instead of typing it: a line this long is lost in a\n" +
		"# freshly created pane, whose tty is still in canonical mode (rangerhq-ybec).\n" +
		cmd + "\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		return "", err
	}
	return ". " + shellQuote(script), nil
}

// DropPaneLine removes a session's spilled line. Called where the session's
// meta is removed, for the same reason.
func (a *App) DropPaneLine(session string) {
	os.Remove(a.LaunchScript(session))
}

// oneLine keeps a name that reached us from outside from breaking out of
// the comment it is written into.
func oneLine(s string) string {
	return strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
}
