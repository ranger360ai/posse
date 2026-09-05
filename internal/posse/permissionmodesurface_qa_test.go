package posse

// The SURFACE half of ranger-base-vwgt: what `posse list` and `posse gates
// <persona>` say about the permission mode a session is actually in, driven
// by the committed corpus in permissionmodepane_qa_test.go rather than by
// strings retyped here. That file pins the READER against verbatim pane
// captures; this one pins that the reading reaches an operator, and that the
// three ways it can fail to reach one are three different things on screen.
//
// Why the states matter more than the happy path (ADR 0035 §3, and the bead's
// own done-when): the field exists FOR the session nobody could classify. A
// blank column reads as "fine", so every state renders a token — and the
// three unknowns are not interchangeable:
//
//	mode:?covered  claude, dialog over the footer — clears on the next read
//	mode:?unnamed  grok, one of the four modes its border cannot name
//	mode:—         codex, which renders no policy on any screen, ever
//
// The last one is the reason `—` is not `?`: no amount of further work closes
// it, and an operator who reads codex's column as "pending" would go looking.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// paneShowing points the fake herdr at what THIS session's pane is rendering.
// It reads the pane id back out of the meta, which is the same join the
// listing makes.
func paneShowing(t *testing.T, b *HerdrBackend, fake, session, text string) {
	t.Helper()
	m, ok := b.readMeta(session)
	if !ok || m.Pane == "" {
		t.Fatalf("session %s has no pane recorded", session)
	}
	dir := filepath.Join(fake, "pane-text")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, strings.ReplaceAll(m.Pane, ":", "_")), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// modeLine returns the listing row for one session.
func modeLine(t *testing.T, b *HerdrBackend, session string) string {
	t.Helper()
	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	for _, ln := range strings.Split(list.String(), "\n") {
		if strings.Contains(ln, " "+session+" ") {
			return ln
		}
	}
	t.Fatalf("session %s not listed:\n%s", session, list.String())
	return ""
}

func modePersona(t *testing.T, b *HerdrBackend, name string) {
	t.Helper()
	if err := os.MkdirAll(b.App.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: d\n---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(b.App.AgentsDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// One pin per runtime, driven by the fixtures — the bead's fourth done-when.
// Each case is a whole `posse list` run, so what is pinned is what an
// operator reads, not what the reader returns.
func TestQAListNamesWhatEachRuntimesPaneShows(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, runtime, pane, want string
	}{
		// claude: all six, and the three spelled without the word "mode"
		// are the three that approve the most.
		{"claude-auto", "claude", claudePaneAuto, "mode:auto"},
		{"claude-manual", "claude", claudePaneManual, "mode:manual"},
		{"claude-plan", "claude", claudePanePlan, "mode:plan"},
		{"claude-accept-edits", "claude", claudePaneAcceptEdits, "mode:acceptEdits"},
		{"claude-bypass", "claude", claudePaneBypass, "mode:bypassPermissions"},
		{"claude-dont-ask", "claude", claudePaneDontAsk, "mode:dontAsk"},
		// claude blind: the dialog took the footer. The launch command is
		// in that pane's own scrollback and must not become the answer.
		{"claude-dialog", "claude", claudePaneDialog, "mode:?covered"},
		// grok: two of six, and the four it cannot name are their own state.
		{"grok-auto", "grok", grokPaneAuto, "mode:auto"},
		{"grok-always-approve", "grok", grokPaneBypass, "mode:always-approve"},
		{"grok-no-suffix", "grok", grokPaneNoSuffix, "mode:?unnamed"},
		{"grok-splash", "grok", grokPaneSplashAuto, "mode:?unnamed"},
		// codex: nothing on any screen, so nothing an operator should wait for.
		{"codex-never", "codex", codexPaneNever, "mode:—"},
		{"codex-on-request", "codex", codexPaneOnRequest, "mode:—"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			modePersona(t, b, "dev")
			mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "dev", Runtime: c.runtime, Tier: TierStandard})
			paneShowing(t, b, fake, "s1", c.pane)
			if ln := modeLine(t, b, "s1"); !strings.Contains(ln, c.want) {
				t.Errorf("posse list row for a %s pane showing %s:\n  %s\nwant %q", c.runtime, c.name, ln, c.want)
			}
		})
	}
}

// The provenance rule, stated as the thing that could go wrong: the session
// was LAUNCHED one way and its pane says another, and the listing reports the
// pane. Nothing in the meta, the launch template or the pane's own scrollback
// may reach this column — that is the entire reason the column is not just
// the flag posse typed.
func TestQAListModeIsThePaneNotTheLaunch(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	mustCreate(t, b, NewSessionOpts{Name: "drifted", Agent: "dev", Runtime: "claude", Tier: TierStandard})
	// posse types --permission-mode auto (ClaudeFleetFlags); this pane is in
	// plan, which is where a hand-relaunch or a drifted template lands one.
	if !strings.Contains(ClaudeFleetFlags, "auto") {
		t.Fatal("the fleet no longer types auto — re-derive this case's drift")
	}
	paneShowing(t, b, fake, "drifted", claudePanePlan)
	if ln := modeLine(t, b, "drifted"); !strings.Contains(ln, "mode:plan") || strings.Contains(ln, "mode:auto") {
		t.Errorf("the listing reported the launch, not the pane:\n  %s", ln)
	}

	// And the trap in the other direction: the dialog pane's SCROLLBACK
	// carries `--permission-mode manual`, which is a claim about the launch.
	// A reader that widened its window past the live screen tail would
	// report it, and the row would name a mode nothing on screen supports.
	mustCreate(t, b, NewSessionOpts{Name: "covered", Agent: "dev", Runtime: "claude", Tier: TierStandard})
	paneShowing(t, b, fake, "covered", claudePaneDialog)
	if ln := modeLine(t, b, "covered"); !strings.Contains(ln, "mode:?covered") || strings.Contains(ln, "mode:manual") {
		t.Errorf("a dialog-covered pane's scrollback argv reached the listing:\n  %s", ln)
	}
}

// No path types into a pane to obtain the mode (the bead's third done-when),
// and codex costs no read at all: what it renders was measured once and does
// not change per session.
func TestQAListReadsPanesAndTypesIntoNone(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	mustCreate(t, b, NewSessionOpts{Name: "cl", Agent: "dev", Runtime: "claude", Tier: TierStandard})
	mustCreate(t, b, NewSessionOpts{Name: "cx", Agent: "dev", Runtime: "codex", Tier: TierStandard})
	paneShowing(t, b, fake, "cl", claudePaneAuto)
	clm, _ := b.readMeta("cl")
	cxm, _ := b.readMeta("cx")
	if err := os.Remove(filepath.Join(fake, "calls.log")); err != nil {
		t.Fatal(err)
	}
	var list strings.Builder
	if err := b.CmdList(&list); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(filepath.Join(fake, "calls.log"))
	if err != nil {
		t.Fatal(err)
	}
	calls := string(log)
	// The positive control: without it, every absence below is vacuous.
	if !strings.Contains(calls, "pane read "+clm.Pane) {
		t.Errorf("the listing never read the claude pane:\n%s", calls)
	}
	for _, typed := range []string{"pane run", "agent prompt", "pane send", "pane keys"} {
		if strings.Contains(calls, typed) {
			t.Errorf("the listing TYPED into a pane (%q) to learn a mode:\n%s", typed, calls)
		}
	}
	if strings.Contains(calls, "pane read "+cxm.Pane) {
		t.Errorf("a codex pane was read; codex renders no policy on any screen, so the read buys a constant:\n%s", calls)
	}
	if !strings.Contains(list.String(), "mode:—") {
		t.Errorf("the codex row lost its column:\n%s", list.String())
	}
}

// The window, pinned. The reader considers only the last three non-empty
// lines, and the corpus file records that no MEASURED capture forces that
// bound: a relaunch in the same pane leaves one footer on screen, and the
// launch command line in the scrollback does not contain a footer spelling.
// What the bound is actually a guard against is text that QUOTES a footer
// above the live one — a transcript, a paste, this very bead being worked in
// a pane — and that case is COMPOSED here rather than measured: two verbatim
// captures from the corpus, an older one quoted in the scrollback and the
// live screen underneath. Nothing is invented but their order.
//
// A reader that scanned the whole pane returns the quoted mode, in line
// order, and reports bypassPermissions for a session sitting in manual.
func TestQAPaneModeReadsOnlyTheLiveScreenTail(t *testing.T) {
	t.Parallel()
	quoted := "  the footer read \"" + strings.TrimSpace(claudePaneBypass[strings.LastIndex(claudePaneBypass, "\n")+1:]) + "\"\n" +
		claudePaneManual
	if !strings.Contains(quoted, "bypass permissions on") || !strings.Contains(quoted, "manual mode on") {
		t.Fatal("the composed fixture no longer carries both footers — recompose it from the corpus")
	}
	m := ReadPaneMode(builtinPaneModeAdapter("claude"), quoted)
	if m.State != PaneModeNamed || m.Mode != "manual" {
		t.Errorf("ReadPaneMode = %+v; want the LIVE footer (manual). A quoted footer above the tail is not a reading", m)
	}
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	mustCreate(t, b, NewSessionOpts{Name: "q1", Agent: "dev", Runtime: "claude", Tier: TierStandard})
	paneShowing(t, b, fake, "q1", quoted)
	if ln := modeLine(t, b, "q1"); !strings.Contains(ln, "mode:manual") {
		t.Errorf("the listing read past the live screen tail:\n  %s", ln)
	}
}

// A read that FAILS is its own state too, and it is not a mode and not a
// blank. The listing still lists the session — a pane posse cannot read is
// not a session that is gone.
func TestQAListSaysSoWhenThePaneCannotBeRead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	mustCreate(t, b, NewSessionOpts{Name: "cl", Agent: "dev", Runtime: "claude", Tier: TierStandard})
	if err := os.WriteFile(filepath.Join(fake, "pane-read-error"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ln := modeLine(t, b, "cl")
	if !strings.Contains(ln, "mode:?") || strings.Contains(ln, "mode:?covered") {
		t.Errorf("a failed pane read must render as its own unknown, not as a covered footer and not as a mode:\n  %s", ln)
	}
}

// The three unknowns and the unread state are four distinct tokens, none of
// them empty and none of them a mode name. This is the "renders the unknown
// states as their own thing" done-when, asked of the vocabulary itself: a
// later tidy-up that collapsed `—` into `?` would pass every case above and
// lose the one distinction the bead's title names.
func TestQAPaneModeUnknownsAreFourDistinctTokens(t *testing.T) {
	t.Parallel()
	seen := map[string]string{}
	for _, c := range []struct {
		what string
		m    PaneMode
	}{
		{"named", ReadPaneMode(builtinPaneModeAdapter("claude"), claudePaneAuto)},
		{"covered", ReadPaneMode(builtinPaneModeAdapter("claude"), claudePaneDialog)},
		{"unnameable", ReadPaneMode(builtinPaneModeAdapter("grok"), grokPaneNoSuffix)},
		{"never", ReadPaneMode(builtinPaneModeAdapter("codex"), codexPaneNever)},
		{"unread", PaneMode{}},
	} {
		tag := c.m.Tag()
		if strings.TrimSpace(tag) == "" {
			t.Fatalf("%s renders as a blank, which is the defect this field replaces", c.what)
		}
		if prev, dup := seen[tag]; dup {
			t.Errorf("%s and %s both render %q — an operator cannot tell them apart", c.what, prev, tag)
		}
		seen[tag] = c.what
		// Every state but the named one owes a sentence: `posse gates`
		// prints it, and "can't tell" with no why is where this started.
		if c.what != "named" && !strings.Contains(c.m.Line(), " — ") {
			t.Errorf("%s renders as a bare token in a report that has room for the reason: %q", c.what, c.m.Line())
		}
	}
	// The unread state is the zero value, so it is what a caller that never
	// asked for a read gets — and it must not look like an answer.
	if (PaneMode{}).State != PaneModeUnread || (PaneMode{}).Mode != "" {
		t.Error("the zero value is no longer 'nothing was read'")
	}
}

// `posse gates <persona>` carries per-SESSION pane state — the half the QA
// evidence on this bead found missing entirely, since gates takes a persona
// and a mode is a fact about a running pane.
func TestQAGatesReportsThePaneModePerSession(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	modePersona(t, b, "other")
	mustCreate(t, b, NewSessionOpts{Name: "g1", Agent: "dev", Runtime: "grok", Tier: TierStandard})
	mustCreate(t, b, NewSessionOpts{Name: "g2", Agent: "dev", Runtime: "grok", Tier: TierStandard})
	mustCreate(t, b, NewSessionOpts{Name: "x1", Agent: "other", Runtime: "claude", Tier: TierStandard})
	paneShowing(t, b, fake, "g1", grokPaneBypass)
	paneShowing(t, b, fake, "g2", grokPaneNoSuffix)
	paneShowing(t, b, fake, "x1", claudePaneAuto)

	var rep strings.Builder
	b.SessionModeReport(&rep, "dev")
	got := rep.String()
	if !strings.Contains(got, "g1") || !strings.Contains(got, "mode:always-approve") {
		t.Errorf("the report lost the session that IS in the approving mode:\n%s", got)
	}
	// ADR 0035 §3's actual subject: the border word is what the flag AND the
	// operator's ~/.grok/config.toml both render, so the report says the
	// layer is unknown rather than crediting the launch.
	if !strings.Contains(got, "config.toml") {
		t.Errorf("the report claims to know which layer set always-approve:\n%s", got)
	}
	if !strings.Contains(got, "g2") || !strings.Contains(got, "mode:?unnamed") {
		t.Errorf("the report lost the session grok cannot classify:\n%s", got)
	}
	if strings.Contains(got, "x1") {
		t.Errorf("another persona's session is in this persona's report:\n%s", got)
	}

	// A persona with nothing running is told that, not shown a blank.
	var none strings.Builder
	b.SessionModeReport(&none, "nobody")
	if !strings.Contains(none.String(), "no live nobody session") {
		t.Errorf("a persona with no session got no sentence:\n%s", none.String())
	}
}

// The documentation half, coupled (the QA note on ranger-base-vwgt).
//
// ADR 0035 §3 declines to give grok a second mode layer and names its
// compensating control: "the actual pane mode is read from the composer
// border and surfaced in `rhq list`/gates". While that sentence stands, a
// NON-TEST symbol has to be able to name a mode a grok pane renders, and the
// listing has to carry it. Written this way the pin is green when the feature
// is built and green if the ADR is instead reworded to say it is not — it
// never asserts the current state of either side, only that they agree.
func TestQAADR0035PaneModeSurfaceClaimIsBuilt(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)
	adr, err := os.ReadFile(filepath.Join(root, "docs", "adr", "0035-mode-second-layer-typed-line-only.md"))
	if err != nil {
		t.Fatalf("ADR 0035 is not where the citation says: %v", err)
	}
	// The claim, matched on its load-bearing clause rather than the whole
	// sentence: a reflow must not silently retire the pin.
	claim := strings.Contains(strings.Join(strings.Fields(string(adr)), " "),
		"the actual pane mode is read from the composer border and surfaced in")
	if !claim {
		return // reworded: nothing to couple
	}
	// Built, asked of shipped code: the reader is in permissionmode.go, not
	// in this package's test corpus.
	if m := ReadPaneMode(builtinPaneModeAdapter("grok"), grokPaneAuto); m.State != PaneModeNamed || m.Mode != "auto" {
		t.Fatalf("ADR 0035 §3 says the composer border is read and surfaced; ReadPaneMode(grok) returns %+v.\n"+
			"Either build it or reword the ADR — the clause is the fleet's only control on a flag-lost grok session.", m)
	}
	// Surfaced, asked of the listing the ADR names by command.
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	mustCreate(t, b, NewSessionOpts{Name: "g1", Agent: "dev", Runtime: "grok", Tier: TierStandard})
	paneShowing(t, b, fake, "g1", grokPaneAuto)
	if ln := modeLine(t, b, "g1"); !strings.Contains(ln, "mode:auto") {
		t.Fatalf("ADR 0035 §3 says the pane mode is surfaced in the listing; the row reads:\n  %s", ln)
	}
	var rep strings.Builder
	b.SessionModeReport(&rep, "dev")
	if !strings.Contains(rep.String(), "mode:auto") {
		t.Fatalf("ADR 0035 §3 names gates too; its report reads:\n%s", rep.String())
	}
}
