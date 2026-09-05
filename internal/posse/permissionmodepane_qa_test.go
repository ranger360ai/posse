package posse

// What a pane actually SAYS about its permission mode, per runtime
// (ranger-base-0emp). `posse list` and `posse gates <persona>` report this
// mode because of it, and the ask behind the gap they used to have is "read
// the pane, not the argv":
// a session relaunched by hand, or launched by a drifted template, carries
// whatever mode it ended up in, and its command line is a claim about the
// launch rather than a fact about the process.
//
// This file is the corpus that measurement produced. Every fixture below is
// verbatim `herdr pane read <pane> --format text` output from 2026-08-29,
// captured on claude 2.1.251, codex-cli 0.150.1 and grok 1.0.5 against a
// SCRATCH herdr server (the livesplash_test.go recipe: `herdr --session
// qa0emp server`, one workspace per arm, no prompt ever submitted, so no
// model turn and no spend). The reader these captures support SHIPPED with
// ranger-base-vwgt — permissionmode.go's ReadPaneMode, behind
// HerdrSession.PermissionMode — and paneMode below is now a two-value handle
// on it, so this corpus and its mutations bear on the code the fleet runs
// rather than on a copy of it.
//
// The measured contract, three runtimes, and it is not the same contract:
//
//   - claude 2.1.251 renders the mode in the footer, all six modes, and the
//     footer wording is NOT the flag wording for three of them: acceptEdits
//     reads "accept edits on", bypassPermissions "bypass permissions on",
//     dontAsk "don't ask on" — no "mode" in the line at all. A reader that
//     matches /(\w+) mode on/ sees auto, manual and plan and goes blind on
//     the three that carry the most privilege. Match the mode NAMES.
//   - claude goes blind while a modal dialog is up: the dialog REPLACES the
//     footer, so a pane sitting on an approval prompt — the exact state that
//     motivated this bead — proves nothing about its own mode. Measured on a
//     trust dialog here, and on the live fleet the same morning: a session in
//     auto mode, blocked on "Auto mode classifier requires confirmation for
//     this command", had no mode line anywhere on screen.
//   - codex 0.150.1 renders NOTHING. The footer is `<model> <effort> · <cwd>`
//     under `-a never` and under `-a on-request` alike; the only difference
//     between the two captures is the echoed shell command in the scrollback,
//     i.e. the argv this bead exists to stop trusting. The mode is reachable
//     in-pane only by typing `/status` ("Permissions: Workspace (never)" vs
//     "Workspace (Ask for approval)"), and typing into a session posse did
//     not start a turn on is not a read.
//   - grok 1.0.5 renders a suffix on the composer border — `Grok 4.6 (high) ·
//     auto` — but only for two of the six modes. auto shows "auto";
//     bypassPermissions shows "always-approve" (the same string the
//     operator's ~/.grok/config.toml `[ui] permission_mode = "always-approve"`
//     produces with no flag at all, so the pane cannot say which layer won);
//     default, acceptEdits, dontAsk and plan render NO suffix, and neither
//     does a pane still sitting on the startup splash. Absence is four modes
//     and an unknown, never "default".
//
// So the honest field is three-valued per runtime: a mode, "no mode on
// screen", and — on grok — "not one of the two the pane can name".

import (
	"os"
	"strings"
	"testing"
)

// ─── the corpus: verbatim pane tails, one arm each ──────────────────────────

// claude 2.1.251, one fresh pane per arm, `env -u CLAUDE_CODE_*` so no
// nested-session marker rode in. The bare arm is the CONTROL and it matters:
// this box's claude defaults to auto with no flag and no settings, which is
// why the --settings arm below names `plan` — a value the ambient default
// cannot produce — instead of asserting auto and proving nothing.
const (
	claudePaneAuto = "❯\n" +
		"─────────────────────────────────────────────────\n" +
		"  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents"

	claudePaneManual = "─────────────────────────────────────────────────\n" +
		"  ⚠ Transcript saving is off — inherited CLAUDE_CODE_CHILD_SESSION marker · restart with C…\n" +
		"  ⏸ manual mode on · ? for shortcuts"

	claudePanePlan = "❯\n" +
		"─────────────────────────────────────────────────\n" +
		"  ⏸ plan mode on (shift+tab to cycle) · ← for agents"

	claudePaneAcceptEdits = "❯\n" +
		"─────────────────────────────────────────────────\n" +
		"  ⏵⏵ accept edits on (shift+tab to cycle) · ← for agents"

	claudePaneBypass = "❯\n" +
		"─────────────────────────────────────────────────\n" +
		"  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← for agents"

	claudePaneDontAsk = "❯\n" +
		"─────────────────────────────────────────────────\n" +
		"  ⏵⏵ don't ask on (shift+tab to cycle) · ← for agents"

	// The blind case. This pane WAS launched `--permission-mode manual`
	// (the command is right there in the scrollback, which is the trap);
	// the dialog has taken the footer, so the pane proves nothing.
	claudePaneDialog = "➜  work claude --permission-mode manual --settings '{\"permissions\":{\"defaultMode\":\"auto\"}}'\n" +
		"─────────────────────────────────────────────────\n" +
		" Accessing workspace:\n" +
		"\n" +
		" Quick safety check: Is this a project you created or one you trust?\n" +
		"\n" +
		" ❯ No, exit\n" +
		"   Yes, I trust this folder\n" +
		"\n" +
		" Enter to confirm · Esc to cancel"
)

// codex-cli 0.150.1. Both arms, tails matched: the TUI chrome is identical
// and the argv is the only thing that differs, up in the shell scrollback.
const (
	codexPaneNever = "➜  work codex -c model='gpt-5.6-sol' -a never --disable hooks -c allow_login_shell=false\n" +
		"⚠ MCP startup incomplete (failed: node_repl)\n" +
		"\n" +
		"› Ask Codex to do anything\n" +
		"\n" +
		"  gpt-5.6-sol xhigh · /private/tmp/claude-501/…/scratchpad/work…"

	codexPaneOnRequest = "➜  work codex -c model='gpt-5.6-sol' -a on-request --disable hooks -c allow_login_shell=false\n" +
		"⚠ MCP startup incomplete (failed: node_repl)\n" +
		"\n" +
		"› Ask Codex to do anything\n" +
		"\n" +
		"  gpt-5.6-sol xhigh · /private/tmp/claude-501/…/scratchpad/work…"

	// What /status prints, for the record: the mode IS in the process, it
	// is just not on the screen until somebody types into the composer.
	codexStatusNever     = "│  Permissions:          Workspace (never)                    │"
	codexStatusOnRequest = "│  Permissions:          Workspace (Ask for approval)         │"
)

// grok 1.0.5, composer focused and empty in every arm (one character typed
// and deleted — the splash is what a launched pane shows first, and the
// splash arm below is what that costs).
const (
	grokPaneAuto = "  │ ❯                                                            │\n" +
		"  ╰──────────────────────────────────── Grok 4.6 (high) · auto ─╯\n" +
		"\n" +
		"  Shift+Tab:mode  │  Ctrl+.:shortcuts"

	// no --permission-mode at all: ~/.grok/config.toml [ui]
	// permission_mode = "always-approve" is what shows.
	grokPaneConfigAlwaysApprove = "  │ ❯                                                            │\n" +
		"  ╰──────────────────────────── Grok 4.6 (high) · always-approve ─╯\n" +
		"\n" +
		"  Shift+Tab:mode  │  Ctrl+.:shortcuts"

	// --permission-mode bypassPermissions renders the SAME string as the
	// config value above. The pane cannot tell the layers apart.
	grokPaneBypass = grokPaneConfigAlwaysApprove

	// default / acceptEdits / dontAsk / plan all render this.
	grokPaneNoSuffix = "  │ ❯                                                            │\n" +
		"  ╰──────────────────────────────────────── Grok 4.6 (high) ─╯\n" +
		"\n" +
		"  Shift+Tab:mode  │  Ctrl+.:shortcuts"

	// --permission-mode auto, splash still drawn: identical border to the
	// four-mode case above, so "no suffix" cannot even be read as "one of
	// those four" until the splash is gone.
	grokPaneSplashAuto = "   │                   New worktree                    ctrl+w  │\n" +
		"   │                   Resume session                  ctrl+s  │\n" +
		"   │                   Quit                            ctrl+q  │\n" +
		"   ╰──────────────────────────────────────────────────────────╯\n" +
		"  ╭────────────────────────────────────────────────────────────╮\n" +
		"  │ ❯                                                          │\n" +
		"  ╰──────────────────────────────────── Grok 4.6 (high) ─╯\n" +
		"\n" +
		"                                                     [stable]"
)

// ─── the reader under test ──────────────────────────────────────────────────

// paneMode is the corpus's handle on the SHIPPED reader. It used to be the
// reader itself, living here because posse had no field to put it on; the
// field landed with ranger-base-vwgt (HerdrSession.PermissionMode) and the
// reader moved to permissionmode.go with it, table and tail window intact.
//
// It delegates rather than duplicates, deliberately: two readers of one
// corpus go green apart, and every mutation this file was checked against —
// dropping the three non-"mode" spellings, guessing auto when no footer
// matches, reading codex's scrollback argv, calling grok's missing suffix
// "default" — has to kill the code that actually runs on the fleet.
//
// ok=false is "the pane does not prove a mode", which the shipped reader
// splits three ways (covered / unnameable / never) for the surfaces that
// have to say WHICH unknown they are showing. The corpus pins the split
// below; these two return values are what its per-runtime cases need.
func paneMode(runtime, pane string) (mode string, ok bool) {
	m := ReadPaneMode(runtime, pane)
	return m.Mode, m.State == PaneModeNamed
}

// ─── what the corpus pins ───────────────────────────────────────────────────

func TestQAClaudePaneNamesAllSixModes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, pane, want string }{
		{"auto", claudePaneAuto, "auto"},
		{"manual", claudePaneManual, "manual"},
		{"plan", claudePanePlan, "plan"},
		{"acceptEdits", claudePaneAcceptEdits, "acceptEdits"},
		{"bypassPermissions", claudePaneBypass, "bypassPermissions"},
		{"dontAsk", claudePaneDontAsk, "dontAsk"},
	} {
		got, ok := paneMode("claude", c.pane)
		if !ok || got != c.want {
			t.Errorf("%s: paneMode = %q,%v; want %q,true — the footer wording moved, or the reader only knows the three spelled \"<x> mode on\"", c.name, got, ok, c.want)
		}
	}
}

// The three modes claude does not spell with the word "mode" are the three
// that approve the most, so a reader that only knows "<x> mode on" reports
// "unknown" exactly where the risk is. This is that regression, stated as a
// property of the corpus rather than of the reader.
func TestQAClaudeFooterDropsTheWordModeOnTheApprovingOnes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, pane string }{
		{"acceptEdits", claudePaneAcceptEdits},
		{"bypassPermissions", claudePaneBypass},
		{"dontAsk", claudePaneDontAsk},
	} {
		if strings.Contains(c.pane, "mode on") {
			t.Errorf("%s: the capture now says \"mode on\" — re-measure; the reader's table may be able to shrink", c.name)
		}
	}
}

// A pane sitting on a dialog proves nothing, and the launch command in its
// own scrollback must not be mistaken for a reading.
func TestQAClaudeDialogHidesTheModeFooter(t *testing.T) {
	t.Parallel()
	if _, ok := paneMode("claude", claudePaneDialog); ok {
		t.Fatal("a dialog-covered pane reported a mode; the only mode text on that screen is the argv in the scrollback")
	}
	if !strings.Contains(claudePaneDialog, "--permission-mode manual") {
		t.Fatal("fixture no longer carries the launch command — it is the trap this case exists to pin")
	}
}

// codex renders no mode: the two arms differ only in the echoed argv.
func TestQACodexPaneCarriesNoPermissionMode(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, pane string }{
		{"-a never", codexPaneNever},
		{"-a on-request", codexPaneOnRequest},
	} {
		if got, ok := paneMode("codex", c.pane); ok {
			t.Errorf("%s: codex pane reported %q — nothing on that screen is a mode; check whether the reader started scanning the scrollback", c.name, got)
		}
	}
	strip := func(s string) string { // drop the shell line: that is the argv
		_, rest, _ := strings.Cut(s, "\n")
		return rest
	}
	if strip(codexPaneNever) != strip(codexPaneOnRequest) {
		t.Error("the codex TUI now distinguishes the two approval policies on screen — re-measure; a reader may become possible")
	}
	if !strings.Contains(codexStatusNever, "(never)") || !strings.Contains(codexStatusOnRequest, "Ask for approval") {
		t.Error("the /status wording moved; it is the only in-pane surface that names codex's policy")
	}
}

func TestQAGrokPaneNamesOnlyTwoOfSixModes(t *testing.T) {
	t.Parallel()
	if got, ok := paneMode("grok", grokPaneAuto); !ok || got != "auto" {
		t.Errorf("grok auto: paneMode = %q,%v; want \"auto\",true", got, ok)
	}
	// bypassPermissions and the operator's config value are the same string.
	got, ok := paneMode("grok", grokPaneBypass)
	if !ok || got != "always-approve" {
		t.Errorf("grok bypassPermissions: paneMode = %q,%v; want \"always-approve\",true", got, ok)
	}
	if cfg, cok := paneMode("grok", grokPaneConfigAlwaysApprove); !cok || cfg != got {
		t.Errorf("grok config-only: paneMode = %q,%v; the flag and ~/.grok/config.toml rendered differently — the pane may now say which layer won", cfg, cok)
	}
	// default / acceptEdits / dontAsk / plan: nothing at all.
	if got, ok := paneMode("grok", grokPaneNoSuffix); ok {
		t.Errorf("an unsuffixed grok composer reported %q; four modes render that way and none of them may be guessed", got)
	}
	// splash up, and the session IS auto.
	if got, ok := paneMode("grok", grokPaneSplashAuto); ok {
		t.Errorf("a grok pane on its startup splash reported %q; that pane was launched --permission-mode auto and shows no suffix", got)
	}
}

// ─── the live arm ───────────────────────────────────────────────────────────

// Re-measure against a real pane. Recipe (2026-08-29), one runtime per run:
//
//	herdr --session qa0emp server &
//	export HERDR_SOCKET_PATH=~/.config/herdr/sessions/qa0emp/herdr.sock
//	herdr workspace create --cwd <scratch> --no-focus       # note the pane
//	herdr pane run <pane> "claude --permission-mode auto"   # or codex/grok
//	RHQ_LIVE_PANE=<pane> RHQ_LIVE_RUNTIME=claude RHQ_LIVE_MODE=auto \
//	  go test ./internal/rhq -run TestQALivePermissionModeInPane -v
//
// It submits nothing, so it costs no turn. grok needs one character typed
// and deleted first (the splash), and a claude pane in a fresh directory
// answers its trust dialog before any footer exists.
func TestQALivePermissionModeInPane(t *testing.T) {
	t.Parallel()
	pane, runtime := os.Getenv("RHQ_LIVE_PANE"), os.Getenv("RHQ_LIVE_RUNTIME")
	if pane == "" || runtime == "" {
		t.Skip("set RHQ_LIVE_PANE=<ws:pane> RHQ_LIVE_RUNTIME=claude|codex|grok (+ HERDR_SOCKET_PATH) — see the comment above")
	}
	bin := os.Getenv("RHQ_HERDR_BIN")
	if bin == "" {
		bin = "herdr"
	}
	text, err := Herdr{Bin: bin}.PaneRead(pane, 0)
	if err != nil {
		t.Fatalf("herdr pane read %s: %v (is the pane live?)", pane, err)
	}
	got, ok := paneMode(runtime, text)
	t.Logf("%s pane %s: mode=%q readable=%v", runtime, pane, got, ok)
	want := os.Getenv("RHQ_LIVE_MODE")
	if want == "" {
		return // reading it is the whole test when no expectation is named
	}
	if want == "-" { // the caller asserts the pane proves NOTHING
		if ok {
			t.Fatalf("expected an unreadable pane, got mode %q", got)
		}
		return
	}
	if !ok || got != want {
		t.Fatalf("mode = %q,%v; want %q — pane text:\n%s", got, ok, want, text)
	}
}
