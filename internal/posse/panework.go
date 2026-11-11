package posse

// What a settled agent is still holding — ranger-base-htafy.
//
// THE INCIDENT (measured by the coordinating persona on 2026-09-02 at
// 08:08Z and 09:29Z, and again at 11:05Z). Two dispatched sessions finished their edits, started the repo's
// full suite behind a Monitor, and went idle to wait for the wake. Every
// store posse reads said "settled": herdr's `agent list` reports
// agent_status idle|done, `agent prompt --wait` returns the instant the
// turn ends, and neither carries a word about the work the agent is waiting
// ON. So the pass judged a settle-open, the pulse reported a `settled:`
// condition to the coordinator, and --resume typed a re-prompt into a
// session that was doing exactly what it had been told. The third
// occurrence added the other half: the re-prompt did not submit — it sat in
// the composer for four hours, and a pass that reads an idle pane as
// settled cannot tell that from an agent that stopped.
//
// WHAT THE STORES ACTUALLY CARRY (herdr 0.8.2, claude 2.1.258, measured
// 2026-09-02 over seven live panes — the bead's "measure what herdr's agent
// JSON carries and use it"):
//
//   - `herdr agent list`: agent, agent_status, cwd, pane/tab/workspace ids,
//     terminal title, revision, state_change_seq. NO task field at all.
//   - the detection manifest (claude.toml 2026.08.31.1) has a working rule
//     for `N MCP tasks still running` and one for `Waiting for N background
//     agents to finish`, and NONE for a live shell or monitor: an agent
//     idle behind its own suite run matches live_prompt_box, state idle.
//   - `herdr agent explain --json`: evaluated_rules, one entry per manifest
//     rule, each naming the SCREEN REGION that rule reads and previewing
//     what was in it — evaluated for every rule, not only the one that
//     matched. Two of those regions answer this bead:
//
//     after_last_horizontal_rule  claude's footer hint line
//     prompt_box_body             the composer: ❯ and whatever is typed
//
// Footers, verbatim from that measurement:
//
//	⏵⏵ auto mode on (shift+tab to cycle) · esc to interrupt · ← for agents
//	⏵⏵ auto mode on · 1 shell · esc to interrupt · ← for agents · ↓ to manage
//	⏵⏵ auto mode on · 1 shell, 1 monitor · esc to interrupt · ← for agents · ↓ to manage
//	⏵⏵ auto mode on · 1 monitor · esc to interrupt · ← for agents · ↓ to manage
//	⏵⏵ auto mode on (shift+tab to cycle) · ← for agents                (idle, nothing running)
//	⏵⏵ auto mode on · 1 shell · ← for agents · ↓ to manage              (IDLE, one still running)
//
// The `1 shell, 1 monitor` part is claude's own summary of its live
// background tasks. Its whole vocabulary is in the 2.1.258 binary — shells,
// monitors, teams, local agents, cloud sessions, remote and background
// dynamic workflows, Artifact comment monitors, MCP tasks, `N background
// tasks` when the kinds are mixed, and the three uncounted ones (dreaming,
// auto-mode scan, ultraplan).
//
// THAT IT SURVIVES THE SETTLE IS THE LAST LINE ABOVE, and it is a capture
// rather than an inference: a live pane at 14:38:10, `agent list` reporting
// agent_status idle, `agent explain` matching live_prompt_box — settled by
// every reading posse had — with `1 shell` still in its footer. The pane
// could not be put into that state on demand (it takes a session and a
// turn), so it was watched for; the renderer says the same thing, which is
// why it was worth watching for. The footer component (2.1.258) returns
// null if and only if the live-task list is empty, and nothing about the
// turn reaches it — while the surrounding parts ARE turn-conditional, which
// is why the idle capture keeps `auto mode on` and `← for agents` and drops
// `esc to interrupt`. If this ever stops holding the summary is simply
// absent, BackgroundWork answers "", and the settle is judged exactly as it
// was before this bead.
//
// PREVIEWS ARE TRUNCATED at 243 characters (measured: a 4949-byte
// whole_recent region previews at 243 chars). The footer measures 73-99
// bytes, so it arrives whole; a long typed prompt does not, so the question
// asked HERE is only whether the box is EMPTY. Whether the text in a
// non-empty box is a prompt somebody is about to send, or the line the pane
// last submitted showing through, is a second question and a different
// store: sentline.go and ranger-base-2hvtv. Truncation is why that
// comparison is prefix-shaped, and ComposerTruncated below is how it knows
// per reading rather than assuming.
//
// BOTH REGIONS ARE CLAUDE'S. Of the twenty detection manifests herdr ships
// (2026-09-02), `prompt_box_body` appears in claude.toml alone, and the task
// vocabulary below is claude's own — so a codex, grok or gemini pane
// previews neither, answers the empty hold, and is judged exactly as it was
// before this bead. That is a gap and not a bug: the reading is only ever
// allowed to say "this settle is not one", never to invent one.
//
// EVERY READING IS BEST-EFFORT AND FAILS TOWARDS TODAY. A herdr that cannot
// explain, a manifest whose rules no longer read those regions, a screen
// posse does not recognize: all of them answer the empty hold, and the
// settle is then judged exactly as it was before this bead. Ignorance is
// not evidence that an agent is waiting, and a hold invented out of one
// would keep a genuinely stuck bead claimed forever.
//
// BOTH REGIONS WERE ALSO HERDR'S TO READ, and that is a different sentence
// from the one above it — the regions are claude's, the READ was `agent
// explain`, which is ADR 0076 §4. So this whole file was off on a
// `multiplexer: rex` instance, silently, from the day the slot changed: not
// "claude is not in the pane" but "the question was never asked". The WORK
// half is realized a second way for that occupant (capturedHold below,
// ranger-base-r5131), off §2's `Read`, which every occupant has. So is the
// COMPOSER half (capturedcomposer.go, ranger-base-qzujz) — it waited on a
// second store, because a Typed reading with no echo check behind it invents
// holds, and the id that check needs was §4's until ADR 0076's Amendment 6
// found it in claude's own per-session file.

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// footerRegion is claude's hint line — the region herdr's blocked-form
	// rules read, and the only one that carries the background-task summary.
	footerRegion = "after_last_horizontal_rule"

	// composerRegion is the prompt box: `❯` and what is typed after it.
	composerRegion = "prompt_box_body"

	// composerMark opens the prompt box body. It is REQUIRED before any of
	// this reads a composer: the same region previews a dialog's body when
	// one is drawn over the box (measured on a live pane holding an
	// AskUserQuestion form), and reading that as typed text would call every
	// blocked pane un-submitted.
	composerMark = "❯"

	// footerSep separates the footer's parts. claude joins them with a
	// middle dot; nothing else in the line uses one.
	footerSep = "·"

	// composerANSIRegion is what the readings log calls the ONE line
	// ghostbox.go decides on: the last composer line of an `agent read
	// --format ansi`, escapes intact. It is posse's own name and not
	// herdr's — herdr has no region for it, which is the whole reason that
	// second read exists — so it is spelled apart from the two above.
	composerANSIRegion = "prompt_box_ansi_line"

	// captureFooterRegion is footerRegion read off a CAPTURE instead of off
	// herdr's working (capturedHold, ranger-base-r5131). Posse's own name
	// for composerANSIRegion's reason, and deliberately NOT footerRegion's
	// spelling: the two are the same line of claude's screen located two
	// different ways, and a readings census that could not tell them apart
	// could not say which route a withheld settle was withheld on.
	captureFooterRegion = "capture_after_last_horizontal_rule"

	// captureComposerRegion is composerRegion read off a CAPTURE, located
	// between the screen's last two rules rather than named by a manifest
	// (capturedcomposer.go, ranger-base-qzujz). Spelled apart from
	// composerRegion for captureFooterRegion's reason: the two are the same
	// box located two different ways, and a readings census that could not
	// tell them apart could not say which route withheld a settle.
	captureComposerRegion = "capture_prompt_box_body"
)

// captureFooterLines is how much of a captured screen's tail capturedHold
// asks for. claude's footer is the last two or three lines of it, so this is
// slack and not a budget: enough that a footer pushed up by a few lines of
// chrome is still in the window, small enough that nothing here ever parses
// a scrollback (captureFooter's measurement).
const captureFooterLines = 40

// footerRuleRune is the character claude draws its horizontal rules with —
// U+2500 BOX DRAWINGS LIGHT HORIZONTAL (MEASURED 2026-11-10 off a live
// capture: the rule lines are that rune and nothing else).
const footerRuleRune = '─'

// backgroundTaskCount matches one counted entry of claude's task summary —
// the nouns are the binary's own (2.1.258), not a guess. A summary of
// several kinds is those entries joined with ", " ("1 shell, 1 monitor"),
// so the parts are checked one at a time and ALL of them must match: a
// footer part that is mostly prose does not become live work because a
// number and a noun appear somewhere inside it.
var backgroundTaskCount = regexp.MustCompile(`^[1-9]\d* (?:shells?|monitors?|teams?|local agents?|cloud sessions?|` +
	`remote dynamic workflows?|background dynamic workflows?|Artifact comment monitors?|MCP tasks?|background tasks?)$`)

// backgroundTaskWord is the rest of that vocabulary: the summaries claude
// draws with no count in front of them.
var backgroundTaskWord = map[string]bool{"dreaming": true, "auto-mode scan": true}

// backgroundTaskPrefix is the ultraplan family, whose summaries are a
// glyph, the word, and a phase ("ultraplan", "ultraplan ready",
// "ultraplan needs your input").
const backgroundTaskPrefix = "ultraplan"

// PaneHold is what a pane that herdr calls settled is still holding: work
// the agent is waiting on, and text sitting unsent in its composer. The
// zero value is "nothing, or nothing posse could see" — the two are one
// answer here on purpose (see the note above).
type PaneHold struct {
	// Work is claude's own summary of its live background tasks, verbatim
	// ("1 shell, 1 monitor"). "" = none visible.
	Work string
	// Typed is the text sitting in the prompt box. "" = the box is empty,
	// no composer was on screen to read, or the box is echoing a line this
	// pane has already submitted (Sent below).
	Typed string
	// Sent is the line the box is echoing back: text claude's own submit
	// log says was already sent in this pane's session, so it is NOT a
	// hold and Typed is empty beside it. "" = no echo was found, which is
	// also what an unreadable store answers (sentline.go).
	Sent string
	// Ghost is the line the box is DRAWING FAINT: claude's own suggested
	// next prompt, written into an empty box by nobody, so it is not a
	// hold either and Typed is empty beside it. "" = the box was not
	// wholly dim, or posse could not read the escapes (ghostbox.go).
	//
	// It is reported by every caller and acted on by all but one. The
	// measurement behind it is half-taken — a dim SUGGESTION is measured
	// 60 times, a typed line measured not-dim zero times — so it may
	// retire a claim posse makes about a box and may never license a
	// keystroke into one. ghostbox.go's note carries both halves and the
	// command that finishes them.
	Ghost string
	// Read is what this hold was read OFF — herdr's own verdict and the
	// exact region bytes the readings above decided on, carried out of the
	// reading so the caller that spends it can write it down (ADR 0066 D1,
	// readingslog.go).
	//
	// It rides on the hold rather than being re-read at the consequence
	// site because the consequence sites do not have a pane: they have a
	// holder's session name, a bead and a verdict, and a second `agent
	// explain` taken to log the first one would be a different screen. The
	// zero value is "nothing was read", which is what every failure arm in
	// this file answers with.
	Read ReadingEvidence
}

// Waiting reports whether this pane's idle is a wait rather than a settle.
func (h PaneHold) Waiting() bool { return h.Work != "" || h.Typed != "" }

// Why says what the pane is holding, in one clause, for the line that
// refuses to treat it as settled. "" when it is holding nothing.
func (h PaneHold) Why() string {
	var parts []string
	if h.Work != "" {
		parts = append(parts, h.Work+" still running")
	}
	if h.Typed != "" {
		parts = append(parts, fmt.Sprintf("a prompt sitting UNSENT in its box (%q)", ellipsis(h.Typed, 60)))
	}
	return strings.Join(parts, " and ")
}

// ellipsis trims a screen reading down to something a report line can carry
// whole.
func ellipsis(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// regionPreview returns herdr's preview of one screen region, taken from
// whichever rule reads it. Rules are evaluated whatever matched, so this
// does not depend on the state the pane is in — only on the manifest still
// having a rule that reads the region at all.
func (d AgentDetection) regionPreview(region string) string {
	for _, r := range d.EvaluatedRules {
		if r.Region == region && r.Evidence.RegionPreview != "" {
			return r.Evidence.RegionPreview
		}
	}
	return ""
}

// ComposerTruncated reports whether herdr cut the composer preview down —
// the 243-character cap the note above measures, asked per reading rather
// than assumed, because `agent explain` reports the region's real byte count
// beside the preview it cut. A region with no rule reading it answers false:
// there is no preview, so nothing was truncated.
func (d AgentDetection) ComposerTruncated() bool {
	for _, r := range d.EvaluatedRules {
		if r.Region == composerRegion && r.Evidence.RegionPreview != "" {
			return r.Evidence.RegionBytes > len(r.Evidence.RegionPreview)
		}
	}
	return false
}

// BackgroundWork returns claude's footer summary of the background tasks it
// is running, or "" when the footer shows none (or posse cannot see one).
func (d AgentDetection) BackgroundWork() string {
	return backgroundWorkIn(d.regionPreview(footerRegion))
}

// backgroundWorkIn is the vocabulary over ONE footer region, whoever read the
// region. It is split out of BackgroundWork above rather than copied beside
// it because the capture route below reads the same line off a different
// store, and a second spelling of claude's task nouns is a second thing to
// keep in step with the binary — which is the whole objection ADR 0076
// records to screen rules on Rex ("posse would maintain a second detection
// engine"). One parser, two ways in.
func backgroundWorkIn(region string) string {
	for _, part := range strings.Split(region, footerSep) {
		if s := backgroundTaskSummary(part); s != "" {
			return s
		}
	}
	return ""
}

// backgroundTaskSummary answers whether one footer part IS a task summary,
// returning it trimmed of the glyph claude prefixes some of them with.
func backgroundTaskSummary(part string) string {
	s := strings.TrimSpace(part)
	// A leading glyph (the cloud-session and ultraplan summaries carry one)
	// is chrome, not vocabulary.
	if i := strings.IndexFunc(s, isSummaryWordStart); i > 0 {
		s = strings.TrimSpace(s[i:])
	}
	if s == "" {
		return ""
	}
	if backgroundTaskWord[s] || strings.HasPrefix(s, backgroundTaskPrefix) {
		return s
	}
	for _, one := range strings.Split(s, ", ") {
		if !backgroundTaskCount.MatchString(one) {
			return ""
		}
	}
	return s
}

// isSummaryWordStart marks where a task summary's own text begins: its
// first digit or ASCII letter. Everything before that is the glyph.
func isSummaryWordStart(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// composerChromePhrases are herdr's own five `not` clauses on the SAME
// region and mark (claude.toml, rule live_prompt_box) — adopted here for the
// same reason herdr carries them: prompt_box_body was seen carrying a
// prompt-box line AND a selection menu at once (a blocked form, a
// permission prompt, the model picker), so composerMark leading the region
// is not enough to tell the two apart. Without this, Composer returns the
// menu's own chrome as text "sitting UNSENT in its box" (Why()), and govern
// G2 files the row under settled-unsent for a pane that is blocked on the
// operator, not holding a lost keystroke (ranger-base-i6t90, from
// ranger-base-htafy). Matched lower-cased: herdr's manifest spells them
// lower-case, and a live capture is Title Case ("Enter to select").
var composerChromePhrases = []string{
	"enter to select", "esc to cancel", "tab/arrow keys",
	"arrow keys to navigate", "↑/↓ to navigate",
}

// Composer returns the text typed into the pane's prompt box, or "" when
// the box is empty, no box is on screen, or the region is a menu drawn over
// the box rather than the box itself.
func (d AgentDetection) Composer() string {
	body := strings.TrimLeft(d.regionPreview(composerRegion), " \t\n")
	if !strings.HasPrefix(body, composerMark) {
		return ""
	}
	lower := strings.ToLower(body)
	for _, phrase := range composerChromePhrases {
		if strings.Contains(lower, phrase) {
			return ""
		}
	}
	// U+00A0 is what claude draws between the mark and the text; TrimSpace
	// takes it, being a space in Unicode's book and Go's.
	return strings.TrimSpace(strings.TrimPrefix(body, composerMark))
}

// Hold reads both of them off one detection.
func (d AgentDetection) Hold() PaneHold {
	return PaneHold{Work: d.BackgroundWork(), Typed: d.Composer()}
}

// PaneHolding asks herdr what target's pane is holding. An explain that
// fails, or a herdr with no explain at all, answers the empty hold — see
// the note at the top of this file for why that is the only safe direction.
func (b *HerdrBackend) PaneHolding(target Seat) PaneHold {
	det, err := b.explain(target)
	if err != nil {
		// ranger-base-r5131. Two silences wear one error here, and only one
		// of them is ignorance. A herdr that WAS asked and failed is the
		// case the note above is about: news, possibly gone by the next
		// pass, and no evidence either way — the empty hold. A slot with no
		// `agent explain` AT ALL (ADR 0076 §4) is not that: nothing was
		// asked, nothing will be on any later pass, and the whole of this
		// reading was off for the life of the instance. muxVerbMissing is
		// the predicate written for exactly that distinction
		// (ranger-base-l32dl), and the capture route is what it reaches.
		if _, missing := muxVerbMissing(err); missing {
			return b.capturedHold(target)
		}
		return PaneHold{}
	}
	hold := det.Hold()
	hold.Read = ReadingEvidenceOf(det)
	if hold.Typed == "" {
		return hold
	}
	// ranger-base-2hvtv. A box previewing the line this pane LAST SUBMITTED
	// is echoing it, not holding it — claude's own submit log says which
	// (sentline.go, and the measurement behind it). Asked only when there is
	// text to ask about, so an empty composer still costs one `explain` and
	// nothing else, and answered "no" by every failure on the way: a pane
	// herdr will not name a claude session for, a store that will not open,
	// a session with no row in it.
	//
	// The STORE IS ASKED FIRST and returns on its own. It is the authority
	// for "was this submitted" (Helland, CIDR 2005: the screen is a derived
	// copy of a fact that store owns), where the reading below is the same
	// derived copy read a second way — so where both could speak, the one
	// with a store behind it does.
	if sess, err := b.paneAgentSession(target); err == nil {
		if sent, ok := lastSubmitted(b.ClaudeHistory, sess); ok && submittedEcho(hold.Typed, sent, det.ComposerTruncated()) {
			hold.Sent, hold.Typed = sent, ""
			return hold
		}
	}
	// ranger-base-6o7wm. The other way that text is nobody's to clear:
	// claude drew it. A suggestion is in NO store — claude logs submits and
	// never suggestions, so the echo reading above answers "not an echo" for
	// every one of them — and it previews in `prompt_box_body` character for
	// character like typed input. What tells them apart is that claude draws
	// its own suggestion FAINT, which herdr's ANSI-stripped preview flattens
	// away and one more read puts back (ghostbox.go, and the corpus behind
	// it). One `agent read` per settled holder whose box has text no store
	// claims, which is the grain the two calls above already work at; an
	// empty box still costs one `explain` and nothing else.
	if ansi, err := b.Mux().Read(target, 0, true); err == nil {
		ghost, read := composerGhostRead(ansi, hold.Typed)
		// The ansi composer line is recorded whether or not it retired the
		// claim: a ghost retirement and a box this reading declined to
		// retire are the two halves of ranger-base-6o7wm's missing
		// measurement, and a log that kept only the first half could never
		// close it (ADR 0066 D1).
		if read != "" {
			hold.Read.Regions = append(hold.Read.Regions, ReadingRegion{
				Name:  composerANSIRegion,
				Bytes: len(read),
				Text:  read,
			})
		}
		if ghost {
			hold.Ghost, hold.Typed = hold.Typed, ""
		}
	}
	return hold
}

// capturedHold is the WORK half of this file's reading on an occupant that
// has no `agent explain` — ranger-base-r5131, and the reason htafy's guard
// had to be realized twice.
//
// WHY IT EXISTS. htafy read both regions off herdr's `agent explain`
// previews, which is §4 and answers "not available" on a `multiplexer: rex`
// instance. So on this box every settled seat answered the empty hold, and
// the three consequences that hold withholds — the --resume skip
// (dispatch.go), the settle-open count (settleopen.go) and the pulse's G2
// row (govern.go) — all ran as though htafy had never landed. MEASURED
// 2026-11-10 on this host (linux, rex CLI 0.0.1794069975, claude 2.1.322),
// in the seat filing this bead, by backgrounding a declared recorder and
// ending the turn across it: at 20:15:53 the turn ended, dispatch counted a
// settle-open against ranger-base-r5131 at 01:15Z and re-prompted it, and a
// second one would have escalated this bead to the operator. That is the
// bug, reproduced on itself.
//
// WHY THE SCREEN, AND WHY THAT IS NOT ADR 0076's REJECTED ALTERNATIVE.
// Neither of Rex's own answers carries this fact, both MEASURED in the same
// run:
//
//   - `program_status` holds ONE root record across the whole settled
//     window — `state=done`, owner claude — and no child record. There is
//     nothing in it about the shell, so rexReading reads `done` correctly
//     and uselessly.
//   - `process` is the block-shell -> claude chain only: `child`,
//     `foreground` and the `ancestors` between them. A background shell is
//     a child of CLAUDE, not of the block, and never appeared in any
//     sample — nor did the gate shell taking them.
//
// claude's footer does carry it, verbatim and across the turn boundary:
// `⏵⏵ auto mode on · 1 shell` in every settled sample. ADR 0076 rejected
// "screen rules on Rex" and that stands: this is not a rule, not a detection
// engine and not a state. It reads ONE line claude publishes about itself,
// through §2's own `Read` verb, which both occupants have, and hands it to
// the vocabulary parser that was already here. It cannot produce a state,
// admit a runtime, or license a keystroke; the only thing it can do is what
// the note at the top of this file already licenses — say "this settle is
// not one". See ADR 0076, "Amendment 5".
//
// BOTH HALVES NOW, and the second one arrived on its own bead because it
// needed a store this one did not. The composer is on the same captured
// screen, but a Typed reading is only honest beside the echo check that
// tells a held prompt from the box showing the line this pane last
// submitted — and that check needs the runtime's own conversation id, which
// was §4's and herdr-only when this function was written. So Typed, Sent and
// Ghost stayed "" here, said out loud rather than guessed, and
// ranger-base-qzujz went and found Rex's store of record for it: claude's own
// `sessions/<pid>.json`, joined on the pid that holds the block
// (runtimesession.go, ADR 0076 Amendment 6). capturedcomposer.go is the
// reading; the one thing to know here is that it is taken off THIS capture,
// so the work and the composer are one instant and one snapshot.
func (b *HerdrBackend) capturedHold(target Seat) PaneHold {
	screen, err := b.Mux().Read(target, captureFooterLines, false)
	if err != nil {
		// The same direction as every other failure here: a screen posse
		// could not read is not evidence that an agent has stopped.
		return PaneHold{}
	}
	footer := captureFooter(screen)
	hold := PaneHold{Work: backgroundWorkIn(footer)}
	if hold.Work != "" {
		// The region is recorded whether or not herdr ever saw one, because
		// the consequence sites log `hold.Read` and a record that named no
		// region could not say what this withheld on (ADR 0066 D1). `Seen`
		// stays false: it means "herdr recognized a screen", and on this
		// route herdr was never asked.
		hold.Read.Regions = append(hold.Read.Regions, ReadingRegion{
			Name: captureFooterRegion, Bytes: len(footer), Text: footer,
		})
	}
	// ranger-base-qzujz: the composer half, off the SAME capture, with the
	// echo and ghost readings that keep it honest (capturedcomposer.go). It
	// is asked whatever the footer said, because the two halves are
	// independent facts about one screen — a seat with nothing running can
	// still be holding an unsent prompt, which is the whole of the pulse's
	// G2 `-unsent` row.
	hold = b.capturedComposerHold(target, screen, hold)
	if !hold.Waiting() && hold.Sent == "" && hold.Ghost == "" {
		// Nothing was read off this screen that any caller can spend, and
		// `PaneHold{}` is what every other failure arm in this file answers
		// with — so a screen that said nothing reads as a screen that said
		// nothing, regions and all.
		return PaneHold{}
	}
	return hold
}

// captureFooter is the lines after the screen's LAST horizontal rule: the
// same place herdr's manifest names `after_last_horizontal_rule`, located
// here rather than asked for, because the slot that needs it has no manifest
// to ask.
//
// POSITIONAL AND NOT A SEARCH, which is the part that keeps this safe. A
// scan of the whole screen for claude's task nouns would match this bead's
// own prose — a transcript line quoting `· 1 shell ·` would read as live
// work — and a hold invented out of a quotation keeps a genuinely stuck bead
// claimed forever, which is the failure the top of this file is written
// against. Anchoring on the rule means the only lines the vocabulary ever
// sees are the ones claude draws below its composer.
//
// MEASURED 2026-11-10, `rex capture --format text` of a live claude 2.1.322
// block: 60 lines, 8317 bytes, 21 ms, three runs byte-identical. The tail is
// a rule, `❯ ` and its U+00A0, a rule, a blank, then the footer — claude is
// on the alternate screen buffer, so the capture is the SCREEN and not the
// 64 MiB of scrollback `rex capture` documents for an ordinary block. The
// `lines` bound is passed anyway: it is what this reading needs, and it is
// the occupant's to honour rather than this function's to assume.
//
// No rule on screen returns "", and so does a screen whose last rule is its
// last line. Both are "posse saw no footer", which answers the empty hold.
func captureFooter(screen string) string {
	lines := strings.Split(screen, "\n")
	last := -1
	for i, l := range lines {
		if isFooterRule(l) {
			last = i
		}
	}
	if last < 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[last+1:], "\n"))
}

// isFooterRule is whether one captured line IS one of claude's rules: box
// drawing horizontal, and nothing else but the spaces around it. A line that
// merely CONTAINS a run of them is not one — the rule is the whole line, and
// a prose line drawing a long dash would otherwise move the footer.
func isFooterRule(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" {
		return false
	}
	return strings.Trim(s, string(footerRuleRune)) == ""
}

// sessionHolding is PaneHolding by session name, for the callers that have
// one and no pane. A session posse cannot find a pane for is not one it can
// read a screen for, and it holds nothing.
//
// The meta's own Seat is asked first — the workspace and root pane on
// record from session creation, which is the pane herdr's prompt and explain
// address (ranger-base-5qe6) — because it is a file read. AgentTarget is the
// fallback and costs a full `Sessions()` plus an `agent list`; a reading
// this cheap is what lets the callers ask per settled holder rather than
// once a pass.
func (b *HerdrBackend) sessionHolding(session string) PaneHold {
	if m, ok := b.readMeta(session); ok && m.Pane != "" {
		return b.PaneHolding(m.Seat())
	}
	target, err := b.AgentTarget(session)
	if err != nil {
		return PaneHold{}
	}
	return b.PaneHolding(target)
}

// promptSubmitPoll paces the read-back below, and promptSubmitWait bounds
// it. The bound is a REPORTING threshold and nothing rests on it: the box
// not having cleared inside it produces a line naming what is still in the
// box, never an action, so choosing it too short costs a warning the
// operator can see is wrong and choosing it too long costs a second of a
// hand command. Nobody has measured how long claude's composer takes to
// clear after herdr submits it; the decisions that MUST be right about an
// unsent prompt are made later and off a screen read with no race in it —
// gather's settle, the --resume skip, the pulse's G2 row.
const (
	promptSubmitPoll = 250 * time.Millisecond
	promptSubmitWait = 2 * time.Second
)

// ConfirmSubmitted reads the composer back after a prompt was submitted and
// returns the text still sitting in it, or "" when the box cleared (or when
// posse could not see one).
//
// MEASURED 2026-09-02, three occurrences: `herdr agent prompt`
// returned agent_prompted, the text was typed, and the submit never
// happened — the prompt sat in the box for four hours on the last of them
// while every store reported the agent idle. A caller that types and walks
// away has no way to know; this is the read-back the bead asked for.
// `herdr agent send-keys <pane> enter` on the stale text did NOT submit it
// (measured on the same session); a fresh `posse prompt --now` did. So this
// reports and does not attempt a recovery keystroke that has been shown not
// to work.
func (b *HerdrBackend) ConfirmSubmitted(target Seat) string {
	deadline := time.Now().Add(promptSubmitWait)
	for {
		det, err := b.explain(target)
		if err != nil {
			// ranger-base-um1bi, and it is the same split PaneHolding makes
			// above: a slot with NO `agent explain` at all (ADR 0076 §4) is
			// not the concession below. Nothing was asked, nothing will be
			// on any later poll, and this whole read-back was off for the
			// life of the instance — so the composer is read the way that
			// occupant can read it, off a capture.
			if _, missing := muxVerbMissing(err); missing {
				return b.confirmSubmittedCaptured(target, deadline)
			}
			// The same concession every reading in this file makes: a
			// diagnostic that will not answer is not evidence of a lost
			// keystroke.
			return ""
		}
		typed := det.Composer()
		if typed == "" {
			return ""
		}
		if !time.Now().Add(promptSubmitPoll).Before(deadline) {
			// ranger-base-6o7wm, and the reason this one is asked at the
			// deadline rather than in the loop: the box a SUCCESSFUL submit
			// leaves behind is usually not empty, because claude writes its
			// next suggestion into it — so the poll above runs out its two
			// seconds and this warned that posse's own delivered prompt was
			// "typed but not submitted". One ansi read, on the path that was
			// about to warn and nowhere else; a dim box warns nothing, and
			// every failure of the reading warns exactly as it did before.
			if ansi, err := b.Mux().Read(target, 0, true); err == nil && composerIsGhost(ansi, typed) {
				return ""
			}
			return typed
		}
		time.Sleep(promptSubmitPoll)
	}
}

// confirmSubmittedCaptured is ConfirmSubmitted's read-back on an occupant
// with no `agent explain` — ranger-base-um1bi, from ranger-base-qzujz.
//
// WHY A READING AND NOT A DEGRADE LINE. explainDegrade (muxherdr.go) is the
// other honest answer to a §4 refusal, and it is the wrong one here on its
// own terms: its note says the §4 cause is "unavailable on every launch by
// design", so a line printed for it is not news — it would print on every
// `posse prompt` on this box, forever, about a reading that IS takeable.
// `posse prompt` already prints one such line for this same missing verb
// (the readiness gate's, promptready.go), and a second one for the same
// cause on the same command is noise where the reading itself is one capture
// away. So: take the reading, print nothing extra, and report exactly what
// the herdr arm reports.
//
// WHAT IT IS WORTH, which is the question the bead asked first. Whether Rex's
// own Prompt verb can leave text in the box the way `herdr agent prompt` was
// MEASURED to (three times, 2026-09-02, the last sitting four hours) is NOT
// measured on this occupant. What is written down is the mechanism, on this
// exact delivery shape: ADR 0060 D5 measured a `send-text` + Enter route
// whose Enter the CLI's command picker ate, leaving the text in the composer
// "through Esc and C-u" while the call reported success — and RexMux.Prompt
// IS that shape (`rex send`, then `send-key enter`), which is why its own
// note ends "Rex has even less to catch it with than herdr — no envelope,
// and no detection to read the box back with". That sentence names this
// absence. Prompt refuses a leading `/` outright, so the one measured CAUSE
// cannot arrive through `posse prompt`; the generic type-then-Enter race the
// 2026-09-02 measurement caught had no `/` in it, and on this occupant its
// window is WIDER, because the gate that guards it is itself §4 and concedes
// here. The reading is report-only either way: if the failure mode turns out
// not to exist on Rex it costs one capture and never prints a word, and if it
// does, the alternative is a prompt sitting for hours with nothing saying so.
//
// WHAT IT COSTS. An empty or cleared box is one ~20ms capture per poll —
// ~160ms of rex calls spread across the two-second window. A box with text in
// it also pays the echo join and the dim read: three more occupant calls,
// which is capturedComposerHold's own price now that ranger-base-k0hd6 hands
// this path a Seat carrying both halves and RexMux.hold's resolution returns
// immediately. (That note's ~330ms per sample was measured when each of the
// three paid a pane-only seat's listing.) A genuinely held prompt costs four
// or five of those inside the window and a retired one costs exactly one,
// because the retirement answers "" on the first poll.
//
// NO SEPARATE GHOST ARM AT THE DEADLINE, unlike the herdr loop above. There
// the dim read is one extra `agent read` and is deferred to the one poll that
// is about to warn; here the dim read and the echo join are inside the
// reading, off the SAME capture, so by the time this sees a Typed at all both
// retirements have already run and there is nothing left to ask. The echo one
// is the better reading for this call site and the herdr arm does not have
// it: posse has just submitted, so a box showing the line claude's own log
// says was submitted last IS the successful submit, and the herdr arm reaches
// that only by waiting out the window and asking the dim question.
//
// IT REPORTS NOW: ranger-base-qzujz LANDED, which this note used to say it
// was waiting for. capturedHold filled Work and left Typed, Sent and Ghost
// empty on this route until that bead's composer reading reached the tree;
// the reading is here (capturedcomposer.go, replayed onto this route's own
// Seat signature under ranger-base-b34wr), so the route this function wired
// and pinned as a route is a route that answers.
func (b *HerdrBackend) confirmSubmittedCaptured(target Seat, deadline time.Time) string {
	for {
		typed := b.capturedHold(target).Typed
		if typed == "" {
			return ""
		}
		if !time.Now().Add(promptSubmitPoll).Before(deadline) {
			return typed
		}
		time.Sleep(promptSubmitPoll)
	}
}
