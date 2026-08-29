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

// codexMenuBack / codexMenuSilenced write the two readings of codex's
// version.json a launch can meet, and point CODEX_HOME at them. Named
// because the difference between them is three characters in a version
// string, and a test that inlines it reads as a fixture rather than as the
// fact it turns on: the dismissal is good for ONE release.
func codexMenuBack(t *testing.T) {
	t.Helper()
	writeCodexVersion(t, `{"latest_version":"0.150.0","dismissed_version":"0.149.1"}`)
}

func codexMenuSilenced(t *testing.T) {
	t.Helper()
	writeCodexVersion(t, `{"latest_version":"0.150.0","dismissed_version":"0.150.0"}`)
}

func writeCodexVersion(t *testing.T, body string) {
	t.Helper()
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		home = t.TempDir()
		t.Setenv("CODEX_HOME", home)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "version.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The other half of ADR 0013 §2's launch rule, and the half the dispatch
// test above cannot see. planLaunch is what every OTHER launch path passes
// through — `posse new`, a recipe, a relaunch, the cockpit's `d` on a
// session it has to create — and there the answer turns on who is watching:
//
//   - a launch carrying a bead is dispatch's, nobody is watching it, and it
//     refuses (the ADR 0015 §3 asymmetry, twelve lines above it in
//     planLaunch);
//   - an interactive launch warns DEGRADED and proceeds, because the
//     operator's remedy for codex's update menu is to ANSWER it in their own
//     codex session. A posse that refused that launch too would have walled
//     off the only way to clear its own refusal — which is also why there is
//     no config escape hatch to test for.
func TestQADangerousInterstitialRefusesABeadLaunchAndWarnsAnInteractiveOne(t *testing.T) {
	b, _ := newTestBackend(t)
	codexMenuBack(t)

	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: codex\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()

	_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger", Bead: "a-1"})
	if err == nil {
		t.Fatal("a bead-carrying launch onto an expired codex dismissal must refuse — nobody is watching it, and the menu's default option runs `brew upgrade --cask codex`")
	}
	for _, want := range []string{"dismissed_version", "latest_version", "refuse"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q and how to clear it: %v", want, err)
		}
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: dir, Agent: "ranger"}); err != nil {
		t.Fatalf("an interactive launch must PROCEED — answering that screen is what the operator opens a codex session to do: %v", err)
	}
	got := warn.String()
	for _, want := range []string{"DEGRADED", "dismissed_version", "latest_version"} {
		if !strings.Contains(got, want) {
			t.Errorf("an interactive launch that proceeds must still say what it is opening on (%q missing):\n%s", want, got)
		}
	}

	// And the silenced reading says nothing at all — the negative control
	// without which both assertions above pass for a rule that fires on
	// every codex launch whatever the file holds.
	at := len(warn.String())
	codexMenuSilenced(t)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s3", Dir: dir, Agent: "ranger", Bead: "a-2"}); err != nil {
		t.Fatalf("a silenced dismissal must not refuse anything: %v", err)
	}
	if rest := warn.String()[at:]; strings.Contains(rest, "dismissed_version") {
		t.Errorf("a silenced dismissal must warn about nothing:\n%s", rest)
	}
}

// The UNKNOWN reading, which is the one that decides whether this rule is a
// launch guard or a wall. posse cannot read version.json on a box codex has
// never checked for a release on, and refusing there would refuse in the
// probe's own words — "cannot tell whether the update menu is silenced".
// Measured: making the unreadable file a "no" reds six unrelated tests in
// this package, every one of them a dispatch on a temp HOME (ranger-base-9r33).
//
// The screen is not unguarded meanwhile: herdr names it `blocked` by its own
// rule, which codexupdatemenu_qa_test.go holds from both sides.
func TestQAUnreadableCodexVersionIsUnknownAndRefusesNothing(t *testing.T) {
	b, _ := newTestBackend(t)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "never-run"))

	rt, err := b.App.LoadRuntime("codex")
	if err != nil {
		t.Fatal(err)
	}
	if line := DangerLine(rt); line != "" {
		t.Errorf("posse refused a launch on a file it could not read: %s", line)
	}
	// The witness that the probe was reached at all and this is not a green
	// from an unloaded runtime: the same runtime, one readable file later,
	// does refuse.
	codexMenuBack(t)
	if DangerLine(rt) == "" {
		t.Fatal("a readable, expired dismissal must refuse — this test's negative arm is measuring nothing")
	}
}
