//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-wtavx — ADR 0062 D3: "PID delivered" is a READING.
//
// THE GAP THESE HOLD. `runtime check`'s launch row printed *PID delivered by
// the template* unconditionally, and the only PID-delivery check on any
// launch path was `PIDVoided`, which asks whether a FLAG on the rendered line
// makes the CLI discard the PID and never whether the template carried one at
// all. ranger-base-5jjtn then measured bob's `-p` dead and took `{file}` off
// its template — and a dispatched bob seat spent a worktree, a pane and a
// `startup_wait` on a session carrying every native rulebook and no persona,
// which is PIDVoided's exact harm reached through a template that names no
// flag to refuse on. The row still read *delivered*. Fourth instance of the
// ADR 0013 §1 class (9r33's danger, vbp3's declared screen, i3q6g's
// detection, now the PID).
//
// WHAT WOULD FAIL SILENTLY WITHOUT THEM, which is why each arm is here:
//
//   - delete launchSession's refusal and the typed ladder still refuses one
//     line later inside planLaunch — with the bead already claimed on the
//     ARGV ladder, which claims before it creates. Only the no-`--claim`
//     assertion sees that (detection_qa_test.go's shape, and the same reason).
//   - refuse `posse new` too and the probe goes with it: `posse runtime
//     probe` renders a persona line, and it is the surface the instance side
//     MEASURES the channel that lifts this refusal with. A probe that refused
//     here could never measure its way out.
//   - make the preflight's `pid` gap non-blocking to keep the probe running
//     and `runtime check` stops being an onboarding gate on the one stage
//     whose absence costs the whole session.
//   - read the RENDERED LINE instead of the template and the reading is
//     green on every launch forever: `{file}` has become a quoted path by
//     then, indistinguishable from `{memory}`'s, and a template that carried
//     nothing renders a perfectly good launch line.
//   - let the grid, `agent check` and the launcher ask the question
//     separately and they drift, which is the failure this row is a repeat of
//     — four times now.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pidChannelFixture is one persona on one declared runtime whose `command:`
// is the lever, plus `beads` ready rows. The argv0 is `mycli` and the caller
// arms herdr to KNOW it, so detection refuses nothing and the only thing
// deciding these subtests is whether the template carries a channel.
//
// A template-only yaml because that is the shape this rule was cut for: bob
// is the only built-in that meets it today, and every `runtimes/<name>.yaml`
// whose `command:` forgot the PID is the same shape.
//
// prompt is the DELIVERY LADDER and it is a parameter for the reason ADR 0013
// §2 gives: the two ladders order the claim and the create differently, which
// is the whole reason dispatch's refusal sits above both branches. A fixture
// that exercised one would pin a placement that holds for the other by
// accident.
func pidChannelFixture(t *testing.T, b *HerdrBackend, prompt, ready, cmd string) string {
	t.Helper()
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "command: " + cmd + "\n"
	if prompt != "" {
		body += "prompt: " + prompt + "\n"
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

// The reading itself, and the one thing about it that cannot be asked any
// other way: it is a question about the TEMPLATE.
//
// The rendered-line arm is the pin that matters. ADR 0053 D1's `{model}`
// refusal asks about the rendered line and can, because a model id is a
// string the launch can look FOR; the PID's absence has no shape on a
// rendered line, so a reading taken there is a reading that is green forever.
func TestQAPIDChannelIsReadFromTheTemplateAndNamesBothPlaceholders(t *testing.T) {
	t.Parallel()
	rt := &Runtime{Name: "mycli"}
	for _, tc := range []struct {
		tmpl string
		want []string
	}{
		{"mycli --sys {file}", []string{PIDChannelFile}},
		{`mycli -c instructions="$(cat {file})" {allow}`, []string{PIDChannelFile}},
		{"mycli chat {mode} -w .", []string{PIDChannelMode}},
		{"mycli {mode} --sys {file}", []string{PIDChannelFile, PIDChannelMode}},
		{"mycli chat --auto-approve -w . {allow} {deny}", nil},
		// {memory} is a path on the line and is not the PID: a reading that
		// counted any placeholder carrying a path would read this as
		// delivered, which is the bob shape exactly.
		{"mycli --add-dir {memory} {allow}", nil},
	} {
		got := rt.PIDChannels(tc.tmpl)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("PIDChannels(%q) = %v, want %v", tc.tmpl, got, tc.want)
		}
	}

	// The RENDERED line of a template that DOES deliver reads as no channel
	// at all — which is why every caller passes the template and why
	// LaunchTemplate exists to name it.
	ag := loadTestAgent(t, "---\nname: p\nruntime: mycli\n---\nYou are p.\n")
	rt.Command = "mycli --sys {file}"
	line := ag.RenderCommandFor(rt, "mycli", DefaultTier)
	if !strings.Contains(line, ag.Path) {
		t.Fatalf("this control measures nothing — the rendered line carries no PID path: %s", line)
	}
	if len(rt.PIDChannels(line)) != 0 {
		t.Errorf("the rendered line must read as NO channel: the placeholder is gone by then, so a reading taken here is green on every launch forever:\n%s", line)
	}

	// {mode} IS named by the reading and is rendered by NOTHING yet: it is
	// ADR 0062 D1's placeholder, and this bead names it so D1 has somewhere
	// to land. No shipping template carries one, and a hand-written profile
	// that does gets the literal text on its launch line — which is a loud
	// failure, and the reason the renderer was NOT given a `{mode}` → ""
	// arm here. That arm would delete the channel the reading had just
	// credited, which is this bead's own defect: a sentence with no reading
	// behind it, pointing the other way.
	//
	// WHEN D1 LANDS this assertion inverts, and that is the point of
	// writing it down: whoever lands the materializer meets this line.
	modeOnly := &Runtime{Name: "mycli", Command: "mycli chat {mode} -w ."}
	if len(modeOnly.PIDChannels(modeOnly.Command)) != 1 {
		t.Error("the reading must name {mode} as a channel — D1's bead has nowhere to land otherwise")
	}
	if !strings.Contains(ag.RenderCommandFor(modeOnly, "mycli", DefaultTier), PIDChannelMode) {
		t.Error("something now renders {mode}: ADR 0062 D1 has landed, so the reading must start asking the runtime whether it can deliver through that channel instead of only whether the template spells it")
	}

	// LaunchTemplate is the one expression for "which template", shared with
	// RenderCommandForModel — and the PID's own `command:` counts only on the
	// PID's OWN runtime (ADR 0002 §1).
	pid := loadTestAgent(t, "---\nname: p\nruntime: mycli\ncommand: mycli --sys {file}\n---\nYou are p.\n")
	bare := &Runtime{Name: "other", Command: "other --no-pid-here"}
	if got := pid.LaunchTemplate(rt, "mycli"); got != pid.Command {
		t.Errorf("on its own runtime the PID's command: is the template: %q", got)
	}
	if got := pid.LaunchTemplate(bare, "mycli"); got != bare.Command {
		t.Errorf("on another runtime the RUNTIME's template is rendered, so that is what the reading must ask about: %q", got)
	}
	if len(bare.PIDChannels(pid.LaunchTemplate(bare, "mycli"))) != 0 {
		t.Error("a PID whose own command: carries {file} does not lend it to the runtime it is overridden onto")
	}
}

// The dispatch arm, over both delivery ladders and against its own negative
// control. Without the control every expectation here holds equally for a
// launcher that refused every bead on this runtime on principle.
func TestQANoPIDChannelRefusesDispatchAndATemplateWithOneLaunches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cmd    string
		refuse bool
	}{
		{"no channel refuses", "mycli --auto-approve", true},
		{"a {file} launches", "mycli --sys {file}", false},
	} {
		for _, ladder := range []string{PromptTyped, PromptArgv} {
			t.Run(tc.name+" on the "+ladder+" ladder", func(t *testing.T) {
				b, fake := newTestBackend(t)
				d := newTestDispatcher(t, b)
				herdrNames(t, fake, "claude", "mycli")
				pidChannelFixture(t, b, ladder, `[{"id":"a-1","title":"t","labels":["go"]}]`, tc.cmd)
				idleClaude(t, fake)

				n, err := d.Run("", "", 0)
				if err != nil {
					t.Fatal(err)
				}
				out, log, bdlog := dispatcherOut(d), calls(t, fake), bdCalls(t, fake)
				if !tc.refuse {
					if n != 1 || !strings.Contains(log, "workspace create") {
						t.Fatalf("a template that delivers the PID walls nothing — dispatched %d bead(s):\n%s\n%s", n, out, log)
					}
					if strings.Contains(out, "delivers no PID") {
						t.Errorf("a launch that proceeded still said the template carried no channel:\n%s", out)
					}
					return
				}
				if n != 0 {
					t.Errorf("dispatched %d bead(s) onto a template that delivers no PID — the session would carry every native rulebook and no persona:\n%s", n, out)
				}
				// Nothing spent, and the bead untouched. On the ARGV ladder
				// these two ARE the placement: it claims before it creates,
				// so a refusal one line later — from inside CreateSession —
				// hands back a bead it has already taken.
				if strings.Contains(log, "workspace create") {
					t.Errorf("a workspace was created for a session that would hold no persona:\n%s\n%s", log, out)
				}
				if strings.Contains(bdlog, "--claim") {
					t.Errorf("the bead was claimed before the refusal — the refuse must sit above the claim and above both delivery branches (ADR 0013 §1 property 3):\n%s\n%s", bdlog, out)
				}
				// The line names BOTH placeholders, the harm, the door and
				// the grid — and never a session, because there is none.
				for _, want := range []string{
					PIDChannelFile, PIDChannelMode,
					"every native rulebook and no persona",
					"posse runtime check mycli",
					"runtimes/mycli.yaml",
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

// ADR 0013 §1 property 5 / ADR 0015 §3's asymmetry, at planLaunch — the
// backstop under every launch path that is not the dispatch loop.
//
// The interactive half is load-bearing twice over: the operator at the
// keyboard can paste the PID into the session they just opened, and `posse
// runtime probe` renders a persona line and is how the channel that lifts
// this refusal gets measured at all.
func TestQANoPIDChannelRefusesABeadLaunchAndWarnsAnInteractiveOne(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	herdrNames(t, fake, "claude", "mycli")
	pidChannelFixture(t, b, PromptTyped, `[]`, "mycli --auto-approve")
	dir := t.TempDir()

	_, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger", Bead: "a-1"})
	if err == nil {
		t.Fatal("a bead-carrying launch onto a template that delivers no PID must refuse — nobody is watching it, and the session would answer as whatever its native rulebooks say")
	}
	for _, want := range []string{PIDChannelFile, PIDChannelMode, "posse runtime check mycli"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q and the door: %v", want, err)
		}
	}

	warn := warnBuf(t, b)
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: dir, Agent: "ranger"}); err != nil {
		t.Fatalf("an interactive launch must PROCEED — the keyboard can paste the PID, and the probe renders this same line to measure the channel that lifts the refusal: %v", err)
	}
	for _, want := range []string{"DEGRADED", PIDChannelFile, PIDChannelMode, "no persona"} {
		if !strings.Contains(warn.String(), want) {
			t.Errorf("an interactive launch that proceeds must still say what it is opening (%q missing):\n%s", want, warn.String())
		}
	}

	// The negative control, without which both halves above pass for a rule
	// that fires on every launch whatever the template says.
	at := len(warn.String())
	pidChannelFixture(t, b, PromptTyped, `[]`, "mycli --sys {file}")
	if _, err := b.planLaunch(NewSessionOpts{Name: "s3", Dir: dir, Agent: "ranger", Bead: "a-2"}); err != nil {
		t.Fatalf("a template that carries {file} must refuse nothing: %v", err)
	}
	if rest := warn.String()[at:]; strings.Contains(rest, "delivers no PID") {
		t.Errorf("a template with a channel must warn about nothing:\n%s", rest)
	}
}

// The RE-TYPE arm — the third path that renders a persona line, and the one
// that types into a pane that already exists. It is reachable even though the
// create refuses: the PID and the runtime file are both re-read from disk, so
// a `{file}` edited out of either AFTER the session opened arrives here, and
// that is the fixture below.
//
// Unconditional, because this path has one caller and it is the unattended
// one — the pin is the call log, no second `pane run`.
func TestQANoPIDChannelRefusesTheRetypeIntoALivePane(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		after  string
		refuse bool
	}{
		{"the channel edited away refuses", "mycli --auto-approve", true},
		{"the channel still there re-types", "mycli --sys {file}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			agentPerLaunch(t, fake)
			herdrNames(t, fake, "claude", "mycli")
			pidChannelFixture(t, b, PromptTyped, `[]`, "mycli --sys {file}")
			if err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
				t.Fatalf("the launch this arm revives must itself have happened: %v", err)
			}
			m, ok := b.readMeta("s1")
			if !ok {
				t.Fatal("no meta for the session that just launched")
			}
			if m.Runtime != "mycli" {
				t.Fatalf("the session came up on runtime %q, not mycli — NOTHING MEASURED", m.Runtime)
			}
			// The edit this path exists to catch, then the state it reads as
			// "the CLI died": past the grace, and herdr sees no agent.
			pidChannelFixture(t, b, PromptTyped, `[]`, tc.after)
			m.Launched = time.Now().Add(-time.Hour)
			if err := b.writeMeta(m); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(fake, "agents.json")); err != nil {
				t.Fatal(err)
			}
			typedAtLaunch := strings.Count(calls(t, fake), "pane run")

			relaunched, err := b.RelaunchAgent("s1", time.Second)
			typed := strings.Count(calls(t, fake), "pane run") - typedAtLaunch

			if !tc.refuse {
				if err != nil || !relaunched {
					t.Fatalf("a template that still delivers the PID must come back exactly as it did before this rule existed: ok=%v err=%v", relaunched, err)
				}
				if typed != 1 {
					t.Errorf("the persona line was not re-typed (%d pane runs), so this control measured nothing", typed)
				}
				return
			}
			if err == nil {
				t.Fatalf("re-typing a line that delivers no PID revives the session WITHOUT its persona: ok=%v", relaunched)
			}
			if typed != 0 {
				t.Errorf("%d line(s) were typed into the live pane before the refusal — the refusal must sit above the typing", typed)
			}
			for _, want := range []string{"s1", PIDChannelFile, PIDChannelMode} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the re-type refusal must name %q: %v", want, err)
				}
			}
		})
	}
}

// "The grid, `agent check` and the launch cannot disagree" — ADR 0062 D3's
// one-reading-three-surfaces shape, and the only assertion here that would
// have failed for a reason other than a missing branch: for four amendments
// running this grid promised something the launcher did not do.
//
// So it is written as an equivalence over one fixture and both readings,
// rather than as separate expectations that could drift apart again, and it
// reads the surfaces an operator and a dispatch actually meet: the printed
// launch row, the preflight's blocking bit, `agent check`'s finding on a
// PID's own `command:`, and the launch refusal itself.
func TestQAPIDChannelAgreesAcrossAllThreeSurfaces(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cmd  string
	}{
		{"no channel", "mycli --auto-approve"},
		{"a {file}", "mycli --sys {file}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			herdrNames(t, fake, "claude", "mycli")
			pidChannelFixture(t, b, PromptTyped, `[]`, tc.cmd)
			rt, err := b.App.LoadRuntime("mycli")
			if err != nil {
				t.Fatal(err)
			}

			var out strings.Builder
			b.App.RuntimeCheck(rt, b.H, &out)
			// Flattened, because the grid WRAPS at a fixed width to keep
			// itself on one screen: a sentence this row prints is split
			// across lines at a column nobody chose, and a Contains over the
			// raw screen is a pin that passes or fails on where the wrap
			// happened to land.
			grid := strings.Join(strings.Fields(out.String()), " ")
			printed := strings.Contains(grid, "NO PID channel")
			blocking := false
			for _, g := range b.App.RuntimeGaps(rt, b.H) {
				if g.Name == PIDChannelGapName && g.Blocking {
					blocking = true
				}
			}
			_, launchErr := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger", Bead: "a-1"})
			refuses := launchErr != nil

			// The fourth surface, on the template a PID OWNS rather than the
			// profile's: `agent check` is where that reader stands, and it
			// reads the same function.
			if err := os.WriteFile(filepath.Join(b.App.AgentsDir, "pidown.md"),
				[]byte("---\nname: pidown\ndescription: test\nruntime: mycli\ncommand: "+tc.cmd+"\n---\nYou are pidown.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			findings, _, err := b.App.CheckAgent("pidown")
			if err != nil {
				t.Fatal(err)
			}
			found := strings.Contains(strings.Join(findings, "\n"), "no PID channel")

			if printed != blocking || printed != refuses || printed != found {
				t.Errorf("%s — the grid's launch row says NO PID channel=%v, the preflight blocks=%v, a bead launch refuses=%v, agent check finds=%v; four readings across the three surfaces, all off one function:\n%s\nlaunch: %v\nfindings: %v",
					tc.name, printed, blocking, refuses, found, out.String(), launchErr, findings)
			}
			// And the grid marks it, so an operator reading the check sees
			// the same ✗ the launcher acts on.
			if printed != strings.Contains(grid, "✗ "+PIDChannelGapName) {
				t.Errorf("%s — the launch row and the preflight's ✗ disagree:\n%s", tc.name, out.String())
			}
			// The row says WHICH placeholder delivers it when one does — the
			// sentence this bead replaced said only "by the template".
			if !printed && !strings.Contains(grid, "PID delivered by "+PIDChannelFile) {
				t.Errorf("%s — the row must name the placeholder that delivers the PID:\n%s", tc.name, out.String())
			}
		})
	}
}

// The probe's exemption, and the reason it is an exemption rather than a
// non-blocking gap (ADR 0062 D3): `posse runtime probe` renders a persona
// line and is the surface the instance side measures the channel that LIFTS
// this refusal with, so it must still open its pane — while `runtime check`'s
// exit code stays an onboarding gate, because a runtime in this state really
// cannot take dispatched work.
//
// Pinned as the pair, in both directions: the gap is BLOCKING, and the
// probe's blocking-gap loop does not refuse on it.
func TestQAPIDChannelGapBlocksTheGridAndNotTheProbe(t *testing.T) {
	t.Parallel()
	// The app is built here rather than through probeParityApp, which lives
	// in arm 3 and so is not compiled into this arm (ranger-base-qp1hm's
	// split). The reserved argv0 spelling is the same idea: a name nothing
	// on this box resolves, so no arm of this test can reach a real CLI.
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "carol.yaml"),
		[]byte("command: posse-wtavx-no-such-exe --auto-approve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime("carol")
	if err != nil {
		t.Fatal(err)
	}
	h := Herdr{Bin: filepath.Join(t.TempDir(), "no-herdr")}

	blocking := false
	for _, g := range a.RuntimeGaps(rt, h) {
		if g.Name == PIDChannelGapName {
			if !g.Blocking {
				t.Errorf("the pid gap must BLOCK — a template that delivers no PID cannot take dispatched work, and `runtime check`'s exit code is the onboarding gate: %s", g.Line)
			}
			blocking = true
		}
	}
	if !blocking {
		t.Fatal("no pid gap on a template carrying neither placeholder — NOTHING MEASURED")
	}

	// The probe refuses here for a different reason — no herdr, so observable
	// 4 cannot be read — and that is the point: whatever stops it, it must
	// never be the gap it exists to measure its way out of.
	_, err = a.RuntimeProbe(rt, h, ProbeOpts{Timeout: time.Second})
	if err == nil {
		t.Fatal("this arm needs the probe to stop somewhere, so the sentence can be read")
	}
	if strings.Contains(err.Error(), "cannot be probed: "+PIDChannelGapName) {
		t.Errorf("the probe refused on the PID-channel gap — it renders a persona line and is how the channel that lifts this refusal gets measured, so it must warn and run: %v", err)
	}
}
