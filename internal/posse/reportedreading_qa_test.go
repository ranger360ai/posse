//go:build posse_arm3

package posse

// QA pins for ranger-base-nm5i6 — ADR 0061 D3, the ONE detection reading for
// a reported pane and its liveness half.
//
// THE GAP THESE HOLD. ranger-base-8eqaa built delivery to a pane herdr labels
// but does not detect, and ranger-base-rx7l7 let such a runtime be launched at
// all. Between them sat six readings that judge a pane from an
// `AgentDetection` — the settle ladder, the delivered wait, the promptable
// gate, the pulse, the pane-holding read, the probe's detection observable —
// every one of them reached through `Herdr.AgentExplain`, the verb herdr
// REFUSES for a reported pane. So each of the six read an error and fell into
// its own concession, and 8eqaa had to build a second ladder for the one that
// could not live with that.
//
// WHAT WOULD FAIL SILENTLY WITHOUT THESE ARMS:
//
//   - the reading not reaching a consumer at all, which is where this started:
//     a reported pane that reads "herdr cannot explain this, prompting anyway"
//     has spent a whole startup wait to arrive at the weakest answer there is,
//     and nothing in the log says a good one was available.
//   - **the stale label.** A reported label is a pidfile: herdr ties it to the
//     PANE, and only the reporter or the pane's close ever clears it (ADR 0061
//     Claims fact 1 — a pane reported `working` ran `sleep 6`, went back to
//     its shell prompt, and `agent get` still read `working`). A CLI that
//     exits to its shell keeps reading `idle`, the promptable gate opens on
//     it, and the work prompt is typed at a SHELL, which executes every line
//     of it. Nothing prints red: the keystrokes land, the call succeeds, and
//     what runs is whatever `Work beads issue …` means to zsh. That is
//     ranger-base-3p0's incident with a cause posse can see and could not
//     read before this bead.
//   - the guard firing the WRONG way — a herdr whose `pane process-info` went
//     missing, or whose output moved, marking every reported label stale and
//     every reported pane unpromptable. A box that has stopped dispatching is
//     not a box that is being careful, which is why the concession arm is
//     pinned beside the guard.
//   - `unknown` read as evidence. An authority reports it precisely when it
//     has LOST track (ranger-base-mx5x9 #3: it keeps the label and herdr's own
//     `agent wait` will not settle on it), so it is the one state a reporter
//     says to mean "do not act on me".
//   - two readings again. The census arm is what keeps a second discriminator
//     from growing beside the door, and keeps a runtime NAME out of all of it
//     (ADR 0017 §3).
//
// None of these names a runtime, and the labels are deliberately of no
// significance: what they are about is the REPORT, not the CLI behind it. A
// pin that named bob would go green the day the mechanism started keying on a
// name, which is the class ADR 0060 rejected a whole adapter over.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// atShell arms the pane's foreground as its own shell — the stale reading.
// The pids are the measured ones (ADR 0061 Claims fact 2), and the file is the
// same lever a live pane leaves absent.
func atShell(t *testing.T, fake string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fake, "pane-foreground"), []byte("34661|34661"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Arm 1, and the whole of ADR 0061 Verification 4. One reading, four inputs,
// and the four answers the contract names — asked of `AgentExplain` itself,
// because that is the door every consumer comes through and a pin that called
// the reported arm directly would prove nothing about who reaches it.
func TestQAOneReadingAnswersForAReportedPaneAndOnlyWhileItIsLive(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		state      string
		shell      bool
		seen       bool
		wantInLine []string
	}{
		{"a live pane an authority reports idle is SEEN", "idle", false, true, nil},
		{"a live pane reported working is SEEN — the state is the caller's to judge", "working", false, true, nil},
		{"a live pane reported done is SEEN", "done", false, true, nil},
		{"the pane went back to its shell: the label is STALE whatever it says", "idle", true, false,
			[]string{`"plugged"`, "STALE", `"idle"`, "SHELL", "ADR 0061 D3.2"}},
		{"unknown is the authority saying it has lost track", "unknown", false, false,
			[]string{`"plugged"`, `"unknown"`, "ADR 0061 D3.1"}},
		{"no state at all reads as unknown, in the authority's own vocabulary", "", false, false,
			[]string{`"unknown"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			reportedPane(t, fake, "plugged", tc.state)
			if tc.shell {
				atShell(t, fake)
			}

			det, err := b.H.AgentExplain("w1:p1")
			if err != nil {
				t.Fatalf("the door must ANSWER for a reported pane, not hand back herdr's refusal: %v", err)
			}
			if det.Reported != "plugged" || det.State != tc.state {
				t.Fatalf("the reading must carry the label and the state the reporter left: %+v", det)
			}
			if det.ShellForeground != tc.shell {
				t.Errorf("the liveness half read shell-foreground=%v, want %v: %+v", det.ShellForeground, tc.shell, det)
			}
			if det.Seen() != tc.seen {
				t.Errorf("Seen()=%v, want %v — %+v", det.Seen(), tc.seen, det)
			}
			// And there is no screen evidence to have read, on any arm.
			if det.Rule.ID != "" || det.VisibleIdle {
				t.Errorf("a reported reading may carry no screen evidence at all: %+v", det)
			}
			line := det.ReportedNotSeen()
			if tc.seen {
				if line != "" {
					t.Errorf("a reading that IS seen has no not-seen clause to print: %q", line)
				}
				return
			}
			for _, want := range tc.wantInLine {
				if !strings.Contains(line, want) {
					t.Errorf("the failure clause must name %s:\n%s", want, line)
				}
			}
		})
	}
}

// Arm 2, and the reason the guard exists at all: the keystroke. A stale label
// is the one shape where every store says "promptable" and the pane is a
// shell — so the gate must refuse with NOTHING typed, and the line must say
// which of the two causes it is.
func TestQAAStaleReportedLabelRefusesThePromptWithNothingTyped(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	promptReadySession(t, b, "plugged", "400ms")
	reportedPane(t, fake, "plugged", "idle")
	atShell(t, fake)

	_, _, err := b.AwaitPromptable("plugged", "w1:p1")
	if err == nil {
		t.Fatal("a label whose pane is back at its shell is not promptable — a work prompt typed there is run by the shell")
	}
	for _, want := range []string{"nothing was sent", "plugged", "STALE", "idle", "SHELL", "posse peek plugged", "--now"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q:\n%v", want, err)
		}
	}
	// The whole point, and the only assertion here that would have caught
	// ranger-base-3p0: not one keystroke went into that pane.
	log := calls(t, fake)
	for _, never := range []string{"pane send-text", "pane send-keys", "agent prompt"} {
		if strings.Contains(log, never) {
			t.Errorf("the gate refused and %q ran anyway — the text is in a shell:\n%s", never, log)
		}
	}
}

// Arm 3, the concession, and it is pinned beside the guard on purpose. A
// `process-info` that cannot be READ is not evidence that the pane is at a
// shell. Answering "stale" over a failed diagnostic would make a herdr with no
// such verb a box where no reported pane is ever promptable — which is not
// caution, it is an outage with a careful-sounding reason.
func TestQAAProcessReadingThatCannotBeTakenIsNotEvidenceOfAShell(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	reportedPane(t, fake, "plugged", "idle")
	if err := os.WriteFile(filepath.Join(fake, "process-info-error"), []byte("bad_request|no such subcommand"), 0o644); err != nil {
		t.Fatal(err)
	}

	det, err := b.H.AgentExplain("w1:p1")
	if err != nil {
		t.Fatalf("a failed liveness read must not fail the whole reading: %v", err)
	}
	if det.ShellForeground {
		t.Error("a process reading that errored says nothing about the foreground — UNKNOWN is never a yes")
	}
	if !det.Seen() {
		t.Errorf("the label stands on its own when the guard cannot be taken, exactly as it did before the guard existed: %+v", det)
	}
	// And the same over an answer whose SHAPE moved: every foreground pid
	// equal to the shell is the reading, and an empty list is not that —
	// "every" over nothing is vacuously true, which is the one way this guard
	// could fire over a herdr that simply said less.
	if p := (PaneProcesses{ShellPID: 34661}); p.AtShellPrompt() {
		t.Error("a process_info with no foreground processes is UNKNOWN, not a pane at a shell")
	}
	if p := (PaneProcesses{Foreground: []int{34661}}); p.AtShellPrompt() {
		t.Error("a process_info with no shell_pid has nothing to compare against")
	}
	if p := (PaneProcesses{ShellPID: 34661, Foreground: []int{34661, 82630}}); p.AtShellPrompt() {
		t.Error("one non-shell process in the foreground is a CLI holding the tty")
	}
}

// Arm 4 — ADR 0061 Verification 5, through the settle ladder no consumer of
// which was touched by this bead. `done` opens it; `working` keeps it waiting,
// as on every detected runtime; and the refusal it eventually writes says what
// the AUTHORITY said, not that herdr failed to recognize a screen (D3.3).
func TestQATheSettleLadderOpensOnAReportedDoneAndWaitsOnAReportedWorking(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, state string
		opens       bool
	}{
		{"done settles", "done", true},
		{"idle settles", "idle", true},
		{"working is not a settle, on this route either", "working", false},
		{"unknown is not a settle", "unknown", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			reportedPane(t, fake, "plugged", tc.state)
			// `agent wait` answers the reported state too — herdr returns on
			// reported transitions, MEASURED 2026-09-28 — so the ladder's two
			// legs agree, which is what makes the explain leg the one under
			// test here.
			waitStatus := tc.state
			if waitStatus == "working" || waitStatus == "unknown" {
				waitStatus = "idle"
			}
			if err := os.WriteFile(filepath.Join(fake, "wait-status"), []byte(waitStatus), 0o644); err != nil {
				t.Fatal(err)
			}
			var out strings.Builder
			d := NewDispatcher(b.App, b, &out)
			d.Poll = 20 * time.Millisecond
			wait := 200 * time.Millisecond

			// The session is deliberately NOT the label: the failure line has
			// to name the label because it read one, not because the two
			// happen to spell the same, which is a way an assertion like
			// this passes over a line that says nothing.
			status, det, err := d.awaitSettled("qa-nm5i6", "seat7", "w1:p1",
				[]string{"idle", "done", "blocked"}, time.Now().Add(wait), wait)
			if !tc.opens {
				if err == nil {
					t.Fatalf("the ladder settled on a reported %q: %q %+v", tc.state, status, det)
				}
				for _, want := range []string{"never became promptable", `"plugged"`, orUnknown(tc.state)} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("the ladder's failure must name %q — the label and the state are the whole reading on this route:\n%v", want, err)
					}
				}
				for _, never := range []string{"no rule matched", "from rule"} {
					if strings.Contains(err.Error(), never) {
						t.Errorf("herdr evaluated no rules for this pane, so %q is a sentence about a screen nobody read:\n%v", never, err)
					}
				}
				// And the block under it is the reporter's word, not herdr's
				// working — the D3.3 half, reached through the real ladder.
				if !strings.Contains(err.Error(), "No screen evidence on this route") {
					t.Errorf("the failure must carry the reported block, which is what WhatHerdrSaw prints here:\n%v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("a reported %q is a settle: %v\n%s", tc.state, err, out.String())
			}
			if status != tc.state || det.Reported != "plugged" || !det.Seen() {
				t.Errorf("the ladder must open on the reported state and carry the label that stated it: %q %+v", status, det)
			}
		})
	}
}

// And `blocked` is handed back AS GIVEN, stale or not (ADR 0061 D3.3). Two
// things make that right rather than an omission: the refusal for a blocked
// pane lives in delivery, where sendTextPrompt mirrors herdr's own
// `agent_blocked` code so both routes refuse in the same place by the same
// rule — and handing a blocker back by name costs no keystroke, which is the
// only hazard the liveness guard is about.
func TestQAAReportedBlockedIsHandedBackAsGivenEvenWhenTheLabelIsStale(t *testing.T) {
	t.Parallel()
	for _, shell := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "stale"}[shell], func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			reportedPane(t, fake, "plugged", "blocked")
			if err := os.WriteFile(filepath.Join(fake, "wait-status"), []byte("idle"), 0o644); err != nil {
				t.Fatal(err)
			}
			if shell {
				atShell(t, fake)
			}
			var out strings.Builder
			d := NewDispatcher(b.App, b, &out)
			d.Poll = 20 * time.Millisecond
			wait := 200 * time.Millisecond

			status, det, err := d.awaitSettled("qa-nm5i6", "plugged", "w1:p1",
				[]string{"idle", "done", "blocked"}, time.Now().Add(wait), wait)
			if err != nil || status != "blocked" {
				t.Fatalf("a reported blocked is handed back by name for the caller to refuse on: %q %v", status, err)
			}
			if det.Reported != "plugged" || det.State != "blocked" {
				t.Errorf("handed back AS GIVEN means the reading is not rewritten on the way out: %+v", det)
			}
		})
	}
}

// Arm 5 — D3.3's other half. A failure line that would have printed herdr's
// working prints the reporter's word instead, and where the caller had the
// PROFILE in hand it quotes `detection_why:`, which is the only sentence in
// the whole mechanism that names the authority to go ask.
func TestQAAReportedFailureLineQuotesTheDeclarationAndNotHerdrsWorking(t *testing.T) {
	t.Parallel()
	det := AgentDetection{State: "idle", Reported: "plugged", ShellForeground: true}
	why := "herdr-bob plugin MartinLoeper/herdr-bob, installed 2026-09-28"

	with := det.WhatHerdrSaw(why)
	for _, want := range []string{`label "plugged"`, "reported state idle", "SHELL", "STALE", "detection_why: " + why, "herdr plugin list"} {
		if !strings.Contains(with, want) {
			t.Errorf("the block must carry %q:\n%s", want, with)
		}
	}
	// `herdr evaluated N rules` is the detected route's sentence and there is
	// nothing behind it here — herdr read no screen, so printing it would
	// describe a measurement nobody took.
	if strings.Contains(with, "evaluated") {
		t.Errorf("a reported reading has no evaluated rules to report:\n%s", with)
	}
	// And a caller with no profile in hand says less rather than guessing: the
	// declaration is the operator's sentence, and posse may not invent one.
	without := det.WhatHerdrSaw("")
	if strings.Contains(without, "detection_why") {
		t.Errorf("with no runtime in hand there is no declaration to quote:\n%s", without)
	}
	for _, want := range []string{`label "plugged"`, "herdr plugin list"} {
		if !strings.Contains(without, want) {
			t.Errorf("the block must still stand on its own without a declaration (%q):\n%s", want, without)
		}
	}
	// The detected route is untouched: a reading with herdr's working still
	// prints herdr's working, and the declaration argument changes nothing
	// about it.
	screen := AgentDetection{State: "idle", EvaluatedRules: []EvaluatedRule{{ID: "osc_title", Region: "osc_title"}}}
	if got := screen.WhatHerdrSaw(why); !strings.Contains(got, "evaluated 1 rules") || strings.Contains(got, "detection_why") {
		t.Errorf("a detected reading's block is herdr's working and nothing else:\n%s", got)
	}
}

// ─── the absence-rules census ────────────────────────────────────────────────

// reportedReadingFiles are the non-test files ADR 0061 D3's reading is allowed
// to live in: the door, and the arm behind it.
var reportedReadingFiles = []string{"herdr.go", "reportedagent.go"}

// reportedReadingFuncs is the reading itself, by function name — the door, its
// reported arm, the liveness read, and the two predicates every consumer
// judges a pane with. Named rather than derived because the claim is about
// THESE functions: they are the ones that may not name a runtime.
var reportedReadingFuncs = []string{
	"AgentExplain", "reportedDetection", "reportedStateIsWatched",
	"PaneProcessInfo", "paneShellForeground", "AtShellPrompt",
	"Seen", "ReportedNotSeen", "ReportedAgent", "AgentInfo",
}

// The census ADR 0061 D3 rests on, and the arm that makes "one door" a
// property of the tree rather than of this bead's diff.
//
// TWO CLAIMS, both mechanical over the package's own non-test sources (this
// package IS the whole reading, so the scan needs no repo root and is not a
// tree-wide pin):
//
//  1. THE DISCRIMINATOR IS IN ONE FILE. `agent_explain_unavailable` is an
//     error code used as a type — the only way posse can tell "this label was
//     reported" from "herdr would not answer", because `agent get` carries no
//     source field (ranger-base-mx5x9 #4). A second file that read that code
//     would be a second reading, and the two would disagree the moment one of
//     them learned about liveness and the other did not — which is exactly
//     what 8eqaa's second gate was.
//
//  2. THE READING NAMES NO RUNTIME. ADR 0017 §3's shadow predicate is `if
//     rt.Name == "…"` in code, and ADR 0060 rejected a whole report-agent
//     adapter over it. A reading that grew one would work on this box and be
//     wrong about every runtime the next plugin labels. Asserted over the
//     functions above rather than over whole files, because herdr.go has an
//     honest `"claude"` in it (PaneAgentSession answers for claude's session
//     shape and nothing else) and a file-wide rule would either red on that
//     or have to except it.
//
// Comments are invisible to both arms — the scan is an ast walk over string
// LITERALS — which is why the head comments in these files may discuss bob and
// the plugin by name while the code may not.
func TestQAOneDoorForTheReportedReadingAndItNamesNoRuntime(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var withCode []string
	seen := map[string]bool{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if v, err := strconv.Unquote(lit.Value); err == nil && v == reportedExplainRefusal && !seen[name] {
				seen[name], withCode = true, append(withCode, name)
			}
			return true
		})
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || !containsString(reportedReadingFuncs, fn.Name.Name) {
				continue
			}
			if !containsString(reportedReadingFiles, name) {
				t.Errorf("%s holds %s, which is part of the one detection reading — it belongs in %v, or the reading has two homes",
					name, fn.Name.Name, reportedReadingFiles)
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				for _, rt := range builtinRuntimes {
					if v == rt.Name {
						t.Errorf("%s: %s names the runtime %q — the reading is keyed on a property of the PANE, asked of herdr, so a runtime whose detection arrives from any plugin tomorrow is read without a line changing here (ADR 0017 §3, ADR 0061 D3)",
							name, fn.Name.Name, v)
					}
				}
				return true
			})
		}
	}
	sort.Strings(withCode)
	if len(withCode) != 1 || withCode[0] != "reportedagent.go" {
		t.Errorf("the reported discriminator %q must appear in exactly one non-test file (reportedagent.go), not %v — a second copy is a second reading",
			reportedExplainRefusal, withCode)
	}
}
