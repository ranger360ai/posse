package rhq

// ranger-base-gu9z: ADR 0013 section 2 says an interstitial whose default
// action mutates the machine is a launch refusal until the operator-owned
// config silences it. Codex's update prompt is that case: its selected item
// runs `brew upgrade --cask codex`.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQADangerousCodexInterstitialRefusesDispatchUntilSilenced(t *testing.T) {
	t.Skip("ranger-base-9r33: dispatch does not enforce the declared dangerous-interstitial launch refusal")

	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)

	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	if err := os.WriteFile(filepath.Join(codexHome, "version.json"),
		[]byte(`{"latest_version":"0.150.0","dismissed_version":"0.149.1"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: codex\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 {
		t.Errorf("dangerous unsilenced interstitial dispatched %d bead(s):\n%s", n, out)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("codex was launched behind an expired update dismissal:\n%s\n%s", log, out)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("bead was claimed before the launch refusal:\n%s\n%s", log, out)
	}
	for _, want := range []string{"dismissed_version", "latest_version", "launch"} {
		if !strings.Contains(out, want) {
			t.Errorf("refusal must name %q and how to clear it:\n%s", want, out)
		}
	}
}
