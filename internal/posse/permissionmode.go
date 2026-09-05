package posse

import (
	"fmt"
	"io"
	"strings"
)

// The permission mode a session is ACTUALLY in, read off its own pane
// (ranger-base-vwgt, ask 1 of ranger-base-0emp).
//
// ADR 0035 §3 rests on this. grok gets no second mode layer, and the ADR
// names what stands in for one: "the actual pane mode is read from the
// composer border and surfaced in `rhq list`/gates, so a flag-lost grok
// session is *visible*, not prevented. Under ADR 0017 this is a DECLARED
// DIFFERENCE, not an UNKNOWN."
//
// The launch template, the session meta and the command line sitting in the
// pane's own scrollback all answer a different question — what the launch
// ASKED for — and the whole reason this field exists is the session that
// ended up somewhere else: a hand-relaunched pane, a drifted template, an
// argv-building path that dropped a token (dispatch has two; ranger-base-unzn).
// So nothing here may be filled from the meta, and the read is bounded to the
// live screen tail on purpose.
//
// What each runtime will say was MEASURED 2026-08-29 against a scratch herdr
// server, one workspace per arm, no prompt ever submitted, on claude 2.1.251,
// codex-cli 0.150.1 and grok 1.0.5. The corpus is verbatim `herdr pane read`
// output; it lives in permissionmodepane_qa_test.go and drives the readers
// below, so there is one reader and one corpus rather than a production copy
// that can drift from the captures that justify it.
//
// The three runtimes do not offer the same contract, so this is three-valued
// PER RUNTIME rather than "a mode or a blank":
//
//   - claude names all six modes in the footer, three of them WITHOUT the
//     word "mode" (accept edits on / bypass permissions on / don't ask on) —
//     hence a table of names and not a /(\w+) mode on/ regexp, which would go
//     blind on exactly the three that approve the most. A modal dialog
//     REPLACES the footer, and that is PaneModeCovered: the state an operator
//     hand-clears queues in, and the one place a UI saying "auto" would be
//     worse than one saying "can't tell".
//   - grok names two of six on the composer border (auto, and always-approve
//     for bypassPermissions). default, acceptEdits, dontAsk, plan and a pane
//     still on the startup splash all render NO suffix, which is
//     PaneModeUnnameable — four modes and an unknown, never "default".
//   - codex renders NOTHING, on any screen: `-a never` and `-a on-request`
//     differ only in the echoed argv. Its column is PaneModeNever — a
//     permanent "—", not an unknown somebody could close with more work.
//
// ADR 0035 §4 governs how any of this may be rendered: a mode is a default
// disposition, not a non-blocking promise (a claude session in auto mode was
// measured blocked on "Auto mode classifier requires confirmation for this
// command"). The tags below name the mode and nothing else; the blocked/queue
// state stays a separate column.
type PaneMode struct {
	State PaneModeState
	// Mode is what the pane NAMED, in the pane's own vocabulary: claude's
	// --permission-mode spellings, and grok's border words (auto,
	// always-approve). Empty unless State is PaneModeNamed.
	Mode string
	// Why is the one clause a surface prints instead of a mode. It is set on
	// every state but PaneModeNamed — and on one named reading, grok's
	// always-approve, where the mode is known and its LAYER is not.
	Why string
}

// PaneModeState is what the pane read established. Its zero value is
// PaneModeUnread, which is "this listing did not read one" and must never be
// rendered as a mode or as a blank — see PaneMode's doc for why a blank is
// the defect.
type PaneModeState string

const (
	PaneModeUnread     PaneModeState = ""           // no reading was taken, or it failed
	PaneModeNamed      PaneModeState = "named"      // the pane named a mode
	PaneModeCovered    PaneModeState = "covered"    // this runtime renders a mode; this screen is not showing one
	PaneModeUnnameable PaneModeState = "unnameable" // this runtime cannot name the mode this session is in
	PaneModeNever      PaneModeState = "never"      // this runtime renders no mode on any screen
)

// paneModeReadLines is how much of the pane a listing carries to the reader.
// The reader narrows to the last three non-empty lines whatever it is handed,
// so this is not the guard — it is the same rule applied a step earlier, so a
// pane with a long scrollback is not marshalled whole to answer one question.
// It is Herdr.PaneRead's client-side tail (herdr's own --lines counts padded
// screen rows), and 40 is the bound the runtime probe's pane read already
// takes.
const paneModeReadLines = 40

// claudeFooterModes maps claude's FOOTER wording to the --permission-mode
// spelling. Three of the six do not contain the word "mode"; that is the
// whole reason this is a table and not a regexp (measured, claude 2.1.251).
var claudeFooterModes = []struct{ footer, mode string }{
	{"auto mode on", "auto"},
	{"manual mode on", "manual"},
	{"plan mode on", "plan"},
	{"accept edits on", "acceptEdits"},
	{"bypass permissions on", "bypassPermissions"},
	{"don't ask on", "dontAsk"},
}

// paneModeReaders is the set of runtimes whose pane can be read for a mode at
// all. codex is deliberately absent rather than mapped to a reader that
// always fails: "no reader" and "measured to render nothing" are different
// facts, and only the second is a permanent "—" (see ReadPaneMode).
var paneModeReaders = map[string]func(tail []string) PaneMode{
	"claude": claudePaneMode,
	"grok":   grokPaneMode,
}

// PaneModeReadable reports whether reading this runtime's pane can say
// anything about its permission mode. It is what keeps a listing from
// spending a herdr call per codex session to learn what was measured once.
func PaneModeReadable(runtime string) bool {
	_, ok := paneModeReaders[runtime]
	return ok
}

// ReadPaneMode reads a permission mode out of rendered pane text. Every
// not-named answer carries its own Why, because the surfaces that render this
// have to say WHICH kind of unknown they are showing.
//
// Only the last three non-empty lines are considered, because the scrollback
// above them holds the launch command line — the argv this whole exercise is
// about not trusting. It does NOT hold an earlier session's footer: measured
// 2026-08-29, exiting a `--permission-mode bypassPermissions` claude with
// /exit and relaunching it manual in the same pane left exactly one footer on
// screen, the live one.
func ReadPaneMode(runtime, pane string) PaneMode {
	if runtime == "codex" {
		// Measured on codex-cli 0.150.1: the footer is `<model> <effort> ·
		// <cwd>` under `-a never` and `-a on-request` alike, and the policy
		// is reachable in-pane only by typing /status. Typing into a session
		// is not a read, so this never becomes an unknown that more work
		// closes.
		return PaneMode{State: PaneModeNever, Why: "codex renders no approval policy on any screen — it is reachable in-pane only by typing /status, and typing into a session is not a read"}
	}
	read, ok := paneModeReaders[runtime]
	if !ok {
		return PaneMode{Why: fmt.Sprintf("posse has no pane reader for runtime %q — nobody has measured what its pane says", runtime)}
	}
	var tail []string
	for _, ln := range strings.Split(pane, "\n") {
		if strings.TrimSpace(ln) != "" {
			tail = append(tail, ln)
		}
	}
	if len(tail) > 3 {
		tail = tail[len(tail)-3:]
	}
	return read(tail)
}

func claudePaneMode(tail []string) PaneMode {
	for _, ln := range tail {
		for _, m := range claudeFooterModes {
			if strings.Contains(ln, m.footer) {
				return PaneMode{State: PaneModeNamed, Mode: m.mode}
			}
		}
	}
	return PaneMode{State: PaneModeCovered, Why: "no mode footer on screen — a modal dialog replaces it, and the launch command in the scrollback is a claim about the launch, not a reading"}
}

func grokPaneMode(tail []string) PaneMode {
	// The startup splash gets no special case, and that is measured rather
	// than assumed: a pane still showing the New worktree / Resume session
	// menu draws its composer border WITHOUT a suffix even when the launch
	// said --permission-mode auto, so the rule below already returns "cannot
	// tell" for it.
	for _, ln := range tail {
		if !strings.Contains(ln, "╰") || !strings.Contains(ln, "Grok ") {
			continue
		}
		border := strings.TrimSuffix(strings.TrimSpace(ln), "─╯")
		i := strings.LastIndex(border, "· ")
		if i < 0 {
			break
		}
		mode := strings.TrimSpace(border[i+len("· "):])
		m := PaneMode{State: PaneModeNamed, Mode: mode}
		if mode == GrokBorderAlwaysApprove {
			// The same string the operator's ~/.grok/config.toml `[ui]
			// permission_mode` produces with no flag at all. The MODE is
			// known; which layer set it is not, and ADR 0035 §3 is the
			// reason anyone is reading this field.
			m.Why = "the mode is bypassPermissions, but the border cannot say whether the launch flag or the operator's ~/.grok/config.toml set it — both render this word"
		}
		return m
	}
	return PaneMode{State: PaneModeUnnameable, Why: "grok names only auto and always-approve on the composer border — default, acceptEdits, dontAsk, plan and a pane still on the startup splash all render nothing, so this is four modes and an unknown, never \"default\""}
}

// GrokBorderAlwaysApprove is grok's border word for bypassPermissions, and
// also what ~/.grok/config.toml `[ui] permission_mode` renders (ADR 0035 §3).
const GrokBorderAlwaysApprove = "always-approve"

// Tag is the one-token form for a listing row. Every state renders something:
// a blank that reads as "fine" is the defect this field exists to replace, and
// the three unknowns are three different things — `?covered` clears on the
// next read, `?unnamed` is grok telling you it names only two of six, and `—`
// is codex, which can never fill this column at all.
func (m PaneMode) Tag() string {
	switch m.State {
	case PaneModeNamed:
		return "mode:" + m.Mode
	case PaneModeCovered:
		return "mode:?covered"
	case PaneModeUnnameable:
		return "mode:?unnamed"
	case PaneModeNever:
		return "mode:—"
	default:
		return "mode:?"
	}
}

// Line is the sentence form, for a report with room for the Why. The tag
// leads it so the two surfaces read as one vocabulary.
func (m PaneMode) Line() string {
	why := m.Why
	if why == "" && m.State != PaneModeNamed {
		// The zero value reaches here whenever a caller renders a listing it
		// never asked for the reads on. It is still not a blank: what it
		// means is that nobody looked, which is a different sentence from
		// every measured unknown above.
		why = "no reading was taken of this session's pane"
	}
	if why == "" {
		return m.Tag()
	}
	return m.Tag() + " — " + why
}

// SessionModeReport writes what each of this persona's live sessions has on
// its pane. It is the `posse gates <persona>` half of ADR 0035 §3: gates
// takes a PERSONA and the mode is a fact about a SESSION, so the report walks
// the persona's sessions rather than claiming one mode for the PID.
//
// It reads panes, so it is called from the report and never from the listing
// path the cockpit refreshes on. A herdr that cannot be reached is reported,
// not fatal: the parity report above it is true whether or not any session is
// running.
func (b *HerdrBackend) SessionModeReport(w io.Writer, persona string) {
	b.PaneModes = true
	defer func() { b.PaneModes = false }()
	sessions, err := b.Sessions()
	if err != nil {
		fmt.Fprintf(w, "  live sessions: unreadable — %v\n", err)
		return
	}
	n := 0
	for _, s := range sessions {
		if s.Agent != persona || s.Foreign {
			continue
		}
		n++
		fmt.Fprintf(w, "  %s %s [%s]: %s\n", s.Emoji, s.Name, s.Runtime, s.PermissionMode.Line())
	}
	if n == 0 {
		fmt.Fprintf(w, "  no live %s session — the permission mode is a fact about a running pane, so there is none to read\n", persona)
	}
}
