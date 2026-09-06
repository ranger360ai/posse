//go:build !posse_arm2 && !posse_arm3

package posse

// Live pin for rangerhq-ybec: the length at which a typed launch is lost.
// paneline_test.go proves what posse *does* with the limit; only a real
// herdr and a real tty can say the limit is there at all, and this is the
// measurement the constant is set from.
//
//	herdr --session ybec server &                      # scratch, not the fleet
//	export HERDR_SOCKET_PATH=~/.config/herdr/sessions/ybec/herdr.sock
//	RHQ_LIVE_PANE_LINE=1 go test ./internal/rhq -run TestLivePaneLine -v
//	herdr server stop
//
// It creates and closes its own workspaces, types only `printf` at them, and
// spends no API turn. Run it against a scratch server if you value your
// fleet's window layout; nothing here reads or writes another pane.
//
// Measured 2026-08-25, macOS 25.4, herdr 0.8.0, zsh 5.9 — on the pane
// `workspace create` had just returned:
//
//	  1023 B typed  3/3 ran      1024 B typed  0/3 ran
//	  1500 B typed  0/3 ran      1500 B spilled to a file  3/3 ran
//	 20000 B typed  0/3 ran     20000 B spilled to a file  3/3 ran
//
// and on a pane left to settle for three seconds first: 1024 B 3/3, 5000 B
// 3/3 — bounded too, only further out, which is why waiting is not the fix.
// Re-measured 2026-08-27 (ranger-base-82u): the fresh-pane cliff is
// unchanged (1022/1023 B 3/3, 1024 B 0/3, 1500 B 0/3, 20000 B spilled 3/3),
// but the settled bound had moved — 20000 B and 24000 B now run 3/3 and
// 28000 B runs 0/3, where 2026-08-25 saw 16000 B 2/2 and 20000 B 0/3. Only
// the fresh-pane cliff is a documented constant (MAX_CANON); the settled
// bound drifts, so this test pins the cliff and not that number.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// liveEcho is a command of exactly n bytes that prints how long its payload
// was — so a pane that ran it says so, and a pane that swallowed it is
// silent rather than ambiguous.
func liveEcho(n int) (cmd, want string) {
	head, tail := "X='", `'; printf 'GOT_%s\n' ${#X}`
	body := strings.Repeat("x", n-len(head)-len(tail))
	return head + body + tail, fmt.Sprintf("GOT_%d", len(body))
}

// liveRan types line into a pane created this instant and reports whether
// want ever appeared on it.
func liveRan(t *testing.T, b *HerdrBackend, want string, line func(pane string) string) bool {
	t.Helper()
	ws, pane, err := b.H.CreateWorkspace("ybec-live", os.TempDir(), nil)
	if err != nil {
		t.Fatalf("workspace create: %v", err)
	}
	defer b.H.CloseWorkspace(ws)
	if err := b.H.PaneRun(pane, line(pane)); err != nil {
		t.Fatalf("pane run: %v", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for {
		txt, err := b.H.PaneRead(pane, 0)
		if err != nil {
			t.Fatalf("pane read: %v", err)
		}
		if strings.Contains(txt, want) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestLivePaneLineLimitAndTheSpillThatClearsIt(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_PANE_LINE") == "" {
		t.Skip("set RHQ_LIVE_PANE_LINE=1 (+ HERDR_SOCKET_PATH, RHQ_HERDR_BIN) — see the file comment")
	}
	b := liveBackend(t, "w0:p0") // only for its App and its herdr binary

	// The limit is where the behaviour changes, so both sides of it are the
	// measurement. One byte under: typed whole, and it runs.
	cmd, want := liveEcho(PaneLineMax)
	if !liveRan(t, b, want, func(string) string { return cmd }) {
		t.Errorf("PaneLineMax is set too high: %d bytes typed into a fresh pane did not run", PaneLineMax)
	}

	// One byte over: the bug itself. If this ever runs, the tty grew a
	// bigger canonical buffer and the constant can be re-measured — it has
	// not become wrong, only conservative.
	cmd, want = liveEcho(PaneLineMax + 1)
	if liveRan(t, b, want, func(string) string { return cmd }) {
		t.Errorf("%d bytes typed into a fresh pane ran — re-measure PaneLineMax on this host", PaneLineMax+1)
	}

	// And the fix, at a length no amount of waiting would have saved: 20000
	// bytes is lost on a settled pane too.
	cmd, want = liveEcho(20000)
	ran := liveRan(t, b, want, func(pane string) string {
		line, err := b.App.PaneLine("ybec-live", cmd)
		if err != nil {
			t.Fatalf("PaneLine: %v", err)
		}
		if len(line) > PaneLineMax {
			t.Fatalf("the spilled line is itself too long: %d bytes", len(line))
		}
		return line
	})
	if !ran {
		t.Errorf("a %d-byte launch spilled to %s did not run", len(cmd), b.App.LaunchScript("ybec-live"))
	}
}
