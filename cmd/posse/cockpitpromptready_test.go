package main

// ranger-base-k99a: the cockpit's `p` was the third typed-prompt caller and
// the one ranger-base-3p0 deliberately left alone. Same entry point, same
// race — the operator dispatches, sees the row appear and presses p before
// the CLI has taken the keyboard — but the fix could not simply be wired in
// where it was on `posse prompt`: rhq.AwaitPromptable holds for the
// session's runtime startup wait (45s, claude), and the key handler runs on
// the render loop. A straight call would freeze the whole cockpit for the
// duration of exactly the case the gate exists for.
//
// So the prompt runs off the event loop, in c.launch's shape. These pin
// both halves: the loop stays live, and the gate still refuses.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/rhq"
)

// promptGateCockpit builds a one-session cockpit over a shell fake herdr
// whose `agent explain` answers `guess` with herdr's GUESS shape
// (matched_rule null, visible_idle false, default_known_agent_idle_fallback
// — rangerhq-3hb5) and anything else with a seen screen. The session's
// runtime declares wait, which is the gate's patience (promptReadyWait), so
// a refusal costs that instead of the claude-shaped 45s.
func promptGateCockpit(t *testing.T, explain, wait string) (*cockpit, string) {
	t.Helper()
	home := t.TempDir()
	binDir := t.TempDir()
	herdr := filepath.Join(binDir, "herdr")
	log := filepath.Join(home, "calls.log")
	script := `#!/bin/sh
echo "$@" >> ` + log + `
case "$1 $2" in
"workspace list")
  printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1","label":"fresh","agent_status":"idle"}]}}'
  exit 0;;
"agent list")
  printf '%s\n' '{"result":{"agents":[{"agent":"claude","agent_status":"idle","pane_id":"p1","workspace_id":"w1"}]}}'
  exit 0;;
"agent prompt")
  printf '%s\n' '{"result":{"submitted":true}}'
  exit 0;;
"agent explain")
  case "` + explain + `" in
  guess)
    printf '%s\n' '{"state":"idle","matched_rule":null,"visible_idle":false,"fallback_reason":"default_known_agent_idle_fallback","evaluated_rules":[{"id":"live_prompt_box","matched":false,"region":"osc_title","state":"idle","evidence":{"region_bytes":0,"region_preview":""}}]}'
    exit 0;;
  esac
  printf '%s\n' '{"state":"idle","matched_rule":{"id":"live_prompt_box"},"visible_idle":true,"fallback_reason":null}'
  exit 0;;
esac
printf '%s\n' '{"error":{"code":"no","message":"unexpected"}}'
exit 1
`
	if err := os.WriteFile(herdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	metaDir := filepath.Join(home, "state", "herdr")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "fresh.yaml"),
		[]byte("name: fresh\nworkspace: w1\npane: p1\nruntime: fastcli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &rhq.App{
		Home:       home,
		ConfigPath: filepath.Join(home, "config.yaml"),
		StateDir:   filepath.Join(home, "state"),
	}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "fastcli.yaml"),
		[]byte("command: fastcli --sys {file}\nstartup_wait: "+wait+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// ServerGen() stats the operator's live herdr socket unless this points
	// it somewhere else (ranger-base-ouf9): a test must not read the box.
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(home, "no-such.sock"))

	c := &cockpit{
		app:      a,
		hb:       &rhq.HerdrBackend{App: a, H: rhq.Herdr{Bin: herdr}, Warn: io.Discard},
		prompts:  make(chan string, 4),
		progress: make(chan string, 4),
	}
	c.refresh()
	c.buildRows()
	if len(c.sessions) != 1 {
		t.Fatalf("want one session row, got %+v", c.sessions)
	}
	return c, log
}

func pressKeys(t *testing.T, c *cockpit, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, err := c.handleKey([]byte(k)); err != nil {
			t.Fatal(err)
		}
	}
}

// The reason this could not be a one-line call. The gate is holding — the
// pane is one herdr only guesses at, which is the whole 3-second patience
// here — and the keystroke that started it has already returned. A cockpit
// that called AwaitPromptable inline would sit inside handleKey for that
// entire window with no draw, no refresh and no q.
func TestCockpitPromptDoesNotFreezeTheRenderLoop(t *testing.T) {
	c, _ := promptGateCockpit(t, "guess", "3s")

	start := time.Now()
	pressKeys(t, c, "p", "h", "i", "\r")
	handled := time.Since(start)

	if handled > time.Second {
		t.Errorf("the keystroke held the render loop for %s — the prompt must run off it", handled)
	}
	if !c.prompting {
		t.Error("nothing is in flight after enter — the prompt never started")
	}
	if !strings.Contains(c.status, "prompting fresh") {
		t.Errorf("the operator is owed a line while it waits: %q", c.status)
	}
	// ...and a second p does not open a second one over the top of it.
	c.status = ""
	pressKeys(t, c, "p")
	if c.mode != modeNormal || !strings.Contains(c.status, "already in flight") {
		t.Errorf("p during an in-flight prompt: mode %v status %q", c.mode, c.status)
	}

	select {
	case msg := <-c.prompts:
		if !strings.Contains(msg, "nothing was sent") {
			t.Errorf("the gate opened on a guess: %q", msg)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the prompt goroutine never reported")
	}
}

// What the gate is for, at this entry point: a pane herdr only guesses at
// is not typed into, and the refusal lands on the status line where the
// operator is looking — the incident's mangled prompt reported success.
func TestCockpitPromptRefusesAPaneHerdrOnlyGuessesAt(t *testing.T) {
	c, log := promptGateCockpit(t, "guess", "600ms")

	pressKeys(t, c, "p", "h", "i", "\r")
	msg := ""
	select {
	case msg = <-c.prompts:
	case <-time.After(30 * time.Second):
		t.Fatal("the prompt goroutine never reported")
	}

	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "agent prompt") {
		t.Errorf("text was typed into a pane herdr only guessed at:\n%s", calls)
	}
	for _, want := range []string{"nothing was sent", "fresh", "default_known_agent_idle_fallback"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal must name %q: %q", want, msg)
		}
	}
}

// The wrong arm. A settled pane — a named rule, herdr's seen shape — is
// prompted exactly as it was before the gate, and the crew mark still goes
// down (ADR 0008). A gate that refused here would have taken `p` away.
func TestCockpitPromptStillSendsToAPaneHerdrHasSeen(t *testing.T) {
	c, log := promptGateCockpit(t, "seen", "600ms")

	start := time.Now()
	pressKeys(t, c, "p", "h", "i", "\r")
	msg := ""
	select {
	case msg = <-c.prompts:
	case <-time.After(30 * time.Second):
		t.Fatal("the prompt goroutine never reported")
	}
	if waited := time.Since(start); waited >= 600*time.Millisecond {
		t.Errorf("an established session paid %s at the gate — it costs one explain", waited)
	}
	if msg != "prompted fresh" {
		t.Errorf("status %q, want %q", msg, "prompted fresh")
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "agent prompt p1 hi") {
		t.Errorf("the prompt never went in:\n%s", calls)
	}
	meta, err := os.ReadFile(filepath.Join(c.app.StateDir, "herdr", "fresh.yaml"))
	if err != nil || !strings.Contains(string(meta), "crew: true") {
		t.Errorf("the crew mark was not recorded (%v): %s", err, meta)
	}
}

// A channel is only a status line if something drains it, and the drain is
// one case in runCockpit's select — which no test runs, because it wants a
// raw-mode terminal (the same pin TestCockpitEventLoopDrainsProgress makes,
// for the same reason). Without it the prompt's result lands in a buffer
// nobody reads, c.prompting is never cleared, and `p` is dead for the rest
// of the session while every test above stays green.
func TestCockpitEventLoopDrainsPrompts(t *testing.T) {
	src, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"case msg := <-c.prompts:", "c.prompting = false"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("runCockpit's event loop does not %s — nothing drains the prompt channel", want)
		}
	}
}
