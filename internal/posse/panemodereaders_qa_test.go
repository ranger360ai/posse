//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-yi2f8 — ADR 0057's simplification: the pane-mode
// DECLARATION REGISTRY is gone; the concrete readers stay and are reached by
// the runtime's own name.
//
// What went, and what the measurement said. `pane_mode:` was a declarable
// registry key over three readers — a runtime yaml naming which one parsed
// its screen — shipped 2026-09-05 (ranger-base-x3hs1) and removed 2026-09-06.
// The bead's first done-when priced it before anything was deleted: the count
// of working adapters supplied through instance declarations that cannot use
// the built-in readers is ZERO. Every runtimes/ directory on the box, both
// repos' whole yaml surface and both repos' git history carry no `pane_mode:`
// declaration at all, and the one instance runtime file is an ADR 0021
// overlay on the built-in claude that was structurally barred from carrying
// the key. So the seam's entire value was a fourth CLI nobody has, and its
// cost was a runtime yaml load per listing plus a level of indirection over a
// fact only Go can establish — which reader parses a screen is fixed at the
// capture corpus in permissionmodepane_qa_test.go, not at a yaml.
//
// What did NOT go, and is what these pins hold:
//
//   - the four measured observations and the fifth state that is their
//     absence. NAMED, COVERED, UNNAMEABLE, NEVER and UNREAD stay five
//     distinguishable things through the surfaces an operator reads;
//   - `none` the ADAPTER is gone, PaneModeNever the STATE is not — codex is
//     still a permanent `—` and still costs no herdr call;
//   - a CLI nobody has measured is still loudly unread with its own name in
//     the sentence, and is never silently folded into `none` or a mode;
//   - the reading decides NOTHING. It is a display observation, which is the
//     whole basis on which ADR 0013 §7 grants the name-keyed reader selection
//     its narrow exception to ADR 0017 §3.
//
// The corpus these run against is permissionmodepane_qa_test.go's verbatim
// captures — the same fixtures the shipped readers are checked on, so a green
// here is the reader the fleet runs, not a second copy of it agreeing with
// itself.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// paneReadCount is how many pane reads THIS test's fake herdr served. Absent
// log = zero, which is the reading the no-read pins need to be able to take.
func paneReadCount(t *testing.T, fake string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fake, "pane-read-log"))
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(b)))
}

// yamlRuntime writes a template-only runtime yaml and returns its name.
// Deliberately not one of the built-ins: it is the CLI posse ships no Go for,
// the one the retired key existed to serve.
func yamlRuntime(t *testing.T, b *HerdrBackend, name, decl string) string {
	t.Helper()
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "command: " + name + " --sys {file}\n" + decl
	if err := os.WriteFile(filepath.Join(b.App.RuntimesDir(), name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

// The bead's second done-when, driven end to end through `posse list`: the
// three concrete observations and the two absences remain five distinct
// things on the screen an operator reads.
//
// A listing, not a call to the reader, because the claim is about what the
// removal preserved for an OPERATOR — a reader that still returns five states
// while the listing collapses two of them would pass a unit assertion and
// lose the distinction the whole field exists for.
func TestQAFiveObservationStatesSurviveTheRegistryRemoval(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, runtime, pane, want, why string
	}{
		{"named-claude", "claude", claudePaneAuto, "mode:auto", ""},
		{"covered-claude", "claude", claudePaneDialog, "mode:?covered", "modal dialog"},
		{"named-grok", "grok", grokPaneAuto, "mode:auto", ""},
		{"unnameable-grok", "grok", grokPaneNoSuffix, "mode:?unnamed", "never \"default\""},
		{"never-codex", "codex", codexPaneNever, "mode:—", "permanent"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			modePersona(t, b, "dev")
			mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "dev", Runtime: c.runtime, Tier: TierStandard})
			paneShowing(t, b, fake, "s1", c.pane)
			if ln := modeLine(t, b, "s1"); !strings.Contains(ln, c.want) {
				t.Errorf("a %s session lists as:\n  %s\nwant %q — the concrete reader is not reached by the runtime's name any more",
					c.runtime, ln, c.want)
			}
			if c.why == "" {
				return
			}
			// The sentence a report prints INSTEAD of a mode. Removing the
			// registry removed where these were written down; the readers
			// still owe them, and a state that stopped saying which unknown
			// it is would pass the token check above.
			var rep strings.Builder
			b.SessionModeReport(&rep, "dev")
			if !strings.Contains(rep.String(), c.why) {
				t.Errorf("`posse gates dev` no longer says WHICH unknown it is showing (want a line containing %q):\n%s", c.why, rep.String())
			}
		})
	}
}

// The fifth state and the control for every arm above: a CLI posse has never
// measured is UNREAD, loudly, with its own name in the sentence — shown a
// claude footer and not read by claude's reader.
//
// Without this arm a green above would not distinguish per-runtime readers
// from a listing that runs every pane through claude's. It also pins the
// distinction the three-valued field turns on: `mode:?` with "nobody has
// measured", NOT `mode:—`. A CLI nobody measured and a CLI measured to render
// nothing must never render the same token — that collapse is exactly the
// "silent unknown-to-none fallback" ADR 0057 forbids as a replacement seam.
func TestQAUnmeasuredRuntimeIsLoudNotNoneAndNotRead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	rt := yamlRuntime(t, b, "mycli", "")
	mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "dev", Runtime: rt, Tier: TierStandard})
	paneShowing(t, b, fake, "s1", claudePaneAuto)
	ln := modeLine(t, b, "s1")
	if !strings.Contains(ln, "mode:?") || strings.Contains(ln, "mode:auto") {
		t.Errorf("an unmeasured runtime showing a claude footer lists as:\n  %s\nwant mode:? — a screen nobody measured is not read by claude's reader", ln)
	}
	if strings.Contains(ln, "mode:—") {
		t.Errorf("an UNMEASURED runtime rendered as a permanent —:\n  %s\nNEVER is a measurement and absence is not; collapsing them is the defect the three-valued field removes", ln)
	}
	m := PaneModeUndeclared(rt)
	if m.State != PaneModeUnread || !strings.Contains(m.Why, "nobody has measured") {
		t.Errorf("PaneModeUndeclared(%q) = %+v; want the unread state with the why that names the runtime", rt, m)
	}
	if !strings.Contains(m.Why, rt) {
		t.Errorf("the why never names the runtime it is about: %q", m.Why)
	}
}

// The bead's fourth done-when — the YAML seam is GONE — asked the only way
// that can fail honestly: the key that used to select a reader is declared,
// and it selects nothing.
//
// Three arms, because "removed" has three separate failure modes and each
// would look fine from the other two:
//
//   - it must not still WORK. A yaml declaring `pane_mode: claude-footer` on
//     a pane showing a claude footer lists `mode:?`, not `mode:auto`. This is
//     the arm that separates the two designs.
//   - it must not REFUSE. The load stays clean: a stale key in an operator's
//     own file is not an outage, and a refusal here would take out every
//     dispatched session reading that file rather than the one line that
//     added it.
//   - it must not be SILENT. The normal unknown-key diagnostic names
//     `pane_mode:` by spelling, which is what the bead asks of an obsolete
//     declaration — the operator hears that the line is dead from posse
//     rather than from a column that quietly stopped being read.
//
// SERIAL, and that is the third arm's price: reading the diagnostic means
// swapping the package-level runtimeNoticeWriter, which a parallel test
// cannot do without stealing another test's notices. TestOverlayWarnsUnknown-
// Keys is serial for the same reason and the same variable.
func TestQAThePaneModeYamlKeyIsRetiredInertAndNamed(t *testing.T) {
	b, fake := newTestBackend(t)
	// Armed BEFORE anything loads the file: the notice is said once per
	// (path, key set), so a listing that loaded the runtime first would eat
	// the only line and leave this pin measuring a dedupe.
	var notice strings.Builder
	old := runtimeNoticeWriter
	runtimeNoticeWriter = &notice
	defer func() { runtimeNoticeWriter = old }()
	modePersona(t, b, "dev")
	rt := yamlRuntime(t, b, "mycli", "pane_mode: claude-footer\n")
	mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "dev", Runtime: rt, Tier: TierStandard})
	paneShowing(t, b, fake, "s1", claudePaneAuto)

	ln := modeLine(t, b, "s1")
	if strings.Contains(ln, "mode:auto") {
		t.Errorf("a yaml declaring pane_mode: still selected a reader:\n  %s\nADR 0057 removes the declaration seam — the key must buy nothing", ln)
	}
	if !strings.Contains(ln, "mode:?") {
		t.Errorf("a runtime whose only pane declaration is the retired key lists as:\n  %s\nwant mode:? — it is unmeasured, which is what it was before the key existed", ln)
	}
	if strings.Contains(ln, "mode:—") {
		t.Errorf("the retired key fell back to NEVER:\n  %s\nADR 0057 forbids a silent unknown-to-none fallback by name", ln)
	}

	if _, err := b.App.LoadRuntime(rt); err != nil {
		t.Fatalf("a stale pane_mode: refused the whole runtime: %v\nA retired key is inert — refusing an instance's launch profile over a dead line is a bigger outage than the one prevented", err)
	}
	if got := notice.String(); !strings.Contains(got, "pane_mode:") {
		t.Errorf("the unknown-key diagnostic does not name the obsolete declaration:\n%s\nwant a line naming pane_mode: — a dropped declaration never arrives, and this is the only place that says so", got)
	}
}

// The same retirement one file over: `pane_mode:` in a BUILT-IN's overlay used
// to refuse the load as an ADR 0021 D2 mechanism key. With no key to be a
// mechanism, that refusal is gone — and the arm that matters is that its
// disappearance moved nothing: a stale overlay line does not become an
// overlay that works. claude's reader is claude's reader.
func TestQAPaneModeInABuiltinOverlayIsInertToo(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	// An OVERLAY, so no command: — that key really is mechanism and refuses
	// on its own, which would make this pin measure the wrong wall.
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.App.RuntimesDir(), "claude.yaml"), []byte("pane_mode: none\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.App.LoadRuntime("claude"); err != nil {
		t.Fatalf("a stale pane_mode: in a built-in overlay refused the load: %v\nThe key is retired, not forbidden — every dispatched claude session reads this file", err)
	}
	mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "dev", Runtime: "claude", Tier: TierStandard})
	paneShowing(t, b, fake, "s1", claudePaneAuto)
	if ln := modeLine(t, b, "s1"); !strings.Contains(ln, "mode:auto") {
		t.Errorf("an overlay carrying the retired key moved claude's reader:\n  %s\nwant mode:auto — a yaml may not turn a read column into a permanent — with no measurement behind it", ln)
	}
}

// The cost half of the observation, which is what `none` was a declaration
// FOR and now needs holding without one: a listing spends a herdr pane read
// only where reading a screen can say something. Codex's answer is a measured
// constant and an unmeasured CLI has no parser, so neither costs a call.
//
// The control arm is the same rig on claude, which MUST read: without it a
// zero here would be a fake herdr nobody asked anything, not a listing that
// skipped the call (NOTES, "a rig must be shown able to fail"). Two sessions
// per arm, because the claim is per session and one of each would not tell a
// skipped read from an unfilled row.
func TestQAOnlyAMeasuredScreenCostsAPaneRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, runtime, want string
		reads               bool
	}{
		{"codex-never", "codex", "mode:—", false},
		{"unmeasured", "", "mode:?", false},
		{"claude-footer", "claude", "mode:auto", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			modePersona(t, b, "dev")
			rt := c.runtime
			if rt == "" {
				rt = yamlRuntime(t, b, "mycli", "")
			}
			for _, s := range []string{"s1", "s2"} {
				mustCreate(t, b, NewSessionOpts{Name: s, Agent: "dev", Runtime: rt, Tier: TierStandard})
				paneShowing(t, b, fake, s, claudePaneAuto)
			}
			for _, s := range []string{"s1", "s2"} {
				if ln := modeLine(t, b, s); !strings.Contains(ln, c.want) {
					t.Errorf("%s: session %s lists as:\n  %s\nwant %q", c.name, s, ln, c.want)
				}
			}
			// modeLine runs `posse list` once per call, so reads are per
			// listing × per session. What is pinned is the ZERO, and that the
			// control is not also zero.
			if got := paneReadCount(t, fake); c.reads != (got > 0) {
				t.Errorf("%s: the fake herdr served %d pane read(s), want %s — a measured constant costs no call, and a real reader must actually read",
					c.name, got, map[bool]string{false: "none", true: "at least one"}[c.reads])
			}
		})
	}
}

// Every built-in has a MEASURED reader. The registry made this checkable as a
// declaration; without one it is a property of paneReaderFor, and the symptom
// of losing it is codex-shaped — every session on a runtime posse measured in
// August listing `mode:?` forever, with nothing refusing at load to say so.
//
// The second half is the one that could actually be wrong. Every reader's
// not-named answer is asserted to be what its own documented sentence says:
// the three Why constants are read back OUT of the readers rather than
// compared to a copy. A doc describing an absence the reader does not report
// is the same defect one layer down from a registry row that did.
func TestQAEveryBuiltinScreenIsMeasuredAndSaysWhatItReturns(t *testing.T) {
	t.Parallel()
	for _, rt := range builtinRuntimes {
		r := paneReaderFor(rt.Name)
		if !r.known {
			t.Errorf("built-in %s has no pane reader — posse measured all three built-ins on 2026-08-29 (permissionmodepane_qa_test.go), so an unmeasured one is a measurement thrown away, not an unknown", rt.Name)
			continue
		}
		// A screen with nothing on it: every reader's not-named answer.
		m := ReadPaneMode(rt.Name, "")
		if m.State == PaneModeNamed {
			t.Errorf("%s: an empty pane named %q — a reader that guesses a mode off no screen is the reading this field exists to replace", rt.Name, m.Mode)
			continue
		}
		if strings.TrimSpace(m.Why) == "" {
			t.Errorf("%s: a not-named reading with no sentence — `posse gates` prints the why, and \"can't tell\" with no reason is where this started", rt.Name)
		}
		if r.read == nil {
			// The cost claim, and it is checkable: a runtime that spends no
			// pane read must answer the same thing whatever it is handed, or
			// the skipped call would lose a reading.
			if PaneModeReadsPane(rt.Name) {
				t.Errorf("%s: no reader func but PaneModeReadsPane says a listing should spend a call on it", rt.Name)
			}
			if got := ReadPaneMode(rt.Name, claudePaneAuto); got != m {
				t.Errorf("%s: answers %+v with a pane and %+v without one — the skipped read was not free", rt.Name, got, m)
			}
		}
	}
	// The three sentences are constants for one reason: they are the same
	// string in the reader and in the prose above it. Asserted by execution,
	// against the corpus, so a reworded constant that stopped reaching a
	// reader reds here.
	for _, c := range []struct {
		what, runtime, pane, want string
	}{
		{"covered", "claude", claudePaneDialog, paneModeCoveredWhy},
		{"unnameable", "grok", grokPaneNoSuffix, paneModeUnnameableWhy},
		{"never", "codex", codexPaneNever, paneModeNeverWhy},
	} {
		if got := ReadPaneMode(c.runtime, c.pane); got.Why != c.want {
			t.Errorf("%s: reader returns %q, the constant says %q", c.what, got.Why, c.want)
		}
	}
}

// ADR 0035 §4's other half, which lost its pin when the registry-derived
// checklist row went: what the surfaces SAY. A named mode is a default
// disposition, and neither surface may render it as a promise about blocking
// — a claude session in auto mode was measured stopping on its own classifier.
//
// Asked of both shipped surfaces at once, on a session that IS in the most
// approving mode claude names, because that is the row where a reassuring
// word would do the damage.
func TestQANeitherSurfacePromisesAModeCannotBlock(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "dev", Runtime: "claude", Tier: TierStandard})
	paneShowing(t, b, fake, "s1", claudePaneBypass)
	var rep strings.Builder
	b.SessionModeReport(&rep, "dev")
	for _, surface := range []struct{ what, text string }{
		{"posse list", modeLine(t, b, "s1")},
		{"posse gates", rep.String()},
	} {
		if !strings.Contains(surface.text, "mode:bypassPermissions") {
			t.Fatalf("%s does not carry the mode at all:\n%s", surface.what, surface.text)
		}
		for _, bad := range []string{"cannot block", "will not block", "non-blocking", "nonblocking", "never asks", "no confirmation", "unattended"} {
			if strings.Contains(strings.ToLower(surface.text), bad) {
				t.Errorf("%s renders the mode as a promise about blocking (%q):\n%s\nADR 0035 §4 — the mode names a default disposition and nothing else", surface.what, bad, surface.text)
			}
		}
	}
}
