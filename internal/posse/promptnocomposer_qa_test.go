//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-hlnyx: a prompt is REFUSED into a claude block whose screen has
// no prompt box on it (ADR 0076 Amendment 7, muxrex.go's
// refuseWithoutComposer).
//
// EVERY SCREEN IN THIS FILE IS A MEASURED ONE, taken off a live claude 2.1.322
// block on this host through `rex capture` on 2026-11-10 and quoted here from
// ranger-base-8eeoy's own session log. docs/notes.d/ranger-base-hlnyx.md has
// the provenance line for each. They are quoted rather than invented because
// the reading under test is entirely about what claude draws: an invented
// screen would pin posse against posse.
//
// TWO NORMALIZATIONS, neither of them a glyph. The rules are written at 80
// columns rather than the capturing terminal's width, because isFooterRule
// asks what a line is MADE of and not how long it is; and the scratch path
// in the permission dialog's "always allow" row is replaced, because a crew
// worktree path under internal/ is what `make crew-check` exists to find.

import (
	"strings"
	"testing"
)

// ─── the measured screens ────────────────────────────────────────────────────

// rexScreenRule is claude's horizontal rule at this box's width — U+2500 and
// nothing else, which is what isFooterRule requires.
var rexScreenRule = strings.Repeat("─", 80)

// rexIdleScreen is a settled seat: the transcript, then the rule / box / rule
// / blank / footer tail (MEASURED, 8eeoy's post-G2 capture). The U+00A0 after
// the mark is claude's own.
var rexIdleScreen = "● ok\n\n  Ran 1 shell command\n\n✻ Cooked for 2s · done 10:49 PM\n\n" +
	rexScreenRule + "\n❯ \n" + rexScreenRule + "\n\n  ⏸ manual mode on"

// rexQueuedScreen is a seat mid-turn with the prompt taken into claude's
// queue: the box is there and holds claude's own faint placeholder
// (MEASURED, 8eeoy's CC-midturn series). It must be typed into — that is the
// mid-turn row of that bead's table, which was accepted and answered.
var rexQueuedScreen = "● working\n\n" + rexScreenRule +
	"\n❯ Press up to edit queued messages\n" + rexScreenRule +
	"\n\n  ⏵⏵ auto mode on · 1 shell\n  esc to interrupt"

// rexPermissionDialogScreen is the one that cost the measurement two
// unreviewed `touch`es (MEASURED, 8eeoy row G2/G4). Note what it has and has
// not: ONE U+2500 rule, the dialog's own separators drawn with U+254C, the
// selection cursor drawn with the composer's own mark, and `Esc to cancel` in
// the last line.
var rexPermissionDialogScreen = "  ⎿  $ touch probe-perm-test.txt\n\n" + rexScreenRule + "\n" +
	" Bash command\n" +
	" Tip: auto mode handles these prompts for you — choose \"switch to auto mode\" below\n" +
	" Create empty file probe-perm-test.txt\n" +
	strings.Repeat("╌", 80) + "\n" +
	" touch probe-perm-test.txt\n" +
	strings.Repeat("╌", 80) + "\n" +
	" Do you want to proceed?\n" +
	" ❯ 1. Yes\n" +
	"   2. Yes, and always allow access to /tmp/scratch/probe from this project\n" +
	"   3. Yes, and switch to auto mode · auto mode handles these prompts for you\n" +
	"   4. No\n\n" +
	" Esc to cancel · Tab to amend"

// rexModelPickerScreen is `/model` (MEASURED, 8eeoy row F1 — the Enter landed
// on `Enter to set as default` and wrote a model into the operator's own
// settings.json). It draws NO U+2500 rule at all, which is why the chrome
// reading is not rules alone.
var rexModelPickerScreen = "   ↑ 3. Fable                  Fable 5.1 · Most capable for your hardest and\n" +
	"                               longest-running tasks\n" +
	"     4. Sonnet                 Sonnet 5.5 · Efficient for routine tasks\n" +
	"   ❯ 5. Haiku ✔                Haiku 5.5 · Fastest for quick answers\n\n\n" +
	"   ◐ Medium effort (default) ←/→ to adjust\n\n" +
	"   Enter to set as default · s to use this session only · Esc to cancel"

// rexTrustDialogScreen is claude's folder-trust prompt (MEASURED 2026-11-10,
// capturedcomposer.go's note). It is the one a seat can be holding at LAUNCH,
// and its highlighted row is `No, exit`.
var rexTrustDialogScreen = "❯ No, exit\n  Yes, I trust this folder\n\nEnter to confirm · Esc to cancel"

// ─── the reading ─────────────────────────────────────────────────────────────

func TestQACaptureComposerBoxTellsAnEmptyBoxFromNoBox(t *testing.T) {
	t.Parallel()
	// The distinction the refusal rests on, and the one captureComposer
	// flattens: both answer "", and only one of them is a seat waiting for a
	// prompt.
	if body, ok := captureComposerBox(rexIdleScreen); body != "" || !ok {
		t.Errorf("captureComposerBox(idle) = (%q, %v), want (\"\", true) — an empty box IS a box", body, ok)
	}
	if body, ok := captureComposerBox(rexPermissionDialogScreen); body != "" || ok {
		t.Errorf("captureComposerBox(permission dialog) = (%q, %v), want (\"\", false)", body, ok)
	}
	// And captureComposer is unchanged by the split: every caller it has
	// reads "" for both, exactly as before.
	if got := captureComposer(rexIdleScreen); got != "" {
		t.Errorf("captureComposer(idle) = %q, want \"\"", got)
	}
}

func TestQACaptureComposerSightJudgesEveryMeasuredScreen(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		screen string
		want   composerSight
	}{
		{"settled idle seat", rexIdleScreen, composerOpen},
		{"mid-turn, prompt queued", rexQueuedScreen, composerOpen},
		{"Bash permission dialog", rexPermissionDialogScreen, composerCovered},
		{"the /model picker", rexModelPickerScreen, composerCovered},
		{"the folder-trust prompt", rexTrustDialogScreen, composerCovered},
		{"nothing of claude's on it", "waiting for the server\n", composerUnreadable},
		{"rex answered nothing", "", composerUnreadable},
	} {
		if got := captureComposerSight(c.screen); got != c.want {
			t.Errorf("captureComposerSight(%s) = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestQACaptureComposerSightDoesNotReadASelectionRowAsABox(t *testing.T) {
	t.Parallel()
	// Both dialogs draw their highlighted row with the composer's own mark —
	// `❯ 1. Yes`, `❯ 5. Haiku ✔`, `❯ No, exit` — so the mark alone can never
	// be what says "there is a box here". This is composerMark's own warning
	// (panework.go) read on the capture route.
	for _, c := range []struct {
		name   string
		screen string
	}{
		{"permission dialog", rexPermissionDialogScreen},
		{"model picker", rexModelPickerScreen},
		{"folder trust", rexTrustDialogScreen},
	} {
		if !strings.Contains(c.screen, composerMark) {
			t.Fatalf("the %s fixture no longer draws the composer mark — it is the whole hazard this arm is about", c.name)
		}
		if got := captureComposerSight(c.screen); got == composerOpen {
			t.Errorf("captureComposerSight(%s) = composerOpen — a dialog's selection row read as a box posse may type into", c.name)
		}
	}
}

func TestQACaptureLastLineIsTheDialogsOwnHint(t *testing.T) {
	t.Parallel()
	// What the refusal quotes, so that an operator is told WHICH dialog is up.
	for screen, want := range map[string]string{
		rexPermissionDialogScreen: "Esc to cancel · Tab to amend",
		rexModelPickerScreen:      "Enter to set as default · s to use this session only · Esc to cancel",
		rexTrustDialogScreen:      "Enter to confirm · Esc to cancel",
	} {
		if got := captureLastLine(screen); got != want {
			t.Errorf("captureLastLine = %q, want %q", got, want)
		}
	}
}

// ─── the refusal ─────────────────────────────────────────────────────────────

// rexSeatShowing is a settled claude seat whose captures answer `screens` in
// order, the last repeating.
func rexSeatShowing(t *testing.T, screens ...string) RexMux {
	t.Helper()
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	rexSeedCaptureQueue(t, rexTestBlock, screens...)
	return m
}

func rexTypedAnything(t *testing.T) bool {
	t.Helper()
	for _, c := range rexCalls(t) {
		if strings.Contains(c, "send") {
			return true
		}
	}
	return false
}

func TestQARexPromptRefusesASeatHoldingADialogAndTypesNothing(t *testing.T) {
	t.Parallel()
	// MEASURED 2026-11-10, three for three: this prompt would have been
	// discarded and its Enter would have pressed `1. Yes`.
	m := rexSeatShowing(t, rexPermissionDialogScreen)
	_, err := m.Prompt(rexTestSeat(), "do the thing", true, 10_000)
	if err == nil {
		t.Fatal("a prompt into a seat holding a permission dialog was accepted")
	}
	if rexTypedAnything(t) {
		t.Errorf("the refusal still typed; calls were:\n%s", strings.Join(rexCalls(t), "\n"))
	}
	// The dialog's own hint, so the operator knows what to go and look at.
	if !strings.Contains(err.Error(), "Esc to cancel · Tab to amend") {
		t.Errorf("the refusal does not quote the screen's last line: %v", err)
	}
	if !strings.Contains(err.Error(), "posse peek") {
		t.Errorf("the refusal does not name the recovery: %v", err)
	}
}

func TestQARexPromptRefusalIsNotDeliveredSoAClaimIsHandedBack(t *testing.T) {
	t.Parallel()
	// ADR 0084 D1's allow-list. Nothing was typed, so the claim must go back
	// rather than be kept over a seat that was never asked anything — and a
	// plain untyped error would be read as INDETERMINATE and kept.
	m := rexSeatShowing(t, rexModelPickerScreen)
	_, err := m.Prompt(rexTestSeat(), "do the thing", true, 10_000)
	if err == nil {
		t.Fatal("a prompt into the /model picker was accepted")
	}
	if !isNotDelivered(err) {
		t.Errorf("Prompt's refusal = %v, which isNotDelivered does not recognize — dispatch would keep the claim over a prompt it never sent", err)
	}
}

func TestQARexPromptTypesIntoASeatWhoseBoxIsOnTheScreen(t *testing.T) {
	t.Parallel()
	for name, screen := range map[string]string{
		"settled idle seat":       rexIdleScreen,
		"mid-turn, prompt queued": rexQueuedScreen,
	} {
		t.Run(name, func(t *testing.T) {
			m := rexSeatShowing(t, screen)
			if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
				t.Fatalf("a prompt into a seat with a box on screen was refused: %v", err)
			}
			rexCallWith(t, "send", "do the thing")
			rexCallWith(t, "send-key", "enter")
		})
	}
}

func TestQARexPromptConcedesAScreenWithNoneOfClaudesChrome(t *testing.T) {
	t.Parallel()
	// A block mid-boot, or a capture rex answered with nothing. Unknown is
	// not no (ADR 0013 §1): this reading licenses nothing and refuses
	// nothing, exactly as every other reading in this family concedes.
	m := rexSeatShowing(t, "starting up\n")
	if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
		t.Fatalf("a screen posse cannot read was treated as evidence: %v", err)
	}
	rexCallWith(t, "send", "do the thing")
}

func TestQARexPromptConcedesACaptureRexWouldNotAnswer(t *testing.T) {
	t.Parallel()
	m := rexSeatWithRuntime(t, []RexStatusRecord{rexRuntimeRec("idle")})
	rexSeedFail(t, "capture", "block target not found")
	if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
		t.Fatalf("a capture that failed was read as a seat holding a dialog: %v", err)
	}
	rexCallWith(t, "send", "do the thing")
}

func TestQARexPromptConcedesABlockHeldBySomethingOtherThanClaude(t *testing.T) {
	t.Parallel()
	// The composer this reads for is claude's idiom, so on any other runtime
	// it would find no box on EVERY screen and refuse every prompt forever.
	// The owner is the scope (refuseWithoutComposer's note).
	m, _ := newTestRexMux(t)
	rexSeedSessions(t, fakeRexSession{SessionID: rexTestSession, Label: "box:dev",
		Blocks: []fakeRexBlock{{BlockID: rexTestBlock, Label: "shell", Flavor: RexTerminalFlavor}}})
	rexSeedProcess(t, rexTestBlock, RexProcess{
		Child:      &RexProc{PID: 10, Name: "zsh"},
		Foreground: &RexProc{PID: 20, Name: "codex"},
		Ancestors:  []RexProc{{PID: 10, Name: "zsh"}},
	})
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: "codex", UpdatedAt: "2026-11-10T04:12:42-04:00",
		Owner: &RexOwner{PID: 20, Name: "codex"}}})
	rexSeedCaptureQueue(t, rexTestBlock, rexPermissionDialogScreen)
	if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
		t.Fatalf("a block held by codex was judged against claude's chrome: %v", err)
	}
	rexCallWith(t, "send", "do the thing")
}

func TestQARexPromptTakesASecondLookBeforeItRefuses(t *testing.T) {
	t.Parallel()
	// promptComposerRecheck: a screen caught without its box must not read as
	// a dialog on one sample. A dialog is up until a human acts on it, so it
	// is in every sample; a redraw is not.
	m := rexSeatShowing(t, rexPermissionDialogScreen, rexIdleScreen)
	if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err != nil {
		t.Fatalf("one no-box sample was spent as a refusal: %v", err)
	}
	rexCallWith(t, "send", "do the thing")
	captures := 0
	for _, c := range rexCalls(t) {
		if strings.Contains(c, "capture") {
			captures++
		}
	}
	if captures < 2 {
		t.Errorf("%d capture(s) before the keystrokes — the second look is what makes the refusal honest", captures)
	}
}

func TestQARexPromptRefusesEveryMeasuredDialogIncludingTheLaunchOne(t *testing.T) {
	t.Parallel()
	// The trust prompt is the one a seat can be holding at LAUNCH, with
	// `No, exit` highlighted — so posse's first prompt into a fresh seat is
	// the Enter that quits it.
	for name, screen := range map[string]string{
		"permission dialog": rexPermissionDialogScreen,
		"model picker":      rexModelPickerScreen,
		"folder trust":      rexTrustDialogScreen,
	} {
		t.Run(name, func(t *testing.T) {
			m := rexSeatShowing(t, screen)
			if _, err := m.Prompt(rexTestSeat(), "do the thing", false, 0); err == nil {
				t.Fatalf("a prompt into the %s was accepted", name)
			}
			if rexTypedAnything(t) {
				t.Errorf("the refusal still typed; calls were:\n%s", strings.Join(rexCalls(t), "\n"))
			}
		})
	}
}

func TestQARexPromptRefusesTheArgumentBeforeItReadsAnyScreen(t *testing.T) {
	t.Parallel()
	// Order, and it is about cost: the leading-'/' refusal is a property of
	// the argument and costs nothing, so it runs before the capture and
	// before the two record reads (Prompt's own note).
	m := rexSeatShowing(t, rexIdleScreen)
	if _, err := m.Prompt(rexTestSeat(), "/compact", true, 10_000); err == nil {
		t.Fatal("a '/'-leading prompt was accepted")
	}
	if n := len(rexCalls(t)); n != 0 {
		t.Errorf("the '/' refusal cost %d rex call(s): %s", n, strings.Join(rexCalls(t), "\n"))
	}
}
