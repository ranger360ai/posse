package posse

import (
	"fmt"
	"io"
	"sort"
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
//     PaneModeUnnameable — four modes and an unknown, never "default". Those
//     two words are a TABLE for the same reason claude's spellings are: the
//     border suffix is not a mode field, so a suffix that is not one of them
//     is the same unknown and not a mode grok named.
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

// The registered pane readers — the legal values of a runtime's `pane_mode:`
// key and the keys of paneModeReaders below (ADR 0057 D1). A runtime declares
// WHICH READER parses its screen, never its own name: two CLIs that paint the
// same footer share one reader, and a CLI measured to paint nothing declares
// that as a reading of its own.
//
// PaneModeNone is a declaration and not an absence, which is the distinction
// the whole three-valued field turns on. It is the ADAPTER whose answer is
// the PaneModeNever STATE; a runtime that declares nothing at all is
// PaneModeUnread instead (PaneModeUndeclared).
const (
	PaneModeClaudeFooter = "claude-footer"
	PaneModeGrokBorder   = "grok-border"
	PaneModeNone         = "none"
)

// The Why every not-named reading carries, one per reader. Named constants
// rather than literals at the return sites because the registry entry and
// the reader that produces it must be the same sentence: the entry is what
// `posse runtime check` renders the dimension's row from (ADR 0057 D3), and
// a row describing an absence the reader does not actually report is scenery.
const (
	paneModeCoveredWhy = "no mode footer on screen — a modal dialog replaces it, and the launch command in the scrollback is a claim about the launch, not a reading"

	paneModeUnnameableWhy = "grok names only auto and always-approve on the composer border — default, acceptEdits, dontAsk, plan and a pane still on the startup splash all render nothing, and a border suffix outside those two words is grok saying something that is not a mode, so this is four modes and an unknown, never \"default\""

	// Generic on purpose: `none` is a reader, and a second CLI measured to
	// render nothing declares the same one. The measurement behind a
	// particular runtime's `none` stays where the declaration is — the
	// built-in's own comment, and the grid row's note (ADR 0057 D3).
	paneModeNeverWhy = "this runtime renders no permission mode on any screen — measured, so the column is a permanent \"—\" rather than an unknown more work could close"
)

// PaneModeReader is one entry of that registry, and it is the whole
// declaration: what parses a pane of this shape, whether a listing should
// spend a herdr call on one at all, what an absent mode MEANS here, and the
// one line the runtime grid renders the dimension's row from.
type PaneModeReader struct {
	// Read turns the pane tail into a reading. A reader with ReadsPane
	// false is handed nil — its answer is the measurement, not a scrape.
	Read func(tail []string) PaneMode
	// ReadsPane is whether reading a pane here can say anything. false is
	// `none`: paying a herdr call per session to re-learn a constant buys
	// nothing, so fillPaneModes skips the read entirely.
	ReadsPane bool
	// Absence is the Why a not-named reading carries — the sentence a
	// surface prints instead of a mode, so it can say WHICH unknown it is
	// showing.
	Absence string
	// Contract is what this reader reads, in one line, for the grid row.
	Contract string
}

// paneModeReaders is the registry. The map is the whole set of legal
// `pane_mode:` values: a name that is not a key here is refused at load
// (runtime.go), so a declaration can never promise a reading nothing
// performs — the same contract turn_outcome: carries (turnfailure.go).
var paneModeReaders = map[string]PaneModeReader{
	PaneModeClaudeFooter: {
		Read: claudePaneMode, ReadsPane: true, Absence: paneModeCoveredWhy,
		Contract: "the footer, in the last three non-empty lines: it names all six modes and three of them without the word \"mode\", and a modal dialog REPLACES it, so absence is COVERED and clears on the next read",
	},
	PaneModeGrokBorder: {
		Read: grokPaneMode, ReadsPane: true, Absence: paneModeUnnameableWhy,
		Contract: "the composer border's suffix: it names two of six modes and the suffix is matched against those two words, so absence is UNNAMEABLE — four modes, a pane still on the splash, and any suffix that is not one of the two, never \"default\"",
	},
	PaneModeNone: {
		Read: neverPaneMode, ReadsPane: false, Absence: paneModeNeverWhy,
		Contract: "no pane read at all: this CLI was measured to render no mode on any screen, so absence is NEVER — a permanent \"—\", and a listing spends no herdr call per session",
	},
}

// PaneModeAdapters is every registered reader name, sorted — what a refused
// declaration lists so the operator can see what is on offer.
func PaneModeAdapters() []string {
	names := make([]string, 0, len(paneModeReaders))
	for name := range paneModeReaders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// PaneModeReaderFor is the reader a `pane_mode:` value resolves to. ok=false
// is an unregistered name, which LoadRuntime refuses — so every caller
// downstream of a loaded runtime holds either a registered reader or the
// empty declaration.
func PaneModeReaderFor(adapter string) (PaneModeReader, bool) {
	r, ok := paneModeReaders[adapter]
	return r, ok
}

// PaneModeReadable reports whether a listing should spend a pane read on
// this ADAPTER. It is what keeps a listing from paying a herdr call per
// codex session to learn what was measured once — and it asks the
// declaration, never the runtime's name (ADR 0017 §3).
func PaneModeReadable(adapter string) bool {
	r, ok := paneModeReaders[adapter]
	return ok && r.ReadsPane
}

// PaneModeUndeclared is the loud default: a runtime that declares no
// `pane_mode:` at all. It is not a mode, not a blank and not `none` — the
// difference between "nobody has measured what this CLI's pane says" and
// "somebody measured it and it says nothing" is the whole reason `none` is a
// declaration.
func PaneModeUndeclared(runtime string) PaneMode {
	return PaneMode{Why: fmt.Sprintf("posse has no pane reader for runtime %q — nobody has measured what its pane says", runtime)}
}

// ReadPaneMode reads a permission mode out of rendered pane text, with the
// reader the runtime DECLARED (its `pane_mode:` adapter). Every not-named
// answer carries its own Why, because the surfaces that render this have to
// say WHICH kind of unknown they are showing.
//
// Only the last three non-empty lines are considered, because the scrollback
// above them holds the launch command line — the argv this whole exercise is
// about not trusting. It does NOT hold an earlier session's footer: measured
// 2026-08-29, exiting a `--permission-mode bypassPermissions` claude with
// /exit and relaunching it manual in the same pane left exactly one footer on
// screen, the live one.
func ReadPaneMode(adapter, pane string) PaneMode {
	r, ok := paneModeReaders[adapter]
	if !ok {
		// Unreachable from a loaded runtime — LoadRuntime refuses an
		// unregistered pane_mode:, and an undeclared one is answered by
		// PaneModeUndeclared with the runtime's name in it. A caller that
		// hands this a name anyway gets an unknown, never a mode.
		return PaneMode{Why: fmt.Sprintf("posse has no pane reader named %q — the registered readers are %s", adapter, strings.Join(PaneModeAdapters(), ", "))}
	}
	if !r.ReadsPane {
		return r.Read(nil)
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
	return r.Read(tail)
}

// neverPaneMode is the `none` reader. It takes no pane because there is
// nothing on any screen to take: codex 0.150.1 renders the same footer under
// `-a never` and `-a on-request`, and the policy is reachable in-pane only by
// typing /status — typing into a session is not a read, so this never becomes
// an unknown that more work closes.
func neverPaneMode([]string) PaneMode {
	return PaneMode{State: PaneModeNever, Why: paneModeNeverWhy}
}

func claudePaneMode(tail []string) PaneMode {
	for _, ln := range tail {
		for _, m := range claudeFooterModes {
			if strings.Contains(ln, m.footer) {
				return PaneMode{State: PaneModeNamed, Mode: m.mode}
			}
		}
	}
	return PaneMode{State: PaneModeCovered, Why: paneModeCoveredWhy}
}

// grokBorderModes is grok's composer-border vocabulary, and it is CLOSED for
// the same reason claudeFooterModes is: what follows the last "· " on that
// border is not a mode FIELD, it is whatever grok chose to draw after the
// model name. Two of the six modes are drawn there and nothing else on that
// border was ever measured to be a mode (grok 1.0.5), so a suffix outside
// this table is an unknown and not a reading — the direction the whole
// three-valued field exists to hold, because a surface that says "auto" for
// a pane it cannot read is worse than one that says "can't tell".
var grokBorderModes = []struct{ border, mode string }{
	{"auto", "auto"},
	{GrokBorderAlwaysApprove, GrokBorderAlwaysApprove},
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
		// TrimRight over the whole box-drawing run, not TrimSuffix of one
		// "─╯": that run is PADDING whose length is the pane width, and the
		// captured corpus happens to carry exactly one dash before the
		// corner. Stripping one left a wider pane reading "auto ────",
		// which the table below would answer safely — as an unknown — but
		// needlessly, because the mode is right there on the screen.
		border := strings.TrimRight(strings.TrimSpace(ln), "─╯ ")
		i := strings.LastIndex(border, "· ")
		if i < 0 {
			break
		}
		suffix := strings.TrimSpace(border[i+len("· "):])
		for _, m := range grokBorderModes {
			if suffix != m.border {
				continue
			}
			p := PaneMode{State: PaneModeNamed, Mode: m.mode}
			if m.mode == GrokBorderAlwaysApprove {
				// The same string the operator's ~/.grok/config.toml `[ui]
				// permission_mode` produces with no flag at all. The MODE is
				// known; which layer set it is not, and ADR 0035 §3 is the
				// reason anyone is reading this field.
				p.Why = "the mode is bypassPermissions, but the border cannot say whether the launch flag or the operator's ~/.grok/config.toml set it — both render this word"
			}
			return p
		}
		// A suffix this table does not carry. The LAST field is the only
		// position a mode was ever measured in, so a token there that is not
		// one of the two words is grok saying something else — a file count,
		// a token counter, an empty pad. Reporting it would put an unmeasured
		// string in front of the operator as if the pane had named it.
		break
	}
	return PaneMode{State: PaneModeUnnameable, Why: paneModeUnnameableWhy}
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
