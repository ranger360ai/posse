package rhq

// Live pin for rangerhq-7sbo / rangerhq-aw9n, narrowed by rangerhq-6723:
// grok is dispatchable with its startup splash on screen, and now without a
// key being pressed at it. The hermetic cases in startupscreen_test.go run
// against a fake herdr that this repo also writes, so they cannot catch the
// three things only a real herdr knows — that `agent wait --until blocked`
// is a valid state, that `agent explain --json` really emits a top-level
// "state" beside "matched_rule".id, and that a settled pane's explain
// carries the visible_idle the readiness gate demands (rangerhq-3hb5).
// This one runs awaitAgent against a real pane; bootrace_live_test.go runs
// it against a pane that is still booting.
//
// It skips unless pointed at one. Recipe (QA, 2026-08-21):
//
//	herdr --session <s> server &                       # scratch, not the fleet
//	export HERDR_SOCKET_PATH=~/.config/herdr/sessions/<s>/herdr.sock
//	herdr workspace create --cwd <scratch> --no-focus  # note the pane id
//	herdr pane run <pane> "grok --permission-mode auto"   # GrokFleetFlags
//	RHQ_LIVE_PANE=<pane> go test ./internal/rhq -run TestLiveAwaitAgent -v
//
// It never prompts: awaitAgent stops at a promptable target, so this costs
// no agent turn. Tear down with `herdr workspace close` + `herdr server stop`.
//
// The pane may be sitting on the splash or on a bare composer; since
// rangerhq-1xsj made the splash a seen `idle` both are the same case, and
// both must come back untouched and silent. That is the assertion no fake
// can make: `blocked` is still in the settle wait, and only a real herdr can
// show that a live splash does not land there.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// liveBackend points posse at one live herdr pane: a hand-written meta, not a
// real session, because these tests drive awaitAgent and launch nothing
// themselves. Sessions() prunes a meta with no socket:, hence the socket.
func liveBackend(t *testing.T, pane string) *HerdrBackend {
	t.Helper()
	bin := os.Getenv("RHQ_HERDR_BIN")
	if bin == "" {
		bin = "herdr"
	}
	home := t.TempDir()
	a := &App{
		Home: home, ConfigPath: filepath.Join(home, "config.yaml"),
		RecipesDir: filepath.Join(home, "recipes"), EnvsDir: filepath.Join(home, "envs"),
		StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"),
	}
	os.WriteFile(a.ConfigPath, []byte("beads: []\n"), 0o644)
	os.MkdirAll(filepath.Join(a.StateDir, "herdr"), 0o755)
	os.WriteFile(filepath.Join(a.StateDir, "herdr", "qalive.yaml"), []byte(fmt.Sprintf(
		"name: qalive\nworkspace: %s\npane: %s\nemoji: x\nsocket: %s\n",
		strings.SplitN(pane, ":", 2)[0], pane, os.Getenv("HERDR_SOCKET_PATH"))), 0o644)
	b := &HerdrBackend{App: a, H: Herdr{Bin: bin}}
	captureWarn(t, b)
	return b
}

func TestLiveAwaitAgentAcceptsAStartupScreen(t *testing.T) {
	pane := os.Getenv("RHQ_LIVE_PANE")
	if pane == "" {
		t.Skip("set RHQ_LIVE_PANE=<ws:pane> (+ HERDR_SOCKET_PATH, RHQ_HERDR_BIN) — see the file comment")
	}
	b := liveBackend(t, pane)
	a := b.App
	before, err := b.H.AgentExplain(pane)
	if err != nil {
		t.Fatalf("herdr agent explain %s: %v (is the pane live?)", pane, err)
	}
	t.Logf("pane %s: state=%q rule=%q seen=%v", pane, before.State, before.Rule.ID, before.Seen())

	var out strings.Builder
	d := NewDispatcher(a, b, &out)
	d.StartupWait = 20 * time.Second
	d.Poll = 500 * time.Millisecond

	start := time.Now()
	target, err := d.awaitAgent("live-7sbo", "qalive", d.StartupWait)
	t.Logf("awaitAgent: %s target=%q err=%v\n%s", time.Since(start), target, err, out.String())
	if err != nil {
		t.Fatalf("awaitAgent refused a live pane herdr calls %q/%q: %v", before.State, before.Rule.ID, err)
	}
	if target != pane {
		t.Fatalf("target %q != %q", target, pane)
	}

	// A settled agent — splash drawn or not — is prompted, never typed at.
	// Silence is the assertion: since rangerhq-6723 there is no line for
	// awaitAgent to print on a pane it accepts, and no key for it to press.
	if out.Len() != 0 {
		t.Errorf("a settled agent must be dispatched in silence, and %q is on screen:\n%s", before.Rule.ID, out.String())
	}
}
