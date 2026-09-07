//go:build posse_arm3

package posse

// rangerhq-mgvx: the governance surface must stay honest when the thing it
// monitors is dead. scripts/verify-govern-honesty.sh is the live pin — a
// scratch --session herdr and a scratch RHQ_HOME, never the fleet — and this
// file pins that the script still asserts the arms that make it a probe
// rather than a sticker.
//
// The claim, from the governance-surface ADR §2: "the view does not depend
// on the loop — `posse status` reads the stores directly and reports G7
// itself, via the flock probe (release *is* death, no staleness class).
// What dies with the loop is *delivery* only."
//
// Live run (scratch session; the fleet's socket is only compared against):
//
//	scripts/verify-govern-honesty.sh
//	RHQ_LIVE_GOVERN_HONESTY=1 go test ./internal/posse -run TestQAGovernHonestyScriptPassesAgainstAScratchServer -v
//
// Measured 2026-08-27 on bin/posse-go: 17/17 PASS. The same script against
// the binary built from the commit before the header fix fails
// alive-cockpit-header-clear and dead-cockpit-header-says-loop-dead — a pin
// that survives the absence of its fix pins nothing (ranger-base-flz7).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func governHonestyScript(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "verify-govern-honesty.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestQAGovernHonestyScriptKeepsItsSafeDirection(t *testing.T) {
	t.Parallel()
	s := governHonestyScript(t)
	// The rangerhq-snd wipe was a real RHQ_HOME against a scratch socket.
	// Scratch on BOTH sides, and the fleet socket compared against, never
	// commanded.
	for _, needle := range []string{
		"unset HERDR_ENV HERDR_SOCKET_PATH",
		"FLEET_SOCK",
		"REFUSING",
		`env RHQ_HOME="$HOMEDIR" "$HERDR" --session "$SESSION" server`,
		"session stop",
		"session delete",
		"autostart_dry_run: true",
	} {
		if !strings.Contains(s, needle) {
			t.Errorf("verify-govern-honesty.sh no longer contains %q — the rig can reach the fleet", needle)
		}
	}
}

func TestQAGovernHonestyScriptPinsEveryArm(t *testing.T) {
	t.Parallel()
	s := governHonestyScript(t)
	for _, needle := range []string{
		// The control arm. Without it the rest is a sticker.
		"alive-status-exit-zero",
		"alive-status-clear",
		"alive-no-G7",
		"alive-cockpit-header-clear",
		"alive-cockpit-no-loop-dead",
		// The bead's own done-when: G7 shows, non-zero exit, the header says it.
		"dead-status-exit-nonzero",
		"dead-status-G7-urgent",
		"dead-status-names-delivery",
		"dead-cockpit-header-says-loop-dead",
		"dead-cockpit-block-has-G7",
		// The husk check the flock retired must not be able to answer.
		"stale-pidfile-still-G7",
		// The arm gate — and what makes the dead arm's exit code G7's doing.
		"disarmed-no-G7",
		"disarmed-exit-zero",
		// Delivery, and only delivery, dies with the loop.
		"pulse-writes-while-alive",
		"pulse-stops-with-the-loop",
		"view-outlives-the-loop",
	} {
		if !strings.Contains(s, needle) {
			t.Errorf("verify-govern-honesty.sh no longer asserts %q", needle)
		}
	}
	// kill -9, not a signal the process can handle: release-is-death is the
	// whole argument for the flock over a pidfile (rangerhq-gir5), and a
	// graceful stop would not test it.
	if !strings.Contains(s, "kill -9") {
		t.Error("the loop must be killed uncleanly, or the lock's own release path is what got tested")
	}
}

func TestQAGovernHonestyScriptPassesAgainstAScratchServer(t *testing.T) {
	t.Parallel()
	if os.Getenv("RHQ_LIVE_GOVERN_HONESTY") == "" {
		t.Skip("set RHQ_LIVE_GOVERN_HONESTY=1 to run scripts/verify-govern-honesty.sh against a scratch herdr session")
	}
	script := filepath.Join("..", "..", "scripts", "verify-govern-honesty.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "HERDR_SESSION=", "HERDR_SOCKET_PATH=", "HERDR_ENV=")
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("verify-govern-honesty.sh: %v", err)
	}
	if !strings.Contains(string(out), "verify-govern-honesty: PASS") {
		t.Fatalf("script exited 0 without PASS:\n%s", out)
	}
}
