//go:build posse_arm3

package posse

// Verifying ranger-base-1mu9r's close (verify bead ranger-base-x8mzn).
//
// The close names three surfaces that stop blaming a missing image for a
// missing engine: `posse cage`'s status line, the four live-test guards, and
// the LAUNCH REFUSAL in WrapInCage. The first two are measurable from outside
// — the status line was re-run here against a scratch RHQ_HOME, and the eight
// live guards were re-run and skip with the new sentence — and the close's own
// pin (TestCageNotReadyNamesTheEngineAndNotTheImage) grades CageNotReady and
// CageEngineNotReady, both mutation-checked here.
//
// The launch refusal was not graded by anything. Measured 2026-09-02 on the
// close's own tree: delete WrapInCage's dead-engine arm outright — make the
// `if !a.CageEngineLive(e)` at cage.go:1035 unreachable — and
// `go test ./internal/posse -run 'Cage|Wrap|Launch'` is still ok in 106.6s.
// So the surface an operator actually meets when a dispatched launch refuses
// could go back to the pre-1mu9r sentence and no test would say so.
//
// This is that missing arm, and it is deliberately the whole discrimination
// rather than one assertion: the same engine, the same failing image probe,
// with `live:` the only thing that differs between the two runs. An engine
// that answers gets the image sentence; an engine that does not gets the
// engine sentence and no build advice. A pin that only asserted the dead case
// would pass just as well on code that said "engine" every time.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQALaunchRefusalNamesTheEngineAndNotTheImage(t *testing.T) {
	t.Parallel()

	// `env` and `true`/`false` lead every template: all three are on PATH and
	// none of them runs a container, so this stays hermetic on a box with no
	// engine — which is the only box this crew has (ranger-base-6mz7).
	launch := func(t *testing.T, live string) string {
		t.Helper()
		a := cageApp(t)
		if err := os.WriteFile(filepath.Join(a.CagesDir(), "fake.yaml"),
			[]byte("command: env {mounts} {env} -w {workdir} {image} {cmd}\n"+
				"probe: false {image}\n"+live), 0o644); err != nil {
			t.Fatal(err)
		}
		ag := cageAgent(t, a, "")
		rt, err := a.LoadRuntime("claude")
		if err != nil {
			t.Fatal(err)
		}
		line, err := a.WrapInCage(ag, rt, "s1", t.TempDir(), "claude",
			[]string{"CLAUDE_CODE_OAUTH_TOKEN"}, "")
		if err == nil {
			t.Fatalf("a failing image probe must refuse the launch, not return a line: %s", line)
		}
		return err.Error()
	}

	// The engine answers; the image really is absent. Unchanged by 1mu9r, and
	// it is the arm that makes the next one mean something.
	if got := launch(t, "live: true\n"); !strings.Contains(got, "is not built") ||
		!strings.Contains(got, "posse cage build") {
		t.Errorf("a live engine with no image must still be told to build it: %q", got)
	}

	// The bug: nothing answered the probe, so the probe said nothing about the
	// image. The refusal must name the engine, and must not send the operator
	// at a build that runs through the same engine.
	dead := launch(t, "live: false\n")
	if !strings.Contains(dead, "fake") || !strings.Contains(dead, "nothing answers it") {
		t.Errorf("a dead engine must be named as the reason the launch refused: %q", dead)
	}
	if strings.Contains(dead, "is not built") || strings.Contains(dead, "posse cage build)") {
		t.Errorf("a dead engine must not be reported as a missing image, and must not advise a build that needs the engine: %q", dead)
	}
}
