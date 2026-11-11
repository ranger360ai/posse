//go:build posse_arm3

package posse

// ranger-base-xdaw9: a typed prompt is REFUSED into a pane whose screen has
// no prompt box on it — the herdr half of ranger-base-hlnyx, over
// reportedagent.go's refuseWithoutComposerPane.
//
// WHAT WAS MEASURED, and it is the reason this file exists rather than a line
// in a profile. 2026-11-11, this box (linux, omarchy 7.2.15-1), herdr 0.9.1
// with no server running until this bead started one, claude 2.1.322,
// `claude-haiku-5-5`. A pane running a live claude, its agent label
// overridden through `pane report-agent --agent fakebob-xdaw9 --state idle`
// so that posse's route reader takes the typed route, then this route's own
// two calls fired into a modal three times:
//
//	pane send-text w1:p1 "PROBEXDAW9<n> …" ; pane send-keys w1:p1 enter
//	→ the token in `pane read` visible / recent / recent-unwrapped: 0, 0, 0
//	→ the Enter actuated the dialog's highlighted row, three for three:
//	  `❯ 1. Yes` on a Bash permission dialog TWICE — both marker files
//	  written at the second the prompt was typed — and `❯ No, exit` on the
//	  folder-trust prompt, which quit the CLI.
//
// EVERY SCREEN IN THIS FILE IS ONE OF THOSE SCREENS, taken off that pane with
// `herdr pane read --source detection --format text` — the source
// PaneReadDetection asks for and the source `herdr agent explain --file`
// replays (ADR 0066 D3). They are quoted rather than invented for the reason
// the Rex fixtures are (promptnocomposer_qa_test.go): the reading under test
// is entirely about what claude draws, so an invented screen would pin posse
// against posse. docs/notes.d/ranger-base-xdaw9.md has the venue and the
// region readings.
//
// THREE NORMALIZATIONS, none of them a glyph. The rules are written at 80
// columns rather than the capturing terminal's 120, because isFooterRule asks
// what a line is MADE of and not how long it is; the scratch paths are
// replaced, because a crew worktree path under internal/ is what
// `make crew-check` exists to find; and the transcript above each screen's
// chrome is trimmed to the lines that matter to the reading. What is NOT
// trimmed is the one transcript line that starts with the composer's own mark
// — an echoed prompt — because a reading that took THAT for the box would
// read every covered screen as open.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── the measured screens ────────────────────────────────────────────────────

// herdrScreenRule is claude's box rule: U+2500 and nothing else, which is
// what isFooterRule requires.
var herdrScreenRule = strings.Repeat("─", 80)

// herdrIdleScreen is the same pane settled, box drawn and empty (MEASURED).
// Lines 2-3 are the echoed prompt from the turn before, and the one at line 2
// opens with the composer's mark: the box is the one between the rules at the
// bottom, and nothing else on the screen is.
var herdrIdleScreen = "❯ Run exactly this bash command and nothing else: touch /tmp/scratch/probe/xdaw9-approved.txt\n\n" +
	"  Ran 1 shell command\n\n" +
	"● I ran the touch command you gave, and it completed with no output.\n\n" +
	"✻ Cogitated for 3s · done 12:25 AM\n\n\n\n" +
	herdrScreenRule + "\n❯ \n" + herdrScreenRule + "\n" +
	"  ⚠ Transcript saving is off — inherited CLAUDE_CODE_CHILD_SESSION marker · restart with CLAUDE_CODE_FORCE_SESSI…\n" +
	"  ⏸ manual mode on · ? for shortcuts"

// herdrPermissionDialogScreen is the dialog whose highlighted `1. Yes` this
// route's Enter pressed TWICE, running a shell command nobody reviewed
// (MEASURED). ONE U+2500 rule, the dialog's own separators drawn with U+254C,
// the selection cursor drawn with the composer's own mark, `Esc to cancel` in
// the last line.
var herdrPermissionDialogScreen = "  Creating empty second marker file in scratchpad probe directory\n" +
	"  ⎿  $ touch /tmp/scratch/probe/xdaw9-second.txt\n\n" +
	herdrScreenRule + "\n" +
	" Bash command\n" +
	" Tip: auto mode handles these prompts for you — choose \"switch to auto mode\" below\n" +
	" Create empty second marker file in scratchpad probe directory\n" +
	strings.Repeat("╌", 80) + "\n" +
	" │ touch /tmp/scratch/probe/xdaw9-second.txt\n" +
	strings.Repeat("╌", 80) + "\n" +
	" Do you want to proceed?\n" +
	" ❯ 1. Yes\n" +
	"   2. Yes, and always allow access to /tmp/scratch/probe from this project\n" +
	"   3. Yes, and switch to auto mode · auto mode handles these prompts for you\n" +
	"   4. No\n\n" +
	" Esc to cancel · Tab to amend"

// herdrModelPickerScreen is `/model` (MEASURED). It draws NO U+2500 rule at
// all — its own top rule is U+2594 — so the phrase arm of the chrome reading
// is the whole of what judges this screen, and its highlighted row is the one
// that wrote a default model into the operator's settings on the Rex route
// (ranger-base-8eeoy). herdr's OWN state for this screen is `unknown`: no
// manifest rule matched it, which is why the refusal cannot be built on that.
var herdrModelPickerScreen = "● Args from unknown skill: answer with exactly ok\n" +
	strings.Repeat("▔", 80) + "\n" +
	"   Select model\n" +
	"   Switch between Claude models. Your pick becomes the default for new sessions. For other/previous model names,\n" +
	"   specify with --model.\n\n" +
	"     1.  Default (recommended)  Fable 5.2\n" +
	"     2.  Opus 5.5               For complex work and everyday tasks\n" +
	"     3.  Fable 5.2              For your toughest challenges\n" +
	"     4.  Sonnet 5.5             Most efficient for simpler tasks\n" +
	"   ❯ 5.  Haiku 5.5 ✔            Fastest for quick answers\n" +
	"     ▸   More models\n\n" +
	"   ◐ Medium effort (default) ←/→ to adjust\n\n" +
	"   Enter to set as default · s to use this session only · Esc to cancel"

// herdrTrustDialogScreen is claude's folder-trust prompt (MEASURED) — the one
// a pane can be holding at LAUNCH, with `No, exit` highlighted, so the first
// typed prompt into a fresh pane is the Enter that quits it. That is what this
// route's Enter did on the first of the three attempts. It carries TWO U+2500
// rules and no box at all; it is also the one screen whose `prompt_box_body`
// region preview is NOT empty — herdr previews the dialog's own body there —
// which is why the region reading could not be the tell.
var herdrTrustDialogScreen = herdrScreenRule + "\n" +
	" Accessing workspace:\n\n" +
	" /tmp/scratch/probe\n\n" +
	" Quick safety check: Is this a project you created or one you trust? (Like your own code, a well-known open source\n" +
	" project, or work from your team). If not, take a moment to review what's in this folder first.\n\n" +
	" Claude Code'll be able to read, edit, and execute files here.\n\n" +
	" Security guide\n\n" +
	" ❯ No, exit\n" +
	"   Yes, I trust this folder\n\n" +
	" Enter to confirm · Esc to cancel"

// ─── the reading, over the screens this route actually reads ─────────────────

func TestQAHerdrPaneScreensAreJudgedByTheOneComposerReading(t *testing.T) {
	t.Parallel()
	// MEASURED 2026-11-11: captureComposerSight, written off `rex capture`
	// screens for ranger-base-hlnyx, judges all four of this route's
	// `pane read --source detection` screens correctly and unchanged. That
	// is what makes this route's refusal one reading rather than a second
	// predicate over herdr's region previews — which would be a second
	// spelling of the same judgement to keep in step with the same binary.
	for _, c := range []struct {
		name   string
		screen string
		want   composerSight
	}{
		{"settled pane, box drawn and empty", herdrIdleScreen, composerOpen},
		{"Bash permission dialog", herdrPermissionDialogScreen, composerCovered},
		{"the /model picker", herdrModelPickerScreen, composerCovered},
		{"the folder-trust prompt", herdrTrustDialogScreen, composerCovered},
		{"nothing of claude's on it", "prompt$ echo hi\nhi\nprompt$\n", composerUnreadable},
		{"herdr answered nothing", "", composerUnreadable},
	} {
		if got := captureComposerSight(c.screen); got != c.want {
			t.Errorf("captureComposerSight(%s) = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestQAHerdrIdleScreensTranscriptMarkIsNotReadAsTheBox(t *testing.T) {
	t.Parallel()
	// The echoed prompt at the top of the idle screen opens with `❯`, and so
	// does every dialog's highlighted row. The mark is never what says "there
	// is a box here" (composerMark's own warning, panework.go) — on this
	// screen source as on the other one.
	if !strings.HasPrefix(herdrIdleScreen, composerMark) {
		t.Fatal("the idle fixture no longer opens with an echoed prompt — that line is the hazard this arm is about")
	}
	if body, ok := captureComposerBox(herdrIdleScreen); body != "" || !ok {
		t.Errorf("captureComposerBox(idle) = (%q, %v), want (\"\", true) — the box is the EMPTY one between the rules", body, ok)
	}
}

// ─── the refusal ─────────────────────────────────────────────────────────────

// herdrClaudeOnAReportedLabel arms the fake the way the box was armed, and the
// arrangement IS the finding (MEASURED 2026-11-11): `pane report-agent`
// overrides herdr's own label on `agent get` while `agent explain` goes on
// answering with herdr's DETECTION, so posse's two authorities disagree about
// one pane — the route reader sees a label with no manifest and types, and the
// screen behind it is a live claude's.
func herdrClaudeOnAReportedLabel(t *testing.T, fake, screen string) {
	t.Helper()
	reportedPane(t, fake, "fakebob-xdaw9", "idle", "claude", "codex", "grok")
	write(t, filepath.Join(fake, "explain-agent-name"), claudeDetectedLabel)
	write(t, filepath.Join(fake, "pane-text", "w1_p1"), screen)
}

func herdrTypedAnything(log string) bool {
	return strings.Contains(log, "send-text") || strings.Contains(log, "send-keys")
}

func TestQAReportedPromptRefusesAPaneHoldingADialogAndTypesNothing(t *testing.T) {
	t.Parallel()
	// MEASURED, twice: this prompt would have been discarded and its Enter
	// would have pressed `1. Yes` on a shell command nobody reviewed.
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrPermissionDialogScreen)

	_, err := b.H.AgentPrompt("w1:p1", "do the thing", true, 10_000)
	if err == nil {
		t.Fatal("a prompt into a pane holding a permission dialog was accepted")
	}
	if log := calls(t, fake); herdrTypedAnything(log) {
		t.Errorf("the refusal still typed; calls were:\n%s", log)
	}
	// The dialog's own hint, so the operator knows what to go and look at.
	if !strings.Contains(err.Error(), "Esc to cancel · Tab to amend") {
		t.Errorf("the refusal does not quote the screen's last line: %v", err)
	}
	if !strings.Contains(err.Error(), "posse peek") {
		t.Errorf("the refusal does not name the recovery: %v", err)
	}
}

func TestQAReportedPromptRefusalIsNotDeliveredSoAClaimIsHandedBack(t *testing.T) {
	t.Parallel()
	// ADR 0084 D1's allow-list. Nothing was typed, so the claim goes back
	// rather than being kept over a pane that was never asked anything — and
	// a plain untyped error would be read as INDETERMINATE and kept.
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrModelPickerScreen)

	_, err := b.H.AgentPrompt("w1:p1", "do the thing", true, 10_000)
	if err == nil {
		t.Fatal("a prompt into the /model picker was accepted")
	}
	if !isNotDelivered(err) {
		t.Errorf("the refusal = %v, which isNotDelivered does not recognize — dispatch would keep the claim over a prompt it never sent", err)
	}
}

func TestQAReportedPromptRefusesEveryMeasuredDialogIncludingTheLaunchOne(t *testing.T) {
	t.Parallel()
	for name, screen := range map[string]string{
		"permission dialog": herdrPermissionDialogScreen,
		"model picker":      herdrModelPickerScreen,
		"folder trust":      herdrTrustDialogScreen,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			herdrClaudeOnAReportedLabel(t, fake, screen)
			if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err == nil {
				t.Fatalf("a prompt into the %s was accepted", name)
			}
			if log := calls(t, fake); herdrTypedAnything(log) {
				t.Errorf("the refusal still typed; calls were:\n%s", log)
			}
		})
	}
}

func TestQAReportedPromptTypesIntoAPaneWhoseBoxIsOnTheScreen(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrIdleScreen)

	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
		t.Fatalf("a prompt into a pane with a box on screen was refused: %v", err)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "pane send-text w1:p1 do the thing") || !strings.Contains(log, "pane send-keys w1:p1 enter") {
		t.Errorf("the route must still deliver:\n%s", log)
	}
}

func TestQAReportedPromptConcedesAScreenWithNoneOfClaudesChrome(t *testing.T) {
	t.Parallel()
	// Unknown is not no (ADR 0013 §1). A pane mid-boot, or one whose screen
	// this vocabulary was never written for: the reading licenses nothing and
	// refuses nothing, exactly as every other reading in this family concedes.
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, "starting up\n")
	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
		t.Fatalf("a screen posse cannot read was treated as evidence: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "pane send-text w1:p1 do the thing") {
		t.Errorf("the prompt must still go in:\n%s", log)
	}
}

func TestQAReportedPromptConcedesAPaneReadHerdrWouldNotAnswer(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "fakebob-xdaw9", "idle", "claude", "codex", "grok")
	write(t, filepath.Join(fake, "explain-agent-name"), claudeDetectedLabel)
	write(t, filepath.Join(fake, "pane-read-error"), "")
	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
		t.Fatalf("a pane read that failed was read as a pane holding a dialog: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "pane send-text w1:p1 do the thing") {
		t.Errorf("the prompt must still go in:\n%s", log)
	}
}

func TestQAReportedPromptConcedesAPaneHerdrDetectsAsSomethingElse(t *testing.T) {
	t.Parallel()
	// The scope, and the hazard it guards (ADR 0076 Amendment 5): the box
	// this reading locates is claude's idiom, so on a pane held by anything
	// else it would find no box on EVERY screen and refuse every prompt
	// forever. herdr's own detected label is what says whose chrome it is.
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrPermissionDialogScreen)
	write(t, filepath.Join(fake, "explain-agent-name"), "codex")
	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
		t.Fatalf("a pane herdr detects as codex was judged against claude's chrome: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "pane send-text w1:p1 do the thing") {
		t.Errorf("the prompt must still go in:\n%s", log)
	}
}

func TestQAReportedPromptConcedesThePaneThisRouteWasBuiltFor(t *testing.T) {
	t.Parallel()
	// The DESIGNED case of this route — a pane whose RUNTIME herdr has no
	// manifest for, a Bob under a plugin — answers agent_explain_unavailable,
	// so posse's reported arm sets its own fields only and names no agent.
	// This concedes, every time: a real Bob is never judged against claude's
	// chrome, and a herdr that cannot be asked never stops a prompt.
	//
	// It is also the whole reason `agent explain` is asked rather than the
	// screen read on its own. The screen armed here is claude's own
	// permission dialog, which the reading would refuse in a heartbeat if
	// nothing scoped it.
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle")
	write(t, filepath.Join(fake, "pane-text", "w1_p1"), herdrPermissionDialogScreen)
	write(t, filepath.Join(fake, "explain-error"), "agent_explain_unavailable|fake herdr: w1:p1 does not have a detected agent label")

	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
		t.Fatalf("the pane this route exists for was refused: %v", err)
	}
	if log := calls(t, fake); !strings.Contains(log, "pane send-text w1:p1 do the thing") {
		t.Errorf("the prompt must still go in:\n%s", log)
	}
}

func TestQAReportedPromptTakesASecondLookBeforeItRefuses(t *testing.T) {
	t.Parallel()
	// promptComposerRecheck: a screen caught without its box must not read as
	// a dialog on one sample. A dialog is up until a human acts on it, so it
	// is in every sample; a redraw is not. The fake's numbered pane-text
	// levers are what make the two instants separable at all.
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrIdleScreen)
	write(t, filepath.Join(fake, "pane-text", "w1_p1-1"), herdrPermissionDialogScreen)
	write(t, filepath.Join(fake, "pane-text", "w1_p1-2"), herdrIdleScreen)

	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err != nil {
		t.Fatalf("one no-box sample was spent as a refusal: %v", err)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "pane send-text w1:p1 do the thing") {
		t.Errorf("the prompt must still go in:\n%s", log)
	}
	if n := strings.Count(log, "pane read"); n < 2 {
		t.Errorf("%d pane read(s) before the keystrokes — the second look is what makes the refusal honest:\n%s", n, log)
	}
}

func TestQAReportedPromptRefusesTheArgumentBeforeItReadsAnyScreen(t *testing.T) {
	t.Parallel()
	// Order, and it is about cost: the leading-'/' refusal is a property of
	// the ARGUMENT and costs nothing, so it runs before the explain and the
	// pane read. The blocked guard is ahead of both, off a reading the route
	// had already taken.
	for name, tc := range map[string]struct {
		status string
		text   string
	}{
		"a leading '/'":   {"idle", "/clear"},
		"a blocked label": {"blocked", "do the thing"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			herdrClaudeOnAReportedLabel(t, fake, herdrIdleScreen)
			reportedPane(t, fake, "fakebob-xdaw9", tc.status, "claude", "codex", "grok")

			if _, err := b.H.AgentPrompt("w1:p1", tc.text, true, 10_000); err == nil {
				t.Fatalf("%s was accepted", name)
			}
			if log := calls(t, fake); strings.Contains(log, "pane read") {
				t.Errorf("%s cost a screen read:\n%s", name, log)
			}
		})
	}
}

func TestQAReportedPromptReadsTheScreenHerdrsOwnDetectionReads(t *testing.T) {
	t.Parallel()
	// `--source detection`, not the CLI's default `recent`: it is the screen
	// `agent explain --file` replays and the source every fixture in
	// etc/herdr/agent-detection/testdata was captured with (ADR 0066 D3,
	// PaneReadDetection's note). A refusal judged off a different snapshot
	// than herdr's own would be a different measurement.
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrPermissionDialogScreen)
	if _, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0); err == nil {
		t.Fatal("a prompt into a permission dialog was accepted")
	}
	log := calls(t, fake)
	if !strings.Contains(log, "pane read w1:p1 --source detection") {
		t.Errorf("the screen must be read the way herdr reads it:\n%s", log)
	}
}

func TestQAReportedPromptRefusalNamesTheMeasurementAndTheLabel(t *testing.T) {
	t.Parallel()
	// The refusal is the whole surface an operator gets, so it carries what
	// happened and whose pane it is: the label that routed the delivery, the
	// two calls, and what the Enter did when this was measured.
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrTrustDialogScreen)
	_, err := b.H.AgentPrompt("w1:p1", "do the thing", false, 0)
	if err == nil {
		t.Fatal("a prompt into the folder-trust prompt was accepted")
	}
	for _, want := range []string{
		"nothing was sent",
		"fakebob-xdaw9",
		"pane send-text",
		"DISCARDED",
		"Enter to confirm · Esc to cancel",
		"ranger-base-xdaw9",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
}

// A guard on the fixture arrangement itself: if the fake ever stops letting a
// reported label and a detected agent coexist on one pane, every arm above
// would go green over a route that is no longer reachable.
func TestQAHerdrStillReportsADetectedAgentUnderAnOverriddenLabel(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	herdrClaudeOnAReportedLabel(t, fake, herdrIdleScreen)

	ag, reported := b.H.ReportedAgent("w1:p1")
	if !reported || ag.Agent != "fakebob-xdaw9" {
		t.Fatalf("ReportedAgent = (%+v, %v) — the typed route is what the hazard rides on", ag, reported)
	}
	det, err := b.H.AgentExplain("w1:p1")
	if err != nil {
		t.Fatalf("agent explain over the same pane: %v", err)
	}
	if det.Agent != claudeDetectedLabel {
		t.Errorf("agent explain names %q, want %q — the scope evidence the refusal rests on", det.Agent, claudeDetectedLabel)
	}
	if _, err := os.Stat(filepath.Join(fake, "pane-text", "w1_p1")); err != nil {
		t.Errorf("the pane's screen fixture is not where the fake serves it: %v", err)
	}
}
