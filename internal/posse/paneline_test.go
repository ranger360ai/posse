package posse

// The launch line's length (rangerhq-ybec). A command typed into a pane
// `herdr workspace create` just returned goes through a tty that is still in
// canonical mode, where MAX_CANON is 1024 bytes: over that the line is
// echoed and dropped, and the launch is simply gone. paneline.go carries the
// measurement; these are the cases that keep the launch sites honest about
// it, and the live pin is panelinelive_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// paneRunLines returns the raw, un-collapsed `pane run` lines the fake herdr
// was given — raw because the whole question here is how long the line is,
// and calls() rewrites the gate prefix to a marker.
func paneRunLines(t *testing.T, fake string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fake, "calls.log"))
	if err != nil {
		t.Fatalf("no call log: %v", err)
	}
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(ln, "pane run ") {
			continue
		}
		rest := strings.TrimPrefix(ln, "pane run ")
		_, cmd, ok := strings.Cut(rest, " ")
		if !ok {
			t.Fatalf("pane run with no command: %q", ln)
		}
		out = append(out, cmd)
	}
	return out
}

func TestPaneLineTypesWhatFitsVerbatim(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	cmd := "echo " + strings.Repeat("x", PaneLineMax-len("echo "))
	if len(cmd) != PaneLineMax {
		t.Fatalf("test setup: %d", len(cmd))
	}
	line, err := b.App.PaneLine("s", cmd)
	if err != nil {
		t.Fatal(err)
	}
	if line != cmd {
		t.Errorf("a line that fits must be typed as it reads, not sourced from a file:\n%s", line)
	}
	if _, err := os.Stat(b.App.LaunchScript("s")); !os.IsNotExist(err) {
		t.Errorf("nothing to spill, so nothing may be written: %v", err)
	}
}

// One byte past the cliff is the whole bug: 1023 ran 3/3 live and 1024 ran
// 0/3, so the boundary is where the behaviour changes, not near it.
func TestPaneLineSpillsTheFirstLineThatWouldBeLost(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	cmd := "echo " + strings.Repeat("x", PaneLineMax+1-len("echo "))
	line, err := b.App.PaneLine("s", cmd)
	if err != nil {
		t.Fatal(err)
	}
	script := b.App.LaunchScript("s")
	if line != ". "+shellQuote(script) {
		t.Errorf("the pane must source the spilled line, got %q", line)
	}
	if len(line) > PaneLineMax {
		t.Errorf("the typed line is itself too long: %d bytes", len(line))
	}
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "\n"+cmd+"\n") {
		t.Errorf("the spilled script must carry the command whole, on its own line:\n%s", body)
	}
	if !strings.Contains(string(body), "rangerhq-ybec") {
		t.Errorf("a rendered file an operator will find while debugging must say why it exists:\n%s", body)
	}
	fi, err := os.Stat(script)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("the launch line is rendered from the PID; keep it to its owner, got %v", fi.Mode().Perm())
	}
}

// A line that shrank back under the limit is typed again — and the script it
// used to need goes, because a rendering nothing runs is what misleads the
// next person to open state/.
func TestPaneLineRemovesASupersededScript(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	long := "echo " + strings.Repeat("x", PaneLineMax*2)
	if _, err := b.App.PaneLine("s", long); err != nil {
		t.Fatal(err)
	}
	line, err := b.App.PaneLine("s", "echo short")
	if err != nil {
		t.Fatal(err)
	}
	if line != "echo short" {
		t.Errorf("got %q", line)
	}
	if _, err := os.Stat(b.App.LaunchScript("s")); !os.IsNotExist(err) {
		t.Errorf("the superseded script must be removed: %v", err)
	}
}

// bigPersona writes a PID whose rendered line cannot be typed: the command:
// template alone is over the limit before the gate prefix goes on it. This
// is the container tier's ~1.6KB engine line in miniature, and the shape of
// everything that grows a line later — another deny rule, a longer
// --settings, more mounts.
func bigPersona(t *testing.T, b *HerdrBackend, name string) string {
	t.Helper()
	marker := "MARKER_" + name
	os.MkdirAll(b.App.AgentsDir, 0o755)
	md := "---\nname: " + name + "\ndescription: test\nlabels: [go]\n" +
		"command: claude --append-system-prompt \"$(cat {file})\" --add-dir {memory} --settings '" +
		marker + strings.Repeat("x", 1200) + "'\n---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, name+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return marker
}

func TestCreateSessionNeverTypesMoreThanAFreshPaneTakes(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	marker := bigPersona(t, b, "big")
	if err := b.CreateSession(NewSessionOpts{Name: "s1", Agent: "big", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	lines := paneRunLines(t, fake)
	if len(lines) != 1 {
		t.Fatalf("expected one typed line, got %d: %v", len(lines), lines)
	}
	if len(lines[0]) > PaneLineMax {
		t.Errorf("a %d-byte line typed into a pane created an instant ago is lost, not slow:\n%s", len(lines[0]), lines[0])
	}
	body, err := os.ReadFile(b.App.LaunchScript("s1"))
	if err != nil {
		t.Fatalf("the launch must exist somewhere: %v", err)
	}
	if !strings.Contains(string(body), marker) {
		t.Errorf("the spilled script is not this persona's line:\n%s", body)
	}
	// Everything the launch line carries still has to reach the session:
	// the gate prefix and the persona's own rendering, only in the file.
	if !strings.Contains(string(body), `:"$PATH" `) {
		t.Errorf("the gate prefix must ride the spilled line (ADR 0002 §3):\n%s", body)
	}
}

// The same at the relaunch site. Its pane's shell settled long ago, so
// canonical mode is not the wall there — but a line long enough is lost on a
// settled pane too, and one rule for both sites is one thing to keep true.
func TestRelaunchAgentSpillsTheLineToo(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	bigPersona(t, b, "big")
	if err := b.CreateSession(NewSessionOpts{Name: "s1", Agent: "big", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	m, _ := b.readMeta("s1")
	m.Launched = m.Launched.Add(-time.Hour)
	b.writeMeta(m)
	os.Remove(filepath.Join(fake, "agents.json"))
	if ok, err := b.RelaunchAgent("s1", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	lines := paneRunLines(t, fake)
	if len(lines) != 2 {
		t.Fatalf("expected the launch and the relaunch, got %d: %v", len(lines), lines)
	}
	for i, ln := range lines {
		if len(ln) > PaneLineMax {
			t.Errorf("typed line %d is %d bytes:\n%s", i, len(ln), ln)
		}
	}
	if lines[0] != lines[1] {
		t.Errorf("a relaunch re-types the same launch:\n%s\n%s", lines[0], lines[1])
	}
}

// The script dies with the session it launched, like the meta beside it.
func TestKillSessionDropsTheSpilledLine(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	bigPersona(t, b, "big")
	if err := b.CreateSession(NewSessionOpts{Name: "s1", Agent: "big", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.App.LaunchScript("s1")); err != nil {
		t.Fatalf("test setup: no spilled script: %v", err)
	}
	if err := b.KillSession("s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.App.LaunchScript("s1")); !os.IsNotExist(err) {
		t.Errorf("a killed session leaves no launch line behind: %v", err)
	}
}

// A line under the limit is typed whole, so the pane's scrollback and
// herdr's log say what the session was launched with. The fixture is a
// minimal PID deliberately — that is what makes a short line, and it is all
// this pins. It is NOT a claim about the crew: their lines were 352-677
// bytes when this was written, and measured 2026-09-01 they are 1867-2058
// against the 1023 limit and every one of them spills (ranger-base-q8ejb).
func TestOrdinaryPersonaLinesAreStillTypedWhole(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	if err := b.CreateSession(NewSessionOpts{Name: "s1", Agent: "ranger", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	lines := paneRunLines(t, fake)
	if len(lines) != 1 || strings.HasPrefix(lines[0], ". ") {
		t.Errorf("an ordinary persona line must not need the spill: %v", lines)
	}
	if _, err := os.Stat(b.App.LaunchScript("s1")); !os.IsNotExist(err) {
		t.Errorf("nothing spilled, nothing written: %v", err)
	}
}
