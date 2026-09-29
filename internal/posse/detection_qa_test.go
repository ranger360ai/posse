//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-d8riq — ADR 0013 §1's launch row, as a launch.
//
// THE GAP THESE HOLD. The grid's launch row said *refuse the launch* and
// nothing refused one: the reading behind it reached `posse runtime check`
// (exit 1) and `posse runtime probe` and no launch path at all. So a bead
// routed to a persona on a runtime herdr cannot name spent, per attempt, the
// worktree, the workspace, the pane and the runtime's own first turn, then
// `startup_wait` of no answer — AgentTarget lists herdr's agents for the
// workspace and finds none, so the wait cannot even reach the settle gate —
// and every line printed along the way named the wrong cause (`no agent
// detected in <session> after 45s — check the session`, which reads as a slow
// start whose remedy is a larger `startup_wait:`). The pane was left alive,
// and the NEXT pass read "no agent status" as the CLI-died signal and typed
// the persona's launch line into the live TUI's composer as a chat turn.
//
// WHAT WOULD FAIL SILENTLY WITHOUT THEM, which is why each arm is here:
//
//   - delete launchSession's refusal and the typed ladder still refuses one
//     line later inside planLaunch — with the bead already claimed on the
//     ARGV ladder, which claims before it creates. Only the no-`--claim`
//     assertion sees that (the shape
//     TestQADangerousCodexInterstitialRefusesDispatchUntilSilenced holds for
//     the danger rule).
//   - read UNKNOWN as a "no" and posse walls every box whose herdr is a
//     version behind or off PATH, in the reading's own words. Nothing else
//     here goes red for it.
//   - refuse `posse new` too and the refusal becomes permanent: the fixtures
//     an upstream detection filing needs are captured from an interactive
//     session (ADR 0060 D2), so that launch is the only route that can end
//     this refusal.
//   - let the grid and the launcher ask the question separately and they
//     drift, which is the failure this whole row is a repeat of — three
//     times now (9r33, vbp3, i3q6g).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// herdrNames arms the shared fake the way a real herdr answers the LABEL
// question: these labels have manifests, every other label is
// `unknown_agent`. No argument at all is the UNKNOWN reading — herdr
// answers neither way — which is a different lever because it is a
// different answer.
func herdrNames(t *testing.T, fake string, labels ...string) {
	t.Helper()
	if len(labels) == 0 {
		if err := os.WriteFile(filepath.Join(fake, "explain-agent-error"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.WriteFile(filepath.Join(fake, "herdr-kinds"), []byte(strings.Join(labels, " ")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// undetectableFixture is one persona on one declared runtime, plus `beads`
// ready rows. The runtime is a template-only yaml because that is the shape
// this rule was cut for: bob is the only built-in that meets it today, and
// every `runtimes/<name>.yaml` whose argv0 herdr does not know is the same
// shape. The argv0 is `mycli`, so whether it is detectable is decided
// entirely by the lever above.
//
// `prompt` is the DELIVERY LADDER, and it is a parameter because the two
// ladders order the claim and the create differently (ADR 0013 §2) — which
// is the whole reason the refusal sits where it does. On the typed ladder
// the create happens first, so a refusal from inside CreateSession is still
// above the claim and launchSession's own is redundant; on the argv ladder
// the bead is claimed BEFORE the session is created, and only a refusal
// above both branches leaves it untouched. A fixture that exercised one of
// them would pin a placement that holds for the other by accident.
// It returns the repo it wrote the ready rows into, which the relaunch arms
// need to name a session the way dispatch does (relaunchdetection_qa_test.go,
// ranger-base-enmu2).
//
// decl is extra yaml LINES for that profile, variadic so the three callers
// that predate it are untouched. It is how the reported arms declare
// `detection:`/`detection_why:` on the same fixture the refusing arms use
// (reporteddetection_qa_test.go, ranger-base-rx7l7) — one profile shape for
// both, because the whole subject is one declaration changing what the same
// herdr answer MEANS, and two fixtures would let the two halves drift.
func undetectableFixture(t *testing.T, b *HerdrBackend, prompt, ready string, decl ...string) string {
	t.Helper()
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "command: mycli {file}\n"
	if prompt != "" {
		body += "prompt: " + prompt + "\n"
	}
	for _, ln := range decl {
		body += ln + "\n"
	}
	if err := os.WriteFile(filepath.Join(b.App.RuntimesDir(), "mycli.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: ranger\ndescription: test\nlabels: [go]\nruntime: mycli\n---\nYou are ranger.\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "ranger.md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	return qaRepo(t, b.App, ready, "")
}

// The rule, over all three readings of the same profile — which is the only
// way to write it. An arm that refused on UNKNOWN and an arm that refused on
// everything both pass the first subtest alone.
func TestQAUndetectableRuntimeRefusesDispatchAndEveryOtherReadingLaunches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		names  []string
		refuse bool
	}{
		// herdr's own answer that it has no manifest for `mycli`. The one
		// state that refuses.
		{"unknown_agent refuses", []string{"claude", "codex", "grok"}, true},
		// The negative control, and the assertion the others are measured
		// against: without it every expectation here holds equally for a
		// launcher that refused every bead on this runtime on principle.
		{"a manifest launches", []string{"claude", "mycli"}, false},
		// UNKNOWN — herdr answers the label question neither way. It refuses
		// NOTHING and the launch proceeds exactly as it did before this rule
		// existed (ADR 0013 §1 property 1: it is a reading, never ignorance).
		{"a herdr that cannot be asked launches", nil, false},
	} {
		for _, ladder := range []string{PromptTyped, PromptArgv} {
			t.Run(tc.name+" on the "+ladder+" ladder", func(t *testing.T) {
				b, fake := newTestBackend(t)
				d := newTestDispatcher(t, b)
				herdrNames(t, fake, tc.names...)
				undetectableFixture(t, b, ladder, `[{"id":"a-1","title":"t","labels":["go"]}]`)
				idleClaude(t, fake)

				n, err := d.Run("", "", 0)
				if err != nil {
					t.Fatal(err)
				}
				out, log, bdlog := dispatcherOut(d), calls(t, fake), bdCalls(t, fake)
				if !tc.refuse {
					if n != 1 || !strings.Contains(log, "workspace create") {
						t.Fatalf("this reading walls nothing — dispatched %d bead(s):\n%s\n%s", n, out, log)
					}
					if strings.Contains(out, "no detection manifest") {
						t.Errorf("a launch that proceeded still said the runtime was undetectable:\n%s", out)
					}
					return
				}
				if n != 0 {
					t.Errorf("dispatched %d bead(s) onto a runtime herdr cannot name — the session would be agent_not_found:\n%s", n, out)
				}
				// Nothing spent, and the bead untouched. On the ARGV ladder
				// these two are the placement: it claims before it creates,
				// so a refusal one line later — from inside CreateSession —
				// hands back a bead it has already taken. MEASURED by
				// deleting launchSession's refusal: the argv subtests red
				// here and the typed ones stay green.
				if strings.Contains(log, "workspace create") {
					t.Errorf("a workspace was created for a session nothing can address:\n%s\n%s", log, out)
				}
				if strings.Contains(bdlog, "--claim") {
					t.Errorf("the bead was claimed before the refusal — the refuse must sit above the claim and above both delivery branches (ADR 0013 §1 property 3):\n%s\n%s", bdlog, out)
				}
				// The line names the cause and the door, and never the
				// session (property 6). "check the session" is the sentence
				// this rule exists to stop printing: there is no session to
				// check, and reading it as a slow start sends an operator to
				// raise `startup_wait:`, which changes nothing.
				for _, want := range []string{
					`argv0 "mycli"`, "agent_not_found",
					"docs/runbooks/agent-detection-manifest.md",
					"posse runtime check mycli",
				} {
					if !strings.Contains(out, want) {
						t.Errorf("the refusal must carry %q:\n%s", want, out)
					}
				}
				if strings.Contains(out, "check the session") {
					t.Errorf("the refusal points at a session that was never created:\n%s", out)
				}
			})
		}
	}
}

// The busy-key split (ADR 0013 §1 property 4). A missing manifest is a fact
// about this persona's RUNTIME, not about a pane: every bead routed here
// meets the same missing manifest, so the slot is benched for the pass on
// the first one and nothing is claimed for the rest. Claiming them one at a
// time to refuse them one at a time is the sterilised queue ADR 0013 §2
// named once.
//
// It works by the SHAPE of the error and not by anything that says "bench":
// fire's three-way switch benches on the default arm, and a plain Die lands
// there because it is neither claimLostError nor sessionFailure. Which is
// exactly why it is worth a test — wrapping the refusal as sessionFailure
// would turn this into two refusals per pass and leave every other
// assertion in this file green.
func TestQAUndetectableRuntimeBenchesTheSlotNotTheBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	herdrNames(t, fake, "claude", "codex", "grok")
	undetectableFixture(t, b, PromptArgv, `[{"id":"a-1","title":"t","labels":["go"]},{"id":"a-2","title":"u","labels":["go"]}]`)
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 {
		t.Fatalf("dispatched %d bead(s) onto a runtime herdr cannot name:\n%s", n, out)
	}
	if got := strings.Count(out, "no detection manifest for argv0"); got != 1 {
		t.Errorf("the refusal is one fact about the persona on this runtime, so it is said ONCE and the slot is benched; said %d times:\n%s", got, out)
	}
	if !strings.Contains(out, "skipped for the rest of this pass") {
		t.Errorf("the second bead must be skipped by name, not claimed and refused:\n%s", out)
	}
	if log := bdCalls(t, fake); strings.Contains(log, "--claim") {
		t.Errorf("no bead may be claimed on a benched slot:\n%s\n%s", log, out)
	}
}

// ADR 0015 §3's asymmetry, and the half that is the escape hatch (property
// 5). A launch carrying a bead is dispatch's and nobody is watching it; an
// interactive one is the operator at their own keyboard, and it proceeds —
// because the fixtures the upstream detection filing needs are captured from
// exactly that session (ADR 0060 D2). A posse that refused it would have
// walled off the only route that ends its own refusal, which is also why
// there is no `--allow-undetected` to test for.
//
// Asked at planLaunch, which is the backstop under every launch path that is
// not the dispatch loop — a cockpit `d` on a session it must create, a
// recipe, and the recreate half of `posse relaunch`, which plans before it
// kills.
func TestQAUndetectableRuntimeRefusesABeadLaunchAndWarnsAnInteractiveOne(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	herdrNames(t, fake, "claude", "codex", "grok")
	undetectableFixture(t, b, PromptTyped, `[]`)
	dir := t.TempDir()

	_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger", Bead: "a-1"})
	if err == nil {
		t.Fatal("a bead-carrying launch onto a runtime herdr cannot name must refuse — nobody is watching it, and every state of the session it would create is agent_not_found")
	}
	for _, want := range []string{`argv0 "mycli"`, "agent_not_found", "posse runtime check mycli"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q and the door: %v", want, err)
		}
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: dir, Agent: "ranger"}); err != nil {
		t.Fatalf("an interactive launch must PROCEED — it is the only route that can capture the fixtures the manifest is written from: %v", err)
	}
	got := warn.String()
	for _, want := range []string{"DEGRADED", `argv0 "mycli"`, "agent-detection-manifest.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("an interactive launch that proceeds must still say what it is opening on (%q missing):\n%s", want, got)
		}
	}

	// The negative control, without which both halves above pass for a rule
	// that fires on every launch whatever herdr answers.
	at := len(warn.String())
	herdrNames(t, fake, "claude", "mycli")
	if _, err := b.planLaunch(NewSessionOpts{Name: "s3", Dir: dir, Agent: "ranger", Bead: "a-2"}); err != nil {
		t.Fatalf("a runtime herdr HAS a manifest for must refuse nothing: %v", err)
	}
	if rest := warn.String()[at:]; strings.Contains(rest, "DEGRADED") {
		t.Errorf("a detected runtime must warn about nothing:\n%s", rest)
	}
}

// "The grid and the launch cannot disagree" — ADR 0013 §1 property 1, and
// the only assertion here that would have failed for a reason other than a
// missing branch: for three amendments running, this grid PROMISED a refuse
// and the launcher did not make it (9r33's danger row, vbp3's declared
// screen, now detection).
//
// So it is written as an equivalence over one fixture and all three
// readings, both directions, rather than as separate expectations that could
// drift apart again — and it reads the surfaces an operator and a dispatch
// actually meet: the printed launch row, the preflight's blocking bit, and
// the launch refusal itself.
func TestQAUndetectableRuntimeAgreesAcrossAllThreeSurfaces(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		names []string
	}{
		{"unknown_agent", []string{"claude", "codex", "grok"}},
		{"a manifest", []string{"claude", "mycli"}},
		{"herdr cannot be asked", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			herdrNames(t, fake, tc.names...)
			undetectableFixture(t, b, PromptTyped, `[]`)
			rt, err := b.App.LoadRuntime("mycli")
			if err != nil {
				t.Fatal(err)
			}

			var out strings.Builder
			b.App.RuntimeCheck(rt, b.H, &out)
			printed := strings.Contains(out.String(), `herdr does NOT recognize argv0 "mycli"`)
			blocking := false
			for _, g := range b.App.RuntimeGaps(rt, b.H) {
				if g.Name == "detection" && g.Blocking {
					blocking = true
				}
			}
			_, launchErr := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger", Bead: "a-1"})
			refuses := launchErr != nil

			if printed != blocking || printed != refuses {
				t.Errorf("%s — the grid's launch row says NOT recognized=%v, the preflight blocks=%v, a bead launch refuses=%v; all three read one function:\n%s\nlaunch: %v",
					tc.name, printed, blocking, refuses, out.String(), launchErr)
			}
			// And the grid marks it, so an operator reading the check sees
			// the same ✗ the launcher acts on.
			if printed != strings.Contains(out.String(), "✗ detection") {
				t.Errorf("%s — the launch row and the preflight's ✗ disagree:\n%s", tc.name, out.String())
			}
		})
	}
}

// Property 6's second clause, and the one place this rule is allowed to
// branch on the runtime at all: the DOOR. A declared runtime's author writes
// the manifest (or aliases their CLI onto one that exists) and detection
// arrives on their own box; a BUILT-IN's argv0 is posse's own and its
// detection is upstream's to ship, so the door is the filing posse already
// carries and the operator sends (ADR 0060 D2).
//
// Branched on rt.Builtin and never on a runtime NAME — a name-keyed clause
// here is the ADR 0017 §3 shadow predicate ADR 0060 rejected a whole adapter
// over, and it would say the wrong thing about the second built-in that ever
// meets this. So the assertion is the pair, in both directions: neither door
// may appear on the other runtime's line, and the gap the operator reads in
// `runtime check` must name the same one the refusal does.
func TestQAUndetectableDoorIsTheFilingForABuiltinAndTheRunbookForADeclaredOne(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	bob, err := a.LoadRuntime("bob")
	if err != nil || !bob.Builtin {
		t.Fatalf("bob must load as a built-in (%v)", err)
	}
	mycli := writeRuntime(t, a, "mycli", "command: mycli {file}\n")

	for _, tc := range []struct {
		rt            *Runtime
		want, unwired string
	}{
		{bob, "etc/herdr/agent-detection/upstream-bob.md", "docs/runbooks/" + detectionDoc},
		{mycli, "docs/runbooks/" + detectionDoc, "upstream-"},
	} {
		r := ManifestReading{Argv0: tc.rt.Exe(), State: ManifestUnknownAgent}
		// Every surface that prints a door, so none of them can drift off on
		// its own: the refusal, the preflight gap, and the interactive warn.
		for _, line := range []string{
			DetectionRefusal(tc.rt, r).Error(),
			DetectionGapLine(tc.rt, r),
			DetectionDegraded(tc.rt, r),
		} {
			if !strings.Contains(line, tc.want) {
				t.Errorf("%s: the line must name %q as the way out:\n%s", tc.rt.Name, tc.want, line)
			}
			if strings.Contains(line, tc.unwired) {
				t.Errorf("%s: the line sends the operator to %q, which is the other runtime's door:\n%s", tc.rt.Name, tc.unwired, line)
			}
		}
	}
	// The built-in's line cites the record that makes it upstream's, since a
	// door with no reason behind it is one an onboarder argues with.
	if !strings.Contains(DetectionRefusal(bob, ManifestReading{Argv0: "bob", State: ManifestUnknownAgent}).Error(), "ADR 0060 D2") {
		t.Error("a built-in's refusal must cite ADR 0060 D2, which is what makes detection upstream's rather than the operator's")
	}
}
