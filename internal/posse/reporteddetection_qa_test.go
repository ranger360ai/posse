//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-rx7l7 — ADR 0061 D1–D2, the two overlayable keys
// and the launch's report-surface reading.
//
// THE GAP THESE HOLD. ranger-base-d8riq made `unknown_agent` refuse a
// bead-carrying launch, and the reason it gave was true of every runtime that
// had ever met it: a session herdr has no manifest for is
// `agent_not_found` and cannot be addressed at all. It is FALSE of a pane an
// outside authority labels through `herdr pane report-agent` — herdr's own
// "integrate your own agent" route, the route every herdr plugin takes, and
// the route ranger-base-8eqaa already built typed delivery onto. So the
// refusal walled off dispatch to exactly the panes upstream's extension model
// produces, and 8eqaa's delivery route had no dispatch caller.
//
// WHAT WOULD FAIL SILENTLY WITHOUT THESE ARMS:
//
//   - lift the refusal on the DECLARATION alone and a box whose herdr has no
//     `pane report-agent` verb at all (0.8.2 — on a seat of this shop the
//     same day as 0.9.1, ranger-base-v1yrt) spends a worktree, a workspace
//     and a pane to wait out startup_wait for a label nothing can send.
//     TestQAReportedRuntimeWithNoReportSurfaceRefusesAndSpendsNothing.
//   - read the surface probe's FAILURE as "no" and the same box walls itself
//     the moment herdr goes off PATH or a call hangs. That is the UNKNOWN
//     rule d8riq wrote for the manifest reading, one reading over.
//   - let `reported` block in `runtime check` and `posse runtime probe`
//     refuses — the probe walks RuntimeGaps and stops at the first Blocking
//     one — so the single surface that can MEASURE what detection reads on a
//     reported runtime is refused by the check that cannot tell it apart.
//   - accept `detection: reported` with no `detection_why:` and the failure
//     line that exists to name the missing authority has nothing to name.
//   - print "check the session" when no label arrives and the operator reads
//     a slow start, raises `startup_wait:`, and waits longer for a report
//     that nothing is going to send (the plugin's watcher needs a hand start
//     today, ranger-base-p8afi #3).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The declaration as the reported arms write it: the plugin id and its
// install date, which is the honest form of an authority's name.
const (
	qaReportedWhy  = "herdr-bob plugin MartinLoeper/herdr-bob, installed 2026-09-28"
	qaReportedDecl = "detection: " + DetectionReported
)

func qaReportedLines() []string {
	return []string{qaReportedDecl, "detection_why: " + qaReportedWhy}
}

// squashSpace collapses every run of whitespace to one space. The grid WRAPS
// (wrapGrid), so a declaration quoted on screen has a newline and an indent
// somewhere in the middle of it at a column nobody chose — asserting the
// unwrapped sentence against the screen would be asserting the wrap width.
func squashSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// noReportSurface arms the shared fake as a herdr with no `pane report-agent`
// verb — the 0.8.2 shape. PRESENT is the fake's default, because it models
// 0.9.1 everywhere else in this package.
func noReportSurface(t *testing.T, fake string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fake, "no-report-surface"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// ADR 0061 Verification 1. Both bad shapes refuse the LOAD, and each refusal
// names what the file should have said.
//
// They are two different mistakes and the refusals are not interchangeable. A
// wrong VALUE is the `prompt: arvg` shape — a typo that reads as a
// declaration and is silently demoted to the default, and the default here is
// the reading that refuses the launch, so the operator who typed `reproted`
// would be told their runtime is undetectable and never told why their key
// did nothing. A `reported` with NO WHY is not a wrong declaration at all:
// the value is one of the two, and what is absent is the sentence the launch's
// failure line prints. So that one names both keys.
func TestQADetectionKeyRefusesBothBadShapesAndNamesWhatIsMissing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{
			"a value that is neither",
			"command: mycli {file}\ndetection: reproted\n",
			[]string{"detection:", `"reproted"`, DetectionHerdr, DetectionReported, "ADR 0061"},
		},
		{
			"reported with no why",
			"command: mycli {file}\n" + qaReportedDecl + "\n",
			[]string{"detection_why:", "detection:", "authority", "ADR 0061"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := checkApp(t)
			writeRuntimeFile(t, a, "mycli", tc.body)
			_, err := a.LoadRuntime("mycli")
			if err == nil {
				t.Fatalf("%s must refuse the load — a declaration that reads as one and does nothing is the silence this contract exists to remove", tc.name)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("the refusal must name %q: %v", w, err)
				}
			}
		})
	}

	// The negative control, without which both arms above are green for a
	// loader that refused the key outright: the well-formed pair LOADS, and
	// both halves arrive.
	a := checkApp(t)
	rt := writeRuntime(t, a, "mycli", "command: mycli {file}\n"+strings.Join(qaReportedLines(), "\n")+"\n")
	if rt.DetectionMode() != DetectionReported || rt.DetectionWhy != qaReportedWhy {
		t.Errorf("a well-formed declaration must arrive whole: detection=%q why=%q", rt.DetectionMode(), rt.DetectionWhy)
	}
	// And absent is `herdr`, reached by doing nothing — the loud default,
	// because it is the reading that refuses.
	bare := writeRuntime(t, a, "barecli", "command: barecli {file}\n")
	if bare.DetectionMode() != DetectionHerdr || bare.Detection != "" {
		t.Errorf("an undeclared runtime must resolve to %s with the field left empty: %q/%q", DetectionHerdr, bare.DetectionMode(), bare.Detection)
	}
}

// ADR 0061 Verification 2, over ONE fake and all three declarations, because
// separate expectations are how a grid and a launcher drift apart — which is
// the failure ADR 0013 §1's launch row is a repeat of three times over.
//
// The same `unknown_agent` answer, read three ways: undeclared it BLOCKS and
// `runtime check` exits 1 (d8riq's line, unchanged); declared `reported` it is
// a non-blocking degrade quoting the authority, and the check exits 0 — which
// is what lets `posse runtime probe` run at all, since the probe refuses on
// the first Blocking gap; and on a runtime herdr DOES have a manifest for the
// same declaration is inert and says to drop the key.
func TestQAReportedDetectionIsADegradeAndAnInertDeclarationIsNamed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		names     []string
		decl      []string
		blocking  bool
		wantLine  []string
		wantClean bool
	}{{
		name:      "undeclared and unknown to herdr — d8riq's blocking gap",
		names:     []string{"claude", "codex", "grok"},
		decl:      nil,
		blocking:  true,
		wantLine:  []string{"agent_not_found", detectionDoc},
		wantClean: false,
	}, {
		name:      "declared reported — a NON-blocking degrade that names the authority",
		names:     []string{"claude", "codex", "grok"},
		decl:      qaReportedLines(),
		blocking:  false,
		wantLine:  []string{qaReportedWhy, "reporter's word", "posse runtime probe"},
		wantClean: true,
	}, {
		name:      "declared reported on a runtime herdr detects — INERT",
		names:     []string{"claude", "mycli"},
		decl:      qaReportedLines(),
		blocking:  false,
		wantLine:  []string{"INERT", "drop detection:"},
		wantClean: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			herdrNames(t, fake, tc.names...)
			undetectableFixture(t, b, PromptTyped, `[]`, tc.decl...)
			rt, err := b.App.LoadRuntime("mycli")
			if err != nil {
				t.Fatal(err)
			}

			var line string
			blocking := false
			for _, g := range b.App.RuntimeGaps(rt, b.H) {
				if g.Name == "detection" {
					line, blocking = g.Line, g.Blocking
				}
			}
			if line == "" {
				t.Fatalf("no detection gap reported at all — every one of these three readings is worth a line")
			}
			if blocking != tc.blocking {
				t.Errorf("detection gap blocking=%v, want %v — a blocking gap refuses `posse runtime probe`, which is the one surface that can measure a reported runtime's detection:\n%s", blocking, tc.blocking, line)
			}
			for _, w := range tc.wantLine {
				if !strings.Contains(line, w) {
					t.Errorf("the gap line must carry %q:\n%s", w, line)
				}
			}

			// The screen an operator reads, and its exit code. `posse runtime
			// check` exits 1 on a blocking gap and 0 otherwise (cmd/posse),
			// so the returned bool IS the exit code.
			var out strings.Builder
			if clean := b.App.RuntimeCheck(rt, b.H, &out); clean != tc.wantClean {
				t.Errorf("runtime check clean=%v, want %v (exit %d):\n%s", clean, tc.wantClean, map[bool]int{true: 0, false: 1}[clean], out.String())
			}
			// And the launch ROW carries the same reading the gap does, which
			// is the whole reason there is one function behind both.
			if !strings.Contains(out.String(), "launch") {
				t.Fatalf("the grid has no launch row:\n%s", out.String())
			}
			if tc.decl != nil && !strings.Contains(squashSpace(out.String()), squashSpace(qaReportedWhy)) {
				t.Errorf("the grid never quotes detection_why:, which is the only thing on the screen that names WHO labels these panes:\n%s", out.String())
			}
		})
	}
}

// ADR 0061 Verification 3, first half — the d8riq dispatch pins INVERTED on
// the same fixture. A bead routed to a persona on a runtime herdr cannot name
// is refused with nothing spent; add `detection: reported` to that profile and
// the same bead is dispatched: the workspace is created and the bead is
// claimed.
//
// Both ladders, because they order the claim and the create differently (ADR
// 0013 §2) and the refusal's placement was written for that difference — an
// arm that lifted the refusal on only one of them would pass whichever half a
// single-ladder test happened to drive.
func TestQAReportedRuntimeDispatchesWhereAnUndeclaredOneIsRefused(t *testing.T) {
	t.Parallel()
	for _, ladder := range []string{PromptTyped, PromptArgv} {
		t.Run("on the "+ladder+" ladder", func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			// herdr has manifests for the three built-ins and NOT for mycli —
			// byte for byte the arm that refuses in detection_qa_test.go.
			herdrNames(t, fake, "claude", "codex", "grok")
			undetectableFixture(t, b, ladder, `[{"id":"a-1","title":"t","labels":["go"]}]`, qaReportedLines()...)
			idleClaude(t, fake)

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out, log, bdlog := dispatcherOut(d), calls(t, fake), bdCalls(t, fake)
			if n != 1 {
				t.Fatalf("a runtime declaring %s must dispatch — the label comes from outside, so agent_not_found is not the reason it was refused for: dispatched %d\n%s\n%s", DetectionReported, n, out, log)
			}
			if !strings.Contains(log, "workspace create") {
				t.Errorf("no workspace was created for a launch that proceeded:\n%s\n%s", log, out)
			}
			if !strings.Contains(bdlog, "--claim") {
				t.Errorf("the bead was never claimed on a launch that proceeded:\n%s\n%s", bdlog, out)
			}
			// The d8riq refusal must be gone by NAME, not merely absent from
			// the count: a launch that proceeded while still printing "no
			// detection manifest ... cannot be addressed at all" would be a
			// launcher telling an operator the opposite of what it did.
			if strings.Contains(out, "launch refused") || strings.Contains(out, "cannot be addressed at all") {
				t.Errorf("a reported runtime must not carry the i3q6g refusal's sentence:\n%s", out)
			}
			// The one line the launch prints instead is pinned on its own
			// stream, in TestQAReportedLaunchPrintsOneLineNamingTheAuthority:
			// the backend's warn writer is not the dispatcher's, and asking
			// for it here would pass over a launch that printed it twice.
		})
	}
}

// The line that says detection here is BY REPORT, and whose — asked at
// planLaunch, the one function under every launch path, so it is printed once
// per launch and not once per surface that thought of it.
//
// Separate from the dispatch arm above because the dispatcher's own stream and
// the backend's warn stream are different writers, and pinning it here is what
// says WHERE the sentence lives: delete the planLaunch arm and this reds while
// every dispatch assertion stays green.
func TestQAReportedLaunchPrintsOneLineNamingTheAuthority(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	herdrNames(t, fake, "claude", "codex", "grok")
	undetectableFixture(t, b, PromptTyped, `[]`, qaReportedLines()...)
	warn := warnBuf(t, b)
	dir := t.TempDir()

	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger", Bead: "a-1"}); err != nil {
		t.Fatalf("a bead-carrying launch onto a reported runtime whose herdr carries the surface must PROCEED: %v", err)
	}
	got := warn.String()
	for _, want := range []string{"BY REPORT", qaReportedWhy, `"mycli"`} {
		if !strings.Contains(got, want) {
			t.Errorf("the launch must say once that detection here is by report, and whose (%q missing):\n%s", want, got)
		}
	}
	if strings.Contains(got, "DEGRADED") {
		t.Errorf("a reported launch that PROCEEDED is not the ADR 0015 §3 degrade — that word belongs to the interactive escape hatch on a refusing reading:\n%s", got)
	}
	if n := strings.Count(got, "BY REPORT"); n != 1 {
		t.Errorf("said %d times; one launch prints the line once (planLaunch is the single placement):\n%s", n, got)
	}

	// The negative control: an INERT declaration prints nothing at all here.
	// It changes no launch, and `runtime check` is where an operator is told
	// to drop a key — a line on every pane of a runtime that works perfectly
	// is noise.
	at := len(warn.String())
	herdrNames(t, fake, "claude", "mycli")
	if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: dir, Agent: "ranger", Bead: "a-2"}); err != nil {
		t.Fatalf("an inert declaration refuses nothing: %v", err)
	}
	if rest := warn.String()[at:]; strings.Contains(rest, "BY REPORT") || strings.Contains(rest, "INERT") {
		t.Errorf("an inert declaration must say nothing at launch — herdr's own detection wins and nothing about the launch changed:\n%s", rest)
	}
}

// ADR 0061 Verification 3, second half, and the refusal this record ADDS. A
// runtime declaring `reported` on a herdr with no `pane report-agent` verb has
// declared an observable this binary cannot produce: nothing can label the
// pane, so the launch can only wait out its startup_wait — having spent a
// worktree, a workspace, a pane and the runtime's own first turn.
//
// Nothing spent and nothing claimed, on the ARGV ladder especially: it claims
// before it creates, so only a refusal above both delivery branches leaves the
// bead untouched.
func TestQAReportedRuntimeWithNoReportSurfaceRefusesAndSpendsNothing(t *testing.T) {
	t.Parallel()
	for _, ladder := range []string{PromptTyped, PromptArgv} {
		t.Run("on the "+ladder+" ladder", func(t *testing.T) {
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			herdrNames(t, fake, "claude", "codex", "grok")
			noReportSurface(t, fake)
			undetectableFixture(t, b, ladder, `[{"id":"a-1","title":"t","labels":["go"]}]`, qaReportedLines()...)
			idleClaude(t, fake)

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out, log, bdlog := dispatcherOut(d), calls(t, fake), bdCalls(t, fake)
			if n != 0 {
				t.Fatalf("dispatched %d bead(s) onto a herdr that has no verb for anyone to report through — the label can never arrive:\n%s", n, out)
			}
			if strings.Contains(log, "workspace create") {
				t.Errorf("a workspace was created for a session nothing can label:\n%s\n%s", log, out)
			}
			if strings.Contains(bdlog, "--claim") {
				t.Errorf("the bead was claimed before the refusal — the refuse must sit above the claim and above both delivery branches:\n%s\n%s", bdlog, out)
			}
			// The line names the SURFACE and the DECLARATION, which is what
			// makes a wrong declaration visible as a wrong declaration, and
			// the door is a herdr UPGRADE — not the manifest runbook, because
			// nothing here is the profile's fault.
			for _, want := range []string{`argv0 "mycli"`, "pane report-agent", qaReportedWhy, "0.9.0", "herdr --version", "posse runtime check mycli"} {
				if !strings.Contains(out, want) {
					t.Errorf("the refusal must carry %q:\n%s", want, out)
				}
			}
			if strings.Contains(out, detectionDoc) {
				t.Errorf("the refusal sends the operator to author a manifest, which fixes nothing here — the profile is fine and the binary is old:\n%s", out)
			}
			if strings.Contains(out, "check the session") {
				t.Errorf("the refusal points at a session that was never created:\n%s", out)
			}
		})
	}

	// ADR 0015 §3's asymmetry, on the NEW refusing state. A bead-carrying
	// launch refuses; the operator's own `posse new` proceeds, loudly. It has
	// to hold here for the same reason it holds for a missing manifest — the
	// interactive session is the one route that can end the refusal, and here
	// ending it means standing the pane up and watching whether an upgrade
	// actually reports — and it needs its own arm because the DEGRADED line
	// branches on the cause: the door is a herdr upgrade, not the manifest
	// runbook, and nothing about this profile needs authoring.
	t.Run("interactive warns and proceeds", func(t *testing.T) {
		b, fake := newTestBackend(t)
		herdrNames(t, fake, "claude", "codex", "grok")
		noReportSurface(t, fake)
		undetectableFixture(t, b, PromptTyped, `[]`, qaReportedLines()...)
		warn := warnBuf(t, b)
		dir := t.TempDir()

		if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger", Bead: "a-1"}); err == nil {
			t.Fatal("a bead-carrying launch must refuse at planLaunch too — it is the backstop under a cockpit `d` and a recipe, and the placement `posse relaunch` needs")
		}
		if _, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: dir, Agent: "ranger"}); err != nil {
			t.Fatalf("an interactive launch must PROCEED — it is the route that can measure whether an upgraded herdr reports at all: %v", err)
		}
		got := warn.String()
		for _, want := range []string{"DEGRADED", "pane report-agent", qaReportedWhy, "herdr --version"} {
			if !strings.Contains(got, want) {
				t.Errorf("an interactive launch that proceeds must say what it is opening on (%q missing):\n%s", want, got)
			}
		}
		if strings.Contains(got, detectionDoc) {
			t.Errorf("the interactive warn sends the operator to author a manifest, which fixes nothing here:\n%s", got)
		}
		// And a RECREATE is on the refusing side, not the interactive one —
		// the ranger-base-enmu2 arm, which is decided by what the launch
		// SPENDS and not by whether a bead rode in.
		if _, err := b.planLaunch(NewSessionOpts{Name: "s3", Dir: dir, Agent: "ranger", Recreate: true}); err == nil {
			t.Error("a recreate must refuse: it buys a session posse cannot read with one that is alive")
		}
	})
}

// UNKNOWN is never a "no", one reading over — d8riq's rule applied to the
// surface probe. A herdr that cannot ANSWER whether it has `pane report-agent`
// decides nothing, and the launch proceeds exactly as it does when the verb is
// there.
//
// Two arms, because the two ways of not knowing reach the reading by different
// routes and only one of them is reachable from a fake: the call HANGS (armed
// here, with the reading's own control timeout cut so the test does not wait
// two minutes), and the reading assembled with the zero Surface, which is what
// every caller that never asked holds. Both must refuse nothing, and
// ReportSurfaceAbsent must be reachable only by herdr having answered.
func TestQAUnknownReportSurfaceRefusesNothing(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	herdrNames(t, fake, "claude", "codex", "grok")
	noReportSurface(t, fake) // armed AND hung: the hang must win
	if err := os.WriteFile(filepath.Join(fake, "report-surface-hang"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	undetectableFixture(t, b, PromptTyped, `[]`, qaReportedLines()...)
	rt, err := b.App.LoadRuntime("mycli")
	if err != nil {
		t.Fatal(err)
	}
	// The manifest half of the reading must still answer, so the control
	// timeout has to be generous enough for a fake process and short enough
	// that a 5s sleep blows it.
	h := Herdr{Bin: b.H.Bin, ControlTimeout: 2 * time.Second, Hangw: &strings.Builder{}}
	r := ReadDetection(h, rt)

	if !r.Reported() {
		t.Fatalf("a hung surface probe must not cost the reading its reported arm: %+v", r)
	}
	if r.Surface != ReportSurfaceUnknown {
		t.Errorf("a hung `pane report-agent --help` is UNKNOWN, never absent: surface=%v", r.Surface)
	}
	if r.Refuses() || r.NoReportSurface() {
		t.Errorf("a herdr that could not be ASKED refuses nothing — that is the rule the manifest reading already keeps: %+v", r)
	}
	// And the reading nobody asked: the zero Surface on a `reported` runtime
	// is the same answer, which is why the two share a value.
	zero := ManifestReading{Argv0: "mycli", State: ManifestUnknownAgent, Declared: DetectionReported, Why: qaReportedWhy}
	if zero.Refuses() || zero.NoReportSurface() {
		t.Error("the ZERO ReportSurface must be the safe one — a reading assembled without asking must not refuse a launch")
	}
	absent := ManifestReading{Argv0: "mycli", State: ManifestUnknownAgent, Declared: DetectionReported, Surface: ReportSurfaceAbsent}
	if !absent.NoReportSurface() || !absent.Refuses() {
		t.Error("ReportSurfaceAbsent is the one surface value that refuses, and it must — otherwise nothing does and the 0.8.2 box spends a pane per bead")
	}
}

// ADR 0061 D2 property 2: the wait's failure line. The observable on a
// reported runtime is the LABEL APPEARING, and an absent label says nothing
// about the pane — the CLI may be up and working with nobody having reported
// it. So the line names the declaration, quotes the authority, and puts
// `herdr plugin list` first.
//
// It must NOT say "check the session", and it must not offer a larger
// `startup_wait:` as the first move: on this runtime a bigger wait is patience
// for a report nothing is going to send (the herdr-bob watcher needs a hand
// start today, ranger-base-p8afi #3). The detected runtime's own sentence is
// the control — there "check the session" is correct, because there IS a
// session and looking at it is what answers.
func TestQAReportedWaitFailureNamesTheAuthorityAndNotTheSession(t *testing.T) {
	t.Parallel()
	rr := ManifestReading{Argv0: "mycli", State: ManifestUnknownAgent, Declared: DetectionReported, Why: qaReportedWhy, Surface: ReportSurfacePresent}
	line := NoAgentLine(rr, "s1", 45*time.Second)
	for _, want := range []string{qaReportedWhy, "herdr plugin list", "45s", "s1"} {
		if !strings.Contains(line, want) {
			t.Errorf("the reported wait's failure line must carry %q:\n%s", want, line)
		}
	}
	for _, bad := range []string{"check the session", "startup_wait"} {
		if strings.Contains(line, bad) {
			t.Errorf("the reported wait's failure line must not say %q — an absent label is not evidence about the CLI, and a bigger wait is patience for a report nothing will send:\n%s", bad, line)
		}
	}

	// The control. Without it the assertions above are green for a line that
	// printed the reported sentence on every runtime there is.
	det := ManifestReading{Argv0: "othercli", State: ManifestKnown, Declared: DetectionHerdr, Version: "2026.09.28.1"}
	plain := NoAgentLine(det, "s2", 45*time.Second)
	if !strings.Contains(plain, "check the session") {
		t.Errorf("on a runtime herdr detects, the session IS the thing to check — that sentence must survive:\n%s", plain)
	}
	if strings.Contains(plain, qaReportedWhy) || strings.Contains(plain, "herdr plugin list") {
		t.Errorf("a detected runtime's failure line must not quote a declaration it does not carry:\n%s", plain)
	}
	// And the ZERO reading — what a caller with no loadable profile passes —
	// renders that same sentence, which is why there is only one copy of it.
	if NoAgentLine(ManifestReading{}, "s2", 45*time.Second) != plain {
		t.Errorf("the zero reading must render the detected sentence; a caller that cannot load a profile has nothing to quote:\n%s", NoAgentLine(ManifestReading{}, "s2", 45*time.Second))
	}
}

// ADR 0061 D2 property 2 asked of the WIRING — ranger-base-mmvrh finding 2,
// the same shape as finding 1 one property over.
//
// TestQAReportedWaitFailureNamesTheAuthorityAndNotTheSession is a good pin on
// the SENTENCE, and it hands NoAgentLine a hand-built reading. Nothing read
// the line off a dispatch, so `(*Dispatcher).noAgentLine` could return
// `NoAgentLine(ManifestReading{}, session, wait)` — the zero reading, which is
// its own load-failure fallback and therefore the most plausible wrong thing
// there — and the whole suite stayed green while every reported runtime's wait
// failed with "check the session".
//
// That sentence is what the operator acts on. The observable this loop waits
// for is an authority LABELLING the pane; an absent label says nothing about
// the CLI, which may be up and working with nobody having reported it. So
// "check the session" sends them to read a healthy screen, and the move it
// invites — a larger `startup_wait:` — is patience for a report nothing is
// going to send (ranger-base-p8afi #3). The door is `herdr plugin list`.
//
// Both ladders, because they reach awaitTarget through different callers
// (awaitAgent on the typed one, awaitDelivered on the argv one) and a pin on
// either alone would pass over the other.
func TestQAReportedWaitFailureIsWiredAtTheDispatch(t *testing.T) {
	t.Parallel()
	for _, ladder := range []string{PromptTyped, PromptArgv} {
		t.Run("on the "+ladder+" ladder", func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			d := newTestDispatcher(t, b)
			d.StartupWait = 150 * time.Millisecond
			herdrNames(t, fake, "claude", "codex", "grok")
			undetectableFixture(t, b, ladder, `[{"id":"a-1","title":"t","labels":["go"]}]`, qaReportedLines()...)
			// agents.json absent on purpose: the launch PROCEEDS (the surface
			// is there, so this is a runtime posse will dispatch), and then
			// nothing ever labels the pane — which is precisely the state ADR
			// 0061 D2 property 2 is about, and the one a bigger wait does not
			// fix.

			n, err := d.Run("", "", 0)
			if err != nil {
				t.Fatal(err)
			}
			out := dispatcherOut(d)
			if n != 0 {
				t.Fatalf("dispatched %d bead(s) into a session no authority ever labelled:\n%s", n, out)
			}
			for _, want := range []string{"no agent reported", qaReportedWhy, "herdr plugin list", "detection: " + DetectionReported} {
				if !strings.Contains(squashSpace(out), squashSpace(want)) {
					t.Errorf("the wait's failure line must carry %q — it is the whole of what the operator can act on here:\n%s", want, out)
				}
			}
			// The two sentences that send them the wrong way. "check the
			// session" is a claim about the CLI that an absent label cannot
			// support, and naming the knob invites the one move that buys
			// nothing.
			for _, bad := range []string{"check the session", "no agent detected", "startup_wait"} {
				if strings.Contains(out, bad) {
					t.Errorf("the reported wait's failure must not say %q — an absent label is not evidence about the CLI, and a bigger wait is patience for a report nothing will send:\n%s", bad, out)
				}
			}
		})
	}

	// The load-failure fallback, which is the branch the mutant above
	// impersonates: a runtime whose profile will not load has made no
	// declaration posse may quote, so it gets the sentence this line has
	// always printed. Asked of the dispatcher's own method rather than the
	// renderer, because the thing being pinned is which reading noAgentLine
	// passes on, and the two arms bracket it from both sides: a noAgentLine
	// that ALWAYS takes the fallback reds on the two ladders above, and one
	// that never takes it reds here — on today's code by dereferencing the nil
	// profile LoadRuntime returned with its error, which is the other reason
	// this branch is not decoration.
	t.Run("a profile that will not load keeps the sentence it always had", func(t *testing.T) {
		t.Parallel()
		b, _ := newTestBackend(t)
		d := newTestDispatcher(t, b)
		line := d.noAgentLine("s9", "no-such-runtime-profile", 45*time.Second)
		if !strings.Contains(line, "check the session") || !strings.Contains(line, "s9") {
			t.Errorf("a runtime posse could not load has nothing to quote, so the session IS the thing to check:\n%s", line)
		}
		if strings.Contains(line, "herdr plugin list") || strings.Contains(line, DetectionReported) {
			t.Errorf("a profile that would not load must not be credited with a declaration:\n%s", line)
		}
	})
}

// ADR 0061 D3.4 — the one arm this record leaves exactly as it found it. The
// RE-TYPE path fires on "the workspace is alive and herdr reports no agent in
// it", and on a reported runtime that is the AUTHORITY's silence, not the
// CLI's death: a live pane whose reporter has not labelled it reads
// identically. So the launch line must not be typed there either, and the
// refusal says which of the two silences it is.
//
// This is the one predicate that is deliberately NOT Refuses(): a reported
// runtime whose herdr carries the surface LAUNCHES and still refuses a retype.
// Nothing else in this file would notice if that arm were wired to the launch
// predicate instead — which was true of the whole suite until
// TestQAReportedRuntimeRetypeRefusalIsWiredAtTheRelaunch below, the arm that
// drives RelaunchAgent for real and is the one that now kills that
// substitution (ranger-base-mmvrh finding 1). This test keeps the READING and
// the SENTENCE; that one keeps the CALL SITE.
func TestQAReportedRuntimeStillRefusesARetype(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := writeRuntime(t, a, "mycli", "command: mycli {file}\n"+strings.Join(qaReportedLines(), "\n")+"\n")
	r := ManifestReading{Argv0: "mycli", State: ManifestUnknownAgent, Declared: DetectionReported, Why: qaReportedWhy, Surface: ReportSurfacePresent}

	if r.Refuses() {
		t.Fatal("this reading must NOT refuse a launch — that is the whole of ADR 0061 D2; the retype arm refuses for a different reason")
	}
	if !r.NoManifest() {
		t.Fatal("the retype arm reads NoManifest(), which is herdr's raw answer and is true here")
	}
	err := DetectionRetypeRefusal(rt, r, "s1")
	for _, want := range []string{"refusing to retype", qaReportedWhy, "herdr plugin list", "composer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the retype refusal on a reported runtime must carry %q: %v", want, err)
		}
	}
	// It must not borrow the detected sentence: "a runtime it cannot name" is
	// not why this one refuses, and an operator sent to author a manifest for
	// a pane an authority is supposed to label is sent to the wrong place.
	if strings.Contains(err.Error(), detectionDoc) {
		t.Errorf("the reported retype refusal points at the manifest runbook, which is not this runtime's door: %v", err)
	}
}

// The same property as the test above, asked of the WIRING instead of the
// renderer — ranger-base-mmvrh finding 1, and the arm the comment above
// predicted would be missing.
//
// TestQAReportedRuntimeStillRefusesARetype asserts the reading
// (`!Refuses()`, `NoManifest()`) and the refusal's SENTENCE
// (DetectionRetypeRefusal called directly), and never the call site. So
// herdrback.go's `if det := ReadDetection(b.H, rt); det.NoManifest()` could be
// wired to `det.Refuses()` — the predicate every CREATE path in this file
// uses, which is what makes it the plausible wrong one here — and nothing in
// the suite would red. The
// undetectable end of the same wiring IS driven for real
// (relaunchdetection_qa_test.go), but both of its fixtures declare nothing, so
// `Refuses()` is true for them and the substitution is invisible there.
//
// This is the costliest end of the whole gap. On a reported runtime whose
// herdr HAS the report surface the launch PROCEEDS, so the pane is real and
// its CLI is alive; `Refuses()` is false and `NoManifest()` is true, so under
// the wrong predicate RelaunchAgent types the persona's launch line into a
// live composer as a chat turn — a work prompt arriving at the operator's own
// session as if they had typed it. Everything the create paths spend is a
// pane; this spends a running session and a turn of the model's attention.
//
// The fixture is the sequence run for real, because a refusal on a rig that
// was never going to re-type measures nothing:
//
//	launch the session                        → proceeds (the surface is there)
//	age the launch past the grace, kill the CLI
//	relaunch                                  → what this pin is about
func TestQAReportedRuntimeRetypeRefusalIsWiredAtTheRelaunch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		names  []string
		refuse bool
	}{
		// herdr has no manifest for mycli and the profile declares the
		// authority: the launch proceeded, the pane is alive, and the retype
		// must still refuse. The ONE arm the predicate substitution shows up
		// in, because it is the one where the two predicates disagree.
		{"declared reported and herdr has no manifest — refuses", []string{"claude", "codex", "grok"}, true},
		// The negative control. An INERT declaration — herdr detects mycli
		// itself — re-types exactly as it did before ADR 0061, and without
		// this arm every assertion above holds equally for a RelaunchAgent
		// that had simply stopped re-typing on any runtime at all.
		{"an inert declaration re-types", []string{"claude", "mycli"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, fake := newTestBackend(t)
			agentPerLaunch(t, fake)
			herdrNames(t, fake, tc.names...)
			undetectableFixture(t, b, PromptTyped, `[]`, qaReportedLines()...)
			m := undetectableSession(t, b, "s1", t.TempDir())

			// The state this path reads as "the CLI died", which on a reported
			// runtime is the AUTHORITY's silence and says nothing about the
			// CLI: the launch is well past the grace, and herdr lists no agent
			// anywhere.
			m.Launched = time.Now().Add(-time.Hour)
			if err := b.writeMeta(m); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(fake, "agents.json")); err != nil {
				t.Fatal(err)
			}
			typedAtLaunch := strings.Count(calls(t, fake), "pane run")

			ok, err := b.RelaunchAgent("s1", time.Second)
			typed := strings.Count(calls(t, fake), "pane run") - typedAtLaunch

			if !tc.refuse {
				if err != nil || !ok {
					t.Fatalf("an inert declaration refuses nothing — herdr detects this argv0 itself, so a dead CLI must come back as it always did: ok=%v err=%v", ok, err)
				}
				if typed != 1 {
					t.Errorf("the persona line was not re-typed (%d pane runs), so this control measured nothing", typed)
				}
				return
			}
			if err == nil {
				t.Fatalf("a reported runtime's silent reporter is not evidence its CLI died — re-typing there lands the launch line in a LIVE composer as a chat turn (ADR 0061 D3.4): ok=%v", ok)
			}
			// The pin: the line was not typed. Every other assertion here
			// could hold for a refusal that still put the command in the pane,
			// which is the chat turn this arm exists to prevent.
			if typed != 0 {
				t.Errorf("%d launch line(s) were typed into the live pane after the refusal:\n%s", typed, calls(t, fake))
			}
			if ok {
				t.Error("RelaunchAgent reported it re-typed the session it had just refused")
			}
			// And it is THIS runtime's refusal, not the undetectable one's: an
			// operator sent to author a manifest for a pane an authority was
			// supposed to label is sent to the wrong door.
			for _, want := range []string{"refusing to retype", "s1", qaReportedWhy, "herdr plugin list"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal must carry %q:\n%v", want, err)
				}
			}
			if strings.Contains(err.Error(), detectionDoc) {
				t.Errorf("the reported retype refusal points at the manifest runbook, which is not this runtime's door:\n%v", err)
			}
			// The stamp this path measures its own grace against must be
			// untouched. Bumped, the refusal comes back next pass as "too
			// young to relaunch" — the same silence in a different costume.
			after, found := b.readMeta("s1")
			if !found || time.Since(after.Launched) < 30*time.Minute {
				t.Errorf("a refusal re-stamped launched: (%s ago) — the next pass would read this session as a CLI still starting up", time.Since(after.Launched).Round(time.Second))
			}
		})
	}
}
