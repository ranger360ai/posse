//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-ymmiv, verification rows 1–2 of ADR 0060: the `bob` built-in
// is DECLARED from the recon and HONEST about the one row that is unmet.
//
// What is worth pinning here is not "the struct has the values it has" —
// a fixture repeating a literal proves nothing — but the three facts that
// fail SILENTLY if they drift:
//
//  1. argv ORDER. `bob -p … chat …`, never `bob chat … -p …`. bobshell 2.0.5
//     is built with .enablePositionalOptions().allowExcessArguments()
//     .allowUnknownOption(), so a `-p` after the subcommand is an unknown
//     option the program SWALLOWS: the persona identity document would
//     vanish with no error on any surface, and the session would open
//     looking perfectly healthy (ADR 0060 D3, docs/notes.d/ranger-base-v1yrt.md §5).
//     The same swallowing is why every other flag on the line is asserted
//     too — a misspelling is accepted and ignored rather than refused.
//  2. the LAUNCH ROW refusing by name. herdr 0.8.2 has no `bob` kind, so a
//     dispatched bob session is agent_not_found. ADR 0060 D1's whole claim
//     is that the profile says so out loud and `runtime check` exits 1 —
//     a profile that quietly printed six green rows over a runtime nothing
//     can address is the failure the grid exists to remove.
//  3. the typed-delivery `/` hazard under promptable (ADR 0060 D5). It has
//     no code guard by design — there is no dispatched caller until
//     detection lands — so the ONE thing carrying it is this line on this
//     screen. Delete the line and nothing else goes red.
//
// The herdr in these tests is a fake binary, and it ANSWERS BOTH WAYS on
// purpose: `Herdr{Bin: "no-such-herdr-binary"}` reads UNKNOWN, which is a
// non-blocking gap, so a pin written against it would be green over a
// runtime herdr recognized perfectly and green over one it did not. The
// control arm below asks the same fake about a label it does know.

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bobFakeHerdr writes a herdr stand-in that answers `agent explain --json`
// the way herdr 0.8.2 answered the recon: `unknown_agent` for any label in
// unknown, a manifest version for everything else. Everything else it is
// asked returns nothing, which is what a Herdr reader treats as "could not
// be asked".
func bobFakeHerdr(t *testing.T, unknown ...string) Herdr {
	t.Helper()
	var arms strings.Builder
	for _, u := range unknown {
		fmt.Fprintf(&arms, "    %s) printf '%%s' '{\"fallback_reason\":\"unknown_agent\"}'; exit 0 ;;\n", u)
	}
	bin := filepath.Join(t.TempDir(), "herdr")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = --warm ] && exit 0\n" +
		"if [ \"$1\" = agent ] && [ \"$2\" = explain ]; then\n" +
		"  for a in \"$@\"; do\n" +
		"    case \"$prev\" in --agent) want=$a ;; esac\n" +
		"    prev=$a\n" +
		"  done\n" +
		"  case \"$want\" in\n" + arms.String() +
		"    *) printf '%s' '{\"manifest_version\":\"2026.09.28.101\"}'; exit 0 ;;\n" +
		"  esac\n" +
		"fi\n" +
		"exit 1\n"
	// WriteExecutable and a warm run, for the ETXTBSY reason hangingBin
	// gives: this file is exec'd in a package where hundreds of parallel
	// tests fork (execwrite.go, ranger-base-d26ak).
	if err := WriteExecutable(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(bin, "--warm").Run(); err != nil {
		t.Fatalf("the fake herdr cannot be run at all (%v) — this test would measure nothing", err)
	}
	return Herdr{Bin: bin}
}

func bobRuntime(t *testing.T, a *App) *Runtime {
	t.Helper()
	rt, err := a.LoadRuntime("bob")
	if err != nil {
		t.Fatalf("bob is not a built-in: %v", err)
	}
	if !rt.Builtin {
		t.Fatalf("bob loaded as a template-only runtime (%s) — the profile is meant to be declared in Go", rt.Path)
	}
	return rt
}

// (1) The rendered launch line, which is verification row 2 of ADR 0060:
// what `posse new --runtime bob` puts in the pane.
func TestQABobLaunchLinePutsTheProgramPromptBeforeTheSubcommand(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := bobRuntime(t, a)
	ag := loadTestAgent(t, "---\nname: p\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")

	for _, tier := range Tiers {
		got := ag.RenderCommandFor(rt, "claude", tier)
		p, chat := strings.Index(got, " -p "), strings.Index(got, " chat ")
		if p < 0 || chat < 0 {
			t.Fatalf("%s: the line names no -p or no chat at all:\n%s", tier, got)
		}
		if p > chat {
			t.Errorf("%s: -p is AFTER the chat subcommand. bobshell allows unknown options, so it is swallowed in silence and the PID never reaches the model (ADR 0060 D3):\n%s", tier, got)
		}
		// Every flag on the line, for the same swallowing reason: a
		// misspelling here is accepted and ignored rather than refused.
		for _, want := range []string{"--accept-license", "--trust", "--auto-approve", "-w .", `"$(cat `} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: the line does not carry %q:\n%s", tier, want, got)
			}
		}
		// Tiers are UNMAPPED on bob (no per-launch model on chat or run), so
		// no tier may smuggle a model flag onto the line.
		if strings.Contains(got, "--model") || strings.Contains(got, "{") {
			t.Errorf("%s: the rendered line carries a model flag or an unrendered placeholder:\n%s", tier, got)
		}
	}
}

// The realizer half of the same line: bob has none, so the PID's rules
// render to NOTHING and the wall carries every gate — the template-only
// shape, declared rather than faked with a guessed flag dialect.
func TestQABobRealizesNoPIDRuleNatively(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := bobRuntime(t, a)
	if rt.Realize != nil {
		t.Fatalf("bob declares a native realizer — ADR 0060 D1 declares none, and a guessed flag dialect is worse than the gap")
	}
	ag := loadTestAgent(t, "---\nname: p\nallow: [Bash(ls:*)]\ndeny: [Bash(git push:*)]\n---\nYou are p.\n")
	got := ag.RenderCommandFor(rt, "claude", TierStrong)
	for _, unwanted := range []string{"git push", "Bash(", "--allow", "--deny"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("a PID rule reached bob's launch line as %q — {allow}/{deny} must render to nothing:\n%s", unwanted, got)
		}
	}
	// And in the matrix: every realized row on bob must come from a WALL
	// layer, never from bob itself. `len(p.Realized) == 0` would be the
	// wrong assertion — L1's shim realizes the git-push deny here as it
	// does everywhere — so the reading is that no row credits the runtime.
	p := a.CheckParity(ag, rt, CageShims, TierStrong)
	for gate, g := range p.Realized {
		if strings.Contains(g.Detail, rt.Name) {
			t.Errorf("the parity matrix credits bob itself with %q (%s) — with no realizer every gate belongs to the wall", gate, g)
		}
	}
}

// (2) The grid and the preflight, with a herdr that says unknown_agent —
// verification row 1 of ADR 0060.
func TestQABobRuntimeCheckRefusesTheLaunchRowByName(t *testing.T) {
	a := checkApp(t)
	t.Setenv("HOME", t.TempDir()) // the preflight reads a home; never the operator's
	rt := bobRuntime(t, a)

	var b bytes.Buffer
	clean := a.RuntimeCheck(rt, bobFakeHerdr(t, "bob"), &b)
	out := b.String()
	if clean {
		t.Errorf("runtime check bob reported a clean preflight — with no detection a dispatched session is agent_not_found and cannot be addressed at all, so this must exit 1:\n%s", out)
	}
	for _, want := range []string{
		`herdr does NOT recognize argv0 "bob"`,
		"✗ detection",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the check never says %q:\n%s", want, out)
		}
	}

	// THE CONTROL. Without it a green above is consistent with a check that
	// says "does NOT recognize" about every runtime on every box.
	var c bytes.Buffer
	cleanKnown := a.RuntimeCheck(rt, bobFakeHerdr(t, "something-else"), &c)
	if strings.Contains(c.String(), `does NOT recognize argv0 "bob"`) {
		t.Errorf("the launch row says bob is unrecognized even when herdr answers with a manifest — the reading is not being made:\n%s", c.String())
	}
	if !cleanKnown {
		// Not a failure of the profile: this box may legitimately lack the
		// bob binary. It is a failure of this control only if the reason is
		// detection.
		if strings.Contains(c.String(), "✗ detection") {
			t.Errorf("detection is still blocking against a herdr that HAS a manifest:\n%s", c.String())
		}
	}
}

// Every row the profile declares, read off the screen an onboarder reads
// rather than off the struct — the grid IS the contract for a
// display-by-design field, and a value the grid drops is a value nobody
// has (ADR 0013 §1, the shape ranger-base-x7m1 pinned for rulebooks).
func TestQABobGridDeclaresEveryRow(t *testing.T) {
	a := checkApp(t)
	t.Setenv("HOME", t.TempDir())
	rt := bobRuntime(t, a)

	var b bytes.Buffer
	a.RuntimeCheck(rt, bobFakeHerdr(t, "bob"), &b)
	flat := strings.Join(strings.Fields(b.String()), " ")

	for _, want := range []string{
		// promptable: typed, on the default wait — startup_wait: stays
		// UNSET because the recon's 20s is one reading, not a measurement.
		"typed — create → await promptable → claim → type",
		"(startup_wait: unset → the default)",
		// record: untrusted. No bob session has been dispatched at all.
		"untrusted — no dispatched session of this runtime has been measured to close its bead",
		// tiers UNMAPPED — no per-launch model on chat or run — and the
		// unknown-id dimension UNDECLARED, which is the honest reading
		// rather than a borrowed one (ADR 0017 §2's two vocabularies).
		"UNMAPPED — this runtime ignores tier: entirely",
		"UNDECLARED — nobody has measured what this CLI does with an id it does not know",
		// the state dir the seatbelt grants, and only it
		"~/.bob",
		// egress: what a caged bob must reach to be a session at all
		"bob.ibm.com", "iam.cloud.ibm.com",
		// the container credential, by NAME
		BobCageCred,
		// the repo→box channel the launch checks before any turn
		".bob/settings", ".bob/mcp.json", ".bob/hooks", ".bob/custom_modes.yaml",
		// the other voice in the session
		"AGENTS.md", "CLAUDE.md", ".bob/rules-agent/AGENTS.md",
		".bob/rules-plan/AGENTS.md", ".bob/rules-ask/AGENTS.md",
		"precedence UNMEASURED",
		// the three first-run screens
		"--accept-license", "--trust", "Complete sign-in in your browser",
	} {
		if !strings.Contains(flat, strings.Join(strings.Fields(want), " ")) {
			t.Errorf("the bob grid never says %q:\n%s", want, b.String())
		}
	}
	// And what it must NOT say. A cost adapter and a turn-outcome reader are
	// both absent on purpose (ADR 0060 D4); a grid claiming either would be
	// claiming a reading nobody can make.
	if !strings.Contains(flat, "uncounted_cap_bob:") {
		t.Errorf("the account row does not name bob's own brake:\n%s", b.String())
	}
	// No screen on bob carries danger:, so nothing here refuses the launch
	// for an interstitial — the detection gap is the only refusal.
	if lines := DangerUnsilenced(rt); len(lines) != 0 {
		t.Errorf("bob declares a machine-mutating first-run screen: %v — ADR 0060 D1 says none of the three is that class", lines)
	}
}

// (3) ADR 0060 D5, and the only thing carrying it. Asserted UNDER the
// promptable row rather than anywhere on the page: the note is for the
// operator reading the delivery stage, and a line that drifted into the
// footer would be a line nobody reads at the moment it matters.
func TestQABobPromptableRowWarnsAgainstALeadingSlash(t *testing.T) {
	a := checkApp(t)
	t.Setenv("HOME", t.TempDir())
	rt := bobRuntime(t, a)

	var b bytes.Buffer
	a.RuntimeCheck(rt, bobFakeHerdr(t, "bob"), &b)
	_, after, ok := strings.Cut(b.String(), "\n  promptable  ")
	if !ok {
		t.Fatalf("the grid drew no promptable row at all:\n%s", b.String())
	}
	row, _, ok := strings.Cut(after, "\n  work ")
	if !ok {
		t.Fatalf("the promptable row has no work row after it; re-anchor this pin:\n%s", b.String())
	}
	flat := strings.Join(strings.Fields(row), " ")
	if !strings.Contains(flat, "typed prompts must NEVER start with `/`") {
		t.Errorf("the promptable row does not carry ADR 0060 D5 — the picker takes the Enter, the text stays in the composer and no turn ever starts, with no error on any surface:\n%s", row)
	}
	// The note is a property of TYPED delivery, not of bob's name (ADR 0017
	// §3): an argv runtime must not print it, or it is prose in the wrong
	// place on two runtimes out of four.
	argv, err := a.LoadRuntime("grok")
	if err != nil {
		t.Fatal(err)
	}
	var g bytes.Buffer
	a.RuntimeCheck(argv, bobFakeHerdr(t, "bob"), &g)
	if strings.Contains(g.String(), "typed prompts must NEVER start with") {
		t.Errorf("an argv-delivery runtime prints the typed-delivery hazard:\n%s", g.String())
	}
}

// The two screens posse answers ON ITS OWN LINE, and the reading behind the
// grid saying "silenced". A probe that always said yes would be a rubber
// stamp: the whole reason these screens never draw is that BobCommand
// carries the flag, so the probe has to be able to say NO when it does not.
func TestQABobFlagInterstitialProbesReadTheLaunchLine(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"--accept-license", "--trust"} {
		if sil := bobFlagSilence(flag)(); !sil.Silenced {
			t.Errorf("%s: the probe does not read the live BobCommand as silenced (%s):\n%s", flag, sil.Why, BobCommand)
		}
		// The falsifying arm: a flag the line does not carry must read NOT
		// silenced, or the two rows above are scenery.
		if sil := bobFlagSilence("--no-such-flag-on-any-line")(); sil.Silenced || sil.Unknown {
			t.Errorf("the probe reports a flag nothing carries as silenced: %+v", sil)
		}
	}
	// The sign-in screen is the opposite declaration: posse declines to
	// look, which is UNKNOWN and never "no" — a launch must not be walled
	// on a reading nobody made (ranger-base-9r33), and the line has to say
	// whose decision it is rather than claiming posse cannot parse a file.
	sil := bobSignInSilence()
	if !sil.Unknown || sil.Silenced {
		t.Errorf("the sign-in probe must read UNKNOWN: %+v", sil)
	}
	if !strings.Contains(sil.Why, "does not look") {
		t.Errorf("the sign-in reading does not say it is a decision rather than an inability: %q", sil.Why)
	}
}

// ADR 0060 verification row 1's second clause, and the reason the built-in
// leaves startup_wait UNSET: the recon watched ONE launch reach a composer
// in about 20s, and a per-runtime wait is the one number that exists
// because it was MEASURED. So the built-in stays on the claude-shaped
// default and says so, and the day the instance side measures a real one it
// arrives as an ADR 0021 overlay — which is a fact about whose declaration
// the grid must attribute correctly, not just about the number.
func TestQABobStartupWaitIsTheDefaultUntilAYamlMeasuresOne(t *testing.T) {
	a := checkApp(t)
	t.Setenv("HOME", t.TempDir())

	rt := bobRuntime(t, a)
	if rt.StartupWait != 0 || rt.Wait() != DefaultStartupWait {
		t.Errorf("the bob built-in declares startup_wait %s — the recon's single reading is not a measurement (ADR 0060 D1)", rt.StartupWait)
	}

	over := writeRuntime(t, a, "bob", "startup_wait: 90s\n")
	if !over.Builtin {
		t.Fatalf("runtimes/bob.yaml replaced the built-in instead of overlaying it (ADR 0021)")
	}
	if over.Wait() != 90*time.Second {
		t.Errorf("the overlay did not reach Wait(): %s", over.Wait())
	}
	// And the grid says WHOSE declaration it is, which is the whole value of
	// the provenance line to an onboarder: a key read from the file and a
	// key that fell back to the built-in are different facts.
	var b bytes.Buffer
	a.RuntimeCheck(over, bobFakeHerdr(t, "bob"), &b)
	flat := strings.Join(strings.Fields(b.String()), " ")
	if !strings.Contains(flat, "runtimes/bob.yaml (startup_wait:)") {
		t.Errorf("the promptable row still credits the built-in for an overlaid startup_wait:\n%s", b.String())
	}
	if strings.Contains(flat, "(startup_wait: unset → the default)") {
		t.Errorf("the grid calls an overlaid wait unset:\n%s", b.String())
	}
}
