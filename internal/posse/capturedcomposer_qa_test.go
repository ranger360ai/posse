//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-qzujz: the composer half of htafy's hold on an
// occupant with no `agent explain`.
//
// WHY THESE ARMS. ranger-base-r5131 realized the WORK half off a capture and
// refused this half, because a Typed reading with no echo check behind it
// INVENTS holds, and an invented hold keeps a genuinely stuck bead claimed
// forever. So every arm here is about one of the three ways this reading can
// claim a box is holding something it is not:
//
//  1. THE ANCHOR. The composer is located between the screen's last two
//     rules, never searched for. A scan for `❯` finds this file's own prose
//     quoting one, and a hold read off a transcript line can never go false
//     because nobody can clear a line of transcript — arm 2 of
//     capturedhold_qa_test.go, pointed at the other region.
//  2. THE WRAP. A capture is whole and wrapped; herdr's preview is truncated
//     and unwrapped. The same submitted line therefore reads as three screen
//     lines here, and an echo comparison that did not rejoin them would miss
//     every echo longer than the terminal is wide — which is a hold invented
//     for every long prompt the operator has just sent.
//  3. THE DIM RUN. claude writes its own suggestion into an empty box, in no
//     store at all, and the only thing that tells it from typed text is that
//     it is drawn faint (ghostbox.go). `rex capture --format vt` carries that
//     run — and spells a redrawn line's gaps as cursor moves, which the dim
//     reading drops, so the words run together and the join fails. That
//     failure is safe in the wrong direction: the box reads as a hold nobody
//     can clear, which is ranger-base-6o7wm's defect back on this occupant.
//
// The fixtures are real captures, trimmed: `rex capture --format text` and
// `--format vt` of a live claude 2.1.322 block on this host, MEASURED
// 2026-11-10 in a scratch Rex session created and killed for the arm, across
// an empty box, claude's own suggestion, a typed line and a typed line long
// enough to wrap.
//
// The mechanism and its measurement are in capturedcomposer.go.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// writeSubmitRows appends to a claude submit log in the shape the live one
// has (2.1.322: display, pastedContents, project, sessionId, timestamp —
// MEASURED 2026-11-10 against this box's own file). Rows are oldest first.
//
// It writes the same file ghostcomposer_qa_test.go's armSubmitted writes, and
// does not call it: that file is arm 2's and these arms are arm 1's, where
// capturedhold_qa_test.go's are. Both spellings are a claude row and neither
// is the rule — the rule is in sentline.go, which parses `display` and
// `sessionId` and nothing else.
func writeSubmitRows(t *testing.T, path, session string, rows ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for i, r := range rows {
		line, err := json.Marshal(map[string]any{
			"display":        r,
			"pastedContents": map[string]any{},
			"project":        "/w",
			"sessionId":      session,
			"timestamp":      1794360000000 + int64(i)*1000,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			t.Fatal(err)
		}
	}
}

// claudeComposerScreen is the measured tail shape: transcript, a rule, the
// composer lines, a rule, a blank, then the footer. The composer is the
// caller's to supply, because it is the only part these arms vary.
func claudeComposerScreen(composer []string, footer string) string {
	rule := strings.Repeat(string(footerRuleRune), 78)
	out := []string{"  I'll hold here until the arms report.", "", rule}
	out = append(out, composer...)
	return strings.Join(append(out, rule, "", footer), "\n")
}

// idleFooter is a seat with nothing running: the composer half has to answer
// for one of these, because a settled seat holding an unsent prompt is the
// whole of the pulse's G2 `-unsent` row.
const idleFooter = "  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents"

// mark is the composer line's opening: the `❯` and the U+00A0 claude draws
// after it (MEASURED 2026-11-10 — the bytes are E2 9D AF C2 A0).
const mark = composerMark + " "

// ─── the region: located between the last two rules ──────────────────────────

func TestQAACaptureComposerIsBetweenTheLastTwoRules(t *testing.T) {
	t.Parallel()

	rule := strings.Repeat(string(footerRuleRune), 40)
	for _, c := range []struct {
		name, screen, want string
	}{
		{
			name:   "a typed line",
			screen: claudeComposerScreen([]string{mark + "this prompt is sitting unsent in the box"}, idleFooter),
			want:   "this prompt is sitting unsent in the box",
		},
		{
			name:   "an empty box",
			screen: claudeComposerScreen([]string{mark}, idleFooter),
			want:   "",
		},
		{
			// MEASURED verbatim: three screen lines for one prompt, broken
			// at spaces the submitted text has, continuations indented two.
			name: "a wrapped line is rejoined",
			screen: claudeComposerScreen([]string{
				mark + "this prompt is sitting unsent in the box and now it gets long enough to wrap",
				"  across more than one screen line because the terminal is eighty columns",
				"  wide and this sentence is not ",
			}, idleFooter),
			want: "this prompt is sitting unsent in the box and now it gets long enough to wrap across more than one screen line because the terminal is eighty columns wide and this sentence is not",
		},
		{
			// THE LAST TWO RULES, told apart from the FIRST two
			// (ranger-base-1tt2e, from ranger-base-s3a9g). Every other case
			// here plants exactly two rules, so the property in this arm's
			// own name was untested: MEASURED, `if isFooterRule(l) &&
			// topRule < 0 {` — a first-two reading — left this arm and the
			// whole package green.
			//
			// NOT THEORETICAL. isFooterRule accepts any line that is nothing
			// but U+2500, which is what claude renders its own markdown
			// horizontal rules with, so one rule in the TRANSCRIPT makes
			// three — and a first-two reading then calls transcript the
			// composer, which is the invented hold capturedcomposer.go's own
			// head comment refuses.
			name: "three rules: the composer is between the LAST two",
			screen: "  as the transcript renders `---`:\n" + rule +
				"\n  and prose under it\n" + rule + "\n" +
				mark + "this prompt is sitting unsent in the box\n" + rule +
				"\n\n" + idleFooter,
			want: "this prompt is sitting unsent in the box",
		},
		{
			// THE ANCHOR. A `❯` in the transcript is somebody quoting one.
			name:   "a mark in the transcript is not the composer",
			screen: "  the composer line is " + mark + "and whatever is typed\n" + rule + "\n" + mark + "\n" + rule + "\n\n" + idleFooter,
			want:   "",
		},
		{"no rule at all is no composer", "just lines\nand more", ""},
		{"one rule is no composer", "a\n" + rule + "\n" + mark + "typed", ""},
		{"two adjacent rules are no composer", "a\n" + rule + "\n" + rule + "\n" + idleFooter, ""},
		{
			name:   "a body that does not open with the mark is not a composer",
			screen: claudeComposerScreen([]string{"  Enter a number between 1 and 5"}, idleFooter),
			want:   "",
		},
	} {
		if got := captureComposer(c.screen); got != c.want {
			t.Errorf("%s: captureComposer = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestQAAMenuChromeAnywhereInTheTailIsNotATypedPrompt(t *testing.T) {
	t.Parallel()

	// herdr hands over a REGION, so its five `not` clauses are matched
	// against exactly the bytes the box held. Here the box is a POSITION on a
	// screen and a dialog draws its hint as its own line below the selection
	// — MEASURED 2026-11-10 on claude's folder-trust prompt: `❯ No, exit`,
	// `  Yes, I trust this folder`, then `Enter to confirm · Esc to cancel`.
	// Which side of a rule that hint lands on was not measured, so both are
	// arms here: read body-only, the second one calls `❯ No, exit` a prompt
	// the operator is holding, and govern files a `-unsent` row for a seat
	// that is blocked on a human.
	for _, c := range []struct{ name, screen string }{
		{
			name: "the hint is inside the box",
			screen: claudeComposerScreen([]string{
				mark + "No, exit", "   Yes, I trust this folder", "   Enter to select · Esc to cancel",
			}, idleFooter),
		},
		{
			name: "the hint is below the rule, where a body-only check cannot see it",
			screen: claudeComposerScreen([]string{
				mark + "No, exit", "   Yes, I trust this folder",
			}, "  Enter to select · Esc to cancel"),
		},
	} {
		if got := captureComposer(c.screen); got != "" {
			t.Errorf("%s: captureComposer = %q, want \"\" — a menu drawn over the box is not text anybody typed", c.name, got)
		}
	}
	// The twin, one phrase different: the same shape with claude's own footer
	// still reads as a prompt, so the arm above is not passing by refusing
	// every screen.
	live := claudeComposerScreen([]string{mark + "No, exit"}, idleFooter)
	if got := captureComposer(live); got != "No, exit" {
		t.Errorf("captureComposer = %q over a footer carrying no menu chrome, want the text", got)
	}
}

// ─── the wrap: the echo comparison is whitespace-insensitive ─────────────────

func TestQAAWrappedBoxEchoesTheLineThatWasSubmitted(t *testing.T) {
	t.Parallel()

	const sent = "this prompt is sitting unsent in the box and now it gets long enough to wrap across more than one screen line"
	typed := captureComposer(claudeComposerScreen([]string{
		mark + "this prompt is sitting unsent in the box and now it gets long enough to wrap",
		"  across more than one screen line",
	}, idleFooter))
	if !capturedEcho(typed, sent) {
		t.Fatalf("capturedEcho(%q, %q) = false — the screen's line breaks are the renderer's, and a comparison that cannot see past them invents a hold for every submitted prompt wider than the terminal", typed, sent)
	}
	// The twin: a DIFFERENT line is still not an echo. Without this the arm
	// above would pass over a comparison that had stopped comparing.
	if capturedEcho(typed, sent+" and one more clause") {
		t.Error("capturedEcho = true for a submit the box is not showing — the collapse must not become a prefix match")
	}
	// And the prefix arm stays shut: a capture is not a truncated preview, so
	// a short box must not match every long submit that starts with it.
	if capturedEcho("this prompt", sent) {
		t.Error("capturedEcho = true for a box holding a prefix of the submit — nothing cut this reading down, so submittedEcho's truncation arm must stay shut")
	}
	// THE COLLAPSE ITSELF (ranger-base-1tt2e, from ranger-base-s3a9g).
	// captureComposer already rejoins the wrapped lines with SINGLE spaces,
	// and no fixture's submitted side carried a whitespace run, so nothing
	// above reaches collapseScreenWrap: MEASURED, `return s` in place of
	// `strings.Join(strings.Fields(s), " ")` left the whole package green.
	// A submit row carries what was TYPED — double spaces, tabs and all —
	// so this is the case where the two sides differ only in runs, and the
	// collapse is the one thing that makes them one line.
	const runsSent = "commit  the  bead \t and  close it"
	runsBox := captureComposer(claudeComposerScreen([]string{
		mark + "commit  the  bead",
		"  and  close it",
	}, idleFooter))
	if !capturedEcho(runsBox, runsSent) {
		t.Errorf("capturedEcho(%q, %q) = false — the comparison collapses whitespace runs on BOTH sides, which is the one place this route differs from herdr's (capturedcomposer.go); without it a submitted line reads as unsent for every run of whitespace in it", runsBox, runsSent)
	}
	if capturedEcho(runsBox, "commit  the  beads \t and  close it") {
		t.Error("capturedEcho = true for a submit the box is not showing — the collapse folds whitespace and nothing else")
	}
}

// ─── the dim run: a vt capture's cursor moves are its spaces ─────────────────

func TestQAACursorForwardIsExpandedBeforeTheDimReading(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ name, in, want string }{
		// MEASURED verbatim off `rex capture --format vt` of a redrawn line.
		{"one column", "this\x1b[1Cprompt", "this prompt"},
		{"several columns", "a\x1b[3Cb", "a   b"},
		{"an omitted parameter is one column", "a\x1b[Cb", "a b"},
		{"another CSI final byte is left alone", "a\x1b[2Kb", "a\x1b[2Kb"},
		{"an SGR run is left alone", "\x1b[0m\x1b[2mdim\x1b[0m", "\x1b[0m\x1b[2mdim\x1b[0m"},
		{"a truncated sequence is passed through", "a\x1b[", "a\x1b["},
		{"an absurd column count is not expanded", "a\x1b[999999Cb", "a\x1b[999999Cb"},
	} {
		if got := expandCursorForward(c.in); got != c.want {
			t.Errorf("%s: expandCursorForward = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestQAAADimBoxOnTheCaptureRouteIsClaudesOwnSuggestion(t *testing.T) {
	t.Parallel()

	// The measured pair, one field different: claude's suggestion comes back
	// under SGR 2 and a typed line comes back with no SGR at all (MEASURED
	// 2026-11-10 on this host — and that second half is the arm
	// ranger-base-6o7wm's corpus could not stage on herdr, where a typed
	// line was measured not-dim zero times).
	const suggestion = `Try "create a util logging.py that..."`
	const typed = "this prompt is sitting unsent in the box"
	for _, c := range []struct {
		name, text, vtLine string
		wantGhost          bool
	}{
		{
			name:      "claude's suggestion, drawn faint",
			text:      suggestion,
			vtLine:    mark + "\x1b[0m\x1b[2m" + suggestion + "\x1b[0m",
			wantGhost: true,
		},
		{
			name:   "a typed line, drawn plain",
			text:   typed,
			vtLine: mark + typed,
		},
		{
			// The same typed line as the renderer actually spelled it on a
			// redraw. Without expandCursorForward the dim reading sees
			// `thisprompt…`, the join fails, and the box stays a hold — which
			// is right here and wrong for the suggestion row below.
			name:   "a typed line whose gaps are cursor moves",
			text:   typed,
			vtLine: mark + strings.ReplaceAll(typed, " ", "\x1b[1C"),
		},
		{
			name:      "a suggestion whose gaps are cursor moves is still a suggestion",
			text:      suggestion,
			vtLine:    mark + "\x1b[0m\x1b[2m" + strings.ReplaceAll(suggestion, " ", "\x1b[1C") + "\x1b[0m",
			wantGhost: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			b.Slot = rexSeatWithRuntime(t)
			rexSeedCapture(t, rexTestBlock, claudeComposerScreen([]string{mark + c.text}, idleFooter))
			rexSeedCaptureAs(t, rexTestBlock, "vt", claudeComposerScreen([]string{c.vtLine}, idleFooter))

			hold := b.PaneHolding(rexTestSeat())
			if c.wantGhost {
				if hold.Ghost != c.text || hold.Typed != "" {
					t.Fatalf("Ghost = %q, Typed = %q, want the text as a GHOST — a suggestion nobody typed is a row nobody can clear", hold.Ghost, hold.Typed)
				}
				if hold.Waiting() {
					t.Error("Waiting() = true over claude's own suggestion")
				}
				return
			}
			if hold.Typed != c.text || hold.Ghost != "" {
				t.Fatalf("Typed = %q, Ghost = %q, want the text as a HOLD", hold.Typed, hold.Ghost)
			}
			if !hold.Waiting() {
				t.Error("Waiting() = false over a prompt sitting unsent — this is the --resume skip and the G2 row")
			}
		})
	}
}

// ─── end to end, through PaneHolding ─────────────────────────────────────────

func TestQAACaptureRouteRetiresABoxEchoingItsOwnLastSubmit(t *testing.T) {
	t.Parallel()

	start := ownProcStart(t)
	pid := os.Getpid()
	const sent = "keep waiting, then commit and close"

	b, _ := newTestBackend(t)
	b.Slot = rexSeedOwnedBy(t, &RexOwner{PID: pid, Name: claudeRuntimeProcName}, claudeProcTable(pid))
	history := armClaudeSessionRow(t, b, pid, map[string]any{
		"pid": pid, "sessionId": qzujzSession, "cwd": "/w", "procStart": start,
	})
	writeSubmitRows(t, history, qzujzSession, "how we doing?", sent)
	rexSeedCapture(t, rexTestBlock, claudeComposerScreen([]string{mark + sent}, idleFooter))

	hold := b.PaneHolding(rexTestSeat())
	if hold.Sent != sent || hold.Typed != "" {
		t.Fatalf("Sent = %q, Typed = %q, want the line retired as an echo — this is the whole reason ranger-base-r5131 refused to read a composer on this route", hold.Sent, hold.Typed)
	}
	if hold.Waiting() {
		t.Errorf("Waiting() = true over a box echoing its own last submit (%q) — the pulse skipped ~586 times on exactly this, ranger-base-wr624", hold.Why())
	}
	// ADR 0066 D1: what the verdict was taken from.
	if !holdNamesRegion(hold, captureComposerRegion, sent) {
		t.Errorf("no %s region carrying the box in the record: %+v", captureComposerRegion, hold.Read.Regions)
	}
	if holdNamesRegion(hold, composerRegion, "") {
		t.Errorf("the capture route recorded herdr's own region name %q — a census that cannot tell the routes apart cannot say which one withheld a settle", composerRegion)
	}
}

func TestQAACaptureRouteKeepsAHoldThatNoStoreClaims(t *testing.T) {
	t.Parallel()

	start := ownProcStart(t)
	pid := os.Getpid()
	const typed = "ping me when it's green"

	b, _ := newTestBackend(t)
	b.Slot = rexSeedOwnedBy(t, &RexOwner{PID: pid, Name: claudeRuntimeProcName}, claudeProcTable(pid))
	history := armClaudeSessionRow(t, b, pid, map[string]any{
		"pid": pid, "sessionId": qzujzSession, "procStart": start,
	})
	// A submit log that answers, and does not answer THIS line. That is the
	// arm that has to still act: both live boxes on the fleet at
	// 2026-09-05T04:2xZ carried text that is in no row, and both stay holds.
	writeSubmitRows(t, history, qzujzSession, "how did the day go?")
	rexSeedCapture(t, rexTestBlock, claudeComposerScreen([]string{mark + typed}, idleFooter))

	hold := b.PaneHolding(rexTestSeat())
	if hold.Typed != typed || hold.Sent != "" {
		t.Fatalf("Typed = %q, Sent = %q, want the text held", hold.Typed, hold.Sent)
	}
	if !strings.Contains(hold.Why(), "UNSENT") {
		t.Errorf("Why() = %q, want it to name the unsent prompt — it is what the skip line and the G2 row both quote", hold.Why())
	}
}

func TestQAAEveryFailureOfTheJoinLeavesTheComposerAHold(t *testing.T) {
	t.Parallel()

	// The concession the whole family makes, in the direction that matters
	// here: a join that cannot be made is not evidence that a prompt was
	// sent. Each row breaks one link and the text must stay a hold.
	const typed = "ping me when it's green"
	for _, c := range []struct {
		name  string
		arm   func(t *testing.T, b *HerdrBackend, pid int)
		owner *RexOwner
	}{
		{
			name:  "no session row for the owner's pid",
			arm:   func(t *testing.T, b *HerdrBackend, pid int) { armClaudeSessionRow(t, b, pid, nil) },
			owner: &RexOwner{PID: os.Getpid(), Name: claudeRuntimeProcName},
		},
		{
			name: "a session row the fence rejects",
			arm: func(t *testing.T, b *HerdrBackend, pid int) {
				armClaudeSessionRow(t, b, pid, map[string]any{"pid": pid, "sessionId": qzujzSession, "procStart": "0"})
			},
			owner: &RexOwner{PID: os.Getpid(), Name: claudeRuntimeProcName},
		},
		{
			name:  "no store at all, which is every test backend",
			arm:   func(t *testing.T, b *HerdrBackend, pid int) {},
			owner: &RexOwner{PID: os.Getpid(), Name: claudeRuntimeProcName},
		},
		{
			name:  "a record the block's own shell wrote",
			arm:   func(t *testing.T, b *HerdrBackend, pid int) { armClaudeSessionRow(t, b, pid, nil) },
			owner: &RexOwner{PID: 10, Name: "zsh"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			pid := os.Getpid()
			b, _ := newTestBackend(t)
			b.Slot = rexSeedOwnedBy(t, c.owner, claudeProcTable(pid))
			c.arm(t, b, pid)
			rexSeedCapture(t, rexTestBlock, claudeComposerScreen([]string{mark + typed}, idleFooter))

			hold := b.PaneHolding(rexTestSeat())
			if hold.Typed != typed || hold.Sent != "" {
				t.Fatalf("Typed = %q, Sent = %q, want the text held — ignorance is not evidence that a prompt was sent", hold.Typed, hold.Sent)
			}
		})
	}
}

func TestQAAWorkAndComposerAreReadOffOneCapture(t *testing.T) {
	t.Parallel()

	// The two halves are independent facts about one screen, and both have to
	// survive: r5131's arm reads a seat with live work and an empty box, and
	// this is the other diagonal — live work AND an unsent prompt, which is
	// the pair Why() joins with "and".
	b, _ := newTestBackend(t)
	b.Slot = rexSeatWithRuntime(t)
	rexSeedCapture(t, rexTestBlock, claudeComposerScreen(
		[]string{mark + "ping me when it's green"},
		"  ⏵⏵ auto mode on · 1 shell, 1 monitor · esc to interrupt · ← for agents"))

	hold := b.PaneHolding(rexTestSeat())
	if hold.Work != "1 shell, 1 monitor" || hold.Typed != "ping me when it's green" {
		t.Fatalf("Work = %q, Typed = %q, want both off the one screen", hold.Work, hold.Typed)
	}
	if got := hold.Why(); !strings.Contains(got, "still running") || !strings.Contains(got, "UNSENT") {
		t.Errorf("Why() = %q, want both halves", got)
	}
	// ONE text capture for both. The vt read is the ghost reading's and is
	// asked only because the box has text; a census of the calls is how the
	// grain stays what the note claims.
	var text int
	for _, call := range rexCalls(t) {
		if strings.Contains(call, "capture") && strings.Contains(call, "--format text") {
			text++
		}
	}
	if text != 1 {
		t.Errorf("%d `capture --format text` calls, want 1 — the work and the composer are one instant and one snapshot", text)
	}
}

// holdNamesRegion reports whether the record carries a region of that name
// whose text holds `want` ("" = any text).
func holdNamesRegion(hold PaneHold, name, want string) bool {
	for _, r := range hold.Read.Regions {
		if r.Name == name && (want == "" || strings.Contains(r.Text, want)) {
			return true
		}
	}
	return false
}
