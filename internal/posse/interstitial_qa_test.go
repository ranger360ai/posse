package posse

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

// ranger-base-vbp3: the same rule one layer in, and the layer where it was
// still a printed sentence. Everything above turns on codex, whose probe is
// Go code posse measured; the runtime that will FIRST meet a machine-
// mutating first-run dialog is not codex. It is a `runtimes/<name>.yaml` —
// typed delivery by default, a screen the profile's author declared because
// only they know their CLI, and therefore no probe at all. ranger-base-9r33
// excluded exactly that shape from DangerUnsilenced, so `runtime check`
// printed "LAUNCH REFUSE until silenced" for a screen dispatch claimed the
// bead and typed into (measured on this fixture before the fix: n=1, a
// `workspace create`, and `update a-1 --claim` in the bd log).
//
// The two arms differ by the `danger:` line and nothing else, which is the
// point: a declared screen documents itself and walls nothing until its
// author writes down that its default action mutates the machine.
//
// What this measures is the OBSERVABLE — no launch, no claim, the operator's
// own words in the output — and not which surface produced it. MEASURED:
// deleting `launchSession`'s refuse leaves this test green, because a typed
// launch reaches `planLaunch` inside CreateSession and refuses there before
// the claim. The placement is load-bearing on the ARGV ladder, which claims
// first, and TestQADangerousCodexInterstitialRefusesDispatchUntilSilenced is
// the test that reds when it moves.
func TestQATypedDispatchRefusesADeclaredDangerScreenAndLaunchesWithoutOne(t *testing.T) {
	const screen = `interstitial_update:
  screen: "Update available!  1. Update now  2. Skip"
  where: ~/.mycli/version.json
  key: dismissed_version
  silence: "the OPERATOR picks 2. Skip, in their own mycli session"
`
	danger := screen + "  danger: \"the selected option runs `brew upgrade --cask mycli`\"\n"

	for _, tc := range []struct {
		name   string
		yaml   string
		refuse bool
	}{
		{"declared danger refuses", danger, true},
		{"the same screen without danger: launches", screen, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(b.App.RuntimesDir(), "mycli.yaml"),
				[]byte("command: mycli {file}\n"+tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			// The fixture's own witness: this is the typed ladder, so the
			// refusal below is the one the bead is about and not argv's.
			rt, err := b.App.LoadRuntime("mycli")
			if err != nil {
				t.Fatal(err)
			}
			if rt.PromptMode() != PromptTyped {
				t.Fatalf("this fixture must exercise TYPED delivery, got %s", rt.PromptMode())
			}
			if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
				t.Fatal(err)
			}
			pid := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: mycli\n---\nYou are ranger.\n"
			if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
				t.Fatal(err)
			}
			qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
			idleClaude(t, fake)

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out, log, bdlog := dispatcherOut(d), calls(t, fake), bdCalls(t, fake)
			if !tc.refuse {
				if n != 1 || !strings.Contains(log, "workspace create") {
					t.Fatalf("a declared screen with no danger: walls nothing — dispatched %d bead(s):\n%s\n%s", n, out, log)
				}
				if strings.Contains(out, "MUTATES THE MACHINE") {
					t.Errorf("nothing declared a default action that mutates anything:\n%s", out)
				}
				return
			}
			if n != 0 {
				t.Errorf("dispatched %d bead(s) onto a screen whose default action mutates the machine:\n%s", n, out)
			}
			if strings.Contains(log, "workspace create") {
				t.Errorf("a session was created onto the declared screen:\n%s\n%s", log, out)
			}
			if strings.Contains(bdlog, "--claim") {
				t.Errorf("the bead was claimed before the refusal — the refuse must sit above the claim (ADR 0013 §2):\n%s\n%s", bdlog, out)
			}
			// The refusal in the operator's own words: the key, the file,
			// the danger they declared, and the Silence text — which is the
			// only thing on the line they can act on.
			for _, want := range []string{
				"dismissed_version", "~/.mycli/version.json",
				"brew upgrade --cask mycli",
				"the OPERATOR picks 2. Skip, in their own mycli session",
				"drop danger:",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("the refusal must carry %q:\n%s", want, out)
				}
			}
		})
	}
}

// The state table DangerUnsilenced is, over one runtime per reading, because
// the dispatch test above can only reach the two states a yaml can express.
// The silenced and unknown arms are the ones that decide whether this rule
// is a launch guard or a wall on every launch, and neither is reachable
// through LoadRuntime: a probe is Go code, and the three runtimes that carry
// one are all built-ins.
func TestQADangerUnsilencedReadsEveryInterstitialState(t *testing.T) {
	probe := func(s Silence) func() Silence { return func() Silence { return s } }
	base := Interstitial{
		Screen: "S", Where: "~/.mycli/f", Key: "k",
		Silence: "the OPERATOR clears it", Danger: "the default option upgrades the machine",
	}
	with := func(f func(*Interstitial)) Interstitial {
		in := base
		f(&in)
		return in
	}
	for _, tc := range []struct {
		name   string
		in     Interstitial
		refuse bool
	}{
		{"probe says NOT silenced", with(func(i *Interstitial) { i.Probe = probe(Silence{Why: "unset"}) }), true},
		{"probe says silenced", with(func(i *Interstitial) { i.Probe = probe(Silence{Silenced: true, Why: "set"}) }), false},
		{"probe says unknown", with(func(i *Interstitial) { i.Probe = probe(Silence{Unknown: true, Why: "unreadable"}) }), false},
		{"no probe at all", base, true},
		{"no probe, no danger", with(func(i *Interstitial) { i.Danger = "" }), false},
		{"seeded, unsilenced", with(func(i *Interstitial) {
			i.Seeded = true
			i.Probe = probe(Silence{Why: "unset"})
		}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := &Runtime{Name: "mycli", Path: "/x/runtimes/mycli.yaml", Interstitials: []Interstitial{tc.in}}
			lines := DangerUnsilenced(rt)
			if tc.refuse != (len(lines) == 1) {
				t.Fatalf("refuse=%v but got %d line(s): %v", tc.refuse, len(lines), lines)
			}
			if !tc.refuse {
				return
			}
			// A refusal an operator cannot clear from the line is a dead
			// end, so every arm that refuses names the way out.
			if !strings.Contains(lines[0], base.Silence) {
				t.Errorf("the refusal must carry the Silence text: %s", lines[0])
			}
			if !strings.Contains(lines[0], base.Danger) {
				t.Errorf("the refusal must say what the default action does: %s", lines[0])
			}
		})
	}
	// And the launch surface agrees with the reading: DangerRefusal is what
	// dispatch hands back, so the line has to survive into it.
	rt := &Runtime{Name: "mycli", Path: "/x/runtimes/mycli.yaml", Interstitials: []Interstitial{base}}
	err := DangerRefusal(rt, DangerLine(rt)).Error()
	if !strings.Contains(err, base.Silence) || !strings.Contains(err, "posse runtime check mycli") {
		t.Errorf("the refusal must carry the operator's action and where to see the grid: %s", err)
	}
}

// The dispatch-level control the codex test above does not have: the same
// runtime, one probe reading later, launches. Without it every assertion in
// TestQADangerousCodexInterstitialRefusesDispatchUntilSilenced holds equally
// for a launchSession that refused every codex bead on principle.
func TestQASilencedDangerInterstitialDispatchesNormally(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	codexMenuSilenced(t)

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
	if n != 1 {
		t.Errorf("a silenced dismissal must refuse nothing — dispatched %d bead(s):\n%s", n, out)
	}
	if !strings.Contains(calls(t, fake), "workspace create") {
		t.Errorf("no session was created:\n%s\n%s", calls(t, fake), out)
	}
	if strings.Contains(out, "MUTATES THE MACHINE") {
		t.Errorf("a silenced dismissal must say nothing about the menu:\n%s", out)
	}
}

// "The runtime check line and dispatch can no longer disagree about what
// LAUNCH REFUSE means" (ranger-base-vbp3) — the bead's own done-when, and
// the only assertion here that would have failed for a reason other than a
// missing branch: the grid PROMISED the refuse and the launcher did not
// make it, for every runtime an operator can declare.
//
// So it is written as an equivalence over one fixture, both directions,
// rather than three separate expectations that could drift apart again.
func TestQADeclaredDangerScreenAgreesAcrossAllThreeSurfaces(t *testing.T) {
	const screen = `command: mycli {file}
interstitial_update:
  screen: "Update available!"
  where: ~/.mycli/version.json
  key: dismissed_version
  silence: "the OPERATOR picks 2. Skip"
`
	for _, danger := range []bool{true, false} {
		body := screen
		if danger {
			body += "  danger: \"the selected option upgrades the machine's tooling\"\n"
		}
		a := checkApp(t)
		rt := writeRuntime(t, a, "mycli", body)
		h := Herdr{Bin: "no-such-herdr-binary"}

		var b strings.Builder
		a.RuntimeCheck(rt, h, &b)
		printed := strings.Contains(b.String(), "LAUNCH REFUSE")
		refuses := DangerLine(rt) != ""
		blocking := false
		for _, g := range a.RuntimeGaps(rt, h) {
			if g.Name == "interstitial" && g.Blocking {
				blocking = true
			}
		}
		if printed != refuses || printed != blocking {
			t.Errorf("danger:%v — the grid says LAUNCH REFUSE=%v, the launcher refuses=%v, the preflight blocks=%v; all three read one function:\n%s",
				danger, printed, refuses, blocking, b.String())
		}
		if !danger {
			continue
		}
		// And the grid says the one thing that lifts a refusal posse can
		// never read its way out of — silencing the screen does not, since
		// there is no probe to read the key with.
		if !strings.Contains(b.String(), "drop danger:") {
			t.Errorf("the grid must say what lifts a probe-less refusal:\n%s", b.String())
		}
	}
}

// The other half of ranger-base-vbp3's WHAT: the busy-key split (ADR 0013
// §2). A screen an operator has to silence is an INSTANCE-CONFIG fact, not
// a pane fluke — the ADR's own Assumed note is that panes cannot fix an
// instance-config fact — so every bead routed to this persona on this
// runtime meets the same screen. Claiming them one at a time to refuse them
// one at a time is the sterilised queue §2 named once.
//
// It works by the shape of the error rather than by anything that says
// "bench": fire's three-way switch benches on the DEFAULT arm, and a plain
// Die from the refuse lands there because it is neither claimLostError nor
// sessionFailure. Which is exactly why it is worth a test — a later refactor
// that wrapped launch errors as sessionFailure would turn this into two
// refusals per pass, and every other assertion in this file would stay green.
func TestQADeclaredDangerScreenBenchesTheSlotNotTheBead(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.RuntimesDir(), "mycli.yaml"), []byte(`command: mycli {file}
interstitial_update:
  screen: "Update available!"
  where: ~/.mycli/version.json
  key: dismissed_version
  silence: "the OPERATOR picks 2. Skip"
  danger: "the selected option upgrades the machine's tooling"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: mycli\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`, "")
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 {
		t.Fatalf("dispatched %d bead(s) onto a declared machine-mutating screen:\n%s", n, out)
	}
	if got := strings.Count(out, "MUTATES THE MACHINE"); got != 1 {
		t.Errorf("the refusal is a fact about the persona on this runtime, so it is said ONCE and the slot is benched; said %d times:\n%s", got, out)
	}
	if !strings.Contains(out, "skipped for the rest of this pass") {
		t.Errorf("the second bead must be skipped by name, not claimed and refused:\n%s", out)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("no bead may be claimed on a benched slot:\n%s\n%s", log, out)
	}
}
